# Module: `pkg/dolt`

**Package role:** Primary persistence layer. Wraps Dolt's MySQL-protocol SQL interface with typed query methods, `WithAuditedTx` for atomic state + audit-log writes, and retry logic.

> _Auto-generated on 2026-04-08 (commit `baefe84`). Edit `scripts/update-docs.sh` to change the template._

---

## Files

| File | Lines | Exported Symbols |
|:---|:---|:---|
| `client.go` | 200 | Store,Client NewClient,NewClientFromDB |
| `client_integration_test.go` | 322 | TestClient_GetNextChildSequence_Integration,TestForeignKeyConstraints_Dependencies TestForeignKeyConstraints_Events |
| `events.go` | 24 | — |
| `mock_client.go` | 70 | MockStore,NewMockStore |
| `retry.go` | 34 | WithDeadlockRetry |
| `retry_test.go` | 75 | TestWithDeadlockRetry_SuccessFirstAttempt,TestWithDeadlockRetry_DeadlockThenSuccess TestWithDeadlockRetry_AlwaysDeadlock,TestWithDeadlockRetry_NonDeadlockError_NoRetry TestIsMySQLDeadlock_True |
| `tx.go` | 51 | AuditEvent,WithAuditedTx |
| `tx_test.go` | 182 | TestWithAuditedTx_CommitsOnSuccess,TestWithAuditedTx_RollsBackOnFnError TestWithAuditedTx_RollsBackOnAuditLogError,TestWithAuditedTx_NoEvents_StillCommits TestWithAuditedTx_MultipleEvents_AllLogged |

## Public API

```
const EventCreate = "create" ...
func WithAuditedTx(ctx context.Context, store Store, events []AuditEvent, ...) error
func WithDeadlockRetry(fn func() error) error
type AuditEvent struct{ ... }
type Client struct{ ... }
    func NewClient(dsn string) (*Client, error)
    func NewClientFromDB(db *sql.DB) *Client
type MockStore struct{ ... }
    func NewMockStore() *MockStore
type Store interface{ ... }
```

