package api

import (
	"context"
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
	setupAuthTokenPattern = regexp.MustCompile(`^[A-Fa-f0-9]{32,128}$`)
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
	parsed.Fragment = ""
	return parsed.String(), nil
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

// SetupCode represents a one-time setup code for secure node registration
type SetupCode struct {
	ExpiresAt time.Time
	Used      bool
	NodeType  string // "pve" or "pbs"
	Host      string // The host URL for validation
	OrgID     string // Organization ID creating this code
}

// ConfigHandlers handles configuration-related API endpoints
type ConfigHandlers struct {
	stateMu       sync.RWMutex
	mtPersistence *config.MultiTenantPersistence
	mtMonitor     *monitoring.MultiTenantMonitor
	// Legacy fields - to be removed or used as fallback
	legacyConfig      *config.Config
	legacyPersistence *config.ConfigPersistence
	legacyMonitor     *monitoring.Monitor

	reloadFunc               func() error
	reloadSystemSettingsFunc func() // Function to reload cached system settings
	wsHub                    *websocket.Hub
	guestMetadataHandler     *GuestMetadataHandler
	setupCodes               map[string]*SetupCode // Map of code hash -> setup code details
	recentSetupTokens        map[string]time.Time  // Temporary map for recently used setup tokens (grace period)
	codeMutex                sync.RWMutex          // Mutex for thread-safe code access
	clusterDetectMutex       sync.Mutex
	lastClusterDetection     map[string]time.Time
	recentAutoRegistered     map[string]time.Time
	recentAutoRegMutex       sync.Mutex
}

// NewConfigHandlers creates a new ConfigHandlers instance
func NewConfigHandlers(mtp *config.MultiTenantPersistence, mtm *monitoring.MultiTenantMonitor, reloadFunc func() error, wsHub *websocket.Hub, guestMetadataHandler *GuestMetadataHandler, reloadSystemSettingsFunc func()) *ConfigHandlers {
	// Initialize with default (legacy) values if available, for backward compat during migration
	// Ideally we fetch them from mtp/mtm for "default" org.
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
		legacyConfig:             defaultConfig,
		legacyMonitor:            defaultMonitor,
		legacyPersistence:        defaultPersistence,
		reloadFunc:               reloadFunc,
		reloadSystemSettingsFunc: reloadSystemSettingsFunc,
		wsHub:                    wsHub,
		guestMetadataHandler:     guestMetadataHandler,
		setupCodes:               make(map[string]*SetupCode),
		recentSetupTokens:        make(map[string]time.Time),
		lastClusterDetection:     make(map[string]time.Time),
		recentAutoRegistered:     make(map[string]time.Time),
	}

	// Clean up expired codes periodically
	go h.cleanupExpiredCodes()

	return h
}

// SetMultiTenantMonitor updates the monitor reference used by the config handlers.
func (h *ConfigHandlers) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	var legacyMonitor *monitoring.Monitor
	var legacyConfig *config.Config
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			legacyMonitor = m
			if m != nil {
				legacyConfig = m.GetConfig()
			}
		}
	}

	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.mtMonitor = mtm
	if legacyMonitor != nil {
		h.legacyMonitor = legacyMonitor
		h.legacyConfig = legacyConfig
	}
}

// SetMonitor updates the monitor reference used by the config handlers (legacy support).
func (h *ConfigHandlers) SetMonitor(m *monitoring.Monitor) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.legacyMonitor = m
	if m != nil {
		h.legacyConfig = m.GetConfig()
	}
}

// SetConfig updates the configuration reference used by the handlers.
func (h *ConfigHandlers) SetConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}

	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.legacyConfig = cfg
}

