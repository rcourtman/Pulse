package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rs/zerolog/log"
)

// ensureFinalTextResponse checks if the result messages contain assistant text
// after the latest user/tool-result anchor. If not, it makes one last LLM call
// without tools so the model can summarize its findings.
// This prevents the loop from exiting silently after making tool calls without answering.
//
// cost-recording-exempt: any tokens this final summary turn consumes
// flow into a.totalInputTokens / a.totalOutputTokens via the done
// event, and the orchestrator (chat.Service.recordChatTurnCost)
// records the loop totals after ExecuteWithTools returns. Recording
// here would double-count.
func (a *AgenticLoop) ensureFinalTextResponse(
	ctx context.Context,
	sessionID string,
	resultMessages []Message,
	providerMessages []providers.Message,
	callback StreamCallback,
) []Message {
	return a.ensureFinalTextResponseWithSystemPrompt(ctx, sessionID, resultMessages, providerMessages, callback, "")
}

// cost-recording-exempt: any tokens this final summary turn consumes flow into
// a.totalInputTokens / a.totalOutputTokens via the done event, and the
// orchestrator records the loop totals after ExecuteWithTools returns.
func (a *AgenticLoop) ensureFinalTextResponseWithSystemPrompt(
	ctx context.Context,
	sessionID string,
	resultMessages []Message,
	providerMessages []providers.Message,
	callback StreamCallback,
	systemPromptOverride string,
) []Message {
	if hasFinalAssistantText(resultMessages) {
		return resultMessages
	}

	// No text content from the model. Make a final text-only call.
	log.Warn().Str("session_id", sessionID).Msg("[AgenticLoop] No text content produced — making final summary call")

	// Build clean message history for the summary call:
	// 1. Strip any trailing empty assistant messages (the model already failed to produce
	//    text with these, so including them would just get the same empty result).
	// 2. Append a user-role nudge to give the model a clear instruction.
	cleanMessages := make([]providers.Message, len(providerMessages))
	copy(cleanMessages, providerMessages)
	for len(cleanMessages) > 0 {
		last := cleanMessages[len(cleanMessages)-1]
		if last.Role == "assistant" && strings.TrimSpace(last.Content) == "" && len(last.ToolCalls) == 0 {
			cleanMessages = cleanMessages[:len(cleanMessages)-1]
		} else {
			break
		}
	}
	cleanMessages = append(cleanMessages, providers.Message{
		Role:    "user",
		Content: "Based on what you've investigated above, provide a complete response to the user. Explain what you found or did, mention any issues or caveats they should know about, and suggest next steps if relevant.",
	})

	summarySystemPrompt := a.getSystemPrompt()
	if override := strings.TrimSpace(systemPromptOverride); override != "" {
		summarySystemPrompt = override
	}
	summaryReq := providers.ChatRequest{
		Messages:    cleanMessages,
		System:      summarySystemPrompt,
		ExecutionID: a.executionID,
		// No Tools field: this is a final narrative turn, and Pulse avoids
		// provider-specific tool_choice transport fields.
	}
	a.mu.Lock()
	requestSanitizer := a.requestSanitizer
	a.mu.Unlock()
	if requestSanitizer != nil {
		summaryReq = requestSanitizer(summaryReq)
	}

	var summaryBuilder strings.Builder
	var suppressLeakedToolContent bool
	var pendingVisibleContent string

	// Keep this bounded so a stuck provider stream doesn't drag the whole request.
	summaryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	summaryErr := a.provider.ChatStream(summaryCtx, summaryReq, func(event providers.StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(providers.ContentEvent); ok {
				if suppressLeakedToolContent {
					return
				}
				visibleText, leakFound := appendVisibleContentBeforeToolLeak(&summaryBuilder, &pendingVisibleContent, data.Text)
				if visibleText != "" {
					jsonData, _ := json.Marshal(ContentData{Text: visibleText})
					callback(StreamEvent{Type: "content", Data: jsonData})
				}
				if leakFound {
					suppressLeakedToolContent = true
				}
			}
		case "done":
			if data, ok := event.Data.(providers.DoneEvent); ok {
				a.totalInputTokens += data.InputTokens
				a.totalOutputTokens += data.OutputTokens
			}
		}
	})

	if summaryErr == nil && !suppressLeakedToolContent {
		if visibleText := flushPendingVisibleContent(&summaryBuilder, &pendingVisibleContent); visibleText != "" {
			jsonData, _ := json.Marshal(ContentData{Text: visibleText})
			callback(StreamEvent{Type: "content", Data: jsonData})
		}
	}

	if summaryErr == nil && summaryBuilder.Len() > 0 {
		summaryMsg := Message{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   cleanToolCallArtifacts(summaryBuilder.String()),
			Timestamp: time.Now(),
		}
		resultMessages = append(resultMessages, summaryMsg)
		log.Info().Str("session_id", sessionID).Int("summary_len", summaryBuilder.Len()).Msg("[AgenticLoop] Final summary produced")
		return resultMessages
	}

	if summaryErr != nil {
		log.Error().Err(summaryErr).Str("session_id", sessionID).Msg("[AgenticLoop] Final summary call failed")
	} else {
		log.Warn().Str("session_id", sessionID).Msg("[AgenticLoop] Final summary call returned empty content")
	}

	// Deterministic fallback so the user always gets a usable response even if
	// the model fails to emit final text.
	fallback := buildAutomaticFallbackSummary(resultMessages)
	fallbackMsg := Message{
		ID:        uuid.New().String(),
		Role:      "assistant",
		Content:   fallback,
		Timestamp: time.Now(),
	}
	resultMessages = append(resultMessages, fallbackMsg)
	jsonData, _ := json.Marshal(ContentData{Text: fallback})
	callback(StreamEvent{Type: "content", Data: jsonData})
	log.Warn().Str("session_id", sessionID).Int("summary_len", len(fallback)).Msg("[AgenticLoop] Emitted deterministic fallback summary")

	return resultMessages
}

