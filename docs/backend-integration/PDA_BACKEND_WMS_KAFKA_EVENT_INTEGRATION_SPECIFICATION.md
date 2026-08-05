# PDA Backend - WMS Kafka Event Integration Specification

- Document status: **IMPLEMENTATION SPECIFICATION; WMS CONTRACT BASELINE VERIFIED, PDA ADAPTER AND SECURE STAGING BLOCKED**
- Generated date: 2026-08-02
- PDA Backend repository: `/home/neurosus/recoh-system/pda-backend`
- Backend branch/commit: `main` / `ae81080f55a9b4830c9a0c12807c9584c632595c`
- PDA App contract source: `docs/integration-pda-app/PDA_APP_API_SPECIFICATION.md`
- OpenAPI source: `api/openapi/pda-v1.yaml`
- Current Kafka mode: explicit `mock` by default; local shared-broker `kafka` mode is verified with PLAINTEXT
- Current WMS mode: explicit `mock` by default; HTTP mode currently supports only warehouse discovery in PDA Backend
- Integration scope: WMS master data, workflow data, inventory projections, PDA commands, Kafka delivery, replay, reconciliation, scanner context, and production readiness
- Intended readers: PDA Backend, WMS, Kafka platform, PDA App, QA, operations, security, and observability teams
- Source-of-truth policy: current Go source and migrations, then OpenAPI and tests, then reports and app requirements; proposed WMS contracts are never represented as implemented

## 1. Executive Summary

The current backend is a PostgreSQL-authoritative PDA workflow service with Redis cache-aside behavior and an outbox-based Kafka publisher. PDA REST mutations execute a database transaction, persist audit/idempotency/outbox state, invalidate cache, and attempt publication. The integration-event service retries unpublished outbox records. The current code has no WMS Kafka consumer, no WMS workflow synchronization projection, no checkpoint/replay store, and no approved ownership contract for PDA task, receiving, movement, cycle-count, or shipping aggregates.

The target integration is:

```text
WMS -> WMS Kafka topics -> PDA Backend consumer/inbox -> PostgreSQL projections
    -> Redis invalidation -> PDA REST API -> PDA App

PDA App -> PDA REST command -> PostgreSQL transaction/outbox
    -> PDA Backend Kafka producer -> WMS consumer
    -> WMS acknowledgement/result -> PDA Backend reconciliation -> PDA App
```

The local WMS adapter currently supports only warehouse discovery through `GET /api/wms/master-data/warehouses?limit=500`. Deterministic WMS fixtures seed workflow rows in local mock mode. This is `CURRENTLY_MOCKED`, not WMS integration evidence. The local Kafka publisher, consumer, inbox, DLQ, and outbox worker are implemented, but the consumer is generic infrastructure and is not wired to WMS projections.

### 1.1 Verified WMS implementation baseline

The following facts were verified against the sibling `ricoh-wms` repository on 2026-08-02. They are the current WMS implementation baseline, not proposed PDA contracts. WMS owns separate databases for master data, inventory, inbound, and outbound. The WMS services are healthy in the local shared environment, but PDA Backend has not yet implemented clients or projections for these APIs/events.

| WMS service/database | Implemented HTTP API surface | Kafka publishes | Kafka consumes | PDA Backend status |
|---|---|---|---|---|
| `wms-master-data-service` / `wms_master_data_db` | `/api/wms/master-data/resources`, warehouses, zones, locations, bins, item-UOM mappings | `WMS.MasterData.WarehouseCreated.v1`, `ZoneCreated.v1`, `LocationCreated.v1`, `StorageBinCreated.v1`, `ItemUOMMappingCreated.v1` | `MES.MasterData.ItemRevisionReleased.v2` | Warehouse list only; all other resources need an adapter/projection |
| `wms-inventory-service` / `wms_inventory_db` | `/api/wms/inventory/balances`, `/movements`, `/movements/receipt`, `/movements/transfer-to-staging` | None | `MES.MasterData.ItemRevisionReleased.v2`, `WMS.MasterData.LocationCreated.v1`, `MES.Execution.MaterialConsumed.v1` | No PDA client, consumer, or WMS inventory projection |
| `wms-inbound-service` / `wms_inbound_db` | `/api/wms/inbound/receipts`, `/receipts/{id}`, `/receipts/{id}/confirm` | None | None | No PDA client; receipt confirmation is synchronous inside WMS |
| `wms-outbound-service` / `wms_outbound_db` | `/api/wms/outbound/material-requests`, `/material-requests/{id}`, `/realtime/ws` | `WMS.Outbound.MaterialStaged.v1`, `WMS.Outbound.MaterialShortageDeclared.v1` | `WMS.MasterData.LocationCreated.v1`, `MES.Execution.MaterialStagingRequested.v1`, `MES.MasterData.ItemRevisionReleased.v2` | No PDA client or consumer; these are MES work-center staging results, not PDA picking/shipping events |

The WMS contract source is `ricoh-wms/packages/contracts/openapi/wms-public-v1.yaml`, `packages/contracts/asyncapi/wms-events-v1.yaml`, and `packages/contracts/events/event-envelope-v1.schema.json`. The AsyncAPI is a frozen WMS compatibility surface for the seven WMS-owned event channels above. It does not define PDA task, receiving, picking, cycle-count, transfer, or shipping synchronization events.

### 1.2 WMS APIs currently backed by WMS databases

| API family | Current endpoints | Database authority | Direct PDA integration |
|---|---|---|---|
| Master data | `GET /resources`; `GET/POST /warehouses`; `GET/PUT /warehouses/{id}`; `GET/POST /warehouses/{id}/zones`; `GET/PUT /zones/{id}`; `GET/POST /zones/{id}/locations`; `GET /locations`; `GET/PUT /locations/{id}`; `GET/POST /locations/{id}/bins`; `GET/PUT /bins/{id}`; `GET/POST /item-uom-mappings`; `GET /item-uom-mappings/{id}` | `wms_master_data_db` | Only warehouse list is implemented in `UpstreamWMS`; remaining APIs are unsupported by PDA Backend |
| Inventory | `GET /api/wms/inventory/balances`, `GET /movements`, `POST /movements/receipt`, `POST /movements/transfer-to-staging` | `wms_inventory_db` | Unsupported by PDA Backend |
| Inbound | `POST /api/wms/inbound/receipts`, `GET /receipts/{id}`, `POST /receipts/{id}/confirm` | `wms_inbound_db` plus synchronous inventory posting | Unsupported by PDA Backend |
| Outbound | `GET/POST /api/wms/outbound/material-requests`, `GET /material-requests/{id}`, `GET /realtime/ws` | `wms_outbound_db` | Unsupported by PDA Backend |

These APIs are WMS service APIs, not PDA App APIs. The PDA App must continue to call PDA Backend only; PDA Backend needs an authenticated, timeout-limited anti-corruption adapter before exposing any of them to the app.

