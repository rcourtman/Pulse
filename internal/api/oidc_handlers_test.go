package api

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestSanitizeOIDCReturnTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		// Valid paths
		{
			name: "simple path",
			raw:  "/dashboard",
			want: "/dashboard",
		},
		{
			name: "root path",
			raw:  "/",
			want: "/",
		},
		{
			name: "nested path",
			raw:  "/settings/alerts",
			want: "/settings/alerts",
		},
		{
			name: "path with query params",
			raw:  "/page?foo=bar",
			want: "/page?foo=bar",
		},
		{
			name: "path with fragment",
			raw:  "/page#section",
			want: "/page#section",
		},

		// Invalid - empty or whitespace
		{
			name: "empty string",
			raw:  "",
			want: "",
		},
		{
			name: "whitespace only",
			raw:  "   ",
			want: "",
		},

		// Invalid - doesn't start with /
		{
			name: "no leading slash",
			raw:  "dashboard",
			want: "",
		},
		{
			name: "http URL",
			raw:  "http://evil.com",
			want: "",
		},
		{
			name: "https URL",
			raw:  "https://evil.com",
			want: "",
		},

		// Invalid - protocol-relative URL (double slash)
		{
			name: "protocol relative URL",
			raw:  "//evil.com",
			want: "",
		},
		{
			name: "protocol relative with path",
			raw:  "//evil.com/path",
			want: "",
		},

		// Whitespace handling
		{
			name: "leading whitespace",
			raw:  "  /dashboard",
			want: "/dashboard",
		},
		{
			name: "trailing whitespace",
			raw:  "/dashboard  ",
			want: "/dashboard",
		},
		{
			name: "both whitespace",
			raw:  "  /dashboard  ",
			want: "/dashboard",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := sanitizeOIDCReturnTo(tc.raw)
			if result != tc.want {
				t.Errorf("sanitizeOIDCReturnTo(%q) = %q, want %q", tc.raw, result, tc.want)
			}
		})
	}
}

func TestAddQueryParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		path  string
		key   string
		value string
		want  string
	}{
		// Basic cases
		{
			name:  "add to simple path",
			path:  "/dashboard",
			key:   "foo",
			value: "bar",
			want:  "/dashboard?foo=bar",
		},
		{
			name:  "add to root",
			path:  "/",
			key:   "key",
			value: "value",
			want:  "/?key=value",
		},

		// Existing query params
		{
			name:  "add to path with existing param",
			path:  "/page?existing=param",
			key:   "new",
			value: "value",
			want:  "/page?existing=param&new=value",
		},
		{
			name:  "replace existing param",
			path:  "/page?key=old",
			key:   "key",
			value: "new",
			want:  "/page?key=new",
		},

		// Empty path
		{
			name:  "empty path becomes root",
			path:  "",
			key:   "foo",
			value: "bar",
			want:  "/?foo=bar",
		},

		// URL encoding
		{
			name:  "value with spaces",
			path:  "/page",
			key:   "message",
			value: "hello world",
			want:  "/page?message=hello+world",
		},
		{
			name:  "value with special chars",
			path:  "/page",
			key:   "data",
			value: "a=b&c=d",
			want:  "/page?data=a%3Db%26c%3Dd",
		},

		// Fragment handling
		{
			name:  "path with fragment",
			path:  "/page#section",
			key:   "foo",
			value: "bar",
			want:  "/page?foo=bar#section",
		},

		// Invalid URL (control character causes parse error)
		{
			name:  "path with control character returns unchanged",
			path:  "/page\x00invalid",
			key:   "foo",
			value: "bar",
			want:  "/page\x00invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := addQueryParam(tc.path, tc.key, tc.value)
			if result != tc.want {
				t.Errorf("addQueryParam(%q, %q, %q) = %q, want %q", tc.path, tc.key, tc.value, result, tc.want)
			}
		})
	}
}

