# Multi-Agent Workflow Test Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Verify that two AI agents operating in two isolated clones of the same Grava workspace can concurrently work, push/pull changes, and successfully leverage the Git Merge Driver to resolve any data conflicts.

**Architecture:** Create two separate Git clones (`agent1_workspace` and `agent2_workspace`) linked to a common designated remote `central.git`. Use a bash script to simulate an end-to-end task breakdown where both agents concurrently update different fields of the same task, and then pull/push to trigger the custom Grava merge driver.

**Tech Stack:** Git, Grava CLI, Bash scripting, jq.

---

### Task 1: Setup Remote and Agent Workspaces

**Files:**
- Create: `scripts/test_two_agents.sh`

**Step 1: Write the setup portion of the test script**

```bash
#!/bin/bash
set -e

echo "Setting up centralized remote..."
rm -rf /tmp/grava-central.git /tmp/agent1_workspace /tmp/agent2_workspace
mkdir -p /tmp/grava-central.git
cd /tmp/grava-central.git
git init --bare

echo "Cloning Agent 1 and Agent 2 workspaces..."
cd /tmp
git clone /tmp/grava-central.git agent1_workspace
git clone /tmp/grava-central.git agent2_workspace
```

**Step 2: Run test to verify it works**

Run: `bash scripts/test_two_agents.sh`
Expected: PASS (directories created and cloned successfully)

**Step 3: Commit**

```bash
git add scripts/test_two_agents.sh
git commit -m "test: add agent workspace setup script"
```

### Task 2: Agent 1 Initialization & Epic Creation

**Files:**
- Modify: `scripts/test_two_agents.sh`

**Step 1: Append Agent 1 behavior sequence**

```bash
echo "Agent 1 initializes Grava and creates an Epic..."
cd /tmp/agent1_workspace
grava init --db-url "root@tcp(127.0.0.1:3307)/dolt" # Isolated Dolt instance port
grava start
sleep 2

grava create "Build the Death Star" --type epic --desc "A moon-sized space station"
git add .
git commit -m "feat: agent 1 created epic"
git push origin main || git push --set-upstream origin main
grava stop
```

**Step 2: Run test to verify it passes**

Run: `bash scripts/test_two_agents.sh`
Expected: PASS (Agent 1 pushes epic to bare repo)

**Step 3: Commit**

```bash
git add scripts/test_two_agents.sh
git commit -m "test: simulate Agent 1 epic creation"
```

### Task 3: Agent 2 Pulls and Creates Subtasks

**Files:**
- Modify: `scripts/test_two_agents.sh`

**Step 1: Append Agent 2 behavior sequence**

```bash
echo "Agent 2 pulls the Epic and creates Subtasks..."
cd /tmp/agent2_workspace
git pull origin main
grava init --db-url "root@tcp(127.0.0.1:3308)/dolt" # Port 3308 for agent 2
grava start
sleep 2

# Get Epic ID dynamically
EPIC_ID=$(grava list --json | jq -r '.[0].id')
grava subtask $EPIC_ID --title "Thermal Exhaust Port Design" --desc "A small 2-meter wide vulnerability"
grava subtask $EPIC_ID --title "Kyber Crystal Procurement" --desc "Find crystals for superlaser"

git add .
git commit -m "feat: agent 2 added subtasks"
git push origin main
grava stop
```

**Step 2: Run test to verify it passes**

Run: `bash scripts/test_two_agents.sh`
Expected: PASS (Agent 2 pushes subtasks to remote)

**Step 3: Commit**

```bash
git add scripts/test_two_agents.sh
git commit -m "test: simulate Agent 2 subtask creation"
```

### Task 4: Concurrent Updates & Trigger Merge Driver

**Files:**
- Modify: `scripts/test_two_agents.sh`

**Step 1: Append concurrent updates and merge sequence**

```bash
echo "Agent 1 pulls updates and modifies STATUS..."
cd /tmp/agent1_workspace
git pull origin main
grava start
sleep 2
SUBTASK_ID=$(grava list --json | jq -r '.[1].id') # Grab the Thermal exhaust port task
grava update $SUBTASK_ID --status in_progress
git add .
git commit -m "feat: agent 1 started exhaustion port"
grava stop

echo "Agent 2 concurrently modifies DESCRIPTION..."
cd /tmp/agent2_workspace
grava start
sleep 2
# Assume Agent 2 knows the same ID
grava update $SUBTASK_ID --desc "A 2-meter ray-shielded vulnerability"
git add .
git commit -m "fix: agent 2 clarified exhaust doc"
grava stop

echo "Agent 1 pushes first..."
cd /tmp/agent1_workspace
git push origin main

echo "Agent 2 pulls (triggering merge driver) and pushes..."
cd /tmp/agent2_workspace
# This will trigger 'grava merge-slot' under the hood if gitattributes are set up right
git config pull.rebase false
git pull origin main

echo "Merge completed! Re-starting DB to verify data..."
grava start
sleep 2
grava show $SUBTASK_ID
grava stop

git push origin main
echo "SUCCESS!"
```

**Step 2: Run test to verify it passes**

Run: `bash scripts/test_two_agents.sh`
Expected: PASS (Merge driver successfully resolves concurrent column edits to same Subtask ID, Agent 2 pushes successfully, output shows both `in_progress` AND updated description)

**Step 3: Commit**

```bash
git add scripts/test_two_agents.sh
git commit -m "test: verify merge driver via simulated agents"
```
