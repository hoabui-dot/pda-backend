# Backend Phase BE-08 — Kafka Delivery

Date: 2026-08-01  
Status: **BLOCKED (partial broker verification)**

## Verification

Re-verification completed after the previous checkpoint with the same result.

- `make clean verify`: **PASS** (format, vet, unit, integration, contract, architecture, and build checks).
- Migration 000006 `event_delivery`: **PASS**; down/up rollback test passed and schema version is `6|f`.
- PostgreSQL: accepting connections.
- Redis: healthy.
- Kafka: local Redpanda broker is now healthy on host port 19092; a topic was created and the Go producer round-trip test passed.

## Implemented safely

- Added optional Kafka configuration fields and documented security/group/topic settings.
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

## Exit criteria not claimable

ACL authorization/TLS, broker outage recovery with durable retry/DLQ assertions, ordering under retry, Testcontainers coverage, and real lag/backlog measurements remain unverified. Producer and consumer-group round trips plus fail-closed outage behavior are verified locally but are insufficient for phase approval.

## Resume condition

Provide a reachable Kafka environment with broker addresses, security/ACL credentials, and a test topic. Re-run BE-08 verification, then implement and verify the concrete producer/consumer adapter before approving the phase.
