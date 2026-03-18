package api

import (
	"errors"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestSanitizeInstallerURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
		errMsg  string
	}{
		// Valid cases
		{
			name:    "empty string returns empty",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "whitespace-only returns empty",
			input:   "   \t  ",
			want:    "",
			wantErr: false,
		},
		{
			name:    "valid http URL",
			input:   "http://example.com",
			want:    "http://example.com",
			wantErr: false,
		},
		{
			name:    "valid https URL",
			input:   "https://example.com",
			want:    "https://example.com",
			wantErr: false,
		},
		{
			name:    "URL with path",
			input:   "https://example.com/path/to/installer",
			want:    "https://example.com/path/to/installer",
			wantErr: false,
		},
		{
			name:    "URL with port",
			input:   "https://example.com:8080",
			want:    "https://example.com:8080",
			wantErr: false,
		},
		{
			name:    "URL with query params rejected",
			input:   "https://example.com/installer?version=1.0&arch=amd64",
			wantErr: true,
			errMsg:  "query parameters are not allowed",
		},
		{
			name:    "URL with fragment rejected",
			input:   "https://example.com/installer#section",
			wantErr: true,
			errMsg:  "fragment is not allowed",
		},
		{
			name:    "URL with fragment and query rejected",
			input:   "https://example.com/installer?version=1.0#section",
			wantErr: true,
			errMsg:  "query parameters are not allowed",
		},
		{
			name:    "URL with leading/trailing whitespace is trimmed",
			input:   "  https://example.com/installer  ",
			want:    "https://example.com/installer",
			wantErr: false,
		},
		{
			name:    "URL with authentication rejected",
			input:   "https://user:pass@example.com/installer",
			wantErr: true,
			errMsg:  "userinfo is not allowed",
		},

		// Error cases
		{
			name:    "carriage return in URL",
			input:   "https://example.com\r/installer",
			wantErr: true,
			errMsg:  "control characters",
		},
		{
			name:    "newline in URL",
			input:   "https://example.com\n/installer",
			wantErr: true,
			errMsg:  "control characters",
		},
		{
			name:    "both CR and LF in URL",
			input:   "https://example.com\r\n/installer",
			wantErr: true,
			errMsg:  "control characters",
		},
		{
			name:    "ftp scheme rejected",
			input:   "ftp://example.com/installer",
			wantErr: true,
			errMsg:  "scheme must be http or https",
		},
		{
			name:    "file scheme rejected",
			input:   "file:///etc/passwd",
			wantErr: true,
			errMsg:  "scheme must be http or https",
		},
		{
			name:    "javascript scheme rejected",
			input:   "javascript:alert(1)",
			wantErr: true,
			errMsg:  "scheme must be http or https",
		},
		{
			name:    "data scheme rejected",
			input:   "data:text/plain,hello",
			wantErr: true,
			errMsg:  "scheme must be http or https",
		},
		{
			name:    "URL without scheme",
			input:   "example.com/installer",
			wantErr: true,
			errMsg:  "scheme must be http or https",
		},
		{
			name:    "URL without host",
			input:   "https:///path",
			wantErr: true,
			errMsg:  "host component is required",
		},
		{
			name:    "relative URL without host",
			input:   "/path/to/installer",
			wantErr: true,
			errMsg:  "scheme must be http or https",
		},
		{
			name:    "malformed URL",
			input:   "https://[invalid",
			wantErr: true,
			errMsg:  "failed to parse URL",
		},
		{
			name:    "URL with only scheme",
			input:   "https://",
			wantErr: true,
			errMsg:  "host component is required",
		},
		{
			name:    "URL host with shell expansion rejected",
			input:   "https://example$(id).com",
			wantErr: true,
			errMsg:  "invalid characters",
		},
		{
			name:    "URL path with shell expansion rejected",
			input:   "https://example.com/path$(id)",
			wantErr: true,
			errMsg:  "shell-expansion characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeInstallerURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("sanitizeInstallerURL() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("sanitizeInstallerURL() error = %q, want substring %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("sanitizeInstallerURL() unexpected error = %v", err)
					return
				}
				if got != tt.want {
					t.Errorf("sanitizeInstallerURL() = %q, want %q", got, tt.want)
				}
			}
		})
	}
}

