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

func TestMapReceiptTaskUsesLPNAsReceivingIdentity(t *testing.T) {
	task := mapReceiptTask(ports.Receipt{
		ReceiptID: "receipt-1", ReceiptCode: "RCV-001", LPNCode: "LPN-IN-001",
		WarehouseLocationID: "location-1",
	}, "warehouse-1")

	if task.LPNCode != "LPN-IN-001" || task.PONumber != "LPN-IN-001" {
		t.Fatalf("receiving identity = %+v, want LPN-IN-001", task)
	}
}

func TestMapReceiptTaskPreservesProductionSourceWithoutFakePurchaseOrder(t *testing.T) {
	task := mapReceiptTask(ports.Receipt{
		ReceiptID: "receipt-fg", ReceiptCode: "RCV-FG-001", WarehouseLocationID: "location-1",
		SourceType: "PRODUCTION_FINISHED_GOODS", SourceSystem: "MES", SourceDocumentType: "WORK_ORDER",
		SourceRequestID: "request-1", SourceOutputID: "output-1", SourceWOID: "wo-1", SourceWOCode: "WO-2026-001",
		SourceConfirmationID: "confirmation-1",
	}, "warehouse-1")

	if task.SourceType != "PRODUCTION_FINISHED_GOODS" || task.SourceDocumentCode != "WO-2026-001" || task.WorkOrderCode != "WO-2026-001" {
		t.Fatalf("production lineage = %+v", task)
	}
	if task.Supplier != "" || task.PONumber != "RCV-FG-001" {
		t.Fatalf("production receipt must not be presented as a fake PO: %+v", task)
	}
}

func TestMapMovementTaskUsesLPNFlowForInboundReceipt(t *testing.T) {
	task := mapMovementTask(executionTask{
		TaskID:      "putaway-1",
		TaskType:    "PUTAWAY",
		Status:      "CREATED",
		WarehouseID: "warehouse-1",
		Details: map[string]any{
			"source_type": "INBOUND_RECEIPT",
			"lpn_code":    "LPN-IN-001",
			"lot_code":    "LOT-001",
			"qty":         float64(10),
		},
	}, "PUTAWAY")

	want := []string{"LPN", "DESTINATION"}
	if len(task.ScanRequirements) != len(want) {
		t.Fatalf("scan requirements = %v, want %v", task.ScanRequirements, want)
	}
	for i := range want {
		if task.ScanRequirements[i] != want[i] {
			t.Fatalf("scan requirements = %v, want %v", task.ScanRequirements, want)
		}
	}
}

func TestMapMovementTaskUsesSourceDestinationFlowForPicking(t *testing.T) {
	task := mapMovementTask(executionTask{
		TaskID: "picking-1", TaskType: "PICKING", Status: "IN_PROGRESS", WarehouseID: "warehouse-1",
		Details: map[string]any{
			"source_location_id": "source-1", "item_code": "ITEM-001", "lot_code": "LOT-001",
			"destination_location_id": "staging-1", "qty": float64(2),
		},
	}, "PICKING")
	want := []string{"SOURCE", "DESTINATION"}
	if len(task.ScanRequirements) != len(want) {
		t.Fatalf("scan requirements = %v, want %v", task.ScanRequirements, want)
	}
	for i := range want {
		if task.ScanRequirements[i] != want[i] {
			t.Fatalf("scan requirements = %v, want %v", task.ScanRequirements, want)
		}
	}
}

func TestMapMovementTaskUsesLPNLotFlowForGroupedInboundReceipt(t *testing.T) {
	task := mapMovementTask(executionTask{
		TaskID: "putaway-group-1", TaskType: "PUTAWAY", Status: "PARTIALLY_COMPLETED", WarehouseID: "warehouse-1",
		Details: map[string]any{
			"source_type": "INBOUND_RECEIPT", "lpn_code": "LPN-IN-001",
			"lines": []any{map[string]any{"line_id": "line-1", "lot_code": "LOT-001", "qty": float64(10)}},
		},
	}, "PUTAWAY")
	want := []string{"LPN", "LOT", "DESTINATION"}
	if len(task.ScanRequirements) != len(want) {
		t.Fatalf("scan requirements = %v, want %v", task.ScanRequirements, want)
	}
	for i := range want {
		if task.ScanRequirements[i] != want[i] {
			t.Fatalf("scan requirements = %v, want %v", task.ScanRequirements, want)
		}
	}
}

func TestMapMovementTaskNormalizesLegacyRelatedTasks(t *testing.T) {
	task := mapMovementTask(executionTask{
		TaskID: "putaway-group-legacy", TaskType: "PUTAWAY", Status: "CREATED", WarehouseID: "warehouse-1",
		Details: map[string]any{
			"source_type":   "INBOUND_RECEIPT",
			"related_tasks": []any{map[string]any{"task_id": "line-1", "details": map[string]any{"lot_code": "LOT-001", "qty": float64(5)}}},
		},
	}, "PUTAWAY")
	if len(task.RelatedTasks) != 1 || task.RelatedTasks[0].Lot != "LOT-001" {
		t.Fatalf("related tasks = %+v, want one normalized line", task.RelatedTasks)
	}
}

