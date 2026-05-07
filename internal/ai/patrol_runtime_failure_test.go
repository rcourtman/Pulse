package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestPatrolRuntimeFailureFromError_ClassifiesToolCallingUnsupported(t *testing.T) {
	err := errors.New(`agentic patrol failed: API error (404): {"error":{"message":"No endpoints found that support the provided 'tool_choice' value."}}`)

	failure := patrolRuntimeFailureFromError(err)

	if failure.Title != "Pulse Patrol: Selected model does not support Patrol tools" {
		t.Fatalf("unexpected title %q", failure.Title)
	}
	if failure.Summary != "Selected model does not support Patrol tools" {
		t.Fatalf("unexpected summary %q", failure.Summary)
	}
	if failure.Cause != PatrolFailureCauseModelUnsupportedTools {
		t.Fatalf("unexpected cause %q", failure.Cause)
	}
	if !strings.Contains(failure.Recommendation, "supports tool calling") {
		t.Fatalf("expected recommendation to mention tool calling, got %q", failure.Recommendation)
	}
	if !strings.Contains(failure.Evidence, "tool_choice") {
		t.Fatalf("expected evidence to keep provider detail, got %q", failure.Evidence)
	}
}

func TestPatrolRuntimeFailureFromError_ClassifiesUnavailableModel(t *testing.T) {
	err := errors.New(`connected to Ollama but model "qwen3.5:2b" is not available; found: qwen3.5:4b`)

	failure := patrolRuntimeFailureFromError(err)

	if failure.Title != "Pulse Patrol: Selected model unavailable" {
		t.Fatalf("unexpected title %q", failure.Title)
	}
	if failure.Summary != "Selected model unavailable" {
		t.Fatalf("unexpected summary %q", failure.Summary)
	}
	if failure.Cause != PatrolFailureCauseModelUnavailable {
		t.Fatalf("unexpected cause %q", failure.Cause)
	}
	if !strings.Contains(failure.Recommendation, "models currently returned by the provider") {
		t.Fatalf("unexpected recommendation %q", failure.Recommendation)
	}
}

func TestPatrolRuntimeFailureFromError_DefaultIsActionable(t *testing.T) {
	failure := patrolRuntimeFailureFromError(errors.New("upstream returned unexpected eof"))

	if strings.Contains(strings.ToLower(failure.Title), "analysis failed") {
		t.Fatalf("default title should not collapse to analysis failed: %q", failure.Title)
	}
	if failure.Summary != "Provider analysis error" {
		t.Fatalf("unexpected summary %q", failure.Summary)
	}
	if failure.Cause != PatrolFailureCauseProviderConnection {
		t.Fatalf("unexpected cause %q", failure.Cause)
	}
}

func TestNewPatrolRuntimeFailureFindingUsesCanonicalRuntimeIdentity(t *testing.T) {
	failure := patrolRuntimeFailureFromError(errors.New("rate limit exceeded"))
	finding := newPatrolRuntimeFailureFinding(failure, time.Unix(123, 0))

	if finding.Key != patrolRuntimeFindingKey {
		t.Fatalf("unexpected key %q", finding.Key)
	}
	if finding.ResourceID != patrolRuntimeResourceID {
		t.Fatalf("unexpected resource ID %q", finding.ResourceID)
	}
	if finding.Title != "Pulse Patrol: Provider rate limited" {
		t.Fatalf("unexpected title %q", finding.Title)
	}
	if finding.FailureCause != string(PatrolFailureCauseProviderRateLimited) {
		t.Fatalf("unexpected failure cause %q", finding.FailureCause)
	}
	if finding.LastSeenAt.Unix() != 123 {
		t.Fatalf("unexpected last seen %v", finding.LastSeenAt)
	}
}

func TestRunPatrolRecordsStructuredRuntimeFailure(t *testing.T) {
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "openrouter/free-model"}
	svc.provider = &mockProvider{}
	svc.SetChatService(&mockChatService{
		executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{}),
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			return nil, errors.New(`API error (404): {"error":{"message":"No endpoints found that support the provided 'tool_choice' value."}}`)
		},
	})

	ps := NewPatrolService(svc, &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{{ID: "node-1", Name: "pve-1", Status: "online", CPU: 95}},
		},
	})
	ps.SetConfig(PatrolConfig{
		Enabled:      true,
		Interval:     time.Hour,
		AnalyzeNodes: true,
	})

	ps.runPatrol(context.Background())

	runs := ps.runHistoryStore.GetRecent(1)
	if len(runs) != 1 {
		t.Fatalf("expected one patrol run, got %d", len(runs))
	}
	if runs[0].ErrorSummary != "Selected model does not support Patrol tools" {
		t.Fatalf("expected structured run error summary, got %q", runs[0].ErrorSummary)
	}
	if !strings.Contains(runs[0].ErrorDetail, "tool_choice") {
		t.Fatalf("expected run error detail to preserve provider message, got %q", runs[0].ErrorDetail)
	}

	finding := ps.findings.Get(generateFindingID(patrolRuntimeResourceID, "reliability", patrolRuntimeFindingKey))
	if finding == nil {
		t.Fatal("expected Patrol runtime finding")
	}
	if finding.Title != "Pulse Patrol: Selected model does not support Patrol tools" {
		t.Fatalf("unexpected runtime finding title %q", finding.Title)
	}
	if finding.FailureCause != string(PatrolFailureCauseModelUnsupportedTools) {
		t.Fatalf("unexpected runtime finding cause %q", finding.FailureCause)
	}
}

func TestRunScopedPatrolRecordsStructuredRuntimeFailure(t *testing.T) {
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "ollama:qwen3.5:2b"}
	svc.provider = &mockProvider{}
	svc.SetChatService(&mockChatService{
		executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{}),
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			return nil, errors.New(`connected to Ollama but model "qwen3.5:2b" is not available; found: qwen3.5:4b`)
		},
	})

	ps := NewPatrolService(svc, &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{{ID: "node-1", Name: "pve-1", Status: "online", CPU: 95}},
		},
	})
	ps.SetConfig(PatrolConfig{
		Enabled:      true,
		Interval:     time.Hour,
		AnalyzeNodes: true,
	})
	ps.findings.Add(&Finding{
		ID:           "existing-node-finding",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-1",
		ResourceName: "pve-1",
		ResourceType: "node",
		Title:        "Existing node finding",
		DetectedAt:   time.Now(),
		LastSeenAt:   time.Now(),
	})

	ps.runScopedPatrol(context.Background(), PatrolScope{
		ResourceIDs: []string{"node-1"},
		Reason:      TriggerReasonManual,
		NoStream:    true,
	})

	runs := ps.runHistoryStore.GetRecent(1)
	if len(runs) != 1 {
		t.Fatalf("expected one scoped patrol run, got %d", len(runs))
	}
	if runs[0].ErrorSummary != "Selected model unavailable" {
		t.Fatalf("expected structured run error summary, got %q", runs[0].ErrorSummary)
	}

	finding := ps.findings.Get(generateFindingID(patrolRuntimeResourceID, "reliability", patrolRuntimeFindingKey))
	if finding == nil {
		t.Fatal("expected Patrol runtime finding")
	}
	if finding.Title != "Pulse Patrol: Selected model unavailable" {
		t.Fatalf("unexpected runtime finding title %q", finding.Title)
	}
}
