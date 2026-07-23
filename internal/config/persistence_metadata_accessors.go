package config

// SetMetadataStores makes persistence consumers share the active monitor's
// in-memory metadata stores. The stores already persist to this scope's data
// directory; sharing the instances prevents API, export/import, and monitor
// projection from observing different caches of the same files.
func (c *ConfigPersistence) SetMetadataStores(
	guest *GuestMetadataStore,
	docker *DockerMetadataStore,
	host *HostMetadataStore,
) {
	if c == nil {
		return
	}
	c.metadataMu.Lock()
	defer c.metadataMu.Unlock()
	if guest != nil {
		c.guestMetadataStore = guest
	}
	if docker != nil {
		c.dockerMetadataStore = docker
	}
	if host != nil {
		c.hostMetadataStore = host
	}
}

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
