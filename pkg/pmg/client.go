package pmg

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

type Client struct {
	baseURL    string
	httpClient *http.Client
	auth       auth
	config     ClientConfig
	mu         sync.Mutex
}

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

type auth struct {
	user       string
	realm      string
	ticket     string
	csrfToken  string
	tokenName  string
	tokenValue string
	expiresAt  time.Time
}

type apiResponse[T any] struct {
	Data T `json:"data"`
}

type VersionInfo struct {
	Version string `json:"version"`
	Release string `json:"release,omitempty"`
}

type MailStatistics struct {
	Count         flexibleFloat `json:"count"`
	CountIn       flexibleFloat `json:"count_in"`
	CountOut      flexibleFloat `json:"count_out"`
	SpamIn        flexibleFloat `json:"spamcount_in"`
	SpamOut       flexibleFloat `json:"spamcount_out"`
	VirusIn       flexibleFloat `json:"viruscount_in"`
	VirusOut      flexibleFloat `json:"viruscount_out"`
	BouncesIn     flexibleFloat `json:"bounces_in"`
	BouncesOut    flexibleFloat `json:"bounces_out"`
	BytesIn       flexibleFloat `json:"bytes_in"`
	BytesOut      flexibleFloat `json:"bytes_out"`
	GreylistCount flexibleFloat `json:"glcount"`
	JunkIn        flexibleFloat `json:"junk_in"`
	RBLRejects    flexibleFloat `json:"rbl_rejects"`
	Pregreet      flexibleFloat `json:"pregreet_rejects"`
	AvgProcessSec flexibleFloat `json:"avptime"`
}

type MailCountEntry struct {
	Index          flexibleInt   `json:"index"`
	Time           flexibleInt   `json:"time"`
	Count          flexibleFloat `json:"count"`
	CountIn        flexibleFloat `json:"count_in"`
	CountOut       flexibleFloat `json:"count_out"`
	SpamIn         flexibleFloat `json:"spamcount_in"`
	SpamOut        flexibleFloat `json:"spamcount_out"`
	VirusIn        flexibleFloat `json:"viruscount_in"`
	VirusOut       flexibleFloat `json:"viruscount_out"`
	BouncesIn      flexibleFloat `json:"bounces_in"`
	BouncesOut     flexibleFloat `json:"bounces_out"`
	RBLRejects     flexibleFloat `json:"rbl_rejects"`
	PregreetReject flexibleFloat `json:"pregreet_rejects"`
	GreylistCount  flexibleFloat `json:"glcount"`
}

type SpamScore struct {
	Level string        `json:"level"`
	Count flexibleInt   `json:"count"`
	Ratio flexibleFloat `json:"ratio"`
}

