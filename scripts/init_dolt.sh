#!/bin/bash
set -e

DOLT_DIR=".grava/dolt"

# Resolve dolt binary: prefer local .grava/bin/dolt, fallback to system dolt
DOLT_BIN="${DOLT_BIN:-$([ -x ".grava/bin/dolt" ] && echo ".grava/bin/dolt" || echo "dolt")}"

echo "Checking execution environment..."

# Check dolt is reachable
if ! command -v "$DOLT_BIN" &> /dev/null && [ ! -x "$DOLT_BIN" ]; then
    echo "Error: dolt is not installed. Run 'grava init' first."
    exit 1
fi

# Ensure user identity is configured for Dolt
if ! "$DOLT_BIN" config --list | grep -q "user.email"; then
    echo "Configuring Dolt global user from Git configuration..."
    if [ -n "$(git config user.email)" ]; then
        "$DOLT_BIN" config --global --add user.email "$(git config user.email)"
        "$DOLT_BIN" config --global --add user.name "$(git config user.name)"
        echo "Dolt user configured: $(git config user.name) <$(git config user.email)>"
    else
        echo "Error: Git user.email not found. Please run:"
        echo "  $DOLT_BIN config --global --add user.email \"your@email.com\""
        echo "  $DOLT_BIN config --global --add user.name \"Your Name\""
        exit 1
    fi
fi

echo "Initializing Dolt database in $DOLT_DIR..."
PROJECT_ROOT=$(git rev-parse --show-toplevel)
cd "$PROJECT_ROOT"

mkdir -p "$DOLT_DIR"
cd "$DOLT_DIR"

if [ ! -d ".dolt" ]; then
    "$DOLT_BIN" init
    echo "✅ Dolt database initialized successfully."
else
    echo "ℹ️  Dolt database already initialized."
fi

# Verify status
echo "Verifying database status..."
"$DOLT_BIN" status
