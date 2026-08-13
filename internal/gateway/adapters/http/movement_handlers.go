package httpadapter

import (
	"encoding/json"
	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	movementdomain "github.com/company/pda-backend/internal/execution/movement/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (r *Router) movementCommand(q *http.Request) (movementapp.Command, error) {
	key := q.Header.Get("Idempotency-Key")
	id, e := uuid.Parse(key)
	if e != nil {
		return movementapp.Command{}, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "A UUID Idempotency-Key is required"}
	}
	v, e := strconv.ParseInt(strings.Trim(q.Header.Get("If-Match"), "\""), 10, 64)
	if e != nil {
		return movementapp.Command{}, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "A numeric If-Match version is required"}
	}
	return movementapp.Command{CommandID: id, IdempotencyKey: key, TaskID: chi.URLParam(q, "taskId"), BaseVersion: v, Actor: r.actor(q)}, nil
}
func movementValue(q *http.Request) (string, error) {
	var x struct {
		Value           string     `json:"value"`
		Location        string     `json:"location"`
		Barcode         string     `json:"barcode"`
		RawValue        string     `json:"rawValue"`
		NormalizedValue string     `json:"normalizedValue"`
		Symbology       string     `json:"symbology"`
		ScanContext     string     `json:"scanContext"`
		TaskID          string     `json:"taskId"`
		BaseVersion     int64      `json:"baseVersion"`
		CommandID       uuid.UUID  `json:"commandId"`
		IdempotencyKey  string     `json:"idempotencyKey"`
		ScannedAt       *time.Time `json:"scannedAt"`
	}
	if e := decode(q, &x); e != nil {
		return "", e
	}
	if x.TaskID != "" && x.TaskID != chi.URLParam(q, "taskId") {
		return "", &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Task context does not match the path"}
	}
	if err := validateMovementMirrors(q, x.TaskID, x.CommandID, x.IdempotencyKey, x.BaseVersion); err != nil {
		return "", err
	}
	if x.ScanContext != "" && x.ScanContext != "PUTAWAY_SOURCE" && x.ScanContext != "PUTAWAY_ITEM" && x.ScanContext != "PUTAWAY_LOT" && x.ScanContext != "PUTAWAY_DESTINATION" && x.ScanContext != "PICKING_LOCATION" && x.ScanContext != "PICKING_ITEM" && x.ScanContext != "REPLENISHMENT_SOURCE" && x.ScanContext != "REPLENISHMENT_ITEM" && x.ScanContext != "REPLENISHMENT_DESTINATION" {
		return "", &platform.DomainError{Code: "BARCODE_WRONG_CONTEXT", SafeMessage: "Scanner context is invalid"}
	}
	if x.NormalizedValue != "" {
		return x.NormalizedValue, nil
	}
	if x.RawValue != "" {
		return x.RawValue, nil
	}
	if x.Value != "" {
		return x.Value, nil
	}
	if x.Location != "" {
		return x.Location, nil
	}
	if x.Barcode != "" {
		return x.Barcode, nil
	}
	return "", &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Validation value is required"}
}
func movementQuantity(q *http.Request) (int64, error) {
	var x struct {
		Quantity       int64      `json:"quantity"`
		TaskID         string     `json:"taskId"`
		CommandID      uuid.UUID  `json:"commandId"`
		IdempotencyKey string     `json:"idempotencyKey"`
		BaseVersion    int64      `json:"baseVersion"`
		ScannedAt      *time.Time `json:"scannedAt"`
	}
	if e := decode(q, &x); e != nil {
		return 0, e
	}
	if err := validateMovementMirrors(q, x.TaskID, x.CommandID, x.IdempotencyKey, x.BaseVersion); err != nil {
		return 0, err
	}
	if x.Quantity <= 0 {
		return 0, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Positive quantity is required"}
	}
	return x.Quantity, nil
}
func (r *Router) movementResponse(w http.ResponseWriter, q *http.Request, v any, e error) {
	if e != nil {
		writeError(w, e, correlation(q.Context()))
		return
	}
	if task, ok := v.(movementdomain.Task); ok {
		v = movementView(task)
	}
	if tasks, ok := v.([]movementdomain.Task); ok {
		items := make([]map[string]any, 0, len(tasks))
		for _, task := range tasks {
			items = append(items, movementView(task))
		}
		now := r.now().UTC()
		v = map[string]any{"items": items, "asOf": now, "stale": false}
	}
	writeData(w, http.StatusOK, v, correlation(q.Context()), r.now())
}

func validateMovementMirrors(q *http.Request, taskID string, commandID uuid.UUID, key string, version int64) error {
	if taskID != "" && taskID != chi.URLParam(q, "taskId") {
		return &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Task context does not match the path"}
	}
	headerKey := q.Header.Get("Idempotency-Key")
	if key != "" && key != headerKey {
		return &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Idempotency key does not match the header"}
	}
	headerID, _ := uuid.Parse(headerKey)
	if commandID != uuid.Nil && commandID != headerID {
		return &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Command ID does not match the idempotency header"}
	}
	if version != 0 {
		headerVersion, err := strconv.ParseInt(strings.Trim(q.Header.Get("If-Match"), `"`), 10, 64)
		if err != nil || version != headerVersion {
			return &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Base version does not match If-Match"}
		}
	}
	return nil
}

