package ports

import (
	"context"
	"time"

	identity "github.com/company/pda-backend/internal/identity/domain"
)

type Claims struct {
	OperatorID, TokenID string
	ExpiresAt           time.Time
}

type OperatorRepository interface {
	ByUsername(context.Context, string) (identity.Operator, bool, error)
	ByID(context.Context, string) (identity.Operator, bool, error)
	Warehouses(context.Context, []string) ([]identity.Warehouse, error)
}

type TokenProvider interface {
	Issue(identity.Operator, time.Time) (string, error)
	Validate(string, time.Time) (Claims, error)
	Revoke(string) error
}

type DeviceRepository interface {
	Register(context.Context, identity.DeviceRegistration) error
	IsRegistered(context.Context, identity.DeviceRegistration) (bool, error)
}

type AuditRecord struct {
	Action, Outcome, OperatorID, DeviceID, WarehouseID, CorrelationID string
	OccurredAt                                                        time.Time
}
type AuditRepository interface {
	Append(context.Context, AuditRecord) error
}

type Authorizer interface {
	Authorize(identity.Operator, string, string) error
}
