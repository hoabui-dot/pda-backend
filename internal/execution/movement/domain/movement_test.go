package domain

import (
	"errors"
	"testing"
	"time"
)

func task(w Workflow) Task {
	return Task{ID: "T", Workflow: w, Status: New, WarehouseID: "WH", Version: 1, SourceLocation: "SRC", DestinationLocation: "DST", ItemID: "ITEM", Barcode: "CODE", RequiredQuantity: 6}
}
func TestExplicitWorkflowValidationAndPartialReplenishment(t *testing.T) {
	now := time.Now()
	r := task(Replenishment)
	if e := r.ValidateSource("OP", "WRONG", 1, now); !errors.Is(e, ErrSource) {
		t.Fatalf("wrong source %v", e)
	}
	if e := r.ValidateSource("OP", "SRC", 1, now); e != nil {
		t.Fatal(e)
	}
	if e := r.ValidateDestination("OP", "WRONG", 2, now); !errors.Is(e, ErrDestination) {
		t.Fatalf("wrong destination %v", e)
	}
	if e := r.ValidateDestination("OP", "DST", 2, now); e != nil {
		t.Fatal(e)
	}
	if e := r.ValidateItem("OP", "WRONG", 3, now); !errors.Is(e, ErrItem) {
		t.Fatalf("wrong item %v", e)
	}
	if e := r.ValidateItem("OP", "CODE", 3, now); e != nil {
		t.Fatal(e)
	}
	if e := r.Move("OP", 2, 4, now); e != nil {
		t.Fatal(e)
	}
	if r.Status != PartiallyCompleted || r.Remaining() != 4 {
		t.Fatalf("partial state %+v", r)
	}
	if e := r.GuardCompletion("OP", 5); e != nil {
		t.Fatal(e)
	}
	if r.Remaining() == 0 {
		t.Fatal("partial replenishment incorrectly complete")
	}
	if e := r.Move("OP", 5, 5, now); !errors.Is(e, ErrQuantity) {
		t.Fatalf("quantity %v", e)
	}
}
func TestAssignmentVersionAndSequence(t *testing.T) {
	p := task(Putaway)
	if e := p.ValidateDestination("OP", "DST", 1, time.Now()); !errors.Is(e, ErrSequence) {
		t.Fatalf("sequence %v", e)
	}
	if e := p.ValidateSource("OP", "SRC", 1, time.Now()); e != nil {
		t.Fatal(e)
	}
	if e := p.ValidateDestination("OTHER", "DST", 2, time.Now()); !errors.Is(e, ErrNotAssigned) {
		t.Fatalf("lock %v", e)
	}
	if e := p.ValidateDestination("OP", "DST", 1, time.Now()); !errors.Is(e, ErrVersion) {
		t.Fatalf("version %v", e)
	}
}
