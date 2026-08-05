# PDA Backend Phase 11 Production Readiness

Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

Date: 2026-08-02
Status: PARTIALLY_APPROVED

## Authentication State

Before implementation: `AUTH_MOCK_ONLY`.

After implementation: `AUTH_PRODUCTION_IMPLEMENTED_BUT_NOT_VERIFIED`.

The normal gateway runtime now requires `PDA_AUTH_MODE=internal` and uses backend-owned PostgreSQL identity storage. The mock identity adapter remains reachable only from existing isolated tests; it is no longer selected by the gateway runtime. WMS remains an operational dependency and is not the identity provider.

## Implemented

- PostgreSQL identity migration `000007_identity`.
- Operators with status and Argon2id password hashes.
- Roles, permissions, warehouse scope, and device/operator/warehouse bindings.
- Durable sessions, opaque hashed refresh tokens, token families, rotation, reuse detection, revocation, and security audit storage.
- Signed access tokens with issuer, audience, session, operator, device, warehouse, issued-at, not-before, expiry, and `kid` claims; RS256 key loading and validation-key overlap are supported.
- Explicit internal-auth configuration with token issuer, audience, access TTL, refresh TTL, and secret validation.
- Gateway runtime composition using the PostgreSQL identity adapter and production session manager.
- Explicit `identity-seed-dev` command; fixture credentials are not loaded by the gateway.
- Docker Compose PostgreSQL/Redis services and internal-auth gateway profile.
- Kafka TLS configuration remains explicit and fail-closed.
- WMS HTTP adapter applies a bounded 10-second default client timeout; resilience policy supplies bounded retry, timeout, bulkhead, and circuit-breaker behavior for the supported warehouse-master port.

## Database Evidence

- PostgreSQL image: `postgres:17.5-alpine`.
- Redis image: `redis:8.0.2-alpine`.
- Docker Compose PostgreSQL/Redis services started and health checks passed.
- `make migrate-up` applied migration `7/u identity`.
- `make identity-seed-dev` inserted an Argon2id development operator, warehouse, role, permissions, and device.
- Identity tables verified in PostgreSQL: operators, roles, permissions, warehouse access, devices, sessions, refresh tokens, and security audit.

## HTTP Authentication Evidence

Against the real PostgreSQL-backed gateway:

- API-001 login passed with password hash verification, device binding, warehouse scope, roles, permissions, and registered-device response.
- API-004 bootstrap passed with PostgreSQL-backed operator, warehouse, device, and permission data.
- API-005 dashboard protected read passed with access-token and warehouse/device context.
- API-002 refresh passed with opaque token rotation.
- Refresh replay returned `401` and revoked the session family.
- The successor access token after refresh-family reuse returned `401`.
- Argon2id unit, identity, gateway, configuration, and command package tests passed.
- RS256 unit coverage passed for algorithm enforcement, `kid` validation, retained-key overlap, and unknown-key rejection.
- PostgreSQL integration test passed against Docker PostgreSQL using an isolated schema and the real `000007_identity` migration. It verifies durable session creation, refresh rotation, refresh-reuse session revocation, logout revocation, and access-token invalidation.

## Authentication Limitations

- HS256 remains available only as an explicit local compatibility mode. Production deployment should select RS256 with mounted keys and a secret-manager rotation procedure.
- Failed-login counters and five-attempt/15-minute account lockout are now wired to durable identity state; distributed abuse protection and lockout policy tuning remain open.
- Automated PostgreSQL coverage now includes concurrent refresh race handling, rehydrating a session manager against persisted state, and applying/removing the identity migration on an isolated empty schema.
- The development seed command is explicit and must never run automatically in production.

## All-Phase Mock and Deferred Audit

