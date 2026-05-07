# Development Guide: Grava

## Prerequisites
- **Go**: 1.24.0 or higher (per `go.mod`)
- **Dolt**: 1.32.0 or higher
- **Git**: any modern version (worktree + merge driver support required)
- **Make**: optional, for task orchestration

## Getting Started

1. **Install Dependencies**
   ```bash
   go mod tidy
   ```

2. **Initialize a Project**
   ```bash
   ./grava init
   ```
   `grava init` provisions `.grava/dolt`, runs migrations, registers the merge driver in `.git/config`, writes `.gitattributes`, and installs hook stubs.

3. **Start the Dolt Server (manual mode)**
   ```bash
   dolt --data-dir .grava/dolt sql-server &
   ```
   Or use the bundled lifecycle commands:
   ```bash
   ./grava db-start
   ./grava db-stop
   ```

4. **Build the CLI**
   ```bash
   go build -o grava ./cmd/grava
   ```

## Development Workflow

### Common Commands
- **Build**: `go build ./...`
- **Unit tests**: `go test ./...`
- **Integration tests** (real `git merge` via the bundled binary): `go test -tags integration ./pkg/merge/...`
- **Linter**: `golangci-lint run ./...`
- **Run locally**: `./grava [command]`

### Database Migrations
Migrations live in `pkg/migrate/migrations/` (`001_…` through `011_…`) and are embedded into the binary via `go:embed`. They are applied automatically on startup through `pkg/migrate/migrate.go` (Goose).

### Testing Layers
- **Unit tests** (`*_test.go` colocated with code): pure-Go logic and mocked DB states (sqlmock).
- **Integration tests** under `//go:build integration`: end-to-end flows that compile and exercise the binary (e.g., `pkg/merge/git_driver_integration_test.go` runs real `git merge` operations through the bundled `grava-merge` driver).
- **Embedded sandbox scenarios**: `grava sandbox <ts01..ts10>` runs the in-binary validation suite against a live Dolt instance.
- **External sandbox harness**: extended multi-agent orchestration scenarios live in the separate `gravav6-sandbox` repository (Python + shell harness).

### Useful Development Conventions
- All write paths must go through `pkg/dolt.WithAuditedTx` so the corresponding `events` rows are recorded atomically.
- Use `dolt.Event*` constants from `pkg/dolt/events.go`; never inline event-type string literals.
- Surface user-facing errors via `pkg/errors.GravaError` with a stable error code; the JSON emitter relies on the code field.
- New CLI commands belong in `pkg/cmd/` (or a sub-package such as `pkg/cmd/issues/`) and must be registered with the appropriate parent in `AddCommands`.

## Project Structure (entry points for common changes)
- **New issue command** → `pkg/cmd/issues/`
- **New doctor check** → `pkg/cmd/maintenance/maintenance.go` (extend `newDoctorCmd`)
- **New schema column or table** → add a migration in `pkg/migrate/migrations/` and update `docs/data-models.md`
- **Merge driver behavior** → `pkg/merge/`; add an integration scenario to `pkg/merge/git_driver_integration_test.go`
- **Git wiring** → `pkg/gitconfig`, `pkg/gitattributes`, `pkg/githooks`
