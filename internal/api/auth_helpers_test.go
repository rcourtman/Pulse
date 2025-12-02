package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestDetectProxy(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    bool
	}{
		{
			name:    "no proxy headers",
			headers: map[string]string{},
			want:    false,
		},
		{
			name:    "X-Forwarded-For present",
			headers: map[string]string{"X-Forwarded-For": "192.168.1.1"},
			want:    true,
		},
		{
			name:    "X-Real-IP present",
			headers: map[string]string{"X-Real-IP": "10.0.0.1"},
			want:    true,
		},
		{
			name:    "X-Forwarded-Proto present",
			headers: map[string]string{"X-Forwarded-Proto": "https"},
			want:    true,
		},
		{
			name:    "X-Forwarded-Host present",
			headers: map[string]string{"X-Forwarded-Host": "example.com"},
			want:    true,
		},
		{
			name:    "Forwarded header present (RFC 7239)",
			headers: map[string]string{"Forwarded": "for=192.168.1.1;proto=https"},
			want:    true,
		},
		{
			name:    "Cloudflare CF-Ray present",
			headers: map[string]string{"CF-Ray": "abc123"},
			want:    true,
		},
		{
			name:    "Cloudflare CF-Connecting-IP present",
			headers: map[string]string{"CF-Connecting-IP": "1.2.3.4"},
			want:    true,
		},
		{
			name:    "X-Forwarded-Server present",
			headers: map[string]string{"X-Forwarded-Server": "proxy.example.com"},
			want:    true,
		},
		{
			name:    "X-Forwarded-Port present",
			headers: map[string]string{"X-Forwarded-Port": "443"},
			want:    true,
		},
		{
			name: "multiple proxy headers",
			headers: map[string]string{
				"X-Forwarded-For":   "192.168.1.1",
				"X-Forwarded-Proto": "https",
				"CF-Ray":            "abc123",
			},
			want: true,
		},
		{
			name:    "unrelated headers only",
			headers: map[string]string{"Content-Type": "application/json", "User-Agent": "test"},
			want:    false,
		},
		{
			name:    "empty X-Forwarded-For",
			headers: map[string]string{"X-Forwarded-For": ""},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			got := detectProxy(req)
			if got != tt.want {
				t.Errorf("detectProxy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConnectionSecure(t *testing.T) {
	tests := []struct {
		name       string
		useTLS     bool
		headers    map[string]string
		want       bool
	}{
		{
			name:    "plain HTTP no headers",
			useTLS:  false,
			headers: map[string]string{},
			want:    false,
		},
		{
			name:    "TLS connection",
			useTLS:  true,
			headers: map[string]string{},
			want:    true,
		},
		{
			name:    "X-Forwarded-Proto https",
			useTLS:  false,
			headers: map[string]string{"X-Forwarded-Proto": "https"},
			want:    true,
		},
		{
			name:    "X-Forwarded-Proto http",
			useTLS:  false,
			headers: map[string]string{"X-Forwarded-Proto": "http"},
			want:    false,
		},
		{
			name:    "X-Forwarded-Proto HTTPS uppercase",
			useTLS:  false,
			headers: map[string]string{"X-Forwarded-Proto": "HTTPS"},
			want:    false, // strict comparison
		},
		{
			name:    "Forwarded header with proto=https",
			useTLS:  false,
			headers: map[string]string{"Forwarded": "for=192.168.1.1;proto=https"},
			want:    true,
		},
		{
			name:    "Forwarded header with proto=http",
			useTLS:  false,
			headers: map[string]string{"Forwarded": "for=192.168.1.1;proto=http"},
			want:    false,
		},
		{
			name:    "TLS with X-Forwarded-Proto http (TLS takes precedence)",
			useTLS:  true,
			headers: map[string]string{"X-Forwarded-Proto": "http"},
			want:    true,
		},
		{
			name: "both X-Forwarded-Proto and Forwarded present",
			useTLS: false,
			headers: map[string]string{
				"X-Forwarded-Proto": "https",
				"Forwarded":         "proto=http",
			},
			want: true, // X-Forwarded-Proto is checked first
		},
		{
			name:    "empty X-Forwarded-Proto",
			useTLS:  false,
			headers: map[string]string{"X-Forwarded-Proto": ""},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			if tt.useTLS {
				req.TLS = &tls.ConnectionState{}
			}

			got := isConnectionSecure(req)
			if got != tt.want {
				t.Errorf("isConnectionSecure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCookieSettings(t *testing.T) {
	tests := []struct {
		name         string
		useTLS       bool
		headers      map[string]string
		wantSecure   bool
		wantSameSite http.SameSite
	}{
		{
			name:         "plain HTTP no proxy",
			useTLS:       false,
			headers:      map[string]string{},
			wantSecure:   false,
			wantSameSite: http.SameSiteLaxMode,
		},
		{
			name:         "HTTPS direct connection",
			useTLS:       true,
			headers:      map[string]string{},
			wantSecure:   true,
			wantSameSite: http.SameSiteLaxMode,
		},
		{
			name:   "HTTPS through proxy",
			useTLS: false,
			headers: map[string]string{
				"X-Forwarded-For":   "192.168.1.1",
				"X-Forwarded-Proto": "https",
			},
			wantSecure:   true,
			wantSameSite: http.SameSiteNoneMode,
		},
		{
			name:   "HTTP through proxy",
			useTLS: false,
			headers: map[string]string{
				"X-Forwarded-For":   "192.168.1.1",
				"X-Forwarded-Proto": "http",
			},
			wantSecure:   false,
			wantSameSite: http.SameSiteLaxMode,
		},
		{
			name:   "Cloudflare tunnel HTTPS",
			useTLS: false,
			headers: map[string]string{
				"CF-Ray":            "abc123",
				"CF-Connecting-IP":  "1.2.3.4",
				"X-Forwarded-Proto": "https",
			},
			wantSecure:   true,
			wantSameSite: http.SameSiteNoneMode,
		},
		{
			name:   "proxy detected but no proto header",
			useTLS: false,
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.1",
			},
			wantSecure:   false,
			wantSameSite: http.SameSiteLaxMode,
		},
		{
			name:   "direct TLS with Forwarded header",
			useTLS: true,
			headers: map[string]string{
				"Forwarded": "for=192.168.1.1;proto=https",
			},
			wantSecure:   true,
			wantSameSite: http.SameSiteNoneMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			if tt.useTLS {
				req.TLS = &tls.ConnectionState{}
			}

			gotSecure, gotSameSite := getCookieSettings(req)
			if gotSecure != tt.wantSecure {
				t.Errorf("getCookieSettings() secure = %v, want %v", gotSecure, tt.wantSecure)
			}
			if gotSameSite != tt.wantSameSite {
				t.Errorf("getCookieSettings() sameSite = %v, want %v", gotSameSite, tt.wantSameSite)
			}
		})
	}
}

func TestGenerateSessionToken(t *testing.T) {
	// Test that tokens are generated
	token := generateSessionToken()
	if token == "" {
		t.Error("generateSessionToken() returned empty string")
	}

	// Test token length (32 bytes = 64 hex characters)
	if len(token) != 64 {
		t.Errorf("generateSessionToken() length = %d, want 64", len(token))
	}

	// Test that tokens are unique
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok := generateSessionToken()
		if tokens[tok] {
			t.Errorf("generateSessionToken() generated duplicate token: %s", tok)
		}
		tokens[tok] = true
	}

	// Test that token is valid hex
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("generateSessionToken() contains non-hex character: %c", c)
		}
	}
}

func TestValidateSession_NonExistentToken(t *testing.T) {
	// Ensure session store is initialized
	InitSessionStore(t.TempDir())

	// A random token that doesn't exist should return false
	result := ValidateSession("nonexistent-token-12345")
	if result {
		t.Error("ValidateSession should return false for non-existent token")
	}
}

func TestValidateSession_EmptyToken(t *testing.T) {
	InitSessionStore(t.TempDir())

	result := ValidateSession("")
	if result {
		t.Error("ValidateSession should return false for empty token")
	}
}

func TestValidateSession_ValidToken(t *testing.T) {
	dir := t.TempDir()
	InitSessionStore(dir)

	// Create a valid session with a generated token
	store := GetSessionStore()
	token := generateSessionToken()
	store.CreateSession(token, 24*time.Hour, "test-agent", "127.0.0.1")

	result := ValidateSession(token)
	if !result {
		t.Error("ValidateSession should return true for valid token")
	}
}

func TestValidateSession_ExpiredToken(t *testing.T) {
	dir := t.TempDir()
	InitSessionStore(dir)

	// Create a session and manually expire it
	store := GetSessionStore()
	token := generateSessionToken()
	store.CreateSession(token, 24*time.Hour, "test-agent", "127.0.0.1")

	// Manually expire the session by modifying the store
	store.mu.Lock()
	hash := sessionHash(token)
	if session, exists := store.sessions[hash]; exists {
		session.ExpiresAt = time.Now().Add(-1 * time.Hour) // Set to past
		store.sessions[hash] = session
	}
	store.mu.Unlock()

	result := ValidateSession(token)
	if result {
		t.Error("ValidateSession should return false for expired token")
	}
}

// CheckProxyAuth tests

func TestCheckProxyAuth_NotConfigured(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret: "", // Not configured
	}
	req := httptest.NewRequest("GET", "/", nil)

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if valid {
		t.Error("CheckProxyAuth should return false when not configured")
	}
	if username != "" {
		t.Errorf("username should be empty, got %q", username)
	}
	if isAdmin {
		t.Error("isAdmin should be false when not authenticated")
	}
}

func TestCheckProxyAuth_InvalidSecret(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret: "correct-secret",
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "wrong-secret")

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if valid {
		t.Error("CheckProxyAuth should return false for invalid secret")
	}
	if username != "" {
		t.Errorf("username should be empty, got %q", username)
	}
	if isAdmin {
		t.Error("isAdmin should be false when not authenticated")
	}
}

