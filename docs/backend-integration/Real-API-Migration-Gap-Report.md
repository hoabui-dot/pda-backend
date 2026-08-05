# Real API Migration Gap Report

## Scope

This report records every production capability that is not yet backed by an implemented PDA client API after removing the mock/demo binding from `WmsAppViewModel` and `AuthViewModel`. It follows `docs/enterprise-pda/integration/PDA_APP_BACKEND_INTEGRATION_IMPLEMENTATION_GUIDE.md`.

## Completed real-API changes

- Production warehouse binding now uses `ApiWarehouseRepository`; there is no `MockWarehouseRepository` fallback in `WmsAppViewModel`.
- Production authentication binding now uses `RemoteAuthRepository`; demo credentials are no longer prefilled or rendered by `LoginScreen`.
- Gateway environment is injected through Gradle properties; no host/port is hardcoded.
- Canonical auth/bootstrap/dashboard/task Retrofit service and DTO envelope exist.
- HTTP headers and redacted logging are implemented.
- Unsupported warehouse mutations return typed `READ_ONLY` failure instead of changing local demo state.
- Generic status labels no longer advertise “local demo” in the active dashboard/module status component.

## API capabilities currently implemented in the PDA client

| Capability | Client status | Backend route from guide | Runtime binding |
|---|---|---|---|
| Login | implemented | `POST /auth/login` | `RemoteAuthRepository` |
| Refresh | DTO/service only | `POST /auth/refresh` | not wired; secure session pending |
| Logout | DTO/service only | `POST /auth/logout` | local auth flow only |
| Bootstrap | service/DTO only | `GET /bootstrap` | not wired into session/context |
| Dashboard | service/DTO only | `GET /dashboard` | dashboard repository mapping pending |
| Task list/detail | service/DTO + task fetch | `GET /tasks`, `/tasks/{taskId}` | task projection only |

## Unsupported or incomplete API capabilities

The following must not use local mock behavior in production. They currently return typed `READ_ONLY`/empty state or are not wired, and require implementation before their UI actions can be enabled.

### Authentication/session

- Secure Keystore-backed access/refresh token storage.
- Single-flight refresh coordinator and rotated refresh token handling.
- Bootstrap context persistence (operator, warehouse, permissions, scanner policy).
- Device registration and warehouse selection validation.
- Logout/revocation API invocation.

### Read APIs

- Dashboard response mapping into Room and UI freshness state.
- `/me` and `/me/warehouses` profile reads.
- Task claim/release and command status.
- Receiving task list/detail.
- Putaway task list/detail.
- Picking task list/detail.
- Replenishment task list/detail.
- Inventory item, balance and movement search.
- Cycle count task/detail.
- Shipment summary/readiness.
- LPN, pallet, package contents and audit reads.

### Scanner validation APIs

- Receiving barcode resolution.
- Putaway source/item/destination validation and destination suggestions.
- Picking location and item/lot/serial resolution.
- Replenishment source/item/destination validation.
- Inventory barcode/item/location/lot/serial query.
- Transfer source/item/destination validation.
- Cycle count location/item validation.
- Shipping package verification.

### Mutation APIs

- Receiving receipt and receiving completion.
- Putaway confirmation.
- Pick, short-pick exception and picking completion.
- Replenishment confirmation and partial completion.
- Atomic stock transfer.
- Cycle count submission, recount and completion.
- Packing confirmation.
- Shipment final confirmation.
- LPN/pallet/adjustment operations.

### Offline/sync

- Production API dispatcher for pending commands.
- Command-status reconciliation after timeout/duplicate response.
- Server-authoritative Room reconciliation for real API responses.
- API-driven conflict/current-version refresh.
- Network-constrained WorkManager submission of approved idempotent commands.

## Mock/demo code intentionally retained outside production binding

Existing mock repository, deterministic fixtures and mock-based tests remain in source because current unit/instrumentation suites use them as test doubles. They are not selected by `WmsAppViewModel` or `AuthViewModel`. Remaining demo-only sources requiring a separate cleanup PR are:

- `data/mock/*` and `data/repository/MockWarehouseRepository.kt`.
- `auth/MockAuthRepository.kt` and mock-based tests.
- Preview providers using `MockWarehouseRepository`.
- Generic deferred module screens and legacy unreachable `InventoryScreen` containing demo/manual operation labels.
- Demo-oriented string resources retained for deferred/test-only screens.