| Domain | PDA Backend support | WMS data required | Inbound event required | Outbound event required | Readiness |
|---|---|---|---:|---:|---|
| Identity/operator mapping | Backend identity, roles, warehouse scope | Optional WMS employee/task-assignment mapping | Proposed `OperatorMappingChanged` | Usually none | BLOCKED_BY_WMS_CONTRACT |
| Warehouse master | `UpstreamWMS.Warehouses`; identity warehouse tables | Warehouse identity/name/status/timezone | Proposed master events | None | Partial |
| Item/barcode master | Workflow rows contain item/barcode fields; no master projection | Item, UOM, aliases, lot/serial policy | Proposed item events | None | BLOCKED_BY_WMS_CONTRACT |
| Locations/zones | Movement rows store location codes; no master projection | Location, zone, capacity, compatibility | Proposed location events | None | BLOCKED_BY_WMS_CONTRACT |
| Inventory | PostgreSQL balances and movement ledger | WMS authority, reservations, lot/serial/ownership | Proposed inventory events/snapshots | Transfer/movement/count commands | BLOCKED_BY_WMS_CONTRACT |
| Tasks | Task, receiving, movement, count, shipment aggregates | Task creation, assignment, cancellation, completion | Proposed task events | Claim/release/workflow commands | BLOCKED_BY_WMS_CONTRACT |
| Receiving | Full local receiving transaction and command status | Inbound order, expected lines, policy | Proposed inbound/receiving events | Receiving confirmation/completion | BLOCKED_BY_WMS_CONTRACT |
| Putaway | Validated movement transaction | Putaway tasks, source/destination policy | Proposed putaway events | Putaway confirmation | BLOCKED_BY_WMS_CONTRACT |
| Picking | Validated picking transaction | Orders, reservations, pick tasks | Proposed outbound/picking events | Pick/short/completion | BLOCKED_BY_WMS_CONTRACT |
| Replenishment | Validated partial movement | Demand, source/destination tasks | Proposed replenishment events | Replenishment confirmation | BLOCKED_BY_WMS_CONTRACT |
| Transfer | Atomic local transfer and ledger | WMS ownership/confirmation policy | Proposed balance/result events | Stock transfer command | BLOCKED_BY_WMS_CONTRACT |
| Cycle count | Count/recount/review workflow | Count plan and authoritative result | Proposed count events | Count submission/recount | BLOCKED_BY_WMS_CONTRACT |
| Shipping | Shipment/package/readiness/confirmation | Shipment, packages, carrier, tracking | Proposed shipment events | Package verification/confirmation | BLOCKED_BY_WMS_CONTRACT |

Real WMS integration cannot begin safely beyond the warehouse-master adapter until WMS ownership, schemas, topic/security configuration, snapshot/replay, and staging credentials are approved.

## 2. Current Capability Inventory

| ID | Capability | Public API | Database state | Events produced today | Events consumed today | WMS dependency | Status |
|---|---|---|---|---|---|---|---|
| C-001 | Login/refresh/logout | `/auth/login`, `/auth/refresh`, `/auth/logout` | identity operators/sessions/tokens | identity audit only | none | WMS not required | CURRENTLY_IMPLEMENTED |
| C-002 | Bootstrap/profile/context | `/bootstrap`, `/me`, `/me/warehouses`, `/devices/registrations` | identity/device/warehouse tables | audit | none | warehouse/device policy may be WMS-derived | CURRENTLY_IMPLEMENTED |
| C-003 | Dashboard/task summary | `/dashboard`, `/tasks/summary` | task projections and cache | none for reads | none | task/inventory freshness | CURRENTLY_IMPLEMENTED / MOCK DATA |
| C-004 | Task list/detail/claim/release | `/tasks`, `/tasks/{id}`, claim/release | `warehouse_task`, idempotency, outbox | `TaskClaimed`, `TaskReleased` | none | task ownership and WMS assignment | CURRENTLY_IMPLEMENTED / WMS BLOCKED |
| C-005 | Receiving | `/receiving/tasks*`, receipts, completion, command status | receiving tasks/lines, balances, commands, audit/outbox | `ReceivingStarted`, `ReceivingQuantityConfirmed`, `ReceivingCompleted` where emitted | none | inbound source and authoritative receipt | CURRENTLY_IMPLEMENTED / MOCK DATA |
| C-006 | Putaway | `/putaway/tasks*`, validation, confirmation | movement task, balances, commands, audit/outbox | `PutawaySourceValidated`, `PutawayDestinationValidated`, `InventoryMoved` | none | task and capacity source | CURRENTLY_IMPLEMENTED / MOCK DATA |
| C-007 | Picking | `/picking/tasks*`, validation, picks, completion | movement task, balances, commands, audit/outbox | `PickingLocationValidated`, `PickingItemValidated`, `InventoryMoved` | none | order/reservation source | CURRENTLY_IMPLEMENTED / MOCK DATA |
| C-008 | Replenishment | `/replenishment/tasks*`, validation, confirmation | movement task, balances, commands, audit/outbox | validation events, `InventoryMoved` | none | demand and task source | CURRENTLY_IMPLEMENTED / MOCK DATA |
| C-009 | Inventory inquiry | `/inventory/search`, `/inventory/items`, `/inventory/balances`, `/inventory/movements` | inventory balances/ledger | none for reads | none | WMS authority/freshness | CURRENTLY_IMPLEMENTED / MOCK DATA |
| C-010 | Transfer | transfer validation and `/inventory/transfers` | balances, ledger, commands, audit/outbox | `StockTransferConfirmed` | none | WMS authorization/result | CURRENTLY_IMPLEMENTED / WMS BLOCKED |
| C-011 | Cycle count | count list/detail, validation, submit/recount/complete | count tasks/lines, commands, audit/outbox | count events | none | WMS count plan/approval | CURRENTLY_IMPLEMENTED / WMS BLOCKED |
| C-012 | Shipping | shipment/readiness/package verify/confirm/status | shipment/packages, commands, audit/outbox | `ShipmentPackageVerified`, `ShipmentReadinessChanged`, `ShipmentConfirmed`, `OrderShipped` | none | WMS shipment authority | CURRENTLY_IMPLEMENTED / MOCK DATA |
| C-013 | Warehouse HTTP adapter | internal integration port only | cache-aside warehouse read | none | none | WMS HTTP master endpoint | CURRENTLY_IMPLEMENTED |
| C-014 | Kafka outbox publisher | internal only | `domain_outbox` | all domain events | none | broker only | CURRENTLY_IMPLEMENTED |
| C-015 | Generic Kafka consumer | internal only | `event_inbox`, `event_dlq` | none | configured topic callback | no WMS handler wired | CURRENTLY_IMPLEMENTED INFRASTRUCTURE |

The current domain envelope requires nonempty `operatorId`, `deviceId`, `warehouseId`, and `causationId`. A WMS-originated event will not satisfy that internal shape without an explicit adapter envelope policy. Do not deserialize WMS records directly into `DomainEventEnvelope` unless the WMS contract supplies equivalent values and the policy is approved.

## 3. Ownership and Consistency Model

