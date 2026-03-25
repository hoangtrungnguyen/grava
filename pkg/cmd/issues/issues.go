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
	"github.com/hoangtrungnguyen/grava/pkg/idgen"
	"github.com/hoangtrungnguyen/grava/pkg/validation"
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
	dropForce     bool
	showTree      bool
	quickPriority int
	quickLimit    int
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
	AffectedFiles []string  `json:"affected_files,omitempty"`
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
}

func newCreateCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new issue",
		Long: `Create a new issue in the Grava tracker.
You can specify title, description, type, and priority.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			title, _ := cmd.Flags().GetString("title")
			desc, _ := cmd.Flags().GetString("desc")
			issueType, _ := cmd.Flags().GetString("type")
			priority, _ := cmd.Flags().GetString("priority")
			parentID, _ := cmd.Flags().GetString("parent")
			ephemeral, _ := cmd.Flags().GetBool("ephemeral")

			affectedFiles := CreateAffectedFiles

			generator := idgen.NewStandardGenerator(*d.Store)

			if err := validation.ValidateIssueType(issueType); err != nil {
				return err
			}

			pInt, err := validation.ValidatePriority(priority)
			if err != nil {
				return err
			}

			var id string
			if parentID != "" {
				id, err = generator.GenerateChildID(parentID)
				if err != nil {
					return fmt.Errorf("failed to generate child ID: %w", err)
				}
			} else {
				id = generator.GenerateBaseID()
			}

			ephemeralVal := 0
			if ephemeral {
				ephemeralVal = 1
			}

			affectedFilesJSON := "[]"
			if len(affectedFiles) > 0 {
				b, _ := json.Marshal(affectedFiles)
				affectedFilesJSON = string(b)
			}

			ctx := context.Background()
			tx, err := (*d.Store).BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to start transaction: %w", err)
			}
			defer tx.Rollback() //nolint:errcheck

			query := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model, affected_files)
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

			_, err = tx.ExecContext(ctx, query, id, title, desc, issueType, pInt, "open", ephemeralVal, time.Now(), time.Now(), *d.Actor, *d.Actor, *d.AgentModel, affectedFilesJSON)
			if err != nil {
				return fmt.Errorf("failed to insert issue: %w", err)
			}

			if parentID != "" {
				var exists int
				err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM issues WHERE id = ?", parentID).Scan(&exists)
				if err != nil {
					return fmt.Errorf("failed to check parent existence: %w", err)
				}
				if exists == 0 {
					return fmt.Errorf("parent issue %s not found", parentID)
				}

				depQuery := `INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`
				_, err = tx.ExecContext(ctx, depQuery, id, parentID, "subtask-of", *d.Actor, *d.Actor, *d.AgentModel)
				if err != nil {
					return fmt.Errorf("failed to create subtask-of dependency: %w", err)
				}
			}

			err = (*d.Store).LogEventTx(ctx, tx, id, "create", *d.Actor, *d.AgentModel, nil, map[string]interface{}{
				"title":    title,
				"type":     issueType,
				"priority": pInt,
				"status":   "open",
			})
			if err != nil {
				return fmt.Errorf("failed to log event: %w", err)
			}

			if parentID != "" {
				err = (*d.Store).LogEventTx(ctx, tx, id, "dependency_add", *d.Actor, *d.AgentModel, nil, map[string]interface{}{
					"to_id": parentID,
					"type":  "subtask-of",
				})
				if err != nil {
					return fmt.Errorf("failed to log dependency event: %w", err)
				}
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			if *d.OutputJSON {
				resp := map[string]string{
					"id":     id,
					"status": "created",
				}
				if ephemeral {
					resp["ephemeral"] = "true"
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				return nil
			}

			if ephemeral {
				cmd.Printf("👻 Created ephemeral issue (Wisp): %s\n", id)
			} else {
				cmd.Printf("✅ Created issue: %s\n", id)
			}

			return nil
		},
	}

	cmd.Flags().StringP("title", "t", "", "Issue title (required)")
	cmd.Flags().StringP("desc", "d", "", "Issue description")
	cmd.Flags().String("type", "task", "Issue type (task, bug, epic, story)")
	cmd.Flags().StringP("priority", "p", "medium", "Issue priority (low, medium, high, critical)")
	cmd.Flags().String("parent", "", "Parent Issue ID for sub-tasks")
	cmd.Flags().Bool("ephemeral", false, "Mark issue as ephemeral (Wisp) — excluded from normal queries")
	cmd.Flags().StringSliceVar(&CreateAffectedFiles, "files", []string{}, "Affected files (comma separated)")
	cmd.MarkFlagRequired("title") //nolint:errcheck

	return cmd
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

func newUpdateCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update an existing issue",
		Long: `Update specific fields of an existing issue.
