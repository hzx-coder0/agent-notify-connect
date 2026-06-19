package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hzx-coder0/claude-codex-notifications/internal/config"
)

func TestApplyProfileSetsGlobalAndPerStatusOptions(t *testing.T) {
	cfg := config.DefaultConfig()

	ApplyProfile(cfg, Profile{
		DesktopEnabled: false,
		DesktopSound:   false,
		WebhookEnabled: true,
		StatusEnabled: map[string]bool{
			"task_complete": false,
			"question":      true,
		},
		StatusDesktop: map[string]bool{
			"question": false,
		},
		StatusWebhook: map[string]bool{
			"question": true,
		},
	})

	if cfg.Notifications.Desktop.Enabled {
		t.Fatal("desktop should be disabled")
	}
	if cfg.Notifications.Desktop.Sound {
		t.Fatal("desktop sound should be disabled")
	}
	if !cfg.Notifications.Webhook.Enabled {
		t.Fatal("webhook should be enabled")
	}
	if cfg.IsStatusEnabled("task_complete") {
		t.Fatal("task_complete status should be disabled")
	}
	if !cfg.IsStatusEnabled("question") {
		t.Fatal("question status should be enabled")
	}
	if cfg.IsStatusDesktopEnabled("question") {
		t.Fatal("question desktop channel should be disabled")
	}
	if !cfg.IsStatusWebhookEnabled("question") {
		t.Fatal("question webhook channel should be enabled")
	}
}

func TestMergeHookSettingsReplacesManagedGroupsOnly(t *testing.T) {
	existing := HookSettings{
		Hooks: map[string][]HookMatcherGroup{
			"Stop": {
				{
					Hooks: []HookCommand{{Type: "command", Command: "custom-tool.exe", Timeout: 10}},
				},
				{
					Hooks: []HookCommand{{Type: "command", Command: "old-claude-notifications.exe handle-codex-hook Stop", Timeout: 30}},
				},
			},
		},
	}
	generated := CodexHookSettings(`C:\Tools\claude-notifications-windows-amd64.exe`)

	merged := MergeHookSettings(existing, generated)
	groups := merged.Hooks["Stop"]
	if len(groups) != 2 {
		t.Fatalf("Stop groups = %d, want 2", len(groups))
	}
	if groups[0].Hooks[0].Command != "custom-tool.exe" {
		t.Fatalf("custom hook was not preserved: %#v", groups)
	}
	if groups[1].Hooks[0].Command == "old-claude-notifications.exe handle-codex-hook Stop" {
		t.Fatalf("old managed hook was not replaced: %#v", groups)
	}
}

func TestReadWriteHookSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".codex", "hooks.json")
	settings := CodexHookSettings(`C:\Tools\claude-notifications-windows-amd64.exe`)

	if err := WriteHookSettings(path, settings); err != nil {
		t.Fatalf("WriteHookSettings() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	read, err := ReadHookSettings(path)
	if err != nil {
		t.Fatalf("ReadHookSettings() error = %v", err)
	}
	if len(read.Hooks["Stop"]) != 1 {
		t.Fatalf("Stop groups = %d, want 1", len(read.Hooks["Stop"]))
	}
}

func TestHasFeishuAppBinding(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Notifications.Webhook.Enabled = true
	cfg.Notifications.Webhook.Preset = "feishu_app"
	cfg.Notifications.Feishu = config.FeishuConfig{
		AppID:         "cli_xxx",
		AppSecret:     "sec_xxx",
		ReceiveIDType: "open_id",
		ReceiveID:     "ou_xxx",
	}

	if !HasFeishuAppBinding(cfg) {
		t.Fatal("expected valid feishu app binding")
	}

	cfg.Notifications.Feishu.AppSecret = ""
	if HasFeishuAppBinding(cfg) {
		t.Fatal("expected missing secret to be unbound")
	}
}

func TestInstallRuntimeCopiesRequiredFiles(t *testing.T) {
	source := t.TempDir()
	target := t.TempDir()
	required := []string{
		filepath.Join("bin", "claude-notifications-windows-amd64.exe"),
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
	for _, rel := range required {
		path := filepath.Join(source, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(rel), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	notifierPath, err := InstallRuntime(source, target)
	if err != nil {
		t.Fatalf("InstallRuntime() error = %v", err)
	}
	if notifierPath == "" {
		t.Fatal("notifierPath is empty")
	}

	for _, rel := range required {
		if rel == filepath.Join("bin", "claude-notifications-windows-amd64.exe") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != rel {
			t.Fatalf("%s copied content = %q, want %q", rel, data, rel)
		}
	}

	data, err := os.ReadFile(notifierPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != filepath.Join("bin", "claude-notifications-windows-amd64.exe") {
		t.Fatalf("notifier content = %q", data)
	}
}
