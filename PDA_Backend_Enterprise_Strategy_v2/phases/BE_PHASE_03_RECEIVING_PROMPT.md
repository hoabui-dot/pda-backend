# Backend Phase BE-03 Prompt — Receiving Reference Vertical Slice


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Implement receiving as the reference enterprise transaction, including database consistency, idempotency, aggregate versioning, outbox, audit, and mock Kafka publication.

## APIs

- receiving task list;
- detail;
- start;
- barcode resolution;
- confirm quantity;
- completion.

## Tasks

1. Model `ReceivingTask` and `ReceivingLine` aggregates.
2. Add explicit allowed state transitions.
3. Implement barcode resolution within active task context.
4. Enforce quantity/over/under/remark policies from configuration/fixture.
5. Implement command idempotency table and result replay.
6. Implement optimistic version checks.
7. Commit task/line/inventory effect plus outbox atomically.
8. Add audit record.
9. Add mock event envelopes and mock event-log assertions.
10. Add command-status lookup for mobile offline retry support.
11. Invalidate task/dashboard projections.
12. Keep Kafka configuration disabled.
13. Create API tests matching SCR-04 through SCR-07.

## Tests

- list/detail;
- unknown/wrong-document barcode;
- duplicate scan handled at client contract level where applicable;
- quantity limits;
- required remark;
- duplicate idempotency key;
- stale version;
- concurrent confirmation;
- outbox atomically present;
- mock publisher;
- full receiving E2E.

## Exit criteria

A receiving command is safe to retry and produces one committed result plus one logical domain event.

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