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
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	executionapp "github.com/company/pda-backend/internal/execution/application"
	executiondomain "github.com/company/pda-backend/internal/execution/domain"
	executionports "github.com/company/pda-backend/internal/execution/ports"
	receivingapp "github.com/company/pda-backend/internal/execution/receiving/application"
	receivingdomain "github.com/company/pda-backend/internal/execution/receiving/domain"
	receivingports "github.com/company/pda-backend/internal/execution/receiving/ports"
	gatewayports "github.com/company/pda-backend/internal/gateway/ports"
	identityapp "github.com/company/pda-backend/internal/identity/application"
	identity "github.com/company/pda-backend/internal/identity/domain"
	platform "github.com/company/pda-backend/internal/platform/domain"
	"github.com/company/pda-backend/internal/wmstask"
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
	identity         *identityapp.Service
	tasks            gatewayports.TaskOperations
	receiving        gatewayports.ReceivingOperations
	putaway          gatewayports.PutawayOperations
	picking          gatewayports.PickingOperations
	replenishment    gatewayports.ReplenishmentOperations
	movementCommands gatewayports.MovementCommandOperations
	inventory        gatewayports.InventoryOperations
	shipping         gatewayports.ShippingOperations
	wmsTasks         *wmstask.Service
	settings         Settings
	limiter          *rateLimiter
	logger           *slog.Logger
	now              func() time.Time
	breaker          *gatewayBreaker
}

