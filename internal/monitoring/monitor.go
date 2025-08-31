package monitoring

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// PVEClientInterface defines the interface for PVE clients (both regular and cluster)
type PVEClientInterface interface {
	GetNodes(ctx context.Context) ([]proxmox.Node, error)
	GetNodeStatus(ctx context.Context, node string) (*proxmox.NodeStatus, error)
	GetVMs(ctx context.Context, node string) ([]proxmox.VM, error)
	GetContainers(ctx context.Context, node string) ([]proxmox.Container, error)
	GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error)
	GetAllStorage(ctx context.Context) ([]proxmox.Storage, error)
	GetBackupTasks(ctx context.Context) ([]proxmox.Task, error)
	GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error)
	GetVMSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error)
	GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error)
	GetVMStatus(ctx context.Context, node string, vmid int) (*proxmox.VMStatus, error)
	GetContainerStatus(ctx context.Context, node string, vmid int) (*proxmox.Container, error)
	GetClusterResources(ctx context.Context, resourceType string) ([]proxmox.ClusterResource, error)
	IsClusterMember(ctx context.Context) (bool, error)
	GetVMFSInfo(ctx context.Context, node string, vmid int) ([]proxmox.VMFileSystem, error)
}

// Monitor handles all monitoring operations
type Monitor struct {
	config           *config.Config
	state            *models.State
	pveClients       map[string]PVEClientInterface
	pbsClients       map[string]*pbs.Client
	mu               sync.RWMutex
	startTime        time.Time
	rateTracker      *RateTracker
	metricsHistory   *MetricsHistory
	alertManager     *alerts.Manager
	notificationMgr  *notifications.NotificationManager
	configPersist    *config.ConfigPersistence
	discoveryService *discovery.Service   // Background discovery service
	activePollCount  int32                // Number of active polling operations
	pollCounter      int64                // Counter for polling cycles
	authFailures     map[string]int       // Track consecutive auth failures per node
	lastAuthAttempt  map[string]time.Time // Track last auth attempt time
}

// safePercentage calculates percentage safely, returning 0 if divisor is 0
func safePercentage(used, total float64) float64 {
	if total == 0 {
		return 0
	}
	result := used / total * 100
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0
	}
	return result
}

// maxInt64 returns the maximum of two int64 values
func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// safeFloat ensures a float value is not NaN or Inf
func safeFloat(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

// sortContent sorts comma-separated content values for consistent display
func sortContent(content string) string {
	if content == "" {
		return ""
	}
	parts := strings.Split(content, ",")
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// GetConnectionStatuses returns the current connection status for all nodes
func (m *Monitor) GetConnectionStatuses() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]bool)

	// Check PVE clients
	for name, client := range m.pveClients {
		// Simple check - if we have a client, consider it connected
		// In reality, you'd want to check if recent API calls succeeded
		statuses["pve-"+name] = client != nil
	}

	// Check PBS clients
	for name, client := range m.pbsClients {
		statuses["pbs-"+name] = client != nil
	}

	return statuses
}

// New creates a new Monitor instance
func New(cfg *config.Config) (*Monitor, error) {
	m := &Monitor{
		config:           cfg,
		state:            models.NewState(),
		pveClients:       make(map[string]PVEClientInterface),
		pbsClients:       make(map[string]*pbs.Client),
		startTime:        time.Now(),
		rateTracker:      NewRateTracker(),
		metricsHistory:   NewMetricsHistory(1000, 24*time.Hour), // Keep up to 1000 points or 24 hours
		alertManager:     alerts.NewManager(),
		notificationMgr:  notifications.NewNotificationManager(),
		configPersist:    config.NewConfigPersistence(cfg.DataPath),
		discoveryService: nil, // Will be initialized in Start()
		authFailures:     make(map[string]int),
		lastAuthAttempt:  make(map[string]time.Time),
	}

	// Load saved configurations
	if alertConfig, err := m.configPersist.LoadAlertConfig(); err == nil {
		m.alertManager.UpdateConfig(*alertConfig)
		// Apply schedule settings to notification manager
		if alertConfig.Schedule.Cooldown > 0 {
			m.notificationMgr.SetCooldown(alertConfig.Schedule.Cooldown)
		}
		if alertConfig.Schedule.GroupingWindow > 0 {
			m.notificationMgr.SetGroupingWindow(alertConfig.Schedule.GroupingWindow)
		} else if alertConfig.Schedule.Grouping.Window > 0 {
			m.notificationMgr.SetGroupingWindow(alertConfig.Schedule.Grouping.Window)
		}
		m.notificationMgr.SetGroupingOptions(
			alertConfig.Schedule.Grouping.ByNode,
			alertConfig.Schedule.Grouping.ByGuest,
		)
	} else {
		log.Warn().Err(err).Msg("Failed to load alert configuration")
	}

	if emailConfig, err := m.configPersist.LoadEmailConfig(); err == nil {
		m.notificationMgr.SetEmailConfig(*emailConfig)
	} else {
		log.Warn().Err(err).Msg("Failed to load email configuration")
	}

	// Migrate webhooks if needed (from unencrypted to encrypted)
	if err := m.configPersist.MigrateWebhooksIfNeeded(); err != nil {
		log.Warn().Err(err).Msg("Failed to migrate webhooks")
	}
	
	if webhooks, err := m.configPersist.LoadWebhooks(); err == nil {
		for _, webhook := range webhooks {
			m.notificationMgr.AddWebhook(webhook)
		}
	} else {
		log.Warn().Err(err).Msg("Failed to load webhook configuration")
	}

	// Check if mock mode is enabled before initializing clients
	mockEnabled := os.Getenv("PULSE_MOCK_MODE") == "true"
	
	if mockEnabled {
		log.Info().Msg("Mock mode enabled - skipping PVE/PBS client initialization")
	} else {
		// Initialize PVE clients
		log.Info().Int("count", len(cfg.PVEInstances)).Msg("Initializing PVE clients")
		for _, pve := range cfg.PVEInstances {
		log.Info().
			Str("name", pve.Name).
			Str("host", pve.Host).
			Str("user", pve.User).
			Bool("hasToken", pve.TokenName != "").
			Msg("Configuring PVE instance")

		// Check if this is a cluster
		if pve.IsCluster && len(pve.ClusterEndpoints) > 0 {
			// Create cluster client
			endpoints := make([]string, 0, len(pve.ClusterEndpoints))
			for _, ep := range pve.ClusterEndpoints {
				// Use IP if available, otherwise use host
				host := ep.IP
				if host == "" {
					host = ep.Host
				}

				// Skip if no host information
				if host == "" {
					log.Warn().
						Str("node", ep.NodeName).
						Msg("Skipping cluster endpoint with no host/IP")
					continue
				}

				// Ensure we have the full URL
				if !strings.HasPrefix(host, "http") {
					if pve.VerifySSL {
						host = fmt.Sprintf("https://%s:8006", host)
					} else {
						host = fmt.Sprintf("https://%s:8006", host)
					}
				}
				endpoints = append(endpoints, host)
			}

			// If no valid endpoints, fall back to single node mode
			if len(endpoints) == 0 {
				log.Warn().
					Str("instance", pve.Name).
					Msg("No valid cluster endpoints found, falling back to single node mode")
				endpoints = []string{pve.Host}
				if !strings.HasPrefix(endpoints[0], "http") {
					endpoints[0] = fmt.Sprintf("https://%s:8006", endpoints[0])
				}
			}

			log.Info().
				Str("cluster", pve.ClusterName).
				Strs("endpoints", endpoints).
				Msg("Creating cluster-aware client")

			clientConfig := config.CreateProxmoxConfig(&pve)
			clientConfig.Timeout = cfg.ConnectionTimeout
			clusterClient := proxmox.NewClusterClient(
				pve.Name,
				clientConfig,
				endpoints,
			)
			m.pveClients[pve.Name] = clusterClient
			log.Info().
				Str("instance", pve.Name).
				Str("cluster", pve.ClusterName).
				Int("endpoints", len(endpoints)).
				Msg("Cluster client created successfully")
		} else {
			// Create regular client
			clientConfig := config.CreateProxmoxConfig(&pve)
			clientConfig.Timeout = cfg.ConnectionTimeout
			client, err := proxmox.NewClient(clientConfig)
			if err != nil {
				monErr := errors.WrapConnectionError("create_pve_client", pve.Name, err)
				log.Error().Err(monErr).Str("instance", pve.Name).Msg("Failed to create PVE client")
				continue
			}
			m.pveClients[pve.Name] = client
			log.Info().Str("instance", pve.Name).Msg("PVE client created successfully")
		}
	}

	// Initialize PBS clients
	log.Info().Int("count", len(cfg.PBSInstances)).Msg("Initializing PBS clients")
	for _, pbsInst := range cfg.PBSInstances {
		log.Info().
			Str("name", pbsInst.Name).
			Str("host", pbsInst.Host).
			Str("user", pbsInst.User).
			Bool("hasToken", pbsInst.TokenName != "").
			Msg("Configuring PBS instance")

		clientConfig := config.CreatePBSConfig(&pbsInst)
		clientConfig.Timeout = 60 * time.Second // Very generous timeout for slow PBS servers
		client, err := pbs.NewClient(clientConfig)
		if err != nil {
			monErr := errors.WrapConnectionError("create_pbs_client", pbsInst.Name, err)
			log.Error().Err(monErr).Str("instance", pbsInst.Name).Msg("Failed to create PBS client")
			continue
		}
		m.pbsClients[pbsInst.Name] = client
		log.Info().Str("instance", pbsInst.Name).Msg("PBS client created successfully")
	}
	} // End of else block for mock mode check

	// Initialize state stats
	m.state.Stats = models.Stats{
		StartTime: m.startTime,
		Version:   "2.0.0-go",
	}

	return m, nil
}

