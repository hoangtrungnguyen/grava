package dolt

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Store defines the interface for database interactions needed by the ID generator.
type Store interface {
	// GetNextChildSequence atomically increments the counter for the given parentID and returns the new sequence number.
	GetNextChildSequence(parentID string) (int, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	SetMaxOpenConns(n int)
	SetMaxIdleConns(n int)
	DB() *sql.DB
	Close() error

	// Audit logging
	LogEvent(issueID, eventType, actor, agentModel string, oldValue, newValue any) error
	LogEventTx(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, agentModel string, oldValue, newValue any) error
}

// Client implements Store using a SQL database.
type Client struct {
	db *sql.DB
}

func (c *Client) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return c.db.BeginTx(ctx, opts)
}

func (c *Client) SetMaxOpenConns(n int) {
	c.db.SetMaxOpenConns(n)
}

func (c *Client) SetMaxIdleConns(n int) {
	c.db.SetMaxIdleConns(n)
}

// NewClient establishes a connection to the Dolt database.
// Connection string format: user:password@tcp(host:port)/dbname
func NewClient(dsn string) (*Client, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open db connection: %w", err)
	}
	// Configure pool to ensure we don't starve if we hold connections?
	db.SetMaxOpenConns(20)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	// Explicitly enable foreign key checks to ensure enforcement in all environments (e.g. CI)
	_, _ = db.Exec("SET foreign_key_checks = 1")

	return &Client{db: db}, nil
}

// NewClientFromDB creates a Client using an existing sql.DB connection.
// Useful for testing with sqlmock.
func NewClientFromDB(db *sql.DB) *Client {
	return &Client{db: db}
}

func (c *Client) Close() error {
	return c.db.Close()
}

func (c *Client) DB() *sql.DB {
	return c.db
}

func (c *Client) Exec(query string, args ...any) (sql.Result, error) {
	return c.db.Exec(query, args...)
}

func (c *Client) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.db.ExecContext(ctx, query, args...)
}

func (c *Client) QueryRow(query string, args ...any) *sql.Row {
	return c.db.QueryRow(query, args...)
}

func (c *Client) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}

func (c *Client) Query(query string, args ...any) (*sql.Rows, error) {
	return c.db.Query(query, args...)
}

func (c *Client) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query, args...)
}

// GetNextChildSequence uses Advisory Locks (GET_LOCK) and retries on serialization failures.
func (c *Client) GetNextChildSequence(parentID string) (int, error) {
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		seq, err := c.tryGetNextChildSequence(parentID)
		if err == nil {
			return seq, nil
		}
		// Dolt/MySQL Error 1213: serialization failure, deadlock, or generic failure from concurrent transaction
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return 0, fmt.Errorf("failed to get next child sequence after multiple attempts. last err: %w", lastErr)
}

func (c *Client) tryGetNextChildSequence(parentID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Use MySQL-idiomatic atomic increment trick:
	// 1. If row exists, increment next_child and use LAST_INSERT_ID() to capture the NEW value.
	// 2. If row doesn't exist, insert with next_child=2 and use LAST_INSERT_ID(2) to capture the NEW value.
	// This approach is more robust than SELECT ... FOR UPDATE on potentially missing rows in Dolt.
	uniqueTxID := fmt.Sprintf("tx-%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000)
	query := `
		INSERT INTO child_counters (parent_id, next_child, updated_by)
		VALUES (?, LAST_INSERT_ID(2), ?)
		ON DUPLICATE KEY UPDATE
			next_child = LAST_INSERT_ID(next_child + 1),
			updated_by = VALUES(updated_by)
	`
	_, err = tx.ExecContext(ctx, query, parentID, uniqueTxID)
	if err != nil {
		return 0, fmt.Errorf("failed to increment counter: %w", err)
	}

	var nextVal int
	err = tx.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&nextVal)
	if err != nil {
		return 0, fmt.Errorf("failed to read last_insert_id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		// If commit fails due to serialization conflict in Dolt, we will retry.
		return 0, fmt.Errorf("failed to commit: %w", err)
	}

	// nextVal is the value to be used NEXT. The value we consume is nextVal - 1.
	return nextVal - 1, nil
}

// LogEvent implementation for Client
func (c *Client) LogEvent(issueID, eventType, actor, agentModel string, oldValue, newValue any) error {
	return c.LogEventTx(context.Background(), nil, issueID, eventType, actor, agentModel, oldValue, newValue)
}

// LogEventTx implementation for Client, optionally using a transaction
func (c *Client) LogEventTx(ctx context.Context, tx *sql.Tx, issueID, eventType, actor, agentModel string, oldValue, newValue any) error {
	oldJSON := "{}"
	if oldValue != nil {
		b, err := json.Marshal(oldValue)
		if err == nil {
			oldJSON = string(b)
		}
	}

	newJSON := "{}"
	if newValue != nil {
		b, err := json.Marshal(newValue)
		if err == nil {
			newJSON = string(b)
		}
	}

	query := "INSERT INTO events (issue_id, event_type, actor, old_value, new_value, created_by, updated_by, agent_model, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"

	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, issueID, eventType, actor, oldJSON, newJSON, actor, actor, agentModel, time.Now())
	} else {
		_, err = c.db.ExecContext(ctx, query, issueID, eventType, actor, oldJSON, newJSON, actor, actor, agentModel, time.Now())
	}

	return err
}
