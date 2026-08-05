# PDA Backend Reconciliation Phase 02

Status: APPROVED
Date: 2026-08-02

## Repository Boundary Confirmation

- Existing PDA Android repository: external client; not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

## Implemented Scope

Phase 02 reconciles API-001 Login, API-002 Refresh, API-003 Logout, API-004 Bootstrap, API-006 Operator Profile, warehouse listing, and device registration.

- Login accepts the PDA identity fields `username`, `password`, `deviceId`, `deviceModel`, `appVersion`, `warehouseId`, and `locale`. Credentials remain redacted from logs and only `en-US`/`vi-VN` are accepted.
- Login returns access/refresh tokens, expiry, operator ID, employee code, display name, username, roles, permissions, warehouse selection, shift code, device-registration status, feature flags, scanner policy, and locale policy.
- Mock authentication now issues separate access and refresh credentials. Refresh requests using `{refreshToken,deviceId}` rotate the refresh credential and reject replay. The prior bearer refresh path remains temporarily supported for local compatibility.
- `/me` returns the approved profile fields rather than the internal operator aggregate.
- Bootstrap returns the server-authoritative operator, warehouse, device status, feature flags, scanner policy, locale policy, and server time. Protected writes still require a registered device and permitted warehouse through existing context validation.
- Device registration remains a separate explicit endpoint. Login does not silently register a device. This avoids treating a credential submission as device enrollment and keeps registration auditable.
- Operator identity, roles, and warehouse access remain token/server-derived. `X-Operator-Id` is only a checked context header; it is never authoritative.
- Production still rejects mock authentication. The current repository does not claim production OIDC verification; the OIDC adapter remains a later integration phase.

## OpenAPI and Verification

The OpenAPI document now includes the PDA login and refresh request shapes plus a reusable `Session` schema. Tests cover enriched session fields, refresh rotation, refresh replay rejection, profile output, bootstrap output, device registration, warehouse scope, and existing logout behavior.

Passed:

- `go test ./internal/identity/... ./internal/gateway/adapters/http ./test/contract`
- Phase 01 focused transport tests
- `make build`
- `make test-kafka`
- `git diff --check`

The repository-wide architecture scanner remains blocked by Android terms in the supplied authoritative PDA documents; those documents were not modified or removed.

## Deferred Decisions

OIDC provider configuration, production refresh-token storage, app-version compatibility policy, feature-flag ownership, scanner policy values, and device blocking/read-only behavior require deployment and security decisions. The local mock contract is explicit and does not represent production OIDC readiness.
