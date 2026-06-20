package notification

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/hzx-coder0/agent-notify-connect/internal/analyzer"
	"github.com/hzx-coder0/agent-notify-connect/internal/config"
	"github.com/hzx-coder0/agent-notify-connect/internal/errorhandler"
	"github.com/hzx-coder0/agent-notify-connect/internal/logging"
	"github.com/hzx-coder0/agent-notify-connect/internal/platform"
	"github.com/hzx-coder0/agent-notify-connect/internal/sessionname"
	"github.com/hzx-coder0/agent-notify-connect/internal/webhook"
)

// DesktopSender sends local desktop notifications.
type DesktopSender interface {
	SendDesktop(status analyzer.Status, message, sessionID, cwd string) error
	Close() error
}

// WebhookSender sends remote webhook notifications.
type WebhookSender interface {
	SendAsyncWithContext(sendCtx webhook.SendContext)
	Shutdown(timeout time.Duration) error
}

// Service fans one classified status out to all configured notification sinks.
type Service struct {
	cfg            *config.Config
	desktop        DesktopSender
	webhook        WebhookSender
	OnDesktopError func(error)
}

type SendOptions struct {
	Status      analyzer.Status
	Body        string
	Actions     string
	WebhookBody string
	SessionID   string
	CWD         string
}

func New(cfg *config.Config, desktop DesktopSender, webhookSender WebhookSender) *Service {
	return &Service{
		cfg:     cfg,
		desktop: desktop,
		webhook: webhookSender,
	}
}

// JoinMessageParts joins the body and action summary with the same contract
// used by the summary package.
func JoinMessageParts(body, actions string) string {
	if actions == "" {
		return body
	}
	return body + " " + actions
}

// Send dispatches a notification using the existing message prefix format.
//
// body is the summary text without metadata. actions is the compact action
// segment, e.g. "📝 1 new  ▶ 2 cmds  ⏱ 41s".
func (s *Service) Send(status analyzer.Status, body, actions, sessionID, cwd string) {
	s.SendWithOptions(SendOptions{
		Status:    status,
		Body:      body,
		Actions:   actions,
		SessionID: sessionID,
		CWD:       cwd,
	})
}

func (s *Service) SendWithOptions(opts SendOptions) {
	defer errorhandler.HandlePanic()

	sessionName := sessionname.GenerateSessionLabel(opts.SessionID)
	gitBranch := platform.GetGitBranch(opts.CWD)
	folderName := filepath.Base(opts.CWD)

	joined := JoinMessageParts(opts.Body, opts.Actions)
	webhookBody := opts.WebhookBody
	if webhookBody == "" {
		webhookBody = opts.Body
	}
	webhookJoined := JoinMessageParts(webhookBody, opts.Actions)

	var enhancedMessage string
	var enhancedWebhookMessage string
	if gitBranch != "" {
		enhancedMessage = fmt.Sprintf("[%s|%s %s] %s", sessionName, gitBranch, folderName, joined)
		enhancedWebhookMessage = fmt.Sprintf("[%s|%s %s] %s", sessionName, gitBranch, folderName, webhookJoined)
	} else {
		enhancedMessage = fmt.Sprintf("[%s %s] %s", sessionName, folderName, joined)
		enhancedWebhookMessage = fmt.Sprintf("[%s %s] %s", sessionName, folderName, webhookJoined)
	}

	logging.Debug("Session name: %s, git branch: %s, folder: %s", sessionName, gitBranch, folderName)

	statusStr := string(opts.Status)

	if s.cfg.IsStatusDesktopEnabled(statusStr) {
		if err := s.desktop.SendDesktop(opts.Status, enhancedMessage, opts.SessionID, opts.CWD); err != nil {
			if s.OnDesktopError != nil {
				s.OnDesktopError(err)
			}
			errorhandler.HandleError(err, "Failed to send desktop notification")
		}
	} else {
		logging.Debug("Desktop notification disabled for status: %s", statusStr)
	}

	if s.cfg.IsStatusWebhookEnabled(statusStr) {
		s.webhook.SendAsyncWithContext(webhook.SendContext{
			Status:        opts.Status,
			Message:       enhancedWebhookMessage,
			SessionID:     opts.SessionID,
			CWD:           opts.CWD,
			SessionName:   sessionName,
			GitBranch:     gitBranch,
			Folder:        folderName,
			RawBody:       webhookBody,
			ActionSummary: opts.Actions,
		})
	} else {
		logging.Debug("Webhook notification disabled for status: %s", statusStr)
	}
}
