package auditlog

import (
	"net/http"
	"net/url"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		request    *http.Request
		expectedIP string
	}{
		{
			name:       "nil request",
			request:    nil,
			expectedIP: "",
		},
		{
			name:       "X-Forwarded-For single IP",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-Forwarded-For": "192.168.1.100"}),
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-Forwarded-For": "192.168.1.100, 10.0.0.1, 172.16.0.1"}),
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For with spaces",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-Forwarded-For": "  192.168.1.100  , 10.0.0.1"}),
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Real-IP when no XFF",
			request:    newRequestWithHeaders(t, "10.0.0.5:1234", map[string]string{"X-Real-IP": "10.0.0.5"}),
			expectedIP: "10.0.0.5",
		},
		{
			name:       "X-Real-IP takes precedence when XFF is empty",
			request:    newRequestWithHeaders(t, "10.0.0.5:1234", map[string]string{"X-Forwarded-For": "", "X-Real-IP": "10.0.0.5"}),
			expectedIP: "10.0.0.5",
		},
		{
			name:       "X-Real-IP with brackets stripped",
			request:    newRequestWithHeaders(t, "10.0.0.5:1234", map[string]string{"X-Real-IP": "[::1]"}),
			expectedIP: "::1",
		},
		{
			name:       "Fallback to RemoteAddr",
			request:    newRequestWithHeaders(t, "192.168.1.50:54321", nil),
			expectedIP: "192.168.1.50",
		},
		{
			name:       "RemoteAddr without port",
			request:    newRequestWithHeaders(t, "192.168.1.50", nil),
			expectedIP: "192.168.1.50",
		},
		{
			name:       "IPv6 RemoteAddr",
			request:    newRequestWithHeaders(t, "[::1]:8080", nil),
			expectedIP: "::1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ClientIP(tc.request)
			if result != tc.expectedIP {
				t.Errorf("ClientIP() = %q, want %q", result, tc.expectedIP)
			}
		})
	}
}

func TestActorID(t *testing.T) {
	tests := []struct {
		name       string
		request    *http.Request
		expectedID string
	}{
		{
			name:       "nil request",
			request:    nil,
			expectedID: "",
		},
		{
			name:       "X-Actor-ID header",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-Actor-ID": "user-123"}),
			expectedID: "user-123",
		},
		{
			name:       "X-Actor-Id header (mixed case)",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-Actor-Id": "user-456"}),
			expectedID: "user-456",
		},
		{
			name:       "X-User-ID fallback",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-User-ID": "user-789"}),
			expectedID: "user-789",
		},
		{
			name:       "X-User-Id fallback",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-User-Id": "user-abc"}),
			expectedID: "user-abc",
		},
		{
			name:       "X-Actor-ID takes precedence over X-User-ID",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-Actor-ID": "actor-1", "X-User-ID": "user-1"}),
			expectedID: "actor-1",
		},
		{
			name:       "whitespace-only returns empty",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-Actor-ID": "   "}),
			expectedID: "",
		},
		{
			name:       "no matching headers",
			request:    newRequestWithHeaders(t, "", map[string]string{"Accept": "application/json"}),
			expectedID: "",
		},
		{
			name:       "empty headers",
			request:    newRequestWithHeaders(t, "", nil),
			expectedID: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ActorID(tc.request)
			if result != tc.expectedID {
				t.Errorf("ActorID() = %q, want %q", result, tc.expectedID)
			}
		})
	}
}

func TestRequestPath(t *testing.T) {
	tests := []struct {
		name         string
		request      *http.Request
		expectedPath string
	}{
		{
			name:         "nil request",
			request:      nil,
			expectedPath: "",
		},
		{
			name:         "nil URL",
			request:      &http.Request{URL: nil},
			expectedPath: "",
		},
		{
			name:         "normal path",
			request:      newRequestWithPath(t, "/api/foo"),
			expectedPath: "/api/foo",
		},
		{
			name:         "root path",
			request:      newRequestWithPath(t, "/"),
			expectedPath: "/",
		},
		{
			name:         "empty path returns root",
			request:      newRequestWithPath(t, ""),
			expectedPath: "/",
		},
		{
			name:         "whitespace path returns root",
			request:      newRequestWithPath(t, "   "),
			expectedPath: "/",
		},
		{
			name:         "nested path",
			request:      newRequestWithPath(t, "/api/v1/users/123"),
			expectedPath: "/api/v1/users/123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := RequestPath(tc.request)
			if result != tc.expectedPath {
				t.Errorf("RequestPath() = %q, want %q", result, tc.expectedPath)
			}
		})
	}
}

// Helper functions

func newRequestWithHeaders(t *testing.T, remoteAddr string, headers map[string]string) *http.Request {
	t.Helper()
	req := &http.Request{}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	if headers != nil {
		req.Header = http.Header{}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	return req
}

func newRequestWithPath(t *testing.T, path string) *http.Request {
	t.Helper()
	return &http.Request{
		URL: &url.URL{Path: path},
	}
}
