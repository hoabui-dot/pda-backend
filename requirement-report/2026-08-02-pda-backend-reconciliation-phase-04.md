# PDA Backend Reconciliation Phase 04

Status: APPROVED
Date: 2026-08-02

## Repository Boundary Confirmation

- Existing PDA Android repository: external client; not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

## Implemented Scope

Phase 04 reconciles API-008 receiving list/detail, API-009 barcode resolution, API-010 receiving confirmation, and receiving command status.

- Added proposed route aliases `GET /api/pda/v1/receiving`, `GET /api/pda/v1/receiving/{taskId}`, `POST /api/pda/v1/receiving/{taskId}/resolve-barcode`, and `POST /api/pda/v1/receiving/{taskId}/confirm`. Existing `/receiving/tasks` routes remain compatible.
- Receiving list/detail responses now include order and purchase-order identifiers, supplier, status, assignment, version, expected/received/remaining quantities, quantity policy, condition policy, and enriched line data.
- Barcode resolution accepts `rawValue`, `normalizedValue`, `symbology`, `scanContext`, `scannedAt`, and optional body context mirrors. Leading zeros are preserved. Device, warehouse, and operator identity are taken from authoritative headers/token; mismatched mirrors are rejected.
- `scanContext` must be `RECEIVING_ITEM`. Barcode resolution is read-only and does not mutate inventory.
- Confirmation accepts command identity, task/line/barcode, quantity, condition, remark, base version, and scanned timestamp. Header `Idempotency-Key` and `If-Match` remain authoritative; body mirrors are checked when supplied.
- Conditions support policy-controlled values. Empty condition remains a local compatibility default of `GOOD`; new policy data can restrict values. Quantity overage and variance remark rules remain server-enforced.
- Confirmation response includes authoritative receiving state, command status, audit identifier, next step, and freshness data. Existing transaction ordering remains repository save, inventory application, outbox, audit, and command-status persistence in one transaction.
- Unknown-success recovery remains available through `GET /api/pda/v1/receiving/commands/{commandId}`. Duplicate idempotency keys replay the stored result without repeating inventory or outbox effects.

## Business Decisions

- Over-receipt is rejected unless the task policy explicitly allows it.
- Under-receipt or quantity variance requires a remark when the task policy requires it.
- Default condition is `GOOD` for legacy local clients; production policy should require an explicit condition before enabling damaged/quarantine workflows.
- Lot/serial requirements are exposed in the policy and line response, but capture and persistence are deferred until WMS receiving master data provides those fields.
- Start and completion remain separate commands. Confirmation returns 200 after the durable transaction commits; 503 with retryable metadata is used only when event publication remains pending.
- Operator, warehouse, and device body fields are compatibility mirrors and never override authenticated/header context.

## Verification

Passed:

- `go test ./...` for all packages except the known architecture-boundary failure
- `make build`
- `make test-kafka`
- `git diff --check`

The architecture scanner still reports Android references in the supplied authoritative PDA documents. Those documents were preserved unchanged.

## Deferred

Supplier and lot/serial values are not present in the current WMS fixture/schema and therefore remain empty/default. A durable audit ID currently uses the command identity at the HTTP boundary; a separate audit-read model is deferred. Full production upstream receiving integration remains subject to the approved WMS contract.
