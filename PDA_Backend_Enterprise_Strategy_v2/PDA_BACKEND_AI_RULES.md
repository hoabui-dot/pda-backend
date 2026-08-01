# PDA WMS Backend — AI Implementation Rules

- **Rule ID:** `PDA-BE-RULES-001`
- **Applies to:** every AI agent creating or modifying the new backend repository.

---

## 0. Repository Boundary

- The existing Kotlin Android `PDA_APP` is an external client.
- The backend is a separate new repository.
- Never create, rebuild, move, or modify the Android application during a backend phase.
- Never run Android Gradle, ADB, emulator, Compose, or Zebra verification as backend phase evidence.
- PRE-00 must create the backend repository from zero.
- Before PRE-00, the absence of a backend build is expected and is not a failure.
- Every phase report must explicitly confirm the repository boundary.

## 1. Authority and Scope

1. Implement only the assigned backend phase.
2. Public API behavior must map to the approved PDA screens.
3. Do not add LPN, pallet, stock-adjustment, printer, or other deferred domains unless separately approved.
4. Do not connect directly to the WMS database.
5. Do not claim Kafka, WMS, OIDC, or production security verification without the real dependency.
6. When a contract is missing, create a blocked report rather than inventing a production contract.

---

## 2. Service and Code Rules

- Use idiomatic Go with explicit domain types, validated constructors, and nil-safe APIs.
- Controllers contain transport mapping only.
- Application services own use-case orchestration and transaction boundaries.
- Domain models do not depend on HTTP routers, PostgreSQL drivers, Kafka, Redis, or transport classes.
- Infrastructure implements ports.
- Do not expose persistence entities or API DTOs across boundaries.
- Keep mutable state private.
- Use explicit constructor functions and dependency injection through interfaces.
- No static global service locator.
- No broad `catch (Exception)` that converts failure to success.
- No silent fallback from production Kafka/WMS mode to mock mode.
- All configuration is validated at startup.
- Production profile must fail fast when mock mode is enabled.

---

## 3. Data Integrity Rules

- PostgreSQL is authoritative for backend-owned transactions.
- Redis is never authoritative.
- Every mutation has an idempotency key.
- Every mutable aggregate has a version.
- Use database transactions for mutation plus outbox insertion.
- Do not publish a Kafka event before database commit.
- Do not acknowledge a command before its required database mutation commits.
- Consumer processing must be idempotent.
- Do not rely only on in-memory locks.
- Prefer optimistic locking; use database row/advisory locking where the business invariant requires it.
- Never auto-adjust inventory from a cycle-count variance without approved workflow.
- Never fabricate shipment success during dependency failure.

---

## 4. Kafka and Mock Mode Rules

Define:

```go
type DomainEventPublisher interface {
    Publish(ctx context.Context, event DomainEventEnvelope) error
}
```

Implement:

- `MockDomainEventPublisher`
- `KafkaDomainEventPublisher`

Rules:

- Business code never imports Kafka producer types.
- Mock mode receives the exact same event envelope.
- Mock mode is explicit configuration.
- Kafka wiring may be temporarily commented only in configuration/bootstrap code and must have a phase/TODO marker.
- Do not comment domain event creation or outbox behavior.
- Production startup must reject `messaging.mode=mock`.
- Kafka events require event ID, version, aggregate version, correlation, causation, warehouse, operator, and device metadata.
- Producers use stable aggregate keys.
- Consumers maintain an inbox table.
- Poison messages go to DLQ with original metadata.

---

## 5. Cache Rules

- Use cache-aside.
- Cache keys include versioned namespace and warehouse scope.
- TTL is configuration.
- Mutations evict/update affected keys after commit.
- A cache failure must not make a valid database mutation fail unless the explicit operation requires Redis.
- Never authorize a mutation solely from cached data.
- Prevent cache stampede for high-volume reads with bounded single-flight or short lock strategy where needed.
- Return freshness/stale metadata for cached operational values.
- Do not cache sensitive tokens or full authentication credentials.

