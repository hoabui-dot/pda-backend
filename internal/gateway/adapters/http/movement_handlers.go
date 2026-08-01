package httpadapter

import (
	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	movementdomain "github.com/company/pda-backend/internal/execution/movement/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"strings"
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
		Value    string `json:"value"`
		Location string `json:"location"`
		Barcode  string `json:"barcode"`
	}
	if e := decode(q, &x); e != nil {
		return "", e
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
		Quantity int64 `json:"quantity"`
	}
	if e := decode(q, &x); e != nil {
		return 0, e
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
	writeData(w, http.StatusOK, v, correlation(q.Context()), r.now())
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
	v, e := r.movements.Putaway.List(q.Context(), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) putawayDetail(w http.ResponseWriter, q *http.Request) {
	v, e := r.movements.Putaway.Detail(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) putawaySuggestions(w http.ResponseWriter, q *http.Request) {
	v, e := r.movements.Putaway.Suggestions(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) putawaySource(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.movements.Putaway.ValidateSource(q.Context(), c, v)
	})
}
func (r *Router) putawayDestination(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.movements.Putaway.ValidateDestination(q.Context(), c, v)
	})
}
func (r *Router) putawayConfirm(w http.ResponseWriter, q *http.Request) {
	r.movementConfirm(w, q, func(c movementapp.Command, v int64) (movementdomain.Task, error) {
		return r.movements.Putaway.Confirm(q.Context(), c, v)
	})
}
func (r *Router) pickingList(w http.ResponseWriter, q *http.Request) {
	v, e := r.movements.Picking.List(q.Context(), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) pickingDetail(w http.ResponseWriter, q *http.Request) {
	v, e := r.movements.Picking.Detail(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) pickingLocation(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.movements.Picking.ValidateLocation(q.Context(), c, v)
	})
}
func (r *Router) pickingBarcode(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.movements.Picking.ResolveBarcode(q.Context(), c, v)
	})
}
func (r *Router) pickingConfirm(w http.ResponseWriter, q *http.Request) {
	r.movementConfirm(w, q, func(c movementapp.Command, v int64) (movementdomain.Task, error) {
		return r.movements.Picking.Confirm(q.Context(), c, v)
	})
}
func (r *Router) pickingComplete(w http.ResponseWriter, q *http.Request) {
	c, e := r.movementCommand(q)
	if e != nil {
		r.movementResponse(w, q, nil, e)
		return
	}
	v, e := r.movements.Picking.Complete(q.Context(), c)
	r.movementResponse(w, q, v, e)
}
func (r *Router) replenishmentList(w http.ResponseWriter, q *http.Request) {
	v, e := r.movements.Replenishment.List(q.Context(), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) replenishmentDetail(w http.ResponseWriter, q *http.Request) {
	v, e := r.movements.Replenishment.Detail(q.Context(), chi.URLParam(q, "taskId"), r.actor(q))
	r.movementResponse(w, q, v, e)
}
func (r *Router) replenishmentSource(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.movements.Replenishment.ValidateSource(q.Context(), c, v)
	})
}
func (r *Router) replenishmentDestination(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.movements.Replenishment.ValidateDestination(q.Context(), c, v)
	})
}
func (r *Router) replenishmentItem(w http.ResponseWriter, q *http.Request) {
	r.movementValidation(w, q, func(c movementapp.Command, v string) (movementdomain.Task, error) {
		return r.movements.Replenishment.ValidateItem(q.Context(), c, v)
	})
}
func (r *Router) replenishmentConfirm(w http.ResponseWriter, q *http.Request) {
	r.movementConfirm(w, q, func(c movementapp.Command, v int64) (movementdomain.Task, error) {
		return r.movements.Replenishment.Confirm(q.Context(), c, v)
	})
}
