package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type scriptedReadinessProvider struct {
	connectionErr       error
	continuationErr     error
	wrongProtocol       bool
	wrongContext        bool
	skipContinuation    bool
	waitForCancellation bool
	contextWindow       int
	calls               int
}

func (p *scriptedReadinessProvider) Chat(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
	return nil, errors.New("readiness evaluator must use the streaming provider path")
}

func (p *scriptedReadinessProvider) TestConnection(context.Context) error { return p.connectionErr }
func (p *scriptedReadinessProvider) Name() string                         { return "scripted" }
func (p *scriptedReadinessProvider) ListModels(context.Context) ([]providers.ModelInfo, error) {
	return []providers.ModelInfo{{ID: "test-model"}}, nil
}
func (p *scriptedReadinessProvider) SupportsThinking(string) bool { return false }
func (p *scriptedReadinessProvider) InspectModel(context.Context, string) (*providers.ModelDiagnostics, error) {
	return &providers.ModelDiagnostics{
		Fingerprint:       "sha256:test-model",
		Family:            "test",
		ParameterSize:     "8B",
		QuantizationLevel: "Q4_K_M",
		ContextWindow:     p.contextWindow,
		Capabilities:      []string{"completion", "tools"},
	}, nil
}

var readinessArgumentPattern = regexp.MustCompile(`(?m)(resource_id|category|severity|evidence_token|nonce)=([^,\s.]+)`)
var readinessNoncePattern = regexp.MustCompile(`Evaluation nonce: ([a-f0-9]+)`)

func readinessArgumentsFromPrompt(prompt string) map[string]interface{} {
	input := make(map[string]interface{})
	for _, match := range readinessArgumentPattern.FindAllStringSubmatch(prompt, -1) {
		input[match[1]] = match[2]
	}
	if _, ok := input["nonce"]; !ok {
		if match := readinessNoncePattern.FindStringSubmatch(prompt); len(match) == 2 {
			input["nonce"] = match[1]
		}
	}
	return input
}

func (p *scriptedReadinessProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	p.calls++
	if p.waitForCancellation {
		<-ctx.Done()
		return ctx.Err()
	}

	last := req.Messages[len(req.Messages)-1]
	if last.ToolResult != nil {
		if p.continuationErr != nil {
			return p.continuationErr
		}
		if p.skipContinuation {
			callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "end_turn", InputTokens: 100, OutputTokens: 10}})
			return nil
		}
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(last.ToolResult.Content), &result); err != nil {
			return err
		}
		call := providers.ToolCall{
			ID:   "receipt-call",
			Name: patrolReadinessReceiptTool,
			Input: map[string]interface{}{
				"accepted":   true,
				"finding_id": result["finding_id"],
				"nonce":      result["nonce"],
			},
		}
		callback(providers.StreamEvent{Type: "tool_start", Data: providers.ToolStartEvent{ID: call.ID, Name: call.Name, Input: call.Input}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "tool_use", ToolCalls: []providers.ToolCall{call}, InputTokens: 100, OutputTokens: 10}})
		return nil
	}

	input := readinessArgumentsFromPrompt(req.Messages[0].Content)
	if p.wrongContext && strings.Contains(req.Messages[0].Content, "ACTIONABLE") {
		input["resource_id"] = "healthy-decoy-001"
	}
	call := providers.ToolCall{ID: "observation-call", Name: patrolReadinessObservationTool, Input: input}
	if p.wrongProtocol {
		call.Name = patrolReadinessInventoryTool
	}
	callback(providers.StreamEvent{Type: "tool_start", Data: providers.ToolStartEvent{ID: call.ID, Name: call.Name, Input: call.Input}})
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "tool_use", ToolCalls: []providers.ToolCall{call}, InputTokens: 100, OutputTokens: 10}})
	return nil
}

func readinessTestConfig() *config.AIConfig {
	return &config.AIConfig{
		Enabled:                       true,
		Model:                         "ollama:test-model",
		PatrolModel:                   "ollama:test-model",
		OllamaBaseURL:                 "http://localhost:11434",
		RequestTimeoutSeconds:         300,
		PatrolInvestigationBudget:     15,
		PatrolInvestigationTimeoutSec: 600,
	}
}