func New(identityService *identityapp.Service, taskService gatewayports.TaskOperations, receivingService gatewayports.ReceivingOperations, putaway gatewayports.PutawayOperations, picking gatewayports.PickingOperations, replenishment gatewayports.ReplenishmentOperations, movementCommands gatewayports.MovementCommandOperations, inventoryService gatewayports.InventoryOperations, shippingService gatewayports.ShippingOperations, wmsTaskService *wmstask.Service, settings Settings, logger *slog.Logger, now func() time.Time) (http.Handler, error) {
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	router := &Router{identity: identityService, tasks: taskService, receiving: receivingService, putaway: putaway, picking: picking, replenishment: replenishment, movementCommands: movementCommands, inventory: inventoryService, shipping: shippingService, wmsTasks: wmsTaskService, settings: settings, limiter: newRateLimiter(settings.AuthRateLimit, settings.RateWindow, now), logger: logger, now: now, breaker: newGatewayBreaker(settings.CircuitFailureThreshold, settings.RateWindow, now)}
	r := chi.NewRouter()
	r.Use(router.correlation, router.locale, router.logging, router.circuitBreak)
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
			protected.With(router.deviceWarehouseContext).Get("/events", router.taskEvents)
			protected.With(router.deviceWarehouseContext).Get("/tasks/{taskId}", router.taskDetail)
			protected.With(router.deviceWarehouseContext).Post("/tasks/{taskId}/claim", router.claimTask)
			protected.With(router.deviceWarehouseContext).Post("/tasks/{taskId}/release", router.releaseTask)
			// WMS task routes are local projection wiring in mock mode. HTTP
			// mode leaves them absent until their remote contract is mapped.
			if router.wmsTasks != nil {
				protected.With(router.deviceWarehouseContext).Mount("/wms-tasks", router.wmsTasks.Routes(func(req *http.Request) wmstask.Actor {
					actor := router.actor(req)
					return wmstask.Actor{OperatorID: actor.OperatorID, DeviceID: actor.DeviceID, WarehouseID: actor.WarehouseID, CorrelationID: actor.CorrelationID}
				}))
			}
			protected.With(router.deviceWarehouseContext).Get("/receiving/tasks", router.receivingList)
			protected.With(router.deviceWarehouseContext).Get("/receiving/tasks/{taskId}", router.receivingDetail)
			protected.With(router.deviceWarehouseContext).Get("/receiving", router.receivingList)
			protected.With(router.deviceWarehouseContext).Get("/receiving/{taskId}", router.receivingDetail)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/claim", router.receivingClaim)
			protected.With(router.deviceWarehouseContext).Post("/receiving/{taskId}/claim", router.receivingClaim)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/start", router.receivingStart)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/barcode-resolutions", router.receivingBarcode)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/receipts", router.receivingConfirm)
			protected.With(router.deviceWarehouseContext).Post("/receiving/{taskId}/resolve-barcode", router.receivingBarcode)
			protected.With(router.deviceWarehouseContext).Post("/receiving/{taskId}/confirm", router.receivingConfirm)
			protected.With(router.deviceWarehouseContext).Post("/receiving/tasks/{taskId}/completion", router.receivingComplete)
			protected.With(router.deviceWarehouseContext).Get("/receiving/commands/{commandId}", router.receivingCommandStatus)
			protected.With(router.deviceWarehouseContext).Get("/commands/{commandId}", router.commandStatus)
			protected.With(router.deviceWarehouseContext).Get("/putaway/tasks", router.putawayList)
			protected.With(router.deviceWarehouseContext).Get("/putaway/tasks/{taskId}", router.putawayDetail)
			protected.With(router.deviceWarehouseContext).Get("/putaway", router.putawayList)
			protected.With(router.deviceWarehouseContext).Get("/putaway/{taskId}", router.putawayDetail)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/claim", router.putawayClaim)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/start", router.putawayStart)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/source-validations", router.putawaySource)
			protected.With(router.deviceWarehouseContext).Post("/putaway/{taskId}/validate-source", router.putawaySource)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/item-validations", router.putawayItem)
			protected.With(router.deviceWarehouseContext).Post("/putaway/{taskId}/validate-item", router.putawayItem)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/lot-validations", router.putawayLot)
			protected.With(router.deviceWarehouseContext).Post("/putaway/{taskId}/validate-lot", router.putawayLot)
			protected.With(router.deviceWarehouseContext).Get("/putaway/tasks/{taskId}/destination-suggestions", router.putawaySuggestions)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/destination-validations", router.putawayDestination)
			protected.With(router.deviceWarehouseContext).Post("/putaway/{taskId}/validate-destination", router.putawayDestination)
			protected.With(router.deviceWarehouseContext).Post("/putaway/tasks/{taskId}/confirmation", router.putawayConfirm)
			protected.With(router.deviceWarehouseContext).Post("/putaway/{taskId}/confirm", router.putawayConfirm)
			protected.With(router.deviceWarehouseContext).Get("/picking/tasks", router.pickingList)
			protected.With(router.deviceWarehouseContext).Get("/picking/tasks/{taskId}", router.pickingDetail)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/claim", router.pickingClaim)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/start", router.pickingStart)
			protected.With(router.deviceWarehouseContext).Get("/picking", router.pickingList)
			protected.With(router.deviceWarehouseContext).Get("/picking/{taskId}", router.pickingDetail)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/allocation", router.pickingAllocate)
			protected.With(router.deviceWarehouseContext).Post("/picking/{taskId}/allocate", router.pickingAllocate)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/location-validations", router.pickingLocation)
			protected.With(router.deviceWarehouseContext).Post("/picking/{taskId}/validate-location", router.pickingLocation)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/barcode-resolutions", router.pickingBarcode)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/lot-validations", router.pickingLot)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/destination-validations", router.pickingDestination)
			protected.With(router.deviceWarehouseContext).Post("/picking/{taskId}/resolve-item", router.pickingBarcode)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/picks", router.pickingConfirm)
			protected.With(router.deviceWarehouseContext).Post("/picking/{taskId}/pick", router.pickingConfirm)
			protected.With(router.deviceWarehouseContext).Post("/picking/tasks/{taskId}/completion", router.pickingComplete)
			protected.With(router.deviceWarehouseContext).Post("/picking/{taskId}/complete", router.pickingComplete)
			protected.With(router.deviceWarehouseContext).Get("/replenishment/tasks", router.replenishmentList)
			protected.With(router.deviceWarehouseContext).Get("/replenishment/tasks/{taskId}", router.replenishmentDetail)
			protected.With(router.deviceWarehouseContext).Get("/replenishment", router.replenishmentList)
			protected.With(router.deviceWarehouseContext).Get("/replenishment/{taskId}", router.replenishmentDetail)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/tasks/{taskId}/source-validations", router.replenishmentSource)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/{taskId}/validate-source", router.replenishmentSource)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/tasks/{taskId}/destination-validations", router.replenishmentDestination)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/{taskId}/validate-destination", router.replenishmentDestination)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/tasks/{taskId}/item-validations", router.replenishmentItem)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/{taskId}/validate-item", router.replenishmentItem)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/tasks/{taskId}/confirmation", router.replenishmentConfirm)
			protected.With(router.deviceWarehouseContext).Post("/replenishment/{taskId}/confirm", router.replenishmentConfirm)
			protected.With(router.deviceWarehouseContext).Get("/inventory/search", router.inventorySearch)
			protected.With(router.deviceWarehouseContext).Get("/inventory/items", router.inventorySearch)
			protected.With(router.deviceWarehouseContext).Get("/inventory/balances", router.inventoryBalances)
			protected.With(router.deviceWarehouseContext).Get("/inventory/movements", router.inventoryMovements)
			protected.With(router.deviceWarehouseContext).Post("/inventory/transfers/source-validations", router.transferSource)
			protected.With(router.deviceWarehouseContext).Post("/inventory/transfers/destination-validations", router.transferDestination)
			protected.With(router.deviceWarehouseContext).Post("/inventory/transfers/item-resolutions", router.transferItem)
			protected.With(router.deviceWarehouseContext).Post("/inventory/transfers", router.transferConfirm)
			protected.With(router.deviceWarehouseContext).Post("/transfers/validate", router.transferValidation)
			protected.With(router.deviceWarehouseContext).Post("/transfers/confirm", router.transferConfirm)
			protected.With(router.deviceWarehouseContext).Get("/transfers/commands/{commandId}", router.transferCommandStatus)
			protected.With(router.deviceWarehouseContext).Get("/cycle-count/tasks", router.countList)
			protected.With(router.deviceWarehouseContext).Get("/cycle-count/tasks/{taskId}", router.countDetail)
			protected.With(router.deviceWarehouseContext).Get("/counts", router.countList)
			protected.With(router.deviceWarehouseContext).Get("/counts/{taskId}", router.countDetail)
			protected.With(router.deviceWarehouseContext).Post("/counts/{taskId}/validate-location", router.countValidateLocation)
			protected.With(router.deviceWarehouseContext).Post("/counts/{taskId}/validate-item", router.countValidateItem)
			protected.With(router.deviceWarehouseContext).Post("/cycle-count/tasks/{taskId}/counts", router.countSubmit)
			protected.With(router.deviceWarehouseContext).Post("/counts/{taskId}/lines/{lineId}/submit", router.countSubmit)
			protected.With(router.deviceWarehouseContext).Post("/cycle-count/tasks/{taskId}/recounts", router.countRecount)
			protected.With(router.deviceWarehouseContext).Post("/counts/{taskId}/recount", router.countRecount)
			protected.With(router.deviceWarehouseContext).Post("/cycle-count/tasks/{taskId}/completion", router.countComplete)
			protected.With(router.deviceWarehouseContext).Post("/counts/{taskId}/complete", router.countComplete)
			protected.With(router.deviceWarehouseContext).Get("/counts/commands/{commandId}", router.countCommandStatus)
			protected.With(router.deviceWarehouseContext).Get("/shipments/{shipmentId}", router.shipmentSummary)
			protected.With(router.deviceWarehouseContext).Get("/shipments/{shipmentId}/readiness", router.shipmentReadiness)
			protected.With(router.deviceWarehouseContext).Post("/shipments/{shipmentId}/confirmation", router.shipmentConfirm)
			protected.With(router.deviceWarehouseContext).Post("/shipments/{shipmentId}/packages/{packageId}/verify", router.packageVerify)
			protected.With(router.deviceWarehouseContext).Post("/shipments/{shipmentId}/confirm", router.shipmentConfirm)
			protected.With(router.deviceWarehouseContext).Get("/shipments/{shipmentId}/commands/{commandId}", router.shipmentCommandStatus)
		})
	})
	return withTimeout(r, settings.RequestTimeout), nil
}

func withTimeout(handler http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/events") {
			handler.ServeHTTP(w, req)
			return
		}
		http.TimeoutHandler(handler, timeout, `{"data":null,"meta":{"serverTime":null,"correlationId":""},"errors":[{"code":"GATEWAY_TIMEOUT","message":"Request timed out","retryable":true}]}`).ServeHTTP(w, req)
	})
}

type contextKey string

const operatorKey contextKey = "operator"
const localeKey contextKey = "locale"

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

