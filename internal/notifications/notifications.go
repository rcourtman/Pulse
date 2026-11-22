package notifications

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// Webhook configuration constants
const (
	// HTTP client settings
	WebhookTimeout          = 30 * time.Second
	WebhookMaxResponseSize  = 1 * 1024 * 1024 // 1 MB max response size
	WebhookMaxRedirects     = 3               // Maximum number of redirects to follow
	WebhookTestTimeout      = 10 * time.Second
	webhookDNSLookupTimeout = 5 * time.Second

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

	// Apprise timeout bounds (seconds), applied during config normalization.
	appriseDefaultTimeoutSecs = 15
	appriseMaxTimeoutSecs     = 120
	appriseMinTimeoutSecs     = 5
)

// notificationEvent represents the type of notification being processed.
// It distinguishes between new alerts and resolved/cleared alerts.
type notificationEvent string

const (
	eventAlert    notificationEvent = "alert"
	eventResolved notificationEvent = "resolved"
)

type notificationDeliveryJob struct {
	QueueID           string
	Type              string
	ReceiptEpoch      string
	ReceiptEpochBound bool
	Event             notificationEvent
	Alerts            []*alerts.Alert
	ResolvedAt        time.Time
	EmailConfig       *EmailConfig
	WebhookConfig     *WebhookConfig
	AppriseConfig     *AppriseConfig
}

type notificationQueueBucket struct {
	job         notificationDeliveryJob
	nextRetryAt *time.Time
}

type notificationDeliveryTarget string

const (
	notificationDeliveryTargetAll     notificationDeliveryTarget = "all"
	notificationDeliveryTargetEmail   notificationDeliveryTarget = "email"
	notificationDeliveryTargetWebhook notificationDeliveryTarget = "webhook"
	notificationDeliveryTargetApprise notificationDeliveryTarget = "apprise"
)

type alertSendOptions struct {
	bypassCooldown bool
	immediate      bool
	target         notificationDeliveryTarget
	destinationIDs []string
}

// TestNodeInfo contains information about nodes for test notifications
type TestNodeInfo struct {
	NodeName    string
	InstanceURL string
}

