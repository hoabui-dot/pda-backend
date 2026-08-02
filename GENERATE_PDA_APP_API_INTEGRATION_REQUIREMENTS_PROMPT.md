# Master Prompt — Generate `PDA_APP_API_INTEGRATION_REQUIREMENTS.md`

## Role

You are working inside the existing Kotlin Android application repository:

```text
PDA_APP
Package: com.example.pda_app
```

Act as a senior Android architect, senior Kotlin engineer, API integration specialist, Zebra PDA workflow engineer, and technical documentation author.

Your task is to inspect the real PDA Android codebase and generate one complete Markdown document:

```text
PDA_APP_API_INTEGRATION_REQUIREMENTS.md
```

This document will be used by another AI agent and backend engineers to verify whether the separate `PDA_BACKEND` repository supports every required PDA screen, workflow, API contract, request payload, response payload, error case, refresh behavior, and scanner-driven action.

This task is documentation and analysis only unless explicitly instructed otherwise.

Do not modify backend code.
Do not create a backend repository.
Do not assume an API already exists.
Do not invent final API contracts without clearly marking them as proposed.

---

# 1. Repository Boundary

There are two separate repositories:

```text
PDA_APP
Existing Kotlin Android application
Current working repository
Responsibility:
- UI
- navigation
- Zebra DataWedge scanner integration
- ViewModels
- local state
- mobile API client requirements

PDA_BACKEND
Separate backend repository
External to this task
Responsibility:
- REST APIs
- authentication
- business workflows
- PostgreSQL
- Redis
- Kafka/mock messaging
- WMS integration
```

The document generated in this task describes what the Android PDA App needs from the backend.

Do not:

- create backend services;
- scaffold Spring Boot;
- modify `PDA_BACKEND`;
- inspect a backend repository unless it is explicitly provided;
- claim backend support based only on target architecture documents;
- confuse Android local mock behavior with real backend behavior.

---

# 2. Primary Objective

Generate a complete, evidence-based contract document describing:

1. every current PDA feature;
2. every active screen and route;
3. every user action;
4. every scanner-triggered action;
5. every current repository call;
6. every local mock mutation;
7. every piece of data displayed;
8. every backend read API required;
9. every backend write API required;
10. the exact request fields needed;
11. the exact response fields expected;
12. status values and error codes required;
13. authentication, device, warehouse, operator, correlation, version, and idempotency requirements;
14. caching and refresh behavior;
15. offline eligibility;
16. conflict and retry behavior;
17. API dependency between screens;
18. gaps between current local mock logic and production API integration.

The final document must be detailed enough that a backend AI agent can compare its OpenAPI and service implementation against the PDA requirements without re-reading the full Android repository.

---

# 3. Mandatory Inspection Before Writing

Inspect the actual repository before producing the document.

At minimum inspect:

1. `AndroidManifest.xml`
2. Gradle configuration and dependencies
3. navigation routes
4. authentication repository and ViewModel
5. every active Compose screen
6. every feature ViewModel
7. every UI state, event, and effect
8. all domain models and enums
9. all repository interfaces and implementations
10. mock database and deterministic fixtures
11. scanner purposes
12. DataWedge receiver and scan coordinator
13. DataStore preferences
14. current API/network dependencies
15. current Internet permission status
16. current DTO/API interfaces, if any
17. tests
18. ADRs
19. `NEW_AI_CONTEXT.md`
20. approved target UI and workflow documents present in the repository

Do not generate the document from old AI context files alone.

Source code is authoritative.

When source code and documentation differ:

- record the mismatch;
- treat the implementation as current state;
- separate current behavior from proposed production requirement.

---

# 4. Required Output

Create exactly:

```text
PDA_APP_API_INTEGRATION_REQUIREMENTS.md
```

at the repository root unless the repository has an explicit documentation convention.

Also create:

```text
requirement-report/YYYY-MM-DD-pda-api-integration-requirements.md
```

The report must include:

- files inspected;
- screens identified;
- current network/API status;
- files created;
- build/test result;
- unresolved business rules;
- backend contract uncertainties;
- physical Zebra verification status.

---

# 5. Documentation Rules

Use these labels consistently:

- **Current implementation**
- **Current local mock behavior**
- **Required backend behavior**
- **Proposed API contract**
- **Backend contract not yet confirmed**
- **Not implemented**
- **Deferred**

Every important claim must include evidence.

Use:

```markdown
Evidence:
- `app/src/main/java/.../ReceivingViewModel.kt`
- `WarehouseRepository.receive(...)`
- `WmsRoute.ReceivingDetail`
```

Do not claim that an API exists unless actual networking code exists.

When defining endpoint paths, label them:

```text
Proposed endpoint — not yet confirmed by backend
```

---

