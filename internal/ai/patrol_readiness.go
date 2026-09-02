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
	PatrolFailureCauseToolChoiceRejected         PatrolFailureCause = "tool_choice_rejected"
	PatrolFailureCauseNoToolCapableEndpoint      PatrolFailureCause = "no_tool_capable_endpoint"
	PatrolFailureCauseMalformedToolHistory       PatrolFailureCause = "malformed_tool_history"
	PatrolFailureCauseModelUnavailable           PatrolFailureCause = "model_unavailable"
	PatrolFailureCauseContextWindowTooSmall      PatrolFailureCause = "context_window_too_small"
	PatrolFailureCauseContextQualityFailed       PatrolFailureCause = "context_quality_failed"
	PatrolFailureCauseLatencyUnsuitable          PatrolFailureCause = "latency_unsuitable"
	PatrolFailureCauseProviderBilling            PatrolFailureCause = "provider_billing"
	PatrolFailureCauseProviderRateLimited        PatrolFailureCause = "provider_rate_limited"
	PatrolFailureCauseProviderAuth               PatrolFailureCause = "provider_auth"
	PatrolFailureCauseProviderConnection         PatrolFailureCause = "provider_connection"
	// PatrolFailureCauseInterrupted marks a run that was cancelled mid-flight
	// (operator cancel or a dropped client connection). It is deliberately not
	// a provider fault: an interrupted run carries no evidence about the
	// provider or model (#1640).
	PatrolFailureCauseInterrupted PatrolFailureCause = "interrupted"
	// PatrolFailureCauseInternalError marks a failure inside Pulse itself (a
	// recovered panic on the evaluation path) rather than anything the provider
	// or model did. It must never be presented as a model verdict (#1640).
	PatrolFailureCauseInternalError PatrolFailureCause = "internal_error"
	PatrolFailureCauseCircuitOpen   PatrolFailureCause = "circuit_open"
	// PatrolFailureCauseBudgetExhausted marks a run skipped because the
	// operator's 30-day cost budget is used up. It is a spending decision,
	// not a provider or model fault: the provider was never called, so it
	// must not feed the circuit breaker or read as a model verdict, and the
	// Patrol page must say so instead of leaving the skip in the logs (#1789).
	PatrolFailureCauseBudgetExhausted PatrolFailureCause = "budget_exhausted"
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
	if IsDemoMode() {
		return patrolConfigReadiness(DemoPatrolProvider, DemoPatrolModel, PatrolReadinessReady, PatrolFailureCauseNone,
			"Demo mode: Patrol analysis is simulated against the demo dataset, so no provider is required.")
	}
	if cfg == nil {
		return patrolConfigReadiness("", "", PatrolReadinessNotReady, PatrolFailureCauseSettingsPersistence, "Pulse Intelligence settings could not be loaded from persistence.")
	}
	if !cfg.Enabled {
		return patrolConfigReadiness("", "", PatrolReadinessNotReady, PatrolFailureCauseAssistantDisabled, "Pulse Intelligence is turned off, so Patrol cannot run.")
	}
	if !cfg.IsConfigured() {
		return patrolConfigReadiness("", "", PatrolReadinessNotReady, PatrolFailureCauseProviderNotConfigured, "No AI provider is configured yet. Add a provider API key or an Ollama server on the Provider & Models settings page.")
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
		return patrolConfigReadiness(provider, model, PatrolReadinessNotReady, PatrolFailureCauseModelProviderUnconfigured, fmt.Sprintf("The selected Patrol model uses %s, but that provider is not configured.", config.AIProviderDisplayName(provider)))
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
	case provider == config.AIProviderDeepSeek:
		return patrolDeepSeekToolReadiness(normalizedModel)
	case strings.Contains(normalizedModel, "deepseek-r1") ||
		strings.Contains(normalizedModel, "/r1") ||
		strings.Contains(normalizedModel, ":r1") ||
		strings.Contains(normalizedModel, "reasoner") ||
		strings.Contains(normalizedModel, "qwq"):
		return PatrolReadinessNotReady, PatrolFailureCauseModelUnsupportedTools, "The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls. Patrol needs tool calling to inspect resources and create governed findings."
	case providerDefinitionIsGateway(provider):
		return PatrolReadinessWarning, PatrolFailureCauseModelToolSupportUnverified, fmt.Sprintf("%s routes vary by model and endpoint. Patrol will fail closed if the routed model rejects tools or tool_choice.", config.AIProviderDisplayName(provider))
	case provider == config.AIProviderOllama:
		if patrolOllamaModelIsSuggested(normalizedModel) {
			return PatrolReadinessReady, PatrolFailureCauseNone, fmt.Sprintf("%s passes Patrol's tool check on Ollama.", config.OllamaSuggestedPatrolModel)
		}
		return PatrolReadinessWarning, PatrolFailureCauseModelToolSupportUnverified, fmt.Sprintf("Ollama connectivity alone does not prove tool support, and %s has not passed Patrol's tool check. %s is the verified Patrol model: run ollama pull %s and select it as the Patrol model.", patrolOllamaModelName(normalizedModel), config.OllamaSuggestedPatrolModel, config.OllamaSuggestedPatrolModel)
	default:
		return PatrolReadinessReady, PatrolFailureCauseNone, "The selected provider path supports Patrol's tool-backed analysis contract."
	}
}

