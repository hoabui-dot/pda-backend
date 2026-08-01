package messagingmock

import (
	"context"
	"fmt"
	"sync"

	"github.com/company/pda-backend/internal/platform/event"
)

type EventLog interface {
	Append(context.Context, event.DomainEventEnvelope) error
	All(context.Context) []event.DomainEventEnvelope
}

type InMemoryEventLog struct {
	mu     sync.RWMutex
	events []event.DomainEventEnvelope
}

func NewInMemoryEventLog() *InMemoryEventLog { return &InMemoryEventLog{} }

func (l *InMemoryEventLog) Append(_ context.Context, envelope event.DomainEventEnvelope) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, envelope)
	return nil
}

func (l *InMemoryEventLog) All(_ context.Context) []event.DomainEventEnvelope {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]event.DomainEventEnvelope(nil), l.events...)
}

type Publisher struct {
	log           EventLog
	failEventType string
}

func NewPublisher(log EventLog, failEventType string) *Publisher {
	return &Publisher{log: log, failEventType: failEventType}
}

func (p *Publisher) Publish(ctx context.Context, envelope event.DomainEventEnvelope) error {
	if err := envelope.Validate(); err != nil {
		return err
	}
	if p.failEventType != "" && envelope.EventType == p.failEventType {
		return fmt.Errorf("deterministic mock publish failure for event type %s", envelope.EventType)
	}
	return p.log.Append(ctx, envelope)
}
