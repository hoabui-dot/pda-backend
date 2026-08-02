package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/company/pda-backend/internal/platform/messaging"
	kafkago "github.com/segmentio/kafka-go"
)

type DLQStore interface {
	messaging.InboxRepository
	MoveToDLQ(context.Context, event.DomainEventEnvelope, string, int) error
}

type Consumer struct {
	reader      *kafkago.Reader
	store       DLQStore
	process     func(context.Context, event.DomainEventEnvelope) error
	maxAttempts int
	Metrics     *messaging.Metrics
}

func NewConsumer(cfg Config, topic string, store DLQStore, process func(context.Context, event.DomainEventEnvelope) error) (*Consumer, error) {
	cfg.Brokers = NormalizeBrokers(cfg.Brokers)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.SecurityProtocol == "" {
		cfg.SecurityProtocol = "PLAINTEXT"
	}
	if cfg.SecurityProtocol != "PLAINTEXT" {
		return nil, ErrSecurityUnsupported
	}
	if topic == "" || store == nil || process == nil {
		return nil, fmt.Errorf("consumer topic, store, and handler are required")
	}
	return &Consumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{Brokers: cfg.Brokers, GroupID: cfg.GroupID, Topic: ResolveTopic(cfg.TopicPrefix, topic), MinBytes: 1, MaxBytes: 10 << 20, CommitInterval: 0}),
		store:  store, process: process, maxAttempts: 3, Metrics: &messaging.Metrics{},
	}, nil
}

func (c *Consumer) Close() error { return c.reader.Close() }

func (c *Consumer) Run(ctx context.Context) error {
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		if c.Metrics != nil {
			c.Metrics.Backlog.Add(1)
			if !message.Time.IsZero() {
				c.Metrics.LagSeconds.Store(int64(time.Since(message.Time).Seconds()))
			}
		}
		var envelope event.DomainEventEnvelope
		if err := json.Unmarshal(message.Value, &envelope); err != nil {
			if err := c.reader.CommitMessages(ctx, message); err != nil {
				return err
			}
			if c.Metrics != nil {
				c.Metrics.Backlog.Add(-1)
			}
			continue
		}
		processed, err := c.store.AlreadyProcessed(ctx, envelope.EventID)
		if err != nil {
			return err
		}
		if processed {
			if err := c.reader.CommitMessages(ctx, message); err != nil {
				return err
			}
			if c.Metrics != nil {
				c.Metrics.Backlog.Add(-1)
			}
			continue
		}
		var processErr error
		for attempt := 1; attempt <= c.maxAttempts; attempt++ {
			processErr = c.process(ctx, envelope)
			if processErr == nil {
				break
			}
			if attempt < c.maxAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(attempt) * 25 * time.Millisecond):
				}
			}
		}
		if processErr != nil {
			if c.Metrics != nil {
				c.Metrics.Failed.Add(1)
			}
			if err := c.store.MoveToDLQ(ctx, envelope, processErr.Error(), c.maxAttempts); err != nil {
				return err
			}
		}
		if c.Metrics != nil {
			c.Metrics.Published.Add(1)
		}
		if err := c.store.MarkProcessed(ctx, envelope.EventID, time.Now().UTC()); err != nil {
			return err
		}
		if err := c.reader.CommitMessages(ctx, message); err != nil {
			return err
		}
		if c.Metrics != nil {
			c.Metrics.Backlog.Add(-1)
		}
	}
}
