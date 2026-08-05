package ports

import (
	"context"
	"errors"
	"time"

	identity "github.com/company/pda-backend/internal/identity/domain"
)

var ErrLoginLocked = errors.New("identity account locked")
var ErrLoginDisabled = errors.New("identity account disabled")

type Claims struct {
	OperatorID, TokenID, SessionID, DeviceID, WarehouseID string
	ExpiresAt                                             time.Time
}

type PasswordVerifier interface {
	Verify(hash, password string) error
}

type LoginProtector interface {
	CheckLogin(context.Context, string, time.Time) error
	RecordLoginFailure(context.Context, string, time.Time) error
	RecordLoginSuccess(context.Context, string, time.Time) error
}

type SessionManager interface {
	Create(context.Context, identity.Operator, string, string, time.Time) (accessToken, refreshToken string, expiresAt time.Time, err error)
	Authenticate(context.Context, string, time.Time) (Claims, error)
	Refresh(context.Context, string, string, time.Time) (accessToken, refreshToken string, expiresAt time.Time, operatorID, deviceID, warehouseID string, err error)
	Logout(context.Context, string, time.Time) error
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

type SessionTokenProvider interface {
	IssueRefresh(identity.Operator, time.Time) (string, error)
	ValidateRefresh(string, time.Time) (Claims, error)
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
