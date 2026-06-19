//go:build !darwin && !linux

package notifier

import (
	"testing"
)

func TestGetTerminalBundleID_NonMacOS(t *testing.T) {
	// On non-macOS, should always return empty string
	result := GetTerminalBundleID("")
	if result != "" {
		t.Errorf("Expected empty string on non-macOS, got '%s'", result)
	}

	// Even with config override, should return empty
	result = GetTerminalBundleID("custom.bundle.id")
	if result != "" {
		t.Errorf("Expected empty string even with override on non-macOS, got '%s'", result)
	}
}

func TestGetTerminalNotifierPath_NonMacOS(t *testing.T) {
	// Should always return error on non-macOS
	path, err := GetTerminalNotifierPath()
	if err == nil {
		t.Errorf("Expected error on non-macOS, got path: %s", path)
	}
	if path != "" {
		t.Errorf("Expected empty path on non-macOS, got: %s", path)
	}
}

func TestIsTerminalNotifierAvailable_NonMacOS(t *testing.T) {
	// Should always return false on non-macOS
	available := IsTerminalNotifierAvailable()
	if available {
		t.Error("Expected false on non-macOS")
	}
}
