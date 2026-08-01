Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified in this phase.
- PDA backend repository: current working repository.
- Android project creation: not part of this phase.

# BE-05 Inventory Inquiry, Transfer, and Cycle Count Report

Status: **APPROVED**

## Scope

Implemented authoritative item/bin inquiry, location balances with freshness metadata, movement history, explicit stock-transfer validation/confirmation, and versioned cycle-count tasks with lines, variance, recount, and completion. Count variance never adjusts inventory.

## Implementation

- Go domain/application/ports/PostgreSQL adapters under `internal/inventory`.
- Migration `000004_inventory_control` adds stock transfer, cycle-count task/line, command-status, and ledger query indexing; down migration is reversible.
- Inventory API: search, balances, movement history, transfer source/destination/item validation, and confirmation.
- Cycle-count API: list, detail, count submission, recount, and completion.
- Bin Query and Item Query are filters over Inventory Inquiry; no unnecessary public endpoints were introduced.
- Cache invalidation is a no-op port until BE-07; PostgreSQL remains authoritative.

## Transaction guarantees

Transfer validation rejects equal locations, missing destinations, and insufficient stock. Confirmation locks authoritative balances and idempotency key, reconciles source/destination, appends `inventory_movement`, audit, durable result, and outbox atomically. Replay does not repeat movement.

Cycle-count commands enforce warehouse/operator scope and aggregate version. Submission stores expected/count/variance evidence and emits audit/outbox records without touching inventory. Recount clears the prior observed quantity, and completion requires every line to be counted without a pending recount.

## Verification

- BE-04 full clean revalidation and infrastructure health: PASS.
- freshness/as-of and movement history queries: PASS.
- equal source/destination, invalid destination, and insufficient stock: PASS.
- duplicate transfer replay and single ledger effect: PASS.
- concurrent transfer: exactly one succeeds when stock supports only one.
- cross-feature source/destination balance reconciliation: PASS.
- count variance and no automatic inventory adjustment: PASS.
- recount, corrected count, completion, stale version, and assignment checks: PASS.
- OpenAPI, architecture, build, unit, and PostgreSQL integration gates: PASS.
- migration 4 down/up: PASS.
- live login/device/inquiry/transfer/replay/history/count-variance flow: PASS.
- live API camel-case contract correction and recheck: PASS.
- final `make clean verify`: PASS.
- Compose, PostgreSQL, and Redis health: PASS.

## Failures and corrections

1. Initial compilation found two event-builder calls passing a timestamp instead of the clock function; both were corrected and all tests passed.
2. Live API smoke exposed default capitalized JSON field names on new inventory/count response fields. Explicit camel-case tags were added and the live inquiry/count/history contract passed.

No test or assertion was disabled or weakened.

## Dependency verification and readiness

PostgreSQL, mock publication, and deterministic fixtures were exercised. Redis is healthy but unused. Kafka, real WMS, and OIDC were not integrated or verified.

BE-05 exit criteria pass: inquiry is authoritative and every transfer/count command is explicit, versioned where aggregate state applies, idempotent, and audited. BE-06 is safe to begin.
