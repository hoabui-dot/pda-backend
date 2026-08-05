# PDA Backend Reconciliation Phase 01

Status: APPROVED FOR COMMON TRANSPORT SCOPE
Date: 2026-08-02

## Repository Boundary Confirmation

- Existing PDA Android repository: external client; not modified.
- PDA backend repository: current working repository.
- Android project creation or modification: not performed.

## Scope

Phase 01 normalizes the common HTTP contract used by API-001 through API-028 before domain-specific reconciliation. It covers the runtime envelope, correlation and context headers, language negotiation, failure status mapping, retry metadata, cursor metadata, freshness metadata, conflict details, command status, and the OpenAPI reusable schemas.

## Decisions

- Every JSON success response uses `data`, `meta`, and `errors`; successful responses return an empty `errors` array.
- Every JSON error response uses the same envelope with `data: null`; dependency and internal errors do not expose upstream payloads.
- `X-Correlation-Id` is accepted when it is a UUID, generated otherwise, returned in the response header, and included in `meta.correlationId`.
- `X-Device-Id` and `X-Warehouse-Id` remain required for protected warehouse workflows and are validated by the existing device/warehouse context middleware.
- `X-Operator-Id` is non-authoritative. When supplied, it must match the authenticated bearer-session operator; the token remains authoritative.
- `Idempotency-Key` is the transport idempotency key. Existing command handlers derive their command identity from this header. PDA body `commandId` compatibility is deferred to domain phases and must reject mismatches rather than silently choose one value.
- `If-Match` is the transport version precondition for versioned mutations. PDA body `baseVersion` compatibility is deferred to domain phases and must reject mismatches rather than silently choose one value.
- `Accept-Language` accepts `en-US` and `vi-VN`, defaults to `en-US`, and returns `Content-Language`. Stable machine-readable error codes remain the interoperable contract; translated messages are deferred until the message catalog is approved.
- `Retry-After` is returned for HTTP 429 responses.
- `meta.nextCursor`, `meta.hasMore`, `meta.asOf`, and `meta.stale` are available on every success envelope. Domain handlers populate them when authoritative pagination or freshness data exists.
- The OpenAPI document now defines reusable `Envelope`, `Meta`, `APIError`, `ConflictDetails`, `CommandStatus`, `SuccessEnvelope`, `ErrorEnvelope`, and `RateLimited` components.

## HTTP Status Mapping

The gateway maps common failures to 400, 401, 403, 404, 409, 422, 429, 500, or 503. Rate limiting emits `Retry-After`; timeout, circuit-open, WMS dependency, and pending messaging failures use retryable error metadata and HTTP 503. Unknown errors are reduced to `INTERNAL_ERROR` with no dependency details.

## Verification

Passed:

- `go test ./internal/gateway/adapters/http ./test/contract ./internal/integration/adapters/kafka`
- `make build`
- `make test-kafka`
- `git diff --check`

The focused gateway tests cover correlation propagation, language validation, operator-context validation, timeout envelopes, rate-limit retry metadata, and sensitive-log redaction. Existing route tests cover the success envelope and cursor metadata.

## Deferred Reconciliation

The common transport is frozen for the next phases. Domain phases must reconcile PDA-specific request fields, command IDs, base versions, conflict detail payloads, freshness population, and localized message catalogs. The existing repository architecture guard currently reports Android references inside the supplied integration documents; those documents are authoritative inputs and were not modified or removed.
