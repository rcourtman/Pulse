package ai

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// newTestPatrolWithFindings creates a minimal PatrolService with a FindingsStore
// seeded with the given findings.
func newTestPatrolWithFindings(findings []*Finding) *PatrolService {
	store := NewFindingsStore()
	for _, f := range findings {
		store.Add(f)
	}
	return &PatrolService{
		findings: store,
	}
}

func TestReconcileStaleFindings_ResolvesUnreported(t *testing.T) {
	f := &Finding{
		ID:           "find-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "vm-100",
		ResourceName: "web",
		Title:        "Disk nearly full",
		DetectedAt:   time.Now().Add(-1 * time.Hour),
	}
	p := newTestPatrolWithFindings([]*Finding{f})

	// Seed context included find-1, but the LLM didn't re-report it
	resolved := p.reconcileStaleFindings(
		nil,            // reportedIDs: nothing re-reported
		nil,            // resolvedIDs: nothing explicitly resolved
		[]string{f.ID}, // seeded: find-1 was in seed context
		false,          // no errors
	)

	if resolved != 1 {
		t.Fatalf("expected 1 auto-resolved finding, got %d", resolved)
	}

	stored := p.findings.Get(f.ID)
	if stored == nil {
		t.Fatal("finding should still exist in store")
	}
	if stored.ResolvedAt == nil {
		t.Fatal("finding should be resolved")
	}
	if !stored.AutoResolved {
		t.Fatal("finding should be marked auto-resolved")
	}
	if stored.ResolveReason != "No longer detected by patrol" {
		t.Fatalf("unexpected resolve reason: %s", stored.ResolveReason)
	}
}

func TestReconcileStaleFindings_KeepsReported(t *testing.T) {
	f := &Finding{
		ID:           "find-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "vm-100",
		ResourceName: "web",
		Title:        "Disk nearly full",
		DetectedAt:   time.Now().Add(-1 * time.Hour),
	}
	p := newTestPatrolWithFindings([]*Finding{f})

	// LLM re-reported the finding — it's still an issue
	resolved := p.reconcileStaleFindings(
		[]string{f.ID}, // re-reported
		nil,
		[]string{f.ID}, // seeded
		false,
	)

	if resolved != 0 {
		t.Fatalf("expected 0 auto-resolved findings, got %d", resolved)
	}

	stored := p.findings.Get(f.ID)
	if stored.ResolvedAt != nil {
		t.Fatal("finding should NOT be resolved")
	}
}

func TestReconcileStaleFindings_KeepsExplicitlyResolved(t *testing.T) {
	f := &Finding{
		ID:           "find-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "vm-100",
		ResourceName: "web",
		Title:        "Disk nearly full",
		DetectedAt:   time.Now().Add(-1 * time.Hour),
	}
	p := newTestPatrolWithFindings([]*Finding{f})

	// LLM already resolved it via tool — we shouldn't double-resolve
	// First, resolve it as the LLM would
	p.findings.Resolve(f.ID, true)

	resolved := p.reconcileStaleFindings(
		nil,
		[]string{f.ID}, // explicitly resolved by LLM
		[]string{f.ID}, // seeded
		false,
	)

	if resolved != 0 {
		t.Fatalf("expected 0 auto-resolved findings (already resolved by LLM), got %d", resolved)
	}
}

func TestReconcileStaleFindings_SkipsIfRunHadErrors(t *testing.T) {
	f := &Finding{
		ID:           "find-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "vm-100",
		ResourceName: "web",
		Title:        "Disk nearly full",
		DetectedAt:   time.Now().Add(-1 * time.Hour),
	}
	p := newTestPatrolWithFindings([]*Finding{f})

	resolved := p.reconcileStaleFindings(
		nil,
		nil,
		[]string{f.ID},
		true, // run had errors
	)

	if resolved != 0 {
		t.Fatalf("expected 0 auto-resolved findings when run had errors, got %d", resolved)
	}

	stored := p.findings.Get(f.ID)
	if stored.ResolvedAt != nil {
		t.Fatal("finding should NOT be resolved when run had errors")
	}
}

