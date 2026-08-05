# Master Prompt — Generate the Final PDA App to PDA Backend Integration Implementation Document

## 1. Role

You are preparing the final integration plan between:

```text
PDA_APP
Existing Kotlin Android application
Package: com.example.pda_app
```

and:

```text
PDA_BACKEND
Existing completed Go backend
```

Act as a:

* senior Android architect;
* senior Kotlin and Jetpack Compose engineer;
* Android networking specialist;
* mobile authentication and secure-session engineer;
* offline-first synchronization architect;
* Zebra DataWedge integration engineer;
* backend API contract specialist;
* WMS workflow integration specialist;
* technical documentation author;
* integration and E2E test engineer.

Your task is to inspect the completed PDA Backend documentation and implementation evidence, then create one final, implementation-ready integration document that tells the PDA App engineering agent exactly how to replace the current local mock remote behavior with the real PDA Backend APIs.

This task is documentation and integration planning only unless explicitly instructed to implement the PDA App afterward.

---

# 2. Repository Boundary

There are two separate repositories:

```text
PDA_BACKEND
Completed backend repository
Contains:
- public PDA REST APIs;
- authentication;
- PostgreSQL;
- Redis;
- Kafka;
- WMS integration;
- OpenAPI;
- integration reports;
- production configuration;
- deployment and runtime documentation.

PDA_APP
Existing Kotlin Android application
External client
Contains:
- Compose UI;
- navigation;
- ViewModels;
- repositories;
- Room;
- WorkManager;
- scanner integration;
- local drafts;
- current mock remote behavior.
```

Do not:

* recreate the backend;
* modify backend business logic during this documentation task;
* connect the Android App directly to PostgreSQL;
* connect the Android App directly to Redis;
* connect the Android App directly to Kafka;
* connect the Android App directly to the WMS database;
* preserve mock warehouse mutations as the production integration path;
* invent backend endpoints without evidence;
* assume ports or paths from old documents when current runtime evidence differs.

The PDA App must communicate only through the PDA Backend public HTTP API.

---

# 3. Mandatory Backend Documents

Before writing the integration document, inspect the PDA Backend repository.

At minimum read:

```text
pda-backend/docs/integration-pda-app/PDA_APP_API_SPECIFICATION.md
pda-backend/PDA_BACKEND_PDA_APP_INTEGRATION_GAP_REPORT.md
pda-backend/api/openapi/pda-v1.yaml
```

Also inspect all completed reconciliation phase prompts and reports, including:

```text
pda-backend/docs/integration-pda-app/PDA_BACKEND_RECONCILIATION_RULES_V2.md
pda-backend/docs/integration-pda-app/README_PHASE_ORDER_V2.md
pda-backend/docs/integration-pda-app/PHASE_00_*.md
pda-backend/docs/integration-pda-app/PHASE_01_*.md
pda-backend/docs/integration-pda-app/PHASE_02_*.md
pda-backend/docs/integration-pda-app/PHASE_03_*.md
pda-backend/docs/integration-pda-app/PHASE_04_*.md
pda-backend/docs/integration-pda-app/PHASE_05_*.md
pda-backend/docs/integration-pda-app/PHASE_06_*.md
pda-backend/docs/integration-pda-app/PHASE_07_*.md
pda-backend/docs/integration-pda-app/PHASE_08_*.md
pda-backend/docs/integration-pda-app/PHASE_09_*.md
pda-backend/docs/integration-pda-app/PHASE_10_*.md
pda-backend/docs/integration-pda-app/PHASE_11_*.md
```

Inspect the corresponding reports under:

```text
pda-backend/requirement-report/
```

Also inspect:

* current Docker Compose files;
* gateway configuration;
* gateway route registration;
* authentication configuration;
* environment examples;
* startup scripts;
* Makefile targets;
* runtime logs or reports proving the exposed gateway port;
* health and readiness endpoints;
* current OpenAPI server/base-path configuration;
* TLS/HTTP runtime mode;
* current CORS or network restrictions where applicable.

