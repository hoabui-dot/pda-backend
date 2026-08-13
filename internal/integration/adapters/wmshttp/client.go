package wmshttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/company/pda-backend/internal/integration/ports"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/google/uuid"
)

const warehousePath = "/api/wms/master-data/warehouses"
const locationPath = "/api/wms/master-data/locations"
const inboundReceiptsPath = "/api/wms/inbound/receipts"
const executionTasksPath = "/api/wms/execution/tasks"
const inventoryPath = "/api/wms/inventory"
const shippingPath = "/api/wms/shipping"
const barcodeResolvePath = "/api/wms/master-data/barcode/resolve"
const cycleCountsPath = "/api/wms/inventory/cycle-counts"

// Client implements the approved WMS public contract without exposing WMS DTOs
// beyond this adapter boundary.
type Client struct {
	baseURL          string
	token            string
	serviceToken     string
	warehouseAliases map[string]string
	http             *http.Client
}

func New(baseURL, token string, client *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("invalid WMS base URL")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("WMS bearer token is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = tracingTransport{base: transport}
	return &Client{
		baseURL:          strings.TrimRight(baseURL, "/"),
		token:            token,
		serviceToken:     strings.TrimSpace(os.Getenv("PDA_UPSTREAM_WMS_SERVICE_TOKEN")),
		warehouseAliases: parseWarehouseAliases(os.Getenv("PDA_UPSTREAM_WMS_WAREHOUSE_ALIASES")),
		http:             client,
	}, nil
}

type tracingTransport struct{ base http.RoundTripper }

func (t tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	started := time.Now()
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		slog.Default().Error("wms_upstream_transport_error", "method", req.Method, "path", req.URL.Path, "query", req.URL.RawQuery, "status", 0, "durationMs", time.Since(started).Milliseconds(), "traceId", req.Header.Get("X-Trace-ID"), "correlationId", req.Header.Get("X-Correlation-ID"), "error", err.Error())
		return nil, err
	}
	slog.Default().Info("wms_upstream_response", "method", req.Method, "path", req.URL.Path, "query", req.URL.RawQuery, "status", resp.StatusCode, "durationMs", time.Since(started).Milliseconds(), "traceId", req.Header.Get("X-Trace-ID"), "correlationId", req.Header.Get("X-Correlation-ID"))
	return resp, nil
}

// CanonicalWarehouse resolves a configured PDA/session alias to the WMS
// warehouse identity used by owner APIs. Empty configuration leaves the
// session value untouched, which keeps non-demo deployments explicit.
func (c *Client) CanonicalWarehouse(value string) string {
	value = strings.TrimSpace(value)
	if canonical, ok := c.warehouseAliases[strings.ToUpper(value)]; ok {
		return canonical
	}
	return value
}

// CanonicalWarehouseID resolves the PDA-facing warehouse alias/code to the
// UUID required by WMS owner APIs. It deliberately resolves through the WMS
// Master Data contract instead of maintaining a second warehouse directory in
// PDA Backend.
func (c *Client) CanonicalWarehouseID(ctx context.Context, value string) (string, error) {
	value = c.CanonicalWarehouse(value)
	if value == "*" {
		return "", nil
	}
	if _, err := uuid.Parse(value); err == nil {
		return value, nil
	}
	warehouses, err := c.Warehouses(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve WMS warehouse %q: %w", value, err)
	}
	for _, warehouse := range warehouses {
		if strings.EqualFold(warehouse.Code, value) {
			return warehouse.ID, nil
		}
	}
	return "", fmt.Errorf("WMS warehouse %q was not found", value)
}

// WarehouseMatches resolves the PDA session context at the WMS boundary.
// MAIN=* is a configured demo context for operators assigned to every WMS
// warehouse; it must not be confused with a physical warehouse ID.
func (c *Client) WarehouseMatches(location ports.Location, value string) bool {
	if strings.EqualFold(strings.TrimSpace(c.CanonicalWarehouse(value)), "*") {
		return true
	}
	canonical := c.CanonicalWarehouse(value)
	return strings.EqualFold(strings.TrimSpace(location.WarehouseID), strings.TrimSpace(canonical)) ||
		strings.EqualFold(strings.TrimSpace(location.WarehouseCode), strings.TrimSpace(canonical))
}

