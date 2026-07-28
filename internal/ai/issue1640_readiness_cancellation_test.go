package ai

// Regression coverage for issue #1640: a Patrol model readiness run cancelled
// mid-flight (operator cancel or a reverse proxy dropping the connection)
// must be classified as interrupted, not blamed on the provider or model, and
// the per-scenario evidence completed before the cancellation must survive
// into the returned result.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestIssue1640ContextCanceledClassifiedAsInterrupted(t *testing.T) {
	live := context.Background()

	// An error that actually wraps context.Canceled is unambiguous wherever it
	// surfaces, with or without a run context to consult.
	for _, err := range []error{
		context.Canceled,
		fmt.Errorf("streaming request failed: %w", context.Canceled),
	} {
		failure := patrolRuntimeFailureFromErrorCtx(live, err)
		if failure.Cause != PatrolFailureCauseInterrupted {
			t.Fatalf("error %q classified as %q, want %q", err, failure.Cause, PatrolFailureCauseInterrupted)
		}
		if strings.Contains(failure.Summary, "Provider analysis error") {
			t.Fatalf("cancellation must not be reported as a provider fault: %q", failure.Summary)
		}
		if plain := patrolRuntimeFailureFromError(err); plain.Cause != PatrolFailureCauseInterrupted {
			t.Fatalf("context-free classification of %q = %q, want %q", err, plain.Cause, PatrolFailureCauseInterrupted)
		}
	}

	// A cancelled run context is itself the signal: whatever symptom a
	// torn-down request produces, it is not evidence about the provider.
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, err := range []error{
		errors.New(`Post "http://ollama.internal:11434/api/chat": context canceled`),
		errors.New("unexpected EOF"),
	} {
		failure := patrolRuntimeFailureFromErrorCtx(cancelledCtx, err)
		if failure.Cause != PatrolFailureCauseInterrupted {
			t.Fatalf("error %q under a cancelled run classified as %q, want %q", err, failure.Cause, PatrolFailureCauseInterrupted)
		}
	}

	diagnostic := ClassifyPatrolRuntimeFailure(context.Canceled)
	if diagnostic.Cause != PatrolFailureCauseInterrupted {
		t.Fatalf("diagnostic cause = %q, want %q", diagnostic.Cause, PatrolFailureCauseInterrupted)
	}
}

// A provider is free to put "context canceled" in its own error body when it
// aborts an upstream request of its own — Ollama does. With the run context
// still live that is a genuine provider failure, and classifying it as
// interrupted would make the readiness path persist it as "not assessed",
// hiding a real fault behind a neutral verdict (#1640).
func TestIssue1640ProviderErrorTextIsNotCancellationEvidence(t *testing.T) {
	live := context.Background()
	err := errors.New(`Post "http://ollama.internal:11434/api/chat": context canceled`)

	for name, failure := range map[string]patrolRuntimeFailure{
		"with_live_ctx":  patrolRuntimeFailureFromErrorCtx(live, err),
		"without_ctx":    patrolRuntimeFailureFromError(err),
		"deadline_error": patrolRuntimeFailureFromErrorCtx(live, context.DeadlineExceeded),
	} {
		if failure.Cause == PatrolFailureCauseInterrupted {
			t.Fatalf("%s: provider error text alone must not be classified as interrupted: %+v", name, failure)
		}
	}

	// The detail rewrite follows the same rule: it must not announce a
	// cancellation that never happened.
	if detail := summarizePatrolRuntimeFailureDetail(err.Error(), false); strings.Contains(detail, "The run was cancelled") {
		t.Fatalf("detail must not claim cancellation for a live run: %q", detail)
	}
	if detail := summarizePatrolRuntimeFailureDetail(err.Error(), true); !strings.Contains(detail, "The run was cancelled") {
		t.Fatalf("a genuinely cancelled run should get the cancellation detail, got %q", detail)
	}
}

// The end-to-end effect of the rule above: a readiness run whose provider fails
// with cancellation-flavoured text while the run context is healthy reports a
// real failure, not "not assessed".
func TestIssue1640LiveRunProviderFailureStaysAFailure(t *testing.T) {
	provider := &issue1640ConnectionFailureProvider{
		err: errors.New(`Post "http://ollama.internal:11434/api/chat": context canceled`),
	}

	result := runPatrolModelReadinessWithProvider(
		context.Background(), readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)

	if result.Cause == PatrolFailureCauseInterrupted {
		t.Fatalf("live-run provider failure classified as interrupted: %+v", result)
	}
	if result.Status != PatrolModelReadinessFail {
		t.Fatalf("live-run provider failure status = %q, want %q", result.Status, PatrolModelReadinessFail)
	}
}

type issue1640ConnectionFailureProvider struct {
	err error
}

func (p *issue1640ConnectionFailureProvider) Chat(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
	return nil, p.err
}
func (p *issue1640ConnectionFailureProvider) TestConnection(context.Context) error { return p.err }
func (p *issue1640ConnectionFailureProvider) Name() string                         { return "issue1640" }
func (p *issue1640ConnectionFailureProvider) ListModels(context.Context) ([]providers.ModelInfo, error) {
	return []providers.ModelInfo{{ID: "test-model"}}, nil
}
func (p *issue1640ConnectionFailureProvider) SupportsThinking(string) bool { return false }
func (p *issue1640ConnectionFailureProvider) ChatStream(context.Context, providers.ChatRequest, providers.StreamCallback) error {
	return p.err
}

