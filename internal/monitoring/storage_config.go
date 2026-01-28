package monitoring

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

// GetStorageConfig fetches Proxmox storage configuration across instances.
// If instance is empty, returns configs for all instances.
func (m *Monitor) GetStorageConfig(ctx context.Context, instance string) (map[string][]proxmox.Storage, error) {
	if m == nil {
		return nil, fmt.Errorf("monitor not available")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	filter := strings.TrimSpace(instance)

	m.mu.RLock()
	clients := make(map[string]PVEClientInterface, len(m.pveClients))
	for name, client := range m.pveClients {
		clients[name] = client
	}
	m.mu.RUnlock()

	if len(clients) == 0 {
		return nil, fmt.Errorf("no PVE clients available")
	}

	results := make(map[string][]proxmox.Storage)
	var firstErr error

	for name, client := range clients {
		if client == nil {
			continue
		}
		if filter != "" && !m.matchesInstanceFilter(name, filter) {
			continue
		}

		storageInstance := name
		if cfg := m.getInstanceConfig(name); cfg != nil && cfg.IsCluster && cfg.ClusterName != "" {
			storageInstance = cfg.ClusterName
		}

		storages, err := client.GetAllStorage(ctx)
		if err != nil {
			if filter != "" {
				return nil, err
			}
			if firstErr == nil {
				firstErr = err
			}
			log.Warn().
				Err(err).
				Str("instance", name).
				Msg("Failed to fetch storage config for instance")
			continue
		}

		results[storageInstance] = append(results[storageInstance], storages...)
	}

	if len(results) == 0 && firstErr != nil {
		return nil, firstErr
	}

	if filter != "" && len(results) == 0 {
		return nil, fmt.Errorf("no PVE instance matches %s", filter)
	}

	return results, nil
}

func (m *Monitor) matchesInstanceFilter(instanceName, filter string) bool {
	if strings.EqualFold(instanceName, filter) {
		return true
	}
	cfg := m.getInstanceConfig(instanceName)
	if cfg != nil && cfg.IsCluster && cfg.ClusterName != "" && strings.EqualFold(cfg.ClusterName, filter) {
		return true
	}
	return false
}
