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
	l.Observe("pbs-main", "backups", "shared@pbs!token", "cluster-a")
	l.Observe("pbs-main", "backups", "shared@pbs!token", "cluster-b")

	if inst, ok := l.Resolve("pbs-main", "backups", "shared@pbs!token"); ok {
		t.Fatalf("shared source resolved to %q, want inconclusive", inst)
	}
}

func TestPBSSourceLearnerUnfamiliarComponentStopsResolution(t *testing.T) {
	l := NewPBSSourceLearner()
	l.Observe("pbs-main", "backups", "cluster-a@pbs!token", "cluster-a")

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

func TestPBSSourceLearnerNilReceiverIsInert(t *testing.T) {
	var l *PBSSourceLearner
	l.Observe("pbs-main", "backups", "owner", "cluster-a")
	if _, ok := l.Resolve("pbs-main", "backups", "owner"); ok {
		t.Fatal("nil learner must never be decisive")
	}
}
