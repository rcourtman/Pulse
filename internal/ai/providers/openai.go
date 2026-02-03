package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	openaiAPIURL         = "https://api.openai.com/v1/chat/completions"
	openaiMaxRetries     = 3
	openaiInitialBackoff = 2 * time.Second
)

// OpenAIClient implements the Provider interface for OpenAI's API
// Also works with OpenAI-compatible APIs like DeepSeek
type OpenAIClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIClient creates a new OpenAI API client
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewOpenAIClient(apiKey, model, baseURL string, timeout time.Duration) *OpenAIClient {
	if baseURL == "" {
		baseURL = openaiAPIURL
	} else {
		// Normalize baseURL: ensure it ends with /chat/completions for chat endpoint
		// Users may provide URLs like "https://openrouter.ai/api/v1" which need the path appended
		baseURL = strings.TrimSuffix(baseURL, "/")
		if !strings.HasSuffix(baseURL, "/chat/completions") {
			// If URL ends with /v1, append /chat/completions
			// Otherwise append /v1/chat/completions
			if strings.HasSuffix(baseURL, "/v1") {
				baseURL = baseURL + "/chat/completions"
			} else if strings.HasSuffix(baseURL, "/completions") {
				// URL already has /completions, make it /chat/completions
				baseURL = strings.TrimSuffix(baseURL, "/completions") + "/chat/completions"
			} else {
				// Assume it's a base URL, append full path
				baseURL = baseURL + "/v1/chat/completions"
			}
		}
	}
	// Strip provider prefix if present - the model should be just the model name
	if strings.HasPrefix(model, "openai:") {
		model = strings.TrimPrefix(model, "openai:")
	} else if strings.HasPrefix(model, "deepseek:") {
		model = strings.TrimPrefix(model, "deepseek:")
	}
	if timeout <= 0 {
		timeout = 300 * time.Second // Default 5 minutes
	}
	return &OpenAIClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the provider name
func (c *OpenAIClient) Name() string {
	return "openai"
}

// openaiRequest is the request body for the OpenAI API
type openaiRequest struct {
	Model               string          `json:"model"`
	Messages            []openaiMessage `json:"messages"`
	MaxTokens           int             `json:"max_tokens,omitempty"`            // Legacy parameter for older models
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"` // For o1/o3 models
	Temperature         float64         `json:"temperature,omitempty"`
	Tools               []openaiTool    `json:"tools,omitempty"`
	ToolChoice          interface{}     `json:"tool_choice,omitempty"` // "auto", "none", or specific tool
}

// openaiTool represents a function tool in OpenAI format
type openaiTool struct {
	Type     string         `json:"type"` // always "function"
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type openaiMessage struct {
	Role             string           `json:"role"`
	Content          interface{}      `json:"content,omitempty"`           // string or null for tool calls
	ReasoningContent string           `json:"reasoning_content,omitempty"` // DeepSeek thinking mode
	ToolCalls        []openaiToolCall `json:"tool_calls,omitempty"`        // For assistant messages with tool calls
	ToolCallID       string           `json:"tool_call_id,omitempty"`      // For tool response messages
}

type openaiToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"` // always "function"
	Function openaiToolFunction `json:"function"`
}

type openaiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string of arguments
}

// openaiResponse is the response from the OpenAI API
type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiRespMsg `json:"message"`
	FinishReason string        `json:"finish_reason"` // "stop", "tool_calls", etc.
}

