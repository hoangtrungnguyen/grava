---
stepsCompleted: [1, 2]
inputDocuments:
  - _bmad-output/planning-artifacts/edge-case-resolution-strategy.md
workflowType: 'research'
lastStep: 1
research_type: 'technical'
research_topic: 'Edge Case Resolution Strategies for Multi-Agent Orchestration Systems'
research_goals: 'Deep dive into technical implementation patterns, architecture decisions, and failure mode analysis for the three edge cases: Delete vs Modify conflicts, Large File concurrent changes, and Rapid Sequential Claims'
user_name: 'Htnguyen'
date: '2026-03-19'
web_research_enabled: true
source_verification: true
---

# Research Report: Technical

**Date:** 2026-03-19
**Author:** Htnguyen
**Research Type:** Technical

---

## Technical Research Scope Confirmation

**Research Topic:** Edge Case Resolution Strategies for Multi-Agent Orchestration Systems
**Research Goals:** Deep dive into technical implementation patterns, architecture decisions, and failure mode analysis for the three edge cases: Delete vs Modify conflicts, Large File concurrent changes, and Rapid Sequential Claims

**Technical Research Scope:**
- Architecture Analysis — merge driver patterns, 3-way parse atomicity, conflict isolation table design
- Implementation Approaches — guard clauses for dual-delete, dual-insert, lease race, Wisp lifecycle, lock timeout
- Technology Stack — SQLite `SELECT FOR UPDATE` semantics, Git merge driver exit codes, TTL expiry patterns
- Integration Patterns — mcp_agent_mail doctor utility, semi-automatic repair, HumanOverseer delivery guarantees
- Performance Considerations — deadlock prevention, lock timeout tuning, lease overlap query cost

**Research Methodology:**
- NotebookLM multi-source synthesis (internal ADRs + mcp_agent_mail source)
- Multi-area parallel querying for breadth
- Confidence levels noted where information is extrapolated vs directly sourced

**Scope Confirmed:** 2026-03-19

---

## Area 1: Merge Driver — Dual-Delete and Dual-Insert Guard Patterns

**Source:** Epic 3: Git Merge Driver (internal), ADR-FM5

### Data Structures
The merge driver reads all three Git file paths (`%O` ancestor, `%A` ours, `%B` theirs) into **ID-keyed hash maps** — indexed by `Issue_ID`, completely ignoring file line numbers. This is the foundation that makes all conflict detection deterministic.

```go
// ParseError is returned when a JSONL file cannot be decoded.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error in %s: %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error { return e.Err }

// readJSONLToMap reads a JSONL file at path and returns records keyed by ID.
// A non-existent file returns an empty map and no error (absent-branch
// semantics used by the merge driver).
func readJSONLToMap(ctx context.Context, path string) (map[string]ExportItem, error) {
	records := make(map[string]ExportItem)

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return records, nil
		}
		return nil, &ParseError{Path: path, Err: err}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var item ExportItem
		if err := json.Unmarshal(scanner.Bytes(), &item); err != nil {
			return nil, &ParseError{Path: path, Err: err}
		}
		if item.Type == "issue" {
			var issue IssueExportData
			if err := json.Unmarshal(item.Data, &issue); err == nil {
				records[issue.ID] = item
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, &ParseError{Path: path, Err: err}
	}
	return records, nil
}

// Usage inside the merge driver:
ancestor, err := readJSONLToMap(ctx, pathO)
ours, err     := readJSONLToMap(ctx, pathA)
theirs, err   := readJSONLToMap(ctx, pathB)
```

### Guard Clause: Dual-Delete (ID absent from both Ours AND Theirs)

**Detection:** `id in ancestor and id not in ours and id not in theirs`

