#!/usr/bin/env bash
# scripts/update-docs.sh
# Regenerates docs/detail-impl/ documentation from source code.
# Called automatically by .git/hooks/post-commit on every commit.
# Can also be run manually: bash scripts/update-docs.sh

set -euo pipefail

PROJECT_ROOT="$(git rev-parse --show-toplevel)"
DOCS_DIR="$PROJECT_ROOT/docs/detail-impl"
DATE="$(date +"%Y-%m-%d")"
COMMIT="$(git rev-parse --short HEAD)"

mkdir -p "$DOCS_DIR"

echo "📚 Updating docs/detail-impl/ at commit $COMMIT ..."

# Helper: write the header for each module doc
write_header() {
  local module="$1"
  local role="$2"
  echo "# Module: \`$module\`"
  echo ""
  echo "**Package role:** $role"
  echo ""
  echo "> _Auto-generated on $DATE (commit \`$COMMIT\`). Edit \`scripts/update-docs.sh\` to change the template._"
  echo ""
  echo "---"
  echo ""
}

# Helper: list files in a package with their first package-doc comment
list_files() {
  local dir="$1"
  echo "## Files"
  echo ""
  echo "| File | Lines | Exported Symbols |"
  echo "|:---|:---|:---|"
  for f in "$dir"/*.go; do
    [ -f "$f" ] || continue
    local fname
    fname=$(basename "$f")
    local lines
    lines=$(wc -l < "$f" | tr -d ' ')
    local symbols
    symbols=$(grep -E '^func [A-Z]|^type [A-Z]|^var [A-Z]' "$f" 2>/dev/null | \
      sed 's/^func //' | sed 's/^type //' | sed 's/^var //' | \
      awk -F'[( ]' '{print $1}' | head -5 | paste -sd', ' - || echo "—")
    echo "| \`$fname\` | $lines | $symbols |"
  done
  echo ""
}

# Helper: public API summary (go doc style)
public_api() {
  local pkg_path="$1"
  echo "## Public API"
  echo ""
  echo "\`\`\`"
  go doc -short "$pkg_path" 2>/dev/null | head -40 || echo "(no exported symbols)"
  echo "\`\`\`"
  echo ""
}

# ─────────────────────────────────────────────
# pkg/cmd
# ─────────────────────────────────────────────
{
  write_header "pkg/cmd" "CLI command registration layer. Wires all Cobra commands, manages lifecycle (PersistentPreRunE / PersistentPostRunE), and builds the shared \`cmddeps.Deps\` container."
  echo "## Sub-commands (pkg/cmd/issues/)"
  echo ""
  echo "| File | Command |"
  echo "|:---|:---|"
  for f in "$PROJECT_ROOT/pkg/cmd/issues/"*.go; do
    [[ "$f" == *_test* ]] && continue
    fname=$(basename "$f")
    cmd=$(grep -m1 'Use:' "$f" 2>/dev/null | sed 's/.*Use: *"\([^"]*\)".*/\1/' || echo "—")
    echo "| \`$fname\` | \`grava $cmd\` |"
  done
  echo ""
  list_files "$PROJECT_ROOT/pkg/cmd"
} > "$DOCS_DIR/pkg-cmd.md"

# ─────────────────────────────────────────────
# pkg/cmddeps
# ─────────────────────────────────────────────
{
  write_header "pkg/cmddeps" "Shared dependency injection container and centralized JSON error emitter. Exists to prevent circular imports between pkg/cmd and pkg/cmd/issues/."
  list_files "$PROJECT_ROOT/pkg/cmddeps"
  public_api "github.com/hoangtrungnguyen/grava/pkg/cmddeps"
} > "$DOCS_DIR/pkg-cmddeps.md"

# ─────────────────────────────────────────────
# pkg/dolt
# ─────────────────────────────────────────────
{
  write_header "pkg/dolt" "Primary persistence layer. Wraps Dolt's MySQL-protocol SQL interface with typed query methods, \`WithAuditedTx\` for atomic state + audit-log writes, and retry logic."
  list_files "$PROJECT_ROOT/pkg/dolt"
  public_api "github.com/hoangtrungnguyen/grava/pkg/dolt"
} > "$DOCS_DIR/pkg-dolt.md"

