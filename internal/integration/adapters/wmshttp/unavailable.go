package wmshttp

import (
	"context"
	"fmt"

	executionapp "github.com/company/pda-backend/internal/execution/application"
	executiondomain "github.com/company/pda-backend/internal/execution/domain"
	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	movementdomain "github.com/company/pda-backend/internal/execution/movement/domain"
	movementports "github.com/company/pda-backend/internal/execution/movement/ports"
	executionports "github.com/company/pda-backend/internal/execution/ports"
	gatewayports "github.com/company/pda-backend/internal/gateway/ports"
	inventoryapp "github.com/company/pda-backend/internal/inventory/application"
	inventorydomain "github.com/company/pda-backend/internal/inventory/domain"
	inventoryports "github.com/company/pda-backend/internal/inventory/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	shippingapp "github.com/company/pda-backend/internal/shipping/application"
	shippingdomain "github.com/company/pda-backend/internal/shipping/domain"
	shippingports "github.com/company/pda-backend/internal/shipping/ports"
	"github.com/google/uuid"
)

// Unavailable is used only for owner capabilities whose remote contract has
// not yet been mapped. It prevents HTTP mode from silently using local PDA
// business stores while keeping the unsupported operation explicit.
type Unavailable struct{ Capability string }

func (u Unavailable) err() error {
	return &platform.DomainError{Code: "UPSTREAM_OPERATION_NOT_IMPLEMENTED", SafeMessage: fmt.Sprintf("WMS %s operation is not mapped", u.Capability), Retryable: false}
}

func (u Unavailable) TaskStatus(context.Context, uuid.UUID, platform.ActorContext) (executionports.IdempotencyResult, error) {
	return executionports.IdempotencyResult{}, u.err()
}
func (u Unavailable) TaskList(context.Context, executionports.TaskFilter, platform.ActorContext) (executionports.TaskPage, error) {
	return executionports.TaskPage{}, u.err()
}
func (u Unavailable) TaskDetail(context.Context, string, platform.ActorContext) (executiondomain.Task, error) {
	return executiondomain.Task{}, u.err()
}
func (u Unavailable) TaskSummary(context.Context, string, string, platform.ActorContext) ([]executionports.SummaryItem, error) {
	return nil, u.err()
}
func (u Unavailable) TaskDashboard(context.Context, platform.ActorContext) (executionports.Dashboard, error) {
	return executionports.Dashboard{}, u.err()
}
func (u Unavailable) TaskClaim(context.Context, executionapp.TaskCommand) (executiondomain.Task, error) {
	return executiondomain.Task{}, u.err()
}
func (u Unavailable) TaskRelease(context.Context, executionapp.TaskCommand) (executiondomain.Task, error) {
	return executiondomain.Task{}, u.err()
}

func (u Unavailable) MovementList(context.Context, platform.ActorContext) ([]movementdomain.Task, error) {
	return nil, u.err()
}
func (u Unavailable) MovementDetail(context.Context, string, platform.ActorContext) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) PickingAllocate(context.Context, movementapp.Command) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) Suggestions(context.Context, string, platform.ActorContext) ([]movementdomain.Location, error) {
	return nil, u.err()
}
func (u Unavailable) ValidateSource(context.Context, movementapp.Command, string) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) ValidateDestination(context.Context, movementapp.Command, string) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) ValidateLocation(context.Context, movementapp.Command, string) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) ResolveBarcode(context.Context, movementapp.Command, string) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) ValidateItem(context.Context, movementapp.Command, string) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) MovementConfirm(context.Context, movementapp.Command, int64) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) MovementComplete(context.Context, movementapp.Command) (movementdomain.Task, error) {
	return movementdomain.Task{}, u.err()
}
func (u Unavailable) MovementStatus(context.Context, uuid.UUID, platform.ActorContext) (movementports.CommandResult, error) {
	return movementports.CommandResult{}, u.err()
}

