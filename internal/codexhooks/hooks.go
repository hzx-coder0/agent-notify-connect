package codexhooks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hzx-coder0/claude-codex-notifications/internal/analyzer"
	"github.com/hzx-coder0/claude-codex-notifications/internal/config"
	"github.com/hzx-coder0/claude-codex-notifications/internal/errorhandler"
	"github.com/hzx-coder0/claude-codex-notifications/internal/logging"
	"github.com/hzx-coder0/claude-codex-notifications/internal/notification"
)

const maxAssistantMessageRunes = 500

// HookData represents the JSON payload received from Codex hooks.
type HookData struct {
	SessionID            string          `json:"session_id"`
	TurnID               string          `json:"turn_id"`
	TranscriptPath       string          `json:"transcript_path"`
	CWD                  string          `json:"cwd"`
	HookEventName        string          `json:"hook_event_name"`
	Model                string          `json:"model"`
	PermissionMode       string          `json:"permission_mode"`
	ToolName             string          `json:"tool_name,omitempty"`
	ToolInput            json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID            string          `json:"tool_use_id,omitempty"`
	StopHookActive       bool            `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string          `json:"last_assistant_message,omitempty"`
}

// Handler handles Codex hook events.
type Handler struct {
	cfg        *config.Config
	desktopSvc notification.DesktopSender
	webhookSvc notification.WebhookSender
}

func NewHandler(cfg *config.Config, desktopSvc notification.DesktopSender, webhookSvc notification.WebhookSender) *Handler {
	return &Handler{
		cfg:        cfg,
		desktopSvc: desktopSvc,
		webhookSvc: webhookSvc,
	}
}

// HandleHook handles one Codex hook event from stdin.
func (h *Handler) HandleHook(hookEvent string, input io.Reader) error {
	defer errorhandler.HandlePanic()

	defer func() {
		if err := h.desktopSvc.Close(); err != nil {
			logging.Warn("Failed to close notifier: %v", err)
		}
	}()

	defer func() {
		if err := h.webhookSvc.Shutdown(5 * time.Second); err != nil {
			logging.Warn("Failed to shutdown webhook sender: %v", err)
		}
	}()

	logging.SetPrefix(fmt.Sprintf("PID:%d", os.Getpid()))
	logging.Debug("=== Codex hook triggered: %s ===", hookEvent)

	var hookData HookData
	if err := json.NewDecoder(skipUTF8BOM(input)).Decode(&hookData); err != nil {
		return fmt.Errorf("failed to parse Codex hook data: %w", err)
	}

	if hookData.SessionID == "" {
		hookData.SessionID = "unknown"
		logging.Warn("Codex session ID is empty, using 'unknown'")
	}

	if hookEvent == "" {
		hookEvent = hookData.HookEventName
	}
	if hookEvent == "" {
		return fmt.Errorf("Codex hook event name is required")
	}

	if !h.cfg.IsAnyNotificationEnabled() {
		logging.Debug("All notifications disabled, exiting")
		return nil
	}

	status := StatusForHook(hookEvent, hookData)
	if status == analyzer.StatusUnknown {
		logging.Debug("Codex status is unknown, skipping notification")
		return nil
	}

	body, actions := MessageForHook(hookEvent, hookData, status)
	webhookBody := WebhookMessageForHook(hookEvent, hookData, status)
	service := notification.New(h.cfg, h.desktopSvc, h.webhookSvc)
	service.SendWithOptions(notification.SendOptions{
		Status:      status,
		Body:        body,
		Actions:     actions,
		WebhookBody: webhookBody,
		SessionID:   hookData.SessionID,
		CWD:         hookData.CWD,
	})

	logging.Debug("=== Codex hook completed: %s ===", hookEvent)
	return nil
}

func StatusForHook(hookEvent string, data HookData) analyzer.Status {
	switch hookEvent {
	case "Stop", "SubagentStop":
		if status := statusFromAssistantMessage(data.LastAssistantMessage); status != analyzer.StatusUnknown {
			return status
		}
		return analyzer.StatusTaskComplete
	case "PermissionRequest":
		return analyzer.StatusQuestion
	case "PreToolUse":
		return analyzer.StatusUnknown
	default:
		return analyzer.StatusUnknown
	}
}

func MessageForHook(hookEvent string, data HookData, status analyzer.Status) (body, actions string) {
	switch hookEvent {
	case "PermissionRequest":
		return permissionRequestMessage(data), ""
	case "Stop", "SubagentStop":
		message := strings.TrimSpace(data.LastAssistantMessage)
		if message == "" {
			message = "Codex turn completed"
		}
		if status == analyzer.StatusSessionLimitReached {
			message = "Codex session limit reached"
		}
		if status == analyzer.StatusAPIError {
			message = "Codex authentication error"
		}
		if status == analyzer.StatusAPIErrorOverloaded {
			message = "Codex API error"
		}
		return truncateRunes(message, maxAssistantMessageRunes), codexActionSummary(data)
	default:
		return "Codex notification", codexActionSummary(data)
	}
}

func WebhookMessageForHook(hookEvent string, data HookData, status analyzer.Status) string {
	switch hookEvent {
	case "Stop", "SubagentStop":
		message := strings.TrimSpace(data.LastAssistantMessage)
		if message == "" {
			return ""
		}
		switch status {
		case analyzer.StatusSessionLimitReached, analyzer.StatusAPIError, analyzer.StatusAPIErrorOverloaded:
			return ""
		default:
			return message
		}
	default:
		return ""
	}
}

func statusFromAssistantMessage(message string) analyzer.Status {
	lower := strings.ToLower(message)
	if lower == "" {
		return analyzer.StatusUnknown
	}

	if strings.Contains(lower, "session limit reached") ||
		strings.Contains(lower, "session limit has been reached") {
		return analyzer.StatusSessionLimitReached
	}

	if strings.Contains(lower, "401") ||
		strings.Contains(lower, "authentication") ||
		strings.Contains(lower, "authentication_error") {
		return analyzer.StatusAPIError
	}

	if strings.Contains(lower, "rate limit") ||
		strings.Contains(lower, "429") ||
		strings.Contains(lower, "529") ||
		strings.Contains(lower, "overloaded") ||
		strings.Contains(lower, "api error") {
		return analyzer.StatusAPIErrorOverloaded
	}

	return analyzer.StatusUnknown
}

func permissionRequestMessage(data HookData) string {
	toolName := strings.TrimSpace(data.ToolName)
	if toolName == "" {
		return "Codex needs permission"
	}

	if command := toolInputString(data.ToolInput, "command"); command != "" {
		return fmt.Sprintf("Codex needs permission for %s: %s", toolName, truncateRunes(command, 240))
	}

	return fmt.Sprintf("Codex needs permission for %s", toolName)
}

func codexActionSummary(data HookData) string {
	parts := make([]string, 0, 2)
	if data.Model != "" {
		parts = append(parts, "Model: "+data.Model)
	}
	if data.TurnID != "" {
		parts = append(parts, "Turn: "+data.TurnID)
	}
	return strings.Join(parts, "  ")
}

func toolInputString(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}

	var values map[string]interface{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if limit <= 0 || len(runes) <= limit {
		return s
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func skipUTF8BOM(input io.Reader) io.Reader {
	reader := bufio.NewReader(input)
	prefix, err := reader.Peek(3)
	if err == nil && bytes.Equal(prefix, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = reader.Discard(3)
	}
	return reader
}
