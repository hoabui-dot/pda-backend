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
	ErrAccessTokenExpired  = &platform.DomainError{Code: "ACCESS_TOKEN_EXPIRED", SafeMessage: "Access token has expired."}
	ErrAccessTokenInvalid  = &platform.DomainError{Code: "ACCESS_TOKEN_INVALID", SafeMessage: "Access token is invalid."}
	ErrRefreshTokenInvalid = &platform.DomainError{Code: "REFRESH_TOKEN_INVALID", SafeMessage: "Refresh token is invalid."}
	ErrRefreshTokenExpired = &platform.DomainError{Code: "REFRESH_TOKEN_EXPIRED", SafeMessage: "Refresh token has expired."}
	ErrRefreshTokenRevoked = &platform.DomainError{Code: "REFRESH_TOKEN_REVOKED", SafeMessage: "Refresh token has been revoked."}
	ErrRefreshTokenReused  = &platform.DomainError{Code: "REFRESH_TOKEN_REUSED", SafeMessage: "Refresh token has already been used."}
	ErrSessionRevoked      = &platform.DomainError{Code: "SESSION_REVOKED", SafeMessage: "Session has been revoked."}
	ErrUserDisabled        = &platform.DomainError{Code: "USER_DISABLED", SafeMessage: "User account is disabled."}
	ErrWarehouseDenied     = &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Warehouse access denied"}
	ErrDeviceNotRegistered = &platform.DomainError{Code: "DEVICE_NOT_REGISTERED", SafeMessage: "Device is not registered"}
)

type Session struct {
	AccessToken           string
	RefreshToken          string
	ExpiresAt             time.Time
	RefreshTokenExpiresAt time.Time
	DeviceID              string
	WarehouseID           string
}

type Service struct {
	operators ports.OperatorRepository
	tokens    ports.TokenProvider
	devices   ports.DeviceRepository
	audit     ports.AuditRepository
	passwords ports.PasswordVerifier
	sessions  ports.SessionManager
	protector ports.LoginProtector
	now       func() time.Time
}

func NewService(operators ports.OperatorRepository, tokens ports.TokenProvider, devices ports.DeviceRepository, audit ports.AuditRepository, now func() time.Time) *Service {
	return &Service{operators: operators, tokens: tokens, devices: devices, audit: audit, now: now}
}

func NewProductionService(operators ports.OperatorRepository, tokens ports.TokenProvider, devices ports.DeviceRepository, audit ports.AuditRepository, passwords ports.PasswordVerifier, sessions ports.SessionManager, now func() time.Time) *Service {
	var protector ports.LoginProtector
	if candidate, ok := operators.(ports.LoginProtector); ok {
		protector = candidate
	}
	return &Service{operators: operators, tokens: tokens, devices: devices, audit: audit, passwords: passwords, sessions: sessions, protector: protector, now: now}
}

func (s *Service) Login(ctx context.Context, username, password, correlationID string) (string, identity.Operator, error) {
	operator, err := s.authenticateCredentials(ctx, username, password, correlationID)
	if err != nil {
		return "", identity.Operator{}, err
	}
	if s.sessions != nil {
		return "", operator, nil
	}
	token, err := s.tokens.Issue(operator, s.now())
	if err != nil {
		return "", identity.Operator{}, err
	}
	return token, operator, nil
}

