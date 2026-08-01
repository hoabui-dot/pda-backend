package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	executionapp "github.com/company/pda-backend/internal/execution/application"
	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	executionports "github.com/company/pda-backend/internal/execution/ports"
	receivingapp "github.com/company/pda-backend/internal/execution/receiving/application"
	receivingports "github.com/company/pda-backend/internal/execution/receiving/ports"
	identityapp "github.com/company/pda-backend/internal/identity/application"
	identity "github.com/company/pda-backend/internal/identity/domain"
	inventoryapp "github.com/company/pda-backend/internal/inventory/application"
	platform "github.com/company/pda-backend/internal/platform/domain"
	shippingapp "github.com/company/pda-backend/internal/shipping/application"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Settings struct {
	RequestTimeout          time.Duration
	AuthRateLimit           int
	RateWindow              time.Duration
	CircuitFailureThreshold int
}

func (s Settings) Validate() error {
	if s.RequestTimeout <= 0 || s.AuthRateLimit < 1 || s.RateWindow <= 0 || s.CircuitFailureThreshold < 1 {
		return fmt.Errorf("gateway timeout, rate limit, rate window, and circuit threshold must be positive")
	}
	return nil
}

type Router struct {
	identity  *identityapp.Service
	tasks     *executionapp.TaskService
	receiving *receivingapp.Service
	movements *movementapp.Services
	inventory *inventoryapp.Service
	shipping  *shippingapp.Service
	settings  Settings
	limiter   *rateLimiter
	logger    *slog.Logger
	now       func() time.Time
	breaker   *gatewayBreaker
}

func New(identityService *identityapp.Service, taskService *executionapp.TaskService, receivingService *receivingapp.Service, movementServices *movementapp.Services, inventoryService *inventoryapp.Service, shippingService *shippingapp.Service, settings Settings, logger *slog.Logger, now func() time.Time) (http.Handler, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	router := &Router{identity: identityService, tasks: taskService, receiving: receivingService, movements: movementServices, inventory: inventoryService, shipping: shippingService, settings: settings, limiter: newRateLimiter(settings.AuthRateLimit, settings.RateWindow, now), logger: logger, now: now, breaker: newGatewayBreaker(settings.CircuitFailureThreshold, settings.RateWindow, now)}
	r := chi.NewRouter()
	r.Use(router.correlation, router.logging, router.circuitBreak)
	r.Get("/healthz", router.operational)
	r.Get("/livez", router.operational)
	r.Get("/readyz", router.operational)
	r.Route("/api/pda/v1", func(api chi.Router) {
		api.With(router.rateLimit).Post("/auth/login", router.login)
		api.With(router.rateLimit).Post("/auth/refresh", router.refresh)
		api.Group(func(protected chi.Router) {
			protected.Use(router.authenticate)
			protected.Post("/auth/logout", router.logout)
			protected.Get("/me", router.me)
			protected.Get("/me/warehouses", router.warehouses)
			protected.Post("/devices/registrations", router.registerDevice)
			protected.With(router.deviceWarehouseContext).Get("/bootstrap", router.bootstrap)
			protected.With(router.deviceWarehouseContext).Get("/dashboard", router.dashboard)
			protected.With(router.deviceWarehouseContext).Get("/tasks/summary", router.taskSummary)
			protected.With(router.deviceWarehouseContext).Get("/tasks", router.tasksList)
			protected.With(router.deviceWarehouseContext).Post("/tasks/{taskId}/claim", router.claimTask)
			protected.With(router.deviceWarehouseContext).Post("/tasks/{taskId}/release", router.releaseTask)
			protected.With(router.deviceWarehouseContext).Get("/receiving/tasks", router.receivingList)
			protected.With(router.deviceWarehouseContext).Get("/receiving/tasks/{taskId}", router.receivingDetail)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/start", router.receivingStart)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/barcode-resolutions", router.receivingBarcode)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/receipts", router.receivingConfirm)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/completion", router.receivingComplete)
			protected.With(router.deviceWarehouseContext).Get("/receiving/commands/{commandId}", router.receivingCommandStatus)
			protected.With(router.deviceWarehouseContext).Get("/putaway/tasks", router.putawayList)
			protected.With(router.deviceWarehouseContext).Get("/putaway/tasks/{taskId}", router.putawayDetail)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/source-validations", router.putawaySource)
			protected.With(router.deviceWarehouseContext).Get("/putaway/tasks/{taskId}/destination-suggestions", router.putawaySuggestions)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/destination-validations", router.putawayDestination)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/confirmation", router.putawayConfirm)
			protected.With(router.deviceWarehouseContext).Get("/picking/tasks", router.pickingList)
			protected.With(router.deviceWarehouseContext).Get("/picking/tasks/{taskId}", router.pickingDetail)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/location-validations", router.pickingLocation)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/barcode-resolutions", router.pickingBarcode)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/picks", router.pickingConfirm)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/completion", router.pickingComplete)
			protected.With(router.deviceWarehouseContext).Get("/replenishment/tasks", router.replenishmentList)
			protected.With(router.deviceWarehouseContext).Get("/replenishment/tasks/{taskId}", router.replenishmentDetail)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/tasks/{taskId}/source-validations", router.replenishmentSource)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/tasks/{taskId}/destination-validations", router.replenishmentDestination)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/tasks/{taskId}/item-validations", router.replenishmentItem)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/tasks/{taskId}/confirmation", router.replenishmentConfirm)
			protected.With(router.deviceWarehouseContext).Get("/inventory/search", router.inventorySearch)
			protected.With(router.deviceWarehouseContext).Get("/inventory/balances", router.inventoryBalances)
			protected.With(router.deviceWarehouseContext).Get("/inventory/movements", router.inventoryMovements)
			protected.With(router.deviceWarehouseContext).Post("/inventory/transfers/source-validations", router.transferSource)
			protected.With(router.deviceWarehouseContext).Post("/inventory/transfers/destination-validations", router.transferDestination)
			protected.With(router.deviceWarehouseContext).Post("/inventory/transfers/item-resolutions", router.transferItem)
			protected.With(router.deviceWarehouseContext).Post("/inventory/transfers", router.transferConfirm)
			protected.With(router.deviceWarehouseContext).Get("/cycle-count/tasks", router.countList)
			protected.With(router.deviceWarehouseContext).Get("/cycle-count/tasks/{taskId}", router.countDetail)
			protected.With(router.deviceWarehouseContext).Post("/cycle-count/tasks/{taskId}/counts", router.countSubmit)
			protected.With(router.deviceWarehouseContext).Post("/cycle-count/tasks/{taskId}/recounts", router.countRecount)
			protected.With(router.deviceWarehouseContext).Post("/cycle-count/tasks/{taskId}/completion", router.countComplete)
			protected.With(router.deviceWarehouseContext).Get("/shipments/{shipmentId}", router.shipmentSummary)
			protected.With(router.deviceWarehouseContext).Get("/shipments/{shipmentId}/readiness", router.shipmentReadiness)
			protected.With(router.deviceWarehouseContext).Post("/shipments/{shipmentId}/confirmation", router.shipmentConfirm)
			protected.With(router.deviceWarehouseContext).Get("/shipments/{shipmentId}/commands/{commandId}", router.shipmentCommandStatus)
		})
	})
	return withTimeout(r, settings.RequestTimeout), nil
}

