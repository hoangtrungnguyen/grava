#!/bin/bash
set -e

# Configuration
TEST_DB_NAME="test_grava"
PORT=3306
HOST="127.0.0.1"
MYSQL_CLIENT="/opt/homebrew/opt/mysql-client/bin/mysql"

if [ ! -f "$MYSQL_CLIENT" ]; then
    # Fallback to system mysql if available
    if command -v mysql &> /dev/null; then
        MYSQL_CLIENT=$(command -v mysql)
    else
        echo "❌ 'mysql' client not found. Please ensure mysql-client is installed and in PATH."
        exit 1
    fi
fi

echo "🛠️  Setting up Test Environment..."

# 1. Check if Dolt is running
if ! lsof -i :$PORT > /dev/null; then
    echo "⚠️  Dolt server not running on port $PORT. Please start it using scripts/start_dolt_server.sh"
    exit 1
fi

echo "✅ Dolt server detected."

# 2. Create Test Database
echo "📦 Creating/Resetting test database '$TEST_DB_NAME'..."

"$MYSQL_CLIENT" -h "$HOST" -P "$PORT" -u root -e "DROP DATABASE IF EXISTS $TEST_DB_NAME; CREATE DATABASE $TEST_DB_NAME;"

echo "✅ Database '$TEST_DB_NAME' created."

# 3. Apply Schema
echo "📜 Applying schemas to '$TEST_DB_NAME'..."

for schema in scripts/schema/*.sql; do
    echo "Applying $(basename "$schema")..."
    "$MYSQL_CLIENT" -h "$HOST" -P "$PORT" -u root -D "$TEST_DB_NAME" < "$schema"
done

echo "✅ All schemas applied successfully."

# 4. Create Environment File for Testing
APP_ENV_FILE=".env.test"
echo "Creating $APP_ENV_FILE..."

echo "DB_URL=root@tcp($HOST:$PORT)/$TEST_DB_NAME?parseTime=true" > "$APP_ENV_FILE"

echo "✅ Test environment configured in $APP_ENV_FILE"
echo "🎉 Ready for testing!"
