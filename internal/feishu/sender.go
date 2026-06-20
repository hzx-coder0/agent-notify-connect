package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hzx-coder0/agent-notify-connect/internal/config"
)

const (
	openFeishuBaseURL = "https://open.feishu.cn"
	openLarkBaseURL   = "https://open.larksuite.com"
)

type AppSender struct {
	Config config.FeishuConfig
	Client *http.Client
}

type tenantTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int64  `json:"expire"`
}

func NewAppSender(cfg config.FeishuConfig) *AppSender {
	return &AppSender{
		Config: cfg,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *AppSender) Send(ctx context.Context, card interface{}) error {
	appID := strings.TrimSpace(s.Config.AppID)
	appSecret := strings.TrimSpace(s.Config.ResolveAppSecret())
	receiveIDType := strings.TrimSpace(s.Config.ReceiveIDType)
	receiveID := strings.TrimSpace(s.Config.ReceiveID)
	if appID == "" {
		return fmt.Errorf("feishu app_id is required")
	}
	if appSecret == "" {
		return fmt.Errorf("feishu app_secret or app_secret_env is required")
	}
	if receiveIDType == "" {
		return fmt.Errorf("feishu receive_id_type is required")
	}
	if receiveID == "" {
		return fmt.Errorf("feishu receive_id is required")
	}

	token, err := s.tenantAccessToken(ctx, appID, appSecret)
	if err != nil {
		return err
	}

	content, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("marshal feishu card: %w", err)
	}

	body, err := json.Marshal(map[string]interface{}{
		"receive_id": receiveID,
		"msg_type":   "interactive",
		"content":    string(content),
	})
	if err != nil {
		return fmt.Errorf("marshal feishu message: %w", err)
	}

	baseURL := s.openBaseURL()
	endpoint := fmt.Sprintf("%s/open-apis/im/v1/messages?receive_id_type=%s", baseURL, receiveIDType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "claude-notifications/1.0")

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("feishu message request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newHTTPError(resp, string(respBody))
	}

	var parsed struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &parsed); err == nil && parsed.Code != 0 {
		return fmt.Errorf("feishu message failed: code=%d msg=%s", parsed.Code, parsed.Msg)
	}

	return nil
}

func (s *AppSender) tenantAccessToken(ctx context.Context, appID, appSecret string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	})
	if err != nil {
		return "", err
	}

	endpoint := s.openBaseURL() + "/open-apis/auth/v3/tenant_access_token/internal"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "claude-notifications/1.0")

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu tenant token request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", newHTTPError(resp, string(respBody))
	}

	var parsed tenantTokenResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("decode feishu tenant token response: %w", err)
	}
	if parsed.Code != 0 {
		return "", fmt.Errorf("feishu tenant token failed: code=%d msg=%s", parsed.Code, parsed.Msg)
	}
	if parsed.TenantAccessToken == "" {
		return "", fmt.Errorf("feishu tenant token response missing token")
	}
	return parsed.TenantAccessToken, nil
}

type httpError struct {
	statusCode int
	status     string
	body       string
}

func (e *httpError) Error() string {
	body := e.body
	if len(body) > 200 {
		body = body[:200] + "..."
	}
	if body != "" {
		return fmt.Sprintf("HTTP %d: %s - %s", e.statusCode, e.status, body)
	}
	return fmt.Sprintf("HTTP %d: %s", e.statusCode, e.status)
}

func newHTTPError(resp *http.Response, body string) error {
	return &httpError{
		statusCode: resp.StatusCode,
		status:     resp.Status,
		body:       body,
	}
}

func (s *AppSender) httpClient() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (s *AppSender) openBaseURL() string {
	if strings.TrimSpace(s.Config.BaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(s.Config.BaseURL), "/")
	}
	switch strings.ToLower(strings.TrimSpace(s.Config.Platform)) {
	case "lark":
		return openLarkBaseURL
	default:
		return openFeishuBaseURL
	}
}
