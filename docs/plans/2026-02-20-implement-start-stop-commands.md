# Implement grava start and stop commands

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement `grava start` and `grava stop` as CLI commands that wrap existing shell scripts.

**Architecture:**
- `pkg/cmd/start.go` and `pkg/cmd/stop.go` added.
- Existing scripts modified to accept port arguments and handle non-interactive mode.
- Commands discover scripts relative to the binary or CWD.

---

### Task 1: Enhance Shell Scripts ✅
**Files:**
- Modify: `scripts/start_dolt_server.sh`
- Modify: `scripts/stop_dolt_server.sh`

**Step 1: Update `start_dolt_server.sh`** ✅
- Accept `PORT` as first argument (default 3306).
- Use `PORT=${1:-3306}`.

**Step 2: Update `stop_dolt_server.sh`** ✅
- Accept `PORT` as first argument.
- Accept `--yes` or `-y` flag to skip interactive check.
- Handle port discovery from arguments.

**Step 3: Commit** ✅
```bash
git add scripts/start_dolt_server.sh scripts/stop_dolt_server.sh
git commit -m "refactor: enhance scripts to support port arguments and non-interactive stop"
```

---

### Task 2: Implement Utility for Script Discovery ✅
**Files:**
- Create: `pkg/utils/path.go`

**Step 1: Add `FindScript(name string) (string, error)`** ✅
- Search in `./scripts/`.
- Search in `<binary_dir>/scripts/`.

**Step 2: Commit** ✅
```bash
git add pkg/utils/path.go
git commit -m "feat: add utility to find scripts relative to binary or CWD"
```

---

### Task 3: Implement `grava start` Command ✅
**Files:**
- Create: `pkg/cmd/start.go`
- Modify: `pkg/cmd/root.go` (register command)

**Step 1: Implement the command** ✅
- Read port from `db_url` in config.
- Find `start_dolt_server.sh`.
- Run it in background using `exec.Command(...).Start()`.
- Redirect output to `.grava/dolt.log`.

**Step 2: Commit** ✅
```bash
git add pkg/cmd/start.go pkg/cmd/root.go
git commit -m "feat: implement grava start command"
```

---

### Task 4: Implement `grava stop` Command ✅
**Files:**
- Create: `pkg/cmd/stop.go`
- Modify: `pkg/cmd/root.go` (register command)

**Step 1: Implement the command** ✅
- Read port from `db_url` in config.
- Find `stop_dolt_server.sh`.
- Run it with the port and `-y` flag.

**Step 2: Commit** ✅
```bash
git add pkg/cmd/stop.go pkg/cmd/root.go
git commit -m "feat: implement grava stop command"
```

---

### Task 5: Verification ✅
**Files:**
- Action: Test in `example/`

**Step 1: Build and Test** ✅
- `go build -o grava`
- `./grava stop`
- `./grava start`
- Check if it's running via `lsof -i :<port>` or `./grava list`.
- `./grava stop`

**Step 2: Cleanup and Final Commit** ✅
```bash
git commit --allow-empty -m "test: verified grava start/stop commands"
```
