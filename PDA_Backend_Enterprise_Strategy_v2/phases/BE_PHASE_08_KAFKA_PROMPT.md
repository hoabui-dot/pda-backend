# Backend Phase BE-08 Prompt — Kafka Producer and Consumer Enablement


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Enable Kafka without changing domain/application code and preserve mock mode for tests/local fallback.

## Prerequisites

- Kafka broker and security details are available.
- Topic naming and event schema are approved.
- Outbox events are stable.
- Mock mode tests pass.

## Tasks

1. Enable conditional `KafkaMessagingConfiguration`.
2. Remove only the temporary bootstrap comment/TODO that disables Kafka.
3. Keep `messaging.mode=mock` supported outside production.
4. Configure serializers/schema strategy.
5. Configure producer idempotence, acknowledgments, retries, and stable keys.
6. Implement outbox publisher batching with `SKIP LOCKED`.
7. Implement consumer groups and inbox idempotency.
8. Implement bounded retry and DLQ.
9. Add Kafka ACL/config documentation.
10. Add consumer lag and publisher backlog metrics.
11. Add Testcontainers for Go Kafka integration tests.
12. Verify event ordering per aggregate.
13. Verify broker outage leaves outbox pending and APIs truthful.
14. Never silently fall back from configured production Kafka mode to mock mode.

## Tests

- producer publish;
- duplicate publish handling;
- consumer duplicate;
- poison event/DLQ;
- ordering;
- broker outage/recovery;
- outbox backlog;
- ACL/auth failure where environment permits;
- mock/kafka parity.

## Blocked behavior

If Kafka is still unavailable, do not fake successful Kafka verification. Produce a blocked report and keep mock mode active.

## Exit criteria

Kafka mode is externally verified and domain behavior remains identical to mock mode.

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