func TestMapMovementTaskRestoresGroupedLPNValidationFromRelatedLine(t *testing.T) {
	task := mapMovementTask(executionTask{
		TaskID: "putaway-group-reopen", TaskType: "PUTAWAY", Status: "NEW", WarehouseID: "warehouse-1",
		Details: map[string]any{
			"source_type": "INBOUND_RECEIPT",
			"lpn_code":    "LPN-IN-001",
			"related_tasks": []any{
				map[string]any{"task_id": "line-1", "details": map[string]any{
					"line_id": "line-1", "lot_code": "LOT-001", "qty": float64(5),
					"scan_state": map[string]any{"LPN": true},
				}},
			},
		},
	}, "PUTAWAY")

	if !task.LPNValidated {
		t.Fatalf("LPN validation was lost when reopening grouped task: %+v", task)
	}
	if len(task.RelatedTasks) != 1 {
		t.Fatalf("related tasks = %+v, want one line", task.RelatedTasks)
	}
}

func TestMapMovementTaskPreservesGroupedPutawayLineIdentityAndState(t *testing.T) {
	task := mapMovementTask(executionTask{
		TaskID: "putaway-group-1", TaskType: "PUTAWAY", Status: "PARTIALLY_COMPLETED", WarehouseID: "warehouse-1",
		Details: map[string]any{
			"source_type": "INBOUND_RECEIPT",
			"lines": []any{
				map[string]any{
					"line_id": "line-1", "status": "COMPLETED", "completed_qty": float64(12), "qty": float64(12),
					"item_revision_id": "item-1", "lot_code": "LOT-1", "source_location_id": "receiving-1",
				},
				map[string]any{
					"line_id": "line-2", "status": "PENDING", "completed_qty": float64(0), "qty": float64(8),
					"item_revision_id": "item-2", "lot_code": "LOT-2", "source_location_id": "receiving-1",
				},
			},
		},
	}, "PUTAWAY")

	if len(task.RelatedTasks) != 2 {
		t.Fatalf("related tasks = %+v, want two line tasks", task.RelatedTasks)
	}
	completed, pending := task.RelatedTasks[0], task.RelatedTasks[1]
	if completed.ID != "line-1" || completed.LineID != "line-1" || completed.ParentTaskID != "putaway-group-1" {
		t.Fatalf("completed line identity = %+v", completed)
	}
	if completed.Status != "COMPLETED" || completed.CompletedQuantity != 12 {
		t.Fatalf("completed line state = %+v", completed)
	}
	if pending.ID != "line-2" || pending.LineID != "line-2" || pending.ParentTaskID != "putaway-group-1" {
		t.Fatalf("pending line identity = %+v", pending)
	}
	if pending.Status != "NEW" || pending.CompletedQuantity != 0 {
		t.Fatalf("pending line state = %+v, want NEW/0", pending)
	}
}

func TestMapMovementTaskCarriesActiveLocationGroup(t *testing.T) {
	task := mapMovementTask(executionTask{
		TaskID: "putaway-group-active", TaskType: "PUTAWAY", Status: "PARTIALLY_COMPLETED", WarehouseID: "warehouse-1",
		Details: map[string]any{
			"source_type":     "INBOUND_RECEIPT",
			"active_line_ids": []any{"line-1", "line-2"},
			"active_line_id":  "line-1",
			"lines": []any{
				map[string]any{"line_id": "line-1", "status": "PENDING", "qty": float64(5), "destination_location_id": "loc-1"},
				map[string]any{"line_id": "line-2", "status": "PENDING", "qty": float64(7), "destination_location_id": "loc-1"},
			},
		},
	}, "PUTAWAY")

	if len(task.ActiveLineIDs) != 2 || task.ActiveLineIDs[0] != "line-1" || task.ActiveLineIDs[1] != "line-2" {
		t.Fatalf("active line ids = %v, want [line-1 line-2]", task.ActiveLineIDs)
	}
	if len(task.RelatedTasks) != 2 {
		t.Fatalf("related tasks = %+v, want two lines", task.RelatedTasks)
	}
	if task.ID != "putaway-group-active" {
		t.Fatalf("group task identity = %q, want parent task ID", task.ID)
	}
	if task.ParentTaskID != "" {
		t.Fatalf("group task must not have a parent task ID: %q", task.ParentTaskID)
	}
}

func TestMapMovementTaskUsesLotFlowForNonInboundPutaway(t *testing.T) {
	task := mapMovementTask(executionTask{
		TaskID:      "putaway-2",
		TaskType:    "PUTAWAY",
		Status:      "CREATED",
		WarehouseID: "warehouse-1",
		Details: map[string]any{
			"source_type": "INVENTORY_TRANSFER",
			"lot_code":    "LOT-001",
		},
	}, "PUTAWAY")

	want := []string{"SOURCE", "ITEM", "LOT", "DESTINATION"}
	if len(task.ScanRequirements) != len(want) {
		t.Fatalf("scan requirements = %v, want %v", task.ScanRequirements, want)
	}
	for i := range want {
		if task.ScanRequirements[i] != want[i] {
			t.Fatalf("scan requirements = %v, want %v", task.ScanRequirements, want)
		}
	}
}
