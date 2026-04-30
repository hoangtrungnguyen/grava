# cmd/

Entry points for grava executables. Each subdirectory is a `main` package built into a CLI binary (e.g. `cmd/grava` → `grava` binary). Keep wiring thin — business logic belongs in `pkg/`.
