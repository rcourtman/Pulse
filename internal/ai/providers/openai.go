package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog/log"
)

const (
	openaiAPIURL               = "https://api.openai.com/v1/chat/completions"
	openaiMaxRetries           = 3
	openaiInitialBackoff       = 2 * time.Second
	openaiStreamMaxRetries     = 1
	openaiStreamInitialBackoff = 1 * time.Second
	openaiStreamChunkTimeout   = 12 * time.Second
	openrouterRefererURL       = "https://pulse.app"
	openrouterAppTitle         = "Pulse"
	// OpenRouter preflights affordability against the requested maximum
	// completion budget. Leaving it unset can make small chat turns reserve a
	// model-scale default and fail against per-key total limits. Keep this high
	// enough for normal detailed Assistant answers; 1024 cuts off ordinary
	// inventory breakdowns mid-sentence.
	openrouterDefaultMaxCompletionTokens = 4096
)

// OpenAIClient implements the Provider interface for OpenAI's API
// Also works with OpenAI-compatible APIs like DeepSeek
type OpenAIClient struct {
	providerName string
	apiKey       string
	model        string
	baseURL      string
	client       *http.Client
	streamClient *http.Client
	// The configured request timeout bounds how long Pulse waits for the
	// stream to START (response headers and first chunk). Once deltas flow
	// there is deliberately no overall wall-clock deadline: reasoning models
	// (qwen3 via Ollama, DeepSeek) stream thinking deltas for minutes before
	// real content, and a live stream must never be killed mid-thought
	// (#1576). Only the stall bounds below end a flowing stream.
	//
	// streamChunkTimeout bounds the gap BETWEEN chunks once the stream is
	// flowing, so chat fallback can move before the drawer looks dead.
	streamChunkTimeout time.Duration
	// Local OpenAI-compatible backends (LM Studio, llama.cpp, vLLM on CPU) can
	// legitimately spend minutes on prompt processing before the first SSE
	// chunk, so the wait for first bytes honors the configured request timeout
	// instead of the inter-chunk gap bound (issue discussion #1571).
	streamFirstChunkTimeout time.Duration
}

var openAICompatibleOutboundHTTPOptions = securityutil.RestrictedOutboundHTTPOptions{
	AllowedSchemes:  []string{"http", "https"},
	AllowPrivateIPs: true,
	AllowLoopback:   true,
}

// NewOpenAIClient creates a new OpenAI API client
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewOpenAIClient(apiKey, model, baseURL string, timeout time.Duration) *OpenAIClient {
	return NewOpenAICompatibleClient("openai", apiKey, model, baseURL, timeout)
}

// NewOpenAICompatibleClient creates a client for OpenAI-compatible chat APIs.
func NewOpenAICompatibleClient(providerName, apiKey, model, baseURL string, timeout time.Duration) *OpenAIClient {
	providerName = strings.ToLower(strings.TrimSpace(providerName))
	if providerName == "" {
		providerName = "openai"
	}
	if baseURL == "" {
		baseURL = openaiAPIURL
	} else {
		baseURL = normalizeOpenAICompatibleChatURL(baseURL)
	}
	model = stripOpenAICompatibleProviderPrefix(providerName, model)
	if timeout <= 0 {
		timeout = 300 * time.Second // Default 5 minutes
	}
	return &OpenAIClient{
		providerName:            providerName,
		apiKey:                  apiKey,
		model:                   model,
		baseURL:                 baseURL,
		client:                  newOpenAICompatibleHTTPClient(timeout, false),
		streamClient:            newOpenAIStreamHTTPClient(timeout),
		streamChunkTimeout:      boundedOpenAIStreamChunkTimeout(timeout),
		streamFirstChunkTimeout: timeout,
	}
}

func newOpenAICompatibleHTTPClient(timeout time.Duration, streaming bool) *http.Client {
	options := openAICompatibleOutboundHTTPOptions
	clientTimeout := timeout
	if streaming {
		clientTimeout = 0
		options.ResponseHeaderTimeout = timeout
	}
	return securityutil.NewRestrictedOutboundHTTPClient(clientTimeout, options)
}

func normalizeOpenAICompatibleChatURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return openaiAPIURL
	}
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	if strings.HasSuffix(trimmed, "/completions") {
		return strings.TrimSuffix(trimmed, "/completions") + "/chat/completions"
	}

	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if strings.Contains(trimmed, "/") {
			return trimmed + "/chat/completions"
		}
		return trimmed + "/v1/chat/completions"
	}

	path := strings.TrimRight(u.Path, "/")
	if path == "" {
		u.Path = "/v1/chat/completions"
	} else {
		u.Path = path + "/chat/completions"
	}
	return u.String()
}

