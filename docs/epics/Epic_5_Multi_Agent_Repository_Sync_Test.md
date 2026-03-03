# Epic 5: Multi-Agent Repository Sync Test

**Goal:** Verify that two or more AI agents operating in isolated clones of the same Grava workspace can concurrently work, push/pull changes, and successfully leverage the Git Merge Driver to resolve any data conflicts without manual intervention.

**Success Criteria:**
- Integration test framework successfully provisions isolated agent environments.
- Agents can simultaneously operate on the same issue hierarchy (e.g., creating tasks under the same epic).
- Concurrent edits to different fields on the same issue ID are successfully resolved without data loss.
- The `grava merge-slot` custom git driver triggers automatically from regular `git pull` and resolves `.jsonl` data appropriately.

## User Stories

### 5.1 Test Environment Setup
**As a** system tester  
**I want to** easily provision separate workspace environments  
**So that** I can test concurrent use-cases locally without interfering with my main workspace

**Acceptance Criteria:**
- A Bash script initializes a bare Git repository as a central remote.
- The script clones two separate workspaces for Agent 1 and Agent 2.
- Each agent workspace initializes Grava using a separate, conflict-free port for its Dolt database.

### 5.2 Concurrent Epic & Task Lifecycle Simulation
**As a** developer  
**I want to** simulate two agents creating and updating issues concurrently  
**So that** I can ensure the database and graph models gracefully handle complex push/pull events

**Acceptance Criteria:**
- Agent 1 successfully pushes a generated Epic to the remote.
- Agent 2 pulls the Epic and generates multiple subtasks simultaneously.
- Both agents establish valid data references inside Dolt without ID sequence conflicts.

### 5.3 Automated Field-Level Conflict Resolution
**As a** user  
**I want to** rely on my Git merge driver  
**So that** two agents modifying different parts of the same issue don't trigger a merge conflict error

**Acceptance Criteria:**
- Agent 1 modifies the Status field of a specific Subtask.
- Agent 2 modifies the Description field of that identical Subtask on their respective head.
- Both agents attempt to push.
- The Git pull automatically triggers `grava merge-slot`, accurately applying the 3-way merge logic.
- After the simulated sync, the Subtask retains both the new Status and the new Description.
