#!/bin/bash
set -e

DOLT_DIR=".grava/dolt"
PORT=${1:-3306}

# Resolve dolt binary: prefer local .grava/bin/dolt, fallback to system dolt
DOLT_BIN="${DOLT_BIN:-$([ -x ".grava/bin/dolt" ] && echo ".grava/bin/dolt" || echo "dolt")}"

# Check dolt is reachable
if ! command -v "$DOLT_BIN" &> /dev/null && [ ! -x "$DOLT_BIN" ]; then
    echo "Error: dolt not found at '$DOLT_BIN'. Run 'grava init' first."
    exit 1
fi

if [ ! -d "$DOLT_DIR" ]; then
    echo "Error: Dolt directory $DOLT_DIR not found. Run 'grava init' first."
    exit 1
fi

echo "Starting Dolt SQL Server on port $PORT..."
echo "Connection String: mysql://root@127.0.0.1:$PORT/dolt"

cd "$DOLT_DIR"
"$DOLT_BIN" sql-server --port=$PORT --host=0.0.0.0 --loglevel=info
