package domain

import (
	platform "github.com/company/pda-backend/internal/platform/domain"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotFound  = &platform.DomainError{Code: "SHIPMENT_NOT_FOUND", SafeMessage: "Shipment not found"}
	ErrNotReady  = &platform.DomainError{Code: "SHIPMENT_NOT_READY", SafeMessage: "Shipment prerequisites are incomplete"}
	ErrPackage   = &platform.DomainError{Code: "PACKAGE_INCOMPLETE", SafeMessage: "Every package must be complete"}
	ErrCarrier   = &platform.DomainError{Code: "CARRIER_INVALID", SafeMessage: "Carrier is invalid"}
	ErrTracking  = &platform.DomainError{Code: "TRACKING_INVALID", SafeMessage: "Tracking number is invalid"}
	ErrVersion   = &platform.DomainError{Code: "SHIPMENT_VERSION_CONFLICT", SafeMessage: "Shipment version has changed"}
	ErrConfirmed = &platform.DomainError{Code: "SHIPMENT_ALREADY_CONFIRMED", SafeMessage: "Shipment is already confirmed"}
)

type Package struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	WeightGrams int64  `json:"weightGrams"`
}
type Shipment struct {
	ID              string    `json:"id"`
	OrderID         string    `json:"orderId"`
	WarehouseID     string    `json:"warehouseId"`
	Status          string    `json:"status"`
	Carrier         *string   `json:"carrier"`
	TrackingNumber  *string   `json:"trackingNumber"`
	PickingComplete bool      `json:"pickingComplete"`
	Version         int64     `json:"version"`
	Packages        []Package `json:"packages"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
type Readiness struct {
	Ready            bool      `json:"ready"`
	PickingComplete  bool      `json:"pickingComplete"`
	PackagesComplete bool      `json:"packagesComplete"`
	BlockingReasons  []string  `json:"blockingReasons"`
	AsOf             time.Time `json:"asOf"`
}

func (s Shipment) Readiness() Readiness {
	r := Readiness{PickingComplete: s.PickingComplete, PackagesComplete: true, AsOf: s.UpdatedAt}
	if !s.PickingComplete {
		r.BlockingReasons = append(r.BlockingReasons, "PICKING_INCOMPLETE")
	}
	for _, p := range s.Packages {
		if p.Status != "COMPLETED" {
			r.PackagesComplete = false
			break
		}
	}
	if !r.PackagesComplete {
		r.BlockingReasons = append(r.BlockingReasons, "PACKAGE_INCOMPLETE")
	}
	r.Ready = r.PickingComplete && r.PackagesComplete && len(s.Packages) > 0
	return r
}
func (s *Shipment) Confirm(carrier, tracking string, base int64, now time.Time) error {
	if s.Version != base {
		return ErrVersion
	}
	if s.Status == "SHIPPED" {
		return ErrConfirmed
	}
	r := s.Readiness()
	if !r.PickingComplete {
		return ErrNotReady
	}
	if !r.PackagesComplete || len(s.Packages) == 0 {
		return ErrPackage
	}
	allowed := map[string]bool{"DHL": true, "FEDEX": true, "UPS": true, "VNPOST": true}
	carrier = strings.ToUpper(strings.TrimSpace(carrier))
	if !allowed[carrier] {
		return ErrCarrier
	}
	tracking = strings.TrimSpace(tracking)
	if ok, _ := regexp.MatchString("^[A-Za-z0-9-]{6,40}$", tracking); !ok {
		return ErrTracking
	}
	s.Carrier = &carrier
	s.TrackingNumber = &tracking
	s.Status = "SHIPPED"
	s.Version++
	s.UpdatedAt = now.UTC()
	return nil
}
