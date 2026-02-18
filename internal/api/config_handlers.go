package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	discoveryinternal "github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/system"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	pkgdiscovery "github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
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
	h.mtMonitor = mtm
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			h.legacyMonitor = m
			h.legacyConfig = m.GetConfig()
		}
	}
}

// SetMonitor updates the monitor reference used by the config handlers (legacy support).
func (h *ConfigHandlers) SetMonitor(m *monitoring.Monitor) {
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
	h.legacyConfig = cfg
}

// getContextState helper to retrieve tenant-specific state
func (h *ConfigHandlers) getContextState(ctx context.Context) (*config.Config, *config.ConfigPersistence, *monitoring.Monitor) {
	orgID := "default"
	if ctx != nil {
		if id := GetOrgID(ctx); id != "" {
			orgID = id
		}
	}

	// Try to get from multi-tenant managers first
	if h.mtMonitor != nil {
		if m, err := h.mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			cfg := m.GetConfig()
			var p *config.ConfigPersistence
			if h.mtPersistence != nil {
				p, _ = h.mtPersistence.GetPersistence(orgID)
			}
			return cfg, p, m
		} else if err != nil {
			log.Warn().Str("orgID", orgID).Err(err).Msg("Falling back to legacy config - failed to get tenant monitor")
		}
	}

	// Fallback to legacy (should mostly happen for "default" or initialization)
	return h.legacyConfig, h.legacyPersistence, h.legacyMonitor
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
	// Check if mock mode is enabled
	if mock.IsMockEnabled() {
		// Return mock nodes for settings page
		mockNodes := []NodeResponse{}

		// Get mock state to extract node information
		state := h.getMonitor(r.Context()).GetState()

		// Get all cluster nodes and standalone nodes
		var clusterNodes []models.Node
		var standaloneNodes []models.Node

		for _, node := range state.Nodes {
			if node.Instance == "mock-cluster" {
				clusterNodes = append(clusterNodes, node)
			} else {
				standaloneNodes = append(standaloneNodes, node)
			}
		}

		// If we have cluster nodes, create ONE config entry for the cluster
		if len(clusterNodes) > 0 {
			// Build cluster endpoints for cluster nodes only
			var clusterEndpoints []config.ClusterEndpoint
			for i, n := range clusterNodes {
				clusterEndpoints = append(clusterEndpoints, config.ClusterEndpoint{
					NodeName: n.Name,
					Host:     fmt.Sprintf("192.168.0.%d:8006", 100+i),
					Online:   n.Status == "online", // Set Online based on node status
				})
			}

			// Create a single cluster entry (representing the cluster config)
			clusterNode := NodeResponse{
				ID:                   generateNodeID("pve", 0),
				Type:                 "pve",
				Name:                 "mock-cluster",       // The cluster name
				Host:                 "192.168.0.100:8006", // Primary entry point
				User:                 "root@pam",
				HasPassword:          true,
				TokenName:            "pulse",
				HasToken:             true,
				Fingerprint:          "",
				VerifySSL:            false,
				MonitorVMs:           true,
				MonitorContainers:    true,
				MonitorStorage:       true,
				MonitorBackups:       true,
				MonitorPhysicalDisks: nil, // nil = enabled by default
				Status:               "connected",
				IsCluster:            true,
				ClusterName:          "mock-cluster",
				ClusterEndpoints:     clusterEndpoints, // All cluster nodes
			}
			mockNodes = append(mockNodes, clusterNode)
		}

		// Add standalone nodes as individual entries
		for i, node := range standaloneNodes {
			standaloneNode := NodeResponse{
				ID:                   generateNodeID("pve", len(mockNodes)+i),
				Type:                 "pve",
				Name:                 node.Name,                               // Use the actual node name
				Host:                 fmt.Sprintf("192.168.0.%d:8006", 150+i), // Different IP range for standalone
				User:                 "root@pam",
				HasPassword:          true,
				TokenName:            "pulse",
				HasToken:             true,
				Fingerprint:          "",
				VerifySSL:            false,
				MonitorVMs:           true,
				MonitorContainers:    true,
				MonitorStorage:       true,
				MonitorBackups:       true,
				MonitorPhysicalDisks: nil, // nil = enabled by default
				Status:               "connected",
				IsCluster:            false, // Not part of a cluster
				ClusterName:          "",
				ClusterEndpoints:     []config.ClusterEndpoint{},
			}
			mockNodes = append(mockNodes, standaloneNode)
		}

		// Add mock PBS instances
		for i, pbs := range state.PBSInstances {
			pbsNode := NodeResponse{
				ID:                 generateNodeID("pbs", i),
				Type:               "pbs",
				Name:               pbs.Name,
				Host:               pbs.Host,
				User:               "pulse@pbs",
				HasPassword:        false,
				TokenName:          "pulse",
				HasToken:           true,
				Fingerprint:        "",
				VerifySSL:          false,
				MonitorDatastores:  true,
				MonitorSyncJobs:    true,
				MonitorVerifyJobs:  true,
				MonitorPruneJobs:   true,
				MonitorGarbageJobs: true,
				Status:             "connected", // Always connected in mock mode
			}
			mockNodes = append(mockNodes, pbsNode)
		}

		// Add mock PMG instances
		for i, pmg := range state.PMGInstances {
			pmgNode := NodeResponse{
				ID:          generateNodeID("pmg", i),
				Type:        "pmg",
				Name:        pmg.Name,
				Host:        pmg.Host,
				User:        "root@pam",
				HasPassword: true,
				TokenName:   "pulse",
				HasToken:    true,
				Fingerprint: "",
				VerifySSL:   false,
				Status:      "connected", // Always connected in mock mode
			}
			mockNodes = append(mockNodes, pmgNode)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockNodes)
		return
	}

	nodes := h.GetAllNodesForAPI(r.Context())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

