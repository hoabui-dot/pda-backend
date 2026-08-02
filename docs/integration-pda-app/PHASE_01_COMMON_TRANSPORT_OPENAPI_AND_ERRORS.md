# Phase 01 Prompt — Common Transport, OpenAPI, Headers, Pagination Metadata, and Errors

## Objective

Normalize the cross-cutting HTTP contract before domain DTO changes.

## API Scope

Applies to API-001 through API-028.

## Tasks

1. Implement one success envelope and one error envelope.
2. Route gateway timeout, auth, authorization, validation, rate-limit, conflict, dependency, and internal failures through it.
3. Add reusable OpenAPI schemas for:
   - `Envelope`;
   - `Meta`;
   - `APIError`;
   - cursor metadata;
   - freshness metadata;
   - conflict details;
   - command status.
4. Document and enforce the common headers from the PDA specification.
5. Freeze `X-Operator-Id` as non-authoritative verification context or remove it from the public requirement with rationale.
6. Freeze how `commandId` relates to `Idempotency-Key`.
7. Freeze how `baseVersion` relates to `If-Match`.
8. Implement `Accept-Language` policy for `vi-VN` and `en-US`, or explicitly return stable codes for client localization.
9. Normalize HTTP status mapping: 400, 401, 403, 404, 409, 422, 429, 500, 503.
10. Ensure `Retry-After` is documented for 429 where used.
11. Add `nextCursor`, `hasMore`, `asOf`, and `stale` schemas.
12. Validate route/OpenAPI parity.

## Required Error Codes

At minimum cover the complete PDA catalog, including:

```text
COUNT_VARIANCE_REQUIRES_REVIEW
ITEM_NOT_IN_DOCUMENT
QUANTITY_EXCEEDS_ALLOWED
TASK_ALREADY_COMPLETED
LOCATION_INVALID
UPSTREAM_WMS_UNAVAILABLE
```

## Tests

- timeout envelope;
- malformed request;
- 401 refresh-compatible response;
- 409 conflict details;
- 422 business validation;
- 429 with retry behavior;
- 503 dependency failure;
- language behavior;
- route/OpenAPI parity;
- no raw dependency leakage.

## Exit Criteria

- All public APIs use one envelope.
- Common headers and metadata are frozen.
- OpenAPI contains reusable schemas and stable errors.
- Phase report states `APPROVED`.
