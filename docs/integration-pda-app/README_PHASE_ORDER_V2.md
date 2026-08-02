# PDA Backend Reconciliation v2 — Phase Order

This version supersedes the earlier generic phase set because the actual PDA specification now provides 28 field-level API cards.

Run in order:

1. `PHASE_00_CONTRACT_FREEZE_AND_28_API_TRACEABILITY.md`
2. `PHASE_01_COMMON_TRANSPORT_OPENAPI_AND_ERRORS.md`
3. `PHASE_02_AUTH_DEVICE_BOOTSTRAP_PROFILE.md`
4. `PHASE_03_DASHBOARD_TASKS_AND_LIST_FOUNDATION.md`
5. `PHASE_04_RECEIVING_API_008_010.md`
6. `PHASE_05_MOVEMENT_PUTAWAY_PICKING_REPLENISHMENT.md`
7. `PHASE_06_INVENTORY_INQUIRY_AND_TRANSFER.md`
8. `PHASE_07_CYCLE_COUNT_API_025.md`
9. `PHASE_08_SHIPPING_PACKAGE_AND_CONFIRMATION.md`
10. `PHASE_09_COMMAND_STATUS_OFFLINE_AND_WORKMANAGER_CONTRACT.md`
11. `PHASE_10_CACHE_FRESHNESS_INVALIDATION_AND_OBSERVABILITY.md`
12. `PHASE_11_EXTERNAL_OIDC_WMS_KAFKA_TLS_ZEBRA_E2E.md`

Every phase follows:

```text
PDA_BACKEND_RECONCILIATION_RULES_V2.md
```

## Important New Findings from the Actual PDA Specification

The previous phase set did not explicitly cover all of these requirements:

- login request device/app/locale fields;
- bootstrap scanner policy and feature flags;
- operator profile details;
- task detail and complete filter set;
- scanner `rawValue`, `normalizedValue`, `symbology`, and `scanContext`;
- receiving condition, scanned timestamp, and lot/serial policy;
- picking short-pick and shipment readiness impact;
- replenishment partial completion;
- inventory lot/serial/LPN query dimensions;
- atomic transfer before/after balances;
- blind count, variance review, recount, and approval;
- shipment package verification API;
- generic command-status contract and explicit mobile status values;
- `hasMore`, `asOf`, stale behavior, and stable cursor ordering;
- exact offline policy by mutation;
- unconfirmed business-rule decision register.

## Scope Exclusions

Do not create production contracts for:

- legacy unreachable `InventoryScreen`;
- generic packing;
- LPN/pallet workflows;
- stock adjustment;
- bin-count/bin-query/item-query generic modules;
- label printing.

These remain deferred unless separately approved.
