package licensing

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LicenseServerClient communicates with the Pulse license server for activation and grant refresh.
type LicenseServerClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewLicenseServerClient creates a client for the license server.
// The base URL defaults to DefaultLicenseServerURL. The PULSE_LICENSE_SERVER_URL
// env var override is only allowed in non-release builds.
func NewLicenseServerClient(baseURL string) *LicenseServerClient {
	if baseURL == "" && allowLicenseServerURLEnvOverride() {
		baseURL = os.Getenv("PULSE_LICENSE_SERVER_URL")
	}
	if baseURL == "" {
		baseURL = DefaultLicenseServerURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &LicenseServerClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Activate calls the license server to create an installation and receive a relay grant.
func (c *LicenseServerClient) Activate(ctx context.Context, req ActivateInstallationRequest) (*ActivateInstallationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal activate request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/activate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create activate request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotency-Key", generateIdempotencyKey())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("activate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var result ActivateInstallationResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode activate response: %w", err)
	}
	return &result, nil
}

// ExchangeLegacyLicense converts a legacy v5 JWT-style license into a v6 activation.
// The response shape matches normal activation so the runtime can persist activation
// state and start using grant refresh immediately.
func (c *LicenseServerClient) ExchangeLegacyLicense(ctx context.Context, req ExchangeLegacyLicenseRequest) (*ActivateInstallationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal exchange request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/licenses/exchange", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create exchange request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Idempotency-Key", generateIdempotencyKey())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, c.parseError(resp)
	}

	var result ActivateInstallationResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode exchange response: %w", err)
	}
	return &result, nil
}

// RefreshGrant calls the license server to refresh a relay grant.
// The installationToken is sent as a Bearer token in the Authorization header.
func (c *LicenseServerClient) RefreshGrant(ctx context.Context, installationID, installationToken string, req RefreshGrantRequest) (*RefreshGrantResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal refresh request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/grants/refresh", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+installationToken)

	// Use a shorter timeout for refresh since it's a background operation.
	refreshClient := *c.httpClient
	refreshClient.Timeout = 10 * time.Second

	resp, err := refreshClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result RefreshGrantResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode refresh response: %w", err)
	}
	return &result, nil
}

// FetchRevocations polls the revocation feed for events after the given sequence number.
// The feedToken authenticates access to the revocation feed.
func (c *LicenseServerClient) FetchRevocations(ctx context.Context, feedToken string, sinceSeq int64, limit int) (*RevocationFeedResponse, error) {
	if limit <= 0 {
		limit = 500
	}

	url := fmt.Sprintf("%s/v1/revocations?since_seq=%d&limit=%d", c.baseURL, sinceSeq, limit)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create revocations request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+feedToken)

	// Use a shorter timeout for polling.
	pollClient := *c.httpClient
	pollClient.Timeout = 10 * time.Second

	resp, err := pollClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("revocations request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}

	var result RevocationFeedResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode revocations response: %w", err)
	}
	return &result, nil
}

// BaseURL returns the configured license server base URL.
func (c *LicenseServerClient) BaseURL() string {
	return c.baseURL
}

// generateIdempotencyKey creates a unique key for idempotent requests.
func generateIdempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// parseError reads an error response from the license server and returns a structured LicenseServerError.
func (c *LicenseServerClient) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	apiErr := &LicenseServerError{
		StatusCode: resp.StatusCode,
		Code:       fmt.Sprintf("http_%d", resp.StatusCode),
		Message:    http.StatusText(resp.StatusCode),
	}

	// Try to parse structured error response from the license server.
	if len(body) > 0 {
		var parsed struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		}
		if json.Unmarshal(body, &parsed) == nil && parsed.Code != "" {
			apiErr.Code = parsed.Code
			apiErr.Message = parsed.Message
			apiErr.Retryable = parsed.Retryable
		}
	}

	// Mark server errors as retryable by default.
	if resp.StatusCode >= 500 {
		apiErr.Retryable = true
	}

	return apiErr
}
