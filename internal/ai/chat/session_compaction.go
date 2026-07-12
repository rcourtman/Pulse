package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/cost"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/modelboundary"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
)

const (
	sessionCompactionKeepRecentUserTurns = 4
	sessionCompactionMinimumMessages     = 4
	sessionCompactionTranscriptMaxChars  = 52000
	sessionCompactionEntryMaxChars       = 2400
	sessionCompactionToolOutputMaxChars  = 1200
	sessionCompactionSummaryMaxChars     = 9000
	sessionCompactionTimeout             = 45 * time.Second
)

const sessionCompactionSystemPrompt = `You compact Pulse Assistant chat sessions for future turns.

Produce a concise handoff summary that preserves:
- The user's goal and current task.
- Named infrastructure resources, hosts, VMs, containers, services, alerts, and findings.
- Facts already established from tools or prior assistant answers.
- Actions taken, approvals requested, safety boundaries, and failed attempts.
- Open questions and caveats the next turn must remember.

Do not invent facts. Do not include hidden reasoning. Do not include raw secrets, tokens, private keys, or passwords. Keep it practical and suitable as context for the next Pulse Assistant turn.`

// SessionCompactionResult describes a durable session rewrite after compacting
// older turns into a model-produced handoff summary.
type SessionCompactionResult struct {
	Success               bool   `json:"success"`
	Status                string `json:"status"`
	Message               string `json:"message,omitempty"`
	SessionID             string `json:"session_id"`
	SummaryMessageID      string `json:"summary_message_id,omitempty"`
	OriginalMessageCount  int    `json:"original_message_count"`
	CompactedMessageCount int    `json:"compacted_message_count"`
	CompactedMessages     int    `json:"compacted_messages"`
	KeptRecentMessages    int    `json:"kept_recent_messages"`
	SummaryChars          int    `json:"summary_chars"`
}

func (r SessionCompactionResult) asMap() map[string]interface{} {
	return map[string]interface{}{
		"success":                 r.Success,
		"status":                  r.Status,
		"message":                 r.Message,
		"session_id":              r.SessionID,
		"summary_message_id":      r.SummaryMessageID,
		"original_message_count":  r.OriginalMessageCount,
		"compacted_message_count": r.CompactedMessageCount,
		"compacted_messages":      r.CompactedMessages,
		"kept_recent_messages":    r.KeptRecentMessages,
		"summary_chars":           r.SummaryChars,
	}
}

// SummarizeSession compacts a durable chat session using the active chat model.
func (s *Service) SummarizeSession(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	normalizedSessionID := strings.TrimSpace(sessionID)
	if normalizedSessionID == "" {
		return nil, fmt.Errorf("session id required")
	}

	s.mu.RLock()
	started := s.started
	sessions := s.sessions
	provider := s.provider
	cfgSnapshot := s.cfg
	costStore := s.costStore
	unifiedResourceProvider := s.unifiedResourceProvider
	s.mu.RUnlock()

	if !started {
		return nil, fmt.Errorf("service not started")
	}
	if sessions == nil {
		return nil, fmt.Errorf("service not started")
	}
	if provider == nil {
		return nil, fmt.Errorf("provider not available")
	}
	if len(s.getActiveLoops(normalizedSessionID)) > 0 {
		return nil, fmt.Errorf("session is currently running; wait for the active response to finish or stop it before compacting")
	}

	messages, err := sessions.GetMessages(normalizedSessionID)
	if err != nil {
		return nil, err
	}
	if len(messages) < sessionCompactionMinimumMessages {
		return SessionCompactionResult{
			Success:               false,
			Status:                "not_needed",
			Message:               "This Assistant session is already short.",
			SessionID:             normalizedSessionID,
			OriginalMessageCount:  len(messages),
			CompactedMessageCount: len(messages),
		}.asMap(), nil
	}

	transcript := buildSessionCompactionTranscript(messages)
	if strings.TrimSpace(transcript) == "" {
		return SessionCompactionResult{
			Success:               false,
			Status:                "empty",
			Message:               "There is no Assistant transcript to compact.",
			SessionID:             normalizedSessionID,
			OriginalMessageCount:  len(messages),
			CompactedMessageCount: len(messages),
		}.asMap(), nil
	}

	requestModel := ""
	if cfgSnapshot != nil {
		requestModel = strings.TrimSpace(cfgSnapshot.GetChatModel())
	}

	compactCtx, cancel := context.WithTimeout(ctx, sessionCompactionTimeout)
	defer cancel()

	compactionRequest := providers.ChatRequest{
		System:      sessionCompactionSystemPrompt,
		ExecutionID: "session-compaction:" + normalizedSessionID,
		MaxTokens:   1600,
		Temperature: 0.2,
		Messages: []providers.Message{{
			Role: "user",
			Content: "Compact this Pulse Assistant transcript into a handoff summary for the next turn.\n\n" +
				transcript,
		}},
	}

	// The transcript is built from PERSISTED messages (original user prompts and
	// tool outputs), which carry raw resource identifiers. Run it through the same
	// model-boundary sanitizer as a normal turn so compaction never leaks
	// local-only identifiers or credentials to a cloud model (local Ollama is
	// unaffected — RequestSanitizerForModel returns nil).
	if sanitizer := modelboundary.RequestSanitizerForModel(requestModel, unifiedResourceProvider); sanitizer != nil {
		compactionRequest = sanitizer(compactionRequest)
	}

	response, err := provider.Chat(compactCtx, compactionRequest)
	if err != nil {
		return nil, fmt.Errorf("compact session: %w", err)
	}
	if costStore != nil && response != nil && (response.InputTokens != 0 || response.OutputTokens != 0) {
		providerName := ""
		if provider != nil {
			providerName = provider.Name()
		}
		costStore.Record(cost.UsageEvent{
			Timestamp:     time.Now(),
			Provider:      providerName,
			RequestModel:  requestModel,
			ResponseModel: strings.TrimSpace(response.Model),
			UseCase:       "chat",
			InputTokens:   response.InputTokens,
			OutputTokens:  response.OutputTokens,
			TargetType:    "assistant_session_compaction",
			TargetID:      normalizedSessionID,
			SessionID:     normalizedSessionID,
		})
	}

	summary := normalizeSessionCompactionSummary(response.Content)
	if summary == "" {
		return nil, fmt.Errorf("compact session: model returned an empty summary")
	}

	responseModel := strings.TrimSpace(response.Model)
	if responseModel == "" {
		responseModel = requestModel
	}

	result, err := sessions.CompactWithSummary(
		normalizedSessionID,
		summary,
		sessionCompactionKeepRecentUserTurns,
		responseModel,
	)
	if err != nil {
		return nil, err
	}

	return result.asMap(), nil
}

