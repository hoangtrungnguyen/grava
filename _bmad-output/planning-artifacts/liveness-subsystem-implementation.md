# Liveness Subsystem — Implementation Design
**Project:** grava | **Date:** 2026-03-19 | **Status:** Ready for Implementation

> Closes 29 unhandled findings across ECH + Adversarial reviews of
> `edge-case-resolution-strategy.md` and `failure-recovery-strategy.md`

---

## 1. Problem Statement

The multi-agent orchestrator has no reliable way to distinguish between:
- A **slow agent** (still working) vs a **dead agent** (crashed)
- A **recoverable failure** vs a **poison task** (will crash every agent)
- A **safe doctor run** vs a **concurrent doctor run** (corrupt repair)

These three gaps produce 60%+ of all reviewed findings. One coherent subsystem closes them all.

---

## 2. Solution Overview

**One unified Liveness Subsystem** consisting of:

| Component | Type | Responsibility |
|-----------|------|---------------|
| `liveness` table | DB schema | Heartbeat rows + doctor advisory lock |
| Issues table extension | DB schema | `claim_count` + extended `state` enum |
| `AgentLiveness` service | Go service | Heartbeat writer + stale detector + circuit breaker |
| `DoctorLock` | Go + DB | Advisory mutex preventing concurrent repair runs |
| `WispValidator` | Go | Validates Wisp entries on read, handles corruption |

---

## 3. Database Schema

### 3.1 Extended Issues Table

```sql
-- Add to existing issues table migration
ALTER TABLE issues ADD COLUMN claim_count  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE issues ADD COLUMN state        TEXT    NOT NULL DEFAULT 'open';

-- Valid state transitions:
-- open → in_progress → completing → closed
-- in_progress → stale (heartbeat expired)
-- stale → open (re-queued) | abandoned (threshold exceeded)
-- open → poisoned (claim_count > max_claim_attempts)
-- poisoned → open (human override only)

-- Enforce via CHECK constraint
ALTER TABLE issues ADD CONSTRAINT chk_state
  CHECK (state IN ('open','in_progress','completing','stale','abandoned','poisoned','closed'));
```

### 3.2 Liveness Table

```sql
CREATE TABLE IF NOT EXISTS liveness (
  agent_id        TEXT      PRIMARY KEY,
  issue_id        TEXT      REFERENCES issues(id),
  last_seen       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  state           TEXT      NOT NULL DEFAULT 'active',
  -- state: active | stale | dead
  started_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  heartbeat_ttl   INTEGER   NOT NULL DEFAULT 60  -- seconds
);

-- Advisory lock row for grava doctor
-- Only one row; acquired via UPDATE with compare-and-swap
CREATE TABLE IF NOT EXISTS doctor_lock (
  id              INTEGER   PRIMARY KEY DEFAULT 1,
  locked_by       TEXT,                          -- process ID or hostname
  locked_at       TIMESTAMP,
  expires_at      TIMESTAMP
);

INSERT OR IGNORE INTO doctor_lock (id, locked_by, locked_at, expires_at)
VALUES (1, NULL, NULL, NULL);
```

### 3.3 Wisps Table Extension

```sql
-- Add sequence number for ordering and corruption detection
ALTER TABLE wisps ADD COLUMN seq         INTEGER NOT NULL DEFAULT 0;
ALTER TABLE wisps ADD COLUMN checksum    TEXT;   -- sha256 of log_entry
ALTER TABLE wisps ADD COLUMN valid       BOOLEAN NOT NULL DEFAULT true;
```

---

## 4. Go Package Structure

```
pkg/
└── liveness/
    ├── liveness.go          # AgentLiveness service (heartbeat + detector)
    ├── circuit_breaker.go   # CircuitBreaker (claim_count + poisoned state)
    ├── doctor_lock.go       # DoctorLock advisory mutex
    ├── wisp_validator.go    # WispValidator (read-time corruption guard)
    ├── states.go            # IssueState enum + transition table
    └── liveness_test.go     # All tests
```

---

## 5. Issue State Machine

### 5.1 State Definitions

