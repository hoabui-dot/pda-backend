package rediscache

import (
	"context"
	pcache "github.com/company/pda-backend/internal/platform/cache"
	"github.com/company/pda-backend/internal/shipping/domain"
	"github.com/company/pda-backend/internal/shipping/ports"
)

type Store struct {
	next  ports.Repository
	cache *pcache.Aside
	keys  pcache.KeyService
}

func New(n ports.Repository, c *pcache.Aside, k pcache.KeyService) *Store { return &Store{n, c, k} }
func (s *Store) Get(c context.Context, id string) (domain.Shipment, error) {
	key := s.keys.Key("shipment", id)
	r, e := pcache.Get(c, s.cache, key, func(c context.Context) (domain.Shipment, error) { return s.next.Get(c, id) })
	return r.Value, e
}
func (s *Store) GetForUpdate(c context.Context, id string) (domain.Shipment, error) {
	return s.next.GetForUpdate(c, id)
}
func (s *Store) Save(c context.Context, x domain.Shipment) error { return s.next.Save(c, x) }
func (s *Store) ProjectPickingComplete(c context.Context, id string) error {
	e := s.next.ProjectPickingComplete(c, id)
	if e == nil {
		_ = s.cache.Delete(c, s.keys.Key("shipment", id))
	}
	return e
}
func (s *Store) InvalidateShippingViews(c context.Context, w, o string) error {
	_ = s.cache.DeletePattern(c, s.keys.Key("shipment")+"*")
	return nil
}
