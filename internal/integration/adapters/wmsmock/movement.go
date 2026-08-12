package wmsmock

import (
	"github.com/company/pda-backend/internal/execution/movement/domain"
	"time"
)

func MovementTasks() []domain.Task {
	n := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	return []domain.Task{
		{ID: "PUT-001", Workflow: domain.Putaway, Status: domain.New, WarehouseID: "WH-01", Version: 1, SourceLocation: "STAGE-01", DestinationLocation: "BULK-01", ItemID: "ITEM-001", Barcode: "PUT-ITEM-001", Lot: "LOT-001", RequiredQuantity: 5, UpdatedAt: n},
		{ID: "PICK-001", Workflow: domain.Picking, Status: domain.New, WarehouseID: "WH-01", Version: 1, SourceLocation: "PICK-01", DestinationLocation: "STAGE-01", ItemID: "ITEM-001", Barcode: "PICK-ITEM-001", RequiredQuantity: 4, UpdatedAt: n},
		{ID: "REP-001", Workflow: domain.Replenishment, Status: domain.New, WarehouseID: "WH-01", Version: 1, SourceLocation: "BULK-01", DestinationLocation: "PICK-02", ItemID: "ITEM-002", Barcode: "REP-ITEM-002", RequiredQuantity: 6, UpdatedAt: n},
	}
}
