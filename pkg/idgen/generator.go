package idgen

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"time"

	"github.com/hoangtrungnguyen/grava/pkg/dolt"
)

// IDGenerator defines the interface for generating issue IDs.
type IDGenerator interface {
	// GenerateBaseID creates a new top-level ID (e.g., grava-a1b2).
	GenerateBaseID() string
	// GenerateChildID creates a child ID based on a parent ID (e.g., grava-a1b2.1).
	GenerateChildID(parentID string) (string, error)
}

// StandardGenerator is the default implementation of IDGenerator.
type StandardGenerator struct {
	Prefix string
	Store  dolt.Store
}

// NewStandardGenerator creates a new generator with the default "grava" prefix.
func NewStandardGenerator(store dolt.Store) *StandardGenerator {
	return &StandardGenerator{
		Prefix: "grava",
		Store:  store,
	}
}

// GenerateBaseID generates a hash-based ID.
// It combines the current nanosecond timestamp with a random value,
// hashes the result using SHA-256, and returns the prefix + the first 4 characters of the hex hash.
// This provides reasonable uniqueness for a project-scale issue tracker.
func (g *StandardGenerator) GenerateBaseID() string {
	timestamp := time.Now().UnixNano()

	// Generate a random number to ensure uniqueness even if called at the exact same nanosecond
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		// In the extremely unlikely event of an entropy failure, fallback to a pseudo-random seed
		// derived from the timestamp itself, though crypto/rand should rarely fail.
		n = big.NewInt(timestamp % 999999)
	}

	input := fmt.Sprintf("%d-%d", timestamp, n.Int64())
	hash := sha256.Sum256([]byte(input))

	// Use the first 4 bytes (8 hex chars) for a short but unique enough ID in a small scope
	// Or first 2 bytes (4 hex chars) if we want very short IDs like grava-a1b2.
	// The requirement says "grava-XXXX" (hash-based), which implies 4 chars usually,
	// but 4 hex chars (16^4 = 65536) might be too small for a long running project.
	// Let's use 6 chars to be safer while still short: 16^6 = 16 million combinations if uniform.
	// Actually, task description examples use "a1b2", which is 4 chars.
	// Let's stick to 4 chars as requested but note the collision risk in comments.
	// If strict 4 chars is required, we use 4. If variable, we might use more.
	// Task-1-3 description says "grava-a1b2".

	shortHash := fmt.Sprintf("%x", hash)[:4]
	return fmt.Sprintf("%s-%s", g.Prefix, shortHash)
}

// GenerateChildID generates a hierarchical child ID using the backing store.
// Returns an error if the store fails (changed signature to (string, error)).
func (g *StandardGenerator) GenerateChildID(parentID string) (string, error) {
	seq, err := g.Store.GetNextChildSequence(parentID)
	if err != nil {
		return "", fmt.Errorf("failed to generate child sequence: %w", err)
	}
	return fmt.Sprintf("%s.%d", parentID, seq), nil
}
