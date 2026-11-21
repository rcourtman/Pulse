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

// GetPVEStorageBackupsForInstance returns detached primary storage rows in
// their existing order. The poller needs only this connection's archive
// evidence, not tasks, snapshots or other connections' archives.
func (s *State) GetPVEStorageBackupsForInstance(instance string) []StorageBackup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyBackupRows(s.PVEBackups.StorageBackups, func(row *StorageBackup) bool {
		return row.Instance == instance
	})
}

// GetPVEGuestSnapshotsForInstance returns detached primary snapshot rows in
// their existing order, without copying backup tasks or storage archives.
func (s *State) GetPVEGuestSnapshotsForInstance(instance string) []GuestSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyBackupRows(s.PVEBackups.GuestSnapshots, func(row *GuestSnapshot) bool {
		return row.Instance == instance
	})
}

// The callers hold the state read lock across both passes. Count first to
// avoid allocating for other connections or growing the result one row at a
// time on large single-connection estates. These callers use value-only rows.
func copyBackupRows[T any](rows []T, matches func(*T) bool) []T {
	count := 0
	for i := range rows {
		if matches(&rows[i]) {
			count++
		}
	}
	result := make([]T, 0, count)
	if count == len(rows) {
		return append(result, rows...)
	}
	if count > 0 {
		for i := range rows {
			if matches(&rows[i]) {
				result = append(result, rows[i])
			}
		}
	}
	return result
}

// GetBackupTasks returns the primary PVE tasks in their existing order, without
// copying backup artifacts or the resource inventory. BackupTask has only value
// fields; the returned slice is detached, including when it is empty.
func (s *State) GetBackupTasks() []BackupTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]BackupTask{}, s.PVEBackups.BackupTasks...)
}

// GetPVEBackupTasksForNodes returns detached primary task rows for the given
// connection and nodes, in stored order. A partial poll only needs evidence
// from its unread nodes, not from healthy nodes or peer connections.
func (s *State) GetPVEBackupTasksForNodes(instance string, nodes map[string]struct{}) []BackupTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyBackupRows(s.PVEBackups.BackupTasks, func(row *BackupTask) bool {
		if row.Instance != instance {
			return false
		}
		_, match := nodes[row.Node]
		return match
	})
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

// GetPBSBackupsForInstance returns ordered, detached primary rows for one
// connection. Select by exact instance before copying the file lists and
// verification metadata so cache preparation does no copy work for peers.
func (s *State) GetPBSBackupsForInstance(instance string) []PBSBackup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows := s.PBSBackups
	count := 0
	for i := range rows {
		if rows[i].Instance == instance {
			count++
		}
	}
	if count == 0 {
		return []PBSBackup{}
	}
	if count == len(rows) {
		return clonePBSBackups(rows)
	}
	result := make([]PBSBackup, 0, count)
	for i := range rows {
		if rows[i].Instance == instance {
			result = append(result, clonePBSBackup(rows[i]))
		}
	}
	return result
}

// GetPMGBackupsForInstance returns the same ordered primary backup rows as
// filtering GetSnapshot().PMGBackups by exact Instance. PMGBackup contains only
// value fields, so the returned slice is detached without copying unrelated
// gateways, backup sources or resource inventory. Keep empty reads non-nil.
func (s *State) GetPMGBackupsForInstance(instance string) []PMGBackup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	backups := []PMGBackup{}
	for _, backup := range s.PMGBackups {
		if backup.Instance == instance {
			backups = append(backups, backup)
		}
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