These files must not be deleted until tests are migrated to API fakes and deferred routes are removed or explicitly retained as non-production source sets. No production navigation path may bind them.

## Backend connectivity blocker

The connected TC26 (`24144523021841`) could not reach the guide-provided backend address: `adb shell ping -c 1 100.68.50.41` returned 100% packet loss. The guide states gateway host port is not published. Therefore no real request was sent and no API success is claimed. Deployment must provide a published host port or HTTPS reverse-proxy/DNS URL plus Tailscale ACL/firewall access.

## Backend request/response contract handoff

This section is the update contract for `PDA_BACKEND`. Paths are canonical paths from the integration guide. The PDA client currently implements only the auth/dashboard/task service subset; all other rows are required backend capabilities and client gaps.

### Required request headers

| Header | Required on | Example | Validation and retry |
|---|---|---|---|
| `Authorization` | protected reads/writes | `Bearer eyJ...` | active session; refresh once on 401 |
| `X-Correlation-Id` | every request | UUID | echo in response/audit; preserve on retry |
| `X-Device-Id` | login/bootstrap/workflow | `TC26-24144523021841` | registered device; preserve |
| `X-Warehouse-Id` | warehouse calls | `MAIN` | must be allowed for token |
| `X-Operator-Id` | approved workflow calls | `EMP-0001` | compare with token; body is not authoritative |
| `Idempotency-Key` | every mutation | UUID | stable per command; never regenerate |
| `If-Match` | mutable aggregate | `"11"` | compare version; 409 on mismatch |
| `Accept-Language` | user-facing response | `vi-VN` | allow-list locale |
| `Content-Type` | JSON requests | `application/json` | reject unsupported media |

### Standard success and error envelopes

```json
{"data":{},"meta":{"serverTime":"2026-08-02T02:00:00Z","correlationId":"uuid","version":"12","nextCursor":null,"hasMore":false,"asOf":"2026-08-02T02:00:00Z","commandStatus":"ACKNOWLEDGED"},"errors":[]}
```

`data` is the typed resource; `serverTime` is the authoritative clock; `correlationId` is echoed for support; `version` is the aggregate version; cursor fields paginate; `asOf` drives stale UI; `commandStatus` is required for mutations. A standard error is:

```json
{"code":"TASK_VERSION_CONFLICT","message":"Task changed by another operator.","details":{"taskId":"REC-001","currentVersion":"12"},"correlationId":"uuid","retryable":false,"retryAfterSeconds":null}
```

`code`, `correlationId` and `retryable` are mandatory. `message` is diagnostic/localization fallback and must not contain secrets. `details` contains safe IDs, field errors or current version. HTTP 400/401/403/404/409/422/429/500/503 map to malformed/session/permission/missing/conflict/business/rate-limit/server/dependency behavior; the PDA must never retry unsafe writes blindly.

### Canonical route and client-status matrix

