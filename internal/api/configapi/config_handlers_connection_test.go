package configapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Re-adding a PBS through a different address must not rotate a token used by
// the existing connection, or silently persist the administrator password.
func TestHandleAddNodePBSTokenCollisionPreservesConfig(t *testing.T) {
	testAddNodePBSTokenCollision(t, false)
}

// The credential regression is also testable without a listening socket. Both
// modes run the real PBS setup/request decoder, add handler and persistence.
// This mode substitutes an already-authenticated in-process HTTP transport;
// it does not assert a TLS handshake or password authentication result.
func TestHandleAddNodePBSTokenCollisionInProcess(t *testing.T) {
	testAddNodePBSTokenCollision(t, true)
}

type pbsSetupRoundTrip func(*http.Request) (*http.Response, error)

func (f pbsSetupRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func logPBSSetupResponse(t *testing.T, tag string, rec *httptest.ResponseRecorder) {
	t.Helper()
	wire, err := json.Marshal(map[string]any{"status": rec.Code, "headers": rec.Result().Header, "body": rec.Body.String()})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("PBS_SETUP_%s_HTTP=%s", tag, base64.StdEncoding.EncodeToString(wire))
	t.Logf("PBS_SETUP_%s_WIRE=%s", tag, base64.StdEncoding.EncodeToString(rec.Body.Bytes()))
}

func testAddNodePBSTokenCollision(t *testing.T, inProcess bool) {
	t.Helper()
	const oldToken = "fixture-existing-secret"
	var revoked atomic.Bool
	var creates, deletes, reloads atomic.Int32
	provider := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api2/json/access/ticket":
			_, _ = w.Write([]byte(`{"data":{"ticket":"fixture-ticket","CSRFPreventionToken":"fixture-csrf"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api2/json/version":
			if revoked.Load() || r.Header.Get("Authorization") != "PBSAPIToken=pulse-monitor@pbs!pulse-pulse-example-test:"+oldToken {
				http.Error(w, "fixture reader lost access", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"version":"3.4","release":"1","repoid":"fixture"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api2/json/access/users":
			_, _ = w.Write([]byte(`{"data":null}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api2/json/access/acl":
			_, _ = w.Write([]byte(`{"data":null}`))
		case strings.Contains(r.URL.Path, "/token/") && r.Method == http.MethodDelete:
			deletes.Add(1)
			revoked.Store(true)
			_, _ = w.Write([]byte(`{"data":null}`))
		case strings.Contains(r.URL.Path, "/token/") && r.Method == http.MethodPost:
			creates.Add(1)
			if !revoked.Load() {
				http.Error(w, "API token already exists; fixture-provider-private-marker", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"tokenid":"pulse-monitor@pbs!pulse-pulse-example-test","value":"fixture-replacement-secret"}}`))
		default:
			t.Errorf("unexpected PBS request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected fixture request", http.StatusNotFound)
		}
	})
	host, fingerprint := "https://pbs-fixture.invalid:8007", ""
	var attachTransport func(*pbs.Client)
	if inProcess {
		attachTransport = func(client *pbs.Client) {
			setUnexportedField(t, client, "httpClient", &http.Client{Transport: pbsSetupRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "pbs-fixture.invalid:8007" {
					t.Fatalf("unexpected fixture destination: %s", r.URL.Host)
				}
				if r.Body != nil {
					defer r.Body.Close()
				}
				rec := httptest.NewRecorder()
				provider.ServeHTTP(rec, r)
				response := rec.Result()
				response.Request = r
				return response, nil
			})})
		}
	} else {
		server := httptest.NewTLSServer(provider)
		t.Cleanup(server.Close)
		host = server.URL
		fingerprint = fmt.Sprintf("%x", sha256.Sum256(server.Certificate().Raw))
		attachTransport = func(*pbs.Client) {}
	}

	cfg := &config.Config{DataPath: t.TempDir(), PublicURL: "https://pulse.example.test", PBSInstances: []config.PBSInstance{{
		Name: "existing", Host: "https://existing-pbs.example.test:8007", TokenName: "pulse-monitor@pbs!pulse-pulse-example-test", TokenValue: oldToken, VerifySSL: true,
	}}}
	t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
	// Keep first-use key migration inside this fixture, never a host key path.
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(t.TempDir(), ".encryption.key"))
	h := newTestConfigHandlers(t, cfg)
	h.reloadFunc = func() error { reloads.Add(1); return nil }
	if inProcess {
		h.pbsSetupClientFactory = func(clientConfig pbs.ClientConfig) (*pbs.Client, error) {
			if clientConfig.Host != host || clientConfig.User != "root@pam" || clientConfig.Password != "fixture-administrator-password" {
				t.Fatal("setup did not forward the submitted credentials/endpoint")
			}
			client, err := pbs.NewClient(pbs.ClientConfig{Host: host, TokenName: "root@pam!fixture-admin", TokenValue: "fixture-admin-secret"})
			if err == nil {
				attachTransport(client)
			}
			return client, err
		}
	}
	if err := h.defaultPersistence.SaveNodesConfig(nil, cfg.PBSInstances, nil); err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := pbs.NewClient(pbs.ClientConfig{Host: host, TokenName: cfg.PBSInstances[0].TokenName, TokenValue: oldToken,
		VerifySSL: true, Fingerprint: fingerprint})
	if err != nil {
		t.Fatal(err)
	}
	attachTransport(reader)
	checkReader := func() {
		t.Helper()
		if version, err := reader.GetVersion(context.Background()); err != nil || version == nil || version.Version != "3.4" {
			t.Errorf("existing saved-token reader did not survive setup: %v", err)
		}
	}
	checkReader()
	body, _ := json.Marshal(map[string]any{"type": "pbs", "name": "new-address", "host": host,
		"user": "root@pam", "password": "fixture-administrator-password", "verifySSL": true})
	rec := httptest.NewRecorder()
	h.HandleAddNode(rec, httptest.NewRequest(http.MethodPost, "/api/config/nodes", bytes.NewReader(body)))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "Use Manual Token Setup") {
		t.Fatalf("collision status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "fixture-") || strings.Contains(rec.Body.String(), "root@pam") {
		t.Fatal("collision response disclosed provider output or credentials")
	}
	after, err := json.Marshal(cfg)
	if err != nil || !bytes.Equal(before, after) || reloads.Load() != 0 {
		t.Fatal("collision saved administrator credentials or changed the running configuration")
	}
	saved, err := h.defaultPersistence.LoadNodesConfig()
	if err != nil || len(saved.PBSInstances) != 1 || saved.PBSInstances[0].TokenValue != oldToken || saved.PBSInstances[0].Password != "" {
		t.Fatal("collision changed persisted connection credentials")
	}
	checkReader()
	if revoked.Load() || creates.Load() != 1 || deletes.Load() != 0 {
		t.Fatalf("collision mutated existing token: creates=%d deletes=%d", creates.Load(), deletes.Load())
	}
	logPBSSetupResponse(t, "COLLISION", rec)

	// The suggested existing manual-token route remains usable, without an
	// extra provisioning attempt or retaining the administrator password.
	body, _ = json.Marshal(map[string]any{"type": "pbs", "name": "new-address", "host": host,
		"tokenName": cfg.PBSInstances[0].TokenName, "tokenValue": oldToken, "verifySSL": true})
	manual := httptest.NewRecorder()
	h.HandleAddNode(manual, httptest.NewRequest(http.MethodPost, "/api/config/nodes", bytes.NewReader(body)))
	if manual.Code != http.StatusCreated || len(cfg.PBSInstances) != 2 || reloads.Load() != 1 {
		t.Fatalf("manual saved-token setup failed: status=%d body=%s", manual.Code, manual.Body.String())
	}
	added := cfg.PBSInstances[1]
	if added.Password != "" || added.User != "" || added.TokenValue != oldToken || !added.VerifySSL || creates.Load() != 1 || deletes.Load() != 0 {
		t.Fatal("manual setup changed access, TLS, or administrator credential persistence")
	}
	saved, err = h.defaultPersistence.LoadNodesConfig()
	if err != nil || len(saved.PBSInstances) != 2 || saved.PBSInstances[1].TokenValue != oldToken || saved.PBSInstances[1].Password != "" || !saved.PBSInstances[1].VerifySSL {
		t.Fatal("manual setup did not persist the deliberate token/verification configuration")
	}
	checkReader()
	logPBSSetupResponse(t, "MANUAL", manual)
}

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
