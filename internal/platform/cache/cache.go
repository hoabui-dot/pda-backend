package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/sync/singleflight"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrMiss = errors.New("cache miss")

type KeyService struct{ Version string }

func (k KeyService) Key(kind string, parts ...string) string {
	clean := []string{"pda", k.Version, kind}
	for _, p := range parts {
		clean = append(clean, strings.ReplaceAll(p, ":", "_"))
	}
	return strings.Join(clean, ":")
}

type Metrics struct {
	Hits, Misses, Errors atomic.Uint64
	LatencyNanos         atomic.Uint64
	StaleServed          atomic.Uint64
	InvalidationSuccess  atomic.Uint64
	InvalidationFailure  atomic.Uint64
}
type Snapshot struct {
	Hits, Misses, Errors, LatencyNanos                    uint64
	StaleServed, InvalidationSuccess, InvalidationFailure uint64
}

func (m *Metrics) Snapshot() Snapshot {
	return Snapshot{
		Hits: m.Hits.Load(), Misses: m.Misses.Load(), Errors: m.Errors.Load(), LatencyNanos: m.LatencyNanos.Load(),
		StaleServed: m.StaleServed.Load(), InvalidationSuccess: m.InvalidationSuccess.Load(), InvalidationFailure: m.InvalidationFailure.Load(),
	}
}

type Store interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, any, time.Duration) error
	Del(context.Context, ...string) error
	Scan(context.Context, uint64, string, int64) ([]string, uint64, error)
}
type Aside struct {
	store   Store
	ttl     time.Duration
	metrics *Metrics
	group   singleflight.Group
	stale   sync.Map
}

func NewAside(s Store, ttl time.Duration, m *Metrics) *Aside {
	return &Aside{s, ttl, m, singleflight.Group{}, sync.Map{}}
}

type Result[T any] struct {
	Value      T
	Hit, Stale bool
}

func Get[T any](c context.Context, a *Aside, key string, load func(context.Context) (T, error)) (Result[T], error) {
	start := time.Now()
	defer func() { a.metrics.LatencyNanos.Add(uint64(time.Since(start))) }()
	raw, e := a.store.Get(c, key)
	if e == nil {
		var v T
		if json.Unmarshal([]byte(raw), &v) == nil {
			a.metrics.Hits.Add(1)
			a.stale.Store(key, v)
			return Result[T]{Value: v, Hit: true}, nil
		}
	}
	if e != nil && !errors.Is(e, ErrMiss) {
		a.metrics.Errors.Add(1)
	} else {
		a.metrics.Misses.Add(1)
	}
	value, e, _ := a.group.Do(key, func() (any, error) {
		v, x := load(c)
		if x != nil {
			return v, x
		}
		b, _ := json.Marshal(v)
		if x = a.store.Set(c, key, b, a.ttl); x != nil {
			a.metrics.Errors.Add(1)
		}
		a.stale.Store(key, v)
		return v, nil
	})
	if e != nil {
		if v, ok := a.stale.Load(key); ok {
			a.metrics.StaleServed.Add(1)
			return Result[T]{Value: v.(T), Stale: true}, nil
		}
		var zero T
		return Result[T]{Value: zero}, e
	}
	return Result[T]{Value: value.(T)}, nil
}
func (a *Aside) Delete(c context.Context, keys ...string) error { return a.store.Del(c, keys...) }
func (a *Aside) DeletePattern(c context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, next, e := a.store.Scan(c, cursor, pattern, 100)
		if e != nil {
			return e
		}
		if len(keys) > 0 {
			if e = a.store.Del(c, keys...); e != nil {
				return e
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
func (a *Aside) TTL() time.Duration { return a.ttl }
func ValidateTTL(v time.Duration) error {
	if v <= 0 || v > 24*time.Hour {
		return fmt.Errorf("cache TTL must be between zero and 24h")
	}
	return nil
}
