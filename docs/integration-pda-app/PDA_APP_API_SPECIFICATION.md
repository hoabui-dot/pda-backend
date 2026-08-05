# PDA App — Complete Backend API Specification

- **Document status:** Proposed contract for backend review; no endpoint below is confirmed.
- **Generated date:** 2026-08-02
- **Repository:** `PDA_APP`
- **Package:** `com.example.pda_app`
- **App version:** `1.0` / versionCode `1`
- **Current API integration status:** The current PDA App does not call a real backend. All warehouse data and mutations are currently handled by local mock repositories.
- **Purpose:** Give PDA_BACKEND engineers a field-level contract for replacing local behavior safely.
- **Intended readers:** PDA Android, WMS backend, QA, device-management and operations teams.
- **Source-of-truth policy:** Source code is current behavior; this document separates it from required/proposed backend behavior.

## Executive summary

The app uses Jetpack Compose, typed navigation, ViewModels, a centralized Zebra DataWedge `ScanCoordinator`, Room v3 as the UI cache, and WorkManager seams for durable synchronization. `MockWarehouseRepository` is the current remote simulator and `RoomBackedWarehouseRepository` reconciles its snapshots into Room. Authentication is `MockAuthRepository`; no Retrofit, Ktor, OkHttp, Internet permission, API DTO, or HTTP service exists. Phase 7 contract seams (`ApiAuthRepository`, `NetworkEnvironment`, typed error mapping) are intentionally blocked until WMS/OIDC contracts are approved.

There are 17 documented screen/workflow records: Login, Dashboard, Profile, Task Center, Receiving (list/detail/scan/confirm), Putaway (list/execution), Picking (list/detail), Replenishment, Inventory Inquiry, Stock Transfer, Cycle Count, Shipping Confirmation, plus generic/deferred module paths. The catalog below contains 28 proposed API operations: 8 read/query groups, 17 scanner/validation or mutation groups, and 3 authentication/bootstrap/session operations. All are **Proposed API — backend contract not yet confirmed**.

| Domain | Current implementation | API support required | Readiness |
|---|---|---|---|
| Authentication | mock credentials and in-memory session | OIDC/login, refresh, logout, profile, device scope | Blocked |
| Dashboard | Room/fake task summary | summary, actionable counts, freshness | Proposed |
| Task Center | Room task projection | paged tasks, filters, claim/release if approved | Proposed |
| Receiving | local resolver + Room draft/outbox | task reads, barcode resolve, versioned receive | Contract-ready |
| Putaway | local step validator | task/detail, source/destination validation, confirm | Proposed |
| Picking | local order/line mutation | location/item validation, pick/short-pick | Proposed |
| Replenishment | local source/item/destination flow | validation and partial completion | Proposed |
| Inventory Inquiry | local item/balance search | item/location/lot/serial/LPN query | Proposed |
| Stock Transfer | local balance mutation | atomic validation and transfer | Online-only pending policy |
| Cycle Count | local count line mutation | blind count, variance/review, recount | Proposed |
| Shipping | package verify/readiness/confirm mock | package, readiness, carrier/tracking | Online-only |

Evidence:
- `app/src/main/java/com/example/pda_app/data/repository/WarehouseRepository.kt`
- `data/mock/MockWarehouseRepository.kt`, `data/mock/MockWarehouseDatabase.kt`
- `data/local/WmsDatabase.kt`, `RoomBackedWarehouseRepository.kt`
- `scanner/ScanCoordinator.kt`, `scanner/DataWedgeReceiver.kt`
- `ui/AppRoot.kt`, active feature screens/ViewModels

## Current network and API capability

| Capability | Current status | Evidence | Impact |
|---|---|---|---|
| Internet permission | Not declared | `app/src/main/AndroidManifest.xml` | Real HTTP cannot be enabled yet |
| Retrofit/Ktor/OkHttp | Not present | `app/build.gradle.kts`, version catalog | No transport |
| JSON serialization | Test/debug serialization artifacts only | `gradle/libs.versions.toml` | No production DTO path |
| API interfaces/DTOs | Not implemented | no API service package | Backend contract cannot be consumed |
| Auth interceptor/refresh | Not implemented | `MockAuthRepository`, `ApiAuthContract.kt` | Session remains mock |
| Base URL | Validation seam only | `NetworkEnvironment.kt` | Explicit environment is required |
| Error mapper | Pure typed HTTP mapping only | `ApiErrorMapping.kt` | Not connected to transport |
| Retry policy | Implemented contract seam | `SyncContracts.kt` | Needs approved dispatcher/status API |
| Room | Implemented schema v3 | local database/DAOs/migrations | Local source of truth |
| WorkManager | Endpoint-neutral workers | `WmsSyncWorkers.kt` | Cannot submit commands without API |

## Screen-to-API inventory

| ID | Screen/route | Composable/ViewModel | Current repository calls | Required APIs |
|---|---|---|---|---|
| SCR-01 | Login | `LoginScreen` / `AuthViewModel` | `authenticate` mock | API-001..004 |
| SCR-02 | Dashboard | `DashboardScreen` / `DashboardViewModel` | `dashboardSummary`, `tasks` | API-005 |
| SCR-03 | Profile | `ProfileScreen` / `ProfileViewModel` | DataStore locale | API-004, API-006 |
| SCR-04 | Task Center | `TaskCenterScreen` / `TaskCenterViewModel` | `tasks` | API-007 |
| SCR-05 | Receiving list | `ReceivingListScreen` | `receivingOrders` | API-008 |
| SCR-06 | Receiving scan | `ReceivingScanScreen` / `ReceivingViewModel` | items, local resolve | API-009 |
| SCR-07 | Receiving confirm | `ReceivingConfirmScreen` / `ReceivingViewModel` | `receive(ReceiveCommand)` | API-010, API-024 |
| SCR-08 | Putaway list | `PutawayListScreen` / `PutawayViewModel` | `putawayTasks` | API-011 |
| SCR-09 | Putaway execution | `PutawayExecutionScreen` / `PutawayViewModel` | validate/complete | API-012..014 |
| SCR-10 | Picking list | `PickingListScreen` / generic VM | `pickingOrders` | API-015 |
| SCR-11 | Picking detail | `PickingDetailScreen` | `pick` | API-016..018 |
| SCR-12 | Replenishment | `ReplenishmentScreen` | `replenishments`, `replenish` | API-019..021 |
| SCR-13 | Inventory Inquiry | `InventoryInquiryScreen` | `findItems`, balances | API-022 |
| SCR-14 | Stock Transfer | `TransferScreen` | `transfer` | API-023 |
| SCR-15 | Cycle Count | `CycleCountScreen` | `submitCount` | API-025 |
| SCR-16 | Shipping | `ShippingScreen` | verify/complete shipment | API-026..028 |
| SCR-17 | Generic/deferred | `ModuleList/DetailScreen` | LPN/pallet/adjustment/demo | Deferred |

