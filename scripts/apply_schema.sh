#!/bin/bash
set -e

DOLT_DIR=".grava/dolt"
SCHEMA_FILE="scripts/schema/001_initial_schema.sql"

# Check required files
if [ ! -d "$DOLT_DIR" ]; then
    echo "Error: Dolt directory $DOLT_DIR not found. Run scripts/init_dolt.sh first."
    exit 1
fi

if [ ! -f "$SCHEMA_FILE" ]; then
    echo "Error: Schema file $SCHEMA_FILE not found."
    exit 1
fi

echo "Applying schemas from scripts/schema to Dolt database..."

(
    cd "$DOLT_DIR"
    for schema in ../../scripts/schema/*.sql; do
        echo "Applying $(basename "$schema")..."
        dolt sql < "$schema"
    done
)

echo "✅ Schema applied successfully."
echo "Verifying tables..."
(
    cd "$DOLT_DIR"
    dolt ls
)
