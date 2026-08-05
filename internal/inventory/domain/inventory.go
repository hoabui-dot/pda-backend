package domain

import (
	platform "github.com/company/pda-backend/internal/platform/domain"
	"time"
)

var (
	ErrNotFound          = &platform.DomainError{Code: "INVENTORY_NOT_FOUND", SafeMessage: "Inventory record not found"}
	ErrSameLocation      = &platform.DomainError{Code: "SOURCE_EQUALS_DESTINATION", SafeMessage: "Source and destination must differ"}
	ErrSource            = &platform.DomainError{Code: "SOURCE_LOCATION_INVALID", SafeMessage: "Source location is invalid"}
	ErrDestination       = &platform.DomainError{Code: "DESTINATION_LOCATION_INVALID", SafeMessage: "Destination location is invalid"}
	ErrStock             = &platform.DomainError{Code: "INSUFFICIENT_STOCK", SafeMessage: "Source location has insufficient stock"}
	ErrVersion           = &platform.DomainError{Code: "TASK_VERSION_CONFLICT", SafeMessage: "Cycle count version has changed"}
	ErrAssigned          = &platform.DomainError{Code: "TASK_NOT_ASSIGNED", SafeMessage: "Cycle count is assigned to another operator"}
	ErrIncomplete        = &platform.DomainError{Code: "TASK_INCOMPLETE", SafeMessage: "All count lines must be submitted"}
	ErrLocationInvalid   = &platform.DomainError{Code: "LOCATION_INVALID", SafeMessage: "Count location is invalid"}
	ErrItemNotInDocument = &platform.DomainError{Code: "ITEM_NOT_IN_DOCUMENT", SafeMessage: "Item is not part of this count task"}
)

type Balance struct {
	WarehouseID  string    `json:"warehouseId"`
	LocationID   string    `json:"locationId"`
	ItemID       string    `json:"itemId"`
	ItemCode     string    `json:"itemCode"`
	Description  string    `json:"description"`
	LocationCode string    `json:"locationCode"`
	Quantity     int64     `json:"quantity"`
	OnHand       int64     `json:"onHand"`
	Reserved     int64     `json:"reserved"`
	Available    int64     `json:"available"`
	Damaged      int64     `json:"damaged"`
	Hold         int64     `json:"hold"`
	Quarantine   int64     `json:"quarantine"`
	InTransit    int64     `json:"inTransit"`
	UOM          string    `json:"uom"`
	LotNumber    string    `json:"lotNumber"`
	SerialNumber string    `json:"serialNumber"`
	Stale        bool      `json:"stale"`
	Version      int64     `json:"version"`
	AsOf         time.Time `json:"asOf"`
}
type Movement struct {
	ID                  string    `json:"id"`
	TaskID              string    `json:"taskId"`
	Workflow            string    `json:"workflow"`
	WarehouseID         string    `json:"warehouseId"`
	ItemID              string    `json:"itemId"`
	SourceLocation      string    `json:"sourceLocation"`
	DestinationLocation string    `json:"destinationLocation"`
	Quantity            int64     `json:"quantity"`
	AggregateVersion    int64     `json:"aggregateVersion"`
	OccurredAt          time.Time `json:"occurredAt"`
}
type Transfer struct {
	CommandID           string    `json:"commandId"`
	WarehouseID         string    `json:"warehouseId"`
	SourceLocation      string    `json:"sourceLocation"`
	DestinationLocation string    `json:"destinationLocation"`
	ItemID              string    `json:"itemId"`
	Quantity            int64     `json:"quantity"`
	Status              string    `json:"status"`
	TransferID          string    `json:"transferId"`
	BeforeSource        *Balance  `json:"beforeSource"`
	AfterSource         *Balance  `json:"afterSource"`
	BeforeDestination   *Balance  `json:"beforeDestination"`
	AfterDestination    *Balance  `json:"afterDestination"`
	AuditID             string    `json:"auditId"`
	AsOf                time.Time `json:"asOf"`
}
type CountLine struct {
	ID               string `json:"id"`
	ItemID           string `json:"itemId"`
	ExpectedQuantity int64  `json:"expectedQuantity"`
	CountedQuantity  *int64 `json:"countedQuantity"`
	Variance         *int64 `json:"variance"`
	RecountRequired  bool   `json:"recountRequired"`
}
type CountTask struct {
	ID          string      `json:"id"`
	WarehouseID string      `json:"warehouseId"`
	LocationID  string      `json:"locationId"`
	Status      string      `json:"status"`
	OperatorID  *string     `json:"operatorId"`
	Version     int64       `json:"version"`
	BlindCount  bool        `json:"blindCount"`
	Tolerance   int64       `json:"tolerance"`
	Lines       []CountLine `json:"lines"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

func (t *CountTask) guard(op string, base int64) error {
	if t.Version != base {
		return ErrVersion
	}
	if t.OperatorID != nil && *t.OperatorID != op {
		return ErrAssigned
	}
	if t.OperatorID == nil {
		t.OperatorID = &op
	}
	return nil
}
func (t *CountTask) Submit(op, line string, q, base int64, now time.Time) error {
	if e := t.guard(op, base); e != nil {
		return e
	}
	for i := range t.Lines {
		if t.Lines[i].ID == line {
			v := q - t.Lines[i].ExpectedQuantity
			t.Lines[i].CountedQuantity = &q
			t.Lines[i].Variance = &v
			t.Lines[i].RecountRequired = v != 0
			t.Status = "IN_PROGRESS"
			t.Version++
			t.UpdatedAt = now.UTC()
			return nil
		}
	}
	return ErrNotFound
}
func (t *CountTask) Recount(op, line string, base int64, now time.Time) error {
	if e := t.guard(op, base); e != nil {
		return e
	}
	for i := range t.Lines {
		if t.Lines[i].ID == line {
			t.Lines[i].CountedQuantity = nil
			t.Lines[i].Variance = nil
			t.Lines[i].RecountRequired = true
			t.Status = "RECOUNT"
			t.Version++
			t.UpdatedAt = now.UTC()
			return nil
		}
	}
	return ErrNotFound
}
func (t *CountTask) Complete(op string, base int64, now time.Time) error {
	if e := t.guard(op, base); e != nil {
		return e
	}
	for _, l := range t.Lines {
		if l.CountedQuantity == nil || l.RecountRequired {
			return ErrIncomplete
		}
	}
	t.Status = "COMPLETED"
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}
