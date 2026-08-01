# PDA Backend

Go backend serving the existing Kotlin Android PDA application through versioned HTTPS/JSON APIs. The Android repository is an external client and is not recreated or modified here.

## Toolchain

- Go 1.24+ (the current workspace is verified with Go 1.25.2)
- Docker Compose for local PostgreSQL and Redis
- Kafka is intentionally not required before BE-08

## Verify

```shell
go work sync
go mod tidy
make migrate-up
make verify
docker compose -f docker/compose.yml config
```

Compose maps PostgreSQL to host port `15432` and Redis to `16379` by default. Override them with `PDA_POSTGRES_PORT` and `PDA_REDIS_PORT`.

Run a service with `go run ./cmd/pda-api-gateway`. Each service exposes `/healthz`, `/livez`, and `/readyz`. Runtime defaults are explicitly mock for messaging, upstream WMS, and authentication.

Schema changes use the pinned `migrate/migrate:v4.18.3` image through `make migrate-up` and `make migrate-down`. The gateway uses `PDA_DATABASE_URL`, defaulting to local Compose PostgreSQL, plus `PDA_REDIS_URL` (default `redis://localhost:16379/0`) and `PDA_CACHE_TTL` (default `30s`). Redis failures degrade reads to PostgreSQL and never replace database command/idempotency truth.

See [docs/architecture.md](docs/architecture.md) for the repository boundaries.
Go package and dependency rules are documented in [docs/coding-standards.md](docs/coding-standards.md).
