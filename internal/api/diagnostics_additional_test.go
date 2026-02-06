package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	diagnosticsCacheMu.Lock()
	prevCache := diagnosticsCache
	prevTimestamp := diagnosticsCacheTimestamp
	diagnosticsCache = cached
	diagnosticsCacheTimestamp = cachedAt
	diagnosticsCacheMu.Unlock()

	t.Cleanup(func() {
		diagnosticsCacheMu.Lock()
		diagnosticsCache = prevCache
		diagnosticsCacheTimestamp = prevTimestamp
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
}

func TestBuildAPITokenDiagnostic_WithDockerUsage(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-time.Hour)
	cfg := &config.Config{
		DataPath: t.TempDir(),
		EnvOverrides: map[string]bool{
			"API_TOKEN": true,
		},
		APITokens: []config.APITokenRecord{
			{
				ID:        "token-1",
				Name:      "Environment token",
				Prefix:    "pre",
				Suffix:    "suf",
				CreatedAt: now,
			},
			{
				ID:         "token-2",
				Name:       "Legacy token",
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
	if !diag.HasEnvTokens || !diag.HasLegacyToken {
		t.Fatalf("expected env and legacy tokens to be detected")
	}
	if diag.LegacyDockerHostCount != 1 {
		t.Fatalf("legacy docker host count = %d, want 1", diag.LegacyDockerHostCount)
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
	if diag.HostsTotal != 2 {
		t.Fatalf("hosts total = %d, want 2", diag.HostsTotal)
	}
	if diag.HostsOutdatedVersion != 1 {
		t.Fatalf("outdated version count = %d, want 1", diag.HostsOutdatedVersion)
	}
	if diag.HostsWithoutVersion != 1 {
		t.Fatalf("missing version count = %d, want 1", diag.HostsWithoutVersion)
	}
	if diag.HostsWithoutTokenBinding != 1 {
		t.Fatalf("hosts without token binding = %d, want 1", diag.HostsWithoutTokenBinding)
	}
	if diag.HostsNeedingAttention == 0 {
		t.Fatalf("expected hosts needing attention")
	}
}

func TestBuildAlertsDiagnostic_LegacySettings(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	monitor := newMonitorForDiagnostics(t, cfg)

	manager := monitor.GetAlertManager()
	alertCfg := manager.GetConfig()
	legacy := 90.0
	alertCfg.GuestDefaults.CPULegacy = &legacy
	alertCfg.TimeThreshold = 5
	alertCfg.Schedule.GroupingWindow = 10
	alertCfg.Schedule.Grouping.Window = 0
	alertCfg.Schedule.Cooldown = 0
	manager.UpdateConfig(alertCfg)

	diag := buildAlertsDiagnostic(monitor)
	if diag == nil {
		t.Fatalf("expected diagnostics")
	}
	if !diag.LegacyThresholdsDetected {
		t.Fatalf("expected legacy thresholds to be detected")
	}
	if !diag.MissingCooldown || !diag.MissingGroupingWindow {
		t.Fatalf("expected missing schedule settings")
	}
	if len(diag.Notes) == 0 {
		t.Fatalf("expected notes to be populated")
	}
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
		Provider:  config.AIProviderOllama,
		ChatModel: "ollama:llama3",
	}
	handler := &AIHandler{
		legacyPersistence: stubAIPersistence{cfg: aiCfg, dataDir: t.TempDir()},
	}

	mockSvc := new(MockAIService)
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("GetBaseURL").Return("http://localhost:1234")
	handler.legacyService = mockSvc

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
