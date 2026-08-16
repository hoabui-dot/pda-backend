package ports

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

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
	ReceiveReceipt(context.Context, string, ReceiptBatchRequest) (ReceiptBatchResult, error)
}

type Location struct {
	ID            string `json:"location_id"`
	Code          string `json:"location_code"`
	Name          string `json:"location_name,omitempty"`
	WarehouseID   string `json:"warehouse_id"`
	WarehouseCode string `json:"warehouse_code"`
}

type ReceiptQuery struct {
	Status              string
	WarehouseLocationID string
	AssignedOperatorID  string
	Query               string
	SourceType          string
	Limit               int
}

type ReceiptSummary struct {
	ReceiptID            string        `json:"receipt_id"`
	ReceiptCode          string        `json:"receipt_code"`
	LPNCode              string        `json:"lpn_code"`
	WarehouseLocationID  string        `json:"warehouse_location_id"`
	Status               string        `json:"status"`
	ConfirmationStatus   string        `json:"confirmation_status"`
	LineCount            int           `json:"line_count"`
	Lines                []ReceiptLine `json:"lines"`
	AssignedOperatorID   *string       `json:"assigned_operator_id"`
	AssignmentStatus     string        `json:"assignment_status"`
	AssignmentVersion    int64         `json:"assignment_version"`
	SourceType           string        `json:"source_type"`
	SourceSystem         string        `json:"source_system"`
	SourceDocumentType   string        `json:"source_document_type"`
	SourceRequestID      string        `json:"source_request_id"`
	SourceOutputID       string        `json:"source_output_id"`
	SourceWOID           string        `json:"source_wo_id"`
	SourceWOCode         string        `json:"source_wo_code"`
	SourceConfirmationID string        `json:"source_confirmation_id"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

type Receipt struct {
	ReceiptID            string        `json:"receipt_id"`
	ReceiptCode          string        `json:"receipt_code"`
	LPNCode              string        `json:"lpn_code"`
	WarehouseLocationID  string        `json:"warehouse_location_id"`
	Status               string        `json:"status"`
	ConfirmationStatus   string        `json:"confirmation_status"`
	AssignedOperatorID   *string       `json:"assigned_operator_id"`
	AssignmentStatus     string        `json:"assignment_status"`
	AssignmentVersion    int64         `json:"assignment_version"`
	SourceType           string        `json:"source_type"`
	SourceSystem         string        `json:"source_system"`
	SourceDocumentType   string        `json:"source_document_type"`
	SourceRequestID      string        `json:"source_request_id"`
	SourceOutputID       string        `json:"source_output_id"`
	SourceWOID           string        `json:"source_wo_id"`
	SourceWOCode         string        `json:"source_wo_code"`
	SourceConfirmationID string        `json:"source_confirmation_id"`
	UpdatedAt            time.Time     `json:"updated_at"`
	Lines                []ReceiptLine `json:"lines"`
}

type ReceiptLine struct {
	LineID           string  `json:"line_id"`
	ItemRevisionID   string  `json:"item_revision_id"`
	ItemName         string  `json:"item_name"`
	LotCode          string  `json:"lot_code"`
	Quantity         float64 `json:"qty"`
	ReceivedQuantity float64 `json:"received_quantity"`
	Expected         float64 `json:"expected_quantity"`
	UOMCode          string  `json:"uom_code"`
	Version          int     `json:"row_version"`
}

// UnmarshalJSON keeps the PDA-side contract as a plain display string while
// accepting both the legacy WMS string snapshot and the localized object
// emitted by current Inbound responses.
func (line *ReceiptLine) UnmarshalJSON(data []byte) error {
	type alias ReceiptLine
	var envelope struct {
		*alias
		ItemName json.RawMessage `json:"item_name"`
	}
	envelope.alias = (*alias)(line)
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.ItemName) == 0 || string(envelope.ItemName) == "null" {
		return nil
	}
	var name string
	if err := json.Unmarshal(envelope.ItemName, &name); err == nil {
		line.ItemName = name
		return nil
	}
	var localized map[string]string
	if err := json.Unmarshal(envelope.ItemName, &localized); err != nil {
		return fmt.Errorf("item_name must be a string or localized object: %w", err)
	}
	for _, key := range []string{"vi", "en", "ja", "ko"} {
		if value := localized[key]; value != "" {
			line.ItemName = value
			return nil
		}
	}
	for _, value := range localized {
		if value != "" {
			line.ItemName = value
			break
		}
	}
	return nil
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

type ReceiptBatchLine struct {
	LineID         string  `json:"line_id"`
	ActualQuantity float64 `json:"actual_quantity"`
	UOMCode        string  `json:"uom_code"`
	ItemRevisionID string  `json:"item_revision_id"`
	LotCode        string  `json:"lot_code"`
}

type ReceiptBatchRequest struct {
	CommandID              string             `json:"command_id"`
	ExpectedReceiptVersion int64              `json:"expected_receipt_version"`
	Lines                  []ReceiptBatchLine `json:"lines"`
	IdempotencyKey         string             `json:"-"`
}

type ReceiptBatchResult struct {
	ReceiptID     string `json:"receipt_id"`
	Status        string `json:"status"`
	Result        string `json:"result"`
	CommandStatus string `json:"command_status"`
	LineCount     int    `json:"line_count"`
}
