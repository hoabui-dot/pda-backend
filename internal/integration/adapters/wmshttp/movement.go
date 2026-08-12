package wmshttp

import (
	"context"

	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	movementdomain "github.com/company/pda-backend/internal/execution/movement/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
)

type movementAdapter struct {
	client   *Client
	workflow string
}

func newMovementAdapter(client *Client, workflow string) *movementAdapter {
	return &movementAdapter{client: client, workflow: workflow}
}

func (a *movementAdapter) list(ctx context.Context, actor platform.ActorContext) ([]movementdomain.Task, error) {
	rows, err := a.client.ListExecutionTasks(ctx, actor.WarehouseID, actor.OperatorID, a.workflow, "", "", 100)
	if err != nil {
		return nil, err
	}
	out := make([]movementdomain.Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapMovementTask(row, a.workflow))
	}
	return out, nil
}
func (a *movementAdapter) detail(ctx context.Context, id string, actor platform.ActorContext) (movementdomain.Task, error) {
	row, err := a.client.GetExecutionTask(ctx, id)
	if err != nil {
		return movementdomain.Task{}, err
	}
	if row.WarehouseID != actor.WarehouseID || row.AssignedOperatorID != nil && *row.AssignedOperatorID != actor.OperatorID {
		return movementdomain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Movement task access denied"}
	}
	return mapMovementTask(row, a.workflow), nil
}
func (a *movementAdapter) scan(ctx context.Context, c movementapp.Command, typ, value string) (movementdomain.Task, error) {
	if err := a.client.RecordExecutionScan(ctx, c.TaskID, typ, value, c.BaseVersion, c.Actor.CorrelationID); err != nil {
		return movementdomain.Task{}, err
	}
	return a.detail(ctx, c.TaskID, c.Actor)
}
func (a *movementAdapter) confirm(ctx context.Context, c movementapp.Command, quantity int64) (movementdomain.Task, error) {
	row, err := a.client.ApplyExecutionTaskCommand(ctx, c.TaskID, executionTaskCommand{CommandID: c.CommandID.String(), CommandType: "CONFIRM", ExpectedVersion: c.BaseVersion, CorrelationID: c.Actor.CorrelationID, CausationID: c.CommandID.String(), ConfirmQty: float64(quantity)}, c.Actor.OperatorID, c.IdempotencyKey)
	if err != nil {
		return movementdomain.Task{}, err
	}
	return mapMovementTask(row, a.workflow), nil
}
func mapMovementTask(row executionTask, workflow string) movementdomain.Task {
	status := movementdomain.Status(row.Status)
	if status == "CLAIMED" {
		status = movementdomain.InProgress
	}
	if status == "CREATED" {
		status = movementdomain.New
	}
	lot := stringValue(row.Details, "lot_code", "lot_id")
	requirements := []string{"SOURCE", "ITEM", "DESTINATION"}
	if lot != "" {
		requirements = []string{"SOURCE", "ITEM", "LOT", "DESTINATION"}
	}
	return movementdomain.Task{ID: row.TaskID, Workflow: movementdomain.Workflow(workflow), Status: status, WarehouseID: row.WarehouseID, OperatorID: row.AssignedOperatorID, Version: row.Version, UpdatedAt: row.UpdatedAt, RequiredQuantity: int64(number(row.Details, "qty", "quantity", "required_quantity")), CompletedQuantity: int64(number(row.Details, "completed_qty", "picked_qty")), SourceLocation: stringValue(row.Details, "source_location_id", "source_location"), DestinationLocation: stringValue(row.Details, "destination_location_id", "destination_location"), ItemID: stringValue(row.Details, "item_revision_id", "item_id"), Barcode: stringValue(row.Details, "barcode", "item_code"), Lot: lot, ScanRequirements: requirements}
}
func number(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			return v
		}
	}
	return 0
}
func stringValue(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

type PutawayAdapter struct{ *movementAdapter }

func NewPutawayAdapter(c *Client) *PutawayAdapter {
	return &PutawayAdapter{newMovementAdapter(c, "PUTAWAY")}
}
func (a *PutawayAdapter) List(c context.Context, x platform.ActorContext) ([]movementdomain.Task, error) {
	return a.list(c, x)
}
func (a *PutawayAdapter) Detail(c context.Context, id string, x platform.ActorContext) (movementdomain.Task, error) {
	return a.detail(c, id, x)
}
func (a *PutawayAdapter) Suggestions(context.Context, string, platform.ActorContext) ([]movementdomain.Location, error) {
	return nil, &platform.DomainError{Code: "UPSTREAM_OPERATION_NOT_IMPLEMENTED", SafeMessage: "WMS putaway destination suggestion is not mapped"}
}
func (a *PutawayAdapter) ValidateSource(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "SOURCE", v)
}
func (a *PutawayAdapter) ValidateItem(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "ITEM", v)
}
func (a *PutawayAdapter) ValidateLot(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "LOT", v)
}
func (a *PutawayAdapter) ValidateDestination(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "DESTINATION", v)
}
func (a *PutawayAdapter) Confirm(c context.Context, x movementapp.Command, q int64) (movementdomain.Task, error) {
	return a.confirm(c, x, q)
}

