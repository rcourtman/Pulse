package models

import "time"

// StateSnapshot represents a snapshot of the state without mutex
type StateSnapshot struct {
	Nodes            []Node            `json:"nodes"`
	VMs              []VM              `json:"vms"`
	Containers       []Container       `json:"containers"`
	Storage          []Storage         `json:"storage"`
	PBSInstances     []PBSInstance     `json:"pbs"`
	PBSBackups       []PBSBackup       `json:"pbsBackups"`
	Metrics          []Metric          `json:"metrics"`
	PVEBackups       PVEBackups        `json:"pveBackups"`
	Performance      Performance       `json:"performance"`
	ConnectionHealth map[string]bool   `json:"connectionHealth"`
	Stats            Stats             `json:"stats"`
	ActiveAlerts     []Alert           `json:"activeAlerts"`
	RecentlyResolved []ResolvedAlert   `json:"recentlyResolved"`
	LastUpdate       time.Time         `json:"lastUpdate"`
}

// GetSnapshot returns a snapshot of the current state without mutex
func (s *State) GetSnapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Create a snapshot without mutex
	snapshot := StateSnapshot{
		Nodes:            append([]Node{}, s.Nodes...),
		VMs:              append([]VM{}, s.VMs...),
		Containers:       append([]Container{}, s.Containers...),
		Storage:          append([]Storage{}, s.Storage...),
		PBSInstances:     append([]PBSInstance{}, s.PBSInstances...),
		PBSBackups:       append([]PBSBackup{}, s.PBSBackups...),
		Metrics:          append([]Metric{}, s.Metrics...),
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