package httpadapter

import (
	platform "github.com/company/pda-backend/internal/platform/domain"
	shippingapp "github.com/company/pda-backend/internal/shipping/application"
	shippingdomain "github.com/company/pda-backend/internal/shipping/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"strings"
)

func (r *Router) shipmentSummary(w http.ResponseWriter, q *http.Request) {
	v, e := r.shipping.Summary(q.Context(), chi.URLParam(q, "shipmentId"), r.actor(q))
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	writeData(w, http.StatusOK, shippingView(v), correlation(q.Context()), r.now())
}
func (r *Router) shipmentReadiness(w http.ResponseWriter, q *http.Request) {
	v, e := r.shipping.Readiness(q.Context(), chi.URLParam(q, "shipmentId"), r.actor(q))
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	writeData(w, http.StatusOK, map[string]any{"readinessStatus": map[bool]string{true: "READY", false: "BLOCKED"}[v.Ready], "ready": v.Ready, "blockingReasons": v.BlockingReasons, "asOf": v.AsOf, "stale": false}, correlation(q.Context()), r.now())
}
func (r *Router) packageVerify(w http.ResponseWriter, q *http.Request) {
	base, err := strconv.ParseInt(strings.Trim(q.Header.Get("If-Match"), `"`), 10, 64)
	if err != nil {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Numeric If-Match required"})
		return
	}
	var x struct {
		ShipmentID      string `json:"shipmentId"`
		PackageID       string `json:"packageId"`
		RawValue        string `json:"rawValue"`
		NormalizedValue string `json:"normalizedValue"`
		Symbology       string `json:"symbology"`
		ScanContext     string `json:"scanContext"`
		BaseVersion     int64  `json:"baseVersion"`
	}
	if err = decode(q, &x); err != nil {
		r.movementResponse(w, q, nil, err)
		return
	}
	if x.ShipmentID != "" && x.ShipmentID != chi.URLParam(q, "shipmentId") || x.PackageID != "" && x.PackageID != chi.URLParam(q, "packageId") || x.BaseVersion != 0 && x.BaseVersion != base || x.ScanContext != "" && x.ScanContext != "SHIPPING_PACKAGE" {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "BARCODE_WRONG_CONTEXT", SafeMessage: "Package scanner context does not match the request"})
		return
	}
	barcode := x.NormalizedValue
	if barcode == "" {
		barcode = x.RawValue
	}
	v, err := r.shipping.VerifyPackage(q.Context(), shippingapp.PackageVerifyCommand{ShipmentID: chi.URLParam(q, "shipmentId"), PackageID: chi.URLParam(q, "packageId"), Barcode: barcode, BaseVersion: base, Actor: r.actor(q)})
	if err != nil {
		r.movementResponse(w, q, nil, err)
		return
	}
	writeData(w, http.StatusOK, shippingView(v), correlation(q.Context()), r.now())
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
		CommandID          uuid.UUID `json:"commandId"`
		IdempotencyKey     string    `json:"idempotencyKey"`
		ShipmentID         string    `json:"shipmentId"`
		Carrier            string    `json:"carrier"`
		CarrierCode        string    `json:"carrierCode"`
		Tracking           string    `json:"trackingNumber"`
		VerifiedPackageIDs []string  `json:"verifiedPackageIds"`
		BaseVersion        int64     `json:"baseVersion"`
	}
	if e = decode(q, &x); e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	if x.Carrier == "" {
		x.Carrier = x.CarrierCode
	}
	if x.ShipmentID != "" && x.ShipmentID != chi.URLParam(q, "shipmentId") || x.CommandID != uuid.Nil && x.CommandID != id || x.IdempotencyKey != "" && x.IdempotencyKey != key || x.BaseVersion != 0 && x.BaseVersion != base {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Shipment command metadata does not match headers"})
		return
	}
	v, e := r.shipping.Confirm(q.Context(), shippingapp.ConfirmCommand{ID: id, Key: key, ShipmentID: chi.URLParam(q, "shipmentId"), Carrier: x.Carrier, Tracking: x.Tracking, VerifiedPackageIDs: x.VerifiedPackageIDs, BaseVersion: base, Actor: r.actor(q)})
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	view := shippingView(v)
	view["manifestReference"] = nil
	view["auditId"] = id
	writeData(w, http.StatusOK, view, correlation(q.Context()), r.now())
}

func shippingView(shipment shippingdomain.Shipment) map[string]any {
	readiness := shipment.Readiness()
	verified := 0
	for _, p := range shipment.Packages {
		if p.Status == "COMPLETED" || p.Status == "VERIFIED" {
			verified++
		}
	}
	carrier := ""
	if shipment.Carrier != nil {
		carrier = *shipment.Carrier
	}
	return map[string]any{"shipmentId": shipment.ID, "salesOrderId": shipment.OrderID, "customer": nil, "shipTo": nil, "packageCount": len(shipment.Packages), "verifiedPackageCount": verified, "carrierCode": carrier, "trackingNumber": shipment.TrackingNumber, "readinessStatus": map[bool]string{true: "READY", false: "BLOCKED"}[readiness.Ready], "blockingReasons": readiness.BlockingReasons, "status": shipment.Status, "version": shipment.Version, "packages": shipment.Packages, "asOf": shipment.UpdatedAt, "manifestReference": nil}
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
