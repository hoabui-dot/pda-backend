package application

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/company/pda-backend/internal/execution/movement/domain"
	"github.com/company/pda-backend/internal/execution/movement/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
	"time"
)

type dependencies struct {
	repo        ports.Repository
	commands    ports.Commands
	tx          ports.TransactionManager
	outbox      ports.Outbox
	audit       ports.Audit
	publisher   event.DomainEventPublisher
	invalidator ports.Invalidator
	now         func() time.Time
}
type Services struct {
	Putaway       *PutawayService
	Picking       *PickingService
	Replenishment *ReplenishmentService
}

func New(repo ports.Repository, commands ports.Commands, tx ports.TransactionManager, outbox ports.Outbox, audit ports.Audit, publisher event.DomainEventPublisher, invalidator ports.Invalidator, now func() time.Time) *Services {
	d := &dependencies{repo, commands, tx, outbox, audit, publisher, invalidator, now}
	return &Services{&PutawayService{d}, &PickingService{d}, &ReplenishmentService{d}}
}

type Command struct {
	CommandID              uuid.UUID
	IdempotencyKey, TaskID string
	BaseVersion            int64
	Actor                  platform.ActorContext
}
type PutawayService struct{ d *dependencies }
type PickingService struct{ d *dependencies }
type ReplenishmentService struct{ d *dependencies }

