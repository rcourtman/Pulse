package api

import (
	"fmt"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// MetadataProviderImpl implements ai.MetadataProvider to allow AI to update resource URLs
type MetadataProviderImpl struct {
	guestStore  *config.GuestMetadataStore
	dockerStore *config.DockerMetadataStore
	hostStore   *config.HostMetadataStore
}

// NewMetadataProvider creates a new MetadataProvider with the given stores
func NewMetadataProvider(
	guestStore *config.GuestMetadataStore,
	dockerStore *config.DockerMetadataStore,
	hostStore *config.HostMetadataStore,
) *MetadataProviderImpl {
	return &MetadataProviderImpl{
		guestStore:  guestStore,
		dockerStore: dockerStore,
		hostStore:   hostStore,
	}
}

// SetGuestURL sets the custom URL for a Proxmox guest (VM/container)
func (m *MetadataProviderImpl) SetGuestURL(guestID, customURL string) error {
	if m.guestStore == nil {
		return fmt.Errorf("guest metadata store not available")
	}

	// Get existing metadata or create new
	existing := m.guestStore.Get(guestID)
	if existing == nil {
		existing = &config.GuestMetadata{
			ID: guestID,
		}
	}

	// Update the URL
	existing.CustomURL = customURL

	return m.guestStore.Set(guestID, existing)
}

// SetDockerURL sets the custom URL for a Docker container/service
func (m *MetadataProviderImpl) SetDockerURL(resourceID, customURL string) error {
	if m.dockerStore == nil {
		return fmt.Errorf("docker metadata store not available")
	}

	// Get existing metadata or create new
	existing := m.dockerStore.Get(resourceID)
	if existing == nil {
		existing = &config.DockerMetadata{
			ID: resourceID,
		}
	}

	// Update the URL
	existing.CustomURL = customURL

	return m.dockerStore.Set(resourceID, existing)
}

// SetHostURL sets the custom URL for a host
func (m *MetadataProviderImpl) SetHostURL(hostID, customURL string) error {
	if m.hostStore == nil {
		return fmt.Errorf("host metadata store not available")
	}

	// Get existing metadata or create new
	existing := m.hostStore.Get(hostID)
	if existing == nil {
		existing = &config.HostMetadata{
			ID: hostID,
		}
	}

	// Update the URL
	existing.CustomURL = customURL

	return m.hostStore.Set(hostID, existing)
}