func stripOpenAICompatibleProviderPrefix(providerName, model string) string {
	model = strings.TrimSpace(model)
	prefix := strings.ToLower(strings.TrimSpace(providerName)) + ":"
	if prefix != ":" && strings.HasPrefix(strings.ToLower(model), prefix) {
		return model[len(prefix):]
	}
	for _, knownPrefix := range []string{
		"openai:",
		"openrouter:",
		"deepseek:",
		"zai:",
		"groq:",
		"mistral:",
		"cerebras:",
		"together:",
		"fireworks:",
	} {
		if strings.HasPrefix(strings.ToLower(model), knownPrefix) {
			return model[len(knownPrefix):]
		}
	}
	return model
}

func newOpenAIStreamHTTPClient(timeout time.Duration) *http.Client {
	// Local backends can hold response headers while the model loads. Bound
	// that startup wait, but do not impose an overall timeout on a live stream.
	return newOpenAICompatibleHTTPClient(timeout, true)
}

func boundedOpenAIStreamChunkTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 && timeout < openaiStreamChunkTimeout {
		return timeout
	}
	return openaiStreamChunkTimeout
}

func streamChunkTimeoutForRequest(base, requestLimit, requested time.Duration) time.Duration {
	if requested <= base {
		return base
	}
	if requestLimit > 0 && requested > requestLimit {
		return requestLimit
	}
	return requested
}

type openAIStreamReadResult struct {
	n   int
	err error
}

func readOpenAIStreamChunk(ctx context.Context, body io.ReadCloser, buf []byte, timeout time.Duration) (int, error) {
	if timeout <= 0 {
		return body.Read(buf)
	}

	resultCh := make(chan openAIStreamReadResult, 1)
	go func() {
		n, err := body.Read(buf)
		resultCh <- openAIStreamReadResult{n: n, err: err}
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case result := <-resultCh:
		return result.n, result.err
	case <-ctx.Done():
		_ = body.Close()
		return 0, ctx.Err()
	case <-timer.C:
		_ = body.Close()
		return 0, fmt.Errorf("stream chunk timed out after %s", timeout)
	}
}

func isRetryableOpenAIStreamStartupError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "eof")
}

func isRetryableOpenAIStreamStatus(statusCode int) bool {
	return statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusGatewayTimeout
}

