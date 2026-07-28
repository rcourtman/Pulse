package ai

// Regression coverage for issue #1640: an evaluation interrupted after every
// tool scenario already passed keeps the tool-protocol dimension at pass while
// the overall run reports not assessed. That snapshot must never be read as a
// Patrol verdict — neither as proof the model is ready nor as a failure that
// blocks Patrol from running in Watch mode.

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// issue1640ContinuationCancellingProvider answers every readiness scenario
// correctly and cancels the run only once the multi-turn continuation probe
// starts — the one ordering that leaves ToolProtocol at pass on an interrupted
// run.
type issue1640ContinuationCancellingProvider struct {
	*scriptedReadinessProvider
	cancel context.CancelFunc
}

func (p *issue1640ContinuationCancellingProvider) ChatStream(ctx context.Context, req providers.ChatRequest, callback providers.StreamCallback) error {
	if req.Messages[len(req.Messages)-1].ToolResult != nil {
		p.cancel()
		<-ctx.Done()
		return ctx.Err()
	}
	return p.scriptedReadinessProvider.ChatStream(ctx, req, callback)
}

func TestIssue1640InterruptedRunKeepsToolPassWithoutAVerdict(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := &issue1640ContinuationCancellingProvider{
		scriptedReadinessProvider: &scriptedReadinessProvider{contextWindow: 32768},
		cancel:                    cancel,
	}
	cfg := readinessTestConfig()

	result := runPatrolModelReadinessWithProvider(
		ctx, cfg, config.AIProviderOllama, "test-model", "ollama:test-model", provider,
	)

	// The shape the readiness gate has to reason about: a passing tool
	// dimension attached to a run that never reached a verdict.
	if result.Dimensions.ToolProtocol.Status != PatrolModelReadinessPass {
		t.Fatalf("tool dimension = %+v, want pass (all scenarios completed before the cancel)", result.Dimensions.ToolProtocol)
	}
	if result.Status != PatrolModelReadinessNotAssessed {
		t.Fatalf("interrupted run status = %q, want %q", result.Status, PatrolModelReadinessNotAssessed)
	}
	if result.Success || result.PatrolCapable {
		t.Fatalf("an interrupted run must not verify the model: %+v", result)
	}
	if result.Cause != PatrolFailureCauseInterrupted {
		t.Fatalf("interrupted run cause = %q, want %q", result.Cause, PatrolFailureCauseInterrupted)
	}

	// The runtime gate on POST /api/ai/patrol/run reads the same snapshot: an
	// interrupted check may not claim readiness, and may not block the run.
	service := NewService(config.NewConfigPersistence(t.TempDir()), nil)
	service.cfg = cfg
	result.CacheKey = patrolModelReadinessCacheKey(cfg, result.Provider, result.Model)
	service.recordPatrolModelReadiness(result, time.Now())

	readiness := service.PatrolRuntimeReadiness()
	if readiness.Status == PatrolReadinessReady {
		t.Fatalf("an interrupted evaluation must not report Patrol ready: %+v", readiness)
	}
	if !readiness.Ready {
		t.Fatalf("an interrupted evaluation must not block Patrol from running: %+v", readiness)
	}
}
