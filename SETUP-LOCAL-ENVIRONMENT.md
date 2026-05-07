# Local Environment Setup Guide for Story 3-4 Integration Tests

**Goal:** Get Go + Dolt running locally so you can execute integration tests for Epic 3 Sandbox Integration Tests

---

## Part 1: Go Installation

### macOS (Recommended)

#### Option A: Homebrew (Easiest)
```bash
# Install Go via Homebrew
brew install go

# Verify installation
go version
# Expected: go version go1.21.x darwin/arm64 (or amd64)
```

#### Option B: Download from golang.org
```bash
# Download from https://go.dev/dl/
# Choose: macOS (Apple Silicon or Intel)
# Run installer and follow prompts

# Verify installation
go version
```

#### Option C: Using asdf (if you use version manager)
```bash
asdf plugin add golang https://github.com/asdf-vm/asdf-golang.git
asdf install golang 1.21.0
asdf global golang 1.21.0
go version
```

---

### Linux (Ubuntu/Debian)

```bash
# Download Go 1.21+
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz

# Remove old Go if installed
sudo rm -rf /usr/local/go

# Extract to /usr/local
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz

# Add to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Verify
go version
```

---

### Windows (PowerShell)

```powershell
# Download installer from https://go.dev/dl/
# Run: go1.21.0.windows-amd64.msi
# Follow installer prompts

# Verify in PowerShell
go version
```

---

## Part 2: Dolt Installation

### macOS

#### Option A: Homebrew
```bash
brew install dolt

# Verify
dolt version
# Expected: Dolt version 1.32.x or higher
```

#### Option B: Download Binary
```bash
# Download from https://github.com/dolthub/dolt/releases
# Download: dolt-darwin-amd64.zip (or arm64 for Apple Silicon)

# Extract and move to PATH
unzip dolt-darwin-amd64.zip
sudo mv dolt /usr/local/bin/

# Verify
dolt version
```

---

### Linux (Ubuntu/Debian)

```bash
# Add Dolt repository
sudo curl -L https://releases.dolt.dev/repo.public.key | sudo apt-key add -
echo 'deb [trusted=yes] https://releases.dolt.dev/deb releases' | sudo tee /etc/apt/sources.list.d/dolt-releases.list

# Install Dolt
sudo apt-get update
sudo apt-get install -y dolt

# Verify
dolt version
```

---

### Windows (PowerShell)

```powershell
# Using Chocolatey (if installed)
choco install dolt

# OR download from: https://github.com/dolthub/dolt/releases
# Download: dolt-windows-amd64.zip
# Extract and add to PATH

# Verify
dolt version
```

---

## Part 3: Project Dependencies

### Initialize Go Modules

```bash
# Navigate to project root
cd /Users/trungnguyenhoang/IdeaProjects/grava

# Tidy dependencies (downloads what's needed)
go mod tidy

# Verify key dependencies are present
go mod graph | grep -E "testify|sqlmock|mysql"
# Should show: testify, sqlmock, mysql driver
```

---

## Part 4: Dolt Database Setup

### Start Dolt Server

```bash
# Navigate to project root
cd /Users/trungnguyenhoang/IdeaProjects/grava

# Start Dolt server in background
dolt --data-dir .grava/dolt sql-server &

# Wait 2-3 seconds for startup
sleep 3

# Verify connection
dolt sql "SELECT 1;"
# Expected: ✅ returns 1
```

### Or Run Dolt in Interactive Mode (Terminal 1)

```bash
# Terminal 1 - Start Dolt server
cd /Users/trungnguyenhoang/IdeaProjects/grava
dolt --data-dir .grava/dolt sql-server

# Should output:
# Starting Dolt server...
# Server running on: 127.0.0.1:3306
```

---

### Test Dolt Connection (Terminal 2)

