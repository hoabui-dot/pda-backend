package httpadapter

import (
	inventoryapp "github.com/company/pda-backend/internal/inventory/application"
	inventorydomain "github.com/company/pda-backend/internal/inventory/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type transferInput struct {
	TaskID              string    `json:"taskId"`
	SourceLocation      string    `json:"sourceLocation"`
	DestinationLocation string    `json:"destinationLocation"`
	ItemID              string    `json:"itemId"`
	Quantity            int64     `json:"quantity"`
	LotNumber           string    `json:"lotNumber"`
	SerialNumber        string    `json:"serialNumber"`
	ScanContext         string    `json:"scanContext"`
	BaseVersion         int64     `json:"baseVersion"`
	CommandID           uuid.UUID `json:"commandId"`
	IdempotencyKey      string    `json:"idempotencyKey"`
	Reason              string    `json:"reason"`
}

func (r *Router) inventorySearch(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.Search(q.Context(), "", r.actor(q))
	if e == nil {
		v = filterBalances(v, q)
	}
	inventoryResponse(w, q, v, e, r.now())
}
func (r *Router) inventoryBalances(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.Balances(q.Context(), "", "", r.actor(q))
	if e == nil {
		v = filterBalances(v, q)
	}
	inventoryResponse(w, q, v, e, r.now())
}
func (r *Router) inventoryMovements(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.Movements(q.Context(), q.URL.Query().Get("itemId"), q.URL.Query().Get("cursor"), r.actor(q))
	inventoryResponse(w, q, v, e, r.now())
}

func inventoryResponse(w http.ResponseWriter, q *http.Request, value any, err error, now time.Time) {
	if err != nil {
		writeError(w, err, correlation(q.Context()))
		return
	}
	writeData(w, http.StatusOK, map[string]any{"items": value, "asOf": now.UTC(), "stale": false}, correlation(q.Context()), now)
}

func filterBalances(values []inventorydomain.Balance, q *http.Request) []inventorydomain.Balance {
	query := strings.ToLower(q.URL.Query().Get("q"))
	item := strings.ToLower(q.URL.Query().Get("itemId"))
	barcode := strings.ToLower(q.URL.Query().Get("barcode"))
	location := strings.ToLower(q.URL.Query().Get("locationCode"))
	if location == "" {
		location = strings.ToLower(q.URL.Query().Get("locationId"))
	}
	result := make([]inventorydomain.Balance, 0, len(values))
	for _, value := range values {
		code := strings.ToLower(value.ItemID)
		loc := strings.ToLower(value.LocationID)
		if query != "" && !strings.Contains(code, query) && !strings.Contains(loc, query) || item != "" && code != item || barcode != "" && code != barcode || location != "" && loc != location {
			continue
		}
		result = append(result, value)
	}
	limit := 50
	if raw := q.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}
func (r *Router) transferValidation(w http.ResponseWriter, q *http.Request) {
	var x transferInput
	if e := decode(q, &x); e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	e := r.inventory.ValidateTransfer(q.Context(), inventoryapp.TransferCommand{Source: x.SourceLocation, Destination: x.DestinationLocation, Item: x.ItemID, Quantity: x.Quantity, Command: inventoryapp.Command{Actor: r.actor(q)}})
	r.movementResponse(w, q, map[string]any{"valid": e == nil}, e)
}
func (r *Router) transferSource(w http.ResponseWriter, q *http.Request) { r.transferValidation(w, q) }
func (r *Router) transferDestination(w http.ResponseWriter, q *http.Request) {
	r.transferValidation(w, q)
}
func (r *Router) transferItem(w http.ResponseWriter, q *http.Request) { r.transferValidation(w, q) }
func (r *Router) inventoryCommand(q *http.Request) (inventoryapp.Command, error) {
	key := q.Header.Get("Idempotency-Key")
	id, e := uuid.Parse(key)
	if e != nil {
		return inventoryapp.Command{}, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "UUID Idempotency-Key required"}
	}
	v := int64(0)
	if raw := strings.Trim(q.Header.Get("If-Match"), "\""); raw != "" {
		v, e = strconv.ParseInt(raw, 10, 64)
		if e != nil {
			return inventoryapp.Command{}, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Numeric If-Match required"}
		}
	}
	return inventoryapp.Command{ID: id, Key: key, BaseVersion: v, Actor: r.actor(q)}, nil
}
func (r *Router) transferConfirm(w http.ResponseWriter, q *http.Request) {
	c, e := r.inventoryCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	var x transferInput
	if e = decode(q, &x); e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	if x.TaskID != "" && x.TaskID != chi.URLParam(q, "taskId") || x.ScanContext != "" && x.ScanContext != "TRANSFER_DESTINATION" && x.ScanContext != "TRANSFER_ITEM" && x.ScanContext != "TRANSFER_SOURCE" {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "BARCODE_WRONG_CONTEXT", SafeMessage: "Transfer scan context is invalid"})
		return
	}
	if x.CommandID != uuid.Nil && x.CommandID != c.ID || x.IdempotencyKey != "" && x.IdempotencyKey != c.Key || x.BaseVersion != 0 && x.BaseVersion != c.BaseVersion {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Transfer command metadata does not match headers"})
		return
	}
	v, e := r.inventory.Transfer(q.Context(), inventoryapp.TransferCommand{Command: c, Source: x.SourceLocation, Destination: x.DestinationLocation, Item: x.ItemID, Quantity: x.Quantity})
	r.movementResponse(w, q, v, e)
}

