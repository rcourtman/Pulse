package chat

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rs/zerolog/log"
)

func pruneMessagesForModel(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	if StatelessContext {
		for i := len(messages) - 1; i >= 0; i-- {
			msg := messages[i]
			if msg.Role == "user" && msg.ToolResult == nil && msg.Content != "" {
				return []Message{msg}
			}
		}
		return []Message{messages[len(messages)-1]}
	}

	if MaxContextMessagesLimit <= 0 || len(messages) <= MaxContextMessagesLimit {
		return messages
	}

	start := len(messages) - MaxContextMessagesLimit
	log.Warn().
		Int("total_messages", len(messages)).
		Int("limit", MaxContextMessagesLimit).
		Int("dropped", start).
		Msg("[AgenticLoop] Pruning oldest messages to fit context limit")
	pruned := messages[start:]

	// Skip leading tool results (orphaned from pruned tool calls)
	for len(pruned) > 0 && pruned[0].ToolResult != nil {
		pruned = pruned[1:]
	}

	// If we start with an assistant message that has tool calls,
	// skip it and its following tool results — we've pruned the
	// user message that preceded it, so the sequence is broken.
	for len(pruned) > 0 && pruned[0].Role == "assistant" && len(pruned[0].ToolCalls) > 0 {
		pruned = pruned[1:]
		// Also skip the tool results that followed
		for len(pruned) > 0 && pruned[0].ToolResult != nil {
			pruned = pruned[1:]
		}
	}

	return pruned
}

func truncateToolResultForModel(text string) string {
	if MaxToolResultCharsLimit <= 0 || len(text) <= MaxToolResultCharsLimit {
		return text
	}

	truncated := text[:MaxToolResultCharsLimit]
	truncatedChars := len(text) - MaxToolResultCharsLimit
	log.Warn().
		Int("original_chars", len(text)).
		Int("truncated_to", MaxToolResultCharsLimit).
		Int("chars_cut", truncatedChars).
		Msg("[AgenticLoop] Truncating oversized tool result")
	return fmt.Sprintf("%s\n\n---\n[TRUNCATED: %d characters cut. The result was too large. If you need specific details that may have been cut, make a more targeted query (e.g., filter by specific resource or type).]", truncated, truncatedChars)
}

// convertToProviderMessages converts our messages to provider format.
func convertToProviderMessages(messages []Message) []providers.Message {
	result := make([]providers.Message, 0, len(messages))

	for _, m := range messages {
		pm := providers.Message{
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
		}

		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				pm.ToolCalls = append(pm.ToolCalls, providers.ToolCall{
					ID:               tc.ID,
					Name:             tc.Name,
					Input:            tc.Input,
					ThoughtSignature: tc.ThoughtSignature,
				})
			}
		}

		if m.ToolResult != nil {
			pm.ToolResult = &providers.ToolResult{
				ToolUseID: m.ToolResult.ToolUseID,
				Content:   truncateToolResultForModel(m.ToolResult.Content),
				IsError:   m.ToolResult.IsError,
			}
		}

		result = append(result, pm)
	}

	return result
}

