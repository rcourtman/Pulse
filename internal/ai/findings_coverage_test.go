package ai

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingPersistence struct {
	findings map[string]*Finding
	loadErr  error
	saveErr  error
	saved    chan map[string]*Finding
}

func (p *recordingPersistence) SaveFindings(findings map[string]*Finding) error {
	if p.saved != nil {
		p.saved <- findings
	}
	return p.saveErr
}

func (p *recordingPersistence) LoadFindings() (map[string]*Finding, error) {
	if p.loadErr != nil {
		return nil, p.loadErr
	}
	if p.findings == nil {
		return make(map[string]*Finding), nil
	}
	return p.findings, nil
}

func TestFindingsStore_SetPersistence_LoadError(t *testing.T) {
	store := NewFindingsStore()
	loadErr := errors.New("load error")

	err := store.SetPersistence(&recordingPersistence{loadErr: loadErr})
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected load error, got %v", err)
	}
}

func TestFindingsStore_SetPersistence_LoadsFindings(t *testing.T) {
	store := NewFindingsStore()
	now := time.Now()
	resolvedAt := now.Add(-time.Hour)

	p := &recordingPersistence{
		findings: map[string]*Finding{
			"active": {
				ID:         "active",
				Severity:   FindingSeverityWarning,
				ResourceID: "res-1",
				Title:      "Active",
				LastSeenAt: now,
			},
			"resolved": {
				ID:         "resolved",
				Severity:   FindingSeverityCritical,
				ResourceID: "res-2",
				Title:      "Resolved",
				LastSeenAt: now,
				ResolvedAt: &resolvedAt,
			},
		},
	}

	if err := store.SetPersistence(p); err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	if store.activeCounts[FindingSeverityWarning] != 1 {
		t.Errorf("expected active warning count 1, got %d", store.activeCounts[FindingSeverityWarning])
	}
	if len(store.byResource["res-1"]) != 1 {
		t.Errorf("expected resource index for res-1 to have 1 entry, got %d", len(store.byResource["res-1"]))
	}
}

func TestFindingsStore_scheduleSave_NoPersistence(t *testing.T) {
	store := NewFindingsStore()

	store.scheduleSave()

	if store.saveTimer != nil {
		t.Error("expected no save timer when persistence is nil")
	}
}

