package configapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestMaybeRefreshClusterInfo_UpdatesMetadata(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	originalDetect := detectPVEClusterContext
	t.Cleanup(func() { detectPVEClusterContext = originalDetect })

	called := false
	detectPVEClusterContext = func(_ context.Context, clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		called = true
		return true, "unknown cluster", []config.ClusterEndpoint{
			{NodeName: "node-1", Host: "https://node-1.local:8006"},
		}
	}

	cfg.PVEInstances = []config.PVEInstance{{
		Name:       "pve-1",
		Host:       "https://pve-1.local:8006",
		TokenValue: "token",
	}}

	handler.maybeRefreshClusterInfo(context.Background(), cfg.PVEInstances[0])

	instance := cfg.PVEInstances[0]
	if !called {
		t.Fatalf("expected detectPVEClusterContext to be called")
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

	originalDetect := detectPVEClusterContext
	t.Cleanup(func() { detectPVEClusterContext = originalDetect })

	called := false
	detectPVEClusterContext = func(_ context.Context, clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		called = true
		return true, "cluster", nil
	}

	instance := config.PVEInstance{
		Name: "pve-1",
		Host: "https://pve-1.local:8006",
	}

	handler.maybeRefreshClusterInfo(context.Background(), instance)

	if called {
		t.Fatalf("expected detectPVEClusterContext to be skipped without credentials")
	}
}

func TestMaybeRefreshClusterInfo_SkipsWithinCooldown(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir()}
	handler := newTestConfigHandlers(t, cfg)

	originalDetect := detectPVEClusterContext
	t.Cleanup(func() { detectPVEClusterContext = originalDetect })

	called := false
	detectPVEClusterContext = func(_ context.Context, clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		called = true
		return true, "cluster", nil
	}

	instance := config.PVEInstance{
		Name:       "pve-1",
		Host:       "https://pve-1.local:8006",
		TokenValue: "token",
	}
	handler.lastClusterDetection[instance.Name] = time.Now()

	handler.maybeRefreshClusterInfo(context.Background(), instance)

	if called {
		t.Fatalf("expected detectPVEClusterContext to be skipped during cooldown")
	}
}

func TestGetNodesSkipsAutomaticClusterDetectionWhilePaused(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{
			{Name: "paused-pve", Host: "https://paused.invalid:8006", TokenValue: "synthetic-paused", Disabled: true},
			{Name: "saved-pve", Host: "https://saved.invalid:8006", TokenValue: "synthetic-saved", Disabled: true, IsCluster: true,
				ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "saved-member", Host: "https://saved-member.invalid:8006"}}},
			{Name: "active-pve", Host: "https://active.invalid:8006", TokenValue: "synthetic-active"},
		},
	}
	h := newTestConfigHandlers(t, cfg)
	t.Cleanup(h.Stop)
	originalDetect := detectPVEClusterContext
	t.Cleanup(func() { detectPVEClusterContext = originalDetect })
	var detected []string
	detectPVEClusterContext = func(_ context.Context, _ proxmox.ClientConfig, name string, _ []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		detected = append(detected, name)
		return true, name + "-cluster", []config.ClusterEndpoint{{NodeName: name + "-member", Host: "https://" + name + ".invalid:8006"}}
	}
	read := func() []NodeResponse {
		t.Helper()
		rec := httptest.NewRecorder()
		h.HandleGetNodes(rec, httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil))
		var nodes []NodeResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &nodes); rec.Code != http.StatusOK || err != nil || len(nodes) != 3 {
			t.Fatalf("node list: status %d, %d nodes, decode error %v", rec.Code, len(nodes), err)
		}
		return nodes
	}
	nodes := read()
	if !reflect.DeepEqual(detected, []string{"active-pve"}) {
		t.Fatalf("automatic discovery ran for %v, want only the active connection", detected)
	}
	if nodes[0].Enabled || nodes[0].IsCluster || nodes[1].Enabled || !nodes[1].IsCluster ||
		len(nodes[1].ClusterEndpoints) != 1 || nodes[1].ClusterEndpoints[0].NodeName != "saved-member" ||
		!nodes[2].Enabled || nodes[2].ClusterName != "active-pve-cluster" {
		t.Fatal("listing did not preserve paused state, saved members, and active discovery")
	}

	// A paused list read must not consume the cooldown needed for discovery
	// as soon as the connection is enabled again.
	config.Mu.Lock()
	cfg.PVEInstances[0].Disabled = false
	config.Mu.Unlock()
	nodes = read()
	if !reflect.DeepEqual(detected, []string{"active-pve", "paused-pve"}) ||
		!nodes[0].Enabled || nodes[0].ClusterName != "paused-pve-cluster" {
		t.Fatal("automatic discovery did not resume for the re-enabled connection")
	}
}

