# Master Execution Prompt — Implement Production Authentication, Eliminate Mock Runtime Dependencies, and Complete Phase 11

## 1. Role

You are working inside the existing Go repository:

```text
PDA_BACKEND
```

Act as a:

* senior Go backend engineer;
* enterprise identity and access architect;
* PostgreSQL database engineer;
* API security engineer;
* WMS integration architect;
* Kafka production-readiness engineer;
* Docker and integration-test engineer;
* PDA mobile-backend integration specialist;
* release-readiness auditor.

Your task is to replace the current mock-only authentication with a complete backend-owned production authentication implementation, run it against a real PostgreSQL database through Docker Compose, verify the complete PDA authentication flow, resume Phase 11, and produce a final Phase 11 report that audits all previous phases for remaining mock implementations or unverified mock behavior.

Do not introduce OIDC unless the approved architecture explicitly selects it.

The approved default architecture for this task is:

```text
PDA Backend owns authentication.
WMS is an operational warehouse dependency.
WMS is not the PDA identity provider.
```

---

# 2. Repository Boundary

The systems remain separate:

```text
PDA_APP
Existing Kotlin Android application
External client
Must not be modified by this task

PDA_BACKEND
Current Go backend repository
Owns:
- authentication;
- token lifecycle;
- sessions;
- operator identity;
- roles and permissions;
- device registration;
- warehouse authorization;
- public PDA APIs;
- workflow orchestration;
- PostgreSQL persistence;
- Redis;
- Kafka;
- WMS integration.
```

Do not:

* create or modify Android code;
* run Android Gradle tasks;
* recreate the PDA App;
* connect the PDA App directly to PostgreSQL, Redis, Kafka, or WMS;
* use the WMS as the PDA identity provider without an approved architecture decision;
* report mock authentication as production authentication;
* report local unit tests as full production verification.

---

# 3. Mandatory Documents

Before editing code, read:

```text
PDA_BACKEND_RECONCILIATION_RULES_V2.md
PDA_APP_API_SPECIFICATION.md
PDA_BACKEND_PDA_APP_AUTHORITATIVE_TRACEABILITY.md
api/openapi/pda-v1.yaml
PHASE_11_EXTERNAL_OIDC_WMS_KAFKA_TLS_ZEBRA_E2E.md
Phase 11 Authentication Verification Report
COMMON-DEFERRED-VERIFICATION.md
```

Also inspect all reports from:

```text
PRE-00
BE-00 through BE-10
PDA reconciliation Phase 00 through Phase 10
Phase 11
```

The actual PDA API specification is the client integration contract for:

* API-001 through API-028;
* request fields;
* response fields;
* authentication;
* device context;
* warehouse scope;
* headers;
* errors;
* command status;
* scanner payloads;
* pagination;
* offline retry;
* freshness;
* invalidation.

---

# 4. Primary Objectives

Complete all of the following:

1. Remove the current mock authentication implementation from executable runtime composition.
2. Remove hardcoded fixture credentials from backend runtime.
3. Remove direct plaintext password comparison.
4. Remove in-memory session and refresh-token truth.
5. Remove in-memory revocation as the production mechanism.
6. Remove in-memory device registration as production truth.
7. Remove in-memory authentication audit as production truth.
8. Add real PostgreSQL-backed identity persistence.
9. Add real database migrations.
10. Add PostgreSQL to Docker Compose when not already available.
11. Start the real database and run migrations.
12. Implement secure password verification.
13. Implement access tokens.
14. Implement durable sessions.
15. Implement durable refresh-token rotation.
16. Implement refresh-token reuse detection.
17. Implement durable logout and revocation.
18. Implement operator, role, permission, warehouse, and device authorization.
19. Reconcile authentication with the PDA App API contract.
20. Run full authentication tests against the real database.
21. Run complete HTTP E2E authentication flow.
22. Restart services and verify authentication state remains correct.
23. Resume the remaining Phase 11 WMS, Kafka, infrastructure, PDA API, and Zebra checks.
24. Audit all earlier phases and identify every remaining mock, fixture, stub, placeholder, deferred adapter, or unverified production dependency.
25. Record that audit directly in the updated Phase 11 report.

---

# 5. Mock Elimination Policy

