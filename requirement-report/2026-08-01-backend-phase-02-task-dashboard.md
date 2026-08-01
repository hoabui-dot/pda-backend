Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified in this phase.
- PDA backend repository: current working repository.
- Android project creation: not part of this phase.

# BE-02 Task Core, Dashboard, and Task Center Report

Status: **APPROVED**

Only the Go backend repository was modified.

## Scope

Implemented SCR-02 Dashboard and SCR-03 Task Center backend support: versioned task aggregate, PostgreSQL persistence and reversible migration, deterministic mock-WMS seeding, warehouse/operator-scoped reads, cursor pagination/filtering, claim/release commands, transactionally persisted idempotency and outbox events, post-commit mock publication, and projection invalidation ports. Receiving behavior was not implemented.

## Modules and files affected

- `internal/execution/domain`: task status/category/assignment/priority/warehouse/version model and lifecycle errors.
- `internal/execution/application`: dashboard/list/summary queries and transactional claim/release orchestration.
- `internal/execution/ports`: task, transaction, idempotency, outbox, invalidation, and upstream seed ports.
- `internal/execution/adapters/postgres`: PostgreSQL repository, row locking, transaction context, advisory idempotency locking, outbox/idempotency persistence.
- `internal/execution/adapters/memory`: test/local adapter with matching semantics.
- `internal/integration/adapters/wmsmock`: embedded deterministic task fixtures.
- Gateway composition and HTTP adapter, OpenAPI, migrations, CI/Make, tests, and architecture documentation.

## Database migrations

- `000001_task_core.up.sql`: `warehouse_task`, `command_idempotency`, `domain_outbox`, and query indexes.
- `000001_task_core.down.sql`: reverses all BE-02 objects.
- Pinned `migrate/migrate:v4.18.3` is used by `make migrate-up`/`make migrate-down`.
- Real CLI down/up cycle: PASS.
- Integration migration test runs in isolated `be02_integration` schema and cannot disturb development migration history.

## Endpoints

- `GET /api/pda/v1/dashboard`
- `GET /api/pda/v1/tasks/summary?status=`
- `GET /api/pda/v1/tasks?category=&status=&cursor=&q=&limit=`
- `POST /api/pda/v1/tasks/{taskId}/claim`
- `POST /api/pda/v1/tasks/{taskId}/release`

Commands require bearer authentication, registered device, warehouse header, UUID `Idempotency-Key`, and numeric `If-Match` version.

## Authorization and task behavior

- Reads expose only unassigned work or tasks assigned to the authenticated operator in the selected warehouse.
- Direct claim of another operator's assignment returns `TASK_LOCKED`.
- Release requires assignment to the authenticated operator.
- Stale writes return `TASK_VERSION_CONFLICT`.
- Cursor ordering is stable by task ID; limit is bounded to 1–100.
- Dashboard returns total available/owned tasks, assigned, in-progress, completed, high-priority, and projection timestamp fields.

## Transaction, idempotency, events, and cache hooks

Mutation path: PostgreSQL transaction → advisory idempotency-key lock → prior-result lookup → task row lock → aggregate/version validation → task save → outbox insert → idempotency insert → commit → invalidation port → mock publication.

Events: `TaskAssigned` and `TaskReleased`, topic `pda.task.events.v1`, with complete actor/correlation/causation metadata. Idempotent replay returns the stored result and does not invalidate or publish again.

Redis is not used. The projection invalidation port is present with a no-op adapter until BE-07.

## Verification

- BE-01 prerequisite and baseline `make clean verify`: PASS.
- aggregate/query tests for dashboard, summary, filtering, pagination: PASS.
- claim/release, locked/not-assigned, version conflict, authorization: PASS.
- idempotent replay and single outbox/mock event: PASS.
- gateway API tests: PASS.
- PostgreSQL migration/repository/transaction/rollback integration: PASS.
- real `golang-migrate` down/up: PASS.
- OpenAPI and architecture tests: PASS.
- final `make clean verify`: PASS.
- database-backed live login → register → dashboard → claim → replay: PASS.
- live DB assertions: one command row and one outbox event after replay: PASS.
- PostgreSQL/Redis infrastructure health: PASS.

## Failures and corrections

1. Latest `pgx/v5.10.0` raised the minimum module version to Go 1.25. Pinned compatible `pgx/v5.7.5` and restored the documented Go 1.24 baseline; focused/full tests passed.
2. A final formatting gate detected a modified integration test that had not been normalized. Ran `gofmt` and reran the entire suite successfully.
3. The database integration test originally used the public schema, which could interfere with development migration history. Isolated it in a dedicated schema and reran integration/full verification.

No test or assertion was disabled or weakened.

## Dependency verification

- PostgreSQL: physically exercised for migration, locks, transaction rollback, idempotency, task mutation, and outbox.
- Mock event publisher: exercised after committed mutation.
- Redis: healthy but intentionally unused.
- Kafka, real WMS, and OIDC: not integrated or verified.

## Remaining gaps and readiness

Receiving-specific aggregates, lines, barcode resolution, receipt confirmation/completion, inventory effects, and receiving outbox events remain BE-03 scope. BE-02 exit criteria pass; BE-03 is safe to begin.
