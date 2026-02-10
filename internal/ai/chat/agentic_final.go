package chat

import (
	"context"
	"encoding/json"
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

	summaryErr := a.provider.ChatStream(ctx, summaryReq, func(event providers.StreamEvent) {
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
	} else if summaryErr != nil {
		log.Error().Err(summaryErr).Str("session_id", sessionID).Msg("[AgenticLoop] Final summary call failed")
	} else {
		log.Warn().Str("session_id", sessionID).Msg("[AgenticLoop] Final summary call returned empty content")
	}

	return resultMessages
}
