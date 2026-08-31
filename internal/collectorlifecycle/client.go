package collectorlifecycle

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agenttls"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/securityutil"
)

const (
	defaultRequestTimeout = 15 * time.Second
	maximumResponseBytes  = 64 << 10
	maximumBearerBytes    = 4 << 10
)

var (
	// ErrRegistrationPending means the server has not yet published a fresh
	// registration for the requested collector identity. Installers may retry.
	ErrRegistrationPending = errors.New("collector registration is not confirmed")
	// ErrCredentialRejected means retrying cannot repair the collector bearer.
	ErrCredentialRejected = errors.New("collector credential was rejected")
	loadSystemCertPool    = x509.SystemCertPool
)

// Config contains the complete trust and credential inputs for the narrow
// installer lifecycle client. The bearer is deliberately accepted only as a
// file path so it never needs to appear in process arguments or curl config.
type Config struct {
	PulseURL          string
	TokenFile         string
	CACertPath        string
	ServerFingerprint string
	// TokenOwnerUID permits the one dedicated collector account that may own a
	// migration-era runtime.token. Root is always trusted; no other UID is.
	TokenOwnerUID *uint64
	Timeout       time.Duration
}

// Client can only reduce the current collector's authority and inspect its
// authoritative registration. It deliberately exposes no general request API.
type Client struct {
	baseURL *url.URL
	bearer  string
	http    *http.Client
}

// Registration is the bounded server registration evidence consumed by the
// safe-profile transaction.
type Registration struct {
	AgentID  string
	Hostname string
	LastSeen time.Time
}

// New validates the destination before reading the bearer and constructs a
// redirect-denying, system-CA/custom-CA/exact-leaf-pin-aware HTTP client.
func New(config Config) (*Client, error) {
	baseURL, err := securityutil.NormalizePulseHTTPBaseURL(config.PulseURL)
	if err != nil {
		return nil, fmt.Errorf("validate collector lifecycle URL: %w", err)
	}
	if baseURL.Scheme == "http" && !exactLifecycleLoopbackHost(baseURL.Hostname()) {
		return nil, errors.New("collector lifecycle plaintext HTTP is allowed only for localhost, 127.0.0.1, or ::1")
	}
	if baseURL.Scheme == "http" && (strings.TrimSpace(config.CACertPath) != "" || strings.TrimSpace(config.ServerFingerprint) != "") {
		return nil, errors.New("collector lifecycle TLS trust options require an HTTPS URL")
	}
	bearer, err := readPrivateBearer(config.TokenFile, config.TokenOwnerUID)
	if err != nil {
		return nil, err
	}
	tlsConfig, err := agenttls.NewClientTLSConfig(config.CACertPath, false, config.ServerFingerprint)
	if err != nil {
		return nil, fmt.Errorf("configure collector lifecycle TLS: %w", err)
	}
	if baseURL.Scheme == "https" && strings.TrimSpace(config.CACertPath) == "" && strings.TrimSpace(config.ServerFingerprint) == "" {
		roots, err := loadSystemCertPool()
		if err != nil || roots == nil {
			return nil, fmt.Errorf("load system certificate authorities: %w", err)
		}
		tlsConfig.RootCAs = roots
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	return &Client{
		baseURL: baseURL,
		bearer:  bearer,
		http: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				return fmt.Errorf("collector lifecycle server returned redirect to %s; use the final Pulse URL explicitly", req.URL)
			},
		},
	}, nil
}

func exactLifecycleLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Close releases idle transport connections held by the lifecycle client.
func (c *Client) Close() {
	if c != nil && c.http != nil {
		c.http.CloseIdleConnections()
	}
}

