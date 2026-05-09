package unified

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func TestUnifiedStore_AddFromAlert(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-101",
		ResourceName:    "web-server",
		Node:            "pve1",
		Message:         "CPU usage is high",
		Value:           85.5,
		Threshold:       80.0,
		StartTime:       time.Now().Add(-5 * time.Minute),
		LastSeen:        time.Now(),
	}

	finding, isNew := store.AddFromAlert(alert)

	if !isNew {
		t.Error("Expected finding to be new")
	}

	if finding == nil {
		t.Fatal("Expected finding to be created")
	}

	if finding.Source != SourceThreshold {
		t.Errorf("Expected source %s, got %s", SourceThreshold, finding.Source)
	}

	if finding.AlertIdentifier != "alert-1" {
		t.Errorf("Expected alert identifier alert-1, got %s", finding.AlertIdentifier)
	}

	if finding.Category != CategoryPerformance {
		t.Errorf("Expected category %s, got %s", CategoryPerformance, finding.Category)
	}

	if finding.Severity != SeverityWarning {
		t.Errorf("Expected severity %s, got %s", SeverityWarning, finding.Severity)
	}

	if finding.ResourceID != "vm-101" {
		t.Errorf("Expected resource ID vm-101, got %s", finding.ResourceID)
	}

	// Impact text is authored per alert type at detection time so the operator
	// sees consequence-if-ignored copy regardless of whether an AI investigation
	// has run.
	if finding.Impact == "" {
		t.Error("Expected detection-time impact text on cpu alert finding, got empty string")
	}
}

func TestGenerateImpact_AuthorsConsequenceForKnownAlertTypes(t *testing.T) {
	cases := []struct {
		alertType   string
		mustContain string
	}{
		{"cpu", "slow down"},
		{"memory", "OOM"},
		{"disk", "fail"},
		{"usage", "writes will start failing"},
		{"storage", "writes will start failing"},
		{"temperature", "throttles"},
		{"offline", "unreachable"},
		{"nodeOffline", "unreachable"},
		{"poweredOff", "unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.alertType, func(t *testing.T) {
			impact := generateImpact(tc.alertType)
			if impact == "" {
				t.Fatalf("expected non-empty impact for %q", tc.alertType)
			}
			if !strings.Contains(strings.ToLower(impact), strings.ToLower(tc.mustContain)) {
				t.Fatalf("impact for %q missing expected token %q: %s", tc.alertType, tc.mustContain, impact)
			}
		})
	}
}

func TestGenerateImpact_ReturnsEmptyForUnknownAlertType(t *testing.T) {
	// Unknown alert types return an empty string rather than synthesizing
	// generic impact copy. The contract is that impact is hand-authored per
	// finding source; if the source has no curated text, the operator sees
	// nothing rather than fabricated analysis.
	if got := generateImpact("totally-unknown-alert-kind"); got != "" {
		t.Fatalf("expected empty impact for unknown alert type, got %q", got)
	}
}

func TestUnifiedStore_AddFromAlert_Update(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-101",
		ResourceName:    "web-server",
		Value:           85.5,
		Threshold:       80.0,
		StartTime:       time.Now().Add(-5 * time.Minute),
		LastSeen:        time.Now(),
	}

	// First add
	finding1, isNew1 := store.AddFromAlert(alert)
	if !isNew1 {
		t.Error("First add should be new")
	}

	// Update the alert
	alert.Value = 90.0
	alert.LastSeen = time.Now()

	// Second add (should update)
	finding2, isNew2 := store.AddFromAlert(alert)
	if isNew2 {
		t.Error("Second add should not be new")
	}

	if finding2.ID != finding1.ID {
		t.Error("Should return the same finding")
	}

	if finding2.TimesRaised != 2 {
		t.Errorf("Expected times raised to be 2, got %d", finding2.TimesRaised)
	}
}

func TestUnifiedStore_ResolveByAlert(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-101",
		ResourceName:    "web-server",
		Value:           85.5,
		Threshold:       80.0,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}

	store.AddFromAlert(alert)

	// Resolve the alert
	resolved := store.ResolveByAlertIdentifier("alert-1")
	if !resolved {
		t.Error("Expected resolve to succeed")
	}

	// Check finding is resolved
	finding := store.GetByAlertIdentifier("alert-1")
	if finding.ResolvedAt == nil {
		t.Error("Expected finding to be resolved")
	}

	// Resolving again should return false
	resolved2 := store.ResolveByAlertIdentifier("alert-1")
	if resolved2 {
		t.Error("Expected resolve to fail on already resolved finding")
	}
}

