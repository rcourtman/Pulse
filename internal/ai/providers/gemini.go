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

	"github.com/rs/zerolog/log"
)

const (
	geminiAPIURL         = "https://generativelanguage.googleapis.com/v1beta"
	geminiMaxRetries     = 3
	geminiInitialBackoff = 2 * time.Second
)

// GeminiClient implements the Provider interface for Google's Gemini API
type GeminiClient struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewGeminiClient creates a new Gemini API client
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewGeminiClient(apiKey, model, baseURL string, timeout time.Duration) *GeminiClient {
	if baseURL == "" {
		baseURL = geminiAPIURL
	}
	// Strip provider prefix if present - the model should be just the model name
	// Strip provider prefix if present - the model should be just the model name
	model = strings.TrimPrefix(model, "gemini:")
	if timeout <= 0 {
		timeout = 300 * time.Second // Default 5 minutes
	}
	return &GeminiClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Name returns the provider name
func (c *GeminiClient) Name() string {
	return "gemini"
}

// geminiRequest is the request body for the Gemini API
type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
	Tools             []geminiToolDef         `json:"tools,omitempty"`
	ToolConfig        *geminiToolConfig       `json:"toolConfig,omitempty"`
}

// geminiToolConfig controls how the model uses tools
// See: https://ai.google.dev/api/caching#ToolConfig
type geminiToolConfig struct {
	FunctionCallingConfig *geminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type geminiFunctionCallingConfig struct {
	Mode string `json:"mode"` // AUTO, ANY, or NONE
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text                  string                  `json:"text,omitempty"`
	FunctionCall          *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse      *geminiFunctionResponse `json:"functionResponse,omitempty"`
	ThoughtSignature      json.RawMessage         `json:"thoughtSignature,omitempty"`
	ThoughtSignatureSnake json.RawMessage         `json:"thought_signature,omitempty"`
}

type geminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string `json:"name"`
	Response struct {
		Content string `json:"content"`
	} `json:"response"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
}

type geminiToolDef struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// geminiResponse is the response from the Gemini API
type geminiResponse struct {
	Candidates     []geminiCandidate     `json:"candidates"`
	UsageMetadata  *geminiUsageMetadata  `json:"usageMetadata"`
	PromptFeedback *geminiPromptFeedback `json:"promptFeedback,omitempty"`
}

type geminiCandidate struct {
	Content       geminiContent  `json:"content"`
	FinishReason  string         `json:"finishReason"`
	SafetyRatings []geminySafety `json:"safetyRatings,omitempty"`
}

type geminySafety struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
	Blocked     bool   `json:"blocked"`
}

type geminiPromptFeedback struct {
	BlockReason   string         `json:"blockReason,omitempty"`
	SafetyRatings []geminySafety `json:"safetyRatings,omitempty"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

// sanitizeGeminiContents validates and repairs message ordering for Gemini's constraints.
// Gemini requires that a model message containing function calls must be immediately
// followed by a user message containing function responses. If pruning or errors
// leave orphaned function calls (model+functionCalls not followed by function responses),
// this strips the function call parts, keeping only text content if present.
func sanitizeGeminiContents(contents []geminiContent) []geminiContent {
	result := make([]geminiContent, 0, len(contents))

	for i, c := range contents {
		// Check if this is a model message with function calls
		hasFunctionCall := false
		for _, p := range c.Parts {
			if p.FunctionCall != nil {
				hasFunctionCall = true
				break
			}
		}

		if c.Role == "model" && hasFunctionCall {
			// Check if the next message is a user message with function responses
			hasFollowingResponse := false
			if i+1 < len(contents) {
				next := contents[i+1]
				if next.Role == "user" {
					for _, p := range next.Parts {
						if p.FunctionResponse != nil {
							hasFollowingResponse = true
							break
						}
					}
				}
			}

			if !hasFollowingResponse {
				// Orphaned function calls — strip them, keep text only
				var textParts []geminiPart
				for _, p := range c.Parts {
					if p.Text != "" && p.FunctionCall == nil {
						textParts = append(textParts, geminiPart{Text: p.Text})
					}
				}
				if len(textParts) > 0 {
					result = append(result, geminiContent{
						Role:  c.Role,
						Parts: textParts,
					})
				}
				log.Debug().
					Int("message_index", i).
					Msg("[Gemini] Stripped orphaned function calls from model message")
				continue
			}
		}

		// Check if this is a user message with function responses
		// that isn't preceded by a model message with function calls
		hasFunctionResponse := false
		for _, p := range c.Parts {
			if p.FunctionResponse != nil {
				hasFunctionResponse = true
				break
			}
		}

		if c.Role == "user" && hasFunctionResponse {
			hasPrecedingCall := false
			if i > 0 {
				prev := contents[i-1]
				if prev.Role == "model" {
					for _, p := range prev.Parts {
						if p.FunctionCall != nil {
							hasPrecedingCall = true
							break
						}
					}
				}
			}
			// Also check if the preceding message in result has function calls
			// (it might have been the immediately previous content we just added)
			if !hasPrecedingCall && len(result) > 0 {
				prev := result[len(result)-1]
				if prev.Role == "model" {
					for _, p := range prev.Parts {
						if p.FunctionCall != nil {
							hasPrecedingCall = true
							break
						}
					}
				}
			}

			if !hasPrecedingCall {
				// Orphaned function responses — drop them
				log.Debug().
					Int("message_index", i).
					Msg("[Gemini] Dropped orphaned function responses from user message")
				continue
			}
		}

		result = append(result, c)
	}

	return result
}

// convertToolChoiceToGemini converts our ToolChoice to Gemini's mode string
// Gemini uses: AUTO (default), ANY (force tool use), NONE (no tools)
// See: https://ai.google.dev/api/caching#FunctionCallingConfig
func convertToolChoiceToGemini(tc *ToolChoice) string {
	if tc == nil {
		return "AUTO"
	}
	switch tc.Type {
	case ToolChoiceAuto:
		return "AUTO"
	case ToolChoiceNone:
		return "NONE"
	case ToolChoiceAny:
		return "ANY"
	case ToolChoiceTool:
		// Gemini doesn't support forcing a specific tool, fall back to ANY
		return "ANY"
	default:
		return "AUTO"
	}
}

// Chat sends a chat request to the Gemini API
func (c *GeminiClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Convert messages to Gemini format
	contents := make([]geminiContent, 0, len(req.Messages))
	for _, m := range req.Messages {
		// Skip system messages - they go in systemInstruction
		if m.Role == "system" {
			continue
		}

		// Convert role names (Gemini uses "user" and "model")
		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		// Handle tool results
		if m.ToolResult != nil {
			contents = append(contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{
					{
						FunctionResponse: &geminiFunctionResponse{
							Name: m.ToolResult.ToolUseID, // In Gemini, this is the function name
							Response: struct {
								Content string `json:"content"`
							}{
								Content: m.ToolResult.Content,
							},
						},
					},
				},
			})
			continue
		}

		// Handle assistant messages with tool calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			parts := make([]geminiPart, 0)
			if m.Content != "" {
				parts = append(parts, geminiPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: tc.Input,
					},
					ThoughtSignature: tc.ThoughtSignature,
				})
			}
			contents = append(contents, geminiContent{
				Role:  "model",
				Parts: parts,
			})
			continue
		}

		// Skip messages with empty content - Gemini requires at least one of text, functionCall, or functionResponse
		if m.Content == "" {
			continue
		}

		// Simple text message
		contents = append(contents, geminiContent{
			Role: role,
			Parts: []geminiPart{
				{Text: m.Content},
			},
		})
	}

	// Sanitize message ordering for Gemini's constraints
	contents = sanitizeGeminiContents(contents)

	// Use provided model or fall back to client default
	model := req.Model
	// Strip provider prefix if present
	if strings.HasPrefix(model, "gemini:") {
		model = strings.TrimPrefix(model, "gemini:")
	}
	if model == "" {
		model = c.model
	}

	geminiReq := geminiRequest{
		Contents: contents,
	}

	// Add system instruction if provided
	if req.System != "" {
		geminiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.System}},
		}
	}

	// Add generation config
	geminiReq.GenerationConfig = &geminiGenerationConfig{}
	if req.MaxTokens > 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	} else {
		geminiReq.GenerationConfig.MaxOutputTokens = 8192
	}
	if req.Temperature > 0 {
		geminiReq.GenerationConfig.Temperature = req.Temperature
	}

	// Add tools if provided (unless ToolChoice is None)
	shouldAddTools := len(req.Tools) > 0
	if req.ToolChoice != nil && req.ToolChoice.Type == ToolChoiceNone {
		shouldAddTools = false
	}

	if shouldAddTools {
		funcDecls := make([]geminiFunctionDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			// Skip non-function tools
			if t.Type != "" && t.Type != "function" {
				continue
			}
			funcDecls = append(funcDecls, geminiFunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		if len(funcDecls) > 0 {
			geminiReq.Tools = []geminiToolDef{{FunctionDeclarations: funcDecls}}

			// Add tool_config based on ToolChoice
			// Gemini uses: AUTO (default), ANY (force tool use), NONE (no tools)
			geminiReq.ToolConfig = &geminiToolConfig{
				FunctionCallingConfig: &geminiFunctionCallingConfig{
					Mode: convertToolChoiceToGemini(req.ToolChoice),
				},
			}

			log.Debug().Int("tool_count", len(funcDecls)).Strs("tool_names", func() []string {
				names := make([]string, len(funcDecls))
				for i, f := range funcDecls {
					names[i] = f.Name
				}
				return names
			}()).Msg("Gemini request includes tools")
		}
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build the URL with API key
	generateContentURL := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, model, c.apiKey)

	log.Debug().Str("model", model).Str("base_url", c.baseURL).Msg("Gemini Chat request")

	// Retry loop for transient errors
	var respBody []byte
	var lastErr error

	for attempt := 0; attempt <= geminiMaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 2s, 4s, 8s
			backoff := geminiInitialBackoff * time.Duration(1<<(attempt-1))
			log.Warn().
				Int("attempt", attempt).
				Dur("backoff", backoff).
				Str("last_error", lastErr.Error()).
				Msg("Retrying Gemini API request after transient error")

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", generateContentURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")

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
		if resp.StatusCode == 429 || resp.StatusCode == 503 || resp.StatusCode >= 500 {
			var errResp geminiError
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
			var errResp geminiError
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
		return nil, fmt.Errorf("request failed after %d retries: %w", geminiMaxRetries, lastErr)
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for prompt-level blocking
	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		log.Warn().
			Str("block_reason", geminiResp.PromptFeedback.BlockReason).
			Msg("Gemini blocked the prompt")
		return nil, fmt.Errorf("prompt blocked by Gemini: %s", geminiResp.PromptFeedback.BlockReason)
	}

	if len(geminiResp.Candidates) == 0 {
		log.Warn().Str("raw_response", string(respBody)).Msg("Gemini returned no candidates")
		return nil, fmt.Errorf("no response candidates returned")
	}

	candidate := geminiResp.Candidates[0]

	// Check for response-level blocking
	if candidate.FinishReason == "SAFETY" {
		blockedCategories := make([]string, 0)
		for _, safety := range candidate.SafetyRatings {
			if safety.Blocked {
				blockedCategories = append(blockedCategories, safety.Category)
			}
		}
		log.Warn().
			Strs("blocked_categories", blockedCategories).
			Msg("Gemini response blocked due to safety filters")
		return nil, fmt.Errorf("response blocked by Gemini safety filters: %v", blockedCategories)
	}

	// Extract content and tool calls from response
	var textContent string
	var toolCalls []ToolCall
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			textContent += part.Text
		}
		if part.FunctionCall != nil {
			// Generate a unique ID for this tool call since Gemini doesn't provide one
			// Use name + index to ensure uniqueness when same function is called multiple times
			toolID := fmt.Sprintf("%s_%d", part.FunctionCall.Name, len(toolCalls))
			signature := part.ThoughtSignature
			if len(signature) == 0 {
				signature = part.ThoughtSignatureSnake
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:               toolID,
				Name:             part.FunctionCall.Name,
				Input:            part.FunctionCall.Args,
				ThoughtSignature: signature,
			})
		}
	}

	log.Debug().
		Str("model", model).
		Int("content_length", len(textContent)).
		Int("tool_calls", len(toolCalls)).
		Str("finish_reason", candidate.FinishReason).
		Msg("Gemini Chat response parsed")

	// Map finish reason - tool_use takes priority if there are tool calls
	stopReason := candidate.FinishReason
	if len(toolCalls) > 0 {
		// If there are tool calls, always signal tool_use so the agentic loop continues
		stopReason = "tool_use"
	} else if stopReason == "STOP" {
		stopReason = "end_turn"
	}

	var inputTokens, outputTokens int
	if geminiResp.UsageMetadata != nil {
		inputTokens = geminiResp.UsageMetadata.PromptTokenCount
		outputTokens = geminiResp.UsageMetadata.CandidatesTokenCount
	}

	return &ChatResponse{
		Content:      textContent,
		Model:        model,
		StopReason:   stopReason,
		ToolCalls:    toolCalls,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}, nil
}

