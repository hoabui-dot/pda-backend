# Backend Phase BE-08 — Kafka Delivery

Date: 2026-08-01  
Status: **PASS (shared MES broker, local PLAINTEXT profile)**

## Verification

Re-verification completed against the existing MES platform Kafka broker rather
than a second PDA-owned broker.

- `make verify`: **PASS** (format, vet, unit, PostgreSQL/Redis integration, shared-broker Kafka, contract, architecture, and build checks).
- Migration 000006 `event_delivery`: **PASS**; down/up rollback test passed and schema version is `6|f`.
- PostgreSQL: accepting connections.
- Redis: healthy.
- Kafka: existing MES `platform-kafka` broker is healthy on host port `19092`; required PDA topics were provisioned idempotently and producer/consumer verification passed.

## Implemented safely

- Added optional Kafka configuration fields and documented security/group/topic settings.
- Removed the duplicate PDA-owned Redpanda service from `docker/compose.yml`; PDA now uses the shared MES broker.
- Added `config/kafka.env.example` with the host and shared Docker-network broker addresses.
- Added `make test-kafka` and made it part of `make verify`.
- Added shared-broker topic provisioning for:
  - `pda.task.events.v1`
  - `pda.receiving.events.v1`
  - `pda.movement.events.v1`
  - `pda.inventory.events.v1`
  - `pda.shipping.events.v1`
  - `pda.dlq`
- Verified producer delivery with aggregate-ID message keys and consumer-group delivery on the shared broker.
- Verified duplicate event delivery is suppressed by inbox idempotency.
- Verified unavailable-broker publication fails without false acknowledgement.
- Added an adapter boundary that fails closed with `ErrBrokerUnavailable`; it never reports an unacknowledged publish as successful and never silently falls back to mock delivery.
- Added outbox retry scheduling metadata, inbox idempotency, and durable DLQ tables.
- Added messaging delivery/backlog/lag metric counters.
- Added a PostgreSQL outbox/inbox adapter with `FOR UPDATE SKIP LOCKED`, attempt scheduling, publication marking, and inbox deduplication.
- Added a real `segmentio/kafka-go` producer with required acknowledgements, bounded attempts, batch timeout, JSON envelope serialization, and aggregate-ID keys.
- Added a consumer-group worker with inbox deduplication, bounded in-process retries, and durable DLQ handoff.
- Fixed live-consumer retry behavior so each failed message is retried immediately before commit, then sent to the DLQ at the bound.
- Added a broker-outage test proving the producer returns an error and never acknowledges an unavailable endpoint.
- Added explicit fail-closed handling for unconfigured SASL/TLS protocols; only the verified local PLAINTEXT profile is selectable.
- Wired published/failed delivery counters into the producer and consumer adapters.
- Added consumer backlog and event-lag gauges based on Kafka message timestamps.
- Wired gateway composition to select the real Kafka publisher when `PDA_MESSAGING_MODE=kafka`; mock mode remains the default and no silent fallback exists.
- Added an outbox worker that claims batches, publishes envelopes, marks successes, and schedules bounded retries on failures.
- Added the shared deferred verification register at `requirement-report/COMMON-DEFERRED-VERIFICATION.md`.
- Kept the verified in-memory publisher and explicit mock mode unchanged.

## Phase 8 recheck corrections

The 2026-08-01 recheck found and corrected two Kafka-mode wiring gaps:

- `integration-event-service` no longer fails generic startup when `PDA_MESSAGING_MODE=kafka`; it now starts a PostgreSQL-backed outbox worker and health endpoint in Kafka mode.
- Kafka topic resolution no longer double-prefixes already-qualified PDA topics such as `pda.task.events.v1`.

Additional recheck evidence:

- `make verify`: **PASS** before an unrelated untracked root prompt file named `GENERATE_PDA_APP_API_INTEGRATION_REQUIREMENTS_PROMPT.md` appeared in the backend worktree.
- Current full `make verify`: **BLOCKED** only by the repository-boundary architecture scan rejecting that untracked Android prompt file; Kafka tests, contract tests, command builds, and direct build still pass.
- `go clean -testcache && make test-kafka`: **PASS** against `127.0.0.1:19092`.
- Kafka-mode `integration-event-service` startup: **PASS**; service stayed running until the test timeout.
- Outbox worker E2E: inserted one pending `domain_outbox` row, ran `integration-event-service`, verified `published_at IS NOT NULL`, and confirmed the event was present on `pda.task.events.v1`.

## Local profile exit criteria

The local PLAINTEXT Phase 8 exit criteria are met: Kafka mode is externally exercised on the same broker used by MES, mock mode remains explicit, producer and consumer paths are real, event keys preserve aggregate affinity, duplicate processing is idempotent, and broker failure is fail-closed.

The following require a separate secured or failure-injection environment and are not represented as successful production claims:

- ACL authorization and TLS/SASL authentication;
- broker restart with durable outbox retry and DLQ assertions;
- ordering verification across a rebalance/retry scenario;
- exported production lag/backlog dashboard measurements;
- Testcontainers-based isolated broker coverage.

## Resume condition

For secured deployment, provide ACL/TLS credentials and rerun the deferred security and failure-injection checks before production approval. The backend is ready to proceed to BE-09 for the verified shared local profile.
