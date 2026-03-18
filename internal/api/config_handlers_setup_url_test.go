package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleSetupScriptURL(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pulse-setup-url-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dummyCfg := &config.Config{
		FrontendPort: 8080,
		PublicURL:    "https://pulse.example.com",
	}
	dummyCfg.DataPath = tempDir
	handler := newTestConfigHandlers(t, dummyCfg)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		verifyResponse func(*testing.T, map[string]interface{}, *ConfigHandlers)
	}{
		{
			name: "success_valid_request",
			requestBody: map[string]interface{}{
				"type": "pve",
				"host": "pve1.local",
			},
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, resp map[string]interface{}, h *ConfigHandlers) {
				url, ok := resp["url"].(string)
				if !ok || url == "" {
					t.Errorf("expected valid url, got %v", resp["url"])
				}
				downloadURL, ok := resp["downloadURL"].(string)
				if !ok || downloadURL == "" {
					t.Errorf("expected valid downloadURL, got %v", resp["downloadURL"])
				}
				token, ok := resp["setupToken"].(string)
				if !ok || token == "" {
					t.Errorf("expected valid setupToken, got %v", resp["setupToken"])
				}
				tokenHint, ok := resp["tokenHint"].(string)
				if !ok || tokenHint == "" {
					t.Errorf("expected canonical tokenHint, got %v", resp["tokenHint"])
				}
				if tokenHint == token {
					t.Errorf("expected tokenHint to mask setup token, got %q", tokenHint)
				}
				if !strings.Contains(downloadURL, "setup_token=") {
					t.Errorf("expected downloadURL to embed setup token, got %q", downloadURL)
				}
				if !strings.Contains(downloadURL, token) {
					t.Errorf("expected downloadURL to contain setup token, got %q", downloadURL)
				}
				respType, ok := resp["type"].(string)
				if !ok || respType != "pve" {
					t.Errorf("expected canonical type, got %v", resp["type"])
				}
				respHost, ok := resp["host"].(string)
				if !ok || respHost != "https://pve1.local:8006" {
					t.Errorf("expected canonical normalized host, got %v", resp["host"])
				}
				scriptFileName, ok := resp["scriptFileName"].(string)
				if !ok || scriptFileName != "pulse-setup-pve.sh" {
					t.Errorf("expected canonical scriptFileName, got %v", resp["scriptFileName"])
				}
				command, ok := resp["command"].(string)
				if !ok || command == "" {
					t.Errorf("expected command in response, got %v", resp["command"])
				}
				commandWithoutEnv, ok := resp["commandWithoutEnv"].(string)
				if !ok || commandWithoutEnv == "" {
					t.Errorf("expected commandWithoutEnv in response, got %v", resp["commandWithoutEnv"])
				}

				// Verify URL construction uses public URL host
				if !strings.Contains(url, "pulse.example.com") {
					t.Errorf("expected URL to contain public host, got %s", url)
				}
				quotedURL := posixShellQuote(url)
				if !strings.Contains(command, "curl -fsSL "+quotedURL+" | ") ||
					!strings.Contains(command, `if [ "$(id -u)" -eq 0 ]; then PULSE_SETUP_TOKEN=`+posixShellQuote(token)+` bash`) ||
					!strings.Contains(command, `elif command -v sudo >/dev/null 2>&1; then sudo env PULSE_SETUP_TOKEN=`+posixShellQuote(token)+` bash`) {
					t.Errorf("expected shell-quoted command, got %s", command)
				}
				if !strings.Contains(commandWithoutEnv, "curl -fsSL "+quotedURL+" | ") ||
					!strings.Contains(commandWithoutEnv, `if [ "$(id -u)" -eq 0 ]; then bash`) ||
					!strings.Contains(commandWithoutEnv, `elif command -v sudo >/dev/null 2>&1; then sudo bash`) {
					t.Errorf("expected shell-quoted commandWithoutEnv, got %s", commandWithoutEnv)
				}
			},
		},
		{
			name:           "fail_invalid_json",
			requestBody:    "invalid-json", // Will fail JSON decoding
			expectedStatus: http.StatusBadRequest,
			verifyResponse: nil,
		},
		{
			name: "fail_unknown_field",
			requestBody: map[string]interface{}{
				"type":       "pve",
				"host":       "pve1.local",
				"setupToken": "unexpected",
			},
			expectedStatus: http.StatusBadRequest,
			verifyResponse: nil,
		},
		{
			name:           "fail_missing_fields",
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			verifyResponse: nil,
		},
		{
			name: "fail_invalid_type",
			requestBody: map[string]interface{}{
				"type": "pmg",
				"host": "pmg.local",
			},
			expectedStatus: http.StatusBadRequest,
			verifyResponse: nil,
		},
		{
			name: "fail_pbs_backup_perms",
			requestBody: map[string]interface{}{
				"type":        "pbs",
				"host":        "pbs.local",
				"backupPerms": true,
			},
			expectedStatus: http.StatusBadRequest,
			verifyResponse: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if s, ok := tt.requestBody.(string); ok && s == "invalid-json" {
				body = []byte(s)
			} else {
				body, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatal(err)
				}
			}

			req := httptest.NewRequest("POST", "/api/setup/url", bytes.NewBuffer(body))
			req.Host = "127.0.0.1:8080" // Simulate loopback request to trigger PublicURL usage logic
			w := httptest.NewRecorder()

			handler.HandleSetupScriptURL(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", w.Code, tt.expectedStatus)
			}

			if tt.verifyResponse != nil && w.Code == http.StatusOK {
				var response map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}
				tt.verifyResponse(t, response, handler)
			}
		})
	}
}

