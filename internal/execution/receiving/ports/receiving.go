package ports

import (
	"context"
	"encoding/json"

	"github.com/company/pda-backend/internal/execution/receiving/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
)

type Filter struct {
	WarehouseID, OperatorID, Status, Cursor string
	Limit                                   int
}
type Page struct {
	Items      []domain.Task `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}
type CommandStatus struct {
	CommandID   uuid.UUID       `json:"commandId"`
	CommandType string          `json:"commandType"`
	Status      string          `json:"status"`
	WarehouseID string          `json:"-"`
	OperatorID  string          `json:"-"`
	Result      json.RawMessage `json:"result"`
}
type Repository interface {
	List(context.Context, Filter) (Page, error)
	Get(context.Context, string) (domain.Task, error)
	GetForUpdate(context.Context, string) (domain.Task, error)
	Save(context.Context, domain.Task) error
	ResolveBarcode(context.Context, string, string) (domain.Line, error)
	ApplyInventory(context.Context, string, string, int64) error
}
type CommandRepository interface {
	FindByKey(context.Context, string) (CommandStatus, bool, error)
	FindByID(context.Context, uuid.UUID) (CommandStatus, bool, error)
	Save(context.Context, string, CommandStatus) error
}
type Outbox interface {
	Append(context.Context, event.DomainEventEnvelope) error
}
type Audit interface {
	AppendAudit(context.Context, string, string, domain.Task, platform.ActorContext, json.RawMessage) error
	AppendDenied(context.Context, string, string, platform.ActorContext, string) error
}
type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type Invalidator interface {
	InvalidateReceivingViews(context.Context, string, string) error
}
