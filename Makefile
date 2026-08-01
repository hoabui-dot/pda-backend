.PHONY: fmt lint test test-integration test-kafka test-contract test-architecture build verify clean migrate-up migrate-down

SERVICES := pda-api-gateway identity-access-service pda-execution-service inventory-service outbound-shipping-service integration-event-service

fmt:
	@test -z "$$(gofmt -l $$(find cmd internal test -name '*.go' -type f))"

lint:
	go vet ./...

test:
	go test ./...

test-integration:
	PDA_TEST_DATABASE_URL="$${PDA_TEST_DATABASE_URL:-postgres://pda:local-only-pda@localhost:15432/pda?sslmode=disable}" go test -tags=integration ./...

test-kafka:
	PDA_KAFKA_BROKERS="$${PDA_KAFKA_BROKERS:-127.0.0.1:19092}" go test ./internal/integration/adapters/kafka -run 'Test(SharedMESKafkaEnvironmentProvidesRequiredPDA8Topics|PublisherRoundTrip|ConsumerGroupProcessesAndMarksInbox|PublisherFailsClosedDuringBrokerOutage)'

test-contract:
	go test ./test/contract/...

test-architecture:
	go test ./test/architecture/...

build:
	@mkdir -p bin
	@for service in $(SERVICES); do go build -trimpath -o bin/$$service ./cmd/$$service; done

verify: fmt lint test test-integration test-kafka test-contract test-architecture build

migrate-up:
	docker run --rm --network pda-backend_default -v "$(CURDIR)/migrations/execution:/migrations:ro" migrate/migrate:v4.18.3 -path=/migrations -database='postgres://pda:local-only-pda@postgres:5432/pda?sslmode=disable' up

migrate-down:
	docker run --rm --network pda-backend_default -v "$(CURDIR)/migrations/execution:/migrations:ro" migrate/migrate:v4.18.3 -path=/migrations -database='postgres://pda:local-only-pda@postgres:5432/pda?sslmode=disable' down 1

clean:
	go clean -testcache
	@for service in $(SERVICES); do test ! -e bin/$$service || unlink bin/$$service; done
	@rmdir bin 2>/dev/null || true