func TestUnifiedStore_AddFromAI(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	finding := &UnifiedFinding{
		ID:           "ai-finding-1",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryCapacity,
		ResourceID:   "storage-local",
		ResourceName: "local-zfs",
		ResourceType: "storage",
		Title:        "Storage filling up",
		Description:  "Storage is at 85% capacity",
	}

	result, isNew := store.AddFromAI(finding)

	if !isNew {
		t.Error("Expected finding to be new")
	}

	if result.Source != SourceAIPatrol {
		t.Errorf("Expected source %s, got %s", SourceAIPatrol, result.Source)
	}

	// Check it's in the store
	retrieved := store.Get("ai-finding-1")
	if retrieved == nil {
		t.Error("Expected to retrieve finding")
	}
}

func TestUnifiedStore_AddFromAI_MergesInvestigationRecord(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	initial := &UnifiedFinding{
		ID:           "finding-1",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryPerformance,
		ResourceID:   "vm-100",
		ResourceName: "vm-100",
		Title:        "High CPU",
		Description:  "CPU high",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
	}
	store.AddFromAI(initial)

	record := &aicontracts.InvestigationRecord{
		ID:        "investigation-1",
		FindingID: "finding-1",
		Status:    aicontracts.InvestigationStatusCompleted,
		Outcome:   aicontracts.OutcomeFixQueued,
		Evidence:  []aicontracts.InvestigationRecordEvidence{},
		ToolsUsed: []string{},
	}
	updated, isNew := store.AddFromAI(&UnifiedFinding{
		ID:                  "finding-1",
		Source:              SourceAIPatrol,
		Severity:            SeverityWarning,
		Category:            CategoryPerformance,
		ResourceID:          "vm-100",
		ResourceName:        "vm-100",
		Title:               "High CPU",
		Description:         "CPU high",
		InvestigationRecord: record,
		DetectedAt:          time.Now(),
		LastSeenAt:          time.Now(),
	})
	if isNew {
		t.Fatal("expected merge into existing finding")
	}
	if updated.InvestigationRecord == nil || updated.InvestigationRecord.ID != "investigation-1" {
		t.Fatalf("expected investigation record to merge, got %#v", updated.InvestigationRecord)
	}
}

func TestUnifiedStore_GetBySource(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	// Add threshold alert
	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-101",
		ResourceName:    "web-server",
		Value:           85.5,
		Threshold:       80.0,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}
	store.AddFromAlert(alert)

	// Add AI finding
	aiFinding := &UnifiedFinding{
		ID:           "ai-finding-1",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryCapacity,
		ResourceID:   "storage-local",
		ResourceName: "local-zfs",
		Title:        "Storage filling up",
	}
	store.AddFromAI(aiFinding)

	// Get by source
	thresholdFindings := store.GetBySource(SourceThreshold)
	if len(thresholdFindings) != 1 {
		t.Errorf("Expected 1 threshold finding, got %d", len(thresholdFindings))
	}

	aiFindings := store.GetBySource(SourceAIPatrol)
	if len(aiFindings) != 1 {
		t.Errorf("Expected 1 AI finding, got %d", len(aiFindings))
	}
}

func TestUnifiedStore_EnhanceWithAI(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-101",
		ResourceName:    "web-server",
		Value:           85.5,
		Threshold:       80.0,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)

	// Enhance with AI
	success := store.EnhanceWithAI(
		finding.ID,
		"High CPU is likely caused by backup process running on host",
		0.85,
		"root-cause-123",
		[]string{"finding-456"},
	)

	if !success {
		t.Error("Expected enhance to succeed")
	}

	// Check enhancement
	enhanced := store.Get(finding.ID)
	if !enhanced.EnhancedByAI {
		t.Error("Expected finding to be marked as AI enhanced")
	}

	if enhanced.AIContext == "" {
		t.Error("Expected AI context to be set")
	}

	if enhanced.AIConfidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", enhanced.AIConfidence)
	}

	if enhanced.RootCauseID != "root-cause-123" {
		t.Errorf("Expected root cause ID root-cause-123, got %s", enhanced.RootCauseID)
	}
}

func TestUnifiedStore_Dismiss(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-101",
		ResourceName:    "web-server",
		Value:           85.5,
		Threshold:       80.0,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)

	// Dismiss
	success := store.Dismiss(finding.ID, "expected_behavior", "This VM is CPU-intensive by design")

	if !success {
		t.Error("Expected dismiss to succeed")
	}

	// Check dismissal
	dismissed := store.Get(finding.ID)
	if dismissed.DismissedReason != "expected_behavior" {
		t.Errorf("Expected dismissed reason 'expected_behavior', got '%s'", dismissed.DismissedReason)
	}

	if dismissed.UserNote == "" {
		t.Error("Expected user note to be set")
	}

	// Should not be active
	if dismissed.IsActive() {
		t.Error("Expected finding to not be active after dismissal")
	}
}