// Start begins the monitoring loop
func (m *Monitor) Start(ctx context.Context, wsHub *websocket.Hub) {
	log.Info().
		Dur("pollingInterval", 10*time.Second).
		Msg("Starting monitoring loop")

	// Initialize and start discovery service if enabled
	if m.config.DiscoveryEnabled {
		discoverySubnet := m.config.DiscoverySubnet
		if discoverySubnet == "" {
			discoverySubnet = "auto"
		}
		m.discoveryService = discovery.NewService(wsHub, 5*time.Minute, discoverySubnet)
		if m.discoveryService != nil {
			m.discoveryService.Start(ctx)
			log.Info().Msg("Discovery service initialized and started")
		} else {
			log.Error().Msg("Failed to initialize discovery service")
		}
	} else {
		log.Info().Msg("Discovery service disabled by configuration")
		m.discoveryService = nil
	}

	// Set up alert callbacks
	m.alertManager.SetAlertCallback(func(alert *alerts.Alert) {
		wsHub.BroadcastAlert(alert)
		// Send notifications
		log.Debug().
			Str("alertID", alert.ID).
			Str("level", string(alert.Level)).
			Msg("Alert raised, sending to notification manager")
		go m.notificationMgr.SendAlert(alert)
	})
	m.alertManager.SetResolvedCallback(func(alertID string) {
		wsHub.BroadcastAlertResolved(alertID)
		// Broadcast updated state immediately so frontend gets the new activeAlerts list
		state := m.GetState()
		wsHub.BroadcastState(state)
	})
	m.alertManager.SetEscalateCallback(func(alert *alerts.Alert, level int) {
		log.Info().
			Str("alertID", alert.ID).
			Int("level", level).
			Msg("Alert escalated - sending notifications")

		// Get escalation config
		config := m.alertManager.GetConfig()
		if level <= 0 || level > len(config.Schedule.Escalation.Levels) {
			return
		}

		escalationLevel := config.Schedule.Escalation.Levels[level-1]

		// Send notifications based on escalation level
		switch escalationLevel.Notify {
		case "email":
			// Only send email
			if emailConfig := m.notificationMgr.GetEmailConfig(); emailConfig.Enabled {
				m.notificationMgr.SendAlert(alert)
			}
		case "webhook":
			// Only send webhooks
			for _, webhook := range m.notificationMgr.GetWebhooks() {
				if webhook.Enabled {
					m.notificationMgr.SendAlert(alert)
					break
				}
			}
		case "all":
			// Send all notifications
			m.notificationMgr.SendAlert(alert)
		}

		// Update WebSocket with escalation
		wsHub.BroadcastAlert(alert)
	})

	// Create separate tickers for polling and broadcasting
	// Hardcoded to 10 seconds since Proxmox updates cluster/resources every 10 seconds
	const pollingInterval = 10 * time.Second
	pollTicker := time.NewTicker(pollingInterval)
	defer pollTicker.Stop()

	broadcastTicker := time.NewTicker(pollingInterval)
	defer broadcastTicker.Stop()

	// Check if mock mode is enabled
	mockEnabled := os.Getenv("PULSE_MOCK_MODE") == "true"
	
	// Do an immediate poll on start (only if not in mock mode)
	if !mockEnabled {
		go m.poll(ctx, wsHub)
	} else {
		log.Info().Msg("Mock mode enabled - skipping real node polling")
	}

	for {
		select {
		case <-pollTicker.C:
			// Start polling in a goroutine so it doesn't block the ticker (only if not in mock mode)
			if !mockEnabled {
				go m.poll(ctx, wsHub)
			} else {
				// In mock mode, still check alerts for mock data
				go m.checkMockAlerts()
			}

		case <-broadcastTicker.C:
			// Broadcast current state regardless of polling status
			// Use GetState() instead of m.state.GetSnapshot() to respect mock mode
			state := m.GetState()
			log.Info().
				Int("nodes", len(state.Nodes)).
				Int("vms", len(state.VMs)).
				Int("containers", len(state.Containers)).
				Int("pbs", len(state.PBSInstances)).
				Int("pbsBackups", len(state.PBSBackups)).
				Msg("Broadcasting state update (ticker)")
			wsHub.BroadcastState(state)

		case <-ctx.Done():
			log.Info().Msg("Monitoring loop stopped")
			return
		}
	}
}

// poll fetches data from all configured instances
func (m *Monitor) poll(ctx context.Context, wsHub *websocket.Hub) {
	// Limit concurrent polls to 2 to prevent resource exhaustion
	currentCount := atomic.AddInt32(&m.activePollCount, 1)
	if currentCount > 2 {
		atomic.AddInt32(&m.activePollCount, -1)
		log.Debug().Int32("activePolls", currentCount-1).Msg("Too many concurrent polls, skipping")
		return
	}
	defer atomic.AddInt32(&m.activePollCount, -1)

	log.Debug().Msg("Starting polling cycle")
	startTime := time.Now()

	if m.config.ConcurrentPolling {
		// Use concurrent polling
		m.pollConcurrent(ctx)
	} else {
		m.pollSequential(ctx)
	}

	// Update performance metrics
	m.state.Performance.LastPollDuration = time.Since(startTime).Seconds()
	m.state.Stats.PollingCycles++
	m.state.Stats.Uptime = int64(time.Since(m.startTime).Seconds())
	m.state.Stats.WebSocketClients = wsHub.GetClientCount()

	// Sync active alerts to state
	activeAlerts := m.alertManager.GetActiveAlerts()
	modelAlerts := make([]models.Alert, 0, len(activeAlerts))
	for _, alert := range activeAlerts {
		modelAlerts = append(modelAlerts, models.Alert{
			ID:           alert.ID,
			Type:         alert.Type,
			Level:        string(alert.Level),
			ResourceID:   alert.ResourceID,
			ResourceName: alert.ResourceName,
			Node:         alert.Node,
			Instance:     alert.Instance,
			Message:      alert.Message,
			Value:        alert.Value,
			Threshold:    alert.Threshold,
			StartTime:    alert.StartTime,
			Acknowledged: alert.Acknowledged,
		})
	}
	m.state.UpdateActiveAlerts(modelAlerts)

	// Sync recently resolved alerts
	recentlyResolved := m.alertManager.GetRecentlyResolved()
	if len(recentlyResolved) > 0 {
		log.Info().Int("count", len(recentlyResolved)).Msg("Syncing recently resolved alerts")
	}
	m.state.UpdateRecentlyResolved(recentlyResolved)

	// Increment poll counter
	m.mu.Lock()
	m.pollCounter++
	m.mu.Unlock()

	log.Debug().Dur("duration", time.Since(startTime)).Msg("Polling cycle completed")

	// Broadcasting is now handled by the timer in Start()
}

// pollConcurrent polls all instances concurrently
func (m *Monitor) pollConcurrent(ctx context.Context) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Poll PVE instances
	for name, client := range m.pveClients {
		// Check if context is already cancelled before starting
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		go func(instanceName string, c PVEClientInterface) {
			defer wg.Done()
			// Pass context to ensure cancellation propagates
			m.pollPVEInstance(ctx, instanceName, c)
		}(name, client)
	}

	// Poll PBS instances
	for name, client := range m.pbsClients {
		// Check if context is already cancelled before starting
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		go func(instanceName string, c *pbs.Client) {
			defer wg.Done()
			// Pass context to ensure cancellation propagates
			m.pollPBSInstance(ctx, instanceName, c)
		}(name, client)
	}

	// Wait for all goroutines to complete or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines completed normally
	case <-ctx.Done():
		// Context cancelled, cancel all operations
		cancel()
		// Still wait for goroutines to finish gracefully
		wg.Wait()
	}
}

// pollSequential polls all instances sequentially
func (m *Monitor) pollSequential(ctx context.Context) {
	// Poll PVE instances
	for name, client := range m.pveClients {
		// Check context before each instance
		select {
		case <-ctx.Done():
			return
		default:
		}
		m.pollPVEInstance(ctx, name, client)
	}

	// Poll PBS instances
	for name, client := range m.pbsClients {
		// Check context before each instance
		select {
		case <-ctx.Done():
			return
		default:
		}
		m.pollPBSInstance(ctx, name, client)
	}
}

