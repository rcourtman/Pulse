package remoteconfig

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog"
)

const (
	maxConfigResponseBodyBytes      int64 = 1 * 1024 * 1024
	maxAgentLookupResponseBodyBytes int64 = 64 * 1024
	agentConfigPathFormat                 = "/api/agents/agent/%s/config"
	agentLookupPath                       = "/api/agents/agent/lookup"
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
	configErr  error
}

// Close releases any idle HTTP connections held by the client transport.
func (c *Client) Close() {
	if c == nil || c.httpClient == nil {
		return
	}
	c.httpClient.CloseIdleConnections()
}

// Response represents the JSON response from the config endpoint.
type Response struct {
	Success bool   `json:"success"`
	AgentID string `json:"agentId"`
	Config  struct {
		CommandsEnabled *bool                  `json:"commandsEnabled,omitempty"`
		Settings        map[string]interface{} `json:"settings,omitempty"`
		IssuedAt        time.Time              `json:"issuedAt,omitempty"`
		ExpiresAt       time.Time              `json:"expiresAt,omitempty"`
		Signature       string                 `json:"signature,omitempty"`
	} `json:"config"`
}

const maxHTTPErrorBodyBytes = 4096

// New creates a new remote config client.
func New(cfg Config) *Client {
	cfg, cfgErr := normalizeConfig(cfg)

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
		configErr:  cfgErr,
	}
}

