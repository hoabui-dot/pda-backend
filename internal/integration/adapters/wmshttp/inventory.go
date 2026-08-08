package wmshttp

import (
	"context"
	"time"

	gatewayports "github.com/company/pda-backend/internal/gateway/ports"
	inventoryapp "github.com/company/pda-backend/internal/inventory/application"
	inventorydomain "github.com/company/pda-backend/internal/inventory/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
)

// InventoryAdapter exposes authoritative WMS inventory reads. Mutations stay
// fail-closed until their owner command DTOs are mapped completely.
type InventoryAdapter struct {
	client *Client
	inventoryAdapter
}

func NewInventoryAdapter(client *Client) *InventoryAdapter {
	return &InventoryAdapter{client: client, inventoryAdapter: inventoryAdapter(Unavailable{"inventory"})}
}

func mapCycleCount(row cycleCountTask) inventorydomain.CountTask {
	lines := make([]inventorydomain.CountLine, 0, len(row.Lines))
	for _, line := range row.Lines {
		var expected int64
		if line.SnapshotQuantity != nil {
			expected = int64(*line.SnapshotQuantity)
		}
		var counted *int64
		if line.SubmittedQuantity != nil {
			v := int64(*line.SubmittedQuantity)
			counted = &v
		}
		var variance *int64
		if line.Variance != nil {
			v := int64(*line.Variance)
			variance = &v
		}
		lines = append(lines, inventorydomain.CountLine{ID: line.ID, ItemID: line.ItemID, ExpectedQuantity: expected, CountedQuantity: counted, Variance: variance, RecountRequired: line.Status == "RECOUNT_REQUIRED"})
	}
	return inventorydomain.CountTask{ID: row.ID, WarehouseID: row.WarehouseID, LocationID: row.LocationID, Status: row.Status, OperatorID: row.OperatorID, Version: row.Version, BlindCount: row.BlindCount, Lines: lines, UpdatedAt: row.UpdatedAt}
}

func (a *InventoryAdapter) ListCounts(ctx context.Context, actor platform.ActorContext) ([]inventorydomain.CountTask, error) {
	rows, err := a.client.ListCycleCounts(ctx, actor.WarehouseID, actor.OperatorID, "")
	if err != nil {
		return nil, err
	}
	out := make([]inventorydomain.CountTask, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapCycleCount(row))
	}
	return out, nil
}

