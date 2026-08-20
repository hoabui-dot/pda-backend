package kafka

// I-08 — WMS warehouse task ingestion.
//
// Before this consumer existed, pda-backend subscribed to nothing: kafka.Consumer
// had no production caller and the deployed gateway ran PDA_UPSTREAM_WMS_MODE=mock.
// Tasks therefore never reached the device from the system that owns them.
//
// Responsibilities (prompt section 13.4):
//   - validate schema and contract version, DLQ on failure;
//   - persist the task idempotently, never creating a duplicate;
//   - reject unsupported task types explicitly;
//   - preserve the WMS task identity, version, and correlation IDs.
//
// It is deliberately not an inventory decision point. The projection mirrors
// WMS-owned state; the operator workflow reads it, and every business effect
// goes back to WMS as a fact for WMS to apply.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"
)

// WMSTaskTopics are the cross-system topics carrying warehouse task dispatch.
// They are absolute names: the pda topic prefix must not be applied to a topic
// owned by another system.
var WMSTaskTopics = []string{
	"WMS.PDA.WarehouseTaskCreated.v1",
	"WMS.PDA.WarehouseTaskUpdated.v1",
	"WMS.PDA.WarehouseTaskCancelled.v1",
}

// supportedContractVersion is the highest WMS.PDA contract this build accepts.
// A higher version is routed to the DLQ instead of being partially applied.
const supportedContractVersion = 1

// wmsTaskPayload mirrors the WMS.PDA.WarehouseTask*.v1 payload. Field names and
// types are the contract; a mismatch here is a contract break, not a detail.
type wmsTaskPayload struct {
	WarehouseTaskID string `json:"warehouse_task_id"`
	TaskType        string `json:"task_type"`
	TaskVersion     int64  `json:"task_version"`
	Status          string `json:"status"`
	Priority        int    `json:"priority"`

	SiteID        string `json:"site_id"`
	WarehouseID   string `json:"warehouse_id"`
	WarehouseCode string `json:"warehouse_code"`

	SourceLocationID        string `json:"source_location_id"`
	SourceLocationCode      string `json:"source_location_code"`
	SourceBinID             string `json:"source_bin_id"`
	SourceBinCode           string `json:"source_bin_code"`
	DestinationLocationID   string `json:"destination_location_id"`
	DestinationLocationCode string `json:"destination_location_code"`
	DestinationBinID        string `json:"destination_bin_id"`
	DestinationBinCode      string `json:"destination_bin_code"`

	WorkOrderID           string `json:"work_order_id"`
	WorkOrderCode         string `json:"work_order_code"`
	MaterialRequestID     string `json:"material_request_id"`
	MaterialRequestLineID string `json:"material_request_line_id"`
	ExpectedReceiptID     string `json:"expected_receipt_id"`

	ItemID       string `json:"item_id"`
	ItemCode     string `json:"item_code"`
	RevisionID   string `json:"revision_id"`
	RevisionCode string `json:"revision_code"`

	RequestedQuantity float64 `json:"requested_quantity"`
	ConfirmedQuantity float64 `json:"confirmed_quantity"`
	RemainingQuantity float64 `json:"remaining_quantity"`
	UOMID             string  `json:"uom_id"`
	UOMCode           string  `json:"uom_code"`

	LotID         string   `json:"lot_id"`
	LotCode       string   `json:"lot_code"`
	LPNCode       string   `json:"lpn_code"`
	SerialNumbers []string `json:"serial_numbers"`
	Allocations   []any    `json:"allocations"`

	ScanRequirements []string `json:"scan_requirements"`

	AssignedOperatorID string `json:"assigned_operator_id"`
	AssignedDeviceID   string `json:"assigned_device_id"`

	CorrelationID string `json:"correlation_id"`
	CausationID   string `json:"causation_id"`
}

