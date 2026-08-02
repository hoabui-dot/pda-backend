# Phase 11 Prompt — External OIDC, WMS, Secure Kafka, and Zebra E2E Verification

## Objective

Close all environment-dependent gaps after API contracts are reconciled.

## Required Environments

- production-like OIDC;
- approved real WMS contract and staging endpoint/events;
- Kafka with TLS and least-privilege ACLs;
- staging PostgreSQL/Redis;
- PDA client implementation using the frozen OpenAPI;
- physical Zebra device where available.

## Tasks

### OIDC

- verify issuer, audience, expiry, refresh, revocation, roles, permissions, warehouse and device scopes;
- verify production rejects mock auth.

### WMS

- verify anti-corruption mappings, status/error handling, checkpoint/replay, reconciliation, timeout, retry, breaker, and no direct database access.

### Kafka

- verify TLS;
- verify producer, consumer-group, topic, and DLQ ACLs;
- test authorization denial;
- test broker restart and rebalance;
- test aggregate-key ordering;
- test durable outbox recovery and DLQ;
- verify real backlog/lag dashboards;
- verify no silent fallback to mock.

### Full PDA E2E

Run API-001 through API-028 as applicable, including:

- leading-zero and symbology scans;
- scanner context errors;
- duplicate commands;
- timeout then command status;
- stale version;
- Redis/Kafka/WMS outage;
- token expiry/refresh;
- process/service restart;
- all screen refresh/invalidation behavior.

### Physical Zebra

Verify DataWedge profile, hardware trigger, source/symbology normalization, duplicate scan guard, scanner suspension during submission, reconnect, and no duplicate writes.

## Exit Criteria

- No unresolved P0/P1 integration gap.
- OIDC, WMS, Kafka ACL/TLS, durable outage/rebalance, lag dashboards, and full E2E are externally verified.
- Production profiles reject all mock modes.
- Phase report states `APPROVED`.
