# PDA WMS Backend — Enterprise Architecture and Execution Strategy

> **Repository boundary:** This document describes a brand-new backend repository. The existing Kotlin Android `PDA_APP` is an external client and must not be recreated or modified by backend phases.

- **Document ID:** `PDA-BE-ARCH-001`
- **Version:** 1.0
- **Date:** 2026-08-01
- **Purpose:** define a new enterprise backend codebase that supports the approved PDA application screens and workflows.
- **Current integration state:** PDA APIs, Kafka, and production WMS database are not yet available.
- **Initial runtime mode:** PostgreSQL + Redis + synchronous REST APIs + deterministic mock messaging/data adapters.
- **Target runtime mode:** secured microservices with API Gateway, domain services, Redis cache, Kafka producers/consumers, transactional outbox, observability, and WMS integration.

---

## 1. Architecture Decision Summary

The backend should be built as a **modular microservice platform**, but it must avoid premature service fragmentation.

Recommended initial deployment units:

1. `pda-api-gateway`
2. `identity-access-service`
3. `pda-execution-service`
4. `inventory-service`
5. `outbound-shipping-service`
6. `integration-event-service`
7. `audit-observability-service`

The `pda-execution-service` initially owns task, receiving, putaway, picking, replenishment, transfer, and cycle-count orchestration. Once traffic, ownership, or team boundaries justify it, the service can be split by bounded context without changing the public PDA API.

### Why this shape

- The mobile API must remain stable while internal services evolve.
- Warehouse transactions require strong database consistency.
- Redis is a cache and coordination aid, not the inventory source of truth.
- Kafka is used for integration and asynchronous propagation, not as the only source of truth for user-facing commands.
- The API must return an authoritative command result to the PDA before emitting downstream events.
- Kafka unavailability must not corrupt or silently discard warehouse mutations.

---

## 2. Recommended Technology Baseline

Use Go modules and pin compatible versions in `go.mod`/`go.sum` at repository creation time.

Recommended stack:

| Area | Recommended technology |
|---|---|
| Language | Go 1.24+ |
| HTTP framework | Go standard `net/http` with `chi` router |
| Gateway | Go reverse proxy/BFF using `net/http`, `chi`, and explicit middleware |
| Security | OIDC/OAuth2 JWT validation using `golang.org/x/oauth2` and `lestrrat-go/jwx` |
| Database | PostgreSQL |
| Data access | `pgx/v5` and explicit SQL; `sqlc` for compile-time checked query bindings |
| Schema migration | `golang-migrate/migrate` |
| Cache | Redis |
| Messaging | Apache Kafka |
| Resilience | Explicit timeout/retry/bulkhead adapters; `sony/gobreaker` for circuit breaking |
| API description | OpenAPI 3 |
| Observability | OpenTelemetry Go + Prometheus client |
| Testing | Go `testing`, `testify`, Testcontainers for Go, and `httptest` |
| Build | Go modules + Makefile |
| Deployment | Docker; Kubernetes-ready manifests later |

Do not use Kafka transactions as a substitute for the database transaction. Use the **transactional outbox pattern** for database mutation plus reliable event publication.

Official references:

- Go documentation: https://go.dev/doc/
- chi router: https://github.com/go-chi/chi
- pgx: https://github.com/jackc/pgx
- Apache Kafka documentation: https://kafka.apache.org/documentation/
- Kafka producer configuration: https://kafka.apache.org/documentation/#producerconfigs
- Redis documentation: https://redis.io/docs/latest/
- OpenTelemetry Go: https://opentelemetry.io/docs/languages/go/

---

## 3. System Context

```text
Zebra PDA Android App
        |
        | HTTPS / JSON / OAuth2
        v
PDA API Gateway
        |
        +--> Identity & Access Service
        |
        +--> PDA Execution Service
        |       |
        |       +--> PostgreSQL
        |       +--> Redis
        |       +--> Outbox table
        |
        +--> Inventory Service
        |       |
        |       +--> PostgreSQL
        |       +--> Redis
        |
        +--> Outbound & Shipping Service
                |
                +--> PostgreSQL
                +--> Redis

Outbox Publishers
        |
        v
Kafka (target mode)
        |
        +--> WMS integration consumer
        +--> Audit consumer
        +--> Reporting consumer
        +--> Notification consumer

Initial development mode:
Outbox/EventPort --> MockEventPublisher --> hardcoded mock event sink/file
```

---

## 4. Runtime Modes