func withTimeout(handler http.Handler, timeout time.Duration) http.Handler {
	return http.TimeoutHandler(handler, timeout, `{"code":"GATEWAY_TIMEOUT","message":"Request timed out","retryable":true}`)
}

type contextKey string

const operatorKey contextKey = "operator"

func (r *Router) correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		id := req.Header.Get("X-Correlation-Id")
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		w.Header().Set("X-Correlation-Id", id)
		next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), contextKey("correlation"), id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (r *Router) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, req)
		r.logger.Info("request", "method", req.Method, "route", req.URL.Path, "status", wrapped.status, "correlationId", correlation(req.Context()))
	})
}

func (r *Router) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		header := req.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, identityapp.ErrUnauthorized, correlation(req.Context()))
			return
		}
		operator, err := r.identity.Authenticate(req.Context(), strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			writeError(w, err, correlation(req.Context()))
			return
		}
		next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), operatorKey, operator)))
	})
}
func (r *Router) deviceWarehouseContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		operator := req.Context().Value(operatorKey).(identity.Operator)
		deviceID, warehouseID := req.Header.Get("X-Device-Id"), req.Header.Get("X-Warehouse-Id")
		if deviceID == "" {
			writeError(w, identityapp.ErrDeviceNotRegistered, correlation(req.Context()))
			return
		}
		if warehouseID == "" {
			writeError(w, identityapp.ErrWarehouseDenied, correlation(req.Context()))
			return
		}
		if err := r.identity.ValidateContext(req.Context(), operator, deviceID, warehouseID); err != nil {
			writeError(w, err, correlation(req.Context()))
			return
		}
		next.ServeHTTP(w, req)
	})
}
func (r *Router) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		host, _, _ := net.SplitHostPort(req.RemoteAddr)
		if host == "" {
			host = req.RemoteAddr
		}
		if !r.limiter.Allow(host) {
			writeError(w, &platform.DomainError{Code: "RATE_LIMITED", SafeMessage: "Too many requests", Retryable: true}, correlation(req.Context()))
			return
		}
		next.ServeHTTP(w, req)
	})
}