// pollPVEInstance polls a single PVE instance
func (m *Monitor) pollPVEInstance(ctx context.Context, instanceName string, client PVEClientInterface) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		log.Debug().Str("instance", instanceName).Msg("Polling cancelled")
		return
	default:
	}

	log.Debug().Str("instance", instanceName).Msg("Polling PVE instance")

	// Get instance config
	var instanceCfg *config.PVEInstance
	for _, cfg := range m.config.PVEInstances {
		if cfg.Name == instanceName {
			instanceCfg = &cfg
			break
		}
	}
	if instanceCfg == nil {
		return
	}

	// Poll nodes
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		monErr := errors.WrapConnectionError("poll_nodes", instanceName, err)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get nodes")
		m.state.SetConnectionHealth(instanceName, false)

		// Track auth failure if it's an authentication error
		if errors.IsAuthError(err) {
			m.recordAuthFailure(instanceName, "pve")
		}
		return
	}

	// Reset auth failures on successful connection
	m.resetAuthFailures(instanceName, "pve")
	m.state.SetConnectionHealth(instanceName, true)

	// Convert to models
	var modelNodes []models.Node
	for _, node := range nodes {
		modelNode := models.Node{
			ID:               instanceName + "-" + node.Node,
			Name:             node.Node,
			Instance:         instanceName,
			Host:             instanceCfg.Host,  // Add the actual host URL
			Status:           node.Status,
			Type:             "node",
			CPU:              safeFloat(node.CPU), // Already in percentage
			Memory: models.Memory{
				Total: int64(node.MaxMem),
				Used:  int64(node.Mem),
				Free:  int64(node.MaxMem - node.Mem),
				Usage: safePercentage(float64(node.Mem), float64(node.MaxMem)),
			},
			Disk: models.Disk{
				Total: int64(node.MaxDisk),
				Used:  int64(node.Disk),
				Free:  int64(node.MaxDisk - node.Disk),
				Usage: safePercentage(float64(node.Disk), float64(node.MaxDisk)),
			},
			Uptime:           int64(node.Uptime),
			LoadAverage:      []float64{},
			LastSeen:         time.Now(),
			ConnectionHealth: "healthy",
			IsClusterMember:  instanceCfg.IsCluster,
			ClusterName:      instanceCfg.ClusterName,
		}

		// Debug logging for disk metrics - note that these values can fluctuate
		// due to thin provisioning and dynamic allocation
		log.Debug().
			Str("node", node.Node).
			Uint64("disk", node.Disk).
			Uint64("maxDisk", node.MaxDisk).
			Float64("diskUsage", safePercentage(float64(node.Disk), float64(node.MaxDisk))).
			Msg("Node disk metrics (raw from Proxmox)")

		// Get detailed node info if available (skip for offline nodes)
		if node.Status == "online" {
			nodeInfo, nodeErr := client.GetNodeStatus(ctx, node.Node)
			if nodeErr != nil {
				// If we can't get node status, log it
				log.Debug().
					Str("instance", instanceName).
					Str("node", node.Node).
					Err(nodeErr).
					Msg("Could not get node status")
			} else if nodeInfo != nil {
				// Convert LoadAvg from interface{} to float64
				loadAvg := make([]float64, 0, len(nodeInfo.LoadAvg))
			for _, val := range nodeInfo.LoadAvg {
				switch v := val.(type) {
				case float64:
					loadAvg = append(loadAvg, v)
				case string:
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						loadAvg = append(loadAvg, f)
					}
				}
			}
			modelNode.LoadAverage = loadAvg
			modelNode.KernelVersion = nodeInfo.KernelVersion
			modelNode.PVEVersion = nodeInfo.PVEVersion

			// Use rootfs data if available for more stable disk metrics
			if nodeInfo.RootFS != nil && nodeInfo.RootFS.Total > 0 {
				modelNode.Disk = models.Disk{
					Total: int64(nodeInfo.RootFS.Total),
					Used:  int64(nodeInfo.RootFS.Used),
					Free:  int64(nodeInfo.RootFS.Free),
					Usage: safePercentage(float64(nodeInfo.RootFS.Used), float64(nodeInfo.RootFS.Total)),
				}
				log.Debug().
					Str("node", node.Node).
					Uint64("rootfsUsed", nodeInfo.RootFS.Used).
					Uint64("rootfsTotal", nodeInfo.RootFS.Total).
					Float64("rootfsUsage", modelNode.Disk.Usage).
					Msg("Using rootfs for disk metrics")
			}

			if nodeInfo.CPUInfo != nil {
				// Use MaxCPU from node data for logical CPU count (includes hyperthreading)
				// If MaxCPU is not available or 0, fall back to physical cores
				logicalCores := node.MaxCPU
				if logicalCores == 0 {
					logicalCores = nodeInfo.CPUInfo.Cores
				}

				mhzStr := nodeInfo.CPUInfo.GetMHzString()
				log.Debug().
					Str("node", node.Node).
					Str("model", nodeInfo.CPUInfo.Model).
					Int("cores", nodeInfo.CPUInfo.Cores).
					Int("logicalCores", logicalCores).
					Int("sockets", nodeInfo.CPUInfo.Sockets).
					Str("mhz", mhzStr).
					Msg("Node CPU info from Proxmox")
				modelNode.CPUInfo = models.CPUInfo{
					Model:   nodeInfo.CPUInfo.Model,
					Cores:   logicalCores, // Use logical cores for display
					Sockets: nodeInfo.CPUInfo.Sockets,
					MHz:     mhzStr,
				}
			}
			}
		}

		modelNodes = append(modelNodes, modelNode)
	}

	// Update state first so we have nodes available
	m.state.UpdateNodesForInstance(instanceName, modelNodes)

	// Now get storage data to use as fallback for disk metrics if needed
	storageByNode := make(map[string]models.Disk)
	if instanceCfg.MonitorStorage {
		_, err := client.GetAllStorage(ctx)
		if err == nil {
			for _, node := range nodes {
				// Skip offline nodes to avoid 595 errors
				if node.Status != "online" {
					continue
				}
				
				nodeStorages, err := client.GetStorage(ctx, node.Node)
				if err == nil {
					// Look for local or local-lvm storage as most stable disk metric
					for _, storage := range nodeStorages {
						if storage.Storage == "local" || storage.Storage == "local-lvm" {
							disk := models.Disk{
								Total: int64(storage.Total),
								Used:  int64(storage.Used),
								Free:  int64(storage.Available),
								Usage: safePercentage(float64(storage.Used), float64(storage.Total)),
							}
							// Prefer "local" over "local-lvm"
							if _, exists := storageByNode[node.Node]; !exists || storage.Storage == "local" {
								storageByNode[node.Node] = disk
								log.Debug().
									Str("node", node.Node).
									Str("storage", storage.Storage).
									Float64("usage", disk.Usage).
									Msg("Using storage for disk metrics fallback")
							}
						}
					}
				}
			}
		}
	}

	// Update nodes with storage fallback if rootfs was not available
	for i := range modelNodes {
		if modelNodes[i].Disk.Total == 0 {
			if disk, exists := storageByNode[modelNodes[i].Name]; exists {
				modelNodes[i].Disk = disk
				log.Debug().
					Str("node", modelNodes[i].Name).
					Float64("usage", disk.Usage).
					Msg("Applied storage fallback for disk metrics")
			}
		}

		// Record node metrics history
		now := time.Now()
		m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "cpu", modelNodes[i].CPU*100, now)
		m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "memory", modelNodes[i].Memory.Usage, now)
		m.metricsHistory.AddNodeMetric(modelNodes[i].ID, "disk", modelNodes[i].Disk.Usage, now)

		// Check thresholds for alerts
		m.alertManager.CheckNode(modelNodes[i])
	}

	// Update state again with corrected disk metrics
	m.state.UpdateNodesForInstance(instanceName, modelNodes)
	
	// Update cluster endpoint online status if this is a cluster
	if instanceCfg.IsCluster && len(instanceCfg.ClusterEndpoints) > 0 {
		// Create a map of online nodes from our polling results
		onlineNodes := make(map[string]bool)
		for _, node := range modelNodes {
			// Node is online if we successfully got its data
			onlineNodes[node.Name] = node.Status == "online"
		}
		
		// Update the online status for each cluster endpoint
		for i := range instanceCfg.ClusterEndpoints {
			if online, exists := onlineNodes[instanceCfg.ClusterEndpoints[i].NodeName]; exists {
				instanceCfg.ClusterEndpoints[i].Online = online
				if online {
					instanceCfg.ClusterEndpoints[i].LastSeen = time.Now()
				}
			}
		}
		
		// Update the config with the new online status
		// This is needed so the UI can reflect the current status
		for idx, cfg := range m.config.PVEInstances {
			if cfg.Name == instanceName {
				m.config.PVEInstances[idx].ClusterEndpoints = instanceCfg.ClusterEndpoints
				break
			}
		}
	}

	// Poll VMs and containers together using cluster/resources for efficiency
	if instanceCfg.MonitorVMs || instanceCfg.MonitorContainers {
		select {
		case <-ctx.Done():
			return
		default:
			// Only try cluster endpoints if this is configured as a cluster
			// This prevents syslog spam on non-clustered nodes from certificate checks
			useClusterEndpoint := false
			if instanceCfg.IsCluster {
				// Double-check that this is actually a cluster to prevent misconfiguration
				// This helps avoid certificate spam on standalone nodes incorrectly marked as clusters
				isActuallyCluster, _ := client.IsClusterMember(ctx)
				if isActuallyCluster {
					// Try to use efficient cluster/resources endpoint
					useClusterEndpoint = m.pollVMsAndContainersEfficient(ctx, instanceName, client)
				} else {
					// Misconfigured - marked as cluster but isn't one
					log.Warn().
						Str("instance", instanceName).
						Msg("Instance marked as cluster but is actually standalone - consider updating configuration")
					instanceCfg.IsCluster = false
				}
			}
			
			if !useClusterEndpoint {
				// Use traditional polling for non-clusters or if cluster endpoint fails
				// Use WithNodes versions to avoid duplicate GetNodes calls
				if instanceCfg.MonitorVMs {
					m.pollVMsWithNodes(ctx, instanceName, client, nodes)
				}
				if instanceCfg.MonitorContainers {
					m.pollContainersWithNodes(ctx, instanceName, client, nodes)
				}
			}
		}
	}

	// Poll storage if enabled
	if instanceCfg.MonitorStorage {
		select {
		case <-ctx.Done():
			return
		default:
			m.pollStorageWithNodes(ctx, instanceName, client, nodes)
		}
	}

	// Poll backups if enabled - using configurable cycle count
	// This prevents slow backup/snapshot queries from blocking real-time stats
	// Also poll on first cycle (pollCounter == 1) to ensure data loads quickly
	backupCycles := 10 // default
	if m.config.BackupPollingCycles > 0 {
		backupCycles = m.config.BackupPollingCycles
	}
	if instanceCfg.MonitorBackups && (m.pollCounter%int64(backupCycles) == 0 || m.pollCounter == 1) {
		select {
		case <-ctx.Done():
			return
		default:
			// Run backup polling in a separate goroutine to not block main polling
			go func() {
				log.Info().Str("instance", instanceName).Msg("Starting background backup/snapshot polling")
				// Create a separate context with longer timeout for backup operations
				backupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				// Poll backup tasks
				m.pollBackupTasks(backupCtx, instanceName, client)

				// Poll storage backups - pass nodes to avoid duplicate API calls
				m.pollStorageBackupsWithNodes(backupCtx, instanceName, client, nodes)

				// Poll guest snapshots
				m.pollGuestSnapshots(backupCtx, instanceName, client)

				log.Info().Str("instance", instanceName).Msg("Completed background backup/snapshot polling")
			}()
		}
	}
}