# ─────────────────────────────────────────────
# pkg/graph
# ─────────────────────────────────────────────
{
  write_header "pkg/graph" "DAG engine for context-aware work dispatching. Implements topological sort, cycle detection, priority inheritance, gate evaluation, and ready-task discovery."
  list_files "$PROJECT_ROOT/pkg/graph"
  public_api "github.com/hoangtrungnguyen/grava/pkg/graph"
} > "$DOCS_DIR/pkg-graph.md"

# ─────────────────────────────────────────────
# pkg/grava
# ─────────────────────────────────────────────
{
  write_header "pkg/grava" "Core domain bootstrap. Implements \`ResolveGravaDir()\` (ADR-004 priority chain) for locating the .grava/ directory."
  list_files "$PROJECT_ROOT/pkg/grava"
  public_api "github.com/hoangtrungnguyen/grava/pkg/grava"
} > "$DOCS_DIR/pkg-grava.md"

# ─────────────────────────────────────────────
# pkg/errors
# ─────────────────────────────────────────────
{
  write_header "pkg/errors" "Structured \`GravaError\` type with machine-readable error codes (SCREAMING_SNAKE_CASE). Supports \`errors.Is\` / \`errors.As\` traversal via code-based matching."
  list_files "$PROJECT_ROOT/pkg/errors"
  public_api "github.com/hoangtrungnguyen/grava/pkg/errors"
} > "$DOCS_DIR/pkg-errors.md"

# ─────────────────────────────────────────────
# pkg/idgen
# ─────────────────────────────────────────────
{
  write_header "pkg/idgen" "Hierarchical issue ID generation. Base IDs use SHA-256 of timestamp+random. Child IDs use the DB-backed child_counters table for atomicity."
  list_files "$PROJECT_ROOT/pkg/idgen"
  public_api "github.com/hoangtrungnguyen/grava/pkg/idgen"
} > "$DOCS_DIR/pkg-idgen.md"

# ─────────────────────────────────────────────
# pkg/migrate
# ─────────────────────────────────────────────
{
  write_header "pkg/migrate" "Goose-based database schema migration runner. All SQL files are embedded in the binary via go:embed."
  list_files "$PROJECT_ROOT/pkg/migrate"
  echo "## Migrations"
  echo ""
  echo "| File | Description |"
  echo "|:---|:---|"
  for f in "$PROJECT_ROOT/pkg/migrate/migrations/"*.sql; do
    fname=$(basename "$f")
    desc=$(head -5 "$f" | grep -v '^--$' | grep '^--' | sed 's/^-- *//' | head -1 || echo "—")
    echo "| \`$fname\` | $desc |"
  done
  echo ""
} > "$DOCS_DIR/pkg-migrate.md"

# ─────────────────────────────────────────────
# pkg/notify
# ─────────────────────────────────────────────
{
  write_header "pkg/notify" "Notification abstraction. ConsoleNotifier writes [GRAVA ALERT] to stderr. Non-fatal contract: Send errors never block the primary workflow."
  list_files "$PROJECT_ROOT/pkg/notify"
  public_api "github.com/hoangtrungnguyen/grava/pkg/notify"
} > "$DOCS_DIR/pkg-notify.md"

# ─────────────────────────────────────────────
# pkg/coordinator
# ─────────────────────────────────────────────
{
  write_header "pkg/coordinator" "Background goroutine lifecycle manager. Returns a buffered error channel. Future home for gate polling and Wisp expiry cleanup."
  list_files "$PROJECT_ROOT/pkg/coordinator"
  public_api "github.com/hoangtrungnguyen/grava/pkg/coordinator"
} > "$DOCS_DIR/pkg-coordinator.md"

# ─────────────────────────────────────────────
# pkg/validation
# ─────────────────────────────────────────────
{
  write_header "pkg/validation" "Input validators for issue fields. Case-insensitive. Validates type, status, priority, and date ranges before any DB write."
  list_files "$PROJECT_ROOT/pkg/validation"
  public_api "github.com/hoangtrungnguyen/grava/pkg/validation"
} > "$DOCS_DIR/pkg-validation.md"

