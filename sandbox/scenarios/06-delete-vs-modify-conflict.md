# Scenario 6: Delete vs. Modify Conflict — Schema-Aware Merge Driver

**Status**: Edge case validation
**Source**: [edge-case-resolution-strategy.md](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md) — Edge Case 1
**Priority**: HIGH (common concurrent editing pattern)

---

## Overview

Agent A deletes a file/record on one branch while Agent B modifies it on a parallel branch. Schema-aware merge driver (`grava merge-slot`) detects the conflict and surfaces it (not silent corruption).

## Setup

### Prerequisites
- `.jsonl` or structured file format being used
- `grava merge-slot` command (Git merge driver) registered
- Test data with issue records
- Conflict isolation table in DB

### Steps

1. **Create JSONL data file** (structured records):
   ```bash
   # File: data/issues.jsonl
   echo '{"Issue_ID": "issue-1", "Title": "Fix bug", "Status": "open"}' >> data/issues.jsonl
   echo '{"Issue_ID": "issue-2", "Title": "Add feature", "Status": "in_progress"}' >> data/issues.jsonl
   echo '{"Issue_ID": "issue-3", "Title": "Docs update", "Status": "closed"}' >> data/issues.jsonl
   ```

2. **Agent F starts work** (delete task):
   ```bash
   grava claim task-f
   # Branch: grava/agent-5/task-f
   ```

3. **Agent G starts work** (modify task):
   ```bash
   grava claim task-g
   # Branch: grava/agent-6/task-g
   ```

4. **Agent F deletes issue-2**:
   - Removes line: `{"Issue_ID": "issue-2", ...}`
   - Commits: "chore: remove stale issue-2"

5. **Agent G modifies issue-2** (on different branch):
   - Changes line: `{"Issue_ID": "issue-2", "Status": "completed"}`
   - Commits: "fix: mark issue-2 as completed"

6. **Attempt merge**:
   ```bash
   git merge grava/agent-6/task-g
   # Merge driver invoked for .jsonl files
   ```

---

## Expected Behavior

✅ Git merge invokes `grava merge-slot` for `.jsonl` files
✅ Driver loads Ancestor, Ours, Theirs into memory
✅ Detects conflict:
   - Ancestor: `issue-2` present
   - Ours (Agent F): `issue-2` missing (deleted)
   - Theirs (Agent G): `issue-2` present with change
✅ Conflict policy evaluated:
   - Default: `conflict` (do not auto-resolve)
   - Merge driver returns exit code 1 (halt merge)
✅ Conflict stored in `conflict_records` table
✅ HumanOverseer alert fired to both agents
✅ Conflict is NOT silently resolved (no data loss)
✅ User manually decides: delete or keep + apply change?

---

## Validation

**Success Criteria**:
1. ✅ Merge driver is invoked:
   ```bash
   git merge grava/agent-6/task-g 2>&1 | grep -i "merge-slot\|merge driver"
   ```

2. ✅ Conflict is detected (not auto-resolved):
   ```bash
   git status --porcelain | grep "data/issues.jsonl"
   # Output should show conflict marker (UU or similar)
   ```

3. ✅ Conflict markers show both versions:
   ```bash
   grep -A2 "<<<<<" data/issues.jsonl
   # Shows both Agent F's delete and Agent G's modification
   ```

4. ✅ Conflict record is stored:
   ```bash
   dolt sql "SELECT COUNT(*) FROM conflict_records WHERE resolved_status='pending'"
   # Should be > 0
   ```

5. ✅ HumanOverseer alert sent:
   ```bash
   # Check agent inbox or message log
   dolt sql "SELECT * FROM agent_messages WHERE subject LIKE '%conflict%'" | wc -l
   # Should be > 0
   ```

6. ✅ Merge is halted (exit code 1):
   ```bash
   git merge grava/agent-6/task-g
   MERGE_EXIT=$?
   [ $MERGE_EXIT -ne 0 ]  # Merge failed
   ```

**Test Assertions**:
```bash
# Setup conflict state
BEFORE_MERGE=$(git status --porcelain | wc -l)

# Attempt merge
git merge grava/agent-6/task-g

# Merge halted
MERGE_EXIT=$?
[ $MERGE_EXIT -ne 0 ]

# Conflict markers exist
grep -q "<<<<<" data/issues.jsonl

# Conflict record exists
dolt sql -r csv "SELECT COUNT(*) FROM conflict_records WHERE resolved_status='pending'" | grep -q "[1-9]"

# File is unmerged
git diff --name-only --diff-filter=U | grep -q "data/issues.jsonl"
```

---

## Cleanup

**Option 1: Keep deletion** (discard Agent G's change):
```bash
git checkout --ours data/issues.jsonl
git add data/issues.jsonl
git commit -m "resolve: keep issue-2 deletion"

# Mark conflict resolved
dolt sql "UPDATE conflict_records SET resolved_status='resolved_by_delete' WHERE id='...'"
```

**Option 2: Keep modification** (undo deletion):
```bash
git checkout --theirs data/issues.jsonl
git add data/issues.jsonl
git commit -m "resolve: keep issue-2 modification"

# Mark conflict resolved
dolt sql "UPDATE conflict_records SET resolved_status='resolved_by_modify' WHERE id='...'"
```

**Option 3: Manual merge** (both coexist):
```bash
# Edit file manually to resolve conflict
git add data/issues.jsonl
git commit -m "resolve: manual merge of issue-2 states"

dolt sql "UPDATE conflict_records SET resolved_status='resolved_manual' WHERE id='...'"
```

---

## Data Structures

From [edge-case-resolution-strategy.md](../../_bmad-output/planning-artifacts/edge-case-resolution-strategy.md):

```sql
conflict_records (
  id             TEXT PRIMARY KEY,
  base_version   JSONB,
  our_version    JSONB,
  their_version  JSONB,
  resolved_status TEXT DEFAULT 'pending'
)
```

---

## Critical Insight

**Schema-aware merge driver prevents silent corruption**:
- Detects semantic conflicts (delete vs. modify)
- Not just textual conflict markers
- Structured data integrity preserved
- User has full context for decision

---

## Notes

- **Duration**: ~3-5 seconds
- **Merge driver registration**: Git `.gitattributes`:
  ```
  *.jsonl merge=grava-merge-slot
  ```
- **Policy options**: `delete-wins` or `conflict`
- **Default**: `conflict` (fail safe)
- **Recovery**: Conflict isolation table for audit trail
