package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	internalauth "github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/tempproxy"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// SetupCode represents a one-time setup code for secure node registration
type SetupCode struct {
	ExpiresAt time.Time
	Used      bool
	NodeType  string // "pve" or "pbs"
	Host      string // The host URL for validation
}

// ConfigHandlers handles configuration-related API endpoints
type ConfigHandlers struct {
	config                   *config.Config
	persistence              *config.ConfigPersistence
	monitor                  *monitoring.Monitor
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
func NewConfigHandlers(cfg *config.Config, monitor *monitoring.Monitor, reloadFunc func() error, wsHub *websocket.Hub, guestMetadataHandler *GuestMetadataHandler, reloadSystemSettingsFunc func()) *ConfigHandlers {
	h := &ConfigHandlers{
		config:                   cfg,
		persistence:              config.NewConfigPersistence(cfg.DataPath),
		monitor:                  monitor,
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

// SetMonitor updates the monitor reference used by the config handlers.
func (h *ConfigHandlers) SetMonitor(m *monitoring.Monitor) {
	h.monitor = m
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
		// Allow tokens while they are valid or within a short grace period after use.
		if now.Before(code.ExpiresAt.Add(2 * time.Minute)) {
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

func (h *ConfigHandlers) findInstanceNameByHost(nodeType, host string) string {
	switch nodeType {
	case "pve":
		for _, node := range h.config.PVEInstances {
			if node.Host == host {
				return node.Name
			}
		}
	case "pbs":
		for _, node := range h.config.PBSInstances {
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

func (h *ConfigHandlers) maybeRefreshClusterInfo(instance *config.PVEInstance) {
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
	isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, instance.Name)
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

	if h.persistence != nil {
		if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances, h.config.PMGInstances); err != nil {
			log.Warn().
				Err(err).
				Str("instance", instance.Name).
				Msg("Failed to persist cluster detection update")
		}
	}
}

// NodeConfigRequest represents a request to add/update a node
type NodeConfigRequest struct {
	Type                 string `json:"type"` // "pve", "pbs", or "pmg"
	Name                 string `json:"name"`
	Host                 string `json:"host"`
	User                 string `json:"user,omitempty"`
	Password             string `json:"password,omitempty"`
	TokenName            string `json:"tokenName,omitempty"`
	TokenValue           string `json:"tokenValue,omitempty"`
	Fingerprint          string `json:"fingerprint,omitempty"`
	VerifySSL            bool   `json:"verifySSL"`
	MonitorVMs           bool   `json:"monitorVMs,omitempty"`           // PVE only
	MonitorContainers    bool   `json:"monitorContainers,omitempty"`    // PVE only
	MonitorStorage       bool   `json:"monitorStorage,omitempty"`       // PVE only
	MonitorBackups       bool   `json:"monitorBackups,omitempty"`       // PVE only
	MonitorPhysicalDisks *bool  `json:"monitorPhysicalDisks,omitempty"` // PVE only (nil = enabled by default)
	MonitorDatastores    bool   `json:"monitorDatastores,omitempty"`    // PBS only
	MonitorSyncJobs      bool   `json:"monitorSyncJobs,omitempty"`      // PBS only
	MonitorVerifyJobs    bool   `json:"monitorVerifyJobs,omitempty"`    // PBS only
	MonitorPruneJobs     bool   `json:"monitorPruneJobs,omitempty"`     // PBS only
	MonitorGarbageJobs   bool   `json:"monitorGarbageJobs,omitempty"`   // PBS only
	MonitorMailStats     bool   `json:"monitorMailStats,omitempty"`     // PMG only
	MonitorQueues        bool   `json:"monitorQueues,omitempty"`        // PMG only
	MonitorQuarantine    bool   `json:"monitorQuarantine,omitempty"`    // PMG only
	MonitorDomainStats   bool   `json:"monitorDomainStats,omitempty"`   // PMG only
}

// NodeResponse represents a node in API responses
type NodeResponse struct {
	ID                   string                   `json:"id"`
	Type                 string                   `json:"type"`
	Name                 string                   `json:"name"`
	Host                 string                   `json:"host"`
	User                 string                   `json:"user,omitempty"`
	HasPassword          bool                     `json:"hasPassword"`
	TokenName            string                   `json:"tokenName,omitempty"`
	HasToken             bool                     `json:"hasToken"`
	Fingerprint          string                   `json:"fingerprint,omitempty"`
	VerifySSL            bool                     `json:"verifySSL"`
	MonitorVMs           bool                     `json:"monitorVMs,omitempty"`
	MonitorContainers    bool                     `json:"monitorContainers,omitempty"`
	MonitorStorage       bool                     `json:"monitorStorage,omitempty"`
	MonitorBackups       bool                     `json:"monitorBackups,omitempty"`
	MonitorPhysicalDisks *bool                    `json:"monitorPhysicalDisks,omitempty"`
	MonitorDatastores    bool                     `json:"monitorDatastores,omitempty"`
	MonitorSyncJobs      bool                     `json:"monitorSyncJobs,omitempty"`
	MonitorVerifyJobs    bool                     `json:"monitorVerifyJobs,omitempty"`
	MonitorPruneJobs     bool                     `json:"monitorPruneJobs,omitempty"`
	MonitorGarbageJobs   bool                     `json:"monitorGarbageJobs,omitempty"`
	MonitorMailStats     bool                     `json:"monitorMailStats,omitempty"`
	MonitorQueues        bool                     `json:"monitorQueues,omitempty"`
	MonitorQuarantine    bool                     `json:"monitorQuarantine,omitempty"`
	MonitorDomainStats   bool                     `json:"monitorDomainStats,omitempty"`
	Status               string                   `json:"status"` // "connected", "disconnected", "error"
	IsCluster            bool                     `json:"isCluster,omitempty"`
	ClusterName          string                   `json:"clusterName,omitempty"`
	ClusterEndpoints     []config.ClusterEndpoint `json:"clusterEndpoints,omitempty"`
}

// validateNodeAPI tests if a cluster node has a working Proxmox API
// This helps filter out qdevice VMs and other non-Proxmox participants
func validateNodeAPI(clusterNode proxmox.ClusterStatus, baseConfig proxmox.ClientConfig) bool {
	// Determine the host to test - prefer IP if available, otherwise use node name
	testHost := clusterNode.IP
	if testHost == "" {
		testHost = clusterNode.Name
	}

	// Skip empty hostnames (shouldn't happen but be safe)
	if testHost == "" {
		return false
	}

	// Create a test configuration for this specific node
	testConfig := baseConfig
	testConfig.Host = testHost
	if !strings.HasPrefix(testConfig.Host, "http") {
		testConfig.Host = fmt.Sprintf("https://%s:8006", testConfig.Host)
	}

	// Use a very short timeout for validation - we just need to know if the API exists
	testConfig.Timeout = 2 * time.Second

	log.Debug().
		Str("node", clusterNode.Name).
		Str("test_host", testConfig.Host).
		Msg("Validating Proxmox API for cluster node")

	// Try to create a client and make a simple API call
	testClient, err := proxmox.NewClient(testConfig)
	if err != nil {
		// Many clusters use unique certificates per node. If the primary node
		// was configured with fingerprint pinning, connecting to peers with the
		// same fingerprint will fail. Fall back to a relaxed TLS check so we can
		// still detect valid cluster members while keeping other errors (like
		// auth) as hard failures.
		isTLSMismatch := strings.Contains(err.Error(), "fingerprint") || strings.Contains(err.Error(), "x509") || strings.Contains(err.Error(), "certificate")
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
			return false
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
			return true
		}

		log.Debug().
			Str("node", clusterNode.Name).
			Err(err).
			Msg("Node failed Proxmox API validation - likely not a Proxmox node")
		return false
	}

	log.Debug().
		Str("node", clusterNode.Name).
		Msg("Node passed Proxmox API validation")

	return true
}

// detectPVECluster checks if a PVE node is part of a cluster and returns cluster information
func detectPVECluster(clientConfig proxmox.ClientConfig, nodeName string) (isCluster bool, clusterName string, clusterEndpoints []config.ClusterEndpoint) {
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
		if strings.Contains(lastErr.Error(), "501") || strings.Contains(lastErr.Error(), "not implemented") {
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

		scheme := "https://"
		if strings.HasPrefix(strings.ToLower(clientConfig.Host), "http://") {
			scheme = "http://"
		}

		var unvalidatedNodes []proxmox.ClusterStatus

		for _, clusterNode := range clusterNodes {
			// Validate that this node actually has a working Proxmox API
			// This filters out qdevice VMs and other non-Proxmox participants
			if !validateNodeAPI(clusterNode, clientConfig) {
				log.Debug().
					Str("node", clusterNode.Name).
					Str("ip", clusterNode.IP).
					Msg("Skipping cluster node - no valid Proxmox API detected (likely qdevice or external node)")
				unvalidatedNodes = append(unvalidatedNodes, clusterNode)
				continue
			}

			// Build the host URL with proper port
			// Prefer IP if available, otherwise use node name
			nodeHost := clusterNode.IP
			if nodeHost == "" {
				nodeHost = clusterNode.Name
			}
			// Ensure host has port (PVE uses 8006)
			if !strings.Contains(nodeHost, ":") {
				nodeHost = nodeHost + ":8006"
			}

			endpoint := config.ClusterEndpoint{
				NodeID:   clusterNode.ID,
				NodeName: clusterNode.Name,
				Host:     scheme + nodeHost,
				Online:   clusterNode.Online == 1,
				LastSeen: time.Now(),
			}

			if clusterNode.IP != "" {
				endpoint.IP = clusterNode.IP
			}

			clusterEndpoints = append(clusterEndpoints, endpoint)
		}

		if len(clusterEndpoints) == 0 && len(unvalidatedNodes) > 0 {
			log.Warn().
				Str("cluster", clusterName).
				Int("total_discovered", len(unvalidatedNodes)).
				Msg("All detected cluster nodes failed validation; falling back to cluster metadata")

			for _, clusterNode := range unvalidatedNodes {
				nodeHost := clusterNode.IP
				if nodeHost == "" {
					nodeHost = clusterNode.Name
				}
				if nodeHost == "" {
					continue
				}
				if !strings.Contains(nodeHost, ":") {
					nodeHost = nodeHost + ":8006"
				}

				endpoint := config.ClusterEndpoint{
					NodeID:   clusterNode.ID,
					NodeName: clusterNode.Name,
					Host:     scheme + nodeHost,
					Online:   clusterNode.Online == 1,
					LastSeen: time.Now(),
				}

				if clusterNode.IP != "" {
					endpoint.IP = clusterNode.IP
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
func (h *ConfigHandlers) GetAllNodesForAPI() []NodeResponse {
	nodes := []NodeResponse{}

	// Add PVE nodes
	for i, pve := range h.config.PVEInstances {
		// Refresh cluster metadata if we previously failed to detect endpoints
		h.maybeRefreshClusterInfo(&h.config.PVEInstances[i])
		pve = h.config.PVEInstances[i]
		node := NodeResponse{
			ID:                   generateNodeID("pve", i),
			Type:                 "pve",
			Name:                 pve.Name,
			Host:                 pve.Host,
			User:                 pve.User,
			HasPassword:          pve.Password != "",
			TokenName:            pve.TokenName,
			HasToken:             pve.TokenValue != "",
			Fingerprint:          pve.Fingerprint,
			VerifySSL:            pve.VerifySSL,
			MonitorVMs:           pve.MonitorVMs,
			MonitorContainers:    pve.MonitorContainers,
			MonitorStorage:       pve.MonitorStorage,
			MonitorBackups:       pve.MonitorBackups,
			MonitorPhysicalDisks: pve.MonitorPhysicalDisks,
			Status:               h.getNodeStatus("pve", pve.Name),
			IsCluster:            pve.IsCluster,
			ClusterName:          pve.ClusterName,
			ClusterEndpoints:     pve.ClusterEndpoints,
		}
		nodes = append(nodes, node)
	}

	// Add PBS nodes
	for i, pbs := range h.config.PBSInstances {
		node := NodeResponse{
			ID:                 generateNodeID("pbs", i),
			Type:               "pbs",
			Name:               pbs.Name,
			Host:               pbs.Host,
			User:               pbs.User,
			HasPassword:        pbs.Password != "",
			TokenName:          pbs.TokenName,
			HasToken:           pbs.TokenValue != "",
			Fingerprint:        pbs.Fingerprint,
			VerifySSL:          pbs.VerifySSL,
			MonitorDatastores:  pbs.MonitorDatastores,
			MonitorSyncJobs:    pbs.MonitorSyncJobs,
			MonitorVerifyJobs:  pbs.MonitorVerifyJobs,
			MonitorPruneJobs:   pbs.MonitorPruneJobs,
			MonitorGarbageJobs: pbs.MonitorGarbageJobs,
			Status:             h.getNodeStatus("pbs", pbs.Name),
		}
		nodes = append(nodes, node)
	}

	// Add PMG nodes
	for i, pmgInst := range h.config.PMGInstances {
		monitorMailStats := pmgInst.MonitorMailStats
		if !pmgInst.MonitorMailStats && !pmgInst.MonitorQueues && !pmgInst.MonitorQuarantine && !pmgInst.MonitorDomainStats {
			monitorMailStats = true
		}

		node := NodeResponse{
			ID:                 generateNodeID("pmg", i),
			Type:               "pmg",
			Name:               pmgInst.Name,
			Host:               pmgInst.Host,
			User:               pmgInst.User,
			HasPassword:        pmgInst.Password != "",
			TokenName:          pmgInst.TokenName,
			HasToken:           pmgInst.TokenValue != "",
			Fingerprint:        pmgInst.Fingerprint,
			VerifySSL:          pmgInst.VerifySSL,
			MonitorMailStats:   monitorMailStats,
			MonitorQueues:      pmgInst.MonitorQueues,
			MonitorQuarantine:  pmgInst.MonitorQuarantine,
			MonitorDomainStats: pmgInst.MonitorDomainStats,
			Status:             h.getNodeStatus("pmg", pmgInst.Name),
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
		state := h.monitor.GetState()

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

	nodes := h.GetAllNodesForAPI()

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

// HandleAddNode adds a new node
func (h *ConfigHandlers) HandleAddNode(w http.ResponseWriter, r *http.Request) {
	// Prevent node modifications in mock mode
	if mock.IsMockEnabled() {
		http.Error(w, "Cannot modify nodes in mock mode. Please disable mock mode first: /opt/pulse/scripts/toggle-mock.sh off", http.StatusForbidden)
		return
	}

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

	// Check for duplicate nodes by name
	switch req.Type {
	case "pve":
		for _, node := range h.config.PVEInstances {
			if node.Name == req.Name {
				http.Error(w, "A node with this name already exists", http.StatusConflict)
				return
			}
		}
	case "pbs":
		for _, node := range h.config.PBSInstances {
			if node.Name == req.Name {
				http.Error(w, "A node with this name already exists", http.StatusConflict)
				return
			}
		}
	case "pmg":
		for _, node := range h.config.PMGInstances {
			if node.Name == req.Name {
				http.Error(w, "A node with this name already exists", http.StatusConflict)
				return
			}
		}
	}

	// Add to appropriate list
	if req.Type == "pve" {
		if req.Password != "" && req.TokenName == "" && req.TokenValue == "" {
			req.User = normalizePVEUser(req.User)
		}
		// Ensure host has protocol
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Add port if missing
		if !strings.Contains(host, ":8006") && !strings.Contains(host, ":443") {
			if strings.HasPrefix(host, "https://") {
				host = strings.Replace(host, "https://", "https://", 1)
				if !strings.Contains(host[8:], ":") {
					host += ":8006"
				}
			}
		}

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
			clientConfig := config.CreateProxmoxConfigFromFields(host, req.User, req.Password, req.TokenName, req.TokenValue, req.Fingerprint, req.VerifySSL)
			isCluster, clusterName, clusterEndpoints = detectPVECluster(clientConfig, req.Name)
		}

		if isCluster {
			log.Info().
				Str("cluster", clusterName).
				Int("endpoints", len(clusterEndpoints)).
				Msg("Detected Proxmox cluster, auto-discovering all nodes")
		}

		pve := config.PVEInstance{
			Name:                 req.Name,
			Host:                 host, // Use normalized host
			User:                 req.User,
			Password:             req.Password,
			TokenName:            req.TokenName,
			TokenValue:           req.TokenValue,
			Fingerprint:          req.Fingerprint,
			VerifySSL:            req.VerifySSL,
			MonitorVMs:           req.MonitorVMs,
			MonitorContainers:    req.MonitorContainers,
			MonitorStorage:       req.MonitorStorage,
			MonitorBackups:       req.MonitorBackups,
			MonitorPhysicalDisks: req.MonitorPhysicalDisks,
			IsCluster:            isCluster,
			ClusterName:          clusterName,
			ClusterEndpoints:     clusterEndpoints,
		}
		h.config.PVEInstances = append(h.config.PVEInstances, pve)

		if isCluster {
			log.Info().
				Str("cluster", clusterName).
				Int("endpoints", len(clusterEndpoints)).
				Msg("Added Proxmox cluster with auto-discovered endpoints")
		}
	} else if req.Type == "pbs" {
		// PBS node - ensure host has protocol and port
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Add default PBS port if missing
		// Check if there's no port specified after the protocol
		protocolEnd := 0
		if strings.HasPrefix(host, "https://") {
			protocolEnd = 8
		} else if strings.HasPrefix(host, "http://") {
			protocolEnd = 7
		}
		// Only add default port if no port is specified
		if protocolEnd > 0 && !strings.Contains(host[protocolEnd:], ":") {
			host = host + ":8007"
		}

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
			// Using password authentication - don't store token fields
			pbsUser = req.User
			pbsPassword = req.Password
			// Ensure user has realm for PBS
			if pbsUser != "" && !strings.Contains(pbsUser, "@") {
				pbsUser = pbsUser + "@pbs" // Default to @pbs realm if not specified
			}
		}

		pbs := config.PBSInstance{
			Name:               req.Name,
			Host:               host,
			User:               pbsUser,
			Password:           pbsPassword,
			TokenName:          pbsTokenName,
			TokenValue:         pbsTokenValue,
			Fingerprint:        req.Fingerprint,
			VerifySSL:          req.VerifySSL,
			MonitorBackups:     true, // Enable by default for PBS
			MonitorDatastores:  req.MonitorDatastores,
			MonitorSyncJobs:    req.MonitorSyncJobs,
			MonitorVerifyJobs:  req.MonitorVerifyJobs,
			MonitorPruneJobs:   req.MonitorPruneJobs,
			MonitorGarbageJobs: req.MonitorGarbageJobs,
		}
		h.config.PBSInstances = append(h.config.PBSInstances, pbs)
	} else if req.Type == "pmg" {
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		protocolEnd := 0
		if strings.HasPrefix(host, "https://") {
			protocolEnd = 8
		} else if strings.HasPrefix(host, "http://") {
			protocolEnd = 7
		}
		if protocolEnd > 0 && !strings.Contains(host[protocolEnd:], ":") {
			host = host + ":8006"
		}

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

		monitorMailStats := req.MonitorMailStats
		if !req.MonitorMailStats && !req.MonitorQueues && !req.MonitorQuarantine && !req.MonitorDomainStats {
			monitorMailStats = true
		}

		pmgInstance := config.PMGInstance{
			Name:               req.Name,
			Host:               host,
			User:               pmgUser,
			Password:           pmgPassword,
			TokenName:          pmgTokenName,
			TokenValue:         pmgTokenValue,
			Fingerprint:        req.Fingerprint,
			VerifySSL:          req.VerifySSL,
			MonitorMailStats:   monitorMailStats,
			MonitorQueues:      req.MonitorQueues,
			MonitorQuarantine:  req.MonitorQuarantine,
			MonitorDomainStats: req.MonitorDomainStats,
		}
		h.config.PMGInstances = append(h.config.PMGInstances, pmgInstance)
	}

	// Save configuration to disk using our persistence instance
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances, h.config.PMGInstances); err != nil {
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
		host := req.Host
		if strings.HasPrefix(host, "http://") {
			host = strings.TrimPrefix(host, "http://")
		}
		if strings.HasPrefix(host, "https://") {
			host = strings.TrimPrefix(host, "https://")
		}
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

	// Test connection based on type
	if req.Type == "pve" {
		// Ensure host has protocol
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Add port if missing
		if !strings.Contains(host, ":8006") && !strings.Contains(host, ":443") {
			if strings.HasPrefix(host, "https://") {
				host = strings.Replace(host, "https://", "https://", 1)
				if !strings.Contains(host[8:], ":") {
					host += ":8006"
				}
			}
		}

		// Create a temporary client
		authUser := req.User
		if req.Password != "" && req.TokenName == "" && req.TokenValue == "" {
			authUser = normalizePVEUser(authUser)
			req.User = authUser
		}
		clientConfig := proxmox.ClientConfig{
			Host:        host,
			User:        authUser,
			Password:    req.Password,
			TokenName:   req.TokenName, // Pass the full token ID
			TokenValue:  req.TokenValue,
			VerifySSL:   req.VerifySSL,
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

		isCluster, _, clusterEndpoints := detectPVECluster(clientConfig, req.Name)

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
		// Ensure host has protocol for PBS
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Add port if missing (PBS defaults to port 8007)
		if strings.HasPrefix(host, "https://") {
			// Check if there's already a port specified after the protocol
			hostWithoutProtocol := host[8:] // Remove "https://"
			if !strings.Contains(hostWithoutProtocol, ":") {
				// No port specified, add default PBS port
				host += ":8007"
			}
		} else if strings.HasPrefix(host, "http://") {
			// Check if there's already a port specified after the protocol
			hostWithoutProtocol := host[7:] // Remove "http://"
			if !strings.Contains(hostWithoutProtocol, ":") {
				// No port specified, add default PBS port
				host += ":8007"
			}
		}

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

		clientConfig := pbs.ClientConfig{
			Host:        host,
			User:        pbsUser,
			Password:    req.Password,
			TokenName:   pbsTokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   req.VerifySSL,
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
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		protocolEnd := 0
		if strings.HasPrefix(host, "https://") {
			protocolEnd = 8
		} else if strings.HasPrefix(host, "http://") {
			protocolEnd = 7
		}
		if protocolEnd > 0 && !strings.Contains(host[protocolEnd:], ":") {
			host = host + ":8006"
		}

		clientConfig := config.CreatePMGConfigFromFields(host, req.User, req.Password, req.TokenName, req.TokenValue, req.Fingerprint, req.VerifySSL)

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

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

		message := "Connected to PMG instance"
		if versionLabel != "" {
			message = fmt.Sprintf("Connected to PMG instance (version %s)", versionLabel)
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

	var req NodeConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
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

	// Update the node
	if nodeType == "pve" && index < len(h.config.PVEInstances) {
		pve := &h.config.PVEInstances[index]

		// Only update name if provided
		if req.Name != "" {
			pve.Name = req.Name
		}

		if req.Host != "" {
			host := req.Host
			if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
				host = "https://" + host
			}
			protocolEnd := 0
			if strings.HasPrefix(host, "https://") {
				protocolEnd = 8
			} else if strings.HasPrefix(host, "http://") {
				protocolEnd = 7
			}
			if protocolEnd > 0 && !strings.Contains(host[protocolEnd:], ":") {
				host = host + ":8006"
			}
			pve.Host = host
		}

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
		pve.VerifySSL = req.VerifySSL
		pve.MonitorVMs = req.MonitorVMs
		pve.MonitorContainers = req.MonitorContainers
		pve.MonitorStorage = req.MonitorStorage
		pve.MonitorBackups = req.MonitorBackups
		pve.MonitorPhysicalDisks = req.MonitorPhysicalDisks
	} else if nodeType == "pbs" && index < len(h.config.PBSInstances) {
		pbs := &h.config.PBSInstances[index]
		pbs.Name = req.Name

		// Ensure PBS host has protocol and port
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Check if port is missing
		protocolEnd := 0
		if strings.HasPrefix(host, "https://") {
			protocolEnd = 8
		} else if strings.HasPrefix(host, "http://") {
			protocolEnd = 7
		}
		// Only add default port if no port is specified
		if protocolEnd > 0 && !strings.Contains(host[protocolEnd:], ":") {
			host = host + ":8007"
		}
		pbs.Host = host

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
		pbs.VerifySSL = req.VerifySSL
		pbs.MonitorBackups = true // Enable by default for PBS
		pbs.MonitorDatastores = req.MonitorDatastores
		pbs.MonitorSyncJobs = req.MonitorSyncJobs
		pbs.MonitorVerifyJobs = req.MonitorVerifyJobs
		pbs.MonitorPruneJobs = req.MonitorPruneJobs
		pbs.MonitorGarbageJobs = req.MonitorGarbageJobs
	} else if nodeType == "pmg" && index < len(h.config.PMGInstances) {
		pmgInst := &h.config.PMGInstances[index]
		pmgInst.Name = req.Name

		if req.Host != "" {
			host := req.Host
			if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
				host = "https://" + host
			}
			protocolEnd := 0
			if strings.HasPrefix(host, "https://") {
				protocolEnd = 8
			} else if strings.HasPrefix(host, "http://") {
				protocolEnd = 7
			}
			if protocolEnd > 0 && !strings.Contains(host[protocolEnd:], ":") {
				host = host + ":8006"
			}
			pmgInst.Host = host
		}

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
		pmgInst.VerifySSL = req.VerifySSL
		monitorMailStats := req.MonitorMailStats
		if !req.MonitorMailStats && !req.MonitorQueues && !req.MonitorQuarantine && !req.MonitorDomainStats {
			monitorMailStats = true
		}
		pmgInst.MonitorMailStats = monitorMailStats
		pmgInst.MonitorQueues = req.MonitorQueues
		pmgInst.MonitorQuarantine = req.MonitorQuarantine
		pmgInst.MonitorDomainStats = req.MonitorDomainStats
	} else {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Save configuration to disk using our persistence instance
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances, h.config.PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save nodes configuration")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// IMPORTANT: Preserve alert overrides when updating nodes
	// This fixes issue #440 where PBS alert thresholds were being reset
	// Alert overrides are stored separately from node configuration
	// and must be explicitly preserved during node updates
	if h.monitor != nil {
		// Load current alert configuration to preserve overrides
		alertConfig, err := h.persistence.LoadAlertConfig()
		if err == nil && alertConfig != nil {
			// For PBS nodes, we need to handle ID mapping
			// PBS monitoring uses "pbs-<name>" but config uses "pbs-<index>"
			// We need to preserve overrides by the monitoring ID
			if nodeType == "pbs" && index < len(h.config.PBSInstances) {
				pbsName := h.config.PBSInstances[index].Name
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
			h.monitor.GetAlertManager().UpdateConfig(*alertConfig)
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
	if h.monitor != nil && h.monitor.GetDiscoveryService() != nil {
		log.Info().Msg("Triggering discovery refresh after adding node")
		h.monitor.GetDiscoveryService().ForceRefresh()

		// Broadcast discovery update via WebSocket
		if h.wsHub != nil {
			// Wait a moment for discovery to complete
			go func() {
				time.Sleep(2 * time.Second)
				result, _ := h.monitor.GetDiscoveryService().GetCachedResult()
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
		Int("pveCount", len(h.config.PVEInstances)).
		Int("pbsCount", len(h.config.PBSInstances)).
		Int("pmgCount", len(h.config.PMGInstances)).
		Msg("Attempting to delete node")

	var deletedNodeHost string

	// Delete the node
	if nodeType == "pve" && index < len(h.config.PVEInstances) {
		deletedNodeHost = h.config.PVEInstances[index].Host
		log.Info().Str("nodeID", nodeID).Int("index", index).Msg("Deleting PVE node")
		h.config.PVEInstances = append(h.config.PVEInstances[:index], h.config.PVEInstances[index+1:]...)
	} else if nodeType == "pbs" && index < len(h.config.PBSInstances) {
		deletedNodeHost = h.config.PBSInstances[index].Host
		log.Info().Str("nodeID", nodeID).Int("index", index).Msg("Deleting PBS node")
		h.config.PBSInstances = append(h.config.PBSInstances[:index], h.config.PBSInstances[index+1:]...)
	} else if nodeType == "pmg" && index < len(h.config.PMGInstances) {
		deletedNodeHost = h.config.PMGInstances[index].Host
		log.Info().Str("nodeID", nodeID).Int("index", index).Msg("Deleting PMG node")
		h.config.PMGInstances = append(h.config.PMGInstances[:index], h.config.PMGInstances[index+1:]...)
	} else {
		log.Warn().
			Str("nodeID", nodeID).
			Str("nodeType", nodeType).
			Int("index", index).
			Int("pveCount", len(h.config.PVEInstances)).
			Int("pbsCount", len(h.config.PBSInstances)).
			Int("pmgCount", len(h.config.PMGInstances)).
			Msg("Node not found for deletion")
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Save configuration to disk using our persistence instance
	if err := h.persistence.SaveNodesConfigAllowEmpty(h.config.PVEInstances, h.config.PBSInstances, h.config.PMGInstances); err != nil {
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
			if h.monitor != nil && h.monitor.GetDiscoveryService() != nil {
				h.monitor.GetDiscoveryService().ForceRefresh()
				log.Info().Msg("Triggered background discovery refresh after node deletion")
			}
		}()
	}

	if deletedNodeType == "pve" && deletedNodeHost != "" {
		go h.triggerPVEHostCleanup(deletedNodeHost)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (h *ConfigHandlers) triggerPVEHostCleanup(host string) {
	client := tempproxy.NewClient()
	if client == nil || !client.IsAvailable() {
		log.Debug().
			Str("host", host).
			Msg("Skipping PVE cleanup request; sensor proxy socket unavailable")
		return
	}

	if err := client.RequestCleanup(host); err != nil {
		log.Warn().
			Err(err).
			Str("host", host).
			Msg("Failed to queue PVE host cleanup via sensor proxy")
		return
	}

	log.Info().
		Str("host", host).
		Msg("Queued PVE host cleanup via sensor proxy")
}

// HandleTestExistingNode tests a connection for an existing node using stored credentials
func (h *ConfigHandlers) HandleTestExistingNode(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimPrefix(r.URL.Path, "/api/config/nodes/")
	nodeID = strings.TrimSuffix(nodeID, "/test")
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

	// Get the node configuration
	var host, user, password, tokenName, tokenValue, fingerprint string
	var verifySSL bool

	if nodeType == "pve" && index < len(h.config.PVEInstances) {
		pve := h.config.PVEInstances[index]
		host = pve.Host
		user = pve.User
		password = pve.Password
		tokenName = pve.TokenName
		tokenValue = pve.TokenValue
		fingerprint = pve.Fingerprint
		verifySSL = pve.VerifySSL
	} else if nodeType == "pbs" && index < len(h.config.PBSInstances) {
		pbs := h.config.PBSInstances[index]
		host = pbs.Host
		user = pbs.User
		password = pbs.Password
		tokenName = pbs.TokenName
		tokenValue = pbs.TokenValue
		fingerprint = pbs.Fingerprint
		verifySSL = pbs.VerifySSL
	} else {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Test the connection based on type
	if nodeType == "pve" {
		clientConfig := proxmox.ClientConfig{
			Host:        host,
			User:        user,
			Password:    password,
			TokenName:   tokenName,
			TokenValue:  tokenValue,
			VerifySSL:   verifySSL,
			Fingerprint: fingerprint,
		}

		tempClient, err := proxmox.NewClient(clientConfig)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "create_client"), http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		nodes, err := tempClient.GetNodes(ctx)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "connection"), http.StatusBadRequest)
			return
		}

		response := map[string]interface{}{
			"status":    "success",
			"message":   fmt.Sprintf("Successfully connected to %d node(s)", len(nodes)),
			"nodeCount": len(nodes),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		// PBS test
		clientConfig := pbs.ClientConfig{
			Host:        host,
			User:        user,
			Password:    password,
			TokenName:   tokenName,
			TokenValue:  tokenValue,
			VerifySSL:   verifySSL,
			Fingerprint: fingerprint,
		}

		tempClient, err := pbs.NewClient(clientConfig)
		if err != nil {
			http.Error(w, sanitizeErrorMessage(err, "create_client"), http.StatusBadRequest)
			return
		}

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
	}
}

// HandleTestNodeConfig tests a node connection from provided configuration
func (h *ConfigHandlers) HandleTestNodeConfig(w http.ResponseWriter, r *http.Request) {
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
		clientConfig := proxmox.ClientConfig{
			Host:        req.Host,
			User:        authUser,
			Password:    req.Password,
			TokenName:   req.TokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   req.VerifySSL,
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
		clientConfig := pbs.ClientConfig{
			Host:        req.Host,
			User:        req.User,
			Password:    req.Password,
			TokenName:   req.TokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   req.VerifySSL,
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
					"message": fmt.Sprintf("Connected to PBS instance"),
					"latency": latency,
				}
			}
		}
	} else if req.Type == "pmg" {
		clientConfig := pmg.ClientConfig{
			Host:        req.Host,
			User:        req.User,
			Password:    req.Password,
			TokenName:   req.TokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   req.VerifySSL,
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

	if nodeType == "pve" && index < len(h.config.PVEInstances) {
		pve := h.config.PVEInstances[index]

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
	} else if nodeType == "pbs" && index < len(h.config.PBSInstances) {
		pbsInstance := h.config.PBSInstances[index]

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
	} else if nodeType == "pmg" && index < len(h.config.PMGInstances) {
		pmgInstance := h.config.PMGInstances[index]

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
func (h *ConfigHandlers) getNodeStatus(nodeType, nodeName string) string {
	if h.monitor == nil {
		if h.isRecentlyAutoRegistered(nodeType, nodeName) {
			return "connected"
		}
		return "disconnected"
	}

	// Get connection statuses from monitor
	connectionStatus := h.monitor.GetConnectionStatuses()

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
	persistedSettings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load persisted system settings")
		persistedSettings = &config.SystemSettings{}
	}

	// Get current values from running config
	settings := config.SystemSettings{
		// Note: PVE polling is hardcoded to 10s
		PBSPollingInterval:      int(h.config.PBSPollingInterval.Seconds()),
		BackupPollingInterval:   int(h.config.BackupPollingInterval.Seconds()),
		BackendPort:             h.config.BackendPort,
		FrontendPort:            h.config.FrontendPort,
		AllowedOrigins:          h.config.AllowedOrigins,
		ConnectionTimeout:       int(h.config.ConnectionTimeout.Seconds()),
		UpdateChannel:           h.config.UpdateChannel,
		AutoUpdateEnabled:       h.config.AutoUpdateEnabled,
		AutoUpdateCheckInterval: int(h.config.AutoUpdateCheckInterval.Hours()),
		AutoUpdateTime:          h.config.AutoUpdateTime,
		LogLevel:                h.config.LogLevel,                     // Include log level
		Theme:                   persistedSettings.Theme,               // Include theme from persisted settings
		AllowEmbedding:          persistedSettings.AllowEmbedding,      // Include allowEmbedding from persisted settings
		AllowedEmbedOrigins:     persistedSettings.AllowedEmbedOrigins, // Include allowedEmbedOrigins from persisted settings
		DiscoveryEnabled:        persistedSettings.DiscoveryEnabled,    // Include discoveryEnabled from persisted settings
		DiscoverySubnet:         persistedSettings.DiscoverySubnet,     // Include discoverySubnet from persisted settings
	}
	backupEnabled := h.config.EnableBackupPolling
	settings.BackupPollingEnabled = &backupEnabled

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// HandleVerifyTemperatureSSH tests SSH connectivity to nodes for temperature monitoring
func (h *ConfigHandlers) HandleVerifyTemperatureSSH(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	tempCollector := monitoring.NewTemperatureCollector("root", sshKeyPath)
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
		response.WriteString("For LXC deployments, consider installing pulse-sensor-proxy on the Proxmox host.\n")
		response.WriteString("See: https://docs.pulseapp.io for detailed SSH configuration options.\n")
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response.String()))
}

// isRunningInContainer detects if Pulse is running inside a container
func isRunningInContainer() bool {
	// Check for /.dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check cgroup for container indicators
	data, err := os.ReadFile("/proc/1/cgroup")
	if err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "lxc") || strings.Contains(content, "containerd") {
			return true
		}
	}

	return false
}

// HandleUpdateSystemSettingsOLD updates system settings in the unified config (DEPRECATED - use SystemSettingsHandler instead)
func (h *ConfigHandlers) HandleUpdateSystemSettingsOLD(w http.ResponseWriter, r *http.Request) {
	var settings config.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate PBS polling interval (must be positive)
	if settings.PBSPollingInterval < 0 {
		http.Error(w, "PBS polling interval must be positive", http.StatusBadRequest)
		return
	}
	if settings.BackupPollingInterval < 0 {
		http.Error(w, "Backup polling interval cannot be negative", http.StatusBadRequest)
		return
	}

	// Update polling intervals
	needsReload := false

	// Note: PVE polling is hardcoded to 10s, only PBS interval can be configured
	if settings.PBSPollingInterval > 0 {
		h.config.PBSPollingInterval = time.Duration(settings.PBSPollingInterval) * time.Second
		needsReload = true
	}
	if settings.BackupPollingInterval > 0 || (settings.BackupPollingInterval == 0 && h.config.BackupPollingInterval != 0) {
		h.config.BackupPollingInterval = time.Duration(settings.BackupPollingInterval) * time.Second
		needsReload = true
	}
	if settings.BackupPollingEnabled != nil {
		h.config.EnableBackupPolling = *settings.BackupPollingEnabled
		needsReload = true
	}

	// Trigger a monitor reload if intervals changed
	if needsReload && h.reloadFunc != nil {
		log.Info().
			Int("pbsInterval", settings.PBSPollingInterval).
			Msg("Triggering monitor reload for new PBS polling interval")
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor with new polling intervals")
			// Don't fail the request, the setting was saved
		}
	}

	// Update allowed origins if provided
	if settings.AllowedOrigins != "" {
		h.config.AllowedOrigins = settings.AllowedOrigins
		// Update WebSocket hub with new origins
		if h.wsHub != nil {
			origins := strings.Split(settings.AllowedOrigins, ",")
			for i := range origins {
				origins[i] = strings.TrimSpace(origins[i])
			}
			h.wsHub.SetAllowedOrigins(origins)
		}
	}

	// Update update-related settings
	if settings.UpdateChannel != "" {
		h.config.UpdateChannel = settings.UpdateChannel
	}
	h.config.AutoUpdateEnabled = settings.AutoUpdateEnabled
	if settings.AutoUpdateCheckInterval > 0 {
		h.config.AutoUpdateCheckInterval = time.Duration(settings.AutoUpdateCheckInterval) * time.Hour
	}
	if settings.AutoUpdateTime != "" {
		h.config.AutoUpdateTime = settings.AutoUpdateTime
	}

	// Save settings to persistence
	if err := h.persistence.SaveSystemSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to persist system settings")
		// Continue anyway - settings are applied in memory
	} else if h.reloadSystemSettingsFunc != nil {
		// Reload cached system settings after successful save
		h.reloadSystemSettingsFunc()
	}

	log.Info().
		Int("pbsPollingInterval", settings.PBSPollingInterval).
		Int("backendPort", settings.BackendPort).
		Int("frontendPort", settings.FrontendPort).
		Msg("Updated system settings in unified config")

	// Trigger monitor reload to apply new settings
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor after system settings update")
			// Continue anyway - settings are saved
		} else {
			log.Info().
				Int("pbsPollingInterval", settings.PBSPollingInterval).
				Msg("Monitor reloaded with new PBS polling interval")
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Settings updated successfully",
	})
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
	var req ExportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode export request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Passphrase == "" {
		http.Error(w, "Passphrase is required", http.StatusBadRequest)
		return
	}

	// Require strong passphrase (at least 12 characters)
	if len(req.Passphrase) < 12 {
		http.Error(w, "Passphrase must be at least 12 characters long", http.StatusBadRequest)
		return
	}

	// Export configuration
	exportedData, err := h.persistence.ExportConfig(req.Passphrase)
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
	var req ImportConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode import request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Passphrase == "" {
		http.Error(w, "Passphrase is required", http.StatusBadRequest)
		return
	}

	if req.Data == "" {
		http.Error(w, "Import data is required", http.StatusBadRequest)
		return
	}

	// Import configuration
	if err := h.persistence.ImportConfig(req.Data, req.Passphrase); err != nil {
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
	*h.config = *newConfig

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
	if h.monitor != nil {
		// Reload alert configuration
		if alertConfig, err := h.persistence.LoadAlertConfig(); err == nil {
			h.monitor.GetAlertManager().UpdateConfig(*alertConfig)
			log.Info().Msg("Reloaded alert configuration after import")
		} else {
			log.Warn().Err(err).Msg("Failed to reload alert configuration after import")
		}

		// Reload webhook configuration
		if webhooks, err := h.persistence.LoadWebhooks(); err == nil {
			// Clear existing webhooks and add new ones
			notificationMgr := h.monitor.GetNotificationManager()
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
		if emailConfig, err := h.persistence.LoadEmailConfig(); err == nil {
			h.monitor.GetNotificationManager().SetEmailConfig(*emailConfig)
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
		if discoveryService := h.monitor.GetDiscoveryService(); discoveryService != nil {
			result, updated := discoveryService.GetCachedResult()

			var updatedUnix int64
			var ageSeconds float64
			if !updated.IsZero() {
				updatedUnix = updated.Unix()
				ageSeconds = time.Since(updated).Seconds()
			}

			response := map[string]interface{}{
				"servers":      result.Servers,
				"errors":       result.Errors,
				"environment":  result.Environment,
				"cached":       true,
				"updated":      updatedUnix,
				"age":          ageSeconds,
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
		var req struct {
			Subnet   string `json:"subnet"`
			UseCache bool   `json:"use_cache"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if req.UseCache {
			if discoveryService := h.monitor.GetDiscoveryService(); discoveryService != nil {
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

		scanner := discovery.NewScanner()
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
	serverHost := query.Get("host")
	pulseURL := query.Get("pulse_url")                 // URL of the Pulse server for auto-registration
	backupPerms := query.Get("backup_perms") == "true" // Whether to add backup management permissions
	authToken := query.Get("auth_token")               // Temporary auth token for auto-registration

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
	}

	log.Info().
		Str("type", serverType).
		Str("host", serverHost).
		Bool("has_auth", h.config.AuthUser != "" || h.config.AuthPass != "" || h.config.HasAPITokens()).
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

	// Get or generate SSH public keys for temperature monitoring (both proxy and sensors)
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

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
   echo "Please run this script as root"
   exit 1
fi

# Check if running inside a container (LXC/Docker)
if [ -f /run/systemd/container ] || [ -f /.dockerenv ] || [ ! -z "${container:-}" ]; then
   echo ""
   echo "❌ ERROR: This script is running inside a container!"
   echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
   echo ""
   echo "This setup script must be run on the Proxmox VE host itself,"
   echo "not inside an LXC container or Docker container."
   echo ""
   echo "Please:"
   echo "  1. Exit this container (type 'exit')"
   echo "  2. Run the script directly on your Proxmox host"
   echo ""
   echo "The script needs access to 'pveum' commands which are only"
   echo "available on the Proxmox VE host system."
   echo ""
   exit 1
fi

# Check if pveum command exists
if ! command -v pveum &> /dev/null; then
   echo ""
   echo "❌ ERROR: 'pveum' command not found!"
   echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
   echo ""
   echo "This script must be run on a Proxmox VE host."
   echo "The 'pveum' command is required to create users and tokens."
   echo ""
   echo "If you're seeing this error, you might be:"
   echo "  • Running on a non-Proxmox system"
   echo "  • Inside an LXC container (exit and run on the host)"
   echo "  • On a PBS server (use the PBS setup script instead)"
   echo ""
   exit 1
fi

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
    echo "  • pulse-sensor-proxy service and systemd unit"
    echo "  • pulse-sensor-proxy system user"
    echo "  • All SSH keys from authorized_keys (current and legacy)"
    echo "  • LXC bind mounts from all container configs"
    echo "  • Pulse monitoring API tokens and user"
    echo "  • All Pulse-related files and directories"
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

    # Run cleanup helper to remove SSH keys from remote nodes
    if [ -x /usr/local/bin/pulse-sensor-cleanup.sh ]; then
        echo "  • Running cleanup helper..."
        /usr/local/bin/pulse-sensor-cleanup.sh 2>/dev/null || echo "  ℹ️  Cleanup helper completed"
        echo ""
    fi

    # Always run manual removal for local services and files
    if true; then
        # Stop and remove pulse-sensor services
        if command -v systemctl &> /dev/null; then
            if systemctl is-active --quiet pulse-sensor-proxy 2>/dev/null; then
                echo "  • Stopping pulse-sensor-proxy service..."
                systemctl stop pulse-sensor-proxy || true
            fi
            if systemctl is-enabled --quiet pulse-sensor-proxy 2>/dev/null; then
                echo "  • Disabling pulse-sensor-proxy service..."
                systemctl disable pulse-sensor-proxy || true
            fi
            if systemctl is-active --quiet pulse-sensor-cleanup.path 2>/dev/null; then
                echo "  • Stopping pulse-sensor-cleanup.path..."
                systemctl stop pulse-sensor-cleanup.path || true
            fi
            if systemctl is-enabled --quiet pulse-sensor-cleanup.path 2>/dev/null; then
                echo "  • Disabling pulse-sensor-cleanup.path..."
                systemctl disable pulse-sensor-cleanup.path || true
            fi
            if systemctl is-enabled --quiet pulse-sensor-cleanup.service 2>/dev/null; then
                echo "  • Disabling pulse-sensor-cleanup.service..."
                systemctl disable pulse-sensor-cleanup.service || true
            fi
            if [ -f /etc/systemd/system/pulse-sensor-proxy.service ] || \
               [ -f /etc/systemd/system/pulse-sensor-cleanup.service ] || \
               [ -f /etc/systemd/system/pulse-sensor-cleanup.path ]; then
                echo "  • Removing systemd unit files..."
                rm -f /etc/systemd/system/pulse-sensor-proxy.service
                rm -f /etc/systemd/system/pulse-sensor-cleanup.service
                rm -f /etc/systemd/system/pulse-sensor-cleanup.path
                systemctl daemon-reload || true
            fi
        fi

        # Remove pulse-sensor-proxy binary
        if [ -f /usr/local/bin/pulse-sensor-proxy ]; then
            echo "  • Removing pulse-sensor-proxy binary..."
            rm -f /usr/local/bin/pulse-sensor-proxy
        fi

        # Remove cleanup helper script
        if [ -f /usr/local/bin/pulse-sensor-cleanup.sh ]; then
            echo "  • Removing cleanup helper script..."
            rm -f /usr/local/bin/pulse-sensor-cleanup.sh
        fi

        # Remove pulse-sensor-proxy data directory
        if [ -d /var/lib/pulse-sensor-proxy ]; then
            echo "  • Removing pulse-sensor-proxy data directory..."
            rm -rf /var/lib/pulse-sensor-proxy
        fi

        # Remove pulse-sensor-proxy user
        if id -u pulse-sensor-proxy >/dev/null 2>&1; then
            echo "  • Removing pulse-sensor-proxy system user..."
            userdel pulse-sensor-proxy 2>/dev/null || true
        fi

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

        # Remove LXC bind mounts from all container configs
        if [ -d /etc/pve/lxc ]; then
            echo "  • Removing LXC bind mounts from container configs..."
            if compgen -G "/etc/pve/lxc/*.conf" > /dev/null; then
                for conf in /etc/pve/lxc/*.conf; do
                    if [ -f "$conf" ] && grep -q "pulse-sensor-proxy" "$conf" 2>/dev/null; then
                        sed -i '/pulse-sensor-proxy/d' "$conf" || true
                    fi
                done
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
echo "Checking for existing Pulse monitoring tokens from this Pulse server..."
OLD_TOKENS=$(pveum user token list pulse-monitor@pam 2>/dev/null | grep -E "│ pulse-${PULSE_IP_PATTERN}-[0-9]+" | awk -F'│' '{print $2}' | sed 's/^ *//;s/ *$//' || true)
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
pveum user add pulse-monitor@pam --comment "Pulse monitoring service" 2>/dev/null || echo "User already exists"

# Generate API token
echo "Generating API token..."

# Check if token already exists
TOKEN_EXISTED=false
if pveum user token list pulse-monitor@pam 2>/dev/null | grep -q "%s"; then
    TOKEN_EXISTED=true
    echo ""
    echo "================================================================"
    echo "WARNING: Token '%s' already exists!"
    echo "================================================================"
    echo ""
    echo "To create a new token, first remove the existing one:"
    echo "  pveum user token remove pulse-monitor@pam %s"
    echo ""
    echo "Or create a token with a different name:"
    echo "  pveum user token add pulse-monitor@pam %s-$(date +%%s) --privsep 0"
    echo ""
    echo "Then use the new token ID in Pulse (e.g., pulse-monitor@pam!%s-1234567890)"
    echo "================================================================"
    echo ""
else
    # Create token silently first
    TOKEN_OUTPUT=$(pveum user token add pulse-monitor@pam %s --privsep 0)
    
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

    # Try auto-registration
    echo "Registering node with Pulse..."

    # Use auth token from URL parameter (much simpler!)
    AUTH_TOKEN="%s"
    
    # Only proceed with auto-registration if we have an auth token
    if [ -n "$AUTH_TOKEN" ]; then
        # Get the server's hostname
        SERVER_HOSTNAME=$(hostname -f 2>/dev/null || hostname)
        SERVER_IP=$(hostname -I | awk '{print $1}')
        
        # Send registration to Pulse
        PULSE_URL="%s"
        
        # Check if host URL was provided
        HOST_URL="%s"
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
            echo "   Token ID: pulse-monitor@pam!%s"
            echo "   Token Value: [See above]"
            echo ""
            exit 1
        fi
        
        # Construct registration request with setup code
        # Build JSON carefully to preserve the exclamation mark
        REGISTER_JSON='{"type":"pve","host":"'"$HOST_URL"'","serverName":"'"$SERVER_HOSTNAME"'","tokenId":"pulse-monitor@pam!%s","tokenValue":"'"$TOKEN_VALUE"'","authToken":"'"$AUTH_TOKEN"'"}'

        # Send registration with setup code
        REGISTER_RESPONSE=$(echo "$REGISTER_JSON" | curl -s -X POST "$PULSE_URL/api/auto-register" \
            -H "Content-Type: application/json" \
            -d @- 2>&1)
    else
        echo "Warning: No authentication token provided"
        echo "Auto-registration skipped"
        AUTO_REG_SUCCESS=false
        REGISTER_RESPONSE=""
    fi

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
    echo ""
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

echo "Setting up additional permissions..."
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
    PRIV_STRING="${EXTRA_PRIVS[*]}"
    pveum role delete PulseMonitor 2>/dev/null || true
    if pveum role add PulseMonitor -privs "$PRIV_STRING" 2>/dev/null; then
        pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
        echo "  • Applied privileges: $PRIV_STRING"
    else
        echo "  • Failed to create PulseMonitor role with: $PRIV_STRING"
        echo "    Assign these privileges manually if Pulse reports permission errors."
    fi
else
    echo "  • No additional privileges detected. Pulse may show limited VM metrics."
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Temperature Monitoring Setup (Optional)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# SSH public keys embedded from Pulse server
# Proxy key: used for ProxyJump (unrestricted but limited to port forwarding)
# Sensors key: used for temperature collection (restricted to sensors -j command)
SSH_PROXY_PUBLIC_KEY="%s"
SSH_SENSORS_PUBLIC_KEY="%s"
SSH_PROXY_KEY_ENTRY="restrict,permitopen=\"*:22\" $SSH_PROXY_PUBLIC_KEY # pulse-proxyjump"
SSH_SENSORS_KEY_ENTRY="command=\"sensors -j\",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $SSH_SENSORS_PUBLIC_KEY # pulse-sensors"
TEMPERATURE_ENABLED=false

# Check if temperature proxy is available and override SSH key if it is
PROXY_KEY_URL="%s/api/system/proxy-public-key"
TEMPERATURE_PROXY_KEY=$(curl -s -f "$PROXY_KEY_URL" 2>/dev/null || echo "")
if [ -n "$TEMPERATURE_PROXY_KEY" ] && [[ "$TEMPERATURE_PROXY_KEY" =~ ^ssh-(rsa|ed25519) ]]; then
    # Proxy is available - use its key instead of container's key
    SSH_SENSORS_PUBLIC_KEY="$TEMPERATURE_PROXY_KEY"
    SSH_SENSORS_KEY_ENTRY="command=\"sensors -j\",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $TEMPERATURE_PROXY_KEY # pulse-sensor-proxy"
fi

# Detect if Pulse is running in a container BEFORE asking about temperature monitoring
PULSE_CTID=""
PULSE_IS_CONTAINERIZED=false
if command -v pct >/dev/null 2>&1; then
    # Extract Pulse IP from URL
    PULSE_IP=$(echo "%s" | sed -E 's|^https?://([^:/]+).*|\1|')

    # Find container with this IP
    if [[ "$PULSE_IP" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        # Check all containers for matching IP
        for CTID in $(pct list | awk 'NR>1 {print $1}'); do
            # Verify container is running before attempting connection
            # Note: status can be "running", "running (healthy)", or "running (unhealthy)"
            CT_STATUS=$(pct status "$CTID" 2>/dev/null || echo "")
            if ! echo "$CT_STATUS" | grep -q "running"; then
                continue
            fi

            # Get all container IPs (handles both IPv4 and IPv6)
            CT_IPS=$(pct exec "$CTID" -- hostname -I 2>/dev/null || printf '')

            # Check if any of the container's IPs match the Pulse IP
            for CT_IP in $CT_IPS; do
                if [ "$CT_IP" = "$PULSE_IP" ]; then
                    # Validate with pct config to ensure it's the right container
                    if pct config "$CTID" >/dev/null 2>&1; then
                        PULSE_CTID="$CTID"
                        PULSE_IS_CONTAINERIZED=true
                        break 2  # Break out of both loops
                    fi
                fi
            done
        done
    fi
fi

# Track whether temperature monitoring can work
TEMP_MONITORING_AVAILABLE=true

# If Pulse is containerized, try to install proxy automatically
if [ "$PULSE_IS_CONTAINERIZED" = true ] && [ -n "$PULSE_CTID" ]; then
    # Try automatic installation - proxy keeps SSH credentials on the host for security
    if true; then
        # Download installer script from Pulse server
        PROXY_INSTALLER="/tmp/install-sensor-proxy-$$.sh"
        INSTALLER_URL="%s/api/install/install-sensor-proxy.sh"

        echo "Installing pulse-sensor-proxy..."
        if curl --fail --silent --location \
            "$INSTALLER_URL" \
            -o "$PROXY_INSTALLER" 2>/dev/null; then
            chmod +x "$PROXY_INSTALLER"

            # Set fallback URL for installer to download binary from Pulse server
            export PULSE_SENSOR_PROXY_FALLBACK_URL="%s/api/install/pulse-sensor-proxy"

            # Run installer
            INSTALL_OUTPUT=$("$PROXY_INSTALLER" --ctid "$PULSE_CTID" --quiet 2>&1)
            INSTALL_STATUS=$?

            if [ -n "$INSTALL_OUTPUT" ]; then
                echo "$INSTALL_OUTPUT" | grep -E "✓|⚠️|ERROR" || true
            fi

            if [ $INSTALL_STATUS -eq 0 ]; then
                # Verify proxy health
                PROXY_HEALTHY=false
                if systemctl is-active --quiet pulse-sensor-proxy 2>/dev/null; then
                    PROXY_HEALTHY=true
                    echo ""
                    echo "✓ Secure proxy architecture enabled"
                    echo "  SSH keys are managed on the host for enhanced security"
                    echo ""
                fi

                # Configure socket bind mount and restart container automatically
                echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
                echo "Finalizing Setup"
                echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
                echo ""
                echo "Configuring socket bind mount for container $PULSE_CTID..."

                # Configure bind mount for proxy socket
                pct set "$PULSE_CTID" -mp0 /run/pulse-sensor-proxy,mp=/mnt/pulse-proxy

                # Check if container is currently running
                if pct status "$PULSE_CTID" 2>/dev/null | grep -q "running"; then
                    echo ""
                    echo "⚠️  Container $PULSE_CTID is currently running."
                    echo "    The proxy socket will be available after a restart."
                    echo ""
                    echo "    Restart manually when ready:"
                    echo "      pct stop $PULSE_CTID && sleep 2 && pct start $PULSE_CTID"
                    echo ""
                    echo "    Or the socket may be hot-plugged automatically (LXC 4.0+)"
                    echo ""
                else
                    echo "Container is stopped, starting it now..."

                    # Set up trap to restart container even if script is interrupted
                    trap "echo 'Starting container before exit...'; pct start $PULSE_CTID 2>/dev/null || true" EXIT INT TERM

                    pct start "$PULSE_CTID"

                    # Clear the trap after successful start
                    trap - EXIT INT TERM

                    echo "  ✓ Container started successfully"
                    echo ""
                fi
            else
                echo ""
                echo "⚠️  Proxy installation had issues - you may need to configure manually"
                if [ -n "$INSTALL_OUTPUT" ]; then
                    echo ""
                    echo "$INSTALL_OUTPUT" | tail -n 40
                    echo ""
                fi
            fi

            rm -f "$PROXY_INSTALLER"
        else
            # Proxy installer not available - configure automatic ProxyJump instead
            echo ""
            echo "ℹ️  Proxy not available - configuring automatic SSH ProxyJump"
            echo ""

            # Get the current Proxmox host's IP/hostname
            PROXY_JUMP_HOST=$(hostname)
            PROXY_JUMP_IP=$(hostname -I | awk '{print $1}')

            # We'll configure Pulse's SSH config to use this host as a jump point
            # This will be done when temperature monitoring is enabled
            CONFIGURE_PROXYJUMP=true
        fi
    fi
fi

# Check if SSH key is already configured
SSH_ALREADY_CONFIGURED=false
SSH_LEGACY_KEY=false

if [ -n "$SSH_PUBLIC_KEY" ] && [ -f /root/.ssh/authorized_keys ]; then
    if grep -qF "$SSH_RESTRICTED_KEY_ENTRY" /root/.ssh/authorized_keys 2>/dev/null; then
        SSH_ALREADY_CONFIGURED=true
    elif grep -qF "$SSH_PUBLIC_KEY" /root/.ssh/authorized_keys 2>/dev/null; then
        SSH_ALREADY_CONFIGURED=true
        SSH_LEGACY_KEY=true
    fi
fi

# Single temperature monitoring prompt
if [ "$SSH_ALREADY_CONFIGURED" = true ]; then
    TEMPERATURE_ENABLED=true
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Temperature monitoring is currently ENABLED"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "What would you like to do?"
    echo ""
    echo "  [1] Keep    - Leave temperature monitoring enabled (no changes)"
    echo "  [2] Remove  - Disable and remove SSH access"
    echo "  [3] Skip    - Skip this section"
    echo ""
    echo -n "Your choice [1/2/3]: "

    if [ -t 0 ]; then
        read -p "> " -n 1 -r SSH_ACTION
    else
        # When stdin is not a terminal (e.g., curl | bash), try /dev/tty first, then stdin for piped input
        if read -p "> " -n 1 -r SSH_ACTION </dev/tty 2>/dev/null; then
            :
        elif read -t 2 -n 1 -r SSH_ACTION 2>/dev/null && [ -n "$SSH_ACTION" ]; then
            echo "$SSH_ACTION"
        else
            echo "(No terminal available - keeping existing configuration)"
            SSH_ACTION="1"
        fi
    fi
    echo ""
    echo ""

    if [[ $SSH_ACTION == "2" ]]; then
        echo "Removing temperature monitoring configuration..."

        # Remove the SSH key from authorized_keys
        if [ -f /root/.ssh/authorized_keys ]; then
            grep -vF "$SSH_PUBLIC_KEY" /root/.ssh/authorized_keys > /root/.ssh/authorized_keys.tmp
            mv /root/.ssh/authorized_keys.tmp /root/.ssh/authorized_keys
            chmod 600 /root/.ssh/authorized_keys
            echo "  ✓ SSH key removed from authorized_keys"
        fi

        echo ""
        echo "Temperature monitoring has been disabled."
        echo "Note: lm-sensors package was NOT removed (in case you use it elsewhere)"
        TEMPERATURE_ENABLED=false
    elif [[ $SSH_ACTION == "3" ]]; then
        echo "Temperature monitoring configuration unchanged."
    else
        if [ "$SSH_LEGACY_KEY" = true ]; then
            echo "Updating Pulse SSH key to sensors-only access..."
            TMP_AUTH_KEYS=$(mktemp)
            if [ -f /root/.ssh/authorized_keys ]; then
                grep -vF "$SSH_PUBLIC_KEY" /root/.ssh/authorized_keys > "$TMP_AUTH_KEYS"
            fi
            echo "$SSH_RESTRICTED_KEY_ENTRY" >> "$TMP_AUTH_KEYS"
            mv "$TMP_AUTH_KEYS" /root/.ssh/authorized_keys
            chmod 600 /root/.ssh/authorized_keys
            echo "  ✓ SSH key restricted to sensors -j"
        else
            echo "Temperature monitoring configuration unchanged."
        fi
    fi
elif [ "$TEMP_MONITORING_AVAILABLE" = true ]; then
    # SECURITY: Block SSH-based temperature monitoring for containerized Pulse (unless dev mode)
    if [ "$PULSE_IS_CONTAINERIZED" = true ]; then
        # Check for dev mode override (from Pulse server environment)
        DEV_MODE_RESPONSE=$(curl -s "%s/api/health" 2>/dev/null | grep -o '"devModeSSH"[[:space:]]*:[[:space:]]*true' || echo "")

        if [ -n "$DEV_MODE_RESPONSE" ]; then
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "⚠️  DEV MODE: SSH Temperature Monitoring"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo ""
            echo "SSH key generation is ENABLED for testing/development."
            echo ""
            echo "WARNING: This grants root SSH access from the container!"
            echo "         NEVER use this in production environments."
            echo ""
            echo "To disable: Remove PULSE_DEV_ALLOW_CONTAINER_SSH from container env"
            echo ""
            # Allow the setup to continue
        else
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo "🔒 Temperature Monitoring - Security Notice"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            echo ""
            echo "⚠️  SSH-based temperature monitoring is DISABLED for containerized Pulse."
            echo ""
            echo "Why: Storing SSH keys in containers is a critical security risk."
            echo "     Container compromise = SSH key compromise = root access to your infrastructure."
            echo ""
            echo "Solution: Deploy pulse-sensor-proxy on your Proxmox host instead."
            echo ""
            echo "Installation:"
            echo "  1. Download: curl -o /usr/local/bin/pulse-sensor-proxy https://github.com/..."
            echo "  2. Make executable: chmod +x /usr/local/bin/pulse-sensor-proxy"
            echo "  3. Create systemd service (see docs)"
            echo "  4. Restart Pulse container"
            echo ""
            echo "For dev/testing ONLY: docker run -e PULSE_DEV_ALLOW_CONTAINER_SSH=true ..."
            echo "Documentation: https://docs.pulseapp.io/security/containerized-deployments"
            echo ""
            TEMPERATURE_ENABLED=false
        fi
    fi

    if [ "$PULSE_IS_CONTAINERIZED" = false ] || [ -n "$DEV_MODE_RESPONSE" ]; then
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

        if [ -n "$SSH_SENSORS_PUBLIC_KEY" ]; then
            # Add keys to root's authorized_keys
            mkdir -p /root/.ssh
            chmod 700 /root/.ssh

            # Remove any old pulse keys
            if [ -f /root/.ssh/authorized_keys ]; then
                grep -vF "# pulse-" /root/.ssh/authorized_keys > /root/.ssh/authorized_keys.tmp 2>/dev/null || touch /root/.ssh/authorized_keys.tmp
                mv /root/.ssh/authorized_keys.tmp /root/.ssh/authorized_keys
            fi

            # If this node is the ProxyJump host, add the proxy key
            if [ "$CONFIGURE_PROXYJUMP" = true ]; then
                echo "$SSH_PROXY_KEY_ENTRY" >> /root/.ssh/authorized_keys
                echo "  ✓ ProxyJump key configured (restricted to port forwarding)"
            fi

            # Always add the sensors key (for temperature collection)
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
                echo "✗ Temperature monitoring could not be enabled"
                echo "  Resolve the installation issues above and rerun this step."
            fi

            # Configure automatic ProxyJump if needed (for containerized Pulse)
            if [ "$CONFIGURE_PROXYJUMP" = true ] && [ -n "$PROXY_JUMP_HOST" ]; then
                echo ""
                echo "Configuring automatic SSH ProxyJump for containerized Pulse..."

                # Get list of all cluster nodes (or just this node if standalone)
                ALL_NODES="${PROXY_JUMP_HOST}"
                if command -v pvecm >/dev/null 2>&1; then
                    CLUSTER_OUTPUT=$(pvecm nodes 2>/dev/null || true)
                    if [ -n "$CLUSTER_OUTPUT" ]; then
                        CLUSTER_NODES=$(echo "$CLUSTER_OUTPUT" | awk 'NR>1 && $1 ~ /^[0-9]+$/ {print $3}')
                        if [ -n "$CLUSTER_NODES" ]; then
                            ALL_NODES="$CLUSTER_NODES"
                        fi
                    fi
                fi

                # Create SSH config with separate aliases for proxy and sensors
                # ${PROXY_JUMP_HOST}-proxy: uses proxy key for ProxyJump
                # ${PROXY_JUMP_HOST}: uses sensors key for temperature collection
                SSH_CONFIG="Host ${PROXY_JUMP_HOST}-proxy
    HostName ${PROXY_JUMP_IP}
    User root
    IdentityFile ~/.ssh/id_ed25519_proxy
    IdentitiesOnly yes
    StrictHostKeyChecking accept-new

Host ${PROXY_JUMP_HOST}
    HostName ${PROXY_JUMP_IP}
    User root
    IdentityFile ~/.ssh/id_ed25519_sensors
    IdentitiesOnly yes
    StrictHostKeyChecking accept-new
"

                # Add ProxyJump config for each cluster node
                for NODE in $ALL_NODES; do
                    if [ "$NODE" != "$PROXY_JUMP_HOST" ]; then
                        # Resolve node IP address (try getent, fallback to just the hostname)
                        NODE_IP=$(getent hosts "$NODE" 2>/dev/null | awk '{print $1}' | head -1)
                        if [ -z "$NODE_IP" ]; then
                            NODE_IP="$NODE"  # Fallback to hostname if resolution fails
                        fi

                        SSH_CONFIG="${SSH_CONFIG}
Host ${NODE}
    HostName ${NODE_IP}
    ProxyJump ${PROXY_JUMP_HOST}-proxy
    User root
    IdentityFile ~/.ssh/id_ed25519_sensors
    IdentitiesOnly yes
    StrictHostKeyChecking accept-new
"
                    fi
                done

                # Write SSH config to Pulse container
                # This will be written to /home/pulse/.ssh/config inside the container
                echo "$SSH_CONFIG" | curl -s -X POST "%s/api/system/ssh-config" \
                    -H "Content-Type: text/plain" \
                    -H "Authorization: Bearer %s" \
                    --data-binary @- > /dev/null 2>&1

                if [ $? -eq 0 ]; then
                    echo "  ✓ ProxyJump configured - temperature monitoring will work automatically"
                else
                    echo "  ⚠️  Could not configure ProxyJump automatically"
                fi
            fi
        else
            echo ""
            echo "⚠️  SSH key not available from Pulse server"
            echo "  Temperature monitoring cannot be configured automatically"
        fi
        else
            echo "Temperature monitoring skipped."
        fi
    fi  # End of non-containerized temperature monitoring
fi  # End of TEMP_MONITORING_AVAILABLE

# Offer to configure other Proxmox cluster nodes if temperature monitoring is enabled here
if [ "$TEMPERATURE_ENABLED" = true ] && command -v pvecm >/dev/null 2>&1 && command -v ssh >/dev/null 2>&1; then
    CLUSTER_OUTPUT=$(pvecm nodes 2>/dev/null || true)
    if [ -n "$CLUSTER_OUTPUT" ]; then
        LOCAL_NODE=$(hostname -s 2>/dev/null || hostname)
        CLUSTER_NODES=$(echo "$CLUSTER_OUTPUT" | awk 'NR>1 && $1 ~ /^[0-9]+$/ {print $3}')

        if [ -n "$CLUSTER_NODES" ]; then
            OTHER_NODES_LIST=()
            while read -r NODE_NAME; do
                if [ -n "$NODE_NAME" ] && [ "$NODE_NAME" != "$LOCAL_NODE" ]; then
                    # Avoid duplicates
                    SKIP_NODE=false
                    for EXISTING in "${OTHER_NODES_LIST[@]}"; do
                        if [ "$EXISTING" = "$NODE_NAME" ]; then
                            SKIP_NODE=true
                            break
                        fi
                    done
                    if [ "$SKIP_NODE" = false ]; then
                        OTHER_NODES_LIST+=("$NODE_NAME")
                    fi
                fi
            done <<< "$CLUSTER_NODES"

            if [ ${#OTHER_NODES_LIST[@]} -gt 0 ]; then
                echo ""
                echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
                echo "Cluster Node Configuration"
                echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
                echo ""
                echo "Detected additional Proxmox nodes in cluster:"
                for NODE in "${OTHER_NODES_LIST[@]}"; do
                    echo "  • $NODE"
                done
                echo ""
                echo "Configure temperature monitoring on these nodes as well?"
                echo -n "[y/N]: "

                if [ -t 0 ]; then
                    read -p "> " -n 1 -r REMOTE_REPLY
                else
                    if read -p "> " -n 1 -r REMOTE_REPLY </dev/tty 2>/dev/null; then
                        :
                    else
                        echo "(No terminal available - skipping remote configuration)"
                        REMOTE_REPLY="n"
                    fi
                fi
                echo ""
                echo ""

                if [[ $REMOTE_REPLY =~ ^[Yy]$ ]]; then
                    for NODE in "${OTHER_NODES_LIST[@]}"; do
                        echo "Configuring temperature monitoring on $NODE..."
                        if ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o ConnectTimeout=5 -o LogLevel=ERROR root@"$NODE" "bash -s" <<EOF
set -e
SSH_SENSORS_PUBLIC_KEY='$SSH_SENSORS_PUBLIC_KEY'
SSH_SENSORS_KEY_ENTRY='$SSH_SENSORS_KEY_ENTRY'
mkdir -p /root/.ssh
chmod 700 /root/.ssh
AUTH_KEYS=/root/.ssh/authorized_keys
# Remove any old pulse keys
if [ -f "\$AUTH_KEYS" ]; then
    grep -vF "# pulse-" "\$AUTH_KEYS" > "\$AUTH_KEYS.tmp" 2>/dev/null || touch "\$AUTH_KEYS.tmp"
    mv "\$AUTH_KEYS.tmp" "\$AUTH_KEYS"
fi
# Add sensors key (cluster nodes only need sensors key, not proxy key)
echo "\$SSH_SENSORS_KEY_ENTRY" >> "\$AUTH_KEYS"
chmod 600 "\$AUTH_KEYS"
if ! command -v sensors >/dev/null 2>&1; then
    echo "  - Installing lm-sensors..."
    export DEBIAN_FRONTEND=noninteractive
    APT_LOG=$(mktemp)
    if ! apt-get update -qq >"$APT_LOG" 2>&1; then
        echo "    ! apt-get update failed."
        if grep -qi "enterprise.proxmox.com" "$APT_LOG"; then
            echo "    - Detected Proxmox enterprise repository without subscription; switching to no-subscription repository."
            if [ -f /etc/apt/sources.list.d/pve-enterprise.list ]; then
                cp /etc/apt/sources.list.d/pve-enterprise.list /etc/apt/sources.list.d/pve-enterprise.list.pulsebak 2>/dev/null || true
                if grep -q "^[[:space:]]*deb" /etc/apt/sources.list.d/pve-enterprise.list; then
                    sed -i 's|^[[:space:]]*deb|# Pulse auto-disabled: deb|' /etc/apt/sources.list.d/pve-enterprise.list
                fi
            fi
            if [ ! -f /etc/apt/sources.list.d/pve-no-subscription.list ]; then
                CODENAME=$(. /etc/os-release 2>/dev/null && echo "$VERSION_CODENAME")
                if [ -z "$CODENAME" ]; then
                    CODENAME=$(lsb_release -cs 2>/dev/null || echo "bookworm")
                fi
                echo "deb http://download.proxmox.com/debian/pve $CODENAME pve-no-subscription" > /etc/apt/sources.list.d/pve-no-subscription.list
            fi
            if apt-get update -qq >>"$APT_LOG" 2>&1; then
                echo "    ✓ Switched to no-subscription repository."
            else
                echo "    ! apt-get update still failed after switching repositories."
            fi
        else
            echo "    ! apt-get update error was not recognized. Please review apt configuration on this node."
        fi
    fi
    if apt-get install -y -qq lm-sensors >/dev/null 2>&1; then
        sensors-detect --auto >/dev/null 2>&1 || true
        echo "  ✓ lm-sensors installed"
    else
        echo "  ! Failed to install lm-sensors automatically. Please resolve apt issues and rerun this script."
    fi
    rm -f "$APT_LOG"
else
    echo "  ✓ lm-sensors package verified"
fi
EOF
                        then
                            echo "  ✓ Temperature monitoring enabled on $NODE"
                        else
                            echo "  ✗ Failed to configure $NODE (check SSH/cluster connectivity)"
                        fi
                        echo ""
                    done

                    # Verify that Pulse can actually SSH to the configured nodes
                    echo ""
                    # Check if we're using the temperature proxy
                    # If proxy key was detected earlier, we're using proxy-based temperature monitoring
                    if [ -n "$TEMPERATURE_PROXY_KEY" ]; then
                        # Using proxy - verification not needed, proxy handles SSH
                        echo "✓ Temperature monitoring configured via pulse-sensor-proxy"
                        echo "  Temperature data will appear in the dashboard within 10 seconds"
                        echo ""
                    elif [ "$PULSE_IS_CONTAINERIZED" != true ]; then
                        # Non-containerized Pulse - can verify SSH directly
                        echo "Verifying temperature monitoring connectivity from Pulse..."
                        echo ""

                        CONFIGURED_NODES="${OTHER_NODES_LIST[@]}"
                        if [ "$TEMPERATURE_ENABLED" = true ]; then
                            # Add current node to the list
                            CONFIGURED_NODES="$(hostname) ${CONFIGURED_NODES}"
                        fi

                        VERIFY_RESPONSE=$(curl -s -X POST "%s/api/system/verify-temperature-ssh" \
                            -H "Content-Type: application/json" \
                            -H "Authorization: Bearer %s" \
                            -d "{\"nodes\": \"$CONFIGURED_NODES\"}" 2>/dev/null || echo "")

                        if [ -n "$VERIFY_RESPONSE" ]; then
                            echo "$VERIFY_RESPONSE"
                        else
                            echo "⚠️  Unable to verify SSH connectivity."
                            echo "   Temperature data will appear once SSH connectivity is configured."
                        fi
                        echo ""
                    else
                        # Containerized without proxy - temperature data will appear automatically
                        echo "✓ Temperature monitoring configured"
                        echo "  Note: Container cannot directly SSH to nodes"
                        echo "  Temperature data will appear once proxy is configured on the host"
                        echo ""
                    fi
                fi
            fi
        fi
    fi
fi

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# Standalone Node Configuration (for non-cluster nodes)
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# Check if this node is standalone (not in a cluster)
IS_STANDALONE=false
if ! command -v pvecm >/dev/null 2>&1 || ! pvecm status >/dev/null 2>&1; then
    IS_STANDALONE=true
fi

# If standalone and temperature monitoring was enabled, try to fetch proxy key
if [ "$IS_STANDALONE" = true ] && [ "$TEMPERATURE_ENABLED" = true ]; then
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Standalone Node Temperature Setup"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "Detected: This is a standalone node (not in a Proxmox cluster)"
    echo ""
    echo "For enhanced security with containerized Pulse, we'll fetch the"
    echo "temperature proxy's SSH key directly from your Pulse server."
    echo ""

    # Try to fetch the proxy's public key from Pulse server
    PROXY_KEY_URL="%s/api/system/proxy-public-key"
    echo "Fetching temperature proxy public key..."

    PROXY_PUBLIC_KEY=$(curl -s -f "$PROXY_KEY_URL" 2>/dev/null || echo "")

    if [ -n "$PROXY_PUBLIC_KEY" ] && [[ "$PROXY_PUBLIC_KEY" =~ ^ssh-(rsa|ed25519) ]]; then
        echo "  ✓ Retrieved proxy public key"

        # Build the forced command entry for the proxy key
        PROXY_RESTRICTED_KEY="command=\"sensors -j\",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty $PROXY_PUBLIC_KEY # pulse-proxy-key"

        # Check if key already exists
        if [ -f /root/.ssh/authorized_keys ] && grep -qF "$PROXY_PUBLIC_KEY" /root/.ssh/authorized_keys 2>/dev/null; then
            echo "  ✓ Proxy key already configured"
        else
            # Add the proxy key
            mkdir -p /root/.ssh
            chmod 700 /root/.ssh

            # Remove any old pulse-proxy-key entries first
            if [ -f /root/.ssh/authorized_keys ]; then
                grep -v '# pulse-proxy-key' /root/.ssh/authorized_keys > /root/.ssh/authorized_keys.tmp 2>/dev/null || true
                mv /root/.ssh/authorized_keys.tmp /root/.ssh/authorized_keys
            fi

            # Add the new proxy key
            echo "$PROXY_RESTRICTED_KEY" >> /root/.ssh/authorized_keys
            chmod 600 /root/.ssh/authorized_keys
            echo "  ✓ Temperature proxy key installed (restricted to sensors -j)"
        fi

        echo ""
        echo "✓ Standalone node temperature monitoring configured"
        echo "  The Pulse temperature proxy can now collect temperature data"
        echo "  from this node using secure SSH with forced commands."
        echo ""
    else
        echo "  ℹ️  Could not retrieve proxy public key from Pulse server"
        echo ""
        echo "This is normal if:"
        echo "  • Pulse is not running in a container (uses direct SSH)"
        echo "  • The temperature proxy service is not installed on the host"
        echo ""
        echo "Temperature monitoring will use the standard SSH key configured earlier."
        echo ""
    fi
fi

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
    echo "  Token ID: pulse-monitor@pam!%s"
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
`, serverName, time.Now().Format("2006-01-02 15:04:05"), pulseIP,
			tokenName, tokenName, tokenName, tokenName, tokenName, tokenName,
			authToken, pulseURL, serverHost, tokenName, tokenName, storagePerms, sshKeys.ProxyPublicKey, sshKeys.SensorsPublicKey, pulseURL, pulseURL, pulseURL, pulseURL, pulseURL, pulseURL, authToken, pulseURL, authToken, pulseURL, tokenName)

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
    
    # Use auth token from URL parameter (much simpler!)
    AUTH_TOKEN="%s"
    
    # Only proceed with auto-registration if we have an auth token
    if [ -n "$AUTH_TOKEN" ]; then
        # Get the server's hostname
        SERVER_HOSTNAME=$(hostname -f 2>/dev/null || hostname)
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
        echo "⚠️  No setup code provided - skipping auto-registration"
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

// generateSetupCode generates a 6-character alphanumeric code for one-time use
func (h *ConfigHandlers) generateSetupCode() string {
	// Use alphanumeric characters (excluding similar looking ones)
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// HandleSetupScriptURL generates a one-time setup code and URL for the setup script
func (h *ConfigHandlers) HandleSetupScriptURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	}
	h.codeMutex.Unlock()

	log.Info().
		Str("token_hash", tokenHash[:8]+"...").
		Time("expiry", expiry).
		Str("type", req.Type).
		Msg("Generated temporary auth token")

	// Build the URL with the token included
	host := r.Host

	if parsedHost, parsedPort, err := net.SplitHostPort(host); err == nil {
		if (parsedHost == "127.0.0.1" || parsedHost == "localhost") && parsedPort == strconv.Itoa(h.config.FrontendPort) {
			// Prefer a user-configured public URL when we're running on loopback.
			if publicURL := strings.TrimSpace(h.config.PublicURL); publicURL != "" {
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

	// Include the token directly in the URL - much simpler!
	scriptURL := fmt.Sprintf("%s/api/setup-script?type=%s%s&pulse_url=%s%s&auth_token=%s",
		pulseURL, req.Type, encodedHost, pulseURL, backupPerms, token)

	// Return a simple curl command - no environment variables needed
	// Don't include setupCode since it's already embedded in the URL
	response := map[string]interface{}{
		"url":     scriptURL,
		"command": fmt.Sprintf(`curl -sSL "%s" | bash`, scriptURL),
		"expires": expiry.Unix(),
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
		if h.monitor != nil {
			h.monitor.SetMockMode(*req.Enabled)
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

// AutoRegisterRequest represents a request from the setup script to auto-register a node
type AutoRegisterRequest struct {
	Type       string `json:"type"`                 // "pve" or "pbs"
	Host       string `json:"host"`                 // The host URL
	TokenID    string `json:"tokenId"`              // Full token ID like pulse-monitor@pam!pulse-token
	TokenValue string `json:"tokenValue,omitempty"` // The token value for the node
	ServerName string `json:"serverName"`           // Hostname or IP
	SetupCode  string `json:"setupCode,omitempty"`  // One-time setup code for authentication (deprecated)
	AuthToken  string `json:"authToken,omitempty"`  // Direct auth token from URL (new approach)
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
		Str("authToken", req.AuthToken).
		Str("authCode", authCode).
		Bool("hasConfigToken", h.config.HasAPITokens()).
		Msg("Checking authentication for auto-register")

	// First check for setup code/auth token in the request
	if authCode != "" {
		matchedAPIToken := false
		if h.config.HasAPITokens() {
			if _, ok := h.config.ValidateAPIToken(authCode); ok {
				authenticated = true
				matchedAPIToken = true
				log.Info().
					Str("type", req.Type).
					Str("host", req.Host).
					Msg("Auto-register authenticated via direct API token")
			}
		}

		if !matchedAPIToken {
			// Not the API token, check if it's a temporary setup code
			codeHash := internalauth.HashAPIToken(authCode)
			log.Debug().
				Str("authCode", authCode).
				Str("codeHash", codeHash[:8]+"...").
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
					// Allow the token to be reused for a brief grace period so the setup
					// script can complete follow-up actions (temperature verification, etc).
					graceExpiry := time.Now().Add(5 * time.Minute)
					if setupCode.ExpiresAt.After(graceExpiry) {
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
	if !authenticated && h.config.HasAPITokens() {
		apiToken := r.Header.Get("X-API-Token")
		if _, ok := h.config.ValidateAPIToken(apiToken); ok {
			authenticated = true
			log.Info().Msg("Auto-register authenticated via API token")
		}
	}

	// If still not authenticated and auth is required, reject
	// BUT: Always allow if a valid setup code/auth token was provided (even if expired/used)
	// This ensures the error message is accurate
	if !authenticated && h.config.HasAPITokens() && authCode == "" {
		log.Warn().Str("ip", r.RemoteAddr).Msg("Unauthorized auto-register attempt - no authentication provided")
		http.Error(w, "Pulse requires authentication", http.StatusUnauthorized)
		return
	} else if !authenticated && h.config.HasAPITokens() {
		// Had a code but it didn't validate
		log.Warn().Str("ip", r.RemoteAddr).Msg("Unauthorized auto-register attempt - invalid or expired setup code")
		http.Error(w, "Invalid or expired setup code", http.StatusUnauthorized)
		return
	}

	// Log source IP for security auditing
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded
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

	// Normalize the host URL
	host := req.Host
	if req.Type == "pve" {
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Add port if missing
		if !strings.Contains(host, ":8006") && !strings.Contains(host, ":443") {
			if strings.HasPrefix(host, "https://") {
				if !strings.Contains(host[8:], ":") {
					host += ":8006"
				}
			}
		}
	} else if req.Type == "pbs" {
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Add PBS port if missing
		protocolEnd := 0
		if strings.HasPrefix(host, "https://") {
			protocolEnd = 8
		} else if strings.HasPrefix(host, "http://") {
			protocolEnd = 7
		}
		// Only add default port if no port is specified
		if protocolEnd > 0 && !strings.Contains(host[protocolEnd:], ":") {
			host += ":8007"
		}
	}

	// Create a node configuration
	nodeConfig := NodeConfigRequest{
		Type:               req.Type,
		Name:               req.ServerName,
		Host:               host, // Use normalized host
		TokenName:          req.TokenID,
		TokenValue:         req.TokenValue,
		VerifySSL:          false, // Default to not verifying SSL for auto-registration
		MonitorVMs:         true,
		MonitorContainers:  true,
		MonitorStorage:     true,
		MonitorBackups:     true,
		MonitorDatastores:  true,
		MonitorSyncJobs:    true,
		MonitorVerifyJobs:  true,
		MonitorPruneJobs:   true,
		MonitorGarbageJobs: false,
	}

	// Check if a node with this host already exists
	existingIndex := -1
	if req.Type == "pve" {
		for i, node := range h.config.PVEInstances {
			if node.Host == host { // Use normalized host for comparison
				existingIndex = i
				break
			}
		}
	} else {
		for i, node := range h.config.PBSInstances {
			if node.Host == host { // Use normalized host for comparison
				existingIndex = i
				break
			}
		}
	}

	// If node exists, update it; otherwise add new
	if existingIndex >= 0 {
		// Update existing node
		if req.Type == "pve" {
			instance := &h.config.PVEInstances[existingIndex]
			// Clear password auth when switching to token auth
			instance.User = ""
			instance.Password = ""
			instance.TokenName = nodeConfig.TokenName
			instance.TokenValue = nodeConfig.TokenValue

			// Check for cluster if not already detected
			if !instance.IsCluster {
				clientConfig := proxmox.ClientConfig{
					Host:       instance.Host,
					TokenName:  nodeConfig.TokenName,
					TokenValue: nodeConfig.TokenValue,
					VerifySSL:  instance.VerifySSL,
				}

				isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, instance.Name)
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
			instance := &h.config.PBSInstances[existingIndex]
			// Clear password auth when switching to token auth
			instance.User = ""
			instance.Password = ""
			instance.TokenName = nodeConfig.TokenName
			instance.TokenValue = nodeConfig.TokenValue
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
			clientConfig := proxmox.ClientConfig{
				Host:       nodeConfig.Host,
				TokenName:  nodeConfig.TokenName,
				TokenValue: nodeConfig.TokenValue,
				VerifySSL:  nodeConfig.VerifySSL,
			}

			isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, nodeConfig.Name)

			newInstance := config.PVEInstance{
				Name:              nodeConfig.Name,
				Host:              nodeConfig.Host,
				TokenName:         nodeConfig.TokenName,
				TokenValue:        nodeConfig.TokenValue,
				VerifySSL:         nodeConfig.VerifySSL,
				MonitorVMs:        nodeConfig.MonitorVMs,
				MonitorContainers: nodeConfig.MonitorContainers,
				MonitorStorage:    nodeConfig.MonitorStorage,
				MonitorBackups:    nodeConfig.MonitorBackups,
				IsCluster:         isCluster,
				ClusterName:       clusterName,
				ClusterEndpoints:  clusterEndpoints,
			}
			h.config.PVEInstances = append(h.config.PVEInstances, newInstance)

			if isCluster {
				log.Info().
					Str("cluster", clusterName).
					Int("endpoints", len(clusterEndpoints)).
					Msg("Added Proxmox cluster via auto-registration")
			}
		} else {
			newInstance := config.PBSInstance{
				Name:               nodeConfig.Name,
				Host:               nodeConfig.Host,
				TokenName:          nodeConfig.TokenName,
				TokenValue:         nodeConfig.TokenValue,
				VerifySSL:          nodeConfig.VerifySSL,
				MonitorBackups:     true, // Enable by default for PBS
				MonitorDatastores:  nodeConfig.MonitorDatastores,
				MonitorSyncJobs:    nodeConfig.MonitorSyncJobs,
				MonitorVerifyJobs:  nodeConfig.MonitorVerifyJobs,
				MonitorPruneJobs:   nodeConfig.MonitorPruneJobs,
				MonitorGarbageJobs: nodeConfig.MonitorGarbageJobs,
			}
			h.config.PBSInstances = append(h.config.PBSInstances, newInstance)
		}
		log.Info().Str("host", req.Host).Str("type", req.Type).Msg("Added new node via auto-registration")
	}

	// Log what we're about to save
	if req.Type == "pve" && len(h.config.PVEInstances) > 0 {
		lastNode := h.config.PVEInstances[len(h.config.PVEInstances)-1]
		log.Info().
			Str("name", lastNode.Name).
			Str("host", lastNode.Host).
			Str("tokenName", lastNode.TokenName).
			Bool("hasTokenValue", lastNode.TokenValue != "").
			Msg("About to save PVE node")
	}

	// Save configuration
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances, h.config.PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("Configuration saved successfully")

	actualName := h.findInstanceNameByHost(req.Type, host)
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
	if h.monitor != nil && h.monitor.GetDiscoveryService() != nil {
		log.Info().Msg("Triggering discovery refresh after auto-registration")
		h.monitor.GetDiscoveryService().ForceRefresh()
	}

	// Broadcast auto-registration success via WebSocket
	fmt.Println("[AUTO-REGISTER] About to broadcast WebSocket message")
	if h.wsHub != nil {
		fmt.Println("[AUTO-REGISTER] WebSocket hub is available")
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
		fmt.Printf("[AUTO-REGISTER] Broadcasting message type: node_auto_registered for host: %s\n", req.Host)
		h.wsHub.BroadcastMessage(websocket.Message{
			Type:      "node_auto_registered",
			Data:      nodeInfo,
			Timestamp: time.Now().Format(time.RFC3339),
		})
		fmt.Println("[AUTO-REGISTER] Broadcast complete")

		// Also broadcast a discovery update to refresh the UI
		if h.monitor != nil && h.monitor.GetDiscoveryService() != nil {
			result, _ := h.monitor.GetDiscoveryService().GetCachedResult()
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
		fmt.Println("[AUTO-REGISTER] ERROR: WebSocket hub is nil!")
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
		hostname = strings.Replace(clientIP, ".", "-", -1)
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

	// Normalize the host URL
	host := req.Host
	if req.Type == "pve" {
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		if !strings.Contains(host, ":8006") && !strings.Contains(host, ":443") {
			if strings.HasPrefix(host, "https://") {
				host = strings.Replace(host, "https://", "", 1)
				if !strings.Contains(host, ":") {
					host = "https://" + host + ":8006"
				} else {
					host = "https://" + host
				}
			}
		}
	} else if req.Type == "pbs" {
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Check if port is missing
		protocolEnd := 0
		if strings.HasPrefix(host, "https://") {
			protocolEnd = 8
		} else if strings.HasPrefix(host, "http://") {
			protocolEnd = 7
		}
		// Only add default port if no port is specified
		if protocolEnd > 0 && !strings.Contains(host[protocolEnd:], ":") {
			host += ":8007"
		}
	}

	// Create the token on the remote server
	var fullTokenID string
	var createErr error

	if req.Type == "pve" {
		// For PVE, create token via API
		fullTokenID = fmt.Sprintf("pulse-monitor@pam!%s", tokenName)
		// Note: This would require implementing token creation in the proxmox package
		// For now, we'll return the token for the script to create
	} else if req.Type == "pbs" {
		// For PBS, create token via API
		fullTokenID = fmt.Sprintf("pulse-monitor@pbs!%s", tokenName)
		// Note: This would require implementing token creation in the pbs package
		// For now, we'll return the token for the script to create
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
			VerifySSL:         false,
			MonitorVMs:        true,
			MonitorContainers: true,
			MonitorStorage:    true,
			MonitorBackups:    true,
		}
		h.config.PVEInstances = append(h.config.PVEInstances, pveNode)
	} else if req.Type == "pbs" {
		pbsNode := config.PBSInstance{
			Name:              serverName,
			Host:              host,
			TokenName:         fullTokenID,
			TokenValue:        tokenValue,
			VerifySSL:         false,
			MonitorBackups:    true,
			MonitorDatastores: true,
			MonitorSyncJobs:   true,
			MonitorVerifyJobs: true,
			MonitorPruneJobs:  true,
		}
		h.config.PBSInstances = append(h.config.PBSInstances, pbsNode)
	}

	// Save configuration
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances, h.config.PMGInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	actualName := h.findInstanceNameByHost(req.Type, host)
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

// SSHKeyPair holds both proxy and sensors SSH keypairs
type SSHKeyPair struct {
	ProxyPublicKey   string
	SensorsPublicKey string
}

// getOrGenerateSSHKeys returns both SSH public keys (proxy + sensors) for temperature monitoring
// If keys don't exist, they are generated automatically
// SECURITY: Blocks key generation when running in containers - use pulse-sensor-proxy instead
func (h *ConfigHandlers) getOrGenerateSSHKeys() SSHKeyPair {
	// CRITICAL SECURITY CHECK: Never generate SSH keys in containers (unless dev mode)
	// Container compromise = SSH key compromise = root access to Proxmox
	devModeAllowSSH := os.Getenv("PULSE_DEV_ALLOW_CONTAINER_SSH") == "true"
	if isRunningInContainer() && !devModeAllowSSH {
		log.Error().Msg("SECURITY BLOCK: SSH key generation disabled in containerized deployments")
		log.Error().Msg("For temperature monitoring in containers, deploy pulse-sensor-proxy on the Proxmox host")
		log.Error().Msg("See: https://docs.pulseapp.io/security/containerized-deployments")
		log.Error().Msg("To test SSH keys in dev/lab only: PULSE_DEV_ALLOW_CONTAINER_SSH=true (NEVER in production!)")
		return SSHKeyPair{}
	}

	if devModeAllowSSH && isRunningInContainer() {
		log.Warn().Msg("⚠️  DEV MODE: SSH key generation ENABLED in container - FOR TESTING ONLY")
		log.Warn().Msg("⚠️  This grants root SSH access from container - NEVER use in production!")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Warn().Err(err).Msg("Could not determine home directory for SSH keys")
		return SSHKeyPair{}
	}

	sshDir := filepath.Join(homeDir, ".ssh")

	// Generate/load proxy key (for ProxyJump)
	proxyPrivPath := filepath.Join(sshDir, "id_ed25519_proxy")
	proxyPubPath := filepath.Join(sshDir, "id_ed25519_proxy.pub")
	proxyKey := h.generateOrLoadSSHKey(sshDir, proxyPrivPath, proxyPubPath, "proxy")

	// Generate/load sensors key (for temperature collection)
	sensorsPrivPath := filepath.Join(sshDir, "id_ed25519_sensors")
	sensorsPubPath := filepath.Join(sshDir, "id_ed25519_sensors.pub")
	sensorsKey := h.generateOrLoadSSHKey(sshDir, sensorsPrivPath, sensorsPubPath, "sensors")

	return SSHKeyPair{
		ProxyPublicKey:   proxyKey,
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
