package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	alertconfig "github.com/rcourtman/pulse-go-rewrite/internal/alerts/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type stubAIPersistence struct {
	cfg     *config.AIConfig
	dataDir string
	err     error
}

func (s stubAIPersistence) LoadAIConfig() (*config.AIConfig, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.cfg, nil
}

func (s stubAIPersistence) DataDir() string {
	return s.dataDir
}

type proxmoxTestResponse struct {
	status int
	body   string
}

func newProxmoxTestServer(t *testing.T, responses map[string]proxmoxTestResponse) *httptest.Server {
	t.Helper()

	return newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		status := resp.status
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if resp.body != "" {
			_, _ = w.Write([]byte(resp.body))
		}
	}))
}

func newProxmoxClient(t *testing.T, serverURL string) *proxmox.Client {
	t.Helper()

	client, err := proxmox.NewClient(proxmox.ClientConfig{
		Host:       serverURL,
		TokenName:  "user@pam!token",
		TokenValue: "secret",
		VerifySSL:  false,
	})
	if err != nil {
		t.Fatalf("proxmox.NewClient: %v", err)
	}
	return client
}

func newMonitorForDiagnostics(t *testing.T, cfg *config.Config) *monitoring.Monitor {
	t.Helper()

	if cfg == nil {
		cfg = &config.Config{DataPath: t.TempDir()}
	}
	if cfg.DataPath == "" {
		cfg.DataPath = t.TempDir()
	}

	monitor, err := monitoring.New(cfg)
	if err != nil {
		t.Fatalf("monitoring.New: %v", err)
	}
	t.Cleanup(func() { monitor.Stop() })
	return monitor
}

func TestHandleDiagnostics_CacheHit(t *testing.T) {
	cached := DiagnosticsInfo{Version: "cached"}
	cachedAt := time.Now()
	scopeKey := diagnosticsScopeKey(context.Background())

	diagnosticsCacheMu.Lock()
	prevCache := diagnosticsCache
	diagnosticsCache = map[string]cachedDiagnosticsEntry{
		scopeKey: {
			diag:     cached,
			cachedAt: cachedAt,
		},
	}
	diagnosticsCacheMu.Unlock()

	t.Cleanup(func() {
		diagnosticsCacheMu.Lock()
		diagnosticsCache = prevCache
		diagnosticsCacheMu.Unlock()
	})

	router := &Router{config: &config.Config{}}
	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
	rec := httptest.NewRecorder()

	router.handleDiagnostics(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload DiagnosticsInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode diagnostics: %v", err)
	}
	if payload.Version != "cached" {
		t.Fatalf("version = %q, want cached", payload.Version)
	}
	if rec.Header().Get("X-Diagnostics-Cached-At") == "" {
		t.Fatalf("expected cached-at header to be set")
	}
}

func TestHandleDiagnostics_CacheIsScopedByOrg(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)
	cachedAt := time.Now()

	diagnosticsCacheMu.Lock()
	prevCache := diagnosticsCache
	diagnosticsCache = map[string]cachedDiagnosticsEntry{
		"org-a": {
			diag:     DiagnosticsInfo{Version: "cached-org-a"},
			cachedAt: cachedAt,
		},
	}
	diagnosticsCacheMu.Unlock()

	t.Cleanup(func() {
		diagnosticsCacheMu.Lock()
		diagnosticsCache = prevCache
		diagnosticsCacheMu.Unlock()
	})

	router := &Router{config: cfg, monitor: monitor}
	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
	req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, "org-b"))
	rec := httptest.NewRecorder()

	router.handleDiagnostics(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload DiagnosticsInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode diagnostics: %v", err)
	}
	if payload.Version == "cached-org-a" {
		t.Fatalf("expected org-b diagnostics to bypass org-a cache entry")
	}
}

