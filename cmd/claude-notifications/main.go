package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/hzx-coder0/claude-codex-notifications/internal/audio"
	"github.com/hzx-coder0/claude-codex-notifications/internal/codexhooks"
	"github.com/hzx-coder0/claude-codex-notifications/internal/config"
	"github.com/hzx-coder0/claude-codex-notifications/internal/errorhandler"
	"github.com/hzx-coder0/claude-codex-notifications/internal/feishu"
	"github.com/hzx-coder0/claude-codex-notifications/internal/hooks"
	"github.com/hzx-coder0/claude-codex-notifications/internal/installer"
	"github.com/hzx-coder0/claude-codex-notifications/internal/logging"
	"github.com/hzx-coder0/claude-codex-notifications/internal/notifier"
	"github.com/hzx-coder0/claude-codex-notifications/internal/webhook"
	"rsc.io/qr"
)

const version = "1.0.0"
const windowsLazyUpdateRetryAfter = time.Hour

var (
	currentGOOS               = runtime.GOOS
	scheduleWindowsLazyUpdate = scheduleWindowsLazyUpdateImpl
)

func main() {
	// Initialize global error handler with panic recovery
	// logToConsole=true: errors will be shown in console
	// exitOnCritical=false: don't exit on critical errors (let caller decide)
	// recoveryEnabled=true: recover from panics
	errorhandler.Init(true, false, true)

	// Add global panic recovery
	defer errorhandler.HandlePanic()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "handle-hook":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: hook event name required\n")
			printUsage()
			os.Exit(1)
		}
		handleHook(os.Args[2])
	case "handle-codex-hook":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: Codex hook event name required\n")
			printUsage()
			os.Exit(1)
		}
		handleCodexHook(os.Args[2])
	case "focus-window":
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "Error: focus-window requires bundleID and cwd arguments\n")
			os.Exit(1)
		}
		opts, err := parseFocusWindowOptions(os.Args[4:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "focus-window: %v\n", err)
			os.Exit(1)
		}
		if err := notifier.FocusAppWindowWithOptions(os.Args[2], os.Args[3], opts); err != nil {
			fmt.Fprintf(os.Stderr, "focus-window: %v\n", err)
			os.Exit(1)
		}
	case "play-sound":
		runPlaySound(os.Args[2:])
	case "daemon", "--daemon":
		runDaemon()
	case "windows-hooks":
		runWindowsHooks(os.Args[2:])
	case "codex-hooks":
		runCodexHooks(os.Args[2:])
	case "install-hooks":
		runInstallHooks(os.Args[2:])
	case "feishu":
		runFeishu(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("claude-notifications v%s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

type hookSettings struct {
	Hooks map[string][]hookMatcherGroup `json:"hooks"`
}

type hookMatcherGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookCommand `json:"hooks"`
}

type hookCommand struct {
	Type          string   `json:"type,omitempty"`
	Command       string   `json:"command"`
	Args          []string `json:"args,omitempty"`
	Timeout       int      `json:"timeout"`
	Shell         string   `json:"shell,omitempty"`
	StatusMessage string   `json:"statusMessage,omitempty"`
}

func runWindowsHooks(args []string) {
	exePath, err := parseWindowsHooksExecutable(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "windows-hooks: %v\n", err)
		os.Exit(1)
	}

	settings := newWindowsHookSettings(exePath)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "windows-hooks: failed to render JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}

func newWindowsHookSettings(exePath string) hookSettings {
	return hookSettings{
		Hooks: map[string][]hookMatcherGroup{
			"PreToolUse": {
				{
					Matcher: "ExitPlanMode|AskUserQuestion",
					Hooks:   []hookCommand{newExecHook(exePath, "PreToolUse")},
				},
			},
			"Notification": {
				{
					Matcher: "permission_prompt",
					Hooks:   []hookCommand{newExecHook(exePath, "Notification")},
				},
			},
			"Stop": {
				{
					Hooks: []hookCommand{newExecHook(exePath, "Stop")},
				},
			},
			"SubagentStop": {
				{
					Hooks: []hookCommand{newExecHook(exePath, "SubagentStop")},
				},
			},
			"TeammateIdle": {
				{
					Hooks: []hookCommand{newExecHook(exePath, "TeammateIdle")},
				},
			},
		},
	}
}

func parseWindowsHooksExecutable(args []string) (string, error) {
	exeOverride := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--exe":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--exe requires a path")
			}
			i++
			exeOverride = args[i]
		default:
			return "", fmt.Errorf("unknown windows-hooks option: %s", args[i])
		}
	}

	if exeOverride != "" {
		return filepath.Abs(exeOverride)
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to detect executable path: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	if strings.EqualFold(filepath.Ext(exePath), ".exe") {
		return exePath, nil
	}

	pluginRoot := getPluginRoot()
	return filepath.Abs(filepath.Join(pluginRoot, "bin", "claude-notifications-windows-amd64.exe"))
}

