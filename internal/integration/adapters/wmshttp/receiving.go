package wmshttp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"time"

	receivingapp "github.com/company/pda-backend/internal/execution/receiving/application"
	receivingdomain "github.com/company/pda-backend/internal/execution/receiving/domain"
	receivingports "github.com/company/pda-backend/internal/execution/receiving/ports"
	"github.com/company/pda-backend/internal/integration/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/google/uuid"
)

// ReceivingAdapter is the gateway-level remote implementation. It maps the
// PDA receiving workflow to existing Inbound receipt contracts and owns no WMS
// transaction, repository, outbox, or inventory behavior.
type ReceivingAdapter struct{ client *Client }

func NewReceivingAdapter(client *Client) *ReceivingAdapter { return &ReceivingAdapter{client: client} }

func (a *ReceivingAdapter) List(ctx context.Context, f receivingports.Filter, actor platform.ActorContext) (receivingports.Page, error) {
	// The operator queue includes both work already assigned to this operator
	// and unassigned Draft receipts that the operator is allowed to claim. The
	// owner service remains authoritative for lease/version conflicts.
	rows, err := a.client.ListReceipts(ctx, ports.ReceiptQuery{Status: f.Status, AssignedOperatorID: f.OperatorID, Query: f.Query, Limit: f.Limit})
	if err != nil {
		return receivingports.Page{}, err
	}
	available, err := a.client.ListReceipts(ctx, ports.ReceiptQuery{Status: f.Status, Query: f.Query, Limit: f.Limit})
	if err != nil {
		return receivingports.Page{}, err
	}
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		seen[row.ReceiptID] = struct{}{}
	}
	for _, row := range available {
		if row.AssignedOperatorID == nil && row.Status == "Draft" {
			if _, exists := seen[row.ReceiptID]; !exists {
				rows = append(rows, row)
				seen[row.ReceiptID] = struct{}{}
			}
		}
	}
	items := make([]receivingdomain.Task, 0, len(rows))
	for _, row := range rows {
		// Keep completed receipts in the operator feed so the Work page can
		// render history after a refresh. The status is mapped to COMPLETED
		// below; active filtering remains available through the status query.
		if row.WarehouseLocationID == "" {
			continue
		}
		if row.AssignedOperatorID != nil && *row.AssignedOperatorID != actor.OperatorID {
			continue
		}
		location, err := a.client.Location(ctx, row.WarehouseLocationID)
		if err != nil {
			return receivingports.Page{}, err
		}
		if !a.client.WarehouseMatches(location, actor.WarehouseID) {
			continue
		}
		// Current Inbound list responses include the complete line snapshot. Use
		// it directly to avoid a per-receipt detail/location N+1 chain. Older
		// owner deployments may still return summary-only rows, so retain a
		// detail fallback for that contract shape.
		if len(row.Lines) > 0 {
			items = append(items, mapReceiptTask(ports.Receipt{
				ReceiptID: row.ReceiptID, ReceiptCode: row.ReceiptCode, LPNCode: row.LPNCode,
				WarehouseLocationID: row.WarehouseLocationID, Status: row.Status,
				ConfirmationStatus: row.ConfirmationStatus, AssignedOperatorID: row.AssignedOperatorID,
				AssignmentStatus: row.AssignmentStatus, AssignmentVersion: row.AssignmentVersion,
				UpdatedAt: row.UpdatedAt, Lines: row.Lines, SourceType: row.SourceType, SourceSystem: row.SourceSystem,
				SourceDocumentType: row.SourceDocumentType, SourceRequestID: row.SourceRequestID, SourceOutputID: row.SourceOutputID,
				SourceWOID: row.SourceWOID, SourceWOCode: row.SourceWOCode, SourceConfirmationID: row.SourceConfirmationID,
			}, actor.WarehouseID))
			continue
		}
		detail, err := a.Detail(ctx, row.ReceiptID, actor)
		if err != nil {
			return receivingports.Page{}, err
		}
		items = append(items, detail)
	}
	return receivingports.Page{Items: items}, nil
}

