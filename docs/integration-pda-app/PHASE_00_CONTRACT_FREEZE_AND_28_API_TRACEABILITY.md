# Phase 00 Prompt — Freeze the Canonical Contract and Map All 28 PDA APIs

## Objective

Use the actual PDA App API specification to replace the previous incomplete audit baseline and produce a field-level backend comparison for API-001 through API-028.

## Tasks

1. Read the v2 rule file.
2. Confirm the input is `PDA App — Complete Backend API Specification`, generated 2026-08-02, not a generation prompt.
3. Inventory every current public and internal backend route.
4. Map all 28 PDA API cards to current backend behavior.
5. For every API compare:
   - path and method;
   - headers;
   - query parameters;
   - request body;
   - response fields;
   - enums;
   - HTTP/error codes;
   - idempotency/versioning;
   - offline policy;
   - scanner behavior;
   - persistence/outbox/cache effects.
6. Freeze a canonical route decision using the classifications in the rule file.
7. Create a request-field mismatch matrix and response-field mismatch matrix.
8. Identify backend-only APIs and do not delete them without ownership analysis.
9. Create a decision register for every unconfirmed business rule.
10. Do not modify production code in this phase.

## Required Outputs

```text
PDA_BACKEND_CANONICAL_API_DECISION.md
PDA_BACKEND_28_API_TRACEABILITY_MATRIX.md
PDA_BACKEND_BUSINESS_RULE_DECISION_REGISTER.md
requirement-report/YYYY-MM-DD-pda-backend-reconciliation-phase-00.md
```

## Mandatory Traceability Table

| API ID | PDA operation | Proposed PDA path | Current backend path | Decision | Request status | Response status | Implementation gap |
|---|---|---|---|---|---|---|---|

## Exit Criteria

- API-001 through API-028 are all mapped.
- No exact DTO or enum comparison remains `NOT_VERIFIED` merely because the earlier audit used the wrong input file.
- Canonical endpoint decisions are documented.
- The business-rule register exists.
- No production code was modified.
- Phase report states `APPROVED`.
