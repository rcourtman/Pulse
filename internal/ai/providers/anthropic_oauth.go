package providers

// anthropic_oauth.go - OAuth client for Claude Pro/Max subscription authentication
//
// STATUS: DISABLED IN UI - Anthropic restricts OAuth to Claude Code only
//
// This implementation follows Claude Code's OAuth flow exactly:
// 1. Get authorization code from claude.ai/oauth/authorize
// 2. Exchange code for tokens at console.anthropic.com/v1/oauth/token
// 3. Try to create API key (works for Team/Enterprise users)
// 4. Use OAuth tokens directly (for Pro/Max users)
//
// However, Anthropic has locked down their OAuth system:
// - OAuth tokens are server-side validated to only work within Claude Code
// - The error "This credential is only authorized for use with Claude Code" is returned
// - Pro/Max users don't have org:create_api_key permission to create API keys
//
// This code is kept intact in case Anthropic opens up OAuth for third-party apps in the future.
// To re-enable, update the frontend AISettings.tsx to un-disable the OAuth button.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	// OAuth endpoints - these are the same endpoints Claude Code uses
	// Authorization happens at claude.ai, token exchange at console.anthropic.com
	anthropicAuthorizeURL = "https://claude.ai/oauth/authorize"
	anthropicTokenURL     = "https://console.anthropic.com/v1/oauth/token"
	// API key creation endpoint - OAuth tokens create an API key for actual use
	anthropicAPIKeyURL = "https://api.anthropic.com/api/oauth/claude_cli/create_api_key"
	// Claude Code's client ID - this is what enables subscription-based auth
	claudeCodeClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	// Scopes needed for inference with subscription (matching Claude Code's Oo0 array)
	// org:create_api_key, user:profile, user:inference, user:sessions:claude_code
	oauthScopes = "org:create_api_key user:profile user:inference user:sessions:claude_code"
)

var (
	oauthAuthorizeURL = anthropicAuthorizeURL
	oauthTokenURL     = anthropicTokenURL
	oauthAPIKeyURL    = anthropicAPIKeyURL

	// oauthHTTPClient is used for token exchange, refresh, and API key creation.
	// It is a package variable to enable deterministic, local unit tests.
	oauthHTTPClient = &http.Client{Timeout: 30 * time.Second}
)

// OAuthTokens represents the tokens obtained from OAuth flow
type OAuthTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	Scope        string    `json:"scope"`
	ExpiresAt    time.Time `json:"-"`
	// APIKey is created from the access token and used for actual API calls
	APIKey string `json:"api_key,omitempty"`
}

// OAuthSession stores the state for an in-progress OAuth flow
type OAuthSession struct {
	State        string
	CodeVerifier string
	RedirectURI  string
	CreatedAt    time.Time
}

// GenerateOAuthSession creates a new OAuth session with PKCE parameters
func GenerateOAuthSession(redirectURI string) (*OAuthSession, error) {
	// Generate state (for CSRF protection)
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Generate code verifier (PKCE)
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	return &OAuthSession{
		State:        state,
		CodeVerifier: codeVerifier,
		RedirectURI:  redirectURI,
		CreatedAt:    time.Now(),
	}, nil
}

