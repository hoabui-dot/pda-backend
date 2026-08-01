# Backend Phase BE-10 Prompt — Production Hardening, Observability, Security, and Full E2E


> **Repository boundary:** Work only in the new PDA backend repository. The existing Kotlin Android `PDA_APP` is an external API client. Do not create, modify, build, or verify Android code in this phase.

## Objective

Produce release-level evidence for the enterprise PDA backend.

## Tasks

1. Replace mock auth with approved OIDC in production.
2. Implement service-to-service authentication.
3. Enforce RBAC/ABAC, device, and warehouse scopes.
4. Add OpenTelemetry traces, metrics, and structured logs.
5. Add audit export and retention.
6. Add Kubernetes manifests/Helm or approved deployment format.
7. Add probes, resource limits, PDBs, autoscaling, and network policies.
8. Add database backup/restore and migration runbooks.
9. Add Kafka/Redis/PostgreSQL operational runbooks.
10. Run load, soak, concurrency, and failure-injection tests.
11. Run full PDA-backend E2E.
12. Test:
    - duplicate commands;
    - task locks;
    - stale versions;
    - Kafka outage;
    - Redis outage;
    - WMS outage;
    - token expiry;
    - service restart;
    - database failover where available.
13. Ensure production profile rejects all mock modes.
14. Produce final architecture/context and release-readiness reports.

## Exit criteria

- no unresolved P0/P1 defect;
- all automated suites pass;
- staging E2E passes;
- operational and security reviews pass;
- production artifact is reproducible;
- mock adapters cannot activate in production.

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