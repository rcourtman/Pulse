package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	internalauth "github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
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
	config               *config.Config
	persistence          *config.ConfigPersistence
	monitor              *monitoring.Monitor
	reloadFunc           func() error
	wsHub                *websocket.Hub
	guestMetadataHandler *GuestMetadataHandler
	setupCodes           map[string]*SetupCode // Map of code hash -> setup code details
	codeMutex            sync.RWMutex          // Mutex for thread-safe code access
}

// NewConfigHandlers creates a new ConfigHandlers instance
func NewConfigHandlers(cfg *config.Config, monitor *monitoring.Monitor, reloadFunc func() error, wsHub *websocket.Hub, guestMetadataHandler *GuestMetadataHandler) *ConfigHandlers {
	h := &ConfigHandlers{
		config:               cfg,
		persistence:          config.NewConfigPersistence(cfg.DataPath),
		monitor:              monitor,
		reloadFunc:           reloadFunc,
		wsHub:                wsHub,
		guestMetadataHandler: guestMetadataHandler,
		setupCodes:           make(map[string]*SetupCode),
	}
	
	// Clean up expired codes periodically
	go h.cleanupExpiredCodes()
	
	return h
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
		h.codeMutex.Unlock()
	}
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

// NodeConfigRequest represents a request to add/update a node
type NodeConfigRequest struct {
	Type              string `json:"type"` // "pve" or "pbs"
	Name              string `json:"name"`
	Host              string `json:"host"`
	User              string `json:"user,omitempty"`
	Password          string `json:"password,omitempty"`
	TokenName         string `json:"tokenName,omitempty"`
	TokenValue        string `json:"tokenValue,omitempty"`
	Fingerprint       string `json:"fingerprint,omitempty"`
	VerifySSL         bool   `json:"verifySSL"`
	MonitorVMs        bool   `json:"monitorVMs,omitempty"`        // PVE only
	MonitorContainers bool   `json:"monitorContainers,omitempty"` // PVE only
	MonitorStorage    bool   `json:"monitorStorage,omitempty"`    // PVE only
	MonitorBackups     bool `json:"monitorBackups,omitempty"`     // PVE only
	MonitorDatastores  bool `json:"monitorDatastores,omitempty"`  // PBS only
	MonitorSyncJobs    bool `json:"monitorSyncJobs,omitempty"`    // PBS only
	MonitorVerifyJobs  bool `json:"monitorVerifyJobs,omitempty"`  // PBS only
	MonitorPruneJobs   bool `json:"monitorPruneJobs,omitempty"`   // PBS only
	MonitorGarbageJobs bool `json:"monitorGarbageJobs,omitempty"` // PBS only
}

// NodeResponse represents a node in API responses
type NodeResponse struct {
	ID                string `json:"id"`
	Type              string `json:"type"`
	Name              string `json:"name"`
	Host              string `json:"host"`
	User              string `json:"user,omitempty"`
	HasPassword       bool   `json:"hasPassword"`
	TokenName         string `json:"tokenName,omitempty"`
	HasToken          bool   `json:"hasToken"`
	Fingerprint       string `json:"fingerprint,omitempty"`
	VerifySSL         bool   `json:"verifySSL"`
	MonitorVMs        bool   `json:"monitorVMs,omitempty"`
	MonitorContainers bool   `json:"monitorContainers,omitempty"`
	MonitorStorage    bool   `json:"monitorStorage,omitempty"`
	MonitorBackups     bool `json:"monitorBackups,omitempty"`
	MonitorDatastores  bool `json:"monitorDatastores,omitempty"`
	MonitorSyncJobs    bool `json:"monitorSyncJobs,omitempty"`
	MonitorVerifyJobs  bool `json:"monitorVerifyJobs,omitempty"`
	MonitorPruneJobs   bool `json:"monitorPruneJobs,omitempty"`
	MonitorGarbageJobs bool                  `json:"monitorGarbageJobs,omitempty"`
	Status             string                `json:"status"` // "connected", "disconnected", "error"
	IsCluster          bool                  `json:"isCluster,omitempty"`
	ClusterName        string                `json:"clusterName,omitempty"`
	ClusterEndpoints   []config.ClusterEndpoint `json:"clusterEndpoints,omitempty"`
}

