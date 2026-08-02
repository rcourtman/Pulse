package unifiedresources

import "testing"

func TestSourceSpecificIDMatchesRegistryIngest(t *testing.T) {
	t.Parallel()

	rr := NewRegistry(nil)

	sourceID := "lab:pve-a:100"

	vm := Resource{
		Type:   ResourceTypeVM,
		Name:   "vm-100",
		Status: StatusOnline,
	}

	rr.IngestRecords(SourceProxmox, []IngestRecord{
		{SourceID: sourceID, Resource: vm},
	})

	resources := rr.ListByType(ResourceTypeVM)
	if len(resources) != 1 {
		t.Fatalf("expected 1 VM resource, got %d", len(resources))
	}

	got := resources[0].ID
	want := SourceSpecificID(ResourceTypeVM, SourceProxmox, sourceID)
	if got != want {
		t.Fatalf("resource ID mismatch: got %q want %q", got, want)
	}
}

func TestSourceSpecificIDCanonicalizesSourceIDWhitespace(t *testing.T) {
	t.Parallel()

	got := SourceSpecificID(ResourceTypeVM, SourceProxmox, "  lab:pve-a:100  ")
	want := SourceSpecificID(ResourceTypeVM, SourceProxmox, "lab:pve-a:100")
	if got != want {
		t.Fatalf("SourceSpecificID should trim source ID whitespace: got %q want %q", got, want)
	}
}

func TestResourceIdentityPinEraIDs(t *testing.T) {
	pin := ResourceIdentityPin{
		CanonicalID:  buildHashID(ResourceTypeAgent, "machine:machine-1"),
		ResourceType: ResourceTypeAgent,
		MachineID:    "machine-1",
		DMIUUID:      "dmi-1",
		ClusterName:  "homelab",
		Hostname:     "delly.lan",
	}

	got := pin.EraIDs()
	// The pin preserves the full dotted hostname, and eras cover both the
	// full name and the short name the historical derivation hashed.
	want := []string{
		buildHashID(ResourceTypeAgent, "machine:machine-1"),
		buildHashID(ResourceTypeAgent, "dmi:dmi-1"),
		buildHashID(ResourceTypeAgent, "cluster:homelab:delly.lan"),
		buildHashID(ResourceTypeAgent, "hostname:delly.lan"),
		buildHashID(ResourceTypeAgent, "cluster:homelab:delly"),
		buildHashID(ResourceTypeAgent, "hostname:delly"),
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d era IDs, got %d: %v", len(want), len(got), got)
	}
	for _, id := range want {
		found := false
		for _, eraID := range got {
			if eraID == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected era set to include %q, got %v", id, got)
		}
	}
}

func TestResourceIdentityPinPreservesDottedPrimaryHostname(t *testing.T) {
	pin := ResourceIdentityPin{
		CanonicalID:  "agent-custom",
		ResourceType: ResourceTypeAgent,
		MachineID:    "machine-1",
		Hostname:     "Cloud.Rnd-Lax1.",
	}.normalized()

	if pin.Hostname != "cloud.rnd-lax1" {
		t.Fatalf("normalized pin hostname = %q, want %q", pin.Hostname, "cloud.rnd-lax1")
	}
}

func TestResourceIdentityPinEraIDsSkipsWeakOnlyKeys(t *testing.T) {
	pin := ResourceIdentityPin{
		CanonicalID:  "agent-custom",
		ResourceType: ResourceTypeAgent,
		Hostname:     "delly",
	}
	got := pin.EraIDs()
	want := []string{"agent-custom", buildHashID(ResourceTypeAgent, "hostname:delly")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("expected era IDs %v, got %v", want, got)
	}
}

func TestMachineIdentityCanonicalIDMatchesRegistryMachineArm(t *testing.T) {
	registry := NewRegistry(NewMemoryStore())
	want := registry.canonicalIDFromIdentity(ResourceTypeAgent, ResourceIdentity{MachineID: "machine-123"})
	if got := MachineIdentityCanonicalID(ResourceTypeAgent, " machine-123 "); got != want {
		t.Fatalf("MachineIdentityCanonicalID = %q, want registry machine-arm ID %q", got, want)
	}
}

func TestParseProxmoxGuestSourceID(t *testing.T) {
	cases := []struct {
		in       string
		instance string
		node     string
		vmid     int
		ok       bool
	}{
		{"delly:pve1:100", "delly", "pve1", 100, true},
		{"cluster:with:colons:pve2:112", "cluster:with:colons", "pve2", 112, true},
		{"delly:pve1:0", "", "", 0, false},
		{"delly:100", "delly", "", 0, false},
		{"just-a-name", "", "", 0, false},
		{"vm-1a2b3c4d5e6f7788", "", "", 0, false},
		{"", "", "", 0, false},
		{"delly:pve1:abc", "", "", 0, false},
	}
	for _, tc := range cases {
		instance, node, vmid, ok := ParseProxmoxGuestSourceID(tc.in)
		if ok != tc.ok {
			t.Fatalf("ParseProxmoxGuestSourceID(%q) ok = %v, want %v", tc.in, ok, tc.ok)
		}
		if !tc.ok {
			continue
		}
		if instance != tc.instance || node != tc.node || vmid != tc.vmid {
			t.Fatalf("ParseProxmoxGuestSourceID(%q) = (%q, %q, %d), want (%q, %q, %d)", tc.in, instance, node, vmid, tc.instance, tc.node, tc.vmid)
		}
	}
}

// ProxmoxGuestCanonicalID must mint exactly what the registry's guest
// identity arm mints, so external consumers (recovery subjects, migrations)
// derive the same node-independent ID as live ingest.
func TestProxmoxGuestCanonicalIDMatchesRegistryDerivation(t *testing.T) {
	registry := NewRegistry(nil)
	want := registry.canonicalIDFromIdentity(ResourceTypeVM, ResourceIdentity{ProxmoxGuestKey: ProxmoxGuestIdentityKey("delly", 100)})
	if got := ProxmoxGuestCanonicalID(ResourceTypeVM, "delly", 100); got != want {
		t.Fatalf("ProxmoxGuestCanonicalID = %q, want registry guest-arm ID %q", got, want)
	}
}

// A VM and a container never share a VMID inside one cluster, but a VMID can
// be destroyed and recreated as the other guest type; the type prefix keeps
// those identities distinct.
func TestProxmoxGuestCanonicalIDSeparatesTypes(t *testing.T) {
	vmID := ProxmoxGuestCanonicalID(ResourceTypeVM, "delly", 200)
	ctID := ProxmoxGuestCanonicalID(ResourceTypeSystemContainer, "delly", 200)
	if vmID == ctID {
		t.Fatalf("VM and container canonical IDs must differ, both %q", vmID)
	}
}
