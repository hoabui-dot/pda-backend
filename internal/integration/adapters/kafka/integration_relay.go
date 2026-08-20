package kafka

// Relay for the cross-system outbox.
//
// domain_outbox carries pda-backend's internal envelope on prefixed pda.*
// topics. integration_outbox carries the canonical snake_case envelope on
// absolute topics defined by the cross-system contract, so it needs its own
// relay that publishes the stored bytes verbatim and never applies the topic
// prefix.

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"
)

// maxIntegrationAttempts bounds retry. A message that still fails after this
// many attempts is a permanent defect, not a transient one, and is moved to the
// dead-letter table so the partition keeps draining (prompt section 20.3).
const maxIntegrationAttempts = 8

type IntegrationRelay struct {
	pool      *pgxpool.Pool
	writer    *kafkago.Writer
	batchSize int
	cancel    context.CancelFunc
}

func NewIntegrationRelay(brokers []string, pool *pgxpool.Pool) (*IntegrationRelay, error) {
	brokers = NormalizeBrokers(brokers)
	if len(brokers) == 0 || pool == nil {
		return nil, ErrBrokerUnavailable
	}
	return &IntegrationRelay{
		pool:      pool,
		batchSize: 100,
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Balancer:     &kafkago.Hash{},
			RequiredAcks: kafkago.RequireAll,
			MaxAttempts:  3,
			BatchTimeout: 20 * time.Millisecond,
		},
	}, nil
}

func (r *IntegrationRelay) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if published, err := r.RunOnce(ctx); err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Warn("integration outbox cycle failed", "error", err)
			} else if published > 0 {
				slog.Info("integration outbox published", "count", published)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (r *IntegrationRelay) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	_ = r.writer.Close()
}

// RunOnce claims and publishes one batch. SKIP LOCKED makes the relay safe to
// run with several replicas: two workers never claim the same row.
func (r *IntegrationRelay) RunOnce(ctx context.Context) (int, error) {
	rows, err := r.pool.Query(ctx, `
WITH claimed AS (
  SELECT event_id FROM integration_outbox
  WHERE published_at IS NULL AND available_at <= now()
  ORDER BY created_at, event_id
  FOR UPDATE SKIP LOCKED LIMIT $1
)
UPDATE integration_outbox o SET attempts = o.attempts + 1
FROM claimed c WHERE o.event_id = c.event_id
RETURNING o.event_id, o.topic, o.event_type, o.partition_key, o.envelope_json, o.attempts`, r.batchSize)
	if err != nil {
		return 0, err
	}
	type pending struct {
		id        uuid.UUID
		topic     string
		eventType string
		key       string
		envelope  []byte
		attempts  int
	}
	batch := make([]pending, 0, r.batchSize)
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.id, &item.topic, &item.eventType, &item.key, &item.envelope, &item.attempts); err != nil {
			rows.Close()
			return 0, err
		}
		batch = append(batch, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	published := 0
	for _, item := range batch {
		envelope := withPublishedAt(item.envelope, time.Now().UTC())
		writeErr := r.writer.WriteMessages(ctx, kafkago.Message{
			Topic: item.topic,
			Key:   []byte(item.key),
			Value: envelope,
		})
		if writeErr == nil {
			if _, err := r.pool.Exec(ctx, `UPDATE integration_outbox SET published_at=now(), last_error=NULL WHERE event_id=$1`, item.id); err != nil {
				return published, err
			}
			published++
			continue
		}
		if item.attempts >= maxIntegrationAttempts {
			if _, err := r.pool.Exec(ctx, `INSERT INTO integration_outbox_dead_letters (event_id, topic, event_type, envelope_json, attempts, last_error) VALUES ($1,$2,$3,$4,$5,$6)`, item.id, item.topic, item.eventType, string(item.envelope), item.attempts, writeErr.Error()); err != nil {
				return published, err
			}
			// Retiring the row stops the partition stalling on a permanent defect;
			// the dead-letter row preserves it for the documented replay path.
			if _, err := r.pool.Exec(ctx, `UPDATE integration_outbox SET published_at=now(), last_error=$2 WHERE event_id=$1`, item.id, "RETRY_EXHAUSTED:"+writeErr.Error()); err != nil {
				return published, err
			}
			slog.Error("integration outbox retry exhausted", "event_id", item.id, "topic", item.topic, "error", writeErr)
			continue
		}
		// Exponential backoff, capped, so a broker outage does not spin.
		backoff := time.Duration(1<<uint(item.attempts)) * time.Second
		if backoff > 5*time.Minute {
			backoff = 5 * time.Minute
		}
		if _, err := r.pool.Exec(ctx, `UPDATE integration_outbox SET available_at=now()+$2::interval, last_error=$3 WHERE event_id=$1`, item.id, backoff.String(), writeErr.Error()); err != nil {
			return published, err
		}
	}
	return published, nil
}

// withPublishedAt adds the publication boundary at the relay, where the
// message is actually handed to Kafka. Older producers may omit this field;
// the relay must make the boundary explicit without changing an existing
// producer timestamp.
func withPublishedAt(raw []byte, publishedAt time.Time) []byte {
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return raw
	}
	if value, ok := envelope["published_at"].(string); ok && value != "" {
		return raw
	}
	envelope["published_at"] = publishedAt.UTC().Format(time.RFC3339Nano)
	updated, err := json.Marshal(envelope)
	if err != nil {
		return raw
	}
	return updated
}

// PendingDepth reports the observability figures required by prompt section 26.
func (r *IntegrationRelay) PendingDepth(ctx context.Context) (count int, oldestAgeSeconds float64, err error) {
	err = r.pool.QueryRow(ctx, `SELECT count(*), COALESCE(EXTRACT(EPOCH FROM (now() - min(created_at))),0) FROM integration_outbox WHERE published_at IS NULL`).Scan(&count, &oldestAgeSeconds)
	return
}
