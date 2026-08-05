# PDA App - PDA Backend Integration Implementation Guide

- Document status: **BLOCKED FOR NETWORK ACTIVATION; IMPLEMENTATION-READY FOR PDA CLIENT WORK**
- Generated date: 2026-08-02
- PDA App repository: external Kotlin Android repository; not present in this workspace
- PDA App package: `com.example.pda_app`
- PDA Backend repository: `/home/neurosus/recoh-system/pda-backend`
- Backend branch/commit: `main` / `ae81080f55a9b4830c9a0c12807c9584c632595c`
- PDA App branch/commit: not available in this workspace
- Backend server Tailscale IP: `100.68.50.41` (provided by the integration prompt; not independently reachable from this checkout)
- Gateway listen port: `8080` inside the gateway process/container
- Gateway host port: `8080` by default through `PDA_GATEWAY_PORT`
- Protocol: HTTP in the current Go gateway; HTTPS is required outside local development
- Public API base path: `/api/pda/v1`
- Integration scope: replace PDA App mock remote behavior with the public PDA Backend HTTP API
- Source-of-truth policy: current Go routes and OpenAPI first, then Compose/configuration, reports, and older documents

## 1. Executive Summary

The PDA App must communicate only with the PDA Backend gateway over HTTP(S) JSON. It must not connect to PostgreSQL, Redis, Kafka, WMS, or backend internal services. The existing PDA App API specification describes mock repositories and 28 logical operations; the backend exposes the corresponding gateway route families, but several older paths remain compatibility aliases. The PDA client should use the canonical paths in the current OpenAPI and treat the older proposed paths as unsupported unless a compatibility test proves them.

Backend phases 00 through 10 are locally implemented and their reconciliation reports are approved. Phase 11 remains partially approved: backend-owned authentication is implemented and tested, but WMS workflow integration, secured Kafka staging, production key policy, PDA client staging tests, and Zebra hardware validation remain external blockers. The gateway also currently fails closed for non-mock WMS configuration and the Compose file does not publish port 8080 to the host.

| Area | Current PDA state | Backend capability | Required PDA work | Readiness |
|---|---|---|---|---|
| Network | Mock/local remote configuration | Gateway HTTP on internal `:8080`; no Compose host mapping | Add injected base URL after host mapping is supplied | BLOCKED externally |
| Authentication | Mock authentication path described by app spec | PostgreSQL identity, Argon2id, durable sessions, opaque rotated refresh tokens | Implement secure session repository and single-flight refresh | Ready for client implementation |
| Dashboard | Local/fake projections | `GET /dashboard`, task summary, tasks | Remote reads into Room projections | Ready locally |
| Tasks | Local task mutations | Task list/detail/claim/release and command status | Map versions, idempotency, `If-Match` | Ready locally |
| Receiving | Local resolve/confirm | Task, start, barcode, receipt, completion, command status | Replace local mutation with remote command flow | Ready locally; WMS-backed validation pending |
| Putaway | Local validation/complete | List/detail, source/destination validation, suggestions, confirmation | Implement scanner state machine and versioned mutation | Ready locally; WMS contract pending |
| Picking | Local line mutation | List/detail, location/barcode validation, pick/completion | Implement remote validation and command flow | Ready locally; WMS contract pending |
| Replenishment | Local task update | List/detail, source/destination/item validation, confirmation | Preserve partial completion and server versions | Ready locally; WMS contract pending |
| Inventory | Local fake balances | Search, balances, movements, transfer validation/confirmation | Read authoritative server data; no local stock mutation | Ready locally; WMS events pending |
| Transfer | Local mutation | Versioned source/destination/item validation and confirmation | Queue only idempotent command metadata | Ready locally |
| Cycle count | Local count update | List/detail, validation, submit, recount, completion | Preserve variance/review states; never adjust locally | Ready locally |
| Shipping | Local package/shipment behavior | Shipment summary/readiness, package verification, confirmation, command status | Online-only verification and final confirmation | Ready locally; Zebra/WMS pending |
| Command status | Not authoritative | Durable command query endpoints | Persist command metadata and reconcile after timeout | Ready locally |
| Scanner | Zebra integration is external | Scanner context fields are accepted by workflow APIs | Send raw/normalized value, symbology, purpose, context | Hardware validation pending |
| Room | Existing local cache/draft boundary | API responses are authoritative | Separate DTOs, entities, drafts, and sync records | Ready for implementation |
| WorkManager | Existing offline direction | Command status supports reconciliation | Use network-constrained pending-command worker | Ready for implementation |