// TestComputeDiagnostics_PVE_HonoursStoredFingerprint verifies that the PVE
// diagnostic test client uses the per-node TLS fingerprint when one is stored.
// Regression test: previously the diagnostics handler omitted node.Fingerprint
// from its testCfg, so connections with VerifySSL=true and a self-signed cert
// (the standard Proxmox configuration) failed system-CA verification in the
// diagnostic probe even when the actual poller — which DOES pass the
// fingerprint — was connecting fine. The PBS section already passed the
// fingerprint; this test pins the PVE branch to do the same.
func TestComputeDiagnostics_PVE_HonoursStoredFingerprint(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"node":"node1","status":"online"}]}`))
		case "/api2/json/nodes/node1/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"pveversion":"pve-manager/9.1.9"}}`))
		case "/api2/json/cluster/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cert := server.Certificate()
	if cert == nil {
		t.Fatalf("expected httptest TLS server to expose its leaf certificate")
	}
	fingerprint := sha256.Sum256(cert.Raw)
	storedFingerprint := hex.EncodeToString(fingerprint[:])

	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{{
			Name:        "fingerprint-pinned",
			Host:        server.URL,
			TokenName:   "user@pam!token",
			TokenValue:  "secret",
			VerifySSL:   true, // strict TLS — must rely on stored fingerprint, not system CAs
			Fingerprint: storedFingerprint,
		}},
	}
	monitor := newMonitorForDiagnostics(t, cfg)
	router := &Router{config: cfg, monitor: monitor}

	diag := router.computeDiagnostics(context.Background())

	if len(diag.Nodes) != 1 {
		t.Fatalf("expected 1 PVE node diagnostic, got %d", len(diag.Nodes))
	}
	node := diag.Nodes[0]
	if !node.Connected {
		t.Fatalf("expected diagnostic to honour stored fingerprint and report Connected=true; got Connected=false, error=%q", node.Error)
	}
	if node.AuthMethod != "api_token" {
		t.Errorf("expected authMethod=api_token, got %q", node.AuthMethod)
	}
}

func TestDiagnosticsInfo_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyDiagnosticsInfo())
	if err != nil {
		t.Fatalf("marshal empty diagnostics info: %v", err)
	}

	for _, want := range []string{
		`"nodes":[]`,
		`"pbs":[]`,
		`"errors":[]`,
		`"nodeSnapshots":[]`,
		`"guestSnapshots":[]`,
		`"memorySources":[]`,
		`"memorySourceBreakdown":[]`,
	} {
		if !strings.Contains(string(payload), want) {
			t.Fatalf("expected empty diagnostics info to retain %s, got %s", want, payload)
		}
	}
}

func TestComputeDiagnostics_Basic(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)

	router := &Router{config: cfg, monitor: monitor}
	diag := router.computeDiagnostics(context.Background())

	if diag.System.OS == "" {
		t.Fatalf("expected system OS to be populated")
	}
	if diag.MetricsStore == nil {
		t.Fatalf("expected metrics store diagnostics")
	}
	if diag.APITokens == nil {
		t.Fatalf("expected api token diagnostics")
	}
	if diag.Discovery == nil {
		t.Fatalf("expected discovery diagnostics")
	}
	if diag.AIChat == nil {
		t.Fatalf("expected ai chat diagnostics")
	}

	payload, err := json.Marshal(diag.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal diagnostics: %v", err)
	}
	for _, forbidden := range []string{
		"commercialFunnel",
		"infrastructureOnboarding",
		"pricing_viewed",
		"checkout_clicked",
		"credentials_opened",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("customer diagnostics leaked internal analytics field %q: %s", forbidden, payload)
		}
	}
}

func TestBuildAPITokenDiagnostic_WithDockerUsage(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-time.Hour)
	cfg := &config.Config{
		DataPath: t.TempDir(),
		APITokens: []config.APITokenRecord{
			{
				ID:        "token-1",
				Name:      "Primary token",
				Prefix:    "pre",
				Suffix:    "suf",
				CreatedAt: now,
			},
			{
				ID:         "token-2",
				Name:       "Secondary token",
				CreatedAt:  now,
				LastUsedAt: &lastUsed,
			},
		},
	}

	monitor := newMonitorForDiagnostics(t, cfg)

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:      "agent-1",
			Version: "1.0.0",
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-host-1",
			MachineID: "machine-1",
		},
		Timestamp: time.Now().UTC(),
	}
	if _, err := monitor.ApplyDockerReport(report, &config.APITokenRecord{ID: "token-1"}); err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}

	legacyReport := report
	legacyReport.Agent.ID = "agent-2"
	legacyReport.Host.Hostname = "docker-legacy"
	legacyReport.Host.MachineID = "machine-2"
	if _, err := monitor.ApplyDockerReport(legacyReport, nil); err != nil {
		t.Fatalf("ApplyDockerReport legacy: %v", err)
	}

	diag := buildAPITokenDiagnostic(cfg, monitor)
	if diag == nil || !diag.Enabled {
		t.Fatalf("expected diagnostics enabled")
	}
	if diag.TokenCount != 2 {
		t.Fatalf("token count = %d, want 2", diag.TokenCount)
	}
	if diag.UnusedTokenCount != 1 {
		t.Fatalf("unused token count = %d, want 1", diag.UnusedTokenCount)
	}
	if len(diag.Usage) != 1 || diag.Usage[0].TokenID != "token-1" {
		t.Fatalf("expected token usage for token-1")
	}
}

