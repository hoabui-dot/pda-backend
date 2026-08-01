//go:build integration

package postgres

import (
	"context"
	"errors"
	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	movementdomain "github.com/company/pda-backend/internal/execution/movement/domain"
	"github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	"github.com/company/pda-backend/internal/integration/adapters/wmsmock"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"testing"
	"time"
)

func TestAllMovementWorkflowsRetryInventoryAndOrdering(t *testing.T) {
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, os.Getenv("PDA_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	_, e = admin.Exec(ctx, "DROP SCHEMA IF EXISTS be04_integration CASCADE;CREATE SCHEMA be04_integration")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA IF EXISTS be04_integration CASCADE") })
	cfg, e := pgxpool.ParseConfig(os.Getenv("PDA_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "be04_integration"
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	for _, p := range []string{"../../../../../migrations/execution/000001_task_core.up.sql", "../../../../../migrations/execution/000002_receiving.up.sql", "../../../../../migrations/execution/000003_movement_workflows.up.sql"} {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(b)); e != nil {
			t.Fatal(e)
		}
	}
	store := New(pool)
	if e = store.Seed(ctx, wmsmock.MovementTasks()); e != nil {
		t.Fatal(e)
	}
	log := messagingmock.NewInMemoryEventLog()
	svc := movementapp.New(store, Commands{store}, store, store, store, messagingmock.NewPublisher(log, ""), store, func() time.Time { return time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC) })
	a := platform.ActorContext{OperatorID: "OP-01", DeviceID: "DEV-01", WarehouseID: "WH-01", CorrelationID: "CORR-04"}
	cmd := func(id string, v int64) movementapp.Command {
		u := uuid.New()
		return movementapp.Command{CommandID: u, IdempotencyKey: u.String(), TaskID: id, BaseVersion: v, Actor: a}
	}
	putTask, _ := store.Get(ctx, "PUT-001")
	if err := store.CheckMove(ctx, putTask, 999); !errors.Is(err, movementdomain.ErrStock) {
		t.Fatalf("insufficient stock: %v", err)
	}
	incompatible := putTask
	incompatible.DestinationLocation = "PICK-02"
	if err := store.CheckMove(ctx, incompatible, 1); !errors.Is(err, movementdomain.ErrCapacity) {
		t.Fatalf("compatibility/capacity: %v", err)
	}
	p1, e := svc.Putaway.ValidateSource(ctx, cmd("PUT-001", 1), "STAGE-01")
	if e != nil || p1.Version != 2 {
		t.Fatalf("put source %+v %v", p1, e)
	}
	p2, e := svc.Putaway.ValidateDestination(ctx, cmd("PUT-001", 2), "BULK-01")
	if e != nil {
		t.Fatal(e)
	}
	pc := cmd("PUT-001", p2.Version)
	p3, e := svc.Putaway.Confirm(ctx, pc, 5)
	if e != nil || p3.Status != movementdomain.Completed {
		t.Fatalf("put confirm %+v %v", p3, e)
	}
	replay, e := svc.Putaway.Confirm(ctx, pc, 5)
	if e != nil || replay.Version != p3.Version {
		t.Fatalf("replay %+v %v", replay, e)
	}
	var wg sync.WaitGroup
	concurrent := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Picking.ValidateLocation(ctx, cmd("PICK-001", 1), "PICK-01")
			concurrent <- err
		}()
	}
	wg.Wait()
	close(concurrent)
	successes, conflicts := 0, 0
	for err := range concurrent {
		if err == nil {
			successes++
		} else if errors.Is(err, movementdomain.ErrVersion) {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent task conflict success=%d conflict=%d", successes, conflicts)
	}
	pick, e := store.Get(ctx, "PICK-001")
	if e != nil {
		t.Fatal(e)
	}
	pick, e = svc.Picking.ResolveBarcode(ctx, cmd("PICK-001", pick.Version), "PICK-ITEM-001")
	if e != nil {
		t.Fatal(e)
	}
	pick, e = svc.Picking.Confirm(ctx, cmd("PICK-001", pick.Version), 4)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = svc.Picking.Complete(ctx, cmd("PICK-001", pick.Version)); e != nil {
		t.Fatal(e)
	}
	rep, e := svc.Replenishment.ValidateSource(ctx, cmd("REP-001", 1), "BULK-01")
	if e != nil {
		t.Fatal(e)
	}
	rep, e = svc.Replenishment.ValidateDestination(ctx, cmd("REP-001", rep.Version), "PICK-02")
	if e != nil {
		t.Fatal(e)
	}
	rep, e = svc.Replenishment.ValidateItem(ctx, cmd("REP-001", rep.Version), "REP-ITEM-002")
	if e != nil {
		t.Fatal(e)
	}
	rep, e = svc.Replenishment.Confirm(ctx, cmd("REP-001", rep.Version), 2)
	if e != nil || rep.Status != movementdomain.PartiallyCompleted {
		t.Fatalf("partial %+v %v", rep, e)
	}
	rep, e = svc.Replenishment.Confirm(ctx, cmd("REP-001", rep.Version), 4)
	if e != nil || rep.Status != movementdomain.Completed {
		t.Fatalf("complete %+v %v", rep, e)
	}
	var moves, putCommands int
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM inventory_movement").Scan(&moves); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM movement_command_status WHERE workflow='PUTAWAY'").Scan(&putCommands); e != nil {
		t.Fatal(e)
	}
	if moves != 4 || putCommands != 3 {
		t.Fatalf("moves=%d put commands=%d", moves, putCommands)
	}
	rows, e := pool.Query(ctx, "SELECT aggregate_version FROM inventory_movement WHERE task_id='REP-001' ORDER BY occurred_at,aggregate_version")
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	var versions []int64
	for rows.Next() {
		var v int64
		_ = rows.Scan(&v)
		versions = append(versions, v)
	}
	if len(versions) != 2 || versions[0] >= versions[1] {
		t.Fatalf("aggregate ordering %v", versions)
	}
	stale := cmd("REP-001", 1)
	if _, e = svc.Replenishment.Confirm(ctx, stale, 1); !errors.Is(e, movementdomain.ErrVersion) {
		t.Fatalf("stale %v", e)
	}
	if len(log.All(ctx)) != 12 {
		t.Fatalf("events=%d", len(log.All(ctx)))
	}
}
