package application

import (
	"context"
	"encoding/json"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/company/pda-backend/internal/shipping/domain"
	"github.com/company/pda-backend/internal/shipping/ports"
	"github.com/google/uuid"
	"time"
)

type Service struct {
	repo        ports.Repository
	commands    ports.Commands
	tx          ports.Tx
	outbox      ports.Outbox
	audit       ports.Audit
	publisher   event.DomainEventPublisher
	invalidator ports.Invalidator
	now         func() time.Time
}

func New(r ports.Repository, c ports.Commands, t ports.Tx, o ports.Outbox, a ports.Audit, p event.DomainEventPublisher, i ports.Invalidator, n func() time.Time) *Service {
	return &Service{r, c, t, o, a, p, i, n}
}

type ConfirmCommand struct {
	ID                                 uuid.UUID
	Key, ShipmentID, Carrier, Tracking string
	BaseVersion                        int64
	Actor                              platform.ActorContext
}

func (s *Service) Summary(c context.Context, id string, a platform.ActorContext) (domain.Shipment, error) {
	x, e := s.repo.Get(c, id)
	if e != nil {
		return x, e
	}
	if x.WarehouseID != a.WarehouseID {
		return x, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Shipment access denied"}
	}
	return x, nil
}
func (s *Service) Readiness(c context.Context, id string, a platform.ActorContext) (domain.Readiness, error) {
	x, e := s.Summary(c, id, a)
	if e != nil {
		return domain.Readiness{}, e
	}
	return x.Readiness(), nil
}
func (s *Service) ProjectPickingComplete(c context.Context, id string) error {
	return s.repo.ProjectPickingComplete(c, id)
}
func (s *Service) CommandStatus(c context.Context, id uuid.UUID, a platform.ActorContext) (ports.CommandResult, error) {
	x, ok, e := s.commands.FindID(c, id)
	if e != nil {
		return x, e
	}
	if !ok {
		return x, domain.ErrNotFound
	}
	if x.WarehouseID != a.WarehouseID || x.OperatorID != a.OperatorID {
		return x, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Command access denied"}
	}
	return x, nil
}
func (s *Service) Confirm(c context.Context, x ConfirmCommand) (domain.Shipment, error) {
	var result domain.Shipment
	var events []event.DomainEventEnvelope
	changed := false
	e := s.tx.WithinTransaction(c, func(tx context.Context) error {
		old, ok, e := s.commands.FindKey(tx, x.Key)
		if e != nil {
			return e
		}
		if ok {
			if old.ShipmentID != x.ShipmentID || old.WarehouseID != x.Actor.WarehouseID || old.OperatorID != x.Actor.OperatorID {
				return &platform.DomainError{Code: "IDEMPOTENCY_KEY_REUSED", SafeMessage: "Idempotency context mismatch"}
			}
			return json.Unmarshal(old.Result, &result)
		}
		sh, e := s.repo.GetForUpdate(tx, x.ShipmentID)
		if e != nil {
			return e
		}
		if sh.WarehouseID != x.Actor.WarehouseID {
			return &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Shipment access denied"}
		}
		if e = sh.Confirm(x.Carrier, x.Tracking, x.BaseVersion, s.now()); e != nil {
			return e
		}
		if e = s.repo.Save(tx, sh); e != nil {
			return e
		}
		payload, _ := json.Marshal(sh)
		for _, typ := range []string{"ShipmentReadinessChanged", "ShipmentConfirmed", "OrderShipped"} {
			ev := event.DomainEventEnvelope{EventID: uuid.New(), EventType: typ, EventVersion: 1, AggregateType: "Shipment", AggregateID: sh.ID, AggregateVersion: sh.Version, OccurredAt: s.now().UTC(), CorrelationID: x.Actor.CorrelationID, CausationID: x.ID, WarehouseID: x.Actor.WarehouseID, OperatorID: x.Actor.OperatorID, DeviceID: x.Actor.DeviceID, Topic: "pda.shipping.events.v1", Payload: payload}
			if e = ev.Validate(); e != nil {
				return e
			}
			if e = s.outbox.Append(tx, ev); e != nil {
				return e
			}
			events = append(events, ev)
		}
		if e = s.audit.AppendShippingAudit(tx, "ShipmentConfirmed", sh, x.Actor, payload); e != nil {
			return e
		}
		if e = s.commands.Save(tx, x.Key, ports.CommandResult{ID: x.ID, ShipmentID: sh.ID, WarehouseID: x.Actor.WarehouseID, OperatorID: x.Actor.OperatorID, Result: payload}); e != nil {
			return e
		}
		result = sh
		changed = true
		return nil
	})
	if e != nil {
		return result, e
	}
	if changed {
		_ = s.invalidator.InvalidateShippingViews(c, x.Actor.WarehouseID, x.Actor.OperatorID)
		for _, ev := range events {
			if e = s.publisher.Publish(c, ev); e != nil {
				return result, &platform.DomainError{Code: "MESSAGING_PUBLISH_PENDING", SafeMessage: "Shipment committed; publication pending", Retryable: true}
			}
		}
	}
	return result, nil
}
