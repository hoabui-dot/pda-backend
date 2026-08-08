package wmshttp

import (
	"context"
	"crypto/sha256"
	"fmt"
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
	rows, err := a.client.ListReceipts(ctx, ports.ReceiptQuery{Status: f.Status, Limit: f.Limit})
	if err != nil {
		return receivingports.Page{}, err
	}
	items := make([]receivingdomain.Task, 0, len(rows))
	for _, row := range rows {
		if row.Status != "Draft" || row.WarehouseLocationID == "" {
			continue
		}
		location, err := a.client.Location(ctx, row.WarehouseLocationID)
		if err != nil {
			return receivingports.Page{}, err
		}
		if location.WarehouseID != actor.WarehouseID {
			continue
		}
		items = append(items, receivingdomain.Task{ID: row.ReceiptID, OrderID: row.ReceiptID, PONumber: row.ReceiptCode, WarehouseID: actor.WarehouseID, Status: receivingdomain.StatusNew, Version: 1})
	}
	return receivingports.Page{Items: items}, nil
}

func (a *ReceivingAdapter) Detail(ctx context.Context, id string, actor platform.ActorContext) (receivingdomain.Task, error) {
	receipt, err := a.client.GetReceipt(ctx, id)
	if err != nil {
		return receivingdomain.Task{}, err
	}
	if receipt.WarehouseLocationID == "" {
		return receivingdomain.Task{}, fmt.Errorf("WAREHOUSE_ACCESS_DENIED")
	}
	location, err := a.client.Location(ctx, receipt.WarehouseLocationID)
	if err != nil {
		return receivingdomain.Task{}, err
	}
	if location.WarehouseID != actor.WarehouseID {
		return receivingdomain.Task{}, fmt.Errorf("WAREHOUSE_ACCESS_DENIED")
	}
	task := receivingdomain.Task{ID: receipt.ReceiptID, OrderID: receipt.ReceiptID, PONumber: receipt.ReceiptCode, WarehouseID: actor.WarehouseID, Status: receivingdomain.StatusNew, Version: 1, UpdatedAt: time.Now().UTC()}
	for _, line := range receipt.Lines {
		if line.LineID == "" {
			continue
		}
		task.Lines = append(task.Lines, receivingdomain.Line{ID: line.LineID, ItemID: line.ItemRevisionID, Barcode: line.ItemRevisionID, ExpectedQuantity: int64(line.Expected), ReceivedQuantity: int64(line.ReceivedQuantity), SKU: line.ItemRevisionID})
	}
	return task, nil
}

func (a *ReceivingAdapter) Claim(ctx context.Context, command receivingapp.Command) (receivingdomain.Task, error) {
	if err := a.client.ClaimReceipt(ctx, command.TaskID, command.Actor.OperatorID, command.IdempotencyKey, 900); err != nil {
		return receivingdomain.Task{}, err
	}
	return a.Detail(ctx, command.TaskID, command.Actor)
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
		if line.Barcode == barcode || line.SKU == barcode {
			return line, nil
		}
		for _, ownerLine := range mustReceiptLines(ctx, a.client, taskID) {
			if ownerLine.LineID != line.ID {
				continue
			}
			scanID := fmt.Sprintf("%x", sha256.Sum256([]byte(taskID+"|"+line.ID+"|"+barcode)))
			resolved, resolveErr := a.client.ResolveBarcode(ctx, barcode, symbology, actor.WarehouseID, taskID, line.ID, scanID)
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
