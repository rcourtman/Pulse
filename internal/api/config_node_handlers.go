package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (h *ConfigHandlers) handleGetNodes(w http.ResponseWriter, r *http.Request) {
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

func (h *ConfigHandlers) handleAddNode(w http.ResponseWriter, r *http.Request) {
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

		if enforceNodeLimitForConfigRegistration(w, r.Context(), h.getConfig(r.Context()), h.getMonitor(r.Context())) {
			return
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
		if enforceNodeLimitForConfigRegistration(w, r.Context(), h.getConfig(r.Context()), h.getMonitor(r.Context())) {
			return
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
		if enforceNodeLimitForConfigRegistration(w, r.Context(), h.getConfig(r.Context()), h.getMonitor(r.Context())) {
			return
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

func (h *ConfigHandlers) handleTestConnection(w http.ResponseWriter, r *http.Request) {
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

func (h *ConfigHandlers) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
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

func (h *ConfigHandlers) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
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

func (h *ConfigHandlers) handleRefreshClusterNodes(w http.ResponseWriter, r *http.Request) {
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

func (h *ConfigHandlers) handleTestNodeConfig(w http.ResponseWriter, r *http.Request) {
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
		testResult = testProxmoxBackupConnection(req)
	} else if req.Type == "pmg" {
		testResult = testProxmoxMailGatewayConnection(req)
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

func (h *ConfigHandlers) handleTestNode(w http.ResponseWriter, r *http.Request) {
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

func testProxmoxBackupConnection(req NodeConfigRequest) map[string]interface{} {
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
		return map[string]interface{}{
			"status":  "error",
			"message": sanitizeErrorMessage(err, "create_client"),
		}
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.GetDatastores(ctx); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": sanitizeErrorMessage(err, "connection"),
		}
	}
	latency := time.Since(startTime).Milliseconds()
	return map[string]interface{}{
		"status":  "success",
		"message": "Connected to PBS instance",
		"latency": latency,
	}
}

func testProxmoxMailGatewayConnection(req NodeConfigRequest) map[string]interface{} {
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
		return map[string]interface{}{
			"status":  "error",
			"message": sanitizeErrorMessage(err, "create_client"),
		}
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.GetVersion(ctx); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": sanitizeErrorMessage(err, "connection"),
		}
	}
	latency := time.Since(startTime).Milliseconds()
	return map[string]interface{}{
		"status":  "success",
		"message": "Connected to PMG instance",
		"latency": latency,
	}
}