func newExecHook(exePath, hookName string) hookCommand {
	return hookCommand{
		Type:    "command",
		Command: exePath,
		Args:    []string{"handle-hook", hookName},
		Timeout: 30,
	}
}

func runCodexHooks(args []string) {
	exePath, err := parseCodexHooksExecutable(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex-hooks: %v\n", err)
		os.Exit(1)
	}

	settings := newCodexHookSettings(exePath)
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex-hooks: failed to render JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}

func parseCodexHooksExecutable(args []string) (string, error) {
	exeOverride := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--exe":
			if i+1 >= len(args) {
				return "", fmt.Errorf("--exe requires a path")
			}
			i++
			exeOverride = args[i]
		default:
			return "", fmt.Errorf("unknown codex-hooks option: %s", args[i])
		}
	}

	if exeOverride != "" {
		return filepath.Abs(exeOverride)
	}

	exePath, err := os.Executable()
	if err == nil {
		if abs, absErr := filepath.Abs(exePath); absErr == nil {
			return abs, nil
		}
	}

	pluginRoot := getPluginRoot()
	if currentGOOS == "windows" {
		return filepath.Abs(filepath.Join(pluginRoot, "bin", "claude-notifications-windows-amd64.exe"))
	}
	return filepath.Abs(filepath.Join(pluginRoot, "bin", "claude-notifications"))
}

func newCodexHookSettings(exePath string) hookSettings {
	return hookSettings{
		Hooks: map[string][]hookMatcherGroup{
			"Stop": {
				{
					Hooks: []hookCommand{newCodexCommandHook(exePath, "Stop", "Sending Codex completion notification")},
				},
			},
			"PermissionRequest": {
				{
					Hooks: []hookCommand{newCodexCommandHook(exePath, "PermissionRequest", "Sending Codex approval notification")},
				},
			},
			"SubagentStop": {
				{
					Hooks: []hookCommand{newCodexCommandHook(exePath, "SubagentStop", "Sending Codex subagent notification")},
				},
			},
		},
	}
}

func newCodexCommandHook(exePath, hookName, statusMessage string) hookCommand {
	return hookCommand{
		Type:          "command",
		Command:       quoteCommandArg(exePath) + " handle-codex-hook " + hookName,
		Timeout:       30,
		StatusMessage: statusMessage,
	}
}

func runInstallHooks(args []string) {
	fs := flag.NewFlagSet("install-hooks", flag.ExitOnError)
	exeOverride := fs.String("exe", "", "Path to claude-notifications executable")
	claudeEnabled := fs.Bool("claude", true, "Write Claude Code hooks to ~/.claude/settings.json")
	codexEnabled := fs.Bool("codex", true, "Write Codex hooks to ~/.codex/hooks.json")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "install-hooks: %v\n", err)
		os.Exit(1)
	}

	exePath := strings.TrimSpace(*exeOverride)
	if exePath == "" {
		if detected, err := parseCodexHooksExecutable(nil); err == nil {
			exePath = detected
		} else {
			fmt.Fprintf(os.Stderr, "install-hooks: %v\n", err)
			os.Exit(1)
		}
	}
	absExe, err := filepath.Abs(exePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "install-hooks: %v\n", err)
		os.Exit(1)
	}

	if *claudeEnabled {
		path, err := installer.ClaudeSettingsPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "install-hooks: %v\n", err)
			os.Exit(1)
		}
		if err := mergeAndWriteHookSettings(path, installer.WindowsClaudeHookSettings(absExe)); err != nil {
			fmt.Fprintf(os.Stderr, "install-hooks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Claude hooks written: %s\n", path)
	}

	if *codexEnabled {
		path, err := installer.CodexHooksPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "install-hooks: %v\n", err)
			os.Exit(1)
		}
		if err := mergeAndWriteHookSettings(path, installer.CodexHookSettings(absExe)); err != nil {
			fmt.Fprintf(os.Stderr, "install-hooks: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Codex hooks written: %s\n", path)
	}
}

