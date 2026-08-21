package configapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func displayNameTestInstance() config.PVEInstance {
	return config.PVEInstance{
		Name:      "cluster",
		IsCluster: true,
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "pve1", NodeIdentity: "cluster-pve1"},
			{NodeName: "pve2", NodeIdentity: "cluster-pve2"},
		},
		ClusterNodeIdentities: []config.PVEClusterNodeIdentity{
			{ID: "cluster-pve1", NativeName: "pve1"},
			{ID: "cluster-pve2", NativeName: "pve2"},
		},
	}
}

func TestApplyClusterNodeDisplayNameOverridesSupportsUnicodeDuplicatesAndClearing(t *testing.T) {
	instance := displayNameTestInstance()
	updated, err := applyClusterNodeDisplayNameOverrides(instance, []ClusterNodeDisplayNameOverrideRequest{
		{NodeIdentity: "cluster-pve1", DisplayName: "  Rendu \u00c9tage  "},
		{NodeIdentity: "cluster-pve2", DisplayName: "Rendu \u00c9tage"},
	})
	if err != nil {
		t.Fatalf("apply display names: %v", err)
	}
	if updated.ClusterNodeIdentities[0].DisplayName != "Rendu \u00c9tage" ||
		updated.ClusterNodeIdentities[1].DisplayName != "Rendu \u00c9tage" {
		t.Fatalf("cosmetic duplicate Unicode labels should be allowed: %+v", updated.ClusterNodeIdentities)
	}
	if instance.ClusterNodeIdentities[0].DisplayName != "" {
		t.Fatal("display-name update mutated the caller's config slice")
	}

	cleared, err := applyClusterNodeDisplayNameOverrides(updated, []ClusterNodeDisplayNameOverrideRequest{{
		NodeIdentity: "cluster-pve1",
		DisplayName:  "",
	}})
	if err != nil || cleared.ClusterNodeIdentities[0].DisplayName != "" {
		t.Fatalf("clear override failed: identities=%+v err=%v", cleared.ClusterNodeIdentities, err)
	}
}

func TestApplyClusterNodeDisplayNameOverridesRejectsStaleAndAmbiguousRequests(t *testing.T) {
	instance := displayNameTestInstance()
	cases := []struct {
		name      string
		overrides []ClusterNodeDisplayNameOverrideRequest
		errorText string
	}{
		{
			name:      "unknown identity",
			overrides: []ClusterNodeDisplayNameOverrideRequest{{NodeIdentity: "missing", DisplayName: "Name"}},
			errorText: "unknown cluster node identity",
		},
		{
			name: "duplicate request",
			overrides: []ClusterNodeDisplayNameOverrideRequest{
				{NodeIdentity: "cluster-pve1", DisplayName: "A"},
				{NodeIdentity: "cluster-pve1", DisplayName: "B"},
			},
			errorText: "provided more than once",
		},
		{
			name:      "control character",
			overrides: []ClusterNodeDisplayNameOverrideRequest{{NodeIdentity: "cluster-pve1", DisplayName: "bad\nname"}},
			errorText: "control characters",
		},
		{
			name: "too long",
			overrides: []ClusterNodeDisplayNameOverrideRequest{{
				NodeIdentity: "cluster-pve1",
				DisplayName:  strings.Repeat("\u754c", 129),
			}},
			errorText: "at most 128",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := applyClusterNodeDisplayNameOverrides(instance, tc.overrides)
			if err == nil || !strings.Contains(err.Error(), tc.errorText) {
				t.Fatalf("error = %v, want containing %q", err, tc.errorText)
			}
		})
	}
}

func TestClusterEndpointResponseSeparatesNativeAndDisplayIdentity(t *testing.T) {
	instance := displayNameTestInstance()
	instance.ClusterNodeIdentities[0].NativeNodeID = 11
	instance.ClusterNodeIdentities[0].NativeAliases = []string{"pve-old"}
	instance.ClusterNodeIdentities[0].DisplayName = "Compute A"
	instance.ClusterEndpoints[0].NativeNodeID = 11

	response := toClusterEndpointResponse(instance.ClusterEndpoints[0], instance.ClusterNodeIdentities)
	if response.NodeIdentity != "cluster-pve1" || response.NativeNodeID != 11 ||
		response.NativeName != "pve1" || response.DisplayName != "Compute A" {
		t.Fatalf("unexpected endpoint response: %+v", response)
	}
	if len(response.NativeAliases) != 1 || response.NativeAliases[0] != "pve-old" {
		t.Fatalf("native diagnostic aliases missing from endpoint response: %+v", response)
	}
}

