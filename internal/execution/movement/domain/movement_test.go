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
	if e := r.Claim("OP", 1, now); e != nil {
		t.Fatal(e)
	}
	if e := r.Start("OP", 2, now); e != nil {
		t.Fatal(e)
	}
	if e := r.ValidateSource("OP", "WRONG", 3, now); !errors.Is(e, ErrSource) {
		t.Fatalf("wrong source %v", e)
	}
	if e := r.ValidateSource("OP", "SRC", 3, now); e != nil {
		t.Fatal(e)
	}
	if e := r.ValidateDestination("OP", "WRONG", 4, now); !errors.Is(e, ErrDestination) {
		t.Fatalf("wrong destination %v", e)
	}
	if e := r.ValidateDestination("OP", "DST", 4, now); e != nil {
		t.Fatal(e)
	}
	if e := r.ValidateItem("OP", "WRONG", 5, now); !errors.Is(e, ErrItem) {
		t.Fatalf("wrong item %v", e)
	}
	if e := r.ValidateItem("OP", "CODE", 5, now); e != nil {
		t.Fatal(e)
	}
	if e := r.Move("OP", 2, 6, now); e != nil {
		t.Fatal(e)
	}
	if r.Status != PartiallyCompleted || r.Remaining() != 4 {
		t.Fatalf("partial state %+v", r)
	}
	if e := r.GuardCompletion("OP", 7); e != nil {
		t.Fatal(e)
	}
	if r.Remaining() == 0 {
		t.Fatal("partial replenishment incorrectly complete")
	}
	if e := r.Move("OP", 5, 7, now); !errors.Is(e, ErrQuantity) {
		t.Fatalf("quantity %v", e)
	}
}
func TestAssignmentVersionAndSequence(t *testing.T) {
	p := task(Putaway)
	now := time.Now()
	if e := p.Claim("OP", 1, now); e != nil {
		t.Fatal(e)
	}
	if e := p.Start("OP", 2, now); e != nil {
		t.Fatal(e)
	}
	if e := p.ValidateDestination("OP", "DST", 3, now); !errors.Is(e, ErrSequence) {
		t.Fatalf("sequence %v", e)
	}
	if e := p.ValidateSource("OP", "SRC", 3, now); e != nil {
		t.Fatal(e)
	}
	if e := p.ValidateDestination("OTHER", "DST", 4, now); !errors.Is(e, ErrNotAssigned) {
		t.Fatalf("lock %v", e)
	}
	if e := p.ValidateDestination("OP", "DST", 3, now); !errors.Is(e, ErrVersion) {
		t.Fatalf("version %v", e)
	}
}

func TestPutawayLotIsValidatedAfterItemBeforeDestination(t *testing.T) {
	p := task(Putaway)
	p.Lot = "LOT-001"
	now := time.Now()
	if err := p.Claim("OP", 1, now); err != nil {
		t.Fatal(err)
	}
	if err := p.Start("OP", 2, now); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateSource("OP", "SRC", 3, now); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateItem("OP", "CODE", 4, now); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateLot("OP", "WRONG-LOT", 5, now); err == nil {
		t.Fatal("wrong lot was accepted")
	}
	if err := p.ValidateLot("OP", "LOT-001", 5, now); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateDestination("OP", "DST", 6, now); err != nil {
		t.Fatal(err)
	}
}

func TestPickingMoveRequiresSourceAndDestinationOnly(t *testing.T) {
	p := task(Picking)
	p.DestinationLocation = "STAGING"
	now := time.Now()
	p.OperatorID = ptr("OP")
	p.Status = InProgress
	p.Version = 1
	if err := p.ValidateSource("OP", "SRC", 1, now); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateDestination("OP", "STAGING", 2, now); err != nil {
		t.Fatal(err)
	}
	if err := p.Move("OP", 6, 3, now); err != nil {
		t.Fatal(err)
	}
	if p.Status != Completed || p.CompletedQuantity != 6 {
		t.Fatalf("unexpected picking result: %+v", p)
	}
}

func ptr(value string) *string { return &value }