func TestExtractStringClaim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims map[string]any
		key    string
		want   string
	}{
		// String value
		{
			name:   "string value",
			claims: map[string]any{"email": "user@example.com"},
			key:    "email",
			want:   "user@example.com",
		},
		{
			name:   "string with whitespace",
			claims: map[string]any{"name": "  John Doe  "},
			key:    "name",
			want:   "John Doe",
		},

		// String slice - returns first element
		{
			name:   "string slice returns first",
			claims: map[string]any{"groups": []string{"admin", "users"}},
			key:    "groups",
			want:   "admin",
		},
		{
			name:   "empty string slice",
			claims: map[string]any{"groups": []string{}},
			key:    "groups",
			want:   "",
		},
		{
			name:   "string slice with whitespace",
			claims: map[string]any{"groups": []string{"  admin  "}},
			key:    "groups",
			want:   "admin",
		},

		// Interface slice - returns first string
		{
			name:   "interface slice with strings",
			claims: map[string]any{"roles": []interface{}{"admin", "user"}},
			key:    "roles",
			want:   "admin",
		},
		{
			name:   "interface slice with mixed types",
			claims: map[string]any{"data": []interface{}{123, "value", true}},
			key:    "data",
			want:   "value",
		},
		{
			name:   "interface slice with no strings",
			claims: map[string]any{"nums": []interface{}{1, 2, 3}},
			key:    "nums",
			want:   "",
		},

		// Missing or empty key
		{
			name:   "key not in claims",
			claims: map[string]any{"other": "value"},
			key:    "email",
			want:   "",
		},
		{
			name:   "empty key",
			claims: map[string]any{"email": "user@example.com"},
			key:    "",
			want:   "",
		},
		{
			name:   "nil claims",
			claims: nil,
			key:    "email",
			want:   "",
		},

		// Unsupported types
		{
			name:   "integer value",
			claims: map[string]any{"count": 42},
			key:    "count",
			want:   "",
		},
		{
			name:   "boolean value",
			claims: map[string]any{"active": true},
			key:    "active",
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := extractStringClaim(tc.claims, tc.key)
			if result != tc.want {
				t.Errorf("extractStringClaim(%v, %q) = %q, want %q", tc.claims, tc.key, result, tc.want)
			}
		})
	}
}

func TestExtractStringSliceClaim(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		claims map[string]any
		key    string
		want   []string
	}{
		// String slice
		{
			name:   "string slice",
			claims: map[string]any{"groups": []string{"admin", "users", "devops"}},
			key:    "groups",
			want:   []string{"admin", "users", "devops"},
		},
		{
			name:   "empty string slice",
			claims: map[string]any{"groups": []string{}},
			key:    "groups",
			want:   []string{},
		},

		// Interface slice
		{
			name:   "interface slice with strings",
			claims: map[string]any{"roles": []interface{}{"admin", "user"}},
			key:    "roles",
			want:   []string{"admin", "user"},
		},
		{
			name:   "interface slice with mixed types filters non-strings",
			claims: map[string]any{"data": []interface{}{"str1", 123, "str2", true}},
			key:    "data",
			want:   []string{"str1", "str2"},
		},
		{
			name:   "interface slice with no strings",
			claims: map[string]any{"nums": []interface{}{1, 2, 3}},
			key:    "nums",
			want:   []string{},
		},

		// String value (comma/space separated)
		{
			name:   "comma separated string",
			claims: map[string]any{"groups": "admin,users,devops"},
			key:    "groups",
			want:   []string{"admin", "users", "devops"},
		},
		{
			name:   "space separated string",
			claims: map[string]any{"groups": "admin users devops"},
			key:    "groups",
			want:   []string{"admin", "users", "devops"},
		},
		{
			name:   "mixed separator string",
			claims: map[string]any{"groups": "admin, users devops"},
			key:    "groups",
			want:   []string{"admin", "users", "devops"},
		},

		// Missing or empty key
		{
			name:   "key not in claims",
			claims: map[string]any{"other": "value"},
			key:    "groups",
			want:   nil,
		},
		{
			name:   "empty key",
			claims: map[string]any{"groups": []string{"admin"}},
			key:    "",
			want:   nil,
		},
		{
			name:   "nil claims",
			claims: nil,
			key:    "groups",
			want:   nil,
		},

		// Unsupported types
		{
			name:   "integer value",
			claims: map[string]any{"count": 42},
			key:    "count",
			want:   nil,
		},
		{
			name:   "boolean value",
			claims: map[string]any{"active": true},
			key:    "active",
			want:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := extractStringSliceClaim(tc.claims, tc.key)

			// Compare slices
			if tc.want == nil {
				if result != nil {
					t.Errorf("extractStringSliceClaim(%v, %q) = %v, want nil", tc.claims, tc.key, result)
				}
				return
			}

			if len(result) != len(tc.want) {
				t.Errorf("extractStringSliceClaim(%v, %q) = %v (len %d), want %v (len %d)",
					tc.claims, tc.key, result, len(result), tc.want, len(tc.want))
				return
			}

			for i, v := range result {
				if v != tc.want[i] {
					t.Errorf("extractStringSliceClaim(%v, %q)[%d] = %q, want %q",
						tc.claims, tc.key, i, v, tc.want[i])
				}
			}
		})
	}
}

func TestMatchesValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		candidate string
		allowed   []string
		want      bool
	}{
		// Matches
		{
			name:      "exact match",
			candidate: "admin",
			allowed:   []string{"admin", "user"},
			want:      true,
		},
		{
			name:      "case insensitive match",
			candidate: "ADMIN",
			allowed:   []string{"admin", "user"},
			want:      true,
		},
		{
			name:      "candidate with whitespace",
			candidate: "  admin  ",
			allowed:   []string{"admin"},
			want:      true,
		},
		{
			name:      "allowed with whitespace",
			candidate: "admin",
			allowed:   []string{"  admin  "},
			want:      true,
		},
		{
			name:      "mixed case both sides",
			candidate: "AdMiN",
			allowed:   []string{"aDmIn"},
			want:      true,
		},

		// No match
		{
			name:      "no match",
			candidate: "guest",
			allowed:   []string{"admin", "user"},
			want:      false,
		},
		{
			name:      "empty candidate",
			candidate: "",
			allowed:   []string{"admin"},
			want:      false,
		},
		{
			name:      "whitespace candidate",
			candidate: "   ",
			allowed:   []string{"admin"},
			want:      false,
		},
		{
			name:      "empty allowed list",
			candidate: "admin",
			allowed:   []string{},
			want:      false,
		},
		{
			name:      "nil allowed list",
			candidate: "admin",
			allowed:   nil,
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := matchesValue(tc.candidate, tc.allowed)
			if result != tc.want {
				t.Errorf("matchesValue(%q, %v) = %v, want %v", tc.candidate, tc.allowed, result, tc.want)
			}
		})
	}
}

func TestMatchesDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		email   string
		allowed []string
		want    bool
	}{
		// Matches
		{
			name:    "exact domain match",
			email:   "user@example.com",
			allowed: []string{"example.com"},
			want:    true,
		},
		{
			name:    "domain with @ prefix in allowed",
			email:   "user@example.com",
			allowed: []string{"@example.com"},
			want:    true,
		},
		{
			name:    "case insensitive email",
			email:   "USER@EXAMPLE.COM",
			allowed: []string{"example.com"},
			want:    true,
		},
		{
			name:    "case insensitive allowed",
			email:   "user@example.com",
			allowed: []string{"EXAMPLE.COM"},
			want:    true,
		},
		{
			name:    "multiple allowed domains",
			email:   "user@company.org",
			allowed: []string{"example.com", "company.org", "test.net"},
			want:    true,
		},
		{
			name:    "email with whitespace",
			email:   "  user@example.com  ",
			allowed: []string{"example.com"},
			want:    true,
		},
		{
			name:    "allowed with whitespace",
			email:   "user@example.com",
			allowed: []string{"  example.com  "},
			want:    true,
		},

		// No match
		{
			name:    "different domain",
			email:   "user@other.com",
			allowed: []string{"example.com"},
			want:    false,
		},
		{
			name:    "subdomain not matched",
			email:   "user@sub.example.com",
			allowed: []string{"example.com"},
			want:    false,
		},

		// Invalid emails
		{
			name:    "empty email",
			email:   "",
			allowed: []string{"example.com"},
			want:    false,
		},
		{
			name:    "whitespace email",
			email:   "   ",
			allowed: []string{"example.com"},
			want:    false,
		},
		{
			name:    "no @ in email",
			email:   "userexample.com",
			allowed: []string{"example.com"},
			want:    false,
		},
		{
			name:    "@ at end",
			email:   "user@",
			allowed: []string{"example.com"},
			want:    false,
		},

		// Empty allowed
		{
			name:    "empty allowed list",
			email:   "user@example.com",
			allowed: []string{},
			want:    false,
		},
		{
			name:    "nil allowed list",
			email:   "user@example.com",
			allowed: nil,
			want:    false,
		},
		{
			name:    "allowed with empty strings",
			email:   "user@example.com",
			allowed: []string{"", "   ", "@"},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := matchesDomain(tc.email, tc.allowed)
			if result != tc.want {
				t.Errorf("matchesDomain(%q, %v) = %v, want %v", tc.email, tc.allowed, result, tc.want)
			}
		})
	}
}

