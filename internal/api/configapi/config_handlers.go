package configapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/tlsutil"
	"github.com/rs/zerolog/log"
)

var (
	setupAuthTokenPattern   = regexp.MustCompile(`^[A-Fa-f0-9]{32,128}$`)
	installerURLHostPattern = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
	pulseTokenSlugPattern   = regexp.MustCompile(`[^a-z0-9]+`)
)

func sanitizeInstallerURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if strings.ContainsAny(trimmed, "\r\n") {
		return "", fmt.Errorf("value must not contain control characters")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("host component is required")
	}

	// Installer URLs are embedded into shell scripts; reject credentials and
	// query/fragment components to reduce injection and ambiguity risk.
	if parsed.User != nil {
		return "", fmt.Errorf("userinfo is not allowed")
	}
	if parsed.RawQuery != "" {
		return "", fmt.Errorf("query parameters are not allowed")
	}
	if parsed.Fragment != "" {
		return "", fmt.Errorf("fragment is not allowed")
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", fmt.Errorf("host component is required")
	}
	if strings.ContainsAny(host, "$`\\\"' \t\r\n") {
		return "", fmt.Errorf("host contains invalid characters")
	}
	if ip := net.ParseIP(host); ip == nil {
		if !installerURLHostPattern.MatchString(host) {
			return "", fmt.Errorf("host contains invalid characters")
		}
	}
	if port := strings.TrimSpace(parsed.Port()); port != "" {
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return "", fmt.Errorf("port must be between 1 and 65535")
		}
	}

	sanitized := parsed.String()
	if strings.ContainsAny(sanitized, "$`\\") {
		return "", fmt.Errorf("URL contains unsupported shell-expansion characters")
	}

	return sanitized, nil
}

func sanitizeSetupAuthToken(token string) (string, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return "", nil
	}
	if strings.ContainsAny(trimmed, "\r\n") {
		return "", fmt.Errorf("token must not contain control characters")
	}
	if !setupAuthTokenPattern.MatchString(trimmed) {
		return "", fmt.Errorf("token must be hexadecimal")
	}
	return trimmed, nil
}

// buildPulseMonitorTokenName returns a deterministic Pulse-managed token name.
// The result is stable across reruns for the same Pulse instance.
func buildPulseMonitorTokenName(candidates ...string) string {
	return "pulse-" + pulseTokenSuffix(candidates...)
}

func BuildPulseMonitorTokenName(candidates ...string) string {
	return buildPulseMonitorTokenName(candidates...)
}

func pulseTokenSuffix(candidates ...string) string {
	for _, candidate := range candidates {
		host := pulseTokenHostCandidate(candidate)
		if host == "" {
			continue
		}
		slug := pulseTokenSlug(host)
		if slug != "" {
			return slug
		}
	}
	return "server"
}

func pulseTokenHostCandidate(candidate string) string {
	raw := strings.TrimSpace(candidate)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		if parsed, err := url.Parse(raw); err == nil && parsed.Hostname() != "" {
			return parsed.Hostname()
		}
	}

	if host, _, err := net.SplitHostPort(raw); err == nil {
		return strings.Trim(host, "[]")
	}

	if parsed, err := url.Parse("https://" + raw); err == nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}

	return strings.Trim(raw, "[]")
}

// PulseTokenHostCandidate normalizes a URL or authority into the host identity
// used for deterministic Pulse-managed token naming.
func PulseTokenHostCandidate(candidate string) string {
	return pulseTokenHostCandidate(candidate)
}

func pulseTokenSlug(raw string) string {
	trimmed := strings.Trim(strings.ToLower(strings.TrimSpace(raw)), ".")
	if trimmed == "" {
		return ""
	}

	slug := pulseTokenSlugPattern.ReplaceAllString(trimmed, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return ""
	}

	const maxSlugLen = 48
	if len(slug) > maxSlugLen {
		slug = strings.Trim(slug[:maxSlugLen], "-")
	}

	return slug
}

// isPulseAgentToken returns true if the token ID was created by the Pulse agent.
// Agent tokens follow "pulse-monitor@pve!pulse-<scope>" (PVE) or
// "pulse-monitor@pbs!pulse-<scope>" (PBS). Legacy tokens may include a timestamp suffix.
func isPulseAgentToken(tokenID string) bool {
	return strings.HasPrefix(tokenID, "pulse-monitor@pve!pulse-") ||
		strings.HasPrefix(tokenID, "pulse-monitor@pbs!pulse-")
}

// SetupTokenRecord represents a one-time setup token for secure node registration.
type SetupTokenRecord struct {
	ExpiresAt time.Time
	Used      bool
	NodeType  string // "pve" or "pbs"
	Host      string // The host URL for validation
	OrgID     string // Organization ID creating this code
	// DesiredName carries the connection name the user typed in the add
	// dialog, so auto-registration can honor it instead of falling back to
	// the node's own hostname.
	DesiredName string
}

// RecentSetupTokenRecord preserves minimal identity metadata for the short
// follow-up grace window after a setup token has been consumed.
type RecentSetupTokenRecord struct {
	ExpiresAt time.Time
	NodeType  string
	Host      string
	OrgID     string
}

// ConfigHandlers handles configuration-related API endpoints
type ConfigHandlers struct {
	stateMu       sync.RWMutex
	mtPersistence *config.MultiTenantPersistence
	mtMonitor     *monitoring.MultiTenantMonitor
	hostedMode    bool
	// Default-org runtime fields used when tenant-specific state is unavailable.
	defaultConfig      *config.Config
	defaultPersistence *config.ConfigPersistence
	defaultMonitor     *monitoring.Monitor

	reloadFunc               func() error
	reloadSystemSettingsFunc func() // Function to reload cached system settings
	mockModeChanged          func(bool)
	wsHub                    *websocket.Hub
	guestMetadataHandler     GuestMetadataReloader
	runtime                  RuntimeDependencies
	setupTokens              map[string]*SetupTokenRecord      // Map of token hash -> setup token details
	recentSetupTokens        map[string]RecentSetupTokenRecord // Temporary map for recently used setup tokens (grace period)
	codeMutex                sync.RWMutex                      // Mutex for thread-safe code access
	clusterDetectMutex       sync.Mutex
	lastClusterDetection     map[string]time.Time
	nodeEditMu               sync.Mutex
	nodeEdits                map[string]*nodeEditGate
	recentAutoRegistered     map[string]time.Time
	recentAutoRegMutex       sync.Mutex
	autoRegisterMutex        sync.Mutex
	tenantReloadMutex        sync.Mutex
	autoRegReloadMutex       sync.Mutex
	lastAutoRegReload        map[string]time.Time
	autoRegReloadQueued      map[string]bool
	importRequests           configImportRequests
	stopCleanup              chan struct{}
	stopCleanupOnce          sync.Once
	cleanupDone              chan struct{}
}

// NewConfigHandlers creates a new ConfigHandlers instance
func NewConfigHandlers(mtp *config.MultiTenantPersistence, mtm *monitoring.MultiTenantMonitor, reloadFunc func() error, wsHub *websocket.Hub, guestMetadataHandler GuestMetadataReloader, reloadSystemSettingsFunc func(), hostedMode bool) *ConfigHandlers {
	// Initialize with default-org values from multi-tenant managers when available.
	var defaultConfig *config.Config
	var defaultMonitor *monitoring.Monitor
	var defaultPersistence *config.ConfigPersistence

	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			defaultMonitor = m
			if m != nil {
				defaultConfig = m.GetConfig()
			}
		}
	}
	if mtp != nil {
		if p, err := mtp.GetPersistence("default"); err == nil {
			defaultPersistence = p
		}
	}

	h := &ConfigHandlers{
		mtPersistence:            mtp,
		mtMonitor:                mtm,
		hostedMode:               hostedMode,
		defaultConfig:            defaultConfig,
		defaultMonitor:           defaultMonitor,
		defaultPersistence:       defaultPersistence,
		reloadFunc:               reloadFunc,
		reloadSystemSettingsFunc: reloadSystemSettingsFunc,
		wsHub:                    wsHub,
		guestMetadataHandler:     guestMetadataHandler,
		setupTokens:              make(map[string]*SetupTokenRecord),
		recentSetupTokens:        make(map[string]RecentSetupTokenRecord),
		lastClusterDetection:     make(map[string]time.Time),
		recentAutoRegistered:     make(map[string]time.Time),
		runtime:                  defaultRuntimeDependencies(),
		stopCleanup:              make(chan struct{}),
		cleanupDone:              make(chan struct{}),
	}

	// Clean up expired setup tokens periodically until Stop.
	go h.cleanupExpiredSetupTokens()

	return h
}

// SetMultiTenantMonitor updates the monitor reference used by the config handlers.
func (h *ConfigHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	var defaultMonitor *monitoring.Monitor
	var defaultConfig *config.Config
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			defaultMonitor = m
			if m != nil {
				defaultConfig = m.GetConfig()
			}
		}
	}

	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.mtMonitor = mtm
	if defaultMonitor != nil {
		h.defaultMonitor = defaultMonitor
		h.defaultConfig = defaultConfig
	}
}

// MultiTenantMonitor returns the current owner used by scoped configuration
// and authentication, including after an instance-wide monitor replacement.
func (h *ConfigHandlers) MultiTenantMonitor() *monitoring.MultiTenantMonitor {
	h.stateMu.RLock()
	defer h.stateMu.RUnlock()
	return h.mtMonitor
}

// reloadMonitor applies a saved node change to the runtime that owns it. A
// non-default organization recreates only its own monitor, as configuration
// import does: the instance-wide hook stops every organization's monitoring
// and discards their in-memory alert state. The default organization and
// single-runtime handlers retain the instance-wide reload.
func (h *ConfigHandlers) reloadMonitor(ctx context.Context) error {
	orgID := apicontext.OrgID(ctx)
	if mtMonitor := h.MultiTenantMonitor(); mtMonitor != nil && orgID != "" && orgID != "default" {
		// ReloadTenant refuses an organization whose shutdown is in progress.
		// Queue concurrent node changes as the instance-wide reload does.
		h.tenantReloadMutex.Lock()
		defer h.tenantReloadMutex.Unlock()
		_, err := mtMonitor.ReloadTenant(orgID)
		return err
	}
	if h.reloadFunc != nil {
		return h.reloadFunc()
	}
	return nil
}

// SetMonitor updates the default-org monitor reference used by the config handlers.
func (h *ConfigHandlers) SetMonitor(m *monitoring.Monitor) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.defaultMonitor = m
	if m != nil {
		h.defaultConfig = m.GetConfig()
	}
}

// SetConfig updates the configuration reference used by the handlers.
func (h *ConfigHandlers) SetConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}

	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.defaultConfig = cfg
}

// SetMockModeChangeHook registers a callback invoked after mock mode changes.
func (h *ConfigHandlers) SetMockModeChangeHook(hook func(bool)) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.mockModeChanged = hook
}