| Phase | Capability | Runtime implementation | Mock/fixture present | Outside-test activation | Real dependency evidence | Status | Required action |
|---|---|---|---|---:|---:|---|---|
| 00 | Contract/traceability | Backend/OpenAPI decisions | Documentation fixtures | No | Local | REAL_AND_VERIFIED | Keep matrix current |
| 01 | Transport/errors | Go HTTP gateway | Test fixtures | No | Local only | REAL_BUT_NOT_EXTERNALLY_VERIFIED | Run staging contract suite |
| 02 | Authentication | PostgreSQL identity runtime | Mock adapter in tests | No in gateway | Docker PostgreSQL | REAL_BUT_NOT_EXTERNALLY_VERIFIED | Add key rotation and staging evidence |
| 03 | Dashboard/tasks | PostgreSQL task store | WMS task fixtures seed runtime | Yes | PostgreSQL local | RUNTIME_MOCK_PRESENT | Replace fixture seed with approved WMS source |
| 04 | Receiving | PostgreSQL workflow store | WMS receiving fixtures | Yes | PostgreSQL local | RUNTIME_MOCK_PRESENT | Wire approved WMS receiving contract |
| 05 | Movement workflows | PostgreSQL workflow store | WMS movement fixtures | Yes | PostgreSQL local | RUNTIME_MOCK_PRESENT | Wire approved WMS movement contract |
| 06 | Inventory/transfer | PostgreSQL and Redis | Deterministic seed data | Yes | PostgreSQL/Redis local | RUNTIME_MOCK_PRESENT | Verify real WMS inventory events |
| 07 | Cycle count | PostgreSQL workflow store | Test fixtures | Yes | PostgreSQL local | REAL_BUT_NOT_EXTERNALLY_VERIFIED | Verify WMS count reconciliation |
| 08 | Shipping | PostgreSQL workflow store | Seed/test fixtures | Yes | PostgreSQL local | REAL_BUT_NOT_EXTERNALLY_VERIFIED | Verify WMS shipment contract |
| 09 | Command status/outbox | PostgreSQL durable records | Mock publisher local mode | Yes | Local Kafka only | REAL_BUT_NOT_EXTERNALLY_VERIFIED | Verify secured Kafka and replay |
| 10 | Cache/freshness/metrics | Redis cache and metrics | In-memory test stores | No normal gateway | Redis local | REAL_BUT_NOT_EXTERNALLY_VERIFIED | Export metrics and outage tests |
| 11 | External integration | Explicit adapters/configuration | WMS/Kafka mocks remain local | Yes by local modes | No staging evidence | BLOCKED_BY_EXTERNAL_DEPENDENCY | Supply external environments |

## Runtime Mock Audit

- Authentication mock: disconnected from gateway runtime; retained for isolated tests only.
- WMS mock: still seeds task, receiving, and movement fixtures when `PDA_UPSTREAM_WMS_MODE=mock`.
- Messaging mock: selected only when `PDA_MESSAGING_MODE=mock`; Kafka mode has no silent fallback.
- PostgreSQL: real runtime store and identity persistence.
- Redis: real cache runtime; mutation invalidation remains best effort.
- Workflow fixtures: runtime-reachable in mock WMS mode and not production WMS evidence.
- Zebra physical integration: unavailable in this backend repository.

## Remaining Phase 11 Blockers

1. Configure RS256 as the production signing mode through the secret manager and verify retired-key removal policy.
2. Add distributed abuse protection and production lockout policy tuning.
3. Supply and wire the approved real WMS API/event contract without fixture seeding; current HTTP support is limited to the warehouse-master port and gateway workflow composition remains fail-closed for non-mock WMS.
4. Verify Kafka TLS, least-privilege ACLs, broker restart/rebalance, ordering, outbox recovery, and DLQ replay.
5. Run API-001 through API-028 staging contract tests with the actual PDA client.
6. Verify physical Zebra DataWedge behavior.
7. Extend PostgreSQL integration tests for process-level restart durability.

## Verification Commands

Passed: `docker compose -f docker/compose.yml config --quiet`, Docker Compose PostgreSQL/Redis startup, `make migrate-up`, development identity seed, `PDA_TEST_DATABASE_URL=... go test -tags=integration ./internal/identity/adapters/postgres`, focused Go tests, `make build`, `make test-kafka`, and `git diff --check`.

The repository-wide architecture scanner still reports the pre-existing false positive caused by the supplied PDA Android/JVM requirement documents. Phase 11 cannot become `APPROVED` until the external blockers and remaining authentication hardening are verified.
