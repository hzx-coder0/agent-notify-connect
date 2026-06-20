package feishu

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hzx-coder0/agent-notify-connect/internal/config"
)

func TestRenderTerminalQR(t *testing.T) {
	var out bytes.Buffer
	if err := RenderTerminalQR(&out, "https://example.com/connect"); err != nil {
		t.Fatalf("RenderTerminalQR() error = %v", err)
	}
	text := out.String()
	if !strings.ContainsAny(text, "█▀▄") {
		t.Fatalf("terminal QR should contain black blocks: %q", text)
	}
	if lines := strings.Count(text, "\n"); lines < 10 {
		t.Fatalf("terminal QR lines = %d, want at least 10", lines)
	}
}

func TestRenderTerminalQREmptyContent(t *testing.T) {
	var out bytes.Buffer
	if err := RenderTerminalQR(&out, " "); err == nil {
		t.Fatal("RenderTerminalQR() error = nil, want error")
	}
}

func TestWriteConfigFileOverwritesExistingConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	initial := []byte(`{"notifications":{"desktop":{"enabled":true}}}`)
	if err := os.WriteFile(configPath, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfigFile(configPath, config.DefaultConfig()); err != nil {
		t.Fatalf("WriteConfigFile() error = %v", err)
	}
	written, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(written, initial) {
		t.Fatalf("config was not overwritten: %s", written)
	}
}
