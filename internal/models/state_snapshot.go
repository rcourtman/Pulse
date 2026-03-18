package models

import "time"

// StateSnapshot represents a snapshot of the state without mutex
type StateSnapshot struct {
	Nodes                        []Node                     `json:"nodes"`
	VMs                          []VM                       `json:"vms"`
	Containers                   []Container                `json:"containers"`
	DockerHosts                  []DockerHost               `json:"dockerHosts"`
	RemovedDockerHosts           []RemovedDockerHost        `json:"removedDockerHosts"`
	KubernetesClusters           []KubernetesCluster        `json:"kubernetesClusters"`
	RemovedKubernetesClusters    []RemovedKubernetesCluster `json:"removedKubernetesClusters"`
	Hosts                        []Host                     `json:"hosts"`
	RemovedHostAgents            []RemovedHostAgent         `json:"removedHostAgents"`
	Storage                      []Storage                  `json:"storage"`
	CephClusters                 []CephCluster              `json:"cephClusters"`
	PhysicalDisks                []PhysicalDisk             `json:"physicalDisks"`
	PBSInstances                 []PBSInstance              `json:"pbs"`
	PMGInstances                 []PMGInstance              `json:"pmg"`
	PBSBackups                   []PBSBackup                `json:"pbsBackups"`
	PMGBackups                   []PMGBackup                `json:"pmgBackups"`
	Backups                      Backups                    `json:"backups"`
	ReplicationJobs              []ReplicationJob           `json:"replicationJobs"`
	Metrics                      []Metric                   `json:"metrics"`
	PVEBackups                   PVEBackups                 `json:"pveBackups"`
	Performance                  Performance                `json:"performance"`
	ConnectionHealth             map[string]bool            `json:"connectionHealth"`
	Stats                        Stats                      `json:"stats"`
	ActiveAlerts                 []Alert                    `json:"activeAlerts"`
	RecentlyResolved             []ResolvedAlert            `json:"recentlyResolved"`
	LastUpdate                   time.Time                  `json:"lastUpdate"`
	TemperatureMonitoringEnabled bool                       `json:"temperatureMonitoringEnabled"`
}

// EmptyStateSnapshot returns a normalized zero-value snapshot for runtime
// paths that need to fail closed without leaking nil collection/map fields.
func EmptyStateSnapshot() StateSnapshot {
	snapshot := StateSnapshot{}
	snapshot.NormalizeCollections()
	return snapshot
}

