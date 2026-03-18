package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestGetRateLimiterForEndpoint(t *testing.T) {
	// Ensure rate limiters are initialized
	InitializeRateLimiters()

	tests := []struct {
		name           string
		path           string
		method         string
		wantLimiterPtr **RateLimiter
		wantLimiterNm  string
	}{
		// Authentication endpoints
		{
			name:           "login endpoint POST",
			path:           "/api/login",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.AuthEndpoints,
			wantLimiterNm:  "AuthEndpoints",
		},
		{
			name:           "login endpoint GET",
			path:           "/api/login",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.AuthEndpoints,
			wantLimiterNm:  "AuthEndpoints",
		},
		{
			name:           "logout endpoint",
			path:           "/api/logout",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.AuthEndpoints,
			wantLimiterNm:  "AuthEndpoints",
		},
		{
			name:           "change password endpoint",
			path:           "/api/security/change-password",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.AuthEndpoints,
			wantLimiterNm:  "AuthEndpoints",
		},
		{
			name:           "generic auth endpoint",
			path:           "/api/auth/something",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.AuthEndpoints,
			wantLimiterNm:  "AuthEndpoints",
		},
		{
			name:           "login with uppercase path still matches (lowercased)",
			path:           "/API/LOGIN",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.AuthEndpoints,
			wantLimiterNm:  "AuthEndpoints",
		},

		// Recovery endpoints
		{
			name:           "recovery endpoint POST",
			path:           "/api/security/recovery",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.RecoveryEndpoints,
			wantLimiterNm:  "RecoveryEndpoints",
		},
		{
			name:           "recovery endpoint GET",
			path:           "/api/security/recovery/status",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.RecoveryEndpoints,
			wantLimiterNm:  "RecoveryEndpoints",
		},

		// Export/Import endpoints
		{
			name:           "config export endpoint",
			path:           "/api/config/export",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.ExportEndpoints,
			wantLimiterNm:  "ExportEndpoints",
		},
		{
			name:           "config import endpoint",
			path:           "/api/config/import",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.ExportEndpoints,
			wantLimiterNm:  "ExportEndpoints",
		},

		// Configuration endpoints (write operations)
		{
			name:           "config nodes POST",
			path:           "/api/config/nodes",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.ConfigEndpoints,
			wantLimiterNm:  "ConfigEndpoints",
		},
		{
			name:           "config nodes PUT",
			path:           "/api/config/nodes/123",
			method:         http.MethodPut,
			wantLimiterPtr: &globalRateLimitConfig.ConfigEndpoints,
			wantLimiterNm:  "ConfigEndpoints",
		},
		{
			name:           "config nodes DELETE",
			path:           "/api/config/nodes/123",
			method:         http.MethodDelete,
			wantLimiterPtr: &globalRateLimitConfig.ConfigEndpoints,
			wantLimiterNm:  "ConfigEndpoints",
		},
		{
			name:           "config system POST",
			path:           "/api/config/system",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.ConfigEndpoints,
			wantLimiterNm:  "ConfigEndpoints",
		},
		{
			name:           "config webhooks POST",
			path:           "/api/config/webhooks",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.ConfigEndpoints,
			wantLimiterNm:  "ConfigEndpoints",
		},
		{
			name:           "config alerts POST",
			path:           "/api/config/alerts",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.ConfigEndpoints,
			wantLimiterNm:  "ConfigEndpoints",
		},

		// Configuration endpoints (read operations - higher limits)
		{
			name:           "config nodes GET",
			path:           "/api/config/nodes",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.PublicEndpoints,
			wantLimiterNm:  "PublicEndpoints",
		},
		{
			name:           "config system GET",
			path:           "/api/config/system",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.PublicEndpoints,
			wantLimiterNm:  "PublicEndpoints",
		},
		{
			name:           "discover GET",
			path:           "/api/discover",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.PublicEndpoints,
			wantLimiterNm:  "PublicEndpoints",
		},
		{
			name:           "security status GET",
			path:           "/api/security/status",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.PublicEndpoints,
			wantLimiterNm:  "PublicEndpoints",
		},

		// Update endpoints
		{
			name:           "updates check GET",
			path:           "/api/updates",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.UpdateEndpoints,
			wantLimiterNm:  "UpdateEndpoints",
		},
		{
			name:           "updates status GET",
			path:           "/api/updates/status",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.UpdateEndpoints,
			wantLimiterNm:  "UpdateEndpoints",
		},

		// WebSocket endpoints
		{
			name:           "ws endpoint",
			path:           "/ws",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.WebSocketEndpoints,
			wantLimiterNm:  "WebSocketEndpoints",
		},
		{
			name:           "ws subpath",
			path:           "/ws/events",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.WebSocketEndpoints,
			wantLimiterNm:  "WebSocketEndpoints",
		},

		// Public endpoints
		{
			name:           "health endpoint",
			path:           "/api/health",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.PublicEndpoints,
			wantLimiterNm:  "PublicEndpoints",
		},
		{
			name:           "version endpoint",
			path:           "/api/version",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.PublicEndpoints,
			wantLimiterNm:  "PublicEndpoints",
		},
		{
			name:           "validate bootstrap token",
			path:           "/api/security/validate-bootstrap-token",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.AuthEndpoints,
			wantLimiterNm:  "AuthEndpoints",
		},
		{
			name:           "metrics endpoint",
			path:           "/metrics",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.PublicEndpoints,
			wantLimiterNm:  "PublicEndpoints",
		},

		// General API (default)
		{
			name:           "guests endpoint",
			path:           "/api/guests",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.GeneralAPI,
			wantLimiterNm:  "GeneralAPI",
		},
		{
			name:           "nodes endpoint",
			path:           "/api/nodes",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.GeneralAPI,
			wantLimiterNm:  "GeneralAPI",
		},
		{
			name:           "unknown api endpoint",
			path:           "/api/unknown/endpoint",
			method:         http.MethodGet,
			wantLimiterPtr: &globalRateLimitConfig.GeneralAPI,
			wantLimiterNm:  "GeneralAPI",
		},
		{
			name:           "containers endpoint POST",
			path:           "/api/containers/start",
			method:         http.MethodPost,
			wantLimiterPtr: &globalRateLimitConfig.GeneralAPI,
			wantLimiterNm:  "GeneralAPI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRateLimiterForEndpoint(tt.path, tt.method)
			want := *tt.wantLimiterPtr
			if got != want {
				t.Errorf("GetRateLimiterForEndpoint(%q, %q) returned %s limiter, want %s",
					tt.path, tt.method, identifyLimiter(got), tt.wantLimiterNm)
			}
		})
	}
}

