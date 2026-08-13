package domain

import (
	platform "github.com/company/pda-backend/internal/platform/domain"
	"strings"
	"time"
)

type Workflow string
type Status string

const (
	Putaway            Workflow = "PUTAWAY"
	Picking            Workflow = "PICKING"
	Replenishment      Workflow = "REPLENISHMENT"
	New                Status   = "NEW"
	InProgress         Status   = "IN_PROGRESS"
	PartiallyCompleted Status   = "PARTIALLY_COMPLETED"
	Completed          Status   = "COMPLETED"
)

var (
	ErrNotFound    = &platform.DomainError{Code: "TASK_NOT_FOUND", SafeMessage: "Movement task not found"}
	ErrNotAssigned = &platform.DomainError{Code: "TASK_NOT_ASSIGNED", SafeMessage: "Movement task is assigned to another operator"}
	ErrVersion     = &platform.DomainError{Code: "TASK_VERSION_CONFLICT", SafeMessage: "Movement task version has changed"}
	ErrSource      = &platform.DomainError{Code: "SOURCE_LOCATION_INVALID", SafeMessage: "Source location is invalid"}
	ErrDestination = &platform.DomainError{Code: "DESTINATION_LOCATION_INVALID", SafeMessage: "Destination location is invalid"}
	ErrItem        = &platform.DomainError{Code: "ITEM_INVALID", SafeMessage: "Item or barcode is invalid"}
	ErrSequence    = &platform.DomainError{Code: "VALIDATION_SEQUENCE_INVALID", SafeMessage: "Required validation steps are incomplete"}
	ErrQuantity    = &platform.DomainError{Code: "QUANTITY_EXCEEDS_ALLOWED", SafeMessage: "Quantity exceeds the remaining requirement"}
	ErrStock       = &platform.DomainError{Code: "INSUFFICIENT_STOCK", SafeMessage: "Source location has insufficient stock"}
	ErrCapacity    = &platform.DomainError{Code: "LOCATION_CAPACITY_EXCEEDED", SafeMessage: "Destination location has insufficient compatible capacity"}
	ErrIncomplete  = &platform.DomainError{Code: "TASK_INCOMPLETE", SafeMessage: "Remaining required quantity must be zero"}
)

type Task struct {
	ID                      string    `json:"id"`
	Workflow                Workflow  `json:"workflow"`
	Status                  Status    `json:"status"`
	WarehouseID             string    `json:"warehouseId"`
	OperatorID              *string   `json:"operatorId"`
	Version                 int64     `json:"version"`
	SourceLocation          string    `json:"sourceLocation"`
	SourceLocationID        string    `json:"sourceLocationId,omitempty"`
	SourceLocationCode      string    `json:"sourceLocationCode,omitempty"`
	SourceBin               string    `json:"sourceBin,omitempty"`
	SourceBinID             string    `json:"sourceBinId,omitempty"`
	SourceBinCode           string    `json:"sourceBinCode,omitempty"`
	DestinationLocation     string    `json:"destinationLocation"`
	DestinationLocationID   string    `json:"destinationLocationId,omitempty"`
	DestinationLocationCode string    `json:"destinationLocationCode,omitempty"`
	DestinationCode         string    `json:"destinationCode,omitempty"`
	DestinationBinID        string    `json:"destinationBinId,omitempty"`
	DestinationBinCode      string    `json:"destinationBinCode,omitempty"`
	ItemID                  string    `json:"itemId"`
	Barcode                 string    `json:"barcode"`
	Lot                     string    `json:"lot,omitempty"`
	RequiredQuantity        int64     `json:"requiredQuantity"`
	CompletedQuantity       int64     `json:"completedQuantity"`
	SourceValidated         bool      `json:"sourceValidated"`
	DestinationValidated    bool      `json:"destinationValidated"`
	ItemValidated           bool      `json:"itemValidated"`
	LotValidated            bool      `json:"lotValidated"`
	ScanRequirements        []string  `json:"scanRequirements,omitempty"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

func (t Task) Remaining() int64 { return t.RequiredQuantity - t.CompletedQuantity }
func (t *Task) guard(operator string, base int64) error {
	if t.Version != base {
		return ErrVersion
	}
	if t.OperatorID != nil && *t.OperatorID != operator {
		return ErrNotAssigned
	}
	if t.OperatorID == nil {
		t.OperatorID = &operator
	}
	return nil
}
func (t *Task) ValidateSource(operator, value string, base int64, now time.Time) error {
	if err := t.guard(operator, base); err != nil {
		return err
	}
	if !matchesAny(value, t.SourceBin, t.SourceBinCode, t.SourceBinID, t.SourceLocation, t.SourceLocationCode, t.SourceLocationID) {
		return ErrSource
	}
	t.SourceValidated = true
	t.Status = InProgress
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}
func (t *Task) ValidateDestination(operator, value string, base int64, now time.Time) error {
	if err := t.guard(operator, base); err != nil {
		return err
	}
	if !t.SourceValidated {
		return ErrSequence
	}
	if !matchesAny(value, t.DestinationBinID, t.DestinationBinCode, t.DestinationLocation, t.DestinationLocationCode, t.DestinationLocationID) {
		return ErrDestination
	}
	t.DestinationValidated = true
	t.Status = InProgress
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}

func matchesAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if candidate != "" && strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(candidate)) {
			return true
		}
	}
	return false
}
func (t *Task) ValidateItem(operator, barcode string, base int64, now time.Time) error {
	if err := t.guard(operator, base); err != nil {
		return err
	}
	if !t.SourceValidated || (t.Workflow == Replenishment && !t.DestinationValidated) {
		return ErrSequence
	}
	if barcode != t.Barcode {
		return ErrItem
	}
	t.ItemValidated = true
	t.Status = InProgress
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}
func (t *Task) ValidateLot(operator, lot string, base int64, now time.Time) error {
	if err := t.guard(operator, base); err != nil {
		return err
	}
	if !t.SourceValidated || !t.ItemValidated {
		return ErrSequence
	}
	if t.Lot != "" && lot != t.Lot {
		return &platform.DomainError{Code: "LOT_INVALID", SafeMessage: "Lot is not assigned to this putaway task"}
	}
	t.LotValidated = true
	t.Status = InProgress
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}
func (t *Task) Move(operator string, quantity, base int64, now time.Time) error {
	if err := t.guard(operator, base); err != nil {
		return err
	}
	if !t.SourceValidated || !t.ItemValidated || !t.DestinationValidated || (t.Lot != "" && !t.LotValidated) {
		return ErrSequence
	}
	if quantity <= 0 || quantity > t.Remaining() {
		return ErrQuantity
	}
	t.CompletedQuantity += quantity
	t.Version++
	t.UpdatedAt = now.UTC()
	if t.Remaining() == 0 {
		t.Status = Completed
	} else {
		t.Status = PartiallyCompleted
	}
	return nil
}
func (t *Task) GuardCompletion(operator string, base int64) error { return t.guard(operator, base) }

type Location struct {
	ID               string  `json:"id"`
	Zone             string  `json:"zone"`
	Capacity         int64   `json:"capacity"`
	UsedCapacity     int64   `json:"usedCapacity"`
	CompatibleItemID *string `json:"compatibleItemId"`
}