// getContextState helper to retrieve tenant-specific state
func (h *ConfigHandlers) getContextState(ctx context.Context) (*config.Config, *config.ConfigPersistence, *monitoring.Monitor) {
	h.stateMu.RLock()
	mtMonitor := h.mtMonitor
	mtPersistence := h.mtPersistence
	defaultConfig := h.defaultConfig
	defaultPersistence := h.defaultPersistence
	defaultMonitor := h.defaultMonitor
	h.stateMu.RUnlock()

	orgID := "default"
	if ctx != nil {
		if requestOrgID := apicontext.OrgID(ctx); requestOrgID != "" {
			orgID = requestOrgID
		}
	}

	// The default org is the primary runtime owned by Router. Its config and
	// monitor are updated in place by setup and reload flows, while the
	// multi-tenant manager keeps isolated tenant copies. Never route default-org
	// requests back through that tenant-copy path or they can observe stale auth
	// and setup state.
	if orgID == "default" &&
		(defaultConfig != nil || defaultPersistence != nil || defaultMonitor != nil) {
		bindMonitorMetadataStores(defaultPersistence, defaultMonitor)
		return defaultConfig, defaultPersistence, defaultMonitor
	}

	// Resolve non-default organizations, plus initialization-only handlers
	// that have not yet been given the primary default runtime, from the
	// multi-tenant managers.
	if mtMonitor != nil {
		if m, err := mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			cfg := m.GetConfig()
			var p *config.ConfigPersistence
			if mtPersistence != nil {
				p, _ = mtPersistence.GetPersistence(orgID)
			}
			bindMonitorMetadataStores(p, m)
			return cfg, p, m
		} else {
			if orgID == "default" {
				log.Warn().Str("orgID", orgID).Err(err).Msg("Default config resolution failed before primary runtime initialization")
				return defaultConfig, defaultPersistence, defaultMonitor
			}
			log.Warn().Str("orgID", orgID).Err(err).Msg("Tenant config resolution failed for non-default org")
			return nil, nil, nil
		}
	}

	if orgID == "default" {
		return defaultConfig, defaultPersistence, defaultMonitor
	}
	// Single-runtime handlers have no tenant manager and therefore no
	// alternate state to resolve. Keep their historical default-state
	// behavior; multi-tenant handlers above continue to fail closed.
	if mtMonitor == nil {
		return defaultConfig, defaultPersistence, defaultMonitor
	}
	return nil, nil, nil
}

func (h *ConfigHandlers) ContextState(ctx context.Context) (*config.Config, *config.ConfigPersistence, *monitoring.Monitor) {
	return h.getContextState(ctx)
}

func bindMonitorMetadataStores(
	persistence *config.ConfigPersistence,
	monitor *monitoring.Monitor,
) {
	if persistence == nil || monitor == nil {
		return
	}
	persistence.SetMetadataStores(
		monitor.GuestMetadataStore(),
		monitor.DockerMetadataStore(),
		monitor.HostMetadataStore(),
	)
}

func (h *ConfigHandlers) getConfig(ctx context.Context) *config.Config {
	c, _, _ := h.getContextState(ctx)
	return c
}

func (h *ConfigHandlers) Config(ctx context.Context) *config.Config { return h.getConfig(ctx) }

func (h *ConfigHandlers) getPersistence(ctx context.Context) *config.ConfigPersistence {
	_, p, _ := h.getContextState(ctx)
	return p
}

func (h *ConfigHandlers) Persistence(ctx context.Context) *config.ConfigPersistence {
	return h.getPersistence(ctx)
}

func (h *ConfigHandlers) getMonitor(ctx context.Context) *monitoring.Monitor {
	_, _, m := h.getContextState(ctx)
	return m
}

func (h *ConfigHandlers) Monitor(ctx context.Context) *monitoring.Monitor { return h.getMonitor(ctx) }

func (h *ConfigHandlers) SetPersistence(persistence *config.ConfigPersistence) {
	h.stateMu.Lock()
	h.defaultPersistence = persistence
	h.stateMu.Unlock()
}

func (h *ConfigHandlers) HostedMode() bool {
	h.stateMu.RLock()
	defer h.stateMu.RUnlock()
	return h.hostedMode
}

func (h *ConfigHandlers) StoreSetupToken(hash string, record *SetupTokenRecord) {
	h.codeMutex.Lock()
	defer h.codeMutex.Unlock()
	if h.setupTokens == nil {
		h.setupTokens = make(map[string]*SetupTokenRecord)
	}
	h.setupTokens[hash] = record
}

func (h *ConfigHandlers) SetupTokenCount() int {
	h.codeMutex.RLock()
	defer h.codeMutex.RUnlock()
	return len(h.setupTokens)
}

func (h *ConfigHandlers) normalizePVEConfigState(ctx context.Context) {
	cfg := h.getConfig(ctx)
	if cfg == nil {
		return
	}
	if normalized, changed := config.ConsolidatePVEInstances(cfg.PVEInstances); changed {
		cfg.PVEInstances = normalized
	}
	config.EnsurePVEClusterNodeIdentities(cfg.PVEInstances)
}

// Stop ends the setup-token cleanup worker and waits for it to exit. The worker
// holds the handlers, and through their callbacks whatever owns them, so they
// stay reachable until it stops.
func (h *ConfigHandlers) Stop() {
	if h == nil || h.stopCleanup == nil {
		return
	}
	h.stopCleanupOnce.Do(func() { close(h.stopCleanup) })
	<-h.cleanupDone
}

// cleanupExpiredSetupTokens removes expired or used setup tokens periodically.
func (h *ConfigHandlers) cleanupExpiredSetupTokens() {
	defer close(h.cleanupDone)
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCleanup:
			return
		case <-ticker.C:
			h.removeExpiredSetupTokens(time.Now())
		}
	}
}

func (h *ConfigHandlers) removeExpiredSetupTokens(now time.Time) {
	h.codeMutex.Lock()
	defer h.codeMutex.Unlock()
	for tokenHash, tokenRecord := range h.setupTokens {
		if now.After(tokenRecord.ExpiresAt) || tokenRecord.Used {
			delete(h.setupTokens, tokenHash)
			log.Debug().Bool("was_used", tokenRecord.Used).Msg("Cleaned up setup token")
		}
	}
	for tokenHash, recentRecord := range h.recentSetupTokens {
		if now.After(recentRecord.ExpiresAt) {
			delete(h.recentSetupTokens, tokenHash)
		}
	}
}

func recentSetupTokenGraceExpiry(record *SetupTokenRecord, now time.Time) time.Time {
	graceExpiry := now.Add(1 * time.Minute)
	if record != nil && !record.ExpiresAt.IsZero() && record.ExpiresAt.Before(graceExpiry) {
		graceExpiry = record.ExpiresAt
	}
	return graceExpiry
}

func buildRecentSetupTokenRecord(record *SetupTokenRecord, expiresAt time.Time) RecentSetupTokenRecord {
	recentRecord := RecentSetupTokenRecord{
		ExpiresAt: expiresAt,
	}
	if record == nil {
		return recentRecord
	}
	recentRecord.NodeType = strings.TrimSpace(record.NodeType)
	recentRecord.Host = strings.TrimSpace(record.Host)
	recentRecord.OrgID = strings.TrimSpace(record.OrgID)
	return recentRecord
}

func normalizeSetupTokenOrgID(orgID string) string {
	trimmed := strings.TrimSpace(orgID)
	if trimmed == "" {
		return "default"
	}
	return trimmed
}

// ValidateSetupToken checks whether the provided temporary setup token is still valid.
func (h *ConfigHandlers) ValidateSetupToken(token string) bool {
	if token == "" {
		return false
	}

	tokenHash := internalauth.HashAPIToken(token)
	now := time.Now()

	h.codeMutex.RLock()
	defer h.codeMutex.RUnlock()

	if tokenRecord, exists := h.setupTokens[tokenHash]; exists {
		if !tokenRecord.Used && now.Before(tokenRecord.ExpiresAt) {
			return true
		}
	}

	if recentRecord, ok := h.recentSetupTokens[tokenHash]; ok && now.Before(recentRecord.ExpiresAt) {
		return true
	}

	return false
}

// ValidateSetupTokenForOrg checks whether the provided setup token is valid for
// the requested organization context. Tokens without explicit org binding are
// treated as default-org tokens for backward compatibility.
func (h *ConfigHandlers) ValidateSetupTokenForOrg(token, orgID string) bool {
	if token == "" {
		return false
	}

	requestOrgID := normalizeSetupTokenOrgID(orgID)

	tokenHash := internalauth.HashAPIToken(token)
	now := time.Now()

	h.codeMutex.RLock()
	defer h.codeMutex.RUnlock()

	if tokenRecord, exists := h.setupTokens[tokenHash]; exists {
		tokenOrgID := normalizeSetupTokenOrgID(tokenRecord.OrgID)
		if tokenRecord.Used {
			if recentRecord, ok := h.recentSetupTokens[tokenHash]; ok && now.Before(recentRecord.ExpiresAt) {
				return normalizeSetupTokenOrgID(recentRecord.OrgID) == requestOrgID
			}
			return false
		}
		if !now.Before(tokenRecord.ExpiresAt) {
			return false
		}
		return tokenOrgID == requestOrgID
	}

	if recentRecord, ok := h.recentSetupTokens[tokenHash]; ok && now.Before(recentRecord.ExpiresAt) {
		return normalizeSetupTokenOrgID(recentRecord.OrgID) == requestOrgID
	}

	return false
}

func (h *ConfigHandlers) markAutoRegistered(nodeType, nodeName string) {
	if nodeType == "" || nodeName == "" {
		return
	}
	key := nodeType + ":" + nodeName
	h.recentAutoRegMutex.Lock()
	h.recentAutoRegistered[key] = time.Now()
	h.recentAutoRegMutex.Unlock()
}

// autoRegisterReloadInterval spaces the monitor reloads that re-registrations
// of existing connections start. A reload stops and rebuilds the monitors of
// the target organization (or the instance for the default organization) and
// drops alerts still pending in them, while an agent token may re-register its
// own connection with a new secret as often as it likes.
var autoRegisterReloadInterval = time.Minute

