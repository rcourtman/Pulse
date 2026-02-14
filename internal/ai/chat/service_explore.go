package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	exploreEnabledEnvVar    = "PULSE_EXPLORE_ENABLED"
	exploreMaxTurns         = 3
	exploreTimeout          = 30 * time.Second
	exploreSummaryCharLimit = 2400
)

const (
	exploreOutcomeSuccess               = "success"
	exploreOutcomeFailed                = "failed"
	exploreOutcomeSkippedModel          = "skipped_no_model"
	exploreOutcomeSkippedTools          = "skipped_no_tools"
	exploreOutcomeSkippedConversational = "skipped_conversational"
)

type ExplorePrepassResult struct {
	Summary      string
	Model        string
	Outcome      string
	Duration     time.Duration
	InputTokens  int
	OutputTokens int
	Err          error
}

func isExploreEnabled() bool {
	raw := strings.TrimSpace(os.Getenv(exploreEnabledEnvVar))
	if raw == "" {
		return true
	}

	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		log.Warn().
			Str("env", exploreEnabledEnvVar).
			Str("value", raw).
			Msg("[ChatService] Invalid explore env value; defaulting to enabled")
		return true
	}
	return enabled
}

func (s *Service) shouldRunExplore(autonomousMode bool, prompt string) bool {
	if autonomousMode {
		return false
	}
	if !isExploreEnabled() {
		return false
	}
	if isConversationalMessage(prompt) {
		return false
	}
	return true
}

// isConversationalMessage returns true when a message is a short conversational
// reply (acknowledgment, confirmation, thanks, etc.) that doesn't warrant a
// full explore pre-pass. This avoids wasting an LLM call + tool execution for
// messages like "ah i see", "thanks", "ok", "got it", etc.
func isConversationalMessage(prompt string) bool {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	// Remove trailing punctuation for matching
	normalized = strings.TrimRight(normalized, "!?.,:;")
	normalized = strings.TrimSpace(normalized)

	if normalized == "" {
		return true
	}

	// Exact-match short phrases that are clearly conversational
	exactConversational := map[string]bool{
		"ok":           true,
		"okay":         true,
		"k":            true,
		"yes":          true,
		"no":           true,
		"yep":          true,
		"nope":         true,
		"yeah":         true,
		"nah":          true,
		"sure":         true,
		"thanks":       true,
		"thank you":    true,
		"ty":           true,
		"thx":          true,
		"cool":         true,
		"nice":         true,
		"great":        true,
		"perfect":      true,
		"awesome":      true,
		"got it":       true,
		"understood":   true,
		"makes sense":  true,
		"i see":        true,
		"ah i see":     true,
		"ah ok":        true,
		"oh i see":     true,
		"oh ok":        true,
		"ah":           true,
		"oh":           true,
		"hmm":          true,
		"hm":           true,
		"interesting":  true,
		"right":        true,
		"alright":      true,
		"sounds good":  true,
		"good to know": true,
		"noted":        true,
		"lol":          true,
		"haha":         true,
		"lgtm":         true,
		"nevermind":    true,
		"never mind":   true,
		"nvm":          true,
	}
	if exactConversational[normalized] {
		return true
	}

	// For very short messages (under 60 chars), check prefix patterns
	// These catch things like "ah i see so the socat is sharing..." or "oh ok thanks"
	if len(normalized) <= 60 {
		conversationalPrefixes := []string{
			"ah i see", "oh i see", "ah ok", "oh ok", "ok so",
			"i see so", "ah so", "oh so", "right so",
			"got it", "makes sense", "thanks for",
			"good to know", "that makes sense",
			"i dont need", "i don't need",
			"i dont use", "i don't use",
			"i dont think", "i don't think",
		}
		for _, prefix := range conversationalPrefixes {
			if strings.HasPrefix(normalized, prefix) {
				// But NOT if it contains a question — that signals a new query
				if !strings.Contains(normalized, "?") {
					return true
				}
			}
		}
	}

	return false
}

