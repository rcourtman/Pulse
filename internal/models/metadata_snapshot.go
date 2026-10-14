package models

// These projections serve backup/job/connection consumers that do not need a
// resource inventory. Clone only their primary fields under the same state
// lock as GetSnapshot; do not read the separately cached Backups aggregate.
// Keep the complete snapshot's normalization and caller-owned collections.
func (s *State) GetBackups() Backups {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Backups{
		PVE: clonePVEBackups(s.PVEBackups),
		PBS: clonePBSBackups(s.PBSBackups),
		PMG: clonePMGBackups(s.PMGBackups),
	}.NormalizeCollections()
}

func (s *State) GetPVEBackups() PVEBackups {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return clonePVEBackups(s.PVEBackups)
}

func (s *State) GetPBSBackups() []PBSBackup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	backups := clonePBSBackups(s.PBSBackups)
	if backups == nil {
		backups = []PBSBackup{}
	}
	return backups
}

func (s *State) GetReplicationJobs() []ReplicationJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := cloneReplicationJobs(s.ReplicationJobs)
	if jobs == nil {
		jobs = []ReplicationJob{}
	}
	return jobs
}

func (s *State) GetConnectionHealth() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	health := make(map[string]bool, len(s.ConnectionHealth))
	for key, healthy := range s.ConnectionHealth {
		health[key] = healthy
	}
	return health
}
