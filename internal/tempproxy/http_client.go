package tempproxy

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPClient communicates with pulse-sensor-proxy via HTTPS
type HTTPClient struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
	timeout    time.Duration
}

// NewHTTPClient creates a new HTTP-based proxy client
func NewHTTPClient(baseURL, authToken string) *HTTPClient {
	// Create HTTP client with TLS and reasonable timeouts
	httpClient := &http.Client{
		Timeout: defaultTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 2,
		},
	}

	return &HTTPClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		authToken:  authToken,
		httpClient: httpClient,
		timeout:    defaultTimeout,
	}
}

// IsAvailable checks if the HTTP proxy is accessible
// For HTTP mode, we consider it available if URL and token are configured
func (c *HTTPClient) IsAvailable() bool {
	return c.baseURL != "" && c.authToken != ""
}

// GetTemperature fetches temperature data from a node via HTTP
func (c *HTTPClient) GetTemperature(nodeHost string) (string, error) {
	if !c.IsAvailable() {
		return "", &ProxyError{
			Type:      ErrorTypeTransport,
			Message:   "HTTP proxy not configured",
			Retryable: false,
		}
	}

	// Build request URL
	reqURL := fmt.Sprintf("%s/temps?node=%s", c.baseURL, url.QueryEscape(nodeHost))

	// Create request
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return "", &ProxyError{
			Type:      ErrorTypeTransport,
			Message:   "failed to create HTTP request",
			Retryable: false,
			Wrapped:   err,
		}
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	req.Header.Set("Accept", "application/json")

	// Execute request with retries
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := calculateBackoff(attempt)
			time.Sleep(backoff)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = &ProxyError{
				Type:      ErrorTypeTransport,
				Message:   fmt.Sprintf("HTTP request failed (attempt %d/%d)", attempt+1, maxRetries),
				Retryable: true,
				Wrapped:   err,
			}
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = &ProxyError{
				Type:      ErrorTypeTransport,
				Message:   "failed to read response body",
				Retryable: true,
				Wrapped:   err,
			}
			continue
		}

		// Check HTTP status
		if resp.StatusCode == http.StatusUnauthorized {
			return "", &ProxyError{
				Type:      ErrorTypeAuth,
				Message:   "authentication failed - invalid token",
				Retryable: false,
			}
		}

		if resp.StatusCode == http.StatusForbidden {
			return "", &ProxyError{
				Type:      ErrorTypeAuth,
				Message:   "node not allowed by proxy",
				Retryable: false,
			}
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = &ProxyError{
				Type:      ErrorTypeTransport,
				Message:   "rate limit exceeded",
				Retryable: true,
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return "", &ProxyError{
				Type:      ErrorTypeTransport,
				Message:   fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
				Retryable: resp.StatusCode >= 500,
			}
		}

		// Parse JSON response
		var jsonResp struct {
			Node        string      `json:"node"`
			Temperature interface{} `json:"temperature"`
		}

		if err := json.Unmarshal(body, &jsonResp); err != nil {
			return "", &ProxyError{
				Type:      ErrorTypeTransport,
				Message:   "failed to parse response JSON",
				Retryable: false,
				Wrapped:   err,
			}
		}

		// Convert temperature data back to JSON string (matching socket interface)
		tempJSON, err := json.Marshal(jsonResp.Temperature)
		if err != nil {
			return "", &ProxyError{
				Type:      ErrorTypeTransport,
				Message:   "failed to marshal temperature data",
				Retryable: false,
				Wrapped:   err,
			}
		}

		return string(tempJSON), nil
	}

	// All retries exhausted
	if lastErr != nil {
		return "", lastErr
	}

	return "", &ProxyError{
		Type:      ErrorTypeUnknown,
		Message:   "all retry attempts failed",
		Retryable: false,
	}
}
