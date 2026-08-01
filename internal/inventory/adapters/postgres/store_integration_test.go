//go:build integration

package postgres

import (
	"context"
	"errors"
	movementpostgres "github.com/company/pda-backend/internal/execution/movement/adapters/postgres"
	"github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	"github.com/company/pda-backend/internal/integration/adapters/wmsmock"
	inventoryapp "github.com/company/pda-backend/internal/inventory/application"
	inventorydomain "github.com/company/pda-backend/internal/inventory/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"testing"
	"time"
)

func TestInventoryTransferInquiryCycleCountAndNoAutoAdjust(t *testing.T) {
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, os.Getenv("PDA_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	_, e = admin.Exec(ctx, "DROP SCHEMA IF EXISTS be05_integration CASCADE;CREATE SCHEMA be05_integration")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA IF EXISTS be05_integration CASCADE") })
	cfg, e := pgxpool.ParseConfig(os.Getenv("PDA_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "be05_integration"
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	for _, p := range []string{"../../../../migrations/execution/000001_task_core.up.sql", "../../../../migrations/execution/000002_receiving.up.sql", "../../../../migrations/execution/000003_movement_workflows.up.sql", "../../../../migrations/execution/000004_inventory_control.up.sql"} {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(b)); e != nil {
			t.Fatal(e)
		}
	}
	ms := movementpostgres.New(pool)
	if e = ms.Seed(ctx, wmsmock.MovementTasks()); e != nil {
		t.Fatal(e)
	}
	store := New(pool)
	if e = store.Seed(ctx); e != nil {
		t.Fatal(e)
	}
	events := messagingmock.NewInMemoryEventLog()
	svc := inventoryapp.New(store, Commands{store}, store, store, store, messagingmock.NewPublisher(events, ""), store, func() time.Time { return time.Date(2026, 8, 1, 4, 0, 0, 0, time.UTC) })
	a := platform.ActorContext{OperatorID: "OP-01", DeviceID: "DEV-01", WarehouseID: "WH-01", CorrelationID: "CORR-05"}
	if e = store.ValidateLocations(ctx, "WH-01", "PICK-01", "PICK-01", "ITEM-001", 1); !errors.Is(e, inventorydomain.ErrSameLocation) {
		t.Fatalf("same %v", e)
	}
	if e = store.ValidateLocations(ctx, "WH-01", "PICK-01", "UNKNOWN", "ITEM-001", 1); !errors.Is(e, inventorydomain.ErrDestination) {
		t.Fatalf("destination %v", e)
	}
	if e = store.ValidateLocations(ctx, "WH-01", "PICK-01", "STAGE-01", "ITEM-001", 999); !errors.Is(e, inventorydomain.ErrStock) {
		t.Fatalf("stock %v", e)
	}
	before, _ := store.Balances(ctx, "WH-01", "ITEM-001", "")
	sum := func(xs []inventorydomain.Balance) int64 {
		var n int64
		for _, x := range xs {
			n += x.Quantity
		}
		return n
	}
	id := uuid.New()
	c := inventoryapp.TransferCommand{Command: inventoryapp.Command{ID: id, Key: id.String(), Actor: a}, Source: "PICK-01", Destination: "STAGE-01", Item: "ITEM-001", Quantity: 2}
	one, e := svc.Transfer(ctx, c)
	if e != nil {
		t.Fatal(e)
	}
	two, e := svc.Transfer(ctx, c)
	if e != nil || one.CommandID != two.CommandID {
		t.Fatalf("replay %+v %v", two, e)
	}
	after, _ := store.Balances(ctx, "WH-01", "ITEM-001", "")
	if sum(before) != sum(after) {
		t.Fatalf("reconcile %d %d", sum(before), sum(after))
	}
	history, e := store.Movements(ctx, "WH-01", "ITEM-001", "")
	if e != nil || len(history) != 1 {
		t.Fatalf("history %d %v", len(history), e)
	}
	var invBefore int64
	_ = pool.QueryRow(ctx, "SELECT quantity FROM inventory_location_balance WHERE warehouse_id='WH-01' AND location_id='PICK-01' AND item_id='ITEM-001'").Scan(&invBefore)
	cmd := func(v int64) inventoryapp.Command {
		u := uuid.New()
		return inventoryapp.Command{ID: u, Key: u.String(), BaseVersion: v, Actor: a}
	}
	count, e := svc.SubmitCount(ctx, "CC-001", "CC-LINE-01", 1, cmd(1))
	if e != nil || !count.Lines[0].RecountRequired {
		t.Fatalf("variance %+v %v", count, e)
	}
	var invAfter int64
	_ = pool.QueryRow(ctx, "SELECT quantity FROM inventory_location_balance WHERE warehouse_id='WH-01' AND location_id='PICK-01' AND item_id='ITEM-001'").Scan(&invAfter)
	if invBefore != invAfter {
		t.Fatalf("variance adjusted stock %d %d", invBefore, invAfter)
	}
	count, e = svc.Recount(ctx, "CC-001", "CC-LINE-01", cmd(2))
	if e != nil {
		t.Fatal(e)
	}
	count, e = svc.SubmitCount(ctx, "CC-001", "CC-LINE-01", 16, cmd(3))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.CompleteCount(ctx, "CC-001", cmd(4)); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			u := uuid.New()
			_, x := svc.Transfer(ctx, inventoryapp.TransferCommand{Command: inventoryapp.Command{ID: u, Key: u.String(), Actor: a}, Source: "PICK-01", Destination: "STAGE-01", Item: "ITEM-001", Quantity: 8})
			errs <- x
		}()
	}
	wg.Wait()
	close(errs)
	successes, stockConflicts := 0, 0
	for x := range errs {
		if x == nil {
			successes++
		} else if errors.Is(x, inventorydomain.ErrStock) {
			stockConflicts++
		}
	}
	if successes != 1 || stockConflicts != 1 {
		t.Fatalf("concurrent transfer success=%d stock=%d", successes, stockConflicts)
	}
	if len(events.All(ctx)) != 6 {
		t.Fatalf("events %d", len(events.All(ctx)))
	}
}
