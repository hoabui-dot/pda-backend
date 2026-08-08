package ports

import "context"

type Warehouse struct{ ID, Code, Name string }

type UpstreamWMS interface {
	Warehouses(context.Context) ([]Warehouse, error)
}

// MasterDataAdapter is read-only. Master Data remains authoritative for
// warehouse and location identity.
type MasterDataAdapter interface {
	UpstreamWMS
	Location(context.Context, string) (Location, error)
}

// InboundAdapter exposes only existing Inbound-owned read/mutation contracts.
// It does not create receiving work or calculate receipt quantities.
type InboundAdapter interface {
	ListReceipts(context.Context, ReceiptQuery) ([]ReceiptSummary, error)
	GetReceipt(context.Context, string) (Receipt, error)
	RecordReceiptQuantity(context.Context, string, string, ReceiptQuantityRequest) (ReceiptQuantityResult, error)
	ConfirmReceipt(context.Context, string, string) (ReceiptConfirmation, error)
}

type Location struct {
	ID            string `json:"location_id"`
	Code          string `json:"location_code"`
	WarehouseID   string `json:"warehouse_id"`
	WarehouseCode string `json:"warehouse_code"`
}

type ReceiptQuery struct {
	Status              string
	WarehouseLocationID string
	Query               string
	Limit               int
}

type ReceiptSummary struct {
	ReceiptID           string `json:"receipt_id"`
	ReceiptCode         string `json:"receipt_code"`
	WarehouseLocationID string `json:"warehouse_location_id"`
	Status              string `json:"status"`
	ConfirmationStatus  string `json:"confirmation_status"`
	LineCount           int    `json:"line_count"`
}

type Receipt struct {
	ReceiptID           string        `json:"receipt_id"`
	ReceiptCode         string        `json:"receipt_code"`
	WarehouseLocationID string        `json:"warehouse_location_id"`
	Status              string        `json:"status"`
	ConfirmationStatus  string        `json:"confirmation_status"`
	Lines               []ReceiptLine `json:"lines"`
}

type ReceiptLine struct {
	LineID           string  `json:"line_id"`
	ItemRevisionID   string  `json:"item_revision_id"`
	LotCode          string  `json:"lot_code"`
	Quantity         float64 `json:"qty"`
	ReceivedQuantity float64 `json:"received_quantity"`
	Expected         float64 `json:"expected_quantity"`
	UOMCode          string  `json:"uom_code"`
	Version          int     `json:"row_version"`
}

type ReceiptConfirmation struct {
	ReceiptID          string `json:"receipt_id"`
	Status             string `json:"status"`
	ConfirmationStatus string `json:"confirmation_status"`
	LineCount          int    `json:"line_count"`
	Result             string `json:"result"`
}

type ReceiptQuantityRequest struct {
	ActualQuantity  float64 `json:"actual_quantity"`
	UOMCode         string  `json:"uom_code"`
	ExpectedVersion int     `json:"expected_version"`
	ItemRevisionID  string  `json:"item_revision_id"`
	LotCode         string  `json:"lot_code"`
	IdempotencyKey  string  `json:"-"`
}

type ReceiptQuantityResult struct {
	CommandID      string  `json:"command_id"`
	ReceiptID      string  `json:"receipt_id"`
	LineID         string  `json:"line_id"`
	Status         string  `json:"status"`
	ActualQuantity float64 `json:"actual_quantity"`
	UOMCode        string  `json:"uom_code"`
	Version        int     `json:"version"`
}
