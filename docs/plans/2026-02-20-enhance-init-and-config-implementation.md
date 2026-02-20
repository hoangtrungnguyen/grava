# Enhance grava init and add grava config Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform `grava init` into a zero-config setup that initializes Dolt, finds a port, and starts the server, plus add a `grava config` command.

**Architecture:**
- `init` command updated with port scanning and background server execution.
- `config` command added to `pkg/cmd/config.go`.
- Port detection uses `net.Listen` to check availability.

**Tech Stack:** Go, Cobra, Viper, Dolt.

---

### Task 1: Add Port Detection Utility
**Files:**
- Create: `pkg/utils/net.go`
- Test: `pkg/utils/net_test.go`

**Step 1: Write helper function for port detection**
Add `FindAvailablePort(start int) int`.

**Step 2: Commit**
```bash
git add pkg/utils/net.go
git commit -m "feat: add available port detection utility"
```

---

### Task 2: Implement `grava config` Command
**Files:**
- Create: `pkg/cmd/config.go`
- Modify: `pkg/cmd/root.go:90` (register command)

**Step 1: Implement the command**
```go
var configCmd = &cobra.Command{
    Use:   "config",
    Short: "Show current configuration",
    Run: func(cmd *cobra.Command, args []string) {
        // Print viper values and config file path
    },
}
```

**Step 2: Commit**
```bash
git add pkg/cmd/config.go pkg/cmd/root.go
git commit -m "feat: add grava config command"
```

---

### Task 3: Enhance `grava init` - Dolt Repos and Port Scanning
**Files:**
- Modify: `pkg/cmd/init.go`

**Step 1: Update `RunE` to init dolt**
- Check if `.grava/dolt` exists.
- If not, run `dolt init` in that directory.

**Step 2: Update `RunE` to find port**
- If port 3306 is taken, scan for the next available.
- Update the generated `db_url` with the new port.

**Step 3: Commit**
```bash
git add pkg/cmd/init.go
git commit -m "feat: enhance init with dolt repo setup and port scan"
```

---

### Task 4: Enhance `grava init` - Start Dolt Server
**Files:**
- Modify: `pkg/cmd/init.go`

**Step 1: Start server in background**
- Use `exec.Command` for `dolt sql-server`.
- Run in background via `cmd.Start()`.
- Add a small delay and check if PID is still alive.

**Step 2: Commit**
```bash
git add pkg/cmd/init.go
git commit -m "feat: init command now starts dolt server in background"
```

---

### Task 5: Integration Testing in `@example`
**Files:**
- Action: Run commands in `example/`

**Step 1: Test full flow**
- `rm -rf example/.grava`
- `cd example && ../grava init`
- `../grava config`
- `../grava list`

**Step 2: Cleanup and Final Commit**
```bash
git commit --allow-empty -m "test: verified init and config in example directory"
```
