package securityutil

import (
	"strings"
	"testing"
)

// These tests target the websocket-origin helpers in websocket_origin.go:
// NormalizeWebSocketOriginHost, SameHostWebSocketOrigin,
// HTTPOriginForWebSocketBaseURL, and HTTPOriginForWebSocketBaseURLWithOptions.
// They exercise both sides of each conditional, the switch arms, error paths,
// and boundary/empty inputs against the package's real exported functions.

func TestBranchCovNormalizeWebSocketOriginHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		// Branch: empty after TrimSpace -> return "".
		{name: "empty string", host: "", want: ""},
		{name: "whitespace only", host: "   ", want: ""},
		{name: "tab newline only", host: "\t\n", want: ""},

		// Branch: SplitHostPort error (no port) -> return lowercased+trimmed host.
		{name: "plain host lowercased", host: "Example.COM", want: "example.com"},
		{name: "plain host trimmed and lowercased", host: "  Host.Example.COM  ", want: "host.example.com"},

		// Branch: port == "80" -> strip, return host only.
		{name: "port 80 stripped", host: "example.com:80", want: "example.com"},
		{name: "uppercase host port 80 stripped", host: "API.Example.COM:80", want: "api.example.com"},

		// Branch: port == "443" -> strip, return host only.
		{name: "port 443 stripped", host: "example.com:443", want: "example.com"},

		// Branch: other port -> JoinHostPort(host, port).
		{name: "nonstandard port kept", host: "example.com:8080", want: "example.com:8080"},
		{name: "uppercase host nonstandard port", host: "API.Host:9000", want: "api.host:9000"},

		// Boundary: "080" is not "80", so the port is preserved verbatim.
		{name: "non canonical port string kept", host: "example.com:080", want: "example.com:080"},

		// IPv6: JoinHostPort re-wraps in brackets.
		{name: "ipv6 nonstandard port joined", host: "[::1]:8080", want: "[::1]:8080"},
		{name: "ipv6 bracketed without port kept", host: "[::1]", want: "[::1]"},

		// IPv6 with standard port: SplitHostPort returns the bare host without
		// brackets, so the returned value loses its IPv6 bracketing. This is the
		// actual current behavior (see report for the same-origin impact).
		{name: "ipv6 port 80 loses brackets", host: "[::1]:80", want: "::1"},
		{name: "ipv6 port 443 loses brackets", host: "[fe80::1]:443", want: "fe80::1"},

		// Boundary: empty host with a port. SplitHostPort(":8080") succeeds with
		// an empty host, so JoinHostPort reproduces ":8080"; ":80" strips to "".
		{name: "empty host with nonstandard port", host: ":8080", want: ":8080"},
		{name: "empty host with port 80 collapses to empty", host: ":80", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeWebSocketOriginHost(tt.host)
			if got != tt.want {
				t.Fatalf("NormalizeWebSocketOriginHost(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestBranchCovSameHostWebSocketOrigin(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		requestHost string
		want        bool
	}{
		// Branch: url.Parse error -> false. "%zz" is an invalid escape that
		// url.Parse rejects (same input used elsewhere for parse-error coverage).
		{name: "parse error invalid escape", origin: "http://host/%zz", requestHost: "host", want: false},

		// Branch: parsed.Host == "" -> false.
		{name: "empty origin", origin: "", requestHost: "example.com", want: false},
		{name: "whitespace only origin", origin: "   ", requestHost: "example.com", want: false},
		{name: "scheme but empty host", origin: "http://", requestHost: "example.com", want: false},

		// Branch: scheme not http/https -> false. Note ws/wss origins are rejected.
		{name: "ftp scheme rejected", origin: "ftp://example.com", requestHost: "example.com", want: false},
		{name: "ws scheme rejected", origin: "ws://localhost", requestHost: "localhost", want: false},
		{name: "wss scheme rejected", origin: "wss://localhost", requestHost: "localhost", want: false},
		{name: "file scheme rejected", origin: "file:///etc/passwd", requestHost: "localhost", want: false},

		// Branch: http/https scheme, equal normalized hosts -> true.
		{name: "https same host", origin: "https://example.com", requestHost: "example.com", want: true},
		{name: "http same host", origin: "http://example.com", requestHost: "example.com", want: true},
		{name: "scheme differs but host equal", origin: "https://example.com", requestHost: "example.com", want: true},
		{name: "origin trimmed before parse", origin: "  https://example.com  ", requestHost: "example.com", want: true},
		{name: "uppercase origin host matches", origin: "https://API.Example.COM", requestHost: "api.example.com", want: true},

		// Integration: standard ports normalize away on both sides -> match.
		{name: "origin port 80 equals bare host", origin: "http://example.com:80", requestHost: "example.com", want: true},
		{name: "both standard port 443 match", origin: "https://example.com:443", requestHost: "example.com:443", want: true},

		// Branch: normalized hosts differ -> false.
		{name: "nonstandard port mismatch", origin: "http://example.com:8080", requestHost: "example.com", want: false},
		{name: "host mismatch", origin: "https://example.com", requestHost: "evil.com", want: false},
		{name: "both same nonstandard port match", origin: "http://example.com:9000", requestHost: "example.com:9000", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SameHostWebSocketOrigin(tt.origin, tt.requestHost)
			if got != tt.want {
				t.Fatalf("SameHostWebSocketOrigin(%q, %q) = %v, want %v", tt.origin, tt.requestHost, got, tt.want)
			}
		})
	}
}