Do not rely only on the original gap report.

The completed source code, OpenAPI, runtime configuration, and final phase reports are authoritative for the current backend state.

---

# 4. Network Environment

The PDA Backend runs on another server.

The PDA App development machine or device reaches that server through Tailscale.

Current backend server Tailscale IP:

```text
100.68.50.41
```

The final integration document must determine the real public gateway port from the current backend Docker Compose, configuration, runtime reports, or running services.

Do not guess the port.

The expected base URL format is:

```text
http://100.68.50.41:<PDA_GATEWAY_PORT>/api/pda/v1
```

or:

```text
https://100.68.50.41:<PDA_GATEWAY_PORT>/api/pda/v1
```

depending on the actual verified backend runtime.

The document must state explicitly:

* backend Tailscale IP;
* gateway host port;
* HTTP or HTTPS;
* public API base path;
* health endpoint;
* readiness endpoint;
* whether TLS certificate hostname validation affects direct IP access;
* whether a Tailscale DNS name should be used instead of the IP;
* whether Android cleartext traffic is required for development;
* whether production must use HTTPS;
* whether the Zebra device must have Tailscale installed and connected;
* how to verify network reachability from the development machine and physical device.

---

# 5. Primary Objective

Create a complete implementation document that enables another AI agent to integrate all supported PDA Backend APIs into `PDA_APP`.

The document must answer:

1. What backend URL must the PDA App use?
2. What port is exposed?
3. How is the base URL configured through environment variables or build configuration?
4. How are development, staging, and production environments separated?
5. How does the physical Zebra device reach the backend through Tailscale?
6. What Android permissions and network security configuration are required?
7. Which HTTP client stack should be used?
8. Which serializers and DTO packages should be created?
9. How are access and refresh tokens stored securely?
10. How is authentication performed?
11. How is token refresh serialized?
12. How are device and warehouse headers added?
13. How are correlation IDs generated and preserved?
14. How are idempotency keys preserved across retries and process death?
15. How are entity versions and `If-Match` handled?
16. How does every current PDA repository operation map to a real backend API?
17. How are backend responses mapped into domain models and Room entities?
18. How are mock repositories removed from the production integration path?
19. How do Room and WorkManager interact with the real remote API?
20. Which mutations are online-only?
21. Which mutations may be queued?
22. How is command status queried after timeout or duplicate response?
23. How are backend errors mapped into the current UI and scanner behavior?
24. How are all scanner purposes integrated with backend validation APIs?
25. What data must refresh after every mutation?
26. What tests are required before removing mock mode?
27. How is full PDA-to-backend E2E verified?

---

# 6. Required Output

Create exactly:

```text
PDA_APP_BACKEND_INTEGRATION_IMPLEMENTATION_GUIDE.md
```

Place it in the PDA App repository under its approved documentation convention.

Recommended path:

```text
docs/backend-integration/PDA_APP_BACKEND_INTEGRATION_IMPLEMENTATION_GUIDE.md
```

Also create:

```text
requirement-report/YYYY-MM-DD-pda-app-backend-integration-document.md
```

The report must include:

* backend documents inspected;
* backend source/configuration inspected;
* verified Tailscale IP;
* verified gateway port;
* verified protocol;
* verified base path;
* APIs mapped;
* screens mapped;
* scanner flows mapped;
* environment variables defined;
* unresolved external configuration;
* files created;
* no Android production code modification confirmation;
* final documentation readiness status.

---

# 7. Source-of-Truth Priority

Use this order:

1. Current PDA Backend source code and route registration.
2. Current PDA Backend OpenAPI.
3. Current Docker Compose and runtime configuration.
4. Final Phase 11 report.
5. Final reconciliation phase reports.
6. `PDA_APP_API_SPECIFICATION.md`.
7. Final integration gap report.
8. Older architecture and strategy documents.

