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

echo "Applying schema $SCHEMA_FILE to Dolt database..."

# Go to Dolt directory to run commands contextually
# We use a subshell or pushd/popd to not affect current shell (though this is a script so it's fine)
(
    cd "$DOLT_DIR"
    # Execute SQL file
    # We need to resolve the absolute path or relative path from $DOLT_DIR to $SCHEMA_FILE
    # Since we are in .grava/dolt, the schema file is at ../../scripts/schema/001_initial_schema.sql
    dolt sql < "../../$SCHEMA_FILE"
)

echo "✅ Schema applied successfully."
echo "Verifying tables..."
(
    cd "$DOLT_DIR"
    dolt ls
)
