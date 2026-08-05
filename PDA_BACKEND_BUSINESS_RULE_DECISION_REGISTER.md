# PDA Backend Business-Rule Decision Register

Status: **OPEN - requires PDA specification and product/WMS decisions**  
Created for reconciliation Phase 00 on 2026-08-02.

| Decision ID | Rule | Current backend behavior | Required decision | Affected APIs | Status |
|---|---|---|---|---|---|
| BR-001 | Login device registration timing | Separate registration endpoint | before login, during login, or separate flow | 001-004 | UNCONFIRMED |
| BR-002 | Login device/app/locale fields | Not accepted by current login DTO | required fields and validation | 001 | UNCONFIRMED |
| BR-003 | Refresh token storage/rotation | Mock token rotation | PDA retry/single-flight behavior | 002 | UNCONFIRMED |
| BR-004 | Logout response | HTTP 204 | envelope or 204 client contract | 003 | UNCONFIRMED |
| BR-005 | Feature flags/scanner policy | Bootstrap returns empty capabilities | authoritative field set and defaults | 004 | UNCONFIRMED |
| BR-006 | Task claim lock duration | Aggregate assignment/version checks | lock timeout and release semantics | 007 | UNCONFIRMED |
| BR-007 | Scanner payload | Backend accepts value/location/barcode variants | raw/normalized/symbology/context fields | 009, 012-020 | UNCONFIRMED |
| BR-008 | Receipt tolerance | Domain policy rejects invalid quantity | over/under tolerance and condition/lot/serial | 010 | UNCONFIRMED |
| BR-009 | Short-pick | Picking quantity validation exists | explicit short-pick reason and completion rule | 018 | UNCONFIRMED |
| BR-010 | Replenishment partial completion | Partial quantities supported | next-step and completion semantics | 021 | UNCONFIRMED |
| BR-011 | Inventory filters | item/location filters supported | lot/serial/LPN dimensions and freshness | 022 | UNCONFIRMED |
| BR-012 | Transfer result | Mutation returns task/result | before/after balances and command status | 023-024 | UNCONFIRMED |
| BR-013 | Blind count | Count domain stores expected quantity | whether expected quantity is hidden | 025 | UNCONFIRMED |
| BR-014 | Count variance | Variance is evidence; no automatic inventory adjustment | review, approval, recount and adjustment ownership | 025 | UNCONFIRMED |
| BR-015 | Package verification | Shipment package state exists; no PDA verification route | barcode/package validation contract | 027-028 | UNCONFIRMED |
| BR-016 | Offline mutation policy | Idempotent/versioned commands; receiving/shipping status routes | per-mutation online/offline/queue policy | 007, 010, 014, 018, 021, 023, 025, 027, 028 | UNCONFIRMED |
| BR-017 | Command retention | Durable scoped command records | retention period and generic lookup scope | 024 | UNCONFIRMED |
| BR-018 | Freshness | server time and Redis TTL exist | `asOf`, stale marker, and list consistency | 005, 008, 011, 015, 019, 022, 026 | UNCONFIRMED |

No rule is silently changed in production code during Phase 00.