func (a *InventoryAdapter) CountDetail(ctx context.Context, id string, actor platform.ActorContext) (inventorydomain.CountTask, error) {
	row, err := a.client.GetCycleCount(ctx, id, actor.WarehouseID)
	if err != nil {
		return inventorydomain.CountTask{}, err
	}
	if row.WarehouseID != actor.WarehouseID || row.OperatorID != nil && *row.OperatorID != actor.OperatorID {
		return inventorydomain.CountTask{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Count access denied"}
	}
	return mapCycleCount(row), nil
}

func (a *InventoryAdapter) ValidateCountLocation(ctx context.Context, id, location string, actor platform.ActorContext) error {
	task, err := a.CountDetail(ctx, id, actor)
	if err != nil {
		return err
	}
	if task.LocationID != location {
		return inventorydomain.ErrLocationInvalid
	}
	return nil
}

func (a *InventoryAdapter) ValidateCountItem(ctx context.Context, id, lineID, item string, actor platform.ActorContext) error {
	task, err := a.CountDetail(ctx, id, actor)
	if err != nil {
		return err
	}
	for _, line := range task.Lines {
		if line.ID == lineID && line.ItemID == item {
			return nil
		}
	}
	return inventorydomain.ErrItemNotInDocument
}

func (a *InventoryAdapter) SubmitCount(ctx context.Context, id, line string, quantity int64, command inventoryapp.Command) (inventorydomain.CountTask, error) {
	if err := a.client.SubmitCycleCount(ctx, id, line, quantity, command.BaseVersion, command.Key, command.Actor.DeviceID, command.Actor.OperatorID); err != nil {
		return inventorydomain.CountTask{}, err
	}
	return a.CountDetail(ctx, id, command.Actor)
}

func (a *InventoryAdapter) Recount(ctx context.Context, id, line string, command inventoryapp.Command) (inventorydomain.CountTask, error) {
	if err := a.client.RecountCycleCount(ctx, id, line, command.ID.String(), command.BaseVersion, command.Key); err != nil {
		return inventorydomain.CountTask{}, err
	}
	return a.CountDetail(ctx, id, command.Actor)
}

func (a *InventoryAdapter) CompleteCount(ctx context.Context, id string, command inventoryapp.Command) (inventorydomain.CountTask, error) {
	if err := a.client.CompleteCycleCount(ctx, id, command.ID.String(), command.BaseVersion, command.Key); err != nil {
		return inventorydomain.CountTask{}, err
	}
	return a.CountDetail(ctx, id, command.Actor)
}

func (a *InventoryAdapter) Search(ctx context.Context, query string, actor platform.ActorContext) ([]inventorydomain.Balance, error) {
	return a.Balances(ctx, query, "", actor)
}

func (a *InventoryAdapter) Balances(ctx context.Context, item, location string, actor platform.ActorContext) ([]inventorydomain.Balance, error) {
	rows, err := a.client.ListInventoryBalances(ctx, item, location)
	if err != nil {
		return nil, err
	}
	out := make([]inventorydomain.Balance, 0, len(rows))
	for _, row := range rows {
		out = append(out, inventorydomain.Balance{WarehouseID: actor.WarehouseID, LocationID: row.LocationID, ItemID: row.ItemRevisionID, ItemCode: row.ItemRevisionID, LocationCode: row.LocationID, Quantity: int64(row.OnHandQty), OnHand: int64(row.OnHandQty), Reserved: int64(row.ReservedQty), Available: int64(row.AvailableQty), LotNumber: row.LotCode, Version: int64(row.RowVersion), AsOf: time.Now().UTC()})
	}
	return out, nil
}

func (a *InventoryAdapter) Movements(ctx context.Context, item, cursor string, actor platform.ActorContext) ([]inventorydomain.Movement, error) {
	rows, err := a.client.ListInventoryMovements(ctx, item, "", cursor)
	if err != nil {
		return nil, err
	}
	out := make([]inventorydomain.Movement, 0, len(rows))
	for _, row := range rows {
		occurred, _ := time.Parse(time.RFC3339, row.OccurredAt)
		from, to := "", ""
		if row.FromLocationID != nil {
			from = *row.FromLocationID
		}
		if row.ToLocationID != nil {
			to = *row.ToLocationID
		}
		out = append(out, inventorydomain.Movement{ID: row.MovementID, Workflow: row.MovementType, WarehouseID: actor.WarehouseID, ItemID: row.ItemRevisionID, SourceLocation: from, DestinationLocation: to, Quantity: int64(row.Qty), OccurredAt: occurred})
	}
	return out, nil
}

func (a *InventoryAdapter) Transfer(ctx context.Context, command inventoryapp.TransferCommand) (inventorydomain.Transfer, error) {
	if command.LotID == "" {
		return inventorydomain.Transfer{}, &platform.DomainError{Code: "LOT_REQUIRED", SafeMessage: "Lot ID is required for an inventory transfer"}
	}
	result, err := a.client.TransferInventory(ctx, command.ID.String(), command.LotID, command.Source, command.Destination, command.Quantity, command.Actor.OperatorID, command.Actor.CorrelationID)
	if err != nil {
		return inventorydomain.Transfer{}, err
	}
	status := result.Result
	if status == "" {
		status = "COMPLETED"
	}
	return inventorydomain.Transfer{CommandID: result.CommandID, TransferID: result.MovementID, WarehouseID: command.Actor.WarehouseID, SourceLocation: result.FromLocationID, DestinationLocation: result.ToLocationID, ItemID: command.Item, Quantity: int64(result.Qty), Status: status, AuditID: result.CommandID, AsOf: time.Now().UTC()}, nil
}

var _ gatewayports.InventoryOperations = (*InventoryAdapter)(nil)
