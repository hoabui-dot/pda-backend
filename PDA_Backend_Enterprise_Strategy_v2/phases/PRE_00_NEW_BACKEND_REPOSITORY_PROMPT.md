# Backend PRE-00 Prompt — Create the New Go PDA Backend Repository

## Non-Negotiable Repository Boundary

This phase creates a completely new Go backend repository. The existing Kotlin Android `PDA_APP` already exists and is an external API client. Do not create, copy, modify, build, or verify Android code or tooling. Before this phase, no backend repository or backend build is expected.

## Objective

Create a new Go multi-service backend repository from zero and produce the first successful backend build and test run.

## Required repository name

Use the user-approved name, or `pda-backend` when none is supplied. Never name it `PDA_APP`.

## Required initial structure

```text
pda-backend/
  go.work
  go.mod
  Makefile
  cmd/
    pda-api-gateway/
    identity-access-service/
    pda-execution-service/
    inventory-service/
    outbound-shipping-service/
    integration-event-service/
  internal/
    platform/
  api/openapi/
  test/fixtures/
  docker/
  docs/
  requirement-report/
  README.md
```

Each deployable service uses `cmd/<service>/main.go`. Shared infrastructure utilities may live under `internal/platform`; mutable domain entities must remain inside their owning bounded context and must not become a shared package.

## Pinned technology baseline

- Go 1.24 or a later explicitly approved stable version, pinned in `go.mod` and CI.
- Standard `net/http` plus `chi` for routing.
- `pgx/v5` for PostgreSQL; versioned SQL migrations with `golang-migrate`.
- `go-redis/v9` for Redis when enabled.
- `slog` for structured logs and OpenTelemetry Go for tracing/metrics.
- Go `testing`, `httptest`, `testify`, and Testcontainers for Go.
- `golangci-lint`, `gofmt`, `go vet`, and `govulncheck` foundations.
- Make targets as the stable developer and CI command surface.

## Tasks

1. Initialize `go.mod` and `go.work` in the new backend repository.
2. Pin Go and dependency versions; commit `go.sum`.
3. Create all required deployable commands and bounded-context directories.
4. Add minimal HTTP services with `/healthz`, `/livez`, and `/readyz`.
5. Add default configuration with `messaging.mode=mock`, `upstream-wms.mode=mock`, and `auth.mode=mock`.
6. Add Make targets: `fmt`, `lint`, `test`, `test-integration`, `test-contract`, `test-architecture`, `build`, and `verify`.
7. Add Dockerfiles for every deployable command.
8. Add Docker Compose for PostgreSQL and Redis. Kafka may appear only behind an optional disabled profile.
9. Add backend CI for formatting, vet/lint, tests, builds, and Compose validation.
10. Add README instructions for native Go and pinned-container execution.
11. Add a repository-boundary test that rejects Android manifests, Android Gradle plugins, Java/Kotlin source trees, and `com.example.pda_app` outside the strategy documents.
12. Run the first backend verification.

## Environment handling

If Go is not installed but Docker is available, use a pinned image:

```shell
docker run --rm -v "$PWD":/workspace -w /workspace golang:1.24-alpine make verify
```

Record the exact image and command. A missing Android toolchain is irrelevant and must not be treated as a failure.

## Verification

- `go work sync` and `go mod tidy` succeed;
- `go test ./...` succeeds;
- `go vet ./...` succeeds;
- `make verify` succeeds;
- every command builds;
- HTTP health/readiness tests pass;
- Docker Compose validates and PostgreSQL/Redis start when Docker is available;
- no Java, Kotlin, Spring, Gradle backend artifact, Android plugin/source/package, or Android manifest exists outside the strategy package.

## Required report

Create `requirement-report/YYYY-MM-DD-backend-pre-00-repository-bootstrap.md`. It must begin with:

```text
Repository boundary confirmed:
- Existing PDA Android repository: external client, not modified.
- PDA backend repository before phase: did not exist.
- PDA backend repository after phase: created as a Go backend.
- Android project creation: not performed.
```

## Exit Criteria

- The new Go backend repository, workspace/modules, services, and first passing build exist.
- PostgreSQL/Redis development infrastructure is defined; Kafka is not required.
- No Java/Kotlin backend or Android project was created or modified.
- The PRE-00 report states `APPROVED`, making BE-00 safe to begin.