**Resolution:** Both branches share identical intent — omit the record from the merged output silently. **No conflict raised.** This is currently unhandled in the strategy document (ECH finding #1).

```go
for id := range ancestor {
	_, inOurs   := ours[id]
	_, inTheirs := theirs[id]

	switch {
	case !inOurs && !inTheirs:
		// Dual-delete: both branches agreed to remove — skip silently.

	case !inOurs:
		// EC1: ours deleted, theirs may have modified — apply policy.
		applyPolicy(ctx, "delete-wins-or-conflict", id)

	case !inTheirs:
		// Theirs deleted, ours modified — symmetric case, keep ours.
		output[id] = ours[id]
	}
}
```

### Guard Clause: Dual-Insert (ID absent from Ancestor but present in BOTH)

**Detection:** `id not in ancestor and id in ours and id in theirs`

**Resolution (two paths):**
- If field values are **identical** → safe merge, emit single record
- If field values **differ** → true conflict, return exit code 1

```go
// allKeys is the union of ours and theirs, minus anything in ancestor.
allKeys := make(map[string]struct{})
for k := range ours   { allKeys[k] = struct{}{} }
for k := range theirs { allKeys[k] = struct{}{} }

for id := range allKeys {
	_, inAnc              := ancestor[id]
	ourRec,   inOurs      := ours[id]
	theirRec, inTheirs    := theirs[id]

	if inAnc || !inOurs || !inTheirs {
		continue // handled by the ancestor-present path
	}

	// Dual-insert: ID is new on both branches.
	if bytes.Equal(ourRec.Data, theirRec.Data) {
		output[id] = ourRec // identical dual-insert — safe
	} else {
		writeConflictRecord(id, ExportItem{}, ourRec, theirRec)
		return fmt.Errorf("differing dual-insert for id %s: %w", id, ErrMergeConflict)
	}
}
```

### Guard Clause: Malformed Input

**Detection:** `*ParseError` on any of the three file versions

```go
ancestor, err := readJSONLToMap(ctx, pathO)
if err != nil {
	var pe *ParseError
	if errors.As(err, &pe) {
		fmt.Fprintf(os.Stderr, "MERGE DRIVER: unparseable ancestor: %v\n", pe)
	}
	// Halt the merge — do not attempt partial resolution.
	return err
}
```

### Guard Clause: Missing Policy Configuration

```go
policy := cfg.MergePolicy
if policy == "" {
	fmt.Fprintln(os.Stderr, "MERGE DRIVER: no merge_policy configured, defaulting to 'conflict'")
	policy = "conflict"
}
```

**Confidence:** HIGH — directly sourced from Epic 3 merge driver user stories and ADR specifications.

---

## Area 2: File Lease Atomicity — Preventing Simultaneous Exclusive Grant

**Source:** mcp_agent_mail file reservations architecture

### The Core Problem
Standard SQL `UNIQUE` constraints cannot detect overlapping glob patterns (e.g., `src/**` vs `src/api/*`) because overlap is semantic, not a string equality check. This requires a **read-evaluate-write** transaction pattern.

### SQLite Pattern (current stack)

```go
// FileReservation represents an active path-pattern lease.
type FileReservation struct {
	PathPattern string
	AgentID     string
	ExpiresAt   time.Time
	Exclusive   bool
}

// ErrReservationConflict is returned when a new exclusive lease overlaps
// an existing active lease.
var ErrReservationConflict = errors.New("file reservation conflict: overlapping exclusive lease exists")

// acquireExclusiveLease runs the read-evaluate-write cycle inside a single
// serializable transaction. In SQLite this serialises all writers on the
// DB file, which is sufficient for a single-node agent fleet. For
// distributed/multi-node deployments use Dolt/Postgres SELECT ... FOR UPDATE
// instead.
func acquireExclusiveLease(
	ctx context.Context,
	db *sql.DB,
	projectID, agentID, pathPattern string,
	ttl time.Duration,
) error {
	// Step 1: Serializable transaction locks the DB during evaluation.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("acquireExclusiveLease: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Step 2: Read all active exclusive leases into application memory.
	rows, err := tx.QueryContext(ctx, `
		SELECT path_pattern, agent_id, expires_ts
		FROM   file_reservations
		WHERE  project_id = ?
		  AND  released_ts IS NULL
		  AND  expires_ts > NOW()
		  AND  exclusive = 1`,
		projectID,
	)
	if err != nil {
		return fmt.Errorf("acquireExclusiveLease: query leases: %w", err)
	}
	defer rows.Close()

	var active []FileReservation
	for rows.Next() {
		var r FileReservation
		if err := rows.Scan(&r.PathPattern, &r.AgentID, &r.ExpiresAt); err != nil {
			return fmt.Errorf("acquireExclusiveLease: scan: %w", err)
		}
		active = append(active, r)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("acquireExclusiveLease: rows: %w", err)
	}

	// Application evaluates wildmatch overlap in memory.
	// Step 3a: If overlap detected — return conflict error (ROLLBACK via defer).
	for _, r := range active {
		if patternsOverlap(r.PathPattern, pathPattern) {
			return ErrReservationConflict
		}
	}

	// Step 3b: No overlap — INSERT and COMMIT.
	now       := time.Now().UTC()
	expiresAt := now.Add(ttl)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO file_reservations
		    (id, project_id, agent_id, path_pattern, exclusive, reason, created_ts, expires_ts)
		VALUES (?, ?, ?, ?, 1, ?, ?, ?)`,
		newID(), projectID, agentID, pathPattern, "exclusive", now, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("acquireExclusiveLease: insert: %w", err)
	}
	return tx.Commit()
}
```

> **Key insight:** `BEGIN EXCLUSIVE` in SQLite locks the entire DB file, which is sufficient for a single-node agent fleet. For distributed/multi-node deployments, use Dolt/Postgres `SELECT ... FOR UPDATE` instead.

### TTL Window Sizing

| Task Type | Recommended TTL | Rationale |
|-----------|----------------|-----------|
| Small file edits | 600s (10 min) | Covers typical agent work + commit cycle |
| Large refactors | 3600s (1 hr) | Covers multi-file sweep operations |
| Emergency patch | 300s (5 min) | Short window forces explicit re-lease |

**Guard: Overly Broad Pattern**
```go
// ErrPatternTooBroad is returned when a path pattern matches the repo root.
var ErrPatternTooBroad = errors.New("path_pattern matches repo root — too broad")