The backend must support explicit runtime modes.

```yaml
pda:
  messaging:
    mode: mock # mock | kafka
  upstream-wms:
    mode: mock # mock | http
  auth:
    mode: mock # mock | oidc
```

### 4.1 Initial development mode

```text
messaging.mode=mock
upstream-wms.mode=mock
auth.mode=mock
```

Behavior:

- REST APIs are fully functional.
- PostgreSQL remains the backend source of truth.
- Redis caching is active where appropriate.
- Kafka producer/consumer adapters are not constructed.
- `MockDomainEventPublisher` receives the same event envelopes that Kafka would receive.
- Mock WMS data comes from deterministic JSON/YAML fixtures.
- Events are stored in a mock event log table/file for test assertions.
- No business service imports or directly calls mock fixture packages.

### 4.2 Kafka target mode

```text
messaging.mode=kafka
```

Behavior:

- `KafkaDomainEventPublisher` is selected through configuration.
- Outbox rows are published to Kafka.
- Consumer groups process integration events idempotently.
- Mock publisher is disabled.
- Topic ACLs, schema compatibility, retry, dead-letter handling, and observability are enabled.

### 4.3 Important implementation rule

Prefer conditional configuration over permanently commenting code:

```go
switch cfg.Messaging.Mode {
case "mock":
    publisher = messaging.NewMockDomainEventPublisher(eventLog)
case "kafka":
    publisher = messaging.NewKafkaDomainEventPublisher(kafkaClient)
default:
    return fmt.Errorf("unsupported messaging mode %q", cfg.Messaging.Mode)
}
```

If the repository must initially contain commented Kafka wiring, keep it only in the composition/configuration layer:

```go
// TODO PHASE-BE-08: construct the Kafka adapter after broker readiness.
// publisher = messaging.NewKafkaDomainEventPublisher(kafkaClient)
```

Domain and application code must depend on `DomainEventPublisher`, never on Kafka client types.

---

## 5. Bounded Contexts and Service Responsibilities

## 5.1 PDA API Gateway

Responsibilities:

- external PDA route ownership;
- OAuth2 token validation;
- device and warehouse headers;
- correlation ID;
- request size limits;
- rate limiting;
- route-level circuit breaker;
- safe retry only for idempotent reads;
- response normalization;
- API version routing;
- no warehouse business logic.

Do not retry POST commands automatically at the gateway unless the endpoint contract explicitly supports idempotency and the implementation preserves the same idempotency key.

## 5.2 Identity and Access Service

Responsibilities:

- mock login in early phases;
- future OIDC federation;
- operator profile;
- role and permission resolution;
- warehouse/site assignment;
- device registration and revocation;
- session/token policy;
- authorization decision inputs.

Roles may include:

- receiving operator;
- picker;
- putaway operator;
- inventory controller;
- shipping operator;
- supervisor.

Authorization must be enforced in services, not only hidden in the PDA UI.

## 5.3 PDA Execution Service

Responsibilities:

- dashboard projection;
- Task Center summary;
- task assignment/claim/lock;
- receiving workflow;
- putaway workflow;
- picking workflow;
- replenishment workflow;
- stock-transfer command orchestration;
- cycle-count workflow;
- command idempotency;
- task state machine;
- transactional outbox writes.

## 5.4 Inventory Service

Responsibilities:

- item/location master projections;
- stock balances;
- availability/reservation calculations;
- movement ledger;
- inventory inquiry;
- source/destination validation;
- count variance projection;
- authoritative movement transaction where service boundaries require it.

For the initial codebase, inventory tables may live in the execution service database while exposed behind an `InventoryPort`. Split physically only when the transaction boundary is well understood.

## 5.5 Outbound and Shipping Service

Responsibilities:

- shipment summary;
- package readiness;
- carrier/tracking validation;
- shipment confirmation;
- shipment idempotency;
- downstream shipping event.

Packing remains a prerequisite projection unless a dedicated PDA packing screen is approved.

## 5.6 Integration Event Service

Responsibilities:

- outbox polling/publication;
- Kafka producer adapter;
- mock producer adapter;
- inbound WMS event consumers;
- schema/version validation;
- idempotent consumer inbox;
- dead-letter processing;
- replay tooling.

## 5.7 Audit and Observability

Responsibilities:

- immutable audit events;
- API correlation;
- operator/device/warehouse attribution;
- command/event traceability;
- OpenTelemetry traces, metrics, and logs;
- security and business audit export.

---

## 6. Enterprise Data Ownership

