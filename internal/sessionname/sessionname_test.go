package sessionname

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateSessionName(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		expected  string
	}{
		{
			name:      "Valid UUID",
			sessionID: "73b5e210-ec1a-4294-96e4-c2aecb2e1063",
			expected:  "zesty", // Deterministic based on hash
		},
		{
			name:      "Different UUID",
			sessionID: "12345678-1234-1234-1234-123456789abc",
			expected:  "bird", // Different deterministic result
		},
		{
			name:      "Empty session ID",
			sessionID: "",
			expected:  "unknown",
		},
		{
			name:      "Unknown session ID",
			sessionID: "unknown",
			expected:  "unknown",
		},
		{
			name:      "Short session ID",
			sessionID: "short",
			expected:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSessionName(tt.sessionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateSessionNameDeterministic(t *testing.T) {
	sessionID := "73b5e210-ec1a-4294-96e4-c2aecb2e1063"

	// Generate name multiple times
	name1 := GenerateSessionName(sessionID)
	name2 := GenerateSessionName(sessionID)
	name3 := GenerateSessionName(sessionID)

	// Should always return the same name
	assert.Equal(t, name1, name2)
	assert.Equal(t, name2, name3)
}

func TestGenerateSessionNameFormat(t *testing.T) {
	sessionID := "73b5e210-ec1a-4294-96e4-c2aecb2e1063"
	name := GenerateSessionName(sessionID)

	// Should be a single word (adjectivenoun)
	assert.NotContains(t, name, "-")
	assert.NotEmpty(t, name)
}

func TestGenerateSessionLabel(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		expected  string
	}{
		{
			name:      "Valid UUID includes prefix",
			sessionID: "06ddb8f7-03ff-4fdb-9fd8-5710213661b1",
			expected:  "happy 06ddb8f7",
		},
		{
			name:      "Another UUID",
			sessionID: "73b5e210-ec1a-4294-96e4-c2aecb2e1063",
			expected:  "zesty 73b5e210",
		},
		{
			name:      "Empty session ID",
			sessionID: "",
			expected:  "unknown",
		},
		{
			name:      "Unknown session ID",
			sessionID: "unknown",
			expected:  "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateSessionLabel(tt.sessionID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHexToInt(t *testing.T) {
	tests := []struct {
		hex      string
		expected int
	}{
		{"73b5e2", 7583202},
		{"ec1a42", 15473218},
		{"000000", 0},
		{"ffffff", 16777215},
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			result := hexToInt(tt.hex)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHexToInt_LongHex(t *testing.T) {
	// Test that hex strings longer than 6 chars are truncated
	result := hexToInt("1234567890")
	expected := hexToInt("123456") // Should be truncated to first 6 chars
	assert.Equal(t, expected, result)
	assert.Equal(t, 0x123456, result)
}

func TestHexToInt_InvalidHex(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid chars", "zzz"},
		{"empty string", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hexToInt(tt.input)
			assert.Equal(t, 0, result, "Invalid hex should return 0")
		})
	}
}

func TestHexToInt_PartiallyValid(t *testing.T) {
	// fmt.Sscanf with %x parses valid hex prefix and stops at first invalid char
	result := hexToInt("12z45")
	assert.Equal(t, 0x12, result, "Should parse valid hex prefix '12'")
	assert.Equal(t, 18, result)
}
