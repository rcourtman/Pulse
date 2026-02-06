package unifiedresources

import (
	"fmt"
	"testing"
)

func TestNormalizeHostname(t *testing.T) {
	cases := map[string]string{
		"PVE1.Homelab.LAN": "pve1",
		"pve1.local":       "pve1",
		"pve1":             "pve1",
		"pve1.":            "pve1",
	}
	for input, want := range cases {
		if got := NormalizeHostname(input); got != want {
			t.Fatalf("NormalizeHostname(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMachineIDMatchMerges(t *testing.T) {
	store := NewMemoryStore()
	registry := NewRegistry(store)

	resA := Resource{Type: ResourceTypeHost, Name: "pve1", Status: StatusOnline}
	idA := ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceAgent, "agent-1", resA, idA)

	resB := Resource{Type: ResourceTypeHost, Name: "pve1", Status: StatusOnline}
	idB := ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceDocker, "docker-1", resB, idB)

	resources := registry.List()
	if len(resources) != 1 {
		t.Fatalf("expected 1 merged resource, got %d", len(resources))
	}
}

func TestDMIUUIDMatchMerges(t *testing.T) {
	store := NewMemoryStore()
	registry := NewRegistry(store)

	resA := Resource{Type: ResourceTypeHost, Name: "pve1", Status: StatusOnline}
	idA := ResourceIdentity{DMIUUID: "uuid-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceAgent, "agent-1", resA, idA)

	resB := Resource{Type: ResourceTypeHost, Name: "pve1", Status: StatusOnline}
	idB := ResourceIdentity{DMIUUID: "uuid-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceDocker, "docker-1", resB, idB)

	resources := registry.List()
	if len(resources) != 1 {
		t.Fatalf("expected 1 merged resource, got %d", len(resources))
	}
}

func TestVMInsideHostNoMerge(t *testing.T) {
	store := NewMemoryStore()
	registry := NewRegistry(store)

	resHost := Resource{Type: ResourceTypeHost, Name: "pve1", Status: StatusOnline}
	idHost := ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceAgent, "agent-1", resHost, idHost)

	resVM := Resource{Type: ResourceTypeVM, Name: "pve1", Status: StatusOnline}
	idVM := ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceProxmox, "vm-101", resVM, idVM)

	resources := registry.List()
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources (host + vm), got %d", len(resources))
	}
}

func TestExclusionPreventsMatch(t *testing.T) {
	store := NewMemoryStore()
	primaryID := buildHashID(ResourceTypeHost, "machine:machine-1")
	candidateID := buildHashID(ResourceTypeHost, fmt.Sprintf("%s:%s", SourceDocker, "docker-1"))

	_ = store.AddExclusion(ResourceExclusion{ResourceA: primaryID, ResourceB: candidateID})
	registry := NewRegistry(store)

	resA := Resource{Type: ResourceTypeHost, Name: "pve1", Status: StatusOnline}
	idA := ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceAgent, "agent-1", resA, idA)

	resB := Resource{Type: ResourceTypeHost, Name: "pve1", Status: StatusOnline}
	idB := ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceDocker, "docker-1", resB, idB)

	resources := registry.List()
	if len(resources) != 2 {
		t.Fatalf("expected exclusion to keep 2 resources, got %d", len(resources))
	}
}

func TestHostnameIPMatchRequiresReview(t *testing.T) {
	matcher := NewIdentityMatcher()
	matcher.Add("host-1", ResourceIdentity{
		Hostnames:   []string{"pve1"},
		IPAddresses: []string{"192.168.1.10"},
	})

	candidates := matcher.FindCandidates(ResourceIdentity{
		Hostnames:   []string{"pve1.homelab.lan"},
		IPAddresses: []string{"192.168.1.10"},
	})

	if len(candidates) == 0 || candidates[0].ID != "host-1" {
		t.Fatalf("expected host-1 candidate, got %+v", candidates)
	}
	if candidates[0].Confidence < 0.80 {
		t.Fatalf("expected >=0.80 confidence, got %.2f", candidates[0].Confidence)
	}
	if !candidates[0].RequiresReview {
		t.Fatalf("expected requires review")
	}
}

func TestHostnameMACMatch(t *testing.T) {
	matcher := NewIdentityMatcher()
	matcher.Add("host-1", ResourceIdentity{
		Hostnames:    []string{"pve1"},
		MACAddresses: []string{"00:11:22:33:44:55"},
	})

	candidates := matcher.FindCandidates(ResourceIdentity{
		Hostnames:    []string{"PVE1"},
		MACAddresses: []string{"00-11-22-33-44-55"},
	})

	if len(candidates) == 0 || candidates[0].ID != "host-1" {
		t.Fatalf("expected host-1 candidate, got %+v", candidates)
	}
	if candidates[0].Confidence < 0.90 {
		t.Fatalf("expected >=0.90 confidence, got %.2f", candidates[0].Confidence)
	}
}

func TestHostnameOnlyMatchRequiresReview(t *testing.T) {
	matcher := NewIdentityMatcher()
	matcher.Add("host-1", ResourceIdentity{Hostnames: []string{"pve1"}})

	candidates := matcher.FindCandidates(ResourceIdentity{Hostnames: []string{"pve1"}})
	if len(candidates) == 0 {
		t.Fatalf("expected candidate")
	}
	if !candidates[0].RequiresReview {
		t.Fatalf("expected requires review")
	}
}

func TestIPOnlyMatchRequiresReview(t *testing.T) {
	matcher := NewIdentityMatcher()
	matcher.Add("host-1", ResourceIdentity{IPAddresses: []string{"192.168.1.10"}})

	candidates := matcher.FindCandidates(ResourceIdentity{IPAddresses: []string{"192.168.1.10"}})
	if len(candidates) == 0 {
		t.Fatalf("expected candidate")
	}
	if !candidates[0].RequiresReview {
		t.Fatalf("expected requires review")
	}
}