When documents differ:

* use current implemented backend behavior;
* record the mismatch;
* do not silently combine incompatible contracts;
* state whether the PDA App must adapt or the backend documentation must be corrected.

---

# 8. Required Document Structure

The generated implementation guide must include all sections below.

---

## 8.1 Document Header

Include:

```markdown
# PDA App — PDA Backend Integration Implementation Guide

- Document status
- Generated date
- PDA App repository
- PDA App package
- PDA Backend repository
- Backend branch/commit
- PDA App branch/commit
- Backend server Tailscale IP
- Backend gateway port
- Protocol
- Public API base path
- Integration scope
- Source-of-truth policy
```

---

## 8.2 Executive Summary

Summarize:

* current PDA App networking state;
* current mock repository state;
* completed backend readiness;
* verified backend connection information;
* number of APIs;
* number of screens;
* number of scanner flows;
* required Android architecture changes;
* authentication strategy;
* Room/WorkManager strategy;
* main integration risks;
* recommended implementation order;
* whether integration can start immediately.

Create:

| Area           | Current PDA state | Backend capability | Required PDA work | Readiness |
| -------------- | ----------------- | ------------------ | ----------------- | --------- |
| Network        |                   |                    |                   |           |
| Authentication |                   |                    |                   |           |
| Dashboard      |                   |                    |                   |           |
| Tasks          |                   |                    |                   |           |
| Receiving      |                   |                    |                   |           |
| Putaway        |                   |                    |                   |           |
| Picking        |                   |                    |                   |           |
| Replenishment  |                   |                    |                   |           |
| Inventory      |                   |                    |                   |           |
| Transfer       |                   |                    |                   |           |
| Cycle count    |                   |                    |                   |           |
| Shipping       |                   |                    |                   |           |
| Command status |                   |                    |                   |           |
| Scanner        |                   |                    |                   |           |
| Room           |                   |                    |                   |           |
| WorkManager    |                   |                    |                   |           |

---

# 9. Verified Backend Connection Information

Create a section with verified values:

```text
Backend Tailscale IP: 100.68.50.41
Gateway port: <verified value>
Protocol: <HTTP or HTTPS>
Base path: /api/pda/v1
Base URL: <complete verified URL>
Health URL: <complete URL>
Readiness URL: <complete URL>
```

Explain where each value was found.

Do not use placeholders in the final generated document when the repository provides the answer.

If the port cannot be verified, mark the document `BLOCKED` and identify exactly which runtime configuration is missing or contradictory.

---

# 10. Tailscale Connectivity Requirements

Document the expected network topology:

```text
Zebra PDA or Android development device
→ Tailscale network
→ 100.68.50.41
→ exposed PDA API Gateway port
→ PDA Backend services
```

Document:

* whether Tailscale must run on the physical Zebra device;
* whether the Android emulator can access the Tailscale IP directly;
* whether the developer laptop must be connected to the same tailnet;
* required Tailscale ACL access;
* required server firewall rules;
* required Docker port exposure;
* whether gateway binds to `0.0.0.0`;
* how to test with `curl`;
* how to test from ADB shell;
* how to test from the physical PDA browser or test screen;
* DNS or MagicDNS option;
* IP-change considerations;
* production recommendation.

Provide commands such as:

```shell
tailscale status
ping 100.68.50.41
curl http://100.68.50.41:<PORT>/healthz
curl http://100.68.50.41:<PORT>/readyz
```

Use the actual verified protocol and paths.

---

# 11. Android Environment Configuration

Define a safe environment configuration strategy.

Recommended options to evaluate:

* Gradle product flavors;
* `buildConfigField`;
* local Gradle properties;
* CI-injected environment variables;
* runtime environment provider;
* managed configuration for enterprise devices.

Do not hardcode:

```text
100.68.50.41
```

inside repositories, ViewModels, Composables, or DTOs.

