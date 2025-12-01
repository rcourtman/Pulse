package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestDemoModeMiddleware(t *testing.T) {
	// Create a simple handler that records if it was called
	handlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		demoMode       bool
		method         string
		path           string
		upgradeHeader  string
		wantCalled     bool
		wantStatus     int
		wantDemoHeader bool
	}{
		// Demo mode disabled - all requests pass through
		{"demo off GET", false, http.MethodGet, "/api/users", "", true, http.StatusOK, false},
		{"demo off POST", false, http.MethodPost, "/api/users", "", true, http.StatusOK, false},
		{"demo off PUT", false, http.MethodPut, "/api/users/1", "", true, http.StatusOK, false},
		{"demo off DELETE", false, http.MethodDelete, "/api/users/1", "", true, http.StatusOK, false},
		{"demo off PATCH", false, http.MethodPatch, "/api/users/1", "", true, http.StatusOK, false},

		// Demo mode enabled - read-only methods allowed
		{"demo on GET", true, http.MethodGet, "/api/users", "", true, http.StatusOK, true},
		{"demo on HEAD", true, http.MethodHead, "/api/users", "", true, http.StatusOK, true},
		{"demo on OPTIONS", true, http.MethodOptions, "/api/users", "", true, http.StatusOK, true},

		// Demo mode enabled - WebSocket upgrades allowed
		{"demo on websocket GET", true, http.MethodGet, "/api/ws", "websocket", true, http.StatusOK, true},
		{"demo on websocket case insensitive", true, http.MethodGet, "/api/ws", "WebSocket", true, http.StatusOK, true},
		{"demo on websocket uppercase", true, http.MethodGet, "/api/ws", "WEBSOCKET", true, http.StatusOK, true},
		// WebSocket upgrade with POST method (tests websocket branch after GET/HEAD/OPTIONS check)
		{"demo on websocket POST", true, http.MethodPost, "/api/ws", "websocket", true, http.StatusOK, true},

		// Demo mode enabled - auth endpoints allowed (POST)
		{"demo on login", true, http.MethodPost, "/api/login", "", true, http.StatusOK, true},
		{"demo on oidc login", true, http.MethodPost, "/api/oidc/login", "", true, http.StatusOK, true},
		{"demo on oidc callback", true, http.MethodPost, "/api/oidc/callback", "", true, http.StatusOK, true},
		{"demo on logout", true, http.MethodPost, "/api/logout", "", true, http.StatusOK, true},

		// Demo mode enabled - modification requests blocked
		{"demo on POST", true, http.MethodPost, "/api/users", "", false, http.StatusForbidden, true},
		{"demo on PUT", true, http.MethodPut, "/api/users/1", "", false, http.StatusForbidden, true},
		{"demo on DELETE", true, http.MethodDelete, "/api/users/1", "", false, http.StatusForbidden, true},
		{"demo on PATCH", true, http.MethodPatch, "/api/users/1", "", false, http.StatusForbidden, true},

		// Demo mode enabled - partial path matches should not be allowed
		{"demo on login prefix", true, http.MethodPost, "/api/login/extra", "", false, http.StatusForbidden, true},
		{"demo on oidc prefix", true, http.MethodPost, "/api/oidc/loginx", "", false, http.StatusForbidden, true},

		// Demo mode enabled - POST to other paths blocked
		{"demo on POST settings", true, http.MethodPost, "/api/settings", "", false, http.StatusForbidden, true},
		{"demo on POST config", true, http.MethodPost, "/api/config", "", false, http.StatusForbidden, true},
		{"demo on DELETE docker host", true, http.MethodDelete, "/api/docker-hosts/1", "", false, http.StatusForbidden, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled = false

			cfg := &config.Config{
				DemoMode: tt.demoMode,
			}

			middleware := DemoModeMiddleware(cfg, nextHandler)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.upgradeHeader != "" {
				req.Header.Set("Upgrade", tt.upgradeHeader)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			// Check if handler was called
			if handlerCalled != tt.wantCalled {
				t.Errorf("handler called = %v, want %v", handlerCalled, tt.wantCalled)
			}

			// Check status code
			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			// Check X-Demo-Mode header
			demoHeader := rr.Header().Get("X-Demo-Mode")
			hasDemoHeader := demoHeader == "true"
			if hasDemoHeader != tt.wantDemoHeader {
				t.Errorf("X-Demo-Mode header present = %v, want %v", hasDemoHeader, tt.wantDemoHeader)
			}
		})
	}
}

func TestDemoModeMiddleware_BlockedResponse(t *testing.T) {
	// Verify the error response format when a request is blocked
	cfg := &config.Config{
		DemoMode: true,
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for blocked request")
	})

	middleware := DemoModeMiddleware(cfg, nextHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	// Check Content-Type header
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	// Parse and verify response body
	var response map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}

	if response["error"] != "Demo mode enabled" {
		t.Errorf("error = %q, want %q", response["error"], "Demo mode enabled")
	}

	if response["message"] == "" {
		t.Error("message should not be empty")
	}
}
