package httpadapter

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/wmstask"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type deliveryAcknowledger interface {
	AckDelivery(context.Context, string, wmstask.Actor) error
}

type postgresDeliveryAck struct{ pool *pgxpool.Pool }

func (s postgresDeliveryAck) AckDelivery(ctx context.Context, eventID string, actor wmstask.Actor) error {
	if eventID == "" || actor.OperatorID == "" || actor.DeviceID == "" || actor.WarehouseID == "" {
		return &domain.DomainError{Code: "OPERATOR_CONTEXT_REQUIRED", SafeMessage: "Operator and device context are required"}
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO pda_event_delivery_ack(event_id,operator_id,device_id,warehouse_id,last_correlation_id)
VALUES($1,$2,$3,$4,$5)
ON CONFLICT(event_id,operator_id,device_id) DO UPDATE SET
  last_ack_at=now(), ack_count=pda_event_delivery_ack.ack_count+1,
  last_correlation_id=EXCLUDED.last_correlation_id, warehouse_id=EXCLUDED.warehouse_id`, eventID, actor.OperatorID, actor.DeviceID, actor.WarehouseID, actor.CorrelationID)
	return err
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
	var acknowledger deliveryAcknowledger = r.deliveryAcks
	if r.wmsTasks != nil {
		acknowledger = r.wmsTasks
	}
	if err := acknowledger.AckDelivery(req.Context(), eventID, wmstask.Actor{OperatorID: actor.OperatorID, DeviceID: actor.DeviceID, WarehouseID: actor.WarehouseID, CorrelationID: actor.CorrelationID}); err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, map[string]any{"eventId": eventID, "acknowledged": true, "receivedAt": body.ReceivedAt}, correlation(req.Context()), r.now())
}
