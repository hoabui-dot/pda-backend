# PDA App Runtime Blocker

Date: 2026-08-07
Status: `PARTIALLY_APPROVED_BLOCKED_BY_RUNTIME_QUALIFICATION_AND_OWNER_CONTRACT_GAPS`

## Evidence

Using the running PDA gateway with the supplied disposable account and its
registered context:

- `POST /api/pda/v1/auth/login`: `200` for `admin` with warehouse `MAIN` and
  device `TC26-24144523021841`.
- `GET /api/pda/v1/bootstrap`: `200`.
- `GET /api/pda/v1/dashboard`, `/tasks/summary`, and
  `/tasks?category=RECEIVING&limit=20`: `200`.
- Generic task detail and idempotent claim for
  `d8032938-8ef5-41ca-80e6-0b28592b62d3`: `200`; status `ASSIGNED`, version `1`.
- Receiving detail and start for the same discovered task: `TASK_NOT_FOUND`.
- The mock receiving fixture IDs `REC-001` and `REC-002`: `WAREHOUSE_ACCESS_DENIED`
  under the `MAIN` session because the fixtures use `WH-01`.

## Historical Root Cause

The running composition uses `PDA_UPSTREAM_WMS_MODE=mock`. The gateway startup
currently refuses non-mock mode until a verified HTTP WMS adapter and its
runtime contract are supplied. The mock task projection and receiving fixture
also do not share a resolvable task identity/warehouse scope for the live
session.

This prevents a real PDA-to-WMS receiving mutation qualification. The generic
authentication and task projection checks are not sufficient evidence of the
WMS workflow.

## Historical Required Resolution

Provide and approve:

1. WMS-backed PDA adapter base URL, authentication, and route contract;
2. task identity mapping between task discovery and receiving detail;
3. warehouse mapping for the `MAIN` PDA context;
4. a disposable PDA/WMS test dataset or supported public fixture API;
5. Android emulator/device access if UI-level evidence is required.

No direct database access, direct WMS service bypass, or mock-to-production
configuration switch was used.

## Implementation Update

Completed:

- Added the gateway-level `ReceivingOperations` use-case port.
- Extracted gateway-level ports for generic tasks, movement workflows,
  inventory, and shipping so the router no longer requires their concrete
  local application services.
- Kept the existing local Receiving service compatible with that boundary.
- Added the WMS HTTP Receiving adapter for existing-receipt discovery, detail,
  quantity recording, and confirmation.
- Added Master Data location-to-warehouse authorization in the remote adapter.
- Used the authoritative receipt-line version for the quantity command.
- Verified that the remote adapter contains no receipt-creation call.

Composition update:

- HTTP mode now composes the real WMS Receiving adapter and explicit
  fail-closed adapters for unmapped capabilities.
- HTTP mode does not construct or seed local task, receiving, movement,
  inventory, shipping, or WMS-task stores.
- Unsupported generic operations return
  `UPSTREAM_OPERATION_NOT_IMPLEMENTED`; they do not fall back to local data.

Current implementation status:

- The gateway now depends on gateway-level use-case ports rather than concrete
  local repository-backed services.
- HTTP mode composes WMS HTTP adapters for Receiving, task reads/claim/release,
  movement scans/confirm, Inventory reads/internal transfer, and Shipping
  shipment reads. It does not construct or seed local business stores.
- Receiving uses existing Inbound receipts, records quantity against the
  authoritative receipt line, and confirms the same receipt. No receipt-create
  call exists in the remote adapter.
- Unmapped operations fail closed with `UPSTREAM_OPERATION_NOT_IMPLEMENTED`.
- Docker Compose now passes `PDA_UPSTREAM_WMS_BASE_URL`,
  `PDA_UPSTREAM_WMS_TOKEN`, and `PDA_UPSTREAM_WMS_SERVICE_TOKEN` into the
  gateway when HTTP mode is selected; configuration validation still rejects
  empty values at application startup.
- Docker Compose now runs the PDA migrations as an owned one-shot service
  before the gateway and integration-event service start.

