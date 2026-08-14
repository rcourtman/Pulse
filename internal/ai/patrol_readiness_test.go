package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestEvaluatePatrolConfigReadiness_AssignsStableCause(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*config.AIConfig)
		wantCause PatrolFailureCause
		wantReady bool
	}{
		{
			name:      "assistant disabled",
			wantCause: PatrolFailureCauseAssistantDisabled,
			wantReady: false,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = false
				cfg.Model = "ollama:llama3.2"
				cfg.OllamaBaseURL = "http://127.0.0.1:11434"
			},
		},
		{
			name:      "provider not configured",
			wantCause: PatrolFailureCauseProviderNotConfigured,
			wantReady: false,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.Model = ""
			},
		},
		{
			name:      "model not selected",
			wantCause: PatrolFailureCauseModelNotSelected,
			wantReady: false,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.OllamaBaseURL = "http://127.0.0.1:11434"
			},
		},
		{
			name:      "model provider unconfigured",
			wantCause: PatrolFailureCauseModelProviderUnconfigured,
			wantReady: false,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.OpenRouterAPIKey = "sk-or"
				cfg.PatrolModel = "ollama:llama3.2"
			},
		},
		{
			name:      "model unsupported tools",
			wantCause: PatrolFailureCauseModelUnsupportedTools,
			wantReady: false,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.OllamaBaseURL = "http://127.0.0.1:11434"
				cfg.PatrolModel = "ollama:deepseek-r1:7b"
			},
		},
		{
			name:      "tool support unverified warning",
			wantCause: PatrolFailureCauseModelToolSupportUnverified,
			wantReady: true,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.OllamaBaseURL = "http://127.0.0.1:11434"
				cfg.PatrolModel = "ollama:llama3.2"
			},
		},
		{
			name:      "ollama suggested patrol model is ready",
			wantCause: PatrolFailureCauseNone,
			wantReady: true,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.OllamaBaseURL = "http://127.0.0.1:11434"
				cfg.PatrolModel = "ollama:" + config.OllamaSuggestedPatrolModel
			},
		},
		{
			name:      "deepseek v4 flash ready",
			wantCause: PatrolFailureCauseNone,
			wantReady: true,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.DeepSeekAPIKey = "sk-test"
				cfg.PatrolModel = "deepseek:deepseek-v4-flash"
			},
		},
		{
			name:      "deepseek v4 pro ready",
			wantCause: PatrolFailureCauseNone,
			wantReady: true,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.DeepSeekAPIKey = "sk-test"
				cfg.PatrolModel = "deepseek:deepseek-v4-pro"
			},
		},
		{
			name:      "deepseek legacy alias warns",
			wantCause: PatrolFailureCauseModelToolSupportUnverified,
			wantReady: true,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.DeepSeekAPIKey = "sk-test"
				cfg.PatrolModel = "deepseek:deepseek-chat"
			},
		},
		{
			name:      "deepseek typo is not ready",
			wantCause: PatrolFailureCauseModelUnavailable,
			wantReady: false,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.DeepSeekAPIKey = "sk-test"
				cfg.PatrolModel = "deepseek:deepseek-v4-flush7pro"
			},
		},
		{
			name:      "ready",
			wantCause: PatrolFailureCauseNone,
			wantReady: true,
			configure: func(cfg *config.AIConfig) {
				cfg.Enabled = true
				cfg.AnthropicAPIKey = "sk-ant"
				cfg.PatrolModel = "anthropic:claude-3-5-sonnet-latest"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefaultAIConfig()
			tt.configure(cfg)

			readiness := EvaluatePatrolConfigReadiness(cfg)
			if readiness.Cause != tt.wantCause {
				t.Fatalf("cause = %q, want %q", readiness.Cause, tt.wantCause)
			}
			if readiness.Ready != tt.wantReady {
				t.Fatalf("ready = %t, want %t", readiness.Ready, tt.wantReady)
			}
		})
	}
}