func mergeAndWriteHookSettings(path string, generated installer.HookSettings) error {
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

func quoteCommandArg(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func handleHook(hookEvent string) {
	// Add panic recovery for this function
	defer errorhandler.HandlePanic()

	// Determine plugin root
	pluginRoot := getPluginRoot()
	maybeScheduleWindowsLazyUpdate(pluginRoot)

	// Initialize logger
	if _, err := logging.InitLogger(pluginRoot); err != nil {
		errorhandler.HandleCriticalError(err, "Failed to initialize logger")
		os.Exit(1)
	}
	defer logging.Close()

	// Create handler
	handler, err := hooks.NewHandler(pluginRoot)
	if err != nil {
		errorhandler.HandleCriticalError(err, "Failed to create handler")
		os.Exit(1)
	}

	// Handle hook
	if err := handler.HandleHook(hookEvent, os.Stdin); err != nil {
		errorhandler.HandleCriticalError(err, "Failed to handle hook")
		os.Exit(1)
	}
}

func handleCodexHook(hookEvent string) {
	defer errorhandler.HandlePanic()

	pluginRoot := getPluginRoot()
	maybeScheduleWindowsLazyUpdate(pluginRoot)

	if _, err := logging.InitLogger(pluginRoot); err != nil {
		errorhandler.HandleCriticalError(err, "Failed to initialize logger")
		os.Exit(1)
	}
	defer logging.Close()

	cfg, err := config.LoadFromPluginRoot(pluginRoot)
	if err != nil {
		errorhandler.HandleCriticalError(err, "Failed to load config")
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		errorhandler.HandleCriticalError(err, "Invalid config")
		os.Exit(1)
	}

	handler := codexhooks.NewHandler(cfg, notifier.New(cfg), webhook.New(cfg))

	if err := handler.HandleHook(hookEvent, os.Stdin); err != nil {
		errorhandler.HandleCriticalError(err, "Failed to handle Codex hook")
		os.Exit(1)
	}
}

func runFeishu(args []string) {
	if len(args) < 1 {
		printFeishuUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "bind":
		runFeishuBind(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "feishu: unknown command: %s\n", args[0])
		printFeishuUsage()
		os.Exit(1)
	}
}

func runFeishuBind(args []string) {
	fs := flag.NewFlagSet("feishu bind", flag.ExitOnError)
	timeoutSeconds := fs.Int("timeout", 600, "QR onboarding timeout in seconds")
	receiveIDType := fs.String("receive-id-type", "open_id", "Feishu receive_id_type: open_id, chat_id, user_id, email, union_id")
	receiveID := fs.String("receive-id", "", "Feishu receive_id; defaults to scanned user's open_id")
	qrImage := fs.String("qr-image", "", "Save QR code as PNG image to this path")
	noBrowser := fs.Bool("no-browser", false, "Do not open the QR onboarding URL in the browser")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "feishu bind: %v\n", err)
		os.Exit(1)
	}

	if *timeoutSeconds <= 0 {
		fmt.Fprintln(os.Stderr, "feishu bind: --timeout must be > 0")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutSeconds)*time.Second)
	defer cancel()

	reg := feishu.NewRegistrationClient()
	begin, err := reg.Begin(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "feishu bind: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Use Feishu/Lark mobile app to scan this QR code:")
	fmt.Printf("URL: %s\n\n", begin.VerificationURIComplete)
	if strings.TrimSpace(*qrImage) != "" {
		if err := saveQRCodeImage(begin.VerificationURIComplete, strings.TrimSpace(*qrImage)); err != nil {
			fmt.Fprintf(os.Stderr, "feishu bind: save QR image failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("QR image: %s\n\n", strings.TrimSpace(*qrImage))
	}
	if !*noBrowser {
		_ = openBrowser(begin.VerificationURIComplete)
	}

	interval := time.Duration(begin.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(begin.ExpireIn) * time.Second)
	if timeoutDeadline, ok := ctx.Deadline(); ok && timeoutDeadline.Before(deadline) {
		deadline = timeoutDeadline
	}

	for time.Now().Before(deadline) {
		poll, err := reg.Poll(ctx, begin.DeviceCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "feishu bind: %v\n", err)
			os.Exit(1)
		}
		if poll.Status == "completed" {
			finalReceiveID := strings.TrimSpace(*receiveID)
			if finalReceiveID == "" {
				finalReceiveID = poll.OwnerOpenID
			}
			if finalReceiveID == "" {
				fmt.Fprintln(os.Stderr, "feishu bind: receive_id is required because registration did not return owner_open_id")
				os.Exit(1)
			}
			if err := saveFeishuBinding(getPluginRoot(), poll, strings.TrimSpace(*receiveIDType), finalReceiveID); err != nil {
				fmt.Fprintf(os.Stderr, "feishu bind: save config failed: %v\n", err)
				os.Exit(1)
			}
			configPath, _ := config.GetStableConfigPath()
			fmt.Println("Feishu binding saved.")
			fmt.Printf("Config: %s\n", configPath)
			fmt.Printf("Preset: feishu_app, receive_id_type=%s\n", strings.TrimSpace(*receiveIDType))
			return
		}
		time.Sleep(interval)
	}

	fmt.Fprintln(os.Stderr, "feishu bind: timed out waiting for QR onboarding result")
	os.Exit(1)
}