func (s *Service) authenticateCredentials(ctx context.Context, username, password, correlationID string) (identity.Operator, error) {
	operator, found, err := s.operators.ByUsername(ctx, username)
	if err != nil {
		return identity.Operator{}, err
	}
	if s.protector != nil {
		if err := s.protector.CheckLogin(ctx, username, s.now()); err != nil {
			if errors.Is(err, ports.ErrLoginLocked) {
				return identity.Operator{}, &platform.DomainError{Code: "AUTH_ACCOUNT_LOCKED", SafeMessage: "Account is temporarily locked", Retryable: true}
			}
			if errors.Is(err, ports.ErrLoginDisabled) {
				return identity.Operator{}, &platform.DomainError{Code: "AUTH_ACCOUNT_DISABLED", SafeMessage: "Account is disabled"}
			}
			return identity.Operator{}, err
		}
	}
	validPassword := found && operator.Active
	if validPassword && s.passwords != nil {
		validPassword = s.passwords.Verify(operator.PasswordHash, password) == nil
	} else if validPassword {
		validPassword = operator.Password == password
	}
	if !validPassword {
		if s.protector != nil && found {
			_ = s.protector.RecordLoginFailure(ctx, operator.Username, s.now())
		}
		if auditErr := s.audit.Append(ctx, ports.AuditRecord{Action: "LOGIN", Outcome: "DENIED", CorrelationID: correlationID, OccurredAt: s.now()}); auditErr != nil {
			return identity.Operator{}, auditErr
		}
		return identity.Operator{}, ErrInvalidCredentials
	}
	if s.protector != nil {
		if err := s.protector.RecordLoginSuccess(ctx, operator.Username, s.now()); err != nil {
			return identity.Operator{}, err
		}
	}
	if err := s.audit.Append(ctx, ports.AuditRecord{Action: "LOGIN", Outcome: "SUCCESS", OperatorID: operator.ID, CorrelationID: correlationID, OccurredAt: s.now()}); err != nil {
		return identity.Operator{}, err
	}
	return operator, nil
}

func (s *Service) LoginSession(ctx context.Context, username, password, correlationID string) (Session, identity.Operator, error) {
	return s.LoginSessionContext(ctx, username, password, "", "", correlationID)
}

