package ai

import (
	"testing"

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

func TestEvaluatePatrolConfigReadiness_NilConfigUsesAssistantPatrolSettingsCopy(t *testing.T) {
	readiness := EvaluatePatrolConfigReadiness(nil)

	if readiness.Cause != PatrolFailureCauseSettingsPersistence {
		t.Fatalf("cause = %q, want %q", readiness.Cause, PatrolFailureCauseSettingsPersistence)
	}
	if readiness.Summary != "Assistant & Patrol settings could not be loaded from persistence." {
		t.Fatalf("summary = %q", readiness.Summary)
	}
}
