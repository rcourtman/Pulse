package ai

// Manual verification harness for the Ollama blessed-model quickstart
// (config.OllamaSuggestedPatrolModel). Skipped unless pointed at a real
// Ollama server, so it never runs in CI. Use it when re-blessing the
// suggested model:
//
//	ollama pull <candidate>
//	PULSE_MANUAL_OLLAMA_PREFLIGHT_URL=http://127.0.0.1:11434 \
//	PULSE_MANUAL_OLLAMA_PREFLIGHT_MODEL=<candidate> \
//	go test ./internal/ai -run TestManualOllamaBlessedModelPreflight -v -count=1
//
// Run it several times: qwen3:4b passed connectivity but emitted a tool
// call in 0 of 4 preflight runs, which is why only qwen3:8b is blessed.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestManualOllamaBlessedModelPreflight(t *testing.T) {
	base := os.Getenv("PULSE_MANUAL_OLLAMA_PREFLIGHT_URL")
	if base == "" {
		t.Skip("manual preflight: set PULSE_MANUAL_OLLAMA_PREFLIGHT_URL")
	}
	model := os.Getenv("PULSE_MANUAL_OLLAMA_PREFLIGHT_MODEL")
	if model == "" {
		model = config.OllamaSuggestedPatrolModel
	}

	svc := &Service{}
	svc.cfg = &config.AIConfig{Enabled: true, OllamaBaseURL: base}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	res := svc.RunPatrolToolPreflight(ctx, config.AIProviderOllama, model)
	t.Logf("provider=%s model=%s success=%v toolCallObserved=%v durationMs=%d cause=%q title=%q summary=%q",
		res.Provider, res.Model, res.Success, res.ToolCallObserved, res.DurationMs, res.Cause, res.Title, res.Summary)
	if !res.Success || !res.ToolCallObserved {
		t.Fatalf("blessed model %q failed Patrol preflight: %+v", model, res)
	}
}
