package dolt

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Store defines the interface for database interactions needed by the ID generator.
type Store interface {
	// GetNextChildSequence atomically increments the counter for the given parentID and returns the new sequence number.
	GetNextChildSequence(parentID string) (int, error)
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Close() error
}

// Client implements Store using a SQL database.
type Client struct {
	db *sql.DB
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

func (c *Client) Exec(query string, args ...any) (sql.Result, error) {
	return c.db.Exec(query, args...)
}

func (c *Client) QueryRow(query string, args ...any) *sql.Row {
	return c.db.QueryRow(query, args...)
}

func (c *Client) Query(query string, args ...any) (*sql.Rows, error) {
	return c.db.Query(query, args...)
}

// GetNextChildSequence uses Advisory Locks (GET_LOCK) on a dedicated connection.
func (c *Client) GetNextChildSequence(parentID string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Get a dedicated connection from the pool
	conn, err := c.db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to obtain connection: %w", err)
	}
	defer conn.Close() // Returns connection to pool

	// 1. Acquire Lock
	lockName := fmt.Sprintf("grava_cc_%s", parentID)
	if len(lockName) > 64 {
		lockName = lockName[:64]
	}

	var locked int
	err = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 10)", lockName).Scan(&locked)
	if err != nil {
		return 0, fmt.Errorf("failed to acquire lock: %w", err)
	}
	if locked != 1 {
		return 0, fmt.Errorf("timeout acquiring lock for %s", parentID)
	}

	// Release lock explicitly before connection is returned to pool
	defer func() {
		conn.ExecContext(ctx, "SELECT RELEASE_LOCK(?)", lockName)
	}()

	// 2. Read-Modify-Write
	var current int
	err = conn.QueryRowContext(ctx, "SELECT next_child FROM child_counters WHERE parent_id = ?", parentID).Scan(&current)

	if err == sql.ErrNoRows {
		// Insert initial
		_, err = conn.ExecContext(ctx, "INSERT INTO child_counters (parent_id, next_child) VALUES (?, 2)", parentID)
		if err != nil {
			return 0, fmt.Errorf("failed to insert counter: %w", err)
		}
		// Return 1 as the first value (we just consumed 1, next is 2)
		return 1, nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to read counter: %w", err)
	}

	// Update
	_, err = conn.ExecContext(ctx, "UPDATE child_counters SET next_child = ? WHERE parent_id = ?", current+1, parentID)
	if err != nil {
		return 0, fmt.Errorf("failed to update counter: %w", err)
	}

	return current, nil
}
