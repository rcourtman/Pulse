package configapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
)

func TestHandleCanonicalAutoRegister_PVE(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-test-node",
		TokenValue: "created-locally",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Fatalf("status = %v, want success", resp["status"])
	}
	if resp["type"] != "pve" {
		t.Fatalf("type = %v, want pve", resp["type"])
	}
	if resp["source"] != "script" {
		t.Fatalf("source = %v, want script", resp["source"])
	}
	if resp["host"] != server.URL {
		t.Fatalf("host = %v, want normalized host %q", resp["host"], server.URL)
	}
	if resp["nodeId"] != "test-node" {
		t.Fatalf("nodeId = %v, want canonical node identity", resp["nodeId"])
	}
	if resp["nodeName"] != "test-node" {
		t.Fatalf("nodeName = %v, want canonical node identity", resp["nodeName"])
	}
	if resp["tokenId"] != "pulse-monitor@pve!pulse-test-node" {
		t.Fatalf("tokenId = %v, want caller token id", resp["tokenId"])
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token", resp["action"])
	}
	if resp["message"] != "Node test-node registered successfully at "+server.URL {
		t.Fatalf("message = %v, want success guidance", resp["message"])
	}
	if resp["tokenValue"] != "created-locally" {
		t.Fatalf("tokenValue = %v, want caller token secret", resp["tokenValue"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(handler.defaultConfig.PVEInstances))
	}
	instance := handler.defaultConfig.PVEInstances[0]
	if !strings.Contains(instance.Host, "https://") {
		t.Fatalf("expected normalized host, got %q", instance.Host)
	}
	if instance.TokenName != "pulse-monitor@pve!pulse-test-node" {
		t.Fatalf("expected stored token id, got %q", instance.TokenName)
	}
	if instance.TokenValue != "created-locally" {
		t.Fatalf("expected stored token secret, got %q", instance.TokenValue)
	}
}

func TestHandleCanonicalAutoRegister_PVEAcceptsCallerProvidedTokenWithoutCredentials(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
		Source:     "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token", resp["action"])
	}
	if resp["type"] != "pve" {
		t.Fatalf("type = %v, want pve", resp["type"])
	}
	if resp["source"] != "agent" {
		t.Fatalf("source = %v, want agent", resp["source"])
	}
	if resp["host"] != server.URL {
		t.Fatalf("host = %v, want normalized host %q", resp["host"], server.URL)
	}
	if resp["nodeId"] != "test-node" || resp["nodeName"] != "test-node" {
		t.Fatalf("node identity = (%v, %v), want test-node", resp["nodeId"], resp["nodeName"])
	}
	if resp["tokenId"] != "pulse-monitor@pve!pulse-server" {
		t.Fatalf("tokenId = %v, want caller-supplied token id", resp["tokenId"])
	}
	if resp["tokenValue"] != "created-locally" {
		t.Fatalf("tokenValue = %v, want caller-supplied token value", resp["tokenValue"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(handler.defaultConfig.PVEInstances))
	}
	instance := handler.defaultConfig.PVEInstances[0]
	if instance.TokenName != "pulse-monitor@pve!pulse-server" {
		t.Fatalf("token name = %q, want caller-supplied token id", instance.TokenName)
	}
	if instance.TokenValue != "created-locally" {
		t.Fatalf("token value = %q, want caller-supplied token value", instance.TokenValue)
	}
	if instance.Source != "agent" {
		t.Fatalf("source = %q, want agent", instance.Source)
	}
}

func TestHandleCanonicalAutoRegisterSelectsReachableFallbackCandidateHost(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	reachable := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer reachable.Close()

	reqBody := AutoRegisterRequest{
		Type:           "pve",
		Host:           "https://127.0.0.1:1",
		CandidateHosts: []string{"https://127.0.0.1:1", reachable.URL},
		ServerName:     "test-node",
		TokenID:        "pulse-monitor@pve!pulse-test-node",
		TokenValue:     "created-locally",
		Source:         "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(handler.defaultConfig.PVEInstances))
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != reachable.URL {
		t.Fatalf("host = %q, want reachable fallback %q", instance.Host, reachable.URL)
	}

	expectedFingerprint, err := tlsutil.FetchFingerprint(reachable.URL)
	if err != nil {
		t.Fatalf("fetch expected fingerprint: %v", err)
	}
	if instance.Fingerprint != expectedFingerprint {
		t.Fatalf("fingerprint = %q, want %q", instance.Fingerprint, expectedFingerprint)
	}
	if !instance.VerifySSL {
		t.Fatal("expected verifySSL to stay enabled when Pulse captured a reachable fingerprint")
	}

	var resp AutoRegisterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Host != reachable.URL {
		t.Fatalf("response host = %q, want %q", resp.Host, reachable.URL)
	}
}

func TestHandleCanonicalAutoRegisterAdoptsClusterMemberConnectionAddress(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "homelab",
				Host:        "https://192.0.2.10:8006",
				TokenName:   "pulse-monitor@pve!pulse-pve01",
				TokenValue:  "cluster-secret",
				IsCluster:   true,
				ClusterName: "homelab",
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeID: "node/pve01", NodeName: "pve01", Host: "https://pve01:8006", IP: "192.0.2.10"},
					{NodeID: "node/pve02", NodeName: "pve02", Host: "https://pve02:8006", IP: "192.0.2.11"},
				},
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// A reinstalled agent on the second cluster member re-registers with a
	// fresh route-aware address; the member's stored short-DNS host must not
	// win. The stale hostname rides along as an outranked candidate.
	reqBody := AutoRegisterRequest{
		Type:           "pve",
		Host:           server.URL,
		CandidateHosts: []string{server.URL, "https://pve02:8006"},
		ServerName:     "pve02",
		TokenID:        "pulse-monitor@pve!pulse-pve02",
		TokenValue:     "member-token",
		Source:         "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected member match to not create an instance, got %d instances", len(handler.defaultConfig.PVEInstances))
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != "https://192.0.2.10:8006" {
		t.Fatalf("cluster host = %q, want untouched primary host", instance.Host)
	}
	if instance.TokenName != "pulse-monitor@pve!pulse-pve01" || instance.TokenValue != "cluster-secret" {
		t.Fatalf("cluster credentials = (%q, %q), want untouched shared token", instance.TokenName, instance.TokenValue)
	}

	member := instance.ClusterEndpoints[1]
	wantOverride, err := normalizeClusterEndpointIPOverride(server.URL)
	if err != nil {
		t.Fatalf("normalize expected override: %v", err)
	}
	if member.IPOverride != wantOverride {
		t.Fatalf("member override = %q, want adopted agent address %q", member.IPOverride, wantOverride)
	}

	expectedFingerprint, err := tlsutil.FetchFingerprint(server.URL)
	if err != nil {
		t.Fatalf("fetch expected fingerprint: %v", err)
	}
	if member.Fingerprint != expectedFingerprint {
		t.Fatalf("member fingerprint = %q, want pin captured from adopted address %q", member.Fingerprint, expectedFingerprint)
	}

	untouched := instance.ClusterEndpoints[0]
	if untouched.IPOverride != "" || untouched.Fingerprint != "" {
		t.Fatalf("primary member endpoint mutated: override=%q fingerprint=%q", untouched.IPOverride, untouched.Fingerprint)
	}
}

