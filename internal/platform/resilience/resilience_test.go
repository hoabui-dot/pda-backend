package resilience

import (
	"context"
	"errors"
	"github.com/sony/gobreaker/v2"
	"sync"
	"testing"
	"time"
)

func settings() Settings {
	return Settings{Name: "test", Timeout: 10 * time.Millisecond, Retries: 2, Bulkhead: 1, BaseDelay: time.Millisecond, Breaker: gobreaker.Settings{MaxRequests: 1, Interval: time.Hour, Timeout: 15 * time.Millisecond, ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 5 }}}
}
func TestRetryTimeoutAndPreservedIdempotency(t *testing.T) {
	p := New[string](settings())
	calls := 0
	token := "same"
	v, e := p.Execute(context.Background(), true, func(context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "", errors.New("temporary")
		}
		return token, nil
	})
	if e != nil || v != token || calls != 3 {
		t.Fatal(v, e, calls)
	}
	p = New[string](settings())
	_, e = p.Execute(context.Background(), false, func(c context.Context) (string, error) { <-c.Done(); return "", c.Err() })
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(e)
	}
}
func TestCircuitOpenHalfOpenClosed(t *testing.T) {
	s := settings()
	s.Retries = 0
	s.Breaker.ReadyToTrip = func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 2 }
	p := New[int](s)
	fail := func(context.Context) (int, error) { return 0, errors.New("fail") }
	_, _ = p.Execute(context.Background(), false, fail)
	_, _ = p.Execute(context.Background(), false, fail)
	if p.State() != gobreaker.StateOpen {
		t.Fatal(p.State())
	}
	time.Sleep(20 * time.Millisecond)
	if p.State() != gobreaker.StateHalfOpen {
		t.Fatal(p.State())
	}
	if _, e := p.Execute(context.Background(), false, func(context.Context) (int, error) { return 1, nil }); e != nil || p.State() != gobreaker.StateClosed {
		t.Fatal(e, p.State())
	}
}
func TestBulkheadIsolationAndNoMutationRetry(t *testing.T) {
	p := New[int](settings())
	entered := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = p.Execute(context.Background(), false, func(context.Context) (int, error) { close(entered); <-release; return 1, nil })
	}()
	<-entered
	calls := 0
	_, e := p.Execute(context.Background(), false, func(context.Context) (int, error) { calls++; return 0, nil })
	if !errors.Is(e, ErrBulkheadFull) || calls != 0 {
		t.Fatal(e, calls)
	}
	close(release)
	wg.Wait()
	p = New[int](settings())
	_, _ = p.Execute(context.Background(), false, func(context.Context) (int, error) { calls++; return 0, errors.New("mutation failed") })
	if calls != 1 {
		t.Fatal(calls)
	}
}
