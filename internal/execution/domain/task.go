package domain

import (
	"errors"
	"time"

	platform "github.com/company/pda-backend/internal/platform/domain"
)

type TaskStatus string

const (
	StatusNew        TaskStatus = "NEW"
	StatusAssigned   TaskStatus = "ASSIGNED"
	StatusInProgress TaskStatus = "IN_PROGRESS"
	StatusCompleted  TaskStatus = "COMPLETED"
	StatusOnHold     TaskStatus = "ON_HOLD"
)

type TaskCategory string

const (
	CategoryReceiving     TaskCategory = "RECEIVING"
	CategoryPutaway       TaskCategory = "PUTAWAY"
	CategoryPicking       TaskCategory = "PICKING"
	CategoryReplenishment TaskCategory = "REPLENISHMENT"
	CategoryCycleCount    TaskCategory = "CYCLE_COUNT"
)

var (
	ErrTaskLocked      = &platform.DomainError{Code: "TASK_LOCKED", SafeMessage: "Task is assigned to another operator"}
	ErrTaskNotAssigned = &platform.DomainError{Code: "TASK_NOT_ASSIGNED", SafeMessage: "Task is not assigned to this operator"}
	ErrVersionConflict = &platform.DomainError{Code: "TASK_VERSION_CONFLICT", SafeMessage: "Task version has changed"}
	ErrTaskNotFound    = &platform.DomainError{Code: "TASK_NOT_FOUND", SafeMessage: "Task not found"}
)

type Task struct {
	ID          string       `json:"id"`
	Category    TaskCategory `json:"category"`
	Status      TaskStatus   `json:"status"`
	Priority    int          `json:"priority"`
	WarehouseID string       `json:"warehouseId"`
	OperatorID  *string      `json:"operatorId"`
	Version     int64        `json:"version"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

func (t *Task) Claim(operatorID string, baseVersion int64, now time.Time) error {
	if t.Version != baseVersion {
		return ErrVersionConflict
	}
	if t.OperatorID != nil && *t.OperatorID != operatorID {
		return ErrTaskLocked
	}
	if t.OperatorID != nil && *t.OperatorID == operatorID && t.Status == StatusAssigned {
		return nil
	}
	t.OperatorID = &operatorID
	t.Status = StatusAssigned
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}

func (t *Task) Release(operatorID string, baseVersion int64, now time.Time) error {
	if t.Version != baseVersion {
		return ErrVersionConflict
	}
	if t.OperatorID == nil || *t.OperatorID != operatorID {
		return ErrTaskNotAssigned
	}
	t.OperatorID = nil
	t.Status = StatusNew
	t.Version++
	t.UpdatedAt = now.UTC()
	return nil
}

func Is(err, target error) bool { return errors.Is(err, target) }
