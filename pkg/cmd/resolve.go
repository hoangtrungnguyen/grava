package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Interactive conflict resolution for issues.jsonl",
	Long:  "Provides an interactive prompt to resolve field-level JSONL merge conflicts.",
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := "issues.jsonl"
		if len(args) > 0 {
			filePath = args[0]
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filePath, err)
		}

		lines := strings.Split(string(content), "\n")
		resolvedLines := []string{}
		conflictsFound := 0

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if !strings.Contains(line, `"_conflict":true`) {
				resolvedLines = append(resolvedLines, line)
				continue
			}

			conflictsFound++
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				return fmt.Errorf("failed to parse conflicting line: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n⚠️  Conflict found in issue ID: %s\n", obj["id"])

			// Resolve fields
			for k, v := range obj {
				if k == "id" {
					continue
				}

				valMap, ok := v.(map[string]interface{})
				if !ok || valMap["_conflict"] != true {
					continue
				}

				localJSON, _ := json.MarshalIndent(valMap["local"], "", "  ")
				remoteJSON, _ := json.MarshalIndent(valMap["remote"], "", "  ")

				fmt.Fprintf(cmd.OutOrStdout(), "Field: %s\n", k)
				fmt.Fprintf(cmd.OutOrStdout(), "[L] Local (Ours):\n%s\n", localJSON)
				fmt.Fprintf(cmd.OutOrStdout(), "[R] Remote (Theirs):\n%s\n", remoteJSON)

				resolved := false
				for !resolved {
					fmt.Fprint(cmd.OutOrStdout(), "Select [L]ocal, [R]emote, or [S]kip: ")
					var choice string
					fmt.Scanln(&choice)
					choice = strings.ToUpper(strings.TrimSpace(choice))

					switch choice {
					case "L":
						obj[k] = valMap["local"]
						resolved = true
					case "R":
						obj[k] = valMap["remote"]
						resolved = true
					case "S":
						fmt.Fprintln(cmd.OutOrStdout(), "Skipping resolution for this field. File remains conflicted.")
						resolvedLines = append(resolvedLines, line) // Keep as is
						return nil                                  // Exit early or continue to save partial? Let's just return.
					default:
						fmt.Fprintln(cmd.OutOrStdout(), "Invalid choice.")
					}
				}
			}

			// Remove full object conflict marker if it was a file-level deletion conflict
			if obj["_conflict"] == true {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDeleted vs Modified conflict for issue ID: %s\n", obj["id"])
				localJSON, _ := json.MarshalIndent(obj["local"], "", "  ")
				remoteJSON, _ := json.MarshalIndent(obj["remote"], "", "  ")
				fmt.Fprintf(cmd.OutOrStdout(), "[L] Local (Ours):\n%s\n", localJSON)
				fmt.Fprintf(cmd.OutOrStdout(), "[R] Remote (Theirs):\n%s\n", remoteJSON)
				resolved := false
				for !resolved {
					fmt.Fprint(cmd.OutOrStdout(), "Select [L]ocal, [R]emote, or [S]kip: ")
					var choice string
					fmt.Scanln(&choice)
					choice = strings.ToUpper(strings.TrimSpace(choice))
					switch choice {
					case "L":
						if obj["local"] != nil {
							b, _ := json.Marshal(obj["local"])
							resolvedLines = append(resolvedLines, string(b))
						}
						resolved = true
					case "R":
						if obj["remote"] != nil {
							b, _ := json.Marshal(obj["remote"])
							resolvedLines = append(resolvedLines, string(b))
						}
						resolved = true
					case "S":
						fmt.Fprintln(cmd.OutOrStdout(), "Skipping...")
						return nil
					}
				}
			} else {
				// Reserialize the resolved object
				b, _ := json.Marshal(obj)
				resolvedLines = append(resolvedLines, string(b))
			}
		}

		if conflictsFound == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ No conflicts found.")
			return nil
		}

		// Write back
		finalContent := strings.Join(resolvedLines, "\n") + "\n"
		if err := os.WriteFile(filePath, []byte(finalContent), 0644); err != nil {
			return fmt.Errorf("failed to save resolved file: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\n✅ All conflicts resolved! You can now run `git add issues.jsonl` and `git commit`.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resolveCmd)
}
