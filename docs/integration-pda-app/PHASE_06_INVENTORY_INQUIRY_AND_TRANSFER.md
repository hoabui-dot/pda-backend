# Phase 06 Prompt — Inventory Inquiry and Stock Transfer

## API Scope

- API-022 Inventory items/balances/movements
- API-023 Transfer validation/confirmation
- API-024 Command status integration for transfer

## Objective

Provide the complete search dimensions and freshness required by SCR-13, and an atomic, online-only-by-default transfer contract for SCR-14.

## Inventory Query Requirements

Evaluate:

```text
q, barcode, itemId, locationCode,
lotNumber, serialNumber, lpnId, cursor, limit
```

Required response fields:

```text
itemId, itemCode, description,
locationId, locationCode,
onHand, reserved, available, damaged,
hold/quarantine, inTransit, UOM,
lotNumber, serialNumber, asOf, version, stale
```

Do not implement LPN lookup as an active workflow unless approved; an optional filter may be supported only if backend domain data exists.

## Transfer Requirements

Validation body:

```text
itemId, sourceLocationCode, destinationLocationCode,
lotNumber?, serialNumber?, quantity, scanContext, baseVersion
```

Confirmation adds:

```text
commandId, idempotencyKey, reason?
```

Success must return transfer ID, before/after balances, versions, and audit ID.

## Tasks

1. Normalize inventory search, balance, and movement pagination.
2. Add `asOf` and stale behavior.
3. Ensure source and destination validation are authoritative.
4. Decide whether existing split validation endpoints or one `/transfers/validate` contract is canonical.
5. Keep final transfer online-only until offline policy is explicitly approved.
6. Expose command status for timeout ambiguity.
7. Invalidate both source and destination balance projections.

## Tests

- all search dimensions;
- leading-zero barcode;
- stale cache;
- source equals destination;
- insufficient stock;
- capacity error;
- lot/serial mismatch;
- atomic concurrent transfer;
- duplicate/timeout/status lookup;
- both-location invalidation.

## Exit Criteria

- SCR-13 and SCR-14 can integrate without missing fields.
- Transfer is atomic and safely recoverable after timeout.
- Phase report states `APPROVED`.
