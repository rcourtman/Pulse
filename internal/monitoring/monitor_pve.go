package monitoring

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func sortContent(content string) string {
	if content == "" {
		return ""
	}
	parts := strings.Split(content, ",")
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (m *Monitor) enrichContainerMetadata(ctx context.Context, client PVEClientInterface, instanceName, nodeName string, container *models.Container) {
	if container == nil {
		return
	}

	ensureContainerRootDiskEntry(container)

	if client == nil {
		return
	}

	isRunning := container.Status == "running"

	var status *proxmox.Container
	if isRunning {
		statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		statusResp, err := client.GetContainerStatus(statusCtx, nodeName, container.VMID)
		cancel()
		if err != nil {
			log.Debug().
				Err(err).
				Str("instance", instanceName).
				Str("node", nodeName).
				Str("container", container.Name).
				Int("vmid", container.VMID).
				Msg("Container status metadata unavailable")
		} else {
			status = statusResp
		}
	}

	rootDeviceHint := ""
	var mountMetadata map[string]containerMountMetadata
	addressSet := make(map[string]struct{})
	addressOrder := make([]string, 0, 4)

	addAddress := func(addr string) {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			return
		}
		if _, exists := addressSet[addr]; exists {
			return
		}
		addressSet[addr] = struct{}{}
		addressOrder = append(addressOrder, addr)
	}

	if status != nil {
		for _, addr := range sanitizeGuestAddressStrings(status.IP) {
			addAddress(addr)
		}
		for _, addr := range sanitizeGuestAddressStrings(status.IP6) {
			addAddress(addr)
		}
		for _, addr := range parseContainerRawIPs(status.IPv4) {
			addAddress(addr)
		}
		for _, addr := range parseContainerRawIPs(status.IPv6) {
			addAddress(addr)
		}
	}

	networkIfaces := make([]models.GuestNetworkInterface, 0, 4)
	if status != nil {
		networkIfaces = make([]models.GuestNetworkInterface, 0, len(status.Network))
		for rawName, cfg := range status.Network {
			if cfg == (proxmox.ContainerNetworkConfig{}) {
				continue
			}

			iface := models.GuestNetworkInterface{}
			name := strings.TrimSpace(cfg.Name)
			if name == "" {
				name = strings.TrimSpace(rawName)
			}
			if name != "" {
				iface.Name = name
			}
			if mac := strings.TrimSpace(cfg.HWAddr); mac != "" {
				iface.MAC = mac
			}

			addrCandidates := make([]string, 0, 4)
			addrCandidates = append(addrCandidates, collectIPsFromInterface(cfg.IP)...)
			addrCandidates = append(addrCandidates, collectIPsFromInterface(cfg.IP6)...)
			addrCandidates = append(addrCandidates, collectIPsFromInterface(cfg.IPv4)...)
			addrCandidates = append(addrCandidates, collectIPsFromInterface(cfg.IPv6)...)

			if len(addrCandidates) > 0 {
				deduped := dedupeStringsPreserveOrder(addrCandidates)
				if len(deduped) > 0 {
					iface.Addresses = deduped
					for _, addr := range deduped {
						addAddress(addr)
					}
				}
			}

			if iface.Name != "" || iface.MAC != "" || len(iface.Addresses) > 0 {
				networkIfaces = append(networkIfaces, iface)
			}
		}
	}

	configCtx, cancelConfig := context.WithTimeout(ctx, 5*time.Second)
	configData, configErr := client.GetContainerConfig(configCtx, nodeName, container.VMID)
	cancelConfig()
	if configErr != nil {
		log.Debug().
			Err(configErr).
			Str("instance", instanceName).
			Str("node", nodeName).
			Str("container", container.Name).
			Int("vmid", container.VMID).
			Msg("Container config metadata unavailable")
	} else if len(configData) > 0 {
		mountMetadata = parseContainerMountMetadata(configData)
		if rootDeviceHint == "" {
			if meta, ok := mountMetadata["rootfs"]; ok && meta.Source != "" {
				rootDeviceHint = meta.Source
			}
		}
		if rootDeviceHint == "" {
			if hint := extractContainerRootDeviceFromConfig(configData); hint != "" {
				rootDeviceHint = hint
			}
		}
		for _, detail := range parseContainerConfigNetworks(configData) {
			if len(detail.Addresses) > 0 {
				for _, addr := range detail.Addresses {
					addAddress(addr)
				}
			}
			mergeContainerNetworkInterface(&networkIfaces, detail)
		}
		// Extract OS type from container config
		if osName := extractContainerOSType(configData); osName != "" {
			container.OSName = osName
		}
		// Detect OCI containers (Proxmox VE 9.1+)
		// Method 1: Check ostemplate for OCI registry patterns
		if osTemplate := extractContainerOSTemplate(configData); osTemplate != "" {
			container.OSTemplate = osTemplate
			if isOCITemplate(osTemplate) {
				container.IsOCI = true
				container.Type = "oci"
				log.Debug().
					Str("container", container.Name).
					Int("vmid", container.VMID).
					Str("osTemplate", osTemplate).
					Msg("Detected OCI container by template")
			}
		}
		// Method 2: Check config fields (entrypoint, ostype, cmode)
		// This is needed because Proxmox doesn't persist ostemplate after creation
		if !container.IsOCI && isOCIContainerByConfig(configData) {
			container.IsOCI = true
			container.Type = "oci"
			log.Debug().
				Str("container", container.Name).
				Int("vmid", container.VMID).
				Msg("Detected OCI container by config (entrypoint/ostype)")
		}
	}

	if len(addressOrder) == 0 {
		if isRunning {
			interfacesCtx, cancelInterfaces := context.WithTimeout(ctx, 5*time.Second)
			ifaceDetails, ifaceErr := client.GetContainerInterfaces(interfacesCtx, nodeName, container.VMID)
			cancelInterfaces()
			if ifaceErr != nil {
				log.Debug().
					Err(ifaceErr).
					Str("instance", instanceName).
					Str("node", nodeName).
					Str("container", container.Name).
					Int("vmid", container.VMID).
					Msg("Container interface metadata unavailable")
			} else if len(ifaceDetails) > 0 {
				for _, detail := range ifaceDetails {
					parsed := containerNetworkDetails{}
					parsed.Name = strings.TrimSpace(detail.Name)
					parsed.MAC = strings.ToUpper(strings.TrimSpace(detail.HWAddr))

					for _, addr := range detail.IPAddresses {
						stripped := strings.TrimSpace(addr.Address)
						if stripped == "" {
							continue
						}
						if slash := strings.Index(stripped, "/"); slash > 0 {
							stripped = stripped[:slash]
						}
						parsed.Addresses = append(parsed.Addresses, sanitizeGuestAddressStrings(stripped)...)
					}

					if len(parsed.Addresses) == 0 && strings.TrimSpace(detail.Inet) != "" {
						parts := strings.Fields(detail.Inet)
						for _, part := range parts {
							stripped := strings.TrimSpace(part)
							if stripped == "" {
								continue
							}
							if slash := strings.Index(stripped, "/"); slash > 0 {
								stripped = stripped[:slash]
							}
							parsed.Addresses = append(parsed.Addresses, sanitizeGuestAddressStrings(stripped)...)
						}
					}

					parsed.Addresses = dedupeStringsPreserveOrder(parsed.Addresses)

					if len(parsed.Addresses) > 0 {
						for _, addr := range parsed.Addresses {
							addAddress(addr)
						}
					}

					if parsed.Name != "" || parsed.MAC != "" || len(parsed.Addresses) > 0 {
						mergeContainerNetworkInterface(&networkIfaces, parsed)
					}
				}
			}
		}
	}

	if len(networkIfaces) > 1 {
		sort.SliceStable(networkIfaces, func(i, j int) bool {
			left := strings.TrimSpace(networkIfaces[i].Name)
			right := strings.TrimSpace(networkIfaces[j].Name)
			return left < right
		})
	}

	if len(addressOrder) > 1 {
		sort.Strings(addressOrder)
	}

	if len(addressOrder) > 0 {
		container.IPAddresses = addressOrder
	}

	if len(networkIfaces) > 0 {
		container.NetworkInterfaces = networkIfaces
	}

	if disks := convertContainerDiskInfo(status, mountMetadata); len(disks) > 0 {
		container.Disks = disks
	}

	ensureContainerRootDiskEntry(container)

	if rootDeviceHint != "" && len(container.Disks) > 0 {
		for i := range container.Disks {
			if container.Disks[i].Mountpoint == "/" && container.Disks[i].Device == "" {
				container.Disks[i].Device = rootDeviceHint
			}
		}
	}
}