Only the flags provided will be updated.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]

			query := "UPDATE issues SET updated_at = ?, updated_by = ?, agent_model = ?"
			queryParams := []any{time.Now(), *d.Actor, *d.AgentModel}

			if cmd.Flags().Changed("title") {
				val, _ := cmd.Flags().GetString("title")
				query += ", title = ?"
				queryParams = append(queryParams, val)
			}
			if cmd.Flags().Changed("desc") {
				val, _ := cmd.Flags().GetString("desc")
				query += ", description = ?"
				queryParams = append(queryParams, val)
			}
			if cmd.Flags().Changed("type") {
				val, _ := cmd.Flags().GetString("type")
				if err := validation.ValidateIssueType(val); err != nil {
					return err
				}
				query += ", issue_type = ?"
				queryParams = append(queryParams, val)
			}
			if cmd.Flags().Changed("priority") {
				query += ", priority = ?"
				val, _ := cmd.Flags().GetString("priority")
				pInt, err := validation.ValidatePriority(val)
				if err != nil {
					return err
				}
				queryParams = append(queryParams, pInt)
			}
			if cmd.Flags().Changed("status") {
				statusVal, _ := cmd.Flags().GetString("status")
				if err := validation.ValidateStatus(statusVal); err != nil {
					return err
				}

				dag, err := graph.LoadGraphFromDB(*d.Store)
				if err != nil {
					return fmt.Errorf("failed to load graph for status propagation: %w", err)
				}
				dag.SetSession(*d.Actor, *d.AgentModel)

				err = dag.SetNodeStatus(id, graph.IssueStatus(statusVal))
				if err != nil {
					return fmt.Errorf("failed to update status via graph: %w", err)
				}
			}
			if cmd.Flags().Changed("files") {
				query += ", affected_files = ?"
				val := UpdateAffectedFiles
				b, _ := json.Marshal(val)
				queryParams = append(queryParams, string(b))
			}

			query += " WHERE id = ?"
			queryParams = append(queryParams, id)

			result, err := (*d.Store).Exec(query, queryParams...)
			if err != nil {
				return fmt.Errorf("failed to update issue %s: %w", id, err)
			}

			if cmd.Flags().Changed("last-commit") {
				val, _ := cmd.Flags().GetString("last-commit")
				if err := setLastCommit(d, id, val); err != nil {
					return err
				}
			}

			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("failed to get rows affected: %w", err)
			}

			if rowsAffected == 0 && !cmd.Flags().Changed("last-commit") {
				return fmt.Errorf("issue %s not found or no changes made", id)
			}

			if *d.OutputJSON {
				resp := map[string]string{
					"id":     id,
					"status": "updated",
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Updated issue %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringP("title", "t", "", "Update title")
	cmd.Flags().StringP("desc", "d", "", "Update description")
	cmd.Flags().String("type", "", "Update type")
	cmd.Flags().StringP("priority", "p", "", "Update priority")
	cmd.Flags().StringP("status", "s", "", "Update status")
	cmd.Flags().StringSliceVar(&UpdateAffectedFiles, "files", []string{}, "Update affected files")
	cmd.Flags().String("last-commit", "", "Store the last session's commit hash")
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

func newAssignCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "assign <id> <user>",
		Short: "Assign an issue to a user or agent",
		Long: `Set the assignee field on an existing issue.

The assignee can be a human username or an agent identity string.
Passing an empty string ("") clears the assignee.

Example:
  grava assign grava-abc alice
  grava assign grava-abc "agent:planner-v2"
  grava assign grava-abc ""   # unassign`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			user := args[1]

			result, err := (*d.Store).Exec(
				`UPDATE issues SET assignee = ?, updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?`,
				user, *d.Actor, *d.AgentModel, id,
			)
			if err != nil {
				return fmt.Errorf("failed to assign issue %s: %w", id, err)
			}

			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("failed to get rows affected: %w", err)
			}
			if rowsAffected == 0 {
				return fmt.Errorf("issue %s not found", id)
			}

			if *d.OutputJSON {
				resp := map[string]string{
					"id":       id,
					"status":   "updated",
					"field":    "assignee",
					"assignee": user,
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				return nil
			}

			if user == "" {
				cmd.Printf("👤 Assignee cleared on %s\n", id)
			} else {
				cmd.Printf("👤 Assigned %s to %s\n", id, user)
			}
			return nil
		},
	}
}

