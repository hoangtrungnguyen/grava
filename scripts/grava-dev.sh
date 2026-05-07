#!/bin/bash

# grava-dev.sh: Facilitate an isolated Claude development session for a Grava issue.
# Usage: ./scripts/grava-dev.sh <issue-id>

ISSUE_ID=$1

if [ -z "$ISSUE_ID" ]; then
    echo "Usage: $0 <issue-id>"
    exit 1
fi

echo "--- [1/2] Claiming Issue $ISSUE_ID ---"
# Attempt to claim the issue. Use --json to parse results if needed in future.
CLAIM_OUTPUT=$(grava claim "$ISSUE_ID")
CLAIM_STATUS=$?

if [ $CLAIM_STATUS -ne 0 ]; then
    echo "❌ Failed to claim issue. It might be already assigned or doesn't exist."
    echo "$CLAIM_OUTPUT"
    exit 1
fi

echo "✅ Issue claimed successfully."

echo "--- [2/2] Launching Isolated Claude Session ---"
echo "Creating Git worktree for $ISSUE_ID..."

# Launch Claude with the worktree flag. 
# Claude handles the branch creation and directory management.
claude --worktree "$ISSUE_ID"

echo ""
echo "--- Session Ended ---"
echo "Don't forget to finalize the status if work is complete:"
echo "  grava close $ISSUE_ID"
echo "Or pause if you need to resume later:"
echo "  grava stop $ISSUE_ID"
