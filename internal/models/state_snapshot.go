package models

import (
	"encoding/json"
	"time"
)

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

	// Preserve non-nil slice semantics from NewState() — cloneX functions return nil
	// for empty input, but callers rely on non-nil to distinguish "empty" from "never set".
	if snapshot.Nodes == nil {
		snapshot.Nodes = []Node{}
	}
	if snapshot.VMs == nil {
		snapshot.VMs = []VM{}
	}
	if snapshot.Containers == nil {
		snapshot.Containers = []Container{}
	}
	if snapshot.DockerHosts == nil {
		snapshot.DockerHosts = []DockerHost{}
	}
	if snapshot.Storage == nil {
		snapshot.Storage = []Storage{}
	}
	if snapshot.ActiveAlerts == nil {
		snapshot.ActiveAlerts = []Alert{}
	}

	// Copy map
	for k, v := range s.ConnectionHealth {
		snapshot.ConnectionHealth[k] = v
	}

	return snapshot
}

// ResourceLocation describes where a resource lives in the infrastructure hierarchy.
// This is the authoritative source of truth for routing commands to resources.
type ResourceLocation struct {
	// What was found
	Found        bool   // True if the resource was found
	Name         string // The resource name
	ResourceType string // "node", "vm", "lxc", "dockerhost", "docker", "host", "k8s_cluster", "k8s_pod", "k8s_deployment"

	// For VMs and LXCs (Proxmox)
	VMID int    // VMID if this is a VM or LXC
	Node string // Proxmox node name

	// For Docker/Podman containers
	DockerHostName string // Name of the Docker host (LXC/VM/standalone)
	DockerHostType string // "lxc", "vm", or "standalone"
	DockerHostVMID int    // VMID if Docker host is an LXC/VM

	// For Kubernetes resources
	K8sClusterName string // Kubernetes cluster name
	K8sNamespace   string // Kubernetes namespace
	K8sAgentID     string // Agent ID for routing kubectl commands

	// For generic hosts (Windows/Linux via Pulse Unified Agent)
	HostID   string // Host ID
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

	// Check LXC containers
	for _, lxc := range s.Containers {
		if lxc.Name == name {
			return ResourceLocation{
				Found:        true,
				Name:         name,
				ResourceType: "lxc",
				VMID:         lxc.VMID,
				Node:         lxc.Node,
				TargetHost:   lxc.Name, // Route to LXC by name
			}
		}
	}

	// Check Docker hosts (LXCs/VMs/standalone hosts running Docker)
	for _, dh := range s.DockerHosts {
		if dh.Hostname == name || dh.ID == name {
			loc := ResourceLocation{
				Found:          true,
				Name:           dh.Hostname,
				ResourceType:   "dockerhost",
				DockerHostName: dh.Hostname,
				TargetHost:     dh.Hostname,
			}
			// Check if this Docker host is an LXC
			for _, lxc := range s.Containers {
				if lxc.Name == dh.Hostname || lxc.Name == dh.ID {
					loc.DockerHostType = "lxc"
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
					ResourceType:   "docker",
					DockerHostName: dh.Hostname,
					TargetHost:     dh.Hostname, // Route to the Docker host, not the container
				}
				// Resolve the Docker host's parent (LXC/VM/standalone)
				for _, lxc := range s.Containers {
					if lxc.Name == dh.Hostname || lxc.Name == dh.ID {
						loc.DockerHostType = "lxc"
						loc.DockerHostVMID = lxc.VMID
						loc.Node = lxc.Node
						loc.TargetHost = lxc.Name // Route to the LXC
						break
					}
				}
				if loc.DockerHostType == "" {
					for _, vm := range s.VMs {
						if vm.Name == dh.Hostname || vm.Name == dh.ID {
							loc.DockerHostType = "vm"
							loc.DockerHostVMID = vm.VMID
							loc.Node = vm.Node
							loc.TargetHost = vm.Name // Route to the VM
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
				ResourceType: "host",
				HostID:       host.ID,
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
				ResourceType:   "k8s_cluster",
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
					ResourceType:   "k8s_pod",
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
					ResourceType:   "k8s_deployment",
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
	// Convert nodes
	nodes := make([]NodeFrontend, len(s.Nodes))
	for i, n := range s.Nodes {
		nodes[i] = n.ToFrontend()
	}

	// Convert VMs
	vms := make([]VMFrontend, len(s.VMs))
	for i, v := range s.VMs {
		vms[i] = v.ToFrontend()
	}

	// Convert containers
	containers := make([]ContainerFrontend, len(s.Containers))
	for i, c := range s.Containers {
		containers[i] = c.ToFrontend()
	}

	dockerHosts := make([]DockerHostFrontend, len(s.DockerHosts))
	for i, host := range s.DockerHosts {
		dockerHosts[i] = host.ToFrontend()
	}

	removedDockerHosts := make([]RemovedDockerHostFrontend, len(s.RemovedDockerHosts))
	for i, entry := range s.RemovedDockerHosts {
		removedDockerHosts[i] = entry.ToFrontend()
	}

	kubernetesClusters := make([]KubernetesClusterFrontend, len(s.KubernetesClusters))
	for i, cluster := range s.KubernetesClusters {
		kubernetesClusters[i] = cluster.ToFrontend()
	}

	removedKubernetesClusters := make([]RemovedKubernetesClusterFrontend, len(s.RemovedKubernetesClusters))
	for i, entry := range s.RemovedKubernetesClusters {
		removedKubernetesClusters[i] = entry.ToFrontend()
	}

	hosts := make([]HostFrontend, len(s.Hosts))
	for i, host := range s.Hosts {
		hosts[i] = host.ToFrontend()
	}

	// Convert storage
	storage := make([]StorageFrontend, len(s.Storage))
	for i, st := range s.Storage {
		storage[i] = st.ToFrontend()
	}

	replicationJobs := make([]ReplicationJobFrontend, len(s.ReplicationJobs))
	for i, job := range s.ReplicationJobs {
		replicationJobs[i] = job.ToFrontend()
	}

	return StateFrontend{
		Nodes:                        nodes,
		VMs:                          vms,
		Containers:                   containers,
		DockerHosts:                  dockerHosts,
		RemovedDockerHosts:           removedDockerHosts,
		KubernetesClusters:           kubernetesClusters,
		RemovedKubernetesClusters:    removedKubernetesClusters,
		Hosts:                        hosts,
		Storage:                      storage,
		PBS:                          s.PBSInstances,
		PMG:                          s.PMGInstances,
		ReplicationJobs:              replicationJobs,
		ActiveAlerts:                 s.ActiveAlerts,
		Metrics:                      make(map[string]json.RawMessage),
		Performance:                  make(map[string]json.RawMessage),
		ConnectionHealth:             s.ConnectionHealth,
		Stats:                        make(map[string]json.RawMessage),
		LastUpdate:                   s.LastUpdate.Unix() * 1000, // JavaScript timestamp
		TemperatureMonitoringEnabled: s.TemperatureMonitoringEnabled,
	}
}
