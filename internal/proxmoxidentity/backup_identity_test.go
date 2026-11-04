package proxmoxidentity

import "testing"

func TestNamespaceMatchesLocation(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		location  string
		expected  bool
	}{
		{"exact match", "pve", "pve", true},
		{"exact match with numbers", "pve1", "pve1", true},
		{"case insensitive", "PVE1", "pve1", true},
		{"namespace suffix of location", "nat", "pve-nat", true},
		{"location suffix of namespace", "backupspve", "pve", true},
		{"prefix is not enough", "pvebackups", "pve", false},
		{"substring is not enough", "production", "my-production-server", false},
		{"empty namespace", "", "pve1", false},
		{"empty location", "pve1", "", false},
		{"different names", "pve1", "pve2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NamespaceMatchesLocation(tt.namespace, tt.location)
			if got != tt.expected {
				t.Fatalf("NamespaceMatchesLocation(%q, %q) = %v, want %v",
					tt.namespace, tt.location, got, tt.expected)
			}
		})
	}
}

func TestNamespaceLocationScorePrefersGuestNodeOverClusterEntrypoint(t *testing.T) {
	if got := NamespaceLocationScore("minipc", "delly", "minipc"); got != NamespaceNodeMatch {
		t.Fatalf("NamespaceLocationScore(minipc, delly, minipc) = %d, want node match", got)
	}
	if got := NamespaceLocationScore("delly", "delly", "minipc"); got != NamespaceWeakInstanceMatch {
		t.Fatalf("NamespaceLocationScore(delly, delly, minipc) = %d, want weak instance match", got)
	}
	if got := NamespaceLocationScore("delly", "delly", "delly"); got != NamespaceNodeMatch {
		t.Fatalf("NamespaceLocationScore(delly, delly, delly) = %d, want node match", got)
	}
}

func TestBackupCommentMatchesGuestName(t *testing.T) {
	if !BackupCommentMatchesGuestName("debian-go", "112", "debian-go") {
		t.Fatal("expected exact guest-name backup comment to match")
	}
	if !BackupCommentMatchesGuestName("debian-go, 112", "112", "debian-go") {
		t.Fatal("expected notes-template style guest-name backup comment to match")
	}
	if BackupCommentMatchesGuestName("112", "112", "debian-go") {
		t.Fatal("numeric-only backup comment should not match a guest name")
	}
}

func TestBackupGuestMatchScoreRanksNodeMatchAboveClusterEntrypoint(t *testing.T) {
	weakScore := BackupGuestMatchScore("delly", "112", "112", "debian-go", "delly", "minipc")
	if weakScore <= 0 {
		t.Fatalf("weak instance match score = %d, want positive", weakScore)
	}
	commentScore := BackupGuestMatchScore("delly", "debian-go", "112", "debian-go", "delly", "minipc")
	if commentScore <= weakScore {
		t.Fatalf("weak instance+comment score = %d, should outrank weak instance-only score %d", commentScore, weakScore)
	}
	if got := BackupGuestMatchScore("minipc", "112", "112", "debian-go", "delly", "minipc"); got <= commentScore {
		t.Fatalf("node namespace score = %d, should outrank weak instance+comment match", got)
	}
}

// Two installations reuse node name pmx1, and one connection's name ends in it
// (#2681). A namespace named after that connection says which installation wrote
// the snapshot. Anything both guests fit equally stays a tie (#1639, #2292).
func TestBackupGuestMatchScoreConnectionNamespaceSettlesSharedNodeName(t *testing.T) {
	const hetzner, home, node = "Hetzner pmx1", "Home", "pmx1"
	score := func(namespace, instance string) int {
		return BackupGuestMatchScore(namespace, "100", "100", "docker1", instance, node)
	}

	for _, tc := range []struct{ namespace, owner, other string }{
		{"Hetzner pmx1", hetzner, home},
		{"hetzner-pmx1", hetzner, home},
		{"Home", home, hetzner},
	} {
		t.Run(tc.namespace, func(t *testing.T) {
			owner, other := score(tc.namespace, tc.owner), score(tc.namespace, tc.other)
			if owner <= other {
				t.Fatalf("namespace %q scores %s = %d, %s = %d, want the named connection ahead",
					tc.namespace, tc.owner, owner, tc.other, other)
			}
		})
	}

	for _, tc := range []struct {
		name, namespace string
		positive        bool
	}{
		{"the shared node name", "pmx1", true},
		{"a longer label ending in the node name", "site-pmx1", true},
		{"a parent path ahead of the connection name", "prod/hetzner-pmx1", true},
		{"only part of the connection name", "hetzner", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			first, second := score(tc.namespace, hetzner), score(tc.namespace, home)
			if first != second {
				t.Fatalf("namespace %q scores %s = %d, %s = %d, want a tie", tc.namespace, hetzner, first, home, second)
			}
			if (first > 0) != tc.positive {
				t.Fatalf("namespace %q scores %d, want positive = %v", tc.namespace, first, tc.positive)
			}
		})
	}

	// A guest-name comment is weaker than the namespace naming a connection.
	if got, other := score("hetzner-pmx1", hetzner),
		BackupGuestMatchScore("hetzner-pmx1", "docker1", "100", "docker1", home, node); got <= other {
		t.Fatalf("connection namespace = %d, should outrank node suffix+comment = %d", got, other)
	}
}