```go
// pkg/liveness/states.go

package liveness

// IssueState represents the lifecycle state of an issue.
type IssueState string

const (
    StateOpen        IssueState = "open"
    StateInProgress  IssueState = "in_progress"
    StateCompleting  IssueState = "completing"
    StateStale       IssueState = "stale"
    StateAbandoned   IssueState = "abandoned"
    StatePoisoned    IssueState = "poisoned"
    StateClosed      IssueState = "closed"
)

// validTransitions defines all legal state transitions.
// Key = from, Value = allowed destinations.
var validTransitions = map[IssueState][]IssueState{
    StateOpen:       {StateInProgress, StatePoisoned},
    StateInProgress: {StateCompleting, StateStale},
    StateCompleting: {StateClosed},
    StateStale:      {StateOpen, StateAbandoned},
    StateAbandoned:  {StateOpen}, // human override only
    StatePoisoned:   {StateOpen}, // human override only
    StateClosed:     {},          // terminal
}

// ErrInvalidTransition is returned when a state change is not permitted.
type ErrInvalidTransition struct {
    From IssueState
    To   IssueState
}

func (e *ErrInvalidTransition) Error() string {
    return fmt.Sprintf("invalid state transition: %s → %s", e.From, e.To)
}

// ValidateTransition checks whether transitioning from → to is permitted.
func ValidateTransition(from, to IssueState) error {
    allowed, ok := validTransitions[from]
    if !ok {
        return &ErrInvalidTransition{From: from, To: to}
    }
    for _, s := range allowed {
        if s == to {
            return nil
        }
    }
    return &ErrInvalidTransition{From: from, To: to}
}

// TransitionIssue atomically updates issue state, enforcing valid transitions.
func TransitionIssue(ctx context.Context, db *sql.DB, issueID string, from, to IssueState) error {
    if err := ValidateTransition(from, to); err != nil {
        return err
    }
    result, err := db.ExecContext(ctx,
        "UPDATE issues SET state = ? WHERE id = ? AND state = ?",
        string(to), issueID, string(from),
    )
    if err != nil {
        return fmt.Errorf("TransitionIssue %s %s→%s: %w", issueID, from, to, err)
    }
    n, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("TransitionIssue %s: rows affected: %w", issueID, err)
    }
    if n == 0 {
        return fmt.Errorf("TransitionIssue %s: state was not %s (concurrent modification?)", issueID, from)
    }
    return nil
}
```

### 5.2 State Transition Diagram

```
                    ┌─────────────────────────────┐
                    │ claim_count > MaxAttempts   │
          ┌─────────▼──────────┐                  │
          │     POISONED       │◄─────────────────┘
          │  (human override)  │
          └─────────┬──────────┘
                    │ human override
                    ▼
   ┌──────────────────────────────────────────────┐
   │                  OPEN                        │
   └──────────────────┬───────────────────────────┘
                      │ claim (atomic UPDATE WHERE state='open')
                      │ claim_count++
                      ▼
   ┌──────────────────────────────────────────────┐
   │               IN_PROGRESS                    │
   └──────┬───────────────────────────────────────┘
          │                        │
          │ heartbeat stops        │ grava close (phase 1+2)
          │ TTL expires            ▼
          ▼              ┌─────────────────┐
   ┌──────────────┐      │   COMPLETING    │
   │    STALE     │      └────────┬────────┘
   └──────┬───────┘               │ worktree clean + DB commit
          │                       ▼
          │             ┌─────────────────┐
          │             │     CLOSED      │
          │             └─────────────────┘
          │ stale_count < AbandonThreshold
          ├────────────────────────────────► OPEN (re-queued)
          │
          │ stale_count >= AbandonThreshold
          └────────────────────────────────► ABANDONED
```

---

## 6. AgentLiveness Service

