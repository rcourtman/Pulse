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
	anthropicAPIURL      = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion  = "2023-06-01"
	maxRetries           = 3
	initialBackoff       = 2 * time.Second
	defaultClientTimeout = 5 * time.Minute
)

// AnthropicClient implements the Provider interface for Anthropic's Claude API
type AnthropicClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewAnthropicClient creates a new Anthropic API client
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewAnthropicClient(apiKey, model string, timeout time.Duration) *AnthropicClient {
	return NewAnthropicClientWithBaseURL(apiKey, model, anthropicAPIURL, timeout)
}

// NewAnthropicClientWithBaseURL creates a new Anthropic client using a custom messages endpoint.
// This is useful for testing and for deployments that route requests through a proxy.
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewAnthropicClientWithBaseURL(apiKey, model, baseURL string, timeout time.Duration) *AnthropicClient {
	if baseURL == "" {
		baseURL = anthropicAPIURL
	}
	if timeout <= 0 {
		timeout = defaultClientTimeout
	}
	return &AnthropicClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the provider name
func (c *AnthropicClient) Name() string {
	return "anthropic"
}

// anthropicRequest is the request body for the Anthropic API
type anthropicRequest struct {
	Model       string               `json:"model"`
	Messages    []anthropicMessage   `json:"messages"`
	MaxTokens   int                  `json:"max_tokens"`
	System      string               `json:"system,omitempty"`
	Temperature float64              `json:"temperature,omitempty"`
	Tools       []anthropicTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
}

// anthropicToolChoice controls how Claude selects tools
// See: https://docs.anthropic.com/en/docs/build-with-claude/tool-use/implement-tool-use#forcing-tool-use
type anthropicToolChoice struct {
	Type string `json:"type"`           // "auto", "any", "tool", or "none"
	Name string `json:"name,omitempty"` // Only used when Type is "tool"
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []anthropicContent
}

// anthropicTool represents a regular function tool
type anthropicTool struct {
	Type         string                 `json:"type,omitempty"` // "web_search_20250305" for web search, omit for regular tools
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`   // Not used for web search
	InputSchema  map[string]interface{} `json:"input_schema,omitempty"`  // Not used for web search
	MaxUses      int                    `json:"max_uses,omitempty"`      // For web search: limit searches per request
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"` // Prompt caching breakpoint
}

// anthropicCacheControl marks a cache breakpoint for Anthropic prompt caching.
// Everything up to and including the tool with this marker is cached.
type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

// anthropicResponse is the response from the Anthropic API
type anthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage     `json:"usage"`
}

