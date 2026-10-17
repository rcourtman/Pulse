package configapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
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

func TestHandleTestNodePBSIsADirectProbeWithoutChangingRuntimeHealth(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer server.Close()

	cfg := &config.Config{
		DataPath: t.TempDir(),
		PBSInstances: []config.PBSInstance{{
			Name:       "backup",
			Host:       server.URL,
			TokenName:  "root@pam!pulse",
			TokenValue: "rotated-secret",
		}},
	}
	handler := newTestConfigHandlers(t, cfg)
	beforeHealth := handler.defaultMonitor.SchedulerHealth()
	beforeStatuses := handler.defaultMonitor.GetConnectionStatuses()

	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes/pbs-0/test", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestNode(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("saved PBS test status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(authorization, "rotated-secret") {
		t.Fatalf("saved PBS test did not use the current stored token: %q", authorization)
	}
	afterHealth := handler.defaultMonitor.SchedulerHealth()
	if !reflect.DeepEqual(afterHealth.Instances, beforeHealth.Instances) ||
		!reflect.DeepEqual(afterHealth.Breakers, beforeHealth.Breakers) ||
		!reflect.DeepEqual(afterHealth.Staleness, beforeHealth.Staleness) {
		t.Fatalf("direct PBS test mutated canonical scheduler health:\nbefore=%+v\nafter=%+v", beforeHealth, afterHealth)
	}
	if !reflect.DeepEqual(handler.defaultMonitor.GetConnectionStatuses(), beforeStatuses) {
		t.Fatal("direct PBS test mutated the runtime connection projection")
	}
}

func TestHandleTestNodeUsesEditedTLSWithoutChangingSavedConfiguration(t *testing.T) {
	for _, platform := range []string{"pve", "pbs", "pmg"} {
		t.Run(platform, func(t *testing.T) {
			var authenticated atomic.Bool
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.Header.Get("Authorization"), "stored-secret") {
					authenticated.Store(true)
				}
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api2/json/access/ticket" {
					var login map[string]string
					if err := json.NewDecoder(r.Body).Decode(&login); err != nil {
						t.Error(err)
					}
					if login["password"] == "stored-secret" {
						authenticated.Store(true)
					}
					_, _ = w.Write([]byte(`{"data":{"ticket":"test-ticket","CSRFPreventionToken":"test-csrf"}}`))
					return
				}
				if platform == "pmg" {
					_, _ = w.Write([]byte(`{"data":{"version":"8.2","release":"1"}}`))
					return
				}
				_, _ = w.Write([]byte(`{"data":[]}`))
			}))
			defer server.Close()
			fingerprint := fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))
			wrongFingerprint := strings.Repeat("00", sha256.Size)

			for _, tc := range []struct {
				name        string
				savedVerify bool
				savedPin    string
				body        string
				wantStatus  int
			}{
				{"edited fingerprint", true, "", fmt.Sprintf(`{"verifySSL":true,"fingerprint":%q}`, fingerprint), http.StatusOK},
				{"edited verification and cleared fingerprint", true, wrongFingerprint, `{"verifySSL":false,"fingerprint":""}`, http.StatusOK},
				{"edited strict verification", false, "", `{"verifySSL":true,"fingerprint":""}`, http.StatusBadRequest},
				{"cleared saved fingerprint", true, fingerprint, `{"fingerprint":""}`, http.StatusBadRequest},
				{"omitted verification retains saved value", true, "", fmt.Sprintf(`{"fingerprint":%q}`, fingerprint), http.StatusOK},
				{"omitted fingerprint retains saved pin", false, wrongFingerprint, `{"verifySSL":false}`, http.StatusBadRequest},
				{"empty body retains saved TLS", true, fingerprint, "", http.StatusOK},
				{"empty object retains saved TLS", true, fingerprint, `{}`, http.StatusOK},
				{"invalid JSON rejected", false, "", `{`, http.StatusBadRequest},
				{"invalid TLS type rejected", false, "", `{"verifySSL":"false"}`, http.StatusBadRequest},
				{"trailing JSON rejected", false, "", `{} {}`, http.StatusBadRequest},
				{"unknown fields rejected", false, "", `{"password":"replacement"}`, http.StatusBadRequest},
				{"oversized body rejected", false, "", `{"fingerprint":"` + strings.Repeat("a", 32*1024) + `"}`, http.StatusBadRequest},
			} {
				t.Run(tc.name, func(t *testing.T) {
					authenticated.Store(false)
					cfg := &config.Config{DataPath: t.TempDir()}
					t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
					switch platform {
					case "pve":
						cfg.PVEInstances = []config.PVEInstance{{Name: "saved", Host: server.URL, TokenName: "root@pam!pulse", TokenValue: "stored-secret", VerifySSL: tc.savedVerify, Fingerprint: tc.savedPin}}
					case "pbs":
						cfg.PBSInstances = []config.PBSInstance{{Name: "saved", Host: server.URL, TokenName: "root@pam!pulse", TokenValue: "stored-secret", VerifySSL: tc.savedVerify, Fingerprint: tc.savedPin}}
					case "pmg":
						cfg.PMGInstances = []config.PMGInstance{{Name: "saved", Host: server.URL, User: "root@pam", Password: "stored-secret", VerifySSL: tc.savedVerify, Fingerprint: tc.savedPin}}
					}
					handler := newTestConfigHandlers(t, cfg)
					before, err := json.Marshal(cfg)
					if err != nil {
						t.Fatal(err)
					}
					beforeHealth := handler.defaultMonitor.SchedulerHealth()
					beforeStatuses := handler.defaultMonitor.GetConnectionStatuses()
					req := httptest.NewRequest(http.MethodPost, "/api/config/nodes/"+platform+"-0/test", strings.NewReader(tc.body))
					rec := httptest.NewRecorder()
					handler.HandleTestNode(rec, req)
					if rec.Code != tc.wantStatus {
						t.Fatalf("probe status = %d, want %d, body=%s", rec.Code, tc.wantStatus, rec.Body.String())
					}
					if tc.wantStatus == http.StatusOK && !authenticated.Load() {
						t.Fatal("probe did not use the saved credential")
					}
					after, err := json.Marshal(cfg)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(before, after) {
						t.Fatal("probe mutated the saved configuration")
					}
					afterHealth := handler.defaultMonitor.SchedulerHealth()
					if !reflect.DeepEqual(afterHealth.Instances, beforeHealth.Instances) ||
						!reflect.DeepEqual(afterHealth.Breakers, beforeHealth.Breakers) ||
						!reflect.DeepEqual(afterHealth.Staleness, beforeHealth.Staleness) ||
						!reflect.DeepEqual(handler.defaultMonitor.GetConnectionStatuses(), beforeStatuses) {
						t.Fatal("probe mutated runtime connection health")
					}
				})
			}
		})
	}
}

