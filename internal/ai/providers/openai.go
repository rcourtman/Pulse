package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	openaiAPIURL = "https://api.openai.com/v1/chat/completions"
)

// OpenAIClient implements the Provider interface for OpenAI's API
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
	return &OpenAIClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the provider name
func (c *OpenAIClient) Name() string {
	return "openai"
}

// openaiRequest is the request body for the OpenAI API
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
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
		messages = append(messages, openaiMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	// Use provided model or fall back to client default
	model := req.Model
	if model == "" {
		model = c.model
	}

	openaiReq := openaiRequest{
		Model:    model,
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		openaiReq.MaxTokens = req.MaxTokens
	}

	if req.Temperature > 0 {
		openaiReq.Temperature = req.Temperature
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		var errResp openaiError
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	return &ChatResponse{
		Content:      openaiResp.Choices[0].Message.Content,
		Model:        openaiResp.Model,
		StopReason:   openaiResp.Choices[0].FinishReason,
		InputTokens:  openaiResp.Usage.PromptTokens,
		OutputTokens: openaiResp.Usage.CompletionTokens,
	}, nil
}

// TestConnection validates the API key by making a minimal request
func (c *OpenAIClient) TestConnection(ctx context.Context) error {
	_, err := c.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		MaxTokens: 10,
	})
	return err
}
