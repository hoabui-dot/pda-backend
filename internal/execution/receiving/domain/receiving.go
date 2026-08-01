package domain

import (
	"strings"
	"time"

	platform "github.com/company/pda-backend/internal/platform/domain"
)

type Status string

const (
	StatusNew                Status = "NEW"
	StatusInProgress         Status = "IN_PROGRESS"
	StatusPartiallyCompleted Status = "PARTIALLY_COMPLETED"
	StatusCompleted          Status = "COMPLETED"
)

var (
	ErrNotFound            = &platform.DomainError{Code: "TASK_NOT_FOUND", SafeMessage: "Receiving task not found"}
	ErrNotAssigned         = &platform.DomainError{Code: "TASK_NOT_ASSIGNED", SafeMessage: "Receiving task is not assigned to this operator"}
	ErrAlreadyCompleted    = &platform.DomainError{Code: "TASK_ALREADY_COMPLETED", SafeMessage: "Receiving task is already completed"}
	ErrVersionConflict     = &platform.DomainError{Code: "TASK_VERSION_CONFLICT", SafeMessage: "Receiving task version has changed"}
	ErrBarcodeUnknown      = &platform.DomainError{Code: "BARCODE_UNKNOWN", SafeMessage: "Barcode is unknown"}
	ErrBarcodeWrongContext = &platform.DomainError{Code: "BARCODE_WRONG_CONTEXT", SafeMessage: "Barcode does not belong to this receiving task"}
	ErrQuantityExceeded    = &platform.DomainError{Code: "QUANTITY_EXCEEDS_ALLOWED", SafeMessage: "Quantity exceeds allowed amount"}
	ErrRemarkRequired      = &platform.DomainError{Code: "REMARK_REQUIRED", SafeMessage: "A remark is required for quantity variance"}
	ErrIncomplete          = &platform.DomainError{Code: "RECEIVING_TASK_INCOMPLETE", SafeMessage: "All receiving lines must be complete"}
)

type Policy struct {
	AllowOverReceipt        bool `json:"allowOverReceipt"`
	RequireRemarkOnVariance bool `json:"requireRemarkOnVariance"`
}
type Line struct {
	ID               string `json:"id"`
	ItemID           string `json:"itemId"`
	ItemName         string `json:"itemName"`
	Barcode          string `json:"barcode"`
	ExpectedQuantity int64  `json:"expectedQuantity"`
	ReceivedQuantity int64  `json:"receivedQuantity"`
}
type Task struct {
	ID          string    `json:"id"`
	PONumber    string    `json:"poNumber"`
	Status      Status    `json:"status"`
	WarehouseID string    `json:"warehouseId"`
	OperatorID  *string   `json:"operatorId"`
	Version     int64     `json:"version"`
	Policy      Policy    `json:"policy"`
	Lines       []Line    `json:"lines"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (t *Task) Start(operator string, base int64, now time.Time) error {
	if t.Version != base {
		return ErrVersionConflict
	}
	if t.Status == StatusCompleted {
		return ErrAlreadyCompleted
	}
	if t.OperatorID != nil && *t.OperatorID != operator {
		return ErrNotAssigned
	}
	t.OperatorID = &operator
	t.Status = StatusInProgress
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}
func (t *Task) Confirm(operator, lineID, barcode string, quantity int64, remark *string, base int64, now time.Time) (Line, error) {
	if t.Version != base {
		return Line{}, ErrVersionConflict
	}
	if t.Status == StatusCompleted {
		return Line{}, ErrAlreadyCompleted
	}
	if t.OperatorID == nil || *t.OperatorID != operator {
		return Line{}, ErrNotAssigned
	}
	for i := range t.Lines {
		line := &t.Lines[i]
		if line.ID != lineID {
			continue
		}
		if line.Barcode != barcode {
			return Line{}, ErrBarcodeWrongContext
		}
		if quantity <= 0 {
			return Line{}, ErrQuantityExceeded
		}
		remaining := line.ExpectedQuantity - line.ReceivedQuantity
		if quantity > remaining && !t.Policy.AllowOverReceipt {
			return Line{}, ErrQuantityExceeded
		}
		if t.Policy.RequireRemarkOnVariance && quantity != remaining && (remark == nil || strings.TrimSpace(*remark) == "") {
			return Line{}, ErrRemarkRequired
		}
		line.ReceivedQuantity += quantity
		t.Status = StatusPartiallyCompleted
		t.Version++
		t.UpdatedAt = now.UTC()
		return *line, nil
	}
	return Line{}, ErrBarcodeWrongContext
}
func (t *Task) Complete(operator string, base int64, now time.Time) error {
	if t.Version != base {
		return ErrVersionConflict
	}
	if t.OperatorID == nil || *t.OperatorID != operator {
		return ErrNotAssigned
	}
	for _, line := range t.Lines {
		if line.ReceivedQuantity < line.ExpectedQuantity {
			return ErrIncomplete
		}
	}
	t.Status = StatusCompleted
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}