func (a *ReceivingAdapter) Detail(ctx context.Context, id string, actor platform.ActorContext) (receivingdomain.Task, error) {
	receipt, err := a.client.GetReceipt(ctx, id)
	if err != nil {
		return receivingdomain.Task{}, err
	}
	if receipt.WarehouseLocationID == "" {
		return receivingdomain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Receiving task has no warehouse location"}
	}
	if receipt.AssignedOperatorID != nil && *receipt.AssignedOperatorID != actor.OperatorID {
		return receivingdomain.Task{}, receivingdomain.ErrNotAssigned
	}
	location, err := a.client.Location(ctx, receipt.WarehouseLocationID)
	if err != nil {
		return receivingdomain.Task{}, err
	}
	if !a.client.WarehouseMatches(location, actor.WarehouseID) {
		return receivingdomain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Receiving task belongs to another warehouse"}
	}
	return mapReceiptTask(receipt, actor.WarehouseID), nil
}

func mapReceiptTask(receipt ports.Receipt, warehouseID string) receivingdomain.Task {
	version := receipt.AssignmentVersion
	if version < 1 {
		version = 1
	}
	updatedAt := receipt.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Unix(0, 0).UTC()
	}
	documentCode := receipt.ReceiptCode
	if receipt.SourceType == "PRODUCTION_FINISHED_GOODS" && receipt.SourceWOCode != "" {
		documentCode = receipt.SourceWOCode
	}
	lpnCode := receipt.LPNCode
	if lpnCode == "" {
		lpnCode = receipt.ReceiptCode
	}
	task := receivingdomain.Task{ID: receipt.ReceiptID, OrderID: receipt.ReceiptID, PONumber: lpnCode, LPNCode: lpnCode, Supplier: "", WarehouseID: warehouseID,
		SourceType: receipt.SourceType, SourceSystem: receipt.SourceSystem, SourceDocumentType: receipt.SourceDocumentType,
		SourceDocumentID: receipt.SourceRequestID, SourceDocumentCode: documentCode, WorkOrderID: receipt.SourceWOID, WorkOrderCode: receipt.SourceWOCode,
		ProductionOutputID: receipt.SourceOutputID, ReceiptRequestID: receipt.SourceRequestID, SourceConfirmationID: receipt.SourceConfirmationID,
		Status: receiptTaskStatus(receipt.Status, receipt.ConfirmationStatus, receipt.AssignmentStatus), OperatorID: receipt.AssignedOperatorID, Version: version, UpdatedAt: updatedAt}
	for _, line := range receipt.Lines {
		if line.LineID == "" {
			continue
		}
		barcode := line.LotCode
		if barcode == "" {
			barcode = line.ItemRevisionID
		}
		expected := int64(line.Expected)
		received := int64(line.ReceivedQuantity)
		task.Lines = append(task.Lines, receivingdomain.Line{ID: line.LineID, ItemID: line.ItemRevisionID, ItemName: line.ItemName, Barcode: barcode, ExpectedQuantity: expected, ReceivedQuantity: received, HandedOverQuantity: expected, RemainingQuantity: expected - received, SKU: line.ItemRevisionID, UOMCode: line.UOMCode, LotCode: line.LotCode, LotRequired: line.LotCode != "", SerialRequired: false})
	}
	return task
}

// warehouseMatches accepts the canonical WMS UUID and the WMS business code
// carried by the PDA session. PDA identity validation still happens before the
// adapter is called; this comparison only bridges the two representations at
// the owner-service boundary.
func warehouseMatches(location ports.Location, warehouse string) bool {
	return strings.EqualFold(strings.TrimSpace(location.WarehouseID), strings.TrimSpace(warehouse)) ||
		strings.EqualFold(strings.TrimSpace(location.WarehouseCode), strings.TrimSpace(warehouse))
}

func receiptTaskStatus(status, confirmationStatus, assignmentStatus string) receivingdomain.Status {
	if strings.EqualFold(confirmationStatus, "CONFIRMED") || strings.EqualFold(status, "CONFIRMED") {
		return receivingdomain.StatusCompleted
	}
	if strings.EqualFold(assignmentStatus, "CLAIMED") || strings.EqualFold(assignmentStatus, "IN_PROGRESS") {
		return receivingdomain.StatusInProgress
	}
	return receivingdomain.StatusNew
}

func (a *ReceivingAdapter) Claim(ctx context.Context, command receivingapp.Command) (receivingdomain.Task, error) {
	if err := a.client.ClaimReceipt(ctx, command.TaskID, command.Actor.OperatorID, command.IdempotencyKey, 900); err != nil {
		return receivingdomain.Task{}, err
	}
	return a.Detail(ctx, command.TaskID, command.Actor)
}