func TestCheckProxyAuth_MissingSecret(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret: "correct-secret",
	}
	req := httptest.NewRequest("GET", "/", nil)
	// No X-Proxy-Secret header

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if valid {
		t.Error("CheckProxyAuth should return false when secret header is missing")
	}
	if username != "" {
		t.Errorf("username should be empty, got %q", username)
	}
	if isAdmin {
		t.Error("isAdmin should be false when not authenticated")
	}
}

func TestCheckProxyAuth_ValidSecretNoUserHeader(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret: "correct-secret",
		// No ProxyAuthUserHeader configured
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "correct-secret")

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if !valid {
		t.Error("CheckProxyAuth should return true for valid secret")
	}
	if username != "" {
		t.Errorf("username should be empty when user header not configured, got %q", username)
	}
	if !isAdmin {
		t.Error("isAdmin should be true by default when no role checking")
	}
}

func TestCheckProxyAuth_MissingUserHeader(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "correct-secret",
		ProxyAuthUserHeader: "X-Remote-User",
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "correct-secret")
	// Missing X-Remote-User header

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if valid {
		t.Error("CheckProxyAuth should return false when user header is missing")
	}
	if username != "" {
		t.Errorf("username should be empty, got %q", username)
	}
	if isAdmin {
		t.Error("isAdmin should be false when not authenticated")
	}
}