// CompactWithSummary rewrites a session to a model-produced summary plus the
// most recent turns. The full transcript is intentionally not kept in the hot
// model path after compaction.
func (s *SessionStore) CompactWithSummary(
	id string,
	summary string,
	keepRecentUserTurns int,
	model string,
) (*SessionCompactionResult, error) {
	if err := validateSessionID(id); err != nil {
		return nil, err
	}
	summary = normalizeSessionCompactionSummary(summary)
	if summary == "" {
		return nil, fmt.Errorf("summary required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.readSession(id)
	if err != nil {
		return nil, err
	}

	originalCount := len(data.Messages)
	recentMessages := selectRecentMessagesForSessionCompaction(data.Messages, keepRecentUserTurns)
	now := time.Now()
	summaryMessage := Message{
		ID:        uuid.New().String(),
		Role:      "assistant",
		Content:   buildCompactedSessionMessage(summary, originalCount, len(recentMessages)),
		Model:     strings.TrimSpace(model),
		Timestamp: now,
	}

	nextMessages := make([]Message, 0, 1+len(recentMessages))
	nextMessages = append(nextMessages, summaryMessage)
	nextMessages = append(nextMessages, recentMessages...)
	data.Messages = nextMessages
	data.TurnRedoStack = nil
	data.UpdatedAt = now

	if err := s.writeSession(*data); err != nil {
		return nil, err
	}

	compactedMessages := originalCount - len(recentMessages)
	if compactedMessages < 0 {
		compactedMessages = 0
	}
	return &SessionCompactionResult{
		Success:               true,
		Status:                "compacted",
		Message:               fmt.Sprintf("Compacted %d older messages into a session summary.", compactedMessages),
		SessionID:             data.ID,
		SummaryMessageID:      summaryMessage.ID,
		OriginalMessageCount:  originalCount,
		CompactedMessageCount: len(data.Messages),
		CompactedMessages:     compactedMessages,
		KeptRecentMessages:    len(recentMessages),
		SummaryChars:          len([]rune(summary)),
	}, nil
}

func selectRecentMessagesForSessionCompaction(messages []Message, keepUserTurns int) []Message {
	if keepUserTurns <= 0 || len(messages) == 0 {
		return nil
	}

	start := len(messages)
	userTurns := 0
	for index := len(messages) - 1; index >= 0; index-- {
		msg := messages[index]
		if msg.ToolResult != nil {
			continue
		}
		if !strings.EqualFold(msg.Role, "user") {
			continue
		}
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}
		start = index
		userTurns++
		if userTurns >= keepUserTurns {
			break
		}
	}
	if start >= len(messages) {
		return nil
	}

	recent := cloneSessionMessages(messages[start:])
	for len(recent) > 0 && recent[0].ToolResult != nil {
		recent = recent[1:]
	}
	return recent
}

func buildCompactedSessionMessage(summary string, originalMessageCount int, keptRecentMessages int) string {
	return strings.TrimSpace(fmt.Sprintf(
		"Session compacted. Older turns were summarized for context; the latest %d messages remain below.\n\n%s\n\nCompaction source: %d original messages.",
		keptRecentMessages,
		summary,
		originalMessageCount,
	))
}

func normalizeSessionCompactionSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	summary, _ = safety.RedactSensitiveText(summary)
	return truncateSessionCompactionText(summary, sessionCompactionSummaryMaxChars)
}

