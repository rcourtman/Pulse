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

func TestHandleAddNode_PBSTurnkeyTokenCreationUsesCanonicalPulseURL(t *testing.T) {
	tempDir := t.TempDir()

	var createdTokenPath string
	pbsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/access/ticket":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{
					"ticket":              "pbs-ticket",
					"CSRFPreventionToken": "csrf-token",
				},
			})
		case "/api2/json/access/users":
			w.WriteHeader(http.StatusOK)
		case "/api2/json/access/acl":
			w.WriteHeader(http.StatusOK)
		case "/api2/json/access/users/pulse-monitor@pbs/token/pulse-public-example-com":
			createdTokenPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{
					"tokenid": "pulse-monitor@pbs!pulse-public-example-com",
					"value":   "pbs-secret",
				},
			})
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer pbsServer.Close()

	cfg := &config.Config{
		DataPath:     tempDir,
		ConfigPath:   tempDir,
		PublicURL:    "https://public.example.com",
		FrontendPort: 7655,
	}
	handler := newTestConfigHandlers(t, cfg)

	body, _ := json.Marshal(map[string]any{
		"name":     "test-pbs",
		"type":     "pbs",
		"host":     pbsServer.URL,
		"user":     "root@pam",
		"password": "secret",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes", bytes.NewBuffer(body))
	req.Host = "127.0.0.1:7655"
	rec := httptest.NewRecorder()

	handler.HandleAddNode(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if createdTokenPath != "/api2/json/access/users/pulse-monitor@pbs/token/pulse-public-example-com" {
		t.Fatalf("created token path = %q, want canonical public-url token scope", createdTokenPath)
	}
	if len(cfg.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance, got %d", len(cfg.PBSInstances))
	}
	if got := cfg.PBSInstances[0].TokenName; got != "pulse-monitor@pbs!pulse-public-example-com" {
		t.Fatalf("TokenName = %q, want canonical public-url token identity", got)
	}
	if got := cfg.PBSInstances[0].TokenValue; got != "pbs-secret" {
		t.Fatalf("TokenValue = %q, want returned token secret", got)
	}
	if got := cfg.PBSInstances[0].Password; got != "" {
		t.Fatalf("Password = %q, want cleared after turnkey token creation", got)
	}
}

func TestHandleAddNode_BlocksNewCountedSystemAtLimit(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)

	setMaxMonitoredSystemsLicenseForTests(t, 1)

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
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestRecords(unifiedresources.SourceAgent, []unifiedresources.IngestRecord{
		{
			SourceID: "host-1",
			Resource: unifiedresources.Resource{
				ID:     "host-1",
				Type:   unifiedresources.ResourceTypeAgent,
				Name:   "existing",
				Status: unifiedresources.StatusOnline,
				Agent: &unifiedresources.AgentData{
					AgentID:   "agent-1",
					Hostname:  "existing",
					MachineID: "machine-1",
				},
				Identity: unifiedresources.ResourceIdentity{
					MachineID: "machine-1",
					Hostnames: []string{"existing"},
				},
			},
		},
	})
	monitor := &monitoring.Monitor{}
	monitor.SetResourceStore(unifiedresources.NewMonitorAdapter(registry))
	handler.defaultMonitor = monitor

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

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 once monitored-system cap is full, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleAddNodeConsolidatesStandaloneOverlapIntoClusterInMemory(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		DataPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "homelab",
				ClusterName: "cluster-A",
				IsCluster:   true,
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "minipc", Host: "https://10.0.0.5:8006"},
				},
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	body, _ := json.Marshal(map[string]any{
		"name":       "test-minipc",
		"type":       "pve",
		"host":       "10.0.0.5",
		"tokenName":  "pulse@pve!token",
		"tokenValue": "secret",
		"verifySSL":  true,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()
	handler.HandleAddNode(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after consolidation, got %d", len(cfg.PVEInstances))
	}
	cluster := cfg.PVEInstances[0]
	if got := cluster.TokenName; got != "pulse@pve!token" {
		t.Fatalf("TokenName = %q, want pulse@pve!token", got)
	}
	if got := cluster.TokenValue; got != "secret" {
		t.Fatalf("TokenValue = %q, want secret", got)
	}
	if !cluster.VerifySSL {
		t.Fatalf("expected VerifySSL to be promoted onto the surviving cluster")
	}
}