Integration can start immediately against a supplied reachable gateway URL for authentication, read APIs, and local workflow fixtures. Full production integration cannot be signed off until the gateway host port, HTTPS certificate, WMS, Kafka, and Zebra environment are supplied.

## 1.1 Real API Migration Gap Reconciliation

`docs/backend-integration/Real-API-Migration-Gap-Report.md` is a PDA client migration audit. It reports which Android repositories and screens have been switched from mock/demo bindings; it does not establish that every listed operation is absent from the backend. The current audit states that the PDA client has login wired, while refresh, logout, bootstrap, dashboard mapping, task reads, workflow reads, scanner validation, mutations, and WorkManager reconciliation remain client work.

| Area | PDA client audit result | Current backend result | Action |
|---|---|---|---|
| Login | Remote repository wired | `POST /auth/login` implemented and PostgreSQL-backed | Connect only after gateway reachability is fixed |
| Refresh/logout | DTO/service only; not wired | `/auth/refresh` rotation and `/auth/logout` revocation implemented | Add Keystore storage and single-flight refresh in the app |
| Bootstrap/profile/context | Not wired | `/bootstrap`, `/me`, `/me/warehouses`, and device registration implemented | Persist trusted context before enabling writes |
| Dashboard/tasks | DTO/service or partial fetch | Dashboard, summary, list, detail, claim, release implemented | Map envelopes into Room and preserve versions |
| Receiving/movement | Client missing | Backend workflow routes implemented against local PostgreSQL fixtures | Replace client local mutations with remote calls |
| Inventory/transfer/count | Client missing | Search, balances, movements, transfer, and count routes implemented | Confirm freshness and WMS authority before production |
| Shipping/package | Client missing | Summary, readiness, package verification, confirmation, and status implemented | Keep final confirmation online-only |
| LPN/pallet/adjustment reads or mutations | Client reports unsupported | No current public routes, domain tables, or approved contract found | Do not invent; obtain an approved contract before implementation |
| Offline/sync | Client missing | Generic command status and durable workflow command records exist | Add client PendingCommand/WorkManager integration |
| Network | TC26 cannot reach `100.68.50.41` | Gateway listens on internal `8080`; Compose publishes no host gateway port | Publish a port/reverse proxy and verify from TC26 |

### Contract corrections from the migration report

- The current backend success and error responses use the common `data`/`meta`/`errors` envelope. A standalone error object such as `{"code":"...","correlationId":"..."}` is not the current gateway wire format. The PDA client must decode `errors[0]` from the envelope, or the backend and OpenAPI must be changed together under a new contract version.
- `Accept-Language` is allow-listed by the gateway (`en-US`, `vi-VN`) and returned as `Content-Language`; it is not a WMS localization contract.
- Authenticated operator identity is derived from the bearer session. `X-Operator-Id` is an optional consistency check, not a substitute for token identity. Device and warehouse headers are required on warehouse-scoped routes.
- The current canonical transfer routes are `/inventory/transfers/source-validations`, `/inventory/transfers/destination-validations`, `/inventory/transfers/item-resolutions`, and `POST /inventory/transfers`, with compatibility aliases under `/transfers`. `/inventory/transfers/validations` is not a registered route.
- The current canonical count, receiving, movement, and shipping paths are those in `api/openapi/pda-v1.yaml`; the client must not activate a path only because it appears in the migration report without a matching OpenAPI/router entry.
- Mutation responses currently expose the authoritative domain result and common metadata. `PENDING_WMS` is not a current generic backend command status because real WMS acknowledgement integration is not implemented. The client must treat transport ambiguity through API-024 and must not claim WMS acceptance.

