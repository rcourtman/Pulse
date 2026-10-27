package configapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"io"
	stdlog "log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

// The standard root pool is process-cached. Keep the synthetic CA experiment in
// a fresh copy of this test process so the normal full suite is order-independent.
func TestPVEEndpointTrustSavedPolling(t *testing.T) {
	if os.Getenv("PULSE_TLS_TEST_CHILD") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(executable, "-test.run=^TestPVEEndpointTrustSavedPolling$", "-test.v")
		cmd.Env = append(os.Environ(), "PULSE_TLS_TEST_CHILD=1")
		output, err := cmd.CombinedOutput()
		t.Log(string(output))
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "isolated polling CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSL_CERT_FILE", caFile)
	t.Setenv("SSL_CERT_DIR", t.TempDir())
	makeLeaf := func(serial int64) tls.Certificate {
		pub, leafKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, KeyUsage: x509.KeyUsageDigitalSignature}
		der, err := x509.CreateCertificate(rand.Reader, leaf, ca, pub, key)
		if err != nil {
			t.Fatal(err)
		}
		return tls.Certificate{Certificate: [][]byte{der, ca.Raw}, PrivateKey: leafKey}
	}
	pin := func(c tls.Certificate) string { return fmt.Sprintf("%x", sha256.Sum256(c.Certificate[0])) }
	old := [2]tls.Certificate{makeLeaf(2), makeLeaf(3)}
	renewed := [2]tls.Certificate{makeLeaf(4), makeLeaf(5)}
	var active [2]atomic.Pointer[tls.Certificate]
	var authenticated [2]atomic.Int32
	var servers [2]*httptest.Server
	for i := range servers {
		active[i].Store(&renewed[i])
		servers[i] = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api2/json/nodes" {
				t.Errorf("unexpected request path %s", r.URL.Path)
				http.NotFound(w, r)
				return
			}
			if r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!pulse=synthetic-only" {
				t.Error("saved credential changed")
				w.WriteHeader(401)
				return
			}
			authenticated[i].Add(1)
			fmt.Fprint(w, `{"data":[{"node":"alpha","status":"online"},{"node":"beta","status":"online"}]}`)
		}))
		servers[i].Config.ErrorLog = stdlog.New(io.Discard, "", 0)
		servers[i].TLS = &tls.Config{MinVersion: tls.VersionTLS12, GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			return &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{*active[i].Load()}}, nil
		}}
		servers[i].StartTLS()
		t.Cleanup(servers[i].Close)
	}
	// Admit the intended, hostname-matching leaf independently of pinning.
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	for i, s := range servers {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", strings.TrimPrefix(s.URL, "https://"), &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12})
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprintf("%x", sha256.Sum256(conn.ConnectionState().PeerCertificates[0].Raw))
		conn.Close()
		if got != pin(renewed[i]) {
			t.Fatal("wrong fixture leaf presented")
		}
	}
	for _, tc := range []struct {
		name, action string
		original     bool
		wantHealth   [2]bool
		wantTest     int
	}{
		{"unapproved-rotation", "", false, [2]bool{false, false}, 400},
		{"primary-edit-is-scoped", "primary-pin", false, [2]bool{true, false}, 200},
		{"primary-CA-clear-is-scoped", "primary-ca", false, [2]bool{true, false}, 200},
		{"explicit-both-pins", "both-pins", false, [2]bool{true, true}, 200},
		{"explicit-both-CA", "both-ca", false, [2]bool{true, true}, 200},
		{"member-CA-does-not-inherit-primary-pin", "member-ca", false, [2]bool{true, true}, 200},
		{"original-leaves-control", "", true, [2]bool{true, true}, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, s := range servers {
				if tc.original {
					active[i].Store(&old[i])
				} else {
					active[i].Store(&renewed[i])
				}
				s.CloseClientConnections()
				authenticated[i].Store(0)
			}
			inst := config.PVEInstance{Name: "fixture-cluster", Host: servers[0].URL, TokenName: "fixture@pve!pulse", TokenValue: "synthetic-only", VerifySSL: true, VerifySSLExplicit: true, IsCluster: true, ClusterName: "fixture-cluster", Fingerprint: pin(old[0]), ClusterEndpoints: []config.ClusterEndpoint{
				{NodeIdentity: "alpha-id", NodeName: "alpha", Host: servers[0].URL, Fingerprint: pin(old[0])},
				{NodeIdentity: "beta-id", NodeName: "beta", Host: servers[1].URL, Fingerprint: pin(old[1])},
			}}
			cfg := &config.Config{DataPath: t.TempDir(), PVEInstances: []config.PVEInstance{inst}}
			t.Setenv("PULSE_DATA_DIR", cfg.DataPath)
			persistence := config.NewConfigPersistence(cfg.DataPath)
			if err := persistence.SaveNodesConfig(cfg.PVEInstances, nil, nil); err != nil {
				t.Fatal(err)
			}
			reloads := 0
			h := &ConfigHandlers{defaultConfig: cfg, defaultPersistence: persistence, runtime: defaultRuntimeDependencies(), reloadFunc: func() error { reloads++; return nil }}
			initialNode := h.GetAllNodesForAPI(context.Background())[0]
			if tc.action != "" {
				primary := pin(renewed[0])
				if strings.Contains(tc.action, "ca") && tc.action != "member-ca" {
					primary = ""
				}
				update := map[string]any{"verifySSL": true, "fingerprint": primary}
				if strings.HasPrefix(tc.action, "both") || tc.action == "member-ca" {
					member := pin(renewed[1])
					if strings.Contains(tc.action, "ca") {
						member = ""
					}
					update["clusterEndpointTrustOverrides"] = []map[string]string{{"endpoint": servers[1].URL, "fingerprint": member}}
				}
				body, _ := json.Marshal(update)
				rec := httptest.NewRecorder()
				h.HandleUpdateNode(rec, httptest.NewRequest("PUT", "/api/config/nodes/pve-0", strings.NewReader(string(body))))
				if rec.Code != 200 || reloads != 1 {
					t.Fatalf("save=%d reloads=%d: %s", rec.Code, reloads, rec.Body.String())
				}
			}
			disk, err := persistence.LoadNodesConfig()
			if err != nil {
				t.Fatal(err)
			}
			cfg.PVEInstances = disk.PVEInstances
			saved := cfg.PVEInstances[0]
			if saved.TokenValue != inst.TokenValue || !saved.VerifySSL || !reflect.DeepEqual(saved.ClusterEndpoints, inst.ClusterEndpoints) {
				t.Fatal("trust edit changed credentials, verification or discovered member records")
			}
			before, _ := json.Marshal(saved)
			rec := httptest.NewRecorder()
			h.HandleTestNode(rec, httptest.NewRequest("POST", "/api/config/nodes/pve-0/test", strings.NewReader(`{}`)))
			if rec.Code != tc.wantTest {
				t.Fatalf("saved Test=%d want=%d: %s", rec.Code, tc.wantTest, rec.Body.String())
			}
			if rec.Code == 200 && !strings.Contains(rec.Body.String(), "Other endpoints and background collection were not tested") {
				t.Fatal("primary test claims cluster collection")
			}
			primaryProbeCode, primaryProbeBody := rec.Code, rec.Body.String()
			projection := pveClusterEndpointTLSResponses(saved, cfg.Discovery)
			if len(projection) != 2 || !projection[0].UsesPrimaryTLS || projection[1].UsesPrimaryTLS || projection[1].TLSEndpoint != servers[1].URL {
				t.Fatalf("incorrect effective endpoint projection: %+v", projection)
			}
			// Init/reconnect share this same exported production builder.
			endpoints, pins, conflicts := monitoring.BuildPVEClusterClientEndpoints(saved, cfg.Discovery)
			clientCfg := config.CreateProxmoxConfig(&saved)
			clientCfg.Timeout = time.Second
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			cc, err := proxmox.NewClusterClientWithContext(ctx, saved.Name, clientCfg, endpoints, pins, conflicts...)
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			nodes, pollErr := cc.GetNodes(ctx)
			cancel()
			health := cc.GetHealthStatus()
			if health[servers[0].URL] != tc.wantHealth[0] || health[servers[1].URL] != tc.wantHealth[1] {
				t.Fatalf("health=%v want=%v", health, tc.wantHealth)
			}
			if tc.wantHealth[0] || tc.wantHealth[1] {
				if pollErr != nil || len(nodes) != 2 {
					t.Fatalf("ordinary nodes poll: %v, %d", pollErr, len(nodes))
				}
			} else if pollErr == nil {
				t.Fatal("unapproved renewal accepted")
			}
			t.Logf("primary=%t member=%t verifySSL=%t", health[servers[0].URL], health[servers[1].URL], saved.VerifySSL)
			// A member probe uses the same scoped policy, saved credentials and no
			// persistent trust or health update. It never captures a replacement pin.
			rec = httptest.NewRecorder()
			body, _ := json.Marshal(map[string]string{"endpoint": servers[1].URL})
			h.HandleTestNode(rec, httptest.NewRequest("POST", "/api/config/nodes/pve-0/test", strings.NewReader(string(body))))
			if (rec.Code == 200) != tc.wantHealth[1] {
				t.Fatalf("member probe=%d: %s", rec.Code, rec.Body.String())
			}
			frame, err := json.Marshal(map[string]any{"scenario": tc.name, "initial_node": initialNode, "saved_node": h.GetAllNodesForAPI(context.Background())[0], "primary_probe": map[string]any{"code": primaryProbeCode, "body": primaryProbeBody}, "member_probe": map[string]any{"code": rec.Code, "body": rec.Body.String()}, "renewed_primary_pin": pin(renewed[0]), "renewed_member_pin": pin(renewed[1])})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("PVE_TLS_CONSUMER_FRAME=%s", frame)
			after, _ := json.Marshal(cfg.PVEInstances[0])
			if string(before) != string(after) {
				t.Fatal("test mutated saved state")
			}
			for i, healthy := range tc.wantHealth {
				if !healthy && authenticated[i].Load() != 0 {
					t.Fatal("rejected TLS sent authenticated HTTP")
				}
			}
		})
	}
}

