package notifications

import (
	"net"
	"testing"
	"time"
)

func TestFormatWebhookDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		// Seconds range (< 1 minute)
		{
			name:     "zero duration",
			duration: 0,
			expected: "0s",
		},
		{
			name:     "one second",
			duration: time.Second,
			expected: "1s",
		},
		{
			name:     "30 seconds",
			duration: 30 * time.Second,
			expected: "30s",
		},
		{
			name:     "59 seconds",
			duration: 59 * time.Second,
			expected: "59s",
		},
		{
			name:     "sub-second rounds down to 0s",
			duration: 500 * time.Millisecond,
			expected: "0s",
		},
		// Minutes range (>= 1 minute, < 1 hour)
		{
			name:     "one minute exactly",
			duration: time.Minute,
			expected: "1m",
		},
		{
			name:     "90 seconds shows as 1m",
			duration: 90 * time.Second,
			expected: "1m",
		},
		{
			name:     "30 minutes",
			duration: 30 * time.Minute,
			expected: "30m",
		},
		{
			name:     "59 minutes",
			duration: 59 * time.Minute,
			expected: "59m",
		},
		// Hours range (>= 1 hour, < 24 hours)
		{
			name:     "one hour exactly",
			duration: time.Hour,
			expected: "1h 0m",
		},
		{
			name:     "1 hour 30 minutes",
			duration: 90 * time.Minute,
			expected: "1h 30m",
		},
		{
			name:     "12 hours",
			duration: 12 * time.Hour,
			expected: "12h 0m",
		},
		{
			name:     "23 hours 59 minutes",
			duration: 23*time.Hour + 59*time.Minute,
			expected: "23h 59m",
		},
		// Days range (>= 24 hours)
		{
			name:     "one day exactly",
			duration: 24 * time.Hour,
			expected: "1d 0h",
		},
		{
			name:     "1.5 days",
			duration: 36 * time.Hour,
			expected: "1d 12h",
		},
		{
			name:     "7 days",
			duration: 7 * 24 * time.Hour,
			expected: "7d 0h",
		},
		{
			name:     "30 days",
			duration: 30 * 24 * time.Hour,
			expected: "30d 0h",
		},
		{
			name:     "30 days 23 hours",
			duration: 30*24*time.Hour + 23*time.Hour,
			expected: "30d 23h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatWebhookDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatWebhookDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		// Valid numeric strings
		{
			name:     "single digit",
			input:    "0",
			expected: true,
		},
		{
			name:     "positive integer",
			input:    "12345",
			expected: true,
		},
		{
			name:     "large number",
			input:    "9876543210",
			expected: true,
		},
		{
			name:     "all zeros",
			input:    "0000",
			expected: true,
		},
		// Invalid strings
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "negative sign",
			input:    "-123",
			expected: false,
		},
		{
			name:     "decimal point",
			input:    "12.34",
			expected: false,
		},
		{
			name:     "letters",
			input:    "abc",
			expected: false,
		},
		{
			name:     "mixed alphanumeric",
			input:    "123abc",
			expected: false,
		},
		{
			name:     "spaces",
			input:    "123 456",
			expected: false,
		},
		{
			name:     "leading space",
			input:    " 123",
			expected: false,
		},
		{
			name:     "trailing space",
			input:    "123 ",
			expected: false,
		},
		{
			name:     "plus sign",
			input:    "+123",
			expected: false,
		},
		{
			name:     "underscore",
			input:    "123_456",
			expected: false,
		},
		{
			name:     "comma separator",
			input:    "1,234",
			expected: false,
		},
		{
			name:     "only whitespace",
			input:    "   ",
			expected: false,
		},
		{
			name:     "unicode digits",
			input:    "١٢٣", // Arabic-Indic digits
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractTelegramChatID(t *testing.T) {
	tests := []struct {
		name        string
		webhookURL  string
		expected    string
		expectError bool
		errorSubstr string
	}{
		// Valid chat IDs
		{
			name:        "valid positive chat ID",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=12345",
			expected:    "12345",
			expectError: false,
		},
		{
			name:        "valid negative chat ID (group chat)",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=-987654321",
			expected:    "-987654321",
			expectError: false,
		},
		{
			name:        "chat_id with other params",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?parse_mode=HTML&chat_id=12345&disable_notification=true",
			expected:    "12345",
			expectError: false,
		},
		{
			name:        "large chat ID",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=1234567890123",
			expected:    "1234567890123",
			expectError: false,
		},
		// Missing chat_id parameter
		{
			name:        "missing chat_id parameter",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage",
			expectError: true,
			errorSubstr: "missing chat_id parameter",
		},
		{
			name:        "empty chat_id value",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=",
			expectError: true,
			errorSubstr: "chat_id parameter is empty",
		},
		{
			name:        "chat_id with no value - detected as missing",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id",
			expectError: true,
			errorSubstr: "missing chat_id parameter", // "chat_id=" not present
		},
		// Invalid chat_id values
		{
			name:        "non-numeric chat_id",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=abc123",
			expectError: true,
			errorSubstr: "chat_id must be numeric",
		},
		{
			name:        "chat_id with spaces",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=123%20456",
			expectError: true,
			errorSubstr: "chat_id must be numeric",
		},
		{
			name:        "chat_id with special chars",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=@channel",
			expectError: true,
			errorSubstr: "chat_id must be numeric",
		},
		{
			name:        "negative with non-numeric suffix",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=-abc",
			expectError: true,
			errorSubstr: "chat_id must be numeric",
		},
		{
			name:        "just a minus sign",
			webhookURL:  "https://api.telegram.org/bot123456:ABC/sendMessage?chat_id=-",
			expectError: true,
			errorSubstr: "chat_id must be numeric",
		},
		// Invalid URLs
		{
			name:        "invalid URL format",
			webhookURL:  "not-a-url",
			expectError: true,
			errorSubstr: "missing chat_id parameter",
		},
		{
			name:        "empty URL",
			webhookURL:  "",
			expectError: true,
			errorSubstr: "missing chat_id parameter",
		},
		{
			name:        "URL with control characters",
			webhookURL:  "https://api.telegram.org/bot\x00/sendMessage?chat_id=12345",
			expectError: true,
			errorSubstr: "invalid URL format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractTelegramChatID(tt.webhookURL)

			if tt.expectError {
				if err == nil {
					t.Errorf("extractTelegramChatID(%q) expected error containing %q, got nil",
						tt.webhookURL, tt.errorSubstr)
				} else if tt.errorSubstr != "" && !contains(err.Error(), tt.errorSubstr) {
					t.Errorf("extractTelegramChatID(%q) error = %q, want error containing %q",
						tt.webhookURL, err.Error(), tt.errorSubstr)
				}
			} else {
				if err != nil {
					t.Errorf("extractTelegramChatID(%q) unexpected error: %v", tt.webhookURL, err)
				}
				if result != tt.expected {
					t.Errorf("extractTelegramChatID(%q) = %q, want %q", tt.webhookURL, result, tt.expected)
				}
			}
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// RFC1918 10.0.0.0/8
		{
			name:     "10.0.0.0 private",
			ip:       "10.0.0.0",
			expected: true,
		},
		{
			name:     "10.0.0.1 private",
			ip:       "10.0.0.1",
			expected: true,
		},
		{
			name:     "10.255.255.255 private",
			ip:       "10.255.255.255",
			expected: true,
		},
		// RFC1918 172.16.0.0/12
		{
			name:     "172.16.0.1 private",
			ip:       "172.16.0.1",
			expected: true,
		},
		{
			name:     "172.31.255.255 private",
			ip:       "172.31.255.255",
			expected: true,
		},
		{
			name:     "172.15.0.1 public (just below range)",
			ip:       "172.15.0.1",
			expected: false,
		},
		{
			name:     "172.32.0.1 public (just above range)",
			ip:       "172.32.0.1",
			expected: false,
		},
		// RFC1918 192.168.0.0/16
		{
			name:     "192.168.0.1 private",
			ip:       "192.168.0.1",
			expected: true,
		},
		{
			name:     "192.168.255.255 private",
			ip:       "192.168.255.255",
			expected: true,
		},
		{
			name:     "192.167.0.1 public (just below range)",
			ip:       "192.167.0.1",
			expected: false,
		},
		{
			name:     "192.169.0.1 public (just above range)",
			ip:       "192.169.0.1",
			expected: false,
		},
		// Loopback 127.0.0.0/8
		{
			name:     "127.0.0.1 loopback",
			ip:       "127.0.0.1",
			expected: true,
		},
		{
			name:     "127.0.0.0 loopback",
			ip:       "127.0.0.0",
			expected: true,
		},
		{
			name:     "127.255.255.255 loopback",
			ip:       "127.255.255.255",
			expected: true,
		},
		// Link-local 169.254.0.0/16
		{
			name:     "169.254.0.1 link-local",
			ip:       "169.254.0.1",
			expected: true,
		},
		{
			name:     "169.254.255.255 link-local",
			ip:       "169.254.255.255",
			expected: true,
		},
		// Public IPv4
		{
			name:     "8.8.8.8 public DNS",
			ip:       "8.8.8.8",
			expected: false,
		},
		{
			name:     "1.1.1.1 public DNS",
			ip:       "1.1.1.1",
			expected: false,
		},
		{
			name:     "93.184.216.34 example.com",
			ip:       "93.184.216.34",
			expected: false,
		},
		// IPv6 loopback
		{
			name:     "::1 IPv6 loopback",
			ip:       "::1",
			expected: true,
		},
		// IPv6 link-local
		{
			name:     "fe80::1 IPv6 link-local",
			ip:       "fe80::1",
			expected: true,
		},
		{
			name:     "fe80::1234:5678:abcd:ef00 IPv6 link-local",
			ip:       "fe80::1234:5678:abcd:ef00",
			expected: true,
		},
		// IPv6 unique local (fc00::/7)
		{
			name:     "fc00::1 IPv6 unique local",
			ip:       "fc00::1",
			expected: true,
		},
		{
			name:     "fd00::1 IPv6 unique local",
			ip:       "fd00::1",
			expected: true,
		},
		// Public IPv6
		{
			name:     "2001:4860:4860::8888 Google DNS IPv6",
			ip:       "2001:4860:4860::8888",
			expected: false,
		},
		{
			name:     "2606:4700:4700::1111 Cloudflare DNS IPv6",
			ip:       "2606:4700:4700::1111",
			expected: false,
		},
		// CGNAT (RFC6598) 100.64.0.0/10
		{
			name:     "100.64.0.1 CGNAT",
			ip:       "100.64.0.1",
			expected: true,
		},
		{
			name:     "100.127.255.254 CGNAT upper",
			ip:       "100.127.255.254",
			expected: true,
		},
		{
			name:     "100.128.0.1 outside CGNAT",
			ip:       "100.128.0.1",
			expected: false,
		},
		// Benchmarking (RFC2544) 198.18.0.0/15
		{
			name:     "198.18.0.1 benchmarking",
			ip:       "198.18.0.1",
			expected: true,
		},
		{
			name:     "198.19.255.254 benchmarking upper",
			ip:       "198.19.255.254",
			expected: true,
		},
		{
			name:     "198.20.0.1 outside benchmarking",
			ip:       "198.20.0.1",
			expected: false,
		},
		// "This" network (RFC1122) 0.0.0.0/8
		{
			name:     "0.0.0.0 this network",
			ip:       "0.0.0.0",
			expected: true,
		},
		// Documentation TEST-NETs (RFC5737)
		{
			name:     "192.0.2.1 TEST-NET-1",
			ip:       "192.0.2.1",
			expected: true,
		},
		{
			name:     "198.51.100.1 TEST-NET-2",
			ip:       "198.51.100.1",
			expected: true,
		},
		{
			name:     "203.0.113.1 TEST-NET-3",
			ip:       "203.0.113.1",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP %q", tt.ip)
			}
			result := isPrivateIP(ip)
			if result != tt.expected {
				t.Errorf("isPrivateIP(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestIsNumericIP(t *testing.T) {
	// NOTE: isNumericIP uses a simple heuristic that only checks for digits, dots, and colons.
	// It intentionally doesn't validate full IPv6 with hex digits (a-f).
	// This is acceptable because its purpose is to warn about HTTPS with IP addresses,
	// and the common case is IPv4 or simple IPv6 like ::1.
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// Valid IPv4 addresses
		{
			name:     "localhost IPv4",
			host:     "127.0.0.1",
			expected: true,
		},
		{
			name:     "public IPv4",
			host:     "8.8.8.8",
			expected: true,
		},
		{
			name:     "private IPv4",
			host:     "192.168.1.1",
			expected: true,
		},
		{
			name:     "all zeros IPv4",
			host:     "0.0.0.0",
			expected: true,
		},
		// IPv6 addresses (only numeric ones detected)
		{
			name:     "IPv6 loopback short",
			host:     "::1",
			expected: true,
		},
		{
			name:     "IPv6 all zeros",
			host:     "::",
			expected: true,
		},
		// IPv6 with hex letters - implementation returns false (known limitation)
		{
			name:     "IPv6 full with hex - returns false (heuristic limitation)",
			host:     "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expected: false, // contains hex letters
		},
		{
			name:     "IPv6 compressed with hex - returns false (heuristic limitation)",
			host:     "2001:db8::1",
			expected: false, // contains hex letters
		},
		// Hostnames (not numeric)
		{
			name:     "domain name",
			host:     "example.com",
			expected: false,
		},
		{
			name:     "localhost hostname",
			host:     "localhost",
			expected: false,
		},
		{
			name:     "subdomain",
			host:     "api.example.com",
			expected: false,
		},
		{
			name:     "hyphenated hostname",
			host:     "my-server.local",
			expected: false,
		},
		{
			name:     "hostname with numbers",
			host:     "server1.example.com",
			expected: false,
		},
		// Edge cases
		{
			name:     "empty string",
			host:     "",
			expected: false,
		},
		{
			name:     "only dots - looks like IP pattern",
			host:     "...",
			expected: true, // contains only dots with dots present
		},
		{
			name:     "only colons",
			host:     ":::",
			expected: true, // contains only colons with colons present
		},
		{
			name:     "single number - no separator",
			host:     "123",
			expected: false, // no dots or colons
		},
		{
			name:     "number with dot",
			host:     "123.",
			expected: true, // contains digit and dot
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumericIP(tt.host)
			if result != tt.expected {
				t.Errorf("isNumericIP(%q) = %v, want %v", tt.host, result, tt.expected)
			}
		})
	}
}

func TestIsEmptyInterface(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		// Nil
		{
			name:     "nil value",
			value:    nil,
			expected: true,
		},
		// Empty strings
		{
			name:     "empty string",
			value:    "",
			expected: true,
		},
		{
			name:     "whitespace only",
			value:    "   ",
			expected: true,
		},
		{
			name:     "tabs and spaces",
			value:    "\t  \t",
			expected: true,
		},
		// Non-empty strings
		{
			name:     "non-empty string",
			value:    "hello",
			expected: false,
		},
		{
			name:     "string with leading space",
			value:    " hello",
			expected: false,
		},
		// Other types (not empty)
		{
			name:     "integer zero",
			value:    0,
			expected: false,
		},
		{
			name:     "integer non-zero",
			value:    42,
			expected: false,
		},
		{
			name:     "boolean false",
			value:    false,
			expected: false,
		},
		{
			name:     "boolean true",
			value:    true,
			expected: false,
		},
		{
			name:     "empty slice",
			value:    []string{},
			expected: false,
		},
		{
			name:     "empty map",
			value:    map[string]string{},
			expected: false,
		},
		// fmt.Stringer types
		{
			name:     "stringer with empty string",
			value:    emptyStringer{},
			expected: true,
		},
		{
			name:     "stringer with whitespace",
			value:    whitespaceStringer{},
			expected: true,
		},
		{
			name:     "stringer with content",
			value:    contentStringer{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmptyInterface(tt.value)
			if result != tt.expected {
				t.Errorf("isEmptyInterface(%v) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

// Test helper types for fmt.Stringer interface
type emptyStringer struct{}

func (emptyStringer) String() string { return "" }

type whitespaceStringer struct{}

func (whitespaceStringer) String() string { return "   \t  " }

type contentStringer struct{}

func (contentStringer) String() string { return "content" }

func TestEnsurePushoverCustomFieldAliases(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty map returns empty",
			input:    map[string]interface{}{},
			expected: map[string]interface{}{},
		},
		{
			name: "token already set, no alias",
			input: map[string]interface{}{
				"token": "my-token",
			},
			expected: map[string]interface{}{
				"token": "my-token",
			},
		},
		{
			name: "user already set, no alias",
			input: map[string]interface{}{
				"user": "my-user",
			},
			expected: map[string]interface{}{
				"user": "my-user",
			},
		},
		{
			name: "app_token aliased to token",
			input: map[string]interface{}{
				"app_token": "legacy-token",
			},
			expected: map[string]interface{}{
				"app_token": "legacy-token",
				"token":     "legacy-token",
			},
		},
		{
			name: "user_token aliased to user",
			input: map[string]interface{}{
				"user_token": "legacy-user",
			},
			expected: map[string]interface{}{
				"user_token": "legacy-user",
				"user":       "legacy-user",
			},
		},
		{
			name: "both aliases applied",
			input: map[string]interface{}{
				"app_token":  "legacy-token",
				"user_token": "legacy-user",
			},
			expected: map[string]interface{}{
				"app_token":  "legacy-token",
				"user_token": "legacy-user",
				"token":      "legacy-token",
				"user":       "legacy-user",
			},
		},
		{
			name: "existing token takes precedence over app_token",
			input: map[string]interface{}{
				"token":     "primary-token",
				"app_token": "legacy-token",
			},
			expected: map[string]interface{}{
				"token":     "primary-token",
				"app_token": "legacy-token",
			},
		},
		{
			name: "existing user takes precedence over user_token",
			input: map[string]interface{}{
				"user":       "primary-user",
				"user_token": "legacy-user",
			},
			expected: map[string]interface{}{
				"user":       "primary-user",
				"user_token": "legacy-user",
			},
		},
		{
			name: "empty token uses app_token alias",
			input: map[string]interface{}{
				"token":     "",
				"app_token": "legacy-token",
			},
			expected: map[string]interface{}{
				"token":     "legacy-token",
				"app_token": "legacy-token",
			},
		},
		{
			name: "whitespace token uses app_token alias",
			input: map[string]interface{}{
				"token":     "   ",
				"app_token": "legacy-token",
			},
			expected: map[string]interface{}{
				"token":     "legacy-token",
				"app_token": "legacy-token",
			},
		},
		{
			name: "empty legacy values not aliased",
			input: map[string]interface{}{
				"app_token":  "",
				"user_token": "   ",
			},
			expected: map[string]interface{}{
				"app_token":  "",
				"user_token": "   ",
			},
		},
		{
			name: "other fields preserved",
			input: map[string]interface{}{
				"app_token": "legacy-token",
				"priority":  2,
				"sound":     "pushover",
			},
			expected: map[string]interface{}{
				"app_token": "legacy-token",
				"token":     "legacy-token",
				"priority":  2,
				"sound":     "pushover",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ensurePushoverCustomFieldAliases(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("ensurePushoverCustomFieldAliases() = %v, want nil", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("ensurePushoverCustomFieldAliases() length = %d, want %d", len(result), len(tt.expected))
			}

			for key, expectedValue := range tt.expected {
				actualValue, ok := result[key]
				if !ok {
					t.Errorf("ensurePushoverCustomFieldAliases() missing key %q", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("ensurePushoverCustomFieldAliases()[%q] = %v, want %v", key, actualValue, expectedValue)
				}
			}
		})
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