```bash
# Terminal 2 - Verify it works
dolt --data-dir .grava/dolt sql "SELECT database();"

# Or via mysql client if installed
mysql -h 127.0.0.1 -P 3306 -u root -e "SELECT VERSION();"

# Check grava database exists
dolt --data-dir .grava/dolt sql "SHOW DATABASES;"
```

---

## Part 5: Verify Complete Setup

### Quick Verification Script

```bash
#!/bin/bash

echo "🔍 Verifying Local Environment Setup..."
echo ""

# Check Go
echo "1️⃣  Checking Go..."
if command -v go &> /dev/null; then
    go version
    echo "✅ Go installed"
else
    echo "❌ Go NOT found - run setup above"
    exit 1
fi

# Check Dolt
echo ""
echo "2️⃣  Checking Dolt..."
if command -v dolt &> /dev/null; then
    dolt version
    echo "✅ Dolt installed"
else
    echo "❌ Dolt NOT found - run setup above"
    exit 1
fi

# Check Go modules
echo ""
echo "3️⃣  Checking Go dependencies..."
if [ -f "go.mod" ]; then
    echo "✅ go.mod found"
    go mod verify
    echo "✅ Modules valid"
else
    echo "❌ go.mod NOT found"
    exit 1
fi

# Check Dolt database
echo ""
echo "4️⃣  Checking Dolt database..."
if [ -d ".grava/dolt" ]; then
    echo "✅ .grava/dolt directory found"
else
    echo "❌ .grava/dolt directory NOT found"
    exit 1
fi

# Try to connect to Dolt
echo ""
echo "5️⃣  Testing Dolt connection..."
if timeout 5 dolt sql "SELECT 1;" > /dev/null 2>&1; then
    echo "✅ Dolt server responding"
else
    echo "⚠️  Dolt server NOT responding - start it with: dolt --data-dir .grava/dolt sql-server"
fi

echo ""
echo "✅ Setup verification complete!"
echo ""
echo "🚀 Ready to run tests!"
```

Save this as `scripts/verify-setup.sh` and run:
```bash
chmod +x scripts/verify-setup.sh
./scripts/verify-setup.sh
```

---

## Part 6: Environment Variables (Optional)

### Override Dolt Connection

Create `.env.test` or set environment variables:

```bash
# Default: root@tcp(127.0.0.1:3306)/?parseTime=true
# Override if your Dolt runs elsewhere:

export GRAVA_TEST_DSN="root@tcp(127.0.0.1:3311)/grava?parseTime=true"

# Verify
echo $GRAVA_TEST_DSN
```

---

## Troubleshooting

### "command not found: go"
```bash
# Add Go to PATH
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### "Dolt server connection refused"
```bash
# Make sure Dolt is running
ps aux | grep dolt

# If not running, start it:
dolt --data-dir .grava/dolt sql-server
```

### "GRAVA_TEST_DSN: connection refused"
```bash
# Check what port Dolt is on
netstat -tuln | grep 3306

# Verify .grava/dolt directory has data
ls -la .grava/dolt/
```

### "module not found: testify"
```bash
# Download dependencies
go mod tidy
go mod download
```

---

## Verification Checklist

After setup, verify each item:

- [ ] `go version` returns v1.21 or higher
- [ ] `dolt version` returns v1.32 or higher
- [ ] `go mod tidy` completes without errors
- [ ] `dolt sql "SELECT 1;"` returns success
- [ ] `.grava/dolt` directory exists with data
- [ ] `./scripts/verify-setup.sh` passes all checks

---

## Next Steps

Once setup is complete:
1. ✅ Run `scripts/verify-setup.sh` to confirm everything works
2. ✅ Follow **TEST-EXECUTION-CHECKLIST.md** for test execution
3. ✅ Run `scripts/run-integration-tests.sh` to execute tests
4. ✅ Check `scripts/generate-test-report.sh` for results

---

**Setup Complete!** 🎉

You're ready to run the integration tests for Story 3-4.