// reloadAfterAutoRegister applies a saved auto-registration to the owning
// monitor without holding up the response. A new connection is picked up at
// once: creating one takes an administrator's setup token or one-time install
// grant. A change to an existing connection reloads at once when none did for
// that organization within autoRegisterReloadInterval, and changes that follow
// inside the interval share one reload at its end.
func (h *ConfigHandlers) reloadAfterAutoRegister(ctx context.Context, created bool) {
	orgID := apicontext.OrgID(ctx)
	if orgID == "" {
		orgID = "default"
	}
	scopedTenant := h.MultiTenantMonitor() != nil && orgID != "default"
	if !scopedTenant && h.reloadFunc == nil {
		return
	}
	var wait time.Duration
	if !created {
		h.autoRegReloadMutex.Lock()
		if h.lastAutoRegReload == nil {
			h.lastAutoRegReload = make(map[string]time.Time)
		}
		if h.autoRegReloadQueued == nil {
			h.autoRegReloadQueued = make(map[string]bool)
		}
		wait = time.Until(h.lastAutoRegReload[orgID].Add(autoRegisterReloadInterval))
		if wait > 0 && h.autoRegReloadQueued[orgID] {
			h.autoRegReloadMutex.Unlock()
			return
		}
		if wait > 0 {
			h.autoRegReloadQueued[orgID] = true
		} else {
			h.lastAutoRegReload[orgID] = time.Now()
		}
		h.autoRegReloadMutex.Unlock()
	}

	reloadCtx := context.WithoutCancel(ctx)
	go func() {
		if wait > 0 {
			select {
			case <-time.After(wait):
			case <-h.stopCleanup:
				return
			}
			// The reload reads what is saved when it begins; a change saved
			// after this point queues the next one.
			h.autoRegReloadMutex.Lock()
			h.autoRegReloadQueued[orgID] = false
			h.lastAutoRegReload[orgID] = time.Now()
			h.autoRegReloadMutex.Unlock()
		}
		if err := h.reloadMonitor(reloadCtx); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor after auto-registration")
		}
	}()
}

func (h *ConfigHandlers) clearAutoRegistered(nodeType, nodeName string) {
	if nodeType == "" || nodeName == "" {
		return
	}
	key := nodeType + ":" + nodeName
	h.recentAutoRegMutex.Lock()
	delete(h.recentAutoRegistered, key)
	h.recentAutoRegMutex.Unlock()
}

func (h *ConfigHandlers) isRecentlyAutoRegistered(nodeType, nodeName string) bool {
	if nodeType == "" || nodeName == "" {
		return false
	}
	key := nodeType + ":" + nodeName
	now := time.Now()
	h.recentAutoRegMutex.Lock()
	defer h.recentAutoRegMutex.Unlock()
	registeredAt, ok := h.recentAutoRegistered[key]
	if !ok {
		return false
	}
	if now.Sub(registeredAt) > 2*time.Minute {
		delete(h.recentAutoRegistered, key)
		return false
	}
	return true
}

func (h *ConfigHandlers) findInstanceNameByHost(ctx context.Context, nodeType, host string) string {
	switch nodeType {
	case "pve":
		for _, node := range h.getConfig(ctx).PVEInstances {
			if node.Host == host {
				return node.Name
			}
		}
	case "pbs":
		for _, node := range h.getConfig(ctx).PBSInstances {
			if node.Host == host {
				return node.Name
			}
		}
	}
	return ""
}

// sanitizeErrorMessage returns a safe error message for external responses.
// It logs the detailed error internally and never echoes raw error text,
// credentials, or internal addresses back to the client. For node connection
// operations ("create_client" and "connection"), it maps recognized failure
// classes to closed, actionable operator diagnostics.
func sanitizeErrorMessage(err error, operation string) string {
	// Log the detailed error internally
	log.Error().Err(err).Str("operation", operation).Msg("Operation failed")

	switch operation {
	case "create_client":
		if guidance := classifyConnectionTestError(err); guidance != "" {
			return guidance
		}
		return "Failed to initialize connection"
	case "connection":
		if guidance := classifyConnectionTestError(err); guidance != "" {
			return guidance
		}
		return "Connection failed. Please check your credentials and network settings"
	case "validation":
		return "Invalid configuration"
	default:
		return "Operation failed"
	}
}

// classifyConnectionTestError maps a connection or authentication error to a
// fixed, content-free diagnostic message so operators know what to fix without
// exposing raw error text, credentials, or internal host details.
func classifyConnectionTestError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Connection timed out. Verify Pulse can reach the host over the network or firewall on the configured port."
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "Hostname could not be resolved. Verify the hostname or enter the server's IP address."
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "Connection timed out. Verify Pulse can reach the host over the network or firewall on the configured port."
	}

	msg := strings.ToLower(err.Error())
	containsAny := func(needles ...string) bool {
		for _, needle := range needles {
			if strings.Contains(msg, needle) {
				return true
			}
		}
		return false
	}

	switch {
	case containsAny("status 403", "http 403", "403 forbidden", "permission check failed", "permission denied", "forbidden"):
		return "Permission denied (HTTP 403). Check read access for the intended resources. If the token uses Privilege Separation, check its own read grants."
	case containsAny("status 401", "http 401", "401 unauthorized", "unauthorized", "authentication failed", "invalid credentials", "invalid ticket", "login failed"):
		return "Authentication failed (HTTP 401). Verify the username with realm (for example root@pam) or API token ID and secret."
	case containsAny("server gave http response to https client", "malformed http response", "first record does not look like a tls handshake"):
		return "Protocol mismatch. Proxmox endpoints require https:// (for example https://host:8006 for PVE or :8007 for PBS)."
	case containsAny("x509", "certificate", "tls:", "unknown authority", "fingerprint mismatch", "handshake failure", "bad certificate"):
		return "TLS certificate verification failed. Keep verification enabled. Check the hostname and trust chain. Verify pins independently."
	case containsAny("connection refused"):
		return "Connection refused. Verify the host address and port (Proxmox VE uses 8006, PBS uses 8007) and confirm the service is running."
	case containsAny("deadline exceeded", "timed out", "i/o timeout"):
		return "Connection timed out. Verify Pulse can reach the host over the network or firewall on the configured port."
	case containsAny("no such host"):
		return "Hostname could not be resolved. Verify the hostname or enter the server's IP address."
	case containsAny("no route to host", "network is unreachable", "host is unreachable", "network unreachable"):
		return "Host is unreachable from Pulse. Check the IP address, VLAN, routing, or container network settings."
	default:
		return ""
	}
}

func normalizePVEUser(user string) string {
	user = strings.TrimSpace(user)
	if user == "" {
		return user
	}
	if strings.Contains(user, "@") {
		return user
	}
	return user + "@pam"
}

const clusterDetectionCooldown = 30 * time.Second

func shouldSkipClusterAutoDetection(host, name string) bool {
	if host == "" {
		return false
	}
	lowerHost := strings.ToLower(host)
	lowerName := strings.ToLower(name)
	return strings.Contains(lowerHost, "192.168.77.") ||
		strings.Contains(lowerHost, "192.168.88.") ||
		strings.Contains(lowerHost, "test-") ||
		strings.Contains(lowerName, "test-") ||
		strings.Contains(lowerName, "persist-") ||
		strings.Contains(lowerName, "concurrent-")
}

// findDetectedPVEInstance returns the configured connection that a cluster
// detection of probed describes, or nil once that connection has been removed
// or pointed at another endpoint or credential. Detection waits on the
// monitored server for as long as that server likes, and the connection list
// is edited meanwhile, so the result is stored on the connection found here
// and never through a list slot captured before the wait: a removal hands
// that slot to the next connection, whose credential would then be sent to
// the members the removed server reported. The caller holds config.Mu.
func findDetectedPVEInstance(cfg *config.Config, probed *config.PVEInstance) *config.PVEInstance {
	if cfg == nil {
		return nil
	}
	index, found := findInstanceIndexByName(cfg, "pve", probed.Name)
	if !found {
		return nil
	}
	current := &cfg.PVEInstances[index]
	if config.CreateProxmoxConfig(current) != config.CreateProxmoxConfig(probed) {
		return nil
	}
	return current
}

// maybeRefreshClusterInfo retries cluster detection for the connection that
// instance is a copy of and stores the result on that connection.
func (h *ConfigHandlers) maybeRefreshClusterInfo(ctx context.Context, instance config.PVEInstance) {
	if shouldSkipClusterAutoDetection(instance.Host, instance.Name) {
		return
	}

	// Require credentials to attempt detection
	if instance.TokenValue == "" && instance.Password == "" {
		return
	}

	trimmedName := strings.TrimSpace(instance.ClusterName)
	needsRefresh := !instance.IsCluster ||
		len(instance.ClusterEndpoints) == 0 ||
		trimmedName == "" ||
		strings.EqualFold(trimmedName, "unknown cluster")

	if !needsRefresh {
		return
	}

	h.clusterDetectMutex.Lock()
	last := h.lastClusterDetection[instance.Name]
	if time.Since(last) < clusterDetectionCooldown {
		h.clusterDetectMutex.Unlock()
		return
	}
	h.lastClusterDetection[instance.Name] = time.Now()
	h.clusterDetectMutex.Unlock()

	clientConfig := config.CreateProxmoxConfig(&instance)
	// Re-detection must not reach a saved cluster address past its endpoint pin.
	discoveryCfg := h.getConfig(ctx).Discovery
	fingerprint, err := monitoring.PVEEndpointFingerprint(instance, clientConfig.Host, discoveryCfg)
	if err != nil {
		log.Warn().Str("instance", instance.Name).Err(err).Msg("Skipping cluster validation retry until saved TLS trust is reconciled")
		return
	}
	clientConfig.Fingerprint = fingerprint
	isCluster, clusterName, clusterEndpoints := detectPVEClusterContext(ctx, clientConfig, instance.Name, clusterEndpointsWithSavedTrust(instance, discoveryCfg))
	if !isCluster || len(clusterEndpoints) == 0 {
		log.Debug().
			Str("instance", instance.Name).
			Bool("previous_cluster", instance.IsCluster).
			Msg("Cluster validation retry did not produce usable endpoints")
		return
	}

	trimmedCluster := strings.TrimSpace(clusterName)
	if trimmedCluster == "" || strings.EqualFold(trimmedCluster, "unknown cluster") {
		clusterName = instance.Name
	}

	config.Mu.Lock()
	current := findDetectedPVEInstance(h.getConfig(ctx), &instance)
	if current == nil {
		config.Mu.Unlock()
		log.Info().
			Str("instance", instance.Name).
			Msg("Discarded cluster validation retry for a connection that was removed or changed meanwhile")
		return
	}
	current.IsCluster = true
	current.ClusterName = clusterName
	current.ClusterEndpoints = clusterEndpoints
	config.Mu.Unlock()

	log.Info().
		Str("instance", instance.Name).
		Str("cluster", clusterName).
		Int("endpoints", len(clusterEndpoints)).
		Msg("Updated cluster metadata after validation retry")

	if h.getPersistence(ctx) != nil {
		h.normalizePVEConfigState(ctx)
		if err := h.getPersistence(ctx).SaveNodesConfig(h.getConfig(ctx).PVEInstances, h.getConfig(ctx).PBSInstances, h.getConfig(ctx).PMGInstances); err != nil {
			log.Warn().
				Err(err).
				Str("instance", instance.Name).
				Msg("Failed to persist cluster detection update")
		}
	}
}

