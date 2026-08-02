# PDA Backend - PDA App Integration Gap Report

- Document status: Implemented audit baseline
- Generated date: 2026-08-01
- Backend repository: `pda-backend`
- Backend branch/commit: not recorded in the supplied workspace
- PDA requirement source: `GENERATE_PDA_APP_API_INTEGRATION_REQUIREMENTS_PROMPT.md`
- Generated `PDA_APP_API_INTEGRATION_REQUIREMENTS.md`: not present
- Runtime modes inspected: mock messaging, Kafka local PLAINTEXT, PostgreSQL, Redis
- Scope: public PDA gateway, application workflows, persistence, outbox/Kafka, OpenAPI, tests
- Android code modified: no
- Backend production code modified during this implementation: common error response envelope only

## Source-of-truth decision

The supplied PDA file is a generation prompt, not an app-generated contract. It defines the
required analysis structure and proposed workflow surface, but it does not provide authoritative
screen DTOs, field names, enum values, or endpoint choices. Existing backend source, migrations,
OpenAPI, tests, and phase reports are therefore authoritative for current support. Any app-specific
contract not evidenced by those sources is classified as `PROPOSED_ONLY` or `NOT_VERIFIED`.

## Executive summary

The backend has a broad, implemented `/api/pda/v1` surface covering authentication/bootstrap,
dashboard/task center, receiving, putaway, picking, replenishment, inventory inquiry and transfer,
cycle count, and shipping. The workflow services persist state, audit records, idempotency results,
optimistic versions, outbox events, and cache invalidation hooks.

Kafka Phase 8 is operationally verified in the local shared MES PLAINTEXT profile. The gateway can
select the real Kafka publisher, `integration-event-service` runs the PostgreSQL outbox worker, and
an inserted outbox record was published and marked delivered on `pda.task.events.v1`.

The principal confirmed API issue was an inconsistent error envelope. Success responses used
`data/meta/errors`, while failures returned `code/message` at the top level. This is now fixed in
`internal/gateway/adapters/http/router.go`.

Full PDA client readiness cannot be declared because the actual generated app contract and Android
source are not in this repository. Production Kafka ACL/TLS, real WMS, and OIDC remain external
verification items.

| Area | Current backend state | Status | Main gap |
|---|---|---|---|
| Authentication/bootstrap | Login, refresh, logout, profile, warehouses, device registration, bootstrap | SUPPORTED in mock mode | OIDC/production identity not verified |
| Dashboard/task center | Reads, filters, cursor pagination, claim/release | SUPPORTED | Exact Android DTO not confirmed |
| Receiving | List/detail/start/barcode/receipt/complete/command status | SUPPORTED by backend evidence | Exact PDA field contract not confirmed |
| Putaway | Full validated movement flow | SUPPORTED by backend evidence | Exact PDA field contract not confirmed |
| Picking | Full validated movement flow | SUPPORTED by backend evidence | Exact PDA field contract not confirmed |
| Replenishment | List/detail/validation/partial confirmation | SUPPORTED by backend evidence | Completion/API expectations need app contract |
| Inventory/transfer | Search, balances, movements, transfer validation/commit | PARTIALLY_SUPPORTED | Pagination and command-status contract need confirmation |
| Cycle count | List/detail/count/recount/complete | PARTIALLY_SUPPORTED | Command-status endpoint and variance contract need confirmation |
| Shipping | Summary/readiness/confirmation/command status | SUPPORTED by backend evidence | Label printing is not implemented |
| Common errors | Stable codes, HTTP mapping, correlation | SUPPORTED after this change | Timeout handler still has legacy raw JSON |
| Kafka | Shared broker, producer, consumer, outbox worker, inbox/DLQ | SUPPORTED for local PLAINTEXT | ACL/TLS/rebalance failure injection deferred |

## Current public API inventory

All routes below are registered in `internal/gateway/adapters/http/router.go` and implemented by
handlers and application services. The OpenAPI source is `api/openapi/pda-v1.yaml`.

| Area | Routes |
|---|---|
| Identity | `POST /auth/login`, `POST /auth/refresh`, `POST /auth/logout`, `GET /me`, `GET /me/warehouses` |
| Device/context | `POST /devices/registrations`, `GET /bootstrap` |
| Tasks | `GET /dashboard`, `GET /tasks/summary`, `GET /tasks`, `POST /tasks/{taskId}/claim`, `POST /tasks/{taskId}/release` |
| Receiving | list, detail, start, barcode resolution, receipt, completion, command status |
| Putaway | list, detail, source validation, destination suggestions, destination validation, confirmation |
| Picking | list, detail, location validation, barcode resolution, pick, completion |
| Replenishment | list, detail, source/destination/item validation, confirmation |
| Inventory | search, balances, movements, transfer source/destination/item validation, transfer commit |
| Cycle count | list, detail, count, recount, completion |
| Shipping | summary, readiness, confirmation, command status |
| Operational | `GET /healthz`, `GET /livez`, `GET /readyz` |

