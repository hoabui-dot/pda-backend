package wmshttp

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"log"
	"strings"

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
	// Unassigned work is claimable work. Filtering by operator at the WMS
	// query boundary hides CREATED tasks before the operator can claim them.
	// Fetch the warehouse-scoped queue, then keep only unassigned tasks or
	// tasks already owned by this operator.
	rows, err := a.client.ListExecutionTasks(ctx, actor.WarehouseID, "", a.workflow, "", "", 100)
	if err != nil {
		return nil, err
	}
	out := make([]movementdomain.Task, 0, len(rows))
	for _, row := range rows {
		// Production Picking is supervisor-assigned work. It may be projected
		// downstream before assignment, but it is not an operator mission until
		// WEX has assigned it to this operator.
		if a.workflow == "PICKING" && row.AssignedOperatorID == nil {
			continue
		}
		if row.AssignedOperatorID != nil && *row.AssignedOperatorID != actor.OperatorID {
			continue
		}
		switch row.Status {
		case "CREATED", "CLAIMED", "IN_PROGRESS", "PARTIALLY_COMPLETED":
		default:
			continue
		}
		task := mapMovementTask(row, a.workflow)
		if err := a.enrichTaskLocations(ctx, &task); err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, nil
}
func (a *movementAdapter) detail(ctx context.Context, id string, actor platform.ActorContext) (movementdomain.Task, error) {
	row, err := a.client.GetExecutionTask(ctx, id)
	if err != nil {
		return movementdomain.Task{}, err
	}
	warehouseID, err := a.client.CanonicalWarehouseID(ctx, actor.WarehouseID)
	if err != nil {
		return movementdomain.Task{}, err
	}
	if warehouseID != "" && row.WarehouseID != warehouseID || row.AssignedOperatorID != nil && *row.AssignedOperatorID != actor.OperatorID {
		return movementdomain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Movement task access denied"}
	}
	if a.workflow == "PICKING" && row.AssignedOperatorID == nil {
		return movementdomain.Task{}, movementdomain.ErrNotAssigned
	}
	task := mapMovementTask(row, a.workflow)
	a.enrichReceiptCode(ctx, &task)
	if a.workflow == "PUTAWAY" {
		receiptID := stringValue(row.Details, "source_receipt_id")
		if receiptID != "" && len(movementLineDetails(row.Details)) == 0 {
			// A single inbound LPN can contain lines routed to different
			// warehouses. Return the operator's sibling putaway tasks so the
			// PDA can render the complete receipt scope. Each task remains an
			// independent Ledger/movement boundary for confirmation.
			// Keep the sibling lookup in the task's canonical warehouse scope.
			// Passing an empty warehouse here makes CanonicalWarehouseID reject the
			// request, turning an otherwise valid detail read into HTTP 500.
			rows, relatedErr := a.client.ListExecutionTasks(ctx, row.WarehouseID, actor.OperatorID, a.workflow, "", receiptID, 100)
			if relatedErr != nil {
				return movementdomain.Task{}, relatedErr
			}
			for _, related := range rows {
				if related.AssignedOperatorID == nil || *related.AssignedOperatorID != actor.OperatorID {
					continue
				}
				task.RelatedTasks = append(task.RelatedTasks, mapMovementTask(related, a.workflow))
			}
		}
	}
	if err := a.enrichTaskLocations(ctx, &task); err != nil {
		return movementdomain.Task{}, err
	}
	return task, nil
}

