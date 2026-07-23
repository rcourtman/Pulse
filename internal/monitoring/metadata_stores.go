package monitoring

import "github.com/rcourtman/pulse-go-rewrite/internal/config"

// GuestMetadataStore returns the live guest metadata store used by resource
// projection, migration, and monitoring decisions.
func (m *Monitor) GuestMetadataStore() *config.GuestMetadataStore {
	if m == nil {
		return nil
	}
	return m.guestMetadataStore
}

// DockerMetadataStore returns the live Docker metadata store used by resource
// projection, migration, and monitoring decisions.
func (m *Monitor) DockerMetadataStore() *config.DockerMetadataStore {
	if m == nil {
		return nil
	}
	return m.dockerMetadataStore
}

// HostMetadataStore returns the live host metadata store used by resource
// projection and monitoring decisions.
func (m *Monitor) HostMetadataStore() *config.HostMetadataStore {
	if m == nil {
		return nil
	}
	return m.hostMetadataStore
}
