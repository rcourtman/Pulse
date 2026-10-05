package unifiedresources

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

// Enumerate every better group, without an owner/form index, candidate pruning
// or early ambiguity return. This oracle intentionally does more work than the
// implementation and checks the whole target/evidence, not just group counts.
func exhaustiveTopLevelFallbackTarget(root int, group topLevelSystemResolvedGroup, groups map[int]topLevelSystemResolvedGroup) (topLevelSystemFallbackTarget, bool) {
	roots := make([]int, 0, len(groups))
	for candidate := range groups {
		if candidate != root && groups[candidate].priority < group.priority {
			roots = append(roots, candidate)
		}
	}
	sort.Ints(roots)
	targets := make(map[int]topLevelSystemGroupingEvidence)
	for _, host := range ownerOracleOrderedStrings(group.exactHosts) {
		for _, candidate := range roots {
			if _, present := groups[candidate].exactHosts[host]; present {
				if _, already := targets[candidate]; !already {
					targets[candidate] = topLevelSystemAttachmentEvidence(group, groups[candidate], "exact-host-attachment", "exact-host", host)
				}
			}
		}
	}
	for _, candidate := range roots {
		if _, already := targets[candidate]; already {
			continue
		}
		left, right := ownerOracleOrderedStrings(group.exactHosts), ownerOracleOrderedStrings(groups[candidate].exactHosts)
		matched := false
		for _, a := range left {
			for _, b := range right {
				if a != b && HostnamesEquivalent(a, b) {
					targets[candidate] = topLevelSystemAttachmentEvidence(group, groups[candidate], "hostname-form-attachment", "short-hostname", topLevelSystemHostMatchValue(a, b))
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
	}
	for _, ip := range ownerOracleOrderedStrings(group.exactIPs) {
		for _, candidate := range roots {
			if _, present := groups[candidate].exactIPs[ip]; present {
				if _, already := targets[candidate]; !already {
					targets[candidate] = topLevelSystemAttachmentEvidence(group, groups[candidate], "exact-ip-attachment", "exact-ip", ip)
				}
			}
		}
	}
	if len(targets) != 1 {
		return topLevelSystemFallbackTarget{}, false
	}
	for candidate, evidence := range targets {
		return topLevelSystemFallbackTarget{root: candidate, evidence: evidence}, true
	}
	return topLevelSystemFallbackTarget{}, false
}

func ownerOracleOrderedStrings(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func ownerTestSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func ownerTestGroup(priority int, hosts, ips []string) topLevelSystemResolvedGroup {
	resource := &Resource{ID: fmt.Sprintf("priority-%d", priority), Type: ResourceTypeAgent}
	return topLevelSystemResolvedGroup{
		priority: priority, attachByHost: len(hosts)+len(ips) > 0,
		resources: []*Resource{resource}, exactHosts: ownerTestSet(hosts...), exactIPs: ownerTestSet(ips...),
	}
}

func assertOwnerTargetMatchesOracle(t *testing.T, root int, groups map[int]topLevelSystemResolvedGroup, index topLevelSystemFallbackIndex) {
	t.Helper()
	got, gotOK := uniqueBetterTopLevelSystemTarget(root, groups[root], index, groups)
	want, wantOK := exhaustiveTopLevelFallbackTarget(root, groups[root], groups)
	if gotOK != wantOK || got != want {
		t.Fatalf("fallback differs from exhaustive oracle: root=%d groups=%#v got=%+v/%t want=%+v/%t", root, groups, got, gotOK, want, wantOK)
	}
	hosts, ips := buildTopLevelSystemFallbackOwners(groups)
	forms := buildTopLevelSystemHostFormOwners(groups)
	before, beforeOK := uniqueBetterTopLevelSystemTargetBeforeOwnerIndex(root, groups[root], hosts, ips, forms, groups)
	if beforeOK != wantOK || before != want {
		t.Fatalf("immediate-parent reference differs from oracle: got=%+v/%t want=%+v/%t", before, beforeOK, want, wantOK)
	}
}

func TestTopLevelOwnerIndexExhaustive(t *testing.T) {
	hostSets := [][]string{nil, {"alpha"}, {"alpha.lab"}, {"alpha.other"}, {"beta"}, {"alpha", "alpha.lab", "beta"}}
	ipSets := [][]string{nil, {"192.0.2.8"}}
	var variants []topLevelSystemResolvedGroup
	for priority := 0; priority < 3; priority++ {
		for _, hosts := range hostSets {
			for _, ips := range ipSets {
				variants = append(variants, ownerTestGroup(priority, hosts, ips))
			}
		}
	}
	checked := 0
	for _, source := range variants {
		for _, left := range variants {
			for _, right := range variants {
				// Sparse roots, including zero, must not be treated as slice positions
				// or as an unset selected target.
				groups := map[int]topLevelSystemResolvedGroup{0: left, 7: source, 42: right}
				assertOwnerTargetMatchesOracle(t, 7, groups, buildTopLevelSystemFallbackIndex(groups))
				checked++
			}
		}
	}
	if checked != 46656 {
		t.Fatalf("exhaustive fixture lost coverage: %d", checked)
	}
	t.Logf("checked %d complete priority/hostname/IP target-and-evidence combinations", checked)
}

func TestTopLevelOwnerIndexRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(2199))
	names := []string{"alpha", "alpha.lab", "alpha.other", " ALPHA.LAB. ", "beta", "beta.lab", "beta.other", "192.0.2.8", "", "localhost", "foo-bar", "foo-bar.example"}
	ips := []string{"192.0.2.8", "192.0.2.9", "2001:db8::8"}
	for trial := 0; trial < 256; trial++ {
		groups := make(map[int]topLevelSystemResolvedGroup)
		for i := 0; i < 16; i++ {
			var hosts, addresses []string
			for j := 0; j < rng.Intn(5); j++ {
				hosts = append(hosts, names[rng.Intn(len(names))])
			}
			for j := 0; j < rng.Intn(3); j++ {
				addresses = append(addresses, ips[rng.Intn(len(ips))])
			}
			groups[i*17] = ownerTestGroup(rng.Intn(6), hosts, addresses)
		}
		index := buildTopLevelSystemFallbackIndex(groups)
		for root := range groups {
			assertOwnerTargetMatchesOracle(t, root, groups, index)
		}
	}
}

func TestTopLevelOwnerIndexBucketsAreOrderedAndDeduplicated(t *testing.T) {
	groups := map[int]topLevelSystemResolvedGroup{
		1: ownerTestGroup(2, []string{"alpha", "alpha.lab", "alpha.other"}, []string{"192.0.2.8"}),
		9: ownerTestGroup(0, []string{"alpha", "alpha.lab"}, []string{"192.0.2.8"}),
		4: ownerTestGroup(1, []string{"alpha", "alpha.other"}, []string{"192.0.2.8"}),
		3: ownerTestGroup(1, []string{"alpha", "alpha.lab"}, []string{"192.0.2.8"}),
	}
	index := buildTopLevelSystemFallbackIndex(groups)
	for name, bucket := range map[string][]int{"host": index.hosts["alpha"], "ip": index.ips["192.0.2.8"], "form": index.hostForms["alpha"]} {
		if !reflect.DeepEqual(bucket, []int{9, 3, 4}) {
			t.Fatalf("%s owners not priority/root ordered and deduplicated: %v", name, bucket)
		}
	}
}

func TestTopLevelOwnerIndexEqualPriorityDoesNotBuildCandidateBuckets(t *testing.T) {
	groups := map[int]topLevelSystemResolvedGroup{
		0: ownerTestGroup(3, []string{"alpha", "alpha.lab"}, []string{"192.0.2.8"}),
		7: ownerTestGroup(3, []string{"alpha"}, []string{"192.0.2.8"}),
	}
	index := buildTopLevelSystemFallbackIndex(groups)
	if len(index.hosts)+len(index.ips)+len(index.hostForms) != 0 {
		t.Fatal("equal-priority roots cannot be candidates for any query")
	}
	for root := range groups {
		assertOwnerTargetMatchesOracle(t, root, groups, index)
	}
}

func TestTopLevelOwnerIndexEvidenceAndAmbiguity(t *testing.T) {
	tests := []struct {
		name                string
		source              topLevelSystemResolvedGroup
		targets             []topLevelSystemResolvedGroup
		wantOK              bool
		wantKind, wantValue string
	}{
		{"exact evidence precedes short and IP for same target", ownerTestGroup(3, []string{"alpha.lab", "alpha", "beta"}, []string{"192.0.2.8"}), []topLevelSystemResolvedGroup{ownerTestGroup(0, []string{"alpha.lab", "alpha", "beta"}, []string{"192.0.2.8"})}, true, "exact-host-attachment", "alpha"},
		{"sorted short form evidence", ownerTestGroup(3, []string{"alpha", "beta"}, nil), []topLevelSystemResolvedGroup{ownerTestGroup(0, []string{"beta.lab", "alpha.lab"}, nil)}, true, "hostname-form-attachment", "alpha.lab"},
		{"sorted IP evidence", ownerTestGroup(3, nil, []string{"192.0.2.9", "192.0.2.8"}), []topLevelSystemResolvedGroup{ownerTestGroup(0, nil, []string{"192.0.2.8", "192.0.2.9"})}, true, "exact-ip-attachment", "192.0.2.8"},
		{"exact and IP disagree", ownerTestGroup(3, []string{"alpha"}, []string{"192.0.2.8"}), []topLevelSystemResolvedGroup{ownerTestGroup(0, []string{"alpha"}, nil), ownerTestGroup(1, nil, []string{"192.0.2.8"})}, false, "", ""},
		{"different priorities do not choose best ambiguous owner", ownerTestGroup(3, []string{"alpha"}, nil), []topLevelSystemResolvedGroup{ownerTestGroup(0, []string{"alpha.lab"}, nil), ownerTestGroup(1, []string{"alpha.other"}, nil)}, false, "", ""},
		{"distinct FQDNs sharing a short name are not equivalent", ownerTestGroup(3, []string{"alpha.lab"}, nil), []topLevelSystemResolvedGroup{ownerTestGroup(0, []string{"alpha.other"}, nil)}, false, "", ""},
		{"equal priority cannot attach", ownerTestGroup(1, []string{"alpha"}, nil), []topLevelSystemResolvedGroup{ownerTestGroup(1, []string{"alpha"}, nil)}, false, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups := map[int]topLevelSystemResolvedGroup{42: tc.source}
			for i, target := range tc.targets {
				groups[i] = target
			}
			index := buildTopLevelSystemFallbackIndex(groups)
			assertOwnerTargetMatchesOracle(t, 42, groups, index)
			got, ok := uniqueBetterTopLevelSystemTarget(42, tc.source, index, groups)
			if ok != tc.wantOK || got.evidence.kind != tc.wantKind || got.evidence.value != tc.wantValue {
				t.Fatalf("wrong target/evidence: %+v/%t", got, ok)
			}
		})
	}
}

func TestTopLevelOwnerIndexResolverPreservesEveryResourceAndRecord(t *testing.T) {
	for _, fixture := range topLevelOwnerIndexFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			got, want := ResolveTopLevelSystems(fixture.resources), resolveTopLevelSystemsBeforeOwnerIndex(fixture.resources)
			if !reflect.DeepEqual(got.resourceToGroup, want.resourceToGroup) {
				t.Fatal("complete resource-to-group mapping changed")
			}
			members := 0
			for _, group := range got.groups {
				members += len(group.resources)
			}
			if members != len(fixture.resources) {
				t.Fatalf("inventory lost members: %d/%d", members, len(fixture.resources))
			}
			actual, err := json.Marshal(got.records())
			if err != nil {
				t.Fatal(err)
			}
			reference, err := json.Marshal(want.records())
			if err != nil {
				t.Fatal(err)
			}
			if string(actual) != string(reference) {
				t.Fatal("ordered records, IDs, surfaces or grouping explanations changed")
			}
			t.Logf("retained %d complete resources in %d groups with byte-equal records", members, got.Count())
		})
	}
}

