package vmware

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	// FeatureVMware allows explicit opt-out of the default-on VMware vCenter
	// platform integration.
	FeatureVMware  = "PULSE_ENABLE_VMWARE"
	defaultTimeout = 10 * time.Second
)

var featureVMwareEnabled atomic.Bool

func init() {
	featureVMwareEnabled.Store(parseFeatureEnabled(os.Getenv(FeatureVMware)))
}

// IsFeatureEnabled returns whether the VMware vCenter integration is enabled.
func IsFeatureEnabled() bool {
	return featureVMwareEnabled.Load()
}

// SetFeatureEnabled allows tests to control the feature flag.
func SetFeatureEnabled(enabled bool) {
	featureVMwareEnabled.Store(enabled)
}

func parseFeatureEnabled(raw string) bool {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "", "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// ConnectionError classifies VMware connection failures for API consumers.
type ConnectionError struct {
	Category string
	Message  string
}

func (e *ConnectionError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// InventorySummary captures the minimum read-side floor proven by a successful
// VMware connection test.
type InventorySummary struct {
	Hosts      int
	VMs        int
	Datastores int
	VIRelease  string
}

// ClientConfig configures a VMware vCenter client.
type ClientConfig struct {
	Host               string
	Port               int
	Username           string
	Password           string
	InsecureSkipVerify bool
	Timeout            time.Duration
}

// Client executes phase-1 VMware connection validation.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	username   string
	password   string
}

// NewClient constructs a VMware client from saved connection input.
func NewClient(cfg ClientConfig) (*Client, error) {
	baseURL, err := normalizeBaseURL(cfg.Host, cfg.Port)
	if err != nil {
		return nil, &ConnectionError{Category: "invalid_config", Message: err.Error()}
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify, // operator-controlled onboarding setting
		},
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			Jar:       jar,
		},
		username: strings.TrimSpace(cfg.Username),
		password: strings.TrimSpace(cfg.Password),
	}, nil
}

// Close releases idle resources held by the underlying HTTP client.
func (c *Client) Close() {
	if c == nil || c.httpClient == nil {
		return
	}
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// TestConnection validates both the Automation API and VI JSON API families and
// returns a minimal inventory summary on success.
func (c *Client) TestConnection(ctx context.Context) (*InventorySummary, error) {
	automationSessionID, err := c.createAutomationSession(ctx)
	if err != nil {
		return nil, err
	}
	hosts, err := c.listAutomationResources(ctx, automationSessionID, "/api/vcenter/host", "host inventory")
	if err != nil {
		return nil, err
	}
	vms, err := c.listAutomationResources(ctx, automationSessionID, "/api/vcenter/vm", "vm inventory")
	if err != nil {
		return nil, err
	}
	datastores, err := c.listAutomationResources(ctx, automationSessionID, "/api/vcenter/datastore", "datastore inventory")
	if err != nil {
		return nil, err
	}
	release, sessionManagerMoID, err := c.resolveVIJSONRelease(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.loginVIJSON(ctx, release, sessionManagerMoID); err != nil {
		return nil, err
	}
	return &InventorySummary{
		Hosts:      len(hosts),
		VMs:        len(vms),
		Datastores: len(datastores),
		VIRelease:  release,
	}, nil
}

func normalizeBaseURL(rawHost string, port int) (*url.URL, error) {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return nil, fmt.Errorf("vmware vcenter host is required")
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid vmware vcenter host: %w", err)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("vmware vcenter connections must use https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("vmware vcenter host is required")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, fmt.Errorf("vmware vcenter host must not include a path")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("vmware vcenter host must not include query or fragment data")
	}
	if parsed.Port() == "" {
		if port <= 0 {
			port = 443
		}
		parsed.Host = net.JoinHostPort(parsed.Hostname(), strconv.Itoa(port))
	}
	parsed.Path = ""
	return parsed, nil
}

func (c *Client) createAutomationSession(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL.String()+"/api/session", nil)
	if err != nil {
		return "", fmt.Errorf("build automation session request: %w", err)
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", classifyTransportError("automation session", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return "", fmt.Errorf("read automation session response: %w", readErr)
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return "", &ConnectionError{Category: "auth", Message: "VMware authentication failed while creating the Automation API session"}
		}
		return "", &ConnectionError{
			Category: "endpoint",
			Message:  fmt.Sprintf("VMware Automation API session request failed with HTTP %d", resp.StatusCode),
		}
	}
	var sessionID string
	if err := json.Unmarshal(body, &sessionID); err != nil || strings.TrimSpace(sessionID) == "" {
		return "", &ConnectionError{Category: "endpoint", Message: "VMware Automation API returned an invalid session payload"}
	}
	return strings.TrimSpace(sessionID), nil
}

