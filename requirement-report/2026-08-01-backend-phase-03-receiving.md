Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified in this phase.
- PDA backend repository: current working repository.
- Android project creation: not part of this phase.

# BE-03 Receiving Reference Vertical Slice Report

Status: **APPROVED**

Only the Go backend repository was modified.

## Scope

Implemented SCR-04 through SCR-07 backend support for receiving: warehouse/operator-scoped list and detail, start, task-context barcode resolution, quantity confirmation, completion, durable command-status lookup, explicit aggregate transitions and policy enforcement, optimistic concurrency, retry-safe commands, atomic inventory/outbox/audit persistence, post-commit mock event publication, and projection invalidation hooks. Kafka, real WMS, OIDC claims, and Android changes remain outside this phase.

## Modules and files affected

- `internal/execution/receiving/domain`: receiving task/line model, lifecycle transitions, quantity policies, versions, and typed errors.
- `internal/execution/receiving/application`: query and command orchestration, idempotent replay, transactions, inventory effects, audit, outbox, invalidation, and publication.
- `internal/execution/receiving/ports`: repository, transaction, command, inventory, audit, outbox, invalidation, and publisher boundaries.
- `internal/execution/receiving/adapters/postgres`: PostgreSQL persistence, row/advisory locking, deterministic seeding, command status, inventory, audit, and outbox.
- `internal/integration/adapters/wmsmock`: deterministic receiving fixtures for `REC-001` and `REC-002`.
- Gateway composition and HTTP handlers, OpenAPI, migrations, architecture documentation, and unit/integration/contract tests.

## Database migrations

- `000002_receiving.up.sql`: `receiving_task`, `receiving_line`, `inventory_balance`, `receiving_command_status`, and `audit_record` with indexes and constraints.
- `000002_receiving.down.sql`: reverses all BE-03 objects.
- Pinned `migrate/migrate:v4.18.3` remains the migration runner.
- Real migration down/up cycle: PASS.
- PostgreSQL integration and API tests use isolated `be03_integration` and `be03_api` schemas.

## Endpoints

- `GET /api/pda/v1/receiving/tasks`
- `GET /api/pda/v1/receiving/tasks/{taskId}`
- `POST /api/pda/v1/receiving/tasks/{taskId}/start`
- `POST /api/pda/v1/receiving/tasks/{taskId}/barcode-resolutions`
- `POST /api/pda/v1/receiving/tasks/{taskId}/receipts`
- `POST /api/pda/v1/receiving/tasks/{taskId}/completion`
- `GET /api/pda/v1/receiving/commands/{commandId}`

Requests require bearer authentication, registered device context, and warehouse scope. Mutations require a UUID `Idempotency-Key` and numeric `If-Match`; receipt command IDs and base versions are checked against those headers.

## Receiving behavior and policies

- Allowed lifecycle: `NEW` → `IN_PROGRESS` → `PARTIALLY_COMPLETED` or `COMPLETED`; completion requires all expected quantities received.
- Barcode lookup is constrained to the active receiving document and distinguishes unknown barcodes from barcodes belonging to another document.
- Confirmation enforces positive quantity, configured over-receipt permission, variance-remark requirements, operator and warehouse ownership, and expected aggregate version.
- Every accepted mutation increments the aggregate version; stale or concurrent use of the same version produces a typed conflict.
- Command-status reads are restricted to the originating operator and warehouse.

## Transaction, idempotency, events, and cache hooks

Mutation path: PostgreSQL transaction → command advisory lock/result lookup → receiving row lock → aggregate and policy validation → task/line save → inventory delta → command-result persistence → audit and outbox inserts → commit → projection invalidation → mock publication.

Accepted start, receipt, and completion commands produce production-shaped receiving event envelopes. Idempotent replay returns the durable result without a second inventory effect, outbox record, invalidation, or publication. A deliberately forced late duplicate-command failure proved that line/task/inventory/outbox/audit changes roll back together. Denied commands are recorded by a separate post-rollback audit transaction.

Redis is healthy but not used; invalidation uses the phase-defined no-op boundary until BE-07. Kafka remains disabled.

## Verification

- BE-02 prerequisite approval and baseline `make clean verify`: PASS.
- receiving aggregate transition, quantity, over-receipt, and remark policy tests: PASS.
- list/detail and unknown/wrong-document barcode tests: PASS.
- stale-version and concurrent same-version confirmation tests: PASS; exactly one concurrent command commits.
- duplicate-idempotency replay: PASS; exactly one inventory effect and one logical event.
- forced late persistence failure and full atomic rollback assertion: PASS.
- command status, operator/warehouse scoping, and denied-command audit: PASS.
- gateway receiving API E2E and OpenAPI contract tests: PASS.
- PostgreSQL migrations/repository/transaction integration: PASS.
- real migration down/up: PASS.
- database-backed live login → register → list → start → resolve → receive/replay → complete → command-status flow: PASS.
- live DB assertions: inventory quantity `3`, one replayed command row, four successful mutation outbox records, and four successful audit records: PASS.
- final `make clean verify`, Compose configuration, PostgreSQL readiness, and Redis ping: PASS.

## Failures and corrections

1. The architecture import scanner initially evaluated `_test.go` integration composition as shipped HTTP-adapter coupling. It was corrected to scan production Go files only; production dependency enforcement remains unchanged, while repository-boundary tests continue to cover test files. The full architecture and verification suites pass after the correction.

No test or business assertion was disabled or weakened.

## Dependency verification

- PostgreSQL: physically exercised for migrations, row/advisory locks, concurrency, rollback, idempotency, receiving state, inventory, command status, audit, and outbox.
- Mock WMS fixture adapter: exercised for deterministic receiving seed data.
- Mock event publisher: exercised only after committed mutations and asserted absent on replay/rollback.
- Redis: infrastructure health verified; receiving correctness does not depend on it.
- Kafka, real WMS, and OIDC: not integrated or verified.

## Remaining gaps and readiness

Production Kafka delivery, real WMS synchronization, Redis-backed projections, OIDC validation, and later workflow modules remain future-phase scope. BE-03 exit criteria pass: a receiving command is safe to retry and produces one committed result plus one logical domain event. BE-04 is safe to begin.
