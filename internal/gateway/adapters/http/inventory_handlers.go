package httpadapter

import (
	inventoryapp "github.com/company/pda-backend/internal/inventory/application"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"strings"
)

type transferInput struct {
	SourceLocation      string `json:"sourceLocation"`
	DestinationLocation string `json:"destinationLocation"`
	ItemID              string `json:"itemId"`
	Quantity            int64  `json:"quantity"`
}

func (r *Router) inventorySearch(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.Search(q.Context(), q.URL.Query().Get("q"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) inventoryBalances(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.Balances(q.Context(), q.URL.Query().Get("itemId"), q.URL.Query().Get("locationId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) inventoryMovements(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.Movements(q.Context(), q.URL.Query().Get("itemId"), q.URL.Query().Get("cursor"), r.actor(q))
	r.movementResponse(w, q, v, e)
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
	v, e := r.inventory.Transfer(q.Context(), inventoryapp.TransferCommand{Command: c, Source: x.SourceLocation, Destination: x.DestinationLocation, Item: x.ItemID, Quantity: x.Quantity})
	r.movementResponse(w, q, v, e)
}
func (r *Router) countList(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.ListCounts(q.Context(), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) countDetail(w http.ResponseWriter, q *http.Request) {
	v, e := r.inventory.CountDetail(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) countSubmit(w http.ResponseWriter, q *http.Request) {
	c, e := r.inventoryCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	var x struct {
		LineID   string `json:"lineId"`
		Quantity int64  `json:"quantity"`
	}
	if e = decode(q, &x); e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.inventory.SubmitCount(q.Context(), chi.URLParam(q, "taskId"), x.LineID, x.Quantity, c)
	r.movementResponse(w, q, v, e)
}
func (r *Router) countRecount(w http.ResponseWriter, q *http.Request) {
	c, e := r.inventoryCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	var x struct {
		LineID string `json:"lineId"`
	}
	if e = decode(q, &x); e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.inventory.Recount(q.Context(), chi.URLParam(q, "taskId"), x.LineID, c)
	r.movementResponse(w, q, v, e)
}
func (r *Router) countComplete(w http.ResponseWriter, q *http.Request) {
	c, e := r.inventoryCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.inventory.CompleteCount(q.Context(), chi.URLParam(q, "taskId"), c)
	r.movementResponse(w, q, v, e)
}
