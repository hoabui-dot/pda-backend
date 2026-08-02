# Master Prompt — Audit PDA Backend Against PDA App API Integration Requirements

## Role

You are working inside the existing backend repository:

```text
PDA_BACKEND
```

Act as a:

* senior enterprise backend architect;
* senior Go engineer;
* REST API and OpenAPI contract specialist;
* WMS integration architect;
* Kafka and distributed-systems engineer;
* mobile-backend integration specialist;
* technical documentation author.

Your task is to perform a complete, evidence-based comparison between the current PDA backend implementation and the API integration requirements produced from the existing Kotlin Android PDA application.

The PDA Android application is an external client and must not be modified during this task.

---

# 1. Required Input Document

The following file must be available to this task:

```text
PDA_APP_API_INTEGRATION_REQUIREMENTS.md
```

This document represents the backend capabilities required by the current PDA Android application, including:

* active screens and routes;
* user actions;
* Zebra DataWedge scanner actions;
* current repository operations;
* required read APIs;
* required write APIs;
* required request fields;
* expected response fields;
* common headers;
* authentication and bootstrap requirements;
* error codes;
* pagination;
* caching and freshness;
* refresh and invalidation;
* offline and retry behavior;
* idempotency;
* optimistic versioning;
* command-status behavior;
* workflow dependencies;
* UI data requirements.

Treat this document as the primary PDA client requirement source for this comparison.

Do not assume that every proposed endpoint in the document is final.

Where the PDA document labels an endpoint or contract as proposed, compare the required behavior and data rather than blindly requiring the same endpoint name.

---

# 2. Repository Boundary

There are two separate repositories:

```text
PDA_APP
Existing Kotlin Android application
External client
Not modified in this task

PDA_BACKEND
Current working repository
Responsible for:
- public PDA REST APIs;
- authentication and authorization;
- business workflow orchestration;
- PostgreSQL persistence;
- Redis caching;
- idempotency;
- optimistic concurrency;
- transactional outbox;
- Kafka integration;
- upstream WMS integration;
- observability;
- audit;
- error contracts.
```

Do not:

* create or modify Android code;
* create Compose screens;
* run Android Gradle tasks;
* require Android Studio, ADB, an emulator, or a Zebra device;
* use Android local mock behavior as proof of backend support;
* claim an API is supported without implementation evidence;
* claim a request or response contract matches without checking actual DTOs, handlers, OpenAPI, application services, and tests;
* add or modify production code during the audit unless explicitly instructed after the report is reviewed.

This task is inspection, comparison, documentation, and implementation planning only.

---

# 3. Primary Objective

Inspect the real PDA backend repository and determine whether it fully supports the attached PDA App integration requirements.

The audit must answer:

1. What PDA features are currently supported by the backend?
2. What PDA features are only partially supported?
3. What PDA features are completely unsupported?
4. Which backend APIs currently exist?
5. Which required PDA APIs are missing?
6. Which backend APIs appear unnecessary, duplicated, obsolete, or not used by the PDA App?
7. Which endpoint paths differ between the PDA requirement and backend implementation?
8. Which HTTP methods differ?
9. Which request headers differ?
10. Which path, query, and request-body fields differ?
11. Which request fields are missing, extra, incorrectly named, incorrectly typed, or incorrectly required?
12. Which response fields are missing, extra, incorrectly named, incorrectly typed, incorrectly nested, or unavailable at the required lifecycle point?
13. Which response envelopes differ?
14. Which status and enum values differ?
15. Which HTTP status codes and machine-readable error codes differ?
16. Which APIs lack idempotency support?
17. Which APIs lack optimistic version handling?
18. Which APIs lack operator, device, warehouse, role, or permission validation?
19. Which scanner workflows cannot currently be completed through backend APIs?
20. Which screens cannot currently be populated with backend response data?
21. Which mutations do not trigger the required cache invalidation or projection refresh?
22. Which APIs do not support required pagination, filters, search, sorting, or freshness metadata?
23. Which offline retry and command-status requirements are unsupported?
24. Which backend capabilities exist but are not exposed through the public PDA API?
25. What exact backend changes are required to fully support the PDA App?

The final report must be detailed enough that another backend AI agent can implement all missing backend functionality without re-inspecting the Android repository.

---

# 4. Source-of-Truth Priority

Use the following priority when sources disagree:

1. Actual backend source code and migrations.
2. Actual backend OpenAPI files.
3. Actual backend tests.
4. Current backend configuration and dependency wiring.
5. Current backend phase reports.
6. Backend architecture and strategy documents.
7. `PDA_APP_API_INTEGRATION_REQUIREMENTS.md`.

Interpretation rule:

