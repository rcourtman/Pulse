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
	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/fsfilters"
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

func (m *Monitor) fetchContainerStatusSnapshot(
	ctx context.Context,
	client PVEClientInterface,
	instanceName, nodeName, containerName string,
	vmid int,
) *proxmox.Container {
	if client == nil {
		return nil
	}

	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	statusResp, err := client.GetContainerStatus(statusCtx, nodeName, vmid)
	cancel()
	if err != nil {
		log.Debug().
			Err(err).
			Str("instance", instanceName).
			Str("node", nodeName).
			Str("container", containerName).
			Int("vmid", vmid).
			Msg("Container status metadata unavailable")
		return nil
	}

	return statusResp
}

func mergeContainerRuntimeCounters(current IOMetrics, status *proxmox.Container) IOMetrics {
	if status == nil {
		return current
	}

	currentPresence := current.Presence.Effective()
	statusPresence := status.IOCounters.Effective()
	if statusPresence.DiskRead {
		current.DiskRead = int64(status.DiskRead)
		currentPresence.DiskRead = true
		current.ObservedAt.DiskRead = status.ObservedAt
	}
	if statusPresence.DiskWrite {
		current.DiskWrite = int64(status.DiskWrite)
		currentPresence.DiskWrite = true
		current.ObservedAt.DiskWrite = status.ObservedAt
	}
	if statusPresence.NetworkIn {
		current.NetworkIn = int64(status.NetIn)
		currentPresence.NetworkIn = true
		current.ObservedAt.NetworkIn = status.ObservedAt
	}
	if statusPresence.NetworkOut {
		current.NetworkOut = int64(status.NetOut)
		currentPresence.NetworkOut = true
		current.ObservedAt.NetworkOut = status.ObservedAt
	}
	current.Presence = currentPresence
	if !status.ObservedAt.IsZero() {
		current.Timestamp = status.ObservedAt
	}
	if status.Uptime > 0 {
		current.SourceUptime = status.Uptime
	}
	return current
}

