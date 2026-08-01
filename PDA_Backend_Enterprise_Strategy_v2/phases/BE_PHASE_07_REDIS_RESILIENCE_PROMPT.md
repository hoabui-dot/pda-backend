# Backend Phase BE-07 Prompt — Redis Cache and Resilience Hardening


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Add Redis cache-aside, cache invalidation, rate limiting support, and enterprise resilience policies without changing business correctness.

## Tasks

1. Implement versioned cache-key service.
2. Cache dashboard, task summaries, master lookups, short-lived inventory inquiry, and shipment readiness.
3. Add configurable TTL.
4. Add metrics for hit/miss/latency/error.
5. Add mutation-driven invalidation.
6. Add stampede protection where load tests justify it.
7. Add explicit outbound timeouts.
8. Add `sony/gobreaker` circuit breakers for upstream/mock WMS client boundaries.
9. Add safe retries with jitter for idempotent calls.
10. Add bulkheads.
11. Add gateway circuit breaker and rate limiter.
12. Verify Redis outage does not corrupt commands.
13. Do not store idempotency truth only in Redis.

## Tests

- cache hit/miss;
- invalidation after each domain event;
- stale read marker;
- Redis unavailable;
- circuit open/half-open/closed;
- timeout;
- retry count and preserved idempotency;
- bulkhead isolation;
- no fallback success for mutations.

## Exit criteria

Reads degrade safely and commands remain database-correct during Redis/dependency failures.

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
