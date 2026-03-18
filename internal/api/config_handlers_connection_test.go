package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleTestConnection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pulse-conn-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{DataPath: tempDir}
	handler := newTestConfigHandlers(t, cfg)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		verifyResponse func(*testing.T, map[string]interface{})
	}{
		{
			name:           "fail_invalid_json",
			requestBody:    "invalid-json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_missing_host",
			requestBody: map[string]string{
				"type": "pve",
				// no host
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_invalid_type",
			requestBody: map[string]string{
				"type": "unknown",
				"host": "10.0.0.1",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_missing_auth",
			requestBody: map[string]string{
				"type": "pve",
				"host": "10.0.0.1",
				// no user/pass or token
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_invalid_host_format",
			requestBody: map[string]string{
				"type":     "pve",
				"host":     "://invalid-url",
				"user":     "root@pam",
				"password": "password",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_connection_creation",
			requestBody: map[string]string{
				"type":     "pve",
				"host":     "127.0.0.1:9999", // Unreachable port
				"user":     "root@pam",
				"password": "password",
			},
			// Expecting 400 Bad Request because sanitizeErrorMessage wraps it
			expectedStatus: http.StatusBadRequest,
		},
		{
			// Token parsing logic test - should fail authentication if token format is invalid
			// but here we just check if it accepts valid auth params struct.
			// Actual successful connection is hard to test without mocking proxmox client.
			// We can verify "connection refused" or "timeout" which confirms it tried.
			name: "fail_connection_refused",
			requestBody: map[string]string{
				"type":     "pve",
				"host":     "127.0.0.1:1", // Port 1 likely closed
				"user":     "root@pam",
				"password": "password",
			},
			expectedStatus: http.StatusBadRequest,
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

			req := httptest.NewRequest("POST", "/api/config/connection/test", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.HandleTestConnection(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v \nBody: %s", w.Code, w.Body.String())
			}

			if tt.verifyResponse != nil {
				var response map[string]interface{}
				_ = json.NewDecoder(w.Body).Decode(&response)
				tt.verifyResponse(t, response)
			}
		})
	}
}
