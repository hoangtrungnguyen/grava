# Epic 3: Dedicated Sync Server and Daemon Orchestration (Archived)

**Goal:** Deploy centralized synchronization infrastructure for multi-agent coordination.

**Success Criteria:**
- Central Dolt SQL server operational
- Workspace daemon with batching logic functional
- Automated pull-merge-commit-push cycle working
- Git hooks export to issues.jsonl
- Multiple clients can sync without conflicts

### User Stories

#### 3.1 Central Dolt SQL Server Deployment
**As a** DevOps engineer  
**I want to** deploy a centralized Dolt SQL server  
**So that** multiple agents can synchronize to a single source of truth

**Acceptance Criteria:**
- `dolt sql-server` deployed on cloud infrastructure
- RemotesAPI endpoint exposed on port 50051
- Server configured with appropriate memory/CPU resources
- Monitoring and health checks implemented
- Backup strategy documented and automated
- Connection pooling configured for 50+ concurrent clients

#### 3.2 Workspace Daemon Engineering
**As a** developer  
**I want to** run a background daemon in each workspace  
**So that** synchronization happens automatically without manual intervention

**Acceptance Criteria:**
- Daemon communicates via Unix domain socket (`.grava/bd.sock`)
- Daemon maintains persistent connection to local Dolt replica
- Process management (start, stop, restart, status) implemented
- Daemon auto-starts on workspace initialization
- Graceful shutdown preserves uncommitted changes
- Daemon logs to `.grava/daemon.log`

#### 3.3 Debouncing and Batching Logic
**As a** developer  
**I want to** batch rapid local mutations before syncing  
**So that** network traffic is minimized and performance is optimized

**Acceptance Criteria:**
- 5-second debounce window after last local write
- Multiple rapid updates batched into single sync operation
- Configurable debounce interval via config file
- Manual sync available via `bd sync --force`
- Batch size limits prevent memory overflow

#### 3.4 Automated Synchronization Protocol
**As an** AI agent  
**I want to** automatic state synchronization with the central server  
**So that** my local view of issues is always up-to-date

**Acceptance Criteria:**
- `dolt pull` retrieves latest commits from server
- Cell-level three-way merge handles concurrent updates
- Conflict resolution strategy applied (newest timestamp wins)
- `dolt commit` creates immutable local snapshot
- `dolt push` sends local commits to server
- Full sync cycle completes in <2 seconds for typical workloads

#### 3.5 Git Hook Integration and JSONL Export
**As a** developer  
**I want to** export database state to human-readable format  
**So that** I can review issues in Git and have fallback recovery

**Acceptance Criteria:**
- Pre-commit hook triggers JSONL export
- `issues.jsonl` contains complete database dump
- Post-merge hook updates local database from JSONL if needed
- Export includes all tables (issues, dependencies, events)
- JSONL format validated and parseable
- Git commit includes both .dolt/ and issues.jsonl changes
