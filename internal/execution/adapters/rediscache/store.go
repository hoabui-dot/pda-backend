package rediscache

import (
	"context"
	"github.com/company/pda-backend/internal/execution/domain"
	"github.com/company/pda-backend/internal/execution/ports"
	pcache "github.com/company/pda-backend/internal/platform/cache"
)

type Store struct {
	next  ports.TaskRepository
	cache *pcache.Aside
	keys  pcache.KeyService
}

func New(n ports.TaskRepository, c *pcache.Aside, k pcache.KeyService) *Store { return &Store{n, c, k} }
func (s *Store) Dashboard(c context.Context, w, o string) (ports.Dashboard, error) {
	key := s.keys.Key("dashboard", w, o)
	r, e := pcache.Get(c, s.cache, key, func(c context.Context) (ports.Dashboard, error) { return s.next.Dashboard(c, w, o) })
	return r.Value, e
}
func (s *Store) Summary(c context.Context, w, o, status string) ([]ports.SummaryItem, error) {
	key := s.keys.Key("task-summary", w, o, status)
	r, e := pcache.Get(c, s.cache, key, func(c context.Context) ([]ports.SummaryItem, error) { return s.next.Summary(c, w, o, status) })
	return r.Value, e
}
func (s *Store) List(c context.Context, f ports.TaskFilter) (ports.TaskPage, error) {
	return s.next.List(c, f)
}
func (s *Store) GetForUpdate(c context.Context, id string) (domain.Task, error) {
	return s.next.GetForUpdate(c, id)
}
func (s *Store) Save(c context.Context, t domain.Task) error { return s.next.Save(c, t) }
func (s *Store) InvalidateTaskViews(c context.Context, w, o string) error {
	_ = s.cache.DeletePattern(c, s.keys.Key("dashboard", w, o))
	_ = s.cache.DeletePattern(c, s.keys.Key("task-summary", w, o)+"*")
	return nil
}