## 2. Verified Backend Connection Information

Current source evidence:

- `cmd/pda-api-gateway/main.go` binds the HTTP server to `:8080`.
- `cmd/pda-api-gateway/Dockerfile` declares `EXPOSE 8080`.
- `docker/compose.yml` publishes gateway `${PDA_GATEWAY_PORT:-8080}:8080`, PostgreSQL `15432`, and Redis `16379`.
- `internal/gateway/adapters/http/router.go` registers health routes at the root and public routes under `/api/pda/v1`.
- The gateway does not terminate TLS itself; HTTPS requires a reverse proxy or TLS-capable deployment in front of it.

```text
Backend Tailscale IP: 100.68.50.41 (prompt-provided, not independently verified here)
Gateway container/listen port: 8080
Gateway host port: 8080 by default (`PDA_GATEWAY_PORT` override)
Protocol in current process: HTTP
Public API base path: /api/pda/v1
Base URL: http://100.68.50.41:8080/api/pda/v1, pending Tailscale/firewall verification
Health URL: http://100.68.50.41:8080/healthz, pending Tailscale/firewall verification
Readiness URL: http://100.68.50.41:8080/readyz, pending Tailscale/firewall verification
```

The Compose mapping now publishes port 8080 by default. The deployment owner must still verify the host firewall, Tailscale ACL, and running service before enabling the physical PDA. For HTTPS, use a reverse proxy/DNS endpoint instead of the direct HTTP URL. The current development URL is:

```text
http://100.68.50.41:8080/api/pda/v1
```

Health and readiness are root paths, not prefixed by `/api/pda/v1`.

## 3. Tailscale Connectivity

```text
Zebra PDA or Android development device
  -> Tailscale connection and ACL
  -> 100.68.50.41
  -> published gateway host port or HTTPS reverse proxy
  -> PDA Backend gateway
  -> PostgreSQL/Redis/Kafka/WMS internal services
```

Requirements:

1. The laptop running the emulator and the physical Zebra device must be connected to the same tailnet, unless an approved routed subnet is used.
2. Tailscale ACLs must allow the device identity to reach only the gateway port, not PostgreSQL, Redis, Kafka, or WMS ports.
3. The backend host firewall must allow the Tailscale interface to reach the published gateway port.
4. Docker must publish the gateway port or a reverse proxy must listen on the Tailscale interface.
5. Tailscale is required on a physical Zebra device unless the device reaches the backend through an approved corporate network route. The Android emulator can normally reach a Tailscale IP through the host, but verify on the selected emulator image.

Laptop verification:

```shell
tailscale status
ping 100.68.50.41
curl -i http://100.68.50.41:<HOST_PORT>/healthz
curl -i http://100.68.50.41:<HOST_PORT>/readyz
curl -i http://100.68.50.41:<HOST_PORT>/api/pda/v1/me
```

Device verification after the port is supplied:

```shell
adb shell ping -c 1 100.68.50.41
adb shell toybox nc -vz 100.68.50.41 <HOST_PORT>
```

Use a Tailscale MagicDNS name for production HTTPS where possible. A certificate issued for a DNS name will not validate when the client connects directly to an IP address. Do not disable hostname verification or trust all certificates.

## 4. Android Environment Configuration

Base URLs must be injected through Gradle flavors or CI-managed build configuration. They must not be hardcoded in repositories, ViewModels, Composables, or DTOs.

Required configuration keys:

```text
PDA_API_SCHEME
PDA_API_HOST
PDA_API_PORT
PDA_API_BASE_PATH
PDA_API_CONNECT_TIMEOUT_SECONDS
PDA_API_READ_TIMEOUT_SECONDS
PDA_API_WRITE_TIMEOUT_SECONDS
PDA_API_CALL_TIMEOUT_SECONDS
PDA_API_DEBUG_LOGGING
PDA_API_ALLOW_CLEARTEXT
```