| Entity/data | PDA Backend ownership | WMS ownership | PDA usage | Kafka source | Local projection | Reconciliation |
|---|---|---|---|---|---|---|
| Warehouse | Identity access copy | Usually WMS master | Context selection | WMS master events | `identity_warehouses` | Yes |
| Zone/location/capacity | Validation projection only | WMS master/policy | Scan validation | WMS master events | Future `wms_locations` | Yes |
| Item/SKU/UOM | Validation projection only | WMS master | Scan/display/quantity | WMS item events | Future `wms_items`/aliases | Yes |
| Barcode/GTIN/QR alias | No authoritative ownership | WMS/item master | Scanner resolution | WMS alias events | Future alias table | Yes |
| Lot/serial/LPN | Transaction references/projection | WMS inventory identity | Scan and constraints | WMS inventory events | Future inventory dimensions | Yes |
| Inventory balance | Current transaction state is backend authoritative locally; target WMS authority unresolved | Expected enterprise authority | Inquiry and validation | WMS snapshots/deltas | Existing `inventory_balance` plus WMS metadata | Mandatory |
| Reservation | Not currently modeled as WMS reservation | WMS | Picking availability | WMS reservation events | Future projection | Mandatory |
| PDA task aggregate | Backend transaction ownership today | Ownership not approved | Task UI/mutations | Proposed WMS task events | Existing task tables | Mandatory |
| Receiving/order lines | Backend local workflow state today | Inbound document authority unresolved | Receiving UI | Proposed inbound events | Existing receiving tables | Mandatory |
| Movement task | Backend local workflow state today | WMS task authority unresolved | Putaway/pick/replenish | Proposed task events | Existing movement table | Mandatory |
| Cycle-count result | Backend records evidence | WMS approval/adjustment authority unresolved | Count UI/review | Proposed count events | Existing count tables | Mandatory |
| Shipment/package | Backend local shipping aggregate today | WMS/shipping authority unresolved | Verify/confirm | Proposed shipment events | Existing shipping tables | Mandatory |
| Audit/idempotency/outbox | PDA Backend owned | WMS has independent audit | Support/replay | PDA events | PostgreSQL | Yes |
| Redis | Never authoritative | Not WMS authority | Read acceleration | No direct source | Cache only | No |
| PDA Room | Never authoritative | No direct relationship | Offline cache/draft | No direct Kafka | PDA-owned | Via REST |

Until WMS ownership is approved, the backend must not apply inbound WMS events to existing aggregates or silently overwrite local state. Use separate WMS projection tables and an explicit reconciliation process first.

## 4. WMS Master Data Requirements

### 4.1 Warehouse

Business purpose: authorize warehouse context and display a selectable site. Current implementation consumes only the HTTP warehouse list with `warehouse_id`, `warehouse_code`, and localized `warehouse_name`.

Required target fields: `warehouseId`, `warehouseCode`, `warehouseName`, `status`, `timezone`, `locale`, `version`, `occurredAt`. Initial snapshot and incremental create/update/deactivate events are required. Deactivation blocks new context selection but must preserve historical audit references. Projection: `identity_warehouses` plus a future WMS metadata table. Affected APIs: bootstrap, warehouses, dashboard, all warehouse-scoped routes.

### 4.2 Zones and locations

Required target fields: `locationId`, `locationCode`, `warehouseId`, `zoneId`, `locationType`, `status`, `capacity`, `capacityUnit`, `allowedItemClasses`, `temperatureClass`, `hazardClass`, `pickSequence`, `putawayPriority`, `version`. Location status/capacity changes must be ordered per location and invalidate movement/inventory caches. No current location master projection exists; this domain is `BLOCKED_BY_WMS_CONTRACT`.

### 4.3 Item, UOM, and policy

Required target fields: `itemId`, `itemCode`, `sku`, `description`, `baseUom`, `supportedUoms`, `conversionRules`, `status`, `lotControlled`, `serialControlled`, `expiryControlled`, `conditionControlled`, `weight`, `dimensions`, `itemClass`, `hazardClass`, `temperatureClass`, `version`. Item/barcode updates must not retroactively reinterpret an already accepted command. Cache invalidation affects receiving, picking, replenishment, inventory, and count screens.

### 4.4 Barcode and QR aliases

Required target fields: `barcode`, `normalizedBarcode`, `barcodeType`, `symbology`, `itemId`, `uom`, `quantityPerBarcode`, `lotEncoded`, `serialEncoded`, `expiryEncoded`, `supplierBarcode`, `customerBarcode`, `status`, `validFrom`, `validTo`, `version`. Alias removal is a tombstone/deactivation event, not physical deletion until retention allows it. No current WMS alias consumer exists.

### 4.5 Inventory dimensions and containers

Required target fields for inventory events: `warehouseId`, `locationId`, `locationCode`, `itemId`, `lotNumber`, `serialNumber`, `lpnId`, `palletId`, `ownerId`, `condition`, `quantity`, `uom`, `reservedQuantity`, `availableQuantity`, `asOf`, `sourceVersion`. LPN/pallet projections must not be enabled until the WMS/PDA contract requires them and provides lifecycle events.

### 4.6 Snapshot contract

Each master/inventory/task snapshot must identify `snapshotId`, `domain`, `snapshotVersion`, `startedAt`, `completedAt`, `highWaterMark`, `sourceSystem`, and an incremental event boundary. The consumer must capture the live-event offset before or atomically with snapshot activation so it cannot lose events between snapshot and incremental consumption. A bulk file/API snapshot is acceptable only with a documented checkpoint and replay procedure.

## 5. Barcode, QR, and GS1 Contract

The current backend accepts raw/normalized barcode, symbology, scan context, and timestamp in several handlers, but it does not implement a shared GS1 parser or master-data alias service. Current receiving and movement fixture matching is simple string equality. Therefore enterprise symbologies are proposed requirements, not current support.

### 5.1 Supported evaluation set

Evaluate Code 128, Code 39, EAN-13, EAN-8, UPC-A, UPC-E, ITF-14, GS1-128, GS1 DataMatrix, GS1 QR, standard QR, Data Matrix, location, item, lot, serial, LPN, pallet, package, and shipment/tracking identifiers. The WMS contract must state which are enabled per warehouse and scanner profile.

### 5.2 Canonical scan request

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

Operator, device, warehouse, correlation, and authorization come from the authenticated PDA request, not from the payload. Maximum length, control-character rejection, normalization, sensitive logging redaction, duplicate-scan suppression, and cross-warehouse rejection must be defined in the PDA API contract.

### 5.3 GS1 responsibility

WMS and PDA Backend must choose one authoritative parser, preferably a shared versioned service/library. Required handling includes FNC1/group separators, leading zero preservation, check digits, AI 01 GTIN, AI 10 lot, AI 17 expiry, AI 21 serial, AI 00 SSCC, variable quantity, UOM conversion, unknown AIs, malformed payloads, and date/timezone rules. The PDA may provide a preview parse, but business acceptance remains server-authoritative. No current parser exists in this repository.

### 5.4 Scan response

Target response fields: `resolvedEntityType`, `itemId`, `itemCode`, `displayCode`, `uom`, `embeddedQuantity`, `lotNumber`, `serialNumber`, `expiryDate`, `validationStatus`, `nextStep`, `quantityConstraints`, and `taskVersion`. WMS master data supplies item/alias/policy fields; PDA Backend supplies task authorization and context. Unknown alias, wrong task context, deactivated alias, malformed GS1, duplicate accepted scan, and stale task version need distinct safe error codes.

## 6. Kafka Boundary and Current Event Envelope

### 6.1 Current backend envelope

The implemented `DomainEventEnvelope` is:

```json
{
  "eventId": "uuid",
  "eventType": "ShipmentConfirmed",
  "eventVersion": 1,
  "aggregateType": "Shipment",
  "aggregateId": "SHIP-001",
  "aggregateVersion": 7,
  "occurredAt": "2026-08-02T12:00:00Z",
  "correlationId": "uuid",
  "causationId": "uuid",
  "warehouseId": "WH-01",
  "operatorId": "OP-01",
  "deviceId": "DEVICE-01",
  "topic": "pda.shipping.events.v1",
  "payload": {}
}
```

