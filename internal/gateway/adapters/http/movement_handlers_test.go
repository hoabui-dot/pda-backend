package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestValidateMovementMirrorsAllowsSeparateCommandAndIdempotencyIDs(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/pda/v1/putaway/tasks/task-1/destination-validations", nil)
	request.Header.Set("Idempotency-Key", "00000000-0000-0000-0000-000000000101")
	request.Header.Set("If-Match", "7")

	if err := validateMovementMirrors(request, "", uuid.MustParse("00000000-0000-0000-0000-000000000202"), request.Header.Get("Idempotency-Key"), 7); err != nil {
		t.Fatalf("separate command and idempotency IDs should be accepted: %v", err)
	}
}
