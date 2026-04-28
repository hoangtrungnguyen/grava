# Package: migrate

Path: `github.com/hoangtrungnguyen/grava/pkg/migrate`

## Purpose

Runs the embedded SQL schema migrations against the Grava Dolt database.
Migrations are applied in numeric order using pressly/goose with the MySQL
dialect (Dolt is MySQL wire-compatible).

## Key Types & Functions

- `Run(db *sql.DB) error` — sets the goose dialect to `mysql`, points goose at
  the embedded `migrations/*.sql` filesystem, and runs `goose.Up`. Returns a
  wrapped error if either step fails.
- `embedMigrations` (unexported `embed.FS`) — holds every `.sql` file under
  `migrations/` so the binary is self-contained.

## Dependencies

- `database/sql` — standard library DB handle.
- `embed` — bundles migration files into the binary.
- `github.com/pressly/goose/v3` — migration runner.

## How It Fits

Grava stores all multi-agent state in a local Dolt database at `.grava/dolt/`.
Commands such as `grava init` open a Dolt connection and call `migrate.Run` to
bring the schema to the expected version. The companion package `pkg/utils`
exposes `SchemaVersion` and writes `.grava/schema_version` after a successful
migration; the migrations directory drives that version counter.

## Usage

```go
import (
    "database/sql"

    _ "github.com/go-sql-driver/mysql"
    "github.com/hoangtrungnguyen/grava/pkg/migrate"
)

db, err := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/grava?parseTime=true")
if err != nil {
    return err
}
if err := migrate.Run(db); err != nil {
    return err
}
```