* Backend source code is authoritative for what currently exists.
* The PDA integration document is authoritative for what the client currently requires.
* Architecture documents describe intended design but are not proof of implementation.
* Passing unit tests are not proof of production infrastructure verification.
* A DTO definition without handler or route wiring is not a supported API.
* A route without application-service and persistence behavior is not a complete feature.
* A backend capability not reachable through the public gateway is not fully supported by the PDA App.
* Mock behavior must not be reported as production support.

When a conflict exists, record it explicitly.

---

# 5. Mandatory Repository Inspection

Before writing the report, inspect the real repository.

At minimum inspect:

1. repository structure;
2. `go.mod` and `go.work`;
3. all service entry points under `cmd/`;
4. gateway route registration;
5. public route prefixes;
6. authentication middleware;
7. authorization policies;
8. device and warehouse context middleware;
9. request correlation middleware;
10. all HTTP handlers;
11. request DTOs;
12. response DTOs;
13. common response envelopes;
14. error mapping;
15. domain models and enums;
16. application services and use cases;
17. repository interfaces;
18. PostgreSQL adapters;
19. SQL migrations;
20. Redis cache adapters;
21. idempotency implementation;
22. optimistic-lock/version implementation;
23. outbox and inbox implementation;
24. Kafka publishers and consumers;
25. WMS integration ports and adapters;
26. command-status implementation;
27. audit implementation;
28. OpenAPI files;
29. contract tests;
30. unit tests;
31. integration tests;
32. architecture tests;
33. Docker Compose and runtime configuration;
34. phase reports under `requirement-report/`;
35. deferred-verification documents;
36. backend architecture and implementation-rule documents;
37. existing TODO, FIXME, deferred, blocked, or placeholder code.

Do not base the audit only on filenames or documentation.

Trace every important backend capability from:

```text
Public route
→ middleware
→ handler
→ request DTO
→ application service
→ domain validation
→ repository/persistence
→ outbox/event behavior
→ response DTO
→ error mapping
→ tests
```

---

# 6. Required Output Files

Create exactly:

```text
PDA_BACKEND_PDA_APP_INTEGRATION_GAP_REPORT.md
```

Also create:

```text
requirement-report/YYYY-MM-DD-pda-backend-app-integration-audit.md
```

Do not modify implementation code during this audit.

---

# 7. Required Status Classification

Use only these primary statuses:

```text
SUPPORTED
PARTIALLY_SUPPORTED
NOT_SUPPORTED
CONTRACT_MISMATCH
BACKEND_ONLY
PROPOSED_ONLY
BLOCKED_BY_EXTERNAL_DEPENDENCY
NOT_VERIFIED
NOT_APPLICABLE
```

Definitions:

* `SUPPORTED`: implemented, publicly reachable, contract-compatible, and sufficiently tested.
* `PARTIALLY_SUPPORTED`: some required behavior exists, but fields, validation, workflow steps, errors, tests, or integrations are incomplete.
* `NOT_SUPPORTED`: no usable implementation exists.
* `CONTRACT_MISMATCH`: the backend API exists but does not match the PDA requirement.
* `BACKEND_ONLY`: the backend exposes functionality not currently required or used by the PDA App.
* `PROPOSED_ONLY`: present only in documentation or OpenAPI proposal, not implemented.
* `BLOCKED_BY_EXTERNAL_DEPENDENCY`: implementation or verification requires unavailable Kafka, WMS, OIDC, security infrastructure, or staging infrastructure.
* `NOT_VERIFIED`: code may exist but no reliable runtime or test evidence proves it.
* `NOT_APPLICABLE`: the PDA requirement does not require this backend capability.

Do not classify an item as `SUPPORTED` based only on architecture intent.

---

# 8. Required Report Structure

The generated `PDA_BACKEND_PDA_APP_INTEGRATION_GAP_REPORT.md` must contain every section below.

---

## 8.1 Document Header

Include:

```markdown
# PDA Backend — PDA App Integration Gap Report

- Document status
- Generated date
- Backend repository
- Backend branch
- Backend commit
- PDA requirement document
- Backend runtime modes inspected
- Audit scope
- Source-of-truth policy
- Implementation modification status
```

State explicitly:

```text
No PDA Android code was modified.
No backend implementation code was modified during this audit.
```

---

## 8.2 Executive Summary

Summarize:

* total PDA features reviewed;
* fully supported features;
* partially supported features;
* unsupported features;
* contract mismatches;
* missing public endpoints;
* backend-only endpoints;
* missing request fields;
* missing response fields;
* mismatched status/error contracts;
* missing idempotency/version support;
* external verification blockers;
* whether full PDA integration can begin immediately;
* highest-priority blockers.

Create:

