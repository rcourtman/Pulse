package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	dummyCfg.DataPath = tempDir

	handler := newTestConfigHandlers(t, dummyCfg)

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

func TestHandleUpdateNode_BlocksProjectedNetNewSystemAtLimit(t *testing.T) {
	setMaxMonitoredSystemsLicenseForTests(t, 1)

	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{{
			Name:       "tower",
			Host:       "https://tower.local:8006",
			TokenName:  "root@pam!pulse",
			TokenValue: "secret",
		}},
	}
	handler := newTestConfigHandlers(t, cfg)

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				ID:     "host-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "tower.local",
				Status: unifiedresources.StatusOnline,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-1",
					Hostname:  "tower.local",
					MachineID: "machine-1",
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"tower.local"},
				},
			},
		},
	})
	registry.IngestRecords(unifiedresources.SourceProxmox, []unifiedresources.IngestRecord{
		{
			SourceID: "pve-1",
			Resource: unifiedresources.Resource{
				ID:     "tower-resource",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "tower",
				Status: unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{
					Instance: "tower",
					NodeName: "tower",
					HostURL:  "https://tower.local:8006",
				},
			},
			Identity: unifiedresources.ResourceIdentity{
				MachineID: "machine-1",
				Hostnames: []string{"tower.local"},
			},
		},
	})
	setUnexportedField(t, handler.defaultMonitor, "resourceStore", monitoring.ResourceStoreInterface(unifiedresources.NewMonitorAdapter(registry)))

	body, _ := json.Marshal(map[string]any{
		"host": "https://backup.local:8006",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pve-0", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.HandleUpdateNode(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 when update would add a new monitored system, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := cfg.PVEInstances[0].Host; got != "https://tower.local:8006" {
		t.Fatalf("expected blocked update to preserve original host, got %q", got)
	}
}
