// Package sandbox implements the `grava sandbox run` command, which executes
// integration validation scenarios against a live Dolt-backed grava
// workspace.
//
// A scenario is a self-contained probe (TS-01 through TS-10 plus the
// merge-driver spike) that exercises a specific concurrency or correctness
// guarantee — concurrent claim, ready-queue ordering, dependency-graph
// edits, export/import round-trip, doctor auto-heal, conflict detection,
// file reservations, worktree crash recovery, swarm claims, etc. Each
// scenario sets up its own data, runs its checks, and cleans up after
// itself. The package's registry pattern (Register / All / Find / Run)
// keeps scenarios additive.
//
// The cobra wrapper supports three selection modes: --scenario=<id>,
// --epic=<n> (run all scenarios gated at or below an epic), and --all.
// Results are emitted as a human-readable summary or JSON when --json is
// set. Scenarios reuse exported helpers from sibling cmd packages
// (cmdgraph.ReadyQueue, synccmd.ExportFlatJSONL/ImportFlatJSONL) so they
// validate the exact code paths users hit.
package sandbox
