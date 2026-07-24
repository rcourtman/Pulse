package monitoring

import (
	"context"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

var detectMonitorPVECluster = defaultDetectMonitorPVECluster

type pveClusterStatusGetter interface {
	GetClusterStatus(context.Context) ([]proxmox.ClusterStatus, error)
}

const pveMembershipRemovalConfirmations = 2

// pveNodeIdentityScope preserves the historical cluster-name node identity
// while it is unambiguous. If two configured clusters share a display name,
// the provider instance becomes the identity scope so unrelated nodes cannot
// collide in state, history, navigation, or resource projection.
func (m *Monitor) pveNodeIdentityScope(instanceName string, instanceCfg *config.PVEInstance) string {
	instanceName = strings.TrimSpace(instanceName)
	if instanceCfg == nil || !instanceCfg.IsCluster || strings.TrimSpace(instanceCfg.ClusterName) == "" {
		return instanceName
	}

	clusterName := strings.TrimSpace(instanceCfg.ClusterName)
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.config != nil {
		matches := 0
		for i := range m.config.PVEInstances {
			cfg := &m.config.PVEInstances[i]
			if cfg.IsCluster && strings.EqualFold(strings.TrimSpace(cfg.ClusterName), clusterName) {
				matches++
				if matches > 1 {
					return instanceName
				}
			}
		}
	}
	return clusterName
}

func clusterMembershipFromStatus(statuses []proxmox.ClusterStatus) (string, map[string]proxmox.ClusterStatus, bool) {
	clusterName := ""
	members := make(map[string]proxmox.ClusterStatus)
	for _, status := range statuses {
		switch strings.ToLower(strings.TrimSpace(status.Type)) {
		case "cluster":
			if clusterName == "" {
				clusterName = strings.TrimSpace(status.Name)
				if clusterName == "" {
					clusterName = strings.TrimSpace(status.ID)
				}
			}
		case "node":
			name := strings.TrimSpace(status.Name)
			if name != "" {
				members[strings.ToLower(name)] = status
			}
		}
	}
	return clusterName, members, clusterName != "" && len(members) > 0
}

func clusterMembershipRemovalAuthoritative(statuses []proxmox.ClusterStatus) bool {
	for _, status := range statuses {
		if strings.EqualFold(strings.TrimSpace(status.Type), "cluster") {
			return status.Quorate == 1
		}
	}
	return false
}

func clusterEndpointsFromStatus(
	clientConfig proxmox.ClientConfig,
	existing []config.ClusterEndpoint,
	statuses []proxmox.ClusterStatus,
) (string, []config.ClusterEndpoint, bool) {
	clusterName, members, authoritative := clusterMembershipFromStatus(statuses)
	if !authoritative {
		return "", nil, false
	}

	scheme, port := monitorClusterEndpointDefaults(clientConfig.Host)
	endpoints := make([]config.ClusterEndpoint, 0, len(members))
	for _, clusterNode := range members {
		lastSeen := time.Time{}
		for _, endpoint := range existing {
			if strings.EqualFold(strings.TrimSpace(endpoint.NodeName), strings.TrimSpace(clusterNode.Name)) {
				lastSeen = endpoint.LastSeen
				break
			}
		}
		if clusterNode.Online == 1 {
			lastSeen = time.Now()
		}
		endpoints = append(endpoints, config.ClusterEndpoint{
			NodeID:      clusterNode.ID,
			NodeName:    clusterNode.Name,
			Host:        monitorBuildClusterEndpointHost(scheme, clusterNode.Name, port),
			IP:          strings.TrimSpace(clusterNode.IP),
			GuestURL:    monitorExistingClusterGuestURL(clusterNode.Name, existing),
			IPOverride:  monitorExistingClusterIPOverride(clusterNode.Name, existing),
			Fingerprint: monitorExistingClusterFingerprint(clusterNode.Name, existing),
			Online:      clusterNode.Online == 1,
			LastSeen:    lastSeen,
		})
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return strings.ToLower(endpoints[i].NodeName) < strings.ToLower(endpoints[j].NodeName)
	})
	return clusterName, endpoints, true
}