| Area               | PDA requirement | Current backend state | Status | Main gap |
| ------------------ | --------------- | --------------------- | ------ | -------- |
| Authentication     |                 |                       |        |          |
| Bootstrap          |                 |                       |        |          |
| Dashboard          |                 |                       |        |          |
| Task Center        |                 |                       |        |          |
| Receiving          |                 |                       |        |          |
| Putaway            |                 |                       |        |          |
| Picking            |                 |                       |        |          |
| Replenishment      |                 |                       |        |          |
| Inventory Inquiry  |                 |                       |        |          |
| Stock Transfer     |                 |                       |        |          |
| Cycle Count        |                 |                       |        |          |
| Shipping           |                 |                       |        |          |
| Scanner validation |                 |                       |        |          |
| Offline/retry      |                 |                       |        |          |
| Error contract     |                 |                       |        |          |
| Cache/freshness    |                 |                       |        |          |
| Kafka              |                 |                       |        |          |
| WMS integration    |                 |                       |        |          |

---

## 8.3 Backend Runtime and Integration State

Document the current runtime support:

| Capability      | Current mode | Implementation evidence | Runtime verified | Production ready |
| --------------- | ------------ | ----------------------- | ---------------: | ---------------: |
| Authentication  |              |                         |                  |                  |
| PostgreSQL      |              |                         |                  |                  |
| Redis           |              |                         |                  |                  |
| Kafka publisher |              |                         |                  |                  |
| Kafka consumer  |              |                         |                  |                  |
| DLQ             |              |                         |                  |                  |
| Kafka ACL       |              |                         |                  |                  |
| Kafka TLS       |              |                         |                  |                  |
| WMS adapter     |              |                         |                  |                  |
| OpenTelemetry   |              |                         |                  |                  |
| Metrics         |              |                         |                  |                  |
| Audit           |              |                         |                  |                  |

Clearly distinguish:

* implemented in code;
* verified by automated tests;
* verified against a real dependency;
* deferred;
* blocked;
* production-ready.

---

## 8.4 Complete Current Backend API Inventory

Create a complete inventory of all public PDA backend routes.

| API ID | Method | Public path | Gateway route | Owning service | Handler | Request DTO | Response DTO | Auth required | Current status |
| ------ | ------ | ----------- | ------------- | -------------- | ------- | ----------- | ------------ | ------------: | -------------- |

Include:

* implemented routes;
* disabled routes;
* internal-only routes;
* health and operational routes;
* routes defined only in OpenAPI;
* routes defined in code but missing from OpenAPI;
* routes exposed by internal services but not by the gateway.

Mark non-PDA operational endpoints separately.

---

## 8.5 PDA Requirement-to-Backend Feature Matrix

For every PDA feature from `PDA_APP_API_INTEGRATION_REQUIREMENTS.md`, create:

| Feature | PDA screens | Required backend operations | Current backend evidence | Status | Missing capability |
| ------- | ----------- | --------------------------- | ------------------------ | ------ | ------------------ |

Do not combine distinct workflows into one vague row.

At minimum include:

* login;
* token refresh;
* logout;
* current profile;
* warehouse selection;
* device registration;
* bootstrap;
* dashboard;
* task summary;
* task search;
* task claim;
* task release;
* receiving list;
* receiving detail;
* receiving start;
* receiving barcode resolution;
* receiving quantity confirmation;
* receiving completion;
* putaway list;
* putaway detail;
* source validation;
* destination suggestion;
* destination validation;
* putaway confirmation;
* picking list;
* picking detail;
* location validation;
* item resolution;
* pick confirmation;
* picking completion;
* replenishment list;
* replenishment detail;
* validation;
* quantity confirmation;
* replenishment completion;
* inventory search;
* inventory balance;
* inventory movement history;
* transfer validation;
* transfer confirmation;
* cycle-count list;
* cycle-count detail;
* count submission;
* variance handling;
* recount;
* shipping summary;
* shipping readiness;
* shipping confirmation;
* command-status lookup;
* audit-relevant mutation metadata.

---

## 8.6 Missing API Analysis

Create:

| Gap ID | PDA feature | Required operation | Required method/path | Existing equivalent | Gap type | Severity | Recommended action |
| ------ | ----------- | ------------------ | -------------------- | ------------------- | -------- | -------- | ------------------ |

Gap types must include:

```text
MISSING_ENDPOINT
MISSING_WORKFLOW_STEP
MISSING_PUBLIC_GATEWAY_ROUTE
MISSING_HANDLER
MISSING_APPLICATION_USE_CASE
MISSING_PERSISTENCE
MISSING_VALIDATION
MISSING_ERROR_MAPPING
MISSING_TEST
MISSING_OPENAPI
MISSING_EXTERNAL_VERIFICATION
```

For each missing API, explain:

