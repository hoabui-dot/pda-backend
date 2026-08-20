package ports

// ReceivingOperations is the gateway-level workflow boundary. It deliberately
// exposes receiving use cases, not repositories, transactions, or outboxes.
// Local and WMS-backed implementations can both satisfy this boundary.
import (
	"context"

	executionapp "github.com/company/pda-backend/internal/execution/application"
	executiondomain "github.com/company/pda-backend/internal/execution/domain"
	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	movementdomain "github.com/company/pda-backend/internal/execution/movement/domain"
	movementports "github.com/company/pda-backend/internal/execution/movement/ports"
	executionports "github.com/company/pda-backend/internal/execution/ports"
	receivingapp "github.com/company/pda-backend/internal/execution/receiving/application"
	receivingdomain "github.com/company/pda-backend/internal/execution/receiving/domain"
	receivingports "github.com/company/pda-backend/internal/execution/receiving/ports"
	inventoryapp "github.com/company/pda-backend/internal/inventory/application"
	inventorydomain "github.com/company/pda-backend/internal/inventory/domain"
	inventoryports "github.com/company/pda-backend/internal/inventory/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	shippingapp "github.com/company/pda-backend/internal/shipping/application"
	shippingdomain "github.com/company/pda-backend/internal/shipping/domain"
	shippingports "github.com/company/pda-backend/internal/shipping/ports"
	"github.com/google/uuid"
)

type TaskOperations interface {
	CommandStatus(context.Context, uuid.UUID, platform.ActorContext) (executionports.IdempotencyResult, error)
	List(context.Context, executionports.TaskFilter, platform.ActorContext) (executionports.TaskPage, error)
	Detail(context.Context, string, platform.ActorContext) (executiondomain.Task, error)
	Summary(context.Context, string, string, platform.ActorContext) ([]executionports.SummaryItem, error)
	Dashboard(context.Context, platform.ActorContext) (executionports.Dashboard, error)
	Claim(context.Context, executionapp.TaskCommand) (executiondomain.Task, error)
	Release(context.Context, executionapp.TaskCommand) (executiondomain.Task, error)
}