Legacy `ui/inventory/InventoryScreen.kt` is unreachable and must not receive a production API contract. LPN, pallet, adjustment, bin-count, bin-query and item-query generic paths are deferred until dedicated workflow contracts are approved.

## Naming, versioning and transport convention

**Proposed API — backend contract not yet confirmed:** base path `/api/pda/v1`; plural nouns for reads (`/receiving`, `/tasks`), explicit command subpaths (`/{id}/confirm`, `/{id}/resolve`), opaque cursor pagination, additive response evolution, and a documented deprecation window. Version changes must not silently alter quantity/version semantics. The PDA never calls Redis; Redis invalidation is backend-owned.

## Common request headers

| Header | Required for | Generated by | Purpose/validation | Retry behavior |
|---|---|---|---|---|
| `Authorization` | authenticated calls | secure session | Bearer access token; reject missing/expired | refresh once on 401 |
| `X-Correlation-Id` | every request | client UUID | trace one logical request; UUID format | preserve during retry |
| `X-Device-Id` | login/bootstrap and writes | registered-device abstraction | Zebra identity; server verifies registration | preserve |
| `X-Warehouse-Id` | warehouse reads/writes | authenticated context | scope data and authorization | preserve |
| `X-Operator-Id` | audit-sensitive writes | token context; body is non-authoritative | server must derive/verify identity | preserve |
| `Idempotency-Key` | every mutation | `CommandMetadata.idempotencyKey` | unique command identity | never regenerate |
| `If-Match` | versioned writes | Room entity version | optimistic concurrency | preserve; conflict is not auto-retried |
| `Accept-Language` | user-facing errors | DataStore locale | `vi-VN`/`en-US` supported values | current locale |
| `Content-Type` | JSON body | HTTP client | `application/json` | unchanged |

No secret is stored in DataStore, logs, drafts or UI. `SecureSecretStore` is currently a rejecting seam until a Keystore contract is approved.

## Common query parameters

| Parameter | Type/default | Validation | Used by |
|---|---|---|---|
| `cursor` | opaque string/null | server-issued only | all list reads |
| `limit` | integer/50 | 1–100 | lists |
| `status` | enum/none | endpoint-specific status | tasks/workflows |
| `category`/`type` | enum/none | known task type | Task Center |
| `q` | string/empty | trimmed, max 100 | task/item/location search |
| `sort`/`direction` | enum/updatedAt/asc | allow-listed fields | lists |
| `dateFrom/dateTo` | ISO date/null | from ≤ to | task/shipment reports |
| `warehouseId` | ID/session | must match scope | all warehouse reads |
| `operatorId` | ID/session | server verifies token | assigned-task reads |
| `priority`/`zone` | enum/string/null | allow-listed | Task Center/location |

## Common response envelope

**Proposed API — backend contract not yet confirmed.**

```json
{"data":{},"meta":{"serverTime":"2026-08-02T02:00:00Z","correlationId":"uuid","version":"12","nextCursor":null,"hasMore":false,"asOf":"2026-08-02T02:00:00Z"},"errors":[]}
```

| Field | Type | Required | Purpose | PDA usage |
|---|---|---:|---|---|
| `data` | object/array/null | yes | domain payload | map to domain then Room |
| `meta.serverTime` | ISO timestamp | yes | server clock | freshness/audit |
| `meta.correlationId` | UUID | yes | trace | safe diagnostic |
| `meta.version` | string/number/null | mutation/read entity version | conflict/cache |
| `meta.nextCursor` | string/null | list only | next page |
| `meta.hasMore` | boolean | list only | paging |
| `meta.asOf` | ISO timestamp | reads | stale indicator |
| `errors` | array | no | non-fatal warnings/errors | typed mapper |

## Common error response and status mapping

```json
{"code":"TASK_VERSION_CONFLICT","message":"Task changed by another operator.","details":{"taskId":"REC-001","currentVersion":12},"correlationId":"uuid","retryable":false}
```

| Field | Type | Required | Purpose | PDA behavior |
|---|---|---:|---|---|
| `code` | machine string | yes | stable branch key | map to `DomainErrorCode` |
| `message` | string | no | diagnostic/localization fallback | never render raw by default |
| `details` | object | no | safe IDs/current version/fields | conflict refresh |
| `correlationId` | UUID | yes | support trace | show/reference safely |
| `retryable` | boolean | yes | transport decision | WorkManager policy |

| HTTP | Category | PDA behavior |
|---:|---|---|
| 400 | malformed | field/blocking error; no retry |
| 401 | expired/invalid session | single-flight refresh, then logout |
| 403 | forbidden | permission error; keep cache |
| 404 | missing | refresh list/detail |
| 409 | version/duplicate/lock | preserve draft; query command status or conflict UI |
| 422 | business validation | inline workflow error; scanner remains active when safe |
| 429 | rate limited | honor retry-after/backoff |
| 500 | server | stale cache/read retry; writes require idempotency |
| 503 | dependency unavailable | cached read or blocked mutation |

## Error code catalog