// GetConnectionStatuses returns the current connection status for all nodes

func shouldTryPortlessFallback(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "client.timeout exceeded") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded") {
		return true
	}
	return false
}

// retryPVEPortFallback handles the case where a normalized :8006 host is unreachable
// because the actual endpoint is fronted by a reverse proxy on 443. If the initial
// GetNodes call fails with a connection error and the host has the default PVE port,
// retry without the default port to hit the proxy. On success, swap the client so
// subsequent polls reuse the working endpoint.
func (m *Monitor) retryPVEPortFallback(ctx context.Context, instanceName string, instanceCfg *config.PVEInstance, currentClient PVEClientInterface, cause error) ([]proxmox.Node, PVEClientInterface, error) {
	if instanceCfg == nil || !shouldTryPortlessFallback(cause) {
		return nil, currentClient, cause
	}

	fallbackHost := config.StripDefaultPort(instanceCfg.Host, config.DefaultPVEPort)
	if fallbackHost == "" || fallbackHost == instanceCfg.Host {
		return nil, currentClient, cause
	}

	clientCfg := config.CreateProxmoxConfigWithHost(instanceCfg, fallbackHost, false)
	if clientCfg.Timeout <= 0 {
		clientCfg.Timeout = m.config.ConnectionTimeout
	}

	fallbackClient, err := newProxmoxClientFunc(clientCfg)
	if err != nil {
		return nil, currentClient, cause
	}

	fallbackNodes, err := fallbackClient.GetNodes(ctx)
	if err != nil {
		return nil, currentClient, cause
	}

	// Switch to the working host for the remainder of the poll (and future polls)
	primaryHost := instanceCfg.Host

	// Persist with an explicit port to avoid re-normalization back to :8006 on reloads.
	persistHost := fallbackHost
	if parsed, err := url.Parse(fallbackHost); err == nil && parsed.Host != "" && parsed.Port() == "" {
		port := "443"
		if strings.EqualFold(parsed.Scheme, "http") {
			port = "80"
		}
		parsed.Host = net.JoinHostPort(parsed.Hostname(), port)
		persistHost = parsed.Scheme + "://" + parsed.Host
	}

	instanceCfg.Host = persistHost
	m.pveClients[instanceName] = fallbackClient

	// Update in-memory config so subsequent polls build clients against the working port.
	for i := range m.config.PVEInstances {
		if m.config.PVEInstances[i].Name == instanceName {
			m.config.PVEInstances[i].Host = persistHost
			break
		}
	}

	// Persist to disk so restarts keep the working endpoint.
	if m.persistence != nil {
		if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
			log.Warn().Err(err).Str("instance", instanceName).Msg("failed to persist fallback PVE host")
		}
	}

	log.Warn().
		Str("instance", instanceName).
		Str("primary", primaryHost).
		Str("fallback", persistHost).
		Msg("Primary PVE host failed; using fallback without default port")

	return fallbackNodes, fallbackClient, nil
}

