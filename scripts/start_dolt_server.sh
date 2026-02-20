#!/bin/bash
set -e

DOLT_DIR=".grava/dolt"
PORT=${1:-3306}

# Check dolt installation
if ! command -v dolt &> /dev/null; then
    echo "Error: dolt is not installed."
    exit 1
fi

if [ ! -d "$DOLT_DIR" ]; then
    echo "Error: Dolt directory $DOLT_DIR not found. Run scripts/init_dolt.sh first."
    exit 1
fi

echo "Starting Dolt SQL Server on port $PORT..."
echo "Connection String: mysql://root@127.0.0.1:$PORT/dolt"

cd "$DOLT_DIR"
# Start server (Dolt SQL Server uses the current repository users).
dolt sql-server --port=$PORT --host=0.0.0.0 --loglevel=info