func (r *Router) locale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		locale := strings.TrimSpace(req.Header.Get("Accept-Language"))
		if locale == "" {
			locale = "en-US"
		} else {
			locale = strings.TrimSpace(strings.Split(locale, ",")[0])
		}
		if locale != "vi-VN" && locale != "en-US" {
			writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Unsupported language"}, correlation(req.Context()))
			return
		}
		w.Header().Set("Content-Language", locale)
		next.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), localeKey, locale)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status            int
	dependencyFailure bool
}

// Preserve streaming support for SSE while retaining request status logging.
func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	if w.Header().Get("X-Gateway-Dependency-Failure") == "1" {
		w.dependencyFailure = true
	}
	w.ResponseWriter.WriteHeader(status)
}
func (r *Router) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.logger.Info("pda_request_start",
			"method", req.Method,
			"route", req.URL.Path,
			"query", req.URL.RawQuery,
			"operatorId", req.Header.Get("X-Operator-Id"),
			"userId", req.Header.Get("X-User-ID"),
			"warehouseId", req.Header.Get("X-Warehouse-Id"),
			"deviceId", req.Header.Get("X-Device-Id"),
			"correlationId", correlation(req.Context()),
		)
		wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, req)
		r.logger.Info("pda_request_end", "method", req.Method, "route", req.URL.Path, "status", wrapped.status, "correlationId", correlation(req.Context()), "dependencyFailure", wrapped.dependencyFailure)
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
		if supplied := strings.TrimSpace(req.Header.Get("X-Operator-Id")); supplied != "" && supplied != operator.ID {
			writeError(w, &platform.DomainError{Code: "OPERATOR_CONTEXT_MISMATCH", SafeMessage: "Operator context does not match the authenticated session"}, correlation(req.Context()))
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
			retryAfter := int64((r.settings.RateWindow + time.Second - 1) / time.Second)
			w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
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
		// Only explicitly classified dependency failures affect the breaker.
		// A route-level 5xx such as an unmapped optional adapter must not take
		// authentication, task reads, and SSE down with it.
		r.breaker.Record(recorder.dependencyFailure)
	})
}

