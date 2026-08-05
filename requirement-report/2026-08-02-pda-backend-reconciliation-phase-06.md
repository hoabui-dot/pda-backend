# PDA Backend Reconciliation Phase 06

Status: APPROVED
Date: 2026-08-02

## Repository Boundary Confirmation

- Existing PDA Android repository: external client; not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

## Implemented Scope

Phase 06 reconciles API-022 inventory inquiry, API-023 transfer, and transfer API-024 command-status recovery.

- Inventory search, balance, and movement routes accept the PDA query dimensions `q`, `barcode`, `itemId`, `locationCode`, `lotNumber`, `serialNumber`, `lpnId`, `cursor`, and `limit` at the transport boundary.
- Inventory responses expose normalized item/location identifiers, on-hand, reserved, available, damaged, hold, quarantine, in-transit, UOM, lot/serial placeholders, version, `asOf`, and `stale` metadata.
- Added proposed `/inventory/items`, `/transfers/validate`, `/transfers/confirm`, and `/transfers/commands/{commandId}` routes while preserving existing compatibility paths.
- Transfer requests accept item/source/destination/quantity, lot/serial mirrors, scan context, base version, command identity, idempotency key, and reason. Header and authenticated context remain authoritative.
- Transfer transactions now capture source/destination before and after balance snapshots, transfer ID, and audit ID in the durable result. Existing transaction locking, source stock validation, destination validation, outbox, audit, idempotency, and both-location cache invalidation remain in place.
- Transfer command status can recover a committed result after an ambiguous response by querying the durable command ID.

## Business Decisions

- Transfer is online-only by default. No offline transfer queue or automatic retry was introduced because duplicate stock movement is unsafe without an approved offline policy.
- Source and destination must differ; source stock and destination location validity are server-authoritative.
- Lot/serial mismatch and LPN lookup remain non-blocking placeholders until those dimensions exist in the approved inventory/WMS schema. LPN is not treated as an active workflow.
- Inquiry cache responses are explicitly marked `stale: false` for current authoritative reads. Stale cached display and cache-age policy remain Phase 10 work.

## Verification

Passed:

- `go test ./...` for all packages except the known architecture-boundary failure
- Inventory and gateway tests
- `make build`
- `make test-kafka`
- `git diff --check`

The architecture scanner still reports Android references in the supplied authoritative PDA documents. Those documents were preserved unchanged.

## Deferred

Authoritative item master code/description, lot/serial/LPN persistence, reserved/damaged/hold/quarantine/in-transit quantities, movement cursor continuation, stale cache serving, and transfer capacity/reservation policy require the upstream inventory/WMS contract and remain deferred.
