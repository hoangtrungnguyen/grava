#!/bin/bash
set -e

OWNER="hoangtrungnguyen"
REPO="grava"
BINARY="grava"

# Default to user-local directory — no sudo required.
# Users can override: INSTALL_DIR=/usr/local/bin bash install.sh
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"

# Parse flags
FROM_SOURCE=0
SOURCE_REF="${GRAVA_REF:-main}"
for arg in "$@"; do
  case "$arg" in
    --from-source)   FROM_SOURCE=1 ;;
    --ref=*)         SOURCE_REF="${arg#--ref=}" ;;
    --help|-h)
      cat <<USAGE
grava installer

Usage:
  curl -sL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/scripts/install.sh | bash
  curl -sL https://raw.githubusercontent.com/${OWNER}/${REPO}/main/scripts/install.sh | bash -s -- --from-source

Flags:
  --from-source        Clone the repo and build with 'go build' (needs Go ≥ 1.24, git)
  --ref=<branch|tag>   With --from-source: ref to build (default: main; env GRAVA_REF)
  -h, --help           Show this help

Env:
  INSTALL_DIR          Install location (default: \$HOME/.local/bin)
  GRAVA_REF            Same as --ref
USAGE
      exit 0
      ;;
    *) echo "❌ Unknown flag: $arg (try --help)"; exit 1 ;;
  esac
done

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

# Detect the user's shell profile file for PATH instructions
detect_shell_profile() {
    local shell_name
    shell_name="$(basename "${SHELL:-/bin/bash}")"
    case "$shell_name" in
        zsh)  echo "${HOME}/.zshrc" ;;
        bash)
            # macOS uses .bash_profile, Linux uses .bashrc
            if [ "$OS" = "darwin" ]; then
                echo "${HOME}/.bash_profile"
            else
                echo "${HOME}/.bashrc"
            fi
            ;;
        fish) echo "${HOME}/.config/fish/config.fish" ;;
        *)    echo "${HOME}/.profile" ;;
    esac
}

# ─── --from-source path ────────────────────────────────────────────────
if [ "$FROM_SOURCE" = "1" ]; then
  command -v go  >/dev/null 2>&1 || { echo "❌ go not found on PATH (need ≥ 1.24)"; exit 1; }
  command -v git >/dev/null 2>&1 || { echo "❌ git not found on PATH"; exit 1; }

  TMP_SRC=$(mktemp -d)
  trap 'rm -rf "$TMP_SRC"' EXIT

  echo "📥 Cloning ${OWNER}/${REPO}@${SOURCE_REF} into $TMP_SRC..."
  git clone --depth 1 --branch "$SOURCE_REF" \
    "https://github.com/${OWNER}/${REPO}.git" "$TMP_SRC/${REPO}" \
    >/dev/null 2>&1 || {
      echo "❌ Clone failed for ref '$SOURCE_REF'"; exit 1
    }

  cd "$TMP_SRC/${REPO}"
  VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "source")
  echo "🔨 Building ${BINARY} ${VERSION} (go build)..."
  go build -ldflags "-X main.Version=$VERSION" -o "${TMP_SRC}/${BINARY}" ./cmd/grava

  mkdir -p "$INSTALL_DIR"
  echo "🔧 Installing to $INSTALL_DIR..."
  mv "${TMP_SRC}/${BINARY}" "$INSTALL_DIR/${BINARY}"
  chmod +x "$INSTALL_DIR/${BINARY}"

  echo "✅ Installed ${BINARY} ${VERSION} to ${INSTALL_DIR}/${BINARY}"

  # PATH warning (shared with release path below — extracted into a fn would be cleaner,
  # but inlined here for the one extra usage to keep the script flat).
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      SHELL_PROFILE=$(detect_shell_profile)
      echo ""
      echo "⚠️  ${INSTALL_DIR} is not in your PATH. Add it to ${SHELL_PROFILE}."
      ;;
  esac

  echo "🚀 Run '${BINARY} version' to verify."
  exit 0
fi
# ────────────────────────────────────────────────────────────────────────

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

# URL encode '+' character because GitHub API returns URL-encoded download links
URL_SAFE_VERSION=$(echo "$VERSION" | sed 's/+/%2B/g')
ASSET_PATTERN="${BINARY}_${URL_SAFE_VERSION}_${OS}_${ARCH}.${EXT}"
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

# Ensure install directory exists (no sudo needed for ~/.local/bin)
mkdir -p "$INSTALL_DIR"

echo "🔧 Installing to $INSTALL_DIR..."
mv "$FOUND_BIN" "$INSTALL_DIR/${BINARY}${EXT_BIN}"

rm -rf "$TMP_DIR"

echo "✅ Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"

# Check if INSTALL_DIR is in PATH and warn if not
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*)
        # Already in PATH — no action needed
        ;;
    *)
        SHELL_PROFILE=$(detect_shell_profile)
        SHELL_NAME="$(basename "${SHELL:-/bin/bash}")"
        echo ""
        echo "⚠️  ${INSTALL_DIR} is not in your PATH."
        echo ""
        if [ "$SHELL_NAME" = "fish" ]; then
            echo "   Add it by running:"
            echo ""
            echo "     fish_add_path ${INSTALL_DIR}"
            echo ""
        else
            echo "   Add it by running:"
            echo ""
            echo "     echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ${SHELL_PROFILE}"
            echo "     source ${SHELL_PROFILE}"
            echo ""
        fi
        ;;
esac

echo "🚀 Run '${BINARY} version' to verify, then '${BINARY} help' to get started!"

# ─── Optional: agent-bot author setup ──────────────────────────────────────
# Offered ONLY when the install runs from a grava repo checkout (so the
# setup script is on disk). The downloadable installer hits release assets
# without cloning, so the prompt is skipped silently.
SETUP_BOT="$(dirname "$0")/setup-agent-bot.sh"
if [ -x "$SETUP_BOT" ]; then
  echo ""
  echo "─── Optional: agent-bot author identity ──────────────────────────"
  echo "  When /ship opens PRs, attribute them to a separate GitHub user"
  echo "  (e.g. 'grava-agent-bot') instead of you. Useful for tagging"
  echo "  pipeline-generated PRs vs human-authored work."
  echo "  Skip this if you'd rather PRs land under your own identity."
  echo ""
  if [ -t 0 ]; then
    read -r -p "Configure agent-bot now? [y/N]: " bot_answer
    case "$bot_answer" in
      y|Y|yes|YES) "$SETUP_BOT" ;;
      *) echo "Skipped. Run $SETUP_BOT later if you change your mind." ;;
    esac
  else
    echo "  Non-interactive shell — skipping. Run $SETUP_BOT manually if wanted."
  fi
fi