Resolved development values for the current Compose profile are:

```properties
PDA_API_SCHEME=http
PDA_API_HOST=100.68.50.41
PDA_API_PORT=8080
PDA_API_BASE_PATH=/api/pda/v1
PDA_API_ALLOW_CLEARTEXT=true
```

Recommended environments:

| Environment | Scheme | Host | Port | Base path | Cleartext | Certificate policy |
|---|---|---|---|---|---|---|
| local | http | emulator/host or local gateway | injected | `/api/pda/v1` | debug only | no production certificate |
| tailscale-dev | http initially, HTTPS preferred | `100.68.50.41` or MagicDNS | `8080` by default | `/api/pda/v1` | debug flavor only | validate cert when HTTPS |
| staging | https | staging DNS | injected | `/api/pda/v1` | false | trusted CA, hostname verification |
| production | https | production DNS | injected | `/api/pda/v1` | false | trusted CA; pinning only by approved operations policy |

The app should expose one `ApiEnvironment` object to dependency injection. It should construct one URL from scheme, host, port, and base path and validate that release builds use HTTPS.

## 5. Manifest and Network Security

Required permissions:

```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
```

For HTTP Tailscale development, use a debug-only `network_security_config.xml` scoped to the approved development host. Do not set global `android:usesCleartextTraffic="true"` in the release manifest. Release builds must require HTTPS. Direct-IP HTTPS requires a certificate whose SAN covers the IP; otherwise use a MagicDNS hostname matching the certificate.

Recommended manifest arrangement:

- `src/main/AndroidManifest.xml`: `INTERNET`, `ACCESS_NETWORK_STATE`, and `android:networkSecurityConfig` reference.
- `src/debug/res/xml/network_security_config.xml`: allow cleartext only for the exact development endpoint if the deployment owner confirms HTTP is temporary.
- `src/release/res/xml/network_security_config.xml`: no cleartext; system trust store only unless a reviewed enterprise CA is required.
- Never add a trust-all `HostnameVerifier`, permissive `TrustManager`, or certificate bypass.

## 6. Recommended Android Networking Stack

Use the project’s existing compatible stack if present; otherwise use Retrofit + OkHttp + `kotlinx.serialization` (or the project-approved serializer) with coroutines. Keep these layers separate:

```text
Compose
  -> ViewModel
  -> Use Case
  -> Repository
       -> Room local data source
       -> Remote API data source
       -> Sync coordinator
  -> Retrofit/OkHttp
  -> PDA Backend
```

Recommended package layout:

```text
data/remote/api/
data/remote/dto/
data/remote/mapper/
data/remote/interceptor/
data/remote/authenticator/
data/remote/error/
data/remote/environment/
data/local/room/
data/sync/
domain/
```

Rules:

- Compose consumes domain models and UI state, never API DTOs.
- ViewModels call use cases, never build URLs or headers.
- Room entities are not serialized request/response DTOs.
- Raw Retrofit/OkHttp exceptions are converted to a sealed domain error type.
- Only the repository/sync layer decides whether a request is online-only or queueable.

HTTP defaults should be connect 10 seconds, read 30 seconds, write 30 seconds, and call 45 seconds unless the app’s approved operational profile differs. Do not blindly retry POST mutations. Retry only safe reads and explicitly idempotent commands, preserving the original idempotency key. JSON parsing should reject malformed required fields, tolerate additive unknown fields, parse ISO-8601 timestamps, and handle unknown enum values as an explicit `UNKNOWN` state rather than crashing.

Debug logs must redact passwords, access tokens, refresh tokens, `Authorization`, sensitive barcode values, and private operator data. Log method, route template, status, duration, correlation ID, and safe error code only.

## 7. Common Headers and Request Context

Implement one request-context provider and interceptors for:

```http
Authorization: Bearer <access-token>
X-Correlation-Id: <UUID>
X-Device-Id: <registered-device-id>
X-Warehouse-Id: <selected-warehouse-id>
X-Operator-Id: <optional; only when required by an approved request>
Idempotency-Key: <stable UUID for a command>
If-Match: "<aggregate-version>"
Accept-Language: vi-VN
Content-Type: application/json
```

The backend generates a UUID correlation ID when absent and returns it in the response. The app should generate one per user operation and preserve it through retries and command-status reconciliation. It must not generate a new idempotency key on retry or after process death. `If-Match` represents the aggregate version read from the server; on `409` the app preserves the draft and fetches the current aggregate before allowing a retry.

Device and warehouse headers are required on protected workflow routes. The app must not trust a locally selected warehouse without bootstrap validation. On warehouse change, clear or mark incompatible drafts and refresh bootstrap, dashboard, task lists, and policy before writes.

## 8. Authentication and Secure Sessions

The current gateway uses backend-owned internal authentication, not OIDC at runtime. OIDC is a future/external identity adapter requirement and must not be invented in the PDA client against the current gateway.

### Login sequence

1. POST `/auth/login` with username, password, device ID, device model, app version, warehouse ID, and locale.
2. Store access and refresh tokens only in Android Keystore-backed encrypted storage. Do not store either token in Room, logs, preferences, or UI state.
3. Register the device with `POST /devices/registrations` when required by deployment policy.
4. GET `/bootstrap` with `X-Device-Id` and `X-Warehouse-Id`.
5. Persist non-secret operator, warehouse, role, permission, scanner-policy, and server-time data in Room/DataStore as appropriate.
6. Fetch dashboard and initial task projections.

### Refresh sequence

Use a process-wide mutex/single-flight refresh coordinator. When a request receives an authentication failure:

1. One coroutine exchanges the stored refresh token at `/auth/refresh`.
2. Other requests await the same result.
3. Persist the new token pair atomically before releasing waiters.
4. Retry the original safe request once with the new access token.
5. Never retry a mutation unless its idempotency key is unchanged and the operation is explicitly retryable.
6. If refresh returns 401, clear secure session state, mark pending commands for re-authentication, clear scanner context, and navigate to login.

Refresh tokens are opaque and rotated. Reusing an old refresh token revokes the session, so concurrent refresh calls must never be issued independently.

### Logout

Call `POST /auth/logout` with the bearer token when available. Treat a 401 as local logout success. Always clear Keystore tokens, selected warehouse, scanner context, cached session policy, and pending UI state locally.

## 9. API and Screen Mapping

The backend’s current canonical route families are below. Compatibility aliases exist in the Go router, but the PDA App should implement canonical OpenAPI paths.

| API | Screen/use case | Canonical backend operation |
|---|---|---|
| API-001 | Login | `POST /auth/login` |
| API-002 | Session refresh | `POST /auth/refresh` |
| API-003 | Logout | `POST /auth/logout` |
| API-004 | Bootstrap/device policy | `GET /bootstrap` |
| API-005 | Dashboard | `GET /dashboard`, `GET /tasks/summary` |
| API-006 | Profile | `GET /me`, `GET /me/warehouses` |
| API-007 | Task Center | `GET /tasks`, `GET /tasks/{taskId}`, `POST /tasks/{taskId}/claim`, `POST /tasks/{taskId}/release` |
| API-008 | Receiving list/detail | `GET /receiving/tasks`, `GET /receiving/tasks/{taskId}` |
| API-009 | Receiving scan | `POST /receiving/tasks/{taskId}/barcode-resolutions` |
| API-010 | Receiving confirmation | `POST /receiving/tasks/{taskId}/receipts`, then completion when applicable |
| API-011 | Putaway list/detail | `GET /putaway/tasks`, `GET /putaway/tasks/{taskId}` |
| API-012 | Putaway source/item validation | `POST /putaway/tasks/{taskId}/source-validations` |
| API-013 | Putaway destination validation | `POST /putaway/tasks/{taskId}/destination-validations`; suggestions via GET |
| API-014 | Putaway confirmation | `POST /putaway/tasks/{taskId}/confirmation` |
| API-015 | Picking list/detail | `GET /picking/tasks`, `GET /picking/tasks/{taskId}` |
| API-016 | Picking location validation | `POST /picking/tasks/{taskId}/location-validations` |
| API-017 | Picking item resolution | `POST /picking/tasks/{taskId}/barcode-resolutions` |
| API-018 | Pick/short/complete | `POST /picking/tasks/{taskId}/picks`, completion route when required |
| API-019 | Replenishment list/detail | `GET /replenishment/tasks`, `GET /replenishment/tasks/{taskId}` |
| API-020 | Replenishment validation | source/destination/item validation routes under `/replenishment/tasks/{taskId}` |
| API-021 | Replenishment confirmation | `POST /replenishment/tasks/{taskId}/confirmation` |
| API-022 | Inventory inquiry | `GET /inventory/search`, `/inventory/items`, `/inventory/balances`, `/inventory/movements` |
| API-023 | Stock transfer | transfer validation routes and `POST /inventory/transfers` |
| API-024 | Command status | `GET /commands/{commandId}` or scoped workflow command route |
| API-025 | Cycle count | count list/detail, validation, submit, recount, completion routes |
| API-026 | Shipment readiness | `GET /shipments/{shipmentId}`, `/readiness` |
| API-027 | Package verification | `POST /shipments/{shipmentId}/packages/{packageId}/verify` |
| API-028 | Shipment confirmation | `POST /shipments/{shipmentId}/confirmation` |

