#!/bin/bash
set -e

echo "🧪 Running full test suite for Grava..."
echo "---------------------------------------"

# 1. Ensure Test Environment is Ready
# We run the setup script to ensure fresh DB state (re-create test_grava).
echo "🔄 Setting up test database..."
./scripts/setup_test_env.sh
echo "---------------------------------------"

# 2. Run Go Tests
echo "🏃 Running Go Tests (Unit & Integration)..."
# We inject the DB_URL explicitly to be sure, though the test code defaults to it.
# -v: Verbose output
# -cover: Show code coverage
# ./...: Run all tests in all subdirectories including cmd, dolt, idgen.

export DB_URL="root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"
go test -v -cover ./...

echo "---------------------------------------"
echo "✅ All tests passed successfully!"
