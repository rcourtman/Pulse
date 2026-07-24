package providers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

const defaultOllamaBaseURL = "http://localhost:11434"

var ollamaOutboundHTTPOptions = securityutil.RestrictedOutboundHTTPOptions{
	AllowedSchemes:  []string{"http", "https"},
	AllowPrivateIPs: true,
	AllowLoopback:   true,
}

// OllamaClient implements the Provider interface for Ollama's local API
type OllamaClient struct {
	model        string
	baseURL      string
	username     string
	password     string
	keepAlive    string
	client       *http.Client // For non-streaming requests (has overall timeout)
	streamClient *http.Client // For streaming requests (no overall timeout — relies on context)
}

// NewOllamaClient creates a new Ollama API client
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewOllamaClient(model, baseURL, username, password string, timeout time.Duration) (*OllamaClient, error) {
	return NewOllamaClientWithKeepAlive(model, baseURL, username, password, config.DefaultOllamaKeepAlive, timeout)
}

// NewOllamaClientWithKeepAlive creates a new Ollama API client with an
// explicit keep_alive request value. Pass an empty keepAlive string to omit
// keep_alive from requests and let the Ollama server default apply.
func NewOllamaClientWithKeepAlive(model, baseURL, username, password, keepAlive string, timeout time.Duration) (*OllamaClient, error) {
	normalizedBaseURL, err := normalizeOllamaBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	normalizedKeepAlive, err := config.NormalizeOllamaKeepAlive(keepAlive)
	if err != nil {
		return nil, fmt.Errorf("normalize Ollama keep_alive: %w", err)
	}
	if timeout <= 0 {
		timeout = 300 * time.Second // Default 5 minutes
	}
	return &OllamaClient{
		model:        model,
		baseURL:      normalizedBaseURL,
		username:     username,
		password:     password,
		keepAlive:    normalizedKeepAlive,
		client:       newOllamaHTTPClient(timeout, false),
		streamClient: newOllamaHTTPClient(timeout, true),
	}, nil
}

func normalizeOllamaBaseURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		raw = defaultOllamaBaseURL
	}

	baseURL, err := securityutil.NormalizeHTTPBaseURL(raw, "")
	if err != nil {
		return "", fmt.Errorf("normalize Ollama base URL: %w", err)
	}

	baseURL.Path = strings.TrimSuffix(baseURL.Path, "/")
	baseURL.Path = strings.TrimSuffix(baseURL.Path, "/api")
	baseURL.Path = strings.TrimSuffix(baseURL.Path, "/")
	baseURL.RawPath = ""

	return baseURL.String(), nil
}

func newOllamaHTTPClient(timeout time.Duration, streaming bool) *http.Client {
	clientTimeout := timeout
	options := ollamaOutboundHTTPOptions
	if streaming {
		clientTimeout = 0
		options.ResponseHeaderTimeout = timeout
	}

	return securityutil.NewRestrictedOutboundHTTPClient(clientTimeout, options)
}

func (c *OllamaClient) applyAuth(req *http.Request) {
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
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
	// KeepAlive controls how long Ollama keeps the model loaded after the
	// request completes. Ollama's default is 5 minutes; without this field
	// every Pulse Chat call refreshes that 5-minute window, so a single
	// call from Patrol or alert analysis can keep the model warm in RAM
	// indefinitely if any other Ollama traffic happens within 5 minutes
	// (#1425). Pulse passes a short value so the model unloads shortly
	// after a Patrol burst or one-shot analysis ends. Accepts duration
	// strings like "30s", "5m", or "0" for immediate unload.
	KeepAlive any `json:"keep_alive,omitempty"`
}