func (s *Services) CommandStatus(ctx context.Context, id uuid.UUID, actor platform.ActorContext) (ports.CommandResult, error) {
	result, found, err := s.Putaway.d.commands.Find(ctx, id.String())
	if err != nil {
		return ports.CommandResult{}, err
	}
	if !found {
		return ports.CommandResult{}, domain.ErrNotFound
	}
	if result.WarehouseID != actor.WarehouseID || result.OperatorID != actor.OperatorID {
		return ports.CommandResult{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Command access denied"}
	}
	return result, nil
}

func list(ctx context.Context, d *dependencies, w domain.Workflow, a platform.ActorContext) ([]domain.Task, error) {
	return d.repo.List(ctx, w, a.WarehouseID, a.OperatorID)
}
func detail(ctx context.Context, d *dependencies, w domain.Workflow, id string, a platform.ActorContext) (domain.Task, error) {
	t, e := d.repo.Get(ctx, id)
	if e != nil {
		return t, e
	}
	if t.Workflow != w || t.WarehouseID != a.WarehouseID || (t.OperatorID != nil && *t.OperatorID != a.OperatorID) {
		return domain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Movement task access denied"}
	}
	return t, nil
}
func (s *PutawayService) List(c context.Context, a platform.ActorContext) ([]domain.Task, error) {
	return list(c, s.d, domain.Putaway, a)
}
func (s *PickingService) List(c context.Context, a platform.ActorContext) ([]domain.Task, error) {
	return list(c, s.d, domain.Picking, a)
}
func (s *ReplenishmentService) List(c context.Context, a platform.ActorContext) ([]domain.Task, error) {
	return list(c, s.d, domain.Replenishment, a)
}
func (s *PutawayService) Detail(c context.Context, id string, a platform.ActorContext) (domain.Task, error) {
	return detail(c, s.d, domain.Putaway, id, a)
}
func (s *PickingService) Detail(c context.Context, id string, a platform.ActorContext) (domain.Task, error) {
	return detail(c, s.d, domain.Picking, id, a)
}
func (s *ReplenishmentService) Detail(c context.Context, id string, a platform.ActorContext) (domain.Task, error) {
	return detail(c, s.d, domain.Replenishment, id, a)
}
func (s *PutawayService) Suggestions(c context.Context, id string, a platform.ActorContext) ([]domain.Location, error) {
	t, e := detail(c, s.d, domain.Putaway, id, a)
	if e != nil {
		return nil, e
	}
	return s.d.repo.Suggestions(c, t)
}
func (s *PutawayService) ValidateSource(c context.Context, x Command, v string) (domain.Task, error) {
	return mutate(c, s.d, domain.Putaway, x, "PutawayTaskStarted", func(t *domain.Task) error { return t.ValidateSource(x.Actor.OperatorID, v, x.BaseVersion, s.d.now()) }, 0)
}
func (s *PutawayService) ValidateDestination(c context.Context, x Command, v string) (domain.Task, error) {
	return mutate(c, s.d, domain.Putaway, x, "PutawayDestinationValidated", func(t *domain.Task) error {
		return t.ValidateDestination(x.Actor.OperatorID, v, x.BaseVersion, s.d.now())
	}, 0)
}
func (s *PutawayService) Confirm(c context.Context, x Command, q int64) (domain.Task, error) {
	return mutate(c, s.d, domain.Putaway, x, "InventoryMoved", func(t *domain.Task) error { return t.Move(x.Actor.OperatorID, q, x.BaseVersion, s.d.now()) }, q)
}
func (s *PickingService) ValidateLocation(c context.Context, x Command, v string) (domain.Task, error) {
	return mutate(c, s.d, domain.Picking, x, "PickingTaskStarted", func(t *domain.Task) error { return t.ValidateSource(x.Actor.OperatorID, v, x.BaseVersion, s.d.now()) }, 0)
}
func (s *PickingService) ResolveBarcode(c context.Context, x Command, v string) (domain.Task, error) {
	return mutate(c, s.d, domain.Picking, x, "PickingBarcodeResolved", func(t *domain.Task) error { return t.ValidateItem(x.Actor.OperatorID, v, x.BaseVersion, s.d.now()) }, 0)
}
func (s *PickingService) Confirm(c context.Context, x Command, q int64) (domain.Task, error) {
	return mutate(c, s.d, domain.Picking, x, "PickConfirmed", func(t *domain.Task) error {
		t.DestinationValidated = true
		return t.Move(x.Actor.OperatorID, q, x.BaseVersion, s.d.now())
	}, q)
}
func (s *PickingService) Complete(c context.Context, x Command) (domain.Task, error) {
	return mutate(c, s.d, domain.Picking, x, "PickingTaskCompleted", func(t *domain.Task) error {
		if t.Remaining() != 0 {
			return domain.ErrIncomplete
		}
		if err := t.GuardCompletion(x.Actor.OperatorID, x.BaseVersion); err != nil {
			return err
		}
		t.Status = domain.Completed
		t.Version++
		t.UpdatedAt = s.d.now().UTC()
		return nil
	}, 0)
}
func (s *ReplenishmentService) ValidateSource(c context.Context, x Command, v string) (domain.Task, error) {
	return mutate(c, s.d, domain.Replenishment, x, "ReplenishmentSourceValidated", func(t *domain.Task) error { return t.ValidateSource(x.Actor.OperatorID, v, x.BaseVersion, s.d.now()) }, 0)
}
func (s *ReplenishmentService) ValidateDestination(c context.Context, x Command, v string) (domain.Task, error) {
	return mutate(c, s.d, domain.Replenishment, x, "ReplenishmentDestinationValidated", func(t *domain.Task) error {
		return t.ValidateDestination(x.Actor.OperatorID, v, x.BaseVersion, s.d.now())
	}, 0)
}
func (s *ReplenishmentService) ValidateItem(c context.Context, x Command, v string) (domain.Task, error) {
	return mutate(c, s.d, domain.Replenishment, x, "ReplenishmentItemValidated", func(t *domain.Task) error { return t.ValidateItem(x.Actor.OperatorID, v, x.BaseVersion, s.d.now()) }, 0)
}
func (s *ReplenishmentService) Confirm(c context.Context, x Command, q int64) (domain.Task, error) {
	return mutate(c, s.d, domain.Replenishment, x, "InventoryMoved", func(t *domain.Task) error { return t.Move(x.Actor.OperatorID, q, x.BaseVersion, s.d.now()) }, q)
}
func mutate(ctx context.Context, d *dependencies, w domain.Workflow, c Command, eventType string, transition func(*domain.Task) error, quantity int64) (domain.Task, error) {
	var result domain.Task
	var envelope event.DomainEventEnvelope
	changed := false
	err := d.tx.WithinTransaction(ctx, func(tx context.Context) error {
		prev, found, e := d.commands.Find(tx, c.IdempotencyKey)
		if e != nil {
			return e
		}
		if found {
			if prev.Workflow != w || prev.WarehouseID != c.Actor.WarehouseID || prev.OperatorID != c.Actor.OperatorID {
				return &platform.DomainError{Code: "IDEMPOTENCY_KEY_REUSED", SafeMessage: "Idempotency key belongs to another command context"}
			}
			if e = json.Unmarshal(prev.Result, &result); e != nil {
				return e
			}
			if result.ID != c.TaskID {
				return &platform.DomainError{Code: "IDEMPOTENCY_KEY_REUSED", SafeMessage: "Idempotency key belongs to another task"}
			}
			return nil
		}
		t, e := d.repo.GetForUpdate(tx, c.TaskID)
		if e != nil {
			return e
		}
		if t.Workflow != w {
			return domain.ErrNotFound
		}
		if t.WarehouseID != c.Actor.WarehouseID {
			return &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Warehouse access denied"}
		}
		if e = transition(&t); e != nil {
			return e
		}
		if quantity > 0 {
			if e = d.repo.CheckMove(tx, t, quantity); e != nil {
				return e
			}
		}
		if e = d.repo.Save(tx, t); e != nil {
			return e
		}
		if quantity > 0 {
			if e = d.repo.ApplyMove(tx, t, quantity); e != nil {
				return e
			}
		}
		payload, _ := json.Marshal(map[string]any{"taskId": t.ID, "workflow": t.Workflow, "status": t.Status, "quantity": quantity})
		envelope = event.DomainEventEnvelope{EventID: uuid.New(), EventType: eventType, EventVersion: 1, AggregateType: string(w) + "Task", AggregateID: t.ID, AggregateVersion: t.Version, OccurredAt: d.now().UTC(), CorrelationID: c.Actor.CorrelationID, CausationID: c.CommandID, WarehouseID: c.Actor.WarehouseID, OperatorID: c.Actor.OperatorID, DeviceID: c.Actor.DeviceID, Topic: "pda.movement.events.v1", Payload: payload}
		if e = envelope.Validate(); e != nil {
			return fmt.Errorf("event envelope: %w", e)
		}
		if e = d.outbox.Append(tx, envelope); e != nil {
			return e
		}
		if e = d.audit.AppendMovementAudit(tx, eventType, t, c.Actor, payload); e != nil {
			return e
		}
		result = t
		encoded, _ := json.Marshal(t)
		if e = d.commands.Save(tx, c.IdempotencyKey, ports.CommandResult{CommandID: c.CommandID, Workflow: w, WarehouseID: c.Actor.WarehouseID, OperatorID: c.Actor.OperatorID, Result: encoded}); e != nil {
			return e
		}
		changed = true
		return nil
	})
	if err != nil {
		return domain.Task{}, err
	}
	if changed {
		// The transaction is committed above; cache invalidation is best effort.
		_ = d.invalidator.InvalidateMovementViews(ctx, c.Actor.WarehouseID, c.Actor.OperatorID)
		if e := d.publisher.Publish(ctx, envelope); e != nil {
			return result, &platform.DomainError{Code: "MESSAGING_PUBLISH_PENDING", SafeMessage: "Movement committed; event publication pending", Retryable: true}
		}
	}
	return result, nil
}