func TestPVEEndpointTrustAdmission(t *testing.T) {
	instance := config.PVEInstance{Name: "cluster", Host: "https://primary.test:8006", Fingerprint: "primary-pin", VerifySSL: true, TokenValue: "preserved", ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "member", Host: "https://member.test:8006", Fingerprint: "member-pin"}}}
	for _, body := range []string{
		`{"clusterEndpointTrustOverrides":[{"endpoint":"https://outside.test:8006","fingerprint":""}]}`,
		`{"clusterEndpointTrustOverrides":[{"endpoint":"https://member.test:8006"}]}`,
		`{"clusterEndpointTrustOverrides":[{"endpoint":"https://member.test:8006","fingerprint":null}]}`,
		`{"clusterEndpointTrustOverrides":[{"endpoint":"https://member.test:8006","fingerprint":""},{"endpoint":"https://MEMBER.test:8006/","fingerprint":""}]}`,
		`{"fingerprint":"new","clusterEndpointTrustOverrides":[{"endpoint":"https://primary.test:8006","fingerprint":"different"}]}`,
		`{"clusterEndpointTrustOverrides":[{"endpoint":"http://member.test:8006","fingerprint":""}]}`,
		`{"clusterEndpointTrustOverrides":[{"endpoint":"https://member.test:8007","fingerprint":""}]}`,
		`{"clusterEndpointTrustOverrides":[{"endpoint":"https://member.test:8006/other","fingerprint":""}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			cfg := &config.Config{DataPath: t.TempDir(), PVEInstances: []config.PVEInstance{instance.DeepCopy()}}
			h := &ConfigHandlers{defaultConfig: cfg, defaultPersistence: config.NewConfigPersistence(cfg.DataPath), runtime: defaultRuntimeDependencies()}
			before, _ := json.Marshal(cfg)
			rec := httptest.NewRecorder()
			h.HandleUpdateNode(rec, httptest.NewRequest("PUT", "/api/config/nodes/pve-0", strings.NewReader(body)))
			after, _ := json.Marshal(cfg)
			if rec.Code != 400 || string(before) != string(after) {
				t.Fatalf("invalid trust update mutated config: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
	// A storage failure must not publish the trust/credential candidate in memory.
	cfg := &config.Config{DataPath: t.TempDir(), PVEInstances: []config.PVEInstance{instance.DeepCopy()}}
	persistence := config.NewConfigPersistence(cfg.DataPath)
	if err := os.Mkdir(filepath.Join(cfg.DataPath, "nodes.enc"), 0700); err != nil {
		t.Fatal(err)
	}
	h := &ConfigHandlers{defaultConfig: cfg, defaultPersistence: persistence, runtime: defaultRuntimeDependencies()}
	before, _ := json.Marshal(cfg)
	rec := httptest.NewRecorder()
	h.HandleUpdateNode(rec, httptest.NewRequest("PUT", "/api/config/nodes/pve-0", strings.NewReader(`{"fingerprint":"new-pin"}`)))
	after, _ := json.Marshal(cfg)
	if rec.Code != 500 || string(before) != string(after) {
		t.Fatalf("failed save published trust: %d %s", rec.Code, rec.Body.String())
	}
}