## 5.1 Authentication Runtime

Delete or remove from executable runtime:

```text
internal/identity/adapters/mock/token.go
internal/identity/adapters/mock/store.go
internal/identity/adapters/mock/testdata/identity.json
```

Also remove:

* mock authentication composition;
* `PDA_AUTH_MODE=mock`;
* deterministic authentication tokens;
* fixture users;
* fixture passwords;
* in-memory auth revocation maps;
* in-memory auth audit records;
* runtime fallback to mock identity;
* production configuration branches that preserve mock authentication.

If files cannot be physically deleted because a migration step still references them, remove all executable references first, migrate tests, then delete them before phase completion.

Production and normal local Docker Compose execution must use the real identity implementation.

## 5.2 Test Doubles

Do not use the removed production mock adapter as proof of correctness.

Focused unit-test fakes may remain only when:

* they are local to the test package;
* they are not selectable by runtime configuration;
* they cannot be activated in any deployed binary;
* integration and E2E tests also exercise the real PostgreSQL implementation.

The final Phase 11 report must clearly distinguish:

```text
Test-only fake
Runtime mock
External dependency simulator
Real integration
Externally verified production dependency
```

## 5.3 Other Backend Phases

Do not automatically delete every deterministic test fixture used for isolated tests.

Instead, audit every phase and classify each mock or fixture.

Any runtime mock that can activate in staging or production must be removed or blocked by fail-fast configuration.

Any domain feature still implemented only through mock data must be reported as incomplete.

---

# 6. Approved Production Identity Architecture

Implement a dedicated identity module owned by PDA Backend.

Recommended logical structure:

```text
internal/identity/
  domain/
  application/
  ports/
  adapters/http/
  adapters/postgres/
  adapters/token/
  adapters/security/
```

Business and application code must depend on ports, not PostgreSQL or JWT libraries directly.

Required production components:

```text
PostgreSQL operator repository
PostgreSQL role and permission repository
PostgreSQL warehouse-access repository
PostgreSQL device repository
PostgreSQL session repository
PostgreSQL refresh-token repository
PostgreSQL security-audit repository
Password hashing service
Access-token issuer and validator
Refresh-token generator and hasher
Login rate-limit and abuse-protection boundary
```

No identity domain package may import HTTP, PostgreSQL, Redis, Kafka, or transport DTO packages.

---

# 7. PostgreSQL Data Model

Add versioned migrations for at least the following tables.

## 7.1 Operators

```text
identity_operators
```

Recommended fields:

```text
id
username
employee_code
display_name
password_hash
status
locale
shift_code
failed_login_count
locked_until
password_changed_at
last_login_at
created_at
updated_at
version
```

Constraints:

* unique normalized username;
* unique employee code where required;
* no plaintext password;
* explicit active, disabled, locked, and deleted behavior;
* optimistic version where mutable administrative updates require it.

## 7.2 Roles

```text
identity_roles
identity_operator_roles
```

Recommended fields:

```text
role_id
role_code
role_name
active
operator_id
created_at
```

## 7.3 Permissions

```text
identity_permissions
identity_role_permissions
```

Recommended fields:

```text
permission_id
permission_code
permission_name
role_id
created_at
```

## 7.4 Warehouses

```text
identity_operator_warehouses
```

Recommended fields:

```text
operator_id
warehouse_id
is_default
active
created_at
updated_at
```

The warehouse ID may reference a backend-owned warehouse projection or approved WMS identifier, but authorization remains enforced by PDA Backend.

## 7.5 Devices

```text
identity_devices
```

Recommended fields:

```text
id
device_code
device_model
app_version
status
approved_at
revoked_at
last_seen_at
created_at
updated_at
version
```

Optional mapping tables:

```text
identity_operator_devices
identity_device_warehouses
```

Use them when the policy binds devices to operators or warehouses.

## 7.6 Sessions

```text
identity_sessions
```

Recommended fields:

```text
id
operator_id
device_id
warehouse_id
status
created_at
last_seen_at
expires_at
revoked_at
revocation_reason
created_ip_hash
created_user_agent
version
```

Do not store unrestricted sensitive client data.

## 7.7 Refresh Tokens

```text
identity_refresh_tokens
```

