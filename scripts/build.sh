#!/bin/bash
set -e

echo "🔨 Building Grava..."

# Ensure we are in the root of the project
# (assuming the script is run from project root or via its path)
go build -o grava ./cmd/grava

echo "✅ Grava build successful. Binary created at: ./grava"
