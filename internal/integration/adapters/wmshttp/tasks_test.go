package wmshttp

import (
	"testing"
	"time"

	receivingdomain "github.com/company/pda-backend/internal/execution/receiving/domain"
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