func TestNamespaceLocationScoreRanksExactNamesAboveSuffixes(t *testing.T) {
	for _, tc := range []struct {
		name, namespace                                    string
		strongInstance, strongNode, weakInstance, weakNode string
	}{
		{"exact connection over node suffix", "hetzner-pmx1", "Hetzner pmx1", "pmx1", "Home", "pmx1"},
		{"exact connection on another node over node suffix", "hetzner-pmx1", "Hetzner pmx1", "node7", "Home", "pmx1"},
		{"exact connection with no node over node suffix", "hetzner-pmx1", "Hetzner pmx1", "", "Home", "pmx1"},
		{"exact node over exact connection", "delly", "other", "delly", "delly", "minipc"},
		{"exact node over exact connection that also suffix-matches its node", "pve", "Home", "pve", "pve", "site-pve"},
		{"exact node over node suffix", "nat", "other", "nat", "home", "pve-nat"},
		{"exact connection over connection suffix", "nat", "nat", "pve1", "pve-nat", "pve2"},
		{"node suffix over connection suffix", "site-pve", "other", "pve", "pve", "minipc"},
		{"connection suffix over nothing", "nat", "pve-nat", "pve1", "home", "pve2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			strong := NamespaceLocationScore(tc.namespace, tc.strongInstance, tc.strongNode)
			weak := NamespaceLocationScore(tc.namespace, tc.weakInstance, tc.weakNode)
			if strong <= weak {
				t.Fatalf("namespace %q scores %s/%s = %d, %s/%s = %d, want the first ahead",
					tc.namespace, tc.strongInstance, tc.strongNode, strong, tc.weakInstance, tc.weakNode, weak)
			}
		})
	}
}

func TestBackupGuestMatchScoreNodeVMIDNameComment(t *testing.T) {
	const comment = "pdm21.21008.pulse"
	if got := BackupGuestMatchScore("", comment, "21008", "pulse", "cluster-a", "pdm21"); got <= 0 {
		t.Fatalf("matching node.VMID.name comment score = %d, want positive", got)
	}
	for _, tc := range []struct {
		name, vmid, guestName, node string
	}{
		{"other node", "21008", "pulse", "pdm22"},
		{"other VMID", "21009", "pulse", "pdm21"},
		{"other name", "21008", "other", "pdm21"},
		{"empty node", "21008", "pulse", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := BackupGuestMatchScore("", comment, tc.vmid, tc.guestName, "cluster-a", tc.node); got != 0 {
				t.Fatalf("mismatched node.VMID.name comment score = %d, want zero", got)
			}
		})
	}
	if got := BackupGuestMatchScore("", "other.pdm21.21008.pulse", "21008", "pulse", "cluster-a", "pdm21"); got != 0 {
		t.Fatalf("prefixed comment score = %d, want zero", got)
	}
}

func TestLocationLabelsEqualRequiresExactNormalizedMatch(t *testing.T) {
	if !LocationLabelsEqual("pve-nat", "PVE Nat") {
		t.Fatal("normalized identical labels should match")
	}
	// NamespaceMatchesLocation would suffix-match these; a connection label
	// must not (#1639).
	if LocationLabelsEqual("nat", "pve-nat") {
		t.Fatal("suffix relation between distinct labels must not match")
	}
	if LocationLabelsEqual("", "") {
		t.Fatal("empty labels must not match")
	}
}

func TestPBSSourceLearnerResolvesOwnerBeforeWeakerComponents(t *testing.T) {
	l := NewPBSSourceLearner()
	l.RegisterCandidate("pbs-main", "cluster-a")
	l.RegisterCandidate("pbs-main", "cluster-b")
	l.Observe("pbs-main", "backups", "cluster-a@pbs!token", "cluster-a")
	l.Observe("pbs-main", "backups", "cluster-b@pbs!token", "cluster-b")

	// Datastore and PBS instance are shared, but each owner token uniquely
	// identifies its cluster.
	if inst, ok := l.Resolve("pbs-main", "backups", "cluster-a@pbs!token"); !ok || inst != "cluster-a" {
		t.Fatalf("owner resolution = %q,%v, want cluster-a,true", inst, ok)
	}
	if inst, ok := l.Resolve("pbs-main", "backups", "CLUSTER-B@pbs!token"); !ok || inst != "cluster-b" {
		t.Fatalf("owner resolution should be case-insensitive, got %q,%v", inst, ok)
	}
}

func TestPBSSourceLearnerSharedSourceIsNotDecisive(t *testing.T) {
	l := NewPBSSourceLearner()
	l.RegisterCandidate("pbs-main", "cluster-a")
	l.RegisterCandidate("pbs-main", "cluster-b")
	l.Observe("pbs-main", "backups", "shared@pbs!token", "cluster-a")
	l.Observe("pbs-main", "backups", "shared@pbs!token", "cluster-b")

	if inst, ok := l.Resolve("pbs-main", "backups", "shared@pbs!token"); ok {
		t.Fatalf("shared source resolved to %q, want inconclusive", inst)
	}
}

