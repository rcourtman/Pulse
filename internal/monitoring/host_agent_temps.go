package monitoring

import (
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

// getHostAgentTemperature looks for a matching host agent and converts
// its sensor data to the Temperature model used by Proxmox nodes.
// It first tries to match by nodeID using the LinkedNodeID field (preferred for
// duplicate hostname scenarios), then falls back to hostname matching.
// Returns nil if no matching host agent is found or if no temperature data is available.
func (m *Monitor) getHostAgentTemperature(nodeName string) *models.Temperature {
	return m.getHostAgentTemperatureForNode(models.Node{Name: nodeName})
}

func shouldSkipTemperatureSSHCollection(hostAgentTemp *models.Temperature) bool {
	if hostAgentTemp == nil {
		return false
	}

	if !hostAgentTemp.Available || !isHostAgentTemperatureRecent(hostAgentTemp.LastUpdate) {
		return false
	}

	return hasUsableTemperatureReading(hostAgentTemp)
}

func hasUsableTemperatureReading(temp *models.Temperature) bool {
	if temp == nil {
		return false
	}

	if temp.CPUPackage > 0 || temp.CPUMax > 0 {
		return true
	}

	for _, core := range temp.Cores {
		if core.Temp > 0 {
			return true
		}
	}

	for _, device := range temp.NVMe {
		if device.Temp > 0 {
			return true
		}
	}

	for _, gpu := range temp.GPU {
		if gpu.Edge > 0 || gpu.Junction > 0 || gpu.Mem > 0 {
			return true
		}
	}

	return hasUsableSMARTTemperature(temp)
}

func hasUsableSMARTTemperature(temp *models.Temperature) bool {
	if temp == nil || !temp.HasSMART {
		return false
	}

	for _, disk := range temp.SMART {
		if disk.Temperature > 0 && !disk.StandbySkipped {
			return true
		}
	}
	return false
}

// getHostAgentTemperatureForNode returns a polled Proxmox node's reading from
// its host agent (see hostAgentForNode), else from a sibling agent's cluster
// sensor cache. node is the poller's model of the node: its source ID, native
// name, connection and identity stamps.
func (m *Monitor) getHostAgentTemperatureForNode(node models.Node) *models.Temperature {
	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return nil
	}
	hosts := readState.Hosts()
	nodes := readState.Nodes()
	slot := m.polledNodeSlot(hosts, nodes, node)

	matchedHost := hostAgentForNode(hosts, nodes, slot, node.Name)
	if matchedHost == nil {
		// No directly-linked host agent found — check cluster sensor cache
		return m.getClusterSensorTemperature(hosts, nodes, node, slot)
	}

	// Check if the host agent has temperature data. SMART-only reports are
	// valid here: some PVE nodes expose no CPU sensor chip but still provide
	// disk temperatures through the local agent.
	sensors := matchedHost.Sensors()
	if sensors == nil || (len(sensors.TemperatureCelsius) == 0 && len(sensors.SMART) == 0) {
		// Host agent exists but has no temperature data — try cluster cache
		return m.getClusterSensorTemperature(hosts, nodes, node, slot)
	}

	// An agent that stopped reporting keeps its last sensors in state. Those
	// describe the machine as it was, so they must not keep feeding the node
	// (and through it alerts and history) as a live reading. The row's own
	// LastSeen cannot tell: a row merged with the Proxmox node stays fresh from
	// every PVE poll, so read the agent source's sighting.
	agentStatus, ok := matchedHost.SourceStatus(unifiedresources.SourceAgent)
	if !ok || !hostAgentReportCurrent(agentStatus.LastSeen, matchedHost.IntervalSeconds(), time.Now()) {
		return m.getClusterSensorTemperature(hosts, nodes, node, slot)
	}

	// Convert host agent sensor data to Temperature model, stamped with the
	// agent's report time rather than the merged row's.
	return convertUnifiedHostSensorsToTemperature(sensors, agentStatus.LastSeen)
}