| Data | Authoritative owner |
|---|---|
| User identity/token | Identity provider / Identity service |
| PDA device registration | Identity service |
| Warehouse task state | PDA Execution service or upstream WMS, based on approved contract |
| Inventory balance | Inventory/WMS authoritative database |
| Local Redis cache | Never authoritative |
| Outbox | Service database that commits the business mutation |
| Kafka event | Integration fact after committed business transaction |
| PDA local Room data | Mobile cache/offline source, not enterprise authority |
| Audit event | Backend audit store |

When the upstream WMS is authoritative, the backend acts as an anti-corruption layer and synchronization boundary. The PDA must never call the WMS database directly.

---

## 7. API Contract Principles

### Required headers

```http
Authorization: Bearer <token>
X-Correlation-Id: <uuid>
X-Device-Id: <registered-device-id>
X-Warehouse-Id: <warehouse-id>
Idempotency-Key: <uuid>          # write commands
If-Match: "<entity-version>"     # versioned writes where applicable
Accept-Language: vi-VN
```

### Response envelope

```json
{
  "data": {},
  "meta": {
    "serverTime": "2026-08-01T04:00:00Z",
    "correlationId": "uuid",
    "version": "12",
    "nextCursor": null
  },
  "errors": []
}
```

### Error envelope

```json
{
  "code": "TASK_VERSION_CONFLICT",
  "message": "Localized fallback or safe operator message",
  "details": {
    "taskId": "REC-001"
  },
  "correlationId": "uuid",
  "retryable": false
}
```

### Command processing order

```text
Authenticate
→ Authorize
→ Validate device/warehouse
→ Validate idempotency key
→ Load aggregate with lock/version
→ Validate command
→ Commit business mutation + outbox
→ Update/evict cache
→ Return authoritative response
→ Publish event asynchronously
```

---

## 8. API Mapping to the Approved PDA Screens

| Screen | Backend capability |
|---|---|
| SCR-01 Login | mock login, then OIDC/device bootstrap |
| SCR-02 Dashboard | aggregated counts, task progress, unread/alert projection, server sync time |
| SCR-03 Task Center | category counts by status and operator |
| SCR-04 Inbound Receiving List | paginated/filterable receiving tasks |
| SCR-05 Receiving Detail | PO/task metadata and lines |
| SCR-06 Receiving Scan Item | barcode resolution in active document context |
| SCR-07 Confirm Quantity | idempotent receipt command |
| SCR-08 Putaway List | paginated task list |
| SCR-09 Putaway Execution | source/destination validation and confirm command |
| SCR-10 Picking List | paginated operator tasks |
| SCR-11 Picking Detail | current line, location/item validation, pick command |
| SCR-12 Replenishment | task list/detail and movement command |
| SCR-13 Inventory Inquiry | freshness-sensitive item/location balances and history |
| SCR-14 Stock Transfer | source/destination/item validation and transfer command |
| SCR-15 Cycle Count | task detail, count submit, variance/recount state |
| SCR-16 Shipping Confirmation | readiness read and idempotent shipment command |

Detailed endpoints are in `02_API_AND_EVENT_CONTRACT_MAP.md`.

---

## 9. Redis Caching Strategy

Redis is a shared backend cache. It is separate from the Android Room/local cache.

### Cache-aside candidates

- dashboard summary;
- Task Center category counts;
- operator/warehouse permission projection;
- item master lookup;
- location master lookup;
- task list query result;
- inventory inquiry result with very short TTL;
- shipment readiness projection with very short TTL.

### Do not cache as authoritative state

- pending commands;
- current task lock without a database ownership record;
- inventory mutation result before commit;
- shipment confirmation result before commit;
- idempotency result only in Redis.

### Example keys

```text
pda:v1:operator:{operatorId}:profile
pda:v1:warehouse:{warehouseId}:dashboard:{operatorId}
pda:v1:warehouse:{warehouseId}:task-summary:{operatorId}:{status}
pda:v1:item:{warehouseId}:{barcode}
pda:v1:location:{warehouseId}:{locationCode}
pda:v1:inventory:{warehouseId}:{itemId}:{locationId}
pda:v1:shipment:{shipmentId}:readiness
```

### TTL guidance

| Cache | Suggested starting TTL |
|---|---:|
| operator/permissions | 5–15 minutes |
| item/location master | 30–120 minutes |
| dashboard/task summary | 15–60 seconds |
| task list page | 15–30 seconds |
| inventory inquiry | 5–15 seconds |
| shipment readiness | 3–10 seconds |

