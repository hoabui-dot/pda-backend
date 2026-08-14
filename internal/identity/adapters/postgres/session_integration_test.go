//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/company/pda-backend/internal/identity/adapters/security"
	"github.com/company/pda-backend/internal/identity/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSessionManagerPersistsRotationReuseAndLogout(t *testing.T) {
	ctx := context.Background()
	databaseURL := os.Getenv("PDA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PDA_TEST_DATABASE_URL is not configured")
	}

	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	const schema = "identity_phase11_integration"
	if _, err = admin.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE; CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	up, err := os.ReadFile("../../../../migrations/execution/000007_identity.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../../../migrations/execution/000007_identity.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}

	passwordHash, err := security.DefaultArgon2id().Hash("integration-password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO identity_operators(id,username,employee_code,display_name,password_hash) VALUES('OP-INT','operator','EMP-INT','Integration Operator',$1)`, passwordHash); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO identity_warehouses(warehouse_id,warehouse_name) VALUES('WH-INT','Integration Warehouse')`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO identity_operator_warehouses(operator_id,warehouse_id) VALUES('OP-INT','WH-INT')`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO identity_devices(device_code,approved_at) VALUES('DEV-INT',now())`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO identity_operator_devices(operator_id,device_code,warehouse_id) VALUES('OP-INT','DEV-INT','WH-INT')`); err != nil {
		t.Fatal(err)
	}

	store := New(pool)
	manager, err := NewSessionManager(store, []byte("integration-secret-with-at-least-32-bytes"), "pda", "pda-api", 15*time.Minute, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	operator, found, err := store.ByUsername(ctx, "operator")
	if err != nil || !found {
		t.Fatalf("load operator: found=%v err=%v", found, err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	access, refresh, _, refreshExpires, err := manager.Create(ctx, operator, "DEV-INT", "WH-INT", now)
	if err != nil {
		t.Fatal(err)
	}
	if want := now.Add(30 * 24 * time.Hour); !refreshExpires.Equal(want) {
		t.Fatalf("refresh expiry=%s, want %s", refreshExpires, want)
	}
	if _, err = manager.Authenticate(ctx, access, now); err != nil {
		t.Fatalf("initial access token: %v", err)
	}
	restartedManager, err := NewSessionManager(store, []byte("integration-secret-with-at-least-32-bytes"), "pda", "pda-api", 15*time.Minute, 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restartedManager.Authenticate(ctx, access, now); err != nil {
		t.Fatalf("session did not survive manager restart: %v", err)
	}

	rotatedAccess, rotatedRefresh, _, rotatedExpiry, _, _, _, err := manager.Refresh(ctx, refresh, "DEV-INT", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if rotatedRefresh == refresh {
		t.Fatal("refresh token was not rotated")
	}
	if !rotatedExpiry.Equal(refreshExpires) {
		t.Fatalf("rotation extended refresh expiry to %s", rotatedExpiry)
	}
	if _, err = manager.Authenticate(ctx, rotatedAccess, now.Add(time.Second)); err != nil {
		t.Fatalf("rotated access token: %v", err)
	}
	if _, _, _, _, _, _, _, err = manager.Refresh(ctx, refresh, "DEV-INT", now.Add(2*time.Second)); err == nil {
		t.Fatal("expected refresh reuse to fail")
	}
	if _, err = manager.Authenticate(ctx, rotatedAccess, now.Add(2*time.Second)); err == nil {
		t.Fatal("refresh reuse did not revoke the session")
	}

	_, logoutRefresh, _, _, err := manager.Create(ctx, operator, "DEV-INT", "WH-INT", now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	logoutAccess, _, _, _, _, _, _, err := manager.Refresh(ctx, logoutRefresh, "DEV-INT", now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	logoutClaims, err := manager.Authenticate(ctx, logoutAccess, now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.Logout(ctx, logoutAccess, now.Add(5*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Authenticate(ctx, logoutAccess, now.Add(5*time.Second)); err == nil {
		t.Fatal("logout did not revoke access token")
	}
	if _, _, _, _, _, _, _, err = manager.Refresh(ctx, logoutRefresh, "DEV-INT", now.Add(5*time.Second)); err == nil {
		t.Fatal("logout did not revoke refresh token")
	}
	if logoutClaims.SessionID == "" {
		t.Fatal("logout session claims missing")
	}
	_, expiredRefresh, _, _, err := manager.Create(ctx, operator, "DEV-INT", "WH-INT", now.Add(8*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, _, _, _, err = manager.Refresh(ctx, expiredRefresh, "DEV-INT", now.Add(8*time.Second+30*24*time.Hour)); !errors.Is(err, ports.ErrRefreshTokenExpired) {
		t.Fatalf("expired refresh error=%v, want REFRESH_TOKEN_EXPIRED", err)
	}

	_, concurrentRefresh, _, _, err := manager.Create(ctx, operator, "DEV-INT", "WH-INT", now.Add(6*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	type refreshResult struct {
		access string
		err    error
	}
	results := make(chan refreshResult, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	for range 2 {
		go func() {
			defer wait.Done()
			accessToken, _, _, _, _, _, _, refreshErr := manager.Refresh(ctx, concurrentRefresh, "DEV-INT", now.Add(7*time.Second))
			results <- refreshResult{access: accessToken, err: refreshErr}
		}()
	}
	wait.Wait()
	close(results)
	var successfulAccess string
	successes := 0
	for result := range results {
		if result.err == nil {
			successes++
			successfulAccess = result.access
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent refresh successes=%d, want 1", successes)
	}
	if _, err = manager.Authenticate(ctx, successfulAccess, now.Add(7*time.Second)); err == nil {
		t.Fatal("concurrent refresh did not revoke the session family")
	}
	if _, err = pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	var identityTable *string
	if err = pool.QueryRow(ctx, `SELECT to_regclass('identity_operators')`).Scan(&identityTable); err != nil {
		t.Fatal(err)
	}
	if identityTable != nil {
		t.Fatalf("down migration left identity_operators table: %s", *identityTable)
	}
}