| Code | Meaning/API | Retry | UI/scanner behavior |
|---|---|---:|---|
| `AUTH_INVALID_CREDENTIALS` | login | no | inline login error |
| `AUTH_SESSION_EXPIRED` | any auth call | refresh once | logout if refresh fails |
| `DEVICE_NOT_REGISTERED` | bootstrap | no | blocking setup |
| `WAREHOUSE_ACCESS_DENIED` | scoped call | no | blocking permission |
| `TASK_NOT_FOUND`/`TASK_NOT_ASSIGNED` | task read/command | no | refresh/select another |
| `TASK_LOCKED`/`TASK_ALREADY_COMPLETED` | task command | no | locked state, suspend scanner |
| `TASK_VERSION_CONFLICT` | versioned mutation | no | preserve draft, reload latest |
| `DUPLICATE_COMMAND` | mutation retry | query status | treat acknowledged replay as success |
| `BARCODE_UNKNOWN`/`BARCODE_WRONG_CONTEXT` | scan resolve | no | red feedback, keep context |
| `LOCATION_INVALID`/`LOCATION_CAPACITY_EXCEEDED` | movement | no | rescan/field error |
| `ITEM_NOT_IN_DOCUMENT` | receiving/picking | no | rescan |
| `QUANTITY_EXCEEDS_ALLOWED`/`INSUFFICIENT_STOCK` | quantity | no | field error |
| `COUNT_VARIANCE_REQUIRES_REVIEW` | count | no | review; no stock adjustment |
| `SHIPMENT_NOT_READY` | shipping | no | verify missing package |
| `RATE_LIMITED`/`UPSTREAM_WMS_UNAVAILABLE` | transport/dependency | yes | stale state + retry |
| `INTERNAL_ERROR` | unknown | safe read retry | generic error + correlation |

## API catalog conventions

Each API below is a complete contract card. Every endpoint is **Proposed API — backend contract not yet confirmed**. IDs and examples are illustrative. Response DTOs must be mapped to Room/domain; raw DTOs/exceptions never enter Compose.

### API-001 — Login

**Status:** Proposed API — backend contract not yet confirmed.

**Purpose/usage:** SCR-01 `AuthViewModel.signIn`; called after user submits credentials. No scanner. Authentication and device/warehouse scope are established.

**Method/path:** `POST /api/pda/v1/auth/login`.

**Auth/headers:** no bearer; correlation, device ID, content type, accept language, app version headers. Device ID may be absent only for first registration according to backend policy.

**Body:**

```json
{"username":"warehouse.operator","password":"<redacted>","deviceId":"TC26-001","deviceModel":"TC26","appVersion":"1.0","warehouseId":"MAIN","locale":"vi-VN"}
```

| Field | Type/req. | Purpose and validation | PDA source |
|---|---|---|---|
| `username` | string/yes | trimmed, non-empty; identity | login field |
| `password` | string/yes | TLS-only, never log/store | password field |
| `deviceId` | string/conditional | registered Zebra identity | device abstraction |
| `deviceModel` | string/yes | policy/diagnostics | Build/device |
| `appVersion` | string/yes | compatibility | BuildConfig |
| `warehouseId` | ID/conditional | selected permitted site | session/bootstrap |
| `locale` | enum/yes | `vi-VN`/`en-US` | DataStore |

**Success 200 response:**

```json
{"data":{"accessToken":"<redacted>","refreshToken":"<redacted>","tokenType":"Bearer","expiresAt":"2026-08-02T03:00:00Z","operatorId":"EMP-0001","employeeCode":"EMP-0001","displayName":"Warehouse Operator","roles":["RECEIVING_OPERATOR"],"permissions":["RECEIVE"],"warehouseId":"MAIN","warehouseName":"Main Warehouse","shiftCode":"DAY","deviceRegistrationStatus":"REGISTERED","featureFlags":{},"scannerPolicy":{}},"meta":{"serverTime":"2026-08-02T02:00:00Z","correlationId":"uuid"}}
```

`accessToken` is short-lived secure-session material; `refreshToken` storage requires approved Keystore. Profile/roles/warehouse feed Dashboard/Profile authorization. Errors: 400 `AUTH_INVALID_CREDENTIALS`, 403 `WAREHOUSE_ACCESS_DENIED`, 409 `DEVICE_NOT_REGISTERED`, 429 `RATE_LIMITED`, 503 `UPSTREAM_WMS_UNAVAILABLE`. No automatic credential retry; cached read-only mode is policy-dependent.

**Cache/refresh/retry/security:** do not cache password; persist only approved secure tokens; refresh on 401 once; clear session on failed refresh; audit login/device/correlation. Local mock mapping: `MockAuthRepository.authenticate`.

**Backend checklist:** OIDC/credential policy, device registration, warehouse scope, token expiry/refresh, permissions, safe error codes, server time, audit.

Evidence: `auth/AuthViewModel.kt`, `auth/MockAuthRepository.kt`, `AuthModels.kt`.

### API-002 — Refresh session

**Method/path:** `POST /api/pda/v1/auth/refresh`; **Proposed API — backend contract not yet confirmed.** Body `{refreshToken, deviceId}` (token field semantics must be approved). Headers correlation/device/content type; no expired bearer requirement. Return the same session fields as API-001 with rotated expiry. 401 `AUTH_SESSION_EXPIRED` logs out; 503 is retryable once. Refresh requests must be single-flight and never duplicate refresh tokens. Evidence: `ApiAuthRepository`, `SecureSecretStore`, `ApiErrorMapping`.

### API-003 — Logout/revoke

**Method/path:** `POST /api/pda/v1/auth/logout`; **Proposed API — backend contract not yet confirmed.** Body/session identifies refresh token/device; bearer required when available. 204 clears server session; 401 is treated as local success. PDA clears secure session, scanner context and navigation. Local logout is immediate; revoke is best effort and must not block safety. Evidence: `ProfileScreen`, `AppRoot`, `AuthViewModel`.

### API-004 — Bootstrap/profile/device policy

**Method/path:** `GET /api/pda/v1/bootstrap`; **Proposed API — backend contract not yet confirmed.** Response fields: operatorId, employeeCode, displayName, roles, permissions, warehouseId/name, shiftCode, deviceRegistrationStatus, serverTime, featureFlags, scannerPolicy, localePolicy. Called after login/resume; cached profile is displayable offline but policy must be fresh before writes. Errors: device not registered, warehouse denied, expired session. Evidence: `AuthenticatedUser`, Profile UI, `NetworkEnvironment`.

### API-005 — Dashboard summary

**Method/path:** `GET /api/pda/v1/dashboard?warehouseId=&operatorId=`; **Proposed API — backend contract not yet confirmed.**

**Response example:**

```json
{"data":{"inboundCount":3,"putawayCount":2,"pickingCount":4,"shippingCount":1,"inProgressCount":5,"completedCount":12,"completionPercent":70.6,"pendingSyncCount":0,"lastUpdatedAt":"2026-08-02T02:00:00Z","actionableAlertCount":0},"meta":{"serverTime":"2026-08-02T02:00:00Z","asOf":"2026-08-02T02:00:00Z","correlationId":"uuid"}}
```

