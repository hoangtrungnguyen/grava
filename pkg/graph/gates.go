package graph

import (
	"fmt"
	"time"
)

// GateEvaluator evaluates gate conditions
type GateEvaluator interface {
	IsGateOpen(node *Node) (bool, error)
	GetGateStatus(node *Node) (string, error)
}

// DefaultGateEvaluator implements basic gate evaluation
type DefaultGateEvaluator struct {
	gitHubClient GitHubClient
}

// NewDefaultGateEvaluator creates default gate evaluator
func NewDefaultGateEvaluator() *DefaultGateEvaluator {
	return &DefaultGateEvaluator{
		gitHubClient: nil, // Injected later
	}
}

// IsGateOpen checks if a gate condition is met
func (ge *DefaultGateEvaluator) IsGateOpen(node *Node) (bool, error) {
	if node.AwaitType == "" {
		return true, nil // No gate
	}

	switch node.AwaitType {
	case "timer":
		return ge.evaluateTimerGate(node)
	case "gh:pr":
		return ge.evaluateGitHubPRGate(node)
	case "human":
		return ge.evaluateHumanGate(node)
	default:
		return false, fmt.Errorf("unknown gate type: %s", node.AwaitType)
	}
}

// GetGateStatus returns a human-readable status of the gate
func (ge *DefaultGateEvaluator) GetGateStatus(node *Node) (string, error) {
	if node.AwaitType == "" {
		return "open", nil
	}
	open, err := ge.IsGateOpen(node)
	if err != nil {
		return "", err
	}
	if open {
		return "open", nil
	}
	return "pending", nil
}

// evaluateTimerGate checks timer-based gates
func (ge *DefaultGateEvaluator) evaluateTimerGate(node *Node) (bool, error) {
	if node.AwaitID == "" {
		return false, fmt.Errorf("timer gate missing await_id")
	}

	// Parse timestamp (ISO 8601 or relative duration)
	targetTime, err := time.Parse(time.RFC3339, node.AwaitID)
	if err != nil {
		// Try parsing as relative duration (e.g., "+7d")
		// Implementation omitted for brevity
		return false, fmt.Errorf("invalid timer format: %w", err)
	}

	return time.Now().After(targetTime), nil
}

// evaluateGitHubPRGate checks GitHub PR status
func (ge *DefaultGateEvaluator) evaluateGitHubPRGate(node *Node) (bool, error) {
	if ge.gitHubClient == nil {
		// Graceful degradation: if GitHub API unavailable, gate is closed
		return false, nil
	}

	// Parse await_id: "owner/repo/pulls/123"
	// Query GitHub API for PR status
	// Check if merged_at != nil
	// Implementation omitted for brevity

	return false, fmt.Errorf("GitHub PR gate not yet implemented")
}

// evaluateHumanGate checks for human approval
func (ge *DefaultGateEvaluator) evaluateHumanGate(node *Node) (bool, error) {
	// For now, return false (needs approval)
	return false, nil
}

// GitHubClient interface for GitHub API
type GitHubClient interface {
	IsPRMerged(owner, repo string, prNumber int) (bool, error)
}
