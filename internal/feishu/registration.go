package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	AccountsFeishuBaseURL = "https://accounts.feishu.cn"
	AccountsLarkBaseURL   = "https://accounts.larksuite.com"
)

type RegistrationClient struct {
	BaseURL string
	HTTP    *http.Client
}

type RegistrationBeginResult struct {
	DeviceCode              string
	VerificationURIComplete string
	Interval                int
	ExpireIn                int
	BaseURL                 string
}

type RegistrationPollResult struct {
	Status      string
	AppID       string
	AppSecret   string
	OwnerOpenID string
	Platform    string
	BaseURL     string
	Error       string
	Description string
}

type registrationInitResponse struct {
	Error                string   `json:"error"`
	ErrorDescription     string   `json:"error_description"`
	SupportedAuthMethods []string `json:"supported_auth_methods"`
}

type registrationBeginResponse struct {
	Error                   string `json:"error"`
	ErrorDescription        string `json:"error_description"`
	DeviceCode              string `json:"device_code"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpireIn                int    `json:"expire_in"`
}

type registrationPollUserInfo struct {
	OpenID      string `json:"open_id"`
	TenantBrand string `json:"tenant_brand"`
}

type registrationPollResponse struct {
	Error            string                   `json:"error"`
	ErrorDescription string                   `json:"error_description"`
	ClientID         string                   `json:"client_id"`
	ClientSecret     string                   `json:"client_secret"`
	UserInfo         registrationPollUserInfo `json:"user_info"`
}

func NewRegistrationClient() *RegistrationClient {
	return &RegistrationClient{
		BaseURL: AccountsFeishuBaseURL,
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *RegistrationClient) Begin(ctx context.Context) (*RegistrationBeginResult, error) {
	var initRes registrationInitResponse
	if err := c.registrationCall(ctx, "init", nil, &initRes); err != nil {
		return nil, fmt.Errorf("feishu init: %w", err)
	}
	if initRes.Error != "" {
		return nil, fmt.Errorf("feishu init: %s: %s", initRes.Error, initRes.ErrorDescription)
	}
	if len(initRes.SupportedAuthMethods) > 0 && !containsString(initRes.SupportedAuthMethods, "client_secret") {
		return nil, fmt.Errorf("feishu registration does not support client_secret auth")
	}

	var beginRes registrationBeginResponse
	params := map[string]string{
		"archetype":         "PersonalAgent",
		"auth_method":       "client_secret",
		"request_user_info": "open_id",
	}
	if err := c.registrationCall(ctx, "begin", params, &beginRes); err != nil {
		return nil, fmt.Errorf("feishu begin: %w", err)
	}
	if beginRes.Error != "" {
		return nil, fmt.Errorf("feishu begin: %s: %s", beginRes.Error, beginRes.ErrorDescription)
	}
	if beginRes.DeviceCode == "" || beginRes.VerificationURIComplete == "" {
		return nil, fmt.Errorf("feishu begin: incomplete response")
	}
	if beginRes.Interval <= 0 {
		beginRes.Interval = 5
	}
	if beginRes.ExpireIn <= 0 {
		beginRes.ExpireIn = 600
	}

	return &RegistrationBeginResult{
		DeviceCode:              beginRes.DeviceCode,
		VerificationURIComplete: beginRes.VerificationURIComplete,
		Interval:                beginRes.Interval,
		ExpireIn:                beginRes.ExpireIn,
		BaseURL:                 c.BaseURL,
	}, nil
}

func (c *RegistrationClient) Poll(ctx context.Context, deviceCode string) (*RegistrationPollResult, error) {
	if strings.TrimSpace(deviceCode) == "" {
		return nil, fmt.Errorf("device_code is required")
	}

	var pollRes registrationPollResponse
	if err := c.registrationPollCall(ctx, map[string]string{"device_code": deviceCode}, &pollRes); err != nil {
		return nil, fmt.Errorf("feishu poll: %w", err)
	}

	platform := "feishu"
	if strings.EqualFold(strings.TrimSpace(pollRes.UserInfo.TenantBrand), "lark") {
		platform = "lark"
		if c.BaseURL != AccountsLarkBaseURL {
			c.BaseURL = AccountsLarkBaseURL
			return &RegistrationPollResult{
				Status:  "pending",
				BaseURL: c.BaseURL,
			}, nil
		}
	}

	if pollRes.ClientID != "" && pollRes.ClientSecret != "" {
		return &RegistrationPollResult{
			Status:      "completed",
			AppID:       pollRes.ClientID,
			AppSecret:   pollRes.ClientSecret,
			OwnerOpenID: pollRes.UserInfo.OpenID,
			Platform:    platform,
			BaseURL:     c.BaseURL,
		}, nil
	}

	switch pollRes.Error {
	case "", "authorization_pending", "slow_down":
		return &RegistrationPollResult{
			Status:      "pending",
			BaseURL:     c.BaseURL,
			Error:       pollRes.Error,
			Description: pollRes.ErrorDescription,
		}, nil
	case "access_denied":
		return nil, fmt.Errorf("feishu poll: authorization denied")
	case "expired_token":
		return nil, fmt.Errorf("feishu poll: onboarding session expired")
	default:
		return nil, fmt.Errorf("feishu poll: %s: %s", pollRes.Error, pollRes.ErrorDescription)
	}
}

func (c *RegistrationClient) registrationPollCall(ctx context.Context, params map[string]string, out interface{}) error {
	statusCode, body, err := c.registrationRawCall(ctx, "poll", params)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		var pollRes registrationPollResponse
		if err := json.Unmarshal(body, &pollRes); err == nil {
			switch pollRes.Error {
			case "authorization_pending", "slow_down", "access_denied", "expired_token":
				return nil
			}
		}
		return fmt.Errorf("http %d: %s", statusCode, truncateString(string(body), 256))
	}
	return nil
}

func (c *RegistrationClient) registrationCall(ctx context.Context, action string, params map[string]string, out interface{}) error {
	statusCode, body, err := c.registrationRawCall(ctx, action, params)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("http %d: %s", statusCode, truncateString(string(body), 256))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *RegistrationClient) registrationRawCall(ctx context.Context, action string, params map[string]string) (int, []byte, error) {
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = AccountsFeishuBaseURL
	}

	form := url.Values{}
	form.Set("action", action)
	for key, value := range params {
		form.Set(key, value)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/oauth/v1/app/registration", strings.NewReader(form.Encode()))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), expected) {
			return true
		}
	}
	return false
}

func truncateString(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
