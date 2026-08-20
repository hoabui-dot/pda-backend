package wmstask

// Command orchestration, idempotency, and the outbound integration fact.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// projection is the WMS-owned task state this service is allowed to read.
type projection struct {
	TaskType          string
	TaskVersion       int64
	Status            string
	WarehouseID       string
	SourceLocationID  string
	DestinationID     string
	RevisionID        string
	ItemCode          string
	LotID             string
	UOMCode           string
	RequestedQuantity float64
	RemainingQuantity float64
	ScanRequirements  []string
	CorrelationID     string
	WorkOrderID       string
	MaterialRequestID string
	SiteID            string
}

// expectedScanValue resolves what a given scan type must match. The values come
// from the WMS dispatch payload, never from device input.
func (p projection) expectedScanValue(scanType string) (string, bool) {
	switch scanType {
	case "SOURCE":
		return p.SourceLocationID, true
	case "DESTINATION":
		return p.DestinationID, true
	case "ITEM":
		return p.RevisionID, true
	case "LOT":
		return p.LotID, true
	}
	return "", false
}

type execution struct {
	PDATaskID string
	State     string
}

// mutate applies the state change and returns the integration fact to publish.
// An empty event type means the change is local to pda-backend.
type mutate func(pgx.Tx, projection, execution) (string, map[string]any, error)