func pveNodeByName(nodes []models.Node) map[string]models.Node {
	byName := make(map[string]models.Node, len(nodes))
	for _, node := range nodes {
		name := strings.ToLower(strings.TrimSpace(node.Name))
		if name == "" {
			continue
		}
		if existing, ok := byName[name]; ok {
			byName[name] = preferNodeForInventory(existing, node)
			continue
		}
		byName[name] = node
	}
	return byName
}

func preferNodeForInventory(existing, candidate models.Node) models.Node {
	if strings.EqualFold(candidate.Status, "online") && !strings.EqualFold(existing.Status, "online") {
		return candidate
	}
	if candidate.LastSeen.After(existing.LastSeen) {
		return candidate
	}
	return existing
}

func pveMembershipMissKey(instanceName, nodeName string) string {
	return strings.ToLower(strings.TrimSpace(instanceName)) + "\x00" + strings.ToLower(strings.TrimSpace(nodeName))
}

func (m *Monitor) clearPVEMembershipMisses(instanceName string) {
	prefix := strings.ToLower(strings.TrimSpace(instanceName)) + "\x00"
	m.mu.Lock()
	for key := range m.pveMembershipMisses {
		if strings.HasPrefix(key, prefix) {
			delete(m.pveMembershipMisses, key)
		}
	}
	m.mu.Unlock()
}

// confirmPVEMembershipRemovals requires two consecutive successful,
// absence-authoritative membership reads before a last-known member is
// retired. Ordinary telemetry omissions and failed membership reads never
// advance this counter. The durable endpoint remains in config until the
// second confirmation, so a restart safely resets the in-memory confirmation
// window instead of turning uncertainty into deletion.
func (m *Monitor) confirmPVEMembershipRemovals(
	instanceName string,
	authoritative map[string]proxmox.ClusterStatus,
	current,
	previous []models.Node,
	instanceCfg *config.PVEInstance,
) (map[string]proxmox.ClusterStatus, map[string]struct{}) {
	effective := make(map[string]proxmox.ClusterStatus, len(authoritative))
	for key, member := range authoritative {
		effective[key] = member
	}

	candidates := pveNodeByName(current)
	for key, node := range pveNodeByName(previous) {
		if _, exists := candidates[key]; !exists {
			candidates[key] = node
		}
	}
	if instanceCfg != nil {
		for _, endpoint := range instanceCfg.ClusterEndpoints {
			name := strings.TrimSpace(endpoint.NodeName)
			key := strings.ToLower(name)
			if key == "" {
				continue
			}
			if _, exists := candidates[key]; !exists {
				candidates[key] = m.placeholderNodeForInstance(instanceName, instanceCfg, name)
			}
		}
	}

	pending := make(map[string]struct{})
	m.mu.Lock()
	if m.pveMembershipMisses == nil {
		m.pveMembershipMisses = make(map[string]int)
	}
	for key := range authoritative {
		delete(m.pveMembershipMisses, pveMembershipMissKey(instanceName, key))
	}
	for key, node := range candidates {
		if _, present := authoritative[key]; present {
			continue
		}
		missKey := pveMembershipMissKey(instanceName, key)
		m.pveMembershipMisses[missKey]++
		if m.pveMembershipMisses[missKey] >= pveMembershipRemovalConfirmations {
			delete(m.pveMembershipMisses, missKey)
			continue
		}
		online := 0
		if strings.EqualFold(strings.TrimSpace(node.Status), "online") {
			online = 1
		}
		effective[key] = proxmox.ClusterStatus{
			Type:   "node",
			Name:   node.Name,
			Online: online,
		}
		pending[key] = struct{}{}
	}
	m.mu.Unlock()
	return effective, pending
}

func retainPendingClusterEndpoints(
	discovered,
	existing []config.ClusterEndpoint,
	current,
	previous []models.Node,
	pending map[string]struct{},
) []config.ClusterEndpoint {
	if len(pending) == 0 {
		return discovered
	}
	byName := make(map[string]config.ClusterEndpoint, len(discovered)+len(pending))
	for _, endpoint := range discovered {
		byName[strings.ToLower(strings.TrimSpace(endpoint.NodeName))] = endpoint
	}
	for _, endpoint := range existing {
		key := strings.ToLower(strings.TrimSpace(endpoint.NodeName))
		if _, keep := pending[key]; keep {
			byName[key] = endpoint
		}
	}
	for key, node := range pveNodeByName(append(append([]models.Node{}, current...), previous...)) {
		if _, keep := pending[key]; !keep {
			continue
		}
		if _, exists := byName[key]; exists {
			continue
		}
		byName[key] = config.ClusterEndpoint{
			NodeName: node.Name,
			Host:     node.Host,
			Online:   strings.EqualFold(strings.TrimSpace(node.Status), "online"),
			LastSeen: node.LastSeen,
		}
	}
	out := make([]config.ClusterEndpoint, 0, len(byName))
	for _, endpoint := range byName {
		out = append(out, endpoint)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].NodeName) < strings.ToLower(out[j].NodeName)
	})
	return out
}

