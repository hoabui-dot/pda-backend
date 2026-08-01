package resilient

import (
	"context"
	"github.com/company/pda-backend/internal/integration/ports"
	"github.com/company/pda-backend/internal/platform/resilience"
)

type WMS struct {
	next   ports.UpstreamWMS
	policy *resilience.Policy[[]ports.Warehouse]
}

func NewWMS(n ports.UpstreamWMS, p *resilience.Policy[[]ports.Warehouse]) *WMS { return &WMS{n, p} }
func (w *WMS) Warehouses(c context.Context) ([]ports.Warehouse, error) {
	return w.policy.Execute(c, true, w.next.Warehouses)
}
