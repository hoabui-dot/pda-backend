package resilient

import (
	"context"
	"errors"
	"github.com/company/pda-backend/internal/integration/ports"
	"github.com/company/pda-backend/internal/platform/resilience"
	"github.com/sony/gobreaker/v2"
	"testing"
	"time"
)

type fake struct{ calls int }

func (f *fake) Warehouses(context.Context) ([]ports.Warehouse, error) {
	f.calls++
	if f.calls < 2 {
		return nil, errors.New("temporary")
	}
	return []ports.Warehouse{{ID: "WH"}}, nil
}
func TestWMSBoundaryRetry(t *testing.T) {
	f := &fake{}
	p := resilience.New[[]ports.Warehouse](resilience.Settings{Name: "wms", Timeout: time.Second, Retries: 1, Bulkhead: 2, BaseDelay: time.Millisecond, Breaker: gobreaker.Settings{ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures > 5 }}})
	v, e := NewWMS(f, p).Warehouses(context.Background())
	if e != nil || len(v) != 1 || f.calls != 2 {
		t.Fatal(v, e, f.calls)
	}
}
