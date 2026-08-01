package resilient

import (
	"context"
	"github.com/company/pda-backend/internal/integration/ports"
	pcache "github.com/company/pda-backend/internal/platform/cache"
)

type CachedWMS struct {
	next  ports.UpstreamWMS
	cache *pcache.Aside
	keys  pcache.KeyService
}

func NewCachedWMS(n ports.UpstreamWMS, c *pcache.Aside, k pcache.KeyService) *CachedWMS {
	return &CachedWMS{n, c, k}
}
func (w *CachedWMS) Warehouses(c context.Context) ([]ports.Warehouse, error) {
	r, e := pcache.Get(c, w.cache, w.keys.Key("master", "warehouses"), w.next.Warehouses)
	return r.Value, e
}
