package httpadapter

import (
	"net/http"

	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/go-chi/chi/v5"
)

func unavailableTraceError() error {
	return &platform.DomainError{Code: "UPSTREAM_OPERATION_NOT_IMPLEMENTED", SafeMessage: "Finished product trace is not available"}
}

func (r *Router) finishedProductTrace(w http.ResponseWriter, req *http.Request) {
	if r.finishedProductTraceOps == nil {
		r.movementResponse(w, req, nil, unavailableTraceError())
		return
	}
	data, err := r.finishedProductTraceOps.Trace(req.Context(), chi.URLParam(req, "barcode"), r.actor(req))
	if err != nil {
		r.movementResponse(w, req, nil, err)
		return
	}
	writeData(w, http.StatusOK, data, correlation(req.Context()), r.now())
}
