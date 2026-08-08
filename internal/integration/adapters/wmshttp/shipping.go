package wmshttp

import (
	"context"
	"fmt"
	"strings"
	"time"

	platform "github.com/company/pda-backend/internal/platform/domain"
	shippingapp "github.com/company/pda-backend/internal/shipping/application"
	shippingdomain "github.com/company/pda-backend/internal/shipping/domain"
	"github.com/google/uuid"
)

// ShippingAdapter maps authoritative shipment reads and delegates mutations
// to Shipping owner commands. It never becomes a second shipment authority.
type ShippingAdapter struct {
	client *Client
	shippingAdapter
}

func NewShippingAdapter(client *Client) *ShippingAdapter {
	return &ShippingAdapter{client: client, shippingAdapter: shippingAdapter(Unavailable{"shipping"})}
}

func (a *ShippingAdapter) Summary(ctx context.Context, id string, actor platform.ActorContext) (shippingdomain.Shipment, error) {
	payload, err := a.client.GetShippingShipment(ctx, id)
	if err != nil {
		return shippingdomain.Shipment{}, err
	}
	shipment := mapShipment(payload)
	if shipment.WarehouseID != "" && shipment.WarehouseID != actor.WarehouseID {
		return shippingdomain.Shipment{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Shipment access denied"}
	}
	return shipment, nil
}

func (a *ShippingAdapter) Readiness(ctx context.Context, id string, actor platform.ActorContext) (shippingdomain.Readiness, error) {
	shipment, err := a.Summary(ctx, id, actor)
	if err != nil {
		return shippingdomain.Readiness{}, err
	}
	return shipment.Readiness(), nil
}

func mapShipment(raw map[string]any) shippingdomain.Shipment {
	text := func(keys ...string) string {
		for _, key := range keys {
			if v, ok := raw[key].(string); ok {
				return v
			}
		}
		return ""
	}
	float := func(keys ...string) int64 {
		for _, key := range keys {
			if v, ok := raw[key].(float64); ok {
				return int64(v)
			}
		}
		return 0
	}
	id := text("id", "shipment_id")
	warehouse := text("warehouseId", "warehouse_id")
	order := text("orderId", "order_id", "sales_order_id")
	status := text("status", "shipment_status")
	version := float("version", "row_version")
	var carrier, tracking *string
	if value := text("carrier", "carrier_code"); value != "" {
		carrier = &value
	}
	if value := text("trackingNumber", "tracking_number"); value != "" {
		tracking = &value
	}
	updated := time.Now().UTC()
	if rawUpdated := text("updatedAt", "updated_at"); rawUpdated != "" {
		if parsed, err := time.Parse(time.RFC3339, rawUpdated); err == nil {
			updated = parsed
		}
	}
	packages := make([]shippingdomain.Package, 0)
	if rawPackages, ok := raw["packages"].([]any); ok {
		for _, rawPackage := range rawPackages {
			item, ok := rawPackage.(map[string]any)
			if !ok {
				continue
			}
			packageID := textFromMap(item, "id", "package_id")
			if packageID == "" {
				continue
			}
			packages = append(packages, shippingdomain.Package{ID: packageID, Status: textFromMap(item, "status", "package_status"), WeightGrams: int64(numberFromMap(item, "weight_grams", "weightGrams"))})
		}
	}
	return shippingdomain.Shipment{ID: id, OrderID: order, WarehouseID: warehouse, Status: status, Carrier: carrier, TrackingNumber: tracking, Version: version, Packages: packages, UpdatedAt: updated}
}

func textFromMap(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := raw[key].(string); ok {
			return value
		}
	}
	return ""
}

func numberFromMap(raw map[string]any, keys ...string) float64 {
	for _, key := range keys {
		if value, ok := raw[key].(float64); ok {
			return value
		}
	}
	return 0
}

func (a *ShippingAdapter) VerifyPackage(ctx context.Context, command shippingapp.PackageVerifyCommand) (shippingdomain.Shipment, error) {
	shipment, err := a.Summary(ctx, command.ShipmentID, command.Actor)
	if err != nil {
		return shippingdomain.Shipment{}, err
	}
	found := false
	for _, pkg := range shipment.Packages {
		if pkg.ID == command.PackageID {
			found = true
			break
		}
	}
	if !found {
		return shippingdomain.Shipment{}, shippingdomain.ErrPackageContext
	}
	commandID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(command.ShipmentID+"|"+command.PackageID+"|"+command.Barcode+"|"+fmt.Sprint(command.BaseVersion)))
	if err := a.client.VerifyShippingPackage(ctx, command.PackageID, command.Barcode, commandID.String(), command.BaseVersion, commandID.String()); err != nil {
		return shippingdomain.Shipment{}, err
	}
	return a.Summary(ctx, command.ShipmentID, command.Actor)
}
func (a *ShippingAdapter) Confirm(ctx context.Context, command shippingapp.ConfirmCommand) (shippingdomain.Shipment, error) {
	shipment, err := a.Summary(ctx, command.ShipmentID, command.Actor)
	if err != nil {
		return shippingdomain.Shipment{}, err
	}
	if command.Carrier != "" && (shipment.Carrier == nil || !strings.EqualFold(strings.TrimSpace(command.Carrier), strings.TrimSpace(*shipment.Carrier))) {
		return shippingdomain.Shipment{}, shippingdomain.ErrCarrier
	}
	if command.Tracking != "" && (shipment.TrackingNumber == nil || strings.TrimSpace(command.Tracking) != strings.TrimSpace(*shipment.TrackingNumber)) {
		return shippingdomain.Shipment{}, shippingdomain.ErrTracking
	}
	if len(command.VerifiedPackageIDs) > 0 {
		known := make(map[string]struct{}, len(shipment.Packages))
		for _, pkg := range shipment.Packages {
			known[pkg.ID] = struct{}{}
		}
		for _, id := range command.VerifiedPackageIDs {
			if _, ok := known[id]; !ok {
				return shippingdomain.Shipment{}, shippingdomain.ErrPackageContext
			}
		}
	}
	if _, err := a.client.ConfirmShipping(ctx, command.ShipmentID, command.ID.String(), command.BaseVersion, command.Key); err != nil {
		return shippingdomain.Shipment{}, err
	}
	return a.Summary(ctx, command.ShipmentID, command.Actor)
}
