package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/company/pda-backend/internal/execution/receiving/domain"
	"github.com/company/pda-backend/internal/execution/receiving/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
)

type Service struct {
	repo        ports.Repository
	commands    ports.CommandRepository
	outbox      ports.Outbox
	audit       ports.Audit
	tx          ports.TransactionManager
	publisher   event.DomainEventPublisher
	invalidator ports.Invalidator
	now         func() time.Time
}

func New(repo ports.Repository, commands ports.CommandRepository, outbox ports.Outbox, audit ports.Audit, tx ports.TransactionManager, publisher event.DomainEventPublisher, invalidator ports.Invalidator, now func() time.Time) *Service {
	return &Service{repo, commands, outbox, audit, tx, publisher, invalidator, now}
}

type Command struct {
	CommandID              uuid.UUID
	IdempotencyKey, TaskID string
	BaseVersion            int64
	Actor                  platform.ActorContext
}
type ConfirmCommand struct {
	Command
	LineID, Barcode string
	Quantity        int64
	Condition       string
	Remark          *string
	ScannedAt       *time.Time
}

func (s *Service) List(ctx context.Context, f ports.Filter, actor platform.ActorContext) (ports.Page, error) {
	if f.WarehouseID != actor.WarehouseID {
		return ports.Page{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Warehouse access denied"}
	}
	f.OperatorID = actor.OperatorID
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}
	return s.repo.List(ctx, f)
}
func (s *Service) Detail(ctx context.Context, id string, actor platform.ActorContext) (domain.Task, error) {
	task, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	if task.WarehouseID != actor.WarehouseID || (task.OperatorID != nil && *task.OperatorID != actor.OperatorID) {
		return domain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Receiving task access denied"}
	}
	return task, nil
}

