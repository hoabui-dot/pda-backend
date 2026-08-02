# Backend Phase BE-09 - Upstream WMS Integration

Date: 2026-08-02  
Status: **PARTIALLY_IMPLEMENTED - WORKFLOW OWNERSHIP BLOCKED**

## Scope

BE-09 was started after the local PLAINTEXT BE-08 exit criteria were met. An approved WMS
OpenAPI/event contract is available in the shared `ricoh-wms` repository under
`packages/contracts`. It defines WMS master-data, inventory, inbound, outbound HTTP APIs and
WMS-owned event channels. It does not define ownership or synchronization APIs for the PDA task,
receiving, putaway, picking, replenishment, cycle-count, or shipping aggregates. The PDA backend
must not invent those upstream mappings.

The existing Kotlin Android PDA remains an external client and was not modified or executed.

## Baseline

- `make fmt`: PASS.
- `make lint`: PASS.
- `make test`: application, integration, contract, and Kafka packages pass.
- The architecture test is blocked by the user-supplied untracked
  `GENERATE_PDA_APP_API_INTEGRATION_REQUIREMENTS_PROMPT.md`, which the backend repository-boundary
  guard rejects because it contains an Android reference.
- Existing WMS resilience and mock adapter tests pass.

## Existing boundary

- `internal/integration/ports.UpstreamWMS` currently exposes only `Warehouses`.
- `internal/integration/adapters/wmsmock` provides deterministic fixture-backed WMS data.
- `internal/integration/adapters/resilient` provides timeout/retry/bulkhead/circuit policy wrapping.
- `internal/integration/adapters/resilient/cached_wms.go` provides cache-aside warehouse reads.
- `PDA_UPSTREAM_WMS_MODE` supports explicit `mock` and reserved `http` values.

## Contract-driven implementation

Implemented `internal/integration/adapters/wmshttp.Client` for the approved WMS master-data
contract:

- `GET /api/wms/master-data/warehouses?limit=500`;
- `Authorization: Bearer <token>`;
- `{ "data": [...] }` response envelope;
- `warehouse_id`, `warehouse_code`, and localized `warehouse_name` mapping to the backend-owned
  `ports.Warehouse` type;
- bounded response decoding and non-success status errors;
- unit tests for authentication, path/query behavior, localized mapping, malformed data, and
  upstream failure.

Configuration validates `PDA_UPSTREAM_WMS_BASE_URL` and `PDA_UPSTREAM_WMS_TOKEN` when HTTP WMS
mode is selected. The gateway remains fail-fast for HTTP mode because only warehouse discovery is
implemented and the gateway still has fixture-backed PDA workflow seeding.

## Correction implemented

Before this phase, configuration accepted `PDA_UPSTREAM_WMS_MODE=http` while
`cmd/pda-api-gateway` still unconditionally loaded mock task/receiving/movement fixtures. That was
an unsafe silent fallback. The gateway now fails fast when the mode is not `mock`:

```text
upstream WMS adapter is not enabled; provide the approved WMS contract before selecting HTTP mode
```

Mock mode remains operational for local development and tests.

## Deferred implementation

The following are intentionally not implemented until upstream ownership and synchronization
contracts are supplied:

- PDA task/receiving/movement WMS client DTOs and endpoint paths;
- WMS event consumers for PDA-owned projections;
- task, receiving, movement, inventory, master-data, and event mappings beyond warehouse discovery;
- WMS status/error translation;
- synchronization checkpoints and replay storage;
- WMS event consumers and duplicate/out-of-order handling;
- reconciliation jobs and mismatch reports;
- staging E2E against the real WMS.

## Required input to resume

Provide the missing PDA/WMS integration ownership package containing:

1. OpenAPI or event schemas and versioning policy;
2. ownership for task, inventory, receiving, movement, shipment, and master-data records;
3. authentication, timeout, rate-limit, and retry policy;
4. idempotency and ordering guarantees;
5. status and error mapping;
6. checkpoint/replay and reconciliation requirements;
7. staging endpoint/credentials and test fixtures.

## Exit decision

BE-09 is **not approved**. The warehouse discovery adapter is implemented and tested, but the
backend remains safe only in explicit mock WMS mode until PDA workflow ownership, event mappings,
and staging verification are supplied. No final WMS production-readiness claim is made.