func TestHandleSetupScriptURL_RejectsTrailingJSON(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{DataPath: tempDir}
	handler := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/setup/url",
		bytes.NewBufferString(`{"type":"pve","host":"pve1.local"} {"extra":true}`),
	)
	w := httptest.NewRecorder()

	handler.HandleSetupScriptURL(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(w.Body.String()); got != "Invalid request" {
		t.Fatalf("body = %q, want canonical invalid request guidance", got)
	}
}

func TestHandleSetupScriptURL_MethodNotAllowed(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pulse-setup-url-test-method")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{DataPath: tempDir}
	handler := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest("GET", "/api/setup/url", nil)
	w := httptest.NewRecorder()

	handler.HandleSetupScriptURL(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected method not allowed, got %v", w.Code)
	}
}

func TestHandleSetupScriptURL_PreservesConfiguredPublicURLSchemeOnLoopback(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:     tempDir,
		FrontendPort: 8080,
		PublicURL:    "https://public.example.com/base",
	}
	handler := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/url", bytes.NewBufferString(`{"type":"pbs","host":"pbs1.local"}`))
	req.Host = "127.0.0.1:8080"
	w := httptest.NewRecorder()

	handler.HandleSetupScriptURL(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	urlValue, _ := response["url"].(string)
	if !strings.HasPrefix(urlValue, "https://public.example.com/base/api/setup-script") {
		t.Fatalf("url = %q, want https public URL preserved", urlValue)
	}

	command, _ := response["command"].(string)
	if !strings.Contains(command, "curl -fsSL "+posixShellQuote(urlValue)+" | ") || !strings.Contains(command, `sudo env PULSE_SETUP_TOKEN=`) {
		t.Fatalf("command = %q, want quoted setup-script URL", command)
	}
}

func TestBuildSetupScriptInstallArtifact_UsesCanonicalTransportContract(t *testing.T) {
	expiresAt := time.Unix(1900000000, 0).UTC().Unix()

	artifact := buildSetupScriptInstallArtifact(
		"https://pulse.example",
		"pve",
		"https://pve1.local:8006",
		"https://pulse.example",
		true,
		"setup-token-123",
		expiresAt,
	)

	if artifact.Type != "pve" {
		t.Fatalf("type = %q, want pve", artifact.Type)
	}
	if artifact.Host != "https://pve1.local:8006" {
		t.Fatalf("host = %q, want canonical host", artifact.Host)
	}
	if artifact.URL == "" || !strings.Contains(artifact.URL, "/api/setup-script?") {
		t.Fatalf("url = %q, want canonical setup-script url", artifact.URL)
	}
	if artifact.DownloadURL == "" || !strings.Contains(artifact.DownloadURL, "setup_token=setup-token-123") {
		t.Fatalf("downloadURL = %q, want setup token embedded", artifact.DownloadURL)
	}
	if artifact.ScriptFileName != "pulse-setup-pve.sh" {
		t.Fatalf("scriptFileName = %q, want canonical filename", artifact.ScriptFileName)
	}
	if artifact.Command != artifact.CommandWithEnv {
		t.Fatalf("command = %q, commandWithEnv = %q, want identical canonical env command", artifact.Command, artifact.CommandWithEnv)
	}
	if !strings.Contains(artifact.CommandWithEnv, "PULSE_SETUP_TOKEN='setup-token-123'") {
		t.Fatalf("commandWithEnv = %q, want setup token env transport", artifact.CommandWithEnv)
	}
	if strings.Contains(artifact.CommandWithoutEnv, "PULSE_SETUP_TOKEN=") {
		t.Fatalf("commandWithoutEnv = %q, want no setup token env transport", artifact.CommandWithoutEnv)
	}
	if artifact.Expires != expiresAt {
		t.Fatalf("expires = %d, want %d", artifact.Expires, expiresAt)
	}
	if artifact.SetupToken != "setup-token-123" {
		t.Fatalf("setupToken = %q, want original setup token", artifact.SetupToken)
	}
	if artifact.TokenHint != "set…123" {
		t.Fatalf("tokenHint = %q, want masked canonical token hint", artifact.TokenHint)
	}
}
