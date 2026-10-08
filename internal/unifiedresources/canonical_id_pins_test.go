package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// The #1559 shape: several standalone agents whose hostnames share a short
// name (cloud.rnd-lax1, cloud.gce-or1, cloud.dmi-lax1). The pin index must
// resolve each full hostname to its own machine and refuse the ambiguous
// short name instead of handing one machine's pin to another.
func TestIdentityPinIndexKeepsDottedHostnamesDistinct(t *testing.T) {
	pins := []ResourceIdentityPin{
		{CanonicalID: "agent-rnd", ResourceType: ResourceTypeAgent, MachineID: "machine-rnd", Hostname: "cloud.rnd-lax1"},
		{CanonicalID: "agent-gce", ResourceType: ResourceTypeAgent, MachineID: "machine-gce", Hostname: "cloud.gce-or1"},
		{CanonicalID: "agent-dmi", ResourceType: ResourceTypeAgent, MachineID: "machine-dmi", Hostname: "cloud.dmi-lax1"},
	}
	index := newIdentityPinIndex(pins)

	for _, pin := range pins {
		got, ok := index.find(ResourceIdentity{Hostnames: []string{pin.Hostname}})
		if !ok {
			t.Fatalf("expected a pin match for %q", pin.Hostname)
		}
		if got.CanonicalID != pin.CanonicalID {
			t.Fatalf("hostname %q resolved pin %q, want %q", pin.Hostname, got.CanonicalID, pin.CanonicalID)
		}
	}

	if pin, ok := index.find(ResourceIdentity{Hostnames: []string{"cloud"}}); ok {
		t.Fatalf("ambiguous short hostname must not resolve a pin, got %q", pin.CanonicalID)
	}
	if pin, ok := index.find(ResourceIdentity{Hostnames: []string{"cloud.aws-fra1"}}); ok {
		t.Fatalf("unknown dotted sibling must not borrow another machine's pin, got %q", pin.CanonicalID)
	}
}

func TestIdentityPinIndexShortAndFQDNStayEquivalent(t *testing.T) {
	index := newIdentityPinIndex([]ResourceIdentityPin{
		{CanonicalID: "agent-web", ResourceType: ResourceTypeAgent, MachineID: "machine-web", Hostname: "web01.lan"},
	})

	if pin, ok := index.find(ResourceIdentity{Hostnames: []string{"web01"}}); !ok || pin.CanonicalID != "agent-web" {
		t.Fatalf("short hostname should resolve its own FQDN pin, got ok=%v pin=%q", ok, pin.CanonicalID)
	}
	if pin, ok := index.find(ResourceIdentity{Hostnames: []string{"web01.example.com"}}); ok {
		t.Fatalf("a different FQDN sharing the short name must not match, got %q", pin.CanonicalID)
	}
	if pin, ok := index.find(ResourceIdentity{Hostnames: []string{"web01"}, MachineID: "machine-other"}); ok {
		t.Fatalf("a contradicting machine ID must refuse the pin, got %q", pin.CanonicalID)
	}
}

func TestIdentityPinIndexClusterLookupsUseFullHostname(t *testing.T) {
	index := newIdentityPinIndex([]ResourceIdentityPin{
		{CanonicalID: "agent-delly", ResourceType: ResourceTypeAgent, MachineID: "machine-delly", ClusterName: "homelab", Hostname: "delly.lan"},
	})

	// A PVE boot-window node record only knows cluster + short node name.
	if pin, ok := index.find(ResourceIdentity{ClusterName: "homelab", Hostnames: []string{"delly"}}); !ok || pin.CanonicalID != "agent-delly" {
		t.Fatalf("cluster + short hostname should resolve the FQDN pin, got ok=%v pin=%q", ok, pin.CanonicalID)
	}
	if pin, ok := index.find(ResourceIdentity{ClusterName: "homelab", Hostnames: []string{"delly.other"}}); ok {
		t.Fatalf("a different FQDN in the same cluster must not match, got %q", pin.CanonicalID)
	}
}