type PutawayOperations interface {
	List(context.Context, platform.ActorContext) ([]movementdomain.Task, error)
	Detail(context.Context, string, platform.ActorContext) (movementdomain.Task, error)
	Claim(context.Context, movementapp.Command) (movementdomain.Task, error)
	Start(context.Context, movementapp.Command) (movementdomain.Task, error)
	Suggestions(context.Context, string, platform.ActorContext) ([]movementdomain.Location, error)
	ValidateSource(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ValidateLPN(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ValidateItem(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ValidateLot(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ValidateDestination(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	Confirm(context.Context, movementapp.Command, int64) (movementdomain.Task, error)
	ConfirmGroup(context.Context, movementapp.Command) (movementdomain.Task, error)
}

type PickingOperations interface {
	List(context.Context, platform.ActorContext) ([]movementdomain.Task, error)
	Detail(context.Context, string, platform.ActorContext) (movementdomain.Task, error)
	Claim(context.Context, movementapp.Command) (movementdomain.Task, error)
	Start(context.Context, movementapp.Command) (movementdomain.Task, error)
	Allocate(context.Context, movementapp.Command) (movementdomain.Task, error)
	ValidateLocation(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ResolveBarcode(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ValidateLot(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ValidateDestination(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	Confirm(context.Context, movementapp.Command, int64) (movementdomain.Task, error)
	Complete(context.Context, movementapp.Command) (movementdomain.Task, error)
}

type ReplenishmentOperations interface {
	List(context.Context, platform.ActorContext) ([]movementdomain.Task, error)
	Detail(context.Context, string, platform.ActorContext) (movementdomain.Task, error)
	ValidateSource(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ValidateDestination(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	ValidateItem(context.Context, movementapp.Command, string) (movementdomain.Task, error)
	Confirm(context.Context, movementapp.Command, int64) (movementdomain.Task, error)
}

type MovementCommandOperations interface {
	CommandStatus(context.Context, uuid.UUID, platform.ActorContext) (movementports.CommandResult, error)
}

type InventoryOperations interface {
	Search(context.Context, string, platform.ActorContext) ([]inventorydomain.Balance, error)
	Balances(context.Context, string, string, platform.ActorContext) ([]inventorydomain.Balance, error)
	Movements(context.Context, string, string, platform.ActorContext) ([]inventorydomain.Movement, error)
	ValidateTransfer(context.Context, inventoryapp.TransferCommand) error
	Transfer(context.Context, inventoryapp.TransferCommand) (inventorydomain.Transfer, error)
	CommandStatus(context.Context, uuid.UUID, platform.ActorContext) (inventoryports.CommandResult, error)
	ListCounts(context.Context, platform.ActorContext) ([]inventorydomain.CountTask, error)
	CountDetail(context.Context, string, platform.ActorContext) (inventorydomain.CountTask, error)
	ValidateCountLocation(context.Context, string, string, platform.ActorContext) error
	ValidateCountItem(context.Context, string, string, string, platform.ActorContext) error
	SubmitCount(context.Context, string, string, int64, inventoryapp.Command) (inventorydomain.CountTask, error)
	Recount(context.Context, string, string, inventoryapp.Command) (inventorydomain.CountTask, error)
	CompleteCount(context.Context, string, inventoryapp.Command) (inventorydomain.CountTask, error)
	CountCommandStatus(context.Context, uuid.UUID, platform.ActorContext) (inventoryports.CommandResult, error)
}

type ShippingOperations interface {
	Summary(context.Context, string, platform.ActorContext) (shippingdomain.Shipment, error)
	Readiness(context.Context, string, platform.ActorContext) (shippingdomain.Readiness, error)
	VerifyPackage(context.Context, shippingapp.PackageVerifyCommand) (shippingdomain.Shipment, error)
	Confirm(context.Context, shippingapp.ConfirmCommand) (shippingdomain.Shipment, error)
	CommandStatus(context.Context, uuid.UUID, platform.ActorContext) (shippingports.CommandResult, error)
}

type ReceivingOperations interface {
	List(context.Context, receivingports.Filter, platform.ActorContext) (receivingports.Page, error)
	Detail(context.Context, string, platform.ActorContext) (receivingdomain.Task, error)
	Claim(context.Context, receivingapp.Command) (receivingdomain.Task, error)
	ResolveBarcode(context.Context, string, string, platform.ActorContext) (receivingdomain.Line, error)
	Start(context.Context, receivingapp.Command) (receivingdomain.Task, error)
	Confirm(context.Context, receivingapp.ConfirmCommand) (receivingdomain.Task, error)
	Complete(context.Context, receivingapp.Command) (receivingdomain.Task, error)
	CommandStatus(context.Context, uuid.UUID, platform.ActorContext) (receivingports.CommandStatus, error)
}

// ReceivingBarcodeOperations is an optional extension for production
// adapters that can preserve scanner symbology when resolving a barcode.
// Existing local adapters remain compatible through ReceivingOperations.
type ReceivingBarcodeOperations interface {
	ResolveBarcodeWithSymbology(context.Context, string, string, string, platform.ActorContext) (receivingdomain.Line, error)
}

// ReceivingScanContextOperations preserves the business scan context for
// production adapters. LPN scans identify the receipt and must not be sent to
// the item barcode resolver.
type ReceivingScanContextOperations interface {
	ResolveBarcodeWithContext(context.Context, string, string, string, string, platform.ActorContext) (receivingdomain.Line, error)
}

// ReceivingBatchOperations is the one-Receipt submit boundary used by the
// production WMS adapter. Local legacy receiving services may omit it.
type ReceivingBatchOperations interface {
	ConfirmBatch(context.Context, receivingapp.BatchConfirmCommand) (receivingdomain.Task, error)
}

// FinishedProductTraceOperations is a read-only BFF boundary. The PDA
// gateway exposes this contract without leaking WMS or MES repositories.
type FinishedProductTraceOperations interface {
	Trace(context.Context, string, platform.ActorContext) (map[string]any, error)
}
