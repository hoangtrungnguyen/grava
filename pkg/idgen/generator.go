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
	// GenerateBaseID creates a new top-level ID (e.g., grava-a1b2c3d4).
	GenerateBaseID() string
	// GenerateChildID creates a child ID based on a parent ID (e.g., grava-a1b2c3d4.1).
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
// hashes the result using SHA-256, and returns the prefix + the first 8 characters of the hex hash.
// This provides strong uniqueness for cross-system mirror flows (Plane sync, imports)
// where many issues live under a single project.
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

	// 8 hex chars → ~4.29B combinations (grava-a1b2c3d4 format).
	//
	// History: pre-2026-05 grava emitted 4 hex chars (~65k combos), fine for
	// human-scale projects but birthday-collision risk crossed 1 % around 36
	// issues and ~50 % around 300, which made cross-system mirror flows
	// (Plane `grava_id` property, JSONL imports) fragile. 8 hex pushes the 1 %
	// floor past ~9k issues per project — safe headroom for bidirectional
	// sync use cases.
	//
	// Backward compatibility: existing 4-hex IDs in the database stay valid.
	// pkg/validation.IssueIDPattern accepts both 4-hex and 8-hex widths so
	// `grava create --id grava-a1b2` (legacy) and `--id grava-a1b2c3d4`
	// (current) both pass.

	shortHash := fmt.Sprintf("%x", hash)[:8]
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
