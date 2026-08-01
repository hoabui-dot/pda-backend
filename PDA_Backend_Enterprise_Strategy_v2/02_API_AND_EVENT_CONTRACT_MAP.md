# PDA Backend — API, Cache, Database, and Event Contract Map

- **Document ID:** `PDA-BE-CONTRACT-001`
- **Status:** proposed backend contract; not yet implemented
- **Rule:** endpoint names may be adjusted only through versioned OpenAPI review.

---

## 1. Public PDA API Base

```text
/api/pda/v1
```

Gateway routes must preserve this public contract even if internal service routes differ.

---

## 2. Authentication and Bootstrap

| PDA action | Method/path | Owner |
|---|---|---|
| Demo login | `POST /auth/login` | Identity |
| Token refresh | `POST /auth/refresh` | Identity |
| Logout | `POST /auth/logout` | Identity |
| Current profile | `GET /me` | Identity |
| Warehouses | `GET /me/warehouses` | Identity |
| Device registration | `POST /devices/registrations` | Identity |
| Bootstrap | `GET /bootstrap` | Gateway/BFF composition |

Mock login returns deterministic credentials only in non-production mode.

---

## 3. Dashboard and Task Center

| Screen | Method/path |
|---|---|
| Dashboard | `GET /dashboard` |
| Task Center category counts | `GET /tasks/summary?status=` |
| Task search/list | `GET /tasks?category=&status=&cursor=&q=` |
| Claim task | `POST /tasks/{taskId}/claim` |
| Release task | `POST /tasks/{taskId}/release` |

Cache:

```text
dashboard:{warehouse}:{operator}
task-summary:{warehouse}:{operator}:{status}
```

Events that invalidate:

- `TaskAssigned`
- `TaskStarted`
- `TaskCompleted`
- `ReceivingQuantityConfirmed`
- `InventoryMoved`
- `ShipmentConfirmed`

---

## 4. Receiving

| Action | Method/path |
|---|---|
| List | `GET /receiving/tasks` |
| Detail | `GET /receiving/tasks/{taskId}` |
| Start/claim | `POST /receiving/tasks/{taskId}/start` |
| Resolve barcode | `POST /receiving/tasks/{taskId}/barcode-resolutions` |
| Confirm quantity | `POST /receiving/tasks/{taskId}/receipts` |
| Complete | `POST /receiving/tasks/{taskId}/completion` |

Confirm request:

```json
{
  "commandId": "uuid",
  "lineId": "LINE-01",
  "barcode": "00012345678905",
  "quantity": 3,
  "remark": "Good condition",
  "baseVersion": 11
}
```

Events:

- `ReceivingTaskStarted`
- `ReceivingBarcodeResolved`
- `ReceivingQuantityConfirmed`
- `ReceivingTaskCompleted`
- `InventoryBalanceChanged`

---

## 5. Putaway

| Action | Method/path |
|---|---|
| List | `GET /putaway/tasks` |
| Detail | `GET /putaway/tasks/{taskId}` |
| Validate source | `POST /putaway/tasks/{taskId}/source-validations` |
| Destination suggestions | `GET /putaway/tasks/{taskId}/destination-suggestions` |
| Validate destination | `POST /putaway/tasks/{taskId}/destination-validations` |
| Confirm | `POST /putaway/tasks/{taskId}/confirmation` |

Events:

- `PutawayTaskStarted`
- `PutawayDestinationValidated`
- `InventoryMoved`
- `PutawayTaskCompleted`

---

## 6. Picking

| Action | Method/path |
|---|---|
| List | `GET /picking/tasks` |
| Detail/current line | `GET /picking/tasks/{taskId}` |
| Validate location | `POST /picking/tasks/{taskId}/location-validations` |
| Resolve item | `POST /picking/tasks/{taskId}/barcode-resolutions` |
| Confirm pick | `POST /picking/tasks/{taskId}/picks` |
| Complete | `POST /picking/tasks/{taskId}/completion` |

Events:

- `PickingTaskStarted`
- `PickConfirmed`
- `PickingTaskCompleted`
- `ShipmentReadinessChanged`

---

## 7. Replenishment

| Action | Method/path |
|---|---|
| List | `GET /replenishment/tasks` |
| Detail | `GET /replenishment/tasks/{taskId}` |
| Validate source/destination/item | nested validation resources |
| Confirm quantity/move | `POST /replenishment/tasks/{taskId}/confirmation` |

