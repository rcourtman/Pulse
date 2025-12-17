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
func NewOpenAIClient(apiKey, model, baseURL string) *OpenAIClient {
	if baseURL == "" {
		baseURL = openaiAPIURL
	}
	// Strip provider prefix if present - the model should be just the model name
	if strings.HasPrefix(model, "openai:") {
		model = strings.TrimPrefix(model, "openai:")
	} else if strings.HasPrefix(model, "deepseek:") {
		model = strings.TrimPrefix(model, "deepseek:")
	}
	return &OpenAIClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			// 5 minutes timeout - DeepSeek reasoning models can take a long time
			Timeout: 300 * time.Second,
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

// deepseekRequest extends openaiRequest with DeepSeek-specific fields
type deepseekRequest struct {
	Model      string          `json:"model"`
	Messages   []openaiMessage `json:"messages"`
	MaxTokens  int             `json:"max_tokens,omitempty"`
	Tools      []openaiTool    `json:"tools,omitempty"`
	ToolChoice interface{}     `json:"tool_choice,omitempty"`
}

// openaiCompletionsRequest is for non-chat models like gpt-5.2-pro that use /v1/completions
type openaiCompletionsRequest struct {
	Model               string  `json:"model"`
	Prompt              string  `json:"prompt"`
	MaxCompletionTokens int     `json:"max_completion_tokens,omitempty"`
	Temperature         float64 `json:"temperature,omitempty"`
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
	Message      openaiRespMsg `json:"message"`       // For chat completions
	Text         string        `json:"text"`          // For completions API (non-chat models)
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
func (c *OpenAIClient) requiresMaxCompletionTokens(model string) bool {
	// o1, o1-mini, o1-preview, o3, o3-mini, o4-mini, gpt-5.2, etc.
	return strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4") || strings.HasPrefix(model, "gpt-5")
}

// isGPT52NonChat returns true if using GPT-5.2 models that require /v1/completions endpoint
// Only gpt-5.2-chat-latest uses chat completions; gpt-5.2, gpt-5.2-pro use completions
func (c *OpenAIClient) isGPT52NonChat(model string) bool {
	if !strings.HasPrefix(model, "gpt-5.2") {
		return false
	}
	// gpt-5.2-chat-latest is the only chat model
	return !strings.Contains(model, "chat")
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
		if len(openaiReq.Tools) > 0 {
			openaiReq.ToolChoice = "auto"
		}
	}

	// Log actual model being sent (INFO level for visibility)
	log.Info().Str("model_in_request", openaiReq.Model).Str("base_url", c.baseURL).Msg("Sending OpenAI/DeepSeek request")

	var body []byte
	var err error

	// GPT-5.2 non-chat models need completions format (prompt instead of messages)
	if c.isGPT52NonChat(model) {
		// Convert messages to a single prompt string
		var promptBuilder strings.Builder
		if req.System != "" {
			promptBuilder.WriteString("System: ")
			promptBuilder.WriteString(req.System)
			promptBuilder.WriteString("\n\n")
		}
		for _, m := range req.Messages {
			promptBuilder.WriteString(m.Role)
			promptBuilder.WriteString(": ")
			promptBuilder.WriteString(m.Content)
			promptBuilder.WriteString("\n\n")
		}
		promptBuilder.WriteString("Assistant: ")

		completionsReq := openaiCompletionsRequest{
			Model:               model,
			Prompt:              promptBuilder.String(),
			MaxCompletionTokens: req.MaxTokens,
		}
		body, err = json.Marshal(completionsReq)
	} else {
		body, err = json.Marshal(openaiReq)
	}
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

		// Use the appropriate endpoint
		endpoint := c.baseURL
		if c.isGPT52NonChat(model) && strings.Contains(c.baseURL, "api.openai.com") {
			// GPT-5.2 non-chat models need completions endpoint
			endpoint = strings.Replace(c.baseURL, "/chat/completions", "/completions", 1)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
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
			lastErr = fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
			continue
		}

		// Non-retryable error
		if resp.StatusCode != http.StatusOK {
			var errResp openaiError
			if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
				return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
			}
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
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
	// Completions API uses Text instead of Message.Content
	if contentToUse == "" && choice.Text != "" {
		contentToUse = choice.Text
	}
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

	base := u.Scheme + "://" + u.Host
	if c.isDeepSeek() {
		return base + "/models"
	}
	return base + "/v1/models"
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
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
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
	for _, m := range result.Data {
		// Filter to only chat-capable models
		if strings.Contains(m.ID, "gpt") || strings.Contains(m.ID, "o1") ||
			strings.Contains(m.ID, "o3") || strings.Contains(m.ID, "deepseek") {
			models = append(models, ModelInfo{
				ID:        m.ID,
				Name:      m.ID, // OpenAI uses ID as name
				CreatedAt: m.Created,
			})
		}
	}

	return models, nil
}