func TestHandleCanonicalAutoRegisterPreservesAdminManagedClusterMemberOverride(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "homelab",
				Host:        "https://192.0.2.10:8006",
				TokenName:   "pulse-monitor@pve!pulse-pve01",
				TokenValue:  "cluster-secret",
				IsCluster:   true,
				ClusterName: "homelab",
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeID: "node/pve01", NodeName: "pve01", Host: "https://pve01:8006", IP: "192.0.2.10"},
					{NodeID: "node/pve02", NodeName: "pve02", Host: "https://pve02:8006", IP: "192.0.2.11", IPOverride: "10.99.0.5:8006", Fingerprint: "AA:BB"},
				},
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:           "pve",
		Host:           server.URL,
		CandidateHosts: []string{server.URL, "https://pve02:8006"},
		ServerName:     "pve02",
		TokenID:        "pulse-monitor@pve!pulse-pve02",
		TokenValue:     "member-token",
		Source:         "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected member match to not create an instance, got %d instances", len(handler.defaultConfig.PVEInstances))
	}

	member := handler.defaultConfig.PVEInstances[0].ClusterEndpoints[1]
	if member.IPOverride != "10.99.0.5:8006" {
		t.Fatalf("member override = %q, want admin-managed override preserved", member.IPOverride)
	}
	if member.Fingerprint != "AA:BB" {
		t.Fatalf("member fingerprint = %q, want pin for the admin-managed address preserved", member.Fingerprint)
	}
}

func TestHandleCanonicalAutoRegisterDisablesStrictTLSWhenFingerprintCaptureFails(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	originalFingerprint := fetchTLSFingerprint
	fetchTLSFingerprint = func(string) (string, error) {
		return "", nil
	}
	t.Cleanup(func() {
		fetchTLSFingerprint = originalFingerprint
	})

	reqBody := AutoRegisterRequest{
		Type:           "pve",
		Host:           "https://pve.local:8006",
		CandidateHosts: []string{"https://pve.local:8006", "https://10.0.0.5:8006"},
		ServerName:     "test-node",
		TokenID:        "pulse-monitor@pve!pulse-test-node",
		TokenValue:     "created-locally",
		Source:         "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(handler.defaultConfig.PVEInstances))
	}

	expectedHost, err := normalizeNodeHost(reqBody.Host, reqBody.Type)
	if err != nil {
		t.Fatalf("normalize expected host: %v", err)
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != expectedHost {
		t.Fatalf("host = %q, want primary normalized host %q", instance.Host, expectedHost)
	}
	if instance.Fingerprint != "" {
		t.Fatalf("fingerprint = %q, want empty when capture failed", instance.Fingerprint)
	}
	if instance.VerifySSL {
		t.Fatal("expected verifySSL to be disabled when Pulse could not capture any candidate fingerprint")
	}
}

func TestHandleCanonicalAutoRegisterRejectsUnknownSource(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
		Source:     "manual",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "source must be 'agent' or 'script'\n" {
		t.Fatalf("body = %q, want canonical unknown-source guidance", body)
	}
	if len(handler.defaultConfig.PVEInstances) != 0 {
		t.Fatalf("expected no stored PVE instances, got %d", len(handler.defaultConfig.PVEInstances))
	}
}

func TestHandleCanonicalAutoRegisterRejectsMissingServerName(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
		Source:     "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "Missing required canonical auto-register fields: serverName\n" {
		t.Fatalf("body = %q, want canonical missing-serverName guidance", body)
	}
	if len(handler.defaultConfig.PVEInstances) != 0 {
		t.Fatalf("expected no stored PVE instances, got %d", len(handler.defaultConfig.PVEInstances))
	}
}

func TestHandleCanonicalAutoRegisterRejectsMissingSource(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-server",
		TokenValue: "created-locally",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "source is required\n" {
		t.Fatalf("body = %q, want canonical missing-source guidance", body)
	}
}

func TestHandleCanonicalAutoRegisterRejectsUnknownType(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:       "pmg",
		Host:       server.URL,
		ServerName: "mail-gateway",
		TokenID:    "pulse-monitor@pmg!pulse-server",
		TokenValue: "created-locally",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "type must be 'pve' or 'pbs'\n" {
		t.Fatalf("body = %q, want canonical unknown-type guidance", body)
	}
	if len(handler.defaultConfig.PVEInstances) != 0 || len(handler.defaultConfig.PBSInstances) != 0 {
		t.Fatalf("expected no stored nodes, got %d PVE and %d PBS", len(handler.defaultConfig.PVEInstances), len(handler.defaultConfig.PBSInstances))
	}
}