func saveFeishuBinding(pluginRoot string, binding *feishu.RegistrationPollResult, receiveIDType, receiveID string) error {
	cfg, err := config.LoadFromPluginRoot(pluginRoot)
	if err != nil {
		return err
	}

	cfg.Notifications.Webhook.Enabled = true
	cfg.Notifications.Webhook.Preset = "feishu_app"
	cfg.Notifications.Webhook.Format = "json"
	cfg.Notifications.Feishu = config.FeishuConfig{
		Mode:          "app_registration",
		Platform:      binding.Platform,
		AppID:         binding.AppID,
		AppSecret:     binding.AppSecret,
		OwnerOpenID:   binding.OwnerOpenID,
		ReceiveIDType: receiveIDType,
		ReceiveID:     receiveID,
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}

	configPath, err := config.GetStableConfigPath()
	if err != nil {
		return err
	}
	return writeConfigFile(configPath, cfg)
}

func writeConfigFile(configPath string, cfg *config.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(configPath), "config-*.json.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(configPath)
	}
	return os.Rename(tmpPath, configPath)
}

func printFeishuUsage() {
	fmt.Println("Usage:")
	fmt.Println("  claude-notifications feishu bind [--timeout 600] [--receive-id-type open_id] [--receive-id <id>] [--qr-image <path>] [--no-browser]")
}

func saveQRCodeImage(content, path string) error {
	if content == "" {
		return fmt.Errorf("QR content is required")
	}
	code, err := qr.Encode(content, qr.M)
	if err != nil {
		return fmt.Errorf("encode QR: %w", err)
	}
	code.Scale = 8
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, code.PNG(), 0o644)
}

func openBrowser(rawURL string) error {
	if rawURL == "" {
		return nil
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

type pluginManifest struct {
	Version string `json:"version"`
}

func maybeScheduleWindowsLazyUpdate(pluginRoot string) {
	if currentGOOS != "windows" {
		return
	}

	pluginVersion := readPluginManifestVersion(pluginRoot)
	if pluginVersion == "" || pluginVersion == version {
		return
	}

	stampKey := version + "->" + pluginVersion
	stampPath := windowsLazyUpdateStampPath(pluginRoot)
	if windowsLazyUpdateRecentlyScheduled(stampPath, stampKey) {
		return
	}

	stampWritten := writeWindowsLazyUpdateStamp(stampPath, stampKey) == nil
	if err := scheduleWindowsLazyUpdate(pluginRoot); err != nil && stampWritten {
		_ = os.Remove(stampPath)
	}
}

func readPluginManifestVersion(pluginRoot string) string {
	data, err := os.ReadFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"))
	if err != nil {
		return ""
	}

	var manifest pluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ""
	}
	return manifest.Version
}

func windowsLazyUpdateStampPath(pluginRoot string) string {
	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		cacheDir = filepath.Join(pluginRoot, ".cache")
	}
	return filepath.Join(cacheDir, "claude-notifications-go", "windows-lazy-update-stamp")
}

