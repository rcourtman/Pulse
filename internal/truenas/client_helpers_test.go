package truenas

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResolveEndpointCoversDefaultsAndErrors(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		useHTTPS     bool
		port         int
		wantHTTPS    bool
		wantHostPort string
		wantErr      string
	}{
		{
			name:         "defaults to https when no hints",
			host:         "truenas.local",
			wantHTTPS:    true,
			wantHostPort: "truenas.local:443",
		},
		{
			name:         "http scheme with explicit port stays http",
			host:         "http://truenas.local",
			port:         80,
			wantHTTPS:    false,
			wantHostPort: "truenas.local:80",
		},
		{
			name:         "https scheme keeps parsed port",
			host:         "https://truenas.local:8443",
			wantHTTPS:    true,
			wantHostPort: "truenas.local:8443",
		},
		{
			name:         "host embedded port is honored",
			host:         "truenas.local:9000",
			wantHTTPS:    true,
			wantHostPort: "truenas.local:9000",
		},
		{
			name:    "empty host",
			host:    "  ",
			wantErr: "host is required",
		},
		{
			name:    "unsupported scheme",
			host:    "ftp://truenas.local",
			wantErr: "unsupported truenas scheme",
		},
		{
			name:    "scheme missing host",
			host:    "https:///api",
			wantErr: "missing host",
		},
		{
			name:    "invalid host port text",
			host:    "truenas.local:notaport",
			wantErr: "invalid truenas port",
		},
		{
			name:    "invalid explicit port range",
			host:    "truenas.local",
			port:    70000,
			wantErr: "invalid truenas port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHTTPS, gotHostPort, err := resolveEndpoint(tt.host, tt.useHTTPS, tt.port)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("resolveEndpoint(%q) expected error containing %q", tt.host, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveEndpoint(%q) error = %v, want substring %q", tt.host, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveEndpoint(%q) unexpected error: %v", tt.host, err)
			}
			if gotHTTPS != tt.wantHTTPS {
				t.Fatalf("resolveEndpoint(%q) useHTTPS = %v, want %v", tt.host, gotHTTPS, tt.wantHTTPS)
			}
			if gotHostPort != tt.wantHostPort {
				t.Fatalf("resolveEndpoint(%q) hostPort = %q, want %q", tt.host, gotHostPort, tt.wantHostPort)
			}
		})
	}
}

func TestSplitHostPortCoversEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		rawHost  string
		wantHost string
		wantPort string
		wantErr  string
	}{
		{name: "empty", rawHost: "", wantHost: "", wantPort: ""},
		{name: "hostname with port", rawHost: "example.com:8443", wantHost: "example.com", wantPort: "8443"},
		{name: "ipv6 with port", rawHost: "[2001:db8::1]:8443", wantHost: "2001:db8::1", wantPort: "8443"},
		{name: "ipv6 bracketed without port", rawHost: "[2001:db8::1]", wantHost: "2001:db8::1", wantPort: ""},
		{name: "ipv6 without brackets", rawHost: "2001:db8::1", wantErr: "invalid truenas host"},
		{name: "malformed host", rawHost: "[2001:db8::1", wantErr: "invalid truenas host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := splitHostPort(tt.rawHost)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("splitHostPort(%q) expected error containing %q", tt.rawHost, tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("splitHostPort(%q) error = %v, want substring %q", tt.rawHost, err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("splitHostPort(%q) unexpected error: %v", tt.rawHost, err)
			}
			if host != tt.wantHost || port != tt.wantPort {
				t.Fatalf("splitHostPort(%q) = (%q,%q), want (%q,%q)", tt.rawHost, host, port, tt.wantHost, tt.wantPort)
			}
		})
	}
}

func TestParseInt64FromAnyCoversBranches(t *testing.T) {
	tests := []struct {
		name    string
		raw     json.RawMessage
		want    int64
		wantErr string
	}{
		{name: "json integer", raw: json.RawMessage(`123`), want: 123},
		{name: "json float", raw: json.RawMessage(`123.9`), want: 123},
		{name: "numeric string", raw: json.RawMessage(`" 42 "`), want: 42},
		{name: "empty string", raw: json.RawMessage(`""`), wantErr: "empty numeric string"},
		{name: "null", raw: json.RawMessage(`null`), wantErr: "numeric value is null"},
		{name: "unsupported type", raw: json.RawMessage(`{"id":1}`), wantErr: "unsupported numeric type"},
		{name: "decode error", raw: json.RawMessage(`{"id"`), wantErr: "unexpected EOF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseInt64FromAny(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseInt64FromAny(%s) expected error containing %q", string(tt.raw), tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseInt64FromAny(%s) error = %v, want substring %q", string(tt.raw), err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseInt64FromAny(%s) unexpected error: %v", string(tt.raw), err)
			}
			if got != tt.want {
				t.Fatalf("parseInt64FromAny(%s) = %d, want %d", string(tt.raw), got, tt.want)
			}
		})
	}
}

