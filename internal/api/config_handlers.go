package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/discovery"
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
	wsHub        *websocket.Hub
}

// NewConfigHandlers creates a new ConfigHandlers instance
func NewConfigHandlers(cfg *config.Config, monitor *monitoring.Monitor, reloadFunc func() error, wsHub *websocket.Hub) *ConfigHandlers {
	return &ConfigHandlers{
		config:       cfg,
		persistence:  config.NewConfigPersistence(cfg.DataPath),
		monitor:      monitor,
		reloadFunc:   reloadFunc,
		wsHub:        wsHub,
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
	clusterStatus, err := tempClient.GetClusterStatus(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get cluster status")
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
		clientConfig := proxmox.ClientConfig{
			Host:        host,
			User:        req.User,
			Password:    req.Password,
			TokenName:   req.TokenName,
			TokenValue:  req.TokenValue,
			VerifySSL:   req.VerifySSL,
			Fingerprint: req.Fingerprint,
		}
		
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
		if !strings.Contains(host, ":8007") && !strings.Contains(host[8:], ":") {
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
		
		// Ensure PBS host has protocol and port
		host := req.Host
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		if !strings.Contains(host, ":8007") && !strings.Contains(host[8:], ":") {
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
	
	var script string
	
	if serverType == "pve" {
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

# Create monitoring user
echo "Creating monitoring user..."
pveum user add pulse-monitor@pam --comment "Pulse monitoring service" 2>/dev/null || echo "User already exists"

# Generate API token
echo "Generating API token..."

# Check if token already exists
TOKEN_EXISTED=false
if pveum user token list pulse-monitor@pam 2>/dev/null | grep -q "pulse-token"; then
    TOKEN_EXISTED=true
    echo ""
    echo "================================================================"
    echo "WARNING: Token 'pulse-token' already exists!"
    echo "================================================================"
    echo ""
    echo "To create a new token, first remove the existing one:"
    echo "  pveum user token remove pulse-monitor@pam pulse-token"
    echo ""
    echo "Or create a token with a different name:"
    echo "  pveum user token add pulse-monitor@pam pulse-token-$(date +%%s) --privsep 0"
    echo ""
    echo "Then use the new token ID in Pulse (e.g., pulse-monitor@pam!pulse-token-1234567890)"
    echo "================================================================"
    echo ""
else
    TOKEN_OUTPUT=$(pveum user token add pulse-monitor@pam pulse-token --privsep 0)
    echo ""
    echo "================================================================"
    echo "IMPORTANT: Copy the token value below - it's only shown once!"
    echo "================================================================"
    echo "$TOKEN_OUTPUT"
    echo "================================================================"
    echo ""
    
    # Extract the token value from the output (last field on the value line)
    TOKEN_VALUE=$(echo "$TOKEN_OUTPUT" | grep "│ value" | awk -F'│' '{print $3}' | tr -d ' ' | tail -1)
    
    # If we successfully got a token, send it back to Pulse
    if [ -n "$TOKEN_VALUE" ]; then
        echo ""
        echo "🔄 Auto-registering with Pulse..."
        
        # Get the server's hostname
        SERVER_HOSTNAME=$(hostname -f 2>/dev/null || hostname)
        
        # Send registration to Pulse
        PULSE_URL="%s"
        # Use jq or manual escaping to properly construct JSON
        REGISTER_JSON=$(cat <<EOF
{
  "type": "pve",
  "host": "%s",
  "tokenId": "pulse-monitor@pam!pulse-token",
  "tokenValue": "$TOKEN_VALUE",
  "serverName": "$SERVER_HOSTNAME"
}
EOF
        )
        # Remove newlines from JSON
        REGISTER_JSON=$(echo "$REGISTER_JSON" | tr -d '\n')
        
        # Debug output
        echo "📤 Sending registration to Pulse..."
        echo "   URL: $PULSE_URL/api/auto-register"
        echo "   JSON: $REGISTER_JSON"
        
        REGISTER_RESPONSE=$(curl -s -X POST "$PULSE_URL/api/auto-register" \
            -H "Content-Type: application/json" \
            -d "$REGISTER_JSON" 2>&1)
        
        if echo "$REGISTER_RESPONSE" | grep -q "success"; then
            echo "✅ Automatically registered with Pulse!"
            echo "   The node should now appear in your Pulse dashboard."
            echo ""
            echo "📌 Note: You may need to close the modal or refresh the page to see the new node."
        else
            echo "⚠️  Auto-registration failed. Manual configuration may be needed."
            echo "   Response: $REGISTER_RESPONSE"
        fi
        echo ""
    fi
fi

# Set up permissions
echo "Setting up permissions..."
pveum aclmod / -user pulse-monitor@pam -role PVEAuditor
pveum aclmod /storage -user pulse-monitor@pam -role PVEDatastoreAdmin

echo ""
echo "✅ Setup complete!"
echo ""
echo "Add this server to Pulse with:"
echo "  Token ID: pulse-monitor@pam!pulse-token"
if [ "$TOKEN_EXISTED" = true ]; then
    echo "  Token Value: [Use your existing token or create a new one as shown above]"
else
    echo "  Token Value: [Copy from above]"
fi
echo "  Host URL: %s"
echo ""
`, serverName, time.Now().Format("2006-01-02 15:04:05"), pulseURL, serverHost, serverHost)
		
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
TOKEN_OUTPUT=$(proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token 2>&1)
if echo "$TOKEN_OUTPUT" | grep -q "already exists"; then
    echo "WARNING: Token 'pulse-token' already exists!"
    echo ""
    echo "You can either:"
    echo "1. Delete the existing token first:"
    echo "   proxmox-backup-manager user delete-token pulse-monitor@pbs pulse-token"
    echo ""
    echo "2. Or create a token with a different name:"
    echo "   proxmox-backup-manager user generate-token pulse-monitor@pbs pulse-token-$(date +%%s)"
    echo ""
    echo "Then use the new token ID in Pulse (e.g., pulse-monitor@pbs!pulse-token-1234567890)"
else
    echo "$TOKEN_OUTPUT"
    
    # Extract the token value from PBS JSON output - look for the "value" field
    TOKEN_VALUE=$(echo "$TOKEN_OUTPUT" | grep '"value"' | sed 's/.*"value"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
    
    # If we successfully got a token, send it back to Pulse
    if [ -n "$TOKEN_VALUE" ]; then
        echo ""
        echo "🔄 Auto-registering with Pulse..."
        
        # Get the server's hostname
        SERVER_HOSTNAME=$(hostname -f 2>/dev/null || hostname)
        
        # Send registration to Pulse
        PULSE_URL="%s"
        # Use jq or manual escaping to properly construct JSON
        REGISTER_JSON=$(cat <<EOF
{
  "type": "pbs",
  "host": "%s",
  "tokenId": "pulse-monitor@pbs!pulse-token",
  "tokenValue": "$TOKEN_VALUE",
  "serverName": "$SERVER_HOSTNAME"
}
EOF
        )
        # Remove newlines from JSON
        REGISTER_JSON=$(echo "$REGISTER_JSON" | tr -d '\n')
        REGISTER_RESPONSE=$(curl -s -X POST "$PULSE_URL/api/auto-register" \
            -H "Content-Type: application/json" \
            -d "$REGISTER_JSON" 2>&1)
        
        if echo "$REGISTER_RESPONSE" | grep -q "success"; then
            echo "✅ Automatically registered with Pulse!"
            echo "   The node should now appear in your Pulse dashboard."
            echo ""
            echo "📌 Note: You may need to close the modal or refresh the page to see the new node."
        else
            echo "⚠️  Auto-registration failed. Manual configuration may be needed."
            echo "   Response: $REGISTER_RESPONSE"
        fi
        echo ""
    fi
fi
echo "================================================================"
echo ""

# Set up permissions
echo "Setting up permissions..."
proxmox-backup-manager acl update / Audit --auth-id pulse-monitor@pbs
proxmox-backup-manager acl update / Audit --auth-id 'pulse-monitor@pbs!pulse-token'

echo ""
echo "✅ Setup complete!"
echo ""
echo "Add this server to Pulse with:"
echo "  Token ID: pulse-monitor@pbs!pulse-token"
echo "  Token Value: [Check the output above for the token or instructions]"
echo "  Host URL: %s"
echo ""
`, serverName, time.Now().Format("2006-01-02 15:04:05"), pulseURL, serverHost, serverHost)
	}
	
	// Set headers for script download
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=pulse-setup-%s.sh", serverType))
	w.Write([]byte(script))
}

// AutoRegisterRequest represents a request from the setup script to auto-register a node
type AutoRegisterRequest struct {
	Type       string `json:"type"`       // "pve" or "pbs"
	Host       string `json:"host"`       // The host URL
	TokenID    string `json:"tokenId"`    // Full token ID like pulse-monitor@pam!pulse-token
	TokenValue string `json:"tokenValue"` // The actual token secret
	ServerName string `json:"serverName"` // Hostname or IP
}

// HandleAutoRegister receives token details from the setup script and auto-configures the node
func (h *ConfigHandlers) HandleAutoRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body first to debug
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read request body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	
	log.Info().Str("body", string(body)).Msg("Auto-register request received")
	
	var req AutoRegisterRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Error().Err(err).Str("body", string(body)).Msg("Failed to decode auto-register request")
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	
	log.Info().
		Str("type", req.Type).
		Str("host", req.Host).
		Str("tokenId", req.TokenID).
		Bool("hasTokenValue", req.TokenValue != "").
		Str("serverName", req.ServerName).
		Msg("Parsed auto-register request")

	// Validate required fields
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
		if !strings.Contains(host, ":8007") && !strings.Contains(host[8:], ":") {
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