The current OpenAPI and router are authoritative if the older `PDA_APP_API_SPECIFICATION.md` says “Proposed API” or uses a different path. Contract tests must be generated from `api/openapi/pda-v1.yaml` and must cover aliases only if the deployment intentionally supports them.

## 10. DTO, Entity, and Repository Design

Create separate DTOs for login, refresh, bootstrap, task pages, workflow validations, workflow confirmations, inventory queries, counts, shipments, and command status. Include `version`, `serverTime`, `correlationId`, `commandId`, `idempotencyKey`, `errorCode`, `retryable`, and `stale`/freshness fields where present.

Suggested Room projections:

```text
OperatorEntity
WarehouseEntity
DevicePolicyEntity
TaskEntity
ReceivingTaskEntity / ReceivingLineEntity
MovementTaskEntity
InventoryBalanceEntity
InventoryMovementEntity
CycleCountEntity / CycleCountLineEntity
ShipmentEntity / PackageEntity
PendingCommandEntity
SyncCursorEntity
```

`PendingCommandEntity` must contain at least command ID, idempotency key, operation type, aggregate ID, warehouse ID, serialized redacted request payload or encrypted payload reference, base version, correlation ID, state, retry count, next attempt, last error code, and timestamps. Never persist access or refresh tokens in this table.

Repository replacement map:

| Existing mock/local operation | Production repository behavior |
|---|---|
| `authenticate` | Remote API-001, secure token store, API-004 bootstrap |
| `dashboardSummary`, `tasks`, `findItems` | Remote read, map response to Room, serve cached read when policy allows |
| `receive` | Resolve remotely, create one durable command, confirm remotely, reconcile API-024 |
| `validatePutaway`, `completePutaway` | Remote validation state machine and versioned confirmation |
| `pick` | Remote location/item validation and idempotent pick command |
| `replenish` | Remote validation and partial confirmation; preserve remaining quantity |
| `transfer` | Remote source/item/destination validation and atomic server command |
| `submitCount` | Remote count submission/recount/completion; never mutate inventory locally |
| `verifyShipmentPackage`, `completeShipment` | Remote package verification; final shipment confirmation online-only |

Remove mock repositories from the production dependency graph only after remote contract tests pass. Keep mocks in test-only modules for deterministic unit tests.

## 11. Room and WorkManager Policy

Reads use a network-first or stale-while-revalidate policy based on screen criticality. Server responses are authoritative and update Room in one transaction. Room is a cache and offline draft store, not a substitute for server inventory or workflow truth.

Online-only operations:

