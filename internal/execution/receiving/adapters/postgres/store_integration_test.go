//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	receivingapp "github.com/company/pda-backend/internal/execution/receiving/application"
	receivingdomain "github.com/company/pda-backend/internal/execution/receiving/domain"
	"github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	"github.com/company/pda-backend/internal/integration/adapters/wmsmock"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReceivingTransactionIdempotencyInventoryOutboxAuditAndConcurrency(t *testing.T) {
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, os.Getenv("PDA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	_, err = admin.Exec(ctx, `DROP SCHEMA IF EXISTS be03_integration CASCADE;CREATE SCHEMA be03_integration`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS be03_integration CASCADE`) })
	config, err := pgxpool.ParseConfig(os.Getenv("PDA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = "be03_integration"
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, path := range []string{"../../../../../migrations/execution/000001_task_core.up.sql", "../../../../../migrations/execution/000002_receiving.up.sql"} {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := wmsmock.NewReceivingAdapter().Tasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	if err := store.Seed(ctx, tasks); err != nil {
		t.Fatal(err)
	}
	log := messagingmock.NewInMemoryEventLog()
	service := receivingapp.New(store, Commands{Store: store}, store, store, store, messagingmock.NewPublisher(log, ""), store, func() time.Time { return time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC) })
	actor := platform.ActorContext{OperatorID: "OP-01", DeviceID: "DEV-01", WarehouseID: "WH-01", CorrelationID: "CORR-03"}
	startID := uuid.New()
	started, err := service.Start(ctx, receivingapp.Command{CommandID: startID, IdempotencyKey: startID.String(), TaskID: "REC-001", BaseVersion: 1, Actor: actor})
	if err != nil || started.Version != 2 {
		t.Fatalf("start: %+v %v", started, err)
	}
	if _, err := service.ResolveBarcode(ctx, "REC-001", "UNKNOWN", actor); !errors.Is(err, receivingdomain.ErrBarcodeUnknown) {
		t.Fatalf("unknown barcode: %v", err)
	}
	if _, err := service.ResolveBarcode(ctx, "REC-001", "00012345678929", actor); !errors.Is(err, receivingdomain.ErrBarcodeWrongContext) {
		t.Fatalf("wrong document barcode: %v", err)
	}
	confirmID := uuid.New()
	confirm := receivingapp.ConfirmCommand{Command: receivingapp.Command{CommandID: confirmID, IdempotencyKey: confirmID.String(), TaskID: "REC-001", BaseVersion: 2, Actor: actor}, LineID: "LINE-01", Barcode: "00012345678905", Quantity: 3}
	confirmed, err := service.Confirm(ctx, confirm)
	if err != nil || confirmed.Version != 3 {
		t.Fatalf("confirm: %+v %v", confirmed, err)
	}
	replay, err := service.Confirm(ctx, confirm)
	if err != nil || replay.Version != 3 {
		t.Fatalf("replay: %+v %v", replay, err)
	}
	var inventory, outbox, audit, commands int
	if err := pool.QueryRow(ctx, `SELECT quantity FROM inventory_balance WHERE warehouse_id='WH-01' AND item_id='ITEM-001'`).Scan(&inventory); err != nil {
		t.Fatal(err)
	}
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM domain_outbox WHERE aggregate_id='REC-001'`).Scan(&outbox)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_record WHERE aggregate_id='REC-001'`).Scan(&audit)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM receiving_command_status WHERE command_id=$1`, confirmID).Scan(&commands)
	if inventory != 3 || outbox != 2 || audit != 2 || commands != 1 || len(log.All(ctx)) != 2 {
		t.Fatalf("effects inventory=%d outbox=%d audit=%d commands=%d events=%d", inventory, outbox, audit, commands, len(log.All(ctx)))
	}
	remark := "partial"
	_, err = service.Confirm(ctx, receivingapp.ConfirmCommand{Command: receivingapp.Command{CommandID: confirmID, IdempotencyKey: uuid.NewString(), TaskID: "REC-001", BaseVersion: 3, Actor: actor}, LineID: "LINE-02", Barcode: "00012345678912", Quantity: 1, Remark: &remark})
	if err == nil {
		t.Fatal("expected duplicate command ID transaction failure")
	}
	var rolledBackInventory int64
	_ = pool.QueryRow(ctx, `SELECT COALESCE((SELECT quantity FROM inventory_balance WHERE warehouse_id='WH-01' AND item_id='ITEM-002'),0)`).Scan(&rolledBackInventory)
	var rolledBackOutbox int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM domain_outbox WHERE aggregate_id='REC-001'`).Scan(&rolledBackOutbox)
	taskAfterRollback, err := store.Get(ctx, "REC-001")
	if err != nil {
		t.Fatal(err)
	}
	if rolledBackInventory != 0 || rolledBackOutbox != 2 || taskAfterRollback.Version != 3 {
		t.Fatalf("atomic rollback inventory=%d outbox=%d version=%d", rolledBackInventory, rolledBackOutbox, taskAfterRollback.Version)
	}
	// Two distinct commands race on the same aggregate version; exactly one may commit.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, line := range []struct{ id, barcode string }{{"LINE-02", "00012345678912"}, {"LINE-02", "00012345678912"}} {
		wg.Add(1)
		go func(lineID, barcode string) {
			defer wg.Done()
			id := uuid.New()
			_, err := service.Confirm(ctx, receivingapp.ConfirmCommand{Command: receivingapp.Command{CommandID: id, IdempotencyKey: id.String(), TaskID: "REC-001", BaseVersion: 3, Actor: actor}, LineID: lineID, Barcode: barcode, Quantity: 2})
			errs <- err
		}(line.id, line.barcode)
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, receivingdomain.ErrVersionConflict) {
			conflict++
		} else {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("concurrency success=%d conflict=%d", success, conflict)
	}
	var denied int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_record WHERE aggregate_id='REC-001' AND outcome='DENIED'`).Scan(&denied); err != nil || denied < 1 {
		t.Fatalf("denied audit=%d err=%v", denied, err)
	}
	status, err := service.CommandStatus(ctx, confirmID, actor)
	if err != nil || status.Status != "COMPLETED" {
		t.Fatalf("command status: %+v %v", status, err)
	}
}
