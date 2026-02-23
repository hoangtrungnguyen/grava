#!/bin/bash
set -e

echo "🔨 Building Grava..."

VERSION=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}

go build -ldflags "-X main.Version=$VERSION" -o grava ./cmd/grava

echo "✅ Grava build successful (version: $VERSION). Binary created at: ./grava"