- Login, refresh, bootstrap, device registration
- Barcode/location/item validation when server policy is authoritative
- Shipment package verification and final shipment confirmation
- Any command whose server-side version or lock cannot be safely reconstructed

Queueable operations:

- Explicitly idempotent task claim/release and workflow commands when the backend contract permits offline enqueue
- Receiving, putaway, picking, replenishment, transfer, count, and shipment commands only with command ID, stable idempotency key, base version, and approved offline policy

The current safe default is to queue command metadata and submit only when network is available; do not pretend a local mutation succeeded. WorkManager should use a network constraint, exponential backoff with bounded retries, and a unique work name per command. After timeout, transport failure, or duplicate response, query API-024 before retrying. `CONFLICT`, `REJECTED`, and permanent failures become visible user actions, not silent retries.

After each successful mutation, invalidate and refresh the affected projections:

| Mutation | Refresh/invalidate |
|---|---|
| Claim/release | task detail/list, summary, dashboard |
| Receiving | receiving task/lines, task list, dashboard, inventory balances |
| Putaway | movement task, task list, source/destination balances, dashboard |
| Picking | picking task/order, task list, inventory, shipment readiness |
| Replenishment | movement task, source/destination balances, dashboard |
| Transfer | both location balances, movement history, task/dashboard if linked |
| Cycle count | count task/lines/review state; no automatic balance update |
| Package verification | shipment packages/readiness |
| Shipment confirmation | shipment, task, dashboard |

## 12. Scanner Integration

Every scan must carry the scanner purpose and context. The app must not use a global “barcode resolves locally” shortcut.

| Scanner purpose | Screen | Backend validation |
|---|---|---|
| `RECEIVING_ITEM` | Receiving scan | API-009 |
| `PUTAWAY_SOURCE` | Putaway source | API-012 |
| `PUTAWAY_ITEM` | Putaway item step | API-012 or approved backend validation payload |
| `PUTAWAY_DESTINATION` | Putaway destination | API-013 |
| `PICKING_LOCATION` | Picking location | API-016 |
| `PICKING_ITEM` | Picking item/lot/serial | API-017 |
| `REPLENISHMENT_SOURCE` | Replenishment source | API-020 |
| `REPLENISHMENT_ITEM` | Replenishment item | API-020 |
| `REPLENISHMENT_DESTINATION` | Replenishment destination | API-020 |
| `INVENTORY_LOOKUP` | Inventory inquiry | API-022 |
| `TRANSFER_SOURCE` / `TRANSFER_ITEM` / `TRANSFER_DESTINATION` | Transfer | API-023 validation |
| `CYCLE_COUNT_LOCATION` / `CYCLE_COUNT_ITEM` | Cycle count | API-025 validation |
| `SHIPPING_PACKAGE` | Shipping | API-027 |

Send raw scan value only when the contract requires it, plus normalized value, symbology, scanner purpose, task/order/shipment ID, line/package ID, device ID, warehouse ID, operator context, and base version. DataWedge intent delivery and scanner profile configuration remain PDA App/hardware work. Physical Zebra testing is still outstanding.

## 13. Error Mapping

Map the backend envelope into a sealed `PdaError`:

```text
Unauthorized/session expired -> refresh once, then login
Forbidden/device/warehouse -> block write, show context action
Validation/422 -> scanner or form correction; no retry
Conflict/409 -> preserve draft, reload aggregate, require user resolution
Rate limited/429 -> honor Retry-After and backoff
Unavailable/timeout/503 -> queue only approved command or show offline state
Gateway circuit open -> show dependency unavailable and retain pending command
Unknown/server error -> correlation ID support action; no unsafe retry
```

Display the safe backend message and error code, never raw exception text. Preserve the response correlation ID for support. A 401 from logout is local success; a 401 from refresh clears the secure session. A duplicate command response must be reconciled through command status, not blindly submitted again.

## 14. Step-by-Step Implementation Order

