package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// Router handles HTTP routing
type Router struct {
	mux        *http.ServeMux
	config     *config.Config
	monitor    *monitoring.Monitor
	wsHub      *websocket.Hub
	reloadFunc func() error
}


// NewRouter creates a new router instance
func NewRouter(cfg *config.Config, monitor *monitoring.Monitor, wsHub *websocket.Hub, reloadFunc func() error) http.Handler {
	r := &Router{
		mux:        http.NewServeMux(),
		config:     cfg,
		monitor:    monitor,
		wsHub:      wsHub,
		reloadFunc: reloadFunc,
	}

	r.setupRoutes()
	return r
}

// setupRoutes configures all routes
func (r *Router) setupRoutes() {
	// Create handlers
	alertHandlers := NewAlertHandlers(r.monitor)
	notificationHandlers := NewNotificationHandlers(r.monitor)
	configHandlers := NewConfigHandlers(r.config, r.monitor, r.reloadFunc)
	
	// API routes
	r.mux.HandleFunc("/api/health", r.handleHealth)
	r.mux.HandleFunc("/api/state", r.handleState)
	r.mux.HandleFunc("/api/version", r.handleVersion)
	r.mux.HandleFunc("/api/storage/", r.handleStorage)
	r.mux.HandleFunc("/api/storage-charts", r.handleStorageCharts)
	r.mux.HandleFunc("/api/charts", r.handleCharts)
	r.mux.HandleFunc("/api/diagnostics", r.handleDiagnostics)
	r.mux.HandleFunc("/api/config", r.handleConfig)
	r.mux.HandleFunc("/api/backups", r.handleBackups)
	r.mux.HandleFunc("/api/backups/", r.handleBackups)
	r.mux.HandleFunc("/api/backups/unified", r.handleBackups)
	r.mux.HandleFunc("/api/backups/pve", r.handleBackupsPVE)
	r.mux.HandleFunc("/api/backups/pbs", r.handleBackupsPBS)
	r.mux.HandleFunc("/api/snapshots", r.handleSnapshots)
	
	// Config management routes
	r.mux.HandleFunc("/api/config/nodes", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configHandlers.HandleGetNodes(w, r)
		case http.MethodPost:
			configHandlers.HandleAddNode(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	r.mux.HandleFunc("/api/config/nodes/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			configHandlers.HandleUpdateNode(w, r)
		case http.MethodDelete:
			configHandlers.HandleDeleteNode(w, r)
		case http.MethodPost:
			// Handle test endpoint
			if strings.HasSuffix(r.URL.Path, "/test") {
				configHandlers.HandleTestNode(w, r)
			} else {
				http.Error(w, "Not found", http.StatusNotFound)
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	// System settings routes
	r.mux.HandleFunc("/api/config/system", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configHandlers.HandleGetSystemSettings(w, r)
		case http.MethodPut:
			configHandlers.HandleUpdateSystemSettings(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	// Alert routes
	r.mux.HandleFunc("/api/alerts/", alertHandlers.HandleAlerts)
	
	// Notification routes
	r.mux.HandleFunc("/api/notifications/", notificationHandlers.HandleNotifications)

	// WebSocket endpoint
	r.mux.HandleFunc("/ws", r.handleWebSocket)
	
	// Socket.io compatibility endpoints
	r.mux.HandleFunc("/socket.io/", r.handleSocketIO)
	
	// Simple stats page
	r.mux.HandleFunc("/simple-stats", r.handleSimpleStats)
	
	// Serve static files from frontend build
	staticDir := "./frontend-modern/dist"
	fileServer := http.FileServer(http.Dir(staticDir))
	r.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Try to serve the file
		path := req.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		
		// Check if file exists
		if _, err := http.Dir(staticDir).Open(strings.TrimPrefix(path, "/")); err == nil {
			fileServer.ServeHTTP(w, req)
			return
		}
		
		// For SPA routing, serve index.html for non-existent paths
		if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/ws") && !strings.HasPrefix(path, "/socket.io/") {
			req.URL.Path = "/index.html"
			fileServer.ServeHTTP(w, req)
			return
		}
		
		// Otherwise, return 404
		http.NotFound(w, req)
	}))

}

// ServeHTTP implements http.Handler
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Add CORS headers if configured
	if r.config.AllowedOrigins != "" {
		w.Header().Set("Access-Control-Allow-Origin", r.config.AllowedOrigins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	}

	// Handle preflight requests
	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Add security headers for API endpoints
	if strings.HasPrefix(req.URL.Path, "/api/") || strings.HasPrefix(req.URL.Path, "/ws") {
		r.addSecurityHeaders(w)
	}

	// Log request
	start := time.Now()
	r.mux.ServeHTTP(w, req)
	log.Debug().
		Str("method", req.Method).
		Str("path", req.URL.Path).
		Dur("duration", time.Since(start)).
		Msg("Request handled")
}

// addSecurityHeaders adds security headers to the response
func (r *Router) addSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", r.config.IframeEmbeddingAllow)
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	
	// CSP header
	csp := "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://cdn.socket.io; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; img-src 'self' data: https:; connect-src 'self' ws: wss: http: https:; font-src 'self' https://cdn.jsdelivr.net;"
	w.Header().Set("Content-Security-Policy", csp)
}

// handleHealth handles health check requests
func (r *Router) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(r.monitor.GetStartTime()).Seconds(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleState handles state requests
func (r *Router) handleState(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check API token if configured
	if r.config.APIToken != "" {
		token := req.Header.Get("Authorization")
		if token == "" {
			token = req.URL.Query().Get("token")
		}
		if token != r.config.APIToken && token != "Bearer "+r.config.APIToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	state := r.monitor.GetState()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// handleVersion handles version requests
func (r *Router) handleVersion(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	version := map[string]interface{}{
		"version": "2.0.0-go",
		"build":   "development",
		"runtime": "go",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(version)
}

// handleStorage handles storage detail requests
func (r *Router) handleStorage(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract storage ID from path
	path := strings.TrimPrefix(req.URL.Path, "/api/storage/")
	if path == "" {
		http.Error(w, "Storage ID required", http.StatusBadRequest)
		return
	}

	// TODO: Implement storage details
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "Not implemented yet",
	})
}

// handleCharts handles chart data requests
func (r *Router) handleCharts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Get time range from query parameters
	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "1h"
	}
	
	// Convert time range to duration
	var duration time.Duration
	switch timeRange {
	case "5m":
		duration = 5 * time.Minute
	case "15m":
		duration = 15 * time.Minute
	case "30m":
		duration = 30 * time.Minute
	case "1h":
		duration = time.Hour
	case "4h":
		duration = 4 * time.Hour
	case "12h":
		duration = 12 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	default:
		duration = time.Hour
	}
	
	// Get current state from monitor
	state := r.monitor.GetState()
	
	// Create chart data structure that matches frontend expectations
	chartData := make(map[string]map[string][]map[string]interface{})
	nodeData := make(map[string]map[string][]map[string]interface{})
	
	currentTime := time.Now().Unix() * 1000 // JavaScript timestamp format
	oldestTimestamp := currentTime
	
	// Process VMs - get historical data
	for _, vm := range state.VMs {
		if chartData[vm.ID] == nil {
			chartData[vm.ID] = make(map[string][]map[string]interface{})
		}
		
		// Get historical metrics
		metrics := r.monitor.GetGuestMetrics(vm.ID, duration)
		
		// Convert metric points to API format
		for metricType, points := range metrics {
			chartData[vm.ID][metricType] = make([]map[string]interface{}, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				chartData[vm.ID][metricType][i] = map[string]interface{}{
					"timestamp": ts,
					"value": point.Value,
				}
			}
		}
		
		// If no historical data, add current value
		if len(chartData[vm.ID]["cpu"]) == 0 {
			chartData[vm.ID]["cpu"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": vm.CPU * 100},
			}
			chartData[vm.ID]["memory"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": vm.Memory.Usage},
			}
			chartData[vm.ID]["disk"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": vm.Disk.Usage},
			}
			chartData[vm.ID]["diskread"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": vm.DiskRead},
			}
			chartData[vm.ID]["diskwrite"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": vm.DiskWrite},
			}
			chartData[vm.ID]["netin"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": vm.NetworkIn},
			}
			chartData[vm.ID]["netout"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": vm.NetworkOut},
			}
		}
	}
	
	// Process Containers - get historical data  
	for _, ct := range state.Containers {
		if chartData[ct.ID] == nil {
			chartData[ct.ID] = make(map[string][]map[string]interface{})
		}
		
		// Get historical metrics
		metrics := r.monitor.GetGuestMetrics(ct.ID, duration)
		
		// Convert metric points to API format
		for metricType, points := range metrics {
			chartData[ct.ID][metricType] = make([]map[string]interface{}, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				chartData[ct.ID][metricType][i] = map[string]interface{}{
					"timestamp": ts,
					"value": point.Value,
				}
			}
		}
		
		// If no historical data, add current value
		if len(chartData[ct.ID]["cpu"]) == 0 {
			chartData[ct.ID]["cpu"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": ct.CPU * 100},
			}
			chartData[ct.ID]["memory"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": ct.Memory.Usage},
			}
			chartData[ct.ID]["disk"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": ct.Disk.Usage},
			}
			chartData[ct.ID]["diskread"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": ct.DiskRead},
			}
			chartData[ct.ID]["diskwrite"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": ct.DiskWrite},
			}
			chartData[ct.ID]["netin"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": ct.NetworkIn},
			}
			chartData[ct.ID]["netout"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": ct.NetworkOut},
			}
		}
	}
	
	// Process Storage - get historical data
	storageData := make(map[string]map[string][]map[string]interface{})
	for _, storage := range state.Storage {
		if storageData[storage.ID] == nil {
			storageData[storage.ID] = make(map[string][]map[string]interface{})
		}
		
		// Get historical metrics
		metrics := r.monitor.GetStorageMetrics(storage.ID, duration)
		
		// Convert usage metrics to chart format
		if usagePoints, ok := metrics["usage"]; ok && len(usagePoints) > 0 {
			// Convert MetricPoint slice to chart format
			storageData[storage.ID]["disk"] = make([]map[string]interface{}, len(usagePoints))
			for i, point := range usagePoints {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				storageData[storage.ID]["disk"][i] = map[string]interface{}{
					"timestamp": ts,
					"value": point.Value,
				}
			}
		} else {
			// Add current value if no historical data
			usagePercent := float64(0)
			if storage.Total > 0 {
				usagePercent = (float64(storage.Used) / float64(storage.Total)) * 100
			}
			storageData[storage.ID]["disk"] = []map[string]interface{}{
				{"timestamp": currentTime, "value": usagePercent},
			}
		}
	}
	
	// Process Nodes - get historical data
	for _, node := range state.Nodes {
		if nodeData[node.ID] == nil {
			nodeData[node.ID] = make(map[string][]map[string]interface{})
		}
		
		// Get historical metrics for each type
		for _, metricType := range []string{"cpu", "memory", "disk"} {
			points := r.monitor.GetNodeMetrics(node.ID, metricType, duration)
			nodeData[node.ID][metricType] = make([]map[string]interface{}, len(points))
			for i, point := range points {
				ts := point.Timestamp.Unix() * 1000
				if ts < oldestTimestamp {
					oldestTimestamp = ts
				}
				nodeData[node.ID][metricType][i] = map[string]interface{}{
					"timestamp": ts,
					"value": point.Value,
				}
			}
			
			// If no historical data, add current value
			if len(nodeData[node.ID][metricType]) == 0 {
				var value float64
				switch metricType {
				case "cpu":
					value = node.CPU * 100
				case "memory":
					value = node.Memory.Usage
				case "disk":
					value = node.Disk.Usage
				}
				nodeData[node.ID][metricType] = []map[string]interface{}{
					{"timestamp": currentTime, "value": value},
				}
			}
		}
	}

	response := map[string]interface{}{
		"data":        chartData,
		"nodeData":    nodeData,
		"storageData": storageData,
		"timestamp":   currentTime,
		"stats": map[string]interface{}{
			"oldestDataTimestamp": oldestTimestamp,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleStorageCharts handles storage chart data requests
func (r *Router) handleStorageCharts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := req.URL.Query()
	rangeMinutes := 60 // default 1 hour
	if rangeStr := query.Get("range"); rangeStr != "" {
		fmt.Sscanf(rangeStr, "%d", &rangeMinutes)
	}

	duration := time.Duration(rangeMinutes) * time.Minute
	state := r.monitor.GetState()
	
	// Build storage chart data
	storageData := make(map[string]interface{})
	
	for _, storage := range state.Storage {
		metrics := r.monitor.GetStorageMetrics(storage.ID, duration)
		
		storageData[storage.ID] = map[string]interface{}{
			"usage": metrics["usage"],
			"used":  metrics["used"],
			"total": metrics["total"],
			"avail": metrics["avail"],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(storageData); err != nil {
		log.Error().Err(err).Msg("Failed to encode storage chart data")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleDiagnostics handles diagnostics requests
func (r *Router) handleDiagnostics(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Collect diagnostic information
	state := r.monitor.GetState()
	
	// System info
	systemInfo := map[string]interface{}{
		"version":        "2.0.0-go",
		"build":          "production",
		"runtime":        "Go 1.21",
		"uptime":         time.Since(r.monitor.GetStartTime()).Seconds(),
		"polling_interval": r.config.PollingInterval.Seconds(),
	}
	
	// Connection health
	connectionStatus := make(map[string]interface{})
	for instance, health := range state.ConnectionHealth {
		connectionStatus[instance] = map[string]interface{}{
			"healthy": health,
			"type":    "pve",
		}
	}
	
	// PVE instances status
	pveStatus := make(map[string]interface{})
	// Use config since we removed GetPVEClients
	for _, inst := range r.config.PVEInstances {
		pveStatus[inst.Name] = map[string]interface{}{
			"configured": true,
			"monitoring": map[string]interface{}{
				"vms":        inst.MonitorVMs,
				"containers": inst.MonitorContainers,
				"storage":    inst.MonitorStorage,
			},
		}
	}
	
	// Metrics summary
	metricsInfo := map[string]interface{}{
		"nodes":      len(state.Nodes),
		"vms":        len(state.VMs),
		"containers": len(state.Containers),
		"storage":    len(state.Storage),
		"last_update": state.LastUpdate,
	}
	
	
	// WebSocket connections
	wsStatus := map[string]interface{}{
		"clients": r.wsHub.GetClientCount(),
		"healthy": true,
	}
	
	// Build full diagnostics response
	diagnostics := map[string]interface{}{
		"status":      "healthy",
		"timestamp":   time.Now().Unix(),
		"system":      systemInfo,
		"connections": connectionStatus,
		"pve_instances": pveStatus,
		"metrics":     metricsInfo,
		"websocket":   wsStatus,
		"features": map[string]bool{
			"historical_metrics": true,
			"guest_actions":      true,
			"notifications":      true,
			"auto_refresh":       true,
			"csv_export":         true,
			"dark_mode":          true,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(diagnostics)
}

// handleConfig handles configuration requests
func (r *Router) handleConfig(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Return public configuration
	config := map[string]interface{}{
		"pollingInterval":   r.config.PollingInterval.Seconds(),
		"csrfProtection":    false, // r.config.CSRFProtection - field not available
		"autoUpdateEnabled": false, // r.config.AutoUpdateEnabled - field not available
		"updateChannel":     "stable", // r.config.UpdateChannel - field not available
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}


// handleBackups handles backup requests
func (r *Router) handleBackups(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get current state
	state := r.monitor.GetState()
	
	// Return backup data structure
	backups := map[string]interface{}{
		"backupTasks": state.PVEBackups.BackupTasks,
		"storageBackups": state.PVEBackups.StorageBackups,
		"guestSnapshots": state.PVEBackups.GuestSnapshots,
		"pbsBackups": state.PBSBackups,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(backups)
}

// handleBackupsPVE handles PVE backup requests
func (r *Router) handleBackupsPVE(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get state and extract PVE backups
	state := r.monitor.GetState()
	
	// Return PVE backup data in expected format
	backups := state.PVEBackups.StorageBackups
	if backups == nil {
		backups = []models.StorageBackup{}
	}
	
	pveBackups := map[string]interface{}{
		"backups": backups,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pveBackups); err != nil {
		log.Error().Err(err).Msg("Failed to encode PVE backups response")
		// Return empty array as fallback
		w.Write([]byte(`{"backups":[]}`))
	}
}

// handleBackupsPBS handles PBS backup requests
func (r *Router) handleBackupsPBS(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get state and extract PBS backups
	state := r.monitor.GetState()
	
	// Return PBS backup data in expected format
	instances := state.PBSInstances
	if instances == nil {
		instances = []models.PBSInstance{}
	}
	
	pbsData := map[string]interface{}{
		"instances": instances,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pbsData); err != nil {
		log.Error().Err(err).Msg("Failed to encode PBS response")
		// Return empty array as fallback
		w.Write([]byte(`{"instances":[]}`))
	}
}

// handleSnapshots handles snapshot requests
func (r *Router) handleSnapshots(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get state and extract guest snapshots
	state := r.monitor.GetState()
	
	// Return snapshot data
	snaps := state.PVEBackups.GuestSnapshots
	if snaps == nil {
		snaps = []models.GuestSnapshot{}
	}
	
	snapshots := map[string]interface{}{
		"snapshots": snaps,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(snapshots); err != nil {
		log.Error().Err(err).Msg("Failed to encode snapshots response")
		// Return empty array as fallback
		w.Write([]byte(`{"snapshots":[]}`))
	}
}

// handleWebSocket handles WebSocket connections
func (r *Router) handleWebSocket(w http.ResponseWriter, req *http.Request) {
	r.wsHub.HandleWebSocket(w, req)
}

// handleSimpleStats serves a simple stats page
func (r *Router) handleSimpleStats(w http.ResponseWriter, req *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Simple Pulse Stats</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 20px;
            background: #f5f5f5;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            background: white;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th, td {
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background: #333;
            color: white;
            font-weight: bold;
            position: sticky;
            top: 0;
        }
        tr:hover {
            background: #f5f5f5;
        }
        .status {
            padding: 4px 8px;
            border-radius: 4px;
            color: white;
            font-size: 12px;
        }
        .running { background: #28a745; }
        .stopped { background: #dc3545; }
        #status {
            margin-bottom: 20px;
            padding: 10px;
            background: #e9ecef;
            border-radius: 4px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .update-indicator {
            display: inline-block;
            width: 10px;
            height: 10px;
            background: #28a745;
            border-radius: 50%;
            animation: pulse 0.5s ease-out;
        }
        @keyframes pulse {
            0% { transform: scale(1); opacity: 1; }
            50% { transform: scale(1.5); opacity: 0.7; }
            100% { transform: scale(1); opacity: 1; }
        }
        .update-timer {
            font-family: monospace;
            font-size: 14px;
            color: #666;
        }
        .metric {
            font-family: monospace;
            text-align: right;
        }
    </style>
</head>
<body>
    <h1>Simple Pulse Stats</h1>
    <div id="status">
        <div>
            <span id="status-text">Connecting...</span>
            <span class="update-indicator" id="update-indicator" style="display:none"></span>
        </div>
        <div class="update-timer" id="update-timer"></div>
    </div>
    
    <h2>Containers</h2>
    <table id="containers">
        <thead>
            <tr>
                <th>Name</th>
                <th>Status</th>
                <th>CPU %</th>
                <th>Memory</th>
                <th>Disk Read</th>
                <th>Disk Write</th>
                <th>Net In</th>
                <th>Net Out</th>
            </tr>
        </thead>
        <tbody></tbody>
    </table>

    <script>
        let ws;
        let lastUpdateTime = null;
        let updateCount = 0;
        let updateInterval = null;
        
        function formatBytes(bytes) {
            if (!bytes || bytes < 0) return '0 B/s';
            const units = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
            let i = 0;
            let value = bytes;
            while (value >= 1024 && i < units.length - 1) {
                value /= 1024;
                i++;
            }
            return value.toFixed(1) + ' ' + units[i];
        }
        
        function formatMemory(used, total) {
            const usedGB = (used / 1024 / 1024 / 1024).toFixed(1);
            const totalGB = (total / 1024 / 1024 / 1024).toFixed(1);
            const percent = ((used / total) * 100).toFixed(0);
            return usedGB + '/' + totalGB + ' GB (' + percent + '%)';
        }
        
        function updateTable(containers) {
            const tbody = document.querySelector('#containers tbody');
            tbody.innerHTML = '';
            
            containers.sort((a, b) => a.name.localeCompare(b.name));
            
            containers.forEach(ct => {
                const row = document.createElement('tr');
                row.innerHTML = 
                    '<td><strong>' + ct.name + '</strong></td>' +
                    '<td><span class="status ' + ct.status + '">' + ct.status + '</span></td>' +
                    '<td class="metric">' + (ct.cpu ? ct.cpu.toFixed(1) : '0.0') + '%</td>' +
                    '<td class="metric">' + formatMemory(ct.mem || 0, ct.maxmem || 1) + '</td>' +
                    '<td class="metric">' + formatBytes(ct.diskread) + '</td>' +
                    '<td class="metric">' + formatBytes(ct.diskwrite) + '</td>' +
                    '<td class="metric">' + formatBytes(ct.netin) + '</td>' +
                    '<td class="metric">' + formatBytes(ct.netout) + '</td>';
                tbody.appendChild(row);
            });
        }
        
        function updateTimer() {
            if (lastUpdateTime) {
                const secondsSince = Math.floor((Date.now() - lastUpdateTime) / 1000);
                document.getElementById('update-timer').textContent = 'Next update in: ' + (2 - (secondsSince % 2)) + 's';
            }
        }
        
        function connect() {
            const statusText = document.getElementById('status-text');
            const indicator = document.getElementById('update-indicator');
            statusText.textContent = 'Connecting to WebSocket...';
            
            ws = new WebSocket('ws://' + window.location.host + '/ws');
            
            ws.onopen = function() {
                statusText.textContent = 'Connected! Updates every 2 seconds';
                console.log('WebSocket connected');
                // Start the countdown timer
                if (updateInterval) clearInterval(updateInterval);
                updateInterval = setInterval(updateTimer, 100);
            };
            
            ws.onmessage = function(event) {
                try {
                    const msg = JSON.parse(event.data);
                    
                    if (msg.type === 'initialState' || msg.type === 'rawData') {
                        if (msg.data && msg.data.containers) {
                            updateCount++;
                            lastUpdateTime = Date.now();
                            
                            // Show update indicator with animation
                            indicator.style.display = 'inline-block';
                            indicator.style.animation = 'none';
                            setTimeout(() => {
                                indicator.style.animation = 'pulse 0.5s ease-out';
                            }, 10);
                            
                            statusText.textContent = 'Update #' + updateCount + ' at ' + new Date().toLocaleTimeString();
                            updateTable(msg.data.containers);
                        }
                    }
                } catch (err) {
                    console.error('Parse error:', err);
                }
            };
            
            ws.onclose = function(event) {
                statusText.textContent = 'Disconnected: ' + event.code + ' ' + event.reason + '. Reconnecting in 3s...';
                indicator.style.display = 'none';
                if (updateInterval) clearInterval(updateInterval);
                setTimeout(connect, 3000);
            };
            
            ws.onerror = function(error) {
                statusText.textContent = 'Connection error. Retrying...';
                console.error('WebSocket error:', error);
            };
        }
        
        // Start connection
        connect();
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}


// handleSocketIO handles socket.io requests
func (r *Router) handleSocketIO(w http.ResponseWriter, req *http.Request) {
	// For socket.io.js, redirect to CDN
	if strings.Contains(req.URL.Path, "socket.io.js") {
		http.Redirect(w, req, "https://cdn.socket.io/4.8.1/socket.io.min.js", http.StatusFound)
		return
	}
	
	// For other socket.io endpoints, use our WebSocket
	// This provides basic compatibility
	if strings.Contains(req.URL.RawQuery, "transport=websocket") {
		r.wsHub.HandleWebSocket(w, req)
		return
	}
	
	// For polling transport, return proper socket.io response
	// Socket.io v4 expects specific format
	if strings.Contains(req.URL.RawQuery, "transport=polling") {
		if strings.Contains(req.URL.RawQuery, "sid=") {
			// Already connected, return empty poll
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("6"))
		} else {
			// Initial handshake
			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			w.WriteHeader(http.StatusOK)
			// Send open packet with session ID and config
			sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
			response := fmt.Sprintf(`0{"sid":"%s","upgrades":["websocket"],"pingInterval":25000,"pingTimeout":60000}`, sessionID)
			w.Write([]byte(response))
		}
		return
	}
	
	// Default: redirect to WebSocket
	http.Redirect(w, req, "/ws", http.StatusFound)
}

