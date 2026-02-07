package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestMaybeRefreshClusterInfo_UpdatesMetadata(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	originalDetect := detectPVECluster
	t.Cleanup(func() { detectPVECluster = originalDetect })

	called := false
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		called = true
		return true, "unknown cluster", []config.ClusterEndpoint{
			{NodeName: "node-1", Host: "https://node-1.local:8006"},
		}
	}

	instance := config.PVEInstance{
		Name:       "pve-1",
		Host:       "https://pve-1.local:8006",
		TokenValue: "token",
	}

	handler.maybeRefreshClusterInfo(context.Background(), &instance)

	if !called {
		t.Fatalf("expected detectPVECluster to be called")
	}
	if !instance.IsCluster {
		t.Fatalf("expected instance to be marked as cluster")
	}
	if instance.ClusterName != "pve-1" {
		t.Fatalf("expected cluster name to default to instance name, got %q", instance.ClusterName)
	}
	if len(instance.ClusterEndpoints) != 1 {
		t.Fatalf("expected cluster endpoints to be updated")
	}
}

func TestMaybeRefreshClusterInfo_SkipsWithoutCredentials(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	originalDetect := detectPVECluster
	t.Cleanup(func() { detectPVECluster = originalDetect })

	called := false
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		called = true
		return true, "cluster", nil
	}

	instance := config.PVEInstance{
		Name: "pve-1",
		Host: "https://pve-1.local:8006",
	}

	handler.maybeRefreshClusterInfo(context.Background(), &instance)

	if called {
		t.Fatalf("expected detectPVECluster to be skipped without credentials")
	}
}

func TestMaybeRefreshClusterInfo_SkipsWithinCooldown(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	originalDetect := detectPVECluster
	t.Cleanup(func() { detectPVECluster = originalDetect })

	called := false
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		called = true
		return true, "cluster", nil
	}

	instance := config.PVEInstance{
		Name:       "pve-1",
		Host:       "https://pve-1.local:8006",
		TokenValue: "token",
	}
	handler.lastClusterDetection[instance.Name] = time.Now()

	handler.maybeRefreshClusterInfo(context.Background(), &instance)

	if called {
		t.Fatalf("expected detectPVECluster to be skipped during cooldown")
	}
}

func TestIsContainerSSHRestricted(t *testing.T) {
	t.Setenv("PULSE_DOCKER", "true")
	t.Setenv("PULSE_DEV_ALLOW_CONTAINER_SSH", "")

	if !isContainerSSHRestricted() {
		t.Fatalf("expected SSH to be restricted in container")
	}

	t.Setenv("PULSE_DEV_ALLOW_CONTAINER_SSH", "true")
	if isContainerSSHRestricted() {
		t.Fatalf("expected SSH restriction to be disabled when override is true")
	}
}

func TestResolveHostnameToIP(t *testing.T) {
	if got := resolveHostnameToIP("https://127.0.0.1:8006"); got != "127.0.0.1" {
		t.Fatalf("expected IP passthrough, got %q", got)
	}

	got := resolveHostnameToIP("https://localhost:8006")
	if got == "" || (got != "127.0.0.1" && got != "::1") {
		t.Fatalf("expected localhost to resolve to loopback, got %q", got)
	}

	if got := resolveHostnameToIP("not-a-url"); got != "" {
		t.Fatalf("expected invalid host to return empty string, got %q", got)
	}
}

