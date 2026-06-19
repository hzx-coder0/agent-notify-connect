package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hzx-coder0/claude-codex-notifications/internal/installer"
)

//go:embed installer_gui.ps1
var guiScript string

func main() {
	if err := run(); err != nil {
		showError(err)
		os.Exit(1)
	}
}

func run() error {
	sourceExe, err := findNotifierExe()
	if err != nil {
		return err
	}
	sourceRoot := pluginRootFromExe(sourceExe)

	installRoot, err := installer.InstallRoot()
	if err != nil {
		return err
	}
	notifierExe, err := installer.InstallRuntime(sourceRoot, installRoot)
	if err != nil {
		return err
	}

	scriptPath := filepath.Join(installRoot, "notification-installer-gui.ps1")
	if err := os.WriteFile(scriptPath, []byte(guiScript), 0o600); err != nil {
		return fmt.Errorf("write GUI script: %w", err)
	}

	cmd := exec.Command("powershell.exe",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-STA",
		"-File", scriptPath,
		"-InstallRoot", installRoot,
		"-NotifierExe", notifierExe,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func showError(err error) {
	msg := strings.ReplaceAll(err.Error(), "'", "''")
	_ = exec.Command("powershell.exe",
		"-NoProfile",
		"-Command",
		"Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.MessageBox]::Show('"+msg+"','Claude Notifications Installer','OK','Error')",
	).Run()
}

func findNotifierExe() (string, error) {
	if override := strings.TrimSpace(os.Getenv("CLAUDE_NOTIFICATIONS_EXE")); override != "" {
		return filepath.Abs(override)
	}
	self, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(self)
		candidate := filepath.Join(dir, "claude-notifications-windows-amd64.exe")
		if fileExists(candidate) {
			return filepath.Abs(candidate)
		}
		if filepath.Base(dir) == "bin" {
			candidate = filepath.Join(dir, "claude-notifications-windows-amd64.exe")
			if fileExists(candidate) {
				return filepath.Abs(candidate)
			}
		}
	}
	candidate := filepath.Join(currentDir(), "bin", "claude-notifications-windows-amd64.exe")
	if fileExists(candidate) {
		return filepath.Abs(candidate)
	}
	return "", fmt.Errorf("cannot find claude-notifications-windows-amd64.exe; set CLAUDE_NOTIFICATIONS_EXE")
}

func pluginRootFromExe(exePath string) string {
	dir := filepath.Dir(exePath)
	if filepath.Base(dir) == "bin" {
		return filepath.Dir(dir)
	}
	return currentDir()
}

func currentDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