func TestFindingsStore_scheduleSave_SavePending(t *testing.T) {
	store := NewFindingsStore()
	store.persistence = &recordingPersistence{saved: make(chan map[string]*Finding, 1)}
	store.savePending = true

	store.scheduleSave()

	select {
	case <-store.persistence.(*recordingPersistence).saved:
		t.Fatal("save should not run when already pending")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestFindingsStore_scheduleSave_SavesAndSkipsDemo(t *testing.T) {
	store := NewFindingsStore()
	store.saveDebounce = 5 * time.Millisecond
	saved := make(chan map[string]*Finding, 1)
	store.persistence = &recordingPersistence{saved: saved}

	store.findings["demo-1"] = &Finding{ID: "demo-1", Title: "Demo"}
	store.findings["real-1"] = &Finding{ID: "real-1", Title: "Real"}

	store.scheduleSave()

	select {
	case data := <-saved:
		if _, exists := data["demo-1"]; exists {
			t.Error("demo findings should not be persisted")
		}
		if _, exists := data["real-1"]; !exists {
			t.Error("expected real finding to be persisted")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for SaveFindings")
	}
}

func TestFindingsStore_scheduleSave_SaveError(t *testing.T) {
	store := NewFindingsStore()
	store.saveDebounce = 5 * time.Millisecond
	saved := make(chan map[string]*Finding, 1)
	store.persistence = &recordingPersistence{
		saveErr: errors.New("save error"),
		saved:   saved,
	}

	store.findings["real-1"] = &Finding{ID: "real-1", Title: "Real"}
	store.scheduleSave()

	select {
	case <-saved:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for SaveFindings")
	}
}

func TestFindingsStore_ForceSave_NoPersistenceStopsTimer(t *testing.T) {
	store := NewFindingsStore()
	store.saveTimer = time.AfterFunc(time.Hour, func() {})

	if err := store.ForceSave(); err != nil {
		t.Fatalf("ForceSave failed: %v", err)
	}
}

func TestFindingsStore_ForceSave_SaveError(t *testing.T) {
	store := NewFindingsStore()
	store.persistence = &recordingPersistence{saveErr: errors.New("save error")}
	store.findings["real-1"] = &Finding{ID: "real-1", Title: "Real"}

	if err := store.ForceSave(); err == nil {
		t.Fatal("expected ForceSave to return error")
	}
}

func TestFindingsStore_ForceSave_SkipsDemo(t *testing.T) {
	store := NewFindingsStore()
	saved := make(chan map[string]*Finding, 1)
	store.persistence = &recordingPersistence{saved: saved}
	store.findings["demo-1"] = &Finding{ID: "demo-1", Title: "Demo"}
	store.findings["real-1"] = &Finding{ID: "real-1", Title: "Real"}

	if err := store.ForceSave(); err != nil {
		t.Fatalf("ForceSave failed: %v", err)
	}

	select {
	case data := <-saved:
		if _, exists := data["demo-1"]; exists {
			t.Error("demo findings should not be persisted")
		}
		if _, exists := data["real-1"]; !exists {
			t.Error("expected real finding to be persisted")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for SaveFindings")
	}
}

func TestFindingsStore_ClearAll(t *testing.T) {
	store := NewFindingsStore()
	store.Add(&Finding{ID: "f1", Severity: FindingSeverityWarning, ResourceID: "res-1", Title: "Test"})
	store.Add(&Finding{ID: "f2", Severity: FindingSeverityCritical, ResourceID: "res-2", Title: "Test"})

	count := store.ClearAll()

	if count != 2 {
		t.Errorf("expected 2 findings cleared, got %d", count)
	}
	if len(store.findings) != 0 {
		t.Error("expected findings map to be empty")
	}
	if len(store.byResource) != 0 {
		t.Error("expected resource index to be empty")
	}
	if len(store.activeCounts) != 0 {
		t.Error("expected active counts to be reset")
	}
}

func TestFindingsStore_Undismiss(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		store := NewFindingsStore()
		if store.Undismiss("missing") {
			t.Error("expected false for missing finding")
		}
	})

	t.Run("not dismissed", func(t *testing.T) {
		store := NewFindingsStore()
		store.findings["f1"] = &Finding{ID: "f1", Severity: FindingSeverityWarning}
		if store.Undismiss("f1") {
			t.Error("expected false for non-dismissed finding")
		}
	})

	t.Run("resolved stays inactive", func(t *testing.T) {
		store := NewFindingsStore()
		resolvedAt := time.Now()
		store.findings["f1"] = &Finding{
			ID:              "f1",
			Severity:        FindingSeverityWarning,
			DismissedReason: "expected_behavior",
			ResolvedAt:      &resolvedAt,
		}

		if !store.Undismiss("f1") {
			t.Fatal("expected Undismiss to succeed")
		}
		if store.activeCounts[FindingSeverityWarning] != 0 {
			t.Errorf("expected active count to remain 0, got %d", store.activeCounts[FindingSeverityWarning])
		}
	})

	t.Run("reactivates", func(t *testing.T) {
		store := NewFindingsStore()
		store.findings["f1"] = &Finding{
			ID:              "f1",
			Severity:        FindingSeverityWarning,
			DismissedReason: "expected_behavior",
		}
		store.activeCounts[FindingSeverityWarning] = 0

		if !store.Undismiss("f1") {
			t.Fatal("expected Undismiss to succeed")
		}
		if store.activeCounts[FindingSeverityWarning] != 1 {
			t.Errorf("expected active count to increment to 1, got %d", store.activeCounts[FindingSeverityWarning])
		}
	})
}

func TestFindingsStore_GetDismissedForContext_SnoozedAndOld(t *testing.T) {
	store := NewFindingsStore()
	now := time.Now()

	store.findings["old"] = &Finding{
		ID:              "old",
		Title:           "Old Finding",
		ResourceName:    "old",
		DismissedReason: "not_an_issue",
		LastSeenAt:      now.Add(-31 * 24 * time.Hour),
	}
	store.findings["snoozed"] = &Finding{
		ID:           "snoozed",
		Title:        "Snoozed Finding",
		ResourceName: "host-1",
		SnoozedUntil: timePtr(now.Add(2 * time.Hour)),
		LastSeenAt:   now,
	}

	ctx := store.GetDismissedForContext()
	if strings.Contains(ctx, "Old Finding") {
		t.Error("old findings should be excluded from context")
	}
	if !strings.Contains(ctx, "Temporarily Snoozed") {
		t.Error("expected snoozed section in context")
	}
	if !strings.Contains(ctx, "Snoozed Finding") {
		t.Error("expected snoozed finding in context")
	}
}

func TestFindingsStore_GetDismissedForContext_SuppressedNoteAndDismissed(t *testing.T) {
	store := NewFindingsStore()
	now := time.Now()

	store.findings["suppressed"] = &Finding{
		ID:              "suppressed",
		Title:           "Suppressed Finding",
		ResourceName:    "host-1",
		Suppressed:      true,
		DismissedReason: "not_an_issue",
		UserNote:        "ignore this",
		LastSeenAt:      now,
	}
	store.findings["dismissed"] = &Finding{
		ID:              "dismissed",
		Title:           "Dismissed Finding",
		ResourceName:    "host-2",
		DismissedReason: "expected_behavior",
		LastSeenAt:      now,
	}
	store.findings["dismissed-note"] = &Finding{
		ID:              "dismissed-note",
		Title:           "Dismissed With Note",
		ResourceName:    "host-3",
		DismissedReason: "will_fix_later",
		UserNote:        "accepted risk",
		LastSeenAt:      now,
	}

	ctx := store.GetDismissedForContext()
	if !strings.Contains(ctx, "User note: ignore this") {
		t.Error("expected suppressed user note in context")
	}
	if !strings.Contains(ctx, "Dismissed Finding") {
		t.Error("expected dismissed finding in context")
	}
	if !strings.Contains(ctx, "User note: accepted risk") {
		t.Error("expected dismissed user note in context")
	}
}

func TestFindingsStore_GetSuppressionRules_DismissedRule(t *testing.T) {
	store := NewFindingsStore()
	now := time.Now()

	store.findings["f1"] = &Finding{
		ID:              "f1",
		ResourceID:      "res-1",
		ResourceName:    "Res 1",
		Category:        FindingCategoryCapacity,
		DismissedReason: "expected_behavior",
		LastSeenAt:      now,
	}

	rules := store.GetSuppressionRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	rule := rules[0]
	if rule.CreatedFrom != "dismissed" {
		t.Errorf("expected created_from dismissed, got %s", rule.CreatedFrom)
	}
	if !rule.CreatedAt.Equal(now) {
		t.Error("expected CreatedAt to use LastSeenAt when AcknowledgedAt is nil")
	}
}

func TestFindingsStore_DeleteSuppressionRule_Explicit(t *testing.T) {
	store := NewFindingsStore()
	rule := store.AddSuppressionRule("res-1", "Res 1", FindingCategoryCapacity, "Manual")

	if !store.DeleteSuppressionRule(rule.ID) {
		t.Fatal("expected DeleteSuppressionRule to succeed")
	}
	if store.DeleteSuppressionRule(rule.ID) {
		t.Error("expected DeleteSuppressionRule to return false after removal")
	}
}

func TestFindingsStore_DeleteSuppressionRule_ReactivatesFinding(t *testing.T) {
	store := NewFindingsStore()
	store.findings["f1"] = &Finding{
		ID:              "f1",
		Severity:        FindingSeverityWarning,
		DismissedReason: "not_an_issue",
		Suppressed:      true,
	}
	store.activeCounts[FindingSeverityWarning] = 0

	if !store.DeleteSuppressionRule("finding_f1") {
		t.Fatal("expected DeleteSuppressionRule to succeed")
	}
	if store.activeCounts[FindingSeverityWarning] != 1 {
		t.Errorf("expected active count to increment, got %d", store.activeCounts[FindingSeverityWarning])
	}
}