func (m *Monitor) fetchPVENodes(ctx context.Context, instanceName string, instanceCfg *config.PVEInstance, client PVEClientInterface) ([]proxmox.Node, PVEClientInterface, error) {
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		if fallbackNodes, fallbackClient, fallbackErr := m.retryPVEPortFallback(ctx, instanceName, instanceCfg, client, err); fallbackErr == nil {
			return fallbackNodes, fallbackClient, nil
		}

		monErr := errors.WrapConnectionError("poll_nodes", instanceName, err)
		log.Error().Err(monErr).Str("instance", instanceName).Msg("failed to get nodes")
		m.state.SetConnectionHealth(instanceName, false)

		// Track auth failure if it's an authentication error
		if errors.IsAuthError(err) {
			m.recordAuthFailure(instanceName, "pve")
		}
		return nil, client, monErr
	}

	// Reset auth failures on successful connection
	m.resetAuthFailures(instanceName, "pve")
	return nodes, client, nil
}

func (m *Monitor) updatePVEConnectionHealth(ctx context.Context, instanceName string, client PVEClientInterface) string {
	connectionHealthStr := "healthy"
	if clusterClient, ok := client.(*proxmox.ClusterClient); ok {
		// For cluster clients, check if all endpoints are healthy
		healthStatus := clusterClient.GetHealthStatus()
		healthyCount := 0
		totalCount := len(healthStatus)

		for _, isHealthy := range healthStatus {
			if isHealthy {
				healthyCount++
			}
		}

		if healthyCount == 0 {
			// All endpoints are down
			connectionHealthStr = "error"
			m.state.SetConnectionHealth(instanceName, false)
		} else if healthyCount < totalCount {
			// Some endpoints are down - check if cluster still has quorum
			// A cluster with quorum is healthy even if some nodes are intentionally offline
			// (e.g., backup nodes not running). Only mark as degraded if no quorum.
			isQuorate, err := clusterClient.IsQuorate(ctx)
			if err != nil {
				// Couldn't check quorum - log but continue (assume healthy if we have connectivity)
				log.Debug().
					Str("instance", instanceName).
					Err(err).
					Msg("Could not check cluster quorum status")
				isQuorate = true // Assume healthy if we can't check
			}

			if isQuorate {
				// Cluster has quorum - healthy even with some nodes offline
				connectionHealthStr = "healthy"
				m.state.SetConnectionHealth(instanceName, true)
				log.Debug().
					Str("instance", instanceName).
					Int("healthy", healthyCount).
					Int("total", totalCount).
					Msg("Cluster has quorum - some API endpoints unreachable but cluster is healthy")
			} else {
				// Cluster lost quorum - this is actually degraded/critical
				connectionHealthStr = "degraded"
				m.state.SetConnectionHealth(instanceName, true) // Still functional but degraded
				log.Warn().
					Str("instance", instanceName).
					Int("healthy", healthyCount).
					Int("total", totalCount).
					Msg("Cluster lost quorum - degraded state")
			}
		} else {
			// All endpoints are healthy
			connectionHealthStr = "healthy"
			m.state.SetConnectionHealth(instanceName, true)
		}
	} else {
		// Regular client - simple healthy/unhealthy
		m.state.SetConnectionHealth(instanceName, true)
	}
	return connectionHealthStr
}

