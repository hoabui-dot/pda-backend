# Phase 04 Prompt — Receiving Contract Reconciliation

## API Scope

- API-008 Receiving list/detail
- API-009 Receiving barcode resolution
- API-010 Receiving confirmation
- API-024 Command status integration for receiving

## Objective

Make receiving the reference end-to-end PDA workflow with exact scanner, quantity, policy, version, idempotency, command-status, and response behavior.

## Required Request Reconciliation

Barcode resolution must evaluate:

```text
taskId, lineId?, rawValue, normalizedValue, symbology,
scanContext=RECEIVING_ITEM, scannedAt
```

Identity/warehouse/device are authoritative from headers/token; decide whether duplicated body fields are rejected, ignored, or supported as compatibility mirrors.

Confirmation must evaluate:

```text
commandId, idempotencyKey, taskId, lineId, barcode,
quantity, condition, remark?, baseVersion, scannedAt
```

## Required Response Data

- order/task and line IDs;
- purchase order/supplier;
- item/SKU/name/barcode;
- expected/received/remaining quantities;
- quantity policy;
- lot/serial/condition requirements;
- task and line status;
- version;
- command status;
- audit ID;
- next step;
- freshness.

## Business Decisions

- over/under tolerance;
- under-receipt remark;
- condition enum;
- lot/serial capture;
- whether start and completion remain separate backend endpoints;
- 200 versus 202 for durable asynchronous completion.

## Tests

- leading-zero barcode;
- symbology/context;
- item not in document;
- quantity and condition policy;
- duplicate command;
- timeout then API-024;
- stale version;
- concurrent confirmation;
- outbox atomicity;
- cache invalidation.

## Exit Criteria

- SCR-05 through SCR-07 are fully supported.
- Receiving request and response examples are frozen in OpenAPI.
- Unknown-success recovery works through command status.
- Phase report states `APPROVED`.