# 6. Required Structure of `PDA_APP_API_INTEGRATION_REQUIREMENTS.md`

The generated file must include all sections below.

---

# 7. Document Header

Include:

```markdown
# PDA App — Backend API Integration Requirements

- Document status
- Generated date
- Repository/package
- App version
- Current branch/commit if available
- Current network integration status
- Purpose
- Intended readers
- Source-of-truth policy
```

---

# 8. Executive Summary

Summarize:

- current PDA architecture;
- whether the app currently calls real APIs;
- current mock behavior;
- current scanner integration;
- number of active screens;
- number of required backend read operations;
- number of required backend mutation operations;
- top integration risks;
- whether backend integration can start immediately;
- what must be finalized first.

Include:

| Area | Current state | Required backend support | Readiness |
|---|---|---|---|
| Authentication | | | |
| Dashboard | | | |
| Task Center | | | |
| Receiving | | | |
| Putaway | | | |
| Picking | | | |
| Replenishment | | | |
| Inventory | | | |
| Transfer | | | |
| Cycle Count | | | |
| Shipping | | | |
| Scanner validation | | | |
| Persistence/offline | | | |
| Error/conflict model | | | |

---

# 9. Current Networking Capability

Inspect and report:

| Capability | Status | Evidence |
|---|---|---|
| Internet permission | | |
| HTTP client | | |
| Retrofit/Ktor/OkHttp | | |
| Serialization | | |
| API service interfaces | | |
| DTO package | | |
| Auth interceptor | | |
| Base URL config | | |
| Environment config | | |
| Error mapper | | |
| Retry policy | | |
| Room | | |
| WorkManager | | |

When no network stack exists, state explicitly:

```text
The current PDA App does not call a real backend. All warehouse data and mutations are local mock behavior.
```

---

# 10. Complete Screen and Workflow Inventory

Create a table:

| Screen ID | Screen | Route | Composable | ViewModel | Current repository calls | Scanner purpose | Backend integration required |
|---|---|---|---|---|---|---|---|

Include all active target screens.

At minimum cover:

- Login
- Dashboard
- Task Center
- Receiving List
- Receiving Detail
- Receiving Scan Item
- Receiving Confirm Quantity
- Putaway Task List
- Putaway Execution
- Picking Task List
- Picking Detail
- Replenishment
- Inventory Inquiry
- Stock Transfer
- Cycle Count
- Shipping Confirmation
- Profile/Settings if active

Clearly identify generic or legacy screens.

---

# 11. Screen-by-Screen API Requirements

For every active target screen create a detailed subsection using this exact structure.

```markdown
## SCR-04 — Inbound Receiving List

### Current implementation

### Current local mock behavior

### Data displayed

### User actions

### Scanner behavior

### Required read APIs

### Required write APIs

### Request fields

### Expected response fields

### Required status values

### Required error codes

### Cache and freshness behavior

### Refresh triggers

### Offline eligibility

### Conflict and retry behavior

### Navigation and downstream invalidation

### Backend contract uncertainties

### Evidence
```

Do not omit sections merely because a screen currently uses mock data.

---

# 12. API Endpoint Matrix

Create one complete matrix:

| Screen | User action | Proposed method/path | Read/write | Request DTO | Response DTO | Idempotency | Version required | Offline allowed | Backend owner |
|---|---|---|---|---|---|---|---|---|---|

Suggested public base path:

```text
/api/pda/v1
```

But mark it as proposed unless already approved.

---

# 13. Authentication and Bootstrap Contract

Document what the PDA needs after login.

Required analysis:

- credential submission;
- token type;
- access token;
- refresh token;
- token expiry;
- operator profile;
- role/permissions;
- warehouse/site;
- device registration;
- language;
- feature visibility;
- server time;
- scanner/device policy;
- logout/revocation.

Provide proposed request/response examples.

Do not preserve insecure mock credentials as a production requirement.

---

# 14. Common Request Headers

Document the exact headers the PDA client requires.

At minimum evaluate:

```http
Authorization: Bearer <token>
X-Correlation-Id: <uuid>
X-Device-Id: <device-id>
X-Warehouse-Id: <warehouse-id>
Idempotency-Key: <uuid>
If-Match: "<version>"
Accept-Language: vi-VN
```

For each header state:

- which calls require it;
- who generates it;
- whether it must survive retry;
- whether it is stored locally;
- validation behavior.

---

# 15. Common Response Envelope

Define the PDA client expectation.

```json
{
  "data": {},
  "meta": {
    "serverTime": "2026-08-01T10:00:00Z",
    "correlationId": "uuid",
    "version": "12",
    "nextCursor": null
  },
  "errors": []
}
```

Document:

