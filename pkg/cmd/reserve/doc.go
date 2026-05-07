// Package reserve implements the `grava reserve` command tree for
// declaring, listing, and releasing advisory file-path leases
// (FR-ECS-1a) and the pre-commit overlap check that consumes them.
//
// A reservation is a row in the file_reservations table identified by a
// short generated ID ("res-xxxxxx"). Each lease records project_id,
// agent_id, path_pattern (glob), exclusive flag, optional reason, TTL,
// and timestamps. DeclareReservation validates inputs, detects glob
// overlap with active exclusive leases held by other agents, and inserts
// the row inside a transaction to prevent TOCTOU. Conflicting requests
// return FILE_RESERVATION_CONFLICT.
//
// CheckStagedConflicts is invoked by the pre-commit git hook to compare
// a list of staged paths against active exclusive leases owned by other
// agents and emit Conflict records the hook can surface to the user.
//
// AddCommands(root, deps) wires the cobra subcommands; the package also
// exports DeclareReservation, DeclareParams, DeclareResult, Reservation,
// Conflict, and CheckStagedConflicts so other parts of the CLI (hook,
// sandbox scenarios) can reuse the same logic.
package reserve
