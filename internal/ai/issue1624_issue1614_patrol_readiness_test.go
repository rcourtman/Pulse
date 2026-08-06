package ai

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Issues #1624 (Ollama) and #1614 (llama.cpp): the readiness advisor probed
// with the provider's server defaults — no runtime context request, pinned
// temperature 0 dropped, a 256-token generation cap consumed by <think>
// reasoning, and a stricter stream stall bound than the real Patrol loop —
// and discarded every validator error, so capable models failed the check
// with nothing actionable in the result.

// requestCapturingReadinessProvider wraps the scripted provider behaviour
// while recording every ChatRequest the advisor sends.
type requestCapturingReadinessProvider struct {
	scriptedReadinessProvider
	requests []providers.ChatRequest
}

func (p *requestCapturingReadinessProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	p.requests = append(p.requests, req)
	return p.scriptedReadinessProvider.ChatStream(ctx, req, callback)
}

// cutOffReadinessProvider simulates a model that burns the whole generation
// budget on reasoning and is cut off before emitting a tool call.
type cutOffReadinessProvider struct {
	scriptedReadinessProvider
}

func (p *cutOffReadinessProvider) ChatStream(_ context.Context, _ providers.ChatRequest, callback providers.StreamCallback) error {
	callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "length", InputTokens: 100, OutputTokens: 256}})
	return nil
}

// failingReadinessProvider fails every streaming probe at the transport level.
type failingReadinessProvider struct {
	scriptedReadinessProvider
	err error
}

func (p *failingReadinessProvider) ChatStream(context.Context, providers.ChatRequest, providers.StreamCallback) error {
	return p.err
}

func TestIssue1624ReadinessProbesPinRuntimeContextTemperatureAndPatrolIdleTimeout(t *testing.T) {
	provider := &requestCapturingReadinessProvider{scriptedReadinessProvider: scriptedReadinessProvider{contextWindow: 32768}}
	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)
	if !result.Success {
		t.Fatalf("expected successful readiness evaluation, got %+v", result)
	}
	if len(provider.requests) != 4 {
		t.Fatalf("captured requests = %d, want 4", len(provider.requests))
	}
	for i, req := range provider.requests {
		if req.MinContextTokens != patrolReadinessProbeContextTokens {
			t.Fatalf("request %d MinContextTokens = %d, want %d (fixtures truncate at Ollama's 4096 default without it)", i, req.MinContextTokens, patrolReadinessProbeContextTokens)
		}
		if !req.TemperatureSet || req.Temperature != 0 {
			t.Fatalf("request %d must pin temperature 0 explicitly, got set=%v value=%v", i, req.TemperatureSet, req.Temperature)
		}
		if req.MaxTokens != patrolReadinessProbeMaxTokens {
			t.Fatalf("request %d MaxTokens = %d, want %d (256 was consumed by <think> reasoning)", i, req.MaxTokens, patrolReadinessProbeMaxTokens)
		}
		if req.StreamIdleTimeout != chat.PatrolProviderStreamIdleTimeout {
			t.Fatalf("request %d StreamIdleTimeout = %s, want the Patrol loop's %s", i, req.StreamIdleTimeout, chat.PatrolProviderStreamIdleTimeout)
		}
	}
	// Fixtures must actually fit the requested runtime context.
	for _, scenario := range patrolReadinessScenarios() {
		if approxTokens := len(scenario.prompt) / 3; approxTokens+patrolReadinessProbeMaxTokens > patrolReadinessProbeContextTokens {
			t.Fatalf("scenario %q (~%d tokens) does not fit the requested %d-token runtime context", scenario.name, approxTokens, patrolReadinessProbeContextTokens)
		}
	}
}