func TestBranchCovHTTPOriginForWebSocketBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
		want    string
	}{
		// Delegating entrypoint: wss -> https.
		{name: "wss to https", raw: "wss://example.com", want: "https://example.com"},
		// ws over loopback -> http.
		{name: "ws loopback to http", raw: "ws://localhost:8080", want: "http://localhost:8080"},
		// https is upgraded to wss by the normalizer, then mapped to https.
		{name: "https round trips to https", raw: "https://example.com", want: "https://example.com"},
		// Error path delegated from the zero-option validator.
		{name: "empty required", raw: "  ", wantErr: "Pulse URL is required"},
		{name: "non loopback http rejected", raw: "http://example.com", wantErr: "must use https/wss unless host is loopback"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HTTPOriginForWebSocketBaseURL(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("HTTPOriginForWebSocketBaseURL(%q) = %q, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("HTTPOriginForWebSocketBaseURL(%q) err = %q, want substring %q", tt.raw, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("HTTPOriginForWebSocketBaseURL(%q) unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("HTTPOriginForWebSocketBaseURL(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestBranchCovHTTPOriginForWebSocketBaseURLWithOptions(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		opts    PulseURLValidationOptions
		wantErr string
		want    string
	}{
		// switch arm "ws" -> "http".
		{name: "ws maps to http", raw: "ws://localhost:8080", opts: PulseURLValidationOptions{}, want: "http://localhost:8080"},
		// switch arm "wss" -> "https".
		{name: "wss maps to https", raw: "wss://example.com", opts: PulseURLValidationOptions{}, want: "https://example.com"},
		// https is normalized to wss upstream, then mapped back to https.
		{name: "https normalized to wss then https", raw: "https://api.example.com", opts: PulseURLValidationOptions{}, want: "https://api.example.com"},

		// The normalizer preserves a non-root path; this function must clear
		// Path/RawPath/RawQuery/Fragment so only the bare origin remains.
		{name: "path stripped to origin", raw: "wss://example.com/a/b", opts: PulseURLValidationOptions{}, want: "https://example.com"},
		{name: "loopback ws path stripped to origin", raw: "ws://127.0.0.1/agent/feed", opts: PulseURLValidationOptions{}, want: "http://127.0.0.1"},

		// opts flow through to the validator: AllowLocalNetworkHTTP permits
		// plaintext to a private IPv4, which then maps ws -> http.
		{name: "opts allow local network plaintext", raw: "http://192.168.1.5:9000", opts: PulseURLValidationOptions{AllowLocalNetworkHTTP: true}, want: "http://192.168.1.5:9000"},

		// Error path: upstream normalization failure is returned as-is.
		{name: "empty required", raw: "", opts: PulseURLValidationOptions{}, wantErr: "Pulse URL is required"},
		{name: "missing scheme websocket message", raw: "example.com", opts: PulseURLValidationOptions{}, wantErr: "must include scheme"},
		{name: "non loopback http without opts rejected", raw: "http://example.com", opts: PulseURLValidationOptions{}, wantErr: "must use https/wss unless host is loopback"},
		{name: "unsupported scheme rejected upstream", raw: "ftp://host", opts: PulseURLValidationOptions{}, wantErr: "unsupported scheme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HTTPOriginForWebSocketBaseURLWithOptions(tt.raw, tt.opts)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("HTTPOriginForWebSocketBaseURLWithOptions(%q) = %q, want error containing %q", tt.raw, got, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("HTTPOriginForWebSocketBaseURLWithOptions(%q) err = %q, want substring %q", tt.raw, err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("HTTPOriginForWebSocketBaseURLWithOptions(%q) unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("HTTPOriginForWebSocketBaseURLWithOptions(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
