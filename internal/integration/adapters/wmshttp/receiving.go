package wmshttp

import (
	"context"
	"crypto/sha256"
	"fmt"
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
	rows, err := a.client.ListReceipts(ctx, ports.ReceiptQuery{Status: f.Status, AssignedOperatorID: f.OperatorID, Limit: f.Limit})
	if err != nil {
		return receivingports.Page{}, err
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
		if !warehouseMatches(location, a.client.CanonicalWarehouse(actor.WarehouseID)) {
			continue
		}
		// Current Inbound list responses include the complete line snapshot. Use
		// it directly to avoid a per-receipt detail/location N+1 chain. Older
		// owner deployments may still return summary-only rows, so retain a
		// detail fallback for that contract shape.
		if len(row.Lines) > 0 {
			items = append(items, mapReceiptTask(ports.Receipt{
				ReceiptID: row.ReceiptID, ReceiptCode: row.ReceiptCode,
				WarehouseLocationID: row.WarehouseLocationID, Status: row.Status,
				ConfirmationStatus: row.ConfirmationStatus, AssignedOperatorID: row.AssignedOperatorID,
				AssignmentStatus: row.AssignmentStatus, AssignmentVersion: row.AssignmentVersion,
				UpdatedAt: row.UpdatedAt, Lines: row.Lines,
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
	if !warehouseMatches(location, a.client.CanonicalWarehouse(actor.WarehouseID)) {
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
	task := receivingdomain.Task{ID: receipt.ReceiptID, OrderID: receipt.ReceiptID, PONumber: receipt.ReceiptCode, WarehouseID: warehouseID, Status: receiptTaskStatus(receipt.Status, receipt.ConfirmationStatus, receipt.AssignmentStatus), OperatorID: receipt.AssignedOperatorID, Version: version, UpdatedAt: updatedAt}
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
		task.Lines = append(task.Lines, receivingdomain.Line{ID: line.LineID, ItemID: line.ItemRevisionID, Barcode: barcode, ExpectedQuantity: expected, ReceivedQuantity: received, HandedOverQuantity: expected, RemainingQuantity: expected - received, SKU: line.ItemRevisionID, UOMCode: line.UOMCode, LotCode: line.LotCode, LotRequired: line.LotCode != "", SerialRequired: false})
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
	task, err := a.Detail(ctx, command.TaskID, command.Actor)
	if err != nil {
		return receivingdomain.Task{}, err
	}
	if task.OperatorID == nil || *task.OperatorID != command.Actor.OperatorID {
		return receivingdomain.Task{}, receivingdomain.ErrNotAssigned
	}
	return task, nil
}

func (a *ReceivingAdapter) ResolveBarcode(ctx context.Context, taskID, barcode string, actor platform.ActorContext) (receivingdomain.Line, error) {
	return a.resolveBarcode(ctx, taskID, barcode, "UNKNOWN", actor)
}

func (a *ReceivingAdapter) ResolveBarcodeWithSymbology(ctx context.Context, taskID, barcode, symbology string, actor platform.ActorContext) (receivingdomain.Line, error) {
	return a.resolveBarcode(ctx, taskID, barcode, symbology, actor)
}

func (a *ReceivingAdapter) resolveBarcode(ctx context.Context, taskID, barcode, symbology string, actor platform.ActorContext) (receivingdomain.Line, error) {
	task, err := a.Detail(ctx, taskID, actor)
	if err != nil {
		return receivingdomain.Line{}, err
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
	operator := command.Actor.OperatorID
	task.OperatorID = &operator
	task.Status = receivingdomain.StatusInProgress
	task.Version++
	return task, nil
}

func (a *ReceivingAdapter) Confirm(ctx context.Context, command receivingapp.ConfirmCommand) (receivingdomain.Task, error) {
	task, err := a.Detail(ctx, command.TaskID, command.Actor)
	if err != nil {
		return task, err
	}
	var selected *ports.ReceiptLine
	for i := range task.Lines {
		if task.Lines[i].ID == command.LineID {
			for _, line := range mustReceiptLines(ctx, a.client, command.TaskID) {
				if line.LineID == command.LineID {
					selected = &line
					break
				}
			}
			break
		}
	}
	if selected == nil {
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

func (a *ReceivingAdapter) ConfirmBatch(ctx context.Context, command receivingapp.BatchConfirmCommand) (receivingdomain.Task, error) {
	lines := make([]ports.ReceiptBatchLine, 0, len(command.Lines))
	for _, line := range command.Lines {
		lines = append(lines, ports.ReceiptBatchLine{LineID: line.LineID, ActualQuantity: float64(line.ActualQuantity), UOMCode: line.UOMCode, ItemRevisionID: line.ItemRevisionID, LotCode: line.LotCode})
	}
	if len(lines) == 0 {
		return receivingdomain.Task{}, fmt.Errorf("RECEIPT_LINES_REQUIRED")
	}
	if _, err := a.client.ReceiveReceipt(ctx, command.TaskID, ports.ReceiptBatchRequest{CommandID: command.CommandID.String(), ExpectedReceiptVersion: command.BaseVersion, Lines: lines, IdempotencyKey: command.IdempotencyKey}); err != nil {
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
