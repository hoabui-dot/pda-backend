package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	executionpostgres "github.com/company/pda-backend/internal/execution/adapters/postgres"
	executionredis "github.com/company/pda-backend/internal/execution/adapters/rediscache"
	executionapp "github.com/company/pda-backend/internal/execution/application"
	movementpostgres "github.com/company/pda-backend/internal/execution/movement/adapters/postgres"
	movementapp "github.com/company/pda-backend/internal/execution/movement/application"
	receivingpostgres "github.com/company/pda-backend/internal/execution/receiving/adapters/postgres"
	receivingapp "github.com/company/pda-backend/internal/execution/receiving/application"
	httpadapter "github.com/company/pda-backend/internal/gateway/adapters/http"
	identitymock "github.com/company/pda-backend/internal/identity/adapters/mock"
	identityapp "github.com/company/pda-backend/internal/identity/application"
	kafkaadapter "github.com/company/pda-backend/internal/integration/adapters/kafka"
	messagingmock "github.com/company/pda-backend/internal/integration/adapters/messagingmock"
	wmsmock "github.com/company/pda-backend/internal/integration/adapters/wmsmock"
	inventorypostgres "github.com/company/pda-backend/internal/inventory/adapters/postgres"
	inventoryredis "github.com/company/pda-backend/internal/inventory/adapters/rediscache"
	inventoryapp "github.com/company/pda-backend/internal/inventory/application"
	platformcache "github.com/company/pda-backend/internal/platform/cache"
	redisadapter "github.com/company/pda-backend/internal/platform/cache/adapters/redis"
	"github.com/company/pda-backend/internal/platform/config"
	"github.com/company/pda-backend/internal/platform/event"
	shippingpostgres "github.com/company/pda-backend/internal/shipping/adapters/postgres"
	shippingredis "github.com/company/pda-backend/internal/shipping/adapters/rediscache"
	shippingapp "github.com/company/pda-backend/internal/shipping/application"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	applicationConfig, err := config.Load()
	if err != nil {
		slog.Error("configuration rejected", "error", err)
		os.Exit(1)
	}
	if applicationConfig.Modes.Auth != "mock" {
		slog.Error("OIDC adapter is not enabled before production integration")
		os.Exit(1)
	}
	store, err := identitymock.LoadDefault()
	if err != nil {
		slog.Error("identity fixture rejected", "error", err)
		os.Exit(1)
	}
	tokens, err := identitymock.NewTokenProvider("local-mock-secret-must-never-be-production", time.Hour)
	if err != nil {
		slog.Error("mock token configuration rejected", "error", err)
		os.Exit(1)
	}
	identityService := identityapp.NewService(store, tokens, store, store, time.Now)
	tasks, err := wmsmock.NewTaskAdapter().Tasks(context.Background())
	if err != nil {
		slog.Error("task fixture rejected", "error", err)
		os.Exit(1)
	}
	pool, err := pgxpool.New(context.Background(), applicationConfig.DatabaseURL)
	if err != nil {
		slog.Error("database configuration rejected", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	redisOptions, err := redis.ParseURL(applicationConfig.RedisURL)
	if err != nil {
		slog.Error("redis configuration rejected", "error", err)
		os.Exit(1)
	}
	redisOptions.DialTimeout = 100 * time.Millisecond
	redisOptions.ReadTimeout = 100 * time.Millisecond
	redisOptions.WriteTimeout = 100 * time.Millisecond
	redisOptions.MaxRetries = -1
	cacheTTL, err := time.ParseDuration(applicationConfig.CacheTTL)
	if err != nil {
		slog.Error("cache TTL rejected", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()
	cacheMetrics := &platformcache.Metrics{}
	cacheAside := platformcache.NewAside(redisadapter.Store{Client: redisClient}, cacheTTL, cacheMetrics)
	cacheKeys := platformcache.KeyService{Version: "v1"}
	cacheInvalidator := platformcache.Invalidator{Cache: cacheAside, Keys: cacheKeys}
	taskStore := executionpostgres.New(pool)
	if err := taskStore.Seed(context.Background(), tasks); err != nil {
		slog.Error("task seed failed; apply migrations first", "error", err)
		os.Exit(1)
	}
	eventLog := messagingmock.NewInMemoryEventLog()
	var publisher event.DomainEventPublisher = messagingmock.NewPublisher(eventLog, "")
	if applicationConfig.Modes.Messaging == "kafka" {
		brokers := strings.Split(applicationConfig.KafkaBrokers, ",")
		kafkaPublisher, publishErr := kafkaadapter.NewPublisher(kafkaadapter.Config{Brokers: brokers, GroupID: applicationConfig.KafkaGroupID, TopicPrefix: applicationConfig.KafkaTopicPrefix, SecurityProtocol: applicationConfig.KafkaSecurityProtocol})
		if publishErr != nil {
			slog.Error("Kafka publisher configuration rejected", "error", publishErr)
			os.Exit(1)
		}
		publisher = kafkaPublisher
	}
	cachedTasks := executionredis.New(taskStore, cacheAside, cacheKeys)
	taskService := executionapp.NewTaskService(cachedTasks, executionpostgres.Idempotency{Store: taskStore}, taskStore, taskStore, publisher, cacheInvalidator, time.Now)
	receivingTasks, err := wmsmock.NewReceivingAdapter().Tasks(context.Background())
	if err != nil {
		slog.Error("receiving fixture rejected", "error", err)
		os.Exit(1)
	}
	receivingStore := receivingpostgres.New(pool)
	if err := receivingStore.Seed(context.Background(), receivingTasks); err != nil {
		slog.Error("receiving seed failed; apply migrations first", "error", err)
		os.Exit(1)
	}
	receivingService := receivingapp.New(receivingStore, receivingpostgres.Commands{Store: receivingStore}, receivingStore, receivingStore, receivingStore, publisher, cacheInvalidator, time.Now)
	movementStore := movementpostgres.New(pool)
	if err := movementStore.Seed(context.Background(), wmsmock.MovementTasks()); err != nil {
		slog.Error("movement seed failed; apply migrations first", "error", err)
		os.Exit(1)
	}
	movementServices := movementapp.New(movementStore, movementpostgres.Commands{Store: movementStore}, movementStore, movementStore, movementStore, publisher, cacheInvalidator, time.Now)
	inventoryStore := inventorypostgres.New(pool)
	if err := inventoryStore.Seed(context.Background()); err != nil {
		slog.Error("inventory seed failed; apply migrations first", "error", err)
		os.Exit(1)
	}
	cachedInventory := inventoryredis.New(inventoryStore, cacheAside, cacheKeys)
	inventoryService := inventoryapp.New(cachedInventory, inventorypostgres.Commands{Store: inventoryStore}, inventoryStore, inventoryStore, inventoryStore, publisher, cacheInvalidator, time.Now)
	shippingStore := shippingpostgres.New(pool)
	if err := shippingStore.Seed(context.Background()); err != nil {
		slog.Error("shipping seed failed; apply migrations first", "error", err)
		os.Exit(1)
	}
	cachedShipping := shippingredis.New(shippingStore, cacheAside, cacheKeys)
	shippingService := shippingapp.New(cachedShipping, shippingpostgres.Commands{Store: shippingStore}, shippingStore, shippingStore, shippingStore, publisher, cacheInvalidator, time.Now)
	handler, err := httpadapter.New(identityService, taskService, receivingService, movementServices, inventoryService, shippingService, httpadapter.Settings{RequestTimeout: 5 * time.Second, AuthRateLimit: 10, RateWindow: time.Minute, CircuitFailureThreshold: 5}, slog.Default(), time.Now)
	if err != nil {
		slog.Error("gateway configuration rejected", "error", err)
		os.Exit(1)
	}
	server := &http.Server{Addr: ":8080", Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("gateway stopped", "error", err)
		os.Exit(1)
	}
}