func (r *Router) circuitBreak(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !r.breaker.Allow() {
			writeError(w, &platform.DomainError{Code: "GATEWAY_CIRCUIT_OPEN", SafeMessage: "Gateway dependency circuit is open", Retryable: true}, correlation(req.Context()))
			return
		}
		recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, req)
		r.breaker.Record(recorder.status >= http.StatusInternalServerError)
	})
}

func (r *Router) login(w http.ResponseWriter, req *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(req, &input); err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	token, operator, err := r.identity.Login(req.Context(), input.Username, input.Password, correlation(req.Context()))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, map[string]any{"accessToken": token, "tokenType": "Bearer", "expiresIn": 3600, "operator": operator}, correlation(req.Context()), r.now())
}
func (r *Router) refresh(w http.ResponseWriter, req *http.Request) {
	header := req.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeError(w, identityapp.ErrUnauthorized, correlation(req.Context()))
		return
	}
	token, err := r.identity.Refresh(req.Context(), strings.TrimPrefix(header, "Bearer "))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, map[string]any{"accessToken": token, "tokenType": "Bearer", "expiresIn": 3600}, correlation(req.Context()), r.now())
}
func (r *Router) logout(w http.ResponseWriter, req *http.Request) {
	if err := r.identity.Logout(strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")); err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (r *Router) operational(w http.ResponseWriter, req *http.Request) {
	writeData(w, http.StatusOK, map[string]string{"status": "UP", "service": "pda-api-gateway"}, correlation(req.Context()), r.now())
}
func (r *Router) me(w http.ResponseWriter, req *http.Request) {
	writeData(w, http.StatusOK, req.Context().Value(operatorKey), correlation(req.Context()), r.now())
}
func (r *Router) warehouses(w http.ResponseWriter, req *http.Request) {
	values, err := r.identity.Warehouses(req.Context(), req.Context().Value(operatorKey).(identity.Operator))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, values, correlation(req.Context()), r.now())
}
func (r *Router) registerDevice(w http.ResponseWriter, req *http.Request) {
	var input struct {
		DeviceID    string `json:"deviceId"`
		WarehouseID string `json:"warehouseId"`
	}
	if err := decode(req, &input); err != nil || input.DeviceID == "" || input.WarehouseID == "" {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Device and warehouse are required"}, correlation(req.Context()))
		return
	}
	value, err := r.identity.RegisterDevice(req.Context(), req.Context().Value(operatorKey).(identity.Operator), input.DeviceID, input.WarehouseID, correlation(req.Context()))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusCreated, value, correlation(req.Context()), r.now())
}
func (r *Router) bootstrap(w http.ResponseWriter, req *http.Request) {
	operator := req.Context().Value(operatorKey).(identity.Operator)
	warehouses, err := r.identity.Warehouses(req.Context(), operator)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, map[string]any{"operator": operator, "warehouses": warehouses, "deviceId": req.Header.Get("X-Device-Id"), "warehouseId": req.Header.Get("X-Warehouse-Id"), "capabilities": []string{}}, correlation(req.Context()), r.now())
}

