// Package wmstask implements the operator workflow for a warehouse task that
// WMS dispatched over Kafka.
//
// The boundary this package defends (prompt sections 4.4, 13.1, 14.3):
//
//   - WMS owns the warehouse task and every inventory effect.
//   - pda-backend owns the operator session and the scan evidence.
//   - pda-backend never decides whether stock exists. It reports what the
//     operator confirmed; WMS validates and applies it.
//
// Every state change writes the operator-facing row and the outbound
// integration fact in one transaction, so a confirmed pick can never be lost
// between the device and WMS.
package wmstask

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	EventTaskStarted   = "PDA.TaskStarted.v1"
	EventTaskCompleted = "PDA.TaskCompleted.v1"
	EventTaskFailed    = "PDA.TaskFailed.v1"

	// PDATaskEventTopic is the single ordered stream carrying every PDA task
	// fact. Kafka orders within a partition, not across topics, so one topic
	// partitioned by warehouse_task_id is what keeps a completion behind the
	// start that produced it.
	PDATaskEventTopic = "PDA.TaskEvents.v1"

	sourceService = "pda-backend"
)

// Error codes are returned to the device verbatim so the operator sees a
// deterministic reason rather than a generic failure.
var (
	ErrTaskNotFound     = errors.New("WMS_TASK_NOT_FOUND")
	ErrVersionConflict  = errors.New("WMS_TASK_VERSION_CONFLICT")
	ErrNotStarted       = errors.New("WMS_TASK_NOT_STARTED")
	ErrAlreadyFinished  = errors.New("WMS_TASK_ALREADY_FINISHED")
	ErrScanMismatch     = errors.New("SCAN_MISMATCH")
	ErrScanTypeInvalid  = errors.New("SCAN_TYPE_NOT_REQUIRED")
	ErrScansIncomplete  = errors.New("REQUIRED_SCANS_INCOMPLETE")
	ErrQuantityInvalid  = errors.New("CONFIRM_QUANTITY_INVALID")
	ErrCommandConflict  = errors.New("COMMAND_ID_PAYLOAD_CONFLICT")
	ErrReasonRequired   = errors.New("FAILURE_REASON_REQUIRED")
	ErrOperatorRequired = errors.New("OPERATOR_CONTEXT_REQUIRED")
)

// Actor is the authenticated operator/device context supplied by the gateway.
type Actor struct {
	OperatorID    string
	DeviceID      string
	WarehouseID   string
	CorrelationID string
}

// Task is the device-facing view of a dispatched warehouse task.
type Task struct {
	WarehouseTaskID    string          `json:"warehouseTaskId"`
	PDATaskID          string          `json:"pdaTaskId"`
	TaskType           string          `json:"taskType"`
	TaskVersion        int64           `json:"taskVersion"`
	WMSStatus          string          `json:"wmsStatus"`
	ExecutionState     string          `json:"executionState"`
	Priority           int             `json:"priority"`
	WarehouseID        string          `json:"warehouseId"`
	WorkOrderCode      string          `json:"workOrderCode,omitempty"`
	ItemCode           string          `json:"itemCode,omitempty"`
	RevisionID         string          `json:"revisionId,omitempty"`
	LotID              string          `json:"lotId,omitempty"`
	SourceLocationID   string          `json:"sourceLocationId,omitempty"`
	SourceLocationCode string          `json:"sourceLocationCode,omitempty"`
	SourceBinID        string          `json:"sourceBinId,omitempty"`
	SourceBinCode      string          `json:"sourceBinCode,omitempty"`
	DestinationID      string          `json:"destinationLocationId,omitempty"`
	DestinationCode    string          `json:"destinationLocationCode,omitempty"`
	DestinationBinID   string          `json:"destinationBinId,omitempty"`
	DestinationBinCode string          `json:"destinationBinCode,omitempty"`
	LPNCode            string          `json:"lpnCode,omitempty"`
	Allocations        json.RawMessage `json:"allocations,omitempty"`
	RequestedQty       float64         `json:"requestedQuantity"`
	ConfirmedQty       float64         `json:"confirmedQuantity"`
	RemainingQty       float64         `json:"remainingQuantity"`
	UOMCode            string          `json:"uomCode,omitempty"`
	ScanRequirements   []string        `json:"scanRequirements"`
	CompletedScans     []string        `json:"completedScans"`
	OperatorID         string          `json:"operatorId,omitempty"`
	CorrelationID      string          `json:"correlationId,omitempty"`
}

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