func TestUnifiedStore_Snooze(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	alert := &SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "warning",
		ResourceID:      "vm-101",
		ResourceName:    "web-server",
		Value:           85.5,
		Threshold:       80.0,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)

	// Snooze for 1 hour
	success := store.Snooze(finding.ID, time.Hour)

	if !success {
		t.Error("Expected snooze to succeed")
	}

	// Check snooze
	snoozed := store.Get(finding.ID)
	if !snoozed.IsSnoozed() {
		t.Error("Expected finding to be snoozed")
	}

	// Should not be active
	if snoozed.IsActive() {
		t.Error("Expected finding to not be active while snoozed")
	}
}

func TestUnifiedStore_GetSummary(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())

	// Add some findings
	store.AddFromAlert(&SimpleAlertAdapter{
		AlertIdentifier: "alert-1",
		AlertType:       "cpu",
		AlertLevel:      "critical",
		ResourceID:      "vm-101",
		Value:           95.0,
		Threshold:       80.0,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	})

	store.AddFromAlert(&SimpleAlertAdapter{
		AlertIdentifier: "alert-2",
		AlertType:       "memory",
		AlertLevel:      "warning",
		ResourceID:      "vm-102",
		Value:           88.0,
		Threshold:       85.0,
		StartTime:       time.Now(),
		LastSeen:        time.Now(),
	})

	store.AddFromAI(&UnifiedFinding{
		ID:         "ai-1",
		Source:     SourceAIPatrol,
		Severity:   SeverityWarning,
		Category:   CategoryCapacity,
		ResourceID: "storage-1",
		Title:      "Storage issue",
	})

	summary := store.GetSummary()

	if summary.Active != 3 {
		t.Errorf("Expected 3 active, got %d", summary.Active)
	}

	if summary.Critical != 1 {
		t.Errorf("Expected 1 critical, got %d", summary.Critical)
	}

	if summary.Warning != 2 {
		t.Errorf("Expected 2 warning, got %d", summary.Warning)
	}

	if summary.BySource[SourceThreshold] != 2 {
		t.Errorf("Expected 2 threshold findings, got %d", summary.BySource[SourceThreshold])
	}

	if summary.BySource[SourceAIPatrol] != 1 {
		t.Errorf("Expected 1 AI patrol finding, got %d", summary.BySource[SourceAIPatrol])
	}
}