Recommended fields:

```text
id
session_id
token_hash
token_family_id
parent_token_id
issued_at
expires_at
used_at
revoked_at
replaced_by_token_id
reuse_detected_at
created_at
```

Store only a secure hash of the refresh token.

## 7.8 Signing Keys

If signing keys are tracked in the database, store only safe public metadata.

Private signing keys must be loaded from:

* deployment secret manager;
* protected mounted secret file;
* approved key-management service.

Do not commit private keys.

## 7.9 Security Audit

```text
identity_security_audit
```

Recommended fields:

```text
id
event_type
operator_id
session_id
device_id
warehouse_id
correlation_id
outcome
safe_error_code
source_ip_hash
metadata_json
created_at
```

Never record:

* passwords;
* access tokens;
* refresh tokens;
* private keys;
* unrestricted sensitive barcode payloads.

---

# 8. Password Security

Use a reviewed password hashing implementation.

Preferred:

```text
Argon2id
```

A configured bcrypt implementation may be used only when already approved by project standards.

Requirements:

* random salt;
* configurable parameters;
* constant-time comparison where applicable;
* password hash rehash detection;
* no plaintext persistence;
* no plaintext logging;
* no password returned through API;
* safe generic invalid-credential response;
* secure seed/bootstrap process.

Provide a development seed command or migration that creates test users using hashed passwords.

Do not commit real production passwords.

Docker Compose test credentials may be injected through environment variables or a local-only seed mechanism.

---

# 9. Access Token Implementation

Implement signed access tokens.

Recommended production model:

```text
asymmetric signing
```

For example:

```text
RS256 or ES256
```

Use an explicit algorithm allowlist.

Required claims:

```text
iss
aud
sub
jti
iat
nbf
exp
session_id
operator_id
device_id or device-session reference
warehouse scope or selected warehouse
roles or permission references as approved
```

Do not trust arbitrary claims without signature validation.

Validate:

* signing algorithm;
* signature;
* issuer;
* audience;
* expiry;
* not-before;
* token ID;
* session state;
* operator state;
* device state;
* warehouse authorization;
* key ID;
* clock skew.

When roles or permissions can change during a token lifetime, define whether each request:

* trusts short-lived claims;
* reloads authorization state;
* validates a permission/version marker;
* checks session/operator status.

Document the chosen consistency model.

---

# 10. Signing Key Management

Implement:

* key ID;
* active signing key;
* validation-key set;
* rotation;
* overlap period;
* retired-key behavior;
* startup validation;
* safe failure when keys are missing.

For local Docker Compose testing, mount test-only generated keys through a local secure path.

Do not:

* commit private key material;
* print key material;
* expose keys through health endpoints;
* silently generate ephemeral production keys on every restart.

Service restart must not invalidate every access token unless explicitly designed.

---

# 11. Refresh Token and Session Flow

Refresh tokens must be:

* cryptographically random;
* opaque;
* hashed before persistence;
* bound to a session;
* bound to the approved device policy;
* short enough for safe transport and sufficiently random;
* rotated after successful use;
* revocable;
* durable after restart.

Implement refresh-token families.

On successful refresh:

```text
Validate token
→ lock token/session row
→ verify not expired/revoked/used
→ mark current token used
→ create replacement token
→ link replacement
→ issue new access token
→ commit transaction
```

On reuse of an already-used refresh token:

```text
Detect reuse
→ revoke the token family or session
→ record security event
→ reject refresh
→ require login
```

Concurrent refresh requests must not produce two valid successor chains.

Use database locking or an equivalent transactional invariant.

---

# 12. Authentication API Contract

Reconcile the implementation with the actual PDA App API specification.

## API-001 — Login

```http
POST /api/pda/v1/auth/login
```

Canonical request:

```json
{
  "username": "warehouse.operator",
  "password": "<redacted>",
  "deviceId": "TC26-001",
  "deviceModel": "TC26",
  "appVersion": "1.0",
  "warehouseId": "MAIN",
  "locale": "vi-VN"
}
```

Required server flow:

```text
Validate request
→ normalize username
→ load operator
→ verify operator status
→ verify password hash
→ apply failed-login policy
→ verify or register device according to approved policy
→ verify device status
→ verify requested warehouse access
→ load roles and permissions
→ create durable session
→ create durable refresh-token record
→ issue access token
→ update last login
→ write security audit
→ return canonical response
```