func (a *ReceivingAdapter) ResolveBarcode(ctx context.Context, taskID, barcode string, actor platform.ActorContext) (receivingdomain.Line, error) {
	return a.resolveBarcode(ctx, taskID, barcode, "UNKNOWN", "RECEIVING_ITEM", actor)
}

func (a *ReceivingAdapter) ResolveBarcodeWithSymbology(ctx context.Context, taskID, barcode, symbology string, actor platform.ActorContext) (receivingdomain.Line, error) {
	return a.resolveBarcode(ctx, taskID, barcode, symbology, "RECEIVING_ITEM", actor)
}

func (a *ReceivingAdapter) ResolveBarcodeWithContext(ctx context.Context, taskID, barcode, symbology, scanContext string, actor platform.ActorContext) (receivingdomain.Line, error) {
	return a.resolveBarcode(ctx, taskID, barcode, symbology, scanContext, actor)
}

func (a *ReceivingAdapter) resolveBarcode(ctx context.Context, taskID, barcode, symbology, scanContext string, actor platform.ActorContext) (receivingdomain.Line, error) {
	task, err := a.Detail(ctx, taskID, actor)
	if err != nil {
		return receivingdomain.Line{}, err
	}
	// LPN is the receipt identity. Resolve it against the authoritative task
	// snapshot before attempting item/lot barcode resolution so the PDA does
	// not send a receipt LPN through the item alias resolver.
	if task.LPNCode != "" && strings.EqualFold(strings.TrimSpace(task.LPNCode), strings.TrimSpace(barcode)) {
		for _, line := range task.Lines {
			if line.ExpectedQuantity > line.ReceivedQuantity {
				line.ReceiptVerified = true
				return line, nil
			}
		}
		return receivingdomain.Line{}, receivingdomain.ErrAlreadyCompleted
	}
	if strings.EqualFold(scanContext, "RECEIVING_LPN") {
		// The context is a client hint, not a reason to reject a normal item
		// scan. LPN equality was checked above; falling through lets older
		// clients and the first scan in a receiving session continue through
		// the authoritative item resolver.
	}
	for _, line := range task.Lines {
		// A receipt may contain multiple lines for the same item revision but
		// different lots. Never resolve a completed line again: exact lot/barcode
		// matches must be evaluated before the generic item identity fallback.
		if line.ExpectedQuantity <= line.ReceivedQuantity {
			continue
		}
		if line.Barcode == barcode {
			return line, nil
		}
	}
	for _, line := range task.Lines {
		if line.ExpectedQuantity <= line.ReceivedQuantity {
			continue
		}
		if line.SKU == barcode {
			return line, nil
		}
		for _, ownerLine := range mustReceiptLines(ctx, a.client, taskID) {
			if ownerLine.LineID != line.ID {
				continue
			}
			scanID := fmt.Sprintf("%x", sha256.Sum256([]byte(taskID+"|"+line.ID+"|"+barcode)))
			resolved, resolveErr := a.client.ResolveBarcode(ctx, barcode, symbology, a.client.CanonicalWarehouse(actor.WarehouseID), taskID, line.ID, scanID)
			if resolveErr != nil {
				return receivingdomain.Line{}, resolveErr
			}
			if resolved.ItemID != line.ItemID {
				return receivingdomain.Line{}, receivingdomain.ErrBarcodeWrongContext
			}
			line.Barcode = barcode
			return line, nil
		}
	}
	return receivingdomain.Line{}, receivingdomain.ErrBarcodeUnknown
}

func (a *ReceivingAdapter) Start(ctx context.Context, command receivingapp.Command) (receivingdomain.Task, error) {
	task, err := a.Detail(ctx, command.TaskID, command.Actor)
	if err != nil {
		return task, err
	}
	if task.OperatorID == nil || *task.OperatorID != command.Actor.OperatorID {
		return receivingdomain.Task{}, receivingdomain.ErrNotAssigned
	}
	if err := task.Start(command.Actor.OperatorID, command.BaseVersion, time.Now().UTC()); err != nil {
		return receivingdomain.Task{}, err
	}
	return task, nil
}