func New(pool *pgxpool.Pool, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{pool: pool, now: now}
}

// AckDelivery records device receipt of an SSE event. It has no warehouse
// side-effect and is idempotent for the event/operator/device tuple.
func (s *Service) AckDelivery(ctx context.Context, eventID string, actor Actor, deviceReceivedAt time.Time) error {
	if strings.TrimSpace(eventID) == "" || actor.OperatorID == "" || actor.DeviceID == "" || actor.WarehouseID == "" {
		return ErrOperatorRequired
	}
	if deviceReceivedAt.IsZero() {
		deviceReceivedAt = s.now().UTC()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	backendRecordedAt := s.now().UTC()
	evidenceEventID := deliveryEvidenceEventID(eventID, actor)
	var ackCount int
	var storedEvidenceEventID *uuid.UUID
	var storedDeviceReceivedAt time.Time
	err = tx.QueryRow(ctx, `
INSERT INTO pda_event_delivery_ack(event_id,operator_id,device_id,warehouse_id,device_received_at,backend_recorded_at,last_correlation_id)
VALUES($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT(event_id,operator_id,device_id) DO UPDATE SET
  last_ack_at=now(), ack_count=pda_event_delivery_ack.ack_count+1,
  last_correlation_id=EXCLUDED.last_correlation_id, warehouse_id=EXCLUDED.warehouse_id
	WHERE pda_event_delivery_ack.device_received_at IS NULL OR pda_event_delivery_ack.device_received_at=EXCLUDED.device_received_at
	RETURNING ack_count,evidence_event_id,device_received_at`, eventID, actor.OperatorID, actor.DeviceID, actor.WarehouseID, deviceReceivedAt.UTC(), backendRecordedAt, actor.CorrelationID).Scan(&ackCount, &storedEvidenceEventID, &storedDeviceReceivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("PDA_ACK_TIMESTAMP_CONFLICT")
	}
	if err != nil {
		return err
	}
	if storedEvidenceEventID == nil {
		storedEvidenceEventID = &evidenceEventID
		if _, err := tx.Exec(ctx, `UPDATE pda_event_delivery_ack SET evidence_event_id=$1 WHERE event_id=$2 AND operator_id=$3 AND device_id=$4`, evidenceEventID, eventID, actor.OperatorID, actor.DeviceID); err != nil {
			return err
		}
	}
	if ackCount == 1 {
		if err := appendDeliveryAckEvent(ctx, tx, eventID, *storedEvidenceEventID, actor, storedDeviceReceivedAt, backendRecordedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func appendDeliveryAckEvent(ctx context.Context, tx pgx.Tx, deliveryEventID string, evidenceEventID uuid.UUID, actor Actor, deviceReceivedAt, backendRecordedAt time.Time) error {
	correlation := actor.CorrelationID
	if correlation == "" {
		correlation = deliveryEventID
	}
	projection, warehouseTaskID, taskVersion, taskType, workOrderID, materialRequestID, sourceEventID, dispatchedAt, mappingError := resolveAckProjection(ctx, tx, deliveryEventID)
	payloadMap := map[string]any{
		"delivery_event_id": deliveryEventID, "operator_id": actor.OperatorID,
		"device_id": actor.DeviceID, "warehouse_id": actor.WarehouseID,
		"device_received_at": deviceReceivedAt, "backend_recorded_at": backendRecordedAt,
		"warehouse_task_id": warehouseTaskID, "task_version": taskVersion, "task_type": taskType,
		"work_order_id": workOrderID, "material_request_id": materialRequestID,
		"source_event_id": sourceEventID, "source_dispatched_at": dispatchedAt,
		"task_projection": projection,
	}
	if mappingError != "" {
		payloadMap["correlation_error"] = mappingError
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		return err
	}
	envelope, err := json.Marshal(map[string]any{
		"event_id": evidenceEventID.String(), "event_type": "PDA.TaskReceivedOnDevice.v1", "event_version": 1,
		"occurred_at": deviceReceivedAt, "source_service": "pda-backend",
		"aggregate_type": "WarehouseTask", "aggregate_id": firstNonEmpty(warehouseTaskID, deliveryEventID),
		"aggregate_version": 1, "correlation_id": correlation, "causation_id": deliveryEventID,
		"schema_version": 1, "metadata": map[string]any{"partition_key": deliveryEventID}, "payload": json.RawMessage(payload),
	})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO integration_outbox(event_id,topic,event_type,aggregate_id,aggregate_version,partition_key,envelope_json) VALUES($1,$2,$3,$4,1,$4,$5) ON CONFLICT(event_id) DO NOTHING`, evidenceEventID, "PDA.TaskReceivedOnDevice.v1", "PDA.TaskReceivedOnDevice.v1", firstNonEmpty(warehouseTaskID, deliveryEventID), string(envelope))
	return err
}

func resolveAckProjection(ctx context.Context, tx pgx.Tx, deliveryEventID string) (map[string]any, string, int64, string, string, string, string, *time.Time, string) {
	var warehouseTaskID, taskType, workOrderID, materialRequestID, sourceEventID string
	var taskVersion int64
	var dispatchedAt time.Time
	var payload []byte
	var projection map[string]any
	err := tx.QueryRow(ctx, `SELECT warehouse_task_id,task_type,task_version,COALESCE(work_order_id,''),COALESCE(material_request_id,''),source_event_id::text,dispatched_at,payload FROM wms_task_projection WHERE source_event_id::text=$1`, deliveryEventID).Scan(&warehouseTaskID, &taskType, &taskVersion, &workOrderID, &materialRequestID, &sourceEventID, &dispatchedAt, &payload)
	if err != nil {
		return map[string]any{}, "", 0, "", "", "", "", nil, "WMS_TASK_PROJECTION_NOT_FOUND"
	}
	if json.Unmarshal(payload, &projection) != nil {
		projection = map[string]any{}
	}
	return projection, warehouseTaskID, taskVersion, taskType, workOrderID, materialRequestID, sourceEventID, &dispatchedAt, ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func deliveryEvidenceEventID(eventID string, actor Actor) uuid.UUID {
	return uuid.NewSHA1(uuid.Nil, []byte("PDA.TaskReceivedOnDevice.v1:"+eventID+":"+actor.OperatorID+":"+actor.DeviceID))
}

// List returns the dispatched tasks visible to an operator in one warehouse.
func (s *Service) List(ctx context.Context, warehouseID, state string) ([]Task, error) {
	rows, err := s.pool.Query(ctx, `
SELECT p.warehouse_task_id, COALESCE(e.pda_task_id::text,''), p.task_type, p.task_version, p.status,
       COALESCE(e.state,'DISPATCHED'), p.priority, p.warehouse_id,
       COALESCE(p.work_order_code,''), COALESCE(p.item_code,''), COALESCE(p.revision_id,''), COALESCE(p.lot_id,''),
       COALESCE(p.source_location_id,''), COALESCE(p.source_location_code,''), COALESCE(p.source_bin_id,''), COALESCE(p.source_bin_code,''),
       COALESCE(p.destination_location_id,''), COALESCE(p.destination_location_code,''), COALESCE(p.destination_bin_id,''), COALESCE(p.destination_bin_code,''), COALESCE(p.lpn_code,''), p.allocations,
       p.requested_quantity, COALESCE(e.confirmed_quantity,0), p.remaining_quantity, COALESCE(p.uom_code,''),
       p.scan_requirements, COALESCE(e.operator_id,''), COALESCE(p.correlation_id,'')
FROM wms_task_projection p
LEFT JOIN pda_task_execution e ON e.warehouse_task_id = p.warehouse_task_id
WHERE ($1='' OR p.warehouse_id=$1 OR p.warehouse_code=$1)
  AND ($2='' OR COALESCE(e.state,'DISPATCHED')=$2)
  AND p.status NOT IN ('CANCELLED','FAILED')
ORDER BY p.priority ASC, p.dispatched_at ASC`, warehouseID, strings.ToUpper(state))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := make([]Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// Get returns one dispatched task with its scan progress.
func (s *Service) Get(ctx context.Context, taskID string) (Task, error) {
	row := s.pool.QueryRow(ctx, `
SELECT p.warehouse_task_id, COALESCE(e.pda_task_id::text,''), p.task_type, p.task_version, p.status,
       COALESCE(e.state,'DISPATCHED'), p.priority, p.warehouse_id,
       COALESCE(p.work_order_code,''), COALESCE(p.item_code,''), COALESCE(p.revision_id,''), COALESCE(p.lot_id,''),
       COALESCE(p.source_location_id,''), COALESCE(p.source_location_code,''), COALESCE(p.source_bin_id,''), COALESCE(p.source_bin_code,''),
       COALESCE(p.destination_location_id,''), COALESCE(p.destination_location_code,''), COALESCE(p.destination_bin_id,''), COALESCE(p.destination_bin_code,''), COALESCE(p.lpn_code,''), p.allocations,
       p.requested_quantity, COALESCE(e.confirmed_quantity,0), p.remaining_quantity, COALESCE(p.uom_code,''),
       p.scan_requirements, COALESCE(e.operator_id,''), COALESCE(p.correlation_id,'')
FROM wms_task_projection p
LEFT JOIN pda_task_execution e ON e.warehouse_task_id = p.warehouse_task_id
WHERE p.warehouse_task_id=$1`, taskID)
	task, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, err
	}
	scans, err := s.acceptedScans(ctx, taskID)
	if err != nil {
		return Task{}, err
	}
	task.CompletedScans = scans
	return task, nil
}

func (s *Service) acceptedScans(ctx context.Context, taskID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT scan_type FROM pda_task_scan WHERE warehouse_task_id=$1 AND accepted ORDER BY scan_type`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		out = append(out, value)
	}
	return out, rows.Err()
}

// Start opens the operator session. It publishes PDA.TaskStarted.v1 so WMS can
// move the warehouse task out of the unclaimed queue.
func (s *Service) Start(ctx context.Context, taskID string, expectedVersion int64, commandID uuid.UUID, actor Actor) (Task, error) {
	if actor.OperatorID == "" || actor.DeviceID == "" {
		return Task{}, ErrOperatorRequired
	}
	hash := requestHash(taskID, "START", expectedVersion, actor.OperatorID, 0, "")
	return s.command(ctx, taskID, "START", commandID, hash, func(tx pgx.Tx, projected projection, execution execution) (string, map[string]any, error) {
		if projected.TaskVersion != expectedVersion {
			return "", nil, ErrVersionConflict
		}
		if execution.State == "COMPLETED" || execution.State == "FAILED" {
			return "", nil, ErrAlreadyFinished
		}
		now := s.now().UTC()
		if _, err := tx.Exec(ctx, `
INSERT INTO pda_task_execution (warehouse_task_id, task_version, state, operator_id, device_id, started_at, updated_at)
VALUES ($1,$2,'STARTED',$3,$4,$5,$5)
ON CONFLICT (warehouse_task_id) DO UPDATE SET
  task_version=EXCLUDED.task_version, state='STARTED', operator_id=EXCLUDED.operator_id,
  device_id=EXCLUDED.device_id, started_at=COALESCE(pda_task_execution.started_at,EXCLUDED.started_at), updated_at=EXCLUDED.updated_at`,
			taskID, projected.TaskVersion, actor.OperatorID, actor.DeviceID, now); err != nil {
			return "", nil, err
		}
		return EventTaskStarted, map[string]any{
			"operator_id": actor.OperatorID,
			"device_id":   actor.DeviceID,
			"started_at":  now.Format(time.RFC3339Nano),
		}, nil
	}, actor)
}

// Scan records one operator scan. A scan that does not match the task detail is
// stored as rejected evidence and the command fails: it must never silently
// advance the task (prompt section 14.5).
func (s *Service) Scan(ctx context.Context, taskID, scanType, scanValue string, expectedVersion int64, commandID uuid.UUID, actor Actor) (Task, error) {
	scanType = strings.ToUpper(strings.TrimSpace(scanType))
	scanValue = strings.TrimSpace(scanValue)
	if scanType == "" || scanValue == "" {
		return Task{}, ErrScanTypeInvalid
	}
	hash := requestHash(taskID, "SCAN", expectedVersion, actor.OperatorID, 0, scanType+"|"+scanValue)
	task, err := s.command(ctx, taskID, "SCAN", commandID, hash, func(tx pgx.Tx, projected projection, execution execution) (string, map[string]any, error) {
		if projected.TaskVersion != expectedVersion {
			return "", nil, ErrVersionConflict
		}
		if execution.State != "STARTED" {
			return "", nil, ErrNotStarted
		}
		if !contains(projected.ScanRequirements, scanType) {
			return "", nil, ErrScanTypeInvalid
		}
		expected, known := projected.expectedScanValue(scanType)
		sum := sha256.Sum256([]byte(scanType + "|" + scanValue))
		hashHex := hex.EncodeToString(sum[:])
		if !known || expected == "" || expected != scanValue {
			// A wrong scan must not advance the task. Returning the error rolls
			// this transaction back; the rejected scan is recorded afterwards by
			// the caller, outside any open transaction, so the audit trail keeps
			// it without a second connection being held here.
			return "", nil, ErrScanMismatch
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO pda_task_scan (warehouse_task_id, scan_type, scan_value, scan_hash, accepted, operator_id, device_id)
VALUES ($1,$2,$3,$4,true,$5,$6)
ON CONFLICT (warehouse_task_id, scan_type, scan_hash) DO NOTHING`,
			taskID, scanType, scanValue, hashHex, actor.OperatorID, actor.DeviceID); err != nil {
			return "", nil, err
		}
		// A scan is local evidence, not a business fact for WMS. No event.
		return "", nil, nil
	}, actor)
	if errors.Is(err, ErrScanMismatch) {
		s.recordRejectedScan(ctx, taskID, scanType, scanValue, actor)
	}
	return task, err
}

// recordRejectedScan preserves a wrong scan as audit evidence. It runs after the
// command transaction has rolled back, so it needs its own statement and must
// never turn a business rejection into an infrastructure error.
func (s *Service) recordRejectedScan(ctx context.Context, taskID, scanType, scanValue string, actor Actor) {
	sum := sha256.Sum256([]byte(scanType + "|" + scanValue))
	if _, err := s.pool.Exec(ctx, `
INSERT INTO pda_task_scan (warehouse_task_id, scan_type, scan_value, scan_hash, accepted, rejection_reason, operator_id, device_id)
VALUES ($1,$2,$3,$4,false,$5,$6,$7)
ON CONFLICT (warehouse_task_id, scan_type, scan_hash) DO NOTHING`,
		taskID, scanType, scanValue, hex.EncodeToString(sum[:]), ErrScanMismatch.Error(), actor.OperatorID, actor.DeviceID); err != nil {
		slog.Warn("rejected scan evidence not recorded", "warehouse_task_id", taskID, "scan_type", scanType, "error", err)
	}
}

// Complete publishes the operator-confirmed result. WMS validates the quantity
// and applies the inventory transaction; pda-backend does not.
func (s *Service) Complete(ctx context.Context, taskID string, confirmedQty float64, expectedVersion int64, commandID uuid.UUID, actor Actor) (Task, error) {
	hash := requestHash(taskID, "COMPLETE", expectedVersion, actor.OperatorID, confirmedQty, "")
	return s.command(ctx, taskID, "COMPLETE", commandID, hash, func(tx pgx.Tx, projected projection, execution execution) (string, map[string]any, error) {
		if projected.TaskVersion != expectedVersion {
			return "", nil, ErrVersionConflict
		}
		if execution.State != "STARTED" {
			return "", nil, ErrNotStarted
		}
		if confirmedQty <= 0 || confirmedQty > projected.RemainingQuantity+1e-6 {
			return "", nil, ErrQuantityInvalid
		}
		accepted, err := acceptedScanTypes(ctx, tx, taskID)
		if err != nil {
			return "", nil, err
		}
		for _, required := range projected.ScanRequirements {
			if !contains(accepted, required) {
				return "", nil, fmt.Errorf("%w:%s", ErrScansIncomplete, required)
			}
		}
		now := s.now().UTC()
		if _, err := tx.Exec(ctx, `UPDATE pda_task_execution SET state='COMPLETED', confirmed_quantity=$2, finished_at=$3, updated_at=$3 WHERE warehouse_task_id=$1`, taskID, confirmedQty, now); err != nil {
			return "", nil, err
		}
		evidence, err := scanEvidence(ctx, tx, taskID)
		if err != nil {
			return "", nil, err
		}
		return EventTaskCompleted, map[string]any{
			"operator_id":        actor.OperatorID,
			"device_id":          actor.DeviceID,
			"confirmed_quantity": confirmedQty,
			"requested_quantity": projected.RequestedQuantity,
			"remaining_quantity": projected.RemainingQuantity - confirmedQty,
			"uom_code":           projected.UOMCode,
			"scan_evidence":      evidence,
			"completed_at":       now.Format(time.RFC3339Nano),
		}, nil
	}, actor)
}

// Fail reports a deterministic operator failure. WMS must not treat this as a
// completion under any circumstance (prompt section 14.5).
func (s *Service) Fail(ctx context.Context, taskID, reasonCode string, expectedVersion int64, commandID uuid.UUID, actor Actor) (Task, error) {
	reasonCode = strings.ToUpper(strings.TrimSpace(reasonCode))
	if reasonCode == "" {
		return Task{}, ErrReasonRequired
	}
	hash := requestHash(taskID, "FAIL", expectedVersion, actor.OperatorID, 0, reasonCode)
	return s.command(ctx, taskID, "FAIL", commandID, hash, func(tx pgx.Tx, projected projection, execution execution) (string, map[string]any, error) {
		if projected.TaskVersion != expectedVersion {
			return "", nil, ErrVersionConflict
		}
		if execution.State == "COMPLETED" || execution.State == "FAILED" {
			return "", nil, ErrAlreadyFinished
		}
		now := s.now().UTC()
		if _, err := tx.Exec(ctx, `
INSERT INTO pda_task_execution (warehouse_task_id, task_version, state, operator_id, device_id, reason_code, finished_at, updated_at)
VALUES ($1,$2,'FAILED',$3,$4,$5,$6,$6)
ON CONFLICT (warehouse_task_id) DO UPDATE SET
  state='FAILED', reason_code=EXCLUDED.reason_code, operator_id=EXCLUDED.operator_id,
  device_id=EXCLUDED.device_id, finished_at=EXCLUDED.finished_at, updated_at=EXCLUDED.updated_at`,
			taskID, projected.TaskVersion, actor.OperatorID, actor.DeviceID, reasonCode, now); err != nil {
			return "", nil, err
		}
		evidence, err := scanEvidence(ctx, tx, taskID)
		if err != nil {
			return "", nil, err
		}
		return EventTaskFailed, map[string]any{
			"operator_id":   actor.OperatorID,
			"device_id":     actor.DeviceID,
			"reason_code":   reasonCode,
			"scan_evidence": evidence,
			"failed_at":     now.Format(time.RFC3339Nano),
		}, nil
	}, actor)
}