func (m *Monitor) snapshotPrevNodes(instanceName string) (map[string]models.Memory, []models.Node) {
	prevState := m.GetState()
	prevNodeMemory := make(map[string]models.Memory)
	prevInstanceNodes := make([]models.Node, 0)
	for _, existingNode := range prevState.Nodes {
		if existingNode.Instance != instanceName {
			continue
		}
		prevNodeMemory[existingNode.ID] = existingNode.Memory
		prevInstanceNodes = append(prevInstanceNodes, existingNode)
	}
	return prevNodeMemory, prevInstanceNodes
}

func (m *Monitor) pollPVENodesParallel(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	nodes []proxmox.Node,
	connectionHealthStr string,
	prevNodeMemory map[string]models.Memory,
	prevInstanceNodes []models.Node,
	debugEnabled bool,
) ([]models.Node, map[string]string) {
	var modelNodes []models.Node
	nodeEffectiveStatus := make(map[string]string)

	type nodePollResult struct {
		node            models.Node
		effectiveStatus string
	}

	resultChan := make(chan nodePollResult, len(nodes))
	var wg sync.WaitGroup

	if debugEnabled {
		log.Debug().
			Str("instance", instanceName).
			Int("nodes", len(nodes)).
			Msg("Starting parallel node polling")
	}

	for _, node := range nodes {
		wg.Add(1)
		go func(node proxmox.Node) {
			defer wg.Done()

			modelNode, effectiveStatus, _ := m.pollPVENode(ctx, instanceName, instanceCfg, client, node, connectionHealthStr, prevNodeMemory, prevInstanceNodes)

			resultChan <- nodePollResult{
				node:            modelNode,
				effectiveStatus: effectiveStatus,
			}
		}(node)
	}

	wg.Wait()
	close(resultChan)

	for res := range resultChan {
		modelNodes = append(modelNodes, res.node)
		nodeEffectiveStatus[res.node.Name] = res.effectiveStatus
	}

	return modelNodes, nodeEffectiveStatus
}