TTL values are configuration, not hardcoded constants.

### Invalidation

- commit database transaction first;
- evict or update related keys;
- emit domain event;
- consumers may invalidate projections;
- stale cache must never authorize a mutation.

Avoid Redis distributed locks for core inventory correctness when a PostgreSQL row/advisory lock or aggregate version can provide the real transaction boundary.

---

## 10. Messaging and Kafka Architecture

### Event envelope

```json
{
  "eventId": "uuid",
  "eventType": "ReceivingQuantityConfirmed",
  "eventVersion": 1,
  "aggregateType": "ReceivingTask",
  "aggregateId": "REC-001",
  "aggregateVersion": 12,
  "occurredAt": "2026-08-01T04:00:00Z",
  "correlationId": "uuid",
  "causationId": "command-uuid",
  "warehouseId": "WH-01",
  "operatorId": "OP-01",
  "deviceId": "TC26-001",
  "payload": {}
}
```

### Topic strategy

```text
pda.task.events.v1
pda.receiving.events.v1
pda.inventory.events.v1
pda.outbound.events.v1
pda.audit.events.v1
wms.pda.commands.v1       # only if upstream command/event contract approves it
wms.masterdata.events.v1
```

Prefer a small number of domain topics with stable event types over one topic per event.

### Producer rules

- publish only committed outbox records;
- use stable message key, generally aggregate ID;
- enable producer idempotence in Kafka mode;
- preserve event order per aggregate;
- include schema/event version;
- observe publish latency and failure;
- never block the PDA request waiting for a non-critical consumer.

### Consumer rules

- maintain an inbox/processed-event table;
- consume at least once and process idempotently;
- commit offset only after successful processing;
- retry transient errors with bounded backoff;
- route poison events to a dead-letter topic;
- preserve original event/correlation metadata;
- do not hide permanent schema/business failures.

### Mock messaging mode

`MockDomainEventPublisher` must:

- implement the same `DomainEventPublisher` interface;
- accept real event envelopes;
- write events to an in-memory sink, JSONL file, or database table;
- support deterministic failure fixtures;
- allow test assertions;
- never alter business service behavior.

Hardcoded event fixture location:

```text
services/integration-event-service/src/main/resources/mock/events/
```

Hardcoded WMS response fixture location:

```text
services/integration-event-service/src/main/resources/mock/wms/
```

---

## 11. Business and State Manager Pattern

The “Business & State Manager” should not become one global god service. Implement feature-specific application services and aggregate state machines.

```text
ReceivingApplicationService
  → ReceivingTask aggregate
  → ReceivingPolicy
  → InventoryPort
  → ReceivingRepository
  → OutboxRepository

PutawayApplicationService
  → PutawayTask aggregate
  → LocationPolicy
  → InventoryPort
  → PutawayRepository
  → OutboxRepository
```

### Aggregate rules

- one aggregate protects one consistency boundary;
- state transitions are explicit;
- invalid transitions return typed errors;
- aggregate version increments on successful writes;
- the application service controls transaction boundaries;
- controllers only map transport to commands/results.

### State machine example

```text
NEW
→ ASSIGNED
→ IN_PROGRESS
→ PARTIALLY_COMPLETED
→ COMPLETED

Any active state
→ ON_HOLD

Only approved transitions
→ CANCELLED / FAILED
```

Each workflow may have a more specific internal step state in addition to the task status.

---

## 12. Code Architecture and Patterns

Use ports and adapters / hexagonal structure inside each service.

```text
service/
  api/
    rest/
    dto/
    mapper/
  application/
    command/
    query/
    handler/
    port/in/
    port/out/
  domain/
    model/
    policy/
    event/
    error/
  infrastructure/
    persistence/
    redis/
    messaging/
    security/
    client/
    observability/
  configuration/
```

Required patterns:

- domain-driven bounded contexts;
- command/query separation without unnecessary framework-heavy CQRS;
- repository port;
- anti-corruption layer for upstream WMS;
- transactional outbox;
- idempotent consumer/inbox;
- cache-aside;
- bulkhead;
- circuit breaker;
- timeout;
- retry with jitter only for safe operations;
- API composition at BFF/gateway/application query layer;
- saga/process manager only for genuinely multi-service long-running workflows.

Avoid:

