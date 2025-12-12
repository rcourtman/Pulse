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

	// Copy map
	for k, v := range s.ConnectionHealth {
		snapshot.ConnectionHealth[k] = v
	}

	return snapshot
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

	// Convert Ceph clusters
	cephClusters := make([]CephClusterFrontend, len(s.CephClusters))
	for i, cluster := range s.CephClusters {
		cephClusters[i] = cluster.ToFrontend()
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
	}
}