func (m *Monitor) reconcileUncertainPVENodeInventory(
	instanceName string,
	instanceCfg *config.PVEInstance,
	current,
	previous []models.Node,
) []models.Node {
	reconciled := pveNodeByName(current)
	missingPrevious := make([]models.Node, 0, len(previous))
	for _, node := range previous {
		name := strings.ToLower(strings.TrimSpace(node.Name))
		if name == "" {
			continue
		}
		if _, present := reconciled[name]; present {
			continue
		}
		missingPrevious = append(missingPrevious, node)
	}
	for _, node := range m.preserveOrExpireNodes(missingPrevious) {
		reconciled[strings.ToLower(strings.TrimSpace(node.Name))] = node
	}
	if instanceCfg != nil {
		for _, endpoint := range instanceCfg.ClusterEndpoints {
			name := strings.TrimSpace(endpoint.NodeName)
			key := strings.ToLower(name)
			if name == "" {
				continue
			}
			if _, present := reconciled[key]; present {
				continue
			}
			reconciled[key] = m.placeholderNodeForInstance(instanceName, instanceCfg, name)
		}
	}
	return sortedPVENodeInventory(reconciled)
}

func sortedPVENodeInventory(nodes map[string]models.Node) []models.Node {
	out := make([]models.Node, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, node)
	}
	sort.Slice(out, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(out[i].Name))
		right := strings.ToLower(strings.TrimSpace(out[j].Name))
		if left == right {
			return out[i].ID < out[j].ID
		}
		return left < right
	})
	return out
}

