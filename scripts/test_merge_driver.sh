#!/bin/bash
set -e

echo "Building grava for integration tests..."
go build -o grava cmd/grava/main.go

TEST_DIR=$(mktemp -d)
echo "Created test repository at $TEST_DIR"
cd "$TEST_DIR"
git config --global init.defaultBranch main || true
export PATH="$OLDPWD:$PATH" # Put built 'grava' in PATH

echo "Initializing Git repo & Grava Merge Driver..."
git init
git config user.name "Test"
git config user.email "test@example.com"
grava install

echo "Removing hooks for integration test to prevent host DB side-effects..."
rm -rf .git/hooks/*

echo "Setting up initial issue data..."
cat <<EOF > issues.jsonl
{"id":"1","title":"Initial Title","status":"open"}
EOF
git add issues.jsonl .gitattributes
git commit -m "Initial commit"

echo "Creating branch A (Title change)..."
git checkout -b branch-a
cat <<EOF > issues.jsonl
{"id":"1","title":"Title from A","status":"open"}
EOF
git add issues.jsonl
git commit -m "Update title in branch A"

echo "Creating branch B (Status change)..."
git checkout main
git checkout -b branch-b
cat <<EOF > issues.jsonl
{"id":"1","title":"Initial Title","status":"closed"}
EOF
git add issues.jsonl
git commit -m "Update status in branch B"

echo "Merging branch A into Branch B..."
if git merge --no-edit branch-a; then
    echo "Merge succeeded!"
else
    echo "MERGE FAILED: Expected auto-merge to succeed!"
    exit 1
fi

echo "Verifying merged output..."
MERGED_JSON=$(cat issues.jsonl)

if [[ ! "$MERGED_JSON" == *"\"title\":\"Title from A\""* ]]; then
    echo "ERROR: Missing 'Title from A'"
    exit 1
fi

if [[ ! "$MERGED_JSON" == *"\"status\":\"closed\""* ]]; then
    echo "ERROR: Missing 'closed' status"
    exit 1
fi

echo "✅ ALL INTEGRATION TESTS PASSED: Auto-merge behaves correctly."
cd - > /dev/null
rm -rf "$TEST_DIR"
