# Phase 11 Prompt — Verify and Complete the Existing PDA Backend Authentication and Production Integration Flow

## 1. Role

You are working inside the existing:

```text
PDA_BACKEND
```

Act as a:

* senior Go backend engineer;
* enterprise authentication and authorization architect;
* API security engineer;
* PDA mobile-backend integration specialist;
* WMS integration architect;
* Kafka and production-readiness engineer;
* test and release engineer.

This phase must verify and complete the production authentication already owned by `PDA_BACKEND`.

Do not assume that OIDC is required.

The PDA Backend authentication system is independent from the upstream WMS unless the approved architecture explicitly states otherwise.

---

# 2. Repository Boundary

There are two separate systems:

```text
PDA_APP
Existing Kotlin Android client
External to this backend phase

PDA_BACKEND
Current working repository
Owns:
- PDA authentication;
- token lifecycle;
- operator identity;
- device context;
- warehouse authorization;
- roles and permissions;
- public PDA APIs;
- workflow orchestration;
- WMS integration;
- Kafka integration.
```

The upstream WMS is an operational dependency for warehouse data and workflow synchronization.

The WMS must not automatically be treated as the identity provider for the PDA Backend.

Do not:

* create or modify Android source code;
* run Android Gradle tasks;
* recreate the PDA App;
* require OIDC only because it appeared in an earlier phase document;
* replace an existing production-grade authentication system without evidence that replacement is required;
* claim authentication is production-ready based only on mock tokens or unit tests.

---

# 3. Mandatory Input Documents

Before implementation, read:

```text
PDA_BACKEND_RECONCILIATION_RULES_V2.md
PDA_APP_API_INTEGRATION_REQUIREMENTS.md
PDA_BACKEND_PDA_APP_AUTHORITATIVE_TRACEABILITY.md
api/openapi/pda-v1.yaml
```

Also inspect:

* authentication architecture documents;
* identity service packages;
* gateway authentication middleware;
* authorization policies;
* device registration;
* warehouse-scope handling;
* token services;
* session persistence;
* refresh-token persistence;
* audit implementation;
* configuration;
* migrations;
* tests;
* current Phase 11 report;
* deferred-verification register.

The PDA API document is the client contract for authentication, bootstrap, profile, device context, headers, errors, and session behavior.

---

# 4. Primary Decision

The first task is to determine the real authentication state of the PDA Backend.

Classify it as exactly one of:

```text
AUTH_NOT_IMPLEMENTED
AUTH_MOCK_ONLY
AUTH_PARTIALLY_IMPLEMENTED
AUTH_PRODUCTION_IMPLEMENTED
AUTH_PRODUCTION_IMPLEMENTED_BUT_NOT_VERIFIED
```

Do not proceed with the full Phase 11 implementation before this classification is supported by repository evidence.

---

# 5. Mandatory Authentication Inspection

Trace the complete authentication flow:

```text
PDA login request
→ gateway route
→ request validation
→ credential verification
→ user/operator lookup
→ device validation
→ warehouse authorization
→ role/permission loading
→ access-token creation
→ refresh-token creation
→ session persistence
→ response mapping
→ authenticated PDA request
→ middleware token validation
→ actor context creation
→ application authorization
→ audit
→ refresh
→ logout/revocation
```

At minimum inspect:

1. `POST /api/pda/v1/auth/login`;
2. `POST /api/pda/v1/auth/refresh`;
3. `POST /api/pda/v1/auth/logout`;
4. `GET /api/pda/v1/bootstrap`;
5. `GET /api/pda/v1/me`;
6. `GET /api/pda/v1/me/warehouses`;
7. `POST /api/pda/v1/devices/registrations`;
8. access-token generation;
9. access-token validation;
10. refresh-token generation;
11. refresh-token rotation;
12. refresh-token revocation;
13. password or credential verification;
14. user/operator persistence;
15. session persistence;
16. device registration persistence;
17. warehouse access persistence;
18. roles and permissions;
19. token signing-key management;
20. key rotation strategy;
21. token expiry;
22. logout;
23. compromised-session revocation;
24. rate limiting;
25. brute-force protection;
26. audit logging;
27. secret handling;
28. production configuration;
29. mock-mode rejection;
30. API and OpenAPI tests.

---

# 6. Stop Conditions

Immediately stop the full Phase 11 implementation if any of the following is true:

