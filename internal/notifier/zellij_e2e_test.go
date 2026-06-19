package notifier

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
)

// TestZellijE2E runs end-to-end tests for zellij integration.
// It spawns a zellij session using a real PTY (via creack/pty).
// Requires: zellij installed, not CI, not already inside zellij.
func TestZellijE2E(t *testing.T) {
	zellijPath, err := exec.LookPath("zellij")
	if err != nil {
		t.Skip("zellij not found, skipping e2e tests")
	}

	// Skip in CI
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		t.Skip("Skipping zellij e2e tests in CI")
	}

	// Skip if already inside zellij (nested sessions cause issues)
	if os.Getenv("ZELLIJ") != "" {
		t.Skip("Already inside zellij, skipping e2e tests")
	}

	sessionName := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())

	// Create a layout file with 3 tabs
	layoutDir := t.TempDir()
	layoutFile := filepath.Join(layoutDir, "test-layout.kdl")
	layoutContent := `layout {
    tab name="default" focus=true {
        pane
    }
    tab name="editor" {
        pane
    }
    tab name="logs" {
        pane
    }
}
`
	if err := os.WriteFile(layoutFile, []byte(layoutContent), 0644); err != nil {
		t.Fatalf("Failed to write layout file: %v", err)
	}

	// Start zellij with a real PTY via creack/pty.
	// Use -s (session name) + -n (new-session-with-layout) to always create a new session.
	// --layout alone tries to attach to an existing session and fails if none exists.
	cmd := exec.Command(zellijPath, "-s", sessionName, "-n", layoutFile)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		t.Skipf("Failed to start zellij with PTY: %v", err)
	}

	t.Cleanup(func() {
		_ = exec.Command(zellijPath, "kill-session", sessionName).Run()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = ptmx.Close()
	})

	// Drain PTY output to prevent blocking
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := ptmx.Read(buf); err != nil {
				return
			}
		}
	}()

	// Poll until zellij session is ready (timeout 10s)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		out, err := exec.Command(zellijPath, "list-sessions").CombinedOutput()
		if err == nil && strings.Contains(string(out), sessionName) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Verify session is running
	out, err := exec.Command(zellijPath, "list-sessions").CombinedOutput()
	if err != nil || !strings.Contains(string(out), sessionName) {
		t.Skipf("zellij session %q did not start, skipping e2e. list-sessions: %s", sessionName, string(out))
	}

	// Save and set env vars for zellij detection
	oldZellij := os.Getenv("ZELLIJ")
	oldZellijSession := os.Getenv("ZELLIJ_SESSION_NAME")
	os.Setenv("ZELLIJ", "0")
	os.Setenv("ZELLIJ_SESSION_NAME", sessionName)
	t.Cleanup(func() {
		if oldZellij != "" {
			os.Setenv("ZELLIJ", oldZellij)
		} else {
			os.Unsetenv("ZELLIJ")
		}
		if oldZellijSession != "" {
			os.Setenv("ZELLIJ_SESSION_NAME", oldZellijSession)
		} else {
			os.Unsetenv("ZELLIJ_SESSION_NAME")
		}
	})

	t.Run("is_zellij_detection", func(t *testing.T) {
		if !IsZellij() {
			t.Error("IsZellij() should return true when ZELLIJ env is set")
		}
	})

	t.Run("dump_layout_parse", func(t *testing.T) {
		// Run dump-layout against our session
		dumpOut, err := exec.Command(zellijPath, "-s", sessionName, "action", "dump-layout").CombinedOutput()
		if err != nil {
			t.Fatalf("dump-layout failed: %v, output: %s", err, string(dumpOut))
		}

		tabName := parseActiveTabName(string(dumpOut))
		if tabName == "" {
			t.Fatalf("parseActiveTabName returned empty for dump-layout output:\n%s", string(dumpOut))
		}
		// Default focus should be on "default" tab
		if tabName != "default" {
			t.Errorf("expected active tab = %q, got %q", "default", tabName)
		}
	})

	t.Run("args_construction", func(t *testing.T) {
		args := buildZellijNotifierArgs("Title", "Message", "editor", sessionName, "com.test.app")

		if !containsArg(args, "-title", "Title") {
			t.Error("Missing -title")
		}
		if !containsArg(args, "-message", "Message") {
			t.Error("Missing -message")
		}
		if !containsArg(args, "-activate", "com.test.app") {
			t.Error("Missing -activate")
		}

		executeCmd := getArgValue(args, "-execute")
		if executeCmd == "" {
			t.Fatal("Missing -execute argument")
		}
		if !strings.Contains(executeCmd, sessionName) {
			t.Errorf("-execute should contain session name %q, got: %s", sessionName, executeCmd)
		}
		if !strings.Contains(executeCmd, "editor") {
			t.Errorf("-execute should contain tab name 'editor', got: %s", executeCmd)
		}
	})

	t.Run("click_switches_to_editor", func(t *testing.T) {
		args := buildZellijNotifierArgs("Test", "Msg", "editor", sessionName, "com.test.app")
		executeCmd := getArgValue(args, "-execute")
		if executeCmd == "" {
			t.Fatal("-execute argument is empty")
		}

		outBytes, err := exec.Command("/bin/sh", "-c", executeCmd).CombinedOutput()
		if err != nil {
			t.Fatalf("execute command failed: %v, output: %s", err, string(outBytes))
		}

		// Small delay for zellij to process
		time.Sleep(300 * time.Millisecond)

		verifyZellijActiveTab(t, zellijPath, sessionName, "editor")
	})

	t.Run("click_switches_to_logs", func(t *testing.T) {
		args := buildZellijNotifierArgs("Test", "Msg", "logs", sessionName, "com.test.app")
		executeCmd := getArgValue(args, "-execute")
		if executeCmd == "" {
			t.Fatal("-execute argument is empty")
		}

		outBytes, err := exec.Command("/bin/sh", "-c", executeCmd).CombinedOutput()
		if err != nil {
			t.Fatalf("execute command failed: %v, output: %s", err, string(outBytes))
		}

		time.Sleep(300 * time.Millisecond)

		verifyZellijActiveTab(t, zellijPath, sessionName, "logs")
	})

	t.Run("click_roundtrip", func(t *testing.T) {
		// Switch to default first
		switchZellijTab(t, zellijPath, sessionName, "default")
		verifyZellijActiveTab(t, zellijPath, sessionName, "default")

		// default → editor
		switchZellijTab(t, zellijPath, sessionName, "editor")
		verifyZellijActiveTab(t, zellijPath, sessionName, "editor")

		// editor → logs
		switchZellijTab(t, zellijPath, sessionName, "logs")
		verifyZellijActiveTab(t, zellijPath, sessionName, "logs")

		// logs → editor
		switchZellijTab(t, zellijPath, sessionName, "editor")
		verifyZellijActiveTab(t, zellijPath, sessionName, "editor")
	})
}

// --- helpers ---

// switchZellijTab switches to a tab by running the go-to-tab-name action.
func switchZellijTab(t *testing.T, zellijPath, sessionName, tabName string) {
	t.Helper()
	out, err := exec.Command(zellijPath, "-s", sessionName, "action", "go-to-tab-name", tabName).CombinedOutput()
	if err != nil {
		t.Fatalf("go-to-tab-name %q failed: %v, output: %s", tabName, err, string(out))
	}
	time.Sleep(300 * time.Millisecond)
}

// verifyZellijActiveTab checks the active tab matches expected by parsing dump-layout.
func verifyZellijActiveTab(t *testing.T, zellijPath, sessionName, expected string) {
	t.Helper()
	out, err := exec.Command(zellijPath, "-s", sessionName, "action", "dump-layout").CombinedOutput()
	if err != nil {
		t.Fatalf("dump-layout failed: %v, output: %s", err, string(out))
	}
	active := parseActiveTabName(string(out))
	if active != expected {
		t.Errorf("active tab = %q, want %q\nlayout:\n%s", active, expected, string(out))
	}
}
