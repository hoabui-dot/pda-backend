package ports

import (
	"context"
	"time"

	"github.com/company/pda-backend/internal/execution/domain"
	"github.com/company/pda-backend/internal/platform/event"
)

type TaskFilter struct {
	WarehouseID, OperatorID, Category, Status, Query, Cursor string
	PriorityMin, PriorityMax                                 int
	Zone, DateFrom, DateTo, Sort, Direction                  string
	Limit                                                    int
}
type TaskPage struct {
	Tasks      []domain.Task `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}
type SummaryItem struct {
	Category domain.TaskCategory `json:"category"`
	Status   domain.TaskStatus   `json:"status"`
	Count    int                 `json:"count"`
}
type Dashboard struct {
	InboundCount      int       `json:"inboundCount"`
	PutawayCount      int       `json:"putawayCount"`
	PickingCount      int       `json:"pickingCount"`
	ShippingCount     int       `json:"shippingCount"`
	InProgressCount   int       `json:"inProgressCount"`
	CompletedCount    int       `json:"completedCount"`
	CompletionPercent float64   `json:"completionPercent"`
	PendingSyncCount  *int      `json:"pendingSyncCount"`
	ActionableAlerts  int       `json:"actionableAlertCount"`
	AsOf              time.Time `json:"asOf"`
	Total             int       `json:"total,omitempty"`
	Assigned          int       `json:"assigned,omitempty"`
	InProgress        int       `json:"inProgress,omitempty"`
	Completed         int       `json:"completed,omitempty"`
	HighPriority      int       `json:"highPriority,omitempty"`
	UpdatedAt         time.Time `json:"updatedAt,omitempty"`
}
type IdempotencyResult struct {
	Task  domain.Task
	Event event.DomainEventEnvelope
}

type TaskRepository interface {
	GetForUpdate(context.Context, string) (domain.Task, error)
	Save(context.Context, domain.Task) error
	List(context.Context, TaskFilter) (TaskPage, error)
	Summary(context.Context, string, string, string) ([]SummaryItem, error)
	Dashboard(context.Context, string, string) (Dashboard, error)
}
type IdempotencyRepository interface {
	Find(context.Context, string) (IdempotencyResult, bool, error)
	Save(context.Context, string, IdempotencyResult) error
}
type OutboxRepository interface {
	Append(context.Context, event.DomainEventEnvelope) error
}
type TransactionManager interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type ProjectionInvalidator interface {
	InvalidateTaskViews(context.Context, string, string) error
}
type UpstreamTaskSource interface {
	Tasks(context.Context) ([]domain.Task, error)
}
