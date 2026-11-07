package models

// GetNodeNames returns the complete primary-state membership used by node-alert
// cleanup. Keep exact membership, including empty names and names shared across
// instances, just as the full snapshot did. No guest, backup or unrelated provider data is needed.
func (s *State) GetNodeNames() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make(map[string]bool, len(s.Nodes))
	for _, node := range s.Nodes {
		names[node.Name] = true
	}
	return names
}

// GetStorageForInstance returns the same ordered, normalized, detached storage
// rows as filtering GetSnapshot().Storage by exact Instance. This producer read
// must not use the unified-resource cache or copy another provider's inventory.
func (s *State) GetStorageForInstance(instance string) []Storage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	storage := []Storage{}
	for _, row := range s.Storage {
		if row.Instance == instance {
			storage = append(storage, cloneStorage(row))
		}
	}
	return storage
}
