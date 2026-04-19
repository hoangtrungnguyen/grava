package utils

import (
	"fmt"
	"os/exec"
)

// CheckClaudeInstalled verifies that the Claude CLI is available on the system
// PATH. Grava is designed to work with Claude Code and requires it to be
// installed. Returns nil if found, or an error with installation guidance.
func CheckClaudeInstalled() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found on PATH — grava requires Claude Code: https://docs.anthropic.com/en/docs/claude-code/overview")
	}
	return nil
}