// carriedTemperatureOutlivesAgentLease reports whether a temperature a node
// carries over from an earlier poll may have come from the node's host agent
// after that agent stopped reporting. Agent readings are stamped with the
// agent's report time and only count inside its reporting lease, so a reading
// stamped no later than a lapsed agent's last report is treated as the agent's
// and is not carried. Readings carry no source, so an SSH or cluster-cache
// reading of that age is dropped too; it is already older than the lease, and
// the bound errs toward a gap rather than a stale value. A reading stamped
// after that report keeps the ordinary carry window.
func (m *Monitor) carriedTemperatureOutlivesAgentLease(node models.Node, temp *models.Temperature, now time.Time) bool {
	if temp == nil {
		return false
	}
	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return false
	}
	hosts := readState.Hosts()
	nodes := readState.Nodes()
	matchedHost := hostAgentForNode(hosts, nodes, m.polledNodeSlot(hosts, nodes, node), node.Name)
	if matchedHost == nil {
		return false
	}
	agentStatus, ok := matchedHost.SourceStatus(unifiedresources.SourceAgent)
	if !ok || agentStatus.LastSeen.IsZero() ||
		hostAgentReportCurrent(agentStatus.LastSeen, matchedHost.IntervalSeconds(), now) {
		return false
	}
	return !temp.LastUpdate.After(agentStatus.LastSeen)
}

// nodeSlot is the read-state identity of one polled Proxmox node: the source
// node IDs and connections that denote it.
type nodeSlot struct {
	ids       []string
	instances []string
}

func (s *nodeSlot) add(id, instance string) {
	if id = strings.TrimSpace(id); id != "" && !s.hasID(id) {
		s.ids = append(s.ids, id)
	}
	if instance = strings.TrimSpace(instance); instance != "" && !s.hasInstance(instance) {
		s.instances = append(s.instances, instance)
	}
}

func (s nodeSlot) hasID(id string) bool {
	return id != "" && slices.Contains(s.ids, id)
}

func (s nodeSlot) hasInstance(instance string) bool {
	return instance != "" && slices.Contains(s.instances, instance)
}

// polledNodeSlot returns the read-state identity of a polled Proxmox node. A
// node the read state holds under its own source ID is that node alone. The
// state folds the views of one machine reached through two connections (a
// cluster added twice, or a multi-homed host added by each address) into one
// slot, kept under whichever view its merge preference keeps, so a polled view
// the read state does not hold also counts as the one same-named node proven to
// be that machine: by models.NodeObservationsSameMachine, or by
// models.HostAgentBridgesNodeViews for the agent linked to that node. Two or
// more such candidates are ambiguous and none is taken, as the state's alias
// resolution declines them. Agent links and cluster-sensor scope are matched
// against the slot, never against a same-named node of another machine.
func (m *Monitor) polledNodeSlot(hosts []*unifiedresources.HostView, nodes []*unifiedresources.NodeView, node models.Node) nodeSlot {
	var slot nodeSlot
	if strings.TrimSpace(node.ID) == "" {
		return slot
	}
	for _, view := range nodes {
		if view != nil && view.SourceID() == node.ID {
			slot.add(node.ID, firstNonEmptyString(node.Instance, view.Instance()))
			return slot
		}
	}
	slot.add(node.ID, node.Instance)

	var partner *unifiedresources.NodeView
	for _, view := range nodes {
		if view == nil || !strings.EqualFold(view.NodeName(), node.Name) {
			continue
		}
		if !m.nodeViewIsSameMachine(hosts, view, node) {
			continue
		}
		if partner != nil {
			return slot
		}
		partner = view
	}
	if partner != nil {
		slot.add(partner.SourceID(), partner.Instance())
	}
	return slot
}

// nodeViewIsSameMachine reports whether a read-state Proxmox node and a polled
// node are one machine by the evidence the state folds node views on.
func (m *Monitor) nodeViewIsSameMachine(hosts []*unifiedresources.HostView, view *unifiedresources.NodeView, node models.Node) bool {
	stateNode := m.stateNodeFromView(view)
	if models.NodeObservationsSameMachine(stateNode, node) {
		return true
	}
	for _, host := range hosts {
		if host.LinkedNodeID() == stateNode.ID {
			return models.HostAgentBridgesNodeViews(hostFromReadStateView(host), stateNode, node)
		}
	}
	return false
}

// nodeHasProvenViewInConnection reports whether a polled node has a view under
// another Proxmox connection that is the same machine: a cluster added through
// two connections lists the same members, each with its TLS fingerprint, so
// that connection's view of the node's name is the polled node.
func (m *Monitor) nodeHasProvenViewInConnection(node models.Node, instance string) bool {
	instanceCfg := m.getInstanceConfig(instance)
	if instanceCfg == nil || !instanceCfg.IsCluster {
		return false
	}
	fingerprint := pveNodeTLSFingerprint(instanceCfg, node.Name)
	if fingerprint == "" {
		return false
	}
	return models.NodeObservationsSameMachine(models.Node{
		Name:            node.Name,
		Instance:        instanceCfg.Name,
		ClusterName:     instanceCfg.ClusterName,
		IsClusterMember: true,
		TLSFingerprint:  fingerprint,
	}, node)
}

