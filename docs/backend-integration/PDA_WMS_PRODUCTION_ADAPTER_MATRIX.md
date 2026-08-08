# PDA-WMS Production Adapter Matrix

Date: 2026-08-07
Status: `PARTIALLY_IMPLEMENTED_WITH_EXPLICIT_OWNER_GATES`

| PDA Operation | PDA Route | WMS Owner | WMS Route | Method | Mapping | Auth | Idempotency / Version | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Warehouse context | `/bootstrap` | Master Data | `/api/wms/master-data/warehouses` | GET | Warehouse list is mapped by `wmshttp.Client` | Bearer plus PDA service identity | Read-only | OWNER_API_EXISTS_ADAPTER_COVERED |
| Barcode resolution | Receiving barcode workflow | Master Data | `/api/wms/master-data/barcode/resolve` is called by the HTTP Receiving adapter with warehouse/task/line context and scanner symbology | POST | Item revision and UOM are mapped from the authoritative resolver; lot/serial-specific lookup remains owner-contract dependent | Service identity propagated | Scan id propagated | ADAPTER_IMPLEMENTED_ITEM_RUNTIME_PENDING |
| Task discovery | `/tasks` | Warehouse Execution | `/api/wms/execution/tasks` | GET | Warehouse-scoped task list/detail mapping | Service identity propagated | Read-only | OWNER_API_EXISTS_ADAPTER_COVERED |
| Task detail/claim/release | `/tasks/{taskId}` and commands | Warehouse Execution | `/api/wms/execution/tasks/{id}`, command routes | GET/POST | Task detail and claim/release map to owner commands | Service identity and operator propagated | Owner version/idempotency propagated | OWNER_API_EXISTS_ADAPTER_COVERED |
| Receiving discovery/detail/confirm | `/receiving/...` | Inbound / Receiving | `/api/wms/inbound/receipts`, `/receipts/{id}`, quantity, `/confirm` | GET/POST | Existing receipt is mapped; no create-on-confirm path | Service identity and operator propagated | Line version and idempotency propagated | OWNER_API_EXISTS_ADAPTER_COVERED |
| Putaway | `/putaway/...` | Warehouse Execution | `/api/wms/execution/tasks`, scans, confirm | GET/POST | Task list/detail, source/destination scans, and quantity confirm map | Service identity propagated | Owner version/idempotency propagated | OWNER_API_EXISTS_ADAPTER_COVERED |
| Picking | `/picking/...` | Warehouse Execution + Inventory | `/api/wms/execution/tasks`, `/tasks/{id}/allocate`, scans, confirm | GET/POST | Task reads, allocation, scans, and confirm map; allocation remains owned by Warehouse Execution/Inventory | Service identity and operator propagated | Allocation command/idempotency plus owner version propagated | ADAPTER_IMPLEMENTED_RUNTIME_PENDING |
| Replenishment | `/replenishment/...` | Warehouse Execution + Inventory | `/api/wms/execution/tasks`, scans, confirm | GET/POST | Task reads/scans/confirm map | Service identity propagated | Owner version/idempotency propagated | OWNER_API_EXISTS_ADAPTER_COVERED |
| Inventory inquiry | `/inventory/...` | Inventory | `/api/wms/inventory/balances`, `/movements` | GET | Authoritative balance and movement mapping | Service identity propagated | Read-only | OWNER_API_EXISTS_ADAPTER_COVERED |
| Internal transfer | `/inventory/transfers` | Inventory | `/api/wms/inventory/movements/internal-transfer` | POST | Lot-scoped transfer maps to owner command | Service identity and operator propagated | Command ID and idempotency propagated | OWNER_API_EXISTS_ADAPTER_COVERED |
| Cycle count lifecycle | `/counts`, `/counts/{taskId}`, submit/recount/complete | Inventory | `/api/wms/inventory/cycle-counts`, detail, submit, recount, complete, approval commands | GET/POST | Warehouse-scoped owner read, submission, durable recount reset, and completion map to PDA count model; blind count hides snapshot quantity | Service identity and operator propagated | Every command has base version, request hash, durable idempotency result | ADAPTER_IMPLEMENTED_RUNTIME_PENDING |
| LPN/pallet | No active PDA production route | Shipping | Owner routes exist in WMS; current PDA API specification explicitly defers LPN/pallet workflow | GET/POST | No adapter intentionally added without an approved PDA workflow contract | N/A | N/A | DEFERRED_BY_APPROVED_PDA_SCOPE |
| Package verification | `/shipments/{id}/packages/{id}/verify` | Shipping | `/api/wms/shipping/packages/{id}/verify` then `/shipments/{id}` | POST/GET | Adapter validates package membership against the authoritative shipment, invokes owner verification, then reloads shipment readback | Service identity propagated | Deterministic owner command ID, version propagated | ADAPTER_IMPLEMENTED_RUNTIME_PENDING |
| Shipment confirmation | `/shipments/{id}/confirm` | Shipping | `/api/wms/shipping/shipments/{id}/confirm` | POST | Adapter reads authoritative carrier/tracking, rejects conflicting PDA mirrors, invokes owner command without duplicating business fields, then reloads shipment | Service identity propagated | Owner command idempotency/version exists | ADAPTER_IMPLEMENTED_RUNTIME_PENDING |

## Findings

The PDA gateway now exposes gateway-level use-case ports and composes remote
WMS adapters in HTTP mode. Local business stores are constructed only in mock
mode. The current composition uses one configured WMS gateway base URL because
that is the repository's existing public contract; it does not invent separate
owner URLs. Service bearer and PDA service identity headers are propagated.

Receiving is mapped to existing Inbound receipts, quantity commands, and
confirmation. It does not create a receipt during confirmation. The HTTP
adapter now delegates item barcode resolution to the existing Master Data
resolver, preserving scanner symbology and scan context; lot/serial-specific
resolution still requires an owner contract if the current receipt workflow
needs it. Package verification uses the existing Shipping command plus an
authoritative shipment readback. Shipment confirmation now uses the existing
owner command: carrier/tracking remain authoritative on the Shipment aggregate,
PDA mirrors are checked for conflicts, and no duplicate business fields are
sent. Cycle-count mutation, picking allocation/short-pick, and LPN/pallet PDA
routes remain separate owner/adapter gates, not reasons to reintroduce local
business authority.