// pollVMsAndContainersEfficient uses the cluster/resources endpoint to get all VMs and containers in one call
// This should only be called for instances configured as clusters
func (m *Monitor) pollVMsAndContainersEfficient(ctx context.Context, instanceName string, client PVEClientInterface) bool {
	log.Info().Str("instance", instanceName).Msg("Polling VMs and containers using cluster/resources")
	
	// Get all resources in a single API call
	resources, err := client.GetClusterResources(ctx, "vm")
	if err != nil {
		log.Debug().Err(err).Str("instance", instanceName).Msg("cluster/resources not available, falling back to traditional polling")
		return false
	}
	
	var allVMs []models.VM
	var allContainers []models.Container
	
	for _, res := range resources {
		// Avoid duplicating node name in ID when instance name equals node name
		var guestID string
		if instanceName == res.Node {
			guestID = fmt.Sprintf("%s-%d", res.Node, res.VMID)
		} else {
			guestID = fmt.Sprintf("%s-%s-%d", instanceName, res.Node, res.VMID)
		}
		
		// Debug log the resource type
		log.Debug().
			Str("instance", instanceName).
			Str("name", res.Name).
			Int("vmid", res.VMID).
			Str("type", res.Type).
			Msg("Processing cluster resource")
		
		// Calculate I/O rates
		currentMetrics := IOMetrics{
			DiskRead:   int64(res.DiskRead),
			DiskWrite:  int64(res.DiskWrite),
			NetworkIn:  int64(res.NetIn),
			NetworkOut: int64(res.NetOut),
			Timestamp:  time.Now(),
		}
		diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)
		
		
		if res.Type == "qemu" {
			// Skip templates if configured
			if res.Template == 1 {
				continue
			}
			
			// Try to get actual disk usage from guest agent if VM is running
			diskUsed := res.Disk
			diskTotal := res.MaxDisk
			diskFree := diskTotal - diskUsed
			diskUsage := safePercentage(float64(diskUsed), float64(diskTotal))
			
			// If VM shows 0 disk usage but has allocated disk, it's likely guest agent issue
			// Set to -1 to indicate "unknown" rather than showing misleading 0%
			if res.Type == "qemu" && diskUsed == 0 && diskTotal > 0 && res.Status == "running" {
				diskUsage = -1
			}
			
			// For running VMs, always try to get filesystem info from guest agent if disk is 0
			// The cluster/resources endpoint often returns 0 for disk even when data is available
			if res.Status == "running" && res.Type == "qemu" && diskUsed == 0 && diskTotal > 0 {
				// First check if agent is enabled by getting VM status
				vmStatus, err := client.GetVMStatus(ctx, res.Node, res.VMID)
				if err != nil {
					log.Debug().
						Err(err).
						Str("instance", instanceName).
						Str("vm", res.Name).
						Int("vmid", res.VMID).
						Msg("Could not get VM status to check guest agent availability")
				} else if vmStatus != nil {
					// Always try to get filesystem info if VM is running and disk shows 0
					// Even if agent flag is 0, the agent might still be available
					if vmStatus.Agent > 0 || (diskUsed == 0 && diskTotal > 0) {
						log.Debug().
							Str("instance", instanceName).
							Str("vm", res.Name).
							Int("vmid", res.VMID).
							Int("agent", vmStatus.Agent).
							Bool("diskIsZero", diskUsed == 0).
							Msg("Attempting to get filesystem info from guest agent")
						
						fsInfo, err := client.GetVMFSInfo(ctx, res.Node, res.VMID)
						if err != nil {
							// Log more helpful error messages based on the error type
							errMsg := err.Error()
							if strings.Contains(errMsg, "500") || strings.Contains(errMsg, "QEMU guest agent is not running") {
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Msg("Guest agent enabled in VM config but not running inside guest OS. Install and start qemu-guest-agent in the VM")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("To verify: ssh into VM and run 'systemctl status qemu-guest-agent' or 'ps aux | grep qemu-ga'")
							} else if strings.Contains(errMsg, "timeout") {
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Msg("Guest agent timeout - agent may be installed but not responding")
							} else if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "authentication error") {
								// Permission error - check if it's the known PVE 9 limitation
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Msg("VM disk monitoring permission denied. This is a known limitation:")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("• Check token has PVEAuditor role (includes VM.GuestAgent.Audit on PVE 9)")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("• Proxmox 8: Re-run setup script to add VM.Monitor permission if added before v4.7")
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Msg("• Verify guest agent is installed and running inside the VM")
							} else {
								log.Debug().
									Err(err).
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("vmid", res.VMID).
									Msg("Failed to get filesystem info from guest agent")
							}
						} else if len(fsInfo) == 0 {
							log.Info().
								Str("instance", instanceName).
								Str("vm", res.Name).
								Int("vmid", res.VMID).
								Msg("Guest agent returned no filesystem info - agent may need restart or VM may have no mounted filesystems")
						} else {
							log.Debug().
								Str("instance", instanceName).
								Str("vm", res.Name).
								Int("filesystems", len(fsInfo)).
								Msg("Got filesystem info from guest agent")
							
							// Aggregate disk usage from all filesystems
							var totalBytes, usedBytes uint64
							var skippedFS []string
							for _, fs := range fsInfo {
								// Skip special filesystems and mounts
								if fs.Type == "tmpfs" || fs.Type == "devtmpfs" || 
								   strings.HasPrefix(fs.Mountpoint, "/dev") ||
								   strings.HasPrefix(fs.Mountpoint, "/proc") ||
								   strings.HasPrefix(fs.Mountpoint, "/sys") ||
								   strings.HasPrefix(fs.Mountpoint, "/run") ||
								   fs.Mountpoint == "/boot/efi" {
									skippedFS = append(skippedFS, fmt.Sprintf("%s(%s)", fs.Mountpoint, fs.Type))
									continue
								}
								
								// Only count real filesystems
								if fs.TotalBytes > 0 {
									totalBytes += fs.TotalBytes
									usedBytes += fs.UsedBytes
									log.Debug().
										Str("instance", instanceName).
										Str("vm", res.Name).
										Str("mountpoint", fs.Mountpoint).
										Str("type", fs.Type).
										Uint64("total", fs.TotalBytes).
										Uint64("used", fs.UsedBytes).
										Msg("Including filesystem in disk usage calculation")
								}
							}
							
							if len(skippedFS) > 0 {
								log.Debug().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Strs("skipped", skippedFS).
									Msg("Skipped special filesystems")
							}
							
							// If we got valid data from guest agent, use it
							if totalBytes > 0 {
								diskTotal = totalBytes
								diskUsed = usedBytes
								diskFree = totalBytes - usedBytes
								diskUsage = safePercentage(float64(usedBytes), float64(totalBytes))
								
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Uint64("totalBytes", totalBytes).
									Uint64("usedBytes", usedBytes).
									Float64("usage", diskUsage).
									Msg("Successfully retrieved disk usage from guest agent (cluster/resources showed 0)")
							} else {
								log.Info().
									Str("instance", instanceName).
									Str("vm", res.Name).
									Int("filesystems_found", len(fsInfo)).
									Msg("Guest agent provided filesystem info but no usable filesystems found (all were special mounts)")
							}
						}
					} else {
						log.Debug().
							Str("instance", instanceName).
							Str("vm", res.Name).
							Int("vmid", res.VMID).
							Int("agent", vmStatus.Agent).
							Msg("VM does not have guest agent enabled in config")
					}
				}
			}
			
			vm := models.VM{
				ID:         guestID,
				VMID:       res.VMID,
				Name:       res.Name,
				Node:       res.Node,
				Instance:   instanceName,
				Status:     res.Status,
				Type:       "qemu",
				CPU:        safeFloat(res.CPU),
				CPUs:       res.MaxCPU,
				Memory: models.Memory{
					Total: int64(res.MaxMem),
					Used:  int64(res.Mem),
					Free:  int64(res.MaxMem - res.Mem),
					Usage: safePercentage(float64(res.Mem), float64(res.MaxMem)),
				},
				Disk: models.Disk{
					Total: int64(diskTotal),
					Used:  int64(diskUsed),
					Free:  int64(diskFree),
					Usage: diskUsage,
				},
				NetworkIn:  maxInt64(0, int64(netInRate)),
				NetworkOut: maxInt64(0, int64(netOutRate)),
				DiskRead:   maxInt64(0, int64(diskReadRate)),
				DiskWrite:  maxInt64(0, int64(diskWriteRate)),
				Uptime:     int64(res.Uptime),
				Template:   res.Template == 1,
				LastSeen:   time.Now(),
			}
			
			// Parse tags
			if res.Tags != "" {
				vm.Tags = strings.Split(res.Tags, ";")
				
				// Log if Pulse-specific tags are detected
				for _, tag := range vm.Tags {
					switch tag {
					case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
						log.Info().
							Str("vm", vm.Name).
							Str("node", vm.Node).
							Str("tag", tag).
							Msg("Pulse control tag detected on VM")
					}
				}
			}
			
			allVMs = append(allVMs, vm)
			
			// For non-running VMs, zero out resource usage metrics to prevent false alerts
			// Proxmox may report stale or residual metrics for stopped VMs
			if vm.Status != "running" {
				log.Debug().
					Str("vm", vm.Name).
					Str("status", vm.Status).
					Float64("originalCpu", vm.CPU).
					Float64("originalMemUsage", vm.Memory.Usage).
					Msg("Non-running VM detected - zeroing metrics")
				
				// Zero out all usage metrics for stopped/paused/suspended VMs
				vm.CPU = 0
				vm.Memory.Usage = 0
				vm.Disk.Usage = 0
				vm.NetworkIn = 0
				vm.NetworkOut = 0
				vm.DiskRead = 0
				vm.DiskWrite = 0
			}
			
			// Check thresholds for alerts
			m.alertManager.CheckGuest(vm, instanceName)
			
		} else if res.Type == "lxc" {
			// Skip templates if configured
			if res.Template == 1 {
				continue
			}
			
			container := models.Container{
				ID:         guestID,
				VMID:       res.VMID,
				Name:       res.Name,
				Node:       res.Node,
				Instance:   instanceName,
				Status:     res.Status,
				Type:       "lxc",
				CPU:        safeFloat(res.CPU),
				CPUs:       int(res.MaxCPU),
				Memory: models.Memory{
					Total: int64(res.MaxMem),
					Used:  int64(res.Mem),
					Free:  int64(res.MaxMem - res.Mem),
					Usage: safePercentage(float64(res.Mem), float64(res.MaxMem)),
				},
				Disk: models.Disk{
					Total: int64(res.MaxDisk),
					Used:  int64(res.Disk),
					Free:  int64(res.MaxDisk - res.Disk),
					Usage: safePercentage(float64(res.Disk), float64(res.MaxDisk)),
				},
				NetworkIn:  maxInt64(0, int64(netInRate)),
				NetworkOut: maxInt64(0, int64(netOutRate)),
				DiskRead:   maxInt64(0, int64(diskReadRate)),
				DiskWrite:  maxInt64(0, int64(diskWriteRate)),
				Uptime:     int64(res.Uptime),
				Template:   res.Template == 1,
				LastSeen:   time.Now(),
			}
			
			// Parse tags
			if res.Tags != "" {
				container.Tags = strings.Split(res.Tags, ";")
				
				// Log if Pulse-specific tags are detected
				for _, tag := range container.Tags {
					switch tag {
					case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
						log.Info().
							Str("container", container.Name).
							Str("node", container.Node).
							Str("tag", tag).
							Msg("Pulse control tag detected on container")
					}
				}
			}
			
			allContainers = append(allContainers, container)
			
			// For non-running containers, zero out resource usage metrics to prevent false alerts
			// Proxmox may report stale or residual metrics for stopped containers
			if container.Status != "running" {
				log.Debug().
					Str("container", container.Name).
					Str("status", container.Status).
					Float64("originalCpu", container.CPU).
					Float64("originalMemUsage", container.Memory.Usage).
					Msg("Non-running container detected - zeroing metrics")
				
				// Zero out all usage metrics for stopped/paused containers
				container.CPU = 0
				container.Memory.Usage = 0
				container.Disk.Usage = 0
				container.NetworkIn = 0
				container.NetworkOut = 0
				container.DiskRead = 0
				container.DiskWrite = 0
			}
			
			// Check thresholds for alerts
			m.alertManager.CheckGuest(container, instanceName)
		}
	}
	
	// Update state
	if len(allVMs) > 0 {
		m.state.UpdateVMsForInstance(instanceName, allVMs)
	}
	if len(allContainers) > 0 {
		m.state.UpdateContainersForInstance(instanceName, allContainers)
	}
	
	log.Info().
		Str("instance", instanceName).
		Int("vms", len(allVMs)).
		Int("containers", len(allContainers)).
		Msg("VMs and containers polled efficiently with cluster/resources")
	
	return true
}

// pollVMs polls VMs from a PVE instance
// Deprecated: This function should not be called directly as it causes duplicate GetNodes calls.
// Use pollVMsWithNodes instead.
func (m *Monitor) pollVMs(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Warn().Str("instance", instanceName).Msg("pollVMs called directly - this causes duplicate GetNodes calls and syslog spam on non-clustered nodes")
	
	// Get all nodes first
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		monErr := errors.WrapConnectionError("get_nodes_for_vms", instanceName, err)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get nodes for VM polling")
		return
	}

	m.pollVMsWithNodes(ctx, instanceName, client, nodes)
}

