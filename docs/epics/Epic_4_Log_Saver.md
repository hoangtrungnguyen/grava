# Epic 4: Grava Flight Recorder (System-Wide Trace Logging)

**Goal:** Implement a persistent, structured logging system that captures command execution, internal errors, stack traces, and agent interactions without cluttering the user's terminal.

**Success Criteria:**
- `slog` infrastructure initialized with custom handler writing to `.grava/logs/grava.log`
- Log rotation preventing infinite file growth
- Global `--debug` flag enabling verbose logging
- Panic recovery middleware capturing stack traces
- `grava debug logs` command implemented for viewing and exporting logs
- Session artifact storage capturing LLM prompts and responses

## User Stories

### 4.1 Structured Logger Initialization
**As a** developer
**I want to** have a structured logging system
**So that** I can debug issues with machine-readable logs

**Acceptance Criteria:**
- `internal/debug/logger.go` created
- `Setup(verbose bool)` function defined
- Logs stored in `.grava/logs/grava.log`
- Format is JSON Lines (`jsonl`) with timestamp, level, message, metadata
- Unique `ExecutionID` (UUID) attached to every log entry

### 4.2 Log Rotation
**As a** user
**I want to** prevent log files from continuously growing
**So that** I don't run out of disk space

**Acceptance Criteria:**
- Check `grava.log` size on startup
- If > 5MB, rename to `grava.log.1` (rolling previous ones)
- Max 5 files kept

### 4.3 Debug Flag
**As a** developer
**I want to** enable verbose logging on demand
**So that** I can see internal state during debugging

**Acceptance Criteria:**
- `--debug` flag added to root command
- `GRAVA_DEBUG` environment variable supported
- Default: Log only `INFO` and `ERROR` to file
- With debug enabled: Log `DEBUG` level and print to `stderr` in real-time

### 4.4 Panic Recovery Middleware
**As a** system maintainer
**I want to** catch crashes and log stack traces
**So that** I can diagnose unexpected failures

**Acceptance Criteria:**
- `defer/recover` block in `main.go`
- Stack trace captured on panic
- Logged with level `FATAL` and `ExecutionID`
- User-friendly message printed to stderr

### 4.5 Command Tracing
**As a** developer
**I want to** trace command execution
**So that** I know exactly what commands were run and with what arguments

**Acceptance Criteria:**
- Log `Command started`, `Args`, `Flags` in `PersistentPreRun`
- Log `Command finished`, `Duration` in `PersistentPostRun`

### 4.6 Session Artifact Storage
**As an** AI agent developer
**I want to** capture LLM prompts and responses
**So that** I can debug agent decision-making ("vibe coding")

**Acceptance Criteria:**
- LLM prompts and responses saved to `.grava/logs/artifacts/<ExecutionID>_prompt.txt`
- Main log entry references the artifact file
- Keeps main logs clean while preserving full context

### 4.7 Debug Logs Command
**As a** user
**I want to** easily view and export logs
**So that** I can troubleshoot issues or report bugs

**Acceptance Criteria:**
- `grava debug logs` prints path to log file
- `grava debug logs --last` prints logs from last execution
- `grava debug logs --export` zips `.grava/logs` into `grava_debug_bundle.zip`
