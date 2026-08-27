package notifications

import (
	"strings"
	"testing"
)

func TestValidateDeadManPingURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "disabled", value: ""},
		{name: "hosted HTTPS", value: "https://hc.example.com/ping/secret-token"},
		{name: "separate LAN host", value: "http://192.168.50.12:8000/ping/secret-token"},
		{name: "IPv6 LAN host", value: "http://[fd00::12]/ping/secret-token"},
		{name: "query token", value: "https://watchdog.example.com/ping?id=secret-token"},
		{name: "non HTTP scheme", value: "ftp://watchdog.example.com/ping/token", wantErr: true},
		{name: "relative URL", value: "/ping/token", wantErr: true},
		{name: "userinfo", value: "https://admin:secret@watchdog.example.com/ping/token", wantErr: true},
		{name: "fragment", value: "https://watchdog.example.com/ping/token#secret", wantErr: true},
		{name: "localhost", value: "http://localhost:8000/ping/token", wantErr: true},
		{name: "localhost subdomain", value: "http://pulse.localhost/ping/token", wantErr: true},
		{name: "IPv4 loopback", value: "http://127.0.0.1/ping/token", wantErr: true},
		{name: "IPv6 loopback", value: "http://[::1]/ping/token", wantErr: true},
		{name: "unspecified", value: "http://0.0.0.0/ping/token", wantErr: true},
		{name: "link local", value: "http://169.254.20.10/ping/token", wantErr: true},
		{name: "failure suffix", value: "https://watchdog.example.com/ping/token/fail", wantErr: true},
		{name: "failure suffix slash", value: "https://watchdog.example.com/ping/token/fail/", wantErr: true},
		{name: "start suffix", value: "https://watchdog.example.com/ping/token/start", wantErr: true},
		{name: "log suffix", value: "https://watchdog.example.com/ping/token/log", wantErr: true},
		{name: "oversized", value: "https://watchdog.example.com/ping/" + strings.Repeat("x", MaxDeadManPingURLLength), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateDeadManPingURL(test.value)
			if test.wantErr && err == nil {
				t.Fatalf("ValidateDeadManPingURL(%q) unexpectedly succeeded", test.value)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ValidateDeadManPingURL(%q): %v", test.value, err)
			}
		})
	}
}

func TestNormalizeDeadManConfig(t *testing.T) {
	t.Parallel()

	got := NormalizeDeadManConfig(DeadManConfig{PingURL: "  https://watchdog.example.com/ping/token  "})
	if got.PingURL != "https://watchdog.example.com/ping/token" {
		t.Fatalf("normalized URL = %q", got.PingURL)
	}
}
