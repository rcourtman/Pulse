package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelboundary"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rs/zerolog/log"
)

const (
	// Slow chat routes (reasoning models via aggregators) routinely take
	// ~10s for even tiny completions; the call runs after the client already
	// received done, so the budget only delays handler teardown.
	sessionTitleTimeout        = 20 * time.Second
	sessionTitleSourceMaxChars = 2000
)

const sessionTitleSystemPrompt = `You title Pulse Assistant chat sessions.

Given the first user message and the assistant's reply, produce a short descriptive title for the session list.

Rules:
- At most 50 characters.
- Plain text only: no quotes, no markdown, no trailing punctuation.
- Name the concrete subject (host, VM, container, service, alert) when one exists.
- Do not invent details that are not in the exchange.

Respond with the title only.`

// upgradeSessionTitleAfterFirstExchange upgrades a fresh session's
// placeholder title (the truncated first prompt) to a model-generated one.
// Called after the done event has already reached the client, so the bounded
// provider call adds no user-visible turn latency. Best-effort: never fails
// the turn. Uses a background context because the client may have stopped
// reading the stream by now.
func (s *Service) upgradeSessionTitleAfterFirstExchange(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), sessionTitleTimeout)
	defer cancel()
	if err := s.generateSessionTitle(ctx, sessionID); err != nil {
		if ctx.Err() != nil {
			// A provider too slow to title within budget is worth surfacing.
			log.Warn().Err(err).Str("session_id", sessionID).Msg("[ChatService] Session title generation timed out")
			return
		}
		log.Debug().Err(err).Str("session_id", sessionID).Msg("[ChatService] Session title generation skipped")
	}
}

// generateSessionTitle generates and persists a model-produced session title.
// It only acts on a session's first exchange while the title still carries the
// auto-truncated placeholder, so a user rename is never overwritten.
func (s *Service) generateSessionTitle(ctx context.Context, sessionID string) error {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID == "" {
		return fmt.Errorf("session id required")
	}

	s.mu.RLock()
	started := s.started
	sessions := s.sessions
	provider := s.provider
	costStore := s.costStore
	cfgSnapshot := s.cfg
	unifiedResourceProvider := s.unifiedResourceProvider
	s.mu.RUnlock()

	if !started || sessions == nil {
		return fmt.Errorf("service not started")
	}
	if provider == nil {
		return fmt.Errorf("provider not available")
	}

	session, err := sessions.Get(normalizedSessionID)
	if err != nil {
		return err
	}
	messages, err := sessions.GetMessages(normalizedSessionID)
	if err != nil {
		return err
	}

	firstUser, firstAssistant, userCount := firstSessionExchange(messages)
	if userCount != 1 {
		return fmt.Errorf("not the first exchange")
	}
	if firstUser == "" {
		return fmt.Errorf("no user prompt to title")
	}
	// Only replace the auto-truncated placeholder. Anything else means the
	// user renamed the session (or a title was already generated).
	if session.Title != "" && session.Title != generateTitle(firstUser) {
		return fmt.Errorf("title already customized")
	}

	requestModel := ""
	if cfgSnapshot != nil {
		requestModel = strings.TrimSpace(cfgSnapshot.GetChatModel())
	}

	titleRequest := providers.ChatRequest{
		System:      sessionTitleSystemPrompt,
		ExecutionID: "session-title:" + normalizedSessionID,
		MaxTokens:   60,
		Temperature: 0.2,
		Messages: []providers.Message{{
			Role: "user",
			Content: "Title this Pulse Assistant session.\n\nUser message:\n" +
				truncateForSessionTitle(firstUser) +
				"\n\nAssistant reply:\n" +
				truncateForSessionTitle(firstAssistant),
		}},
	}

	// Persisted prompts carry raw resource identifiers; run the same
	// model-boundary sanitizer as a normal turn (local providers unaffected).
	if sanitizer := modelboundary.RequestSanitizerForModel(requestModel, unifiedResourceProvider); sanitizer != nil {
		titleRequest = sanitizer(titleRequest)
	}

	response, err := provider.Chat(ctx, titleRequest)
	if err != nil {
		return fmt.Errorf("generate session title: %w", err)
	}
	if costStore != nil && response != nil && (response.InputTokens != 0 || response.OutputTokens != 0) {
		costStore.Record(cost.UsageEvent{
			Timestamp:     time.Now(),
			Provider:      provider.Name(),
			RequestModel:  requestModel,
			ResponseModel: strings.TrimSpace(response.Model),
			UseCase:       "chat",
			InputTokens:   response.InputTokens,
			OutputTokens:  response.OutputTokens,
			TargetType:    "assistant_session_title",
			TargetID:      normalizedSessionID,
			SessionID:     normalizedSessionID,
		})
	}

	title := normalizeGeneratedSessionTitle(response.Content)
	if title == "" {
		return fmt.Errorf("model returned an empty title")
	}

	// Re-check for a concurrent user rename before persisting.
	current, err := sessions.Get(normalizedSessionID)
	if err != nil {
		return err
	}
	if current.Title != "" && current.Title != generateTitle(firstUser) {
		return fmt.Errorf("title customized while generating")
	}

	if _, err := sessions.Rename(normalizedSessionID, title); err != nil {
		return err
	}
	log.Debug().Str("session_id", normalizedSessionID).Str("title", title).Msg("[ChatService] Session title generated")
	return nil
}

// firstSessionExchange returns the first user prompt, the first assistant
// answer that follows it, and the total count of real user prompts (tool
// results excluded).
func firstSessionExchange(messages []Message) (string, string, int) {
	firstUser := ""
	firstAssistant := ""
	userCount := 0
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			if msg.ToolResult != nil {
				continue
			}
			userCount++
			if firstUser == "" {
				firstUser = strings.TrimSpace(msg.Content)
			}
		case "assistant":
			if firstUser != "" && firstAssistant == "" && strings.TrimSpace(msg.Content) != "" {
				firstAssistant = strings.TrimSpace(msg.Content)
			}
		}
	}
	return firstUser, firstAssistant, userCount
}

func truncateForSessionTitle(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "(empty)"
	}
	runes := []rune(text)
	if len(runes) <= sessionTitleSourceMaxChars {
		return text
	}
	return string(runes[:sessionTitleSourceMaxChars]) + "…"
}

// normalizeGeneratedSessionTitle shapes raw model output into a session title:
// first line only, stripped of wrapping quotes and trailing punctuation, then
// run through the same word-boundary truncation as placeholder titles.
func normalizeGeneratedSessionTitle(raw string) string {
	line := strings.TrimSpace(raw)
	if idx := strings.IndexAny(line, "\r\n"); idx >= 0 {
		line = line[:idx]
	}
	line = strings.Trim(line, "\"'`*# ")
	line = strings.TrimRight(line, ".!,;: ")
	if line == "" {
		return ""
	}
	return generateTitle(line)
}
