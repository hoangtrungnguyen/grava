#!/usr/bin/env bash
# scripts/agent-bot-token.sh — runtime helper for the pr-creator agent.
#
# Sourced (or invoked + eval'd) before `gh pr create` / `git push`. Exports
# the bot's identity when configured; emits nothing when not. The pr-creator
# agent must then test the env vars and act accordingly:
#
#   source scripts/agent-bot-token.sh
#   if [ -n "${GRAVA_AGENT_BOT_TOKEN:-}" ]; then
#     # bot path: GH_TOKEN + git author override
#     GH_TOKEN="$GRAVA_AGENT_BOT_TOKEN" gh pr create ...
#     git -c user.name="$GRAVA_AGENT_BOT_USER" \
#         -c user.email="$GRAVA_AGENT_BOT_EMAIL" \
#         commit --amend --no-edit --reset-author
#   else
#     # fallback: user's gh auth + git config — no override
#     gh pr create ...
#   fi
#
# Exposes (env vars when bot is configured; unset otherwise):
#   GRAVA_AGENT_BOT_TOKEN   — fine-grained PAT
#   GRAVA_AGENT_BOT_USER    — GitHub login
#   GRAVA_AGENT_BOT_EMAIL   — git author email
#
# Run-mode (`./scripts/agent-bot-token.sh`) prints status to stderr and
# the token to stdout (if available) — useful for debugging or piping
# into `GH_TOKEN=$(scripts/agent-bot-token.sh) gh ...`.

REPO_ROOT="${CLAUDE_PROJECT_DIR:-$(pwd)}"
TOKEN_FILE="$REPO_ROOT/.grava/agent-bot.token"
USER_FILE="$REPO_ROOT/.grava/agent-bot.username"
EMAIL_FILE="$REPO_ROOT/.grava/agent-bot.email"

# Always clear previous state so re-sourcing in the same shell behaves predictably.
unset GRAVA_AGENT_BOT_TOKEN GRAVA_AGENT_BOT_USER GRAVA_AGENT_BOT_EMAIL

if [ -f "$TOKEN_FILE" ] && [ -s "$TOKEN_FILE" ]; then
  GRAVA_AGENT_BOT_TOKEN="$(cat "$TOKEN_FILE")"
  GRAVA_AGENT_BOT_USER="$(cat "$USER_FILE" 2>/dev/null || echo "")"
  GRAVA_AGENT_BOT_EMAIL="$(cat "$EMAIL_FILE" 2>/dev/null || echo "")"
  export GRAVA_AGENT_BOT_TOKEN GRAVA_AGENT_BOT_USER GRAVA_AGENT_BOT_EMAIL
fi

# When invoked directly (not sourced), behave like a token printer.
# Robust source-detection: `return` is only legal from a sourced file or a
# function. Subshell call returns 0 when sourced, errors when invoked.
# Works in both bash and zsh.
(return 0 2>/dev/null) && _agent_bot_token_sourced=1 || _agent_bot_token_sourced=0

if [ "$_agent_bot_token_sourced" -eq 0 ]; then
  if [ -n "${GRAVA_AGENT_BOT_TOKEN:-}" ]; then
    echo "agent-bot: $GRAVA_AGENT_BOT_USER <$GRAVA_AGENT_BOT_EMAIL>" >&2
    printf '%s' "$GRAVA_AGENT_BOT_TOKEN"
  else
    echo "agent-bot: not configured (run scripts/setup-agent-bot.sh to enable)" >&2
    exit 1
  fi
fi
unset _agent_bot_token_sourced