* authentication does not exist;
* only hardcoded users exist;
* login always returns deterministic mock tokens;
* passwords are not securely verified;
* refresh tokens are not persisted;
* refresh tokens cannot be revoked;
* access-token signatures are not verified;
* production mode still uses mock authentication;
* authorization trusts role, operator, warehouse, or device values directly from the PDA request;
* no approved production identity architecture exists;
* authentication behavior cannot be determined safely from the repository.

When stopped, do not fabricate a production authentication implementation.

Create:

```text
requirement-report/YYYY-MM-DD-phase-11-authentication-blocked.md
```

The blocked report must include:

* authentication classification;
* files inspected;
* current routes;
* current login behavior;
* current token behavior;
* mock behavior found;
* persistence found or missing;
* password verification found or missing;
* refresh/revocation found or missing;
* authorization gaps;
* device and warehouse gaps;
* production risks;
* exact work required before Phase 11 may continue;
* recommended authentication implementation phase;
* final status: `BLOCKED`.

Do not continue to WMS, Kafka, or production E2E approval when authentication is missing or mock-only.

---

# 7. Authentication Architecture Rule

If production authentication already exists, preserve and complete it.

The approved production identity implementation may be:

* a dedicated identity service inside PDA Backend;
* an internal authentication module owned by PDA Backend;
* a shared enterprise authentication service;
* OIDC;
* OAuth2 with an approved identity provider;
* another approved token-based identity architecture.

OIDC is optional unless explicitly selected by the approved architecture.

Do not replace the existing authentication implementation with OIDC merely to satisfy an outdated prompt.

Update Phase 11 terminology from:

```text
OIDC verification
```

to:

```text
Production identity verification
```

The exit criterion must be:

```text
The approved production identity implementation is complete and externally verified.
```

It must not require:

```text
OIDC is externally verified.
```

unless OIDC is the selected architecture.

---

# 8. PDA Authentication Contract

Reconcile the backend with the actual PDA App API contract.

## API-001 — Login

Expected route:

```http
POST /api/pda/v1/auth/login
```

Expected request contract:

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

For every field determine:

* whether it is required;
* whether it is optional;
* whether it belongs in the body or headers;
* validation rules;
* maximum length;
* source of authority;
* logging/redaction behavior;
* backward compatibility.

The server must not trust client-supplied operator roles or permissions.

Expected response capabilities:

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

Do not blindly copy this response.

Compare it with the current backend DTO and produce one canonical compatible contract.

---

## API-002 — Refresh Session

Expected route:

```http
POST /api/pda/v1/auth/refresh
```

Verify:

* refresh-token transport;
* refresh-token persistence;
* refresh-token hashing;
* token rotation;
* reuse detection;
* device binding;
* expiry;
* revocation;
* one active refresh operation per session where required;
* safe retry behavior;
* session invalidation after refresh-token reuse;
* correlation and audit.

The PDA client expects a single-flight refresh flow.

Concurrent refresh requests must not create inconsistent token chains.

---

## API-003 — Logout and Revocation

Expected route:

```http
POST /api/pda/v1/auth/logout
```

Verify:

* current access token invalidation policy;
* refresh-token revocation;
* session revocation;
* device session cleanup;
* logout audit;
* idempotent logout;
* behavior when the token is already expired;
* behavior when the backend is temporarily unavailable.

The PDA App may clear its local secure session even when server revocation fails, but the backend must still define safe revocation behavior.

---

## API-004 — Bootstrap

Expected route:

```http
GET /api/pda/v1/bootstrap
```

Expected capabilities:

* operator identity;
* employee code;
* display name;
* roles;
* permissions;
* allowed warehouses;
* current warehouse;
* shift;
* device registration status;
* feature flags;
* scanner policy;
* locale policy;
* server time;
* compatibility information;
* minimum supported app version where applicable.

The bootstrap response must be authoritative before warehouse mutations are enabled.

---

## API-006 — Current Profile

Expected route:

```http
GET /api/pda/v1/me
```

Expected capabilities:

* operator identity;
* username;
* employee code;
* display name;
* roles;
* permissions;
* warehouse access;
* shift;
* active state;
* last-updated timestamp.

---

## Warehouses

Expected route:

```http
GET /api/pda/v1/me/warehouses
```

Verify:

* only authorized warehouses are returned;
* selected warehouse is validated;
* warehouse changes affect actor context safely;
* warehouse scope cannot be overridden using request-body data.

---

## Device Registration

Expected route:

```http
POST /api/pda/v1/devices/registrations
```

Verify:

* device identity;
* device model;
* app version;
* registration state;
* activation;
* revocation;
* operator-device relationship;
* warehouse relationship;
* security policy;
* audit;
* duplicate registration behavior;
* device-disabled behavior.

---

# 9. Access Token Requirements

The existing authentication system must provide a documented access-token contract.

Verify:

* signing algorithm;
* signing key source;
* public/private key or shared-secret policy;
* issuer;
* audience;
* subject;
* token ID;
* issued-at time;
* expiry;
* not-before;
* operator ID;
* session ID;
* device ID or device-session reference;
* warehouse scope;
* roles;
* permissions;
* key ID;
* signature validation;
* clock-skew policy.

Do not include unrestricted or frequently changing data in the access token when authoritative lookup is required.

Do not log access tokens.

Production secrets must not be committed.

---

# 10. Refresh Token Requirements

Refresh tokens must:

* be cryptographically random;
* be stored hashed where appropriate;
* have an explicit expiry;
* be bound to a session;
* be bound to device policy where required;
* support rotation;
* support revocation;
* support reuse detection;
* survive authorized service restart;
* not be stored in logs;
* not be returned after revocation;
* not be accepted across unrelated operators or devices.

Document the selected model.

---

# 11. Password and Credential Requirements

If the PDA Backend owns usernames and passwords:

* use an approved password hashing algorithm;
* use per-password salt;
* configure cost appropriately;
* never store plaintext passwords;
* never log credentials;
* implement safe comparison;
* apply rate limits;
* add brute-force protection;
* avoid revealing whether a username exists;
* provide secure administrator provisioning or password reset policy.

If another internal identity system verifies credentials, document the trust and transport boundary.

---

# 12. Authorization Requirements

Authentication success is not sufficient.

Verify authorization at the application/use-case layer for every protected operation.

The backend must derive authoritative context from validated authentication:

```text
operator
session
roles
permissions
device
warehouse
correlation
```

Do not authorize warehouse mutations solely in gateway middleware.

For each PDA workflow verify permission mapping:

* dashboard;
* task access;
* task claim/release;
* receiving;
* putaway;
* picking;
* replenishment;
* inventory inquiry;
* stock transfer;
* cycle count;
* shipping.

Create a permission matrix:

| PDA capability | Required permission | Warehouse scoped | Device scoped | Current implementation | Status |
| -------------- | ------------------- | ---------------: | ------------: | ---------------------- | ------ |

---

# 13. Header and Actor Context Requirements

Verify the following headers:

```http
Authorization: Bearer <access-token>
X-Correlation-Id: <uuid>
X-Device-Id: <device-id>
X-Warehouse-Id: <warehouse-id>
Accept-Language: vi-VN
```

Rules:

* token identity is authoritative;
* `X-Device-Id` must match the authenticated or registered device policy;
* `X-Warehouse-Id` must be inside the operator's allowed warehouse scope;
* client-supplied operator ID is non-authoritative;
* correlation ID must propagate into logs, audit, outbox events, and Kafka metadata;
* language must not change stable machine error codes.

---

# 14. Required Error Contract

Ensure authentication endpoints return the canonical PDA envelope.

Required stable codes include:

```text
AUTH_INVALID_CREDENTIALS
AUTH_SESSION_EXPIRED
AUTH_TOKEN_INVALID
AUTH_TOKEN_REVOKED
AUTH_REFRESH_REUSED
AUTH_ACCOUNT_DISABLED
DEVICE_NOT_REGISTERED
DEVICE_DISABLED
DEVICE_CONTEXT_MISMATCH
WAREHOUSE_ACCESS_DENIED
PERMISSION_DENIED
RATE_LIMITED
INTERNAL_ERROR
```

Example:

```json
{
  "data": null,
  "meta": {
    "serverTime": "2026-08-02T02:00:00Z",
    "correlationId": "uuid"
  },
  "errors": [
    {
      "code": "AUTH_SESSION_EXPIRED",
      "message": "Session expired",
      "details": {},
      "retryable": false
    }
  ]
}
```

Do not leak:

* raw token errors;
* cryptographic details;
* database errors;
* usernames;
* account existence;
* stack traces.

---

# 15. Required Authentication Tests

Authentication may not be approved based only on unit tests.

Implement and run all applicable tests below.

## 15.1 Login Tests

* valid login;
* invalid password;
* unknown username;
* disabled operator;
* missing device;
* unregistered device;
* disabled device;
* unauthorized warehouse;
* missing required fields;
* invalid locale;
* unsupported app version;
* rate limiting;
* brute-force threshold;
* safe error response;
* password and token redaction.

## 15.2 Access Token Tests

* valid signature;
* invalid signature;
* expired token;
* not-before token;
* wrong issuer when applicable;
* wrong audience when applicable;
* unknown signing key;
* revoked session;
* disabled operator after token issue;
* wrong device;
* unauthorized warehouse;
* missing permission;
* key rotation;
* clock skew.

## 15.3 Refresh Tests

* valid refresh;
* expired refresh token;
* revoked refresh token;
* token rotation;
* reuse of an old refresh token;
* concurrent refresh requests;
* wrong device;
* wrong session;
* disabled operator;
* service restart;
* persistence durability;
* correlation and audit.

## 15.4 Logout Tests

* normal logout;
* repeated logout;
* logout with expired access token;
* revoked refresh token;
* session invalid after logout;
* access behavior after logout;
* audit record.

## 15.5 Bootstrap and Profile Tests

* valid bootstrap;
* allowed warehouses;
* current warehouse;
* role and permission mapping;
* device policy;
* scanner policy;
* feature flags;
* server time;
* locale;
* account disabled after login;
* warehouse access removed after login.

## 15.6 Authorization Tests

For each protected PDA workflow:

* allowed operator;
* denied operator;
* wrong warehouse;
* wrong device;
* missing permission;
* task assigned to another operator;
* cross-warehouse entity ID;
* body operator mismatch;
* body warehouse mismatch.

## 15.7 Security Tests

* passwords never logged;
* tokens never logged;
* refresh tokens never stored in plaintext where hashing is required;
* secure cookie/header behavior if used;
* secret configuration validation;
* production startup rejects default secrets;
* production startup rejects mock auth;
* request-size limit;
* malformed token resilience;
* panic and raw dependency error protection.

---

# 16. PDA App Integration Test Flow

Run the full client-facing authentication sequence using the API contract.

```text
1. Register or resolve device.
2. Login.
3. Validate login response.
4. Call bootstrap.
5. Call /me.
6. Call /me/warehouses.
7. Call dashboard with the access token.
8. Execute one permitted warehouse read.
9. Execute one permitted idempotent mutation in a test fixture.
10. Expire or invalidate the access token.
11. Refresh the session.
12. Retry the protected call.
13. Verify actor, device, warehouse, and correlation context.
14. Logout.
15. Verify the session can no longer refresh.
16. Verify protected calls fail according to the approved revocation model.
```

Use a production-like backend composition, not a mock token provider.

If a real PDA build is available, run the flow through the actual PDA client.

If the PDA build is not available, run contract-compatible HTTP E2E tests using the exact PDA request and response shapes.

---

# 17. Authentication Runtime Evidence

Record:

* authentication mode;
* identity service used;
* persistence used;
* signing algorithm;
* token expiry;
* refresh expiry;
* key source;
* key rotation behavior;
* password hashing policy;
* device policy;
* warehouse policy;
* permission policy;
* test users;
* test devices;
* test warehouses;
* exact commands executed;
* test results;
* service restart result;
* revocation result;
* logs and metrics without secrets.

---

# 18. Continue the Remaining Phase 11 Work

Only after production authentication is classified as implemented and the full authentication test suite passes may Phase 11 continue.

Then complete:

## 18.1 WMS Integration

* use the approved WMS API/event contract;
* enable non-mock WMS composition;
* keep WMS DTOs inside the adapter boundary;
* validate endpoint and credentials;
* implement timeout, retry, circuit breaker, checkpoint, replay, and reconciliation;
* verify no direct WMS database access;
* verify WMS operator mapping is independent from PDA authentication unless explicitly approved.

## 18.2 Secure Kafka