* which PDA screen or action requires it;
* required request data;
* required response data;
* expected errors;
* idempotency requirement;
* version requirement;
* authorization scope;
* cache/refresh impact;
* recommended backend owner.

---

## 8.7 Extra, Duplicate, and Unused API Analysis

Create:

| API | Current backend purpose | PDA usage found | Classification | Risk | Recommendation |
| --- | ----------------------- | --------------- | -------------- | ---- | -------------- |

Classify each as:

```text
REQUIRED
BACKEND_INTERNAL
OPERATIONAL
FUTURE_USE
DUPLICATE
OBSOLETE
UNUSED_BY_PDA
OUT_OF_SCOPE
```

Do not recommend deleting an endpoint solely because the current PDA App does not use it.

Explain whether it belongs to:

* internal service communication;
* operations;
* WMS integration;
* asynchronous event handling;
* future approved functionality;
* accidental duplication;
* legacy functionality.

---

## 8.8 Endpoint Path and HTTP Method Comparison

Create:

| PDA action | PDA required/proposed method | PDA required/proposed path | Backend method | Backend path | Match | Recommended canonical contract |
| ---------- | ---------------------------- | -------------------------- | -------------- | ------------ | ----- | ------------------------------ |

Identify:

* path naming differences;
* pluralization differences;
* nested-resource differences;
* command-style versus resource-style differences;
* query parameter differences;
* route version differences;
* gateway prefix differences;
* HTTP method differences.

Do not classify a naming difference as a blocking issue when the behavior is compatible and the PDA contract was only proposed.

Still record the difference and recommend one canonical contract.

---

## 8.9 Request Contract Comparison

For every required API, compare:

* path parameters;
* query parameters;
* request headers;
* request body;
* required versus optional fields;
* nullability;
* field names;
* data types;
* formats;
* enum values;
* validation rules;
* default values;
* maximum sizes;
* idempotency;
* versioning.

Create:

| API | Field location | PDA field | PDA type/requirement | Backend field | Backend type/requirement | Status | Impact |
| --- | -------------- | --------- | -------------------- | ------------- | ------------------------ | ------ | ------ |

Use detailed mismatch types:

```text
FIELD_MISSING_IN_BACKEND
FIELD_EXTRA_IN_BACKEND
NAME_MISMATCH
TYPE_MISMATCH
NULLABILITY_MISMATCH
REQUIREDNESS_MISMATCH
FORMAT_MISMATCH
ENUM_MISMATCH
VALIDATION_MISMATCH
HEADER_MISMATCH
QUERY_MISMATCH
PATH_PARAMETER_MISMATCH
BODY_STRUCTURE_MISMATCH
```

For every mutation, include the complete PDA-required request JSON and current backend request JSON.

Then provide a normalized recommended request JSON.

---

## 8.10 Response Contract Comparison

For every API, compare:

* top-level envelope;
* `data`;
* `meta`;
* `errors`;
* entity fields;
* nested entities;
* pagination;
* version;
* server time;
* correlation ID;
* freshness timestamp;
* command status;
* warnings;
* next workflow step;
* scanner validation result;
* allowed actions.

Create:

| API | PDA-required response field | Backend response field | Type | Status | PDA impact | Recommended change |
| --- | --------------------------- | ---------------------- | ---- | ------ | ---------- | ------------------ |

Use mismatch types:

```text
RESPONSE_FIELD_MISSING
RESPONSE_FIELD_EXTRA
RESPONSE_NAME_MISMATCH
RESPONSE_TYPE_MISMATCH
RESPONSE_NESTING_MISMATCH
RESPONSE_ENUM_MISMATCH
RESPONSE_ENVELOPE_MISMATCH
PAGINATION_MISMATCH
VERSION_MISSING
FRESHNESS_MISSING
CORRELATION_MISSING
SERVER_TIME_MISSING
COMMAND_STATUS_MISSING
```

For each API, include:

1. PDA-required response example;
2. current backend response example;
3. recommended normalized response example.

Do not invent current backend examples. Derive them from actual DTOs, handlers, tests, or runtime fixtures.

---

## 8.11 Common Header Comparison

Compare at least:

```http
Authorization: Bearer <token>
X-Correlation-Id: <uuid>
X-Device-Id: <device-id>
X-Warehouse-Id: <warehouse-id>
Idempotency-Key: <uuid>
If-Match: "<version>"
Accept-Language: vi-VN
Content-Type: application/json
```

Create:

| Header | PDA requirement | Backend implementation | Required APIs | Retry behavior | Status | Gap |
| ------ | --------------- | ---------------------- | ------------- | -------------- | ------ | --- |

Check:

* case-insensitive handling;
* generation responsibility;
* propagation;
* validation;
* logging/redaction;
* persistence where needed;
* retry reuse;
* correlation propagation to Kafka and audit events.

