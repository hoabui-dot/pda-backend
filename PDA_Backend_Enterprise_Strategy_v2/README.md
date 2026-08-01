# PDA Backend Enterprise Strategy v2

This package replaces the earlier backend strategy package.

The authoritative backend implementation stack is **Go** (`net/http` + `chi`, `pgx`, Go modules, and Make). Java, Kotlin, Spring, and Gradle are not backend technologies in this strategy. The external PDA Android client remains Kotlin and is outside this repository.

The critical correction is that the backend is a **new, separate repository**. The existing Kotlin Android `PDA_APP` is an external client and must not be recreated, modified, or built by backend phases.

## Read first

1. `00_REPOSITORY_BOUNDARY.md`
2. `00_ENTERPRISE_BACKEND_ARCHITECTURE.md`
3. `01_BACKEND_PHASE_STRATEGY.md`
4. `02_API_AND_EVENT_CONTRACT_MAP.md`
5. `03_BACKEND_CODE_PATTERNS.md`
6. `PDA_BACKEND_AI_RULES.md`

## Correct phase order

```text
PRE-00 — Create the new backend repository
BE-00  — Add architecture guardrails and mock adapters
BE-01  — Gateway, authentication, and device context
BE-02  — Dashboard and Task Center
BE-03  — Receiving
BE-04  — Putaway, Picking, Replenishment
BE-05  — Inventory, Transfer, Cycle Count
BE-06  — Shipping
BE-07  — Redis and resilience
BE-08  — Kafka enablement
BE-09  — WMS integration
BE-10  — Production hardening and E2E
```

## Initial runtime mode

```yaml
pda:
  messaging:
    mode: mock
  upstream-wms:
    mode: mock
  auth:
    mode: mock
```

Kafka is not required before BE-08.

## Important

A missing backend build before PRE-00 is expected. PRE-00 creates the repository and the first build.

Android build verification is never a PRE-00 or BE-00 task.