func TestParseBoolFromAnyCoversBranches(t *testing.T) {
	tests := []struct {
		name    string
		raw     json.RawMessage
		want    bool
		wantErr string
	}{
		{name: "bool true", raw: json.RawMessage(`true`), want: true},
		{name: "json number true", raw: json.RawMessage(`1`), want: true},
		{name: "json number false", raw: json.RawMessage(`0`), want: false},
		{name: "json number parse error", raw: json.RawMessage(`1.5`), wantErr: "parse json number"},
		{name: "truthy string", raw: json.RawMessage(`" yes "`), want: true},
		{name: "invalid string", raw: json.RawMessage(`"maybe"`), wantErr: "parse bool from string"},
		{name: "null", raw: json.RawMessage(`null`), want: false},
		{name: "unsupported type", raw: json.RawMessage(`[1]`), wantErr: "unsupported bool type"},
		{name: "decode error", raw: json.RawMessage(`{"value"`), wantErr: "unexpected EOF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBoolFromAny(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("parseBoolFromAny(%s) expected error containing %q", string(tt.raw), tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseBoolFromAny(%s) error = %v, want substring %q", string(tt.raw), err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBoolFromAny(%s) unexpected error: %v", string(tt.raw), err)
			}
			if got != tt.want {
				t.Fatalf("parseBoolFromAny(%s) = %v, want %v", string(tt.raw), got, tt.want)
			}
		})
	}
}

func TestNestedValueHelpersFallbackToRawValue(t *testing.T) {
	numeric := nestedValue{Parsed: json.RawMessage(`"not-a-number"`), RawValue: "55"}
	value, err := numeric.int64Value()
	if err != nil {
		t.Fatalf("int64Value() fallback error = %v", err)
	}
	if value != 55 {
		t.Fatalf("int64Value() fallback = %d, want 55", value)
	}

	numericEmpty := nestedValue{Parsed: json.RawMessage(`"not-a-number"`), RawValue: "  "}
	if _, err := numericEmpty.int64Value(); err == nil || !strings.Contains(err.Error(), "missing numeric field") {
		t.Fatalf("expected missing numeric field error, got %v", err)
	}

	numericBadRaw := nestedValue{Parsed: json.RawMessage(`"not-a-number"`), RawValue: "abc"}
	if _, err := numericBadRaw.int64Value(); err == nil || !strings.Contains(err.Error(), "parse int64 from rawvalue") {
		t.Fatalf("expected rawvalue parse error, got %v", err)
	}

	flag := nestedValue{Parsed: json.RawMessage(`"not-a-bool"`), RawValue: "on"}
	boolValue, err := flag.boolValue()
	if err != nil {
		t.Fatalf("boolValue() fallback error = %v", err)
	}
	if !boolValue {
		t.Fatal("expected boolValue() fallback to true")
	}

	flagUnknown := nestedValue{Parsed: json.RawMessage(`"not-a-bool"`), RawValue: "maybe"}
	if _, err := flagUnknown.boolValue(); err == nil || !strings.Contains(err.Error(), "parse bool from rawvalue") {
		t.Fatalf("expected rawvalue bool parse error, got %v", err)
	}
}

func TestRawIDToStringCoversStringNumericAndError(t *testing.T) {
	id, err := rawIDToString(json.RawMessage(`"  abc  "`))
	if err != nil {
		t.Fatalf("rawIDToString(string) error = %v", err)
	}
	if id != "abc" {
		t.Fatalf("rawIDToString(string) = %q, want %q", id, "abc")
	}

	numericID, err := rawIDToString(json.RawMessage(`42`))
	if err != nil {
		t.Fatalf("rawIDToString(number) error = %v", err)
	}
	if numericID != "42" {
		t.Fatalf("rawIDToString(number) = %q, want %q", numericID, "42")
	}

	if _, err := rawIDToString(json.RawMessage(`{"id":42}`)); err == nil || !strings.Contains(err.Error(), "unsupported alert id") {
		t.Fatalf("expected unsupported alert id error, got %v", err)
	}
}
