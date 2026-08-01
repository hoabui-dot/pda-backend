package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memory struct {
	mu   sync.Mutex
	data map[string]string
	down bool
}

func (m *memory) Get(_ context.Context, k string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.down {
		return "", errors.New("redis down")
	}
	v, ok := m.data[k]
	if !ok {
		return "", ErrMiss
	}
	return v, nil
}
func (m *memory) Set(_ context.Context, k string, v any, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.down {
		return errors.New("redis down")
	}
	m.data[k] = string(v.([]byte))
	return nil
}
func (m *memory) Del(_ context.Context, k ...string) error {
	for _, x := range k {
		delete(m.data, x)
	}
	return nil
}
func (m *memory) Scan(context.Context, uint64, string, int64) ([]string, uint64, error) {
	return nil, 0, nil
}
func TestHitMissOutageStaleAndStampede(t *testing.T) {
	m := &memory{data: map[string]string{}}
	metrics := &Metrics{}
	a := NewAside(m, time.Minute, metrics)
	loads := 0
	load := func(context.Context) (int, error) { loads++; return 7, nil }
	x, e := Get(context.Background(), a, "k", load)
	if e != nil || x.Hit || x.Value != 7 {
		t.Fatal(x, e)
	}
	x, e = Get(context.Background(), a, "k", load)
	if e != nil || !x.Hit || loads != 1 {
		t.Fatal(x, e, loads)
	}
	m.down = true
	x, e = Get(context.Background(), a, "k", func(context.Context) (int, error) { return 0, errors.New("db down") })
	if e != nil || !x.Stale || x.Value != 7 {
		t.Fatal(x, e)
	}
	if metrics.Snapshot().Hits != 1 || metrics.Snapshot().Errors == 0 {
		t.Fatal(metrics.Snapshot())
	}
}
func TestVersionedKeysAndTTL(t *testing.T) {
	k := KeyService{Version: "v1"}.Key("dashboard", "WH:01", "OP")
	if k != "pda:v1:dashboard:WH_01:OP" {
		t.Fatal(k)
	}
	if ValidateTTL(0) == nil || ValidateTTL(time.Minute) != nil {
		t.Fatal("ttl validation")
	}
}

func TestBestEffortInvalidationDuringRedisOutage(t *testing.T) {
	m := &memory{data: map[string]string{}, down: true}
	inv := Invalidator{Cache: NewAside(m, time.Minute, &Metrics{}), Keys: KeyService{Version: "v1"}}
	if err := inv.InvalidateMovementViews(context.Background(), "WH", "OP"); err != nil {
		t.Fatal(err)
	}
}

func TestSingleflightPreventsStampede(t *testing.T) {
	m := &memory{data: map[string]string{}}
	a := NewAside(m, time.Minute, &Metrics{})
	var loads atomic.Int64
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := Get(context.Background(), a, "shared", func(context.Context) (int, error) {
				loads.Add(1)
				time.Sleep(10 * time.Millisecond)
				return 9, nil
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if loads.Load() != 1 {
		t.Fatalf("loads=%d", loads.Load())
	}
}
