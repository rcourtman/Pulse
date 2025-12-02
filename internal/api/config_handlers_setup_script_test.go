package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleSetupScriptRejectsUnsafeAuthToken(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com&auth_token=$(touch%20/tmp/pwned)", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request for unsafe auth token, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestHandleSetupScriptRejectsUnsafePulseURL(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com&pulse_url=http://example.com%5C%0Aecho%20oops", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request for unsafe pulse_url, got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestPVESetupScriptArgumentAlignment(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	// Use sentinel values to verify fmt.Sprintf argument alignment
	req := httptest.NewRequest(http.MethodGet,
		"/api/setup-script?type=pve&host=http://SENTINEL_HOST:8006&pulse_url=http://SENTINEL_URL:7656&auth_token=deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (%s)", rr.Code, rr.Body.String())
	}

	script := rr.Body.String()

	// Critical alignment checks to prevent fmt.Sprintf argument mismatch bugs
	// After refactor: script uses bash variables ($PULSE_URL, $TOKEN_NAME) instead of fmt.Sprintf substitutions
	tests := []struct {
		name     string
		contains string
		desc     string
	}{
		{
			name:     "repair_installer_url",
			contains: `INSTALLER_URL="$PULSE_URL/api/install/install-sensor-proxy.sh"`,
			desc:     "Repair block INSTALLER_URL should use $PULSE_URL bash variable",
		},
		{
			name:     "repair_ctid_pulse_server",
			contains: `--pulse-server $PULSE_URL`,
			desc:     "Repair --ctid --pulse-server should use $PULSE_URL bash variable",
		},
		{
			name:     "runtime_auth_token_ssh_config",
			contains: `-H "Authorization: Bearer $AUTH_TOKEN"`,
			desc:     "SSH config Authorization header should use runtime $AUTH_TOKEN variable",
		},
		{
			name:     "token_id_uses_tokenname",
			contains: `Token ID: $PULSE_TOKEN_ID`,
			desc:     "Token ID should use $PULSE_TOKEN_ID bash variable",
		},
		{
			name:     "bash_variables_defined",
			contains: `PULSE_URL="http://SENTINEL_URL:7656"`,
			desc:     "Bash variable PULSE_URL should be defined at top of script",
		},
		{
			name:     "token_name_variable_defined",
			contains: `TOKEN_NAME="pulse-SENTINEL_URL-`,
			desc:     "Bash variable TOKEN_NAME should be defined with correct format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !containsString(script, tt.contains) {
				t.Errorf("%s\nExpected to find: %s\nIn generated script (first 500 chars):\n%s",
					tt.desc, tt.contains, truncate(script, 500))
			}
		})
	}

	// Additional check: ensure authToken doesn't appear in --pulse-server flags
	if containsString(script, "--pulse-server deadbeef") {
		t.Error("BUG: authToken appearing in --pulse-server URL (argument misalignment)")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(findSubstring(s, substr) >= 0))
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func TestHandleSetupScript_MethodNotAllowed(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/setup-script?type=pve", nil)
		rr := httptest.NewRecorder()

		handlers.HandleSetupScript(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405 Method Not Allowed, got %d", method, rr.Code)
		}
	}
}

func TestHandleSetupScript_MissingTypeParameter(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	// No type parameter
	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?host=https://example.com", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for missing type, got %d", rr.Code)
	}
}

func TestHandleSetupScript_InvalidHostParameter(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handlers := newTestConfigHandlers(t, cfg)

	// Host with shell injection attempt
	req := httptest.NewRequest(http.MethodGet, "/api/setup-script?type=pve&host=https://example.com%5C%0Aecho%20pwned", nil)
	rr := httptest.NewRecorder()

	handlers.HandleSetupScript(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid host, got %d (%s)", rr.Code, rr.Body.String())
	}
}
