# PDA Backend WMS and OIDC Integration Requirements

**Document status:** Integration handoff

**Applies to:** PDA API integration Phases 00-11, API-001 through API-028

**Repository boundary:**

- Existing PDA Android repository: external client, not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

This document defines the information and environment required to connect the PDA backend to the real WMS and an external OIDC provider. It is the setup contract for the WMS/OIDC integration work; it does not replace the frozen PDA API specification or the backend OpenAPI document.

## 1. Required Setup Package

Provide one non-production staging package containing:

| Area | Required input |
|---|---|
| OIDC | issuer URL, discovery URL, JWKS URL or discovery support, audience, client ID if applicable, scopes, claim mapping, token lifetimes, revocation/introspection behavior |
| WMS | base URL, API version, OpenAPI or endpoint specification, staging tenant/warehouse IDs, service-account authentication, timeout and rate limits |
| Events | Kafka bootstrap servers, TLS CA, optional client certificate/key, server name, topic names, consumer groups, ACL matrix, DLQ policy |
| Database | staging PostgreSQL URL, migration ownership, schema ownership, timezone, backup/restore contact |
| Cache | staging Redis URL, TLS/auth requirements, database/index, eviction policy, availability expectations |
| Test data | disposable operators, devices, warehouses, items, locations, receiving orders, putaway tasks, pick orders, replenishment tasks, count tasks, shipments |
| Operations | correlation-ID tracing destination, metrics sink, alert destination, on-call owner, incident escalation path |

Do not send production passwords, private keys, refresh tokens, or bearer tokens in this document or repository. Deliver secrets through the deployment secret manager.

## 2. OIDC Provider Requirements

### 2.1 Protocol

The provider must support OpenID Connect Discovery and signed JWT access tokens using a supported asymmetric signing algorithm. The backend must validate the token locally using the provider JWKS and must not trust PDA-supplied identity headers.

Required validation:

- `iss` exactly matches the configured issuer;
- `aud` contains the configured PDA backend audience;
- `exp` is in the future with a small configured clock-skew allowance;
- `nbf` is valid when present;
- signature algorithm is an explicitly allowlisted asymmetric algorithm;
- signing key exists in the current JWKS;
- token type and required scopes are valid;
- revoked sessions/tokens are rejected according to the provider’s revocation or introspection policy.

The backend must reject unsigned tokens, unsupported algorithms, wrong issuer, wrong audience, expired tokens, unknown keys, and malformed claims with a safe `AUTH_SESSION_EXPIRED` or `AUTH_INVALID_TOKEN` error. It must not fall back to mock authentication.

### 2.2 Required claims

The provider must supply or map these values:

| Logical value | Preferred claim | Requirement |
|---|---|---|
| Operator identity | `sub` | Stable, immutable operator identifier |
| Username/employee code | `preferred_username`, `employee_code`, or agreed claim | Must map to an active backend operator |
| Display name | `name` | Optional display value; backend may use directory data |
| Roles | `roles` or agreed namespaced claim | Array of role identifiers |
| Permissions/scopes | `scope`, `scp`, or agreed namespaced claim | Must map to backend permissions |
| Warehouse scope | `warehouse_ids` or agreed namespaced claim | Allowed warehouse identifiers |
| Device scope | `device_ids` or agreed namespaced claim | Required when device binding is enforced |
| Session/token ID | `jti` | Required for revocation and audit when supported |
| Issued time | `iat` | Required for audit and replay analysis |
| Expiry | `exp` | Required |

The exact claim names, namespace, value types, and examples must be supplied before enabling OIDC. A claim containing a warehouse or device value must be treated as an allowlist, not as a default context.

### 2.3 Login, refresh, logout, and revocation

The PDA-facing API remains:

- `POST /api/pda/v1/auth/login` (`API-001`)
- `POST /api/pda/v1/auth/refresh` (`API-002`)
- `POST /api/pda/v1/auth/logout` (`API-003`)
- `GET /api/pda/v1/bootstrap` (`API-004`)
- `GET /api/pda/v1/me` (`API-006`)