func parseWarehouseAliases(raw string) map[string]string {
	aliases := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			continue
		}
		aliases[strings.ToUpper(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
	}
	return aliases
}

type warehouseListResponse struct {
	Data []warehouse `json:"data"`
}

type warehouse struct {
	ID   string          `json:"warehouse_id"`
	Code string          `json:"warehouse_code"`
	Name json.RawMessage `json:"warehouse_name"`
}

func (c *Client) Warehouses(ctx context.Context) ([]ports.Warehouse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+warehousePath+"?limit=500", nil)
	if err != nil {
		return nil, fmt.Errorf("build WMS warehouse request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("WMS warehouse request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("WMS warehouse request returned HTTP %d", resp.StatusCode)
	}
	var payload warehouseListResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode WMS warehouse response: %w", err)
	}
	out := make([]ports.Warehouse, 0, len(payload.Data))
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Code) == "" {
			return nil, fmt.Errorf("WMS warehouse response contains an incomplete warehouse")
		}
		name, err := localizedName(item.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, ports.Warehouse{ID: item.ID, Code: item.Code, Name: name})
	}
	return out, nil
}

func (c *Client) Location(ctx context.Context, id string) (ports.Location, error) {
	var out ports.Location
	if err := c.getJSON(ctx, locationPath+"/"+url.PathEscape(id), &out); err != nil {
		return ports.Location{}, err
	}
	if out.ID == "" || out.WarehouseID == "" {
		return ports.Location{}, fmt.Errorf("WMS location response contains incomplete identity")
	}
	return out, nil
}