func (r *Router) actor(req *http.Request) platform.ActorContext {
	operator := req.Context().Value(operatorKey).(identity.Operator)
	return platform.ActorContext{OperatorID: operator.ID, DeviceID: req.Header.Get("X-Device-Id"), WarehouseID: req.Header.Get("X-Warehouse-Id"), CorrelationID: correlation(req.Context())}
}
func (r *Router) dashboard(w http.ResponseWriter, req *http.Request) {
	value, err := r.tasks.Dashboard(req.Context(), r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, value, correlation(req.Context()), r.now())
}
func (r *Router) taskSummary(w http.ResponseWriter, req *http.Request) {
	values, err := r.tasks.Summary(req.Context(), req.Header.Get("X-Warehouse-Id"), req.URL.Query().Get("status"), r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, values, correlation(req.Context()), r.now())
}
func (r *Router) tasksList(w http.ResponseWriter, req *http.Request) {
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	filter := executionports.TaskFilter{WarehouseID: req.Header.Get("X-Warehouse-Id"), Category: req.URL.Query().Get("category"), Status: req.URL.Query().Get("status"), Query: req.URL.Query().Get("q"), Cursor: req.URL.Query().Get("cursor"), Limit: limit}
	page, err := r.tasks.List(req.Context(), filter, r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, page, correlation(req.Context()), r.now())
}
func (r *Router) taskCommand(req *http.Request) (executionapp.TaskCommand, error) {
	key := req.Header.Get("Idempotency-Key")
	commandID, err := uuid.Parse(key)
	if err != nil {
		return executionapp.TaskCommand{}, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "A UUID Idempotency-Key is required"}
	}
	match := strings.Trim(req.Header.Get("If-Match"), `"`)
	version, err := strconv.ParseInt(match, 10, 64)
	if err != nil {
		return executionapp.TaskCommand{}, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "A numeric If-Match version is required"}
	}
	return executionapp.TaskCommand{TaskID: chi.URLParam(req, "taskId"), IdempotencyKey: key, CommandID: commandID, BaseVersion: version, Actor: r.actor(req)}, nil
}
func (r *Router) claimTask(w http.ResponseWriter, req *http.Request) {
	command, err := r.taskCommand(req)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	task, err := r.tasks.Claim(req.Context(), command)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, task, correlation(req.Context()), r.now())
}
func (r *Router) releaseTask(w http.ResponseWriter, req *http.Request) {
	command, err := r.taskCommand(req)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	task, err := r.tasks.Release(req.Context(), command)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, task, correlation(req.Context()), r.now())
}

func (r *Router) receivingList(w http.ResponseWriter, req *http.Request) {
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	page, err := r.receiving.List(req.Context(), receivingports.Filter{WarehouseID: req.Header.Get("X-Warehouse-Id"), Status: req.URL.Query().Get("status"), Cursor: req.URL.Query().Get("cursor"), Limit: limit}, r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, page, correlation(req.Context()), r.now())
}
func (r *Router) receivingDetail(w http.ResponseWriter, req *http.Request) {
	task, err := r.receiving.Detail(req.Context(), chi.URLParam(req, "taskId"), r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, task, correlation(req.Context()), r.now())
}
func (r *Router) receivingBaseCommand(req *http.Request) (receivingapp.Command, error) {
	key := req.Header.Get("Idempotency-Key")
	id, err := uuid.Parse(key)
	if err != nil {
		return receivingapp.Command{}, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "A UUID Idempotency-Key is required"}
	}
	version, err := strconv.ParseInt(strings.Trim(req.Header.Get("If-Match"), `"`), 10, 64)
	if err != nil {
		return receivingapp.Command{}, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "A numeric If-Match version is required"}
	}
	return receivingapp.Command{CommandID: id, IdempotencyKey: key, TaskID: chi.URLParam(req, "taskId"), BaseVersion: version, Actor: r.actor(req)}, nil
}
func (r *Router) receivingStart(w http.ResponseWriter, req *http.Request) {
	command, err := r.receivingBaseCommand(req)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	task, err := r.receiving.Start(req.Context(), command)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, task, correlation(req.Context()), r.now())
}
func (r *Router) receivingBarcode(w http.ResponseWriter, req *http.Request) {
	var input struct {
		Barcode string `json:"barcode"`
	}
	if err := decode(req, &input); err != nil || input.Barcode == "" {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Barcode is required"}, correlation(req.Context()))
		return
	}
	line, err := r.receiving.ResolveBarcode(req.Context(), chi.URLParam(req, "taskId"), input.Barcode, r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, line, correlation(req.Context()), r.now())
}
func (r *Router) receivingConfirm(w http.ResponseWriter, req *http.Request) {
	base, err := r.receivingBaseCommand(req)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	var input struct {
		CommandID   uuid.UUID `json:"commandId"`
		LineID      string    `json:"lineId"`
		Barcode     string    `json:"barcode"`
		Quantity    int64     `json:"quantity"`
		Remark      *string   `json:"remark"`
		BaseVersion int64     `json:"baseVersion"`
	}
	if err := decode(req, &input); err != nil || input.CommandID != base.CommandID || input.BaseVersion != base.BaseVersion {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Command metadata does not match headers"}, correlation(req.Context()))
		return
	}
	task, err := r.receiving.Confirm(req.Context(), receivingapp.ConfirmCommand{Command: base, LineID: input.LineID, Barcode: input.Barcode, Quantity: input.Quantity, Remark: input.Remark})
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, task, correlation(req.Context()), r.now())
}
func (r *Router) receivingComplete(w http.ResponseWriter, req *http.Request) {
	command, err := r.receivingBaseCommand(req)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	task, err := r.receiving.Complete(req.Context(), command)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, task, correlation(req.Context()), r.now())
}
func (r *Router) receivingCommandStatus(w http.ResponseWriter, req *http.Request) {
	id, err := uuid.Parse(chi.URLParam(req, "commandId"))
	if err != nil {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Invalid command ID"}, correlation(req.Context()))
		return
	}
	status, err := r.receiving.CommandStatus(req.Context(), id, r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, status, correlation(req.Context()), r.now())
}

