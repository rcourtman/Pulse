package unified

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type stubCorrelationEngine struct {
	rootCauseID  string
	correlated   []string
	explanation  string
	returnErr    error
	calledWithID string
}

func (s *stubCorrelationEngine) AnalyzeForFinding(findingID string, resourceID string) (string, []string, string, error) {
	s.calledWithID = findingID
	return s.rootCauseID, s.correlated, s.explanation, s.returnErr
}

type stubRemediationEngine struct {
	planID string
	err    error
	called chan string
}

func (s *stubRemediationEngine) GeneratePlanForFinding(finding *UnifiedFinding) (string, error) {
	if s.called != nil {
		s.called <- finding.ID
	}
	return s.planID, s.err
}

type stubLearningStore struct {
	suppress bool
	last     string
}

func (s *stubLearningStore) RecordFindingFeedback(findingID, resourceID, category, action, reason, note string) {
	s.last = fmt.Sprintf("%s:%s:%s", findingID, action, reason)
}

func (s *stubLearningStore) ShouldSuppress(resourceID, category, severity string) bool {
	return s.suppress
}

func TestIntegration_AddAIFinding_Suppressed(t *testing.T) {
	integration := NewIntegration(DefaultIntegrationConfig(t.TempDir()))
	learning := &stubLearningStore{suppress: true}
	integration.SetLearningStore(learning)

	finding := &UnifiedFinding{
		ID:         "ai-1",
		Source:     SourceAIPatrol,
		Severity:   SeverityWarning,
		Category:   CategoryCapacity,
		ResourceID: "res-1",
		Title:      "test",
	}

	result, isNew := integration.AddAIFinding(finding)
	if result != nil || isNew {
		t.Fatalf("expected suppressed finding to be dropped")
	}
}

func TestIntegration_AddAIFinding_Remediation(t *testing.T) {
	integration := NewIntegration(DefaultIntegrationConfig(t.TempDir()))
	remediation := &stubRemediationEngine{
		planID: "plan-1",
		called: make(chan string, 1),
	}
	integration.SetRemediationEngine(remediation)

	finding := &UnifiedFinding{
		ID:         "ai-1",
		Source:     SourceAIPatrol,
		Severity:   SeverityWarning,
		Category:   CategoryCapacity,
		ResourceID: "res-1",
		Title:      "test",
	}
	integration.AddAIFinding(finding)

	select {
	case <-remediation.called:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected remediation to be invoked")
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		stored := integration.store.Get(finding.ID)
		if stored != nil && stored.RemediationID == "plan-1" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected remediation ID to be linked")
}

func TestIntegration_EnhanceFindingWithCorrelation(t *testing.T) {
	integration := NewIntegration(DefaultIntegrationConfig(t.TempDir()))
	engine := &stubCorrelationEngine{
		rootCauseID: "root-1",
		correlated:  []string{"c1", "c2", "c3"},
		explanation: "cause",
	}
	integration.SetCorrelationEngine(engine)

	finding := &UnifiedFinding{
		ID:         "ai-1",
		Source:     SourceAIPatrol,
		Severity:   SeverityWarning,
		Category:   CategoryCapacity,
		ResourceID: "res-1",
		Title:      "test",
	}
	integration.store.AddFromAI(finding)

	if err := integration.enhanceFindingWithCorrelation("ai-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := integration.store.Get("ai-1")
	if updated == nil || !updated.EnhancedByAI {
		t.Fatalf("expected finding to be enhanced")
	}
	if updated.AIConfidence != 1.0 {
		t.Fatalf("expected confidence 1.0, got %f", updated.AIConfidence)
	}
	if updated.RootCauseID != "root-1" {
		t.Fatalf("expected root cause ID")
	}
}

func TestIntegration_EnhanceFindingWithCorrelation_Errors(t *testing.T) {
	integration := NewIntegration(DefaultIntegrationConfig(t.TempDir()))
	engine := &stubCorrelationEngine{returnErr: errTest}
	integration.SetCorrelationEngine(engine)
	if err := integration.enhanceFindingWithCorrelation("missing"); err == nil {
		t.Fatalf("expected error for missing finding")
	}
	finding := &UnifiedFinding{
		ID:         "ai-1",
		Source:     SourceAIPatrol,
		Severity:   SeverityWarning,
		Category:   CategoryCapacity,
		ResourceID: "res-1",
		Title:      "test",
	}
	integration.store.AddFromAI(finding)
	if err := integration.enhanceFindingWithCorrelation("ai-1"); err == nil {
		t.Fatalf("expected correlation error")
	}
}

func TestIntegration_DismissAndSnoozeFeedback(t *testing.T) {
	integration := NewIntegration(DefaultIntegrationConfig(t.TempDir()))
	learning := &stubLearningStore{}
	integration.SetLearningStore(learning)

	finding := &UnifiedFinding{
		ID:         "ai-1",
		Source:     SourceAIPatrol,
		Severity:   SeverityWarning,
		Category:   CategoryCapacity,
		ResourceID: "res-1",
		Title:      "test",
	}
	integration.store.AddFromAI(finding)

	if !integration.DismissFinding("ai-1", "expected", "note") {
		t.Fatalf("expected dismiss to succeed")
	}
	if !strings.Contains(learning.last, "dismiss") {
		t.Fatalf("expected dismiss feedback to be recorded")
	}

	if !integration.SnoozeFinding("ai-1", time.Minute) {
		t.Fatalf("expected snooze to succeed")
	}
	if !strings.Contains(learning.last, "snooze") {
		t.Fatalf("expected snooze feedback to be recorded")
	}
}

func TestIntegration_SummaryAndSnapshots(t *testing.T) {
	integration := NewIntegration(DefaultIntegrationConfig(t.TempDir()))

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
	finding, _ := integration.store.AddFromAlert(alert)
	integration.store.EnhanceWithAI(finding.ID, "context", 0.8, "", nil)

	summary := integration.GetActiveIssuesSummary()
	if !strings.Contains(summary, "active issues") {
		t.Fatalf("expected active issues summary")
	}

	before := integration.TakeSnapshot()
	integration.store.Resolve(finding.ID)
	after := integration.TakeSnapshot()

	diff := CompareSnapshots(before, after)
	if diff == nil || !diff.HasChanges() {
		t.Fatalf("expected snapshot changes")
	}
	if diff.Summary() == "" {
		t.Fatalf("expected summary")
	}
}

func TestIntegration_GetContextForPatrol(t *testing.T) {
	integration := NewIntegration(DefaultIntegrationConfig(t.TempDir()))
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
	integration.store.AddFromAlert(alert)

	context := integration.GetContextForPatrol()
	if context == "" {
		t.Fatalf("expected patrol context")
	}
}

func TestMinInt(t *testing.T) {
	if minInt(3, 1) != 1 {
		t.Fatalf("expected minInt result")
	}
}
