// Package migrate runs embedded SQL schema migrations against the Grava Dolt
// database.
//
// Migrations live as numbered .sql files under pkg/migrate/migrations and are
// embedded into the binary via go:embed. The package exposes a single Run
// entry point that delegates to pressly/goose with the MySQL dialect (Dolt is
// MySQL wire-compatible) and applies every pending migration in order.
//
// Grava commands such as `grava init` invoke Run after opening a connection
// to the local Dolt server. The expected schema version is tracked in
// pkg/utils.SchemaVersion and persisted to .grava/schema_version on success;
// keep that constant in lock-step with the number of migration files in this
// package.
package migrate