func (s *Service) LoginSessionContext(ctx context.Context, username, password, deviceID, warehouseID, correlationID string) (Session, identity.Operator, error) {
	operator, err := s.authenticateCredentials(ctx, username, password, correlationID)
	if err != nil {
		return Session{}, identity.Operator{}, err
	}
	if s.sessions != nil {
		if deviceID == "" {
			return Session{}, identity.Operator{}, ErrDeviceNotRegistered
		}
		if warehouseID == "" {
			return Session{}, identity.Operator{}, ErrWarehouseDenied
		}
		if err := s.ValidateContext(ctx, operator, deviceID, warehouseID); err != nil {
			return Session{}, identity.Operator{}, err
		}
		return s.createProductionSession(ctx, operator, deviceID, warehouseID, correlationID)
	}
	accessToken, err := s.tokens.Issue(operator, s.now())
	if err != nil {
		return Session{}, identity.Operator{}, err
	}
	session := Session{AccessToken: accessToken, ExpiresAt: s.now().Add(time.Hour)}
	if provider, ok := s.tokens.(ports.SessionTokenProvider); ok {
		session.RefreshToken, err = provider.IssueRefresh(operator, s.now())
		if err != nil {
			return Session{}, identity.Operator{}, err
		}
	}
	return session, operator, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (identity.Operator, error) {
	if s.sessions != nil {
		claims, err := s.sessions.Authenticate(ctx, token, s.now())
		if err != nil {
			return identity.Operator{}, mapSessionError(err, false)
		}
		operator, found, err := s.operators.ByID(ctx, claims.OperatorID)
		if err != nil || !found || !operator.Active {
			return identity.Operator{}, ErrUserDisabled
		}
		return operator, nil
	}
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
	if s.sessions != nil {
		return "", ErrRefreshTokenInvalid
	}
	operator, err := s.Authenticate(ctx, token)
	if err != nil {
		return "", err
	}
	if err := s.tokens.Revoke(token); err != nil {
		return "", ErrUnauthorized
	}
	return s.tokens.Issue(operator, s.now())
}

func (s *Service) RefreshSession(ctx context.Context, refreshToken string) (Session, identity.Operator, error) {
	return s.RefreshSessionContext(ctx, refreshToken, "")
}

func (s *Service) RefreshSessionContext(ctx context.Context, refreshToken, deviceID string) (Session, identity.Operator, error) {
	if s.sessions != nil {
		access, refresh, expires, refreshExpires, operatorID, sessionDevice, warehouseID, err := s.sessions.Refresh(ctx, refreshToken, deviceID, s.now())
		if err != nil {
			return Session{}, identity.Operator{}, mapSessionError(err, true)
		}
		operator, found, err := s.operators.ByID(ctx, operatorID)
		if err != nil || !found || !operator.Active {
			return Session{}, identity.Operator{}, ErrUserDisabled
		}
		return Session{AccessToken: access, RefreshToken: refresh, ExpiresAt: expires, RefreshTokenExpiresAt: refreshExpires, DeviceID: sessionDevice, WarehouseID: warehouseID}, operator, nil
	}
	provider, ok := s.tokens.(ports.SessionTokenProvider)
	if !ok {
		return Session{}, identity.Operator{}, ErrUnauthorized
	}
	claims, err := provider.ValidateRefresh(refreshToken, s.now())
	if err != nil {
		return Session{}, identity.Operator{}, ErrUnauthorized
	}
	operator, found, err := s.operators.ByID(ctx, claims.OperatorID)
	if err != nil || !found {
		return Session{}, identity.Operator{}, ErrUnauthorized
	}
	if err := s.tokens.Revoke(refreshToken); err != nil {
		return Session{}, identity.Operator{}, ErrUnauthorized
	}
	accessToken, err := s.tokens.Issue(operator, s.now())
	if err != nil {
		return Session{}, identity.Operator{}, err
	}
	newRefresh, err := provider.IssueRefresh(operator, s.now())
	if err != nil {
		return Session{}, identity.Operator{}, err
	}
	return Session{AccessToken: accessToken, RefreshToken: newRefresh, ExpiresAt: s.now().Add(time.Hour)}, operator, nil
}

func (s *Service) Logout(token string) error {
	return s.LogoutContext(context.Background(), token, "")
}

func (s *Service) LogoutContext(ctx context.Context, accessToken, refreshToken string) error {
	if s.sessions != nil {
		if refreshToken != "" {
			if err := s.sessions.RevokeRefresh(ctx, refreshToken, s.now()); err != nil {
				return mapSessionError(err, true)
			}
			return nil
		}
		if err := s.sessions.Logout(ctx, accessToken, s.now()); err != nil {
			return mapSessionError(err, false)
		}
		return nil
	}
	if err := s.tokens.Revoke(accessToken); err != nil {
		return ErrUnauthorized
	}
	return nil
}

func (s *Service) createProductionSession(ctx context.Context, operator identity.Operator, deviceID, warehouseID, correlationID string) (Session, identity.Operator, error) {
	access, refresh, expires, refreshExpires, err := s.sessions.Create(ctx, operator, deviceID, warehouseID, s.now())
	if err != nil {
		return Session{}, identity.Operator{}, err
	}
	return Session{AccessToken: access, RefreshToken: refresh, ExpiresAt: expires, RefreshTokenExpiresAt: refreshExpires, DeviceID: deviceID, WarehouseID: warehouseID}, operator, nil
}

func mapSessionError(err error, refresh bool) error {
	switch {
	case errors.Is(err, ports.ErrAccessTokenExpired):
		return ErrAccessTokenExpired
	case errors.Is(err, ports.ErrAccessTokenInvalid):
		return ErrAccessTokenInvalid
	case errors.Is(err, ports.ErrRefreshTokenExpired):
		return ErrRefreshTokenExpired
	case errors.Is(err, ports.ErrRefreshTokenRevoked):
		return ErrRefreshTokenRevoked
	case errors.Is(err, ports.ErrRefreshTokenReused):
		return ErrRefreshTokenReused
	case errors.Is(err, ports.ErrRefreshTokenInvalid):
		return ErrRefreshTokenInvalid
	case errors.Is(err, ports.ErrSessionRevoked):
		return ErrSessionRevoked
	case errors.Is(err, ports.ErrUserDisabled):
		return ErrUserDisabled
	case errors.Is(err, ports.ErrDeviceMismatch):
		return &platform.DomainError{Code: "SESSION_DEVICE_MISMATCH", SafeMessage: "Session device does not match."}
	default:
		if refresh {
			return ErrRefreshTokenInvalid
		}
		return ErrUnauthorized
	}
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

func (s *Service) DeviceStatus(ctx context.Context, operator identity.Operator, deviceID, warehouseID string) (string, error) {
	if deviceID == "" {
		return "UNREGISTERED", nil
	}
	registered, err := s.devices.IsRegistered(ctx, identity.DeviceRegistration{DeviceID: deviceID, OperatorID: operator.ID, WarehouseID: warehouseID})
	if err != nil {
		return "", err
	}
	if registered {
		return "REGISTERED", nil
	}
	return "UNREGISTERED", nil
}

func IsDomainError(err error, target *platform.DomainError) bool { return errors.Is(err, target) }
