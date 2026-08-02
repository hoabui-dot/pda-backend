# Phase 08 Prompt — Shipment Readiness, Package Verification, and Final Confirmation

## API Scope

- API-026 Shipment summary/detail/readiness
- API-027 Package verification
- API-028 Shipment confirmation
- API-024 command-status integration for final confirmation

## Objective

Add any missing package verification and align final shipping confirmation with PDA scanner, readiness, carrier, tracking, package, version, and online-only behavior.

## Confirmed Risk to Review

The previous backend report described shipping summary/readiness/confirmation but did not clearly confirm a package verification route matching API-027. Treat this as a required explicit audit item.

## Package Verification Request

```text
shipmentId, packageId, rawValue, normalizedValue,
symbology, scanContext=SHIPPING_PACKAGE, baseVersion
```

Operator/device/warehouse are authoritative from authenticated context.

## Shipment Response Requirements

```text
shipmentId, salesOrderId, customer, shipTo,
packageCount, verifiedPackageCount,
carrierCode, trackingNumber,
readinessStatus, blockingReasons,
status, version, asOf
```

## Final Confirmation Request

```text
commandId, idempotencyKey, shipmentId,
carrierCode, trackingNumber,
verifiedPackageIds, baseVersion
```

## Tasks

1. Verify or add public package verification.
2. Ensure verification updates package set/readiness/version without falsely confirming shipment.
3. Validate carrier/tracking policy.
4. Require fresh authoritative readiness before confirmation.
5. Keep final shipment confirmation online-only.
6. Return manifest reference only if it exists; do not fabricate printing/label data.
7. Add command-status recovery for timeout/duplicate.
8. Invalidate shipment/task/dashboard projections.

## Tests

- unknown/wrong-context package;
- duplicate package scan;
- incomplete package set;
- stale version;
- invalid carrier/tracking;
- readiness blockers;
- duplicate final confirmation;
- timeout/status lookup;
- no fabricated label/manifest;
- invalidation.

## Exit Criteria

- SCR-16 package scanning and final confirmation are both supported.
- Shipping cannot complete before readiness.
- Label printing remains explicitly out of scope unless approved.
- Phase report states `APPROVED`.
