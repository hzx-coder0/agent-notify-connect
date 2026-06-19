//go:build !darwin && !linux

package notifier

import (
	"fmt"

	"github.com/hzx-coder0/claude-codex-notifications/internal/config"
	"github.com/gen2brain/beeep"
)

// GetTerminalBundleID returns empty string on non-macOS platforms
// as terminal bundle IDs are a macOS-specific concept.
func GetTerminalBundleID(configOverride string) string {
	return ""
}

// GetTerminalNotifierPath returns an error on non-macOS platforms
// as terminal-notifier is macOS-only.
func GetTerminalNotifierPath() (string, error) {
	return "", fmt.Errorf("terminal-notifier is only available on macOS")
}

// IsTerminalNotifierAvailable returns false on non-macOS platforms.
func IsTerminalNotifierAvailable() bool {
	return false
}

// EnsureClaudeNotificationsApp is a no-op on non-macOS platforms.
func EnsureClaudeNotificationsApp() error {
	return nil
}

// sendLinuxNotification is a stub for non-Linux platforms.
// On Windows, this falls back to beeep directly.
func sendLinuxNotification(title, body, appIcon string, cfg *config.Config, cwd string) error {
	return beeep.Notify(title, body, appIcon)
}

// IsDaemonAvailable returns false on non-Linux platforms.
func IsDaemonAvailable() bool {
	return false
}

// StartDaemon is a no-op on non-Linux platforms.
func StartDaemon() bool {
	return false
}

// StopDaemon is a no-op on non-Linux platforms.
func StopDaemon() error {
	return nil
}
