package application

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/company/pda-backend/internal/inventory/domain"
	"github.com/company/pda-backend/internal/inventory/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
	"time"
)

type Service struct {
	repo      ports.Repository
	commands  ports.Commands
	tx        ports.Tx
	outbox    ports.Outbox
	audit     ports.Audit
	publisher event.DomainEventPublisher
	cache     ports.Cache
	now       func() time.Time
}

func New(r ports.Repository, c ports.Commands, t ports.Tx, o ports.Outbox, a ports.Audit, p event.DomainEventPublisher, k ports.Cache, n func() time.Time) *Service {
	return &Service{r, c, t, o, a, p, k, n}
}

type Command struct {
	ID          uuid.UUID
	Key         string
	BaseVersion int64
	Actor       platform.ActorContext
}
type TransferCommand struct {
	Command
	Source, Destination, Item, LotID string
	Quantity                         int64
}

func (s *Service) Search(c context.Context, q string, a platform.ActorContext) ([]domain.Balance, error) {
	values, err := s.repo.Search(c, a.WarehouseID, q)
	return normalizeBalances(values), err
}
func (s *Service) Balances(c context.Context, item, loc string, a platform.ActorContext) ([]domain.Balance, error) {
	values, err := s.repo.Balances(c, a.WarehouseID, item, loc)
	return normalizeBalances(values), err
}
func (s *Service) Movements(c context.Context, item, cursor string, a platform.ActorContext) ([]domain.Movement, error) {
	return s.repo.Movements(c, a.WarehouseID, item, cursor)
}

func normalizeBalances(values []domain.Balance) []domain.Balance {
	for i := range values {
		values[i].ItemCode = values[i].ItemID
		values[i].LocationCode = values[i].LocationID
		values[i].OnHand = values[i].Quantity
		values[i].Available = values[i].Quantity - values[i].Reserved
		if values[i].UOM == "" {
			values[i].UOM = "EA"
		}
	}
	return values
}
func (s *Service) ValidateTransfer(c context.Context, x TransferCommand) error {
	return s.repo.ValidateLocations(c, x.Actor.WarehouseID, x.Source, x.Destination, x.Item, x.Quantity)
}
func (s *Service) Transfer(c context.Context, x TransferCommand) (domain.Transfer, error) {
	var result domain.Transfer
	var env event.DomainEventEnvelope
	changed := false
	e := s.tx.WithinTransaction(c, func(tx context.Context) error {
		old, found, e := s.commands.Find(tx, x.Key)
		if e != nil {
			return e
		}
		if found {
			if old.WarehouseID != x.Actor.WarehouseID || old.OperatorID != x.Actor.OperatorID {
				return &platform.DomainError{Code: "IDEMPOTENCY_KEY_REUSED", SafeMessage: "Idempotency key context mismatch"}
			}
			return json.Unmarshal(old.Result, &result)
		}
		if e = s.repo.ValidateLocations(tx, x.Actor.WarehouseID, x.Source, x.Destination, x.Item, x.Quantity); e != nil {
			return e
		}
		beforeSource, beforeDestination := s.balanceSnapshot(tx, x.Actor.WarehouseID, x.Item, x.Source), s.balanceSnapshot(tx, x.Actor.WarehouseID, x.Item, x.Destination)
		result = domain.Transfer{CommandID: x.ID.String(), TransferID: x.ID.String(), WarehouseID: x.Actor.WarehouseID, SourceLocation: x.Source, DestinationLocation: x.Destination, ItemID: x.Item, Quantity: x.Quantity, Status: "COMPLETED", BeforeSource: beforeSource, BeforeDestination: beforeDestination, AuditID: x.ID.String(), AsOf: s.now().UTC()}
		if e = s.repo.Transfer(tx, result); e != nil {
			return e
		}
		result.AfterSource = s.balanceSnapshot(tx, x.Actor.WarehouseID, x.Item, x.Source)
		result.AfterDestination = s.balanceSnapshot(tx, x.Actor.WarehouseID, x.Item, x.Destination)
		payload, _ := json.Marshal(result)
		env = makeEvent("StockTransferConfirmed", "StockTransfer", x.ID.String(), 1, x.Actor, x.ID, payload, s.now)
		if e = env.Validate(); e != nil {
			return e
		}
		if e = s.outbox.Append(tx, env); e != nil {
			return e
		}
		if e = s.audit.AppendInventoryAudit(tx, "StockTransferConfirmed", x.ID.String(), x.Actor, payload); e != nil {
			return e
		}
		if e = s.commands.Save(tx, x.Key, "TRANSFER", ports.CommandResult{CommandID: x.ID, WarehouseID: x.Actor.WarehouseID, OperatorID: x.Actor.OperatorID, Result: payload}); e != nil {
			return e
		}
		changed = true
		return nil
	})
	if e != nil {
		return result, e
	}
	if changed {
		_ = s.cache.InvalidateInventory(c, x.Actor.WarehouseID, x.Item, x.Source)
		_ = s.cache.InvalidateInventory(c, x.Actor.WarehouseID, x.Item, x.Destination)
		if e = s.publisher.Publish(c, env); e != nil {
			return result, &platform.DomainError{Code: "MESSAGING_PUBLISH_PENDING", SafeMessage: "Transfer committed; publication pending", Retryable: true}
		}
	}
	return result, nil
}

func (s *Service) balanceSnapshot(ctx context.Context, warehouse, item, location string) *domain.Balance {
	values, err := s.repo.Balances(ctx, warehouse, item, location)
	if err != nil || len(values) == 0 {
		return nil
	}
	values = normalizeBalances(values)
	return &values[0]
}