func TestPatrolRuntimeReadinessUsesMatchingPreflightEvidence(t *testing.T) {
	cfg := config.NewDefaultAIConfig()
	cfg.Enabled = true
	cfg.OpenRouterAPIKey = "test-key"
	cfg.PatrolModel = "openrouter:test/model"

	tests := []struct {
		name       string
		preflight  PatrolPreflightResult
		wantStatus string
		wantCause  PatrolFailureCause
		wantReady  bool
	}{
		{
			name: "completed billing failure blocks Patrol",
			preflight: PatrolPreflightResult{
				Provider: "openrouter", Model: "test/model",
				Cause: PatrolFailureCauseProviderBilling, Summary: "Provider billing or quota issue",
			},
			wantStatus: PatrolReadinessNotReady,
			wantCause:  PatrolFailureCauseProviderBilling,
			wantReady:  false,
		},
		{
			name: "accepted request without a tool remains a warning",
			preflight: PatrolPreflightResult{
				Success: true, Provider: "openrouter", Model: "test/model",
				Cause: PatrolFailureCauseModelToolSupportUnverified, Summary: "The model returned text instead of a tool call.",
			},
			wantStatus: PatrolReadinessWarning,
			wantCause:  PatrolFailureCauseModelToolSupportUnverified,
			wantReady:  true,
		},
		{
			name: "observed tool call clears the generic gateway warning",
			preflight: PatrolPreflightResult{
				Success: true, ToolCallObserved: true, Provider: "openrouter", Model: "test/model",
			},
			wantStatus: PatrolReadinessReady,
			wantCause:  PatrolFailureCauseNone,
			wantReady:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{cfg: cfg}
			service.recordPatrolPreflight(tt.preflight, time.Now(), service.patrolPreflightGeneration())

			readiness := service.PatrolRuntimeReadiness()
			if readiness.Status != tt.wantStatus || readiness.Cause != tt.wantCause || readiness.Ready != tt.wantReady {
				t.Fatalf("readiness = %+v, want status=%q cause=%q ready=%t", readiness, tt.wantStatus, tt.wantCause, tt.wantReady)
			}
		})
	}
}

func TestPatrolPreflightEvidenceCannotCrossRoutesOrInvalidationGeneration(t *testing.T) {
	cfg := config.NewDefaultAIConfig()
	cfg.Enabled = true
	cfg.OpenRouterAPIKey = "test-key"
	cfg.PatrolModel = "openrouter:test/model"
	service := &Service{cfg: cfg}

	staleRoute := PatrolPreflightResult{
		Provider: "ollama", Model: "qwen3:8b",
		Cause: PatrolFailureCauseLatencyUnsuitable, Summary: "Local model timed out.",
	}
	service.recordPatrolPreflight(staleRoute, time.Now(), service.patrolPreflightGeneration())
	if cached, _ := service.CachedPatrolPreflight(); cached != nil {
		t.Fatalf("preflight from another route entered the selected-route cache: %+v", cached)
	}

	matchingFailure := PatrolPreflightResult{
		Provider: "openrouter", Model: "test/model",
		Cause: PatrolFailureCauseProviderBilling, Summary: "Old credentials failed.",
	}
	oldGeneration := service.patrolPreflightGeneration()
	service.InvalidatePatrolPreflight()
	service.recordPatrolPreflight(matchingFailure, time.Now(), oldGeneration)
	if cached, _ := service.CachedPatrolPreflight(); cached != nil {
		t.Fatalf("preflight from an invalidated generation replaced current evidence: %+v", cached)
	}

	matchingSuccess := PatrolPreflightResult{
		Success: true, ToolCallObserved: true, Provider: "openrouter", Model: "test/model",
	}
	service.recordPatrolPreflight(matchingSuccess, time.Now(), service.patrolPreflightGeneration())
	if cached, _ := service.CachedPatrolPreflight(); cached == nil || !cached.ToolCallObserved {
		t.Fatalf("current-generation matching preflight was not cached: %+v", cached)
	}
}

func TestEvaluatePatrolConfigReadiness_NilConfigUsesAssistantPatrolSettingsCopy(t *testing.T) {
	readiness := EvaluatePatrolConfigReadiness(nil)

	if readiness.Cause != PatrolFailureCauseSettingsPersistence {
		t.Fatalf("cause = %q, want %q", readiness.Cause, PatrolFailureCauseSettingsPersistence)
	}
	if readiness.Summary != "Pulse Intelligence settings could not be loaded from persistence." {
		t.Fatalf("summary = %q", readiness.Summary)
	}
}