type anthropicContent struct {
	Type      string                 `json:"type"` // "text", "tool_use", "tool_result", "server_tool_use", "web_search_tool_result"
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`          // For tool_use
	Name      string                 `json:"name,omitempty"`        // For tool_use
	Input     map[string]interface{} `json:"input,omitempty"`       // For tool_use
	ToolUseID string                 `json:"tool_use_id,omitempty"` // For tool_result
	Content   json.RawMessage        `json:"content,omitempty"`     // Can be string or array (for web_search_tool_result)
	IsError   bool                   `json:"is_error,omitempty"`    // For tool_result
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type anthropicError struct {
	Type  string               `json:"type"`
	Error anthropicErrorDetail `json:"error"`
}

type anthropicErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Chat sends a chat request to the Anthropic API
func (c *AnthropicClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert messages to Anthropic format
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		// Anthropic doesn't use "system" role in messages array
		if m.Role == "system" {
			continue
		}

		// Handle tool results specially
		if m.ToolResult != nil {
			// Tool result message - Content needs to be JSON-encoded string
			contentJSON, err := json.Marshal(m.ToolResult.Content)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool result content for %s: %w", m.ToolResult.ToolUseID, err)
			}
			messages = append(messages, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{
					{
						Type:      "tool_result",
						ToolUseID: m.ToolResult.ToolUseID,
						Content:   contentJSON,
						IsError:   m.ToolResult.IsError,
					},
				},
			})
			continue
		}

		// Handle assistant messages with tool calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			contentBlocks := make([]anthropicContent, 0)
			if m.Content != "" {
				contentBlocks = append(contentBlocks, anthropicContent{
					Type: "text",
					Text: m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				contentBlocks = append(contentBlocks, anthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
			messages = append(messages, anthropicMessage{
				Role:    "assistant",
				Content: contentBlocks,
			})
			continue
		}

		// Simple text message
		messages = append(messages, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Use provided model or fall back to client default
	model := req.Model
	// Strip provider prefix if present - callers may pass the full "provider:model" string
	if len(model) > 10 && model[:10] == "anthropic:" {
		model = model[10:]
	}
	if model == "" {
		model = c.model
	}

	// Set max tokens (Anthropic requires this)
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	anthropicReq := anthropicRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		System:    req.System,
	}

	if req.Temperature > 0 {
		anthropicReq.Temperature = req.Temperature
	}

	// Add tools if provided
	shouldAddTools := len(req.Tools) > 0
	if req.ToolChoice != nil && req.ToolChoice.Type == ToolChoiceNone {
		shouldAddTools = false
	}
	if shouldAddTools {
		anthropicReq.Tools = make([]anthropicTool, len(req.Tools))
		for i, t := range req.Tools {
			if t.Type == "web_search_20250305" {
				anthropicReq.Tools[i] = anthropicTool{
					Type:    t.Type,
					Name:    t.Name,
					MaxUses: t.MaxUses,
				}
			} else {
				anthropicReq.Tools[i] = anthropicTool{
					Name:        t.Name,
					Description: t.Description,
					InputSchema: t.InputSchema,
				}
			}
		}
		// Mark the last tool with cache_control so Anthropic caches all tool
		// definitions (and everything before them) on subsequent turns.
		anthropicReq.Tools[len(anthropicReq.Tools)-1].CacheControl = &anthropicCacheControl{Type: "ephemeral"}
	}

	// Add tool_choice if specified
	// This controls whether Claude MUST use tools vs just being able to
	// See: https://docs.anthropic.com/en/docs/build-with-claude/tool-use/implement-tool-use#forcing-tool-use
	// Anthropic may reject tool_choice if tools are not provided.
	if shouldAddTools && req.ToolChoice != nil {
		anthropicReq.ToolChoice = &anthropicToolChoice{
			Type: string(req.ToolChoice.Type),
			Name: req.ToolChoice.Name,
		}
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop for transient errors (429, 529, 5xx)
	var respBody []byte
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			backoff := initialBackoff * time.Duration(1<<(attempt-1))
			log.Warn().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Str("last_error", lastErr.Error()).
				Msg("Retrying Anthropic API request after transient error")

			backoffTimer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				if !backoffTimer.Stop() {
					select {
					case <-backoffTimer.C:
					default:
					}
				}
				return nil, ctx.Err()
			case <-backoffTimer.C:
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", c.apiKey)
		httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// Check if this is a retryable error
		if resp.StatusCode == 429 || resp.StatusCode == 529 || resp.StatusCode >= 500 {
			var errResp anthropicError
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
			var errResp anthropicError
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
		return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract content and tool calls from response
	var textContent string
	var toolCalls []ToolCall
	for _, c := range anthropicResp.Content {
		switch c.Type {
		case "text":
			textContent += c.Text
		case "tool_use":
			// Regular tool use - we need to execute these
			toolCalls = append(toolCalls, ToolCall{
				ID:    c.ID,
				Name:  c.Name,
				Input: c.Input,
			})
		case "server_tool_use":
			// Server-side tool (like web_search) - Anthropic handles these automatically
			// We just log it for debugging, no action needed
			log.Debug().
				Str("tool_name", c.Name).
				Msg("Server tool use detected (handled by Anthropic)")
		case "web_search_tool_result":
			// Results from web search - already incorporated into Claude's response
			log.Debug().Msg("web search results received")
		}
	}

	// Log content summary and cache stats for debugging
	logEvent := log.Debug().
		Int("content_blocks", len(anthropicResp.Content)).
		Int("text_length", len(textContent)).
		Int("tool_calls", len(toolCalls)).
		Str("stop_reason", anthropicResp.StopReason)
	if anthropicResp.Usage.CacheCreationInputTokens > 0 || anthropicResp.Usage.CacheReadInputTokens > 0 {
		logEvent = logEvent.
			Int("cache_creation_tokens", anthropicResp.Usage.CacheCreationInputTokens).
			Int("cache_read_tokens", anthropicResp.Usage.CacheReadInputTokens)
	}
	logEvent.Msg("anthropic response parsed")

	return &ChatResponse{
		Content:      textContent,
		Model:        anthropicResp.Model,
		StopReason:   anthropicResp.StopReason,
		ToolCalls:    toolCalls,
		InputTokens:  anthropicResp.Usage.InputTokens,
		OutputTokens: anthropicResp.Usage.OutputTokens,
	}, nil
}

// TestConnection validates the API key by listing models
// This avoids dependencies on specific model names which may get deprecated
func (c *AnthropicClient) TestConnection(ctx context.Context) error {
	if _, err := c.ListModels(ctx); err != nil {
		return fmt.Errorf("anthropic test connection failed: %w", err)
	}
	return nil
}

func (c *AnthropicClient) modelsEndpoint() string {
	defaultURL := "https://api.anthropic.com/v1/models"
	u, err := url.Parse(c.baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return defaultURL
	}
	return u.Scheme + "://" + u.Host + "/v1/models"
}

// ListModels fetches available models from the Anthropic API
func (c *AnthropicClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.modelsEndpoint(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

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
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			CreatedAt   string `json:"created_at"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Data))
	cache := GetNotableCache()
	notableCount := 0
	for _, m := range result.Data {
		createdAt := parseModelCreatedAt(m.CreatedAt)
		isNotable := cache.IsNotable("anthropic", m.ID, createdAt)
		if isNotable {
			notableCount++
		}
		models = append(models, ModelInfo{
			ID:        m.ID,
			Name:      m.DisplayName,
			CreatedAt: createdAt,
			Notable:   isNotable,
		})
	}

	log.Info().Int("total", len(models)).Int("notable", notableCount).Msg("anthropic ListModels returned")

	// Log model IDs for debugging
	var modelIDs []string
	for _, m := range models {
		modelIDs = append(modelIDs, m.ID)
	}
	log.Debug().Strs("models", modelIDs).Msg("anthropic models")

	return models, nil
}

