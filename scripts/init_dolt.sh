#!/bin/bash
set -e

DOLT_DIR=".grava/dolt"

echo "Checking execution environment..."

# Check dolt installation
if ! command -v dolt &> /dev/null; then
    echo "Error: dolt is not installed. Please install it first."
    exit 1
fi

# Ensure user identity is configured for Dolt
# This checks global config but sets local fallback if missing
if ! dolt config --list | grep -q "user.email"; then
    echo "Configuring Dolt global user from Git configuration..."
    if [ -n "$(git config user.email)" ]; then
        dolt config --global --add user.email "$(git config user.email)"
        dolt config --global --add user.name "$(git config user.name)"
        echo "Dolt user configured: $(git config user.name) <$(git config user.email)>"
    else
        echo "Error: Git user.email not found. Please run:"
        echo "  dolt config --global --add user.email \"your@email.com\""
        echo "  dolt config --global --add user.name \"Your Name\""
        exit 1
    fi
fi

echo "Initializing Dolt database in $DOLT_DIR..."
# Ensure we are relative to project root regardless of where script is run
PROJECT_ROOT=$(git rev-parse --show-toplevel)
cd "$PROJECT_ROOT"

mkdir -p "$DOLT_DIR"
cd "$DOLT_DIR"

if [ ! -d ".dolt" ]; then
    dolt init
    echo "✅ Dolt database initialized successfully."
else
    echo "ℹ️  Dolt database already initialized."
fi

# Verify status
echo "Verifying database status..."
dolt status
