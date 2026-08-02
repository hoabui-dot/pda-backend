# PDA Backend Reconciliation Rules v2

## 1. Purpose

These rules govern all phases that reconcile `PDA_BACKEND` with the field-level contract in:

```text
PDA_APP_COMPLETE_BACKEND_API_SPECIFICATION.md
```

The specification currently defines 17 screen/workflow records and 28 proposed API operations. It is authoritative for PDA-required fields and client behavior, but endpoint names remain proposed until backend review freezes the canonical contract.

## 2. Repository Boundary

- Work only in `PDA_BACKEND`.
- `PDA_APP` is an external Kotlin Android client.
- Do not create, modify, build, or verify Android code.
- Android source evidence may be read only to resolve contract ambiguity.
- Do not implement deferred LPN, pallet, adjustment, packing, bin-query, or generic module APIs unless separately approved.

Every phase report must begin with:

```text
Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.
```

## 3. Mandatory Inputs

Read before every phase:

1. `PDA_BACKEND_RECONCILIATION_RULES_V2.md`.
2. The actual PDA specification containing API-001 through API-028.
3. The latest backend integration-gap report.
4. The latest approved reconciliation phase report.
5. Current backend OpenAPI, routes, DTOs, services, migrations, and tests.

A generation prompt is not a client contract. The API specification dated 2026-08-02 is the current client requirement source.

## 4. Source-of-Truth and Contract Freeze

Use this priority:

1. Backend source and migrations for current implementation.
2. Backend OpenAPI and tests for published behavior.
3. PDA API specification for client-required data and behavior.
4. Approved business decisions produced during reconciliation.
5. Architecture documents for intended design.

Do not blindly rename backend routes to match proposed PDA paths. Compare semantics and freeze one canonical API.

Every endpoint decision must be classified:

- `KEEP_BACKEND_CONTRACT`
- `ADAPT_BACKEND_CONTRACT`
- `ADD_NEW_ENDPOINT`
- `ADD_COMPATIBILITY_ALIAS`
- `REJECT_PDA_PROPOSAL_WITH_REASON`
- `DEFERRED`

## 5. Mandatory API Coverage

No phase set is complete unless API-001 through API-028 are each mapped to:

- public route;
- request headers/query/body;
- response DTO;
- HTTP and machine error codes;
- authorization;
- idempotency/versioning where relevant;
- database/outbox/cache effects;
- tests;
- final status.

Required catalog:

```text
API-001 Login
API-002 Refresh
API-003 Logout
API-004 Bootstrap
API-005 Dashboard
API-006 Profile
API-007 Tasks/detail/claim/release
API-008 Receiving list/detail
API-009 Receiving barcode resolve
API-010 Receiving confirm
API-011 Putaway list/detail
API-012 Putaway source/item validation
API-013 Putaway destination validation/suggestion
API-014 Putaway confirm
API-015 Picking list/detail
API-016 Picking location validation
API-017 Picking item resolution
API-018 Pick/short-pick/complete
API-019 Replenishment list/detail
API-020 Replenishment validation
API-021 Replenishment confirm
API-022 Inventory inquiry/search/balance/history
API-023 Transfer validation/confirm
API-024 Command status
API-025 Cycle count workflow
API-026 Shipment summary/detail/readiness
API-027 Package verification
API-028 Shipment confirmation
```

## 6. Canonical Headers

Evaluate and document per route:

```http
Authorization: Bearer <token>
X-Correlation-Id: <uuid>
X-Device-Id: <device-id>
X-Warehouse-Id: <warehouse-id>
X-Operator-Id: <operator-id>
Idempotency-Key: <uuid>
If-Match: "<version>"
Accept-Language: vi-VN | en-US
Content-Type: application/json
```

Rules:

- Operator identity is token-derived; `X-Operator-Id` is verification/audit context, never authority.
- Warehouse and device values must be authorized server-side.
- Preserve correlation, idempotency, and version values across retries.
- Do not log secrets, tokens, passwords, or unrestricted barcode payloads.

