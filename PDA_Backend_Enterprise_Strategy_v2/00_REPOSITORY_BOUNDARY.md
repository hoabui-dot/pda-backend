# PDA Backend — Repository Boundary and Non-Negotiable Scope

- **Document ID:** `PDA-BE-BOUNDARY-001`
- **Version:** 2.0
- **Status:** Mandatory
- **Purpose:** prevent any AI agent from confusing the existing Android PDA application with the new backend repository.

---

## 1. Two Separate Repositories

The project consists of two independent codebases.

```text
PDA_APP
Existing Kotlin Android application
Package: com.example.pda_app
Status: already exists
Responsibility: mobile UI, Zebra DataWedge scanning, local mobile state, API client

PDA_BACKEND
New backend repository
Status: does not exist before PRE-00
Responsibility: REST APIs, authentication, business workflows, persistence,
Redis cache, messaging adapters, Kafka integration, WMS integration
```

The backend repository must not contain or recreate the Android application.

---

## 2. System Relationship

```text
Existing Zebra PDA Android App
        |
        | HTTPS REST API
        | OAuth2/OIDC bearer token
        | JSON requests/responses
        v
New PDA Backend API Gateway
        |
        +--> Identity and Access
        +--> PDA Execution
        +--> Inventory
        +--> Shipping
        +--> Redis
        +--> PostgreSQL
        +--> Mock Event Publisher initially
        +--> Kafka later
        +--> Upstream WMS later
```

The integration boundary between the existing PDA App and the new backend is the public API contract.

---

## 3. Strictly Forbidden Actions

An AI agent working on the backend must not:

- create a new Android project;
- create Compose screens;
- modify `com.example.pda_app`;
- copy the PDA source code into the backend repository;
- run Android Gradle tasks;
- require Android Studio, Android SDK, ADB, emulator, or Zebra device for backend phases;
- treat missing Android files as a backend error;
- search for an existing backend build before repository bootstrap;
- claim that the backend repository already exists before PRE-00 creates it;
- use the PDA mock database as the backend database;
- directly connect the PDA App to PostgreSQL, Redis, Kafka, or the WMS database.

---

## 4. Correct PRE-00 Interpretation

Before PRE-00:

```text
PDA_BACKEND repository: not created
Backend Go workspace/modules: not created
Backend modules: not created
Backend build: not available
```

Therefore, the correct first action is:

```text
Create a new backend repository
→ initialize Go workspace and modules
→ scaffold backend modules
→ add source files
→ run the first backend build
```

The following statement is incorrect for PRE-00:

```text
Baseline verification found no existing build to run.
```

There is intentionally no backend baseline before bootstrap.

The correct statement is:

```text
The backend repository does not yet exist. PRE-00 will create it.
No pre-existing backend build is expected.
```

---

## 5. Environment Interpretation

The backend requires the pinned Go toolchain declared in `go.mod` after bootstrap.

If the host has no Go installation but Docker is available, the AI may:

- create the backend repository and Go workspace/modules;
- run backend builds in a pinned Go container;
- record this as an execution-environment choice.

This does not indicate a code failure and does not authorize the AI to scaffold an Android application.

Acceptable:

```text
The new backend repository was created and verified in a pinned Go container
because the host does not provide Go.
```

Unacceptable:

```text
The PDA application has no build, so a backend build cannot be run.
```

---

## 6. Backend Source of Functional Requirements

The backend must map to the approved PDA screens and workflows, but it must not implement those screens.

Backend requirements come from:

- the approved 16-screen PDA specification;
- `NEW_AI_CONTEXT.md`;
- the PDA gap analysis;
- the PDA phased implementation strategy;
- the backend API and event contract map.

Examples:

```text
PDA screen: Receiving Confirm Quantity
Backend responsibility:
- validate receiving task and line;
- validate quantity policy;
- enforce idempotency and version;
- commit database mutation;
- return response;
- append outbox event.

Backend does not render the quantity screen.
```

---

## 7. Build and Verification Boundary

Backend verification includes:

- Go backend build;
- Go unit tests;
- service bootstrap and HTTP health tests;
- database migrations;
- REST API tests;
- Redis tests;
- mock messaging tests;
- Kafka tests after Kafka is enabled;
- WMS contract tests after WMS integration exists;
- Docker Compose smoke tests.

Backend verification does not include:

- Android instrumentation tests;
- Compose tests;
- ADB;
- Zebra DataWedge verification;
- APK installation.

Cross-system E2E with the PDA may be performed only in later integration phases, using the already-existing Android repository as an external client.

---

## 8. Required Agent Confirmation

At the beginning of every backend phase report, include:

```text
Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified in this phase.
- PDA backend repository: current working repository.
- Android project creation: not part of this phase.
```

For PRE-00, include:

```text
Backend repository existence before phase: no.
Expected action: create a new backend repository from zero.
```