Each count maps to Dashboard metric cards; `completionPercent` maps progress; `pendingSyncCount` maps offline status; `lastUpdatedAt/asOf` drives stale display. Authenticated read, cache in Room, refresh on resume and mutation acknowledgement, retry transport with backoff. Errors 401/403/503. Local mapping: `dashboardSummary()` and `DashboardViewModel`. Evidence: `DashboardScreen.kt`, `DashboardViewModel.kt`.

### API-006 — Operator profile

**Method/path:** `GET /api/pda/v1/me`; **Proposed API — backend contract not yet confirmed.** Returns operatorId, employeeCode, displayName, username, roles, permissions, warehouse and shift, active state, updatedAt. Called on Profile/resume; Room/DataStore may cache non-secret fields. 401/403 behavior as above. Local mapping: `AuthenticatedUser` and `ProfileViewModel`.

### API-007 — Task Center page/detail/claim/release

**Methods/paths:** `GET /api/pda/v1/tasks`, `GET /api/pda/v1/tasks/{taskId}`, optional `POST /api/pda/v1/tasks/{taskId}/claim` and `/release`; all **Proposed API — backend contract not yet confirmed**.

Query: cursor, limit (1–100, default 50), status, type/category, q, priority, zone, dateFrom/dateTo, sort/direction. Task response fields: id, category/type, status, priority, title, lineCount, pieceCount, dueAt, assignedOperatorId, lockState, version, createdAt, updatedAt, warehouseId. Claim/release body: commandId/idempotencyKey, taskId, baseVersion. Errors: task not found/assigned/locked/version conflict. Lists cached in Room, stale label required, refresh after every workflow mutation. Local mapping: `tasks()`, `TaskCenterViewModel`.

### API-008 — Receiving task list/detail

**Methods/paths:** `GET /api/pda/v1/receiving`, `GET /api/pda/v1/receiving/{taskId}`; **Proposed API — backend contract not yet confirmed**.

Query supports cursor/limit/status/supplier/q/priority/date. Response includes orderId, purchaseOrderId, supplier, status, version, expectedQuantity, receivedQuantity, remainingQuantity, lines, itemId, SKU/name/barcode, lotRequired, serialRequired, conditionPolicy, assignedOperatorId, dueAt, updatedAt, freshness. Cache list/detail in Room; drafts/outbox survive process death. Errors task not found/assigned/locked/upstream unavailable. Local mapping: `receivingOrders` and `RoomSnapshotSynchronizer`.

### API-009 — Receiving barcode resolution

**Method/path:** `POST /api/pda/v1/receiving/{taskId}/resolve-barcode`; **Proposed API — backend contract not yet confirmed**. Body:

```json
{"taskId":"REC-001","lineId":null,"rawValue":"00012345678905","normalizedValue":"00012345678905","symbology":"EAN-13","scanContext":"RECEIVING_ITEM","operatorId":"EMP-0001","warehouseId":"MAIN","deviceId":"TC26-001","scannedAt":"2026-08-02T02:00:00Z"}
```

Fields are required except lineId; raw/normalized values preserve leading zeroes, symbology is DataWedge metadata, context must equal active coordinator purpose. Success returns resolved itemId/display code/SKU/name, lineId, remainingQuantity, quantity policy, lot/serial requirements, taskVersion, nextStep. Errors barcode unknown/wrong context/item not in document/task version conflict. No inventory mutation; scanner stays active on recoverable validation errors. Local mapping: `ReceivingWorkflow.resolve`, `ScanPurpose.RECEIVING_ITEM`.

### API-010 — Receiving confirmation

**Method/path:** `POST /api/pda/v1/receiving/{taskId}/confirm`; **Proposed API — backend contract not yet confirmed**.

```json
{"commandId":"uuid","idempotencyKey":"uuid","taskId":"REC-001","lineId":"LINE-001","barcode":"00012345678905","quantity":3,"condition":"GOOD","remark":null,"baseVersion":11,"scannedAt":"2026-08-02T02:00:00Z"}
```

| Field | Required/validation |
|---|---|
| commandId/idempotencyKey | required UUID; stable across retries |
| taskId/lineId | required IDs belonging to same order |
| barcode | required string; preserve zeros; resolve to line |
| quantity | integer ≥1 and within policy |
| condition | approved enum; policy may require lot/serial |
| remark | nullable; required for under-receipt when policy says so |
| baseVersion | required optimistic version |
| scannedAt | client timestamp; server remains authoritative |

Success 200/202 returns authoritative order/line, received/expected/remaining, status, version, commandStatus, auditId, serverTime. Errors 409 version/duplicate/locked, 422 item/quantity/policy, 503 upstream. Durable outbox inserts before request; on timeout preserve command and query API-024. Successful mutation invalidates receiving/task/dashboard/balance Room projections. Local mapping: `RoomBackedWarehouseRepository.receive`, `ReceivingViewModel.confirmLine`.

### API-011 — Putaway task list/detail

**Method/path:** `GET /api/pda/v1/putaway`, `GET /api/pda/v1/putaway/{taskId}`; **Proposed API — backend contract not yet confirmed.** Return taskId/itemId/source/destination/quantity/completed/status/version/assignment/warehouse. Cache active tasks/drafts. Local mapping: `putawayTasks`, `PutawayViewModel`.

### API-012 — Putaway source/item validation

**Method/path:** `POST /api/pda/v1/putaway/{taskId}/validate-source`; **Proposed API — backend contract not yet confirmed.** Body `{taskId, sourceLocationCode, itemId?, barcode?, symbology?, scanContext, baseVersion}`. Validate assignment, source location and available stock; return resolved item/location, quantity, nextStep, version. Errors location invalid, barcode wrong context, insufficient stock, task locked.

### API-013 — Putaway destination validation/suggestion

**Method/path:** `POST /api/pda/v1/putaway/{taskId}/validate-destination`; **Proposed API — backend contract not yet confirmed.** Body `{taskId, destinationLocationCode, itemId, lpnId?, quantity, baseVersion}`. Return capacity, accepted flag, suggested destinations and nextStep. Errors invalid/capacity exceeded/version conflict. No mutation.

### API-014 — Putaway confirmation

**Method/path:** `POST /api/pda/v1/putaway/{taskId}/confirm`; **Proposed API — backend contract not yet confirmed.** Body `{commandId,idempotencyKey,taskId,sourceLocationCode,destinationLocationCode,itemId,lpnId?,quantity,baseVersion}`. Success returns task status/version, source/destination balances, auditId. Idempotent, versioned, transport-retry only; invalidates movement/task/balance/dashboard. Local mapping: `completePutaway`.