func TestIssue1640DeadlineExceededStaysProviderConnection(t *testing.T) {
	// A deadline is a real timeout on the provider path and must keep its
	// existing provider-connection classification.
	failure := patrolRuntimeFailureFromError(context.DeadlineExceeded)
	if failure.Cause != PatrolFailureCauseProviderConnection {
		t.Fatalf("deadline exceeded classified as %q, want %q", failure.Cause, PatrolFailureCauseProviderConnection)
	}
}

// issue1640CancellingProvider answers the first readiness scenario correctly,
// then cancels the run context before the second scenario completes —
// simulating a proxy cutting the connection partway through a slow local run.
type issue1640CancellingProvider struct {
	cancel context.CancelFunc
	calls  int
}

func (p *issue1640CancellingProvider) Chat(context.Context, providers.ChatRequest) (*providers.ChatResponse, error) {
	return nil, errors.New("readiness evaluator must use the streaming provider path")
}

func (p *issue1640CancellingProvider) TestConnection(context.Context) error { return nil }
func (p *issue1640CancellingProvider) Name() string                         { return "issue1640" }
func (p *issue1640CancellingProvider) ListModels(context.Context) ([]providers.ModelInfo, error) {
	return []providers.ModelInfo{{ID: "test-model"}}, nil
}
func (p *issue1640CancellingProvider) SupportsThinking(string) bool { return false }

func (p *issue1640CancellingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	p.calls++
	if p.calls == 1 {
		input := readinessArgumentsFromPrompt(req.Messages[0].Content)
		call := providers.ToolCall{ID: "observation-call", Name: patrolReadinessObservationTool, Input: input}
		callback(providers.StreamEvent{Type: "tool_start", Data: providers.ToolStartEvent{ID: call.ID, Name: call.Name, Input: call.Input}})
		callback(providers.StreamEvent{Type: "done", Data: providers.DoneEvent{StopReason: "tool_use", ToolCalls: []providers.ToolCall{call}, InputTokens: 100, OutputTokens: 10}})
		return nil
	}
	p.cancel()
	<-ctx.Done()
	return ctx.Err()
}

func TestIssue1640MidRunCancellationKeepsPartialEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := &issue1640CancellingProvider{cancel: cancel}

	result := runPatrolModelReadinessWithProvider(
		ctx, readinessTestConfig(), config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)

	if result.Cause != PatrolFailureCauseInterrupted {
		t.Fatalf("mid-run cancellation classified as %q, want %q", result.Cause, PatrolFailureCauseInterrupted)
	}
	if result.Status != PatrolModelReadinessNotAssessed {
		t.Fatalf("interrupted run status = %q, want %q", result.Status, PatrolModelReadinessNotAssessed)
	}
	if result.Success || result.PatrolCapable {
		t.Fatalf("interrupted run must not verify the model: %+v", result)
	}

	// Evidence from the completed scenario survives.
	tool := result.Dimensions.ToolProtocol
	if tool.Status != PatrolModelReadinessNotAssessed || tool.Passed != 1 {
		t.Fatalf("interrupted tool dimension must keep partial evidence, got %+v", tool)
	}
	if !strings.Contains(tool.Summary, "Interrupted after 1/3") {
		t.Fatalf("tool summary should report partial progress, got %q", tool.Summary)
	}
	if len(result.Details) == 0 || !strings.Contains(strings.Join(result.Details, "\n"), "context canceled") {
		t.Fatalf("per-scenario details must be preserved, got %v", result.Details)
	}

	// The unfinished dimensions and modes report not assessed instead of
	// blaming the model.
	if result.Dimensions.ContextQuality.Status != PatrolModelReadinessNotAssessed {
		t.Fatalf("context dimension = %+v, want not assessed", result.Dimensions.ContextQuality)
	}
	if result.Dimensions.Latency.Status != PatrolModelReadinessNotAssessed {
		t.Fatalf("latency dimension = %+v, want not assessed", result.Dimensions.Latency)
	}
	if result.Modes.Monitor.Status != PatrolModeNotAssessed || result.Modes.Approval.Status != PatrolModeNotAssessed {
		t.Fatalf("interrupted run must leave modes unassessed: %+v", result.Modes)
	}
	if strings.Contains(result.Summary, "Provider analysis error") {
		t.Fatalf("interrupted run summary must not blame the provider: %q", result.Summary)
	}
}

func TestIssue1640PreCancelledRunReportsInterrupted(t *testing.T) {
	cfg := readinessTestConfig()
	service := NewService(nil, nil)
	service.cfg = cfg

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := service.RunPatrolModelReadiness(ctx, "", "")
	if result.Cause != PatrolFailureCauseInterrupted {
		t.Fatalf("pre-cancelled run classified as %q, want %q", result.Cause, PatrolFailureCauseInterrupted)
	}
	if result.Status != PatrolModelReadinessNotAssessed {
		t.Fatalf("pre-cancelled run status = %q, want %q", result.Status, PatrolModelReadinessNotAssessed)
	}
}