func (m *Monitor) enrichContainerMetadata(ctx context.Context, client PVEClientInterface, instanceName, nodeName string, container *models.Container, prefetchedStatus ...*proxmox.Container) {
	if container == nil {
		return
	}

	ensureContainerRootDiskEntry(container)

	if client == nil {
		return
	}

	isRunning := container.Status == "running"

	var status *proxmox.Container
	if len(prefetchedStatus) > 0 {
		status = prefetchedStatus[0]
	}
	if isRunning && status == nil {
		status = m.fetchContainerStatusSnapshot(
			ctx,
			client,
			instanceName,
			nodeName,
			container.Name,
			container.VMID,
		)
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
		// Extract onboot (autostart) setting so the alert engine can
		// suppress powered-off alerts for guests not configured to autostart.
		if onBoot := parseProxmoxOnBoot(configData); onBoot != nil {
			container.OnBoot = onBoot
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
		m.mu.RLock()
		if m.config != nil {
			clientCfg.Timeout = m.config.ConnectionTimeout
		}
		m.mu.RUnlock()
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

	var persistence *config.ConfigPersistence
	var pveInstances []config.PVEInstance
	var pbsInstances []config.PBSInstance
	var pmgInstances []config.PMGInstance

	m.mu.Lock()
	instanceCfg.Host = persistHost
	m.pveClients[instanceName] = fallbackClient

	// Update in-memory config so subsequent polls build clients against the working port.
	if m.config != nil {
		for i := range m.config.PVEInstances {
			if m.config.PVEInstances[i].Name == instanceName {
				m.config.PVEInstances[i].Host = persistHost
				break
			}
		}

		pveInstances = append(pveInstances, m.config.PVEInstances...)
		pbsInstances = append(pbsInstances, m.config.PBSInstances...)
		pmgInstances = append(pmgInstances, m.config.PMGInstances...)
	}
	persistence = m.persistence
	m.mu.Unlock()

	// Persist to disk so restarts keep the working endpoint.
	if persistence != nil {
		if err := persistence.SaveNodesConfig(pveInstances, pbsInstances, pmgInstances); err != nil {
			log.Warn().Err(err).Str("instance", instanceName).Msg("Failed to persist fallback PVE host")
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
		m.setProviderConnectionHealth(InstanceTypePVE, instanceName, false)

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

func (m *Monitor) refreshPVETagColors(ctx context.Context, instanceName string, client PVEClientInterface) {
	type clusterOptionsGetter interface {
		GetClusterOptions(ctx context.Context) (*proxmox.ClusterOptions, error)
	}

	getter, ok := client.(clusterOptionsGetter)
	if !ok || getter == nil {
		return
	}

	opts, err := getter.GetClusterOptions(ctx)
	if err != nil || opts == nil {
		return
	}
	tagStyle := proxmox.ParseTagStyle(opts.TagStyle)
	m.state.MergePVETagStyle(instanceName, models.PVETagStyle{
		Colors:        tagStyle.Colors,
		CaseSensitive: tagStyle.CaseSensitive,
	})
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
			m.setProviderConnectionHealth(InstanceTypePVE, instanceName, false)
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
				m.setProviderConnectionHealth(InstanceTypePVE, instanceName, true)
				log.Debug().
					Str("instance", instanceName).
					Int("healthy", healthyCount).
					Int("total", totalCount).
					Msg("Cluster has quorum - some API endpoints unreachable but cluster is healthy")
			} else {
				// Cluster lost quorum - this is actually degraded/critical
				connectionHealthStr = "degraded"
				m.setProviderConnectionHealth(InstanceTypePVE, instanceName, true) // Still functional but degraded
				log.Warn().
					Str("instance", instanceName).
					Int("healthy", healthyCount).
					Int("total", totalCount).
					Msg("Cluster lost quorum - degraded state")
			}
		} else {
			// All endpoints are healthy
			connectionHealthStr = "healthy"
			m.setProviderConnectionHealth(InstanceTypePVE, instanceName, true)
		}
	} else {
		// Regular client - simple healthy/unhealthy
		m.setProviderConnectionHealth(InstanceTypePVE, instanceName, true)
	}
	return connectionHealthStr
}

func (m *Monitor) snapshotPrevNodes(instanceName string) (map[string]models.Memory, []models.Node) {
	return m.previousNodesForInstance(instanceName)
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
) ([]models.Node, map[string]string, map[string]string) {
	var modelNodes []models.Node
	nodeEffectiveStatus := make(map[string]string)
	nodeDiskSources := make(map[string]string)

	type nodePollResult struct {
		node            models.Node
		effectiveStatus string
		diskSource      string
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

			modelNode, effectiveStatus, diskSource, _ := m.pollPVENode(ctx, instanceName, instanceCfg, client, node, connectionHealthStr, prevNodeMemory, prevInstanceNodes)

			resultChan <- nodePollResult{
				node:            modelNode,
				effectiveStatus: effectiveStatus,
				diskSource:      diskSource,
			}
		}(node)
	}

	wg.Wait()
	close(resultChan)

	for res := range resultChan {
		modelNodes = append(modelNodes, res.node)
		nodeEffectiveStatus[res.node.Name] = res.effectiveStatus
		nodeDiskSources[res.node.Name] = res.diskSource
	}

	return modelNodes, nodeEffectiveStatus, nodeDiskSources
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
	m.setProviderConnectionHealth(InstanceTypePVE, instanceName, false)

	return m.preserveOrExpireNodes(prevInstanceNodes)
}

// markPVEInstanceNodesUnreachable applies the node offline grace policy to an
// instance whose poll failed outright (connection refused, timeout, all
// cluster endpoints down). Without this, the early return on a poll error
// leaves the last successful snapshot in state forever, so a shut-down host
// keeps showing its final online status (#1441).
func (m *Monitor) markPVEInstanceNodesUnreachable(instanceName string) {
	_, prevInstanceNodes := m.snapshotPrevNodes(instanceName)
	if len(prevInstanceNodes) == 0 {
		// Never polled successfully this process lifetime (e.g. Pulse
		// started while the host was already down). Synthesize offline
		// entries from the instance config so the configured instance is
		// still represented on the platform pages instead of vanishing
		// until its first successful poll (#1433).
		if placeholders := m.placeholderNodesForInstance(instanceName); len(placeholders) > 0 {
			m.state.UpdateNodesForInstance(instanceName, placeholders)
		}
		return
	}
	m.state.UpdateNodesForInstance(instanceName, m.preserveOrExpireNodes(prevInstanceNodes))
}

// placeholderNodesForInstance builds offline node entries from the instance
// configuration for an instance with no node state yet. Cluster configs carry
// their discovered member endpoints, so each member gets its own row with the
// same node ID a live poll would assign; a standalone instance gets a single
// row named after the configured instance. The next successful poll replaces
// these wholesale via UpdateNodesForInstance.
func (m *Monitor) placeholderNodesForInstance(instanceName string) []models.Node {
	instanceCfg := m.getInstanceConfig(instanceName)
	if instanceCfg == nil {
		return nil
	}

	makeNode := func(nodeName string) models.Node {
		nodeID := instanceName + "-" + nodeName
		if instanceCfg.IsCluster && instanceCfg.ClusterName != "" {
			nodeID = instanceCfg.ClusterName + "-" + nodeName
		}
		connectionHost, guestURL := resolveNodeConnectionInfo(instanceCfg, monitorDiscoveryConfig(m), nodeName)
		return models.Node{
			ID:                           nodeID,
			Name:                         nodeName,
			DisplayName:                  getNodeDisplayName(instanceCfg, nodeName),
			Instance:                     instanceName,
			Host:                         connectionHost,
			GuestURL:                     guestURL,
			Status:                       "offline",
			Type:                         "node",
			ConnectionHealth:             "error",
			LoadAverage:                  []float64{},
			IsClusterMember:              instanceCfg.IsCluster,
			ClusterName:                  instanceCfg.ClusterName,
			TemperatureMonitoringEnabled: instanceCfg.TemperatureMonitoringEnabled,
		}
	}

	if instanceCfg.IsCluster && len(instanceCfg.ClusterEndpoints) > 0 {
		nodes := make([]models.Node, 0, len(instanceCfg.ClusterEndpoints))
		seen := make(map[string]struct{}, len(instanceCfg.ClusterEndpoints))
		for _, endpoint := range instanceCfg.ClusterEndpoints {
			nodeName := strings.TrimSpace(endpoint.NodeName)
			if nodeName == "" {
				continue
			}
			if _, dup := seen[strings.ToLower(nodeName)]; dup {
				continue
			}
			seen[strings.ToLower(nodeName)] = struct{}{}
			nodes = append(nodes, makeNode(nodeName))
		}
		if len(nodes) > 0 {
			return nodes
		}
	}

	nodeName := strings.TrimSpace(instanceCfg.Name)
	if nodeName == "" {
		nodeName = instanceName
	}
	return []models.Node{makeNode(nodeName)}
}

// preserveOrExpireNodes keeps recently seen nodes online through transient
// gaps and marks nodes offline once the grace period has lapsed.
func (m *Monitor) preserveOrExpireNodes(prevInstanceNodes []models.Node) []models.Node {
	preserved := make([]models.Node, 0, len(prevInstanceNodes))
	now := time.Now()
	gracePeriod := m.pveNodeOfflineGracePeriod()
	for _, prevNode := range prevInstanceNodes {
		nodeCopy := prevNode

		// Keep recently seen nodes online during transient GetNodes gaps.
		// This mirrors the node grace behavior used in regular node polling.
		// Grace requires evidence of a real online sighting: either this
		// process saw the node online (nodeLastOnline), or the previous
		// snapshot itself reports it online with a recent poll timestamp.
		// prevNode.LastSeen alone is NOT enough for an offline node: a
		// synthesized offline placeholder has never been sighted, and even
		// though the unified registry now preserves its zero LastSeen
		// instead of stamping ingest time, requiring an online sighting
		// keeps this policy correct independent of registry stamping.
		m.mu.Lock()
		lastOnline, sawOnline := m.nodeLastOnline[prevNode.ID]
		m.mu.Unlock()

		withinGrace := sawOnline && now.Sub(lastOnline) < gracePeriod
		if !withinGrace && strings.EqualFold(strings.TrimSpace(prevNode.Status), "online") {
			withinGrace = !prevNode.LastSeen.IsZero() && now.Sub(prevNode.LastSeen) < gracePeriod
		}
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
		previous := m.previousGuestContextForInstance(instanceName)
		vms := previous.vms
		containers := previous.containers
		if instanceCfg.MonitorVMs {
			vms = m.collectVMsWithNodes(ctx, instanceName, instanceCfg.ClusterName, instanceCfg.IsCluster, client, nodes, nodeEffectiveStatus)
		}
		if instanceCfg.MonitorContainers {
			containers = m.collectContainersWithNodes(ctx, instanceName, instanceCfg.ClusterName, instanceCfg.IsCluster, client, nodes, nodeEffectiveStatus)
		}
		m.state.UpdateGuestsForInstance(instanceName, vms, containers)
	}

	return nil
}

func (m *Monitor) maybePollPhysicalDisksAsync(
	ctx context.Context,
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
		readState := m.GetUnifiedReadStateOrSnapshot()
		existing := physicalDisksForInstanceFromReadState(readState, instanceName)
		if len(existing) > 0 {
			nodes := nodesForInstanceFromReadState(readState, instanceName)
			hosts := hostsFromReadState(readState)
			updated := mergeNVMeTempsIntoDisks(existing, nodes)
			updated = mergeHostAgentSMARTIntoDisks(updated, nodes, hosts)
			m.state.UpdatePhysicalDisks(instanceName, updated)
		}
		return
	}

	// Run physical disk polling in background to avoid blocking the main task
	go func(inst string, pveClient PVEClientInterface, nodeList []proxmox.Node, nodeStatus map[string]string, modelNodesCopy []models.Node) {
		defer recoverFromPanic(fmt.Sprintf("pollPhysicalDisks-%s", inst))

		// Use a generous timeout for disk polling. Each node gets its own
		// attempt window below, so the overall budget scales to the polling
		// interval: one slow node must not consume the whole instance budget,
		// and staying inside the interval means polls can never overlap.
		diskTimeout := 60 * time.Second
		if pollingInterval > diskTimeout {
			diskTimeout = pollingInterval
		}
		// Use monitor lifecycle context so shutdown can interrupt detached async polling.
		parentCtx := m.getRuntimeContext()
		if parentCtx == nil {
			parentCtx = ctx
		}
		if parentCtx == nil {
			parentCtx = context.Background()
		}
		diskCtx, diskCancel := context.WithTimeout(parentCtx, diskTimeout)
		defer diskCancel()

		log.Debug().
			Int("nodeCount", len(nodeList)).
			Dur("interval", pollingInterval).
			Msg("Starting disk health polling")

		// Get existing disks from state to preserve data for offline nodes
		readState := m.GetUnifiedReadStateOrSnapshot()
		nodesFromState := nodesForInstanceFromReadState(readState, inst)
		hosts := hostsFromReadState(readState)
		existingDisks := physicalDisksForInstanceFromReadState(readState, inst)
		existingDisksMap := make(map[string]models.PhysicalDisk, len(existingDisks))
		for _, disk := range existingDisks {
			if disk.Instance == inst {
				existingDisksMap[disk.ID] = disk
			}
		}

		// Build a lookup map of node name → disk exclusion patterns by
		// cross-referencing linked host agents. This lets --disk-exclude on the
		// agent suppress server-side Proxmox disk health/wearout alerts.
		diskExcludeByNode := make(map[string][]string)
		hostByID := make(map[string]models.Host, len(hosts))
		for _, h := range hosts {
			hostByID[h.ID] = h
		}
		// Also map node name → linked host agent SMART inventory so a node whose
		// Proxmox disk query fails can still populate its physical disks from
		// the agent's smartctl view (#1516).
		smartByNode := make(map[string][]models.HostDiskSMART)
		for _, n := range nodesFromState {
			if n.LinkedAgentID == "" || n.Instance != inst {
				continue
			}
			if linkedHost, ok := hostByID[n.LinkedAgentID]; ok {
				if len(linkedHost.DiskExclude) > 0 {
					diskExcludeByNode[n.Name] = linkedHost.DiskExclude
				}
				if len(linkedHost.Sensors.SMART) > 0 {
					smartByNode[n.Name] = linkedHost.Sensors.SMART
				}
			}
		}

		var allDisks []models.PhysicalDisk
		polledNodes := make(map[string]bool) // Track which nodes we successfully polled
		zfsPoolingEnabled := zfsMonitoringEnabledFromEnv()

	nodeLoop:
		for _, node := range nodeList {
			// Check if context timed out. Break instead of returning so the
			// nodes already polled still land in state.
			select {
			case <-diskCtx.Done():
				log.Debug().
					Str("instance", inst).
					Msg("Physical disk polling timed out - saving partial results")
				break nodeLoop
			default:
			}

			// Skip offline nodes but preserve their existing disk data
			if nodeStatus[node.Node] != "online" {
				log.Debug().Str("node", node.Node).Msg("skipping disk poll for offline node - preserving existing data")
				continue
			}

			// Each node gets its own attempt window so one slow node cannot
			// starve the rest of the cluster's disk poll.
			nodeCtx, nodeCancel := context.WithTimeout(diskCtx, 60*time.Second)

			// Get disk list for this node
			log.Debug().Str("node", node.Node).Msg("getting disk list for node")
			disks, err := pveClient.GetDisks(nodeCtx, node.Node)
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
				// The Proxmox query failed, but a linked host agent sees the
				// same disks via smartctl. Wide nodes (dozens of disks) can
				// exceed the API window PVE itself allows for disks/list, so
				// without this fallback their Physical Disks view empties
				// permanently (#1516).
				if fallback := physicalDisksFromHostAgentSMART(inst, node.Node, smartByNode[node.Node]); len(fallback) > 0 {
					polledNodes[node.Node] = true
					allDisks = append(allDisks, fallback...)
					log.Info().
						Str("node", node.Node).
						Int("diskCount", len(fallback)).
						Msg("Proxmox disk query failed - using linked host agent SMART inventory for physical disks")
				}
				nodeCancel()
				continue
			}

			// Mark this node as successfully polled
			polledNodes[node.Node] = true

			// Build a disk→pool assignment for this node so each physical disk
			// knows which ZFS pool (if any) it belongs to. Errors are
			// non-fatal; we simply leave StorageGroup empty.
			var poolAssignment *diskPoolAssignment
			poolStatus := diskinventory.Unavailable("proxmox_zfs", "ZFS pool monitoring is disabled")
			if zfsPoolingEnabled {
				if pools, pErr := pveClient.GetZFSPoolsWithDetails(nodeCtx, node.Node); pErr == nil {
					poolAssignment = buildDiskPoolAssignment(pools)
					poolStatus = diskinventory.Available("proxmox_zfs")
				} else {
					poolStatus = diskinventory.Unavailable("proxmox_zfs", "ZFS pool membership query failed")
					log.Debug().
						Err(pErr).
						Str("node", node.Node).
						Msg("Could not fetch ZFS pool details for disk→pool mapping; StorageGroup will be empty")
				}
			}
			nodeCancel()

			// Record each disk; alert evaluation happens after host-agent SMART merges
			// so the canonical disk view includes post-merge health/wearout data.
			for _, disk := range disks {
				diskID := unifiedresources.ProxmoxPhysicalDiskSourceID(
					inst,
					node.Node,
					disk.DevPath,
					"",
					"",
				)
				physicalDisk := models.PhysicalDisk{
					ID:       diskID,
					Node:     node.Node,
					Instance: inst,
					DevPath:  disk.DevPath,
					Model:    disk.Model,
					Vendor:   disk.Vendor,
					Serial:   disk.Serial,
					WWN:      disk.WWN,
					Type:     disk.Type,
					Size:     disk.Size,
					Health:   normalizeProxmoxDiskHealth(disk.Health),
					Wearout:  disk.Wearout,
					RPM:      disk.RPM,
					Used:     disk.Used,
					Collection: &diskinventory.CollectionStatus{
						Temperature: diskinventory.Unsupported("proxmox_disks", "Proxmox disk inventory does not expose temperature"),
						IO:          diskinventory.Unsupported("proxmox_disks", "Proxmox disk inventory does not expose physical-disk I/O counters"),
						Controller:  diskinventory.Missing("proxmox_disks", "controller association was not reported"),
						Pool:        poolStatus,
					},
					LastChecked: time.Now(),
				}
				if strings.TrimSpace(disk.Serial) != "" {
					physicalDisk.Collection.Serial = diskinventory.Available("proxmox_disks")
				} else {
					physicalDisk.Collection.Serial = diskinventory.Missing("proxmox_disks", "disk serial was not reported")
				}
				if poolAssignment != nil {
					physicalDisk.StorageGroup = poolAssignment.lookup(physicalDisk)
				}

				allDisks = append(allDisks, physicalDisk)
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

		allDisks = mergeNVMeTempsIntoDisks(allDisks, nodesFromState)
		allDisks = mergeHostAgentSMARTIntoDisks(allDisks, nodesFromState, hosts)
		for index := range allDisks {
			if previous, ok := previousPhysicalDiskEvidence(allDisks[index], existingDisks); ok {
				allDisks[index] = preserveUnavailablePhysicalDiskEvidence(allDisks[index], previous)
			}
		}
		for _, disk := range allDisks {
			if !polledNodes[disk.Node] {
				continue
			}

			log.Debug().
				Str("node", disk.Node).
				Str("disk", disk.DevPath).
				Str("model", disk.Model).
				Str("health", disk.Health).
				Int("wearout", disk.Wearout).
				Msg("Checking disk health")

			if excludePatterns, ok := diskExcludeByNode[disk.Node]; ok && fsfilters.MatchesDeviceExclude(disk.DevPath, excludePatterns) {
				healthyDisk := proxmoxDiskFromPhysicalDisk(disk)
				healthyDisk.Health = "PASSED"
				healthyDisk.Wearout = 100
				m.alertManager.CheckDiskHealth(inst, disk.Node, healthyDisk)
				continue
			}

			m.alertManager.CheckDiskHealth(inst, disk.Node, proxmoxDiskFromPhysicalDisk(disk))
		}

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

// normalizeProxmoxDiskHealth maps the raw health strings the Proxmox disks API
// reports onto the canonical PASSED/FAILED vocabulary the disk model carries.
// ATA drives come back as PASSED or FAILED!, while SCSI/SAS drives report OK
// or a failure sentence, and the raw OK previously rendered as Unknown in the
// UI (#1595). Unrecognized values pass through untouched so nothing real is
// masked.
func normalizeProxmoxDiskHealth(health string) string {
	trimmed := strings.TrimSpace(health)
	upper := strings.ToUpper(trimmed)
	switch {
	case upper == "OK", strings.Contains(upper, "PASS"):
		return "PASSED"
	case strings.Contains(upper, "FAIL"):
		return "FAILED"
	default:
		return trimmed
	}
}

// physicalDisksFromHostAgentSMART builds PhysicalDisk entries for a node from
// its linked host agent's SMART inventory. This is the fallback when the
// Proxmox disks/list query fails: PVE probes SMART per disk inside that call,
// so a node with dozens of disks can exceed the API window and the view would
// otherwise empty even though the agent on the node sees every disk (#1516).
// Only what the agent actually reports is carried over; nothing is fabricated.
func physicalDisksFromHostAgentSMART(inst, nodeName string, smartEntries []models.HostDiskSMART) []models.PhysicalDisk {
	if len(smartEntries) == 0 {
		return nil
	}
	now := time.Now()
	disks := make([]models.PhysicalDisk, 0, len(smartEntries))
	for _, smart := range smartEntries {
		device := strings.TrimSpace(smart.Device)
		if device == "" {
			continue
		}
		devPath := device
		if !strings.HasPrefix(devPath, "/dev/") {
			devPath = "/dev/" + devPath
		}
		health := strings.TrimSpace(smart.Health)
		if health == "" {
			health = "UNKNOWN"
		}
		disks = append(disks, models.PhysicalDisk{
			ID: unifiedresources.ProxmoxPhysicalDiskSourceID(
				inst,
				nodeName,
				devPath,
				smart.Controller,
				smart.Target,
			),
			Node:            nodeName,
			Instance:        inst,
			DevPath:         devPath,
			Model:           strings.TrimSpace(smart.Model),
			Serial:          strings.TrimSpace(smart.Serial),
			WWN:             strings.TrimSpace(smart.WWN),
			Type:            strings.TrimSpace(smart.Type),
			Controller:      strings.TrimSpace(smart.Controller),
			Target:          strings.TrimSpace(smart.Target),
			Size:            smart.SizeBytes,
			Health:          health,
			Wearout:         deriveWearoutFromSMARTAttributes(smart.Attributes),
			Temperature:     smart.Temperature,
			StorageGroup:    strings.TrimSpace(smart.Pool),
			IO:              cloneDiskIO(smart.IO),
			Collection:      diskinventory.CloneStatus(smart.Collection),
			SmartAttributes: smartAttributesCopy(smart.Attributes),
			LastChecked:     now,
		})
	}
	return disks
}

func cloneDiskIO(in *models.DiskIO) *models.DiskIO {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func previousPhysicalDiskEvidence(current models.PhysicalDisk, previous []models.PhysicalDisk) (models.PhysicalDisk, bool) {
	sameScope := func(candidate models.PhysicalDisk) bool {
		return strings.EqualFold(strings.TrimSpace(candidate.Instance), strings.TrimSpace(current.Instance)) &&
			strings.EqualFold(strings.TrimSpace(candidate.Node), strings.TrimSpace(current.Node))
	}
	uniqueMatch := func(matches func(models.PhysicalDisk) bool) (models.PhysicalDisk, bool) {
		var matched models.PhysicalDisk
		found := false
		for _, candidate := range previous {
			if !sameScope(candidate) || !matches(candidate) {
				continue
			}
			if found {
				return models.PhysicalDisk{}, false
			}
			matched = candidate
			found = true
		}
		return matched, found
	}

	if serial := strings.TrimSpace(current.Serial); diskinventory.IsUsableHardwareID(serial) {
		if matched, ok := uniqueMatch(func(candidate models.PhysicalDisk) bool {
			return candidate.Serial != "" && strings.EqualFold(strings.TrimSpace(candidate.Serial), serial)
		}); ok {
			return matched, true
		}
	}
	if wwn := strings.TrimSpace(current.WWN); diskinventory.IsUsableHardwareID(wwn) {
		if matched, ok := uniqueMatch(func(candidate models.PhysicalDisk) bool {
			return candidate.WWN != "" && strings.EqualFold(strings.TrimSpace(candidate.WWN), wwn)
		}); ok {
			return matched, true
		}
	}
	if id := strings.TrimSpace(current.ID); id != "" {
		if matched, ok := uniqueMatch(func(candidate models.PhysicalDisk) bool {
			return strings.TrimSpace(candidate.ID) == id
		}); ok {
			return matched, true
		}
	}

	device := normalizeSMARTDeviceIdentifier(current.DevPath)
	if device == "" {
		return models.PhysicalDisk{}, false
	}
	return uniqueMatch(func(candidate models.PhysicalDisk) bool {
		return normalizeSMARTDeviceIdentifier(candidate.DevPath) == device &&
			diskTopologyCompatible(
				current.Controller,
				current.Target,
				candidate.Controller,
				candidate.Target,
			)
	})
}

func preserveUnavailablePhysicalDiskEvidence(current, previous models.PhysicalDisk) models.PhysicalDisk {
	if current.Collection == nil {
		return current
	}
	if current.Collection.Serial.State != diskinventory.FieldAvailable &&
		strings.TrimSpace(current.Serial) == "" {
		current.Serial = previous.Serial
	}
	if current.Collection.Temperature.State != diskinventory.FieldAvailable &&
		current.Temperature <= 0 {
		current.Temperature = previous.Temperature
	}
	if current.Collection.Controller.State != diskinventory.FieldAvailable {
		if current.Controller == "" {
			current.Controller = previous.Controller
		}
		if current.Target == "" {
			current.Target = previous.Target
		}
	}
	if current.Collection.Pool.State != diskinventory.FieldAvailable &&
		strings.TrimSpace(current.StorageGroup) == "" {
		current.StorageGroup = previous.StorageGroup
	}
	if current.Collection.IO.State != diskinventory.FieldAvailable && current.IO == nil {
		current.IO = cloneDiskIO(previous.IO)
	}
	return current
}

func proxmoxDiskFromPhysicalDisk(disk models.PhysicalDisk) proxmox.Disk {
	return proxmox.Disk{
		DevPath: disk.DevPath,
		Model:   disk.Model,
		Vendor:  disk.Vendor,
		Serial:  disk.Serial,
		Type:    disk.Type,
		Health:  disk.Health,
		Wearout: disk.Wearout,
		Size:    disk.Size,
		RPM:     disk.RPM,
		Used:    disk.Used,
		WWN:     disk.WWN,
	}
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
	defer func() {
		m.recordTaskResult(InstanceTypePVE, instanceName, pollErr)
	}()

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
	if instanceCfg.Disabled {
		if debugEnabled {
			log.Debug().Str("instance", instanceName).Msg("Skipping PVE poll: instance is paused")
		}
		return
	}

	// Poll nodes
	nodes, updatedClient, err := m.fetchPVENodes(ctx, instanceName, instanceCfg, client)
	if err != nil {
		pollErr = err
		m.markPVEInstanceNodesUnreachable(instanceName)
		return
	}
	client = updatedClient
	m.refreshPVETagColors(ctx, instanceName, client)

	// Check if client is a ClusterClient to determine health status
	connectionHealthStr := m.updatePVEConnectionHealth(ctx, instanceName, client)

	// Capture previous memory metrics so we can preserve them if detailed status fails
	prevNodeMemory, prevInstanceNodes := m.snapshotPrevNodes(instanceName)

	// Convert to models
	modelNodes, nodeEffectiveStatus, nodeDiskSources := m.pollPVENodesParallel(
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

	// Storage fallback is used to provide disk metrics when the node summary only
	// has the low-confidence /nodes figure or no disk truth at all.
	// We run this asynchronously with a short timeout so it doesn't block VM/container polling.
	// This addresses the issue where slow storage APIs (e.g., NFS mounts) can cause the entire
	// polling task to timeout before reaching VM/container polling.
	storageFallback := m.startStorageFallback(ctx, instanceName, instanceCfg, client, nodes, nodeEffectiveStatus)

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

	m.maybePollPhysicalDisksAsync(ctx, instanceName, instanceCfg, client, nodes, nodeEffectiveStatus, modelNodes)
	// Note: Physical disk monitoring is now enabled by default with a 5-minute polling interval.
	// Users can explicitly disable it in node settings. Disk data is preserved between polls.

	// Wait for storage fallback to complete (with a short timeout) before using the data.
	// This is non-blocking in the sense that VM/container polling has already completed by now.
	// We give the storage fallback goroutine up to 2 additional seconds to finish if it's still running.
	localStorageByNode := m.awaitStorageFallback(instanceName, storageFallback, 2*time.Second)

	modelNodes = m.applyStorageFallbackAndRecordNodeMetrics(instanceName, client, modelNodes, nodeDiskSources, localStorageByNode)

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