type openaiRespMsg struct {
	Role             string           `json:"role"`
	Content          string           `json:"content,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"` // DeepSeek thinking mode
	ToolCalls        []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiError struct {
	Error openaiErrorDetail `json:"error"`
}

type openaiErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// isDeepSeek returns true if this client is configured for DeepSeek
func (c *OpenAIClient) isDeepSeek() bool {
	return strings.Contains(c.baseURL, "deepseek.com")
}

// isDeepSeekReasoner returns true if using DeepSeek's reasoning model
func (c *OpenAIClient) isDeepSeekReasoner() bool {
	return c.isDeepSeek() && strings.Contains(c.model, "reasoner")
}

// requiresMaxCompletionTokens returns true for models that need max_completion_tokens instead of max_tokens
// Per OpenAI docs, o1/o3/o4 reasoning models require max_completion_tokens; max_tokens will error.
func (c *OpenAIClient) requiresMaxCompletionTokens(model string) bool {
	return strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4")
}

// convertToolChoiceToOpenAI converts our ToolChoice to OpenAI's format
// OpenAI uses "required" instead of Anthropic's "any" to force tool use
// See: https://platform.openai.com/docs/api-reference/chat/create#chat-create-tool_choice
func convertToolChoiceToOpenAI(tc *ToolChoice) interface{} {
	if tc == nil {
		return "auto"
	}
	switch tc.Type {
	case ToolChoiceAuto:
		return "auto"
	case ToolChoiceNone:
		return "none"
	case ToolChoiceAny:
		// OpenAI uses "required" to force the model to use one of the provided tools
		return "required"
	case ToolChoiceTool:
		// Force a specific tool
		return map[string]interface{}{
			"type": "function",
			"function": map[string]string{
				"name": tc.Name,
			},
		}
	default:
		return "auto"
	}
}

// Chat sends a chat request to the OpenAI API
func (c *OpenAIClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert messages to OpenAI format
	messages := make([]openaiMessage, 0, len(req.Messages)+1)

	// Add system message if provided
	if req.System != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		msg := openaiMessage{
			Role: m.Role,
		}

		// Handle tool calls in assistant messages
		if len(m.ToolCalls) > 0 {
			msg.Content = nil // Content is null when there are tool calls
			if m.Content != "" {
				msg.Content = m.Content
			}
			// For DeepSeek reasoner, include reasoning_content if present
			if c.isDeepSeekReasoner() && m.ReasoningContent != "" {
				msg.ReasoningContent = m.ReasoningContent
			}
			for _, tc := range m.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Input)
				msg.ToolCalls = append(msg.ToolCalls, openaiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openaiToolFunction{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		} else if m.ToolResult != nil {
			// This is a tool result message
			msg.Role = "tool"
			msg.Content = m.ToolResult.Content
			msg.ToolCallID = m.ToolResult.ToolUseID
		} else {
			msg.Content = m.Content
			// For assistant messages with reasoning content (DeepSeek)
			if c.isDeepSeekReasoner() && m.ReasoningContent != "" {
				msg.ReasoningContent = m.ReasoningContent
			}
		}

		messages = append(messages, msg)
	}

	// Use provided model or fall back to client default
	model := req.Model
	// Strip provider prefix if present - callers may pass the full "provider:model" string
	if strings.HasPrefix(model, "openai:") {
		model = strings.TrimPrefix(model, "openai:")
	} else if strings.HasPrefix(model, "deepseek:") {
		model = strings.TrimPrefix(model, "deepseek:")
	}
	if model == "" {
		model = c.model
	}

	// Debug log to trace model issues
	log.Debug().Str("model", model).Str("req_model", req.Model).Str("c_model", c.model).Str("base_url", c.baseURL).Msg("OpenAI/DeepSeek Chat request")

	// Build request
	openaiReq := openaiRequest{
		Model:    model,
		Messages: messages,
	}

	// Use max_completion_tokens for all OpenAI models (newer API, backward compatible)
	// DeepSeek still uses max_tokens
	if req.MaxTokens > 0 {
		if c.isDeepSeek() {
			openaiReq.MaxTokens = req.MaxTokens
		} else {
			openaiReq.MaxCompletionTokens = req.MaxTokens
		}
	}

	// DeepSeek reasoner and newer OpenAI models don't support temperature
	if req.Temperature > 0 && !c.isDeepSeekReasoner() && !c.requiresMaxCompletionTokens(model) {
		openaiReq.Temperature = req.Temperature
	}

	// Convert tools to OpenAI format
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			// Skip non-function tools (like web_search)
			if t.Type != "" && t.Type != "function" {
				continue
			}
			openaiReq.Tools = append(openaiReq.Tools, openaiTool{
				Type: "function",
				Function: openaiFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
		if len(openaiReq.Tools) > 0 && !c.isDeepSeekReasoner() {
			// Map ToolChoice to OpenAI format
			// OpenAI uses "required" instead of Anthropic's "any"
			// DeepSeek Reasoner does not support tool_choice — it decides tool use via reasoning
			openaiReq.ToolChoice = convertToolChoiceToOpenAI(req.ToolChoice)
		}
	}

	// Log actual model being sent (INFO level for visibility)
	log.Info().Str("model_in_request", openaiReq.Model).Str("base_url", c.baseURL).Msg("Sending OpenAI/DeepSeek request")

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop for transient errors (connection resets, 429, 5xx)
	var respBody []byte
	var lastErr error

	for attempt := 0; attempt <= openaiMaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			backoff := openaiInitialBackoff * time.Duration(1<<(attempt-1))
			log.Warn().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Str("last_error", lastErr.Error()).
				Msg("Retrying OpenAI/DeepSeek API request after transient error")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.client.Do(httpReq)
		if err != nil {
			// Check if this is a retryable connection error
			errStr := err.Error()
			if strings.Contains(errStr, "connection reset") ||
				strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "EOF") ||
				strings.Contains(errStr, "timeout") {
				lastErr = fmt.Errorf("connection error: %w", err)
				continue
			}
			return nil, fmt.Errorf("request failed: %w", err)
		}

		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// Check for retryable HTTP errors
		if resp.StatusCode == 429 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			var errResp openaiError
			errMsg := string(respBody)
			if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
				errMsg = errResp.Error.Message
			}
			errMsg = appendRateLimitInfo(errMsg, resp)
			lastErr = fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
			continue
		}

		// Non-retryable error
		if resp.StatusCode != http.StatusOK {
			var errResp openaiError
			if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
				errMsg := appendRateLimitInfo(errResp.Error.Message, resp)
				return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
			}
			errMsg := appendRateLimitInfo(string(respBody), resp)
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
		}

		// Success - break out of retry loop
		lastErr = nil
		break
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", openaiMaxRetries, lastErr)
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	choice := openaiResp.Choices[0]

	// For DeepSeek reasoner, the actual content may be in reasoning_content
	// when content is empty (it shows the "thinking" but that's the full response)
	contentToUse := choice.Message.Content
	if contentToUse == "" && choice.Message.ReasoningContent != "" {
		// DeepSeek reasoner puts output in reasoning_content
		contentToUse = choice.Message.ReasoningContent
	}

	result := &ChatResponse{
		Content:          contentToUse,
		ReasoningContent: choice.Message.ReasoningContent, // DeepSeek thinking mode
		Model:            openaiResp.Model,
		StopReason:       choice.FinishReason,
		InputTokens:      openaiResp.Usage.PromptTokens,
		OutputTokens:     openaiResp.Usage.CompletionTokens,
	}

	// Convert tool calls from OpenAI format to our format
	if len(choice.Message.ToolCalls) > 0 {
		result.StopReason = "tool_use" // Normalize to match Anthropic's format
		for _, tc := range choice.Message.ToolCalls {
			var input map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = map[string]interface{}{"raw": tc.Function.Arguments}
			}
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			})
		}
	}

	return result, nil
}