// validateIPAddress validates if a string is a valid IP address
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
	// Prevent node modifications in mock mode
	if mock.IsMockEnabled() {
		http.Error(w, "Cannot modify nodes in mock mode. Please disable mock mode first: /opt/pulse/scripts/toggle-mock.sh off", http.StatusForbidden)
		return
	}

	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	var req NodeConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode add node request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Info().
		Str("type", req.Type).
		Str("name", req.Name).
		Str("host", req.Host).
		Str("user", req.User).
		Str("tokenName", req.TokenName).
		Bool("hasTokenValue", req.TokenValue != "").
		Msg("Add node request received")

	// Validate required fields
	if req.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		http.Error(w, "Type is required", http.StatusBadRequest)
		return
	}

	if req.Host == "" {
		http.Error(w, "Host is required", http.StatusBadRequest)
		return
	}

	// Validate host format (IP address or hostname with optional port)
	host, port, err := extractHostAndPort(req.Host)
	if err != nil {
		http.Error(w, "Invalid host format", http.StatusBadRequest)
		return
	}

	// If it looks like an IP address, validate it strictly
	// Check if it starts with a digit (likely an IP)
	if len(host) > 0 && (host[0] >= '0' && host[0] <= '9') {
		// Likely an IP address, validate strictly
		if !validateIPAddress(host) {
			http.Error(w, "Invalid IP address", http.StatusBadRequest)
			return
		}
	} else if strings.Contains(host, ":") && strings.Contains(host, "[") {
		// IPv6 address with brackets
		ipv6 := strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
		if !validateIPAddress(ipv6) {
			http.Error(w, "Invalid IPv6 address", http.StatusBadRequest)
			return
		}
	} else if req.Type == "pbs" {
		// Validate as hostname - no spaces or special characters
		if strings.ContainsAny(host, " /\\<>|\"'`;") {
			http.Error(w, "Invalid hostname", http.StatusBadRequest)
			return
		}
	}

	// Validate port if provided
	if port != "" && !validatePort(port) {
		http.Error(w, "Invalid port number", http.StatusBadRequest)
		return
	}

	if req.Type != "pve" && req.Type != "pbs" && req.Type != "pmg" {
		http.Error(w, "Invalid node type", http.StatusBadRequest)
		return
	}

	// Check for authentication
	hasAuth := (req.User != "" && req.Password != "") || (req.TokenName != "" && req.TokenValue != "")
	if !hasAuth {
		http.Error(w, "Authentication credentials required", http.StatusBadRequest)
		return
	}

	normalizedHost, err := normalizeNodeHost(req.Host, req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check for duplicate nodes by HOST URL (not name!)
	// Different physical hosts can share the same hostname (Issue #891).
	// We disambiguate names later, but Host URLs must be unique.
	switch req.Type {
	case "pve":
		for _, node := range h.getConfig(r.Context()).PVEInstances {
			if node.Host == normalizedHost {
				http.Error(w, "A node with this host URL already exists", http.StatusConflict)
				return
			}
		}
	case "pbs":
		for _, node := range h.getConfig(r.Context()).PBSInstances {
			if node.Host == normalizedHost {
				http.Error(w, "A node with this host URL already exists", http.StatusConflict)
				return
			}
		}
	case "pmg":
		for _, node := range h.getConfig(r.Context()).PMGInstances {
			if node.Host == normalizedHost {
				http.Error(w, "A node with this host URL already exists", http.StatusConflict)
				return
			}
		}
	}

	// Add to appropriate list
	if req.Type == "pve" {
		if req.Password != "" && req.TokenName == "" && req.TokenValue == "" {
			req.User = normalizePVEUser(req.User)
		}
		host := normalizedHost

		// Check if node is part of a cluster (skip for test/invalid IPs)
		var isCluster bool
		var clusterName string
		var clusterEndpoints []config.ClusterEndpoint

		// Skip cluster detection for obviously test/invalid IPs
		skipClusterDetection := strings.Contains(req.Host, "192.168.77.") ||
			strings.Contains(req.Host, "192.168.88.") ||
			strings.Contains(req.Host, "test-") ||
			strings.Contains(req.Name, "test-") ||
			strings.Contains(req.Name, "persist-") ||
			strings.Contains(req.Name, "concurrent-")

		if !skipClusterDetection {
			verifySSL := false
			if req.VerifySSL != nil {
				verifySSL = *req.VerifySSL
			}
			clientConfig := config.CreateProxmoxConfigFromFields(host, req.User, req.Password, req.TokenName, req.TokenValue, req.Fingerprint, verifySSL)
			isCluster, clusterName, clusterEndpoints = detectPVECluster(clientConfig, req.Name, nil)
		}

		// CLUSTER DEDUPLICATION: If this node is part of a cluster, check if we already
		// have that cluster configured. If so, this is a duplicate - we should merge
		// the node as an endpoint to the existing cluster instead of creating a new instance.
		// This prevents duplicate VMs/containers when users install agents on multiple cluster nodes.
		if isCluster && clusterName != "" {
			for i := range h.getConfig(r.Context()).PVEInstances {
				existingInstance := &h.getConfig(r.Context()).PVEInstances[i]
				if existingInstance.IsCluster && existingInstance.ClusterName == clusterName {
					// Found existing cluster with same name - merge endpoints!
					log.Info().
						Str("cluster", clusterName).
						Str("existingInstance", existingInstance.Name).
						Str("newNode", req.Name).
						Msg("New node belongs to already-configured cluster - merging as endpoint instead of creating duplicate")

					// Merge any new endpoints from the detected cluster
					existingEndpointMap := make(map[string]bool)
					for _, ep := range existingInstance.ClusterEndpoints {
						existingEndpointMap[ep.NodeName] = true
					}
					for _, newEp := range clusterEndpoints {
						if !existingEndpointMap[newEp.NodeName] {
							existingInstance.ClusterEndpoints = append(existingInstance.ClusterEndpoints, newEp)
							log.Info().
								Str("cluster", clusterName).
								Str("endpoint", newEp.NodeName).
								Msg("Added new endpoint to existing cluster")
						}
					}

					// Save the updated configuration
					if h.getPersistence(r.Context()) != nil {
						if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
							log.Warn().Err(err).Msg("Failed to persist cluster endpoint merge")
						}
					}

					// Reload the monitor to pick up the updated endpoints
					if h.reloadFunc != nil {
						if err := h.reloadFunc(); err != nil {
							log.Warn().Err(err).Msg("Failed to reload monitor after cluster merge")
						}
					}

					// Return success - the cluster is now updated with new endpoints
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success":        true,
						"merged":         true,
						"cluster":        clusterName,
						"existingNode":   existingInstance.Name,
						"message":        fmt.Sprintf("Node merged into existing cluster '%s' (already configured as '%s')", clusterName, existingInstance.Name),
						"totalEndpoints": len(existingInstance.ClusterEndpoints),
					})
					return
				}
			}
		}

		if isCluster {
			log.Info().
				Str("cluster", clusterName).
				Int("endpoints", len(clusterEndpoints)).
				Msg("Detected new Proxmox cluster, auto-discovering all nodes")
		}

		// Use sensible defaults for boolean fields if not provided
		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}
		monitorVMs := true // Default to true
		if req.MonitorVMs != nil {
			monitorVMs = *req.MonitorVMs
		}
		monitorContainers := true // Default to true
		if req.MonitorContainers != nil {
			monitorContainers = *req.MonitorContainers
		}
		monitorStorage := true // Default to true
		if req.MonitorStorage != nil {
			monitorStorage = *req.MonitorStorage
		}
		monitorBackups := true // Default to true
		if req.MonitorBackups != nil {
			monitorBackups = *req.MonitorBackups
		}

		// Disambiguate name if duplicate hostnames exist (Issue #891)
		displayName := h.disambiguateNodeName(r.Context(), req.Name, host, "pve")

		pve := config.PVEInstance{
			Name:                         displayName,
			Host:                         host, // Use normalized host
			GuestURL:                     req.GuestURL,
			User:                         req.User,
			Password:                     req.Password,
			TokenName:                    req.TokenName,
			TokenValue:                   req.TokenValue,
			Fingerprint:                  req.Fingerprint,
			VerifySSL:                    verifySSL,
			MonitorVMs:                   monitorVMs,
			MonitorContainers:            monitorContainers,
			MonitorStorage:               monitorStorage,
			MonitorBackups:               monitorBackups,
			MonitorPhysicalDisks:         req.MonitorPhysicalDisks,
			PhysicalDiskPollingMinutes:   0,
			TemperatureMonitoringEnabled: req.TemperatureMonitoringEnabled,
			IsCluster:                    isCluster,
			ClusterName:                  clusterName,
			ClusterEndpoints:             clusterEndpoints,
		}
		if req.PhysicalDiskPollingMinutes != nil {
			pve.PhysicalDiskPollingMinutes = *req.PhysicalDiskPollingMinutes
		}

		h.getConfig(r.Context()).PVEInstances = append(h.getConfig(r.Context()).PVEInstances, pve)

		if isCluster {
			log.Info().
				Str("cluster", clusterName).
				Int("endpoints", len(clusterEndpoints)).
				Msg("Added Proxmox cluster with auto-discovered endpoints")
		}
	} else if req.Type == "pbs" {
		host := normalizedHost

		// Parse PBS authentication details
		var pbsUser string
		var pbsPassword string
		var pbsTokenName string
		var pbsTokenValue string

		// Determine authentication method
		if req.TokenName != "" && req.TokenValue != "" {
			// Using token authentication - don't store user/password
			pbsTokenName = req.TokenName
			pbsTokenValue = req.TokenValue
			// Token name might contain the full format (user@realm!tokenname)
			// The backend PBS client will parse this
		} else if req.Password != "" {
			// Using password authentication - try to create a token via API
			// This enables turnkey setup for Docker/containerized PBS
			pbsUser = req.User
			if pbsUser != "" && !strings.Contains(pbsUser, "@") {
				pbsUser = pbsUser + "@pbs" // Default to @pbs realm if not specified
			}

			log.Info().
				Str("host", host).
				Str("user", pbsUser).
				Msg("PBS: Attempting turnkey token creation via API")

			// Try to create a token using the provided credentials
			pbsClient, err := pbs.NewClient(pbs.ClientConfig{
				Host:      host,
				User:      pbsUser,
				Password:  req.Password,
				VerifySSL: false, // Self-signed certs common
			})

			if err != nil {
				log.Warn().Err(err).Str("host", host).Msg("PBS: Failed to connect for token creation, falling back to password auth")
				// Fallback to password auth
				pbsPassword = req.Password
			} else {
				// Generate a unique token name
				hostname, _ := os.Hostname()
				if hostname == "" {
					hostname = "pulse"
				}
				timestamp := time.Now().Unix()
				tokenName := fmt.Sprintf("pulse-%s-%d", hostname, timestamp)

				tokenID, tokenSecret, err := pbsClient.SetupMonitoringAccess(context.Background(), tokenName)
				if err != nil {
					log.Warn().Err(err).Str("host", host).Msg("PBS: Failed to create token via API, falling back to password auth")
					// Fallback to password auth
					pbsPassword = req.Password
				} else {
					// Successfully created token - use it instead of password
					pbsTokenName = tokenID
					pbsTokenValue = tokenSecret
					pbsUser = "" // Clear password auth fields
					pbsPassword = ""
					log.Info().
						Str("host", host).
						Str("tokenID", tokenID).
						Msg("PBS: Successfully created monitoring token via API")
				}
			}
		}

		// Use sensible defaults for boolean fields if not provided
		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}
		monitorBackups := true // Default to true for PBS
		if req.MonitorBackups != nil {
			monitorBackups = *req.MonitorBackups
		}
		monitorDatastores := false
		if req.MonitorDatastores != nil {
			monitorDatastores = *req.MonitorDatastores
		}
		monitorSyncJobs := false
		if req.MonitorSyncJobs != nil {
			monitorSyncJobs = *req.MonitorSyncJobs
		}
		monitorVerifyJobs := false
		if req.MonitorVerifyJobs != nil {
			monitorVerifyJobs = *req.MonitorVerifyJobs
		}
		monitorPruneJobs := false
		if req.MonitorPruneJobs != nil {
			monitorPruneJobs = *req.MonitorPruneJobs
		}
		monitorGarbageJobs := false
		if req.MonitorGarbageJobs != nil {
			monitorGarbageJobs = *req.MonitorGarbageJobs
		}

		// Disambiguate name if duplicate hostnames exist (Issue #891)
		pbsDisplayName := h.disambiguateNodeName(r.Context(), req.Name, host, "pbs")

		pbs := config.PBSInstance{
			Name:                         pbsDisplayName,
			Host:                         host,
			GuestURL:                     req.GuestURL,
			User:                         pbsUser,
			Password:                     pbsPassword,
			TokenName:                    pbsTokenName,
			TokenValue:                   pbsTokenValue,
			Fingerprint:                  req.Fingerprint,
			VerifySSL:                    verifySSL,
			MonitorBackups:               monitorBackups,
			MonitorDatastores:            monitorDatastores,
			MonitorSyncJobs:              monitorSyncJobs,
			MonitorVerifyJobs:            monitorVerifyJobs,
			MonitorPruneJobs:             monitorPruneJobs,
			MonitorGarbageJobs:           monitorGarbageJobs,
			TemperatureMonitoringEnabled: req.TemperatureMonitoringEnabled,
		}
		h.getConfig(r.Context()).PBSInstances = append(h.getConfig(r.Context()).PBSInstances, pbs)
	} else if req.Type == "pmg" {
		host := normalizedHost

		var pmgUser string
		var pmgPassword string
		var pmgTokenName string
		var pmgTokenValue string

		if req.TokenName != "" && req.TokenValue != "" {
			pmgTokenName = req.TokenName
			pmgTokenValue = req.TokenValue
		} else if req.Password != "" {
			pmgUser = req.User
			pmgPassword = req.Password
			if pmgUser != "" && !strings.Contains(pmgUser, "@") {
				pmgUser = pmgUser + "@pmg"
			}
		}

		// Use sensible defaults for boolean fields if not provided
		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}

		// Check if any monitoring flags are explicitly set to true
		anyMonitoringEnabled := (req.MonitorMailStats != nil && *req.MonitorMailStats) ||
			(req.MonitorQueues != nil && *req.MonitorQueues) ||
			(req.MonitorQuarantine != nil && *req.MonitorQuarantine) ||
			(req.MonitorDomainStats != nil && *req.MonitorDomainStats)

		// Default MonitorMailStats to true if no monitoring is explicitly enabled
		monitorMailStats := true // Default to true
		if req.MonitorMailStats != nil {
			monitorMailStats = *req.MonitorMailStats
		} else if anyMonitoringEnabled {
			monitorMailStats = false // Don't default to true if other monitoring is enabled
		}

		monitorQueues := false
		if req.MonitorQueues != nil {
			monitorQueues = *req.MonitorQueues
		}
		monitorQuarantine := false
		if req.MonitorQuarantine != nil {
			monitorQuarantine = *req.MonitorQuarantine
		}
		monitorDomainStats := false
		if req.MonitorDomainStats != nil {
			monitorDomainStats = *req.MonitorDomainStats
		}

		// Disambiguate name if duplicate hostnames exist (Issue #891)
		// Note: PMG uses similar logic to PBS - we check against PMG instances
		pmgDisplayName := req.Name
		for _, node := range h.getConfig(r.Context()).PMGInstances {
			if strings.EqualFold(node.Name, req.Name) && node.Host != host {
				parsed, err := url.Parse(host)
				if err == nil && parsed.Host != "" {
					pmgDisplayName = fmt.Sprintf("%s (%s)", req.Name, parsed.Hostname())
				}
				break
			}
		}

		pmgInstance := config.PMGInstance{
			Name:                         pmgDisplayName,
			Host:                         host,
			GuestURL:                     req.GuestURL,
			User:                         pmgUser,
			Password:                     pmgPassword,
			TokenName:                    pmgTokenName,
			TokenValue:                   pmgTokenValue,
			Fingerprint:                  req.Fingerprint,
			VerifySSL:                    verifySSL,
			MonitorMailStats:             monitorMailStats,
			MonitorQueues:                monitorQueues,
			MonitorQuarantine:            monitorQuarantine,
			MonitorDomainStats:           monitorDomainStats,
			TemperatureMonitoringEnabled: req.TemperatureMonitoringEnabled,
		}
		h.getConfig(r.Context()).PMGInstances = append(h.getConfig(r.Context()).PMGInstances, pmgInstance)
	}

	// Save configuration to disk using our persistence instance
	if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save nodes configuration")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Reload monitor with new configuration
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor")
			http.Error(w, "Configuration saved but failed to apply changes", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleTestConnection tests a node connection without saving
func (h *ConfigHandlers) HandleTestConnection(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	var req NodeConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode test connection request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Info().
		Str("type", req.Type).
		Str("name", req.Name).
		Str("host", req.Host).
		Str("user", req.User).
		Str("tokenName", req.TokenName).
		Bool("hasTokenValue", req.TokenValue != "").
		Msg("Test connection request received")

	// Parse token format if needed
	user := req.User
	tokenName := req.TokenName

	// If tokenName contains the full format (user@realm!tokenname), parse it
	if strings.Contains(req.TokenName, "!") {
		parts := strings.Split(req.TokenName, "!")
		if len(parts) == 2 {
			user = parts[0]
			tokenName = parts[1]
		}
	}
	// If user field contains the full format, extract just the user part
	if strings.Contains(user, "!") {
		parts := strings.Split(user, "!")
		if len(parts) >= 1 {
			user = parts[0]
		}
	}

	log.Info().
		Str("parsedUser", user).
		Str("parsedTokenName", tokenName).
		Msg("Parsed authentication details")

	// Validate request
	if req.Host == "" {
		http.Error(w, "Host is required", http.StatusBadRequest)
		return
	}

	// Auto-generate name if not provided for test
	if req.Name == "" {
		// Extract hostname from URL
		host := strings.TrimPrefix(strings.TrimPrefix(req.Host, "http://"), "https://")
		// Remove port
		if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
			host = host[:colonIndex]
		}
		req.Name = host
	}

	if req.Type != "pve" && req.Type != "pbs" && req.Type != "pmg" {
		http.Error(w, "Invalid node type", http.StatusBadRequest)
		return
	}

	// Check for authentication
	hasAuth := (user != "" && req.Password != "") || (tokenName != "" && req.TokenValue != "")
	if !hasAuth {
		http.Error(w, "Authentication credentials required", http.StatusBadRequest)
		return
	}

	normalizedHost, err := normalizeNodeHost(req.Host, req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Test connection based on type
	if req.Type == "pve" {
		host := normalizedHost

		// Create a temporary client
		authUser := req.User
		if req.Password != "" && req.TokenName == "" && req.TokenValue == "" {
			authUser = normalizePVEUser(authUser)
			req.User = authUser
		}
		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}
		clientConfig := proxmox.ClientConfig{
			Host:        host,
			User:        authUser,
			Password:    req.Password,
			TokenName:   req.TokenName, // Pass the full token ID
			TokenValue:  req.TokenValue,
			VerifySSL:   verifySSL,
			Fingerprint: req.Fingerprint,
		}

		tempClient, err := proxmox.NewClient(clientConfig)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "create_client"), http.StatusBadRequest)
			return
		}

		// Try to get nodes to test connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nodes, err := tempClient.GetNodes(ctx)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "connection"), http.StatusBadRequest)
			return
		}

		isCluster, _, clusterEndpoints := detectPVECluster(clientConfig, req.Name, nil)

		response := map[string]interface{}{
			"status":    "success",
			"message":   fmt.Sprintf("Successfully connected to %d node(s)", len(nodes)),
			"isCluster": isCluster,
			"nodeCount": len(nodes),
		}

		if isCluster {
			response["clusterNodeCount"] = len(clusterEndpoints)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else if req.Type == "pbs" {
		host := normalizedHost

		log.Info().
			Str("processedHost", host).
			Msg("PBS host after port processing")

		// PBS test connection
		// Parse PBS authentication details
		pbsUser := user
		pbsTokenName := tokenName

		// Handle different token input formats
		if req.TokenName != "" && req.TokenValue != "" {
			// Check if token name contains the full format (user@realm!tokenname)
			if strings.Contains(req.TokenName, "!") {
				// Token name is in full format, leave it as is
				// The PBS client will parse it
			} else if pbsUser != "" && !strings.Contains(pbsUser, "@") {
				// User provided separately without realm, add default realm
				pbsUser = pbsUser + "@pbs"
			}
		} else if pbsUser != "" && !strings.Contains(pbsUser, "@") {
			// Password auth: ensure user has realm
			pbsUser = pbsUser + "@pbs" // Default to @pbs realm if not specified
		}

		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}
		clientConfig := pbs.ClientConfig{
			Host:        host,
			User:        pbsUser,
			Password:    req.Password,
			TokenName:   pbsTokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   verifySSL,
			Fingerprint: req.Fingerprint,
		}

		tempClient, err := pbs.NewClient(clientConfig)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "create_client"), http.StatusBadRequest)
			return
		}

		// Try to get datastores to test connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		datastores, err := tempClient.GetDatastores(ctx)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "connection"), http.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"status":         "success",
			"message":        fmt.Sprintf("Successfully connected. Found %d datastore(s)", len(datastores)),
			"datastoreCount": len(datastores),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		host := normalizedHost

		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}
		clientConfig := config.CreatePMGConfigFromFields(host, req.User, req.Password, req.TokenName, req.TokenValue, req.Fingerprint, verifySSL)

		if req.Password != "" && req.TokenName == "" && req.TokenValue == "" {
			if clientConfig.User != "" && !strings.Contains(clientConfig.User, "@") {
				clientConfig.User = clientConfig.User + "@pmg"
			}
		} else if req.TokenName != "" && req.TokenValue != "" {
			if user != "" {
				normalizedUser := user
				if !strings.Contains(normalizedUser, "@") {
					normalizedUser = normalizedUser + "@pmg"
				}
				clientConfig.User = normalizedUser
			}
		}

		tempClient, err := pmg.NewClient(clientConfig)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "create_client"), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		version, err := tempClient.GetVersion(ctx)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "connection"), http.StatusBadRequest)
			return
		}

		versionLabel := ""
		if version != nil && strings.TrimSpace(version.Version) != "" {
			versionLabel = strings.TrimSpace(version.Version)
			if strings.TrimSpace(version.Release) != "" {
				versionLabel = versionLabel + "-" + strings.TrimSpace(version.Release)
			}
		}

		// Test actual metrics endpoints to ensure monitoring will work
		warnings := []string{}

		// Test mail statistics endpoint (core PMG functionality)
		if _, err := tempClient.GetMailStatistics(ctx, "day"); err != nil {
			warnings = append(warnings, "Mail statistics endpoint unavailable - check user permissions")
			log.Warn().Err(err).Msg("PMG connection test: mail statistics check failed")
		}

		// Test cluster status endpoint
		if _, err := tempClient.GetClusterStatus(ctx, true); err != nil {
			warnings = append(warnings, "Cluster status endpoint unavailable")
			log.Warn().Err(err).Msg("PMG connection test: cluster status check failed")
		}

		// Test quarantine endpoint
		if _, err := tempClient.GetQuarantineStatus(ctx, "spam"); err != nil {
			warnings = append(warnings, "Quarantine endpoint unavailable")
			log.Warn().Err(err).Msg("PMG connection test: quarantine check failed")
		}

		message := "Connected to PMG instance"
		if versionLabel != "" {
			message = fmt.Sprintf("Connected to PMG instance (version %s)", versionLabel)
		}
		if len(warnings) > 0 {
			message += " (some metrics may be unavailable - check logs for details)"
		}

		response := map[string]interface{}{
			"status":  "success",
			"message": message,
		}

		if version != nil {
			if version.Version != "" {
				response["version"] = strings.TrimSpace(version.Version)
			}
			if version.Release != "" {
				response["release"] = strings.TrimSpace(version.Release)
			}
		}

		if len(warnings) > 0 {
			response["warnings"] = warnings
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleUpdateNode updates an existing node
func (h *ConfigHandlers) HandleUpdateNode(w http.ResponseWriter, r *http.Request) {
	// Prevent node modifications in mock mode
	if mock.IsMockEnabled() {
		http.Error(w, "Cannot modify nodes in mock mode", http.StatusForbidden)
		return
	}

	nodeID := strings.TrimPrefix(r.URL.Path, "/api/config/nodes/")
	if nodeID == "" {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}

	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	var req NodeConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Debug: Log the received temperatureMonitoringEnabled value
	log.Info().
		Str("nodeID", nodeID).
		Interface("temperatureMonitoringEnabled", req.TemperatureMonitoringEnabled).
		Msg("Received node update request")

	// Parse node ID
	parts := strings.Split(nodeID, "-")
	if len(parts) != 2 {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	nodeType := parts[0]
	index := 0
	if _, err := fmt.Sscanf(parts[1], "%d", &index); err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	// Update the node
	if nodeType == "pve" && index < len(h.getConfig(r.Context()).PVEInstances) {
		pve := &h.getConfig(r.Context()).PVEInstances[index]

		// Only update name if provided
		if req.Name != "" {
			pve.Name = req.Name
		}

		if req.Host != "" {
			host, err := normalizeNodeHost(req.Host, nodeType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			pve.Host = host
		}

		// Update GuestURL if provided
		pve.GuestURL = req.GuestURL

		// Handle authentication updates - only switch auth method if explicitly provided
		if req.TokenName != "" || req.TokenValue != "" {
			// Switching to or updating token authentication
			if req.TokenName != "" {
				pve.TokenName = req.TokenName
			}
			if req.TokenValue != "" {
				pve.TokenValue = req.TokenValue
			}
			// Clear password to avoid conflicts
			pve.Password = ""
			if req.User != "" {
				pve.User = req.User
			}
		} else if req.Password != "" {
			// Explicitly switching to password authentication
			if req.User != "" {
				pve.User = normalizePVEUser(req.User)
			} else if pve.User != "" {
				pve.User = normalizePVEUser(pve.User)
			}
			pve.Password = req.Password
			// Clear token fields when switching to password auth
			pve.TokenName = ""
			pve.TokenValue = ""
		} else {
			// No authentication changes - preserve existing auth fields
			// Only normalize user if it exists
			if pve.User != "" {
				pve.User = normalizePVEUser(pve.User)
			}
		}

		pve.Fingerprint = req.Fingerprint
		if req.VerifySSL != nil {
			pve.VerifySSL = *req.VerifySSL
		}
		if req.MonitorVMs != nil {
			pve.MonitorVMs = *req.MonitorVMs
		}
		if req.MonitorContainers != nil {
			pve.MonitorContainers = *req.MonitorContainers
		}
		if req.MonitorStorage != nil {
			pve.MonitorStorage = *req.MonitorStorage
		}
		if req.MonitorBackups != nil {
			pve.MonitorBackups = *req.MonitorBackups
		}
		if req.MonitorPhysicalDisks != nil {
			pve.MonitorPhysicalDisks = req.MonitorPhysicalDisks
		}
		if req.PhysicalDiskPollingMinutes != nil {
			pve.PhysicalDiskPollingMinutes = *req.PhysicalDiskPollingMinutes
		}
		if req.TemperatureMonitoringEnabled != nil {
			pve.TemperatureMonitoringEnabled = req.TemperatureMonitoringEnabled
		}
	} else if nodeType == "pbs" && index < len(h.getConfig(r.Context()).PBSInstances) {
		pbs := &h.getConfig(r.Context()).PBSInstances[index]

		// Only update name if provided
		if req.Name != "" {
			pbs.Name = req.Name
		}

		if req.Host != "" {
			host, err := normalizeNodeHost(req.Host, nodeType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			pbs.Host = host
		}

		// Update GuestURL if provided
		pbs.GuestURL = req.GuestURL

		// Handle authentication updates - only switch auth method if explicitly provided
		if req.TokenName != "" && req.TokenValue != "" {
			// Switching to token authentication
			pbs.TokenName = req.TokenName
			pbs.TokenValue = req.TokenValue
			// Clear user/password when switching to token auth
			pbs.User = ""
			pbs.Password = ""
		} else if req.TokenName != "" {
			// Token name provided without new value - keep existing token value
			pbs.TokenName = req.TokenName
			// Clear user/password when using token auth
			pbs.User = ""
			pbs.Password = ""
		} else if req.Password != "" {
			// Switching to password authentication
			pbs.Password = req.Password
			// Ensure user has realm for PBS
			pbsUser := req.User
			if req.User != "" && !strings.Contains(req.User, "@") {
				pbsUser = req.User + "@pbs" // Default to @pbs realm if not specified
			}
			pbs.User = pbsUser
			// Clear token fields when switching to password auth
			pbs.TokenName = ""
			pbs.TokenValue = ""
		} else if req.User != "" {
			// User provided - assume password auth but keep existing password
			// Ensure user has realm for PBS
			pbsUser := req.User
			if !strings.Contains(req.User, "@") {
				pbsUser = req.User + "@pbs" // Default to @pbs realm if not specified
			}
			pbs.User = pbsUser
			// Clear token fields when using password auth
			pbs.TokenName = ""
			pbs.TokenValue = ""
		}
		// else: No authentication changes - preserve existing auth fields

		pbs.Fingerprint = req.Fingerprint
		if req.VerifySSL != nil {
			pbs.VerifySSL = *req.VerifySSL
		}
		if req.MonitorBackups != nil {
			pbs.MonitorBackups = *req.MonitorBackups
		} else {
			pbs.MonitorBackups = true // Enable by default for PBS
		}
		if req.MonitorDatastores != nil {
			pbs.MonitorDatastores = *req.MonitorDatastores
		}
		if req.MonitorSyncJobs != nil {
			pbs.MonitorSyncJobs = *req.MonitorSyncJobs
		}
		if req.MonitorVerifyJobs != nil {
			pbs.MonitorVerifyJobs = *req.MonitorVerifyJobs
		}
		if req.MonitorPruneJobs != nil {
			pbs.MonitorPruneJobs = *req.MonitorPruneJobs
		}
		if req.MonitorGarbageJobs != nil {
			pbs.MonitorGarbageJobs = *req.MonitorGarbageJobs
		}
		if req.TemperatureMonitoringEnabled != nil {
			pbs.TemperatureMonitoringEnabled = req.TemperatureMonitoringEnabled
		}
		// Update datastore exclusion list
		if req.ExcludeDatastores != nil {
			pbs.ExcludeDatastores = req.ExcludeDatastores
		}
	} else if nodeType == "pmg" && index < len(h.getConfig(r.Context()).PMGInstances) {
		pmgInst := &h.getConfig(r.Context()).PMGInstances[index]
		pmgInst.Name = req.Name

		if req.Host != "" {
			host, err := normalizeNodeHost(req.Host, nodeType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			pmgInst.Host = host
		}

		// Update GuestURL if provided
		pmgInst.GuestURL = req.GuestURL

		// Handle authentication updates - only switch auth method if explicitly provided
		if req.TokenName != "" && req.TokenValue != "" {
			// Switching to token authentication
			pmgInst.TokenName = req.TokenName
			pmgInst.TokenValue = req.TokenValue
			// Clear user/password when switching to token auth
			pmgInst.User = ""
			pmgInst.Password = ""
		} else if req.Password != "" {
			// Switching to password authentication
			if req.User != "" {
				user := req.User
				if !strings.Contains(user, "@") {
					user = user + "@pmg"
				}
				pmgInst.User = user
			}
			pmgInst.Password = req.Password
			// Clear token fields when switching to password auth
			pmgInst.TokenName = ""
			pmgInst.TokenValue = ""
		} else if req.User != "" {
			// User provided - assume password auth but keep existing password
			user := req.User
			if !strings.Contains(user, "@") {
				user = user + "@pmg"
			}
			pmgInst.User = user
			// Clear token fields when using password auth
			pmgInst.TokenName = ""
			pmgInst.TokenValue = ""
		}
		// else: No authentication changes - preserve existing auth fields

		pmgInst.Fingerprint = req.Fingerprint
		if req.VerifySSL != nil {
			pmgInst.VerifySSL = *req.VerifySSL
		}
		// Special logic for MonitorMailStats: default to true if all monitor flags are false/unset
		if req.MonitorMailStats != nil {
			pmgInst.MonitorMailStats = *req.MonitorMailStats
		} else if (req.MonitorMailStats == nil || !*req.MonitorMailStats) &&
			(req.MonitorQueues == nil || !*req.MonitorQueues) &&
			(req.MonitorQuarantine == nil || !*req.MonitorQuarantine) &&
			(req.MonitorDomainStats == nil || !*req.MonitorDomainStats) {
			pmgInst.MonitorMailStats = true
		}
		if req.MonitorQueues != nil {
			pmgInst.MonitorQueues = *req.MonitorQueues
		}
		if req.MonitorQuarantine != nil {
			pmgInst.MonitorQuarantine = *req.MonitorQuarantine
		}
		if req.MonitorDomainStats != nil {
			pmgInst.MonitorDomainStats = *req.MonitorDomainStats
		}
		if req.TemperatureMonitoringEnabled != nil {
			pmgInst.TemperatureMonitoringEnabled = req.TemperatureMonitoringEnabled
		}
	} else {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Save configuration to disk using our persistence instance
	if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save nodes configuration")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// IMPORTANT: Preserve alert overrides when updating nodes
	// This fixes issue #440 where PBS alert thresholds were being reset
	// Alert overrides are stored separately from node configuration
	// and must be explicitly preserved during node updates
	if h.getMonitor(r.Context()) != nil {
		// Load current alert configuration to preserve overrides
		alertConfig, err := h.getPersistence(r.Context()).LoadAlertConfig()
		if err == nil && alertConfig != nil {
			// For PBS nodes, we need to handle ID mapping
			// PBS monitoring uses "pbs-<name>" but config uses "pbs-<index>"
			// We need to preserve overrides by the monitoring ID
			if nodeType == "pbs" && index < len(h.getConfig(r.Context()).PBSInstances) {
				pbsName := h.getConfig(r.Context()).PBSInstances[index].Name
				monitoringID := "pbs-" + pbsName

				// Check if there are overrides for this PBS node
				if alertConfig.Overrides != nil {
					if _, exists := alertConfig.Overrides[monitoringID]; exists {
						log.Debug().
							Str("nodeID", nodeID).
							Str("monitoringID", monitoringID).
							Str("pbsName", pbsName).
							Msg("Preserving PBS alert overrides using monitoring ID")
					}
				}
			}

			// Apply the alert configuration to preserve all overrides
			h.getMonitor(r.Context()).GetAlertManager().UpdateConfig(*alertConfig)
			log.Debug().
				Str("nodeID", nodeID).
				Str("nodeType", nodeType).
				Msg("Preserved alert overrides after node update")
		}
	}

	// Reload monitor with new configuration
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor")
			http.Error(w, "Configuration saved but failed to apply changes", http.StatusInternalServerError)
			return
		}
	}

	// Trigger discovery refresh after adding node
	if h.getMonitor(r.Context()) != nil && h.getMonitor(r.Context()).GetDiscoveryService() != nil {
		log.Info().Msg("Triggering discovery refresh after adding node")
		h.getMonitor(r.Context()).GetDiscoveryService().ForceRefresh()

		// Broadcast discovery update via WebSocket
		if h.wsHub != nil {
			// Wait a moment for discovery to complete
			go func() {
				time.Sleep(2 * time.Second)
				result, _ := h.getMonitor(r.Context()).GetDiscoveryService().GetCachedResult()
				if result != nil {
					h.wsHub.BroadcastMessage(websocket.Message{
						Type: "discovery_update",
						Data: map[string]interface{}{
							"servers":   result.Servers,
							"errors":    result.Errors,
							"timestamp": time.Now().Unix(),
						},
						Timestamp: time.Now().Format(time.RFC3339),
					})
					log.Info().Msg("Broadcasted discovery update after adding node")
				}
			}()
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleDeleteNode deletes a node
func (h *ConfigHandlers) HandleDeleteNode(w http.ResponseWriter, r *http.Request) {
	log.Info().Msg("HandleDeleteNode called")

	// Prevent node modifications in mock mode
	if mock.IsMockEnabled() {
		http.Error(w, "Cannot modify nodes in mock mode", http.StatusForbidden)
		return
	}

	nodeID := strings.TrimPrefix(r.URL.Path, "/api/config/nodes/")
	if nodeID == "" {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}

	// Parse node ID
	parts := strings.Split(nodeID, "-")
	if len(parts) != 2 {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	nodeType := parts[0]
	index := 0
	if _, err := fmt.Sscanf(parts[1], "%d", &index); err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	log.Debug().
		Str("nodeID", nodeID).
		Str("nodeType", nodeType).
		Int("index", index).
		Int("pveCount", len(h.getConfig(r.Context()).PVEInstances)).
		Int("pbsCount", len(h.getConfig(r.Context()).PBSInstances)).
		Int("pmgCount", len(h.getConfig(r.Context()).PMGInstances)).
		Msg("Attempting to delete node")

	var deletedNodeHost string

	// Delete the node
	if nodeType == "pve" && index < len(h.getConfig(r.Context()).PVEInstances) {
		deletedNodeHost = h.getConfig(r.Context()).PVEInstances[index].Host
		log.Info().Str("nodeID", nodeID).Int("index", index).Msg("Deleting PVE node")
		h.getConfig(r.Context()).PVEInstances = append(h.getConfig(r.Context()).PVEInstances[:index], h.getConfig(r.Context()).PVEInstances[index+1:]...)
	} else if nodeType == "pbs" && index < len(h.getConfig(r.Context()).PBSInstances) {
		deletedNodeHost = h.getConfig(r.Context()).PBSInstances[index].Host
		log.Info().Str("nodeID", nodeID).Int("index", index).Msg("Deleting PBS node")
		h.getConfig(r.Context()).PBSInstances = append(h.getConfig(r.Context()).PBSInstances[:index], h.getConfig(r.Context()).PBSInstances[index+1:]...)
	} else if nodeType == "pmg" && index < len(h.getConfig(r.Context()).PMGInstances) {
		deletedNodeHost = h.getConfig(r.Context()).PMGInstances[index].Host
		log.Info().Str("nodeID", nodeID).Int("index", index).Msg("Deleting PMG node")
		h.getConfig(r.Context()).PMGInstances = append(h.getConfig(r.Context()).PMGInstances[:index], h.getConfig(r.Context()).PMGInstances[index+1:]...)
	} else {
		log.Warn().
			Str("nodeID", nodeID).
			Str("nodeType", nodeType).
			Int("index", index).
			Int("pveCount", len(h.getConfig(r.Context()).PVEInstances)).
			Int("pbsCount", len(h.getConfig(r.Context()).PBSInstances)).
			Int("pmgCount", len(h.getConfig(r.Context()).PMGInstances)).
			Msg("Node not found for deletion")
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Save configuration to disk using our persistence instance
	if err := h.getPersistence(r.Context()).SaveNodesConfigAllowEmpty(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save nodes configuration")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Immediately trigger discovery scan BEFORE reloading monitor
	// Capture node type for cleanup
	var deletedNodeType string = nodeType

	// deletedNodeHost already captured before removal when available

	// Reload monitor with new configuration
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor")
			http.Error(w, "Configuration saved but failed to apply changes", http.StatusInternalServerError)
			return
		}
	}

	// Broadcast node deletion to refresh the frontend
	if h.wsHub != nil {
		// Send a node_deleted message to trigger a refresh of the nodes list
		h.wsHub.BroadcastMessage(websocket.Message{
			Type: "node_deleted",
			Data: map[string]interface{}{
				"nodeType": nodeType,
			},
			Timestamp: time.Now().Format(time.RFC3339),
		})
		log.Info().Msg("Broadcasted node deletion event")

		// Trigger a full discovery scan in the background to update the discovery cache
		// This ensures the next time discovery modal is opened, it shows fresh results
		go func() {
			// Short delay to let the monitor stabilize
			time.Sleep(500 * time.Millisecond)

			// Trigger full discovery refresh
			if h.getMonitor(r.Context()) != nil && h.getMonitor(r.Context()).GetDiscoveryService() != nil {
				h.getMonitor(r.Context()).GetDiscoveryService().ForceRefresh()
				log.Info().Msg("Triggered background discovery refresh after node deletion")
			}
		}()
	}

	if deletedNodeType == "pve" && deletedNodeHost != "" {
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleRefreshClusterNodes re-detects cluster membership and updates endpoints
// This handles the case where nodes are added to a Proxmox cluster after initial configuration
func (h *ConfigHandlers) HandleRefreshClusterNodes(w http.ResponseWriter, r *http.Request) {
	// Prevent modifications in mock mode
	if mock.IsMockEnabled() {
		http.Error(w, "Cannot refresh cluster in mock mode", http.StatusForbidden)
		return
	}

	// Path format: /api/config/nodes/{id}/refresh-cluster
	path := strings.TrimPrefix(r.URL.Path, "/api/config/nodes/")
	path = strings.TrimSuffix(path, "/refresh-cluster")
	nodeID := path

	if nodeID == "" {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}

	// Parse node ID
	parts := strings.Split(nodeID, "-")
	if len(parts) != 2 {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	nodeType := parts[0]
	index := 0
	if _, err := fmt.Sscanf(parts[1], "%d", &index); err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	// Only PVE nodes can have clusters
	if nodeType != "pve" {
		http.Error(w, "Only PVE nodes can be cluster members", http.StatusBadRequest)
		return
	}

	if index >= len(h.getConfig(r.Context()).PVEInstances) {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	pve := &h.getConfig(r.Context()).PVEInstances[index]

	// Create client config for cluster detection
	clientConfig := config.CreateProxmoxConfig(pve)

	// Force cluster re-detection (ignore existing endpoints)
	isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, pve.Name, pve.ClusterEndpoints)

	if !isCluster {
		http.Error(w, "Node is not part of a cluster", http.StatusBadRequest)
		return
	}

	if len(clusterEndpoints) == 0 {
		http.Error(w, "Could not detect cluster nodes", http.StatusInternalServerError)
		return
	}

	oldEndpointCount := len(pve.ClusterEndpoints)
	newEndpointCount := len(clusterEndpoints)

	// Update cluster info
	pve.IsCluster = true
	if clusterName != "" && !strings.EqualFold(clusterName, "unknown cluster") {
		pve.ClusterName = clusterName
	}
	pve.ClusterEndpoints = clusterEndpoints

	// Save configuration
	if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save nodes configuration after cluster refresh")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("instance", pve.Name).
		Str("cluster", pve.ClusterName).
		Int("old_endpoints", oldEndpointCount).
		Int("new_endpoints", newEndpointCount).
		Msg("Refreshed cluster membership")

	// Reload monitor with new configuration
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor after cluster refresh")
			// Don't fail the request, config was saved successfully
		}
	}

	// Broadcast update to refresh frontend
	if h.wsHub != nil {
		h.wsHub.BroadcastMessage(websocket.Message{
			Type: "nodes_updated",
			Data: map[string]interface{}{
				"nodeType": "pve",
				"action":   "cluster_refresh",
			},
			Timestamp: time.Now().Format(time.RFC3339),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "success",
		"clusterName":  pve.ClusterName,
		"oldNodeCount": oldEndpointCount,
		"newNodeCount": newEndpointCount,
		"nodesAdded":   newEndpointCount - oldEndpointCount,
		"clusterNodes": clusterEndpoints,
	})
}

// HandleTestNodeConfig tests a node connection from provided configuration
func (h *ConfigHandlers) HandleTestNodeConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)

	var req NodeConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var testResult map[string]interface{}

	if req.Type == "pve" {
		// Create a temporary client to test connection
		authUser := req.User
		if req.Password != "" && req.TokenName == "" && req.TokenValue == "" {
			authUser = normalizePVEUser(authUser)
			req.User = authUser
		}
		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}
		clientConfig := proxmox.ClientConfig{
			Host:        req.Host,
			User:        authUser,
			Password:    req.Password,
			TokenName:   req.TokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   verifySSL,
			Fingerprint: req.Fingerprint,
		}
		client, err := proxmox.NewClient(clientConfig)
		if err != nil {
			testResult = map[string]interface{}{
				"status":  "error",
				"message": sanitizeErrorMessage(err, "create_client"),
			}
		} else {
			// Test connection by getting nodes list
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if nodes, err := client.GetNodes(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": sanitizeErrorMessage(err, "connection"),
				}
			} else {
				latency := time.Since(startTime).Milliseconds()
				testResult = map[string]interface{}{
					"status":  "success",
					"message": fmt.Sprintf("Connected to PVE cluster with %d nodes", len(nodes)),
					"latency": latency,
				}
			}
		}
	} else if req.Type == "pbs" {
		// Create a temporary client to test connection
		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}
		clientConfig := pbs.ClientConfig{
			Host:        req.Host,
			User:        req.User,
			Password:    req.Password,
			TokenName:   req.TokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   verifySSL,
			Fingerprint: req.Fingerprint,
		}
		client, err := pbs.NewClient(clientConfig)
		if err != nil {
			testResult = map[string]interface{}{
				"status":  "error",
				"message": sanitizeErrorMessage(err, "create_client"),
			}
		} else {
			// Test connection by getting datastores
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if _, err := client.GetDatastores(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": sanitizeErrorMessage(err, "connection"),
				}
			} else {
				latency := time.Since(startTime).Milliseconds()
				testResult = map[string]interface{}{
					"status":  "success",
					"message": "Connected to PBS instance",
					"latency": latency,
				}
			}
		}
	} else if req.Type == "pmg" {
		verifySSL := false
		if req.VerifySSL != nil {
			verifySSL = *req.VerifySSL
		}
		clientConfig := pmg.ClientConfig{
			Host:        req.Host,
			User:        req.User,
			Password:    req.Password,
			TokenName:   req.TokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   verifySSL,
			Fingerprint: req.Fingerprint,
		}
		client, err := pmg.NewClient(clientConfig)
		if err != nil {
			testResult = map[string]interface{}{
				"status":  "error",
				"message": sanitizeErrorMessage(err, "create_client"),
			}
		} else {
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if _, err := client.GetVersion(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": sanitizeErrorMessage(err, "connection"),
				}
			} else {
				latency := time.Since(startTime).Milliseconds()
				testResult = map[string]interface{}{
					"status":  "success",
					"message": "Connected to PMG instance",
					"latency": latency,
				}
			}
		}
	} else {
		http.Error(w, "Invalid node type", http.StatusBadRequest)
		return
	}

	// Return appropriate HTTP status based on test result
	w.Header().Set("Content-Type", "application/json")
	if testResult["status"] == "error" {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(testResult)
}

// HandleTestNode tests a node connection
func (h *ConfigHandlers) HandleTestNode(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/config/nodes/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "test" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	nodeID := parts[0]

	// Parse node ID
	idParts := strings.Split(nodeID, "-")
	if len(idParts) != 2 {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	nodeType := idParts[0]
	index := 0
	if _, err := fmt.Sscanf(idParts[1], "%d", &index); err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	// Find the node to test
	var testResult map[string]interface{}

	if nodeType == "pve" && index < len(h.getConfig(r.Context()).PVEInstances) {
		pve := h.getConfig(r.Context()).PVEInstances[index]

		// Create a temporary client to test connection
		authUser := pve.User
		if pve.TokenName == "" && pve.TokenValue == "" {
			authUser = normalizePVEUser(authUser)
		}
		clientConfig := proxmox.ClientConfig{
			Host:        pve.Host,
			User:        authUser,
			Password:    pve.Password,
			TokenName:   pve.TokenName,
			TokenValue:  pve.TokenValue,
			VerifySSL:   pve.VerifySSL,
			Fingerprint: pve.Fingerprint,
		}
		client, err := proxmox.NewClient(clientConfig)
		if err != nil {
			testResult = map[string]interface{}{
				"status":  "error",
				"message": sanitizeErrorMessage(err, "create_client"),
			}
		} else {
			// Test connection by getting nodes list
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if nodes, err := client.GetNodes(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": sanitizeErrorMessage(err, "connection"),
				}
			} else {
				latency := time.Since(startTime).Milliseconds()
				testResult = map[string]interface{}{
					"status":  "success",
					"message": fmt.Sprintf("Connected to PVE cluster with %d nodes", len(nodes)),
					"latency": latency,
				}
			}
		}
	} else if nodeType == "pbs" && index < len(h.getConfig(r.Context()).PBSInstances) {
		pbsInstance := h.getConfig(r.Context()).PBSInstances[index]

		// Create a temporary client to test connection
		clientConfig := pbs.ClientConfig{
			Host:        pbsInstance.Host,
			User:        pbsInstance.User,
			Password:    pbsInstance.Password,
			TokenName:   pbsInstance.TokenName,
			TokenValue:  pbsInstance.TokenValue,
			VerifySSL:   pbsInstance.VerifySSL,
			Fingerprint: pbsInstance.Fingerprint,
		}
		client, err := pbs.NewClient(clientConfig)
		if err != nil {
			testResult = map[string]interface{}{
				"status":  "error",
				"message": sanitizeErrorMessage(err, "create_client"),
			}
		} else {
			// Test connection by getting datastores
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if _, err := client.GetDatastores(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": sanitizeErrorMessage(err, "connection"),
				}
			} else {
				latency := time.Since(startTime).Milliseconds()
				testResult = map[string]interface{}{
					"status":  "success",
					"message": "Connected to PBS",
					"latency": latency,
				}
			}
		}
	} else if nodeType == "pmg" && index < len(h.getConfig(r.Context()).PMGInstances) {
		pmgInstance := h.getConfig(r.Context()).PMGInstances[index]

		clientConfig := config.CreatePMGConfig(&pmgInstance)
		if pmgInstance.Password != "" && pmgInstance.TokenName == "" && pmgInstance.TokenValue == "" {
			if clientConfig.User != "" && !strings.Contains(clientConfig.User, "@") {
				clientConfig.User = clientConfig.User + "@pmg"
			}
		}

		client, err := pmg.NewClient(clientConfig)
		if err != nil {
			testResult = map[string]interface{}{
				"status":  "error",
				"message": sanitizeErrorMessage(err, "create_client"),
			}
		} else {
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if version, err := client.GetVersion(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": sanitizeErrorMessage(err, "connection"),
				}
			} else {
				latency := time.Since(startTime).Milliseconds()
				versionLabel := ""
				if version != nil && strings.TrimSpace(version.Version) != "" {
					versionLabel = strings.TrimSpace(version.Version)
					if strings.TrimSpace(version.Release) != "" {
						versionLabel = versionLabel + "-" + strings.TrimSpace(version.Release)
					}
				}

				message := "Connected to PMG instance"
				if versionLabel != "" {
					message = fmt.Sprintf("Connected to PMG instance (version %s)", versionLabel)
				}

				testResult = map[string]interface{}{
					"status":  "success",
					"message": message,
					"latency": latency,
				}

				if version != nil {
					if version.Version != "" {
						testResult["version"] = strings.TrimSpace(version.Version)
					}
					if version.Release != "" {
						testResult["release"] = strings.TrimSpace(version.Release)
					}
				}
			}
		}
	} else {
		testResult = map[string]interface{}{
			"status":  "error",
			"message": "Node not found",
		}
	}

	// Return appropriate HTTP status based on test result
	w.Header().Set("Content-Type", "application/json")
	if testResult["status"] == "error" {
		w.WriteHeader(http.StatusBadRequest)
	}
	json.NewEncoder(w).Encode(testResult)
}

// getNodeStatus returns the connection status for a node
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
	// Load settings from persistence to get all fields including theme
	persistedSettings, err := h.getPersistence(r.Context()).LoadSystemSettings()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load persisted system settings")
		persistedSettings = config.DefaultSystemSettings()
	}
	if persistedSettings == nil {
		persistedSettings = config.DefaultSystemSettings()
	}

	// Get current values from running config
	settings := *persistedSettings
	settings.PVEPollingInterval = int(h.getConfig(r.Context()).PVEPollingInterval.Seconds())
	settings.PBSPollingInterval = int(h.getConfig(r.Context()).PBSPollingInterval.Seconds())
	settings.BackupPollingInterval = int(h.getConfig(r.Context()).BackupPollingInterval.Seconds())
	settings.FrontendPort = h.getConfig(r.Context()).FrontendPort
	settings.AllowedOrigins = h.getConfig(r.Context()).AllowedOrigins
	settings.ConnectionTimeout = int(h.getConfig(r.Context()).ConnectionTimeout.Seconds())
	settings.UpdateChannel = h.getConfig(r.Context()).UpdateChannel
	settings.AutoUpdateEnabled = h.getConfig(r.Context()).AutoUpdateEnabled
	settings.AutoUpdateCheckInterval = int(h.getConfig(r.Context()).AutoUpdateCheckInterval.Hours())
	settings.AutoUpdateTime = h.getConfig(r.Context()).AutoUpdateTime
	settings.LogLevel = h.getConfig(r.Context()).LogLevel
	settings.DiscoveryEnabled = h.getConfig(r.Context()).DiscoveryEnabled
	settings.DiscoverySubnet = h.getConfig(r.Context()).DiscoverySubnet
	settings.DiscoveryConfig = config.CloneDiscoveryConfig(h.getConfig(r.Context()).Discovery)
	backupEnabled := h.getConfig(r.Context()).EnableBackupPolling
	settings.BackupPollingEnabled = &backupEnabled

	// Create response structure that includes environment overrides
	response := struct {
		config.SystemSettings
		EnvOverrides map[string]bool `json:"envOverrides,omitempty"`
	}{
		SystemSettings: settings,
		EnvOverrides:   make(map[string]bool),
	}

	// Check for environment variable overrides
	if os.Getenv("PULSE_AUTH_HIDE_LOCAL_LOGIN") != "" {
		response.EnvOverrides["hideLocalLogin"] = true
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleVerifyTemperatureSSH tests SSH connectivity to nodes for temperature monitoring
func (h *ConfigHandlers) HandleVerifyTemperatureSSH(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	var req struct {
		Nodes string `json:"nodes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("⚠️  Unable to parse verification request"))
		return
	}

	// Parse node list
	nodeList := strings.Fields(req.Nodes)
	if len(nodeList) == 0 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("✓ No nodes to verify"))
		return
	}

	// Test SSH connectivity using temperature collector with the correct SSH key
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/pulse"
	}
	sshKeyPath := filepath.Join(homeDir, ".ssh/id_ed25519_sensors")
	tempCollector := monitoring.NewTemperatureCollectorWithPort("root", sshKeyPath, h.getConfig(r.Context()).SSHPort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	successNodes := []string{}
	failedNodes := []string{}

	for _, node := range nodeList {
		// Try to SSH and run sensors command
		temp, err := tempCollector.CollectTemperature(ctx, node, node)
		if err == nil && temp != nil && temp.Available {
			successNodes = append(successNodes, node)
		} else {
			failedNodes = append(failedNodes, node)
		}
	}

	// Build response message
	var response strings.Builder

	if len(successNodes) > 0 {
		response.WriteString("✓ SSH connectivity verified for:\n")
		for _, node := range successNodes {
			response.WriteString(fmt.Sprintf("  • %s\n", node))
		}
	}

	if len(failedNodes) > 0 {
		if len(successNodes) > 0 {
			response.WriteString("\n")
		}
		response.WriteString("ℹ️  Temperature monitoring will be available once SSH connectivity is configured.\n")
		response.WriteString("\n")
		response.WriteString("Nodes pending configuration:\n")
		for _, node := range failedNodes {
			response.WriteString(fmt.Sprintf("  • %s\n", node))
		}
		response.WriteString("\n")
		response.WriteString("See: https://github.com/rcourtman/Pulse/blob/main/SECURITY.md for detailed SSH configuration options.\n")
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response.String()))
}

// generateNodeID creates a unique ID for a node
func generateNodeID(nodeType string, index int) string {
	return fmt.Sprintf("%s-%d", nodeType, index)
}

// ExportConfigRequest represents a request to export configuration
type ExportConfigRequest struct {
	Passphrase string `json:"passphrase"`
}

// ImportConfigRequest represents a request to import configuration
type ImportConfigRequest struct {
	Data       string `json:"data"`
	Passphrase string `json:"passphrase"`
}

// HandleExportConfig exports all configuration with encryption
func (h *ConfigHandlers) HandleExportConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	// SECURITY: Validating scope for config export
	if !ensureScope(w, r, config.ScopeSettingsRead) {
		return
	}

	var req ExportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode export request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Passphrase == "" {
		log.Warn().Msg("Export rejected: passphrase is required")
		http.Error(w, "Passphrase is required", http.StatusBadRequest)
		return
	}

	// Require strong passphrase (at least 12 characters)
	if len(req.Passphrase) < 12 {
		log.Warn().Int("length", len(req.Passphrase)).Msg("Export rejected: passphrase too short (minimum 12 characters)")
		http.Error(w, "Passphrase must be at least 12 characters long", http.StatusBadRequest)
		return
	}

	// Export configuration
	exportedData, err := h.getPersistence(r.Context()).ExportConfig(req.Passphrase)
	if err != nil {
		log.Error().Err(err).Msg("Failed to export configuration")
		http.Error(w, "Failed to export configuration", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("Configuration exported successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data":   exportedData,
	})
}

// HandleImportConfig imports configuration from encrypted export
func (h *ConfigHandlers) HandleImportConfig(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 1MB to prevent memory exhaustion (config imports can be large)
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	// SECURITY: Validating scope for config import
	if !ensureScope(w, r, config.ScopeSettingsWrite) {
		return
	}

	var req ImportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode import request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Passphrase == "" {
		log.Warn().Msg("Import rejected: passphrase is required")
		http.Error(w, "Passphrase is required", http.StatusBadRequest)
		return
	}

	if req.Data == "" {
		log.Warn().Msg("Import rejected: encrypted data is required (ensure backup file has 'data' field)")
		http.Error(w, "Import data is required", http.StatusBadRequest)
		return
	}

	// Import configuration
	if err := h.getPersistence(r.Context()).ImportConfig(req.Data, req.Passphrase); err != nil {
		log.Error().Err(err).Msg("Failed to import configuration")
		http.Error(w, "Failed to import configuration: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Reload configuration from disk
	newConfig, err := config.Load()
	if err != nil {
		log.Error().Err(err).Msg("Failed to reload configuration after import")
		http.Error(w, "Configuration imported but failed to reload", http.StatusInternalServerError)
		return
	}

	// Update the config reference
	*h.getConfig(r.Context()) = *newConfig

	// Reload monitor with new configuration
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor after import")
			http.Error(w, "Configuration imported but failed to apply changes", http.StatusInternalServerError)
			return
		}
	}

	// Also reload alert and notification configs explicitly
	// (the monitor reload only reloads nodes unless it's a full reload)
	if h.getMonitor(r.Context()) != nil {
		// Reload alert configuration
		if alertConfig, err := h.getPersistence(r.Context()).LoadAlertConfig(); err == nil {
			h.getMonitor(r.Context()).GetAlertManager().UpdateConfig(*alertConfig)
			log.Info().Msg("Reloaded alert configuration after import")
		} else {
			log.Warn().Err(err).Msg("Failed to reload alert configuration after import")
		}

		// Reload webhook configuration
		if webhooks, err := h.getPersistence(r.Context()).LoadWebhooks(); err == nil {
			// Clear existing webhooks and add new ones
			notificationMgr := h.getMonitor(r.Context()).GetNotificationManager()
			// Get current webhooks to clear them
			for _, webhook := range notificationMgr.GetWebhooks() {
				if err := notificationMgr.DeleteWebhook(webhook.ID); err != nil {
					log.Warn().Err(err).Str("webhook", webhook.ID).Msg("Failed to delete existing webhook during reload")
				}
			}
			// Add imported webhooks
			for _, webhook := range webhooks {
				notificationMgr.AddWebhook(webhook)
			}
			log.Info().Int("count", len(webhooks)).Msg("Reloaded webhook configuration after import")
		} else {
			log.Warn().Err(err).Msg("Failed to reload webhook configuration after import")
		}

		// Reload email configuration
		if emailConfig, err := h.getPersistence(r.Context()).LoadEmailConfig(); err == nil {
			h.getMonitor(r.Context()).GetNotificationManager().SetEmailConfig(*emailConfig)
			log.Info().Msg("Reloaded email configuration after import")
		} else {
			log.Warn().Err(err).Msg("Failed to reload email configuration after import")
		}
	}

	// Reload guest metadata from disk
	if h.guestMetadataHandler != nil {
		if err := h.guestMetadataHandler.Reload(); err != nil {
			log.Warn().Err(err).Msg("Failed to reload guest metadata after import")
		} else {
			log.Info().Msg("Reloaded guest metadata after import")
		}
	}

	log.Info().Msg("Configuration imported successfully")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Configuration imported successfully",
	})
}

// HandleDiscoverServers handles network discovery of Proxmox/PBS servers
func (h *ConfigHandlers) HandleDiscoverServers(w http.ResponseWriter, r *http.Request) {
	// Support both GET (for cached results) and POST (for manual scan)
	switch r.Method {
	case http.MethodGet:
		// Return cached results from background discovery service
		if discoveryService := h.getMonitor(r.Context()).GetDiscoveryService(); discoveryService != nil {
			result, updated := discoveryService.GetCachedResult()

			var updatedUnix int64
			var ageSeconds float64
			if !updated.IsZero() {
				updatedUnix = updated.Unix()
				ageSeconds = time.Since(updated).Seconds()
			}

			response := map[string]interface{}{
				"servers":     result.Servers,
				"errors":      result.Errors,
				"environment": result.Environment,
				"cached":      true,
				"updated":     updatedUnix,
				"age":         ageSeconds,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"servers":     []interface{}{},
			"errors":      []string{},
			"environment": nil,
			"cached":      false,
			"updated":     int64(0),
			"age":         float64(0),
		})
		return

	case http.MethodPost:
		// Limit request body to 8KB to prevent memory exhaustion
		r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

		var req struct {
			Subnet   string `json:"subnet"`
			UseCache bool   `json:"use_cache"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.UseCache {
			if discoveryService := h.getMonitor(r.Context()).GetDiscoveryService(); discoveryService != nil {
				result, updated := discoveryService.GetCachedResult()

				var updatedUnix int64
				var ageSeconds float64
				if !updated.IsZero() {
					updatedUnix = updated.Unix()
					ageSeconds = time.Since(updated).Seconds()
				}

				response := map[string]interface{}{
					"servers":     result.Servers,
					"errors":      result.Errors,
					"environment": result.Environment,
					"cached":      true,
					"updated":     updatedUnix,
					"age":         ageSeconds,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		subnet := strings.TrimSpace(req.Subnet)
		if subnet == "" {
			subnet = "auto"
		}

		log.Info().Str("subnet", subnet).Msg("Starting manual discovery scan")

		scanner, buildErr := discoveryinternal.BuildScanner(h.getConfig(r.Context()).Discovery)
		if buildErr != nil {
			log.Warn().Err(buildErr).Msg("Falling back to default scanner for manual discovery")
			scanner = pkgdiscovery.NewScanner()
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := scanner.DiscoverServers(ctx, subnet)
		if err != nil {
			log.Error().Err(err).Msg("Discovery failed")
			http.Error(w, fmt.Sprintf("Discovery failed: %v", err), http.StatusInternalServerError)
			return
		}

		if result.Environment != nil {
			log.Info().
				Str("environment", result.Environment.Type).
				Float64("confidence", result.Environment.Confidence).
				Int("phases", len(result.Environment.Phases)).
				Msg("Manual discovery environment summary")
		}

		response := map[string]interface{}{
			"servers":     result.Servers,
			"errors":      result.Errors,
			"environment": result.Environment,
			"cached":      false,
			"scanning":    false,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleSetupScript serves the setup script for Proxmox/PBS nodes
func (h *ConfigHandlers) HandleSetupScript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	query := r.URL.Query()
	serverType := query.Get("type") // "pve" or "pbs"
	serverHost := strings.TrimSpace(query.Get("host"))
	pulseURL := strings.TrimSpace(query.Get("pulse_url"))   // URL of the Pulse server for auto-registration
	backupPerms := query.Get("backup_perms") == "true"      // Whether to add backup management permissions
	authToken := strings.TrimSpace(query.Get("auth_token")) // Temporary auth token for auto-registration

	if serverHost != "" {
		safeHost, err := sanitizeInstallerURL(serverHost)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid host parameter: %v", err), http.StatusBadRequest)
			return
		}
		serverHost = safeHost
	}

	if pulseURL != "" {
		safeURL, err := sanitizeInstallerURL(pulseURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid pulse_url parameter: %v", err), http.StatusBadRequest)
			return
		}
		pulseURL = safeURL
	}

	if sanitizedToken, err := sanitizeSetupAuthToken(authToken); err != nil {
		http.Error(w, fmt.Sprintf("Invalid auth_token parameter: %v", err), http.StatusBadRequest)
		return
	} else {
		authToken = sanitizedToken
	}

	// Validate required parameters
	if serverType == "" {
		http.Error(w, "Missing required parameter: type (must be 'pve' or 'pbs')", http.StatusBadRequest)
		return
	}

	// If host is not provided, try to use a sensible default
	if serverHost == "" {
		if serverType == "pve" {
			serverHost = "https://YOUR_PROXMOX_HOST:8006"
		} else {
			serverHost = "https://YOUR_PBS_HOST:8007"
		}
		log.Warn().
			Str("type", serverType).
			Msg("No host parameter provided, using placeholder. Auto-registration will fail.")
	}

	// If pulseURL is not provided, use the current request host
	if pulseURL == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		pulseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	} else {
		// Ensure derived pulseURL is still sanitized (should already be, but double check)
		if safeURL, err := sanitizeInstallerURL(pulseURL); err == nil {
			pulseURL = safeURL
		}
	}

	log.Info().
		Str("type", serverType).
		Str("host", serverHost).
		Bool("has_auth", h.getConfig(r.Context()).AuthUser != "" || h.getConfig(r.Context()).AuthPass != "" || h.getConfig(r.Context()).HasAPITokens()).
		Msg("HandleSetupScript called")

	// The setup script is now public - authentication happens via setup code
	// No need to check auth here since the script will prompt for a code

	// Default to PVE if not specified
	if serverType == "" {
		serverType = "pve"
	}

	// If pulse URL not provided, try to construct from request
	if pulseURL == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		pulseURL = fmt.Sprintf("%s://%s", scheme, r.Host)
	}

	// Extract hostname/IP from the host URL if provided
	serverName := "your-server"
	if serverHost != "" {
		// Extract hostname/IP from URL
		if match := strings.Contains(serverHost, "://"); match {
			parts := strings.Split(serverHost, "://")
			if len(parts) > 1 {
				hostPart := strings.Split(parts[1], ":")[0]
				serverName = hostPart
			}
		} else {
			// Just a hostname/IP
			serverName = strings.Split(serverHost, ":")[0]
		}
	}

	// Extract Pulse IP from the pulse URL to make token name unique
	pulseIP := "pulse"
	if pulseURL != "" {
		// Extract IP/hostname from Pulse URL
		if match := strings.Contains(pulseURL, "://"); match {
			parts := strings.Split(pulseURL, "://")
			if len(parts) > 1 {
				hostPart := strings.Split(parts[1], ":")[0]
				// Replace dots with dashes for token name compatibility
				pulseIP = strings.ReplaceAll(hostPart, ".", "-")
			}
		}
	}

	// Create unique token name based on Pulse IP and timestamp
	// Adding timestamp ensures truly unique tokens even when running from same Pulse server
	timestamp := time.Now().Unix()
	tokenName := fmt.Sprintf("pulse-%s-%d", pulseIP, timestamp)

	// Log the token name for debugging
	log.Info().
		Str("pulseURL", pulseURL).
		Str("pulseIP", pulseIP).
		Str("tokenName", tokenName).
		Int64("timestamp", timestamp).
		Msg("Generated unique token name for setup script")

	// Get or generate SSH public key for temperature monitoring
	sshKeys := h.getOrGenerateSSHKeys()

	var script string

	if serverType == "pve" {
		// Build storage permissions command if needed
		storagePerms := ""
		if backupPerms {
			storagePerms = "\npveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin"
		}

		script = fmt.Sprintf(`#!/bin/bash
# Pulse Monitoring Setup Script for %s
# Generated: %s

echo "============================================"
echo "  Pulse Monitoring Setup for Proxmox VE"
echo "============================================"
echo ""

PULSE_URL="%s"
SERVER_HOST="%s"
TOKEN_NAME="%s"
PULSE_TOKEN_ID="pulse-monitor@pam!${TOKEN_NAME}"
SETUP_SCRIPT_URL="$PULSE_URL/api/setup-script?type=pve&host=$SERVER_HOST&pulse_url=$PULSE_URL"

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
   echo "Please run this script as root"
   exit 1
fi

# Detect environment (Proxmox host vs LXC guest)
detect_environment() {
    if command -v pveum >/dev/null 2>&1 && command -v pveversion >/dev/null 2>&1; then
        echo "pve_host"
        return
    fi

    if [ -f /proc/1/cgroup ] && grep -qE '/(lxc|machine\.slice/machine-lxc)' /proc/1/cgroup 2>/dev/null; then
        echo "lxc_guest"
        return
    fi

    if command -v systemd-detect-virt >/dev/null 2>&1; then
        if systemd-detect-virt -q -c 2>/dev/null; then
            local virt_type
            virt_type=$(systemd-detect-virt -c 2>/dev/null | tr '[:upper:]' '[:lower:]')
            if echo "$virt_type" | grep -q "lxc"; then
                echo "lxc_guest"
                return
            fi
        fi
    fi

    echo "unknown"
}

detect_lxc_ctid() {
    local ctid=""
    if [ -f /proc/1/cgroup ]; then
        ctid=$(sed 's/\\x2d/-/g' /proc/1/cgroup 2>/dev/null | grep -Eo '(lxc|machine-lxc)-[0-9]+' | tail -n1 | grep -Eo '[0-9]+' | tail -n1)
        if [ -n "$ctid" ]; then
            echo "$ctid"
            return
        fi
    fi

    if command -v hostname >/dev/null 2>&1; then
        ctid=$(hostname 2>/dev/null)
        if echo "$ctid" | grep -qE '^[0-9]+$'; then
            echo "$ctid"
            return
        fi
    fi

    echo ""
}

ENVIRONMENT=$(detect_environment)

case "$ENVIRONMENT" in
    pve_host)
        echo "Detected Proxmox VE host environment."
        echo ""
        ;;
    lxc_guest)
        echo "Detected Proxmox LXC container environment."
        echo ""
        echo "Run this script on the Proxmox host:"
        echo "  curl -sSL \"$SETUP_SCRIPT_URL\" | bash"
        echo ""
        exit 1
        ;;
    *)
        echo "This script requires Proxmox host tooling (pveum)."
        echo ""
        echo "Run on your Proxmox host:"
        echo "  curl -sSL \"$SETUP_SCRIPT_URL\" | bash"
        echo ""
        echo "Manual setup steps:"
        echo "  1. On Proxmox host, create API token:"
        echo "       pveum user add pulse-monitor@pam --comment \"Pulse monitoring service\""
        echo "       pveum aclmod / -user pulse-monitor@pam -role PVEAuditor"
        echo "       pveum user token add pulse-monitor@pam "$TOKEN_NAME" --privsep 0"
        echo ""
        echo "  2. In Pulse: Settings → Nodes → Add Node (enter token from above)"
        echo ""
        exit 1
        ;;
esac

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Main Menu
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
echo ""
echo "What would you like to do?"
echo ""
echo "  [1] Install/Configure - Set up Pulse monitoring"
echo "  [2] Remove All        - Uninstall everything Pulse has configured"
echo "  [3] Cancel            - Exit without changes"
echo ""
echo -n "Your choice [1/2/3]: "

MAIN_ACTION=""
if [ -t 0 ]; then
    read -n 1 -r MAIN_ACTION
else
    if read -n 1 -r MAIN_ACTION </dev/tty 2>/dev/null; then
        :
    else
        echo "(No terminal available - defaulting to Install)"
        MAIN_ACTION="1"
    fi
fi
echo ""
echo ""

# Handle Cancel
if [[ $MAIN_ACTION =~ ^[3Cc]$ ]]; then
    echo "Cancelled. No changes made."
    exit 0
fi

# Handle Remove All
if [[ $MAIN_ACTION =~ ^[2Rr]$ ]]; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🗑️  Complete Removal"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "This will remove:"
    echo "  • SSH keys from authorized_keys (Pulse-managed entries)"
    echo "  • Pulse monitoring API tokens and user"
    echo ""
    echo "⚠️  WARNING: This is a destructive operation!"
    echo ""
    echo -n "Are you sure? [y/N]: "

    CONFIRM_REMOVE=""
    if [ -t 0 ]; then
        read -n 1 -r CONFIRM_REMOVE
    else
        if read -n 1 -r CONFIRM_REMOVE </dev/tty 2>/dev/null; then
            :
        else
            echo "(No terminal available - cancelling removal)"
            CONFIRM_REMOVE="n"
        fi
    fi
    echo ""
    echo ""

    if [[ ! $CONFIRM_REMOVE =~ ^[Yy]$ ]]; then
        echo "Removal cancelled. No changes made."
        exit 0
    fi

    echo "Removing Pulse monitoring components..."
    echo ""


    # Always run manual removal for local services and files
    if true; then
        # Remove SSH keys from authorized_keys (only Pulse-managed entries)
        if [ -f /root/.ssh/authorized_keys ]; then
            echo "  • Removing SSH keys from authorized_keys..."
            TMP_AUTH_KEYS=$(mktemp)
            if [ -f "$TMP_AUTH_KEYS" ]; then
                grep -vF '# pulse-managed-key' /root/.ssh/authorized_keys > "$TMP_AUTH_KEYS" 2>/dev/null
                GREP_EXIT=$?
                if [ $GREP_EXIT -eq 0 ] || [ $GREP_EXIT -eq 1 ]; then
                    chmod --reference=/root/.ssh/authorized_keys "$TMP_AUTH_KEYS" 2>/dev/null || chmod 600 "$TMP_AUTH_KEYS"
                    chown --reference=/root/.ssh/authorized_keys "$TMP_AUTH_KEYS" 2>/dev/null || true
                    if mv "$TMP_AUTH_KEYS" /root/.ssh/authorized_keys; then
                        :
                    else
                        rm -f "$TMP_AUTH_KEYS"
                    fi
                else
                    rm -f "$TMP_AUTH_KEYS"
                fi
            fi
        fi

        # Remove Pulse monitoring API tokens and user
        echo "  • Removing Pulse monitoring API tokens and user..."
        if command -v pveum &> /dev/null; then
            TOKEN_LIST=$(pveum user token list pulse-monitor@pam 2>/dev/null | awk 'NR>3 {print $2}' | grep -v '^$' || printf '')
            if [ -n "$TOKEN_LIST" ]; then
                while IFS= read -r TOKEN; do
                    if [ -n "$TOKEN" ]; then
                        pveum user token remove pulse-monitor@pam "$TOKEN" 2>/dev/null || true
                    fi
                done <<< "$TOKEN_LIST"
            fi
            pveum user delete pulse-monitor@pam 2>/dev/null || true
            pveum role delete PulseMonitor 2>/dev/null || true
        fi

        if command -v proxmox-backup-manager &> /dev/null; then
            proxmox-backup-manager user delete pulse-monitor@pbs 2>/dev/null || true
        fi
    fi

    echo ""
    echo "✓ Complete removal finished"
    echo ""
    echo "All Pulse monitoring components have been removed from this host."
    exit 0
fi

# If we get here, user chose Install (or default)
echo "Proceeding with installation..."
echo ""

# Extract Pulse server IP from the URL for token matching
PULSE_IP_PATTERN=$(echo "%s" | sed 's/\./\-/g')

# Check for old Pulse tokens from the same Pulse server and offer to clean them up
OLD_TOKENS=$(pveum user token list pulse-monitor@pam 2>/dev/null | grep -E "│ pulse-${PULSE_IP_PATTERN}-[0-9]+" | awk -F'│' '{print $2}' | sed 's/^ *//;s/ *$//' || true)
if [ ! -z "$OLD_TOKENS" ]; then
    echo "Checking for existing Pulse monitoring tokens from this Pulse server..."
    TOKEN_COUNT=$(echo "$OLD_TOKENS" | wc -l)
    echo ""
    echo "⚠️  Found $TOKEN_COUNT old Pulse monitoring token(s) from this Pulse server (${PULSE_IP_PATTERN}):"
    echo "$OLD_TOKENS" | sed 's/^/   - /'
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🗑️  CLEANUP OPTION"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Would you like to remove these old tokens? Type 'y' for yes, 'n' for no: "
    # Read from terminal, not from stdin (which is the piped script)
    if [ -t 0 ]; then
        # Running interactively
        read -p "> " -n 1 -r REPLY
    else
        # Being piped - try to read from terminal if available
        if read -p "> " -n 1 -r REPLY </dev/tty 2>/dev/null; then
            # Successfully read from terminal
            :
        else
            # No terminal available (e.g., in Docker without -t flag)
            echo "(No terminal available for input - keeping existing tokens)"
            REPLY="n"
        fi
    fi
    echo ""
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing old tokens..."
        while IFS= read -r TOKEN; do
            if [ ! -z "$TOKEN" ]; then
                pveum user token remove pulse-monitor@pam "$TOKEN" 2>/dev/null && echo "   ✓ Removed token: $TOKEN" || echo "   ✗ Failed to remove: $TOKEN"
            fi
        done <<< "$OLD_TOKENS"
        echo ""
    else
        echo "Keeping existing tokens."
    fi
    echo ""
fi

# Create monitoring user
echo "Creating monitoring user..."
pveum user add pulse-monitor@pam --comment "Pulse monitoring service" 2>/dev/null || true

SETUP_AUTH_TOKEN="%s"
AUTO_REG_SUCCESS=false

attempt_auto_registration() {

    if [ -z "$SETUP_AUTH_TOKEN" ] && [ -n "$PULSE_SETUP_TOKEN" ]; then
        SETUP_AUTH_TOKEN="$PULSE_SETUP_TOKEN"
    fi

    if [ -z "$SETUP_AUTH_TOKEN" ]; then
        if [ -t 0 ]; then
            printf "Pulse setup token: "
            if command -v stty >/dev/null 2>&1; then stty -echo; fi
            IFS= read -r SETUP_AUTH_TOKEN
            if command -v stty >/dev/null 2>&1; then stty echo; fi
            printf "\n"
		elif [ -c /dev/tty ] && [ -r /dev/tty ] && [ -w /dev/tty ]; then
			printf "Pulse setup token: " >/dev/tty
			if command -v stty >/dev/null 2>&1; then stty -echo </dev/tty 2>/dev/null || true; fi
			IFS= read -r SETUP_AUTH_TOKEN </dev/tty || true
			if command -v stty >/dev/null 2>&1; then stty echo </dev/tty 2>/dev/null || true; fi
			printf "\n" >/dev/tty
		fi
	fi

    if [ -z "$TOKEN_VALUE" ]; then
        echo "⚠️  Auto-registration skipped: token value unavailable"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
        return
    fi

    if [ -z "$SETUP_AUTH_TOKEN" ]; then
        echo "⚠️  Auto-registration skipped: no setup token provided"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
        return
    fi

    SERVER_HOSTNAME=$(hostname -s 2>/dev/null || hostname)
    SERVER_IP=$(hostname -I | awk '{print $1}')

    HOST_URL="$SERVER_HOST"
    if [ "$HOST_URL" = "https://YOUR_PROXMOX_HOST:8006" ] || [ -z "$HOST_URL" ]; then
        echo ""
        echo "❌ ERROR: No Proxmox host URL provided!"
        echo "   The setup script URL is missing the 'host' parameter."
        echo ""
        echo "   Please use the correct URL format:"
        echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pve&host=YOUR_PVE_URL&pulse_url=$PULSE_URL\" | bash"
        echo ""
        echo "   Example:"
        echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pve&host=https://192.168.0.5:8006&pulse_url=$PULSE_URL\" | bash"
        echo ""
        echo "📝 For manual setup, use the token created above with:"
        echo "   Token ID: $PULSE_TOKEN_ID"
        echo "   Token Value: [See above]"
        echo ""
        exit 1
    fi

    REGISTER_JSON='{"type":"pve","host":"'"$HOST_URL"'","serverName":"'"$SERVER_HOSTNAME"'","tokenId":"'"$PULSE_TOKEN_ID"'","tokenValue":"'"$TOKEN_VALUE"'","authToken":"'"$SETUP_AUTH_TOKEN"'"}'

    REGISTER_RESPONSE=$(echo "$REGISTER_JSON" | curl -s -X POST "$PULSE_URL/api/auto-register" \
        -H "Content-Type: application/json" \
        -d @- 2>&1)

    AUTO_REG_SUCCESS=false
    if echo "$REGISTER_RESPONSE" | grep -q "success"; then
        AUTO_REG_SUCCESS=true
        echo "Node registered successfully"
        echo ""
    else
        if echo "$REGISTER_RESPONSE" | grep -q "Authentication required"; then
            echo "Error: Auto-registration failed - authentication required"
            echo ""
            if [ -z "$PULSE_API_TOKEN" ]; then
                echo "To enable auto-registration, add your API token to the setup URL"
                echo "You can find your API token in Pulse Settings → Security"
            else
                echo "The provided API token was invalid"
            fi
        else
            echo "⚠️  Auto-registration failed. Manual configuration may be needed."
            echo "   Response: $REGISTER_RESPONSE"
        fi
        echo ""
        echo "📝 For manual setup:"
        echo "   1. Copy the token value shown above"
        echo "   2. Add this node manually in Pulse Settings"
    fi
}

# Generate API token
echo "Generating API token..."

# Check if token already exists
TOKEN_EXISTED=false
if pveum user token list pulse-monitor@pam 2>/dev/null | grep -q "$TOKEN_NAME"; then
    TOKEN_EXISTED=true
    echo ""
    echo "================================================================"
    echo "WARNING: Token '$TOKEN_NAME' already exists!"
    echo "================================================================"
    echo ""
    echo "To create a new token, first remove the existing one:"
    echo "  pveum user token remove pulse-monitor@pam $TOKEN_NAME"
    echo ""
    echo "Or create a token with a different name:"
    echo "  pveum user token add pulse-monitor@pam ${TOKEN_NAME}-$(date +%%s) --privsep 0"
    echo ""
    echo "Then use the new token ID in Pulse (e.g., ${PULSE_TOKEN_ID}-1234567890)"
    echo "================================================================"
    echo ""
else
    # Create token silently first
    TOKEN_OUTPUT=$(pveum user token add pulse-monitor@pam "$TOKEN_NAME" --privsep 0)
    
    # Extract the token value for auto-registration
    TOKEN_VALUE=$(echo "$TOKEN_OUTPUT" | grep "│ value" | awk -F'│' '{print $3}' | tr -d ' ' | tail -1)
    
    if [ -z "$TOKEN_VALUE" ]; then
        # If we can't extract the token, show it to the user
        echo ""
        echo "================================================================"
        echo "IMPORTANT: Copy the token value below - it's only shown once!"
        echo "================================================================"
        echo "$TOKEN_OUTPUT"
        echo "================================================================"
        echo ""
        echo "⚠️  Failed to extract token value from output."
        echo "   Manual registration may be required."
        echo ""
    else
        # Token created successfully
        echo "API token generated successfully"
        echo ""
    fi

fi

# Set up permissions
echo "Setting up permissions..."
pveum aclmod / -user pulse-monitor@pam -role PVEAuditor%s

# Detect Proxmox version and apply appropriate permissions
# Method 1: Try to check if VM.Monitor exists (reliable for PVE 8 and below)
HAS_VM_MONITOR=false
if pveum role list 2>/dev/null | grep -q "VM.Monitor" || 
   pveum role add TestMonitor -privs VM.Monitor 2>/dev/null; then
    HAS_VM_MONITOR=true
    pveum role delete TestMonitor 2>/dev/null || true
fi

# Detect availability of newer guest agent privileges (PVE 9+)
HAS_VM_GUEST_AGENT_AUDIT=false
if pveum role list 2>/dev/null | grep -q "VM.GuestAgent.Audit"; then
    HAS_VM_GUEST_AGENT_AUDIT=true
else
    if pveum role add TestGuestAgentAudit -privs VM.GuestAgent.Audit 2>/dev/null; then
        HAS_VM_GUEST_AGENT_AUDIT=true
        pveum role delete TestGuestAgentAudit 2>/dev/null || true
    fi
fi

# Detect availability of Sys.Audit (needed for Ceph metrics)
HAS_SYS_AUDIT=false
if pveum role list 2>/dev/null | grep -q "Sys.Audit"; then
    HAS_SYS_AUDIT=true
else
    if pveum role add TestSysAudit -privs Sys.Audit 2>/dev/null; then
        HAS_SYS_AUDIT=true
        pveum role delete TestSysAudit 2>/dev/null || true
    fi
fi

# Method 2: Try to detect PVE version directly
PVE_VERSION=""
if command -v pveversion >/dev/null 2>&1; then
    # Extract major version (e.g., "9" from "pve-manager/9.0.5/...")
    PVE_VERSION=$(pveversion --verbose 2>/dev/null | grep "pve-manager" | awk -F'/' '{print $2}' | cut -d'.' -f1)
fi

EXTRA_PRIVS=()

if [ "$HAS_SYS_AUDIT" = true ]; then
    EXTRA_PRIVS+=("Sys.Audit")
fi

if [ "$HAS_VM_MONITOR" = true ]; then
    # PVE 8 or below - VM.Monitor exists
    EXTRA_PRIVS+=("VM.Monitor")
elif [ "$HAS_VM_GUEST_AGENT_AUDIT" = true ]; then
    # PVE 9+ - VM.Monitor removed, prefer VM.GuestAgent.Audit for guest data
    EXTRA_PRIVS+=("VM.GuestAgent.Audit")
fi

	if [ ${#EXTRA_PRIVS[@]} -gt 0 ]; then
	    # Join as comma-separated list (pveum expects comma-separated privilege names).
	    PRIV_STRING="$(IFS=,; echo "${EXTRA_PRIVS[*]}")"

	    # Prefer modify (non-destructive) in case PulseMonitor already exists.
	    if pveum role modify PulseMonitor -privs "$PRIV_STRING" 2>/dev/null || pveum role add PulseMonitor -privs "$PRIV_STRING" 2>/dev/null; then
	        pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
	        echo "  • Applied privileges: $PRIV_STRING"
	    else
	        echo "  • Failed to configure PulseMonitor role with: $PRIV_STRING"
	        echo "    Assign these privileges manually if Pulse reports permission errors."
	    fi
	else
	    echo "  • No additional privileges detected. Pulse may show limited VM metrics."
	fi

attempt_auto_registration

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Temperature Monitoring Setup (Optional)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

SSH_SENSORS_PUBLIC_KEY="%s"
SSH_SENSORS_KEY_ENTRY="command="sensors -j",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $SSH_SENSORS_PUBLIC_KEY # pulse-sensors"
TEMPERATURE_ENABLED=false

if [ -n "$SSH_SENSORS_PUBLIC_KEY" ]; then
    echo "📊 Enable Temperature Monitoring?"
    echo ""
    echo "Collect CPU and drive temperatures via secure SSH connection."
    echo ""
    echo "Security:"
    echo "  • SSH key authentication with forced command (sensors -j only)"
    echo "  • No shell access, port forwarding, or other SSH features"
    echo "  • Keys stored in Pulse service user's home directory"
    echo ""
    echo "Enable temperature monitoring? [y/N]"
    echo -n "> "

    if [ -t 0 ]; then
        read -n 1 -r SSH_REPLY
    else
        # When stdin is not a terminal (e.g., curl | bash), try /dev/tty first, then stdin for piped input
        if read -n 1 -r SSH_REPLY </dev/tty 2>/dev/null; then
            :
        elif read -t 2 -n 1 -r SSH_REPLY 2>/dev/null && [ -n "$SSH_REPLY" ]; then
            echo "$SSH_REPLY"
        else
            echo "(No terminal available - skipping temperature monitoring)"
            SSH_REPLY="n"
        fi
    fi
    echo ""
    echo ""

    if [[ $SSH_REPLY =~ ^[Yy]$ ]]; then
        echo "Configuring temperature monitoring..."

        # Add key to root's authorized_keys
        mkdir -p /root/.ssh
        chmod 700 /root/.ssh

        # Remove any old pulse keys
        if [ -f /root/.ssh/authorized_keys ]; then
            grep -vF "# pulse-" /root/.ssh/authorized_keys > /root/.ssh/authorized_keys.tmp 2>/dev/null || touch /root/.ssh/authorized_keys.tmp
            mv /root/.ssh/authorized_keys.tmp /root/.ssh/authorized_keys
        fi

        echo "$SSH_SENSORS_KEY_ENTRY" >> /root/.ssh/authorized_keys
        chmod 600 /root/.ssh/authorized_keys
        echo "  ✓ Sensors key configured (restricted to sensors -j)"

        # Check if this is a Raspberry Pi
        IS_RPI=false
        if [ -f /proc/device-tree/model ] && grep -qi "raspberry pi" /proc/device-tree/model 2>/dev/null; then
            IS_RPI=true
        fi

        TEMPERATURE_SETUP_SUCCESS=false

        # Install lm-sensors if not present (skip on Raspberry Pi)
        if ! command -v sensors &> /dev/null; then
            if [ "$IS_RPI" = true ]; then
                echo "  ℹ️  Raspberry Pi detected - using native RPi temperature interface"
                echo "    Pulse will read temperature from /sys/class/thermal/thermal_zone0/temp"
                TEMPERATURE_SETUP_SUCCESS=true
            else
                echo "  ✓ Installing lm-sensors..."

                # Try to update and install, but provide helpful errors if it fails
                UPDATE_OUTPUT=$(apt-get update -qq 2>&1)
                if echo "$UPDATE_OUTPUT" | grep -q "Could not create temporary file\|/tmp"; then
                    echo ""
                    echo "    ⚠️  APT cannot write to /tmp directory"
                    echo "    This may be a permissions issue. To fix:"
                    echo "      sudo chown root:root /tmp"
                    echo "      sudo chmod 1777 /tmp"
                    echo ""
                    echo "    Attempting installation anyway..."
                elif echo "$UPDATE_OUTPUT" | grep -q "Failed to fetch\|GPG error\|no longer has a Release file"; then
                    echo "    ⚠️  Some repository errors detected, attempting installation anyway..."
                fi

                if apt-get install -y lm-sensors > /dev/null 2>&1; then
                    sensors-detect --auto > /dev/null 2>&1 || true
                    echo "    ✓ lm-sensors installed successfully"
                    TEMPERATURE_SETUP_SUCCESS=true
                else
                    echo ""
                    echo "    ⚠️  Could not install lm-sensors"
                    echo "    Possible causes:"
                    echo "      - Repository configuration errors"
                    echo "      - /tmp directory permission issues"
                    echo "      - Network connectivity problems"
                    echo ""
                    echo "    To fix manually:"
                    echo "      1. Check /tmp permissions: ls -ld /tmp"
                    echo "         (should be: drwxrwxrwt owned by root:root)"
                    echo "      2. Fix if needed: sudo chown root:root /tmp && sudo chmod 1777 /tmp"
                    echo "      3. Install: sudo apt-get update && sudo apt-get install -y lm-sensors"
                    echo ""
                fi
            fi
        else
            echo "  ✓ lm-sensors package verified"
            TEMPERATURE_SETUP_SUCCESS=true
        fi

        echo ""
        if [ "$TEMPERATURE_SETUP_SUCCESS" = true ]; then
            echo "✓ Temperature monitoring enabled"
            if [ "$IS_RPI" = true ]; then
                echo "  Using Raspberry Pi native temperature interface"
            fi
            echo "  Temperature data will appear in the dashboard within 10 seconds"
            TEMPERATURE_ENABLED=true
        else
            echo "⚠️  Temperature monitoring setup incomplete"
            echo "  You can re-run this script after installing lm-sensors"
        fi
    else
        echo "Skipping temperature monitoring."
    fi
else
    echo "Temperature monitoring keys are not available from Pulse."
fi

echo ""
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Setup Complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Node successfully registered with Pulse monitoring."
echo "Data will appear in your dashboard within 10 seconds."
echo ""

# Only show manual setup instructions if auto-registration failed
if [ "$AUTO_REG_SUCCESS" != true ]; then
    echo "Manual setup instructions:"
    echo "  Token ID: $PULSE_TOKEN_ID"
    if [ "$TOKEN_EXISTED" = true ]; then
        echo "  Token Value: [Use your existing token or create a new one as shown above]"
    elif [ -n "$TOKEN_VALUE" ]; then
        echo "  Token Value: $TOKEN_VALUE"
    else
        echo "  Token Value: [See token output above]"
    fi
    echo "  Host URL: YOUR_PROXMOX_HOST:8006"
echo ""
fi
`, serverName, time.Now().Format("2006-01-02 15:04:05"),
			pulseURL, serverHost, tokenName,
			pulseIP,
			authToken,
			storagePerms,
			sshKeys.SensorsPublicKey)

	} else { // PBS
		script = fmt.Sprintf(`#!/bin/bash
# Pulse Monitoring Setup Script for PBS %s
# Generated: %s

echo "============================================"
echo "  Pulse Monitoring Setup for PBS"
echo "============================================"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
   echo "Please run this script as root"
   exit 1
fi

# Check if proxmox-backup-manager command exists
if ! command -v proxmox-backup-manager &> /dev/null; then
   echo ""
   echo "❌ ERROR: 'proxmox-backup-manager' command not found!"
   echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
   echo ""
   echo "This script must be run on a Proxmox Backup Server."
   echo "The 'proxmox-backup-manager' command is required to create users and tokens."
   echo ""
   echo "If you're seeing this error, you might be:"
   echo "  • Running on a non-PBS system"
   echo "  • On a PVE server (use the PVE setup script instead)"
   echo "  • Missing PBS installation or in wrong environment"
   echo ""
   echo "If PBS is running in Docker, ensure you're inside the PBS container."
   echo ""
   exit 1
fi

# Extract Pulse server IP from the URL for token matching
PULSE_IP_PATTERN=$(echo "%s" | sed 's/\./\-/g')

# Check for old Pulse tokens from the same Pulse server and offer to clean them up
echo "Checking for existing Pulse monitoring tokens from this Pulse server..."
# PBS outputs tokens differently than PVE - extract just the token names matching this Pulse server
OLD_TOKENS=$(proxmox-backup-manager user list-tokens pulse-monitor@pbs 2>/dev/null | grep -oE "pulse-${PULSE_IP_PATTERN}-[0-9]+" | sort -u || true)
if [ ! -z "$OLD_TOKENS" ]; then
    TOKEN_COUNT=$(echo "$OLD_TOKENS" | wc -l)
    echo ""
    echo "⚠️  Found $TOKEN_COUNT old Pulse monitoring token(s) from this Pulse server (${PULSE_IP_PATTERN}):"
    echo "$OLD_TOKENS" | sed 's/^/   - /'
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🗑️  CLEANUP OPTION"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Would you like to remove these old tokens? Type 'y' for yes, 'n' for no: "
    # Read from terminal, not from stdin (which is the piped script)
    if [ -t 0 ]; then
        # Running interactively
        read -p "> " -n 1 -r REPLY
    else
        # Being piped - try to read from terminal if available
        if read -p "> " -n 1 -r REPLY </dev/tty 2>/dev/null; then
            # Successfully read from terminal
            :
        else
            # No terminal available (e.g., in Docker without -t flag)
            echo "(No terminal available for input - keeping existing tokens)"
            REPLY="n"
        fi
    fi
    echo ""
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Removing old tokens..."
        while IFS= read -r TOKEN; do
            if [ ! -z "$TOKEN" ]; then
                proxmox-backup-manager user delete-token pulse-monitor@pbs "$TOKEN" 2>/dev/null && echo "   ✓ Removed token: $TOKEN" || echo "   ✗ Failed to remove: $TOKEN"
            fi
        done <<< "$OLD_TOKENS"
        echo ""
    else
        echo "Keeping existing tokens."
    fi
    echo ""
fi

# Create monitoring user
echo "Creating monitoring user..."
proxmox-backup-manager user create pulse-monitor@pbs 2>/dev/null || echo "User already exists"

# Generate API token
echo "Generating API token..."

# Check if token already exists (PBS tokens can be regenerated with same name)
echo ""
echo "================================================================"
echo "IMPORTANT: Copy the token value below - it's only shown once!"
echo "================================================================"
TOKEN_OUTPUT=$(proxmox-backup-manager user generate-token pulse-monitor@pbs %s 2>&1)
if echo "$TOKEN_OUTPUT" | grep -q "already exists"; then
    echo "WARNING: Token '%s' already exists!"
    echo ""
    echo "You can either:"
    echo "1. Delete the existing token first:"
    echo "   proxmox-backup-manager user delete-token pulse-monitor@pbs %s"
    echo ""
    echo "2. Or create a token with a different name:"
    echo "   proxmox-backup-manager user generate-token pulse-monitor@pbs %s-$(date +%%s)"
    echo ""
    echo "Then use the new token ID in Pulse (e.g., pulse-monitor@pbs!%s-1234567890)"
else
    echo "$TOKEN_OUTPUT"
    
    # Extract the token value for auto-registration
    TOKEN_VALUE=$(echo "$TOKEN_OUTPUT" | grep '"value"' | sed 's/.*"value"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
    
    if [ -z "$TOKEN_VALUE" ]; then
        echo "⚠️  Failed to extract token value from output."
        echo "   Manual registration may be required."
        echo ""
    else
        echo "✅ Token created for Pulse monitoring"
        echo ""
    fi
    
    # Try auto-registration
    echo "🔄 Attempting auto-registration with Pulse..."
    echo ""
    
    # Use auth token from URL parameter when provided (automation workflows)
    AUTH_TOKEN="%s"

    # Allow non-interactive override via environment variable
    if [ -z "$AUTH_TOKEN" ] && [ -n "$PULSE_SETUP_TOKEN" ]; then
        AUTH_TOKEN="$PULSE_SETUP_TOKEN"
    fi

    # Prompt the operator if we still don't have a token and a TTY is available
    if [ -z "$AUTH_TOKEN" ]; then
        if [ -t 0 ]; then
            printf "Pulse setup token: "
            if command -v stty >/dev/null 2>&1; then stty -echo; fi
            IFS= read -r AUTH_TOKEN
            if command -v stty >/dev/null 2>&1; then stty echo; fi
            printf "\n"
		elif [ -c /dev/tty ] && [ -r /dev/tty ] && [ -w /dev/tty ]; then
			printf "Pulse setup token: " >/dev/tty
			if command -v stty >/dev/null 2>&1; then stty -echo </dev/tty 2>/dev/null || true; fi
			IFS= read -r AUTH_TOKEN </dev/tty || true
			if command -v stty >/dev/null 2>&1; then stty echo </dev/tty 2>/dev/null || true; fi
			printf "\n" >/dev/tty
		fi
	fi

    # Only proceed with auto-registration if we have an auth token
    if [ -n "$AUTH_TOKEN" ]; then
        # Get the server's hostname (short form to match Pulse node names)
        SERVER_HOSTNAME=$(hostname -s 2>/dev/null || hostname)
        SERVER_IP=$(hostname -I | awk '{print $1}')
        
        # Send registration to Pulse
        PULSE_URL="%s"
        
        # Check if host URL was provided
        HOST_URL="%s"
        if [ "$HOST_URL" = "https://YOUR_PBS_HOST:8007" ] || [ -z "$HOST_URL" ]; then
            echo ""
            echo "❌ ERROR: No PBS host URL provided!"
            echo "   The setup script URL is missing the 'host' parameter."
            echo ""
            echo "   Please use the correct URL format:"
            echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pbs&host=YOUR_PBS_URL&pulse_url=$PULSE_URL\" | bash"
            echo ""
            echo "   Example:"
            echo "   curl -sSL \"$PULSE_URL/api/setup-script?type=pbs&host=https://192.168.0.8:8007&pulse_url=$PULSE_URL\" | bash"
            echo ""
            echo "📝 For manual setup, use the token created above with:"
            echo "   Token ID: pulse-monitor@pbs!%s"
            echo "   Token Value: [See above]"
            echo ""
            exit 1
        fi
        
        # Construct registration request with setup code
        REGISTER_JSON=$(cat <<EOF
{
  "type": "pbs",
  "host": "$HOST_URL",
  "serverName": "$SERVER_HOSTNAME",
  "tokenId": "pulse-monitor@pbs!%s",
  "tokenValue": "$TOKEN_VALUE",
  "authToken": "$AUTH_TOKEN"
}
EOF
        )
        # Remove newlines from JSON
        REGISTER_JSON=$(echo "$REGISTER_JSON" | tr -d '\n')
        
        # Send registration with setup code
        REGISTER_RESPONSE=$(curl -s -X POST "$PULSE_URL/api/auto-register" \
            -H "Content-Type: application/json" \
            -d "$REGISTER_JSON" 2>&1)
    else
        echo "⚠️  Auto-registration skipped: no setup token provided"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
    fi
    
    AUTO_REG_SUCCESS=false
    if echo "$REGISTER_RESPONSE" | grep -q "success"; then
        AUTO_REG_SUCCESS=true
        echo "✅ Successfully registered with Pulse!"
    else
        if echo "$REGISTER_RESPONSE" | grep -q "Authentication required"; then
            echo "Error: Auto-registration failed - authentication required"
            echo ""
            if [ -z "$PULSE_API_TOKEN" ]; then
                echo "To enable auto-registration, add your API token to the setup URL"
                echo "You can find your API token in Pulse Settings → Security"
            else
                echo "The provided API token was invalid"
            fi
        else
            echo "⚠️  Auto-registration failed. Manual configuration may be needed."
            echo "   Response: $REGISTER_RESPONSE"
        fi
        echo ""
        echo "📝 For manual setup:"
        echo "   1. Copy the token value shown above"
        echo "   2. Add this node manually in Pulse Settings"
    fi
    echo ""
fi
echo "================================================================"
echo ""

# Set up permissions
echo "Setting up permissions..."
proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs
proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!%s'

echo ""
echo "✅ Setup complete!"
echo ""

# Only show manual setup instructions if auto-registration failed
if [ "$AUTO_REG_SUCCESS" != true ]; then
    echo "Add this server to Pulse with:"
    echo "  Token ID: pulse-monitor@pbs!%s"
    echo "  Token Value: [Check the output above for the token or instructions]"
    echo "  Host URL: https://$SERVER_IP:8007"
    echo ""
    echo "If auto-registration is enabled but requires a token:"
    echo "  1. Generate a registration token in Pulse Settings → Security"
    echo "  2. Re-run this script with: PULSE_REG_TOKEN=your-token ./setup.sh"
    echo ""
fi
`, serverName, time.Now().Format("2006-01-02 15:04:05"), pulseIP,
			tokenName, tokenName, tokenName, tokenName, tokenName,
			authToken, pulseURL, serverHost, tokenName, tokenName, tokenName, tokenName)
	}

	// Set headers for script download
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=pulse-setup-%s.sh", serverType))
	w.Write([]byte(script))
}

// generateSetupCode generates a secure hex token that satisfies sanitizeSetupAuthToken.
func (h *ConfigHandlers) generateSetupCode() string {
	// 16 bytes => 32 hex characters which matches the sanitizer's lower bound.
	const tokenBytes = 16
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}

	// rand.Read should never fail, but if it does fall back to timestamp-based token.
	log.Warn().Msg("fallback setup token generator used due to entropy failure")
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// HandleSetupScriptURL generates a one-time setup code and URL for the setup script
func (h *ConfigHandlers) HandleSetupScriptURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	// Parse request
	var req struct {
		Type        string `json:"type"`
		Host        string `json:"host"`
		BackupPerms bool   `json:"backupPerms"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Generate a temporary auth token (simpler than setup codes)
	token := h.generateSetupCode() // Reuse the generation function
	tokenHash := internalauth.HashAPIToken(token)

	// Store the token with expiry (5 minutes)
	expiry := time.Now().Add(5 * time.Minute)
	h.codeMutex.Lock()
	h.setupCodes[tokenHash] = &SetupCode{
		ExpiresAt: expiry,
		Used:      false,
		NodeType:  req.Type,
		Host:      req.Host,
		OrgID:     GetOrgID(r.Context()),
	}
	h.codeMutex.Unlock()

	log.Info().
		Str("token_hash", safePrefixForLog(tokenHash, 8)+"...").
		Time("expiry", expiry).
		Str("type", req.Type).
		Msg("Generated temporary auth token")

	// Build the URL with the token included
	host := r.Host

	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		if (parsedHost == "127.0.0.1" || parsedHost == "localhost") && parsedPort == strconv.Itoa(h.getConfig(r.Context()).FrontendPort) {
			// Prefer a user-configured public URL when we're running on loopback.
			if publicURL := strings.TrimSpace(h.getConfig(r.Context()).PublicURL); publicURL != "" {
				if parsedURL, err := url.Parse(publicURL); err == nil && parsedURL.Host != "" {
					host = parsedURL.Host
				}
			}
		}
	}

	// Detect protocol - check both TLS and proxy headers
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	pulseURL := fmt.Sprintf("%s://%s", scheme, host)

	encodedHost := ""
	if req.Host != "" {
		encodedHost = "&host=" + url.QueryEscape(req.Host)
	}

	backupPerms := ""
	if req.BackupPerms {
		backupPerms = "&backup_perms=true"
	}

	// Build script URL (setup token is passed via environment variable).
	scriptURL := fmt.Sprintf("%s/api/setup-script?type=%s%s&pulse_url=%s%s",
		pulseURL, req.Type, encodedHost, pulseURL, backupPerms)

	// Return a curl command; the setup token is passed via environment variable.
	// The setup token is returned separately so the script can prompt the user.
	tokenHint := token
	if len(token) > 6 {
		tokenHint = fmt.Sprintf("%s…%s", token[:3], token[len(token)-3:])
	}

	command := fmt.Sprintf(`curl -sSL "%s" | PULSE_SETUP_TOKEN=%s bash`, scriptURL, token)

	response := map[string]interface{}{
		"url":               scriptURL,
		"command":           command,
		"expires":           expiry.Unix(),
		"setupToken":        token,
		"tokenHint":         tokenHint,
		"commandWithEnv":    command,
		"commandWithoutEnv": fmt.Sprintf(`curl -sSL "%s" | bash`, scriptURL),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetMockMode returns the current mock mode state and configuration.
func (h *ConfigHandlers) HandleGetMockMode(w http.ResponseWriter, r *http.Request) {
	status := struct {
		Enabled bool            `json:"enabled"`
		Config  mock.MockConfig `json:"config"`
	}{
		Enabled: mock.IsMockEnabled(),
		Config:  mock.GetConfig(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error().Err(err).Msg("Failed to encode mock mode status")
	}
}

type mockModeRequest struct {
	Enabled *bool `json:"enabled"`
	Config  struct {
		NodeCount      *int     `json:"nodeCount"`
		VMsPerNode     *int     `json:"vmsPerNode"`
		LXCsPerNode    *int     `json:"lxcsPerNode"`
		RandomMetrics  *bool    `json:"randomMetrics"`
		HighLoadNodes  []string `json:"highLoadNodes"`
		StoppedPercent *float64 `json:"stoppedPercent"`
	} `json:"config"`
}

// HandleUpdateMockMode updates mock mode and optionally its configuration.
func (h *ConfigHandlers) HandleUpdateMockMode(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 16KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var req mockModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode mock mode request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update configuration first if provided.
	currentCfg := mock.GetConfig()
	if req.Config.NodeCount != nil {
		if *req.Config.NodeCount <= 0 {
			http.Error(w, "nodeCount must be greater than zero", http.StatusBadRequest)
			return
		}
		currentCfg.NodeCount = *req.Config.NodeCount
	}
	if req.Config.VMsPerNode != nil {
		if *req.Config.VMsPerNode < 0 {
			http.Error(w, "vmsPerNode cannot be negative", http.StatusBadRequest)
			return
		}
		currentCfg.VMsPerNode = *req.Config.VMsPerNode
	}
	if req.Config.LXCsPerNode != nil {
		if *req.Config.LXCsPerNode < 0 {
			http.Error(w, "lxcsPerNode cannot be negative", http.StatusBadRequest)
			return
		}
		currentCfg.LXCsPerNode = *req.Config.LXCsPerNode
	}
	if req.Config.RandomMetrics != nil {
		currentCfg.RandomMetrics = *req.Config.RandomMetrics
	}
	if req.Config.HighLoadNodes != nil {
		currentCfg.HighLoadNodes = req.Config.HighLoadNodes
	}
	if req.Config.StoppedPercent != nil {
		if *req.Config.StoppedPercent < 0 || *req.Config.StoppedPercent > 1 {
			http.Error(w, "stoppedPercent must be between 0 and 1", http.StatusBadRequest)
			return
		}
		currentCfg.StoppedPercent = *req.Config.StoppedPercent
	}

	mock.SetMockConfig(currentCfg)

	if req.Enabled != nil {
		if h.getMonitor(r.Context()) != nil {
			h.getMonitor(r.Context()).SetMockMode(*req.Enabled)
		} else {
			mock.SetEnabled(*req.Enabled)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	status := struct {
		Enabled bool            `json:"enabled"`
		Config  mock.MockConfig `json:"config"`
	}{
		Enabled: mock.IsMockEnabled(),
		Config:  mock.GetConfig(),
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error().Err(err).Msg("Failed to encode mock mode response")
	}
}

// AutoRegisterRequest represents a request from the setup script or agent to auto-register a node
type AutoRegisterRequest struct {
	Type       string `json:"type"`                 // "pve" or "pbs"
	Host       string `json:"host"`                 // The host URL
	TokenID    string `json:"tokenId"`              // Full token ID like pulse-monitor@pam!pulse-token
	TokenValue string `json:"tokenValue,omitempty"` // The token value for the node
	ServerName string `json:"serverName"`           // Hostname or IP
	SetupCode  string `json:"setupCode,omitempty"`  // One-time setup code for authentication (deprecated)
	AuthToken  string `json:"authToken,omitempty"`  // Direct auth token from URL (new approach)
	Source     string `json:"source,omitempty"`     // "agent" or "script" - indicates how the node was registered
	// New secure fields
	RequestToken bool   `json:"requestToken,omitempty"` // If true, Pulse will generate and return a token
	Username     string `json:"username,omitempty"`     // Username for creating token (e.g., "root@pam")
	Password     string `json:"password,omitempty"`     // Password for authentication (never stored)
}

// HandleAutoRegister receives token details from the setup script and auto-configures the node
func (h *ConfigHandlers) HandleAutoRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body first to get the setup code
	var req AutoRegisterRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(body, &req); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to parse auto-register request")
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Check authentication - require either setup code or API token if auth is enabled
	authenticated := false

	// Support both setupCode (old) and authToken (new) fields
	authCode := req.SetupCode
	if req.AuthToken != "" {
		authCode = req.AuthToken
	}

	log.Debug().
		Bool("hasAuthToken", strings.TrimSpace(req.AuthToken) != "").
		Bool("hasSetupCode", strings.TrimSpace(authCode) != "").
		Bool("hasConfigToken", h.getConfig(r.Context()).HasAPITokens()).
		Msg("Checking authentication for auto-register")

	// First check for setup code/auth token in the request
	if authCode != "" {
		matchedAPIToken := false
		if h.getConfig(r.Context()).HasAPITokens() {
			if record, ok := h.getConfig(r.Context()).ValidateAPIToken(authCode); ok {
				// Accept settings:write (admin tokens) or host-agent:report (agent tokens)
				if record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeHostReport) {
					authenticated = true
					matchedAPIToken = true
					log.Info().
						Str("type", req.Type).
						Str("host", req.Host).
						Msg("Auto-register authenticated via direct API token")
				} else {
					log.Warn().
						Str("type", req.Type).
						Str("host", req.Host).
						Msg("Auto-register rejected: API token missing required scope")
				}
			}
		}

		if !matchedAPIToken {
			// Not the API token, check if it's a temporary setup code
			codeHash := internalauth.HashAPIToken(authCode)
			log.Debug().
				Bool("hasAuthCode", true).
				Str("codeHash", safePrefixForLog(codeHash, 8)+"...").
				Msg("Checking auth token as setup code")
			h.codeMutex.Lock()
			setupCode, exists := h.setupCodes[codeHash]
			log.Debug().
				Bool("exists", exists).
				Int("totalCodes", len(h.setupCodes)).
				Msg("Setup code lookup result")
			if exists && !setupCode.Used && time.Now().Before(setupCode.ExpiresAt) {
				// Validate that the code matches the node type
				// Note: We don't validate the host anymore as it may differ between
				// what's entered in the UI and what's provided in the setup script URL
				if setupCode.NodeType == req.Type {
					setupCode.Used = true // Mark as used immediately

					// Inject OrgID from setup code into context for subsequent processing
					if setupCode.OrgID != "" {
						ctx := context.WithValue(r.Context(), OrgIDContextKey, setupCode.OrgID)
						r = r.WithContext(ctx)
					}
					// Allow a short grace period for follow-up actions without keeping tokens alive too long
					graceExpiry := time.Now().Add(1 * time.Minute)
					if setupCode.ExpiresAt.Before(graceExpiry) {
						graceExpiry = setupCode.ExpiresAt
					}
					h.recentSetupTokens[codeHash] = graceExpiry
					authenticated = true
					log.Info().
						Str("type", req.Type).
						Str("host", req.Host).
						Bool("via_authToken", req.AuthToken != "").
						Msg("Auto-register authenticated via setup code/token")
				} else {
					log.Warn().
						Str("expected_type", setupCode.NodeType).
						Str("got_type", req.Type).
						Msg("Setup code validation failed - type mismatch")
				}
			} else if exists && setupCode.Used {
				log.Warn().Msg("Setup code already used")
			} else if exists {
				log.Warn().Msg("Setup code expired")
			} else {
				log.Warn().Msg("Invalid setup code/token - not in setup codes map")
			}
			h.codeMutex.Unlock()
		}
	}

	// If not authenticated via setup code, check API token if configured
	if !authenticated && h.getConfig(r.Context()).HasAPITokens() {
		apiToken := r.Header.Get("X-API-Token")
		if record, ok := h.getConfig(r.Context()).ValidateAPIToken(apiToken); ok {
			// Accept settings:write (admin tokens) or host-agent:report (agent tokens)
			if record.HasScope(config.ScopeSettingsWrite) || record.HasScope(config.ScopeHostReport) {
				authenticated = true
				log.Info().Msg("Auto-register authenticated via API token")
			} else {
				log.Warn().Msg("Auto-register rejected: API token missing required scope")
			}
		}
	}

	// Abort when no authentication succeeded. This applies even when API tokens
	// are not configured to ensure one-time setup tokens are always required.
	if !authenticated {
		log.Warn().
			Str("ip", r.RemoteAddr).
			Bool("has_auth_code", authCode != "").
			Msg("Unauthorized auto-register attempt rejected")

		if authCode == "" && r.Header.Get("X-API-Token") == "" {
			http.Error(w, "Pulse requires authentication", http.StatusUnauthorized)
		} else {
			http.Error(w, "Invalid or expired setup code", http.StatusUnauthorized)
		}
		return
	}

	// Log source IP for security auditing
	clientIP := r.RemoteAddr
	// Only trust X-Forwarded-For if request comes from a trusted proxy
	peerIP := extractRemoteIP(clientIP)
	if isTrustedProxyIP(peerIP) {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = forwarded
		}
	}
	log.Info().Str("clientIP", clientIP).Msg("Auto-register request from")

	// Registration token validation removed - feature deprecated

	log.Info().
		Str("type", req.Type).
		Str("host", req.Host).
		Str("tokenId", req.TokenID).
		Bool("hasTokenValue", req.TokenValue != "").
		Str("serverName", req.ServerName).
		Msg("Processing auto-register request")

	// Check if this is a new secure registration request
	if req.RequestToken {
		// New secure mode - generate token on Pulse side
		if req.Type == "" || req.Host == "" || req.Username == "" || req.Password == "" {
			log.Error().
				Str("type", req.Type).
				Str("host", req.Host).
				Bool("hasUsername", req.Username != "").
				Bool("hasPassword", req.Password != "").
				Msg("Missing required fields for secure registration")
			http.Error(w, "Missing required fields", http.StatusBadRequest)
			return
		}
		// Handle secure registration
		h.handleSecureAutoRegister(w, r, &req, clientIP)
		return
	}

	// Legacy mode - validate old required fields
	if req.Type == "" || req.Host == "" || req.TokenID == "" || req.TokenValue == "" {
		log.Error().
			Str("type", req.Type).
			Str("host", req.Host).
			Str("tokenId", req.TokenID).
			Bool("hasToken", req.TokenValue != "").
			Msg("Missing required fields")
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	host, err := normalizeNodeHost(req.Host, req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fingerprint := ""
	if fp, err := tlsutil.FetchFingerprint(host); err != nil {
		log.Warn().Err(err).Str("host", host).Msg("Failed to fetch TLS fingerprint for auto-register")
	} else {
		fingerprint = fp
	}

	// Create a node configuration
	boolFalse := false
	boolTrue := true
	verifySSL := true
	nodeConfig := NodeConfigRequest{
		Type:               req.Type,
		Name:               req.ServerName,
		Host:               host, // Use normalized host
		TokenName:          req.TokenID,
		TokenValue:         req.TokenValue,
		Fingerprint:        fingerprint,
		VerifySSL:          &verifySSL,
		MonitorVMs:         &boolTrue,
		MonitorContainers:  &boolTrue,
		MonitorStorage:     &boolTrue,
		MonitorBackups:     &boolTrue,
		MonitorDatastores:  &boolTrue,
		MonitorSyncJobs:    &boolTrue,
		MonitorVerifyJobs:  &boolTrue,
		MonitorPruneJobs:   &boolTrue,
		MonitorGarbageJobs: &boolFalse,
	}

	// Check if a node with this host already exists
	// IMPORTANT: Match by Host URL primarily.
	// Also match by name+tokenID for DHCP scenarios where IP changed but it's the same host.
	// Different physical hosts can have the same hostname (e.g., "px1" on different networks)
	// but they'll have different tokens, so we only merge if BOTH name AND token match.
	// See: Issue #891, #104, #924, #940 and multiple fix attempts in Dec 2025.
	existingIndex := -1
	preserveHost := false // When true, keep user's configured hostname instead of overwriting with agent's IP

	// Extract IP from the new host URL for DNS comparison
	newHostIP := extractHostIP(host)

	if req.Type == "pve" {
		for i, node := range h.getConfig(r.Context()).PVEInstances {
			if node.Host == host {
				existingIndex = i
				break
			}
			// DHCP case: same hostname AND same token = same physical host with new IP
			// This allows IP changes to update existing nodes without creating duplicates
			if req.ServerName != "" && strings.EqualFold(node.Name, req.ServerName) && node.TokenName == req.TokenID {
				existingIndex = i
				// Update the host to the new IP
				log.Info().
					Str("oldHost", node.Host).
					Str("newHost", host).
					Str("node", req.ServerName).
					Msg("Detected IP change for existing node - updating host")
				break
			}
			// Agent registration: check if existing hostname resolves to the new IP
			// This catches the case where a node was manually added by hostname and
			// then the agent registers using the IP address. (Issue #924)
			// We preserve the user's configured hostname instead of overwriting with IP. (Issue #940)
			if req.Source == "agent" && newHostIP != "" {
				existingHostIP := extractHostIP(node.Host)
				if existingHostIP == "" {
					// Existing config uses hostname, try to resolve it
					existingHostIP = resolveHostnameToIP(node.Host)
				}
				if existingHostIP == newHostIP {
					existingIndex = i
					preserveHost = true // Keep user's configured hostname
					log.Info().
						Str("existingHost", node.Host).
						Str("newHost", host).
						Str("resolvedIP", newHostIP).
						Str("node", node.Name).
						Msg("Agent registration detected existing node by IP resolution - preserving configured hostname")
					break
				}
			}
		}
	} else {
		for i, node := range h.getConfig(r.Context()).PBSInstances {
			if node.Host == host {
				existingIndex = i
				break
			}
			// DHCP case: same hostname AND same token = same physical host with new IP
			if req.ServerName != "" && strings.EqualFold(node.Name, req.ServerName) && node.TokenName == req.TokenID {
				existingIndex = i
				log.Info().
					Str("oldHost", node.Host).
					Str("newHost", host).
					Str("node", req.ServerName).
					Msg("Detected IP change for existing node - updating host")
				break
			}
			// Agent registration: check if existing hostname resolves to the new IP
			// We preserve the user's configured hostname instead of overwriting with IP. (Issue #940)
			if req.Source == "agent" && newHostIP != "" {
				existingHostIP := extractHostIP(node.Host)
				if existingHostIP == "" {
					existingHostIP = resolveHostnameToIP(node.Host)
				}
				if existingHostIP == newHostIP {
					existingIndex = i
					preserveHost = true // Keep user's configured hostname
					log.Info().
						Str("existingHost", node.Host).
						Str("newHost", host).
						Str("resolvedIP", newHostIP).
						Str("node", node.Name).
						Msg("Agent registration detected existing node by IP resolution - preserving configured hostname")
					break
				}
			}
		}
	}

	// If node exists, update it; otherwise add new
	if existingIndex >= 0 {
		// Update existing node
		if req.Type == "pve" {
			instance := &h.getConfig(r.Context()).PVEInstances[existingIndex]
			// Update host in case IP changed (DHCP scenario)
			// But preserve user's configured hostname when matched by IP resolution (Issue #940)
			if !preserveHost {
				instance.Host = host
			}
			// Clear password auth when switching to token auth
			instance.User = ""
			instance.Password = ""
			instance.TokenName = nodeConfig.TokenName
			instance.TokenValue = nodeConfig.TokenValue
			// Update source if provided (allows upgrade from script to agent)
			if req.Source != "" {
				instance.Source = req.Source
			}

			// Check for cluster if not already detected
			if !instance.IsCluster {
				clientConfig := proxmox.ClientConfig{
					Host:       instance.Host,
					TokenName:  nodeConfig.TokenName,
					TokenValue: nodeConfig.TokenValue,
					VerifySSL:  instance.VerifySSL,
				}

				isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, instance.Name, instance.ClusterEndpoints)
				if isCluster {
					instance.IsCluster = true
					instance.ClusterName = clusterName
					instance.ClusterEndpoints = clusterEndpoints
					log.Info().
						Str("cluster", clusterName).
						Int("endpoints", len(clusterEndpoints)).
						Msg("Detected Proxmox cluster during auto-registration update")
				}
			}
			// Keep other settings as they were
		} else {
			instance := &h.getConfig(r.Context()).PBSInstances[existingIndex]
			// Update host in case IP changed (DHCP scenario)
			// But preserve user's configured hostname when matched by IP resolution (Issue #940)
			if !preserveHost {
				instance.Host = host
			}
			// Clear password auth when switching to token auth
			instance.User = ""
			instance.Password = ""
			instance.TokenName = nodeConfig.TokenName
			instance.TokenValue = nodeConfig.TokenValue
			// Update source if provided (allows upgrade from script to agent)
			if req.Source != "" {
				instance.Source = req.Source
			}
			// Keep other settings as they were
		}
		log.Info().
			Str("host", req.Host).
			Str("type", req.Type).
			Str("tokenName", nodeConfig.TokenName).
			Bool("hasTokenValue", nodeConfig.TokenValue != "").
			Msg("Updated existing node with new token")
	} else {
		// Add new node
		if req.Type == "pve" {
			// Check for cluster detection using helper
			verifySSL := false
			if nodeConfig.VerifySSL != nil {
				verifySSL = *nodeConfig.VerifySSL
			}
			clientConfig := proxmox.ClientConfig{
				Host:       nodeConfig.Host,
				TokenName:  nodeConfig.TokenName,
				TokenValue: nodeConfig.TokenValue,
				VerifySSL:  verifySSL,
			}

			isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, nodeConfig.Name, nil)

			// CLUSTER DEDUPLICATION: Check if we already have this cluster configured
			// If so, merge this node as an endpoint instead of creating a duplicate instance
			if isCluster && clusterName != "" {
				for i := range h.getConfig(r.Context()).PVEInstances {
					existingInstance := &h.getConfig(r.Context()).PVEInstances[i]
					if existingInstance.IsCluster && existingInstance.ClusterName == clusterName {
						// Found existing cluster with same name - merge endpoints!
						log.Info().
							Str("cluster", clusterName).
							Str("existingInstance", existingInstance.Name).
							Str("newNode", nodeConfig.Name).
							Msg("Auto-registered node belongs to already-configured cluster - merging endpoints")

						// Merge any new endpoints from the detected cluster
						existingEndpointMap := make(map[string]bool)
						for _, ep := range existingInstance.ClusterEndpoints {
							existingEndpointMap[ep.NodeName] = true
						}
						for _, newEp := range clusterEndpoints {
							if !existingEndpointMap[newEp.NodeName] {
								existingInstance.ClusterEndpoints = append(existingInstance.ClusterEndpoints, newEp)
								log.Info().
									Str("cluster", clusterName).
									Str("endpoint", newEp.NodeName).
									Msg("Added new endpoint to existing cluster via auto-registration")
							}
						}

						// Save and reload
						if h.getPersistence(r.Context()) != nil {
							if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
								log.Warn().Err(err).Msg("Failed to persist cluster endpoint merge during auto-registration")
							}
						}
						if h.reloadFunc != nil {
							if err := h.reloadFunc(); err != nil {
								log.Warn().Err(err).Msg("Failed to reload monitor after cluster merge during auto-registration")
							}
						}

						// Return success - merged into existing cluster
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(map[string]interface{}{
							"success":        true,
							"merged":         true,
							"cluster":        clusterName,
							"existingNode":   existingInstance.Name,
							"message":        fmt.Sprintf("Agent merged into existing cluster '%s'", clusterName),
							"totalEndpoints": len(existingInstance.ClusterEndpoints),
						})
						return
					}
				}
			}

			monitorVMs := true
			if nodeConfig.MonitorVMs != nil {
				monitorVMs = *nodeConfig.MonitorVMs
			}
			monitorContainers := true
			if nodeConfig.MonitorContainers != nil {
				monitorContainers = *nodeConfig.MonitorContainers
			}
			monitorStorage := true
			if nodeConfig.MonitorStorage != nil {
				monitorStorage = *nodeConfig.MonitorStorage
			}
			monitorBackups := true
			if nodeConfig.MonitorBackups != nil {
				monitorBackups = *nodeConfig.MonitorBackups
			}

			// Disambiguate node name if duplicate hostnames exist
			displayName := h.disambiguateNodeName(r.Context(), nodeConfig.Name, nodeConfig.Host, "pve")

			newInstance := config.PVEInstance{
				Name:              displayName,
				Host:              nodeConfig.Host,
				TokenName:         nodeConfig.TokenName,
				TokenValue:        nodeConfig.TokenValue,
				VerifySSL:         verifySSL,
				MonitorVMs:        monitorVMs,
				MonitorContainers: monitorContainers,
				MonitorStorage:    monitorStorage,
				MonitorBackups:    monitorBackups,
				IsCluster:         isCluster,
				ClusterName:       clusterName,
				ClusterEndpoints:  clusterEndpoints,
				Source:            req.Source, // Track how this node was registered
			}
			h.getConfig(r.Context()).PVEInstances = append(h.getConfig(r.Context()).PVEInstances, newInstance)

			if isCluster {
				log.Info().
					Str("cluster", clusterName).
					Int("endpoints", len(clusterEndpoints)).
					Msg("Added new Proxmox cluster via auto-registration")
			}
		} else {
			verifySSL := false
			if nodeConfig.VerifySSL != nil {
				verifySSL = *nodeConfig.VerifySSL
			}
			monitorDatastores := false
			if nodeConfig.MonitorDatastores != nil {
				monitorDatastores = *nodeConfig.MonitorDatastores
			}
			monitorSyncJobs := false
			if nodeConfig.MonitorSyncJobs != nil {
				monitorSyncJobs = *nodeConfig.MonitorSyncJobs
			}
			monitorVerifyJobs := false
			if nodeConfig.MonitorVerifyJobs != nil {
				monitorVerifyJobs = *nodeConfig.MonitorVerifyJobs
			}
			monitorPruneJobs := false
			if nodeConfig.MonitorPruneJobs != nil {
				monitorPruneJobs = *nodeConfig.MonitorPruneJobs
			}
			monitorGarbageJobs := false
			if nodeConfig.MonitorGarbageJobs != nil {
				monitorGarbageJobs = *nodeConfig.MonitorGarbageJobs
			}

			// Disambiguate node name if duplicate hostnames exist
			pbsDisplayName := h.disambiguateNodeName(r.Context(), nodeConfig.Name, nodeConfig.Host, "pbs")

			newInstance := config.PBSInstance{
				Name:               pbsDisplayName,
				Host:               nodeConfig.Host,
				TokenName:          nodeConfig.TokenName,
				TokenValue:         nodeConfig.TokenValue,
				VerifySSL:          verifySSL,
				MonitorBackups:     true, // Enable by default for PBS
				MonitorDatastores:  monitorDatastores,
				MonitorSyncJobs:    monitorSyncJobs,
				MonitorVerifyJobs:  monitorVerifyJobs,
				MonitorPruneJobs:   monitorPruneJobs,
				MonitorGarbageJobs: monitorGarbageJobs,
				Source:             req.Source, // Track how this node was registered
			}
			h.getConfig(r.Context()).PBSInstances = append(h.getConfig(r.Context()).PBSInstances, newInstance)
		}
		log.Info().Str("host", req.Host).Str("type", req.Type).Msg("Added new node via auto-registration")
	}

	// Log what we're about to save
	if req.Type == "pve" && len(h.getConfig(r.Context()).PVEInstances) > 0 {
		lastNode := h.getConfig(r.Context()).PVEInstances[len(h.getConfig(r.Context()).PVEInstances)-1]
		log.Info().
			Str("name", lastNode.Name).
			Str("host", lastNode.Host).
			Str("tokenName", lastNode.TokenName).
			Bool("hasTokenValue", lastNode.TokenValue != "").
			Msg("About to save PVE node")
	}

	// Save configuration
	if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("Configuration saved successfully")

	actualName := h.findInstanceNameByHost(r.Context(), req.Type, host)
	if actualName == "" {
		actualName = strings.TrimSpace(req.ServerName)
	}
	if actualName == "" {
		actualName = strings.TrimSpace(nodeConfig.Name)
	}
	if actualName == "" {
		actualName = host
	}
	h.markAutoRegistered(req.Type, actualName)

	// Reload monitor to pick up new configuration
	if h.reloadFunc != nil {
		log.Info().Msg("Reloading monitor after auto-registration")
		go func() {
			// Run reload in background to avoid blocking the response
			if err := h.reloadFunc(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor after auto-registration")
			} else {
				log.Info().Msg("Monitor reloaded successfully after auto-registration")
			}
		}()
	}

	// Trigger a discovery refresh to remove the node from discovered list
	if h.getMonitor(r.Context()) != nil && h.getMonitor(r.Context()).GetDiscoveryService() != nil {
		log.Info().Msg("Triggering discovery refresh after auto-registration")
		h.getMonitor(r.Context()).GetDiscoveryService().ForceRefresh()
	}

	// Broadcast auto-registration success via WebSocket
	if h.wsHub != nil {
		nodeInfo := map[string]interface{}{
			"type":      req.Type,
			"host":      req.Host,
			"name":      req.ServerName,
			"tokenId":   req.TokenID,
			"hasToken":  true,
			"verifySSL": false,
			"status":    "connected",
		}

		// Broadcast the auto-registration success
		h.wsHub.BroadcastMessage(websocket.Message{
			Type:      "node_auto_registered",
			Data:      nodeInfo,
			Timestamp: time.Now().Format(time.RFC3339),
		})

		// Also broadcast a discovery update to refresh the UI
		if h.getMonitor(r.Context()) != nil && h.getMonitor(r.Context()).GetDiscoveryService() != nil {
			result, _ := h.getMonitor(r.Context()).GetDiscoveryService().GetCachedResult()
			if result != nil {
				h.wsHub.BroadcastMessage(websocket.Message{
					Type: "discovery_update",
					Data: map[string]interface{}{
						"servers":   result.Servers,
						"errors":    result.Errors,
						"timestamp": time.Now().Unix(),
					},
					Timestamp: time.Now().Format(time.RFC3339),
				})
				log.Info().Msg("Broadcasted discovery update after auto-registration")
			}
		}

		log.Info().
			Str("host", req.Host).
			Str("name", req.ServerName).
			Str("type", "node_auto_registered").
			Msg("Broadcasted auto-registration success via WebSocket")
	} else {
		log.Warn().Msg("WebSocket hub is nil, cannot broadcast auto-registration")
	}

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Node %s auto-registered successfully", req.Host),
		"nodeId":  req.Host,
	})
}

// handleSecureAutoRegister handles the new secure registration flow where Pulse generates the token
func (h *ConfigHandlers) handleSecureAutoRegister(w http.ResponseWriter, r *http.Request, req *AutoRegisterRequest, clientIP string) {
	log.Info().
		Str("type", req.Type).
		Str("host", req.Host).
		Str("username", req.Username).
		Msg("Processing secure auto-register request")

	// Generate a unique token name based on Pulse's IP/hostname
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = strings.ReplaceAll(clientIP, ".", "-")
	}
	timestamp := time.Now().Unix()
	tokenName := fmt.Sprintf("pulse-%s-%d", hostname, timestamp)

	// Generate a secure random token value
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Error().Err(err).Msg("Failed to generate secure token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	tokenValue := fmt.Sprintf("%x-%x-%x-%x-%x",
		tokenBytes[0:4], tokenBytes[4:6], tokenBytes[6:8], tokenBytes[8:10], tokenBytes[10:16])

	host, err := normalizeNodeHost(req.Host, req.Type)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fingerprint := ""
	if fp, err := tlsutil.FetchFingerprint(host); err != nil {
		log.Warn().Err(err).Str("host", host).Msg("Failed to fetch TLS fingerprint for auto-register")
	} else {
		fingerprint = fp
	}
	verifySSL := true

	// Create the token on the remote server
	var fullTokenID string
	var createErr error

	if req.Type == "pve" {
		// For PVE, create token via API
		fullTokenID = fmt.Sprintf("pulse-monitor@pam!%s", tokenName)
		// Note: This would require implementing token creation in the proxmox package
		// For now, we'll return the token for the script to create
		// TODO: Implement PVE token creation via API
	} else if req.Type == "pbs" {
		// For PBS, create token via API using the new client methods
		log.Info().
			Str("host", host).
			Str("username", req.Username).
			Msg("Creating PBS token via API")

		pbsClient, err := pbs.NewClient(pbs.ClientConfig{
			Host:        host,
			User:        req.Username,
			Password:    req.Password,
			Fingerprint: fingerprint,
			VerifySSL:   verifySSL,
		})
		if err != nil {
			log.Error().Err(err).Str("host", host).Msg("Failed to create PBS client")
			http.Error(w, fmt.Sprintf("Failed to connect to PBS: %v", err), http.StatusBadRequest)
			return
		}

		// Use the turnkey method to create user + token
		tokenID, tokenSecret, err := pbsClient.SetupMonitoringAccess(context.Background(), tokenName)
		if err != nil {
			log.Error().Err(err).Str("host", host).Msg("Failed to create PBS monitoring access")
			http.Error(w, fmt.Sprintf("Failed to create token: %v", err), http.StatusInternalServerError)
			return
		}

		fullTokenID = tokenID
		tokenValue = tokenSecret
		log.Info().
			Str("host", host).
			Str("tokenID", fullTokenID).
			Msg("Successfully created PBS token via API")
	}

	if createErr != nil {
		log.Error().Err(createErr).Msg("Failed to create token on remote server")
		http.Error(w, "Failed to create token on remote server", http.StatusInternalServerError)
		return
	}

	// Determine server name
	serverName := req.ServerName
	if serverName == "" {
		// Extract from host
		serverName = host
		serverName = strings.TrimPrefix(serverName, "https://")
		serverName = strings.TrimPrefix(serverName, "http://")
		if idx := strings.Index(serverName, ":"); idx > 0 {
			serverName = serverName[:idx]
		}
	}

	// Add the node to configuration
	if req.Type == "pve" {
		pveNode := config.PVEInstance{
			Name:              serverName,
			Host:              host,
			TokenName:         fullTokenID,
			TokenValue:        tokenValue,
			Fingerprint:       fingerprint,
			VerifySSL:         verifySSL,
			MonitorVMs:        true,
			MonitorContainers: true,
			MonitorStorage:    true,
			MonitorBackups:    true,
		}
		// Deduplicate by host to keep secure auto-registration idempotent on reruns.
		existingIndex := -1
		for i, node := range h.getConfig(r.Context()).PVEInstances {
			if node.Host == host {
				existingIndex = i
				break
			}
		}
		if existingIndex >= 0 {
			instance := &h.getConfig(r.Context()).PVEInstances[existingIndex]
			instance.Host = host
			instance.User = ""
			instance.Password = ""
			instance.TokenName = pveNode.TokenName
			instance.TokenValue = pveNode.TokenValue
			instance.Fingerprint = pveNode.Fingerprint
			instance.VerifySSL = pveNode.VerifySSL
			log.Info().Str("host", host).Str("type", "pve").Msg("Secure auto-register matched existing node by host; updated token in-place")
		} else {
			h.getConfig(r.Context()).PVEInstances = append(h.getConfig(r.Context()).PVEInstances, pveNode)
		}
	} else if req.Type == "pbs" {
		pbsNode := config.PBSInstance{
			Name:              serverName,
			Host:              host,
			TokenName:         fullTokenID,
			TokenValue:        tokenValue,
			Fingerprint:       fingerprint,
			VerifySSL:         verifySSL,
			MonitorBackups:    true,
			MonitorDatastores: true,
			MonitorSyncJobs:   true,
			MonitorVerifyJobs: true,
			MonitorPruneJobs:  true,
		}
		// Deduplicate by host to keep secure auto-registration idempotent on reruns.
		existingIndex := -1
		for i, node := range h.getConfig(r.Context()).PBSInstances {
			if node.Host == host {
				existingIndex = i
				break
			}
		}
		if existingIndex >= 0 {
			instance := &h.getConfig(r.Context()).PBSInstances[existingIndex]
			instance.Host = host
			instance.User = ""
			instance.Password = ""
			instance.TokenName = pbsNode.TokenName
			instance.TokenValue = pbsNode.TokenValue
			instance.Fingerprint = pbsNode.Fingerprint
			instance.VerifySSL = pbsNode.VerifySSL
			log.Info().Str("host", host).Str("type", "pbs").Msg("Secure auto-register matched existing node by host; updated token in-place")
		} else {
			h.getConfig(r.Context()).PBSInstances = append(h.getConfig(r.Context()).PBSInstances, pbsNode)
		}
	}

	// Save configuration
	if err := h.getPersistence(r.Context()).SaveNodesConfig(h.getConfig(r.Context()).PVEInstances, h.getConfig(r.Context()).PBSInstances, h.getConfig(r.Context()).PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	actualName := h.findInstanceNameByHost(r.Context(), req.Type, host)
	if actualName == "" {
		actualName = serverName
	}
	h.markAutoRegistered(req.Type, actualName)

	// Reload monitor
	if h.reloadFunc != nil {
		go func() {
			if err := h.reloadFunc(); err != nil {
				log.Error().Err(err).Msg("Failed to reload monitor after auto-registration")
			}
		}()
	}

	// Send success response with token details for script to create
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "success",
		"message":    fmt.Sprintf("Node %s registered successfully", req.Host),
		"nodeId":     serverName,
		"tokenId":    fullTokenID,
		"tokenValue": tokenValue,
		"action":     "create_token", // Tells the script to create this token
	})

}

// SSHKeyPair holds the sensors SSH public key for temperature monitoring.
type SSHKeyPair struct {
	SensorsPublicKey string
}

// getOrGenerateSSHKeys returns the SSH public key for temperature monitoring
// If keys don't exist, they are generated automatically
// SECURITY: Blocks key generation when running in containers unless dev mode override is enabled
func (h *ConfigHandlers) getOrGenerateSSHKeys() SSHKeyPair {
	// CRITICAL SECURITY CHECK: Never generate SSH keys in containers (unless dev mode)
	// Container compromise = SSH key compromise = root access to Proxmox
	devModeAllowSSH := os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true"
	isContainer := os.Getenv("PULSE_DOCKER") == "true" || system.InContainer()

	if isContainer && !devModeAllowSSH {
		log.Error().Msg("SECURITY BLOCK: SSH key generation disabled in containerized deployments")
		log.Error().Msg("Temperature monitoring via SSH is disabled in containerized deployments")
		log.Error().Msg("See: https://github.com/rcourtman/Pulse/blob/main/SECURITY.md#critical-security-notice-for-container-deployments")
		log.Error().Msg("To test SSH keys in dev/lab only: PULSE_DEV_ALLOW_CONTAINER_SSH=true (NEVER in production!)")
		return SSHKeyPair{}
	}

	if devModeAllowSSH && isContainer {
		log.Warn().Msg("⚠️  DEV MODE: SSH key generation ENABLED in container - FOR TESTING ONLY")
		log.Warn().Msg("⚠️  This grants root SSH access from container - NEVER use in production!")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Warn().Err(err).Msg("Could not determine home directory for SSH keys")
		return SSHKeyPair{}
	}

	sshDir := filepath.Join(homeDir, ".ssh")

	// Generate/load sensors key (for temperature collection)
	sensorsPrivPath := filepath.Join(sshDir, "id_ed25519_sensors")
	sensorsPubPath := filepath.Join(sshDir, "id_ed25519_sensors.pub")
	sensorsKey := h.generateOrLoadSSHKey(sshDir, sensorsPrivPath, sensorsPubPath, "sensors")

	return SSHKeyPair{
		SensorsPublicKey: sensorsKey,
	}
}

// generateOrLoadSSHKey generates or loads a single SSH keypair
func (h *ConfigHandlers) generateOrLoadSSHKey(sshDir, privateKeyPath, publicKeyPath, keyType string) string {
	// Check if public key already exists
	if pubKeyBytes, err := os.ReadFile(publicKeyPath); err == nil {
		publicKey := strings.TrimSpace(string(pubKeyBytes))
		log.Info().Str("keyPath", publicKeyPath).Str("type", keyType).Msg("Using existing SSH public key")
		return publicKey
	}

	// Key doesn't exist - generate one
	log.Info().Str("sshDir", sshDir).Str("type", keyType).Msg("Generating new SSH keypair for temperature monitoring")

	// Create .ssh directory if it doesn't exist
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		log.Error().Err(err).Str("sshDir", sshDir).Msg("Failed to create .ssh directory")
		return ""
	}

	// Generate Ed25519 key pair (more secure and faster than RSA)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate Ed25519 key")
		return ""
	}

	// Save private key in OpenSSH format
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Error().Err(err).Str("path", privateKeyPath).Msg("Failed to create private key file")
		return ""
	}
	defer privateKeyFile.Close()

	// Marshal Ed25519 private key to OpenSSH format
	privKeyBytes, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal private key")
		return ""
	}
	if err := pem.Encode(privateKeyFile, privKeyBytes); err != nil {
		log.Error().Err(err).Msg("Failed to write private key")
		return ""
	}

	// Generate public key in OpenSSH format
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate public key")
		return ""
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(sshPublicKey)
	publicKeyString := strings.TrimSpace(string(publicKeyBytes))

	// Save public key
	if err := os.WriteFile(publicKeyPath, publicKeyBytes, 0644); err != nil {
		log.Error().Err(err).Str("path", publicKeyPath).Msg("Failed to write public key")
		return ""
	}

	log.Info().
		Str("privateKey", privateKeyPath).
		Str("publicKey", publicKeyPath).
		Msg("Successfully generated SSH keypair")

	return publicKeyString
}

// AgentInstallCommandRequest represents a request for an agent install command
type AgentInstallCommandRequest struct {
	Type string `json:"type"` // "pve" or "pbs"
}

// AgentInstallCommandResponse contains the generated install command
type AgentInstallCommandResponse struct {
	Command string `json:"command"`
	Token   string `json:"token"`
}

// HandleAgentInstallCommand generates an API token and install command for agent-based Proxmox setup
func (h *ConfigHandlers) HandleAgentInstallCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AgentInstallCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate type
	if req.Type != "pve" && req.Type != "pbs" {
		http.Error(w, "Type must be 'pve' or 'pbs'", http.StatusBadRequest)
		return
	}

	// Generate a new API token with host report and host manage scopes
	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate API token for agent install")
		http.Error(w, "Failed to generate API token", http.StatusInternalServerError)
		return
	}

	tokenName := fmt.Sprintf("proxmox-agent-%s-%d", req.Type, time.Now().Unix())
	scopes := []string{
		config.ScopeHostReport,
		config.ScopeHostConfigRead,
		config.ScopeHostManage,
		config.ScopeAgentExec,
	}

	record, err := config.NewAPITokenRecord(rawToken, tokenName, scopes)
	if err != nil {
		log.Error().Err(err).Str("token_name", tokenName).Msg("Failed to construct API token record")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Persist the token
	config.Mu.Lock()
	h.getConfig(r.Context()).APITokens = append(h.getConfig(r.Context()).APITokens, *record)
	h.getConfig(r.Context()).SortAPITokens()

	if h.getPersistence(r.Context()) != nil {
		if err := h.getPersistence(r.Context()).SaveAPITokens(h.getConfig(r.Context()).APITokens); err != nil {
			// Rollback the in-memory addition
			h.getConfig(r.Context()).APITokens = h.getConfig(r.Context()).APITokens[:len(h.getConfig(r.Context()).APITokens)-1]
			config.Mu.Unlock()
			log.Error().Err(err).Msg("Failed to persist API tokens after creation")
			http.Error(w, "Failed to save token to disk: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	config.Mu.Unlock()

	// Derive Pulse URL from the request
	host := r.Host
	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		if (parsedHost == "127.0.0.1" || parsedHost == "localhost") && parsedPort == strconv.Itoa(h.getConfig(r.Context()).FrontendPort) {
			// Prefer a user-configured public URL when we're running on loopback
			if publicURL := strings.TrimSpace(h.getConfig(r.Context()).PublicURL); publicURL != "" {
				if parsedURL, err := url.Parse(publicURL); err == nil && parsedURL.Host != "" {
					host = parsedURL.Host
				}
			}
		}
	}

	// Detect protocol - check both TLS and proxy headers
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	pulseURL := fmt.Sprintf("%s://%s", scheme, host)

	// Generate the install command
	command := fmt.Sprintf(`curl -fsSL %s/install.sh | bash -s -- \
  --url %s \
  --token %s \
  --enable-proxmox`,
		pulseURL, pulseURL, rawToken)

	log.Info().
		Str("token_name", tokenName).
		Str("type", req.Type).
		Msg("Generated agent install command with API token")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AgentInstallCommandResponse{
		Command: command,
		Token:   rawToken,
	})
}
