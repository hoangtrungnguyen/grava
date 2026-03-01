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

echo "🔍 Detecting latest version..."
# Get the latest release tag from GitHub API
LATEST_RELEASE_DATA=$(curl -s "https://api.github.com/repos/${OWNER}/${REPO}/releases/latest")
VERSION=$(echo "$LATEST_RELEASE_DATA" | grep '"tag_name":' | cut -d '"' -f 4)

if [ -z "$VERSION" ]; then
    echo "❌ Could not determine latest version."
    exit 1
fi

echo "⬇️  Downloading ${BINARY} ${VERSION} for ${OS}/${ARCH}..."

# Match the naming convention in build_release.sh
# Example: grava_v0.0.4_darwin_arm64.tar.gz
EXT="tar.gz"
if [ "$OS" = "windows" ]; then
    EXT="zip"
fi

ASSET_PATTERN="${BINARY}_${VERSION}_${OS}_${ARCH}.${EXT}"
DOWNLOAD_URL=$(echo "$LATEST_RELEASE_DATA" | grep "browser_download_url" | grep "$ASSET_PATTERN" | cut -d '"' -f 4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "❌ Could not find a release asset matching: $ASSET_PATTERN"
    echo "Available assets at https://github.com/${OWNER}/${REPO}/releases/tag/${VERSION}"
    exit 1
fi

TMP_DIR=$(mktemp -d)
TMP_ARCHIVE="${TMP_DIR}/grava_archive.${EXT}"

echo "🔗 Downloading from: $DOWNLOAD_URL"
curl -sL -o "$TMP_ARCHIVE" "$DOWNLOAD_URL"

echo "📦 Extracting..."
if [ "$EXT" = "zip" ]; then
    unzip -q -j "$TMP_ARCHIVE" -d "$TMP_DIR"
else
    tar -xzf "$TMP_ARCHIVE" -C "$TMP_DIR"
fi

# The binary inside the archive is named grava_${os}_${arch}
# But we want to install it as just 'grava'
EXT_BIN=""
if [ "$OS" = "windows" ]; then
    EXT_BIN=".exe"
fi

FOUND_BIN=$(find "$TMP_DIR" -maxdepth 1 -name "${BINARY}_${OS}_${ARCH}${EXT_BIN}" | head -n 1)

if [ -z "$FOUND_BIN" ]; then
    # Fallback: maybe just 'grava' or similarly named?
    FOUND_BIN=$(find "$TMP_DIR" -maxdepth 1 -type f -executable | grep -v "grava_archive" | head -n 1)
fi

if [ -z "$FOUND_BIN" ]; then
    echo "❌ Could not find binary in archive."
    exit 1
fi

chmod +x "$FOUND_BIN"

echo "🔧 Installing to $INSTALL_DIR..."
if [ -w "$INSTALL_DIR" ]; then
    mv "$FOUND_BIN" "$INSTALL_DIR/${BINARY}${EXT_BIN}"
else
    sudo mv "$FOUND_BIN" "$INSTALL_DIR/${BINARY}${EXT_BIN}"
fi

rm -rf "$TMP_DIR"

echo "✅ Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
echo "🚀 Run '${BINARY} help' to get started!"
