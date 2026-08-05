# PDA Backend Canonical API Decision Register

- Status: **Phase 00 decision freeze complete; implementation reconciliation required**
- Date: 2026-08-02
- PDA source: `docs/integration-pda-app/PDA_APP_API_SPECIFICATION.md`
- Contract status: proposed by PDA; backend canonical decisions below freeze the compatible backend route

The PDA specification explicitly marks endpoint names as proposed. Decisions below preserve existing
backend routes where semantics are compatible and use adapters/new routes only where the PDA operation
is absent or the current contract cannot represent required fields.

| API ID | PDA proposed operation | Canonical backend decision | Decision | Main contract gap |
|---|---|---|---|---|
| API-001 | `POST /api/pda/v1/auth/login` | Keep route; extend request/response DTO | ADAPT_BACKEND_CONTRACT | device/model/app/warehouse/locale fields and session fields |
| API-002 | `POST /api/pda/v1/auth/refresh` | Keep route; reconcile body refresh token with current bearer rotation | ADAPT_BACKEND_CONTRACT | request identity and secure refresh-token contract |
| API-003 | `POST /api/pda/v1/auth/logout` | Keep route; preserve 204 only if PDA approves compatibility | ADAPT_BACKEND_CONTRACT | PDA expects session/device revoke semantics |
| API-004 | `GET /api/pda/v1/bootstrap` | Keep route; expand bootstrap response | ADAPT_BACKEND_CONTRACT | scanner policy, locale policy, flags, profile fields |
| API-005 | `GET /api/pda/v1/dashboard` | Keep route; scope from trusted headers, add response fields/meta | ADAPT_BACKEND_CONTRACT | PDA count/progress/freshness fields |
| API-006 | `GET /api/pda/v1/me` | Keep route; expand profile DTO | ADAPT_BACKEND_CONTRACT | employee, shift, active and permission fields |
| API-007 | `GET /tasks`, `GET /tasks/{id}`, claim/release | Keep existing list/commands; add task detail | ADD_NEW_ENDPOINT | task detail and PDA query/filter fields |
| API-008 | `GET /api/pda/v1/receiving`, `/{taskId}` | Add compatibility aliases or freeze existing `/receiving/tasks` | ADAPT_BACKEND_CONTRACT | proposed path and response policy/line fields |
| API-009 | `POST /receiving/{taskId}/resolve-barcode` | Adapt existing barcode resolution route and DTO | ADAPT_BACKEND_CONTRACT | raw/normalized/symbology/context/scan timestamp |
| API-010 | `POST /receiving/{taskId}/confirm` | Adapt existing receipt route and DTO | ADAPT_BACKEND_CONTRACT | condition, scannedAt, command identity/body fields |
| API-011 | `GET /putaway`, `/{taskId}` | Adapt existing `/putaway/tasks` routes or add aliases | ADAPT_BACKEND_CONTRACT | proposed path and balance/assignment fields |
| API-012 | `POST /putaway/{taskId}/validate-source` | Adapt source validation; add item validation only if required | ADAPT_BACKEND_CONTRACT | scanner payload and resolved entity fields |
| API-013 | `POST /putaway/{taskId}/validate-destination` | Adapt existing validation and suggestion routes | ADAPT_BACKEND_CONTRACT | accepted/capacity/next-step response |
| API-014 | `POST /putaway/{taskId}/confirm` | Adapt existing confirmation route and DTO | ADAPT_BACKEND_CONTRACT | source/destination/item/LPN and balance response |
| API-015 | `GET /picking`, `/{orderId}` | Adapt existing `/picking/tasks` routes | ADAPT_BACKEND_CONTRACT | order/customer/current-line/readiness fields |
| API-016 | `POST /picking/{orderId}/validate-location` | Adapt existing location validation route | ADAPT_BACKEND_CONTRACT | order/line/scanner fields and accepted next step |
| API-017 | `POST /picking/{orderId}/resolve-item` | Adapt existing barcode resolution route | ADAPT_BACKEND_CONTRACT | lot/serial and available quantity fields |
| API-018 | pick/short-pick/complete | Keep pick/complete; add explicit short-pick command if approved | ADAPT_BACKEND_CONTRACT | reason code and shipment impact |
| API-019 | `GET /replenishment`, `/{taskId}` | Adapt existing `/replenishment/tasks` routes | ADAPT_BACKEND_CONTRACT | partial policy and proposed path |
| API-020 | `POST /replenishment/{taskId}/validate` | Adapt existing three validation routes | ADAPT_BACKEND_CONTRACT | consolidated scanner/quantity contract |
| API-021 | `POST /replenishment/{taskId}/confirm` | Adapt existing confirmation route | ADAPT_BACKEND_CONTRACT | partial completion and balances |
| API-022 | inventory items/balances/movements | Adapt existing search/balances/movements routes | ADAPT_BACKEND_CONTRACT | lot/serial/LPN filters, pagination, asOf/stale |
| API-023 | `/transfers/validate`, `/transfers/confirm` | Adapt existing validation/`POST /transfers` routes | ADAPT_BACKEND_CONTRACT | atomic before/after balances and request identity |
| API-024 | `GET /api/pda/v1/commands/{commandId}` | Add generic scoped command-status endpoint | ADD_NEW_ENDPOINT | only receiving/shipping scoped status exists |
| API-025 | counts list/detail/validate/submit/recount/complete | Adapt current cycle-count routes; add validation routes | ADAPT_BACKEND_CONTRACT | blind count, variance review, line command shape |
| API-026 | shipments list/detail/readiness | Adapt current shipment/readiness routes | ADAPT_BACKEND_CONTRACT | shipment list/detail and freshness fields |
| API-027 | package verify | Add package verification endpoint and persistence | ADD_NEW_ENDPOINT | no current public route or package verification use case |
| API-028 | `POST /shipments/{id}/confirm` | Adapt current `/confirmation` route | ADAPT_BACKEND_CONTRACT | verified packages and canonical command path |

## Compatibility policy

Existing `/api/pda/v1` routes remain supported while DTO/path adapters are introduced. Proposed PDA
paths must not be added as duplicate behavior until the request/response mapper and OpenAPI contract
are tested. No backend-only operational or deferred generic module route is deleted.