// detectPVECluster checks if a PVE node is part of a cluster and returns cluster information
func detectPVECluster(clientConfig proxmox.ClientConfig, nodeName string) (isCluster bool, clusterName string, clusterEndpoints []config.ClusterEndpoint) {
	tempClient, err := proxmox.NewClient(clientConfig)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create client for cluster detection")
		return false, "", nil
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Get full cluster status to find the actual cluster name
	// Note: This can cause certificate lookup errors on standalone nodes, but it's only done once during configuration
	clusterStatus, err := tempClient.GetClusterStatus(ctx)
	if err != nil {
		// This is expected for standalone nodes - they will return an error when accessing cluster endpoints
		log.Debug().Err(err).Msg("Could not get cluster status - likely a standalone node")
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
		
		for _, clusterNode := range clusterNodes {
			endpoint := config.ClusterEndpoint{
				NodeID:   clusterNode.ID,
				NodeName: clusterNode.Name,
				Host:     clusterNode.Name,
				Online:   clusterNode.Online == 1,
				LastSeen: time.Now(),
			}
			
			if clusterNode.IP != "" {
				endpoint.IP = clusterNode.IP
			}
			
			clusterEndpoints = append(clusterEndpoints, endpoint)
		}
		
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
		node := NodeResponse{
			ID:                generateNodeID("pve", i),
			Type:              "pve",
			Name:              pve.Name,
			Host:              pve.Host,
			User:              pve.User,
			HasPassword:       pve.Password != "",
			TokenName:         pve.TokenName,
			HasToken:          pve.TokenValue != "",
			Fingerprint:       pve.Fingerprint,
			VerifySSL:         pve.VerifySSL,
			MonitorVMs:        pve.MonitorVMs,
			MonitorContainers: pve.MonitorContainers,
			MonitorStorage:    pve.MonitorStorage,
			MonitorBackups:    pve.MonitorBackups,
			Status:            h.getNodeStatus("pve", pve.Name),
			IsCluster:         pve.IsCluster,
			ClusterName:       pve.ClusterName,
			ClusterEndpoints:  pve.ClusterEndpoints,
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

	return nodes
}

// HandleGetNodes returns all configured nodes
func (h *ConfigHandlers) HandleGetNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.GetAllNodesForAPI()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

// HandleAddNode adds a new node
func (h *ConfigHandlers) HandleAddNode(w http.ResponseWriter, r *http.Request) {
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

	// Validate request
	if req.Host == "" {
		http.Error(w, "Host is required", http.StatusBadRequest)
		return
	}
	
	if req.Type != "pve" && req.Type != "pbs" {
		http.Error(w, "Invalid node type", http.StatusBadRequest)
		return
	}

	// Check for authentication
	hasAuth := (req.User != "" && req.Password != "") || (req.TokenName != "" && req.TokenValue != "")
	if !hasAuth {
		http.Error(w, "Authentication credentials required", http.StatusBadRequest)
		return
	}

	// Add to appropriate list
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
		
		// Auto-generate name if not provided
		if req.Name == "" {
			// Extract hostname from URL
			nameHost := host
			if strings.HasPrefix(nameHost, "http://") {
				nameHost = strings.TrimPrefix(nameHost, "http://")
			}
			if strings.HasPrefix(nameHost, "https://") {
				nameHost = strings.TrimPrefix(nameHost, "https://")
			}
			// Remove port
			if colonIndex := strings.Index(nameHost, ":"); colonIndex != -1 {
				nameHost = nameHost[:colonIndex]
			}
			req.Name = nameHost
		}
		
		// Check if node is part of a cluster
		clientConfig := config.CreateProxmoxConfigFromFields(host, req.User, req.Password, req.TokenName, req.TokenValue, req.Fingerprint, req.VerifySSL)
		
		isCluster, clusterName, clusterEndpoints := detectPVECluster(clientConfig, req.Name)
		
		if isCluster {
			log.Info().
				Str("cluster", clusterName).
				Int("endpoints", len(clusterEndpoints)).
				Msg("Detected Proxmox cluster, auto-discovering all nodes")
		}
		
		pve := config.PVEInstance{
			Name:              req.Name,
			Host:              host, // Use normalized host
			User:              req.User,
			Password:          req.Password,
			TokenName:         req.TokenName,
			TokenValue:        req.TokenValue,
			Fingerprint:       req.Fingerprint,
			VerifySSL:         req.VerifySSL,
			MonitorVMs:        req.MonitorVMs,
			MonitorContainers: req.MonitorContainers,
			MonitorStorage:    req.MonitorStorage,
			MonitorBackups:    req.MonitorBackups,
			IsCluster:         isCluster,
			ClusterName:       clusterName,
			ClusterEndpoints:  clusterEndpoints,
		}
		h.config.PVEInstances = append(h.config.PVEInstances, pve)
		
		if isCluster {
			log.Info().
				Str("cluster", clusterName).
				Int("endpoints", len(clusterEndpoints)).
				Msg("Added Proxmox cluster with auto-discovered endpoints")
		}
	} else {
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
		
		// Auto-generate name if not provided
		if req.Name == "" {
			// Try to get the actual hostname from PBS
			discovered := false
			
			// Create a temporary PBS client to discover the hostname
			pbsConfig := pbs.ClientConfig{
				Host:        host,
				TokenName:   req.TokenName,
				TokenValue:  req.TokenValue,
				User:        req.User,
				Password:    req.Password,
				VerifySSL:   req.VerifySSL,
				Fingerprint: req.Fingerprint,
				Timeout:     5 * time.Second,
			}
			
			if tempClient, err := pbs.NewClient(pbsConfig); err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				
				if nodeName, err := tempClient.GetNodeName(ctx); err == nil && nodeName != "" {
					req.Name = nodeName
					discovered = true
					log.Info().Str("discoveredName", nodeName).Msg("Auto-discovered PBS hostname")
				}
			}
			
			// Fallback to extracting from URL if discovery failed
			if !discovered {
				nameHost := host
				if strings.HasPrefix(nameHost, "http://") {
					nameHost = strings.TrimPrefix(nameHost, "http://")
				}
				if strings.HasPrefix(nameHost, "https://") {
					nameHost = strings.TrimPrefix(nameHost, "https://")
				}
				// Remove port
				if colonIndex := strings.Index(nameHost, ":"); colonIndex != -1 {
					nameHost = nameHost[:colonIndex]
				}
				req.Name = nameHost
			}
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
			Name:              req.Name,
			Host:              host,
			User:              pbsUser,
			Password:          pbsPassword,
			TokenName:         pbsTokenName,
			TokenValue:        pbsTokenValue,
			Fingerprint:       req.Fingerprint,
			VerifySSL:         req.VerifySSL,
			MonitorBackups:     true, // Enable by default for PBS
			MonitorDatastores:  req.MonitorDatastores,
			MonitorSyncJobs:    req.MonitorSyncJobs,
			MonitorVerifyJobs:  req.MonitorVerifyJobs,
			MonitorPruneJobs:   req.MonitorPruneJobs,
			MonitorGarbageJobs: req.MonitorGarbageJobs,
		}
		h.config.PBSInstances = append(h.config.PBSInstances, pbs)
	}

	// Save configuration to disk using our persistence instance
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances); err != nil {
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

	if req.Type != "pve" && req.Type != "pbs" {
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
		clientConfig := proxmox.ClientConfig{
			Host:        host,
			User:        req.User,
			Password:    req.Password,
			TokenName:   req.TokenName,  // Pass the full token ID
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
			"status": "success",
			"message": fmt.Sprintf("Successfully connected to %d node(s)", len(nodes)),
			"isCluster": isCluster,
			"nodeCount": len(nodes),
		}
		
		if isCluster {
			response["clusterNodeCount"] = len(clusterEndpoints)
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
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
			"status": "success",
			"message": fmt.Sprintf("Successfully connected. Found %d datastore(s)", len(datastores)),
			"datastoreCount": len(datastores),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleUpdateNode updates an existing node
func (h *ConfigHandlers) HandleUpdateNode(w http.ResponseWriter, r *http.Request) {
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
		pve.Name = req.Name
		pve.Host = req.Host
		if req.User != "" {
			pve.User = req.User
		}
		if req.Password != "" {
			pve.Password = req.Password
		}
		if req.TokenName != "" {
			pve.TokenName = req.TokenName
		}
		if req.TokenValue != "" {
			pve.TokenValue = req.TokenValue
		}
		pve.Fingerprint = req.Fingerprint
		pve.VerifySSL = req.VerifySSL
		pve.MonitorVMs = req.MonitorVMs
		pve.MonitorContainers = req.MonitorContainers
		pve.MonitorStorage = req.MonitorStorage
		pve.MonitorBackups = req.MonitorBackups
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
		
		// Determine authentication method and clear the unused fields
		if req.TokenName != "" && req.TokenValue != "" {
			// Using token authentication - clear user/password
			pbs.User = ""
			pbs.Password = ""
			pbs.TokenName = req.TokenName
			pbs.TokenValue = req.TokenValue
		} else if req.TokenName != "" {
			// Token name provided without new value - keep existing token value
			pbs.User = ""
			pbs.Password = ""
			pbs.TokenName = req.TokenName
		} else if req.Password != "" {
			// Using password authentication - clear token fields
			pbs.TokenName = ""
			pbs.TokenValue = ""
			pbs.Password = req.Password
			// Ensure user has realm for PBS
			pbsUser := req.User
			if req.User != "" && !strings.Contains(req.User, "@") {
				pbsUser = req.User + "@pbs" // Default to @pbs realm if not specified
			}
			pbs.User = pbsUser
		} else if req.User != "" {
			// User provided without password - keep existing password if any
			pbs.TokenName = ""
			pbs.TokenValue = ""
			// Ensure user has realm for PBS
			pbsUser := req.User
			if !strings.Contains(req.User, "@") {
				pbsUser = req.User + "@pbs" // Default to @pbs realm if not specified
			}
			pbs.User = pbsUser
		}
		
		pbs.Fingerprint = req.Fingerprint
		pbs.VerifySSL = req.VerifySSL
		pbs.MonitorBackups = true // Enable by default for PBS
		pbs.MonitorDatastores = req.MonitorDatastores
		pbs.MonitorSyncJobs = req.MonitorSyncJobs
		pbs.MonitorVerifyJobs = req.MonitorVerifyJobs
		pbs.MonitorPruneJobs = req.MonitorPruneJobs
		pbs.MonitorGarbageJobs = req.MonitorGarbageJobs
	} else {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Save configuration to disk using our persistence instance
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances); err != nil {
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

	// Delete the node
	if nodeType == "pve" && index < len(h.config.PVEInstances) {
		h.config.PVEInstances = append(h.config.PVEInstances[:index], h.config.PVEInstances[index+1:]...)
	} else if nodeType == "pbs" && index < len(h.config.PBSInstances) {
		h.config.PBSInstances = append(h.config.PBSInstances[:index], h.config.PBSInstances[index+1:]...)
	} else {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Save configuration to disk using our persistence instance
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save nodes configuration")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Immediately trigger discovery scan BEFORE reloading monitor
	// This way we can get the deleted node's info for immediate discovery
	var deletedNodeHost string
	var deletedNodeType string = nodeType
	
	// Get the host info of the deleted node for quick re-discovery
	if nodeType == "pve" && index < len(h.config.PVEInstances) {
		deletedNodeHost = h.config.PVEInstances[index].Host
	} else if nodeType == "pbs" && index < len(h.config.PBSInstances) {
		deletedNodeHost = h.config.PBSInstances[index].Host
	}
	
	// Extract IP and port from the host URL for targeted discovery
	var targetIP string
	var targetPort int
	if deletedNodeHost != "" {
		// Parse the host URL to get IP and port
		hostURL := deletedNodeHost
		hostURL = strings.TrimPrefix(hostURL, "https://")
		hostURL = strings.TrimPrefix(hostURL, "http://")
		parts := strings.Split(hostURL, ":")
		if len(parts) > 0 {
			targetIP = parts[0]
			if len(parts) > 1 {
				fmt.Sscanf(parts[1], "%d", &targetPort)
			} else {
				if deletedNodeType == "pve" {
					targetPort = 8006
				} else {
					targetPort = 8007
				}
			}
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
		
		// If we know the deleted node's details, immediately add it to discovered list
		if targetIP != "" && targetPort > 0 {
			// Create a synthetic discovery result with just the deleted node
			immediateResult := map[string]interface{}{
				"servers": []map[string]interface{}{
					{
						"ip":       targetIP,
						"port":     targetPort,
						"type":     deletedNodeType,
						"version":  "Unknown",
						"hostname": targetIP,
					},
				},
				"errors":    []string{},
				"timestamp": time.Now().Unix(),
				"immediate": true, // Flag to indicate this is immediate, not from a full scan
			}
			
			// Immediately broadcast the deleted node as discovered
			h.wsHub.BroadcastMessage(websocket.Message{
				Type: "discovery_update",
				Data: immediateResult,
				Timestamp: time.Now().Format(time.RFC3339),
			})
			log.Info().
				Str("ip", targetIP).
				Int("port", targetPort).
				Str("type", deletedNodeType).
				Msg("Immediately added deleted node to discovery")
		}
		
		// Schedule a full discovery scan in the background
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

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
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
			"status": "success",
			"message": fmt.Sprintf("Successfully connected to %d node(s)", len(nodes)),
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
			"status": "success",
			"message": fmt.Sprintf("Successfully connected. Found %d datastore(s)", len(datastores)),
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
		clientConfig := proxmox.ClientConfig{
			Host:        req.Host,
			User:        req.User,
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
	} else {
		http.Error(w, "Invalid node type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
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
		clientConfig := proxmox.ClientConfig{
			Host:        pve.Host,
			User:        pve.User,
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
	} else {
		testResult = map[string]interface{}{
			"status":  "error",
			"message": "Node not found",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(testResult)
}

// getNodeStatus returns the connection status for a node
func (h *ConfigHandlers) getNodeStatus(nodeType, nodeName string) string {
	if h.monitor == nil {
		return "disconnected"
	}
	
	// Get connection statuses from monitor
	connectionStatus := h.monitor.GetConnectionStatuses()
	
	key := fmt.Sprintf("%s-%s", nodeType, nodeName)
	if connected, ok := connectionStatus[key]; ok && connected {
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
		PollingInterval:         int(h.config.PollingInterval.Seconds()),
		PVEPollingInterval:      int(h.config.PVEPollingInterval.Seconds()),
		PBSPollingInterval:      int(h.config.PBSPollingInterval.Seconds()),
		BackendPort:             h.config.BackendPort,
		FrontendPort:            h.config.FrontendPort,
		AllowedOrigins:          h.config.AllowedOrigins,
		ConnectionTimeout:       int(h.config.ConnectionTimeout.Seconds()),
		UpdateChannel:           h.config.UpdateChannel,
		AutoUpdateEnabled:       h.config.AutoUpdateEnabled,
		AutoUpdateCheckInterval: int(h.config.AutoUpdateCheckInterval.Hours()),
		AutoUpdateTime:          h.config.AutoUpdateTime,
		Theme:                   persistedSettings.Theme, // Include theme from persisted settings
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// HandleUpdateSystemSettingsOLD updates system settings in the unified config (DEPRECATED - use SystemSettingsHandler instead)
func (h *ConfigHandlers) HandleUpdateSystemSettingsOLD(w http.ResponseWriter, r *http.Request) {
	var settings config.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Update polling intervals
	needsReload := false
	
	// Handle PVE polling interval
	if settings.PVEPollingInterval > 0 {
		h.config.PVEPollingInterval = time.Duration(settings.PVEPollingInterval) * time.Second
		needsReload = true
	} else if settings.PollingInterval > 0 {
		// Fallback to legacy interval
		h.config.PVEPollingInterval = time.Duration(settings.PollingInterval) * time.Second
		needsReload = true
	}
	
	// Handle PBS polling interval
	if settings.PBSPollingInterval > 0 {
		h.config.PBSPollingInterval = time.Duration(settings.PBSPollingInterval) * time.Second
		needsReload = true
	} else if settings.PollingInterval > 0 {
		// Fallback to legacy interval
		h.config.PBSPollingInterval = time.Duration(settings.PollingInterval) * time.Second
		needsReload = true
	}
	
	// Keep legacy interval updated for compatibility
	if settings.PollingInterval > 0 {
		h.config.PollingInterval = time.Duration(settings.PollingInterval) * time.Second
	}
	
	// Trigger a monitor reload if intervals changed
	if needsReload && h.reloadFunc != nil {
		log.Info().
			Int("pveInterval", settings.PVEPollingInterval).
			Int("pbsInterval", settings.PBSPollingInterval).
			Msg("Triggering monitor reload for new polling intervals")
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
	}
	
	log.Info().
		Int("pollingInterval", settings.PollingInterval).
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
				Int("pollingInterval", settings.PollingInterval).
				Msg("Monitor reloaded with new polling interval")
		}
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"message": fmt.Sprintf("Polling interval updated to %d seconds", settings.PollingInterval),
		"pollingInterval": settings.PollingInterval,
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
				notificationMgr.DeleteWebhook(webhook.ID)
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
			
			// Add metadata about the cache
			response := map[string]interface{}{
				"servers":   result.Servers,
				"errors":    result.Errors,
				"cached":    true,
				"updated":   updated.Unix(),
				"age":       time.Since(updated).Seconds(),
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		
		// No discovery service available, return empty result
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"servers": []interface{}{},
			"errors":  []string{},
			"cached":  false,
		})
		return
		
	case http.MethodPost:
		// Parse request
		var req struct {
			Subnet string `json:"subnet"` // CIDR notation or "auto"
			UseCache bool `json:"use_cache"` // Whether to return cached results
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// If use_cache is true and we have cached results, return them
		if req.UseCache {
			if discoveryService := h.monitor.GetDiscoveryService(); discoveryService != nil {
				result, updated := discoveryService.GetCachedResult()
				
				response := map[string]interface{}{
					"servers":   result.Servers,
					"errors":    result.Errors,
					"cached":    true,
					"updated":   updated.Unix(),
					"age":       time.Since(updated).Seconds(),
				}
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		// Check if discovery service is available for triggering a refresh
		if discoveryService := h.monitor.GetDiscoveryService(); discoveryService != nil {
			// Update subnet if provided
			if req.Subnet != "" {
				discoveryService.SetSubnet(req.Subnet)
			}
			
			// Trigger a refresh
			discoveryService.ForceRefresh()
			
			// Wait a moment for scan to start, then return current cache
			time.Sleep(100 * time.Millisecond)
			
			result, updated := discoveryService.GetCachedResult()
			response := map[string]interface{}{
				"servers":   result.Servers,
				"errors":    result.Errors,
				"cached":    true,
				"scanning":  true,
				"updated":   updated.Unix(),
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Fallback to direct scan if no discovery service
		log.Info().Str("subnet", req.Subnet).Msg("Starting network discovery (fallback)")

		scanner := discovery.NewScanner()
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		result, err := scanner.DiscoverServers(ctx, req.Subnet)
		if err != nil {
			log.Error().Err(err).Msg("Discovery failed")
			http.Error(w, fmt.Sprintf("Discovery failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		
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
	pulseURL := query.Get("pulse_url") // URL of the Pulse server for auto-registration
	backupPerms := query.Get("backup_perms") == "true" // Whether to add backup management permissions
	authToken := query.Get("auth_token") // Temporary auth token for auto-registration
	
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
		Bool("has_auth", h.config.AuthUser != "" || h.config.AuthPass != "" || h.config.APIToken != "").
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
        REGISTER_JSON=$(cat <<EOF
{
  "type": "pve",
  "host": "$HOST_URL",
  "serverName": "$SERVER_HOSTNAME",
  "tokenId": "pulse-monitor@pam!%s",
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
            echo "⚠️  Auto-registration failed: Pulse requires authentication"
            echo ""
            if [ -z "$PULSE_API_TOKEN" ]; then
                echo "   To enable auto-registration, add your Pulse API token to the setup URL:"
                echo "   &api_token=YOUR_PULSE_API_TOKEN"
                echo ""
                echo "   You can find your API token in Pulse Settings → Security"
            else
                echo "   The provided API token was invalid. Please check your token."
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

# Method 2: Try to detect PVE version directly
PVE_VERSION=""
if command -v pveversion >/dev/null 2>&1; then
    # Extract major version (e.g., "9" from "pve-manager/9.0.5/...")
    PVE_VERSION=$(pveversion --verbose 2>/dev/null | grep "pve-manager" | awk -F'/' '{print $2}' | cut -d'.' -f1)
fi

if [ "$HAS_VM_MONITOR" = true ]; then
    # PVE 8 or below - VM.Monitor exists
    echo "Detected Proxmox 8 or below - adding VM.Monitor permission"
    pveum role delete PulseMonitor 2>/dev/null || true
    pveum role add PulseMonitor -privs VM.Monitor
    pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
    
    echo ""
    echo "ℹ️  Note for Proxmox 8 VM Disk Monitoring:"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "VM disk monitoring requires:"
    echo "  1. qemu-guest-agent installed and running in the VM"
    echo "  2. VM.Monitor permission (just added above)"
    echo ""
    echo "If you previously added this node before Pulse v4.7,"
    echo "you needed to re-run this script to add VM.Monitor permission."
    echo ""
else
    # PVE 9+ - VM.Monitor was removed
    echo "Detected Proxmox 9+ - VM.Monitor was removed"
    
    # For PVE 9, the PVEAuditor role should be sufficient as it includes VM.GuestAgent.Audit
    # However, some users report issues even with these permissions
    # This appears to be a Proxmox 9 limitation/bug with guest agent API access
    
    # Try to add Sys.Audit which replaced VM.Monitor for basic KVM access
    pveum role delete PulseMonitor 2>/dev/null || true
    if pveum role add PulseMonitor -privs "Sys.Audit" 2>/dev/null; then
        echo "Added Sys.Audit permission (replacement for VM.Monitor)"
        pveum aclmod / -user pulse-monitor@pam -role PulseMonitor
    fi
    
    echo ""
    echo "⚠️  IMPORTANT: Proxmox 9 VM Disk Usage Limitation"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "VM disk usage may show 0% on Proxmox 9 due to API token limitations."
    echo ""
    echo "This is a known Proxmox issue where guest agent data (get-fsinfo)"
    echo "is not accessible to API tokens, even with correct permissions."
    echo "Proxmox's own web UI also shows 0% for VM disk usage (bug #1373)."
    echo ""
    echo "Unfortunately, there are NO working workarounds for API tokens:"
    echo "  1. VM.Monitor was removed in PVE 9 (replaced with VM.GuestAgent.Audit)"
    echo "  2. Even with correct permissions, tokens can't access guest agent data"
    echo "  3. Container (LXC) disk usage works correctly with tokens"
    echo "  4. You'll have to accept 0% VM disk usage until Proxmox fixes this"
    echo ""
    echo "Note: qemu-guest-agent must be installed in VMs for any disk"
    echo "monitoring to work. The data exists but token access is restricted."
fi

echo ""
echo "✅ Setup complete!"
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
    echo "  Host URL: %s"
    echo ""
fi
`, serverName, time.Now().Format("2006-01-02 15:04:05"), pulseIP,
			tokenName, tokenName, tokenName, tokenName, tokenName, tokenName,
			authToken, pulseURL, serverHost, tokenName, tokenName, storagePerms, tokenName, serverHost)
		
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
            echo "⚠️  Auto-registration failed: Pulse requires authentication"
            echo ""
            if [ -z "$PULSE_API_TOKEN" ]; then
                echo "   To enable auto-registration, add your Pulse API token to the setup URL:"
                echo "   &api_token=YOUR_PULSE_API_TOKEN"
                echo ""
                echo "   You can find your API token in Pulse Settings → Security"
            else
                echo "   The provided API token was invalid. Please check your token."
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
	pulseURL := fmt.Sprintf("%s://%s", "http", r.Host)
	if r.TLS != nil {
		pulseURL = fmt.Sprintf("%s://%s", "https", r.Host)
	}
	
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
		"url":        scriptURL,
		"command":    fmt.Sprintf(`curl -sSL "%s" | bash`, scriptURL),
		"expires":    expiry.Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AutoRegisterRequest represents a request from the setup script to auto-register a node
type AutoRegisterRequest struct {
	Type       string `json:"type"`       // "pve" or "pbs"
	Host       string `json:"host"`       // The host URL
	TokenID    string `json:"tokenId"`    // Full token ID like pulse-monitor@pam!pulse-token
	TokenValue string `json:"tokenValue,omitempty"` // The token value for the node
	ServerName string `json:"serverName"` // Hostname or IP
	SetupCode  string `json:"setupCode,omitempty"` // One-time setup code for authentication (deprecated)
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
	
	// First check for setup code/auth token in the request
	if authCode != "" {
		codeHash := internalauth.HashAPIToken(authCode)
		log.Debug().
			Str("authCode", authCode).
			Str("codeHash", codeHash[:8]+"...").
			Msg("Checking auth token")
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
			log.Warn().Msg("Invalid setup code/token")
		}
		h.codeMutex.Unlock()
	}
	
	// If not authenticated via setup code, check API token if configured
	if !authenticated && h.config.APIToken != "" {
		apiToken := r.Header.Get("X-API-Token")
		// Config always has hashed token now (auto-hashed on load)
		if apiToken != "" && internalauth.CompareAPIToken(apiToken, h.config.APIToken) {
			authenticated = true
			log.Info().Msg("Auto-register authenticated via API token")
		}
	}
	
	// If still not authenticated and auth is required, reject
	// BUT: Always allow if a valid setup code/auth token was provided (even if expired/used)
	// This ensures the error message is accurate
	if !authenticated && h.config.APIToken != "" && authCode == "" {
		log.Warn().Str("ip", r.RemoteAddr).Msg("Unauthorized auto-register attempt - no authentication provided")
		http.Error(w, "Pulse requires authentication", http.StatusUnauthorized)
		return
	} else if !authenticated && h.config.APIToken != "" {
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
		Type:              req.Type,
		Name:              req.ServerName,
		Host:              host, // Use normalized host
		TokenName:         req.TokenID,
		TokenValue:        req.TokenValue,
		VerifySSL:         false, // Default to not verifying SSL for auto-registration
		MonitorVMs:        true,
		MonitorContainers: true,
		MonitorStorage:    true,
		MonitorBackups:    true,
		MonitorDatastores: true,
		MonitorSyncJobs:   true,
		MonitorVerifyJobs: true,
		MonitorPruneJobs:  true,
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
					Host:        instance.Host,
					TokenName:   nodeConfig.TokenName,
					TokenValue:  nodeConfig.TokenValue,
					VerifySSL:   instance.VerifySSL,
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
				Host:        nodeConfig.Host,
				TokenName:   nodeConfig.TokenName,
				TokenValue:  nodeConfig.TokenValue,
				VerifySSL:   nodeConfig.VerifySSL,
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
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}
	
	log.Info().Msg("Configuration saved successfully")

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
			"type":       req.Type,
			"host":       req.Host,
			"name":       req.ServerName,
			"tokenId":    req.TokenID,
			"hasToken":   true,
			"verifySSL":  false,
			"status":     "connected",
		}
		
		// Broadcast the auto-registration success
		fmt.Printf("[AUTO-REGISTER] Broadcasting message type: node_auto_registered for host: %s\n", req.Host)
		h.wsHub.BroadcastMessage(websocket.Message{
			Type: "node_auto_registered",
			Data: nodeInfo,
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
			Name:               serverName,
			Host:               host,
			TokenName:          fullTokenID,
			TokenValue:         tokenValue,
			VerifySSL:          false,
			MonitorBackups:     true,
			MonitorDatastores:  true,
			MonitorSyncJobs:    true,
			MonitorVerifyJobs:  true,
			MonitorPruneJobs:   true,
		}
		h.config.PBSInstances = append(h.config.PBSInstances, pbsNode)
	}
	
	// Save configuration
	if err := h.persistence.SaveNodesConfig(h.config.PVEInstances, h.config.PBSInstances); err != nil {
		log.Error().Err(err).Msg("Failed to save auto-registered node")
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}
	
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
	