func TestCheckProxyAuth_ValidWithUsername(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "correct-secret",
		ProxyAuthUserHeader: "X-Remote-User",
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "correct-secret")
	req.Header.Set("X-Remote-User", "testuser")

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if !valid {
		t.Error("CheckProxyAuth should return true for valid auth")
	}
	if username != "testuser" {
		t.Errorf("username should be 'testuser', got %q", username)
	}
	if !isAdmin {
		t.Error("isAdmin should be true by default when no role checking")
	}
}

func TestCheckProxyAuth_RoleCheckingEmptyRolesHeader(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "correct-secret",
		ProxyAuthUserHeader: "X-Remote-User",
		ProxyAuthRoleHeader: "X-Remote-Roles",
		ProxyAuthAdminRole:  "admin",
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "correct-secret")
	req.Header.Set("X-Remote-User", "testuser")
	// No X-Remote-Roles header

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if !valid {
		t.Error("CheckProxyAuth should return true for valid auth")
	}
	if username != "testuser" {
		t.Errorf("username should be 'testuser', got %q", username)
	}
	// When role header is empty, isAdmin stays true (default)
	if !isAdmin {
		t.Error("isAdmin should be true when roles header is empty")
	}
}

func TestCheckProxyAuth_RoleCheckingWithAdminRole(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "correct-secret",
		ProxyAuthUserHeader: "X-Remote-User",
		ProxyAuthRoleHeader: "X-Remote-Roles",
		ProxyAuthAdminRole:  "admin",
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "correct-secret")
	req.Header.Set("X-Remote-User", "adminuser")
	req.Header.Set("X-Remote-Roles", "user|admin|viewer")

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if !valid {
		t.Error("CheckProxyAuth should return true for valid auth")
	}
	if username != "adminuser" {
		t.Errorf("username should be 'adminuser', got %q", username)
	}
	if !isAdmin {
		t.Error("isAdmin should be true when user has admin role")
	}
}

