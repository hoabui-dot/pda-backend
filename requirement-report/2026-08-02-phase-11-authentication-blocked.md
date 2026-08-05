# Phase 11 Authentication Verification Report

Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

Date: 2026-08-02
Status: BLOCKED
Authentication classification: `AUTH_MOCK_ONLY`

## Decision

The Phase 11 prompt requires the authentication state to be classified before continuing. The current PDA backend authentication is a local mock implementation and does not meet the stop-condition requirements for production authentication. Phase 11 therefore stops at authentication verification. WMS, Kafka production approval, and full external E2E approval must not be claimed from this repository state.

## Evidence inspected

- `internal/gateway/adapters/http/router.go`
- `internal/identity/application/service.go`
- `internal/identity/adapters/mock/token.go`
- `internal/identity/adapters/mock/store.go`
- `internal/identity/adapters/mock/testdata/identity.json`
- `internal/identity/domain/model.go`
- `internal/identity/ports/identity.go`
- `internal/platform/config/config.go`
- `api/openapi/pda-v1.yaml`
- `docs/integration-pda-app/PDA_BACKEND_RECONCILIATION_RULES_V2.md`
- `docs/integration-pda-app/PDA_APP_API_SPECIFICATION.md`
- `docs/integration-pda-app/PHASE_11_EXTERNAL_OIDC_WMS_KAFKA_TLS_ZEBRA_E2E.md`
- `docs/integration-pda-app/Phase-11-Prompt-—-Verify-and-Complete-the-Existing-PDA-Backend-Authentication-and-Production-Integration-Flow.md`
- existing gateway, identity, configuration, and contract tests

## Current authentication flow

### Login: `POST /api/pda/v1/auth/login`

The route accepts the PDA login fields `username`, `password`, `deviceId`, `deviceModel`, `appVersion`, `warehouseId`, and `locale`. It validates only basic presence and locale shape before calling `identity.Service.LoginSession`.

`Login` currently:

1. Looks up a user in the embedded fixture store.
2. Compares the supplied password directly with `operator.Password`.
3. Issues a locally signed mock access token.
4. Issues a locally signed mock refresh token.
5. Appends an in-memory audit record.

The login path does not validate the device or warehouse before issuing tokens. Device and warehouse context is checked later for protected routes, and the login response can therefore report the requested context without proving registration during authentication.

### Access tokens

`internal/identity/adapters/mock/token.go` creates JWT-shaped tokens with:

- HMAC-SHA256 (`HS256`);
- a process-configured local secret;
- issuer `pda-local-mock`;
- a `mock:true` header field;
- operator subject and token ID;
- access/refresh kind;
- expiry.

The token provider validates the local HMAC signature, issuer, token kind, expiry, and in-memory revocation map. It is explicitly a mock provider and has no external issuer, audience, JWKS, asymmetric signing key, key rotation, or durable revocation state.

### Refresh

`POST /api/pda/v1/auth/refresh` supports a refresh-token body and a legacy access-token path. Refresh tokens are issued and rotated by the mock provider, but revoked token IDs exist only in process memory. There is no persisted session, refresh-token hash, rotation chain, reuse-detection record, device binding, or restart durability.

### Logout

`POST /api/pda/v1/auth/logout` revokes the supplied token in the in-memory mock provider. Logout is not backed by a durable session record and cannot provide persistent compromised-session revocation after restart.

### Identity and persistence

The current identity store is `internal/identity/adapters/mock.Store`:

- users and warehouses come from embedded `identity.json`;
- passwords are stored as fixture strings and compared directly;
- devices are held in an in-memory map;
- audit records are held in an in-memory slice;
- no operator/session/refresh/device authorization persistence is used by the gateway composition.

### Production configuration

`internal/platform/config.Config` allows `mock` and `oidc` authentication mode values and rejects mock modes when `PDA_ENVIRONMENT=production`. However, `cmd/pda-api-gateway/main.go` explicitly exits when the selected authentication mode is not `mock`, because no OIDC or other production identity adapter is wired. This is a correct fail-closed guard, but it is not production authentication.

## Current routes and status

| Route | API | Current state |
|---|---|---|
| `POST /api/pda/v1/auth/login` | API-001 | Mock fixture credentials and mock tokens |
| `POST /api/pda/v1/auth/refresh` | API-002 | In-memory mock rotation; no durable session |
| `POST /api/pda/v1/auth/logout` | API-003 | In-memory token revocation |
| `GET /api/pda/v1/bootstrap` | API-004 | Mock operator, warehouse, device, and policy data |
| `GET /api/pda/v1/me` | API-006 | Rehydrates operator from mock store |
| `GET /api/pda/v1/me/warehouses` | identity support | Mock warehouse repository |
| `POST /api/pda/v1/devices/registrations` | device support | In-memory registration and audit |