func TestGetAllNodesForAPI(t *testing.T) {
	monitorDisks := true
	tempEnabled := true
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{
			{
				Name:                         "pve-1",
				Host:                         "https://pve-1.local:8006",
				GuestURL:                     "https://guest.local",
				User:                         "root@pam",
				Password:                     "secret",
				TokenName:                    "token",
				TokenValue:                   "token-value",
				Fingerprint:                  "fp",
				VerifySSL:                    true,
				MonitorVMs:                   true,
				MonitorContainers:            true,
				MonitorStorage:               false,
				MonitorBackups:               true,
				MonitorPhysicalDisks:         &monitorDisks,
				PhysicalDiskPollingMinutes:   15,
				TemperatureMonitoringEnabled: &tempEnabled,
				IsCluster:                    true,
				ClusterName:                  "cluster-1",
				ClusterEndpoints: []config.ClusterEndpoint{
					{NodeName: "pve-1", Host: "https://pve-1.local:8006"},
				},
				Source: "agent",
			},
		},
		PBSInstances: []config.PBSInstance{
			{
				Name:              "pbs-1",
				Host:              "https://pbs-1.local:8007",
				GuestURL:          "https://pbs-guest.local",
				User:              "backup@pam",
				TokenName:         "token",
				TokenValue:        "pbs-token",
				VerifySSL:         false,
				MonitorDatastores: true,
				ExcludeDatastores: []string{"ds1"},
				Source:            "script",
			},
		},
		PMGInstances: []config.PMGInstance{
			{
				Name: "pmg-1",
				Host: "https://pmg-1.local:8008",
				User: "admin@pam",
			},
		},
	}

	handler := newTestConfigHandlers(t, cfg)
	nodes := handler.GetAllNodesForAPI(context.Background())

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	var pveNode, pbsNode, pmgNode *NodeResponse
	for i := range nodes {
		node := nodes[i]
		switch node.Type {
		case "pve":
			pveNode = &node
		case "pbs":
			pbsNode = &node
		case "pmg":
			pmgNode = &node
		}
	}

	if pveNode == nil || pbsNode == nil || pmgNode == nil {
		t.Fatalf("expected pve, pbs, and pmg nodes to be present")
	}

	if !pveNode.HasPassword || !pveNode.HasToken || pveNode.ClusterName != "cluster-1" {
		t.Fatalf("unexpected PVE node fields: %+v", pveNode)
	}
	if pveNode.MonitorPhysicalDisks == nil || !*pveNode.MonitorPhysicalDisks {
		t.Fatalf("expected PVE MonitorPhysicalDisks to be true")
	}
	if pveNode.Status != "disconnected" {
		t.Fatalf("expected PVE status to be disconnected, got %q", pveNode.Status)
	}

	if !pbsNode.HasToken || len(pbsNode.ExcludeDatastores) != 1 {
		t.Fatalf("unexpected PBS node fields: %+v", pbsNode)
	}

	if !pmgNode.MonitorMailStats || pmgNode.MonitorQueues || pmgNode.MonitorQuarantine {
		t.Fatalf("unexpected PMG monitoring flags: %+v", pmgNode)
	}
}