func newLabelCmd(d *cmddeps.Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "label <id> <label>",
		Short: "Add a label to an issue",
		Long: `Add a label to an existing issue.

Labels are stored as a JSON array in the issue's metadata column.
Adding a label that already exists is a no-op (idempotent).

Example:
  grava label grava-abc "needs-review"
  grava label grava-abc "priority:high"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			label := args[1]

			row := (*d.Store).QueryRow(`SELECT COALESCE(metadata, '{}') FROM issues WHERE id = ?`, id)
			var rawMeta string
			if err := row.Scan(&rawMeta); err != nil {
				return fmt.Errorf("issue %s not found: %w", id, err)
			}

			var meta map[string]any
			if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil {
				return fmt.Errorf("failed to parse metadata for %s: %w", id, err)
			}

			var labels []string
			if existing, ok := meta["labels"]; ok {
				if arr, ok := existing.([]any); ok {
					for _, v := range arr {
						if s, ok := v.(string); ok {
							labels = append(labels, s)
						}
					}
				}
			}

			for _, l := range labels {
				if l == label {
					if *d.OutputJSON {
						resp := map[string]string{
							"id":     id,
							"status": "unchanged",
							"field":  "labels",
							"note":   fmt.Sprintf("Label %q already present", label),
						}
						b, _ := json.MarshalIndent(resp, "", "  ")
						_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
						return nil
					}
					cmd.Printf("🏷️  Label %q already present on %s\n", label, id)
					return nil
				}
			}
			labels = append(labels, label)
			meta["labels"] = labels

			if err := updateIssueMetadata(d, id, meta); err != nil {
				return fmt.Errorf("failed to save label on %s: %w", id, err)
			}

			if *d.OutputJSON {
				resp := map[string]string{
					"id":     id,
					"status": "updated",
					"field":  "labels",
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			cmd.Printf("🏷️  Label %q added to %s\n", label, id)
			return nil
		},
	}
}

func newCommentCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment <id> <text>",
		Short: "Append a comment to an issue",
		Long: `Append a comment to an existing issue.

Comments are stored as a JSON array in the issue's metadata column.
Each comment entry records the text, the timestamp, and the actor.

Example:
  grava comment grava-abc "Investigated root cause, see PR #42"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			text := args[1]

			if err := addCommentToIssue(d, id, text); err != nil {
				return err
			}

			if cmd.Flags().Changed("last-commit") {
				if err := setLastCommit(d, id, commentLastCommit); err != nil {
					return err
				}
			}

			if *d.OutputJSON {
				resp := map[string]string{
					"id":     id,
					"status": "updated",
					"field":  "comments",
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(b)) //nolint:errcheck
				return nil
			}

			cmd.Printf("💬 Comment added to %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&commentLastCommit, "last-commit", "", "Store the last session's commit hash")
	return cmd
}

func newSubtaskCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subtask <parent_id>",
		Short: "Create a subtask",
		Long: `Create a new subtask for an existing issue.
The subtask ID will be hierarchical (e.g., parent_id.1).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parentID := args[0]
			subtaskTitle, _ := cmd.Flags().GetString("title")
			subtaskDesc, _ := cmd.Flags().GetString("desc")
			subtaskType, _ := cmd.Flags().GetString("type")
			subtaskPriority, _ := cmd.Flags().GetString("priority")
			subtaskEphemeral, _ := cmd.Flags().GetBool("ephemeral")

			generator := idgen.NewStandardGenerator(*d.Store)
			ctx := context.Background()

			if err := validation.ValidateIssueType(subtaskType); err != nil {
				return err
			}

			pInt, err := validation.ValidatePriority(subtaskPriority)
			if err != nil {
				return err
			}

			tx, err := (*d.Store).BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to start transaction: %w", err)
			}
			defer tx.Rollback() //nolint:errcheck

			var exists int
			err = tx.QueryRowContext(ctx, "SELECT 1 FROM issues WHERE id = ?", parentID).Scan(&exists)
			if err != nil {
				return fmt.Errorf("parent issue %s not found: %w", parentID, err)
			}

			id, err := generator.GenerateChildID(parentID)
			if err != nil {
				return fmt.Errorf("failed to generate subtask ID: %w", err)
			}

			ephemeralVal := 0
			if subtaskEphemeral {
				ephemeralVal = 1
			}

			affectedFilesJSON := "[]"
			if len(SubtaskAffectedFiles) > 0 {
				b, _ := json.Marshal(SubtaskAffectedFiles)
				affectedFilesJSON = string(b)
			}

			query := `INSERT INTO issues (id, title, description, issue_type, priority, status, ephemeral, created_at, updated_at, created_by, updated_by, agent_model, affected_files)
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

			_, err = tx.ExecContext(ctx, query, id, subtaskTitle, subtaskDesc, subtaskType, pInt, "open", ephemeralVal, time.Now(), time.Now(), *d.Actor, *d.Actor, *d.AgentModel, affectedFilesJSON)
			if err != nil {
				return fmt.Errorf("failed to insert subtask: %w", err)
			}

			depQuery := `INSERT INTO dependencies (from_id, to_id, type, created_by, updated_by, agent_model) VALUES (?, ?, ?, ?, ?, ?)`
			_, err = tx.ExecContext(ctx, depQuery, id, parentID, "subtask-of", *d.Actor, *d.Actor, *d.AgentModel)
			if err != nil {
				return fmt.Errorf("failed to create subtask-of dependency: %w", err)
			}

			err = (*d.Store).LogEventTx(ctx, tx, id, "create", *d.Actor, *d.AgentModel, nil, map[string]interface{}{
				"title":     subtaskTitle,
				"type":      subtaskType,
				"priority":  pInt,
				"parent_id": parentID,
			})
			if err != nil {
				return fmt.Errorf("failed to log event: %w", err)
			}

			err = (*d.Store).LogEventTx(ctx, tx, id, "dependency_add", *d.Actor, *d.AgentModel, nil, map[string]interface{}{
				"to_id": parentID,
				"type":  "subtask-of",
			})
			if err != nil {
				return fmt.Errorf("failed to log dependency event: %w", err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit transaction: %w", err)
			}

			if *d.OutputJSON {
				resp := map[string]string{
					"id":     id,
					"status": "created",
				}
				if subtaskEphemeral {
					resp["ephemeral"] = "true"
				}
				b, _ := json.MarshalIndent(resp, "", "  ")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
				return nil
			}

			if subtaskEphemeral {
				cmd.Printf("👻 Created ephemeral subtask (Wisp): %s\n", id)
			} else {
				cmd.Printf("✅ Created subtask: %s\n", id)
			}

			return nil
		},
	}

	cmd.Flags().StringP("title", "t", "", "Subtask title (required)")
	cmd.Flags().StringP("desc", "d", "", "Subtask description")
	cmd.Flags().String("type", "task", "Subtask type (task, bug, epic, story)")
	cmd.Flags().StringP("priority", "p", "medium", "Subtask priority (low, medium, high, critical)")
	cmd.Flags().Bool("ephemeral", false, "Mark subtask as ephemeral (Wisp)")
	cmd.Flags().StringSliceVar(&SubtaskAffectedFiles, "files", []string{}, "Affected files (comma separated)")
	cmd.MarkFlagRequired("title") //nolint:errcheck
	return cmd
}