// TestConnection validates the API key by listing models
// This avoids dependencies on specific model names which may get deprecated
func (c *OpenAIClient) TestConnection(ctx context.Context) error {
	_, err := c.ListModels(ctx)
	return err
}

func (c *OpenAIClient) modelsEndpoint() string {
	// Default to public API endpoints to preserve current behavior if baseURL is invalid.
	modelsURL := "https://api.openai.com/v1/models"
	if c.isDeepSeek() {
		modelsURL = "https://api.deepseek.com/models"
	}

	u, err := url.Parse(c.baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return modelsURL
	}

	// For custom endpoints, replace /chat/completions with /models in the path
	// This preserves any custom path structure (e.g., /api/v1 for OpenRouter)
	if c.isDeepSeek() {
		return u.Scheme + "://" + u.Host + "/models"
	}

	// Replace /chat/completions with /models to get the models endpoint
	// baseURL is already normalized to end with /chat/completions
	path := u.Path
	if strings.HasSuffix(path, "/chat/completions") {
		path = strings.TrimSuffix(path, "/chat/completions") + "/models"
	} else {
		// Fallback: strip path and use /v1/models
		path = "/v1/models"
	}
	return u.Scheme + "://" + u.Host + path
}

// SupportsThinking returns true if the model supports extended thinking
func (c *OpenAIClient) SupportsThinking(model string) bool {
	// DeepSeek reasoner models support extended thinking
	if c.isDeepSeek() && strings.Contains(model, "reasoner") {
		return true
	}
	// OpenAI o1/o3/o4 models have reasoning but not in the same streaming format
	return false
}

