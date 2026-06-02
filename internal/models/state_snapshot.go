package models

import (
	"strings"
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
	RemovedHosts                 []RemovedHost              `json:"removedHosts"`
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
	PVETagColors                 map[string]string          `json:"pveTagColors,omitempty"`
	// TemplateVMIDs holds Proxmox template VMID→node mappings per instance.
	// The presence of an instance key also indicates that template inventory was
	// successfully populated for that instance, even when the inner map is empty.
	// Not serialised to JSON or the frontend API; used only for backup orphan detection.
	TemplateVMIDs map[string]map[int]string `json:"-"`
}

// GetSnapshot returns a snapshot of the current state without mutex
func (s *State) GetSnapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pbsBackups := append([]PBSBackup{}, s.PBSBackups...)
	pmgBackups := append([]PMGBackup{}, s.PMGBackups...)
	pveBackups := PVEBackups{
		BackupTasks:    append([]BackupTask{}, s.PVEBackups.BackupTasks...),
		StorageBackups: append([]StorageBackup{}, s.PVEBackups.StorageBackups...),
		GuestSnapshots: append([]GuestSnapshot{}, s.PVEBackups.GuestSnapshots...),
	}

	// Create a snapshot without mutex
	snapshot := StateSnapshot{
		Nodes:                     append([]Node{}, s.Nodes...),
		VMs:                       append([]VM{}, s.VMs...),
		Containers:                append([]Container{}, s.Containers...),
		DockerHosts:               append([]DockerHost{}, s.DockerHosts...),
		RemovedDockerHosts:        append([]RemovedDockerHost{}, s.RemovedDockerHosts...),
		KubernetesClusters:        append([]KubernetesCluster{}, s.KubernetesClusters...),
		RemovedKubernetesClusters: append([]RemovedKubernetesCluster{}, s.RemovedKubernetesClusters...),
		RemovedHosts:              append([]RemovedHost{}, s.RemovedHosts...),
		Hosts:                     append([]Host{}, s.Hosts...),
		Storage:                   append([]Storage{}, s.Storage...),
		CephClusters:              append([]CephCluster{}, s.CephClusters...),
		PhysicalDisks:             append([]PhysicalDisk{}, s.PhysicalDisks...),
		PBSInstances:              append([]PBSInstance{}, s.PBSInstances...),
		PMGInstances:              append([]PMGInstance{}, s.PMGInstances...),
		PBSBackups:                pbsBackups,
		PMGBackups:                pmgBackups,
		Backups: Backups{
			PVE: pveBackups,
			PBS: pbsBackups,
			PMG: pmgBackups,
		},
		ReplicationJobs:              append([]ReplicationJob{}, s.ReplicationJobs...),
		Metrics:                      append([]Metric{}, s.Metrics...),
		PVEBackups:                   pveBackups,
		Performance:                  s.Performance,
		ConnectionHealth:             make(map[string]bool),
		Stats:                        s.Stats,
		ActiveAlerts:                 append([]Alert{}, s.ActiveAlerts...),
		RecentlyResolved:             append([]ResolvedAlert{}, s.RecentlyResolved...),
		LastUpdate:                   s.LastUpdate,
		TemperatureMonitoringEnabled: s.TemperatureMonitoringEnabled,
	}

	// Copy maps
	for k, v := range s.ConnectionHealth {
		snapshot.ConnectionHealth[k] = v
	}
	if len(s.PVETagColors) > 0 {
		snapshot.PVETagColors = make(map[string]string, len(s.PVETagColors))
		for k, v := range s.PVETagColors {
			snapshot.PVETagColors[k] = v
		}
	}
	if len(s.templateVMIDs) > 0 {
		snapshot.TemplateVMIDs = make(map[string]map[int]string, len(s.templateVMIDs))
		for instance, vmids := range s.templateVMIDs {
			vmap := make(map[int]string, len(vmids))
			for vmid, node := range vmids {
				vmap[vmid] = node
			}
			snapshot.TemplateVMIDs[instance] = vmap
		}
	}

	return snapshot
}