1. Obtain the gateway host port/reverse-proxy URL, HTTPS certificate name, and Tailscale ACL approval. Do not start with a hardcoded `:8080` assumption.
2. Add Gradle environment flavors and `ApiEnvironment`; make release builds reject HTTP.
3. Add manifest permissions and debug-only network security configuration.
4. Add Retrofit/OkHttp, JSON configuration, redacted logging, correlation, device/warehouse, and idempotency interceptors.
5. Implement Keystore-backed `SecureSessionStore` and a mutex-based `TokenRefreshCoordinator`.
6. Implement API-001 through API-006 and bootstrap-driven session/context state.
7. Implement Room DTO mappers and read-through repositories for API-005, API-007, API-008, API-011, API-015, API-019, API-022, API-025, and API-026.
8. Implement common version/idempotency/command metadata and `PendingCommandEntity`.
9. Implement receiving, movement, inventory transfer, count, and shipping mutations one workflow at a time.
10. Add API-024 command reconciliation and WorkManager network-constrained processing.
11. Integrate Zebra DataWedge scan purposes and backend validation calls.
12. Remove production mock repository bindings while retaining test doubles.
13. Run emulator, Tailscale physical device, staging, and full PDA-to-backend E2E tests.
14. Enable production release only after HTTPS, WMS, Kafka, and Zebra acceptance evidence is attached.

## 15. Test Plan and Exit Criteria

Required client tests:

- URL construction for every environment; release rejects HTTP.
- JSON envelope, date/time, enum, pagination, and unknown-field behavior.
- Header injection and correlation preservation.
- Secure token storage; no token in logs or Room.
- Concurrent refresh single-flight; rotated refresh replay is not sent.
- Login, bootstrap, warehouse switching, logout, disabled device, and expired session.
- API-001 through API-028 contract tests generated from current OpenAPI.
- Every scanner purpose sends the correct route, context, raw/normalized data, and symbology.
- Stable idempotency key across retry, process death, duplicate response, and command-status recovery.
- `If-Match` conflict preserves local draft and reloads the aggregate.
- Room transaction updates and invalidation matrix per mutation.
- WorkManager backoff, network constraint, process restart, and terminal failure UX.
- Offline read behavior and online-only mutation blocking.
- Tailscale emulator and physical Zebra connectivity.
- HTTPS certificate hostname validation and release network-security policy.

Backend-side acceptance remains blocked until:

- a host gateway port or reverse proxy is configured;
- the actual Tailscale route is reachable from the PDA device;
- real WMS API/event ownership and credentials are supplied;
- Kafka TLS, ACL, rebalance, outbox, and DLQ evidence is supplied;
- production RS256 key management and retired-key policy are approved;
- API-001 through API-028 pass with the real PDA client;
- Zebra DataWedge behavior is verified on physical hardware.

## 16. Known Mismatches and Decisions

1. The app specification describes OIDC and proposed endpoint variants, but the current gateway uses backend-owned internal authentication and current OpenAPI/router paths. The PDA App must follow the current gateway contract; do not implement OIDC client flows without a separately approved backend OIDC adapter.
2. The app specification describes a complete real WMS path, while the current gateway uses mock WMS fixtures in the only operational local mode and fails closed for non-mock WMS. Do not treat local fixture success as production WMS evidence.
3. The app specification expects a reachable Tailscale base URL, but Compose currently does not publish the gateway port. Deployment configuration must resolve this before device integration.
4. The README describes HTTPS APIs, while the current gateway process is plain HTTP. HTTPS must be provided by deployment infrastructure before production use.
5. Kafka TLS configuration is implemented and local Kafka tests pass, but secure staging broker, ACL, and replay evidence is not present.

## 17. Final Readiness

**Document readiness: BLOCKED FOR NETWORK ACTIVATION, READY FOR PDA APP IMPLEMENTATION.**

No Android production code was modified in this repository. This document is the backend-side handoff. The PDA App team may implement the client layers and tests using an injected endpoint, but must not claim full E2E completion until the gateway host port, HTTPS/Tailscale route, WMS, Kafka, and Zebra prerequisites are verified.
