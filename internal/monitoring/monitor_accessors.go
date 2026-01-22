package monitoring

import "github.com/rcourtman/pulse-go-rewrite/internal/config"

// GetConfig returns the current configuration used by the monitor
func (m *Monitor) GetConfig() *config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}
