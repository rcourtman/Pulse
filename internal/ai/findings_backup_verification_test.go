package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func staleRollup(t *testing.T, now time.Time) *recovery.ProtectionRollup {
	t.Helper()
	// Recent successful backup (within one window), no verification at all.
	successAt := now.Add(-2 * 24 * time.Hour)
	return &recovery.ProtectionRollup{
		RollupID:          "res:vm-100",
		SubjectResourceID: "vm-100",
		Display: &recovery.RecoveryPointDisplay{
			SubjectLabel: "web-prod",
			SubjectType:  "proxmox-vm",
		},
		LastSuccessAt: &successAt,
		LastAttemptAt: &successAt,
		LastOutcome:   recovery.OutcomeSuccess,
		VerifyIntent:  recovery.VerifyIntentStale,
	}
}

func TestBuildBackupVerificationStaleFinding_EmitsFindingForStaleRollup(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	rollup := staleRollup(t, now)

	f := BuildBackupVerificationStaleFinding(rollup, now)
	if f == nil {
		t.Fatal("expected a finding for a stale rollup, got nil")
	}
	if f.Category != FindingCategoryBackup {
		t.Fatalf("Category = %q, want %q", f.Category, FindingCategoryBackup)
	}
	if f.Key != BackupVerificationStaleFindingKey {
		t.Fatalf("Key = %q, want %q", f.Key, BackupVerificationStaleFindingKey)
	}
	// Default severity is Watch when only a single staleness window has elapsed.
	if f.Severity != FindingSeverityWatch {
		t.Fatalf("Severity = %q, want %q", f.Severity, FindingSeverityWatch)
	}
	if f.ResourceID != "vm-100" {
		t.Fatalf("ResourceID = %q, want %q", f.ResourceID, "vm-100")
	}
	if f.ResourceName != "web-prod" {
		t.Fatalf("ResourceName = %q, want preferred display label", f.ResourceName)
	}

	// ID must be deterministic across two emissions for the same rollup.
	second := BuildBackupVerificationStaleFinding(rollup, now)
	if second == nil || second.ID != f.ID {
		t.Fatalf("expected deterministic finding ID across emissions, got %q vs %q", f.ID, secondID(second))
	}

	// Different resource → different finding ID.
	other := staleRollup(t, now)
	other.SubjectResourceID = "vm-999"
	otherFinding := BuildBackupVerificationStaleFinding(other, now)
	if otherFinding == nil || otherFinding.ID == f.ID {
		t.Fatalf("expected distinct IDs for distinct resources, got %q vs %q", f.ID, secondID(otherFinding))
	}
}

func TestBuildBackupVerificationStaleFinding_NilForNonStaleRollup(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(-24 * time.Hour)
	rollup := &recovery.ProtectionRollup{
		RollupID:          "res:vm-100",
		SubjectResourceID: "vm-100",
		LastSuccessAt:     &verifiedAt,
		LastVerifiedAt:    &verifiedAt,
		LastOutcome:       recovery.OutcomeSuccess,
		VerifyIntent:      recovery.VerifyIntentVerified,
	}

	if f := BuildBackupVerificationStaleFinding(rollup, now); f != nil {
		t.Fatalf("expected nil for verified rollup, got %#v", f)
	}

	rollup.VerifyIntent = recovery.VerifyIntentUnknown
	if f := BuildBackupVerificationStaleFinding(rollup, now); f != nil {
		t.Fatalf("expected nil for unknown rollup, got %#v", f)
	}

	if f := BuildBackupVerificationStaleFinding(nil, now); f != nil {
		t.Fatal("expected nil for nil rollup")
	}
}

func TestBuildBackupVerificationStaleFinding_EscalatesAfterMultipleWindows(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	// Last successful backup is 21 days old → 3 staleness windows of 7d.
	old := now.Add(-21 * 24 * time.Hour)
	rollup := &recovery.ProtectionRollup{
		RollupID:          "res:vm-200",
		SubjectResourceID: "vm-200",
		Display: &recovery.RecoveryPointDisplay{
			SubjectLabel: "db-prod",
			SubjectType:  "proxmox-vm",
		},
		LastSuccessAt: &old,
		LastAttemptAt: &old,
		LastOutcome:   recovery.OutcomeSuccess,
		VerifyIntent:  recovery.VerifyIntentStale,
	}

	f := BuildBackupVerificationStaleFinding(rollup, now)
	if f == nil {
		t.Fatal("expected finding")
	}
	if f.Severity != FindingSeverityWarning {
		t.Fatalf("Severity = %q, want %q for multi-window stale", f.Severity, FindingSeverityWarning)
	}
}

func TestBuildBackupVerificationStaleFinding_PatrolIntakeDedupes(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	store := NewFindingsStore()
	rollup := staleRollup(t, now)

	first := BuildBackupVerificationStaleFinding(rollup, now)
	if first == nil {
		t.Fatal("expected initial finding")
	}
	if !store.Add(first) {
		t.Fatal("expected first Add to create a new finding")
	}

	// Same patrol tick, same rollup. Should dedup on (resourceID, category, key).
	second := BuildBackupVerificationStaleFinding(rollup, now.Add(15*time.Minute))
	if second == nil {
		t.Fatal("expected second emission for still-stale rollup")
	}
	if second.ID != first.ID {
		t.Fatalf("expected dedup-stable ID, got %q (first) vs %q (second)", first.ID, second.ID)
	}
	if store.Add(second) {
		t.Fatal("expected second Add to update existing (return false), not create new")
	}

	stored := store.Get(first.ID)
	if stored == nil {
		t.Fatal("expected stored finding to exist")
	}
	if stored.TimesRaised != 2 {
		t.Fatalf("TimesRaised = %d, want 2", stored.TimesRaised)
	}
}

func TestBuildBackupVerificationStaleFinding_ResolutionPathFiresOnReverify(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	store := NewFindingsStore()

	rollup := staleRollup(t, now)
	finding := BuildBackupVerificationStaleFinding(rollup, now)
	if finding == nil {
		t.Fatal("expected stale finding")
	}
	if !store.Add(finding) {
		t.Fatal("expected first Add to create")
	}

	// Verification reappears on the rollup. The detector returns nil; the
	// caller is expected to drive the resolution path. Mirror the lifecycle
	// fixtures: Resolve(auto=true) and confirm the auto_resolved event lands.
	verifiedAt := now.Add(5 * time.Minute)
	rollup.VerifyIntent = recovery.VerifyIntentVerified
	rollup.LastVerifiedAt = &verifiedAt

	if reemitted := BuildBackupVerificationStaleFinding(rollup, now.Add(10*time.Minute)); reemitted != nil {
		t.Fatalf("expected detector to skip emission once verified, got %#v", reemitted)
	}

	if !store.Resolve(finding.ID, true) {
		t.Fatal("expected Resolve to succeed")
	}

	stored := store.Get(finding.ID)
	if stored == nil {
		t.Fatal("expected finding to exist after resolve")
	}
	if stored.ResolvedAt == nil {
		t.Fatal("expected ResolvedAt to be set after Resolve")
	}
	if !stored.AutoResolved {
		t.Fatal("expected AutoResolved=true after Resolve(true)")
	}
	foundAutoResolved := false
	for _, e := range stored.Lifecycle {
		if e.Type == "auto_resolved" {
			foundAutoResolved = true
			break
		}
	}
	if !foundAutoResolved {
		t.Fatalf("expected auto_resolved lifecycle event; got: %+v", stored.Lifecycle)
	}
}

func secondID(f *Finding) string {
	if f == nil {
		return "<nil>"
	}
	return f.ID
}
