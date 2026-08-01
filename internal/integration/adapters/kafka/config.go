package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/company/pda-backend/internal/platform/messaging"
	kafkago "github.com/segmentio/kafka-go"
)

var ErrBrokerUnavailable = errors.New("kafka broker is unavailable; no delivery was acknowledged")
var ErrSecurityUnsupported = errors.New("requested Kafka security protocol is not configured")

type Config struct {
	Brokers          []string
	GroupID          string
	TopicPrefix      string
	SecurityProtocol string
}

func (c Config) Validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("at least one Kafka broker is required")
	}
	if strings.TrimSpace(c.GroupID) == "" || strings.TrimSpace(c.TopicPrefix) == "" {
		return fmt.Errorf("Kafka group ID and topic prefix are required")
	}
	return nil
}

// Publisher is intentionally fail-closed until a verified Kafka client is selected.
// It never reports success or silently falls back to the mock publisher.
type Publisher struct {
	Config  Config
	writer  *kafkago.Writer
	Metrics *messaging.Metrics
}

func NewPublisher(cfg Config) (*Publisher, error) {
	if cfg.SecurityProtocol == "" {
		cfg.SecurityProtocol = "PLAINTEXT"
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.SecurityProtocol != "PLAINTEXT" {
		return nil, ErrSecurityUnsupported
	}
	return &Publisher{Config: cfg, Metrics: &messaging.Metrics{}, writer: &kafkago.Writer{
		Addr: kafkago.TCP(cfg.Brokers...), Topic: "", Balancer: &kafkago.Hash{},
		RequiredAcks: kafkago.RequireAll, MaxAttempts: 3, BatchTimeout: 20 * time.Millisecond,
	}}, nil
}

func (p *Publisher) Publish(ctx context.Context, envelope event.DomainEventEnvelope) error {
	if p == nil || p.writer == nil {
		return ErrBrokerUnavailable
	}
	if err := envelope.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	topic := envelope.Topic
	if p.Config.TopicPrefix != "" {
		topic = p.Config.TopicPrefix + "." + topic
	}
	err = p.writer.WriteMessages(ctx, kafkago.Message{Topic: topic, Key: []byte(envelope.AggregateID), Value: data, Time: envelope.OccurredAt})
	if p.Metrics != nil {
		if err != nil {
			p.Metrics.Failed.Add(1)
		} else {
			p.Metrics.Published.Add(1)
		}
	}
	return err
}
