package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	// defaultQuickstartProxyURL is the owned public quickstart proxy surface.
	// The API key lives server-side — self-hosted binaries never see it.
	defaultQuickstartProxyURL = "https://license.pulserelay.pro/v1/quickstart/patrol"

	quickstartModel          = config.DefaultAIModelQuickstart
	quickstartRequestTimeout = 300 * time.Second // 5 minutes
	quickstartMaxRetries     = 2
	quickstartInitialBackoff = 2 * time.Second
)

// QuickstartClient implements the Provider interface for the Pulse-hosted
// quickstart proxy. It forwards patrol chat requests through the public
// commercial API edge so users don't need their own API key.
type QuickstartClient struct {
	// licenseID is the legacy caller-chosen identity path.
	licenseID string
	// quickstartToken is the server-issued bearer token path.
	quickstartToken string
	client          *http.Client
	onStateSync     func(QuickstartServerState)
	onTokenInvalid  func()
}

func quickstartProxyURL() string {
	if override := strings.TrimSpace(os.Getenv("PULSE_AI_QUICKSTART_PROXY_URL")); override != "" {
		return override
	}
	return defaultQuickstartProxyURL
}

// NewQuickstartClient creates a quickstart provider that uses the hosted proxy.
// This is the legacy identity path retained for non-Patrol callers.
func NewQuickstartClient(licenseID string) *QuickstartClient {
	return &QuickstartClient{
		licenseID: licenseID,
		client: &http.Client{
			Timeout: quickstartRequestTimeout,
		},
	}
}

// QuickstartServerState is the authoritative quickstart inventory snapshot
// returned by the license server alongside a Patrol response.
type QuickstartServerState struct {
	CreditsRemaining *int
	CreditsTotal     *int
	TokenExpiresAt   *time.Time
}

// NewQuickstartClientWithToken creates a quickstart provider that authenticates
// requests with a server-issued bearer token.
func NewQuickstartClientWithToken(quickstartToken string, onStateSync func(QuickstartServerState), onTokenInvalid func()) *QuickstartClient {
	return &QuickstartClient{
		quickstartToken: strings.TrimSpace(quickstartToken),
		onStateSync:     onStateSync,
		onTokenInvalid:  onTokenInvalid,
		client: &http.Client{
			Timeout: quickstartRequestTimeout,
		},
	}
}

// quickstartRequest is the payload sent to the hosted proxy.
type quickstartRequest struct {
	Messages    []Message `json:"messages"`
	System      string    `json:"system,omitempty"`
	Tools       []Tool    `json:"tools,omitempty"`
	ExecutionID string    `json:"execution_id,omitempty"`
	// LicenseID is retained only for the legacy caller-chosen identity path.
	LicenseID string `json:"license_id,omitempty"`
}

// quickstartResponse is the response from the hosted proxy.
type quickstartResponse struct {
	Content                  string     `json:"content"`
	Model                    string     `json:"model"`
	StopReason               string     `json:"stop_reason,omitempty"`
	ToolCalls                []ToolCall `json:"tool_calls,omitempty"`
	InputTokens              int        `json:"input_tokens,omitempty"`
	OutputTokens             int        `json:"output_tokens,omitempty"`
	CreditsRemaining         *int       `json:"credits_remaining,omitempty"`
	CreditsTotal             *int       `json:"credits_total,omitempty"`
	QuickstartTokenExpiresAt string     `json:"quickstart_token_expires_at,omitempty"`
	TokenExpiresAt           string     `json:"token_expires_at,omitempty"`
	Code                     string     `json:"code,omitempty"`
	// Error is set when the server returns a structured error.
	Error string `json:"error,omitempty"`
}

// QuickstartRequestError captures a structured quickstart proxy failure.
type QuickstartRequestError struct {
	StatusCode       int
	Code             string
	Message          string
	CreditsRemaining int
	CreditsTotal     int
}

func (e *QuickstartRequestError) Error() string {
	if e == nil {
		return "quickstart: request failed"
	}
	if msg := strings.TrimSpace(e.Message); msg != "" {
		return "quickstart: " + msg
	}
	return fmt.Sprintf("quickstart: server returned %d", e.StatusCode)
}

