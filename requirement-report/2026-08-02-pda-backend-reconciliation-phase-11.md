# PDA Backend Reconciliation Report — Phase 11

> Authentication implementation has progressed since this report was written. See [Phase 11 Production Readiness](/home/neurosus/recoh-system/pda-backend/requirement-report/2026-08-02-phase-11-production-readiness.md) for the current audit and evidence.

Date: 2026-08-02
Status: BLOCKED

## Scope

Phase 11 covers production-like OIDC, approved WMS, secure Kafka, staging infrastructure, the frozen PDA client, and physical Zebra verification. These are environment-dependent exit criteria and cannot be marked verified from this local repository alone.

## Implemented backend readiness

- Production configuration continues to reject every mock mode.
- Kafka remains fail-closed and never falls back from Kafka to the mock publisher.
- Kafka now supports explicit `TLS` mode with TLS 1.2 minimum, a required CA PEM file, optional client certificate/key pair, and optional server name. `PLAINTEXT` remains available only for local/shared-broker verification.
- Kafka publisher and consumer use the same explicit security configuration. Consumer-group delivery, durable outbox, inbox idempotency, retries, DLQ boundary, aggregate message keys, and lag/backlog counters remain in place.
- WMS HTTP mapping validates the configured endpoint and bearer token and keeps WMS DTOs inside the adapter boundary.

## Required external verification still outstanding

1. OIDC issuer/audience/signature/JWKS, expiry, refresh, revocation, role/permission claims, warehouse scope, and device scope require a staging identity provider. The current gateway composition still intentionally fails when `PDA_AUTH_MODE=oidc` because no approved provider adapter is present.
2. The current executable composition still loads workflow fixtures and rejects non-mock WMS mode. A real WMS endpoint and approved workflow/event contract are required before enabling production WMS composition.
3. Kafka TLS handshake, producer/consumer ACL denial, broker restart/rebalance, topic/DLQ permissions, and production lag export require a secured staging broker and disposable ACL principals.
4. API-001 through API-028 with the actual PDA client, token expiry, restart, Redis/Kafka/WMS failure injection, and refresh/invalidation behavior require the PDA build and staging services.
5. Zebra DataWedge profile, hardware trigger, symbology/leading-zero behavior, duplicate scan guard, scanner suspension, reconnect, and no-duplicate-write behavior require a physical device.

## Configuration

Secure Kafka variables:

- `PDA_KAFKA_SECURITY_PROTOCOL=TLS`
- `PDA_KAFKA_TLS_CA_FILE=/secure/path/ca.pem`
- `PDA_KAFKA_TLS_CERT_FILE=/secure/path/client.crt` and `PDA_KAFKA_TLS_KEY_FILE=/secure/path/client.key` when mTLS is required
- `PDA_KAFKA_TLS_SERVER_NAME=broker.example.internal` when broker certificate name differs from the address

Secrets must be mounted through the deployment secret manager and must not be committed or logged.

## Verification

- Kafka, configuration, WMS adapter, and existing repository tests pass locally.
- `make test-kafka` verifies the shared local PLAINTEXT broker path, not TLS or ACL authorization.
- Phase 11 remains `BLOCKED` until the external environments and PDA/Zebra E2E evidence above are supplied. No P0/P1 production-readiness claim is made.
