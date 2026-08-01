# Backend Phase BE-04 Prompt — Putaway, Picking, and Replenishment


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Implement the three task-driven movement workflows required by the PDA.

## Putaway

- list/detail;
- source validation;
- destination suggestions;
- destination validation;
- confirm movement.

## Picking

- list/detail/current line;
- location validation;
- barcode resolution;
- confirm pick;
- complete.

## Replenishment

- list/detail;
- source/destination/item validation;
- quantity confirmation;
- complete only when remaining required quantity is zero.

## Shared tasks

1. Create inventory/location ports.
2. Implement explicit movement commands and versions.
3. Enforce task assignment/lock.
4. Commit state plus outbox.
5. Add mock events.
6. Update dashboard/task projections.
7. Add cross-workflow consistency tests.
8. Do not use a universal switch-based movement handler.

## Tests

- wrong source/destination/item;
- capacity/compatibility fixture;
- insufficient stock;
- partial replenishment regression;
- concurrent task conflict;
- idempotency;
- movement event ordering by aggregate key;
- PDA API mapping.

## Exit criteria

All three workflows support explicit validated steps and safe retry behavior.

## Mandatory execution behavior

- Read `00_ENTERPRISE_BACKEND_ARCHITECTURE.md`, `01_BACKEND_PHASE_STRATEGY.md`, `02_API_AND_EVENT_CONTRACT_MAP.md`, `03_BACKEND_CODE_PATTERNS.md`, and `PDA_BACKEND_AI_RULES.md`.
- Inspect the repository before editing.
- Implement only this phase.
- Run baseline tests first.
- Keep mock mode operational.
- Add tests with code.
- Update OpenAPI and documentation.
- Create the required phase report.
- Do not report Kafka/WMS/OIDC verification unless the real dependency was exercised.