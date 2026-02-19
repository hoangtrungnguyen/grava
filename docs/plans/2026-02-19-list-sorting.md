# list sorting Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement multi-column sorting for the `grava list` command using a `--sort` flag.

**Architecture:** Add a `--sort` flag to the `list` command. Use a parser to translate user input (e.g., `priority:desc`) into SQL `ORDER BY` segments, validated against a whitelist of columns.

**Tech Stack:** Go, Cobra, SQL (Dolt/MySQL).

---

### Task 1: Add Sorting Data Structures and Parser
**Goal:** Implement the logic to parse the `--sort` flag into SQL segments.

**Files:**
- Modify: `pkg/cmd/list.go`

**Step 1: Define the sort column map and parsing function**
Add a map for safe column names and a function to parse the sort string.

```go
var sortColumnMap = map[string]string{
	"id":       "id",
	"title":    "title",
	"type":     "issue_type",
	"status":   "status",
	"priority": "priority",
	"created":  "created_at",
	"updated":  "updated_at",
	"assignee": "assignee",
}

func parseSortFlag(sortStr string) (string, error) {
	if sortStr == "" {
		return "priority ASC, created_at DESC, id ASC", nil
	}

	parts := strings.Split(sortStr, ",")
	var segments []string

	for _, p := range parts {
		subparts := strings.Split(strings.TrimSpace(p), ":")
		field := strings.ToLower(subparts[0])
		col, ok := sortColumnMap[field]
		if !ok {
			return "", fmt.Errorf("invalid sort field %q", field)
		}

		order := "ASC"
		if len(subparts) > 1 {
			o := strings.ToUpper(subparts[1])
			if o != "ASC" && o != "DESC" {
				return "", fmt.Errorf("invalid order %q for field %q", subparts[1], field)
			}
			order = o
		}
		segments = append(segments, fmt.Sprintf("%s %s", col, order))
	}

	// Always add ID for stable sorting
	segments = append(segments, "id ASC")
	return strings.Join(segments, ", "), nil
}
```

**Step 2: Add unit tests for the parser**
Create `pkg/cmd/list_test.go` (if it doesn't exist) or add to it.

**Step 3: Run tests and verify**
Run: `go test ./pkg/cmd -v`
Expected: PASS

**Step 4: Commit**
```bash
git add pkg/cmd/list.go
git commit -m "feat(cli): add sort flag parser logic"
```

---

### Task 2: Integrate Sort Flag into `list` Command
**Goal:** Hook up the `--sort` flag to the Cobra command and update the SQL query.

**Files:**
- Modify: `pkg/cmd/list.go`

**Step 1: Register the `--sort` flag**
Update `init()` to include the flag.

**Step 2: Update `RunE` to use the parser**
Replace the hardcoded `ORDER BY` with the output of `parseSortFlag`.

**Step 3: Run local verification**
Run: `./grava list --sort created:desc`
Expected: Results sorted by creation date (newest first).

**Step 4: Commit**
```bash
git add pkg/cmd/list.go
git commit -m "feat(cli): integrate --sort flag into grava list"
```

---

### Task 3: Update Documentation
**Goal:** Add the new flag to the CLI reference and help text.

**Files:**
- Modify: `docs/CLI_REFERENCE.md`
- Modify: `pkg/cmd/list.go` (Long description)

**Step 1: Update CLI_REFERENCE.md**
Add `--sort` to the `list` command flags and provide examples.

**Step 2: Update help text**
Ensure `grava list --help` shows the new flag and format.

**Step 3: Commit**
```bash
git add docs/CLI_REFERENCE.md pkg/cmd/list.go
git commit -m "docs: document --sort flag for list command"
```