## Missing production requirements

### Credential verification

- approved production credential authority is not selected;
- password hashing and verification are not implemented;
- no secure operator provisioning or password-reset path exists;
- fixture passwords are not acceptable for production.

### Token security

- no production issuer or audience contract;
- no asymmetric signing or JWKS validation;
- no signing-key source or rotation strategy;
- no durable token/session revocation;
- no access-token/session relationship;
- no durable refresh-token hash or rotation chain;
- no refresh-token reuse detection;
- no durable device binding.

### Authorization

- roles and permissions originate from mock fixture data;
- no production role/permission authority is connected;
- device registration is process-local;
- warehouse access is fixture-backed;
- account disablement and permission removal are not durable runtime checks;
- application-level permission enforcement is not demonstrated for every API-001 through API-028 use case.

### Abuse protection and audit

- gateway rate limiting is process-local and not a distributed brute-force control;
- no account-level failed-login threshold or lockout policy is implemented;
- audit records are process-local;
- no durable security event sink is configured;
- production secret and key-management policy is not wired.

## Required work before Phase 11 may continue

The owner must first select and approve one production identity architecture. OIDC is optional; it must not be introduced solely because an earlier document mentioned it. The selected architecture must provide:

1. Credential verification or a documented trust boundary to an enterprise identity service.
2. Signed access-token validation with issuer, audience, expiry, algorithm allowlist, key rotation, and clock-skew policy.
3. Durable sessions and refresh tokens, including hashing, expiry, rotation, reuse detection, revocation, device policy, and restart durability.
4. Durable operator, role, permission, warehouse, device, and account-status data or a documented authoritative directory integration.
5. Secure login, refresh, logout, compromised-session revocation, and account-disable behavior.
6. Distributed rate limiting and brute-force protection.
7. Durable audit records with correlation ID, operator, device, warehouse, outcome, and timestamp, without secrets.
8. Production configuration validation and secret/key loading through the deployment secret manager.
9. Authentication integration tests covering login, token validation, refresh concurrency/reuse, logout, key rotation, disabled users, device/warehouse scope, and service restart.
10. A production-like HTTP E2E flow through API-001, API-002, API-003, API-004, API-006, warehouse lookup, and device registration.

## Required decisions from the integration owner

Provide these decisions before implementation resumes:

| Decision | Required value |
|---|---|
| Identity architecture | Dedicated backend identity, enterprise service, OIDC, OAuth2, or approved alternative |
| Credential authority | Backend-owned password verification or external authority |
| Token issuer | Issuer URL/name and trust model |
| Audience | PDA backend audience |
| Signing | Algorithm, key source, key ID, rotation and overlap policy |
| Access token | TTL, claims, clock skew, introspection/revocation policy |
| Refresh token | TTL, rotation, reuse detection, device binding, persistence model |
| Session policy | Concurrent sessions, logout, forced revocation, restart behavior |
| Operator authority | Directory/WMS/backend ownership of operator status and attributes |
| Warehouse authority | Claim, directory, backend, or WMS ownership |
| Device policy | Registration, approval, revocation, operator binding, warehouse binding |
| Permissions | Role and permission source plus API capability matrix |
| Audit | Durable store, retention, alerting, and access controls |

## Recommended next implementation phase

Create a dedicated **Production Identity Implementation** phase before resuming the rest of Phase 11. Implement the selected architecture behind the existing identity ports, preserve the PDA API routes and envelope, and keep the mock adapter available only for explicit local tests. Do not modify the external PDA Android repository.

After production identity tests pass, resume Phase 11 WMS and Kafka verification. WMS must remain an operational dependency and must not automatically become the PDA identity provider. The WMS/OIDC setup requirements are also consolidated in `docs/integration-pda-app/PDA_WMS_OIDC_INTEGRATION_REQUIREMENTS.md`.

## Verification result

- Authentication classification: `AUTH_MOCK_ONLY`.
- Production authentication: not implemented.
- Production authentication: not externally verified.
- Phase 11 continuation to WMS, Kafka, and production E2E: stopped by mandatory prompt condition.
- Final status: `BLOCKED`.