func TestRunPatrolModelReadinessWithProvider_VerifiesMonitorAndApproval(t *testing.T) {
	provider := &scriptedReadinessProvider{contextWindow: 32768}
	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)

	if !result.Success || result.Status != PatrolModelReadinessPass || !result.TransportHealthy || !result.PatrolCapable {
		t.Fatalf("expected successful readiness evaluation, got %+v", result)
	}
	if result.MaxVerifiedMode != config.PatrolAutonomyApproval {
		t.Fatalf("max verified mode = %q, want approval", result.MaxVerifiedMode)
	}
	if result.Dimensions.ToolProtocol.Passed != 3 || result.Dimensions.ToolProtocol.Attempts != 3 {
		t.Fatalf("unexpected tool evidence: %+v", result.Dimensions.ToolProtocol)
	}
	if !result.Dimensions.ToolProtocol.ContinuationObserved {
		t.Fatal("expected multi-turn continuation to pass")
	}
	if result.Dimensions.ContextQuality.Passed != 2 {
		t.Fatalf("unexpected context evidence: %+v", result.Dimensions.ContextQuality)
	}
	if result.Modes.Assisted.Status != PatrolModeNotAssessed || result.Modes.Full.Status != PatrolModeNotAssessed {
		t.Fatalf("higher autonomy must remain unassessed: %+v", result.Modes)
	}
	if provider.calls != 4 {
		t.Fatalf("stream calls = %d, want 4", provider.calls)
	}
	if result.providerCalls != 4 || result.inputTokens != 400 || result.outputTokens != 40 {
		t.Fatalf("unexpected usage accounting: calls=%d input=%d output=%d", result.providerCalls, result.inputTokens, result.outputTokens)
	}
}

func TestPatrolReadinessContextFixturesDoNotPutEvidenceAtTheRecencyEdge(t *testing.T) {
	scenarios := patrolReadinessScenarios()
	for _, scenario := range scenarios {
		if !scenario.context {
			continue
		}
		action := strings.Index(scenario.prompt, "ACTIONABLE ")
		firstDecoy := strings.Index(scenario.prompt, "healthy-decoy-000")
		lastDecoy := strings.Index(scenario.prompt, "healthy-decoy-279")
		if action <= firstDecoy || action >= lastDecoy {
			t.Fatalf("scenario %q placed actionable evidence at a context edge", scenario.name)
		}
	}
}

func TestPatrolReadinessToolsUseClosedObjectSchemas(t *testing.T) {
	for _, tool := range patrolReadinessTools() {
		if tool.InputSchema["type"] != "object" {
			t.Fatalf("%s input schema type = %#v", tool.Name, tool.InputSchema["type"])
		}
		if tool.InputSchema["additionalProperties"] != false {
			t.Fatalf("%s input schema is not closed: %#v", tool.Name, tool.InputSchema)
		}
	}
}

func TestRunPatrolModelReadinessWithProvider_SeparatesProtocolFromContextQuality(t *testing.T) {
	provider := &scriptedReadinessProvider{contextWindow: 32768, wrongContext: true}
	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)

	if result.Dimensions.ToolProtocol.Status != PatrolModelReadinessPass || result.Dimensions.ToolProtocol.Passed != 3 {
		t.Fatalf("typed tool protocol should pass independently, got %+v", result.Dimensions.ToolProtocol)
	}
	if result.Dimensions.ContextQuality.Status != PatrolModelReadinessFail || result.Dimensions.ContextQuality.Passed != 0 {
		t.Fatalf("context quality should fail, got %+v", result.Dimensions.ContextQuality)
	}
	if result.Cause != PatrolFailureCauseContextQualityFailed || result.Success {
		t.Fatalf("expected context-quality failure, got %+v", result)
	}
}

func TestRunPatrolModelReadinessWithProvider_ProtocolFailureIsCapabilityWarning(t *testing.T) {
	provider := &scriptedReadinessProvider{contextWindow: 32768, wrongProtocol: true}
	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)

	if result.Dimensions.ToolProtocol.Passed != 0 || result.Success {
		t.Fatalf("expected hard protocol failure, got %+v", result)
	}
	if !result.TransportHealthy || result.PatrolCapable || result.Status != PatrolModelReadinessWarning {
		t.Fatalf("provider health must remain distinct from Patrol capability, got %+v", result)
	}
	if result.Modes.Monitor.Status != PatrolModeNotSuitable {
		t.Fatalf("Watch only should be unsuitable, got %+v", result.Modes.Monitor)
	}
}

