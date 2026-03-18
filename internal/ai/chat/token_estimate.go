package chat

import (
	"encoding/json"
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

const (
	charsPerToken        = 4
	messageTokenOverhead = 4
)

// EstimateTokens returns an approximate token count for a text string.
// Uses ~4 chars/token heuristic, accurate enough for budgeting.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	return (len(text) + charsPerToken - 1) / charsPerToken
}

// EstimateMessagesTokens estimates total tokens across all messages,
// including content, tool calls (serialized input), and tool results.
func EstimateMessagesTokens(msgs []providers.Message) int {
	if len(msgs) == 0 {
		return 0
	}

	total := 0
	for _, msg := range msgs {
		total += messageTokenOverhead
		total += EstimateTokens(msg.Content)
		total += EstimateTokens(msg.ReasoningContent)

		for _, call := range msg.ToolCalls {
			total += EstimateTokens(call.Name)
			total += estimateJSONTokens(call.Input)
		}

		if msg.ToolResult != nil {
			total += EstimateTokens(msg.ToolResult.Content)
		}
	}

	return total
}

// EstimateToolsTokens estimates tokens for tool definitions by
// serializing their schemas to JSON.
func EstimateToolsTokens(tools []providers.Tool) int {
	if len(tools) == 0 {
		return 0
	}

	total := 0
	for _, tool := range tools {
		total += estimateJSONTokens(tool)
	}

	return total
}

// EstimateRequestTokens estimates total input tokens for a ChatRequest:
// system prompt + messages + tools.
func EstimateRequestTokens(req providers.ChatRequest) int {
	return EstimateTokens(req.System) +
		EstimateMessagesTokens(req.Messages) +
		EstimateToolsTokens(req.Tools)
}

func estimateJSONTokens(v interface{}) int {
	if v == nil {
		return 0
	}

	b, err := json.Marshal(v)
	if err != nil {
		return EstimateTokens(fmt.Sprintf("%v", v))
	}
	return EstimateTokens(string(b))
}