// Name returns the provider name
func (c *OpenAIClient) Name() string {
	if strings.TrimSpace(c.providerName) != "" {
		return c.providerName
	}
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
	ToolChoice          interface{}     `json:"tool_choice,omitempty"` // "none", "required", or provider-specific choices
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
	ReasoningContent string           `json:"reasoning_content,omitempty"` // DeepSeek direct thinking mode
	Reasoning        string           `json:"reasoning,omitempty"`         // OpenRouter / OpenAI-compatible gateways
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

func convertOpenAIResponse(openaiResp openaiResponse) (*ChatResponse, error) {
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	choice := openaiResp.Choices[0]
	reasoning := choice.Message.ReasoningContent
	if reasoning == "" {
		reasoning = choice.Message.Reasoning
	}
	result := &ChatResponse{
		Content:          choice.Message.Content,
		ReasoningContent: reasoning,
		Model:            openaiResp.Model,
		StopReason:       choice.FinishReason,
		InputTokens:      openaiResp.Usage.PromptTokens,
		OutputTokens:     openaiResp.Usage.CompletionTokens,
	}
	if len(choice.Message.ToolCalls) > 0 {
		result.StopReason = "tool_use"
		for _, tc := range choice.Message.ToolCalls {
			if strings.TrimSpace(tc.ID) == "" || strings.TrimSpace(tc.Function.Name) == "" {
				return nil, fmt.Errorf("tool call is missing an id or function name")
			}
			input, ok := agentcapabilities.ParseProviderToolInput(tc.Function.Arguments)
			if !ok {
				return nil, fmt.Errorf("tool call %q returned invalid arguments", tc.Function.Name)
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

func emitBufferedOpenAIResponse(callback StreamCallback, response *ChatResponse) error {
	if response == nil {
		return fmt.Errorf("empty non-streaming response")
	}
	if response.ReasoningContent != "" {
		callback(StreamEvent{Type: "thinking", Data: ThinkingEvent{Text: response.ReasoningContent}})
	}
	if response.Content != "" {
		callback(StreamEvent{Type: "content", Data: ContentEvent{Text: response.Content}})
	}
	for _, call := range response.ToolCalls {
		callback(StreamEvent{
			Type: "tool_start",
			Data: ToolStartEvent{ID: call.ID, Name: call.Name, Input: call.Input}.NormalizeCollections(),
		})
	}
	callback(StreamEvent{
		Type: "done",
		Data: DoneEvent{
			StopReason:   normalizeOpenAIStreamStopReason(response.StopReason, response.ToolCalls),
			ToolCalls:    response.ToolCalls,
			InputTokens:  response.InputTokens,
			OutputTokens: response.OutputTokens,
		},
	})
	return nil
}

func openAIStreamingExplicitlyUnsupported(statusCode int, message string) bool {
	if statusCode != http.StatusBadRequest && statusCode != http.StatusUnprocessableEntity && statusCode != http.StatusNotImplemented {
		return false
	}
	lower := strings.ToLower(message)
	return strings.Contains(lower, "streaming is not supported") ||
		strings.Contains(lower, "stream is not supported") ||
		strings.Contains(lower, "streaming unsupported") ||
		strings.Contains(lower, "stream must be false") ||
		strings.Contains(lower, "unsupported stream")
}

// isDeepSeek returns true if this client is configured for DeepSeek
func (c *OpenAIClient) isDeepSeek() bool {
	return c.Name() == "deepseek" || strings.Contains(c.baseURL, "deepseek.com")
}

func (c *OpenAIClient) isOpenRouter() bool {
	return c.Name() == "openrouter" || strings.Contains(c.baseURL, "openrouter.ai")
}

func (c *OpenAIClient) usesOfficialOpenAIEndpoint() bool {
	if c.Name() != "openai" {
		return false
	}
	u, err := url.Parse(c.baseURL)
	if err != nil || u.Host == "" {
		return true
	}
	return strings.EqualFold(u.Hostname(), "api.openai.com")
}

// isDeepSeekReasoner returns true if using DeepSeek's reasoning model
func (c *OpenAIClient) isDeepSeekReasoner() bool {
	return c.isDeepSeek() && strings.Contains(c.model, "reasoner")
}

func (c *OpenAIClient) shouldSendReasoningContent() bool {
	return c.isDeepSeek()
}

func (c *OpenAIClient) applyProviderHeaders(req *http.Request) {
	if !c.isOpenRouter() {
		return
	}
	req.Header.Set("HTTP-Referer", openrouterRefererURL)
	req.Header.Set("X-Title", openrouterAppTitle)
}

func (c *OpenAIClient) applyAuthorization(req *http.Request) {
	if strings.TrimSpace(c.apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func (c *OpenAIClient) requestMaxTokens(req ChatRequest) int {
	if req.MaxTokens > 0 {
		return req.MaxTokens
	}
	if c.isOpenRouter() {
		return openrouterDefaultMaxCompletionTokens
	}
	return 0
}

// requiresMaxCompletionTokens returns true for models that need max_completion_tokens instead of max_tokens
// Per OpenAI docs, o1/o3/o4 reasoning models require max_completion_tokens; max_tokens will error.
func (c *OpenAIClient) requiresMaxCompletionTokens(model string) bool {
	if c.isOpenRouter() {
		return true
	}
	if !c.usesOfficialOpenAIEndpoint() {
		return false
	}
	return strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4")
}

func (c *OpenAIClient) supportsStreamOptions() bool {
	// stream_options is an OpenAI extension, not part of the minimum compatible
	// protocol implemented by llama.cpp, LocalAI, and some LM Studio releases.
	return c.Name() != "openai" || c.usesOfficialOpenAIEndpoint()
}

// convertToolChoiceToOpenAI converts our ToolChoice to OpenAI's format.
// Pulse omits automatic tool_choice so tool use stays model-owned, and only
// serializes native override modes when the caller explicitly requests them.
// See: https://platform.openai.com/docs/api-reference/chat/create#chat-create-tool_choice
func convertToolChoiceToOpenAI(tc *ToolChoice) interface{} {
	if tc == nil {
		return nil
	}
	switch tc.Type {
	case ToolChoiceNone:
		return "none"
	case ToolChoiceRequired:
		return "required"
	default:
		return nil
	}
}

// convertMessagesToOpenAI converts provider-neutral messages into the
// OpenAI-compatible chat message shape used by both non-streaming and streaming
// requests.
func convertMessagesToOpenAI(req ChatRequest, includeReasoningContent bool) ([]openaiMessage, error) {
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
			if includeReasoningContent && m.ReasoningContent != "" {
				msg.ReasoningContent = m.ReasoningContent
			}
			for _, tc := range m.ToolCalls {
				argsJSON, err := json.Marshal(tc.Input)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool call input for %s: %w", tc.Name, err)
				}
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
			if includeReasoningContent && m.ReasoningContent != "" {
				msg.ReasoningContent = m.ReasoningContent
			}
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// Chat sends a chat request to the OpenAI API
func (c *OpenAIClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert messages to OpenAI format
	messages, err := convertMessagesToOpenAI(req, c.shouldSendReasoningContent())
	if err != nil {
		return nil, err
	}

	// Use provided model or fall back to client default
	model := req.Model
	model = stripOpenAICompatibleProviderPrefix(c.Name(), model)
	if model == "" {
		model = c.model
	}

	// Debug log to trace model issues
	log.Debug().Str("provider", c.Name()).Str("model", model).Str("req_model", req.Model).Str("c_model", c.model).Str("base_url", c.baseURL).Msg("OpenAI-compatible chat request")

	// Build request
	openaiReq := openaiRequest{
		Model:    model,
		Messages: messages,
	}

	// max_completion_tokens is required by official OpenAI reasoning models.
	// The portable OpenAI-compatible field is max_tokens.
	if maxTokens := c.requestMaxTokens(req); maxTokens > 0 {
		if c.requiresMaxCompletionTokens(model) {
			openaiReq.MaxCompletionTokens = maxTokens
		} else {
			openaiReq.MaxTokens = maxTokens
		}
	}

	// DeepSeek reasoner and newer OpenAI models don't support temperature
	if req.Temperature > 0 && !c.isDeepSeekReasoner() && !c.requiresMaxCompletionTokens(model) {
		openaiReq.Temperature = req.Temperature
	}

	// Convert tools to OpenAI format
	shouldAddTools := len(req.Tools) > 0
	if req.ToolChoice != nil && req.ToolChoice.Type == ToolChoiceNone {
		shouldAddTools = false
	}
	if shouldAddTools {
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
			if toolChoice := convertToolChoiceToOpenAI(req.ToolChoice); toolChoice != nil {
				openaiReq.ToolChoice = toolChoice
			}
		}
	}

	// Log actual model being sent (INFO level for visibility)
	log.Info().Str("provider", c.Name()).Str("model_in_request", openaiReq.Model).Str("base_url", c.baseURL).Msg("sending OpenAI-compatible request")

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop for transient errors (connection resets, 429, 5xx)
	var respBody []byte
	var lastErr error
	maxRetries := openaiMaxRetries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			backoff := openaiInitialBackoff * time.Duration(1<<(attempt-1))
			log.Warn().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Str("last_error", lastErr.Error()).
				Msg("Retrying OpenAI/DeepSeek API request after transient error")

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
		c.applyAuthorization(httpReq)
		c.applyProviderHeaders(httpReq)

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
		return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return convertOpenAIResponse(openaiResp)
}

// TestConnection validates the API key by listing models
func (c *OpenAIClient) TestConnection(ctx context.Context) error {
	if c.isOpenRouter() {
		if err := c.testOpenRouterKey(ctx); err != nil {
			return fmt.Errorf("openrouter test connection failed: %w", err)
		}
		return nil
	}
	if _, err := c.ListModels(ctx); err != nil {
		return fmt.Errorf("openai test connection failed: %w", err)
	}
	return nil
}

func (c *OpenAIClient) testOpenRouterKey(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.openrouterKeyEndpoint(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.applyAuthorization(req)
	req.Header.Set("Accept", "application/json")
	c.applyProviderHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		var errResp openaiError
		errMsg := string(respBody)
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			errMsg = errResp.Error.Message
		}
		errMsg = appendRateLimitInfo(errMsg, resp)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
	}
	return nil
}

func (c *OpenAIClient) openrouterKeyEndpoint() string {
	u, err := url.Parse(c.baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "https://openrouter.ai/api/v1/key"
	}

	path := u.Path
	if strings.HasSuffix(path, "/chat/completions") {
		path = strings.TrimSuffix(path, "/chat/completions") + "/key"
	} else if strings.HasSuffix(path, "/v1") {
		path = path + "/key"
	} else {
		path = "/api/v1/key"
	}
	return u.Scheme + "://" + u.Host + path
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
	// DeepSeek reasoning-backed models expose reasoning_content.
	if c.isDeepSeek() && (strings.Contains(model, "reasoner") || strings.HasPrefix(model, "deepseek-v4-")) {
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
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
	// ReasoningContent carries reasoning tokens on DeepSeek's direct API.
	ReasoningContent string `json:"reasoning_content,omitempty"`
	// Reasoning carries reasoning tokens on OpenRouter and other OpenAI-compatible
	// gateways, which normalize chain-of-thought into "reasoning" rather than
	// DeepSeek's "reasoning_content". Without this, reasoning models routed via
	// OpenRouter stream their thinking into a field Pulse never read, so the user
	// saw a long dead pause instead of a live thinking stream.
	Reasoning string                `json:"reasoning,omitempty"`
	ToolCalls []openaiToolCallDelta `json:"tool_calls,omitempty"`
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

type openaiStreamToolCallBuilder struct {
	id   string
	name string
	args strings.Builder
}

func finalizeOpenAIStreamToolCalls(builders map[int]*openaiStreamToolCallBuilder) []ToolCall {
	if len(builders) == 0 {
		return nil
	}

	indexes := make([]int, 0, len(builders))
	for index := range builders {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	toolCalls := make([]ToolCall, 0, len(indexes))
	for _, index := range indexes {
		builder := builders[index]
		if builder == nil {
			continue
		}

		input := agentcapabilities.ProviderToolInputOrRaw(builder.args.String())
		toolCalls = append(toolCalls, ToolCall{
			ID:    builder.id,
			Name:  builder.name,
			Input: input,
		})
	}

	return toolCalls
}

func normalizeOpenAIStreamStopReason(finishReason string, toolCalls []ToolCall) string {
	stopReason := finishReason
	if len(toolCalls) > 0 {
		return "tool_use"
	}
	if stopReason == "" || stopReason == "stop" {
		return "end_turn"
	}
	return stopReason
}

// ChatStream sends a chat request and streams the response via callback
func (c *OpenAIClient) ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	// Convert messages to OpenAI format (same as Chat)
	messages, err := convertMessagesToOpenAI(req, c.shouldSendReasoningContent())
	if err != nil {
		return err
	}

	model := req.Model
	model = stripOpenAICompatibleProviderPrefix(c.Name(), model)
	if model == "" {
		model = c.model
	}

	openaiReq := openaiStreamRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}
	if c.supportsStreamOptions() {
		openaiReq.StreamOptions = &streamOptions{IncludeUsage: true}
	}

	if maxTokens := c.requestMaxTokens(req); maxTokens > 0 {
		if c.requiresMaxCompletionTokens(model) {
			openaiReq.MaxCompletionTokens = maxTokens
		} else {
			openaiReq.MaxTokens = maxTokens
		}
	}

	if req.Temperature > 0 && !c.isDeepSeekReasoner() && !c.requiresMaxCompletionTokens(model) {
		openaiReq.Temperature = req.Temperature
	}

	shouldAddTools := len(req.Tools) > 0
	if req.ToolChoice != nil && req.ToolChoice.Type == ToolChoiceNone {
		shouldAddTools = false
	}
	if shouldAddTools {
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
		if len(openaiReq.Tools) > 0 {
			if toolChoice := convertToolChoiceToOpenAI(req.ToolChoice); toolChoice != nil {
				openaiReq.ToolChoice = toolChoice
			}
		}
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// No overall wall-clock deadline on the stream: header and first-chunk
	// waits honor the configured request timeout, and once deltas flow the
	// inter-chunk stall bound plus caller cancellation are the only limits.
	// Reasoning models legitimately stream thinking deltas for longer than
	// any fixed turn budget (#1576).
	var resp *http.Response
	var lastErr error
	streamClient := c.streamClient
	if streamClient == nil {
		streamClient = c.client
	}
	maxRetries := openaiStreamMaxRetries

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := openaiStreamInitialBackoff * time.Duration(1<<(attempt-1))
			log.Warn().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Str("last_error", lastErr.Error()).
				Msg("Retrying OpenAI/DeepSeek stream startup after transient error")

			backoffTimer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				if !backoffTimer.Stop() {
					select {
					case <-backoffTimer.C:
					default:
					}
				}
				return ctx.Err()
			case <-backoffTimer.C:
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		c.applyAuthorization(httpReq)
		httpReq.Header.Set("Accept", "text/event-stream")
		c.applyProviderHeaders(httpReq)

		resp, err = streamClient.Do(httpReq)
		if err != nil {
			if isRetryableOpenAIStreamStartupError(err) {
				lastErr = fmt.Errorf("connection error: %w", err)
				continue
			}
			return fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			lastErr = nil
			break
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var errResp openaiError
		errMsg := string(respBody)
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			errMsg = errResp.Error.Message
		}
		errMsg = appendRateLimitInfo(errMsg, resp)
		lastErr = fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
		if isRetryableOpenAIStreamStatus(resp.StatusCode) {
			continue
		}
		if openAIStreamingExplicitlyUnsupported(resp.StatusCode, errMsg) {
			// Some otherwise compatible endpoints only implement buffered chat
			// completions. Retry exactly once without stream=true, then emit the
			// complete validated response through the canonical stream callback.
			buffered, fallbackErr := c.Chat(ctx, req)
			if fallbackErr != nil {
				return fmt.Errorf("streaming unsupported and buffered fallback failed: %w", fallbackErr)
			}
			return emitBufferedOpenAIResponse(callback, buffered)
		}
		return lastErr
	}

	if lastErr != nil {
		return fmt.Errorf("request failed after %d stream retries: %w", maxRetries, lastErr)
	}
	defer resp.Body.Close()

	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		// A few compatible servers accept stream=true but return one ordinary
		// JSON completion. Buffer and validate it before emitting any tool call,
		// preserving fail-closed action semantics.
		var bufferedResponse openaiResponse
		if err := json.NewDecoder(resp.Body).Decode(&bufferedResponse); err != nil {
			return fmt.Errorf("failed to parse buffered stream response: %w", err)
		}
		buffered, err := convertOpenAIResponse(bufferedResponse)
		if err != nil {
			return err
		}
		return emitBufferedOpenAIResponse(callback, buffered)
	}

	// Parse SSE stream
	reader := resp.Body
	buf := make([]byte, 4096)
	var pendingData string
	toolCallBuilders := make(map[int]*openaiStreamToolCallBuilder)
	var inputTokens, outputTokens int
	var finishReason string

	emitDone := func() {
		toolCalls := finalizeOpenAIStreamToolCalls(toolCallBuilders)
		callback(StreamEvent{
			Type: "done",
			Data: DoneEvent{
				StopReason:   normalizeOpenAIStreamStopReason(finishReason, toolCalls),
				ToolCalls:    toolCalls,
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
			},
		})
	}

	processLine := func(line string) (bool, error) {
		line = strings.TrimSpace(line)

		if !strings.HasPrefix(line, "data:") {
			return false, nil
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			return false, nil
		}

		if data == "[DONE]" {
			emitDone()
			return true, nil
		}

		var event openaiStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			log.Debug().Err(err).Str("data", data).Msg("failed to parse stream event")
			return false, nil
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

			// Reasoning tokens. DeepSeek's direct API uses "reasoning_content";
			// OpenRouter and other OpenAI-compatible gateways use "reasoning".
			// A provider emits one or the other, so surface whichever is present.
			if reasoning := delta.ReasoningContent; reasoning != "" {
				callback(StreamEvent{
					Type: "thinking",
					Data: ThinkingEvent{Text: reasoning},
				})
			} else if reasoning := delta.Reasoning; reasoning != "" {
				callback(StreamEvent{
					Type: "thinking",
					Data: ThinkingEvent{Text: reasoning},
				})
			}

			// Tool calls
			for _, tc := range delta.ToolCalls {
				builder, exists := toolCallBuilders[tc.Index]
				if !exists {
					builder = &openaiStreamToolCallBuilder{}
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
					if rawArgs := builder.args.String(); rawArgs != "" {
						parsedInput, _ := agentcapabilities.ParseProviderToolInput(rawArgs)
						callback(StreamEvent{
							Type: "tool_progress",
							Data: ToolProgressEvent{
								ID:       builder.id,
								Name:     builder.name,
								Input:    parsedInput,
								RawInput: rawArgs,
								Phase:    "pending",
								Message:  "Receiving tool input.",
							},
						})
					}
				}
				if tc.Function.Arguments != "" {
					builder.args.WriteString(tc.Function.Arguments)
					if builder.name != "" {
						rawArgs := builder.args.String()
						parsedInput, _ := agentcapabilities.ParseProviderToolInput(rawArgs)
						message := "Receiving tool input."
						if parsedInput != nil {
							message = "Prepared tool input."
						}
						callback(StreamEvent{
							Type: "tool_progress",
							Data: ToolProgressEvent{
								ID:       builder.id,
								Name:     builder.name,
								Input:    parsedInput,
								RawInput: rawArgs,
								Phase:    "pending",
								Message:  message,
							},
						})
					}
				}
			}
		}

		return false, nil
	}

	receivedFirstChunk := false
	streamChunkTimeout := streamChunkTimeoutForRequest(c.streamChunkTimeout, c.streamFirstChunkTimeout, req.StreamIdleTimeout)
	for {
		chunkTimeout := streamChunkTimeout
		if !receivedFirstChunk {
			chunkTimeout = c.streamFirstChunkTimeout
		}
		n, err := readOpenAIStreamChunk(ctx, reader, buf, chunkTimeout)
		if n > 0 {
			receivedFirstChunk = true
			pendingData += string(buf[:n])
			lines := strings.Split(pendingData, "\n")

			// Keep the last incomplete line for next iteration
			pendingData = lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			for _, line := range lines {
				done, lineErr := processLine(line)
				if lineErr != nil {
					return lineErr
				}
				if done {
					return nil
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

	if strings.TrimSpace(pendingData) != "" {
		done, err := processLine(pendingData)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}

	if finishReason == "" {
		if len(toolCallBuilders) > 0 {
			return fmt.Errorf("stream ended before tool call completion")
		}
		return fmt.Errorf("stream ended before completion marker")
	}
	if len(toolCallBuilders) > 0 && finishReason != "tool_calls" {
		return fmt.Errorf("stream ended with finish_reason %q before completing tool calls", finishReason)
	}

	emitDone()
	return nil
}

// ListModels fetches available models from the OpenAI API
func (c *OpenAIClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	modelsURL := c.modelsEndpoint()
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.applyAuthorization(req)
	c.applyProviderHeaders(req)

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
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Object      string `json:"object"`
			Created     int64  `json:"created"`
			OwnedBy     string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Data))
	cache := GetNotableCache()
	for _, m := range result.Data {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}

		if c.isOpenRouter() {
			modelName := strings.TrimSpace(m.Name)
			if modelName == "" {
				modelName = m.ID
			}
			models = append(models, ModelInfo{
				ID:          m.ID,
				Name:        modelName,
				Description: strings.TrimSpace(m.Description),
				CreatedAt:   m.Created,
				Notable:     cache.IsNotable(notableProviderForOpenRouterModel(m.ID), m.ID, m.Created),
			})
			continue
		}

		if c.Name() != "openai" {
			modelName := strings.TrimSpace(m.Name)
			if modelName == "" {
				modelName = m.ID
			}
			description := strings.TrimSpace(m.Description)
			if description == "" && strings.TrimSpace(m.OwnedBy) != "" {
				description = strings.TrimSpace(m.OwnedBy)
			}
			models = append(models, ModelInfo{
				ID:          m.ID,
				Name:        modelName,
				Description: description,
				CreatedAt:   m.Created,
				Notable:     cache.IsNotable(c.Name(), m.ID, m.Created),
			})
			continue
		}

		if !c.usesOfficialOpenAIEndpoint() {
			modelName := strings.TrimSpace(m.Name)
			if modelName == "" {
				modelName = m.ID
			}
			description := strings.TrimSpace(m.Description)
			if description == "" && strings.TrimSpace(m.OwnedBy) != "" {
				description = strings.TrimSpace(m.OwnedBy)
			}
			models = append(models, ModelInfo{
				ID:          m.ID,
				Name:        modelName,
				Description: description,
				CreatedAt:   m.Created,
				Notable:     cache.IsNotable(c.Name(), m.ID, m.Created),
			})
			continue
		}

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

func notableProviderForOpenRouterModel(modelID string) string {
	parts := strings.SplitN(strings.TrimSpace(modelID), "/", 2)
	if len(parts) != 2 {
		return "openai"
	}

	provider := strings.ToLower(strings.TrimSpace(parts[0]))
	if provider == "" {
		return "openai"
	}
	return provider
}