- shared database tables across independently deployed services;
- distributed two-phase commit;
- Redis as the inventory database;
- Kafka as synchronous request-response;
- automatic retries for non-idempotent commands;
- one universal workflow service with switch statements for all domains;
- common library that contains mutable domain entities shared by all services.

---

## 13. Resilience Policy

### Timeouts

Set explicit connect/read/request timeouts for every outbound call. No unbounded waits.

### Circuit breaker

Apply to:

- upstream WMS reads;
- identity/profile dependencies;
- carrier/readiness provider;
- non-critical downstream service queries.

Do not use a circuit breaker to convert a failed inventory mutation into success.

### Retry

Retry:

- idempotent GET;
- outbox publication;
- idempotent consumer handling;
- explicitly idempotent POST with preserved key.

Do not retry:

- validation failures;
- authorization failures;
- stale-version conflicts;
- non-idempotent commands without a stable key.

### Bulkheads

Separate thread/connection pools for:

- gateway traffic;
- WMS integration;
- Kafka consumers;
- outbox publishers;
- scheduled synchronization.

### Fallback

Allowed fallbacks:

- cached dashboard/task summary with stale metadata;
- mock WMS adapter only in non-production configured mode;
- read-only degradation for inventory inquiry.

Forbidden fallback:

- fabricating a successful receipt, movement, count, or shipment;
- silently switching production Kafka mode to mock mode;
- returning stale stock as authoritative mutation validation.

---

## 14. Security

- OAuth2/OIDC bearer validation at gateway and services.
- Service-to-service authentication using mTLS or approved OAuth2 client credentials/private key JWT.
- RBAC/ABAC checks in application services.
- Registered device ID and warehouse scope.
- TLS everywhere.
- Secrets through secret manager/Kubernetes Secrets, not source control.
- Audit all mutations and authorization denials.
- Rate limit login and high-cost query endpoints.
- Do not log tokens, passwords, full sensitive barcodes, or personal data.
- Use database least-privilege accounts per service.
- Kafka ACL per producer/consumer group/topic.

---

## 15. Observability and SLO Readiness

Every request/command/event includes:

- trace ID;
- correlation ID;
- command/event ID;
- operator;
- device;
- warehouse;
- aggregate ID/version;
- outcome/error code.

Recommended metrics:

- API latency/error rate;
- command conflict/idempotency hit;
- task operation duration;
- Redis hit/miss;
- database lock/conflict;
- outbox backlog age/count;
- Kafka publish failure;
- consumer lag/retry/DLQ;
- WMS dependency latency;
- authentication failure;
- scanner-originated invalid barcode rate.

---

## 16. Deployment Topology

Initial local development:

```text
Docker Compose:
- gateway
- identity-service
- execution-service
- inventory-service
- shipping-service
- integration-event-service
- PostgreSQL instances/databases
- Redis
- Kafka disabled by default
- optional Kafka profile
- OpenTelemetry Collector
```

Production target:

- Kubernetes namespaces per environment;
- one database/schema owner per service;
- Redis HA;
- Kafka cluster;
- ingress/API gateway;
- secret manager;
- autoscaling based on latency/CPU/consumer lag;
- PodDisruptionBudgets;
- readiness/liveness/startup probes;
- network policies.

---

## 17. Repository Bootstrap Sequence

The backend repository does not exist before the bootstrap phase.

```text
PRE-00
Create new repository
→ initialize Go workspace and modules
→ create backend modules
→ add Docker Compose
→ add initial application code
→ run first backend build

BE-00
Add architecture guardrails, domain contracts, mock adapters, and architecture tests
```

There is no backend baseline build before PRE-00. Android build verification is outside this repository.

## 18. Implementation Phase Summary

| Phase | Outcome |
|---:|---|
| PRE-00 | create the new backend repository, Go workspace/modules, services, and first successful backend build |
| BE-00 | architecture guardrails, domain contracts, mock adapters, architecture tests |
| BE-01 | gateway, mock auth, device/warehouse context |
| BE-02 | core execution data model, tasks, dashboard, Task Center |
| BE-03 | receiving APIs and event/outbox reference flow |
| BE-04 | putaway, picking, replenishment |
| BE-05 | inventory inquiry, transfer, cycle count |
| BE-06 | shipping confirmation and cross-domain projections |
| BE-07 | Redis caching and resilience hardening |
| BE-08 | Kafka enablement, consumers, inbox/DLQ |
| BE-09 | real WMS integration adapter |
| BE-10 | security, observability, performance, E2E and production readiness |

One English implementation prompt is provided for PRE-00 and every backend phase.