func (u Unavailable) InventorySearch(context.Context, string, platform.ActorContext) ([]inventorydomain.Balance, error) {
	return nil, u.err()
}
func (u Unavailable) InventoryBalances(context.Context, string, string, platform.ActorContext) ([]inventorydomain.Balance, error) {
	return nil, u.err()
}
func (u Unavailable) InventoryMovements(context.Context, string, string, platform.ActorContext) ([]inventorydomain.Movement, error) {
	return nil, u.err()
}
func (u Unavailable) ValidateTransfer(context.Context, inventoryapp.TransferCommand) error {
	return u.err()
}
func (u Unavailable) Transfer(context.Context, inventoryapp.TransferCommand) (inventorydomain.Transfer, error) {
	return inventorydomain.Transfer{}, u.err()
}
func (u Unavailable) InventoryStatus(context.Context, uuid.UUID, platform.ActorContext) (inventoryports.CommandResult, error) {
	return inventoryports.CommandResult{}, u.err()
}
func (u Unavailable) ListCounts(context.Context, platform.ActorContext) ([]inventorydomain.CountTask, error) {
	return nil, u.err()
}
func (u Unavailable) CountDetail(context.Context, string, platform.ActorContext) (inventorydomain.CountTask, error) {
	return inventorydomain.CountTask{}, u.err()
}
func (u Unavailable) ValidateCountLocation(context.Context, string, string, platform.ActorContext) error {
	return u.err()
}
func (u Unavailable) ValidateCountItem(context.Context, string, string, string, platform.ActorContext) error {
	return u.err()
}
func (u Unavailable) SubmitCount(context.Context, string, string, int64, inventoryapp.Command) (inventorydomain.CountTask, error) {
	return inventorydomain.CountTask{}, u.err()
}
func (u Unavailable) Recount(context.Context, string, string, inventoryapp.Command) (inventorydomain.CountTask, error) {
	return inventorydomain.CountTask{}, u.err()
}
func (u Unavailable) CompleteCount(context.Context, string, inventoryapp.Command) (inventorydomain.CountTask, error) {
	return inventorydomain.CountTask{}, u.err()
}
func (u Unavailable) CountStatus(context.Context, uuid.UUID, platform.ActorContext) (inventoryports.CommandResult, error) {
	return inventoryports.CommandResult{}, u.err()
}

func (u Unavailable) ShipmentSummary(context.Context, string, platform.ActorContext) (shippingdomain.Shipment, error) {
	return shippingdomain.Shipment{}, u.err()
}
func (u Unavailable) Readiness(context.Context, string, platform.ActorContext) (shippingdomain.Readiness, error) {
	return shippingdomain.Readiness{}, u.err()
}
func (u Unavailable) VerifyPackage(context.Context, shippingapp.PackageVerifyCommand) (shippingdomain.Shipment, error) {
	return shippingdomain.Shipment{}, u.err()
}
func (u Unavailable) ShipmentConfirm(context.Context, shippingapp.ConfirmCommand) (shippingdomain.Shipment, error) {
	return shippingdomain.Shipment{}, u.err()
}
func (u Unavailable) ShipmentStatus(context.Context, uuid.UUID, platform.ActorContext) (shippingports.CommandResult, error) {
	return shippingports.CommandResult{}, u.err()
}

var _ gatewayports.TaskOperations = taskAdapter(Unavailable{})
var _ gatewayports.PutawayOperations = putawayAdapter(Unavailable{})
var _ gatewayports.PickingOperations = pickingAdapter(Unavailable{})
var _ gatewayports.ReplenishmentOperations = replenishmentAdapter(Unavailable{})
var _ gatewayports.MovementCommandOperations = movementCommandsAdapter(Unavailable{})
var _ gatewayports.InventoryOperations = inventoryAdapter(Unavailable{})
var _ gatewayports.ShippingOperations = shippingAdapter(Unavailable{})

type taskAdapter Unavailable

func (u taskAdapter) CommandStatus(c context.Context, id uuid.UUID, a platform.ActorContext) (executionports.IdempotencyResult, error) {
	return Unavailable(u).TaskStatus(c, id, a)
}
func (u taskAdapter) List(c context.Context, f executionports.TaskFilter, a platform.ActorContext) (executionports.TaskPage, error) {
	return Unavailable(u).TaskList(c, f, a)
}
func (u taskAdapter) Detail(c context.Context, id string, a platform.ActorContext) (executiondomain.Task, error) {
	return Unavailable(u).TaskDetail(c, id, a)
}
func (u taskAdapter) Summary(c context.Context, w, s string, a platform.ActorContext) ([]executionports.SummaryItem, error) {
	return Unavailable(u).TaskSummary(c, w, s, a)
}
func (u taskAdapter) Dashboard(c context.Context, a platform.ActorContext) (executionports.Dashboard, error) {
	return Unavailable(u).TaskDashboard(c, a)
}
func (u taskAdapter) Claim(c context.Context, x executionapp.TaskCommand) (executiondomain.Task, error) {
	return Unavailable(u).TaskClaim(c, x)
}
func (u taskAdapter) Release(c context.Context, x executionapp.TaskCommand) (executiondomain.Task, error) {
	return Unavailable(u).TaskRelease(c, x)
}

