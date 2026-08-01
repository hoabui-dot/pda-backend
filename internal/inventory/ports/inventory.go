package ports

import (
	"context"
	"encoding/json"
	"github.com/company/pda-backend/internal/inventory/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
	"time"
)

type Repository interface {
	Search(context.Context, string, string) ([]domain.Balance, error)
	Balances(context.Context, string, string, string) ([]domain.Balance, error)
	Movements(context.Context, string, string, string) ([]domain.Movement, error)
	ValidateLocations(context.Context, string, string, string, string, int64) error
	Transfer(context.Context, domain.Transfer) error
	ListCounts(context.Context, string, string) ([]domain.CountTask, error)
	GetCount(context.Context, string) (domain.CountTask, error)
	GetCountForUpdate(context.Context, string) (domain.CountTask, error)
	SaveCount(context.Context, domain.CountTask) error
}
type CommandResult struct {
	CommandID               uuid.UUID
	WarehouseID, OperatorID string
	Result                  json.RawMessage
}
type Commands interface {
	Find(context.Context, string) (CommandResult, bool, error)
	Save(context.Context, string, string, CommandResult) error
}
type Tx interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type Outbox interface {
	Append(context.Context, event.DomainEventEnvelope) error
}
type Audit interface {
	AppendInventoryAudit(context.Context, string, string, platform.ActorContext, json.RawMessage) error
}
type Cache interface {
	InvalidateInventory(context.Context, string, string, string) error
}
type Clock func() time.Time