## Remaining blockers

1. A separate production run of this `pda-backend` HTTP composition has not
   been executed against the running WMS stack. The available running PDA
   endpoint belongs to the WMS repository and is not the separate gateway's
   `/api/pda/v1` process. Therefore no PDA HTTP/Kafka/Inventory runtime pass is
   claimed.
   The currently running WMS owner containers also do not expose a configured
   `PDA_BACKEND_SERVICE_TOKEN`, so service-authenticated qualification cannot
   safely start until the deployment supplies the same secret to the owners and
   the separate PDA gateway.
2. Generic Master Data barcode resolution for item/lot scans is not exposed as
   a complete owner contract in the current adapter. Location ownership is
   authoritative and mapped; item/lot resolution remains fail-closed rather
   than implemented by collection filtering.
3. Shipping package verification returns package-only data while the PDA port
   requires a shipment aggregate result. Shipment confirmation also does not
   accept the PDA carrier/tracking/verified-package fields. Mapping either by
   dropping fields would change semantics, so those operations remain blocked
   by owner-contract mismatch.
4. Cycle-count and other Inventory mutation ports require separate owner
   commands not present in the audited WMS HTTP contract.

## Verification Update

`GOWORK=off go mod tidy` followed by `GOWORK=off go test ./...` passes.
`node --test tests/contracts/*.test.mjs` passes (38 tests),
`npm run test:migrations` passes (7 tests), and `git diff --check` passes.
These are static/contract results; they do not replace the missing separate
PDA HTTP runtime qualification.

## Runtime Environment Re-Audit (2026-08-07)

An existing production-like Compose stack was found:

- Compose project: `phase08-clean-1786067194377`
- WMS public gateway health: `200` on port `22400`
- WMS PDA backend health: `200` on port `22312`
- PDA reconciliation read: `consistent=true`, `pending_events=0`,
  `pending_cache_invalidations=0`, `requires_attention=true`
- Existing PDA reconciliation evidence reports 3 historical conflict events;
  these were not changed or deleted.
- The stack injects owner-service URLs and a PDA service token into the
  WMS-side PDA backend container.

This confirms that a production-like WMS runtime exists, but it is not proof
that the separate `pda-backend` repository's HTTP composition is wired to that
runtime. The public gateway did not expose the separate PDA `/api/pda/v1`
routes, and no approved disposable Receipt/quantity mutation was executed
against the running stack. No production data was mutated during this audit.

## Runtime Requalification Update (2026-08-07)

The disposable HTTP composition was subsequently started and exercised through
isolated DB-less Kong. The separate gateway discovered an existing Inbound
Receipt without using local business stores. The owner-side approval and
confirmation path completed for the disposable over-receipt:

```text
receipt=e4f44b6c-4300-47f8-b17e-342d3317ade0 -> Confirmed
approval=08ba9b3e-2501-4e6c-8c7e-82e7dd05d1dc -> APPROVED/effect_applied=true
inventory movement=8886c94a-a58e-43cf-851d-27a82a86b54c, quantity=12
outbox event=c9c838c1-f828-47b3-b26c-13ee1431b5f8 -> PUBLISHED
Inventory reconciliation=complete,mismatch_count=0
```

The event was observed in Kafka as `WMS.Inbound.ReceiptConfirmed.v1` with
the same event and receipt identifiers. Evidence is stored in the WMS
repository under `.artifacts/pda-wms-receiving-http/20260807-over-receipt/`.

The separate PDA integration-event service now consumes
`WMS.Inbound.ReceiptConfirmed.v1` with a durable Inbox guard and projects the
receipt into `receiving_work_projection`. The projection reached
`Confirmed`, version 1, with the source event ID, and remained unchanged after
an integration-service restart. The owner transaction, outbox, Kafka
publication, Inventory reconciliation, PDA Inbox, and PDA projection are
verified for this reference event.

The broader PDA reconciliation API was not required for this adapter-level
qualification; its existing historical `requires_attention` state remains
unchanged and was not cleared.