func TestIntersects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  []string
		allowed []string
		want    bool
	}{
		// Intersects
		{
			name:    "single common element",
			values:  []string{"admin", "user"},
			allowed: []string{"admin", "guest"},
			want:    true,
		},
		{
			name:    "multiple common elements",
			values:  []string{"admin", "user", "devops"},
			allowed: []string{"admin", "user", "guest"},
			want:    true,
		},
		{
			name:    "case insensitive",
			values:  []string{"ADMIN"},
			allowed: []string{"admin"},
			want:    true,
		},
		{
			name:    "with whitespace",
			values:  []string{"  admin  "},
			allowed: []string{"admin"},
			want:    true,
		},

		// No intersection
		{
			name:    "no common elements",
			values:  []string{"admin", "user"},
			allowed: []string{"guest", "viewer"},
			want:    false,
		},
		{
			name:    "empty values",
			values:  []string{},
			allowed: []string{"admin"},
			want:    false,
		},
		{
			name:    "nil values",
			values:  nil,
			allowed: []string{"admin"},
			want:    false,
		},
		{
			name:    "empty allowed",
			values:  []string{"admin"},
			allowed: []string{},
			want:    false,
		},
		{
			name:    "nil allowed",
			values:  []string{"admin"},
			allowed: nil,
			want:    false,
		},
		{
			name:    "both empty",
			values:  []string{},
			allowed: []string{},
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := intersects(tc.values, tc.allowed)
			if result != tc.want {
				t.Errorf("intersects(%v, %v) = %v, want %v", tc.values, tc.allowed, result, tc.want)
			}
		})
	}
}

func TestBuildRedirectURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		configuredURL string
		host          string
		tls           bool
		xForwardProto string
		xForwardHost  string
		wantContains  string
	}{
		// Configured URL takes precedence
		{
			name:          "configured URL used",
			configuredURL: "https://configured.example.com/auth/callback",
			host:          "request.example.com",
			tls:           false,
			wantContains:  "https://configured.example.com/auth/callback",
		},
		{
			name:          "configured URL with whitespace",
			configuredURL: "  https://configured.example.com/auth/callback  ",
			host:          "request.example.com",
			tls:           false,
			wantContains:  "https://configured.example.com/auth/callback",
		},

		// Build from request - no proxies
		{
			name:          "http request",
			configuredURL: "",
			host:          "example.com:7655",
			tls:           false,
			wantContains:  "http://example.com:7655",
		},
		{
			name:          "https request with TLS",
			configuredURL: "",
			host:          "example.com:7655",
			tls:           true,
			wantContains:  "https://example.com:7655",
		},

		// Build from request - with proxies
		{
			name:          "X-Forwarded-Proto overrides scheme",
			configuredURL: "",
			host:          "example.com",
			tls:           false,
			xForwardProto: "https",
			wantContains:  "https://example.com",
		},
		{
			name:          "X-Forwarded-Host overrides host",
			configuredURL: "",
			host:          "internal.local",
			tls:           false,
			xForwardHost:  "public.example.com",
			wantContains:  "http://public.example.com",
		},
		{
			name:          "both X-Forwarded headers",
			configuredURL: "",
			host:          "internal.local",
			tls:           false,
			xForwardProto: "https",
			xForwardHost:  "public.example.com",
			wantContains:  "https://public.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, _ := http.NewRequest("GET", "http://"+tc.host+"/", nil)
			req.Host = tc.host
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tc.xForwardProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.xForwardProto)
			}
			if tc.xForwardHost != "" {
				req.Header.Set("X-Forwarded-Host", tc.xForwardHost)
			}

			result := buildRedirectURL(req, tc.configuredURL)

			if tc.wantContains != "" {
				if result != tc.wantContains && !stringContains(result, tc.wantContains) {
					t.Errorf("buildRedirectURL() = %q, want to contain %q", result, tc.wantContains)
				}
			}
		})
	}
}

// stringContains checks if s contains substr (helper for tests)
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstrInStr(s, substr)))
}

func findSubstrInStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
