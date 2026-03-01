#!/bin/bash
set -e

# Configuration
APP_NAME="grava"
DIST_DIR="dist"
VERSION=${1:-"dev"}
ENTRYPOINT="./cmd/grava/main.go"

echo "🛠️  Building $APP_NAME version $VERSION..."

# Clean dist directory
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# Ensure we're in the project root
if [ ! -f "go.mod" ]; then
    echo "❌ Please run this script from the project root."
    exit 1
fi

# Define targets: OS/ARCH
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for target in "${TARGETS[@]}"; do
    IFS='/' read -r os arch <<< "$target"
    echo "📦 Building for $os/$arch..."
    
    output_name="${APP_NAME}_${os}_${arch}"
    if [ "$os" = "windows" ]; then
        output_name+=".exe"
    fi
    
    GOOS=$os GOARCH=$arch go build -ldflags "-s -w -X 'main.Version=$VERSION'" -o "$DIST_DIR/$output_name" "$ENTRYPOINT"
    
    # Create archive
    if command -v zip >/dev/null 2>&1 && [ "$os" = "windows" ]; then
        (cd "$DIST_DIR" && zip -q "${APP_NAME}_${VERSION}_${os}_${arch}.zip" "$output_name" && rm "$output_name")
    else
        (cd "$DIST_DIR" && tar -czf "${APP_NAME}_${VERSION}_${os}_${arch}.tar.gz" "$output_name" && rm "$output_name")
    fi
done

echo "✅ Build complete. Artifacts are in the $DIST_DIR directory."