// taskCategory maps a WMS task type onto the PDA operator module that executes
// it. An unmapped type is rejected explicitly rather than defaulted, so new WMS
// work can never land silently in the wrong operator queue.
func taskCategory(taskType string) (string, bool) {
	switch strings.ToUpper(taskType) {
	case "PICKING", "STAGE_TO_PRODUCTION", "ISSUE_TO_PRODUCTION":
		return "PICKING", true
	case "PUTAWAY":
		return "PUTAWAY", true
	case "RECEIVE_FINISHED_GOODS", "RECEIVE_SEMI_FINISHED", "RETURN_FROM_PRODUCTION":
		return "RECEIVING", true
	case "REPLENISHMENT", "TRANSFER":
		return "REPLENISHMENT", true
	}
	return "", false
}

// queueStatus maps the WMS task lifecycle onto the operator queue status.
func queueStatus(status string) (string, bool) {
	switch strings.ToUpper(status) {
	case "CREATED":
		return "NEW", true
	case "ASSIGNED", "CLAIMED":
		return "ASSIGNED", true
	case "IN_PROGRESS", "PARTIALLY_COMPLETED":
		return "IN_PROGRESS", true
	case "COMPLETED":
		return "COMPLETED", true
	case "CANCELLED", "FAILED":
		return "", true // withdrawn from the operator queue
	}
	return "", false
}

// WMSTaskConsumer subscribes to the WMS dispatch topics and maintains the local
// task projection.
type WMSTaskConsumer struct {
	readers []*kafkago.Reader
	pool    *pgxpool.Pool
	group   string
	cancel  context.CancelFunc
}

func NewWMSTaskConsumer(brokers []string, groupID string, pool *pgxpool.Pool, topics []string) (*WMSTaskConsumer, error) {
	brokers = NormalizeBrokers(brokers)
	if len(brokers) == 0 {
		return nil, fmt.Errorf("at least one Kafka broker is required")
	}
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("consumer group ID is required")
	}
	if pool == nil {
		return nil, fmt.Errorf("database pool is required")
	}
	if len(topics) == 0 {
		topics = WMSTaskTopics
	}
	readers := make([]*kafkago.Reader, 0, len(topics))
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		readers = append(readers, kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:     brokers,
			GroupID:     groupID,
			Topic:       topic,
			StartOffset: kafkago.FirstOffset,
			MinBytes:    1,
			MaxBytes:    10 << 20,
		}))
	}
	if len(readers) == 0 {
		return nil, fmt.Errorf("no valid WMS task topic was configured")
	}
	return &WMSTaskConsumer{readers: readers, pool: pool, group: groupID}, nil
}

func (c *WMSTaskConsumer) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	for _, reader := range c.readers {
		reader := reader
		go func() {
			for {
				message, err := reader.FetchMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					slog.Warn("WMS task read failed; retrying", "topic", reader.Config().Topic, "error", err)
					select {
					case <-ctx.Done():
						return
					case <-time.After(2 * time.Second):
					}
					continue
				}
				if err := c.process(ctx, message); err != nil {
					// A processing failure that reaches here is infrastructural;
					// contract failures are already routed to the DLQ. Leave the
					// offset uncommitted so the delivery is retried.
					slog.Error("WMS task processing failed", "topic", message.Topic, "offset", message.Offset, "error", err)
					continue
				}
				if err := reader.CommitMessages(ctx, message); err != nil {
					slog.Error("WMS task commit failed", "topic", message.Topic, "error", err)
				}
			}
		}()
	}
}

func (c *WMSTaskConsumer) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	for _, reader := range c.readers {
		_ = reader.Close()
	}
}