// NodeConfigRequest represents a request to add/update a node
type NodeConfigRequest struct {
	Type                         string   `json:"type"` // "pve", "pbs", or "pmg"
	Name                         string   `json:"name"`
	Host                         string   `json:"host"`
	GuestURL                     string   `json:"guestURL,omitempty"` // Optional guest-accessible URL (for navigation)
	User                         string   `json:"user,omitempty"`
	Password                     string   `json:"password,omitempty"`
	TokenName                    string   `json:"tokenName,omitempty"`
	TokenValue                   string   `json:"tokenValue,omitempty"`
	TokenID                      string   `json:"tokenId,omitempty"`
	TokenSecret                  string   `json:"tokenSecret,omitempty"`
	Fingerprint                  string   `json:"fingerprint,omitempty"`
	VerifySSL                    *bool    `json:"verifySSL,omitempty"`
	MonitorVMs                   *bool    `json:"monitorVMs,omitempty"`                   // PVE only
	MonitorContainers            *bool    `json:"monitorContainers,omitempty"`            // PVE only
	MonitorStorage               *bool    `json:"monitorStorage,omitempty"`               // PVE only
	MonitorBackups               *bool    `json:"monitorBackups,omitempty"`               // PVE only
	MonitorPhysicalDisks         *bool    `json:"monitorPhysicalDisks,omitempty"`         // PVE only (nil = enabled by default)
	PhysicalDiskPollingMinutes   *int     `json:"physicalDiskPollingMinutes,omitempty"`   // PVE only (0 = default 5m)
	TemperatureMonitoringEnabled *bool    `json:"temperatureMonitoringEnabled,omitempty"` // All types (nil = use global setting)
	MonitorDatastores            *bool    `json:"monitorDatastores,omitempty"`            // PBS only
	MonitorSyncJobs              *bool    `json:"monitorSyncJobs,omitempty"`              // PBS only
	MonitorVerifyJobs            *bool    `json:"monitorVerifyJobs,omitempty"`            // PBS only
	MonitorPruneJobs             *bool    `json:"monitorPruneJobs,omitempty"`             // PBS only
	MonitorGarbageJobs           *bool    `json:"monitorGarbageJobs,omitempty"`           // PBS only
	ExcludeDatastores            []string `json:"excludeDatastores,omitempty"`            // PBS only - datastores to exclude from monitoring
	MonitorMailStats             *bool    `json:"monitorMailStats,omitempty"`             // PMG only
	MonitorQueues                *bool    `json:"monitorQueues,omitempty"`                // PMG only
	MonitorQuarantine            *bool    `json:"monitorQuarantine,omitempty"`            // PMG only
	MonitorDomainStats           *bool    `json:"monitorDomainStats,omitempty"`           // PMG only
	Enabled                      *bool    `json:"enabled,omitempty"`                      // Lifecycle toggle; nil on update preserves current
	// PVE only: per-cluster-member connection address overrides. Only the
	// members named here are touched; an empty ipOverride clears the override.
	ClusterEndpointTrustOverrides   []ClusterEndpointTrustOverrideRequest   `json:"clusterEndpointTrustOverrides,omitempty"`
	ClusterEndpointOverrides        []ClusterEndpointOverrideRequest        `json:"clusterEndpointOverrides,omitempty"`
	ClusterNodeDisplayNameOverrides []ClusterNodeDisplayNameOverrideRequest `json:"clusterNodeDisplayNameOverrides,omitempty"`
	guestURLSet                     bool                                    `json:"-"`
	fingerprintSet                  bool                                    `json:"-"`
}

// ClusterEndpointOverrideRequest sets or clears the connection address Pulse
// uses for one cluster member (ClusterEndpoint.IPOverride). Host and IP are
// rebuilt from cluster status on every re-discovery, so IPOverride is the
// only durable user-editable endpoint field.
type ClusterEndpointOverrideRequest struct {
	NodeName   string `json:"nodeName"`
	IPOverride string `json:"ipOverride"`
}

// ClusterNodeDisplayNameOverrideRequest sets or clears a presentation-only
// name using the immutable, connection-scoped node identity.
type ClusterNodeDisplayNameOverrideRequest struct {
	NodeIdentity string `json:"nodeIdentity"`
	DisplayName  string `json:"displayName"`
}

func (r *NodeConfigRequest) UnmarshalJSON(data []byte) error {
	type nodeConfigRequestJSON NodeConfigRequest

	var decoded nodeConfigRequestJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*r = NodeConfigRequest(decoded)
	_, r.guestURLSet = raw["guestURL"]
	_, r.fingerprintSet = raw["fingerprint"]
	return nil
}

func (r *NodeConfigRequest) normalizeTokenAliases() {
	r.TokenName = strings.TrimSpace(r.TokenName)
	r.TokenValue = strings.TrimSpace(r.TokenValue)

	if r.TokenName == "" {
		r.TokenName = strings.TrimSpace(r.TokenID)
	}
	if r.TokenValue == "" {
		r.TokenValue = strings.TrimSpace(r.TokenSecret)
	}
	if isRedactedCredentialPlaceholder(r.TokenValue) {
		r.TokenValue = ""
	}
	if isRedactedCredentialPlaceholder(r.Password) {
		r.Password = ""
	}
}

func (r *NodeConfigRequest) NormalizeTokenAliases() { r.normalizeTokenAliases() }

func (r NodeConfigRequest) hasGuestURLField() bool {
	return r.guestURLSet
}

func (r NodeConfigRequest) HasGuestURLField() bool { return r.hasGuestURLField() }

func (r NodeConfigRequest) hasFingerprintField() bool {
	return r.fingerprintSet
}

func (r NodeConfigRequest) HasFingerprintField() bool { return r.hasFingerprintField() }

func isRedactedCredentialPlaceholder(value string) bool {
	trimmed := strings.TrimSpace(value)
	switch strings.ToLower(trimmed) {
	case "[redacted]", "redacted", "********":
		return true
	}
	if len([]rune(trimmed)) < 6 {
		return false
	}
	for _, r := range trimmed {
		switch r {
		case '*', '•', '●':
			continue
		default:
			return false
		}
	}
	return true
}

// NodeResponse represents a node in API responses
type NodeResponse struct {
	ID                           string                    `json:"id"`
	Type                         string                    `json:"type"`
	Name                         string                    `json:"name"`
	Host                         string                    `json:"host"`
	GuestURL                     string                    `json:"guestURL,omitempty"`
	User                         string                    `json:"user,omitempty"`
	HasPassword                  bool                      `json:"hasPassword"`
	TokenName                    string                    `json:"tokenName,omitempty"`
	HasToken                     bool                      `json:"hasToken"`
	Fingerprint                  string                    `json:"fingerprint,omitempty"`
	VerifySSL                    bool                      `json:"verifySSL"`
	MonitorVMs                   bool                      `json:"monitorVMs"`
	MonitorContainers            bool                      `json:"monitorContainers"`
	MonitorStorage               bool                      `json:"monitorStorage"`
	MonitorBackups               bool                      `json:"monitorBackups"`
	MonitorPhysicalDisks         *bool                     `json:"monitorPhysicalDisks,omitempty"`
	PhysicalDiskPollingMinutes   int                       `json:"physicalDiskPollingMinutes,omitempty"`
	TemperatureMonitoringEnabled *bool                     `json:"temperatureMonitoringEnabled,omitempty"`
	MonitorDatastores            bool                      `json:"monitorDatastores,omitempty"`
	MonitorSyncJobs              bool                      `json:"monitorSyncJobs,omitempty"`
	MonitorVerifyJobs            bool                      `json:"monitorVerifyJobs,omitempty"`
	MonitorPruneJobs             bool                      `json:"monitorPruneJobs,omitempty"`
	MonitorGarbageJobs           bool                      `json:"monitorGarbageJobs,omitempty"`
	ExcludeDatastores            []string                  `json:"excludeDatastores"` // PBS only
	MonitorMailStats             bool                      `json:"monitorMailStats"`
	MonitorQueues                bool                      `json:"monitorQueues"`
	MonitorQuarantine            bool                      `json:"monitorQuarantine"`
	MonitorDomainStats           bool                      `json:"monitorDomainStats"`
	Enabled                      bool                      `json:"enabled"` // Lifecycle; false = paused
	Status                       string                    `json:"status"`  // "connected", "disconnected", "error"
	IsCluster                    bool                      `json:"isCluster,omitempty"`
	ClusterName                  string                    `json:"clusterName,omitempty"`
	ClusterEndpoints             []ClusterEndpointResponse `json:"clusterEndpoints"`
	Source                       string                    `json:"source,omitempty"` // "agent" or "script" - how this node was registered
}

func EmptyNodeResponse() NodeResponse {
	return NodeResponse{}.NormalizeCollections()
}

func (r NodeResponse) NormalizeCollections() NodeResponse {
	if r.ExcludeDatastores == nil {
		r.ExcludeDatastores = []string{}
	}
	if r.ClusterEndpoints == nil {
		r.ClusterEndpoints = []ClusterEndpointResponse{}
	}
	return r
}

// ClusterEndpointResponse is the v6 API shape for cluster endpoint payloads.
// It keeps config storage structs internal while returning canonical JSON keys.
type ClusterEndpointResponse struct {
	NodeID         string     `json:"nodeId,omitempty"`
	NodeIdentity   string     `json:"nodeIdentity"`
	NativeNodeID   int        `json:"nativeNodeId,omitempty"`
	NodeName       string     `json:"nodeName,omitempty"`
	NativeName     string     `json:"nativeName,omitempty"`
	NativeAliases  []string   `json:"nativeAliases,omitempty"`
	DisplayName    string     `json:"displayName,omitempty"`
	Host           string     `json:"host,omitempty"`
	GuestURL       string     `json:"guestURL,omitempty"`
	IP             string     `json:"ip,omitempty"`
	IPOverride     string     `json:"ipOverride,omitempty"`
	TLSEndpoint    string     `json:"tlsEndpoint,omitempty"`
	UsesPrimaryTLS bool       `json:"usesPrimaryTLS,omitempty"`
	TLSFirstUse    bool       `json:"tlsFirstUse,omitempty"`
	Fingerprint    string     `json:"fingerprint,omitempty"`
	Online         bool       `json:"online"`
	LastSeen       time.Time  `json:"lastSeen,omitempty"`
	PulseReachable *bool      `json:"pulseReachable,omitempty"`
	LastPulseCheck *time.Time `json:"lastPulseCheck,omitempty"`
	PulseError     string     `json:"pulseError,omitempty"`
}

func toClusterEndpointResponse(endpoint config.ClusterEndpoint, identitySets ...[]config.PVEClusterNodeIdentity) ClusterEndpointResponse {
	displayName := ""
	nativeName := endpoint.NodeName
	var nativeAliases []string
	for _, identities := range identitySets {
		for _, identity := range identities {
			if identity.ID != endpoint.NodeIdentity {
				continue
			}
			displayName = identity.DisplayName
			nativeAliases = append([]string(nil), identity.NativeAliases...)
			if identity.NativeName != "" {
				nativeName = identity.NativeName
			}
			break
		}
	}
	return ClusterEndpointResponse{
		NodeID:         endpoint.NodeID,
		NodeIdentity:   endpoint.NodeIdentity,
		NativeNodeID:   endpoint.NativeNodeID,
		NodeName:       endpoint.NodeName,
		NativeName:     nativeName,
		NativeAliases:  nativeAliases,
		DisplayName:    displayName,
		Host:           endpoint.Host,
		GuestURL:       endpoint.GuestURL,
		IP:             endpoint.IP,
		IPOverride:     endpoint.IPOverride,
		Fingerprint:    endpoint.Fingerprint,
		Online:         endpoint.Online,
		LastSeen:       endpoint.LastSeen,
		PulseReachable: endpoint.PulseReachable,
		LastPulseCheck: endpoint.LastPulseCheck,
		PulseError:     endpoint.PulseError,
	}
}