```go
// pkg/liveness/liveness.go

package liveness

import (
    "context"
    "database/sql"
    "fmt"
    "log/slog"
    "time"
)

// Config holds all tunable liveness parameters.
type Config struct {
    HeartbeatInterval  time.Duration // how often the agent writes a pulse
    HeartbeatTTL       time.Duration // how long before agent is considered stale
    AbandonThreshold   time.Duration // how long stale before issue is abandoned
    MaxClaimAttempts   int           // claim_count ceiling before issue is poisoned
    DetectorInterval   time.Duration // how often the stale detector scans
}

// DefaultConfig returns safe production defaults.
func DefaultConfig() Config {
    return Config{
        HeartbeatInterval: 15 * time.Second,
        HeartbeatTTL:      60 * time.Second,
        AbandonThreshold:  4 * time.Hour,
        MaxClaimAttempts:  3,
        DetectorInterval:  30 * time.Second,
    }
}

// AgentLiveness manages the heartbeat writer and stale detector for one agent.
type AgentLiveness struct {
    agentID string
    issueID string
    db      *sql.DB
    cfg     Config
    log     *slog.Logger
}

// New creates and registers an AgentLiveness instance. Call Start to begin
// heartbeating.
func New(db *sql.DB, agentID, issueID string, cfg Config, log *slog.Logger) (*AgentLiveness, error) {
    if cfg.HeartbeatInterval <= 0 {
        return nil, fmt.Errorf("liveness: HeartbeatInterval must be > 0")
    }
    if cfg.HeartbeatTTL <= 0 {
        return nil, fmt.Errorf("liveness: HeartbeatTTL must be > 0")
    }
    if cfg.HeartbeatInterval >= cfg.HeartbeatTTL {
        return nil, fmt.Errorf("liveness: HeartbeatInterval (%s) must be < HeartbeatTTL (%s)",
            cfg.HeartbeatInterval, cfg.HeartbeatTTL)
    }
    return &AgentLiveness{
        agentID: agentID,
        issueID: issueID,
        db:      db,
        cfg:     cfg,
        log:     log,
    }, nil
}

// Start launches the heartbeat goroutine. It stops when ctx is cancelled —
// context cancellation signals agent death to the detector.
func (a *AgentLiveness) Start(ctx context.Context) error {
    // Register in liveness table.
    _, err := a.db.ExecContext(ctx, `
        INSERT INTO liveness (agent_id, issue_id, last_seen, state, heartbeat_ttl)
        VALUES (?, ?, NOW(), 'active', ?)
        ON CONFLICT(agent_id) DO UPDATE SET
            issue_id = excluded.issue_id,
            last_seen = excluded.last_seen,
            state = 'active'`,
        a.agentID, a.issueID, int(a.cfg.HeartbeatTTL.Seconds()),
    )
    if err != nil {
        return fmt.Errorf("liveness.Start %s: register: %w", a.agentID, err)
    }

    go a.runHeartbeat(ctx)
    return nil
}

// runHeartbeat writes a pulse every HeartbeatInterval until ctx is cancelled.
func (a *AgentLiveness) runHeartbeat(ctx context.Context) {
    // Renew at HeartbeatTTL/2 to guarantee renewal before expiry.
    interval := a.cfg.HeartbeatTTL / 2
    if a.cfg.HeartbeatInterval < interval {
        interval = a.cfg.HeartbeatInterval
    }

    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if _, err := a.db.ExecContext(ctx,
                "UPDATE liveness SET last_seen = NOW() WHERE agent_id = ?",
                a.agentID,
            ); err != nil {
                a.log.Error("liveness: heartbeat write failed",
                    "agent_id", a.agentID, "err", err)
            }
        case <-ctx.Done():
            // Context cancelled = agent is shutting down or crashed.
            // Mark as dead so detector can act immediately.
            bgCtx := context.Background()
            if _, err := a.db.ExecContext(bgCtx,
                "UPDATE liveness SET state = 'dead' WHERE agent_id = ?",
                a.agentID,
            ); err != nil {
                a.log.Error("liveness: dead mark failed",
                    "agent_id", a.agentID, "err", err)
            }
            return
        }
    }
}

// StaleDetector scans the liveness table and transitions stale issues.
// Run as a singleton background goroutine in the orchestrator.
type StaleDetector struct {
    db  *sql.DB
    cfg Config
    log *slog.Logger
}

// NewStaleDetector creates a detector. There should be exactly one per
// orchestrator instance.
func NewStaleDetector(db *sql.DB, cfg Config, log *slog.Logger) *StaleDetector {
    return &StaleDetector{db: db, cfg: cfg, log: log}
}

// Run starts the detection loop. Blocks until ctx is cancelled.
func (d *StaleDetector) Run(ctx context.Context) {
    ticker := time.NewTicker(d.cfg.DetectorInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := d.scan(ctx); err != nil {
                d.log.Error("liveness: stale scan failed", "err", err)
            }
        case <-ctx.Done():
            return
        }
    }
}

// scan finds agents whose last_seen has exceeded HeartbeatTTL and transitions
// their issues accordingly.
func (d *StaleDetector) scan(ctx context.Context) error {
    rows, err := d.db.QueryContext(ctx, `
        SELECT agent_id, issue_id
        FROM   liveness
        WHERE  state = 'active'
          AND  last_seen < datetime('now', '-' || heartbeat_ttl || ' seconds')`,
    )
    if err != nil {
        return fmt.Errorf("stale scan query: %w", err)
    }
    defer rows.Close()

    type staleAgent struct {
        agentID string
        issueID string
    }
    var stale []staleAgent
    for rows.Next() {
        var a staleAgent
        if err := rows.Scan(&a.agentID, &a.issueID); err != nil {
            return fmt.Errorf("stale scan: scan row: %w", err)
        }
        stale = append(stale, a)
    }
    if err := rows.Err(); err != nil {
        return fmt.Errorf("stale scan: rows: %w", err)
    }

    for _, a := range stale {
        if err := d.markStale(ctx, a.agentID, a.issueID); err != nil {
            d.log.Error("liveness: markStale failed",
                "agent_id", a.agentID, "issue_id", a.issueID, "err", err)
        }
    }
    return nil
}

// markStale transitions one agent's issue from in_progress → stale.
func (d *StaleDetector) markStale(ctx context.Context, agentID, issueID string) error {
    tx, err := d.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("markStale %s: begin tx: %w", agentID, err)
    }
    defer tx.Rollback() //nolint:errcheck

    // Transition issue state.
    if err := TransitionIssue(ctx, d.db, issueID, StateInProgress, StateStale); err != nil {
        return fmt.Errorf("markStale %s: transition issue: %w", agentID, err)
    }

    // Mark liveness row.
    if _, err := tx.ExecContext(ctx,
        "UPDATE liveness SET state = 'stale' WHERE agent_id = ?", agentID,
    ); err != nil {
        return fmt.Errorf("markStale %s: update liveness: %w", agentID, err)
    }

    d.log.Warn("liveness: agent marked stale",
        "agent_id", agentID, "issue_id", issueID)
    return tx.Commit()
}
```