### API-015 — Picking task list/detail

**Method/path:** `GET /api/pda/v1/picking`, `GET /api/pda/v1/picking/{orderId}`; **Proposed API — backend contract not yet confirmed.** Return order/customer/salesOrder/status/version, currentLine, expectedLocation, item, lot/serial policy, quantityToPick/quantityPicked/remainingQuantity, nextLineId, taskProgress, shipmentReadinessImpact. Cache task; refresh after pick. Local mapping: `pickingOrders`.

### API-016 — Picking location validation

**Method/path:** `POST /api/pda/v1/picking/{orderId}/validate-location`; **Proposed API — backend contract not yet confirmed.** Body `{orderId,lineId,locationCode,rawValue,symbology,scanContext,baseVersion}`. Return expected location, accepted flag and next step. Errors location invalid/wrong context/task locked.

### API-017 — Picking item resolution

**Method/path:** `POST /api/pda/v1/picking/{orderId}/resolve-item`; **Proposed API — backend contract not yet confirmed.** Body `{orderId,lineId,itemId?,barcode,lotNumber?,serialNumber?,symbology,scanContext,baseVersion}`. Return resolved item/lot/serial, available quantity, line constraints and next step. Errors unknown/wrong context/insufficient stock.

### API-018 — Pick confirmation/short-pick/complete

**Methods/paths:** `POST /api/pda/v1/picking/{orderId}/lines/{lineId}/pick`, optional `/short-pick`, and `/complete`; **Proposed API — backend contract not yet confirmed**. Body `{commandId,idempotencyKey,orderId,lineId,locationCode,itemId,lotNumber?,serialNumber?,quantity,reasonCode?,baseVersion}`. Success returns currentLine/remaining/nextLine/taskProgress/shipmentReadinessImpact/version/auditId. Quantity and stock errors are non-retryable; command status API handles timeout. Local mapping: `pick`.

### API-019 — Replenishment list/detail

**Methods/paths:** `GET /api/pda/v1/replenishment`, `GET /api/pda/v1/replenishment/{taskId}`; **Proposed API — backend contract not yet confirmed**. Return task/source/destination/item/required/completed/status/version/partialPolicy.

### API-020 — Replenishment validation

**Method/path:** `POST /api/pda/v1/replenishment/{taskId}/validate`; **Proposed API — backend contract not yet confirmed**. Body includes task/source/destination/item/barcode/symbology/scanContext/quantity/baseVersion. Return resolved entities, available/capacity, nextStep; errors invalid/capacity/stock/context.

### API-021 — Replenishment confirmation

**Method/path:** `POST /api/pda/v1/replenishment/{taskId}/confirm`; **Proposed API — backend contract not yet confirmed**. Body `{commandId,idempotencyKey,taskId,sourceLocationCode,destinationLocationCode,itemId,quantity,baseVersion}`. Return partial completion/status/version/balances/auditId. Backend must define partial completion and idempotency. Local mapping: `replenish`.

### API-022 — Inventory inquiry/search/history

**Methods/paths:** `GET /api/pda/v1/inventory/items`, `/inventory/balances`, `/inventory/movements`; **Proposed API — backend contract not yet confirmed**. Query q/barcode/itemId/locationCode/lotNumber/serialNumber/lpnId/cursor/limit. Response fields: itemId/itemCode/description, locationId/locationCode, onHand, reserved, available, damaged, hold/quarantine, inTransit, UOM, lotNumber, serialNumber, asOf, version, freshness. Cache allowed with explicit stale state; no mutation. Local mapping: `findItems`, balances, `InventoryInquiryViewModel`; scanner `INVENTORY_LOOKUP`.

### API-023 — Stock transfer validation/confirmation

**Methods/paths:** `POST /api/pda/v1/transfers/validate`, `/transfers/confirm`; **Proposed API — backend contract not yet confirmed**. Validate body `{itemId,sourceLocationCode,destinationLocationCode,lotNumber?,serialNumber?,quantity,scanContext,baseVersion}`. Confirm body adds commandId/idempotencyKey/reason. Success returns transferId, balances before/after, versions, auditId. Must be atomic and online-only until approved offline policy; errors capacity/stock/version/duplicate. Local mapping: `transfer`.

### API-024 — Command status

**Method/path:** `GET /api/pda/v1/commands/{commandId}`; **Proposed API — backend contract not yet confirmed**. Returns commandId/idempotencyKey/type/status (`PENDING`, `ACKNOWLEDGED`, `CONFLICT`, `PERMANENT_FAILURE`), aggregateId, version, result reference, error code, correlationId, processedAt. Called after timeout/duplicate and by pending worker. Must be retained long enough for mobile retry and process death.

### API-025 — Cycle count task/validation/submission/recount/complete

**Methods/paths:** `GET /api/pda/v1/counts`, `/counts/{taskId}`; `POST /counts/{taskId}/validate-location`, `/validate-item`, `/lines/{lineId}/submit`, `/recount`, `/complete`; **Proposed API — backend contract not yet confirmed**. Request includes task/line/location/item/lot/serial, countedQuantity, blindCount, reasonCode, recount flag, commandId/idempotencyKey/baseVersion. Response includes systemQuantity only when non-blind, countedQuantity, variance, variance state, review/approval/recountRequired, task/version/auditId. Count variance must not silently adjust stock. Offline draft permitted; submission queue/review policy requires approval. Local mapping: `submitCount`, count entities; scanner `CYCLE_COUNT_LOCATION/ITEM`.

### API-026 — Shipment summary/readiness

**Methods/paths:** `GET /api/pda/v1/shipments`, `/shipments/{shipmentId}`, `/shipments/{shipmentId}/readiness`; **Proposed API — backend contract not yet confirmed**. Return shipmentId, salesOrderId, customer, shipTo, packageCount, verifiedPackageCount, carrierCode, trackingNumber, readinessStatus, blockingReasons, status, version, asOf. Cache read allowed; final confirmation requires fresh readiness. Local mapping: `shipments`, `ShippingReadinessProjector`.

### API-027 — Package verification

