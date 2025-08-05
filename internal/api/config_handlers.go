package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// ConfigHandlers handles configuration-related API endpoints
type ConfigHandlers struct {
	config       *config.Config
	persistence  *config.ConfigPersistence
	monitor      *monitoring.Monitor
	reloadFunc   func() error
}

// NewConfigHandlers creates a new ConfigHandlers instance
func NewConfigHandlers(cfg *config.Config, monitor *monitoring.Monitor, reloadFunc func() error) *ConfigHandlers {
	return &ConfigHandlers{
		config:       cfg,
		persistence:  config.NewConfigPersistence(cfg.ConfigPath),
		monitor:      monitor,
		reloadFunc:   reloadFunc,
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

// HandleGetNodes returns all configured nodes
func (h *ConfigHandlers) HandleGetNodes(w http.ResponseWriter, r *http.Request) {
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
			ID:                generateNodeID("pbs", i),
			Type:              "pbs",
			Name:              pbs.Name,
			Host:              pbs.Host,
			User:              pbs.User,
			HasPassword:       pbs.Password != "",
			TokenName:         pbs.TokenName,
			HasToken:          pbs.TokenValue != "",
			Fingerprint:       pbs.Fingerprint,
			VerifySSL:         pbs.VerifySSL,
			MonitorDatastores:  pbs.MonitorDatastores,
			MonitorSyncJobs:    pbs.MonitorSyncJobs,
			MonitorVerifyJobs:  pbs.MonitorVerifyJobs,
			MonitorPruneJobs:   pbs.MonitorPruneJobs,
			MonitorGarbageJobs: pbs.MonitorGarbageJobs,
			Status:            h.getNodeStatus("pbs", pbs.Name),
		}
		nodes = append(nodes, node)
	}

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
	
	// Auto-generate name if not provided
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
		
		// Create a temporary client to check if it's part of a cluster
		clientConfig := proxmox.ClientConfig{
			Host:        host,
			User:        req.User,
			Password:    req.Password,
			TokenName:   req.TokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   req.VerifySSL,
			Fingerprint: req.Fingerprint,
		}
		
		tempClient, err := proxmox.NewClient(clientConfig)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusBadRequest)
			return
		}
		
		// Check if node is part of a cluster
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		var clusterEndpoints []config.ClusterEndpoint
		var isCluster bool
		var clusterName string
		
		clusterNodes, err := tempClient.GetClusterNodes(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to get cluster nodes")
		} else {
			log.Info().Int("cluster_nodes", len(clusterNodes)).Msg("Got cluster nodes")
		}
		
		if err == nil && len(clusterNodes) > 1 {
			// This is a cluster - auto-discover all nodes
			isCluster = true
			log.Info().
				Str("name", req.Name).
				Int("nodes", len(clusterNodes)).
				Msg("Detected Proxmox cluster, auto-discovering all nodes")
			
			for _, clusterNode := range clusterNodes {
				// Get the cluster name from any online node
				if clusterName == "" && clusterNode.Online == 1 {
					clusterName = clusterNode.Name
				}
				
				endpoint := config.ClusterEndpoint{
					NodeID:   clusterNode.ID,
					NodeName: clusterNode.Name,
					Host:     clusterNode.Name, // Will be resolved to IP if needed
					Online:   clusterNode.Online == 1,
					LastSeen: time.Now(),
				}
				
				// Try to get the IP address
				if clusterNode.IP != "" {
					endpoint.IP = clusterNode.IP
				}
				
				clusterEndpoints = append(clusterEndpoints, endpoint)
			}
			
			// Use the provided name as cluster name if we couldn't get one
			if clusterName == "" {
				clusterName = req.Name
			}
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
		// Parse PBS authentication details
		pbsUser := req.User
		pbsTokenName := req.TokenName
		
		// Handle different token input formats
		if req.TokenName != "" && req.TokenValue != "" {
			// Check if token name contains the full format (user@realm!tokenname)
			if strings.Contains(req.TokenName, "!") {
				parts := strings.Split(req.TokenName, "!")
				if len(parts) == 2 {
					// Extract user from token ID if not already provided
					if pbsUser == "" {
						pbsUser = parts[0]
					}
					pbsTokenName = parts[1]
				}
			}
		}
		
		// Ensure user has realm for PBS (if using user/password or token with user)
		if pbsUser != "" && !strings.Contains(pbsUser, "@") {
			pbsUser = pbsUser + "@pbs" // Default to @pbs realm if not specified
		}
		
		pbs := config.PBSInstance{
			Name:              req.Name,
			Host:              req.Host,
			User:              pbsUser,
			Password:          req.Password,
			TokenName:         pbsTokenName,
			TokenValue:        req.TokenValue,
			Fingerprint:       req.Fingerprint,
			VerifySSL:         req.VerifySSL,
			MonitorDatastores:  req.MonitorDatastores,
			MonitorSyncJobs:    req.MonitorSyncJobs,
			MonitorVerifyJobs:  req.MonitorVerifyJobs,
			MonitorPruneJobs:   req.MonitorPruneJobs,
			MonitorGarbageJobs: req.MonitorGarbageJobs,
		}
		h.config.PBSInstances = append(h.config.PBSInstances, pbs)
	}

	// Save configuration to disk
	if err := config.SaveConfig(h.config); err != nil {
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
			http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusBadRequest)
			return
		}
		
		// Try to get nodes to test connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		nodes, err := tempClient.GetNodes(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Connection failed: %v", err), http.StatusBadRequest)
			return
		}
		
		// Check if it's a cluster
		isCluster := false
		clusterNodes, err := tempClient.GetClusterNodes(ctx)
		if err == nil && len(clusterNodes) > 1 {
			isCluster = true
		}
		
		response := map[string]interface{}{
			"status": "success",
			"message": fmt.Sprintf("Successfully connected to %d node(s)", len(nodes)),
			"isCluster": isCluster,
			"nodeCount": len(nodes),
		}
		
		if isCluster {
			response["clusterNodeCount"] = len(clusterNodes)
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	} else {
		// Ensure host has protocol for PBS
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		// Add port if missing
		if !strings.Contains(host, ":8007") && !strings.Contains(host, ":443") {
			if strings.HasPrefix(host, "https://") {
				host = strings.Replace(host, "https://", "https://", 1)
				if !strings.Contains(host[8:], ":") {
					host += ":8007"
				}
			}
		}
		
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
			http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusBadRequest)
			return
		}
		
		// Try to get datastores to test connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		datastores, err := tempClient.GetDatastores(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Connection failed: %v", err), http.StatusBadRequest)
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
		pbs.Host = req.Host
		if req.User != "" {
			// Ensure user has realm for PBS
			pbsUser := req.User
			if !strings.Contains(req.User, "@") {
				pbsUser = req.User + "@pbs" // Default to @pbs realm if not specified
			}
			pbs.User = pbsUser
		}
		if req.Password != "" {
			pbs.Password = req.Password
		}
		if req.TokenName != "" {
			pbs.TokenName = req.TokenName
		}
		if req.TokenValue != "" {
			pbs.TokenValue = req.TokenValue
		}
		pbs.Fingerprint = req.Fingerprint
		pbs.VerifySSL = req.VerifySSL
		pbs.MonitorDatastores = req.MonitorDatastores
		pbs.MonitorSyncJobs = req.MonitorSyncJobs
		pbs.MonitorVerifyJobs = req.MonitorVerifyJobs
		pbs.MonitorPruneJobs = req.MonitorPruneJobs
		pbs.MonitorGarbageJobs = req.MonitorGarbageJobs
	} else {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Save configuration to disk
	if err := config.SaveConfig(h.config); err != nil {
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

	// Save configuration to disk
	if err := config.SaveConfig(h.config); err != nil {
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
			http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusBadRequest)
			return
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		nodes, err := tempClient.GetNodes(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Connection failed: %v", err), http.StatusBadRequest)
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
			http.Error(w, fmt.Sprintf("Failed to create client: %v", err), http.StatusBadRequest)
			return
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		datastores, err := tempClient.GetDatastores(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Connection failed: %v", err), http.StatusBadRequest)
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
				"message": fmt.Sprintf("Failed to create client: %v", err),
			}
		} else {
			// Test connection by getting nodes list
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			if nodes, err := client.GetNodes(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("Connection failed: %v", err),
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
				"message": fmt.Sprintf("Failed to create client: %v", err),
			}
		} else {
			// Test connection by getting datastores
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			if _, err := client.GetDatastores(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("Connection failed: %v", err),
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
				"message": fmt.Sprintf("Failed to create client: %v", err),
			}
		} else {
			// Test connection by getting nodes list
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			if nodes, err := client.GetNodes(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("Connection failed: %v", err),
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
				"message": fmt.Sprintf("Failed to create client: %v", err),
			}
		} else {
			// Test connection by getting datastores
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			if _, err := client.GetDatastores(ctx); err != nil {
				testResult = map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("Connection failed: %v", err),
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
	// Get current values from running config
	settings := config.SystemSettings{
		PollingInterval:   int(h.config.PollingInterval.Seconds()),
		BackendPort:       h.config.BackendPort,
		FrontendPort:      h.config.FrontendPort,
		AllowedOrigins:    h.config.AllowedOrigins,
		ConnectionTimeout: int(h.config.ConnectionTimeout.Seconds()),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// HandleUpdateSystemSettings updates system settings in the unified config
func (h *ConfigHandlers) HandleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	var settings config.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Update polling interval using our persistence
	if settings.PollingInterval > 0 {
		if err := config.UpdatePollingInterval(settings.PollingInterval); err != nil {
			log.Error().Err(err).Msg("Failed to save polling interval")
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}
		
		// Update the running config
		h.config.PollingInterval = time.Duration(settings.PollingInterval) * time.Second
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