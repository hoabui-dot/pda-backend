package ports

import (
	"context"
	"encoding/json"
	"github.com/company/pda-backend/internal/execution/movement/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
)

type Repository interface {
	List(context.Context, domain.Workflow, string, string) ([]domain.Task, error)
	Get(context.Context, string) (domain.Task, error)
	GetForUpdate(context.Context, string) (domain.Task, error)
	Save(context.Context, domain.Task) error
	Suggestions(context.Context, domain.Task) ([]domain.Location, error)
	CheckMove(context.Context, domain.Task, int64) error
	ApplyMove(context.Context, domain.Task, int64) error
}
type CommandResult struct {
	CommandID   uuid.UUID       `json:"commandId"`
	Workflow    domain.Workflow `json:"workflow"`
	WarehouseID string          `json:"-"`
	OperatorID  string          `json:"-"`
	Result      json.RawMessage `json:"result"`
}
type Commands interface {
	Find(context.Context, string) (CommandResult, bool, error)
	Save(context.Context, string, CommandResult) error
}
type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type Outbox interface {
	Append(context.Context, event.DomainEventEnvelope) error
}
type Audit interface {
	AppendMovementAudit(context.Context, string, domain.Task, platform.ActorContext, json.RawMessage) error
}
type Invalidator interface {
	InvalidateMovementViews(context.Context, string, string) error
}
