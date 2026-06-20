package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/hzx-coder0/agent-notify-connect/internal/config"
	"rsc.io/qr"
)

type BindingOptions struct {
	Timeout         time.Duration
	ReceiveIDType   string
	ReceiveID       string
	QRImagePath     string
	OpenBrowser     bool
	PrintURL        bool
	PrintTerminalQR bool
	Out             io.Writer
}

type BindingResult struct {
	ConfigPath    string
	ReceiveIDType string
	ReceiveID     string
	Platform      string
}

func Bind(ctx context.Context, pluginRoot string, opts BindingOptions) (*BindingResult, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	if strings.TrimSpace(opts.ReceiveIDType) == "" {
		opts.ReceiveIDType = "open_id"
	}
	out := opts.Out
	if out == nil {
		out = io.Discard
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	reg := NewRegistrationClient()
	begin, err := reg.Begin(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(out, "Use Feishu/Lark mobile app to scan this QR code:")
	if opts.PrintURL {
		fmt.Fprintf(out, "URL: %s\n\n", begin.VerificationURIComplete)
	}
	if opts.PrintTerminalQR {
		if err := RenderTerminalQR(out, begin.VerificationURIComplete); err != nil {
			return nil, err
		}
		fmt.Fprintln(out)
	}
	if strings.TrimSpace(opts.QRImagePath) != "" {
		if err := SaveQRCodeImage(begin.VerificationURIComplete, strings.TrimSpace(opts.QRImagePath)); err != nil {
			return nil, fmt.Errorf("save QR image failed: %w", err)
		}
		fmt.Fprintf(out, "QR image: %s\n\n", strings.TrimSpace(opts.QRImagePath))
	}
	if opts.OpenBrowser {
		_ = OpenBrowser(begin.VerificationURIComplete)
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
			return nil, err
		}
		if poll.Status == "completed" {
			finalReceiveID := strings.TrimSpace(opts.ReceiveID)
			if finalReceiveID == "" {
				finalReceiveID = poll.OwnerOpenID
			}
			if finalReceiveID == "" {
				return nil, fmt.Errorf("receive_id is required because registration did not return owner_open_id")
			}
			result, err := SaveBinding(pluginRoot, poll, strings.TrimSpace(opts.ReceiveIDType), finalReceiveID)
			if err != nil {
				return nil, err
			}
			return result, nil
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("timed out waiting for QR onboarding result")
		case <-timer.C:
		}
	}

	return nil, fmt.Errorf("timed out waiting for QR onboarding result")
}

func SaveBinding(pluginRoot string, binding *RegistrationPollResult, receiveIDType, receiveID string) (*BindingResult, error) {
	cfg, err := config.LoadFromPluginRoot(pluginRoot)
	if err != nil {
		return nil, err
	}

	cfg.Notifications.Webhook.Enabled = true
	cfg.Notifications.Webhook.Preset = "feishu_app"
	cfg.Notifications.Webhook.Format = "json"
	cfg.Notifications.Feishu = config.FeishuConfig{
		Mode:          "app_registration",
		Platform:      binding.Platform,
		BaseURL:       binding.BaseURL,
		AppID:         binding.AppID,
		AppSecret:     binding.AppSecret,
		OwnerOpenID:   binding.OwnerOpenID,
		ReceiveIDType: receiveIDType,
		ReceiveID:     receiveID,
	}
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	configPath, err := config.GetStableConfigPath()
	if err != nil {
		return nil, err
	}
	if err := WriteConfigFile(configPath, cfg); err != nil {
		return nil, err
	}

	return &BindingResult{
		ConfigPath:    configPath,
		ReceiveIDType: receiveIDType,
		ReceiveID:     receiveID,
		Platform:      binding.Platform,
	}, nil
}

func WriteConfigFile(configPath string, cfg *config.Config) error {
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

func RenderTerminalQR(w io.Writer, content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("QR content is required")
	}
	code, err := qr.Encode(content, qr.M)
	if err != nil {
		return fmt.Errorf("encode QR: %w", err)
	}
	for y := -2; y <= code.Size+1; y += 2 {
		for x := -2; x <= code.Size+1; x++ {
			top := code.Black(x, y)
			bottom := code.Black(x, y+1)
			switch {
			case top && bottom:
				fmt.Fprint(w, "█")
			case top:
				fmt.Fprint(w, "▀")
			case bottom:
				fmt.Fprint(w, "▄")
			default:
				fmt.Fprint(w, " ")
			}
		}
		fmt.Fprintln(w)
	}
	return nil
}

func SaveQRCodeImage(content, path string) error {
	if strings.TrimSpace(content) == "" {
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

func OpenBrowser(rawURL string) error {
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
