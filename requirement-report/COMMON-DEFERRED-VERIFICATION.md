# Common Deferred Verification Register

This file records verification cases that may be intentionally deferred when an integration dependency is unavailable or when local infrastructure cannot reproduce production controls. Deferred items must be revisited before the affected phase is approved.

| Case | Current state | Recheck trigger |
|---|---|---|
| Kafka ACL/TLS | Shared MES broker is verified with explicit PLAINTEXT only; SASL/TLS fails closed | Supply credentials/certificates and an ACL-enabled broker |
| Kafka outage recovery | Producer fail-closed test passes; durable retry/DLQ recovery is not yet exercised | Stop/restart broker while outbox worker is active |
| Kafka ordering | Aggregate-ID keys are set and single-broker delivery is verified; ordering under retries/rebalances is not verified | Run multi-message partition/rebalance test |
| Kafka lag/backlog | Adapter counters/gauges are populated; no production dashboard/export exists | Connect metrics sink and run load test |
| External WMS/auth integrations | Mock adapters remain explicit and production rejects mock modes | Provide real OIDC/WMS endpoints and credentials |

Rule: a deferred case is not a successful verification. It is a tracked prerequisite and must be checked again before phase approval.
