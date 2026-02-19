#!/bin/bash
set -e

# Define version (can be passed as argument or extracted from git tag)
# Check if this is a release version
VERSION=${1:-$(git describe --tags --always --dirty)}

if [ -n "$1" ]; then
    if git rev-parse "$1" >/dev/null 2>&1; then
        echo "Tag $1 already exists. Building from existing tag."
    else
        echo "Creating release $1..."
        
        # Ensure clean git state
        if [ -n "$(git status --porcelain)" ]; then
            echo "❌ Error: Git working directory is not clean. Commit or stash changes before releasing."
            exit 1
        fi

        # Get previous tag
        PREV_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
        
        if [ -z "$PREV_TAG" ]; then
            echo "No previous tag found. Generating changelog from start."
            COMMITS=$(git log --pretty=format:"* %s (%h)" --no-merges)
        else
            echo "Generating changelog since $PREV_TAG..."
            COMMITS=$(git log --pretty=format:"* %s (%h)" --no-merges "${PREV_TAG}..HEAD")
        fi
        
        # Update CHANGELOG.md
        DATE=$(date +%Y-%m-%d)
        HEADER="## [$VERSION] - $DATE"
        
        TEMP_FILE="temp_changelog.md"
        
        # Create CHANGELOG.md if it doesn't exist
        if [ ! -f CHANGELOG.md ]; then
            echo "# Changelog" > CHANGELOG.md
            echo "" >> CHANGELOG.md
            echo "All notable changes to this project will be documented in this file." >> CHANGELOG.md
            echo "" >> CHANGELOG.md
        fi

        # Extract header (lines 1-7 or less if file is short)
        if [ $(wc -l < CHANGELOG.md) -ge 7 ]; then
             head -n 7 CHANGELOG.md > "$TEMP_FILE"
             TAIL_CMD="tail -n +8"
        else
             cat CHANGELOG.md > "$TEMP_FILE"
             TAIL_CMD="true" # No tail needed or empty
        fi

        echo "" >> "$TEMP_FILE"
        echo "$HEADER" >> "$TEMP_FILE"
        echo "" >> "$TEMP_FILE"
        echo "$COMMITS" >> "$TEMP_FILE"
        echo "" >> "$TEMP_FILE"
        
        if [ "$TAIL_CMD" != "true" ]; then
            $TAIL_CMD CHANGELOG.md >> "$TEMP_FILE"
        fi
        
        mv "$TEMP_FILE" CHANGELOG.md
        
        # Commit and Tag
        git add CHANGELOG.md
        git commit -m "chore(release): prepare release $VERSION"
        git tag -a "$VERSION" -m "Release $VERSION"
        echo "✅ Tagged $VERSION"
    fi
fi

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