`event.Validate()` requires all identity, version, timestamp, context, topic, and JSON fields. The publisher serializes the envelope as JSON and uses `aggregateId` as the Kafka key. It resolves a logical topic with `PDA_KAFKA_TOPIC_PREFIX`, defaulting to `pda`.

The verified WMS wire envelope is different:

```json
{
  "event_id": "uuid",
  "event_type": "WMS.MasterData.LocationCreated.v1",
  "occurred_at": "2026-08-02T12:00:00Z",
  "trace_id": "trace-id",
  "producer": "wms-master-data-service",
  "payload": {}
}
```

WMS requires `event_id`, `event_type`, `occurred_at`, and `payload`; `trace_id` and `producer` are optional in the shared JSON Schema, although the WMS master-data producer schema also requires `source_service` and `trace_id`. WMS uses snake_case names and does not require PDA operator/device context, aggregate version, or PDA topic metadata. A PDA consumer must validate the WMS schema, preserve the original record, derive only approved projection identifiers, and never fabricate operator or device IDs.

### 6.2 Target WMS adapter envelope

WMS events need an external envelope that can represent service-originated records without a PDA operator/device. The proposed shared contract is:

```json
{
  "eventId": "uuid",
  "eventType": "ReceivingTaskUpdated",
  "eventVersion": 1,
  "schemaId": "wms.receiving-task-updated.v1",
  "sourceSystem": "WMS",
  "producer": "wms-task-service",
  "aggregateType": "RECEIVING_TASK",
  "aggregateId": "REC-001",
  "aggregateVersion": 7,
  "warehouseId": "WH-01",
  "occurredAt": "2026-08-02T12:00:00Z",
  "publishedAt": "2026-08-02T12:00:01Z",
  "correlationId": "uuid",
  "causationId": "uuid",
  "traceId": "trace-id",
  "tenantId": "tenant",
  "replay": false,
  "payload": {}
}
```

This is `PROPOSED_WMS_EVENT`, not an approved contract. The anti-corruption adapter must validate the external envelope, map it to an internal projection command, retain the original event metadata, and never manufacture an operator or device identity.

## 7. Topic, Key, and Consumer-Group Model

### 7.1 Current topics

| Topic | Current producer | Current use | Key | Status |
|---|---|---|---|---|
| `pda.task.events.v1` | task service | task claim/release | `aggregateId=taskId` | CURRENTLY_IMPLEMENTED |
| `pda.receiving.events.v1` | receiving service | receiving mutation events where configured | aggregate ID | CURRENTLY_IMPLEMENTED / verify emitted list |
| `pda.movement.events.v1` | movement service | putaway/picking/replenishment | `taskId` | CURRENTLY_IMPLEMENTED |
| `pda.inventory.events.v1` | inventory service | transfer/count events | aggregate ID | CURRENTLY_IMPLEMENTED |
| `pda.shipping.events.v1` | shipping service | package/readiness/ship events | `shipmentId` | CURRENTLY_IMPLEMENTED |

### 7.2 Verified WMS topics and event status

These are the topic addresses defined by the current WMS AsyncAPI and service manifests. WMS uses the snake_case envelope defined in `event-envelope-v1.schema.json`; it is not wire-compatible with the PDA Backend `DomainEventEnvelope` and must be mapped by an anti-corruption consumer.

| Topic | WMS owner | Current payload/use | WMS status | PDA Backend status |
|---|---|---|---|---|
| `WMS.MasterData.WarehouseCreated.v1` | Master data | `warehouse_id`, code, localized name, status and source metadata | APPROVED_WMS_EVENT | No consumer; identity warehouse synchronization not implemented |
| `WMS.MasterData.ZoneCreated.v1` | Master data | `zone_id`, warehouse, code, localized name, type, status | APPROVED_WMS_EVENT | No consumer or zone projection |
| `WMS.MasterData.LocationCreated.v1` | Master data | location identity, zone, localized name, status, location purpose, staging work-center reference | APPROVED_WMS_EVENT | No consumer or location projection |
| `WMS.MasterData.StorageBinCreated.v1` | Master data | bin identity, location, localized name, capacity and status | APPROVED_WMS_EVENT | No consumer or bin projection |
| `WMS.MasterData.ItemUOMMappingCreated.v1` | Master data | mapping, item revision, storage UOM, conversion and bin capacity | APPROVED_WMS_EVENT | No consumer or item/UOM projection |
| `WMS.Outbound.MaterialStaged.v1` | Outbound | material request result, requested/available/transferred quantities, staging location, requirements | APPROVED_WMS_EVENT; runtime observed | No consumer; not mapped to PDA picking/shipping |
| `WMS.Outbound.MaterialShortageDeclared.v1` | Outbound | all-or-nothing shortage result and requirement/request context | APPROVED_WMS_EVENT; runtime observed | No consumer; no PDA shortage projection |

WMS inventory consumes master/MES events but publishes no WMS inventory events. WMS inbound publishes no Kafka events; receipt confirmation calls the WMS inventory API synchronously. Therefore a PDA inventory or receiving projection cannot be built from WMS Kafka today. It requires either approved WMS snapshot/HTTP reads or new WMS result events.

### 7.3 Proposed WMS topics

| Topic | Owner | Producer | Consumers | Partition key | Compaction/retention | Status |
|---|---|---|---|---|---|---|
| `wms.master-data.events.v1` | WMS | WMS master service | PDA master projection | entity ID | compacted plus tombstones | PROPOSED_WMS_EVENT; do not create while the seven typed WMS topics are authoritative |
| `wms.inventory.events.v1` | WMS | inventory service | PDA inventory projection/reconciliation | balance identity | replay retention | PROPOSED_WMS_EVENT |
| `wms.task.events.v1` | WMS | task orchestration | PDA task projection | task ID | replay retention | PROPOSED_WMS_EVENT |
| `wms.receiving.events.v1` | WMS | inbound service | PDA receiving projection | receiving task ID | replay retention | PROPOSED_WMS_EVENT |
| `wms.outbound.events.v1` | WMS | order/picking service | PDA picking/replenishment projection | order/task ID | replay retention | PROPOSED_WMS_EVENT |
| `wms.shipping.events.v1` | WMS | shipping service | PDA shipment projection | shipment ID | replay retention | PROPOSED_WMS_EVENT |
| `pda.*.events.v1` | PDA Backend | outbox publisher | WMS consumers/reconciliation | aggregate ID | replay retention | CURRENTLY_IMPLEMENTED PDA side |
| `integration.dlq.v1` | platform | consumer/DLQ writer | operations/replay tool | original event ID | long retention | CURRENTLY_IMPLEMENTED storage, topic unverified |

Global ordering is not promised. Ordering is per aggregate key and aggregate version. Inventory keys may become hot; capacity tests must determine whether warehouse plus item plus location is sufficient or whether a partitioning strategy is required.

### 7.3 Consumer groups

| Group | Topics | Purpose | Current state |
|---|---|---|---|
| `pda-backend` | configured consumer topic | Generic backend consumer/outbox store | CURRENTLY_IMPLEMENTED infrastructure |
| `pda-wms-master-projection` | `wms.master-data.events.v1` | Master projection | PROPOSED |
| `pda-wms-task-projection` | task/receiving/outbound topics | Workflow projection | PROPOSED |
| `pda-wms-inventory-projection` | inventory topic | Inventory projection | PROPOSED |
| `pda-wms-reconciliation` | result and snapshot topics | Reconciliation | PROPOSED |

