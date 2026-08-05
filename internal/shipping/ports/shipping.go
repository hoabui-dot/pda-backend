package ports

import (
	"context"
	"encoding/json"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/company/pda-backend/internal/shipping/domain"
	"github.com/google/uuid"
)

type Repository interface {
	Get(context.Context, string) (domain.Shipment, error)
	GetForUpdate(context.Context, string) (domain.Shipment, error)
	Save(context.Context, domain.Shipment) error
	VerifyPackage(context.Context, string, string) error
	ProjectPickingComplete(context.Context, string) error
}
type CommandResult struct {
	ID                                  uuid.UUID
	ShipmentID, WarehouseID, OperatorID string
	Result                              json.RawMessage
}
type Commands interface {
	FindKey(context.Context, string) (CommandResult, bool, error)
	FindID(context.Context, uuid.UUID) (CommandResult, bool, error)
	Save(context.Context, string, CommandResult) error
}
type Tx interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type Outbox interface {
	Append(context.Context, event.DomainEventEnvelope) error
}
type Audit interface {
	AppendShippingAudit(context.Context, string, domain.Shipment, platform.ActorContext, json.RawMessage) error
}
type Invalidator interface {
	InvalidateShippingViews(context.Context, string, string) error
}