---

## 7. Circuit Breaker

```go
// pkg/liveness/circuit_breaker.go

package liveness

import (
    "context"
    "database/sql"
    "fmt"
)

// ErrIssuePoisoned is returned when an issue has exceeded MaxClaimAttempts.
type ErrIssuePoisoned struct {
    IssueID     string
    ClaimCount  int
    MaxAttempts int
}

func (e *ErrIssuePoisoned) Error() string {
    return fmt.Sprintf("issue %s is POISONED: claimed %d times (max %d) — human intervention required",
        e.IssueID, e.ClaimCount, e.MaxAttempts)
}

// ClaimIssue atomically claims an issue for an agent, enforcing:
//   - State must be 'open' (not in_progress, stale, poisoned, etc.)
//   - claim_count is incremented
//   - If claim_count > MaxClaimAttempts → transition to 'poisoned'
//     and trigger HumanOverseer alert
func ClaimIssue(
    ctx context.Context,
    db *sql.DB,
    issueID, agentID string,
    cfg Config,
    overseer HumanOverseer,
) error {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("ClaimIssue %s: begin tx: %w", issueID, err)
    }
    defer tx.Rollback() //nolint:errcheck

    // Read current state and claim_count atomically.
    var state IssueState
    var claimCount int
    if err := tx.QueryRowContext(ctx,
        "SELECT state, claim_count FROM issues WHERE id = ? FOR UPDATE",
        issueID,
    ).Scan(&state, &claimCount); err != nil {
        return fmt.Errorf("ClaimIssue %s: read state: %w", issueID, err)
    }

    // Guard: only open issues can be claimed.
    if state != StateOpen {
        return fmt.Errorf("ClaimIssue %s: %w", issueID, ErrAlreadyClaimed)
    }

    newClaimCount := claimCount + 1

    // Circuit breaker: poison the issue if max attempts exceeded.
    if newClaimCount > cfg.MaxClaimAttempts {
        if _, err := tx.ExecContext(ctx,
            "UPDATE issues SET state = 'poisoned', claim_count = ? WHERE id = ?",
            newClaimCount, issueID,
        ); err != nil {
            return fmt.Errorf("ClaimIssue %s: poison: %w", issueID, err)
        }
        if err := tx.Commit(); err != nil {
            return fmt.Errorf("ClaimIssue %s: commit poison: %w", issueID, err)
        }
        // Alert human overseer outside the transaction.
        _ = overseer.Broadcast(ctx, fmt.Sprintf(
            "🚨 Issue %s is POISONED after %d failed claim attempts. Manual inspection required.",
            issueID, newClaimCount,
        ), "critical")
        return &ErrIssuePoisoned{
            IssueID:     issueID,
            ClaimCount:  newClaimCount,
            MaxAttempts: cfg.MaxClaimAttempts,
        }
    }

    // Normal claim: transition to in_progress, increment count.
    if _, err := tx.ExecContext(ctx,
        "UPDATE issues SET state = 'in_progress', assignee = ?, claim_count = ? WHERE id = ?",
        agentID, newClaimCount, issueID,
    ); err != nil {
        return fmt.Errorf("ClaimIssue %s: update: %w", issueID, err)
    }

    return tx.Commit()
}

// OverridePoisoned allows a human operator to reset a poisoned issue back to
// open. Requires explicit acknowledgement string to prevent accidental use.
func OverridePoisoned(
    ctx context.Context,
    db *sql.DB,
    issueID string,
    acknowledgement string,
) error {
    const required = "I acknowledge this issue has repeatedly failed"
    if acknowledgement != required {
        return fmt.Errorf("OverridePoisoned: acknowledgement mismatch — pass %q", required)
    }
    _, err := db.ExecContext(ctx,
        "UPDATE issues SET state = 'open', claim_count = 0 WHERE id = ? AND state = 'poisoned'",
        issueID,
    )
    return err
}

// HumanOverseer is the interface for emergency broadcasts.
type HumanOverseer interface {
    Broadcast(ctx context.Context, text, importance string) error
}
```

---

## 8. Doctor Lock

