#!/bin/bash
# ci_setup_dolt.sh
#
# Downloads the Dolt binary, initialises a local database repo inside
# .grava/dolt, and starts the SQL server on port 3306.
#
# Designed for headless CI environments (GitHub Actions / Linux amd64).
# No Docker and no root/sudo required.
#
# Usage:
#   chmod +x ./scripts/ci_setup_dolt.sh
#   ./scripts/ci_setup_dolt.sh
#
# Environment variables (all optional):
#   DOLT_PORT   – port to listen on (default: 3306)
#   DOLT_HOST   – host to bind      (default: 0.0.0.0)
#   DOLT_LOG    – path to log file  (default: .grava/dolt.log)

set -euo pipefail

# ── Configuration ────────────────────────────────────────────────────────────
GRAVA_DIR=".grava"
BIN_DIR="${GRAVA_DIR}/bin"
DOLT_BIN="${BIN_DIR}/dolt"
DOLT_REPO_DIR="${GRAVA_DIR}/dolt"
MULTI_DB_DIR="${GRAVA_DIR}"          # parent dir — Dolt detects any dolt-repo sub-dirs
DOLT_PORT="${DOLT_PORT:-3306}"
DOLT_HOST="${DOLT_HOST:-0.0.0.0}"
DOLT_LOG="${DOLT_LOG:-.grava/dolt.log}"
MYSQL_CLIENT="${MYSQL_CLIENT:-mysql}"

# ── 1. Install Dolt binary ───────────────────────────────────────────────────
echo "📥 Step 1/4 – Installing Dolt binary to ${BIN_DIR}/ ..."
mkdir -p "${BIN_DIR}"

if [ -x "${DOLT_BIN}" ]; then
    echo "   ✅ Dolt binary already present at ${DOLT_BIN}, skipping download."
else
    # Fetch the latest release tag from GitHub API
    LATEST_VERSION=$(curl -sSf "https://api.github.com/repos/dolthub/dolt/releases/latest" \
        | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

    if [ -z "${LATEST_VERSION}" ]; then
        echo "❌ Could not determine latest Dolt version from GitHub API." >&2
        exit 1
    fi
    echo "   Latest Dolt version: ${LATEST_VERSION}"

    # Detect OS / arch
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    case "${ARCH}" in
        x86_64)  ARCH="amd64" ;;
        aarch64) ARCH="arm64" ;;
        arm64)   ARCH="arm64" ;;
        *)
            echo "❌ Unsupported architecture: ${ARCH}" >&2
            exit 1
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    TARBALL="dolt-${PLATFORM}.tar.gz"
    DOWNLOAD_URL="https://github.com/dolthub/dolt/releases/download/${LATEST_VERSION}/${TARBALL}"

    echo "   Downloading ${DOWNLOAD_URL} ..."
    TMP_TAR=$(mktemp /tmp/dolt-XXXXXX.tar.gz)
    curl -sSfL "${DOWNLOAD_URL}" -o "${TMP_TAR}"

    echo "   Extracting binary ..."
    tar -xzf "${TMP_TAR}" -C "${BIN_DIR}" --strip-components=2 "dolt-${PLATFORM}/bin/dolt"
    rm -f "${TMP_TAR}"

    chmod +x "${DOLT_BIN}"
    echo "   ✅ Dolt $(${DOLT_BIN} version) installed at ${DOLT_BIN}"
fi

# ── 2. Configure Dolt global user (required for dolt init) ──────────────────
echo "👤 Step 2/4 – Configuring Dolt user identity ..."
if ! "${DOLT_BIN}" config --global --list 2>/dev/null | grep -q "user.email"; then
    GIT_EMAIL=$(git config user.email 2>/dev/null || echo "ci@github.com")
    GIT_NAME=$(git config user.name  2>/dev/null || echo "CI Bot")
    "${DOLT_BIN}" config --global --add user.email "${GIT_EMAIL}"
    "${DOLT_BIN}" config --global --add user.name  "${GIT_NAME}"
    echo "   ✅ Configured Dolt user: ${GIT_NAME} <${GIT_EMAIL}>"
else
    echo "   ✅ Dolt user already configured."
fi

# ── 3. Initialise Dolt database repo ────────────────────────────────────────
echo "📦 Step 3/4 – Initialising Dolt repo in ${DOLT_REPO_DIR}/ ..."
mkdir -p "${DOLT_REPO_DIR}"

DOLT_BIN_ABS="$(pwd)/${DOLT_BIN}"
if [ ! -d "${DOLT_REPO_DIR}/.dolt" ]; then
    pushd "${DOLT_REPO_DIR}" > /dev/null
    "${DOLT_BIN_ABS}" init
    popd > /dev/null
    echo "   ✅ Dolt repo initialised."
else
    echo "   ✅ Dolt repo already exists, skipping init."
fi

# ── 4. Start Dolt SQL Server in the background ───────────────────────────────
echo "🚀 Step 4/4 – Starting Dolt SQL Server on ${DOLT_HOST}:${DOLT_PORT} ..."
mkdir -p "$(dirname "${DOLT_LOG}")"
DOLT_LOG_ABS="$(pwd)/${DOLT_LOG}"

# Launch directly in parent shell (not a subshell) so $! is set correctly
# --multi-db-dir lets Dolt serve ALL dolt repos under .grava/ and allows
# CREATE DATABASE via SQL (new repos are created as sub-dirs of .grava/).
pushd "${GRAVA_DIR}" > /dev/null
"${DOLT_BIN_ABS}" sql-server \
    --host="${DOLT_HOST}" \
    --port="${DOLT_PORT}" \
    --multi-db-dir="." \
    --loglevel=info \
    >> "${DOLT_LOG_ABS}" 2>&1 &
DOLT_PID=$!
popd > /dev/null

echo "   Server PID: ${DOLT_PID}"
echo "   Log file  : ${DOLT_LOG}"

# ── Health-check loop ────────────────────────────────────────────────────────
echo "⏳ Waiting for Dolt SQL Server to become ready ..."
for i in $(seq 1 30); do
    if mysqladmin ping -h 127.0.0.1 -P "${DOLT_PORT}" -u root --silent 2>/dev/null; then
        echo "✅ Dolt SQL Server is ready on port ${DOLT_PORT}!"
        exit 0
    fi
    echo "   Attempt ${i}/30 – not ready yet, retrying in 2 s ..."
    sleep 2
done

echo "❌ Dolt SQL Server did not become ready within 60 s." >&2
echo "--- Last 30 lines of ${DOLT_LOG} ---" >&2
tail -30 "${DOLT_LOG}" >&2
exit 1