func TestBuildDockerAgentDiagnostic(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)

	now := time.Now().UTC()
	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:      "agent-1",
			Version: "0.9.0",
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-old",
			MachineID: "machine-old",
		},
		Timestamp: now.Add(-5 * time.Minute),
	}
	if _, err := monitor.ApplyDockerReport(report, &config.APITokenRecord{ID: "token-1"}); err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}

	legacyReport := report
	legacyReport.Agent.ID = "agent-2"
	legacyReport.Agent.Version = ""
	legacyReport.Host.Hostname = "docker-legacy"
	legacyReport.Host.MachineID = "machine-legacy"
	legacyReport.Timestamp = now.Add(-20 * time.Minute)
	if _, err := monitor.ApplyDockerReport(legacyReport, nil); err != nil {
		t.Fatalf("ApplyDockerReport legacy: %v", err)
	}

	diag := buildDockerAgentDiagnostic(monitor, "1.0.0")
	if diag == nil {
		t.Fatalf("expected diagnostics")
	}
	if diag.AgentsTotal != 2 {
		t.Fatalf("agents total = %d, want 2", diag.AgentsTotal)
	}
	if diag.AgentsOutdatedVersion != 1 {
		t.Fatalf("outdated version count = %d, want 1", diag.AgentsOutdatedVersion)
	}
	if diag.AgentsWithoutVersion != 1 {
		t.Fatalf("missing version count = %d, want 1", diag.AgentsWithoutVersion)
	}
	if diag.AgentsWithoutTokenBinding != 1 {
		t.Fatalf("agents without token binding = %d, want 1", diag.AgentsWithoutTokenBinding)
	}
	if diag.AgentsNeedingAttention == 0 {
		t.Fatalf("expected agents needing attention")
	}

	diagnosticText := strings.Join(diag.Notes, "\n")
	for _, attention := range diag.Attention {
		diagnosticText += "\n" + strings.Join(attention.Issues, "\n")
	}
	if strings.Contains(strings.ToLower(diagnosticText), "container runtime") {
		t.Fatalf("Docker diagnostics should not expose generic container-runtime copy, got %q", diagnosticText)
	}
	for _, want := range []string{
		"Docker / Podman agent is still using the shared API token",
		"1 Docker / Podman agent is out of date",
		"Settings → Infrastructure",
	} {
		if !strings.Contains(diagnosticText, want) {
			t.Fatalf("expected Docker / Podman diagnostic copy %q in %q", want, diagnosticText)
		}
	}
}

func TestBuildDockerAgentDiagnosticEmptyUsesDockerPodmanCopy(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)

	diag := buildDockerAgentDiagnostic(monitor, "1.0.0")
	if diag == nil {
		t.Fatalf("expected diagnostics")
	}

	notes := strings.Join(diag.Notes, "\n")
	if strings.Contains(strings.ToLower(notes), "container runtime") {
		t.Fatalf("empty Docker diagnostics should not expose generic container-runtime copy, got %q", notes)
	}
	for _, want := range []string{
		"No Docker / Podman agents have reported in yet",
		"Settings → Infrastructure",
	} {
		if !strings.Contains(notes, want) {
			t.Fatalf("expected empty Docker / Podman diagnostic copy %q in %q", want, notes)
		}
	}
}

