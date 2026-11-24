package auditlog

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/proxytrust"
)

func TestClientIP(t *testing.T) {
	t.Setenv("CP_TRUSTED_PROXY_CIDRS", "127.0.0.1/32,10.0.0.0/8")
	proxytrust.ResetForTesting()

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
			name:       "X-Forwarded-For single IP from trusted proxy",
			request:    newRequestWithHeaders(t, "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "192.168.1.100"}),
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For multiple IPs selects right-most untrusted hop",
			request:    newRequestWithHeaders(t, "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "192.168.1.100, 10.0.0.1"}),
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For with spaces",
			request:    newRequestWithHeaders(t, "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "  192.168.1.100  , 10.0.0.1"}),
			expectedIP: "192.168.1.100",
		},
		{
			name:       "X-Forwarded-For ignored from untrusted remote",
			request:    newRequestWithHeaders(t, "198.51.100.10:1234", map[string]string{"X-Forwarded-For": "192.168.1.100"}),
			expectedIP: "198.51.100.10",
		},
		{
			name:       "X-Real-IP when no XFF from trusted proxy",
			request:    newRequestWithHeaders(t, "127.0.0.1:1234", map[string]string{"X-Real-IP": "10.0.0.5"}),
			expectedIP: "10.0.0.5",
		},
		{
			name:       "X-Real-IP takes precedence when XFF is empty",
			request:    newRequestWithHeaders(t, "127.0.0.1:1234", map[string]string{"X-Forwarded-For": "", "X-Real-IP": "10.0.0.5"}),
			expectedIP: "10.0.0.5",
		},
		{
			name:       "X-Real-IP with brackets stripped",
			request:    newRequestWithHeaders(t, "127.0.0.1:1234", map[string]string{"X-Real-IP": "[::1]"}),
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
	withActor := func(r *http.Request, actorID string) *http.Request {
		return r.WithContext(WithActor(r.Context(), actorID))
	}

	tests := []struct {
		name       string
		request    *http.Request
		expectedID string
	}{
		{
			name:       "nil request",
			request:    nil,
			expectedID: ActorAnonymous,
		},
		{
			name:       "no authenticated actor",
			request:    newRequestWithHeaders(t, "", nil),
			expectedID: ActorAnonymous,
		},
		{
			name:       "authenticated actor from context",
			request:    withActor(newRequestWithHeaders(t, "", nil), "user-123"),
			expectedID: "user-123",
		},
		{
			name:       "admin key actor from context",
			request:    withActor(newRequestWithHeaders(t, "", nil), ActorAdminKey),
			expectedID: ActorAdminKey,
		},
		{
			name:       "authenticated actor is trimmed",
			request:    withActor(newRequestWithHeaders(t, "", nil), "  user-456  "),
			expectedID: "user-456",
		},
		{
			name:       "whitespace-only actor is anonymous",
			request:    withActor(newRequestWithHeaders(t, "", nil), "   "),
			expectedID: ActorAnonymous,
		},
		{
			name:       "X-Actor-ID header is ignored",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-Actor-ID": "actor-1"}),
			expectedID: ActorAnonymous,
		},
		{
			name:       "X-User-ID header is ignored",
			request:    newRequestWithHeaders(t, "", map[string]string{"X-User-ID": "user-789"}),
			expectedID: ActorAnonymous,
		},
		{
			name:       "client headers do not override authenticated actor",
			request:    withActor(newRequestWithHeaders(t, "", map[string]string{"X-Actor-ID": "actor-1", "X-User-ID": "user-1"}), "user-123"),
			expectedID: "user-123",
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