func buildSessionCompactionTranscript(messages []Message) string {
	entries := make([]string, 0, len(messages))
	for index, message := range messages {
		if entry := formatSessionCompactionTranscriptEntry(index, message); entry != "" {
			entries = append(entries, entry)
		}
	}
	return fitSessionCompactionEntries(entries, sessionCompactionTranscriptMaxChars)
}

func formatSessionCompactionTranscriptEntry(index int, message Message) string {
	var parts []string
	role := strings.TrimSpace(message.Role)
	if role == "" {
		role = "message"
	}

	if message.ToolResult != nil {
		status := "ok"
		if message.ToolResult.IsError {
			status = "error"
		}
		content := sanitizeSessionCompactionText(message.ToolResult.Content)
		if content == "" {
			return ""
		}
		return fmt.Sprintf(
			"[%03d] TOOL RESULT %s (%s)\n%s",
			index+1,
			strings.TrimSpace(message.ToolResult.ToolUseID),
			status,
			truncateSessionCompactionText(content, sessionCompactionToolOutputMaxChars),
		)
	}

	content := sanitizeSessionCompactionText(message.Content)
	if content != "" {
		parts = append(parts, truncateSessionCompactionText(content, sessionCompactionEntryMaxChars))
	}

	for _, toolCall := range message.ToolCalls {
		parts = append(parts, formatSessionCompactionToolCall(toolCall))
	}

	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("[%03d] %s\n%s", index+1, strings.ToUpper(role), strings.Join(parts, "\n"))
}

func formatSessionCompactionToolCall(toolCall ToolCall) string {
	input := "{}"
	if len(toolCall.Input) > 0 {
		if data, err := json.Marshal(toolCall.Input); err == nil {
			input = string(data)
			input, _ = safety.RedactSensitiveText(input)
		}
	}
	parts := []string{
		fmt.Sprintf("Tool call: %s %s", strings.TrimSpace(toolCall.Name), input),
	}
	output := sanitizeSessionCompactionText(toolCall.Output)
	if output != "" {
		parts = append(parts, "Tool output: "+truncateSessionCompactionText(output, sessionCompactionToolOutputMaxChars))
	}
	return strings.Join(parts, "\n")
}

func sanitizeSessionCompactionText(value string) string {
	value = cleanToolCallArtifacts(strings.TrimSpace(value))
	value, _ = safety.RedactSensitiveText(value)
	return strings.TrimSpace(value)
}

func fitSessionCompactionEntries(entries []string, maxChars int) string {
	if len(entries) == 0 || maxChars <= 0 {
		return ""
	}

	full := strings.Join(entries, "\n\n")
	if len([]rune(full)) <= maxChars {
		return full
	}

	headBudget := maxChars / 2
	tailBudget := maxChars - headBudget
	var head []string
	var headChars int
	headEnd := 0
	for headEnd < len(entries) {
		entryChars := len([]rune(entries[headEnd])) + 2
		if headChars+entryChars > headBudget {
			break
		}
		head = append(head, entries[headEnd])
		headChars += entryChars
		headEnd++
	}

	var tail []string
	var tailChars int
	tailStart := len(entries)
	for tailStart > headEnd {
		entry := entries[tailStart-1]
		entryChars := len([]rune(entry)) + 2
		if tailChars+entryChars > tailBudget {
			break
		}
		tail = append([]string{entry}, tail...)
		tailChars += entryChars
		tailStart--
	}

	omitted := tailStart - headEnd
	if omitted < 0 {
		omitted = 0
	}
	notice := fmt.Sprintf("[Compaction transcript omitted %d middle entries to fit the request budget.]", omitted)
	combined := append(head, notice)
	combined = append(combined, tail...)
	return strings.Join(combined, "\n\n")
}

func truncateSessionCompactionText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return strings.TrimSpace(string(runes[:maxRunes])) + "\n[truncated]"
}