The integration owner must specify whether the PDA uses Authorization Code with PKCE, device authorization, or another approved flow. The backend must not accept a PDA password and must not log credentials.

Required behavior:

1. Access tokens are short-lived and stored only in the approved PDA secure storage.
2. Refresh tokens are rotated when the provider supports rotation; reuse detection revokes the session.
3. A PDA request receiving one `401` may refresh once and retry once with the same idempotency key, then must clear the session on refresh failure.
4. Logout revokes the provider session/token where supported and clears backend session state.
5. Token refresh must preserve the selected warehouse/device authorization checks.
6. `X-Operator-Id` is an audit consistency check only; token `sub` is authoritative.
7. `X-Warehouse-Id` and `X-Device-Id` must be checked against token claims and backend registrations on every protected workflow request.

### 2.4 OIDC acceptance tests

Provide evidence for:

- valid token accepted;
- wrong issuer rejected;
- wrong audience rejected;
- expired and not-yet-valid tokens rejected;
- rotated JWKS key accepted after refresh and retired key rejected according to overlap policy;
- missing role or permission rejected with `403`;
- unauthorized warehouse rejected with `403 WAREHOUSE_ACCESS_DENIED`;
- unregistered or unauthorized device rejected;
- revoked session rejected;
- refresh rotation and reuse detection;
- production startup rejects `PDA_AUTH_MODE=mock`;
- no token, password, or secret appears in logs, cache keys, events, or API error bodies.

## 3. WMS Integration Boundary

PostgreSQL is the PDA backend system of record for committed PDA commands, audit records, idempotency records, inventory projections, command status, outbox, inbox, and DLQ metadata. WMS remains authoritative for the operational warehouse data that it owns. Redis is cache only.

The WMS adapter is an anti-corruption layer:

- WMS DTOs stay inside `internal/integration/adapters`;
- application services use backend domain models and ports;
- no PDA handler or domain service calls WMS HTTP directly;
- no WMS database is accessed directly by the PDA backend;
- all outbound WMS requests carry correlation, tenant, warehouse, operator/service identity, idempotency, and concurrency data as agreed;
- inbound WMS events are validated before inbox deduplication and projection;
- unknown WMS statuses and malformed payloads fail closed and are observable.

### 3.1 WMS transport requirements

- HTTPS only in staging and production;
- certificate verification enabled; no insecure TLS bypass;
- service-account or OAuth2 client-credentials authentication, as approved by WMS;
- explicit connect, read, and total request timeouts;
- bounded retries only for connection failures, `408`, `429`, and selected `5xx` responses;
- exponential backoff with jitter and a retry budget;
- no retry for validation, authorization, conflict, or business-rule errors;
- circuit breaker after repeated upstream failures;
- rate-limit handling honors `Retry-After` where provided;
- upstream error bodies are redacted before logs;
- correlation ID is preserved across retries and WMS calls.

### 3.2 Required WMS response contract

Every WMS response used by the backend must define:

- HTTP status and machine error code;
- correlation/request ID;
- warehouse and tenant identity;
- aggregate identifier and version or update timestamp;
- server timestamp/timezone;
- pagination cursor for lists;
- stable status enum values;
- retryability and conflict semantics;
- event or checkpoint position when the response is replayable.

The backend maps WMS failures to PDA-safe errors such as `UPSTREAM_WMS_UNAVAILABLE`, `RATE_LIMITED`, `WMS_VALIDATION_FAILED`, `TASK_VERSION_CONFLICT`, or `COMMAND_NOT_FOUND`. WMS stack traces, SQL errors, access tokens, and sensitive operational details must never reach the PDA.

## 4. Phase-by-Phase WMS Requirements

### Phase 00: Contract freeze and traceability

WMS must provide the canonical source for each operational field and status. Freeze one mapping for every API-001 through API-028 operation before production enablement.

