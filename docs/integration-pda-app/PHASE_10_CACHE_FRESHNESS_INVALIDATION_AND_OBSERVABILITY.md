# Phase 10 Prompt — Cache, Freshness, Invalidation, and PDA-Facing Observability

## Objective

Verify exact Redis keys, read freshness, mutation invalidation, stale behavior, and operational metrics for every PDA screen.

## Tasks

1. Inventory cache keys and TTLs for dashboard, tasks, workflow lists/details, inventory, and shipment readiness.
2. Map every mutation to all affected reads and keys.
3. Add `asOf` and `stale` where cached data may be served.
4. Ensure all list reads provide server time and required freshness metadata.
5. Verify post-commit invalidation for:
   - receiving;
   - putaway;
   - picking;
   - replenishment;
   - transfer;
   - count;
   - package verification;
   - shipment confirmation.
6. Ensure Redis outage cannot corrupt or roll back a committed mutation.
7. Define server versus PDA ownership of `pendingSyncCount`.
8. Add metrics:
   - hit/miss/error/latency;
   - stale serve;
   - invalidation success/failure;
   - command backlog;
   - Kafka consumer lag/event lag.
9. Create a real dashboard specification for lag/backlog.

## Tests

- exact key generation;
- cross-warehouse isolation;
- stale response metadata;
- invalidation by mutation;
- Redis outage before/after commit;
- no cached authorization;
- dashboard count refresh;
- inventory balance refresh;
- shipment readiness refresh.

## Exit Criteria

- Every cached PDA read has a verified invalidation path.
- Freshness metadata matches the PDA contract.
- Required metrics and dashboard definitions exist.
- Phase report states `APPROVED`.
