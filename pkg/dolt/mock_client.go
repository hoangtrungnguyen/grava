package dolt

import (
	"context"
	"database/sql"
)

// MockStore is a mock implementation of Store for testing.
//
// Deprecated: Use internal/testutil.MockStore instead. This mock will be removed
// once all consumers migrate to the testutil version.
type MockStore struct {
	Sequences map[string]int
}

func NewMockStore() *MockStore {
	return &MockStore{
		Sequences: make(map[string]int),
	}
}

func (m *MockStore) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return nil, nil
}

func (m *MockStore) Exec(query string, args ...any) (sql.Result, error) {

	return nil, nil
}

func (m *MockStore) QueryRow(query string, args ...any) *sql.Row {
	return nil // This might panic if used, but for now we just want to satisfy interface
}

func (m *MockStore) Query(query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (m *MockStore) GetNextChildSequence(parentID string) (int, error) {
	m.Sequences[parentID]++
	return m.Sequences[parentID], nil
}

func (m *MockStore) Close() error {
	return nil
}

func (m *MockStore) SetMaxOpenConns(n int) {}
func (m *MockStore) SetMaxIdleConns(n int) {}
func (m *MockStore) DB() *sql.DB           { return nil }

func (m *MockStore) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return nil, nil
}

func (m *MockStore) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return nil
}

func (m *MockStore) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return nil, nil
}

func (m *MockStore) LogEvent(issueID, eventType, actor, agentModel string, oldValue, newValue any) error {
	return nil
}

func (m *MockStore) LogEventTx(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, agentModel string, oldValue, newValue any) error {
	return nil
}