// Fetch retrieves the remote configuration for this agent.
// It returns a map of settings to apply, or an error if the fetch fails.
// Returns (settings, commandsEnabled, error)
func (c *Client) Fetch(ctx context.Context) (map[string]interface{}, *bool, error) {
	if c.configErr != nil {
		return nil, nil, fmt.Errorf("invalid remote config client configuration: %w", c.configErr)
	}
	if c.cfg.APIToken == "" {
		return nil, nil, fmt.Errorf("API token is required to fetch remote config")
	}
	if c.cfg.AgentID == "" {
		return nil, nil, fmt.Errorf("agent ID is required to fetch remote config")
	}

	logger := c.logger()
	signatureRequired := isConfigSignatureRequired()
	agentID := c.cfg.AgentID
	if resolved, err := c.resolveAgentID(ctx); err != nil {
		return nil, nil, fmt.Errorf("resolve agent ID: %w", err)
	} else if resolved != "" {
		logger.Debug().
			Str("action", "resolve_agent_id_success").
			Str("hostname", c.cfg.Hostname).
			Str("requested_agent_id", c.cfg.AgentID).
			Str("resolved_agent_id", resolved).
			Msg("Remote config agent lookup resolved agent ID")
		agentID = resolved
	}

	endpointURL := c.cfg.PulseURL + fmt.Sprintf(agentConfigPathFormat, url.PathEscape(agentID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("X-API-Token", c.cfg.APIToken)
	req.Header.Set("User-Agent", "pulse-agent-config-client")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("action", "fetch_request_failed").
			Str("endpoint_url", endpointURL).
			Str("agent_id", agentID).
			Msg("Remote config request failed")
		return nil, nil, fmt.Errorf("do request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.cfg.Logger.Warn().Err(closeErr).Msg("Failed to close config response body")
		}
	}()

	if resp.StatusCode >= 300 {
		logger.Warn().
			Str("action", "fetch_non_success_status").
			Str("endpoint_url", endpointURL).
			Int("status_code", resp.StatusCode).
			Str("status", resp.Status).
			Str("agent_id", agentID).
			Msg("Remote config request returned non-success status")
		return nil, nil, fmt.Errorf("server responded with status %s", resp.Status)
	}

	var configResp Response
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxConfigResponseBodyBytes)).Decode(&configResp); err != nil {
		logger.Warn().
			Err(err).
			Str("action", "decode_response_failed").
			Str("endpoint_url", endpointURL).
			Str("agent_id", agentID).
			Msg("Remote config response decode failed")
		return nil, nil, fmt.Errorf("decode response: %w", err)
	}

	responseAgentID := strings.TrimSpace(configResp.AgentID)

	if configResp.Config.Signature != "" {
		if responseAgentID == "" {
			return nil, nil, fmt.Errorf("config signature missing agent metadata")
		}
		if responseAgentID != agentID {
			return nil, nil, fmt.Errorf("config signature agent mismatch: expected %q, got %q", agentID, responseAgentID)
		}
		if configResp.Config.IssuedAt.IsZero() || configResp.Config.ExpiresAt.IsZero() {
			logger.Warn().
				Str("action", "signature_missing_timestamps").
				Str("agent_id", agentID).
				Str("response_agent_id", responseAgentID).
				Msg("Remote config signature metadata missing timestamps")
			return nil, nil, fmt.Errorf("config signature missing timestamp metadata")
		}
		now := time.Now().UTC()
		if now.After(configResp.Config.ExpiresAt.Add(2 * time.Minute)) {
			logger.Warn().
				Str("action", "signature_expired").
				Str("agent_id", agentID).
				Str("response_agent_id", responseAgentID).
				Time("issued_at", configResp.Config.IssuedAt).
				Time("expires_at", configResp.Config.ExpiresAt).
				Time("observed_at", now).
				Msg("Remote config signature expired")
			return nil, nil, fmt.Errorf("config signature expired")
		}
		if configResp.Config.IssuedAt.After(now.Add(2 * time.Minute)) {
			logger.Warn().
				Str("action", "signature_issued_in_future").
				Str("agent_id", agentID).
				Str("response_agent_id", responseAgentID).
				Time("issued_at", configResp.Config.IssuedAt).
				Time("observed_at", now).
				Msg("Remote config signature issued in the future")
			return nil, nil, fmt.Errorf("config signature issued in the future")
		}

		payload := SignedConfigPayload{
			AgentID:         responseAgentID,
			IssuedAt:        configResp.Config.IssuedAt,
			ExpiresAt:       configResp.Config.ExpiresAt,
			CommandsEnabled: configResp.Config.CommandsEnabled,
			Settings:        configResp.Config.Settings,
		}
		if err := VerifyConfigPayloadSignature(payload, configResp.Config.Signature); err != nil {
			logger.Warn().
				Err(err).
				Str("action", "signature_verification_failed").
				Str("agent_id", agentID).
				Str("response_agent_id", responseAgentID).
				Msg("Remote config signature verification failed")
			return nil, nil, fmt.Errorf("config signature verification failed: %w", err)
		}
	} else if signatureRequired {
		logger.Warn().
			Str("action", "signature_missing_required").
			Str("agent_id", agentID).
			Str("response_agent_id", responseAgentID).
			Bool("signature_required", signatureRequired).
			Msg("Remote config signature required but missing")
		return nil, nil, fmt.Errorf("config signature required but missing")
	} else if len(configResp.Config.Settings) > 0 || configResp.Config.CommandsEnabled != nil {
		logger.Warn().
			Str("action", "missing_signature_skip_verification").
			Str("agent_id", agentID).
			Str("response_agent_id", responseAgentID).
			Int("settings_count", len(configResp.Config.Settings)).
			Bool("commands_enabled_present", configResp.Config.CommandsEnabled != nil).
			Bool("signature_required", signatureRequired).
			Msg("Remote config response missing signature - skipping verification")
	}

	return configResp.Config.Settings, configResp.Config.CommandsEnabled, nil
}

func isConfigSignatureRequired() bool {
	return utils.ParseBool(utils.GetenvTrim("PULSE_AGENT_CONFIG_SIGNATURE_REQUIRED"))
}

func (c *Client) logger() zerolog.Logger {
	return c.cfg.Logger.With().Str("component", "remote_config_client").Logger()
}