// stateNodeFromView restores the identity a read-state Proxmox node carries in
// state, including the config-derived TLS fingerprint the registry does not
// keep, the way previousNodesForInstance does.
func (m *Monitor) stateNodeFromView(view *unifiedresources.NodeView) models.Node {
	node := previousNodeFromView(view)
	node.TLSFingerprint = pveNodeTLSFingerprint(m.getInstanceConfig(node.Instance), node.Name)
	return node
}

// hostAgentForNode returns the host agent for a polled Proxmox node: the agent
// linked to the node's slot, else the one unlinked agent whose hostname is the
// node's name. Several connections can each have a node with the same name
// (one "px1" per site), so the hostname fallback never takes an agent linked to
// another node or to a guest, and finds nothing when a Proxmox node outside the
// slot or a second unlinked agent has the name. Every link source (automatic,
// manual, restored from continuity) stores the Proxmox source node ID, so slot
// IDs compare with LinkedNodeID directly.
func hostAgentForNode(hosts []*unifiedresources.HostView, nodes []*unifiedresources.NodeView, slot nodeSlot, nodeName string) *unifiedresources.HostView {
	for _, host := range hosts {
		if slot.hasID(host.LinkedNodeID()) {
			log.Debug().
				Str("nodeID", host.LinkedNodeID()).
				Str("hostAgentID", host.ID()).
				Str("hostname", host.Hostname()).
				Msg("Matched host agent to node via LinkedNodeID")
			return host
		}
	}

	// Fallback for an agent that is not linked yet.
	name := strings.ToLower(strings.TrimSpace(nodeName))
	if name == "" || otherProxmoxNodeHasName(nodes, slot, name) {
		return nil
	}
	var match *unifiedresources.HostView
	for _, host := range hosts {
		if host.LinkedNodeID() != "" || host.LinkedVMID() != "" || host.LinkedContainerID() != "" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(host.Hostname())) != name {
			continue
		}
		if match != nil {
			return nil
		}
		match = host
	}
	return match
}

// otherProxmoxNodeHasName reports whether a Proxmox node outside slot is named
// name (lowercase). With an empty slot every node of that name counts.
func otherProxmoxNodeHasName(nodes []*unifiedresources.NodeView, slot nodeSlot, name string) bool {
	for _, node := range nodes {
		if node == nil || slot.hasID(node.SourceID()) {
			continue
		}
		if strings.ToLower(node.NodeName()) == name {
			return true
		}
	}
	return false
}

// proxmoxNodeInstance returns the Proxmox connection a node belongs to, or ""
// when the node is not in the read state.
func proxmoxNodeInstance(nodes []*unifiedresources.NodeView, nodeID string) string {
	if nodeID == "" {
		return ""
	}
	for _, node := range nodes {
		if node != nil && node.SourceID() == nodeID {
			return node.Instance()
		}
	}
	return ""
}

func convertUnifiedHostSensorsToTemperature(sensors *unifiedresources.HostSensorMeta, lastSeen time.Time) *models.Temperature {
	if sensors == nil {
		return nil
	}

	return convertHostSensorsToTemperature(models.HostSensorSummary{
		TemperatureCelsius: cloneStringFloatMap(sensors.TemperatureCelsius),
		FanRPM:             cloneStringFloatMap(sensors.FanRPM),
		PowerWatts:         cloneStringFloatMap(sensors.PowerWatts),
		Additional:         cloneStringFloatMap(sensors.Additional),
		GPU:                convertUnifiedHostGPU(sensors.GPU),
		SMART:              convertUnifiedHostSMART(sensors.SMART),
	}, lastSeen)
}

func convertUnifiedHostGPU(gpus []unifiedresources.HostGPUSensor) []models.HostGPUSensor {
	if len(gpus) == 0 {
		return nil
	}
	result := make([]models.HostGPUSensor, len(gpus))
	for i, gpu := range gpus {
		result[i] = models.HostGPUSensor{
			ID:                 gpu.ID,
			Name:               gpu.Name,
			TemperatureCelsius: cloneFloat64Ptr(gpu.TemperatureCelsius),
			UtilizationPercent: cloneFloat64Ptr(gpu.UtilizationPercent),
			MemoryUsedBytes:    cloneInt64Ptr(gpu.MemoryUsedBytes),
			MemoryTotalBytes:   cloneInt64Ptr(gpu.MemoryTotalBytes),
		}
	}
	return result
}