**Method/path:** `POST /api/pda/v1/shipments/{shipmentId}/packages/{packageId}/verify`; **Proposed API — backend contract not yet confirmed**. Body `{shipmentId,packageId,rawValue,normalizedValue,symbology,scanContext,operatorId,deviceId,baseVersion}`. Return verified package set, readiness, version. Errors barcode unknown, wrong context, shipment not ready, version conflict. Scanner `SHIPPING_PACKAGE`; no final shipment mutation unless verification is accepted. Local mapping: `verifyShipmentPackage`.

### API-028 — Shipment confirmation

**Method/path:** `POST /api/pda/v1/shipments/{shipmentId}/confirm`; **Proposed API — backend contract not yet confirmed**. Body `{commandId,idempotencyKey,shipmentId,carrierCode,trackingNumber,verifiedPackageIds,baseVersion}`. Carrier/tracking required and validated; readiness must be true. Success returns shipment status/version/manifest/auditId. Online-only, idempotent, no unsafe automatic retry; invalidates shipment/task/dashboard. Local mapping: `completeShipment(ShipCommand)`.

## Scanner-to-API mapping

DataWedge supplies `rawValue`, normalized barcode, symbology, source and elapsed scan timestamp through `ScanCoordinator`; app supplies taskId, lineId, warehouseId, operatorId, deviceId and active scan context. Backend must not depend on Android Intent extra names.

| Purpose | Screen/API | Expected entity | Next step |
|---|---|---|---|
| `RECEIVING_ITEM` | SCR-06/API-009 | item/receiving line | quantity |
| `PUTAWAY_SOURCE`/`ITEM`/`DESTINATION` | SCR-09/API-012/013 | location/item | review |
| `PICKING_LOCATION`/`ITEM` | SCR-11/API-016/017 | location/item/lot/serial | quantity |
| `REPLENISHMENT_SOURCE`/`ITEM`/`DESTINATION` | SCR-12/API-020 | source/item/destination | quantity |
| `INVENTORY_LOOKUP` | SCR-13/API-022 | item/location/lot/serial | inquiry |
| `TRANSFER_SOURCE`/`ITEM`/`DESTINATION` | SCR-14/API-023 | location/item | confirm |
| `CYCLE_COUNT_LOCATION`/`ITEM` | SCR-15/API-025 | location/item | count |
| `PACKING_ITEM` | generic/deferred | item | packing |
| `SHIPPING_PACKAGE` | SCR-16/API-027 | package/tracking | readiness |
| `LPN_LOOKUP`/`PALLET_LPN`/`BIN_QUERY`/`ITEM_QUERY` | deferred | container/location/item | deferred |

The app preserves leading zeros, applies context/symbology validation and deduplicates according to the active coordinator session. On validation error, scanner input remains active unless the workflow is blocked or submitting.

## Request DTO catalog

| DTO | API | Required fields | PDA source |
|---|---|---|---|
| `LoginRequest` | API-001 | username/password/device/app/locale | AuthViewModel/BuildConfig |
| `RefreshRequest` | API-002 | refresh token/device | Secure session |
| `BarcodeResolveRequest` | API-009/012/016/017/020 | raw/normalized/symbology/context/task | ScanEvent + VM state |
| `ReceiveConfirmRequest` | API-010 | command/order/line/qty/version | Receiving VM |
| `PutawayConfirmRequest` | API-014 | command/task/locations/item/qty/version | Putaway VM |
| `PickRequest` | API-018 | command/order/line/location/item/qty/version | Picking VM |
| `ReplenishmentConfirmRequest` | API-021 | command/task/source/destination/item/qty/version | Replenishment VM |
| `TransferRequest` | API-023 | command/item/source/destination/qty/version | Transfer VM |
| `CountSubmitRequest` | API-025 | command/task/line/count/version | Count VM |
| `PackageVerifyRequest` | API-027 | shipment/package/scan/version | Shipping VM |
| `ShipmentConfirmRequest` | API-028 | command/shipment/carrier/tracking/packages/version | Shipping VM |

All mutation DTOs require command identity, operator/device/warehouse context through authenticated headers, entity version and audit timestamp where contract permits. Exact nullable fields must be finalized with backend domain owners.

## Response DTO catalog

| DTO | API | Required UI fields | Cache/navigation use |
|---|---|---|---|
| `BootstrapResponse` | API-001..004 | operator/roles/warehouse/token/policy | session/profile |
| `DashboardResponse` | API-005 | counts/progress/freshness | dashboard cache |
| `TaskPage/TaskDetail` | API-007 | id/type/status/priority/title/due/version | Task Center/navigation |
| `ReceivingOrder/ResolveResponse` | API-008..010 | lines/item/remaining/policy/version | Room/draft/next step |
| `PutawayResponse` | API-011..014 | task/locations/balances/status/version | task/balance invalidation |
| `PickingOrder/PickResponse` | API-015..018 | current line/remaining/progress | order/balance refresh |
| `ReplenishmentResponse` | API-019..021 | progress/partial/status/version | task/balance refresh |
| `InventoryPage/Balance` | API-022 | item/location/quantities/asOf | stale inquiry cache |
| `TransferResponse` | API-023 | transfer/balances/versions | both balance locations |
| `CountResponse` | API-025 | count/system/variance/review | count/task, no implicit adjust |
| `ShipmentResponse` | API-026..028 | package/readiness/carrier/status/version | shipping/task/dashboard |
| `CommandStatusResponse` | API-024 | status/result/error/version | outbox reconciliation |

## Repository method-to-API mapping

| Current method | Current local behavior | Required API | Notes |
|---|---|---|---|
| `dashboardSummary`, `tasks`, `findItems` | Room/fake projections | API-005/007/022 | read-through-cache |
| `receive` | outbox then fake mutation | API-009/010/024 | durable/idempotent |
| `validatePutaway`, `completePutaway` | local validator/complete | API-012..014 | explicit state machine |
| `pick` | fake line quantity update | API-016..018 | line/version |
| `replenish` | fake task update | API-019..021 | partial rules unknown |
| `transfer` | fake balance mutation | API-023 | atomic server transaction |
| `submitCount` | fake count update | API-025 | review before adjustment |
| `pack` | fake packing update | packing API; proposed | generic/deferred UI |
| `verifyShipmentPackage`, `completeShipment` | fake verify/ship | API-026..028 | final online-only |
| LPN/pallet/adjustment methods | demo mutations | Deferred | no approved production screen |

## UI action-to-API mapping and invalidation

