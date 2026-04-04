// Package issues contains the issue management commands (create, show, list, update, drop, assign, label, comment, subtask, quick).
package issues

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/cmddeps"
	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
)

// CreateAffectedFiles is the StringSliceVar target for --files on the create command.
// Tests may reset this to nil between runs.
var CreateAffectedFiles []string

// UpdateAffectedFiles is the StringSliceVar target for --files on the update command.
var UpdateAffectedFiles []string

// SubtaskAffectedFiles is the StringSliceVar target for --files on the subtask command.
var SubtaskAffectedFiles []string

// StdinReader is overridable in tests to simulate interactive input for the drop command.
var StdinReader io.Reader = os.Stdin

var (
	dropForce        bool
	showTree         bool
	commentLastCommit string
)

// IssueListItem is the JSON output model for list/search/quick results.
type IssueListItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	Priority  int       `json:"priority"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// IssueDetail is the JSON output model for the show command.
type IssueDetail struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Type          string    `json:"type"`
	Priority      int       `json:"priority"`
	PriorityLevel string    `json:"priority_level"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CreatedBy     string    `json:"created_by"`
	UpdatedBy     string    `json:"updated_by"`
	AgentModel    string    `json:"agent_model,omitempty"`
	AffectedFiles []string       `json:"affected_files,omitempty"`
	Subtasks      []string       `json:"subtasks,omitempty"`
	Labels        []string       `json:"labels,omitempty"`
	Comments      []CommentEntry `json:"comments,omitempty"`
}

