# PDA Backend Reconciliation Phase 05

Status: APPROVED
Date: 2026-08-02

## Repository Boundary Confirmation

- Existing PDA Android repository: external client; not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

## Implemented Scope

Phase 05 reconciles API-011 through API-021 for putaway, picking, and replenishment.

- Added PDA route aliases for movement list/detail, source/location/item validation, confirmation, and completion while preserving the existing `/tasks` route family.
- Movement validation and quantity requests now accept raw/normalized scanner values, symbology, scan context, task ID, command ID, idempotency key, base version, and scanned timestamp. Header/token context and `If-Match`/`Idempotency-Key` remain authoritative; incompatible body mirrors are rejected.
- Movement responses now expose remaining quantity, progress percentage, deterministic `nextStep`, source/destination balance placeholders, and explicit `shortPickPolicy`.
- Existing domain behavior remains authoritative for source/destination/item sequence, assignment, optimistic versioning, duplicate command replay, stock checks, destination capacity checks, outbox/audit ordering, and cache invalidation.
- Replenishment partial confirmation is supported: positive quantities below the remaining requirement produce `PARTIALLY_COMPLETED`, preserve the remaining quantity, and can be continued with a new version.
- Picking short-pick is explicitly disabled. No automatic short completion or inferred reason code was added without an approved outbound policy.
- LPN/lot/serial workflows remain deferred because the active screen and current movement persistence do not provide approved source fields.

## Business Decisions

- Location capacity and source stock are authoritative repository checks; validation endpoints do not silently reserve inventory.
- Task assignment is acquired by the first successful validation and remains locked to that operator until completion or explicit operational override.
- Putaway requires source and destination validation; picking requires location and item validation; replenishment requires source, destination, and item validation.
- Partial replenishment is allowed and returns a remaining quantity. Putaway and picking cannot exceed their remaining requirement.
- Short-pick remains disabled and returns the existing incomplete/quantity policy errors rather than fabricating a shortage outcome.

## Verification

Passed:

- `go test ./...` for all packages except the known architecture-boundary failure
- Movement domain and gateway tests
- `make build`
- `make test-kafka`
- `git diff --check`

The architecture scanner still reports Android references in the supplied authoritative PDA documents. Those documents were preserved unchanged.

## Deferred

Approved WMS-backed LPN/lot/serial data, source/destination balance projections, location reservation semantics, picking short-pick reason codes, and shipment-readiness propagation remain deferred to later integration/business approval phases.