func TestCheckProxyAuth_RoleCheckingWithoutAdminRole(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "correct-secret",
		ProxyAuthUserHeader: "X-Remote-User",
		ProxyAuthRoleHeader: "X-Remote-Roles",
		ProxyAuthAdminRole:  "admin",
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "correct-secret")
	req.Header.Set("X-Remote-User", "regularuser")
	req.Header.Set("X-Remote-Roles", "user|viewer")

	valid, username, isAdmin := CheckProxyAuth(cfg, req)
	if !valid {
		t.Error("CheckProxyAuth should return true for valid auth")
	}
	if username != "regularuser" {
		t.Errorf("username should be 'regularuser', got %q", username)
	}
	if isAdmin {
		t.Error("isAdmin should be false when user lacks admin role")
	}
}

func TestCheckProxyAuth_CustomRoleSeparator(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:        "correct-secret",
		ProxyAuthUserHeader:    "X-Remote-User",
		ProxyAuthRoleHeader:    "X-Remote-Roles",
		ProxyAuthAdminRole:     "administrator",
		ProxyAuthRoleSeparator: ",",
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "correct-secret")
	req.Header.Set("X-Remote-User", "adminuser")
	req.Header.Set("X-Remote-Roles", "user,administrator,viewer")

	valid, _, isAdmin := CheckProxyAuth(cfg, req)
	if !valid {
		t.Error("CheckProxyAuth should return true for valid auth")
	}
	if !isAdmin {
		t.Error("isAdmin should be true with custom separator")
	}
}

func TestCheckProxyAuth_RoleWithWhitespace(t *testing.T) {
	cfg := &config.Config{
		ProxyAuthSecret:     "correct-secret",
		ProxyAuthUserHeader: "X-Remote-User",
		ProxyAuthRoleHeader: "X-Remote-Roles",
		ProxyAuthAdminRole:  "admin",
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Proxy-Secret", "correct-secret")
	req.Header.Set("X-Remote-User", "adminuser")
	req.Header.Set("X-Remote-Roles", "user| admin |viewer") // Whitespace around admin

	valid, _, isAdmin := CheckProxyAuth(cfg, req)
	if !valid {
		t.Error("CheckProxyAuth should return true for valid auth")
	}
	if !isAdmin {
		t.Error("isAdmin should be true when role matches after trimming whitespace")
	}
}