func TestReconcileStaleFindings_SkipsUnseededFindings(t *testing.T) {
	f1 := &Finding{
		ID:           "find-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "vm-100",
		ResourceName: "web",
		Title:        "Disk nearly full",
		DetectedAt:   time.Now().Add(-1 * time.Hour),
	}
	f2 := &Finding{
		ID:           "find-2",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-200",
		ResourceName: "api",
		Title:        "High CPU",
		DetectedAt:   time.Now().Add(-1 * time.Hour),
	}
	p := newTestPatrolWithFindings([]*Finding{f1, f2})

	// Only find-1 was in seed context; find-2 was not (e.g., created between seed build and run)
	resolved := p.reconcileStaleFindings(
		nil,
		nil,
		[]string{f1.ID}, // only find-1 seeded
		false,
	)

	if resolved != 1 {
		t.Fatalf("expected 1 auto-resolved finding, got %d", resolved)
	}

	// find-1 should be resolved (seeded but not re-reported)
	stored1 := p.findings.Get(f1.ID)
	if stored1.ResolvedAt == nil {
		t.Fatal("find-1 should be resolved")
	}

	// find-2 should NOT be resolved (not in seed context)
	stored2 := p.findings.Get(f2.ID)
	if stored2.ResolvedAt != nil {
		t.Fatal("find-2 should NOT be resolved (was not in seed context)")
	}
}

func TestReconcileStaleFindings_NoSeededFindings(t *testing.T) {
	f := &Finding{
		ID:           "find-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "vm-100",
		ResourceName: "web",
		Title:        "Disk nearly full",
		DetectedAt:   time.Now().Add(-1 * time.Hour),
	}
	p := newTestPatrolWithFindings([]*Finding{f})

	// No seeded findings at all — nothing to reconcile
	resolved := p.reconcileStaleFindings(
		nil,
		nil,
		nil, // empty seed
		false,
	)

	if resolved != 0 {
		t.Fatalf("expected 0 auto-resolved findings when no seeded IDs, got %d", resolved)
	}
}

// Category gate: only performance and capacity findings (continuous current-state
// metrics) may be auto-resolved from absence. Reliability, backup, security, and
// general findings represent events or persistent states and must stay active
// until explicitly resolved — silent absence-based auto-resolve there caused
// bogus auto_resolved → re-detected → regressed cycles that polluted the trust
// strip and inflated the regression counter.
func TestReconcileStaleFindings_SkipsNonCurrentStateCategories(t *testing.T) {
	nonEligible := []struct {
		name     string
		category FindingCategory
	}{
		{"reliability", FindingCategoryReliability},
		{"backup", FindingCategoryBackup},
		{"security", FindingCategorySecurity},
		{"general", FindingCategoryGeneral},
	}
	for _, tc := range nonEligible {
		t.Run(string(tc.category), func(t *testing.T) {
			f := &Finding{
				ID:           "find-" + string(tc.category),
				Severity:     FindingSeverityWarning,
				Category:     tc.category,
				ResourceID:   "vm-100",
				ResourceName: "web",
				Title:        "Some persistent event",
				DetectedAt:   time.Now().Add(-1 * time.Hour),
			}
			p := newTestPatrolWithFindings([]*Finding{f})

			resolved := p.reconcileStaleFindings(
				nil,            // not re-reported
				nil,            // not explicitly resolved
				[]string{f.ID}, // seeded
				false,          // run succeeded
			)

			if resolved != 0 {
				t.Fatalf("expected 0 auto-resolved findings for category %s, got %d", tc.category, resolved)
			}
			stored := p.findings.Get(f.ID)
			if stored == nil {
				t.Fatal("finding should still exist in store")
			}
			if stored.ResolvedAt != nil {
				t.Fatalf("category %s finding must NOT be auto-resolved from absence; resolved at %s", tc.category, stored.ResolvedAt)
			}
		})
	}
}

func TestCategorySupportsStaleAutoResolve(t *testing.T) {
	cases := []struct {
		category FindingCategory
		want     bool
	}{
		{FindingCategoryPerformance, true},
		{FindingCategoryCapacity, true},
		{FindingCategoryReliability, false},
		{FindingCategoryBackup, false},
		{FindingCategorySecurity, false},
		{FindingCategoryGeneral, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.category), func(t *testing.T) {
			if got := CategorySupportsStaleAutoResolve(tc.category); got != tc.want {
				t.Fatalf("CategorySupportsStaleAutoResolve(%s) = %v, want %v", tc.category, got, tc.want)
			}
		})
	}
}

