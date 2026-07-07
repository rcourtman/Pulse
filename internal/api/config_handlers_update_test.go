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
		{
			// The unified connections aggregator emits IDs of the form
			// "pve:<name>"; the update endpoint accepts this form and resolves
			// by Name.
			name:   "success_semantic_id_pve_by_name",
			nodeID: "pve:test-renamed-pve",
			requestBody: map[string]interface{}{
				"name": "renamed-via-semantic-id",
			},
			expectedStatus: http.StatusOK,
			verifyConfig: func(t *testing.T, c *config.Config) {
				if c.PVEInstances[0].Name != "renamed-via-semantic-id" {
					t.Errorf("semantic-id update didn't take effect: got %q", c.PVEInstances[0].Name)
				}
			},
		},
		{
			// Previous case just renamed to "renamed-via-semantic-id", so
			// resolving by an outdated name should 404, not 400.
			name:   "fail_semantic_id_unknown_name",
			nodeID: "pve:test-renamed-pve",
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

func TestHandleUpdateNode_AllowsProjectedNetNewSystemWithoutPaidLimit(t *testing.T) {
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

	if rec.Code != http.StatusOK {
		t.Fatalf("expected update to remain allowed without monitored-system paid limits, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := cfg.PVEInstances[0].Host; got != "https://backup.local:8006" {
		t.Fatalf("expected update to apply new host without paid limit enforcement, got %q", got)
	}
}

func TestHandleUpdateNode_PreservesPVESecretsAndConnectionFieldsWhenOmitted(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{{
			Name:        "cluster",
			Host:        "https://pve.local:8006",
			GuestURL:    "https://guest.pve.local",
			Fingerprint: "AA:BB:CC",
			TokenName:   "pulse-monitor@pve!pulse-pve-local",
			TokenValue:  "saved-secret",
			VerifySSL:   true,
		}},
	}
	handler := newTestConfigHandlers(t, cfg)

	body, _ := json.Marshal(map[string]any{
		"name":       "cluster-renamed",
		"tokenName":  "pulse-monitor@pve!pulse-pve-local",
		"tokenValue": "********",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pve-0", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.HandleUpdateNode(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected update to succeed, got %d: %s", rec.Code, rec.Body.String())
	}
	node := cfg.PVEInstances[0]
	if node.TokenValue != "saved-secret" {
		t.Fatalf("token value = %q, want preserved saved secret", node.TokenValue)
	}
	if node.GuestURL != "https://guest.pve.local" {
		t.Fatalf("guestURL = %q, want preserved guest URL", node.GuestURL)
	}
	if node.Fingerprint != "AA:BB:CC" {
		t.Fatalf("fingerprint = %q, want preserved fingerprint", node.Fingerprint)
	}
}

func TestHandleUpdateNode_RejectsTokenNameChangeWithoutNewSecret(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{{
			Name:       "cluster",
			Host:       "https://pve.local:8006",
			TokenName:  "pulse-monitor@pve!old",
			TokenValue: "saved-secret",
		}},
	}
	handler := newTestConfigHandlers(t, cfg)

	body, _ := json.Marshal(map[string]any{
		"tokenName": "pulse-monitor@pve!new",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pve-0", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.HandleUpdateNode(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected token-name-only change to fail, got %d: %s", rec.Code, rec.Body.String())
	}
	node := cfg.PVEInstances[0]
	if node.TokenName != "pulse-monitor@pve!old" || node.TokenValue != "saved-secret" {
		t.Fatalf("token auth changed after rejected update: %+v", node)
	}
}

func TestHandleUpdateNode_RejectsTokenNameWithoutPreservedSecret(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{{
			Name:     "cluster",
			Host:     "https://pve.local:8006",
			User:     "root@pam",
			Password: "saved-password",
		}},
	}
	handler := newTestConfigHandlers(t, cfg)

	body, _ := json.Marshal(map[string]any{
		"tokenName": "pulse-monitor@pve!new",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pve-0", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.HandleUpdateNode(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected token auth without a secret to fail, got %d: %s", rec.Code, rec.Body.String())
	}
	node := cfg.PVEInstances[0]
	if node.Password != "saved-password" || node.TokenName != "" || node.TokenValue != "" {
		t.Fatalf("password auth changed after rejected update: %+v", node)
	}
}

func TestHandleUpdateNode_UserOnlyEditDoesNotClearProxmoxTokenAuth(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PBSInstances: []config.PBSInstance{{
			Name:           "backup",
			Host:           "https://pbs.local:8007",
			User:           "",
			Password:       "",
			TokenName:      "pulse-monitor@pbs!pulse-pbs-local",
			TokenValue:     "pbs-secret",
			MonitorBackups: false,
		}},
		PMGInstances: []config.PMGInstance{{
			Name:             "mail",
			Host:             "https://pmg.local:8006",
			User:             "",
			Password:         "",
			TokenName:        "pulse-monitor@pmg!pulse-pmg-local",
			TokenValue:       "pmg-secret",
			MonitorMailStats: false,
		}},
	}
	handler := newTestConfigHandlers(t, cfg)

	for _, tc := range []struct {
		name   string
		nodeID string
	}{
		{name: "pbs", nodeID: "pbs-0"},
		{name: "pmg", nodeID: "pmg-0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"user": "someone",
			})
			req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/"+tc.nodeID, bytes.NewBuffer(body))
			rec := httptest.NewRecorder()

			handler.HandleUpdateNode(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected update to succeed, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}

	pbs := cfg.PBSInstances[0]
	if pbs.TokenName != "pulse-monitor@pbs!pulse-pbs-local" || pbs.TokenValue != "pbs-secret" || pbs.User != "" || pbs.Password != "" {
		t.Fatalf("PBS token auth changed after user-only edit: %+v", pbs)
	}
	if pbs.MonitorBackups {
		t.Fatalf("PBS monitor backups should preserve false when omitted")
	}

	pmg := cfg.PMGInstances[0]
	if pmg.TokenName != "pulse-monitor@pmg!pulse-pmg-local" || pmg.TokenValue != "pmg-secret" || pmg.User != "" || pmg.Password != "" {
		t.Fatalf("PMG token auth changed after user-only edit: %+v", pmg)
	}
	if pmg.MonitorMailStats {
		t.Fatalf("PMG monitor mail stats should preserve false when omitted")
	}
}
