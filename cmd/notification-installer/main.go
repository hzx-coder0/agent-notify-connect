package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hzx-coder0/agent-notify-connect/internal/analyzer"
	"github.com/hzx-coder0/agent-notify-connect/internal/config"
	"github.com/hzx-coder0/agent-notify-connect/internal/feishu"
	"github.com/hzx-coder0/agent-notify-connect/internal/installer"
	"github.com/hzx-coder0/agent-notify-connect/internal/logging"
	"github.com/hzx-coder0/agent-notify-connect/internal/webhook"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "installer: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	reader := bufio.NewReader(os.Stdin)
	sourceExePath, err := findNotifierExe()
	if err != nil {
		return err
	}
	sourceRoot := pluginRootFromExe(sourceExePath)
	installRoot, err := installer.InstallRoot()
	if err != nil {
		return err
	}
	exePath, err := installer.InstallRuntime(sourceRoot, installRoot)
	if err != nil {
		return err
	}

	cfg, err := config.LoadFromPluginRoot(installRoot)
	if err != nil {
		return err
	}
	cfg.ApplyDefaults()
	installer.SetRuntimeAssetPaths(cfg, installRoot)

	fmt.Println("Claude/Codex Notifications Installer")
	fmt.Printf("Installed to: %s\n", installRoot)
	fmt.Printf("Notifier exe: %s\n", exePath)
	fmt.Println()

	profile := collectProfile(reader, cfg)
	installer.ApplyProfile(cfg, profile)

	installClaude := askYesNo(reader, "Write Claude Code hooks to ~/.claude/settings.json?", true)
	installCodex := askYesNo(reader, "Write Codex hooks to ~/.codex/hooks.json?", true)

	if askYesNo(reader, "Connect Feishu/Lark now?", !installer.HasFeishuAppBinding(cfg)) {
		result, err := feishu.Bind(context.Background(), installRoot, feishu.BindingOptions{
			Timeout:         10 * time.Minute,
			ReceiveIDType:   "open_id",
			OpenBrowser:     true,
			PrintURL:        true,
			PrintTerminalQR: true,
			Out:             os.Stdout,
		})
		if err != nil {
			return fmt.Errorf("feishu bind: %w", err)
		}
		fmt.Println("Feishu binding saved.")
		fmt.Printf("Config: %s\n", result.ConfigPath)
		reloaded, err := config.LoadFromPluginRoot(installRoot)
		if err != nil {
			return err
		}
		cfg = reloaded
		cfg.ApplyDefaults()
		installer.SetRuntimeAssetPaths(cfg, installRoot)
		installer.ApplyProfile(cfg, profile)
		installer.EnableFeishuApp(cfg)
	}

	if cfg.Notifications.Webhook.Enabled && !installer.HasFeishuAppBinding(cfg) && strings.TrimSpace(cfg.Notifications.Webhook.URL) == "" {
		fmt.Println("Webhook is enabled, but no Feishu binding or custom webhook URL is configured.")
		if askYesNo(reader, "Disable webhook for now?", true) {
			cfg.Notifications.Webhook.Enabled = false
		}
	}

	configPath, err := config.GetStableConfigPath()
	if err != nil {
		return err
	}
	if err := installer.WriteConfigFile(configPath, cfg); err != nil {
		return err
	}
	fmt.Printf("Config written: %s\n", configPath)

	if installClaude {
		path, err := installer.ClaudeSettingsPath()
		if err != nil {
			return err
		}
		if err := mergeAndWriteHooks(path, installer.WindowsClaudeHookSettings(exePath)); err != nil {
			return err
		}
		fmt.Printf("Claude hooks written: %s\n", path)
	}

	if installCodex {
		path, err := installer.CodexHooksPath()
		if err != nil {
			return err
		}
		if err := mergeAndWriteHooks(path, installer.CodexHookSettings(exePath)); err != nil {
			return err
		}
		fmt.Printf("Codex hooks written: %s\n", path)
	}

	printConnectionStatus(cfg)
	if cfg.IsWebhookEnabled() && askYesNo(reader, "Send test webhook notification now?", installer.HasFeishuAppBinding(cfg)) {
		if err := sendWebhookTest(cfg); err != nil {
			return err
		}
		fmt.Println("Webhook test sent.")
	}

	fmt.Println("Done. Restart Claude Code/Codex and trust hooks when prompted.")
	return nil
}

