package codexhooks

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hzx-coder0/claude-codex-notifications/internal/analyzer"
)

func TestStatusForHook(t *testing.T) {
	tests := []struct {
		name      string
		eventName string
		data      HookData
		want      analyzer.Status
	}{
		{
			name:      "stop defaults to task complete",
			eventName: "Stop",
			want:      analyzer.StatusTaskComplete,
		},
		{
			name:      "permission request is question",
			eventName: "PermissionRequest",
			want:      analyzer.StatusQuestion,
		},
		{
			name:      "pre tool use is quiet",
			eventName: "PreToolUse",
			want:      analyzer.StatusUnknown,
		},
		{
			name:      "session limit",
			eventName: "Stop",
			data: HookData{
				LastAssistantMessage: "Session limit reached",
			},
			want: analyzer.StatusSessionLimitReached,
		},
		{
			name:      "authentication error",
			eventName: "Stop",
			data: HookData{
				LastAssistantMessage: "401 authentication_error",
			},
			want: analyzer.StatusAPIError,
		},
		{
			name:      "rate limit",
			eventName: "Stop",
			data: HookData{
				LastAssistantMessage: "API rate limit exceeded",
			},
			want: analyzer.StatusAPIErrorOverloaded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StatusForHook(tt.eventName, tt.data)
			if got != tt.want {
				t.Fatalf("StatusForHook() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestPermissionRequestMessageIncludesCommand(t *testing.T) {
	input, err := json.Marshal(map[string]string{"command": "go test ./..."})
	if err != nil {
		t.Fatal(err)
	}

	got := permissionRequestMessage(HookData{
		ToolName:  "shell_command",
		ToolInput: input,
	})

	if !strings.Contains(got, "shell_command") || !strings.Contains(got, "go test ./...") {
		t.Fatalf("permissionRequestMessage() = %q", got)
	}
}

func TestMessageForStopTruncatesLongAssistantMessage(t *testing.T) {
	body, _ := MessageForHook("Stop", HookData{
		LastAssistantMessage: strings.Repeat("a", maxAssistantMessageRunes+20),
	}, analyzer.StatusTaskComplete)

	if len([]rune(body)) != maxAssistantMessageRunes {
		t.Fatalf("body length = %d, want %d", len([]rune(body)), maxAssistantMessageRunes)
	}
	if !strings.HasSuffix(body, "...") {
		t.Fatalf("body should be ellipsized: %q", body)
	}
}

func TestWebhookMessageForStopKeepsFullAssistantMessage(t *testing.T) {
	full := strings.Repeat("飞书完整回复", 120)

	got := WebhookMessageForHook("Stop", HookData{
		LastAssistantMessage: full,
	}, analyzer.StatusTaskComplete)

	if got != full {
		t.Fatalf("WebhookMessageForHook() length = %d, want full length %d", len([]rune(got)), len([]rune(full)))
	}
}

func TestDecodeCodexHookKeepsUTF8Chinese(t *testing.T) {
	input := strings.NewReader(`{"session_id":"s","hook_event_name":"Stop","last_assistant_message":"中文不会变成问号"}`)
	var data HookData
	if err := json.NewDecoder(skipUTF8BOM(input)).Decode(&data); err != nil {
		t.Fatalf("decode error = %v", err)
	}

	if data.LastAssistantMessage != "中文不会变成问号" {
		t.Fatalf("LastAssistantMessage = %q", data.LastAssistantMessage)
	}
}