```go
// pkg/liveness/doctor_lock.go

package liveness

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "os"
    "time"
)

const doctorLockTTL = 10 * time.Minute

// ErrDoctorLocked is returned when another doctor instance holds the lock.
var ErrDoctorLocked = errors.New("grava doctor is already running — only one instance permitted")

// DoctorLock is an advisory mutex stored in the DB that prevents concurrent
// grava doctor --fix runs from corrupting repair state.
type DoctorLock struct {
    db      *sql.DB
    lockedBy string
}

// NewDoctorLock creates a DoctorLock using hostname+PID as the lock identity.
func NewDoctorLock(db *sql.DB) *DoctorLock {
    hostname, _ := os.Hostname()
    return &DoctorLock{
        db:       db,
        lockedBy: fmt.Sprintf("%s:%d", hostname, os.Getpid()),
    }
}

// Acquire attempts to acquire the doctor lock. Returns ErrDoctorLocked if
// another live instance holds it. Expired locks (> doctorLockTTL) are
// forcibly released before acquisition.
func (d *DoctorLock) Acquire(ctx context.Context) error {
    tx, err := d.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("DoctorLock.Acquire: begin tx: %w", err)
    }
    defer tx.Rollback() //nolint:errcheck

    var lockedBy sql.NullString
    var expiresAt sql.NullTime
    if err := tx.QueryRowContext(ctx,
        "SELECT locked_by, expires_at FROM doctor_lock WHERE id = 1",
    ).Scan(&lockedBy, &expiresAt); err != nil {
        return fmt.Errorf("DoctorLock.Acquire: read lock row: %w", err)
    }

    // Check if lock is held by a live holder.
    if lockedBy.Valid && expiresAt.Valid && time.Now().Before(expiresAt.Time) {
        return fmt.Errorf("%w (held by %s, expires %s)",
            ErrDoctorLocked, lockedBy.String, expiresAt.Time.Format(time.RFC3339))
    }

    // Expired or unheld — acquire it.
    now := time.Now().UTC()
    if _, err := tx.ExecContext(ctx, `
        UPDATE doctor_lock
        SET    locked_by = ?, locked_at = ?, expires_at = ?
        WHERE  id = 1`,
        d.lockedBy, now, now.Add(doctorLockTTL),
    ); err != nil {
        return fmt.Errorf("DoctorLock.Acquire: write lock: %w", err)
    }

    return tx.Commit()
}

// Release releases the doctor lock. Safe to call if lock is not held.
func (d *DoctorLock) Release(ctx context.Context) error {
    _, err := d.db.ExecContext(ctx, `
        UPDATE doctor_lock
        SET    locked_by = NULL, locked_at = NULL, expires_at = NULL
        WHERE  id = 1 AND locked_by = ?`,
        d.lockedBy,
    )
    if err != nil {
        return fmt.Errorf("DoctorLock.Release: %w", err)
    }
    return nil
}

// WithDoctorLock acquires the lock, runs fn, then releases. Ensures release
// even if fn panics.
func WithDoctorLock(ctx context.Context, db *sql.DB, fn func(ctx context.Context) error) error {
    lock := NewDoctorLock(db)
    if err := lock.Acquire(ctx); err != nil {
        return err
    }
    defer lock.Release(context.Background()) //nolint:errcheck
    return fn(ctx)
}
```

---

## 9. Wisp Validator