func (m *Monitor) preserveNodesWhenEmpty(instanceName string, modelNodes []models.Node, prevInstanceNodes []models.Node) []models.Node {
	if len(modelNodes) > 0 || len(prevInstanceNodes) == 0 {
		return modelNodes
	}

	log.Warn().
		Str("instance", instanceName).
		Int("previousCount", len(prevInstanceNodes)).
		Msg("No Proxmox nodes returned this cycle - preserving previous state")

	// Mark connection health as degraded to reflect polling failure
	m.state.SetConnectionHealth(instanceName, false)

	preserved := make([]models.Node, 0, len(prevInstanceNodes))
	now := time.Now()
	for _, prevNode := range prevInstanceNodes {
		nodeCopy := prevNode

		// Keep recently seen nodes online during transient GetNodes gaps.
		// This mirrors the node grace behavior used in regular node polling.
		lastSeen := prevNode.LastSeen
		if lastSeen.IsZero() {
			m.mu.Lock()
			if lastOnline, ok := m.nodeLastOnline[prevNode.ID]; ok {
				lastSeen = lastOnline
			}
			m.mu.Unlock()
		}

		withinGrace := !lastSeen.IsZero() && now.Sub(lastSeen) < nodeOfflineGracePeriod
		if withinGrace {
			if strings.TrimSpace(nodeCopy.Status) == "" || strings.EqualFold(nodeCopy.Status, "offline") {
				nodeCopy.Status = "online"
			}
			if nodeCopy.ConnectionHealth == "" || strings.EqualFold(nodeCopy.ConnectionHealth, "error") {
				nodeCopy.ConnectionHealth = "degraded"
			}
			preserved = append(preserved, nodeCopy)
			continue
		}

		nodeCopy.Status = "offline"
		nodeCopy.ConnectionHealth = "error"
		nodeCopy.Uptime = 0
		nodeCopy.CPU = 0
		preserved = append(preserved, nodeCopy)
	}
	return preserved
}

func (m *Monitor) seedNodeDisplayNames(modelNodes []models.Node) {
	for i := range modelNodes {
		if modelNodes[i].DisplayName != "" {
			m.alertManager.UpdateNodeDisplayName(modelNodes[i].Name, modelNodes[i].DisplayName)
		}
	}
}