// getContextState helper to retrieve tenant-specific state
func (h *ConfigHandlers) getContextState(ctx context.Context) (*config.Config, *config.ConfigPersistence, *monitoring.Monitor) {
	h.stateMu.RLock()
	mtMonitor := h.mtMonitor
	mtPersistence := h.mtPersistence
	legacyConfig := h.legacyConfig
	legacyPersistence := h.legacyPersistence
	legacyMonitor := h.legacyMonitor
	h.stateMu.RUnlock()

	orgID := "default"
	if ctx != nil {
		if id := GetOrgID(ctx); id != "" {
			orgID = id
		}
	}

	// Try to get from multi-tenant managers first
	if mtMonitor != nil {
		if m, err := mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			cfg := m.GetConfig()
			var p *config.ConfigPersistence
			if mtPersistence != nil {
				p, _ = mtPersistence.GetPersistence(orgID)
			}
			return cfg, p, m
		} else if err != nil {
			log.Warn().Str("orgID", orgID).Err(err).Msg("Falling back to legacy config - failed to get tenant monitor")
		}
	}

	// Fallback to legacy (should mostly happen for "default" or initialization)
	return legacyConfig, legacyPersistence, legacyMonitor
}

func (h *ConfigHandlers) getConfig(ctx context.Context) *config.Config {
	c, _, _ := h.getContextState(ctx)
	return c
}

func (h *ConfigHandlers) getPersistence(ctx context.Context) *config.ConfigPersistence {
	_, p, _ := h.getContextState(ctx)
	return p
}

func (h *ConfigHandlers) getMonitor(ctx context.Context) *monitoring.Monitor {
	_, _, m := h.getContextState(ctx)
	return m
}

