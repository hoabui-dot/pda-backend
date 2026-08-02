# Phase 07 Prompt — Cycle Count, Blind Count, Variance, Recount, and Review

## API Scope

- API-025 complete cycle-count workflow
- API-024 command-status integration for count submissions

## Objective

Reconcile the current backend count routes with the PDA requirement for location/item validation, blind counts, variance review, recount, completion, and no implicit inventory adjustment.

## Required Operations

Evaluate canonical support for:

```text
GET counts
GET counts/{taskId}
POST validate-location
POST validate-item
POST lines/{lineId}/submit
POST recount
POST complete
```

## Required Request Fields

```text
commandId, idempotencyKey, taskId, lineId,
location, item, lot, serial, countedQuantity,
blindCount, reasonCode, recount, baseVersion
```

## Required Response Fields

```text
countedQuantity
systemQuantity only when policy permits
variance
varianceState
reviewRequired
approvalRequired
recountRequired
task status
version
auditId
```

## Business Decisions

- blind-count visibility;
- tolerance;
- who approves variance;
- when recount is mandatory;
- whether submission may queue offline;
- whether completion is allowed with unresolved variance;
- retention and status of rejected counts.

## Error Requirements

Implement and test:

```text
COUNT_VARIANCE_REQUIRES_REVIEW
TASK_VERSION_CONFLICT
TASK_LOCKED
ITEM_NOT_IN_DOCUMENT
LOCATION_INVALID
DUPLICATE_COMMAND
```

## Tests

- blind versus non-blind response;
- variance below/above threshold;
- no automatic stock adjustment;
- recount;
- unresolved-review completion rejection;
- duplicate/timeout/status lookup;
- version conflict;
- audit and outbox.

## Exit Criteria

- SCR-15 is fully supported.
- Blind and variance behavior is explicit.
- No count mutation silently adjusts stock.
- Phase report states `APPROVED`.
