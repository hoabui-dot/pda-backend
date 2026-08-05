package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	kafkaadapter "github.com/company/pda-backend/internal/integration/adapters/kafka"
	"github.com/company/pda-backend/internal/platform/config"
	"github.com/company/pda-backend/internal/platform/httpserver"
	"github.com/company/pda-backend/internal/platform/messaging"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	applicationConfig, err := config.Load()
	if err != nil {
		slog.Error("configuration rejected", "error", err)
		os.Exit(1)
	}
	if applicationConfig.Modes.Messaging != "kafka" {
		if err := httpserver.Run("integration-event-service", ":8085"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("service stopped", "error", err)
			os.Exit(1)
		}
		return
	}
	if err := runKafka(applicationConfig); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("service stopped", "error", err)
		os.Exit(1)
	}
}

func runKafka(applicationConfig config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, applicationConfig.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	brokers := strings.Split(applicationConfig.KafkaBrokers, ",")
	publisher, err := kafkaadapter.NewPublisher(kafkaadapter.Config{Brokers: brokers, GroupID: applicationConfig.KafkaGroupID, TopicPrefix: applicationConfig.KafkaTopicPrefix, SecurityProtocol: applicationConfig.KafkaSecurityProtocol, TLSCAFile: applicationConfig.KafkaTLSCAFile, TLSCertFile: applicationConfig.KafkaTLSCertFile, TLSKeyFile: applicationConfig.KafkaTLSKeyFile, TLSServerName: applicationConfig.KafkaTLSServerName})
	if err != nil {
		return err
	}
	metrics := &messaging.Metrics{}
	worker := kafkaadapter.OutboxWorker{
		Store:     kafkaadapter.NewPostgresStoreWithGroup(pool, applicationConfig.KafkaGroupID),
		Publisher: publisher,
		BatchSize: 100,
		Metrics:   metrics,
	}

	// I-08: subscribe to the WMS warehouse task dispatch topics. Until this
	// existed the service only published; it received no work from the system
	// that owns the warehouse task.
	taskConsumer, err := kafkaadapter.NewWMSTaskConsumer(brokers, applicationConfig.WMSTaskGroupID, pool, strings.Split(applicationConfig.WMSTaskTopics, ","))
	if err != nil {
		return err
	}
	taskConsumer.Start()
	defer taskConsumer.Stop()
	slog.Info("WMS task consumer started", "group", applicationConfig.WMSTaskGroupID, "topics", applicationConfig.WMSTaskTopics)

	// I-08 (outbound): relay the cross-system outbox. These rows carry the
	// canonical snake_case envelope on absolute topics and must not be
	// topic-prefixed, so they need their own relay.
	integrationRelay, err := kafkaadapter.NewIntegrationRelay(brokers, pool)
	if err != nil {
		return err
	}
	integrationRelay.Start()
	defer integrationRelay.Stop()
	slog.Info("integration outbox relay started")

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if _, err := worker.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Warn("outbox publish cycle failed", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	server := &http.Server{
		Addr:              ":8085",
		Handler:           httpserver.Handler("integration-event-service", applicationConfig.Modes, time.Now),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("service listening", "service", "integration-event-service", "address", ":8085", "messaging", "kafka")
	return server.ListenAndServe()
}
