#!/bin/bash
set -e

OWNER="hoangtrungnguyen"
REPO="grava"
BINARY="grava"
INSTALL_DIR="/usr/local/bin"

# Detect OS and Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

map_arch() {
    case "$1" in
        x86_64) echo "amd64" ;;
        aarch64) echo "arm64" ;;
        arm64) echo "arm64" ;;
        *) echo "$1" ;;
    esac
}

ARCH=$(map_arch "$ARCH")
BINARY_NAME="${BINARY}-${OS}-${ARCH}"

if [ "$OS" = "windows" ]; then
    BINARY_NAME="${BINARY_NAME}.exe"
fi

echo "⬇️  Downloading ${BINARY} for ${OS}/${ARCH}..."

# Get the latest release tag from GitHub API
LATEST_RELEASE_URL="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
DOWNLOAD_URL=$(curl -s $LATEST_RELEASE_URL | grep "browser_download_url.*${BINARY_NAME}" | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "❌ Could not find a release for ${OS}/${ARCH}."
    echo "Please check https://github.com/${OWNER}/${REPO}/releases"
    exit 1
fi

echo "🔗 Downloading from: $DOWNLOAD_URL"
curl -sL -o "/tmp/${BINARY}" "$DOWNLOAD_URL"

echo "🔧 Installing to $INSTALL_DIR..."
chmod +x "/tmp/${BINARY}"

if [ -w "$INSTALL_DIR" ]; then
    mv "/tmp/${BINARY}" "$INSTALL_DIR/${BINARY}"
else
    sudo mv "/tmp/${BINARY}" "$INSTALL_DIR/${BINARY}"
fi

echo "✅ Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
echo "🚀 Run '${BINARY} help' to get started!"