Do not use one consumer group for projections with different failure/replay policies unless the ownership and transaction boundary are intentionally shared.

## 8. Inbound WMS Event Catalog

All events in this proposed section are `PROPOSED_WMS_EVENT`. The seven typed events in section 7.2 are the only currently verified WMS event contracts. Each future event must use a stable aggregate key, a versioned schema, and an approved ownership decision before PDA consumes it.

| Event family | Event names | Topic | Aggregate/key | Projection/API impact |
|---|---|---|---|---|
| Warehouse | `WarehouseCreated`, `WarehouseUpdated`, `WarehouseDeactivated` | master | warehouse ID | identity warehouse/bootstrap |
| Zone/location | `ZoneCreated`, `ZoneUpdated`, `ZoneDeactivated`, `LocationCreated`, `LocationUpdated`, `LocationBlocked`, `LocationUnblocked`, `LocationDeactivated` | master | zone/location ID | movement/inventory validation |
| Item | `ItemCreated`, `ItemUpdated`, `ItemDeactivated`, `ItemBarcodeAssigned`, `ItemBarcodeUpdated`, `ItemBarcodeRemoved`, `UnitOfMeasureUpdated`, `ItemHandlingPolicyUpdated` | master | item/alias ID | receiving/picking/inventory/count |
| Inventory | `InventoryBalanceSnapshot`, `InventoryBalanceChanged`, `InventoryReserved`, `InventoryReservationReleased`, `InventoryHoldChanged`, `InventoryLotChanged`, `InventorySerialChanged`, `InventoryOwnershipChanged` | inventory | balance identity | inventory APIs and task validation |
| Task | `TaskCreated`, `TaskAssigned`, `TaskUpdated`, `TaskPriorityChanged`, `TaskCancelled`, `TaskSuspended`, `TaskReleased`, `TaskCompletedExternally` | task | task ID | dashboard/task center |
| Receiving | `InboundOrderCreated`, `InboundOrderUpdated`, `InboundOrderCancelled`, `ReceivingTaskCreated`, `ReceivingTaskUpdated`, `ReceivingTaskCancelled`, `ReceivingLineUpdated`, `ReceivingPolicyUpdated` | receiving | receiving task ID | receiving APIs |
| Putaway | `PutawayTaskCreated`, `PutawayTaskUpdated`, `PutawayTaskCancelled`, `PutawayDestinationPolicyUpdated` | task | putaway task ID | putaway APIs |
| Picking | `OutboundOrderCreated`, `OutboundOrderUpdated`, `OutboundOrderCancelled`, `PickingTaskCreated`, `PickingTaskUpdated`, `PickingTaskCancelled`, `PickingLineUpdated`, `ReservationChanged` | outbound | order/task ID | picking APIs |
| Replenishment | `ReplenishmentTaskCreated`, `ReplenishmentTaskUpdated`, `ReplenishmentTaskCancelled`, `ReplenishmentDemandChanged` | task | replenishment task ID | replenishment APIs |
| Count | `CycleCountTaskCreated`, `CycleCountTaskUpdated`, `CycleCountTaskCancelled`, `CycleCountReviewRequested`, `CycleCountApproved`, `CycleCountRejected` | task | count task ID | count APIs |
| Shipping | `ShipmentCreated`, `ShipmentUpdated`, `ShipmentCancelled`, `PackageCreated`, `PackageUpdated`, `PackageCancelled`, `ShipmentReadinessChanged`, `CarrierAssignmentChanged`, `TrackingNumberAssigned`, `ShipmentConfirmedByWms` | shipping | shipment/package ID | shipping APIs |

For every event the implementation must record: direction, business purpose, topic, partition key, producer, consumer group, trigger, preconditions, envelope, payload schema, ownership, versioning, ordering, duplicate behavior, out-of-order behavior, missing-event behavior, retry/DLQ, database effect, cache invalidation, affected PDA APIs/screens, audit, metrics, example JSON, contract tests, and reconciliation. The family matrix above is the catalog; field-level schema approval is still required before implementation.

## 9. Outbound PDA Backend Event Catalog

| Event | Triggering API | Current topic | WMS action | Acknowledgement | Command status |
|---|---|---|---|---|---|
| `TaskClaimed` | task claim | `pda.task.events.v1` | Reserve/assign if WMS owns assignment | Proposed WMS ack | local acknowledged; WMS pending if required |
| `TaskReleased` | task release | task | Release reservation/assignment | Proposed ack | local acknowledged; reconcile |
| `ReceivingBarcodeResolved` | barcode resolution | receiving | Usually no event; audit only unless WMS requires | No | no command |
| `ReceivingQuantityConfirmed` | receipt | receiving | Apply receipt | Required if WMS authority | `PENDING_WMS` target state |
| `ReceivingTaskCompleted` | completion | receiving | Close inbound task | Required | pending until result if required |
| `PutawayConfirmed` | putaway confirmation | movement | Apply movement | Required | pending/result |
| `InventoryMoved` | movement/transfer | movement or inventory | Apply balance movement | Required | pending/result |
| `PickConfirmed` | pick | movement | Consume reservation/stock | Required | pending/result |
| `ShortPickReported` | short pick | movement | Apply shortage policy | Required | pending/result |
| `PickingTaskCompleted` | completion | movement | Close pick task | Required | pending/result |
| `ReplenishmentConfirmed` | replenishment | movement | Apply replenishment | Required | pending/result |
| `StockTransferConfirmed` | transfer | `pda.inventory.events.v1` | Apply transfer | Required | pending/result |
| `CycleCountSubmitted` | count submit | `pda.inventory.events.v1` | Review/accept count | Required | pending/review |
| `CycleCountRecountRequested` | recount | inventory | Request recount | Required | pending |
| `CycleCountCompleted` | count completion | inventory | Close count | Required | pending/result |
| `ShipmentPackageVerified` | package verification | `pda.shipping.events.v1` | Record package verification | Usually no separate ack | local result |
| `ShipmentConfirmed` | shipment confirmation | shipping | Ship/close order | Required | pending/result |

Validation-only scans remain internal unless WMS requires audit/event evidence. The outbound event must be written in the same transaction as the backend mutation, using the command ID as `causationId` and the aggregate ID as Kafka key.

## 10. Inbox, Idempotency, Ordering, and Replay

The current generic consumer performs JSON decode, `AlreadyProcessed`, up to three in-process attempts, DLQ insertion on failure, `MarkProcessed`, and Kafka offset commit. `event_inbox` is unique on `(event_id, consumer_group)`. `event_dlq` stores envelope, attempts, and last error. This is useful infrastructure, but the current implementation does not atomically write a projection transaction with inbox insertion, does not persist payload hashes, does not compare aggregate versions, and commits malformed JSON without DLQ preservation. WMS projection consumers must close these gaps before production.

Required target transaction:

```text
Fetch record
-> validate external envelope/schema
-> begin PostgreSQL transaction
-> insert event_id, consumer_group, payload hash into inbox
-> duplicate same hash: commit no-op
-> duplicate different hash: security/reconciliation alert and DLQ
-> compare received aggregate version with projection version
-> APPLY or IGNORE_STALE or PARK_FOR_RETRY
-> update projection and reconciliation metadata
-> write audit/invalidation marker
-> commit transaction
-> commit Kafka offset
```

