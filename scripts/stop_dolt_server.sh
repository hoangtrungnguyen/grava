#!/bin/bash
set -e

PORT=3306
NO_CONFIRM=false

while [[ "$#" -gt 0 ]]; do
    case $1 in
        -y|--yes) NO_CONFIRM=true ;;
        *) PORT=$1 ;;
    esac
    shift
done

# Find PID of process listening on port (or dolt sql-server specifically)
# Using lsof to find the PID listening on the port
PID=$(lsof -t -i:$PORT)

if [ -z "$PID" ]; then
    echo "No process found listening on port $PORT."
    exit 0
fi

# Check if it's actually dolt (optional but good practice)
PROCESS_NAME=$(ps -p $PID -o comm=)
if [[ "$PROCESS_NAME" != *"dolt"* ]]; then
    if [ "$NO_CONFIRM" = true ]; then
        echo "Warning: Process on port $PORT ($PROCESS_NAME, PID $PID) does not appear to be Dolt. Forcing stop due to -y/--yes."
    else
        echo "Warning: Process on port $PORT ($PROCESS_NAME, PID $PID) does not appear to be Dolt."
        read -p "Are you sure you want to kill it? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    fi
fi

echo "Stopping Dolt SQL Server (PID $PID)..."
kill $PID

# Wait for it to exit
while kill -0 $PID 2>/dev/null; do
    sleep 0.5
done

echo "✅ Dolt SQL Server stopped."