func (c *Client) listAutomationResources(
	ctx context.Context,
	sessionID string,
	path string,
	label string,
) ([]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL.String()+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", label, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("vmware-api-session-id", sessionID)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, classifyTransportError(label, err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return nil, fmt.Errorf("read %s response: %w", label, readErr)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, &ConnectionError{Category: "auth", Message: fmt.Sprintf("VMware authentication failed while reading %s", label)}
	case http.StatusForbidden:
		return nil, &ConnectionError{Category: "permission", Message: fmt.Sprintf("VMware permissions are insufficient for %s", label)}
	default:
		return nil, &ConnectionError{
			Category: "endpoint",
			Message:  fmt.Sprintf("VMware %s request failed with HTTP %d", label, resp.StatusCode),
		}
	}
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, &ConnectionError{Category: "endpoint", Message: fmt.Sprintf("VMware %s response was not valid JSON", label)}
	}
	return items, nil
}

func (c *Client) resolveVIJSONRelease(ctx context.Context) (string, string, error) {
	releases := []string{"9.0.0.0", "8.0.3", "8.0.2.0", "8.0.1.0"}
	var lastErr error
	for _, release := range releases {
		moID, err := c.fetchSessionManagerMoID(ctx, release)
		if err == nil {
			return release, moID, nil
		}
		lastErr = err
		connectionErr, ok := err.(*ConnectionError)
		if !ok || connectionErr.Category != "endpoint" {
			return "", "", err
		}
	}
	if lastErr != nil {
		return "", "", lastErr
	}
	return "", "", &ConnectionError{Category: "endpoint", Message: "VMware VI JSON API release negotiation failed"}
}

func (c *Client) fetchSessionManagerMoID(ctx context.Context, release string) (string, error) {
	endpoint := fmt.Sprintf("%s/sdk/vim25/%s/ServiceInstance/ServiceInstance/content", c.baseURL.String(), release)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build vi-json service instance request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", classifyTransportError("vi-json service content", err)
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return "", fmt.Errorf("read vi-json service content response: %w", readErr)
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", &ConnectionError{Category: "endpoint", Message: fmt.Sprintf("VMware VI JSON API release %s is unavailable", release)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &ConnectionError{Category: "endpoint", Message: fmt.Sprintf("VMware VI JSON API service-instance request failed with HTTP %d", resp.StatusCode)}
	}
	var payload struct {
		SessionManager struct {
			Value string `json:"value"`
		} `json:"sessionManager"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", &ConnectionError{Category: "endpoint", Message: "VMware VI JSON API service-instance response was not valid JSON"}
	}
	moID := strings.TrimSpace(payload.SessionManager.Value)
	if moID == "" {
		return "", &ConnectionError{Category: "endpoint", Message: "VMware VI JSON API service-instance response did not include a session manager reference"}
	}
	return moID, nil
}

func (c *Client) loginVIJSON(ctx context.Context, release string, sessionManagerMoID string) error {
	body, err := json.Marshal(map[string]string{
		"userName": c.username,
		"password": c.password,
	})
	if err != nil {
		return fmt.Errorf("marshal vi-json login request: %w", err)
	}
	endpoint := fmt.Sprintf("%s/sdk/vim25/%s/SessionManager/%s/Login", c.baseURL.String(), release, sessionManagerMoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("build vi-json login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return classifyTransportError("vi-json login", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	switch resp.StatusCode {
	case http.StatusOK:
		if strings.TrimSpace(resp.Header.Get("vmware-api-session-id")) == "" {
			return &ConnectionError{Category: "endpoint", Message: "VMware VI JSON API login succeeded without returning a session id"}
		}
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ConnectionError{Category: "auth", Message: "VMware authentication failed while creating the VI JSON API session"}
	default:
		return &ConnectionError{Category: "endpoint", Message: fmt.Sprintf("VMware VI JSON API login failed with HTTP %d", resp.StatusCode)}
	}
}

func classifyTransportError(stage string, err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "x509") || strings.Contains(lower, "certificate") || strings.Contains(lower, "tls") {
		return &ConnectionError{Category: "tls", Message: fmt.Sprintf("VMware TLS validation failed during %s", stage)}
	}
	var unknownAuthority *x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return &ConnectionError{Category: "tls", Message: fmt.Sprintf("VMware TLS validation failed during %s", stage)}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return &ConnectionError{Category: "network", Message: fmt.Sprintf("VMware network error during %s", stage)}
	}
	return &ConnectionError{Category: "network", Message: fmt.Sprintf("VMware connection failed during %s", stage)}
}
