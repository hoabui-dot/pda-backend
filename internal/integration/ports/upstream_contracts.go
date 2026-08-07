package ports

import (
	"context"
	"time"
)

// These contracts use PDA-owned neutral types. WMS wire DTOs stay inside adapters.
type Entitlement struct {
	OperatorID    string
	WarehouseIDs  []string
	ZoneIDs       []string
	PermissionIDs []string
	Version       int64
}

type DevicePolicy struct {
	DeviceID          string
	WarehouseID       string
	Status            string
	MinimumAppVersion string
	OfflineAllowed    bool
	Version           int64
}

type ShiftContext struct {
	ShiftID     string
	OperatorID  string
	WarehouseID string
	Status      string
	StartsAt    time.Time
	EndsAt      time.Time
	Version     int64
}

type TaskAssignment struct {
	TaskID      string
	OperatorID  string
	DeviceID    string
	WarehouseID string
	Status      string
	Version     int64
}

type WorkflowPolicy struct {
	Workflow string
	Version  int64
	Rules    map[string]string
}

type BarcodeResolution struct {
	RawValue        string
	NormalizedValue string
	ItemID          string
	UOM             string
	LotNumber       string
	SerialNumber    string
	Expiry          *time.Time
	Version         int64
}

type InventoryValidation struct {
	Accepted          bool
	AvailableQuantity float64
	ReservedQuantity  float64
	UOM               string
	Version           int64
}

type ShipmentReadiness struct {
	ShipmentID string
	Ready      bool
	Reason     string
	Version    int64
}

type Snapshot struct {
	ID            string
	Domain        string
	HighWaterMark string
	CapturedAt    time.Time
	Complete      bool
}

type ReplayRequest struct {
	EventID       string
	Topic         string
	ConsumerGroup string
}

type Reconciliation struct {
	Domain        string
	CheckedAt     time.Time
	MismatchCount int
	Consistent    bool
}

type EntitlementReader interface {
	Entitlement(context.Context, string) (Entitlement, error)
}

type DevicePolicyReader interface {
	DevicePolicy(context.Context, string) (DevicePolicy, error)
}

type WarehouseShiftContextReader interface {
	ShiftContext(context.Context, string, string) (ShiftContext, error)
}

type TaskAssignmentPort interface {
	TaskAssignment(context.Context, string) (TaskAssignment, error)
}

type WorkflowPolicyReader interface {
	WorkflowPolicy(context.Context, string, string) (WorkflowPolicy, error)
}

type BarcodeResolver interface {
	ResolveBarcode(context.Context, string, string) (BarcodeResolution, error)
}

type InventoryValidator interface {
	ValidateInventory(context.Context, string, string, float64, string) (InventoryValidation, error)
}

type ShipmentReadinessReader interface {
	ShipmentReadiness(context.Context, string) (ShipmentReadiness, error)
}

type SnapshotReader interface {
	Snapshot(context.Context, string) (Snapshot, error)
}

type ReplayController interface {
	Replay(context.Context, ReplayRequest) error
}

type ReconciliationReader interface {
	Reconciliation(context.Context, string) (Reconciliation, error)
}
