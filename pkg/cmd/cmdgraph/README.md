# Package: cmdgraph (placeholder)

Path: `github.com/hoangtrungnguyen/grava/pkg/cmd/cmdgraph`

## Purpose

This directory is currently empty. The actual implementation of the
`cmdgraph` Go package — the cobra commands for `dep`, `graph`, `ready`,
`blocked`, `search`, and `stats` — lives in [`pkg/cmd/graph/`](../graph/),
which declares `package cmdgraph` (the package name is `cmdgraph` to avoid
collision with the underlying `pkg/graph` library).

This `cmdgraph/` directory appears to be a reserved or leftover path. No
Go files are present, so it is not currently a Go package.

## Key Types & Functions

None — directory is empty. See [`pkg/cmd/graph/README.md`](../graph/README.md)
for the actual command surface.

## Dependencies

None.

## How It Fits

Reserved/empty. Nothing imports this path. If a future refactor renames
`pkg/cmd/graph` to `pkg/cmd/cmdgraph` to align directory and package
names, this is the destination.

## Usage

Not applicable.