// identifyLimiter returns a string name for a rate limiter pointer for test output
func identifyLimiter(rl *RateLimiter) string {
	if rl == nil {
		return "nil"
	}
	if globalRateLimitConfig == nil {
		return "unknown (config nil)"
	}
	switch rl {
	case globalRateLimitConfig.AuthEndpoints:
		return "AuthEndpoints"
	case globalRateLimitConfig.ConfigEndpoints:
		return "ConfigEndpoints"
	case globalRateLimitConfig.ExportEndpoints:
		return "ExportEndpoints"
	case globalRateLimitConfig.RecoveryEndpoints:
		return "RecoveryEndpoints"
	case globalRateLimitConfig.UpdateEndpoints:
		return "UpdateEndpoints"
	case globalRateLimitConfig.WebSocketEndpoints:
		return "WebSocketEndpoints"
	case globalRateLimitConfig.GeneralAPI:
		return "GeneralAPI"
	case globalRateLimitConfig.PublicEndpoints:
		return "PublicEndpoints"
	default:
		return "unknown"
	}
}

func TestGetRateLimiterForEndpoint_PriorityOrder(t *testing.T) {
	// Test that more specific patterns match before general ones
	InitializeRateLimiters()

	tests := []struct {
		name          string
		path          string
		method        string
		wantLimiterNm string
		reason        string
	}{
		{
			name:          "recovery takes priority over security status",
			path:          "/api/security/recovery",
			method:        http.MethodPost,
			wantLimiterNm: "RecoveryEndpoints",
			reason:        "recovery paths should match before generic security paths",
		},
		{
			name:          "export takes priority over config read",
			path:          "/api/config/export",
			method:        http.MethodGet,
			wantLimiterNm: "ExportEndpoints",
			reason:        "export paths should match before generic config read paths",
		},
		{
			name:          "auth path takes priority over other paths",
			path:          "/api/auth/callback",
			method:        http.MethodGet,
			wantLimiterNm: "AuthEndpoints",
			reason:        "auth paths should be rate limited strictly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRateLimiterForEndpoint(tt.path, tt.method)
			gotName := identifyLimiter(got)
			if gotName != tt.wantLimiterNm {
				t.Errorf("GetRateLimiterForEndpoint(%q, %q) = %s, want %s; %s",
					tt.path, tt.method, gotName, tt.wantLimiterNm, tt.reason)
			}
		})
	}
}

