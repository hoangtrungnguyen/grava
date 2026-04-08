# Source Tree Analysis: Grava

## Folder Structure

```
grava/
├── cmd/
│   └── grava/           # Main entry point (main.go)
├── pkg/
│   ├── cmd/             # Cobra command definitions
│   │   ├── issues/      # Issue management subcommands
│   │   └── sync/        # Git/GitHub sync logic
│   ├── cmddeps/         # Shared CLI dependencies (JSON error handling)
│   ├── dolt/            # Dolt client and transaction wrappers
│   ├── doltinstall/     # Dolt installation utilities
│   ├── graph/           # Dependency graph logic
│   ├── grava/           # Core domain models and resolver
│   ├── idgen/           # ID generation logic (hierarchical)
│   ├── migrate/         # SQL schema migrations (Goose)
│   ├── notify/          # Notification system (GitHub, slack, etc.)
│   └── utils/           # Shared utility functions
├── internal/
│   └── testutil/        # Shared testing helpers
├── sandbox/             # Python integration test suite
│   ├── schemas/         # Expected output schemas
│   └── scripts/         # Test runner scripts
├── scripts/             # Operational scripts (setup, clean, etc.)
├── docs/                # Project documentation
└── _bmad/               # BMAD framework configuration
```

## Critical Files
- **go.mod**: Dependency manifest.
- **cmd/grava/main.go**: Application entry point.
- **pkg/cmd/root.go**: CLI root command definition.
- **pkg/dolt/client.go**: Database client implementation.
- **pkg/migrate/migrations/**: SQL schema history.

## Integration Points
- **Dolt/MySQL**: Persistence layer.
- **GitHub**: (via `pkg/notify` and `pkg/cmd/sync`) External issue tracker integration.
- **BMAD**: (via `_bmad/`) Agentic workflow orchestration.