// Claim is the receiving workflow boundary. Local mode keeps the existing
// durable task claim semantics; HTTP mode maps this call to Inbound's receipt
// assignment claim instead of creating a local business record.
func (s *Service) Claim(ctx context.Context, command Command) (domain.Task, error) {
	return s.Start(ctx, command)
}
func (s *Service) ResolveBarcode(ctx context.Context, taskID, barcode string, actor platform.ActorContext) (domain.Line, error) {
	task, err := s.Detail(ctx, taskID, actor)
	if err != nil {
		return domain.Line{}, err
	}
	if task.Status != domain.StatusInProgress && task.Status != domain.StatusPartiallyCompleted {
		return domain.Line{}, domain.ErrNotAssigned
	}
	return s.repo.ResolveBarcode(ctx, taskID, barcode)
}
func (s *Service) CommandStatus(ctx context.Context, id uuid.UUID, actor platform.ActorContext) (ports.CommandStatus, error) {
	status, found, err := s.commands.FindByID(ctx, id)
	if err != nil {
		return ports.CommandStatus{}, err
	}
	if !found {
		return ports.CommandStatus{}, domain.ErrNotFound
	}
	if status.WarehouseID != actor.WarehouseID || status.OperatorID != actor.OperatorID {
		return ports.CommandStatus{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Command access denied"}
	}
	return status, nil
}
func (s *Service) Start(ctx context.Context, c Command) (domain.Task, error) {
	return s.mutate(ctx, c, "ReceivingTaskStarted", "START", func(t *domain.Task) (string, int64, error) {
		return "", 0, t.Start(c.Actor.OperatorID, c.BaseVersion, s.now())
	})
}
func (s *Service) Confirm(ctx context.Context, c ConfirmCommand) (domain.Task, error) {
	return s.mutate(ctx, c.Command, "ReceivingQuantityConfirmed", "CONFIRM_QUANTITY", func(t *domain.Task) (string, int64, error) {
		var before int64
		for _, line := range t.Lines {
			if line.ID == c.LineID {
				before = line.ReceivedQuantity
			}
		}
		line, err := t.ConfirmWithCondition(c.Actor.OperatorID, c.LineID, c.Barcode, c.Quantity, c.Condition, c.Remark, c.BaseVersion, s.now())
		if err != nil {
			return "", 0, err
		}
		return line.ItemID, line.ReceivedQuantity - before, nil
	})
}
func (s *Service) Complete(ctx context.Context, c Command) (domain.Task, error) {
	return s.mutate(ctx, c, "ReceivingTaskCompleted", "COMPLETE", func(t *domain.Task) (string, int64, error) {
		return "", 0, t.Complete(c.Actor.OperatorID, c.BaseVersion, s.now())
	})
}

func (s *Service) mutate(ctx context.Context, c Command, eventType, commandType string, transition func(*domain.Task) (string, int64, error)) (domain.Task, error) {
	var result domain.Task
	var envelope event.DomainEventEnvelope
	mutated := false
	err := s.tx.WithinTransaction(ctx, func(tx context.Context) error {
		previous, found, err := s.commands.FindByKey(tx, c.IdempotencyKey)
		if err != nil {
			return err
		}
		if found {
			return json.Unmarshal(previous.Result, &result)
		}
		task, err := s.repo.GetForUpdate(tx, c.TaskID)
		if err != nil {
			return err
		}
		if task.WarehouseID != c.Actor.WarehouseID {
			return &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Warehouse access denied"}
		}
		itemID, delta, err := transition(&task)
		if err != nil {
			return err
		}
		payload := map[string]any{"taskId": task.ID, "status": task.Status}
		payloadJSON, _ := json.Marshal(payload)
		envelope = event.DomainEventEnvelope{EventID: uuid.New(), EventType: eventType, EventVersion: 1, AggregateType: "ReceivingTask", AggregateID: task.ID, AggregateVersion: task.Version, OccurredAt: s.now().UTC(), CorrelationID: c.Actor.CorrelationID, CausationID: c.CommandID, WarehouseID: c.Actor.WarehouseID, OperatorID: c.Actor.OperatorID, DeviceID: c.Actor.DeviceID, Topic: "pda.receiving.events.v1", Payload: payloadJSON}
		if err := envelope.Validate(); err != nil {
			return fmt.Errorf("event envelope: %w", err)
		}
		if err := s.repo.Save(tx, task); err != nil {
			return err
		}
		if delta > 0 {
			if err := s.repo.ApplyInventory(tx, task.WarehouseID, itemID, delta); err != nil {
				return err
			}
		}
		if err := s.outbox.Append(tx, envelope); err != nil {
			return err
		}
		if err := s.audit.AppendAudit(tx, eventType, "SUCCESS", task, c.Actor, payloadJSON); err != nil {
			return err
		}
		result = task
		encoded, _ := json.Marshal(result)
		status := ports.CommandStatus{CommandID: c.CommandID, CommandType: commandType, Status: "COMPLETED", WarehouseID: c.Actor.WarehouseID, OperatorID: c.Actor.OperatorID, Result: encoded}
		if err := s.commands.Save(tx, c.IdempotencyKey, status); err != nil {
			return err
		}
		mutated = true
		return nil
	})
	if err != nil {
		code := "INTERNAL_ERROR"
		if domainErr, ok := err.(*platform.DomainError); ok {
			code = domainErr.Code
		}
		if auditErr := s.audit.AppendDenied(ctx, eventType, c.TaskID, c.Actor, code); auditErr != nil {
			return domain.Task{}, auditErr
		}
		return domain.Task{}, err
	}
	if mutated {
		// The transaction is committed above; cache invalidation is best effort.
		_ = s.invalidator.InvalidateReceivingViews(ctx, c.Actor.WarehouseID, c.Actor.OperatorID)
		if err := s.publisher.Publish(ctx, envelope); err != nil {
			return result, &platform.DomainError{Code: "MESSAGING_PUBLISH_PENDING", SafeMessage: "Receiving committed; event publication pending", Retryable: true}
		}
	}
	return result, nil
}