func parseModelCreatedAt(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.Unix()
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Unix()
	}
	return 0
}

// SupportsThinking returns true if the model supports extended thinking
func (c *AnthropicClient) SupportsThinking(model string) bool {
	// Claude 3.5+ models with "thinking" capability
	// Currently no public Claude models have extended thinking like DeepSeek
	return false
}

// anthropicStreamRequest is the request body for streaming API calls
type anthropicStreamRequest struct {
	Model       string               `json:"model"`
	Messages    []anthropicMessage   `json:"messages"`
	MaxTokens   int                  `json:"max_tokens"`
	System      string               `json:"system,omitempty"`
	Temperature float64              `json:"temperature,omitempty"`
	Tools       []anthropicTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
	Stream      bool                 `json:"stream"`
}

// anthropicStreamEvent represents a streaming event from the Anthropic API
type anthropicStreamEvent struct {
	Type         string             `json:"type"`
	Index        int                `json:"index,omitempty"`
	ContentBlock *anthropicContent  `json:"content_block,omitempty"`
	Delta        *anthropicDelta    `json:"delta,omitempty"`
	Message      *anthropicResponse `json:"message,omitempty"`
	Usage        *anthropicUsage    `json:"usage,omitempty"`
}

type anthropicDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// ChatStream sends a chat request and streams the response via callback
func (c *AnthropicClient) ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	// Convert messages to Anthropic format (same as Chat)
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue
		}

		if m.ToolResult != nil {
			contentJSON, err := json.Marshal(m.ToolResult.Content)
			if err != nil {
				return fmt.Errorf("failed to marshal tool result content for %s: %w", m.ToolResult.ToolUseID, err)
			}
			messages = append(messages, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{
					{
						Type:      "tool_result",
						ToolUseID: m.ToolResult.ToolUseID,
						Content:   contentJSON,
						IsError:   m.ToolResult.IsError,
					},
				},
			})
			continue
		}

		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			contentBlocks := make([]anthropicContent, 0)
			if m.Content != "" {
				contentBlocks = append(contentBlocks, anthropicContent{
					Type: "text",
					Text: m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				contentBlocks = append(contentBlocks, anthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
			messages = append(messages, anthropicMessage{
				Role:    "assistant",
				Content: contentBlocks,
			})
			continue
		}

		messages = append(messages, anthropicMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	model := req.Model
	if len(model) > 10 && model[:10] == "anthropic:" {
		model = model[10:]
	}
	if model == "" {
		model = c.model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	anthropicReq := anthropicStreamRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		System:    req.System,
		Stream:    true,
	}

	if req.Temperature > 0 {
		anthropicReq.Temperature = req.Temperature
	}

	shouldAddTools := len(req.Tools) > 0
	if req.ToolChoice != nil && req.ToolChoice.Type == ToolChoiceNone {
		shouldAddTools = false
	}
	if shouldAddTools {
		anthropicReq.Tools = make([]anthropicTool, len(req.Tools))
		for i, t := range req.Tools {
			if t.Type == "web_search_20250305" {
				anthropicReq.Tools[i] = anthropicTool{
					Type:    t.Type,
					Name:    t.Name,
					MaxUses: t.MaxUses,
				}
			} else {
				anthropicReq.Tools[i] = anthropicTool{
					Name:        t.Name,
					Description: t.Description,
					InputSchema: t.InputSchema,
				}
			}
		}
		// Mark the last tool with cache_control for prompt caching (same as non-streaming).
		anthropicReq.Tools[len(anthropicReq.Tools)-1].CacheControl = &anthropicCacheControl{Type: "ephemeral"}
	}

	// Add tool_choice if specified (same as non-streaming)
	// Anthropic may reject tool_choice if tools are not provided.
	if shouldAddTools && req.ToolChoice != nil {
		anthropicReq.ToolChoice = &anthropicToolChoice{
			Type: string(req.ToolChoice.Type),
			Name: req.ToolChoice.Name,
		}
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		var errResp anthropicError
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
	var currentToolID string
	var currentToolName string
	var currentToolInput strings.Builder
	var inputTokens, outputTokens int

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			pendingData += string(buf[:n])
			lines := strings.Split(pendingData, "\n")
			pendingData = lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			for _, line := range lines {
				line = strings.TrimSpace(line)

				if strings.HasPrefix(line, "event:") {
					continue
				}

				if !strings.HasPrefix(line, "data:") {
					continue
				}

				eventData := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if eventData == "" {
					continue
				}

				var event anthropicStreamEvent
				if err := json.Unmarshal([]byte(eventData), &event); err != nil {
					log.Debug().Err(err).Str("data", eventData).Msg("failed to parse stream event")
					continue
				}

				switch event.Type {
				case "message_start":
					if event.Message != nil && event.Message.Usage.InputTokens > 0 {
						inputTokens = event.Message.Usage.InputTokens
					}

				case "content_block_start":
					if event.ContentBlock != nil {
						switch event.ContentBlock.Type {
						case "tool_use":
							currentToolID = event.ContentBlock.ID
							currentToolName = event.ContentBlock.Name
							currentToolInput.Reset()
							callback(StreamEvent{
								Type: "tool_start",
								Data: ToolStartEvent{
									ID:   currentToolID,
									Name: currentToolName,
								},
							})
						}
					}

				case "content_block_delta":
					if event.Delta != nil {
						switch event.Delta.Type {
						case "text_delta":
							if event.Delta.Text != "" {
								callback(StreamEvent{
									Type: "content",
									Data: ContentEvent{Text: event.Delta.Text},
								})
							}
						case "input_json_delta":
							currentToolInput.WriteString(event.Delta.PartialJSON)
						}
					}

				case "content_block_stop":
					if currentToolID != "" {
						// Parse the accumulated tool input
						var input map[string]interface{}
						if err := json.Unmarshal([]byte(currentToolInput.String()), &input); err != nil {
							input = map[string]interface{}{"raw": currentToolInput.String()}
						}
						toolCalls = append(toolCalls, ToolCall{
							ID:    currentToolID,
							Name:  currentToolName,
							Input: input,
						})
						currentToolID = ""
						currentToolName = ""
					}

				case "message_delta":
					if event.Delta != nil && event.Delta.StopReason != "" {
						if event.Usage != nil {
							outputTokens = event.Usage.OutputTokens
						}
					}

				case "message_stop":
					stopReason := "end_turn"
					if len(toolCalls) > 0 {
						stopReason = "tool_use"
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

				case "error":
					callback(StreamEvent{
						Type: "error",
						Data: ErrorEvent{Message: "Stream error from Anthropic"},
					})
					return fmt.Errorf("stream error from Anthropic")
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("stream read error: %w", err)
		}
	}

	return nil
}