```go
// pkg/liveness/wisp_validator.go

package liveness

import (
    "context"
    "crypto/sha256"
    "database/sql"
    "encoding/hex"
    "fmt"
    "log/slog"
)

// Wisp is a checkpoint entry written by an agent during task execution.
type Wisp struct {
    ID       string
    IssueID  string
    AgentID  string
    Seq      int
    Entry    string
    Checksum string
    Valid    bool
}

// ResumeMode describes how a claiming agent should treat an in-progress issue.
type ResumeMode string

const (
    ResumeFresh  ResumeMode = "fresh"  // no prior wisps — start clean
    ResumeResume ResumeMode = "resume" // valid wisps found — checkpoint available
)

// LoadWisps reads and validates all Wisps for an issue. Corrupted entries are
// marked invalid and logged but do NOT fail the load — the caller receives
// whatever valid checkpoints remain.
func LoadWisps(ctx context.Context, db *sql.DB, issueID string, log *slog.Logger) (ResumeMode, []Wisp, error) {
    rows, err := db.QueryContext(ctx, `
        SELECT id, issue_id, agent_id, seq, log_entry, checksum, valid
        FROM   wisps
        WHERE  issue_id = ?
        ORDER  BY seq ASC`,
        issueID,
    )
    if err != nil {
        return "", nil, fmt.Errorf("LoadWisps %s: query: %w", issueID, err)
    }
    defer rows.Close()

    var all []Wisp
    for rows.Next() {
        var w Wisp
        if err := rows.Scan(&w.ID, &w.IssueID, &w.AgentID,
            &w.Seq, &w.Entry, &w.Checksum, &w.Valid); err != nil {
            return "", nil, fmt.Errorf("LoadWisps %s: scan: %w", issueID, err)
        }
        all = append(all, w)
    }
    if err := rows.Err(); err != nil {
        return "", nil, fmt.Errorf("LoadWisps %s: rows: %w", issueID, err)
    }

    // Zero wisps = prior agent crashed before writing anything. Fresh start.
    if len(all) == 0 {
        return ResumeFresh, nil, nil
    }

    // Validate each wisp: checksum + sequence continuity.
    var valid []Wisp
    expectedSeq := 0
    for _, w := range all {
        if !validateWisp(w, expectedSeq, log) {
            // Mark invalid in DB for audit — do not block recovery.
            _, _ = db.ExecContext(ctx,
                "UPDATE wisps SET valid = false WHERE id = ?", w.ID)
            continue
        }
        valid = append(valid, w)
        expectedSeq = w.Seq + 1
    }

    if len(valid) == 0 {
        // All wisps corrupted — treat as fresh, log warning.
        log.Warn("LoadWisps: all wisps corrupted, starting fresh",
            "issue_id", issueID, "total", len(all))
        return ResumeFresh, nil, nil
    }

    return ResumeResume, valid, nil
}

// validateWisp returns true if the wisp passes integrity checks.
func validateWisp(w Wisp, expectedSeq int, log *slog.Logger) bool {
    // Sequence continuity check.
    if w.Seq != expectedSeq {
        log.Warn("LoadWisps: sequence gap detected",
            "wisp_id", w.ID, "expected_seq", expectedSeq, "got_seq", w.Seq)
        return false
    }

    // Checksum verification.
    if w.Checksum != "" {
        h := sha256.Sum256([]byte(w.Entry))
        computed := hex.EncodeToString(h[:])
        if computed != w.Checksum {
            log.Warn("LoadWisps: checksum mismatch",
                "wisp_id", w.ID, "expected", w.Checksum, "got", computed)
            return false
        }
    }

    return true
}

// WriteWisp appends a new validated Wisp entry for the active agent.
func WriteWisp(ctx context.Context, db *sql.DB, issueID, agentID, entry string, seq int) error {
    h := sha256.Sum256([]byte(entry))
    checksum := hex.EncodeToString(h[:])

    _, err := db.ExecContext(ctx, `
        INSERT INTO wisps (id, issue_id, agent_id, seq, log_entry, checksum, valid, created_ts)
        VALUES (?, ?, ?, ?, ?, ?, true, NOW())`,
        newID(), issueID, agentID, seq, entry, checksum,
    )
    if err != nil {
        return fmt.Errorf("WriteWisp %s seq %d: %w", issueID, seq, err)
    }
    return nil
}
```

---

## 10. Abandon Requeue & Doctor Integration

```go
// pkg/liveness/liveness.go (continued — RequeueStale)

// RequeueStale re-opens issues that have been stale past the AbandonThreshold,
// or abandons them if they exceed it. Preserves crash context as a DB annotation.
func (d *StaleDetector) RequeueStale(ctx context.Context) error {
    rows, err := d.db.QueryContext(ctx, `
        SELECT i.id, i.claim_count,
               julianday('now') - julianday(l.last_seen) as stale_days
        FROM   issues i
        JOIN   liveness l ON l.issue_id = i.id
        WHERE  i.state = 'stale'`,
    )
    if err != nil {
        return fmt.Errorf("RequeueStale: query: %w", err)
    }
    defer rows.Close()

    type staleIssue struct {
        id         string
        claimCount int
        staleDays  float64
    }
    var issues []staleIssue
    for rows.Next() {
        var s staleIssue
        if err := rows.Scan(&s.id, &s.claimCount, &s.staleDays); err != nil {
            return fmt.Errorf("RequeueStale: scan: %w", err)
        }
        issues = append(issues, s)
    }

    abandonSeconds := d.cfg.AbandonThreshold.Seconds()
    for _, s := range issues {
        if s.staleDays*86400 >= abandonSeconds {
            if err := TransitionIssue(ctx, d.db, s.id, StateStale, StateAbandoned); err != nil {
                d.log.Error("RequeueStale: abandon failed", "issue_id", s.id, "err", err)
            } else {
                d.log.Warn("liveness: issue abandoned", "issue_id", s.id,
                    "claim_count", s.claimCount, "stale_days", s.staleDays)
            }
        } else {
            // Re-open: preserve claim context (claim_count is NOT reset)
            if err := TransitionIssue(ctx, d.db, s.id, StateStale, StateOpen); err != nil {
                d.log.Error("RequeueStale: reopen failed", "issue_id", s.id, "err", err)
            } else {
                d.log.Info("liveness: stale issue requeued", "issue_id", s.id)
            }
        }
    }
    return nil
}
```