func (c *Client) ResolveBarcode(ctx context.Context, rawValue, symbology, warehouseID, taskID, lineID, scanID string) (ports.BarcodeResolution, error) {
	request := map[string]any{
		"raw_value":    rawValue,
		"symbology":    symbology,
		"scan_context": "RECEIVING_ITEM",
		"warehouse_id": warehouseID,
		"task_id":      taskID,
		"line_id":      lineID,
		"scan_id":      scanID,
		"scanned_at":   time.Now().UTC().Format(time.RFC3339Nano),
	}
	body, err := json.Marshal(request)
	if err != nil {
		return ports.BarcodeResolution{}, fmt.Errorf("encode WMS barcode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+barcodeResolvePath, strings.NewReader(string(body)))
	if err != nil {
		return ports.BarcodeResolution{}, fmt.Errorf("build WMS barcode request: %w", err)
	}
	c.applyHeaders(req, scanID)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return ports.BarcodeResolution{}, fmt.Errorf("WMS barcode request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ports.BarcodeResolution{}, statusError(resp)
	}
	var payload struct {
		NormalizedValue string `json:"normalized_value"`
		ItemRevisionID  string `json:"item_revision_id"`
		UOMCode         string `json:"uom_code"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return ports.BarcodeResolution{}, fmt.Errorf("decode WMS barcode response: %w", err)
	}
	if payload.ItemRevisionID == "" {
		return ports.BarcodeResolution{}, fmt.Errorf("WMS barcode response contains incomplete item identity")
	}
	return ports.BarcodeResolution{RawValue: rawValue, NormalizedValue: payload.NormalizedValue, ItemID: payload.ItemRevisionID, UOM: payload.UOMCode}, nil
}

func (c *Client) ListReceipts(ctx context.Context, query ports.ReceiptQuery) ([]ports.ReceiptSummary, error) {
	params := url.Values{}
	if query.Status != "" {
		params.Set("status", query.Status)
	}
	if query.WarehouseLocationID != "" {
		params.Set("warehouse_location_id", query.WarehouseLocationID)
	}
	if query.AssignedOperatorID != "" {
		params.Set("assigned_operator_id", query.AssignedOperatorID)
	}
	if query.Query != "" {
		params.Set("q", query.Query)
	}
	if query.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", query.Limit))
	}
	var payload struct {
		Data []ports.ReceiptSummary `json:"data"`
	}
	path := inboundReceiptsPath
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if err := c.getJSON(ctx, path, &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func (c *Client) GetReceipt(ctx context.Context, id string) (ports.Receipt, error) {
	var out ports.Receipt
	if err := c.getJSON(ctx, inboundReceiptsPath+"/"+url.PathEscape(id), &out); err != nil {
		return ports.Receipt{}, err
	}
	return out, nil
}

func (c *Client) ClaimReceipt(ctx context.Context, id, operatorID, idempotencyKey string, leaseSeconds int) error {
	body, err := json.Marshal(map[string]any{"operator_id": operatorID, "lease_seconds": leaseSeconds})
	if err != nil {
		return fmt.Errorf("encode WMS receipt claim request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+inboundReceiptsPath+"/"+url.PathEscape(id)+"/claim", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build WMS receipt claim request: %w", err)
	}
	c.applyHeaders(req, idempotencyKey)
	req.Header.Set("X-User-ID", operatorID)
	req.Header.Set("X-Operator-ID", operatorID)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("WMS receipt claim request: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read WMS receipt claim response: %w", err)
	}
	if err := ownerResponseError(resp.StatusCode, payload); err != nil {
		return err
	}
	return nil
}

func (c *Client) RecordReceiptQuantity(ctx context.Context, receiptID, lineID string, input ports.ReceiptQuantityRequest) (ports.ReceiptQuantityResult, error) {
	var out ports.ReceiptQuantityResult
	body, err := json.Marshal(input)
	if err != nil {
		return out, fmt.Errorf("encode WMS receipt quantity request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+inboundReceiptsPath+"/"+url.PathEscape(receiptID)+"/lines/"+url.PathEscape(lineID)+"/quantity", strings.NewReader(string(body)))
	if err != nil {
		return out, fmt.Errorf("build WMS receipt quantity request: %w", err)
	}
	c.applyHeaders(req, input.IdempotencyKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return out, fmt.Errorf("WMS receipt quantity request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return out, statusError(resp)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return out, fmt.Errorf("decode WMS receipt quantity response: %w", err)
	}
	return out, nil
}

func (c *Client) ConfirmReceipt(ctx context.Context, id, idempotencyKey string) (ports.ReceiptConfirmation, error) {
	var out ports.ReceiptConfirmation
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+inboundReceiptsPath+"/"+url.PathEscape(id)+"/confirm", nil)
	if err != nil {
		return ports.ReceiptConfirmation{}, fmt.Errorf("build WMS receipt confirmation request: %w", err)
	}
	c.applyHeaders(req, idempotencyKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return ports.ReceiptConfirmation{}, fmt.Errorf("WMS receipt confirmation request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return ports.ReceiptConfirmation{}, statusError(resp)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ports.ReceiptConfirmation{}, fmt.Errorf("read WMS receipt confirmation response: %w", err)
	}
	if err := ownerResponseError(resp.StatusCode, payload); err != nil {
		return ports.ReceiptConfirmation{}, err
	}
	if err := json.Unmarshal(payload, &out); err != nil {
		return ports.ReceiptConfirmation{}, fmt.Errorf("decode WMS receipt confirmation response: %w", err)
	}
	return out, nil
}

func (c *Client) ReceiveReceipt(ctx context.Context, id string, input ports.ReceiptBatchRequest) (ports.ReceiptBatchResult, error) {
	var out ports.ReceiptBatchResult
	body, err := json.Marshal(input)
	if err != nil {
		return out, fmt.Errorf("encode WMS receipt batch request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+inboundReceiptsPath+"/"+url.PathEscape(id)+"/receive", strings.NewReader(string(body)))
	if err != nil {
		return out, fmt.Errorf("build WMS receipt batch request: %w", err)
	}
	c.applyHeaders(req, input.IdempotencyKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return out, fmt.Errorf("WMS receipt batch request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return out, statusError(resp)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return out, fmt.Errorf("decode WMS receipt batch response: %w", err)
	}
	return out, nil
}

type executionTask struct {
	TaskID             string         `json:"task_id"`
	TaskType           string         `json:"task_type"`
	Status             string         `json:"status"`
	WarehouseID        string         `json:"warehouse_id"`
	AssignedOperatorID *string        `json:"assigned_operator_id"`
	Version            int64          `json:"version"`
	Priority           int            `json:"priority"`
	Details            map[string]any `json:"details"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type executionTaskCommand struct {
	CommandID       string  `json:"command_id,omitempty"`
	CommandType     string  `json:"command_type"`
	ExpectedVersion int64   `json:"expected_version"`
	CorrelationID   string  `json:"correlation_id,omitempty"`
	CausationID     string  `json:"causation_id,omitempty"`
	ConfirmQty      float64 `json:"confirm_qty,omitempty"`
}

func (c *Client) ListExecutionTasks(ctx context.Context, warehouseID, operatorID, category, status, query string, limit int) ([]executionTask, error) {
	var err error
	warehouseID, err = c.CanonicalWarehouseID(ctx, warehouseID)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	if warehouseID != "" {
		params.Set("warehouse_id", warehouseID)
	}
	if operatorID != "" {
		params.Set("operator_id", operatorID)
	}
	if category != "" {
		params.Set("task_type", category)
	}
	if status != "" {
		params.Set("status", status)
	}
	if query != "" {
		params.Set("q", query)
	}
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	var payload struct {
		Data []executionTask `json:"data"`
	}
	path := executionTasksPath
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	if err := c.getJSON(ctx, path, &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func (c *Client) GetExecutionTask(ctx context.Context, id string) (executionTask, error) {
	var out executionTask
	if err := c.getJSON(ctx, executionTasksPath+"/"+url.PathEscape(id), &out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *Client) ApplyExecutionTaskCommand(ctx context.Context, id string, command executionTaskCommand, actorID, idempotencyKey string) (executionTask, error) {
	body, err := json.Marshal(command)
	if err != nil {
		return executionTask{}, fmt.Errorf("encode WMS task command: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+executionTasksPath+"/"+url.PathEscape(id)+"/"+strings.ToLower(command.CommandType), strings.NewReader(string(body)))
	if err != nil {
		return executionTask{}, fmt.Errorf("build WMS task command request: %w", err)
	}
	c.applyHeaders(req, idempotencyKey)
	// PDA Backend acts as the authenticated service, while the operator role
	// remains explicit for Warehouse Execution authorization.
	req.Header.Set("X-Role-Code", "WMS_EXECUTION_OPERATOR")
	if actorID != "" {
		req.Header.Set("X-User-ID", actorID)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return executionTask{}, fmt.Errorf("WMS task command request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return executionTask{}, statusError(resp)
	}
	var payload struct {
		Task executionTask `json:"task"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return executionTask{}, fmt.Errorf("decode WMS task command response: %w", err)
	}
	return payload.Task, nil
}

func (c *Client) AllocateExecutionTask(ctx context.Context, id, commandID, traceID, actorID, idempotencyKey string) (executionTask, error) {
	body, err := json.Marshal(map[string]any{"command_id": commandID})
	if err != nil {
		return executionTask{}, fmt.Errorf("encode WMS task allocation request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+executionTasksPath+"/"+url.PathEscape(id)+"/allocate", strings.NewReader(string(body)))
	if err != nil {
		return executionTask{}, fmt.Errorf("build WMS task allocation request: %w", err)
	}
	c.applyHeaders(req, idempotencyKey)
	if actorID != "" {
		req.Header.Set("X-User-ID", actorID)
	}
	if traceID != "" {
		req.Header.Set("X-Trace-ID", traceID)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return executionTask{}, fmt.Errorf("WMS task allocation request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return executionTask{}, statusError(resp)
	}
	var out executionTask
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return executionTask{}, fmt.Errorf("decode WMS task allocation response: %w", err)
	}
	return out, nil
}

func (c *Client) RecordExecutionScan(ctx context.Context, id, scanType, value string, version int64, actorID, traceID string) error {
	body, err := json.Marshal(map[string]any{"scan_type": scanType, "scan_value": value, "expected_version": version})
	if err != nil {
		return fmt.Errorf("encode WMS task scan: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+executionTasksPath+"/"+url.PathEscape(id)+"/scans", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build WMS task scan request: %w", err)
	}
	c.applyHeaders(req, "")
	// Scan validation is an operator action forwarded by PDA Backend.
	req.Header.Set("X-Role-Code", "WMS_EXECUTION_OPERATOR")
	if actorID != "" {
		req.Header.Set("X-User-ID", actorID)
	}
	if traceID != "" {
		req.Header.Set("X-Trace-ID", traceID)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("WMS task scan request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return statusError(resp)
	}
	return nil
}

type inventoryBalance struct {
	LotID          string  `json:"lot_id"`
	LotCode        string  `json:"lot_code"`
	LocationID     string  `json:"location_id"`
	ItemRevisionID string  `json:"item_revision_id"`
	OnHandQty      float64 `json:"on_hand_qty"`
	ReservedQty    float64 `json:"reserved_qty"`
	AvailableQty   float64 `json:"available_qty"`
	Status         string  `json:"status"`
	RowVersion     int     `json:"row_version"`
}

type inventoryMovement struct {
	MovementID     string  `json:"movement_id"`
	MovementType   string  `json:"movement_type"`
	LotID          string  `json:"lot_id"`
	LotCode        string  `json:"lot_code"`
	ItemRevisionID string  `json:"item_revision_id"`
	FromLocationID *string `json:"from_location_id"`
	ToLocationID   *string `json:"to_location_id"`
	Qty            float64 `json:"qty"`
	OccurredAt     string  `json:"occurred_at"`
}

type cycleCountLine struct {
	ID                string   `json:"count_line_id"`
	ItemID            string   `json:"item_revision_id"`
	SnapshotQuantity  *float64 `json:"snapshot_quantity"`
	SubmittedQuantity *float64 `json:"submitted_quantity"`
	Variance          *float64 `json:"variance_quantity"`
	Status            string   `json:"status"`
	Version           int64    `json:"version"`
}

type cycleCountTask struct {
	ID          string           `json:"count_task_id"`
	WarehouseID string           `json:"warehouse_id"`
	LocationID  string           `json:"location_id"`
	Status      string           `json:"status"`
	BlindCount  bool             `json:"blind_count"`
	OperatorID  *string          `json:"assigned_operator_id"`
	Version     int64            `json:"version"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Lines       []cycleCountLine `json:"lines"`
}

func (c *Client) ListCycleCounts(ctx context.Context, warehouseID, operatorID, status string) ([]cycleCountTask, error) {
	var err error
	warehouseID, err = c.CanonicalWarehouseID(ctx, warehouseID)
	if err != nil {
		return nil, err
	}
	params := url.Values{"warehouse_id": []string{warehouseID}}
	if operatorID != "" {
		params.Set("operator_id", operatorID)
	}
	if status != "" {
		params.Set("status", status)
	}
	var out []cycleCountTask
	if err := c.getJSON(ctx, cycleCountsPath+"?"+params.Encode(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetCycleCount(ctx context.Context, id, warehouseID string) (cycleCountTask, error) {
	var out cycleCountTask
	path := cycleCountsPath + "/" + url.PathEscape(id) + "?warehouse_id=" + url.QueryEscape(warehouseID)
	if err := c.getJSON(ctx, path, &out); err != nil {
		return out, err
	}
	return out, nil
}

func (c *Client) SubmitCycleCount(ctx context.Context, taskID, lineID string, quantity int64, baseVersion int64, idempotencyKey, deviceID, operatorID string) error {
	body, err := json.Marshal(map[string]any{"quantity": quantity, "base_version": baseVersion, "idempotency_key": idempotencyKey, "device_id": deviceID})
	if err != nil {
		return fmt.Errorf("encode WMS cycle count request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+cycleCountsPath+"/"+url.PathEscape(taskID)+"/lines/"+url.PathEscape(lineID)+"/submit", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build WMS cycle count request: %w", err)
	}
	c.applyHeaders(req, idempotencyKey)
	if operatorID != "" {
		req.Header.Set("X-User-ID", operatorID)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("WMS cycle count request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return statusError(resp)
	}
	return nil
}

func (c *Client) RecountCycleCount(ctx context.Context, taskID, lineID string, commandID string, baseVersion int64, idempotencyKey string) error {
	return c.cycleCountCommand(ctx, cycleCountsPath+"/"+url.PathEscape(taskID)+"/lines/"+url.PathEscape(lineID)+"/recount", commandID, baseVersion, idempotencyKey)
}

func (c *Client) CompleteCycleCount(ctx context.Context, taskID string, commandID string, baseVersion int64, idempotencyKey string) error {
	return c.cycleCountCommand(ctx, cycleCountsPath+"/"+url.PathEscape(taskID)+"/complete", commandID, baseVersion, idempotencyKey)
}

func (c *Client) cycleCountCommand(ctx context.Context, path, commandID string, baseVersion int64, idempotencyKey string) error {
	body, err := json.Marshal(map[string]any{"command_id": commandID, "expected_version": baseVersion, "idempotency_key": idempotencyKey})
	if err != nil {
		return fmt.Errorf("encode WMS cycle count command: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build WMS cycle count command: %w", err)
	}
	c.applyHeaders(req, idempotencyKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("WMS cycle count command: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return statusError(resp)
	}
	return nil
}

type inventoryTransferResult struct {
	CommandID      string  `json:"command_id"`
	MovementID     string  `json:"movement_id"`
	LotID          string  `json:"lot_id"`
	FromLocationID string  `json:"from_location_id"`
	ToLocationID   string  `json:"to_location_id"`
	Qty            float64 `json:"qty"`
	Replay         bool    `json:"replayed"`
	Result         string  `json:"result"`
}

func (c *Client) TransferInventory(ctx context.Context, commandID, lotID, from, to string, quantity int64, actorID, traceID string) (inventoryTransferResult, error) {
	body, err := json.Marshal(map[string]any{"command_id": commandID, "lot_id": lotID, "from_location_id": from, "to_location_id": to, "qty": quantity})
	if err != nil {
		return inventoryTransferResult{}, fmt.Errorf("encode WMS inventory transfer: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+inventoryPath+"/movements/internal-transfer", strings.NewReader(string(body)))
	if err != nil {
		return inventoryTransferResult{}, fmt.Errorf("build WMS inventory transfer request: %w", err)
	}
	c.applyHeaders(req, commandID)
	req.Header.Set("X-Role-Code", "SYSTEM")
	if actorID != "" {
		req.Header.Set("X-User-ID", actorID)
	}
	if traceID != "" {
		req.Header.Set("X-Trace-ID", traceID)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return inventoryTransferResult{}, fmt.Errorf("WMS inventory transfer request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return inventoryTransferResult{}, statusError(resp)
	}
	var payload struct {
		Data inventoryTransferResult `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return inventoryTransferResult{}, fmt.Errorf("decode WMS inventory transfer response: %w", err)
	}
	return payload.Data, nil
}

func (c *Client) ListInventoryBalances(ctx context.Context, item, location string) ([]inventoryBalance, error) {
	params := url.Values{}
	if item != "" {
		params.Set("item_revision_id", item)
	}
	if location != "" {
		params.Set("location_id", location)
	}
	path := inventoryPath + "/balances"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	payload, err := c.getJSONBytes(ctx, path)
	if err != nil {
		return nil, err
	}
	var rows []inventoryBalance
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, fmt.Errorf("decode WMS inventory balances response: %w", err)
		}
		return rows, nil
	}
	var envelope struct {
		Data []inventoryBalance `json:"data"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, fmt.Errorf("decode WMS inventory balances response: %w", err)
	}
	return envelope.Data, nil
}

func (c *Client) ListInventoryMovements(ctx context.Context, item, location, cursor string) ([]inventoryMovement, error) {
	params := url.Values{}
	if item != "" {
		params.Set("item_revision_id", item)
	}
	if location != "" {
		params.Set("location_id", location)
	}
	path := inventoryPath + "/movements"
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var payload struct {
		Data []inventoryMovement `json:"data"`
	}
	if err := c.getJSON(ctx, path, &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func (c *Client) GetShippingShipment(ctx context.Context, id string) (map[string]any, error) {
	var payload map[string]any
	if err := c.getJSON(ctx, shippingPath+"/shipments/"+url.PathEscape(id), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c *Client) VerifyShippingPackage(ctx context.Context, packageID, barcode, commandID string, expectedVersion int64, idempotencyKey string) error {
	body, err := json.Marshal(map[string]any{
		"barcode":          barcode,
		"command_id":       commandID,
		"expected_version": expectedVersion,
	})
	if err != nil {
		return fmt.Errorf("encode WMS package verification request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+shippingPath+"/packages/"+url.PathEscape(packageID)+"/verify", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build WMS package verification request: %w", err)
	}
	c.applyHeaders(req, idempotencyKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("WMS package verification request: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read WMS package verification response: %w", err)
	}
	if err := ownerResponseError(resp.StatusCode, payload); err != nil {
		return err
	}
	return nil
}

// ConfirmShipping invokes the owner command. Carrier and tracking are not
// sent because Shipping validates the values stored on its Shipment aggregate.
func (c *Client) ConfirmShipping(ctx context.Context, shipmentID, commandID string, expectedVersion int64, idempotencyKey string) (map[string]any, error) {
	body, err := json.Marshal(map[string]any{
		"command_id":       commandID,
		"expected_version": expectedVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("encode WMS shipment confirmation request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+shippingPath+"/shipments/"+url.PathEscape(shipmentID)+"/confirm", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("build WMS shipment confirmation request: %w", err)
	}
	c.applyHeaders(req, idempotencyKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("WMS shipment confirmation request: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read WMS shipment confirmation response: %w", err)
	}
	if err := ownerResponseError(resp.StatusCode, payload); err != nil {
		return nil, err
	}
	var result map[string]any
	if len(payload) > 0 && string(payload) != "null" {
		if err := json.Unmarshal(payload, &result); err != nil {
			return nil, fmt.Errorf("decode WMS shipment confirmation response: %w", err)
		}
	}
	return result, nil
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	payload, err := c.getJSONBytes(ctx, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(payload, target); err != nil {
		slog.Default().Error("wms_upstream_decode_error", "path", path, "status", http.StatusOK, "error", err.Error(), "payload", strings.TrimSpace(string(payload)))
		return fmt.Errorf("decode WMS read response: %w", err)
	}
	return nil
}

func (c *Client) getJSONBytes(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build WMS read request: %w", err)
	}
	c.applyHeaders(req, "")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("WMS read request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, statusError(resp)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read WMS read response: %w", err)
	}
	return payload, nil
}

func (c *Client) applyHeaders(req *http.Request, idempotencyKey string) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("X-Calling-Service", "PDA_BACKEND")
	if c.serviceToken != "" {
		req.Header.Set("X-Service-Token", c.serviceToken)
	}
	req.Header.Set("Accept", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
}

func statusError(resp *http.Response) error {
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	slog.Default().Error("wms_upstream_error_body", "status", resp.StatusCode, "errorBody", strings.TrimSpace(string(payload)))
	return ownerResponseError(resp.StatusCode, payload)
}

func ownerResponseError(status int, payload []byte) error {
	var body struct {
		Error   string `json:"error"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(payload, &body)
	// WMS uses `error` for both machine codes and human text (for example
	// `Not Found`). Never expose the latter as a gateway code; prefer the
	// explicit `code` field and fall back to HTTP semantics for unstructured
	// owner responses.
	code := strings.TrimSpace(body.Code)
	if code == "" && isMachineErrorCode(body.Error) {
		code = strings.TrimSpace(body.Error)
	}
	if status >= http.StatusOK && status < http.StatusMultipleChoices && code == "" {
		return nil
	}
	if code == "" {
		switch status {
		case http.StatusUnauthorized:
			code = "UPSTREAM_UNAUTHORIZED"
		case http.StatusForbidden:
			code = "WAREHOUSE_ACCESS_DENIED"
		case http.StatusNotFound:
			code = "UPSTREAM_NOT_FOUND"
		case http.StatusConflict:
			code = "UPSTREAM_CONFLICT"
		default:
			code = "UPSTREAM_HTTP_ERROR"
		}
	}
	message := strings.TrimSpace(body.Message)
	if message == "" {
		message = "WMS owner request failed"
	}
	return &platform.DomainError{Code: code, SafeMessage: message, Retryable: status >= 500 || status == http.StatusTooManyRequests}
}

func isMachineErrorCode(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func localizedName(raw json.RawMessage) (string, error) {
	var localized map[string]string
	if err := json.Unmarshal(raw, &localized); err == nil {
		for _, language := range []string{"vi", "en", "ja", "ko"} {
			if value := strings.TrimSpace(localized[language]); value != "" {
				return value, nil
			}
		}
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil && strings.TrimSpace(value) != "" {
		return value, nil
	}
	return "", fmt.Errorf("WMS warehouse response contains no supported warehouse name")
}
