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

func TestFindingsStore_SetPersistence_NormalizesRegressedAcknowledgementState(t *testing.T) {
	store := NewFindingsStore()
	store.saveDebounce = 5 * time.Millisecond
	now := time.Now()
	acknowledgedAt := now.Add(-2 * time.Hour)
	lastRegressionAt := now.Add(-time.Hour)
	saved := make(chan map[string]*Finding, 1)

	p := &recordingPersistence{
		findings: map[string]*Finding{
			"regressed": {
				ID:               "regressed",
				Severity:         FindingSeverityWarning,
				ResourceID:       "res-3",
				Title:            "Regressed",
				LastSeenAt:       now,
				AcknowledgedAt:   &acknowledgedAt,
				LastRegressionAt: &lastRegressionAt,
			},
		},
		saved: saved,
	}

	if err := store.SetPersistence(p); err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	got := store.Get("regressed")
	if got == nil {
		t.Fatal("expected finding to load")
	}
	if got.AcknowledgedAt != nil {
		t.Fatal("expected stale acknowledgement to be cleared on load")
	}

	select {
	case persisted := <-saved:
		if persisted["regressed"].AcknowledgedAt != nil {
			t.Fatal("expected normalized acknowledgement state to be persisted")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for normalized findings save")
	}
}

func TestFindingsStore_SetPersistence_RetiresLegacyAlertMirrorFindings(t *testing.T) {
	store := NewFindingsStore()
	store.saveDebounce = 5 * time.Millisecond
	now := time.Now()
	saved := make(chan map[string]*Finding, 1)

	p := &recordingPersistence{
		findings: map[string]*Finding{
			// Active "Active alert detected" finding from the removed
			// SignalActiveAlert path — must be retired on load.
			"legacy-mirror": {
				ID:         "legacy-mirror",
				Severity:   FindingSeverityWarning,
				ResourceID: "vm-100",
				Title:      "Active alert detected",
				Source:     "ai-analysis",
				Category:   FindingCategoryGeneral,
				LastSeenAt: now,
			},
			// Distinct finding with matching title but different source —
			// should NOT be retired (defensive: never retire something we
			// can't positively identify as the alert-mirror artifact).
			"foreign-title-match": {
				ID:         "foreign-title-match",
				Severity:   FindingSeverityWarning,
				ResourceID: "vm-200",
				Title:      "Active alert detected",
				Source:     "operator",
				Category:   FindingCategoryGeneral,
				LastSeenAt: now,
			},
			// Different finding — must remain active untouched.
			"unrelated": {
				ID:         "unrelated",
				Severity:   FindingSeverityCritical,
				ResourceID: "vm-300",
				Title:      "Disk nearly full",
				Source:     "ai-analysis",
				Category:   FindingCategoryCapacity,
				LastSeenAt: now,
			},
		},
		saved: saved,
	}

	if err := store.SetPersistence(p); err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	mirror := store.Get("legacy-mirror")
	if mirror == nil {
		t.Fatal("expected legacy mirror finding to load")
	}
	if mirror.ResolvedAt == nil {
		t.Fatal("legacy mirror finding must be retired (ResolvedAt set) on load")
	}
	if !mirror.AutoResolved {
		t.Fatal("legacy mirror finding must be marked AutoResolved")
	}
	if mirror.ResolveReason == "" {
		t.Fatal("legacy mirror finding must carry a retirement reason")
	}
	if mirror.IsActive() {
		t.Fatal("legacy mirror finding must not stay active")
	}
	foundAutoResolvedEvent := false
	for _, e := range mirror.Lifecycle {
		if e.Type == "auto_resolved" {
			foundAutoResolvedEvent = true
			break
		}
	}
	if !foundAutoResolvedEvent {
		t.Fatal("legacy mirror retirement must append an auto_resolved lifecycle event")
	}

	foreign := store.Get("foreign-title-match")
	if foreign == nil {
		t.Fatal("expected foreign-title-match to load")
	}
	if foreign.ResolvedAt != nil {
		t.Fatalf("foreign-title-match must not be retired by the alert-mirror migration; resolved at %s", foreign.ResolvedAt)
	}

	unrelated := store.Get("unrelated")
	if unrelated == nil {
		t.Fatal("expected unrelated finding to load")
	}
	if unrelated.ResolvedAt != nil {
		t.Fatal("unrelated finding must not be retired")
	}

	select {
	case persisted := <-saved:
		if persisted["legacy-mirror"].ResolvedAt == nil {
			t.Fatal("retired mirror state must be persisted back")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for retirement save")
	}
}

func TestFindingsStore_SetPersistence_ResetsRegressionCounterPollutedByBogusCycles(t *testing.T) {
	store := NewFindingsStore()
	store.saveDebounce = 5 * time.Millisecond
	now := time.Now()
	lastRegress := now.Add(-30 * time.Minute)
	saved := make(chan map[string]*Finding, 1)

	withBogusCycle := &Finding{
		ID:               "with-bogus",
		Severity:         FindingSeverityWarning,
		ResourceID:       "vm-bogus",
		Title:            "Backup failed",
		Source:           "ai-analysis",
		Category:         FindingCategoryBackup,
		LastSeenAt:       now,
		RegressionCount:  6,
		LastRegressionAt: &lastRegress,
		Lifecycle: []FindingLifecycleEvent{
			{At: now.Add(-3 * time.Hour), Type: "detected"},
			{At: now.Add(-2 * time.Hour), Type: "auto_resolved", Message: "No longer detected by patrol"},
			{At: now.Add(-90 * time.Minute), Type: "regressed", Message: "Finding re-detected after resolution"},
			{At: now.Add(-60 * time.Minute), Type: "auto_resolved", Message: "Resource no longer exists in infrastructure"},
			{At: lastRegress, Type: "regressed", Message: "Finding re-detected after resolution"},
		},
	}

	withRealCycle := &Finding{
		ID:               "with-real",
		Severity:         FindingSeverityWarning,
		ResourceID:       "vm-real",
		Title:            "High CPU usage",
		Source:           "ai-analysis",
		Category:         FindingCategoryPerformance,
		LastSeenAt:       now,
		RegressionCount:  2,
		LastRegressionAt: &lastRegress,
		Lifecycle: []FindingLifecycleEvent{
			{At: now.Add(-2 * time.Hour), Type: "detected"},
			// An LLM-driven explicit resolve (empty message via Resolve(_,true)
			// is NOT one of the bogus-signature reasons) — the regression
			// counter that follows reflects a legitimate recurrence and
			// must be preserved.
			{At: now.Add(-90 * time.Minute), Type: "auto_resolved"},
			{At: lastRegress, Type: "regressed", Message: "Finding re-detected after resolution"},
		},
	}

	alreadyMigrated := &Finding{
		ID:               "already",
		Severity:         FindingSeverityWarning,
		ResourceID:       "vm-already",
		Title:            "Backup failed",
		Source:           "ai-analysis",
		Category:         FindingCategoryBackup,
		LastSeenAt:       now,
		RegressionCount:  4,
		LastRegressionAt: &lastRegress,
		Lifecycle: []FindingLifecycleEvent{
			{At: now.Add(-3 * time.Hour), Type: "auto_resolved", Message: "No longer detected by patrol"},
			{At: now.Add(-2 * time.Hour), Type: "regression_counter_reset"},
			// Genuine regressions accrued after the migration must be kept.
			{At: now.Add(-90 * time.Minute), Type: "regressed"},
		},
	}

	p := &recordingPersistence{
		findings: map[string]*Finding{
			withBogusCycle.ID:  withBogusCycle,
			withRealCycle.ID:   withRealCycle,
			alreadyMigrated.ID: alreadyMigrated,
		},
		saved: saved,
	}

	if err := store.SetPersistence(p); err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	bogus := store.Get("with-bogus")
	if bogus.RegressionCount != 0 {
		t.Fatalf("expected polluted regression counter reset to 0, got %d", bogus.RegressionCount)
	}
	if bogus.LastRegressionAt != nil {
		t.Fatal("expected LastRegressionAt cleared on reset")
	}
	foundResetEvent := false
	for _, e := range bogus.Lifecycle {
		if e.Type == "regression_counter_reset" {
			foundResetEvent = true
			break
		}
	}
	if !foundResetEvent {
		t.Fatal("expected regression_counter_reset lifecycle event on the migrated finding")
	}

	real := store.Get("with-real")
	if real.RegressionCount != 2 {
		t.Fatalf("finding without bogus signature must keep its regression counter; got %d", real.RegressionCount)
	}
	for _, e := range real.Lifecycle {
		if e.Type == "regression_counter_reset" {
			t.Fatal("finding without bogus signature must not gain a reset event")
		}
	}

	already := store.Get("already")
	if already.RegressionCount != 4 {
		t.Fatalf("already-migrated finding must not be reset again; got %d", already.RegressionCount)
	}
	resetCount := 0
	for _, e := range already.Lifecycle {
		if e.Type == "regression_counter_reset" {
			resetCount++
		}
	}
	if resetCount != 1 {
		t.Fatalf("already-migrated finding must keep exactly one reset event, got %d", resetCount)
	}

	select {
	case <-saved:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for migration save")
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

	// Wait for the error to be recorded
	time.Sleep(50 * time.Millisecond)

	// Check that error is tracked
	lastErr, _, _ := store.GetPersistenceStatus()
	if lastErr == nil {
		t.Error("expected lastSaveError to be set after save failure")
	}
}

func TestFindingsStore_SetOnSaveError(t *testing.T) {
	store := NewFindingsStore()
	store.saveDebounce = 5 * time.Millisecond
	saved := make(chan map[string]*Finding, 1)
	errReceived := make(chan error, 1)

	store.persistence = &recordingPersistence{
		saveErr: errors.New("callback test error"),
		saved:   saved,
	}

	// Set error callback
	store.SetOnSaveError(func(err error) {
		errReceived <- err
	})

	store.findings["real-1"] = &Finding{ID: "real-1", Title: "Real"}
	store.scheduleSave()

	select {
	case <-saved:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for SaveFindings")
	}

	select {
	case err := <-errReceived:
		if err == nil {
			t.Error("expected error to be passed to callback")
		}
		if err.Error() != "callback test error" {
			t.Errorf("expected 'callback test error', got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for error callback")
	}
}

func TestFindingsStore_GetPersistenceStatus_NoPersistence(t *testing.T) {
	store := NewFindingsStore()

	lastErr, lastSaveTime, hasPersistence := store.GetPersistenceStatus()

	if lastErr != nil {
		t.Error("expected no error when no persistence")
	}
	if !lastSaveTime.IsZero() {
		t.Error("expected zero time when no persistence")
	}
	if hasPersistence {
		t.Error("expected hasPersistence to be false")
	}
}

func TestFindingsStore_GetPersistenceStatus_WithPersistence(t *testing.T) {
	store := NewFindingsStore()
	store.saveDebounce = 5 * time.Millisecond
	saved := make(chan map[string]*Finding, 1)
	store.persistence = &recordingPersistence{saved: saved}

	_, _, hasPersistence := store.GetPersistenceStatus()
	if !hasPersistence {
		t.Error("expected hasPersistence to be true")
	}

	// Trigger a successful save
	store.findings["real-1"] = &Finding{ID: "real-1", Title: "Real"}
	store.scheduleSave()

	select {
	case <-saved:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for SaveFindings")
	}

	// Wait for status to be updated
	time.Sleep(50 * time.Millisecond)

	lastErr, lastSaveTime, _ := store.GetPersistenceStatus()
	if lastErr != nil {
		t.Errorf("expected no error after successful save, got %v", lastErr)
	}
	if lastSaveTime.IsZero() {
		t.Error("expected lastSaveTime to be set after successful save")
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
