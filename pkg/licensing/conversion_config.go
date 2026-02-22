package licensing

import (
	"sort"
	"strings"
	"sync"
)

// CollectionConfigSnapshot is the API-facing runtime configuration state.
type CollectionConfigSnapshot struct {
	Enabled          bool     `json:"enabled"`
	DisabledSurfaces []string `json:"disabled_surfaces"`
}

// CollectionConfig controls runtime conversion event collection.
type CollectionConfig struct {
	mu               sync.RWMutex
	enabled          bool
	disabledSurfaces map[string]bool
}

// NewCollectionConfig returns a default runtime config with collection enabled.
func NewCollectionConfig() *CollectionConfig {
	return &CollectionConfig{
		enabled:          true,
		disabledSurfaces: make(map[string]bool),
	}
}

// IsEnabled reports whether global conversion collection is enabled.
func (c *CollectionConfig) IsEnabled() bool {
	if c == nil {
		return true
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

// IsSurfaceEnabled reports whether the given surface can collect events.
func (c *CollectionConfig) IsSurfaceEnabled(surface string) bool {
	if c == nil {
		return true
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.enabled {
		return false
	}
	return !c.disabledSurfaces[strings.TrimSpace(surface)]
}

// GetConfig returns a copy of the current runtime config.
func (c *CollectionConfig) GetConfig() CollectionConfigSnapshot {
	if c == nil {
		return CollectionConfigSnapshot{
			Enabled:          true,
			DisabledSurfaces: []string{},
		}
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	disabled := make([]string, 0, len(c.disabledSurfaces))
	for surface := range c.disabledSurfaces {
		disabled = append(disabled, surface)
	}
	sort.Strings(disabled)

	return CollectionConfigSnapshot{
		Enabled:          c.enabled,
		DisabledSurfaces: disabled,
	}
}

// UpdateConfig atomically replaces current runtime config with the snapshot.
func (c *CollectionConfig) UpdateConfig(snapshot CollectionConfigSnapshot) {
	if c == nil {
		return
	}

	disabled := make(map[string]bool, len(snapshot.DisabledSurfaces))
	for _, surface := range snapshot.DisabledSurfaces {
		trimmed := strings.TrimSpace(surface)
		if trimmed == "" {
			continue
		}
		disabled[trimmed] = true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = snapshot.Enabled
	c.disabledSurfaces = disabled
}
