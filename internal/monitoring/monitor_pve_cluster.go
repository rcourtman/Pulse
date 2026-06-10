package monitoring

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

var detectMonitorPVECluster = defaultDetectMonitorPVECluster

func (m *Monitor) detectClusterMembership(ctx context.Context, instanceName string, instanceCfg *config.PVEInstance, client PVEClientInterface) {
	_ = client

	// Re-check cluster status every 5 minutes. Standalone instances may turn
	// out to be cluster members (#437). Cluster instances re-discover their
	// endpoints, because the addresses captured at add time go stale when the
	// cluster re-IPs - without a refresh Pulse keeps dialing (and displaying)
	// dead addresses forever (#1493).
	if time.Since(m.lastClusterCheck[instanceName]) <= 5*time.Minute {
		return
	}

	m.lastClusterCheck[instanceName] = time.Now()

	clientConfig := config.CreateProxmoxConfig(instanceCfg)
	isCluster, clusterName, clusterEndpoints := detectMonitorPVECluster(clientConfig, instanceCfg.ClusterEndpoints)
	if !isCluster || strings.TrimSpace(clusterName) == "" || len(clusterEndpoints) == 0 {
		return
	}

	if instanceCfg.IsCluster {
		m.refreshClusterEndpoints(instanceName, instanceCfg, clusterName, clusterEndpoints)
		return
	}

	log.Info().
		Str("instance", instanceName).
		Str("cluster", clusterName).
		Int("endpoints", len(clusterEndpoints)).
		Msg("Detected that standalone node is actually part of a cluster - updating configuration")

	updated := false
	for i := range m.config.PVEInstances {
		if m.config.PVEInstances[i].Name == instanceName {
			m.config.PVEInstances[i].IsCluster = true
			m.config.PVEInstances[i].ClusterName = clusterName
			m.config.PVEInstances[i].ClusterEndpoints = clusterEndpoints
			updated = true
			log.Info().
				Str("instance", instanceName).
				Str("cluster", clusterName).
				Msg("Updated standalone PVE instance with full cluster metadata")
			break
		}
	}
	if !updated {
		return
	}

	m.normalizePVEConfigState()
	if m.persistence != nil {
		if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
			log.Warn().Err(err).Msg("failed to persist updated node configuration")
		}
	}
}

// refreshClusterEndpoints reconciles the stored cluster endpoints with what
// the cluster reports now, keeping user-managed fields (IP overrides, guest
// URLs, fingerprints - already carried over by the discovery helpers). When
// node addresses changed, the config is persisted and the failover client is
// rebuilt so polling stops dialing the dead addresses.
func (m *Monitor) refreshClusterEndpoints(instanceName string, instanceCfg *config.PVEInstance, clusterName string, discovered []config.ClusterEndpoint) {
	refreshed := mergeRefreshedClusterEndpoints(instanceCfg.ClusterEndpoints, discovered)
	if !clusterEndpointIdentityChanged(instanceCfg.ClusterEndpoints, refreshed) {
		return
	}

	log.Info().
		Str("instance", instanceName).
		Str("cluster", clusterName).
		Int("endpoints", len(refreshed)).
		Msg("Cluster node addresses changed since discovery - refreshing stored endpoints")

	instanceCfg.ClusterEndpoints = refreshed
	if !strings.EqualFold(clusterName, "unknown cluster") {
		instanceCfg.ClusterName = clusterName
	}

	updated := false
	for i := range m.config.PVEInstances {
		if m.config.PVEInstances[i].Name == instanceName {
			m.config.PVEInstances[i].ClusterName = instanceCfg.ClusterName
			m.config.PVEInstances[i].ClusterEndpoints = refreshed
			updated = true
			break
		}
	}
	if !updated {
		return
	}

	m.normalizePVEConfigState()
	if m.persistence != nil {
		if err := m.persistence.SaveNodesConfig(m.config.PVEInstances, m.config.PBSInstances, m.config.PMGInstances); err != nil {
			log.Warn().Err(err).Msg("failed to persist refreshed cluster endpoints")
		}
	}

	m.rebuildPVEClusterClient(instanceName)
}