* configure TLS 1.2 or later;
* load CA trust correctly;
* configure mTLS when required;
* configure least-privilege ACLs;
* verify producer permissions;
* verify consumer-group permissions;
* verify DLQ permissions;
* verify ACL denial;
* verify broker restart;
* verify rebalance;
* verify ordering per aggregate key;
* verify durable outbox recovery;
* verify durable DLQ;
* verify backlog and lag export;
* verify no silent fallback to mock mode.

## 18.3 Redis and PostgreSQL

* verify production-like connection security;
* verify restart behavior;
* verify Redis outage;
* verify database migration;
* verify session durability;
* verify command-status durability;
* verify cache invalidation.

## 18.4 PDA API-001 Through API-028

Run the complete API contract suite covering:

* authentication;
* bootstrap;
* dashboard;
* tasks;
* receiving;
* putaway;
* picking;
* replenishment;
* inventory;
* transfer;
* cycle count;
* shipping;
* command status.

## 18.5 Physical Zebra Verification

When a device is available, verify:

* DataWedge profile;
* hardware trigger;
* leading zeroes;
* symbology;
* scan context;
* duplicate-scan protection;
* scanner suspension during submission;
* reconnect;
* token refresh during active workflow;
* expired session behavior;
* wrong-device rejection;
* no duplicate writes after timeout or reconnect.

---

# 19. Required Files to Update

Update:

```text
PHASE_11_EXTERNAL_OIDC_WMS_KAFKA_TLS_ZEBRA_E2E.md
```

Rename it when repository conventions allow:

```text
PHASE_11_PRODUCTION_IDENTITY_WMS_KAFKA_TLS_ZEBRA_E2E.md
```

Update all Phase 11 references from mandatory OIDC wording to approved production identity wording.

Also update:

```text
PDA_BACKEND_RECONCILIATION_RULES_V2.md
README_PHASE_ORDER_V2.md
COMMON-DEFERRED-VERIFICATION.md
api/openapi/pda-v1.yaml
```

Update source code, migrations, tests, configuration, and runbooks only when required by the verified existing authentication architecture.

---

# 20. Required Reports

Create:

```text
requirement-report/YYYY-MM-DD-phase-11-production-identity-verification.md
```

If successful, also create or update:

```text
requirement-report/YYYY-MM-DD-phase-11-production-readiness.md
```

The identity report must include:

* authentication classification;
* authentication architecture;
* whether OIDC is used;
* why OIDC is or is not required;
* login contract;
* token contract;
* refresh contract;
* revocation contract;
* device contract;
* warehouse contract;
* permission matrix;
* request changes;
* response changes;
* OpenAPI changes;
* migrations;
* tests;
* full E2E result;
* unresolved security risks;
* external blockers;
* final authentication status.

---

# 21. Exit Criteria

Phase 11 authentication is `APPROVED` only when:

* authentication is not mock;
* production startup rejects mock authentication;
* login works;
* access-token validation works;
* refresh works;
* rotation and reuse behavior are tested;
* logout and revocation work;
* device policy works;
* warehouse authorization works;
* roles and permissions work;
* PDA request and response contracts match;
* OpenAPI matches implementation;
* secrets are managed safely;
* full authentication E2E passes;
* service restart does not lose required authentication state;
* no unresolved P0/P1 authentication defect exists.

Full Phase 11 is `APPROVED` only when:

* production identity is approved;
* WMS integration is externally verified;
* secure Kafka TLS/ACL is externally verified;
* PDA API-001 through API-028 pass;
* required failure injection passes;
* physical Zebra verification passes when required;
* no unresolved P0/P1 production-readiness defect exists.

If authentication is mock-only or missing, the correct status is:

```text
BLOCKED
```

Do not continue and do not claim partial production approval.

---

# 22. Final Execution Instruction

Proceed in this order:

1. Inspect and classify the existing PDA Backend authentication.
2. If missing or mock-only, stop and produce the blocked report.
3. If production authentication exists, preserve and complete it.
4. Reconcile it with the actual PDA App API contract.
5. Implement missing authentication behavior.
6. Update OpenAPI.
7. Run the complete authentication test suite.
8. Run the full PDA authentication E2E flow.
9. Update Phase 11 to use the approved production identity architecture instead of mandatory OIDC.
10. Continue WMS, Kafka, staging, and Zebra verification only after authentication is approved.
11. Produce final reports with exact evidence.

Do not return only an analysis.

Implement, test, document, and report the actual repository state.