func isProviderModelString(model string) bool {
	parts := strings.SplitN(strings.TrimSpace(model), ":", 2)
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func appendUniqueModel(list []string, seen map[string]struct{}, model string) []string {
	model = strings.TrimSpace(model)
	if model == "" {
		return list
	}
	if _, ok := seen[model]; ok {
		return list
	}
	seen[model] = struct{}{}
	return append(list, model)
}

func (s *Service) exploreModelCandidates(overrideModel string) []string {
	seen := make(map[string]struct{}, 4)
	candidates := make([]string, 0, 4)
	candidates = appendUniqueModel(candidates, seen, overrideModel)

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg == nil {
		return candidates
	}

	// BYOK rule: only use explicitly configured model fields.
	candidates = appendUniqueModel(candidates, seen, cfg.DiscoveryModel)
	candidates = appendUniqueModel(candidates, seen, cfg.ChatModel)
	candidates = appendUniqueModel(candidates, seen, cfg.Model)
	return candidates
}

// exploreHTTPTimeout is the per-request HTTP client timeout for explore providers.
// This is deliberately short so that unresponsive models (cold-start, overloaded,
// or deprecated) fail fast within the explore context deadline rather than hanging
// for the default 5-minute provider timeout.
const exploreHTTPTimeout = 10 * time.Second

// createProviderForExplore creates a provider with a short HTTP timeout suitable
// for the explore pre-pass. This ensures that if a model is unresponsive, the
// HTTP call fails quickly and we can either retry or abort gracefully.
func (s *Service) createProviderForExplore(modelStr string) (providers.StreamingProvider, error) {
	if s.providerFactory != nil {
		return s.providerFactory(modelStr)
	}
	if s.cfg == nil {
		return nil, fmt.Errorf("no Pulse Assistant config")
	}

	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid model format: %s (expected provider:model)", modelStr)
	}
	providerName := parts[0]
	modelName := parts[1]

	switch providerName {
	case "anthropic":
		if s.cfg.AnthropicAPIKey == "" {
			return nil, fmt.Errorf("Anthropic API key not configured")
		}
		return providers.NewAnthropicClient(s.cfg.AnthropicAPIKey, modelName, exploreHTTPTimeout), nil
	case "openai":
		if s.cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured")
		}
		return providers.NewOpenAIClient(s.cfg.OpenAIAPIKey, modelName, s.cfg.OpenAIBaseURL, exploreHTTPTimeout), nil
	case "openrouter":
		if s.cfg.OpenRouterAPIKey == "" {
			return nil, fmt.Errorf("OpenRouter API key not configured")
		}
		return providers.NewOpenAIClient(s.cfg.OpenRouterAPIKey, modelName, s.cfg.GetBaseURLForProvider(config.AIProviderOpenRouter), exploreHTTPTimeout), nil
	case "deepseek":
		if s.cfg.DeepSeekAPIKey == "" {
			return nil, fmt.Errorf("DeepSeek API key not configured")
		}
		return providers.NewOpenAIClient(s.cfg.DeepSeekAPIKey, modelName, "https://api.deepseek.com", exploreHTTPTimeout), nil
	case "gemini":
		if s.cfg.GeminiAPIKey == "" {
			return nil, fmt.Errorf("Gemini API key not configured")
		}
		return providers.NewGeminiClient(s.cfg.GeminiAPIKey, modelName, "", exploreHTTPTimeout), nil
	case "ollama":
		baseURL := s.cfg.OllamaBaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return providers.NewOllamaClient(modelName, baseURL, exploreHTTPTimeout), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

func (s *Service) resolveExploreProvider(
	overrideModel string,
	selectedModel string,
	defaultProvider providers.StreamingProvider,
) (providers.StreamingProvider, string) {
	candidates := s.exploreModelCandidates(overrideModel)
	for _, candidate := range candidates {
		if !isProviderModelString(candidate) {
			log.Warn().
				Str("model", candidate).
				Msg("[ChatService] Explore model candidate is invalid; expected provider:model")
			continue
		}

		// For explore, always create a dedicated provider with short timeout
		// rather than reusing the default provider (which has a 5-min timeout).
		p, err := s.createProviderForExplore(candidate)
		if err != nil {
			log.Warn().
				Str("model", candidate).
				Err(err).
				Msg("[ChatService] Explore model unavailable; trying next explicit model")
			continue
		}
		return p, candidate
	}
	return nil, ""
}

func (s *Service) filterToolsForExplorePrompt(ctx context.Context, prompt string) []providers.Tool {
	candidates := s.filterToolsForPrompt(ctx, prompt, true)
	filtered := make([]providers.Tool, 0, len(candidates))
	for _, tool := range candidates {
		if tool.Name == pulseQuestionToolName {
			continue
		}
		if isWriteTool(tool.Name) {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

func (s *Service) buildExploreSystemPrompt() string {
	return `You are Pulse AI Explore, a fast read-only scout that prepares context for the main assistant.

Rules:
- Use tools only for discovery/read context gathering.
- Never perform write/mutating actions.
- Never ask the user questions.
- Focus only on facts relevant to the user's latest request.

Return concise structured output with these headings:
1. Scope
2. Findings
3. Unknowns
4. Suggested next tool step`
}

func latestAssistantContent(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role == "assistant" {
			trimmed := strings.TrimSpace(msg.Content)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func truncateExploreSummary(summary string) string {
	if len(summary) <= exploreSummaryCharLimit {
		return summary
	}
	return summary[:exploreSummaryCharLimit] + "\n\n[Truncated explore summary]"
}

func injectExploreSummaryIntoLatestUserMessage(messages []Message, summary, model string) {
	if strings.TrimSpace(summary) == "" || len(messages) == 0 {
		return
	}
	lastIdx := len(messages) - 1
	if messages[lastIdx].Role != "user" {
		return
	}

	header := "Explore scout context (read-only pre-pass)"
	if model != "" {
		header += " | model: " + model
	}
	block := header +
		"\n<explore_context>\n" +
		"Use this as preliminary context. Re-verify facts before write actions.\n\n" +
		summary +
		"\n</explore_context>"
	messages[lastIdx].Content = block + "\n\n---\n" + messages[lastIdx].Content
}

func emitExploreStatus(callback StreamCallback, phase, message, model, outcome string) {
	if callback == nil || strings.TrimSpace(message) == "" {
		return
	}
	jsonData, _ := json.Marshal(ExploreStatusData{
		Phase:   phase,
		Message: message,
		Model:   model,
		Outcome: outcome,
	})
	callback(StreamEvent{Type: "explore_status", Data: jsonData})
}

func (s *Service) runExplorePrepass(
	ctx context.Context,
	sessionID string,
	prompt string,
	overrideModel string,
	selectedModel string,
	messages []Message,
	executor *tools.PulseToolExecutor,
	defaultProvider providers.StreamingProvider,
	callback StreamCallback,
) ExplorePrepassResult {
	start := time.Now()
	result := ExplorePrepassResult{
		Outcome: exploreOutcomeFailed,
	}
	defer func() {
		result.Duration = time.Since(start)
		if metrics := GetAIMetrics(); metrics != nil {
			metrics.RecordExploreRun(result.Outcome, result.Model, result.Duration, result.InputTokens, result.OutputTokens)
		}
	}()

	exploreTools := s.filterToolsForExplorePrompt(ctx, prompt)
	if len(exploreTools) == 0 {
		result.Outcome = exploreOutcomeSkippedTools
		emitExploreStatus(callback, "skipped", "Explore pre-pass skipped: no read-only tools available.", "", result.Outcome)
		return result
	}

	provider, exploreModel := s.resolveExploreProvider(overrideModel, selectedModel, defaultProvider)
	if provider == nil || exploreModel == "" {
		result.Outcome = exploreOutcomeSkippedModel
		emitExploreStatus(callback, "skipped", "Explore pre-pass skipped: no explicit model configured.", "", result.Outcome)
		return result
	}
	result.Model = exploreModel

	emitExploreStatus(callback, "started", "Explore pre-pass running (read-only context).", exploreModel, "")
	exploreLoop := NewAgenticLoop(provider, executor, s.buildExploreSystemPrompt())
	exploreLoop.SetAutonomousMode(false)
	// Isolate pre-pass state from the main loop to avoid cross-contamination
	// of FSM enforcement/knowledge behavior.
	exploreLoop.SetSessionFSM(NewSessionFSM())
	exploreLoop.SetKnowledgeAccumulator(NewKnowledgeAccumulator())
	exploreLoop.SetMaxTurns(exploreMaxTurns)
	if s.budgetChecker != nil {
		exploreLoop.SetBudgetChecker(s.budgetChecker)
	}

	parts := strings.SplitN(exploreModel, ":", 2)
	if len(parts) == 2 {
		exploreLoop.SetProviderInfo(parts[0], parts[1])
	}

	exploreCtx, cancel := context.WithTimeout(ctx, exploreTimeout)
	defer cancel()

	resultMessages, err := exploreLoop.ExecuteWithTools(exploreCtx, sessionID, messages, exploreTools, func(StreamEvent) {})
	result.InputTokens = exploreLoop.GetTotalInputTokens()
	result.OutputTokens = exploreLoop.GetTotalOutputTokens()
	if err != nil {
		log.Warn().
			Str("session_id", sessionID).
			Str("model", exploreModel).
			Err(err).
			Msg("[ChatService] Explore pre-pass failed; continuing with main loop")
		result.Outcome = exploreOutcomeFailed
		result.Err = err
		emitExploreStatus(callback, "failed", "Explore pre-pass failed; continuing with main analysis.", exploreModel, result.Outcome)
		return result
	}

	result.Outcome = exploreOutcomeSuccess
	result.Summary = truncateExploreSummary(latestAssistantContent(resultMessages))
	emitExploreStatus(callback, "completed", "Explore pre-pass completed.", exploreModel, result.Outcome)
	return result
}