func TestIssue1624ReadinessClampsRuntimeContextToTrainedWindow(t *testing.T) {
	provider := &requestCapturingReadinessProvider{scriptedReadinessProvider: scriptedReadinessProvider{contextWindow: 12000}}
	runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)
	if len(provider.requests) == 0 {
		t.Fatal("expected captured probe requests")
	}
	for i, req := range provider.requests {
		if req.MinContextTokens != 12000 {
			t.Fatalf("request %d MinContextTokens = %d, want the model's trained window 12000", i, req.MinContextTokens)
		}
	}
}

func TestIssue1624ReadinessSurfacesValidatorErrorsAndDoneReason(t *testing.T) {
	provider := &cutOffReadinessProvider{}
	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)
	if result.Success || result.Dimensions.ToolProtocol.Passed != 0 {
		t.Fatalf("expected failed tool protocol, got %+v", result)
	}
	if len(result.Details) == 0 {
		t.Fatalf("validator errors must be surfaced in Details, got none: %+v", result)
	}
	joined := strings.Join(result.Details, "\n")
	if !strings.Contains(joined, "expected exactly one tool call, got 0") {
		t.Fatalf("Details must carry the validator error, got: %s", joined)
	}
	if !strings.Contains(joined, "done_reason=length") {
		t.Fatalf("Details must surface the generation-cap stop reason, got: %s", joined)
	}
}

func TestIssue1614ReadinessWrongProtocolDetailsNameEachScenario(t *testing.T) {
	provider := &scriptedReadinessProvider{contextWindow: 32768, wrongProtocol: true}
	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)
	if len(result.Details) < 3 {
		t.Fatalf("expected a detail per failing scenario, got %v", result.Details)
	}
	joined := strings.Join(result.Details, "\n")
	for _, scenario := range []string{"typed-tool", "backup-failure", "storage-pressure"} {
		if !strings.Contains(joined, scenario) {
			t.Fatalf("Details missing scenario %q: %s", scenario, joined)
		}
	}
}

func TestIssue1614ReadinessProbeFailureKeepsSpecificDiagnosis(t *testing.T) {
	provider := &failingReadinessProvider{err: errors.New("stream chunk timed out after 12s")}
	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)
	failure := patrolRuntimeFailureFromError(provider.err)
	if result.Summary != failure.Summary {
		t.Fatalf("probe failure summary was overwritten: got %q, want %q", result.Summary, failure.Summary)
	}
	if result.Recommendation != failure.Recommendation {
		t.Fatalf("probe failure recommendation was overwritten: got %q, want %q", result.Recommendation, failure.Recommendation)
	}
	if result.Cause != failure.Cause {
		t.Fatalf("probe failure cause was overwritten: got %q, want %q", result.Cause, failure.Cause)
	}
	if len(result.Details) == 0 || !strings.Contains(strings.Join(result.Details, "\n"), "probe failed") {
		t.Fatalf("probe error must be surfaced in Details, got %v", result.Details)
	}
}

func TestIssue1624ReadinessDetailsSurviveCacheCloneAndPersistence(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	cfg := readinessTestConfig()
	service := NewService(persistence, nil)
	service.cfg = cfg

	result := emptyPatrolModelReadinessResult()
	result.Provider = config.AIProviderOllama
	result.Model = "test-model"
	result.Details = []string{`Scenario "typed-tool" tool protocol: nonce did not match`}
	result.CacheKey = service.patrolModelReadinessCacheKey(cfg, result.Provider, result.Model)
	service.recordPatrolModelReadiness(result, time.Now())

	reloaded := NewService(persistence, nil)
	reloaded.cfg = cfg
	cached, _ := reloaded.CachedPatrolModelReadiness()
	if cached == nil || len(cached.Details) != 1 || cached.Details[0] != result.Details[0] {
		t.Fatalf("Details must survive persistence and clone, got %+v", cached)
	}
	cached.Details[0] = "mutated"
	again, _ := reloaded.CachedPatrolModelReadiness()
	if again.Details[0] != result.Details[0] {
		t.Fatal("cached Details must be cloned, not shared with callers")
	}
}