// NormalizeCollections preserves stable empty-slice semantics for snapshot
// collection fields so downstream callers do not have to distinguish nil from
// "present but empty".
func (s *StateSnapshot) NormalizeCollections() {
	if s.Nodes == nil {
		s.Nodes = []Node{}
	}
	for i := range s.Nodes {
		s.Nodes[i] = s.Nodes[i].NormalizeCollections()
	}
	if s.VMs == nil {
		s.VMs = []VM{}
	}
	for i := range s.VMs {
		s.VMs[i] = s.VMs[i].NormalizeCollections()
	}
	if s.Containers == nil {
		s.Containers = []Container{}
	}
	for i := range s.Containers {
		s.Containers[i] = s.Containers[i].NormalizeCollections()
	}
	if s.DockerHosts == nil {
		s.DockerHosts = []DockerHost{}
	}
	for i := range s.DockerHosts {
		s.DockerHosts[i] = s.DockerHosts[i].NormalizeCollections()
	}
	if s.RemovedDockerHosts == nil {
		s.RemovedDockerHosts = []RemovedDockerHost{}
	}
	if s.KubernetesClusters == nil {
		s.KubernetesClusters = []KubernetesCluster{}
	}
	for i := range s.KubernetesClusters {
		s.KubernetesClusters[i] = s.KubernetesClusters[i].NormalizeCollections()
	}
	if s.RemovedKubernetesClusters == nil {
		s.RemovedKubernetesClusters = []RemovedKubernetesCluster{}
	}
	if s.Hosts == nil {
		s.Hosts = []Host{}
	}
	for i := range s.Hosts {
		s.Hosts[i] = s.Hosts[i].NormalizeCollections()
	}
	if s.RemovedHostAgents == nil {
		s.RemovedHostAgents = []RemovedHostAgent{}
	}
	if s.Storage == nil {
		s.Storage = []Storage{}
	}
	for i := range s.Storage {
		s.Storage[i] = s.Storage[i].NormalizeCollections()
	}
	if s.CephClusters == nil {
		s.CephClusters = []CephCluster{}
	}
	for i := range s.CephClusters {
		s.CephClusters[i] = s.CephClusters[i].NormalizeCollections()
	}
	if s.PhysicalDisks == nil {
		s.PhysicalDisks = []PhysicalDisk{}
	}
	if s.PBSInstances == nil {
		s.PBSInstances = []PBSInstance{}
	}
	for i := range s.PBSInstances {
		s.PBSInstances[i] = s.PBSInstances[i].NormalizeCollections()
	}
	if s.PMGInstances == nil {
		s.PMGInstances = []PMGInstance{}
	}
	for i := range s.PMGInstances {
		s.PMGInstances[i] = s.PMGInstances[i].NormalizeCollections()
	}
	if s.PBSBackups == nil {
		s.PBSBackups = []PBSBackup{}
	}
	for i := range s.PBSBackups {
		s.PBSBackups[i] = s.PBSBackups[i].NormalizeCollections()
	}
	if s.PMGBackups == nil {
		s.PMGBackups = []PMGBackup{}
	}
	if s.ReplicationJobs == nil {
		s.ReplicationJobs = []ReplicationJob{}
	}
	if s.Metrics == nil {
		s.Metrics = []Metric{}
	}
	for i := range s.Metrics {
		s.Metrics[i] = s.Metrics[i].NormalizeCollections()
	}
	if s.ActiveAlerts == nil {
		s.ActiveAlerts = []Alert{}
	}
	if s.RecentlyResolved == nil {
		s.RecentlyResolved = []ResolvedAlert{}
	}
	s.PVEBackups = s.PVEBackups.NormalizeCollections()
	s.Backups = s.Backups.NormalizeCollections()
	if s.ConnectionHealth == nil {
		s.ConnectionHealth = map[string]bool{}
	}
	s.Performance = s.Performance.NormalizeCollections()
}

// GetSnapshot returns a snapshot of the current state without mutex
func (s *State) GetSnapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pbsBackups := clonePBSBackups(s.PBSBackups)
	pmgBackups := clonePMGBackups(s.PMGBackups)
	pveBackups := clonePVEBackups(s.PVEBackups)

	// Create a snapshot without mutex
	snapshot := StateSnapshot{
		Nodes:                     cloneNodes(s.Nodes),
		VMs:                       cloneVMs(s.VMs),
		Containers:                cloneContainers(s.Containers),
		DockerHosts:               cloneDockerHosts(s.DockerHosts),
		RemovedDockerHosts:        append([]RemovedDockerHost(nil), s.RemovedDockerHosts...),
		KubernetesClusters:        cloneKubernetesClusters(s.KubernetesClusters),
		RemovedKubernetesClusters: append([]RemovedKubernetesCluster(nil), s.RemovedKubernetesClusters...),
		Hosts:                     cloneHosts(s.Hosts),
		RemovedHostAgents:         append([]RemovedHostAgent(nil), s.RemovedHostAgents...),
		Storage:                   cloneStorages(s.Storage),
		CephClusters:              cloneCephClusters(s.CephClusters),
		PhysicalDisks:             clonePhysicalDisks(s.PhysicalDisks),
		PBSInstances:              clonePBSInstances(s.PBSInstances),
		PMGInstances:              clonePMGInstances(s.PMGInstances),
		PBSBackups:                pbsBackups,
		PMGBackups:                pmgBackups,
		Backups: Backups{
			PVE: pveBackups,
			PBS: pbsBackups,
			PMG: pmgBackups,
		},
		ReplicationJobs:              cloneReplicationJobs(s.ReplicationJobs),
		Metrics:                      cloneMetrics(s.Metrics),
		PVEBackups:                   pveBackups,
		Performance:                  clonePerformance(s.Performance),
		ConnectionHealth:             make(map[string]bool),
		Stats:                        s.Stats,
		ActiveAlerts:                 cloneAlerts(s.ActiveAlerts),
		RecentlyResolved:             cloneResolvedAlerts(s.RecentlyResolved),
		LastUpdate:                   s.LastUpdate,
		TemperatureMonitoringEnabled: s.TemperatureMonitoringEnabled,
	}

	snapshot.NormalizeCollections()

	// Copy map
	for k, v := range s.ConnectionHealth {
		snapshot.ConnectionHealth[k] = v
	}

	return snapshot
}