## Requirement-to-backend matrix

| Required capability | Backend evidence | Status |
|---|---|---|
| Login and session lifecycle | identity application service, gateway auth middleware, gateway tests | SUPPORTED in mock mode |
| Trusted operator/device/warehouse context | `deviceWarehouseContext`, identity repositories, bootstrap route | SUPPORTED |
| Scanner validation | receiving barcode and movement validation handlers/services | SUPPORTED by backend evidence |
| Quantity confirmation | receiving, movement, inventory application services | SUPPORTED |
| Task claim/release and locking | task service, PostgreSQL row locks, version checks | SUPPORTED |
| Idempotent mutations | command status tables and transactional command saves | SUPPORTED for implemented mutations |
| Optimistic concurrency | `If-Match` parsing and aggregate version checks | SUPPORTED for implemented mutations |
| Durable command lookup | receiving and shipping public endpoints | PARTIALLY_SUPPORTED for other domains |
| Cursor pagination | task and receiving pages; inventory movement cursor | PARTIALLY_SUPPORTED across all lists |
| Cache/freshness | Redis adapters and invalidation ports | PARTIALLY_SUPPORTED; response freshness is limited to server time |
| Offline retry reconciliation | durable command status for receiving/shipping | PARTIALLY_SUPPORTED |
| Common response envelope | success and, after this change, error responses | SUPPORTED |
| Kafka outbox delivery | Kafka adapter, worker, PostgreSQL outbox/inbox/DLQ | SUPPORTED local PLAINTEXT |
| Real WMS | WMS port plus deterministic mock adapter | BLOCKED_BY_EXTERNAL_DEPENDENCY |
| OIDC/production auth | explicit mock token provider only | BLOCKED_BY_EXTERNAL_DEPENDENCY |

## Confirmed contract comparison

### Headers

The gateway generates or validates `X-Correlation-Id`, requires `Authorization` for protected
routes, and requires `X-Device-Id` and `X-Warehouse-Id` for warehouse-scoped routes. Mutations
require UUID `Idempotency-Key`; versioned mutations require numeric `If-Match`. Correlation is
propagated into actor context and event envelopes. `Accept-Language` is not currently interpreted.

### Success envelope

```json
{
  "data": {},
  "meta": {
    "serverTime": "2026-08-01T10:00:00Z",
    "correlationId": "uuid"
  },
  "errors": []
}
```

### Error envelope

All normal gateway errors now use the same envelope:

```json
{
  "data": null,
  "meta": {
    "serverTime": "2026-08-01T10:00:00Z",
    "correlationId": "uuid"
  },
  "errors": [
    {
      "code": "TASK_VERSION_CONFLICT",
      "message": "Task version conflict",
      "details": {},
      "retryable": false
    }
  ]
}
```

### Stable errors

The implementation maps authentication, warehouse/device scope, rate limiting, circuit state,
not-found, conflict, workflow validation, stock/capacity, readiness, and idempotency errors to
stable machine-readable codes. The explicitly required codes `AUTH_INVALID_CREDENTIALS`,
`AUTH_SESSION_EXPIRED`, `DEVICE_NOT_REGISTERED`, `WAREHOUSE_ACCESS_DENIED`, `TASK_LOCKED`,
`TASK_VERSION_CONFLICT`, `BARCODE_UNKNOWN`, `BARCODE_WRONG_CONTEXT`, `INSUFFICIENT_STOCK`,
`LOCATION_CAPACITY_EXCEEDED`, `SHIPMENT_NOT_READY`, `RATE_LIMITED`, and `INTERNAL_ERROR` are
represented or mapped in the gateway/application layers. `COUNT_VARIANCE_REQUIRES_REVIEW` and a
generic cross-domain command-status error contract are not confirmed.

## Workflow and persistence traceability

For implemented mutations the normal path is:

```text
gateway route
-> authentication/device/warehouse context
-> application service
-> aggregate validation and version check
-> PostgreSQL transaction and row lock
-> state, audit, command result, and outbox event
-> commit
-> invalidation and publisher/worker
-> common response envelope
```