func (a *ReceivingAdapter) Confirm(ctx context.Context, command receivingapp.ConfirmCommand) (receivingdomain.Task, error) {
	// A successful owner confirmation releases the receiving assignment. On an
	// exact retry the receipt is therefore no longer executable by assignment,
	// but the owner batch endpoint can still replay the same business payload
	// idempotently. Check that state before applying the normal assigned-task
	// mutation path.
	ownerReceipt, ownerErr := a.client.GetReceipt(ctx, command.TaskID)
	if ownerErr != nil {
		return receivingdomain.Task{}, ownerErr
	}
	if strings.EqualFold(ownerReceipt.Status, "CONFIRMED") || strings.EqualFold(ownerReceipt.ConfirmationStatus, "CONFIRMED") {
		var selected *ports.ReceiptLine
		for i := range ownerReceipt.Lines {
			if ownerReceipt.Lines[i].LineID == command.LineID {
				selected = &ownerReceipt.Lines[i]
				break
			}
		}
		if selected == nil || normalizeBarcode(selected.LotCode) != normalizeBarcode(command.Barcode) {
			return receivingdomain.Task{}, receivingdomain.ErrBarcodeWrongContext
		}
		if _, err := a.client.ReceiveReceipt(ctx, command.TaskID, ports.ReceiptBatchRequest{
			CommandID:              command.CommandID.String(),
			ExpectedReceiptVersion: ownerReceipt.AssignmentVersion,
			Lines:                  []ports.ReceiptBatchLine{{LineID: selected.LineID, ActualQuantity: float64(command.Quantity), UOMCode: selected.UOMCode, ItemRevisionID: selected.ItemRevisionID, LotCode: selected.LotCode}},
			IdempotencyKey:         command.IdempotencyKey,
		}); err != nil {
			return receivingdomain.Task{}, err
		}
		return a.Detail(ctx, command.TaskID, command.Actor)
	}
	task, err := a.Detail(ctx, command.TaskID, command.Actor)
	if err != nil {
		return task, err
	}
	if task.OperatorID == nil || *task.OperatorID != command.Actor.OperatorID || (task.Status != receivingdomain.StatusInProgress && task.Status != receivingdomain.StatusPartiallyCompleted) {
		return receivingdomain.Task{}, receivingdomain.ErrNotAssigned
	}
	var taskLine *receivingdomain.Line
	for i := range task.Lines {
		if task.Lines[i].ID == command.LineID {
			taskLine = &task.Lines[i]
			break
		}
	}
	if taskLine == nil {
		slog.Warn("pda receiving line not found", "task_id", command.TaskID, "line_id", command.LineID)
		return receivingdomain.Task{}, receivingdomain.ErrBarcodeWrongContext
	}
	receipt, err := a.client.GetReceipt(ctx, command.TaskID)
	if err != nil {
		return receivingdomain.Task{}, err
	}
	var selected *ports.ReceiptLine
	for i := range receipt.Lines {
		if receipt.Lines[i].LineID == command.LineID {
			selected = &receipt.Lines[i]
			break
		}
	}
	if selected == nil {
		slog.Warn("pda receiving owner line not found", "task_id", command.TaskID, "line_id", command.LineID)
		return receivingdomain.Task{}, receivingdomain.ErrBarcodeWrongContext
	}
	if normalizeBarcode(taskLine.Barcode) != normalizeBarcode(command.Barcode) {
		slog.Warn("pda receiving barcode context rejected", "task_id", command.TaskID, "line_id", command.LineID, "expected_barcode", taskLine.Barcode, "received_barcode", command.Barcode)
		return receivingdomain.Task{}, receivingdomain.ErrBarcodeWrongContext
	}
	_, err = a.client.RecordReceiptQuantity(ctx, command.TaskID, command.LineID, ports.ReceiptQuantityRequest{ActualQuantity: float64(command.Quantity), UOMCode: selected.UOMCode, ExpectedVersion: selected.Version, ItemRevisionID: selected.ItemRevisionID, LotCode: selected.LotCode, IdempotencyKey: command.IdempotencyKey})
	if err != nil {
		return receivingdomain.Task{}, err
	}
	if _, err = a.client.ConfirmReceipt(ctx, command.TaskID, command.IdempotencyKey); err != nil {
		return receivingdomain.Task{}, err
	}
	// Confirmation may apply approval/tolerance policy and inventory posting in
	// Inbound. Reload after the owner command instead of synthesizing a PDA
	// quantity or status from the request payload.
	return a.Detail(ctx, command.TaskID, command.Actor)
}