Version behavior:

| Received version | Current version | Action |
|---:|---:|---|
| equal already applied | same | `IGNORE_DUPLICATE` |
| current + 1 | current | `APPLY` |
| less than current | higher | `IGNORE_STALE` and metric |
| greater than current + 1 | lower | `PARK_FOR_RETRY`, request replay/snapshot |
| no projection and version 1/create | none | `APPLY` |
| no projection and update/version >1 | none | `PARK_FOR_RETRY` or snapshot |
| delete/cancel before create | none | retain tombstone or park, per contract |

Missing-event detection requires per-domain high-water marks, expected-version gaps, consumer lag, event age, and reconciliation scans. Projection rebuilds must create a new versioned projection schema/table, load a WMS snapshot, replay events from the captured high-water mark, validate counts/checksums, then switch reads atomically. Do not repair inventory by blindly replaying PDA outbound events.

## 11. Initial Synchronization and Reconciliation

Required snapshots: warehouse, zone/location, item/UOM/barcode, inventory, active tasks, inbound documents, outbound documents, open cycle counts, open shipments, and reservations. A snapshot must include a high-water event boundary and be idempotently resumable.

| Job | Compared data | Frequency | Mismatch action | Severity |
|---|---|---|---|---|
| Master checksum | WMS item/location/barcode vs projection | hourly and after replay | targeted snapshot/rebuild | high |
| Active task reconciliation | WMS task status/assignment vs PDA rows | 5-15 minutes | park conflicting task, operator alert | high |
| Inventory balance | WMS balance vs PDA projection/ledger | scheduled and on demand | block unsafe mutation; manual approval | critical |
| Receiving totals | expected/received/remaining | 15 minutes | re-fetch aggregate, investigate | high |
| Picking/reservation | reserved/picked/remaining | 15 minutes | block over-pick, request WMS snapshot | high |
| Shipment/package | package/readiness/status | 5 minutes | refresh shipment, block confirmation | high |
| Count outcome | count/review/approval | after each event | preserve evidence; no automatic stock correction | critical |
| Outbox acknowledgement | unpublished/unacknowledged commands | continuous | retry/replay/DLQ alert | high |

Automatic inventory correction is prohibited without an approved WMS policy. Reconciliation creates a mismatch record, evidence links, correlation/causation IDs, and an operator/platform action. PDA App receives refreshed REST state, not Kafka records.

## 12. Workflow Integration Requirements

### Authentication and context

Identity remains PDA Backend-owned. WMS may provide operator/task-assignment mappings only through an approved contract. Login, refresh, bootstrap, device registration, and warehouse scope do not consume Kafka directly. WMS-derived warehouse master updates must not grant access automatically without identity policy.

### Dashboard and task center

PDA APIs: `/dashboard`, `/tasks/summary`, `/tasks`, detail, claim, release. WMS supplies task creation, status, priority, assignment, cancellation, and completion if it owns task orchestration. PDA Backend stores a versioned projection and local lock/claim transaction. Claim/release events are outbound only if WMS owns reservations. A task event gap blocks mutation or causes a fresh aggregate request; it never silently uses stale task state.

### Receiving

PDA APIs: list/detail/start/barcode/receipt/completion/command status. Required WMS fields: inbound/PO ID, supplier, receiving task/line, item/barcode/UOM, expected/received/remaining quantity, lot/serial/expiry/condition policy, tolerance, assignment, status, version. Barcode resolution is validation-only. Receipt confirmation writes backend state/outbox atomically; if WMS is authoritative, the command remains `PENDING_WMS` until acceptance. WMS rejection must return authoritative quantity/status and retain the PDA draft for resolution. Receiving inventory effects require explicit ownership; no duplicate local and WMS stock adjustments.

### Putaway

Required WMS fields: task, source, destination policy/capacity, item, quantity, lot/serial/LPN, assignment, version. Source/item/destination validations may use local projections only when freshness and policy permit; otherwise query WMS-backed projection. Confirmation emits one idempotent command and waits for WMS result if WMS owns inventory. Cache invalidation covers task, balances, dashboard, and movement.

### Picking

Required WMS fields: order, customer, line, expected location, item/barcode, lot/serial, reserved quantity, quantity to pick, short-pick policy/reasons, task version, shipment-readiness impact. Location/item scans validate context; pick/short/completion commands must not over-consume a WMS reservation. WMS rejection returns authoritative remaining/reservation state. Shipment readiness is refreshed after accepted pick events.

### Replenishment

Required fields: demand, task, source/destination, item/UOM, available/capacity, quantity remaining, partial policy, version. Backend supports partial completion locally. WMS must define whether each partial confirmation is an enterprise movement or only a task progress notification. No implicit final completion is allowed.

### Inventory inquiry and transfer

Inventory search/balances/movements are projections with freshness metadata; Redis and PDA Room are never authoritative. Transfer requires source/destination/item validation, quantity, lot/serial/LPN dimensions, base version, and idempotency. If WMS owns balances, `StockTransferConfirmed` is a command request and backend must expose `PENDING_WMS` until a result. A lost acknowledgement is reconciled by command ID and WMS transaction ID.

### Cycle count

Required fields: count plan/task, location, item, blind-count policy, system quantity visibility, counted quantity/UOM, variance, recount/review/approval state, version. Count submission is evidence and does not adjust inventory locally. WMS/approved authority decides approval and adjustment. `CycleCountReviewRequired`, accepted, and rejected results must be explicit.

### Shipping

Required fields: shipment/order, package IDs and barcode aliases, package contents/status, readiness blockers, carrier, tracking, version. Package verification is not final shipment confirmation. Final confirmation is online-only, idempotent, and requires fresh readiness. Current backend has no real manifest/label printing subsystem. WMS result must include shipment status, transaction ID, package state, and authoritative error/retryability.

## 13. Database and Projection Map

| State | Current table/package | Target WMS integration change |
|---|---|---|
| Task | `warehouse_task`, execution PostgreSQL store | add WMS source/version/reconciliation metadata or separate projection after ownership approval |
| Receiving | receiving task/line tables | map inbound event fields and WMS transaction/result IDs |
| Movement | `movement_task`, movement repository | add source event/version and WMS task identity |
| Inventory | balance/movement tables | distinguish WMS projection version from PDA command ledger; define authority |
| Count | count task/line tables | store WMS plan/review/approval IDs |
| Shipping | shipment/package tables | store WMS shipment/package/transaction IDs |
| Outbox | `domain_outbox` and `000006_event_delivery` | add event schema/source metadata and acknowledgement correlation as approved |
| Inbox | `event_inbox` | add payload hash, source event version, received topic/partition/offset, status |
| DLQ | `event_dlq` | preserve original Kafka metadata and safe failure classification |
| Reconciliation | not implemented | add mismatch/checkpoint/snapshot/replay tables |
| Cache | Redis adapters/key service | invalidate only after committed projection transaction |

## 14. Acknowledgement and Result Model

Each outbound command that WMS must process carries `commandId`, `idempotencyKey`, `correlationId`, aggregate ID/version, warehouse, operator/device context where applicable, command type, requested quantities/dimensions, and created time. WMS results must return `commandId`, WMS transaction ID, result event ID, aggregate ID, accepted/current version, authoritative quantities/status, error code, retryable flag, and processed time.