type putawayAdapter Unavailable

func (u putawayAdapter) List(c context.Context, a platform.ActorContext) ([]movementdomain.Task, error) {
	return Unavailable(u).MovementList(c, a)
}
func (u putawayAdapter) Detail(c context.Context, id string, a platform.ActorContext) (movementdomain.Task, error) {
	return Unavailable(u).MovementDetail(c, id, a)
}
func (u putawayAdapter) Suggestions(c context.Context, id string, a platform.ActorContext) ([]movementdomain.Location, error) {
	return Unavailable(u).Suggestions(c, id, a)
}
func (u putawayAdapter) ValidateSource(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return Unavailable(u).ValidateSource(c, x, v)
}
func (u putawayAdapter) ValidateDestination(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return Unavailable(u).ValidateDestination(c, x, v)
}
func (u putawayAdapter) Confirm(c context.Context, x movementapp.Command, q int64) (movementdomain.Task, error) {
	return Unavailable(u).MovementConfirm(c, x, q)
}

type pickingAdapter Unavailable

func (u pickingAdapter) List(c context.Context, a platform.ActorContext) ([]movementdomain.Task, error) {
	return Unavailable(u).MovementList(c, a)
}
func (u pickingAdapter) Detail(c context.Context, id string, a platform.ActorContext) (movementdomain.Task, error) {
	return Unavailable(u).MovementDetail(c, id, a)
}
func (u pickingAdapter) Allocate(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	return Unavailable(u).PickingAllocate(c, x)
}
func (u pickingAdapter) ValidateLocation(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return Unavailable(u).ValidateLocation(c, x, v)
}
func (u pickingAdapter) ResolveBarcode(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return Unavailable(u).ResolveBarcode(c, x, v)
}
func (u pickingAdapter) Confirm(c context.Context, x movementapp.Command, q int64) (movementdomain.Task, error) {
	return Unavailable(u).MovementConfirm(c, x, q)
}
func (u pickingAdapter) Complete(c context.Context, x movementapp.Command) (movementdomain.Task, error) {
	return Unavailable(u).MovementComplete(c, x)
}

type replenishmentAdapter Unavailable

func (u replenishmentAdapter) List(c context.Context, a platform.ActorContext) ([]movementdomain.Task, error) {
	return Unavailable(u).MovementList(c, a)
}
func (u replenishmentAdapter) Detail(c context.Context, id string, a platform.ActorContext) (movementdomain.Task, error) {
	return Unavailable(u).MovementDetail(c, id, a)
}
func (u replenishmentAdapter) ValidateSource(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return Unavailable(u).ValidateSource(c, x, v)
}
func (u replenishmentAdapter) ValidateDestination(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return Unavailable(u).ValidateDestination(c, x, v)
}
func (u replenishmentAdapter) ValidateItem(c context.Context, x movementapp.Command, v string) (movementdomain.Task, error) {
	return Unavailable(u).ValidateItem(c, x, v)
}
func (u replenishmentAdapter) Confirm(c context.Context, x movementapp.Command, q int64) (movementdomain.Task, error) {
	return Unavailable(u).MovementConfirm(c, x, q)
}

type movementCommandsAdapter Unavailable

func (u movementCommandsAdapter) CommandStatus(c context.Context, id uuid.UUID, a platform.ActorContext) (movementports.CommandResult, error) {
	return Unavailable(u).MovementStatus(c, id, a)
}

type inventoryAdapter Unavailable

