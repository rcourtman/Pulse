// Package providers contains AI provider client implementations
package providers

import (
	"context"
	"encoding/json"
)

// Message represents a chat message
type Message struct {
	Role             string      `json:"role"`                        // "user", "assistant", "system"
	Content          string      `json:"content"`                     // Text content (simple case)
	ReasoningContent string      `json:"reasoning_content,omitempty"` // DeepSeek thinking mode
	ToolCalls        []ToolCall  `json:"tool_calls"`                  // For assistant messages with tool calls
	ToolResult       *ToolResult `json:"tool_result,omitempty"`       // For user messages with tool results
}

func EmptyMessage() Message {
	return Message{}.NormalizeCollections()
}

func (m Message) NormalizeCollections() Message {
	if m.ToolCalls == nil {
		m.ToolCalls = []ToolCall{}
	}
	for i := range m.ToolCalls {
		m.ToolCalls[i] = m.ToolCalls[i].NormalizeCollections()
	}
	return m
}

// ToolCall represents a tool invocation from the AI
type ToolCall struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Input            map[string]interface{} `json:"input"`
	ThoughtSignature json.RawMessage        `json:"thought_signature,omitempty"`
}

func EmptyToolCall() ToolCall {
	return ToolCall{}.NormalizeCollections()
}

func (t ToolCall) NormalizeCollections() ToolCall {
	if t.Input == nil {
		t.Input = map[string]interface{}{}
	}
	return t
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
	InputSchema map[string]interface{} `json:"input_schema"`
	MaxUses     int                    `json:"max_uses,omitempty"` // For web search: limit searches per request
}

func EmptyTool() Tool {
	return Tool{}.NormalizeCollections()
}

func (t Tool) NormalizeCollections() Tool {
	if t.InputSchema == nil {
		t.InputSchema = map[string]interface{}{}
	}
	return t
}

// ToolChoiceType represents how the model should choose tools
type ToolChoiceType string

const (
	// ToolChoiceAuto lets the model decide whether to use tools (default)
	ToolChoiceAuto ToolChoiceType = "auto"
	// ToolChoiceAny forces the model to use one of the provided tools
	ToolChoiceAny ToolChoiceType = "any"
	// ToolChoiceNone prevents the model from using any tools
	ToolChoiceNone ToolChoiceType = "none"
	// ToolChoiceTool forces the model to use a specific tool (set ToolName)
	ToolChoiceTool ToolChoiceType = "tool"
)

// ToolChoice controls how the model selects tools
type ToolChoice struct {
	Type ToolChoiceType `json:"type"`
	Name string         `json:"name,omitempty"` // Only used when Type is ToolChoiceTool
}

// ChatRequest represents a request to the AI provider
type ChatRequest struct {
	Messages    []Message   `json:"messages"`
	Model       string      `json:"model"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Temperature float64     `json:"temperature,omitempty"`
	System      string      `json:"system,omitempty"`      // System prompt (Anthropic style)
	Tools       []Tool      `json:"tools,omitempty"`       // Available tools
	ToolChoice  *ToolChoice `json:"tool_choice,omitempty"` // How to select tools (nil = auto)
}

func (r ChatRequest) NormalizeCollections() ChatRequest {
	if r.Messages == nil {
		r.Messages = []Message{}
	}
	for i := range r.Messages {
		r.Messages[i] = r.Messages[i].NormalizeCollections()
	}
	if r.Tools == nil {
		r.Tools = []Tool{}
	}
	for i := range r.Tools {
		r.Tools[i] = r.Tools[i].NormalizeCollections()
	}
	return r
}

// ChatResponse represents a response from the AI provider
type ChatResponse struct {
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"` // DeepSeek thinking mode
	Model            string     `json:"model"`
	StopReason       string     `json:"stop_reason,omitempty"` // "end_turn", "tool_use"
	ToolCalls        []ToolCall `json:"tool_calls"`            // Tool invocations
	InputTokens      int        `json:"input_tokens,omitempty"`
	OutputTokens     int        `json:"output_tokens,omitempty"`
}

func EmptyChatResponse() ChatResponse {
	return ChatResponse{}.NormalizeCollections()
}

func (r ChatResponse) NormalizeCollections() ChatResponse {
	if r.ToolCalls == nil {
		r.ToolCalls = []ToolCall{}
	}
	for i := range r.ToolCalls {
		r.ToolCalls[i] = r.ToolCalls[i].NormalizeCollections()
	}
	return r
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
	Input map[string]interface{} `json:"input"`
}

func (e ToolStartEvent) NormalizeCollections() ToolStartEvent {
	if e.Input == nil {
		e.Input = map[string]interface{}{}
	}
	return e
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
	ToolCalls    []ToolCall `json:"tool_calls"`
	InputTokens  int        `json:"input_tokens,omitempty"`
	OutputTokens int        `json:"output_tokens,omitempty"`
}

func (e DoneEvent) NormalizeCollections() DoneEvent {
	if e.ToolCalls == nil {
		e.ToolCalls = []ToolCall{}
	}
	for i := range e.ToolCalls {
		e.ToolCalls[i] = e.ToolCalls[i].NormalizeCollections()
	}
	return e
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