| Screen/action | ViewModel action | API | Refresh after success |
|---|---|---|---|
| Login submit | `signIn` | API-001 | bootstrap/profile/dashboard |
| Dashboard refresh/resume | `refresh` | API-005/007 | dashboard/task Room |
| Task search/filter/open | query/select | API-007 | task list/detail |
| Receiving scan | `resolveScan` | API-009 | no mutation; next step |
| Receiving confirm | `confirmLine` | API-010 | receiving/task/dashboard/balance |
| Putaway scan steps | `scan` | API-012/013 | no mutation; next step |
| Putaway confirm | `confirm` | API-014 | task/movement/balance |
| Pick location/item | scan events | API-016/017 | line state |
| Pick confirm | pick event | API-018 | order/balance/task |
| Replenish confirm | confirm event | API-021 | task/balances |
| Inventory lookup | `lookup` | API-022 | inquiry cache only |
| Transfer confirm | `confirm` | API-023 | source/destination balances |
| Count submit/recount | count event | API-025 | count/task/review |
| Package scan | `scanPackage` | API-027 | readiness/package |
| Shipment confirm | `confirm` | API-028 | shipment/task/dashboard |
| Logout | `logout` | API-003 | clear session/scanner/navigation |

## Cache and freshness requirements

Android local cache is Room/DataStore inside the PDA; backend shared cache (for example Redis) is never called by Android. Dashboard/tasks are minute-scale cache with stale indicator; item/location master is shift/day cache; active task and drafts are task-session cache; balances are short-lived and must show `asOf`; mutation/outbox records are durable until command status is terminal. Every list response must provide `asOf`/serverTime. A cached read may be shown offline; a mutation must show pending/conflict/failure state.

## Refresh and invalidation matrix

| Mutation | Screens | Room entities | Backend projection |
|---|---|---|---|
| Receiving | receiving/task/dashboard/inventory | task, receiving lines, balances, pending | backend WMS/inventory/task |
| Putaway | putaway/task/inventory/dashboard | movement, task, balances | location/inventory/task |
| Picking | picking/task/inventory | task/order, balances | reservation/order |
| Replenishment | replenishment/task/inventory | movement, balances | task/inventory |
| Transfer | transfer/inventory/dashboard | both balances, transfer audit | inventory/location |
| Count | count/task/inventory review | count lines/task | count approval; no implicit stock |
| Shipment | shipping/task/dashboard | shipment/packages/task | shipment/manifest |

## Offline, retry, idempotency and conflict

| Operation | Draft | Queue | Online-only | Idempotency | Version | Retry |
|---|---:|---:|---:|---:|---:|---|
| receiving | yes | policy-approved | policy-dependent | required | required | transport only |
| putaway/pick/replenish | yes | policy-approved | policy-dependent | required | required | transport only |
| inventory inquiry | no mutation | no | no | n/a | response | cached/read retry |
| transfer | yes | no until approved | yes | required | required | no unsafe auto retry |
| count | yes | draft first | submit policy-dependent | required | required | transport only |
| shipment final | yes | no until approved | yes | required | required | no unsafe auto retry |

Command ID and idempotency key are generated once in `CommandMetadata` and reused after process death. On timeout/409 duplicate, call API-024. `TASK_VERSION_CONFLICT` preserves draft and requires fresh aggregate; never overwrite a newer version. WorkManager applies network constraint and exponential backoff. `ACKNOWLEDGED` remains in Room for audit; terminal conflict/permanent failure remains visible.

## Pagination and filtering requirements

Every list endpoint uses opaque cursor, default limit 50/max 100, stable `updatedAt,id` ordering, optional status/category/q/date/priority/zone filters, and explicit empty-page semantics. Cursor must be bound to filter/sort context. Counts are optional unless backend can guarantee consistency. Current Android lists are snapshots and must be upgraded to cursor-backed Room paging only after contract approval.

## Security and audit fields

Every mutation audit records operatorId (derived from token and checked against body), deviceId, warehouseId, commandId, correlationId, task/entity ID, base/current version, client/server timestamp, result and errorCode. Raw password, access/refresh tokens, full sensitive barcode and private operator data must not be logged. TLS/certificate policy, device registration and Keystore implementation require security review.

## Backend capability checklists

### Authentication/bootstrap

- [ ] Login/OIDC and refresh contract
- [ ] Device registration and policy
- [ ] Warehouse/role/permission scope
- [ ] Server time/feature flags/scanner policy
- [ ] Logout/revocation and audit

### Dashboard/tasks

- [ ] Summary counts and freshness
- [ ] Cursor list/detail/filter
- [ ] Assignment/lock/claim policy
- [ ] Version and task status semantics

### Receiving

- [ ] List/detail and line policy
- [ ] Barcode alias/lot/serial resolution
- [ ] Quantity/condition/under-over receipt rules
- [ ] Idempotent confirm, version conflict, command status
- [ ] Inventory/task/audit transaction

### Putaway/picking/replenishment

- [ ] Assignment and lock validation
- [ ] Location/item/stock/capacity validation
- [ ] Partial/short completion rules
- [ ] Versioned idempotent mutation and audit

### Inventory/transfer/count

- [ ] Search dimensions and freshness
- [ ] Atomic transfer and balance versions
- [ ] Blind count/variance/recount/approval
- [ ] No implicit count stock adjustment

### Shipping

- [ ] Shipment/package/readiness reads
- [ ] Package barcode validation
- [ ] Carrier/tracking validation
- [ ] Online-only final confirmation, manifest and audit

## Backend comparison readiness

| Requirement | Backend inspected | Status | Evidence | Gap |
|---|---:|---|---|---|
| Auth/bootstrap | No | Backend implementation not inspected | PDA_BACKEND not supplied | OIDC/device contract |
| Dashboard/tasks | No | Backend implementation not inspected | — | paging/freshness |
| Receiving | No | Backend implementation not inspected | — | policy/idempotency/version |
| Movement | No | Backend implementation not inspected | — | locks/capacity/stock |
| Inventory/transfer/count | No | Backend implementation not inspected | — | atomicity/review |
| Shipping | No | Backend implementation not inspected | — | carrier/manifest |

## Unconfirmed business rules