// pollVMsWithNodes polls VMs using a provided nodes list to avoid duplicate GetNodes calls
func (m *Monitor) pollVMsWithNodes(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node) {
	var allVMs []models.VM
	for _, node := range nodes {
		// Skip offline nodes to avoid 595 errors when trying to access their resources
		if node.Status != "online" {
			log.Debug().
				Str("node", node.Node).
				Str("status", node.Status).
				Msg("Skipping offline node for VM polling")
			continue
		}
		
		vms, err := client.GetVMs(ctx, node.Node)
		if err != nil {
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_vms", instanceName, err).WithNode(node.Node)
			log.Error().Err(monErr).Str("node", node.Node).Msg("Failed to get VMs")
			continue
		}

		for _, vm := range vms {
			// Skip templates if configured
			if vm.Template == 1 {
				continue
			}

			// Parse tags
			var tags []string
			if vm.Tags != "" {
				tags = strings.Split(vm.Tags, ";")
				
				// Log if Pulse-specific tags are detected
				for _, tag := range tags {
					switch tag {
					case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
						log.Info().
							Str("vm", vm.Name).
							Str("node", node.Node).
							Str("tag", tag).
							Msg("Pulse control tag detected on VM (legacy API)")
					}
				}
			}

			// Calculate I/O rates
			// Avoid duplicating node name in ID when instance name equals node name
			var guestID string
			if instanceName == node.Node {
				guestID = fmt.Sprintf("%s-%d", node.Node, vm.VMID)
			} else {
				guestID = fmt.Sprintf("%s-%s-%d", instanceName, node.Node, vm.VMID)
			}
			currentMetrics := IOMetrics{
				DiskRead:   int64(vm.DiskRead),
				DiskWrite:  int64(vm.DiskWrite),
				NetworkIn:  int64(vm.NetIn),
				NetworkOut: int64(vm.NetOut),
				Timestamp:  time.Now(),
			}
			diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

			// For running VMs, try to get detailed status with balloon info
			memUsed := uint64(0)
			memTotal := vm.MaxMem

			if vm.Status == "running" {
				// Try to get detailed VM status for more accurate memory reporting
				if vmStatus, err := client.GetVMStatus(ctx, node.Node, vm.VMID); err == nil {
					// If balloon is enabled, use balloon as the total available memory
					if vmStatus.Balloon > 0 && vmStatus.Balloon < vmStatus.MaxMem {
						memTotal = vmStatus.Balloon
					}
					
					// If we have free memory from guest agent, calculate actual usage
					if vmStatus.FreeMem > 0 {
						// Guest agent reports free memory, so calculate used
						memUsed = memTotal - vmStatus.FreeMem
					} else if vmStatus.Mem > 0 {
						// No guest agent free memory data, but we have actual memory usage
						// Use the reported memory usage from Proxmox
						memUsed = vmStatus.Mem
					} else {
						// No memory data available at all - show 0% usage
						memUsed = 0
					}
				} else {
					// Failed to get detailed status - show 0% usage
					memUsed = 0
				}
			} else {
				// VM is not running, show 0 usage
				memUsed = 0
			}

			// Set CPU to 0 for non-running VMs to avoid false alerts
			// VMs can have status: running, stopped, paused, suspended
			cpuUsage := safeFloat(vm.CPU)
			if vm.Status != "running" {
				cpuUsage = 0
			}

			// Try to get actual disk usage from guest agent if available
			diskUsed := uint64(vm.Disk)
			diskTotal := uint64(vm.MaxDisk)
			diskFree := diskTotal - diskUsed
			diskUsage := safePercentage(float64(diskUsed), float64(diskTotal))
			
			// If VM shows 0 disk usage but has allocated disk, it's likely guest agent issue
			// Set to -1 to indicate "unknown" rather than showing misleading 0%
			if diskUsed == 0 && diskTotal > 0 && vm.Status == "running" {
				diskUsage = -1
			}
			
			// For running VMs with 0 disk usage, always try guest agent (even if agent flag is 0)
			// The agent flag might not be reliable or the API might return 0 incorrectly
			if vm.Status == "running" && (vm.Agent > 0 || (diskUsed == 0 && diskTotal > 0)) {
				log.Debug().
					Str("instance", instanceName).
					Str("vm", vm.Name).
					Int("vmid", vm.VMID).
					Int("agent", vm.Agent).
					Bool("diskIsZero", diskUsed == 0).
					Msg("Attempting to get filesystem info from guest agent (legacy API)")
				
				fsInfo, err := client.GetVMFSInfo(ctx, node.Node, vm.VMID)
				if err != nil {
					// Log more helpful error messages based on the error type
					errMsg := err.Error()
					if strings.Contains(errMsg, "500") || strings.Contains(errMsg, "QEMU guest agent is not running") {
						log.Info().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Int("vmid", vm.VMID).
							Msg("Guest agent enabled in VM config but not running inside guest OS. Install and start qemu-guest-agent in the VM (legacy API)")
					} else if strings.Contains(errMsg, "timeout") {
						log.Info().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Int("vmid", vm.VMID).
							Msg("Guest agent timeout - agent may be installed but not responding (legacy API)")
					} else if strings.Contains(errMsg, "403") || strings.Contains(errMsg, "401") || strings.Contains(errMsg, "authentication error") {
						// Permission error - check if it's the known PVE 9 limitation
						log.Info().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Int("vmid", vm.VMID).
							Msg("VM disk monitoring permission denied (legacy API). This is a known limitation:")
						log.Info().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Msg("• Check token has PVEAuditor role (includes VM.GuestAgent.Audit on PVE 9)")
						log.Info().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Msg("• Proxmox 8: Re-run setup script to add VM.Monitor permission if added before v4.7")
						log.Info().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Msg("• Verify guest agent is installed and running inside the VM")
					} else {
						log.Debug().
							Err(err).
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Int("vmid", vm.VMID).
							Msg("Failed to get filesystem info from guest agent (legacy API)")
					}
				} else if len(fsInfo) == 0 {
					log.Info().
						Str("instance", instanceName).
						Str("vm", vm.Name).
						Int("vmid", vm.VMID).
						Msg("Guest agent returned no filesystem info - agent may need restart or VM may have no mounted filesystems (legacy API)")
				} else {
					log.Debug().
						Str("instance", instanceName).
						Str("vm", vm.Name).
						Int("filesystems", len(fsInfo)).
						Msg("Got filesystem info from guest agent (legacy API)")
					// Aggregate disk usage from all filesystems
					// Focus on actual filesystems, skip special mounts like /dev, /proc, /sys
					var totalBytes, usedBytes uint64
					var skippedFS []string
					for _, fs := range fsInfo {
						// Skip special filesystems and mounts
						if fs.Type == "tmpfs" || fs.Type == "devtmpfs" || 
						   strings.HasPrefix(fs.Mountpoint, "/dev") ||
						   strings.HasPrefix(fs.Mountpoint, "/proc") ||
						   strings.HasPrefix(fs.Mountpoint, "/sys") ||
						   strings.HasPrefix(fs.Mountpoint, "/run") ||
						   fs.Mountpoint == "/boot/efi" {
							skippedFS = append(skippedFS, fmt.Sprintf("%s(%s)", fs.Mountpoint, fs.Type))
							continue
						}
						
						// Only count real filesystems (ext4, xfs, btrfs, ntfs, etc.)
						if fs.TotalBytes > 0 {
							totalBytes += fs.TotalBytes
							usedBytes += fs.UsedBytes
							log.Debug().
								Str("instance", instanceName).
								Str("vm", vm.Name).
								Str("mountpoint", fs.Mountpoint).
								Str("type", fs.Type).
								Uint64("total", fs.TotalBytes).
								Uint64("used", fs.UsedBytes).
								Msg("Including filesystem in disk usage calculation (legacy API)")
						}
					}
					
					if len(skippedFS) > 0 {
						log.Debug().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Strs("skipped", skippedFS).
							Msg("Skipped special filesystems (legacy API)")
					}
					
					// If we got valid data from guest agent, use it
					if totalBytes > 0 {
						diskTotal = totalBytes
						diskUsed = usedBytes
						diskFree = totalBytes - usedBytes
						diskUsage = safePercentage(float64(usedBytes), float64(totalBytes))
						
						log.Info().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Int("vmid", vm.VMID).
							Uint64("totalBytes", totalBytes).
							Uint64("usedBytes", usedBytes).
							Float64("usage", diskUsage).
							Msg("Successfully retrieved disk usage from guest agent (node API showed 0)")
					} else {
						log.Info().
							Str("instance", instanceName).
							Str("vm", vm.Name).
							Int("filesystems_found", len(fsInfo)).
							Msg("Guest agent provided filesystem info but no usable filesystems found (all were special mounts) (legacy API)")
					}
				}
			} else {
				if vm.Agent == 0 {
					log.Debug().
						Str("instance", instanceName).
						Str("vm", vm.Name).
						Int("vmid", vm.VMID).
						Int("agent", vm.Agent).
						Msg("VM does not have guest agent enabled in config (legacy API)")
				}
			}

			modelVM := models.VM{
				ID:       guestID,
				VMID:     vm.VMID,
				Name:     vm.Name,
				Node:     node.Node,
				Instance: instanceName,
				Status:   vm.Status,
				Type:     "qemu",
				CPU:      cpuUsage, // Already in percentage
				CPUs:     vm.CPUs,
				Memory: models.Memory{
					Total: int64(memTotal),
					Used:  int64(memUsed),
					Free:  int64(memTotal - memUsed),
					Usage: safePercentage(float64(memUsed), float64(memTotal)),
				},
				Disk: models.Disk{
					Total: int64(diskTotal),
					Used:  int64(diskUsed),
					Free:  int64(diskFree),
					Usage: diskUsage,
				},
				NetworkIn:  maxInt64(0, int64(netInRate)),
				NetworkOut: maxInt64(0, int64(netOutRate)),
				DiskRead:   maxInt64(0, int64(diskReadRate)),
				DiskWrite:  maxInt64(0, int64(diskWriteRate)),
				Uptime:     int64(vm.Uptime),
				Template:   vm.Template == 1,
				Tags:       tags,
				Lock:       vm.Lock,
				LastSeen:   time.Now(),
			}
			allVMs = append(allVMs, modelVM)

			// Record metrics history
			now := time.Now()
			m.metricsHistory.AddGuestMetric(modelVM.ID, "cpu", modelVM.CPU*100, now)
			m.metricsHistory.AddGuestMetric(modelVM.ID, "memory", modelVM.Memory.Usage, now)
			m.metricsHistory.AddGuestMetric(modelVM.ID, "disk", modelVM.Disk.Usage, now)
			m.metricsHistory.AddGuestMetric(modelVM.ID, "diskread", float64(modelVM.DiskRead), now)
			m.metricsHistory.AddGuestMetric(modelVM.ID, "diskwrite", float64(modelVM.DiskWrite), now)
			m.metricsHistory.AddGuestMetric(modelVM.ID, "netin", float64(modelVM.NetworkIn), now)
			m.metricsHistory.AddGuestMetric(modelVM.ID, "netout", float64(modelVM.NetworkOut), now)

			// Check thresholds for alerts
			m.alertManager.CheckGuest(modelVM, instanceName)
		}
	}

	m.state.UpdateVMsForInstance(instanceName, allVMs)
}

// pollContainers polls containers from a PVE instance
// Deprecated: This function should not be called directly as it causes duplicate GetNodes calls.
// Use pollContainersWithNodes instead.
func (m *Monitor) pollContainers(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Warn().Str("instance", instanceName).Msg("pollContainers called directly - this causes duplicate GetNodes calls and syslog spam on non-clustered nodes")
	
	// Get all nodes first
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		monErr := errors.WrapConnectionError("get_nodes_for_containers", instanceName, err)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get nodes for container polling")
		return
	}

	m.pollContainersWithNodes(ctx, instanceName, client, nodes)
}

// pollContainersWithNodes polls containers using a provided nodes list to avoid duplicate GetNodes calls
func (m *Monitor) pollContainersWithNodes(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node) {

	var allContainers []models.Container
	for _, node := range nodes {
		// Skip offline nodes to avoid 595 errors when trying to access their resources
		if node.Status != "online" {
			log.Debug().
				Str("node", node.Node).
				Str("status", node.Status).
				Msg("Skipping offline node for container polling")
			continue
		}
		
		containers, err := client.GetContainers(ctx, node.Node)
		if err != nil {
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_containers", instanceName, err).WithNode(node.Node)
			log.Error().Err(monErr).Str("node", node.Node).Msg("Failed to get containers")
			continue
		}

		for _, ct := range containers {
			// Skip templates if configured
			if ct.Template == 1 {
				continue
			}

			// Parse tags
			var tags []string
			if ct.Tags != "" {
				tags = strings.Split(ct.Tags, ";")
				
				// Log if Pulse-specific tags are detected
				for _, tag := range tags {
					switch tag {
					case "pulse-no-alerts", "pulse-monitor-only", "pulse-relaxed":
						log.Info().
							Str("container", ct.Name).
							Str("node", node.Node).
							Str("tag", tag).
							Msg("Pulse control tag detected on container (legacy API)")
					}
				}
			}

			// Calculate I/O rates
			// Avoid duplicating node name in ID when instance name equals node name
			var guestID string
			if instanceName == node.Node {
				guestID = fmt.Sprintf("%s-%d", node.Node, int(ct.VMID))
			} else {
				guestID = fmt.Sprintf("%s-%s-%d", instanceName, node.Node, int(ct.VMID))
			}
			currentMetrics := IOMetrics{
				DiskRead:   int64(ct.DiskRead),
				DiskWrite:  int64(ct.DiskWrite),
				NetworkIn:  int64(ct.NetIn),
				NetworkOut: int64(ct.NetOut),
				Timestamp:  time.Now(),
			}
			diskReadRate, diskWriteRate, netInRate, netOutRate := m.rateTracker.CalculateRates(guestID, currentMetrics)

			// Set CPU to 0 for non-running containers to avoid false alerts
			// Containers can have status: running, stopped, paused, suspended
			cpuUsage := safeFloat(ct.CPU)
			if ct.Status != "running" {
				cpuUsage = 0
			}

			// For containers, memory reporting is more accurate than VMs
			// ct.Mem shows actual usage for running containers
			memUsed := uint64(0)
			memTotal := ct.MaxMem
			
			if ct.Status == "running" {
				// For running containers, ct.Mem is actual usage
				memUsed = ct.Mem
			}

			// Convert -1 to nil for I/O metrics when VM is not running
			// We'll use -1 to indicate "no data" which will be converted to null for the frontend
			modelCT := models.Container{
				ID:       guestID,
				VMID:     int(ct.VMID),
				Name:     ct.Name,
				Node:     node.Node,
				Instance: instanceName,
				Status:   ct.Status,
				Type:     "lxc",
				CPU:      cpuUsage, // Already in percentage
				CPUs:     int(ct.CPUs),
				Memory: models.Memory{
					Total: int64(memTotal),
					Used:  int64(memUsed),
					Free:  int64(memTotal - memUsed),
					Usage: safePercentage(float64(memUsed), float64(memTotal)),
				},
				Disk: models.Disk{
					Total: int64(ct.MaxDisk),
					Used:  int64(ct.Disk),
					Free:  int64(ct.MaxDisk - ct.Disk),
					Usage: safePercentage(float64(ct.Disk), float64(ct.MaxDisk)),
				},
				NetworkIn:  maxInt64(0, int64(netInRate)),
				NetworkOut: maxInt64(0, int64(netOutRate)),
				DiskRead:   maxInt64(0, int64(diskReadRate)),
				DiskWrite:  maxInt64(0, int64(diskWriteRate)),
				Uptime:     int64(ct.Uptime),
				Template:   ct.Template == 1,
				Tags:       tags,
				Lock:       ct.Lock,
				LastSeen:   time.Now(),
			}
			allContainers = append(allContainers, modelCT)

			// Record metrics history
			now := time.Now()
			m.metricsHistory.AddGuestMetric(modelCT.ID, "cpu", modelCT.CPU*100, now)
			m.metricsHistory.AddGuestMetric(modelCT.ID, "memory", modelCT.Memory.Usage, now)
			m.metricsHistory.AddGuestMetric(modelCT.ID, "disk", modelCT.Disk.Usage, now)
			m.metricsHistory.AddGuestMetric(modelCT.ID, "diskread", float64(modelCT.DiskRead), now)
			m.metricsHistory.AddGuestMetric(modelCT.ID, "diskwrite", float64(modelCT.DiskWrite), now)
			m.metricsHistory.AddGuestMetric(modelCT.ID, "netin", float64(modelCT.NetworkIn), now)
			m.metricsHistory.AddGuestMetric(modelCT.ID, "netout", float64(modelCT.NetworkOut), now)

			// Check thresholds for alerts
			log.Info().Str("container", modelCT.Name).Msg("Checking container alerts")
			m.alertManager.CheckGuest(modelCT, instanceName)
		}
	}

	m.state.UpdateContainersForInstance(instanceName, allContainers)
}