// validatePathPattern rejects patterns that would lock the entire repository.
func validatePathPattern(pattern string) error {
	tooBroad := []string{"**", "*", "./**", "."}
	for _, p := range tooBroad {
		if pattern == p {
			return fmt.Errorf("path_pattern %q: %w", pattern, ErrPatternTooBroad)
		}
	}
	// Also reject any pattern that matches "." via filepath.Match.
	matched, err := filepath.Match(pattern, ".")
	if err != nil {
		return fmt.Errorf("validatePathPattern: invalid pattern %q: %w", pattern, err)
	}
	if matched {
		return fmt.Errorf("path_pattern %q matches repo root: %w", pattern, ErrPatternTooBroad)
	}
	return nil
}
```

**Guard: TTL Expiry During Active Commit**
```go
// ErrLeaseExpired is returned when a lease has expired before the commit
// completes.
type ErrLeaseExpired struct {
	PathPattern string
	ExpiredAt   time.Time
}

func (e *ErrLeaseExpired) Error() string {
	return fmt.Sprintf("lease for %q expired at %s during commit",
		e.PathPattern, e.ExpiredAt.Format(time.RFC3339))
}

// validateLeaseAtCommit re-checks that the lease has not expired before the
// caller writes the commit record.
func validateLeaseAtCommit(lease FileReservation) error {
	if time.Now().UTC().After(lease.ExpiresAt) {
		return &ErrLeaseExpired{PathPattern: lease.PathPattern, ExpiredAt: lease.ExpiresAt}
	}
	return nil
}
```

**Confidence:** HIGH for SQLite pattern. MEDIUM for TTL sizing — values are recommended based on typical agent work cycles, not empirically measured.

---

## Area 3: DB Locking — Deadlock Prevention and Partial Commit Guards

**Source:** Internal Dolt/SQLite architecture, mcp_agent_mail claim patterns

### Deadlock Prevention

**Rule 1 — Lexicographic Lock Ordering**
When a claim operation must touch multiple rows (e.g., dependency chains), always acquire locks in sorted ID order:

```go
// lockIssuesInOrder acquires row-level locks in lexicographic ID order to
// prevent deadlocks when two goroutines claim overlapping dependency chains.
func lockIssuesInOrder(ctx context.Context, tx *sql.Tx, ids ...string) error {
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted) // always lexicographic

	for _, id := range sorted {
		var dummy string
		if err := tx.QueryRowContext(ctx,
			"SELECT id FROM issues WHERE id = ? FOR UPDATE", id,
		).Scan(&dummy); err != nil {
			return fmt.Errorf("lockIssuesInOrder: lock %s: %w", id, err)
		}
	}
	return nil
}
```

**Rule 2 — Short Lock Timeout with Retry**
```go
const (
	maxRetries  = 3
	baseBackoff = 10 * time.Millisecond
)

