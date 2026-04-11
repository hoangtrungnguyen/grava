# Deferred Work

This file tracks technical debt, refactoring tasks, and pre-existing issues identified during code reviews or development sessions that were deferred to maintain focus on the current epic.

## Deferred from: code review of 4-1-establish-and-remove-dependency-relationships.md (2026-04-10)

- **Normalize JSON success/error structure**: The CLI success output for `--json` is a flat object, while error outputs use a nested structure. This should be standardized across the `grava` command suite to improve DX for JSON consumers. (Ref: `pkg/cmd/graph/graph.go`)