Target status model: `PENDING_WMS`, `ACKNOWLEDGED`, `CONFLICT`, `REJECTED`, `PERMANENT_FAILURE`. Current PDA API-024 normalizes durable local command results to `ACKNOWLEDGED`, `CONFLICT`, or `PERMANENT_FAILURE`; it does not yet persist a generic `PENDING_WMS` state. This is a required implementation gap before asynchronous WMS acknowledgements are exposed uniformly.

## 15. Retry, DLQ, and Poison Events

Retryable: broker timeout, temporary database unavailability, dependency unavailable, consumer rebalance interruption, and explicitly retryable WMS result. Non-retryable: invalid JSON/schema, missing required IDs, unsupported event version, invalid signature/authentication, impossible negative quantity, and policy rejection.

The DLQ record must retain source topic, partition, offset, message key, headers, event ID, event type/version, schema ID, aggregate/version, received time, attempt count, failure class, redacted error, and original payload. The current `event_dlq` schema lacks topic/partition/offset/headers and must be extended or supplemented before production replay. Replay must revalidate schema, preserve event ID, use a replay marker, and be authorized by operations. Replaying a poison event without fixing the cause is prohibited.

## 16. Kafka Security and ACLs

Production requires TLS 1.2+, CA trust, broker hostname validation, mTLS/SASL as required by the platform, secret-manager storage, and certificate rotation. Current code supports `PLAINTEXT` or `TLS` with CA and optional client certificate/key; it does not implement SASL configuration. Local shared broker tests are not production security evidence.

| Principal | Topic/group | Permission | Reason | Status |
|---|---|---|---|---|
| `pda-backend-producer` | `pda.*.events.v1` | write/describe | publish committed PDA outbox events | PROPOSED |
| `pda-wms-consumer` | `wms.*.events.v1` | read/describe | consume WMS projections | PROPOSED |
| `pda-wms-consumer` | `pda-wms-*` groups | group read | consumer membership | PROPOSED |
| `pda-dlq-writer` | integration DLQ | write | preserve poison events | PROPOSED |
| `wms-consumer` | `pda.*.events.v1` | read/describe | WMS command/event processing | PROPOSED |
| operations replay identity | DLQ/replay topics | read/write only by approval | controlled replay | PROPOSED |

No secrets, certificates, or ACL credentials belong in this document or repository.

## 17. Observability and Capacity

Required metrics: `consumer_lag`, `event_processing_latency`, `event_age`, `inbox_duplicates_total`, `out_of_order_events_total`, `version_gap_total`, `projection_failures_total`, `dlq_messages_total`, `outbox_backlog`, `outbox_oldest_age`, `publish_failures_total`, `wms_ack_pending_total`, `reconciliation_mismatch_total`, `barcode_resolution_failures_total`, and `unknown_barcode_total`.

Every log/trace must include event ID, aggregate ID/version, warehouse, topic, partition, offset, correlation ID, causation ID, command ID, and safe error code. Never log raw tokens, passwords, full sensitive barcodes, or private operator data.

No business throughput or PDA count is available in this repository. Capacity must be measured using active PDA count, peak scans/minute, mutations/minute, WMS events/minute, snapshot size, inventory event volume, consumer concurrency, partitions, retention, and replay target. Size with formulas, then load test hot warehouse keys, inventory snapshots, task bursts, reconnect storms, and outage backlogs.

## 18. Failure Matrix

| Failure | PDA API behavior | Database | Kafka | Operator visibility | Recovery |
|---|---|---|---|---|---|
| Kafka unavailable | mutation may commit with publication-pending retryable result | outbox remains unpublished | worker backoff | command/event pending metric | restore broker and drain outbox |
| WMS producer stopped | serve last-known reads with freshness; block unsafe writes when stale | projection unchanged | lag/age rises | stale banner/alert | restart producer, snapshot/replay |
| Backend consumer stopped | existing projections serve last known state | no new inbox rows | consumer lag | readiness/lag alert | restart consumer |
| Schema invalid | safe error for consumer; no projection | DLQ metadata | offset commit only after DLQ | DLQ alert | fix producer/schema and replay |
| Duplicate event | no-op same event/hash | inbox unique | offset committed | duplicate metric | none |
| Version gap | do not overwrite | park/gap record | retry or quarantine | reconciliation alert | replay/snapshot |
| DB unavailable | reads/mutations fail safely | transaction rollback | offsets not committed | dependency error | restore DB |
| Redis unavailable | fall back to PostgreSQL | authoritative state unaffected | none | cache error metric | restore Redis |
| WMS rejects command | return/reconcile rejection | retain command/audit | result event consumed | actionable error | operator resolve/retry per code |
| Ack lost | API-024 remains pending/unknown | command retained | result/reconciliation lookup | pending alert | query WMS/replay safely |
| Backend restart | resume outbox/consumer from durable state | state preserved | group resumes | no fabricated success | restart and reconcile |
| Full projection rebuild | serve controlled stale/blocked state | build separate projection | replay from checkpoint | maintenance/readiness signal | validate checksums and swap |

## 19. Testing Strategy

### Unit

Envelope validation, WMS-to-projection mapping, barcode normalization, GS1 parsing, key generation, version comparison, duplicate/hash detection, error classification, cache invalidation, and command result mapping.

### Contract

One golden schema and example for each inbound/outbound family; required fields, enums, additive compatibility, event keys, topic names, result/error codes, and correlation/causation behavior. Use a schema registry or approved equivalent. JSON is current backend transport; Avro/Protobuf is a WMS platform decision, not assumed here.

### Kafka integration

Real broker producer/consumer, TLS/ACL denial, duplicate, ordering, partition key, rebalance, restart, retry, DLQ, replay, backlog, lag, and outbox recovery. Current `make test-kafka` proves local shared PLAINTEXT behavior only.

### PostgreSQL integration

Inbox/projection atomic transaction, duplicate no-op, different-payload duplicate alert, version gap, snapshot checkpoint, reconciliation mismatch, outbox publication, concurrent consumers, restart durability, and DLQ replay.

### Full E2E

```text
WMS snapshot/event -> Kafka -> PDA projection -> PDA REST -> PDA App
-> Zebra scan -> PDA command -> PostgreSQL/outbox -> Kafka -> WMS result
-> PDA reconciliation -> API-024 -> PDA App refresh
```

Run receiving, putaway, picking, replenishment, transfer, cycle count, and shipping scenarios with real WMS staging data. A simulator may supplement tests but cannot satisfy production verification.

## 20. WMS Contract Checklist

- [x] WMS OpenAPI lists current master-data, inventory, inbound, and outbound APIs backed by separate WMS databases.
- [x] WMS AsyncAPI defines seven WMS-owned Kafka event channels.
- [x] WMS event envelope and the PDA Backend envelope mismatch are documented.
- [x] WMS inventory and inbound publication gaps are documented: inventory publishes none and inbound publishes none.
- [ ] PDA Backend adds authenticated adapters for WMS master-data, inventory, inbound, and outbound APIs.
- [ ] PDA Backend adds a WMS Kafka consumer, inbox/checkpoint/replay handling, and typed projections.
- [ ] PDA/WMS ownership and command/result contracts are approved for PDA workflows.
- [ ] Secure Kafka ACL/TLS staging and PDA-to-WMS end-to-end evidence are supplied.