// compactOldToolResults replaces full tool result content with short summaries
// for tool results from older turns. This prevents context window blowout during
// long agentic loops (e.g., patrol runs with 20+ tool calls).
//
// Only tool results before currentTurnStartIndex are candidates for compaction.
// Results from the most recent keepTurns turns are kept in full.
// Results shorter than minChars are not compacted (not worth it).
//
// The model retains all its assistant messages (reasoning, analysis, findings) in full.
// Only the raw tool result data from older turns gets replaced with a summary line.
func compactOldToolResults(messages []providers.Message, currentTurnStartIndex, keepTurns, minChars int, ka *KnowledgeAccumulator) {
	if currentTurnStartIndex <= 0 || keepTurns < 0 {
		return
	}

	// Walk backwards from currentTurnStartIndex to find the compaction boundary.
	// We keep the last keepTurns turns' tool results in full. Each "turn" starts
	// with an assistant message. Once we've skipped keepTurns assistant messages,
	// everything before that point is old enough to compact.
	var compactBefore int
	if keepTurns <= 0 {
		// Compact everything before the current turn
		compactBefore = currentTurnStartIndex
	} else {
		turnsFound := 0
		for i := currentTurnStartIndex - 1; i >= 0; i-- {
			if messages[i].Role == "assistant" {
				turnsFound++
				if turnsFound >= keepTurns {
					// This is the keepTurns-th assistant message from the end.
					// Everything before this index is old enough to compact.
					compactBefore = i
					break
				}
			}
		}
	}

	// Nothing old enough to compact
	if compactBefore <= 0 {
		return
	}

	// Build a map of tool call ID -> (name, input) from assistant messages,
	// so we can label compacted results with the tool name and key params.
	toolCallInfo := make(map[string]struct {
		Name  string
		Input map[string]interface{}
	})
	for i := 0; i < compactBefore; i++ {
		msg := messages[i]
		if msg.Role == "assistant" {
			for _, tc := range msg.ToolCalls {
				toolCallInfo[tc.ID] = struct {
					Name  string
					Input map[string]interface{}
				}{Name: tc.Name, Input: tc.Input}
			}
		}
	}

	// Compact tool results before the boundary
	compacted := 0
	savedChars := 0
	for i := 0; i < compactBefore; i++ {
		msg := &messages[i]
		if msg.ToolResult == nil || msg.ToolResult.IsError {
			continue
		}
		content := msg.ToolResult.Content
		if len(content) < minChars {
			continue
		}

		// Build summary
		toolName := "unknown_tool"
		var toolInput map[string]interface{}
		if info, ok := toolCallInfo[msg.ToolResult.ToolUseID]; ok {
			toolName = info.Name
			toolInput = info.Input
		}

		summary := buildCompactSummary(toolName, toolInput, content, ka, msg.ToolResult.ToolUseID)
		savedChars += len(content) - len(summary)
		msg.ToolResult.Content = summary
		compacted++
	}

	if compacted > 0 {
		log.Info().
			Int("compacted_results", compacted).
			Int("saved_chars", savedChars).
			Int("compact_before_index", compactBefore).
			Int("total_messages", len(messages)).
			Msg("[AgenticLoop] Compacted old tool results to reduce context size")
	}
}

// buildCompactSummary creates a short summary line for a compacted tool result.
// When a KnowledgeAccumulator is provided and has facts for this tool_use_id,
// the summary includes those facts so the model knows what it learned.
func buildCompactSummary(toolName string, toolInput map[string]interface{}, originalContent string, ka *KnowledgeAccumulator, toolUseID string) string {
	params := formatKeyParams(toolInput)
	charCount := len(originalContent)

	// Try to include KA facts for this specific tool call
	if ka != nil && toolUseID != "" {
		if factSummary := ka.FactSummaryForTool(toolUseID); factSummary != "" {
			var summary string
			if params != "" {
				summary = fmt.Sprintf("[Compacted: %s(%s) — Key facts: %s]",
					toolName, params, factSummary)
			} else {
				summary = fmt.Sprintf("[Compacted: %s — Key facts: %s]",
					toolName, factSummary)
			}
			log.Info().
				Str("tool", toolName).
				Str("tool_use_id", toolUseID).
				Int("original_chars", charCount).
				Int("summary_chars", len(summary)).
				Msg("[SmartCompaction] Used KA facts for compacted summary")
			return summary
		}
	}

	// Fallback: generic format when no KA facts available
	lineCount := strings.Count(originalContent, "\n") + 1
	if params != "" {
		return fmt.Sprintf("[Tool result compacted: %s(%s) — %d chars, %d lines. Full data was provided to the model in an earlier turn and has already been processed.]",
			toolName, params, charCount, lineCount)
	}
	return fmt.Sprintf("[Tool result compacted: %s — %d chars, %d lines. Full data was provided to the model in an earlier turn and has already been processed.]",
		toolName, charCount, lineCount)
}

// formatKeyParams extracts the most important parameters from tool input for display.
func formatKeyParams(input map[string]interface{}) string {
	if len(input) == 0 {
		return ""
	}

	// Priority keys that are most informative
	priorityKeys := []string{"type", "resource_id", "action", "host", "node", "instance", "query", "command", "period"}
	var parts []string

	for _, key := range priorityKeys {
		if val, ok := input[key]; ok {
			if str, ok := val.(string); ok && str != "" {
				parts = append(parts, fmt.Sprintf("%s=%s", key, str))
			}
		}
	}

	// If nothing from priority keys, take the first 2 non-empty string values
	if len(parts) == 0 {
		for key, val := range input {
			if str, ok := val.(string); ok && str != "" {
				parts = append(parts, fmt.Sprintf("%s=%s", key, str))
				if len(parts) >= 2 {
					break
				}
			}
		}
	}

	return strings.Join(parts, ", ")
}
