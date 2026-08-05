# Master Prompt — Generate the Complete PDA Backend to WMS Kafka Event Integration Specification

## 1. Role

You are working inside the completed Go repository:

```text
PDA_BACKEND
```

Act as a:

* senior enterprise solution architect;
* senior Go backend engineer;
* WMS domain architect;
* Apache Kafka integration architect;
* event-driven architecture specialist;
* warehouse data-model specialist;
* Zebra PDA and barcode/QR workflow specialist;
* data consistency and reconciliation engineer;
* API and event-contract specialist;
* observability and production-readiness engineer;
* technical documentation author.

Your task is to inspect the entire PDA Backend repository and generate one complete, implementation-ready Markdown document describing how the PDA Backend must integrate with the upstream WMS through Kafka events.

The document must cover every PDA Backend feature required by the PDA App, every WMS-owned data dependency, every inbound WMS event, every outbound PDA event, all Kafka topics, keys, schemas, ordering rules, idempotency, replay, reconciliation, scanner and QR-code requirements, failure handling, observability, security, and end-to-end workflow behavior.

This is primarily an inspection, analysis, contract-definition, and documentation task.

Do not modify production code unless explicitly instructed after the document is reviewed.

---

# 2. System Boundary

There are three independent systems:

```text
PDA_APP
Existing Kotlin Android application
Responsibilities:
- UI;
- navigation;
- Zebra DataWedge scanner integration;
- local Room cache;
- local drafts;
- WorkManager synchronization;
- calls only PDA Backend public APIs.

PDA_BACKEND
Current Go backend repository
Responsibilities:
- public PDA REST APIs;
- authentication and authorization;
- operator, device, and warehouse context;
- workflow orchestration;
- backend-owned transaction state;
- PostgreSQL;
- Redis;
- idempotency;
- optimistic concurrency;
- outbox and inbox;
- Kafka producer and consumer;
- audit;
- WMS anti-corruption layer.

UPSTREAM_WMS
External enterprise warehouse-management system
Responsibilities may include:
- warehouse master data;
- items and barcodes;
- units of measure;
- locations and zones;
- inventory ownership and balances;
- inbound documents;
- outbound orders;
- warehouse tasks;
- reservations;
- replenishment demand;
- cycle-count plans;
- shipment and package data;
- business policies;
- final enterprise system-of-record processing.
```

The exact ownership must be derived from the repository and existing documents.

Do not assume that every warehouse entity belongs to WMS.

Clearly distinguish:

* PDA Backend-owned state;
* WMS-owned state;
* locally projected WMS data;
* cached data;
* transient workflow validation data;
* authoritative inventory data;
* audit-only data.

The PDA App must never consume Kafka directly.

Required integration path:

```text
WMS
→ Kafka
→ PDA Backend consumers
→ inbox/idempotency
→ PostgreSQL projections or workflow state
→ Redis invalidation
→ PDA REST APIs
→ PDA App
```

Outbound path:

```text
PDA App command
→ PDA Backend REST API
→ database transaction
→ outbox event
→ Kafka
→ WMS consumer
→ WMS processing
→ acknowledgement/result event
→ PDA Backend projection/reconciliation
→ PDA App refresh or command-status query
```

---

# 3. Mandatory Documents to Inspect

Before writing the integration specification, inspect the complete repository.

At minimum read:

```text
docs/integration-pda-app/PDA_APP_API_SPECIFICATION.md
PDA_BACKEND_PDA_APP_INTEGRATION_GAP_REPORT.md
api/openapi/pda-v1.yaml
docs/integration-pda-app/PDA_BACKEND_RECONCILIATION_RULES_V2.md
docs/integration-pda-app/README_PHASE_ORDER_V2.md
```

Read all completed reconciliation phase documents and reports:

```text
docs/integration-pda-app/PHASE_00_*.md
docs/integration-pda-app/PHASE_01_*.md
docs/integration-pda-app/PHASE_02_*.md
docs/integration-pda-app/PHASE_03_*.md
docs/integration-pda-app/PHASE_04_*.md
docs/integration-pda-app/PHASE_05_*.md
docs/integration-pda-app/PHASE_06_*.md
docs/integration-pda-app/PHASE_07_*.md
docs/integration-pda-app/PHASE_08_*.md
docs/integration-pda-app/PHASE_09_*.md
docs/integration-pda-app/PHASE_10_*.md
docs/integration-pda-app/PHASE_11_*.md
requirement-report/
COMMON-DEFERRED-VERIFICATION.md
```

Also inspect:

* all `cmd/` service entry points;
* gateway routes;
* application services;
* domain models;
* database migrations;
* PostgreSQL repositories;
* Redis adapters;
* Kafka producers;
* Kafka consumers;
* outbox worker;
* inbox handling;
* DLQ handling;
* topic configuration;
* event envelopes;
* WMS ports;
* WMS HTTP adapters;
* mock WMS adapters;
* fixtures;
* configuration;
* Docker Compose;
* Makefile;
* Kafka integration tests;
* contract tests;
* architecture tests;
* observability metrics;
* operational runbooks.

Do not generate the document only from architecture files.

