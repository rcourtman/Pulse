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
	MonitorGarbageJobs bool `json:"monitorGarbageJobs,omitempty"`
	Status            string `json:"status"` // "connected", "disconnected", "error"
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
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Name == "" || req.Host == "" {
		http.Error(w, "Name and host are required", http.StatusBadRequest)
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
		pve := config.PVEInstance{
			Name:              req.Name,
			Host:              req.Host,
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
		}
		h.config.PVEInstances = append(h.config.PVEInstances, pve)
	} else {
		pbs := config.PBSInstance{
			Name:              req.Name,
			Host:              req.Host,
			User:              req.User,
			Password:          req.Password,
			TokenName:         req.TokenName,
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
			pbs.User = req.User
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

	// Save configuration to disk
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
	settings := config.SystemSettings{
		PollingInterval: int(h.config.PollingInterval.Seconds()),
	}
	
	// Try to load saved settings
	if savedSettings, err := h.persistence.LoadSystemSettings(); err == nil && savedSettings != nil {
		settings = *savedSettings
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// HandleUpdateSystemSettings updates system settings
func (h *ConfigHandlers) HandleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	var settings config.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Save settings
	if err := h.persistence.SaveSystemSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save system settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}
	
	// Apply polling interval to config
	if settings.PollingInterval > 0 {
		h.config.PollingInterval = time.Duration(settings.PollingInterval) * time.Second
	}
	
	// Trigger monitor reload to apply new settings
	if h.reloadFunc != nil {
		if err := h.reloadFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor after system settings update")
			// Continue anyway - settings are saved
		}
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// generateNodeID creates a unique ID for a node
func generateNodeID(nodeType string, index int) string {
	return fmt.Sprintf("%s-%d", nodeType, index)
}