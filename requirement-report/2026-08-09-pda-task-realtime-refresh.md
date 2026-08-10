# PDA Task Realtime Refresh Fix

Date: 2026-08-09

## Finding

The PDA gateway served SSE and REST successfully initially, then both
`/api/pda/v1/events` and `/api/pda/v1/tasks` returned `401` continuously after
the access token expired. The mobile client retried with the same expired
token, so a logout/login was required before a newly assigned task appeared.

The backend SSE source was healthy: it connected with the operator/warehouse
scope and emitted a task snapshot. This was an authentication/session recovery
failure, not an assignment projection failure.

## Changes

- Added a synchronized OkHttp token authenticator for REST and SSE.
- Refreshes the PDA session with the durable refresh token, then retries the
  original request once.
- Added the same refresh behavior to background command synchronization.
- Serialized task refreshes with a mutex to prevent SSE/polling cache races.
- Added redacted PDA trace file logging at `files/pda-runtime-trace.log`.
- Added backend logs for SSE connect, snapshot, source failure, and disconnect.
- A reconnect now triggers an immediate authoritative REST refresh on the app;
  the fallback REST poll is 2 seconds and the backend task-source poll is 1
  second for the demo runtime.

## Runtime Evidence

After rebuilding the PDA Backend:

```text
GET /healthz = 200
sse_connected operatorId=... warehouseId=MAIN
sse_task_snapshot operatorId=... warehouseId=MAIN taskCount=1
```

No access token, password, or barcode is written to the mobile trace.

## Verification

- `GOWORK=off go test ./...`: PASS
- PDA Backend Docker image rebuilt and containers recreated: PASS
- PDA Backend health check: PASS
- PDA Backend Docker rebuild after the 1-second task-source polling change:
  PASS
- Android unit test/build attempt: blocked because the environment has no
  Android SDK (`ANDROID_HOME`/`local.properties` is missing).
