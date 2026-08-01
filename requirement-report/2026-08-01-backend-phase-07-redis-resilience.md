Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified in this phase.
- PDA backend repository: current working repository.
- Android project creation: not part of this phase.

# BE-07 Redis Cache and Resilience Report

Status: **APPROVED**

## Scope

Implemented Redis cache-aside, versioned keys, configurable TTL, cache metrics, mutation invalidation, stampede protection, distributed rate-limit support, gateway circuit behavior, and resilient upstream WMS boundaries. PostgreSQL remains authoritative for every command, idempotency record, inventory effect, audit, and outbox event.

## Cache implementation

- Versioned keys use `pda:v1:<view>:...` with scope components escaped.
- Cached reads: dashboard, task summary, short-lived inventory search/balances, shipment/readiness, and master WMS lookup.
- TTL is configured by `PDA_CACHE_TTL`; Redis uses `PDA_REDIS_URL`.
- Hit, miss, latency, and error counters are maintained.
- Singleflight suppresses concurrent cold-key stampedes.
- Results carry an internal stale marker when Redis and the authoritative loader both fail but an in-process prior value exists.
- Task, receiving, movement, inventory, and shipping invalidators clear affected views. Inventory events wildcard all operator dashboard/summary views.
- Redis clients are confined to `adapters/redis` by the architecture rule.

## Resilience implementation

- Redis I/O fails fast with 100 ms dial/read/write bounds and no client retries.
- Redis failures fall through to PostgreSQL; invalidation is best-effort after a committed mutation.
- A Redis fixed-window limiter supports distributed gateway rate limiting; the existing gateway limiter remains available during Redis outage.
- WMS boundaries use explicit timeouts, idempotent-only retry with jitter, `sony/gobreaker/v2` closed/open/half-open transitions, and channel bulkheads.
- Non-idempotent calls are attempted once and never receive fabricated fallback success.
- The gateway consecutive-failure circuit returns a retryable 503 while open and supports a half-open recovery trial.

## Verification

- BE-06 approval, clean full gate, PostgreSQL readiness, and Redis health reverified: PASS.
- versioned key and configurable TTL validation: PASS.
- cache miss → load, hit, latency metrics, and error metrics: PASS.
- concurrent cold-key stampede: one loader execution across 20 callers.
- stale marker and Redis-unavailable unit behavior: PASS.
- real Redis TTL, hit/miss, pattern invalidation, and distributed limiter: PASS.
- invalidation for task, receiving, movement, inventory, and shipping: PASS.
- breaker open/half-open/closed transitions: PASS.
- outbound timeout and exact retry count with preserved identity: PASS.
- bulkhead isolation and no mutation retry/fallback success: PASS.
- WMS boundary retry adapter: PASS.
- gateway circuit state/recovery and existing gateway rate/timeout tests: PASS.
- live Redis outage: dashboard read returned PostgreSQL data within gateway timeout.
- live Redis outage transfer: HTTP 200, exactly one PostgreSQL command row and one movement row.
- Redis restart and health: PASS.
- architecture, contract, unit, integration, build, and final `make clean verify`: PASS.
- Migration state remains clean at version 5; BE-07 adds no database migration.

## Failures and corrections

1. Architecture testing rejected direct `go-redis` imports in the platform cache core. Redis storage and limiter code moved into `internal/platform/cache/adapters/redis`; the core now depends only on a cache port.
2. The first retry test opened its breaker before the intended successful retry. Test policy thresholds were separated so retry and breaker state behavior are independently asserted.
3. A compatibility test constructed configuration directly without Redis fields. Validation now permits omitted optional cache fields while runtime loading supplies and validates defaults; non-positive configured TTL is rejected.
4. The real invalidation matrix found inventory events targeting an empty operator. Operatorless inventory invalidation now wildcards all operator-scoped dashboard and summary keys.
5. The first live outage showed default Redis retries exceeding the five-second gateway deadline. Redis was changed to 100 ms bounded I/O with client retries disabled; the repeated outage returned the read and committed command successfully.

No test or business assertion was disabled or weakened.

## Dependency verification and readiness

Real Redis and PostgreSQL were physically exercised. The deterministic mock WMS boundary exercised the resilience wrapper. Kafka, real WMS, and OIDC were not integrated or verified.

BE-07 exit criteria pass: reads degrade safely and commands remain database-correct during Redis or dependency failures. BE-08 is safe to begin.
