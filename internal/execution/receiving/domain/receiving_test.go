package domain

import (
	"errors"
	"testing"
	"time"
)

func sampleTask() Task {
	operator := "OP-01"
	return Task{ID: "REC-1", Status: StatusInProgress, WarehouseID: "WH-01", OperatorID: &operator, Version: 2, Policy: Policy{RequireRemarkOnVariance: true}, Lines: []Line{{ID: "L1", ItemID: "I1", Barcode: "B1", ExpectedQuantity: 3}}, UpdatedAt: time.Now()}
}
func TestQuantityPoliciesAndVersion(t *testing.T) {
	task := sampleTask()
	if _, err := task.Confirm("OP-01", "L1", "B1", 4, nil, 2, time.Now()); !errors.Is(err, ErrQuantityExceeded) {
		t.Fatalf("expected exceeded: %v", err)
	}
	remark := "damaged"
	if _, err := task.Confirm("OP-01", "L1", "B1", 1, nil, 2, time.Now()); !errors.Is(err, ErrRemarkRequired) {
		t.Fatalf("expected remark: %v", err)
	}
	line, err := task.Confirm("OP-01", "L1", "B1", 1, &remark, 2, time.Now())
	if err != nil || line.ReceivedQuantity != 1 || task.Version != 3 {
		t.Fatalf("confirm: %+v %v", task, err)
	}
	if _, err := task.Confirm("OP-01", "L1", "B1", 2, nil, 2, time.Now()); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected stale version: %v", err)
	}
}
func TestStateTransitions(t *testing.T) {
	task := Task{ID: "REC", Status: StatusNew, Version: 1, Lines: []Line{{ExpectedQuantity: 1}}}
	if err := task.Start("OP-01", 1, time.Now()); err != nil || task.Status != StatusInProgress || task.Version != 2 {
		t.Fatalf("start: %+v %v", task, err)
	}
	if err := task.Complete("OP-01", 2, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("expected incomplete: %v", err)
	}
}