The current source code and final OpenAPI are authoritative for implemented PDA Backend behavior.

---

# 4. Primary Objective

Create one authoritative integration document that answers:

1. What features does the PDA Backend currently support?
2. Which PDA App screens and workflows depend on WMS data?
3. Which data must PDA Backend receive from WMS?
4. Which data must arrive through Kafka events?
5. Which data may be requested synchronously from WMS?
6. Which WMS events must initialize backend projections?
7. Which WMS events must update or cancel existing tasks?
8. Which PDA commands must produce outbound Kafka events?
9. Which outbound events require WMS acknowledgements?
10. Which commands may be considered complete immediately by PDA Backend?
11. Which commands remain pending until WMS confirms them?
12. How are WMS event versions and ordering enforced?
13. How are duplicates handled?
14. How are out-of-order events handled?
15. How are missing events detected?
16. How are projections rebuilt?
17. How are WMS and PDA Backend reconciled?
18. What data is required for barcode, QR, GS1, lot, serial, LPN, location, package, and shipment scanning?
19. Which validations are local and which require authoritative WMS data?
20. Which topics, event names, schemas, keys, partitions, and consumer groups are required?
21. How are Kafka outages handled?
22. How are poison messages and DLQ handled?
23. How are security, ACL, TLS, observability, and lag monitored?
24. What exact contracts must WMS engineers implement?
25. What test data and end-to-end scenarios are required?

---

# 5. Required Output

Create exactly:

```text
PDA_BACKEND_WMS_KAFKA_EVENT_INTEGRATION_SPECIFICATION.md
```

Recommended location:

```text
docs/wms-integration/PDA_BACKEND_WMS_KAFKA_EVENT_INTEGRATION_SPECIFICATION.md
```

Also create:

```text
requirement-report/YYYY-MM-DD-pda-backend-wms-kafka-integration-analysis.md
```

Do not create multiple fragmented specifications.

The single main document must be sufficient for:

* PDA Backend engineers;
* WMS engineers;
* Kafka platform engineers;
* PDA App engineers;
* QA;
* operations;
* security;
* observability teams.

---

# 6. Source-of-Truth Policy

Use the following priority:

1. Current PDA Backend source code.
2. Current database migrations.
3. Current OpenAPI.
4. Current tests.
5. Final implementation reports.
6. `PDA_APP_API_SPECIFICATION.md`.
7. Existing architecture documents.
8. Proposed WMS contracts.

When sources differ:

* record the mismatch;
* use implementation as current backend state;
* use PDA specification as client requirement;
* clearly mark WMS contracts as proposed unless an approved WMS contract already exists;
* do not invent WMS fields without identifying them as required/proposed.

Use these labels consistently:

```text
CURRENTLY_IMPLEMENTED
CURRENTLY_MOCKED
BACKEND_OWNED
WMS_OWNED
SHARED_CONTRACT
PROPOSED_WMS_EVENT
APPROVED_WMS_EVENT
NOT_IMPLEMENTED
NOT_VERIFIED
BLOCKED_BY_WMS_CONTRACT
BLOCKED_BY_EXTERNAL_ENVIRONMENT
DEFERRED
```

---

# 7. Required Document Structure

The generated document must contain every section below.

---

## 7.1 Document Header

Include:

```markdown
# PDA Backend — WMS Kafka Event Integration Specification

- Document status
- Generated date
- PDA Backend repository
- Backend branch and commit
- PDA App contract source
- OpenAPI source
- Current Kafka mode
- Current WMS mode
- Integration scope
- Source-of-truth policy
- Intended readers
```

---

## 7.2 Executive Summary

Summarize:

* current backend architecture;
* current WMS integration state;
* current Kafka integration state;
* number of PDA workflows;
* number of WMS data domains;
* number of required inbound event families;
* number of required outbound event families;
* current mocks or fixtures;
* main integration blockers;
* highest-risk data-consistency issues;
* whether real WMS integration can begin immediately.

Create:

| Domain                    | PDA Backend support | WMS data required | Inbound event required | Outbound event required | Readiness |
| ------------------------- | ------------------- | ----------------- | ---------------------: | ----------------------: | --------- |
| Identity/operator mapping |                     |                   |                        |                         |           |
| Warehouse master          |                     |                   |                        |                         |           |
| Item/barcode master       |                     |                   |                        |                         |           |
| Locations/zones           |                     |                   |                        |                         |           |
| Inventory                 |                     |                   |                        |                         |           |
| Tasks                     |                     |                   |                        |                         |           |
| Receiving                 |                     |                   |                        |                         |           |
| Putaway                   |                     |                   |                        |                         |           |
| Picking                   |                     |                   |                        |                         |           |
| Replenishment             |                     |                   |                        |                         |           |
| Transfer                  |                     |                   |                        |                         |           |
| Cycle count               |                     |                   |                        |                         |           |
| Shipping                  |                     |                   |                        |                         |           |

---

# 8. Complete PDA Backend Capability Inventory

Inspect the entire codebase and list every currently supported capability.

Create:

| Capability ID | Domain | Public API | Application use case | Database state | Events produced | Events consumed | Current WMS dependency | Status |
| ------------- | ------ | ---------- | -------------------- | -------------- | --------------- | --------------- | ---------------------- | ------ |

At minimum include:

## Authentication and Context

* login;
* refresh;
* logout;
* bootstrap;
* profile;
* warehouses;
* device registration;
* permissions;
* warehouse scope.

## Dashboard and Tasks

* dashboard summary;
* task summary;
* task listing;
* task detail;
* claim;
* release;
* task locking;
* task versions.

## Receiving

* task list;
* detail;
* barcode resolution;
* receipt confirmation;
* completion;
* command status.

## Putaway

* task list;
* detail;
* source validation;
* item validation;
* destination suggestion;
* destination validation;
* movement confirmation.

## Picking

* order/task list;
* detail;
* location validation;
* item resolution;
* pick confirmation;
* short-pick;
* completion.

## Replenishment

* task list;
* detail;
* source/item/destination validation;
* partial confirmation;
* completion.

## Inventory

* item search;
* balances;
* movement history;
* lot/serial/LPN inquiry where implemented.

## Transfer

* source validation;
* destination validation;
* item validation;
* confirmation.

## Cycle Count

* task list;
* detail;
* location validation;
* item validation;
* blind count;
* submission;
* variance;
* recount;
* completion.

## Shipping

* shipment summary;
* detail;
* readiness;
* package verification;
* carrier/tracking validation;
* final confirmation;
* command status.

Do not claim support from documentation alone.

Include implementation evidence.

---

# 9. WMS Data Ownership Matrix

For every important entity, determine ownership.

Create:

| Entity/data | PDA Backend ownership | WMS ownership | PDA App usage | Kafka source | Local projection | Reconciliation required |
| ----------- | --------------------- | ------------- | ------------- | ------------ | ---------------- | ----------------------- |

At minimum include:

* warehouse;
* zone;
* location/bin;
* location type;
* capacity;
* item;
* SKU;
* GTIN;
* barcode alias;
* QR identifier;
* unit of measure;
* conversion rules;
* lot;
* serial number;
* LPN;
* pallet;
* supplier;
* customer;
* purchase order;
* inbound order;
* receiving task;
* putaway task;
* picking order;
* picking task;
* replenishment task;
* inventory balance;
* inventory reservation;
* transfer policy;
* cycle-count task;
* shipment;
* package;
* carrier;
* tracking number;
* operator mapping;
* task assignment;
* task status;
* workflow policy.

Clearly state what PDA Backend may modify locally and what must only be projected from WMS.

---

# 10. Required WMS Master Data

Document every master-data feed required by PDA Backend.

For each domain create:

```markdown
## Master Data Domain

### Business purpose

### Required fields

### Event source

### Initial snapshot requirement

### Incremental event requirement

### Delete/deactivate behavior

### Versioning

### Projection table

### Cache behavior

### PDA APIs affected

### Scanner flows affected

### Reconciliation
```

Required domains include:

## Warehouse

Fields may include:

```text
warehouseId
warehouseCode
warehouseName
status
timezone
locale
```

## Zones and Locations

Fields may include:

```text
locationId
locationCode
warehouseId
zoneId
locationType
status
capacity
capacityUnit
allowedItemClasses
temperatureClass
hazardClass
pickSequence
putawayPriority
version
```

## Item Master

Fields may include:

```text
itemId
itemCode
sku
description
baseUom
supportedUoms
conversionRules
status
lotControlled
serialControlled
expiryControlled
conditionControlled
weight
dimensions
itemClass
hazardClass
temperatureClass
version
```

## Barcode and QR Aliases

Fields may include:

```text
barcode
normalizedBarcode
barcodeType
symbology
itemId
uom
quantityPerBarcode
lotEncoded
serialEncoded
expiryEncoded
supplierBarcode
customerBarcode
status
validFrom
validTo
version
```

## LPN and Container Master

Only include if current backend/PDA contract requires it.

Do not enable deferred LPN or pallet workflows without approved support.

---

# 11. Enterprise Barcode and QR-Code Integration

Create a dedicated enterprise scanning section.

The backend must treat scan input as an identifier that requires contextual resolution.

Document support for:

* Code 128;
* Code 39 where required;
* EAN-13;
* EAN-8;
* UPC-A;
* UPC-E;
* ITF-14;
* GS1-128;
* GS1 DataMatrix;
* GS1 QR Code;
* standard QR Code;
* Data Matrix;
* location barcode;
* item barcode;
* lot barcode;
* serial barcode;
* LPN barcode;
* pallet barcode;
* package barcode;
* shipment/tracking barcode.

Do not claim a symbology is supported unless current scanner and backend contracts support it.

## 11.1 Required Scan Request

Document the canonical PDA-to-backend scan request:

```json
{
  "rawValue": "01000123456789051726010110LOT12321SERIAL001",
  "normalizedValue": "00012345678905",
  "symbology": "GS1_DATAMATRIX",
  "scanContext": "RECEIVING_ITEM",
  "scannedAt": "2026-08-02T12:00:00Z",
  "taskId": "REC-001",
  "lineId": null
}
```