// WebhookDelivery tracks webhook delivery attempts for debugging
type WebhookDelivery struct {
	WebhookName     string    `json:"webhookName"`
	WebhookURL      string    `json:"webhookUrl"`
	Service         string    `json:"service"`
	AlertIdentifier string    `json:"alertIdentifier,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	StatusCode      int       `json:"statusCode"`
	Success         bool      `json:"success"`
	ErrorMessage    string    `json:"errorMessage,omitempty"`
	RetryAttempts   int       `json:"retryAttempts"`
	PayloadSize     int       `json:"payloadSize"`
}

// webhookRateLimit tracks rate limiting for webhook deliveries
type webhookRateLimit struct {
	lastSent  time.Time
	sentCount int
}

type notificationLifecycleState int

const (
	notificationLifecycleRunning notificationLifecycleState = iota
	notificationLifecycleDraining
	notificationLifecycleClosed
)

// notificationLifecycle owns work that holds the queue outside the manager
// lock. Each lease is taken in the critical section that reads the queue, so
// a lease holder always writes to an open queue. Stop drains the lifecycle,
// refusing new alerts while cancellations, operator pause and direct delivery
// already owed continue, then closes it once every lease has ended.
type notificationLifecycle struct {
	mu     sync.Mutex
	idle   sync.Cond
	state  notificationLifecycleState
	leases int
}

// admit leases new alert or recovery work. Stop refuses it from the start.
func (l *notificationLifecycle) admit() bool {
	return l.acquire(notificationLifecycleRunning)
}

// join leases work that must still reach an open queue while Stop drains:
// cancellation, operator pause or channel disable, and direct delivery
// started by admitted work. It is refused only once the drain has finished.
func (l *notificationLifecycle) join() bool {
	return l.acquire(notificationLifecycleDraining)
}

func (l *notificationLifecycle) acquire(latest notificationLifecycleState) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.state > latest {
		return false
	}
	l.leases++
	return true
}

func (l *notificationLifecycle) release() {
	l.mu.Lock()
	l.leases--
	if l.leases == 0 {
		l.idle.Broadcast()
	}
	l.mu.Unlock()
}

// accepting reports whether new work may still be buffered for later
// admission.
func (l *notificationLifecycle) accepting() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state == notificationLifecycleRunning
}

func (l *notificationLifecycle) drain() {
	l.mu.Lock()
	if l.state == notificationLifecycleRunning {
		l.state = notificationLifecycleDraining
	}
	l.mu.Unlock()
}

// close waits for every lease to end, then refuses all later work.
func (l *notificationLifecycle) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.idle.L == nil {
		l.idle.L = &l.mu
	}
	for l.leases > 0 {
		l.idle.Wait()
	}
	l.state = notificationLifecycleClosed
}

// NotificationManager handles sending notifications
type NotificationManager struct {
	mu                  sync.RWMutex
	destinationMu       sync.Mutex // Serialize destination generations without holding mu across storage.
	destinationRevision uint64
	stopOnce            sync.Once
	cleanupWG           sync.WaitGroup
	lifecycle           notificationLifecycle // Admission and shutdown ownership of queue and direct delivery work
	emailConfig         EmailConfig
	emailManager        *EnhancedEmailManager // Shared email manager for rate limiting
	webhooks            []WebhookConfig
	appriseConfig       AppriseConfig
	enabled             bool
	configurationError  error // Sticky bootstrap hold; restore files and reconstruct the owner.
	cooldown            time.Duration
	deliveryCooldown    map[string]notificationRecord
	initialTarget       notificationDeliveryTarget
	notifyOnResolve     bool
	lastNotified        map[string]notificationRecord
	deliveryReceipts    map[string]struct{}
	groupWindow         time.Duration
	groupingEnabled     bool
	pendingAlerts       []*alerts.Alert
	pendingAlertIndex   map[pendingFiringOccurrence]int
	groupTimer          *time.Timer
	groupDeadline       time.Time // First admission fixes this window; shutdown must not restart or shorten it
	pendingResolved     []*alerts.Alert
	resolvedGroupTimer  *time.Timer
	groupByNode         bool
	publicURL           string // Full URL to access Pulse
	groupByGuest        bool
	webhookHistory      []WebhookDelivery            // Keep last 100 webhook deliveries for debugging
	webhookRateLimits   map[string]*webhookRateLimit // Track rate limits per webhook URL
	webhookRateMu       sync.Mutex                   // Separate mutex for webhook rate limiting
	webhookRateCleanup  time.Time                    // Last cleanup time for webhook rate limit entries
	appriseExec         appriseExecFunc
	queue               *NotificationQueue                              // Persistent notification queue
	tenantID            string                                          // Tenant identity stamped into webhook payloads (env default)
	tenantName          string                                          // Tenant display name stamped into webhook payloads (env default)
	tenantResolver      func() (id string, name string)                 // Org-backed tenant identity override (multi-tenant)
	webhookClient       *http.Client                                    // Shared HTTP client for webhooks
	stopCleanup         chan struct{}                                   // Signal to stop cleanup goroutine
	cleanupDone         chan struct{}                                   // Signals cleanup goroutine exit during shutdown
	allowedPrivateNets  []*net.IPNet                                    // Parsed CIDR ranges allowed for private webhook targets
	allowedPrivateMu    sync.RWMutex                                    // Protects allowedPrivateNets
	resolveWebhookIPs   func(context.Context, string) ([]net.IP, error) // Test seam; nil uses the context-aware default resolver
}

type webhookHTTPResult struct {
	statusCode           int
	headers              http.Header
	responseBytes        int64
	responseLimitReached bool
	responseIncomplete   bool
}

type webhookRequestOptions struct {
	alertType       string
	eventID         string // Legacy X-Pulse-Event-ID correlation header, not a unique delivery ID
	timeout         time.Duration
	userAgent       string
	responseLogging bool
	validateURL     bool
}

// spawnAsync is used for fire-and-forget notification delivery.
// It exists so tests can disable or inline async behavior without relying on sleeps.
var spawnAsync = func(f func()) { go f() }

type appriseExecFunc func(ctx context.Context, args []string) (int64, error)

// NewNotificationManager creates a new notification manager using the global data directory.
// For multi-tenant deployments, use NewNotificationManagerWithDataDir instead.
func NewNotificationManager(publicURL string) *NotificationManager {
	return NewNotificationManagerWithDataDir(publicURL, "")
}

// NewNotificationManagerWithDataDir creates a new notification manager with a custom data directory.
// This enables tenant-scoped notification queue persistence in multi-tenant deployments.
// If dataDir is empty, it uses the global data directory.
func NewNotificationManagerWithDataDir(publicURL string, dataDir string) *NotificationManager {
	return newNotificationManagerWithDataDir(publicURL, dataDir, false)
}

// NewNotificationManagerWithDeferredQueue creates a manager whose persisted
// deliveries stay idle until StartQueueProcessing is called. Owners that load
// saved destinations after construction must use this form so a queue ticker
// cannot interpret incomplete startup configuration as a removed destination.
func NewNotificationManagerWithDeferredQueue(publicURL string, dataDir string) *NotificationManager {
	return newNotificationManagerWithDataDir(publicURL, dataDir, true)
}

func newNotificationManagerWithDataDir(publicURL string, dataDir string, deferQueueProcessing bool) *NotificationManager {
	cleanURL := strings.TrimRight(strings.TrimSpace(publicURL), "/")
	if cleanURL != "" {
		log.Info().Str("publicURL", cleanURL).Msg("notification manager initialized with public URL")
	} else {
		log.Info().Msg("notification manager initialized without public URL - webhook links may not work")
	}

	// Initialize persistent queue with tenant-specific data directory
	queue, err := NewNotificationQueue(dataDir)
	persistentQueueErr := err
	if err != nil && !deferQueueProcessing {
		log.Error().Err(err).Msg("failed to initialize persistent notification queue, falling back to in-memory queue")
		queue, err = NewInMemoryNotificationQueue()
		if err != nil {
			log.Error().Err(err).Msg("failed to initialize in-memory notification queue, notifications will be unavailable")
			queue = nil
		}
	}

	nm := &NotificationManager{
		enabled:          true,
		cooldown:         5 * time.Minute,
		initialTarget:    notificationDeliveryTargetAll,
		notifyOnResolve:  true,
		lastNotified:     make(map[string]notificationRecord),
		deliveryReceipts: make(map[string]struct{}),
		webhooks:         []WebhookConfig{},
		appriseConfig: AppriseConfig{
			Enabled:        false,
			Mode:           AppriseModeCLI,
			Targets:        []string{},
			CLIPath:        "apprise",
			TimeoutSeconds: 15,
			APIKeyHeader:   "X-API-KEY",
		},
		groupWindow:       30 * time.Second,
		groupingEnabled:   true,
		tenantID:          strings.TrimSpace(os.Getenv("PULSE_TENANT_ID")),
		tenantName:        strings.TrimSpace(os.Getenv("PULSE_TENANT_NAME")),
		pendingAlerts:     make([]*alerts.Alert, 0),
		groupByNode:       true,
		groupByGuest:      false,
		webhookHistory:    make([]WebhookDelivery, 0, WebhookHistoryMaxSize),
		webhookRateLimits: make(map[string]*webhookRateLimit),
		publicURL:         cleanURL,
		appriseExec:       defaultAppriseExec,
		queue:             queue,
		stopCleanup:       make(chan struct{}),
		cleanupDone:       make(chan struct{}),
	}

	if deferQueueProcessing && persistentQueueErr != nil {
		nm.HoldStorageAuthority(persistentQueueErr)
	}

	// Create webhook client after NotificationManager is initialized
	nm.webhookClient = nm.createSecureWebhookClient(WebhookTimeout)

	// Ordinary callers retain the historical autonomous queue. A bootstrap
	// owner leaves the processor absent until every saved destination is loaded;
	// not waking the worker alone cannot guard against its periodic ticker.
	if queue != nil && !deferQueueProcessing {
		queue.installProcessor(nm.ProcessQueuedNotification)
	}

	// Start periodic cleanup of old lastNotified entries (every 1 hour)
	nm.cleanupWG.Add(1)
	go nm.cleanupOldNotificationRecords()

	return nm
}

// StartQueueProcessing retains the legacy activation API. It does not attest a
// complete saved destination set and therefore cannot retire omitted identities.
func (n *NotificationManager) StartQueueProcessing() error { return n.startQueueProcessing(false) }

// StartQueueProcessingWithConfiguration activates only after the bootstrap owner
// has successfully loaded every destination. An unreadable file is not removal.
func (n *NotificationManager) StartQueueProcessingWithConfiguration(complete bool) error {
	if !complete {
		log.Error().Msg("Notification queue held because saved destination configuration is incomplete")
		return fmt.Errorf("saved notification destination configuration is incomplete")
	}
	return n.startQueueProcessing(true)
}

func (n *NotificationManager) startQueueProcessing(complete bool) error {
	if err := n.ConfigurationError(); err != nil {
		return err
	}
	n.destinationMu.Lock()
	var err error
	if complete {
		err = n.reconcileDeliveryDestinations()
	}
	n.destinationMu.Unlock()
	if err != nil {
		log.Error().Err(err).Msg("Notification queue held because destination reconciliation failed")
		return err
	}
	n.mu.RLock()
	queue := n.queue
	n.mu.RUnlock()
	if queue != nil {
		queue.SetProcessor(n.ProcessQueuedNotification)
	}
	return nil
}

// HoldIncompleteConfiguration is a bootstrap fence. A partial inventory must
// neither activate persisted jobs nor become a writable replacement inventory.
// The hold lasts until the owner is reconstructed after storage is repaired.
func (n *NotificationManager) HoldIncompleteConfiguration(err error) {
	if err == nil {
		return
	}
	n.mu.Lock()
	n.configurationError = errors.Join(n.configurationError, err)
	n.mu.Unlock()
}

// ErrNotificationStorageAuthority identifies a bootstrap storage hold without
// exposing paths or driver errors to the operator-facing health response.
var ErrNotificationStorageAuthority = errors.New("notification storage authority unavailable")

// HoldStorageAuthority pauses a bootstrap owner while persistent delivery state is unavailable.
func (n *NotificationManager) HoldStorageAuthority(err error) {
	if err != nil {
		n.HoldIncompleteConfiguration(errors.Join(ErrNotificationStorageAuthority, err))
	}
}

// ConfigurationHoldReason returns a content-free health reason for a bootstrap hold.
func ConfigurationHoldReason(err error) string {
	if errors.Is(err, ErrNotificationStorageAuthority) {
		return NotificationReasonStorageBootstrapUnavailable
	}
	return NotificationReasonConfigurationUnavailable
}

// ConfigurationHoldMessage describes the operator action without exposing driver details.
func ConfigurationHoldMessage(err error) string {
	if errors.Is(err, ErrNotificationStorageAuthority) {
		return "Notification delivery storage is unavailable. Check server storage and logs, then restart Pulse."
	}
	return "Saved notification configuration is unavailable. Restore a configuration backup and restart Pulse."
}

func (n *NotificationManager) ConfigurationError() error {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.configurationError
}

// SetQuietHoursPolicyProvider binds persistent replay to the alert owner's
// current schedule. Install it before StartQueueProcessing during bootstrap.
// Each provider call must return a lock-independent, read-only evaluator.
func (n *NotificationManager) SetQuietHoursPolicyProvider(provider func() func(*alerts.Alert, time.Time) *time.Time) {
	n.mu.RLock()
	queue := n.queue
	n.mu.RUnlock()
	if queue != nil {
		queue.mu.Lock()
		queue.quietHoursPolicy = provider
		queue.mu.Unlock()
	}
}

// SetTenantIdentityResolver installs an org-backed resolver for the tenant
// identity stamped into webhook payloads. It overrides the
// PULSE_TENANT_ID / PULSE_TENANT_NAME environment defaults; multi-tenant
// deployments use it so each org's manager reports that org's identity.
func (n *NotificationManager) SetTenantIdentityResolver(resolver func() (id string, name string)) {
	n.mu.Lock()
	n.tenantResolver = resolver
	n.mu.Unlock()
}

// TenantIdentity returns the tenant ID and display name stamped into webhook
// payloads. The name falls back to the ID so receivers always get a usable
// label when any tenant identity is configured at all.
func (n *NotificationManager) TenantIdentity() (string, string) {
	n.mu.RLock()
	resolver := n.tenantResolver
	id, name := n.tenantID, n.tenantName
	n.mu.RUnlock()

	if resolver != nil {
		if rid, rname := resolver(); strings.TrimSpace(rid) != "" {
			id, name = strings.TrimSpace(rid), strings.TrimSpace(rname)
		}
	}
	if name == "" {
		name = id
	}
	return id, name
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
func (n *NotificationManager) SetEmailConfig(config EmailConfig) error {
	if err := n.ConfigurationError(); err != nil {
		return err
	}
	if !n.lifecycle.join() {
		return fmt.Errorf("notification manager stopping")
	}
	defer n.lifecycle.release()
	n.destinationMu.Lock()
	unlockDestination := sync.OnceFunc(n.destinationMu.Unlock)
	defer unlockDestination()
	n.mu.Lock()
	config = normalizeEmailConfig(copyEmailConfig(config))
	oldKey := configuredDeliveryDestinationKey(notificationDeliveryJob{Type: "email", EmailConfig: &n.emailConfig})
	oldFingerprint := notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "email", EmailConfig: &n.emailConfig})
	newKey := configuredDeliveryDestinationKey(notificationDeliveryJob{Type: "email", EmailConfig: &config})
	if oldFingerprint != notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "email", EmailConfig: &config}) {
		newKey = ""
	}

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
	preparedManager := NewEnhancedEmailManager(providerConfig)
	var queue *NotificationQueue
	if !config.Enabled && n.queue != nil {
		queue = n.queue
	}
	n.mu.Unlock()
	if err := n.retireDestination(oldKey, newKey); err != nil {
		return fmt.Errorf("retire previous email destination: %w", err)
	}
	n.mu.Lock()
	n.emailConfig, n.emailManager = config, preparedManager
	n.destinationRevision++
	n.mu.Unlock()

	unlockDestination()

	if queue != nil {
		if err := queue.CancelByTypes([]string{"email", "email" + queueTypeSuffixResolved}, "Email notifications disabled"); err != nil {
			log.Error().Err(err).Msg("failed to cancel queued email notifications after disabling email delivery")
		}
	}
	return nil
}

// SetAppriseConfig updates Apprise configuration.
func (n *NotificationManager) SetAppriseConfig(config AppriseConfig) error {
	if err := n.ConfigurationError(); err != nil {
		return err
	}
	if !n.lifecycle.join() {
		return fmt.Errorf("notification manager stopping")
	}
	defer n.lifecycle.release()
	n.destinationMu.Lock()
	unlockDestination := sync.OnceFunc(n.destinationMu.Unlock)
	defer unlockDestination()
	n.mu.Lock()
	config = NormalizeAppriseConfig(config)
	oldKey := configuredDeliveryDestinationKey(notificationDeliveryJob{Type: "apprise", AppriseConfig: &n.appriseConfig})
	oldFingerprint := notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "apprise", AppriseConfig: &n.appriseConfig})
	newKey := configuredDeliveryDestinationKey(notificationDeliveryJob{Type: "apprise", AppriseConfig: &config})
	if oldFingerprint != notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "apprise", AppriseConfig: &config}) {
		newKey = ""
	}
	var queue *NotificationQueue
	if !config.Enabled && n.queue != nil {
		queue = n.queue
	}
	n.mu.Unlock()
	if err := n.retireDestination(oldKey, newKey); err != nil {
		return fmt.Errorf("retire previous Apprise destination: %w", err)
	}
	n.mu.Lock()
	n.appriseConfig = config
	n.destinationRevision++
	n.mu.Unlock()

	unlockDestination()

	if queue != nil {
		if err := queue.CancelByTypes([]string{"apprise", "apprise" + queueTypeSuffixResolved}, "Apprise notifications disabled"); err != nil {
			log.Error().Err(err).Msg("failed to cancel queued Apprise notifications after disabling delivery")
		}
	}
	return nil
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
	if !enabled {
		n.takePendingResolvedLocked()
	}
	n.mu.Unlock()

	if was != enabled {
		log.Info().Bool("enabled", enabled).Msg("updated resolved alert notifications")
	}
}

// SetInitialNotifyTarget selects destinations for firing and grouped
// notifications. Matching recovery delivery follows firing delivery receipts,
// while escalation levels retain their own per-level routing.
func (n *NotificationManager) SetInitialNotifyTarget(target string) {
	normalized := normalizeNotificationDeliveryTarget(target)
	n.mu.Lock()
	was := n.initialTarget
	n.initialTarget = normalized
	n.mu.Unlock()

	if was != normalized {
		log.Info().Str("target", string(normalized)).Msg("updated initial alert notification target")
	}
}

// GetInitialNotifyTarget returns the normalized initial delivery target.
func (n *NotificationManager) GetInitialNotifyTarget() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return string(n.initialTarget)
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

// SetGroupingConfig atomically applies the complete grouping policy. Disabling
// grouping (or selecting a zero-second window) flushes already-pending alerts
// as individual deliveries so a live config change cannot merge or lose them.
func (n *NotificationManager) SetGroupingConfig(enabled bool, seconds int, byNode, byGuest bool) {
	n.destinationMu.Lock()
	if seconds < 0 {
		seconds = 0
	}

	n.mu.Lock()
	n.groupingEnabled = enabled
	n.groupWindow = time.Duration(seconds) * time.Second
	n.groupByNode = byNode
	n.groupByGuest = byGuest

	var pending []*alerts.Alert
	var resolved []*alerts.Alert
	var emailConfig EmailConfig
	var webhooks []WebhookConfig
	var appriseConfig AppriseConfig
	var initialTarget notificationDeliveryTarget
	var queue *NotificationQueue
	if (!enabled || seconds == 0) && n.lifecycle.accepting() {
		resolved = n.takePendingResolvedLocked()
		// Only a live manager may flush to delivery; Stop checkpoints any
		// remaining buffer without bypassing its deadline.
		if len(n.pendingAlerts) > 0 && n.lifecycle.admit() {
			pending = append(pending, n.pendingAlerts...)
		}
		clear(n.pendingAlerts)
		n.pendingAlerts = n.pendingAlerts[:0]
		n.pendingAlertIndex = nil
		if n.groupTimer != nil {
			n.groupTimer.Stop()
			n.groupTimer = nil
			n.groupDeadline = time.Time{}
		}
		emailConfig = copyEmailConfig(n.emailConfig)
		webhooks = copyWebhookConfigs(n.webhooks)
		appriseConfig = copyAppriseConfig(n.appriseConfig)
		initialTarget = n.initialTarget
		queue = n.queue
	}
	n.mu.Unlock()
	n.destinationMu.Unlock()
	if len(pending) > 0 {
		defer n.lifecycle.release()
	}

	for _, alert := range resolved {
		n.dispatchResolvedAlerts([]*alerts.Alert{alert})
	}
	for _, alert := range pending {
		n.dispatchFiringAlerts(emailConfig, webhooks, appriseConfig, []*alerts.Alert{alert}, initialTarget, nil, queue)
	}

	log.Info().
		Bool("enabled", enabled).
		Int("seconds", seconds).
		Bool("byNode", byNode).
		Bool("byGuest", byGuest).
		Int("flushedIndividually", len(pending)).
		Msg("updated notification grouping configuration")
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
	if n.ConfigurationError() != nil {
		return
	}
	n.destinationMu.Lock()
	defer n.destinationMu.Unlock()
	n.mu.Lock()
	defer n.mu.Unlock()
	n.webhooks = append(n.webhooks, copyWebhookConfig(NormalizeWebhookConfig(webhook)))
	n.destinationRevision++
}

func (n *NotificationManager) UpdateWebhook(id string, webhook WebhookConfig) error {
	if err := n.ConfigurationError(); err != nil {
		return err
	}
	n.destinationMu.Lock()
	defer n.destinationMu.Unlock()
	n.mu.Lock()
	for i, w := range n.webhooks {
		if w.ID == id {
			oldKey := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &w})
			prepared := copyWebhookConfig(NormalizeWebhookConfig(webhook))
			newKey := configuredDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &prepared})
			if notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "webhook", WebhookConfig: &w}) != notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "webhook", WebhookConfig: &prepared}) {
				newKey = ""
			}
			aliases := removedLegacyWebhookAliases(n.webhooks, i, &prepared)
			n.mu.Unlock()
			if err := n.retireDestination(oldKey, newKey, aliases...); err != nil {
				return fmt.Errorf("retire previous webhook destination: %w", err)
			}
			n.mu.Lock()
			n.webhooks[i] = prepared
			n.destinationRevision++
			n.mu.Unlock()
			return nil
		}
	}
	n.mu.Unlock()
	return fmt.Errorf("webhook not found: %s", id)
}

func (n *NotificationManager) DeleteWebhook(id string) error {
	if err := n.ConfigurationError(); err != nil {
		return err
	}
	n.destinationMu.Lock()
	defer n.destinationMu.Unlock()
	n.mu.Lock()
	for i, w := range n.webhooks {
		if w.ID == id {
			key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &w})
			aliases := removedLegacyWebhookAliases(n.webhooks, i, nil)
			n.mu.Unlock()
			if err := n.retireDestination(key, "", aliases...); err != nil {
				return fmt.Errorf("retire removed webhook destination: %w", err)
			}
			n.mu.Lock()
			n.webhooks = append(n.webhooks[:i], n.webhooks[i+1:]...)
			n.destinationRevision++
			n.mu.Unlock()
			return nil
		}
	}
	n.mu.Unlock()
	return fmt.Errorf("webhook not found: %s", id)
}

func removedLegacyWebhookAliases(hooks []WebhookConfig, index int, replacement *WebhookConfig) []string {
	old := hooks[index]
	if !old.Enabled {
		return nil
	}
	key, _ := legacyWebhookDestination(old)
	if replacement != nil && replacement.Enabled {
		next, _ := legacyWebhookDestination(*replacement)
		if key == next {
			return nil
		}
	}
	for i, hook := range hooks {
		if i != index && hook.Enabled {
			other, _ := legacyWebhookDestination(hook)
			if key == other {
				return nil
			}
		}
	}
	return []string{key}
}

// GetWebhooks returns all webhook configurations
func (n *NotificationManager) GetWebhooks() []WebhookConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if len(n.webhooks) == 0 {
		return []WebhookConfig{}
	}

	return copyWebhookConfigs(n.webhooks)
}

// GetEmailConfig returns the email configuration
func (n *NotificationManager) GetEmailConfig() EmailConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return copyEmailConfig(n.emailConfig)
}

// GetQueue returns the notification queue
func (n *NotificationManager) GetQueue() *NotificationQueue {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.queue
}

// SetEnabled toggles notification delivery globally for this runtime instance.
func (n *NotificationManager) SetEnabled(enabled bool) {
	var (
		queue   *NotificationQueue
		changed bool
	)

	n.mu.Lock()
	changed = n.enabled != enabled
	n.enabled = enabled
	if !enabled {
		n.takePendingResolvedLocked()
		clear(n.pendingAlerts)
		n.pendingAlerts = n.pendingAlerts[:0]
		n.pendingAlertIndex = nil
		if n.groupTimer != nil {
			n.groupTimer.Stop()
			n.groupTimer = nil
			n.groupDeadline = time.Time{}
		}
		if n.queue != nil && n.lifecycle.join() {
			queue = n.queue
		}
	}
	n.mu.Unlock()

	if changed {
		log.Info().Bool("enabled", enabled).Msg("updated notification manager enabled state")
	}

	if queue != nil {
		defer n.lifecycle.release()
		types := []string{
			"email", "email" + queueTypeSuffixResolved,
			"webhook", "webhook" + queueTypeSuffixResolved,
			"apprise", "apprise" + queueTypeSuffixResolved,
		}
		if err := queue.CancelByTypes(types, "Notifications disabled"); err != nil {
			log.Error().Err(err).Msg("failed to cancel queued notifications after disabling notification manager")
		}
	}
}

// IsEnabled reports whether notification delivery is currently enabled.
func (n *NotificationManager) IsEnabled() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.enabled
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
	if err := n.ConfigurationError(); err != nil {
		return nil, err
	}
	n.mu.RLock()
	queue := n.queue
	n.mu.RUnlock()

	if queue == nil {
		return nil, fmt.Errorf("notification queue not initialized")
	}
	return queue.GetQueueStats()
}

// GetTelemetryStats returns only aggregate delivery outcomes from the
// persistent queue. It never exposes destinations, alert content, or IDs.
func (n *NotificationManager) GetTelemetryStats(since time.Time) (TelemetryStats, error) {
	n.mu.RLock()
	queue := n.queue
	n.mu.RUnlock()

	if queue == nil {
		return TelemetryStats{}, nil
	}
	return queue.GetTelemetryStats(since)
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
		return n.sendSingleEmailWithError(testAlert, n.emailConfig)
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
		return n.SendTestWebhook(*webhookToTest)
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
		Bool("hasServerURL", cfg.ServerURL != "").
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
	_, _, err := n.TestEnhancedWebhook(BuildEnhancedWebhookTestConfig(webhook, webhook.Service))
	return err
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

		return n.sendSingleEmailWithError(testAlert, *config)

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
func (n *NotificationManager) ProcessQueuedNotification(notif *QueuedNotification) (resultErr error) {
	defer func() {
		// Once the provider acknowledgement is durable, decoding/binding errors
		// are local settlement faults, never a new provider failure verdict.
		if notif.ProviderAcceptedAt != nil && resultErr != nil && !errors.Is(resultErr, errNotificationSettlementPending) {
			resultErr = notificationSettlementError(resultErr)
		}
	}()
	baseType, event := normalizeQueueType(notif.Type)
	if q := n.GetQueue(); q != nil && notif.ProviderAcceptedAt == nil {
		at, err := q.providerAcknowledgedAt(notif.ID)
		if err != nil {
			return notificationSettlementError(err)
		}
		notif.ProviderAcceptedAt = at
	}
	if q := n.GetQueue(); q != nil && notif.ProviderAcceptedAt != nil {
		if found, err := q.settleAcknowledgedNotification(notif.ID); found || err != nil {
			return notificationSettlementError(err)
		}
	}
	if notif.ProviderAcceptedAt == nil && rehearsalExpired(notif.Alerts, time.Now()) {
		return ErrNotificationDeliverySkipped
	}
	if notif.ProviderAcceptedAt == nil {
		if q := n.GetQueue(); q != nil && q.rehearsalEvidenceUnavailable(notif.Alerts) {
			return ErrNotificationDeliverySkipped
		}
	}
	if notif.ReceiptEpoch != nil {
		if err := validateReceiptEpoch(*notif.ReceiptEpoch); err != nil {
			return failNotificationConfiguration("invalid retained notification destination generation", err)
		}
	}

	log.Debug().
		Str("notificationID", notif.ID).
		Str("type", baseType).
		Str("event", string(event)).
		Int("alertCount", len(notif.Alerts)).
		Msg("processing queued notification")

	var (
		err          error
		deliveredJob notificationDeliveryJob
	)
	switch baseType {
	case "email":
		var emailConfig EmailConfig
		if err = json.Unmarshal(notif.Config, &emailConfig); err != nil {
			return failNotificationConfiguration("failed to unmarshal email config", err)
		}
		currentEmailConfig := n.GetEmailConfig()
		if notif.ProviderAcceptedAt == nil && (!n.IsEnabled() || !currentEmailConfig.Enabled) {
			log.Info().
				Str("notificationID", notif.ID).
				Str("type", baseType).
				Str("event", string(event)).
				Msg("skipping queued email notification because email delivery is disabled")
			return ErrNotificationDeliverySkipped
		}
		deliveredJob = notificationDeliveryJob{
			QueueID:      notif.ID,
			Type:         "email",
			ReceiptEpoch: queuedReceiptEpoch(notif), ReceiptEpochBound: notif.ReceiptEpoch != nil,
			Event:       event,
			Alerts:      notif.Alerts,
			ResolvedAt:  resolvedTimeFromAlerts(notif.Alerts),
			EmailConfig: &emailConfig,
		}
		deliveredJob, err = n.prepareQueuedReceiptEpoch(notif, deliveredJob)
		if err == nil {
			err = n.deliverQueuedNotificationJob(notif, deliveredJob)
		}

	case "webhook":
		var webhookConfig WebhookConfig
		if err = json.Unmarshal(notif.Config, &webhookConfig); err != nil {
			return failNotificationConfiguration("failed to unmarshal webhook config", err)
		}
		if notif.ProviderAcceptedAt == nil && (!n.IsEnabled() || !n.isQueuedWebhookStillEnabled(webhookConfig)) {
			log.Info().
				Str("notificationID", notif.ID).
				Str("type", baseType).
				Str("event", string(event)).
				Str("webhookID", webhookConfig.ID).
				Msg("skipping queued webhook notification because delivery is disabled")
			return ErrNotificationDeliverySkipped
		}
		deliveredJob = notificationDeliveryJob{
			QueueID:      notif.ID,
			Type:         "webhook",
			ReceiptEpoch: queuedReceiptEpoch(notif), ReceiptEpochBound: notif.ReceiptEpoch != nil,
			Event:         event,
			Alerts:        notif.Alerts,
			ResolvedAt:    resolvedTimeFromAlerts(notif.Alerts),
			WebhookConfig: &webhookConfig,
		}
		deliveredJob, err = n.prepareQueuedReceiptEpoch(notif, deliveredJob)
		if err == nil {
			err = n.deliverQueuedNotificationJob(notif, deliveredJob)
		}

	case "apprise":
		var appriseConfig AppriseConfig
		if err = json.Unmarshal(notif.Config, &appriseConfig); err != nil {
			return failNotificationConfiguration("failed to unmarshal apprise config", err)
		}
		currentAppriseConfig := n.GetAppriseConfig()
		if notif.ProviderAcceptedAt == nil && (!n.IsEnabled() || !currentAppriseConfig.Enabled) {
			log.Info().
				Str("notificationID", notif.ID).
				Str("type", baseType).
				Str("event", string(event)).
				Msg("skipping queued Apprise notification because delivery is disabled")
			return ErrNotificationDeliverySkipped
		}
		deliveredJob = notificationDeliveryJob{
			QueueID:      notif.ID,
			Type:         "apprise",
			ReceiptEpoch: queuedReceiptEpoch(notif), ReceiptEpochBound: notif.ReceiptEpoch != nil,
			Event:         event,
			Alerts:        notif.Alerts,
			ResolvedAt:    resolvedTimeFromAlerts(notif.Alerts),
			AppriseConfig: &appriseConfig,
		}
		deliveredJob, err = n.prepareQueuedReceiptEpoch(notif, deliveredJob)
		if err == nil {
			err = n.deliverQueuedNotificationJob(notif, deliveredJob)
		}

	default:
		return FailfWithClass(NotificationFailureConfiguration, "unknown notification type")
	}

	var acknowledgementErr *notificationAcknowledgementError
	if errors.As(err, &acknowledgementErr) {
		// An unavailable audit must not prevent independent durable receipt
		// evidence from preserving ordinary cooldown/recovery when it can commit.
		if receiptErr := n.recordSuccessfulDelivery(deliveredJob, time.Now()); receiptErr != nil {
			log.Error().Err(receiptErr).Msg("Provider acknowledgement and receipt both remain unsettled")
		}
	}
	if err == nil {
		if q := n.GetQueue(); q != nil && notif.ProviderAcceptedAt != nil {
			if found, settleErr := q.settleAcknowledgedNotification(notif.ID); found || settleErr != nil {
				if settleErr == nil && event == eventAlert && notificationDeliveryDestinationKey(deliveredJob) != "" {
					n.markAlertsNotified(notif.Alerts, time.Now())
				}
				return notificationSettlementError(settleErr)
			}
		}
		at := time.Now()
		if notif.ProviderAcceptedAt != nil {
			at = *notif.ProviderAcceptedAt
		}
		err = notificationSettlementError(n.recordSuccessfulDelivery(deliveredJob, at))
	}

	if err != nil {
		return fmt.Errorf("process queued %s notification %q (%s): %w", baseType, notif.ID, event, err)
	}
	return nil
}

func (n *NotificationManager) isQueuedWebhookStillEnabled(queuedWebhook WebhookConfig) bool {
	if !queuedWebhook.Enabled {
		return false
	}

	currentWebhooks := n.GetWebhooks()
	if len(currentWebhooks) == 0 {
		return false
	}

	if queuedWebhook.ID != "" {
		for _, current := range currentWebhooks {
			if current.ID == queuedWebhook.ID {
				return current.Enabled
			}
		}
		// A removed destination cannot borrow another destination's permission
		// just because it shares the URL. Retained work carries the old payload
		// and credentials; only legacy work without an ID may fall back to URL.
		return false
	}

	if queuedWebhook.URL == "" {
		return false
	}

	for _, current := range currentWebhooks {
		if current.URL == queuedWebhook.URL {
			return current.Enabled
		}
	}

	return false
}

// cleanupOldNotificationRecords periodically cleans up old entries from lastNotified map
func (n *NotificationManager) cleanupOldNotificationRecords() {
	if n.cleanupDone != nil {
		defer close(n.cleanupDone)
	}

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n.mu.Lock()
			now := time.Now()
			cutoff := now.Add(-24 * time.Hour)
			cleaned := 0

			for key, record := range n.deliveryCooldown {
				if record.lastSent.Before(cutoff) {
					delete(n.deliveryCooldown, key)
				}
			}

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
	n.stopOnce.Do(func() {
		// Refuse new admissions and stop the firing timer, but keep its snapshots
		// until the queue is quiescent. Shutdown checkpoints the accepted batch
		// without sending it or waiting for its grouping deadline.
		n.mu.Lock()
		n.lifecycle.drain()
		n.takePendingResolvedLocked()
		if n.groupTimer != nil {
			n.groupTimer.Stop()
			n.groupTimer = nil
		}
		queue := n.queue
		n.mu.Unlock()

		// Shutdown is a restart boundary, not a delivery verdict. Quiesce the
		// queue while this manager still permits delivery and records receipts
		// durably: a queued job reaching the processor after delivery is
		// withdrawn below would be cancelled as disabled, and a send completing
		// after the queue is detached would keep its firing receipt only in
		// memory. Rows no worker has begun stay pending for the next start.
		if queue != nil {
			queue.stopProcessing()
		}
		// Finish every admitted read/write before the final buffer snapshot. An
		// admitted SendAlert can still append after a slow cooldown read while
		// draining; its lease owns that append as well as the storage read.
		n.lifecycle.close()
		// Workers and admitted policy changes have finished. Keep the remaining
		// buffer and destination inventory stable through the queue handoff.
		n.destinationMu.Lock()
		n.mu.Lock()
		if n.groupTimer != nil {
			n.groupTimer.Stop()
			n.groupTimer = nil
		}
		n.checkpointPendingFiringGroupLocked(queue)
		n.mu.Unlock()
		n.destinationMu.Unlock()

		n.mu.Lock()
		n.enabled = false
		cleanupDone := n.cleanupDone
		client := n.webhookClient

		// Stop cleanup goroutine. Do not nil out the channel — the cleanup
		// goroutine reads n.stopCleanup in its select loop, and nilling it
		// between close and the goroutine re-entering select causes a race
		// where the goroutine blocks on a nil channel forever.
		if n.stopCleanup != nil {
			close(n.stopCleanup)
		}

		// Nil out queue before unlocking
		n.queue = nil
		n.mu.Unlock()

		// Wait for cleanup goroutine outside the lock to avoid deadlock.
		if cleanupDone != nil {
			<-cleanupDone
		}

		// Stop the notification queue if it exists
		if queue != nil {
			if err := queue.Stop(); err != nil {
				log.Warn().Err(err).Msg("Notification queue stop returned error")
			}
		}

		// Explicitly release pooled webhook connections during shutdown.
		if client != nil {
			client.CloseIdleConnections()
		}

		log.Info().Msg("NotificationManager stopped")
	})
}

func hashDeliveryDestination(kind, identity string) string {
	digest := sha256.Sum256([]byte(kind + "\x00" + identity))
	return kind + ":" + hex.EncodeToString(digest[:16])
}
