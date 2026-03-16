# How Beads Manages Conflict Resolution

> **System:** Beads (`bd`) — a git-native issue tracker built on Dolt (version-controlled SQL database)
> **Source:** [deepwiki.com/steveyegge/beads](https://deepwiki.com/steveyegge/beads)

---

## Table of Contents

1. [Architecture Context](#1-architecture-context)
2. [The Three Conflict Types](#2-the-three-conflict-types)
3. [Conflict Detection](#3-conflict-detection)
4. [Resolution Pathways](#4-resolution-pathways)
5. [The `bd doctor` System](#5-the-bd-doctor-system)
6. [Human Intervention Points](#6-human-intervention-points)
7. [Conflict Prevention Strategies](#7-conflict-prevention-strategies)
8. [End-to-End Decision Flow](#8-end-to-end-decision-flow)

---

## 1. Architecture Context

Beads uses a **dual-layer persistence model**, which is the foundation of its conflict story:

| Layer | Component | Role |
|---|---|---|
| Primary DB | Dolt SQL Server | Version-controlled MySQL-compatible DB. All writes land here first. |
| Source of Truth | `.beads/issues.jsonl` | Git-tracked flat file, updated by `bd sync`. |
| Remote Sync | DoltHub / S3 / GCS / `file://` | Push/pull targets for Dolt commits. |
| Config Surface 1 | `dolt_remotes` SQL table | In-server remote registry for `CALL DOLT_PUSH/PULL`. |
| Config Surface 2 | `.dolt/repo_state.json` | CLI filesystem remote registry for the `dolt` subprocess. |

The dual-surface remote architecture (SQL + CLI) is the root cause of **remote configuration conflicts**. The dual-layer persistence (Dolt + JSONL) defines the sync workflow after conflicts are resolved.

Every write also goes through a **two-phase commit**:

- **Phase 1 — SQL COMMIT:** Persists the working set (including ephemeral wisp data in `dolt_ignore` tables).
- **Phase 2 — DOLT_COMMIT:** Creates a version history snapshot (permanent tables only, excludes `dolt_ignore`).

If Phase 2 returns `"nothing to commit"` (only wisps were written), this is treated as benign — data is already safe from Phase 1.

---

## 2. The Three Conflict Types

### 2.1 Data Conflicts

Arise when two users or agents modify the same data and then merge.

#### Auto-resolved: Cell-level merge

Dolt operates at **cell granularity**, not row or file level. If two writers touch *different columns* of the same row, Dolt auto-merges with no conflict.

> **Example:** User A sets `issue.priority = 'critical'`. User B sets `issue.status = 'in_progress'` on the same issue. → Auto-merged. ✅

#### True conflict: Same cell modified

When both sides modify the **exact same cell**, Dolt writes conflict records into `dolt_conflicts_<table>`:

| Column Prefix | Meaning |
|---|---|
| `base_*` | Common ancestor value |
| `our_*` | Local branch value |
| `their_*` | Remote branch value |

---

### 2.2 Push/Pull State Conflicts

Not about data content — about repository state.

**Divergent History**
```
Error: push rejected: remote has diverged
```
Remote has commits the local DB lacks. Push is blocked to prevent data loss.

**Uncommitted Working Set**
```
Error: cannot push: working set has uncommitted changes
```
Local changes must be committed before pushing.

---

### 2.3 Remote Configuration Conflicts

Beads maintains remotes on two independent surfaces. Discrepancies cause push/pull failures.

| State | SQL Surface | CLI Surface | Severity |
|---|---|---|---|
| ✅ In Sync | `origin → https://dolthub.com/org/repo` | same | None |
| ⚠️ SQL Only | `origin → https://...` | *(none)* | Warning |
| ⚠️ CLI Only | *(none)* | `origin → git@github.com:...` | Warning |
| ❌ CONFLICT | `origin → https://url1.com` | `origin → https://url2.com` | Error |

---

## 3. Conflict Detection

| Conflict Type | How Detected | When |
|---|---|---|
| Data conflict | `CALL DOLT_PULL(?)` populates `dolt_conflicts_<table>` | During `bd dolt pull` |
| Divergent history | Push is rejected by remote | During `bd dolt push` |
| Uncommitted working set | DoltStore checks working set cleanliness | During `bd dolt push` |
| Remote config mismatch | `bd doctor` / `bd dolt remote list` compare both surfaces | On demand |

The DoltStore resilience layer adds **exponential backoff retry** (up to 30s) and a **circuit breaker** that trips after 3 consecutive failures, distinguishing transient network errors from true conflicts.

---

## 4. Resolution Pathways

### 4.1 Data Conflict — Automatic

For non-overlapping cell changes: fully automatic. No human needed.

The two-phase commit persists data safely even when `DOLT_COMMIT` has nothing to snapshot.

---

### 4.2 Data Conflict — Manual Resolution

When Dolt cannot auto-resolve, a human or agent must intervene via SQL:

**Step-by-step:**

1. **Detect** — `bd dolt pull` fails with conflict notice
2. **Inspect** — Query `dolt_conflicts_issues`
3. **Decide** — Choose which value wins (or craft a merged value)
4. **Apply** — `UPDATE` the table to the correct value
5. **Clear** — `CALL DOLT_CONFLICTS_RESOLVE(...)` to remove conflict markers
6. **Commit** — `bd dolt commit` to finalize
7. **Push** — `bd dolt push` to propagate

**Example SQL workflow:**

```sql
-- 1. Inspect the conflict
SELECT base_priority, our_priority, their_priority
  FROM dolt_conflicts_issues WHERE our_id = 'bd-abc123';

-- 2. Apply resolution (accept the remote value)
UPDATE issues SET priority = (
  SELECT their_priority FROM dolt_conflicts_issues WHERE our_id = 'bd-abc123'
) WHERE id = 'bd-abc123';

-- 3. Clear conflict marker
CALL DOLT_CONFLICTS_RESOLVE('issues', 'theirs');
```

```bash
# 4. Commit and push
bd dolt commit -m "Resolved priority conflict on bd-abc123"
bd dolt push
```

---

### 4.3 Divergent History Resolution

| Scenario | Resolution | Risk |
|---|---|---|
| Remote has new commits | `bd dolt pull` → then `bd dolt push` | Low |
| Pull reveals data conflicts | Follow §4.2 manual workflow | Medium |
| Remote changes are disposable | `bd dolt push --force` | ⚠️ High — destroys remote history |

> **Warning:** `--force` should never be used on shared production remotes. Reserve for personal staging areas or coordinated rollback only.

---

### 4.4 Uncommitted Working Set

Run `bd dolt commit` before pushing. Three commit modes are supported:

| Mode | Behavior |
|---|---|
| `immediate` (default) | Auto-commit after every write command |
| `batch` | Accumulate changes; human runs `bd dolt commit` manually |
| `off` | All commits manual |

In `immediate` mode, a `PersistentPostRun` hook fires after every write. The flag `commandDidExplicitDoltCommit` prevents double-committing when the user already called `bd dolt commit` explicitly.

---

### 4.5 Remote Configuration Resolution

```bash
# Auto-fix one-sided discrepancies (SQL-only or CLI-only)
bd doctor --fix

# Manual fix for URL conflicts (same name, different URL)
bd dolt remote remove origin
bd dolt remote add origin https://dolthub.com/org/correct-repo
```

`bd doctor --fix` always registers remotes on **both surfaces atomically** via `bd dolt remote add` (not the raw `dolt remote add`), preventing future drift.

---

## 5. The `bd doctor` System

`bd doctor` is Beads' primary diagnostic and self-repair command — intended as the **first command run** when troubleshooting.

| Command | Purpose |
|---|---|
| `bd doctor` | Full health check: 30+ checks across 7 categories |
| `bd doctor --fix` | Auto-repair: remote config, lock files, gitignore, hooks |
| `bd doctor --deep` | Extended: parent/child consistency, dependency graph integrity |
| `bd doctor --agent` | AI-facing output: observed state, expected state, exact remediation commands |
| `bd doctor --check=validate` | Data integrity: duplicates, orphaned deps, test pollution |
| `bd doctor --migration=pre\|post` | Validate before/after a Dolt migration |
| `bd doctor --server` | Validate Dolt server-mode connection (remote server setups) |

Output has three severity levels: **blocking** (prevents operation), **degraded** (partial function), **advisory** (informational).

---

## 6. Human Intervention Points

| Conflict Type | Automatic | Human Must Do |
|---|---|---|
| Different cells modified | ✅ Auto-merged | Nothing |
| Same cell modified | ❌ | Inspect `dolt_conflicts_<table>`, UPDATE rows, `DOLT_CONFLICTS_RESOLVE`, commit, push |
| Remote diverged | ❌ | `bd dolt pull` first, then resolve data conflicts if any, then push |
| Uncommitted changes | ❌ | `bd dolt commit` to flush working set |
| Remote config — one-sided | ✅ `bd doctor --fix` | Nothing |
| Remote config — URL conflict | ❌ | Manual: `remote remove` + `remote add` |
| Lock file | ✅ `bd doctor --fix` | Nothing |

---

## 7. Conflict Prevention Strategies

| Strategy | Description | Trade-off |
|---|---|---|
| Frequent Pull | Pull before each work session | More network calls; minimizes divergence |
| Immediate Auto-Commit | Default mode — commit after every write | Dense history; reduces uncommitted-change conflicts |
| Branch-per-Feature | Use `DOLT_CHECKOUT` for isolation | Safe isolation; requires explicit merge coordination |
| Advisory Locking | Use issue status fields (`hooked`, `pinned`) as soft locks | Prevents logical conflicts; not enforced at DB level |
| Remote Consistency Checks | Run `bd doctor` before push/pull in CI | Catches config drift early |

---

## 8. End-to-End Decision Flow

```
bd dolt pull
    │
    ├── Different cells modified ──────────────────────── Auto-merged ✅
    │
    └── Same cell modified ─────────────────────────────► dolt_conflicts_<table>
                                                               │
                                                    Inspect base/our/their values
                                                               │
                                                    UPDATE rows to correct value
                                                               │
                                                    CALL DOLT_CONFLICTS_RESOLVE
                                                               │
                                                    bd dolt commit → bd dolt push ✅

bd dolt push fails (diverged)
    │
    ├── bd dolt pull succeeds ──────────────────────────── bd dolt push ✅
    ├── bd dolt pull has conflicts ──────────────────────── Manual resolution (above)
    └── Remote disposable ────────────────────────────────  bd dolt push --force ⚠️

bd doctor (remote config conflict)
    ├── SQL-only or CLI-only remote ───────────────────── bd doctor --fix ✅
    └── URL conflict ─────────────────────────────────────  Manual: remove + re-add remote
```

---

## Summary

Beads' conflict resolution is built on three principles:

1. **Automate the unambiguous** — Dolt's cell-level merge silently handles the majority of concurrent edits.
2. **Surface what requires judgment** — True conflicts land in queryable SQL system tables (`dolt_conflicts_*`), inspectable and resolvable by both humans and AI agents via standard SQL.
3. **Provide a diagnostics safety net** — `bd doctor` detects configuration drift, lock issues, and integration failures, with `--fix` for auto-repair and `--agent` mode for AI consumption.

Most conflicts resolve silently. Those that require judgment are presented as structured, programmable data — not opaque merge errors.

---

*Sources: [deepwiki.com/steveyegge/beads](https://deepwiki.com/steveyegge/beads) — sections 4.5, 5.1, 5.5, 6.1, 6.5, 10.1*
