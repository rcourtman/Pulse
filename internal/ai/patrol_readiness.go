package ai

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

const (
	PatrolReadinessReady    = "ready"
	PatrolReadinessWarning  = "warning"
	PatrolReadinessNotReady = "not_ready"
)

type PatrolFailureCause string

const (
	PatrolFailureCauseNone                       PatrolFailureCause = "none"
	PatrolFailureCauseSettingsPersistence        PatrolFailureCause = "settings_persistence"
	PatrolFailureCauseServiceUnavailable         PatrolFailureCause = "service_unavailable"
	PatrolFailureCauseAssistantDisabled          PatrolFailureCause = "assistant_disabled"
	PatrolFailureCauseProviderNotConfigured      PatrolFailureCause = "provider_not_configured"
	PatrolFailureCauseModelNotSelected           PatrolFailureCause = "model_not_selected"
	PatrolFailureCauseModelProviderUnconfigured  PatrolFailureCause = "model_provider_unconfigured"
	PatrolFailureCauseModelUnsupportedTools      PatrolFailureCause = "model_unsupported_tools"
	PatrolFailureCauseModelToolSupportUnverified PatrolFailureCause = "model_tool_support_unverified"
	PatrolFailureCauseModelUnavailable           PatrolFailureCause = "model_unavailable"
	PatrolFailureCauseContextWindowTooSmall      PatrolFailureCause = "context_window_too_small"
	PatrolFailureCauseProviderBilling            PatrolFailureCause = "provider_billing"
	PatrolFailureCauseProviderRateLimited        PatrolFailureCause = "provider_rate_limited"
	PatrolFailureCauseProviderAuth               PatrolFailureCause = "provider_auth"
	PatrolFailureCauseProviderConnection         PatrolFailureCause = "provider_connection"
	PatrolFailureCauseCircuitOpen                PatrolFailureCause = "circuit_open"
)

type PatrolConfigReadiness struct {
	Status   string
	Ready    bool
	Cause    PatrolFailureCause
	Summary  string
	Provider string
	Model    string
}

func EvaluatePatrolConfigReadiness(cfg *config.AIConfig) PatrolConfigReadiness {
	if cfg == nil {
		return patrolConfigReadiness("", "", PatrolReadinessNotReady, PatrolFailureCauseSettingsPersistence, "Pulse Assistant settings could not be loaded from persistence.")
	}
	if !cfg.Enabled {
		return patrolConfigReadiness("", "", PatrolReadinessNotReady, PatrolFailureCauseAssistantDisabled, "Pulse Assistant is disabled, so Patrol cannot run model-backed verification.")
	}
	if !cfg.IsConfigured() {
		return patrolConfigReadiness("", "", PatrolReadinessNotReady, PatrolFailureCauseProviderNotConfigured, "No AI provider is configured for Patrol.")
	}

	model := strings.TrimSpace(cfg.GetPatrolModel())
	if model == "" {
		model = strings.TrimSpace(cfg.GetChatModel())
	}
	provider, _ := config.ParseModelString(model)
	if model == "" || provider == "" || provider == config.AIProviderQuickstart {
		return patrolConfigReadiness(provider, model, PatrolReadinessNotReady, PatrolFailureCauseModelNotSelected, "No concrete Patrol model is selected.")
	}
	if !cfg.HasProvider(provider) {
		return patrolConfigReadiness(provider, model, PatrolReadinessNotReady, PatrolFailureCauseModelProviderUnconfigured, fmt.Sprintf("The selected Patrol model uses %s, but that provider is not configured.", provider))
	}

	status, cause, message := PatrolToolReadinessForModel(provider, model)
	if status == PatrolReadinessReady {
		cause = PatrolFailureCauseNone
		message = "Patrol is ready to run tool-backed verification."
	}
	return patrolConfigReadiness(provider, model, status, cause, message)
}

func PatrolToolReadinessForModel(provider, model string) (string, PatrolFailureCause, string) {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(normalizedModel, "deepseek-r1") ||
		strings.Contains(normalizedModel, "/r1") ||
		strings.Contains(normalizedModel, ":r1") ||
		strings.Contains(normalizedModel, "reasoner") ||
		strings.Contains(normalizedModel, "qwq"):
		return PatrolReadinessNotReady, PatrolFailureCauseModelUnsupportedTools, "The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls. Patrol needs tool calling to inspect resources and create governed findings."
	case provider == config.AIProviderOpenRouter:
		return PatrolReadinessWarning, PatrolFailureCauseModelToolSupportUnverified, "OpenRouter routes vary by model and endpoint. Patrol will fail closed if the routed model rejects tools or tool_choice."
	case provider == config.AIProviderOllama:
		return PatrolReadinessWarning, PatrolFailureCauseModelToolSupportUnverified, "Ollama connectivity alone does not prove tool support. Use an Ollama model that returns tool_calls for Patrol verification."
	case provider == config.AIProviderDeepSeek:
		return PatrolReadinessWarning, PatrolFailureCauseModelToolSupportUnverified, "DeepSeek model capability varies by model. Patrol requires a model that supports tool calling."
	default:
		return PatrolReadinessReady, PatrolFailureCauseNone, "The selected provider path supports Patrol's tool-backed analysis contract."
	}
}

func patrolConfigReadiness(provider, model, status string, cause PatrolFailureCause, summary string) PatrolConfigReadiness {
	if cause == "" {
		cause = PatrolFailureCauseNone
	}
	return PatrolConfigReadiness{
		Status:   status,
		Ready:    status != PatrolReadinessNotReady,
		Cause:    cause,
		Summary:  summary,
		Provider: provider,
		Model:    model,
	}
}

func (s *Service) PatrolRuntimeReadiness() PatrolConfigReadiness {
	if s == nil {
		return patrolConfigReadiness("", "", PatrolReadinessNotReady, PatrolFailureCauseServiceUnavailable, "Pulse AI runtime service is not available.")
	}
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	return EvaluatePatrolConfigReadiness(cfg)
}