func (u inventoryAdapter) Search(c context.Context, q string, a platform.ActorContext) ([]inventorydomain.Balance, error) {
	return Unavailable(u).InventorySearch(c, q, a)
}
func (u inventoryAdapter) Balances(c context.Context, i, l string, a platform.ActorContext) ([]inventorydomain.Balance, error) {
	return Unavailable(u).InventoryBalances(c, i, l, a)
}
func (u inventoryAdapter) Movements(c context.Context, i, cur string, a platform.ActorContext) ([]inventorydomain.Movement, error) {
	return Unavailable(u).InventoryMovements(c, i, cur, a)
}
func (u inventoryAdapter) ValidateTransfer(c context.Context, x inventoryapp.TransferCommand) error {
	return Unavailable(u).ValidateTransfer(c, x)
}
func (u inventoryAdapter) Transfer(c context.Context, x inventoryapp.TransferCommand) (inventorydomain.Transfer, error) {
	return Unavailable(u).Transfer(c, x)
}
func (u inventoryAdapter) CommandStatus(c context.Context, id uuid.UUID, a platform.ActorContext) (inventoryports.CommandResult, error) {
	return Unavailable(u).InventoryStatus(c, id, a)
}
func (u inventoryAdapter) ListCounts(c context.Context, a platform.ActorContext) ([]inventorydomain.CountTask, error) {
	return Unavailable(u).ListCounts(c, a)
}
func (u inventoryAdapter) CountDetail(c context.Context, id string, a platform.ActorContext) (inventorydomain.CountTask, error) {
	return Unavailable(u).CountDetail(c, id, a)
}
func (u inventoryAdapter) ValidateCountLocation(c context.Context, id, l string, a platform.ActorContext) error {
	return Unavailable(u).ValidateCountLocation(c, id, l, a)
}
func (u inventoryAdapter) ValidateCountItem(c context.Context, id, line, item string, a platform.ActorContext) error {
	return Unavailable(u).ValidateCountItem(c, id, line, item, a)
}
func (u inventoryAdapter) SubmitCount(c context.Context, id, line string, q int64, x inventoryapp.Command) (inventorydomain.CountTask, error) {
	return Unavailable(u).SubmitCount(c, id, line, q, x)
}
func (u inventoryAdapter) Recount(c context.Context, id, line string, x inventoryapp.Command) (inventorydomain.CountTask, error) {
	return Unavailable(u).Recount(c, id, line, x)
}
func (u inventoryAdapter) CompleteCount(c context.Context, id string, x inventoryapp.Command) (inventorydomain.CountTask, error) {
	return Unavailable(u).CompleteCount(c, id, x)
}
func (u inventoryAdapter) CountCommandStatus(c context.Context, id uuid.UUID, a platform.ActorContext) (inventoryports.CommandResult, error) {
	return Unavailable(u).CountStatus(c, id, a)
}

type shippingAdapter Unavailable

func (u shippingAdapter) Summary(c context.Context, id string, a platform.ActorContext) (shippingdomain.Shipment, error) {
	return Unavailable(u).ShipmentSummary(c, id, a)
}
func (u shippingAdapter) Readiness(c context.Context, id string, a platform.ActorContext) (shippingdomain.Readiness, error) {
	return Unavailable(u).Readiness(c, id, a)
}
func (u shippingAdapter) VerifyPackage(c context.Context, x shippingapp.PackageVerifyCommand) (shippingdomain.Shipment, error) {
	return Unavailable(u).VerifyPackage(c, x)
}
func (u shippingAdapter) Confirm(c context.Context, x shippingapp.ConfirmCommand) (shippingdomain.Shipment, error) {
	return Unavailable(u).ShipmentConfirm(c, x)
}
func (u shippingAdapter) CommandStatus(c context.Context, id uuid.UUID, a platform.ActorContext) (shippingports.CommandResult, error) {
	return Unavailable(u).ShipmentStatus(c, id, a)
}

func NewUnavailableAdapters() (gatewayports.TaskOperations, gatewayports.PutawayOperations, gatewayports.PickingOperations, gatewayports.ReplenishmentOperations, gatewayports.MovementCommandOperations, gatewayports.InventoryOperations, gatewayports.ShippingOperations) {
	return taskAdapter(Unavailable{"execution"}), putawayAdapter(Unavailable{"putaway"}), pickingAdapter(Unavailable{"picking"}), replenishmentAdapter(Unavailable{"replenishment"}), movementCommandsAdapter(Unavailable{"movement"}), inventoryAdapter(Unavailable{"inventory"}), shippingAdapter(Unavailable{"shipping"})
}
