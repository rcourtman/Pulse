package monitoring

import (
	"context"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

var connRetryDelays = []time.Duration{
	5 * time.Second,
	10 * time.Second,
	20 * time.Second,
	40 * time.Second,
	60 * time.Second,
}

// retryFailedConnections attempts to recreate clients that failed during initialization
// This handles cases where Proxmox/network isn't ready when Pulse starts
func (m *Monitor) retryFailedConnections(ctx context.Context) {
	defer recoverFromPanic("retryFailedConnections")

	// Retry schedule: 5s, 10s, 20s, 40s, 60s, then every 60s for up to 5 minutes total
	retryDelays := connRetryDelays

	maxRetryDuration := 5 * time.Minute
	startTime := time.Now()
	retryIndex := 0

	for {
		// Stop retrying after max duration or if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		if time.Since(startTime) > maxRetryDuration {
			log.Info().Msg("connection retry period expired")
			return
		}

		// Calculate next retry delay
		var delay time.Duration
		if retryIndex < len(retryDelays) {
			delay = retryDelays[retryIndex]
			retryIndex++
		} else {
			delay = 60 * time.Second // Continue retrying every 60s
		}

		// Wait before retry
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}

		// Check for missing clients and try to recreate them
		m.mu.Lock()
		missingPVE := []config.PVEInstance{}
		missingPBS := []config.PBSInstance{}

		// Find PVE instances without clients
		for _, pve := range m.config.PVEInstances {
			if _, exists := m.pveClients[pve.Name]; !exists {
				missingPVE = append(missingPVE, pve)
			}
		}

		// Find PBS instances without clients
		for _, pbsInst := range m.config.PBSInstances {
			if _, exists := m.pbsClients[pbsInst.Name]; !exists {
				missingPBS = append(missingPBS, pbsInst)
			}
		}
		m.mu.Unlock()

		// If no missing clients, we're done
		if len(missingPVE) == 0 && len(missingPBS) == 0 {
			log.Info().Msg("all client connections established successfully")
			return
		}

		log.Info().
			Int("missingPVE", len(missingPVE)).
			Int("missingPBS", len(missingPBS)).
			Dur("nextRetry", delay).
			Msg("Attempting to reconnect failed clients")

		// Try to recreate PVE clients
		for _, pve := range missingPVE {
			if pve.IsCluster && len(pve.ClusterEndpoints) > 0 {
				// Create cluster client
				endpoints, endpointFingerprints := m.buildClusterEndpointsForReconnect(pve)

				clientConfig := config.CreateProxmoxConfig(&pve)
				clientConfig.Timeout = m.config.ConnectionTimeout
				clusterClient := proxmox.NewClusterClient(pve.Name, clientConfig, endpoints, endpointFingerprints)

				m.mu.Lock()
				m.pveClients[pve.Name] = clusterClient
				m.state.SetConnectionHealth(pve.Name, true)
				m.mu.Unlock()

				log.Info().
					Str("instance", pve.Name).
					Str("cluster", pve.ClusterName).
					Msg("Successfully reconnected cluster client")
				continue
			}

			// Create regular client
			clientConfig := config.CreateProxmoxConfig(&pve)
			clientConfig.Timeout = m.config.ConnectionTimeout
			client, err := newProxmoxClientFunc(clientConfig)
			if err != nil {
				log.Warn().
					Err(err).
					Str("instance", pve.Name).
					Msg("Failed to reconnect PVE client, will retry")
				continue
			}

			m.mu.Lock()
			m.pveClients[pve.Name] = client
			m.state.SetConnectionHealth(pve.Name, true)
			m.mu.Unlock()

			log.Info().
				Str("instance", pve.Name).
				Msg("Successfully reconnected PVE client")
		}

		// Try to recreate PBS clients
		for _, pbsInst := range missingPBS {
			clientConfig := config.CreatePBSConfig(&pbsInst)
			clientConfig.Timeout = 60 * time.Second
			client, err := pbs.NewClient(clientConfig)
			if err != nil {
				log.Warn().
					Err(err).
					Str("instance", pbsInst.Name).
					Msg("Failed to reconnect PBS client, will retry")
				continue
			}

			m.mu.Lock()
			m.pbsClients[pbsInst.Name] = client
			m.state.SetConnectionHealth("pbs-"+pbsInst.Name, true)
			m.mu.Unlock()

			log.Info().
				Str("instance", pbsInst.Name).
				Msg("Successfully reconnected PBS client")
		}
	}
}