func TestMaybeRefreshClusterInfoDiscardsResultAfterPause(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), PVEInstances: []config.PVEInstance{{
		Name: "pve", Host: "https://pve.invalid:8006", TokenValue: "synthetic",
	}}}
	h := newTestConfigHandlers(t, cfg)
	t.Cleanup(h.Stop)
	originalDetect := detectPVEClusterContext
	t.Cleanup(func() { detectPVEClusterContext = originalDetect })
	detectPVEClusterContext = func(_ context.Context, _ proxmox.ClientConfig, _ string, _ []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		config.Mu.Lock()
		cfg.PVEInstances[0].Disabled = true
		config.Mu.Unlock()
		return true, "late", []config.ClusterEndpoint{{NodeName: "late-member", Host: "https://pve.invalid:8006"}}
	}
	h.maybeRefreshClusterInfo(context.Background(), cfg.PVEInstances[0].DeepCopy())
	if instance := cfg.PVEInstances[0]; !instance.Disabled || instance.IsCluster || len(instance.ClusterEndpoints) != 0 {
		t.Fatal("a passive discovery result changed the connection after it was paused")
	}
}

// Detection waits on the monitored server. A connection removed meanwhile
// hands its list slot to the next connection, which must not inherit the
// cluster members the removed server reported: its credential is sent to them.
func TestClusterDetection_ResultNotStoredOnConnectionThatTookTheSlot(t *testing.T) {
	hostileMembers := []config.ClusterEndpoint{{NodeName: "node-1", Host: "https://hostile.local:8006", Fingerprint: "hostile"}}
	setup := func(t *testing.T) (*config.Config, *ConfigHandlers) {
		cfg := &config.Config{
			DataPath: t.TempDir(),
			PVEInstances: []config.PVEInstance{
				{Name: "pve-a", Host: "https://pve-a.local:8006", TokenName: "pulse@pve!a", TokenValue: "token-a"},
				{Name: "pve-b", Host: "https://pve-b.local:8006", TokenName: "pulse@pve!b", TokenValue: "token-b", IsCluster: true, ClusterName: "b", ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "b-1", Host: "https://pve-b.local:8006"}}},
			},
		}
		handler := newTestConfigHandlers(t, cfg)

		originalDetect := detectPVEClusterContext
		t.Cleanup(func() { detectPVEClusterContext = originalDetect })
		detectPVEClusterContext = func(_ context.Context, clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
			if nodeName != "pve-a" || clientConfig.TokenValue != "token-a" {
				t.Errorf("detection ran for %q with token %q, want pve-a with its own token", nodeName, clientConfig.TokenValue)
			}
			// The administrator removes pve-a while its detection is pending.
			req := httptest.NewRequest(http.MethodDelete, "/api/config/nodes/pve:pve-a", nil)
			rec := httptest.NewRecorder()
			handler.HandleDeleteNode(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("delete during detection: status %d: %s", rec.Code, rec.Body.String())
			}
			return true, "moved", hostileMembers
		}
		return cfg, handler
	}
	check := func(t *testing.T, cfg *config.Config, handler *ConfigHandlers) {
		t.Helper()
		if len(cfg.PVEInstances) != 1 || cfg.PVEInstances[0].Name != "pve-b" {
			t.Fatalf("connections after removal = %+v, want only pve-b", cfg.PVEInstances)
		}
		nodes, err := handler.getPersistence(context.Background()).LoadNodesConfig()
		if err != nil {
			t.Fatalf("LoadNodesConfig: %v", err)
		}
		if len(nodes.PVEInstances) != 1 {
			t.Fatalf("saved connections = %+v, want only pve-b", nodes.PVEInstances)
		}
		for _, pveB := range []config.PVEInstance{cfg.PVEInstances[0], nodes.PVEInstances[0]} {
			if pveB.Name != "pve-b" || pveB.ClusterName != "b" || len(pveB.ClusterEndpoints) != 1 || pveB.ClusterEndpoints[0].Host != "https://pve-b.local:8006" {
				t.Fatalf("pve-b took the removed connection's cluster: %q %+v", pveB.ClusterName, pveB.ClusterEndpoints)
			}
		}
	}

	t.Run("list", func(t *testing.T) {
		cfg, handler := setup(t)
		nodes := handler.GetAllNodesForAPI(context.Background())
		if len(nodes) != 1 || nodes[0].Name != "pve-b" {
			t.Fatalf("listed connections = %+v, want only pve-b", nodes)
		}
		check(t, cfg, handler)
	})

	t.Run("refresh", func(t *testing.T) {
		cfg, handler := setup(t)
		req := httptest.NewRequest(http.MethodPost, "/api/config/nodes/pve:pve-a/refresh-cluster", nil)
		rec := httptest.NewRecorder()
		handler.HandleRefreshClusterNodes(rec, req)
		if rec.Code != http.StatusConflict {
			t.Fatalf("refresh of a connection removed meanwhile: status %d, want 409: %s", rec.Code, rec.Body.String())
		}
		check(t, cfg, handler)
	})
}