// reconcilePVENodeInventory separates membership from telemetry. /nodes is a
// live metrics snapshot and may be partial when a member is powered off or one
// endpoint is unreachable. A valid /cluster/status response is the only
// absence-authoritative membership source; otherwise Pulse preserves the
// durable endpoint/last-known union with honest offline or unknown state.
func (m *Monitor) reconcilePVENodeInventory(
	ctx context.Context,
	instanceName string,
	instanceCfg *config.PVEInstance,
	client PVEClientInterface,
	current,
	previous []models.Node,
) []models.Node {
	if instanceCfg == nil || !instanceCfg.IsCluster {
		if len(current) > 0 || len(previous) == 0 {
			return current
		}
		m.setProviderConnectionHealth(InstanceTypePVE, instanceName, false)
		return m.preserveOrExpireNodes(previous)
	}
	if len(current) == 0 {
		m.setProviderConnectionHealth(InstanceTypePVE, instanceName, false)
	}

	getter, ok := client.(pveClusterStatusGetter)
	if !ok {
		m.clearPVEMembershipMisses(instanceName)
		return m.reconcileUncertainPVENodeInventory(instanceName, instanceCfg, current, previous)
	}
	statuses, err := getter.GetClusterStatus(ctx)
	if err != nil {
		m.clearPVEMembershipMisses(instanceName)
		log.Warn().
			Err(err).
			Str("instance", instanceName).
			Msg("Cluster membership unavailable - retaining last-known Proxmox nodes")
		return m.reconcileUncertainPVENodeInventory(instanceName, instanceCfg, current, previous)
	}
	if !clusterMembershipRemovalAuthoritative(statuses) {
		m.clearPVEMembershipMisses(instanceName)
		log.Warn().
			Str("instance", instanceName).
			Msg("Cluster membership is not quorate - retaining last-known Proxmox nodes")
		return m.reconcileUncertainPVENodeInventory(instanceName, instanceCfg, current, previous)
	}
	clusterName, members, authoritative := clusterMembershipFromStatus(statuses)
	if !authoritative {
		m.clearPVEMembershipMisses(instanceName)
		log.Warn().
			Str("instance", instanceName).
			Msg("Cluster membership response incomplete - retaining last-known Proxmox nodes")
		return m.reconcileUncertainPVENodeInventory(instanceName, instanceCfg, current, previous)
	}
	if configuredName := strings.TrimSpace(instanceCfg.ClusterName); configuredName != "" &&
		!strings.EqualFold(configuredName, clusterName) {
		m.clearPVEMembershipMisses(instanceName)
		log.Warn().
			Str("instance", instanceName).
			Str("configuredCluster", configuredName).
			Str("observedCluster", clusterName).
			Msg("Cluster identity changed during membership read - retaining last-known Proxmox nodes")
		return m.reconcileUncertainPVENodeInventory(instanceName, instanceCfg, current, previous)
	}
	members, pendingRemovals := m.confirmPVEMembershipRemovals(
		instanceName,
		members,
		current,
		previous,
		instanceCfg,
	)

	if discoveredName, endpoints, ok := clusterEndpointsFromStatus(config.CreateProxmoxConfig(instanceCfg), instanceCfg.ClusterEndpoints, statuses); ok {
		endpoints = retainPendingClusterEndpoints(
			endpoints,
			instanceCfg.ClusterEndpoints,
			current,
			previous,
			pendingRemovals,
		)
		m.refreshClusterEndpoints(instanceName, instanceCfg, discoveredName, endpoints)
	}

	currentByName := pveNodeByName(current)
	previousByName := pveNodeByName(previous)
	reconciled := make(map[string]models.Node, len(members))
	for key, membership := range members {
		node, present := currentByName[key]
		if !present {
			node, present = previousByName[key]
		}
		if !present {
			node = m.placeholderNodeForInstance(instanceName, instanceCfg, membership.Name)
		}

		if membership.Online == 0 {
			node.Status = "offline"
			node.ConnectionHealth = "error"
			node.CPU = 0
			node.Uptime = 0
		} else if _, observed := currentByName[key]; !observed {
			node.Status = "unknown"
			node.ConnectionHealth = "degraded"
			node.CPU = 0
			node.Uptime = 0
		}
		reconciled[key] = node
	}

	log.Info().
		Str("instance", instanceName).
		Str("cluster", clusterName).
		Int("observedNodes", len(current)).
		Int("membershipNodes", len(reconciled)).
		Msg("Reconciled Proxmox node telemetry against authoritative cluster membership")
	return sortedPVENodeInventory(reconciled)
}

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
	refreshed := mergeRefreshedClusterEndpoints(instanceCfg.ClusterEndpoints, discovered, instanceCfg.VerifySSL)
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
// bookkeeping is carried over only while the effective dial target is
// unchanged. Reusing a success/error from an address that was just replaced
// would present stale reachability evidence after a cluster network move.
func mergeRefreshedClusterEndpoints(existing, discovered []config.ClusterEndpoint, verifySSL bool) []config.ClusterEndpoint {
	merged := make([]config.ClusterEndpoint, 0, len(discovered))
	for _, ep := range discovered {
		for _, old := range existing {
			if !strings.EqualFold(strings.TrimSpace(old.NodeName), strings.TrimSpace(ep.NodeName)) {
				continue
			}
			oldEffectiveURL := clusterEndpointEffectiveURL(old, verifySSL, false)
			if strings.TrimSpace(ep.IP) == "" {
				ep.IP = old.IP
			}
			if strings.TrimSpace(ep.Host) == "" {
				ep.Host = old.Host
			}
			if strings.TrimSpace(ep.NodeID) == "" {
				ep.NodeID = old.NodeID
			}
			if clusterEndpointEffectiveURL(ep, verifySSL, false) == oldEffectiveURL {
				ep.PulseReachable = old.PulseReachable
				ep.LastPulseCheck = old.LastPulseCheck
				ep.PulseError = old.PulseError
			}
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

	clusterName, clusterEndpoints, authoritative := clusterEndpointsFromStatus(clientConfig, existingEndpoints, clusterStatus)
	if !authoritative || len(clusterEndpoints) <= 1 {
		return false, "", nil
	}
	return true, clusterName, clusterEndpoints
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
