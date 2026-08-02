package kafka

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
)

type testInbox struct{ seen map[uuid.UUID]bool }

func testBrokers() []string {
	value := os.Getenv("PDA_KAFKA_BROKERS")
	if value == "" {
		value = "127.0.0.1:19092"
	}
	return strings.Split(value, ",")
}

func ensureTestTopics(t *testing.T, topics ...string) {
	t.Helper()
	conn, err := kafkago.Dial("tcp", testBrokers()[0])
	if err != nil {
		t.Skipf("Kafka broker unavailable: %v", err)
	}
	defer conn.Close()
	configs := make([]kafkago.TopicConfig, 0, len(topics))
	for _, topic := range topics {
		configs = append(configs, kafkago.TopicConfig{Topic: topic, NumPartitions: 3, ReplicationFactor: 1})
	}
	if err := conn.CreateTopics(configs...); err != nil {
		t.Fatalf("create Kafka test topics: %v", err)
	}
}

func (s *testInbox) AlreadyProcessed(_ context.Context, id uuid.UUID) (bool, error) {
	return s.seen[id], nil
}
func (s *testInbox) MarkProcessed(_ context.Context, id uuid.UUID, _ time.Time) error {
	s.seen[id] = true
	return nil
}
func (s *testInbox) MoveToDLQ(context.Context, event.DomainEventEnvelope, string, int) error {
	return nil
}

func TestPublisherRoundTrip(t *testing.T) {
	brokers := testBrokers()
	if conn, err := net.DialTimeout("tcp", brokers[0], time.Second); err != nil {
		t.Skip("Kafka broker unavailable")
	} else {
		_ = conn.Close()
	}
	topic := "pda.task.events.v1"
	ensureTestTopics(t, topic)
	p, err := NewPublisher(Config{Brokers: brokers, GroupID: "pda-be08-producer-test", TopicPrefix: "pda"})
	if err != nil {
		t.Fatal(err)
	}
	e := event.DomainEventEnvelope{EventID: uuid.New(), CausationID: uuid.New(), EventVersion: 1, AggregateVersion: 1, OccurredAt: time.Now().UTC(), EventType: "BE08.Test", AggregateType: "task", AggregateID: "TASK-1", CorrelationID: "corr", WarehouseID: "WH-1", OperatorID: "OP-1", DeviceID: "DEV-1", Topic: topic, Payload: []byte(`{"ok":true}`)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Publish(ctx, e); err != nil {
		t.Fatal(err)
	}
	r := kafkago.NewReader(kafkago.ReaderConfig{Brokers: brokers, Topic: topic, GroupID: "pda-be08-reader-" + uuid.NewString(), MinBytes: 1, MaxBytes: 1 << 20})
	defer r.Close()
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(m.Value), e.EventID.String()) {
			continue
		}
		if string(m.Key) != e.AggregateID {
			t.Fatalf("unexpected Kafka message key %q", string(m.Key))
		}
		break
	}
}

func TestConsumerGroupProcessesAndMarksInbox(t *testing.T) {
	brokers := testBrokers()
	if conn, err := net.DialTimeout("tcp", brokers[0], time.Second); err != nil {
		t.Skip("Kafka broker unavailable")
	} else {
		_ = conn.Close()
	}
	topic := "pda.task.events.v1"
	ensureTestTopics(t, topic)
	p, err := NewPublisher(Config{Brokers: brokers, GroupID: "pda-be08-consumer-producer-test", TopicPrefix: "pda"})
	if err != nil {
		t.Fatal(err)
	}
	e := event.DomainEventEnvelope{EventID: uuid.New(), CausationID: uuid.New(), EventVersion: 1, AggregateVersion: 1, OccurredAt: time.Now().UTC(), EventType: "BE08.Consumer", AggregateType: "task", AggregateID: "TASK-C", CorrelationID: "corr", WarehouseID: "WH-1", OperatorID: "OP-1", DeviceID: "DEV-1", Topic: topic, Payload: []byte(`{"ok":true}`)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Publish(ctx, e); err != nil {
		t.Fatal(err)
	}
	store := &testInbox{seen: map[uuid.UUID]bool{}}
	processed := make(chan struct{}, 1)
	c, err := NewConsumer(Config{Brokers: brokers, GroupID: "pda-be08-consumer-" + uuid.NewString(), TopicPrefix: "pda"}, topic, store, func(_ context.Context, got event.DomainEventEnvelope) error {
		if got.EventID == e.EventID {
			processed <- struct{}{}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	go func() { _ = c.Run(runCtx) }()
	select {
	case <-processed:
	case <-ctx.Done():
		t.Fatal("consumer did not process message")
	}
	if !store.seen[e.EventID] {
		t.Fatal("consumer did not mark inbox")
	}
	if err := p.Publish(ctx, e); err != nil {
		t.Fatal(err)
	}
	select {
	case <-processed:
		t.Fatal("duplicate event was processed twice")
	case <-time.After(250 * time.Millisecond):
	}
}

func TestPublisherFailsClosedDuringBrokerOutage(t *testing.T) {
	p, err := NewPublisher(Config{Brokers: []string{"127.0.0.1:19999"}, GroupID: "be08-outage", TopicPrefix: "pda"})
	if err != nil {
		t.Fatal(err)
	}
	e := event.DomainEventEnvelope{EventID: uuid.New(), CausationID: uuid.New(), EventVersion: 1, AggregateVersion: 1, OccurredAt: time.Now().UTC(), EventType: "BE08.Outage", AggregateType: "task", AggregateID: "TASK-O", CorrelationID: "corr", WarehouseID: "WH-1", OperatorID: "OP-1", DeviceID: "DEV-1", Topic: "outage", Payload: []byte(`{"ok":true}`)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Publish(ctx, e); err == nil {
		t.Fatal("publisher must not acknowledge an unavailable broker")
	}
}
