package chat

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rs/zerolog/log"
)

const (
	exploreEnabledEnvVar    = "PULSE_EXPLORE_ENABLED"
	exploreMaxTurns         = 3
	exploreTimeout          = 20 * time.Second
	exploreSummaryCharLimit = 2400
)

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

func (s *Service) shouldRunExplore(autonomousMode bool) bool {
	if autonomousMode {
		return false
	}
	return isExploreEnabled()
}

func (s *Service) chooseExploreModel(overrideModel, selectedModel string) string {
	if strings.TrimSpace(overrideModel) != "" {
		return strings.TrimSpace(overrideModel)
	}

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()
	if cfg == nil {
		return selectedModel
	}

	m := strings.TrimSpace(cfg.GetDiscoveryModel())
	if m == "" {
		return selectedModel
	}

	// Provider creation requires provider:model format.
	if !strings.Contains(m, ":") {
		return selectedModel
	}
	return m
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

func injectExploreSummaryIntoLatestUserMessage(messages []Message, summary string) {
	if strings.TrimSpace(summary) == "" || len(messages) == 0 {
		return
	}
	lastIdx := len(messages) - 1
	if messages[lastIdx].Role != "user" {
		return
	}
	messages[lastIdx].Content = "Explore findings (read-only pre-pass):\n" + summary + "\n\n---\n" + messages[lastIdx].Content
}

func (s *Service) runExplorePrepass(
	ctx context.Context,
	sessionID string,
	prompt string,
	overrideModel string,
	selectedModel string,
	messages []Message,
	executor *tools.PulseToolExecutor,
	sessionFSM *SessionFSM,
	ka *KnowledgeAccumulator,
	defaultProvider providers.StreamingProvider,
) string {
	// If we don't have a concrete model for the main request, skip pre-pass.
	// This keeps compatibility with tests and edge paths that stub the service
	// without a configured chat model.
	if strings.TrimSpace(selectedModel) == "" {
		return ""
	}

	exploreTools := s.filterToolsForExplorePrompt(ctx, prompt)
	if len(exploreTools) == 0 {
		return ""
	}

	exploreModel := s.chooseExploreModel(overrideModel, selectedModel)
	provider := defaultProvider
	if exploreModel != "" && exploreModel != selectedModel {
		p, err := s.createProviderForModel(exploreModel)
		if err != nil {
			log.Warn().
				Str("explore_model", exploreModel).
				Err(err).
				Msg("[ChatService] Failed to create explore model provider; falling back to chat provider")
		} else {
			provider = p
		}
	}
	if provider == nil {
		return ""
	}

	exploreLoop := NewAgenticLoop(provider, executor, s.buildExploreSystemPrompt())
	exploreLoop.SetAutonomousMode(false)
	exploreLoop.SetSessionFSM(sessionFSM)
	exploreLoop.SetKnowledgeAccumulator(ka)
	exploreLoop.SetMaxTurns(exploreMaxTurns)
	if s.budgetChecker != nil {
		exploreLoop.SetBudgetChecker(s.budgetChecker)
	}

	if exploreModel == "" {
		exploreModel = selectedModel
	}
	parts := strings.SplitN(exploreModel, ":", 2)
	if len(parts) == 2 {
		exploreLoop.SetProviderInfo(parts[0], parts[1])
	}

	exploreCtx, cancel := context.WithTimeout(ctx, exploreTimeout)
	defer cancel()

	resultMessages, err := exploreLoop.ExecuteWithTools(exploreCtx, sessionID, messages, exploreTools, func(StreamEvent) {})
	if err != nil {
		log.Warn().
			Str("session_id", sessionID).
			Err(err).
			Msg("[ChatService] Explore pre-pass failed; continuing with main loop")
		return ""
	}

	return truncateExploreSummary(latestAssistantContent(resultMessages))
}
