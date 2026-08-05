package postgres

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	identity "github.com/company/pda-backend/internal/identity/domain"
	"github.com/company/pda-backend/internal/identity/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ Pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{Pool: pool} }

func (s *Store) ByUsername(ctx context.Context, username string) (identity.Operator, bool, error) {
	return s.operator(ctx, `WHERE lower(username)=lower($1)`, strings.TrimSpace(username))
}
func (s *Store) ByID(ctx context.Context, id string) (identity.Operator, bool, error) {
	return s.operator(ctx, `WHERE id=$1`, id)
}
func (s *Store) CheckLogin(ctx context.Context, username string, now time.Time) error {
	var status string
	var lockedUntil *time.Time
	err := s.Pool.QueryRow(ctx, `SELECT status,locked_until FROM identity_operators WHERE lower(username)=lower($1)`, strings.TrimSpace(username)).Scan(&status, &lockedUntil)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if status == "DISABLED" || status == "DELETED" {
		return ports.ErrLoginDisabled
	}
	if status == "LOCKED" || lockedUntil != nil && lockedUntil.After(now) {
		return ports.ErrLoginLocked
	}
	return nil
}
func (s *Store) RecordLoginFailure(ctx context.Context, username string, now time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE identity_operators SET failed_login_count=failed_login_count+1,locked_until=CASE WHEN failed_login_count+1 >= 5 THEN $2 ELSE locked_until END,status=CASE WHEN failed_login_count+1 >= 5 THEN 'LOCKED' ELSE status END,updated_at=$2 WHERE lower(username)=lower($1)`, strings.TrimSpace(username), now.Add(15*time.Minute))
	return err
}
func (s *Store) RecordLoginSuccess(ctx context.Context, username string, now time.Time) error {
	_, err := s.Pool.Exec(ctx, `UPDATE identity_operators SET failed_login_count=0,locked_until=NULL,status='ACTIVE',last_login_at=$2,updated_at=$2 WHERE lower(username)=lower($1)`, strings.TrimSpace(username), now)
	return err
}
func (s *Store) operator(ctx context.Context, where string, arg string) (identity.Operator, bool, error) {
	var o identity.Operator
	var status string
	err := s.Pool.QueryRow(ctx, `SELECT id,employee_code,username,display_name,password_hash,status,shift_code FROM identity_operators `+where, arg).Scan(&o.ID, &o.EmployeeCode, &o.Username, &o.DisplayName, &o.PasswordHash, &status, &o.ShiftCode)
	if err == pgx.ErrNoRows {
		return identity.Operator{}, false, nil
	}
	if err != nil {
		return identity.Operator{}, false, err
	}
	o.Active = status == "ACTIVE"
	rows, err := s.Pool.Query(ctx, `SELECT w.warehouse_id FROM identity_operator_warehouses w WHERE w.operator_id=$1 AND w.active=true ORDER BY w.warehouse_id`, o.ID)
	if err != nil {
		return identity.Operator{}, false, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return identity.Operator{}, false, err
		}
		o.WarehouseIDs = append(o.WarehouseIDs, id)
	}
	rows.Close()
	rows, err = s.Pool.Query(ctx, `SELECT r.role_code,p.permission_code FROM identity_operator_roles x JOIN identity_roles r ON r.role_id=x.role_id AND r.active=true LEFT JOIN identity_role_permissions rp ON rp.role_id=r.role_id LEFT JOIN identity_permissions p ON p.permission_id=rp.permission_id WHERE x.operator_id=$1 ORDER BY r.role_code,p.permission_code`, o.ID)
	if err != nil {
		return identity.Operator{}, false, err
	}
	seenRoles := map[string]bool{}
	seenPermissions := map[string]bool{}
	for rows.Next() {
		var role, permission string
		if err := rows.Scan(&role, &permission); err != nil {
			rows.Close()
			return identity.Operator{}, false, err
		}
		if role != "" && !seenRoles[role] {
			o.Roles = append(o.Roles, role)
			seenRoles[role] = true
		}
		if permission != "" && !seenPermissions[permission] {
			o.Permissions = append(o.Permissions, permission)
			seenPermissions[permission] = true
		}
	}
	rows.Close()
	return o, true, rows.Err()
}
func (s *Store) Warehouses(ctx context.Context, ids []string) ([]identity.Warehouse, error) {
	rows, err := s.Pool.Query(ctx, `SELECT warehouse_id,warehouse_name FROM identity_warehouses WHERE warehouse_id=ANY($1) ORDER BY warehouse_id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []identity.Warehouse{}
	for rows.Next() {
		var w identity.Warehouse
		if err := rows.Scan(&w.ID, &w.Name); err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}
func (s *Store) Register(ctx context.Context, registration identity.DeviceRegistration) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO identity_devices(device_code,approved_at) VALUES($1,now()) ON CONFLICT(device_code) DO UPDATE SET updated_at=now(),status='ACTIVE',revoked_at=NULL`, registration.DeviceID)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO identity_operator_devices(operator_id,device_code,warehouse_id) VALUES($1,$2,$3) ON CONFLICT(operator_id,device_code,warehouse_id) DO UPDATE SET active=true`, registration.OperatorID, registration.DeviceID, registration.WarehouseID)
	return err
}
func (s *Store) IsRegistered(ctx context.Context, registration identity.DeviceRegistration) (bool, error) {
	var ok bool
	err := s.Pool.QueryRow(ctx, `SELECT d.status='ACTIVE' AND x.active FROM identity_devices d JOIN identity_operator_devices x ON x.device_code=d.device_code WHERE x.operator_id=$1 AND x.device_code=$2 AND x.warehouse_id=$3`, registration.OperatorID, registration.DeviceID, registration.WarehouseID).Scan(&ok)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return ok, err
}
func (s *Store) Append(ctx context.Context, record ports.AuditRecord) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO identity_security_audit(event_type,operator_id,device_code,warehouse_id,correlation_id,outcome,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, record.Action, nullString(record.OperatorID), nullString(record.DeviceID), nullString(record.WarehouseID), nullString(record.CorrelationID), record.Outcome, record.OccurredAt)
	return err
}
func nullString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

var _ ports.OperatorRepository = (*Store)(nil)
var _ ports.DeviceRepository = (*Store)(nil)
var _ ports.AuditRepository = (*Store)(nil)
var _ ports.LoginProtector = (*Store)(nil)

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func auditJSON(value any) []byte { data, _ := json.Marshal(value); return data }

var _ = time.Time{}