func ollamaKeepAliveRequestValue(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		return seconds
	}
	return value
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	Thinking  string           `json:"thinking,omitempty"`   // Prior-turn reasoning for assistant messages
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
	Thinking  string           `json:"thinking,omitempty"` // Reasoning tokens from thinking models (e.g. qwen3)
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
			Role:     m.Role,
			Content:  m.Content,
			Thinking: m.ReasoningContent,
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
	if model == "" {
		return nil, fmt.Errorf("no model specified")
	}

	ollamaReq := ollamaRequest{
		Model:     model,
		Messages:  messages,
		Stream:    false, // Non-streaming for now
		KeepAlive: ollamaKeepAliveRequestValue(c.keepAlive),
	}

	// Convert tools to Ollama format
	// Note: Ollama doesn't support tool_choice like Anthropic/OpenAI
	// We handle ToolChoiceNone by not adding tools, but can't force tool use
	shouldAddTools := len(req.Tools) > 0
	if req.ToolChoice != nil && req.ToolChoice.Type == ToolChoiceNone {
		shouldAddTools = false
	}

	if shouldAddTools {
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

	chatURL := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	c.applyAuth(httpReq)

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
		Content:          ollamaResp.Message.Content,
		ReasoningContent: ollamaResp.Message.Thinking,
		Model:            ollamaResp.Model,
		StopReason:       ollamaResp.DoneReason,
		InputTokens:      ollamaResp.PromptEvalCount,
		OutputTokens:     ollamaResp.EvalCount,
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
	// Ollama's native /api/chat streams reasoning in message.thinking for
	// thinking models (e.g. qwen3). Whether a given model actually thinks is
	// decided by the model itself; the provider passes through whatever it emits.
	return true
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
			Role:     m.Role,
			Content:  m.Content,
			Thinking: m.ReasoningContent,
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
		return fmt.Errorf("no model specified")
	}

	ollamaReq := ollamaRequest{
		Model:     model,
		Messages:  messages,
		Stream:    true, // Enable streaming
		KeepAlive: ollamaKeepAliveRequestValue(c.keepAlive),
	}

	// Handle tools with tool_choice support (same as non-streaming)
	shouldAddTools := len(req.Tools) > 0
	if req.ToolChoice != nil && req.ToolChoice.Type == ToolChoiceNone {
		shouldAddTools = false
	}

	if shouldAddTools {
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

	chatURL := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	c.applyAuth(httpReq)

	// Use streamClient which has no overall timeout — http.Client.Timeout
	// includes response body reading time, which kills slow streaming responses.
	// Context cancellation (from the agentic loop) handles timeouts instead.
	resp, err := c.streamClient.Do(httpReq)
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

		// Reasoning tokens arrive in message.thinking before any content for
		// thinking models (e.g. qwen3); surface them live instead of dead air.
		if chunk.Message.Thinking != "" {
			callback(StreamEvent{
				Type: "thinking",
				Data: ThinkingEvent{Text: chunk.Message.Thinking},
			})
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
// and ensuring the configured model is available from the local model registry.
func (c *OllamaClient) TestConnection(ctx context.Context) error {
	versionURL := c.baseURL + "/api/version"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.applyAuth(httpReq)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	models, err := c.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("connected to Ollama version endpoint but failed to list models: %w", err)
	}
	if c.model != "" && !ollamaModelAvailable(c.model, models) {
		available := make([]string, 0, len(models))
		for _, model := range models {
			label := strings.TrimSpace(model.ID)
			if label == "" {
				label = strings.TrimSpace(model.Name)
			}
			if label != "" {
				available = append(available, label)
			}
		}
		if len(available) > 0 {
			return fmt.Errorf("connected to Ollama but model %q is not available; found: %s", normalizeOllamaModelRef(c.model), strings.Join(available, ", "))
		}
		return fmt.Errorf("connected to Ollama but model %q is not available", normalizeOllamaModelRef(c.model))
	}

	return nil
}

type ollamaShowResponse struct {
	ModifiedAt   string                 `json:"modified_at"`
	Details      ollamaShowDetails      `json:"details"`
	ModelInfo    map[string]interface{} `json:"model_info"`
	Capabilities []string               `json:"capabilities"`
}

type ollamaShowDetails struct {
	Family            string `json:"family"`
	ParameterSize     string `json:"parameter_size"`
	QuantizationLevel string `json:"quantization_level"`
}

// InspectModel reads Ollama's local model metadata. This binds readiness
// evidence to a concrete model build and replaces the unsafe generic context
// assumption for local models. It deliberately does not treat a declared
// "tools" capability as proof; the readiness advisor still runs streaming,
// multi-turn tool probes.
func (c *OllamaClient) InspectModel(ctx context.Context, model string) (*ModelDiagnostics, error) {
	model = normalizeOllamaModelRef(model)
	if model == "" {
		model = normalizeOllamaModelRef(c.model)
	}
	if model == "" {
		return nil, fmt.Errorf("no model specified")
	}

	body, err := json.Marshal(map[string]string{"model": model})
	if err != nil {
		return nil, fmt.Errorf("marshal Ollama model inspection request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/show", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Ollama model inspection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("inspect Ollama model: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read Ollama model inspection response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama model inspection failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var inspected ollamaShowResponse
	if err := json.Unmarshal(responseBody, &inspected); err != nil {
		return nil, fmt.Errorf("parse Ollama model inspection response: %w", err)
	}
	contextWindow := 0
	for key, value := range inspected.ModelInfo {
		if !strings.HasSuffix(strings.ToLower(key), ".context_length") {
			continue
		}
		if n, ok := value.(float64); ok && int(n) > contextWindow {
			contextWindow = int(n)
		}
	}

	capabilities := append([]string(nil), inspected.Capabilities...)
	sort.Strings(capabilities)
	fingerprintPayload, _ := json.Marshal(struct {
		Model        string                 `json:"model"`
		ModifiedAt   string                 `json:"modified_at"`
		Details      ollamaShowDetails      `json:"details"`
		ModelInfo    map[string]interface{} `json:"model_info"`
		Capabilities []string               `json:"capabilities"`
	}{model, inspected.ModifiedAt, inspected.Details, inspected.ModelInfo, capabilities})
	fingerprint := sha256.Sum256(fingerprintPayload)

	return &ModelDiagnostics{
		Fingerprint:       fmt.Sprintf("sha256:%x", fingerprint[:]),
		Family:            inspected.Details.Family,
		ParameterSize:     inspected.Details.ParameterSize,
		QuantizationLevel: inspected.Details.QuantizationLevel,
		ContextWindow:     contextWindow,
		Capabilities:      capabilities,
	}, nil
}

func normalizeOllamaModelRef(model string) string {
	model = strings.TrimSpace(model)
	if strings.HasPrefix(model, "ollama:") {
		model = strings.TrimPrefix(model, "ollama:")
	}
	return model
}

func splitOllamaModelRef(model string) (string, string) {
	model = normalizeOllamaModelRef(model)
	if model == "" {
		return "", ""
	}
	idx := strings.LastIndex(model, ":")
	if idx == -1 {
		return model, ""
	}
	return model[:idx], model[idx+1:]
}

func ollamaModelAvailable(model string, available []ModelInfo) bool {
	wantName, wantTag := splitOllamaModelRef(model)
	if wantName == "" {
		return len(available) > 0
	}

	for _, candidate := range available {
		ref := strings.TrimSpace(candidate.ID)
		if ref == "" {
			ref = strings.TrimSpace(candidate.Name)
		}
		haveName, haveTag := splitOllamaModelRef(ref)
		if haveName == "" {
			continue
		}
		if wantName != haveName {
			continue
		}
		if wantTag == "" || haveTag == "" || wantTag == haveTag {
			return true
		}
	}

	return false
}

// ListModels fetches available models from the local Ollama instance
func (c *OllamaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	modelsURL := c.baseURL + "/api/tags"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.applyAuth(httpReq)

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
