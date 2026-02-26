package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// quickstartProxyURL is the Pulse-hosted proxy that forwards to MiniMax.
	// The API key lives server-side — self-hosted binaries never see it.
	quickstartProxyURL = "https://api.pulserelay.pro/v1/quickstart/patrol"

	quickstartModel          = "minimax-2.5m"
	quickstartRequestTimeout = 300 * time.Second // 5 minutes
	quickstartMaxRetries     = 2
	quickstartInitialBackoff = 2 * time.Second
)

// QuickstartClient implements the Provider interface for the Pulse-hosted
// quickstart proxy. It forwards patrol chat requests to the MiniMax 2.5M
// model via api.pulserelay.pro so users don't need their own API key.
type QuickstartClient struct {
	// licenseID identifies the workspace (for rate limiting / credit tracking server-side).
	licenseID string
	client    *http.Client
}

// NewQuickstartClient creates a quickstart provider that uses the hosted proxy.
func NewQuickstartClient(licenseID string) *QuickstartClient {
	return &QuickstartClient{
		licenseID: licenseID,
		client: &http.Client{
			Timeout: quickstartRequestTimeout,
		},
	}
}

// quickstartRequest is the payload sent to the hosted proxy.
type quickstartRequest struct {
	Messages []Message `json:"messages"`
	System   string    `json:"system,omitempty"`
	Tools    []Tool    `json:"tools,omitempty"`
	// LicenseID lets the server track credit usage per workspace.
	LicenseID string `json:"license_id"`
}

// quickstartResponse is the response from the hosted proxy.
type quickstartResponse struct {
	Content      string     `json:"content"`
	Model        string     `json:"model"`
	StopReason   string     `json:"stop_reason,omitempty"`
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	InputTokens  int        `json:"input_tokens,omitempty"`
	OutputTokens int        `json:"output_tokens,omitempty"`
	// Error is set when the server returns a structured error.
	Error string `json:"error,omitempty"`
}

// Chat sends a chat request through the Pulse-hosted quickstart proxy.
func (c *QuickstartClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	payload := quickstartRequest{
		Messages:  req.Messages,
		System:    req.System,
		Tools:     req.Tools,
		LicenseID: c.licenseID,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("quickstart: marshal request: %w", err)
	}

	var lastErr error
	backoff := quickstartInitialBackoff

	for attempt := 0; attempt <= quickstartMaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, quickstartProxyURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("quickstart: create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("quickstart: request failed: %w", err)
			log.Warn().Err(err).Int("attempt", attempt+1).Msg("Quickstart proxy request failed")
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("quickstart: read response: %w", readErr)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("quickstart: server returned %d: %s", resp.StatusCode, string(respBody))
			// Don't retry on client errors (4xx)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return nil, lastErr
			}
			continue
		}

		var result quickstartResponse
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, fmt.Errorf("quickstart: decode response: %w", err)
		}

		if result.Error != "" {
			return nil, fmt.Errorf("quickstart: server error: %s", result.Error)
		}

		return &ChatResponse{
			Content:      result.Content,
			Model:        result.Model,
			StopReason:   result.StopReason,
			ToolCalls:    result.ToolCalls,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
		}, nil
	}

	return nil, lastErr
}

// TestConnection validates connectivity to the quickstart proxy.
func (c *QuickstartClient) TestConnection(ctx context.Context) error {
	// Simple connectivity check — send a minimal request.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, quickstartProxyURL, bytes.NewReader([]byte(`{"messages":[],"license_id":"`+c.licenseID+`"}`)))
	if err != nil {
		return fmt.Errorf("quickstart: create test request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("quickstart credits require internet access. Connect your API key for offline AI Patrol")
	}
	resp.Body.Close()

	// Any response (even 400) means the server is reachable.
	return nil
}

// Name returns the provider name.
func (c *QuickstartClient) Name() string {
	return "quickstart"
}

// ChatStream implements StreamingProvider by calling Chat and emitting the result
// as a single content event. The quickstart proxy is synchronous (no server-sent events).
func (c *QuickstartClient) ChatStream(ctx context.Context, req ChatRequest, callback StreamCallback) error {
	resp, err := c.Chat(ctx, req)
	if err != nil {
		return err
	}

	// Emit content as a single event
	if resp.Content != "" {
		callback(StreamEvent{Type: "content", Data: ContentEvent{Text: resp.Content}})
	}

	// Emit tool calls as ToolStartEvent (matches what the agentic loop expects)
	for _, tc := range resp.ToolCalls {
		callback(StreamEvent{Type: "tool_start", Data: ToolStartEvent{
			ID:    tc.ID,
			Name:  tc.Name,
			Input: tc.Input,
		}})
	}

	// Emit done with usage and tool calls (agentic loop reads ToolCalls from DoneEvent)
	callback(StreamEvent{Type: "done", Data: DoneEvent{
		StopReason:   resp.StopReason,
		ToolCalls:    resp.ToolCalls,
		InputTokens:  resp.InputTokens,
		OutputTokens: resp.OutputTokens,
	}})

	return nil
}

// SupportsThinking returns false — the quickstart model does not support extended thinking.
func (c *QuickstartClient) SupportsThinking(_ string) bool {
	return false
}

// ListModels returns the single model available via quickstart.
func (c *QuickstartClient) ListModels(_ context.Context) ([]ModelInfo, error) {
	return []ModelInfo{
		{
			ID:      quickstartModel,
			Name:    "MiniMax 2.5M (Quickstart)",
			Notable: true,
		},
	}, nil
}
