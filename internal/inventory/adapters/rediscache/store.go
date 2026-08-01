package rediscache

import (
	"context"
	"github.com/company/pda-backend/internal/inventory/domain"
	"github.com/company/pda-backend/internal/inventory/ports"
	pcache "github.com/company/pda-backend/internal/platform/cache"
)

type Store struct {
	next  ports.Repository
	cache *pcache.Aside
	keys  pcache.KeyService
}

func New(n ports.Repository, c *pcache.Aside, k pcache.KeyService) *Store { return &Store{n, c, k} }
func (s *Store) Search(c context.Context, w, q string) ([]domain.Balance, error) {
	key := s.keys.Key("inventory-search", w, q)
	r, e := pcache.Get(c, s.cache, key, func(c context.Context) ([]domain.Balance, error) { return s.next.Search(c, w, q) })
	return r.Value, e
}
func (s *Store) Balances(c context.Context, w, i, l string) ([]domain.Balance, error) {
	key := s.keys.Key("inventory-balance", w, i, l)
	r, e := pcache.Get(c, s.cache, key, func(c context.Context) ([]domain.Balance, error) { return s.next.Balances(c, w, i, l) })
	return r.Value, e
}
func (s *Store) Movements(c context.Context, w, i, x string) ([]domain.Movement, error) {
	return s.next.Movements(c, w, i, x)
}
func (s *Store) ValidateLocations(c context.Context, w, a, b, i string, q int64) error {
	return s.next.ValidateLocations(c, w, a, b, i, q)
}
func (s *Store) Transfer(c context.Context, t domain.Transfer) error { return s.next.Transfer(c, t) }
func (s *Store) ListCounts(c context.Context, w, o string) ([]domain.CountTask, error) {
	return s.next.ListCounts(c, w, o)
}
func (s *Store) GetCount(c context.Context, id string) (domain.CountTask, error) {
	return s.next.GetCount(c, id)
}
func (s *Store) GetCountForUpdate(c context.Context, id string) (domain.CountTask, error) {
	return s.next.GetCountForUpdate(c, id)
}
func (s *Store) SaveCount(c context.Context, t domain.CountTask) error { return s.next.SaveCount(c, t) }
func (s *Store) InvalidateInventory(c context.Context, w, i, l string) error {
	_ = s.cache.DeletePattern(c, s.keys.Key("inventory-search", w)+"*")
	_ = s.cache.DeletePattern(c, s.keys.Key("inventory-balance", w)+"*")
	return nil
}