Operator, device, warehouse, and correlation context must come from authenticated headers/context.

## 11.2 GS1 Parsing

Document whether parsing occurs:

* on PDA;
* on backend;
* in both places;
* in a shared library;
* through a WMS barcode service.

Prefer server-authoritative interpretation for business-critical identifiers.

For GS1 data, evaluate application identifiers such as:

```text
01 — GTIN
10 — batch/lot
17 — expiry date
21 — serial number
00 — SSCC
30 — variable count
37 — count of trade items
```

Do not hardcode only these identifiers when the business requires more.

Document:

* FNC1 handling;
* group separators;
* leading zero preservation;
* check digit validation;
* lot and serial extraction;
* expiry parsing;
* variable quantity;
* UOM conversion;
* unknown application identifiers;
* malformed payload behavior.

## 11.3 Scan Resolution Response

Required response capabilities:

```json
{
  "data": {
    "resolvedEntityType": "ITEM",
    "itemId": "ITEM-001",
    "itemCode": "SKU-001",
    "displayCode": "00012345678905",
    "uom": "EA",
    "embeddedQuantity": 1,
    "lotNumber": "LOT123",
    "serialNumber": "SERIAL001",
    "expiryDate": "2026-01-01",
    "validationStatus": "ACCEPTED",
    "nextStep": "ENTER_QUANTITY",
    "quantityConstraints": {
      "minimum": 1,
      "maximum": 10
    },
    "taskVersion": 11
  },
  "meta": {},
  "errors": []
}
```

Document which fields depend on WMS master data.

## 11.4 Scan Security and Integrity

Document:

* maximum barcode length;
* character allowlist where applicable;
* control-character handling;
* normalization;
* injection-safe logging;
* sensitive barcode redaction;
* duplicate-scan suppression;
* replay behavior;
* task-context validation;
* cross-warehouse rejection;
* unknown barcode behavior;
* deactivated alias behavior.

---

# 12. Required Inbound WMS Event Catalog

List every event PDA Backend must consume from WMS.

Create:

| Event ID | Event name | Topic | Producer | Key | Purpose | Projection affected | PDA APIs affected | Required |
| -------- | ---------- | ----- | -------- | --- | ------- | ------------------- | ----------------- | -------: |

At minimum evaluate the following event families.

## Master Data Events

```text
WarehouseCreated
WarehouseUpdated
WarehouseDeactivated

ZoneCreated
ZoneUpdated
ZoneDeactivated

LocationCreated
LocationUpdated
LocationBlocked
LocationUnblocked
LocationDeactivated

ItemCreated
ItemUpdated
ItemDeactivated

ItemBarcodeAssigned
ItemBarcodeUpdated
ItemBarcodeRemoved

UnitOfMeasureUpdated
ItemHandlingPolicyUpdated
```

## Inventory Events

```text
InventoryBalanceSnapshot
InventoryBalanceChanged
InventoryReserved
InventoryReservationReleased
InventoryHoldChanged
InventoryLotChanged
InventorySerialChanged
InventoryOwnershipChanged
```

## Task Events

```text
TaskCreated
TaskAssigned
TaskUpdated
TaskPriorityChanged
TaskCancelled
TaskSuspended
TaskReleased
TaskCompletedExternally
```

## Receiving Events

```text
InboundOrderCreated
InboundOrderUpdated
InboundOrderCancelled
ReceivingTaskCreated
ReceivingTaskUpdated
ReceivingTaskCancelled
ReceivingLineUpdated
ReceivingPolicyUpdated
```

## Putaway Events

```text
PutawayTaskCreated
PutawayTaskUpdated
PutawayTaskCancelled
PutawayDestinationPolicyUpdated
```

## Picking Events

```text
OutboundOrderCreated
OutboundOrderUpdated
OutboundOrderCancelled
PickingTaskCreated
PickingTaskUpdated
PickingTaskCancelled
PickingLineUpdated
ReservationChanged
```

## Replenishment Events

```text
ReplenishmentTaskCreated
ReplenishmentTaskUpdated
ReplenishmentTaskCancelled
ReplenishmentDemandChanged
```

## Cycle Count Events

```text
CycleCountTaskCreated
CycleCountTaskUpdated
CycleCountTaskCancelled
CycleCountReviewRequested
CycleCountApproved
CycleCountRejected
```

## Shipping Events

```text
ShipmentCreated
ShipmentUpdated
ShipmentCancelled
PackageCreated
PackageUpdated
PackageCancelled
ShipmentReadinessChanged
CarrierAssignmentChanged
TrackingNumberAssigned
ShipmentConfirmedByWms
```

Only retain events that are supported by actual requirements.

Clearly mark proposed events.

---

# 13. Required Event Contract Template

For every inbound and outbound event use this structure:

```markdown
## Event — EventName

### Direction

WMS → PDA Backend  
or  
PDA Backend → WMS

### Business purpose

### Topic

### Partition key

### Producer

### Consumer group

### Trigger

### Preconditions

### Event envelope

### Payload schema

### Field descriptions

### Ownership

### Versioning

### Ordering requirements

### Duplicate behavior

### Out-of-order behavior

### Missing-event behavior

### Retry behavior

### DLQ behavior

### Projection/database effect

### Cache invalidation

### PDA APIs/screens affected

### Audit requirements

### Metrics and alerts

### Example JSON

### Contract tests

### Reconciliation behavior
```

---

# 14. Canonical Kafka Event Envelope

Reconcile with the backend's current envelope.

Document a canonical structure such as:

```json
{
  "eventId": "uuid",
  "eventType": "ReceivingTaskCreated",
  "eventVersion": 1,
  "occurredAt": "2026-08-02T12:00:00Z",
  "publishedAt": "2026-08-02T12:00:01Z",
  "producer": "wms",
  "aggregateType": "RECEIVING_TASK",
  "aggregateId": "REC-001",
  "aggregateVersion": 7,
  "warehouseId": "MAIN",
  "correlationId": "uuid",
  "causationId": "uuid",
  "traceId": "trace-id",
  "schemaId": "wms.receiving-task-created.v1",
  "payload": {}
}
```

Evaluate required metadata:

* event ID;
* type;
* version;
* schema ID;
* aggregate ID;
* aggregate version;
* warehouse;
* correlation;
* causation;
* trace;
* producer;
* occurred time;
* published time;
* tenant/site when applicable;
* replay marker;
* source system;
* actor/service identity.

Do not allow application code to depend directly on Kafka client-specific record structures.

---

# 15. Topic Design

Inspect current topics and propose the final topic model.

Current backend topics may include:

```text
pda.task.events.v1
pda.receiving.events.v1
pda.movement.events.v1
pda.inventory.events.v1
pda.shipping.events.v1
```

Determine whether WMS-owned topics should be separate, for example:

```text
wms.master-data.events.v1
wms.task.events.v1
wms.receiving.events.v1
wms.inventory.events.v1
wms.outbound.events.v1
wms.shipping.events.v1
```

For every topic document:

| Topic | Owner | Producers | Consumers | Key | Partitions | Retention | Compaction | Schema policy |
| ----- | ----- | --------- | --------- | --- | ---------: | --------- | ---------: | ------------- |

Evaluate:

* domain-based topics;
* event-type topics;
* compacted master-data topics;
* snapshot topics;
* transactional event topics;
* replay topics;
* DLQ topics.

Avoid one universal topic containing unrelated warehouse domains unless clearly justified.

---

# 16. Partition Key and Ordering

For every event define a stable key.

Recommended examples:

```text
warehouse master → warehouseId
item master → itemId
location master → locationId
inventory balance → warehouseId + itemId + locationId + lot + serial
task → taskId
receiving → receivingTaskId
picking → pickingTaskId or orderId
shipment → shipmentId
package → shipmentId
```

Document:

* required ordering scope;
* partitioning implications;
* hot-key risk;
* aggregate version checks;
* sequence numbers;
* cross-aggregate ordering limitations.

Do not promise global ordering.

PDA Backend must reject or quarantine stale aggregate versions according to the contract.

---

# 17. Inbox, Idempotency, and Duplicate Handling

Inspect current inbox implementation.

Document the required consumer transaction:

```text
receive Kafka record
→ validate envelope/schema
→ begin PostgreSQL transaction
→ insert event ID into inbox
→ if duplicate: commit successful no-op
→ validate aggregate version
→ update projection/domain state
→ write audit/reconciliation metadata
→ commit
→ invalidate cache
→ commit Kafka offset
```

Document:

* unique inbox constraints;
* event ID retention;
* payload hash where required;
* duplicate event with different payload behavior;
* poison event behavior;
* retry count;
* offset-commit policy;
* consumer restart behavior.

---

# 18. Out-of-Order and Missing Events

For every event family specify:

```text
expectedVersion
receivedVersion
currentProjectionVersion
```

Define behavior for:

* duplicate version;
* next version;
* stale version;
* future version gap;
* event received before create;
* delete before create;
* cancellation after completion;
* snapshot after incremental events.

Possible actions:

```text
APPLY
IGNORE_DUPLICATE
IGNORE_STALE
PARK_FOR_RETRY
REQUEST_REPLAY
REQUEST_SNAPSHOT
SEND_TO_DLQ
CREATE_RECONCILIATION_ALERT
```

Do not silently overwrite newer state.

---

# 19. Initial Synchronization and Bootstrap

Kafka incremental events alone may not be sufficient for a new environment.

Document required initialization:

* full warehouse snapshot;
* item master snapshot;
* barcode alias snapshot;
* location snapshot;
* inventory snapshot;
* active task snapshot;
* active inbound documents;
* active outbound documents;
* open cycle-count tasks;
* open shipments.

Define:

```text
snapshot ID
snapshot version
snapshot started time
snapshot completed time
high-water mark
incremental event offset
```

Document how PDA Backend avoids gaps between snapshot and live events.

---

# 20. Outbound PDA Backend Event Catalog

List every event the PDA Backend produces for WMS.

Create:

| Event | Triggering PDA API | Database transaction | WMS action required | Acknowledgement required | PDA command status impact |
| ----- | ------------------ | -------------------- | ------------------- | -----------------------: | ------------------------- |

At minimum evaluate:

```text
TaskClaimed
TaskReleased

ReceivingBarcodeResolved
ReceivingQuantityConfirmed
ReceivingTaskCompleted

PutawayConfirmed
InventoryMoved

PickConfirmed
ShortPickReported
PickingTaskCompleted

ReplenishmentConfirmed
ReplenishmentTaskCompleted

StockTransferConfirmed

CycleCountSubmitted
CycleCountRecountRequested
CycleCountCompleted

ShipmentPackageVerified
ShipmentConfirmed
```

Determine whether pure validation events should be sent to WMS or remain internal.

Do not publish unnecessary events for read-only scans unless WMS requires trace/audit.

---

# 21. WMS Acknowledgement and Result Events

For outbound commands determine whether the PDA Backend:

* commits locally and informs WMS asynchronously;
* waits synchronously for WMS;
* records `PENDING_WMS`;
* records `ACKNOWLEDGED`;
* records `REJECTED`;
* compensates or reconciles after rejection.

Define possible result events:

```text
ReceivingConfirmationAccepted
ReceivingConfirmationRejected

PutawayAccepted
PutawayRejected

PickAccepted
PickRejected

TransferAccepted
TransferRejected

CycleCountAccepted
CycleCountReviewRequired
CycleCountRejected

ShipmentConfirmationAccepted
ShipmentConfirmationRejected
```

For every result document:

* command ID;
* event ID;
* WMS transaction ID;
* aggregate ID;
* accepted version;
* error code;
* retryability;
* authoritative quantities;
* authoritative balances;
* processing timestamp.

---

# 22. Workflow-by-Workflow Integration

Create a complete subsection for every PDA workflow.

Use this exact format:

```markdown
## Workflow — Receiving

### PDA screens

### PDA APIs

### Backend-owned state

### WMS-owned state

### Required WMS inbound events

### Required WMS data fields

### PDA command flow

### Outbound backend events

### WMS acknowledgement/result events

### Database transaction

### Projection changes

### Inventory effects

### Cache invalidation

### Scanner and QR behavior

### Idempotency

### Versioning and ordering

### Offline behavior

### Failure behavior

### Reconciliation

### End-to-end test scenarios
```

Cover:

* authentication context mapping where relevant;
* dashboard;
* task center;
* receiving;
* putaway;
* picking;
* replenishment;
* inventory inquiry;
* transfer;
* cycle count;
* shipping.

---

# 23. Detailed WMS Data Requirements per Workflow

For every workflow, create a field-level matrix:

| Data field | Required by PDA screen/action | Source WMS event | Projection table | Mandatory | Freshness | Missing behavior |
| ---------- | ----------------------------- | ---------------- | ---------------- | --------: | --------- | ---------------- |

Examples for receiving:

```text
receivingTaskId
purchaseOrderId
supplierId
supplierName
lineId
itemId
itemCode
barcode aliases
expectedQuantity
receivedQuantity
remainingQuantity
uom
lotRequired
serialRequired
expiryRequired
conditionPolicy
overReceiptTolerance
underReceiptPolicy
remarkRequired
assignedOperatorId
taskStatus
taskVersion
dueAt
```

Examples for picking:

```text
pickingTaskId
salesOrderId
customer
lineId
itemId
expectedLocation
lot
serial
reservedQuantity
quantityToPick
quantityPicked
shortPickAllowed
shortPickReasonCodes
taskVersion
shipmentReadinessImpact
```

Do this for every domain.

---

# 24. Inventory Consistency Model

Document the relationship between:

* WMS authoritative inventory;
* PDA Backend inventory projection;
* PDA Backend transaction ledger;
* Redis cache;
* PDA App Room cache.

Define whether PDA Backend inventory is:

```text
authoritative transaction state
operational projection
temporary reservation state
WMS-derived read model
```

For every mutation document:

* local database effect;
* outbound event;
* WMS confirmation;
* balance projection update;
* reconciliation;
* mismatch behavior.

Never let Redis or the PDA App become inventory authority.

---

# 25. Reconciliation

Define scheduled and on-demand reconciliation.

Required comparisons may include:

* active tasks;
* task versions;
* inventory balances;
* receiving totals;
* picking totals;
* shipment status;
* package verification;
* cycle-count outcomes;
* command acknowledgements;
* unacknowledged outbox events.

Create:

| Reconciliation job | Data compared | Frequency | Mismatch action | Alert severity |
| ------------------ | ------------- | --------- | --------------- | -------------- |

Define:

* snapshot request;
* targeted aggregate replay;
* manual operator review;
* automatic projection repair;
* prohibited automatic inventory correction.

Do not automatically correct inventory without an approved policy.

---

# 26. Event Schema Governance

Document:

* JSON, Avro, Protobuf, or approved format;
* schema registry;
* subject naming;
* compatibility mode;
* required versus optional fields;
* additive changes;
* enum evolution;
* field deprecation;
* event-version migration;
* consumer compatibility;
* golden contract tests.

Prefer backward-compatible additive changes.