func TestUnifiedFinding_IsActive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		finding  UnifiedFinding
		expected bool
	}{
		{
			name:     "active finding",
			finding:  UnifiedFinding{},
			expected: true,
		},
		{
			name: "resolved finding",
			finding: UnifiedFinding{
				ResolvedAt: &now,
			},
			expected: false,
		},
		{
			name: "snoozed finding",
			finding: UnifiedFinding{
				SnoozedUntil: func() *time.Time { t := now.Add(time.Hour); return &t }(),
			},
			expected: false,
		},
		{
			name: "dismissed finding",
			finding: UnifiedFinding{
				DismissedReason: "not_an_issue",
			},
			expected: false,
		},
		{
			name: "suppressed finding",
			finding: UnifiedFinding{
				Suppressed: true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.finding.IsActive(); got != tt.expected {
				t.Errorf("IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCategoryMapping(t *testing.T) {
	config := DefaultAlertToFindingConfig()

	tests := []struct {
		alertType        string
		expectedCategory UnifiedCategory
	}{
		{"cpu", CategoryPerformance},
		{"memory", CategoryPerformance},
		{"disk", CategoryCapacity},
		{"storage", CategoryCapacity},
		{"temperature", CategoryReliability},
		{"offline", CategoryConnectivity},
		{"backup", CategoryBackup},
	}

	for _, tt := range tests {
		t.Run(tt.alertType, func(t *testing.T) {
			category, ok := config.TypeCategoryMap[tt.alertType]
			if !ok {
				t.Errorf("Expected category mapping for %s", tt.alertType)
				return
			}
			if category != tt.expectedCategory {
				t.Errorf("Expected category %s for type %s, got %s", tt.expectedCategory, tt.alertType, category)
			}
		})
	}
}

func TestUnifiedStore_AddFromAI_MirrorsAutoResolved(t *testing.T) {
	// AutoResolved distinguishes Pulse's own auto-detection of the condition
	// clearing from an operator-driven manual Mark resolved. The unified
	// store mirror must propagate that flag onto the existing record so the
	// API surface can attribute the closure honestly. This test pins the
	// dedup-merge AutoResolved propagation.
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	store.AddFromAI(&UnifiedFinding{
		ID:           "ai-auto",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryReliability,
		ResourceID:   "vm-101",
		ResourceName: "db-01",
		ResourceType: "vm",
		Title:        "CPU sustained high",
	})
	resolved := time.Date(2026, 5, 9, 11, 0, 0, 0, time.UTC)

	// First resolution: Pulse's auto-detection cleared the condition.
	store.AddFromAI(&UnifiedFinding{
		ID:           "ai-auto",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryReliability,
		ResourceID:   "vm-101",
		ResourceName: "db-01",
		ResourceType: "vm",
		Title:        "CPU sustained high",
		ResolvedAt:   &resolved,
		AutoResolved: true,
	})
	got := store.Get("ai-auto")
	if got == nil {
		t.Fatal("expected finding to exist after merge")
	}
	if !got.AutoResolved {
		t.Errorf("AutoResolved must mirror true through dedup-merge; got %v", got.AutoResolved)
	}

	// Second resolution: operator manually re-resolved (e.g. after a
	// regression they fixed out-of-band). The flag must flip to false so
	// the surface stops attributing closure to Pulse's auto-detection.
	store.AddFromAI(&UnifiedFinding{
		ID:           "ai-auto",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryReliability,
		ResourceID:   "vm-101",
		ResourceName: "db-01",
		ResourceType: "vm",
		Title:        "CPU sustained high",
		ResolvedAt:   &resolved,
		AutoResolved: false,
	})
	got = store.Get("ai-auto")
	if got == nil {
		t.Fatal("expected finding to still exist after manual close")
	}
	if got.AutoResolved {
		t.Errorf("AutoResolved must mirror false on manual close; got %v", got.AutoResolved)
	}
}

func TestUnifiedStore_AddFromAI_MirrorsRemindAt(t *testing.T) {
	// RemindAt is the will_fix_later wake-up deadline persisted on the
	// canonical findings store. The unified store mirrors finding state for
	// the API surface, so AddFromAI must propagate RemindAt on the update
	// branch — including clearing it on remind-at wake or undismiss —
	// otherwise dismissed-as-will_fix_later rows on the operator surface
	// would either show stale reminders or no reminder at all.
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	store.AddFromAI(&UnifiedFinding{
		ID:           "ai-wfl",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryReliability,
		ResourceID:   "vm-101",
		ResourceName: "db-01",
		ResourceType: "vm",
		Title:        "Disk pressure",
		Description:  "Will fix during the Q3 storage upgrade.",
	})

	// Re-add with the will_fix_later commitment populated; the dedup-merge
	// path must mirror RemindAt onto the existing record.
	when := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)
	store.AddFromAI(&UnifiedFinding{
		ID:              "ai-wfl",
		Source:          SourceAIPatrol,
		Severity:        SeverityWarning,
		Category:        CategoryReliability,
		ResourceID:      "vm-101",
		ResourceName:    "db-01",
		ResourceType:    "vm",
		Title:           "Disk pressure",
		Description:     "Will fix during the Q3 storage upgrade.",
		DismissedReason: "will_fix_later",
		RemindAt:        &when,
	})

	got := store.Get("ai-wfl")
	if got == nil {
		t.Fatal("expected finding to exist after merge")
	}
	if got.DismissedReason != "will_fix_later" {
		t.Errorf("expected DismissedReason mirrored, got %q", got.DismissedReason)
	}
	if got.RemindAt == nil {
		t.Fatal("RemindAt must be mirrored onto the existing record")
	}
	if !got.RemindAt.Equal(when) {
		t.Errorf("RemindAt mismatch: got %v want %v", got.RemindAt, when)
	}

	// Re-add without RemindAt (simulates remind-at wake clearing the
	// dismissal in the canonical store) — the mirror must clear too,
	// not preserve the stale deadline.
	store.AddFromAI(&UnifiedFinding{
		ID:           "ai-wfl",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryReliability,
		ResourceID:   "vm-101",
		ResourceName: "db-01",
		ResourceType: "vm",
		Title:        "Disk pressure",
		Description:  "Reawakened — operator commitment lapsed",
	})
	if got = store.Get("ai-wfl"); got == nil {
		t.Fatal("expected finding to still exist after wake")
	}
	if got.RemindAt != nil {
		t.Errorf("RemindAt must be cleared on wake, got %v", got.RemindAt)
	}
	if got.DismissedReason != "" {
		t.Errorf("DismissedReason must be cleared on wake, got %q", got.DismissedReason)
	}
}