func convertUnifiedHostSMART(smart []unifiedresources.HostSMARTMeta) []models.HostDiskSMART {
	if len(smart) == 0 {
		return nil
	}

	result := make([]models.HostDiskSMART, len(smart))
	for i, disk := range smart {
		result[i] = models.HostDiskSMART{
			Device:      disk.Device,
			Model:       disk.Model,
			Serial:      disk.Serial,
			WWN:         disk.WWN,
			Type:        disk.Type,
			SizeBytes:   disk.SizeBytes,
			Temperature: disk.Temperature,
			Health:      disk.Health,
			Standby:     disk.Standby,
			Pool:        disk.Pool,
			Attributes:  cloneSMARTAttributesModel(disk.Attributes),
		}
	}
	return result
}

func cloneSMARTAttributesModel(src *models.SMARTAttributes) *models.SMARTAttributes {
	if src == nil {
		return nil
	}
	dest := *src
	return &dest
}

// getClusterSensorTemperature returns the reading a host agent on a sibling
// cluster node collected for this node over SSH. Siblings are reported by bare
// node name, which nodes of different connections can share, so a reading only
// serves the node when the node its reporting agent is linked to belongs to one
// of the slot's connections, or to a connection with a proven view of the node
// (a cluster added twice). Readings from an unlinked agent serve no node.
func (m *Monitor) getClusterSensorTemperature(hosts []*unifiedresources.HostView, nodes []*unifiedresources.NodeView, node models.Node, slot nodeSlot) *models.Temperature {
	name := strings.ToLower(strings.TrimSpace(node.Name))
	if name == "" {
		return nil
	}

	// Reuse the same staleness threshold as direct host agents (2 minutes)
	var candidates []clusterSensorsCacheEntry
	m.clusterSensorsMu.RLock()
	for _, entry := range m.clusterSensorsCache {
		if entry.nodeName == name && isHostAgentTemperatureRecent(entry.updatedAt) {
			candidates = append(candidates, entry)
		}
	}
	m.clusterSensorsMu.RUnlock()

	var newest *clusterSensorsCacheEntry
	for i := range candidates {
		reporterInstance := proxmoxNodeInstance(nodes, linkedNodeIDForAgent(hosts, candidates[i].reporterID))
		if reporterInstance == "" ||
			(!slot.hasInstance(reporterInstance) && !m.nodeHasProvenViewInConnection(node, reporterInstance)) {
			continue
		}
		if newest == nil || candidates[i].updatedAt.After(newest.updatedAt) {
			newest = &candidates[i]
		}
	}
	if newest == nil {
		return nil
	}
	return convertHostSensorsToTemperature(newest.sensors, newest.updatedAt)
}

// linkedNodeIDForAgent returns the Proxmox source node ID the agent with the
// given agent ID (models.Host.ID) is linked to, or "".
func linkedNodeIDForAgent(hosts []*unifiedresources.HostView, agentID string) string {
	if agentID == "" {
		return ""
	}
	for _, host := range hosts {
		if host.AgentID() == agentID {
			return host.LinkedNodeID()
		}
	}
	return ""
}