func hasFinalAssistantText(resultMessages []Message) bool {
	anchor := -1
	for i, msg := range resultMessages {
		if msg.ToolResult != nil {
			anchor = i
			continue
		}
		if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
			anchor = i
		}
	}

	for i := len(resultMessages) - 1; i > anchor; i-- {
		if resultMessages[i].Role == "assistant" && strings.TrimSpace(resultMessages[i].Content) != "" {
			return true
		}
	}
	return false
}

func buildAutomaticFallbackSummary(resultMessages []Message) string {
	successCount := 0
	errorCount := 0
	toolCounts := make(map[string]int)

	// Resolve provider tool-call ids to the real tool name so the summary names
	// the checks (query, metrics, ...) instead of leaking raw `call_…` ids.
	idToName := make(map[string]string)
	for _, msg := range resultMessages {
		for _, call := range msg.ToolCalls {
			id := strings.TrimSpace(call.ID)
			name := strings.TrimSpace(call.Name)
			if id != "" && name != "" {
				idToName[id] = name
			}
		}
	}

	for _, msg := range resultMessages {
		if msg.ToolResult == nil {
			continue
		}
		if msg.ToolResult.IsError {
			errorCount++
			continue
		}
		successCount++
		if name := fallbackToolDisplayName(msg.ToolResult.ToolUseID, idToName); name != "" {
			toolCounts[name]++
		}
	}

	if successCount == 0 && errorCount == 0 {
		return "I couldn't generate a final narrative response from the model in this run. Please retry, and I can run the checks again."
	}

	type toolCount struct {
		Name  string
		Count int
	}
	var ranked []toolCount
	for name, count := range toolCounts {
		ranked = append(ranked, toolCount{Name: name, Count: count})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Count == ranked[j].Count {
			return ranked[i].Name < ranked[j].Name
		}
		return ranked[i].Count > ranked[j].Count
	})

	toolList := "infrastructure tools"
	if len(ranked) > 0 {
		names := make([]string, 0, len(ranked))
		maxNames := 4
		if len(ranked) < maxNames {
			maxNames = len(ranked)
		}
		for i := 0; i < maxNames; i++ {
			names = append(names, ranked[i].Name)
		}
		toolList = strings.Join(names, ", ")
	}

	checkWord := "checks"
	if successCount == 1 {
		checkWord = "check"
	}
	summary := fmt.Sprintf(
		"I ran %d %s (%s) but the model didn't return a written summary this time. Ask me again and I'll pull the results together.",
		successCount, checkWord, toolList,
	)
	if errorCount > 0 {
		errWord := "errors"
		if errorCount == 1 {
			errWord = "error"
		}
		summary += fmt.Sprintf(" %d tool %s occurred during the run.", errorCount, errWord)
	}
	return summary
}

// fallbackToolDisplayName resolves an operator-facing tool name for the
// automatic fallback summary. It prefers the real tool name from the assistant's
// tool call (idToName), and only falls back to the tool-use id when that id
// itself reads like a tool name — never an opaque provider call id
// (`call_…`, `toolu_…`, `fc_…`), which must not leak into chat-visible text.
func fallbackToolDisplayName(toolUseID string, idToName map[string]string) string {
	if name := strings.TrimSpace(idToName[strings.TrimSpace(toolUseID)]); name != "" {
		return displayToolName(name)
	}
	normalized := normalizeToolUseID(toolUseID)
	lower := strings.ToLower(normalized)
	if normalized == "" ||
		strings.HasPrefix(lower, "call_") ||
		strings.HasPrefix(lower, "toolu_") ||
		strings.HasPrefix(lower, "fc_") {
		return ""
	}
	return displayToolName(normalized)
}

func displayToolName(name string) string {
	return strings.TrimPrefix(strings.TrimSpace(name), "pulse_")
}

func normalizeToolUseID(toolUseID string) string {
	toolUseID = strings.TrimSpace(toolUseID)
	if toolUseID == "" {
		return ""
	}

	lastUnderscore := strings.LastIndex(toolUseID, "_")
	if lastUnderscore <= 0 || lastUnderscore >= len(toolUseID)-1 {
		return toolUseID
	}

	suffix := toolUseID[lastUnderscore+1:]
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return toolUseID
		}
	}
	return toolUseID[:lastUnderscore]
}