func (s *Service) CommandStatus(c context.Context, id uuid.UUID, a platform.ActorContext) (ports.CommandResult, error) {
	result, found, err := s.commands.Find(c, id.String())
	if err != nil {
		return ports.CommandResult{}, err
	}
	if !found {
		return ports.CommandResult{}, domain.ErrNotFound
	}
	if result.WarehouseID != a.WarehouseID || result.OperatorID != a.OperatorID {
		return ports.CommandResult{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Command access denied"}
	}
	return result, nil
}
func (s *Service) ListCounts(c context.Context, a platform.ActorContext) ([]domain.CountTask, error) {
	return s.repo.ListCounts(c, a.WarehouseID, a.OperatorID)
}
func (s *Service) CountDetail(c context.Context, id string, a platform.ActorContext) (domain.CountTask, error) {
	t, e := s.repo.GetCount(c, id)
	if e != nil {
		return t, e
	}
	if t.WarehouseID != a.WarehouseID || (t.OperatorID != nil && *t.OperatorID != a.OperatorID) {
		return domain.CountTask{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Count access denied"}
	}
	return t, nil
}
func (s *Service) ValidateCountLocation(c context.Context, id, location string, a platform.ActorContext) error {
	task, err := s.CountDetail(c, id, a)
	if err != nil {
		return err
	}
	if task.LocationID != location {
		return domain.ErrLocationInvalid
	}
	return nil
}
func (s *Service) ValidateCountItem(c context.Context, id, lineID, item string, a platform.ActorContext) error {
	task, err := s.CountDetail(c, id, a)
	if err != nil {
		return err
	}
	for _, line := range task.Lines {
		if line.ID == lineID && line.ItemID == item {
			return nil
		}
	}
	return domain.ErrItemNotInDocument
}
func (s *Service) SubmitCount(c context.Context, id, line string, q int64, x Command) (domain.CountTask, error) {
	return s.countMutation(c, id, x, "CycleCountSubmitted", func(t *domain.CountTask) error { return t.Submit(x.Actor.OperatorID, line, q, x.BaseVersion, s.now()) })
}
func (s *Service) Recount(c context.Context, id, line string, x Command) (domain.CountTask, error) {
	return s.countMutation(c, id, x, "CycleCountRecountRequested", func(t *domain.CountTask) error { return t.Recount(x.Actor.OperatorID, line, x.BaseVersion, s.now()) })
}
func (s *Service) CompleteCount(c context.Context, id string, x Command) (domain.CountTask, error) {
	return s.countMutation(c, id, x, "CycleCountCompleted", func(t *domain.CountTask) error { return t.Complete(x.Actor.OperatorID, x.BaseVersion, s.now()) })
}
func (s *Service) CountCommandStatus(c context.Context, id uuid.UUID, a platform.ActorContext) (ports.CommandResult, error) {
	result, found, err := s.commands.Find(c, id.String())
	if err != nil {
		return ports.CommandResult{}, err
	}
	if !found {
		return ports.CommandResult{}, domain.ErrNotFound
	}
	if result.WarehouseID != a.WarehouseID || result.OperatorID != a.OperatorID {
		return ports.CommandResult{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Command access denied"}
	}
	return result, nil
}
func (s *Service) countMutation(c context.Context, id string, x Command, eventType string, f func(*domain.CountTask) error) (domain.CountTask, error) {
	var result domain.CountTask
	var env event.DomainEventEnvelope
	changed := false
	e := s.tx.WithinTransaction(c, func(tx context.Context) error {
		old, found, e := s.commands.Find(tx, x.Key)
		if e != nil {
			return e
		}
		if found {
			return json.Unmarshal(old.Result, &result)
		}
		t, e := s.repo.GetCountForUpdate(tx, id)
		if e != nil {
			return e
		}
		if t.WarehouseID != x.Actor.WarehouseID {
			return &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Count access denied"}
		}
		if e = f(&t); e != nil {
			return e
		}
		if e = s.repo.SaveCount(tx, t); e != nil {
			return e
		}
		payload, _ := json.Marshal(t)
		env = makeEvent(eventType, "CycleCountTask", t.ID, t.Version, x.Actor, x.ID, payload, s.now)
		if e = env.Validate(); e != nil {
			return fmt.Errorf("event: %w", e)
		}
		if e = s.outbox.Append(tx, env); e != nil {
			return e
		}
		if e = s.audit.AppendInventoryAudit(tx, eventType, t.ID, x.Actor, payload); e != nil {
			return e
		}
		if e = s.commands.Save(tx, x.Key, "COUNT", ports.CommandResult{CommandID: x.ID, WarehouseID: x.Actor.WarehouseID, OperatorID: x.Actor.OperatorID, Result: payload}); e != nil {
			return e
		}
		result = t
		changed = true
		return nil
	})
	if e != nil {
		return result, e
	}
	if changed {
		_ = s.cache.InvalidateInventory(c, x.Actor.WarehouseID, "", result.LocationID)
		if e = s.publisher.Publish(c, env); e != nil {
			return result, e
		}
	}
	return result, nil
}
func makeEvent(typ, agg, id string, v int64, a platform.ActorContext, cause uuid.UUID, p json.RawMessage, n func() time.Time) event.DomainEventEnvelope {
	return event.DomainEventEnvelope{EventID: uuid.New(), EventType: typ, EventVersion: 1, AggregateType: agg, AggregateID: id, AggregateVersion: v, OccurredAt: n().UTC(), CorrelationID: a.CorrelationID, CausationID: cause, WarehouseID: a.WarehouseID, OperatorID: a.OperatorID, DeviceID: a.DeviceID, Topic: "pda.inventory.events.v1", Payload: p}
}