Kafka events use versioned topics `pda.task.events.v1`, `pda.receiving.events.v1`,
`pda.movement.events.v1`, `pda.inventory.events.v1`, and `pda.shipping.events.v1`. The outbox
worker uses `FOR UPDATE SKIP LOCKED`, bounded retry scheduling, publication marking, inbox
deduplication, and durable DLQ handoff.

## Missing or unverified capabilities

| Gap | Status | Priority | Required action |
|---|---|---:|---|
| Generated `PDA_APP_API_INTEGRATION_REQUIREMENTS.md` is absent | NOT_VERIFIED | P0 | Provide/generate it from the Android source before freezing DTOs and enums |
| Android source/screens were not available | NOT_VERIFIED | P0 | Map every ViewModel/repository/scanner action to the implemented routes |
| Generic command-status lookup for movement/inventory/count is not public | PARTIALLY_SUPPORTED | P1 | Confirm app retry behavior, then expose scoped status reads or document online-only behavior |
| List response pagination/filter contract varies by domain | PARTIALLY_SUPPORTED | P1 | Normalize cursor, limit, sort, and freshness metadata in OpenAPI |
| Error response schema was absent/inconsistent in OpenAPI | PARTIALLY_SUPPORTED | P1 | Add explicit reusable success/error envelope schemas and response references |
| Timeout response uses legacy raw JSON | PARTIALLY_SUPPORTED | P1 | Route timeout output through the common envelope with correlation metadata |
| `Accept-Language` localization behavior is absent | NOT_SUPPORTED | P2 | Confirm PDA language requirement and add localized message policy if required |
| Redis projection invalidation is largely hook-based | PARTIALLY_SUPPORTED | P2 | Verify concrete cache keys and post-mutation refresh against PDA freshness needs |
| OIDC, real WMS, Kafka ACL/TLS | BLOCKED_BY_EXTERNAL_DEPENDENCY | P0/P1 | Provide secured external environments and rerun deferred verification |
| Shipping label printing | NOT_SUPPORTED | P3 | Implement only if the app contract requires it |

## Scanner audit

Receiving barcode resolution is task-scoped and validates the barcode against the receiving task.
Movement workflows expose source, destination, location, and barcode validation operations. Actor
context is authoritative from the bearer token and required headers, not from client-supplied
operator identity. Symbology and scanner-source fields are not currently part of the backend
request contract; this is `NOT_VERIFIED` until the generated app document confirms they are sent.

## Idempotency, versioning, offline, and cache audit

Task, receiving, movement, inventory, count, and shipping mutations persist idempotency truth in
PostgreSQL and use aggregate versions where the domain supports them. Receiving and shipping expose
durable command-status reads. Other domains persist command results but do not currently expose a
matching public status route. Redis is a backend cache and is never a PDA data source; response
metadata currently provides server time and correlation ID, but not a domain-specific freshness
timestamp or stale marker.

The backend therefore supports online retry safety for implemented mutations. Full offline queue
behavior, unknown-success reconciliation for every workflow, and conflict UI behavior remain
`NOT_VERIFIED` without the actual PDA contract.

## Test and runtime evidence

- `go test ./...`: application and integration packages pass; architecture test is blocked by the
  user-supplied untracked Android prompt file being rejected by the repository-boundary guard.
- `go test ./internal/integration/adapters/kafka`: PASS.
- `make test-kafka`: PASS against `127.0.0.1:19092`.
- Kafka-mode `integration-event-service`: stayed running and served its health endpoint.
- Outbox E2E: pending PostgreSQL outbox record became published and was observed on
  `pda.task.events.v1`.
- `make build`: PASS.
- Contract tests and command builds: PASS.
- Kafka ACL/TLS, broker restart/rebalance failure injection, real WMS, OIDC, and physical Zebra:
  deferred/not verified.

## Implementation backlog

1. Obtain the generated PDA API requirements document and Android source evidence; reconcile exact
   paths, DTOs, enums, headers, scanner payloads, pagination, and error UX.
2. Add reusable OpenAPI schemas for `Envelope`, `Meta`, and `Error`, and document every route's
   response and header contract.
3. Normalize command-status behavior across all offline-retry-capable mutations.
4. Normalize list pagination and freshness metadata across receiving, movement, inventory, count,
   and shipment reads.
5. Verify Redis invalidation keys and stale-data behavior with the PDA refresh model.
6. Run secured Kafka, WMS, OIDC, and physical-device verification before production approval.

## Final decision

Backend workflow implementation is ready for contract reconciliation and local integration testing.
It is not yet safe to declare full PDA integration complete because the actual generated PDA
requirements document is missing, production identity/WMS dependencies are unavailable, and secure
Kafka verification remains deferred.