Login must not issue tokens before device and warehouse policy is resolved.

Canonical response capabilities:

```json
{
  "data": {
    "accessToken": "<redacted>",
    "refreshToken": "<redacted>",
    "tokenType": "Bearer",
    "expiresAt": "2026-08-02T03:00:00Z",
    "operatorId": "EMP-0001",
    "employeeCode": "EMP-0001",
    "displayName": "Warehouse Operator",
    "roles": [
      "RECEIVING_OPERATOR"
    ],
    "permissions": [
      "RECEIVE"
    ],
    "warehouseId": "MAIN",
    "warehouseName": "Main Warehouse",
    "shiftCode": "DAY",
    "deviceRegistrationStatus": "REGISTERED",
    "featureFlags": {},
    "scannerPolicy": {}
  },
  "meta": {
    "serverTime": "2026-08-02T02:00:00Z",
    "correlationId": "uuid"
  },
  "errors": []
}
```

Reconcile exact field names and requiredness against the approved contract and OpenAPI.

## API-002 — Refresh

```http
POST /api/pda/v1/auth/refresh
```

Implement:

* opaque refresh token;
* device context;
* durable lookup;
* rotation;
* reuse detection;
* session validation;
* operator validation;
* device validation;
* warehouse validation where required;
* security audit;
* canonical response.

Remove legacy refresh-via-access-token behavior unless the approved PDA contract explicitly requires a compatibility period.

## API-003 — Logout

```http
POST /api/pda/v1/auth/logout
```

Implement:

* session revocation;
* refresh-token family revocation;
* idempotent repeated logout;
* audit;
* safe behavior when access token has expired;
* optional all-sessions revocation only through a separately authorized command.

## API-004 — Bootstrap

```http
GET /api/pda/v1/bootstrap
```

Return authoritative:

* operator;
* employee code;
* display name;
* roles;
* permissions;
* allowed warehouses;
* selected warehouse;
* shift;
* device registration status;
* feature flags;
* scanner policy;
* locale policy;
* server time;
* compatibility policy;
* minimum supported app version when implemented.

## API-006 — Current Profile

```http
GET /api/pda/v1/me
```

Return:

* operator identity;
* username;
* employee code;
* display name;
* roles;
* permissions;
* warehouse access;
* shift;
* active status;
* updated timestamp.

## Warehouses

```http
GET /api/pda/v1/me/warehouses
```

Return only authorized warehouses.

## Device Registration

```http
POST /api/pda/v1/devices/registrations
```

Implement durable registration, status, approval, duplicate behavior, revocation, operator/device relationship, and audit.

---

# 13. Authorization

Create a stable permission catalog covering at least:

```text
DASHBOARD_READ
TASK_READ
TASK_CLAIM
TASK_RELEASE
RECEIVING_READ
RECEIVING_EXECUTE
PUTAWAY_READ
PUTAWAY_EXECUTE
PICKING_READ
PICKING_EXECUTE
REPLENISHMENT_READ
REPLENISHMENT_EXECUTE
INVENTORY_READ
TRANSFER_EXECUTE
CYCLE_COUNT_READ
CYCLE_COUNT_EXECUTE
SHIPPING_READ
SHIPPING_VERIFY_PACKAGE
SHIPPING_CONFIRM
DEVICE_REGISTER
```

Reconcile exact codes with existing domain terminology.

For every API-001 through API-028:

* identify authentication requirement;
* identify required permission;
* identify warehouse scope;
* identify device scope;
* identify task assignment/lock requirement;
* enforce authorization in the application/use-case layer.

Gateway middleware is not sufficient as the only authorization layer.

Never trust:

* `operatorId` in the request body;
* roles sent by the PDA;
* permissions sent by the PDA;
* an unvalidated warehouse header;
* an unvalidated device header.

---

# 14. Failed Login and Abuse Protection

Implement:

* per-account failed-attempt tracking;
* lock or delay policy;
* IP/device-aware gateway rate limiting;
* safe generic authentication errors;
* security audit;
* reset after successful login according to policy;
* no account enumeration.

