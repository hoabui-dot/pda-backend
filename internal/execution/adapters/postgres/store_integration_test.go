//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/company/pda-backend/internal/execution/domain"
	"github.com/company/pda-backend/internal/execution/ports"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrationRepositoryTransactionAndRollback(t *testing.T) {
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, os.Getenv("PDA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err = admin.Exec(ctx, `DROP SCHEMA IF EXISTS be02_integration CASCADE; CREATE SCHEMA be02_integration`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS be02_integration CASCADE`) })
	poolConfig, err := pgxpool.ParseConfig(os.Getenv("PDA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = "be02_integration"
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	down, err := os.ReadFile("../../../../migrations/execution/000001_task_core.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, err := os.ReadFile("../../../../migrations/execution/000001_task_core.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, string(down)) })
	if _, err = pool.Exec(ctx, `INSERT INTO warehouse_task(task_id,category,status,priority,warehouse_id,version,updated_at) VALUES('TASK-DB','RECEIVING','NEW',90,'WH-01',1,$1)`, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	idem := Idempotency{Store: store}
	eventID := uuid.New()
	envelope := event.DomainEventEnvelope{EventID: eventID, EventType: "TaskAssigned", EventVersion: 1, AggregateType: "WarehouseTask", AggregateID: "TASK-DB", AggregateVersion: 2, OccurredAt: time.Now().UTC(), CorrelationID: "CORR", CausationID: uuid.New(), WarehouseID: "WH-01", OperatorID: "OP-01", DeviceID: "DEV-01", Topic: "pda.task.events.v1", Payload: json.RawMessage(`{}`)}
	err = store.WithinTransaction(ctx, func(tx context.Context) error {
		task, err := store.GetForUpdate(tx, "TASK-DB")
		if err != nil {
			return err
		}
		operator := "OP-01"
		task.OperatorID = &operator
		task.Status = domain.StatusAssigned
		task.Version = 2
		if err := store.Save(tx, task); err != nil {
			return err
		}
		result := ports.IdempotencyResult{Task: task, Event: envelope}
		if err := idem.Save(tx, "KEY-1", result); err != nil {
			return err
		}
		return store.Append(tx, envelope)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result, found, err := idem.Find(ctx, "KEY-1"); err != nil || !found || result.Task.Version != 2 {
		t.Fatalf("idempotency result: %+v %v %v", result, found, err)
	}
	var outboxCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM domain_outbox WHERE event_id=$1`, eventID).Scan(&outboxCount); err != nil || outboxCount != 1 {
		t.Fatalf("outbox: %d %v", outboxCount, err)
	}
	rollbackErr := store.WithinTransaction(ctx, func(tx context.Context) error {
		task, err := store.GetForUpdate(tx, "TASK-DB")
		if err != nil {
			return err
		}
		task.Version = 3
		if err := store.Save(tx, task); err != nil {
			return err
		}
		return context.Canceled
	})
	if rollbackErr == nil {
		t.Fatal("expected rollback")
	}
	task, err := store.GetForUpdate(ctx, "TASK-DB")
	if err != nil {
		t.Fatal(err)
	}
	if task.Version != 2 {
		t.Fatalf("rollback failed, version=%d", task.Version)
	}
	if _, err = pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	var table *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('warehouse_task')`).Scan(&table); err != nil {
		t.Fatal(err)
	}
	if table != nil {
		t.Fatal("down migration did not remove warehouse_task")
	}
}