// CommentEntry is the JSON output model for a single comment in show output.
type CommentEntry struct {
	ID         int64     `json:"id"`
	Message    string    `json:"message"`
	Actor      string    `json:"actor"`
	AgentModel string    `json:"agent_model,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SortColumnMap maps CLI field names to SQL column names.
var SortColumnMap = map[string]string{
	"id":       "id",
	"title":    "title",
	"type":     "issue_type",
	"status":   "status",
	"priority": "priority",
	"created":  "created_at",
	"updated":  "updated_at",
	"assignee": "assignee",
}

// ParseSortFlag converts a CLI --sort string into an ORDER BY clause.
func ParseSortFlag(sortStr string) (string, error) {
	if sortStr == "" {
		return "priority ASC, created_at DESC, id ASC", nil
	}

	parts := strings.Split(sortStr, ",")
	var segments []string

	for _, p := range parts {
		subparts := strings.Split(p, ":")
		field := strings.ToLower(strings.TrimSpace(subparts[0]))
		col, ok := SortColumnMap[field]
		if !ok {
			return "", fmt.Errorf("invalid sort field %q", field)
		}

		order := "ASC"
		if len(subparts) > 1 {
			o := strings.ToUpper(strings.TrimSpace(subparts[1]))
			if o != "ASC" && o != "DESC" {
				return "", fmt.Errorf("invalid order %q for field %q", subparts[1], field)
			}
			order = o
		}
		segments = append(segments, fmt.Sprintf("%s %s", col, order))
	}

	segments = append(segments, "id ASC")
	return strings.Join(segments, ", "), nil
}

// AddCommands registers all issue management commands with the root cobra.Command.
// d provides runtime dependencies (Store, actor, agentModel, outputJSON).
func AddCommands(root *cobra.Command, d *cmddeps.Deps) {
	root.AddCommand(newCreateCmd(d))
	root.AddCommand(newShowCmd(d))
	root.AddCommand(newListCmd(d))
	root.AddCommand(newUpdateCmd(d))
	root.AddCommand(newDropCmd(d))
	root.AddCommand(newAssignCmd(d))
	root.AddCommand(newLabelCmd(d))
	root.AddCommand(newCommentCmd(d))
	root.AddCommand(newSubtaskCmd(d))
	root.AddCommand(newQuickCmd(d))
	root.AddCommand(newClaimCmd(d))
	root.AddCommand(newStartCmd(d))
	root.AddCommand(newStopCmd(d))
}

func newShowCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show details of an issue",
		Long:  `Display detailed information about a specific issue.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			if showTree {
				return showTreeVisualization(d, id)
			}

			query := `SELECT title, description, issue_type, priority, status, created_at, updated_at, created_by, updated_by, agent_model, affected_files
                  FROM issues WHERE id = ?`

			var title, desc, iType, status string
			var priority int
			var createdAt, updatedAt time.Time
			var createdBy, updatedBy string
			var agentModelStr *string
			var affectedFilesJSON *string

			err := (*d.Store).QueryRow(query, id).Scan(&title, &desc, &iType, &priority, &status, &createdAt, &updatedAt, &createdBy, &updatedBy, &agentModelStr, &affectedFilesJSON)
			if err != nil {
				return fmt.Errorf("failed to fetch issue %s: %w", id, err)
			}

			pStr := "backlog"
			switch priority {
			case 0:
				pStr = "critical"
			case 1:
				pStr = "high"
			case 2:
				pStr = "medium"
			case 3:
				pStr = "low"
			}

			var files []string
			if affectedFilesJSON != nil && *affectedFilesJSON != "" && *affectedFilesJSON != "[]" {
				_ = json.Unmarshal([]byte(*affectedFilesJSON), &files)
			}

			// Query child subtasks via the dependencies table (canonical source for parent-child)
			subtaskRows, err := (*d.Store).Query(
				`SELECT d.from_id FROM dependencies d WHERE d.to_id = ? AND d.type = 'subtask-of' ORDER BY d.from_id`,
				id,
			)
			if err != nil {
				return fmt.Errorf("failed to query subtasks for %s: %w", id, err)
			}
			defer subtaskRows.Close() //nolint:errcheck
			var subtasks []string
			for subtaskRows.Next() {
				var childID string
				if scanErr := subtaskRows.Scan(&childID); scanErr == nil {
					subtasks = append(subtasks, childID)
				}
			}
			if err := subtaskRows.Err(); err != nil {
				return fmt.Errorf("error reading subtask rows for %s: %w", id, err)
			}

			// Query labels from issue_labels table
			labelRows, err := (*d.Store).Query(
				`SELECT label FROM issue_labels WHERE issue_id = ? ORDER BY label`, id,
			)
			if err != nil {
				return fmt.Errorf("failed to query labels for %s: %w", id, err)
			}
			defer labelRows.Close() //nolint:errcheck
			var labels []string
			for labelRows.Next() {
				var label string
				if scanErr := labelRows.Scan(&label); scanErr == nil {
					labels = append(labels, label)
				}
			}
			if err := labelRows.Err(); err != nil {
				return fmt.Errorf("error reading label rows for %s: %w", id, err)
			}

			// Query comments from issue_comments table
			commentRows, err := (*d.Store).Query(
				`SELECT id, message, COALESCE(actor, ''), COALESCE(agent_model, ''), created_at FROM issue_comments WHERE issue_id = ? ORDER BY created_at`, id,
			)
			if err != nil {
				return fmt.Errorf("failed to query comments for %s: %w", id, err)
			}
			defer commentRows.Close() //nolint:errcheck
			var comments []CommentEntry
			for commentRows.Next() {
				var c CommentEntry
				if scanErr := commentRows.Scan(&c.ID, &c.Message, &c.Actor, &c.AgentModel, &c.CreatedAt); scanErr == nil {
					comments = append(comments, c)
				}
			}
			if err := commentRows.Err(); err != nil {
				return fmt.Errorf("error reading comment rows for %s: %w", id, err)
			}

			if *d.OutputJSON {
				detail := IssueDetail{
					ID:            id,
					Title:         title,
					Description:   desc,
					Type:          iType,
					Priority:      priority,
					PriorityLevel: pStr,
					Status:        status,
					CreatedAt:     createdAt,
					UpdatedAt:     updatedAt,
					CreatedBy:     createdBy,
					UpdatedBy:     updatedBy,
					AffectedFiles: files,
					Subtasks:      subtasks,
					Labels:        labels,
					Comments:      comments,
				}
				if agentModelStr != nil {
					detail.AgentModel = *agentModelStr
				}
				b, err := json.MarshalIndent(detail, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			cmd.Printf("ID:          %s\n", id)
			cmd.Printf("Title:       %s\n", title)
			cmd.Printf("Type:        %s\n", iType)
			cmd.Printf("Priority:    %s (%d)\n", pStr, priority)
			if status == "tombstone" {
				status = "🗑️  DELETED (tombstone)"
			}
			cmd.Printf("Status:      %s\n", status)
			cmd.Printf("Created:     %s by %s\n", createdAt.Format(time.RFC3339), createdBy)
			cmd.Printf("Updated:     %s by %s\n", updatedAt.Format(time.RFC3339), updatedBy)
			if agentModelStr != nil && *agentModelStr != "" {
				cmd.Printf("Model:       %s\n", *agentModelStr)
			}
			if len(files) > 0 {
				cmd.Printf("Files:       %v\n", files)
			}
			if len(subtasks) > 0 {
				cmd.Printf("Subtasks:    %v\n", subtasks)
			}
			if len(labels) > 0 {
				cmd.Printf("Labels:      %v\n", labels)
			}
			if len(comments) > 0 {
				cmd.Printf("\nComments:\n")
				for _, c := range comments {
					cmd.Printf("  [%s] %s: %s\n", c.CreatedAt.Format(time.RFC3339), c.Actor, c.Message)
				}
			}
			cmd.Printf("\nDescription:\n%s\n", desc)

			return nil
		},
	}

	cmd.Flags().BoolVar(&showTree, "tree", false, "Show hierarchical tree visualization")
	return cmd
}