func decode(req *http.Request, target any) error {
	defer req.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(req.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Invalid request"}
	}
	return nil
}
func correlation(ctx context.Context) string {
	value, _ := ctx.Value(contextKey("correlation")).(string)
	return value
}
func writeData(w http.ResponseWriter, status int, data any, correlationID string, now time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": map[string]any{"serverTime": now.UTC(), "correlationId": correlationID}, "errors": []any{}})
}
func writeError(w http.ResponseWriter, err error, correlationID string) {
	var domainError *platform.DomainError
	if !errors.As(err, &domainError) {
		domainError = &platform.DomainError{Code: "INTERNAL_ERROR", SafeMessage: "Internal error"}
	}
	status := http.StatusBadRequest
	switch domainError.Code {
	case "AUTH_INVALID_CREDENTIALS", "AUTH_SESSION_EXPIRED":
		status = http.StatusUnauthorized
	case "WAREHOUSE_ACCESS_DENIED":
		status = http.StatusForbidden
	case "RATE_LIMITED":
		status = http.StatusTooManyRequests
	case "GATEWAY_CIRCUIT_OPEN":
		status = http.StatusServiceUnavailable
	case "TASK_LOCKED", "TASK_VERSION_CONFLICT", "SHIPMENT_VERSION_CONFLICT":
		status = http.StatusConflict
	case "TASK_NOT_FOUND", "INVENTORY_NOT_FOUND", "SHIPMENT_NOT_FOUND":
		status = http.StatusNotFound
	case "BARCODE_UNKNOWN", "BARCODE_WRONG_CONTEXT", "QUANTITY_EXCEEDS_ALLOWED", "REMARK_REQUIRED", "RECEIVING_TASK_INCOMPLETE", "TASK_NOT_ASSIGNED", "TASK_ALREADY_COMPLETED", "SOURCE_LOCATION_INVALID", "DESTINATION_LOCATION_INVALID", "ITEM_INVALID", "VALIDATION_SEQUENCE_INVALID", "INSUFFICIENT_STOCK", "LOCATION_CAPACITY_EXCEEDED", "TASK_INCOMPLETE", "IDEMPOTENCY_KEY_REUSED", "SOURCE_EQUALS_DESTINATION", "SHIPMENT_NOT_READY", "PACKAGE_INCOMPLETE", "CARRIER_INVALID", "TRACKING_INVALID", "SHIPMENT_ALREADY_CONFIRMED":
		status = http.StatusUnprocessableEntity
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": domainError.Code, "message": domainError.SafeMessage, "details": domainError.Details, "correlationId": correlationID, "retryable": domainError.Retryable})
}

type rateEntry struct {
	count int
	reset time.Time
}
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	now     func() time.Time
	entries map[string]rateEntry
}

type gatewayBreaker struct {
	mu                  sync.Mutex
	failures, threshold int
	reset               time.Duration
	openUntil           time.Time
	now                 func() time.Time
}

func newGatewayBreaker(threshold int, reset time.Duration, now func() time.Time) *gatewayBreaker {
	return &gatewayBreaker{threshold: threshold, reset: reset, now: now}
}
func (b *gatewayBreaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openUntil.IsZero() {
		return true
	}
	if !b.now().Before(b.openUntil) {
		b.openUntil = time.Time{}
		b.failures = 0
		return true
	}
	return false
}
func (b *gatewayBreaker) Record(failed bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !failed {
		b.failures = 0
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.openUntil = b.now().Add(b.reset)
	}
}

func newRateLimiter(limit int, window time.Duration, now func() time.Time) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, now: now, entries: map[string]rateEntry{}}
}
func (l *rateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	current := l.now()
	entry := l.entries[key]
	if !current.Before(entry.reset) {
		entry = rateEntry{reset: current.Add(l.window)}
	}
	entry.count++
	l.entries[key] = entry
	return entry.count <= l.limit
}