If distributed rate limiting uses Redis:

* Redis is only a coordination mechanism;
* permanent account/security state remains durable;
* Redis outage must fail according to an explicit security policy;
* do not silently disable brute-force protection in production.

---

# 15. Canonical Authentication Errors

Support stable machine codes:

```text
AUTH_INVALID_CREDENTIALS
AUTH_SESSION_EXPIRED
AUTH_TOKEN_INVALID
AUTH_TOKEN_REVOKED
AUTH_REFRESH_REUSED
AUTH_ACCOUNT_DISABLED
AUTH_ACCOUNT_LOCKED
DEVICE_NOT_REGISTERED
DEVICE_DISABLED
DEVICE_CONTEXT_MISMATCH
WAREHOUSE_ACCESS_DENIED
PERMISSION_DENIED
RATE_LIMITED
INTERNAL_ERROR
```

Use the common response envelope:

```json
{
  "data": null,
  "meta": {
    "serverTime": "2026-08-02T02:00:00Z",
    "correlationId": "uuid"
  },
  "errors": [
    {
      "code": "AUTH_INVALID_CREDENTIALS",
      "message": "Invalid credentials",
      "details": {},
      "retryable": false
    }
  ]
}
```

Do not leak:

* whether the username exists;
* password details;
* token parsing internals;
* cryptographic failures;
* raw database errors;
* stack traces.

---

# 16. Docker Compose

Add or update Docker Compose so the real authentication flow runs locally against a real PostgreSQL instance.

Required services:

```text
postgres
redis
pda-api-gateway
required backend services
integration-event-service when Kafka testing is enabled
kafka when the project owns a local test profile
```

PostgreSQL requirements:

* pinned version;
* named persistent volume;
* health check;
* non-default local credentials through environment variables;
* database initialization;
* migration execution;
* readiness dependency;
* no hardcoded production secret.

Example environment categories:

```text
PDA_DATABASE_URL
PDA_AUTH_MODE=internal
PDA_TOKEN_ISSUER
PDA_TOKEN_AUDIENCE
PDA_ACCESS_TOKEN_TTL
PDA_REFRESH_TOKEN_TTL
PDA_SIGNING_KEY_FILE
PDA_VALIDATION_KEY_FILES
PDA_PASSWORD_HASH_PARAMETERS
```

Do not preserve:

```text
PDA_AUTH_MODE=mock
```

as a valid normal runtime choice.

Docker Compose startup must fail clearly when:

* database is unavailable;
* migrations fail;
* signing keys are missing;
* token configuration is invalid;
* production uses default credentials;
* required identity data cannot be loaded.

---

# 17. Database Migration and Seed Execution

Provide stable commands such as:

```shell
make db-up
make migrate-up
make identity-seed-dev
make run
make test-auth-integration
make test-auth-e2e
make verify
```

The development seed must:

* create hashed-password operators;
* create roles and permissions;
* create warehouse access;
* create approved test devices;
* avoid plaintext database passwords beyond initial seed input;
* never run automatically in production.

Verify migration:

```text
empty database
→ apply all migrations
→ seed development identity
→ start services
→ run auth E2E
```

Also verify migration repeatability and rollback policy where supported.

---

# 18. Required Unit Tests

Add tests for:

* username normalization;
* password hashing;
* password verification;
* password rehash decision;
* token claims;
* token signing;
* token validation;
* algorithm allowlist;
* issuer;
* audience;
* expiry;
* not-before;
* key ID;
* key rotation;
* refresh-token hashing;
* refresh rotation;
* reuse detection;
* session revocation;
* operator disabled;
* device disabled;
* warehouse denied;
* permission denied;
* audit redaction;
* canonical error mapping.

---

# 19. Required PostgreSQL Integration Tests

Run against a real PostgreSQL container or Docker Compose database.

Test:

* operator persistence;
* role and permission loading;
* warehouse access;
* device registration;
* session creation;
* refresh-token persistence;
* refresh rotation transaction;
* concurrent refresh;
* refresh reuse;
* logout revocation;
* operator disablement;
* permission removal;
* warehouse removal;
* device revocation;
* service restart durability;
* audit persistence;
* row locking;
* migration from an empty database.

