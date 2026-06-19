package feishu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPollTreatsHTTP400AuthorizationPendingAsPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/v1/app/registration" {
			t.Fatalf("path = %s, want /oauth/v1/app/registration", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.Form.Get("action"); got != "poll" {
			t.Fatalf("action = %q, want poll", got)
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"authorization_pending","error_description":"","code":20094}`))
	}))
	defer server.Close()

	client := NewRegistrationClient()
	client.BaseURL = server.URL

	result, err := client.Poll(context.Background(), "device-code")
	if err != nil {
		t.Fatalf("Poll() error = %v", err)
	}
	if result.Status != "pending" {
		t.Fatalf("status = %q, want pending", result.Status)
	}
}

func TestPollReturnsUnknownHTTP400Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad_request","error_description":"bad"}`))
	}))
	defer server.Close()

	client := NewRegistrationClient()
	client.BaseURL = server.URL

	_, err := client.Poll(context.Background(), "device-code")
	if err == nil {
		t.Fatal("Poll() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "http 400") {
		t.Fatalf("error = %q, want http 400", err.Error())
	}
}
