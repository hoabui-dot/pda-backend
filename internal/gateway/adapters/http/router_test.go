package httpadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	executionmemory "github.com/company/pda-backend/internal/execution/adapters/memory"
	executionapp "github.com/company/pda-backend/internal/execution/application"
	identitymock "github.com/company/pda-backend/internal/identity/adapters/mock"
	identityapp "github.com/company/pda-backend/internal/identity/application"
	messagingmock "github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	wmsmock "github.com/company/pda-backend/internal/integration/adapters/wmsmock"
)

var fixedTime = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

func setup(t *testing.T, limit int, logger *slog.Logger) http.Handler {
	t.Helper()
	store, err := identitymock.LoadDefault()
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := identitymock.NewTokenProvider("local-mock-secret-must-never-be-production", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	service := identityapp.NewService(store, tokens, store, store, func() time.Time { return fixedTime })
	tasks, err := wmsmock.NewTaskAdapter().Tasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	taskStore := executionmemory.New(tasks)
	taskService := executionapp.NewTaskService(taskStore, executionmemory.Idempotency{Store: taskStore}, taskStore, taskStore, messagingmock.NewPublisher(messagingmock.NewInMemoryEventLog(), ""), taskStore, func() time.Time { return fixedTime })
	handler, err := New(service, taskService, nil, nil, nil, nil, Settings{RequestTimeout: 100 * time.Millisecond, AuthRateLimit: limit, RateWindow: time.Minute, CircuitFailureThreshold: 3}, logger, func() time.Time { return fixedTime })
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func request(handler http.Handler, method, path, body, token string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.10:1234"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func login(t *testing.T, handler http.Handler) string {
	t.Helper()
	response := request(handler, http.MethodPost, "/api/pda/v1/auth/login", `{"username":"operator","password":"demo-password"}`, "", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data.AccessToken
}

func TestLoginProfileRegistrationAndBootstrap(t *testing.T) {
	handler := setup(t, 10, slog.New(slog.NewTextHandler(io.Discard, nil)))
	token := login(t, handler)
	if response := request(handler, http.MethodGet, "/api/pda/v1/me", "", token, nil); response.Code != http.StatusOK {
		t.Fatalf("me: %d %s", response.Code, response.Body.String())
	}
	if response := request(handler, http.MethodGet, "/api/pda/v1/me/warehouses", "", token, nil); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "WH-01") {
		t.Fatalf("warehouses: %d %s", response.Code, response.Body.String())
	}
	registration := request(handler, http.MethodPost, "/api/pda/v1/devices/registrations", `{"deviceId":"DEVICE-01","warehouseId":"WH-01"}`, token, nil)
	if registration.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", registration.Code, registration.Body.String())
	}
	bootstrap := request(handler, http.MethodGet, "/api/pda/v1/bootstrap", "", token, map[string]string{"X-Device-Id": "DEVICE-01", "X-Warehouse-Id": "WH-01"})
	if bootstrap.Code != http.StatusOK || !strings.Contains(bootstrap.Body.String(), "Demo Operator") {
		t.Fatalf("bootstrap: %d %s", bootstrap.Code, bootstrap.Body.String())
	}
}

func TestPDASessionFieldsAndRotatingRefreshToken(t *testing.T) {
	handler := setup(t, 10, slog.New(slog.NewTextHandler(io.Discard, nil)))
	response := request(handler, http.MethodPost, "/api/pda/v1/auth/login", `{"username":"operator","password":"demo-password","deviceId":"DEVICE-01","deviceModel":"TC26","appVersion":"1.0","warehouseId":"WH-01","locale":"vi-VN"}`, "", map[string]string{"Accept-Language": "vi-VN"})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"employeeCode":"EMP-0001"`) || !strings.Contains(response.Body.String(), `"refreshToken"`) {
		t.Fatalf("PDA login contract: %d %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	refreshed := request(handler, http.MethodPost, "/api/pda/v1/auth/refresh", `{"refreshToken":"`+envelope.Data.RefreshToken+`","deviceId":"DEVICE-01"}`, "", nil)
	if refreshed.Code != http.StatusOK || strings.Contains(refreshed.Body.String(), envelope.Data.RefreshToken) {
		t.Fatalf("refresh rotation: %d %s", refreshed.Code, refreshed.Body.String())
	}
	replayed := request(handler, http.MethodPost, "/api/pda/v1/auth/refresh", `{"refreshToken":"`+envelope.Data.RefreshToken+`","deviceId":"DEVICE-01"}`, "", nil)
	if replayed.Code != http.StatusUnauthorized {
		t.Fatalf("refresh token replay: %d %s", replayed.Code, replayed.Body.String())
	}
}

func TestInvalidLoginUnauthorizedAndScopeFailures(t *testing.T) {
	handler := setup(t, 10, slog.New(slog.NewTextHandler(io.Discard, nil)))
	invalid := request(handler, http.MethodPost, "/api/pda/v1/auth/login", `{"username":"operator","password":"wrong"}`, "", nil)
	if invalid.Code != http.StatusUnauthorized || !strings.Contains(invalid.Body.String(), "AUTH_INVALID_CREDENTIALS") {
		t.Fatalf("invalid login: %d %s", invalid.Code, invalid.Body.String())
	}
	if response := request(handler, http.MethodGet, "/api/pda/v1/me", "", "", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized: %d", response.Code)
	}
	token := login(t, handler)
	wrongWarehouse := request(handler, http.MethodPost, "/api/pda/v1/devices/registrations", `{"deviceId":"DEVICE-01","warehouseId":"WH-99"}`, token, nil)
	if wrongWarehouse.Code != http.StatusForbidden {
		t.Fatalf("wrong warehouse: %d %s", wrongWarehouse.Code, wrongWarehouse.Body.String())
	}
	wrongDevice := request(handler, http.MethodGet, "/api/pda/v1/bootstrap", "", token, map[string]string{"X-Device-Id": "UNKNOWN", "X-Warehouse-Id": "WH-01"})
	if wrongDevice.Code != http.StatusBadRequest || !strings.Contains(wrongDevice.Body.String(), "DEVICE_NOT_REGISTERED") {
		t.Fatalf("wrong device: %d %s", wrongDevice.Code, wrongDevice.Body.String())
	}
}

func TestErrorsUseCommonEnvelope(t *testing.T) {
	handler := setup(t, 10, slog.New(slog.NewTextHandler(io.Discard, nil)))
	response := request(handler, http.MethodPost, "/api/pda/v1/auth/login", `{"username":"operator","password":"wrong"}`, "", nil)
	var envelope struct {
		Data any `json:"data"`
		Meta struct {
			CorrelationID string    `json:"correlationId"`
			ServerTime    time.Time `json:"serverTime"`
		} `json:"meta"`
		Errors []struct {
			Code      string `json:"code"`
			Retryable bool   `json:"retryable"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusUnauthorized || envelope.Data != nil || len(envelope.Errors) != 1 || envelope.Errors[0].Code != "AUTH_INVALID_CREDENTIALS" {
		t.Fatalf("unexpected error envelope: %s", response.Body.String())
	}
	if envelope.Meta.CorrelationID == "" || envelope.Meta.ServerTime.IsZero() {
		t.Fatalf("missing error metadata: %s", response.Body.String())
	}
}

func TestCommonLanguageAndOperatorHeaders(t *testing.T) {
	handler := setup(t, 10, slog.New(slog.NewTextHandler(io.Discard, nil)))
	localized := request(handler, http.MethodGet, "/healthz", "", "", map[string]string{"Accept-Language": "vi-VN"})
	if localized.Code != http.StatusOK || localized.Header().Get("Content-Language") != "vi-VN" {
		t.Fatalf("language policy: %d %s %s", localized.Code, localized.Header().Get("Content-Language"), localized.Body.String())
	}
	invalidLanguage := request(handler, http.MethodGet, "/healthz", "", "", map[string]string{"Accept-Language": "fr-FR"})
	if invalidLanguage.Code != http.StatusBadRequest || !strings.Contains(invalidLanguage.Body.String(), "INVALID_REQUEST") {
		t.Fatalf("invalid language: %d %s", invalidLanguage.Code, invalidLanguage.Body.String())
	}
	token := login(t, handler)
	mismatch := request(handler, http.MethodGet, "/api/pda/v1/me", "", token, map[string]string{"X-Operator-Id": "OTHER-OPERATOR"})
	if mismatch.Code != http.StatusForbidden || !strings.Contains(mismatch.Body.String(), "OPERATOR_CONTEXT_MISMATCH") {
		t.Fatalf("operator mismatch: %d %s", mismatch.Code, mismatch.Body.String())
	}
}

func TestRefreshRotatesTokenAndLogoutRevokesIt(t *testing.T) {
	handler := setup(t, 10, slog.New(slog.NewTextHandler(io.Discard, nil)))
	original := login(t, handler)
	refreshedResponse := request(handler, http.MethodPost, "/api/pda/v1/auth/refresh", "", original, nil)
	if refreshedResponse.Code != http.StatusOK {
		t.Fatalf("refresh: %d %s", refreshedResponse.Code, refreshedResponse.Body.String())
	}
	var envelope struct {
		Data struct {
			AccessToken string `json:"accessToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(refreshedResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.AccessToken == "" || envelope.Data.AccessToken == original {
		t.Fatal("refresh did not rotate token")
	}
	if response := request(handler, http.MethodGet, "/api/pda/v1/me", "", original, nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("old token remained valid: %d", response.Code)
	}
	if response := request(handler, http.MethodPost, "/api/pda/v1/auth/logout", "", envelope.Data.AccessToken, nil); response.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", response.Code, response.Body.String())
	}
	if response := request(handler, http.MethodGet, "/api/pda/v1/me", "", envelope.Data.AccessToken, nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("logged-out token remained valid: %d", response.Code)
	}
}

func TestRateLimitCorrelationTimeoutAndRedactedLogging(t *testing.T) {
	var logs bytes.Buffer
	handler := setup(t, 1, slog.New(slog.NewJSONHandler(&logs, nil)))
	correlationID := "00000000-0000-0000-0000-000000000099"
	first := request(handler, http.MethodPost, "/api/pda/v1/auth/login", `{"username":"operator","password":"demo-password"}`, "", map[string]string{"X-Correlation-Id": correlationID})
	if first.Header().Get("X-Correlation-Id") != correlationID || !strings.Contains(first.Body.String(), correlationID) {
		t.Fatalf("correlation not propagated: %s", first.Body.String())
	}
	second := request(handler, http.MethodPost, "/api/pda/v1/auth/login", `{"username":"operator","password":"demo-password"}`, "", nil)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limit: %d", second.Code)
	}
	if second.Header().Get("Retry-After") == "" {
		t.Fatal("rate limit did not return Retry-After")
	}
	if strings.Contains(logs.String(), "demo-password") || strings.Contains(logs.String(), "eyJ") {
		t.Fatalf("sensitive value logged: %s", logs.String())
	}

	slow := withTimeout(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { time.Sleep(20 * time.Millisecond) }), time.Millisecond)
	timed := httptest.NewRecorder()
	slow.ServeHTTP(timed, httptest.NewRequest(http.MethodGet, "/slow", nil))
	if timed.Code != http.StatusServiceUnavailable || !strings.Contains(timed.Body.String(), "GATEWAY_TIMEOUT") {
		t.Fatalf("timeout: %d %s", timed.Code, timed.Body.String())
	}
}

func TestInvalidGatewayResilienceSettingsRejected(t *testing.T) {
	if _, err := New(nil, nil, nil, nil, nil, nil, Settings{}, slog.Default(), time.Now); err == nil {
		t.Fatal("expected invalid settings rejection")
	}
}

func TestGatewayCircuitOpenHalfOpenClosed(t *testing.T) {
	now := fixedTime
	b := newGatewayBreaker(2, time.Second, func() time.Time { return now })
	b.Record(true)
	if !b.Allow() {
		t.Fatal("opened too early")
	}
	b.Record(true)
	if b.Allow() {
		t.Fatal("circuit did not open")
	}
	now = now.Add(2 * time.Second)
	if !b.Allow() {
		t.Fatal("circuit did not enter half-open trial")
	}
	b.Record(false)
	if !b.Allow() {
		t.Fatal("circuit did not close")
	}
}

func TestDashboardTaskCenterAndClaimReleaseRoutes(t *testing.T) {
	handler := setup(t, 20, slog.New(slog.NewTextHandler(io.Discard, nil)))
	token := login(t, handler)
	registration := request(handler, http.MethodPost, "/api/pda/v1/devices/registrations", `{"deviceId":"DEVICE-01","warehouseId":"WH-01"}`, token, nil)
	if registration.Code != http.StatusCreated {
		t.Fatalf("registration: %d %s", registration.Code, registration.Body.String())
	}
	headers := map[string]string{"X-Device-Id": "DEVICE-01", "X-Warehouse-Id": "WH-01"}
	if response := request(handler, http.MethodGet, "/api/pda/v1/dashboard", "", token, headers); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"inboundCount"`) || !strings.Contains(response.Body.String(), `"actionableAlertCount"`) || !strings.Contains(response.Body.String(), `"asOf"`) {
		t.Fatalf("dashboard: %d %s", response.Code, response.Body.String())
	}
	if response := request(handler, http.MethodGet, "/api/pda/v1/tasks/summary?status=NEW", "", token, headers); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "RECEIVING") {
		t.Fatalf("summary: %d %s", response.Code, response.Body.String())
	}
	if response := request(handler, http.MethodGet, "/api/pda/v1/tasks?status=NEW&limit=1", "", token, headers); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "nextCursor") {
		t.Fatalf("tasks: %d %s", response.Code, response.Body.String())
	}
	if response := request(handler, http.MethodGet, "/api/pda/v1/tasks/TASK-001", "", token, headers); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"lockState":"AVAILABLE"`) || !strings.Contains(response.Body.String(), `"title":"RECEIVING task"`) {
		t.Fatalf("task detail: %d %s", response.Code, response.Body.String())
	}
	claimHeaders := map[string]string{"X-Device-Id": "DEVICE-01", "X-Warehouse-Id": "WH-01", "Idempotency-Key": "00000000-0000-0000-0000-000000000301", "If-Match": `"1"`}
	claim := request(handler, http.MethodPost, "/api/pda/v1/tasks/TASK-001/claim", "", token, claimHeaders)
	if claim.Code != http.StatusOK || !strings.Contains(claim.Body.String(), `"version":2`) {
		t.Fatalf("claim: %d %s", claim.Code, claim.Body.String())
	}
	status := request(handler, http.MethodGet, "/api/pda/v1/commands/00000000-0000-0000-0000-000000000301", "", token, headers)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"status":"ACKNOWLEDGED"`) || !strings.Contains(status.Body.String(), "TASK_MUTATION") {
		t.Fatalf("generic command status: %d %s", status.Code, status.Body.String())
	}
	replay := request(handler, http.MethodPost, "/api/pda/v1/tasks/TASK-001/claim", "", token, claimHeaders)
	if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"version":2`) {
		t.Fatalf("replay: %d %s", replay.Code, replay.Body.String())
	}
	releaseHeaders := map[string]string{"X-Device-Id": "DEVICE-01", "X-Warehouse-Id": "WH-01", "Idempotency-Key": "00000000-0000-0000-0000-000000000302", "If-Match": `"2"`}
	release := request(handler, http.MethodPost, "/api/pda/v1/tasks/TASK-001/release", "", token, releaseHeaders)
	if release.Code != http.StatusOK || !strings.Contains(release.Body.String(), `"status":"NEW"`) {
		t.Fatalf("release: %d %s", release.Code, release.Body.String())
	}
}