Do not reuse an event type with incompatible semantics.

---

# 27. Kafka Security

Document production requirements:

* TLS 1.2 or later;
* CA trust;
* mTLS when required;
* SASL when required;
* broker hostname validation;
* producer identity;
* consumer identity;
* least-privilege ACL;
* topic permissions;
* consumer-group permissions;
* DLQ permissions;
* secret management;
* certificate rotation.

Create an ACL matrix:

| Principal | Topic/group | Permission | Reason |
| --------- | ----------- | ---------- | ------ |

Do not include actual secrets.

---

# 28. Retry, DLQ, and Poison Events

Document:

* retryable errors;
* non-retryable schema errors;
* bounded retries;
* exponential backoff;
* retry topic versus in-process retry;
* DLQ event envelope;
* original topic;
* original partition;
* original offset;
* original headers;
* failure reason;
* retry count;
* stack/error redaction;
* remediation;
* replay procedure.

Poison events must preserve original metadata.

---

# 29. Observability

Define metrics:

```text
consumer_lag
event_processing_latency
event_age
inbox_duplicates_total
out_of_order_events_total
version_gap_total
projection_failures_total
dlq_messages_total
outbox_backlog
outbox_oldest_age
publish_failures_total
wms_ack_pending_total
reconciliation_mismatch_total
barcode_resolution_failures_total
unknown_barcode_total
```

Define logs and traces:

* event ID;
* aggregate ID;
* warehouse;
* topic;
* partition;
* offset;
* correlation ID;
* causation ID;
* command ID;
* safe error code.

Create alert recommendations.

---

# 30. Performance and Capacity

Estimate or derive:

* active PDA count;
* scan rate;
* mutation rate;
* inbound WMS event rate;
* snapshot size;
* inventory event volume;
* consumer concurrency;
* partition count;
* retention;
* replay duration.

Do not invent business volume as fact.

Where data is unavailable, provide a sizing questionnaire and formulas.

Identify risks:

* hot warehouse key;
* hot item key;
* mass inventory snapshot;
* task burst;
* reconnect storm;
* backlog after WMS outage.

---

# 31. Failure Scenarios

Document behavior for:

* Kafka unavailable;
* WMS producer stopped;
* PDA Backend consumer stopped;
* schema registry unavailable;
* invalid schema;
* duplicate event;
* stale event;
* version gap;
* partition reassignment;
* consumer rebalance;
* database unavailable;
* Redis unavailable;
* outbox backlog;
* DLQ unavailable;
* WMS rejects PDA command;
* WMS acknowledgement lost;
* PDA retries after timeout;
* backend restarts;
* full projection rebuild.

Create:

| Failure | PDA API behavior | Database behavior | Kafka behavior | Operator visibility | Recovery |
| ------- | ---------------- | ----------------- | -------------- | ------------------- | -------- |

---

# 32. Testing Strategy

## Unit Tests

* envelope validation;
* field mapping;
* barcode normalization;
* GS1 parsing;
* event key generation;
* version comparison;
* duplicate detection;
* error classification.

## Contract Tests

* every inbound event;
* every outbound event;
* schema compatibility;
* required fields;
* enums;
* example payloads.

## Kafka Integration Tests

* real broker;
* producer/consumer;
* duplicate;
* ordering;
* partition key;
* rebalance;
* restart;
* retry;
* DLQ;
* backlog;
* lag.

## PostgreSQL Integration Tests

* inbox transaction;
* projection update;
* version gap;
* duplicate no-op;
* reconciliation;
* outbox publication.

## WMS Simulator Tests

A simulator may be used for development, but it must be explicitly marked as a simulator.

It must not be treated as real WMS verification.

## Full E2E

```text
WMS event
→ Kafka
→ PDA Backend projection
→ PDA API
→ PDA App screen
→ QR/barcode scan
→ PDA command
→ PostgreSQL transaction
→ outbox
→ Kafka
→ WMS acknowledgement
→ PDA Backend reconciliation
→ PDA command status
→ PDA App refresh
```

Run at least one full E2E scenario for:

* receiving;
* putaway;
* picking;
* replenishment;
* transfer;
* cycle count;
* shipping.

---

# 33. Required WMS Contract Checklist

Create one checklist for the WMS team:

* [ ] Kafka bootstrap servers supplied.
* [ ] TLS/security mode supplied.
* [ ] ACL principals supplied.
* [ ] Topic names approved.
* [ ] Partition keys approved.
* [ ] Event schemas approved.
* [ ] Schema compatibility policy approved.
* [ ] Initial snapshot mechanism approved.
* [ ] Replay mechanism approved.
* [ ] Task ownership approved.
* [ ] Inventory authority approved.
* [ ] Operator mapping approved.
* [ ] Barcode/QR master source approved.
* [ ] GS1 parsing responsibility approved.
* [ ] Error/result events approved.
* [ ] Reconciliation APIs or events approved.
* [ ] Retention and compaction approved.
* [ ] DLQ ownership approved.
* [ ] SLA and lag thresholds approved.
* [ ] Staging environment available.
* [ ] Test data available.

---