// openaiStreamRequest extends openaiRequest with streaming field
type openaiStreamRequest struct {
	Model               string          `json:"model"`
	Messages            []openaiMessage `json:"messages"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	Temperature         float64         `json:"temperature,omitempty"`
	Tools               []openaiTool    `json:"tools,omitempty"`
	ToolChoice          interface{}     `json:"tool_choice,omitempty"`
	Stream              bool            `json:"stream"`
	StreamOptions       *streamOptions  `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// openaiStreamEvent represents a streaming event from the OpenAI API
type openaiStreamEvent struct {
	ID      string               `json:"id"`
	Object  string               `json:"object"`
	Created int64                `json:"created"`
	Model   string               `json:"model"`
	Choices []openaiStreamChoice `json:"choices"`
	Usage   *openaiUsage         `json:"usage,omitempty"`
}

type openaiStreamChoice struct {
	Index        int               `json:"index"`
	Delta        openaiStreamDelta `json:"delta"`
	FinishReason string            `json:"finish_reason"`
}

type openaiStreamDelta struct {
	Role             string                `json:"role,omitempty"`
	Content          string                `json:"content,omitempty"`
	ReasoningContent string                `json:"reasoning_content,omitempty"`
	ToolCalls        []openaiToolCallDelta `json:"tool_calls,omitempty"`
}

type openaiToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

// ChatStream sends a chat request and streams the response via callback
func (c *OpenAIClient) ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	// Convert messages to OpenAI format (same as Chat)
	messages := make([]openaiMessage, 0, len(req.Messages)+1)

	if req.System != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		msg := openaiMessage{
			Role: m.Role,
		}

		if len(m.ToolCalls) > 0 {
			msg.Content = nil
			if m.Content != "" {
				msg.Content = m.Content
			}
			if c.isDeepSeekReasoner() && m.ReasoningContent != "" {
				msg.ReasoningContent = m.ReasoningContent
			}
			for _, tc := range m.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Input)
				msg.ToolCalls = append(msg.ToolCalls, openaiToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openaiToolFunction{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		} else if m.ToolResult != nil {
			msg.Role = "tool"
			msg.Content = m.ToolResult.Content
			msg.ToolCallID = m.ToolResult.ToolUseID
		} else {
			msg.Content = m.Content
			if c.isDeepSeekReasoner() && m.ReasoningContent != "" {
				msg.ReasoningContent = m.ReasoningContent
			}
		}

		messages = append(messages, msg)
	}

	model := req.Model
	if strings.HasPrefix(model, "openai:") {
		model = strings.TrimPrefix(model, "openai:")
	} else if strings.HasPrefix(model, "deepseek:") {
		model = strings.TrimPrefix(model, "deepseek:")
	}
	if model == "" {
		model = c.model
	}

	openaiReq := openaiStreamRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
		StreamOptions: &streamOptions{
			IncludeUsage: true,
		},
	}

	if req.MaxTokens > 0 {
		if c.isDeepSeek() {
			openaiReq.MaxTokens = req.MaxTokens
		} else {
			openaiReq.MaxCompletionTokens = req.MaxTokens
		}
	}

	if req.Temperature > 0 && !c.isDeepSeekReasoner() && !c.requiresMaxCompletionTokens(model) {
		openaiReq.Temperature = req.Temperature
	}

	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			if t.Type != "" && t.Type != "function" {
				continue
			}
			openaiReq.Tools = append(openaiReq.Tools, openaiTool{
				Type: "function",
				Function: openaiFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
		}
		if len(openaiReq.Tools) > 0 && !c.isDeepSeekReasoner() {
			// Map ToolChoice to OpenAI format (same as non-streaming)
			// DeepSeek Reasoner does not support tool_choice — it decides tool use via reasoning
			openaiReq.ToolChoice = convertToolChoiceToOpenAI(req.ToolChoice)
		}
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		var errResp openaiError
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			errMsg := appendRateLimitInfo(errResp.Error.Message, resp)
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
		}
		errMsg := appendRateLimitInfo(string(respBody), resp)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
	}

	// Parse SSE stream
	reader := resp.Body
	buf := make([]byte, 4096)
	var pendingData string
	var toolCalls []ToolCall
	toolCallBuilders := make(map[int]*struct {
		id   string
		name string
		args strings.Builder
	})
	var inputTokens, outputTokens int
	var finishReason string

	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("stream read error: %w", err)
		}

		pendingData += string(buf[:n])
		lines := strings.Split(pendingData, "\n")

		// Keep the last incomplete line for next iteration
		pendingData = lines[len(lines)-1]
		lines = lines[:len(lines)-1]

		for _, line := range lines {
			line = strings.TrimSpace(line)

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)

			if data == "[DONE]" {
				// Build final tool calls from builders
				for _, builder := range toolCallBuilders {
					var input map[string]interface{}
					if err := json.Unmarshal([]byte(builder.args.String()), &input); err != nil {
						input = map[string]interface{}{"raw": builder.args.String()}
					}
					toolCalls = append(toolCalls, ToolCall{
						ID:    builder.id,
						Name:  builder.name,
						Input: input,
					})
				}

				stopReason := finishReason
				if len(toolCalls) > 0 {
					stopReason = "tool_use"
				} else if stopReason == "stop" {
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

			var event openaiStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				log.Debug().Err(err).Str("data", data).Msg("Failed to parse stream event")
				continue
			}

			// Handle usage info
			if event.Usage != nil {
				inputTokens = event.Usage.PromptTokens
				outputTokens = event.Usage.CompletionTokens
			}

			for _, choice := range event.Choices {
				if choice.FinishReason != "" {
					finishReason = choice.FinishReason
				}

				delta := choice.Delta

				// Regular content
				if delta.Content != "" {
					callback(StreamEvent{
						Type: "content",
						Data: ContentEvent{Text: delta.Content},
					})
				}

				// Reasoning content (DeepSeek)
				if delta.ReasoningContent != "" {
					callback(StreamEvent{
						Type: "thinking",
						Data: ThinkingEvent{Text: delta.ReasoningContent},
					})
				}

				// Tool calls
				for _, tc := range delta.ToolCalls {
					builder, exists := toolCallBuilders[tc.Index]
					if !exists {
						builder = &struct {
							id   string
							name string
							args strings.Builder
						}{}
						toolCallBuilders[tc.Index] = builder
					}

					if tc.ID != "" {
						builder.id = tc.ID
					}
					if tc.Function.Name != "" {
						builder.name = tc.Function.Name
						callback(StreamEvent{
							Type: "tool_start",
							Data: ToolStartEvent{
								ID:   builder.id,
								Name: builder.name,
							},
						})
					}
					if tc.Function.Arguments != "" {
						builder.args.WriteString(tc.Function.Arguments)
					}
				}
			}
		}
	}

	return nil
}

// ListModels fetches available models from the OpenAI API
func (c *OpenAIClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	modelsURL := c.modelsEndpoint()
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := appendRateLimitInfo(string(body), resp)
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
	}

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Data))
	cache := GetNotableCache()
	for _, m := range result.Data {
		// Filter to only chat-capable models
		if strings.Contains(m.ID, "gpt") || strings.Contains(m.ID, "o1") ||
			strings.Contains(m.ID, "o3") || strings.Contains(m.ID, "o4") ||
			strings.Contains(m.ID, "deepseek") {
			// Use correct provider for notable detection
			provider := "openai"
			if strings.Contains(m.ID, "deepseek") {
				provider = "deepseek"
			}
			models = append(models, ModelInfo{
				ID:        m.ID,
				Name:      m.ID, // OpenAI uses ID as name
				CreatedAt: m.Created,
				Notable:   cache.IsNotable(provider, m.ID, m.Created),
			})
		}
	}

	return models, nil
}
