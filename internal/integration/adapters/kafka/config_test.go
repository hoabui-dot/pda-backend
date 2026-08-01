package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
)

func TestPublisherFailsClosedWithoutBroker(t *testing.T) {
	e := event.DomainEventEnvelope{EventID: uuid.New(), CausationID: uuid.New(), EventVersion: 1, AggregateVersion: 1, OccurredAt: time.Now(), EventType: "x", AggregateType: "x", AggregateID: "a", CorrelationID: "c", WarehouseID: "w", OperatorID: "o", DeviceID: "d", Topic: "x", Payload: []byte(`{}`)}
	if !errors.Is((&Publisher{}).Publish(context.Background(), e), ErrBrokerUnavailable) {
		t.Fatal("publisher must fail closed")
	}
}
