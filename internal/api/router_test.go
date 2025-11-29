package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsDirectLoopbackRequest(t *testing.T) {
	t.Helper()

	tests := []struct {
		name       string
		req        *http.Request
		remoteAddr string
		headers    map[string]string
		want       bool
	}{
		// Nil request
		{
			name: "nil request",
			req:  nil,
			want: false,
		},

		// Valid loopback IPs without proxy headers
		{
			name:       "loopback IPv4 without port",
			remoteAddr: "127.0.0.1",
			want:       true,
		},
		{
			name:       "loopback IPv4 with port",
			remoteAddr: "127.0.0.1:8080",
			want:       true,
		},
		{
			name:       "loopback IPv4 alternate",
			remoteAddr: "127.0.0.2:54321",
			want:       true,
		},
		{
			name:       "loopback IPv6 without port",
			remoteAddr: "::1",
			want:       true,
		},
		{
			name:       "loopback IPv6 with port",
			remoteAddr: "[::1]:8080",
			want:       true,
		},
		{
			name:       "loopback IPv6 with brackets no port",
			remoteAddr: "[::1]",
			want:       true,
		},

		// Loopback with proxy headers (should reject)
		{
			name:       "loopback with X-Forwarded-For",
			remoteAddr: "127.0.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
			},
			want: false,
		},
		{
			name:       "loopback with Forwarded",
			remoteAddr: "127.0.0.1:8080",
			headers: map[string]string{
				"Forwarded": "for=192.168.1.1",
			},
			want: false,
		},
		{
			name:       "loopback with X-Real-IP",
			remoteAddr: "127.0.0.1:8080",
			headers: map[string]string{
				"X-Real-IP": "192.168.1.1",
			},
			want: false,
		},
		{
			name:       "loopback with multiple proxy headers",
			remoteAddr: "127.0.0.1:8080",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
				"X-Real-IP":       "10.0.0.1",
			},
			want: false,
		},
		{
			name:       "loopback IPv6 with X-Forwarded-For",
			remoteAddr: "[::1]:8080",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.42",
			},
			want: false,
		},

		// Non-loopback IPs (should reject)
		{
			name:       "private IPv4",
			remoteAddr: "192.168.1.1:8080",
			want:       false,
		},
		{
			name:       "private IPv4 10.x",
			remoteAddr: "10.0.0.1:54321",
			want:       false,
		},
		{
			name:       "private IPv4 172.x",
			remoteAddr: "172.16.0.1:8080",
			want:       false,
		},
		{
			name:       "public IPv4",
			remoteAddr: "203.0.113.42:8080",
			want:       false,
		},
		{
			name:       "public IPv6",
			remoteAddr: "[2001:db8::1]:8080",
			want:       false,
		},
		{
			name:       "link-local IPv6",
			remoteAddr: "[fe80::1]:8080",
			want:       false,
		},

		// Edge cases
		{
			name:       "empty RemoteAddr",
			remoteAddr: "",
			want:       false,
		},
		{
			name:       "invalid IP format",
			remoteAddr: "not-an-ip:8080",
			want:       false,
		},
		{
			name:       "invalid IP with port",
			remoteAddr: "999.999.999.999:8080",
			want:       false,
		},
		{
			name:       "malformed IPv6",
			remoteAddr: "[::g]:8080",
			want:       false,
		},
		{
			name:       "just port",
			remoteAddr: ":8080",
			want:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.req != nil {
				req = tt.req
			} else if tt.name != "nil request" {
				req = httptest.NewRequest("GET", "/", nil)
				req.RemoteAddr = tt.remoteAddr
				for key, value := range tt.headers {
					req.Header.Set(key, value)
				}
			}

			got := isDirectLoopbackRequest(req)
			if got != tt.want {
				t.Errorf("isDirectLoopbackRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