Required WMS deliverables:

- endpoint and event catalog;
- field-level mapping and ownership matrix;
- status mapping with unknown-status policy;
- warehouse, operator, device, item, location, lot, serial, LPN, shipment, and task identifiers;
- version/concurrency model;
- idempotency and replay rules;
- audit and retention requirements.

### Phase 01: Common transport, OpenAPI, and errors

WMS calls must preserve `X-Correlation-Id`, warehouse context, server time, and version information. WMS failures must map to the frozen PDA envelope and not create a second error shape. Define whether WMS pagination cursors can be passed through or must be translated into opaque PDA cursors.

### Phase 02: OIDC, device, bootstrap, and profile

OIDC claims must authorize the WMS warehouse scope. Bootstrap must return only warehouses the operator and device may use. WMS directory synchronization must define whether operator, role, warehouse, shift, and device registration data is pulled from WMS, identity, or backend-owned tables.

Required decision: identity ownership for operator status, role changes, warehouse assignment, shift, and device registration.

### Phase 03: Dashboard, tasks, and list foundation

WMS must expose or publish task summaries and task records for receiving, putaway, picking, replenishment, and shipping. Required fields include task ID, type, status, priority, warehouse, zone, due time, assigned operator, lock state, line/piece counts, version, and updated time.

Define whether dashboard counts are WMS snapshots, backend projections, or calculated PostgreSQL views. Define checkpoint/replay behavior when task events are delayed.

### Phase 04: Receiving

WMS must provide receiving order/task, purchase order, supplier, expected lines, item/barcode, lot/serial policy, condition policy, quantities, version, and receipt status.

Required commands or events:

- start receiving task;
- resolve barcode in receiving context;
- confirm received quantity and condition/lot/serial data;
- complete receiving task;
- publish or accept receipt/inventory-change events.

Define over-receipt, under-receipt, tolerance, partial receipt, duplicate receipt, and inventory-posting ownership. A successful PDA command must have one durable result and one logical WMS/backend effect.

### Phase 05: Putaway, picking, and replenishment

WMS must provide task lines, source/destination locations, item identity, expected quantities, available/reserved stock, capacity/eligibility, and task versions.

Required validations:

- source location and item/barcode;
- destination capacity and location policy;
- picking location and item resolution;
- replenishment source, destination, and item;
- quantity and partial-completion rules.

Define short-pick behavior explicitly. If short-pick is disabled, WMS must reject it consistently and return a mapped safe error. Aggregate IDs must be used as Kafka keys to preserve ordering for each task/order.

### Phase 06: Inventory inquiry and transfer

WMS must provide inventory balances and history with item, location, lot, serial, LPN, on-hand, reserved, available, damaged, hold/quarantine, in-transit, UOM, version, and `asOf` values.

For transfers, define:

- source and destination authorization;
- reservation/locking behavior;
- atomicity across source and destination;
- transfer command idempotency;
- conflict/version behavior;
- event names and replay checkpoints;
- whether WMS or backend owns the final inventory posting.

No cache result may authorize a mutation. Transfer confirmation must use fresh authoritative validation.

### Phase 07: Cycle count

WMS must provide count task, line, location, item, lot/serial, system quantity policy, count version, and approval state.

Required decisions:

- blind count visibility;
- variance thresholds;
- recount and approval ownership;
- reason codes;
- whether WMS or backend applies approved adjustments;
- event emitted for count submitted, recount requested, approved, rejected, and completed.

Count submission must not silently adjust inventory. A count response must distinguish counted quantity, variance, review required, approval required, and recount required.

### Phase 08: Shipping, package verification, and confirmation

WMS must provide shipment/order, customer/ship-to, package/LPN identifiers, expected package count, verified package count, carrier, tracking, readiness blockers, shipment status, and version.

Required commands/events:

- package barcode verification;
- readiness projection refresh;
- final shipment confirmation;
- manifest/label reference if WMS owns it;
- shipment confirmed/order shipped event.