func normalizeBarcode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func (a *ReceivingAdapter) ConfirmBatch(ctx context.Context, command receivingapp.BatchConfirmCommand) (receivingdomain.Task, error) {
	ownerReceipt, ownerErr := a.client.GetReceipt(ctx, command.TaskID)
	if ownerErr != nil {
		return receivingdomain.Task{}, ownerErr
	}
	if strings.EqualFold(ownerReceipt.Status, "CONFIRMED") || strings.EqualFold(ownerReceipt.ConfirmationStatus, "CONFIRMED") {
		lines := make([]ports.ReceiptBatchLine, 0, len(command.Lines))
		for _, line := range command.Lines {
			var ownerLine *ports.ReceiptLine
			for i := range ownerReceipt.Lines {
				if ownerReceipt.Lines[i].LineID == line.LineID {
					ownerLine = &ownerReceipt.Lines[i]
					break
				}
			}
			if ownerLine == nil {
				return receivingdomain.Task{}, receivingdomain.ErrBarcodeWrongContext
			}
			lines = append(lines, ports.ReceiptBatchLine{LineID: line.LineID, ActualQuantity: float64(line.ActualQuantity), UOMCode: line.UOMCode, ItemRevisionID: line.ItemRevisionID, LotCode: line.LotCode})
		}
		if _, err := a.client.ReceiveReceipt(ctx, command.TaskID, ports.ReceiptBatchRequest{CommandID: command.CommandID.String(), ExpectedReceiptVersion: ownerReceipt.AssignmentVersion, Lines: lines, IdempotencyKey: command.IdempotencyKey}); err != nil {
			return receivingdomain.Task{}, err
		}
		return a.Detail(ctx, command.TaskID, command.Actor)
	}
	task, err := a.Detail(ctx, command.TaskID, command.Actor)
	if err != nil {
		return receivingdomain.Task{}, err
	}
	if task.OperatorID == nil || *task.OperatorID != command.Actor.OperatorID || (task.Status != receivingdomain.StatusInProgress && task.Status != receivingdomain.StatusPartiallyCompleted) {
		return receivingdomain.Task{}, receivingdomain.ErrNotAssigned
	}
	lines := make([]ports.ReceiptBatchLine, 0, len(command.Lines))
	for _, line := range command.Lines {
		lines = append(lines, ports.ReceiptBatchLine{LineID: line.LineID, ActualQuantity: float64(line.ActualQuantity), UOMCode: line.UOMCode, ItemRevisionID: line.ItemRevisionID, LotCode: line.LotCode})
	}
	if len(lines) == 0 {
		return receivingdomain.Task{}, fmt.Errorf("RECEIPT_LINES_REQUIRED")
	}
	// Inbound versions the assignment/receipt owner state independently from
	// the PDA task projection. Use the authoritative owner version instead of
	// forwarding the PDA workflow version, which may advance on START.
	if _, err := a.client.ReceiveReceipt(ctx, command.TaskID, ports.ReceiptBatchRequest{CommandID: command.CommandID.String(), ExpectedReceiptVersion: ownerReceipt.AssignmentVersion, Lines: lines, IdempotencyKey: command.IdempotencyKey}); err != nil {
		return receivingdomain.Task{}, err
	}
	return a.Detail(ctx, command.TaskID, command.Actor)
}

func mustReceiptLines(ctx context.Context, client *Client, id string) []ports.ReceiptLine {
	r, err := client.GetReceipt(ctx, id)
	if err != nil {
		return nil
	}
	return r.Lines
}

func (a *ReceivingAdapter) Complete(ctx context.Context, command receivingapp.Command) (receivingdomain.Task, error) {
	return a.Detail(ctx, command.TaskID, command.Actor)
}

func (a *ReceivingAdapter) CommandStatus(context.Context, uuid.UUID, platform.ActorContext) (receivingports.CommandStatus, error) {
	return receivingports.CommandStatus{}, fmt.Errorf("COMMAND_STATUS_UNAVAILABLE_FROM_INBOUND")
}