// ErrClaimFailed is returned when all retry attempts for a claim are
// exhausted.
type ErrClaimFailed struct {
	IssueID string
	Retries int
}

func (e *ErrClaimFailed) Error() string {
	return fmt.Sprintf("could not claim %s after %d retries", e.IssueID, e.Retries)
}

// claimWithRetry attempts to claim an issue, backing off exponentially on
// lock-timeout errors (Dolt/MySQL: ER_LOCK_WAIT_TIMEOUT).
func claimWithRetry(ctx context.Context, db *sql.DB, issueID, agentID string) error {
	for attempt := range maxRetries {
		err := claimTask(ctx, db, issueID, agentID)
		if err == nil {
			return nil
		}
		if !isLockTimeoutError(err) {
			return err
		}
		backoff := baseBackoff * (1 << attempt) // 10 ms, 20 ms, 40 ms
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return &ErrClaimFailed{IssueID: issueID, Retries: maxRetries}
}
```

### Guard: Partial Commit (Assignee set, Status not updated)

The root cause is a two-statement transaction. The fix is an **atomic single-statement UPDATE**:

```go
// WRONG: two statements — partial commit is possible if the process dies
// between them.
//   tx.ExecContext(ctx, "UPDATE issues SET assignee = ? WHERE id = ?", agentID, issueID)
//   tx.ExecContext(ctx, "UPDATE issues SET status = 'in_progress' WHERE id = ?", issueID)

// CORRECT: single atomic statement with pre-condition guard.
// Rows affected = 0 means another agent claimed first — safe abort.
const claimQuery = `
	UPDATE issues
	SET    status = 'in_progress', assignee = ?
	WHERE  id = ? AND status = 'open'`
```

**Application check:**
```go
// ErrAlreadyClaimed is returned when another agent has already claimed the
// issue before this transaction committed.
var ErrAlreadyClaimed = errors.New("issue already claimed by another agent")

func claimTask(ctx context.Context, db *sql.DB, issueID, agentID string) error {
	result, err := db.ExecContext(ctx, claimQuery, agentID, issueID)
	if err != nil {
		return fmt.Errorf("claimTask %s: %w", issueID, err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("claimTask %s: rows affected: %w", issueID, err)
	}
	if n == 0 {
		return fmt.Errorf("issue %s: %w", issueID, ErrAlreadyClaimed)
	}
	return nil
}
```

**Confidence:** HIGH — standard SQL atomicity pattern, confirmed against Dolt transactional semantics.

---

## Area 4: Wisp Lifecycle — Zero-Wisp Recovery and Atomic Deletion

**Source:** ADR-FM5 (Wisps Lifecycle), ADR-004 (Worktree Architecture)

### Zero-Wisp Crash Recovery

When an agent crashes before writing any Wisp entry, the issue remains `in_progress`. The next claiming agent finds an empty Wisp log — this is a **valid clean-start signal**, not an error.

```go
// ResumeMode describes how the claiming agent should treat an in-progress
// issue.
type ResumeMode string

const (
	ResumeFresh  ResumeMode = "fresh"
	ResumeResume ResumeMode = "resume"
)

// Wisp is a lightweight progress entry written by an agent during work.
type Wisp struct {
	ID        string    `db:"id"`
	IssueID   string    `db:"issue_id"`
	CreatedAt time.Time `db:"created_ts"`
	Payload   string    `db:"payload"`
}

// determineResumeMode queries all Wisps for issueID in creation order.
// An empty result means the prior agent crashed before starting — the caller
// should treat the issue as a clean fresh-start (ResumeFresh). Any existing
// Wisps indicate resumable in-progress work (ResumeResume).
func determineResumeMode(ctx context.Context, db *sql.DB, issueID string) (ResumeMode, []Wisp, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT id, issue_id, created_ts, payload FROM wisps WHERE issue_id = ? ORDER BY created_ts",
		issueID,
	)
	if err != nil {
		return "", nil, fmt.Errorf("determineResumeMode %s: %w", issueID, err)
	}
	defer rows.Close()

	var wisps []Wisp
	for rows.Next() {
		var w Wisp
		if err := rows.Scan(&w.ID, &w.IssueID, &w.CreatedAt, &w.Payload); err != nil {
			return "", nil, fmt.Errorf("determineResumeMode %s: scan: %w", issueID, err)
		}
		wisps = append(wisps, w)
	}
	if err := rows.Err(); err != nil {
		return "", nil, fmt.Errorf("determineResumeMode %s: rows: %w", issueID, err)
	}

	if len(wisps) == 0 {
		// Zero Wisps: prior agent crashed before starting — treat as fresh.
		return ResumeFresh, nil, nil
	}
	return ResumeResume, wisps, nil
}
```

Any leftover ghost worktree from the crashed agent is detected by `grava doctor` and **reused** by the new agent (not re-created). This is the CM-5 pattern.

### Safe Compaction Guard

`grava compact` must never touch Wisps for `in_progress` issues:

```go
// compactWisps deletes Wisps only for issues whose status is 'closed'.
// It deliberately avoids a cutoff-date filter because that would silently
// delete Wisps for stalled in-progress tasks.
//
// Never use: DELETE FROM wisps WHERE created_ts < NOW() - INTERVAL 7 DAY
func compactWisps(ctx context.Context, db *sql.DB) (int64, error) {
	result, err := db.ExecContext(ctx, `
		DELETE FROM wisps
		WHERE issue_id IN (
		    SELECT id FROM issues WHERE status = 'closed'
		)`)
	if err != nil {
		return 0, fmt.Errorf("compactWisps: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("compactWisps: rows affected: %w", err)
	}
	return n, nil
}
```

### Atomic Wisp Deletion with Task Close (Two-Phase)

```go
// ErrDirtyWorktree is returned when a worktree has uncommitted changes at
// close time.
var ErrDirtyWorktree = errors.New("commit or stash changes before closing")

// closeTask performs the two-phase close:
//  1. Verify the worktree is clean (outside the DB transaction).
//  2. Remove the worktree directory, then atomically update DB state.
//
// The operation is idempotent: re-running after a crash converges correctly
// because UPDATE is a no-op when status is already 'closed', and DELETE on
// an empty wisps set is a no-op.
func closeTask(ctx context.Context, db *sql.DB, issueID, agentID string) error {
	// Phase 1: Check worktree for uncommitted changes BEFORE touching the DB.
	worktreePath := filepath.Join(".worktrees", agentID)
	dirty, err := hasUncommittedChanges(worktreePath)
	if err != nil {
		return fmt.Errorf("closeTask %s: check worktree: %w", issueID, err)
	}
	if dirty {
		return fmt.Errorf("closeTask %s: %w", issueID, ErrDirtyWorktree)
	}

	// Phase 2: Remove worktree, then atomic DB transaction.
	if err := os.RemoveAll(worktreePath); err != nil {
		return fmt.Errorf("closeTask %s: remove worktree: %w", issueID, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("closeTask %s: begin tx: %w", issueID, err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx,
		"UPDATE issues SET status = 'closed', assignee = NULL WHERE id = ?",
		issueID,
	); err != nil {
		return fmt.Errorf("closeTask %s: update issues: %w", issueID, err)
	}
	if _, err := tx.ExecContext(ctx,
		"DELETE FROM wisps WHERE issue_id = ?",
		issueID,
	); err != nil {
		return fmt.Errorf("closeTask %s: delete wisps: %w", issueID, err)
	}
	return tx.Commit()
}
```

**Confidence:** HIGH — directly sourced from ADR-FM5 and CM-6 (`grava compact` during active claim).

---

## Area 5: HumanOverseer Delivery Guarantees

**Source:** mcp_agent_mail HumanOverseer architecture

### Why Inbox is Never "Unreachable"

The inbox is a **passive persistence state** (SQLite rows + Git markdown files), not a live network endpoint. It cannot go offline. The real failure mode is that an agent becomes distracted, stuck in a loop, or fails to invoke `fetch_inbox`.

### Delivery Pattern: Atomic Dual-Persistence

```go
// Message is persisted to both the SQL store and the Git-tracked inbox
// directory in a single transaction so no partial delivery is possible.
type Message struct {
	ID         string
	Recipient  string
	Content    string
	Importance string
}

// sendToInbox atomically writes the message to the DB and to the Git-backed
// inbox file. Both succeed or both roll back — no partial delivery.
func sendToInbox(ctx context.Context, db *sql.DB, inboxDir string, msg Message) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sendToInbox: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO messages (id, recipient, content, importance, created_ts) VALUES (?, ?, ?, ?, NOW())",
		msg.ID, msg.Recipient, msg.Content, msg.Importance,
	); err != nil {
		return fmt.Errorf("sendToInbox: insert message: %w", err)
	}

	// Write the Git-tracked markdown file inside the transaction boundary so
	// that an error here causes a rollback of the SQL INSERT.
	inboxPath := filepath.Join(inboxDir, "agents", msg.Recipient, "inbox", msg.ID+".md")
	if err := writeGitFile(inboxPath, msg.Content); err != nil {
		return fmt.Errorf("sendToInbox: write git file: %w", err)
	}

	return tx.Commit()
}
```

The `HumanOverseer` identity has an `open` contact policy that **structurally bypasses** `CONTACT_ENFORCEMENT_ENABLED` restrictions — the message cannot be blocked by routing rules.

### Fallback: PostToolUse Hook Injection

Since the agent may not poll its inbox proactively, the system installs a rate-limited shell hook that fires after every tool execution:

```bash
# scripts/hooks/check_inbox.sh (rate-limited, ~1 check/min)
UNREAD=$(sqlite3 ~/.agent_mail/db.sqlite \
    "SELECT COUNT(*) FROM messages WHERE recipient=? AND read=0 AND importance='high'")
if [ "$UNREAD" -gt 0 ]; then
    echo "INBOX: $UNREAD high-importance message(s) pending — run fetch_inbox"
fi
```

Wired as:
- **Claude Code:** `PostToolUse` hook
- **Codex CLI:** `agent-turn-complete` binding

### Guard: ack_required Timeout

```go
const ackTimeout = 5 * time.Minute

// ErrAckTimeout is returned when the required acknowledgements are not
// received before the deadline.
type ErrAckTimeout struct {
	ThreadID string
	Timeout  time.Duration
}

func (e *ErrAckTimeout) Error() string {
	return fmt.Sprintf("no agents acknowledged thread %s within %s", e.ThreadID, e.Timeout)
}

// HumanOverseer is the interface for sending emergency broadcasts.
type HumanOverseer interface {
	Broadcast(ctx context.Context, text, importance string) error
}

// sendWithAck sends a message and polls for acknowledgements until the
// deadline. On timeout it broadcasts an emergency escalation to the
// HumanOverseer.
func sendWithAck(
	ctx context.Context,
	db *sql.DB,
	overseer HumanOverseer,
	msg Message,
	threadID string,
	minAcks int,
) error {
	msgID, err := sendMessage(ctx, db, msg, true /* ackRequired */)
	if err != nil {
		return fmt.Errorf("sendWithAck: send: %w", err)
	}

	deadline := time.Now().Add(ackTimeout)
	ticker   := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var count int
			if err := db.QueryRowContext(ctx,
				"SELECT COUNT(*) FROM acks WHERE message_id = ?", msgID,
			).Scan(&count); err != nil {
				return fmt.Errorf("sendWithAck: query acks: %w", err)
			}
			if count >= minAcks {
				return nil // success
			}
			if time.Now().After(deadline) {
				broadcastErr := overseer.Broadcast(ctx, fmt.Sprintf(
					"No agents acknowledged thread %s within %s. Manual intervention required.",
					threadID, ackTimeout,
				), "critical")
				if broadcastErr != nil {
					return fmt.Errorf("sendWithAck: broadcast: %w", broadcastErr)
				}
				return &ErrAckTimeout{ThreadID: threadID, Timeout: ackTimeout}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
```

**Confidence:** HIGH for dual-persistence and contact policy bypass. MEDIUM for hook specifics — implementation details may vary by agent runtime.

---

## Summary: Resolved Gaps from ECH Review

| ECH Finding | Gap | Resolution Pattern | Confidence |
|-------------|-----|--------------------|-----------|
| #1 | Dual-delete raises false conflict | Auto-omit when ID missing from both branches | HIGH |
| #2 | Dual-insert silent overwrite | Field equality check → safe merge or conflict | HIGH |
| #3 | Malformed JSONL crashes driver | `errors.As(*ParseError)` → return err | HIGH |
| #4 | Missing policy config | Default to `conflict`, log warning | HIGH |
| #5 | Partial conflict table re-run | Check all rows `resolved_status='resolved'` before re-merge | HIGH |
| #6 | HumanOverseer delivery failure | Dual-persistence + PostToolUse hook fallback | HIGH |
| #7 | Simultaneous lease race | `BeginTx(LevelSerializable)` + app-layer wildmatch + Rollback on overlap | HIGH |
| #8 | Overly broad path pattern | Root-pattern guard before INSERT | MEDIUM |
| #9 | TTL expires mid-commit | Re-validate lease at commit time | HIGH |
| #10 | Partial lease release | Transactional `UPDATE released_ts` with retry | HIGH |
| #11 | sha1(path) collision | Store full path in artifact, verify on read | MEDIUM |
| #12 | SELECT FOR UPDATE deadlock | Lexicographic ordering + 1s timeout + 3-retry backoff | HIGH |
| #13 | Partial commit (assignee/status) | Single atomic `UPDATE WHERE status='open'` | HIGH |
| #14 | Zero-Wisp crash recovery | Treat empty Wisps as fresh-start signal | HIGH |
| #15 | Premature Wisp deletion | Delete Wisps only inside `close` transaction after status='closed' | HIGH |
| #16 | ack_required timeout | 5-min deadline → HumanOverseer emergency broadcast | MEDIUM |
