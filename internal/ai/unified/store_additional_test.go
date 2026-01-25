package unified

import (
	"strings"
	"testing"
	"time"
)

type stubUnifiedPersistence struct {
	loadFindings map[string]*UnifiedFinding
	loadErr      error
	saved        map[string]*UnifiedFinding
	saveCalls    int
	saveErr      error
}

func (s *stubUnifiedPersistence) SaveFindings(findings map[string]*UnifiedFinding) error {
	s.saveCalls++
	s.saved = make(map[string]*UnifiedFinding, len(findings))
	for id, f := range findings {
		copy := *f
		s.saved[id] = &copy
	}
	return s.saveErr
}

func (s *stubUnifiedPersistence) LoadFindings() (map[string]*UnifiedFinding, error) {
	return s.loadFindings, s.loadErr
}

func TestUnifiedStore_SetPersistence_LoadsFindings(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	loaded := map[string]*UnifiedFinding{
		"f1": {
			ID:         "f1",
			Source:     SourceThreshold,
			ResourceID: "r1",
			AlertID:    "a1",
		},
		"f2": {
			ID:         "f2",
			Source:     SourceAIPatrol,
			ResourceID: "r1",
		},
	}
	persistence := &stubUnifiedPersistence{loadFindings: loaded}
	if err := store.SetPersistence(persistence); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.Get("f1") == nil {
		t.Fatalf("expected finding f1 to load")
	}
	if store.GetByAlert("a1") == nil {
		t.Fatalf("expected alert index to load")
	}
	byResource := store.GetByResource("r1")
	if len(byResource) != 2 {
		t.Fatalf("expected 2 findings for resource, got %d", len(byResource))
	}
}

func TestUnifiedStore_SetPersistence_Error(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	persistence := &stubUnifiedPersistence{loadErr: errTest}
	if err := store.SetPersistence(persistence); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUnifiedStore_ConvertAlert_MetadataAndRecommendation(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "disk",
		AlertLevel:   "critical",
		ResourceID:   "vm-1",
		ResourceName: "db",
		Message:      "disk full",
		Value:        96,
		Threshold:    90,
		StartTime:    time.Now().Add(-time.Minute),
		LastSeen:     time.Now(),
		Metadata:     map[string]interface{}{"resourceType": "node"},
	}

	finding := store.ConvertAlert(alert)
	if finding.Severity != SeverityCritical {
		t.Fatalf("expected critical severity")
	}
	if finding.Category != CategoryCapacity {
		t.Fatalf("expected capacity category")
	}
	if finding.ResourceType != "node" {
		t.Fatalf("expected resource type from metadata")
	}
	if !strings.Contains(finding.Recommendation, "URGENT") {
		t.Fatalf("expected urgent recommendation, got %q", finding.Recommendation)
	}
}

func TestUnifiedStore_AddFromAlert_ReopensResolved(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "cpu",
		AlertLevel:   "warning",
		ResourceID:   "vm-1",
		ResourceName: "web",
		Value:        90,
		Threshold:    80,
		StartTime:    time.Now().Add(-time.Minute),
		LastSeen:     time.Now(),
	}

	finding, _ := store.AddFromAlert(alert)
	now := time.Now().Add(-time.Second)
	store.findings[finding.ID].ResolvedAt = &now

	alert.AlertLevel = "critical"
	updated, isNew := store.AddFromAlert(alert)
	if isNew {
		t.Fatalf("expected update for existing alert")
	}
	if updated.ResolvedAt != nil {
		t.Fatalf("expected resolvedAt cleared")
	}
	if updated.Severity != SeverityCritical {
		t.Fatalf("expected severity to upgrade")
	}
}

func TestUnifiedStore_AddFromAI_Existing(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	initial := &UnifiedFinding{
		ID:         "ai-1",
		Source:     SourceAIPatrol,
		Severity:   SeverityWatch,
		ResourceID: "res-1",
		Title:      "initial",
	}
	store.AddFromAI(initial)

	update := &UnifiedFinding{
		ID:          "ai-1",
		Source:      SourceAIPatrol,
		Severity:    SeverityCritical,
		ResourceID:  "res-1",
		Description: "updated",
	}
	updated, isNew := store.AddFromAI(update)
	if isNew {
		t.Fatalf("expected existing update")
	}
	if updated.Severity != SeverityCritical {
		t.Fatalf("expected severity update")
	}
	if updated.Description != "updated" {
		t.Fatalf("expected description update")
	}
}

func TestUnifiedStore_BasicMutations(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "cpu",
		AlertLevel:   "warning",
		ResourceID:   "vm-1",
		ResourceName: "web",
		Value:        90,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)

	if store.ResolveByAlert("missing") {
		t.Fatalf("expected resolve to fail for missing alert")
	}
	if store.EnhanceWithAI("missing", "ctx", 0.5, "", nil) {
		t.Fatalf("expected enhance to fail for missing finding")
	}
	if store.LinkRemediation("missing", "plan") {
		t.Fatalf("expected link remediation to fail for missing finding")
	}

	if !store.Acknowledge(finding.ID) {
		t.Fatalf("expected acknowledge to succeed")
	}
	if !store.Snooze(finding.ID, time.Minute) {
		t.Fatalf("expected snooze to succeed")
	}
	if !store.Resolve(finding.ID) {
		t.Fatalf("expected resolve to succeed")
	}
	resolved := store.Get(finding.ID)
	if resolved.ResolvedAt == nil {
		t.Fatalf("expected resolved at set")
	}
	if resolved.SnoozedUntil != nil {
		t.Fatalf("expected snooze cleared on resolve")
	}
}

