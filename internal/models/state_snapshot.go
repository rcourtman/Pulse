package models

import "time"

// StateSnapshot represents a snapshot of the state without mutex
type StateSnapshot struct {
	Nodes            []Node          `json:"nodes"`
	VMs              []VM            `json:"vms"`
	Containers       []Container     `json:"containers"`
	DockerHosts      []DockerHost    `json:"dockerHosts"`
	Storage          []Storage       `json:"storage"`
	CephClusters     []CephCluster   `json:"cephClusters"`
	PhysicalDisks    []PhysicalDisk  `json:"physicalDisks"`
	PBSInstances     []PBSInstance   `json:"pbs"`
	PMGInstances     []PMGInstance   `json:"pmg"`
	PBSBackups       []PBSBackup     `json:"pbsBackups"`
	Metrics          []Metric        `json:"metrics"`
	PVEBackups       PVEBackups      `json:"pveBackups"`
	Performance      Performance     `json:"performance"`
	ConnectionHealth map[string]bool `json:"connectionHealth"`
	Stats            Stats           `json:"stats"`
	ActiveAlerts     []Alert         `json:"activeAlerts"`
	RecentlyResolved []ResolvedAlert `json:"recentlyResolved"`
	LastUpdate       time.Time       `json:"lastUpdate"`
}

// GetSnapshot returns a snapshot of the current state without mutex
func (s *State) GetSnapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a snapshot without mutex
	snapshot := StateSnapshot{
		Nodes:         append([]Node{}, s.Nodes...),
		VMs:           append([]VM{}, s.VMs...),
		Containers:    append([]Container{}, s.Containers...),
		DockerHosts:   append([]DockerHost{}, s.DockerHosts...),
		Storage:       append([]Storage{}, s.Storage...),
		CephClusters:  append([]CephCluster{}, s.CephClusters...),
		PhysicalDisks: append([]PhysicalDisk{}, s.PhysicalDisks...),
		PBSInstances:  append([]PBSInstance{}, s.PBSInstances...),
		PMGInstances:  append([]PMGInstance{}, s.PMGInstances...),
		PBSBackups:    append([]PBSBackup{}, s.PBSBackups...),
		Metrics:       append([]Metric{}, s.Metrics...),
		PVEBackups: PVEBackups{
			BackupTasks:    append([]BackupTask{}, s.PVEBackups.BackupTasks...),
			StorageBackups: append([]StorageBackup{}, s.PVEBackups.StorageBackups...),
			GuestSnapshots: append([]GuestSnapshot{}, s.PVEBackups.GuestSnapshots...),
		},
		Performance:      s.Performance,
		ConnectionHealth: make(map[string]bool),
		Stats:            s.Stats,
		ActiveAlerts:     append([]Alert{}, s.ActiveAlerts...),
		RecentlyResolved: append([]ResolvedAlert{}, s.RecentlyResolved...),
		LastUpdate:       s.LastUpdate,
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

	return StateFrontend{
		Nodes:            nodes,
		VMs:              vms,
		Containers:       containers,
		DockerHosts:      dockerHosts,
		Storage:          storage,
		CephClusters:     cephClusters,
		PhysicalDisks:    s.PhysicalDisks,
		PBS:              s.PBSInstances,
		PMG:              s.PMGInstances,
		ActiveAlerts:     s.ActiveAlerts,
		Metrics:          make(map[string]any),
		PVEBackups:       s.PVEBackups,
		Performance:      make(map[string]any),
		ConnectionHealth: s.ConnectionHealth,
		Stats:            make(map[string]any),
		LastUpdate:       s.LastUpdate.Unix() * 1000, // JavaScript timestamp
	}
}