func toClusterEndpointResponses(endpoints []config.ClusterEndpoint, identitySets ...[]config.PVEClusterNodeIdentity) []ClusterEndpointResponse {
	if len(endpoints) == 0 {
		return []ClusterEndpointResponse{}
	}

	out := make([]ClusterEndpointResponse, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, toClusterEndpointResponse(endpoint, identitySets...))
	}
	return out
}

func isContainerSSHRestricted() bool {
	isContainer := os.Getenv("PULSE_DOCKER") == "true" || system.InContainer()
	if !isContainer {
		return false
	}
	return strings.ToLower(strings.TrimSpace(os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH"))) != "true"
}

// deriveSchemeAndPort infers the scheme (without ://) and port from a base host URL.
// Defaults align with Proxmox expectations when details are omitted.
func deriveSchemeAndPort(baseHost string) (scheme string, port string) {
	scheme = "https"
	port = "8006"

	baseHost = strings.TrimSpace(baseHost)
	if baseHost == "" {
		return scheme, port
	}

	candidate := baseHost
	if !strings.Contains(candidate, "://") {
		candidate = "https://" + candidate
	}

	parsed, err := url.Parse(candidate)
	if err != nil {
		return scheme, port
	}

	if parsed.Scheme != "" {
		scheme = parsed.Scheme
	}

	if parsed.Port() != "" {
		port = parsed.Port()
	}

	return scheme, port
}

// ensureHostHasPort guarantees that a host string contains an explicit port.
func ensureHostHasPort(host, port string) string {
	host = strings.TrimSpace(host)
	if host == "" || port == "" {
		return host
	}

	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}

	if parsed, err := url.Parse(host); err == nil && parsed.Host != "" {
		if parsed.Port() != "" {
			return parsed.Host
		}
		host = parsed.Host
	}

	trimmed := strings.TrimPrefix(host, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")

	return net.JoinHostPort(trimmed, port)
}

// isNodeValidationTLSError reports whether a validation error came from TLS
// certificate or fingerprint verification rather than the Proxmox API itself.
func isNodeValidationTLSError(errStr string) bool {
	return strings.Contains(errStr, "fingerprint") || strings.Contains(errStr, "x509") || strings.Contains(errStr, "certificate")
}

// isNodeValidationFingerprintMismatch reports whether validation failed the
// local certificate-pin check. Only that typed failure can admit a member on
// first use; error text that merely mentions certificates is not trust evidence.
func isNodeValidationFingerprintMismatch(err error) bool {
	var mismatch *tlsutil.FingerprintMismatchError
	return errors.As(err, &mismatch)
}

// clusterEndpointsWithSavedTrust returns a shallow copy of instance's cluster
// endpoints with explicit endpoint-level TLS overrides materialized into each
// matching member's Fingerprint field so cluster discovery validates members
// against the same pin polling enforces.
func clusterEndpointsWithSavedTrust(instance config.PVEInstance, discoveryCfg config.DiscoveryConfig) []config.ClusterEndpoint {
	if len(instance.ClusterEndpoints) == 0 {
		return nil
	}
	endpoints := make([]config.ClusterEndpoint, len(instance.ClusterEndpoints))
	copy(endpoints, instance.ClusterEndpoints)
	if len(instance.TLSFingerprintOverrides) == 0 {
		return endpoints
	}
	scheme, defaultPort := deriveSchemeAndPort(instance.Host)
	for i := range endpoints {
		ep := &endpoints[i]
		candidates := make([]string, 0, 4)
		for _, raw := range []string{ep.IP, ep.NodeName, ep.Host} {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			target := raw
			if !strings.HasPrefix(target, "http") {
				target = fmt.Sprintf("%s://%s", scheme, ensureHostHasPort(target, defaultPort))
			}
			candidates = append(candidates, target)
		}
		if runtimeURL := monitoring.PVEClusterEndpointURL(*ep, instance.VerifySSL, discoveryCfg); runtimeURL != "" {
			candidates = append(candidates, runtimeURL)
		}
		for _, candidate := range candidates {
			if overridePin, known, err := proxmox.ResolveEndpointFingerprint(candidate, instance.TLSFingerprintOverrides); err == nil && known && overridePin != "" {
				ep.Fingerprint = overridePin
				break
			}
		}
	}
	return endpoints
}

// isNodeValidationAuthError reports whether the node's API answered but denied
// access — proof enough that a real Proxmox endpoint is listening.
func isNodeValidationAuthError(errStr string) bool {
	return strings.Contains(errStr, "401") || strings.Contains(errStr, "403") || strings.Contains(errStr, "permission")
}

// nodeValidationFailureReason maps a validation error to a short operator-facing
// category so cluster discovery can distinguish "could not resolve or connect"
// from "TLS rejected" without leaking raw error internals.
func nodeValidationFailureReason(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()
	switch {
	case isNodeValidationTLSError(errStr):
		return "TLS certificate validation failed"
	case strings.Contains(errStr, "no such host"):
		return "hostname did not resolve from the Pulse server"
	case strings.Contains(errStr, "connection refused"):
		return "connection refused"
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return "connection timed out"
	default:
		return "not reachable"
	}
}

// validateNodeAPI tests if a cluster node has a working Proxmox API
// This helps filter out qdevice VMs and other non-Proxmox participants.
// savedFingerprint is the pin already saved for this member, empty when none.
// Returns (isValid, fingerprint, failureReason) - fingerprint is captured for
// TOFU (Trust On First Use); failureReason is empty when the node validates.
func validateNodeAPI(ctx context.Context, clusterNode proxmox.ClusterStatus, baseConfig proxmox.ClientConfig, savedFingerprint string) (bool, string, string) {
	// Determine the host to test - prefer IP if available, otherwise use node name
	testHost := clusterNode.IP
	if testHost == "" {
		testHost = clusterNode.Name
	}

	// Skip empty hostnames (shouldn't happen but be safe)
	if testHost == "" {
		return false, "", "no hostname or IP reported for this cluster member"
	}

	scheme, defaultPort := deriveSchemeAndPort(baseConfig.Host)

	// Create a test configuration for this specific node
	testConfig := baseConfig
	testConfig.Host = testHost
	if !strings.HasPrefix(testConfig.Host, "http") {
		hostWithPort := ensureHostHasPort(testConfig.Host, defaultPort)
		testConfig.Host = fmt.Sprintf("%s://%s", scheme, hostWithPort)
	}

	// Use a very short timeout for validation - we just need to know if the API exists
	testConfig.Timeout = 2 * time.Second

	// Validation sends the cluster credential to this member, so it must not
	// trust the member more loosely than polling will. A saved member pin is
	// the only certificate accepted and is never replaced. Only a member
	// without one may be trusted on first use, and only under a pinned primary
	// (#1664) whose own endpoint this is not: CA validation and the primary's
	// pin are never relaxed to whatever certificate the peer presents.
	firstUse := false
	if savedFingerprint != "" {
		testConfig.Fingerprint = savedFingerprint
	} else if baseConfig.Fingerprint != "" {
		member, memberValid := proxmox.ClusterEndpointIdentity(testConfig.Host)
		primary, primaryValid := proxmox.ClusterEndpointIdentity(baseConfig.Host)
		firstUse = memberValid && primaryValid && member != primary
	}

	log.Debug().
		Str("node", clusterNode.Name).
		Str("test_host", testConfig.Host).
		Msg("Validating Proxmox API for cluster node")

	// Capture the fingerprint for TOFU (Trust On First Use). A pin already
	// established for this endpoint is kept instead, and under CA validation
	// the capture is verified as well, so that a certificate the configured
	// policy would refuse can never be saved as this member's pin.
	var capturedFingerprint string
	if !firstUse {
		capturedFingerprint = testConfig.Fingerprint
	}
	if capturedFingerprint == "" {
		captureFingerprint := tlsutil.FetchFingerprintContext
		if testConfig.VerifySSL && !firstUse {
			captureFingerprint = tlsutil.FetchVerifiedFingerprintContext
		}
		fp, err := captureFingerprint(ctx, testConfig.Host)
		if err != nil {
			log.Debug().
				Str("node", clusterNode.Name).
				Err(err).
				Msg("Could not fetch TLS fingerprint for cluster node")
		} else {
			capturedFingerprint = fp
			log.Debug().
				Str("node", clusterNode.Name).
				Str("fingerprint", fp[:16]+"...").
				Msg("Captured TLS fingerprint for cluster node")
		}
	}

	// Try to create a client and make a simple API call
	testClient, err := proxmox.NewClientWithContext(ctx, testConfig)
	if err != nil {
		// Many clusters use unique certificates per node. If the primary node
		// was configured with fingerprint pinning, connecting to peers with the
		// same fingerprint will fail. Password authentication logs in here, so
		// retry a first-use member against its own captured certificate only;
		// every other failure (like auth, CA validation or a saved member pin)
		// stays a hard failure and the password is not sent again.
		if firstUse && capturedFingerprint != "" && isNodeValidationFingerprintMismatch(err) {
			log.Debug().
				Str("node", clusterNode.Name).
				Msg("Retrying cluster node validation with the member's own certificate")
			testConfig.Fingerprint = capturedFingerprint
			testClient, err = proxmox.NewClientWithContext(ctx, testConfig)
		}
		if err != nil {
			log.Debug().
				Str("node", clusterNode.Name).
				Err(err).
				Msg("Failed to create test client for cluster node")
			return false, "", nodeValidationFailureReason(err)
		}
	}
	defer testClient.CloseIdleConnections()

	// Test with a simple API call that all Proxmox nodes should support
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Try to get the node version - this is a very lightweight API call
	_, err = testClient.GetNodes(readCtx)
	if err != nil {
		errMsg := err.Error()
		// If we reached the API but were denied (common when per-node permissions
		// differ), treat it as valid. We only want to filter out hosts that aren't
		// actually Proxmox endpoints. A failed TLS check never reached the API,
		// whatever status-like digits its fingerprints or addresses contain.
		if isNodeValidationAuthError(errMsg) && !isNodeValidationTLSError(errMsg) {
			log.Debug().
				Str("node", clusterNode.Name).
				Err(err).
				Msg("Cluster node API responded but denied access; accepting for discovery")
			return true, capturedFingerprint, ""
		}

		// A pinned-fingerprint mismatch surfaces here, not from NewClient: the
		// verifier runs during the TLS handshake of the first request. Cluster
		// members normally carry their own per-node certificates, so retry a
		// first-use member with its own captured fingerprint before rejecting
		// it. A failed CA validation or saved member pin is never retried: that
		// would hand the credential to the peer that just failed the check.
		if firstUse && capturedFingerprint != "" && isNodeValidationFingerprintMismatch(err) {
			retryConfig := testConfig
			retryConfig.Fingerprint = capturedFingerprint
			if retryNodeValidation(ctx, retryConfig) {
				log.Debug().
					Str("node", clusterNode.Name).
					Msg("Cluster node validated with its own certificate after pinned-fingerprint mismatch")
				return true, capturedFingerprint, ""
			}
		}

		log.Debug().
			Str("node", clusterNode.Name).
			Err(err).
			Msg("Node failed Proxmox API validation - likely not a Proxmox node")
		return false, "", nodeValidationFailureReason(err)
	}

	log.Debug().
		Str("node", clusterNode.Name).
		Msg("Node passed Proxmox API validation")

	return true, capturedFingerprint, ""
}

// retryNodeValidation re-runs the lightweight validation call with an adjusted
// TLS configuration. Auth denials still count as success (see validateNodeAPI).
func retryNodeValidation(ctx context.Context, cfg proxmox.ClientConfig) bool {
	retryClient, err := proxmox.NewClientWithContext(ctx, cfg)
	if err != nil {
		return false
	}
	defer retryClient.CloseIdleConnections()
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := retryClient.GetNodes(readCtx); err != nil {
		return isNodeValidationAuthError(err.Error()) && !isNodeValidationTLSError(err.Error())
	}
	return true
}

// findExistingGuestURL looks up the GuestURL for a node from existing endpoints
func findExistingGuestURL(nodeName string, existingEndpoints []config.ClusterEndpoint) string {
	for _, ep := range existingEndpoints {
		if ep.NodeName == nodeName {
			return ep.GuestURL
		}
	}
	return ""
}

// findExistingIPOverride looks up the IPOverride for a node from existing endpoints
func findExistingIPOverride(nodeName string, existingEndpoints []config.ClusterEndpoint) string {
	for _, ep := range existingEndpoints {
		if ep.NodeName == nodeName {
			return ep.IPOverride
		}
	}
	return ""
}

func findExistingClusterEndpoint(nodeName string, existingEndpoints []config.ClusterEndpoint) (config.ClusterEndpoint, bool) {
	for _, endpoint := range existingEndpoints {
		if strings.EqualFold(strings.TrimSpace(endpoint.NodeName), strings.TrimSpace(nodeName)) {
			return endpoint, true
		}
	}
	return config.ClusterEndpoint{}, false
}

func FindExistingClusterEndpoint(nodeName string, existingEndpoints []config.ClusterEndpoint) (config.ClusterEndpoint, bool) {
	return findExistingClusterEndpoint(nodeName, existingEndpoints)
}

func retainUnreconciledClusterEndpoints(discovered, existing []config.ClusterEndpoint) []config.ClusterEndpoint {
	seen := make(map[string]struct{}, len(discovered))
	for _, endpoint := range discovered {
		seen[strings.ToLower(strings.TrimSpace(endpoint.NodeName))] = struct{}{}
	}
	for _, endpoint := range existing {
		key := strings.ToLower(strings.TrimSpace(endpoint.NodeName))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		discovered = append(discovered, endpoint)
		seen[key] = struct{}{}
	}
	return discovered
}

// extractIPFromHost extracts the IP address from a host URL.
// For example, "https://198.51.100.5:8006" returns 198.51.100.5 as net.IP.
func extractIPFromHost(host string) net.IP {
	// Parse the URL to get the hostname/IP
	parsed, err := url.Parse(host)
	if err != nil {
		return nil
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		hostname = host
	}

	// Try to parse as IP
	ip := net.ParseIP(hostname)
	if ip == nil {
		// Try resolving hostname
		ips, err := net.LookupIP(hostname)
		if err != nil || len(ips) == 0 {
			return nil
		}
		ip = ips[0]
	}

	return ip
}

// ipsOnSameNetwork checks if two IPs appear to be on the same network.
// It tries progressively larger subnets (/24, /20, /16 for IPv4) to handle
// various network topologies without requiring explicit subnet configuration.
func ipsOnSameNetwork(ip1, ip2 net.IP) bool {
	if ip1 == nil || ip2 == nil {
		return false
	}

	// Normalize to IPv4 if possible
	ip1v4 := ip1.To4()
	ip2v4 := ip2.To4()

	if ip1v4 != nil && ip2v4 != nil {
		// Try common IPv4 subnet sizes: /24 (most common), /20, /16
		// This handles everything from small home networks to large enterprise networks
		for _, bits := range []int{24, 20, 16} {
			mask := net.CIDRMask(bits, 32)
			if ip1v4.Mask(mask).Equal(ip2v4.Mask(mask)) {
				return true
			}
		}
		return false
	}

	// IPv6: try /64, /48
	ip1v6 := ip1.To16()
	ip2v6 := ip2.To16()
	if ip1v6 != nil && ip2v6 != nil {
		for _, bits := range []int{64, 48} {
			mask := net.CIDRMask(bits, 128)
			if ip1v6.Mask(mask).Equal(ip2v6.Mask(mask)) {
				return true
			}
		}
	}

	return false
}

// findPreferredIP looks through a list of node network interfaces and returns
// an IP that appears to be on the same network as the reference IP.
// Returns empty string if no match found.
func findPreferredIP(interfaces []proxmox.NodeNetworkInterface, referenceIP net.IP) string {
	if referenceIP == nil {
		return ""
	}

	for _, iface := range interfaces {
		// Skip inactive interfaces
		if iface.Active != 1 {
			continue
		}

		// Check IPv4 address
		if iface.Address != "" {
			ip := net.ParseIP(iface.Address)
			if ip != nil && ipsOnSameNetwork(ip, referenceIP) {
				return iface.Address
			}
		}
	}
	return ""
}

var (
	detectPVEClusterContext      = defaultDetectPVECluster
	validatePVEClusterNode       = validateNodeAPI
	fetchTLSFingerprint          = tlsutil.FetchFingerprint
	verifyAutoRegisterCredential = defaultVerifyAutoRegisterCredential
)

func ConfigureAutoRegisterNetworkDependencies(
	detect func(proxmox.ClientConfig, string, []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint),
	fingerprint func(string) (string, error),
) func() {
	previousDetect := detectPVEClusterContext
	previousFingerprint := fetchTLSFingerprint
	if detect != nil {
		detectPVEClusterContext = func(_ context.Context, clientConfig proxmox.ClientConfig, nodeName string, endpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
			return detect(clientConfig, nodeName, endpoints)
		}
	}
	if fingerprint != nil {
		fetchTLSFingerprint = fingerprint
	}
	return func() {
		detectPVEClusterContext = previousDetect
		fetchTLSFingerprint = previousFingerprint
	}
}

// ConfigureAutoRegisterCredentialVerifier overrides the live credential probe
// used by agent-token auto-register updates and returns a restore callback.
func ConfigureAutoRegisterCredentialVerifier(
	credential func(context.Context, string, proxmox.ClientConfig) error,
) func() {
	previousCredential := verifyAutoRegisterCredential
	if credential != nil {
		verifyAutoRegisterCredential = credential
	}
	return func() {
		verifyAutoRegisterCredential = previousCredential
	}
}

// detectPVECluster checks if a PVE node is part of a cluster and returns cluster information
// If existingEndpoints is provided, GuestURL values will be preserved for matching nodes
func detectPVECluster(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
	return detectPVEClusterContext(context.Background(), clientConfig, nodeName, existingEndpoints)
}

func defaultDetectPVECluster(ctx context.Context, clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
	return detectPVEClusterWithValidator(ctx, clientConfig, nodeName, existingEndpoints, validatePVEClusterNode)
}

func DetectPVEClusterWithValidator(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint, validator func(proxmox.ClusterStatus, proxmox.ClientConfig, string) (bool, string, string)) (isCluster bool, clusterName string, clusterEndpoints []config.ClusterEndpoint) {
	return detectPVEClusterWithValidator(context.Background(), clientConfig, nodeName, existingEndpoints,
		func(_ context.Context, node proxmox.ClusterStatus, cfg proxmox.ClientConfig, savedFingerprint string) (bool, string, string) {
			return validator(node, cfg, savedFingerprint)
		})
}

func detectPVEClusterWithValidator(ctx context.Context, clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint, validator func(context.Context, proxmox.ClusterStatus, proxmox.ClientConfig, string) (bool, string, string)) (isCluster bool, clusterName string, clusterEndpoints []config.ClusterEndpoint) {
	tempClient, err := proxmox.NewClientWithContext(ctx, clientConfig)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create client for cluster detection")
		return false, "", nil
	}
	defer tempClient.CloseIdleConnections()

	// Try to get cluster status with retries to handle API permission propagation delays
	// This addresses issue #437 where cluster detection fails on first attempt
	var clusterStatus []proxmox.ClusterStatus
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Wait a bit for permissions to propagate
			timer := time.NewTimer(time.Duration(attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false, "", nil
			case <-timer.C:
			}
			log.Debug().Int("attempt", attempt+1).Msg("Retrying cluster detection")
		}

		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

		// Get full cluster status to find the actual cluster name
		// Note: This can cause certificate lookup errors on standalone nodes, but it's only done once during configuration
		clusterStatus, lastErr = tempClient.GetClusterStatus(readCtx)
		cancel()
		if ctx.Err() != nil {
			return false, "", nil
		}
		if lastErr == nil {
			// Success!
			break
		}

		// Check if this is definitely not a cluster (e.g., 501 not implemented)
		lastErrStr := lastErr.Error()
		if strings.Contains(lastErrStr, "501") || strings.Contains(lastErrStr, "not implemented") {
			// This is a standalone node, no need to retry
			log.Debug().Err(lastErr).Msg("Standalone node detected - cluster API not available")
			return false, "", nil
		}
	}

	if lastErr != nil {
		// This is expected for standalone nodes - they will return an error when accessing cluster endpoints
		log.Debug().Err(lastErr).Msg("Could not get cluster status after retries - likely a standalone node")
		return false, "", nil
	}

	// Find the cluster name and collect nodes
	var clusterNodes []proxmox.ClusterStatus
	for _, status := range clusterStatus {
		if status.Type == "cluster" {
			// This is the actual cluster name
			clusterName = status.Name
			log.Info().Str("cluster_name", clusterName).Msg("Found cluster name")
		} else if status.Type == "node" {
			clusterNodes = append(clusterNodes, status)
		}
	}

	log.Info().Int("cluster_nodes", len(clusterNodes)).Msg("Got cluster nodes")

	if len(clusterNodes) > 1 {
		isCluster = true
		log.Info().
			Str("cluster", clusterName).
			Str("node", nodeName).
			Int("nodes", len(clusterNodes)).
			Msg("Detected Proxmox cluster")
		scheme, defaultPort := deriveSchemeAndPort(clientConfig.Host)
		schemePrefix := scheme + "://"

		// Extract the connection IP to use as reference for preferred network
		// This allows us to prefer management network IPs over internal cluster IPs
		connectionIP := extractIPFromHost(clientConfig.Host)
		if connectionIP != nil {
			log.Debug().
				Str("connection_ip", connectionIP.String()).
				Str("from_host", clientConfig.Host).
				Msg("Extracted connection IP for network preference")
		}

		validatedCount := 0
		for _, clusterNode := range clusterNodes {
			// Reachability enriches a member endpoint but never defines cluster
			// membership. Powered-off nodes remain canonical members even
			// though their individual API cannot answer during discovery.
			existingEndpoint, hadExistingEndpoint := findExistingClusterEndpoint(clusterNode.Name, existingEndpoints)
			isValid, nodeFingerprint, failureReason := validator(ctx, clusterNode, clientConfig, existingEndpoint.Fingerprint)
			if ctx.Err() != nil {
				return false, "", nil
			}
			if !isValid {
				log.Info().
					Str("node", clusterNode.Name).
					Str("ip", clusterNode.IP).
					Msg("Cluster member API is unreachable - preserving membership with unavailable endpoint state")
			} else {
				validatedCount++
			}

			// Build the host URL with proper port
			// Store hostname in Host field (for TLS validation), IP separately
			now := time.Now()
			lastSeen := time.Time{}
			if hadExistingEndpoint {
				lastSeen = existingEndpoint.LastSeen
			}
			if isValid {
				lastSeen = now
			}
			// Discovery can establish a member's pin but never replaces a saved one.
			if hadExistingEndpoint && existingEndpoint.Fingerprint != "" {
				nodeFingerprint = existingEndpoint.Fingerprint
			}
			pulseReachable := isValid
			endpoint := config.ClusterEndpoint{
				NodeID:         clusterNode.ID,
				NativeNodeID:   clusterNode.Nodeid,
				NodeName:       clusterNode.Name,
				GuestURL:       findExistingGuestURL(clusterNode.Name, existingEndpoints),
				IPOverride:     findExistingIPOverride(clusterNode.Name, existingEndpoints), // Preserve user override
				Fingerprint:    nodeFingerprint,                                             // Store captured fingerprint for per-node TLS verification
				Online:         clusterNode.Online == 1,
				LastSeen:       lastSeen,
				PulseReachable: &pulseReachable,
				LastPulseCheck: &now,
			}
			if !isValid {
				endpoint.PulseError = "Proxmox API endpoint unavailable during cluster discovery"
				if failureReason != "" {
					endpoint.PulseError = fmt.Sprintf("Proxmox API endpoint unavailable during cluster discovery (%s)", failureReason)
				}
			}

			// Populate Host field with hostname (if available) for TLS certificate validation
			if clusterNode.Name != "" {
				nodeHost := ensureHostHasPort(clusterNode.Name, defaultPort)
				endpoint.Host = schemePrefix + nodeHost
			}

			// Populate IP field with cluster-reported IP (may be internal network)
			if clusterNode.IP != "" {
				endpoint.IP = clusterNode.IP
			}

			// Try to find a better IP on the same network as initial connection (management network)
			// Only do this if no manual override is set
			if endpoint.IPOverride == "" && connectionIP != nil && clusterNode.IP != "" {
				// Check if cluster-reported IP is already on the same network as our connection
				clusterIP := net.ParseIP(clusterNode.IP)
				if clusterIP != nil && !ipsOnSameNetwork(clusterIP, connectionIP) {
					// Cluster IP is on a different network, try to find one on the same network
					readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					nodeInterfaces, err := tempClient.GetNodeNetworkInterfaces(readCtx, clusterNode.Name)
					cancel()
					if ctx.Err() != nil {
						return false, "", nil
					}

					if err == nil {
						preferredIP := findPreferredIP(nodeInterfaces, connectionIP)
						if preferredIP != "" && preferredIP != clusterNode.IP {
							log.Info().
								Str("node", clusterNode.Name).
								Str("cluster_ip", clusterNode.IP).
								Str("preferred_ip", preferredIP).
								Str("connection_ip", connectionIP.String()).
								Msg("Found preferred management IP for cluster node")
							endpoint.IPOverride = preferredIP
						}
					} else {
						log.Debug().
							Err(err).
							Str("node", clusterNode.Name).
							Msg("Could not query node network interfaces for network preference")
					}
				}
			}

			clusterEndpoints = append(clusterEndpoints, endpoint)
		}

		// Configuration discovery can add or enrich membership, but it has no
		// repeated-absence reconciliation state and therefore cannot safely
		// retire an existing member. Monitoring owns confirmed removal.
		clusterEndpoints = retainUnreconciledClusterEndpoints(clusterEndpoints, existingEndpoints)

		// Membership count and reachable API count are deliberately separate.
		log.Info().
			Str("cluster", clusterName).
			Int("total_discovered", len(clusterNodes)).
			Int("persisted_members", len(clusterEndpoints)).
			Int("reachable_api_endpoints", validatedCount).
			Msg("Cluster node validation complete")

		// Fallback if we couldn't get the cluster name
		if clusterName == "" {
			clusterName = "Unknown Cluster"
		}
	}

	return isCluster, clusterName, clusterEndpoints
}

