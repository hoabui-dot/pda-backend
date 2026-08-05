# PDA Backend 28-API Traceability Matrix

- Status: **Mapped against PDA specification dated 2026-08-02**
- Current implementation evidence: backend source, `api/openapi/pda-v1.yaml`, migrations, and tests
- Proposed PDA paths remain proposed; the decision register freezes the backend compatibility approach

| API | PDA proposed path/method | Current backend path/method | Request comparison | Response comparison | Error/version/idempotency | Status |
|---|---|---|---|---|---|---|
| 001 | `POST /auth/login` | `POST /auth/login` | Backend only username/password; PDA adds device/model/app/warehouse/locale | Backend token/operator; PDA adds refresh expiry/profile/policy | auth/rate limit; no body device policy | CONTRACT_MISMATCH |
| 002 | `POST /auth/refresh` | `POST /auth/refresh` | PDA body refreshToken/device; backend uses bearer token | Backend access token only; PDA requires rotated session fields | 401 supported; rotation exists | CONTRACT_MISMATCH |
| 003 | `POST /auth/logout` | `POST /auth/logout` | PDA session/device revoke fields not represented | Backend returns 204; common envelope requirement differs | 401 supported | CONTRACT_MISMATCH |
| 004 | `GET /bootstrap` | `GET /bootstrap` | Device/warehouse headers supported | Backend operator/warehouses/device IDs; capabilities empty, policy fields missing | 401/403/device errors | PARTIALLY_SUPPORTED |
| 005 | `GET /dashboard` | `GET /dashboard` | Backend uses trusted headers; PDA proposes query scope | Backend counts/updatedAt; PDA needs progress/alerts/pending sync/asOf | auth/scope; cached read | PARTIALLY_SUPPORTED |
| 006 | `GET /me` | `GET /me` | no body | Backend operator fixture; PDA needs employee/shift/active/permission detail | auth/scope | PARTIALLY_SUPPORTED |
| 007 | task list/detail/claim/release | list, claim, release; no detail | filters partially supported; claim uses headers/idempotency/If-Match | task fields lack title/line/piece/due/lock detail; detail missing | idempotency/version/lock supported | PARTIALLY_SUPPORTED |
| 008 | receiving list/detail | `/receiving/tasks`, `/receiving/tasks/{id}` | status/cursor/limit; supplier/q/priority/date missing | domain task/lines exist; PDA policy/freshness fields missing | scope/version in mutations | CONTRACT_MISMATCH |
| 009 | `/receiving/{id}/resolve-barcode` | `/receiving/tasks/{id}/barcode-resolutions` | backend accepts barcode only; PDA requires raw/normalized/symbology/context/timestamp | backend line; PDA resolved policy/next-step/version fields missing | barcode errors supported | CONTRACT_MISMATCH |
| 010 | `/receiving/{id}/confirm` | `/receiving/tasks/{id}/receipts` | backend command body includes line/barcode/qty/remark; PDA adds condition/scannedAt and explicit command fields | backend task; PDA requires command status/audit/remaining/meta | idempotency/version/outbox supported | CONTRACT_MISMATCH |
| 011 | putaway list/detail | `/putaway/tasks`, `/putaway/tasks/{id}` | no PDA cursor contract in current route | task exists; balance/assignment response fields not normalized | reads scoped | CONTRACT_MISMATCH |
| 012 | `/putaway/{id}/validate-source` | `/putaway/tasks/{id}/source-validations` | value/location/barcode accepted; PDA requires task/item/scanner/baseVersion | task response; PDA resolved location/item/nextStep fields missing | idempotency/version currently required | CONTRACT_MISMATCH |
| 013 | `/putaway/{id}/validate-destination` | destination validation + suggestions | current body generic value; PDA requires destination/item/LPN/qty/version | suggestions/task exist; accepted/capacity/nextStep not explicit | validation/version supported | CONTRACT_MISMATCH |
| 014 | `/putaway/{id}/confirm` | `/putaway/tasks/{id}/confirmation` | current quantity only; PDA requires full movement identity | task returned; balances/audit/meta not explicit | idempotent/version/outbox supported | CONTRACT_MISMATCH |
| 015 | picking list/detail | `/picking/tasks`, `/picking/tasks/{id}` | filters absent | task exists; order/customer/current-line/readiness fields not normalized | scope/version | CONTRACT_MISMATCH |
| 016 | validate location | `/picking/tasks/{id}/location-validations` | current generic value; PDA requires order/line/raw/symbology/context/version | task result; expected/accepted/nextStep absent | idempotent/version | CONTRACT_MISMATCH |
| 017 | resolve item | `/picking/tasks/{id}/barcode-resolutions` | current generic barcode; PDA adds lot/serial/scanner fields | task result; resolved item/available/constraints absent | validation/version | CONTRACT_MISMATCH |
| 018 | pick/short-pick/complete | picks/completion | quantity exists; short-pick reason and explicit command body absent | task exists; current line/remaining/progress/readiness impact absent | idempotent/version | CONTRACT_MISMATCH |
| 019 | replenishment list/detail | `/replenishment/tasks`, `/replenishment/tasks/{id}` | filters absent | task exists; partial policy not explicit | scope/version | CONTRACT_MISMATCH |
| 020 | replenishment validate | three nested validations | PDA consolidated scanner/quantity contract differs | task result; available/capacity/nextStep absent | idempotent/version | CONTRACT_MISMATCH |
| 021 | replenishment confirm | `/replenishment/tasks/{id}/confirmation` | current quantity only; PDA full source/destination/item command | task exists; partial/balance/audit response absent | idempotent/version/outbox | CONTRACT_MISMATCH |
| 022 | inventory item/balance/history | search/balances/movements | item/location only; lot/serial/LPN/cursor limit incomplete | balance/movement exists; reserved/available/asOf/stale missing | read cache exists | CONTRACT_MISMATCH |
| 023 | transfer validate/confirm | validation routes + `POST /inventory/transfers` | current transfer body lacks command/reason/baseVersion in body | valid flag or result; before/after balances absent | idempotency exists; status absent | CONTRACT_MISMATCH |
| 024 | `GET /commands/{id}` | receiving/shipping scoped command status only | generic command query absent | current scoped result differs from required status enum/result fields | durable records partial | NOT_SUPPORTED |
| 025 | count list/detail/validation/submission/recount/complete | list/detail/count/recount/complete | count body only line/quantity; PDA blind/variance/reason/command fields | domain variance exists; review/approval/recount metadata incomplete | idempotency/version exists | PARTIALLY_SUPPORTED |
| 026 | shipment list/detail/readiness | shipment detail/readiness | list route absent | shipment/readiness domain exists; package freshness/list fields missing | scoped read | PARTIALLY_SUPPORTED |
| 027 | package verify | no route | entire request unsupported | entire response/use case unsupported | no package command | NOT_SUPPORTED |
| 028 | `/shipments/{id}/confirm` | `/shipments/{id}/confirmation` | backend carrier/tracking; PDA adds command/shipment/verified packages/version | shipment returned; manifest/audit/command status not normalized | idempotency/version/status supported | CONTRACT_MISMATCH |

## Header comparison

`Authorization`, `X-Correlation-Id`, `X-Device-Id`, `X-Warehouse-Id`, `Idempotency-Key`, and
`If-Match` are implemented for the applicable backend routes. `X-Operator-Id` is not authoritative
and is not currently required; operator identity is token-derived. `Accept-Language` is not currently
validated or used. The PDA-required `Content-Type` is enforced implicitly by JSON decoding but is not
strictly checked.

## Envelope comparison

Success responses use `data/meta/errors`; normal gateway failures now use the same envelope. Backend
meta currently contains `serverTime` and `correlationId`; `version`, `nextCursor`, `hasMore`, `asOf`,
and `stale` are not consistently present. PDA's legacy error example is top-level while the canonical
rule requires envelope errors; backend canonical decision is the envelope form.