// GetAuthorizationURL returns the URL to redirect the user to for OAuth authorization
// The user will visit this URL, log in, and get an authorization code to paste back
func GetAuthorizationURL(session *OAuthSession) string {
	// Generate code challenge from code verifier (S256)
	h := sha256.Sum256([]byte(session.CodeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	params := url.Values{}
	// Match Claude Code's exact parameter order and values
	params.Set("code", "true") // Claude Code adds this
	params.Set("client_id", claudeCodeClientID)
	params.Set("response_type", "code")
	// Use Anthropic's official callback which displays the code for the user to copy
	params.Set("redirect_uri", "https://console.anthropic.com/oauth/code/callback")
	params.Set("scope", oauthScopes)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", session.State)

	return oauthAuthorizeURL + "?" + params.Encode()
}

// ExchangeCodeForTokens exchanges an authorization code for access tokens
func ExchangeCodeForTokens(ctx context.Context, code string, session *OAuthSession) (*OAuthTokens, error) {
	// Build JSON body (matching Claude Code's exact implementation)
	// Claude Code uses Content-Type: application/json, NOT form-urlencoded
	payload := map[string]interface{}{
		"grant_type":    "authorization_code",
		"code":          code,
		"redirect_uri":  "https://console.anthropic.com/oauth/code/callback",
		"client_id":     claudeCodeClientID,
		"code_verifier": session.CodeVerifier,
		"state":         session.State,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token request: %w", err)
	}

	log.Debug().
		Str("grant_type", "authorization_code").
		Str("client_id", claudeCodeClientID).
		Str("code_len", fmt.Sprintf("%d", len(code))).
		Str("verifier_len", fmt.Sprintf("%d", len(session.CodeVerifier))).
		Str("state_prefix", session.State[:min(8, len(session.State))]).
		Str("redirect_uri", "https://console.anthropic.com/oauth/code/callback").
		Msg("Sending OAuth token exchange request (JSON)")

	req, err := http.NewRequestWithContext(ctx, "POST", oauthTokenURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("OAuth token exchange failed")
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokens OAuthTokens
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Calculate expiration time
	if tokens.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	}

	log.Info().
		Str("token_type", tokens.TokenType).
		Int("expires_in", tokens.ExpiresIn).
		Str("scope", tokens.Scope).
		Msg("Successfully exchanged OAuth code for tokens")

	return &tokens, nil
}

// CreateAPIKeyFromOAuth uses an OAuth access token to create a real API key
// This is how Claude Code uses the OAuth flow - the OAuth token creates an API key
func CreateAPIKeyFromOAuth(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", oauthAPIKeyURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create API key request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API key request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read API key response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("Failed to create API key from OAuth token")
		return "", fmt.Errorf("API key creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		RawKey string `json:"raw_key"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse API key response: %w", err)
	}

	if result.RawKey == "" {
		return "", fmt.Errorf("no API key returned from OAuth")
	}

	log.Info().Msg("Successfully created API key from OAuth token")
	return result.RawKey, nil
}

// RefreshAccessToken uses a refresh token to get a new access token
func RefreshAccessToken(ctx context.Context, refreshToken string) (*OAuthTokens, error) {
	// Build JSON body (matching Claude Code's implementation)
	payload := map[string]interface{}{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     claudeCodeClientID,
		"scope":         oauthScopes,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	log.Debug().
		Str("grant_type", "refresh_token").
		Str("client_id", claudeCodeClientID).
		Msg("Sending OAuth token refresh request")

	req, err := http.NewRequestWithContext(ctx, "POST", oauthTokenURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Warn().
			Int("status", resp.StatusCode).
			Str("body", string(body)).
			Msg("OAuth token refresh failed")
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokens OAuthTokens
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse refresh response: %w", err)
	}

	// Keep the original refresh token if a new one wasn't returned
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = refreshToken
	}

	// Calculate expiration time
	if tokens.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	}

	log.Info().
		Int("expires_in", tokens.ExpiresIn).
		Msg("Successfully refreshed OAuth access token")

	return &tokens, nil
}

// AnthropicOAuthClient is a client that uses OAuth tokens instead of API keys
type AnthropicOAuthClient struct {
	accessToken    string
	refreshToken   string
	expiresAt      time.Time
	model          string
	baseURL        string
	client         *http.Client
	onTokenRefresh func(tokens *OAuthTokens) // Callback when tokens are refreshed
}

// NewAnthropicOAuthClient creates a new Anthropic client using OAuth tokens
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewAnthropicOAuthClient(accessToken, refreshToken string, expiresAt time.Time, model string, timeout time.Duration) *AnthropicOAuthClient {
	return NewAnthropicOAuthClientWithBaseURL(accessToken, refreshToken, expiresAt, model, "https://api.anthropic.com/v1/messages?beta=true", timeout)
}

// NewAnthropicOAuthClientWithBaseURL creates a new Anthropic OAuth client using a custom messages endpoint.
// This is useful for testing and for deployments that route requests through a proxy.
// timeout is optional - pass 0 to use the default 5 minute timeout
func NewAnthropicOAuthClientWithBaseURL(accessToken, refreshToken string, expiresAt time.Time, model, baseURL string, timeout time.Duration) *AnthropicOAuthClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1/messages?beta=true"
	}
	if timeout <= 0 {
		timeout = 300 * time.Second // Default 5 minutes
	}
	return &AnthropicOAuthClient{
		accessToken:  accessToken,
		refreshToken: refreshToken,
		expiresAt:    expiresAt,
		model:        model,
		baseURL:      baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// SetTokenRefreshCallback sets a callback that will be called when tokens are refreshed
func (c *AnthropicOAuthClient) SetTokenRefreshCallback(callback func(tokens *OAuthTokens)) {
	c.onTokenRefresh = callback
}

// Name returns the provider name
func (c *AnthropicOAuthClient) Name() string {
	return "anthropic-oauth"
}

// ensureValidToken checks if the token is expired and refreshes if needed
func (c *AnthropicOAuthClient) ensureValidToken(ctx context.Context) error {
	// Add some buffer (5 minutes) before expiration
	if time.Now().Add(5 * time.Minute).Before(c.expiresAt) {
		return nil // Token is still valid
	}

	return c.forceRefreshToken(ctx)
}

// forceRefreshToken forces a token refresh (used on 401 errors)
func (c *AnthropicOAuthClient) forceRefreshToken(ctx context.Context) error {
	if c.refreshToken == "" {
		return fmt.Errorf("access token expired and no refresh token available")
	}

	log.Info().Msg("OAuth access token expired, refreshing...")

	tokens, err := RefreshAccessToken(ctx, c.refreshToken)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}

	c.accessToken = tokens.AccessToken
	if tokens.RefreshToken != "" {
		c.refreshToken = tokens.RefreshToken
	}
	c.expiresAt = tokens.ExpiresAt

	// Notify callback of new tokens
	if c.onTokenRefresh != nil {
		c.onTokenRefresh(tokens)
	}

	return nil
}

