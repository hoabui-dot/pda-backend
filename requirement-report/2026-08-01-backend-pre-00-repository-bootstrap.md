Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified.
- PDA backend repository before phase: did not exist.
- PDA backend repository after phase: created as a Go backend.
- Android project creation: not performed.

# PRE-00 Repository Bootstrap Report

Status: **APPROVED**

## Scope

Created the new `pda-backend` Go repository from zero according to the revised v2 strategy. No BE-00 domain contracts, mock adapters, workflow endpoints, Kafka, WMS, or OIDC implementation was added.

## Files and modules

- Root Go module/workspace: `go.mod`, `go.sum`, `go.work`.
- Six deployable commands under `cmd/`: API gateway, identity, execution, inventory, shipping, and integration-event services.
- Shared bootstrap packages: runtime modes and HTTP operational endpoints under `internal/platform`.
- Operational OpenAPI seed, config defaults, deterministic fixture version, CI, Makefile, Dockerfiles, Compose, README, and architecture boundary documentation.

## Endpoints

Every service exposes `GET /healthz`, `GET /livez`, and `GET /readyz`. Feature endpoints under `/api/pda/v1` remain assigned to later phases.

## Runtime modes

- `messaging.mode=mock`
- `upstream-wms.mode=mock`
- `auth.mode=mock`

Kafka is not configured or required.

## Database and cache

No migrations were required in PRE-00. PostgreSQL 17.5 and Redis 8.0.2 are defined in Compose and both reached healthy state. Conflict-safe default host ports are PostgreSQL `15432` and Redis `16379`; container ports remain standard.

## Verification

- `go work sync`: PASS
- `go mod tidy`: PASS
- `make verify`: PASS
- `go vet ./...`: PASS
- unit and operational HTTP tests: PASS
- integration-tag suite: PASS
- OpenAPI contract test: PASS
- repository-boundary architecture test: PASS
- all six service binaries: BUILD PASS
- `docker compose -f docker/compose.yml config --quiet`: PASS
- PostgreSQL/Redis Compose startup and health checks: PASS

Host toolchain: Go 1.25.2 darwin/arm64. Module compatibility is pinned to Go 1.24.0.

## Failures and corrections

1. Formatting gate failed on new files; ran `gofmt` and reran verification.
2. `go vet` rejected an unkeyed `config.Modes` test literal; changed it to keyed fields.
3. The boundary test matched its own forbidden-string fixture; excluded the test file itself while retaining full repository scanning.
4. PostgreSQL host port `5432` was already occupied; made host ports configurable and selected safe defaults, then verified both containers healthy.

## Dependency verification

PostgreSQL and Redis were physically exercised. Kafka, upstream WMS, OIDC, and production security were not exercised and are not claimed.

## Remaining gaps and readiness

Business APIs, persistence migrations, mock publishers/WMS adapters, production-mode guards, and hexagonal bounded-context packages belong to BE-00 and later. PRE-00 exit criteria pass; BE-00 is safe to begin.