# 34. Required Implementation Backlog

Create:

| Work item | Priority | Backend package | WMS dependency | Kafka topic/event | Tests | Acceptance criteria |
| --------- | -------- | --------------- | -------------- | ----------------- | ----- | ------------------- |

Group into phases:

```text
WMS-KAFKA Phase 00 — Ownership and contract freeze
WMS-KAFKA Phase 01 — Topic, schema, security and environment setup
WMS-KAFKA Phase 02 — Master-data projections
WMS-KAFKA Phase 03 — Task and workflow projections
WMS-KAFKA Phase 04 — Inventory projection and reconciliation
WMS-KAFKA Phase 05 — Receiving integration
WMS-KAFKA Phase 06 — Putaway, picking and replenishment
WMS-KAFKA Phase 07 — Transfer and cycle count
WMS-KAFKA Phase 08 — Shipping and package integration
WMS-KAFKA Phase 09 — QR, GS1, barcode and scanner validation
WMS-KAFKA Phase 10 — Replay, reconciliation and DLQ
WMS-KAFKA Phase 11 — Secure staging E2E and production readiness
```

Each work item must be specific enough for another AI agent to implement.

---

# 35. Required Appendices

Include:

## Appendix A — PDA Backend Capability Inventory

## Appendix B — WMS Data Ownership Matrix

## Appendix C — Required WMS Master Data

## Appendix D — Inbound WMS Event Catalog

## Appendix E — Outbound PDA Event Catalog

## Appendix F — Event Envelope

## Appendix G — Topic and Consumer-Group Matrix

## Appendix H — Field-Level Event Schemas

## Appendix I — Barcode and QR Contract

## Appendix J — GS1 Application Identifier Handling

## Appendix K — PDA Workflow-to-WMS Data Matrix

## Appendix L — Projection Table Map

## Appendix M — Cache Invalidation Map

## Appendix N — Idempotency and Ordering Rules

## Appendix O — DLQ and Replay Runbook

## Appendix P — Reconciliation Matrix

## Appendix Q — Kafka ACL Matrix

## Appendix R — Observability Metrics and Alerts

## Appendix S — E2E Test Scenarios

## Appendix T — Open Decisions and WMS Questions

---

# 36. Final Validation Checklist

Before finishing, verify:

* [ ] Entire PDA Backend codebase was inspected.
* [ ] All public PDA APIs were inventoried.
* [ ] All PDA workflows were mapped.
* [ ] All WMS-owned data was identified.
* [ ] All required inbound events were listed.
* [ ] All required outbound events were listed.
* [ ] Every event has a key and ordering rule.
* [ ] Event envelopes were documented.
* [ ] Duplicate handling was documented.
* [ ] Out-of-order behavior was documented.
* [ ] Snapshot and replay were documented.
* [ ] Reconciliation was documented.
* [ ] QR and barcode requirements were documented.
* [ ] GS1 handling was evaluated.
* [ ] Scanner contexts were mapped.
* [ ] Inventory consistency was defined.
* [ ] WMS acknowledgement behavior was defined.
* [ ] Kafka security was documented.
* [ ] ACLs were documented.
* [ ] Retry and DLQ were documented.
* [ ] Lag and backlog metrics were documented.
* [ ] Every proposed field was clearly marked.
* [ ] Current mock WMS behavior was identified.
* [ ] Real WMS verification gaps were identified.
* [ ] Implementation phases were created.
* [ ] The document is sufficient for both WMS and PDA Backend teams.

---

# 37. Execution Instruction

Proceed directly with repository inspection.

Inspect the complete PDA Backend implementation, especially:

```text
cmd/
internal/
api/openapi/
migrations/
docs/integration-pda-app/
requirement-report/
docker-compose*.yml
Makefile
```

Search for:

```text
Kafka
topic
consumer
producer
outbox
inbox
DLQ
WMS
mock WMS
fixture
barcode
QR
GS1
scan
symbology
inventory
task
receiving
putaway
picking
replenishment
transfer
count
shipment
package
reconciliation
checkpoint
replay
```

Trace every workflow from public PDA API to database, outbox, Kafka, and WMS boundary.

Then create:

```text
docs/wms-integration/PDA_BACKEND_WMS_KAFKA_EVENT_INTEGRATION_SPECIFICATION.md
```

and:

```text
requirement-report/YYYY-MM-DD-pda-backend-wms-kafka-integration-analysis.md
```

Do not return only an outline.

Do not invent existing WMS contracts.

Do not claim real WMS verification unless a real WMS environment was exercised.

Do not modify PDA Android code.

Do not implement code during this documentation task unless explicitly instructed.

After completion, report:

1. files and packages inspected;
2. PDA Backend capabilities identified;
3. PDA APIs mapped;
4. WMS data domains identified;
5. inbound event families proposed;
6. outbound event families proposed;
7. Kafka topics identified;
8. scanner purposes mapped;
9. barcode and QR formats evaluated;
10. projection tables mapped;
11. reconciliation flows defined;
12. security and ACL requirements defined;
13. unresolved WMS decisions;
14. files created;
15. final integration-document readiness status.