// cleanupExpiredCodes removes expired or used setup codes periodically
func (h *ConfigHandlers) cleanupExpiredCodes() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		h.codeMutex.Lock()
		now := time.Now()
		for codeHash, code := range h.setupCodes {
			if now.After(code.ExpiresAt) || code.Used {
				delete(h.setupCodes, codeHash)
				log.Debug().Bool("was_used", code.Used).Msg("Cleaned up setup code")
			}
		}
		for tokenHash, expiresAt := range h.recentSetupTokens {
			if now.After(expiresAt) {
				delete(h.recentSetupTokens, tokenHash)
			}
		}
		h.codeMutex.Unlock()
	}
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

	if code, exists := h.setupCodes[tokenHash]; exists {
		if !code.Used && now.Before(code.ExpiresAt) {
			return true
		}
	}

	if expiresAt, ok := h.recentSetupTokens[tokenHash]; ok && now.Before(expiresAt) {
		return true
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

// sanitizeErrorMessage returns a safe error message for external responses
// It logs the detailed error internally while returning a generic message
func sanitizeErrorMessage(err error, operation string) string {
	// Log the detailed error internally
	log.Error().Err(err).Str("operation", operation).Msg("Operation failed")

	// Return generic messages based on operation type
	switch operation {
	case "create_client":
		return "Failed to initialize connection"
	case "connection":
		return "Connection failed. Please check your credentials and network settings"
	case "validation":
		return "Invalid configuration"
	default:
		return "Operation failed"
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

func (h *ConfigHandlers) maybeRefreshClusterInfo(ctx context.Context, instance *config.PVEInstance) {
	if instance == nil {
		return
	}

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

	clientConfig := config.CreateProxmoxConfig(instance)
	isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, instance.Name, instance.ClusterEndpoints)
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

	instance.IsCluster = true
	instance.ClusterName = clusterName
	instance.ClusterEndpoints = clusterEndpoints

	log.Info().
		Str("instance", instance.Name).
		Str("cluster", clusterName).
		Int("endpoints", len(clusterEndpoints)).
		Msg("Updated cluster metadata after validation retry")

	if h.getPersistence(ctx) != nil {
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
}

// NodeResponse represents a node in API responses
type NodeResponse struct {
	ID                           string                   `json:"id"`
	Type                         string                   `json:"type"`
	Name                         string                   `json:"name"`
	Host                         string                   `json:"host"`
	GuestURL                     string                   `json:"guestURL,omitempty"`
	User                         string                   `json:"user,omitempty"`
	HasPassword                  bool                     `json:"hasPassword"`
	TokenName                    string                   `json:"tokenName,omitempty"`
	HasToken                     bool                     `json:"hasToken"`
	Fingerprint                  string                   `json:"fingerprint,omitempty"`
	VerifySSL                    bool                     `json:"verifySSL"`
	MonitorVMs                   bool                     `json:"monitorVMs,omitempty"`
	MonitorContainers            bool                     `json:"monitorContainers,omitempty"`
	MonitorStorage               bool                     `json:"monitorStorage,omitempty"`
	MonitorBackups               bool                     `json:"monitorBackups,omitempty"`
	MonitorPhysicalDisks         *bool                    `json:"monitorPhysicalDisks,omitempty"`
	PhysicalDiskPollingMinutes   int                      `json:"physicalDiskPollingMinutes,omitempty"`
	TemperatureMonitoringEnabled *bool                    `json:"temperatureMonitoringEnabled,omitempty"`
	MonitorDatastores            bool                     `json:"monitorDatastores,omitempty"`
	MonitorSyncJobs              bool                     `json:"monitorSyncJobs,omitempty"`
	MonitorVerifyJobs            bool                     `json:"monitorVerifyJobs,omitempty"`
	MonitorPruneJobs             bool                     `json:"monitorPruneJobs,omitempty"`
	MonitorGarbageJobs           bool                     `json:"monitorGarbageJobs,omitempty"`
	ExcludeDatastores            []string                 `json:"excludeDatastores,omitempty"` // PBS only
	MonitorMailStats             bool                     `json:"monitorMailStats,omitempty"`
	MonitorQueues                bool                     `json:"monitorQueues,omitempty"`
	MonitorQuarantine            bool                     `json:"monitorQuarantine,omitempty"`
	MonitorDomainStats           bool                     `json:"monitorDomainStats,omitempty"`
	Status                       string                   `json:"status"` // "connected", "disconnected", "error"
	IsCluster                    bool                     `json:"isCluster,omitempty"`
	ClusterName                  string                   `json:"clusterName,omitempty"`
	ClusterEndpoints             []config.ClusterEndpoint `json:"clusterEndpoints,omitempty"`
	Source                       string                   `json:"source,omitempty"` // "agent" or "script" - how this node was registered
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

// validateNodeAPI tests if a cluster node has a working Proxmox API
// This helps filter out qdevice VMs and other non-Proxmox participants.
// Returns (isValid, fingerprint) - fingerprint is captured for TOFU (Trust On First Use).
func validateNodeAPI(clusterNode proxmox.ClusterStatus, baseConfig proxmox.ClientConfig) (bool, string) {
	// Determine the host to test - prefer IP if available, otherwise use node name
	testHost := clusterNode.IP
	if testHost == "" {
		testHost = clusterNode.Name
	}

	// Skip empty hostnames (shouldn't happen but be safe)
	if testHost == "" {
		return false, ""
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

	log.Debug().
		Str("node", clusterNode.Name).
		Str("test_host", testConfig.Host).
		Msg("Validating Proxmox API for cluster node")

	// Capture the fingerprint for TOFU (Trust On First Use)
	var capturedFingerprint string
	if testHost != "" {
		fp, err := tlsutil.FetchFingerprint(testConfig.Host)
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
	testClient, err := proxmox.NewClient(testConfig)
	if err != nil {
		// Many clusters use unique certificates per node. If the primary node
		// was configured with fingerprint pinning, connecting to peers with the
		// same fingerprint will fail. Fall back to a relaxed TLS check so we can
		// still detect valid cluster members while keeping other errors (like
		// auth) as hard failures.
		errStr := err.Error()
		isTLSMismatch := strings.Contains(errStr, "fingerprint") || strings.Contains(errStr, "x509") || strings.Contains(errStr, "certificate")
		if isTLSMismatch {
			log.Debug().
				Str("node", clusterNode.Name).
				Msg("Retrying cluster node validation without fingerprint pinning")
			testConfig.Fingerprint = ""
			testConfig.VerifySSL = false
			testClient, err = proxmox.NewClient(testConfig)
		}
		if err != nil {
			log.Debug().
				Str("node", clusterNode.Name).
				Err(err).
				Msg("Failed to create test client for cluster node")
			return false, ""
		}
	}

	// Test with a simple API call that all Proxmox nodes should support
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to get the node version - this is a very lightweight API call
	_, err = testClient.GetNodes(ctx)
	if err != nil {
		errMsg := err.Error()
		// If we reached the API but were denied (common when per-node permissions
		// differ), treat it as valid. We only want to filter out hosts that aren't
		// actually Proxmox endpoints.
		if strings.Contains(errMsg, "401") || strings.Contains(errMsg, "403") || strings.Contains(errMsg, "permission") {
			log.Debug().
				Str("node", clusterNode.Name).
				Err(err).
				Msg("Cluster node API responded but denied access; accepting for discovery")
			return true, capturedFingerprint
		}

		log.Debug().
			Str("node", clusterNode.Name).
			Err(err).
			Msg("Node failed Proxmox API validation - likely not a Proxmox node")
		return false, ""
	}

	log.Debug().
		Str("node", clusterNode.Name).
		Msg("Node passed Proxmox API validation")

	return true, capturedFingerprint
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

// extractIPFromHost extracts the IP address from a host URL.
// For example, "https://10.1.1.5:8006" returns 10.1.1.5 as net.IP.
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

var detectPVECluster = defaultDetectPVECluster

// detectPVECluster checks if a PVE node is part of a cluster and returns cluster information
// If existingEndpoints is provided, GuestURL values will be preserved for matching nodes
func defaultDetectPVECluster(clientConfig proxmox.ClientConfig, nodeName string, existingEndpoints []config.ClusterEndpoint) (isCluster bool, clusterName string, clusterEndpoints []config.ClusterEndpoint) {
	tempClient, err := proxmox.NewClient(clientConfig)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create client for cluster detection")
		return false, "", nil
	}

	// Try to get cluster status with retries to handle API permission propagation delays
	// This addresses issue #437 where cluster detection fails on first attempt
	var clusterStatus []proxmox.ClusterStatus
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			// Wait a bit for permissions to propagate
			time.Sleep(time.Duration(attempt) * time.Second)
			log.Debug().Int("attempt", attempt+1).Msg("Retrying cluster detection")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Get full cluster status to find the actual cluster name
		// Note: This can cause certificate lookup errors on standalone nodes, but it's only done once during configuration
		clusterStatus, lastErr = tempClient.GetClusterStatus(ctx)
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

		var unvalidatedNodes []proxmox.ClusterStatus

		for _, clusterNode := range clusterNodes {
			// Validate that this node actually has a working Proxmox API
			// This filters out qdevice VMs and other non-Proxmox participants
			// Also captures the node's TLS fingerprint for TOFU
			isValid, nodeFingerprint := validateNodeAPI(clusterNode, clientConfig)
			if !isValid {
				log.Debug().
					Str("node", clusterNode.Name).
					Str("ip", clusterNode.IP).
					Msg("Skipping cluster node - no valid Proxmox API detected (likely qdevice or external node)")
				unvalidatedNodes = append(unvalidatedNodes, clusterNode)
				continue
			}

			// Build the host URL with proper port
			// Store hostname in Host field (for TLS validation), IP separately
			endpoint := config.ClusterEndpoint{
				NodeID:      clusterNode.ID,
				NodeName:    clusterNode.Name,
				GuestURL:    findExistingGuestURL(clusterNode.Name, existingEndpoints),
				IPOverride:  findExistingIPOverride(clusterNode.Name, existingEndpoints), // Preserve user override
				Fingerprint: nodeFingerprint,                                             // Store captured fingerprint for per-node TLS verification
				Online:      clusterNode.Online == 1,
				LastSeen:    time.Now(),
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
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					nodeInterfaces, err := tempClient.GetNodeNetworkInterfaces(ctx, clusterNode.Name)
					cancel()

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

		if len(clusterEndpoints) == 0 && len(unvalidatedNodes) > 0 {
			log.Warn().
				Str("cluster", clusterName).
				Int("total_discovered", len(unvalidatedNodes)).
				Msg("All detected cluster nodes failed validation; falling back to cluster metadata")

			for _, clusterNode := range unvalidatedNodes {
				if clusterNode.Name == "" && clusterNode.IP == "" {
					continue
				}

				endpoint := config.ClusterEndpoint{
					NodeID:   clusterNode.ID,
					NodeName: clusterNode.Name,
					GuestURL: findExistingGuestURL(clusterNode.Name, existingEndpoints),
					Online:   clusterNode.Online == 1,
					LastSeen: time.Now(),
				}

				// Populate Host field with hostname (if available) for TLS certificate validation
				if clusterNode.Name != "" {
					nodeHost := ensureHostHasPort(clusterNode.Name, defaultPort)
					endpoint.Host = schemePrefix + nodeHost
				}

				// Populate IP field separately for DNS-free connections
				if clusterNode.IP != "" {
					endpoint.IP = clusterNode.IP
				}

				// Apply subnet preference even in fallback path (refs #929)
				// Node validation may have failed because cluster-reported IPs are on internal
				// network, but we can still query node interfaces via the initial connection
				if connectionIP != nil && clusterNode.IP != "" && clusterNode.Name != "" {
					clusterIP := net.ParseIP(clusterNode.IP)
					if clusterIP != nil && !ipsOnSameNetwork(clusterIP, connectionIP) {
						// Cluster IP is on a different network, try to find one on the same network
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						nodeInterfaces, err := tempClient.GetNodeNetworkInterfaces(ctx, clusterNode.Name)
						cancel()

						if err == nil {
							preferredIP := findPreferredIP(nodeInterfaces, connectionIP)
							if preferredIP != "" && preferredIP != clusterNode.IP {
								log.Info().
									Str("node", clusterNode.Name).
									Str("cluster_ip", clusterNode.IP).
									Str("preferred_ip", preferredIP).
									Str("connection_ip", connectionIP.String()).
									Msg("Found preferred management IP for unvalidated cluster node")
								endpoint.IPOverride = preferredIP
							}
						} else {
							log.Debug().
								Err(err).
								Str("node", clusterNode.Name).
								Msg("Could not query node network interfaces in fallback path")
						}
					}
				}

				clusterEndpoints = append(clusterEndpoints, endpoint)
			}
		}

		// Log the final count of valid Proxmox nodes found
		log.Info().
			Str("cluster", clusterName).
			Int("total_discovered", len(clusterNodes)).
			Int("valid_proxmox_nodes", len(clusterEndpoints)).
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

	// Add PVE nodes
	for i := range h.getConfig(ctx).PVEInstances {
		// Refresh cluster metadata if we previously failed to detect endpoints
		h.maybeRefreshClusterInfo(ctx, &h.getConfig(ctx).PVEInstances[i])
		pve := h.getConfig(ctx).PVEInstances[i]
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
			Fingerprint:                  pve.Fingerprint,
			VerifySSL:                    pve.VerifySSL,
			MonitorVMs:                   pve.MonitorVMs,
			MonitorContainers:            pve.MonitorContainers,
			MonitorStorage:               pve.MonitorStorage,
			MonitorBackups:               pve.MonitorBackups,
			MonitorPhysicalDisks:         pve.MonitorPhysicalDisks,
			PhysicalDiskPollingMinutes:   pve.PhysicalDiskPollingMinutes,
			TemperatureMonitoringEnabled: pve.TemperatureMonitoringEnabled,
			Status:                       h.getNodeStatus(ctx, "pve", pve.Name),
			IsCluster:                    pve.IsCluster,
			ClusterName:                  pve.ClusterName,
			ClusterEndpoints:             pve.ClusterEndpoints,
			Source:                       pve.Source,
		}
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
			Status:                       h.getNodeStatus(ctx, "pbs", pbs.Name),
			Source:                       pbs.Source,
		}
		nodes = append(nodes, node)
	}

	// Add PMG nodes
	for i, pmgInst := range h.getConfig(ctx).PMGInstances {
		monitorMailStats := pmgInst.MonitorMailStats
		if !pmgInst.MonitorMailStats && !pmgInst.MonitorQueues && !pmgInst.MonitorQuarantine && !pmgInst.MonitorDomainStats {
			monitorMailStats = true
		}

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
			Status:                       h.getNodeStatus(ctx, "pmg", pmgInst.Name),
		}
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

// HandleSetupScriptURL generates a one-time setup code and URL for the setup script.
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

// HandleAgentInstallCommand generates an API token and install command for agent-based Proxmox setup.
func (h *ConfigHandlers) HandleAgentInstallCommand(w http.ResponseWriter, r *http.Request) {
	h.handleAgentInstallCommand(w, r)
}
