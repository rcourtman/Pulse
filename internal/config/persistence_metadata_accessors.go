package config

// GetGuestMetadataStore returns the guest metadata store, creating it if necessary
func (c *ConfigPersistence) GetGuestMetadataStore() *GuestMetadataStore {
	c.metadataMu.Lock()
	defer c.metadataMu.Unlock()

	if c.guestMetadataStore == nil {
		c.guestMetadataStore = NewGuestMetadataStore(c.configDir, c.fs)
	}
	return c.guestMetadataStore
}

// GetDockerMetadataStore returns the docker metadata store, creating it if necessary
func (c *ConfigPersistence) GetDockerMetadataStore() *DockerMetadataStore {
	c.metadataMu.Lock()
	defer c.metadataMu.Unlock()

	if c.dockerMetadataStore == nil {
		c.dockerMetadataStore = NewDockerMetadataStore(c.configDir, c.fs)
	}
	return c.dockerMetadataStore
}

// GetHostMetadataStore returns the host metadata store, creating it if necessary
func (c *ConfigPersistence) GetHostMetadataStore() *HostMetadataStore {
	c.metadataMu.Lock()
	defer c.metadataMu.Unlock()

	if c.hostMetadataStore == nil {
		c.hostMetadataStore = NewHostMetadataStore(c.configDir, c.fs)
	}
	return c.hostMetadataStore
}
