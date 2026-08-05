package domain

import (
	platform "github.com/company/pda-backend/internal/platform/domain"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotFound        = &platform.DomainError{Code: "SHIPMENT_NOT_FOUND", SafeMessage: "Shipment not found"}
	ErrNotReady        = &platform.DomainError{Code: "SHIPMENT_NOT_READY", SafeMessage: "Shipment prerequisites are incomplete"}
	ErrPackage         = &platform.DomainError{Code: "PACKAGE_INCOMPLETE", SafeMessage: "Every package must be complete"}
	ErrCarrier         = &platform.DomainError{Code: "CARRIER_INVALID", SafeMessage: "Carrier is invalid"}
	ErrTracking        = &platform.DomainError{Code: "TRACKING_INVALID", SafeMessage: "Tracking number is invalid"}
	ErrVersion         = &platform.DomainError{Code: "SHIPMENT_VERSION_CONFLICT", SafeMessage: "Shipment version has changed"}
	ErrConfirmed       = &platform.DomainError{Code: "SHIPMENT_ALREADY_CONFIRMED", SafeMessage: "Shipment is already confirmed"}
	ErrPackageNotFound = &platform.DomainError{Code: "BARCODE_UNKNOWN", SafeMessage: "Package is unknown"}
	ErrPackageContext  = &platform.DomainError{Code: "BARCODE_WRONG_CONTEXT", SafeMessage: "Package does not belong to this shipment"}
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
		if p.Status != "COMPLETED" && p.Status != "VERIFIED" {
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

func (s *Shipment) VerifyPackage(packageID, barcode string, base int64, now time.Time) error {
	if s.Version != base {
		return ErrVersion
	}
	for i := range s.Packages {
		if s.Packages[i].ID != packageID {
			continue
		}
		if barcode != "" && barcode != packageID {
			return ErrPackageContext
		}
		if s.Packages[i].Status == "COMPLETED" || s.Packages[i].Status == "VERIFIED" {
			return nil
		}
		s.Packages[i].Status = "VERIFIED"
		s.Version++
		s.UpdatedAt = now.UTC()
		return nil
	}
	return ErrPackageNotFound
}
func (s *Shipment) Confirm(carrier, tracking string, base int64, now time.Time) error {
	return s.ConfirmWithPackages(carrier, tracking, nil, base, now)
}

func (s *Shipment) ConfirmWithPackages(carrier, tracking string, verifiedPackageIDs []string, base int64, now time.Time) error {
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
	if len(verifiedPackageIDs) > 0 {
		verified := map[string]bool{}
		for _, id := range verifiedPackageIDs {
			verified[id] = true
		}
		for _, p := range s.Packages {
			if !verified[p.ID] {
				return ErrPackage
			}
		}
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
