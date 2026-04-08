# Development Guide: Grava

## Prerequisites
- **Go**: 1.24.0 or higher
- **Dolt**: 1.32.0 or higher
- **Python**: 3.9+ (for sandbox integration tests)
- **Make**: (optional) for task orchestration

## Getting Started

1. **Install Dependencies**:
   ```bash
   go mod tidy
   ```

2. **Initialize Database**:
   ```bash
   mkdir -p .grava/dolt
   dolt init --data-dir .grava/dolt
   ```

3. **Start Dolt Server**:
   ```bash
   dolt --data-dir .grava/dolt sql-server &
   ```

4. **Build CLI**:
   ```bash
   go build -o grava cmd/grava/main.go
   ```

## Development Workflow

### Commands
- **Build**: `go build ./...`
- **Unit Tests**: `go test ./...`
- **Linter**: `golangci-lint run ./...`
- **Run Locally**: `./grava [command] --config .grava.yaml`

### Database Migrations
Goose is used for migrations. Migrations are situated in `pkg/migrate/migrations`.
- **Apply migrations**: The CLI applies migrations automatically on startup via `pkg/migrate/migrate.go`.

### Testing
- **Unit Tests**: Standards Go testing focusing on business logic and mocked DB states.
- **Integration Tests (Sandbox)**:
  - Located in `sandbox/`.
  - Driven by Python scripts that execute the `grava` binary against a fresh Dolt instance.
  - Run with: `python3 sandbox/run_scenarios.py`

## Project Structure
- `pkg/cmd/issues`: Add new CLI commands here.
- `pkg/grava`: Core business logic and types.
- `pkg/dolt`: SQL query logic.
- `pkg/migrate`: Database schema evolution.