func windowsLazyUpdateRecentlyScheduled(stampPath, stampKey string) bool {
	info, err := os.Stat(stampPath)
	if err != nil {
		return false
	}
	if time.Since(info.ModTime()) > windowsLazyUpdateRetryAfter {
		return false
	}

	data, err := os.ReadFile(stampPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == stampKey
}

func writeWindowsLazyUpdateStamp(stampPath, stampKey string) error {
	if err := os.MkdirAll(filepath.Dir(stampPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(stampPath, []byte(stampKey+"\n"), 0o600)
}

func scheduleWindowsLazyUpdateImpl(pluginRoot string) error {
	installScript := filepath.Join(pluginRoot, "bin", "install.sh")
	if _, err := os.Stat(installScript); err != nil {
		return err
	}

	powershellPath, err := findWindowsPowerShell()
	if err != nil {
		return err
	}

	bashPath, err := findWindowsBash()
	if err != nil {
		return err
	}

	targetDir := filepath.ToSlash(filepath.Join(pluginRoot, "bin"))
	installScript = filepath.ToSlash(installScript)
	shCommand := "INSTALL_TARGET_DIR=" + shellSingleQuoted(targetDir) + " " + shellSingleQuoted(installScript) + " --force"
	psCommand := "$ErrorActionPreference = 'SilentlyContinue'; " +
		"Start-Sleep -Milliseconds 750; " +
		"for ($i = 0; $i -lt 6; $i++) { " +
		"& " + powershellSingleQuoted(bashPath) + " -lc " + powershellSingleQuoted(shCommand) + " *> $null; " +
		"if ($LASTEXITCODE -eq 0) { break }; " +
		"Start-Sleep -Seconds 5 }"

	cmd := exec.Command(powershellPath, "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCommand)
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err == nil {
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}

	if err := cmd.Start(); err != nil {
		if devNull != nil {
			_ = devNull.Close()
		}
		return err
	}

	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	if devNull != nil {
		_ = devNull.Close()
	}
	return nil
}

func findWindowsPowerShell() (string, error) {
	if path, err := exec.LookPath("powershell.exe"); err == nil {
		return path, nil
	}

	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		candidate := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("powershell.exe not found")
}

func findWindowsBash() (string, error) {
	if override := os.Getenv("CLAUDE_NOTIFICATIONS_BASH"); override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		return "", fmt.Errorf("CLAUDE_NOTIFICATIONS_BASH not found: %s", override)
	}

	for _, name := range []string{"bash.exe", "bash"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	for _, candidate := range windowsBashCandidates() {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("bash.exe not found")
}

func windowsBashCandidates() []string {
	var candidates []string
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
		if root == "" {
			continue
		}
		candidates = append(candidates,
			filepath.Join(root, "Git", "bin", "bash.exe"),
			filepath.Join(root, "Programs", "Git", "bin", "bash.exe"),
		)
	}
	return candidates
}

func shellSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func powershellSingleQuoted(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func getPluginRoot() string {
	// Try CLAUDE_PLUGIN_ROOT environment variable first
	if root := os.Getenv("CLAUDE_PLUGIN_ROOT"); root != "" {
		return root
	}

	// Try to find plugin root relative to executable
	exe, err := os.Executable()
	if err == nil {
		// Executable is in bin/, so plugin root is parent directory
		exeDir := filepath.Dir(exe)
		if filepath.Base(exeDir) == "bin" {
			return filepath.Dir(exeDir)
		}
		// Otherwise, try parent of executable dir
		return filepath.Dir(exeDir)
	}

	// Fallback to current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// runPlaySound plays a sound file and exits. Designed to be spawned as a detached
// child process so the parent hook process does not wait for audio to finish.
// Usage: play-sound <path> [--volume <0.0-1.0>] [--device <name>]
func runPlaySound(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "play-sound: sound file path required\n")
		os.Exit(1)
	}

	soundPath := args[0]
	volume := 1.0
	deviceName := ""

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--volume":
			if i+1 < len(args) {
				i++
				if v, err := strconv.ParseFloat(args[i], 64); err == nil {
					volume = v
				}
			}
		case "--device":
			if i+1 < len(args) {
				i++
				deviceName = args[i]
			}
		}
	}

	player, err := audio.NewPlayer(deviceName, volume)
	if err != nil {
		fmt.Fprintf(os.Stderr, "play-sound: failed to init player: %v\n", err)
		os.Exit(1)
	}
	defer player.Close()

	if err := player.Play(soundPath); err != nil {
		fmt.Fprintf(os.Stderr, "play-sound: failed to play %s: %v\n", soundPath, err)
		os.Exit(1)
	}
}

func parseFocusWindowOptions(args []string) (notifier.FocusWindowOptions, error) {
	var opts notifier.FocusWindowOptions

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ghostty-terminal-id":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--ghostty-terminal-id requires a value")
			}
			i++
			opts.GhosttyTerminalID = args[i]
		default:
			return opts, fmt.Errorf("unknown focus-window option: %s", args[i])
		}
	}

	return opts, nil
}