Do not replace these tests with in-memory repositories.

---

# 20. Required HTTP Authentication E2E

Run the exact sequence:

```text
1. Start PostgreSQL and Redis.
2. Apply migrations.
3. Seed development identity data.
4. Start the real PDA Backend composition.
5. Register or verify the test device.
6. Login using API-001.
7. Validate the canonical response.
8. Validate access-token signature and claims.
9. Call API-004 bootstrap.
10. Call API-006 /me.
11. Call /me/warehouses.
12. Call dashboard.
13. Call one authorized warehouse read.
14. Execute one authorized idempotent test mutation.
15. Verify audit, actor, device, warehouse, and correlation metadata.
16. Expire or invalidate the access token.
17. Refresh through API-002.
18. Verify rotation.
19. Retry the protected operation.
20. Logout through API-003.
21. Verify refresh is rejected.
22. Verify protected access follows the approved revocation policy.
23. Restart the gateway and identity composition.
24. Verify revoked session and used refresh token remain invalid.
25. Run refresh-token reuse attack test.
26. Verify the session family is revoked.
```

Record exact commands and results.

---

# 21. PDA App Contract Verification

Compare implementation and OpenAPI against the PDA App API specification.

Verify:

* login fields;
* login response fields;
* refresh fields;
* logout behavior;
* bootstrap;
* `/me`;
* warehouses;
* device registration;
* correlation;
* `X-Device-Id`;
* `X-Warehouse-Id`;
* `Accept-Language`;
* token expiry behavior;
* refresh-on-401 behavior;
* canonical error codes;
* scanner policy;
* feature flags;
* permission behavior;
* app-version policy.

Create a contract matrix:

| PDA requirement | Backend implementation | OpenAPI | Test evidence | Status |
| --------------- | ---------------------- | ------- | ------------- | ------ |

No authentication API may be marked supported unless all four columns are complete.

---

# 22. Resume Phase 11

After authentication is fully implemented and approved, continue the remaining Phase 11 work.

## 22.1 WMS

Verify:

* real non-mock WMS composition;
* approved API/event contract;
* endpoint authentication;
* timeout;
* retry;
* circuit breaker;
* checkpoint;
* replay;
* reconciliation;
* no direct WMS database access;
* PDA operator-to-WMS operator mapping when needed.

If real WMS is unavailable, keep the item blocked and do not claim full Phase 11 approval.

## 22.2 Kafka

Verify:

* TLS handshake;
* CA trust;
* mTLS when required;
* producer ACL;
* consumer ACL;
* consumer-group ACL;
* topic ACL;
* DLQ ACL;
* authorization denial;
* broker restart;
* rebalance;
* ordering by aggregate key;
* durable outbox recovery;
* durable DLQ;
* consumer lag export;
* publisher backlog export;
* no fallback to mock messaging.

Local PLAINTEXT verification must not be presented as secure Kafka approval.

## 22.3 Redis and PostgreSQL

Verify:

* restart behavior;
* Redis outage;
* PostgreSQL restart;
* migration;
* backup/restore evidence where required;
* auth session durability;
* command-status durability;
* cache invalidation;
* health/readiness behavior.

## 22.4 API-001 Through API-028

Run full contract tests for all PDA APIs.

Verify each endpoint is backed by:

```text
public route
→ middleware
→ handler
→ application use case
→ domain validation
→ PostgreSQL transaction where required
→ audit
→ outbox/event where required
→ response DTO
→ OpenAPI
→ automated test
```

## 22.5 Zebra

When a physical device is available, verify:

* DataWedge profile;
* hardware trigger;
* leading zeroes;
* symbology;
* scan context;
* duplicate-scan guard;
* scanner suspension while submitting;
* reconnect;
* token expiry during active workflow;
* session refresh;
* wrong-device rejection;
* no duplicate write after timeout or reconnect.

---

# 23. Full Mock and Deferred Audit of Every Phase

Before finalizing Phase 11, audit all previous backend and reconciliation phases.

At minimum inspect:

```text
PRE-00
BE-00
BE-01
BE-02
BE-03
BE-04
BE-05
BE-06
BE-07
BE-08
BE-09
BE-10
Reconciliation Phase 00
Reconciliation Phase 01
Reconciliation Phase 02
Reconciliation Phase 03
Reconciliation Phase 04
Reconciliation Phase 05
Reconciliation Phase 06
Reconciliation Phase 07
Reconciliation Phase 08
Reconciliation Phase 09
Reconciliation Phase 10
Phase 11
```

Search the repository for:

```text
mock
fixture
stub
fake
placeholder
TODO
FIXME
deferred
blocked
not implemented
not verified
hardcoded
in-memory
development only
PLAINTEXT
sample
demo
```

Do not classify solely by keyword.

Inspect actual composition and runtime reachability.

Create this matrix directly in the Phase 11 report:

| Phase | Capability | Runtime implementation | Mock/fixture present | Can activate outside tests | Real dependency verified | Status | Required action |
| ----- | ---------- | ---------------------- | -------------------- | -------------------------: | -----------------------: | ------ | --------------- |

Use statuses:

```text
REAL_AND_VERIFIED
REAL_BUT_NOT_EXTERNALLY_VERIFIED
TEST_FIXTURE_ONLY
RUNTIME_MOCK_PRESENT
MOCK_REMOVED
BLOCKED_BY_EXTERNAL_DEPENDENCY
INCOMPLETE
NOT_APPLICABLE
```

Required conclusions:

* authentication mock removed;
* WMS mock status;
* Kafka mock status;
* Redis status;
* PostgreSQL status;
* workflow fixture status;
* API implementation status;
* command-status status;
* cache invalidation status;
* OpenAPI status;
* observability status;
* physical-device status.

---

# 24. No-Mock Release Rule

Phase 11 must not be marked `APPROVED` while any production-required capability is still runtime-mock-only.

The following must not remain mock-only:

* authentication;
* session persistence;
* device registration;
* warehouse authorization;
* operator roles and permissions;
* database persistence;
* WMS integration when production approval requires it;
* Kafka integration when production approval requires it;
* audit;
* public API workflows required by the PDA App.

Test-only fakes may exist, but must be unreachable from runtime composition.

A capability blocked by unavailable external infrastructure must remain:

```text
BLOCKED_BY_EXTERNAL_DEPENDENCY
```

It must not be converted to approved by deleting the mock alone.

---

# 25. Required Report Updates

Update the existing Phase 11 report rather than creating an unrelated replacement with no history.

Recommended file:

```text
requirement-report/2026-08-02-phase-11-production-readiness.md
```

The updated report must include:

## 25.1 Authentication Before State

Record:

```text
AUTH_MOCK_ONLY
```

and summarize the removed behavior.

## 25.2 Authentication After State

Classify as:

```text
AUTH_PRODUCTION_IMPLEMENTED
```

or:

```text
AUTH_PRODUCTION_IMPLEMENTED_BUT_NOT_VERIFIED
```

based on actual evidence.

## 25.3 Removed Mock Components

List every deleted or disconnected authentication mock file and configuration path.

## 25.4 Database Evidence

Include:

* PostgreSQL image/version;
* Docker Compose service;
* migrations;
* tables;
* seed command;
* startup result;
* restart result;
* persistence result.

## 25.5 API Contract Evidence

Include API-001, API-002, API-003, API-004, API-006, warehouse lookup, and device registration.

## 25.6 Test Evidence

Include:

* unit tests;
* PostgreSQL integration tests;
* HTTP E2E;
* concurrent refresh;
* reuse detection;
* logout;
* restart durability;
* authorization;
* PDA contract tests;
* full verification commands.

## 25.7 All-Phase Mock Audit

Include the complete matrix required by Section 23.

## 25.8 Remaining External Blockers

Include only blockers that truly remain, such as:

* real WMS environment;
* Kafka ACL/TLS staging broker;
* physical Zebra device;
* production secret manager;
* production deployment environment.

## 25.9 Final Status

Use one:

```text
APPROVED
PARTIALLY_APPROVED
BLOCKED
```

Do not use `APPROVED` when:

* authentication remains mock;
* database tests use only an in-memory adapter;
* WMS remains mock when production WMS is an exit criterion;
* Kafka secure verification remains required but absent;
* a P0/P1 defect remains;
* full PDA integration contract tests fail.

---