// TestConnection validates the API key by listing models
func (c *GeminiClient) TestConnection(ctx context.Context) error {
	if _, err := c.ListModels(ctx); err != nil {
		return fmt.Errorf("gemini test connection failed: %w", err)
	}
	return nil
}

// SupportsThinking returns true if the model supports extended thinking
func (c *GeminiClient) SupportsThinking(model string) bool {
	// Gemini models don't currently expose extended thinking in the streaming API
	return false
}

// geminiStreamEvent represents a streaming event from the Gemini API
type geminiStreamEvent struct {
	Candidates    []geminiCandidate    `json:"candidates,omitempty"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// ChatStream sends a chat request and streams the response via callback
func (c *GeminiClient) ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	// Convert messages to Gemini format (same as Chat)
	contents := make([]geminiContent, 0, len(req.Messages))

	for i := 0; i < len(req.Messages); i++ {
		m := req.Messages[i]
		if m.Role == "system" {
			continue
		}

		// Convert role names (Gemini uses "user" and "model")
		role := m.Role
		if role == "assistant" {
			role = "model"
		}

		// Handle tool results - merge consecutive tool results into one content block
		if m.ToolResult != nil {
			// Find the preceding assistant message to resolve function names
			// Gemini requires the 'name' in FunctionResponse to match the function name, not the ID
			var assistantMsg *Message
			if i > 0 && req.Messages[i-1].Role == "assistant" {
				assistantMsg = &req.Messages[i-1]
			}

			// Helper to resolve name
			resolveName := func(id string) string {
				if assistantMsg != nil {
					for _, call := range assistantMsg.ToolCalls {
						if call.ID == id {
							return call.Name
						}
					}
				}
				return id
			}

			toolName := resolveName(m.ToolResult.ToolUseID)

			parts := []geminiPart{
				{
					FunctionResponse: &geminiFunctionResponse{
						Name: toolName,
						Response: struct {
							Content string `json:"content"`
						}{
							Content: m.ToolResult.Content,
						},
					},
				},
			}

			// Look ahead for more tool results
			for i+1 < len(req.Messages) {
				next := req.Messages[i+1]
				if next.ToolResult == nil {
					break
				}

				nextToolName := resolveName(next.ToolResult.ToolUseID)

				// Add next tool result to parts
				parts = append(parts, geminiPart{
					FunctionResponse: &geminiFunctionResponse{
						Name: nextToolName,
						Response: struct {
							Content string `json:"content"`
						}{
							Content: next.ToolResult.Content,
						},
					},
				})

				// Advance index
				i++
			}

			contents = append(contents, geminiContent{
				Role:  "user",
				Parts: parts,
			})
			continue
		}

		// Handle assistant messages with tool calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			parts := make([]geminiPart, 0)
			if m.Content != "" {
				parts = append(parts, geminiPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFunctionCall{
						Name: tc.Name,
						Args: tc.Input,
					},
					ThoughtSignature: tc.ThoughtSignature,
				})
			}
			contents = append(contents, geminiContent{
				Role:  "model",
				Parts: parts,
			})
			continue
		}

		// Skip messages with empty content - Gemini requires at least one of text, functionCall, or functionResponse
		if m.Content == "" {
			continue
		}

		// Simple text message
		contents = append(contents, geminiContent{
			Role: role,
			Parts: []geminiPart{
				{Text: m.Content},
			},
		})
	}

	// Sanitize message ordering for Gemini's constraints
	contents = sanitizeGeminiContents(contents)

	model := req.Model
	if strings.HasPrefix(model, "gemini:") {
		model = strings.TrimPrefix(model, "gemini:")
	}
	if model == "" {
		model = c.model
	}

	geminiReq := geminiRequest{
		Contents: contents,
	}

	if req.System != "" {
		geminiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.System}},
		}
	}

	geminiReq.GenerationConfig = &geminiGenerationConfig{}
	if req.MaxTokens > 0 {
		geminiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	} else {
		geminiReq.GenerationConfig.MaxOutputTokens = 8192
	}
	if req.Temperature > 0 {
		geminiReq.GenerationConfig.Temperature = req.Temperature
	}

	// Add tools if provided (unless ToolChoice is None) - same as non-streaming
	shouldAddTools := len(req.Tools) > 0
	if req.ToolChoice != nil && req.ToolChoice.Type == ToolChoiceNone {
		shouldAddTools = false
	}

	if shouldAddTools {
		funcDecls := make([]geminiFunctionDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			if t.Type != "" && t.Type != "function" {
				continue
			}
			funcDecls = append(funcDecls, geminiFunctionDeclaration{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			})
		}
		if len(funcDecls) > 0 {
			geminiReq.Tools = []geminiToolDef{{FunctionDeclarations: funcDecls}}

			// Add tool_config based on ToolChoice (same as non-streaming)
			geminiReq.ToolConfig = &geminiToolConfig{
				FunctionCallingConfig: &geminiFunctionCallingConfig{
					Mode: convertToolChoiceToGemini(req.ToolChoice),
				},
			}

			// Log tool names for debugging tool selection issues
			toolNames := make([]string, len(funcDecls))
			for i, f := range funcDecls {
				toolNames[i] = f.Name
			}
			log.Debug().
				Int("tool_count", len(funcDecls)).
				Strs("tool_names", toolNames).
				Msg("Gemini stream request includes tools")
		}
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log the full request body for debugging (at trace level to avoid noise)
	log.Trace().
		Str("model", model).
		RawJSON("request_body", body).
		Msg("Gemini stream request body")

	// Use streamGenerateContent endpoint for streaming
	streamGenerateContentURL := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", c.baseURL, model, c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", streamGenerateContentURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		var errResp geminiError
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
	var inputTokens, outputTokens int
	var finishReason string

	for {
		n, err := reader.Read(buf)
		if n > 0 {
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

				if data == "" {
					continue
				}

				var event geminiStreamEvent
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					log.Debug().Err(err).Str("data", data).Msg("Failed to parse Gemini stream event")
					continue
				}

				if event.UsageMetadata != nil {
					inputTokens = event.UsageMetadata.PromptTokenCount
					outputTokens = event.UsageMetadata.CandidatesTokenCount
				}

				for _, candidate := range event.Candidates {
					if candidate.FinishReason != "" {
						finishReason = candidate.FinishReason
					}

					for _, part := range candidate.Content.Parts {
						if part.Text != "" {
							callback(StreamEvent{
								Type: "content",
								Data: ContentEvent{Text: part.Text},
							})
						}

						if part.FunctionCall != nil {
							toolID := fmt.Sprintf("%s_%d", part.FunctionCall.Name, len(toolCalls))
							signature := part.ThoughtSignature
							if len(signature) == 0 {
								signature = part.ThoughtSignatureSnake
							}
							log.Debug().
								Str("tool_name", part.FunctionCall.Name).
								Interface("tool_args", part.FunctionCall.Args).
								Msg("Gemini called tool")
							callback(StreamEvent{
								Type: "tool_start",
								Data: ToolStartEvent{
									ID:    toolID,
									Name:  part.FunctionCall.Name,
									Input: part.FunctionCall.Args,
								},
							})
							toolCalls = append(toolCalls, ToolCall{
								ID:               toolID,
								Name:             part.FunctionCall.Name,
								Input:            part.FunctionCall.Args,
								ThoughtSignature: signature,
							})
						}
					}
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

	// Send done event
	stopReason := finishReason
	if len(toolCalls) > 0 {
		stopReason = "tool_use"
	} else if stopReason == "STOP" {
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

// ListModels fetches available models from the Gemini API
func (c *GeminiClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	modelsURL := fmt.Sprintf("%s/models?key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

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
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	models := make([]ModelInfo, 0, len(result.Models))
	cache := GetNotableCache()
	for _, m := range result.Models {
		// Only include models that support generateContent (chat)
		supportsChat := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsChat = true
				break
			}
		}
		if !supportsChat {
			continue
		}

		// Extract model ID from the full name (e.g., "models/gemini-1.5-pro" -> "gemini-1.5-pro")
		modelID := strings.TrimPrefix(m.Name, "models/")

		models = append(models, ModelInfo{
			ID:          modelID,
			Name:        m.DisplayName,
			Description: m.Description,
			Notable:     cache.IsNotable("gemini", modelID, 0),
		})
	}

	return models, nil
}