Final confirmation must require fresh readiness and must be online-only. Package verification and shipment confirmation must be idempotent and must not fabricate shipment success when WMS is unavailable.

### Phase 09: Command status and offline recovery

Every WMS-backed mutation must return or persist a command ID that can be resolved through `GET /api/pda/v1/commands/{commandId}`. The status model must include at least `RECEIVED`, `ACKNOWLEDGED`, `PROCESSING`, `COMPLETED`, `FAILED`, `CONFLICT`, and `REJECTED`.

Define mapping between:

- PDA `commandId`;
- `Idempotency-Key`;
- backend command row;
- WMS request ID/idempotency key;
- WMS event ID and checkpoint;
- outbox/inbox record;
- DLQ record.

A timeout is never treated as failure or success until command status is resolved. Offline PDA synchronization is limited by workflow policy; shipment confirmation remains online-only.

### Phase 10: Cache, freshness, invalidation, and observability

WMS responses must include authoritative `asOf`, version, and server time where available. Backend cache keys must remain warehouse/operator scoped and must never contain authorization tokens. Every WMS mutation must identify affected projections for invalidation.

Required operational metrics:

- WMS request count, error count, timeout count, retry count, breaker-open count;
- WMS latency p50/p95/p99;
- cache hit/miss/error/stale/invalidation metrics;
- command backlog and oldest pending age;
- Kafka consumer lag, event age, retry count, and DLQ count;
- reconciliation mismatch count by warehouse and aggregate type.

Define reconciliation jobs that compare WMS authoritative state with backend projections, report mismatches, and never silently overwrite committed PDA command history.

### Phase 11: External verification

Before approval, execute the staging test plan in Section 8 with real OIDC, WMS, Kafka TLS/ACL, PostgreSQL, Redis, PDA client, and Zebra hardware. Mock tests remain useful regression tests but do not satisfy external verification.

## 5. Event and Kafka Requirements

### 5.1 Topic contract

Provide a table containing:

| Topic | Producer | Consumer group | Key | Schema/version | Retention | DLQ |
|---|---|---|---|---|---:|---|
| task events | WMS/backend | PDA backend group | aggregate/task ID | required | required | required |
| inventory events | WMS/backend | PDA backend group | item/location or movement ID | required | required | required |
| count events | WMS/backend | PDA backend group | count task ID | required | required | required |
| shipment events | WMS/backend | PDA backend group | shipment ID | required | required | required |

Use aggregate IDs as keys. Preserve ordering per aggregate. Include event ID, event type, schema version, aggregate version, occurred time, correlation ID, causation ID, warehouse ID, operator/service ID, and payload.

### 5.2 Security and reliability

- TLS with a trusted CA is required outside local development.
- ACLs must separately authorize producer, consumer group, topic read/write, and DLQ operations.
- The backend must fail closed on authorization errors and must not switch to mock delivery.
- Outbox records remain durable until published or scheduled for retry.
- Consumer inbox deduplication must happen before applying an event.
- Poison messages move to DLQ with reason and attempt count after the configured retry limit.
- Broker restart and consumer rebalance must not duplicate business effects.
- Backlog and event-age metrics must be exported to the operations dashboard.

## 6. WMS-to-PDA Error Mapping

| WMS condition | PDA/backend response | Retry |
|---|---|---|
| Invalid credentials/token | `401 AUTH_SESSION_EXPIRED` or integration alarm | no PDA mutation retry |
| Missing warehouse/device permission | `403 WAREHOUSE_ACCESS_DENIED` | no |
| Invalid field/business rule | `400` mapped WMS validation code | no |
| Stale aggregate/version | `409 TASK_VERSION_CONFLICT` | refresh then user decision |
| Duplicate idempotency key | original durable result | no second effect |
| Rate limit | `429 RATE_LIMITED` | bounded backoff |
| Timeout/connection failure | `503 UPSTREAM_WMS_UNAVAILABLE` or pending command | command-status recovery |
| WMS 5xx | `503 UPSTREAM_WMS_UNAVAILABLE` | bounded retry/circuit breaker |
| Unknown status/schema | `502` integration error and alert | no silent conversion |
| Event duplicate | acknowledge after inbox check | no second effect |
| Event poison payload | retry then DLQ | operator replay |