// GetAllNodesForAPI returns all configured nodes for API responses
func (h *ConfigHandlers) GetAllNodesForAPI(ctx context.Context) []NodeResponse {
	nodes := []NodeResponse{}

	// Refresh cluster metadata if we previously failed to detect endpoints.
	// Detection waits on the monitored servers, so it works from copies and
	// the list is read again afterwards.
	config.Mu.RLock()
	detectable := make([]config.PVEInstance, len(h.getConfig(ctx).PVEInstances))
	for i, instance := range h.getConfig(ctx).PVEInstances {
		detectable[i] = instance.DeepCopy()
	}
	config.Mu.RUnlock()
	for _, instance := range detectable {
		h.maybeRefreshClusterInfo(ctx, instance)
	}

	// Add PVE nodes
	config.Mu.RLock()
	pveSnapshot := make([]config.PVEInstance, len(h.getConfig(ctx).PVEInstances))
	for i, instance := range h.getConfig(ctx).PVEInstances {
		pveSnapshot[i] = instance.DeepCopy()
	}
	discoveryCfg := h.getConfig(ctx).Discovery
	config.Mu.RUnlock()
	for i, pve := range pveSnapshot {
		node := NodeResponse{
			ID:                           generateNodeID("pve", i),
			Type:                         "pve",
			Name:                         pve.Name,
			Host:                         pve.Host,
			GuestURL:                     pve.GuestURL,
			User:                         pve.User,
			HasPassword:                  pve.Password != "",
			TokenName:                    pve.TokenName,
			HasToken:                     pve.TokenValue != "",
			Fingerprint:                  pvePrimaryTLSFingerprint(pve),
			VerifySSL:                    pve.VerifySSL,
			MonitorVMs:                   pve.MonitorVMs,
			MonitorContainers:            pve.MonitorContainers,
			MonitorStorage:               pve.MonitorStorage,
			MonitorBackups:               pve.MonitorBackups,
			MonitorPhysicalDisks:         pve.MonitorPhysicalDisks,
			PhysicalDiskPollingMinutes:   pve.PhysicalDiskPollingMinutes,
			TemperatureMonitoringEnabled: pve.TemperatureMonitoringEnabled,
			Enabled:                      !pve.Disabled,
			Status:                       h.getNodeStatus(ctx, "pve", pve.Name),
			IsCluster:                    pve.IsCluster,
			ClusterName:                  pve.ClusterName,
			ClusterEndpoints:             pveClusterEndpointTLSResponses(pve, discoveryCfg),
			Source:                       pve.Source,
		}.NormalizeCollections()
		nodes = append(nodes, node)
	}

	// Add PBS nodes
	for i, pbs := range h.getConfig(ctx).PBSInstances {
		node := NodeResponse{
			ID:                           generateNodeID("pbs", i),
			Type:                         "pbs",
			Name:                         pbs.Name,
			Host:                         pbs.Host,
			GuestURL:                     pbs.GuestURL,
			User:                         pbs.User,
			HasPassword:                  pbs.Password != "",
			TokenName:                    pbs.TokenName,
			HasToken:                     pbs.TokenValue != "",
			Fingerprint:                  pbs.Fingerprint,
			VerifySSL:                    pbs.VerifySSL,
			TemperatureMonitoringEnabled: pbs.TemperatureMonitoringEnabled,
			MonitorDatastores:            pbs.MonitorDatastores,
			MonitorSyncJobs:              pbs.MonitorSyncJobs,
			MonitorVerifyJobs:            pbs.MonitorVerifyJobs,
			MonitorPruneJobs:             pbs.MonitorPruneJobs,
			MonitorGarbageJobs:           pbs.MonitorGarbageJobs,
			ExcludeDatastores:            pbs.ExcludeDatastores,
			Enabled:                      !pbs.Disabled,
			Status:                       h.getNodeStatus(ctx, "pbs", pbs.Name),
			Source:                       pbs.Source,
		}.NormalizeCollections()
		nodes = append(nodes, node)
	}

	// Add PMG nodes
	for i, pmgInst := range h.getConfig(ctx).PMGInstances {
		monitorMailStats := pmgInst.MailStatsEnabled()

		node := NodeResponse{
			ID:                           generateNodeID("pmg", i),
			Type:                         "pmg",
			Name:                         pmgInst.Name,
			Host:                         pmgInst.Host,
			GuestURL:                     pmgInst.GuestURL,
			User:                         pmgInst.User,
			HasPassword:                  pmgInst.Password != "",
			TokenName:                    pmgInst.TokenName,
			HasToken:                     pmgInst.TokenValue != "",
			Fingerprint:                  pmgInst.Fingerprint,
			VerifySSL:                    pmgInst.VerifySSL,
			TemperatureMonitoringEnabled: pmgInst.TemperatureMonitoringEnabled,
			MonitorMailStats:             monitorMailStats,
			MonitorQueues:                pmgInst.MonitorQueues,
			MonitorQuarantine:            pmgInst.MonitorQuarantine,
			MonitorDomainStats:           pmgInst.MonitorDomainStats,
			Enabled:                      !pmgInst.Disabled,
			Status:                       h.getNodeStatus(ctx, "pmg", pmgInst.Name),
		}.NormalizeCollections()
		nodes = append(nodes, node)
	}

	return nodes
}

