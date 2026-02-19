# Implementation Plan: History and Undo Commands (grava-de78.11)

This plan outlines the implementation of `grava history` and `grava undo` commands, leveraging Dolt's version control capabilities.

## 1. `grava history <issue_id>`

The `history` command will display the modification history of a specific issue.

### Implementation Details:
- **Command:** `grava history <issue_id>`
- **Database Query:** Query the `dolt_history_issues` table.
- **Columns to Display:** `commit_hash`, `committer`, `date`, `status_change` (derived), `title_change` (derived).
- **Output Format:** Table view of changes.

### Query Strategy:
```sql
SELECT 
    commit_hash, 
    committer, 
    commit_date,
    title,
    status,
    description
FROM dolt_history_issues 
WHERE id = ? 
ORDER BY commit_date DESC;
```
We may need to join with `dolt_log` or similar to get cleaner commit metadata if `dolt_history_issues` lacks it, but `dolt_history_issues` should suffice for state snapshots.

## 2. `grava undo <issue_id>`

The `undo` command will revert the last change to a specific issue.

### Implementation Details:
- **Command:** `grava undo <issue_id>`
- **Logic:**
    1.  Find the *current* state of the issue.
    2.  Find the *previous* state of the issue from `dolt_history_issues`.
    3.  Update the `issues` table with the values from the previous state.
    4.  Commit the revert with a message like "Revert issue <id> to state from <date>".

### Logic Steps:
1.  **Fetch History:** Get the last 2 records from `dolt_history_issues` for the given ID, ordered by `commit_date` DESC.
    *   Record 1: Current state (mostly).
    *   Record 2: Previous state.
2.  **Validation:** If only 1 record exists, there's nothing to undo (it's the creation state).
3.  **Restore:**
    ```sql
    UPDATE issues 
    SET title = ?, description = ?, status = ?, priority = ?, ...
    WHERE id = ?
    ```
    (Using values from Record 2).

## 3. Plan Steps

1.  **Create `pkg/cmd/history.go`**: Implement the history command.
2.  **Create `pkg/cmd/undo.go`**: Implement the undo command.
3.  **Register Commands**: Add to `root.go`.
4.  **Tests**: Add unit/integration tests.
5.  **Documentation**: Update `CLI_REFERENCE.md`.

## 4. Verification

- Create an issue.
- Update it.
- Run `grava history` -> Should show 2 entries.
- Run `grava undo` -> Should revert the update.
- Run `grava history` -> Should show 3 entries (Creation, Update, Revert).