func TestGetRateLimiterForEndpoint_InitializesIfNeeded(t *testing.T) {
	// Save and restore global state
	saved := globalRateLimitConfig
	globalRateLimitConfig = nil
	t.Cleanup(func() {
		globalRateLimitConfig = saved
	})

	// Call GetRateLimiterForEndpoint with nil config - should initialize
	got := GetRateLimiterForEndpoint("/api/login", http.MethodPost)
	if got == nil {
		t.Fatal("GetRateLimiterForEndpoint returned nil after initialization")
	}
	if globalRateLimitConfig == nil {
		t.Fatal("globalRateLimitConfig should have been initialized")
	}
}

func TestUniversalRateLimitMiddleware_HeaderFormat(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := UniversalRateLimitMiddleware(handler)

	limiter := GetRateLimiterForEndpoint("/api/login", http.MethodPost)
	testIP := "203.0.113.55"
	t.Cleanup(func() {
		ResetRateLimitForIP(testIP)
	})

	makeRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
		req.RemoteAddr = testIP + ":12345"
		return req
	}

	// Make requests up to the limit - all should succeed
	for i := 0; i < limiter.limit; i++ {
		rr := httptest.NewRecorder()
		middleware.ServeHTTP(rr, makeRequest())
		if rr.Code != http.StatusOK {
			t.Fatalf("expected OK while under limit, got %d on request %d", rr.Code, i+1)
		}
	}

	// Next request should trigger rate limit
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, makeRequest())
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate-limit status, got %d", rr.Code)
	}

	// Verify X-RateLimit-Limit header is a proper decimal string
	limitHeader := rr.Result().Header.Get("X-RateLimit-Limit")
	expected := strconv.Itoa(limiter.limit)
	if limitHeader != expected {
		t.Fatalf("expected X-RateLimit-Limit %q, got %q", expected, limitHeader)
	}

	// Verify the header parses as a valid integer (not a control character)
	if _, err := strconv.Atoi(limitHeader); err != nil {
		t.Fatalf("header %q should parse as decimal: %v", limitHeader, err)
	}
}

func TestUniversalRateLimitMiddleware_InitializesIfNeeded(t *testing.T) {
	// Save and restore global state
	saved := globalRateLimitConfig
	globalRateLimitConfig = nil
	t.Cleanup(func() {
		globalRateLimitConfig = saved
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create middleware with nil config - should initialize
	middleware := UniversalRateLimitMiddleware(handler)

	if globalRateLimitConfig == nil {
		t.Fatal("globalRateLimitConfig should have been initialized by middleware creation")
	}

	// Verify middleware works correctly after initialization
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestUniversalRateLimitMiddleware_StaticAssetBypass(t *testing.T) {
	InitializeRateLimiters()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})
	middleware := UniversalRateLimitMiddleware(handler)

	// Static asset paths (not /api or /ws prefixed) should bypass rate limiting
	tests := []struct {
		path string
	}{
		{"/index.html"},
		{"/static/js/app.js"},
		{"/favicon.ico"},
		{"/assets/style.css"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			handlerCalled = false
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			middleware.ServeHTTP(rr, req)

			if !handlerCalled {
				t.Errorf("handler should have been called for static asset path %s", tt.path)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("expected status 200 for static asset, got %d", rr.Code)
			}
		})
	}
}

func TestResetRateLimitForIP(t *testing.T) {
	t.Run("nil globalRateLimitConfig does not panic", func(t *testing.T) {
		// Save current config and restore after test
		savedConfig := globalRateLimitConfig
		globalRateLimitConfig = nil
		defer func() {
			globalRateLimitConfig = savedConfig
		}()

		// This should not panic
		ResetRateLimitForIP("192.168.1.1")
	})

	t.Run("resets rate limit for specific IP", func(t *testing.T) {
		InitializeRateLimiters()
		testIP := "10.0.0.50"
		limiter := globalRateLimitConfig.AuthEndpoints

		// Make some requests to add this IP to the limiter
		for i := 0; i < 3; i++ {
			limiter.Allow(testIP)
		}

		// Verify IP has attempts recorded
		limiter.mu.Lock()
		if _, exists := limiter.attempts[testIP]; !exists {
			limiter.mu.Unlock()
			t.Fatal("expected IP to have attempts recorded before reset")
		}
		limiter.mu.Unlock()

		// Reset this specific IP
		ResetRateLimitForIP(testIP)

		// Verify IP is removed from all limiters
		limiter.mu.Lock()
		if _, exists := limiter.attempts[testIP]; exists {
			limiter.mu.Unlock()
			t.Fatal("expected IP to be cleared after reset")
		}
		limiter.mu.Unlock()
	})
}
