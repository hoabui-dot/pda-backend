//go:build integration

package httpadapter

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	receivingpostgres "github.com/company/pda-backend/internal/execution/receiving/adapters/postgres"
	receivingapp "github.com/company/pda-backend/internal/execution/receiving/application"
	identitymock "github.com/company/pda-backend/internal/identity/adapters/mock"
	identityapp "github.com/company/pda-backend/internal/identity/application"
	"github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	"github.com/company/pda-backend/internal/integration/adapters/wmsmock"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReceivingAPIFromListThroughQuantityConfirmation(t *testing.T) {
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, os.Getenv("PDA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	_, err = admin.Exec(ctx, `DROP SCHEMA IF EXISTS be03_api CASCADE;CREATE SCHEMA be03_api`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA IF EXISTS be03_api CASCADE`) })
	cfg, err := pgxpool.ParseConfig(os.Getenv("PDA_TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "be03_api"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, path := range []string{"../../../../migrations/execution/000001_task_core.up.sql", "../../../../migrations/execution/000002_receiving.up.sql"} {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(sql)); err != nil {
			t.Fatal(err)
		}
	}
	identityStore, err := identitymock.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := identitymock.NewTokenProvider("local-mock-secret-must-never-be-production", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	identityService := identityapp.NewService(identityStore, tokens, identityStore, identityStore, func() time.Time { return fixedTime })
	receivingStore := receivingpostgres.New(pool)
	fixtures, err := wmsmock.NewReceivingAdapter().Tasks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := receivingStore.Seed(ctx, fixtures); err != nil {
		t.Fatal(err)
	}
	log := messagingmock.NewInMemoryEventLog()
	receivingService := receivingapp.New(receivingStore, receivingpostgres.Commands{Store: receivingStore}, receivingStore, receivingStore, receivingStore, messagingmock.NewPublisher(log, ""), receivingStore, func() time.Time { return fixedTime })
	handler, err := New(identityService, nil, receivingService, nil, nil, nil, nil, nil, nil, nil, Settings{RequestTimeout: time.Second, AuthRateLimit: 20, RateWindow: time.Minute, CircuitFailureThreshold: 3}, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return fixedTime })
	if err != nil {
		t.Fatal(err)
	}
	token := login(t, handler)
	if response := request(handler, http.MethodPost, "/api/pda/v1/devices/registrations", `{"deviceId":"DEV-R","warehouseId":"WH-01"}`, token, nil); response.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", response.Code, response.Body.String())
	}
	headers := map[string]string{"X-Device-Id": "DEV-R", "X-Warehouse-Id": "WH-01"}
	if response := request(handler, http.MethodGet, "/api/pda/v1/receiving/tasks", "", token, headers); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "REC-001") {
		t.Fatalf("list: %d %s", response.Code, response.Body.String())
	}
	// The cross-workflow task feed includes receiving work. The generic detail
	// route must resolve the same receiving owner record instead of returning
	// TASK_NOT_FOUND from the execution store.
	if response := request(handler, http.MethodGet, "/api/pda/v1/tasks/REC-001", "", token, headers); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"category":"RECEIVING"`) || !strings.Contains(response.Body.String(), "LINE-01") {
		t.Fatalf("generic receiving detail: %d %s", response.Code, response.Body.String())
	}
	startHeaders := map[string]string{"X-Device-Id": "DEV-R", "X-Warehouse-Id": "WH-01", "Idempotency-Key": "00000000-0000-0000-0000-000000000601", "If-Match": `"1"`}
	if response := request(handler, http.MethodPost, "/api/pda/v1/receiving/tasks/REC-001/start", "", token, startHeaders); response.Code != http.StatusOK {
		t.Fatalf("start: %d %s", response.Code, response.Body.String())
	}
	if response := request(handler, http.MethodPost, "/api/pda/v1/receiving/tasks/REC-001/barcode-resolutions", `{"barcode":"00012345678905"}`, token, headers); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "LINE-01") {
		t.Fatalf("barcode: %d %s", response.Code, response.Body.String())
	}
	confirmHeaders := map[string]string{"X-Device-Id": "DEV-R", "X-Warehouse-Id": "WH-01", "Idempotency-Key": "00000000-0000-0000-0000-000000000602", "If-Match": `"2"`}
	body := `{"commandId":"00000000-0000-0000-0000-000000000602","lineId":"LINE-01","barcode":"00012345678905","quantity":3,"baseVersion":2}`
	response := request(handler, http.MethodPost, "/api/pda/v1/receiving/tasks/REC-001/receipts", body, token, confirmHeaders)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"receivedQuantity":3`) {
		t.Fatalf("confirm: %d %s", response.Code, response.Body.String())
	}
	status := request(handler, http.MethodGet, "/api/pda/v1/receiving/commands/00000000-0000-0000-0000-000000000602", "", token, headers)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), "COMPLETED") {
		t.Fatalf("status: %d %s", status.Code, status.Body.String())
	}
}