func TestBuildAlertsDiagnostic_LegacySettings(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)

	manager := monitor.GetAlertManager()
	alertCfg := manager.GetConfig()
	alertCfg.Schedule.Grouping.Window = 0
	alertCfg.Schedule.Cooldown = 0
	manager.UpdateConfig(alertCfg)

	diag := buildAlertsDiagnostic(monitor)
	if diag == nil {
		t.Fatalf("expected diagnostics")
	}
	if !diag.MissingCooldown || !diag.MissingGroupingWindow {
		t.Fatalf("expected missing schedule settings")
	}
	if len(diag.Notes) == 0 {
		t.Fatalf("expected notes to be populated")
	}
}

// Triage cases like #1341 hinge on whether a persisted override key matches
// the runtime resource ID. Surface override keys + thresholds in the
// diagnostics export so we don't have to ask users to cat the alerts config.
func TestBuildAlertsDiagnostic_OverridesEmittedForTriage(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)

	manager := monitor.GetAlertManager()
	alertCfg := manager.GetConfig()
	alertCfg.Schedule.Cooldown = 600
	alertCfg.Schedule.Grouping.Window = 30

	usage := alertconfig.HysteresisThreshold{Trigger: 50, Clear: 45}
	cpu := alertconfig.HysteresisThreshold{Trigger: 75, Clear: 70}
	alertCfg.Overrides = map[string]alertconfig.ThresholdConfig{
		"pve5-ceph-pool-data_replication": {Usage: &usage},
		"pve5-101":                        {CPU: &cpu, Disabled: true},
	}
	manager.UpdateConfig(alertCfg)

	diag := buildAlertsDiagnostic(monitor)
	if diag == nil {
		t.Fatalf("expected diagnostics")
	}
	if len(diag.Overrides) != 2 {
		t.Fatalf("override count = %d, want 2", len(diag.Overrides))
	}

	byKey := map[string]AlertsOverrideDiagnostic{}
	for _, o := range diag.Overrides {
		byKey[o.Key] = o
	}

	ceph, ok := byKey["pve5-ceph-pool-data_replication"]
	if !ok {
		t.Fatalf("expected ceph pool override key in diagnostics, got keys %v", keysOfOverrideDiag(byKey))
	}
	if got := ceph.Thresholds["usage"]; got != 50 {
		t.Fatalf("ceph usage threshold = %.1f, want 50", got)
	}
	if ceph.Disabled {
		t.Fatalf("ceph override should not be disabled")
	}

	guest, ok := byKey["pve5-101"]
	if !ok {
		t.Fatalf("expected guest override key in diagnostics, got keys %v", keysOfOverrideDiag(byKey))
	}
	if got := guest.Thresholds["cpu"]; got != 75 {
		t.Fatalf("guest cpu threshold = %.1f, want 75", got)
	}
	if !guest.Disabled {
		t.Fatalf("guest override Disabled flag lost in summary")
	}
}

func keysOfOverrideDiag(m map[string]AlertsOverrideDiagnostic) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestBuildDiscoveryDiagnostic_ConfigOnly(t *testing.T) {
	cfg := &config.Config{
		DiscoveryEnabled: true,
		DiscoverySubnet:  "",
		Discovery: config.DiscoveryConfig{
			EnvironmentOverride: " 10.0.0.0/24 ",
		},
	}

	diag := buildDiscoveryDiagnostic(cfg, nil)
	if diag == nil {
		t.Fatalf("expected discovery diagnostics")
	}
	if diag.ConfiguredSubnet != "auto" {
		t.Fatalf("configured subnet = %q, want auto", diag.ConfiguredSubnet)
	}
	if diag.EnvironmentOverride != "10.0.0.0/24" {
		t.Fatalf("environment override = %q, want trimmed", diag.EnvironmentOverride)
	}
	if diag.SubnetAllowlist == nil {
		t.Fatalf("expected allowlist to be initialized")
	}
}

func TestBuildAIChatDiagnostic_WithService(t *testing.T) {
	aiCfg := &config.AIConfig{
		Enabled:   true,
		ChatModel: "ollama:llama3",
	}
	handler := &AIHandler{
		defaultPersistence: stubAIPersistence{cfg: aiCfg, dataDir: t.TempDir()},
	}

	mockSvc := new(MockAIService)
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("GetBaseURL").Return("http://localhost:1234")
	handler.defaultService = mockSvc

	diag := buildAIChatDiagnostic(&config.Config{}, handler)
	if diag == nil || !diag.Enabled {
		t.Fatalf("expected ai chat to be enabled")
	}
	if !diag.Running || !diag.Healthy {
		t.Fatalf("expected ai chat to be running and healthy")
	}
	if diag.Port != 1234 {
		t.Fatalf("port = %d, want 1234", diag.Port)
	}
	if diag.Model != "ollama:llama3" {
		t.Fatalf("model = %q, want ollama:llama3", diag.Model)
	}
}

