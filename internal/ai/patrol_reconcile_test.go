package ai

import (
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