### `grava doctor` Integration

```go
// cmd/doctor/doctor.go

func runDoctor(ctx context.Context, db *sql.DB, fix, dryRun bool) error {
    // Guard: mutually exclusive flags
    if fix && dryRun {
        return errors.New("--fix and --dry-run are mutually exclusive")
    }

    // Guard: advisory lock prevents concurrent repair runs
    if fix {
        return liveness.WithDoctorLock(ctx, db, func(ctx context.Context) error {
            return runChecks(ctx, db, true)
        })
    }
    return runChecks(ctx, db, false)
}

func runChecks(ctx context.Context, db *sql.DB, fix bool) error {
    checks := []checkFn{
        checkStaleHeartbeats,
        checkGhostWorktrees,
        checkOrphanedDirectories,
        checkOrphanedBranches,
        checkStaleLockFiles,
        checkExpiredReservations,
    }
    for _, check := range checks {
        if err := check(ctx, db, fix); err != nil {
            slog.Error("doctor check failed", "err", err)
        }
    }
    return nil
}
```

---

## 11. Backup Before Purge

```go
// pkg/liveness/backup.go

// BackupOrphanedState exports orphaned records to a JSON file before purge.
// Verifies backup integrity by comparing record count before returning.
func BackupOrphanedState(ctx context.Context, db *sql.DB, backupPath string, issueIDs []string) error {
    if len(issueIDs) == 0 {
        return nil
    }

    // Collect all related records.
    backup := struct {
        Issues []map[string]any `json:"issues"`
        Wisps  []map[string]any `json:"wisps"`
    }{}

    for _, id := range issueIDs {
        issue, err := fetchIssueMap(ctx, db, id)
        if err != nil {
            return fmt.Errorf("backup: fetch issue %s: %w", id, err)
        }
        backup.Issues = append(backup.Issues, issue)

        wisps, err := fetchWispsMap(ctx, db, id)
        if err != nil {
            return fmt.Errorf("backup: fetch wisps %s: %w", id, err)
        }
        backup.Wisps = append(backup.Wisps, wisps...)
    }

    // Write backup file.
    data, err := json.MarshalIndent(backup, "", "  ")
    if err != nil {
        return fmt.Errorf("backup: marshal: %w", err)
    }
    if err := os.WriteFile(backupPath, data, 0644); err != nil {
        return fmt.Errorf("backup: write file: %w", err)
    }

    // Verify integrity: re-read and compare record count.
    var verify struct {
        Issues []map[string]any `json:"issues"`
        Wisps  []map[string]any `json:"wisps"`
    }
    raw, err := os.ReadFile(backupPath)
    if err != nil {
        return fmt.Errorf("backup: verify read: %w", err)
    }
    if err := json.Unmarshal(raw, &verify); err != nil {
        return fmt.Errorf("backup: verify parse: %w", err)
    }
    if len(verify.Issues) != len(backup.Issues) {
        return fmt.Errorf("backup: integrity check failed: wrote %d issues, verified %d",
            len(backup.Issues), len(verify.Issues))
    }

    return nil
}
```

---

## 12. Acceptance Criteria

### AC-1: Heartbeat & Stale Detection
- [ ] Agent starts heartbeat on `liveness.Start(ctx)`
- [ ] Heartbeat writes at `HeartbeatTTL/2` interval (not HeartbeatInterval if larger)
- [ ] `New()` returns error if `HeartbeatInterval == 0` or `HeartbeatInterval >= HeartbeatTTL`
- [ ] Context cancellation marks agent as `dead` in liveness table
- [ ] StaleDetector transitions `in_progress → stale` within 2× DetectorInterval of TTL expiry
- [ ] Stale issue is re-queued to `open` if under AbandonThreshold
- [ ] Stale issue transitions to `abandoned` if over AbandonThreshold

### AC-2: Circuit Breaker
- [ ] `claim_count` increments on every `ClaimIssue` call
- [ ] Issue transitions to `poisoned` when `claim_count > MaxClaimAttempts`
- [ ] `ClaimIssue` returns `ErrIssuePoisoned` for poisoned issues
- [ ] HumanOverseer `Broadcast` is called exactly once on poison transition
- [ ] `OverridePoisoned` requires exact acknowledgement string
- [ ] `OverridePoisoned` resets `claim_count` to 0

### AC-3: Doctor Lock
- [ ] `WithDoctorLock` prevents two concurrent `--fix` runs
- [ ] Expired lock (> `doctorLockTTL`) is forcibly released before acquisition
- [ ] `--fix` and `--dry-run` flags together return error before acquiring lock
- [ ] Lock is released via `defer` even if repair function panics

