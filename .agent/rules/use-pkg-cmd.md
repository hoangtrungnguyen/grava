# Always Prefer Using Commands via `pkg/cmd`

When interacting with the Grava system (e.g., creating issues, listing tasks, initializing environments), ALWAYS prefer using the Go source code directly via `go run cmd/grava/main.go` instead of the pre-compiled `grava` binary.

This ensures that:
1.  All latest changes in the Go code are reflected immediately.
2.  Environment-specific behaviors are consistent across developer and agent sessions.
3.  We are testing the core implementation directly.

**Example Usage:**
Check issue list:
```bash
go run cmd/grava/main.go list
```

Create a new issue:
```bash
go run cmd/grava/main.go create -t "Issue Title" -d "Detailed description" --type bug
```
