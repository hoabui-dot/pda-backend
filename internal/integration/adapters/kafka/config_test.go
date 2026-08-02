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

func TestResolveTopicDoesNotDoublePrefixQualifiedTopic(t *testing.T) {
	if got := ResolveTopic("pda", "pda.task.events.v1"); got != "pda.task.events.v1" {
		t.Fatalf("qualified topic was rewritten to %q", got)
	}
	if got := ResolveTopic("pda", "task.events.v1"); got != "pda.task.events.v1" {
		t.Fatalf("logical topic resolved to %q", got)
	}
}

func TestConfigRejectsOnlyBlankBrokers(t *testing.T) {
	if err := (Config{Brokers: []string{"", " "}, GroupID: "group", TopicPrefix: "pda"}).Validate(); err == nil {
		t.Fatal("blank broker list must be rejected")
	}
	if got := NormalizeBrokers([]string{" 127.0.0.1:19092 ", ""}); len(got) != 1 || got[0] != "127.0.0.1:19092" {
		t.Fatalf("unexpected normalized brokers: %#v", got)
	}
}
