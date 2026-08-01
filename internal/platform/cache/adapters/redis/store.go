package redisadapter

import (
	"context"
	"errors"
	pcache "github.com/company/pda-backend/internal/platform/cache"
	"github.com/redis/go-redis/v9"
	"time"
)

type Store struct{ Client *redis.Client }

func (r Store) Get(c context.Context, k string) (string, error) {
	v, e := r.Client.Get(c, k).Result()
	if errors.Is(e, redis.Nil) {
		return "", pcache.ErrMiss
	}
	return v, e
}
func (r Store) Set(c context.Context, k string, v any, t time.Duration) error {
	return r.Client.Set(c, k, v, t).Err()
}
func (r Store) Del(c context.Context, k ...string) error {
	if len(k) == 0 {
		return nil
	}
	return r.Client.Del(c, k...).Err()
}
func (r Store) Scan(c context.Context, x uint64, p string, n int64) ([]string, uint64, error) {
	return r.Client.Scan(c, x, p, n).Result()
}

type RateLimiter struct {
	Client *redis.Client
	Keys   pcache.KeyService
	Limit  int
	Window time.Duration
}

var rateScript = redis.NewScript("local n=redis.call('INCR',KEYS[1]);if n==1 then redis.call('PEXPIRE',KEYS[1],ARGV[1]) end;return n")

func (r RateLimiter) Allow(c context.Context, subject string) (bool, error) {
	n, e := rateScript.Run(c, r.Client, []string{r.Keys.Key("rate", subject)}, r.Window.Milliseconds()).Int64()
	return n <= int64(r.Limit), e
}