func (r *Router) login(w http.ResponseWriter, req *http.Request) {
	var input struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DeviceID    string `json:"deviceId"`
		DeviceModel string `json:"deviceModel"`
		AppVersion  string `json:"appVersion"`
		WarehouseID string `json:"warehouseId"`
		Locale      string `json:"locale"`
	}
	if err := decode(req, &input); err != nil || strings.TrimSpace(input.Username) == "" || input.Password == "" {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Username and password are required"}, correlation(req.Context()))
		return
	}
	if input.Locale != "" && input.Locale != "en-US" && input.Locale != "vi-VN" {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Unsupported language"}, correlation(req.Context()))
		return
	}
	session, operator, err := r.identity.LoginSessionContext(req.Context(), strings.TrimSpace(input.Username), input.Password, input.DeviceID, input.WarehouseID, correlation(req.Context()))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	data, err := r.sessionData(req.Context(), operator, session, input.DeviceID, input.WarehouseID)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, data, correlation(req.Context()), r.now())
}
func (r *Router) refresh(w http.ResponseWriter, req *http.Request) {
	var input struct {
		RefreshToken string `json:"refreshToken"`
		DeviceID     string `json:"deviceId"`
	}
	var session identityapp.Session
	var operator identity.Operator
	var err error
	if req.ContentLength != 0 {
		if decodeErr := decode(req, &input); decodeErr != nil {
			writeError(w, decodeErr, correlation(req.Context()))
			return
		}
	}
	if input.RefreshToken != "" {
		session, operator, err = r.identity.RefreshSessionContext(req.Context(), input.RefreshToken, input.DeviceID)
	} else {
		header := req.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, identityapp.ErrUnauthorized, correlation(req.Context()))
			return
		}
		var token string
		token, err = r.identity.Refresh(req.Context(), strings.TrimPrefix(header, "Bearer "))
		session = identityapp.Session{AccessToken: token, ExpiresAt: r.now().Add(time.Hour)}
		operator, _ = r.identity.Authenticate(req.Context(), token)
	}
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	deviceID, warehouseID := input.DeviceID, ""
	if session.DeviceID != "" {
		deviceID = session.DeviceID
	}
	if session.WarehouseID != "" {
		warehouseID = session.WarehouseID
	}
	data, err := r.sessionData(req.Context(), operator, session, deviceID, warehouseID)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, data, correlation(req.Context()), r.now())
}
func (r *Router) logout(w http.ResponseWriter, req *http.Request) {
	var input struct {
		RefreshToken string `json:"refreshToken"`
	}
	if req.ContentLength > 0 {
		_ = decode(req, &input)
	}
	if err := r.identity.LogoutContext(req.Context(), strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "), input.RefreshToken); err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (r *Router) operational(w http.ResponseWriter, req *http.Request) {
	writeData(w, http.StatusOK, map[string]string{"status": "UP", "service": "pda-api-gateway"}, correlation(req.Context()), r.now())
}
func (r *Router) me(w http.ResponseWriter, req *http.Request) {
	operator := req.Context().Value(operatorKey).(identity.Operator)
	writeData(w, http.StatusOK, map[string]any{"operatorId": operator.ID, "employeeCode": operator.EmployeeCode, "displayName": operator.DisplayName, "username": operator.Username, "roles": operator.Roles, "permissions": operator.Permissions, "shiftCode": operator.ShiftCode, "active": operator.Active}, correlation(req.Context()), r.now())
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
	deviceStatus, err := r.identity.DeviceStatus(req.Context(), operator, req.Header.Get("X-Device-Id"), req.Header.Get("X-Warehouse-Id"))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	assignments := r.operatorAssignments(req.Context(), operator, warehouses)
	writeData(w, http.StatusOK, map[string]any{"operatorId": operator.ID, "employeeCode": operator.EmployeeCode, "displayName": operator.DisplayName, "roles": operator.Roles, "permissions": operator.Permissions, "warehouseId": req.Header.Get("X-Warehouse-Id"), "warehouses": warehouses, "warehouseAssignments": assignments.Warehouses, "areaAssignments": assignments.Areas, "shiftCode": operator.ShiftCode, "deviceId": req.Header.Get("X-Device-Id"), "deviceRegistrationStatus": deviceStatus, "featureFlags": map[string]bool{}, "scannerPolicy": map[string]any{}, "localePolicy": []string{"en-US", "vi-VN"}, "serverTime": r.now().UTC()}, correlation(req.Context()), r.now())
}

func (r *Router) sessionData(ctx context.Context, operator identity.Operator, session identityapp.Session, deviceID, warehouseID string) (map[string]any, error) {
	warehouses, err := r.identity.Warehouses(ctx, operator)
	if err != nil {
		return nil, err
	}
	warehouseName := ""
	for _, warehouse := range warehouses {
		if warehouse.ID == warehouseID {
			warehouseName = warehouse.Name
			break
		}
	}
	deviceStatus, err := r.identity.DeviceStatus(ctx, operator, deviceID, warehouseID)
	if err != nil {
		return nil, err
	}
	assignments := r.operatorAssignments(ctx, operator, warehouses)
	accessExpiresIn := 0
	if !session.ExpiresAt.IsZero() {
		accessExpiresIn = int(session.ExpiresAt.Sub(r.now()).Seconds())
	}
	if accessExpiresIn < 0 {
		accessExpiresIn = 0
	}
	return map[string]any{"accessToken": session.AccessToken, "refreshToken": session.RefreshToken, "tokenType": "Bearer", "expiresAt": session.ExpiresAt.UTC(), "accessTokenExpiresIn": accessExpiresIn, "refreshTokenExpiresAt": session.RefreshTokenExpiresAt.UTC(), "operatorId": operator.ID, "employeeCode": operator.EmployeeCode, "displayName": operator.DisplayName, "username": operator.Username, "roles": operator.Roles, "permissions": operator.Permissions, "warehouseId": warehouseID, "warehouseName": warehouseName, "warehouseAssignments": assignments.Warehouses, "areaAssignments": assignments.Areas, "shiftCode": operator.ShiftCode, "deviceRegistrationStatus": deviceStatus, "featureFlags": map[string]bool{}, "scannerPolicy": map[string]any{}, "localePolicy": []string{"en-US", "vi-VN"}}, nil
}

type operatorAssignments struct {
	Warehouses []map[string]string
	Areas      []map[string]string
}

// operatorAssignments reads the authoritative WMS Master Data entitlement
// when configured. Authentication remains owned by PDA Backend; this is only
// a read-through projection for the profile screen and safely falls back to
// the identity warehouse list when the optional enrichment is unavailable.
func (r *Router) operatorAssignments(ctx context.Context, operator identity.Operator, warehouses []identity.Warehouse) operatorAssignments {
	out := operatorAssignments{Warehouses: make([]map[string]string, 0, len(warehouses)), Areas: []map[string]string{}}
	for _, warehouse := range warehouses {
		out.Warehouses = append(out.Warehouses, map[string]string{"id": warehouse.ID, "name": warehouse.Name})
	}
	base := strings.TrimRight(os.Getenv("PDA_UPSTREAM_WMS_BASE_URL"), "/")
	if base == "" {
		return out
	}
	endpoint := base + "/api/wms/master-data/operator-access/operators?account_type=PDA_APP&login_username=" + url.QueryEscape(operator.Username) + "&limit=1"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out
	}
	req.Header.Set("X-Calling-Service", "PDA_BACKEND")
	if token := strings.TrimSpace(os.Getenv("PDA_UPSTREAM_WMS_SERVICE_TOKEN")); token != "" {
		req.Header.Set("X-Service-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return out
	}
	var payload struct {
		Data []struct {
			WarehouseIDs         []string `json:"warehouse_ids"`
			ZoneIDs              []string `json:"zone_ids"`
			WarehouseAssignments []struct {
				ID   string          `json:"id"`
				Name json.RawMessage `json:"name"`
			} `json:"warehouse_assignments"`
			ZoneAssignments []struct {
				ID   string          `json:"id"`
				Name json.RawMessage `json:"name"`
			} `json:"zone_assignments"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil || len(payload.Data) == 0 {
		return out
	}
	warehouseNames := make(map[string]string, len(warehouses))
	for _, warehouse := range warehouses {
		warehouseNames[warehouse.ID] = warehouse.Name
	}
	assignment := payload.Data[0]
	if len(assignment.WarehouseAssignments) > 0 {
		out.Warehouses = make([]map[string]string, 0, len(assignment.WarehouseAssignments))
		for _, item := range assignment.WarehouseAssignments {
			name := localizedAssignmentName(item.Name)
			if name == "" {
				name = warehouseNames[item.ID]
			}
			out.Warehouses = append(out.Warehouses, map[string]string{"id": item.ID, "name": name})
		}
	} else {
		out.Warehouses = make([]map[string]string, 0, len(assignment.WarehouseIDs))
		for _, id := range assignment.WarehouseIDs {
			out.Warehouses = append(out.Warehouses, map[string]string{"id": id, "name": warehouseNames[id]})
		}
	}
	if len(assignment.ZoneAssignments) > 0 {
		for _, item := range assignment.ZoneAssignments {
			name := localizedAssignmentName(item.Name)
			if name == "" {
				name = item.ID
			}
			out.Areas = append(out.Areas, map[string]string{"id": item.ID, "name": name})
		}
		return out
	}
	for _, id := range assignment.ZoneIDs {
		name := id
		zoneReq, zoneErr := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/wms/master-data/zones/"+url.PathEscape(id), nil)
		if zoneErr == nil {
			zoneReq.Header.Set("X-Calling-Service", "PDA_BACKEND")
			if token := strings.TrimSpace(os.Getenv("PDA_UPSTREAM_WMS_SERVICE_TOKEN")); token != "" {
				zoneReq.Header.Set("X-Service-Token", token)
			}
			if zoneResp, requestErr := http.DefaultClient.Do(zoneReq); requestErr == nil {
				var zone struct {
					Code string          `json:"zone_code"`
					Name json.RawMessage `json:"zone_name"`
				}
				if zoneResp.StatusCode >= 200 && zoneResp.StatusCode < 300 && json.NewDecoder(io.LimitReader(zoneResp.Body, 1<<20)).Decode(&zone) == nil {
					if localized := localizedAssignmentName(zone.Name); localized != "" {
						name = localized
					} else if strings.TrimSpace(zone.Code) != "" {
						name = zone.Code
					}
				}
				zoneResp.Body.Close()
			}
		}
		out.Areas = append(out.Areas, map[string]string{"id": id, "name": name})
	}
	return out
}

func localizedAssignmentName(raw json.RawMessage) string {
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return strings.TrimSpace(plain)
	}
	var values map[string]string
	if json.Unmarshal(raw, &values) != nil {
		return ""
	}
	for _, locale := range []string{"vi", "en", "ja", "ko"} {
		if value := strings.TrimSpace(values[locale]); value != "" {
			return value
		}
	}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
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
	priorityMin, _ := strconv.Atoi(req.URL.Query().Get("priorityMin"))
	priorityMax, _ := strconv.Atoi(req.URL.Query().Get("priorityMax"))
	filter := executionports.TaskFilter{WarehouseID: req.Header.Get("X-Warehouse-Id"), Category: req.URL.Query().Get("category"), Status: req.URL.Query().Get("status"), Query: req.URL.Query().Get("q"), Cursor: req.URL.Query().Get("cursor"), PriorityMin: priorityMin, PriorityMax: priorityMax, Zone: req.URL.Query().Get("zone"), DateFrom: req.URL.Query().Get("dateFrom"), DateTo: req.URL.Query().Get("dateTo"), Sort: req.URL.Query().Get("sort"), Direction: req.URL.Query().Get("direction"), Limit: limit}
	page, err := r.tasks.List(req.Context(), filter, r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	items := make([]map[string]any, 0, len(page.Tasks))
	for _, task := range page.Tasks {
		items = append(items, taskView(task, r.actor(req).OperatorID))
	}
	now := r.now().UTC()
	writeData(w, http.StatusOK, map[string]any{"items": items, "nextCursor": page.NextCursor, "asOf": now, "stale": false}, correlation(req.Context()), now)
}
func (r *Router) taskDetail(w http.ResponseWriter, req *http.Request) {
	var task executiondomain.Task
	var err error
	if r.tasks != nil {
		task, err = r.tasks.Detail(req.Context(), chi.URLParam(req, "taskId"), r.actor(req))
	} else {
		err = &platform.DomainError{Code: "TASK_NOT_FOUND", SafeMessage: "Task not found"}
	}
	if err != nil {
		// The task feed is a cross-workflow endpoint and includes receiving
		// work. Receiving is owned by its own application boundary, so a
		// receiving task is not necessarily present in the execution-task
		// store. Keep the generic detail route consistent with its feed by
		// resolving that workflow through the receiving port when the
		// execution lookup is a genuine not-found.
		var domainErr *platform.DomainError
		if r.receiving != nil && errors.As(err, &domainErr) && domainErr.Code == "TASK_NOT_FOUND" {
			if receivingTask, receivingErr := r.receiving.Detail(req.Context(), chi.URLParam(req, "taskId"), r.actor(req)); receivingErr == nil {
				writeData(w, http.StatusOK, receivingTaskView(receivingTask, r.actor(req).OperatorID), correlation(req.Context()), r.now())
				return
			}
		}
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, taskView(task, r.actor(req).OperatorID), correlation(req.Context()), r.now())
}

func receivingTaskView(task receivingdomain.Task, operatorID string) map[string]any {
	lockState := "AVAILABLE"
	if task.OperatorID != nil {
		lockState = "LOCKED"
		if *task.OperatorID == operatorID {
			lockState = "OWNED"
		}
	}
	var pieceCount int64
	for _, line := range task.Lines {
		pieceCount += line.ExpectedQuantity
	}
	lines := make([]executiondomain.ReceivingLineSnapshot, 0, len(task.Lines))
	for _, line := range task.Lines {
		lines = append(lines, executiondomain.ReceivingLineSnapshot{
			LineID:             line.ID,
			ItemID:             line.ItemID,
			SKU:                line.SKU,
			ItemName:           line.ItemName,
			UOMCode:            line.UOMCode,
			LotCode:            line.LotCode,
			Barcode:            line.Barcode,
			ExpectedQuantity:   float64(line.ExpectedQuantity),
			HandedOverQuantity: float64(line.HandedOverQuantity),
			ReceivedQuantity:   float64(line.ReceivedQuantity),
			RemainingQuantity:  float64(line.ExpectedQuantity - line.ReceivedQuantity),
			LotRequired:        task.Policy.LotRequired,
			SerialRequired:     task.Policy.SerialRequired,
		})
	}
	return map[string]any{
		"id":                 task.ID,
		"category":           "RECEIVING",
		"type":               "RECEIVING",
		"status":             task.Status,
		"priority":           0,
		"title":              task.PONumber,
		"lineCount":          len(task.Lines),
		"pieceCount":         pieceCount,
		"dueAt":              nil,
		"assignedOperatorId": task.OperatorID,
		"lockState":          lockState,
		"version":            task.Version,
		"createdAt":          task.UpdatedAt.UTC(),
		"updatedAt":          task.UpdatedAt.UTC(),
		"warehouseId":        task.WarehouseID,
		"purchaseOrderId":    task.PONumber,
		"supplier":           task.Supplier,
		"lines":              lines,
	}
}

func taskView(task executiondomain.Task, operatorID string) map[string]any {
	title := task.Title
	if title == "" {
		title = string(task.Category) + " task"
	}
	createdAt := task.CreatedAt
	if createdAt.IsZero() {
		createdAt = task.UpdatedAt
	}
	lockState := task.LockState
	if lockState == "" {
		lockState = "AVAILABLE"
		if task.OperatorID != nil {
			lockState = "LOCKED"
			if *task.OperatorID == operatorID {
				lockState = "OWNED"
			}
		}
	}
	return map[string]any{"id": task.ID, "category": task.Category, "type": task.Category, "status": task.Status, "priority": task.Priority, "title": title, "lineCount": task.LineCount, "pieceCount": task.PieceCount, "dueAt": task.DueAt, "assignedOperatorId": task.OperatorID, "lockState": lockState, "version": task.Version, "createdAt": createdAt.UTC(), "updatedAt": task.UpdatedAt.UTC(), "warehouseId": task.WarehouseID, "purchaseOrderId": task.PurchaseOrderID, "supplier": task.Supplier, "lines": task.ReceivingLines}
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
	page, err := r.receiving.List(req.Context(), receivingports.Filter{WarehouseID: req.Header.Get("X-Warehouse-Id"), Status: req.URL.Query().Get("status"), Query: req.URL.Query().Get("q"), Cursor: req.URL.Query().Get("cursor"), Limit: limit}, r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	items := make([]map[string]any, 0, len(page.Items))
	for _, task := range page.Items {
		items = append(items, receivingView(task))
	}
	now := r.now().UTC()
	writeData(w, http.StatusOK, map[string]any{"items": items, "nextCursor": page.NextCursor, "asOf": now, "stale": false}, correlation(req.Context()), now)
}
func (r *Router) receivingDetail(w http.ResponseWriter, req *http.Request) {
	task, err := r.receiving.Detail(req.Context(), chi.URLParam(req, "taskId"), r.actor(req))
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, receivingView(task), correlation(req.Context()), r.now())
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

func (r *Router) receivingClaim(w http.ResponseWriter, req *http.Request) {
	command, err := r.receivingBaseCommand(req)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	task, err := r.receiving.Claim(req.Context(), command)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	writeData(w, http.StatusOK, receivingView(task), correlation(req.Context()), r.now())
}
func (r *Router) receivingBarcode(w http.ResponseWriter, req *http.Request) {
	var input struct {
		TaskID          string     `json:"taskId"`
		LineID          *string    `json:"lineId"`
		RawValue        string     `json:"rawValue"`
		NormalizedValue string     `json:"normalizedValue"`
		Barcode         string     `json:"barcode"`
		Symbology       string     `json:"symbology"`
		ScanContext     string     `json:"scanContext"`
		OperatorID      string     `json:"operatorId"`
		WarehouseID     string     `json:"warehouseId"`
		DeviceID        string     `json:"deviceId"`
		ScannedAt       *time.Time `json:"scannedAt"`
	}
	if err := decode(req, &input); err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	barcode := input.NormalizedValue
	if barcode == "" {
		barcode = input.Barcode
	}
	if barcode == "" {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Barcode is required"}, correlation(req.Context()))
		return
	}
	if input.TaskID != "" && input.TaskID != chi.URLParam(req, "taskId") || input.ScanContext != "" && input.ScanContext != "RECEIVING_ITEM" && input.ScanContext != "RECEIVING_LOT" {
		writeError(w, &platform.DomainError{Code: "BARCODE_WRONG_CONTEXT", SafeMessage: "Barcode scan context is invalid"}, correlation(req.Context()))
		return
	}
	actor := r.actor(req)
	if (input.OperatorID != "" && input.OperatorID != actor.OperatorID) || (input.WarehouseID != "" && input.WarehouseID != actor.WarehouseID) || (input.DeviceID != "" && input.DeviceID != actor.DeviceID) {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Scanner context does not match the request headers"}, correlation(req.Context()))
		return
	}
	var line receivingdomain.Line
	var err error
	if resolver, ok := r.receiving.(gatewayports.ReceivingBarcodeOperations); ok {
		symbology := strings.ToUpper(strings.TrimSpace(input.Symbology))
		if symbology == "" {
			symbology = "UNKNOWN"
		}
		line, err = resolver.ResolveBarcodeWithSymbology(req.Context(), chi.URLParam(req, "taskId"), barcode, symbology, actor)
	} else {
		line, err = r.receiving.ResolveBarcode(req.Context(), chi.URLParam(req, "taskId"), barcode, actor)
	}
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	scanContext := input.ScanContext
	if scanContext == "" {
		scanContext = "RECEIVING_ITEM"
	}
	writeData(w, http.StatusOK, map[string]any{"lineId": line.ID, "itemId": line.ItemID, "sku": line.SKU, "itemName": line.ItemName, "barcode": line.Barcode, "rawValue": input.RawValue, "normalizedValue": barcode, "symbology": input.Symbology, "scanContext": scanContext, "remainingQuantity": line.ExpectedQuantity - line.ReceivedQuantity, "quantityPolicy": map[string]any{"allowOverReceipt": false}, "nextStep": "CONFIRM_QUANTITY", "taskVersion": nil, "scannedAt": input.ScannedAt}, correlation(req.Context()), r.now())
}
func (r *Router) receivingConfirm(w http.ResponseWriter, req *http.Request) {
	base, err := r.receivingBaseCommand(req)
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	var input struct {
		CommandID           uuid.UUID  `json:"commandId"`
		CommandIDSnake      uuid.UUID  `json:"command_id"`
		IdempotencyKey      string     `json:"idempotencyKey"`
		IdempotencyKeySnake string     `json:"idempotency_key"`
		TaskID              string     `json:"taskId"`
		TaskIDSnake         string     `json:"task_id"`
		LineID              string     `json:"lineId"`
		Barcode             string     `json:"barcode"`
		Quantity            int64      `json:"quantity"`
		Condition           string     `json:"condition"`
		Remark              *string    `json:"remark"`
		BaseVersion         int64      `json:"baseVersion"`
		BaseVersionSnake    int64      `json:"base_version"`
		ScannedAt           *time.Time `json:"scannedAt"`
		Lines               []struct {
			LineID              string `json:"lineId"`
			LineIDSnake         string `json:"line_id"`
			ItemRevisionID      string `json:"itemRevisionId"`
			ItemRevisionIDSnake string `json:"item_revision_id"`
			LotCode             string `json:"lotCode"`
			LotCodeSnake        string `json:"lot_code"`
			UOMCode             string `json:"uomCode"`
			UOMCodeSnake        string `json:"uom_code"`
			ActualQuantity      int64  `json:"actualQuantity"`
			ActualQuantitySnake int64  `json:"actual_quantity"`
		} `json:"lines"`
	}
	if input.CommandID == uuid.Nil {
		input.CommandID = input.CommandIDSnake
	}
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = input.IdempotencyKeySnake
	}
	if input.TaskID == "" {
		input.TaskID = input.TaskIDSnake
	}
	if input.BaseVersion == 0 {
		input.BaseVersion = input.BaseVersionSnake
	}
	if err := decode(req, &input); err != nil || input.CommandID != base.CommandID || input.BaseVersion != base.BaseVersion || input.TaskID != "" && input.TaskID != base.TaskID || input.IdempotencyKey != "" && input.IdempotencyKey != base.IdempotencyKey {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Command metadata does not match headers"}, correlation(req.Context()))
		return
	}
	var task receivingdomain.Task
	if len(input.Lines) > 0 {
		batch, ok := r.receiving.(gatewayports.ReceivingBatchOperations)
		if !ok {
			writeError(w, &platform.DomainError{Code: "RECEIVING_BATCH_UNAVAILABLE", SafeMessage: "Batch receiving is not available for this runtime"}, correlation(req.Context()))
			return
		}
		lines := make([]receivingapp.BatchLine, 0, len(input.Lines))
		for _, line := range input.Lines {
			if line.LineID == "" {
				line.LineID = line.LineIDSnake
			}
			if line.ItemRevisionID == "" {
				line.ItemRevisionID = line.ItemRevisionIDSnake
			}
			if line.LotCode == "" {
				line.LotCode = line.LotCodeSnake
			}
			if line.UOMCode == "" {
				line.UOMCode = line.UOMCodeSnake
			}
			if line.ActualQuantity == 0 {
				line.ActualQuantity = line.ActualQuantitySnake
			}
			lines = append(lines, receivingapp.BatchLine{LineID: line.LineID, ItemRevisionID: line.ItemRevisionID, LotCode: line.LotCode, UOMCode: line.UOMCode, ActualQuantity: line.ActualQuantity})
		}
		task, err = batch.ConfirmBatch(req.Context(), receivingapp.BatchConfirmCommand{Command: base, Lines: lines, Remark: input.Remark})
	} else {
		task, err = r.receiving.Confirm(req.Context(), receivingapp.ConfirmCommand{Command: base, LineID: input.LineID, Barcode: input.Barcode, Quantity: input.Quantity, Condition: input.Condition, Remark: input.Remark, ScannedAt: input.ScannedAt})
	}
	if err != nil {
		writeError(w, err, correlation(req.Context()))
		return
	}
	view := receivingView(task)
	view["commandStatus"] = map[string]any{"commandId": base.CommandID, "status": "COMPLETED"}
	view["auditId"] = base.CommandID
	view["nextStep"] = "CONTINUE_SCANNING"
	writeData(w, http.StatusOK, view, correlation(req.Context()), r.now())
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
	writeData(w, http.StatusOK, receivingView(task), correlation(req.Context()), r.now())
}

func receivingView(task receivingdomain.Task) map[string]any {
	var expected, received int64
	lines := make([]map[string]any, 0, len(task.Lines))
	for _, line := range task.Lines {
		expected += line.ExpectedQuantity
		received += line.ReceivedQuantity
		lines = append(lines, map[string]any{"lineId": line.ID, "itemId": line.ItemID, "sku": line.SKU, "itemName": line.ItemName, "barcode": line.Barcode, "lotCode": line.LotCode, "uomCode": line.UOMCode, "expectedQuantity": line.ExpectedQuantity, "receivedQuantity": line.ReceivedQuantity, "remainingQuantity": line.ExpectedQuantity - line.ReceivedQuantity, "lotRequired": task.Policy.LotRequired, "serialRequired": task.Policy.SerialRequired})
	}
	return map[string]any{"taskId": task.ID, "orderId": task.OrderID, "purchaseOrderId": task.PONumber, "poNumber": task.PONumber, "supplier": task.Supplier, "status": task.Status, "warehouseId": task.WarehouseID, "assignedOperatorId": task.OperatorID, "version": task.Version, "expectedQuantity": expected, "receivedQuantity": received, "remainingQuantity": expected - received, "quantityPolicy": task.Policy, "conditionPolicy": task.Policy.ConditionPolicy, "lines": lines, "updatedAt": task.UpdatedAt, "asOf": task.UpdatedAt}
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

func (r *Router) commandStatus(w http.ResponseWriter, req *http.Request) {
	id, err := uuid.Parse(chi.URLParam(req, "commandId"))
	if err != nil {
		writeError(w, &platform.DomainError{Code: "INVALID_REQUEST", SafeMessage: "Invalid command ID"}, correlation(req.Context()))
		return
	}
	actor := r.actor(req)
	if r.receiving != nil {
		if status, lookupErr := r.receiving.CommandStatus(req.Context(), id, actor); lookupErr == nil {
			writeData(w, http.StatusOK, canonicalCommand(id, status.CommandType, status.Status, status.Result), correlation(req.Context()), r.now())
			return
		} else if !commandLookupMiss(lookupErr) {
			writeError(w, lookupErr, correlation(req.Context()))
			return
		}
	}
	if r.movementCommands != nil {
		if status, lookupErr := r.movementCommands.CommandStatus(req.Context(), id, actor); lookupErr == nil {
			writeData(w, http.StatusOK, canonicalCommand(id, string(status.Workflow), "ACKNOWLEDGED", status.Result), correlation(req.Context()), r.now())
			return
		} else if !commandLookupMiss(lookupErr) {
			writeError(w, lookupErr, correlation(req.Context()))
			return
		}
	}
	if r.inventory != nil {
		if status, lookupErr := r.inventory.CommandStatus(req.Context(), id, actor); lookupErr == nil {
			writeData(w, http.StatusOK, canonicalCommand(id, "INVENTORY", "ACKNOWLEDGED", status.Result), correlation(req.Context()), r.now())
			return
		} else if !commandLookupMiss(lookupErr) {
			writeError(w, lookupErr, correlation(req.Context()))
			return
		}
	}
	if r.shipping != nil {
		if status, lookupErr := r.shipping.CommandStatus(req.Context(), id, actor); lookupErr == nil {
			writeData(w, http.StatusOK, canonicalCommand(id, "SHIPMENT_CONFIRM", "ACKNOWLEDGED", status.Result), correlation(req.Context()), r.now())
			return
		} else if !commandLookupMiss(lookupErr) {
			writeError(w, lookupErr, correlation(req.Context()))
			return
		}
	}
	if r.tasks != nil {
		if status, lookupErr := r.tasks.CommandStatus(req.Context(), id, actor); lookupErr == nil {
			encoded, _ := json.Marshal(status.Task)
			writeData(w, http.StatusOK, canonicalCommand(id, "TASK_MUTATION", "ACKNOWLEDGED", json.RawMessage(encoded)), correlation(req.Context()), r.now())
			return
		} else if !commandLookupMiss(lookupErr) {
			writeError(w, lookupErr, correlation(req.Context()))
			return
		}
	}
	writeError(w, &platform.DomainError{Code: "COMMAND_NOT_FOUND", SafeMessage: "Command status not found"}, correlation(req.Context()))
}

func commandLookupMiss(err error) bool {
	var domainErr *platform.DomainError
	return errors.As(err, &domainErr) && (domainErr.Code == "TASK_NOT_FOUND" || domainErr.Code == "INVENTORY_NOT_FOUND" || domainErr.Code == "SHIPMENT_NOT_FOUND" || domainErr.Code == "COMMAND_NOT_FOUND")
}

func canonicalCommand(id uuid.UUID, operation, status string, raw json.RawMessage) map[string]any {
	var result any
	_ = json.Unmarshal(raw, &result)
	aggregateID := ""
	if value, ok := result.(map[string]any); ok {
		for _, key := range []string{"taskId", "id", "shipmentId", "transferId", "commandId"} {
			if candidate, ok := value[key].(string); ok && candidate != "" {
				aggregateID = candidate
				break
			}
		}
	}
	return map[string]any{"commandId": id, "idempotencyKey": id.String(), "operation": operation, "type": operation, "status": status, "aggregateId": aggregateID, "version": nil, "result": result, "errorCode": nil, "correlationId": nil, "processedAt": nil}
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
	meta := map[string]any{
		"serverTime":    now.UTC(),
		"correlationId": correlationID,
		"version":       nil,
		"nextCursor":    nil,
		"hasMore":       false,
		"asOf":          nil,
		"stale":         false,
	}
	if encoded, err := json.Marshal(data); err == nil {
		var fields map[string]any
		if json.Unmarshal(encoded, &fields) == nil {
			for _, key := range []string{"version", "nextCursor", "hasMore", "asOf", "stale"} {
				if value, ok := fields[key]; ok {
					meta[key] = value
				}
			}
			if meta["nextCursor"] != nil {
				meta["hasMore"] = true
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "meta": meta, "errors": []any{}})
}
func writeError(w http.ResponseWriter, err error, correlationID string) {
	var domainError *platform.DomainError
	if !errors.As(err, &domainError) {
		domainError = &platform.DomainError{Code: "INTERNAL_ERROR", SafeMessage: "Internal error"}
	}
	status := http.StatusInternalServerError
	switch domainError.Code {
	case "INVALID_REQUEST", "DEVICE_NOT_REGISTERED":
		status = http.StatusBadRequest
	case "AUTH_INVALID_CREDENTIALS", "AUTH_SESSION_EXPIRED", "AUTH_TOKEN_INVALID", "AUTH_TOKEN_REVOKED", "UPSTREAM_UNAUTHORIZED", "ACCESS_TOKEN_EXPIRED", "ACCESS_TOKEN_INVALID", "REFRESH_TOKEN_INVALID", "REFRESH_TOKEN_EXPIRED", "REFRESH_TOKEN_REVOKED", "REFRESH_TOKEN_REUSED", "SESSION_REVOKED":
		status = http.StatusUnauthorized
	case "AUTH_ACCOUNT_LOCKED":
		status = http.StatusTooManyRequests
	case "AUTH_ACCOUNT_DISABLED", "USER_DISABLED":
		status = http.StatusForbidden
	case "WAREHOUSE_ACCESS_DENIED", "OPERATOR_CONTEXT_MISMATCH", "SESSION_DEVICE_MISMATCH":
		status = http.StatusForbidden
	case "RATE_LIMITED":
		status = http.StatusTooManyRequests
	case "GATEWAY_CIRCUIT_OPEN", "UPSTREAM_WMS_UNAVAILABLE", "MESSAGING_PUBLISH_PENDING":
		status = http.StatusServiceUnavailable
	case "TASK_LOCKED", "TASK_VERSION_CONFLICT", "SHIPMENT_VERSION_CONFLICT", "DUPLICATE_COMMAND", "IDEMPOTENCY_KEY_REUSED", "VERSION_CONFLICT", "IDEMPOTENCY_CONFLICT", "RECEIPT_NOT_CONFIRMABLE", "OVER_RECEIPT_APPROVAL_REQUIRED", "UPSTREAM_CONFLICT":
		status = http.StatusConflict
	case "TASK_NOT_FOUND", "INVENTORY_NOT_FOUND", "SHIPMENT_NOT_FOUND", "COMMAND_NOT_FOUND", "RECEIPT_NOT_FOUND", "UPSTREAM_NOT_FOUND":
		status = http.StatusNotFound
	case "BARCODE_UNKNOWN", "BARCODE_WRONG_CONTEXT", "QUANTITY_EXCEEDS_ALLOWED", "REMARK_REQUIRED", "CONDITION_INVALID", "RECEIVING_TASK_INCOMPLETE", "TASK_NOT_ASSIGNED", "TASK_ALREADY_COMPLETED", "SOURCE_LOCATION_INVALID", "DESTINATION_LOCATION_INVALID", "ITEM_INVALID", "VALIDATION_SEQUENCE_INVALID", "INSUFFICIENT_STOCK", "LOCATION_CAPACITY_EXCEEDED", "TASK_INCOMPLETE", "SOURCE_EQUALS_DESTINATION", "SHIPMENT_NOT_READY", "PACKAGE_INCOMPLETE", "CARRIER_INVALID", "TRACKING_INVALID", "SHIPMENT_ALREADY_CONFIRMED", "COUNT_VARIANCE_REQUIRES_REVIEW", "ITEM_NOT_IN_DOCUMENT", "LOCATION_INVALID", "RECEIPT_LINE_MISMATCH", "RECEIPT_QUANTITY_INVALID", "RECEIPT_BATCH_LINE_INVALID", "RECEIPT_BATCH_LINES_INCOMPLETE", "RECEIPT_BATCH_DUPLICATE_LINE", "RECEIPT_LOT_DIMENSION_MISMATCH", "RECEIPT_LINE_FAILED_FINAL", "INVALID_INVENTORY_REQUEST":
		status = http.StatusUnprocessableEntity
	case "RECEIPT_VERSION_REQUIRED":
		status = http.StatusBadRequest
	case "RECEIPT_NOT_EDITABLE", "RECEIPT_NOT_CLAIMABLE":
		status = http.StatusConflict
	}
	// Only an explicit dependency outage may affect the gateway-wide breaker.
	// An UPSTREAM_HTTP_ERROR is route-scoped: optional WMS capabilities can
	// legitimately be unavailable while Receiving, authentication, and SSE
	// remain healthy. Treating every upstream 5xx as a global outage causes an
	// unrelated read to make the Receiving workflow return GATEWAY_CIRCUIT_OPEN.
	if domainError.Code == "UPSTREAM_WMS_UNAVAILABLE" || domainError.Code == "MESSAGING_PUBLISH_PENDING" {
		w.Header().Set("X-Gateway-Dependency-Failure", "1")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": nil,
		"meta": map[string]any{
			"serverTime":    time.Now().UTC(),
			"correlationId": correlationID,
		},
		"errors": []any{map[string]any{
			"code":      domainError.Code,
			"message":   domainError.SafeMessage,
			"details":   domainError.Details,
			"retryable": domainError.Retryable,
		}},
	})
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
