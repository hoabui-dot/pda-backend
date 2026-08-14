# PDA App Authentication Session Integration Prompt

Implement the PDA mobile authentication lifecycle against the PDA Backend API. Preserve the existing API envelope and use the base path `/api/pda/v1`.

## Login

Call `POST /auth/login` with the existing login fields, including `username`, `password`, `deviceId`, and `warehouseId` when available. On success, persist both `accessToken` and `refreshToken` together with `refreshTokenExpiresAt`, `expiresAt`, device ID, and warehouse ID.

Store tokens only in Android Keystore-backed secure storage, such as `EncryptedSharedPreferences` or the project equivalent. Never write passwords, access tokens, or refresh tokens to logs, analytics, crash reports, screenshots, or ordinary preferences.

The backend returns these relevant session fields:

- `accessToken`
- `refreshToken`
- `tokenType` (`Bearer`)
- `expiresAt` and `accessTokenExpiresIn`
- `refreshTokenExpiresAt`

## Normal API Requests

Send `Authorization: Bearer <accessToken>` on protected requests. When a response is `401` with error code `ACCESS_TOKEN_EXPIRED`, refresh the session and retry the original request exactly once with the new access token.

Do not refresh merely because the Android process stopped, the app was closed, the device rebooted, the battery died, or the app was upgraded. These lifecycle events do not invalidate the backend session.

## Refresh

Call `POST /auth/refresh` with:

```json
{
  "refreshToken": "<stored refresh token>",
  "deviceId": "<stored device id>"
}
```

Refresh token rotation is enabled. A successful response replaces **both** stored tokens atomically. Never keep using the old refresh token after a successful response.

Implement a single-flight refresh coordinator. If several requests receive `ACCESS_TOKEN_EXPIRED` at the same time, only one refresh request may run. Other requests must wait for that result, then retry with the returned access token. If the refresh fails, each waiting request must receive the same authentication result and must not start an uncontrolled refresh loop.

If the app is restarted or the process is killed, load the stored refresh token and call `/auth/refresh` when an authenticated API session is needed. Do not require the user to log in again while the refresh token remains valid.

## Required Error Handling

Handle these backend codes explicitly:

- `ACCESS_TOKEN_EXPIRED`: perform one coordinated refresh and retry once.
- `ACCESS_TOKEN_INVALID`: clear the local session and show login.
- `REFRESH_TOKEN_EXPIRED`: clear the local session and require login.
- `REFRESH_TOKEN_REVOKED`: clear the local session and require login.
- `REFRESH_TOKEN_REUSED`: stop retries, clear the local session, and require login.
- `REFRESH_TOKEN_INVALID`: clear the local session and require login.
- `SESSION_REVOKED`: clear the local session and require login.
- `USER_DISABLED`: stop retries and show the account-disabled state.

Treat an unexpected network or server failure differently from an invalid session. Keep the secure tokens for a later retry when the refresh request did not receive a definitive authentication response.

## App Startup and Logout

At startup, restore the secure session and verify it lazily with the first protected request or an explicit refresh. Do not discard the refresh token because the access token is expired. Clear the local session only after refresh expiration, revocation, reuse, invalidation, or logout.

For logout, call `POST /auth/logout` with the bearer access token and, when available, the JSON body:

```json
{
  "refreshToken": "<stored refresh token>"
}
```

After logout succeeds, clear all local authentication material. Never attempt to refresh after logout.

## Acceptance Tests

Verify all of the following on a real or instrumented Android device:

1. Login returns and securely stores both tokens.
2. An expired access token causes one refresh and the original request succeeds after one retry.
3. Five simultaneous expired requests produce one refresh request and all eligible requests complete.
4. Closing/reopening the app, killing its process, rebooting the device, and losing battery power preserve session recovery.
5. Every rotated refresh response atomically replaces the stored refresh token.
6. Reusing an old refresh token takes the app to login without an infinite retry loop.
7. A refresh token expires at its original `refreshTokenExpiresAt`, exactly 30 days after login; rotation must not extend this deadline.
8. Logout, administrator revocation, disabled users, and expired sessions require login again.

