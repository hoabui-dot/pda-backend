//go:build integration

package postgres

import (
	"context"
	"errors"
	"github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	platform "github.com/company/pda-backend/internal/platform/domain"
	shippingapp "github.com/company/pda-backend/internal/shipping/application"
	shippingdomain "github.com/company/pda-backend/internal/shipping/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"testing"
	"time"
)

func TestShippingReadinessConfirmationReplayAndEvents(t *testing.T) {
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, os.Getenv("PDA_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	_, e = admin.Exec(ctx, "DROP SCHEMA IF EXISTS be06_integration CASCADE;CREATE SCHEMA be06_integration")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA IF EXISTS be06_integration CASCADE") })
	cfg, _ := pgxpool.ParseConfig(os.Getenv("PDA_TEST_DATABASE_URL"))
	cfg.ConnConfig.RuntimeParams["search_path"] = "be06_integration"
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	for _, p := range []string{"../../../../migrations/execution/000001_task_core.up.sql", "../../../../migrations/execution/000002_receiving.up.sql", "../../../../migrations/execution/000003_movement_workflows.up.sql", "../../../../migrations/execution/000004_inventory_control.up.sql", "../../../../migrations/execution/000005_shipping.up.sql"} {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(b)); e != nil {
			t.Fatal(e)
		}
	}
	s := New(pool)
	if e = s.Seed(ctx); e != nil {
		t.Fatal(e)
	}
	log := messagingmock.NewInMemoryEventLog()
	svc := shippingapp.New(s, Commands{s}, s, s, s, messagingmock.NewPublisher(log, ""), s, func() time.Time { return time.Date(2026, 8, 1, 5, 0, 0, 0, time.UTC) })
	a := platform.ActorContext{OperatorID: "OP-01", DeviceID: "DEV", WarehouseID: "WH-01", CorrelationID: "CORR"}
	id := uuid.New()
	cmd := shippingapp.ConfirmCommand{ID: id, Key: id.String(), ShipmentID: "SHIP-001", Carrier: "DHL", Tracking: "TRACK-001", BaseVersion: 1, Actor: a}
	if _, e = svc.Confirm(ctx, cmd); !errors.Is(e, shippingdomain.ErrNotReady) {
		t.Fatalf("not ready %v", e)
	}
	if e = svc.ProjectPickingComplete(ctx, "SHIP-001"); e != nil {
		t.Fatal(e)
	}
	cmd.BaseVersion = 2
	one, e := svc.Confirm(ctx, cmd)
	if e != nil || one.Status != "SHIPPED" {
		t.Fatalf("confirm %+v %v", one, e)
	}
	two, e := svc.Confirm(ctx, cmd)
	if e != nil || two.Version != 3 {
		t.Fatalf("replay %+v %v", two, e)
	}
	var outbox, commands int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM domain_outbox WHERE aggregate_id='SHIP-001'").Scan(&outbox)
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM shipping_command_status WHERE command_id=$1", id).Scan(&commands)
	if outbox != 3 || commands != 1 || len(log.All(ctx)) != 3 {
		t.Fatalf("effects %d %d %d", outbox, commands, len(log.All(ctx)))
	}
	if _, e = svc.Confirm(ctx, shippingapp.ConfirmCommand{ID: uuid.New(), Key: uuid.NewString(), ShipmentID: "SHIP-001", Carrier: "DHL", Tracking: "TRACK-002", BaseVersion: 2, Actor: a}); !errors.Is(e, shippingdomain.ErrVersion) {
		t.Fatalf("stale %v", e)
	}
}