func TestNodeMutationEventsStayWithRequestOrganization(t *testing.T) {
	for _, orgID := range []string{"org-a", "default", ""} {
		t.Run("org="+orgID, func(t *testing.T) {
			hub := websocket.NewHub(nil)
			owner, other := tenantEventConnections(t, hub, orgID)
			h := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir(), PVEInstances: []config.PVEInstance{
				{Name: "node-a", Host: "https://192.0.2.10:8006", TokenValue: "synthetic"},
			}})
			h.wsHub = hub
			original := detectPVECluster
			t.Cleanup(func() { detectPVECluster = original })
			detectPVECluster = func(proxmox.ClientConfig, string, []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
				return true, "cluster-a", []config.ClusterEndpoint{{NodeName: "node-a", Host: "https://192.0.2.10:8006"}}
			}
			ctx := context.WithValue(context.Background(), apicontext.OrgIDContextKey, orgID)
			refresh := httptest.NewRequest(http.MethodPost, "/api/config/nodes/pve-0/refresh-cluster", nil).WithContext(ctx)
			rec := httptest.NewRecorder()
			h.HandleRefreshClusterNodes(rec, refresh)
			if rec.Code != http.StatusOK {
				t.Fatalf("refresh: %d %s", rec.Code, rec.Body.String())
			}
			remove := httptest.NewRequest(http.MethodDelete, "/api/config/nodes/pve-0", nil).WithContext(ctx)
			rec = httptest.NewRecorder()
			h.HandleDeleteNode(rec, remove)
			if rec.Code != http.StatusOK {
				t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
			}
			assertTenantEvents(t, owner, other, "nodes_updated", "node_deleted")
		})
	}
}
