package application

import (
	"context"
	"errors"
	"time"

	identity "github.com/company/pda-backend/internal/identity/domain"
	"github.com/company/pda-backend/internal/identity/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
)

var (
	ErrInvalidCredentials  = &platform.DomainError{Code: "AUTH_INVALID_CREDENTIALS", SafeMessage: "Invalid credentials"}
	ErrUnauthorized        = &platform.DomainError{Code: "AUTH_SESSION_EXPIRED", SafeMessage: "Authentication is required"}
	ErrWarehouseDenied     = &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Warehouse access denied"}
	ErrDeviceNotRegistered = &platform.DomainError{Code: "DEVICE_NOT_REGISTERED", SafeMessage: "Device is not registered"}
)

type Service struct {
	operators ports.OperatorRepository
	tokens    ports.TokenProvider
	devices   ports.DeviceRepository
	audit     ports.AuditRepository
	now       func() time.Time
}

func NewService(operators ports.OperatorRepository, tokens ports.TokenProvider, devices ports.DeviceRepository, audit ports.AuditRepository, now func() time.Time) *Service {
	return &Service{operators: operators, tokens: tokens, devices: devices, audit: audit, now: now}
}

func (s *Service) Login(ctx context.Context, username, password, correlationID string) (string, identity.Operator, error) {
	operator, found, err := s.operators.ByUsername(ctx, username)
	if err != nil {
		return "", identity.Operator{}, err
	}
	if !found || operator.Password != password {
		if auditErr := s.audit.Append(ctx, ports.AuditRecord{Action: "LOGIN", Outcome: "DENIED", CorrelationID: correlationID, OccurredAt: s.now()}); auditErr != nil {
			return "", identity.Operator{}, auditErr
		}
		return "", identity.Operator{}, ErrInvalidCredentials
	}
	token, err := s.tokens.Issue(operator, s.now())
	if err != nil {
		return "", identity.Operator{}, err
	}
	if err := s.audit.Append(ctx, ports.AuditRecord{Action: "LOGIN", Outcome: "SUCCESS", OperatorID: operator.ID, CorrelationID: correlationID, OccurredAt: s.now()}); err != nil {
		return "", identity.Operator{}, err
	}
	return token, operator, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (identity.Operator, error) {
	claims, err := s.tokens.Validate(token, s.now())
	if err != nil {
		return identity.Operator{}, ErrUnauthorized
	}
	operator, found, err := s.operators.ByID(ctx, claims.OperatorID)
	if err != nil {
		return identity.Operator{}, err
	}
	if !found {
		return identity.Operator{}, ErrUnauthorized
	}
	return operator, nil
}

func (s *Service) Refresh(ctx context.Context, token string) (string, error) {
	operator, err := s.Authenticate(ctx, token)
	if err != nil {
		return "", err
	}
	if err := s.tokens.Revoke(token); err != nil {
		return "", ErrUnauthorized
	}
	return s.tokens.Issue(operator, s.now())
}

func (s *Service) Logout(token string) error {
	if err := s.tokens.Revoke(token); err != nil {
		return ErrUnauthorized
	}
	return nil
}

func (s *Service) Warehouses(ctx context.Context, operator identity.Operator) ([]identity.Warehouse, error) {
	return s.operators.Warehouses(ctx, operator.WarehouseIDs)
}

func (s *Service) RegisterDevice(ctx context.Context, operator identity.Operator, deviceID, warehouseID, correlationID string) (identity.DeviceRegistration, error) {
	if !operator.CanAccessWarehouse(warehouseID) {
		return identity.DeviceRegistration{}, ErrWarehouseDenied
	}
	registration := identity.DeviceRegistration{DeviceID: deviceID, OperatorID: operator.ID, WarehouseID: warehouseID}
	if err := s.devices.Register(ctx, registration); err != nil {
		return identity.DeviceRegistration{}, err
	}
	if err := s.audit.Append(ctx, ports.AuditRecord{Action: "DEVICE_REGISTER", Outcome: "SUCCESS", OperatorID: operator.ID, DeviceID: deviceID, WarehouseID: warehouseID, CorrelationID: correlationID, OccurredAt: s.now()}); err != nil {
		return identity.DeviceRegistration{}, err
	}
	return registration, nil
}

func (s *Service) ValidateContext(ctx context.Context, operator identity.Operator, deviceID, warehouseID string) error {
	if !operator.CanAccessWarehouse(warehouseID) {
		return ErrWarehouseDenied
	}
	registered, err := s.devices.IsRegistered(ctx, identity.DeviceRegistration{DeviceID: deviceID, OperatorID: operator.ID, WarehouseID: warehouseID})
	if err != nil {
		return err
	}
	if !registered {
		return ErrDeviceNotRegistered
	}
	return nil
}

func IsDomainError(err error, target *platform.DomainError) bool { return errors.Is(err, target) }
