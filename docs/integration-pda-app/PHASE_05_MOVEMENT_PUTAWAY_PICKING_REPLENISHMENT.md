# Phase 05 Prompt — Putaway, Picking, and Replenishment Reconciliation

## API Scope

- API-011 through API-021

## Objective

Align all movement list/detail, scanner validation, partial completion, short-pick, balance, version, and next-step contracts.

## Putaway

Cover:

- list/detail;
- source/item validation;
- destination validation and suggestions;
- confirmation;
- capacity and stock errors;
- source/destination balance response;
- optional LPN fields only if approved for the active screen.

## Picking

Cover:

- list/detail/current line;
- location validation;
- item/lot/serial resolution;
- normal pick;
- short-pick decision and reason codes;
- completion;
- next line;
- progress;
- shipment readiness impact.

Do not implement short-pick merely because the PDA document proposes it. Freeze the business decision first.

## Replenishment

Cover:

- list/detail;
- source/item/destination validation;
- partial confirmation;
- remaining quantity;
- completion semantics;
- source/destination balances.

## Scanner Payload

For every validation reconcile:

```text
rawValue, normalizedValue, symbology, scanContext,
location/item/lot/serial context, baseVersion
```

## Business Decisions

- location capacity/reservation;
- task assignment and lock;
- LPN usage;
- short-pick policy and reason codes;
- replenishment partial completion;
- lot/serial enforcement.

## Tests

- wrong context/source/destination/item;
- insufficient stock;
- capacity exceeded;
- partial replenishment;
- short-pick disabled/enabled behavior;
- ordering and version conflicts;
- duplicate commands;
- command-status recovery;
- balance and task invalidation.

## Exit Criteria

- SCR-08 through SCR-12 are fully supported.
- Each scanner step returns a deterministic next step.
- Partial/short completion rules are explicit.
- Phase report states `APPROVED`.