| API | Method/path | Request body/query | Expected response | Client status |
|---|---|---|---|---|
| API-001 | `POST /auth/login` | credentials + device/app/warehouse/locale | session/token/profile | client implemented; unreachable |
| API-002 | `POST /auth/refresh` | refreshToken, deviceId | rotated session | service only; secure store missing |
| API-003 | `POST /auth/logout` | approved session body | 204/command status | service only; not wired |
| API-004 | `GET /bootstrap` | headers | operator/warehouse/policy/flags | service only; not wired |
| API-005 | `GET /dashboard` | warehouse/operator query | counts/progress/freshness | DTO only; Room mapping missing |
| API-006 | `GET /me`, `/me/warehouses` | headers/query | profile/permitted sites | missing |
| API-007 | `GET /tasks`, `GET /tasks/{id}` | cursor,limit,status,type,q,priority,zone | page/detail/version/lock | list service only |
| API-008 | `GET /receiving/tasks`, `/{id}` | cursor/status/supplier/q | task/lines/policy/version | missing |
| API-009 | `POST /receiving/tasks/{id}/barcode-resolutions` | scan body | resolved line/quantity policy/next step | missing |
| API-010 | `POST /receiving/tasks/{id}/receipts` | receipt command | updated task/line/status/version/audit | missing |
| API-011 | `GET /putaway/tasks`, `/{id}` | cursor/status | task/detail/version | missing |
| API-012 | `POST /putaway/tasks/{id}/source-validations` | source/item scan | acceptance/quantity/next step | missing |
| API-013 | `GET/POST /putaway/tasks/{id}/destination-validations` | destination/quantity | capacity/accepted/suggestions | missing |
| API-014 | `POST /putaway/tasks/{id}/confirmation` | command | task/balances/version/audit | missing |
| API-015 | `GET /picking/tasks`, `/{id}` | cursor/status | order/current line/progress | missing |
| API-016 | `POST /picking/tasks/{id}/location-validations` | line/location scan | accepted/next step | missing |
| API-017 | `POST /picking/tasks/{id}/barcode-resolutions` | item/lot/serial scan | resolved item/availability | missing |
| API-018 | `POST /picking/tasks/{id}/picks` | pick command | line progress/next line/version | missing |
| API-019 | `GET /replenishment/tasks`, `/{id}` | cursor/status | task/partial policy | missing |
| API-020 | `POST /replenishment/tasks/{id}/validations` | source/item/destination scan | accepted stock/capacity | missing |
| API-021 | `POST /replenishment/tasks/{id}/confirmation` | command | partial/full task/balances | missing |
| API-022 | `GET /inventory/search`, `/balances`, `/movements` | q/barcode/item/location/lot/serial/LPN/cursor | balances/movements/asOf | missing |
| API-023 | `POST /inventory/transfers/validations`, `/inventory/transfers` | validation/command | atomic transfer/both balances | missing |
| API-024 | `GET /commands/{commandId}` | path commandId | status/result/error/version | missing |
| API-025 | count task/read/validate/submit/recount/complete | count body | count/variance/review/version | missing |
| API-026 | `GET /shipments/{id}`, `/readiness` | shipment ID | package/readiness/blockers/version | missing |
| API-027 | `POST /shipments/{id}/packages/{packageId}/verify` | package scan | verified set/readiness | missing |
| API-028 | `POST /shipments/{id}/confirmation` | shipment command | status/manifest/version/audit | missing |

### Request body contract

Login:

```json
{"username":"warehouse.operator","password":"REDACTED","deviceId":"TC26-24144523021841","deviceModel":"TC26","appVersion":"1.0","warehouseId":"MAIN","locale":"vi-VN"}
```

Scanner validation (all workflow variants):

```json
{"taskId":"REC-001","lineId":"LINE-001","rawValue":"00012345678905","normalizedValue":"00012345678905","symbology":"EAN-13","scanContext":"RECEIVING_ITEM","operatorId":"EMP-0001","warehouseId":"MAIN","deviceId":"TC26-24144523021841","baseVersion":11,"scannedAt":"2026-08-02T02:00:00Z"}
```

The backend must preserve leading zeroes, validate active task/line/context/symbology, resolve aliases, enforce lot/serial policy and return the next workflow step. `rawValue`/`normalizedValue`/`symbology` originate in DataWedge; task/line/operator/warehouse/device/version originate in app/session state.

Receiving receipt:

```json
{"commandId":"uuid","idempotencyKey":"uuid","taskId":"REC-001","lineId":"LINE-001","barcode":"00012345678905","quantity":3,"condition":"GOOD","remark":null,"baseVersion":11,"scannedAt":"2026-08-02T02:00:00Z"}
```

Putaway confirmation:

```json
{"commandId":"uuid","idempotencyKey":"uuid","taskId":"PUT-001","sourceLocationCode":"STAGE-01","destinationLocationCode":"BIN-A-01","itemId":"ITEM-001","lpnId":null,"quantity":3,"baseVersion":5}
```

Pick confirmation:

```json
{"commandId":"uuid","idempotencyKey":"uuid","taskId":"PICK-001","lineId":"LINE-001","locationCode":"PICK-A-01","itemId":"ITEM-001","lotNumber":null,"serialNumber":null,"quantity":2,"baseVersion":7,"shortPickReason":null}
```

Replenishment confirmation:

```json
{"commandId":"uuid","idempotencyKey":"uuid","taskId":"REP-001","sourceLocationCode":"RESERVE-01","destinationLocationCode":"PICK-A-01","itemId":"ITEM-001","quantity":5,"baseVersion":3}
```

Transfer confirmation:

```json
{"commandId":"uuid","idempotencyKey":"uuid","itemId":"ITEM-001","sourceLocationCode":"A-01","destinationLocationCode":"B-01","lotNumber":null,"serialNumber":null,"quantity":4,"reasonCode":"REPLENISHMENT","baseBalanceVersion":8}
```

Cycle count submission:

```json
{"commandId":"uuid","idempotencyKey":"uuid","taskId":"CC-001","lineId":"CCL-001","locationCode":"BIN-A-01","itemId":"ITEM-001","lotNumber":null,"serialNumber":null,"countedQuantity":12,"blindCount":true,"reasonCode":null,"baseVersion":4}
```

Package verification and shipment confirmation:

```json
{"shipmentId":"SHP-001","packageId":"PKG-001","rawValue":"PKG-001","normalizedValue":"PKG-001","symbology":"CODE128","scanContext":"SHIPPING_PACKAGE","baseVersion":2}
```

```json
{"commandId":"uuid","idempotencyKey":"uuid","shipmentId":"SHP-001","carrierCode":"DHL","trackingNumber":"TRACK-001","verifiedPackageIds":["PKG-001","PKG-002"],"baseVersion":3}
```

### Expected response contract

Every read/detail response must contain stable IDs, warehouse scope, status, assignment where relevant, version, `updatedAt`, `serverTime`, `correlationId` and `asOf`. Every mutation must return commandId/idempotencyKey, `commandStatus` (`ACKNOWLEDGED`, `PENDING`, `DUPLICATE`, `CONFLICT`, `REJECTED`), authoritative aggregate, new version, auditId and serverTime.

Receiving response additionally requires order/line/item IDs, expected/received/remaining quantities, condition/lot/serial policy and task status. Putaway/replenishment responses require source/destination balances and moved/remaining quantities. Picking requires currentLine, quantityToPick, quantityPicked, remainingQuantity, nextLineId, taskProgress and shipmentReadinessImpact. Inventory requires item/location/lot/serial dimensions, onHand/reserved/available/damaged/quarantine/hold/inTransit/UOM and `asOf`. Count requires counted/system/variance/recountRequired/approvalRequired (hide system quantity for blind count). Shipping requires packageCount, verifiedPackageCount, readinessStatus, blockingReasons, carrier/tracking and manifest.

### Required backend error behavior

| Workflow | Required errors | PDA behavior |
|---|---|---|
| Auth | invalid credentials, session expired, device not registered, warehouse denied | inline error/refresh/block |
| Receiving | barcode unknown/wrong context, item not in document, quantity exceeds, version conflict, duplicate | keep scanner/draft; conflict reload; command lookup |
| Putaway | invalid source/destination, capacity, stock, locked/version conflict | preserve step; block confirm |
| Picking | location/item mismatch, stock, short-pick policy, locked/version conflict | preserve line; explicit exception only |
| Replenishment | invalid locations, capacity/stock, partial policy, version conflict | preserve remainder |
| Transfer | invalid locations, stock/capacity, version/duplicate | no local stock mutation |
| Count | wrong context, variance review, locked/version conflict | preserve count; no implicit adjustment |
| Shipping | unknown/wrong package, not ready, carrier invalid, version/duplicate | keep verification; final online-only |

### Backend update checklist

- [ ] Confirm canonical paths and aliases against OpenAPI.
- [ ] Confirm header mandatory rules, token subject/operator mapping and device identity format.
- [ ] Confirm all request JSON fields and nullable/enum rules above.
- [ ] Return correlationId, serverTime, version, auditId and commandStatus on mutations.
- [ ] Implement duplicate idempotency replay and command-status retention.
- [ ] Return current aggregate/version in 409 conflict details.
- [ ] Define barcode, symbology, lot/serial, location capacity and quantity policies.
- [ ] Define offline eligibility per mutation; PDA default is online-only when uncertain.
- [ ] Publish gateway port or HTTPS reverse proxy and verify `/healthz`/`/readyz` from TC26.
- [ ] Supply OpenAPI contract fixtures for API-001 through API-028.

## Required next implementation order

1. Supply and verify gateway URL with `/healthz` and `/readyz` from TC26.
2. Implement Keystore session and refresh coordinator.
3. Wire bootstrap/context and dashboard/task reads into Room.
4. Implement receiving read/resolve/confirm and command status.
5. Implement putaway, picking, replenishment, inventory, transfer, count and shipping API clients one workflow at a time.
6. Wire outbox dispatcher and remove unsupported action controls from UI until each route is implemented.
7. Migrate mock tests to HTTP fakes and remove production mock/demo source.

## Verification after migration changes

- `testDebugUnitTest assembleDebug`: passed.
- Full TC26 instrumentation after production-binding cleanup: 23/23 passed.
- Final APK was reinstalled and cold-launched after the connected test; `MainActivity` resumed with no fatal runtime logs.