type topLevelOwnerIndexFixture struct {
	name      string
	resources []Resource
}

func topLevelOwnerIndexFixtures() []topLevelOwnerIndexFixture {
	fixtures := []topLevelOwnerIndexFixture{{name: "empty"}, {name: "one", resources: []Resource{topLevelTestAgent("agent", "tower.lab", "machine", "agent")}}}
	for _, name := range []string{"spread-1220", "shared-exact-1220", "shared-short-1220", "ambiguous-1220", "different-fqdns-200", "equal-priority-proxmox-1220"} {
		count, nodes := 1220, 1
		switch name {
		case "spread-1220":
			nodes = 20
		case "ambiguous-1220":
			nodes = 2
		case "different-fqdns-200":
			count, nodes = 200, 100
		}
		resources := make([]Resource, 0, count)
		for i := 0; i < nodes; i++ {
			host := "tower.lab"
			switch name {
			case "spread-1220":
				host = fmt.Sprintf("node-%d.lab", i)
			case "different-fqdns-200":
				host = fmt.Sprintf("tower.domain-%d", i)
			}
			resources = append(resources, topLevelTestProxmoxNode(fmt.Sprintf("node-%d", i), host, fmt.Sprintf("pve-%d", i), "https://"+host+":8006"))
		}
		for i := nodes; i < count; i++ {
			host := "tower.lab"
			switch name {
			case "spread-1220":
				host = fmt.Sprintf("node-%d.lab", i%nodes)
			case "shared-short-1220":
				host = "tower"
			case "different-fqdns-200":
				host = fmt.Sprintf("tower.other-%d", i)
			}
			if name == "equal-priority-proxmox-1220" {
				resources = append(resources, Resource{ID: fmt.Sprintf("vm-%d", i), Type: ResourceTypeVM, Name: fmt.Sprintf("workload-%d", i), Proxmox: &ProxmoxData{NodeName: host, VMID: i + 100}})
			} else {
				resources = append(resources, topLevelTestAgent(fmt.Sprintf("agent-%d", i), host, fmt.Sprintf("machine-%d", i), fmt.Sprintf("agent-id-%d", i)))
			}
		}
		fixtures = append(fixtures, topLevelOwnerIndexFixture{name: name, resources: resources})
	}
	return fixtures
}

