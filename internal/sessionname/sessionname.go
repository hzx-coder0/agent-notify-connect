package sessionname

import (
	"fmt"
	"strings"
)

// Lists for friendly name generation (same as bash version)
var adjectives = []string{
	"bold", "brave", "bright", "calm", "clever",
	"cool", "cosmic", "crisp", "daring", "eager",
	"fair", "fancy", "fast", "gentle", "glad",
	"grand", "happy", "kind", "lively", "lucky",
	"merry", "noble", "proud", "quick", "quiet",
	"rapid", "smart", "solid", "swift", "warm",
	"wise", "witty", "zesty", "agile", "alert",
}

var nouns = []string{
	"bear", "bird", "cat", "deer", "eagle",
	"fish", "fox", "hawk", "lion", "owl",
	"star", "moon", "sun", "wind", "wave",
	"tree", "river", "mountain", "ocean", "cloud",
	"tiger", "wolf", "dragon", "phoenix", "falcon",
	"comet", "galaxy", "planet", "nova", "meteor",
	"forest", "canyon", "valley", "peak", "storm",
}

// GenerateSessionName generates a friendly single-word name from a session ID (UUID).
// Returns a deterministic name like "cat" or "eagle".
//
// Args:
//   - sessionID: UUID string (e.g., "73b5e210-ec1a-4294-96e4-c2aecb2e1063")
//
// Returns:
//   - Friendly name string (e.g., "peak")
func GenerateSessionName(sessionID string) string {
	// Return "unknown" if no session ID
	if sessionID == "" || sessionID == "unknown" {
		return "unknown"
	}

	// Remove dashes and convert to lowercase
	cleanID := strings.ToLower(strings.ReplaceAll(sessionID, "-", ""))

	// Use first 8 hex chars as seed for word selection
	if len(cleanID) < 8 {
		// Fallback for short IDs
		return "unknown"
	}

	seed := cleanID[0:8]

	// Combine adjectives and nouns into a single pool for more variety
	allWords := append(adjectives, nouns...)

	// Convert hex to decimal for array indexing
	index := hexToInt(seed) % len(allWords)

	return allWords[index]
}

// GenerateSessionLabel generates a friendly name with session ID prefix.
// Returns a string like "bold 06ddb8f7" for better session identification.
func GenerateSessionLabel(sessionID string) string {
	name := GenerateSessionName(sessionID)
	if name == "unknown" {
		return "unknown"
	}

	// Extract first 8 chars of UUID (before first dash)
	prefix := sessionID
	if idx := strings.Index(sessionID, "-"); idx != -1 {
		prefix = sessionID[:idx]
	}
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	return name + " " + prefix
}

// hexToInt converts hex string to int (takes first 6 characters for safety)
func hexToInt(hex string) int {
	if len(hex) > 6 {
		hex = hex[0:6]
	}

	var result int
	if _, err := fmt.Sscanf(hex, "%x", &result); err != nil {
		return 0 // Return 0 on parse error
	}
	return result
}