func (c *WMSTaskConsumer) process(ctx context.Context, message kafkago.Message) error {
	var envelope IntegrationEnvelope
	if err := json.Unmarshal(message.Value, &envelope); err != nil {
		return c.deadLetter(ctx, uuid.Nil, "", "MALFORMED_ENVELOPE", message)
	}
	if err := envelope.Validate(); err != nil {
		return c.deadLetter(ctx, uuid.Nil, envelope.EventType, "INVALID_ENVELOPE:"+err.Error(), message)
	}
	eventID, err := uuid.Parse(envelope.EventID)
	if err != nil {
		return c.deadLetter(ctx, uuid.Nil, envelope.EventType, "EVENT_ID_NOT_UUID", message)
	}
	if version := envelope.ContractVersion(); version == 0 || version > supportedContractVersion {
		return c.deadLetter(ctx, eventID, envelope.EventType, fmt.Sprintf("UNSUPPORTED_EVENT_VERSION:%d", version), message)
	}
	var payload wmsTaskPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return c.deadLetter(ctx, eventID, envelope.EventType, "MALFORMED_PAYLOAD", message)
	}
	if payload.WarehouseTaskID == "" || payload.WarehouseID == "" || payload.TaskVersion < 1 {
		return c.deadLetter(ctx, eventID, envelope.EventType, "MISSING_REQUIRED_IDENTITY", message)
	}
	category, supported := taskCategory(payload.TaskType)
	if !supported {
		return c.deadLetter(ctx, eventID, envelope.EventType, "UNSUPPORTED_TASK_TYPE:"+payload.TaskType, message)
	}
	status, known := queueStatus(payload.Status)
	if !known {
		return c.deadLetter(ctx, eventID, envelope.EventType, "UNSUPPORTED_TASK_STATUS:"+payload.Status, message)
	}

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Durable idempotency: the inbox row and the business effect commit together,
	// so a duplicate delivery cannot produce a second effect (section 20.2).
	claim, err := tx.Exec(ctx, `INSERT INTO event_inbox (event_id, consumer_group) VALUES ($1,$2) ON CONFLICT (event_id, consumer_group) DO NOTHING`, eventID, c.group)
	if err != nil {
		return err
	}
	if claim.RowsAffected() == 0 {
		// Already applied. Commit nothing and let the offset advance.
		return tx.Commit(ctx)
	}

	var projected int64
	err = tx.QueryRow(ctx, `SELECT task_version FROM wms_task_projection WHERE warehouse_task_id=$1 FOR UPDATE`, payload.WarehouseTaskID).Scan(&projected)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		projected = 0
	case err != nil:
		return err
	}
	if payload.TaskVersion <= projected {
		// Out-of-order or replayed delivery. Record it for the UC-13 assertion
		// and apply nothing.
		if _, err := tx.Exec(ctx, `INSERT INTO wms_task_stale_delivery (event_id, warehouse_task_id, incoming_version, projected_version, event_type) VALUES ($1,$2,$3,$4,$5)`, eventID, payload.WarehouseTaskID, payload.TaskVersion, projected, envelope.EventType); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	var sourceDispatchedAt *time.Time
	if envelope.OccurredAt != "" {
		parsed, parseErr := parseTimestamp(envelope.OccurredAt)
		if parseErr != nil {
			return c.deadLetter(ctx, eventID, envelope.EventType, "INVALID_OCCURRED_AT", message)
		}
		sourceDispatchedAt = &parsed
	}
	backendReceivedAt := message.Time.UTC()
	if backendReceivedAt.IsZero() {
		backendReceivedAt = time.Now().UTC()
	}
	if err := upsertProjection(ctx, tx, eventID, payload, envelope.Payload, sourceDispatchedAt, backendReceivedAt); err != nil {
		return err
	}
	if err := syncOperatorQueue(ctx, tx, payload, category, status); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func upsertProjection(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, p wmsTaskPayload, raw json.RawMessage, sourceDispatchedAt *time.Time, backendReceivedAt time.Time) error {
	serials, err := json.Marshal(nonNilStrings(p.SerialNumbers))
	if err != nil {
		return err
	}
	scans, err := json.Marshal(nonNilStrings(p.ScanRequirements))
	if err != nil {
		return err
	}
	allocations, err := json.Marshal(nonNilValues(p.Allocations))
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO wms_task_projection (
  warehouse_task_id, task_type, task_version, status, priority,
  site_id, warehouse_id, warehouse_code, source_location_id, destination_location_id,
  work_order_id, work_order_code, material_request_id, material_request_line_id, expected_receipt_id,
  item_id, item_code, revision_id, revision_code,
  requested_quantity, confirmed_quantity, remaining_quantity, uom_id, uom_code,
  lot_id, lot_code, serial_numbers, scan_requirements,
  correlation_id, causation_id, payload, source_event_id, source_dispatched_at, backend_received_at,
  source_location_code, source_bin_id, source_bin_code, destination_location_code, destination_bin_id, destination_bin_code, lpn_code, allocations, updated_at
) VALUES (
  $1,$2,$3,$4,$5,
  NULLIF($6,''),$7,NULLIF($32,''),NULLIF($8,''),NULLIF($9,''),
  NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),NULLIF($13,''),NULLIF($14,''),
  NULLIF($15,''),NULLIF($16,''),NULLIF($17,''),NULLIF($18,''),
  $19,$20,$21,NULLIF($22,''),NULLIF($23,''),
  NULLIF($24,''),NULLIF($25,''),$26::jsonb,$27::jsonb,
  NULLIF($28,''),NULLIF($29,''),$30::jsonb,$31,$33,$34,
  NULLIF($35,''),NULLIF($36,''),NULLIF($37,''),NULLIF($38,''),NULLIF($39,''),NULLIF($40,''),NULLIF($41,''),$42::jsonb,now()
)
ON CONFLICT (warehouse_task_id) DO UPDATE SET
  task_type=EXCLUDED.task_type, task_version=EXCLUDED.task_version, status=EXCLUDED.status,
  priority=EXCLUDED.priority, site_id=EXCLUDED.site_id, warehouse_id=EXCLUDED.warehouse_id,
  warehouse_code=EXCLUDED.warehouse_code,
  source_location_id=EXCLUDED.source_location_id, destination_location_id=EXCLUDED.destination_location_id,
  work_order_id=EXCLUDED.work_order_id, work_order_code=EXCLUDED.work_order_code,
  material_request_id=EXCLUDED.material_request_id, material_request_line_id=EXCLUDED.material_request_line_id,
  expected_receipt_id=EXCLUDED.expected_receipt_id,
  item_id=EXCLUDED.item_id, item_code=EXCLUDED.item_code, revision_id=EXCLUDED.revision_id, revision_code=EXCLUDED.revision_code,
  requested_quantity=EXCLUDED.requested_quantity, confirmed_quantity=EXCLUDED.confirmed_quantity,
  remaining_quantity=EXCLUDED.remaining_quantity, uom_id=EXCLUDED.uom_id, uom_code=EXCLUDED.uom_code,
  lot_id=EXCLUDED.lot_id, lot_code=EXCLUDED.lot_code, serial_numbers=EXCLUDED.serial_numbers,
  scan_requirements=EXCLUDED.scan_requirements,
  correlation_id=EXCLUDED.correlation_id, causation_id=EXCLUDED.causation_id,
  payload=EXCLUDED.payload, source_event_id=EXCLUDED.source_event_id,
  source_dispatched_at=COALESCE(EXCLUDED.source_dispatched_at,wms_task_projection.source_dispatched_at),
  backend_received_at=LEAST(wms_task_projection.backend_received_at,EXCLUDED.backend_received_at),
  source_location_code=EXCLUDED.source_location_code, source_bin_id=EXCLUDED.source_bin_id, source_bin_code=EXCLUDED.source_bin_code,
  destination_location_code=EXCLUDED.destination_location_code, destination_bin_id=EXCLUDED.destination_bin_id, destination_bin_code=EXCLUDED.destination_bin_code,
  lpn_code=EXCLUDED.lpn_code, allocations=EXCLUDED.allocations, updated_at=now()`,
		p.WarehouseTaskID, strings.ToUpper(p.TaskType), p.TaskVersion, strings.ToUpper(p.Status), priorityOrDefault(p.Priority),
		p.SiteID, p.WarehouseID, p.SourceLocationID, p.DestinationLocationID,
		p.WorkOrderID, p.WorkOrderCode, p.MaterialRequestID, p.MaterialRequestLineID, p.ExpectedReceiptID,
		p.ItemID, p.ItemCode, p.RevisionID, p.RevisionCode,
		p.RequestedQuantity, p.ConfirmedQuantity, p.RemainingQuantity, p.UOMID, p.UOMCode,
		p.LotID, p.LotCode, string(serials), string(scans),
		p.CorrelationID, p.CausationID, string(raw), eventID, p.WarehouseCode, sourceDispatchedAt, backendReceivedAt,
		p.SourceLocationCode, p.SourceBinID, p.SourceBinCode, p.DestinationLocationCode, p.DestinationBinID, p.DestinationBinCode, p.LPNCode, string(allocations))
	return err
}

// syncOperatorQueue keeps the operator-facing warehouse_task queue consistent
// with the projection. A cancelled or failed task is withdrawn from the queue so
// no operator can start work WMS has already closed.
func syncOperatorQueue(ctx context.Context, tx pgx.Tx, p wmsTaskPayload, category, status string) error {
	if status == "" {
		_, err := tx.Exec(ctx, `DELETE FROM warehouse_task WHERE task_id=$1`, p.WarehouseTaskID)
		return err
	}
	var operator any
	if p.AssignedOperatorID != "" {
		operator = p.AssignedOperatorID
	}
	_, err := tx.Exec(ctx, `
INSERT INTO warehouse_task (task_id, category, status, priority, warehouse_id, operator_id, version, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,now())
ON CONFLICT (task_id) DO UPDATE SET
  category=EXCLUDED.category, status=EXCLUDED.status, priority=EXCLUDED.priority,
  warehouse_id=EXCLUDED.warehouse_id,
  operator_id=COALESCE(EXCLUDED.operator_id, warehouse_task.operator_id),
  version=EXCLUDED.version, updated_at=now()`,
		p.WarehouseTaskID, category, status, priorityOrDefault(p.Priority), p.WarehouseID, operator, p.TaskVersion)
	return err
}

// deadLetter records a permanently rejected delivery. It returns nil so the
// offset is committed: retrying a schema or contract failure forever would only
// stall the partition (section 20.3, 20.4).
func (c *WMSTaskConsumer) deadLetter(ctx context.Context, eventID uuid.UUID, eventType, failure string, message kafkago.Message) error {
	id := eventID
	if id == uuid.Nil {
		// The DLQ is keyed by event ID; a message we could not identify still
		// needs a stable, unique key. Derive it from the topic coordinates.
		id = uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s|%d|%d", message.Topic, message.Partition, message.Offset)))
	}
	envelope, err := json.Marshal(map[string]any{
		"raw_payload": string(message.Value),
		"topic":       message.Topic,
		"partition":   message.Partition,
		"offset":      message.Offset,
		"event_type":  eventType,
	})
	if err != nil {
		return err
	}
	slog.Warn("WMS task delivery rejected to DLQ", "failure", failure, "topic", message.Topic, "offset", message.Offset)
	_, err = c.pool.Exec(ctx, `INSERT INTO event_dlq (event_id, consumer_group, envelope_json, attempts, last_error) VALUES ($1,$2,$3,1,$4) ON CONFLICT (event_id, consumer_group) DO UPDATE SET attempts=event_dlq.attempts+1, last_error=EXCLUDED.last_error, failed_at=now()`, id, c.group, string(envelope), failure)
	return err
}

func priorityOrDefault(priority int) int {
	if priority <= 0 || priority > 100 {
		return 5
	}
	return priority
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilValues(values []any) []any {
	if values == nil {
		return []any{}
	}
	return values
}