### AC-4: Wisp Validator
- [ ] Zero wisps returns `ResumeFresh`, nil slice, nil error
- [ ] All-corrupted wisps returns `ResumeFresh` with warning log (not error)
- [ ] Sequence gap marks wisp invalid, skips it, continues with remaining
- [ ] Checksum mismatch marks wisp invalid in DB
- [ ] `WriteWisp` computes and stores sha256 checksum

### AC-5: State Machine
- [ ] Invalid transition returns `ErrInvalidTransition` (not panic)
- [ ] `TransitionIssue` uses `WHERE state = ?` guard — returns error if state changed concurrently
- [ ] All 7 states representable; `closed` has no outgoing transitions

### AC-6: Backup Before Purge
- [ ] Backup file written before any DELETE executed
- [ ] Integrity check (record count match) runs before returning success
- [ ] Purge aborts if backup file write or verify fails

---

## 13. Findings Resolution Map

| # | Finding | Closed By |
|---|---------|-----------|
| ECH-1 | Dual-delete raises false conflict | merge driver (existing) |
| ECH-2 | Dual-insert silent overwrite | merge driver (existing) |
| ECH-3 | Malformed JSONL crashes driver | merge driver (existing) |
| ECH-4 | Missing policy config | merge driver (existing) |
| ECH-10 | Poison task infinite crash loop | **AC-2 CircuitBreaker** |
| ECH-12 | SELECT FOR UPDATE deadlock | **AC-5 TransitionIssue** |
| ECH-13 | Partial commit (assignee/status) | **AC-5 TransitionIssue** |
| ECH-14 | Zero-Wisp crash recovery | **AC-4 WispValidator** |
| ECH-15 | Premature Wisp deletion | **AC-4 WriteWisp + closeTask** |
| ECH-16 | ack_required timeout | HumanOverseer (existing) |
| ADV-1 | No heartbeat — cannot detect dead agent | **AC-1 AgentLiveness** |
| ADV-2 | Wisp write frequency unspecified | **AC-4 WriteWisp (seq + checksum)** |
| ADV-3 | Directory removed before DB commit | **AC-5 closeTask two-phase** |
| ADV-4 | Backup mechanism unspecified | **AC-6 BackupOrphanedState** |
| ADV-5 | TOCTOU race on branch deletion | **AC-5 TransitionIssue WHERE guard** |
| ADV-6 | Heartbeat mechanism undefined | **AC-1 liveness table + StaleDetector** |
| ADV-7 | Background cleanup job unspecified | **AC-1 StaleDetector.Run goroutine** |
| ADV-8 | open vs closed branch conflated | **AC-5 state machine — open ≠ orphaned** |
| ADV-9 | Detached HEAD / merge conflict state | **AC-4 WispValidator + hasUncommittedChanges** |
| ADV-10 | No circuit breaker for poison tasks | **AC-2 CircuitBreaker** |
| ADV-11 | Ghost reset loses crash context | **AC-1 liveness row preserved on reset** |
| ADV-12 | Concurrent doctor runs | **AC-3 DoctorLock** |
| FECH-1 | Heartbeat threshold not validated | **AC-1 New() validation** |
| FECH-2 | Background cleanup crashes mid-release | **AC-1 idempotent UPDATE released_ts** |
| FECH-3 | Issue stuck in_progress permanently | **AC-1 AbandonThreshold → abandoned** |
| FECH-4 | Wisps corrupted at crash boundary | **AC-4 WispValidator checksum + seq** |
| FECH-7 | Double failure on close | **AC-5 closeTask idempotent** |
| FECH-13 | --fix and --dry-run simultaneously | **AC-3 DoctorLock mutual exclusion** |
| FECH-14 | Backup not verified before purge | **AC-6 BackupOrphanedState verify** |
| FECH-15 | Active agent loses lease (TTL too short) | **AC-1 renew at HeartbeatTTL/2** |

---

## 14. Implementation Order

```
Sprint 1 (foundation):
  1. DB migrations (liveness table, doctor_lock, issues extension, wisps extension)
  2. states.go — IssueState enum + ValidateTransition + TransitionIssue
  3. Tests for state machine transitions

Sprint 2 (liveness):
  4. liveness.go — AgentLiveness + StaleDetector + RequeueStale
  5. circuit_breaker.go — ClaimIssue + OverridePoisoned
  6. Tests for heartbeat, stale detection, circuit breaker

Sprint 3 (safety):
  7. doctor_lock.go — DoctorLock + WithDoctorLock
  8. wisp_validator.go — LoadWisps + WriteWisp + validateWisp
  9. backup.go — BackupOrphanedState

Sprint 4 (integration):
  10. Wire AgentLiveness.Start into grava claim
  11. Wire StaleDetector.Run into orchestrator main loop
  12. Wire WithDoctorLock into grava doctor --fix
  13. Wire LoadWisps into grava claim (resume path)
  14. Full integration tests
```
