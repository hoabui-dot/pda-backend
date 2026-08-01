package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/company/pda-backend/internal/platform/config"
)

func TestHealthEndpoints(t *testing.T) {
	fixed := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	handler := Handler("test-service", config.Modes{
		Messaging: "mock", UpstreamWMS: "mock", Auth: "mock",
	}, func() time.Time { return fixed })
	for _, path := range []string{"/healthz", "/livez", "/readyz"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d", path, response.Code)
		}
		if !strings.Contains(response.Body.String(), `"service":"test-service"`) {
			t.Fatalf("unexpected response: %s", response.Body.String())
		}
	}
}