func newQuickCmd(d *cmddeps.Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quick",
		Short: "List high-priority or quick tasks",
		Long: `Quick shows open issues at or above the given priority threshold.

Priority levels:
  0 = critical
  1 = high      (default threshold)
  2 = medium
  3 = low
  4 = backlog

Examples:
  grava quick                  # show critical + high priority open issues
  grava quick --priority 2     # include medium priority as well
  grava quick --limit 5        # cap results at 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			sql := `SELECT id, title, issue_type, priority, status, created_at
		        FROM issues
		        WHERE ephemeral = 0
		          AND status = 'open'
		          AND priority <= ?
		        ORDER BY priority ASC, created_at DESC
		        LIMIT ?`

			rows, err := (*d.Store).Query(sql, quickPriority, quickLimit)
			if err != nil {
				return fmt.Errorf("failed to query quick issues: %w", err)
			}
			defer rows.Close() //nolint:errcheck

			var results []IssueListItem
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if !*d.OutputJSON {
				_, _ = fmt.Fprintln(w, "ID\tTitle\tType\tPriority\tStatus\tCreated")
			}

			found := 0
			for rows.Next() {
				var id, title, iType, status string
				var prio int
				var createdAt time.Time
				if err := rows.Scan(&id, &title, &iType, &prio, &status, &createdAt); err != nil {
					return fmt.Errorf("failed to scan row: %w", err)
				}

				if *d.OutputJSON {
					results = append(results, IssueListItem{
						ID:        id,
						Title:     title,
						Type:      iType,
						Priority:  prio,
						Status:    status,
						CreatedAt: createdAt,
					})
				} else {
					if len(title) > 50 {
						title = title[:47] + "..."
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
						id, title, iType, prio, status, createdAt.Format("2006-01-02"))
				}
				found++
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("row iteration error: %w", err)
			}

			if *d.OutputJSON {
				b, _ := json.MarshalIndent(results, "", "  ")
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(b))
			} else {
				w.Flush() //nolint:errcheck
				if found == 0 {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "🎉 No high-priority open issues. You're all caught up!")
				} else {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n⚡ %d high-priority issue(s) need attention.\n", found)
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&quickPriority, "priority", 1, "Show issues at or above this priority level (0=critical, 1=high, 2=medium, 3=low)")
	cmd.Flags().IntVar(&quickLimit, "limit", 20, "Maximum number of results to return")
	return cmd
}

// addCommentToIssue appends a comment to the issue's metadata.
func addCommentToIssue(d *cmddeps.Deps, id string, text string) error {
	comment := map[string]any{
		"text":        text,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"actor":       *d.Actor,
		"agent_model": *d.AgentModel,
	}

	row := (*d.Store).QueryRow(`SELECT COALESCE(metadata, '{}') FROM issues WHERE id = ?`, id)
	var rawMeta string
	if err := row.Scan(&rawMeta); err != nil {
		return fmt.Errorf("issue %s not found: %w", id, err)
	}

	var meta map[string]any
	if err := json.Unmarshal([]byte(rawMeta), &meta); err != nil {
		return fmt.Errorf("failed to parse metadata for %s: %w", id, err)
	}

	var comments []any
	if existing, ok := meta["comments"]; ok {
		if arr, ok := existing.([]any); ok {
			comments = arr
		}
	}
	comments = append(comments, comment)
	meta["comments"] = comments

	return updateIssueMetadata(d, id, meta)
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
