package notifications

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// Webhook configuration constants
const (
	// HTTP client settings
	WebhookTimeout         = 30 * time.Second
	WebhookMaxResponseSize = 1 * 1024 * 1024 // 1 MB max response size
	WebhookMaxRedirects    = 3               // Maximum number of redirects to follow
	WebhookTestTimeout     = 10 * time.Second

	// Retry settings
	WebhookInitialBackoff = 1 * time.Second
	WebhookMaxBackoff     = 30 * time.Second
	WebhookDefaultRetries = 3

	// History settings
	WebhookHistoryMaxSize = 100

	// Rate limiting settings
	WebhookRateLimitWindow = 1 * time.Minute // Time window for rate limiting
	WebhookRateLimitMax    = 10              // Max requests per window per webhook
)

const (
	queueTypeSuffixResolved = "_resolved"
	metadataResolvedAt      = "resolvedAt"
)

type notificationEvent string

const (
	eventAlert    notificationEvent = "alert"
	eventResolved notificationEvent = "resolved"
)

// createSecureWebhookClient creates an HTTP client with security controls
func (n *NotificationManager) createSecureWebhookClient(timeout time.Duration) *http.Client {
	return n.createSecureWebhookClientWithTLS(timeout, false)
}

// createSecureWebhookClientWithTLS creates a secure HTTP client with optional TLS verification override.
func (n *NotificationManager) createSecureWebhookClientWithTLS(timeout time.Duration, skipTLSVerify bool) *http.Client {
	// dedicated transport that pins DNS resolution to prevent rebinding
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Extract hostname and port
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("parse webhook address %q: %w", addr, err)
			}

			// Validate IP if it's already an IP
			if ip := net.ParseIP(host); ip != nil {
				if isPrivateIP(ip) && !n.isIPInAllowlist(ip) {
					return nil, fmt.Errorf("blocked private IP: %s", ip)
				}
				// It's an IP, dial directly
				d := net.Dialer{Timeout: 10 * time.Second}
				return d.DialContext(ctx, network, addr)
			}

			// Resolve hostname
			ips, err := net.LookupIP(host)
			if err != nil {
				return nil, fmt.Errorf("resolve webhook host %q: %w", host, err)
			}

			// Find first permitted IP
			var permittedIP net.IP
			for _, ip := range ips {
				if !isPrivateIP(ip) || n.isIPInAllowlist(ip) {
					permittedIP = ip
					break
				}
			}

			if permittedIP == nil {
				return nil, fmt.Errorf("hostname %s resolves to blocked private IPs", host)
			}

			// Log if we filtered some IPs
			if len(ips) > 1 {
				log.Debug().
					Str("host", host).
					Str("selected_ip", permittedIP.String()).
					Msg("dns resolution pinned for webhook security")
			}

			// Dial the permitted IP
			d := net.Dialer{Timeout: 10 * time.Second}
			return d.DialContext(ctx, network, net.JoinHostPort(permittedIP.String(), port))
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if skipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= WebhookMaxRedirects {
				return fmt.Errorf("stopped after %d redirects", WebhookMaxRedirects)
			}
			// Re-validate strictly on redirect
			return n.ValidateWebhookURL(req.URL.String())
		},
	}
}

// TestNodeInfo contains information about nodes for test notifications
type TestNodeInfo struct {
	NodeName    string
	InstanceURL string
}