// HandleGetNodes returns all configured nodes
func (h *ConfigHandlers) HandleGetNodes(w http.ResponseWriter, r *http.Request) {
	h.handleGetNodes(w, r)
}
func validateIPAddress(ip string) bool {
	// Parse as IP address
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Ensure it's IPv4 or IPv6
	return parsedIP.To4() != nil || parsedIP.To16() != nil
}

// validatePort validates if a port number is in valid range
func validatePort(portStr string) bool {
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false
	}
	return port > 0 && port <= 65535
}

// extractHostAndPort extracts the host and port from a URL or host:port string
func extractHostAndPort(hostStr string) (string, string, error) {
	// Remove protocol if present
	if strings.HasPrefix(hostStr, "http://") {
		hostStr = strings.TrimPrefix(hostStr, "http://")
	} else if strings.HasPrefix(hostStr, "https://") {
		hostStr = strings.TrimPrefix(hostStr, "https://")
	}

	// Remove trailing slash and path if present
	if idx := strings.Index(hostStr, "/"); idx != -1 {
		hostStr = hostStr[:idx]
	}

	// Check if it contains a port
	if strings.Contains(hostStr, ":") {
		host, port, err := net.SplitHostPort(hostStr)
		if err != nil {
			// Might be IPv6 without port
			if strings.Count(hostStr, ":") > 1 && !strings.Contains(hostStr, "[") {
				return hostStr, "", nil
			}
			return "", "", fmt.Errorf("invalid host:port format")
		}
		return host, port, nil
	}

	return hostStr, "", nil
}