Define at least:

```text
PDA_API_SCHEME
PDA_API_HOST
PDA_API_PORT
PDA_API_BASE_PATH
PDA_API_CONNECT_TIMEOUT_SECONDS
PDA_API_READ_TIMEOUT_SECONDS
PDA_API_WRITE_TIMEOUT_SECONDS
PDA_API_CALL_TIMEOUT_SECONDS
PDA_API_DEBUG_LOGGING
PDA_API_ALLOW_CLEARTEXT
```

Provide the resolved development example:

```properties
PDA_API_SCHEME=http
PDA_API_HOST=100.68.50.41
PDA_API_PORT=<verified-port>
PDA_API_BASE_PATH=/api/pda/v1
```

If HTTPS is verified, use HTTPS and document certificate handling.

Define recommended environments:

```text
local
tailscale-dev
staging
production
```

Create a table:

| Environment | Scheme | Host | Port | Base path | Cleartext | Certificate policy |
| ----------- | ------ | ---: | ---: | --------- | --------: | ------------------ |

---

# 12. Android Manifest and Network Security

Inspect and document required changes.

At minimum evaluate:

```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
```

If development uses HTTP over Tailscale, document a development-only network security configuration.

Do not globally enable cleartext traffic for production without an approved reason.

Recommended pattern:

```text
debug/tailscale-dev:
- allow cleartext only for the approved Tailscale host/IP

release/production:
- require HTTPS
```

Document:

* `networkSecurityConfig`;
* manifest placeholders;
* debug versus release manifests;
* certificate trust;
* hostname verification;
* direct-IP certificate limitations;
* whether MagicDNS is required for HTTPS.

---

# 13. Recommended Android Networking Stack

Inspect current dependencies before making a recommendation.

Recommend one production stack compatible with the current project, such as:

```text
Retrofit
OkHttp
kotlinx.serialization
```

or an approved Ktor client stack.

The guide must define:

```text
data/remote/
  api/
  dto/
  mapper/
  interceptor/
  authenticator/
  error/
  environment/
```

Recommended flow:

```text
Compose
→ ViewModel
→ Use Case
→ Repository
   ├─ Room local data source
   ├─ Remote API data source
   └─ Sync coordinator
→ HTTP client
→ PDA Backend
```

Do not let:

* Compose consume raw API DTOs;
* ViewModels build HTTP requests directly;
* Room entities become API DTOs;
* raw Retrofit/OkHttp exceptions reach UI;
* repositories contain hardcoded base URLs.

---

# 14. HTTP Client Configuration

Document:

* base URL creation;
* connect timeout;
* read timeout;
* write timeout;
* call timeout;
* retry policy;
* connection pooling;
* JSON configuration;
* unknown-field behavior;
* enum handling;
* date/time parsing;
* request/response logging;
* redaction;
* TLS behavior;
* certificate pinning decision.

Logging must redact:

* passwords;
* access tokens;
* refresh tokens;
* authorization headers;
* sensitive barcode values;
* private operator information.

---

# 15. Common Headers

Define an interceptor and request-context strategy for:

```http
Authorization: Bearer <access-token>
X-Correlation-Id: <uuid>
X-Device-Id: <device-id>
X-Warehouse-Id: <warehouse-id>
Idempotency-Key: <uuid>
If-Match: "<version>"
Accept-Language: vi-VN
Content-Type: application/json
```

For every header specify:

* which requests require it;
* who generates it;
* where it is stored;
* whether it survives retry;
* whether it survives process death;
* whether it is authoritative;
* error handling.

Rules:

* correlation ID represents one logical request;
* idempotency key represents one logical mutation;
* retry must preserve the same idempotency key;
* version conflict must not be automatically retried;
* operator identity must come from the access token;
* warehouse and device headers must match the authenticated session.

---

# 16. Authentication Integration

Document the full PDA authentication flow.

## Login