// patrolOllamaModelName strips the ollama provider prefix so readiness
// copy names the model the way the operator selected it.
func patrolOllamaModelName(normalizedModel string) string {
	return strings.TrimPrefix(normalizedModel, string(config.AIProviderOllama)+":")
}

// patrolOllamaModelIsSuggested reports whether the selected Ollama model
// is the blessed Patrol model, which has passed the tool check and must
// not be reported as unverified.
func patrolOllamaModelIsSuggested(normalizedModel string) bool {
	return patrolOllamaModelName(normalizedModel) == strings.ToLower(config.OllamaSuggestedPatrolModel)
}

func providerDefinitionIsGateway(provider string) bool {
	def, ok := config.LookupAIProviderDefinition(provider)
	return ok && def.Gateway
}

func patrolDeepSeekToolReadiness(normalizedModel string) (string, PatrolFailureCause, string) {
	model := patrolDeepSeekModelName(normalizedModel)
	switch {
	case patrolDeepSeekModelSupportsTools(model):
		return PatrolReadinessReady, PatrolFailureCauseNone, "The selected DeepSeek model supports Patrol's tool-backed analysis contract."
	case patrolDeepSeekLegacyAlias(model):
		return PatrolReadinessWarning, PatrolFailureCauseModelToolSupportUnverified, "The selected DeepSeek legacy alias currently routes to V4 Flash, but DeepSeek will retire legacy aliases on July 24, 2026. Select deepseek-v4-flash or deepseek-v4-pro for Patrol."
	default:
		return PatrolReadinessNotReady, PatrolFailureCauseModelUnavailable, "The selected DeepSeek model is not in the current official DeepSeek API catalog. Patrol supports deepseek-v4-flash or deepseek-v4-pro."
	}
}

func patrolDeepSeekModelName(normalizedModel string) string {
	model := strings.ToLower(strings.TrimSpace(normalizedModel))
	return strings.TrimPrefix(model, string(config.AIProviderDeepSeek)+":")
}

func patrolDeepSeekModelSupportsTools(normalizedModel string) bool {
	model := patrolDeepSeekModelName(normalizedModel)
	return config.IsDeepSeekV4Model(model)
}

func patrolDeepSeekLegacyAlias(normalizedModel string) bool {
	model := patrolDeepSeekModelName(normalizedModel)
	return config.IsDeepSeekLegacyAliasModel(model)
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
		return patrolConfigReadiness("", "", PatrolReadinessNotReady, PatrolFailureCauseServiceUnavailable, "Pulse Assistant runtime service is not available.")
	}
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	base := EvaluatePatrolConfigReadiness(cfg)
	if !base.Ready || cfg == nil {
		return base
	}
	if preflight, _ := s.CachedPatrolPreflight(); preflight != nil {
		if !preflight.Success {
			// Cancellation is not evidence about the route. Every completed
			// failure is: Patrol must not claim it is ready while the exact
			// selected provider/model cannot pass the lightweight tool check.
			if preflight.Cause != PatrolFailureCauseInterrupted {
				cause := preflight.Cause
				if cause == PatrolFailureCauseNone || cause == "" {
					cause = PatrolFailureCauseModelToolSupportUnverified
				}
				return patrolConfigReadiness(base.Provider, base.Model, PatrolReadinessNotReady, cause, patrolPreflightReadinessSummary(preflight))
			}
		} else if !preflight.ToolCallObserved {
			return patrolConfigReadiness(base.Provider, base.Model, PatrolReadinessWarning, PatrolFailureCauseModelToolSupportUnverified, patrolPreflightReadinessSummary(preflight))
		} else if base.Cause == PatrolFailureCauseModelToolSupportUnverified {
			base = patrolConfigReadiness(base.Provider, base.Model, PatrolReadinessReady, PatrolFailureCauseNone, "The selected Patrol model passed the live tool-call check.")
		}
	}
	advisor, _ := s.CachedPatrolModelReadiness()
	if advisor == nil {
		return base
	}

	selectedMode := cfg.GetPatrolAutonomyLevel()
	var suitability PatrolModeSuitability
	switch selectedMode {
	case config.PatrolAutonomyApproval:
		suitability = advisor.Modes.Approval
	case config.PatrolAutonomyAssisted:
		suitability = advisor.Modes.Assisted
	case config.PatrolAutonomyFull:
		suitability = advisor.Modes.Full
	default:
		suitability = advisor.Modes.Monitor
	}

	switch suitability.Status {
	case PatrolModeVerified:
		return patrolConfigReadiness(advisor.Provider, advisor.Model, PatrolReadinessReady, PatrolFailureCauseNone, suitability.Summary)
	case PatrolModeWarning, PatrolModeNotAssessed:
		return patrolConfigReadiness(advisor.Provider, advisor.Model, PatrolReadinessWarning, advisor.Cause, suitability.Summary)
	default:
		cause := advisor.Cause
		if cause == PatrolFailureCauseNone || cause == "" {
			cause = PatrolFailureCauseModelToolSupportUnverified
		}
		return patrolConfigReadiness(advisor.Provider, advisor.Model, PatrolReadinessNotReady, cause, suitability.Summary)
	}
}

func patrolPreflightReadinessSummary(result *PatrolPreflightResult) string {
	if result == nil {
		return "The selected Patrol model has not passed the live tool-call check."
	}
	if summary := strings.TrimSpace(result.Summary); summary != "" {
		return summary
	}
	if title := strings.TrimSpace(result.Title); title != "" {
		return title
	}
	return "The selected Patrol model has not passed the live tool-call check."
}