func movementView(task movementdomain.Task) map[string]any {
	encoded, _ := json.Marshal(task)
	view := map[string]any{}
	_ = json.Unmarshal(encoded, &view)
	view["remainingQuantity"] = task.Remaining()
	view["progressPercent"] = float64(task.CompletedQuantity) * 100 / float64(maxInt64(task.RequiredQuantity, 1))
	view["shortPickPolicy"] = "DISABLED"
	view["sourceBalance"] = nil
	view["destinationBalance"] = nil
	next := "CONFIRM_QUANTITY"
	if !task.SourceValidated {
		next = "VALIDATE_SOURCE"
	} else if !task.ItemValidated {
		next = "VALIDATE_ITEM"
	} else if task.Lot != "" && !task.LotValidated {
		next = "VALIDATE_LOT"
	} else if !task.DestinationValidated {
		next = "VALIDATE_DESTINATION"
	} else if task.Status == movementdomain.Completed {
		next = "COMPLETED"
	}
	view["nextStep"] = next
	return view
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
func (r *Router) movementValidation(w http.ResponseWriter, q *http.Request, f func(movementapp.Command, string) (movementdomain.Task, error)) {
	c, e := r.movementCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := movementValue(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	out, e := f(c, v)
	r.movementResponse(w, q, out, e)
}
func (r *Router) movementConfirm(w http.ResponseWriter, q *http.Request, f func(movementapp.Command, int64) (movementdomain.Task, error)) {
	c, e := r.movementCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := movementQuantity(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	out, e := f(c, v)
	r.movementResponse(w, q, out, e)
}
func (r *Router) putawayList(w http.ResponseWriter, q *http.Request) {
	v, e := r.putaway.List(q.Context(), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) putawayDetail(w http.ResponseWriter, q *http.Request) {
	v, e := r.putaway.Detail(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) putawayClaim(w http.ResponseWriter, q *http.Request) {
	c, e := r.movementCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.putaway.Claim(q.Context(), c)
	r.movementResponse(w, q, v, e)
}
func (r *Router) putawayStart(w http.ResponseWriter, q *http.Request) {
	c, e := r.movementCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.putaway.Start(q.Context(), c)
	r.movementResponse(w, q, v, e)
}
func (r *Router) putawaySuggestions(w http.ResponseWriter, q *http.Request) {
	v, e := r.putaway.Suggestions(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) putawaySource(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.putaway.ValidateSource(q.Context(), c, v)
	})
}
func (r *Router) putawayItem(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.putaway.ValidateItem(q.Context(), c, v)
	})
}
func (r *Router) putawayLot(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.putaway.ValidateLot(q.Context(), c, v)
	})
}
func (r *Router) putawayDestination(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.putaway.ValidateDestination(q.Context(), c, v)
	})
}
func (r *Router) putawayConfirm(w http.ResponseWriter, q *http.Request) {
	r.movementConfirm(w, q, func(c movementapp.Command, v int64) (movementdomain.Task, error) {
		return r.putaway.Confirm(q.Context(), c, v)
	})
}
func (r *Router) pickingList(w http.ResponseWriter, q *http.Request) {
	v, e := r.picking.List(q.Context(), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) pickingDetail(w http.ResponseWriter, q *http.Request) {
	v, e := r.picking.Detail(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) pickingAllocate(w http.ResponseWriter, q *http.Request) {
	c, e := r.movementCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.picking.Allocate(q.Context(), c)
	r.movementResponse(w, q, v, e)
}
func (r *Router) pickingLocation(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.picking.ValidateLocation(q.Context(), c, v)
	})
}
func (r *Router) pickingBarcode(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.picking.ResolveBarcode(q.Context(), c, v)
	})
}
func (r *Router) pickingConfirm(w http.ResponseWriter, q *http.Request) {
	r.movementConfirm(w, q, func(c movementapp.Command, v int64) (movementdomain.Task, error) {
		return r.picking.Confirm(q.Context(), c, v)
	})
}
func (r *Router) pickingComplete(w http.ResponseWriter, q *http.Request) {
	c, e := r.movementCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.picking.Complete(q.Context(), c)
	r.movementResponse(w, q, v, e)
}
func (r *Router) replenishmentList(w http.ResponseWriter, q *http.Request) {
	v, e := r.replenishment.List(q.Context(), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) replenishmentDetail(w http.ResponseWriter, q *http.Request) {
	v, e := r.replenishment.Detail(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) replenishmentSource(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.replenishment.ValidateSource(q.Context(), c, v)
	})
}
func (r *Router) replenishmentDestination(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.replenishment.ValidateDestination(q.Context(), c, v)
	})
}
func (r *Router) replenishmentItem(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.replenishment.ValidateItem(q.Context(), c, v)
	})
}
func (r *Router) replenishmentConfirm(w http.ResponseWriter, q *http.Request) {
	r.movementConfirm(w, q, func(c movementapp.Command, v int64) (movementdomain.Task, error) {
		return r.replenishment.Confirm(q.Context(), c, v)
	})
}
