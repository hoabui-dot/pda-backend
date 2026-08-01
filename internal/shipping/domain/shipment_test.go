package domain

import (
	"errors"
	"testing"
	"time"
)

func TestReadinessAndConfirmationGuards(t *testing.T) {
	s := Shipment{Version: 1, Packages: []Package{{Status: "PENDING"}}}
	if e := s.Confirm("DHL", "TRACK-1", 1, time.Now()); !errors.Is(e, ErrNotReady) {
		t.Fatal(e)
	}
	s.PickingComplete = true
	if e := s.Confirm("DHL", "TRACK-1", 1, time.Now()); !errors.Is(e, ErrPackage) {
		t.Fatal(e)
	}
	s.Packages[0].Status = "COMPLETED"
	if e := s.Confirm("BAD", "TRACK-1", 1, time.Now()); !errors.Is(e, ErrCarrier) {
		t.Fatal(e)
	}
	if e := s.Confirm("DHL", "x", 1, time.Now()); !errors.Is(e, ErrTracking) {
		t.Fatal(e)
	}
	if e := s.Confirm("DHL", "TRACK-1", 2, time.Now()); !errors.Is(e, ErrVersion) {
		t.Fatal(e)
	}
	if e := s.Confirm("DHL", "TRACK-1", 1, time.Now()); e != nil {
		t.Fatal(e)
	}
}