| Rule | Affected APIs | Safe temporary behavior | Decision owner |
|---|---|---|---|
| over/under receipt tolerance | API-009/010 | validate locally; preserve draft | WMS receiving |
| lot/serial/condition capture | receiving/pick/count | require explicit policy; no inference | WMS master data |
| location capacity/reservation | putaway/replenish/transfer | server validation required | inventory |
| picking short-pick | API-018 | no automatic short completion | outbound |
| replenishment partial completion | API-021 | preserve partial draft | inventory |
| blind count visibility | API-025 | hide system quantity when blind | inventory control |
| variance approval/recount | API-025 | do not adjust stock silently | finance/inventory |
| task lock duration/claim | API-007 and workflows | reject stale/locked action | operations |
| shipment offline policy | API-028 | online-only | shipping |
| command status retention | API-024 | retain local command indefinitely until resolved | backend platform |
| device registration | API-001/004 | block writes when unregistered | security/IT |

## Required test plan

Unit tests: request validation, response mappers, error/status mapper, header generation, scanner request construction, version conflict, command identity reuse, redaction and retry policy. Integration tests with a fake HTTP server: login/refresh single-flight, pagination/filter, every barcode resolve, success/error envelopes, duplicate command, stale version, 401/403/404/409/422/429/5xx and malformed data. Room/API tests: remote reconciliation, stale display, outbox restart/process death, command-status recovery. Instrumentation: cached dashboard offline, each workflow scan context, modal scanner suspension, mutation invalidation, logout. Zebra E2E: DataWedge profile, hardware trigger, leading zeros/symbology, duplicate scan guard, reconnect and no duplicate writes. Backend comparison tests are blocked until PDA_BACKEND/OpenAPI is supplied.

## Appendices

### Appendix A — Complete API list

API-001 login; API-002 refresh; API-003 logout; API-004 bootstrap/profile; API-005 dashboard; API-006 operator profile; API-007 tasks; API-008 receiving list/detail; API-009 receiving resolve; API-010 receiving confirm; API-011 putaway list/detail; API-012 source/item validation; API-013 destination validation; API-014 putaway confirm; API-015 picking list/detail; API-016 location validation; API-017 item resolution; API-018 pick/short/complete; API-019 replenishment list/detail; API-020 replenishment validation; API-021 replenishment confirm; API-022 inventory query; API-023 transfer; API-024 command status; API-025 cycle count; API-026 shipment/readiness; API-027 package verify; API-028 shipment confirm.

### Appendix B — Request DTO catalog

`LoginRequest`, `RefreshRequest`, `BootstrapRequest`, `TaskQuery`, `BarcodeResolveRequest`, `ReceiveConfirmRequest`, `PutawayValidateRequest`, `PutawayConfirmRequest`, `PickRequest`, `ReplenishmentConfirmRequest`, `InventoryQuery`, `TransferRequest`, `CountSubmitRequest`, `PackageVerifyRequest`, `ShipmentConfirmRequest`, and `CommandStatusQuery`. All names are proposed; fields are defined in the API cards.

### Appendix C — Response DTO catalog

`BootstrapResponse`, `DashboardResponse`, `TaskPage`, `TaskDetail`, `ReceivingOrder`, `BarcodeResolveResponse`, `PutawayResponse`, `PickingOrder`, `PickResponse`, `ReplenishmentResponse`, `InventoryPage`, `Balance`, `TransferResponse`, `CountResponse`, `ShipmentResponse`, `CommandStatusResponse`, and `ErrorResponse`.

### Appendix D — Error code catalog

See the complete catalog above. Backend must return stable machine codes, safe details, correlation ID, retryability and conflict version where applicable.

### Appendix E — Screen-to-API map

See **Screen-to-API inventory** and **UI action-to-API mapping**. Dedicated screens are SCR-01 through SCR-16; SCR-17 is generic/deferred.

### Appendix F — Scanner-to-API map

See **Scanner-to-API mapping**. DataWedge Intent extras are normalized by `DataWedgeReceiver`/`ScanCoordinator`; backend receives domain-neutral scan fields.

### Appendix G — Repository-to-API map

See **Repository method-to-API mapping**. `WarehouseRepository` is the migration boundary; local mock methods are not evidence of backend support.

### Appendix H — Mock-to-backend replacement map

`MockAuthRepository` → approved auth repository; `MockWarehouseRepository` → remote API data source; `MockWarehouseDatabase` → Room sync source; fake mutations → versioned idempotent commands; endpoint-neutral workers → approved dispatcher/command-status API; `RejectingSecureSecretStore` → reviewed Keystore implementation.

### Appendix I — Unconfirmed rules

All questions in the business-rule table require explicit backend/WMS ownership decisions before enabling production writes.

### Appendix J — Source evidence map

- `app/build.gradle.kts`, `gradle/libs.versions.toml`, `app/src/main/AndroidManifest.xml`
- `navigation/WmsRoute.kt`, `navigation/WmsNavigator.kt`, `ui/AppRoot.kt`
- `auth/*`, `security/SecureSecretStore.kt`, `data/remote/*`
- `data/repository/WarehouseRepository.kt`, `RoomBackedWarehouseRepository.kt`
- `data/mock/MockWarehouseRepository.kt`, `MockWarehouseDatabase.kt`, `MockDataFactory.kt`
- `data/local/WmsDatabase.kt`, `WmsEntities.kt`, `WmsDaos.kt`, `RoomSnapshotSynchronizer.kt`
- `scanner/DataWedgeReceiver.kt`, `scanner/ScanCoordinator.kt`
- active `ui/*` screens and `feature/*ViewModel.kt`
- `docs/enterprise-pda/01_WMS_PDA_Functional_UI_Specification.md`
- `docs/enterprise-pda/02_WMS_PDA_Peripheral_API_Integration_Specification.md`
- `docs/enterprise-pda/execution/PDA_APP_AI_IMPLEMENTATION_RULES.md`
- `NEW_AI_CONTEXT.md`, previous API integration requirements/report

## Final validation checklist

- [x] Every active target screen and deferred/generic path identified.
- [x] Every current repository method used by active workflows mapped.
- [x] Every scanner purpose mapped to validation/read/mutation APIs.
- [x] Every API has purpose, method/path, headers, fields, response/error, cache, retry, idempotency and conflict guidance.
- [x] Proposed APIs clearly marked unconfirmed; no backend support claimed.
- [x] Request and response DTO catalogs included.
- [x] Offline, refresh, invalidation, pagination, security and audit requirements included.
- [x] Unconfirmed business rules and tests included.
- [ ] Backend OpenAPI comparison — blocked because PDA_BACKEND was not supplied.
