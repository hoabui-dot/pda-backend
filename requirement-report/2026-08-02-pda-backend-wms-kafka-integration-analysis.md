# PDA Backend to WMS Kafka Integration Analysis

- Date: 2026-08-02
- Status: **SPECIFICATION COMPLETE; IMPLEMENTATION BLOCKED BY WMS CONTRACT AND SECURE STAGING**
- Main document: `docs/backend-integration/PDA_BACKEND_WMS_KAFKA_EVENT_INTEGRATION_SPECIFICATION.md`
- Production code modified: No

## Files and Packages Inspected

- Master prompt: `requirement/Generate-the-Complete-PDA-Backend-to-WMS-Kafka-Event-Integration-Specification.md`
- Public API/OpenAPI: `api/openapi/pda-v1.yaml`, `internal/gateway/adapters/http/router.go`
- PDA integration contract: `docs/integration-pda-app/PDA_APP_API_SPECIFICATION.md`
- Reconciliation rules and phases 00-11 under `docs/integration-pda-app/`
- All corresponding reconciliation reports and Phase 08/09/11 WMS/Kafka reports under `requirement-report/`
- Gap report: `PDA_BACKEND_PDA_APP_INTEGRATION_GAP_REPORT.md`
- Configuration/runtime: `internal/platform/config/config.go`, `cmd/pda-api-gateway/main.go`, `cmd/integration-event-service/main.go`, `docker/compose.yml`, `Makefile`, Dockerfiles, README, architecture docs
- Event infrastructure: `internal/platform/event`, `internal/platform/messaging`, `internal/integration/adapters/kafka`, migration `000006_event_delivery`
- WMS boundary: `internal/integration/ports/upstream_wms.go`, `wmshttp`, `wmsmock`, `resilient`
- Workflow/application/domain/PostgreSQL packages for execution, receiving, movement, inventory, shipping, identity, cache, and audit
- Kafka unit/integration/security tests and workflow integration tests

## Findings

1. PDA Backend supports authentication, dashboard/tasks, receiving, putaway, picking, replenishment, inventory/transfer, cycle count, and shipping REST workflows.
2. PDA mutations persist PostgreSQL state, idempotency, audit, and outbox records before publication.
3. Current Kafka topics are `pda.task.events.v1`, `pda.receiving.events.v1`, `pda.movement.events.v1`, `pda.inventory.events.v1`, and `pda.shipping.events.v1`.
4. Current Kafka publisher uses `aggregateId` as key and `PDA_KAFKA_TOPIC_PREFIX`; local shared PLAINTEXT Kafka tests pass.
5. Generic Kafka consumer/inbox/DLQ infrastructure exists, but no WMS event consumer or workflow projection handler is wired.
6. `UpstreamWMS` currently exposes only `Warehouses`; `wmshttp` implements only warehouse master discovery.
7. WMS workflow ownership, event schemas, snapshots, replay, acknowledgements, and reconciliation contracts are not approved.
8. WMS fixtures are runtime-reachable in explicit mock mode and are not real WMS evidence.
9. Kafka TLS configuration is fail-closed and implemented, but secure staging ACL/TLS/rebalance/DLQ evidence is unavailable.
10. Barcode and scanner context fields exist in selected REST handlers, but shared GS1 parsing and WMS barcode master integration do not exist.

## Required WMS Decisions

- Entity ownership for tasks, receiving, movement, inventory, counts, shipments, packages, and operator assignments.
- Authoritative inventory and reservation model.
- WMS event envelope, schema format/registry, topic/key/partition/retention policy.
- Snapshot/high-water-mark/replay and reconciliation APIs/events.
- PDA command acknowledgement/result event semantics and WMS transaction IDs.
- Barcode/QR/GS1 parser ownership, enabled symbologies, and alias master.
- Kafka TLS/SASL/mTLS, principals, ACLs, staging broker, and alert thresholds.

## Status

The specification is implementation-ready for WMS contract review and backlog planning. It must not be used to claim production WMS/Kafka integration, because the source repository contains no approved WMS workflow event contract and no secure external staging evidence.