// convertHostSensorsToTemperature converts HostSensorSummary to the Temperature model.
// The host agent reports temperatures in a flat map with keys like:
// - "cpu_package" -> CPU package temperature
// - "cpu_core_0", "cpu_core_1", etc. -> individual core temperatures
// - "nvme0", "nvme1", etc. -> NVMe temperatures
// - "gpu_edge", "gpu_junction", etc. -> GPU temperatures
func convertHostSensorsToTemperature(sensors models.HostSensorSummary, lastSeen time.Time) *models.Temperature {
	if len(sensors.TemperatureCelsius) == 0 && len(sensors.SMART) == 0 {
		return nil
	}

	temp := &models.Temperature{
		Available:  true,
		LastUpdate: lastSeen,
		Cores:      []models.CoreTemp{},
		NVMe:       []models.NVMeTemp{},
		GPU:        []models.GPUTemp{},
	}

	var coreTemps []float64
	corePattern := regexp.MustCompile(`^cpu_core_(\d+)$`)
	nvmePattern := regexp.MustCompile(`^(nvme\d+)$`)
	gpuPattern := regexp.MustCompile(`^gpu_(.+)$`)

	for key, value := range sensors.TemperatureCelsius {
		keyLower := strings.ToLower(key)

		// CPU package temperature
		if keyLower == "cpu_package" {
			temp.CPUPackage = value
			temp.HasCPU = true
			continue
		}

		// CPU core temperatures
		if matches := corePattern.FindStringSubmatch(keyLower); len(matches) == 2 {
			coreNum, err := strconv.Atoi(matches[1])
			if err == nil {
				temp.Cores = append(temp.Cores, models.CoreTemp{
					Core: coreNum,
					Temp: value,
				})
				coreTemps = append(coreTemps, value)
				temp.HasCPU = true
			}
			continue
		}

		// NVMe temperatures
		if matches := nvmePattern.FindStringSubmatch(keyLower); len(matches) == 2 {
			temp.NVMe = append(temp.NVMe, models.NVMeTemp{
				Device: matches[1],
				Temp:   value,
			})
			temp.HasNVMe = true
			continue
		}

		// GPU temperatures (gpu_edge, gpu_junction, gpu_mem, or generic gpu_<device>)
		if matches := gpuPattern.FindStringSubmatch(keyLower); len(matches) == 2 {
			gpuKey := matches[1]
			// Handle specific GPU temp types
			if gpuKey == "edge" || gpuKey == "junction" || gpuKey == "mem" {
				// Find or create GPU entry for the default device
				found := false
				for i := range temp.GPU {
					if temp.GPU[i].Device == "gpu0" {
						switch gpuKey {
						case "edge":
							temp.GPU[i].Edge = value
						case "junction":
							temp.GPU[i].Junction = value
						case "mem":
							temp.GPU[i].Mem = value
						}
						found = true
						break
					}
				}
				if !found {
					gpu := models.GPUTemp{Device: "gpu0"}
					switch gpuKey {
					case "edge":
						gpu.Edge = value
					case "junction":
						gpu.Junction = value
					case "mem":
						gpu.Mem = value
					}
					temp.GPU = append(temp.GPU, gpu)
				}
			} else {
				// Generic GPU entry with edge temp
				temp.GPU = append(temp.GPU, models.GPUTemp{
					Device: gpuKey,
					Edge:   value,
				})
			}
			temp.HasGPU = true
			continue
		}
	}

	// Sort cores by core number
	sort.Slice(temp.Cores, func(i, j int) bool {
		return temp.Cores[i].Core < temp.Cores[j].Core
	})

	// Sort NVMe by device name
	sort.Slice(temp.NVMe, func(i, j int) bool {
		return temp.NVMe[i].Device < temp.NVMe[j].Device
	})

	// Calculate CPUMax from core temperatures if package temp wasn't available
	if len(coreTemps) > 0 {
		maxTemp := 0.0
		for _, t := range coreTemps {
			if t > maxTemp {
				maxTemp = t
			}
		}
		temp.CPUMax = maxTemp

		// If no package temp, use max core temp as package
		if temp.CPUPackage == 0 {
			temp.CPUPackage = maxTemp
		}
	}

	// Convert S.M.A.R.T. data from host agent
	if len(sensors.SMART) > 0 {
		temp.SMART = make([]models.DiskTemp, 0, len(sensors.SMART))
		for _, disk := range sensors.SMART {
			// Skip disks in standby (no temperature data)
			if disk.Standby {
				continue
			}
			temp.SMART = append(temp.SMART, models.DiskTemp{
				Device:      canonicalSMARTDevicePath(disk.Device),
				Serial:      disk.Serial,
				WWN:         disk.WWN,
				Model:       disk.Model,
				Type:        disk.Type,
				Temperature: disk.Temperature,
				LastUpdated: lastSeen,
			})
		}
		temp.HasSMART = len(temp.SMART) > 0
	}

	// Validate we have at least some data
	if !temp.HasCPU && !temp.HasGPU && !temp.HasNVMe && !temp.HasSMART {
		return nil
	}

	log.Debug().
		Str("source", "agent").
		Float64("cpuPackage", temp.CPUPackage).
		Float64("cpuMax", temp.CPUMax).
		Int("coreCount", len(temp.Cores)).
		Int("nvmeCount", len(temp.NVMe)).
		Int("gpuCount", len(temp.GPU)).
		Int("smartCount", len(temp.SMART)).
		Msg("Converted host agent sensors to temperature data")

	return temp
}