// mergeRefreshedClusterEndpoints folds freshly discovered endpoints over the
// stored set. The cluster status API can omit per-node fields, so discovery
// never erases information Pulse already had; Pulse-side reachability
// bookkeeping is carried over because it is recomputed each poll and dropping
// it would flap the UI to "unknown" after every refresh.
func mergeRefreshedClusterEndpoints(existing, discovered []config.ClusterEndpoint) []config.ClusterEndpoint {
	merged := make([]config.ClusterEndpoint, 0, len(discovered))
	for _, ep := range discovered {
		for _, old := range existing {
			if !strings.EqualFold(strings.TrimSpace(old.NodeName), strings.TrimSpace(ep.NodeName)) {
				continue
			}
			if strings.TrimSpace(ep.IP) == "" {
				ep.IP = old.IP
			}
			if strings.TrimSpace(ep.Host) == "" {
				ep.Host = old.Host
			}
			if strings.TrimSpace(ep.NodeID) == "" {
				ep.NodeID = old.NodeID
			}
			ep.PulseReachable = old.PulseReachable
			ep.LastPulseCheck = old.LastPulseCheck
			ep.PulseError = old.PulseError
			break
		}
		merged = append(merged, ep)
	}
	return merged
}

// clusterEndpointIdentityChanged reports whether the set of nodes or any
// node's address changed. Volatile fields (Online, LastSeen, Pulse
// reachability) are deliberately ignored - they change every poll and are not
// a reason to rewrite config or rebuild clients.
func clusterEndpointIdentityChanged(existing, refreshed []config.ClusterEndpoint) bool {
	if len(existing) != len(refreshed) {
		return true
	}
	byName := make(map[string]config.ClusterEndpoint, len(existing))
	for _, ep := range existing {
		byName[strings.ToLower(strings.TrimSpace(ep.NodeName))] = ep
	}
	for _, ep := range refreshed {
		old, ok := byName[strings.ToLower(strings.TrimSpace(ep.NodeName))]
		if !ok {
			return true
		}
		if strings.TrimSpace(old.IP) != strings.TrimSpace(ep.IP) ||
			strings.TrimSpace(old.Host) != strings.TrimSpace(ep.Host) ||
			strings.TrimSpace(old.NodeID) != strings.TrimSpace(ep.NodeID) {
			return true
		}
	}
	return false
}

// rebuildPVEClusterClient swaps the failover client for a cluster instance so
// it picks up the refreshed endpoint list. The in-flight poll keeps using the
// old client; the next poll cycle gets the new one.
func (m *Monitor) rebuildPVEClusterClient(instanceName string) {
	if m.config == nil || m.pveClients == nil {
		return
	}

	var pve *config.PVEInstance
	for i := range m.config.PVEInstances {
		if m.config.PVEInstances[i].Name == instanceName {
			pve = &m.config.PVEInstances[i]
			break
		}
	}
	if pve == nil || !pve.IsCluster || len(pve.ClusterEndpoints) == 0 {
		return
	}

	endpoints, endpointFingerprints := m.buildClusterEndpointsForReconnect(*pve)
	clientConfig := config.CreateProxmoxConfig(pve)
	clientConfig.Timeout = m.config.ConnectionTimeout
	clusterClient := proxmox.NewClusterClient(pve.Name, clientConfig, endpoints, endpointFingerprints)

	m.mu.Lock()
	m.pveClients[instanceName] = clusterClient
	m.mu.Unlock()

	log.Info().
		Str("instance", instanceName).
		Strs("endpoints", endpoints).
		Msg("Rebuilt cluster client with refreshed endpoints")
}

