package wmshttp

import (
	"testing"
	"time"

	receivingdomain "github.com/company/pda-backend/internal/execution/receiving/domain"
	"github.com/company/pda-backend/internal/integration/ports"
)

func TestMapReceivingTaskCarriesDeliverySummary(t *testing.T) {
	operator := "operator-1"
	task := mapReceivingTask(receivingdomain.Task{
		ID: "receipt-1", PONumber: "RCV-001", WarehouseID: "WH-1",
		Status: receivingdomain.StatusInProgress, OperatorID: &operator, Version: 3,
		UpdatedAt: time.Date(2026, 8, 9, 7, 35, 0, 0, time.UTC),
		Lines: []receivingdomain.Line{
			{ID: "line-1", ExpectedQuantity: 10},
			{ID: "line-2", ExpectedQuantity: 4},
		},
	})

	if task.LineCount != 2 {
		t.Fatalf("LineCount = %d, want 2", task.LineCount)
	}
	if task.PieceCount != 14 {
		t.Fatalf("PieceCount = %d, want 14", task.PieceCount)
	}
	if task.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be populated for a delivered task")
	}
	if len(task.ReceivingLines) != 2 || task.ReceivingLines[0].LineID != "line-1" {
		t.Fatalf("ReceivingLines = %+v, want delivered line snapshot", task.ReceivingLines)
	}
}

func TestMapReceiptTaskPreservesAuthoritativeItemName(t *testing.T) {
	task := mapReceiptTask(ports.Receipt{
		ReceiptID: "receipt-1", ReceiptCode: "RCV-001", WarehouseLocationID: "location-1",
		Lines: []ports.ReceiptLine{{
			LineID: "line-1", ItemRevisionID: "item-1", ItemName: "Carbon black N330 revision 1",
			LotCode: "LOT-001", Expected: 12, UOMCode: "KG",
		}},
	}, "warehouse-1")

	if len(task.Lines) != 1 || task.Lines[0].ItemName != "Carbon black N330 revision 1" {
		t.Fatalf("receiving line item name = %+v, want authoritative item name", task.Lines)
	}
}
