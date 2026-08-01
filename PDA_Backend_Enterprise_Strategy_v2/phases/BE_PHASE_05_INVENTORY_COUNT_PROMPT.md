# Backend Phase BE-05 Prompt — Inventory Inquiry, Stock Transfer, and Cycle Count


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Implement the inventory read and control capabilities required by SCR-13, SCR-14, and SCR-15.

## Tasks

1. Implement item/location search.
2. Implement balance and movement-history queries with `asOf`.
3. Implement stock transfer validation and confirmation.
4. Implement cycle-count tasks, lines, count submission, variance, and recount state.
5. Never automatically adjust stock from variance.
6. Add movement ledger.
7. Add idempotency/version/authorization.
8. Add outbox and audit events.
9. Add cache interfaces but keep correctness database-based.
10. Map Bin Query/Item Query behavior into Inventory Inquiry; do not create extra public PDA endpoints without need.

## Tests

- freshness metadata;
- source equals destination;
- insufficient stock;
- destination invalid;
- duplicate transfer;
- concurrent transfer;
- count variance;
- no auto-adjust;
- recount;
- movement history;
- cross-feature balance reconciliation.

## Exit criteria

Inventory inquiry is authoritative and every transfer/count command is explicit, versioned, and audited.

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