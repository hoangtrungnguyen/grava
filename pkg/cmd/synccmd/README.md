# Package: synccmd (placeholder)

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd/synccmd`

## Purpose

This directory is currently empty. The actual `synccmd` Go package —
which implements `grava commit`, `grava export`, and `grava import` and
defines the canonical `issues.jsonl` schema — lives in
[`pkg/cmd/sync/`](../sync/), which declares `package synccmd` (renamed to
avoid collision with the stdlib `sync` package).

This `synccmd/` directory appears to be a reserved or leftover path. No
Go files are present, so it is not currently a Go package.

## Key Types & Functions

None — directory is empty. See [`pkg/cmd/sync/README.md`](../sync/README.md)
for the actual command surface and `IssueJSONLRecord` schema.

## Dependencies

None.

## How It Fits

Reserved/empty. Nothing imports this path. If a future refactor renames
`pkg/cmd/sync` to `pkg/cmd/synccmd` to align directory and package names,
this is the destination.

## Usage

Not applicable.