// GetLastUpdate returns the current state freshness marker without cloning the
// full snapshot payload.
func (s *State) GetLastUpdate() time.Time {
	if s == nil {
		return time.Time{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastUpdate
}

// ResourceLocation describes where a resource lives in the infrastructure hierarchy.
// This is the authoritative source of truth for routing commands to resources.
type ResourceLocation struct {
	// What was found
	Found        bool   // True if the resource was found
	Name         string // The resource name
	ResourceType string // "node", "vm", "system-container", "docker-host", "app-container", "agent", "k8s-cluster", "k8s-pod", "k8s-deployment"

	// For VMs and system containers (guest resources on a hypervisor node)
	VMID int    // Guest ID (VMID) if this is a VM or system container
	Node string // Hypervisor node name (e.g., Proxmox node)

	// For Docker/Podman containers
	DockerHostName string // Name of the Docker host (system-container/VM/standalone)
	DockerHostType string // "system-container", "vm", or "standalone"
	DockerHostVMID int    // Guest ID (VMID) if Docker host is a system container or VM

	// For Kubernetes resources
	K8sClusterName string // Kubernetes cluster name
	K8sNamespace   string // Kubernetes namespace
	K8sAgentID     string // Agent ID for routing kubectl commands

	// For generic hosts (Windows/Linux via Pulse Unified Agent)
	TargetID string // Canonical target ID (agent ID for host resources)
	Platform string // "linux", "windows", etc.

	// The key output: where to route commands
	TargetHost string // The target_host to use for pulse_control/pulse_file_edit
	AgentID    string // Direct agent ID if known (for K8s, standalone hosts)
}

// ResolveResource looks up a resource by name and returns its location in the hierarchy.
// This is the single source of truth for determining where any resource lives.
func (s StateSnapshot) ResolveResource(name string) ResourceLocation {
	// Check Proxmox nodes first
	for _, node := range s.Nodes {
		if node.Name == name {
			return ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "node",
				Node:         node.Name,
				TargetHost:   node.Name,
			}
		}
	}

	// Check VMs
	for _, vm := range s.VMs {
		if vm.Name == name {
			return ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "vm",
				VMID:         vm.VMID,
				Node:         vm.Node,
				TargetHost:   vm.Name, // Route to VM by name
			}
		}
	}

	// Check system containers (LXC/Incus)
	for _, lxc := range s.Containers {
		if lxc.Name == name {
			return ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "system-container",
				VMID:         lxc.VMID,
				Node:         lxc.Node,
				TargetHost:   lxc.Name,
			}
		}
	}

	// Check Docker hosts (system-containers/VMs/standalone hosts running Docker)
	for _, dh := range s.DockerHosts {
		if dh.Hostname == name || dh.ID == name {
			loc := ResourceLocation{
				Found:          true,
				Name:           dh.Hostname,
				ResourceType:   "docker-host",
				DockerHostName: dh.Hostname,
				TargetHost:     dh.Hostname,
			}
			// Check if this Docker host is a system container
			for _, lxc := range s.Containers {
				if lxc.Name == dh.Hostname || lxc.Name == dh.ID {
					loc.DockerHostType = "system-container"
					loc.DockerHostVMID = lxc.VMID
					loc.Node = lxc.Node
					break
				}
			}
			// Check if this Docker host is a VM
			if loc.DockerHostType == "" {
				for _, vm := range s.VMs {
					if vm.Name == dh.Hostname || vm.Name == dh.ID {
						loc.DockerHostType = "vm"
						loc.DockerHostVMID = vm.VMID
						loc.Node = vm.Node
						break
					}
				}
			}
			if loc.DockerHostType == "" {
				loc.DockerHostType = "standalone"
			}
			return loc
		}
	}

	// Check Docker containers - this is the critical path for "homepage" -> "homepage-docker"
	for _, dh := range s.DockerHosts {
		for _, container := range dh.Containers {
			if container.Name == name {
				loc := ResourceLocation{
					Found:          true,
					Name:           name,
					ResourceType:   "app-container",
					DockerHostName: dh.Hostname,
					TargetHost:     dh.Hostname, // Route to the Docker host, not the container
				}
				// Resolve the Docker host's parent (system-container/VM/standalone)
				for _, lxc := range s.Containers {
					if lxc.Name == dh.Hostname || lxc.Name == dh.ID {
						loc.DockerHostType = "system-container"
						loc.DockerHostVMID = lxc.VMID
						loc.Node = lxc.Node
						loc.TargetHost = lxc.Name
						break
					}
				}
				if loc.DockerHostType == "" {
					for _, vm := range s.VMs {
						if vm.Name == dh.Hostname || vm.Name == dh.ID {
							loc.DockerHostType = "vm"
							loc.DockerHostVMID = vm.VMID
							loc.Node = vm.Node
							loc.TargetHost = vm.Name
							break
						}
					}
				}
				if loc.DockerHostType == "" {
					loc.DockerHostType = "standalone"
				}
				return loc
			}
		}
	}

	// Check generic Hosts (Windows/Linux via Pulse Unified Agent)
	for _, host := range s.Hosts {
		if host.Hostname == name || host.ID == name {
			return ResourceLocation{
				Found:        true,
				Name:         host.Hostname,
				ResourceType: "agent",
				TargetID:     host.ID,
				Platform:     host.Platform,
				TargetHost:   host.Hostname,
			}
		}
	}

	// Check Kubernetes clusters, pods, and deployments
	for _, cluster := range s.KubernetesClusters {
		if cluster.Name == name || cluster.ID == name || cluster.DisplayName == name {
			return ResourceLocation{
				Found:          true,
				Name:           cluster.Name,
				ResourceType:   "k8s-cluster",
				K8sClusterName: cluster.Name,
				K8sAgentID:     cluster.AgentID,
				TargetHost:     cluster.Name,
				AgentID:        cluster.AgentID,
			}
		}

		// Check pods within this cluster
		for _, pod := range cluster.Pods {
			if pod.Name == name {
				return ResourceLocation{
					Found:          true,
					Name:           pod.Name,
					ResourceType:   "k8s-pod",
					K8sClusterName: cluster.Name,
					K8sNamespace:   pod.Namespace,
					K8sAgentID:     cluster.AgentID,
					TargetHost:     cluster.Name,
					AgentID:        cluster.AgentID,
				}
			}
		}

		// Check deployments within this cluster
		for _, deploy := range cluster.Deployments {
			if deploy.Name == name {
				return ResourceLocation{
					Found:          true,
					Name:           deploy.Name,
					ResourceType:   "k8s-deployment",
					K8sClusterName: cluster.Name,
					K8sNamespace:   deploy.Namespace,
					K8sAgentID:     cluster.AgentID,
					TargetHost:     cluster.Name,
					AgentID:        cluster.AgentID,
				}
			}
		}
	}

	return ResourceLocation{Found: false, Name: name}
}

// ToFrontend converts a StateSnapshot to frontend format with proper tag handling
func (s StateSnapshot) ToFrontend() StateFrontend {
	frontend := EmptyStateFrontend()
	frontend.ActiveAlerts = append([]Alert{}, s.ActiveAlerts...)
	frontend.RecentlyResolved = append([]ResolvedAlert{}, s.RecentlyResolved...)
	frontend.Metrics = append([]Metric{}, s.Metrics...)
	frontend.Performance = s.Performance
	frontend.ConnectionHealth = make(map[string]bool, len(s.ConnectionHealth))
	for key, value := range s.ConnectionHealth {
		frontend.ConnectionHealth[key] = value
	}
	frontend.Stats = s.Stats
	frontend.LastUpdate = s.LastUpdate.Unix() * 1000 // JavaScript timestamp
	frontend.TemperatureMonitoringEnabled = s.TemperatureMonitoringEnabled
	return frontend.NormalizeCollections()
}