---

## 8.12 Authentication, Authorization, Device, and Warehouse Comparison

Create:

| Requirement | PDA expectation | Backend implementation | Status | Gap |
| ----------- | --------------- | ---------------------- | ------ | --- |

Cover:

* login request;
* login response;
* access token;
* refresh token;
* expiration;
* refresh endpoint;
* logout/revocation;
* current operator;
* roles;
* permissions;
* allowed warehouses;
* selected warehouse;
* device registration;
* device status;
* warehouse scope;
* operator-device binding;
* feature visibility;
* language;
* server time;
* mock versus production authentication;
* RBAC/ABAC;
* denied-operation audit.

Identify any trust placed incorrectly in PDA-provided role, operator, warehouse, or device values.

---

## 8.13 Status and Enum Comparison

Create:

| Domain | PDA-required value | Backend value | Match | Mapping required | Risk |
| ------ | ------------------ | ------------- | ----- | ---------------- | ---- |

Cover all relevant enums:

* task status;
* task category;
* priority;
* receiving status;
* line status;
* movement status;
* picking status;
* replenishment status;
* count status;
* variance status;
* shipment status;
* readiness status;
* command status;
* scanner validation status;
* retryability;
* freshness/stale state.

Do not assume semantically similar enum names are directly compatible.

---

## 8.14 Error Contract Comparison

Create:

| PDA-required error code | Backend code | HTTP status | Retryable | Client behavior | Status | Required change |
| ----------------------- | ------------ | ----------: | --------: | --------------- | ------ | --------------- |

At minimum evaluate:

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

Also inventory every stable backend error code not listed by the PDA requirement.

Check whether raw PostgreSQL, Redis, Kafka, HTTP-client, or stack-trace details can leak to the PDA client.

---

## 8.15 Screen-to-API Coverage

For every PDA screen, create:

| Screen | Data required | Current backend API | Missing fields | Missing actions | Status | Integration blocker |
| ------ | ------------- | ------------------- | -------------- | --------------- | ------ | ------------------- |

Explain whether the screen can:

* load;
* display all required fields;
* refresh;
* handle an empty state;
* handle errors;
* execute all actions;
* recover from conflicts;
* handle scanner input;
* navigate to the next workflow step.

Do not mark a screen as supported if only its list API exists but required mutation steps are missing.

---

## 8.16 Scanner-to-API Comparison

For every scanner purpose from the PDA requirement, create:

| Scan purpose | Screen/workflow | Required backend validation | Current endpoint | Request mismatch | Response mismatch | Status |
| ------------ | --------------- | --------------------------- | ---------------- | ---------------- | ----------------- | ------ |

Check support for:

* raw barcode;
* normalized barcode;
* symbology;
* scanner source;
* task ID;
* line ID;
* warehouse ID;
* device ID;
* operator context;
* correlation ID;
* resolved entity;
* validation status;
* next workflow step;
* quantity constraints;
* task/entity version;
* warning/error code.

Identify scanner validations that are currently performed only locally and require authoritative backend validation.

---

## 8.17 Repository Operation and Use-Case Traceability

Create:

| PDA repository operation | PDA screen/ViewModel | Required API | Backend route | Backend use case | Persistence/event effect | Status |
| ------------------------ | -------------------- | ------------ | ------------- | ---------------- | ------------------------ | ------ |

Every PDA read and mutation operation must have one traceable backend path or a documented gap.

---

## 8.18 Idempotency and Versioning Audit

Create:

| Mutation | PDA requires idempotency | Backend support | PDA requires version | Backend support | Duplicate behavior | Conflict behavior | Status |
| -------- | -----------------------: | --------------: | -------------------: | --------------: | ------------------ | ----------------- | ------ |

Audit at least:

* task claim;
* task release;
* receiving start;
* receiving confirmation;
* receiving completion;
* putaway confirmation;
* pick confirmation;
* picking completion;
* replenishment confirmation;
* transfer confirmation;
* cycle-count submission;
* recount;
* shipping confirmation.

Check:

* `Idempotency-Key`;
* `commandId`;
* persisted idempotency truth;
* replay of previous result;
* duplicate-in-progress handling;
* `If-Match`;
* `baseVersion`;
* optimistic locking;
* conflict response;
* command-status lookup;
* retry reuse of the same identifier.

---

## 8.19 Cache, Freshness, Refresh, and Invalidation Audit

Create:

| API/data | PDA freshness requirement | Backend Redis behavior | Response freshness metadata | Mutation invalidation | Status | Gap |
| -------- | ------------------------- | ---------------------- | --------------------------- | --------------------- | ------ | --- |

Cover:

* dashboard;
* task summary;
* task lists;
* master data;
* inventory inquiry;
* shipment readiness;
* barcode resolution where relevant.

Verify that mutations invalidate or refresh:

* dashboard;
* task counts;
* task detail;
* inventory balance;
* movement history;
* receiving state;
* picking state;
* shipment readiness.

Check whether stale data is clearly marked.

---

## 8.20 Offline, Retry, and Command-Status Audit

Create:

| Mutation | PDA offline requirement | Backend retry safety | Idempotency | Command-status API | Conflict handling | Status |
| -------- | ----------------------- | -------------------- | ----------- | ------------------ | ----------------- | ------ |

Identify:

* operations that may be queued offline;
* operations that must remain online-only;
* automatic retry eligibility;
* command ID reuse;
* timeout ambiguity;
* unknown-success-state handling;
* command-status lookup support;
* duplicate handling;
* stale-version behavior.

---

## 8.21 Database, Outbox, Kafka, and WMS Effect Audit

For each mutation, create:

| Mutation | Database state change | Transaction boundary | Outbox event | Kafka publication | WMS effect | Audit record | Status |
| -------- | --------------------- | -------------------- | ------------ | ----------------- | ---------- | ------------ | ------ |

Verify that:

* the database mutation commits before success is returned;
* outbox insertion is atomic with the mutation;
* Kafka is not used as the sole source of truth;
* Kafka failure does not fabricate mutation failure after database commit;
* Kafka failure does not silently discard events;
* WMS failure follows the approved consistency policy;
* audit metadata includes operator, device, warehouse, command, and correlation context.

Clearly mark any behavior that is implemented but not externally verified.

---

## 8.22 OpenAPI Comparison

Compare the PDA requirements against the backend OpenAPI.

Create:

| API | Implemented in code | Present in OpenAPI | Contract matches code | Contract matches PDA | Status |
| --- | ------------------: | -----------------: | --------------------: | -------------------: | ------ |

Identify:

* undocumented implementation;
* documented but unimplemented endpoints;
* stale schemas;
* missing examples;
* incorrect required fields;
* incorrect enums;
* incorrect HTTP status codes;
* missing security definitions;
* missing headers;
* missing error schemas.

---

## 8.23 Test Coverage Analysis

Create:

| Requirement | Unit test | Handler/API test | Database integration test | Contract test | E2E test | External dependency test | Status |
| ----------- | --------: | ---------------: | ------------------------: | ------------: | -------: | -----------------------: | ------ |

Do not treat mocked Kafka tests as proof of:

* Kafka ACL;
* Kafka TLS;
* durable broker outage recovery;
* real DLQ delivery;
* production consumer lag;
* production event ordering.

Do not treat mocked WMS tests as proof of real WMS integration.

---

## 8.24 Severity and Priority Rules

Use:

```text
P0 — data corruption, security violation, fabricated success, duplicate warehouse mutation, or impossible core workflow
P1 — blocks a required PDA workflow or prevents safe production integration
P2 — important contract mismatch, missing validation, missing refresh, or poor recoverability
P3 — non-blocking inconsistency, documentation, naming, or maintainability issue
```

Each gap must include:

* severity;
* affected screens;
* affected APIs;
* user impact;
* data-integrity impact;
* security impact;
* implementation dependency;
* recommended phase.

---

## 8.25 Detailed Implementation Backlog

Create an actionable backlog:

| Work item ID | Priority | Backend area | Required change | Files/packages likely affected | Tests required | Dependency | Acceptance criteria |
| ------------ | -------- | ------------ | --------------- | ------------------------------ | -------------- | ---------- | ------------------- |

Group work into:

1. common API contract foundation;
2. authentication/bootstrap;
3. dashboard and tasks;
4. receiving;
5. putaway;
6. picking;
7. replenishment;
8. inventory inquiry;
9. stock transfer;
10. cycle count;
11. shipping;
12. scanner validation;
13. idempotency/versioning;
14. cache/invalidation;
15. offline/command status;
16. Kafka/WMS integration;
17. observability/security;
18. E2E and release verification.

The backlog must be implementable directly by another AI agent.

Avoid vague tasks such as:

```text
Fix receiving API.
Improve response.
Add missing validation.
```

Use explicit tasks such as:

```text
Add `baseVersion` to `ConfirmReceiptRequest`, validate it against the current
receiving aggregate version, return `409 TASK_VERSION_CONFLICT` with the current
version, update OpenAPI, and add stale-version handler and PostgreSQL integration tests.
```

---

## 8.26 Recommended Canonical API Contract

After identifying all mismatches, propose one canonical backend contract for the PDA App.

For each endpoint include:

```markdown
### METHOD /api/pda/v1/...

#### Purpose

#### Authentication and authorization

#### Required headers

#### Path parameters

#### Query parameters

#### Request body

#### Success response

#### Error responses

#### Idempotency behavior

#### Version/conflict behavior

#### Cache/freshness behavior

#### Side effects

#### Events

#### PDA screens using this endpoint
```

Only include endpoints required for full PDA support.

Clearly distinguish:

* existing compatible endpoint;
* existing endpoint requiring modification;
* new endpoint required;
* backend-only endpoint not part of the PDA public contract.

---

## 8.27 Proposed Implementation Sequence

Provide a safe implementation sequence.

Recommended default order:

1. freeze common response and error envelope;
2. freeze authentication, headers, device, warehouse, and correlation contract;
3. reconcile OpenAPI with existing implementation;
4. complete dashboard and task reads;
5. complete receiving as the reference vertical slice;
6. complete inventory inquiry;
7. complete putaway;
8. complete picking;
9. complete replenishment;
10. complete stock transfer;
11. complete cycle count;
12. complete shipping;
13. complete command-status and offline retry support;
14. complete cache invalidation and freshness metadata;
15. complete Kafka and WMS external verification;
16. execute full PDA-to-backend E2E.

Adjust the order when repository dependencies require it, but explain every change.

---

## 8.28 Integration Readiness Decision

Provide one final decision:

```text
READY
READY_WITH_NON_BLOCKING_GAPS
NOT_READY
BLOCKED_BY_MISSING_CLIENT_CONTRACT
BLOCKED_BY_EXTERNAL_DEPENDENCY
```

The decision must include:

* blocking APIs;
* blocking contract mismatches;
* blocking request fields;
* blocking response fields;
* blocking error behavior;
* blocking security issues;
* blocking external dependencies;
* minimum work required before the PDA App can replace mock repositories with real API calls.

---

# 9. Evidence Requirements

Every important claim must include backend evidence.

Use evidence such as:

```markdown
Evidence:
- `cmd/pda-api-gateway/main.go`
- `internal/execution/receiving/adapters/http/handler.go`
- `internal/execution/receiving/application/confirm.go`
- `internal/execution/receiving/adapters/postgres/repository.go`
- `api/openapi/pda-v1.yaml`
- `internal/execution/receiving/adapters/http/handler_test.go`
```

For missing functionality, include the locations inspected before concluding that it is missing.

Example:

```markdown
Evidence inspected:
- gateway route registration;
- receiving HTTP adapter;
- receiving application package;
- OpenAPI receiving paths;
- repository-wide search for `barcode-resolutions`.

Conclusion:
No public receiving barcode-resolution endpoint is currently implemented.
```

Do not claim something is missing after searching only one package.

---

# 10. Required Detailed API Comparison Format

For every required endpoint use this exact structure:

```markdown
## API-XXX — Feature and operation

### PDA requirement

### Current backend implementation

### Public route availability

### Request contract comparison

### Response contract comparison

### Header comparison

### Authentication and authorization

### Validation behavior

### Idempotency behavior

### Version and conflict behavior

### Error behavior

### Database and transaction effect

### Outbox and event effect

### Cache and invalidation effect

### Offline and retry compatibility

### Test coverage

### Status

### Severity

### Required backend changes

### Acceptance criteria

### Evidence
```

Do not omit sections because an endpoint is missing.

For a missing endpoint, describe the full required contract and implementation acceptance criteria.

---

# 11. Required Request and Response Examples

For every mutation include:

1. PDA-required request;
2. current backend request;
3. recommended canonical request;
4. PDA-required response;
5. current backend response;
6. recommended canonical response;
7. error response examples.

Use actual backend field names for current examples.

Never invent current fields.

When backend behavior is unavailable, write:

```text
Current backend request: not implemented.
Current backend response: not implemented.
```

---

# 12. No Silent Contract Relaxation

Do not dismiss a mismatch because the backend contains “similar” data.

Examples:

* `operatorId` is not automatically equivalent to authenticated operator context.
* `version` is not automatically equivalent to `baseVersion`.
* `commandId` is not automatically equivalent to an `Idempotency-Key`.
* an HTTP `200` with an error field is not equivalent to a typed `409`.
* a local barcode lookup is not equivalent to authoritative backend validation.
* a task list API is not equivalent to a task detail API.
* a generic mutation endpoint is not automatically equivalent to explicit workflow-step endpoints.
* an internal service route is not equivalent to a public gateway route.
* an outbox row is not proof that Kafka publication was externally verified.
* a mock WMS fixture is not proof of WMS integration.

Record each semantic difference.

---

# 13. No Unnecessary API Duplication

When the PDA requirement proposes an endpoint that overlaps with an existing backend endpoint:

1. compare semantics;
2. compare request data;
3. compare response data;
4. compare validation;
5. compare error behavior;
6. compare idempotency/versioning;
7. determine whether the existing endpoint can be retained;
8. recommend an adapter or DTO change before recommending a duplicate endpoint.

Prefer a stable canonical API over parallel endpoints for the same operation.

---

# 14. Backward Compatibility Analysis

For every recommended contract change, classify:

```text
NON_BREAKING
BREAKING_FOR_PDA_CLIENT
BREAKING_FOR_EXISTING_BACKEND_CLIENT
INTERNAL_ONLY
```

Explain:

* whether API versioning is required;
* whether an additive field is sufficient;
* whether an alias or compatibility mapper can be used;
* whether a migration period is needed;
* whether existing consumers are known.

Do not change `/api/pda/v1` behavior casually.

---

# 15. External Dependency and Deferred Verification Rules

If Kafka, WMS, OIDC, TLS certificates, Kafka ACLs, staging, or production infrastructure are unavailable:

* inspect implementation and automated tests;
* record what is implemented;
* record what is not externally verified;
* classify it as `BLOCKED_BY_EXTERNAL_DEPENDENCY` or `NOT_VERIFIED`;
* preserve deferred verification items;
* do not report production readiness;
* do not silently treat mock verification as equivalent.

The integration gap report must distinguish code gaps from environment-verification gaps.

---

# 16. Quality Requirements

The report must:

* be written in technical English;
* be complete even when long;
* use actual repository evidence;
* avoid vague conclusions;
* cover every PDA screen and workflow;
* cover every current public backend API;
* identify missing APIs;
* identify extra APIs;
* compare every request contract;
* compare every response contract;
* compare headers and common envelopes;
* compare status and error codes;
* compare idempotency and version behavior;
* compare cache, freshness, and invalidation;
* compare scanner validation;
* compare offline and retry behavior;
* identify security and authorization gaps;
* distinguish implementation from external verification;
* provide an actionable implementation backlog;
* provide acceptance criteria;
* be sufficient for another backend AI agent to implement full PDA App support.

Do not return only a summary or an outline.

---

# 17. Final Validation Checklist

Before finishing, verify that the report answers:

* [ ] Was the actual backend repository inspected?
* [ ] Was `PDA_APP_API_INTEGRATION_REQUIREMENTS.md` fully reviewed?
* [ ] Is every PDA feature mapped?
* [ ] Is every public backend route inventoried?
* [ ] Are missing endpoints identified?
* [ ] Are backend-only endpoints identified?
* [ ] Are duplicate or obsolete endpoints identified?
* [ ] Are method and path mismatches documented?
* [ ] Is every required request field compared?
* [ ] Is every required response field compared?
* [ ] Are required and optional fields compared?
* [ ] Are field types and nullability compared?
* [ ] Are enums and statuses compared?
* [ ] Are headers compared?
* [ ] Is the common response envelope compared?
* [ ] Are HTTP status codes and stable error codes compared?
* [ ] Is authentication and token behavior compared?
* [ ] Are device and warehouse scopes compared?
* [ ] Is scanner-to-API behavior compared?
* [ ] Is idempotency checked for every mutation?
* [ ] Is optimistic versioning checked for every conflict-sensitive mutation?
* [ ] Is command-status support checked?
* [ ] Is offline retry compatibility checked?
* [ ] Are cache freshness and invalidation checked?
* [ ] Are database, outbox, Kafka, and WMS effects checked?
* [ ] Is OpenAPI compared with implementation and PDA requirements?
* [ ] Is test coverage documented?
* [ ] Are external verification blockers separated from code gaps?
* [ ] Is a prioritized implementation backlog included?
* [ ] Are precise acceptance criteria included?
* [ ] Is a final integration-readiness decision provided?
* [ ] Can another AI agent implement full PDA support using this report alone?

---

# 18. Execution Instruction

Proceed directly with repository inspection.

First read:

```text
PDA_APP_API_INTEGRATION_REQUIREMENTS.md
```

Then inspect the complete PDA backend repository and create:

```text
PDA_BACKEND_PDA_APP_INTEGRATION_GAP_REPORT.md
```

Also create:

```text
requirement-report/YYYY-MM-DD-pda-backend-app-integration-audit.md
```

Do not modify Android code.

Do not modify backend production code during this audit.

After completing the files, report:

1. backend files and packages inspected;
2. input requirement document reviewed;
3. files created;
4. number of PDA features reviewed;
5. number of current public backend APIs;
6. number of fully supported APIs;
7. number of partially supported APIs;
8. number of missing APIs;
9. number of contract mismatches;
10. number of backend-only APIs;
11. number of P0, P1, P2, and P3 gaps;
12. external verification blockers;
13. final integration-readiness decision;
14. recommended first implementation work item.
