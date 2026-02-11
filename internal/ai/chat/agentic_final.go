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

// ensureFinalTextResponse checks if the result messages contain any assistant text.
// If not, it makes one last text-only LLM call to force the model to summarize its findings.
// This prevents the loop from exiting silently after making tool calls without answering.
func (a *AgenticLoop) ensureFinalTextResponse(
	ctx context.Context,
	sessionID string,
	resultMessages []Message,
	providerMessages []providers.Message,
	callback StreamCallback,
) []Message {
	// Check if any assistant message has text content
	for i := len(resultMessages) - 1; i >= 0; i-- {
		if resultMessages[i].Role == "assistant" && strings.TrimSpace(resultMessages[i].Content) != "" {
			return resultMessages // Already has text — nothing to do
		}
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

	summaryReq := providers.ChatRequest{
		Messages:   cleanMessages,
		System:     a.getSystemPrompt(),
		ToolChoice: &providers.ToolChoice{Type: providers.ToolChoiceNone},
		// No Tools field — completely omit tools to prevent hallucinated function calls
	}

	var summaryBuilder strings.Builder

	// Keep this bounded so a stuck provider stream doesn't drag the whole request.
	summaryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	summaryErr := a.provider.ChatStream(summaryCtx, summaryReq, func(event providers.StreamEvent) {
		switch event.Type {
		case "content":
			if data, ok := event.Data.(providers.ContentEvent); ok {
				summaryBuilder.WriteString(data.Text)
				jsonData, _ := json.Marshal(ContentData{Text: data.Text})
				callback(StreamEvent{Type: "content", Data: jsonData})
			}
		case "done":
			if data, ok := event.Data.(providers.DoneEvent); ok {
				a.totalInputTokens += data.InputTokens
				a.totalOutputTokens += data.OutputTokens
			}
		}
	})

	if summaryErr == nil && summaryBuilder.Len() > 0 {
		summaryMsg := Message{
			ID:        uuid.New().String(),
			Role:      "assistant",
			Content:   cleanDeepSeekArtifacts(summaryBuilder.String()),
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

func buildAutomaticFallbackSummary(resultMessages []Message) string {
	successCount := 0
	errorCount := 0
	toolCounts := make(map[string]int)
	lastSuccessSnippet := ""

	for _, msg := range resultMessages {
		if msg.ToolResult == nil {
			continue
		}
		if msg.ToolResult.IsError {
			errorCount++
			continue
		}
		successCount++
		if tool := normalizeToolUseID(msg.ToolResult.ToolUseID); tool != "" {
			toolCounts[tool]++
		}
		content := strings.TrimSpace(msg.ToolResult.Content)
		if content != "" {
			lastSuccessSnippet = compactSnippet(content, 360)
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

	summary := fmt.Sprintf(
		"I completed %d successful check(s) using %s. The model didn't provide a final narrative, so this is an automatic summary.",
		successCount, toolList,
	)
	if errorCount > 0 {
		summary += fmt.Sprintf(" There were %d tool error(s) during this run.", errorCount)
	}
	if lastSuccessSnippet != "" {
		summary += "\n\nLatest successful result snippet:\n" + lastSuccessSnippet
	}
	return summary
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

func compactSnippet(content string, maxLen int) string {
	compact := strings.Join(strings.Fields(content), " ")
	if len(compact) <= maxLen {
		return compact
	}
	if maxLen <= 3 {
		return compact[:maxLen]
	}
	return compact[:maxLen-3] + "..."
}
