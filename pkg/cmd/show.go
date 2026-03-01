package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/graph"
	"github.com/spf13/cobra"
)

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

var (
	showTree bool
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show details of an issue",
	Long:  `Display detailed information about a specific issue.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		if showTree {
			return showTreeVisualization(id)
		}

		query := `SELECT title, description, issue_type, priority, status, created_at, updated_at, created_by, updated_by, agent_model, affected_files 
                  FROM issues WHERE id = ?`
		// ... rest of details logic ...

		var title, desc, iType, status string
		var priority int
		var createdAt, updatedAt time.Time
		var createdBy, updatedBy string
		var agentModelStr *string
		var affectedFilesJSON *string

		err := Store.QueryRow(query, id).Scan(&title, &desc, &iType, &priority, &status, &createdAt, &updatedAt, &createdBy, &updatedBy, &agentModelStr, &affectedFilesJSON)
		if err != nil {
			return fmt.Errorf("failed to fetch issue %s: %w", id, err)
		}

		// Map priority back via array or switch
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

		if outputJSON {
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

func init() {
	rootCmd.AddCommand(showCmd)
	showCmd.Flags().BoolVar(&showTree, "tree", false, "Show hierarchical tree visualization")
}

func showTreeVisualization(rootID string) error {
	dag, err := graph.LoadGraphFromDB(Store)
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

	// Status glyph and color
	glyph := "●"
	color := "\033[90m" // Gray (open)
	switch node.Status {
	case graph.StatusClosed:
		glyph = "✔"
		color = "\033[32m" // Green
	case graph.StatusInProgress:
		glyph = "▶"
		color = "\033[34m" // Blue
	case graph.StatusBlocked:
		glyph = "✖"
		color = "\033[31m" // Red
	}
	reset := "\033[0m"

	// Progress
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

		// Small progress bar
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
