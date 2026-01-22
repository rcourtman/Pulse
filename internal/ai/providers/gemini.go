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

	// Add tools if provided
	if len(req.Tools) > 0 {
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
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, model, c.apiKey)

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

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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
			lastErr = fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
			continue
		}

		// Non-retryable error
		if resp.StatusCode != http.StatusOK {
			var errResp geminiError
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
	_, err := c.ListModels(ctx)
	return err
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

	if len(req.Tools) > 0 {
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
		}
	}

	body, err := json.Marshal(geminiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Use streamGenerateContent endpoint for streaming
	url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", c.baseURL, model, c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
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
	url := fmt.Sprintf("%s/models?key=%s", c.baseURL, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
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

		// Only include the useful Gemini models for chat/agentic tasks
		// Filter out Gemma (open-source, no function calling), embedding, AQA, vision-only models
		// Keep: gemini-3-*, gemini-2.5-*, gemini-2.0-*, gemini-1.5-* (pro and flash variants)
		isUsefulModel := false
		usefulPrefixes := []string{
			"gemini-3-pro", "gemini-3-flash",
			"gemini-2.5-pro", "gemini-2.5-flash",
			"gemini-2.0-pro", "gemini-2.0-flash",
			"gemini-1.5-pro", "gemini-1.5-flash",
			"gemini-flash", "gemini-pro", // Latest aliases
		}
		for _, prefix := range usefulPrefixes {
			if strings.HasPrefix(modelID, prefix) {
				isUsefulModel = true
				break
			}
		}
		if !isUsefulModel {
			continue
		}

		// Skip experimental/deprecated variants
		if strings.Contains(modelID, "exp-") ||
			strings.Contains(modelID, "-exp") ||
			strings.Contains(modelID, "tuning") ||
			strings.Contains(modelID, "8b") { // Skip smaller variants
			continue
		}

		models = append(models, ModelInfo{
			ID:          modelID,
			Name:        m.DisplayName,
			Description: m.Description,
			Notable:     cache.IsNotable("gemini", modelID, 0),
		})
	}

	return models, nil
}
