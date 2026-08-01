Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified in this phase.
- PDA backend repository: current working repository.
- Android project creation: not part of this phase.

# BE-04 Putaway, Picking, and Replenishment Report

Status: **APPROVED**

Only the Go backend repository was modified.

## Scope

Implemented the three PDA movement workflows as separate Go application services. Putaway supports list/detail, source validation, capacity/compatibility-filtered destination suggestions, destination validation, and confirmation. Picking supports list/detail/current task line, location validation, barcode resolution, pick confirmation, and completion. Replenishment supports list/detail, source/destination/item validation, partial quantity confirmation, and completion only when remaining required quantity is zero.

## Architecture and persistence

- `internal/execution/movement/domain`: explicit movement aggregate, assignment/version guards, validation sequence, quantities, and workflow errors.
- `internal/execution/movement/application`: distinct `PutawayService`, `PickingService`, and `ReplenishmentService`; no universal switch-based movement handler.
- `internal/execution/movement/ports`: shared inventory, location, repository, transaction, command, outbox, audit, invalidation, and publisher ports.
- `internal/execution/movement/adapters/postgres`: row/advisory locking, inventory checks/effects, idempotency, audit/outbox, and deterministic seed.
- `internal/integration/adapters/wmsmock/movement.go`: deterministic `PUT-001`, `PICK-001`, and `REP-001` fixtures.
- Gateway routes, OpenAPI contract, architecture documentation, and unit/integration/API tests were updated.

Migration `000003_movement_workflows` adds `movement_task`, `warehouse_location`, `inventory_location_balance`, `inventory_movement`, and `movement_command_status`. Its down migration reverses all BE-04 objects. A real migration up → down → up cycle passed, leaving schema version `3` clean.

## API surface

- Putaway: `/putaway/tasks`, detail, `source-validations`, `destination-suggestions`, `destination-validations`, and `confirmation`.
- Picking: `/picking/tasks`, detail, `location-validations`, `barcode-resolutions`, `picks`, and `completion`.
- Replenishment: `/replenishment/tasks`, detail, `source-validations`, `destination-validations`, `item-validations`, and `confirmation`.

All routes remain under `/api/pda/v1` and require trusted bearer/device/warehouse context. Mutations require UUID `Idempotency-Key` and numeric `If-Match`.

## Consistency and retry behavior

Each command serializes its idempotency key, locks its movement aggregate, validates assignment/warehouse/version and workflow sequence, and checks source stock plus destination capacity/compatibility. The same transaction updates movement state and Task Center projection, applies source/destination inventory effects, records `inventory_movement`, audit, durable command result, and outbox. Invalidation and mock publication happen after commit.

Replay returns the stored task without a second movement/event. Reuse across tasks, workflows, operators, or warehouses is rejected. Outbox events use the task aggregate ID and monotonically increasing aggregate version, preserving per-aggregate ordering.

## Verification

- BE-03 approval and clean baseline: PASS.
- wrong source/destination/item, validation sequence, assignment lock, stale version, and quantity limits: PASS.
- fixture-backed compatible destination suggestions, incompatible destination, capacity, and insufficient-stock checks: PASS.
- partial replenishment regression and zero-remaining completion: PASS.
- concurrent same-version picking validation: exactly one success and one version conflict.
- idempotent putaway replay: one command result and one inventory movement.
- cross-workflow PostgreSQL flow for putaway, picking, and replenishment: PASS.
- per-aggregate inventory movement ordering: PASS.
- PDA HTTP mapping and OpenAPI/architecture contracts: PASS.
- migration up/down/up and isolated integration schemas: PASS.
- final `make clean verify`: PASS.
- live gateway login → register → list → suggestions → source → destination → confirm → replay: PASS.
- live assertions: completed version `4`, one inventory movement, one replayed command row, three ordered outbox records, Task Center state `COMPLETED:4`.
- Compose config, PostgreSQL readiness, and Redis ping: PASS.

## Failures and corrections

1. Extending the gateway constructor exposed one older resilience test call still using the previous argument list. The test composition was updated to pass an explicit nil movement dependency and the full suite passed.
2. Shared OpenAPI movement path fragments initially carried repeated operation IDs. Those IDs were removed from reusable fragments and required task-path/header parameters were made explicit.
3. Idempotent replay was tightened after review to validate workflow, task, operator, and warehouse ownership before returning a stored result.

No test or business assertion was disabled or weakened.

## Dependency verification

- PostgreSQL: physically exercised for migrations, aggregate locks, concurrency, stock/capacity checks, balance movement, projection synchronization, idempotency, audit, and outbox.
- Mock WMS fixtures and mock event publisher: exercised.
- Redis: healthy but intentionally unused until BE-07.
- Kafka, real WMS, and OIDC: not integrated or verified.

## Remaining gaps and readiness

Inventory inquiry/transfer/cycle count remain BE-05 scope. Production Kafka, real WMS, OIDC, and Redis projections remain later phases. BE-04 exit criteria pass: all three workflows expose explicit validated steps and safe retry behavior. BE-05 is safe to begin.
