package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/company/pda-backend/internal/integration/adapters/kafka"
	"github.com/company/pda-backend/internal/platform/config"
	"github.com/go-chi/chi/v5"
)

type Status struct {
	Status      string       `json:"status"`
	Service     string       `json:"service"`
	ServerTime  time.Time    `json:"serverTime"`
	RuntimeMode config.Modes `json:"runtimeMode"`
}

func Handler(service string, modes config.Modes, now func() time.Time) http.Handler {
	router := chi.NewRouter()
	status := func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(Status{"UP", service, now().UTC(), modes})
	}
	router.Get("/healthz", status)
	router.Get("/livez", status)
	router.Get("/readyz", status)
	return router
}

func Run(service, address string) error {
	applicationConfig, err := config.Load()
	if err != nil {
		return err
	}
	if applicationConfig.Modes.Messaging == "kafka" {
		return kafka.ErrBrokerUnavailable
	}
	server := &http.Server{
		Addr: address, Handler: Handler(service, applicationConfig.Modes, time.Now),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	slog.Info("service listening", "service", service, "address", address)
	return server.ListenAndServe()
}
