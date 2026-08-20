package httpadapter

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/wmstask"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type deliveryAcknowledger interface {
	AckDelivery(context.Context, string, wmstask.Actor, time.Time) error
}

type postgresDeliveryAck struct{ pool *pgxpool.Pool }

func (s postgresDeliveryAck) AckDelivery(ctx context.Context, eventID string, actor wmstask.Actor, deviceReceivedAt time.Time) error {
	if eventID == "" || actor.OperatorID == "" || actor.DeviceID == "" || actor.WarehouseID == "" {
		return &domain.DomainError{Code: "OPERATOR_CONTEXT_REQUIRED", SafeMessage: "Operator and device context are required"}
	}
	if deviceReceivedAt.IsZero() {
		deviceReceivedAt = time.Now().UTC()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	backendRecordedAt := time.Now().UTC()
	result, err := tx.Exec(ctx, `INSERT INTO pda_event_delivery_ack(event_id,operator_id,device_id,warehouse_id,device_received_at,backend_recorded_at,last_correlation_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(event_id,operator_id,device_id) DO UPDATE SET last_ack_at=now(),ack_count=pda_event_delivery_ack.ack_count+1,last_correlation_id=EXCLUDED.last_correlation_id,warehouse_id=EXCLUDED.warehouse_id WHERE pda_event_delivery_ack.device_received_at IS NULL OR pda_event_delivery_ack.device_received_at=EXCLUDED.device_received_at`, eventID, actor.OperatorID, actor.DeviceID, actor.WarehouseID, deviceReceivedAt.UTC(), backendRecordedAt, actor.CorrelationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return &domain.DomainError{Code: "PDA_ACK_TIMESTAMP_CONFLICT", SafeMessage: "Delivery acknowledgement timestamp conflicts with existing evidence"}
	}
	payload, _ := json.Marshal(map[string]any{"delivery_event_id": eventID, "operator_id": actor.OperatorID, "device_id": actor.DeviceID, "warehouse_id": actor.WarehouseID, "device_received_at": deviceReceivedAt.UTC(), "backend_recorded_at": backendRecordedAt})
	outEvent := uuid.New()
	envelope, _ := json.Marshal(map[string]any{"event_id": outEvent.String(), "event_type": "PDA.TaskReceivedOnDevice.v1", "event_version": 1, "occurred_at": deviceReceivedAt.UTC(), "source_service": "pda-backend", "aggregate_type": "PDAEventDelivery", "aggregate_id": eventID, "aggregate_version": 1, "correlation_id": actor.CorrelationID, "causation_id": eventID, "schema_version": 1, "metadata": map[string]any{"partition_key": eventID}, "payload": json.RawMessage(payload)})
	if _, err = tx.Exec(ctx, `INSERT INTO integration_outbox(event_id,topic,event_type,aggregate_id,aggregate_version,partition_key,envelope_json) VALUES($1,$2,$3,$4,1,$4,$5) ON CONFLICT(event_id) DO NOTHING`, outEvent, "PDA.TaskReceivedOnDevice.v1", "PDA.TaskReceivedOnDevice.v1", eventID, string(envelope)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Router) deliveryAck(w http.ResponseWriter, req *http.Request) {
	if r.wmsTasks == nil && r.deliveryAcks == nil {
		writeError(w, &domain.DomainError{Code: "DELIVERY_ACK_UNAVAILABLE", SafeMessage: "Delivery acknowledgement is unavailable"}, correlation(req.Context()))
		return
	}
	actor := r.actor(req)
	if actor.OperatorID == "" || actor.DeviceID == "" || actor.WarehouseID == "" {
		writeError(w, &domain.DomainError{Code: "OPERATOR_CONTEXT_REQUIRED", SafeMessage: "Operator and device context are required"}, correlation(req.Context()))
		return
	}
	eventID := chi.URLParam(req, "eventId")
	if eventID == "" {
		writeError(w, &domain.DomainError{Code: "EVENT_ID_REQUIRED", SafeMessage: "Event ID is required"}, correlation(req.Context()))
		return
	}
	var body struct {
		ReceivedAt string `json:"receivedAt"`
	}
	if req.Body != nil {
		_ = json.NewDecoder(req.Body).Decode(&body)
	}
	deviceReceivedAt := time.Time{}
	if body.ReceivedAt != "" {
		parsed, err := time.Parse(time.RFC3339Nano, body.ReceivedAt)
		if err != nil {
			writeError(w, &domain.DomainError{Code: "DELIVERY_TIMESTAMP_INVALID", SafeMessage: "Delivery timestamp is invalid"}, correlation(req.Context()))
			return
		}
		deviceReceivedAt = parsed
	}
	var acknowledger deliveryAcknowledger = r.deliveryAcks
	if r.wmsTasks != nil {
		acknowledger = r.wmsTasks
	}
	if err := acknowledger.AckDelivery(req.Context(), eventID, wmstask.Actor{OperatorID: actor.OperatorID, DeviceID: actor.DeviceID, WarehouseID: actor.WarehouseID, CorrelationID: actor.CorrelationID}, deviceReceivedAt); err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, map[string]any{"eventId": eventID, "acknowledged": true, "receivedAt": body.ReceivedAt}, correlation(req.Context()), r.now())
}