// Chat sends a chat request to the Anthropic API using OAuth bearer token
func (c *AnthropicOAuthClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Ensure we have a valid token
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	// Convert messages to Anthropic format (same as regular client)
	messages := make([]anthropicMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == "system" {
			continue
		}

		if m.ToolResult != nil {
			contentJSON, _ := json.Marshal(m.ToolResult.Content)
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
	// Strip provider prefix if present - callers may pass the full "provider:model" string
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

	anthropicReq := anthropicRequest{
		Model:     model,
		Messages:  messages,
		MaxTokens: maxTokens,
		System:    req.System,
	}

	if req.Temperature > 0 {
		anthropicReq.Temperature = req.Temperature
	}

	if len(req.Tools) > 0 {
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
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var respBody []byte
	var lastErr error
	var skipBackoff bool

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if skipBackoff {
				skipBackoff = false
			} else {
				backoff := initialBackoff * time.Duration(1<<(attempt-1))
				log.Warn().
					Int("attempt", attempt).
					Dur("backoff", backoff).
					Str("last_error", lastErr.Error()).
					Msg("Retrying Anthropic OAuth API request after transient error")

				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		// OAuth uses Authorization: Bearer instead of x-api-key
		httpReq.Header.Set("Authorization", "Bearer "+c.accessToken)
		httpReq.Header.Set("anthropic-version", anthropicAPIVersion)
		// Required for OAuth authentication - enables subscription-based access
		httpReq.Header.Set("anthropic-beta", "oauth-2025-04-20")
		// Claude Code identification headers
		httpReq.Header.Set("x-app", "cli")
		httpReq.Header.Set("User-Agent", "claude-code/2.0.60")

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

		// Check for token expiry error (401) - force refresh
		if resp.StatusCode == 401 && c.refreshToken != "" {
			log.Info().Msg("Got 401, forcing token refresh...")
			if err := c.forceRefreshToken(ctx); err == nil {
				// Token refreshed, retry immediately without backoff
				lastErr = fmt.Errorf("token expired, retried with refreshed token")
				skipBackoff = true
				continue
			} else {
				log.Error().Err(err).Msg("Failed to refresh token after 401")
			}
		}

		if resp.StatusCode == 429 || resp.StatusCode == 529 || resp.StatusCode >= 500 {
			var errResp anthropicError
			errMsg := string(respBody)
			if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
				errMsg = errResp.Error.Message
			}
			lastErr = fmt.Errorf("API error (%d): %s", resp.StatusCode, errMsg)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			var errResp anthropicError
			if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
				return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
			}
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respBody))
		}

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

	var textContent string
	var toolCalls []ToolCall
	for _, c := range anthropicResp.Content {
		switch c.Type {
		case "text":
			textContent += c.Text
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:    c.ID,
				Name:  c.Name,
				Input: c.Input,
			})
		case "server_tool_use":
			log.Debug().
				Str("tool_name", c.Name).
				Msg("Server tool use detected (handled by Anthropic)")
		case "web_search_tool_result":
			log.Debug().Msg("Web search results received")
		}
	}

	return &ChatResponse{
		Content:      textContent,
		Model:        anthropicResp.Model,
		StopReason:   anthropicResp.StopReason,
		ToolCalls:    toolCalls,
		InputTokens:  anthropicResp.Usage.InputTokens,
		OutputTokens: anthropicResp.Usage.OutputTokens,
	}, nil
}

// TestConnection validates the OAuth token by listing models
// This avoids dependencies on specific model names which may get deprecated
func (c *AnthropicOAuthClient) TestConnection(ctx context.Context) error {
	_, err := c.ListModels(ctx)
	return err
}

func (c *AnthropicOAuthClient) modelsEndpoint() string {
	defaultURL := "https://api.anthropic.com/v1/models"
	u, err := url.Parse(c.baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return defaultURL
	}
	return u.Scheme + "://" + u.Host + "/v1/models"
}

// ListModels fetches available models from the Anthropic API using OAuth
func (c *AnthropicOAuthClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	// Ensure we have a valid token
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.modelsEndpoint(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
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
	for _, m := range result.Data {
		models = append(models, ModelInfo{
			ID:   m.ID,
			Name: m.DisplayName,
		})
	}

	return models, nil
}
