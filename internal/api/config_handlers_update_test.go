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

func TestHandleUpdateNode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pulse-update-node-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Setup initial config with a PVE node
	dummyCfg := &config.Config{
		PVEInstances: []config.PVEInstance{
			{
				Name:       "test-pve-update-target",
				Host:       "10.0.0.1",
				User:       "initial@pam",
				Password:   "initialpass",
				TokenName:  "",
				TokenValue: "",
			},
		},
	}

	handler := NewConfigHandlers(dummyCfg, nil, func() error { return nil }, nil, nil, func() {})
	handler.persistence = config.NewConfigPersistence(tempDir)

	tests := []struct {
		name           string
		nodeID         string
		requestBody    map[string]interface{}
		expectedStatus int
		verifyConfig   func(*testing.T, *config.Config)
	}{
		{
			name:   "success_update_name_only",
			nodeID: "pve-0",
			requestBody: map[string]interface{}{
				"name": "test-renamed-pve",
			},
			expectedStatus: http.StatusOK,
			verifyConfig: func(t *testing.T, c *config.Config) {
				if c.PVEInstances[0].Name != "test-renamed-pve" {
					t.Errorf("expected name 'test-renamed-pve', got '%s'", c.PVEInstances[0].Name)
				}
				// Verify other fields untouched
				if c.PVEInstances[0].Host != "10.0.0.1" {
					t.Errorf("host changed unexpectedly")
				}
			},
		},
		{
			name:   "success_switch_to_token",
			nodeID: "pve-0",
			requestBody: map[string]interface{}{
				"tokenName":  "root@pam!newtoken",
				"tokenValue": "newsecret",
			},
			expectedStatus: http.StatusOK,
			verifyConfig: func(t *testing.T, c *config.Config) {
				node := c.PVEInstances[0]
				if node.TokenName != "root@pam!newtoken" {
					t.Errorf("tokenName not updated")
				}
				if node.TokenValue != "newsecret" {
					t.Errorf("tokenValue not updated")
				}
				if node.Password != "" {
					t.Errorf("password not cleared when switching to token")
				}
			},
		},
		{
			name:   "success_switch_back_to_password",
			nodeID: "pve-0",
			requestBody: map[string]interface{}{
				"user":     "root@pam",
				"password": "newpassword",
			},
			expectedStatus: http.StatusOK,
			verifyConfig: func(t *testing.T, c *config.Config) {
				node := c.PVEInstances[0]
				if node.Password != "newpassword" {
					t.Errorf("password not updated")
				}
				if node.TokenName != "" || node.TokenValue != "" {
					t.Errorf("tokens not cleared when switching to password")
				}
			},
		},
		{
			name:   "fail_invalid_ip_update",
			nodeID: "pve-0",
			requestBody: map[string]interface{}{
				"host": "http:// invalid-url", // Space makes it invalid for url.Parse
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "fail_invalid_node_id",
			nodeID: "invalid-id",
			requestBody: map[string]interface{}{
				"name": "wont-work",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "fail_nonexistent_node",
			nodeID: "pve-99",
			requestBody: map[string]interface{}{
				"name": "wont-work",
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("PUT", "/api/config/nodes/"+tt.nodeID, bytes.NewBuffer(body))
			w := httptest.NewRecorder()

			handler.HandleUpdateNode(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", w.Code, tt.expectedStatus)
			}

			if tt.verifyConfig != nil {
				tt.verifyConfig(t, dummyCfg)
			}
		})
	}
}
