package unifiedresources

import "testing"

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
