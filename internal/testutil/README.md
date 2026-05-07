# Package: testutil

Path: `github.com/hoangtrungnguyen/grava/internal/testutil`

## Purpose

Shared test helpers for Grava unit tests. Supplies a configurable mock
implementation of `dolt.Store`, a sqlmock-backed `*sql.DB` factory, and a
typed-error assertion helper.

## Key Types & Functions

- `MockStore` — implements `dolt.Store` with per-method `Fn` fields
  (`ExecContextFn`, `QueryRowContextFn`, `QueryContextFn`, `LogEventTxFn`,
  `BeginTxFn`, `ExecFn`, `QueryRowFn`, `QueryFn`,
  `GetNextChildSequenceFn`). When `Fn` is nil each method returns a safe
  zero value.
- `MockStore.ExecContextCalls` — append-only log of every `ExecContext`
  invocation as `ExecContextCall{Query, Args}` for assertion.
- `ExecContextCall` — recorded query/args pair.
- `NewMockStore() *MockStore` — constructor with all defaults.
- `NewTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock)` — sqlmock-backed
  `*sql.DB` with cleanup wired into `t.Cleanup`.
- `AssertGravaError(t, err, code)` — asserts `err` unwraps to
  `*gravaerrors.GravaError` with the given `Code`.
- Compile-time check `_ dolt.Store = (*MockStore)(nil)` ensures the mock
  stays in sync with the production interface.

## Dependencies

- `github.com/DATA-DOG/go-sqlmock`
- `github.com/stretchr/testify/{assert,require}`
- `github.com/hoangtrungnguyen/grava/pkg/dolt` — interface under test.
- `github.com/hoangtrungnguyen/grava/pkg/errors` — typed Grava error.

## How It Fits

Used by unit tests across the repository (`pkg/issues`, `pkg/orchestrator`,
command tests) to avoid spinning up a real Dolt server. Tests that only
need to assert which SQL was issued use `MockStore`; tests that need to
verify scan logic or transactional flows use `NewTestDB` + sqlmock. Living
under `internal/` keeps the helpers private to the grava module.

## Usage

```go
store := testutil.NewMockStore()
store.ExecContextFn = func(ctx context.Context, q string, args ...any) (sql.Result, error) {
    return driver.ResultNoRows, nil
}
err := myCommand(store)
require.NoError(t, err)
require.Len(t, store.ExecContextCalls, 1)
testutil.AssertGravaError(t, otherErr, "NOT_INITIALIZED")
```
