package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestFindingsStore_PersistsManualSuppressionRules(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	store := NewFindingsStore()
	if err := store.SetPersistence(adapter); err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	rule := store.AddSuppressionRule("res-1", "Resource 1", FindingCategoryPerformance, "Manual suppression")
	if rule == nil {
		t.Fatal("AddSuppressionRule returned nil")
	}
	if err := store.ForceSave(); err != nil {
		t.Fatalf("ForceSave failed: %v", err)
	}

	store2 := NewFindingsStore()
	if err := store2.SetPersistence(NewFindingsPersistenceAdapter(persistence)); err != nil {
		t.Fatalf("SetPersistence (reload) failed: %v", err)
	}
	if !store2.MatchesSuppressionRule("res-1", FindingCategoryPerformance) {
		t.Fatalf("expected suppression rule to persist across restart")
	}

	found := false
	for _, r := range store2.GetSuppressionRules() {
		if r != nil && r.ID == rule.ID {
			found = true
			if r.Description != "Manual suppression" {
				t.Fatalf("expected persisted rule description, got %q", r.Description)
			}
			if r.CreatedFrom != "manual" {
				t.Fatalf("expected persisted rule created_from=manual, got %q", r.CreatedFrom)
			}
		}
	}
	if !found {
		t.Fatalf("expected to find persisted rule ID %q in GetSuppressionRules()", rule.ID)
	}
}

func TestFindingsStore_PersistsSuppressionFromSuppressAction(t *testing.T) {
	tmp := t.TempDir()
	persistence := config.NewConfigPersistence(tmp)
	adapter := NewFindingsPersistenceAdapter(persistence)

	store := NewFindingsStore()
	if err := store.SetPersistence(adapter); err != nil {
		t.Fatalf("SetPersistence failed: %v", err)
	}

	now := time.Now()
	f := &Finding{
		ID:           "f1",
		Key:          "cpu-high",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "res-1",
		ResourceName: "Resource 1",
		ResourceType: "node",
		Title:        "High CPU",
		Description:  "CPU is high",
		DetectedAt:   now,
		LastSeenAt:   now,
	}
	store.Add(f)

	if ok := store.Suppress("f1"); !ok {
		t.Fatalf("expected Suppress to return true")
	}
	if err := store.ForceSave(); err != nil {
		t.Fatalf("ForceSave failed: %v", err)
	}

	store2 := NewFindingsStore()
	if err := store2.SetPersistence(NewFindingsPersistenceAdapter(persistence)); err != nil {
		t.Fatalf("SetPersistence (reload) failed: %v", err)
	}
	if !store2.MatchesSuppressionRule("res-1", FindingCategoryPerformance) {
		t.Fatalf("expected suppress-derived rule to persist across restart")
	}

	// Confirm we have at least one explicit persisted rule created from suppression.
	hasExplicitSuppression := false
	for _, r := range store2.GetSuppressionRules() {
		if r != nil && r.CreatedFrom == "suppress" && r.ResourceID == "res-1" && r.Category == FindingCategoryPerformance {
			hasExplicitSuppression = true
			break
		}
	}
	if !hasExplicitSuppression {
		t.Fatalf("expected an explicit persisted suppression rule with created_from=suppress")
	}
}
