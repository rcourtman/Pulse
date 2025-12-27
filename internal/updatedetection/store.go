package updatedetection

import (
	"sync"
	"time"
)

// Store manages the in-memory storage of update information.
// It provides thread-safe access to update data and supports
// queries by host, resource, or global listing.
type Store struct {
	mu         sync.RWMutex
	updates    map[string]*UpdateInfo // keyed by UpdateInfo.ID
	byHost     map[string][]string    // hostID -> []updateID
	byResource map[string]string      // resourceID -> updateID
}

// NewStore creates a new update store.
func NewStore() *Store {
	return &Store{
		updates:    make(map[string]*UpdateInfo),
		byHost:     make(map[string][]string),
		byResource: make(map[string]string),
	}
}

// UpsertUpdate adds or updates an update entry.
func (s *Store) UpsertUpdate(info *UpdateInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this is an update to an existing entry
	existing, exists := s.updates[info.ID]
	if exists {
		// Preserve FirstDetected from the original
		info.FirstDetected = existing.FirstDetected
	} else {
		// New entry, set FirstDetected if not already set
		if info.FirstDetected.IsZero() {
			info.FirstDetected = time.Now()
		}
	}

	// Update the main store
	s.updates[info.ID] = info

	// Update byResource index
	s.byResource[info.ResourceID] = info.ID

	// Update byHost index
	if !exists {
		s.byHost[info.HostID] = append(s.byHost[info.HostID], info.ID)
	}
}

// GetUpdatesForHost returns all updates for a specific host.
func (s *Store) GetUpdatesForHost(hostID string) []*UpdateInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	updateIDs := s.byHost[hostID]
	result := make([]*UpdateInfo, 0, len(updateIDs))
	for _, id := range updateIDs {
		if update, ok := s.updates[id]; ok {
			result = append(result, update)
		}
	}
	return result
}

// GetUpdatesForResource returns the update for a specific resource, if any.
func (s *Store) GetUpdatesForResource(resourceID string) *UpdateInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	updateID, ok := s.byResource[resourceID]
	if !ok {
		return nil
	}
	return s.updates[updateID]
}

// GetAllUpdates returns all tracked updates.
func (s *Store) GetAllUpdates() []*UpdateInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*UpdateInfo, 0, len(s.updates))
	for _, update := range s.updates {
		result = append(result, update)
	}
	return result
}

// DeleteUpdate removes an update entry by ID.
func (s *Store) DeleteUpdate(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	update, exists := s.updates[id]
	if !exists {
		return
	}

	// Remove from main store
	delete(s.updates, id)

	// Remove from byResource index
	delete(s.byResource, update.ResourceID)

	// Remove from byHost index
	hostUpdates := s.byHost[update.HostID]
	for i, updateID := range hostUpdates {
		if updateID == id {
			s.byHost[update.HostID] = append(hostUpdates[:i], hostUpdates[i+1:]...)
			break
		}
	}

	// Clean up empty host entries
	if len(s.byHost[update.HostID]) == 0 {
		delete(s.byHost, update.HostID)
	}
}

// DeleteUpdatesForResource removes any update associated with a resource.
func (s *Store) DeleteUpdatesForResource(resourceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updateID, ok := s.byResource[resourceID]
	if !ok {
		return
	}

	update := s.updates[updateID]
	if update == nil {
		delete(s.byResource, resourceID)
		return
	}

	// Remove from main store
	delete(s.updates, updateID)
	delete(s.byResource, resourceID)

	// Remove from byHost index
	hostUpdates := s.byHost[update.HostID]
	for i, id := range hostUpdates {
		if id == updateID {
			s.byHost[update.HostID] = append(hostUpdates[:i], hostUpdates[i+1:]...)
			break
		}
	}

	if len(s.byHost[update.HostID]) == 0 {
		delete(s.byHost, update.HostID)
	}
}

// DeleteUpdatesForHost removes all updates for a host.
func (s *Store) DeleteUpdatesForHost(hostID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updateIDs := s.byHost[hostID]
	for _, id := range updateIDs {
		if update := s.updates[id]; update != nil {
			delete(s.byResource, update.ResourceID)
		}
		delete(s.updates, id)
	}
	delete(s.byHost, hostID)
}

// GetSummary returns aggregated update statistics.
func (s *Store) GetSummary() map[string]*UpdateSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries := make(map[string]*UpdateSummary)

	for _, update := range s.updates {
		summary, exists := summaries[update.HostID]
		if !exists {
			summary = &UpdateSummary{
				HostID:   update.HostID,
				HostName: update.HostID, // Will be enriched by caller
			}
			summaries[update.HostID] = summary
		}

		summary.TotalUpdates++
		if update.LastChecked.After(summary.LastChecked) {
			summary.LastChecked = update.LastChecked
		}

		if update.Severity == SeveritySecurity {
			summary.SecurityUpdates++
		}

		switch update.Type {
		case UpdateTypeDockerImage, UpdateTypeKubernetesImage:
			summary.ContainerUpdates++
		case UpdateTypePackage, UpdateTypeProxmox:
			summary.PackageUpdates++
		}
	}

	return summaries
}

// Count returns the total number of tracked updates.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.updates)
}

// CountForHost returns the number of updates for a specific host.
func (s *Store) CountForHost(hostID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.byHost[hostID])
}
