package memory

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

// Additional tests to improve coverage

func TestChangeDetector_GetChangesSummary(t *testing.T) {
	d := &ChangeDetector{maxChanges: 100, changes: []Change{
		{ID: "c1", ResourceID: "vm-100", ResourceType: "vm", ResourceName: "web-server", ChangeType: ChangeCreated, DetectedAt: time.Now(), Description: "vm 'web-server' created"},
	}}

	since := time.Now().Add(-1 * time.Hour)
	summary := d.GetChangesSummary(since, 5)

	want := FormatRecentChangesContext(d.GetRecentChanges(5, since), false, "##")
	if summary == "" || summary != want {
		t.Fatalf("expected shared recent-changes formatter, got %q want %q", summary, want)
	}
}

func TestChangeDetector_GetChangesSummary_NoChanges(t *testing.T) {
	d := NewChangeDetector(ChangeDetectorConfig{MaxChanges: 100})

	// No changes yet
	summary := d.GetChangesSummary(time.Now().Add(-1*time.Hour), 5)

	if summary != "" {
		t.Errorf("Expected empty summary for no changes, got: %s", summary)
	}
}

func TestChangeDetector_LegacyHistoryIsReadOnly(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "ai_changes.json")
	legacy := []byte(`[{"id":"legacy-1","resource_id":"vm-100","resource_type":"vm","resource_name":"web-server","change_type":"created","detected_at":"2026-01-01T00:00:00Z","description":"vm 'web-server' created"}]`)
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatalf("write legacy history: %v", err)
	}

	d := NewChangeDetector(ChangeDetectorConfig{
		MaxChanges: 100,
		DataDir:    tmpDir,
	})

	// The legacy history is served through the read APIs.
	if got := d.GetChangesForResource("vm-100", 10); len(got) != 1 || got[0].ID != "legacy-1" {
		t.Fatalf("expected the legacy change, got %v", got)
	}
	if summary := d.GetChangesSummary(time.Time{}, 10); !containsStr(summary, "web-server") {
		t.Fatalf("expected the legacy change in the summary, got %q", summary)
	}

	// Nothing rewrites the file or adds to the data directory.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("read data dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "ai_changes.json" {
		t.Fatalf("data dir must hold only the legacy history, got %v", entries)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read legacy history: %v", err)
	}
	if !bytes.Equal(data, legacy) {
		t.Fatalf("legacy history was rewritten: %s", data)
	}
}

func TestRemediationLog_LogCommand(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	r.LogCommand(
		"vm-100",
		"vm",
		"web-server",
		"finding-123",
		"High CPU usage",
		"systemctl restart nginx",
		"Service restarted successfully",
		true,
		false,
	)

	records := r.GetForResource("vm-100", 10)
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if records[0].Action != "systemctl restart nginx" {
		t.Errorf("Expected action 'systemctl restart nginx', got '%s'", records[0].Action)
	}
	if records[0].Outcome != OutcomeResolved {
		t.Errorf("Expected OutcomeResolved for success, got %s", records[0].Outcome)
	}
}

func TestRemediationLog_LogCommand_Failed(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	r.LogCommand(
		"vm-100",
		"vm",
		"web-server",
		"finding-123",
		"High CPU usage",
		"systemctl restart nginx",
		"Error: service not found",
		false, // failed
		true,  // automatic
	)

	records := r.GetForResource("vm-100", 10)
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	if records[0].Outcome != OutcomeFailed {
		t.Errorf("Expected OutcomeFailed for failure, got %s", records[0].Outcome)
	}
	if !records[0].Automatic {
		t.Error("Expected Automatic to be true")
	}
}

func TestRemediationLog_GetForFinding(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	_ = r.Log(RemediationRecord{
		ResourceID: "vm-100",
		FindingID:  "finding-123",
		Problem:    "High memory usage",
		Action:     "Restart service",
		Outcome:    OutcomeResolved,
	})
	_ = r.Log(RemediationRecord{
		ResourceID: "vm-200",
		FindingID:  "finding-456",
		Problem:    "Disk full",
		Action:     "Cleanup logs",
		Outcome:    OutcomeResolved,
	})

	records := r.GetForFinding("finding-123", 10)
	if len(records) != 1 {
		t.Fatalf("Expected 1 record for finding-123, got %d", len(records))
	}
	if records[0].FindingID != "finding-123" {
		t.Errorf("Expected finding-123, got %s", records[0].FindingID)
	}
}

func TestRemediationLog_GetRecentRemediations(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	_ = r.Log(RemediationRecord{
		Problem: "Issue 1",
		Action:  "Action 1",
		Outcome: OutcomeResolved,
	})
	_ = r.Log(RemediationRecord{
		Problem: "Issue 2",
		Action:  "Action 2",
		Outcome: OutcomeResolved,
	})

	since := time.Now().Add(-1 * time.Hour)
	records := r.GetRecentRemediations(10, since)

	if len(records) != 2 {
		t.Errorf("Expected 2 recent records, got %d", len(records))
	}
}

func TestRemediationLog_FormatForContext(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	_ = r.Log(RemediationRecord{
		ResourceID: "vm-100",
		Problem:    "High memory usage",
		Action:     "Restart nginx",
		Outcome:    OutcomeResolved,
	})

	formatted := r.FormatForContext("vm-100", 5)

	if formatted == "" {
		t.Error("Expected non-empty formatted context")
	}
}

func TestRemediationLog_FormatForContext_NoRecords(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 100})

	formatted := r.FormatForContext("nonexistent", 5)

	if formatted != "" {
		t.Errorf("Expected empty formatted context for nonexistent resource, got: %s", formatted)
	}
}

func TestRemediationLog_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create log with persistence
	r := NewRemediationLog(RemediationLogConfig{
		MaxRecords: 100,
		DataDir:    tmpDir,
	})

	if err := r.Log(RemediationRecord{
		ResourceID: "vm-100",
		Problem:    "Test problem",
		Action:     "Test action",
		Outcome:    OutcomeResolved,
	}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	// Log persists before returning.
	filePath := filepath.Join(tmpDir, "ai_remediations.json")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("expected remediation file after Log: %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{30 * time.Second, "just now"}, // < 1 minute returns "just now"
		{1 * time.Second, "just now"},  // < 1 minute returns "just now"
		{5 * time.Minute, "5 minutes"},
		{1 * time.Minute, "1 minute"},
		{2 * time.Hour, "2 hours"},
		{1 * time.Hour, "1 hour"},
		{24 * time.Hour, "1 day"},
	}

	for _, tt := range tests {
		result := utils.FormatDuration(tt.input)
		if result != tt.expected {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"longer string", 5, "lo..."}, // truncates at maxLen-3 + "..."
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncateOutput(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateOutput(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestRemediationLog_TrimRecords(t *testing.T) {
	r := NewRemediationLog(RemediationLogConfig{MaxRecords: 3})

	// Add 5 records (exceeds max of 3)
	for i := 0; i < 5; i++ {
		_ = r.Log(RemediationRecord{
			ResourceID: "vm-100",
			Problem:    "Problem",
			Action:     "Action",
			Outcome:    OutcomeResolved,
		})
	}

	records := r.GetRecentRemediations(100, time.Time{})
	if len(records) > 3 {
		t.Errorf("Expected at most 3 records after trimming, got %d", len(records))
	}
}

// Helper function
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