func (m *Monitor) pollGuestsWithFallback(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	nodes []proxmox.Node,
	nodeEffectiveStatus map[string]string,
) error {
	if !instanceCfg.MonitorVMs && !instanceCfg.MonitorContainers {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Always try the efficient cluster/resources endpoint first
	// This endpoint works on both clustered and standalone nodes
	// Testing confirmed it works on standalone nodes like pimox
	useClusterEndpoint := m.pollVMsAndContainersEfficient(ctx, instanceName, instanceCfg.ClusterName, instanceCfg.IsCluster, client, nodeEffectiveStatus)

	if !useClusterEndpoint {
		// Fall back to traditional polling only if cluster/resources not available
		// This should be rare - only for very old Proxmox versions
		log.Debug().
			Str("instance", instanceName).
			Msg("cluster/resources endpoint not available, using traditional polling")

		// Check if configuration needs updating
		if instanceCfg.IsCluster {
			isActuallyCluster, checkErr := client.IsClusterMember(ctx)
			if checkErr == nil && !isActuallyCluster {
				log.Warn().
					Str("instance", instanceName).
					Msg("Instance marked as cluster but is actually standalone - consider updating configuration")
				instanceCfg.IsCluster = false
			}
		}

		// Use optimized parallel polling for better performance
		if instanceCfg.MonitorVMs {
			m.pollVMsWithNodes(ctx, instanceName, instanceCfg.ClusterName, instanceCfg.IsCluster, client, nodes, nodeEffectiveStatus)
		}
		if instanceCfg.MonitorContainers {
			m.pollContainersWithNodes(ctx, instanceName, instanceCfg.ClusterName, instanceCfg.IsCluster, client, nodes, nodeEffectiveStatus)
		}
	}

	return nil
}

func (m *Monitor) maybePollPhysicalDisksAsync(
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	nodes []proxmox.Node,
	nodeEffectiveStatus map[string]string,
	modelNodes []models.Node,
) {
	// Poll physical disks for health monitoring (enabled by default unless explicitly disabled)
	// Skip if MonitorPhysicalDisks is explicitly set to false
	// Physical disk polling runs in a background goroutine since GetDisks can be slow
	// and we don't want it to cause task timeouts. It has its own 5-minute interval anyway.
	if instanceCfg.MonitorPhysicalDisks != nil && !*instanceCfg.MonitorPhysicalDisks {
		log.Debug().Str("instance", instanceName).Msg("physical disk monitoring explicitly disabled")
		// Keep any existing disk data visible (don't clear it)
		return
	}

	// Enabled by default (when nil or true)
	// Determine polling interval (default 5 minutes to avoid spinning up HDDs too frequently)
	pollingInterval := 5 * time.Minute
	if instanceCfg.PhysicalDiskPollingMinutes > 0 {
		pollingInterval = time.Duration(instanceCfg.PhysicalDiskPollingMinutes) * time.Minute
	}

	// Check if enough time has elapsed since last poll
	m.mu.Lock()
	lastPoll, exists := m.lastPhysicalDiskPoll[instanceName]
	shouldPoll := !exists || time.Since(lastPoll) >= pollingInterval
	if shouldPoll {
		m.lastPhysicalDiskPoll[instanceName] = time.Now()
	}
	m.mu.Unlock()

	if !shouldPoll {
		log.Debug().
			Str("instance", instanceName).
			Dur("sinceLastPoll", time.Since(lastPoll)).
			Dur("interval", pollingInterval).
			Msg("Skipping physical disk poll - interval not elapsed")
		// Refresh NVMe temperatures using the latest sensor data even when we skip the disk poll
		currentState := m.state.GetSnapshot()
		existing := make([]models.PhysicalDisk, 0)
		for _, disk := range currentState.PhysicalDisks {
			if disk.Instance == instanceName {
				existing = append(existing, disk)
			}
		}
		if len(existing) > 0 {
			// Use nodes from state snapshot - they have LinkedHostAgentID populated
			// (the local modelNodes variable doesn't have this field set)
			updated := mergeNVMeTempsIntoDisks(existing, currentState.Nodes)
			// Also merge SMART data from linked host agents
			updated = mergeHostAgentSMARTIntoDisks(updated, currentState.Nodes, currentState.Hosts)
			m.state.UpdatePhysicalDisks(instanceName, updated)
		}
		return
	}

	// Run physical disk polling in background to avoid blocking the main task
	go func(inst string, pveClient PVEClientInterface, nodeList []proxmox.Node, nodeStatus map[string]string, modelNodesCopy []models.Node) {
		defer recoverFromPanic(fmt.Sprintf("pollPhysicalDisks-%s", inst))

		// Use a generous timeout for disk polling
		diskTimeout := 60 * time.Second
		diskCtx, diskCancel := context.WithTimeout(context.Background(), diskTimeout)
		defer diskCancel()

		log.Debug().
			Int("nodeCount", len(nodeList)).
			Dur("interval", pollingInterval).
			Msg("Starting disk health polling")

		// Get existing disks from state to preserve data for offline nodes
		currentState := m.state.GetSnapshot()
		existingDisksMap := make(map[string]models.PhysicalDisk)
		for _, disk := range currentState.PhysicalDisks {
			if disk.Instance == inst {
				existingDisksMap[disk.ID] = disk
			}
		}

		var allDisks []models.PhysicalDisk
		polledNodes := make(map[string]bool) // Track which nodes we successfully polled

		for _, node := range nodeList {
			// Check if context timed out
			select {
			case <-diskCtx.Done():
				log.Debug().
					Str("instance", inst).
					Msg("Physical disk polling timed out - preserving existing data")
				return
			default:
			}

			// Skip offline nodes but preserve their existing disk data
			if nodeStatus[node.Node] != "online" {
				log.Debug().Str("node", node.Node).Msg("skipping disk poll for offline node - preserving existing data")
				continue
			}

			// Get disk list for this node
			log.Debug().Str("node", node.Node).Msg("getting disk list for node")
			disks, err := pveClient.GetDisks(diskCtx, node.Node)
			if err != nil {
				// Check if it's a permission error or if the endpoint doesn't exist
				errStr := err.Error()
				if strings.Contains(errStr, "401") || strings.Contains(errStr, "403") {
					log.Warn().
						Str("node", node.Node).
						Err(err).
						Msg("Insufficient permissions to access disk information - check API token permissions")
				} else if strings.Contains(errStr, "404") || strings.Contains(errStr, "501") {
					log.Info().
						Str("node", node.Node).
						Msg("Disk monitoring not available on this node (may be using non-standard storage)")
				} else {
					log.Warn().
						Str("node", node.Node).
						Err(err).
						Msg("Failed to get disk list")
				}
				continue
			}

			// Mark this node as successfully polled
			polledNodes[node.Node] = true

			// Check each disk for health issues and add to state
			for _, disk := range disks {
				// Create PhysicalDisk model
				diskID := fmt.Sprintf("%s-%s-%s", inst, node.Node, strings.ReplaceAll(disk.DevPath, "/", "-"))
				physicalDisk := models.PhysicalDisk{
					ID:          diskID,
					Node:        node.Node,
					Instance:    inst,
					DevPath:     disk.DevPath,
					Model:       disk.Model,
					Serial:      disk.Serial,
					WWN:         disk.WWN,
					Type:        disk.Type,
					Size:        disk.Size,
					Health:      disk.Health,
					Wearout:     disk.Wearout,
					RPM:         disk.RPM,
					Used:        disk.Used,
					LastChecked: time.Now(),
				}

				allDisks = append(allDisks, physicalDisk)

				log.Debug().
					Str("node", node.Node).
					Str("disk", disk.DevPath).
					Str("model", disk.Model).
					Str("health", disk.Health).
					Int("wearout", disk.Wearout).
					Msg("Checking disk health")

				normalizedHealth := strings.ToUpper(strings.TrimSpace(disk.Health))
				if normalizedHealth != "" && normalizedHealth != "UNKNOWN" && normalizedHealth != "PASSED" && normalizedHealth != "OK" {
					// Disk has failed or is failing - alert manager will handle this
					log.Warn().
						Str("node", node.Node).
						Str("disk", disk.DevPath).
						Str("model", disk.Model).
						Str("health", disk.Health).
						Int("wearout", disk.Wearout).
						Msg("Disk health issue detected")

					// Pass disk info to alert manager
					m.alertManager.CheckDiskHealth(inst, node.Node, disk)
				} else if disk.Wearout > 0 && disk.Wearout < 10 {
					// Low wearout warning (less than 10% life remaining)
					log.Warn().
						Str("node", node.Node).
						Str("disk", disk.DevPath).
						Str("model", disk.Model).
						Int("wearout", disk.Wearout).
						Msg("SSD wearout critical - less than 10% life remaining")

					// Pass to alert manager for wearout alert
					m.alertManager.CheckDiskHealth(inst, node.Node, disk)
				}
			}
		}

		// Preserve existing disk data for nodes that weren't polled (offline or error)
		for _, existingDisk := range existingDisksMap {
			// Only preserve if we didn't poll this node
			if !polledNodes[existingDisk.Node] {
				// Keep the existing disk data but update the LastChecked to indicate it's stale
				allDisks = append(allDisks, existingDisk)
				log.Debug().
					Str("node", existingDisk.Node).
					Str("disk", existingDisk.DevPath).
					Msg("Preserving existing disk data for unpolled node")
			}
		}

		// Use nodes from state snapshot - they have LinkedHostAgentID populated
		// (modelNodesCopy passed to this goroutine doesn't have this field set)
		allDisks = mergeNVMeTempsIntoDisks(allDisks, currentState.Nodes)
		// Also merge SMART data from linked host agents
		allDisks = mergeHostAgentSMARTIntoDisks(allDisks, currentState.Nodes, currentState.Hosts)

		// Write SMART metrics to persistent store
		if m.metricsStore != nil {
			now := time.Now()
			for _, disk := range allDisks {
				m.writeSMARTMetrics(disk, now)
			}
		}

		// Update physical disks in state
		log.Debug().
			Str("instance", inst).
			Int("diskCount", len(allDisks)).
			Int("preservedCount", len(existingDisksMap)-len(polledNodes)).
			Msg("Updating physical disks in state")
		m.state.UpdatePhysicalDisks(inst, allDisks)
	}(instanceName, client, nodes, nodeEffectiveStatus, modelNodes)
}

// pollPVEInstance polls a single PVE instance
func (m *Monitor) pollPVEInstance(ctx context.Context, instanceName string, client PVEClientInterface) {
	defer recoverFromPanic(fmt.Sprintf("pollPVEInstance-%s", instanceName))

	start := time.Now()
	debugEnabled := logging.IsLevelEnabled(zerolog.DebugLevel)
	var pollErr error
	if m.pollMetrics != nil {
		m.pollMetrics.IncInFlight("pve")
		defer m.pollMetrics.DecInFlight("pve")
		defer func() {
			m.pollMetrics.RecordResult(PollResult{
				InstanceName: instanceName,
				InstanceType: "pve",
				Success:      pollErr == nil,
				Error:        pollErr,
				StartTime:    start,
				EndTime:      time.Now(),
			})
		}()
	}
	if m.stalenessTracker != nil {
		defer func() {
			if pollErr == nil {
				m.stalenessTracker.UpdateSuccess(InstanceTypePVE, instanceName, nil)
			} else {
				m.stalenessTracker.UpdateError(InstanceTypePVE, instanceName)
			}
		}()
	}
	defer m.recordTaskResult(InstanceTypePVE, instanceName, pollErr)

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		pollErr = ctx.Err()
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("polling cancelled")
		}
		return
	default:
	}

	if debugEnabled {
		log.Debug().Str("instance", instanceName).Msg("polling PVE instance")
	}

	// Get instance config
	instanceCfg := m.getInstanceConfig(instanceName)
	if instanceCfg == nil {
		pollErr = fmt.Errorf("pve instance config not found for %s", instanceName)
		return
	}

	// Poll nodes
	nodes, updatedClient, err := m.fetchPVENodes(ctx, instanceName, instanceCfg, client)
	if err != nil {
		pollErr = err
		return
	}
	client = updatedClient

	// Check if client is a ClusterClient to determine health status
	connectionHealthStr := m.updatePVEConnectionHealth(ctx, instanceName, client)

	// Capture previous memory metrics so we can preserve them if detailed status fails
	prevNodeMemory, prevInstanceNodes := m.snapshotPrevNodes(instanceName)

	// Convert to models
	modelNodes, nodeEffectiveStatus := m.pollPVENodesParallel(
		ctx,
		instanceName,
		instanceCfg,
		client,
		nodes,
		connectionHealthStr,
		prevNodeMemory,
		prevInstanceNodes,
		debugEnabled,
	)

	modelNodes = m.preserveNodesWhenEmpty(instanceName, modelNodes, prevInstanceNodes)

	// Update state first so we have nodes available
	m.state.UpdateNodesForInstance(instanceName, modelNodes)

	// Storage fallback is used to provide disk metrics when rootfs is not available.
	// We run this asynchronously with a short timeout so it doesn't block VM/container polling.
	// This addresses the issue where slow storage APIs (e.g., NFS mounts) can cause the entire
	// polling task to timeout before reaching VM/container polling.
	storageFallback := m.startStorageFallback(instanceName, instanceCfg, client, nodes, nodeEffectiveStatus)

	// Pre-populate node display name cache so guest alerts created below
	// can resolve friendly names. CheckNode() also does this, but it runs
	// after guest polling — without this, the first alert notification for
	// a guest would show the raw Proxmox node name.
	m.seedNodeDisplayNames(modelNodes)

	// Poll VMs and containers FIRST - this is the most critical data.
	// This happens immediately after starting the storage fallback goroutine,
	// so VM/container polling runs in parallel with (and is not blocked by) storage operations.
	if err := m.pollGuestsWithFallback(ctx, instanceName, instanceCfg, client, nodes, nodeEffectiveStatus); err != nil {
		pollErr = err
		return
	}

	m.maybePollPhysicalDisksAsync(instanceName, instanceCfg, client, nodes, nodeEffectiveStatus, modelNodes)
	// Note: Physical disk monitoring is now enabled by default with a 5-minute polling interval.
	// Users can explicitly disable it in node settings. Disk data is preserved between polls.

	// Wait for storage fallback to complete (with a short timeout) before using the data.
	// This is non-blocking in the sense that VM/container polling has already completed by now.
	// We give the storage fallback goroutine up to 2 additional seconds to finish if it's still running.
	localStorageByNode := m.awaitStorageFallback(instanceName, storageFallback, 2*time.Second)

	modelNodes = m.applyStorageFallbackAndRecordNodeMetrics(instanceName, client, modelNodes, localStorageByNode)

	// Periodically re-check cluster status for nodes marked as standalone
	// This addresses issue #437 where clusters aren't detected on first attempt
	m.detectClusterMembership(ctx, instanceName, instanceCfg, client)

	// Update cluster endpoint online status if this is a cluster
	m.updateClusterEndpointStatus(instanceName, instanceCfg, client, modelNodes)

	if err := m.pollStorageAsync(ctx, instanceName, instanceCfg, client, nodes); err != nil {
		pollErr = err
		return
	}

	if err := m.pollPVEBackupsAsync(ctx, instanceName, instanceCfg, client, nodes, nodeEffectiveStatus); err != nil {
		pollErr = err
		return
	}
}

func copyFloatPointer(src *float64) *float64 {
	if src == nil {
		return nil
	}
	val := *src
	return &val
}

// matchesDatastoreExclude checks if a datastore name matches any exclusion pattern.
// Patterns can be exact names or wildcards (* for any characters).
// Examples: "exthdd*" matches "exthdd1500gb", "*backup*" matches "my-backup-store"