func (m *Monitor) updateClusterEndpointStatus(instanceName string, instanceCfg *config.PVEInstance, client PVEClientInterface, modelNodes []models.Node) {
	if !instanceCfg.IsCluster || len(instanceCfg.ClusterEndpoints) == 0 {
		return
	}

	// Create a map of online nodes from our polling results
	onlineNodes := make(map[string]bool)
	for _, node := range modelNodes {
		// Node is online if we successfully got its data
		onlineNodes[node.Name] = node.Status == "online"
	}

	// Get Pulse connectivity status from ClusterClient if available
	var pulseHealth map[string]proxmox.EndpointHealth
	if clusterClient, ok := client.(*proxmox.ClusterClient); ok {
		pulseHealth = clusterClient.GetHealthStatusWithErrors()
	}

	// Update the online status for each cluster endpoint
	hasFingerprint := instanceCfg.Fingerprint != ""
	for i := range instanceCfg.ClusterEndpoints {
		if online, exists := onlineNodes[instanceCfg.ClusterEndpoints[i].NodeName]; exists {
			instanceCfg.ClusterEndpoints[i].Online = online
			if online {
				instanceCfg.ClusterEndpoints[i].LastSeen = time.Now()
			}
		}

		// Update Pulse connectivity status
		if pulseHealth != nil {
			// Try to find the endpoint in the health map by matching the effective URL
			endpointURL := clusterEndpointEffectiveURL(instanceCfg.ClusterEndpoints[i], instanceCfg.VerifySSL, hasFingerprint)
			if health, exists := pulseHealth[endpointURL]; exists {
				reachable := health.Healthy
				instanceCfg.ClusterEndpoints[i].PulseReachable = &reachable
				if !health.LastCheck.IsZero() {
					instanceCfg.ClusterEndpoints[i].LastPulseCheck = &health.LastCheck
				}
				instanceCfg.ClusterEndpoints[i].PulseError = health.LastError
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

func defaultDetectMonitorPVECluster(clientConfig proxmox.ClientConfig, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
	tempClient, err := proxmox.NewClient(clientConfig)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create client for runtime cluster detection")
		return false, "", nil
	}

	var (
		clusterStatus []proxmox.ClusterStatus
		lastErr       error
	)
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		detectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		clusterStatus, lastErr = tempClient.GetClusterStatus(detectCtx)
		cancel()
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return false, "", nil
	}

	clusterName := ""
	clusterNodes := make([]proxmox.ClusterStatus, 0, len(clusterStatus))
	for _, status := range clusterStatus {
		switch status.Type {
		case "cluster":
			clusterName = strings.TrimSpace(status.Name)
		case "node":
			clusterNodes = append(clusterNodes, status)
		}
	}
	if len(clusterNodes) <= 1 || clusterName == "" {
		return false, "", nil
	}

	scheme, port := monitorClusterEndpointDefaults(clientConfig.Host)
	endpoints := make([]config.ClusterEndpoint, 0, len(clusterNodes))
	for _, clusterNode := range clusterNodes {
		endpoint := config.ClusterEndpoint{
			NodeID:      clusterNode.ID,
			NodeName:    clusterNode.Name,
			Host:        monitorBuildClusterEndpointHost(scheme, clusterNode.Name, port),
			IP:          strings.TrimSpace(clusterNode.IP),
			GuestURL:    monitorExistingClusterGuestURL(clusterNode.Name, existingEndpoints),
			IPOverride:  monitorExistingClusterIPOverride(clusterNode.Name, existingEndpoints),
			Fingerprint: monitorExistingClusterFingerprint(clusterNode.Name, existingEndpoints),
			Online:      clusterNode.Online == 1,
			LastSeen:    time.Now(),
		}
		endpoints = append(endpoints, endpoint)
	}

	return true, clusterName, endpoints
}

func monitorClusterEndpointDefaults(rawHost string) (string, string) {
	value := strings.TrimSpace(rawHost)
	if value == "" {
		return "https", config.DefaultPVEPort
	}
	if !strings.HasPrefix(strings.ToLower(value), "http://") && !strings.HasPrefix(strings.ToLower(value), "https://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "https", config.DefaultPVEPort
	}
	scheme := strings.TrimSpace(parsed.Scheme)
	if scheme == "" {
		scheme = "https"
	}
	port := strings.TrimSpace(parsed.Port())
	if port == "" {
		port = config.DefaultPVEPort
	}
	return scheme, port
}

func monitorBuildClusterEndpointHost(scheme, host, port string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return scheme + "://" + host
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}

func monitorExistingClusterGuestURL(nodeName string, existing []config.ClusterEndpoint) string {
	for _, endpoint := range existing {
		if strings.EqualFold(strings.TrimSpace(endpoint.NodeName), strings.TrimSpace(nodeName)) {
			return strings.TrimSpace(endpoint.GuestURL)
		}
	}
	return ""
}

func monitorExistingClusterIPOverride(nodeName string, existing []config.ClusterEndpoint) string {
	for _, endpoint := range existing {
		if strings.EqualFold(strings.TrimSpace(endpoint.NodeName), strings.TrimSpace(nodeName)) {
			return strings.TrimSpace(endpoint.IPOverride)
		}
	}
	return ""
}

func monitorExistingClusterFingerprint(nodeName string, existing []config.ClusterEndpoint) string {
	for _, endpoint := range existing {
		if strings.EqualFold(strings.TrimSpace(endpoint.NodeName), strings.TrimSpace(nodeName)) {
			return strings.TrimSpace(endpoint.Fingerprint)
		}
	}
	return ""
}