## 7. Configuration Contract

Use deployment-managed environment or secret references:

```text
PDA_ENVIRONMENT=staging|production
PDA_AUTH_MODE=oidc
PDA_OIDC_ISSUER_URL=<issuer>
PDA_OIDC_AUDIENCE=<backend-audience>
PDA_OIDC_JWKS_URL=<jwks-url-if-not-discovered>
PDA_OIDC_REQUIRED_SCOPES=<space-separated-scopes>
PDA_OIDC_CLOCK_SKEW=30s

PDA_UPSTREAM_WMS_MODE=http
PDA_UPSTREAM_WMS_BASE_URL=https://<wms-host>
PDA_UPSTREAM_WMS_TOKEN=<secret-reference>
PDA_UPSTREAM_WMS_TIMEOUT=10s

PDA_MESSAGING_MODE=kafka
PDA_KAFKA_BROKERS=<broker-1:port,broker-2:port>
PDA_KAFKA_SECURITY_PROTOCOL=TLS
PDA_KAFKA_TLS_CA_FILE=<mounted-ca-file>
PDA_KAFKA_TLS_CERT_FILE=<mounted-client-cert-if-mTLS>
PDA_KAFKA_TLS_KEY_FILE=<mounted-client-key-if-mTLS>
PDA_KAFKA_TLS_SERVER_NAME=<certificate-server-name>
PDA_KAFKA_GROUP_ID=pda-backend-<environment>
PDA_KAFKA_TOPIC_PREFIX=<approved-prefix>
```

The exact OIDC and WMS variable names may be adapted to the deployment system, but the semantics must remain unchanged. Production must reject missing required real-mode settings and all mock modes.

## 8. Staging Acceptance Plan

Run and retain request/response/event evidence for:

1. Login, refresh, logout, bootstrap, profile, warehouse selection, and device authorization.
2. Dashboard/task refresh from WMS-backed data and delayed-event reconciliation.
3. Receiving scan with leading zeros and symbology, duplicate receipt, stale version, timeout, and command-status recovery.
4. Putaway, picking, and replenishment validation, partial quantity policy, duplicate command, and event replay.
5. Inventory search/balance freshness, transfer atomicity, duplicate transfer, and WMS outage.
6. Blind count, variance, recount, approval, and no-silent-adjustment behavior.
7. Package verification, readiness refresh, final confirmation, duplicate confirmation, and online-only enforcement.
8. Redis outage before and after a committed mutation.
9. Kafka TLS handshake, ACL denial, broker restart, consumer rebalance, ordering, outbox recovery, retry, and DLQ replay.
10. OIDC key rotation, token expiry, refresh rotation, revocation, unauthorized role/warehouse/device, and service restart.
11. PDA process restart and WorkManager command recovery using API-024.
12. Physical Zebra DataWedge trigger, profile, symbology, leading-zero preservation, duplicate scan guard, scanner suspension, reconnect, and no duplicate writes.

## 9. Approval Evidence and Ownership

The integration owner must return:

- completed OIDC claim mapping and discovery/JWKS details;
- WMS OpenAPI/event schemas and status/error mapping;
- Kafka topic/group/ACL/TLS matrix;
- staging endpoint and test credentials through the secret manager;
- test data IDs and reset procedure;
- exported test results and correlation IDs;
- reconciliation and lag/backlog dashboard links;
- named owners for OIDC, WMS, Kafka, database, PDA, and Zebra verification.

Phase 11 can be marked `APPROVED` only after all external checks pass. Until then, the backend remains safe in explicit mock/local modes and must not claim production OIDC or WMS readiness.