func TestHandleUpdateNodePersistsDisplayNameWithoutChangingConnectionAuthority(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{{
			Name: "cluster", Host: "https://configured.example.test:8006",
			TokenName: "root@pam!pulse", TokenValue: "secret",
			IsCluster: true, ClusterName: "production",
			ClusterEndpoints: []config.ClusterEndpoint{{
				NodeID: "node/pve1", NodeIdentity: "production-pve1", NativeNodeID: 1,
				NodeName: "pve1", Host: "https://pve1:8006", IP: "10.0.0.1", IPOverride: "10.20.0.1",
			}},
			ClusterNodeIdentities: []config.PVEClusterNodeIdentity{{
				ID: "production-pve1", NativeNodeID: 1, NativeName: "pve1",
			}},
		}},
	}
	handler := newTestConfigHandlers(t, cfg)
	body, _ := json.Marshal(map[string]any{
		"clusterNodeDisplayNameOverrides": []map[string]string{{
			"nodeIdentity": "production-pve1",
			"displayName":  "Render East",
		}},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pve-0", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateNode(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", rec.Code, rec.Body.String())
	}

	instance := cfg.PVEInstances[0]
	if instance.ClusterNodeIdentities[0].DisplayName != "Render East" {
		t.Fatalf("display name was not persisted in config: %+v", instance.ClusterNodeIdentities)
	}
	if instance.Host != "https://configured.example.test:8006" ||
		instance.TokenValue != "secret" ||
		instance.ClusterEndpoints[0].Host != "https://pve1:8006" ||
		instance.ClusterEndpoints[0].IPOverride != "10.20.0.1" {
		t.Fatalf("presentation update changed routing or credentials: %+v", instance)
	}

	reloaded, err := handler.defaultPersistence.LoadNodesConfig()
	if err != nil {
		t.Fatalf("reload persisted nodes: %v", err)
	}
	if got := reloaded.PVEInstances[0].ClusterNodeIdentities[0].DisplayName; got != "Render East" {
		t.Fatalf("reloaded display name = %q, want Render East", got)
	}
}

func TestHandleUpdateNodeRejectDoesNotMutateLegacyIdentityState(t *testing.T) {
	cfg := &config.Config{
		DataPath: t.TempDir(),
		PVEInstances: []config.PVEInstance{{
			Name: "cluster", Host: "https://configured.example.test:8006",
			TokenName: "root@pam!pulse", TokenValue: "secret",
			IsCluster: true, ClusterName: "production",
			ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "pve1"}},
		}},
	}
	handler := newTestConfigHandlers(t, cfg)
	body, _ := json.Marshal(map[string]any{
		"clusterNodeDisplayNameOverrides": []map[string]string{{
			"nodeIdentity": "stale-browser-identity",
			"displayName":  "Wrong node",
		}},
	})
	req := httptest.NewRequest(http.MethodPut, "/api/config/nodes/pve-0", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateNode(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("update status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if cfg.PVEInstances[0].ClusterEndpoints[0].NodeIdentity != "" ||
		len(cfg.PVEInstances[0].ClusterNodeIdentities) != 0 {
		t.Fatalf("rejected update mutated live legacy config: %+v", cfg.PVEInstances[0])
	}
}

func TestMergeDiscoveredClusterEndpointConfigurationUsesStrongIdentityOnly(t *testing.T) {
	existing := []config.ClusterEndpoint{
		{NodeIdentity: "identity-a", NativeNodeID: 1, NodeName: "old-a", IP: "10.0.0.1", IPOverride: "route-a"},
		{NodeIdentity: "identity-b", NativeNodeID: 2, NodeName: "old-b", IP: "10.0.0.2", IPOverride: "route-b"},
	}
	renamed := mergeDiscoveredClusterEndpointConfiguration(existing, []config.ClusterEndpoint{{
		NativeNodeID: 1, NodeName: "new-a", IP: "10.20.0.1",
	}})
	if renamed[0].NodeIdentity != "identity-a" || renamed[0].IPOverride != "route-a" {
		t.Fatalf("numeric identity did not preserve user metadata through rename/re-IP: %+v", renamed[0])
	}

	ambiguousLegacy := mergeDiscoveredClusterEndpointConfiguration([]config.ClusterEndpoint{
		{NodeIdentity: "identity-a", NodeName: "old-a", IP: "10.0.0.1"},
		{NodeIdentity: "identity-b", NodeName: "old-b", IP: "10.0.0.1"},
	}, []config.ClusterEndpoint{{
		NodeName: "new-name", IP: "10.0.0.1",
	}})
	if ambiguousLegacy[0].NodeIdentity != "" {
		t.Fatalf("ambiguous legacy address evidence must not corrupt canonical identity: %+v", ambiguousLegacy[0])
	}
}

func TestApplyClusterNodeDisplayNameOverridesConcurrentCopies(t *testing.T) {
	instance := displayNameTestInstance()
	var wait sync.WaitGroup
	for worker := 0; worker < 32; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			updated, err := applyClusterNodeDisplayNameOverrides(instance, []ClusterNodeDisplayNameOverrideRequest{{
				NodeIdentity: "cluster-pve1",
				DisplayName:  "Render East",
			}})
			if err != nil || updated.ClusterNodeIdentities[0].DisplayName != "Render East" {
				t.Errorf("concurrent immutable update failed: identities=%+v err=%v", updated.ClusterNodeIdentities, err)
			}
		}()
	}
	wait.Wait()
	if instance.ClusterNodeIdentities[0].DisplayName != "" {
		t.Fatal("concurrent presentation updates mutated shared tenant config input")
	}
}