// Rows persisted before the fix hold the collapsed short hostname. A single
// legacy pin must keep matching its own host's full hostname until the next
// persist heals the row.
func TestIdentityPinIndexLegacyCollapsedPinStillMatches(t *testing.T) {
	index := newIdentityPinIndex([]ResourceIdentityPin{
		{CanonicalID: "agent-rnd", ResourceType: ResourceTypeAgent, MachineID: "machine-rnd", Hostname: "cloud"},
	})

	if pin, ok := index.find(ResourceIdentity{Hostnames: []string{"cloud.rnd-lax1"}}); !ok || pin.CanonicalID != "agent-rnd" {
		t.Fatalf("legacy collapsed pin should match its host's full hostname, got ok=%v pin=%q", ok, pin.CanonicalID)
	}
}

// A Proxmox node linked to a standalone agent: while linked, the merged
// resource's identity pin recorded the node's endpoint hostname with the
// agent's machine key. After unlink the node completed that key from the pin,
// minted the agent's machine-derived ID and folded into it by ID before any
// exclusion check ran, so the pair stayed merged. Each step is judged after
// three monitor rebuilds, since pins persist on one rebuild and steer the next.
func TestUnlinkSplitsHostPairWhosePinTookTheOtherSidesKey(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name         string
		nodeEndpoint string
		agentPrimary bool
	}{
		{"agent-primary", "https://10.0.0.5:8006", true},
		{"node-primary", "https://10.0.0.5:8006", false},
		{"agent-primary-fqdn", "https://pve1.example.lan:8006", true},
		{"node-primary-fqdn", "https://pve1.example.lan:8006", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewSQLiteResourceStore(t.TempDir(), "default")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			snapshot := models.StateSnapshot{
				LastUpdate: now,
				Nodes:      []models.Node{{ID: "lab-pve1", Name: "pve1", Instance: "lab", Host: tc.nodeEndpoint, Status: "online", LastSeen: now}},
				Hosts:      []models.Host{{ID: "host-box", Hostname: "box-agent", MachineID: "fedcba9876543210", Status: "online", LastSeen: now}},
			}
			adapter := NewMonitorAdapter(NewRegistry(store))
			rebuild := func() []Resource {
				for i := 0; i < 3; i++ {
					adapter.PopulateFromSnapshot(snapshot)
				}
				return adapter.GetAll()
			}
			var nodeID, agentID string
			for _, resource := range rebuild() {
				switch {
				case resource.Proxmox != nil && resource.Agent == nil:
					nodeID = resource.ID
				case resource.Agent != nil && resource.Proxmox == nil:
					agentID = resource.ID
				}
			}
			if nodeID == "" || agentID == "" {
				t.Fatalf("fixture did not produce a separate node and agent: %+v", adapter.GetAll())
			}
			primaryID, otherID := nodeID, agentID
			if tc.agentPrimary {
				primaryID, otherID = agentID, nodeID
			}
			if err := store.AddLink(ResourceLink{ResourceA: primaryID, ResourceB: otherID, PrimaryID: primaryID, CreatedAt: time.Now().UTC()}); err != nil {
				t.Fatal(err)
			}
			linked := rebuild()
			if len(linked) != 1 {
				t.Fatalf("link did not merge the pair: %d resources", len(linked))
			}
			// Unlink as the API is called: on the merged resource, naming the
			// side that disappeared into it.
			mergedID := linked[0].ID
			splitID := agentID
			if mergedID == agentID {
				splitID = nodeID
			}
			if err := store.AddExclusion(ResourceExclusion{ResourceA: mergedID, ResourceB: splitID, CreatedAt: time.Now().UTC()}); err != nil {
				t.Fatal(err)
			}
			unlinked := rebuild()
			if len(unlinked) != 2 {
				t.Fatalf("unlink left %d resources, want the node and the agent apart: %+v", len(unlinked), unlinked)
			}
			for _, resource := range unlinked {
				if resource.Proxmox != nil && resource.Agent != nil {
					t.Fatalf("resource %s still carries both facets after unlink", resource.ID)
				}
			}
		})
	}
}
