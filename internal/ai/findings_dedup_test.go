package ai

import (
	"testing"
)

func TestSemanticSimilarity(t *testing.T) {
	// Base finding
	f1 := &Finding{
		ResourceID:  "res-1",
		Category:    FindingCategoryPerformance,
		Title:       "High CPU Usage",
		Description: "CPU is at 99%",
	}

	tests := []struct {
		name     string
		f2       *Finding
		minScore float64 // Expected minimum similarity score
		maxScore float64 // Expected maximum similarity score
	}{
		{
			name:     "Nil finding",
			f2:       nil,
			minScore: 0.0,
			maxScore: 0.0,
		},
		{
			name: "Different resource and category",
			f2: &Finding{
				ResourceID:  "res-2",
				Category:    FindingCategorySecurity,
				Title:       "Login failed",
				Description: "Auth error",
			},
			minScore: 0.0,
			maxScore: 0.1, // Maybe some keyword overlap
		},
		{
			name: "Same resource, different category",
			f2: &Finding{
				ResourceID:  "res-1",
				Category:    FindingCategorySecurity,
				Title:       "Login failed",
				Description: "Auth error",
			},
			minScore: 0.3, // 0.3 for resource
			maxScore: 0.4,
		},
		{
			name: "Different resource, same category",
			f2: &Finding{
				ResourceID:  "res-2",
				Category:    FindingCategoryPerformance,
				Title:       "High Memory Usage",
				Description: "RAM is full",
			},
			minScore: 0.2, // 0.2 for category
			maxScore: 0.4, // float precision might nudge it slightly over 0.2
		},
		{
			name: "Same resource and category",
			f2: &Finding{
				ResourceID:  "res-1",
				Category:    FindingCategoryPerformance,
				Title:       "Different Issue",
				Description: "Something else",
			},
			minScore: 0.5, // 0.3 + 0.2
			maxScore: 0.6,
		},
		{
			name: "Similar title and description",
			f2: &Finding{
				ResourceID:  "res-2",
				Category:    FindingCategoryPerformance,
				Title:       "High CPU Usage",
				Description: "CPU is at 99%",
			},
			minScore: 0.4, // Title/description boost
			maxScore: 1.0,
		},
		{
			name: "Exact duplicate",
			f2: &Finding{
				ResourceID:  "res-1",
				Category:    FindingCategoryPerformance,
				Title:       "High CPU Usage",
				Description: "CPU is at 99%",
			},
			minScore: 0.7, // Max is 0.8 without Key (0.3+0.2+0.2+0.1)
			maxScore: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := SemanticSimilarity(f1, tt.f2)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("SemanticSimilarity() = %v, want between %v and %v", score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestFindingsStore_SemanticDeduplication(t *testing.T) {
	store := NewFindingsStore()

	f1 := &Finding{
		ID:          "f1",
		ResourceID:  "res-1",
		Category:    FindingCategoryPerformance,
		Title:       "High CPU detected",
		Description: "CPU usage is consistently above 90%",
		Severity:    FindingSeverityWarning,
		TimesRaised: 1,
	}

	// Add first finding
	id1, isNew := store.AddWithDeduplication(f1, 0.75)
	if !isNew {
		t.Error("Expected first finding to be new")
	}
	if id1 != "f1" {
		t.Errorf("Expected ID f1, got %s", id1)
	}

	// Add second finding (very similar)
	f2 := &Finding{
		ID:          "f2",
		ResourceID:  "res-1",
		Category:    FindingCategoryPerformance,
		Title:       "High CPU detected",
		Description: "CPU usage is consistently above 90%",
		Severity:    FindingSeverityCritical, // Escalation
		TimesRaised: 1,
	}

	id2, isNew := store.AddWithDeduplication(f2, 0.75)
	if isNew {
		t.Error("Expected similar finding to be deduplicated (not new)")
	}
	if id2 != "f1" { // Should return ID of existing finding
		t.Errorf("Expected deduplication to existing ID f1, got %s", id2)
	}

	// Verify merged state
	merged := store.Get("f1")
	if merged.TimesRaised != 2 {
		t.Errorf("Expected TimesRaised to increment to 2, got %d", merged.TimesRaised)
	}
	if merged.Severity != FindingSeverityCritical {
		t.Errorf("Expected severity to escalate to Critical, got %v", merged.Severity)
	}

	// Add distinct finding
	f3 := &Finding{
		ID:         "f3",
		ResourceID: "res-2", // Different resource
		Category:   FindingCategorySecurity,
		Title:      "SSH Login Failed",
		Severity:   FindingSeverityWarning,
	}

	id3, isNew := store.AddWithDeduplication(f3, 0.8)
	if !isNew {
		t.Error("Expected distinct finding to be new")
	}
	if id3 != "f3" {
		t.Errorf("Expected ID f3, got %s", id3)
	}
}

func TestFindingsStore_FindSimilarFindings(t *testing.T) {
	store := NewFindingsStore()

	store.Add(&Finding{
		ID:          "f1",
		ResourceID:  "res-1",
		Category:    FindingCategoryPerformance,
		Title:       "High CPU",
		Description: "High CPU usage detected",
	})

	store.Add(&Finding{
		ID:          "f2",
		ResourceID:  "res-2",
		Category:    FindingCategorySecurity,
		Title:       "Login Failed",
		Description: "Auth failure",
	})

	// Search for similar to f1
	query := &Finding{
		ResourceID:  "res-1",
		Category:    FindingCategoryPerformance,
		Title:       "High CPU",
		Description: "High CPU usage detected",
	}

	similar := store.FindSimilarFindings(query, 0.75)
	if len(similar) != 1 {
		t.Fatalf("Expected 1 similar finding, got %d", len(similar))
	}
	if similar[0].ID != "f1" {
		t.Errorf("Expected similarity to f1, got %s", similar[0].ID)
	}

	// Search for unrelated
	queryUnsupported := &Finding{
		ResourceID: "res-3",
		Category:   FindingCategoryCapacity, // Capacity != Performance
	}

	similar = store.FindSimilarFindings(queryUnsupported, 0.75)
	if len(similar) != 0 {
		t.Errorf("Expected 0 similar findings, got %d", len(similar))
	}
}
