package proxmox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
	"github.com/rs/zerolog/log"
)

// FlexInt handles JSON fields that can be int, float, or string (for cpulimit support)
type FlexInt int

func (f *FlexInt) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as int first
	var i int
	if err := json.Unmarshal(data, &i); err == nil {
		*f = FlexInt(i)
		return nil
	}

	// Try as float (handles cpulimit like 1.5)
	var fl float64
	if err := json.Unmarshal(data, &fl); err == nil {
		*f = FlexInt(int(fl))
		return nil
	}

	// If that fails, try as string
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Parse string to float first (handles "1.5" format from cpulimit)
	floatVal, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}

	// Convert to int
	*f = FlexInt(int(floatVal))
	return nil
}

func coerceUint64(field string, value interface{}) (uint64, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("invalid float value for %s", field)
		}
		if v <= 0 {
			return 0, nil
		}
		if v >= math.MaxUint64 {
			return math.MaxUint64, nil
		}
		return uint64(math.Round(v)), nil
	case int:
		if v < 0 {
			return 0, nil
		}
		return uint64(v), nil
	case int64:
		if v < 0 {
			return 0, nil
		}
		return uint64(v), nil
	case int32:
		if v < 0 {
			return 0, nil
		}
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case json.Number:
		return coerceUint64(field, string(v))
	case string:
		s := strings.TrimSpace(v)
		if s == "" || strings.EqualFold(s, "null") {
			return 0, nil
		}
		s = strings.Trim(s, "\"'")
		s = strings.TrimSpace(s)
		if s == "" || strings.EqualFold(s, "null") {
			return 0, nil
		}
		s = strings.ReplaceAll(s, ",", "")
		if strings.ContainsAny(s, ".eE") {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse float for %s: %w", field, err)
			}
			return coerceUint64(field, f)
		}
		val, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse uint for %s: %w", field, err)
		}
		return val, nil
	default:
		return 0, fmt.Errorf("unsupported type %T for field %s", value, field)
	}
}

// Client represents a Proxmox VE API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       auth
	config     ClientConfig
}

// ClientConfig holds configuration for the Proxmox client
type ClientConfig struct {
	Host        string
	User        string
	Password    string
	TokenName   string
	TokenValue  string
	Fingerprint string
	VerifySSL   bool
	Timeout     time.Duration
}

// auth represents authentication details
type auth struct {
	user       string
	realm      string
	ticket     string
	csrfToken  string
	tokenName  string
	tokenValue string
	expiresAt  time.Time
}

