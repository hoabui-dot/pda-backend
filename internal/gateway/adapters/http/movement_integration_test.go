//go:build integration

package httpadapter

import (
	"context"
	movementpostgres "github.com/company/pda-backend/internal/execution/movement/adapters/postgres"
	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	identitymock "github.com/company/pda-backend/internal/identity/adapters/mock"
	identityapp "github.com/company/pda-backend/internal/identity/application"
	"github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	"github.com/company/pda-backend/internal/integration/adapters/wmsmock"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestMovementPDAAPIMapping(t *testing.T) {
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, os.Getenv("PDA_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	_, e = admin.Exec(ctx, "DROP SCHEMA IF EXISTS be04_api CASCADE;CREATE SCHEMA be04_api")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA IF EXISTS be04_api CASCADE") })
	cfg, e := pgxpool.ParseConfig(os.Getenv("PDA_TEST_DATABASE_URL"))
	if e != nil {
		t.Fatal(e)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "be04_api"
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	for _, p := range []string{"../../../../migrations/execution/000001_task_core.up.sql", "../../../../migrations/execution/000002_receiving.up.sql", "../../../../migrations/execution/000003_movement_workflows.up.sql"} {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(b)); e != nil {
			t.Fatal(e)
		}
	}
	ms := movementpostgres.New(pool)
	if e = ms.Seed(ctx, wmsmock.MovementTasks()); e != nil {
		t.Fatal(e)
	}
	events := messagingmock.NewInMemoryEventLog()
	moves := movementapp.New(ms, movementpostgres.Commands{Store: ms}, ms, ms, ms, messagingmock.NewPublisher(events, ""), ms, func() time.Time { return fixedTime })
	ids, e := identitymock.LoadDefault()
	if e != nil {
		t.Fatal(e)
	}
	tokens, e := identitymock.NewTokenProvider("local-mock-secret-must-never-be-production", time.Hour)
	if e != nil {
		t.Fatal(e)
	}
	identity := identityapp.NewService(ids, tokens, ids, ids, func() time.Time { return fixedTime })
	h, e := New(identity, nil, nil, moves, nil, nil, Settings{RequestTimeout: time.Second, AuthRateLimit: 30, RateWindow: time.Minute, CircuitFailureThreshold: 3}, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return fixedTime })
	if e != nil {
		t.Fatal(e)
	}
	token := login(t, h)
	if x := request(h, http.MethodPost, "/api/pda/v1/devices/registrations", `{"deviceId":"DEV-04","warehouseId":"WH-01"}`, token, nil); x.Code != http.StatusCreated {
		t.Fatal(x.Body.String())
	}
	headers := map[string]string{"X-Device-Id": "DEV-04", "X-Warehouse-Id": "WH-01"}
	if x := request(h, http.MethodGet, "/api/pda/v1/putaway/tasks", "", token, headers); x.Code != http.StatusOK {
		t.Fatalf("list %d %s", x.Code, x.Body.String())
	}
	step := func(path, body string, version int64) {
		hh := map[string]string{"X-Device-Id": "DEV-04", "X-Warehouse-Id": "WH-01", "Idempotency-Key": uuid.NewString(), "If-Match": strconv.FormatInt(version, 10)}
		x := request(h, http.MethodPost, path, body, token, hh)
		if x.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, x.Code, x.Body.String())
		}
	}
	step("/api/pda/v1/putaway/tasks/PUT-001/source-validations", `{"location":"STAGE-01"}`, 1)
	step("/api/pda/v1/putaway/tasks/PUT-001/destination-validations", `{"location":"BULK-01"}`, 2)
	step("/api/pda/v1/putaway/tasks/PUT-001/confirmation", `{"quantity":5}`, 3)
	if len(events.All(ctx)) != 3 {
		t.Fatalf("events=%d", len(events.All(ctx)))
	}
}
