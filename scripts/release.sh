#!/bin/bash
set -e

# Define version (can be passed as argument or extracted from git tag)
VERSION=${1:-$(git describe --tags --always --dirty)}
echo "🚀 Building Grava version: $VERSION"

platforms=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
)

# Output directory for binaries
mkdir -p bin

for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    output_name="grava-$GOOS-$GOARCH"
    if [ "$GOOS" = "windows" ]; then
        output_name+='.exe'
    fi

    echo "Building for $GOOS/$GOARCH..."
    env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-X main.Version=$VERSION" -o bin/$output_name ./cmd/grava
    
    if [ $? -ne 0 ]; then
        echo "❌ An error has occurred! Aborting the script execution..."
        exit 1
    fi
done

echo "✅ Build successful! Binaries are in ./bin/"
