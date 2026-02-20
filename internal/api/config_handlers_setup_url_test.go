package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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
				token, ok := resp["setupToken"].(string)
				if !ok || token == "" {
					t.Errorf("expected valid setupToken, got %v", resp["setupToken"])
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
				if !strings.Contains(command, "curl -sSL "+quotedURL+" | PULSE_SETUP_TOKEN="+posixShellQuote(token)+" bash") {
					t.Errorf("expected shell-quoted command, got %s", command)
				}
				if !strings.Contains(commandWithoutEnv, "curl -sSL "+quotedURL+" | bash") {
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
			name:        "success_missing_fields_defaults",
			requestBody: map[string]interface{}{}, // Empty body might be valid if fields are optional?
			// Looking at code: it just decodes. If type/host are empty, it proceeds to generate token.
			// But type is used in setupCode struct.
			expectedStatus: http.StatusOK,
			verifyResponse: func(t *testing.T, resp map[string]interface{}, h *ConfigHandlers) {
				// Should still return a valid response even with empty fields
				if _, ok := resp["url"]; !ok {
					t.Errorf("expected url field in response")
				}
			},
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
