#!/bin/bash
# Idempotent installer for repo-local git hooks.
set -e
ROOT=$(git rev-parse --show-toplevel)
SRC="$ROOT/scripts/git-hooks"
DST="$ROOT/.git/hooks"

mkdir -p "$DST"
for hook in "$SRC"/*; do
  name=$(basename "$hook")
  cp "$hook" "$DST/$name"
  chmod +x "$DST/$name"
  echo "installed: $name"
done
