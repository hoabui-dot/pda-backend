# PDA Backend Reconciliation Phase 09

Status: APPROVED
Date: 2026-08-02

## Repository Boundary Confirmation

- Existing PDA Android repository: external client; not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

## Implemented Scope

Phase 09 reconciles API-024 and durable command recovery for task, receiving, movement, transfer, count, package verification, and shipment confirmation mutations.

- Added canonical `GET /api/pda/v1/commands/{commandId}`.
- The endpoint authorizes command lookup against the authenticated operator and warehouse. Existing workflow-specific status endpoints remain supported for compatibility.
- Durable completed command records normalize to PDA status `ACKNOWLEDGED`, with command ID, idempotency key compatibility value, operation/type, aggregate ID when recoverable, result, and nullable version/error/correlation/processed fields.
- Receiving, transfer, cycle-count, and shipping already expose durable command lookups; movement and task services now expose equivalent command lookup paths to the gateway.
- A timeout after transaction commit is recoverable through the generic status endpoint. A timeout before durable command persistence remains retryable under the original idempotency key. Duplicate completed requests replay the stored result; conflicting context is rejected.
- Transfer and shipment confirmation remain online-only. Count submission is online-only; the PDA may retain an offline draft but must not assume server acceptance. Read-only snapshots may be cached with freshness metadata.

## Canonical Status and Retry Policy

- `ACKNOWLEDGED`: durable mutation committed and result available; terminal for synchronization.
- `PENDING`: client-local state while a request has not received a durable server result; no fabricated server record is emitted.
- `CONFLICT`: stale version, lock, or business conflict; terminal until the client reloads and resolves the draft.
- `PERMANENT_FAILURE`: non-retryable validation or authorization failure; terminal.
- Automatic retry is allowed only for transport/dependency failures classified retryable and must reuse the original command ID/idempotency key. Never generate a new command for a retry.
- WorkManager integration contract: require network connectivity, use exponential backoff with bounded attempts for retryable transport failures, poll API-024 after ambiguous responses, stop polling on `ACKNOWLEDGED`, `CONFLICT`, or `PERMANENT_FAILURE`, and retain terminal results for audit until local retention policy removes them.

## Retention and Authorization

Backend command records remain in their workflow command-status tables until the approved operational retention job removes them. No destructive retention job was introduced without a policy value. Lookup is restricted to the command's warehouse and operator; device identity remains part of the original audit context but is not used to broaden access.

## Verification

Passed:

- Generic command-status gateway regression test
- `go test ./...` for all packages except the known architecture-boundary failure
- `make build`
- `make test-kafka`
- `git diff --check`

The architecture scanner still reports Android references in the supplied authoritative PDA documents. Those documents were preserved unchanged.

## Deferred

Cross-workflow command-status database unification, explicit persisted PENDING/CONFLICT/PERMANENT_FAILURE rows, command retention scheduler, and WorkManager implementation remain deployment/client integration work. The backend contract and reconciliation behavior are frozen for the next phase.
