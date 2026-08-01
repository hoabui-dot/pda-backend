# Backend Phase BE-01 Prompt — API Gateway, Mock Authentication, and Device Context


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Implement the public PDA API boundary, mock login/bootstrap, correlation, operator/device/warehouse context, and authorization scaffolding.

## Tasks

1. Configure the Go HTTP gateway/BFF routes under `/api/pda/v1`.
2. Add correlation-ID generation/propagation.
3. Add request logging with redaction.
4. Add rate limiting for auth endpoints.
5. Add route timeouts and circuit-breaker configuration.
6. Implement mock auth endpoints using deterministic operator fixtures.
7. Implement `/me`, `/me/warehouses`, device registration, and `/bootstrap`.
8. Add JWT-shaped mock tokens or approved local token mechanism that is clearly non-production.
9. Add service authorization abstractions.
10. Validate `X-Device-Id` and `X-Warehouse-Id`.
11. Add OpenAPI definitions.
12. Keep future OIDC adapters behind ports.
13. Do not implement warehouse business mutations.

## Tests

- valid/invalid login;
- rate limit;
- correlation propagation;
- unauthorized/forbidden;
- wrong warehouse/device;
- gateway route and timeout;
- production mock-auth guard;
- OpenAPI contract.

## Exit criteria

The PDA can authenticate in local mode and obtain a trusted operator/device/warehouse bootstrap context.

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