type ClusterStatusEntry struct {
	CID         int    `json:"cid"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	IP          string `json:"ip"`
	Fingerprint string `json:"fingerprint"`
}

type QuarantineStatus struct {
	Count     flexibleInt   `json:"count"`
	AvgBytes  flexibleFloat `json:"avgbytes"`
	AvgSpam   flexibleFloat `json:"avgspam,omitempty"`
	Megabytes flexibleFloat `json:"mbytes"`
}

type QueueStatusEntry struct {
	Active    flexibleInt `json:"active"`
	Deferred  flexibleInt `json:"deferred"`
	Hold      flexibleInt `json:"hold"`
	Incoming  flexibleInt `json:"incoming"`
	OldestAge flexibleInt `json:"oldest_age,omitempty"` // Age of oldest message in seconds
}

// BackupEntry represents a PMG configuration backup stored on a node.
type BackupEntry struct {
	Filename  string      `json:"filename"`
	Size      flexibleInt `json:"size"`
	Timestamp flexibleInt `json:"timestamp"`
}

// NewClient creates a new PMG (Proxmox Mail Gateway) API client.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	if !strings.HasPrefix(cfg.Host, "http://") && !strings.HasPrefix(cfg.Host, "https://") {
		cfg.Host = "https://" + cfg.Host
	}

	if strings.HasPrefix(cfg.Host, "http://") {
		log.Warn().Str("host", cfg.Host).Msg("Using HTTP for PMG connection - consider enabling HTTPS")
	}

	var user, realm string

	if cfg.TokenName != "" && cfg.TokenValue != "" {
		if strings.Contains(cfg.TokenName, "!") {
			parts := strings.Split(cfg.TokenName, "!")
			if len(parts) == 2 && strings.Contains(parts[0], "@") {
				userParts := strings.Split(parts[0], "@")
				if len(userParts) == 2 {
					user = userParts[0]
					realm = userParts[1]
					cfg.TokenName = parts[1]
				}
			}
		}
		if user == "" && cfg.User != "" {
			user = cfg.User
			if strings.Contains(cfg.User, "@") {
				parts := strings.Split(cfg.User, "@")
				if len(parts) == 2 {
					user = parts[0]
					realm = parts[1]
				}
			}
		}
		if realm == "" {
			realm = "pmg"
		}
	} else {
		parts := strings.Split(cfg.User, "@")
		if len(parts) == 2 {
			user = parts[0]
			realm = parts[1]
		} else {
			user = cfg.User
			realm = "pmg"
		}
	}

	httpClient := tlsutil.CreateHTTPClientWithTimeout(cfg.VerifySSL, cfg.Fingerprint, cfg.Timeout)

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

	if cfg.Password != "" && cfg.TokenName == "" {
		if err := client.authenticate(context.Background()); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	return client, nil
}

func (c *Client) authenticate(ctx context.Context) error {
	username := c.auth.user
	if username != "" && !strings.Contains(username, "@") {
		username = username + "@" + c.auth.realm
	}

	payload := map[string]string{
		"username": username,
		"password": c.config.Password,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/access/ticket", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := c.handleAuthResponse(resp); err != nil {
		if shouldFallbackToForm(err) {
			return c.authenticateForm(ctx, username, c.config.Password)
		}
		return err
	}

	return nil
}

func (c *Client) authenticateForm(ctx context.Context, username, password string) error {
	data := url.Values{
		"username": {username},
		"password": {password},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/access/ticket", strings.NewReader(data.Encode()))
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

	c.mu.Lock()
	defer c.mu.Unlock()

	c.auth.ticket = result.Data.Ticket
	c.auth.csrfToken = result.Data.CSRFPreventionToken
	c.auth.expiresAt = time.Now().Add(90 * time.Minute)

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

func (c *Client) ensureAuth(ctx context.Context) error {
	if c.config.Password == "" || c.auth.tokenName != "" {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Now().After(c.auth.expiresAt) {
		if err := c.authenticate(ctx); err != nil {
			return fmt.Errorf("re-authentication failed: %w", err)
		}
	}

	return nil
}

func (c *Client) request(ctx context.Context, method, path string, params url.Values, body io.Reader, contentType string) (*http.Response, error) {
	if err := c.ensureAuth(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	if params != nil {
		req.URL.RawQuery = params.Encode()
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	c.mu.Lock()
	tokenName := c.auth.tokenName
	tokenValue := c.auth.tokenValue
	ticket := c.auth.ticket
	csrf := c.auth.csrfToken
	user := c.auth.user
	realm := c.auth.realm
	c.mu.Unlock()

	if tokenName != "" && tokenValue != "" {
		authHeader := fmt.Sprintf("PMGAPIToken=%s@%s!%s:%s", user, realm, tokenName, tokenValue)
		req.Header.Set("Authorization", authHeader)
	} else if ticket != "" {
		req.Header.Set("Cookie", "PMGAuthCookie="+ticket)
		if method != http.MethodGet && csrf != "" {
			req.Header.Set("CSRFPreventionToken", csrf)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		apiErr := fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("authentication error: %w", apiErr)
		}
		return nil, apiErr
	}

	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, path string, params url.Values, out interface{}) error {
	resp, err := c.request(ctx, http.MethodGet, path, params, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// GetVersion returns version information for the PMG instance.
func (c *Client) GetVersion(ctx context.Context) (*VersionInfo, error) {
	var resp apiResponse[VersionInfo]
	if err := c.getJSON(ctx, "/version", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetMailStatistics(ctx context.Context, timeframe string) (*MailStatistics, error) {
	// PMG API does not accept timeframe parameter - it returns current day statistics
	// The timeframe parameter is ignored to maintain API compatibility
	var resp apiResponse[MailStatistics]
	if err := c.getJSON(ctx, "/statistics/mail", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetMailCount(ctx context.Context, timespanSeconds int) ([]MailCountEntry, error) {
	params := url.Values{}
	if timespanSeconds > 0 {
		params.Set("timespan", fmt.Sprintf("%d", timespanSeconds))
	}

	var resp apiResponse[[]MailCountEntry]
	if err := c.getJSON(ctx, "/statistics/mailcount", params, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetSpamScores(ctx context.Context) ([]SpamScore, error) {
	var resp apiResponse[[]SpamScore]
	if err := c.getJSON(ctx, "/statistics/spamscores", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetClusterStatus(ctx context.Context, listSingle bool) ([]ClusterStatusEntry, error) {
	params := url.Values{}
	if listSingle {
		params.Set("list_single_node", "1")
	}
	var resp apiResponse[[]ClusterStatusEntry]
	if err := c.getJSON(ctx, "/config/cluster/status", params, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) GetQuarantineStatus(ctx context.Context, category string) (*QuarantineStatus, error) {
	path := fmt.Sprintf("/quarantine/%sstatus", category)
	var resp apiResponse[QuarantineStatus]
	if err := c.getJSON(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetQueueStatus(ctx context.Context, node string) (*QueueStatusEntry, error) {
	path := fmt.Sprintf("/nodes/%s/postfix/queue", node)
	var resp apiResponse[QueueStatusEntry]
	if err := c.getJSON(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// ListBackups returns configuration backup archives available on a PMG node.
func (c *Client) ListBackups(ctx context.Context, node string) ([]BackupEntry, error) {
	path := fmt.Sprintf("/nodes/%s/backup", url.PathEscape(node))
	var resp apiResponse[[]BackupEntry]
	if err := c.getJSON(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}