func TestSanitizeSetupAuthToken(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
		errMsg  string
	}{
		// Valid cases
		{
			name:    "empty string returns empty",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "whitespace-only returns empty",
			input:   "   \t  ",
			want:    "",
			wantErr: false,
		},
		{
			name:    "valid 32-char hex token lowercase",
			input:   "0123456789abcdef0123456789abcdef",
			want:    "0123456789abcdef0123456789abcdef",
			wantErr: false,
		},
		{
			name:    "valid 32-char hex token uppercase",
			input:   "0123456789ABCDEF0123456789ABCDEF",
			want:    "0123456789ABCDEF0123456789ABCDEF",
			wantErr: false,
		},
		{
			name:    "valid 32-char hex token mixed case",
			input:   "0123456789aBcDeF0123456789AbCdEf",
			want:    "0123456789aBcDeF0123456789AbCdEf",
			wantErr: false,
		},
		{
			name:    "valid 64-char hex token",
			input:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantErr: false,
		},
		{
			name:    "valid 128-char hex token (max length)",
			input:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			wantErr: false,
		},
		{
			name:    "valid hex token with leading/trailing whitespace is trimmed",
			input:   "  0123456789abcdef0123456789abcdef  ",
			want:    "0123456789abcdef0123456789abcdef",
			wantErr: false,
		},

		// Error cases
		{
			name:    "carriage return in token",
			input:   "0123456789abcdef\r0123456789abcdef",
			wantErr: true,
			errMsg:  "control characters",
		},
		{
			name:    "newline in token",
			input:   "0123456789abcdef\n0123456789abcdef",
			wantErr: true,
			errMsg:  "control characters",
		},
		{
			name:    "both CR and LF in token",
			input:   "0123456789abcdef\r\n123456789abcdef",
			wantErr: true,
			errMsg:  "control characters",
		},
		{
			name:    "non-hex character g",
			input:   "0123456789abcdefg123456789abcdef",
			wantErr: true,
			errMsg:  "hexadecimal",
		},
		{
			name:    "non-hex character z",
			input:   "0123456789abcdefz123456789abcdef",
			wantErr: true,
			errMsg:  "hexadecimal",
		},
		{
			name:    "token with spaces",
			input:   "0123456789abcdef 0123456789abcdef",
			wantErr: true,
			errMsg:  "hexadecimal",
		},
		{
			name:    "token with dash",
			input:   "0123456789abcdef-0123456789abcdef",
			wantErr: true,
			errMsg:  "hexadecimal",
		},
		{
			name:    "token too short (31 chars)",
			input:   "0123456789abcdef0123456789abcde",
			wantErr: true,
			errMsg:  "hexadecimal",
		},
		{
			name:    "token too long (129 chars)",
			input:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0",
			wantErr: true,
			errMsg:  "hexadecimal",
		},
		{
			name:    "special characters",
			input:   "0123456789abcdef!@#$%^&*()abcdef",
			wantErr: true,
			errMsg:  "hexadecimal",
		},
		{
			name:    "empty hex string (just numbers)",
			input:   "12345678901234567890123456789012",
			want:    "12345678901234567890123456789012",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeSetupAuthToken(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("sanitizeSetupAuthToken() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("sanitizeSetupAuthToken() error = %q, want substring %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("sanitizeSetupAuthToken() unexpected error = %v", err)
					return
				}
				if got != tt.want {
					t.Errorf("sanitizeSetupAuthToken() = %q, want %q", got, tt.want)
				}
			}
		})
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		operation string
		expected  string
	}{
		{
			name:      "create_client operation",
			err:       errors.New("connection refused"),
			operation: "create_client",
			expected:  "Failed to initialize connection",
		},
		{
			name:      "connection operation",
			err:       errors.New("network unreachable"),
			operation: "connection",
			expected:  "Connection failed. Please check your credentials and network settings",
		},
		{
			name:      "validation operation",
			err:       errors.New("invalid field"),
			operation: "validation",
			expected:  "Invalid configuration",
		},
		{
			name:      "unknown operation",
			err:       errors.New("something went wrong"),
			operation: "unknown_operation",
			expected:  "Operation failed",
		},
		{
			name:      "empty operation string",
			err:       errors.New("error"),
			operation: "",
			expected:  "Operation failed",
		},
		{
			name:      "nil error still returns message",
			err:       nil,
			operation: "create_client",
			expected:  "Failed to initialize connection",
		},
		{
			name:      "detailed error hidden from response",
			err:       errors.New("x509: certificate signed by unknown authority"),
			operation: "connection",
			expected:  "Connection failed. Please check your credentials and network settings",
		},
		{
			name:      "sensitive error hidden from response",
			err:       errors.New("password: secret123"),
			operation: "validation",
			expected:  "Invalid configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := sanitizeErrorMessage(tt.err, tt.operation)
			if result != tt.expected {
				t.Errorf("sanitizeErrorMessage() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFindExistingGuestURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nodeName  string
		endpoints []config.ClusterEndpoint
		expected  string
	}{
		{
			name:      "empty endpoints",
			nodeName:  "node1",
			endpoints: []config.ClusterEndpoint{},
			expected:  "",
		},
		{
			name:      "nil endpoints",
			nodeName:  "node1",
			endpoints: nil,
			expected:  "",
		},
		{
			name:     "exact match",
			nodeName: "pve1",
			endpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", GuestURL: "https://pve1.local:8006"},
				{NodeName: "pve2", GuestURL: "https://pve2.local:8006"},
			},
			expected: "https://pve1.local:8006",
		},
		{
			name:     "match second node",
			nodeName: "pve2",
			endpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", GuestURL: "https://pve1.local:8006"},
				{NodeName: "pve2", GuestURL: "https://pve2.local:8006"},
			},
			expected: "https://pve2.local:8006",
		},
		{
			name:     "no match",
			nodeName: "pve3",
			endpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", GuestURL: "https://pve1.local:8006"},
				{NodeName: "pve2", GuestURL: "https://pve2.local:8006"},
			},
			expected: "",
		},
		{
			name:     "empty node name",
			nodeName: "",
			endpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", GuestURL: "https://pve1.local:8006"},
			},
			expected: "",
		},
		{
			name:     "case sensitive match",
			nodeName: "PVE1",
			endpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", GuestURL: "https://pve1.local:8006"},
			},
			expected: "",
		},
		{
			name:     "returns first match when duplicates exist",
			nodeName: "pve1",
			endpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", GuestURL: "https://first.local:8006"},
				{NodeName: "pve1", GuestURL: "https://second.local:8006"},
			},
			expected: "https://first.local:8006",
		},
		{
			name:     "empty GuestURL returned when matched",
			nodeName: "pve1",
			endpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", GuestURL: ""},
			},
			expected: "",
		},
		{
			name:     "single endpoint match",
			nodeName: "pve1",
			endpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", GuestURL: "https://pve1.local:8006"},
			},
			expected: "https://pve1.local:8006",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := findExistingGuestURL(tt.nodeName, tt.endpoints)
			if result != tt.expected {
				t.Errorf("findExistingGuestURL(%q, endpoints) = %q, want %q", tt.nodeName, result, tt.expected)
			}
		})
	}
}
