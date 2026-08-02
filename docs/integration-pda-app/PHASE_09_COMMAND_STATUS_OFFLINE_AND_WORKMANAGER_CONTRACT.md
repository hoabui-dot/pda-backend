# Phase 09 Prompt — Generic Command Status, Offline Policy, and WorkManager Contract

## API Scope

- API-024
- Every mutation from API-007, API-010, API-014, API-018, API-021, API-023, API-025, API-027, and API-028

## Objective

Standardize durable command reconciliation across all workflows and provide the exact contract required by Room/WorkManager after timeout, duplicate, process death, or reconnect.

## Canonical Status Values to Reconcile

PDA proposes:

```text
PENDING
ACKNOWLEDGED
CONFLICT
PERMANENT_FAILURE
```

Backend may have additional states. Freeze one mapped enum and define terminal/non-terminal behavior.

## Required Response Fields

```text
commandId
idempotencyKey
type/operation
status
aggregateId
version
result or result reference
error code
correlationId
processedAt
```

## Tasks

1. Add or normalize `GET /api/pda/v1/commands/{commandId}`.
2. Authorize lookup by warehouse/device/operator policy.
3. Map existing receiving/shipping domain status routes into the canonical model or deprecate duplicates safely.
4. Expose status for movement, transfer, and cycle count.
5. Define retention.
6. Define duplicate completed, in-progress, conflicting-payload, and permanent-failure behavior.
7. Define timeout and 409 duplicate client flow.
8. Classify each mutation:
   - draft allowed;
   - queue allowed;
   - online-only;
   - automatic retry allowed.
9. Keep transfer and shipment online-only unless explicitly approved.
10. Document WorkManager backoff and network constraints without modifying Android code.

## Tests

- service restart persistence;
- process-death simulation at API level;
- timeout then status;
- unauthorized lookup;
- duplicate with same/different payload;
- retention expiry;
- terminal status mapping;
- concurrent worker polling.

## Exit Criteria

- Every retry-capable mutation has deterministic reconciliation.
- PDA WorkManager can implement one documented status flow.
- Phase report states `APPROVED`.