```http
POST /api/pda/v1/auth/login
```

Map exact request and response fields from the current backend OpenAPI.

Document:

* username/password;
* device ID;
* device model;
* app version;
* warehouse selection;
* locale;
* access token;
* refresh token;
* expiry;
* operator;
* roles;
* permissions;
* warehouse;
* device status;
* scanner policy;
* feature flags.

## Secure Storage

Document the approved Android secure storage mechanism.

Do not store secrets in:

* DataStore plaintext;
* SharedPreferences plaintext;
* Room plaintext;
* logs;
* saved-state handles;
* UI state snapshots.

Use Android Keystore-backed encrypted storage or the approved project abstraction.

## Refresh

Document:

```text
401
→ pause new authenticated requests
→ perform one single-flight refresh
→ update secure session
→ replay eligible request once
→ logout when refresh fails
```

Prevent multiple simultaneous refresh calls.

Do not automatically replay unsafe mutations unless their idempotency behavior permits it.

## Logout

Document:

* server revoke request;
* local secure token cleanup;
* Room session-scoped cleanup;
* scanner-context cleanup;
* WorkManager cancellation or safe retention;
* navigation reset;
* behavior when server revoke fails.

---

# 17. Common Response and Error Mapping

Use the actual backend envelope.

Document exact DTOs for:

```text
Envelope<T>
Meta
APIError
```

Map:

* `serverTime`;
* `correlationId`;
* `version`;
* `nextCursor`;
* `hasMore`;
* `asOf`;
* `stale`;
* `errors`.

Create a typed error mapper for all backend stable codes.

At minimum cover:

```text
AUTH_INVALID_CREDENTIALS
AUTH_SESSION_EXPIRED
AUTH_TOKEN_INVALID
AUTH_TOKEN_REVOKED
DEVICE_NOT_REGISTERED
DEVICE_DISABLED
WAREHOUSE_ACCESS_DENIED
PERMISSION_DENIED
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

For every code specify:

* domain error type;
* retryability;
* UI presentation;
* scanner behavior;
* Room draft behavior;
* navigation behavior;
* command-status behavior.

---

# 18. Complete API Mapping

Create one implementation card for every supported API from API-001 through API-028.

For every API include:

```markdown
## API-XXX — Name

### Backend method and path

### PDA screens and ViewModels

### Current repository method

### Request headers

### Path parameters

### Query parameters

### Request DTO

### Response DTO

### Domain mapper

### Room entities affected

### Cache behavior

### Refresh behavior

### Idempotency

### Version handling

### Offline eligibility

### Retry behavior

### Command-status behavior

### Error mapping

### Scanner behavior

### Tests

