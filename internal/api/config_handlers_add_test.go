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

func TestHandleAddNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pulse-add-node-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Pre-populate with one node to test duplicate detection
	dummyCfg := &config.Config{
		PVEInstances: []config.PVEInstance{
			// Host must be normalized (https + port) for duplicate check to work
			{Name: "existing", Host: "https://10.0.0.1:8006"},
		},
	}
	dummyCfg.DataPath = tempDir

	// Create handler
	handler := newTestConfigHandlers(t, dummyCfg)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		verifyConfig   func(*testing.T, *config.Config)
	}{
		{
			name: "fail_missing_name",
			requestBody: map[string]interface{}{
				"type":     "pve",
				"host":     "10.0.0.2",
				"user":     "root@pam",
				"password": "password",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_missing_type",
			requestBody: map[string]interface{}{
				"name":     "test-newnode",
				"host":     "10.0.0.2",
				"user":     "root@pam",
				"password": "password",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_missing_host",
			requestBody: map[string]interface{}{
				"name":     "test-newnode",
				"type":     "pve",
				"user":     "root@pam",
				"password": "password",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_invalid_ip",
			requestBody: map[string]interface{}{
				"name":     "test-invalidip",
				"type":     "pve",
				"host":     "999.999.999.999",
				"user":     "root@pam",
				"password": "password",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "fail_duplicate_host",
			requestBody: map[string]interface{}{
				"name":     "test-duplicate",
				"type":     "pve",
				"host":     "10.0.0.1", // Will normalize to https://10.0.0.1:8006 and match existing
				"user":     "root@pam",
				"password": "password",
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name: "fail_missing_auth",
			requestBody: map[string]interface{}{
				"name": "test-noauth",
				"type": "pve",
				"host": "10.0.0.2",
				// No user/pass
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "success_add_pve_password",
			requestBody: map[string]interface{}{
				"name":     "test-new-pve",
				"type":     "pve",
				"host":     "10.0.0.2",
				"user":     "root@pam",
				"password": "secret",
			},
			expectedStatus: http.StatusCreated,
			verifyConfig: func(t *testing.T, c *config.Config) {
				found := false
				for _, node := range c.PVEInstances {
					if node.Name == "test-new-pve" {
						found = true
						if node.Password != "secret" {
							t.Errorf("expected password 'secret', got '%s'", node.Password)
						}
						// Verify host normalization
						if node.Host != "https://10.0.0.2:8006" {
							t.Errorf("expected normalized host, got '%s'", node.Host)
						}
						break
					}
				}
				if !found {
					t.Error("new PVE node not found in config")
				}
			},
		},
		{
			name: "success_add_pve_token",
			requestBody: map[string]interface{}{
				"name":       "test-token-pve",
				"type":       "pve",
				"host":       "10.0.0.3",
				"tokenName":  "root@pam!token",
				"tokenValue": "abcdef",
			},
			expectedStatus: http.StatusCreated,
			verifyConfig: func(t *testing.T, c *config.Config) {
				found := false
				for _, node := range c.PVEInstances {
					if node.Name == "test-token-pve" {
						found = true
						if node.TokenValue != "abcdef" {
							t.Errorf("expected token 'abcdef', got '%s'", node.TokenValue)
						}
						break
					}
				}
				if !found {
					t.Error("new PVE node (token) not found in config")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/api/config/nodes", bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.HandleAddNode(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", w.Code, tt.expectedStatus)
			}

			if tt.verifyConfig != nil {
				tt.verifyConfig(t, dummyCfg)
			}
		})
	}
}

// TestHandleAddNode_NoLimitForConfigRegistration verifies that PVE/PBS/PMG
// config registrations are never blocked by the agent limit (agents-only model).
func TestHandleAddNode_NoLimitForConfigRegistration(t *testing.T) {
	setMaxAgentsLicenseForTests(t, 1)

	tempDir, err := os.MkdirTemp("", "pulse-add-node-limit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DataPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{Name: "existing", Host: "https://10.0.0.1:8006"},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	body, _ := json.Marshal(map[string]any{
		"name":     "new-node",
		"type":     "pve",
		"host":     "10.0.0.2",
		"user":     "root@pam",
		"password": "secret",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	handler.HandleAddNode(rec, req)

	// Under agents-only model, config registrations are not limited.
	if rec.Code == http.StatusPaymentRequired {
		t.Fatalf("PVE config registration should not be blocked by agent limit, got 402")
	}
}