type PickingAdapter struct{ *movementAdapter }

func NewPickingAdapter(c *Client) *PickingAdapter {
	return &PickingAdapter{newMovementAdapter(c, "PICKING")}
}
func (a *PickingAdapter) List(c context.Context, x platform.ActorContext) ([]movementdomain.Task, error) {
	return a.list(c, x)
}
func (a *PickingAdapter) Detail(c context.Context, id string, x platform.ActorContext) (movementdomain.Task, error) {
	return a.detail(c, id, x)
}
func (a *PickingAdapter) Allocate(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	row, err := a.client.AllocateExecutionTask(c, x.TaskID, x.CommandID.String(), x.Actor.CorrelationID, x.Actor.OperatorID, x.IdempotencyKey)
	if err != nil {
		return movementdomain.Task{}, err
	}
	if row.WarehouseID != x.Actor.WarehouseID || row.AssignedOperatorID != nil && *row.AssignedOperatorID != x.Actor.OperatorID {
		return movementdomain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Picking task access denied"}
	}
	return mapMovementTask(row, "PICKING"), nil
}
func (a *PickingAdapter) ValidateLocation(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "SOURCE", v)
}
func (a *PickingAdapter) ResolveBarcode(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "ITEM", v)
}
func (a *PickingAdapter) Confirm(c context.Context, x movementapp.Command, q int64) (movementdomain.Task, error) {
	return a.confirm(c, x, q)
}
func (a *PickingAdapter) Complete(context.Context, movementapp.Command) (movementdomain.Task, error) {
	return movementdomain.Task{}, &platform.DomainError{Code: "UPSTREAM_OPERATION_NOT_IMPLEMENTED", SafeMessage: "WMS picking completion is not mapped"}
}

type ReplenishmentAdapter struct{ *movementAdapter }

func NewReplenishmentAdapter(c *Client) *ReplenishmentAdapter {
	return &ReplenishmentAdapter{newMovementAdapter(c, "REPLENISHMENT")}
}
func (a *ReplenishmentAdapter) List(c context.Context, x platform.ActorContext) ([]movementdomain.Task, error) {
	return a.list(c, x)
}
func (a *ReplenishmentAdapter) Detail(c context.Context, id string, x platform.ActorContext) (movementdomain.Task, error) {
	return a.detail(c, id, x)
}
func (a *ReplenishmentAdapter) ValidateSource(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "SOURCE", v)
}
func (a *ReplenishmentAdapter) ValidateDestination(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "DESTINATION", v)
}
func (a *ReplenishmentAdapter) ValidateItem(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "ITEM", v)
}
func (a *ReplenishmentAdapter) Confirm(c context.Context, x movementapp.Command, q int64) (movementdomain.Task, error) {
	return a.confirm(c, x, q)
}
