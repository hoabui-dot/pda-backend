package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/company/pda-backend/internal/execution/domain"
	"github.com/company/pda-backend/internal/execution/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
)

type TaskService struct {
	tasks       ports.TaskRepository
	idempotency ports.IdempotencyRepository
	outbox      ports.OutboxRepository
	tx          ports.TransactionManager
	publisher   event.DomainEventPublisher
	invalidator ports.ProjectionInvalidator
	now         func() time.Time
}

func NewTaskService(tasks ports.TaskRepository, idempotency ports.IdempotencyRepository, outbox ports.OutboxRepository, tx ports.TransactionManager, publisher event.DomainEventPublisher, invalidator ports.ProjectionInvalidator, now func() time.Time) *TaskService {
	return &TaskService{tasks, idempotency, outbox, tx, publisher, invalidator, now}
}

type TaskCommand struct {
	TaskID, IdempotencyKey string
	CommandID              uuid.UUID
	BaseVersion            int64
	Actor                  platform.ActorContext
}

func (s *TaskService) List(ctx context.Context, filter ports.TaskFilter, actor platform.ActorContext) (ports.TaskPage, error) {
	if filter.WarehouseID != actor.WarehouseID {
		return ports.TaskPage{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Warehouse access denied"}
	}
	filter.OperatorID = actor.OperatorID
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 20
	}
	return s.tasks.List(ctx, filter)
}
func (s *TaskService) Summary(ctx context.Context, warehouseID, status string, actor platform.ActorContext) ([]ports.SummaryItem, error) {
	if warehouseID != actor.WarehouseID {
		return nil, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Warehouse access denied"}
	}
	return s.tasks.Summary(ctx, warehouseID, actor.OperatorID, status)
}
func (s *TaskService) Dashboard(ctx context.Context, actor platform.ActorContext) (ports.Dashboard, error) {
	return s.tasks.Dashboard(ctx, actor.WarehouseID, actor.OperatorID)
}
func (s *TaskService) Claim(ctx context.Context, command TaskCommand) (domain.Task, error) {
	return s.mutate(ctx, command, "TaskAssigned", func(task *domain.Task) error {
		return task.Claim(command.Actor.OperatorID, command.BaseVersion, s.now())
	})
}
func (s *TaskService) Release(ctx context.Context, command TaskCommand) (domain.Task, error) {
	return s.mutate(ctx, command, "TaskReleased", func(task *domain.Task) error {
		return task.Release(command.Actor.OperatorID, command.BaseVersion, s.now())
	})
}

func (s *TaskService) mutate(ctx context.Context, command TaskCommand, eventType string, transition func(*domain.Task) error) (domain.Task, error) {
	var result ports.IdempotencyResult
	mutated := false
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		previous, found, err := s.idempotency.Find(txCtx, command.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			result = previous
			return nil
		}
		task, err := s.tasks.GetForUpdate(txCtx, command.TaskID)
		if err != nil {
			return err
		}
		if task.WarehouseID != command.Actor.WarehouseID {
			return &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Warehouse access denied"}
		}
		originalVersion := task.Version
		if err := transition(&task); err != nil {
			return err
		}
		if task.Version == originalVersion {
			result = ports.IdempotencyResult{Task: task}
			return s.idempotency.Save(txCtx, command.IdempotencyKey, result)
		}
		payload, _ := json.Marshal(map[string]any{"taskId": task.ID, "category": task.Category, "status": task.Status})
		envelope := event.DomainEventEnvelope{EventID: uuid.New(), EventType: eventType, EventVersion: 1, AggregateType: "WarehouseTask", AggregateID: task.ID, AggregateVersion: task.Version, OccurredAt: s.now().UTC(), CorrelationID: command.Actor.CorrelationID, CausationID: command.CommandID, WarehouseID: command.Actor.WarehouseID, OperatorID: command.Actor.OperatorID, DeviceID: command.Actor.DeviceID, Topic: "pda.task.events.v1", Payload: payload}
		if err := envelope.Validate(); err != nil {
			return fmt.Errorf("create task event: %w", err)
		}
		if err := s.tasks.Save(txCtx, task); err != nil {
			return err
		}
		if err := s.outbox.Append(txCtx, envelope); err != nil {
			return err
		}
		result = ports.IdempotencyResult{Task: task, Event: envelope}
		mutated = true
		return s.idempotency.Save(txCtx, command.IdempotencyKey, result)
	})
	if err != nil {
		return domain.Task{}, err
	}
	if mutated {
		if err := s.invalidator.InvalidateTaskViews(ctx, command.Actor.WarehouseID, command.Actor.OperatorID); err != nil {
			return domain.Task{}, err
		}
	}
	if mutated && result.Event.EventID != uuid.Nil {
		if err := s.publisher.Publish(ctx, result.Event); err != nil {
			return result.Task, &platform.DomainError{Code: "MESSAGING_PUBLISH_PENDING", SafeMessage: "Task committed; event publication pending", Retryable: true}
		}
	}
	return result.Task, nil
}