func (e *QuickstartRequestError) exhausted() bool {
	if e == nil {
		return false
	}
	if e.StatusCode == http.StatusPaymentRequired {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(e.Code), "quickstart_credits_exhausted") {
		return true
	}
	return e.CreditsTotal > 0 && e.CreditsRemaining == 0
}

func (e *QuickstartRequestError) unavailable() bool {
	if e == nil {
		return false
	}
	switch e.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return strings.Contains(strings.ToLower(strings.TrimSpace(e.Code)), "unavailable")
	}
}

// IsQuickstartCreditsExhausted reports whether err represents an exhausted
// quickstart inventory response from the server.
func IsQuickstartCreditsExhausted(err error) bool {
	typed, ok := err.(*QuickstartRequestError)
	return ok && typed.exhausted()
}

// IsQuickstartUnavailable reports whether err represents a quickstart transport
// failure returned by the license server.
func IsQuickstartUnavailable(err error) bool {
	typed, ok := err.(*QuickstartRequestError)
	return ok && typed.unavailable()
}

// Chat sends a chat request through the Pulse-hosted quickstart proxy.
func (c *QuickstartClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	payload := quickstartRequest{
		Messages:    req.Messages,
		System:      req.System,
		Tools:       req.Tools,
		ExecutionID: strings.TrimSpace(req.ExecutionID),
	}
	if strings.TrimSpace(c.quickstartToken) == "" {
		payload.LicenseID = c.licenseID
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

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, quickstartProxyURL(), bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("quickstart: create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if token := strings.TrimSpace(c.quickstartToken); token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+token)
		}

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
			var failure quickstartResponse
			if err := json.Unmarshal(respBody, &failure); err == nil {
				if failure.Code == "quickstart_credits_exhausted" && failure.CreditsRemaining == nil {
					remaining := 0
					failure.CreditsRemaining = &remaining
				}
				c.syncServerState(failure)
			}
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				c.invalidateToken()
			}
			lastErr = &QuickstartRequestError{
				StatusCode:       resp.StatusCode,
				Code:             strings.TrimSpace(failure.Code),
				Message:          firstNonEmpty(strings.TrimSpace(failure.Error), strings.TrimSpace(string(respBody))),
				CreditsRemaining: quickstartOptionalInt(failure.CreditsRemaining),
				CreditsTotal:     quickstartOptionalInt(failure.CreditsTotal),
			}
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
		c.syncServerState(result)

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
	requestBody := `{"messages":[]}`
	if strings.TrimSpace(c.quickstartToken) == "" {
		requestBody = `{"messages":[],"license_id":"` + c.licenseID + `"}`
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, quickstartProxyURL(), bytes.NewReader([]byte(requestBody)))
	if err != nil {
		return fmt.Errorf("quickstart: create test request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(c.quickstartToken); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

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
			Name:    "Pulse Hosted Quickstart",
			Notable: true,
		},
	}, nil
}

func (c *QuickstartClient) syncServerState(result quickstartResponse) {
	if c == nil || c.onStateSync == nil {
		return
	}
	state := QuickstartServerState{}
	hasState := false
	if result.CreditsRemaining != nil {
		remaining := *result.CreditsRemaining
		state.CreditsRemaining = &remaining
		hasState = true
	}
	if result.CreditsTotal != nil {
		total := *result.CreditsTotal
		state.CreditsTotal = &total
		hasState = true
	}
	if rawExpiry := firstNonEmpty(result.QuickstartTokenExpiresAt, result.TokenExpiresAt); rawExpiry != "" {
		if parsed, err := time.Parse(time.RFC3339, rawExpiry); err == nil {
			state.TokenExpiresAt = &parsed
			hasState = true
		}
	}
	if !hasState {
		return
	}
	c.onStateSync(state)
}

func (c *QuickstartClient) invalidateToken() {
	if c == nil || c.onTokenInvalid == nil {
		return
	}
	c.onTokenInvalid()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func quickstartOptionalInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