- whether envelope is required;
- pagination metadata;
- entity version;
- freshness timestamp;
- server time;
- correlation ID;
- command status;
- pending state;
- localized versus machine-readable messages.

---

# 16. Common Error Contract

Create a required error matrix.

| Error code | Screen/workflow | Client behavior | Retryable | User action |
|---|---|---|---:|---|

At minimum include:

```text
AUTH_INVALID_CREDENTIALS
AUTH_SESSION_EXPIRED
DEVICE_NOT_REGISTERED
WAREHOUSE_ACCESS_DENIED
TASK_NOT_FOUND
TASK_NOT_ASSIGNED
TASK_LOCKED
TASK_ALREADY_COMPLETED
TASK_VERSION_CONFLICT
DUPLICATE_COMMAND
BARCODE_UNKNOWN
BARCODE_WRONG_CONTEXT
LOCATION_INVALID
LOCATION_CAPACITY_EXCEEDED
ITEM_NOT_IN_DOCUMENT
QUANTITY_EXCEEDS_ALLOWED
INSUFFICIENT_STOCK
COUNT_VARIANCE_REQUIRES_REVIEW
SHIPMENT_NOT_READY
RATE_LIMITED
UPSTREAM_WMS_UNAVAILABLE
INTERNAL_ERROR
```

Map each error to:

- Snackbar;
- inline field error;
- blocking dialog;
- screen refresh;
- logout;
- retry;
- conflict-resolution screen;
- scanner re-enable behavior.

---

# 17. Data Model Requirements

Inventory all PDA domain models that need backend representation.

Create:

| PDA model | Current fields | Backend DTO fields required | Missing production fields | Mapping concern |
|---|---|---|---|---|

Include:

- operator;
- warehouse;
- task;
- item;
- location;
- inventory balance;
- receiving task/line;
- putaway task;
- picking task/line;
- replenishment task;
- transfer;
- count task/line;
- shipment/package;
- audit metadata.

Required cross-cutting backend fields:

```text
id
warehouseId
status
version
createdAt
updatedAt
assignedOperatorId
priority
commandId
correlationId
sync/freshness timestamp
```

---

# 18. Current Repository Call to API Mapping

Create a traceability matrix:

| Current repository method | Current local behavior | Calling ViewModel/screen | Proposed backend endpoint | Request body | Response | Migration concern |
|---|---|---|---|---|---|---|

Include every current read and mutation method.

---

# 19. User Action to API Traceability

Create:

| Screen | UI control | Current ViewModel action | Current repository call | Required API call | Expected UI refresh |
|---|---|---|---|---|---|

Include all major actions, including login, logout, dashboard refresh, scan steps, quantity confirmations, transfers, count submission, shipping confirmation, retry, and refresh.

---

# 20. Scanner-to-API Contract

Document every scanner purpose.

| Scan purpose | Screen | Expected scanned entity | Local validation | Required backend validation | API response |
|---|---|---|---|---|---|

For each scan validation API specify:

- barcode raw value;
- normalized value;
- symbology;
- source;
- active task ID;
- active line ID;
- warehouse ID;
- device ID;
- operator ID;
- correlation ID.

Expected response should identify:

```text
resolved entity
entity ID
display code
validation status
next workflow step
quantity constraints
task/entity version
warning/error code
```

---

# 21. Request Body Specifications

For each mutation provide a proposed JSON request.

At minimum include:

## Receiving confirmation

```json
{
  "commandId": "uuid",
  "lineId": "LINE-01",
  "barcode": "00012345678905",
  "quantity": 3,
  "remark": null,
  "baseVersion": 11
}
```

## Putaway confirmation

```json
{
  "commandId": "uuid",
  "taskId": "PUT-001",
  "sourceLocationCode": "STAGE-01",
  "destinationLocationCode": "BIN-A-01",
  "itemId": "ITEM-001",
  "quantity": 3,
  "baseVersion": 5
}
```

Also include pick, replenishment, transfer, cycle count, and shipping requests.

These are proposed contracts and must be reconciled with actual PDA state fields.

---

# 22. Response Body Specifications

For each read and mutation provide the fields the PDA needs.

Document exact UI fields populated from each response.

---

# 23. Pagination, Search, Filter, and Sorting

For list APIs document:

- cursor pagination;
- page size;
- search field;
- status filter;
- date filter;
- supplier/customer;
- priority;
- warehouse/zone;
- default sort;
- count consistency;
- empty result behavior.

---

# 24. Cache and Freshness Requirements

For every API classify data:

| API/data | Freshness class | PDA local cache allowed | Immediate fetch | Stale UI required |
|---|---|---:|---:|---:|

Distinguish:

```text
PDA local cache:
Room/DataStore on Android

Backend local/shared cache:
Redis inside PDA_BACKEND
```

