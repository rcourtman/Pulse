package ai

import (
	"context"
	"encoding/json"
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
	if strings.Contains(failure.Evidence, "tool_choice") || strings.Contains(failure.Evidence, "No endpoints found") {
		t.Fatalf("evidence leaked raw provider detail: %q", failure.Evidence)
	}
	if !strings.Contains(failure.Evidence, "Provider rejected Patrol tool calls") {
		t.Fatalf("expected evidence to keep safe classified detail, got %q", failure.Evidence)
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

func TestClassifyPatrolRuntimeFailureOmitsRawProviderEvidence(t *testing.T) {
	diagnostic := ClassifyPatrolRuntimeFailure(errors.New(`API error (401): {"error":"raw upstream credential body"}`))

	if diagnostic.Summary != "Provider authentication issue" {
		t.Fatalf("unexpected summary %q", diagnostic.Summary)
	}
	if diagnostic.Cause != PatrolFailureCauseProviderAuth {
		t.Fatalf("unexpected cause %q", diagnostic.Cause)
	}
	if strings.Contains(diagnostic.Description, "raw upstream credential body") {
		t.Fatalf("description leaked raw provider detail: %q", diagnostic.Description)
	}
	if strings.Contains(diagnostic.Recommendation, "raw upstream credential body") {
		t.Fatalf("recommendation leaked raw provider detail: %q", diagnostic.Recommendation)
	}
}

func TestPatrolRuntimeFailureFromErrorRedactsSecretLikeDetail(t *testing.T) {
	failure := patrolRuntimeFailureFromError(errors.New(`request failed: Get "https://generativelanguage.googleapis.com/v1beta/models?key=AIzaSy-secret-token": Authorization: Bearer sk-live-secret {"api_key":"sk-json-secret"} https://user:pass@example.test/v1`))

	for _, secret := range []string{"AIzaSy-secret-token", "sk-live-secret", "sk-json-secret", "user:pass@"} {
		if strings.Contains(failure.Evidence, secret) {
			t.Fatalf("evidence leaked secret-shaped detail %q: %s", secret, failure.Evidence)
		}
	}
	if !strings.Contains(failure.Evidence, "[redacted]") {
		t.Fatalf("expected evidence to retain redacted context, got %q", failure.Evidence)
	}
}

func TestPatrolRuntimeFailureFromErrorSummarizesReasoningContentProtocolDetail(t *testing.T) {
	failure := patrolRuntimeFailureFromError(errors.New("API error (400): The `reasoning_content` in the thinking mode must be passed back to the API."))

	if strings.Contains(failure.Evidence, "reasoning_content") {
		t.Fatalf("evidence leaked raw provider protocol detail: %q", failure.Evidence)
	}
	if !strings.Contains(failure.Evidence, "Provider rejected Patrol reasoning state") {
		t.Fatalf("expected safe reasoning-state summary, got %q", failure.Evidence)
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
	if strings.Contains(runs[0].ErrorDetail, "tool_choice") || strings.Contains(runs[0].ErrorDetail, "No endpoints found") {
		t.Fatalf("run error detail leaked raw provider message: %q", runs[0].ErrorDetail)
	}
	if !strings.Contains(runs[0].ErrorDetail, "Provider rejected Patrol tool calls") {
		t.Fatalf("expected run error detail to preserve safe classified detail, got %q", runs[0].ErrorDetail)
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

func TestPatrolRunRecordJSONNormalizesRuntimeFailureDetail(t *testing.T) {
	run := PatrolRunRecord{
		ID:           "run-1",
		Status:       "error",
		ErrorSummary: "Selected model does not support Patrol tools",
		ErrorDetail:  `agentic patrol failed: provider error: API error (400): deepseek-reasoner does not support this tool_choice at https://openrouter.ai/settings/keys for user_test`,
	}

	body, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	text := string(body)
	for _, raw := range []string{"tool_choice", "openrouter.ai/settings/keys", "user_test", "deepseek-reasoner"} {
		if strings.Contains(text, raw) {
			t.Fatalf("marshaled run leaked raw provider detail %q: %s", raw, text)
		}
	}
	if !strings.Contains(text, "Provider rejected Patrol tool calls") {
		t.Fatalf("expected safe classified detail in marshaled run, got %s", text)
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

func TestRunScopedPatrolResolvesPreviousRuntimeFailureOnSuccess(t *testing.T) {
	svc := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	svc.cfg = &config.AIConfig{Enabled: true, PatrolModel: "deepseek:deepseek-v4-flash", DeepSeekAPIKey: "sk-test"}
	svc.provider = &mockProvider{}
	var patrolStreamCalled bool
	svc.SetChatService(&mockChatService{
		executor: tools.NewPulseToolExecutor(tools.ExecutorConfig{}),
		executePatrolStreamFunc: func(ctx context.Context, req PatrolExecuteRequest, callback ChatStreamCallback) (*PatrolStreamResponse, error) {
			patrolStreamCalled = true
			return &PatrolStreamResponse{Content: "scoped patrol completed without provider errors"}, nil
		},
	})

	ps := NewPatrolService(svc, &mockStateProvider{
		state: models.StateSnapshot{
			Nodes: []models.Node{{ID: "node-1", Name: "pve-1", Status: "online", CPU: 0.95}},
		},
	})
	ps.SetConfig(PatrolConfig{
		Enabled:      true,
		Interval:     time.Hour,
		AnalyzeNodes: true,
	})
	runtimeFinding := newPatrolRuntimeFailureFinding(patrolRuntimeFailureFromError(errors.New("provider connection failed")), time.Now().Add(-time.Hour))
	ps.recordFinding(runtimeFinding)

	ps.runScopedPatrol(context.Background(), PatrolScope{
		ResourceIDs: []string{"node-1"},
		Reason:      TriggerReasonManual,
		NoStream:    true,
	})

	if !patrolStreamCalled {
		t.Fatal("expected scoped patrol success to exercise the provider-backed patrol stream")
	}

	finding := ps.findings.Get(runtimeFinding.ID)
	if finding == nil {
		t.Fatal("expected runtime finding to remain available as resolved history")
	}
	if !finding.IsResolved() {
		t.Fatalf("expected successful scoped patrol to auto-resolve runtime finding, got resolved_at=%v", finding.ResolvedAt)
	}

	runs := ps.runHistoryStore.GetRecent(1)
	if len(runs) != 1 {
		t.Fatalf("expected one scoped patrol run, got %d", len(runs))
	}
	if runs[0].ErrorCount != 0 {
		t.Fatalf("expected successful scoped run without runtime errors, got %d", runs[0].ErrorCount)
	}
	if runs[0].ResolvedFindings != 1 {
		t.Fatalf("expected scoped run to record resolved runtime finding, got %d", runs[0].ResolvedFindings)
	}
}
