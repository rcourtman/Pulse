package remoteconfig

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Config holds configuration for the remote config client.
type Config struct {
	PulseURL           string
	APIToken           string
	AgentID            string
	Hostname           string
	InsecureSkipVerify bool
	Logger             zerolog.Logger
}

// Client handles fetching remote configuration from the Pulse server.
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// Response represents the JSON response from the config endpoint.
type Response struct {
	Success bool   `json:"success"`
	HostID  string `json:"hostId"`
	Config  struct {
		CommandsEnabled *bool                  `json:"commandsEnabled,omitempty"`
		Settings        map[string]interface{} `json:"settings,omitempty"`
	} `json:"config"`
}

// New creates a new remote config client.
func New(cfg Config) *Client {
	if cfg.PulseURL == "" {
		cfg.PulseURL = "http://localhost:7655"
	}
	cfg.PulseURL = strings.TrimRight(cfg.PulseURL, "/")

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.InsecureSkipVerify {
		//nolint:gosec // Insecure mode is explicitly user-controlled.
		tlsConfig.InsecureSkipVerify = true
	}

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return fmt.Errorf("server returned redirect to %s", req.URL)
		},
	}

	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

// Fetch retrieves the remote configuration for this agent.
// It returns a map of settings to apply, or an error if the fetch fails.
// Returns (settings, commandsEnabled, error)
func (c *Client) Fetch(ctx context.Context) (map[string]interface{}, *bool, error) {
	if c.cfg.AgentID == "" {
		return nil, nil, fmt.Errorf("agent ID is required to fetch remote config")
	}

	hostID := c.cfg.AgentID
	if resolved, err := c.resolveHostID(ctx); err != nil {
		return nil, nil, err
	} else if resolved != "" {
		hostID = resolved
	}

	url := fmt.Sprintf("%s/api/agents/host/%s/config", c.cfg.PulseURL, hostID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("X-API-Token", c.cfg.APIToken)
	req.Header.Set("User-Agent", "pulse-agent-config-client")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("server responded with status %s", resp.Status)
	}

	var configResp Response
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		return nil, nil, fmt.Errorf("decode response: %w", err)
	}

	return configResp.Config.Settings, configResp.Config.CommandsEnabled, nil
}

func (c *Client) resolveHostID(ctx context.Context) (string, error) {
	hostname := strings.TrimSpace(c.cfg.Hostname)
	if hostname == "" {
		return "", nil
	}

	url := fmt.Sprintf("%s/api/agents/host/lookup?hostname=%s", c.cfg.PulseURL, hostname)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create host lookup request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("X-API-Token", c.cfg.APIToken)
	req.Header.Set("User-Agent", "pulse-agent-config-client")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("host lookup request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("host lookup responded with status %s", resp.Status)
	}

	var payload struct {
		Success bool `json:"success"`
		Host    struct {
			ID string `json:"id"`
		} `json:"host"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode host lookup response: %w", err)
	}
	if !payload.Success {
		return "", nil
	}
	return strings.TrimSpace(payload.Host.ID), nil
}