func TestHandleCanonicalAutoRegisterRejectsNonCanonicalTokenID(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse@pve!token",
		TokenValue: "created-locally",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "tokenId must be a canonical Pulse-managed token id\n" {
		t.Fatalf("body = %q, want canonical tokenId guidance", body)
	}
	if len(handler.defaultConfig.PVEInstances) != 0 {
		t.Fatalf("expected no stored PVE instances, got %d", len(handler.defaultConfig.PVEInstances))
	}
}

func TestHandleCanonicalAutoRegisterRejectsMismatchedCompletionPayload(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}
	handler := newTestConfigHandlers(t, cfg)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-server",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "tokenId and tokenValue must be provided together\n" {
		t.Fatalf("body = %q, want canonical completion-payload guidance", body)
	}
}

func TestHandleCanonicalAutoRegisterConsolidatesStandaloneOverlapIntoClusterInMemory(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "homelab",
				ClusterName: "cluster-A",
				IsCluster:   true,
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "minipc", Host: server.URL},
				},
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "minipc",
		TokenID:    "pulse-monitor@pve!pulse-cluster-node",
		TokenValue: "cluster-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after consolidation, got %d", len(handler.defaultConfig.PVEInstances))
	}
	cluster := handler.defaultConfig.PVEInstances[0]
	if cluster.TokenName != "pulse-monitor@pve!pulse-cluster-node" {
		t.Fatalf("expected token id to be promoted onto surviving cluster, got %q", cluster.TokenName)
	}
	if cluster.TokenValue != "cluster-token" {
		t.Fatalf("expected token secret to be promoted onto surviving cluster, got %q", cluster.TokenValue)
	}
	if cluster.Source != "script" {
		t.Fatalf("source = %q, want script", cluster.Source)
	}
}

func TestHandleCanonicalAutoRegisterNoOpReRegistrationSkipsMonitorReload(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Start with a cluster that already has minipc as an endpoint; the first
	// auto-register call below promotes the caller's token onto the cluster
	// and is a legitimate state change. The second identical call simulates
	// the periodic agent heartbeat that was producing the UI flicker, and
	// must short-circuit before SaveNodesConfig + monitor reload.
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "delly",
				ClusterName: "delly",
				IsCluster:   true,
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "minipc", Host: server.URL},
				},
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)
	reloadCalls := make(chan struct{}, 1)
	handler.reloadFunc = func() error {
		select {
		case reloadCalls <- struct{}{}:
		default:
		}
		return nil
	}

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "minipc",
		TokenID:    "pulse-monitor@pve!pulse-cluster-node",
		TokenValue: "cluster-token",
		Source:     "agent",
	}

	// First call: warms up the cluster's host/token/source/etc. from the
	// caller's payload. This is a real state change so a reload IS expected.
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()
	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")
	if rec.Code != http.StatusOK {
		t.Fatalf("warm-up call status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	select {
	case <-reloadCalls:
	case <-time.After(time.Second):
		t.Fatal("expected the first state-changing auto-register to call reloadFunc")
	}

	// Second call with the identical payload: this is the no-op scenario
	// the fix targets. No persisted-config delta → no save → no reload.
	req2 := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec2 := httptest.NewRecorder()
	handler.handleCanonicalAutoRegister(rec2, req2, &reqBody, "127.0.0.1")
	if rec2.Code != http.StatusOK {
		t.Fatalf("no-op call status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}

	select {
	case <-reloadCalls:
		t.Fatal("no-op auto-register triggered a monitor reload; expected zero so the WebSocket-visible resource store is not wiped")
	case <-time.After(150 * time.Millisecond):
	}
}

func TestHandleCanonicalAutoRegisterStateChangingRegistrationStillReloads(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Initial state: a cluster with a stale token. The auto-register payload
	// carries a fresh token that must be persisted, so SaveNodesConfig and the
	// monitor reload should still fire.
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "delly",
				ClusterName: "delly",
				IsCluster:   true,
				TokenName:   "pulse-monitor@pve!pulse-cluster-node",
				TokenValue:  "stale-token",
				Source:      "script",
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "minipc", Host: server.URL},
				},
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)
	reloadCalls := make(chan struct{}, 1)
	handler.reloadFunc = func() error {
		select {
		case reloadCalls <- struct{}{}:
		default:
		}
		return nil
	}

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "minipc",
		TokenID:    "pulse-monitor@pve!pulse-cluster-node",
		TokenValue: "fresh-token",
		Source:     "agent",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	select {
	case <-reloadCalls:
	case <-time.After(time.Second):
		t.Fatal("expected a monitor reload after a real token rotation, none observed")
	}
}

func TestHandleCanonicalAutoRegister_PVEPreservesExistingHostnameWhenRequestUsesSameIP(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hostnameURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "existing-node",
				Host:       hostnameURL,
				TokenName:  "pulse-monitor@pve!pulse-existing-node",
				TokenValue: "existing-token",
				Source:     "script",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "existing-node",
		TokenID:    "pulse-monitor@pve!pulse-existing-node",
		TokenValue: "existing-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token for reused canonical /api/auto-register token", resp["action"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after resolved-host dedupe, got %d", len(handler.defaultConfig.PVEInstances))
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != hostnameURL {
		t.Fatalf("host = %q, want existing hostname-preserving host %q", instance.Host, hostnameURL)
	}
	if instance.TokenName != "pulse-monitor@pve!pulse-existing-node" {
		t.Fatalf("token name = %q, want reused Pulse-managed token", instance.TokenName)
	}
	if instance.TokenValue != "existing-token" {
		t.Fatalf("token value = %q, want reused Pulse-managed token", instance.TokenValue)
	}
	if instance.Source != "script" {
		t.Fatalf("source = %q, want script", instance.Source)
	}
}

func TestHandleCanonicalAutoRegisterReturnsCanonicalStoredNodeIdentity(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "test-node",
				Host:       "https://existing.local:8006",
				TokenName:  "pulse-monitor@pve!pulse-existing-node",
				TokenValue: "existing-token",
				Source:     "script",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "test-node",
		TokenID:    "pulse-monitor@pve!pulse-new-node",
		TokenValue: "new-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(handler.defaultConfig.PVEInstances) != 2 {
		t.Fatalf("expected 2 PVE instances, got %d", len(handler.defaultConfig.PVEInstances))
	}
	storedName := handler.defaultConfig.PVEInstances[1].Name
	if storedName == "test-node" {
		t.Fatalf("stored name = %q, want disambiguated canonical node name", storedName)
	}
	if resp["nodeId"] != storedName {
		t.Fatalf("nodeId = %v, want canonical stored node identity %q", resp["nodeId"], storedName)
	}
}

