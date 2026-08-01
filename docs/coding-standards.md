# Go coding and package standards

- Domain packages contain behavior and explicit value types; they do not import HTTP, PostgreSQL, Redis, or Kafka clients.
- Application packages orchestrate use cases and transaction ports.
- Port packages define consumer-owned interfaces. Adapters implement ports for HTTP, PostgreSQL, Redis, mock messaging, and later Kafka/WMS.
- HTTP handlers only decode/validate DTOs, invoke one application use case, and map safe results.
- Mutable domain entities stay in their bounded context. `internal/platform` contains only stable technical contracts and immutable metadata.
- Constructors validate required fields. Public APIs accept `context.Context` first where cancellation is relevant.
- Errors preserve their cause internally; API responses use stable `DomainError` codes and safe messages.
- All timestamps are UTC, all mutations carry command/idempotency metadata, and event envelopes carry complete actor and causation metadata.
- Configuration is explicit and validated at startup. Production never falls back to mock modes.
