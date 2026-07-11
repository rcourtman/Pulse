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

// The end-to-end #1559 scenario: multiple standalone agents with dotted
// hostnames must mint distinct canonical resources, persist full dotted
// hostnames in their pins, and boot-window records that only know the
// hostname must resolve to the right machine instead of the first pin.
func TestStandaloneDottedHostnameAgentsStayDistinct(t *testing.T) {
	store := NewMemoryStore()

	hosts := []struct {
		hostname  string
		machineID string
	}{
		{"cloud.rnd-lax1", "machine-rnd"},
		{"cloud.gce-or1", "machine-gce"},
		{"cloud.dmi-lax1", "machine-dmi"},
	}

	agentResource := func(hostname, machineID string) Resource {
		return Resource{
			Type:   ResourceTypeAgent,
			Name:   hostname,
			Status: StatusOnline,
			Agent:  &AgentData{AgentID: machineID, Hostname: hostname, MachineID: machineID},
		}
	}

	steady := NewRegistry(store)
	idByHostname := make(map[string]string, len(hosts))
	for _, host := range hosts {
		identity := ResourceIdentity{MachineID: host.machineID, Hostnames: []string{host.hostname}}
		id := steady.ingest(SourceAgent, host.machineID, agentResource(host.hostname, host.machineID), identity)
		if id == "" {
			t.Fatalf("agent ingest for %q returned no ID", host.hostname)
		}
		for hostname, existingID := range idByHostname {
			if existingID == id {
				t.Fatalf("hosts %q and %q collapsed to canonical ID %q", hostname, host.hostname, id)
			}
		}
		idByHostname[host.hostname] = id
	}
	if got := len(steady.List()); got != len(hosts) {
		t.Fatalf("expected %d distinct resources, got %d", len(hosts), got)
	}
	steady.PersistIdentityPins()

	pins, err := store.ListResourceIdentityPins()
	if err != nil {
		t.Fatalf("ListResourceIdentityPins: %v", err)
	}
	pinnedHostnames := make(map[string]struct{}, len(pins))
	for _, pin := range pins {
		pinnedHostnames[pin.Hostname] = struct{}{}
	}
	for _, host := range hosts {
		if _, ok := pinnedHostnames[host.hostname]; !ok {
			t.Fatalf("expected pinned primary hostname %q, got %v", host.hostname, pinnedHostnames)
		}
	}

	// Boot window: runtime records that only know the hostname (no machine
	// ID yet) arrive before the agents. Each must complete from its own pin
	// and land on its own canonical ID.
	boot := NewRegistry(store)
	for _, host := range hosts {
		record := Resource{
			Type:   ResourceTypeAgent,
			Name:   host.hostname,
			Status: StatusOnline,
			Docker: &DockerData{HostSourceID: "docker:" + host.hostname, Hostname: host.hostname},
		}
		identity := ResourceIdentity{Hostnames: []string{host.hostname}}
		id := boot.ingest(SourceDocker, "docker:"+host.hostname, record, identity)
		if want := idByHostname[host.hostname]; id != want {
			t.Fatalf("boot-window record for %q resolved %q, want its own canonical ID %q", host.hostname, id, want)
		}
	}
	for _, host := range hosts {
		identity := ResourceIdentity{MachineID: host.machineID, Hostnames: []string{host.hostname}}
		id := boot.ingest(SourceAgent, host.machineID, agentResource(host.hostname, host.machineID), identity)
		if want := idByHostname[host.hostname]; id != want {
			t.Fatalf("agent ingest for %q resolved %q, want %q", host.hostname, id, want)
		}
	}
	if got := len(boot.List()); got != len(hosts) {
		t.Fatalf("expected %d merged resources after boot-window ingest, got %d", len(hosts), got)
	}
}
