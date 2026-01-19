// Package chat provides direct AI chat integration without external sidecar processes
package chat

import (
	"encoding/json"
	"time"
)

// Session represents a chat session
type Session struct {
	ID           string    `json:"id"`
	Title        string    `json:"title,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count,omitempty"`
}

// Message represents a chat message
type Message struct {
	ID               string      `json:"id"`
	Role             string      `json:"role"` // "user", "assistant", "system"
	Content          string      `json:"content"`
	ReasoningContent string      `json:"reasoning_content,omitempty"` // For extended thinking
	ToolCalls        []ToolCall  `json:"tool_calls,omitempty"`
	ToolResult       *ToolResult `json:"tool_result,omitempty"`
	Timestamp        time.Time   `json:"timestamp"`
}

// ToolCall represents a tool invocation
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

// StreamEvent represents a streaming event sent to the frontend
type StreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// StreamCallback is called for each streaming event
type StreamCallback func(event StreamEvent)

// ExecuteRequest represents a chat execution request
type ExecuteRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id,omitempty"`
	Model     string `json:"model,omitempty"`
}

// QuestionAnswer represents a user's answer to a question
type QuestionAnswer struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// ContentData is the data for "content" events
type ContentData struct {
	Text string `json:"text"`
}

// ThinkingData is the data for "thinking" events (extended thinking/reasoning)
type ThinkingData struct {
	Text string `json:"text"`
}

// ToolStartData is the data for "tool_start" events
type ToolStartData struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input string `json:"input"` // JSON string of input parameters
}

// ToolEndData is the data for "tool_end" events
type ToolEndData struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Input   string `json:"input,omitempty"`
	Output  string `json:"output,omitempty"`
	Success bool   `json:"success"`
}

// ApprovalNeededData is the data for "approval_needed" events
type ApprovalNeededData struct {
	ApprovalID  string `json:"approval_id"`
	ToolID      string `json:"tool_id"`
	ToolName    string `json:"tool_name"`
	Command     string `json:"command"`
	RunOnHost   bool   `json:"run_on_host"`
	TargetHost  string `json:"target_host,omitempty"`
	Risk        string `json:"risk,omitempty"`
	Description string `json:"description,omitempty"`
}

// QuestionData is the data for "question" events
type QuestionData struct {
	QuestionID string     `json:"question_id"`
	Questions  []Question `json:"questions"`
}

// Question represents a question from the AI to the user
type Question struct {
	ID       string           `json:"id"`
	Question string           `json:"question"`
	Header   string           `json:"header,omitempty"`
	Options  []QuestionOption `json:"options,omitempty"`
}

// QuestionOption represents an option for a question
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// ErrorData is the data for "error" events
type ErrorData struct {
	Message string `json:"message"`
}

// DoneData is the data for "done" events
type DoneData struct {
	SessionID    string `json:"session_id,omitempty"`
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

// Control level constants
const (
	ControlLevelReadOnly   = "read_only"
	ControlLevelSuggest    = "suggest"
	ControlLevelControlled = "controlled"
	ControlLevelAutonomous = "autonomous"
)

// Max turns for the agentic loop to prevent infinite loops
const MaxAgenticTurns = 20