func canonicalSMARTDevicePath(device string) string {
	normalized := normalizeSMARTDeviceIdentifier(device)
	if normalized == "" {
		return ""
	}
	return "/dev/" + normalized
}

// isHostAgentTemperatureRecent checks if the host agent temperature data is recent enough to use.
// We consider data stale if the host hasn't reported in more than 2 minutes.
// hostAgentReportCurrent reports whether an agent's last report is still inside
// the reporting lease that keeps the agent online (evaluateHostAgents uses the
// same window), so its sensor readings still describe the machine.
func hostAgentReportCurrent(lastSeen time.Time, intervalSeconds int, now time.Time) bool {
	return !lastSeen.IsZero() && now.Sub(lastSeen) <= hostAgentHealthWindow(intervalSeconds)
}

func isHostAgentTemperatureRecent(lastSeen time.Time) bool {
	const staleDuration = 2 * time.Minute
	return time.Since(lastSeen) < staleDuration
}

// mergeTemperatureData merges host agent temperature with existing/proxy temperature data.
// Host agent data takes priority for CPU temperatures since it's more reliable (no SSH required).
// NVMe/SMART data is merged - host agent NVMe data supplements proxy SMART data.
func mergeTemperatureData(hostAgentTemp, proxyTemp *models.Temperature) *models.Temperature {
	if hostAgentTemp == nil {
		return proxyTemp
	}
	if proxyTemp == nil {
		return hostAgentTemp
	}

	// Start with host agent data as base since it's more reliable
	result := &models.Temperature{
		CPUPackage:   hostAgentTemp.CPUPackage,
		CPUMax:       hostAgentTemp.CPUMax,
		CPUMin:       proxyTemp.CPUMin,                                           // Preserve historical min
		CPUMaxRecord: math.Max(hostAgentTemp.CPUPackage, proxyTemp.CPUMaxRecord), // Update historical max
		MinRecorded:  proxyTemp.MinRecorded,
		MaxRecorded:  proxyTemp.MaxRecorded,
		Cores:        hostAgentTemp.Cores,
		GPU:          hostAgentTemp.GPU,
		NVMe:         hostAgentTemp.NVMe,
		Available:    true,
		HasCPU:       hostAgentTemp.HasCPU,
		HasGPU:       hostAgentTemp.HasGPU,
		HasNVMe:      hostAgentTemp.HasNVMe,
		// The legacy-format marker describes the node's SSH sensor setup; a
		// linked host agent doesn't fix that setup, so the flag survives the
		// merge (the UI gate hides the notice once disk temps actually arrive).
		LegacySensorsFormat: proxyTemp.LegacySensorsFormat,
		LastUpdate:          hostAgentTemp.LastUpdate,
	}

	// Use host agent CPU data if available, fall back to proxy
	if !hostAgentTemp.HasCPU && proxyTemp.HasCPU {
		result.CPUPackage = proxyTemp.CPUPackage
		result.CPUMax = proxyTemp.CPUMax
		result.Cores = proxyTemp.Cores
		result.HasCPU = true
	}

	// Merge SMART data. Prefer host-agent SMART only when it carries usable
	// temperatures; otherwise let the SSH/proxy wrapper fill disk temps and keep
	// host-agent SMART inventory only when neither side has temperatures.
	switch {
	case hasUsableSMARTTemperature(hostAgentTemp):
		result.SMART = hostAgentTemp.SMART
	case hasUsableSMARTTemperature(proxyTemp):
		result.SMART = proxyTemp.SMART
	case hostAgentTemp.HasSMART:
		result.SMART = hostAgentTemp.SMART
	case proxyTemp.HasSMART:
		result.SMART = proxyTemp.SMART
	}
	result.HasSMART = len(result.SMART) > 0

	// Merge GPU data - prefer host agent if available
	if !hostAgentTemp.HasGPU && proxyTemp.HasGPU {
		result.GPU = proxyTemp.GPU
		result.HasGPU = true
	}

	// Merge NVMe data - prefer host agent if available, fall back to proxy
	if !hostAgentTemp.HasNVMe && proxyTemp.HasNVMe {
		result.NVMe = proxyTemp.NVMe
		result.HasNVMe = true
	}

	// Update historical max if current is higher
	currentTemp := result.CPUPackage
	if currentTemp == 0 && result.CPUMax > 0 {
		currentTemp = result.CPUMax
	}
	if currentTemp > result.CPUMaxRecord {
		result.CPUMaxRecord = currentTemp
		result.MaxRecorded = time.Now()
	}

	return result
}
