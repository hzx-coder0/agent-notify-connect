package feishu

import "testing"

func TestSignCustomWebhook(t *testing.T) {
	got := SignCustomWebhook(1700000000, "secret")
	want := "fiWS2+gh28DOydAv7hzONH/mDn9+b1Y4Y5ivXWXy8vA="
	if got != want {
		t.Fatalf("SignCustomWebhook() = %q, want %q", got, want)
	}
}