func (c *Client) resolveAgentID(ctx context.Context) (string, error) {
	if c.configErr != nil {
		return "", fmt.Errorf("invalid remote config client configuration: %w", c.configErr)
	}
	if c.cfg.APIToken == "" {
		return "", fmt.Errorf("API token is required for agent lookup")
	}

	hostname := c.cfg.Hostname
	if hostname == "" {
		return "", nil
	}
	logger := c.logger()

	endpointURL := fmt.Sprintf("%s%s?hostname=%s", c.cfg.PulseURL, agentLookupPath, url.QueryEscape(hostname))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("action", "agent_lookup_request_build_failed").
			Str("endpoint_url", endpointURL).
			Str("hostname", hostname).
			Msg("Remote config agent lookup request creation failed")
		return "", fmt.Errorf("create agent lookup request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("X-API-Token", c.cfg.APIToken)
	req.Header.Set("User-Agent", "pulse-agent-config-client")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("action", "agent_lookup_request_failed").
			Str("endpoint_url", endpointURL).
			Str("hostname", hostname).
			Msg("Remote config agent lookup request failed")
		return "", fmt.Errorf("agent lookup request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.cfg.Logger.Warn().Err(closeErr).Msg("Failed to close agent lookup response body")
		}
	}()

	if resp.StatusCode == http.StatusNotFound {
		logger.Debug().
			Str("action", "agent_lookup_not_found").
			Str("hostname", hostname).
			Msg("Remote config agent lookup returned not found")
		return "", nil
	}
	if resp.StatusCode >= 300 {
		logger.Warn().
			Str("action", "agent_lookup_non_success_status").
			Str("endpoint_url", endpointURL).
			Str("hostname", hostname).
			Int("status_code", resp.StatusCode).
			Str("status", resp.Status).
			Msg("Remote config agent lookup returned non-success status")
		return "", fmt.Errorf("agent lookup responded with status %s", resp.Status)
	}

	var payload struct {
		Success bool `json:"success"`
		Agent   struct {
			ID string `json:"id"`
		} `json:"agent"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxAgentLookupResponseBodyBytes)).Decode(&payload); err != nil {
		logger.Warn().
			Err(err).
			Str("action", "agent_lookup_decode_failed").
			Str("endpoint_url", endpointURL).
			Str("hostname", hostname).
			Msg("Remote config agent lookup response decode failed")
		return "", fmt.Errorf("decode agent lookup response: %w", err)
	}
	if !payload.Success {
		logger.Debug().
			Str("action", "agent_lookup_unsuccessful").
			Str("hostname", hostname).
			Msg("Remote config agent lookup returned unsuccessful response")
		return "", nil
	}

	return strings.TrimSpace(payload.Agent.ID), nil
}

func normalizeConfig(cfg Config) (Config, error) {
	cfg.PulseURL = strings.TrimSpace(cfg.PulseURL)
	if cfg.PulseURL == "" {
		cfg.PulseURL = "http://localhost:7655"
	}
	cfg.APIToken = strings.TrimSpace(cfg.APIToken)
	cfg.AgentID = strings.TrimSpace(cfg.AgentID)
	cfg.Hostname = strings.TrimSpace(cfg.Hostname)

	normalizedPulseURL, err := normalizePulseURL(cfg.PulseURL)
	if err != nil {
		return cfg, err
	}
	cfg.PulseURL = normalizedPulseURL

	return cfg, nil
}

func normalizePulseURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid pulse URL: %w", err)
	}

	switch parsed.Scheme {
	case "http", "https":
	default:
		return "", fmt.Errorf("invalid pulse URL scheme %q: must be http or https", parsed.Scheme)
	}

	if parsed.Hostname() == "" {
		return "", errors.New("invalid pulse URL: missing host")
	}
	if parsed.User != nil {
		return "", errors.New("invalid pulse URL: userinfo is not allowed")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid pulse URL: query and fragment are not allowed")
	}

	if port := parsed.Port(); port != "" {
		portValue, err := strconv.Atoi(port)
		if err != nil || portValue < 1 || portValue > 65535 {
			return "", fmt.Errorf("invalid pulse URL port %q: must be between 1 and 65535", port)
		}
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}
