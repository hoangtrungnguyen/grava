#!/bin/bash
set -e

# Initialize Grava if the configuration doesn't exist
# We check if .grava folder or .grava.yaml is present in the workspace
if [ ! -f .grava.yaml ]; then
    echo "========================================="
    echo "Welcome to the Grava Sandbox Environment!"
    echo "========================================="
    echo "Initializing Grava..."
    
    # Configure dolt global settings if they don't exist
    if ! dolt config --list | grep -q 'user.name'; then
        dolt config --global --add user.email "sandbox@example.com"
        dolt config --global --add user.name "Grava Sandbox User"
    fi

    # Initialize grava directory
    grava init || echo "Warning: Grava initialization encountered an issue. You can still run 'grava init' manually."
    
    echo "Starting Grava background server..."
    grava start

    echo "Grava is ready to use! Enjoy exploring."
    echo ""
fi

# Execute the specified CMD
exec "$@"