- [ ] Kafka bootstrap servers and DNS supplied.
- [ ] TLS/SASL/mTLS mode, CA, server names, and rotation process supplied.
- [ ] Producer/consumer principals and ACL matrix approved.
- [ ] Topic names, partition keys, partitions, retention, and compaction approved.
- [ ] Envelope and payload schemas approved in a registry or governed repository.
- [ ] Event-version and compatibility policy approved.
- [ ] Warehouse/item/location/barcode/UOM ownership approved.
- [ ] Task/receiving/movement/count/shipping ownership approved.
- [ ] Inventory authority and adjustment policy approved.
- [ ] Operator/device/task assignment mapping approved.
- [ ] GS1 parsing responsibility and supported AIs approved.
- [ ] Initial snapshots, high-water marks, replay, and checkpoint process approved.
- [ ] WMS acknowledgement/result event catalog approved.
- [ ] Idempotency and ordering guarantees approved.
- [ ] Reconciliation APIs/events and mismatch ownership approved.
- [ ] DLQ ownership and authorized replay process approved.
- [ ] Lag/backlog SLA and alert thresholds approved.
- [ ] Staging endpoint, credentials, test data, and E2E window available.

## 21. Implementation Backlog

| Phase | Work item | Backend area | WMS dependency | Acceptance |
|---|---|---|---|---|
| 00 | Freeze ownership, authority, event names, and schemas | architecture/contracts | WMS sign-off | approved contract pack |
| 01 | Configure topics, schema registry, TLS, ACL, groups | integration/kafka/config | platform credentials | secure handshake and ACL tests |
| 02 | Add snapshot/checkpoint/replay tables and master projections | new WMS projection package | master snapshots/events | checksum and rebuild test |
| 03 | Add task/receiving/outbound projections | execution/receiving/movement | workflow ownership | event-to-REST contract tests |
| 04 | Define inventory authority and projection/reconciliation | inventory | balance/reservation contract | mismatch and no-auto-adjust tests |
| 05 | Integrate receiving commands/results | receiving/outbox | inbound result events | accept/reject/timeout E2E |
| 06 | Integrate putaway/picking/replenishment | movement/outbox | task/result events | version/idempotency E2E |
| 07 | Integrate transfer and cycle count | inventory/count | adjustment/review policy | approval/recount E2E |
| 08 | Integrate shipment/package results | shipping | shipping contract | readiness/ack E2E |
| 09 | Implement shared barcode/QR/GS1 resolution | scanner/WMS anti-corruption | alias/GS1 policy | symbology and malformed scan tests |
| 10 | Implement DLQ, replay, reconciliation, lag alerts | integration/ops | replay access | controlled recovery drill |
| 11 | Secure staging and PDA/Zebra E2E | all | real environments | production readiness sign-off |

## 22. Appendices

### Appendix A - PDA workflow to WMS data matrix

| Workflow | WMS data needed | Backend projection | Outbound command |
|---|---|---|---|
| Receiving | inbound/order/line/item/policy | receiving tables plus WMS metadata | receipt/complete |
| Putaway | task/source/destination/capacity/item | movement task and inventory | putaway/move |
| Picking | order/reservation/location/item/short policy | movement task and shipment impact | pick/short/complete |
| Replenishment | demand/source/destination/capacity | movement task | replenishment |
| Inventory | balance/reservation/lot/serial/owner | inventory tables | transfer/count |
| Shipping | shipment/package/readiness/carrier/tracking | shipping tables | package verify/confirm |

### Appendix B - Event field requirements

| Event type | Mandatory payload baseline | Version/order |
|---|---|---|
| Master create/update | entity ID, warehouse, status, changed fields/full snapshot, source version, occurred time | entity version |
| Task update | task ID/type/status/assignment/priority/version, workflow fields | task version |
| Inventory change | balance identity, delta or absolute quantity, UOM, lot/serial/LPN/owner, source version | balance version |
| Command result | command ID, WMS transaction ID, aggregate ID/version, result, authoritative quantities/status, error/retryability | result sequence |

The seven event types in section 7.2 are `APPROVED_WMS_EVENT` at the channel/envelope baseline, but their payload-to-PDA mappings are not implemented. The task, inventory-change, command-result, and shipment event families in this appendix remain `PROPOSED_WMS_EVENT` until WMS and PDA approve names, nullability, units, enum values, ownership, and compatibility behavior.

### Appendix C - Projection/cache invalidation map

| Inbound change | PostgreSQL projection | Redis/PDA API invalidation |
|---|---|---|
| item/barcode | item/alias projection | receiving, picking, replenishment, inventory, count |
| location/capacity | location projection | movement, inventory |
| task | task projection | dashboard, summary, task lists/details |
| inventory | balance/ledger projection | balances, search, movements, task validation |
| shipment/package | shipment projection | shipment/readiness, task, dashboard |

Invalidate only after the projection transaction commits.

### Appendix D - Open decisions

1. Does WMS or PDA Backend own each task aggregate and assignment?
2. Is WMS the inventory authority, and are PDA transactions commands or local operational commitments?
3. Which WMS event schemas, topic names, keys, partitions, and retention are approved?
4. Is the WMS contract JSON, Avro, or Protobuf, and which registry governs compatibility?
5. Which WMS acknowledgements are required for each mutation?
6. What are WMS transaction IDs and idempotency semantics?
7. How are snapshots and event high-water marks coordinated?
8. Which GS1 AIs and scanner symbologies are enabled by warehouse?
9. Are LPN/pallet, ownership, condition, and expiry workflows in scope?
10. What is the correction policy for inventory mismatch and count variance?
11. What is the WMS staging endpoint, TLS identity, ACL principal, and test data?
12. What are lag, outbox age, and reconciliation SLA thresholds?

## 23. Final Validation

- [x] Current PDA APIs, routes, application services, migrations, outbox, inbox, DLQ, Kafka adapters, WMS port, HTTP/mock WMS adapters, config, Compose, tests, and reports inspected.
- [x] PDA workflows mapped to WMS data dependencies.
- [x] Current and proposed event families separated.
- [x] Current topics, keys, envelope, consumer group, outbox, inbox, and DLQ behavior documented.
- [x] Duplicate, stale, out-of-order, missing-event, snapshot, replay, and reconciliation behavior defined.
- [x] Barcode, QR, GS1, scanner contexts, and ownership gaps documented.
- [x] Inventory authority and cache/Room boundaries defined.
- [x] Security, ACL, TLS, observability, capacity, failure, and testing requirements defined.
- [x] Current mock WMS behavior and local PLAINTEXT Kafka limitation identified.
- [x] Verified WMS APIs, databases, service ownership, event topics, and envelope baseline added from `ricoh-wms`.
- [x] Proposed PDA/WMS fields/events clearly labeled separately from the verified WMS contract.
- [x] WMS implementation backlog created.
- [ ] PDA/WMS ownership and workflow command/result contract supplied.
- [ ] PDA WMS API adapters and Kafka WMS consumers implemented.
- [ ] Secure Kafka staging verified.
- [ ] Real WMS and PDA/Zebra E2E verified.

**Final readiness: WMS_EVENT_BASELINE_VERIFIED; PDA_ADAPTER_AND_WORKFLOW_CONTRACT_BLOCKED; SECURE_STAGING_AND_E2E_BLOCKED.** The current WMS APIs/events are documented and backed by WMS services/databases, but PDA Backend does not yet consume or expose them beyond warehouse discovery. This document is not evidence that PDA-to-WMS integration is complete.
