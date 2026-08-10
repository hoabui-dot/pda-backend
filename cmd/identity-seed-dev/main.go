package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	identitysecurity "github.com/company/pda-backend/internal/identity/adapters/security"
	"github.com/company/pda-backend/internal/platform/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	username := required("PDA_SEED_USERNAME")
	password := required("PDA_SEED_PASSWORD")
	operatorID := value("PDA_SEED_OPERATOR_ID", "OP-DEV-01")
	employeeCode := value("PDA_SEED_EMPLOYEE_CODE", "EMP-DEV-01")
	displayName := value("PDA_SEED_DISPLAY_NAME", "Development Operator")
	warehouseID := value("PDA_SEED_WAREHOUSE_ID", "WH-01")
	warehouseName := value("PDA_SEED_WAREHOUSE_NAME", "Development Warehouse")
	deviceID := os.Getenv("PDA_SEED_DEVICE_ID")
	hash, err := identitysecurity.DefaultArgon2id().Hash(password)
	if err != nil {
		panic(err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		panic(err)
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO identity_operators(id,username,employee_code,display_name,password_hash,status,shift_code) VALUES($1,$2,$3,$4,$5,'ACTIVE','DAY') ON CONFLICT(username) DO UPDATE SET employee_code=EXCLUDED.employee_code,display_name=EXCLUDED.display_name,password_hash=EXCLUDED.password_hash,status='ACTIVE',failed_login_count=0,locked_until=NULL,updated_at=now()`, operatorID, strings.ToLower(strings.TrimSpace(username)), employeeCode, displayName, hash)
	if err != nil {
		panic(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_warehouses(warehouse_id,warehouse_name) VALUES($1,$2) ON CONFLICT(warehouse_id) DO UPDATE SET warehouse_name=EXCLUDED.warehouse_name`, warehouseID, warehouseName)
	if err != nil {
		panic(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_operator_warehouses(operator_id,warehouse_id,is_default) VALUES($1,$2,true) ON CONFLICT(operator_id,warehouse_id) DO UPDATE SET active=true`, operatorID, warehouseID)
	if err != nil {
		panic(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_roles(role_code,role_name) VALUES('RECEIVING_OPERATOR','Receiving Operator') ON CONFLICT(role_code) DO NOTHING`)
	if err != nil {
		panic(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_permissions(permission_code,permission_name) VALUES('RECEIVE','Receive warehouse goods'),('DASHBOARD_READ','Read dashboard'),('TASK_READ','Read tasks') ON CONFLICT(permission_code) DO NOTHING`)
	if err != nil {
		panic(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_operator_roles(operator_id,role_id) SELECT $1,role_id FROM identity_roles WHERE role_code='RECEIVING_OPERATOR' ON CONFLICT DO NOTHING`, operatorID)
	if err != nil {
		panic(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO identity_role_permissions(role_id,permission_id) SELECT r.role_id,p.permission_id FROM identity_roles r CROSS JOIN identity_permissions p WHERE r.role_code='RECEIVING_OPERATOR' AND p.permission_code IN ('RECEIVE','DASHBOARD_READ','TASK_READ') ON CONFLICT DO NOTHING`)
	if err != nil {
		panic(err)
	}
	if deviceID != "" {
		_, err = tx.Exec(ctx, `INSERT INTO identity_devices(device_code,device_model,status,approved_at) VALUES($1,'TC26','ACTIVE',now()) ON CONFLICT(device_code) DO UPDATE SET status='ACTIVE',updated_at=now()`, deviceID)
		if err != nil {
			panic(err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO identity_operator_devices(operator_id,device_code,warehouse_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, operatorID, deviceID, warehouseID)
		if err != nil {
			panic(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		panic(err)
	}
	fmt.Println("development identity seed complete")
}
func required(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		panic(key + " is required")
	}
	return value
}
func value(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
