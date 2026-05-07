# Package: idgen

Path: `github.com/hoangtrungnguyen/grava/pkg/idgen`

## Purpose

Generate grava issue IDs: top-level hash IDs like `grava-a1b2` and
hierarchical child IDs like `grava-a1b2.1`.

## Key Types & Functions

- `IDGenerator` — interface with `GenerateBaseID()` and
  `GenerateChildID(parentID) (string, error)`.
- `StandardGenerator{Prefix, Store}` — default implementation; default
  prefix `grava`.
- `NewStandardGenerator(store dolt.Store) *StandardGenerator`.
- `(*StandardGenerator).GenerateBaseID()` — SHA-256 over
  `nanoseconds-randomInt`, returns `<prefix>-<first 4 hex chars>` (~65k
  combinations).
- `(*StandardGenerator).GenerateChildID(parentID)` — delegates to
  `Store.GetNextChildSequence(parentID)`; returns `<parentID>.<seq>`.

## Dependencies

- `github.com/hoangtrungnguyen/grava/pkg/dolt` for the child-sequence
  counter.
- Standard library `crypto/rand`, `crypto/sha256`, `math/big`, `time`,
  `fmt`.

## How It Fits

Called from issue-creation paths (CLI commands, importers) before nodes are
inserted into the `issues` table and added to the in-memory DAG in
`pkg/graph`. Child-ID monotonicity is enforced by the database, not the
generator.

## Usage

```go
gen := idgen.NewStandardGenerator(store)
parent := gen.GenerateBaseID()           // "grava-a1b2"
child, err := gen.GenerateChildID(parent) // "grava-a1b2.1"
```
