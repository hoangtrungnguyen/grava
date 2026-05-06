#!/usr/bin/env bash
# scripts/setup-agent-bot.sh — interactive setup for the agent-bot author identity.
#
# Configures a separate GitHub identity that the pr-creator agent uses for
# pushing branches and opening PRs. With this set up, PRs land on GitHub as
# "opened by <agent-bot-username>" and commits are authored by the bot —
# instead of the human running /ship. Useful for distinguishing automated
# pipeline output from human work in the project history.
#
# Setup is OPTIONAL. If skipped, pr-creator falls back to the user's `gh`
# auth and `git config user.*` and PRs/commits show the human identity.
#
# Usage:
#   ./scripts/setup-agent-bot.sh           # interactive
#   ./scripts/setup-agent-bot.sh --status  # report current config
#   ./scripts/setup-agent-bot.sh --remove  # delete saved token + identity
#
# Files written (all gitignored under .grava/agent-bot.*):
#   .grava/agent-bot.token       — fine-grained PAT, chmod 600
#   .grava/agent-bot.username    — GitHub login, chmod 644
#   .grava/agent-bot.email       — git author email, chmod 644

set -euo pipefail

REPO_ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
cd "$REPO_ROOT"

GRAVA_DIR=".grava"
TOKEN_FILE="$GRAVA_DIR/agent-bot.token"
USER_FILE="$GRAVA_DIR/agent-bot.username"
EMAIL_FILE="$GRAVA_DIR/agent-bot.email"

cmd="${1:-setup}"

print_status() {
  if [ -f "$USER_FILE" ] && [ -f "$TOKEN_FILE" ]; then
    local user email
    user=$(cat "$USER_FILE")
    email=$(cat "$EMAIL_FILE" 2>/dev/null || echo "<unset>")
    echo "✅ agent-bot is configured"
    echo "   username: $user"
    echo "   email:    $email"
    echo "   token:    $TOKEN_FILE (hidden, chmod $(stat -f '%Lp' "$TOKEN_FILE" 2>/dev/null || stat -c '%a' "$TOKEN_FILE" 2>/dev/null || echo "?"))"
    return 0
  fi
  echo "ℹ️  agent-bot is NOT configured"
  echo "   pr-creator agent will fall back to the user's gh auth + git config"
  echo "   Run \`scripts/setup-agent-bot.sh\` to enable bot attribution."
  return 1
}

remove_config() {
  rm -f "$TOKEN_FILE" "$USER_FILE" "$EMAIL_FILE"
  echo "🗑️  Removed $TOKEN_FILE, $USER_FILE, $EMAIL_FILE"
  echo "   pr-creator now uses the user's gh auth + git config."
}

case "$cmd" in
  --status|status)
    print_status
    exit $?
    ;;
  --remove|remove)
    remove_config
    exit 0
    ;;
  --help|-h|help)
    sed -n '2,20p' "$0"
    exit 0
    ;;
  setup|"")
    ;; # fall through to interactive setup
  *)
    echo "Unknown command: $cmd"
    echo "Try: --status | --remove | --help"
    exit 1
    ;;
esac

# ─── Interactive setup ─────────────────────────────────────────────────────

cat <<'EOF'

Agent-bot author setup
══════════════════════

This is OPTIONAL. With it, PRs opened by the /ship pipeline are attributed
to a separate GitHub user (e.g. `grava-agent-bot`) instead of you. Skip if
you'd rather pipeline PRs land under your own identity.

What you'll need beforehand:
  1. A second GitHub account for the bot (create at github.com/signup if needed).
  2. The bot account added as a collaborator on this repo with Write access.
  3. A fine-grained personal access token from the bot account, scoped to
     THIS repository, with permissions:
        - Contents:       Read and write
        - Pull requests:  Read and write
        - Metadata:       Read

If you don't have all three yet, hit Ctrl+C and come back when you do.

EOF

read -r -p "Continue with setup? [y/N]: " answer
case "$answer" in
  y|Y|yes|YES) ;;
  *) echo "Aborted. No changes made."; exit 0 ;;
esac

mkdir -p "$GRAVA_DIR"

# ── Bot username
while true; do
  read -r -p "Bot GitHub username (e.g. grava-agent-bot): " bot_user
  if [ -z "$bot_user" ]; then
    echo "  Empty username — try again."
    continue
  fi
  break
done

# ── Bot email (default to noreply pattern that GitHub accepts for commits)
default_email="${bot_user}@users.noreply.github.com"
read -r -p "Bot git author email [${default_email}]: " bot_email
bot_email="${bot_email:-$default_email}"

# ── PAT
echo ""
echo "Paste the bot's fine-grained PAT (input hidden). Ctrl+D when done if no value:"
read -r -s bot_token
echo ""
if [ -z "$bot_token" ]; then
  echo "❌ No token provided. Aborting."
  exit 1
fi

# ── Validate token by hitting GitHub
echo "🔍 Verifying token against GitHub /user…"
api_user=$(curl -sS -H "Authorization: token $bot_token" \
                  -H "Accept: application/vnd.github+json" \
                  https://api.github.com/user \
            | jq -r '.login // empty' 2>/dev/null || true)

if [ -z "$api_user" ]; then
  echo "❌ Token validation failed. GitHub rejected the credential."
  echo "   Check the token is fine-grained, not expired, and was generated"
  echo "   from the bot account (not yours)."
  exit 1
fi

if [ "$api_user" != "$bot_user" ]; then
  echo "⚠️  Token belongs to '$api_user' but you entered '$bot_user'."
  read -r -p "Use '$api_user' as the bot username? [Y/n]: " yn
  case "$yn" in
    n|N|no|NO) echo "Aborted — usernames don't match."; exit 1 ;;
    *) bot_user="$api_user" ;;
  esac
fi
echo "✅ Token valid for user: $bot_user"

# ── Persist
printf '%s' "$bot_token" > "$TOKEN_FILE"
printf '%s' "$bot_user"  > "$USER_FILE"
printf '%s' "$bot_email" > "$EMAIL_FILE"
chmod 600 "$TOKEN_FILE"
chmod 644 "$USER_FILE" "$EMAIL_FILE"

echo ""
echo "✅ agent-bot configured"
echo "   username: $bot_user"
echo "   email:    $bot_email"
echo "   token:    $TOKEN_FILE (chmod 600)"
echo ""
echo "Next: when you run /ship, PRs and commits will be attributed to '$bot_user'."
echo "To remove: scripts/setup-agent-bot.sh --remove"