// --- Verified auto-clear of event/persistent findings ---------------------
//
// Event/persistent categories must not auto-resolve from absence, but an
// affirmative deterministic verification ("the failure signal is gone") is
// stronger evidence than absence — the same standard the LLM-resolve gate
// demands. These tests pin the asymmetric half of the "Backup failed" flap
// fix: a fixed issue clears once the verifier confirms it, and anything
// short of an affirmative confirmation fails closed.

func newVerifierFinding(id, key string) *Finding {
	return &Finding{
		ID:           id,
		Key:          key,
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryBackup,
		ResourceID:   "vm-100",
		ResourceName: "web",
		Title:        "Backup failed",
		DetectedAt:   time.Now().Add(-1 * time.Hour),
	}
}

func TestReconcileStaleFindings_VerifiedAutoClearResolvesFixedFinding(t *testing.T) {
	f := newVerifierFinding("find-backup", "backup-failed")
	p := newTestPatrolWithFindings([]*Finding{f})
	p.verifyFixResolvedFn = func(ctx context.Context, resourceID, resourceType, findingKey, findingID string) (bool, error) {
		if resourceID != "vm-100" || findingKey != "backup-failed" || findingID != "find-backup" {
			t.Fatalf("verifier called with wrong identity: %s/%s/%s", resourceID, findingKey, findingID)
		}
		return true, nil // signal affirmatively gone
	}

	resolved := p.reconcileStaleFindings(nil, nil, []string{f.ID}, false)

	if resolved != 1 {
		t.Fatalf("expected 1 verified auto-resolve, got %d", resolved)
	}
	stored := p.findings.Get(f.ID)
	if stored == nil || stored.ResolvedAt == nil {
		t.Fatal("finding should be resolved after affirmative verification")
	}
	if !stored.AutoResolved {
		t.Fatal("verified clear should be marked auto-resolved")
	}
	if stored.ResolveReason != verifiedStaleResolveReason {
		t.Fatalf("unexpected resolve reason: %s", stored.ResolveReason)
	}
}

func TestReconcileStaleFindings_VerifierStillDetectsKeepsActive(t *testing.T) {
	f := newVerifierFinding("find-backup", "backup-failed")
	p := newTestPatrolWithFindings([]*Finding{f})
	p.verifyFixResolvedFn = func(ctx context.Context, resourceID, resourceType, findingKey, findingID string) (bool, error) {
		return false, nil // failure signal still present
	}

	if resolved := p.reconcileStaleFindings(nil, nil, []string{f.ID}, false); resolved != 0 {
		t.Fatalf("expected 0 resolves while signal still present, got %d", resolved)
	}
	if stored := p.findings.Get(f.ID); stored == nil || stored.ResolvedAt != nil {
		t.Fatal("finding must stay active while the failure signal is still present")
	}
}

func TestReconcileStaleFindings_VerifierInconclusiveFailsClosed(t *testing.T) {
	f := newVerifierFinding("find-backup", "backup-failed")
	p := newTestPatrolWithFindings([]*Finding{f})
	p.verifyFixResolvedFn = func(ctx context.Context, resourceID, resourceType, findingKey, findingID string) (bool, error) {
		return false, context.DeadlineExceeded // inconclusive
	}

	if resolved := p.reconcileStaleFindings(nil, nil, []string{f.ID}, false); resolved != 0 {
		t.Fatalf("expected 0 resolves on inconclusive verification, got %d", resolved)
	}
	if stored := p.findings.Get(f.ID); stored == nil || stored.ResolvedAt != nil {
		t.Fatal("finding must stay active when verification is inconclusive (fail closed)")
	}
}

