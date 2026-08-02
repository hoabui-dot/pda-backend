# Phase 02 Prompt — Authentication, Device Registration, Bootstrap, Session, and Profile

## API Scope

- API-001 Login
- API-002 Refresh
- API-003 Logout
- API-004 Bootstrap
- API-006 Operator profile
- Existing device-registration and warehouse-list endpoints

## Objective

Align identity/session/device/warehouse contracts with the PDA fields and define the transition from mock auth to production OIDC without blocking local integration.

## Required Login Request Review

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

## Required Session/Profile Fields

Evaluate and return where approved:

- access token;
- refresh token;
- token type;
- expiry;
- operator ID;
- employee code;
- display name;
- username;
- roles;
- permissions;
- warehouse ID/name;
- allowed warehouses;
- shift code;
- device registration status;
- feature flags;
- scanner policy;
- locale policy;
- active state;
- server time.

## Tasks

1. Compare current mock login request/response with API-001.
2. Decide whether device registration occurs before login, during login, or through the existing registration endpoint.
3. Align refresh rotation and single-flight semantics.
4. Align logout response: canonical envelope versus 204; document PDA behavior for 401 logout.
5. Align `/bootstrap`, `/me`, `/me/warehouses`, and device registration without duplicate payloads.
6. Enforce token-derived roles/operator/warehouse authority.
7. Add app-version compatibility and locale validation if approved.
8. Add mock-mode behavior for local development and production guards rejecting mock auth.
9. Prepare OIDC adapter contract but do not claim production OIDC verification without a real provider.
10. Update OpenAPI and tests.

## Business Decisions

- first-use unregistered-device behavior;
- warehouse selection timing;
- refresh-token storage/rotation;
- feature flags and scanner policy ownership;
- device blocking versus read-only mode.

## Exit Criteria

- PDA can implement login, refresh, bootstrap, profile, and logout without guessing fields.
- Device and warehouse authority are server validated.
- Local mock and future OIDC contracts are explicitly separated.
- Phase report states `APPROVED`.
