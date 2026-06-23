package ai

import "testing"

func TestFindingsToolAdapter(t *testing.T) {
	if NewFindingsToolAdapter(nil) != nil {
		t.Fatal("expected nil adapter for nil store")
	}

	store := NewFindingsStore()
	finding := &Finding{
		ID:           "f1",
		Key:          "k1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "vm-1",
		ResourceName: "vm1",
		Title:        "Issue",
	}
	store.Add(finding)
	store.Dismiss("f1", "not_an_issue", "ok")

	adapter := NewFindingsToolAdapter(store)
	active := adapter.GetActiveFindings()
	if len(active) != 0 {
		t.Fatalf("expected no active findings, got %d", len(active))
	}

	dismissed := adapter.GetDismissedFindings()
	if len(dismissed) != 1 || dismissed[0].ID != "f1" {
		t.Fatalf("unexpected dismissed findings: %+v", dismissed)
	}

	adapter = &FindingsToolAdapter{}
	if adapter.GetActiveFindings() != nil || adapter.GetDismissedFindings() != nil {
		t.Fatal("expected nil results when store missing")
	}
}