// A connection that is pointed at another server while its detection is
// pending keeps its name, but the result describes the server it used to be.
func TestClusterDetection_ResultNotStoredOnConnectionChangedMeanwhile(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{
			{Name: "pve-a", Host: "https://pve-a.local:8006", TokenName: "pulse@pve!a", TokenValue: "token-a"},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	originalDetect := detectPVEClusterContext
	t.Cleanup(func() { detectPVEClusterContext = originalDetect })
	detectPVEClusterContext = func(_ context.Context, clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		config.Mu.Lock()
		cfg.PVEInstances[0].Host = "https://replacement.local:8006"
		cfg.PVEInstances[0].TokenValue = "token-replacement"
		config.Mu.Unlock()
		return true, "old", []config.ClusterEndpoint{{NodeName: "node-1", Host: "https://old.local:8006"}}
	}

	handler.maybeRefreshClusterInfo(context.Background(), cfg.PVEInstances[0].DeepCopy())

	if instance := cfg.PVEInstances[0]; instance.IsCluster || instance.ClusterName != "" || len(instance.ClusterEndpoints) != 0 {
		t.Fatalf("changed connection took the previous server's cluster: %q %+v", instance.ClusterName, instance.ClusterEndpoints)
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

func TestHandleGetNodes_ClusterEndpointJSONUsesCanonicalKeys(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{
			{
				Name:        "pve-1",
				Host:        "https://pve-1.local:8006",
				TokenValue:  "token",
				IsCluster:   true,
				ClusterName: "cluster-1",
				ClusterEndpoints: []config.ClusterEndpoint{
					{
						NodeID:     "node-1",
						NodeName:   "pve-1",
						Host:       "https://pve-1.local:8006",
						GuestURL:   "https://guest.local",
						IP:         "10.0.0.1",
						IPOverride: "10.0.0.10",
						Online:     true,
					},
				},
			},
		},
	}
	handler := newTestConfigHandlers(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config/nodes", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetNodes(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "\"nodeId\":\"node-1\"") {
		t.Fatalf("expected canonical nodeId key in response body: %s", body)
	}
	if !strings.Contains(body, "\"guestURL\":\"https://guest.local\"") {
		t.Fatalf("expected canonical guestURL key in response body: %s", body)
	}
	if strings.Contains(body, "\"NodeID\"") || strings.Contains(body, "\"GuestURL\"") {
		t.Fatalf("unexpected legacy endpoint key casing in response body: %s", body)
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

	originalDetect := detectPVEClusterContext
	t.Cleanup(func() { detectPVEClusterContext = originalDetect })
	detectPVEClusterContext = func(_ context.Context, clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
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
	clusterNodes, ok := resp["clusterNodes"].([]interface{})
	if !ok || len(clusterNodes) == 0 {
		t.Fatalf("expected non-empty clusterNodes in response")
	}
	firstNode, ok := clusterNodes[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first cluster node to be an object")
	}
	if _, exists := firstNode["nodeName"]; !exists {
		t.Fatalf("expected nodeName key in clusterNodes response, got %v", firstNode)
	}
	if _, exists := firstNode["NodeName"]; exists {
		t.Fatalf("unexpected legacy NodeName key in clusterNodes response, got %v", firstNode)
	}
	if cfg.PVEInstances[0].ClusterName != "cluster-1" || !cfg.PVEInstances[0].IsCluster {
		t.Fatalf("expected instance to be updated as cluster")
	}
	if len(cfg.PVEInstances[0].ClusterEndpoints) != 2 {
		t.Fatalf("expected cluster endpoints to be updated")
	}
}

// Cluster re-detection dials the saved address outside the cluster client. It
// must carry the pin polling enforces for that endpoint, and must not dial at
// all while the saved pins for it contradict each other.
func TestClusterRedetectionUsesSavedEndpointTrust(t *testing.T) {
	const primary = "https://192.0.2.10:8006"
	for _, tc := range []struct {
		name     string
		extra    []config.ClusterEndpoint
		retry    bool // maybeRefreshClusterInfo, which only retries an unnamed cluster
		wantPins []string
		wantCode int
	}{
		{name: "refresh uses the primary endpoint pin", wantPins: []string{"aa:bb"}, wantCode: http.StatusOK},
		{name: "refresh refuses contradicting pins", extra: []config.ClusterEndpoint{{NodeName: "pve-1-alias", Host: primary, Fingerprint: "cc:dd"}}, wantCode: http.StatusBadRequest},
		{name: "validation retry uses the primary endpoint pin", retry: true, wantPins: []string{"aa:bb"}},
		{name: "validation retry skips contradicting pins", retry: true, extra: []config.ClusterEndpoint{{NodeName: "pve-1-alias", Host: primary, Fingerprint: "cc:dd"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			instance := config.PVEInstance{
				Name: "pve-1", Host: primary, TokenValue: "token", IsCluster: true, ClusterName: "cluster-1",
				ClusterEndpoints: append([]config.ClusterEndpoint{
					{NodeName: "pve-1", Host: primary, Fingerprint: "aa:bb"},
					{NodeName: "pve-2", Host: "https://192.0.2.11:8006"},
				}, tc.extra...),
			}
			if tc.retry {
				instance.ClusterName = ""
			}
			cfg := &config.Config{DataPath: t.TempDir(), PVEInstances: []config.PVEInstance{instance}}
			handler := newTestConfigHandlers(t, cfg)

			originalDetect := detectPVEClusterContext
			t.Cleanup(func() { detectPVEClusterContext = originalDetect })
			var pins []string
			detectPVEClusterContext = func(_ context.Context, clientConfig proxmox.ClientConfig, nodeName string, existing []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
				pins = append(pins, clientConfig.Fingerprint)
				return true, "cluster-1", existing
			}

			if tc.retry {
				handler.maybeRefreshClusterInfo(context.Background(), cfg.PVEInstances[0].DeepCopy())
			} else {
				rec := httptest.NewRecorder()
				handler.HandleRefreshClusterNodes(rec, httptest.NewRequest(http.MethodPost, "/api/config/nodes/pve-0/refresh-cluster", nil))
				if rec.Code != tc.wantCode {
					t.Fatalf("status = %d, want %d: %s", rec.Code, tc.wantCode, rec.Body.String())
				}
			}
			if !reflect.DeepEqual(pins, tc.wantPins) {
				t.Fatalf("cluster detection dialed with pins %q, want %q", pins, tc.wantPins)
			}
		})
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
		LogLevel:                     "debug",
		DiscoveryEnabled:             true,
		DiscoverySubnet:              "10.0.0.0/24",
		Discovery:                    config.DefaultDiscoveryConfig(),
		EnableBackupPolling:          false,
		PublicURL:                    "https://public.example",
		TemperatureMonitoringEnabled: true,
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
	if !resp.EnvOverrides["hideLocalLogin"] {
		t.Fatalf("expected hideLocalLogin env override to be true")
	}
}

func TestHandleGetMockMode(t *testing.T) {
	prevConfig := mock.GetConfig()
	prevEnabled := mock.IsMockEnabled()
	t.Cleanup(func() {
		mock.SetMockConfig(prevConfig)
		if err := mock.SetEnabled(prevEnabled); err != nil {
			t.Errorf("restore mock mode: %v", err)
		}
	})

	if err := mock.SetEnabled(false); err != nil {
		t.Fatalf("disable mock mode: %v", err)
	}
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

func TestHandleUpdateMockMode_InvokesChangeHook(t *testing.T) {
	prevEnabled := mock.IsMockEnabled()
	t.Cleanup(func() {
		if err := mock.SetEnabled(prevEnabled); err != nil {
			t.Errorf("restore mock mode: %v", err)
		}
	})

	handler := newTestConfigHandlers(t, &config.Config{DataPath: t.TempDir()})
	called := false
	gotEnabled := false
	handler.SetMockModeChangeHook(func(enabled bool) {
		called = true
		gotEnabled = enabled
	})

	req := httptest.NewRequest(http.MethodPost, "/api/config/mock", bytes.NewReader([]byte(`{"enabled":true}`)))
	rec := httptest.NewRecorder()
	handler.HandleUpdateMockMode(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Fatal("expected mock mode change hook to be invoked")
	}
	if !gotEnabled {
		t.Fatal("expected mock mode change hook to receive enabled=true")
	}
}

func TestHandleAgentInstallCommand(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "hashed-password"}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":"pve","enableCommands":true}`)
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
	if bytes.Contains([]byte(resp.Command), []byte(resp.Token)) || !strings.Contains(resp.Command, `--token-file "$token_file"`) {
		t.Fatal("credential must be entered privately, not included in copied command")
	}
	if resp.Command != BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "http://example.com:8080", Token: resp.Token, InstallType: "pve", IncludeInstallType: true, EnableCommands: true}) {
		t.Fatalf("expected command to include proxmox type")
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected API token to be persisted")
	}
	if got := cfg.APITokens[0].Metadata[agentInstallCommandPolicyIntentKey]; got != "enabled" {
		t.Fatalf("command_policy_intent metadata = %q, want enabled", got)
	}
	if !bytes.Contains([]byte(resp.Command), []byte("--enable-commands")) {
		t.Fatalf("expected command execution flag in command: %s", resp.Command)
	}
}

// The endpoint also mints host agent tokens. Its refusal used to name only
// the Proxmox types, which sent a scripted install to build a token by hand.
func TestHandleAgentInstallCommand_UnknownTypeNamesHost(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "hashed-password"}
	handler := newTestConfigHandlers(t, cfg)

	for _, body := range []string{`{}`, `{"type":"docker"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", strings.NewReader(body))
		req.Host = "example.com:8080"
		rec := httptest.NewRecorder()
		handler.HandleAgentInstallCommand(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400 (%s)", body, rec.Code, rec.Body.String())
		}
		if got := strings.TrimSpace(rec.Body.String()); got != "Type must be 'host', 'pve' or 'pbs'" {
			t.Fatalf("body %s: response = %q, want the three accepted types", body, got)
		}
		if len(cfg.APITokens) != 0 {
			t.Fatalf("body %s: refused request minted %d tokens", body, len(cfg.APITokens))
		}
	}
}

func TestHandleAgentInstallCommand_InsecureHTTPS(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "hashed-password"}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":"pve","insecure":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/config/agent-install", bytes.NewReader(body))
	req.Host = "pulse.example.com:7655"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp AgentInstallCommandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !bytes.Contains([]byte(resp.Command), []byte("curl -kfsSL")) {
		t.Fatalf("expected insecure installer download in command, got: %s", resp.Command)
	}
	if !bytes.Contains([]byte(resp.Command), []byte("--insecure")) {
		t.Fatalf("expected insecure agent connection in command, got: %s", resp.Command)
	}
}

func TestHandleAgentInstallCommand_OmitsTokenWhenAuthOptional(t *testing.T) {
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
	if resp.Token != "" {
		t.Fatalf("expected optional-auth install response to omit token, got %q", resp.Token)
	}
	if bytes.Contains([]byte(resp.Command), []byte("--token")) {
		t.Fatalf("expected optional-auth install command to omit token transport, got: %s", resp.Command)
	}
	if len(cfg.APITokens) != 0 {
		t.Fatalf("expected optional-auth install command to avoid persisting API tokens")
	}
}

func TestHandleAgentInstallCommand_NormalizesType(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "hashed-password"}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":" PBS "}`)
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
	if resp.Command != BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{BaseURL: "http://example.com:8080", Token: resp.Token, InstallType: "pbs", IncludeInstallType: true}) {
		t.Fatalf("expected normalized proxmox type in command, got: %s", resp.Command)
	}
}

func TestHandleAgentInstallCommand_CustomAgentURLOverride(t *testing.T) {
	cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "hashed-password"}
	handler := newTestConfigHandlers(t, cfg)

	body := []byte(`{"type":"pve","enableCommands":false,"insecure":true,"agentUrl":" https://pulse.internal.lan:7655/pulse/ "}`)
	req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", bytes.NewReader(body))
	req.Host = "127.0.0.1:7655"
	rec := httptest.NewRecorder()
	handler.HandleAgentInstallCommand(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp AgentInstallCommandResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	want := BuildProxmoxAgentInstallCommand(AgentInstallCommandOptions{
		BaseURL:            "https://pulse.internal.lan:7655/pulse",
		Token:              resp.Token,
		InstallType:        "pve",
		IncludeInstallType: true,
		EnableCommands:     false,
		Insecure:           true,
	})
	if resp.Command != want {
		t.Fatalf("command mismatch:\ngot:  %s\nwant: %s", resp.Command, want)
	}
	if len(cfg.APITokens) != 1 {
		t.Fatalf("expected 1 persisted token, got %d", len(cfg.APITokens))
	}
	if got := cfg.APITokens[0].Metadata[agentInstallCommandPolicyIntentKey]; got != "disabled" {
		t.Fatalf("command_policy_intent = %q, want disabled", got)
	}
	for _, scope := range cfg.APITokens[0].Scopes {
		if scope == config.ScopeAgentExec {
			t.Fatalf("monitoring-only Proxmox token unexpectedly received %q scope", config.ScopeAgentExec)
		}
	}
}

func TestHandleAgentInstallCommand_RejectsInvalidAgentURLWithoutMintingToken(t *testing.T) {
	cases := []string{
		`{"type":"pve","agentUrl":"ftp://pulse.lan:7655"}`,
		`{"type":"pve","agentUrl":"pulse.lan:7655"}`,
		`{"type":"pve","agentUrl":"https://"}`,
		`{"type":"pve","agentUrl":"https://:7655"}`,
		`{"type":"pve","agentUrl":"https://user:pass@pulse.lan:7655"}`,
		`{"type":"pve","agentUrl":"https://pulse.lan:7655?foo=bar"}`,
		`{"type":"pve","agentUrl":"https://pulse.lan:7655#frag"}`,
		`{"type":"pve","agentUrl":"https://pulse.lan:7655;id"}`,
		`{"type":"pve","agentUrl":"https://pulse.lan:7655\nrm -rf /"}`,
		`{"type":"pbs","agentUrl":"https://pulse.lan:7655 extra"}`,
	}

	for _, body := range cases {
		cfg := &config.Config{DataPath: t.TempDir(), AuthUser: "admin", AuthPass: "hashed-password"}
		handler := newTestConfigHandlers(t, cfg)

		req := httptest.NewRequest(http.MethodPost, "/api/agent-install-command", strings.NewReader(body))
		req.Host = "pulse.example.com:7655"
		rec := httptest.NewRecorder()
		handler.HandleAgentInstallCommand(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400 (%s)", body, rec.Code, rec.Body.String())
		}
		if len(cfg.APITokens) != 0 {
			t.Fatalf("body %s: invalid agentUrl minted %d tokens", body, len(cfg.APITokens))
		}
	}
}
