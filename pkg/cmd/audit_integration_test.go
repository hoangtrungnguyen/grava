package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
	"github.com/hoangtrungnguyen/grava/pkg/migrate"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/subosito/gotenv"
)

func TestCreateAuditIntegration(t *testing.T) {
	// Try to load .env.test
	_ = gotenv.Load("../../.env.test")
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		t.Skip("Skipping integration test: DB_URL not set")
	}

	// 1. Setup real Store
	client, err := dolt.NewClient(dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer client.Close() //nolint:errcheck
	Store = client

	// Prevent rootCmd from closing our connection between commands
	originalPostRun := rootCmd.PersistentPostRunE
	rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		return nil
	}
	defer func() { rootCmd.PersistentPostRunE = originalPostRun }()

	// 2. Run migrations
	if err := migrate.Run(client.DB()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// 3. Clear data
	_, _ = client.Exec("DELETE FROM issues")
	_, _ = client.Exec("DELETE FROM dependencies")
	_, _ = client.Exec("DELETE FROM events")
	_, _ = client.Exec("DELETE FROM child_counters")

	// 4. Create a parent issue
	output, err := executeCommand(rootCmd, "create", "--title", "Parent Task")
	assert.NoError(t, err)
	assert.Contains(t, output, "Created issue:")

	// Extract ID (e.g. grava-a1b2)
	idParts := strings.Split(output, ": ")
	parentID := strings.TrimSpace(idParts[len(idParts)-1])

	// 5. Create a subtask
	output, err = executeCommand(rootCmd, "subtask", parentID, "--title", "Child Task")
	assert.NoError(t, err)
	assert.Contains(t, output, "Created subtask:")

	idParts = strings.Split(output, ": ")
	childID := strings.TrimSpace(idParts[len(idParts)-1])

	// 6. Verify Database State
	// Verify issue
	var title string
	err = client.QueryRow("SELECT title FROM issues WHERE id = ?", childID).Scan(&title)
	assert.NoError(t, err)
	assert.Equal(t, "Child Task", title)

	// Verify dependency
	var fromID, toID, depType string
	err = client.QueryRow("SELECT from_id, to_id, type FROM dependencies WHERE from_id = ?", childID).Scan(&fromID, &toID, &depType)
	assert.NoError(t, err)
	assert.Equal(t, childID, fromID)
	assert.Equal(t, parentID, toID)
	assert.Equal(t, "subtask-of", depType)

	// Verify Audit Log
	var count int
	err = client.QueryRow("SELECT COUNT(*) FROM events WHERE issue_id = ? AND event_type = 'create'", childID).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count, "Subtask creation should be logged")

	err = client.QueryRow("SELECT COUNT(*) FROM events WHERE issue_id = ? AND event_type = 'dependency_add'", childID).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count, "Subtask relationship should be logged on the child")

	// 7. Verify Wisp (ephemeral)
	output, err = executeCommand(rootCmd, "create", "--title", "Temporary Note", "--ephemeral")
	assert.NoError(t, err)
	assert.Contains(t, output, "Wisp")

	idParts = strings.Split(output, ": ")
	wispID := strings.TrimSpace(idParts[len(idParts)-1])

	var ephemeral int
	err = client.QueryRow("SELECT ephemeral FROM issues WHERE id = ?", wispID).Scan(&ephemeral)
	assert.NoError(t, err)
	assert.Equal(t, 1, ephemeral, "Wisp should have ephemeral=1")
}