// pollStorage polls storage from a PVE instance
// Deprecated: This function should not be called directly as it causes duplicate GetNodes calls.
// Use pollStorageWithNodes instead.
func (m *Monitor) pollStorage(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Warn().Str("instance", instanceName).Msg("pollStorage called directly - this causes duplicate GetNodes calls and syslog spam on non-clustered nodes")

	// Get all nodes first
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		monErr := errors.WrapConnectionError("get_nodes_for_storage", instanceName, err)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get nodes for storage polling")
		return
	}

	m.pollStorageWithNodes(ctx, instanceName, client, nodes)
}

// pollStorageWithNodes polls storage using a provided nodes list to avoid duplicate GetNodes calls
func (m *Monitor) pollStorageWithNodes(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node) {

	// Get cluster storage configuration for shared/enabled status
	clusterStorages, err := client.GetAllStorage(ctx)
	clusterStorageAvailable := err == nil
	if err != nil {
		monErr := errors.WrapAPIError("get_cluster_storage", instanceName, err, 0)
		log.Warn().Err(monErr).Str("instance", instanceName).Msg("Failed to get cluster storage config - will continue with node storage only")
	}

	// Create a map for quick lookup of cluster storage config
	clusterStorageMap := make(map[string]proxmox.Storage)
	if clusterStorageAvailable {
		for _, cs := range clusterStorages {
			clusterStorageMap[cs.Storage] = cs
		}
	}

	var allStorage []models.Storage
	seenStorage := make(map[string]bool)

	// Get storage from each node (this includes capacity info)
	log.Debug().Str("instance", instanceName).Int("nodeCount", len(nodes)).Msg("Starting storage polling for nodes")
	
	for _, node := range nodes {
		// Skip offline nodes to avoid 595 errors when trying to access their resources
		if node.Status != "online" {
			log.Debug().
				Str("node", node.Node).
				Str("status", node.Status).
				Msg("Skipping offline node for storage polling")
			continue
		}
		
		log.Debug().Str("node", node.Node).Msg("Getting storage for node")
		nodeStorage, err := client.GetStorage(ctx, node.Node)
		if err != nil {
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_node_storage", instanceName, err).WithNode(node.Node)
			log.Warn().Err(monErr).Str("node", node.Node).Msg("Failed to get node storage - continuing with other nodes")
			continue
		}
		
		log.Debug().Str("node", node.Node).Int("storageCount", len(nodeStorage)).Msg("Retrieved storage for node")

		for _, storage := range nodeStorage {
			// Get cluster config for this storage
			clusterConfig, hasClusterConfig := clusterStorageMap[storage.Storage]

			// Determine if shared
			shared := hasClusterConfig && clusterConfig.Shared == 1

			// For shared storage, only include it once
			storageKey := storage.Storage
			if shared {
				if seenStorage[storageKey] {
					continue
				}
				seenStorage[storageKey] = true
			}

			// Use appropriate node name
			nodeID := node.Node
			storageID := fmt.Sprintf("%s-%s-%s", instanceName, nodeID, storage.Storage)
			if shared {
				nodeID = "shared"
				// Use instance-specific ID for shared storage to prevent conflicts between clusters
				storageID = fmt.Sprintf("%s-shared-%s", instanceName, storage.Storage)
			}

			modelStorage := models.Storage{
				ID:       storageID,
				Name:     storage.Storage,
				Node:     nodeID,
				Instance: instanceName,
				Type:     storage.Type,
				Status:   "available",
				Total:    int64(storage.Total),
				Used:     int64(storage.Used),
				Free:     int64(storage.Available),
				Usage:    0,
				Content:  sortContent(storage.Content),
				Shared:   shared,
				Enabled:  true,
				Active:   true,
			}

			// Override with cluster config if available
			if hasClusterConfig {
				// Sort content values for consistent display
				if clusterConfig.Content != "" {
					contentParts := strings.Split(clusterConfig.Content, ",")
					sort.Strings(contentParts)
					modelStorage.Content = strings.Join(contentParts, ",")
				} else {
					modelStorage.Content = clusterConfig.Content
				}
				modelStorage.Enabled = clusterConfig.Enabled == 1
				modelStorage.Active = clusterConfig.Active == 1
			}

			// Calculate usage percentage
			if modelStorage.Total > 0 {
				modelStorage.Usage = safePercentage(float64(modelStorage.Used), float64(modelStorage.Total))
			}

			// Determine status based on active/enabled flags
			if storage.Active == 1 || modelStorage.Active {
				modelStorage.Status = "available"
			} else if modelStorage.Enabled {
				modelStorage.Status = "inactive"
			} else {
				modelStorage.Status = "disabled"
			}

			allStorage = append(allStorage, modelStorage)

			// Record storage metrics history
			now := time.Now()
			m.metricsHistory.AddStorageMetric(modelStorage.ID, "usage", modelStorage.Usage, now)
			m.metricsHistory.AddStorageMetric(modelStorage.ID, "used", float64(modelStorage.Used), now)
			m.metricsHistory.AddStorageMetric(modelStorage.ID, "total", float64(modelStorage.Total), now)
			m.metricsHistory.AddStorageMetric(modelStorage.ID, "avail", float64(modelStorage.Free), now)

			// Check thresholds for alerts
			m.alertManager.CheckStorage(modelStorage)
		}
	}

	// Update storage for this instance only
	var instanceStorage []models.Storage
	for _, st := range allStorage {
		st.Instance = instanceName
		instanceStorage = append(instanceStorage, st)
	}
	
	log.Info().
		Str("instance", instanceName).
		Int("storageCount", len(instanceStorage)).
		Bool("clusterConfigAvailable", clusterStorageAvailable).
		Msg("Updating storage for instance")
		
	m.state.UpdateStorageForInstance(instanceName, instanceStorage)
}

// pollBackupTasks polls backup tasks from a PVE instance
func (m *Monitor) pollBackupTasks(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Debug().Str("instance", instanceName).Msg("Polling backup tasks")

	tasks, err := client.GetBackupTasks(ctx)
	if err != nil {
		monErr := errors.WrapAPIError("get_backup_tasks", instanceName, err, 0)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get backup tasks")
		return
	}

	var backupTasks []models.BackupTask
	for _, task := range tasks {
		// Extract VMID from task ID (format: "UPID:node:pid:starttime:type:vmid:user@realm:")
		vmid := 0
		if task.ID != "" {
			if vmidInt, err := strconv.Atoi(task.ID); err == nil {
				vmid = vmidInt
			}
		}

		taskID := fmt.Sprintf("%s-%s", instanceName, task.UPID)

		backupTask := models.BackupTask{
			ID:        taskID,
			Node:      task.Node,
			Type:      task.Type,
			VMID:      vmid,
			Status:    task.Status,
			StartTime: time.Unix(task.StartTime, 0),
		}

		if task.EndTime > 0 {
			backupTask.EndTime = time.Unix(task.EndTime, 0)
		}

		backupTasks = append(backupTasks, backupTask)
	}

	// Update state with new backup tasks for this instance
	m.state.UpdateBackupTasksForInstance(instanceName, backupTasks)
}