func collectProfile(reader *bufio.Reader, cfg *config.Config) installer.Profile {
	profile := installer.Profile{
		DesktopEnabled: askYesNo(reader, "Enable desktop notifications?", cfg.Notifications.Desktop.Enabled),
		DesktopSound:   askYesNo(reader, "Enable notification sound?", cfg.Notifications.Desktop.Sound),
		WebhookEnabled: askYesNo(reader, "Enable Feishu/Lark/webhook notifications?", cfg.Notifications.Webhook.Enabled),
		StatusEnabled:  map[string]bool{},
		StatusDesktop:  map[string]bool{},
		StatusWebhook:  map[string]bool{},
	}

	fmt.Println()
	fmt.Println("Choose statuses:")
	for _, status := range installer.StatusOrder {
		profile.StatusEnabled[status] = askYesNo(reader, fmt.Sprintf("Notify on %s?", status), cfg.IsStatusEnabled(status))
		if profile.DesktopEnabled {
			profile.StatusDesktop[status] = askYesNo(reader, fmt.Sprintf("  Desktop for %s?", status), cfg.IsStatusDesktopEnabled(status))
		}
		if profile.WebhookEnabled {
			profile.StatusWebhook[status] = askYesNo(reader, fmt.Sprintf("  Webhook for %s?", status), cfg.IsStatusWebhookEnabled(status))
		}
	}
	fmt.Println()
	return profile
}

func mergeAndWriteHooks(path string, generated installer.HookSettings) error {
	existing, err := installer.ReadHookSettings(path)
	if err != nil {
		return fmt.Errorf("read hook settings %s: %w", path, err)
	}
	merged := installer.MergeHookSettings(existing, generated)
	if err := installer.WriteHookSettings(path, merged); err != nil {
		return fmt.Errorf("write hook settings %s: %w", path, err)
	}
	return nil
}

func printConnectionStatus(cfg *config.Config) {
	fmt.Println()
	fmt.Println("Status:")
	fmt.Printf("  desktop: %s\n", yesNo(cfg.IsDesktopEnabled()))
	fmt.Printf("  webhook: %s\n", yesNo(cfg.IsWebhookEnabled()))
	if cfg.Notifications.Webhook.Preset == "feishu_app" {
		fmt.Printf("  feishu_app binding: %s\n", yesNo(installer.HasFeishuAppBinding(cfg)))
		fmt.Printf("  receive_id_type: %s\n", cfg.Notifications.Feishu.ReceiveIDType)
	}
}

func sendWebhookTest(cfg *config.Config) error {
	if _, err := logging.InitLogger(pluginRootFromCurrentDir()); err != nil {
		return err
	}
	defer logging.Close()

	sender := webhook.New(cfg)
	defer sender.Shutdown(5 * time.Second) //nolint:errcheck

	return sender.SendWithContext(webhook.SendContext{
		Status:        analyzer.StatusTaskComplete,
		Message:       "[installer test] Claude/Codex notification test",
		SessionID:     "installer-test",
		CWD:           currentDir(),
		RawBody:       "Claude/Codex notification test",
		ActionSummary: "installer connectivity check",
	})
}

func askYesNo(reader *bufio.Reader, question string, defaultValue bool) bool {
	suffix := "[y/N]"
	if defaultValue {
		suffix = "[Y/n]"
	}
	for {
		fmt.Printf("%s %s ", question, suffix)
		line, _ := reader.ReadString('\n')
		answer := strings.ToLower(strings.TrimSpace(line))
		if answer == "" {
			return defaultValue
		}
		switch answer {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Println("Please answer y or n.")
		}
	}
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

func pluginRootFromCurrentDir() string {
	cwd := currentDir()
	if filepath.Base(cwd) == "bin" {
		return filepath.Dir(cwd)
	}
	return cwd
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

func yesNo(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}
