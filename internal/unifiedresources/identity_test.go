package unifiedresources

import (
	"fmt"
	"reflect"
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

func TestNormalizeFullHostnamePreservesDottedNames(t *testing.T) {
	cases := map[string]string{
		"PVE1.Homelab.LAN": "pve1.homelab.lan",
		"pve1.local":       "pve1.local",
		"pve1":             "pve1",
		"pve1.":            "pve1",
		"pve1...":          "pve1",
	}
	for input, want := range cases {
		if got := NormalizeFullHostname(input); got != want {
			t.Fatalf("NormalizeFullHostname(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMachineIDMatchMerges(t *testing.T) {
	store := NewMemoryStore()
	registry := NewRegistry(store)

	resA := Resource{Type: ResourceTypeAgent, Name: "pve1", Status: StatusOnline}
	idA := ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceAgent, "agent-1", resA, idA)

	resB := Resource{Type: ResourceTypeAgent, Name: "pve1", Status: StatusOnline}
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

	resA := Resource{Type: ResourceTypeAgent, Name: "pve1", Status: StatusOnline}
	idA := ResourceIdentity{DMIUUID: "uuid-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceAgent, "agent-1", resA, idA)

	resB := Resource{Type: ResourceTypeAgent, Name: "pve1", Status: StatusOnline}
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

	resHost := Resource{Type: ResourceTypeAgent, Name: "pve1", Status: StatusOnline}
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
	primaryID := buildHashID(ResourceTypeAgent, "machine:machine-1")
	candidateID := buildHashID(ResourceTypeAgent, fmt.Sprintf("%s:%s", SourceDocker, "docker-1"))

	_ = store.AddExclusion(ResourceExclusion{ResourceA: primaryID, ResourceB: candidateID})
	registry := NewRegistry(store)

	resA := Resource{Type: ResourceTypeAgent, Name: "pve1", Status: StatusOnline}
	idA := ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}
	registry.ingest(SourceAgent, "agent-1", resA, idA)

	resB := Resource{Type: ResourceTypeAgent, Name: "pve1", Status: StatusOnline}
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

func TestIdentityMatcherRejectsUnspecifiedIPIdentity(t *testing.T) {
	matcher := NewIdentityMatcher()
	matcher.Add("host-1", ResourceIdentity{
		IPAddresses: []string{"0.0.0.0", "::"},
	})

	if candidates := matcher.FindCandidates(ResourceIdentity{IPAddresses: []string{"0.0.0.0", "::"}}); len(candidates) != 0 {
		t.Fatalf("expected unspecified IP identity to be ignored, got %+v", candidates)
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

func assertCandidateFloorMatchesReference(t testing.TB, matcher *IdentityMatcher, query ResourceIdentity) {
	t.Helper()
	full := matcher.findCandidatesReference(query)
	if got := matcher.FindCandidates(query); !reflect.DeepEqual(got, full) {
		t.Fatalf("general identity candidate/review results changed: got=%v want=%v", got, full)
	}
	for _, minimum := range []float64{0, 0.4, 0.5, 0.8, HighConfidenceThreshold, 0.99, 1.0, 1.1} {
		want := make([]MatchCandidate, 0)
		for _, match := range full {
			if match.Confidence >= minimum {
				want = append(want, match)
			}
		}
		if got := matcher.findCandidatesAtLeast(query, minimum); !reflect.DeepEqual(got, want) {
			t.Fatalf("floor %v changed identity/reason/order/review: got=%v want=%v", minimum, got, want)
		}
	}
}

func TestBroadcastIdentityFloorMatchesGeneralMatching(t *testing.T) {
	matcher := NewIdentityMatcher()
	identities := []ResourceIdentity{
		{MachineID: "machine", DMIUUID: "uuid", Hostnames: []string{"tower.local"}, IPAddresses: []string{"192.0.2.10"}, MACAddresses: []string{"00:11:22:33:44:55"}},
		{Hostnames: []string{"tower.remote"}, IPAddresses: []string{"192.0.2.10"}, MACAddresses: []string{"00-11-22-33-44-55"}},
		{Hostnames: []string{"tower.other"}, IPAddresses: []string{"192.0.2.11"}},
		{Hostnames: []string{"distinct"}, IPAddresses: []string{"127.0.0.1", "172.17.0.1"}},
		{},
	}
	for i, identity := range identities {
		matcher.Add(fmt.Sprint(i), identity)
	}
	for _, query := range append(identities, ResourceIdentity{MachineID: " machine ", DMIUUID: " uuid ", Hostnames: []string{"TOWER"}}) {
		assertCandidateFloorMatchesReference(t, matcher, query)
	}
}

func TestBroadcastIdentityFloorDoesNotAllocateDiscardedPeers(t *testing.T) {
	makeMatcher := func(count int) *IdentityMatcher {
		matcher := NewIdentityMatcher()
		for i := 0; i < count; i++ {
			matcher.Add(fmt.Sprint(i), ResourceIdentity{Hostnames: []string{"shared-host.local"}})
		}
		return matcher
	}
	query := ResourceIdentity{Hostnames: []string{"shared-host.local"}}
	small, large := makeMatcher(8), makeMatcher(1000)
	if len(large.FindCandidates(query)) != 1000 || len(large.findCandidatesAtLeast(query, HighConfidenceThreshold)) != 0 {
		t.Fatal("candidate floor changed hostname confidence")
	}
	smallAllocs := testing.AllocsPerRun(20, func() { small.findCandidatesAtLeast(query, HighConfidenceThreshold) })
	largeAllocs := testing.AllocsPerRun(20, func() { large.findCandidatesAtLeast(query, HighConfidenceThreshold) })
	if largeAllocs > smallAllocs+8 {
		t.Fatalf("discarded peers allocate with estate size: %v vs %v", smallAllocs, largeAllocs)
	}
}

func FuzzBroadcastIdentityFloorMatchesGeneralMatching(f *testing.F) {
	f.Add("machine", "uuid", "tower.local", "192.0.2.10", "00:11:22:33:44:55")
	f.Add("", "", "tower.remote", "127.0.0.1", "")
	f.Fuzz(func(t *testing.T, machine, uuid, host, ip, mac string) {
		query := ResourceIdentity{MachineID: machine, DMIUUID: uuid, Hostnames: []string{host}, IPAddresses: []string{ip}, MACAddresses: []string{mac}}
		matcher := NewIdentityMatcher()
		matcher.Add("exact", query)
		matcher.Add("hostname", ResourceIdentity{Hostnames: query.Hostnames})
		matcher.Add("ip", ResourceIdentity{IPAddresses: query.IPAddresses})
		matcher.Add("hostname-mac", ResourceIdentity{Hostnames: query.Hostnames, MACAddresses: query.MACAddresses})
		assertCandidateFloorMatchesReference(t, matcher, query)
	})
}
