# PDA Receiving Detail Runtime Fix

## Finding

The PDA task feed returned receiving work from `GET /api/pda/v1/tasks`, but the
generic detail handler queried only the Warehouse Execution task store. A
receiving record therefore produced:

```text
GET /api/pda/v1/tasks/{receiving-id} -> 404 TASK_NOT_FOUND
GET /api/pda/v1/receiving/tasks/{receiving-id} -> 200
```

This was an owner-boundary mismatch, not missing receiving data. The receiving
detail response contained the purchase order, line, barcode, expected and
received quantities, version, assignment, and warehouse.

## Change

`GET /api/pda/v1/tasks/{id}` now falls back to the Receiving use-case port only
when the execution lookup returns `TASK_NOT_FOUND`. It returns the same generic
task shape, including receiving line snapshots. No database, ownership, or
business mutation was added.

The integration test now verifies that a receiving task listed in the generic
feed is readable through the generic detail route before running the existing
receiving confirmation flow.

## Verification

```text
GOWORK=off go test ./...                                             PASS
PDA_TEST_DATABASE_URL=<runtime URL> GOWORK=off go test -tags=integration \
  ./internal/gateway/adapters/http \
  -run TestReceivingAPIFromListThroughQuantityConfirmation -count=1       PASS
git diff --check                                                   PASS
```

Runtime observations after rebuild:

```text
GET /api/pda/v1/tasks                                      200
GET /api/pda/v1/tasks/14ea555d-df0d-4091-9ee8-5ff9e263b5ef 200, lineCount=1
GET /api/pda/v1/receiving/tasks/14ea555d-df0d-4091-9ee8-5ff9e263b5ef 200, lineCount=1
```

No access token or secret is stored in this report.

## Follow-up: `c4f826d0-f4ca-4417-b8da-e0fc1c852990`

This identifier is a WMS Receipt ID, but it is not valid for the current PDA
warehouse context. WMS resolves its receiving location as:

```text
location: FG-PACK-01
warehouse: WH-KZ3-FG
PDA MAIN alias: WH-KZ3-RM
```

The receipt was therefore correctly excluded from the `MAIN` operator task
feed. The adapter previously converted this authorization mismatch into an
unhelpful `500 INTERNAL_ERROR`; it now returns:

```text
403 WAREHOUSE_ACCESS_DENIED
Receiving task belongs to another warehouse
```

The same operator can read the valid `MAIN` receipt task
`1f826589-807a-416f-840c-a8f37c374300` with `200` and one line. This is a
warehouse fixture/assignment mismatch, not a missing PDA detail payload.

## Remaining Device Note

The physical PDA APK could not be rebuilt in this workspace because the Android
SDK is not installed and no device was connected to ADB. The gateway contract
and runtime endpoint are verified; install/retest the current APK against the
rebuilt gateway on the TC26 device.