# 26. Required Additional Reports

Create or update:

```text
requirement-report/YYYY-MM-DD-production-identity-implementation.md
requirement-report/YYYY-MM-DD-production-identity-test-evidence.md
requirement-report/YYYY-MM-DD-phase-11-all-phase-mock-audit.md
COMMON-DEFERRED-VERIFICATION.md
```

Update:

```text
PDA_BACKEND_RECONCILIATION_RULES_V2.md
README_PHASE_ORDER_V2.md
api/openapi/pda-v1.yaml
Docker Compose documentation
authentication runbook
key-rotation runbook
session-revocation runbook
database migration runbook
```

Rename mandatory OIDC wording to:

```text
production identity
```

unless OIDC has been explicitly selected.

---

# 27. Verification Commands

Run the repository-approved equivalents of:

```shell
docker compose down -v
docker compose up -d postgres redis
make migrate-up
make identity-seed-dev
docker compose up -d --build
make test
make test-auth-integration
make test-auth-e2e
make test-contract
make test-architecture
make test-integration
make test-kafka
make clean verify
go test ./...
go vet ./...
```

Also run:

* service restart tests;
* database restart tests;
* concurrent refresh test;
* refresh reuse test;
* logout persistence test;
* wrong-device test;
* wrong-warehouse test;
* permission-denied test;
* Phase 11 repository-wide mock audit.

Do not report a command as passing unless it was executed successfully.

Record skipped commands and exact reasons.

---

# 28. Exit Criteria

## Production Authentication Approved

Authentication is approved only when:

* no runtime mock identity adapter remains;
* no fixture password is used by runtime;
* passwords are hashed;
* PostgreSQL stores identity state;
* PostgreSQL stores sessions;
* PostgreSQL stores hashed refresh tokens;
* rotation works;
* reuse detection works;
* revocation survives restart;
* operator disablement works;
* device revocation works;
* warehouse authorization works;
* permission checks work;
* token validation works;
* key configuration is durable;
* Docker Compose uses the real database;
* migrations pass from an empty database;
* API contract tests pass;
* HTTP E2E passes;
* restart durability passes;
* no unresolved P0/P1 authentication defect remains.

## Full Phase 11 Approved

Phase 11 is approved only when:

* production authentication is approved;
* all API-001 through API-028 requirements are implemented and tested;
* real WMS integration is verified where required;
* secure Kafka ACL/TLS is verified where required;
* Redis and PostgreSQL production-like behavior is verified;
* the all-phase mock audit has no unresolved runtime mock for a production-required capability;
* required Zebra E2E is complete;
* no unresolved P0/P1 defect remains.

If external systems remain unavailable, report:

```text
PARTIALLY_APPROVED
```

only when all repository-owned work is complete and the remaining items are exclusively documented external verification blockers.

Otherwise report:

```text
BLOCKED
```

---

# 29. Final Execution Instruction

Proceed directly.

1. Inspect the current authentication implementation.
2. Capture the existing mock-only baseline.
3. Design the backend-owned production identity implementation.
4. Add PostgreSQL migrations.
5. Add or update Docker Compose.
6. Remove authentication mock runtime code and configuration.
7. Implement password security.
8. Implement access tokens.
9. Implement durable sessions and refresh tokens.
10. Implement rotation, reuse detection, logout, and revocation.
11. Implement roles, permissions, warehouses, devices, and audit.
12. Reconcile API-001, API-002, API-003, API-004, API-006, warehouse lookup, and device registration with the PDA API document.
13. Run real PostgreSQL integration tests.
14. Run full authentication HTTP E2E.
15. Run restart and security tests.
16. Resume the remaining Phase 11 checks.
17. Audit every prior phase for remaining mocks, fixtures, stubs, placeholders, and deferred production dependencies.
18. Add the complete all-phase audit directly to the updated Phase 11 report.
19. Run full repository verification.
20. Report exact implemented work, exact tests, remaining blockers, and the truthful final status.

Do not stop after creating interfaces, schemas, migrations, or reports.

Do not leave authentication as a scaffold.

Do not return only an implementation plan.

Implement the complete production authentication flow, execute the real database tests, complete all repository-owned Phase 11 work, audit every phase, and produce evidence-based reports.