func printUsage() {
	fmt.Println("claude-notifications - Smart notifications for Claude Code")
	fmt.Println()
	fmt.Printf("Version: %s\n", version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  claude-notifications handle-hook <HookName>")
	fmt.Println("  claude-notifications handle-codex-hook <HookName>")
	fmt.Println("  claude-notifications daemon")
	fmt.Println("  claude-notifications windows-hooks [--exe <path>]")
	fmt.Println("  claude-notifications codex-hooks [--exe <path>]")
	fmt.Println("  claude-notifications install-hooks [--exe <path>] [--claude=true] [--codex=true]")
	fmt.Println("  claude-notifications feishu bind [--timeout 600] [--receive-id-type open_id] [--receive-id <id>]")
	fmt.Println("  claude-notifications version")
	fmt.Println("  claude-notifications help")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  handle-hook <HookName>  Handle a Claude Code hook event")
	fmt.Println("                          HookName: PreToolUse, Stop, SubagentStop, Notification")
	fmt.Println("  handle-codex-hook <HookName>")
	fmt.Println("                          Handle a Codex hook event")
	fmt.Println("                          HookName: Stop, PermissionRequest, SubagentStop")
	fmt.Println("  daemon                  Run the notification daemon (Linux only)")
	fmt.Println("                          For click-to-focus support on desktop notifications")
	fmt.Println("  focus-window <bundleID> <cwd> [--ghostty-terminal-id <id>]")
	fmt.Println("                          Focus specific app window (internal, used by click-to-focus)")
	fmt.Println("  windows-hooks           Print exec-form hook JSON for Windows settings")
	fmt.Println("                          Does not modify ~/.claude/settings.json")
	fmt.Println("  codex-hooks             Print Codex hook JSON for ~/.codex/hooks.json")
	fmt.Println("  install-hooks           Merge Claude Code and Codex hook JSON into user config files")
	fmt.Println("  feishu bind             Bind Feishu/Lark app notifications through QR onboarding")
	fmt.Println("  version                 Show version information")
	fmt.Println("  help                    Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Handle PreToolUse hook (reads JSON from stdin)")
	fmt.Println("  echo '{\"session_id\":\"test\",\"tool_name\":\"ExitPlanMode\"}' | claude-notifications handle-hook PreToolUse")
	fmt.Println()
	fmt.Println("  # Handle Stop hook")
	fmt.Println("  echo '{\"session_id\":\"test\",\"transcript_path\":\"/path/to/transcript.jsonl\"}' | claude-notifications handle-hook Stop")
	fmt.Println()
	fmt.Println("  # Run notification daemon (Linux only, started automatically)")
	fmt.Println("  claude-notifications daemon")
	fmt.Println()
	fmt.Println("  # Print Windows exec-form hook configuration")
	fmt.Println("  claude-notifications windows-hooks")
	fmt.Println()
	fmt.Println("  # Print Codex hook configuration")
	fmt.Println("  claude-notifications codex-hooks")
	fmt.Println()
	fmt.Println("  # Install hooks into user config files")
	fmt.Println("  claude-notifications install-hooks")
	fmt.Println()
	fmt.Println("  # Bind Feishu/Lark app notifications")
	fmt.Println("  claude-notifications feishu bind")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  CLAUDE_PLUGIN_ROOT  Plugin root directory (auto-detected if not set)")
	fmt.Println()
}
