package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hzx-coder0/claude-codex-notifications/internal/config"
)

var StatusOrder = []string{
	"task_complete",
	"review_complete",
	"question",
	"plan_ready",
	"session_limit_reached",
	"api_error",
	"api_error_overloaded",
}

type HookSettings struct {
	Hooks map[string][]HookMatcherGroup `json:"hooks"`
}

type HookMatcherGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookCommand `json:"hooks"`
}

type HookCommand struct {
	Type          string   `json:"type,omitempty"`
	Command       string   `json:"command"`
	Args          []string `json:"args,omitempty"`
	Timeout       int      `json:"timeout"`
	StatusMessage string   `json:"statusMessage,omitempty"`
}

type Profile struct {
	DesktopEnabled bool
	DesktopSound   bool
	WebhookEnabled bool
	StatusEnabled  map[string]bool
	StatusDesktop  map[string]bool
	StatusWebhook  map[string]bool
}

func ApplyProfile(cfg *config.Config, profile Profile) {
	cfg.Notifications.Desktop.Enabled = profile.DesktopEnabled
	cfg.Notifications.Desktop.Sound = profile.DesktopSound
	cfg.Notifications.Webhook.Enabled = profile.WebhookEnabled

	if cfg.Statuses == nil {
		cfg.Statuses = config.DefaultConfig().Statuses
	}

	for _, status := range StatusOrder {
		info := cfg.Statuses[status]
		if enabled, ok := profile.StatusEnabled[status]; ok {
			info.Enabled = boolPtr(enabled)
		}
		if enabled, ok := profile.StatusDesktop[status]; ok {
			info.Desktop = &config.StatusChannelConfig{Enabled: boolPtr(enabled)}
		}
		if enabled, ok := profile.StatusWebhook[status]; ok {
			info.Webhook = &config.StatusChannelConfig{Enabled: boolPtr(enabled)}
		}
		cfg.Statuses[status] = info
	}
}

func EnableFeishuApp(cfg *config.Config) {
	cfg.Notifications.Webhook.Enabled = true
	cfg.Notifications.Webhook.Preset = "feishu_app"
	cfg.Notifications.Webhook.Format = "json"
	if cfg.Notifications.Feishu.Mode == "" {
		cfg.Notifications.Feishu.Mode = "app_registration"
	}
	if cfg.Notifications.Feishu.Platform == "" {
		cfg.Notifications.Feishu.Platform = "feishu"
	}
	if cfg.Notifications.Feishu.ReceiveIDType == "" {
		cfg.Notifications.Feishu.ReceiveIDType = "open_id"
	}
}

func HasFeishuAppBinding(cfg *config.Config) bool {
	feishuCfg := cfg.Notifications.Feishu
	return cfg.Notifications.Webhook.Enabled &&
		cfg.Notifications.Webhook.Preset == "feishu_app" &&
		feishuCfg.AppID != "" &&
		feishuCfg.ResolveAppSecret() != "" &&
		feishuCfg.ReceiveIDType != "" &&
		feishuCfg.ReceiveID != ""
}

func WriteConfigFile(path string, cfg *config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func InstallRoot() (string, error) {
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "ClaudeNotificationsGo"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "AppData", "Local", "ClaudeNotificationsGo"), nil
}

func InstalledNotifierPath(installRoot string) string {
	return filepath.Join(installRoot, "bin", "claude-notifications-windows-amd64.exe")
}

func NewVersionedNotifierPath(installRoot string) string {
	stamp := time.Now().Format("20060102150405")
	return filepath.Join(installRoot, "bin", "claude-notifications-windows-amd64-"+stamp+".exe")
}

func InstallRuntime(sourceRoot, installRoot string) (string, error) {
	notifierPath := NewVersionedNotifierPath(installRoot)
	if err := copyFile(filepath.Join(sourceRoot, "bin", "claude-notifications-windows-amd64.exe"), notifierPath); err != nil {
		return "", err
	}

	files := []string{
		filepath.Join("bin", "sound-preview-windows-amd64.exe"),
		filepath.Join("bin", "list-sounds-windows-amd64.exe"),
		filepath.Join("bin", "list-devices-windows-amd64.exe"),
		filepath.Join("sounds", "task-complete.mp3"),
		filepath.Join("sounds", "review-complete.mp3"),
		filepath.Join("sounds", "question.mp3"),
		filepath.Join("sounds", "plan-ready.mp3"),
		filepath.Join("sounds", "error.mp3"),
		"claude_icon.png",
	}

	for _, rel := range files {
		src := filepath.Join(sourceRoot, rel)
		dst := filepath.Join(installRoot, rel)
		if err := copyFile(src, dst); err != nil {
			return "", err
		}
	}
	return notifierPath, nil
}