// pollPBSInstance polls a single PBS instance
func (m *Monitor) pollPBSInstance(ctx context.Context, instanceName string, client *pbs.Client) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		log.Debug().Str("instance", instanceName).Msg("Polling cancelled")
		return
	default:
	}

	log.Debug().Str("instance", instanceName).Msg("Polling PBS instance")

	// Get instance config
	var instanceCfg *config.PBSInstance
	for _, cfg := range m.config.PBSInstances {
		if cfg.Name == instanceName {
			instanceCfg = &cfg
			log.Debug().
				Str("instance", instanceName).
				Bool("monitorDatastores", cfg.MonitorDatastores).
				Msg("Found PBS instance config")
			break
		}
	}
	if instanceCfg == nil {
		log.Error().Str("instance", instanceName).Msg("PBS instance config not found")
		return
	}

	// Initialize PBS instance with default values
	pbsInst := models.PBSInstance{
		ID:               "pbs-" + instanceName,
		Name:             instanceName,
		Host:             instanceCfg.Host,
		Status:           "offline",
		Version:          "unknown",
		ConnectionHealth: "unhealthy",
		LastSeen:         time.Now(),
	}

	// Try to get version first
	version, versionErr := client.GetVersion(ctx)
	if versionErr == nil {
		// Version succeeded - PBS is online
		pbsInst.Status = "online"
		pbsInst.Version = version.Version
		pbsInst.ConnectionHealth = "healthy"
		m.resetAuthFailures(instanceName, "pbs")
		m.state.SetConnectionHealth("pbs-"+instanceName, true)
		
		log.Debug().
			Str("instance", instanceName).
			Str("version", version.Version).
			Bool("monitorDatastores", instanceCfg.MonitorDatastores).
			Msg("PBS version retrieved successfully")
	} else {
		log.Debug().Err(versionErr).Str("instance", instanceName).Msg("Failed to get PBS version, trying fallback")
		
		// Version failed, try datastores as fallback (like test connection does)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		
		_, datastoreErr := client.GetDatastores(ctx2)
		if datastoreErr == nil {
			// Datastores succeeded - PBS is online but version unavailable
			pbsInst.Status = "online"
			pbsInst.Version = "connected"
			pbsInst.ConnectionHealth = "healthy"
			m.resetAuthFailures(instanceName, "pbs")
			m.state.SetConnectionHealth("pbs-"+instanceName, true)
			
			log.Info().
				Str("instance", instanceName).
				Msg("PBS connected (version unavailable but datastores accessible)")
		} else {
			// Both failed - PBS is offline
			pbsInst.Status = "offline"
			pbsInst.ConnectionHealth = "error"
			monErr := errors.WrapConnectionError("get_pbs_version", instanceName, versionErr)
			log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to connect to PBS")
			m.state.SetConnectionHealth("pbs-"+instanceName, false)
			
			// Track auth failure if it's an authentication error
			if errors.IsAuthError(versionErr) || errors.IsAuthError(datastoreErr) {
				m.recordAuthFailure(instanceName, "pbs")
				// Don't continue if auth failed
				return
			}
		}
	}

	// Get node status (CPU, memory, etc.)
	// Note: This requires Sys.Audit permission on PBS which read-only tokens often don't have
	nodeStatus, err := client.GetNodeStatus(ctx)
	if err != nil {
		// Log as debug instead of error since this is often a permission issue
		log.Debug().Err(err).Str("instance", instanceName).Msg("Could not get PBS node status (may need Sys.Audit permission)")
	} else {
		pbsInst.CPU = nodeStatus.CPU
		if nodeStatus.Memory.Total > 0 {
			pbsInst.Memory = float64(nodeStatus.Memory.Used) / float64(nodeStatus.Memory.Total) * 100
			pbsInst.MemoryUsed = nodeStatus.Memory.Used
			pbsInst.MemoryTotal = nodeStatus.Memory.Total
		}
		pbsInst.Uptime = nodeStatus.Uptime

		log.Debug().
			Str("instance", instanceName).
			Float64("cpu", pbsInst.CPU).
			Float64("memory", pbsInst.Memory).
			Int64("uptime", pbsInst.Uptime).
			Msg("PBS node status retrieved")
	}

	// Poll datastores if enabled
	log.Debug().Bool("monitorDatastores", instanceCfg.MonitorDatastores).Str("instance", instanceName).Msg("Checking if datastore monitoring is enabled")
	if instanceCfg.MonitorDatastores {
		datastores, err := client.GetDatastores(ctx)
		if err != nil {
			monErr := errors.WrapAPIError("get_datastores", instanceName, err, 0)
			log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get datastores")
		} else {
			log.Info().
				Str("instance", instanceName).
				Int("count", len(datastores)).
				Msg("Got PBS datastores")

			for _, ds := range datastores {
				// Use whichever fields are populated
				total := ds.Total
				if total == 0 && ds.TotalSpace > 0 {
					total = ds.TotalSpace
				}
				used := ds.Used
				if used == 0 && ds.UsedSpace > 0 {
					used = ds.UsedSpace
				}
				avail := ds.Avail
				if avail == 0 && ds.AvailSpace > 0 {
					avail = ds.AvailSpace
				}

				// If still 0, try to calculate from each other
				if total == 0 && used > 0 && avail > 0 {
					total = used + avail
				}

				log.Debug().
					Str("store", ds.Store).
					Int64("total", total).
					Int64("used", used).
					Int64("avail", avail).
					Int64("orig_total", ds.Total).
					Int64("orig_total_space", ds.TotalSpace).
					Msg("PBS datastore details")

				modelDS := models.PBSDatastore{
					Name:   ds.Store,
					Total:  total,
					Used:   used,
					Free:   avail,
					Usage:  safePercentage(float64(used), float64(total)),
					Status: "available",
					DeduplicationFactor: ds.DeduplicationFactor,
				}

				// Discover namespaces for this datastore
				namespaces, err := client.ListNamespaces(ctx, ds.Store, "", 0)
				if err != nil {
					log.Warn().Err(err).
						Str("instance", instanceName).
						Str("datastore", ds.Store).
						Msg("Failed to list namespaces")
				} else {
					// Convert PBS namespaces to model namespaces
					for _, ns := range namespaces {
						nsPath := ns.NS
						if nsPath == "" {
							nsPath = ns.Path
						}
						if nsPath == "" {
							nsPath = ns.Name
						}

						modelNS := models.PBSNamespace{
							Path:   nsPath,
							Parent: ns.Parent,
							Depth:  strings.Count(nsPath, "/"),
						}
						modelDS.Namespaces = append(modelDS.Namespaces, modelNS)
					}

					// Always include root namespace
					hasRoot := false
					for _, ns := range modelDS.Namespaces {
						if ns.Path == "" {
							hasRoot = true
							break
						}
					}
					if !hasRoot {
						modelDS.Namespaces = append([]models.PBSNamespace{{Path: "", Depth: 0}}, modelDS.Namespaces...)
					}
				}

				pbsInst.Datastores = append(pbsInst.Datastores, modelDS)
			}
		}
	}

	// Update state - merge with existing instances
	m.state.UpdatePBSInstance(pbsInst)
	log.Info().
		Str("instance", instanceName).
		Str("id", pbsInst.ID).
		Int("datastores", len(pbsInst.Datastores)).
		Msg("PBS instance updated in state")

	// Check PBS metrics against alert thresholds
	if m.alertManager != nil {
		m.alertManager.CheckPBS(pbsInst)
	}

	// Poll backups if enabled
	if instanceCfg.MonitorBackups {
		log.Info().
			Str("instance", instanceName).
			Int("datastores", len(pbsInst.Datastores)).
			Msg("Polling PBS backups")
		m.pollPBSBackups(ctx, instanceName, client, pbsInst.Datastores)
	} else {
		log.Debug().
			Str("instance", instanceName).
			Msg("PBS backup monitoring disabled")
	}
}

// GetState returns the current state
func (m *Monitor) GetState() models.StateSnapshot {
	// Check if mock mode is enabled
	if mock.IsMockEnabled() {
		return mock.GetMockState()
	}
	return m.state.GetSnapshot()
}

// GetStartTime returns the monitor start time
func (m *Monitor) GetStartTime() time.Time {
	return m.startTime
}

// GetDiscoveryService returns the discovery service
func (m *Monitor) GetDiscoveryService() *discovery.Service {
	return m.discoveryService
}

// StartDiscoveryService starts the discovery service if not already running
func (m *Monitor) StartDiscoveryService(ctx context.Context, wsHub *websocket.Hub, subnet string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.discoveryService != nil {
		log.Debug().Msg("Discovery service already running")
		return
	}
	
	if subnet == "" {
		subnet = "auto"
	}
	
	m.discoveryService = discovery.NewService(wsHub, 5*time.Minute, subnet)
	if m.discoveryService != nil {
		m.discoveryService.Start(ctx)
		log.Info().Str("subnet", subnet).Msg("Discovery service started")
	} else {
		log.Error().Msg("Failed to create discovery service")
	}
}

// StopDiscoveryService stops the discovery service if running
func (m *Monitor) StopDiscoveryService() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.discoveryService != nil {
		m.discoveryService.Stop()
		m.discoveryService = nil
		log.Info().Msg("Discovery service stopped")
	}
}

// GetGuestMetrics returns historical metrics for a guest
func (m *Monitor) GetGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint {
	return m.metricsHistory.GetAllGuestMetrics(guestID, duration)
}

// GetNodeMetrics returns historical metrics for a node
func (m *Monitor) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
}

// GetStorageMetrics returns historical metrics for storage
func (m *Monitor) GetStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	return m.metricsHistory.GetAllStorageMetrics(storageID, duration)
}

// GetAlertManager returns the alert manager
func (m *Monitor) GetAlertManager() *alerts.Manager {
	return m.alertManager
}

// GetNotificationManager returns the notification manager
func (m *Monitor) GetNotificationManager() *notifications.NotificationManager {
	return m.notificationMgr
}

// GetConfigPersistence returns the config persistence manager
func (m *Monitor) GetConfigPersistence() *config.ConfigPersistence {
	return m.configPersist
}

// pollStorageBackups polls backup files from storage
// Deprecated: This function should not be called directly as it causes duplicate GetNodes calls.
// Use pollStorageBackupsWithNodes instead.
func (m *Monitor) pollStorageBackups(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Warn().Str("instance", instanceName).Msg("pollStorageBackups called directly - this causes duplicate GetNodes calls and syslog spam on non-clustered nodes")

	// Get all nodes
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		monErr := errors.WrapConnectionError("get_nodes_for_backups", instanceName, err)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("Failed to get nodes for backup polling")
		return
	}

	m.pollStorageBackupsWithNodes(ctx, instanceName, client, nodes)
}

// pollStorageBackupsWithNodes polls backups using a provided nodes list to avoid duplicate GetNodes calls
func (m *Monitor) pollStorageBackupsWithNodes(ctx context.Context, instanceName string, client PVEClientInterface, nodes []proxmox.Node) {

	var allBackups []models.StorageBackup
	seenVolids := make(map[string]bool) // Track seen volume IDs to avoid duplicates

	// For each node, get storage and check content
	for _, node := range nodes {
		if node.Status != "online" {
			continue
		}

		// Get storage for this node - retry once on timeout
		var storages []proxmox.Storage
		var err error
		
		for attempt := 1; attempt <= 2; attempt++ {
			storages, err = client.GetStorage(ctx, node.Node)
			if err == nil {
				break // Success
			}
			
			// Check if it's a timeout error
			if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
				if attempt == 1 {
					log.Warn().
						Str("node", node.Node).
						Str("instance", instanceName).
						Msg("Storage query timed out, retrying with extended timeout...")
					// Give it a bit more time on retry
					time.Sleep(2 * time.Second)
					continue
				}
			}
			// Non-timeout error or second attempt failed
			break
		}
		
		if err != nil {
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_storage_for_backups", instanceName, err).WithNode(node.Node)
			log.Warn().Err(monErr).Str("node", node.Node).Msg("Failed to get storage for backups - skipping node")
			continue
		}

		// For each storage that can contain backups or templates
		for _, storage := range storages {
			// Check if storage supports backup content
			if !strings.Contains(storage.Content, "backup") {
				continue
			}

			// Get storage content
			contents, err := client.GetStorageContent(ctx, node.Node, storage.Storage)
			if err != nil {
				monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_storage_content", instanceName, err).WithNode(node.Node)
				log.Debug().Err(monErr).
					Str("node", node.Node).
					Str("storage", storage.Storage).
					Msg("Failed to get storage content")
				continue
			}

			// Convert to models
			for _, content := range contents {
				// Skip if we've already seen this item (shared storage duplicate)
				if seenVolids[content.Volid] {
					continue
				}
				seenVolids[content.Volid] = true

				// Skip templates and ISOs - they're not backups
				if content.Content == "vztmpl" || content.Content == "iso" {
					continue
				}

				// Determine type from content type and volid
				backupType := "unknown"
				// Check for PMG host backups (VMID=0 and contains pmgbackup)
				if content.VMID == 0 && strings.Contains(content.Volid, "pmgbackup") {
					backupType = "host"
				} else if strings.Contains(content.Volid, "/vm/") || strings.Contains(content.Volid, "qemu") {
					backupType = "qemu"
				} else if strings.Contains(content.Volid, "/ct/") || strings.Contains(content.Volid, "lxc") {
					backupType = "lxc"
				} else if strings.Contains(content.Format, "pbs-ct") {
					// PBS format check as fallback
					backupType = "lxc"
				} else if strings.Contains(content.Format, "pbs-vm") {
					// PBS format check as fallback
					backupType = "qemu"
				} else if content.VMID == 0 {
					// Any other VMID=0 backup is likely a host backup
					backupType = "host"
				}

				// Always use the actual node name
				backupNode := node.Node
				isPBSStorage := strings.HasPrefix(storage.Storage, "pbs-") || storage.Type == "pbs"

				// Check verification status for PBS backups
				verified := false
				verificationInfo := ""
				if isPBSStorage {
					// Check if verified flag is set
					if content.Verified > 0 {
						verified = true
					}
					// Also check verification map if available
					if content.Verification != nil {
						if state, ok := content.Verification["state"].(string); ok {
							verified = (state == "ok")
							verificationInfo = state
						}
					}
				}

				backup := models.StorageBackup{
					ID:           fmt.Sprintf("%s-%s", instanceName, content.Volid),
					Storage:      storage.Storage,
					Node:         backupNode,
					Type:         backupType,
					VMID:         content.VMID,
					Time:         time.Unix(content.CTime, 0),
					CTime:        content.CTime,
					Size:         int64(content.Size),
					Format:       content.Format,
					Notes:        content.Notes,
					Protected:    content.Protected > 0,
					Volid:        content.Volid,
					IsPBS:        isPBSStorage,
					Verified:     verified,
					Verification: verificationInfo,
				}

				allBackups = append(allBackups, backup)
			}
		}
	}

	// Update state with storage backups for this instance
	m.state.UpdateStorageBackupsForInstance(instanceName, allBackups)

	log.Debug().
		Str("instance", instanceName).
		Int("count", len(allBackups)).
		Msg("Storage backups polled")
}

