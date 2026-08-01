package messagingmock

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
)

func validEnvelope() event.DomainEventEnvelope {
	return event.DomainEventEnvelope{
		EventID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), EventType: "BootstrapVerified", EventVersion: 1,
		AggregateType: "Bootstrap", AggregateID: "BE-00", AggregateVersion: 1, OccurredAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		CorrelationID: "CORR-01", CausationID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), WarehouseID: "WH-01",
		OperatorID: "OP-01", DeviceID: "DEVICE-01", Topic: "pda.audit.events.v1", Payload: json.RawMessage(`{"fixtureVersion":1}`),
	}
}

func TestPublisherRecordsExactEnvelope(t *testing.T) {
	log := NewInMemoryEventLog()
	publisher := NewPublisher(log, "")
	envelope := validEnvelope()
	if err := publisher.Publish(context.Background(), envelope); err != nil {
		t.Fatal(err)
	}
	if events := log.All(context.Background()); len(events) != 1 || events[0].EventID != envelope.EventID {
		t.Fatalf("unexpected event log: %+v", events)
	}
}

func TestPublisherSupportsDeterministicFailure(t *testing.T) {
	publisher := NewPublisher(NewInMemoryEventLog(), "BootstrapVerified")
	if err := publisher.Publish(context.Background(), validEnvelope()); err == nil {
		t.Fatal("expected deterministic failure")
	}
}