func TestBuildAutoRegisterEventDataUsesCanonicalAutoRegisterTokenIdentity(t *testing.T) {
	req := &AutoRegisterRequest{
		Type:       "pbs",
		Source:     "agent",
		ServerName: "backup-node",
	}

	event := buildAutoRegisterEventData(req, "https://pbs.local:8007", "backup-node (2)", "pulse-monitor@pbs!pulse-backup")

	if got := event["name"]; got != "backup-node (2)" {
		t.Fatalf("name = %#v, want canonical stored node identity", got)
	}
	if got := event["nodeId"]; got != "backup-node (2)" {
		t.Fatalf("nodeId = %#v, want canonical stored node identity", got)
	}
	if got := event["tokenId"]; got != "pulse-monitor@pbs!pulse-backup" {
		t.Fatalf("tokenId = %#v, want canonical token id", got)
	}
	if got := event["source"]; got != "agent" {
		t.Fatalf("source = %#v, want canonical registration source", got)
	}
}

func TestAutoRegisterWebSocketEventTypeDistinguishesCreatedFromExistingRefresh(t *testing.T) {
	if got := autoRegisterWebSocketEventType(true); got != "node_auto_registered" {
		t.Fatalf("created event type = %q, want node_auto_registered", got)
	}
	if got := autoRegisterWebSocketEventType(false); got != "nodes_changed" {
		t.Fatalf("existing-node refresh event type = %q, want nodes_changed", got)
	}
}

func TestHandleCanonicalAutoRegister_PVERotatesStoredPulseTokenOnCallerReplacement(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hostnameURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "existing-node",
				Host:       hostnameURL,
				TokenName:  "pulse-monitor@pve!pulse-existing-node",
				TokenValue: "existing-token",
				Source:     "script",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "existing-node",
		TokenID:    "pulse-monitor@pve!pulse-existing-node",
		TokenValue: "rotated-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token for canonical /api/auto-register token replacement", resp["action"])
	}
	if resp["message"] != "Node existing-node registered successfully at "+server.URL {
		t.Fatalf("message = %v, want success guidance", resp["message"])
	}
	if resp["tokenValue"] != "rotated-token" {
		t.Fatalf("tokenValue = %v, want rotated token secret", resp["tokenValue"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after resolved-host dedupe, got %d", len(handler.defaultConfig.PVEInstances))
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != hostnameURL {
		t.Fatalf("host = %q, want existing hostname-preserving host %q", instance.Host, hostnameURL)
	}
	if instance.TokenValue != "rotated-token" {
		t.Fatalf("token value = %q, want rotated token secret", instance.TokenValue)
	}
	if instance.Source != "script" {
		t.Fatalf("source = %q, want script after caller-supplied token replacement", instance.Source)
	}
	if instance.TokenValue == "existing-token" {
		t.Fatalf("stored token value = %q, want caller replacement to rotate stale token secret", instance.TokenValue)
	}
}

func TestHandleCanonicalAutoRegister_PVEUpdatesExistingNodeOnDHCPHostChange(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "existing-node",
				Host:       "https://10.0.0.10:8006",
				TokenName:  buildPulseMonitorTokenName("pulse.example.com"),
				TokenValue: "existing-token",
				Source:     "script",
			},
		},
	}
	cfg.PVEInstances[0].TokenName = "pulse-monitor@pve!" + cfg.PVEInstances[0].TokenName
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "existing-node",
		TokenID:    "pulse-monitor@pve!" + buildPulseMonitorTokenName("pulse.example.com"),
		TokenValue: "existing-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token for DHCP continuity rerun", resp["action"])
	}

	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance after DHCP continuity dedupe, got %d", len(handler.defaultConfig.PVEInstances))
	}

	instance := handler.defaultConfig.PVEInstances[0]
	if instance.Host != server.URL {
		t.Fatalf("host = %q, want updated host %q", instance.Host, server.URL)
	}
	if instance.TokenName != "pulse-monitor@pve!"+buildPulseMonitorTokenName("pulse.example.com") {
		t.Fatalf("token name = %q, want reused Pulse-managed token", instance.TokenName)
	}
	if instance.TokenValue != "existing-token" {
		t.Fatalf("token value = %q, want reused Pulse-managed token", instance.TokenValue)
	}
	if instance.Source != "script" {
		t.Fatalf("source = %q, want script", instance.Source)
	}
}

func TestHandleCanonicalAutoRegister_PVEKeepsDistinctSameNameTokenWhenFingerprintsConflict(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	incomingFingerprint, err := tlsutil.FetchFingerprint(server.URL)
	if err != nil {
		t.Fatalf("fetch incoming fingerprint: %v", err)
	}
	existingFingerprint := strings.Repeat("a", 64)
	if strings.EqualFold(existingFingerprint, incomingFingerprint) {
		existingFingerprint = strings.Repeat("b", 64)
	}
	tokenID := "pulse-monitor@pve!" + buildPulseMonitorTokenName("pulse.example.com")
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "pve01",
				Host:        "https://10.0.0.10:8006",
				TokenName:   tokenID,
				TokenValue:  "site-one-token",
				Fingerprint: existingFingerprint,
				Source:      "script",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "pve01",
		TokenID:    tokenID,
		TokenValue: "site-two-token",
		Source:     "script",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PVEInstances) != 2 {
		t.Fatalf("PVE instances = %#v, want both sites to remain configured", handler.defaultConfig.PVEInstances)
	}

	existing := handler.defaultConfig.PVEInstances[0]
	if existing.Host != "https://10.0.0.10:8006" || existing.TokenValue != "site-one-token" || existing.Fingerprint != existingFingerprint {
		t.Fatalf("existing site was mutated: %#v", existing)
	}
	registered := handler.defaultConfig.PVEInstances[1]
	if registered.Host != server.URL || registered.TokenValue != "site-two-token" || registered.Fingerprint != incomingFingerprint {
		t.Fatalf("new site = %#v, want independently registered endpoint", registered)
	}
	if strings.EqualFold(registered.Name, existing.Name) {
		t.Fatalf("new site name = %q, want disambiguation from existing %q", registered.Name, existing.Name)
	}
}

