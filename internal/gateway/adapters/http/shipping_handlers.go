package httpadapter

import (
	platform "github.com/company/pda-backend/internal/platform/domain"
	shippingapp "github.com/company/pda-backend/internal/shipping/application"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"strings"
)

func (r *Router) shipmentSummary(w http.ResponseWriter, q *http.Request) {
	v, e := r.shipping.Summary(q.Context(), chi.URLParam(q, "shipmentId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) shipmentReadiness(w http.ResponseWriter, q *http.Request) {
	v, e := r.shipping.Readiness(q.Context(), chi.URLParam(q, "shipmentId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) shipmentConfirm(w http.ResponseWriter, q *http.Request) {
	key := q.Header.Get("Idempotency-Key")
	id, e := uuid.Parse(key)
	if e != nil {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "UUID Idempotency-Key required"})
		return
	}
	base, e := strconv.ParseInt(strings.Trim(q.Header.Get("If-Match"), "\""), 10, 64)
	if e != nil {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Numeric If-Match required"})
		return
	}
	var x struct {
		Carrier  string `json:"carrier"`
		Tracking string `json:"trackingNumber"`
	}
	if e = decode(q, &x); e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.shipping.Confirm(q.Context(), shippingapp.ConfirmCommand{ID: id, Key: key, ShipmentID: chi.URLParam(q, "shipmentId"), Carrier: x.Carrier, Tracking: x.Tracking, BaseVersion: base, Actor: r.actor(q)})
	r.movementResponse(w, q, v, e)
}
func (r *Router) shipmentCommandStatus(w http.ResponseWriter, q *http.Request) {
	id, e := uuid.Parse(chi.URLParam(q, "commandId"))
	if e != nil {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Invalid command ID"})
		return
	}
	v, e := r.shipping.CommandStatus(q.Context(), id, r.actor(q))
	r.movementResponse(w, q, v, e)
}
