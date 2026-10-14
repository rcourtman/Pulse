package config

import (
	"reflect"
	"testing"
)

// Loading multiple saved views must not collapse one view's contradictory
// rows before monitoring can refuse its unresolved trust. Cover both merge
// orders and a final repeated row matching the other view's selected pin.
func TestConsolidatePVEInstancesRetainsInternallyConflictingPins(t *testing.T) {
	for _, layout := range []string{"same-node", "same-address"} {
		for _, conflictingPin := range []string{"cc:dd", ":", " ", "sha256:aa:bb"} {
			for _, reverse := range []bool{false, true} {
				t.Run(layout+"/pin="+conflictingPin+map[bool]string{false: "/forward", true: "/reverse"}[reverse], func(t *testing.T) {
					member := ClusterEndpoint{NodeName: "member", Host: "https://member.invalid:8006", Fingerprint: "aa:bb"}
					conflicting := member
					conflicting.Fingerprint = conflictingPin
					if layout == "same-address" {
						conflicting.NodeName = "member-alias"
					}
					ambiguous := PVEInstance{Name: "Ambiguous", IsCluster: true, ClusterName: "home", Host: member.Host, Fingerprint: "aa:bb", ClusterEndpoints: []ClusterEndpoint{member, conflicting, member}}
					ordinary := PVEInstance{Name: "Ordinary", IsCluster: true, ClusterName: "home", Host: member.Host, Fingerprint: "aa:bb", ClusterEndpoints: []ClusterEndpoint{member}}
					instances := []PVEInstance{ambiguous, ordinary}
					if reverse {
						instances[0], instances[1] = instances[1], instances[0]
					}
					saved := []PVEInstance{instances[0].DeepCopy(), instances[1].DeepCopy()}
					got, changed := ConsolidatePVEInstances(instances)
					if changed || !reflect.DeepEqual(got, saved) || !reflect.DeepEqual(instances, saved) {
						t.Fatal("consolidation discarded contradictory saved trust or changed source rows")
					}
					if PVEClusterInstancesShareIdentity(instances[0], instances[1]) {
						t.Fatal("internally contradictory saved trust admitted an identity merge")
					}
					// Independently reconciled saved formats can still consolidate normally.
					ambiguous.ClusterEndpoints[1].Fingerprint = "AA:BB"
					if _, changed := ConsolidatePVEInstances([]PVEInstance{ambiguous, ordinary}); !changed {
						t.Fatal("agreeing independently reconciled views could no longer consolidate")
					}
				})
			}
		}
	}
}
