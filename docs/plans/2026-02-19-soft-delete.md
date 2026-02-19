# Implementation Plan - Soft Delete (grava-de78.15)

Implement soft delete functionality for the Grava issue tracker. Instead of permanently removing issues from the database, they will be marked with a `tombstone` status. This preserves history and audit trails while hiding the issues from standard queries.

## User Review Required

> [!IMPORTANT]
> - `grava drop` will REMAINS a hard delete (nuclear reset).
> - Soft deleted issues will be hidden from `list`, `search`, and `quick` commands.
> - `grava show` will still be able to display a soft-deleted issue if the ID is known, but it will be visually marked as "deleted".

## Proposed Changes

### 1. Validation Layer
- **File:** `pkg/validation/validation.go`
- **Action:** Add `"tombstone"` to `AllowedStatuses`.

### 2. Command Refactoring (Delete -> Soft Delete)
- **File:** `pkg/cmd/compact.go`
- **Action:**
    - Replace `DELETE FROM issues WHERE id = ?` with an `UPDATE issues SET status = 'tombstone', updated_at = NOW(), updated_by = ?, agent_model = ? WHERE id = ?`.
    - Continue recording the deletion in the `deletions` table as before.
- **File:** `pkg/cmd/clear.go`
- **Action:**
    - Replace `DELETE FROM issues WHERE id = ?` with the same `UPDATE` logic.
    - Continue recording in `deletions` table.

### 3. Query Visibility
- **File:** `pkg/cmd/list.go`
- **Action:** Update SQL query to include `AND status != 'tombstone'`.
- **File:** `pkg/cmd/search.go`
- **Action:** Update SQL query to include `AND status != 'tombstone'`.
- **File:** `pkg/cmd/quick.go`
- **Action:** Update SQL query to include `AND status != 'tombstone'`.

### 4. Detail View
- **File:** `pkg/cmd/show.go`
- **Action:** Update the output to indicate if an issue is a tombstone (e.g., prefix status with 🗑️ or "DELETED").

## Verification Plan

### Automated Tests
- Update `pkg/cmd/clear_test.go` and other relevant tests to verify that `DELETE` is no longer called and `UPDATE status = 'tombstone'` is called instead.
- Add a new test case to `pkg/cmd/list_test.go` ensuring tombstones are hidden.

### Manual Verification
1. Create a few test issues.
2. Run `grava clear` on a range containing them.
3. Verify `grava list` does not show them.
4. Verify `grava show <id>` shows them as "DELETED".
5. Verify DB directly: `SELECT status FROM issues WHERE id = <id>` returns `tombstone`.
