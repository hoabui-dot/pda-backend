# PDA Backend Reconciliation Report — Phase 10

Date: 2026-08-02
Status: APPROVED

## Scope

Phase 10 verifies Redis cache keys, TTL behavior, freshness metadata, mutation invalidation, Redis outage behavior, and operational observability for the PDA API.

## Cache contract

The shared Redis cache is versioned with `pda:v1:<kind>:<parts>`. Parts replace `:` with `_`, so tenant and warehouse boundaries remain explicit and deterministic.

| Read | Key | TTL | Freshness |
|---|---|---:|---|
| Dashboard | `pda:v1:dashboard:<warehouse>:<operator>` | `PDA_CACHE_TTL` (default 30s) | `meta.asOf`, `meta.stale` |
| Task summary | `pda:v1:task-summary:<warehouse>:<operator>:<status>` | same | `meta.asOf`, `meta.stale` |
| Inventory search | `pda:v1:inventory-search:<warehouse>:<query>` | same | `meta.asOf`, `meta.stale` |
| Inventory balance | `pda:v1:inventory-balance:<warehouse>:<item>:<location>` | same | `meta.asOf`, `meta.stale` |
| Shipment | `pda:v1:shipment:<shipmentId>` | same | shipment `asOf`, `meta.asOf`, `meta.stale` |
| Warehouse master | `pda:v1:master:warehouses` | same | `meta.serverTime` |

Task/workflow lists and details are currently database-backed and still return `serverTime`, `asOf`, and `stale:false`; they do not claim a Redis hit. Shipment IDs are globally scoped by the WMS contract, so shipment cache keys do not require a second warehouse component.

## Invalidation matrix

After a successful database transaction, invalidation is best effort and never changes the mutation result:

| Mutation | Invalidated views |
|---|---|
| Receiving start, barcode resolution, receipt, completion | dashboard, task summary, inventory projections, shipment projections |
| Putaway, picking, replenishment | dashboard, task summary, inventory projections, shipment projections |
| Inventory transfer | dashboard, task summary, inventory search/balances, shipment projections |
| Count submit, recount, completion | dashboard, task summary, inventory search/balances, shipment projections |
| Package verification | shipment projections |
| Shipment confirmation | shipment projections, dashboard/task projections |

The invalidator records success/failure counters for each Redis pattern operation. A Redis outage can cause a stale read or a cache miss, but cannot roll back or turn a committed mutation into a failed business operation.

## Freshness and synchronization ownership

Every PDA list response includes `meta.serverTime`; cached or projected reads also include `asOf` and `stale`. When the source cannot be loaded and a prior value exists, the backend serves that value with `stale:true` and increments `cache.stale_served`.

`pendingSyncCount` is PDA-owned. It counts durable Room/WorkManager commands that have not reached a terminal command status. The server must not fabricate or overwrite it in dashboard projections. Server-side command backlog and Kafka lag are separate operational metrics.

## Metrics and dashboard specification

Cache metrics: `cache.hits`, `cache.misses`, `cache.errors`, `cache.latency_nanos`, `cache.stale_served`, `cache.invalidation_success`, and `cache.invalidation_failure`.

Messaging metrics: `messaging.published`, `messaging.failed`, `messaging.backlog`, and `messaging.lag_seconds` (consumer event age). Command backlog is the count of non-terminal command/outbox records grouped by warehouse and command type.

Dashboard panels and alert guidance:

1. Cache hit ratio: `hits / (hits + misses)` by cache kind; warn below 0.70 for 15 minutes.
2. Cache errors and invalidation failures by warehouse; page when non-zero for 5 minutes or when the rate exceeds 1% of operations.
3. Read latency p50/p95/p99, with a p95 target below 250 ms for cache hits and below 1 s for source fallback.
4. Command backlog by warehouse and age bucket; warn above 100 pending commands or oldest age above 5 minutes, page above 1,000 or 15 minutes.
5. Kafka consumer backlog and `lag_seconds` by topic/group/partition; warn above 100 messages or 30 seconds, page above 1,000 messages or 5 minutes.
6. Stale serve rate by endpoint; warn above 1% and page above 5%, because stale reads are availability behavior and not mutation confirmation.

All metrics must exclude passwords, tokens, barcode payloads, and authorization data. Correlation IDs remain in request logs and response metadata for trace lookup.

## Verification

- Exact versioned keys, warehouse/operator isolation, stale fallback, and invalidation failure counters covered by `internal/platform/cache/cache_test.go`.
- Redis adapter integration tests cover actual key deletion and rate limiter behavior when Redis is available.
- Focused gateway and application tests pass.
- Full repository verification remains subject to the pre-existing architecture scanner finding that scans the supplied authoritative PDA Android/JVM requirement documents.
