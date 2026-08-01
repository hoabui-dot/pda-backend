package domain

import (
	"errors"
	"testing"
	"time"
)

func TestCountVarianceRecountAndCompletion(t *testing.T) {
	x := CountTask{ID: "C", Version: 1, Lines: []CountLine{{ID: "L", ExpectedQuantity: 10}}}
	if e := x.Submit("OP", "L", 8, 1, time.Now()); e != nil {
		t.Fatal(e)
	}
	if x.Lines[0].Variance == nil || *x.Lines[0].Variance != -2 || !x.Lines[0].RecountRequired {
		t.Fatalf("variance %+v", x)
	}
	if e := x.Complete("OP", 2, time.Now()); !errors.Is(e, ErrIncomplete) {
		t.Fatalf("variance completed %v", e)
	}
	if e := x.Recount("OP", "L", 2, time.Now()); e != nil {
		t.Fatal(e)
	}
	if x.Lines[0].CountedQuantity != nil {
		t.Fatal("recount retained quantity")
	}
	if e := x.Submit("OP", "L", 10, 3, time.Now()); e != nil {
		t.Fatal(e)
	}
	if e := x.Complete("OP", 4, time.Now()); e != nil {
		t.Fatal(e)
	}
}
func TestCountVersionAndAssignment(t *testing.T) {
	op := "OTHER"
	x := CountTask{Version: 2, OperatorID: &op, Lines: []CountLine{{ID: "L"}}}
	if e := x.Submit("OP", "L", 1, 2, time.Now()); !errors.Is(e, ErrAssigned) {
		t.Fatal(e)
	}
	x.OperatorID = nil
	if e := x.Submit("OP", "L", 1, 1, time.Now()); !errors.Is(e, ErrVersion) {
		t.Fatal(e)
	}
}