### Implementation files to create or modify
```

Do not group APIs so broadly that request or response differences are lost.

---

# 19. Required API Groups

At minimum document:

## Authentication and Context

* API-001 login;
* API-002 refresh;
* API-003 logout;
* API-004 bootstrap;
* API-006 current profile;
* warehouses;
* device registration.

## Dashboard and Tasks

* API-005 dashboard;
* API-007 task list;
* task detail;
* claim;
* release.

## Receiving

* API-008 list/detail;
* API-009 barcode resolution;
* API-010 confirmation;
* completion where implemented;
* command status.

## Putaway

* API-011 list/detail;
* API-012 source/item validation;
* API-013 destination validation/suggestions;
* API-014 confirmation.

## Picking

* API-015 list/detail;
* API-016 location validation;
* API-017 item resolution;
* API-018 pick;
* short-pick where supported;
* completion.

## Replenishment

* API-019 list/detail;
* API-020 validation;
* API-021 confirmation.

## Inventory and Transfer

* API-022 inventory search/balances/movements;
* API-023 transfer validation/confirmation.

## Command Status

* API-024 command status.

## Cycle Count

* API-025 list/detail;
* location validation;
* item validation;
* line submission;
* recount;
* completion.

## Shipping

* API-026 shipment/readiness;
* API-027 package verification;
* API-028 confirmation.

---

# 20. Scanner Integration

Map every `ScanPurpose` to its backend API.

Create:

| Scan purpose | Screen | Backend API | Request fields | Success next step | Recoverable errors | Blocking errors |
| ------------ | ------ | ----------- | -------------- | ----------------- | ------------------ | --------------- |

Cover:

```text
RECEIVING_ITEM
PUTAWAY_SOURCE
PUTAWAY_ITEM
PUTAWAY_DESTINATION
PICKING_LOCATION
PICKING_ITEM
REPLENISHMENT_SOURCE
REPLENISHMENT_ITEM
REPLENISHMENT_DESTINATION
INVENTORY_LOOKUP
TRANSFER_SOURCE
TRANSFER_ITEM
TRANSFER_DESTINATION
CYCLE_COUNT_LOCATION
CYCLE_COUNT_ITEM
SHIPPING_PACKAGE
```

Document transport fields:

```text
rawValue
normalizedValue
symbology
scanContext
scannedAt
taskId
lineId
```

Rules:

* preserve leading zeroes;
* backend must not depend on Android Intent extra names;
* scanner remains active after recoverable validation error;
* scanner suspends while mutation submission is in progress;
* duplicate scan guards must not replace backend idempotency;
* token refresh must not lose active scanner context.

---

# 21. Room Integration

Document how backend responses map into Room.

For each domain specify:

* remote DTO;
* domain model;
* Room entity;
* DAO;
* transaction;
* freshness timestamp;
* version;
* stale state;
* pending command state;
* reconciliation behavior.

Do not immediately delete Room.

Use it for:

* offline reads;
* cached task state;
* drafts;
* pending commands;
* command results;
* stale-state UI;
* process-death recovery.

Define the source-of-truth strategy for:

```text
remote server truth
Room cached projection
local draft
pending mutation
terminal command result
```

---

# 22. Repository Migration

Map every current local repository method to the remote implementation.

Create:

| Current repository method | Current mock/local behavior | Remote API | Local persistence | Replacement strategy |
| ------------------------- | --------------------------- | ---------- | ----------------- | -------------------- |

Define how to replace:

```text
MockAuthRepository
MockWarehouseRepository
MockWarehouseDatabase
```

Recommended migration:

```text
Repository interface
├─ Remote API data source
├─ Room local data source
└─ synchronization coordinator
```

Do not remove mock implementations before:

* remote contract tests pass;
* fake-server tests pass;
* Room reconciliation tests pass;
* feature integration tests pass.

Mocks may remain test-only but must not be selectable in production builds.

---

# 23. WorkManager and Offline Commands

Document the final worker design.

For every mutation classify:

| Mutation | Draft allowed | Queue allowed | Online-only | Automatic retry | Command status required |
| -------- | ------------: | ------------: | ----------: | --------------: | ----------------------: |

Document:

* network constraints;
* command serialization;
* command ID;
* idempotency key;
* base version;
* retry count;
* backoff;
* terminal error;
* conflict state;
* process death;
* app restart;
* command-status query.

Rules:

* never generate a new idempotency key during retry;
* do not automatically retry version conflicts;
* transfer and final shipment remain online-only unless backend policy explicitly allows queueing;
* timeout does not mean failure;
* query command status before repeating an ambiguous mutation.

---

# 24. Pagination, Search, and Freshness

Document the backend list convention:

```text
cursor
limit
status
category/type
q
priority
zone
dateFrom
dateTo
sort
direction
```

Document:

* cursor storage;
* cursor/filter binding;
* stable ordering;
* Room paging strategy;
* refresh;
* append;
* invalid cursor;
* empty page;
* `nextCursor`;
* `hasMore`;
* `asOf`;
* `stale`.

Explain whether Paging 3 should be introduced or whether the current repository pagination abstraction is sufficient.

---

# 25. Refresh and Invalidation

Create:

| Mutation | Backend response | Room entities refreshed | Screens refreshed | Command-status follow-up |
| -------- | ---------------- | ----------------------- | ----------------- | ------------------------ |

Cover:

* receiving;
* putaway;
* picking;
* replenishment;
* transfer;
* cycle count;
* shipping;
* task claim/release.

Specify whether to:

* update from mutation response;
* fetch detail;
* refresh list;
* refresh dashboard;
* refresh inventory balance;
* refresh shipment readiness.

Avoid unnecessary full-database replacement after every mutation.

---

# 26. Environment and Dependency Injection

Define how environment configuration enters the application.

Recommended:

```text
BuildConfig
→ NetworkEnvironment
→ HTTP client factory
→ API services
→ remote data sources
→ repositories
→ ViewModels
```

Document integration with the current dependency-injection approach.

Do not read environment values directly in:

* Composables;
* ViewModels;
* repository methods;
* DTOs.

Provide sample development configuration using:

```text
100.68.50.41
```

and the verified port.

---

# 27. Required Test Plan

## Unit Tests

* environment parsing;
* base URL construction;
* header interceptors;
* token storage;
* token refresh coordinator;
* DTO mappers;
* error mapper;
* scanner request builders;
* idempotency reuse;
* version conflict;
* redaction.

## Fake HTTP Server Tests

* API-001 through API-028;
* success envelope;
* error envelope;
* malformed JSON;
* timeout;
* 401 refresh;
* concurrent 401;
* 403;
* 404;
* 409;
* 422;
* 429;
* 500;
* 503;
* pagination;
* stale metadata;
* duplicate command;
* command status.

## Room and Synchronization Tests

* remote-to-Room mapping;
* stale cache;
* pending command persistence;
* process death;
* app restart;
* refresh after mutation;
* command-status reconciliation;
* conflict retention.

## Integration Tests

* real backend through Tailscale;
* login;
* bootstrap;
* dashboard;
* one complete workflow per domain;
* token expiry;
* refresh;
* logout;
* wrong device;
* wrong warehouse;
* permission denied;
* backend restart;
* network interruption.

## Physical Zebra E2E

* Tailscale connectivity;
* DataWedge;
* hardware trigger;
* leading zeroes;
* symbology;
* duplicate scan;
* token refresh during scan workflow;
* reconnect;
* no duplicate write.

---

# 28. Integration Implementation Phases

The document must propose an Android implementation sequence.

Recommended:

```text
PDA Integration Phase 00 — Environment, network permission and Tailscale connectivity
PDA Integration Phase 01 — HTTP client, envelopes, errors and interceptors
PDA Integration Phase 02 — Secure authentication and bootstrap
PDA Integration Phase 03 — Dashboard, profile and tasks
PDA Integration Phase 04 — Receiving
PDA Integration Phase 05 — Putaway
PDA Integration Phase 06 — Picking
PDA Integration Phase 07 — Replenishment
PDA Integration Phase 08 — Inventory inquiry and transfer
PDA Integration Phase 09 — Cycle count
PDA Integration Phase 10 — Shipping
PDA Integration Phase 11 — WorkManager, offline queue and command status
PDA Integration Phase 12 — Full Tailscale, backend and Zebra E2E
PDA Integration Phase 13 — Remove production mock selection and release verification
```

For every phase provide:

* objective;
* files to inspect;
* files to create;
* APIs;
* Room changes;
* tests;
* exit criteria;
* rollback considerations.

---

# 29. Mock Removal Rules

The final document must define when production mock behavior is removed.

Production builds must not select:

```text
MockAuthRepository
MockWarehouseRepository
MockWarehouseDatabase
```

after real integration approval.

Test-only fakes may remain.

The release build must fail or refuse startup when:

* base URL is missing;
* environment is invalid;
* production uses a mock repository;
* secure token store is unavailable;
* required network configuration is missing.

---

# 30. Required Deliverables Inside the Document

Include:

## Appendix A — Verified Backend Connection Values

## Appendix B — Environment Variable Catalog

## Appendix C — API-001 Through API-028 Map

## Appendix D — Request DTO Catalog

## Appendix E — Response DTO Catalog

## Appendix F — Error Code Mapping

## Appendix G — Scanner-to-API Map

## Appendix H — Repository Method-to-API Map

## Appendix I — Room Entity Reconciliation Map

## Appendix J — WorkManager Command Matrix

## Appendix K — Refresh and Invalidation Matrix

## Appendix L — Tailscale Connectivity Checklist

## Appendix M — Android Integration Phase Plan

## Appendix N — Full E2E Test Checklist

---

# 31. Final Validation Checklist

Before finishing, verify that the document answers:

* [ ] Was the final backend source inspected?
* [ ] Was the final OpenAPI inspected?
* [ ] Were all phase reports inspected?
* [ ] Was the gateway port verified?
* [ ] Was HTTP versus HTTPS verified?
* [ ] Was the complete base URL documented?
* [ ] Was Tailscale IP `100.68.50.41` documented?
* [ ] Were health and readiness URLs documented?
* [ ] Were Android network permissions documented?
* [ ] Was cleartext-versus-HTTPS behavior documented?
* [ ] Were environment variables defined?
* [ ] Was dependency injection defined?
* [ ] Was the HTTP client stack defined?
* [ ] Were authentication and secure token storage defined?
* [ ] Was single-flight refresh defined?
* [ ] Were headers defined?
* [ ] Were all 28 APIs mapped?
* [ ] Were exact request DTOs documented?
* [ ] Were exact response DTOs documented?
* [ ] Were errors mapped to UI/scanner behavior?
* [ ] Were all scanner purposes mapped?
* [ ] Was Room reconciliation documented?
* [ ] Was WorkManager behavior documented?
* [ ] Was command status documented?
* [ ] Were idempotency and version rules documented?
* [ ] Were pagination and freshness documented?
* [ ] Were refresh and invalidation documented?
* [ ] Was the mock-removal strategy documented?
* [ ] Was the full Tailscale integration test plan included?
* [ ] Was physical Zebra E2E included?
* [ ] Can another AI agent implement the complete PDA App integration from this document alone?

---

# 32. Execution Instruction

Proceed directly with the PDA Backend repository inspection.

First inspect:

```text
pda-backend/docs/integration-pda-app/PDA_APP_API_SPECIFICATION.md
pda-backend/PDA_BACKEND_PDA_APP_INTEGRATION_GAP_REPORT.md
pda-backend/api/openapi/pda-v1.yaml
pda-backend/docs/integration-pda-app/
pda-backend/requirement-report/
pda-backend/docker-compose*.yml
pda-backend/Makefile
pda-backend/internal/gateway/
pda-backend/internal/platform/config/
```

Determine the actual gateway port, protocol, base path, and runtime requirements.

Use:

```text
100.68.50.41
```

as the current Tailscale backend server address.

Then create:

```text
PDA_APP_BACKEND_INTEGRATION_IMPLEMENTATION_GUIDE.md
```

and:

```text
requirement-report/YYYY-MM-DD-pda-app-backend-integration-document.md
```

Do not return only an outline.

Do not leave the port unverified when the repository contains the answer.

Do not invent API contracts.

Do not modify the Android App or backend production code during this documentation task.

After completing the document, report:

1. backend documents inspected;
2. backend source and configuration inspected;
3. verified gateway IP;
4. verified gateway port;
5. verified protocol;
6. verified base URL;
7. number of APIs mapped;
8. number of screens mapped;
9. number of scanner purposes mapped;
10. environment variables defined;
11. Android permissions required;
12. integration phases proposed;
13. unresolved infrastructure requirements;
14. files created;
15. final integration-document readiness status.
