# Phase 03 Prompt — Dashboard, Task Center, Task Detail, Claim/Release, and List Foundation

## API Scope

- API-005 Dashboard
- API-007 Task Center page/detail/claim/release

## Objective

Supply every Dashboard and Task Center field required by the PDA and establish the shared list-query behavior reused by later phases.

## Dashboard Required Fields

```text
inboundCount
putawayCount
pickingCount
shippingCount
inProgressCount
completedCount
completionPercent
pendingSyncCount
actionableAlertCount
lastUpdatedAt/asOf
```

Determine whether `pendingSyncCount` is backend-owned or PDA Room-owned. Do not fabricate a server value for client-local pending commands.

## Task Required Fields

```text
id, category/type, status, priority, title,
lineCount, pieceCount, dueAt, assignedOperatorId,
lockState, version, createdAt, updatedAt, warehouseId
```

## Tasks

1. Add/verify task detail endpoint if absent.
2. Reconcile claim/release request bodies with `commandId`, idempotency, and version headers/body.
3. Normalize task filters: cursor, limit, status, category/type, q, priority, zone, date range, sort/direction.
4. Use deterministic cursor ordering.
5. Add `nextCursor`, `hasMore`, `asOf`, and stale metadata.
6. Define task claim/lock duration and ownership rules in the decision register.
7. Ensure workflow mutations invalidate dashboard/task projections.
8. Add exact response-contract tests.

## Exit Criteria

- SCR-02 and SCR-04 can be fully populated.
- Task detail, claim, release, lock, and version behavior are documented.
- Shared list foundation is reusable by domain phases.
- Phase report states `APPROVED`.
