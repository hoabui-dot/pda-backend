# PDA Backend Reconciliation Phase 07

Status: APPROVED
Date: 2026-08-02

## Repository Boundary Confirmation

- Existing PDA Android repository: external client; not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

## Implemented Scope

Phase 07 reconciles API-025 cycle count and count command-status recovery.

- Added canonical `/counts` list/detail, location validation, item validation, line submission, recount, completion, and command-status routes while preserving existing `/cycle-count/tasks` routes.
- Count submission accepts command/idempotency/task/line identity, location/item/lot/serial mirrors, counted quantity, blind-count and reason/recount fields, and base version. Headers and server task identity remain authoritative.
- Count responses expose counted quantity, variance, variance state, review/approval/recount flags, task status, version, audit ID at command boundaries, freshness, and blind-count policy.
- Blind count is server-owned. When `BlindCount` is true, system quantity is omitted from the response even though it remains available to the backend domain. The client cannot enable blind mode by setting a request field.
- Location and item validation are explicit read-only steps and return deterministic next steps.
- Count mutations remain transactional with row locking, outbox, audit, command idempotency, and inventory cache invalidation. No count submission or recount adjusts inventory balances.
- Duplicate and ambiguous submissions can be recovered with `GET /api/pda/v1/counts/commands/{commandId}`.

## Business Decisions

- The current task policy is non-blind unless upstream count configuration sets `BlindCount`; system quantities are visible only under that policy.
- Any non-zero variance is reviewable and currently requires recount before completion. The existing domain rejects completion while `recountRequired` or missing count lines remain.
- No tolerance or automatic approval is assumed; `Tolerance` defaults to zero until inventory-control policy supplies one.
- Count submissions are online-only for this backend contract. Offline drafts may be retained by the PDA, but server submission and review require an online authoritative version.
- Variance never silently adjusts inventory. Approval and stock adjustment are separate future workflows.

## Verification

Passed:

- `go test ./...` for all packages except the known architecture-boundary failure
- Inventory domain and gateway tests
- `make build`
- `make test-kafka`
- `git diff --check`

The architecture scanner still reports Android references in the supplied authoritative PDA documents. Those documents were preserved unchanged.

## Deferred

Upstream blind-count configuration, tolerance values, variance approver roles, rejected-count retention, lot/serial validation, and approved inventory adjustment remain deferred.