## 7. Canonical Success and Error Envelopes

Success:

```json
{
  "data": {},
  "meta": {
    "serverTime": "2026-08-02T02:00:00Z",
    "correlationId": "uuid",
    "version": null,
    "nextCursor": null,
    "hasMore": false,
    "asOf": null,
    "stale": false
  },
  "errors": []
}
```

Error:

```json
{
  "data": null,
  "meta": {
    "serverTime": "2026-08-02T02:00:00Z",
    "correlationId": "uuid"
  },
  "errors": [
    {
      "code": "TASK_VERSION_CONFLICT",
      "message": "Safe fallback message",
      "details": {"currentVersion": 12},
      "retryable": false
    }
  ]
}
```

The PDA specification contains a legacy top-level error example. Backend reconciliation must freeze one envelope; do not support two shapes without an explicit compatibility requirement.

## 8. Request Identity and Version Rules

For every mutation:

- `commandId` is the domain command identifier.
- `Idempotency-Key` is the transport replay key.
- If both are retained, define equality or mapping rules and reject conflicting reuse.
- `baseVersion` is the body representation used by current PDA drafts.
- `If-Match` is the HTTP concurrency precondition.
- Freeze whether both are required, one is canonical, or one is a compatibility mirror.
- Duplicate success must replay the original result.
- Timeout ambiguity must be resolvable through API-024.

## 9. Scanner Contract Rules

Scanner validation APIs must evaluate where required:

- `rawValue` preserving leading zeros;
- `normalizedValue`;
- `symbology`;
- `scanContext`;
- source/device context;
- task/order/line context;
- client scan timestamp;
- authoritative token/device/warehouse context.

Do not depend on Android Intent extra names. Recoverable validation errors must not unnecessarily end the scanner workflow.

## 10. Business Rules Requiring Explicit Decisions

Do not silently choose these rules:

- over/under receipt tolerance;
- lot/serial/condition capture;
- location capacity and reservation;
- short-pick behavior;
- replenishment partial completion;
- blind-count visibility;
- variance approval/recount;
- task claim and lock duration;
- command-status retention;
- device registration policy;
- transfer, count submission, and shipment offline policies.

Each affected phase must create a decision table. Until approved, implement safe behavior stated in the PDA specification and mark the rule `UNCONFIRMED`.

## 11. Pagination and Freshness

All lists must evaluate:

- opaque cursor;
- default limit 50, maximum 100;
- deterministic `updatedAt,id` ordering;
- cursor binding to filters/sort;
- endpoint-specific filters;
- `nextCursor` and `hasMore`;
- `serverTime` and `asOf`;
- stale behavior.

Do not return total counts unless consistency is defined.

## 12. Data Integrity

- PostgreSQL remains authoritative.
- Redis is cache only.
- Mutations and outbox inserts are atomic.
- Count variance never silently adjusts inventory.
- Shipment success is never fabricated.
- Client timestamps are audit inputs; server timestamps are authoritative.
- Identity fields supplied by PDA cannot override authenticated context.

## 13. Testing and Verification

Every phase must run:

- baseline before editing;
- focused unit tests;
- handler/contract tests;
- PostgreSQL integration tests where state changes;
- OpenAPI validation;
- `go test ./...`;
- `make clean verify`.

Mock Kafka, WMS, and auth tests do not prove production verification.

## 14. Required Phase Report

Create:

```text
requirement-report/YYYY-MM-DD-pda-backend-reconciliation-phase-XX.md
```

Include:

- boundary confirmation;
- API IDs covered;
- current routes inspected;
- canonical route decisions;
- request changes;
- response changes;
- business decisions and unresolved rules;
- compatibility classification;
- migrations;
- tests;
- deferred external verification;
- status: `APPROVED`, `PARTIALLY_APPROVED`, or `BLOCKED`.

A later phase may start only after the previous phase is `APPROVED`, except independently documented external verification work.