# ─────────────────────────────────────────────
# pkg/utils
# ─────────────────────────────────────────────
{
  write_header "pkg/utils" "Miscellaneous utilities: Dolt binary resolution (local vs PATH), git exclude management, and network helpers."
  list_files "$PROJECT_ROOT/pkg/utils"
  public_api "github.com/hoangtrungnguyen/grava/pkg/utils"
} > "$DOCS_DIR/pkg-utils.md"

# ─────────────────────────────────────────────
# pkg/log + pkg/devlog
# ─────────────────────────────────────────────
{
  write_header "pkg/log + pkg/devlog" "Structured logging via zerolog. pkg/devlog is a deprecated no-op stub — all new code should use pkg/log."
  echo "## pkg/log"
  echo ""
  list_files "$PROJECT_ROOT/pkg/log"
  public_api "github.com/hoangtrungnguyen/grava/pkg/log"
  echo "## pkg/devlog (DEPRECATED)"
  echo ""
  echo "> All functions are guaranteed no-ops. Do not add new call sites."
  echo ""
} > "$DOCS_DIR/pkg-log.md"

# ─────────────────────────────────────────────
# pkg/doltinstall
# ─────────────────────────────────────────────
{
  write_header "pkg/doltinstall" "Downloads and installs the latest Dolt binary to .grava/bin/ without root/sudo. Called by grava init."
  list_files "$PROJECT_ROOT/pkg/doltinstall"
  public_api "github.com/hoangtrungnguyen/grava/pkg/doltinstall"
} > "$DOCS_DIR/pkg-doltinstall.md"

# ─────────────────────────────────────────────
# Regenerate the index file
# ─────────────────────────────────────────────
cat > "$DOCS_DIR/index.md" <<INDEXEOF
# \`docs/detail-impl/\` — Module Implementation Reference

This folder contains **detailed implementation documentation** for every Go package in the Grava project.

> _Auto-generated on $DATE at commit \`$COMMIT\`. Updated on every \`git commit\` via \`.git/hooks/post-commit\`._

---

## Module Index

| Module | File | Purpose |
|:---|:---|:---|
| \`pkg/cmd\` | [pkg-cmd.md](./pkg-cmd.md) | CLI command registration, lifecycle, and all sub-commands |
| \`pkg/cmddeps\` | [pkg-cmddeps.md](./pkg-cmddeps.md) | Shared dependency injection, JSON error emitter |
| \`pkg/dolt\` | [pkg-dolt.md](./pkg-dolt.md) | Database persistence layer, \`WithAuditedTx\`, retry logic |
| \`pkg/graph\` | [pkg-graph.md](./pkg-graph.md) | DAG engine, traversal, priority inheritance, gate evaluation |
| \`pkg/grava\` | [pkg-grava.md](./pkg-grava.md) | Domain bootstrap, \`.grava/\` directory resolution |
| \`pkg/errors\` | [pkg-errors.md](./pkg-errors.md) | Structured \`GravaError\` type and error code catalogue |
| \`pkg/idgen\` | [pkg-idgen.md](./pkg-idgen.md) | Hierarchical ID generation (\`grava-xxxx\` and \`grava-xxxx.1\`) |
| \`pkg/migrate\` | [pkg-migrate.md](./pkg-migrate.md) | Goose-based schema migrations (embedded SQL) |
| \`pkg/notify\` | [pkg-notify.md](./pkg-notify.md) | Notification abstraction (\`ConsoleNotifier\`, future integrations) |
| \`pkg/coordinator\` | [pkg-coordinator.md](./pkg-coordinator.md) | Background goroutine lifecycle and error propagation |
| \`pkg/validation\` | [pkg-validation.md](./pkg-validation.md) | Input validators (type, status, priority, date range) |
| \`pkg/utils\` | [pkg-utils.md](./pkg-utils.md) | Dolt binary resolution, git exclude, network helpers |
| \`pkg/log\` + \`pkg/devlog\` | [pkg-log.md](./pkg-log.md) | Zerolog global logger; devlog is deprecated no-op stub |
| \`pkg/doltinstall\` | [pkg-doltinstall.md](./pkg-doltinstall.md) | Automated Dolt binary download + install |

---

## How to Update

Regenerated automatically on each commit. To run manually:
\`\`\`bash
bash scripts/update-docs.sh
\`\`\`
INDEXEOF

echo "✅ docs/detail-impl/ regenerated (commit $COMMIT)"