func TestPBSSourceLearnerUnfamiliarComponentStopsResolution(t *testing.T) {
	l := NewPBSSourceLearner()
	l.RegisterCandidate("pbs-main", "cluster-a")
	l.RegisterCandidate("pbs-main", "cluster-b")
	l.Observe("pbs-main", "backups", "cluster-a@pbs!token", "cluster-a")
	l.Observe("pbs-main", "store-b", "cluster-b@pbs!token", "cluster-b")

	// The datastore alone would resolve to cluster-a, but an owner token
	// that was never observed means the backup may come from a cluster we
	// have no evidence for — weaker components must not be consulted.
	if inst, ok := l.Resolve("pbs-main", "backups", "mystery@pbs!token"); ok {
		t.Fatalf("unfamiliar owner resolved to %q, want inconclusive", inst)
	}
	// Without an owner on the backup, the datastore evidence applies.
	if inst, ok := l.Resolve("pbs-main", "backups", ""); !ok || inst != "cluster-a" {
		t.Fatalf("datastore resolution = %q,%v, want cluster-a,true", inst, ok)
	}
}

// TestPBSSourceLearnerUnobservedCandidateBlocksResolution covers the
// asymmetry that made a singleton source set unsafe: clusters only become
// visible through snapshots that were already attributable, so a cluster with
// none looks like it does not exist. A source shared with that cluster then
// maps to a single observed cluster and hands it the other cluster's
// snapshots (#1639).
func TestPBSSourceLearnerUnobservedCandidateBlocksResolution(t *testing.T) {
	l := NewPBSSourceLearner()
	l.RegisterCandidate("pbs-main", "cluster-a")
	l.RegisterCandidate("pbs-main", "cluster-b")
	l.Observe("pbs-main", "backups", "shared@pbs!token", "cluster-a")

	if inst, ok := l.Resolve("pbs-main", "backups", "shared@pbs!token"); ok {
		t.Fatalf("resolved to %q while cluster-b had no attributed snapshot, want inconclusive", inst)
	}

	// Once cluster-b is visible with a source of its own, the singleton is
	// backed by evidence that the token is not shared.
	l.Observe("pbs-main", "store-b", "cluster-b@pbs!token", "cluster-b")
	if inst, ok := l.Resolve("pbs-main", "backups", "shared@pbs!token"); !ok || inst != "cluster-a" {
		t.Fatalf("resolution with full coverage = %q,%v, want cluster-a,true", inst, ok)
	}
}

// TestPBSSourceLearnerCandidateVisibleOnOtherPBSInstanceResolves covers the
// reported topology: each cluster owns a PBS server, so the PBS instance is
// itself the discriminator. cluster-b never submits to pbs-one, but it is
// visible on pbs-two, which is what makes pbs-one's singleton meaningful.
func TestPBSSourceLearnerCandidateVisibleOnOtherPBSInstanceResolves(t *testing.T) {
	l := NewPBSSourceLearner()
	l.RegisterCandidate("pbs-one", "cluster-a")
	l.RegisterCandidate("pbs-one", "cluster-b")
	l.RegisterCandidate("pbs-two", "cluster-a")
	l.RegisterCandidate("pbs-two", "cluster-b")
	l.Observe("pbs-one", "backups", "", "cluster-a")
	l.Observe("pbs-two", "backups", "", "cluster-b")

	if inst, ok := l.Resolve("pbs-one", "backups", ""); !ok || inst != "cluster-a" {
		t.Fatalf("pbs-one resolution = %q,%v, want cluster-a,true", inst, ok)
	}
	if inst, ok := l.Resolve("pbs-two", "backups", ""); !ok || inst != "cluster-b" {
		t.Fatalf("pbs-two resolution = %q,%v, want cluster-b,true", inst, ok)
	}
}

// TestPBSSourceLearnerWithoutCandidatesIsInert guards the fail-closed
// default: a caller that never declares the candidate clusters gets no
// resolution rather than one drawn from whatever it happened to observe.
func TestPBSSourceLearnerWithoutCandidatesIsInert(t *testing.T) {
	l := NewPBSSourceLearner()
	l.Observe("pbs-main", "backups", "cluster-a@pbs!token", "cluster-a")

	if inst, ok := l.Resolve("pbs-main", "backups", "cluster-a@pbs!token"); ok {
		t.Fatalf("resolved to %q without declared candidates, want inconclusive", inst)
	}
}

func TestPBSSourceLearnerNilReceiverIsInert(t *testing.T) {
	var l *PBSSourceLearner
	l.RegisterCandidate("pbs-main", "cluster-a")
	l.Observe("pbs-main", "backups", "owner", "cluster-a")
	if _, ok := l.Resolve("pbs-main", "backups", "owner"); ok {
		t.Fatal("nil learner must never be decisive")
	}
}
