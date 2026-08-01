//go:build integration

package redisadapter

import (
	"context"
	pcache "github.com/company/pda-backend/internal/platform/cache"
	"github.com/redis/go-redis/v9"
	"testing"
	"time"
)

func TestRealRedisCacheInvalidationTTLAndRateLimit(t *testing.T) {
	c := context.Background()
	r := redis.NewClient(&redis.Options{Addr: "localhost:16379", DB: 15})
	defer r.Close()
	if e := r.FlushDB(c).Err(); e != nil {
		t.Fatal(e)
	}
	m := &pcache.Metrics{}
	a := pcache.NewAside(Store{r}, time.Second, m)
	key := pcache.KeyService{Version: "v1"}.Key("dashboard", "WH", "OP")
	loads := 0
	load := func(context.Context) (map[string]int, error) { loads++; return map[string]int{"total": 1}, nil }
	if _, e := pcache.Get(c, a, key, load); e != nil {
		t.Fatal(e)
	}
	x, e := pcache.Get(c, a, key, load)
	if e != nil || !x.Hit || loads != 1 {
		t.Fatal(x, e, loads)
	}
	if m.Snapshot().Hits != 1 || m.Snapshot().Misses != 1 || m.Snapshot().LatencyNanos == 0 {
		t.Fatal(m.Snapshot())
	}
	if ttl := r.TTL(c, key).Val(); ttl <= 0 || ttl > time.Second {
		t.Fatal(ttl)
	}
	inv := pcache.Invalidator{Cache: a, Keys: pcache.KeyService{Version: "v1"}}
	if e = inv.InvalidateTaskViews(c, "WH", "OP"); e != nil {
		t.Fatal(e)
	}
	if r.Exists(c, key).Val() != 0 {
		t.Fatal("not invalidated")
	}
	lim := RateLimiter{r, pcache.KeyService{Version: "v1"}, 2, time.Minute}
	for i := 0; i < 2; i++ {
		ok, e := lim.Allow(c, "device")
		if e != nil || !ok {
			t.Fatal(ok, e)
		}
	}
	ok, e := lim.Allow(c, "device")
	if e != nil || ok {
		t.Fatal(ok, e)
	}
}

func TestEveryDomainInvalidatorRemovesViews(t *testing.T) {
	c := context.Background()
	r := redis.NewClient(&redis.Options{Addr: "localhost:16379", DB: 15})
	defer r.Close()
	if err := r.FlushDB(c).Err(); err != nil {
		t.Fatal(err)
	}
	keys := pcache.KeyService{Version: "v1"}
	a := pcache.NewAside(Store{r}, time.Minute, &pcache.Metrics{})
	inv := pcache.Invalidator{Cache: a, Keys: keys}
	cases := []struct {
		name string
		call func() error
	}{
		{"task", func() error { return inv.InvalidateTaskViews(c, "WH", "OP") }},
		{"receiving", func() error { return inv.InvalidateReceivingViews(c, "WH", "OP") }},
		{"movement", func() error { return inv.InvalidateMovementViews(c, "WH", "OP") }},
		{"inventory", func() error { return inv.InvalidateInventory(c, "WH", "ITEM", "LOC") }},
		{"shipping", func() error { return inv.InvalidateShippingViews(c, "WH", "OP") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := r.FlushDB(c).Err(); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{keys.Key("dashboard", "WH", "OP"), keys.Key("task-summary", "WH", "OP", ""), keys.Key("inventory-balance", "WH", "ITEM", "LOC"), keys.Key("shipment", "SHIP")} {
				if err := r.Set(c, key, "{}", time.Minute).Err(); err != nil {
					t.Fatal(err)
				}
			}
			if err := tc.call(); err != nil {
				t.Fatal(err)
			}
			if n := r.DBSize(c).Val(); n != 0 {
				t.Fatalf("remaining keys=%d", n)
			}
		})
	}
}
