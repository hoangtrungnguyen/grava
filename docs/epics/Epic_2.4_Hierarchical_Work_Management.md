# Epic 2.4: Hierarchical Work Management

**Goal:** Implement formal support for hierarchical task structures (Epic -> Task -> Subtask) with automated status propagation ("Active Linkage") to ensure project state consistency.

**Success Criteria:**
- Dot-notation ID convention fully integrated into CRUD operations.
- `grava show --tree <id>` visualizes the hierarchical tree with progress indicators.
- Automatic status bubbling: Closing all child nodes automatically closes the parent node.
- Data integrity: Preventing cycles in hierarchy and ensuring parent nodes exist before children.

---

## 1. User Stories

### 2.4.1 Hierarchical ID Convention
**As a** developer 
**I want to** use dot-notation for related tasks (e.g., `grava-a1b2.1`)
**So that** I can intuitively understand the organizational relationship of work items without querying the graph.

**Acceptance Criteria:**
- `idgen` package generates child IDs by appending `.N` to parent IDs.
- CLI supports creating subtasks via `grava create task --parent <id>`.
- The `issues` table `id` field enforces the convention for newly created subtasks.

### 2.4.2 Active Linkage: Automated Status Propagation
**As an** AI agent
**I want** parent tasks to automatically close when their subtasks are finished
**So that** I don't have to spend management overhead updating "container" tasks manually.

**Acceptance Criteria:**
- When an issue status is set to `closed`, the system triggers an `CheckParentStatus` event.
- If all siblings of the closed issue are also `closed`, the parent issue status is automatically updated to `closed`.
- This logic recurses up the tree (Subtask -> Task -> Epic).
- Automated transitions are recorded in the `events` table with actor `system`.

### 2.4.3 Tree Visualization (`grava show --tree`)
**As a** user
**I want to** view my project structure as a tree
**So that** I can see the completion progress of a large Epic at a glance.

**Acceptance Criteria:**
- `grava show --tree <id>` command implemented.
- Uses BFS/DFS to traverse `subtask-of` (or `parent-child`) edges.
- Displays nested structure with indentation and color-coded status.
- Includes a progress bar or percentage per parent node (e.g., `[███░░] 60%`).

---

## 2. Technical Implementation Details

### 2.1 Parent-Child Identification
The system will rely on both ID naming conventions and explicit graph edges:
- **ID Check:** `strings.Contains(id, ".")`
- **Edge Check:** `type = 'subtask-of'` in the `dependencies` table.

### 2.2 Status Bubbling Algorithm
When `SetNodeStatus(childID, StatusClosed)` is called:
1. Find `parentID` via `subtask-of` edge.
2. If `parentID` exists:
   - Query all children of `parentID`.
   - If `allChildren.Status == StatusClosed`:
     - Recursively call `SetNodeStatus(parentID, StatusClosed)`.
   - Else:
     - Update parent status to `in_progress` if it was `open`.

---

## 3. Verification Plan

- **Unit Tests:** Verify `idgen` correctly increments child suffixes and doesn't collide.
- **Graph Tests:** Test the recursive status update in `pkg/graph/dag.go`.
- **CLI Tests:** Verify the tree output matches the actual graph state for a 3-level hierarchy (Epic-Task-Subtask).
