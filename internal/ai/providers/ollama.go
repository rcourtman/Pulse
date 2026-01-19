package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaClient implements the Provider interface for Ollama's local API
type OllamaClient struct {
	model   string
	baseURL string
	client  *http.Client
}

// NewOllamaClient creates a new Ollama API client
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewOllamaClient(model, baseURL string, timeout time.Duration) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	// Normalize the URL: strip trailing slashes and /api suffix
	// Users sometimes enter http://host:11434/ or http://host:11434/api
	baseURL = strings.TrimSuffix(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/api")
	baseURL = strings.TrimSuffix(baseURL, "/") // In case it was /api/
	if timeout <= 0 {
		timeout = 300 * time.Second // Default 5 minutes
	}
	return &OllamaClient{
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the provider name
func (c *OllamaClient) Name() string {
	return "ollama"
}

// ollamaRequest is the request body for the Ollama API
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
	Tools    []ollamaTool    `json:"tools,omitempty"` // Tool definitions for function calling
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"` // For assistant messages with tool calls
}

type ollamaToolCall struct {
	ID       string             `json:"id,omitempty"` // Ollama provides an ID for tool calls
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Index     int                    `json:"index,omitempty"` // Index in the tool call array
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ollamaTool represents a tool definition for Ollama
type ollamaTool struct {
	Type     string             `json:"type"` // "function"
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ollamaOptions struct {
	NumPredict  int     `json:"num_predict,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// ollamaResponse is the response from the Ollama API
type ollamaResponse struct {
	Model           string            `json:"model"`
	CreatedAt       string            `json:"created_at"`
	Message         ollamaMessageResp `json:"message"`
	Done            bool              `json:"done"`
	DoneReason      string            `json:"done_reason,omitempty"`
	TotalDuration   int64             `json:"total_duration,omitempty"`
	LoadDuration    int64             `json:"load_duration,omitempty"`
	PromptEvalCount int               `json:"prompt_eval_count,omitempty"`
	EvalCount       int               `json:"eval_count,omitempty"`
}

// ollamaMessageResp is the response message format (can include tool_calls)
type ollamaMessageResp struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

// Chat sends a chat request to the Ollama API
func (c *OllamaClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert messages to Ollama format
	messages := make([]ollamaMessage, 0, len(req.Messages)+1)

	// Add system message if provided
	if req.System != "" {
		messages = append(messages, ollamaMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		msg := ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		// Include tool calls for assistant messages (for multi-turn with tool use)
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, ollamaToolCall{
					Function: ollamaFunctionCall{
						Name:      tc.Name,
						Arguments: tc.Input,
					},
				})
			}
		}
		// Handle tool results - Ollama expects role "tool" with content
		if m.ToolResult != nil {
			msg.Role = "tool"
			msg.Content = m.ToolResult.Content
		}
		messages = append(messages, msg)
	}

	// Use provided model or fall back to client default
	model := req.Model
	// Strip "ollama:" prefix if present - callers may pass the full "provider:model" string
	if strings.HasPrefix(model, "ollama:") {
		model = strings.TrimPrefix(model, "ollama:")
	}
	if model == "" {
		model = c.model
	}
	// Ultimate fallback - if no model configured anywhere, use llama3
	if model == "" {
		model = "llama3"
	}

	ollamaReq := ollamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   false, // Non-streaming for now
	}

	// Convert tools to Ollama format
	if len(req.Tools) > 0 {
		ollamaReq.Tools = make([]ollamaTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			// Skip non-function tools (like web_search which Ollama doesn't support)
			if t.Type != "" && t.Type != "function" {
				continue
			}
			ollamaReq.Tools = append(ollamaReq.Tools, ollamaTool{
				Type: "function",
				Function: ollamaToolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
	}

	if req.MaxTokens > 0 || req.Temperature > 0 {
		ollamaReq.Options = &ollamaOptions{}
		if req.MaxTokens > 0 {
			ollamaReq.Options.NumPredict = req.MaxTokens
		}
		if req.Temperature > 0 {
			ollamaReq.Options.Temperature = req.Temperature
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Build response with tool calls if present
	chatResp := &ChatResponse{
		Content:      ollamaResp.Message.Content,
		Model:        ollamaResp.Model,
		StopReason:   ollamaResp.DoneReason,
		InputTokens:  ollamaResp.PromptEvalCount,
		OutputTokens: ollamaResp.EvalCount,
	}

	// Convert Ollama tool calls to our format
	if len(ollamaResp.Message.ToolCalls) > 0 {
		chatResp.StopReason = "tool_use" // Signal that we need to execute tools
		for _, tc := range ollamaResp.Message.ToolCalls {
			// Use Ollama's ID if provided, otherwise generate one
			toolCallID := tc.ID
			if toolCallID == "" {
				toolCallID = fmt.Sprintf("ollama_%s_%d", tc.Function.Name, time.Now().UnixNano())
			}
			chatResp.ToolCalls = append(chatResp.ToolCalls, ToolCall{
				ID:    toolCallID,
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments,
			})
		}
	}

	return chatResp, nil
}

// SupportsThinking returns true if the model supports extended thinking
func (c *OllamaClient) SupportsThinking(model string) bool {
	// Ollama models don't currently support extended thinking in stream output
	return false
}

// ollamaStreamResponse is a single chunk from the Ollama streaming API
type ollamaStreamResponse struct {
	Model           string            `json:"model"`
	CreatedAt       string            `json:"created_at"`
	Message         ollamaMessageResp `json:"message"`
	Done            bool              `json:"done"`
	DoneReason      string            `json:"done_reason,omitempty"`
	TotalDuration   int64             `json:"total_duration,omitempty"`
	LoadDuration    int64             `json:"load_duration,omitempty"`
	PromptEvalCount int               `json:"prompt_eval_count,omitempty"`
	EvalCount       int               `json:"eval_count,omitempty"`
}

// ChatStream sends a chat request and streams the response via callback
func (c *OllamaClient) ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	// Convert messages to Ollama format (same as Chat)
	messages := make([]ollamaMessage, 0, len(req.Messages)+1)

	if req.System != "" {
		messages = append(messages, ollamaMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		msg := ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				msg.ToolCalls = append(msg.ToolCalls, ollamaToolCall{
					Function: ollamaFunctionCall{
						Name:      tc.Name,
						Arguments: tc.Input,
					},
				})
			}
		}
		if m.ToolResult != nil {
			msg.Role = "tool"
			msg.Content = m.ToolResult.Content
		}
		messages = append(messages, msg)
	}

	model := req.Model
	if strings.HasPrefix(model, "ollama:") {
		model = strings.TrimPrefix(model, "ollama:")
	}
	if model == "" {
		model = c.model
	}
	if model == "" {
		model = "llama3"
	}

	ollamaReq := ollamaRequest{
		Model:    model,
		Messages: messages,
		Stream:   true, // Enable streaming
	}

	if len(req.Tools) > 0 {
		ollamaReq.Tools = make([]ollamaTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			if t.Type != "" && t.Type != "function" {
				continue
			}
			ollamaReq.Tools = append(ollamaReq.Tools, ollamaTool{
				Type: "function",
				Function: ollamaToolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
	}

	if req.MaxTokens > 0 || req.Temperature > 0 {
		ollamaReq.Options = &ollamaOptions{}
		if req.MaxTokens > 0 {
			ollamaReq.Options.NumPredict = req.MaxTokens
		}
		if req.Temperature > 0 {
			ollamaReq.Options.Temperature = req.Temperature
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse NDJSON stream (newline-delimited JSON, not SSE)
	decoder := json.NewDecoder(resp.Body)
	var toolCalls []ToolCall
	var inputTokens, outputTokens int
	var doneReason string

	for {
		var chunk ollamaStreamResponse
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("stream decode error: %w", err)
		}

		// Regular content
		if chunk.Message.Content != "" {
			callback(StreamEvent{
				Type: "content",
				Data: ContentEvent{Text: chunk.Message.Content},
			})
		}

		// Tool calls
		for _, tc := range chunk.Message.ToolCalls {
			toolCallID := tc.ID
			if toolCallID == "" {
				toolCallID = fmt.Sprintf("ollama_%s_%d", tc.Function.Name, len(toolCalls))
			}
			callback(StreamEvent{
				Type: "tool_start",
				Data: ToolStartEvent{
					ID:    toolCallID,
					Name:  tc.Function.Name,
					Input: tc.Function.Arguments,
				},
			})
			toolCalls = append(toolCalls, ToolCall{
				ID:    toolCallID,
				Name:  tc.Function.Name,
				Input: tc.Function.Arguments,
			})
		}

		if chunk.Done {
			inputTokens = chunk.PromptEvalCount
			outputTokens = chunk.EvalCount
			doneReason = chunk.DoneReason
			break
		}
	}

	// Send done event
	stopReason := doneReason
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
	} else if stopReason == "" || stopReason == "stop" {
		stopReason = "end_turn"
	}

	callback(StreamEvent{
		Type: "done",
		Data: DoneEvent{
			StopReason:   stopReason,
			ToolCalls:    toolCalls,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
		},
	})

	return nil
}

// TestConnection validates connectivity by checking the Ollama version endpoint
func (c *OllamaClient) TestConnection(ctx context.Context) error {
	url := c.baseURL + "/api/version"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	return nil
}

// ListModels fetches available models from the local Ollama instance
func (c *OllamaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := c.baseURL + "/api/tags"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Models))
	for _, m := range result.Models {
		models = append(models, ModelInfo{
			ID:      m.Name,
			Name:    m.Name,
			Notable: true, // Ollama models are always notable - user explicitly pulled them
		})
	}

	return models, nil
}