// MergeLinkedHostDisksIntoGuests supplements the filesystem listings of VMs
// and containers with disks reported by a unified pulse-agent running
// inside the same guest (matched via Host.LinkedVMID / LinkedContainerID).
//
// The qemu-guest-agent's get-fsinfo can miss filesystems on certain guest
// configurations (notably ZFS mounts on PBS — see #1438), but the unified
// pulse-agent has direct OS-level visibility through hostmetrics. When a
// Host is linked to a guest, this method appends the host agent's Disks
// (deduped by mountpoint) to the guest's Disks slice and updates the
// guest's aggregate Disk usage to include the new partitions.
//
// Disks already present on the guest from the qemu-guest-agent path take
// precedence; we only add host-agent entries for mountpoints the guest
// doesn't already list.
func (s *StateSnapshot) MergeLinkedHostDisksIntoGuests() {
	if s == nil || len(s.Hosts) == 0 {
		return
	}

	hostByVMID := make(map[string]*Host)
	hostByContainerID := make(map[string]*Host)
	for i := range s.Hosts {
		h := &s.Hosts[i]
		if id := strings.TrimSpace(h.LinkedVMID); id != "" {
			hostByVMID[id] = h
		}
		if id := strings.TrimSpace(h.LinkedContainerID); id != "" {
			hostByContainerID[id] = h
		}
	}

	if len(hostByVMID) > 0 {
		for i := range s.VMs {
			mergeHostDisksIntoGuest(&s.VMs[i].Disk, &s.VMs[i].Disks, hostByVMID[s.VMs[i].ID])
		}
	}
	if len(hostByContainerID) > 0 {
		for i := range s.Containers {
			mergeHostDisksIntoGuest(&s.Containers[i].Disk, &s.Containers[i].Disks, hostByContainerID[s.Containers[i].ID])
		}
	}
}

// mergeHostDisksIntoGuest is the shared body of MergeLinkedHostDisksIntoGuests
// for a single VM or container. It mutates *guestDisks and *guestAggregate in
// place when the linked host reports filesystems the guest does not.
func mergeHostDisksIntoGuest(guestAggregate *Disk, guestDisks *[]Disk, host *Host) {
	if host == nil || len(host.Disks) == 0 || guestDisks == nil {
		return
	}

	existingMounts := make(map[string]struct{}, len(*guestDisks))
	for _, d := range *guestDisks {
		mp := strings.TrimSpace(d.Mountpoint)
		if mp != "" {
			existingMounts[mp] = struct{}{}
		}
	}

	var added []Disk
	for _, d := range host.Disks {
		mp := strings.TrimSpace(d.Mountpoint)
		if mp == "" {
			continue
		}
		if _, dup := existingMounts[mp]; dup {
			continue
		}
		existingMounts[mp] = struct{}{}
		added = append(added, d)
	}

	if len(added) == 0 {
		return
	}

	merged := make([]Disk, 0, len(*guestDisks)+len(added))
	merged = append(merged, *guestDisks...)
	merged = append(merged, added...)
	*guestDisks = merged

	if guestAggregate == nil {
		return
	}
	for _, d := range added {
		if d.Total > 0 {
			guestAggregate.Total += d.Total
			guestAggregate.Used += d.Used
			guestAggregate.Free += d.Free
		}
	}
	if guestAggregate.Total > 0 {
		guestAggregate.Usage = float64(guestAggregate.Used) / float64(guestAggregate.Total) * 100
	}
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

	removedHosts := make([]RemovedHostFrontend, len(s.RemovedHosts))
	for i, entry := range s.RemovedHosts {
		removedHosts[i] = entry.ToFrontend()
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

	// Convert Ceph clusters - collapse the same physical cluster reported by
	// more than one source (Proxmox API + host-agent) to a single deterministic
	// identity so the UI and alert evaluation agree on one pool ID (#1341).
	dedupedCeph := DedupeCephClusters(s.CephClusters)
	cephClusters := make([]CephClusterFrontend, 0, len(dedupedCeph))
	for _, cluster := range dedupedCeph {
		cephClusters = append(cephClusters, cluster.ToFrontend())
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
		RemovedHosts:                 removedHosts,
		Hosts:                        hosts,
		Storage:                      storage,
		CephClusters:                 cephClusters,
		PhysicalDisks:                s.PhysicalDisks,
		PBS:                          s.PBSInstances,
		PMG:                          s.PMGInstances,
		PBSBackups:                   s.PBSBackups,
		PMGBackups:                   s.PMGBackups,
		Backups:                      s.Backups,
		ReplicationJobs:              replicationJobs,
		ActiveAlerts:                 s.ActiveAlerts,
		Metrics:                      make(map[string]any),
		PVEBackups:                   s.PVEBackups,
		Performance:                  make(map[string]any),
		ConnectionHealth:             s.ConnectionHealth,
		Stats:                        make(map[string]any),
		LastUpdate:                   s.LastUpdate.Unix() * 1000, // JavaScript timestamp
		TemperatureMonitoringEnabled: s.TemperatureMonitoringEnabled,
		PVETagColors:                 s.PVETagColors,
	}
}