// NewClient creates a new Proxmox VE API client
func NewClient(cfg ClientConfig) (*Client, error) {
	var user, realm string

	// Log what auth method we're using
	log.Debug().
		Str("host", cfg.Host).
		Bool("hasToken", cfg.TokenName != "").
		Bool("hasPassword", cfg.Password != "").
		Str("tokenName", cfg.TokenName).
		Str("user", cfg.User).
		Msg("Creating Proxmox client")

	// For token authentication, we don't need user@realm format
	if cfg.TokenName != "" && cfg.TokenValue != "" {
		// Extract user and realm from token name (format: user@realm!tokenname)
		if strings.Contains(cfg.TokenName, "@") && strings.Contains(cfg.TokenName, "!") {
			parts := strings.Split(cfg.TokenName, "!")
			if len(parts) == 2 {
				userRealm := parts[0]
				userRealmParts := strings.Split(userRealm, "@")
				if len(userRealmParts) == 2 {
					user = userRealmParts[0]
					realm = userRealmParts[1]
				}
			}
		}
	} else {
		// For password authentication, parse user and realm from User field
		parts := strings.Split(cfg.User, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid user format, expected user@realm")
		}
		user = parts[0]
		realm = parts[1]
	}

	// Create HTTP client with proper TLS configuration
	// Use configured timeout or default to 60 seconds
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	httpClient := tlsutil.CreateHTTPClientWithTimeout(cfg.VerifySSL, cfg.Fingerprint, timeout)

	// Extract just the token name part for API token authentication
	tokenName := cfg.TokenName
	if cfg.TokenName != "" && strings.Contains(cfg.TokenName, "!") {
		parts := strings.Split(cfg.TokenName, "!")
		if len(parts) == 2 {
			tokenName = parts[1] // Just the token name part (e.g., "pulse-token")
		}
	}

	log.Debug().
		Str("user", user).
		Str("realm", realm).
		Bool("hasToken", cfg.TokenValue != "").
		Msg("Proxmox client configured")

	client := &Client{
		baseURL:    strings.TrimSuffix(cfg.Host, "/") + "/api2/json",
		httpClient: httpClient,
		config:     cfg,
		auth: auth{
			user:       user,
			realm:      realm,
			tokenName:  tokenName,
			tokenValue: cfg.TokenValue,
		},
	}

	// Authenticate if using password
	if cfg.Password != "" && cfg.TokenName == "" {
		if err := client.authenticate(context.Background()); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	return client, nil
}

// authenticate performs password-based authentication
func (c *Client) authenticate(ctx context.Context) error {
	username := c.auth.user + "@" + c.auth.realm
	password := c.config.Password

	if err := c.authenticateJSON(ctx, username, password); err == nil {
		return nil
	} else if shouldFallbackToForm(err) {
		return c.authenticateForm(ctx, username, password)
	} else {
		return err
	}
}

func (c *Client) authenticateJSON(ctx context.Context, username, password string) error {
	payload := map[string]string{
		"username": username,
		"password": password,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/access/ticket", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleAuthResponse(resp)
}

func (c *Client) authenticateForm(ctx context.Context, username, password string) error {
	data := url.Values{
		"username": {username},
		"password": {password},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/access/ticket", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleAuthResponse(resp)
}

func (c *Client) handleAuthResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &authHTTPError{status: resp.StatusCode, body: string(body)}
	}

	var result struct {
		Data struct {
			Ticket              string `json:"ticket"`
			CSRFPreventionToken string `json:"CSRFPreventionToken"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	c.auth.ticket = result.Data.Ticket
	c.auth.csrfToken = result.Data.CSRFPreventionToken
	c.auth.expiresAt = time.Now().Add(2 * time.Hour) // PVE tickets expire after 2 hours

	return nil
}

type authHTTPError struct {
	status int
	body   string
}

func (e *authHTTPError) Error() string {
	if e.status == http.StatusUnauthorized || e.status == http.StatusForbidden {
		return fmt.Sprintf("authentication failed (status %d): %s", e.status, e.body)
	}
	return fmt.Sprintf("authentication failed: %s", e.body)
}

func shouldFallbackToForm(err error) bool {
	if authErr, ok := err.(*authHTTPError); ok {
		switch authErr.status {
		case http.StatusBadRequest, http.StatusUnsupportedMediaType:
			return true
		}
	}
	return false
}

// request performs an API request
func (c *Client) request(ctx context.Context, method, path string, data url.Values) (*http.Response, error) {
	// Re-authenticate if needed
	if c.config.Password != "" && c.auth.tokenName == "" && time.Now().After(c.auth.expiresAt) {
		if err := c.authenticate(ctx); err != nil {
			return nil, fmt.Errorf("re-authentication failed: %w", err)
		}
	}

	var body io.Reader
	if data != nil {
		body = strings.NewReader(data.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	// Set headers
	if data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	// Set authentication
	if c.auth.tokenName != "" && c.auth.tokenValue != "" {
		// API token authentication
		authHeader := fmt.Sprintf("PVEAPIToken=%s@%s!%s=%s",
			c.auth.user, c.auth.realm, c.auth.tokenName, c.auth.tokenValue)
		req.Header.Set("Authorization", authHeader)
		// NEVER log the actual token value - only log that we're using token auth
		maskedHeader := fmt.Sprintf("PVEAPIToken=%s@%s!%s=***",
			c.auth.user, c.auth.realm, c.auth.tokenName)
		log.Debug().
			Str("authHeader", maskedHeader).
			Str("url", req.URL.String()).
			Msg("Setting API token authentication")
	} else if c.auth.ticket != "" {
		// Ticket authentication
		req.Header.Set("Cookie", "PVEAuthCookie="+c.auth.ticket)
		if method != "GET" && c.auth.csrfToken != "" {
			req.Header.Set("CSRFPreventionToken", c.auth.csrfToken)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check for errors
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		// Create base error with helpful guidance for common issues
		var err error
		if resp.StatusCode == 403 && c.config.TokenName != "" {
			// Special case for 403 with API token - this is usually a permission issue
			err = fmt.Errorf("API error 403 (Forbidden): The API token does not have sufficient permissions. Note: In Proxmox GUI, permissions must be set on the USER (not just the token). Please verify the user '%s@%s' has the required permissions", c.auth.user, c.auth.realm)
		} else if resp.StatusCode == 595 {
			// 595 can mean authentication failed OR trying to access an offline node in a cluster
			// Check if this is a node-specific endpoint
			if strings.Contains(req.URL.Path, "/nodes/") && strings.Count(req.URL.Path, "/") > 3 {
				// This looks like a node-specific resource request
				err = fmt.Errorf("API error 595: Cannot access node resource - node may be offline or credentials may be invalid")
			} else {
				err = fmt.Errorf("API error 595: Authentication failed - please check your credentials")
			}
		} else if resp.StatusCode == 401 {
			err = fmt.Errorf("API error 401 (Unauthorized): Invalid credentials or token")
		} else {
			err = fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		// Log auth issues for debugging (595 is Proxmox "no ticket" error)
		if resp.StatusCode == 595 || resp.StatusCode == 401 || resp.StatusCode == 403 {
			// Some endpoints are optional and may return 403 if the token is intentionally
			// scoped read-only. Avoid warning-level log spam for those.
			event := log.Warn()
			msg := "Proxmox authentication error"
			if resp.StatusCode == 403 && strings.Contains(req.URL.Path, "/apt/update") {
				event = log.Debug()
				msg = "Proxmox permission error (optional endpoint)"
			}

			event.
				Str("url", req.URL.String()).
				Int("status", resp.StatusCode).
				Bool("hasToken", c.config.TokenName != "").
				Bool("hasPassword", c.config.Password != "").
				Str("tokenName", c.config.TokenName).
				Msg(msg)
		}

		// Wrap with appropriate error type
		if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 595 {
			// Import errors package at top of file
			return nil, fmt.Errorf("authentication error: %w", err)
		}

		return nil, err
	}

	return resp, nil
}

// get performs a GET request
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	return c.request(ctx, "GET", path, nil)
}

// Node represents a Proxmox VE node
type Node struct {
	Node    string  `json:"node"`
	Status  string  `json:"status"`
	CPU     float64 `json:"cpu"`
	MaxCPU  int     `json:"maxcpu"`
	Mem     uint64  `json:"mem"`
	MaxMem  uint64  `json:"maxmem"`
	Disk    uint64  `json:"disk"`
	MaxDisk uint64  `json:"maxdisk"`
	Uptime  uint64  `json:"uptime"`
	Level   string  `json:"level"`
}

// NodeRRDPoint represents a single RRD datapoint for a node.
type NodeRRDPoint struct {
	Time         int64    `json:"time"`
	MemTotal     *float64 `json:"memtotal,omitempty"`
	MemUsed      *float64 `json:"memused,omitempty"`
	MemAvailable *float64 `json:"memavailable,omitempty"`
}

// GuestRRDPoint represents a single RRD datapoint for a VM or LXC container.
type GuestRRDPoint struct {
	Time         int64    `json:"time"`
	MaxMem       *float64 `json:"maxmem,omitempty"`
	MemUsed      *float64 `json:"memused,omitempty"`
	MemAvailable *float64 `json:"memavailable,omitempty"`
}

// NodeStatus represents detailed node status from /nodes/{node}/status endpoint
// This endpoint provides real-time metrics that update every second
type NodeStatus struct {
	CPU           float64       `json:"cpu"`     // Real-time CPU usage (0-1)
	Memory        *MemoryStatus `json:"memory"`  // Real-time memory stats
	Swap          *SwapStatus   `json:"swap"`    // Swap usage
	LoadAvg       []interface{} `json:"loadavg"` // Can be float64 or string
	KernelVersion string        `json:"kversion"`
	PVEVersion    string        `json:"pveversion"`
	CPUInfo       *CPUInfo      `json:"cpuinfo"`
	RootFS        *RootFS       `json:"rootfs"`
	Uptime        uint64        `json:"uptime"`  // Uptime in seconds
	Wait          float64       `json:"wait"`    // IO wait
	IODelay       float64       `json:"iodelay"` // IO delay
	Idle          float64       `json:"idle"`    // CPU idle time
}

// MemoryStatus represents real-time memory information
type MemoryStatus struct {
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Free      uint64 `json:"free"`
	Available uint64 `json:"available"` // Memory available for allocation (excludes non-reclaimable cache)
	Avail     uint64 `json:"avail"`     // Older Proxmox field name for available memory
	Buffers   uint64 `json:"buffers"`   // Reclaimable buffers
	Cached    uint64 `json:"cached"`    // Reclaimable page cache
	Shared    uint64 `json:"shared"`    // Shared memory (informational)
}

func (m *MemoryStatus) UnmarshalJSON(data []byte) error {
	type rawMemoryStatus struct {
		Total     interface{} `json:"total"`
		Used      interface{} `json:"used"`
		Free      interface{} `json:"free"`
		Available interface{} `json:"available"`
		Avail     interface{} `json:"avail"`
		Buffers   interface{} `json:"buffers"`
		Cached    interface{} `json:"cached"`
		Shared    interface{} `json:"shared"`
	}

	var raw rawMemoryStatus
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	total, err := coerceUint64("total", raw.Total)
	if err != nil {
		return err
	}
	used, err := coerceUint64("used", raw.Used)
	if err != nil {
		return err
	}
	free, err := coerceUint64("free", raw.Free)
	if err != nil {
		return err
	}
	available, err := coerceUint64("available", raw.Available)
	if err != nil {
		return err
	}
	avail, err := coerceUint64("avail", raw.Avail)
	if err != nil {
		return err
	}
	buffers, err := coerceUint64("buffers", raw.Buffers)
	if err != nil {
		return err
	}
	cached, err := coerceUint64("cached", raw.Cached)
	if err != nil {
		return err
	}
	shared, err := coerceUint64("shared", raw.Shared)
	if err != nil {
		return err
	}

	*m = MemoryStatus{
		Total:     total,
		Used:      used,
		Free:      free,
		Available: available,
		Avail:     avail,
		Buffers:   buffers,
		Cached:    cached,
		Shared:    shared,
	}
	return nil
}

// EffectiveAvailable returns the best-effort estimate of reclaimable memory.
// Prefer the dedicated "available"/"avail" fields when present, otherwise derive
// from free + buffers + cached which mirrors Linux's MemAvailable calculation.
func (m *MemoryStatus) EffectiveAvailable() uint64 {
	if m == nil {
		return 0
	}

	if m.Available > 0 {
		return m.Available
	}
	if m.Avail > 0 {
		return m.Avail
	}

	derived := m.Free + m.Buffers + m.Cached
	if m.Total > 0 && m.Used > 0 && m.Total >= m.Used {
		availableFromUsed := m.Total - m.Used
		if availableFromUsed > derived {
			derived = availableFromUsed
		}
	}

	if derived == 0 {
		return 0
	}

	// Cap at total to guard against over-reporting when buffers/cached exceed total.
	if m.Total > 0 && derived > m.Total {
		return m.Total
	}

	return derived
}

// SwapStatus represents swap information
type SwapStatus struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	Free  uint64 `json:"free"`
}

// RootFS represents root filesystem information
type RootFS struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
	Free  uint64 `json:"avail"`
}

// CPUInfo represents CPU information
type CPUInfo struct {
	Model   string      `json:"model"`
	Cores   int         `json:"cores"`
	Sockets int         `json:"sockets"`
	MHz     interface{} `json:"mhz"` // Can be string or number
}

// GetMHzString returns MHz as a string
func (c *CPUInfo) GetMHzString() string {
	if c.MHz == nil {
		return ""
	}
	switch v := c.MHz.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// GetNodes returns all nodes in the cluster
func (c *Client) GetNodes(ctx context.Context) ([]Node, error) {
	resp, err := c.get(ctx, "/nodes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Node `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetNodeStatus returns detailed status for a specific node
func (c *Client) GetNodeStatus(ctx context.Context, node string) (*NodeStatus, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/status", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data NodeStatus `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// GetNodeRRDData retrieves RRD metrics for a node.
func (c *Client) GetNodeRRDData(ctx context.Context, node, timeframe, cf string, ds []string) ([]NodeRRDPoint, error) {
	if timeframe == "" {
		timeframe = "hour"
	}
	if cf == "" {
		cf = "AVERAGE"
	}

	params := url.Values{}
	params.Set("timeframe", timeframe)
	params.Set("cf", cf)
	if len(ds) > 0 {
		params.Set("ds", strings.Join(ds, ","))
	}

	path := fmt.Sprintf("/nodes/%s/rrddata", url.PathEscape(node))
	if query := params.Encode(); query != "" {
		path = fmt.Sprintf("%s?%s", path, query)
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []NodeRRDPoint `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetLXCRRDData retrieves RRD metrics for an LXC container.
func (c *Client) GetLXCRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]GuestRRDPoint, error) {
	if timeframe == "" {
		timeframe = "hour"
	}
	if cf == "" {
		cf = "AVERAGE"
	}

	params := url.Values{}
	params.Set("timeframe", timeframe)
	params.Set("cf", cf)
	if len(ds) > 0 {
		params.Set("ds", strings.Join(ds, ","))
	}

	path := fmt.Sprintf("/nodes/%s/lxc/%d/rrddata", url.PathEscape(node), vmid)
	if query := params.Encode(); query != "" {
		path = fmt.Sprintf("%s?%s", path, query)
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []GuestRRDPoint `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetVMRRDData retrieves RRD metrics for a QEMU VM.
func (c *Client) GetVMRRDData(ctx context.Context, node string, vmid int, timeframe, cf string, ds []string) ([]GuestRRDPoint, error) {
	if timeframe == "" {
		timeframe = "hour"
	}
	if cf == "" {
		cf = "AVERAGE"
	}

	params := url.Values{}
	params.Set("timeframe", timeframe)
	params.Set("cf", cf)
	if len(ds) > 0 {
		params.Set("ds", strings.Join(ds, ","))
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%d/rrddata", url.PathEscape(node), vmid)
	if query := params.Encode(); query != "" {
		path = fmt.Sprintf("%s?%s", path, query)
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []GuestRRDPoint `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// VM represents a Proxmox VE virtual machine
type VM struct {
	VMID      int     `json:"vmid"`
	Name      string  `json:"name"`
	Node      string  `json:"node"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	CPUs      int     `json:"cpus"`
	Mem       uint64  `json:"mem"`
	MaxMem    uint64  `json:"maxmem"`
	Disk      uint64  `json:"disk"`
	MaxDisk   uint64  `json:"maxdisk"`
	NetIn     uint64  `json:"netin"`
	NetOut    uint64  `json:"netout"`
	DiskRead  uint64  `json:"diskread"`
	DiskWrite uint64  `json:"diskwrite"`
	Uptime    uint64  `json:"uptime"`
	Template  int     `json:"template"`
	Tags      string  `json:"tags"`
	Lock      string  `json:"lock"`
	Agent     int     `json:"agent"`
}

// Container represents a Proxmox VE LXC container
type Container struct {
	VMID      FlexInt                           `json:"vmid"` // Changed to FlexInt to handle string VMIDs from some Proxmox versions
	Name      string                            `json:"name"`
	Node      string                            `json:"node"`
	Status    string                            `json:"status"`
	CPU       float64                           `json:"cpu"`
	CPUs      FlexInt                           `json:"cpus"`
	Mem       uint64                            `json:"mem"`
	MaxMem    uint64                            `json:"maxmem"`
	Swap      uint64                            `json:"swap"`
	MaxSwap   uint64                            `json:"maxswap"`
	Disk      uint64                            `json:"disk"`
	MaxDisk   uint64                            `json:"maxdisk"`
	NetIn     uint64                            `json:"netin"`
	NetOut    uint64                            `json:"netout"`
	DiskRead  uint64                            `json:"diskread"`
	DiskWrite uint64                            `json:"diskwrite"`
	Uptime    uint64                            `json:"uptime"`
	Template  int                               `json:"template"`
	Tags      string                            `json:"tags"`
	Lock      string                            `json:"lock"`
	Hostname  string                            `json:"hostname,omitempty"`
	IP        string                            `json:"ip,omitempty"`
	IP6       string                            `json:"ip6,omitempty"`
	IPv4      json.RawMessage                   `json:"ipv4,omitempty"`
	IPv6      json.RawMessage                   `json:"ipv6,omitempty"`
	Network   map[string]ContainerNetworkConfig `json:"network,omitempty"`
	DiskInfo  map[string]ContainerDiskUsage     `json:"diskinfo,omitempty"`
	RootFS    string                            `json:"rootfs,omitempty"`
}

// ContainerNetworkConfig captures basic container network status information.
type ContainerNetworkConfig struct {
	Name     string      `json:"name,omitempty"`
	HWAddr   string      `json:"hwaddr,omitempty"`
	Bridge   string      `json:"bridge,omitempty"`
	Method   string      `json:"method,omitempty"`
	Type     string      `json:"type,omitempty"`
	IP       interface{} `json:"ip,omitempty"`
	IP6      interface{} `json:"ip6,omitempty"`
	IPv4     interface{} `json:"ipv4,omitempty"`
	IPv6     interface{} `json:"ipv6,omitempty"`
	Firewall interface{} `json:"firewall,omitempty"`
	Tag      interface{} `json:"tag,omitempty"`
}

// ContainerDiskUsage captures disk usage details returned by the LXC status API.
type ContainerDiskUsage struct {
	Total uint64 `json:"total,omitempty"`
	Used  uint64 `json:"used,omitempty"`
}

// ContainerInterfaceAddress describes an IP entry associated with a container interface.
type ContainerInterfaceAddress struct {
	Address string `json:"ip-address"`
	Type    string `json:"ip-address-type"`
	Prefix  string `json:"prefix"`
}

// ContainerInterface describes a container network interface returned by Proxmox.
type ContainerInterface struct {
	Name        string                      `json:"name"`
	HWAddr      string                      `json:"hwaddr"`
	Inet        string                      `json:"inet,omitempty"`
	IPAddresses []ContainerInterfaceAddress `json:"ip-addresses,omitempty"`
}

// NodeNetworkInterface describes a network interface on a Proxmox node.
type NodeNetworkInterface struct {
	Iface    string `json:"iface"`              // Interface name (e.g., "eth0", "vmbr0")
	Type     string `json:"type"`               // Type (e.g., "eth", "bridge", "bond")
	Address  string `json:"address,omitempty"`  // IPv4 address
	Address6 string `json:"address6,omitempty"` // IPv6 address
	Netmask  string `json:"netmask,omitempty"`  // IPv4 netmask
	CIDR     string `json:"cidr,omitempty"`     // CIDR notation (e.g., "10.1.1.5/24")
	Active   int    `json:"active"`             // 1 if active
}

// GetNodeNetworkInterfaces returns the network interfaces configured on a Proxmox node.
// This can be used to find all IPs available on a node for connection purposes.
func (c *Client) GetNodeNetworkInterfaces(ctx context.Context, node string) ([]NodeNetworkInterface, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/network", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get node network interfaces (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Data []NodeNetworkInterface `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetContainerConfig returns the configuration of a specific container
func (c *Client) GetContainerConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/config", node, vmid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Data == nil {
		result.Data = make(map[string]interface{})
	}

	return result.Data, nil
}

// Storage represents a Proxmox VE storage
type Storage struct {
	Storage   string `json:"storage"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	Active    int    `json:"active"`
	Enabled   int    `json:"enabled"`
	Shared    int    `json:"shared"`
	Nodes     string `json:"nodes,omitempty"`
	Path      string `json:"path,omitempty"`
	Total     uint64 `json:"total"`
	Used      uint64 `json:"used"`
	Available uint64 `json:"avail"`
}

// StorageContent represents content in a storage
type StorageContent struct {
	Volid        string                 `json:"volid"`
	Content      string                 `json:"content"`
	CTime        int64                  `json:"ctime"`
	Format       string                 `json:"format"`
	Size         uint64                 `json:"size"`
	Used         uint64                 `json:"used"`
	VMID         int                    `json:"vmid"`
	Notes        string                 `json:"notes"`
	Protected    int                    `json:"protected"`
	Encryption   string                 `json:"encryption"`
	Verification map[string]interface{} `json:"verification"` // PBS verification info
	Verified     int                    `json:"verified"`     // Simple verified flag
}

// Snapshot represents a VM or container snapshot
type Snapshot struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SnapTime    int64  `json:"snaptime"`
	Parent      string `json:"parent"`
	VMID        int    `json:"vmid"`
}

// GetVMs returns all VMs on a specific node
func (c *Client) GetVMs(ctx context.Context, node string) ([]VM, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []VM `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetContainers returns all LXC containers on a specific node
func (c *Client) GetContainers(ctx context.Context, node string) ([]Container, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Container `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetStorage returns storage information for a specific node
func (c *Client) GetStorage(ctx context.Context, node string) ([]Storage, error) {
	// Storage queries can take longer on large clusters or slow storage backends
	// Create a new context with shorter timeout for storage API calls
	// Storage endpoints can hang when NFS/network storage is unavailable
	// Using 30s timeout as a balance between responsiveness and reliability
	storageCtx := ctx
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 30*time.Second {
		var cancel context.CancelFunc
		storageCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	resp, err := c.get(storageCtx, fmt.Sprintf("/nodes/%s/storage", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Storage `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetAllStorage returns storage information across all nodes
func (c *Client) GetAllStorage(ctx context.Context) ([]Storage, error) {
	// Storage queries can take longer on large clusters
	// Create a new context with shorter timeout for storage API calls
	// Storage endpoints can hang when NFS/network storage is unavailable
	// Using 30s timeout as a balance between responsiveness and reliability
	storageCtx := ctx
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 30*time.Second {
		var cancel context.CancelFunc
		storageCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	resp, err := c.get(storageCtx, "/storage")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Storage `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// Task represents a Proxmox task
type Task struct {
	UPID      string `json:"upid"`
	Node      string `json:"node"`
	PID       int    `json:"pid"`
	PStart    int64  `json:"pstart"`
	StartTime int64  `json:"starttime"`
	Type      string `json:"type"`
	ID        string `json:"id"`
	User      string `json:"user"`
	Status    string `json:"status,omitempty"`
	EndTime   int64  `json:"endtime,omitempty"`
}

// GetNodeTasks gets tasks for a specific node
func (c *Client) GetNodeTasks(ctx context.Context, node string) ([]Task, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/tasks", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Task `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetBackupTasks gets all backup tasks across all nodes
func (c *Client) GetBackupTasks(ctx context.Context) ([]Task, error) {
	// First get all nodes
	nodes, err := c.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	var allTasks []Task
	for _, node := range nodes {
		if node.Status != "online" {
			continue
		}

		tasks, err := c.GetNodeTasks(ctx, node.Node)
		if err != nil {
			// Log error but continue with other nodes
			continue
		}

		// Filter for backup tasks
		for _, task := range tasks {
			if task.Type == "vzdump" {
				allTasks = append(allTasks, task)
			}
		}
	}

	return allTasks, nil
}

// GetContainerInterfaces returns the network interfaces (with IPs) for a container.
func (c *Client) GetContainerInterfaces(ctx context.Context, node string, vmid int) ([]ContainerInterface, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", node, vmid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get container interfaces (status %d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Data []ContainerInterface `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetStorageContent returns the content of a specific storage
func (c *Client) GetStorageContent(ctx context.Context, node, storage string) ([]StorageContent, error) {
	// Storage content queries can take longer on large storages, especially PBS
	// with encrypted backups which can take 10-20+ seconds to enumerate.
	// Using 60s timeout to accommodate slow PBS storage backends while still
	// preventing indefinite hangs on unavailable NFS/network storage.
	storageCtx := ctx
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 60*time.Second {
		var cancel context.CancelFunc
		storageCtx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	resp, err := c.get(storageCtx, fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []StorageContent `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Filter for backup content only
	var backups []StorageContent
	for _, content := range result.Data {
		if content.Content == "backup" || content.Content == "vztmpl" {
			backups = append(backups, content)
		}
	}

	return backups, nil
}

// GetVMSnapshots returns snapshots for a specific VM
func (c *Client) GetVMSnapshots(ctx context.Context, node string, vmid int) ([]Snapshot, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/snapshot", node, vmid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Snapshot `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Filter out the 'current' snapshot which is not a real snapshot
	var snapshots []Snapshot
	for _, snap := range result.Data {
		if snap.Name != "current" {
			snap.VMID = vmid
			snapshots = append(snapshots, snap)
		}
	}

	return snapshots, nil
}

// GetContainerSnapshots returns snapshots for a specific container
func (c *Client) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]Snapshot, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/snapshot", node, vmid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Snapshot `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Filter out the 'current' snapshot which is not a real snapshot
	var snapshots []Snapshot
	for _, snap := range result.Data {
		if snap.Name != "current" {
			snap.VMID = vmid
			snapshots = append(snapshots, snap)
		}
	}

	return snapshots, nil
}

// ClusterStatus represents the cluster status response
type ClusterStatus struct {
	Type    string `json:"type"`    // "cluster" or "node"
	ID      string `json:"id"`      // Node ID or cluster name
	Name    string `json:"name"`    // Node name
	IP      string `json:"ip"`      // Node IP address
	Local   int    `json:"local"`   // 1 if this is the local node
	Nodeid  int    `json:"nodeid"`  // Node ID in cluster
	Online  int    `json:"online"`  // 1 if online
	Level   string `json:"level"`   // Connection level
	Quorate int    `json:"quorate"` // 1 if cluster has quorum
}

// GetClusterStatus returns the cluster status including all nodes
func (c *Client) GetClusterStatus(ctx context.Context) ([]ClusterStatus, error) {
	resp, err := c.get(ctx, "/cluster/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []ClusterStatus `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// IsClusterMember checks if this node is part of a cluster
func (c *Client) IsClusterMember(ctx context.Context) (bool, error) {
	status, err := c.GetClusterStatus(ctx)
	if err != nil {
		// If we can't get cluster status, assume it's not a cluster
		// This prevents treating API errors as cluster membership
		return false, nil
	}

	// Check for explicit cluster entry (most reliable indicator)
	for _, s := range status {
		if s.Type == "cluster" {
			// Found a cluster entry - this is definitely a cluster
			return true, nil
		}
	}

	// Fallback: If we have more than one node entry, it's likely a cluster
	// (though this shouldn't happen without a cluster entry)
	nodeCount := 0
	for _, s := range status {
		if s.Type == "node" {
			nodeCount++
		}
	}

	return nodeCount > 1, nil
}

// GetVMConfig returns the configuration for a specific VM
func (c *Client) GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetVMAgentInfo returns guest agent information for a VM if available
func (c *Client) GetVMAgentInfo(ctx context.Context, node string, vmid int) (map[string]interface{}, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/get-osinfo", node, vmid))
	if err != nil {
		// Guest agent might not be installed or running
		return nil, fmt.Errorf("guest agent get-osinfo: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetVMAgentVersion returns the guest agent version information for a VM if available.
func (c *Client) GetVMAgentVersion(ctx context.Context, node string, vmid int) (string, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/info", node, vmid))
	if err != nil {
		return "", fmt.Errorf("guest agent info: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Result map[string]interface{} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	extractVersion := func(val interface{}) string {
		switch v := val.(type) {
		case string:
			return strings.TrimSpace(v)
		case map[string]interface{}:
			if ver, ok := v["version"]; ok {
				if s, ok := ver.(string); ok {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}

	if result.Data.Result != nil {
		if version := extractVersion(result.Data.Result["version"]); version != "" {
			return version, nil
		}
		if qemuGA, ok := result.Data.Result["qemu-ga"]; ok {
			if version := extractVersion(qemuGA); version != "" {
				return version, nil
			}
		}
	}

	return "", nil
}

// VMFileSystem represents filesystem information from QEMU guest agent
type VMFileSystem struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Mountpoint string        `json:"mountpoint"`
	TotalBytes uint64        `json:"total-bytes"`
	UsedBytes  uint64        `json:"used-bytes"`
	Disk       string        // Extracted disk device name for duplicate detection
	DiskRaw    []interface{} `json:"disk"` // Raw disk device info from API
}

func (fs *VMFileSystem) UnmarshalJSON(data []byte) error {
	type rawVMFileSystem struct {
		Name       string        `json:"name"`
		Type       string        `json:"type"`
		Mountpoint string        `json:"mountpoint"`
		TotalBytes interface{}   `json:"total-bytes"`
		UsedBytes  interface{}   `json:"used-bytes"`
		DiskRaw    []interface{} `json:"disk"`
	}

	var raw rawVMFileSystem
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	total, err := parseUint64Flexible(raw.TotalBytes)
	if err != nil {
		return err
	}
	used, err := parseUint64Flexible(raw.UsedBytes)
	if err != nil {
		return err
	}

	fs.Name = raw.Name
	fs.Type = raw.Type
	fs.Mountpoint = raw.Mountpoint
	fs.TotalBytes = total
	fs.UsedBytes = used
	fs.DiskRaw = raw.DiskRaw
	fs.Disk = ""
	return nil
}

func parseUint64Flexible(value interface{}) (uint64, error) {
	switch v := value.(type) {
	case nil:
		return 0, nil
	case uint64:
		return v, nil
	case int:
		if v < 0 {
			return 0, nil
		}
		return uint64(v), nil
	case int64:
		if v < 0 {
			return 0, nil
		}
		return uint64(v), nil
	case float64:
		if v < 0 {
			return 0, nil
		}
		return uint64(v), nil
	case json.Number:
		return parseUint64Flexible(v.String())
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, nil
		}
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			u, err := strconv.ParseUint(s[2:], 16, 64)
			if err != nil {
				return 0, err
			}
			return u, nil
		}
		if strings.ContainsAny(s, ".eE") {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return 0, err
			}
			if f < 0 {
				return 0, nil
			}
			return uint64(f), nil
		}
		u, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, err
		}
		return u, nil
	default:
		return 0, fmt.Errorf("unsupported type %T for uint64 conversion", value)
	}
}

type VMIpAddress struct {
	Address string `json:"ip-address"`
	Prefix  int    `json:"prefix"`
}

type VMNetworkInterface struct {
	Name          string        `json:"name"`
	HardwareAddr  string        `json:"hardware-address"`
	IPAddresses   []VMIpAddress `json:"ip-addresses"`
	Statistics    interface{}   `json:"statistics,omitempty"`
	HasIp4Gateway bool          `json:"has-ipv4-synth-gateway,omitempty"`
	HasIp6Gateway bool          `json:"has-ipv6-synth-gateway,omitempty"`
}

// GetVMFSInfo returns filesystem information from QEMU guest agent
func (c *Client) GetVMFSInfo(ctx context.Context, node string, vmid int) ([]VMFileSystem, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/get-fsinfo", node, vmid))
	if err != nil {
		// Guest agent might not be installed or running
		return nil, fmt.Errorf("guest agent get-fsinfo: %w", err)
	}
	defer resp.Body.Close()

	// First, read the response body into bytes so we can try multiple unmarshal attempts
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Log the raw response for debugging
	log.Debug().
		Str("node", node).
		Int("vmid", vmid).
		Str("response", string(bodyBytes)).
		Msg("Raw response from guest agent get-fsinfo")

	// Try to unmarshal as an array first (expected format)
	var arrayResult struct {
		Data struct {
			Result []VMFileSystem `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &arrayResult); err == nil && arrayResult.Data.Result != nil {
		// Post-process to extract disk device names
		for i := range arrayResult.Data.Result {
			fs := &arrayResult.Data.Result[i]
			// Extract disk device name from the DiskRaw field
			if len(fs.DiskRaw) > 0 {
				// The disk field usually contains device info as a map
				if diskMap, ok := fs.DiskRaw[0].(map[string]interface{}); ok {
					// Try to get the device name from various possible fields
					if dev, ok := diskMap["dev"].(string); ok {
						fs.Disk = dev
					} else if serial, ok := diskMap["serial"].(string); ok {
						fs.Disk = serial
					} else if bus, ok := diskMap["bus-type"].(string); ok {
						if target, ok := diskMap["target"].(float64); ok {
							fs.Disk = fmt.Sprintf("%s-%d", bus, int(target))
						}
					}
				}
			}
			// If we still don't have a disk identifier, use the mountpoint as a fallback
			if fs.Disk == "" && fs.Mountpoint != "" {
				// For root filesystem, use a special identifier
				if fs.Mountpoint == "/" {
					fs.Disk = "root-filesystem"
				} else {
					// For Windows, normalize drive letters to prevent duplicate counting
					// Windows guest agent can return multiple directory entries (C:\, C:\Users, C:\Windows)
					// all on the same physical drive. Without disk[] metadata, we must deduplicate by drive letter.
					isWindowsDrive := len(fs.Mountpoint) >= 2 && fs.Mountpoint[1] == ':' && strings.Contains(fs.Mountpoint, "\\")
					if isWindowsDrive {
						// Use drive letter as identifier (e.g., "C:" for C:\, C:\Users, etc.)
						driveLetter := strings.ToUpper(fs.Mountpoint[:2])
						fs.Disk = driveLetter
						log.Debug().
							Str("node", node).
							Int("vmid", vmid).
							Str("mountpoint", fs.Mountpoint).
							Str("synthesized_disk", driveLetter).
							Msg("Synthesized Windows drive identifier from mountpoint")
					} else {
						// Use mountpoint as unique identifier for non-Windows paths
						fs.Disk = fs.Mountpoint
					}
				}
			}
		}
		return arrayResult.Data.Result, nil
	}

	// If that fails, try as an object (might be an error response or different format)
	var objectResult struct {
		Data struct {
			Result interface{} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &objectResult); err == nil {
		// If result is an object, it might be an error or empty response
		// Check if it's null or an error
		if objectResult.Data.Result == nil {
			log.Debug().
				Str("node", node).
				Int("vmid", vmid).
				Msg("GetVMFSInfo received null result - guest agent may not be providing disk info")
		} else {
			log.Debug().
				Str("node", node).
				Int("vmid", vmid).
				Interface("result", objectResult.Data.Result).
				Msg("GetVMFSInfo received object instead of array")
		}
		// Return empty array to indicate no filesystem info available
		return []VMFileSystem{}, nil
	}

	// If both fail, return error
	return nil, fmt.Errorf("unexpected response format from guest agent get-fsinfo")
}

// GetVMNetworkInterfaces returns network interfaces reported by the guest agent
func (c *Client) GetVMNetworkInterfaces(ctx context.Context, node string, vmid int) ([]VMNetworkInterface, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, vmid))
	if err != nil {
		return nil, fmt.Errorf("guest agent network-get-interfaces: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Result []VMNetworkInterface `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data.Result, nil
}

// GetVMStatus returns detailed VM status including balloon info
func (c *Client) GetVMStatus(ctx context.Context, node string, vmid int) (*VMStatus, error) {
	// Note: Proxmox 9.x removed support for the "full" parameter
	// The endpoint now returns all data by default
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/qemu/%d/status/current", node, vmid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data VMStatus `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// GetContainerStatus returns detailed container status using real-time endpoint
func (c *Client) GetContainerStatus(ctx context.Context, node string, vmid int) (*Container, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/lxc/%d/status/current", node, vmid))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data Container `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// ClusterResource represents a resource from /cluster/resources
type ClusterResource struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Node      string  `json:"node"`
	Status    string  `json:"status"`
	Name      string  `json:"name,omitempty"`
	VMID      int     `json:"vmid,omitempty"`
	CPU       float64 `json:"cpu,omitempty"`
	MaxCPU    int     `json:"maxcpu,omitempty"`
	Mem       uint64  `json:"mem,omitempty"`
	MaxMem    uint64  `json:"maxmem,omitempty"`
	Disk      uint64  `json:"disk,omitempty"`
	MaxDisk   uint64  `json:"maxdisk,omitempty"`
	NetIn     uint64  `json:"netin,omitempty"`
	NetOut    uint64  `json:"netout,omitempty"`
	DiskRead  uint64  `json:"diskread,omitempty"`
	DiskWrite uint64  `json:"diskwrite,omitempty"`
	Uptime    uint64  `json:"uptime,omitempty"`
	Template  int     `json:"template,omitempty"`
	Tags      string  `json:"tags,omitempty"`
}

// GetClusterResources returns all resources (VMs, containers) across the cluster
func (c *Client) GetClusterResources(ctx context.Context, resourceType string) ([]ClusterResource, error) {
	path := "/cluster/resources"
	if resourceType != "" {
		path = fmt.Sprintf("%s?type=%s", path, resourceType)
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []ClusterResource `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// ZFSPoolStatus represents the status of a ZFS pool (list endpoint)
type ZFSPoolStatus struct {
	Name   string  `json:"name"`
	Health string  `json:"health"` // ONLINE, DEGRADED, FAULTED, etc.
	Size   uint64  `json:"size"`
	Alloc  uint64  `json:"alloc"`
	Free   uint64  `json:"free"`
	Frag   int     `json:"frag"`
	Dedup  float64 `json:"dedup"`
}

// ZFSPoolDetail represents detailed status of a ZFS pool
type ZFSPoolDetail struct {
	Name     string          `json:"name"`
	State    string          `json:"state"`    // ONLINE, DEGRADED, FAULTED, etc.
	Status   string          `json:"status"`   // Detailed status message
	Action   string          `json:"action"`   // Recommended action
	Scan     string          `json:"scan"`     // Scan status
	Errors   string          `json:"errors"`   // Error summary
	Children []ZFSPoolDevice `json:"children"` // Top-level vdevs
}

// ZFSPoolDevice represents a device in a ZFS pool
type ZFSPoolDevice struct {
	Name     string          `json:"name"`
	State    string          `json:"state"` // ONLINE, DEGRADED, FAULTED, etc.
	Read     int64           `json:"read"`
	Write    int64           `json:"write"`
	Cksum    int64           `json:"cksum"`
	Msg      string          `json:"msg"`
	Leaf     int             `json:"leaf"` // 1 for leaf devices, 0 for vdevs
	Children []ZFSPoolDevice `json:"children,omitempty"`
}

// VMStatus represents detailed VM status
// VMMemInfo describes memory statistics reported by the guest agent.
// Proxmox surfaces guest /proc/meminfo values (in bytes). The available
// field is only present on newer agent versions, so we keep the raw
// components to reconstruct it when missing.
type VMMemInfo struct {
	Total     uint64 `json:"total,omitempty"`
	Used      uint64 `json:"used,omitempty"`
	Free      uint64 `json:"free,omitempty"`
	Available uint64 `json:"available,omitempty"`
	Buffers   uint64 `json:"buffers,omitempty"`
	Cached    uint64 `json:"cached,omitempty"`
	Shared    uint64 `json:"shared,omitempty"`
}

// VMAgentField handles the polymorphic agent field that changed in Proxmox 8.3+.
// Older versions: integer (0 or 1)
// Proxmox 8.3+: object {"enabled":1,"available":1} or similar
type VMAgentField struct {
	Value int
}

// UnmarshalJSON implements custom JSON unmarshaling to handle both int and object formats
func (a *VMAgentField) UnmarshalJSON(data []byte) error {
	// Try parsing as int first (older Proxmox versions)
	var intValue int
	if err := json.Unmarshal(data, &intValue); err == nil {
		a.Value = intValue
		return nil
	}

	// Try parsing as object (Proxmox 8.3+)
	var objValue struct {
		Enabled   int `json:"enabled"`
		Available int `json:"available"`
	}
	if err := json.Unmarshal(data, &objValue); err == nil {
		// Agent is considered enabled if either field is > 0
		// Typically we want to check "available" for actual functionality
		if objValue.Available > 0 {
			a.Value = objValue.Available
		} else if objValue.Enabled > 0 {
			a.Value = objValue.Enabled
		} else {
			a.Value = 0
		}
		return nil
	}

	// If neither worked, default to 0 (agent disabled)
	a.Value = 0
	return nil
}

// VMStatus represents detailed VM status returned by Proxmox.
type VMStatus struct {
	Status     string       `json:"status"`
	CPU        float64      `json:"cpu"`
	CPUs       int          `json:"cpus"`
	Mem        uint64       `json:"mem"`
	MaxMem     uint64       `json:"maxmem"`
	Balloon    uint64       `json:"balloon"`
	BalloonMin uint64       `json:"balloon_min"`
	FreeMem    uint64       `json:"freemem"`
	MemInfo    *VMMemInfo   `json:"meminfo,omitempty"`
	Disk       uint64       `json:"disk"`
	MaxDisk    uint64       `json:"maxdisk"`
	DiskRead   uint64       `json:"diskread"`
	DiskWrite  uint64       `json:"diskwrite"`
	NetIn      uint64       `json:"netin"`
	NetOut     uint64       `json:"netout"`
	Uptime     uint64       `json:"uptime"`
	Agent      VMAgentField `json:"agent"`
}

// GetZFSPoolStatus gets the status of ZFS pools on a node
func (c *Client) GetZFSPoolStatus(ctx context.Context, node string) ([]ZFSPoolStatus, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/disks/zfs", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []ZFSPoolStatus `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// GetZFSPoolDetail gets detailed status of a specific ZFS pool
func (c *Client) GetZFSPoolDetail(ctx context.Context, node, pool string) (*ZFSPoolDetail, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/disks/zfs/%s", node, pool))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Proxmox returns {"data": {...}}
	var result struct {
		Data ZFSPoolDetail `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

const wearoutUnknown = -1

// Disk represents a physical disk on a Proxmox node
type Disk struct {
	DevPath string `json:"devpath"`
	Model   string `json:"model"`
	Serial  string `json:"serial"`
	Type    string `json:"type"`   // nvme, sata, sas
	Health  string `json:"health"` // PASSED, FAILED, UNKNOWN
	Wearout int    `json:"-"`      // SSD wear percentage (0-100, 100 is best, -1 when unavailable)
	Size    int64  `json:"size"`   // Size in bytes
	RPM     int    `json:"rpm"`    // 0 for SSDs
	Used    string `json:"used"`   // Filesystem or partition usage
	Vendor  string `json:"vendor"`
	WWN     string `json:"wwn"` // World Wide Name
}

// UnmarshalJSON custom unmarshaler for Disk to handle non-numeric wearout values
func (d *Disk) UnmarshalJSON(data []byte) error {
	type Alias Disk
	aux := &struct {
		Wearout interface{} `json:"wearout"`
		RPM     interface{} `json:"rpm"`
		*Alias
	}{
		Alias: (*Alias)(d),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle wearout field which can be int, string ("N/A"), or null
	switch v := aux.Wearout.(type) {
	case float64:
		d.Wearout = int(v)
	case string:
		// Proxmox returns "N/A" or empty string for HDDs/RAID controllers.
		// Some controllers also return numeric wearout values as strings, so try to parse them.
		d.Wearout = parseWearoutValue(v)
	case nil:
		d.Wearout = wearoutUnknown
	default:
		// Unexpected type, normalize to unknown
		d.Wearout = wearoutUnknown
	}

	d.Wearout = clampWearoutConsumed(d.Wearout)

	// Handle rpm field which can be number, string descriptor ("SSD"/"N/A"), or null
	switch v := aux.RPM.(type) {
	case float64:
		d.RPM = int(v)
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || strings.EqualFold(trimmed, "ssd") || strings.EqualFold(trimmed, "n/a") {
			d.RPM = 0
			break
		}
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			d.RPM = parsed
		} else {
			d.RPM = 0
		}
	case nil:
		d.RPM = 0
	default:
		d.RPM = 0
	}

	return nil
}

// parseWearoutValue normalizes the wearout value returned by Proxmox into an integer percentage.
// The API occasionally wraps numeric values in escaped quotes (\"81\"), appends percent symbols,
// or reports descriptive strings like "N/A". We strip those variations so downstream code can work
// with a simple integer. Non-numeric results bubble up wearoutUnknown (-1) so callers can treat them
// as "not reported" instead of a critical wearout value.
func parseWearoutValue(raw string) int {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return wearoutUnknown
	}

	// Remove escaped quotes and surrounding quotes the API sometimes includes.
	cleaned = strings.ReplaceAll(cleaned, "\\\"", "")
	cleaned = strings.Trim(cleaned, "\"'")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return wearoutUnknown
	}

	switch strings.ToLower(cleaned) {
	case "n/a", "na", "none", "unknown":
		return wearoutUnknown
	}

	if parsed, err := strconv.Atoi(cleaned); err == nil {
		return parsed
	}

	if parsed, err := strconv.ParseFloat(cleaned, 64); err == nil {
		if parsed <= 0 {
			return int(parsed)
		}
		return int(parsed)
	}

	var digits strings.Builder
	for _, r := range cleaned {
		if unicode.IsDigit(r) {
			digits.WriteRune(r)
		}
	}

	if digits.Len() > 0 {
		if parsed, err := strconv.Atoi(digits.String()); err == nil {
			return parsed
		}
	}

	return wearoutUnknown
}

func clampWearoutConsumed(val int) int {
	if val == wearoutUnknown {
		return wearoutUnknown
	}
	if val < 0 {
		return 0
	}
	if val > 100 {
		return 100
	}
	return val
}

// DiskSmart represents SMART data for a disk
type DiskSmart struct {
	Health  string `json:"health"`  // PASSED, FAILED, UNKNOWN
	Wearout int    `json:"wearout"` // SSD wear percentage
	Type    string `json:"type"`    // Type of response (text, attributes)
	Text    string `json:"text"`    // Raw SMART output text
}

// GetDisks returns the list of physical disks on a node
func (c *Client) GetDisks(ctx context.Context, node string) ([]Disk, error) {
	resp, err := c.request(ctx, "GET", fmt.Sprintf("/nodes/%s/disks/list", node), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Disk `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// AptPackage represents a pending package update from apt
type AptPackage struct {
	Package     string `json:"Package"`     // Package name
	Title       string `json:"Title"`       // Human-readable title
	Description string `json:"Description"` // Package description
	OldVersion  string `json:"OldVersion"`  // Currently installed version
	NewVersion  string `json:"Version"`     // Available version
	Priority    string `json:"Priority"`    // Update priority (e.g., "important", "optional")
	Section     string `json:"Section"`     // Package section
	Origin      string `json:"Origin"`      // Repository origin
}

// GetNodePendingUpdates returns the list of pending apt updates for a node
// Requires Sys.Audit permission on /nodes/{node}
func (c *Client) GetNodePendingUpdates(ctx context.Context, node string) ([]AptPackage, error) {
	resp, err := c.get(ctx, fmt.Sprintf("/nodes/%s/apt/update", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []AptPackage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}