func TestHandleCanonicalAutoRegister_PVERotatesFingerprintAtExactHost(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	incomingFingerprint, err := tlsutil.FetchFingerprint(server.URL)
	if err != nil {
		t.Fatalf("fetch incoming fingerprint: %v", err)
	}
	tokenID := "pulse-monitor@pve!" + buildPulseMonitorTokenName("pulse.example.com")
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "pve01",
				Host:        server.URL,
				TokenName:   tokenID,
				TokenValue:  "old-token",
				Fingerprint: strings.Repeat("a", 64),
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "pve01",
		TokenID:    tokenID,
		TokenValue: "rotated-token",
		Source:     "script",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PVEInstances) != 1 {
		t.Fatalf("expected exact-host rerun to remain one PVE instance, got %d", len(handler.defaultConfig.PVEInstances))
	}
	instance := handler.defaultConfig.PVEInstances[0]
	if instance.TokenValue != "rotated-token" || instance.Fingerprint != incomingFingerprint {
		t.Fatalf("exact-host rerun = %#v, want refreshed token and fingerprint", instance)
	}
}

func TestHandleCanonicalAutoRegister_PVEKeepsDistinctClusterMemberWhenFingerprintsConflict(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	incomingFingerprint, err := tlsutil.FetchFingerprint(server.URL)
	if err != nil {
		t.Fatalf("fetch incoming fingerprint: %v", err)
	}
	existingFingerprint := strings.Repeat("a", 64)
	if strings.EqualFold(existingFingerprint, incomingFingerprint) {
		existingFingerprint = strings.Repeat("b", 64)
	}
	tokenID := "pulse-monitor@pve!" + buildPulseMonitorTokenName("pulse.example.com")
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:        "site-one",
				Host:        "https://10.0.0.10:8006",
				TokenName:   tokenID,
				TokenValue:  "site-one-token",
				IsCluster:   true,
				ClusterName: "production",
				ClusterEndpoints: []config.ClusterEndpoint{
					{
						NodeName:    "pve01",
						Host:        "https://10.0.0.11:8006",
						IP:          "10.0.0.11",
						Fingerprint: existingFingerprint,
					},
				},
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "pve01",
		TokenID:    tokenID,
		TokenValue: "site-two-token",
		Source:     "script",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PVEInstances) != 2 {
		t.Fatalf("PVE instances = %#v, want existing cluster and new site", handler.defaultConfig.PVEInstances)
	}
	existing := handler.defaultConfig.PVEInstances[0]
	if existing.TokenValue != "site-one-token" || existing.ClusterEndpoints[0].Fingerprint != existingFingerprint || existing.ClusterEndpoints[0].IPOverride != "" {
		t.Fatalf("existing cluster member was mutated: %#v", existing)
	}
	registered := handler.defaultConfig.PVEInstances[1]
	if registered.Name != "pve01" || registered.Host != server.URL || registered.TokenValue != "site-two-token" || registered.Fingerprint != incomingFingerprint {
		t.Fatalf("new site = %#v, want independent pve01 registration", registered)
	}
}

func TestHandleCanonicalAutoRegister_PBSPreservesExistingHostnameWhenRequestUsesSameIP(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hostnameURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PBSInstances: []config.PBSInstance{
			{
				Name:       "existing-backup",
				Host:       hostnameURL,
				TokenName:  "pulse-monitor@pbs!pulse-existing-backup",
				TokenValue: "existing-token",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pbs",
		Host:       server.URL,
		ServerName: "existing-backup",
		TokenID:    "pulse-monitor@pbs!pulse-existing-backup",
		TokenValue: "existing-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance after resolved-host dedupe, got %d", len(handler.defaultConfig.PBSInstances))
	}

	instance := handler.defaultConfig.PBSInstances[0]
	if instance.Host != hostnameURL {
		t.Fatalf("host = %q, want existing hostname-preserving host %q", instance.Host, hostnameURL)
	}
	if instance.TokenName != "pulse-monitor@pbs!pulse-existing-backup" {
		t.Fatalf("token name = %q, want reused Pulse-managed token", instance.TokenName)
	}
	if instance.TokenValue != "existing-token" {
		t.Fatalf("token value = %q, want reused Pulse-managed token", instance.TokenValue)
	}
}

