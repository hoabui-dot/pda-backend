package resilience

import (
	"context"
	"errors"
	"github.com/sony/gobreaker/v2"
	"math/rand/v2"
	"time"
)

var ErrBulkheadFull = errors.New("bulkhead capacity exhausted")

type Policy[T any] struct {
	breaker   *gobreaker.CircuitBreaker[T]
	timeout   time.Duration
	retries   int
	baseDelay time.Duration
	slots     chan struct{}
}
type Settings struct {
	Name              string
	Timeout           time.Duration
	Retries, Bulkhead int
	BaseDelay         time.Duration
	Breaker           gobreaker.Settings
}

func New[T any](s Settings) *Policy[T] {
	s.Breaker.Name = s.Name
	return &Policy[T]{gobreaker.NewCircuitBreaker[T](s.Breaker), s.Timeout, s.Retries, s.BaseDelay, make(chan struct{}, s.Bulkhead)}
}
func (p *Policy[T]) Execute(ctx context.Context, idempotent bool, call func(context.Context) (T, error)) (T, error) {
	var zero T
	select {
	case p.slots <- struct{}{}:
		defer func() { <-p.slots }()
	default:
		return zero, ErrBulkheadFull
	}
	attempts := 1
	if idempotent {
		attempts += p.retries
	}
	var last error
	for i := 0; i < attempts; i++ {
		v, e := p.breaker.Execute(func() (T, error) { c, cancel := context.WithTimeout(ctx, p.timeout); defer cancel(); return call(c) })
		if e == nil {
			return v, nil
		}
		last = e
		if i+1 < attempts {
			delay := p.baseDelay + time.Duration(rand.Int64N(int64(p.baseDelay)+1))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return zero, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return zero, last
}
func (p *Policy[T]) State() gobreaker.State { return p.breaker.State() }
