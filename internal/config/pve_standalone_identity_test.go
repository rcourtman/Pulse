package config

import (
	"reflect"
	"testing"
)

func TestConsolidatePVEStandaloneRequiresEndpointProof(t *testing.T) {
	for _, tc := range []struct {
		name, standaloneHost, clusterHost, endpointHost, ip, override, standaloneFP, endpointFP string
		merge                                                                                   bool
	}{
		{name: "discovered short member hostname is not identity", standaloneHost: "pmx1", clusterHost: "https://home.example:8006", endpointHost: "https://pmx1:8006"},
		{name: "discovered full member hostname is not saved authority", standaloneHost: "pmx1.example", clusterHost: "https://home.example:8006", endpointHost: "https://pmx1.example:8006"},
		{name: "only standalone fingerprint known", standaloneHost: "pmx1", endpointHost: "https://pmx1:8006", standaloneFP: "AA:AA"},
		{name: "only member fingerprint known", standaloneHost: "pmx1", endpointHost: "https://pmx1:8006", endpointFP: "AA:AA"},
		{name: "contradicting fingerprints", standaloneHost: "pmx1", endpointHost: "https://pmx1:8006", standaloneFP: "AA:AA", endpointFP: "BB:BB"},
		{name: "same pinned member", standaloneHost: "pmx1", endpointHost: "https://pmx1:8006", standaloneFP: "SHA256:AA:AA", endpointFP: "aaaa", merge: true},
		{name: "same saved cluster authority", standaloneHost: "PMX1", clusterHost: "https://pmx1:8006", endpointHost: "https://pmx1:8006", merge: true},
		{name: "literal endpoint address", standaloneHost: "10.0.0.5", endpointHost: "https://10.0.0.5:8006", merge: true},
		{name: "reported member address", standaloneHost: "10.0.0.5", endpointHost: "https://pmx1:8006", ip: "10.0.0.5", merge: true},
		{name: "operator override address", standaloneHost: "10.0.0.5", endpointHost: "https://pmx1:8006", override: "10.0.0.5", merge: true},
		{name: "same address different certificate", standaloneHost: "10.0.0.5", endpointHost: "https://pmx1:8006", ip: "10.0.0.5", standaloneFP: "aaaa", endpointFP: "bbbb"},
		{name: "same saved authority different certificate", standaloneHost: "pmx1", clusterHost: "https://pmx1:8006", endpointHost: "https://pmx1:8006", standaloneFP: "aaaa", endpointFP: "bbbb"},
		{name: "no endpoint overlap despite shared certificate", standaloneHost: "standalone", endpointHost: "https://pmx1:8006", standaloneFP: "aaaa", endpointFP: "aaaa"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []PVEInstance{
				{Name: "Home", Host: tc.clusterHost, IsCluster: true, ClusterName: "Home", TokenName: "home-token", ClusterEndpoints: []ClusterEndpoint{{NodeName: "pmx1", Host: tc.endpointHost, IP: tc.ip, IPOverride: tc.override, Fingerprint: tc.endpointFP}}},
				{Name: "Hetzner pmx1", Host: tc.standaloneHost, Fingerprint: tc.standaloneFP, TokenName: "standalone-token", GuestURL: "https://standalone.example"},
			}
			before := clonePVEInstances(input)
			got, changed := ConsolidatePVEInstances(input)
			if changed != tc.merge || len(got) != 2-boolInt(tc.merge) {
				t.Fatalf("changed=%v instances=%d, want merge=%v", changed, len(got), tc.merge)
			}
			if !reflect.DeepEqual(input, before) {
				t.Fatal("consolidation mutated input")
			}
			if !tc.merge && !reflect.DeepEqual(got, before) {
				t.Fatal("unproved match altered connection or credentials")
			}
			if tc.merge && got[0].ClusterEndpoints[0].GuestURL != "https://standalone.example" {
				t.Fatal("proved merge lost endpoint metadata")
			}
		})
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestConsolidatePVEStandaloneKeepsAmbiguousEndpointOwners(t *testing.T) {
	input := []PVEInstance{
		{Name: "Home", IsCluster: true, ClusterName: "Home", ClusterEndpoints: []ClusterEndpoint{{NodeName: "pmx1", Host: "https://pmx1:8006", Fingerprint: "aaaa"}}},
		{Name: "Other", IsCluster: true, ClusterName: "Other", ClusterEndpoints: []ClusterEndpoint{{NodeName: "pmx1", Host: "https://pmx1:8006", Fingerprint: "aaaa"}}},
		{Name: "Hetzner pmx1", Host: "pmx1", Fingerprint: "aaaa"},
	}
	got, changed := ConsolidatePVEInstances(input)
	if changed || !reflect.DeepEqual(got, input) {
		t.Fatal("ambiguous member owner retired a standalone connection")
	}
}