func TestHandleCanonicalAutoRegister_PBSUpdatesExistingNodeOnDHCPHostChange(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PBSInstances: []config.PBSInstance{
			{
				Name:       "existing-backup",
				Host:       "https://10.0.0.20:8007",
				TokenName:  "pulse-monitor@pbs!" + buildPulseMonitorTokenName("pulse.example.com"),
				TokenValue: "existing-token",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pbs",
		Host:       server.URL,
		ServerName: "existing-backup",
		TokenID:    "pulse-monitor@pbs!" + buildPulseMonitorTokenName("pulse.example.com"),
		TokenValue: "existing-token",
		Source:     "script",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["action"] != "use_token" {
		t.Fatalf("action = %v, want use_token for DHCP continuity rerun", resp["action"])
	}

	if len(handler.defaultConfig.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance after DHCP continuity dedupe, got %d", len(handler.defaultConfig.PBSInstances))
	}

	instance := handler.defaultConfig.PBSInstances[0]
	if instance.Host != server.URL {
		t.Fatalf("host = %q, want updated host %q", instance.Host, server.URL)
	}
	if instance.TokenName != "pulse-monitor@pbs!"+buildPulseMonitorTokenName("pulse.example.com") {
		t.Fatalf("token name = %q, want reused Pulse-managed token", instance.TokenName)
	}
	if instance.TokenValue != "existing-token" {
		t.Fatalf("token value = %q, want reused Pulse-managed token", instance.TokenValue)
	}
}

func TestHandleCanonicalAutoRegister_PBSKeepsDistinctSameNameTokenWhenFingerprintsConflict(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	incomingFingerprint, err := tlsutil.FetchFingerprint(server.URL)
	if err != nil {
		t.Fatalf("fetch incoming fingerprint: %v", err)
	}
	existingFingerprint := strings.Repeat("a", 64)
	if strings.EqualFold(existingFingerprint, incomingFingerprint) {
		existingFingerprint = strings.Repeat("b", 64)
	}
	tokenID := "pulse-monitor@pbs!" + buildPulseMonitorTokenName("pulse.example.com")
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PBSInstances: []config.PBSInstance{
			{
				Name:        "pbs01",
				Host:        "https://10.0.0.20:8007",
				TokenName:   tokenID,
				TokenValue:  "site-one-token",
				Fingerprint: existingFingerprint,
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pbs",
		Host:       server.URL,
		ServerName: "pbs01",
		TokenID:    tokenID,
		TokenValue: "site-two-token",
		Source:     "script",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(handler.defaultConfig.PBSInstances) != 2 {
		t.Fatalf("PBS instances = %#v, want both sites to remain configured", handler.defaultConfig.PBSInstances)
	}
	existing := handler.defaultConfig.PBSInstances[0]
	if existing.Host != "https://10.0.0.20:8007" || existing.TokenValue != "site-one-token" || existing.Fingerprint != existingFingerprint {
		t.Fatalf("existing site was mutated: %#v", existing)
	}
	registered := handler.defaultConfig.PBSInstances[1]
	if registered.Host != server.URL || registered.TokenValue != "site-two-token" || registered.Fingerprint != incomingFingerprint {
		t.Fatalf("new site = %#v, want independently registered endpoint", registered)
	}
	if strings.EqualFold(registered.Name, existing.Name) {
		t.Fatalf("new site name = %q, want disambiguation from existing %q", registered.Name, existing.Name)
	}
}

// TestHandleCanonicalAutoRegister_PVEPreservesDisabledVerifySSL covers #2140:
// an operator who disables "Verify SSL certificate" on an existing node must
// not have it silently re-enabled by an agent health-check re-registration.
func TestHandleCanonicalAutoRegister_PVEPreservesDisabledVerifySSL(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tokenID := "pulse-monitor@pve!" + buildPulseMonitorTokenName("pulse.example.com")
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve01",
				Host:       server.URL,
				TokenName:  tokenID,
				TokenValue: "existing-token",
				VerifySSL:  false,
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pve",
		Host:       server.URL,
		ServerName: "pve01",
		TokenID:    tokenID,
		TokenValue: "rotated-token",
		Source:     "agent",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	instance := handler.defaultConfig.PVEInstances[0]
	if instance.VerifySSL {
		t.Fatalf("re-registration re-enabled VerifySSL on an existing node that disabled it (#2140)")
	}
}

// TestHandleCanonicalAutoRegister_PBSReservesDisabledVerifySSL is the PBS twin
// of TestHandleCanonicalAutoRegister_PVEPreservesDisabledVerifySSL.
func TestHandleCanonicalAutoRegister_PBSReservesDisabledVerifySSL(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", tempDir)

	server := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tokenID := "pulse-monitor@pbs!" + buildPulseMonitorTokenName("pulse.example.com")
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
		PBSInstances: []config.PBSInstance{
			{
				Name:       "pbs01",
				Host:       server.URL,
				TokenName:  tokenID,
				TokenValue: "existing-token",
				VerifySSL:  false,
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	reqBody := AutoRegisterRequest{
		Type:       "pbs",
		Host:       server.URL,
		ServerName: "pbs01",
		TokenID:    tokenID,
		TokenValue: "rotated-token",
		Source:     "agent",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auto-register", nil)
	rec := httptest.NewRecorder()

	handler.handleCanonicalAutoRegister(rec, req, &reqBody, "127.0.0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	instance := handler.defaultConfig.PBSInstances[0]
	if instance.VerifySSL {
		t.Fatalf("re-registration re-enabled VerifySSL on an existing node that disabled it (#2140)")
	}
}

func agentAuthorityFixture(t *testing.T, rawToken string, metadata map[string]string) (*ConfigHandlers, *config.Config) {
	t.Helper()
	stubAutoRegisterNetworkDeps(t)
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport, config.ScopeDockerReport}, metadata)
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve-real",
				Host:       "https://pve-real.local:8006",
				TokenName:  "pulse-monitor@pve!pulse-real",
				TokenValue: "real-secret",
			},
			{
				Name:       "homelab",
				Host:       "https://pve1.local:8006",
				IsCluster:  true,
				TokenName:  "pulse-monitor@pve!pulse-cluster",
				TokenValue: "cluster-secret",
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "pve1", Host: "https://pve1.local:8006", IP: "pve1.local"},
					{NodeName: "pve2", Host: "https://pve2.local:8006", IP: "pve2.local"},
				},
			},
		},
		PBSInstances: []config.PBSInstance{{
			Name:       "pbs-real",
			Host:       "https://pbs-real.local:8007",
			TokenName:  "pulse-monitor@pbs!pulse-real",
			TokenValue: "real-secret",
		}},
	}
	return newTestConfigHandlers(t, cfg), cfg
}

func snapshotAutoRegisterSources(cfg *config.Config) ([]config.PVEInstance, []config.PBSInstance) {
	pve := make([]config.PVEInstance, 0, len(cfg.PVEInstances))
	for _, instance := range cfg.PVEInstances {
		pve = append(pve, instance.DeepCopy())
	}
	return pve, append([]config.PBSInstance(nil), cfg.PBSInstances...)
}

func assertAutoRegisterSourcesUnchanged(t *testing.T, label string, cfg *config.Config, pve []config.PVEInstance, pbs []config.PBSInstance) {
	t.Helper()
	if !reflect.DeepEqual(cfg.PVEInstances, pve) {
		t.Fatalf("%s changed PVE connections:\nbefore %+v\nafter  %+v", label, pve, cfg.PVEInstances)
	}
	if !reflect.DeepEqual(cfg.PBSInstances, pbs) {
		t.Fatalf("%s changed PBS connections:\nbefore %+v\nafter  %+v", label, pbs, cfg.PBSInstances)
	}
}

// An agent token sits on every monitored host. Without an install grant, it
// may change only the connection its spent grant recorded. Nothing the agent
// says about itself counts: not serverName, not candidate addresses, and not
// a bound hostname, which a command-enabled agent may rebind.
func TestAgentTokenAutoRegisterCannotChangeAnotherHostsConnection(t *testing.T) {
	tokens := map[string]map[string]string{
		"unregistered":    nil,
		"bound hostname":  {"bound_hostname": "pve-real", "bound_agent_id": "agent-1"},
		"owns pve-real":   {proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve-real.local:8006"},
		"owns other type": {proxmoxInstallRegistrationTypeKey: "pve", proxmoxInstallRegistrationHostKey: "https://pbs-real.local:8007"},
	}
	cases := []struct {
		name string
		req  AutoRegisterRequest
	}{
		{"pve same host", AutoRegisterRequest{
			Type: "pve", Host: "https://pve-real.local:8006", ServerName: "pve-real", Source: "agent",
			TokenID: "pulse-monitor@pve!pulse-other", TokenValue: "other-secret",
		}},
		{"pve claimed name", AutoRegisterRequest{
			Type: "pve", Host: "https://192.0.2.10:8006", ServerName: "pve-real", Source: "agent",
			TokenID: "pulse-monitor@pve!pulse-real", TokenValue: "other-secret",
		}},
		{"cluster member address", AutoRegisterRequest{
			Type: "pve", Host: "https://pve2.local:8006", CandidateHosts: []string{"https://pve2.local:8006"},
			ServerName: "pve2", Source: "agent",
			TokenID: "pulse-monitor@pve!pulse-cluster", TokenValue: "other-secret",
		}},
		{"pbs same host", AutoRegisterRequest{
			Type: "pbs", Host: "https://pbs-real.local:8007", ServerName: "pbs-real", Source: "agent",
			TokenID: "pulse-monitor@pbs!pulse-other", TokenValue: "other-secret",
		}},
	}
	for tokenName, metadata := range tokens {
		rawToken := strings.ReplaceAll(tokenName, " ", "-") + ".12345678"
		handler, cfg := agentAuthorityFixture(t, rawToken, metadata)
		pve, pbs := snapshotAutoRegisterSources(cfg)
		for _, tc := range cases {
			if tokenName == "owns pve-real" && strings.HasPrefix(tc.name, "pve ") {
				continue // its own connection; covered by the next test
			}
			rec := runAgentAutoRegister(t, handler, rawToken, tc.req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s / %s: status = %d, want 403; body=%s", tokenName, tc.name, rec.Code, rec.Body.String())
			}
			assertAutoRegisterSourcesUnchanged(t, tokenName+" / "+tc.name, cfg, pve, pbs)
		}
	}
}

// The heal flow the agent-token path exists for keeps working: an agent whose
// spent grant recorded a connection rotates that connection's token, whether
// it is a standalone node, a cluster the node belongs to, or a PBS recorded
// before per-type keys.
func TestAgentTokenAutoRegisterUpdatesItsRecordedConnection(t *testing.T) {
	rawToken := "pve-real-agent.12345678"
	handler, cfg := agentAuthorityFixture(t, rawToken, map[string]string{
		proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve-real.local:8006",
	})
	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://pve-real.local:8006", ServerName: "pve-real", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-rotated", TokenValue: "rotated-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("recorded connection status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := cfg.PVEInstances[0]; got.TokenName != "pulse-monitor@pve!pulse-rotated" || got.TokenValue != "rotated-secret" {
		t.Fatalf("recorded connection not updated: token=%q", got.TokenName)
	}

	clusterToken := "pve1-agent.12345678"
	clusterHandler, clusterCfg := agentAuthorityFixture(t, clusterToken, map[string]string{
		proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve1.local:8006",
	})
	rec = runAgentAutoRegister(t, clusterHandler, clusterToken, AutoRegisterRequest{
		Type: "pve", Host: "https://pve1.local:8006", ServerName: "pve1", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-cluster", TokenValue: "rotated-cluster-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("cluster member status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := clusterCfg.PVEInstances[1]; got.TokenValue != "rotated-cluster-secret" {
		t.Fatalf("cluster connection not refreshed by its recorded member: token value %q", got.TokenValue)
	}

	pbsToken := "pbs-real-agent.12345678"
	pbsHandler, pbsCfg := agentAuthorityFixture(t, pbsToken, map[string]string{
		proxmoxInstallRegistrationTypeKey: "pbs",
		proxmoxInstallRegistrationHostKey: "https://pbs-real.local:8007",
	})
	rec = runAgentAutoRegister(t, pbsHandler, pbsToken, AutoRegisterRequest{
		Type: "pbs", Host: "https://pbs-real.local:8007", ServerName: "pbs-real", Source: "agent",
		TokenID: "pulse-monitor@pbs!pulse-rotated", TokenValue: "rotated-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy PBS record status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := pbsCfg.PBSInstances[0]; got.TokenValue != "rotated-secret" {
		t.Fatalf("legacy PBS record did not authorize its own connection: token value %q", got.TokenValue)
	}
}

// A non-primary cluster member heals the shared connection through its own
// recorded address.
func TestAgentTokenAutoRegisterHealsThroughItsClusterMember(t *testing.T) {
	rawToken := "pve2-agent.12345678"
	handler, cfg := agentAuthorityFixture(t, rawToken, map[string]string{
		proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve2.local:8006",
	})
	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://pve2.local:8006", CandidateHosts: []string{"https://pve2.local:8006"},
		ServerName: "pve2", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-cluster", TokenValue: "rotated-cluster-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("cluster member heal status = %d, body=%s", rec.Code, rec.Body.String())
	}
	cluster := cfg.PVEInstances[1]
	if cluster.TokenValue != "rotated-cluster-secret" {
		t.Fatalf("cluster member did not refresh the shared token: %q", cluster.TokenValue)
	}
	if cluster.Host != "https://pve1.local:8006" || cluster.ClusterEndpoints[1].IPOverride != "" {
		t.Fatalf("cluster member heal moved an address: host=%q override=%q", cluster.Host, cluster.ClusterEndpoints[1].IPOverride)
	}
}

// Without an install grant, a token rotates its own connection's credentials
// but never moves the stored address or re-pins it to another address.
func TestAgentTokenAutoRegisterKeepsTheRecordedAddress(t *testing.T) {
	rawToken := "pve-real-mover.12345678"
	handler, cfg := agentAuthorityFixture(t, rawToken, map[string]string{
		proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve-real.local:8006",
	})
	// A stored pin that conflicts with the new address already stops the
	// inferred match; an unpinned connection is the case that reaches here.
	fetchTLSFingerprint = func(string) (string, error) { return "BB:BB", nil }

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://192.0.2.50:8006", ServerName: "pve-real", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-real", TokenValue: "rotated-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("own connection status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got := cfg.PVEInstances[0]
	if got.Host != "https://pve-real.local:8006" {
		t.Fatalf("ungranted agent token moved its connection to %q", got.Host)
	}
	if got.Fingerprint != "" {
		t.Fatalf("ungranted agent token re-pinned its connection from another address: %q", got.Fingerprint)
	}
	if got.TokenValue != "rotated-secret" {
		t.Fatalf("own connection credentials not rotated: %q", got.TokenValue)
	}
}

// The registration check reports canRegister only when completion would
// accept the change, so an agent does not rotate a credential it then cannot
// register.
func TestAgentTokenRegistrationCheckMatchesCompletionAuthority(t *testing.T) {
	check := AutoRegisterRequest{
		Type: "pve", Host: "https://pve-real.local:8006", ServerName: "pve-real", Source: "agent",
		CheckRegistration: true,
	}
	for name, tc := range map[string]struct {
		metadata map[string]string
		want     bool
	}{
		"unowned": {metadata: map[string]string{"bound_hostname": "pve-real"}, want: false},
		"owned": {metadata: map[string]string{
			proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve-real.local:8006",
		}, want: true},
	} {
		rawToken := name + "-check.12345678"
		handler, _ := agentAuthorityFixture(t, rawToken, tc.metadata)
		rec := runAgentAutoRegister(t, handler, rawToken, check)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s check status = %d, body=%s", name, rec.Code, rec.Body.String())
		}
		var response autoRegisterCheckResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("%s decode check: %v", name, err)
		}
		if !response.SourceExists {
			t.Fatalf("%s check did not find the configured source", name)
		}
		if response.CanRegister != tc.want {
			t.Fatalf("%s canRegister = %v, want %v", name, response.CanRegister, tc.want)
		}
	}
}

// Ownership is per service: a token that owns one connection cannot reach
// another connection on the same address but a different port.
func TestAgentTokenOwnershipKeepsPortsDistinct(t *testing.T) {
	rawToken := "proxy-port-agent.12345678"
	handler, cfg := agentAuthorityFixture(t, rawToken, map[string]string{
		proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve"): "https://pve-real.local:8006",
	})
	cfg.PVEInstances = append(cfg.PVEInstances, config.PVEInstance{
		Name:       "pve-behind-port",
		Host:       "https://pve-real.local:18006",
		TokenName:  "pulse-monitor@pve!pulse-behind-port",
		TokenValue: "behind-port-secret",
	})
	pve, pbs := snapshotAutoRegisterSources(cfg)
	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://pve-real.local:18006", ServerName: "pve-behind-port", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-other", TokenValue: "other-secret",
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("other port status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	assertAutoRegisterSourcesUnchanged(t, "other port", cfg, pve, pbs)
}

// A grant spent by the registration check on an existing source records that
// source's own address, even when the agent reached it through a fallback
// candidate, so the same token can heal it later without a grant.
func TestInstallGrantCheckRecordsTheMatchedSourceAddress(t *testing.T) {
	stubAutoRegisterNetworkDeps(t)
	rawToken := "install-fallback.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAgentReport}, map[string]string{
		"install_type": "pve",
		"issued_via":   agentInstallIssuedViaConfig,
	})
	cfg := &config.Config{
		DataPath:  t.TempDir(),
		APITokens: []config.APITokenRecord{record},
		PVEInstances: []config.PVEInstance{{
			Name:       "pve-real",
			Host:       "https://pve-real.local:8006",
			TokenName:  "pulse-monitor@pve!pulse-real",
			TokenValue: "real-secret",
		}},
	}
	handler := newTestConfigHandlers(t, cfg)

	rec := runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://10.0.0.5:8006",
		CandidateHosts: []string{"https://10.0.0.5:8006", "https://pve-real.local:8006"},
		ServerName:     "pve-real", Source: "agent", CheckRegistration: true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("check status = %d, body=%s", rec.Code, rec.Body.String())
	}
	key := proxmoxInstallRegistrationTypedKey(proxmoxInstallRegistrationHostKey, "pve")
	if got := cfg.APITokens[0].Metadata[key]; got != "https://pve-real.local:8006" {
		t.Fatalf("%s = %q, want the matched source address", key, got)
	}

	rec = runAgentAutoRegister(t, handler, rawToken, AutoRegisterRequest{
		Type: "pve", Host: "https://pve-real.local:8006", ServerName: "pve-real", Source: "agent",
		TokenID: "pulse-monitor@pve!pulse-real", TokenValue: "healed-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("heal status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := cfg.PVEInstances[0].TokenValue; got != "healed-secret" {
		t.Fatalf("heal did not rotate the adopted source: %q", got)
	}
}