func (r *Router) transferCommandStatus(w http.ResponseWriter, q *http.Request) {
	id, err := uuid.Parse(chi.URLParam(q, "commandId"))
	if err != nil {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Invalid command ID"}, correlation(q.Context()))
		return
	}
	status, err := r.inventory.CommandStatus(q.Context(), id, r.actor(q))
	r.movementResponse(w, q, status, err)
}
func (r *Router) countList(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.ListCounts(q.Context(), r.actor(q))
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	items := make([]map[string]any, 0, len(v))
	for _, task := range v {
		items = append(items, countView(task))
	}
	writeData(w, http.StatusOK, map[string]any{"items": items, "asOf": r.now().UTC(), "stale": false}, correlation(q.Context()), r.now())
}
func (r *Router) countDetail(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.CountDetail(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	writeData(w, http.StatusOK, countView(v), correlation(q.Context()), r.now())
}
func (r *Router) countValidateLocation(w http.ResponseWriter, q *http.Request) {
	var x struct {
		Location string `json:"location"`
		TaskID   string `json:"taskId"`
	}
	if err := decode(q, &x); err != nil {
		r.movementResponse(w, q, nil, err)
		return
	}
	if x.TaskID != "" && x.TaskID != chi.URLParam(q, "taskId") {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Task context does not match the path"})
		return
	}
	err := r.inventory.ValidateCountLocation(q.Context(), chi.URLParam(q, "taskId"), x.Location, r.actor(q))
	r.movementResponse(w, q, map[string]any{"valid": err == nil, "nextStep": "VALIDATE_ITEM"}, err)
}
func (r *Router) countValidateItem(w http.ResponseWriter, q *http.Request) {
	var x struct {
		Item   string `json:"item"`
		ItemID string `json:"itemId"`
		LineID string `json:"lineId"`
	}
	if err := decode(q, &x); err != nil {
		r.movementResponse(w, q, nil, err)
		return
	}
	item := x.Item
	if item == "" {
		item = x.ItemID
	}
	err := r.inventory.ValidateCountItem(q.Context(), chi.URLParam(q, "taskId"), x.LineID, item, r.actor(q))
	r.movementResponse(w, q, map[string]any{"valid": err == nil, "nextStep": "SUBMIT_COUNT"}, err)
}
func (r *Router) countSubmit(w http.ResponseWriter, q *http.Request) {
	c, e := r.inventoryCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	var x struct {
		TaskID          string    `json:"taskId"`
		LineID          string    `json:"lineId"`
		Quantity        int64     `json:"quantity"`
		CountedQuantity int64     `json:"countedQuantity"`
		CommandID       uuid.UUID `json:"commandId"`
		IdempotencyKey  string    `json:"idempotencyKey"`
		BaseVersion     int64     `json:"baseVersion"`
		Location        string    `json:"location"`
		Item            string    `json:"item"`
		Lot             string    `json:"lot"`
		Serial          string    `json:"serial"`
		BlindCount      bool      `json:"blindCount"`
		ReasonCode      string    `json:"reasonCode"`
		Recount         bool      `json:"recount"`
	}
	if e = decode(q, &x); e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	if x.CountedQuantity > 0 {
		x.Quantity = x.CountedQuantity
	}
	if x.TaskID != "" && x.TaskID != chi.URLParam(q, "taskId") || x.CommandID != uuid.Nil && x.CommandID != c.ID || x.IdempotencyKey != "" && x.IdempotencyKey != c.Key || x.BaseVersion != 0 && x.BaseVersion != c.BaseVersion {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Count command metadata does not match headers"})
		return
	}
	lineID := x.LineID
	if lineID == "" {
		lineID = chi.URLParam(q, "lineId")
	}
	v, e := r.inventory.SubmitCount(q.Context(), chi.URLParam(q, "taskId"), lineID, x.Quantity, c)
	if e == nil {
		vmap := countView(v)
		vmap["auditId"] = c.ID
		vmap["nextStep"] = "REVIEW_VARIANCE"
		writeData(w, http.StatusOK, vmap, correlation(q.Context()), r.now())
		return
	}
	r.movementResponse(w, q, nil, e)
}
func (r *Router) countRecount(w http.ResponseWriter, q *http.Request) {
	c, e := r.inventoryCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	var x struct {
		LineID         string    `json:"lineId"`
		CommandID      uuid.UUID `json:"commandId"`
		IdempotencyKey string    `json:"idempotencyKey"`
		BaseVersion    int64     `json:"baseVersion"`
	}
	if e = decode(q, &x); e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	if x.CommandID != uuid.Nil && x.CommandID != c.ID || x.IdempotencyKey != "" && x.IdempotencyKey != c.Key || x.BaseVersion != 0 && x.BaseVersion != c.BaseVersion {
		r.movementResponse(w, q, nil, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Count command metadata does not match headers"})
		return
	}
	v, e := r.inventory.Recount(q.Context(), chi.URLParam(q, "taskId"), x.LineID, c)
	if e == nil {
		view := countView(v)
		view["auditId"] = c.ID
		view["nextStep"] = "SUBMIT_COUNT"
		writeData(w, http.StatusOK, view, correlation(q.Context()), r.now())
		return
	}
	r.movementResponse(w, q, nil, e)
}
func (r *Router) countComplete(w http.ResponseWriter, q *http.Request) {
	c, e := r.inventoryCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.inventory.CompleteCount(q.Context(), chi.URLParam(q, "taskId"), c)
	if e == nil {
		view := countView(v)
		view["auditId"] = c.ID
		view["nextStep"] = "COMPLETED"
		writeData(w, http.StatusOK, view, correlation(q.Context()), r.now())
		return
	}
	r.movementResponse(w, q, nil, e)
}

func (r *Router) countCommandStatus(w http.ResponseWriter, q *http.Request) {
	id, err := uuid.Parse(chi.URLParam(q, "commandId"))
	if err != nil {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Invalid command ID"}, correlation(q.Context()))
		return
	}
	status, err := r.inventory.CountCommandStatus(q.Context(), id, r.actor(q))
	r.movementResponse(w, q, status, err)
}

func countView(task inventorydomain.CountTask) map[string]any {
	lines := make([]map[string]any, 0, len(task.Lines))
	for _, line := range task.Lines {
		varianceState := "NONE"
		variance := int64(0)
		if line.Variance != nil {
			variance = *line.Variance
			if variance < 0 {
				varianceState = "UNDER"
			} else if variance > 0 {
				varianceState = "OVER"
			}
		}
		lineView := map[string]any{"lineId": line.ID, "itemId": line.ItemID, "countedQuantity": line.CountedQuantity, "variance": line.Variance, "varianceState": varianceState, "reviewRequired": variance != 0, "approvalRequired": variance != 0, "recountRequired": line.RecountRequired}
		if !task.BlindCount {
			lineView["systemQuantity"] = line.ExpectedQuantity
		}
		lines = append(lines, lineView)
	}
	return map[string]any{"taskId": task.ID, "warehouseId": task.WarehouseID, "locationId": task.LocationID, "status": task.Status, "assignedOperatorId": task.OperatorID, "version": task.Version, "blindCount": task.BlindCount, "tolerance": task.Tolerance, "lines": lines, "updatedAt": task.UpdatedAt, "asOf": task.UpdatedAt}
}