func TestReconcileStaleFindings_NoVerifierEventFindingStaysActive(t *testing.T) {
	// pbs-job-failed has no deterministic verifier and must not be mapped
	// onto backup-failed; the finding stays active until LLM/operator resolve.
	f := newVerifierFinding("find-pbs", "pbs-job-failed")
	p := newTestPatrolWithFindings([]*Finding{f})
	p.verifyFixResolvedFn = func(ctx context.Context, resourceID, resourceType, findingKey, findingID string) (bool, error) {
		t.Fatal("verifier must not be called for keys without a deterministic verifier")
		return false, nil
	}

	if resolved := p.reconcileStaleFindings(nil, nil, []string{f.ID}, false); resolved != 0 {
		t.Fatalf("expected 0 resolves for verifier-less event finding, got %d", resolved)
	}
	if stored := p.findings.Get(f.ID); stored == nil || stored.ResolvedAt != nil {
		t.Fatal("verifier-less event finding must stay active")
	}
}

func TestReconcileStaleFindings_VerifiedAutoClearIsIdempotent(t *testing.T) {
	// The no-noise invariant: repeating reconcile over unchanged state must
	// not produce new resolves, lifecycle events, or resolve/re-detect cycles.
	f := newVerifierFinding("find-backup", "backup-failed")
	p := newTestPatrolWithFindings([]*Finding{f})
	p.verifyFixResolvedFn = func(ctx context.Context, resourceID, resourceType, findingKey, findingID string) (bool, error) {
		return true, nil
	}

	if first := p.reconcileStaleFindings(nil, nil, []string{f.ID}, false); first != 1 {
		t.Fatalf("expected first pass to resolve 1, got %d", first)
	}
	stored := p.findings.Get(f.ID)
	resolvedAt := *stored.ResolvedAt
	lifecycleLen := len(stored.Lifecycle)

	if second := p.reconcileStaleFindings(nil, nil, []string{f.ID}, false); second != 0 {
		t.Fatalf("expected second pass over unchanged state to resolve 0, got %d", second)
	}
	stored = p.findings.Get(f.ID)
	if stored.ResolvedAt == nil || !stored.ResolvedAt.Equal(resolvedAt) {
		t.Fatal("second pass must not touch the resolution")
	}
	if len(stored.Lifecycle) != lifecycleLen {
		t.Fatalf("second pass must not append lifecycle events: %d -> %d", lifecycleLen, len(stored.Lifecycle))
	}
}

func TestReconcileStaleFindings_VerificationCapDefersExcessCandidates(t *testing.T) {
	findings := []*Finding{
		newVerifierFinding("find-1", "backup-failed"),
		newVerifierFinding("find-2", "backup-failed"),
		newVerifierFinding("find-3", "backup-failed"),
		newVerifierFinding("find-4", "backup-failed"),
	}
	// Distinct resources so the store keeps all four.
	for i, f := range findings {
		f.ResourceID = fmt.Sprintf("vm-%d", 100+i)
	}
	p := newTestPatrolWithFindings(findings)
	calls := 0
	p.verifyFixResolvedFn = func(ctx context.Context, resourceID, resourceType, findingKey, findingID string) (bool, error) {
		calls++
		return true, nil
	}

	ids := []string{"find-1", "find-2", "find-3", "find-4"}
	resolved := p.reconcileStaleFindings(nil, nil, ids, false)

	if calls != maxVerifiedStaleResolvesPerRun {
		t.Fatalf("expected verification calls capped at %d, got %d", maxVerifiedStaleResolvesPerRun, calls)
	}
	if resolved != maxVerifiedStaleResolvesPerRun {
		t.Fatalf("expected %d resolves this run, got %d", maxVerifiedStaleResolvesPerRun, resolved)
	}
	deferred := p.findings.Get("find-4")
	if deferred == nil || deferred.ResolvedAt != nil {
		t.Fatal("candidate beyond the cap must stay active and retry next run")
	}
}

func TestNormalizeFindingKey_CanonicalAliases(t *testing.T) {
	cases := map[string]string{
		"high-cpu":     "cpu-high",
		"High_Memory":  "memory-high",
		"high disk":    "disk-high",
		"cpu-high":     "cpu-high",
		"backup-stale": "backup-stale",
		// Non-aliased keys pass through normalization unchanged.
		"pbs-job-failed": "pbs-job-failed",
	}
	for in, want := range cases {
		if got := normalizeFindingKey(in); got != want {
			t.Fatalf("normalizeFindingKey(%q) = %q, want %q", in, got, want)
		}
	}
}