func TestCheckVMDiskMonitoring_Success(t *testing.T) {
	responses := map[string]proxmoxTestResponse{
		"/api2/json/nodes":                                {body: `{"data":[{"node":"pve1","status":"online"}]}`},
		"/api2/json/nodes/pve1/qemu":                      {body: `{"data":[{"vmid":100,"name":"vm-100","node":"pve1","status":"running","template":0}]}`},
		"/api2/json/nodes/pve1/qemu/100/status/current":   {body: `{"data":{"agent":1}}`},
		"/api2/json/nodes/pve1/qemu/100/agent/get-fsinfo": {body: `{"data":{"result":[{"name":"root","type":"ext4","mountpoint":"/","total-bytes":100,"used-bytes":50}]}}`},
	}
	server := newProxmoxTestServer(t, responses)
	defer server.Close()

	client := newProxmoxClient(t, server.URL)
	router := &Router{}
	result := router.checkVMDiskMonitoring(context.Background(), client, "")

	if result.VMsFound != 1 || result.VMsWithAgent != 1 || result.VMsWithDiskData != 1 {
		t.Fatalf("unexpected VM stats: %+v", result)
	}
	if !strings.Contains(result.TestResult, "SUCCESS") {
		t.Fatalf("expected success test result, got %q", result.TestResult)
	}
}

func TestCheckVMDiskMonitoring_NoNodes(t *testing.T) {
	responses := map[string]proxmoxTestResponse{
		"/api2/json/nodes": {body: `{"data":[]}`},
	}
	server := newProxmoxTestServer(t, responses)
	defer server.Close()

	client := newProxmoxClient(t, server.URL)
	router := &Router{}
	result := router.checkVMDiskMonitoring(context.Background(), client, "")
	if result.TestResult != "No nodes found" {
		t.Fatalf("test result = %q, want No nodes found", result.TestResult)
	}
}

func TestCheckPhysicalDisks_Found(t *testing.T) {
	responses := map[string]proxmoxTestResponse{
		"/api2/json/nodes":                 {body: `{"data":[{"node":"pve1","status":"online"}]}`},
		"/api2/json/nodes/pve1/disks/list": {body: `{"data":[{"devpath":"/dev/sda","model":"Test","serial":"ABC","type":"sata","health":"PASSED"}]}`},
	}
	server := newProxmoxTestServer(t, responses)
	defer server.Close()

	client := newProxmoxClient(t, server.URL)
	router := &Router{}
	result := router.checkPhysicalDisks(context.Background(), client, "")

	if result.NodesWithDisks != 1 || result.TotalDisks != 1 {
		t.Fatalf("unexpected disk totals: %+v", result)
	}
	if !strings.Contains(result.TestResult, "Found 1 disks") {
		t.Fatalf("unexpected test result: %q", result.TestResult)
	}
}

func TestCheckPhysicalDisks_PermissionDenied(t *testing.T) {
	responses := map[string]proxmoxTestResponse{
		"/api2/json/nodes":                 {body: `{"data":[{"node":"pve1","status":"online"}]}`},
		"/api2/json/nodes/pve1/disks/list": {status: http.StatusForbidden, body: `{"errors":"forbidden"}`},
	}
	server := newProxmoxTestServer(t, responses)
	defer server.Close()

	client := newProxmoxClient(t, server.URL)
	router := &Router{}
	result := router.checkPhysicalDisks(context.Background(), client, "")

	if len(result.NodeResults) != 1 {
		t.Fatalf("expected one node result")
	}
	if result.NodeResults[0].APIResponse != "Permission denied" {
		t.Fatalf("api response = %q, want Permission denied", result.NodeResults[0].APIResponse)
	}
	foundNote := false
	for _, note := range result.Recommendations {
		if strings.Contains(note, "permissions") {
			foundNote = true
			break
		}
	}
	if !foundNote {
		t.Fatalf("expected permissions recommendation")
	}
}
