# PDA Task Delivery Runtime Fix

## Result

`PDA_TASK_DELIVERY = VERIFIED`

## Findings

- The PDA gateway circuit breaker treated every HTTP 5xx as a dependency
  failure. Optional, unmapped WMS routes could therefore open the circuit and
  return `GATEWAY_CIRCUIT_OPEN` for login, task reads, and SSE.
- The assigned receipt was in WMS warehouse `WH-KZ3-RM`, while the PDA session
  used the configured business alias `MAIN`. The adapter rejected the receipt
  before returning it to the device.

## Changes

- Circuit state now changes only for explicitly classified dependency errors.
- WMS adapter supports deployment-configured warehouse aliases through
  `PDA_UPSTREAM_WMS_WAREHOUSE_ALIASES`; current demo mapping is
  `MAIN=WH-KZ3-RM`.
- Warehouse comparison accepts the resolved WMS UUID or warehouse code at the
  adapter boundary without moving authorization out of PDA identity context.
- SSE task digests now use the authoritative Receipt `updated_at` instead of a
  local read timestamp, preventing the same task from being emitted every poll.

## Runtime Evidence

| Check | Result |
| --- | --- |
| PDA `/healthz` | HTTP 200 |
| `pda.operator.01` login with registered TC26 | HTTP 200 |
| PDA `/api/pda/v1/tasks` through tunnel | HTTP 200, 1 task |
| Task | `f4c1cd18-6409-4678-bb6f-f44b7c29d47c` |
| Task type/status | `RECEIVING` / `IN_PROGRESS` |
| Assigned operator | `4c3f3b9f-6e0a-4e1c-87db-1d9ed9ad1001` |
| Receiving detail | HTTP 200, 2 receipt lines |
| PDA `/api/pda/v1/events` SSE | HTTP 200; stream remains open |
| SSE task event for current receipt | 1 `TASK_UPDATED` event in 5 seconds |
| Unsupported optional route followed by task read | task read remains HTTP 200 |

## Verification

- `GOWORK=off go test ./...` passed before the final runtime rebuild.
- Focused adapter and gateway tests passed after the warehouse alias change.
- `git diff --check` passed.

## Required Runtime Configuration

For this disposable demo dataset, set:

```text
PDA_UPSTREAM_WMS_WAREHOUSE_ALIASES=MAIN=WH-KZ3-RM
```

This is configuration, not a database mutation. A different WMS warehouse
must use a different explicit alias; the adapter must not infer a warehouse
when multiple WMS warehouses exist.