// enrichTaskLocations resolves operator-facing names and codes through the
// Master Data owner. WMS execution details intentionally carry IDs as stable
// identities, but PDA must not expose those UUIDs as physical locations.
func (a *movementAdapter) enrichTaskLocations(ctx context.Context, task *movementdomain.Task) error {
	if task.SourceLocationID != "" {
		location, err := a.client.Location(ctx, task.SourceLocationID)
		if err != nil {
			return fmt.Errorf("resolve source location %s: %w", task.SourceLocationID, err)
		}
		task.SourceLocationCode = firstNonBlank(task.SourceLocationCode, location.Code)
		task.SourceLocationName = location.Name
	}
	if task.DestinationLocationID != "" {
		location, err := a.client.Location(ctx, task.DestinationLocationID)
		if err != nil {
			return fmt.Errorf("resolve destination location %s: %w", task.DestinationLocationID, err)
		}
		task.DestinationLocationCode = firstNonBlank(task.DestinationLocationCode, location.Code)
		task.DestinationLocationName = location.Name
	}
	for index := range task.RelatedTasks {
		if err := a.enrichTaskLocations(ctx, &task.RelatedTasks[index]); err != nil {
			return err
		}
	}
	return nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func (a *movementAdapter) scan(ctx context.Context, c movementapp.Command, typ, value string) (movementdomain.Task, error) {
	log.Printf("[PDA][%s] scan_forward task_id=%s scan_type=%s base_version=%d operator_id=%s value=%s", a.workflow, c.TaskID, typ, c.BaseVersion, c.Actor.OperatorID, value)
	if err := a.client.RecordExecutionScan(ctx, c.TaskID, typ, value, c.BaseVersion, c.Actor.OperatorID, c.Actor.CorrelationID); err != nil {
		log.Printf("[PDA][%s] scan_rejected task_id=%s scan_type=%s error=%v", a.workflow, c.TaskID, typ, err)
		return movementdomain.Task{}, err
	}
	return a.detail(ctx, c.TaskID, c.Actor)
}
func (a *movementAdapter) confirm(ctx context.Context, c movementapp.Command, quantity int64) (movementdomain.Task, error) {
	log.Printf("[PDA][%s] confirm_forward task_id=%s base_version=%d quantity=%d operator_id=%s", a.workflow, c.TaskID, c.BaseVersion, quantity, c.Actor.OperatorID)
	row, err := a.client.ApplyExecutionTaskCommand(ctx, c.TaskID, executionTaskCommand{CommandID: c.CommandID.String(), CommandType: "CONFIRM", ExpectedVersion: c.BaseVersion, CorrelationID: c.Actor.CorrelationID, CausationID: c.CommandID.String(), ConfirmQty: float64(quantity)}, c.Actor.OperatorID, c.IdempotencyKey)
	if err != nil {
		log.Printf("[PDA][%s] confirm_rejected task_id=%s error=%v", a.workflow, c.TaskID, err)
		return movementdomain.Task{}, err
	}
	return a.enrichedTask(ctx, row, a.workflow)
}

func (a *movementAdapter) enrichedTask(ctx context.Context, row executionTask, workflow string) (movementdomain.Task, error) {
	task := mapMovementTask(row, workflow)
	a.enrichReceiptCode(ctx, &task)
	if err := a.enrichTaskLocations(ctx, &task); err != nil {
		return movementdomain.Task{}, err
	}
	return task, nil
}

func (a *movementAdapter) enrichReceiptCode(ctx context.Context, task *movementdomain.Task) {
	if task.SourceReceiptID != "" && task.SourceReceiptCode == "" {
		receipt, err := a.client.GetReceipt(ctx, task.SourceReceiptID)
		if err == nil {
			task.SourceReceiptCode = receipt.ReceiptCode
		}
	}
	for index := range task.RelatedTasks {
		a.enrichReceiptCode(ctx, &task.RelatedTasks[index])
	}
}
func mapMovementTask(row executionTask, workflow string) movementdomain.Task {
	activeLineIDs := stringValues(row.Details["active_line_ids"])
	// Keep the group details long enough to build the complete relatedTasks
	// response. The active line is only validation/quantity context; it must
	// never replace the group identity used by subsequent commands.
	groupLines := movementLineDetails(row.Details)
	groupTask := len(groupLines) > 0 && strings.EqualFold(workflow, "PUTAWAY")
	row.Details = activeMovementLineDetails(row.Details)
	lineID := stringValue(row.Details, "line_id")
	status := movementdomain.Status(row.Status)
	if lineStatus := stringValue(row.Details, "status"); lineID != "" && lineStatus != "" {
		status = movementdomain.Status(lineStatus)
	}
	if status == "CREATED" {
		status = movementdomain.New
	} else if status == "CLAIMED" {
		status = movementdomain.Assigned
	} else if status == "PENDING" || status == "READY" {
		status = movementdomain.New
	}
	lot := stringValue(row.Details, "lot_code", "lot_id")
	requirements := []string{"SOURCE", "ITEM", "DESTINATION"}
	inboundReceipt := strings.EqualFold(stringValue(row.Details, "source_type"), "INBOUND_RECEIPT")
	if inboundReceipt {
		if _, grouped := row.Details["lines"]; grouped {
			requirements = []string{"LPN", "LOT", "DESTINATION"}
		} else {
			requirements = []string{"LPN", "DESTINATION"}
		}
	} else if lot != "" {
		requirements = []string{"SOURCE", "ITEM", "LOT", "DESTINATION"}
	}
	scanState := map[string]bool{}
	if raw, ok := row.Details["scan_state"].(map[string]any); ok {
		for key, value := range raw {
			if scanned, ok := value.(bool); ok {
				scanState[strings.ToUpper(key)] = scanned
			}
		}
	}
	// A grouped inbound putaway is one operator workflow, but older WMS
	// projections may persist the receipt-level LPN scan on a related line.
	// Merge that durable evidence into the group snapshot so reopening the
	// group cannot incorrectly send the operator back to VALIDATE_LPN.
	if groupTask {
		for _, line := range groupLines {
			if raw, ok := line["scan_state"].(map[string]any); ok {
				for key, value := range raw {
					if scanned, ok := value.(bool); ok && scanned {
						scanState[strings.ToUpper(key)] = true
					}
				}
			}
			if scanned, ok := line["lpn_validated"].(bool); ok && scanned {
				scanState["LPN"] = true
			}
		}
	}
	task := movementdomain.Task{
		ID: func() string {
			if groupTask {
				return row.TaskID
			}
			return firstNonBlank(lineID, row.TaskID)
		}(), ParentTaskID: func() string {
			if !groupTask && lineID != "" {
				return row.TaskID
			}
			return ""
		}(), LineID: lineID, Workflow: movementdomain.Workflow(workflow), Status: status, WarehouseID: row.WarehouseID,
		OperatorID: row.AssignedOperatorID, Version: row.Version, UpdatedAt: row.UpdatedAt,
		RequiredQuantity:  int64(number(row.Details, "qty", "quantity", "required_quantity")),
		CompletedQuantity: int64(number(row.Details, "completed_qty", "consumed_qty", "picked_qty")),
		SourceLocation:    stringValue(row.Details, "source_location_id", "source_location"),
		SourceLocationID:  stringValue(row.Details, "source_location_id"), SourceLocationCode: stringValue(row.Details, "source_location_code"),
		SourceBin: stringValue(row.Details, "source_bin_code", "source_bin_id"), SourceBinID: stringValue(row.Details, "source_bin_id"), SourceBinCode: stringValue(row.Details, "source_bin_code"),
		DestinationLocation:   stringValue(row.Details, "destination_location_id", "destination_location"),
		DestinationLocationID: stringValue(row.Details, "destination_location_id"), DestinationLocationCode: stringValue(row.Details, "destination_location_code"),
		DestinationCode:  stringValue(row.Details, "destination_location_code", "destination_location_code"),
		DestinationBinID: stringValue(row.Details, "destination_bin_id"), DestinationBinCode: stringValue(row.Details, "destination_bin_code"),
		ItemID: stringValue(row.Details, "item_revision_id", "item_id"), ItemCode: stringValue(row.Details, "item_code", "barcode"), ItemName: stringValue(row.Details, "item_name"), Barcode: stringValue(row.Details, "barcode", "item_code"), Lot: lot, LotID: stringValue(row.Details, "lot_id"), UOMCode: stringValue(row.Details, "uom_code"),
		LPNCode:            stringValue(row.Details, "lpn_code"),
		SalesFulfillmentID: stringValue(row.Details, "sales_fulfillment_id", "fulfillment_id"), SalesOrderID: stringValue(row.Details, "sales_order_id"), SalesOrderCode: stringValue(row.Details, "sales_order_code"), MaterialRequestID: stringValue(row.Details, "material_request_id"), WorkOrderID: stringValue(row.Details, "work_order_id"), WorkOrderCode: stringValue(row.Details, "work_order_code"), SourceReceiptID: stringValue(row.Details, "source_receipt_id"), SourceReceiptCode: stringValue(row.Details, "source_receipt_code", "receipt_code"), SourceValidated: scanState["SOURCE"], LPNValidated: scanState["LPN"], ItemValidated: scanState["ITEM"], LotValidated: scanState["LOT"], DestinationValidated: scanState["DESTINATION"], ScanRequirements: requirements, ActiveLineIDs: activeLineIDs,
	}
	for _, line := range groupLines {
		lineRow := row
		lineRow.Details = line
		task.RelatedTasks = append(task.RelatedTasks, mapMovementTask(lineRow, workflow))
	}
	return task
}

func stringValues(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

// movementLineDetails normalizes both the Phase-2 group shape (details.lines)
// and the legacy WMS sibling shape (details.related_tasks). Keeping this
// normalization at the PDA Backend boundary means PDA_APP receives one stable
// relatedTasks contract regardless of which WMS task projection produced it.
func movementLineDetails(details map[string]any) []map[string]any {
	var out []map[string]any
	if raw, ok := details["lines"]; ok {
		switch lines := raw.(type) {
		case []any:
			for _, value := range lines {
				if line, ok := value.(map[string]any); ok {
					out = append(out, line)
				}
			}
		case []map[string]any:
			out = append(out, lines...)
		}
	}
	if len(out) > 0 {
		return out
	}
	if raw, ok := details["related_tasks"].([]any); ok {
		for _, value := range raw {
			task, ok := value.(map[string]any)
			if !ok {
				continue
			}
			if nested, ok := task["details"].(map[string]any); ok {
				merged := make(map[string]any, len(task)+len(nested))
				for key, item := range task {
					merged[key] = item
				}
				for key, item := range nested {
					merged[key] = item
				}
				out = append(out, merged)
			} else {
				out = append(out, task)
			}
		}
	}
	return out
}

func activeMovementLineDetails(details map[string]any) map[string]any {
	activeID := stringValue(details, "active_line_id")
	if activeID == "" {
		return details
	}
	lines, _ := details["lines"].([]any)
	for _, raw := range lines {
		line, ok := raw.(map[string]any)
		if !ok || stringValue(line, "line_id") != activeID {
			continue
		}
		merged := make(map[string]any, len(details)+len(line))
		for key, value := range details {
			merged[key] = value
		}
		for key, value := range line {
			merged[key] = value
		}
		return merged
	}
	return details
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
func (a *PutawayAdapter) Claim(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	row, err := a.client.ApplyExecutionTaskCommand(c, x.TaskID, executionTaskCommand{CommandID: x.CommandID.String(), CommandType: "CLAIM", ExpectedVersion: x.BaseVersion, CorrelationID: x.Actor.CorrelationID}, x.Actor.OperatorID, x.IdempotencyKey)
	if err != nil {
		return movementdomain.Task{}, err
	}
	return a.enrichedTask(c, row, a.workflow)
}
func (a *PutawayAdapter) Start(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	current, err := a.detail(context.Background(), x.TaskID, x.Actor)
	if err != nil {
		return movementdomain.Task{}, err
	}
	claimed := current
	switch current.Status {
	case movementdomain.New:
		claimed, err = a.Claim(c, x)
		if err != nil {
			return movementdomain.Task{}, err
		}
	case movementdomain.Assigned, movementdomain.InProgress:
		// The task is already owned by this operator. Re-claiming a CLAIMED
		// task is an invalid WMS transition; proceed with START directly.
	default:
		return movementdomain.Task{}, &platform.DomainError{Code: "TASK_NOT_STARTABLE", SafeMessage: "Putaway task is not ready to start"}
	}
	if claimed.Status == movementdomain.InProgress {
		return claimed, nil
	}
	row, err := a.client.ApplyExecutionTaskCommand(c, x.TaskID, executionTaskCommand{CommandID: uuid.NewString(), CommandType: "START", ExpectedVersion: claimed.Version, CorrelationID: x.Actor.CorrelationID}, x.Actor.OperatorID, x.IdempotencyKey+":start")
	if err != nil {
		return movementdomain.Task{}, err
	}
	return a.enrichedTask(c, row, a.workflow)
}
func (a *PutawayAdapter) Suggestions(context.Context, string, platform.ActorContext) ([]movementdomain.Location, error) {
	return nil, &platform.DomainError{Code: "UPSTREAM_OPERATION_NOT_IMPLEMENTED", SafeMessage: "WMS putaway destination suggestion is not mapped"}
}
func (a *PutawayAdapter) ValidateSource(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "SOURCE", v)
}
func (a *PutawayAdapter) ValidateLPN(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "LPN", v)
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
func (a *PutawayAdapter) ConfirmGroup(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	row, err := a.client.ApplyExecutionTaskCommand(c, x.TaskID, executionTaskCommand{CommandID: x.CommandID.String(), CommandType: "CONFIRM_GROUP", ExpectedVersion: x.BaseVersion, CorrelationID: x.Actor.CorrelationID, CausationID: x.CommandID.String()}, x.Actor.OperatorID, x.IdempotencyKey)
	if err != nil {
		return movementdomain.Task{}, err
	}
	return a.enrichedTask(c, row, a.workflow)
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
func (a *PickingAdapter) Claim(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	current, err := a.detail(c, x.TaskID, x.Actor)
	if err != nil {
		return movementdomain.Task{}, err
	}
	if current.OperatorID == nil || *current.OperatorID != x.Actor.OperatorID {
		return movementdomain.Task{}, movementdomain.ErrNotAssigned
	}
	row, err := a.client.ApplyExecutionTaskCommand(c, x.TaskID, executionTaskCommand{CommandID: x.CommandID.String(), CommandType: "CLAIM", ExpectedVersion: x.BaseVersion, CorrelationID: x.Actor.CorrelationID}, x.Actor.OperatorID, x.IdempotencyKey)
	if err != nil {
		return movementdomain.Task{}, err
	}
	return a.enrichedTask(c, row, a.workflow)
}
func (a *PickingAdapter) Start(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	current, err := a.detail(c, x.TaskID, x.Actor)
	if err != nil {
		return movementdomain.Task{}, err
	}
	claimed := current
	switch current.Status {
	case movementdomain.New:
		claimed, err = a.Claim(c, x)
	case movementdomain.Assigned:
		// WMS does not allow ASSIGNED -> START. Assignment identifies the
		// operator eligible to execute the task, while CLAIM acquires the
		// execution lock. Always acquire that lock before starting.
		claimed, err = a.Claim(c, x)
	case movementdomain.InProgress:
	default:
		return movementdomain.Task{}, &platform.DomainError{Code: "TASK_NOT_STARTABLE", SafeMessage: "Picking task is not ready to start"}
	}
	if err != nil {
		return movementdomain.Task{}, err
	}
	if claimed.Status == movementdomain.InProgress {
		return claimed, nil
	}
	row, err := a.client.ApplyExecutionTaskCommand(c, x.TaskID, executionTaskCommand{CommandID: uuid.NewString(), CommandType: "START", ExpectedVersion: claimed.Version, CorrelationID: x.Actor.CorrelationID}, x.Actor.OperatorID, x.IdempotencyKey+":start")
	if err != nil {
		return movementdomain.Task{}, err
	}
	return a.enrichedTask(c, row, a.workflow)
}
func (a *PickingAdapter) Allocate(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	row, err := a.client.AllocateExecutionTask(c, x.TaskID, x.CommandID.String(), x.Actor.CorrelationID, x.Actor.OperatorID, x.IdempotencyKey)
	if err != nil {
		return movementdomain.Task{}, err
	}
	warehouseID, err := a.client.CanonicalWarehouseID(context.Background(), x.Actor.WarehouseID)
	if err != nil {
		return movementdomain.Task{}, err
	}
	if warehouseID != "" && row.WarehouseID != warehouseID || row.AssignedOperatorID != nil && *row.AssignedOperatorID != x.Actor.OperatorID {
		return movementdomain.Task{}, &platform.DomainError{Code: "WAREHOUSE_ACCESS_DENIED", SafeMessage: "Picking task access denied"}
	}
	return a.enrichedTask(c, row, "PICKING")
}
func (a *PickingAdapter) ValidateLocation(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "SOURCE", v)
}
func (a *PickingAdapter) ResolveBarcode(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "ITEM", v)
}
func (a *PickingAdapter) ValidateLot(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "LOT", v)
}
func (a *PickingAdapter) ValidateDestination(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return a.scan(c, x, "DESTINATION", v)
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
