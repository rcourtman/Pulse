package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
)

func TestIsDemoMode(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })

	mockruntime.SetEnabled(true)
	if !IsDemoMode() {
		t.Fatal("expected demo mode true when runtime mock mode is enabled")
	}

	mockruntime.SetEnabled(false)
	if IsDemoMode() {
		t.Fatal("expected demo mode false when runtime mock mode is disabled")
	}
}

func TestPatrolService_InjectDemoFindings(t *testing.T) {
	service := NewPatrolService(nil, nil)
	if service.findings == nil || service.runHistoryStore == nil {
		t.Fatal("expected findings and run history to be initialized")
	}

	service.InjectDemoFindings()

	findings := service.findings.GetAll(nil)
	if len(findings) != 6 {
		t.Fatalf("expected 6 demo findings, got %d", len(findings))
	}
	if service.runHistoryStore.Count() != 13 {
		t.Fatalf("expected 13 demo run history entries, got %d", service.runHistoryStore.Count())
	}
}

func TestPatrolService_InjectDemoFindings_NoStore(t *testing.T) {
	service := &PatrolService{}
	service.InjectDemoFindings()
}

func TestPatrolService_InjectDemoRunHistory_NoStore(t *testing.T) {
	service := &PatrolService{}
	service.injectDemoRunHistory()
}

func TestPatrolRunHistoryFiltersDemoEvidenceOutsideDemoMode(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(false)

	service := NewPatrolService(nil, nil)
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	service.runHistoryStore.Add(PatrolRunRecord{
		ID:               "live-run-1",
		StartedAt:        now.Add(-15 * time.Minute),
		CompletedAt:      now.Add(-14 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 12,
		FindingsSummary:  "No issues found",
		FindingIDs:       []string{},
		Status:           "healthy",
	})
	service.runHistoryStore.Add(PatrolRunRecord{
		ID:               "demo-run-legacy",
		StartedAt:        now.Add(-5 * time.Minute),
		CompletedAt:      now.Add(-4 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 47,
		ExistingFindings: 5,
		FindingsSummary:  "2 critical, 3 warnings",
		FindingIDs:       []string{"demo-storage-critical"},
		Status:           "issues_found",
	})

	runs := service.GetRunHistory(1)
	if len(runs) != 1 {
		t.Fatalf("expected one live run after filtering demo evidence, got %d", len(runs))
	}
	if runs[0].ID != "live-run-1" {
		t.Fatalf("expected live-run-1 after filtering demo evidence, got %q", runs[0].ID)
	}
	if _, ok := service.GetRunByID("demo-run-legacy"); ok {
		t.Fatal("expected legacy demo run lookup to be hidden outside demo mode")
	}
}

func TestPatrolRunHistoryKeepsDemoEvidenceInDemoMode(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(true)

	service := NewPatrolService(nil, nil)
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	service.runHistoryStore.Add(PatrolRunRecord{
		ID:               "live-run-1",
		StartedAt:        now.Add(-15 * time.Minute),
		CompletedAt:      now.Add(-14 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 12,
		FindingsSummary:  "No issues found",
		FindingIDs:       []string{},
		Status:           "healthy",
	})
	service.runHistoryStore.Add(PatrolRunRecord{
		ID:               "demo-run-1",
		Source:           PatrolRunSourceDemo,
		StartedAt:        now.Add(-5 * time.Minute),
		CompletedAt:      now.Add(-4 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 47,
		ExistingFindings: 5,
		FindingsSummary:  "2 critical, 3 warnings",
		FindingIDs:       []string{"demo-storage-critical"},
		Status:           "issues_found",
	})

	runs := service.GetRunHistory(1)
	if len(runs) != 1 {
		t.Fatalf("expected one run, got %d", len(runs))
	}
	if runs[0].ID != "demo-run-1" || runs[0].Source != PatrolRunSourceDemo {
		t.Fatalf("expected current demo run in demo mode, got %+v", runs[0])
	}
	if _, ok := service.GetRunByID("demo-run-1"); !ok {
		t.Fatal("expected demo run lookup to be available in demo mode")
	}
}

func TestPatrolCoverageIgnoresPersistedDemoRunsOutsideDemoMode(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })
	mockruntime.SetEnabled(false)

	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	store := NewPatrolRunHistoryStore(10)
	store.Add(PatrolRunRecord{
		ID:               "demo-run-legacy",
		StartedAt:        now.Add(-5 * time.Minute),
		CompletedAt:      now.Add(-4 * time.Minute),
		Type:             "patrol",
		ResourcesChecked: 47,
		ErrorCount:       1,
		Status:           "error",
		FindingsSummary:  "2 critical, 3 warnings",
		FindingIDs:       []string{"demo-storage-critical"},
	})

	if factor, ok := summarizeRecentPatrolCoverage(store, now); ok {
		t.Fatalf("expected persisted demo run to be excluded from coverage scoring, got %+v", factor)
	}
}

func TestGenerateDemoAIResponse(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		{"patrol", "Analyze the infrastructure for issues", "ZFS pool 'local-zfs' is 94% full"},
		{"disk", "disk is full", "Disk Usage Analysis"},
		{"memory", "memory pressure", "Memory Analysis"},
		{"backup", "pbs backup status", "Backup Status Review"},
		{"cpu", "cpu load is high", "CPU/Performance Analysis"},
		{"hello", "hello there", "Pulse Assistant"},
		{"default", "status report", "This Demo Shows"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := GenerateDemoAIResponse(tt.prompt)
			if resp == nil {
				t.Fatal("expected response")
			}
			if !strings.Contains(resp.Content, tt.expected) {
				t.Fatalf("expected response to contain %q, got %q", tt.expected, resp.Content)
			}
			if resp.Model == "" {
				t.Fatal("expected model to be set")
			}
		})
	}
}

func TestGenerateDemoAIStream(t *testing.T) {
	var content strings.Builder
	done := false

	resp, err := GenerateDemoAIStream("disk usage", func(event StreamEvent) {
		switch event.Type {
		case "content":
			chunk, ok := event.Data.(string)
			if !ok {
				t.Fatalf("expected string content chunk, got %T", event.Data)
			}
			content.WriteString(chunk)
		case "done":
			done = true
		}
	})
	if err != nil {
		t.Fatalf("GenerateDemoAIStream failed: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if !done {
		t.Fatal("expected done event")
	}
	if content.String() != resp.Content {
		t.Fatal("expected streamed content to match response content")
	}
}