func TestUnifiedStore_AIFilteringAndDismiss(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "cpu",
		AlertLevel:   "warning",
		ResourceID:   "vm-1",
		ResourceName: "web",
		Value:        90,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}
	store.AddFromAlert(alert)

	ai := &UnifiedFinding{
		ID:           "ai-1",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryPerformance,
		ResourceID:   "vm-2",
		ResourceName: "db",
		Title:        "ai",
	}
	store.AddFromAI(ai)

	if len(store.GetAIFindings()) != 1 {
		t.Fatalf("expected 1 AI finding")
	}
	if len(store.GetUnenhancedThresholdFindings()) != 1 {
		t.Fatalf("expected 1 unenhanced threshold finding")
	}

	for _, f := range store.GetThresholdFindings() {
		if !store.Dismiss(f.ID, "not_an_issue", "expected") {
			t.Fatalf("expected dismiss to succeed")
		}
		dismissed := store.Get(f.ID)
		if !dismissed.Suppressed {
			t.Fatalf("expected suppression for not_an_issue")
		}
	}
}

func TestUnifiedStore_FormatForContext(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	alert := &SimpleAlertAdapter{
		AlertID:      "alert-1",
		AlertType:    "cpu",
		AlertLevel:   "critical",
		ResourceID:   "vm-1",
		ResourceName: "web",
		Value:        95,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)
	store.EnhanceWithAI(finding.ID, "context", 0.9, "root-1", nil)

	ai := &UnifiedFinding{
		ID:           "ai-1",
		Source:       SourceAIPatrol,
		Severity:     SeverityWarning,
		Category:     CategoryPerformance,
		ResourceID:   "vm-2",
		ResourceName: "db",
		Title:        "ai finding",
		RootCauseID:  "root-1",
	}
	store.AddFromAI(ai)

	out := store.FormatForContext()
	if !strings.Contains(out, "Threshold Alerts") {
		t.Fatalf("expected threshold section")
	}
	if !strings.Contains(out, "AI context") {
		t.Fatalf("expected AI context")
	}
	if !strings.Contains(out, "Root cause linked") {
		t.Fatalf("expected root cause linkage")
	}
}

func TestUnifiedStore_SummaryIncludesEnhanced(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	alert := &SimpleAlertAdapter{
		AlertID:    "alert-1",
		AlertType:  "cpu",
		AlertLevel: "warning",
		ResourceID: "vm-1",
		Value:      90,
		Threshold:  80,
		StartTime:  time.Now(),
		LastSeen:   time.Now(),
	}
	finding, _ := store.AddFromAlert(alert)
	store.EnhanceWithAI(finding.ID, "context", 0.8, "", nil)

	summary := store.GetSummary()
	if summary.EnhancedByAI != 1 {
		t.Fatalf("expected enhanced count 1, got %d", summary.EnhancedByAI)
	}
}

func TestUnifiedStore_ForceSave(t *testing.T) {
	store := NewUnifiedStore(DefaultAlertToFindingConfig())
	persistence := &stubUnifiedPersistence{}
	store.persistence = persistence

	alert := &SimpleAlertAdapter{
		AlertID:    "alert-1",
		AlertType:  "cpu",
		AlertLevel: "warning",
		ResourceID: "vm-1",
		Value:      90,
		Threshold:  80,
		StartTime:  time.Now(),
		LastSeen:   time.Now(),
	}
	store.AddFromAlert(alert)

	if err := store.ForceSave(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if persistence.saveCalls == 0 {
		t.Fatalf("expected save to be called")
	}
}

func TestUnifiedHelpers(t *testing.T) {
	if severityOrder("unknown") != 0 {
		t.Fatalf("expected default severity order")
	}
	if determineResourceType("nodeOffline", nil) != "node" {
		t.Fatalf("expected node resource type")
	}
	if determineResourceType("usage", nil) != "storage" {
		t.Fatalf("expected storage resource type")
	}
	if determineResourceType("backup", nil) != "backup" {
		t.Fatalf("expected backup resource type")
	}
	if determineResourceType("snapshot", nil) != "snapshot" {
		t.Fatalf("expected snapshot resource type")
	}
	if determineResourceType("imageUpdateAvail", nil) != "docker" {
		t.Fatalf("expected docker resource type")
	}
	if determineResourceType("other", map[string]interface{}{"resourceType": "custom"}) != "custom" {
		t.Fatalf("expected custom resource type")
	}

	title := generateTitle("offline", "node-1", 0, 0)
	if !strings.Contains(title, "offline") {
		t.Fatalf("expected offline title")
	}
	if !strings.Contains(generateRecommendation("temperature", 0, 0), "cooling") {
		t.Fatalf("expected temperature recommendation")
	}
	if !strings.Contains(generateRecommendation("unknown", 0, 0), "Investigate") {
		t.Fatalf("expected default recommendation")
	}
	if formatSourceName("custom") != "custom" {
		t.Fatalf("expected fallback source name")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string {
	return "test error"
}
