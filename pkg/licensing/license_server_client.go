package licensing

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
)

// LicenseServerClient communicates with the Pulse license server for activation and grant refresh.
type LicenseServerClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewLicenseServerClient creates a client for the license server.
// The base URL defaults to DefaultLicenseServerURL unless PULSE_LICENSE_SERVER_URL is set.
func NewLicenseServerClient(baseURL string) *LicenseServerClient {
	if baseURL == "" {
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/installations", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create activate request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

// RefreshGrant calls the license server to refresh a relay grant.
// The installationToken is sent as a Bearer token in the Authorization header.
func (c *LicenseServerClient) RefreshGrant(ctx context.Context, installationID, installationToken string, req RefreshGrantRequest) (*RefreshGrantResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal refresh request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/installations/%s/grant/refresh", c.baseURL, installationID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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

// BaseURL returns the configured license server base URL.
func (c *LicenseServerClient) BaseURL() string {
	return c.baseURL
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