// Test-only bridges let the external package exercise actual provider adapters
// and mock graphs without importing mock (which itself uses this package).
func CheckTopLevelOwnerIndexResourcesForTest(t testing.TB, resources []Resource) {
	t.Helper()
	got, want := ResolveTopLevelSystems(resources), resolveTopLevelSystemsBeforeOwnerIndex(resources)
	if !reflect.DeepEqual(got.resourceToGroup, want.resourceToGroup) {
		t.Fatal("connected resource-to-group mapping changed")
	}
	actual, err := json.Marshal(got.records())
	if err != nil {
		t.Fatal(err)
	}
	reference, err := json.Marshal(want.records())
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(reference) {
		t.Fatal("connected ordered records or grouping explanation changed")
	}
	members := 0
	for _, group := range got.groups {
		members += len(group.resources)
	}
	if members != len(resources) {
		t.Fatalf("connected inventory lost members: %d/%d", members, len(resources))
	}
	t.Logf("connected grouping retains %d resources in %d groups with byte-equal records", members, got.Count())
}

func RunTopLevelOwnerIndexBenchmarkForTest(b *testing.B, resources []Resource) {
	for _, method := range []struct {
		name    string
		resolve func([]Resource) TopLevelSystemResolver
	}{{"before", resolveTopLevelSystemsBeforeOwnerIndex}, {"indexed", ResolveTopLevelSystems}} {
		b.Run(method.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkTopLevelOwnerResolver = method.resolve(resources)
			}
		})
	}
}
