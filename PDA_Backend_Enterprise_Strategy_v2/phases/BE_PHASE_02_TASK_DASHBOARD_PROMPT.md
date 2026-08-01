# Backend Phase BE-02 Prompt — Task Core, Dashboard, and Task Center


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Implement the backend capabilities required by SCR-02 Dashboard and SCR-03 Task Center.

## Tasks

1. Create task aggregate/model:
   - status;
   - category;
   - assignment;
   - priority;
   - warehouse;
   - operator;
   - version.
2. Add versioned `golang-migrate` SQL migrations.
3. Implement deterministic task fixtures through mock WMS port.
4. Implement:
   - `GET /dashboard`
   - `GET /tasks/summary`
   - `GET /tasks`
   - claim/release commands.
5. Add cursor pagination and filtering.
6. Enforce assignment/warehouse authorization.
7. Add task lifecycle events to outbox.
8. Publish through mock event publisher.
9. Add task/dashboard projection invalidation hooks; Redis use remains optional until BE-07.
10. Map exact fields required by the PDA UI.

## Tests

- dashboard counts;
- category/status counts;
- pagination/filtering;
- claim/release;
- task locked/not assigned;
- optimistic version conflict;
- idempotent claim;
- outbox/mock event;
- authorization.

## Exit criteria

Dashboard and Task Center APIs are stable, versioned, test-covered, and mapped to PDA states.

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
