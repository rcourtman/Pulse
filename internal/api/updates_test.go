package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
)

func TestUpdateHandlers_HandleCheckUpdates_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/updates/check", nil)
	rec := httptest.NewRecorder()

	handlers.HandleCheckUpdates(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestUpdateHandlers_HandleApplyUpdate_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodGet, "/api/updates/apply", nil)
	rec := httptest.NewRecorder()

	handlers.HandleApplyUpdate(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestUpdateHandlers_HandleApplyUpdate_InvalidJSONBody(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", strings.NewReader("invalid json"))
	rec := httptest.NewRecorder()

	handlers.HandleApplyUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdateHandlers_HandleApplyUpdate_MissingDownloadURL(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/updates/apply", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	handlers.HandleApplyUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdateHandlers_HandleUpdateStatus_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/updates/status", nil)
	rec := httptest.NewRecorder()

	handlers.HandleUpdateStatus(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestUpdateHandlers_HandleUpdateStream_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/updates/stream", nil)
	rec := httptest.NewRecorder()

	handlers.HandleUpdateStream(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestUpdateHandlers_HandleGetUpdatePlan_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/updates/plan", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetUpdatePlan(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestUpdateHandlers_HandleListUpdateHistory_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/updates/history", nil)
	rec := httptest.NewRecorder()

	handlers.HandleListUpdateHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestUpdateHandlers_HandleListUpdateHistory_NoHistory(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{
		history: nil,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/updates/history", nil)
	rec := httptest.NewRecorder()

	handlers.HandleListUpdateHistory(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestUpdateHandlers_HandleListUpdateHistory_Success(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	history, _ := updates.NewUpdateHistory(tmp)

	handlers := &UpdateHandlers{
		history: history,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/updates/history", nil)
	rec := httptest.NewRecorder()

	handlers.HandleListUpdateHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected content-type application/json, got %q", ct)
	}
}

func TestUpdateHandlers_HandleGetUpdateHistoryEntry_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/updates/history/123", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetUpdateHistoryEntry(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestUpdateHandlers_HandleGetUpdateHistoryEntry_NoHistory(t *testing.T) {
	t.Parallel()

	handlers := &UpdateHandlers{
		history: nil,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/updates/history/123", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetUpdateHistoryEntry(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestUpdateHandlers_HandleGetUpdateHistoryEntry_MissingID(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	history, _ := updates.NewUpdateHistory(tmp)

	handlers := &UpdateHandlers{
		history: history,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/updates/history/entry", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetUpdateHistoryEntry(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestUpdateHandlers_HandleGetUpdateHistoryEntry_NotFound(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	history, _ := updates.NewUpdateHistory(tmp)

	handlers := &UpdateHandlers{
		history: history,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/updates/history/entry?id=nonexistent", nil)
	rec := httptest.NewRecorder()

	handlers.HandleGetUpdateHistoryEntry(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string // X-Forwarded-For header
		xri        string // X-Real-IP header
		remoteAddr string // Request.RemoteAddr
		expectedIP string
	}{
		// X-Forwarded-For takes priority
		{
			name:       "XFF with valid IPv4",
			xff:        "192.168.1.100",
			xri:        "",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "XFF with valid IPv6",
			xff:        "2001:db8::1",
			xri:        "",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "XFF with IPv6 loopback",
			xff:        "::1",
			xri:        "",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "::1",
		},

		// X-Real-IP fallback when XFF not valid
		{
			name:       "XRI with valid IPv4",
			xff:        "",
			xri:        "172.16.0.50",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "172.16.0.50",
		},
		{
			name:       "XRI with valid IPv6",
			xff:        "",
			xri:        "fe80::1",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "fe80::1",
		},
		{
			name:       "XRI preferred when XFF invalid",
			xff:        "invalid-ip",
			xri:        "192.168.1.1",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.1",
		},

		// RemoteAddr fallback
		{
			name:       "RemoteAddr with port",
			xff:        "",
			xri:        "",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "10.0.0.1",
		},
		{
			name:       "RemoteAddr IPv6 with port",
			xff:        "",
			xri:        "",
			remoteAddr: "[::1]:12345",
			expectedIP: "::1",
		},
		{
			name:       "RemoteAddr without port",
			xff:        "",
			xri:        "",
			remoteAddr: "10.0.0.1",
			expectedIP: "10.0.0.1",
		},

		// Invalid headers fall through
		{
			name:       "XFF invalid falls to XRI",
			xff:        "not-an-ip",
			xri:        "192.168.1.1",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "Both headers invalid falls to RemoteAddr",
			xff:        "not-an-ip",
			xri:        "also-not-an-ip",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "10.0.0.1",
		},

		// Edge cases
		{
			name:       "Empty XFF ignored",
			xff:        "",
			xri:        "192.168.1.1",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "All empty uses RemoteAddr",
			xff:        "",
			xri:        "",
			remoteAddr: "127.0.0.1:8080",
			expectedIP: "127.0.0.1",
		},
		{
			name:       "Loopback IPv4",
			xff:        "127.0.0.1",
			xri:        "",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "127.0.0.1",
		},

		// Note: The current implementation has a bug with multiple IPs in XFF
		// It tries to parse the entire string as a single IP, which fails
		// This test documents current behavior, not ideal behavior
		{
			name:       "XFF with multiple IPs - current behavior",
			xff:        "192.168.1.100, 10.0.0.1",
			xri:        "172.16.0.1",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "172.16.0.1", // Falls through because "192.168.1.100, 10.0.0.1" is not a valid single IP
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header:     make(http.Header),
				RemoteAddr: tt.remoteAddr,
			}

			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			result := getClientIP(req)
			if result != tt.expectedIP {
				t.Errorf("getClientIP() = %q, want %q", result, tt.expectedIP)
			}
		})
	}
}

func TestGetClientIP_NilHeaders(t *testing.T) {
	// Test with a request that has nil headers (edge case)
	req := &http.Request{
		Header:     nil,
		RemoteAddr: "10.0.0.1:12345",
	}

	// This will panic if headers aren't handled correctly
	// The function should gracefully handle nil headers
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("getClientIP panicked with nil headers: %v", r)
		}
	}()

	// Note: With nil headers, Header.Get() will panic
	// This test documents that the function expects non-nil headers
	// If this test panics, it's documenting current behavior
	_ = getClientIP(req)
}

func TestGetClientIP_HeaderCaseSensitivity(t *testing.T) {
	// HTTP headers are case-insensitive per RFC 7230
	// http.Header.Get handles this automatically
	tests := []struct {
		name       string
		headerKey  string
		headerVal  string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "lowercase x-forwarded-for",
			headerKey:  "x-forwarded-for",
			headerVal:  "192.168.1.100",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "lowercase x-real-ip",
			headerKey:  "x-real-ip",
			headerVal:  "192.168.1.100",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "mixed case X-Forwarded-FOR",
			headerKey:  "X-Forwarded-FOR",
			headerVal:  "192.168.1.100",
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header:     make(http.Header),
				RemoteAddr: tt.remoteAddr,
			}
			req.Header.Set(tt.headerKey, tt.headerVal)

			result := getClientIP(req)
			if result != tt.expectedIP {
				t.Errorf("getClientIP() = %q, want %q", result, tt.expectedIP)
			}
		})
	}
}
