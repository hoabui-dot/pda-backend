# PDA Assignment and SSE Runtime Fix

## Root Cause

The WMS seed created the assignment correctly:

```text
receipt = c0ed91ab-bed5-4739-80ea-5edded4c8596
operator = 4c3f3b9f-6e0a-4e1c-87db-1d9ed9ad1001
assignment = CLAIMED
```

The PDA App was using an obsolete tunnel and warehouse identity. In addition, PDA Backend production `TaskAdapter` read only Warehouse Execution tasks; it did not merge Inbound receipt work into the common `/tasks` feed. The running PDA container was also in `PDA_UPSTREAM_WMS_MODE=mock`, which explains the `Development Operator` profile and the missing WMS assignment.

## Changes

* PDA Backend HTTP task adapter now merges operator-scoped Inbound receipts into the task feed as `RECEIVING` work.
* Inbound receipt filtering propagates `assigned_operator_id`.
* PDA Backend now exposes authenticated `GET /api/pda/v1/events` SSE.
* SSE emits `TASK_UPDATED` and `RESYNC_REQUIRED`; REST remains authoritative.
* Streaming status logging preserves `http.Flusher`.
* PDA production composition now defaults to HTTP mode and requires WMS credentials/configuration.
* PDA identity seed supports an explicit display name and was reseeded as `Nhân viên PDA 01`.
* PDA App now points to the live PDA tunnel and canonical WMS warehouse UUID `b29799e0-9c2e-49e6-91cc-6e8b620e6a1e`.
* Cached PDA profile hydrates from bootstrap so stale `Development Operator` data is replaced.

## Runtime Evidence

```text
POST /api/pda/v1/auth/login = 200
displayName = Nhân viên PDA 01
operatorId = 4c3f3b9f-6e0a-4e1c-87db-1d9ed9ad1001
warehouseId = b29799e0-9c2e-49e6-91cc-6e8b620e6a1e
GET /api/pda/v1/tasks = 200
RECEIVING task = c0ed91ab-bed5-4739-80ea-5edded4c8596
assignment = CLAIMED / IN_PROGRESS
local SSE = TASK_UPDATED
```

The local PDA gateway stream was verified. The Cloudflare quick tunnel did not flush the long-lived SSE body during the bounded probe, so tunnel buffering remains an environment limitation; REST task hydration remains the authoritative recovery path.

## Verification

```text
GOWORK=off go test ./... = PASS
PDA App git diff --check = PASS
PDA App commit/push = 5692488
```
