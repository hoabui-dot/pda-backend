# Architecture boundary

This repository is the Go PDA backend. The existing Kotlin Android PDA application is an external client connected only through `/api/pda/v1` HTTPS/JSON contracts.

Deployable entry points live in `cmd`. Bounded contexts will live under `internal`; mutable domain entities remain owned by one context. `internal/platform` is limited to immutable technical primitives and bootstrap support. PostgreSQL is authoritative, Redis is non-authoritative, and Kafka remains disabled until BE-08.

## Hexagonal boundaries

Each bounded context is divided into `domain`, `application`, `ports`, and `adapters`. Domain code points inward only. Application code owns use-case orchestration. Port interfaces are owned by their consumers. Infrastructure is selected only in command composition code.

BE-00 provides the common actor/command/event contracts, outbox/inbox ports, deterministic fixture loading, an in-memory mock event publisher, and a fixture-backed mock WMS adapter. These adapters are test infrastructure for explicit mock mode; they are never an implicit production fallback.

## BE-01 public boundary

The gateway owns `/api/pda/v1`, correlation propagation, bounded request bodies, request timeouts, redacted request metadata logs, and auth rate limiting. Identity credentials are resolved from a deterministic fixture only in explicit mock mode. Mock tokens are signed, expiring, JWT-shaped local tokens; they are not production credentials. Application services rehydrate the operator from the trusted token, validate warehouse membership, and validate device registration before bootstrap context is returned.

Future OIDC validation implements the identity token-provider port. Production startup rejects mock auth, and the current composition fails explicitly when the OIDC adapter is selected before its integration phase.

## BE-02 task core

`warehouse_task` is the versioned task aggregate store. Claim and release run inside a PostgreSQL transaction that serializes the idempotency key with an advisory transaction lock, locks the task row, applies the aggregate transition, and inserts both idempotency result and outbox envelope before commit. Mock publication occurs after commit through `DomainEventPublisher`; an idempotent replay returns its stored result without producing another event.

Task reads are warehouse-scoped and expose only unassigned tasks or tasks assigned to the authenticated operator. Cursor pagination uses the stable task ID ordering. Dashboard and Task Center invalidation is represented by a port; the current adapter is a no-op because Redis is deferred to BE-07.

Deterministic mock-WMS task fixtures seed missing PostgreSQL rows in explicit mock mode. After seeding, PostgreSQL—not the fixture or Redis—is authoritative for task mutations.

## BE-03 receiving reference transaction

Receiving is a dedicated aggregate with explicit `NEW → IN_PROGRESS → PARTIALLY_COMPLETED → COMPLETED` transitions and version checks. Barcode resolution is restricted to an active, authorized document and distinguishes unknown barcodes from known barcodes in the wrong document.

Quantity confirmation locks the receiving task, enforces configured over-receipt and variance-remark policies, updates receiving lines and inventory balance, stores a durable command result, appends audit and outbox records, and commits them together. Denied commands are audited after rollback. The mock event publisher receives the committed production-shaped envelope; duplicate idempotency keys replay the durable result without a second inventory effect or event.

`GET /api/pda/v1/receiving/commands/{commandId}` exposes the operator/warehouse-scoped durable result for mobile offline retry reconciliation. Kafka remains disabled.

## BE-04 movement workflows

Putaway, picking, and replenishment are separate application services with explicit validation commands. They share inventory/location ports and PostgreSQL transaction infrastructure, but there is no workflow switch handler. Every step enforces warehouse and assignment scope plus an aggregate version. Putaway validates source and a capacity/compatibility-filtered destination before movement; picking validates its location and item barcode before pick/complete; replenishment validates source, destination, and item and remains partial until its required quantity reaches zero.

A movement command locks its aggregate and idempotency key, validates location stock/capacity, updates source and destination balances, records `inventory_movement`, persists task state, audit, outbox, and durable result, then commits before invalidation and mock publication. Outbox aggregate ID/version preserve per-task event ordering. Redis and Kafka remain disabled.

## BE-05 inventory control

Inventory inquiry reads authoritative PostgreSQL location balances and movement history and returns balance timestamps as freshness metadata. Bin Query and Item Query are query modes of the same inventory endpoints. Stock transfer locks and validates both locations and stock, moves source/destination balances, appends the ledger, audit, command result, and outbox atomically, and supports scoped idempotent replay.

Cycle count is a versioned task with lines, variance, recount, and completion states. Count variance is evidence only: submission never changes inventory. Cache invalidation is a port with a no-op adapter until BE-07; database reads remain the correctness path.

## BE-06 shipping

Shipment is an online-authoritative aggregate containing order, packages, carrier, tracking, readiness, and version. A direct application port projects picking completion into PostgreSQL readiness. Confirmation locks the shipment and idempotency key and requires picking complete, every package complete, an approved carrier, valid tracking, and the expected version.

The transaction persists shipment state, audit, durable command result, and the ordered `ShipmentReadinessChanged`, `ShipmentConfirmed`, and `OrderShipped` outbox envelopes before post-commit mock publication and projection invalidation. Label printing is not implemented.

## BE-07 Redis and resilience

Redis is a non-authoritative cache-aside adapter with versioned keys and configurable TTL. It caches dashboard, task-summary, inventory inquiry, shipment/readiness, and master lookup results. Singleflight protects cold keys; hit, miss, latency, and error counters describe behavior. Mutation invalidators clear related views after task, receiving, movement, inventory, and shipping events. The in-process stale marker is available only when both Redis and the database loader fail.

Redis uses bounded 100 ms I/O and no client retries. Cache get/set/delete failures degrade to PostgreSQL and never affect transactional idempotency, outbox, or command truth. Distributed Redis rate-limit support is available alongside the gateway limiter.

Outbound WMS adapters use explicit timeout, idempotent-only retry with jitter, `sony/gobreaker` closed/open/half-open behavior, and bounded bulkheads. Non-idempotent calls are never retried or reported as fallback success. The gateway adds a consecutive-failure circuit and retains request timeout and rate limiting.

## BE-08 conditional Kafka delivery

Kafka delivery is configuration-gated and fail-closed. `PDA_MESSAGING_MODE=mock` remains the default local application mode. Compose provides a single-node Redpanda Kafka-compatible broker on host port 19092 for development verification. Selecting `kafka` requires `PDA_KAFKA_BROKERS`, a stable group ID/topic prefix, and a reachable, ACL-authorized broker; startup does not silently fall back to mock delivery. The delivery migration adds retry scheduling, inbox idempotency, and a durable DLQ boundary. Consumer groups, ACL verification, broker-outage behavior, and lag measurements remain pending.