func showTreeVisualization(d *cmddeps.Deps, rootID string) error {
	dag, err := graph.LoadGraphFromDB(*d.Store)
	if err != nil {
		return fmt.Errorf("failed to load graph: %w", err)
	}

	if !dag.HasNode(rootID) {
		return fmt.Errorf("issue %s not found in graph", rootID)
	}

	fmt.Printf("Hierarchical Tree for %s:\n\n", rootID)
	renderTreeNode(dag, rootID, "", true, true)
	fmt.Println()
	return nil
}

func renderTreeNode(dag *graph.AdjacencyDAG, id string, indent string, isLast bool, isRoot bool) {
	node, _ := dag.GetNode(id)
	children := dag.GetTreeChildren(id)

	marker := ""
	if !isRoot {
		marker = "├── "
		if isLast {
			marker = "└── "
		}
	}

	glyph := "●"
	color := "\033[90m"
	switch node.Status {
	case graph.StatusClosed:
		glyph = "✔"
		color = "\033[32m"
	case graph.StatusInProgress:
		glyph = "▶"
		color = "\033[34m"
	case graph.StatusBlocked:
		glyph = "✖"
		color = "\033[31m"
	}
	reset := "\033[0m"

	progress := ""
	if len(children) > 0 {
		total := len(children)
		closed := 0
		for _, cid := range children {
			cn, _ := dag.GetNode(cid)
			if cn.Status == graph.StatusClosed {
				closed++
			}
		}
		percentage := (closed * 100) / total
		barWidth := 5
		filled := (percentage * barWidth) / 100
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		progress = fmt.Sprintf(" [%s] %d%%", bar, percentage)
	}

	fmt.Printf("%s%s%s%s%s %s (%s)%s %s\n",
		indent, marker, color, glyph, reset, id, node.Type, progress, node.Title)

	newIndent := indent
	if !isRoot {
		if isLast {
			newIndent += "    "
		} else {
			newIndent += "│   "
		}
	}

	for i, cid := range children {
		renderTreeNode(dag, cid, newIndent, i == len(children)-1, false)
	}
}

func newListCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all issues",
		Long: `List all issues in the Grava tracker.
You can filter by status or type, and sort by various criteria.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			listStatus, _ := cmd.Flags().GetString("status")
			listType, _ := cmd.Flags().GetString("type")
			listWisp, _ := cmd.Flags().GetBool("wisp")
			listSort, _ := cmd.Flags().GetString("sort")

			query := "SELECT id, title, issue_type, priority, status, created_at FROM issues"
			var params []any

			whereParts := []string{}
			params = []any{}

			if listWisp {
				whereParts = append(whereParts, "ephemeral = 1")
			} else {
				whereParts = append(whereParts, "ephemeral = 0")
			}

			whereParts = append(whereParts, "status != 'tombstone'")

			if listStatus != "" {
				whereParts = append(whereParts, "status = ?")
				params = append(params, listStatus)
			}
			if listType != "" {
				whereParts = append(whereParts, "issue_type = ?")
				params = append(params, listType)
			}

			if len(whereParts) > 0 {
				query += " WHERE "
				for i, part := range whereParts {
					if i > 0 {
						query += " AND "
					}
					query += part
				}
			}

			sortClause, err := ParseSortFlag(listSort)
			if err != nil {
				return err
			}
			query += " ORDER BY " + sortClause

			rows, err := (*d.Store).Query(query, params...)
			if err != nil {
				return fmt.Errorf("failed to query issues: %w", err)
			}
			defer rows.Close() //nolint:errcheck

			var results []IssueListItem
			count := 0

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if !*d.OutputJSON {
				_, _ = fmt.Fprintln(w, "ID\tTitle\tType\tPriority\tStatus\tCreated")
			}

			for rows.Next() {
				var id, title, iType, status string
				var priority int
				var createdAt time.Time
				if err := rows.Scan(&id, &title, &iType, &priority, &status, &createdAt); err != nil {
					return fmt.Errorf("failed to scan row: %w", err)
				}

				if *d.OutputJSON {
					results = append(results, IssueListItem{
						ID:        id,
						Title:     title,
						Type:      iType,
						Priority:  priority,
						Status:    status,
						CreatedAt: createdAt,
					})
				} else {
					count++
					if len(title) > 50 {
						title = title[:47] + "..."
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
						id, title, iType, priority, status, createdAt.Format("2006-01-02"))
				}
			}

			if *d.OutputJSON {
				b, err := json.MarshalIndent(results, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				_ = count
				w.Flush() //nolint:errcheck
			}

			return nil
		},
	}

	cmd.Flags().StringP("status", "s", "", "Filter by status")
	cmd.Flags().StringP("type", "t", "", "Filter by type")
	cmd.Flags().Bool("wisp", false, "Show only ephemeral Wisp issues")
	cmd.Flags().String("sort", "", "Sort by fields (e.g. priority:asc,created:desc)")
	return cmd
}


func newDropCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Delete ALL data from the Grava database (nuclear reset)",
		Long: `Drop deletes ALL data from every table in the Grava database.
