package contract_test

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAPIContainsOperationalEndpoints(t *testing.T) {
	content, err := os.ReadFile("../../api/openapi/pda-v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"/healthz", "/livez", "/readyz", "/auth/login", "/auth/refresh", "/auth/logout", "/me", "/me/warehouses", "/devices/registrations", "/bootstrap", "/dashboard", "/tasks/summary", "/tasks", "/tasks/{taskId}/claim", "/tasks/{taskId}/release", "/receiving/tasks", "/receiving/tasks/{taskId}", "/receiving/tasks/{taskId}/start", "/receiving/tasks/{taskId}/barcode-resolutions", "/receiving/tasks/{taskId}/receipts", "/receiving/tasks/{taskId}/completion", "/receiving/commands/{commandId}", "/putaway/tasks", "/putaway/tasks/{taskId}/source-validations", "/putaway/tasks/{taskId}/destination-suggestions", "/putaway/tasks/{taskId}/destination-validations", "/putaway/tasks/{taskId}/confirmation", "/picking/tasks", "/picking/tasks/{taskId}/location-validations", "/picking/tasks/{taskId}/barcode-resolutions", "/picking/tasks/{taskId}/picks", "/picking/tasks/{taskId}/completion", "/replenishment/tasks", "/replenishment/tasks/{taskId}/source-validations", "/replenishment/tasks/{taskId}/destination-validations", "/replenishment/tasks/{taskId}/item-validations", "/replenishment/tasks/{taskId}/confirmation"} {
		if !strings.Contains(string(content), endpoint) {
			t.Fatalf("missing %s", endpoint)
		}
	}
	for _, contract := range []string{"bearerAuth", "X-Correlation-Id", "X-Device-Id", "X-Warehouse-Id", "Idempotency-Key", "If-Match"} {
		if !strings.Contains(string(content), contract) {
			t.Fatalf("missing contract %s", contract)
		}
	}
	for _, endpoint := range []string{"/inventory/search", "/inventory/balances", "/inventory/movements", "/inventory/transfers/source-validations", "/inventory/transfers/destination-validations", "/inventory/transfers/item-resolutions", "/inventory/transfers", "/cycle-count/tasks", "/cycle-count/tasks/{taskId}/counts", "/cycle-count/tasks/{taskId}/recounts", "/cycle-count/tasks/{taskId}/completion"} {
		if !strings.Contains(string(content), endpoint) {
			t.Fatalf("missing BE-05 endpoint %s", endpoint)
		}
	}
	for _, endpoint := range []string{"/shipments/{shipmentId}", "/shipments/{shipmentId}/readiness", "/shipments/{shipmentId}/confirmation", "/shipments/{shipmentId}/commands/{commandId}"} {
		if !strings.Contains(string(content), endpoint) {
			t.Fatalf("missing BE-06 endpoint %s", endpoint)
		}
	}
}
