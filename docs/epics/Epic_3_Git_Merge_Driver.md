# Epic 3: Git Merge Driver

**Goal:** Transform Git from a text-based version control system into a schema-aware database manager for Grava. The system will automatically resolve conflicts when two users modify different fields of the same issue, or add different issues simultaneously.

**Success Criteria:**
- Git merge driver `grava merge-slot` implemented and handling JSONL merges
- 3-way merge logic correctly resolving field-level conflicts
- Automated installer for `.git/config` and `.gitattributes`
- Integration tests verifying conflict-free merges across branches

## User Stories

### 3.1 Git Merge Driver Command
**As a** Git user  
**I want to** have a custom merge driver command `grava merge-slot`  
**So that** Git can delegate JSONL file merging to a schema-aware tool

**Acceptance Criteria:**
- `grava merge-slot` command accepts `%O` (Ancestor), `%A` (Current), and `%B` (Other) arguments
- Command reads the three provided file paths into memory
- Command parses JSONL content correctly into internal structures
- Command handles file read errors gracefully and returns appropriate exit codes

### 3.2 Three-Way Merge Logic
**As a** developer  
**I want to** merge changes based on Issue ID rather than line numbers  
**So that** concurrent edits to different fields don't cause text conflicts

**Acceptance Criteria:**
- Logic compares Ours vs Ancestor and Theirs vs Ancestor based on ID
- Merges non-conflicting field updates (e.g., changes to `title` vs `status`) automatically
- Detects true conflicts (same field modified differently) and fails merge (exit 1)
- Handles deletion vs modification conflicts according to policy (Delete wins or Conflict)
- Serializes result back to `%A` (Current file) in strict JSONL format

### 3.3 Automated Driver Installation
**As a** user  
**I want to** automatically configure my Git environment  
**So that** I don't have to manually edit config files

**Acceptance Criteria:**
- `grava doctor` or install command checks/updates `.git/config`
- Driver definition added: `[merge "grava"] name = Grava merge driver driver = grava merge-slot --ancestor %O --current %A --other %B --output %A`
- `.grava/.gitattributes` checked/updated to encompass `*.jsonl merge=grava`
- Implementation verifies `grava` binary is in the user's system `$PATH`

### 3.4 Integration Testing & Verification
**As a** QA engineer  
**I want to** verify the driver works in a real Git environment  
**So that** I can trust it with production data

**Acceptance Criteria:**
- Automated test script initializes a Git repo and branches
- Simulates concurrent edits to different fields of the same issue ID on separate branches
- Verifies `git merge` completes automatically
- Verifies `issues.jsonl` content is correct after merge (contains both changes)
- Verifies `git merge` fails on true conflicts (same field modified)
