#!/bin/bash
# =============================================================================
# Grava-Aware Qwen Assistant
#
# A convenience script to interact with the local Qwen model using the context
# of the current Grava repository.
# =============================================================================

set -e

# Configuration
MODEL="grava-coder"
PROJECT_ROOT="/root/grava"

# 1. Initialize the specialized model if it doesn't exist
if ! ollama list | grep -q "$MODEL"; then
    echo "[INFO] Creating specialized '$MODEL' model..."
    ollama create "$MODEL" -f "$PROJECT_ROOT/Modelfile-grava"
fi

usage() {
    echo "Usage: ./scripts/qwen.sh [command] [args]"
    echo ""
    echo "Commands:"
    echo "  chat             - Full interactive chat with project context"
    echo "  ask \"prompt\"     - Simple one-off question"
    echo "  explain <file>   - Feed a specific file to the model and ask for an explanation"
    echo "  scan <dir>       - Feed all files in a directory to the model (use with caution)"
    echo "  status           - Check status of Ollama and model memory"
    echo ""
}

case "${1:-}" in
    chat)
        ollama run "$MODEL"
        ;;
    ask)
        shift
        ollama run "$MODEL" "$*"
        ;;
    explain)
        shift
        if [ -f "$1" ]; then
            echo "[INFO] Explaining file: $1"
            cat "$1" | ollama run "$MODEL" "Review and explain this file within the context of the Grava architecture:"
        else
            echo "[ERROR] File not found: $1"
        fi
        ;;
    scan)
        shift
        if [ -d "$1" ]; then
            echo "[INFO] Scanning directory: $1"
            FILES=$(find "$1" -maxdepth 1 -type f -not -path '*/.*')
            for f in $FILES; do
                echo "--- Processing: $f ---"
                cat "$f"
            done | ollama run "$MODEL" "Review these files from the $1 directory. Identify potential architectural improvements:"
        else
            echo "[ERROR] Directory not found: $1"
        fi
        ;;
    status)
        ollama list
        ollama ps
        ;;
    *)
        usage
        ;;
esac
