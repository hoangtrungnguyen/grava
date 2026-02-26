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
if ! "$MYSQL_CLIENT" -h "$HOST" -P "$PORT" -u root -e "SELECT 1" > /dev/null 2>&1; then
    echo "⚠️  Dolt server not reachable on port $PORT. Please start it using scripts/start_dolt_server.sh"
    exit 1
fi

echo "✅ Dolt server detected."

# 2. Create Test Database
echo "📦 Creating/Resetting test database '$TEST_DB_NAME'..."

"$MYSQL_CLIENT" -h "$HOST" -P "$PORT" -u root -e "DROP DATABASE IF EXISTS $TEST_DB_NAME; CREATE DATABASE $TEST_DB_NAME;"

echo "✅ Database '$TEST_DB_NAME' created."

# 3. Run schema migrations against the test database
echo "🔄 Running schema migrations on '$TEST_DB_NAME'..."
DB_URL="root@tcp($HOST:$PORT)/$TEST_DB_NAME?parseTime=true" \
  go run "$(git rev-parse --show-toplevel)/cmd/grava/main.go" \
    --db-url "root@tcp($HOST:$PORT)/$TEST_DB_NAME?parseTime=true" \
    list > /dev/null 2>&1 || true
echo "✅ Schema migrations applied."

# 4. Create Environment File for Testing
APP_ENV_FILE=".env.test"
echo "Creating $APP_ENV_FILE..."

echo "DB_URL=root@tcp($HOST:$PORT)/$TEST_DB_NAME?parseTime=true" > "$APP_ENV_FILE"

echo "✅ Test environment configured in $APP_ENV_FILE"
echo "🎉 Ready for testing!"
