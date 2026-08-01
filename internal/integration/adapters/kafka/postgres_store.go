package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/company/pda-backend/internal/platform/event"
	"github.com/company/pda-backend/internal/platform/messaging"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct{ pool *pgxpool.Pool }

const defaultConsumerGroup = "pda-backend"

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }

func (s *PostgresStore) Append(ctx context.Context, record messaging.OutboxRecord) error {
	b, err := json.Marshal(record.Envelope)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO domain_outbox(event_id,aggregate_id,aggregate_version,event_type,envelope_json,created_at,published_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(event_id) DO NOTHING`, record.ID, record.Envelope.AggregateID, record.Envelope.AggregateVersion, record.Envelope.EventType, b, record.CreatedAt, record.PublishedAt)
	return err
}

func (s *PostgresStore) ClaimPending(ctx context.Context, limit int) ([]messaging.OutboxRecord, error) {
	if limit < 1 {
		return nil, fmt.Errorf("claim limit must be positive")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `WITH claimed AS (SELECT event_id FROM domain_outbox WHERE published_at IS NULL AND available_at <= now() ORDER BY created_at,event_id FOR UPDATE SKIP LOCKED LIMIT $1) UPDATE domain_outbox d SET attempts=d.attempts+1 FROM claimed c WHERE d.event_id=c.event_id RETURNING d.event_id,d.envelope_json,d.created_at,d.published_at`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []messaging.OutboxRecord
	for rows.Next() {
		var id uuid.UUID
		var raw []byte
		var created time.Time
		var published *time.Time
		if err := rows.Scan(&id, &raw, &created, &published); err != nil {
			return nil, err
		}
		var envelope event.DomainEventEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		records = append(records, messaging.OutboxRecord{ID: id, Envelope: envelope, CreatedAt: created, PublishedAt: published})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *PostgresStore) MarkPublished(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE domain_outbox SET published_at=$2,last_error=NULL WHERE event_id=$1`, id, at)
	return err
}

func (s *PostgresStore) MarkFailed(ctx context.Context, id uuid.UUID, cause error, next time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE domain_outbox SET available_at=$2,last_error=$3 WHERE event_id=$1 AND published_at IS NULL`, id, next, cause.Error())
	return err
}

func (s *PostgresStore) AlreadyProcessed(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM event_inbox WHERE event_id=$1 AND consumer_group=$2)`, id, defaultConsumerGroup).Scan(&exists)
	return exists, err
}

func (s *PostgresStore) MarkProcessed(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO event_inbox(event_id,consumer_group,processed_at) VALUES($1,$2,$3) ON CONFLICT(event_id,consumer_group) DO NOTHING`, id, defaultConsumerGroup, at)
	return err
}

func (s *PostgresStore) MoveToDLQ(ctx context.Context, envelope event.DomainEventEnvelope, cause string, attempts int) error {
	b, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO event_dlq(event_id,consumer_group,envelope_json,attempts,last_error) VALUES($1,$2,$3,$4,$5) ON CONFLICT(event_id,consumer_group) DO UPDATE SET attempts=EXCLUDED.attempts,last_error=EXCLUDED.last_error,failed_at=now()`, envelope.EventID, defaultConsumerGroup, b, attempts, cause)
	return err
}

var _ messaging.OutboxRepository = (*PostgresStore)(nil)
var _ messaging.InboxRepository = (*PostgresStore)(nil)
var _ DLQStore = (*PostgresStore)(nil)
