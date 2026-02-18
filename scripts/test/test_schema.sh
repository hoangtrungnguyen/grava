#!/bin/bash

DOLT_DIR=".grava/dolt"

cd "$DOLT_DIR" || exit 1

echo "Testing valid insert..."
if dolt sql -q "INSERT INTO issues (id, title, status, priority) VALUES ('grava-test-1', 'Valid Issue', 'open', 1);"; then
    echo "✅ Valid insert successful"
else
    echo "❌ Valid insert failed"
    exit 1
fi

echo "Testing invalid status..."
if dolt sql -q "INSERT INTO issues (id, title, status) VALUES ('grava-test-2', 'Invalid Status', 'invalid');" 2>/dev/null; then
    echo "❌ Invalid status check failed (should have been rejected)"
    exit 1
else
    echo "✅ Invalid status correctly rejected"
fi

echo "Testing invalid priority..."
if dolt sql -q "INSERT INTO issues (id, title, priority) VALUES ('grava-test-3', 'Invalid Priority', 5);" 2>/dev/null; then
    echo "❌ Invalid priority check failed (should have been rejected)"
    exit 1
else
    echo "✅ Invalid priority correctly rejected"
fi

echo "Cleaning up..."
dolt sql -q "DELETE FROM issues WHERE id LIKE 'grava-test%';"

echo "Schema validation passed!"