// command is the shared envelope for every operator action: idempotency check,
// row locking, state change, and the outbound fact — all in one transaction.
func (s *Service) command(ctx context.Context, taskID, commandType string, commandID uuid.UUID, hash string, apply mutate, actor Actor) (Task, error) {
	if commandID == uuid.Nil {
		commandID = uuid.New()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback(ctx)

	// A retried command with the same ID replays the stored result rather than
	// producing a second effect. A different payload under the same ID is a
	// client bug and is rejected loudly.
	var storedHash string
	var storedResult []byte
	err = tx.QueryRow(ctx, `SELECT request_hash, result_json FROM pda_task_command WHERE command_id=$1 FOR UPDATE`, commandID).Scan(&storedHash, &storedResult)
	switch {
	case err == nil:
		if storedHash != hash {
			return Task{}, ErrCommandConflict
		}
		var replay Task
		if json.Unmarshal(storedResult, &replay) == nil {
			return replay, nil
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return Task{}, err
	}

	projected, err := loadProjection(ctx, tx, taskID)
	if err != nil {
		return Task{}, err
	}
	current, err := loadExecution(ctx, tx, taskID)
	if err != nil {
		return Task{}, err
	}

	eventType, payload, err := apply(tx, projected, current)
	if err != nil {
		return Task{}, err
	}

	if eventType != "" {
		if err := s.appendIntegrationEvent(ctx, tx, eventType, taskID, projected, payload, actor); err != nil {
			return Task{}, err
		}
	}

	result, err := readTask(ctx, tx, taskID)
	if err != nil {
		return Task{}, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return Task{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO pda_task_command (command_id, warehouse_task_id, command_type, request_hash, result_json) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (command_id) DO NOTHING`, commandID, taskID, commandType, hash, encoded); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Task{}, err
	}
	return result, nil
}

// appendIntegrationEvent writes the canonical snake_case fact to the
// cross-system outbox in the caller's transaction (prompt section 4.2).
func (s *Service) appendIntegrationEvent(ctx context.Context, tx pgx.Tx, eventType, taskID string, p projection, payload map[string]any, actor Actor) error {
	pdaTaskID, err := ensurePDATaskID(ctx, tx, taskID)
	if err != nil {
		return err
	}
	// Identities every PDA fact must reference (prompt section 14.6).
	payload["pda_task_id"] = pdaTaskID
	payload["warehouse_task_id"] = taskID
	payload["task_version"] = p.TaskVersion
	payload["task_type"] = p.TaskType
	payload["warehouse_id"] = p.WarehouseID
	if p.WorkOrderID != "" {
		payload["work_order_id"] = p.WorkOrderID
	}
	if p.MaterialRequestID != "" {
		payload["material_request_id"] = p.MaterialRequestID
	}
	correlation := p.CorrelationID
	if correlation == "" {
		correlation = actor.CorrelationID
	}
	payload["correlation_id"] = correlation

	encodedPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	eventID := uuid.New()
	now := s.now().UTC()
	envelope := map[string]any{
		"event_id":          eventID.String(),
		"event_type":        eventType,
		"event_version":     1,
		"occurred_at":       now.Format(time.RFC3339Nano),
		"source_service":    sourceService,
		"producer":          sourceService,
		"aggregate_type":    "WarehouseTask",
		"aggregate_id":      taskID,
		"aggregate_version": p.TaskVersion,
		"correlation_id":    correlation,
		"causation_id":      taskID,
		"site_id":           p.SiteID,
		"schema_version":    1,
		// Ordering per warehouse task: a completion must never overtake the
		// start that preceded it.
		"metadata": map[string]any{"partition_key": taskID, "producer": sourceService},
		"payload":  json.RawMessage(encodedPayload),
	}
	encodedEnvelope, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO integration_outbox (event_id, topic, event_type, aggregate_id, aggregate_version, partition_key, envelope_json) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		eventID, PDATaskEventTopic, eventType, taskID, p.TaskVersion, taskID, string(encodedEnvelope))
	return err
}

// ensurePDATaskID returns the stable PDA-side task identity, creating the
// execution row if the operator has not started the task yet.
func ensurePDATaskID(ctx context.Context, tx pgx.Tx, taskID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `SELECT pda_task_id::text FROM pda_task_execution WHERE warehouse_task_id=$1`, taskID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	err = tx.QueryRow(ctx, `INSERT INTO pda_task_execution (warehouse_task_id, task_version, state) SELECT $1, task_version, 'DISPATCHED' FROM wms_task_projection WHERE warehouse_task_id=$1 RETURNING pda_task_id::text`, taskID).Scan(&id)
	return id, err
}

func loadProjection(ctx context.Context, tx pgx.Tx, taskID string) (projection, error) {
	var p projection
	var scans []byte
	err := tx.QueryRow(ctx, `
SELECT task_type, task_version, status, warehouse_id,
       COALESCE(source_location_id,''), COALESCE(destination_location_id,''),
       COALESCE(revision_id,''), COALESCE(item_code,''), COALESCE(lot_id,''), COALESCE(uom_code,''),
       requested_quantity, remaining_quantity, scan_requirements,
       COALESCE(correlation_id,''), COALESCE(work_order_id,''), COALESCE(material_request_id,''), COALESCE(site_id,'')
FROM wms_task_projection WHERE warehouse_task_id=$1 FOR UPDATE`, taskID).
		Scan(&p.TaskType, &p.TaskVersion, &p.Status, &p.WarehouseID,
			&p.SourceLocationID, &p.DestinationID,
			&p.RevisionID, &p.ItemCode, &p.LotID, &p.UOMCode,
			&p.RequestedQuantity, &p.RemainingQuantity, &scans,
			&p.CorrelationID, &p.WorkOrderID, &p.MaterialRequestID, &p.SiteID)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrTaskNotFound
	}
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(scans, &p.ScanRequirements); err != nil {
		return p, err
	}
	return p, nil
}

func loadExecution(ctx context.Context, tx pgx.Tx, taskID string) (execution, error) {
	var e execution
	err := tx.QueryRow(ctx, `SELECT pda_task_id::text, state FROM pda_task_execution WHERE warehouse_task_id=$1 FOR UPDATE`, taskID).Scan(&e.PDATaskID, &e.State)
	if errors.Is(err, pgx.ErrNoRows) {
		return execution{State: "DISPATCHED"}, nil
	}
	return e, err
}

func acceptedScanTypes(ctx context.Context, tx pgx.Tx, taskID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT scan_type FROM pda_task_scan WHERE warehouse_task_id=$1 AND accepted`, taskID)
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

func scanEvidence(ctx context.Context, tx pgx.Tx, taskID string) ([]map[string]any, error) {
	rows, err := tx.Query(ctx, `SELECT scan_type, scan_value, accepted, scanned_at FROM pda_task_scan WHERE warehouse_task_id=$1 ORDER BY scanned_at`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	evidence := make([]map[string]any, 0)
	for rows.Next() {
		var scanType, scanValue string
		var accepted bool
		var at time.Time
		if err := rows.Scan(&scanType, &scanValue, &accepted, &at); err != nil {
			return nil, err
		}
		evidence = append(evidence, map[string]any{"scan_type": scanType, "scan_value": scanValue, "accepted": accepted, "scanned_at": at.UTC().Format(time.RFC3339Nano)})
	}
	return evidence, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner) (Task, error) {
	var t Task
	var scans []byte
	err := row.Scan(&t.WarehouseTaskID, &t.PDATaskID, &t.TaskType, &t.TaskVersion, &t.WMSStatus,
		&t.ExecutionState, &t.Priority, &t.WarehouseID,
		&t.WorkOrderCode, &t.ItemCode, &t.RevisionID, &t.LotID,
		&t.SourceLocationID, &t.SourceLocationCode, &t.SourceBinID, &t.SourceBinCode,
		&t.DestinationID, &t.DestinationCode, &t.DestinationBinID, &t.DestinationBinCode, &t.LPNCode, &t.Allocations,
		&t.RequestedQty, &t.ConfirmedQty, &t.RemainingQty, &t.UOMCode,
		&scans, &t.OperatorID, &t.CorrelationID)
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal(scans, &t.ScanRequirements); err != nil {
		return t, err
	}
	if t.CompletedScans == nil {
		t.CompletedScans = []string{}
	}
	return t, nil
}

func readTask(ctx context.Context, tx pgx.Tx, taskID string) (Task, error) {
	row := tx.QueryRow(ctx, `
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
	if err != nil {
		return task, err
	}
	accepted, err := acceptedScanTypes(ctx, tx, taskID)
	if err != nil {
		return task, err
	}
	task.CompletedScans = accepted
	return task, nil
}

func requestHash(taskID, commandType string, version int64, operator string, qty float64, extra string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s|%.6f|%s", taskID, commandType, version, operator, qty, extra)))
	return hex.EncodeToString(sum[:])
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}
