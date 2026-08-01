# Backend Phase BE-00 Prompt — Architecture Guardrails and Mock Adapters

## Repository Boundary

The backend repository was already created by PRE-00.

The existing Android `PDA_APP` remains an external client. Do not create or modify Android code.

## Objective

Add enterprise architecture boundaries, shared contracts, mock adapters, architecture tests, and production mock-mode safeguards to the new backend repository.

## Prerequisite

PRE-00 report must state `APPROVED`.

Run the existing backend baseline:

```shell
make clean build
```

This is the backend baseline created in PRE-00, not an Android build.

## Tasks

1. Establish hexagonal package structures in each service.
2. Add architecture tests enforcing:
   - domain does not depend on HTTP/infrastructure packages;
   - controllers do not access persistence directly;
   - Kafka types remain in infrastructure;
   - Redis types remain in infrastructure;
   - DTOs/entities do not leak into domain APIs.
3. Define:
   - `DomainError`
   - `ActorContext`
   - `CommandMetadata`
   - `DomainEventEnvelope`
   - `DomainEventPublisher`
4. Implement:
   - `MockDomainEventPublisher`
   - mock event log
   - deterministic fixture loader
   - mock upstream WMS adapter
5. Add production startup guards rejecting:
   - `messaging.mode=mock`
   - `upstream-wms.mode=mock`
   - `auth.mode=mock`
6. Add application configuration validation.
7. Add initial outbox and inbox abstractions without enabling Kafka.
8. Add coding, package, and architecture documentation.
9. Add unit and architecture tests.
10. Do not implement Android code, real Kafka, real WMS, or production OIDC.

## Verification

- backend clean build;
- architecture tests;
- application context tests;
- mock event publication tests;
- deterministic fixture tests;
- production mock-mode rejection;
- no Android plugin/source/package check.

## Required report

Create:

```text
requirement-report/YYYY-MM-DD-backend-phase-00-architecture-guardrails.md
```

The report must explicitly confirm that only the backend repository was modified.

## Exit Criteria

- architecture boundaries are enforced;
- mock adapters are fully testable;
- production profiles reject mock modes;
- Kafka is not required;
- BE-01 is safe to begin.