func TestRunPatrolModelReadinessWithProvider_ReproducesHealthyAssistantButMissingPatrolTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"opaque-local-model"}]}`))
		case "/v1/chat/completions":
			var request map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			if request["stream"] == true {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"plain text only\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"opaque-local-model","choices":[{"message":{"role":"assistant","content":"Assistant works"},"finish_reason":"stop"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := &config.AIConfig{
		Enabled:               true,
		Model:                 "openai:opaque-local-model",
		PatrolModel:           "openai:opaque-local-model",
		OpenAIBaseURL:         server.URL,
		RequestTimeoutSeconds: 2,
	}
	provider := providers.NewOpenAICompatibleClient("openai", "", "opaque-local-model", server.URL, 2*time.Second)
	assistantResponse, err := provider.Chat(context.Background(), providers.ChatRequest{
		Messages: []providers.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil || assistantResponse.Content != "Assistant works" {
		t.Fatalf("ordinary Assistant chat failed: response=%+v err=%v", assistantResponse, err)
	}

	result := runPatrolModelReadinessWithProvider(
		context.Background(), cfg, config.AIProviderOpenAI, "opaque-local-model", "openai:opaque-local-model", provider,
	)
	if !result.TransportHealthy || result.PatrolCapable || result.Success {
		t.Fatalf("transport and Patrol capability were not separated: %+v", result)
	}
	if result.Status != PatrolModelReadinessWarning || result.Dimensions.Connectivity.Status != PatrolModelReadinessPass {
		t.Fatalf("healthy provider should remain a warning, not a connection failure: %+v", result)
	}
	if result.Dimensions.ToolProtocol.Passed != 0 || result.Cause != PatrolFailureCauseModelToolSupportUnverified {
		t.Fatalf("missing tool calls should be reported as an unverified Patrol capability: %+v", result)
	}
}

func TestRunPatrolModelReadinessWithProvider_ContinuationCapsAtMonitor(t *testing.T) {
	provider := &scriptedReadinessProvider{contextWindow: 32768, skipContinuation: true}
	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)

	if !result.Success || result.MaxVerifiedMode != config.PatrolAutonomyMonitor {
		t.Fatalf("expected Watch-only verification, got %+v", result)
	}
	if result.Modes.Approval.Status != PatrolModeNotSuitable {
		t.Fatalf("Ask first should require continuation, got %+v", result.Modes.Approval)
	}
}

func TestRunPatrolModelReadinessWithProvider_CancellationStopsStreamingProbe(t *testing.T) {
	provider := &scriptedReadinessProvider{contextWindow: 32768, waitForCancellation: true}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	result := runPatrolModelReadinessWithProvider(
		ctx, readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)
	if time.Since(started) > time.Second {
		t.Fatal("cancelled readiness evaluation did not stop promptly")
	}
	if result.Success || result.Dimensions.ToolProtocol.Status != PatrolModelReadinessNotAssessed {
		t.Fatalf("cancelled evaluation should not pass and must stay unassessed, got %+v", result)
	}
	if result.Cause != PatrolFailureCauseInterrupted {
		t.Fatalf("cancelled evaluation must be classified as interrupted, not %q", result.Cause)
	}
}

func TestPatrolModelReadinessCachePersistsAndInvalidates(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	cfg := readinessTestConfig()
	first := NewService(persistence, nil)
	first.cfg = cfg
	result := emptyPatrolModelReadinessResult()
	result.Provider = config.AIProviderOllama
	result.Model = "test-model"
	result.Success = true
	result.Status = PatrolModelReadinessPass
	result.CacheKey = first.patrolModelReadinessCacheKey(cfg, result.Provider, result.Model)
	first.recordPatrolModelReadiness(result, time.Now())

	reloaded := NewService(persistence, nil)
	reloaded.cfg = cfg
	cached, recordedAt := reloaded.CachedPatrolModelReadiness()
	if cached == nil || recordedAt.IsZero() || !cached.Success {
		t.Fatalf("expected persisted readiness evidence, got result=%+v at=%v", cached, recordedAt)
	}
	cfg.PatrolModel = "ollama:another-model"
	if cached, _ := reloaded.CachedPatrolModelReadiness(); cached != nil {
		t.Fatalf("evidence for a different selected model must not be reused: %+v", cached)
	}
	cfg.PatrolModel = "ollama:test-model"
	cfg.OllamaPassword = "rotated-password"
	if cached, _ := reloaded.CachedPatrolModelReadiness(); cached != nil {
		t.Fatalf("evidence must be invalidated after provider credentials change: %+v", cached)
	}
	cfg.OllamaPassword = ""

	reloaded.InvalidatePatrolModelReadiness()
	if cached, _ := reloaded.CachedPatrolModelReadiness(); cached != nil {
		t.Fatalf("expected invalidated cache, got %+v", cached)
	}
	if _, err := os.Stat(filepath.Join(persistence.DataDir(), patrolModelReadinessCacheFilename)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected persisted readiness file removed, got %v", err)
	}
}

func TestRunPatrolModelReadinessCancellationPreservesCompletedEvidence(t *testing.T) {
	cfg := readinessTestConfig()
	service := NewService(nil, nil)
	service.cfg = cfg
	completed := emptyPatrolModelReadinessResult()
	completed.Provider = config.AIProviderOllama
	completed.Model = "test-model"
	completed.Success = true
	completed.Status = PatrolModelReadinessPass
	completed.Summary = "completed evidence"
	completed.CacheKey = service.patrolModelReadinessCacheKey(cfg, completed.Provider, completed.Model)
	service.recordPatrolModelReadiness(completed, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := service.RunPatrolModelReadiness(ctx, "", "")
	if result.Summary != "The Patrol model readiness evaluation was cancelled." {
		t.Fatalf("unexpected cancellation result: %+v", result)
	}
	cached, _ := service.CachedPatrolModelReadiness()
	if cached == nil || cached.Summary != completed.Summary {
		t.Fatalf("cancellation replaced completed evidence: %+v", cached)
	}
}

func TestPatrolModelReadinessUsageIsRecordedWithoutProbeContent(t *testing.T) {
	service := NewService(nil, nil)
	result := emptyPatrolModelReadinessResult()
	result.Provider = config.AIProviderOllama
	result.Model = "qwen3:8b"
	result.providerCalls = 4
	result.inputTokens = 1200
	result.outputTokens = 80
	service.recordPatrolModelReadinessUsage(result)

	events := service.ListCostEvents(1)
	if len(events) != 1 {
		t.Fatalf("usage event count = %d, want 1", len(events))
	}
	event := events[0]
	if event.UseCase != "patrol_readiness" || event.RequestModel != "ollama:qwen3:8b" || event.InputTokens != 1200 || event.OutputTokens != 80 {
		t.Fatalf("unexpected readiness usage event: %+v", event)
	}
	if event.TargetID != "" || event.FindingID != "" || event.SessionID != "" {
		t.Fatalf("synthetic readiness usage must not carry infrastructure or transcript identifiers: %+v", event)
	}
}

func TestPatrolRuntimeReadinessUsesAdvisorForSelectedAutonomyMode(t *testing.T) {
	cfg := readinessTestConfig()
	cfg.PatrolAutonomyLevel = config.PatrolAutonomyApproval
	service := &Service{cfg: cfg}
	result := emptyPatrolModelReadinessResult()
	result.Provider = config.AIProviderOllama
	result.Model = "test-model"
	result.Cause = PatrolFailureCauseModelToolSupportUnverified
	result.CacheKey = service.patrolModelReadinessCacheKey(cfg, result.Provider, result.Model)
	result.Modes.Monitor = PatrolModeSuitability{Status: PatrolModeVerified, Summary: "Watch only verified."}
	result.Modes.Approval = PatrolModeSuitability{Status: PatrolModeNotSuitable, Summary: "Continuation failed."}
	service.recordPatrolModelReadiness(result, time.Now())

	readiness := service.PatrolRuntimeReadiness()
	if readiness.Ready || readiness.Status != PatrolReadinessNotReady {
		t.Fatalf("selected Ask-first mode should be blocked by failed continuation: %+v", readiness)
	}

	cfg.PatrolAutonomyLevel = config.PatrolAutonomyAssisted
	result.CacheKey = service.patrolModelReadinessCacheKey(cfg, result.Provider, result.Model)
	service.recordPatrolModelReadiness(result, time.Now().Add(time.Millisecond))
	readiness = service.PatrolRuntimeReadiness()
	if !readiness.Ready || readiness.Status != PatrolReadinessWarning {
		t.Fatalf("unassessed higher autonomy should remain advisory in the MVP: %+v", readiness)
	}
}

func TestRunPatrolModelReadinessWithProvider_IncompleteContinuationIsNotLatencyFailure(t *testing.T) {
	provider := &scriptedReadinessProvider{contextWindow: 32768, continuationErr: errors.New("provider rejected continuation request")}
	result := runPatrolModelReadinessWithProvider(context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider)
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("readiness evidence: %s", encoded)

	if result.Success || result.PatrolCapable || result.MaxVerifiedMode != "" {
		t.Fatalf("incomplete provider evaluation must not authorize Patrol: %+v", result)
	}
	if result.Dimensions.Latency.Status != PatrolModelReadinessNotAssessed {
		t.Fatalf("unfinished evaluation is not measured latency failure: %+v", result.Dimensions.Latency)
	}
	if result.Dimensions.ToolProtocol.Passed != 3 || result.Dimensions.ContextQuality.Passed != 2 {
		t.Fatalf("completed evidence must survive: %+v", result.Dimensions)
	}
	if result.Modes.Monitor.Status != PatrolModeNotAssessed || result.Cause != PatrolFailureCauseProviderConnection {
		t.Fatalf("preserve provider failure without asserting model incapability: %+v", result)
	}
}

func TestPatrolReadinessCacheCorrectsLegacyIncompleteLatency(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	cfg := readinessTestConfig()
	service := NewService(persistence, nil)
	service.cfg = cfg
	result := runPatrolModelReadinessWithProvider(context.Background(), cfg, config.AIProviderOllama, "test-model", "ollama:test-model", &scriptedReadinessProvider{contextWindow: 32768, continuationErr: errors.New("provider rejected continuation request")})
	result.Dimensions.Latency = PatrolModelReadinessDimension{Status: PatrolModelReadinessFail, Summary: "Latency could not be measured because the streaming probe did not complete."}
	result.Modes.Monitor.Status = PatrolModeNotSuitable
	result.CacheKey = service.patrolModelReadinessCacheKey(cfg, result.Provider, result.Model)
	at := time.Now()
	service.recordPatrolModelReadiness(result, at)
	reloaded := NewService(persistence, nil)
	reloaded.cfg = cfg
	cached, recordedAt := reloaded.CachedPatrolModelReadiness()
	if cached == nil {
		t.Fatal("expected retained evidence")
	}
	if cached.Success || cached.PatrolCapable || cached.MaxVerifiedMode != "" {
		t.Fatalf("cache correction must not authorize Patrol: %+v", cached)
	}
	if cached.Dimensions.Latency.Status != PatrolModelReadinessNotAssessed || cached.Modes.Monitor.Status != PatrolModeNotAssessed {
		t.Fatalf("legacy incomplete probe still misclassified: %+v", cached)
	}
	for _, mode := range []string{config.PatrolAutonomyMonitor, config.PatrolAutonomyApproval, config.PatrolAutonomyAssisted, config.PatrolAutonomyFull} {
		cfg.PatrolAutonomyLevel = mode
		cached.CacheKey = reloaded.patrolModelReadinessCacheKey(cfg, cached.Provider, cached.Model)
		reloaded.recordPatrolModelReadiness(*cached, at.Add(time.Second))
		if readiness := reloaded.PatrolRuntimeReadiness(); readiness.Ready {
			t.Fatalf("unassessed Watch must block %s, got %+v", mode, readiness)
		}
	}
	if !recordedAt.Equal(at) || cached.Cause != result.Cause || cached.Dimensions.ToolProtocol.Passed != 3 || cached.Dimensions.ContextQuality.Passed != 2 {
		t.Fatalf("recorded evidence changed: %+v", cached)
	}
}
