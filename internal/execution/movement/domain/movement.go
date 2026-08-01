package domain

import (
	platform "github.com/company/pda-backend/internal/platform/domain"
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
	ID                   string    `json:"id"`
	Workflow             Workflow  `json:"workflow"`
	Status               Status    `json:"status"`
	WarehouseID          string    `json:"warehouseId"`
	OperatorID           *string   `json:"operatorId"`
	Version              int64     `json:"version"`
	SourceLocation       string    `json:"sourceLocation"`
	DestinationLocation  string    `json:"destinationLocation"`
	ItemID               string    `json:"itemId"`
	Barcode              string    `json:"barcode"`
	RequiredQuantity     int64     `json:"requiredQuantity"`
	CompletedQuantity    int64     `json:"completedQuantity"`
	SourceValidated      bool      `json:"sourceValidated"`
	DestinationValidated bool      `json:"destinationValidated"`
	ItemValidated        bool      `json:"itemValidated"`
	UpdatedAt            time.Time `json:"updatedAt"`
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
	if value != t.SourceLocation {
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
	if value != t.DestinationLocation {
		return ErrDestination
	}
	t.DestinationValidated = true
	t.Status = InProgress
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
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
func (t *Task) Move(operator string, quantity, base int64, now time.Time) error {
	if err := t.guard(operator, base); err != nil {
		return err
	}
	if !t.SourceValidated || !t.DestinationValidated || (t.Workflow != Putaway && !t.ItemValidated) {
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
