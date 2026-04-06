# Story 3-4 Quick Start Guide

**Epic 3 Sandbox Integration Tests** — Complete Workflow for Local Test Execution

---

## 🚀 Quick Start (5 Minutes)

### 1. Verify Everything is Ready
```bash
./scripts/verify-story-3-4-workflow.sh
```

This checks:
- ✅ Go is installed
- ✅ Dolt is installed
- ✅ Project files exist
- ✅ Story documentation is ready
- ✅ Test scripts are executable

### 2. Verify Your Environment
```bash
./scripts/verify-environment.sh
```

This confirms:
- ✅ All dependencies installed
- ✅ Dolt server connectivity
- ✅ Database access

### 3. Run Tests
```bash
# Start Dolt (if not already running)
dolt --data-dir .grava/dolt sql-server &

# Run all tests (in another terminal)
./scripts/run-integration-tests.sh --full --verbose
```

**Expected Result:** ✅ All tests pass → Story ready for code review

---

## 📋 Complete Setup & Testing

### **Option A: Complete Setup From Scratch**

#### Step 1: Install Go
```bash
# macOS
brew install go

# Linux (Ubuntu)
sudo apt-get install golang-go

# Verify
go version  # Should be 1.21+
```

#### Step 2: Install Dolt
```bash
# macOS
brew install dolt

# Linux (Ubuntu)
sudo apt-get install dolt

# Verify
dolt version  # Should be 1.32+
```

#### Step 3: Prepare Project Dependencies
```bash
cd /Users/trungnguyenhoang/IdeaProjects/grava
go mod tidy
```

#### Step 4: Verify Environment
```bash
./scripts/verify-environment.sh
```

#### Step 5: Run Tests
```bash
# Terminal 1: Start Dolt server
dolt --data-dir .grava/dolt sql-server

# Terminal 2: Run tests
./scripts/run-integration-tests.sh --full --verbose
```

---

## 📚 Documentation Files

All documents are in the project root:

1. **SETUP-LOCAL-ENVIRONMENT.md** — Complete setup guide for Go + Dolt
2. **TEST-EXECUTION-CHECKLIST.md** — Step-by-step test execution checklist
3. **3-4-TEST-EXECUTION-PLAN.md** — Comprehensive test plan

---

## 🔧 Available Scripts

### `./scripts/verify-story-3-4-workflow.sh`
Complete workflow verification

```bash
./scripts/verify-story-3-4-workflow.sh
```

---

### `./scripts/verify-environment.sh`
Environment readiness check

```bash
./scripts/verify-environment.sh
```

---

### `./scripts/run-integration-tests.sh`
Main test runner

```bash
./scripts/run-integration-tests.sh --full --verbose
```

---

## ✅ What You'll Test

| AC | What Gets Tested |
|---|---|
| **AC#1** | Concurrent claims (exactly one succeeds) |
| **AC#2** | Agent crash + resume via Wisp |
| **AC#3** | Full Epic 3 lifecycle |
| **AC#4** | Reporting infrastructure |
| **AC#5** | No regressions |

---

## 🎯 Next Steps

```bash
# Step 1: Verify workflow
./scripts/verify-story-3-4-workflow.sh

# Step 2: Verify environment
./scripts/verify-environment.sh

# Step 3: Start Dolt (terminal 1)
dolt --data-dir .grava/dolt sql-server &

# Step 4: Run tests (terminal 2)
./scripts/run-integration-tests.sh --full --verbose
```

**Result:** ✅ All tests pass → Story 3-4 ready for code review!

---

**Ready?** Start with: `./scripts/verify-story-3-4-workflow.sh` 🚀
