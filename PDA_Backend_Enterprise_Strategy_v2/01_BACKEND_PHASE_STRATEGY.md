# PDA WMS Backend — Phased Delivery Strategy

| Phase | Name | Primary result |
|---:|---|---|
| PRE-00 | New Backend Repository Bootstrap | create the backend repository from zero and produce the first successful backend build |
| BE-00 | Architecture Guardrails and Mock Adapters | compiling multi-module architecture with tested boundaries |
| BE-01 | Gateway, Mock Auth, Device Context | secured public API boundary |
| BE-02 | Task Core, Dashboard, Task Center | PDA read model and task lifecycle |
| BE-03 | Receiving Reference Vertical Slice | transaction, idempotency, outbox, mock event |
| BE-04 | Putaway, Picking, Replenishment | movement workflows |
| BE-05 | Inventory, Transfer, Cycle Count | inventory commands and inquiry |
| BE-06 | Shipping and Cross-Domain State | readiness and final confirmation |
| BE-07 | Redis and Resilience | cache, circuit breaker, timeout, bulkhead |
| BE-08 | Kafka Producer/Consumer Enablement | real asynchronous integration |
| BE-09 | Upstream WMS Integration | anti-corruption adapter and sync |
| BE-10 | Production Hardening and E2E | security, observability, performance, release readiness |

## Dependency rule

PRE-00 is the first phase and has no prior backend baseline dependency.

A later phase may begin only after the previous phase's report states `APPROVED`.

The existing Android PDA repository is not a previous backend phase and must not be used as the backend baseline.

## Initial Kafka condition

Phases PRE-00 through BE-07 run with:

```yaml
pda.messaging.mode: mock
```

Kafka configuration remains disabled by conditional configuration. BE-08 enables and verifies Kafka. The system must not require a running Kafka broker before BE-08.

## PDA/backend synchronization

Backend phases should align with mobile phases:

| Mobile phase | Backend support |
|---|---|
| PDA Phase 1 UI/navigation | BE-00 to BE-02 mock endpoints/OpenAPI |
| PDA Phase 2 receiving | BE-03 |
| PDA Phase 3 movement/inventory | BE-04 and BE-05 |
| PDA Phase 4 shipping/E2E | BE-06 |
| PDA Phase 5 scanner | no backend scanner dependency beyond barcode APIs |
| PDA Phase 6 Room/offline | BE-03+ idempotent commands and command-status endpoints |
| PDA Phase 7 network/auth | BE-01 and BE-07 |
| PDA Phase 8 endpoint migration | BE-03 through BE-09 |
| PDA Phase 9 final E2E | BE-10 |

## Required verification per phase

- unit tests;
- integration tests;
- OpenAPI/contract tests;
- database migration tests;
- architecture tests;
- build;
- Docker Compose smoke test;
- end-to-end scenario;
- phase report;
- updated architecture/context.

If Kafka or WMS is unavailable, mock mode must be tested and a blocked external-integration report must be created.
