package application

import (
	"context"
	"testing"
	"time"

	"github.com/company/pda-backend/internal/execution/adapters/memory"
	"github.com/company/pda-backend/internal/execution/domain"
	"github.com/company/pda-backend/internal/execution/ports"
	"github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	"github.com/company/pda-backend/internal/integration/adapters/wmsmock"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/google/uuid"
)

var testNow = time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC)

func taskSetup(t *testing.T) (*TaskService, *memory.Store, *messagingmock.InMemoryEventLog, platform.ActorContext) {
	t.Helper()
	tasks, err := wmsmock.NewTaskAdapter().Tasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	store := memory.New(tasks)
	log := messagingmock.NewInMemoryEventLog()
	service := NewTaskService(store, memory.Idempotency{Store: store}, store, store, messagingmock.NewPublisher(log, ""), store, func() time.Time { return testNow })
	actor := platform.ActorContext{OperatorID: "OP-01", DeviceID: "DEV-01", WarehouseID: "WH-01", CorrelationID: "CORR-01"}
	return service, store, log, actor
}

func TestDashboardSummaryPaginationAndFiltering(t *testing.T) {
	service, _, _, actor := taskSetup(t)
	dashboard, err := service.Dashboard(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Total != 3 || dashboard.Assigned != 1 || dashboard.HighPriority != 2 {
		t.Fatalf("dashboard: %+v", dashboard)
	}
	summary, err := service.Summary(context.Background(), "WH-01", "NEW", actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary) != 2 {
		t.Fatalf("summary: %+v", summary)
	}
	page, err := service.List(context.Background(), ports.TaskFilter{WarehouseID: "WH-01", Status: "NEW", Limit: 1}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Tasks) != 1 || page.NextCursor == nil {
		t.Fatalf("page1: %+v", page)
	}
	page2, err := service.List(context.Background(), ports.TaskFilter{WarehouseID: "WH-01", Status: "NEW", Limit: 1, Cursor: *page.NextCursor}, actor)
	if err != nil || len(page2.Tasks) != 1 || page2.Tasks[0].ID == page.Tasks[0].ID {
		t.Fatalf("page2: %+v %v", page2, err)
	}
	filtered, err := service.List(context.Background(), ports.TaskFilter{WarehouseID: "WH-01", Category: "PICKING", Limit: 20}, actor)
	if err != nil || len(filtered.Tasks) != 1 || filtered.Tasks[0].ID != "TASK-002" {
		t.Fatalf("filter: %+v %v", filtered, err)
	}
}

func TestClaimReleaseIdempotencyOutboxAndEvents(t *testing.T) {
	service, store, log, actor := taskSetup(t)
	key := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	command := TaskCommand{TaskID: "TASK-001", IdempotencyKey: key.String(), CommandID: key, BaseVersion: 1, Actor: actor}
	claimed, err := service.Claim(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != domain.StatusAssigned || claimed.Version != 2 || claimed.OperatorID == nil || *claimed.OperatorID != "OP-01" {
		t.Fatalf("claim: %+v", claimed)
	}
	replayed, err := service.Claim(context.Background(), command)
	if err != nil || replayed.Version != 2 {
		t.Fatalf("replay: %+v %v", replayed, err)
	}
	if len(store.Outbox()) != 1 || len(log.All(context.Background())) != 1 {
		t.Fatalf("outbox/events duplicated: %d %d", len(store.Outbox()), len(log.All(context.Background())))
	}
	releaseID := uuid.MustParse("00000000-0000-0000-0000-000000000202")
	released, err := service.Release(context.Background(), TaskCommand{TaskID: "TASK-001", IdempotencyKey: releaseID.String(), CommandID: releaseID, BaseVersion: 2, Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	if released.Status != domain.StatusNew || released.Version != 3 || released.OperatorID != nil {
		t.Fatalf("release: %+v", released)
	}
	if len(store.Outbox()) != 2 || len(log.All(context.Background())) != 2 {
		t.Fatal("release event missing")
	}
}

func TestTaskConflictsAndAuthorization(t *testing.T) {
	service, _, _, actor := taskSetup(t)
	commandID := uuid.New()
	_, err := service.Claim(context.Background(), TaskCommand{TaskID: "TASK-003", IdempotencyKey: commandID.String(), CommandID: commandID, BaseVersion: 2, Actor: actor})
	if !domain.Is(err, domain.ErrTaskLocked) {
		t.Fatalf("expected locked, got %v", err)
	}
	commandID = uuid.New()
	_, err = service.Claim(context.Background(), TaskCommand{TaskID: "TASK-001", IdempotencyKey: commandID.String(), CommandID: commandID, BaseVersion: 99, Actor: actor})
	if !domain.Is(err, domain.ErrVersionConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	commandID = uuid.New()
	_, err = service.Release(context.Background(), TaskCommand{TaskID: "TASK-002", IdempotencyKey: commandID.String(), CommandID: commandID, BaseVersion: 1, Actor: actor})
	if !domain.Is(err, domain.ErrTaskNotAssigned) {
		t.Fatalf("expected not assigned, got %v", err)
	}
	wrong := actor
	wrong.WarehouseID = "WH-02"
	_, err = service.Claim(context.Background(), TaskCommand{TaskID: "TASK-001", IdempotencyKey: uuid.NewString(), CommandID: uuid.New(), BaseVersion: 1, Actor: wrong})
	if err == nil {
		t.Fatal("expected warehouse denial")
	}
}