The PDA App must never directly call Redis.

---

# 25. Refresh and Invalidation Requirements

Create:

| Mutation | PDA screens/data to refresh | Backend cache/projection to invalidate |
|---|---|---|

Cover receiving, putaway, picking, replenishment, transfer, count, and shipping.

---

# 26. Offline and Retry Requirements

For every mutation classify:

| Mutation | Offline draft | Offline queue | Online-only | Safe automatic retry | Idempotency required |
|---|---:|---:|---:|---:|---:|

Explain command-ID reuse, duplicate-command handling, command-status checks, conflicts, and online-only actions.

---

# 27. API Dependency Graph

Provide workflow graphs for receiving, putaway, picking, replenishment, transfer, count, and shipping.

---

# 28. Backend Capability Checklist

Create one backend checklist per feature covering read APIs, write APIs, validation, idempotency, versioning, audit, inventory effects, errors, and response fields required by the PDA.

---

# 29. Backend Comparison Matrix

Create:

| PDA requirement | Proposed endpoint | Backend implementation evidence | Status | Gap |
|---|---|---|---|---|

If the backend repository is not available, use:

```text
Backend not inspected
```

Do not invent implementation evidence.

---

# 30. Integration Readiness Risks

Create:

| Priority | Risk | PDA impact | Backend requirement | Decision needed |
|---|---|---|---|---|

Prioritize data integrity, API contracts, scanner/API mismatch, auth/device scope, idempotency/version, stale cache, task locking, pagination/filter mismatch, localization, and observability.

---

# 31. Proposed Client API Architecture

Document the recommended Android integration shape:

```text
Compose
→ ViewModel
→ Use Case
→ Repository
   ├─ Room local data source
   ├─ Remote API data source
   └─ Sync/outbox coordinator
→ HTTP client
```

Do not add dependencies in this documentation task.

---

# 32. Implementation Sequence for PDA API Integration

Provide a recommended order:

1. common network contracts;
2. authentication/bootstrap;
3. dashboard/task reads;
4. receiving;
5. inventory inquiry;
6. putaway;
7. picking;
8. replenishment;
9. transfer;
10. cycle count;
11. shipping;
12. offline/outbox;
13. full E2E.

---

# 33. Required Test Plan

Document unit, integration, fake-server, Room/API synchronization, process-death, token-expiry, duplicate-command, version-conflict, backend-unavailable, and full E2E tests.

---

# 34. Required Appendices

## Appendix A — Screen-to-API Map
## Appendix B — Repository Method-to-API Map
## Appendix C — Scanner Purpose-to-API Map
## Appendix D — Request DTO Catalog
## Appendix E — Response DTO Catalog
## Appendix F — Error Code Catalog
## Appendix G — Current Mock-to-Backend Replacement Map
## Appendix H — Unconfirmed Business Rules
## Appendix I — Source Evidence Map

---

# 35. Quality Requirements

The document must:

- be written in technical English;
- be complete even when long;
- avoid vague statements;
- separate current behavior from proposed API behavior;
- include exact field names where source evidence exists;
- mark proposed endpoint names clearly;
- include request/response JSON examples;
- map every screen and major button;
- map every current repository operation;
- include scanner integration requirements;
- include caching, retry, conflict, and offline behavior;
- allow backend engineers to verify OpenAPI coverage;
- avoid implementing code during this task.

---

# 36. Final Validation Checklist

Before finishing, verify that the document answers:

- [ ] Does the PDA currently call any real API?
- [ ] What HTTP/network dependencies currently exist?
- [ ] What active screens exist?
- [ ] What API does every screen require?
- [ ] What request body does every write need?
- [ ] What response fields does every screen consume?
- [ ] What headers must be sent?
- [ ] Which writes require idempotency?
- [ ] Which writes require entity version?
- [ ] Which errors must the backend return?
- [ ] How does every scan map to backend validation?
- [ ] What is cached locally?
- [ ] What must be fetched immediately?
- [ ] What refreshes after every mutation?
- [ ] Which operations may work offline?
- [ ] Which operations must be online-only?
- [ ] How do current repository methods map to APIs?
- [ ] What backend capability is missing for each workflow?
- [ ] What tests are required before enabling real endpoints?
- [ ] Is the document sufficient for a backend AI to compare its OpenAPI and implementation?

---

# 37. Execution Instruction

Proceed directly with repository inspection.

Do not return only an outline.

Create:

```text
PDA_APP_API_INTEGRATION_REQUIREMENTS.md
```

Then report:

1. files inspected;
2. files created;
3. current network/API status;
4. number of screens mapped;
5. number of proposed read APIs;
6. number of proposed write APIs;
7. unresolved backend contracts;
8. build/test result;
9. physical Zebra verification status.
