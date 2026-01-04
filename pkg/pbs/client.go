package pbs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
	"github.com/rs/zerolog/log"
)

// Client represents a Proxmox Backup Server API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       auth
	config     ClientConfig
}

// ClientConfig holds configuration for the PBS client
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

// NewClient creates a new PBS API client
func NewClient(cfg ClientConfig) (*Client, error) {
	// Normalize host URL - ensure it has a protocol
	if !strings.HasPrefix(cfg.Host, "http://") && !strings.HasPrefix(cfg.Host, "https://") {
		// Default to HTTPS if no protocol specified
		cfg.Host = "https://" + cfg.Host
		// Log that we're defaulting to HTTPS
		log.Debug().Str("host", cfg.Host).Msg("No protocol specified in PBS host, defaulting to HTTPS")
	}

	// Warn if using HTTP
	if strings.HasPrefix(cfg.Host, "http://") {
		log.Warn().Str("host", cfg.Host).Msg("Using HTTP for PBS connection. PBS typically requires HTTPS. If connection fails, try using https:// instead")
	}

	var user, realm string

	// For token auth, user might be empty or in a different format
	if cfg.TokenName != "" && cfg.TokenValue != "" {
		// Token authentication - parse the token name to extract user info if needed
		if strings.Contains(cfg.TokenName, "!") {
			// Token name contains full format: user@realm!tokenname
			parts := strings.Split(cfg.TokenName, "!")
			if len(parts) == 2 && strings.Contains(parts[0], "@") {
				userParts := strings.Split(parts[0], "@")
				if len(userParts) == 2 {
					user = userParts[0]
					realm = userParts[1]
					// Update token name to just the token part
					cfg.TokenName = parts[1]
				}
			}
		} else if cfg.User != "" {
			// User provided separately
			parts := strings.Split(cfg.User, "@")
			if len(parts) == 2 {
				user = parts[0]
				realm = parts[1]
			} else {
				// If no realm specified, default to pbs
				user = cfg.User
				realm = "pbs"
			}
		} else {
			return nil, fmt.Errorf("token authentication requires user information either in token name (user@realm!tokenname) or user field")
		}

		if user == "" {
			return nil, fmt.Errorf("could not parse user information from token name")
		}
	} else {
		// Password authentication - user@realm format is required
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

	client := &Client{
		baseURL:    strings.TrimSuffix(cfg.Host, "/") + "/api2/json",
		httpClient: httpClient,
		config:     cfg,
		auth: auth{
			user:       user,
			realm:      realm,
			tokenName:  cfg.TokenName,
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
	c.auth.expiresAt = time.Now().Add(2 * time.Hour) // PBS tickets expire after 2 hours

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
		// Note: tokenName already contains just the token part (e.g., "pulse-token")
		// after parsing in NewClient, so we reconstruct the full format
		authHeader := fmt.Sprintf("PBSAPIToken=%s@%s!%s:%s",
			c.auth.user, c.auth.realm, c.auth.tokenName, c.auth.tokenValue)
		req.Header.Set("Authorization", authHeader)
		// NEVER log the actual token value - only log that we're using token auth
		// Log the auth header format (without the secret)
		maskedHeader := fmt.Sprintf("PBSAPIToken=%s@%s!%s:***",
			c.auth.user, c.auth.realm, c.auth.tokenName)
		log.Debug().
			Str("user", c.auth.user).
			Str("realm", c.auth.realm).
			Str("tokenName", c.auth.tokenName).
			Str("authHeaderFormat", maskedHeader).
			Str("url", req.URL.String()).
			Msg("Setting PBS API token authentication")
	} else if c.auth.ticket != "" {
		// Ticket authentication
		req.Header.Set("Cookie", "PBSAuthCookie="+c.auth.ticket)
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

		// Create base error
		err := fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))

		// Wrap with appropriate error type
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
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

// post performs a POST request
func (c *Client) post(ctx context.Context, path string, data url.Values) (*http.Response, error) {
	return c.request(ctx, "POST", path, data)
}

// TokenResponse represents the response from token creation
type TokenResponse struct {
	TokenID string `json:"tokenid"`
	Value   string `json:"value"`
}

// CreateUser creates a new user on the PBS server
// This requires admin privileges (typically root@pam)
func (c *Client) CreateUser(ctx context.Context, userID, comment string) error {
	log.Debug().Str("userID", userID).Msg("PBS CreateUser: creating user")

	data := url.Values{}
	data.Set("userid", userID)
	if comment != "" {
		data.Set("comment", comment)
	}

	resp, err := c.post(ctx, "/access/users", data)
	if err != nil {
		// User might already exist, which is okay
		if strings.Contains(err.Error(), "already exists") {
			log.Debug().Str("userID", userID).Msg("PBS CreateUser: user already exists")
			return nil
		}
		return fmt.Errorf("create user: %w", err)
	}
	defer resp.Body.Close()

	log.Info().Str("userID", userID).Msg("PBS CreateUser: user created successfully")
	return nil
}

// SetUserACL sets ACL permissions for a user
func (c *Client) SetUserACL(ctx context.Context, authID, path, role string) error {
	log.Debug().Str("authID", authID).Str("path", path).Str("role", role).Msg("PBS SetUserACL: setting ACL")

	data := url.Values{}
	data.Set("auth-id", authID)
	data.Set("path", path)
	data.Set("role", role)

	resp, err := c.post(ctx, "/access/acl", data)
	if err != nil {
		return fmt.Errorf("set ACL: %w", err)
	}
	defer resp.Body.Close()

	log.Info().Str("authID", authID).Str("role", role).Msg("PBS SetUserACL: ACL set successfully")
	return nil
}

// CreateUserToken creates an API token for a user
// Returns the full token ID and the secret value
func (c *Client) CreateUserToken(ctx context.Context, userID, tokenName string) (*TokenResponse, error) {
	log.Debug().Str("userID", userID).Str("tokenName", tokenName).Msg("PBS CreateUserToken: creating token")

	// PBS API: POST /access/users/{userid}/token/{tokenname}
	path := fmt.Sprintf("/access/users/%s/token/%s", url.PathEscape(userID), url.PathEscape(tokenName))

	// Token with no expiry
	data := url.Values{}
	data.Set("expire", "0")

	resp, err := c.post(ctx, path, data)
	if err != nil {
		return nil, fmt.Errorf("create token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Parse response - PBS returns {"data": {"tokenid": "...", "value": "..."}}
	var result struct {
		Data TokenResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(body))
	}

	if result.Data.Value == "" {
		return nil, fmt.Errorf("empty token value in response: %s", string(body))
	}

	log.Info().
		Str("userID", userID).
		Str("tokenName", tokenName).
		Str("tokenID", result.Data.TokenID).
		Msg("PBS CreateUserToken: token created successfully")

	return &result.Data, nil
}

// SetupMonitoringAccess creates a monitoring user with Audit role and returns API token
// This is the turnkey method for setting up PBS monitoring access
func (c *Client) SetupMonitoringAccess(ctx context.Context, tokenName string) (tokenID, tokenValue string, err error) {
	const (
		monitorUser    = "pulse-monitor@pbs"
		monitorComment = "Pulse monitoring service"
		auditRole      = "Audit"
	)

	log.Info().Str("tokenName", tokenName).Msg("PBS SetupMonitoringAccess: starting turnkey setup")

	// Step 1: Create monitoring user (ignore if exists)
	if err := c.CreateUser(ctx, monitorUser, monitorComment); err != nil {
		log.Warn().Err(err).Msg("PBS SetupMonitoringAccess: failed to create user (may already exist)")
		// Continue - user might already exist
	}

	// Step 2: Grant Audit role on / (root path)
	if err := c.SetUserACL(ctx, monitorUser, "/", auditRole); err != nil {
		return "", "", fmt.Errorf("set user ACL: %w", err)
	}

	// Step 3: Create API token
	token, err := c.CreateUserToken(ctx, monitorUser, tokenName)
	if err != nil {
		return "", "", fmt.Errorf("create token: %w", err)
	}

	// Step 4: Grant Audit role to the token as well
	if err := c.SetUserACL(ctx, token.TokenID, "/", auditRole); err != nil {
		log.Warn().Err(err).Msg("PBS SetupMonitoringAccess: failed to set ACL on token (may not be required)")
		// Continue - might not be strictly necessary
	}

	log.Info().
		Str("tokenID", token.TokenID).
		Msg("PBS SetupMonitoringAccess: turnkey setup complete")

	return token.TokenID, token.Value, nil
}

// Version represents PBS version information
type Version struct {
	Version string `json:"version"`
	Release string `json:"release"`
	Repoid  string `json:"repoid"`
}

// Datastore represents a PBS datastore
type Datastore struct {
	Store string `json:"store"`
	Total int64  `json:"total,omitempty"`
	Used  int64  `json:"used,omitempty"`
	Avail int64  `json:"avail,omitempty"`
	// Alternative field names PBS might use
	TotalSpace int64 `json:"total-space,omitempty"`
	UsedSpace  int64 `json:"used-space,omitempty"`
	AvailSpace int64 `json:"avail-space,omitempty"`
	// Status fields
	GCStatus            string  `json:"gc-status,omitempty"`
	DeduplicationFactor float64 `json:"deduplication_factor,omitempty"`
	Error               string  `json:"error,omitempty"`
}

// GetVersion returns PBS version information
func (c *Client) GetVersion(ctx context.Context) (*Version, error) {
	log.Debug().Msg("PBS GetVersion: starting request")
	resp, err := c.get(ctx, "/version")
	if err != nil {
		log.Debug().Err(err).Msg("PBS GetVersion: request failed")
		return nil, err
	}
	defer resp.Body.Close()
	log.Debug().Msg("PBS GetVersion: request succeeded")

	var result struct {
		Data Version `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

// GetNodeName returns the PBS node's hostname
func (c *Client) GetNodeName(ctx context.Context) (string, error) {
	log.Debug().Msg("PBS GetNodeName: fetching node name")

	resp, err := c.get(ctx, "/nodes")
	if err != nil {
		return "", fmt.Errorf("failed to get nodes: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode nodes response: %w", err)
	}

	if len(result.Data) == 0 {
		return "", fmt.Errorf("no nodes found")
	}

	// Return the first (usually only) node name
	nodeName := result.Data[0].Node
	log.Debug().Str("nodeName", nodeName).Msg("PBS GetNodeName: found node name")
	return nodeName, nil
}

// GetNodeStatus returns the status of the PBS node (CPU, memory, etc.)
func (c *Client) GetNodeStatus(ctx context.Context) (*NodeStatus, error) {
	log.Debug().Msg("PBS GetNodeStatus: starting")

	// The /nodes/localhost/status endpoint requires special permissions that API tokens often don't have
	// This is a known PBS limitation - the endpoint is primarily for internal use
	// We'll gracefully handle the permission error and return nil
	statusResp, err := c.get(ctx, "/nodes/localhost/status")
	if err != nil {
		// Check if this is a permission error (403)
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "permission") {
			log.Debug().Msg("PBS GetNodeStatus: permission denied (expected with API tokens) - returning nil")
			return nil, nil // Return nil without error for permission issues
		}
		return nil, fmt.Errorf("failed to get node status: %w", err)
	}
	defer statusResp.Body.Close()

	// Read the response body to log it
	body, err := io.ReadAll(statusResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read status response: %w", err)
	}

	log.Debug().Str("response", string(body)).Msg("PBS node status response")

	var statusResult struct {
		Data NodeStatus `json:"data"`
	}

	if err := json.Unmarshal(body, &statusResult); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return &statusResult.Data, nil
}

// GetDatastores returns all datastores with their status
func (c *Client) GetDatastores(ctx context.Context) ([]Datastore, error) {
	// First get the list of datastores
	resp, err := c.get(ctx, "/admin/datastore")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if response is HTML (error page) instead of JSON
	if len(body) > 0 && body[0] == '<' {
		// If using HTTP, suggest HTTPS
		if strings.HasPrefix(c.config.Host, "http://") {
			return nil, fmt.Errorf("PBS returned HTML instead of JSON. PBS typically requires HTTPS, not HTTP. Try changing your URL from %s to %s", c.config.Host, strings.Replace(c.config.Host, "http://", "https://", 1))
		}
		return nil, fmt.Errorf("PBS returned HTML instead of JSON (likely an error page). Please check your PBS URL and port (default is 8007)")
	}

	var datastoreList struct {
		Data []struct {
			Store   string `json:"store"`
			Comment string `json:"comment,omitempty"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &datastoreList); err != nil {
		log.Error().
			Str("response", string(body)).
			Err(err).
			Msg("Failed to parse PBS datastore list response")
		return nil, fmt.Errorf("failed to parse datastore list: %w", err)
	}

	// Now get status for each datastore
	var datastores []Datastore
	for _, ds := range datastoreList.Data {
		// Try to get RRD data first which has more statistics
		// RRD requires cf (consolidation function) and timeframe parameters
		// Valid cf values: AVERAGE, MAXIMUM, MINIMUM (all caps per PBS API spec)
		rrdPath := fmt.Sprintf("/admin/datastore/%s/rrd?cf=AVERAGE&timeframe=hour", ds.Store)
		rrdResp, err := c.get(ctx, rrdPath)
		var dedupFactor float64
		if err == nil {
			defer rrdResp.Body.Close()
			rrdBody, _ := io.ReadAll(rrdResp.Body)

			var rrdResult struct {
				Data []struct {
					Time        float64 `json:"time"`
					DedupFactor float64 `json:"dedup_factor"`
				} `json:"data"`
			}

			if json.Unmarshal(rrdBody, &rrdResult) == nil && len(rrdResult.Data) > 0 {
				// Get the most recent deduplication factor
				dedupFactor = rrdResult.Data[len(rrdResult.Data)-1].DedupFactor
				log.Info().Float64("dedup_from_rrd", dedupFactor).Str("store", ds.Store).Msg("Got dedup factor from RRD")
			}
		}

		// Get individual datastore status
		statusResp, err := c.get(ctx, fmt.Sprintf("/admin/datastore/%s/status", ds.Store))
		if err != nil {
			log.Error().Str("store", ds.Store).Err(err).Msg("Failed to get datastore status")
			// Create entry with no size info if status fails
			datastores = append(datastores, Datastore{
				Store: ds.Store,
				Error: fmt.Sprintf("Failed to get status: %v", err),
			})
			continue
		}
		defer statusResp.Body.Close()

		statusBody, err := io.ReadAll(statusResp.Body)
		if err != nil {
			log.Error().Str("store", ds.Store).Err(err).Msg("Failed to read datastore status response")
			datastores = append(datastores, Datastore{
				Store: ds.Store,
				Error: fmt.Sprintf("Failed to read status: %v", err),
			})
			continue
		}

		var statusResult struct {
			Data map[string]interface{} `json:"data"`
		}

		if err := json.Unmarshal(statusBody, &statusResult); err != nil {
			log.Error().
				Str("store", ds.Store).
				Str("response", string(statusBody)).
				Err(err).
				Msg("Failed to parse datastore status")
			datastores = append(datastores, Datastore{
				Store: ds.Store,
				Error: fmt.Sprintf("Failed to parse status: %v", err),
			})
			continue
		}

		// Extract fields from the map
		total, _ := statusResult.Data["total"].(float64)
		used, _ := statusResult.Data["used"].(float64)
		avail, _ := statusResult.Data["avail"].(float64)

		totalSpace, _ := statusResult.Data["total-space"].(float64)
		usedSpace, _ := statusResult.Data["used-space"].(float64)
		availSpace, _ := statusResult.Data["avail-space"].(float64)

		// Check for deduplication_factor in status response
		if df, ok := statusResult.Data["deduplication-factor"].(float64); ok {
			dedupFactor = df
		} else if df, ok := statusResult.Data["deduplication_factor"].(float64); ok {
			dedupFactor = df
		}

		// If still no dedup factor, try gc-status endpoint
		if dedupFactor == 0 {
			gcResp, err := c.get(ctx, fmt.Sprintf("/admin/datastore/%s/gc", ds.Store))
			if err == nil {
				defer gcResp.Body.Close()
				gcBody, _ := io.ReadAll(gcResp.Body)
				var gcResult struct {
					Data struct {
						IndexDataBytes float64 `json:"index-data-bytes"`
						DiskBytes      float64 `json:"disk-bytes"`
					} `json:"data"`
				}
				if json.Unmarshal(gcBody, &gcResult) == nil {
					// Calculate deduplication factor from index-data-bytes / disk-bytes
					if gcResult.Data.DiskBytes > 0 && gcResult.Data.IndexDataBytes > 0 {
						dedupFactor = gcResult.Data.IndexDataBytes / gcResult.Data.DiskBytes
						log.Info().
							Float64("index_bytes", gcResult.Data.IndexDataBytes).
							Float64("disk_bytes", gcResult.Data.DiskBytes).
							Float64("dedup_factor", dedupFactor).
							Str("store", ds.Store).
							Msg("Calculated dedup factor from gc endpoint")
					}
				}
			}
		}

		// Create datastore with status info
		datastore := Datastore{
			Store:               ds.Store,
			Total:               int64(total),
			Used:                int64(used),
			Avail:               int64(avail),
			TotalSpace:          int64(totalSpace),
			UsedSpace:           int64(usedSpace),
			AvailSpace:          int64(availSpace),
			DeduplicationFactor: dedupFactor,
		}

		// Log all fields to see what's available
		log.Info().
			Str("store", datastore.Store).
			Int64("total", datastore.Total).
			Int64("used", datastore.Used).
			Int64("avail", datastore.Avail).
			Float64("dedup_factor", datastore.DeduplicationFactor).
			Interface("all_fields", statusResult.Data).
			Msg("PBS datastore status - ALL FIELDS")

		datastores = append(datastores, datastore)
	}

	return datastores, nil
}

// NodeStatus represents PBS node status information
type NodeStatus struct {
	CPU         float64   `json:"cpu"`     // CPU usage percentage
	Memory      Memory    `json:"memory"`  // Memory information
	Uptime      int64     `json:"uptime"`  // Uptime in seconds
	LoadAverage []float64 `json:"loadavg"` // Load average [1min, 5min, 15min]
	KSM         KSMInfo   `json:"ksm"`     // Kernel Same-page Merging info
	Swap        Memory    `json:"swap"`    // Swap information
	RootFS      FSInfo    `json:"root"`    // Root filesystem info
}

// Memory represents memory information
type Memory struct {
	Total int64 `json:"total"` // Total memory in bytes
	Used  int64 `json:"used"`  // Used memory in bytes
	Free  int64 `json:"free"`  // Free memory in bytes
}

// KSMInfo represents Kernel Same-page Merging information
type KSMInfo struct {
	Shared int64 `json:"shared"` // Shared memory in bytes
}

// FSInfo represents filesystem information
type FSInfo struct {
	Total int64 `json:"total"` // Total space in bytes
	Used  int64 `json:"used"`  // Used space in bytes
	Free  int64 `json:"free"`  // Free space in bytes
}

// Namespace represents a PBS namespace
type Namespace struct {
	NS     string `json:"ns"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Parent string `json:"parent,omitempty"`
}

// BackupGroup represents a group of backups for a specific VM/CT
type BackupGroup struct {
	BackupType  string   `json:"backup-type"` // "vm" or "ct"
	BackupID    string   `json:"backup-id"`   // VMID
	LastBackup  int64    `json:"last-backup"` // Unix timestamp
	BackupCount int      `json:"backup-count"`
	Files       []string `json:"files,omitempty"`
	Owner       string   `json:"owner,omitempty"`
}

// BackupSnapshot represents a single backup snapshot
type BackupSnapshot struct {
	BackupType   string        `json:"backup-type"`     // "vm" or "ct"
	BackupID     string        `json:"backup-id"`       // VMID
	BackupTime   int64         `json:"backup-time"`     // Unix timestamp
	Files        []interface{} `json:"files,omitempty"` // Can be strings or objects
	Size         int64         `json:"size"`
	Protected    bool          `json:"protected"`
	Comment      string        `json:"comment,omitempty"`
	Owner        string        `json:"owner,omitempty"`
	Verification interface{}   `json:"verification,omitempty"` // Can be string or object
}

// ListNamespaces lists namespaces for a datastore
func (c *Client) ListNamespaces(ctx context.Context, datastore string, parentNamespace string, maxDepth int) ([]Namespace, error) {
	path := fmt.Sprintf("/admin/datastore/%s/namespace", datastore)

	// Build query parameters
	params := url.Values{}
	if parentNamespace != "" {
		params.Set("ns", parentNamespace)
	}
	if maxDepth > 0 {
		params.Set("max-depth", fmt.Sprintf("%d", maxDepth))
	}

	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		// If namespace endpoint doesn't exist (older PBS versions), return empty list
		if strings.Contains(err.Error(), "404") {
			return []Namespace{}, nil
		}
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []Namespace `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// ListBackupGroups lists all backup groups in a datastore/namespace
func (c *Client) ListBackupGroups(ctx context.Context, datastore string, namespace string) ([]BackupGroup, error) {
	path := fmt.Sprintf("/admin/datastore/%s/groups", datastore)

	// Add namespace parameter if provided
	params := url.Values{}
	if namespace != "" {
		params.Set("ns", namespace)
	}

	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	// Log the API call
	log.Debug().Str("url", c.baseURL+path).Msg("PBS API: ListBackupGroups")

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []BackupGroup `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode backup groups: %w", err)
	}

	log.Debug().
		Str("namespace", namespace).
		Int("count", len(result.Data)).
		Msg("PBS API: Backup groups found")
	return result.Data, nil
}

// ListBackupSnapshots lists all snapshots for a specific backup group
func (c *Client) ListBackupSnapshots(ctx context.Context, datastore string, namespace string, backupType string, backupID string) ([]BackupSnapshot, error) {
	path := fmt.Sprintf("/admin/datastore/%s/snapshots", datastore)

	// Build parameters
	params := url.Values{}
	if namespace != "" {
		params.Set("ns", namespace)
	}
	params.Set("backup-type", backupType)
	params.Set("backup-id", backupID)

	path = path + "?" + params.Encode()

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []BackupSnapshot `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode snapshots: %w", err)
	}

	return result.Data, nil
}

// ListAllBackups fetches all backups from all namespaces concurrently
func (c *Client) ListAllBackups(ctx context.Context, datastore string, namespaces []string) (map[string][]BackupSnapshot, error) {
	type namespaceResult struct {
		namespace string
		snapshots []BackupSnapshot
		err       error
	}

	// Channel for results
	resultCh := make(chan namespaceResult, len(namespaces))

	// WaitGroup to track goroutines
	var wg sync.WaitGroup

	// Semaphore to limit concurrent requests
	sem := make(chan struct{}, 3) // Max 3 concurrent requests

	// Fetch backups from each namespace concurrently
	for _, ns := range namespaces {
		wg.Add(1)
		go func(namespace string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Get groups first
			groups, err := c.ListBackupGroups(ctx, datastore, namespace)
			if err != nil {
				log.Error().
					Str("datastore", datastore).
					Str("namespace", namespace).
					Err(err).
					Msg("Failed to list backup groups")
				resultCh <- namespaceResult{namespace: namespace, err: err}
				return
			}

			log.Info().
				Str("datastore", datastore).
				Str("namespace", namespace).
				Int("groups", len(groups)).
				Msg("Found backup groups")

			var (
				allSnapshots []BackupSnapshot
				snapshotsMu  sync.Mutex
			)

			groupSem := make(chan struct{}, 5)
			var groupWG sync.WaitGroup

			// For each group, get snapshots concurrently with a small worker limit to avoid hammering PBS
			for _, group := range groups {
				if ctx.Err() != nil {
					log.Debug().
						Str("datastore", datastore).
						Str("namespace", namespace).
						Msg("Context cancelled before completing snapshot fetch")
					break
				}

				group := group

				groupWG.Add(1)
				go func() {
					defer groupWG.Done()

					select {
					case groupSem <- struct{}{}:
					case <-ctx.Done():
						return
					}
					defer func() { <-groupSem }()

					snapshots, err := c.ListBackupSnapshots(ctx, datastore, namespace, group.BackupType, group.BackupID)
					if err != nil {
						log.Error().
							Str("datastore", datastore).
							Str("namespace", namespace).
							Str("type", group.BackupType).
							Str("id", group.BackupID).
							Err(err).
							Msg("Failed to list snapshots")
						return
					}

					if len(snapshots) == 0 {
						return
					}

					snapshotsMu.Lock()
					allSnapshots = append(allSnapshots, snapshots...)
					snapshotsMu.Unlock()
				}()
			}

			groupWG.Wait()

			if ctx.Err() != nil {
				resultCh <- namespaceResult{namespace: namespace, err: ctx.Err()}
				return
			}

			resultCh <- namespaceResult{
				namespace: namespace,
				snapshots: allSnapshots,
				err:       nil,
			}
		}(ns)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results
	results := make(map[string][]BackupSnapshot)
	var errors []error

	for result := range resultCh {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("namespace %s: %w", result.namespace, result.err))
		} else {
			results[result.namespace] = result.snapshots
		}
	}

	// Return combined error if any occurred
	if len(errors) > 0 {
		return results, fmt.Errorf("errors fetching backups: %v", errors)
	}

	return results, nil
}
