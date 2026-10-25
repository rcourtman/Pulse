package monitoring

import (
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Tenant handlers can mutate the configuration as soon as GetMonitor returns.
// Startup consolidation must therefore finish in New, not its detached Start
// goroutine. Checking before Start makes this regression deterministic without
// delaying registration or depending on the race detector's scheduling.
func TestMonitorNewConsolidatesConfigurationBeforePublication(t *testing.T) {
	for _, tc := range []struct {
		name        string
		secondHost  string
		secondPin   string
		wantNames   []string
		wantPersist bool
	}{
		{
			name: "overlapping cluster", secondHost: "https://pve-a.invalid:8006", secondPin: "aa",
			wantNames: []string{"primary"}, wantPersist: true,
		},
		{
			name: "same name different estate", secondHost: "https://pve-b.invalid:8006", secondPin: "aa",
			wantNames: []string{"primary", "other"},
		},
		{
			name: "contradicting trust", secondHost: "https://pve-a.invalid:8006", secondPin: "bb",
			wantNames: []string{"primary", "other"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{
				DataPath: t.TempDir(),
				PVEInstances: []config.PVEInstance{
					{
						Name: "primary", Host: "https://pve-a.invalid:8006", IsCluster: true, ClusterName: "shared-name",
						Disabled: true, Fingerprint: "aa", VerifySSL: true, TokenValue: "synthetic-primary-secret",
						ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "pve-a", Host: "https://pve-a.invalid:8006", Fingerprint: "aa"}},
					},
					{
						Name: "other", Host: tc.secondHost, IsCluster: true, ClusterName: "shared-name",
						Disabled: true, Fingerprint: tc.secondPin, VerifySSL: true, TokenValue: "synthetic-other-secret",
						ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "other-node", Host: tc.secondHost, Fingerprint: tc.secondPin}},
					},
				},
				PBSInstances: []config.PBSInstance{{Name: "backups", Disabled: true, TokenValue: "synthetic-pbs-secret"}},
				PMGInstances: []config.PMGInstance{{Name: "mail", Disabled: true, TokenValue: "synthetic-pmg-secret"}},
				APITokens: []config.APITokenRecord{{
					ID: "install-token", Hash: "synthetic-install-hash", OrgID: "tenant-a",
					Scopes: []string{config.ScopeAgentReport}, Metadata: map[string]string{"grant": "synthetic-consumed"},
				}},
			}
			before := cfg.DeepCopy()
			monitor, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(monitor.Stop)
			if monitor.GetConfig() != cfg {
				t.Fatal("startup replaced the canonical configuration pointer")
			}
			var gotNames []string
			for _, instance := range cfg.PVEInstances {
				gotNames = append(gotNames, instance.Name)
			}
			if !reflect.DeepEqual(gotNames, tc.wantNames) {
				t.Fatalf("configuration before Start = %v, want %v", gotNames, tc.wantNames)
			}
			if !reflect.DeepEqual(cfg.APITokens, before.APITokens) ||
				!reflect.DeepEqual(cfg.PBSInstances, before.PBSInstances) ||
				!reflect.DeepEqual(cfg.PMGInstances, before.PMGInstances) {
				t.Fatal("PVE startup changed token grants or another provider's inventory")
			}
			// Caches must never retain an instance retired by consolidation.
			if len(monitor.instanceInfoCache) != len(tc.wantNames)+2 {
				t.Fatalf("startup cache has %d entries, want %d", len(monitor.instanceInfoCache), len(tc.wantNames)+2)
			}
			for _, name := range tc.wantNames {
				instance := monitor.getInstanceConfig(name)
				if instance == nil || !instance.VerifySSL || !instance.Disabled {
					t.Errorf("startup lost trust/pause settings for %q", name)
				}
			}
			if !tc.wantPersist {
				return
			}
			persisted, err := config.NewConfigPersistence(cfg.DataPath).LoadNodesConfig()
			if err != nil {
				t.Fatal(err)
			}
			if persisted == nil || len(persisted.PVEInstances) != 1 || persisted.PVEInstances[0].Name != "primary" {
				t.Fatal("consolidated configuration was not persisted before publication")
			}
			if !reflect.DeepEqual(persisted.PBSInstances, before.PBSInstances) ||
				!reflect.DeepEqual(persisted.PMGInstances, before.PMGInstances) {
				t.Fatal("consolidation did not preserve other persisted providers")
			}
		})
	}
}