func SetRuntimeAssetPaths(cfg *config.Config, installRoot string) {
	cfg.Notifications.Desktop.AppIcon = filepath.Join(installRoot, "claude_icon.png")

	if cfg.Statuses == nil {
		cfg.Statuses = config.DefaultConfig().Statuses
	}

	sounds := map[string]string{
		"task_complete":         "task-complete.mp3",
		"review_complete":       "review-complete.mp3",
		"question":              "question.mp3",
		"plan_ready":            "plan-ready.mp3",
		"session_limit_reached": "error.mp3",
		"api_error":             "error.mp3",
		"api_error_overloaded":  "error.mp3",
	}
	for status, sound := range sounds {
		info := cfg.Statuses[status]
		info.Sound = filepath.Join(installRoot, "sounds", sound)
		cfg.Statuses[status] = info
	}
}

func WindowsClaudeHookSettings(exePath string) HookSettings {
	return HookSettings{
		Hooks: map[string][]HookMatcherGroup{
			"PreToolUse": {
				{
					Matcher: "ExitPlanMode|AskUserQuestion",
					Hooks:   []HookCommand{newClaudeHook(exePath, "PreToolUse")},
				},
			},
			"Notification": {
				{
					Matcher: "permission_prompt",
					Hooks:   []HookCommand{newClaudeHook(exePath, "Notification")},
				},
			},
			"Stop": {
				{
					Hooks: []HookCommand{newClaudeHook(exePath, "Stop")},
				},
			},
			"SubagentStop": {
				{
					Hooks: []HookCommand{newClaudeHook(exePath, "SubagentStop")},
				},
			},
			"TeammateIdle": {
				{
					Hooks: []HookCommand{newClaudeHook(exePath, "TeammateIdle")},
				},
			},
		},
	}
}

func CodexHookSettings(exePath string) HookSettings {
	return HookSettings{
		Hooks: map[string][]HookMatcherGroup{
			"Stop": {
				{
					Hooks: []HookCommand{newCodexHook(exePath, "Stop", "Sending Codex completion notification")},
				},
			},
			"PermissionRequest": {
				{
					Hooks: []HookCommand{newCodexHook(exePath, "PermissionRequest", "Sending Codex approval notification")},
				},
			},
			"SubagentStop": {
				{
					Hooks: []HookCommand{newCodexHook(exePath, "SubagentStop", "Sending Codex subagent notification")},
				},
			},
		},
	}
}

func MergeHookSettings(existing, generated HookSettings) HookSettings {
	if existing.Hooks == nil {
		existing.Hooks = map[string][]HookMatcherGroup{}
	}
	for hookName, groups := range generated.Hooks {
		existing.Hooks[hookName] = replaceManagedGroups(existing.Hooks[hookName], groups)
	}
	return existing
}

func ReadHookSettings(path string) (HookSettings, error) {
	var settings HookSettings
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return HookSettings{Hooks: map[string][]HookMatcherGroup{}}, nil
		}
		return settings, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return HookSettings{Hooks: map[string][]HookMatcherGroup{}}, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}
	if settings.Hooks == nil {
		settings.Hooks = map[string][]HookMatcherGroup{}
	}
	return settings, nil
}

func WriteHookSettings(path string, settings HookSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hook settings: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create hook config dir: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func CodexHooksPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "hooks.json"), nil
}

func ClaudeSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func newClaudeHook(exePath, hookName string) HookCommand {
	return HookCommand{
		Type:    "command",
		Command: exePath,
		Args:    []string{"handle-hook", hookName},
		Timeout: 30,
	}
}

func newCodexHook(exePath, hookName, statusMessage string) HookCommand {
	return HookCommand{
		Type:          "command",
		Command:       quoteCommandArg(exePath) + " handle-codex-hook " + hookName,
		Timeout:       30,
		StatusMessage: statusMessage,
	}
}

func quoteCommandArg(value string) string {
	if value == "" {
		return `""`
	}
	if !containsAny(value, " \t\"") {
		return value
	}
	return `"` + replaceAll(value, `"`, `\"`) + `"`
}

func replaceManagedGroups(existing, generated []HookMatcherGroup) []HookMatcherGroup {
	filtered := make([]HookMatcherGroup, 0, len(existing)+len(generated))
	for _, group := range existing {
		if isManagedGroup(group) {
			continue
		}
		filtered = append(filtered, group)
	}
	filtered = append(filtered, generated...)
	return filtered
}

func isManagedGroup(group HookMatcherGroup) bool {
	for _, hook := range group.Hooks {
		if contains(hook.Command, "claude-notifications") ||
			contains(hook.Command, "notification-installer") {
			return true
		}
		for _, arg := range hook.Args {
			if contains(arg, "claude-notifications") ||
				contains(arg, "handle-hook") ||
				contains(arg, "handle-codex-hook") {
				return true
			}
		}
	}
	return false
}

func boolPtr(value bool) *bool {
	return &value
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create destination dir for %s: %w", dst, err)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("open destination %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", dst, err)
	}
	return nil
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

func containsAny(s, chars string) bool {
	for _, r := range s {
		for _, c := range chars {
			if r == c {
				return true
			}
		}
	}
	return false
}

func replaceAll(s, old, new string) string {
	return string(bytes.ReplaceAll([]byte(s), []byte(old), []byte(new)))
}