---

## 6. Resilience Rules

- Every outbound call has explicit timeout.
- Retry only safe/idempotent operations.
- Preserve the same idempotency key across retry.
- Apply circuit breaker to unstable dependencies, not local domain validation.
- Use bulkheads for WMS calls, Kafka consumers, and background publishers.
- Fallback may serve stale read data with a clear marker.
- Fallback may not return a fake successful warehouse mutation.
- Gateway retry for write commands is disabled unless explicitly reviewed.

---

## 7. Security Rules

- Validate token at gateway and service.
- Enforce permissions in the application/domain layer.
- Validate operator, device, and warehouse scope for every command.
- Never trust PDA-provided operator/role without token-derived verification.
- Never log tokens, passwords, secrets, or unrestricted barcode payloads.
- Use secret manager/environment injection.
- Apply least privilege to database and Kafka ACLs.
- Audit successful and denied mutations.

---

## 8. API Rules

- Maintain `/api/pda/v1`.
- Use cursor pagination.
- Use UTC ISO-8601.
- Return correlation ID and server time.
- Use typed stable error codes.
- Do not leak stack traces or raw dependency errors.
- Use `Idempotency-Key` for writes.
- Use `If-Match`/version for conflict-sensitive commands.
- Validate request size and fields.
- Generate and validate OpenAPI in CI.
- Add contract tests for each endpoint.

---

## 9. Testing Rules

Every phase must add:

- unit tests;
- repository/database integration tests;
- API tests;
- architecture tests;
- relevant E2E tests.

Use Testcontainers for Go for PostgreSQL, Redis, and Kafka when those integrations are enabled. Do not replace real integration tests with mocks once the dependency phase is active.

Required command targets:

```text
make test
make test-integration
make test-contract
make test-architecture
make build
```

When Docker/Kafka is unavailable, run all unaffected suites and create a blocked report.

---

## 10. Verification and Self-Correction

Before editing:

1. Read the repository-boundary document, architecture, contract map, rules, and phase prompt.
2. For PRE-00, do not search for a pre-existing backend build; create the repository first.
3. For BE-00 and later, run the existing backend baseline tests.
3. Inspect affected modules and contracts.
4. Create a phase report checklist.

When failure occurs:

1. capture exact command/error;
2. classify source, test, environment, dependency, or contract failure;
3. fix root cause;
4. rerun focused test;
5. rerun feature suite;
6. rerun full regression;
7. document correction.

Never:

- disable a valid test;
- weaken assertions;
- hardcode success in production code;
- ignore migration failure;
- continue to the next phase with unresolved data-integrity failure.

---

## 11. Cross-Service Workflow Verification

After every transactional phase, verify:

```text
API command
→ authentication/authorization
→ application service
→ aggregate validation
→ database transaction
→ idempotency record
→ outbox record
→ cache invalidation
→ API response
→ mock/Kafka publication
→ consumer/inbox
→ downstream projection
→ audit trace
```

If Kafka is disabled, verify through `MockDomainEventPublisher` and `mock_event_log`.

---

## 12. Required Phase Report

Create:

```text
requirement-report/YYYY-MM-DD-backend-phase-<N>-<name>.md
```

Include:

- scope;
- files changed;
- services/modules affected;
- database migrations;
- endpoints;
- events;
- caches;
- resilience;
- tests;
- commands/results;
- failures/corrections;
- dependency verification;
- remaining gaps;
- next-phase readiness.

---

## 13. Definition of Done

A backend phase is complete only when:

- exit criteria pass;
- API maps to required PDA behavior;
- tests/build pass;
- database migration is reversible/tested where applicable;
- idempotency/version behavior is verified;
- mock event path is verified;
- Kafka/WMS physical status is reported truthfully;
- documentation and OpenAPI are updated;
- no P0/P1 defect introduced by the phase remains.
