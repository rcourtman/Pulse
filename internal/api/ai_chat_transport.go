package api

import (
	"encoding/json"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
)

// chatMessageTruncationNotice is appended to a message whose content had to be
// cut to fit a transport byte budget. The full text stays available in the
// Pulse web UI, which does not go through the size-capped relay proxy.
const chatMessageTruncationNotice = "\n\n[Message shortened for mobile. Open Pulse in a browser for the full text.]"

// compactChatMessageForTransport strips message fields that constrained
// clients (the mobile app behind the relay proxy's 47KB response cap) never
// render: tool outputs, tool result bodies, and provider thought signatures.
// Tool identity and the tool_use_id linkage used to mark tools completed are
// preserved.
func compactChatMessageForTransport(m chat.Message) chat.Message {
	for i := range m.ToolCalls {
		m.ToolCalls[i].Output = ""
		m.ToolCalls[i].ThoughtSignature = nil
	}
	if m.ToolResult != nil {
		m.ToolResult = &chat.ToolResult{
			ToolUseID: m.ToolResult.ToolUseID,
			IsError:   m.ToolResult.IsError,
		}
	}
	return m
}

// trimChatMessagesToByteBudget returns the newest messages whose combined JSON
// encoding fits within budget bytes (including array brackets and separators).
// The newest message is always returned; if it alone exceeds the budget its
// content is truncated to fit.
func trimChatMessagesToByteBudget(messages []chat.Message, budget int) []chat.Message {
	if len(messages) == 0 || budget <= 0 {
		return messages
	}

	total := 2 // array brackets
	kept := 0
	for i := len(messages) - 1; i >= 0; i-- {
		encoded, err := json.Marshal(messages[i])
		if err != nil {
			break
		}
		cost := len(encoded)
		if kept > 0 {
			cost++ // separating comma
		}
		if total+cost > budget {
			break
		}
		total += cost
		kept++
	}

	if kept == 0 {
		return []chat.Message{truncateChatMessageContent(messages[len(messages)-1], budget-2)}
	}
	return messages[len(messages)-kept:]
}

// truncateChatMessageContent shrinks a message's content until its JSON
// encoding fits within budget bytes. Non-content fields are left intact, so a
// message dominated by other fields may still exceed the budget; callers treat
// the result as best effort.
func truncateChatMessageContent(m chat.Message, budget int) chat.Message {
	original := m
	for {
		encoded, err := json.Marshal(m)
		if err != nil {
			return original
		}
		if len(encoded) <= budget {
			return m
		}
		content := strings.TrimSuffix(m.Content, chatMessageTruncationNotice)
		if len(content) == 0 {
			return m
		}
		cut := len(content) - (len(encoded) - budget) - len(chatMessageTruncationNotice)
		if cut < 0 {
			cut = 0
		}
		m.Content = strings.ToValidUTF8(content[:cut], "") + chatMessageTruncationNotice
	}
}
