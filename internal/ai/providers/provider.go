// Package providers contains AI provider client implementations
package providers

import (
	"context"
)

// Message represents a chat message
type Message struct {
	Role             string      `json:"role"`                        // "user", "assistant", "system"
	Content          string      `json:"content"`                     // Text content (simple case)
	ReasoningContent string      `json:"reasoning_content,omitempty"` // DeepSeek thinking mode
	ToolCalls        []ToolCall  `json:"tool_calls,omitempty"`        // For assistant messages with tool calls
	ToolResult       *ToolResult `json:"tool_result,omitempty"`       // For user messages with tool results
}

// ToolCall represents a tool invocation from the AI
type ToolCall struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Tool represents an AI tool definition
type Tool struct {
	Type        string                 `json:"type,omitempty"` // "web_search_20250305" for web search, empty for regular tools
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
	MaxUses     int                    `json:"max_uses,omitempty"` // For web search: limit searches per request
}

// ChatRequest represents a request to the AI provider
type ChatRequest struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	System      string    `json:"system,omitempty"` // System prompt (Anthropic style)
	Tools       []Tool    `json:"tools,omitempty"`  // Available tools
}

// ChatResponse represents a response from the AI provider
type ChatResponse struct {
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"` // DeepSeek thinking mode
	Model            string     `json:"model"`
	StopReason       string     `json:"stop_reason,omitempty"` // "end_turn", "tool_use"
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`  // Tool invocations
	InputTokens      int        `json:"input_tokens,omitempty"`
	OutputTokens     int        `json:"output_tokens,omitempty"`
}

// ModelInfo represents information about an available model
type ModelInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   int64  `json:"created_at,omitempty"`
	Notable     bool   `json:"notable"` // Whether this is a "latest and greatest" model
}

// Provider defines the interface for AI providers
type Provider interface {
	// Chat sends a chat request and returns the response
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// TestConnection validates the API key and connectivity
	TestConnection(ctx context.Context) error

	// Name returns the provider name
	Name() string

	// ListModels returns available models from the provider's API
	// Returns nil if the provider doesn't support listing models
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// StreamEvent represents a streaming event from the AI provider
type StreamEvent struct {
	Type string      // "content", "thinking", "tool_start", "tool_end", "done", "error"
	Data interface{} // Type-specific data
}

// ContentEvent is the data for "content" stream events
type ContentEvent struct {
	Text string `json:"text"`
}

// ThinkingEvent is the data for "thinking" stream events (extended thinking/reasoning)
type ThinkingEvent struct {
	Text string `json:"text"`
}

// ToolStartEvent is the data for "tool_start" stream events
type ToolStartEvent struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input,omitempty"`
}

// ToolEndEvent is the data for "tool_end" stream events
type ToolEndEvent struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Result  string `json:"result,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

// ErrorEvent is the data for "error" stream events
type ErrorEvent struct {
	Message string `json:"message"`
}

// DoneEvent is the data for "done" stream events
type DoneEvent struct {
	StopReason   string     `json:"stop_reason,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	InputTokens  int        `json:"input_tokens,omitempty"`
	OutputTokens int        `json:"output_tokens,omitempty"`
}

// StreamCallback is called for each streaming event
type StreamCallback func(event StreamEvent)

// StreamingProvider extends Provider with streaming support
type StreamingProvider interface {
	Provider
	// ChatStream sends a chat request and streams the response via callback
	ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) error
	// SupportsThinking returns true if the model supports extended thinking/reasoning
	SupportsThinking(model string) bool
}