// pollGuestSnapshots polls snapshots for all VMs and containers
func (m *Monitor) pollGuestSnapshots(ctx context.Context, instanceName string, client PVEClientInterface) {
	log.Debug().Str("instance", instanceName).Msg("Polling guest snapshots")

	// Create a separate context with a longer timeout for snapshot queries
	// Snapshot queries can be slow, especially with many VMs/containers
	snapshotCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get current VMs and containers from state
	m.mu.RLock()
	vms := append([]models.VM{}, m.state.VMs...)
	containers := append([]models.Container{}, m.state.Containers...)
	m.mu.RUnlock()

	var allSnapshots []models.GuestSnapshot

	// Poll VM snapshots
	for _, vm := range vms {
		// Skip templates
		if vm.Template {
			continue
		}

		snapshots, err := client.GetVMSnapshots(snapshotCtx, vm.Node, vm.VMID)
		if err != nil {
			// This is common for VMs without snapshots, so use debug level
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_vm_snapshots", instanceName, err).WithNode(vm.Node)
			log.Debug().
				Err(monErr).
				Str("node", vm.Node).
				Int("vmid", vm.VMID).
				Msg("Failed to get VM snapshots")
			continue
		}

		for _, snap := range snapshots {
			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("%s-%s-%d-%s", instanceName, vm.Node, vm.VMID, snap.Name),
				Name:        snap.Name,
				Node:        vm.Node,
				Type:        "qemu",
				VMID:        vm.VMID,
				Time:        time.Unix(snap.SnapTime, 0),
				Description: snap.Description,
				Parent:      snap.Parent,
				VMState:     true, // VM state support enabled
			}

			allSnapshots = append(allSnapshots, snapshot)
		}
	}

	// Poll container snapshots
	for _, ct := range containers {
		// Skip templates
		if ct.Template {
			continue
		}

		snapshots, err := client.GetContainerSnapshots(snapshotCtx, ct.Node, ct.VMID)
		if err != nil {
			// API error 596 means snapshots not supported/available - this is expected for many containers
			errStr := err.Error()
			if strings.Contains(errStr, "596") || strings.Contains(errStr, "not available") {
				// Silently skip containers without snapshot support
				continue
			}
			// Log other errors at debug level
			monErr := errors.NewMonitorError(errors.ErrorTypeAPI, "get_container_snapshots", instanceName, err).WithNode(ct.Node)
			log.Debug().
				Err(monErr).
				Str("node", ct.Node).
				Int("vmid", ct.VMID).
				Msg("Failed to get container snapshots")
			continue
		}

		for _, snap := range snapshots {
			snapshot := models.GuestSnapshot{
				ID:          fmt.Sprintf("%s-%s-%d-%s", instanceName, ct.Node, ct.VMID, snap.Name),
				Name:        snap.Name,
				Node:        ct.Node,
				Type:        "lxc",
				VMID:        ct.VMID,
				Time:        time.Unix(snap.SnapTime, 0),
				Description: snap.Description,
				Parent:      snap.Parent,
				VMState:     false,
			}

			allSnapshots = append(allSnapshots, snapshot)
		}
	}

	// Update state with guest snapshots for this instance
	m.state.UpdateGuestSnapshotsForInstance(instanceName, allSnapshots)

	log.Debug().
		Str("instance", instanceName).
		Int("count", len(allSnapshots)).
		Msg("Guest snapshots polled")
}

// Stop gracefully stops the monitor
func (m *Monitor) Stop() {
	log.Info().Msg("Stopping monitor")

	// Stop the alert manager to save history
	if m.alertManager != nil {
		m.alertManager.Stop()
	}

	// Stop notification manager
	if m.notificationMgr != nil {
		m.notificationMgr.Stop()
	}

	log.Info().Msg("Monitor stopped")
}

// recordAuthFailure records an authentication failure for a node
func (m *Monitor) recordAuthFailure(instanceName string, nodeType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeID := instanceName
	if nodeType != "" {
		nodeID = nodeType + "-" + instanceName
	}

	// Increment failure count
	m.authFailures[nodeID]++
	m.lastAuthAttempt[nodeID] = time.Now()

	log.Warn().
		Str("node", nodeID).
		Int("failures", m.authFailures[nodeID]).
		Msg("Authentication failure recorded")

	// If we've exceeded the threshold, remove the node
	const maxAuthFailures = 5
	if m.authFailures[nodeID] >= maxAuthFailures {
		log.Error().
			Str("node", nodeID).
			Int("failures", m.authFailures[nodeID]).
			Msg("Maximum authentication failures reached, removing node from state")

		// Remove from state based on type
		if nodeType == "pve" {
			m.removeFailedPVENode(instanceName)
		} else if nodeType == "pbs" {
			m.removeFailedPBSNode(instanceName)
		}

		// Reset the counter since we've removed the node
		delete(m.authFailures, nodeID)
		delete(m.lastAuthAttempt, nodeID)
	}
}

// resetAuthFailures resets the failure count for a node after successful auth
func (m *Monitor) resetAuthFailures(instanceName string, nodeType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeID := instanceName
	if nodeType != "" {
		nodeID = nodeType + "-" + instanceName
	}

	if count, exists := m.authFailures[nodeID]; exists && count > 0 {
		log.Info().
			Str("node", nodeID).
			Int("previousFailures", count).
			Msg("Authentication succeeded, resetting failure count")

		delete(m.authFailures, nodeID)
		delete(m.lastAuthAttempt, nodeID)
	}
}

// removeFailedPVENode updates a PVE node to show failed authentication status
func (m *Monitor) removeFailedPVENode(instanceName string) {
	// Get instance config to get host URL
	var hostURL string
	for _, cfg := range m.config.PVEInstances {
		if cfg.Name == instanceName {
			hostURL = cfg.Host
			break
		}
	}
	
	// Create a failed node entry to show in UI with error status
	failedNode := models.Node{
		ID:               instanceName + "-failed",
		Name:             instanceName,
		Instance:         instanceName,
		Host:             hostURL,  // Include host URL even for failed nodes
		Status:           "offline",
		Type:             "node",
		ConnectionHealth: "error",
		LastSeen:         time.Now(),
		// Set other fields to zero values to indicate no data
		CPU:    0,
		Memory: models.Memory{},
		Disk:   models.Disk{},
	}

	// Update with just the failed node
	m.state.UpdateNodesForInstance(instanceName, []models.Node{failedNode})

	// Remove all other resources associated with this instance
	m.state.UpdateVMsForInstance(instanceName, []models.VM{})
	m.state.UpdateContainersForInstance(instanceName, []models.Container{})
	m.state.UpdateStorageForInstance(instanceName, []models.Storage{})
	m.state.UpdateBackupTasksForInstance(instanceName, []models.BackupTask{})
	m.state.UpdateStorageBackupsForInstance(instanceName, []models.StorageBackup{})
	m.state.UpdateGuestSnapshotsForInstance(instanceName, []models.GuestSnapshot{})

	// Set connection health to false
	m.state.SetConnectionHealth(instanceName, false)
}

// removeFailedPBSNode removes a PBS node and all its resources from state
func (m *Monitor) removeFailedPBSNode(instanceName string) {
	// Remove PBS instance by passing empty array
	currentInstances := m.state.PBSInstances
	var updatedInstances []models.PBSInstance
	for _, inst := range currentInstances {
		if inst.Name != instanceName {
			updatedInstances = append(updatedInstances, inst)
		}
	}
	m.state.UpdatePBSInstances(updatedInstances)

	// Remove PBS backups
	m.state.UpdatePBSBackups(instanceName, []models.PBSBackup{})

	// Set connection health to false
	m.state.SetConnectionHealth("pbs-"+instanceName, false)
}

// pollPBSBackups fetches all backups from PBS datastores
func (m *Monitor) pollPBSBackups(ctx context.Context, instanceName string, client *pbs.Client, datastores []models.PBSDatastore) {
	log.Debug().Str("instance", instanceName).Msg("Polling PBS backups")

	var allBackups []models.PBSBackup

	// Process each datastore
	for _, ds := range datastores {
		// Get namespace paths
		namespacePaths := make([]string, 0, len(ds.Namespaces))
		for _, ns := range ds.Namespaces {
			namespacePaths = append(namespacePaths, ns.Path)
		}

		log.Info().
			Str("instance", instanceName).
			Str("datastore", ds.Name).
			Int("namespaces", len(namespacePaths)).
			Strs("namespace_paths", namespacePaths).
			Msg("Processing datastore namespaces")

		// Fetch backups from all namespaces concurrently
		backupsMap, err := client.ListAllBackups(ctx, ds.Name, namespacePaths)
		if err != nil {
			log.Error().Err(err).
				Str("instance", instanceName).
				Str("datastore", ds.Name).
				Msg("Failed to fetch PBS backups")
			continue
		}

		// Convert PBS backups to model backups
		for namespace, snapshots := range backupsMap {
			for _, snapshot := range snapshots {
				backupTime := time.Unix(snapshot.BackupTime, 0)

				// Generate unique ID
				id := fmt.Sprintf("pbs-%s-%s-%s-%s-%s-%d",
					instanceName, ds.Name, namespace,
					snapshot.BackupType, snapshot.BackupID,
					snapshot.BackupTime)

				// Extract file names from files (which can be strings or objects)
				var fileNames []string
				for _, file := range snapshot.Files {
					switch f := file.(type) {
					case string:
						fileNames = append(fileNames, f)
					case map[string]interface{}:
						if filename, ok := f["filename"].(string); ok {
							fileNames = append(fileNames, filename)
						}
					}
				}

				// Extract verification status
				verified := false
				if snapshot.Verification != nil {
					switch v := snapshot.Verification.(type) {
					case string:
						verified = v == "ok"
					case map[string]interface{}:
						if state, ok := v["state"].(string); ok {
							verified = state == "ok"
						}
					}

					// Debug log verification data
					log.Debug().
						Str("vmid", snapshot.BackupID).
						Int64("time", snapshot.BackupTime).
						Interface("verification", snapshot.Verification).
						Bool("verified", verified).
						Msg("PBS backup verification status")
				}

				backup := models.PBSBackup{
					ID:         id,
					Instance:   instanceName,
					Datastore:  ds.Name,
					Namespace:  namespace,
					BackupType: snapshot.BackupType,
					VMID:       snapshot.BackupID,
					BackupTime: backupTime,
					Size:       snapshot.Size,
					Protected:  snapshot.Protected,
					Verified:   verified,
					Comment:    snapshot.Comment,
					Files:      fileNames,
					Owner:      snapshot.Owner,
				}

				allBackups = append(allBackups, backup)
			}
		}
	}

	log.Info().
		Str("instance", instanceName).
		Int("count", len(allBackups)).
		Msg("PBS backups fetched")

	// Update state
	m.state.UpdatePBSBackups(instanceName, allBackups)
}

// checkMockAlerts checks alerts for mock data
func (m *Monitor) checkMockAlerts() {
	if !mock.IsMockEnabled() {
		return
	}
	
	// Get mock state
	state := mock.GetMockState()
	
	log.Info().
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Msg("Checking alerts for mock data")
	
	// Check alerts for each VM
	for _, vm := range state.VMs {
		m.alertManager.CheckGuest(vm, "mock")
	}
	
	// Check alerts for each container
	for _, container := range state.Containers {
		m.alertManager.CheckGuest(container, "mock")
	}
	
	// Check alerts for each node
	for _, node := range state.Nodes {
		m.alertManager.CheckNode(node)
	}
	
	// Check alerts for storage
	for _, storage := range state.Storage {
		m.alertManager.CheckStorage(storage)
	}
}