Events:

- `ReplenishmentConfirmed`
- `InventoryMoved`
- `ReplenishmentTaskCompleted`

---

## 8. Inventory Inquiry

| Action | Method/path |
|---|---|
| Search | `GET /inventory/search?q=` |
| Balances | `GET /inventory/balances?itemId=&locationId=&lot=&serial=` |
| History | `GET /inventory/movements?itemId=&locationId=&cursor=` |

Cache TTL must be short and response must include `asOf`.

Events invalidate item/location keys:

- `InventoryBalanceChanged`
- `InventoryMoved`
- `CycleCountAccepted`

---

## 9. Stock Transfer

| Action | Method/path |
|---|---|
| Validate source | `POST /inventory/transfers/source-validations` |
| Validate destination | `POST /inventory/transfers/destination-validations` |
| Resolve item | `POST /inventory/transfers/item-resolutions` |
| Confirm | `POST /inventory/transfers` |
| Command status | `GET /inventory/transfers/commands/{commandId}` |

Events:

- `StockTransferConfirmed`
- `InventoryMoved`

---

## 10. Cycle Count

| Action | Method/path |
|---|---|
| List | `GET /cycle-count/tasks` |
| Detail | `GET /cycle-count/tasks/{taskId}` |
| Validate location/item | nested validation resources |
| Submit count | `POST /cycle-count/tasks/{taskId}/counts` |
| Request recount | `POST /cycle-count/tasks/{taskId}/recounts` |
| Complete | `POST /cycle-count/tasks/{taskId}/completion` |

Events:

- `CycleCountSubmitted`
- `CycleCountVarianceDetected`
- `CycleCountRecountRequested`
- `CycleCountCompleted`

A count event must not directly imply an inventory adjustment unless the approved WMS process explicitly accepts it.

---

## 11. Shipping Confirmation

| Action | Method/path |
|---|---|
| Summary | `GET /shipments/{shipmentId}` |
| Readiness | `GET /shipments/{shipmentId}/readiness` |
| Confirm | `POST /shipments/{shipmentId}/confirmation` |
| Command status | `GET /shipments/{shipmentId}/commands/{commandId}` |

Events:

- `ShipmentReadinessChanged`
- `ShipmentConfirmed`
- `OrderShipped`

Shipping confirmation is online-only by default.

---

## 12. Database Tables — Initial Logical Model

Execution database:

```text
warehouse_task
task_assignment
receiving_task
receiving_line
putaway_task
picking_task
picking_line
replenishment_task
cycle_count_task
cycle_count_line
command_idempotency
domain_outbox
audit_record
```

Inventory database/schema:

```text
item
warehouse_location
inventory_balance
inventory_reservation
inventory_movement
stock_transfer
```

Shipping database/schema:

```text
shipment
shipment_package
shipment_readiness
shipment_confirmation
```

Integration:

```text
processed_inbox_event
outbox_publish_attempt
dead_letter_record
mock_event_log
wms_sync_checkpoint
```

---

## 13. Event-to-Consumer Map

| Event | Consumers |
|---|---|
| ReceivingQuantityConfirmed | inventory projection, audit, WMS integration |
| InventoryMoved | dashboard projection, inventory cache invalidator, audit |
| PickingTaskCompleted | shipping readiness, task summary, WMS integration |
| CycleCountVarianceDetected | supervisor workflow, audit |
| ShipmentConfirmed | dashboard, WMS integration, audit |
| MasterDataChanged | item/location cache invalidator |
| TaskAssigned | dashboard/task cache invalidator |

---

## 14. Mock Fixture Contract

Directory:

```text
mock-data/
  operators.json
  warehouses.json
  items.json
  locations.json
  receiving-tasks.json
  putaway-tasks.json
  picking-tasks.json
  replenishment-tasks.json
  cycle-count-tasks.json
  shipments.json
```

Rules:

- deterministic IDs;
- explicit fixture version;
- no random current timestamps in assertions;
- reset endpoint only in local/test profile;
- fixtures mapped through the same domain ports as real WMS data;
- production profile must refuse mock mode.

---

## 15. Error Codes

Minimum codes:

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
UPSTREAM_WMS_UNAVAILABLE
MESSAGING_PUBLISH_PENDING
RATE_LIMITED
INTERNAL_ERROR
```