// ReduceAuthority durably removes execution and cross-host management scopes
// from the exact bearer-bound collector. Only HTTP 204 is success authority.
func (c *Client) ReduceAuthority(ctx context.Context, agentID, hostname string) error {
	agentID = strings.TrimSpace(agentID)
	hostname = strings.TrimSpace(hostname)
	if !validBoundedIdentity(agentID, 256) || !validBoundedIdentity(hostname, 253) {
		return errors.New("collector authority reduction requires a valid agent identity and hostname")
	}
	body, err := json.Marshal(map[string]string{"agentId": agentID, "hostname": hostname})
	if err != nil {
		return err
	}
	response, err := c.do(ctx, http.MethodPost, "/api/agents/collector/reduce-authority", bytes.NewReader(body), "application/json")
	if err != nil {
		return fmt.Errorf("reduce collector authority: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maximumResponseBytes))
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("reduce collector authority: server returned %s", response.Status)
	}
	return nil
}

// VerifyRegistration asks the authenticated server for the exact collector
// registration. previousLastSeen, when non-zero, requires a later observation
// so a stopped predecessor row cannot authorize a safe-profile commit.
func (c *Client) VerifyRegistration(ctx context.Context, agentID, hostname string, previousLastSeen time.Time) (Registration, error) {
	agentID = strings.TrimSpace(agentID)
	hostname = strings.TrimSpace(hostname)
	query := url.Values{}
	switch {
	case validBoundedIdentity(agentID, 256):
		query.Set("id", agentID)
	case validBoundedIdentity(hostname, 253):
		query.Set("hostname", hostname)
	default:
		return Registration{}, errors.New("collector registration verification requires a valid agent identity or hostname")
	}
	response, err := c.do(ctx, http.MethodGet, "/api/agents/agent/lookup?"+query.Encode(), nil, "")
	if err != nil {
		return Registration{}, fmt.Errorf("verify collector registration: %w", err)
	}
	defer response.Body.Close()

	limited := io.LimitReader(response.Body, maximumResponseBytes+1)
	encoded, readErr := io.ReadAll(limited)
	if readErr != nil {
		return Registration{}, fmt.Errorf("read collector registration response: %w", readErr)
	}
	if len(encoded) > maximumResponseBytes {
		return Registration{}, errors.New("collector registration response exceeds the size limit")
	}
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return Registration{}, fmt.Errorf("%w: server returned %s", ErrCredentialRejected, response.Status)
	case http.StatusForbidden:
		var failure struct {
			Code  string `json:"code"`
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(encoded, &failure) == nil && (failure.Code == "agent_lookup_forbidden" || failure.Error.Code == "agent_lookup_forbidden") {
			return Registration{}, fmt.Errorf("%w: prior credential binding is still visible", ErrRegistrationPending)
		}
		return Registration{}, fmt.Errorf("%w: server returned %s", ErrCredentialRejected, response.Status)
	case http.StatusOK:
		// Continue below.
	default:
		return Registration{}, fmt.Errorf("%w: server returned %s", ErrRegistrationPending, response.Status)
	}

	var payload struct {
		Success bool `json:"success"`
		Agent   struct {
			ID       string    `json:"id"`
			Hostname string    `json:"hostname"`
			LastSeen time.Time `json:"lastSeen"`
		} `json:"agent"`
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	if err := decoder.Decode(&payload); err != nil {
		return Registration{}, fmt.Errorf("%w: invalid server response", ErrRegistrationPending)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Registration{}, fmt.Errorf("%w: invalid trailing server response", ErrRegistrationPending)
	}
	payload.Agent.ID = strings.TrimSpace(payload.Agent.ID)
	payload.Agent.Hostname = strings.TrimSpace(payload.Agent.Hostname)
	if !payload.Success || !validBoundedIdentity(payload.Agent.ID, 256) || !validBoundedIdentity(payload.Agent.Hostname, 253) || payload.Agent.LastSeen.IsZero() {
		return Registration{}, fmt.Errorf("%w: incomplete server response", ErrRegistrationPending)
	}
	if agentID != "" && payload.Agent.ID != agentID {
		return Registration{}, fmt.Errorf("%w: server returned a different agent identity", ErrRegistrationPending)
	}
	if hostname != "" && !strings.EqualFold(payload.Agent.Hostname, hostname) && !unifiedresources.HostnamesEquivalent(payload.Agent.Hostname, hostname) {
		return Registration{}, fmt.Errorf("%w: server returned a different collector hostname", ErrRegistrationPending)
	}
	lastSeen := payload.Agent.LastSeen.UTC()
	if !previousLastSeen.IsZero() && !lastSeen.After(previousLastSeen.UTC()) {
		return Registration{}, fmt.Errorf("%w: registration freshness did not advance", ErrRegistrationPending)
	}
	return Registration{AgentID: payload.Agent.ID, Hostname: payload.Agent.Hostname, LastSeen: lastSeen}, nil
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Response, error) {
	if c == nil || c.baseURL == nil || c.http == nil || c.bearer == "" {
		return nil, errors.New("collector lifecycle client is not initialized")
	}
	target := strings.TrimRight(c.baseURL.String(), "/") + path
	request, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+c.bearer)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return c.http.Do(request)
}

func readPrivateBearer(path string, tokenOwnerUID *uint64) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("collector lifecycle token file is required")
	}
	file, err := openCredentialFile(path)
	if err != nil {
		return "", fmt.Errorf("open collector lifecycle token file: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect collector lifecycle token file descriptor: %w", err)
	}
	// Apply the platform-native privacy boundary before validating the owner.
	// Unix uses mode bits; Windows enforces the equivalent protected DACL below.
	if !info.Mode().IsRegular() || !credentialFileModePrivate(info) {
		return "", errors.New("collector lifecycle token file must be a private regular file")
	}
	if err := validateCredentialFileOwner(path, info, tokenOwnerUID); err != nil {
		return "", err
	}
	encoded, err := io.ReadAll(io.LimitReader(file, maximumBearerBytes+1))
	if err != nil {
		return "", fmt.Errorf("read collector lifecycle token file: %w", err)
	}
	if len(encoded) > maximumBearerBytes {
		return "", errors.New("collector lifecycle token file exceeds the size limit")
	}
	bearer := strings.TrimSpace(string(encoded))
	if bearer == "" || strings.ContainsAny(bearer, "\r\n") {
		return "", errors.New("collector lifecycle token file is empty or invalid")
	}
	return bearer, nil
}

// ReadPrivateValueFile reads one bounded private local value through the same
// single-open, no-follow, nonblocking descriptor boundary used for collector
// credentials. Root is always an allowed owner; tokenOwnerUID may name the
// one dedicated collector account for migration-era identity state.
func ReadPrivateValueFile(path string, tokenOwnerUID *uint64) (string, error) {
	return readPrivateBearer(path, tokenOwnerUID)
}

// ReadAgentIDFile resolves the identity that binds a separate action runner
// while retaining the same single-open, no-follow owner and size checks used
// for the collector credential.
func ReadAgentIDFile(path string, tokenOwnerUID *uint64) (string, error) {
	identity, err := ReadPrivateValueFile(path, tokenOwnerUID)
	if err != nil {
		return "", fmt.Errorf("read collector agent identity: %w", err)
	}
	if !validBoundedIdentity(identity, 128) {
		return "", errors.New("collector agent identity file is invalid")
	}
	return identity, nil
}

func validBoundedIdentity(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for index, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			continue
		}
		if index > 0 && (character == '.' || character == '_' || character == ':' || character == '-') {
			continue
		}
		return false
	}
	return true
}
