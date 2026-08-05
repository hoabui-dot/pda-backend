package wmstask

// Device-facing HTTP surface for a WMS-dispatched warehouse task.
//
// The route shape mirrors the existing picking/putaway/receiving modules so the
// PDA app treats a WMS task like any other work: list, detail, start, scan,
// completion, failure.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Routes returns the sub-router to mount under the authenticated API group.
func (s *Service) Routes(actorFrom func(*http.Request) Actor) chi.Router {
	r := chi.NewRouter()
	r.Get("/", s.handleList(actorFrom))
	r.Get("/{taskId}", s.handleDetail)
	r.Post("/{taskId}/start", s.handleStart(actorFrom))
	r.Post("/{taskId}/scans", s.handleScan(actorFrom))
	r.Post("/{taskId}/completion", s.handleComplete(actorFrom))
	r.Post("/{taskId}/failure", s.handleFail(actorFrom))
	return r
}

func (s *Service) handleList(actorFrom func(*http.Request) Actor) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		actor := actorFrom(req)
		warehouse := actor.WarehouseID
		if supplied := req.URL.Query().Get("warehouseId"); supplied != "" {
			warehouse = supplied
		}
		tasks, err := s.List(req.Context(), warehouse, req.URL.Query().Get("state"))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": tasks})
	}
}

func (s *Service) handleDetail(w http.ResponseWriter, req *http.Request) {
	task, err := s.Get(req.Context(), chi.URLParam(req, "taskId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": task})
}

type commandBody struct {
	CommandID       string  `json:"commandId"`
	ExpectedVersion int64   `json:"expectedVersion"`
	ScanType        string  `json:"scanType"`
	ScanValue       string  `json:"scanValue"`
	ConfirmedQty    float64 `json:"confirmedQuantity"`
	ReasonCode      string  `json:"reasonCode"`
}

// decode reads the command body and resolves the idempotency key. The device
// may supply it either in the body or via the Idempotency-Key header, matching
// the convention already used by the other PDA modules.
func decode(req *http.Request) (commandBody, uuid.UUID, error) {
	var body commandBody
	if req.Body != nil {
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil && err.Error() != "EOF" {
			return body, uuid.Nil, errors.New("INVALID_REQUEST_BODY")
		}
	}
	raw := strings.TrimSpace(body.CommandID)
	if raw == "" {
		raw = strings.TrimSpace(req.Header.Get("Idempotency-Key"))
	}
	if raw == "" {
		return body, uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return body, uuid.Nil, errors.New("INVALID_COMMAND_ID")
	}
	// If-Match carries the expected task version when the body omits it.
	if body.ExpectedVersion == 0 {
		if match := strings.Trim(req.Header.Get("If-Match"), `"`); match != "" {
			if version, convErr := strconv.ParseInt(match, 10, 64); convErr == nil {
				body.ExpectedVersion = version
			}
		}
	}
	return body, id, nil
}

func (s *Service) handleStart(actorFrom func(*http.Request) Actor) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, commandID, err := decode(req)
		if err != nil {
			writeError(w, err)
			return
		}
		task, err := s.Start(req.Context(), chi.URLParam(req, "taskId"), body.ExpectedVersion, commandID, actorFrom(req))
		respond(w, task, err)
	}
}

func (s *Service) handleScan(actorFrom func(*http.Request) Actor) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, commandID, err := decode(req)
		if err != nil {
			writeError(w, err)
			return
		}
		task, err := s.Scan(req.Context(), chi.URLParam(req, "taskId"), body.ScanType, body.ScanValue, body.ExpectedVersion, commandID, actorFrom(req))
		respond(w, task, err)
	}
}

func (s *Service) handleComplete(actorFrom func(*http.Request) Actor) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, commandID, err := decode(req)
		if err != nil {
			writeError(w, err)
			return
		}
		task, err := s.Complete(req.Context(), chi.URLParam(req, "taskId"), body.ConfirmedQty, body.ExpectedVersion, commandID, actorFrom(req))
		respond(w, task, err)
	}
}

func (s *Service) handleFail(actorFrom func(*http.Request) Actor) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, commandID, err := decode(req)
		if err != nil {
			writeError(w, err)
			return
		}
		task, err := s.Fail(req.Context(), chi.URLParam(req, "taskId"), body.ReasonCode, body.ExpectedVersion, commandID, actorFrom(req))
		respond(w, task, err)
	}
}

func respond(w http.ResponseWriter, task Task, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": task})
}

// statusFor maps a domain error onto the HTTP status the PDA app expects. A
// version conflict is 409 so the device knows to refresh rather than retry.
func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrTaskNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrVersionConflict), errors.Is(err, ErrCommandConflict), errors.Is(err, ErrAlreadyFinished):
		return http.StatusConflict
	case errors.Is(err, ErrOperatorRequired):
		return http.StatusForbidden
	default:
		return http.StatusUnprocessableEntity
	}
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, statusFor(err), map[string]any{"error": map[string]any{"code": err.Error()}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
