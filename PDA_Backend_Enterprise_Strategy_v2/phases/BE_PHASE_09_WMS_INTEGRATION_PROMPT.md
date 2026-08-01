# Backend Phase BE-09 Prompt — Upstream WMS Integration and Anti-Corruption Layer


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Replace hardcoded WMS fixtures with approved WMS APIs/events while preserving the PDA public contract.

## Tasks

1. Obtain approved WMS OpenAPI/event/database ownership contract.
2. Implement `UpstreamWmsPort`.
3. Keep `MockUpstreamWmsAdapter` for tests/local mode.
4. Implement HTTP and/or Kafka WMS adapters behind the port.
5. Map WMS identifiers/status/errors into PDA domain types.
6. Implement synchronization checkpoints and replay.
7. Handle upstream unavailability with safe cached reads and blocked writes according to policy.
8. Add circuit breaker, timeout, retry, and bulkhead.
9. Add reconciliation jobs/reports.
10. Add master-data and task-event consumers.
11. Verify no PDA service calls the WMS database directly.
12. Add contract tests and staging E2E.

## Tests

- DTO/event mapping;
- unknown status;
- duplicate WMS event;
- out-of-order version;
- WMS outage;
- checkpoint/replay;
- reconciliation mismatch;
- staging happy/failure paths.

## Blocked behavior

If WMS contracts are unavailable, implement only interfaces and mocks, create a blocked report, and do not invent final endpoints.

## Exit criteria

The backend uses the approved WMS integration and keeps the PDA API stable.

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