func defaultPortForNodeType(nodeType string) string {
	switch nodeType {
	case "pve", "pmg":
		return "8006"
	case "pbs":
		return "8007"
	default:
		return ""
	}
}

// normalizeNodeHost ensures hosts always include a scheme and default port when one
// isn't provided. Defaults align with Proxmox APIs (PVE/PMG: 8006, PBS: 8007) while
// preserving any explicit scheme/port the user supplies.
func normalizeNodeHost(rawHost, nodeType string) (string, error) {
	host := strings.TrimSpace(rawHost)
	if host == "" {
		return "", fmt.Errorf("host is required")
	}

	scheme := "https"
	if strings.HasPrefix(host, "http://") {
		scheme = "http"
		host = strings.TrimPrefix(host, "http://")
	} else if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	}

	// Strip any path/query fragments before parsing
	if slash := strings.Index(host, "/"); slash != -1 {
		host = host[:slash]
	}

	hostWithoutBrackets := strings.Trim(host, "[]")
	if ip := net.ParseIP(hostWithoutBrackets); ip != nil && strings.Contains(hostWithoutBrackets, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}

	hostForParse := scheme + "://" + host
	parsed, err := url.Parse(hostForParse)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("invalid host format")
	}

	// Drop any path/query fragments to avoid persisting unsafe values
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	if parsed.Port() == "" {
		defaultPort := defaultPortForNodeType(nodeType)
		if defaultPort != "" {
			parsed.Host = net.JoinHostPort(parsed.Hostname(), defaultPort)
		}
	}

	return parsed.String(), nil
}

// extractHostIP extracts the IP address from a host URL if it's an IP-based URL.
// Returns empty string if the URL uses a hostname instead of an IP.
func extractHostIP(hostURL string) string {
	parsed, err := url.Parse(hostURL)
	if err != nil {
		return ""
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return ""
	}
	// Check if hostname is an IP address
	if ip := net.ParseIP(hostname); ip != nil {
		return ip.String()
	}
	return ""
}

// resolveHostnameToIP attempts to resolve a hostname URL to its first IP address.
// Returns empty string if resolution fails or times out.
func resolveHostnameToIP(hostURL string) string {
	parsed, err := url.Parse(hostURL)
	if err != nil {
		return ""
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return ""
	}

	// Don't try to resolve if it's already an IP
	if ip := net.ParseIP(hostname); ip != nil {
		return ip.String()
	}

	// Resolve with a short timeout to avoid blocking
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var resolver net.Resolver
	addrs, err := resolver.LookupHost(ctx, hostname)
	if err != nil || len(addrs) == 0 {
		log.Debug().
			Str("hostname", hostname).
			Err(err).
			Msg("Failed to resolve hostname for duplicate detection")
		return ""
	}

	return addrs[0]
}

// hostsShareResolvedIdentity reports whether two node hosts resolve to the
// same concrete IP identity, even when one side is a hostname URL and the
// other is stored as an IP URL.
func hostsShareResolvedIdentity(existingHost, candidateHost string) bool {
	if strings.TrimSpace(existingHost) == "" || strings.TrimSpace(candidateHost) == "" {
		return false
	}

	existingIP := extractHostIP(existingHost)
	if existingIP == "" {
		existingIP = resolveHostnameToIP(existingHost)
	}

	candidateIP := extractHostIP(candidateHost)
	if candidateIP == "" {
		candidateIP = resolveHostnameToIP(candidateHost)
	}

	if existingIP == "" || candidateIP == "" {
		return false
	}
	if existingIP == candidateIP {
		return true
	}

	existingParsed := net.ParseIP(existingIP)
	candidateParsed := net.ParseIP(candidateIP)
	return existingParsed != nil &&
		candidateParsed != nil &&
		existingParsed.IsLoopback() &&
		candidateParsed.IsLoopback()
}

func HostsShareResolvedIdentity(existingHost, candidateHost string) bool {
	return hostsShareResolvedIdentity(existingHost, candidateHost)
}

// disambiguateNodeName ensures a node name is unique by appending the host IP if needed.
// This handles cases where multiple Proxmox hosts have the same hostname (e.g., "px1" on different networks).
// Returns the original name if unique, or "name (ip)" if duplicates exist.
func (h *ConfigHandlers) disambiguateNodeName(ctx context.Context, name, host, nodeType string) string {
	if name == "" {
		return name
	}

	// Check if any existing node has the same name
	hasDuplicate := false
	if nodeType == "pve" {
		for _, node := range h.getConfig(ctx).PVEInstances {
			if strings.EqualFold(node.Name, name) && node.Host != host {
				hasDuplicate = true
				break
			}
		}
	} else if nodeType == "pbs" {
		for _, node := range h.getConfig(ctx).PBSInstances {
			if strings.EqualFold(node.Name, name) && node.Host != host {
				hasDuplicate = true
				break
			}
		}
	}

	if !hasDuplicate {
		return name
	}

	// Extract IP/hostname from host URL for disambiguation
	parsed, err := url.Parse(host)
	if err != nil || parsed.Host == "" {
		// Fallback: use a short hash of the host
		return fmt.Sprintf("%s (%s)", name, host[:min(15, len(host))])
	}

	hostname := parsed.Hostname()
	return fmt.Sprintf("%s (%s)", name, hostname)
}

// HandleAddNode adds a new node
func (h *ConfigHandlers) HandleAddNode(w http.ResponseWriter, r *http.Request) {
	h.handleAddNode(w, r)
}

// HandleTestConnection tests a node connection without saving
func (h *ConfigHandlers) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	h.handleTestConnection(w, r)
}

// HandleUpdateNode updates an existing node
func (h *ConfigHandlers) HandleUpdateNode(w http.ResponseWriter, r *http.Request) {
	h.handleUpdateNode(w, r)
}

// HandleDeleteNode deletes a node
func (h *ConfigHandlers) HandleDeleteNode(w http.ResponseWriter, r *http.Request) {
	h.handleDeleteNode(w, r)
}

// HandleRefreshClusterNodes re-detects cluster membership and updates endpoints
// This handles the case where nodes are added to a Proxmox cluster after initial configuration
func (h *ConfigHandlers) HandleRefreshClusterNodes(w http.ResponseWriter, r *http.Request) {
	h.handleRefreshClusterNodes(w, r)
}

// HandleTestNodeConfig tests a node connection from provided configuration
func (h *ConfigHandlers) HandleTestNodeConfig(w http.ResponseWriter, r *http.Request) {
	h.handleTestNodeConfig(w, r)
}

// HandleTestNode tests a node connection
func (h *ConfigHandlers) HandleTestNode(w http.ResponseWriter, r *http.Request) {
	h.handleTestNode(w, r)
}

func (h *ConfigHandlers) getNodeStatus(ctx context.Context, nodeType, nodeName string) string {
	if h.getMonitor(ctx) == nil {
		if h.isRecentlyAutoRegistered(nodeType, nodeName) {
			return "connected"
		}
		return "disconnected"
	}

	// Get connection statuses from monitor
	connectionStatus := h.getMonitor(ctx).GetConnectionStatuses()

	key := fmt.Sprintf("%s-%s", nodeType, nodeName)
	if connected, ok := connectionStatus[key]; ok {
		if connected {
			h.clearAutoRegistered(nodeType, nodeName)
			return "connected"
		}
		if h.isRecentlyAutoRegistered(nodeType, nodeName) {
			return "connected"
		}
		return "disconnected"
	}

	if h.isRecentlyAutoRegistered(nodeType, nodeName) {
		return "connected"
	}

	return "disconnected"
}

// HandleGetSystemSettings returns current system settings
func (h *ConfigHandlers) HandleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	h.handleGetSystemSettings(w, r)
}

// HandleVerifyTemperatureSSH tests SSH connectivity to nodes for temperature monitoring
func (h *ConfigHandlers) HandleVerifyTemperatureSSH(w http.ResponseWriter, r *http.Request) {
	h.handleVerifyTemperatureSSH(w, r)
}

// generateNodeID creates a unique ID for a node
func generateNodeID(nodeType string, index int) string {
	return fmt.Sprintf("%s-%d", nodeType, index)
}

// HandleExportConfig exports all configuration with encryption
func (h *ConfigHandlers) HandleExportConfig(w http.ResponseWriter, r *http.Request) {
	h.handleExportConfig(w, r)
}

// HandleImportConfig imports configuration from encrypted export
func (h *ConfigHandlers) HandleImportConfig(w http.ResponseWriter, r *http.Request) {
	h.handleImportConfig(w, r)
}

// HandleDiscoverServers handles network discovery of Proxmox/PBS servers
func (h *ConfigHandlers) HandleDiscoverServers(w http.ResponseWriter, r *http.Request) {
	h.handleDiscoverServers(w, r)
}

// HandleSetupScript serves the setup script for Proxmox/PBS nodes.
func (h *ConfigHandlers) HandleSetupScript(w http.ResponseWriter, r *http.Request) {
	h.handleSetupScript(w, r)
}

// HandleSetupScriptURL generates a one-time setup token and URL for the setup script.
func (h *ConfigHandlers) HandleSetupScriptURL(w http.ResponseWriter, r *http.Request) {
	h.handleSetupScriptURL(w, r)
}

// HandleGetMockMode returns the current mock mode state and configuration.
func (h *ConfigHandlers) HandleGetMockMode(w http.ResponseWriter, r *http.Request) {
	h.handleGetMockMode(w, r)
}

// HandleUpdateMockMode updates mock mode and optionally its configuration.
func (h *ConfigHandlers) HandleUpdateMockMode(w http.ResponseWriter, r *http.Request) {
	h.handleUpdateMockMode(w, r)
}

// HandleAutoRegister receives token details from the setup script and auto-configures the node.
func (h *ConfigHandlers) HandleAutoRegister(w http.ResponseWriter, r *http.Request) {
	h.handleAutoRegister(w, r)
}

// HandleAutoUnregister receives teardown requests from the setup script and removes matching nodes.
func (h *ConfigHandlers) HandleAutoUnregister(w http.ResponseWriter, r *http.Request) {
	h.handleAutoUnregister(w, r)
}

// HandleAgentInstallCommand generates an API token and install command for agent-based Proxmox setup.
func (h *ConfigHandlers) HandleAgentInstallCommand(w http.ResponseWriter, r *http.Request) {
	h.handleAgentInstallCommand(w, r)
}