func TestHandleRefreshClusterNodes_Success(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{
			{
				Name:       "pve-1",
				Host:       "https://pve-1.local:8006",
				TokenValue: "token",
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	originalDetect := detectPVECluster
	t.Cleanup(func() { detectPVECluster = originalDetect })
	detectPVECluster = func(clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return true, "cluster-1", []config.ClusterEndpoint{
			{NodeName: "node-1", Host: "https://node-1.local:8006"},
			{NodeName: "node-2", Host: "https://node-2.local:8006"},
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes/pve-0/refresh-cluster", nil)
	rec := httptest.NewRecorder()
	handler.HandleRefreshClusterNodes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["clusterName"] != "cluster-1" {
		t.Fatalf("expected clusterName to be cluster-1, got %v", resp["clusterName"])
	}
	if cfg.PVEInstances[0].ClusterName != "cluster-1" || !cfg.PVEInstances[0].IsCluster {
		t.Fatalf("expected instance to be updated as cluster")
	}
	if len(cfg.PVEInstances[0].ClusterEndpoints) != 2 {
		t.Fatalf("expected cluster endpoints to be updated")
	}
}

func TestHandleRefreshClusterNodes_InvalidNodeType(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes/pbs-0/refresh-cluster", nil)
	rec := httptest.NewRecorder()
	handler.HandleRefreshClusterNodes(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleTestNodeConfig_InvalidType(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":"invalid"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config/nodes/test-config", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleTestNodeConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleTestNode_InvalidPath(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config/nodes/pve-0", nil)
	rec := httptest.NewRecorder()
	handler.HandleTestNode(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestGetNodeStatus_RecentlyAutoRegistered(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	handler.markAutoRegistered("pve", "node-1")

	if status := handler.getNodeStatus(context.Background(), "pve", "node-1"); status != "connected" {
		t.Fatalf("expected connected for recently auto-registered node, got %q", status)
	}
	if status := handler.getNodeStatus(context.Background(), "pve", "node-2"); status != "disconnected" {
		t.Fatalf("expected disconnected for unknown node, got %q", status)
	}
}

func TestHandleGetSystemSettings_ConfigOverrides(t *testing.T) {
	cfg := &config.Config{
		DataPath:                     t.TempDir(),
		PVEPollingInterval:           30 * time.Second,
		PBSPollingInterval:           90 * time.Second,
		BackupPollingInterval:        12 * time.Second,
		FrontendPort:                 3000,
		AllowedOrigins:               "https://example.com",
		ConnectionTimeout:            15 * time.Second,
		UpdateChannel:                "stable",
		AutoUpdateEnabled:            true,
		AutoUpdateCheckInterval:      6 * time.Hour,
		AutoUpdateTime:               "03:30",
		LogLevel:                     "debug",
		DiscoveryEnabled:             true,
		DiscoverySubnet:              "10.0.0.0/24",
		Discovery:                    config.DefaultDiscoveryConfig(),
		EnableBackupPolling:          false,
		PublicURL:                    "https://public.example",
		TemperatureMonitoringEnabled: true,
		DisableLegacyRouteRedirects:  true,
	}
	handler := newTestConfigHandlers(t, cfg)

	settings := config.DefaultSystemSettings()
	settings.Theme = "light"
	settings.FullWidthMode = true
	if err := handler.getPersistence(context.Background()).SaveSystemSettings(*settings); err != nil {
		t.Fatalf("SaveSystemSettings: %v", err)
	}

	t.Setenv("PULSE_AUTH_HIDE_LOCAL_LOGIN", "1")

	req := httptest.NewRequest(http.MethodGet, "/api/config/system", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetSystemSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d", rec.Code)
	}

	var resp struct {
		config.SystemSettings
		EnvOverrides map[string]bool `json:"envOverrides"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.PVEPollingInterval != 30 {
		t.Fatalf("expected PVEPollingInterval to be 30, got %d", resp.PVEPollingInterval)
	}
	if resp.Theme != "light" || !resp.FullWidthMode {
		t.Fatalf("expected persisted theme settings to remain, got %+v", resp.SystemSettings)
	}
	if resp.BackupPollingEnabled == nil || *resp.BackupPollingEnabled {
		t.Fatalf("expected backup polling enabled to be false")
	}
	if !resp.DisableLegacyRouteRedirects {
		t.Fatalf("expected disableLegacyRouteRedirects to be true")
	}
	if !resp.EnvOverrides["hideLocalLogin"] {
		t.Fatalf("expected hideLocalLogin env override to be true")
	}
}

func TestHandleGetMockMode(t *testing.T) {
	prevConfig := mock.GetConfig()
	prevEnabled := mock.IsMockEnabled()
	t.Cleanup(func() {
		mock.SetMockConfig(prevConfig)
		mock.SetEnabled(prevEnabled)
	})

	mock.SetEnabled(false)
	mock.SetMockConfig(mock.MockConfig{
		NodeCount:     3,
		RandomMetrics: false,
	})

	handler := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir()})
	req := httptest.NewRequest(http.MethodGet, "/api/config/mock", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetMockMode(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d", rec.Code)
	}

	var resp struct {
		Enabled bool            `json:"enabled"`
		Config  mock.MockConfig `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Enabled {
		t.Fatalf("expected mock mode disabled")
	}
	if resp.Config.NodeCount != 3 || resp.Config.RandomMetrics {
		t.Fatalf("unexpected mock config: %+v", resp.Config)
	}
}

func TestHandleAgentInstallCommand(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":"pve"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config/agent-install", bytes.NewReader(body))
	req.Host = "example.com:8080"
	rec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp AgentInstallCommandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Token == "" || resp.Command == "" {
		t.Fatalf("expected token and command in response")
	}
	if !bytes.Contains([]byte(resp.Command), []byte(resp.Token)) {
		t.Fatalf("expected command to include token")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected API token to be persisted")
	}
}