This is a destructive, non-reversible operation intended for development
resets or clean-slate scenarios.

Tables are truncated in foreign-key-safe order:
  1. dependencies
  2. events
  3. deletions
  4. child_counters
  5. issues

Example:
  grava drop           # prompts for confirmation
  grava drop --force   # skip confirmation (for CI/scripts)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dropForce && !*d.OutputJSON {
				cmd.Print("⚠️  This will DELETE ALL DATA from the Grava database.\nType \"yes\" to confirm: ")

				scanner := bufio.NewScanner(StdinReader)
				scanner.Scan()
				answer := strings.TrimSpace(scanner.Text())

				if answer != "yes" {
					cmd.Println("Aborted. No data was deleted.")
					return fmt.Errorf("user cancelled drop operation")
				}
			}

			tables := []string{
				"dependencies",
				"events",
				"deletions",
				"child_counters",
				"issues",
			}

			ctx := context.Background()
			tx, err := (*d.Store).BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to start transaction: %w", err)
			}
			defer tx.Rollback() //nolint:errcheck

			for _, table := range tables {
				_, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", table))
				if err != nil {
					return fmt.Errorf("failed to delete from %s: %w", table, err)
				}
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			if *d.OutputJSON {
				resp := map[string]string{
					"status": "dropped",
					"note":   "All data deleted from every table",
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			cmd.Println("💣 All Grava data has been dropped.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&dropForce, "force", false, "Skip interactive confirmation prompt")
	return cmd
}

func newQuickCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "quick <title>",
		Short: "Create an issue quickly with defaults",
		Long: `Quick creates a new issue with a single argument — the title.
Defaults: type=task, priority=medium. The old list behavior is available via:
  grava list --sort priority:asc

Examples:
  grava quick "Fix login bug"
  grava quick "Add dark mode support"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := createIssue(cmd.Context(), *d.Store, CreateParams{
				Title:     args[0],
				IssueType: "task",
				Priority:  "medium",
				Actor:     *d.Actor,
				Model:     *d.AgentModel,
			})
			if err != nil {
				if *d.OutputJSON {
					return writeJSONError(cmd, err)
				}
				return err
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(result, "", "  ")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Created issue: %s\n", result.ID)
			return nil
		},
	}
}

// updateIssueMetadata updates the metadata column for an issue.
func updateIssueMetadata(d *cmddeps.Deps, id string, meta map[string]any) error {
	updated, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = (*d.Store).Exec(
		`UPDATE issues SET metadata = ?, updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?`,
		string(updated), *d.Actor, *d.AgentModel, id,
	)
	if err != nil {
		return fmt.Errorf("failed to save metadata for %s: %w", id, err)
	}

	return nil
}

// setLastCommit stores the commit hash in the issue's metadata.
func setLastCommit(d *cmddeps.Deps, id string, hash string) error {
	row := (*d.Store).QueryRow(`SELECT COALESCE(metadata, '{}') FROM issues WHERE id = ?`, id)
	var rawMeta string
	if err := row.Scan(&rawMeta); err != nil {
		return fmt.Errorf("issue %s not found: %w", id, err)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil {
		return fmt.Errorf("failed to parse metadata for %s: %w", id, err)
	}

	meta["last_commit"] = hash

	return updateIssueMetadata(d, id, meta)
}
