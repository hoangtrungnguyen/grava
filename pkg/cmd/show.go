package cmd

import (
	"encoding/json"
	"fmt"
	"time"

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

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show details of an issue",
	Long:  `Display detailed information about a specific issue.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		query := `SELECT title, description, issue_type, priority, status, created_at, updated_at, created_by, updated_by, agent_model, affected_files 
                  FROM issues WHERE id = ?`

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
			fmt.Fprintln(cmd.OutOrStdout(), string(b))
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
}
