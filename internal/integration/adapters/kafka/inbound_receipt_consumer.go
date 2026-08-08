package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"
)

const InboundReceiptConfirmedTopic = "WMS.Inbound.ReceiptConfirmed.v1"

type inboundReceiptPayload struct {
	ReceiptID           string `json:"receipt_id"`
	Status              string `json:"status"`
	WarehouseLocationID string `json:"warehouse_location_id"`
}

// InboundReceiptConsumer projects the authoritative Inbound result into the
// PDA read model. It never calls Inventory or mutates an owner aggregate.
type InboundReceiptConsumer struct {
	reader *kafkago.Reader
	pool   *pgxpool.Pool
	group  string
}

func NewInboundReceiptConsumer(brokers []string, group string, pool *pgxpool.Pool, topic string) (*InboundReceiptConsumer, error) {
	brokers = NormalizeBrokers(brokers)
	if len(brokers) == 0 || pool == nil || strings.TrimSpace(group) == "" {
		return nil, fmt.Errorf("inbound receipt consumer requires broker, group, and database")
	}
	if strings.TrimSpace(topic) == "" {
		topic = InboundReceiptConfirmedTopic
	}
	return &InboundReceiptConsumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers: brokers, GroupID: group, Topic: topic,
			MinBytes: 1, MaxBytes: 10 << 20, CommitInterval: 0,
		}),
		pool: pool, group: group,
	}, nil
}

func (c *InboundReceiptConsumer) Close() error { return c.reader.Close() }

func (c *InboundReceiptConsumer) Run(ctx context.Context) error {
	for {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			return err
		}
		if err := c.process(ctx, message.Value); err != nil {
			slog.Error("inbound receipt projection failed", "topic", message.Topic, "offset", message.Offset, "error", err)
			continue
		}
		if err := c.reader.CommitMessages(ctx, message); err != nil {
			return err
		}
	}
}

func (c *InboundReceiptConsumer) process(ctx context.Context, raw []byte) error {
	var envelope IntegrationEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		return err
	}
	if envelope.EventType != InboundReceiptConfirmedTopic || envelope.ContractVersion() != 1 {
		return fmt.Errorf("unsupported inbound receipt event %q", envelope.EventType)
	}
	eventID, err := parseUUID(envelope.EventID)
	if err != nil {
		return err
	}
	var payload inboundReceiptPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return fmt.Errorf("decode receipt payload: %w", err)
	}
	if payload.ReceiptID == "" || payload.WarehouseLocationID == "" || payload.Status == "" {
		return fmt.Errorf("receipt confirmation payload is missing identity")
	}

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	claim, err := tx.Exec(ctx, `INSERT INTO event_inbox(event_id,consumer_group,processed_at) VALUES($1,$2,$3) ON CONFLICT(event_id,consumer_group) DO NOTHING`, eventID, c.group, time.Now().UTC())
	if err != nil {
		return err
	}
	if claim.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO receiving_work_projection
    (receipt_id, warehouse_location_id, status, projection_version, source_event_id, correlation_id, trace_id, payload, updated_at)
VALUES ($1,$2,$3,1,$4,$5,$6,$7,now())
ON CONFLICT (receipt_id) DO UPDATE SET
    warehouse_location_id=EXCLUDED.warehouse_location_id,
    status=EXCLUDED.status,
    projection_version=receiving_work_projection.projection_version+1,
    source_event_id=EXCLUDED.source_event_id,
    correlation_id=EXCLUDED.correlation_id,
    trace_id=EXCLUDED.trace_id,
    payload=EXCLUDED.payload,
    updated_at=now()`, payload.ReceiptID, payload.WarehouseLocationID, payload.Status, eventID, envelope.CorrelationID, envelope.TraceID, envelope.Payload)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func parseUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid event_id %q: %w", value, err)
	}
	return id, nil
}
