# Backend Phase BE-06 Prompt — Shipping Confirmation and Cross-Domain State


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Implement SCR-16 backend support and cross-domain readiness projections.

## Tasks

1. Model shipment, packages, readiness, carrier, tracking, and version.
2. Implement summary and readiness APIs.
3. Consume/project picking completion through direct application port or mock events.
4. Confirm shipment only when all prerequisites pass.
5. Require online authoritative confirmation.
6. Add idempotent shipping command and command status.
7. Add outbox events:
   - `ShipmentReadinessChanged`
   - `ShipmentConfirmed`
   - `OrderShipped`
8. Update dashboard/task projections.
9. Do not implement label printing unless separately approved.
10. Add full backend E2E from receiving through shipment using deterministic fixtures.

## Tests

- not ready;
- package incomplete;
- invalid carrier/tracking;
- stale version;
- duplicate confirmation;
- cross-domain readiness;
- event/outbox;
- full workflow E2E.

## Exit criteria

Shipment confirmation cannot be fabricated, duplicated, or completed before readiness.

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