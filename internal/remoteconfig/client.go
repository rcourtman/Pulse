package remoteconfig

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
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
	configErr  error
}

// Response represents the JSON response from the config endpoint.
type Response struct {
	Success bool   `json:"success"`
	HostID  string `json:"hostId"`
	Config  struct {
		CommandsEnabled *bool                  `json:"commandsEnabled,omitempty"`
		Settings        map[string]interface{} `json:"settings,omitempty"`
		IssuedAt        time.Time              `json:"issuedAt,omitempty"`
		ExpiresAt       time.Time              `json:"expiresAt,omitempty"`
		Signature       string                 `json:"signature,omitempty"`
	} `json:"config"`
}

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
	hostID := c.cfg.AgentID
	if resolved, err := c.resolveHostID(ctx); err != nil {
		logger.Warn().
			Err(err).
			Str("action", "resolve_host_id_failed").
			Str("hostname", c.cfg.Hostname).
			Str("requested_host_id", c.cfg.AgentID).
			Msg("Remote config host lookup failed")
		return nil, nil, err
	} else if resolved != "" {
		logger.Debug().
			Str("action", "resolve_host_id_success").
			Str("hostname", c.cfg.Hostname).
			Str("requested_host_id", c.cfg.AgentID).
			Str("resolved_host_id", resolved).
			Msg("Remote config host lookup resolved host ID")
		hostID = resolved
	}

	endpointURL := fmt.Sprintf("%s/api/agents/host/%s/config", c.cfg.PulseURL, url.PathEscape(hostID))
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
			Str("host_id", hostID).
			Msg("Remote config request failed")
		return nil, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		logger.Warn().
			Str("action", "fetch_non_success_status").
			Str("endpoint_url", endpointURL).
			Int("status_code", resp.StatusCode).
			Str("status", resp.Status).
			Str("host_id", hostID).
			Msg("Remote config request returned non-success status")
		return nil, nil, fmt.Errorf("server responded with status %s", resp.Status)
	}

	var configResp Response
	if err := json.NewDecoder(resp.Body).Decode(&configResp); err != nil {
		logger.Warn().
			Err(err).
			Str("action", "decode_response_failed").
			Str("endpoint_url", endpointURL).
			Str("host_id", hostID).
			Msg("Remote config response decode failed")
		return nil, nil, fmt.Errorf("decode response: %w", err)
	}

	if configResp.Config.Signature != "" {
		if configResp.Config.IssuedAt.IsZero() || configResp.Config.ExpiresAt.IsZero() {
			logger.Warn().
				Str("action", "signature_missing_timestamps").
				Str("host_id", hostID).
				Str("response_host_id", configResp.HostID).
				Msg("Remote config signature metadata missing timestamps")
			return nil, nil, fmt.Errorf("config signature missing timestamp metadata")
		}
		now := time.Now().UTC()
		if now.After(configResp.Config.ExpiresAt.Add(2 * time.Minute)) {
			logger.Warn().
				Str("action", "signature_expired").
				Str("host_id", hostID).
				Str("response_host_id", configResp.HostID).
				Time("issued_at", configResp.Config.IssuedAt).
				Time("expires_at", configResp.Config.ExpiresAt).
				Time("observed_at", now).
				Msg("Remote config signature expired")
			return nil, nil, fmt.Errorf("config signature expired")
		}
		if configResp.Config.IssuedAt.After(now.Add(2 * time.Minute)) {
			logger.Warn().
				Str("action", "signature_issued_in_future").
				Str("host_id", hostID).
				Str("response_host_id", configResp.HostID).
				Time("issued_at", configResp.Config.IssuedAt).
				Time("observed_at", now).
				Msg("Remote config signature issued in the future")
			return nil, nil, fmt.Errorf("config signature issued in the future")
		}

		payload := SignedConfigPayload{
			HostID:          configResp.HostID,
			IssuedAt:        configResp.Config.IssuedAt,
			ExpiresAt:       configResp.Config.ExpiresAt,
			CommandsEnabled: configResp.Config.CommandsEnabled,
			Settings:        configResp.Config.Settings,
		}
		if err := VerifyConfigPayloadSignature(payload, configResp.Config.Signature); err != nil {
			logger.Warn().
				Err(err).
				Str("action", "signature_verification_failed").
				Str("host_id", hostID).
				Str("response_host_id", configResp.HostID).
				Msg("Remote config signature verification failed")
			return nil, nil, fmt.Errorf("config signature verification failed: %w", err)
		}
	} else if signatureRequired {
		logger.Warn().
			Str("action", "signature_missing_required").
			Str("host_id", hostID).
			Str("response_host_id", configResp.HostID).
			Bool("signature_required", signatureRequired).
			Msg("Remote config signature required but missing")
		return nil, nil, fmt.Errorf("config signature required but missing")
	} else if len(configResp.Config.Settings) > 0 || configResp.Config.CommandsEnabled != nil {
		logger.Warn().
			Str("action", "missing_signature_skip_verification").
			Str("host_id", hostID).
			Str("response_host_id", configResp.HostID).
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

func (c *Client) resolveHostID(ctx context.Context) (string, error) {
	if c.configErr != nil {
		return "", fmt.Errorf("invalid remote config client configuration: %w", c.configErr)
	}
	if c.cfg.APIToken == "" {
		return "", fmt.Errorf("API token is required for host lookup")
	}

	hostname := c.cfg.Hostname
	if hostname == "" {
		return "", nil
	}
	logger := c.logger()

	endpointURL := fmt.Sprintf("%s/api/agents/host/lookup?hostname=%s", c.cfg.PulseURL, url.QueryEscape(hostname))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("action", "host_lookup_request_build_failed").
			Str("endpoint_url", endpointURL).
			Str("hostname", hostname).
			Msg("Remote config host lookup request creation failed")
		return "", fmt.Errorf("create host lookup request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.cfg.APIToken)
	req.Header.Set("X-API-Token", c.cfg.APIToken)
	req.Header.Set("User-Agent", "pulse-agent-config-client")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Warn().
			Err(err).
			Str("action", "host_lookup_request_failed").
			Str("endpoint_url", endpointURL).
			Str("hostname", hostname).
			Msg("Remote config host lookup request failed")
		return "", fmt.Errorf("host lookup request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		logger.Debug().
			Str("action", "host_lookup_not_found").
			Str("hostname", hostname).
			Msg("Remote config host lookup returned not found")
		return "", nil
	}
	if resp.StatusCode >= 300 {
		logger.Warn().
			Str("action", "host_lookup_non_success_status").
			Str("endpoint_url", endpointURL).
			Str("hostname", hostname).
			Int("status_code", resp.StatusCode).
			Str("status", resp.Status).
			Msg("Remote config host lookup returned non-success status")
		return "", fmt.Errorf("host lookup responded with status %s", resp.Status)
	}

	var payload struct {
		Success bool `json:"success"`
		Host    struct {
			ID string `json:"id"`
		} `json:"host"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		logger.Warn().
			Err(err).
			Str("action", "host_lookup_decode_failed").
			Str("endpoint_url", endpointURL).
			Str("hostname", hostname).
			Msg("Remote config host lookup response decode failed")
		return "", fmt.Errorf("decode host lookup response: %w", err)
	}
	if !payload.Success {
		logger.Debug().
			Str("action", "host_lookup_unsuccessful").
			Str("hostname", hostname).
			Msg("Remote config host lookup returned unsuccessful response")
		return "", nil
	}
	return strings.TrimSpace(payload.Host.ID), nil
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
