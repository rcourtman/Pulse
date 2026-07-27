package monitoring

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// The TLS fingerprint propagated onto a node is per-machine identity
// evidence: a standalone node inherits the instance fingerprint, a cluster
// node inherits only its own named endpoint's fingerprint, and ambiguous or
// absent records stay unknown rather than asserting identity.
func TestPVENodeTLSFingerprint(t *testing.T) {
	standalone := &config.PVEInstance{Fingerprint: "AA:BB"}
	if got := pveNodeTLSFingerprint(standalone, "pve01"); got != "AA:BB" {
		t.Fatalf("standalone fingerprint = %q, want AA:BB", got)
	}

	cluster := &config.PVEInstance{
		IsCluster:   true,
		Fingerprint: "CC:DD",
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "pve01", Fingerprint: "AA:BB"},
			{NodeName: "pve02"},
		},
	}
	if got := pveNodeTLSFingerprint(cluster, "pve01"); got != "AA:BB" {
		t.Fatalf("cluster endpoint fingerprint = %q, want AA:BB", got)
	}
	// The instance-level fingerprint pins whichever member the connection URL
	// reaches; it must never be attributed to a different named member.
	if got := pveNodeTLSFingerprint(cluster, "pve02"); got != "" {
		t.Fatalf("member without endpoint fingerprint = %q, want unknown", got)
	}
	if got := pveNodeTLSFingerprint(cluster, "pve99"); got != "" {
		t.Fatalf("unknown member = %q, want unknown", got)
	}

	ambiguous := &config.PVEInstance{
		IsCluster: true,
		ClusterEndpoints: []config.ClusterEndpoint{
			{NodeName: "pve01", Fingerprint: "AA:BB"},
			{NodeName: "PVE01", Fingerprint: "EE:FF"},
		},
	}
	if got := pveNodeTLSFingerprint(ambiguous, "pve01"); got != "" {
		t.Fatalf("ambiguous duplicate endpoints = %q, want unknown", got)
	}

	if got := pveNodeTLSFingerprint(nil, "pve01"); got != "" {
		t.Fatalf("nil instance = %q, want unknown", got)
	}
}