// WebhookDelivery tracks webhook delivery attempts for debugging
type WebhookDelivery struct {
	WebhookName   string    `json:"webhookName"`
	WebhookURL    string    `json:"webhookUrl"`
	Service       string    `json:"service"`
	AlertID       string    `json:"alertId"`
	Timestamp     time.Time `json:"timestamp"`
	StatusCode    int       `json:"statusCode"`
	Success       bool      `json:"success"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
	RetryAttempts int       `json:"retryAttempts"`
	PayloadSize   int       `json:"payloadSize"`
}

// webhookRateLimit tracks rate limiting for webhook deliveries
type webhookRateLimit struct {
	lastSent  time.Time
	sentCount int
}

// NotificationManager handles sending notifications
type NotificationManager struct {
	mu                 sync.RWMutex
	emailConfig        EmailConfig
	emailManager       *EnhancedEmailManager // Shared email manager for rate limiting
	webhooks           []WebhookConfig
	appriseConfig      AppriseConfig
	enabled            bool
	cooldown           time.Duration
	notifyOnResolve    bool
	lastNotified       map[string]notificationRecord
	groupWindow        time.Duration
	pendingAlerts      []*alerts.Alert
	groupTimer         *time.Timer
	groupByNode        bool
	publicURL          string // Full URL to access Pulse
	groupByGuest       bool
	webhookHistory     []WebhookDelivery            // Keep last 100 webhook deliveries for debugging
	webhookRateLimits  map[string]*webhookRateLimit // Track rate limits per webhook URL
	webhookRateMu      sync.Mutex                   // Separate mutex for webhook rate limiting
	webhookRateCleanup time.Time                    // Last cleanup time for webhook rate limit entries
	appriseExec        appriseExecFunc
	queue              *NotificationQueue // Persistent notification queue
	webhookClient      *http.Client       // Shared HTTP client for webhooks
	stopCleanup        chan struct{}      // Signal to stop cleanup goroutine
	allowedPrivateNets []*net.IPNet       // Parsed CIDR ranges allowed for private webhook targets
	allowedPrivateMu   sync.RWMutex       // Protects allowedPrivateNets
}

// spawnAsync is used for fire-and-forget notification delivery.
// It exists so tests can disable or inline async behavior without relying on sleeps.
var spawnAsync = func(f func()) { go f() }

type appriseExecFunc func(ctx context.Context, path string, args []string) ([]byte, error)

// copyEmailConfig returns a defensive copy of EmailConfig including its slices to avoid data races.
func copyEmailConfig(cfg EmailConfig) EmailConfig {
	copy := cfg
	if len(cfg.To) > 0 {
		copy.To = append([]string(nil), cfg.To...)
	}
	return copy
}

// copyWebhookConfigs deep-copies webhook configurations to isolate concurrent writers from background senders.
func copyWebhookConfigs(webhooks []WebhookConfig) []WebhookConfig {
	if len(webhooks) == 0 {
		return nil
	}

	copies := make([]WebhookConfig, 0, len(webhooks))
	for _, webhook := range webhooks {
		clone := webhook
		if len(webhook.Headers) > 0 {
			headers := make(map[string]string, len(webhook.Headers))
			for k, v := range webhook.Headers {
				headers[k] = v
			}
			clone.Headers = headers
		}
		if len(webhook.CustomFields) > 0 {
			custom := make(map[string]string, len(webhook.CustomFields))
			for k, v := range webhook.CustomFields {
				custom[k] = v
			}
			clone.CustomFields = custom
		}
		copies = append(copies, clone)
	}

	return copies
}

func copyAppriseConfig(cfg AppriseConfig) AppriseConfig {
	copy := cfg
	if len(cfg.Targets) > 0 {
		copy.Targets = append([]string(nil), cfg.Targets...)
	}
	return copy
}

// annotateResolvedMetadata stores the resolution timestamp on the alert metadata for queue persistence.
func annotateResolvedMetadata(alert *alerts.Alert, resolvedAt time.Time) {
	if alert == nil {
		return
	}
	if alert.Metadata == nil {
		alert.Metadata = make(map[string]interface{})
	}
	alert.Metadata[metadataResolvedAt] = resolvedAt.Format(time.RFC3339)
}

// NormalizeAppriseConfig cleans and normalizes Apprise configuration values.
func NormalizeAppriseConfig(cfg AppriseConfig) AppriseConfig {
	normalized := cfg

	mode := strings.ToLower(strings.TrimSpace(string(normalized.Mode)))
	switch mode {
	case string(AppriseModeHTTP):
		normalized.Mode = AppriseModeHTTP
	default:
		normalized.Mode = AppriseModeCLI
	}

	normalized.CLIPath = "apprise" // Force default binary for security

	if normalized.TimeoutSeconds <= 0 {
		normalized.TimeoutSeconds = 15
	} else if normalized.TimeoutSeconds > 120 {
		normalized.TimeoutSeconds = 120
	} else if normalized.TimeoutSeconds < 5 {
		normalized.TimeoutSeconds = 5
	}

	cleanTargets := make([]string, 0, len(normalized.Targets))
	seen := make(map[string]struct{}, len(normalized.Targets))
	for _, target := range normalized.Targets {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		cleanTargets = append(cleanTargets, trimmed)
	}
	normalized.Targets = cleanTargets

	normalized.ServerURL = strings.TrimSpace(normalized.ServerURL)
	normalized.ServerURL = strings.TrimRight(normalized.ServerURL, "/")

	normalized.ConfigKey = strings.TrimSpace(normalized.ConfigKey)

	normalized.APIKey = strings.TrimSpace(normalized.APIKey)
	normalized.APIKeyHeader = strings.TrimSpace(normalized.APIKeyHeader)
	if normalized.APIKeyHeader == "" {
		normalized.APIKeyHeader = "X-API-KEY"
	}

	switch normalized.Mode {
	case AppriseModeCLI:
		if len(normalized.Targets) == 0 {
			normalized.Enabled = false
		}
	case AppriseModeHTTP:
		if normalized.ServerURL == "" {
			normalized.Enabled = false
		}
	}

	return normalized
}

func defaultAppriseExec(ctx context.Context, path string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	return cmd.CombinedOutput()
}

type notificationRecord struct {
	lastSent   time.Time
	alertStart time.Time
}

// Alert represents an alert (interface to avoid circular dependency)
type Alert interface {
	GetID() string
	GetResourceName() string
	GetType() string
	GetLevel() string
	GetValue() float64
	GetThreshold() float64
	GetMessage() string
	GetNode() string
	GetInstance() string
	GetStartTime() time.Time
}

// EmailConfig holds email notification settings
type EmailConfig struct {
	Enabled   bool     `json:"enabled"`
	Provider  string   `json:"provider"` // Email provider name (Gmail, SendGrid, etc.)
	SMTPHost  string   `json:"server"`   // Changed from smtpHost to server for frontend consistency
	SMTPPort  int      `json:"port"`     // Changed from smtpPort to port for frontend consistency
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	TLS       bool     `json:"tls"`
	StartTLS  bool     `json:"startTLS"`  // STARTTLS support
	RateLimit int      `json:"rateLimit"` // Max emails per minute (0 = default 60)
}

// WebhookConfig holds webhook settings
type WebhookConfig struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	Enabled      bool              `json:"enabled"`
	Service      string            `json:"service"`  // discord, slack, teams, etc.
	Template     string            `json:"template"` // Custom payload template
	CustomFields map[string]string `json:"customFields,omitempty"`
	Mention      string            `json:"mention,omitempty"` // Platform-specific mention (e.g., @everyone, @channel, <@USER_ID>)
}

// AppriseMode identifies how Pulse should deliver notifications through Apprise.
type AppriseMode string

const (
	AppriseModeCLI  AppriseMode = "cli"
	AppriseModeHTTP AppriseMode = "http"
)

const (
	defaultSMTPPort       = 587
	defaultEmailRateLimit = 60
)

// AppriseConfig holds Apprise notification settings.
type AppriseConfig struct {
	Enabled        bool        `json:"enabled"`
	Mode           AppriseMode `json:"mode,omitempty"`
	Targets        []string    `json:"targets"`
	CLIPath        string      `json:"cliPath,omitempty"`
	TimeoutSeconds int         `json:"timeoutSeconds,omitempty"`
	ServerURL      string      `json:"serverUrl,omitempty"`
	ConfigKey      string      `json:"configKey,omitempty"`
	APIKey         string      `json:"apiKey,omitempty"`
	APIKeyHeader   string      `json:"apiKeyHeader,omitempty"`
	SkipTLSVerify  bool        `json:"skipTlsVerify,omitempty"`
}

func normalizeEmailConfig(cfg EmailConfig) EmailConfig {
	normalized := cfg
	normalized.Provider = strings.TrimSpace(normalized.Provider)
	normalized.SMTPHost = strings.TrimSpace(normalized.SMTPHost)
	normalized.Username = strings.TrimSpace(normalized.Username)
	normalized.From = strings.TrimSpace(normalized.From)

	if normalized.SMTPPort <= 0 || normalized.SMTPPort > 65535 {
		log.Warn().
			Int("smtpPort", normalized.SMTPPort).
			Int("defaultPort", defaultSMTPPort).
			Msg("Invalid SMTP port in email config, using default")
		normalized.SMTPPort = defaultSMTPPort
	}

	if normalized.RateLimit < 0 {
		log.Warn().
			Int("rateLimit", normalized.RateLimit).
			Msg("Invalid negative email rate limit, using default behavior")
		normalized.RateLimit = 0
	}

	if len(normalized.To) > 0 {
		cleaned := make([]string, 0, len(normalized.To))
		seen := make(map[string]struct{}, len(normalized.To))
		for _, recipient := range normalized.To {
			trimmed := strings.TrimSpace(recipient)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			cleaned = append(cleaned, trimmed)
		}
		normalized.To = cleaned
	}

	return normalized
}

func effectiveEmailRateLimit(configured int) int {
	if configured <= 0 {
		return defaultEmailRateLimit
	}
	return configured
}

// NewNotificationManager creates a new notification manager using the global data directory.
// For multi-tenant deployments, use NewNotificationManagerWithDataDir instead.
func NewNotificationManager(publicURL string) *NotificationManager {
	return NewNotificationManagerWithDataDir(publicURL, "")
}

// NewNotificationManagerWithDataDir creates a new notification manager with a custom data directory.
// This enables tenant-scoped notification queue persistence in multi-tenant deployments.
// If dataDir is empty, it uses the global data directory.
func NewNotificationManagerWithDataDir(publicURL string, dataDir string) *NotificationManager {
	cleanURL := strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if cleanURL != "" {
		log.Info().Str("publicURL", cleanURL).Msg("notification manager initialized with public URL")
	} else {
		log.Info().Msg("notification manager initialized without public URL - webhook links may not work")
	}

	// Initialize persistent queue with tenant-specific data directory
	queue, err := NewNotificationQueue(dataDir)
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize notification queue, notifications will be in-memory only")
		queue = nil
	}

	nm := &NotificationManager{
		enabled:         true,
		cooldown:        5 * time.Minute,
		notifyOnResolve: true,
		lastNotified:    make(map[string]notificationRecord),
		webhooks:        []WebhookConfig{},
		appriseConfig: AppriseConfig{
			Enabled:        false,
			Mode:           AppriseModeCLI,
			Targets:        []string{},
			CLIPath:        "apprise",
			TimeoutSeconds: 15,
			APIKeyHeader:   "X-API-KEY",
		},
		groupWindow:        30 * time.Second,
		pendingAlerts:      make([]*alerts.Alert, 0),
		groupByNode:        true,
		groupByGuest:       false,
		webhookHistory:     make([]WebhookDelivery, 0, WebhookHistoryMaxSize),
		webhookRateLimits:  make(map[string]*webhookRateLimit),
		webhookRateCleanup: time.Now(),
		publicURL:          cleanURL,
		appriseExec:        defaultAppriseExec,
		queue:              queue,
		stopCleanup:        make(chan struct{}),
	}

	// Create webhook client after NotificationManager is initialized
	nm.webhookClient = nm.createSecureWebhookClient(WebhookTimeout)

	// Wire up queue processor if queue is available
	if queue != nil {
		queue.SetProcessor(nm.ProcessQueuedNotification)
	}

	// Start periodic cleanup of old lastNotified entries (every 1 hour)
	go nm.cleanupOldNotificationRecords()

	return nm
}

// SetPublicURL updates the public URL used for webhook payloads.
func (n *NotificationManager) SetPublicURL(publicURL string) {
	trimmed := strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if trimmed == "" {
		return
	}

	n.mu.Lock()
	if n.publicURL == trimmed {
		n.mu.Unlock()
		return
	}
	n.publicURL = trimmed
	n.mu.Unlock()

	log.Info().Str("publicURL", trimmed).Msg("notification manager public URL updated")
}

// GetPublicURL returns the configured public URL for notifications.
func (n *NotificationManager) GetPublicURL() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.publicURL
}

// SetEmailConfig updates email configuration
func (n *NotificationManager) SetEmailConfig(config EmailConfig) {
	n.mu.Lock()
	defer n.mu.Unlock()
	config = normalizeEmailConfig(config)
	n.emailConfig = config

	// Recreate email manager with new config to preserve rate limiting state
	rateLimit := effectiveEmailRateLimit(config.RateLimit)
	providerConfig := EmailProviderConfig{
		EmailConfig:   config,
		Provider:      "",
		MaxRetries:    3,
		RetryDelay:    5,
		RateLimit:     rateLimit,
		StartTLS:      config.StartTLS,
		SkipTLSVerify: false,
		AuthRequired:  config.Username != "" && config.Password != "",
	}
	n.emailManager = NewEnhancedEmailManager(providerConfig)
}

// SetAppriseConfig updates Apprise configuration.
func (n *NotificationManager) SetAppriseConfig(config AppriseConfig) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.appriseConfig = NormalizeAppriseConfig(config)
}

// GetAppriseConfig returns a copy of the Apprise configuration.
func (n *NotificationManager) GetAppriseConfig() AppriseConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return copyAppriseConfig(n.appriseConfig)
}

// SetCooldown updates the cooldown duration
func (n *NotificationManager) SetCooldown(minutes int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if minutes < 0 {
		minutes = 0
	}
	n.cooldown = time.Duration(minutes) * time.Minute
	log.Info().Int("minutes", minutes).Msg("updated notification cooldown")
}

// SetNotifyOnResolve toggles whether resolved alerts send notifications.
func (n *NotificationManager) SetNotifyOnResolve(enabled bool) {
	n.mu.Lock()
	was := n.notifyOnResolve
	n.notifyOnResolve = enabled
	n.mu.Unlock()

	if was != enabled {
		log.Info().Bool("enabled", enabled).Msg("updated resolved alert notifications")
	}
}

// GetNotifyOnResolve returns whether resolved alerts trigger notifications.
func (n *NotificationManager) GetNotifyOnResolve() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.notifyOnResolve
}

// SetGroupingWindow updates the grouping window duration
func (n *NotificationManager) SetGroupingWindow(seconds int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if seconds < 0 {
		seconds = 0
	}
	n.groupWindow = time.Duration(seconds) * time.Second
	log.Info().Int("seconds", seconds).Msg("updated notification grouping window")
}

// SetGroupingOptions updates grouping options
func (n *NotificationManager) SetGroupingOptions(byNode, byGuest bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.groupByNode = byNode
	n.groupByGuest = byGuest
	log.Info().Bool("byNode", byNode).Bool("byGuest", byGuest).Msg("updated notification grouping options")
}

// AddWebhook adds a webhook configuration
func (n *NotificationManager) AddWebhook(webhook WebhookConfig) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.webhooks = append(n.webhooks, webhook)
}

// UpdateWebhook updates an existing webhook
func (n *NotificationManager) UpdateWebhook(webhookID string, webhook WebhookConfig) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, w := range n.webhooks {
		if w.ID == webhookID {
			n.webhooks[i] = webhook
			return nil
		}
	}
	return fmt.Errorf("webhook not found: %s", webhookID)
}

// DeleteWebhook removes a webhook
func (n *NotificationManager) DeleteWebhook(webhookID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, w := range n.webhooks {
		if w.ID == webhookID {
			n.webhooks = append(n.webhooks[:i], n.webhooks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("webhook not found: %s", webhookID)
}

// GetWebhooks returns all webhook configurations
func (n *NotificationManager) GetWebhooks() []WebhookConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if len(n.webhooks) == 0 {
		return []WebhookConfig{}
	}

	webhooks := make([]WebhookConfig, len(n.webhooks))
	copy(webhooks, n.webhooks)
	return webhooks
}

// GetEmailConfig returns the email configuration
func (n *NotificationManager) GetEmailConfig() EmailConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.emailConfig
}

// GetQueue returns the notification queue
func (n *NotificationManager) GetQueue() *NotificationQueue {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.queue
}

// SendAlert sends notifications for an alert
func (n *NotificationManager) SendAlert(alert *alerts.Alert) {
	n.mu.Lock()
	defer n.mu.Unlock()

	log.Info().
		Str("alertID", alert.ID).
		Bool("enabled", n.enabled).
		Int("webhooks", len(n.webhooks)).
		Bool("emailEnabled", n.emailConfig.Enabled).
		Msg("send alert called")

	if !n.enabled {
		log.Debug().Msg("notifications disabled, skipping")
		return
	}

	// Check cooldown
	record, exists := n.lastNotified[alert.ID]
	if exists && record.alertStart.Equal(alert.StartTime) && time.Since(record.lastSent) < n.cooldown {
		log.Info().
			Str("alertID", alert.ID).
			Str("resourceName", alert.ResourceName).
			Str("type", alert.Type).
			Dur("timeSince", time.Since(record.lastSent)).
			Dur("cooldown", n.cooldown).
			Dur("remainingCooldown", n.cooldown-time.Since(record.lastSent)).
			Msg("alert notification in cooldown for active alert - notification suppressed")
		return
	}

	log.Info().
		Str("alertID", alert.ID).
		Str("resourceName", alert.ResourceName).
		Str("type", alert.Type).
		Float64("value", alert.Value).
		Float64("threshold", alert.Threshold).
		Bool("inCooldown", exists).
		Msg("alert passed cooldown check - adding to pending notifications")

	// Add to pending alerts for grouping
	n.pendingAlerts = append(n.pendingAlerts, alert)

	// If this is the first alert in the group, start the timer
	if n.groupTimer == nil {
		n.groupTimer = time.AfterFunc(n.groupWindow, func() {
			n.sendGroupedAlerts()
		})
		log.Debug().
			Int("pendingCount", len(n.pendingAlerts)).
			Dur("groupWindow", n.groupWindow).
			Msg("started alert grouping timer")
	}
}

// SendResolvedAlert delivers notifications for a resolved alert immediately.
func (n *NotificationManager) SendResolvedAlert(resolved *alerts.ResolvedAlert) {
	if resolved == nil || resolved.Alert == nil {
		return
	}

	// Clone the alert so downstream goroutines cannot mutate shared state.
	alertCopy := resolved.Alert.Clone()
	if alertCopy == nil {
		return
	}

	resolvedAt := resolved.ResolvedTime
	if resolvedAt.IsZero() {
		resolvedAt = time.Now()
	}
	annotateResolvedMetadata(alertCopy, resolvedAt)

	n.mu.RLock()
	enabled := n.enabled && n.notifyOnResolve
	emailConfig := copyEmailConfig(n.emailConfig)
	webhooks := copyWebhookConfigs(n.webhooks)
	appriseConfig := copyAppriseConfig(n.appriseConfig)
	queue := n.queue
	n.mu.RUnlock()

	if !enabled {
		log.Debug().
			Str("alertID", alertCopy.ID).
			Msg("resolved notifications disabled, skipping")
		return
	}

	alertsToSend := []*alerts.Alert{alertCopy}

	if queue != nil {
		n.enqueueResolvedNotifications(queue, emailConfig, webhooks, appriseConfig, alertsToSend, resolvedAt)
	} else {
		n.sendResolvedNotificationsDirect(emailConfig, webhooks, appriseConfig, alertsToSend, resolvedAt)
	}
}

// CancelAlert removes pending notifications for a resolved alert
func (n *NotificationManager) CancelAlert(alertID string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.pendingAlerts) == 0 {
		return
	}

	filtered := n.pendingAlerts[:0]
	removed := 0
	for _, pending := range n.pendingAlerts {
		if pending == nil {
			continue
		}
		if pending.ID == alertID {
			removed++
			continue
		}
		filtered = append(filtered, pending)
	}

	if removed == 0 {
		return
	}

	for i := len(filtered); i < len(n.pendingAlerts); i++ {
		n.pendingAlerts[i] = nil
	}

	n.pendingAlerts = filtered

	if len(n.pendingAlerts) == 0 && n.groupTimer != nil {
		if n.groupTimer.Stop() {
			log.Debug().Str("alertID", alertID).Msg("stopped grouping timer after alert cancellation")
		}
		n.groupTimer = nil
	}

	// Clean up cooldown record for resolved alert
	delete(n.lastNotified, alertID)

	// Cancel any queued notifications containing this alert
	if n.queue != nil {
		if err := n.queue.CancelByAlertIDs([]string{alertID}); err != nil {
			log.Error().Err(err).Str("alertID", alertID).Msg("failed to cancel queued notifications")
		}
	}

	log.Debug().
		Str("alertID", alertID).
		Int("remaining", len(n.pendingAlerts)).
		Msg("removed resolved alert from pending notifications and cooldown map")
}

// sendGroupedAlerts sends all pending alerts as a group
func (n *NotificationManager) sendGroupedAlerts() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.pendingAlerts) == 0 {
		return
	}

	// Copy alerts to send
	alertsToSend := make([]*alerts.Alert, len(n.pendingAlerts))
	copy(alertsToSend, n.pendingAlerts)

	// Clear pending alerts
	n.pendingAlerts = n.pendingAlerts[:0]
	n.groupTimer = nil

	log.Info().
		Int("alertCount", len(alertsToSend)).
		Msg("sending grouped alert notifications")

	// Snapshot configuration while holding the lock to avoid races with concurrent updates
	emailConfig := copyEmailConfig(n.emailConfig)
	webhooks := copyWebhookConfigs(n.webhooks)
	appriseConfig := copyAppriseConfig(n.appriseConfig)

	// Use persistent queue if available, otherwise send directly
	if n.queue != nil {
		n.enqueueNotifications(emailConfig, webhooks, appriseConfig, alertsToSend)
		// Note: Cooldown will be marked after successful dequeue and send
	} else {
		n.sendNotificationsDirect(emailConfig, webhooks, appriseConfig, alertsToSend)
		// For direct sends, mark cooldown immediately (fire-and-forget)
		now := time.Now()
		for _, alert := range alertsToSend {
			n.lastNotified[alert.ID] = notificationRecord{
				lastSent:   now,
				alertStart: alert.StartTime,
			}
		}
	}
}

// enqueueNotifications adds notifications to the persistent queue
// Falls back to direct sending if enqueue fails
func (n *NotificationManager) enqueueNotifications(emailConfig EmailConfig, webhooks []WebhookConfig, appriseConfig AppriseConfig, alertsToSend []*alerts.Alert) {
	anyFailed := false

	// Enqueue email notification
	if emailConfig.Enabled {
		configJSON, err := json.Marshal(emailConfig)
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal email config for queue")
		} else {
			notif := &QueuedNotification{
				Type:        "email",
				Alerts:      alertsToSend,
				Config:      configJSON,
				MaxAttempts: 3,
			}
			if err := n.queue.Enqueue(notif); err != nil {
				log.Error().Err(err).Msg("failed to enqueue email notification - falling back to direct send")
				anyFailed = true
				spawnAsync(func() {
					if sendErr := n.sendGroupedEmail(emailConfig, alertsToSend); sendErr != nil {
						log.Error().Err(sendErr).Msg("Failed to send grouped email notification after queue enqueue failure")
					}
				})
			} else {
				log.Debug().Int("alertCount", len(alertsToSend)).Msg("enqueued email notification")
			}
		}
	}

	// Enqueue webhook notifications
	for _, webhook := range webhooks {
		if webhook.Enabled {
			configJSON, err := json.Marshal(webhook)
			if err != nil {
				log.Error().Err(err).Str("webhookName", webhook.Name).Msg("failed to marshal webhook config for queue")
			} else {
				notif := &QueuedNotification{
					Type:        "webhook",
					Alerts:      alertsToSend,
					Config:      configJSON,
					MaxAttempts: 3,
				}
				if err := n.queue.Enqueue(notif); err != nil {
					log.Error().Err(err).Str("webhookName", webhook.Name).Msg("failed to enqueue webhook notification - falling back to direct send")
					anyFailed = true
					webhookCopy := webhook
					spawnAsync(func() {
						if sendErr := n.sendGroupedWebhook(webhookCopy, alertsToSend); sendErr != nil {
							log.Error().
								Err(sendErr).
								Str("webhookName", webhookCopy.Name).
								Msg("Failed to send grouped webhook notification after queue enqueue failure")
						}
					})
				} else {
					log.Debug().Str("webhookName", webhook.Name).Int("alertCount", len(alertsToSend)).Msg("enqueued webhook notification")
				}
			}
		}
	}

	// Enqueue apprise notification
	if appriseConfig.Enabled {
		configJSON, err := json.Marshal(appriseConfig)
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal apprise config for queue")
		} else {
			notif := &QueuedNotification{
				Type:        "apprise",
				Alerts:      alertsToSend,
				Config:      configJSON,
				MaxAttempts: 3,
			}
			if err := n.queue.Enqueue(notif); err != nil {
				log.Error().Err(err).Msg("failed to enqueue apprise notification - falling back to direct send")
				anyFailed = true
				spawnAsync(func() {
					if sendErr := n.sendGroupedApprise(appriseConfig, alertsToSend); sendErr != nil {
						log.Error().Err(sendErr).Msg("Failed to send grouped Apprise notification after queue enqueue failure")
					}
				})
			} else {
				log.Debug().Int("alertCount", len(alertsToSend)).Msg("enqueued apprise notification")
			}
		}
	}

	// If any enqueue failed, mark cooldown immediately for fire-and-forget sends
	if anyFailed {
		n.mu.Lock()
		now := time.Now()
		for _, alert := range alertsToSend {
			n.lastNotified[alert.ID] = notificationRecord{
				lastSent:   now,
				alertStart: alert.StartTime,
			}
		}
		n.mu.Unlock()
	}
}

// enqueueResolvedNotifications adds resolved notifications to the persistent queue.
func (n *NotificationManager) enqueueResolvedNotifications(queue *NotificationQueue, emailConfig EmailConfig, webhooks []WebhookConfig, appriseConfig AppriseConfig, alertsToSend []*alerts.Alert, resolvedAt time.Time) {
	if queue == nil {
		return
	}

	anyFailed := false

	if emailConfig.Enabled {
		configJSON, err := json.Marshal(emailConfig)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal email config for resolved queue")
		} else {
			notif := &QueuedNotification{
				Type:        "email" + queueTypeSuffixResolved,
				Alerts:      alertsToSend,
				Config:      configJSON,
				MaxAttempts: 3,
			}
			if err := queue.Enqueue(notif); err != nil {
				log.Error().Err(err).Msg("Failed to enqueue resolved email notification - falling back to direct send")
				anyFailed = true
				spawnAsync(func() {
					if sendErr := n.sendResolvedEmail(emailConfig, alertsToSend, resolvedAt); sendErr != nil {
						log.Error().Err(sendErr).Msg("Failed to send resolved email notification after queue enqueue failure")
					}
				})
			} else {
				log.Debug().Int("alertCount", len(alertsToSend)).Msg("enqueued resolved email notification")
			}
		}
	}

	for _, webhook := range webhooks {
		if !webhook.Enabled {
			continue
		}
		webhookCopy := webhook
		configJSON, err := json.Marshal(webhookCopy)
		if err != nil {
			log.Error().Err(err).Str("webhookName", webhookCopy.Name).Msg("failed to marshal webhook config for resolved queue")
			continue
		}
		notif := &QueuedNotification{
			Type:        "webhook" + queueTypeSuffixResolved,
			Alerts:      alertsToSend,
			Config:      configJSON,
			MaxAttempts: 3,
		}
		if err := queue.Enqueue(notif); err != nil {
			log.Error().Err(err).Str("webhookName", webhookCopy.Name).Msg("Failed to enqueue resolved webhook notification - falling back to direct send")
			anyFailed = true
			webhookCopy2 := webhookCopy
			spawnAsync(func() {
				if sendErr := n.sendResolvedWebhook(webhookCopy2, alertsToSend, resolvedAt); sendErr != nil {
					log.Error().
						Err(sendErr).
						Str("webhookName", webhookCopy2.Name).
						Msg("Failed to send resolved webhook notification after queue enqueue failure")
				}
			})
		} else {
			log.Debug().Str("webhookName", webhookCopy.Name).Int("alertCount", len(alertsToSend)).Msg("enqueued resolved webhook notification")
		}
	}

	if appriseConfig.Enabled {
		configJSON, err := json.Marshal(appriseConfig)
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal apprise config for resolved queue")
		} else {
			notif := &QueuedNotification{
				Type:        "apprise" + queueTypeSuffixResolved,
				Alerts:      alertsToSend,
				Config:      configJSON,
				MaxAttempts: 3,
			}
			if err := queue.Enqueue(notif); err != nil {
				log.Error().Err(err).Msg("failed to enqueue resolved Apprise notification - falling back to direct send")
				anyFailed = true
				spawnAsync(func() {
					if sendErr := n.sendResolvedApprise(appriseConfig, alertsToSend, resolvedAt); sendErr != nil {
						log.Error().Err(sendErr).Msg("Failed to send resolved Apprise notification after queue enqueue failure")
					}
				})
			} else {
				log.Debug().Int("alertCount", len(alertsToSend)).Msg("enqueued resolved Apprise notification")
			}
		}
	}

	if anyFailed {
		log.Debug().Msg("at least one resolved notification enqueue failed; direct sends were triggered")
	}
}

// sendNotificationsDirect sends notifications without using the queue (fallback)
func (n *NotificationManager) sendNotificationsDirect(emailConfig EmailConfig, webhooks []WebhookConfig, appriseConfig AppriseConfig, alertsToSend []*alerts.Alert) {
	// Send notifications using the captured snapshots outside the lock to avoid blocking writers
	if emailConfig.Enabled {
		log.Info().
			Int("alertCount", len(alertsToSend)).
			Str("smtpHost", emailConfig.SMTPHost).
			Int("smtpPort", emailConfig.SMTPPort).
			Strs("recipients", emailConfig.To).
			Bool("hasAuth", emailConfig.Username != "" && emailConfig.Password != "").
			Msg("Email notifications enabled - sending grouped email")
		spawnAsync(func() {
			if err := n.sendGroupedEmail(emailConfig, alertsToSend); err != nil {
				log.Error().Err(err).Msg("Failed to send grouped email notification")
			}
		})
	} else {
		log.Debug().
			Int("alertCount", len(alertsToSend)).
			Msg("email notifications disabled - skipping email delivery")
	}

	for _, webhook := range webhooks {
		if webhook.Enabled {
			webhookCopy := webhook
			spawnAsync(func() {
				if err := n.sendGroupedWebhook(webhookCopy, alertsToSend); err != nil {
					log.Error().
						Err(err).
						Str("webhookName", webhookCopy.Name).
						Msg("Failed to send grouped webhook notification")
				}
			})
		}
	}

	if appriseConfig.Enabled {
		spawnAsync(func() {
			if err := n.sendGroupedApprise(appriseConfig, alertsToSend); err != nil {
				log.Error().Err(err).Msg("Failed to send grouped Apprise notification")
			}
		})
	}
}

// sendResolvedNotificationsDirect delivers resolved notifications without queue persistence.
func (n *NotificationManager) sendResolvedNotificationsDirect(emailConfig EmailConfig, webhooks []WebhookConfig, appriseConfig AppriseConfig, alertsToSend []*alerts.Alert, resolvedAt time.Time) {
	if len(alertsToSend) == 0 {
		return
	}

	if emailConfig.Enabled {
		spawnAsync(func() {
			if err := n.sendResolvedEmail(emailConfig, alertsToSend, resolvedAt); err != nil {
				log.Error().Err(err).Msg("failed to send resolved email notification")
			}
		})
	}

	for _, webhook := range webhooks {
		if !webhook.Enabled {
			continue
		}
		webhookCopy := webhook
		spawnAsync(func() {
			if err := n.sendResolvedWebhook(webhookCopy, alertsToSend, resolvedAt); err != nil {
				log.Error().
					Err(err).
					Str("webhookName", webhookCopy.Name).
					Msg("failed to send resolved webhook notification")
			}
		})
	}

	if appriseConfig.Enabled {
		spawnAsync(func() {
			if err := n.sendResolvedApprise(appriseConfig, alertsToSend, resolvedAt); err != nil {
				log.Error().Err(err).Msg("failed to send resolved Apprise notification")
			}
		})
	}
}

// sendGroupedEmail sends a grouped email notification
func (n *NotificationManager) sendGroupedEmail(config EmailConfig, alertList []*alerts.Alert) error {

	// Don't check for recipients here - sendHTMLEmail handles empty recipients
	// by using the From address as the recipient

	// Generate email using template
	subject, htmlBody, textBody := EmailTemplate(alertList, false)

	// Send using HTML-aware method
	return n.sendHTMLEmailWithError(subject, htmlBody, textBody, config)
}

func (n *NotificationManager) sendResolvedEmail(config EmailConfig, alertList []*alerts.Alert, resolvedAt time.Time) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	subject, htmlBody, textBody := buildResolvedNotificationContent(alertList, resolvedAt, n.publicURL)
	if subject == "" && textBody == "" {
		return fmt.Errorf("failed to build resolved email content")
	}

	return n.sendHTMLEmailWithError(subject, htmlBody, textBody, config)
}

func (n *NotificationManager) sendGroupedApprise(config AppriseConfig, alertList []*alerts.Alert) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	cfg := NormalizeAppriseConfig(config)
	if !cfg.Enabled {
		return fmt.Errorf("apprise not enabled")
	}

	title, body, notifyType := buildApprisePayload(alertList, n.publicURL)
	if title == "" && body == "" {
		return fmt.Errorf("failed to build apprise payload")
	}

	switch cfg.Mode {
	case AppriseModeHTTP:
		if err := n.sendAppriseViaHTTP(cfg, title, body, notifyType); err != nil {
			log.Warn().
				Err(err).
				Str("mode", string(cfg.Mode)).
				Str("serverUrl", cfg.ServerURL).
				Msg("failed to send Apprise notification via API")
			return fmt.Errorf("apprise HTTP send failed: %w", err)
		}
	default:
		if err := n.sendAppriseViaCLI(cfg, title, body); err != nil {
			log.Warn().
				Err(err).
				Str("mode", string(cfg.Mode)).
				Str("cliPath", cfg.CLIPath).
				Strs("targets", cfg.Targets).
				Msg("failed to send Apprise notification")
			return fmt.Errorf("apprise CLI send failed: %w", err)
		}
	}
	return nil
}

func buildApprisePayload(alertList []*alerts.Alert, publicURL string) (string, string, string) {
	validAlerts := make([]*alerts.Alert, 0, len(alertList))
	var primary *alerts.Alert
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		if primary == nil {
			primary = alert
		}
		validAlerts = append(validAlerts, alert)
	}

	if len(validAlerts) == 0 || primary == nil {
		return "", "", "info"
	}

	title := fmt.Sprintf("Pulse alert: %s", primary.ResourceName)
	if len(validAlerts) > 1 {
		title = fmt.Sprintf("Pulse alerts (%d)", len(validAlerts))
	}

	var bodyBuilder strings.Builder
	bodyBuilder.WriteString(primary.Message)
	bodyBuilder.WriteString("\n\n")

	for _, alert := range validAlerts {
		bodyBuilder.WriteString(fmt.Sprintf("[%s] %s", strings.ToUpper(string(alert.Level)), alert.ResourceName))
		bodyBuilder.WriteString(fmt.Sprintf(" — value %.2f (threshold %.2f)\n", alert.Value, alert.Threshold))
		if alert.Node != "" {
			bodyBuilder.WriteString(fmt.Sprintf("Node: %s\n", alertNodeDisplay(alert)))
		}
		if alert.Instance != "" && alert.Instance != alert.Node {
			bodyBuilder.WriteString(fmt.Sprintf("Instance: %s\n", alert.Instance))
		}
		bodyBuilder.WriteString("\n")
	}

	if publicURL != "" {
		bodyBuilder.WriteString("Dashboard: " + publicURL + "\n")
	}

	return title, bodyBuilder.String(), resolveAppriseNotificationType(validAlerts)
}

func buildResolvedNotificationContent(alertList []*alerts.Alert, resolvedAt time.Time, publicURL string) (string, string, string) {
	validAlerts := make([]*alerts.Alert, 0, len(alertList))
	var primary *alerts.Alert
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		if primary == nil {
			primary = alert
		}
		validAlerts = append(validAlerts, alert)
	}

	if len(validAlerts) == 0 || primary == nil {
		return "", "", ""
	}

	if resolvedAt.IsZero() {
		resolvedAt = time.Now()
	}
	resolvedLabel := resolvedAt.Format(time.RFC3339)

	title := fmt.Sprintf("Pulse alert resolved: %s", primary.ResourceName)
	if len(validAlerts) > 1 {
		title = fmt.Sprintf("Pulse alerts resolved (%d)", len(validAlerts))
	}

	var bodyBuilder strings.Builder
	bodyBuilder.WriteString("Resolved at ")
	bodyBuilder.WriteString(resolvedLabel)
	bodyBuilder.WriteString("\n\n")

	for _, alert := range validAlerts {
		bodyBuilder.WriteString(fmt.Sprintf("[%s] %s\n", strings.ToUpper(string(alert.Level)), alert.ResourceName))
		if alert.Message != "" {
			bodyBuilder.WriteString(alert.Message)
			bodyBuilder.WriteString("\n")
		}
		if !alert.StartTime.IsZero() {
			bodyBuilder.WriteString("Started: ")
			bodyBuilder.WriteString(alert.StartTime.Format(time.RFC3339))
			bodyBuilder.WriteString("\n")
		}
		bodyBuilder.WriteString("Cleared: ")
		bodyBuilder.WriteString(resolvedLabel)
		bodyBuilder.WriteString("\n")
		if alert.Node != "" {
			bodyBuilder.WriteString("Node: ")
			bodyBuilder.WriteString(alertNodeDisplay(alert))
			bodyBuilder.WriteString("\n")
		}
		if alert.Instance != "" && alert.Instance != alert.Node {
			bodyBuilder.WriteString("Instance: ")
			bodyBuilder.WriteString(alert.Instance)
			bodyBuilder.WriteString("\n")
		}
		if alert.Threshold != 0 || alert.Value != 0 {
			bodyBuilder.WriteString(fmt.Sprintf("Last value %.2f (threshold %.2f)\n", alert.Value, alert.Threshold))
		}
		bodyBuilder.WriteString("\n")
	}

	if publicURL != "" {
		bodyBuilder.WriteString("Dashboard: ")
		bodyBuilder.WriteString(publicURL)
		bodyBuilder.WriteString("\n")
	}

	textBody := bodyBuilder.String()
	htmlBody := "<pre style=\"font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, \\\"Liberation Mono\\\", \\\"Courier New\\\", monospace\">" +
		html.EscapeString(textBody) + "</pre>"

	return title, htmlBody, textBody
}

func resolveAppriseNotificationType(alertList []*alerts.Alert) string {
	notifyType := "info"
	for _, alert := range alertList {
		if alert == nil {
			continue
		}
		switch alert.Level {
		case alerts.AlertLevelCritical:
			return "failure"
		case alerts.AlertLevelWarning:
			notifyType = "warning"
		}
	}
	return notifyType
}

func (n *NotificationManager) sendAppriseViaCLI(cfg AppriseConfig, title, body string) error {
	if len(cfg.Targets) == 0 {
		return fmt.Errorf("no Apprise targets configured for CLI delivery")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	args := []string{"-t", title, "-b", body}
	args = append(args, cfg.Targets...)

	execFn := n.appriseExec
	if execFn == nil {
		execFn = defaultAppriseExec
	}

	output, err := execFn(ctx, cfg.CLIPath, args)
	if err != nil {
		if len(output) > 0 {
			log.Debug().
				Str("cliPath", cfg.CLIPath).
				Strs("targets", cfg.Targets).
				Str("output", string(output)).
				Msg("apprise CLI output (error)")
		}
		return fmt.Errorf("execute apprise CLI %q: %w", cfg.CLIPath, err)
	}

	if len(output) > 0 {
		log.Debug().
			Str("cliPath", cfg.CLIPath).
			Strs("targets", cfg.Targets).
			Str("output", string(output)).
			Msg("apprise CLI output")
	}
	return nil
}

func (n *NotificationManager) sendAppriseViaHTTP(cfg AppriseConfig, title, body, notifyType string) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("apprise server URL is not configured")
	}

	serverURL := cfg.ServerURL
	lowerURL := strings.ToLower(serverURL)
	if !strings.HasPrefix(lowerURL, "http://") && !strings.HasPrefix(lowerURL, "https://") {
		return fmt.Errorf("apprise server URL must start with http or https: %s", serverURL)
	}

	// Validate Apprise server URL to prevent SSRF
	if err := n.ValidateWebhookURL(serverURL); err != nil {
		log.Error().
			Err(err).
			Str("serverURL", serverURL).
			Msg("apprise server URL validation failed - possible SSRF attempt")
		return fmt.Errorf("apprise server URL validation failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	notifyEndpoint := "/notify"
	if cfg.ConfigKey != "" {
		notifyEndpoint = "/notify/" + url.PathEscape(cfg.ConfigKey)
	}

	requestURL := strings.TrimRight(serverURL, "/") + notifyEndpoint

	payload := map[string]any{
		"body":  body,
		"title": title,
	}
	if len(cfg.Targets) > 0 {
		payload["urls"] = cfg.Targets
	}
	if notifyType != "" {
		payload["type"] = notifyType
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Apprise payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create Apprise request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if cfg.APIKey != "" {
		if cfg.APIKeyHeader == "" {
			req.Header.Set("X-API-KEY", cfg.APIKey)
		} else {
			req.Header.Set(cfg.APIKeyHeader, cfg.APIKey)
		}
	}

	client := n.createSecureWebhookClientWithTLS(
		time.Duration(cfg.TimeoutSeconds)*time.Second,
		strings.HasPrefix(lowerURL, "https://") && cfg.SkipTLSVerify,
	)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach Apprise server: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, WebhookMaxResponseSize)
	respBody, _ := io.ReadAll(limited)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if len(respBody) > 0 {
			return fmt.Errorf("apprise server returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		}
		return fmt.Errorf("apprise server returned HTTP %d", resp.StatusCode)
	}

	if len(respBody) > 0 {
		log.Debug().
			Str("mode", string(cfg.Mode)).
			Str("serverUrl", cfg.ServerURL).
			Str("response", string(respBody)).
			Msg("apprise API response")
	}

	return nil
}

func (n *NotificationManager) sendResolvedApprise(config AppriseConfig, alertList []*alerts.Alert, resolvedAt time.Time) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	cfg := NormalizeAppriseConfig(config)
	if !cfg.Enabled {
		return fmt.Errorf("apprise not enabled")
	}

	title, _, body := buildResolvedNotificationContent(alertList, resolvedAt, n.publicURL)
	if title == "" && body == "" {
		return fmt.Errorf("failed to build resolved apprise payload")
	}

	switch cfg.Mode {
	case AppriseModeHTTP:
		if err := n.sendAppriseViaHTTP(cfg, title, body, "info"); err != nil {
			log.Warn().
				Err(err).
				Str("mode", string(cfg.Mode)).
				Str("serverUrl", cfg.ServerURL).
				Msg("failed to send resolved Apprise notification via API")
			return fmt.Errorf("apprise HTTP send failed: %w", err)
		}
	default:
		if err := n.sendAppriseViaCLI(cfg, title, body); err != nil {
			log.Warn().
				Err(err).
				Str("mode", string(cfg.Mode)).
				Str("cliPath", cfg.CLIPath).
				Strs("targets", cfg.Targets).
				Msg("failed to send resolved Apprise notification")
			return fmt.Errorf("apprise CLI send failed: %w", err)
		}
	}
	return nil
}

// sendEmail sends an email notification
func (n *NotificationManager) sendEmail(alert *alerts.Alert) {
	n.mu.RLock()
	config := n.emailConfig
	n.mu.RUnlock()

	// Don't check for recipients here - sendHTMLEmail handles empty recipients
	// by using the From address as the recipient

	// Generate email using template
	subject, htmlBody, textBody := EmailTemplate([]*alerts.Alert{alert}, true)

	// Send using HTML-aware method
	n.sendHTMLEmail(subject, htmlBody, textBody, config)
}

// sendHTMLEmailWithError sends an HTML email with multipart content and returns any error
func (n *NotificationManager) sendHTMLEmailWithError(subject, htmlBody, textBody string, config EmailConfig) error {
	config = normalizeEmailConfig(config)

	// Use From address as recipient if To is empty
	recipients := config.To
	if len(recipients) == 0 && config.From != "" {
		recipients = []string{config.From}
		log.Info().
			Str("from", config.From).
			Msg("using From address as recipient since To is empty")
	}

	// Use shared email manager for rate limiting, or create a new one if not available
	n.mu.RLock()
	manager := n.emailManager
	n.mu.RUnlock()

	if manager == nil {
		// Create email manager if not yet initialized
		rl := effectiveEmailRateLimit(config.RateLimit)
		enhancedConfig := EmailProviderConfig{
			EmailConfig: EmailConfig{
				From:     config.From,
				To:       recipients,
				SMTPHost: config.SMTPHost,
				SMTPPort: config.SMTPPort,
				Username: config.Username,
				Password: config.Password,
			},
			Provider:      config.Provider,
			StartTLS:      config.StartTLS,
			MaxRetries:    2,
			RetryDelay:    3,
			RateLimit:     rl,
			SkipTLSVerify: false,
			AuthRequired:  config.Username != "" && config.Password != "",
		}
		manager = NewEnhancedEmailManager(enhancedConfig)
	} else {
		// Update manager config while preserving accumulated rate limiter state.
		manager.config.EmailConfig = EmailConfig{
			From:     config.From,
			To:       recipients,
			SMTPHost: config.SMTPHost,
			SMTPPort: config.SMTPPort,
			Username: config.Username,
			Password: config.Password,
			TLS:      config.TLS,
			StartTLS: config.StartTLS,
		}
		manager.config.Provider = config.Provider
		manager.config.StartTLS = config.StartTLS
		manager.config.RateLimit = effectiveEmailRateLimit(config.RateLimit)
		manager.config.AuthRequired = config.Username != "" && config.Password != ""

		if manager.rateLimit != nil {
			manager.rateLimit.mu.Lock()
			manager.rateLimit.rate = manager.config.RateLimit
			manager.rateLimit.mu.Unlock()
		}
	}

	log.Info().
		Str("smtp", fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)).
		Str("from", config.From).
		Strs("to", recipients).
		Bool("hasAuth", config.Username != "" && config.Password != "").
		Bool("startTLS", manager.config.StartTLS).
		Msg("attempting to send email via SMTP with enhanced support")

	err := manager.SendEmailWithRetry(subject, htmlBody, textBody)

	if err != nil {
		log.Error().
			Err(err).
			Str("smtp", fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)).
			Strs("recipients", recipients).
			Msg("failed to send email notification")
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Info().
		Strs("recipients", recipients).
		Int("recipientCount", len(recipients)).
		Msg("email notification sent successfully")
	return nil
}

// sendHTMLEmail sends an HTML email with multipart content
func (n *NotificationManager) sendHTMLEmail(subject, htmlBody, textBody string, config EmailConfig) {
	config = normalizeEmailConfig(config)

	// Use From address as recipient if To is empty
	recipients := config.To
	if len(recipients) == 0 && config.From != "" {
		recipients = []string{config.From}
		log.Info().
			Str("from", config.From).
			Msg("using From address as recipient since To is empty")
	}

	// Create enhanced email configuration with proper STARTTLS support
	rl := effectiveEmailRateLimit(config.RateLimit)
	enhancedConfig := EmailProviderConfig{
		EmailConfig: EmailConfig{
			From:     config.From,
			To:       recipients,
			SMTPHost: config.SMTPHost,
			SMTPPort: config.SMTPPort,
			Username: config.Username,
			Password: config.Password,
		},
		Provider:      config.Provider,
		StartTLS:      config.StartTLS, // Use the configured StartTLS setting
		MaxRetries:    2,
		RetryDelay:    3,
		RateLimit:     rl,
		SkipTLSVerify: false,
		AuthRequired:  config.Username != "" && config.Password != "",
	}

	// Use enhanced email manager for better compatibility
	enhancedManager := NewEnhancedEmailManager(enhancedConfig)

	log.Info().
		Str("smtp", fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)).
		Str("from", config.From).
		Strs("to", recipients).
		Bool("hasAuth", config.Username != "" && config.Password != "").
		Bool("startTLS", enhancedConfig.StartTLS).
		Msg("attempting to send email via SMTP with enhanced support")

	err := enhancedManager.SendEmailWithRetry(subject, htmlBody, textBody)

	if err != nil {
		log.Error().
			Err(err).
			Str("smtp", fmt.Sprintf("%s:%d", config.SMTPHost, config.SMTPPort)).
			Strs("recipients", recipients).
			Msg("failed to send email notification")
	} else {
		log.Info().
			Strs("recipients", recipients).
			Int("recipientCount", len(recipients)).
			Msg("email notification sent successfully")
	}
}

// sendGroupedWebhook sends a grouped webhook notification
func (n *NotificationManager) sendGroupedWebhook(webhook WebhookConfig, alertList []*alerts.Alert) error {
	var jsonData []byte
	var err error

	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	// Create a shallow copy of the primary alert to avoid mutating the original memory
	// when we modify the message for grouped summaries.
	originalPrimary := alertList[0]
	alertCopy := *originalPrimary
	primaryAlert := &alertCopy
	customFields := convertWebhookCustomFields(webhook.CustomFields)

	var templateData WebhookPayloadData
	var dataPrepared bool
	var urlRendered bool
	var serviceDataApplied bool

	prepareData := func() *WebhookPayloadData {
		if !dataPrepared {
			prepared := n.prepareWebhookData(primaryAlert, customFields)
			prepared.AlertCount = len(alertList)
			prepared.Alerts = alertList
			prepared.Mention = webhook.Mention
			templateData = prepared
			dataPrepared = true
		}
		return &templateData
	}

	ensureURLAndServiceData := func() (*WebhookPayloadData, bool) {
		dataPtr := prepareData()

		if !urlRendered {
			rendered, renderErr := renderWebhookURL(webhook.URL, *dataPtr)
			if renderErr != nil {
				log.Error().
					Err(renderErr).
					Str("webhook", webhook.Name).
					Msg("failed to render webhook URL template for grouped notification")
				return nil, false
			}
			webhook.URL = rendered
			urlRendered = true
		}

		if !serviceDataApplied {
			switch webhook.Service {
			case "telegram":
				chatID, chatErr := extractTelegramChatID(webhook.URL)
				if chatErr != nil {
					log.Error().
						Err(chatErr).
						Str("webhook", webhook.Name).
						Msg("failed to extract Telegram chat_id for grouped notification")
					return nil, false
				}
				if chatID != "" {
					dataPtr.ChatID = chatID
					log.Debug().
						Str("webhook", webhook.Name).
						Str("chatID", chatID).
						Msg("extracted Telegram chat_id from rendered URL for grouped notification")
				}
			case "pagerduty":
				if dataPtr.CustomFields == nil {
					dataPtr.CustomFields = make(map[string]interface{})
				}
				if routingKey, ok := webhook.Headers["routing_key"]; ok {
					dataPtr.CustomFields["routing_key"] = routingKey
				}
			case "pushover":
				dataPtr.CustomFields = ensurePushoverCustomFieldAliases(dataPtr.CustomFields)
			}
			serviceDataApplied = true
		}

		return dataPtr, true
	}

	// Check if webhook has a custom template first
	// Only use custom template if it's not empty
	if webhook.Template != "" && strings.TrimSpace(webhook.Template) != "" && len(alertList) > 0 {
		// Use custom template with enhanced message for grouped alerts
		alert := primaryAlert
		if len(alertList) > 1 {
			// Build a full list of all alerts
			summary := alert.Message
			otherAlerts := []string{}
			for i := 1; i < len(alertList); i++ { // Show ALL alerts
				otherAlerts = append(otherAlerts, fmt.Sprintf("• %s: %.1f%%", alertList[i].ResourceName, alertList[i].Value))
			}
			if len(otherAlerts) > 0 {
				// For custom templates, we need to escape newlines since they're likely
				// used in shell commands or other contexts that need escaping
				alert.Message = fmt.Sprintf("%s\\n\\nAll %d alerts:\\n%s", summary, len(alertList), strings.Join(otherAlerts, "\\n"))
			}
		}

		enhanced := EnhancedWebhookConfig{
			WebhookConfig:   webhook,
			Service:         webhook.Service,
			PayloadTemplate: webhook.Template,
			CustomFields:    customFields,
		}

		if dataPtr, ok := ensureURLAndServiceData(); ok {
			jsonData, err = n.generatePayloadFromTemplateWithService(enhanced.PayloadTemplate, *dataPtr, webhook.Service)
		} else {
			return fmt.Errorf("failed to prepare webhook URL and service data")
		}
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Int("alertCount", len(alertList)).
				Msg("failed to generate grouped payload from custom template")
			return fmt.Errorf("failed to generate payload from custom template: %w", err)
		}
	} else if webhook.Service != "" && webhook.Service != "generic" && len(alertList) > 0 {
		// For service-specific webhooks, use the first alert with a note about others
		// For simplicity, send the first alert with a note about others
		// Most webhook services work better with single structured payloads
		alert := primaryAlert

		enhanced := EnhancedWebhookConfig{
			WebhookConfig: webhook,
			Service:       webhook.Service,
			CustomFields:  customFields,
		}

		// Get service template
		templates := GetWebhookTemplates()
		templateFound := false
		for _, tmpl := range templates {
			if tmpl.Service == webhook.Service {
				enhanced.PayloadTemplate = tmpl.PayloadTemplate
				templateFound = true
				break
			}
		}

		if templateFound {
			// Modify message if multiple alerts - but format differently for Discord
			if len(alertList) > 1 {
				summary := alert.Message
				otherAlerts := []string{}
				for i := 1; i < len(alertList); i++ {
					otherAlerts = append(otherAlerts, fmt.Sprintf("• %s: %.1f%%", alertList[i].ResourceName, alertList[i].Value))
				}
				if len(otherAlerts) > 0 {
					// For Discord, format as a single line list to avoid newline issues
					// Discord embeds don't render \n in description anyway
					if webhook.Service == "discord" {
						// Use comma-separated list for Discord
						alert.Message = fmt.Sprintf("%s | %d alerts: %s", summary, len(alertList), strings.Join(otherAlerts, ", "))
					} else {
						// For other services, escape newlines properly
						alert.Message = fmt.Sprintf("%s\\n\\nAll %d alerts:\\n%s", summary, len(alertList), strings.Join(otherAlerts, "\\n"))
					}
				}
			}

			if dataPtr, ok := ensureURLAndServiceData(); ok {
				jsonData, err = n.generatePayloadFromTemplateWithService(enhanced.PayloadTemplate, *dataPtr, webhook.Service)
			} else {
				return fmt.Errorf("failed to prepare webhook URL and service data")
			}
			if err != nil {
				log.Error().
					Err(err).
					Str("webhook", webhook.Name).
					Int("alertCount", len(alertList)).
					Msg("failed to generate payload for grouped alerts")
				return fmt.Errorf("failed to generate payload for grouped alerts: %w", err)
			}
		} else {
			// No template found, use generic payload
			webhook.Service = "generic"
		}
	}

	// Use generic payload if no service or template not found
	// But ONLY if jsonData hasn't been set yet (from custom template)
	if jsonData == nil && (webhook.Service == "" || webhook.Service == "generic") {
		if _, ok := ensureURLAndServiceData(); !ok {
			return fmt.Errorf("failed to prepare webhook URL and service data")
		}

		// Use generic payload for other services
		payload := map[string]interface{}{
			"alerts":    alertList,
			"count":     len(alertList),
			"timestamp": time.Now().Unix(),
			"source":    "pulse-monitoring",
			"grouped":   true,
		}

		jsonData, err = json.Marshal(payload)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Int("alertCount", len(alertList)).
				Msg("failed to marshal grouped webhook payload")
			return fmt.Errorf("failed to marshal grouped webhook payload: %w", err)
		}
	}

	if _, ok := ensureURLAndServiceData(); !ok {
		return fmt.Errorf("failed to prepare webhook URL and service data")
	}

	// Send using same request logic
	return n.sendWebhookRequest(webhook, jsonData, "grouped")
}

func (n *NotificationManager) sendResolvedWebhook(webhook WebhookConfig, alertList []*alerts.Alert, resolvedAt time.Time) error {
	if len(alertList) == 0 {
		return fmt.Errorf("no alerts to send")
	}

	if !webhook.Enabled {
		return fmt.Errorf("webhook is disabled")
	}

	if resolvedAt.IsZero() {
		resolvedAt = time.Now()
	}

	// ntfy needs plain-text body + headers, not JSON
	if webhook.Service == "ntfy" {
		return n.sendResolvedWebhookNtfy(webhook, alertList, resolvedAt)
	}

	payload := map[string]interface{}{
		"event":         string(eventResolved),
		"alerts":        alertList,
		"count":         len(alertList),
		"resolvedAt":    resolvedAt.Unix(),
		"resolvedAtIso": resolvedAt.Format(time.RFC3339),
		"source":        "pulse-monitoring",
	}

	if n.publicURL != "" {
		payload["dashboard"] = n.publicURL
	}

	if len(alertList) == 1 && alertList[0] != nil {
		payload["alertId"] = alertList[0].ID
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Int("alertCount", len(alertList)).
			Msg("failed to marshal resolved webhook payload")
		return fmt.Errorf("failed to marshal resolved webhook payload: %w", err)
	}

	return n.sendWebhookRequest(webhook, jsonData, "resolved")
}

// sendResolvedWebhookNtfy sends a resolved webhook formatted for ntfy (plain text + headers)
func (n *NotificationManager) sendResolvedWebhookNtfy(webhook WebhookConfig, alertList []*alerts.Alert, resolvedAt time.Time) error {
	// Re-validate webhook URL
	if err := n.ValidateWebhookURL(webhook.URL); err != nil {
		return fmt.Errorf("webhook URL validation failed: %w", err)
	}

	if !n.checkWebhookRateLimit(webhook.URL) {
		return fmt.Errorf("rate limit exceeded for webhook %s", webhook.Name)
	}

	// Build plain-text body
	var body strings.Builder
	if len(alertList) == 1 && alertList[0] != nil {
		a := alertList[0]
		fmt.Fprintf(&body, "Resolved: %s on %s is now healthy", a.ResourceName, a.Node)
	} else {
		fmt.Fprintf(&body, "%d alerts resolved at %s:\n", len(alertList), resolvedAt.Format(time.RFC822))
		for _, a := range alertList {
			if a != nil {
				fmt.Fprintf(&body, "- %s on %s\n", a.ResourceName, a.Node)
			}
		}
	}

	// Build title
	title := "RESOLVED"
	if len(alertList) == 1 && alertList[0] != nil {
		title = fmt.Sprintf("RESOLVED: %s", alertList[0].ResourceName)
	} else {
		title = fmt.Sprintf("RESOLVED: %d alerts", len(alertList))
	}

	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, webhook.URL, bytes.NewBufferString(body.String()))
	if err != nil {
		return fmt.Errorf("failed to create ntfy request: %w", err)
	}

	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Title", title)
	req.Header.Set("Priority", "default")
	req.Header.Set("Tags", "white_check_mark,pulse,resolved")
	req.Header.Set("User-Agent", "Pulse-Monitoring/2.0")

	// Apply any custom headers from webhook config
	for key, value := range webhook.Headers {
		if !strings.Contains(value, "{{") {
			req.Header.Set(key, value)
		}
	}

	resp, err := n.webhookClient.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Msg("failed to send resolved ntfy webhook")
		return fmt.Errorf("failed to send ntfy webhook: %w", err)
	}
	defer resp.Body.Close()

	// Read response with size limit
	limitedReader := io.LimitReader(resp.Body, WebhookMaxResponseSize)
	var respBody bytes.Buffer
	respBody.ReadFrom(limitedReader)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Info().
			Str("webhook", webhook.Name).
			Str("service", "ntfy").
			Str("type", "resolved").
			Int("status", resp.StatusCode).
			Int("alertCount", len(alertList)).
			Msg("resolved ntfy webhook sent successfully")
		return nil
	}

	log.Warn().
		Str("webhook", webhook.Name).
		Str("service", "ntfy").
		Int("status", resp.StatusCode).
		Str("response", respBody.String()).
		Msg("resolved ntfy webhook returned non-success status")
	return fmt.Errorf("ntfy webhook returned HTTP %d: %s", resp.StatusCode, respBody.String())
}

// checkWebhookRateLimit checks if a webhook can be sent based on rate limits
func (n *NotificationManager) checkWebhookRateLimit(webhookURL string) bool {
	n.webhookRateMu.Lock()
	defer n.webhookRateMu.Unlock()

	now := time.Now()
	n.cleanupWebhookRateLimitsLocked(now)
	limit, exists := n.webhookRateLimits[webhookURL]

	if !exists {
		// First time sending to this webhook
		n.webhookRateLimits[webhookURL] = &webhookRateLimit{
			lastSent:  now,
			sentCount: 1,
		}
		return true
	}

	// Check if we're still in the rate limit window
	if now.Sub(limit.lastSent) > WebhookRateLimitWindow {
		// Window expired, reset counter
		limit.lastSent = now
		limit.sentCount = 1
		return true
	}

	// Still in window, check if we've exceeded the limit
	if limit.sentCount >= WebhookRateLimitMax {
		log.Warn().
			Str("webhookURL", webhookURL).
			Int("sentCount", limit.sentCount).
			Dur("window", WebhookRateLimitWindow).
			Msg("webhook rate limit exceeded, dropping request")
		return false
	}

	// Increment counter and allow
	limit.sentCount++
	return true
}

func (n *NotificationManager) cleanupWebhookRateLimitsLocked(now time.Time) {
	if now.Sub(n.webhookRateCleanup) < WebhookRateLimitWindow {
		return
	}

	cutoff := now.Add(-WebhookRateLimitWindow)
	cleaned := 0
	for webhookURL, limit := range n.webhookRateLimits {
		if limit.lastSent.Before(cutoff) {
			delete(n.webhookRateLimits, webhookURL)
			cleaned++
		}
	}
	n.webhookRateCleanup = now

	if cleaned > 0 {
		log.Debug().
			Int("cleaned", cleaned).
			Int("remaining", len(n.webhookRateLimits)).
			Msg("Cleaned up stale webhook rate limit entries")
	}
}

// sendWebhookRequest sends the actual webhook request
func (n *NotificationManager) sendWebhookRequest(webhook WebhookConfig, jsonData []byte, alertType string) error {
	// Re-validate webhook URL to prevent DNS rebinding attacks
	if err := n.ValidateWebhookURL(webhook.URL); err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("url", webhook.URL).
			Msg("webhook URL validation failed at send time - possible DNS rebinding")
		return fmt.Errorf("webhook URL validation failed: %w", err)
	}

	// Check rate limit before sending
	if !n.checkWebhookRateLimit(webhook.URL) {
		log.Warn().
			Str("webhook", webhook.Name).
			Str("url", webhook.URL).
			Msg("Webhook request dropped due to rate limiting")
		return fmt.Errorf("rate limit exceeded for webhook %s", webhook.Name)
	}

	// Create request
	method := webhook.Method
	if method == "" {
		method = "POST"
	}

	// For Telegram webhooks, strip chat_id from URL if present
	// The chat_id should only be in the JSON body, not the URL
	webhookURL := webhook.URL
	if webhook.Service == "telegram" && strings.Contains(webhookURL, "chat_id=") {
		if u, err := url.Parse(webhookURL); err == nil {
			q := u.Query()
			q.Del("chat_id") // Remove chat_id from query params
			u.RawQuery = q.Encode()
			webhookURL = u.String()
			log.Debug().
				Str("original", webhook.URL).
				Str("cleaned", webhookURL).
				Msg("stripped chat_id from Telegram webhook URL")
		}
	}

	req, err := http.NewRequest(method, webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("type", alertType).
			Msg("failed to create webhook request")
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Pulse-Monitoring/2.0")

	// Special handling for ntfy service
	if webhook.Service == "ntfy" {
		// Set Content-Type for ntfy (plain text)
		req.Header.Set("Content-Type", "text/plain")
		// Note: Dynamic headers for ntfy are set in sendWebhook for individual alerts
	}

	// Apply any custom headers from webhook config
	for key, value := range webhook.Headers {
		// Skip template-like headers (those with {{) to prevent errors
		if !strings.Contains(value, "{{") {
			req.Header.Set(key, value)
		}
	}

	// Debug log for Telegram and Gotify webhooks (without secrets)
	if webhook.Service == "telegram" || webhook.Service == "gotify" {
		log.Debug().
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Msg("sending webhook")
	}

	// Send request with shared secure client
	resp, err := n.webhookClient.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("webhook", webhook.Name).
			Str("type", alertType).
			Msg("failed to send webhook")
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	// Read response body with size limit to prevent memory exhaustion
	limitedReader := io.LimitReader(resp.Body, WebhookMaxResponseSize)
	var respBody bytes.Buffer
	bytesRead, err := respBody.ReadFrom(limitedReader)
	if err != nil {
		log.Warn().
			Err(err).
			Str("webhook", webhook.Name).
			Str("type", alertType).
			Msg("failed to read webhook response body")
		return fmt.Errorf("failed to read webhook response: %w", err)
	}

	// Check if we hit the size limit
	if bytesRead >= WebhookMaxResponseSize {
		log.Warn().
			Str("webhook", webhook.Name).
			Int64("bytesRead", bytesRead).
			Int("maxSize", WebhookMaxResponseSize).
			Msg("webhook response exceeded size limit, truncated")
	}

	responseBody := respBody.String()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Info().
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Str("type", alertType).
			Int("status", resp.StatusCode).
			Int("payloadSize", len(jsonData)).
			Msg("webhook notification sent successfully")

		// Log response body only in debug mode for successful requests
		if len(responseBody) > 0 {
			log.Debug().
				Str("webhook", webhook.Name).
				Str("response", responseBody).
				Msg("webhook response body")
		}
		return nil
	} else {
		log.Warn().
			Str("webhook", webhook.Name).
			Str("service", webhook.Service).
			Str("type", alertType).
			Int("status", resp.StatusCode).
			Str("response", responseBody).
			Msg("webhook returned non-success status")
		return fmt.Errorf("webhook returned HTTP %d: %s", resp.StatusCode, responseBody)
	}
}

// sendWebhook sends a webhook notification
func (n *NotificationManager) sendWebhook(webhook WebhookConfig, alert *alerts.Alert) {
	var jsonData []byte
	var err error

	customFields := convertWebhookCustomFields(webhook.CustomFields)
	data := n.prepareWebhookData(alert, customFields)

	// Render URL template if placeholders are present
	renderedURL, renderErr := renderWebhookURL(webhook.URL, data)
	if renderErr != nil {
		log.Error().
			Err(renderErr).
			Str("webhook", webhook.Name).
			Msg("failed to render webhook URL template")
		return
	}
	webhook.URL = renderedURL

	// Service-specific data enrichment
	if webhook.Service == "telegram" {
		chatID, chatErr := extractTelegramChatID(renderedURL)
		if chatErr != nil {
			log.Error().
				Err(chatErr).
				Str("webhook", webhook.Name).
				Msg("failed to extract Telegram chat_id - skipping webhook")
			return
		}
		if chatID != "" {
			data.ChatID = chatID
			log.Debug().
				Str("webhook", webhook.Name).
				Str("chatID", chatID).
				Msg("extracted Telegram chat_id from rendered URL")
		}
	} else if webhook.Service == "pagerduty" {
		if data.CustomFields == nil {
			data.CustomFields = make(map[string]interface{})
		}
		if routingKey, ok := webhook.Headers["routing_key"]; ok {
			data.CustomFields["routing_key"] = routingKey
		}
	}

	// Check if webhook has a custom template first
	// Only use custom template if it's not empty
	if webhook.Template != "" && strings.TrimSpace(webhook.Template) != "" {
		// Use custom template provided by user
		enhanced := EnhancedWebhookConfig{
			WebhookConfig:   webhook,
			Service:         webhook.Service,
			PayloadTemplate: webhook.Template,
			CustomFields:    customFields,
		}

		jsonData, err = n.generatePayloadFromTemplateWithService(enhanced.PayloadTemplate, data, webhook.Service)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Str("alertID", alert.ID).
				Msg("failed to generate webhook payload from custom template")
			return
		}
	} else if webhook.Service != "" && webhook.Service != "generic" {
		// Check if this webhook has a service type and use the proper template
		// Convert to enhanced webhook to use template
		enhanced := EnhancedWebhookConfig{
			WebhookConfig: webhook,
			Service:       webhook.Service,
			CustomFields:  customFields,
		}

		// Get service template
		templates := GetWebhookTemplates()
		templateFound := false
		for _, tmpl := range templates {
			if tmpl.Service == webhook.Service {
				enhanced.PayloadTemplate = tmpl.PayloadTemplate
				templateFound = true
				break
			}
		}

		// Only use template if found, otherwise fall back to generic
		if templateFound {
			jsonData, err = n.generatePayloadFromTemplateWithService(enhanced.PayloadTemplate, data, webhook.Service)
			if err != nil {
				log.Error().
					Err(err).
					Str("webhook", webhook.Name).
					Str("service", webhook.Service).
					Str("alertID", alert.ID).
					Msg("failed to generate webhook payload")
				return
			}
		} else {
			// No template found, use generic payload
			webhook.Service = "generic"
		}
	}

	// Use generic payload if no service or template not found
	// But ONLY if jsonData hasn't been set yet (from custom template)
	if jsonData == nil && (webhook.Service == "" || webhook.Service == "generic") {
		// Use generic payload for other services
		payload := map[string]interface{}{
			"alert":     alert,
			"timestamp": time.Now().Unix(),
			"source":    "pulse-monitoring",
		}

		jsonData, err = json.Marshal(payload)
		if err != nil {
			log.Error().
				Err(err).
				Str("webhook", webhook.Name).
				Str("alertID", alert.ID).
				Msg("failed to marshal webhook payload")
			return
		}
	}

	// Send using common request logic
	n.sendWebhookRequest(webhook, jsonData, fmt.Sprintf("alert-%s", alert.ID))
}

func convertWebhookCustomFields(fields map[string]string) map[string]interface{} {
	if len(fields) == 0 {
		return nil
	}

	converted := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		converted[key] = value
	}
	return converted
}

func ensurePushoverCustomFieldAliases(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}

	if _, ok := fields["token"]; !ok || isEmptyInterface(fields["token"]) {
		if legacy, ok := fields["app_token"]; ok && !isEmptyInterface(legacy) {
			fields["token"] = legacy
		}
	}

	if _, ok := fields["user"]; !ok || isEmptyInterface(fields["user"]) {
		if legacy, ok := fields["user_token"]; ok && !isEmptyInterface(legacy) {
			fields["user"] = legacy
		}
	}

	return fields
}

func isEmptyInterface(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case fmt.Stringer:
		return strings.TrimSpace(v.String()) == ""
	case nil:
		return true
	default:
		return false
	}
}

// prepareWebhookData prepares data for template rendering
func (n *NotificationManager) prepareWebhookData(alert *alerts.Alert, customFields map[string]interface{}) WebhookPayloadData {
	duration := time.Since(alert.StartTime)

	// Construct full Pulse URL if publicURL is configured
	// The Instance field should contain the full URL to the Pulse dashboard
	instance := ""
	if n.publicURL != "" {
		// Remove trailing slash from publicURL if present
		instance = strings.TrimRight(n.publicURL, "/")
	} else if alert.Instance != "" && (strings.HasPrefix(alert.Instance, "http://") || strings.HasPrefix(alert.Instance, "https://")) {
		// If publicURL is not set but alert.Instance contains a full URL, use it
		instance = alert.Instance
	}

	resourceType := ""
	if alert.Metadata != nil {
		if rt, ok := alert.Metadata["resourceType"].(string); ok {
			resourceType = rt
		}
	}

	var metadataCopy map[string]interface{}
	if alert.Metadata != nil {
		metadataCopy = make(map[string]interface{}, len(alert.Metadata))
		for k, v := range alert.Metadata {
			metadataCopy[k] = v
		}
	}

	var ackTime string
	if alert.AckTime != nil {
		ackTime = alert.AckTime.Format(time.RFC3339)
	}

	// Round Value and Threshold to 1 decimal place for cleaner webhook payloads
	roundedValue := math.Round(alert.Value*10) / 10
	roundedThreshold := math.Round(alert.Threshold*10) / 10

	return WebhookPayloadData{
		ID:                 alert.ID,
		Level:              string(alert.Level),
		Type:               alert.Type,
		ResourceName:       alert.ResourceName,
		ResourceID:         alert.ResourceID,
		Node:               alert.Node,
		NodeDisplayName:    alertNodeDisplay(alert),
		Instance:           instance,
		Message:            alert.Message,
		Value:              roundedValue,
		Threshold:          roundedThreshold,
		ValueFormatted:     formatMetricValue(alert.Type, alert.Value),
		ThresholdFormatted: formatMetricThreshold(alert.Type, alert.Threshold),
		StartTime:          alert.StartTime.Format(time.RFC3339),
		Duration:           formatWebhookDuration(duration),
		Timestamp:          time.Now().Format(time.RFC3339),
		ResourceType:       resourceType,
		Acknowledged:       alert.Acknowledged,
		AckTime:            ackTime,
		AckUser:            alert.AckUser,
		Metadata:           metadataCopy,
		CustomFields:       customFields,
		AlertCount:         1,
	}
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"title": func(s string) string {
			if s == "" {
				return s
			}
			return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
		},
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"printf":    fmt.Sprintf,
		"urlquery":  template.URLQueryEscaper,
		"urlencode": template.URLQueryEscaper,
		"urlpath":   url.PathEscape,
		"pathescape": func(s string) string {
			return url.PathEscape(s)
		},
	}
}

// generatePayloadFromTemplateWithService renders the payload using Go templates with service-specific handling
func (n *NotificationManager) generatePayloadFromTemplateWithService(templateStr string, data WebhookPayloadData, service string) ([]byte, error) {
	tmpl, err := template.New("webhook").Funcs(templateFuncMap()).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	// Skip JSON validation for services that use plain text payloads
	if service == "ntfy" {
		// ntfy uses plain text, not JSON
		return buf.Bytes(), nil
	}

	// Validate that the generated payload is valid JSON for other services
	var jsonCheck interface{}
	if err := json.Unmarshal(buf.Bytes(), &jsonCheck); err != nil {
		log.Error().
			Err(err).
			Str("payload", buf.String()).
			Msg("generated webhook payload is invalid JSON")
		return nil, fmt.Errorf("template produced invalid JSON: %w", err)
	}

	return buf.Bytes(), nil
}

// renderWebhookURL applies template rendering to webhook URLs and ensures the result is a valid URL
func renderWebhookURL(urlTemplate string, data WebhookPayloadData) (string, error) {
	trimmed := strings.TrimSpace(urlTemplate)
	if trimmed == "" {
		return "", fmt.Errorf("webhook URL cannot be empty")
	}

	if !strings.Contains(trimmed, "{{") {
		return trimmed, nil
	}

	tmpl, err := template.New("webhook_url").Funcs(templateFuncMap()).Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid webhook URL template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("webhook URL template execution failed: %w", err)
	}

	rendered := strings.TrimSpace(buf.String())
	if rendered == "" {
		return "", fmt.Errorf("webhook URL template produced empty URL")
	}

	parsed, err := url.Parse(rendered)
	if err != nil {
		return "", fmt.Errorf("webhook URL template produced invalid URL: %w", err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("webhook URL template produced invalid URL: missing scheme or host")
	}

	return parsed.String(), nil
}

// formatWebhookDuration formats a duration in a human-readable way
func formatWebhookDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	} else {
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}

// extractTelegramChatID extracts and validates the chat_id from a Telegram webhook URL
func extractTelegramChatID(webhookURL string) (string, error) {
	if !strings.Contains(webhookURL, "chat_id=") {
		return "", fmt.Errorf("telegram webhook URL missing chat_id parameter")
	}

	u, err := url.Parse(webhookURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	chatID := u.Query().Get("chat_id")
	if chatID == "" {
		return "", fmt.Errorf("chat_id parameter is empty")
	}

	// Validate that chat_id is numeric (Telegram chat IDs are always numeric)
	// Handle negative IDs (group chats) and positive IDs (private chats)
	if strings.HasPrefix(chatID, "-") {
		if !isNumeric(chatID[1:]) {
			return "", fmt.Errorf("chat_id must be numeric, got: %s", chatID)
		}
	} else if !isNumeric(chatID) {
		return "", fmt.Errorf("chat_id must be numeric, got: %s", chatID)
	}

	return chatID, nil
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return len(s) > 0
}

// ValidateWebhookURL validates that a webhook URL is safe and properly formed
func (n *NotificationManager) ValidateWebhookURL(webhookURL string) error {
	if webhookURL == "" {
		return fmt.Errorf("webhook URL cannot be empty")
	}

	u, err := url.Parse(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Must be HTTP or HTTPS
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https protocol")
	}

	// Get hostname for validation
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("webhook URL missing hostname")
	}

	// Block localhost and loopback addresses (SSRF protection) unless allowlisted
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "127.") {
		// Check if localhost is in the allowlist
		localhostIP := net.ParseIP("127.0.0.1")
		if !n.isIPInAllowlist(localhostIP) {
			return fmt.Errorf("webhook URLs pointing to localhost are not allowed for security reasons")
		}
		log.Debug().
			Str("host", host).
			Str("url", webhookURL).
			Msg("localhost webhook URL allowed via allowlist")
	}

	// Block link-local addresses
	if strings.HasPrefix(host, "169.254.") || strings.HasPrefix(host, "fe80:") {
		return fmt.Errorf("webhook URLs pointing to link-local addresses are not allowed")
	}

	// Resolve hostname to IPs and check for private ranges (DNS rebinding protection)
	ips, err := net.LookupIP(host)
	if err != nil {
		// DNS resolution failed - reject for security
		return fmt.Errorf("failed to resolve webhook hostname %s: %w (DNS resolution required for security)", host, err)
	}

	// Check all resolved IPs for private ranges
	for _, ip := range ips {
		if isPrivateIP(ip) {
			// Check if this private IP is in the allowlist
			if n.isIPInAllowlist(ip) {
				log.Debug().
					Str("ip", ip.String()).
					Str("url", webhookURL).
					Msg("webhook URL resolves to private IP in allowlist")
			} else {
				return fmt.Errorf("webhook URL resolves to private IP %s - private networks are not allowed for security (configure allowlist in System Settings)", ip.String())
			}
		}
	}

	// Block common metadata service endpoints (cloud providers)
	metadataHosts := []string{
		"169.254.169.254", // AWS, Azure, GCP metadata
		"metadata.google.internal",
		"metadata.goog",
	}
	for _, metadataHost := range metadataHosts {
		if host == metadataHost {
			return fmt.Errorf("webhook URLs pointing to cloud metadata services are not allowed")
		}
	}

	// Ensure hostname is not just an IP address without proper DNS
	// This helps prevent SSRF attacks using numeric IPs to bypass filters
	if u.Scheme == "https" && isNumericIP(host) {
		log.Warn().
			Str("url", webhookURL).
			Msg("webhook URL uses numeric IP with HTTPS - certificate validation may fail")
	}

	return nil
}

// isPrivateIP checks if an IP address is in a private range
func isPrivateIP(ip net.IP) bool {
	// Private IPv4 ranges
	privateRanges := []string{
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"127.0.0.0/8",    // Loopback
		"169.254.0.0/16", // Link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local
	}

	for _, cidr := range privateRanges {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}

// isNumericIP checks if a string is a numeric IP address
func isNumericIP(host string) bool {
	// Simple check: if it contains only digits, dots, and colons, it's likely an IP
	for _, char := range host {
		if !(char >= '0' && char <= '9') && char != '.' && char != ':' {
			return false
		}
	}
	return len(host) > 0 && (strings.Contains(host, ".") || strings.Contains(host, ":"))
}

// UpdateAllowedPrivateCIDRs parses and updates the list of allowed private CIDR ranges for webhooks
func (n *NotificationManager) UpdateAllowedPrivateCIDRs(cidrsString string) error {
	n.allowedPrivateMu.Lock()
	defer n.allowedPrivateMu.Unlock()

	// Clear existing allowlist
	n.allowedPrivateNets = nil

	// Empty string means no allowlist (block all private IPs)
	if cidrsString == "" {
		log.Info().Msg("webhook private IP allowlist cleared - all private IPs blocked")
		return nil
	}

	// Parse comma-separated CIDRs
	cidrs := strings.Split(cidrsString, ",")
	var parsedNets []*net.IPNet

	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}

		// Support bare IPs by adding /32 or /128
		if !strings.Contains(cidr, "/") {
			ip := net.ParseIP(cidr)
			if ip == nil {
				return fmt.Errorf("invalid IP address: %s", cidr)
			}
			if ip.To4() != nil {
				cidr = cidr + "/32"
			} else {
				cidr = cidr + "/128"
			}
		}

		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("invalid CIDR range %s: %w", cidr, err)
		}

		parsedNets = append(parsedNets, ipNet)
	}

	n.allowedPrivateNets = parsedNets
	log.Info().
		Str("cidrs", cidrsString).
		Int("count", len(parsedNets)).
		Msg("webhook private IP allowlist updated")

	return nil
}

// isIPInAllowlist checks if an IP is in the configured allowlist
func (n *NotificationManager) isIPInAllowlist(ip net.IP) bool {
	n.allowedPrivateMu.RLock()
	defer n.allowedPrivateMu.RUnlock()

	// No allowlist means block all private IPs
	if len(n.allowedPrivateNets) == 0 {
		return false
	}

	// Check if IP is in any allowed range
	for _, ipNet := range n.allowedPrivateNets {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// addWebhookDelivery adds a webhook delivery record to the history
func (n *NotificationManager) addWebhookDelivery(delivery WebhookDelivery) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Add to history
	n.webhookHistory = append(n.webhookHistory, delivery)

	// Keep only last 100 entries
	if len(n.webhookHistory) > 100 {
		// Remove oldest entry
		n.webhookHistory = n.webhookHistory[1:]
	}
}

// GetWebhookHistory returns recent webhook delivery history
func (n *NotificationManager) GetWebhookHistory() []WebhookDelivery {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Return a copy to avoid concurrent access issues
	history := make([]WebhookDelivery, len(n.webhookHistory))
	copy(history, n.webhookHistory)
	return history
}

func buildNotificationTestAlert() *alerts.Alert {
	return &alerts.Alert{
		ID:           "test-alert",
		Type:         "cpu",
		Level:        "warning",
		ResourceID:   "test-resource",
		ResourceName: "Test Resource",
		Node:         "pve-node-01",
		Instance:     "https://192.168.1.100:8006",
		Message:      "This is a test alert from Pulse Monitoring to verify your notification settings are working correctly",
		Value:        95.5,
		Threshold:    90,
		StartTime:    time.Now().Add(-5 * time.Minute), // Show it's been active for 5 minutes
		LastSeen:     time.Now(),
		Metadata: map[string]interface{}{
			"resourceType": "vm",
		},
	}
}

// GetQueueStats returns statistics about the notification queue
func (n *NotificationManager) GetQueueStats() (map[string]int, error) {
	n.mu.RLock()
	queue := n.queue
	n.mu.RUnlock()

	if queue == nil {
		return nil, fmt.Errorf("notification queue not initialized")
	}
	return queue.GetQueueStats()
}

// SendTestNotification sends a test notification
func (n *NotificationManager) SendTestNotification(method string) error {
	testAlert := buildNotificationTestAlert()

	switch method {
	case "email":
		log.Info().
			Bool("enabled", n.emailConfig.Enabled).
			Str("smtp", n.emailConfig.SMTPHost).
			Int("port", n.emailConfig.SMTPPort).
			Str("from", n.emailConfig.From).
			Int("toCount", len(n.emailConfig.To)).
			Msg("testing email notification")
		if !n.emailConfig.Enabled {
			return fmt.Errorf("email notifications are not enabled")
		}
		n.sendEmail(testAlert)
		return nil
	case "webhook":
		n.mu.RLock()
		if len(n.webhooks) == 0 {
			n.mu.RUnlock()
			return fmt.Errorf("no webhooks configured")
		}
		// Find first enabled webhook and copy it before releasing lock
		var webhookToTest *WebhookConfig
		for _, webhook := range n.webhooks {
			if webhook.Enabled {
				// Copy webhook to avoid race condition
				webhookCopy := webhook
				webhookToTest = &webhookCopy
				break
			}
		}
		n.mu.RUnlock()

		if webhookToTest == nil {
			return fmt.Errorf("no enabled webhooks found")
		}
		n.sendWebhook(*webhookToTest, testAlert)
		return nil
	case "apprise":
		n.mu.RLock()
		appriseConfig := n.appriseConfig
		n.mu.RUnlock()

		log.Info().
			Bool("enabled", appriseConfig.Enabled).
			Str("mode", string(appriseConfig.Mode)).
			Int("targetCount", len(appriseConfig.Targets)).
			Msg("testing Apprise notification")

		if !appriseConfig.Enabled {
			return fmt.Errorf("apprise notifications are not enabled")
		}

		// Use sendGroupedApprise with a single test alert
		return n.sendGroupedApprise(appriseConfig, []*alerts.Alert{testAlert})
	default:
		return fmt.Errorf("unknown notification method: %s", method)
	}
}

// SendTestAppriseWithConfig sends a test Apprise notification using provided config
func (n *NotificationManager) SendTestAppriseWithConfig(config AppriseConfig) error {
	cfg := NormalizeAppriseConfig(config)

	log.Info().
		Bool("enabled", cfg.Enabled).
		Str("mode", string(cfg.Mode)).
		Int("targetCount", len(cfg.Targets)).
		Str("serverURL", cfg.ServerURL).
		Msg("testing Apprise notification with provided config")

	if !cfg.Enabled {
		switch cfg.Mode {
		case AppriseModeCLI:
			return fmt.Errorf("apprise notifications are not enabled in the provided configuration: at least one target is required for CLI mode")
		case AppriseModeHTTP:
			return fmt.Errorf("apprise notifications are not enabled in the provided configuration: server URL is required for API mode")
		default:
			return fmt.Errorf("apprise notifications are not enabled in the provided configuration")
		}
	}

	return n.sendGroupedApprise(cfg, []*alerts.Alert{buildNotificationTestAlert()})
}

// SendTestWebhook sends a test notification to a specific webhook
func (n *NotificationManager) SendTestWebhook(webhook WebhookConfig) error {
	// Create a test alert for webhook testing with realistic values
	// Use the configured publicURL if available, otherwise use a placeholder
	instanceURL := n.publicURL
	if instanceURL == "" {
		instanceURL = "http://your-pulse-instance:7655"
	}

	testAlert := &alerts.Alert{
		ID:           "test-webhook-" + webhook.ID,
		Type:         "cpu",
		Level:        "warning",
		ResourceID:   "webhook-test",
		ResourceName: "Test Alert",
		Node:         "test-node",
		Instance:     instanceURL, // Use the actual Pulse URL
		Message:      fmt.Sprintf("This is a test alert from Pulse to verify your %s webhook is working correctly", webhook.Name),
		Value:        85.5,
		Threshold:    80.0,
		StartTime:    time.Now().Add(-5 * time.Minute), // Alert started 5 minutes ago
		LastSeen:     time.Now(),
		Metadata: map[string]interface{}{
			"webhookName": webhook.Name,
			"webhookURL":  webhook.URL,
			"testTime":    time.Now().Format(time.RFC3339),
		},
	}

	// Send the test webhook
	n.sendWebhook(webhook, testAlert)
	return nil
}

// SendTestNotificationWithConfig sends a test notification using provided config
func (n *NotificationManager) SendTestNotificationWithConfig(method string, config *EmailConfig, nodeInfo *TestNodeInfo) error {
	// Use actual node info if provided, otherwise use defaults
	nodeName := "test-node"
	instanceURL := n.publicURL
	if instanceURL == "" {
		instanceURL = "https://proxmox.local:8006"
	}
	if nodeInfo != nil {
		if nodeInfo.NodeName != "" {
			nodeName = nodeInfo.NodeName
		}
		if nodeInfo.InstanceURL != "" {
			instanceURL = nodeInfo.InstanceURL
		}
	}

	testAlert := &alerts.Alert{
		ID:           "test-alert",
		Type:         "cpu",
		Level:        "warning",
		ResourceID:   "test-email-config",
		ResourceName: "Email Configuration Test",
		Node:         nodeName,
		Instance:     instanceURL,
		Message:      "This is a test alert to verify your email notification settings are working correctly",
		Value:        85.5,
		Threshold:    80,
		StartTime:    time.Now(),
		LastSeen:     time.Now(),
		Metadata: map[string]interface{}{
			"resourceType": "test",
		},
	}

	switch method {
	case "email":
		if config == nil {
			return fmt.Errorf("email configuration is required")
		}

		log.Info().
			Bool("enabled", config.Enabled).
			Str("smtp", config.SMTPHost).
			Int("port", config.SMTPPort).
			Str("from", config.From).
			Int("toCount", len(config.To)).
			Strs("to", config.To).
			Bool("smtpEmpty", config.SMTPHost == "").
			Bool("fromEmpty", config.From == "").
			Msg("testing email notification with provided config")

		if !config.Enabled {
			return fmt.Errorf("email notifications are not enabled in the provided configuration")
		}

		if config.SMTPHost == "" || config.From == "" {
			return fmt.Errorf("email configuration is incomplete: SMTP host and from address are required")
		}

		// Generate email using template
		subject, htmlBody, textBody := EmailTemplate([]*alerts.Alert{testAlert}, true)

		// Send using provided config and return any error
		return n.sendHTMLEmailWithError(subject, htmlBody, textBody, *config)

	default:
		return fmt.Errorf("unsupported method for config-based testing: %s", method)
	}
}

func normalizeQueueType(notifType string) (string, notificationEvent) {
	if strings.HasSuffix(notifType, queueTypeSuffixResolved) {
		return strings.TrimSuffix(notifType, queueTypeSuffixResolved), eventResolved
	}
	return notifType, eventAlert
}

func resolvedTimeFromAlerts(alerts []*alerts.Alert) time.Time {
	for _, alert := range alerts {
		if alert == nil || alert.Metadata == nil {
			continue
		}
		raw, ok := alert.Metadata[metadataResolvedAt]
		if !ok {
			continue
		}
		switch ts := raw.(type) {
		case string:
			if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
				return parsed
			}
		case float64:
			if ts > 0 {
				return time.Unix(int64(ts), 0)
			}
		}
	}

	return time.Now()
}

// ProcessQueuedNotification processes a notification from the persistent queue
func (n *NotificationManager) ProcessQueuedNotification(notif *QueuedNotification) error {
	baseType, event := normalizeQueueType(notif.Type)

	log.Debug().
		Str("notificationID", notif.ID).
		Str("type", baseType).
		Str("event", string(event)).
		Int("alertCount", len(notif.Alerts)).
		Msg("processing queued notification")

	var err error
	switch baseType {
	case "email":
		var emailConfig EmailConfig
		if err = json.Unmarshal(notif.Config, &emailConfig); err != nil {
			return fmt.Errorf("failed to unmarshal email config: %w", err)
		}
		if event == eventResolved {
			err = n.sendResolvedEmail(emailConfig, notif.Alerts, resolvedTimeFromAlerts(notif.Alerts))
		} else {
			err = n.sendGroupedEmail(emailConfig, notif.Alerts)
		}

	case "webhook":
		var webhookConfig WebhookConfig
		if err = json.Unmarshal(notif.Config, &webhookConfig); err != nil {
			return fmt.Errorf("failed to unmarshal webhook config: %w", err)
		}
		if event == eventResolved {
			err = n.sendResolvedWebhook(webhookConfig, notif.Alerts, resolvedTimeFromAlerts(notif.Alerts))
		} else {
			err = n.sendGroupedWebhook(webhookConfig, notif.Alerts)
		}

	case "apprise":
		var appriseConfig AppriseConfig
		if err = json.Unmarshal(notif.Config, &appriseConfig); err != nil {
			return fmt.Errorf("failed to unmarshal apprise config: %w", err)
		}
		if event == eventResolved {
			err = n.sendResolvedApprise(appriseConfig, notif.Alerts, resolvedTimeFromAlerts(notif.Alerts))
		} else {
			err = n.sendGroupedApprise(appriseConfig, notif.Alerts)
		}

	default:
		return fmt.Errorf("unknown notification type: %s", baseType)
	}

	// Mark cooldown after successful send for active alerts only
	if err == nil && event == eventAlert {
		n.mu.Lock()
		now := time.Now()
		for _, alert := range notif.Alerts {
			n.lastNotified[alert.ID] = notificationRecord{
				lastSent:   now,
				alertStart: alert.StartTime,
			}
		}
		n.mu.Unlock()
	}

	if err != nil {
		return fmt.Errorf("process queued %s notification %q (%s): %w", baseType, notif.ID, event, err)
	}
	return nil
}

// cleanupOldNotificationRecords periodically cleans up old entries from lastNotified map
func (n *NotificationManager) cleanupOldNotificationRecords() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n.mu.Lock()
			now := time.Now()
			cutoff := now.Add(-24 * time.Hour)
			cleaned := 0

			for alertID, record := range n.lastNotified {
				// Remove entries older than 24 hours
				if record.lastSent.Before(cutoff) {
					delete(n.lastNotified, alertID)
					cleaned++
				}
			}

			if cleaned > 0 {
				log.Debug().
					Int("cleaned", cleaned).
					Int("remaining", len(n.lastNotified)).
					Msg("cleaned up old notification cooldown records")
			}
			n.mu.Unlock()
		case <-n.stopCleanup:
			// Stop cleanup when manager is stopped
			return
		}
	}
}

// Stop gracefully stops the notification manager
func (n *NotificationManager) Stop() {
	n.mu.Lock()

	// Stop cleanup goroutine
	close(n.stopCleanup)

	// Get queue reference before unlocking
	queue := n.queue

	// Unlock before stopping queue to avoid deadlock with queue workers
	// that may need to acquire n.mu during ProcessQueuedNotification
	n.mu.Unlock()

	// Stop the notification queue if it exists
	if queue != nil {
		queue.Stop()
	}

	// Relock for remaining cleanup
	n.mu.Lock()
	defer n.mu.Unlock()

	// Cancel any pending group timer
	if n.groupTimer != nil {
		n.groupTimer.Stop()
		n.groupTimer = nil
	}

	// Clear pending alerts
	n.pendingAlerts = nil

	log.Info().Msg("notification manager stopped")
}
