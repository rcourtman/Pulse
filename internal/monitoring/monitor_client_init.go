package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/errors"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) initPVEClients(cfg *config.Config) {
	log.Info().Int("count", len(cfg.PVEInstances)).Msg("initializing PVE clients")
	for _, pve := range cfg.PVEInstances {
		log.Info().
			Str("name", pve.Name).
			Str("host", pve.Host).
			Str("user", pve.User).
			Bool("hasToken", pve.TokenName != "").
			Msg("Configuring PVE instance")

		// Check if this is a cluster
		if pve.IsCluster && len(pve.ClusterEndpoints) > 0 {
			endpoints, endpointFingerprints := m.buildClusterEndpointsForInit(pve)

			log.Info().
				Str("cluster", pve.ClusterName).
				Strs("endpoints", endpoints).
				Int("fingerprints", len(endpointFingerprints)).
				Msg("Creating cluster-aware client")

			clientConfig := config.CreateProxmoxConfig(&pve)
			clientConfig.Timeout = cfg.ConnectionTimeout
			clusterClient := proxmox.NewClusterClient(
				pve.Name,
				clientConfig,
				endpoints,
				endpointFingerprints,
			)
			m.pveClients[pve.Name] = clusterClient
			log.Info().
				Str("instance", pve.Name).
				Str("cluster", pve.ClusterName).
				Int("endpoints", len(endpoints)).
				Msg("Cluster client created successfully")
			// Set initial connection health to true for cluster
			m.state.SetConnectionHealth(pve.Name, true)
			continue
		}

		// Create regular client
		clientConfig := config.CreateProxmoxConfig(&pve)
		clientConfig.Timeout = cfg.ConnectionTimeout
		client, err := newProxmoxClientFunc(clientConfig)
		if err != nil {
			monErr := errors.WrapConnectionError("create_pve_client", pve.Name, err)
			log.Error().
				Err(monErr).
				Str("instance", pve.Name).
				Str("host", pve.Host).
				Str("user", pve.User).
				Bool("hasPassword", pve.Password != "").
				Bool("hasToken", pve.TokenValue != "").
				Msg("Failed to create PVE client - node will show as disconnected")
			// Set initial connection health to false for this node
			m.state.SetConnectionHealth(pve.Name, false)
			continue
		}
		m.pveClients[pve.Name] = client
		log.Info().Str("instance", pve.Name).Msg("PVE client created successfully")
		// Set initial connection health to true
		m.state.SetConnectionHealth(pve.Name, true)
	}
}

func (m *Monitor) initPBSClients(cfg *config.Config) {
	log.Info().Int("count", len(cfg.PBSInstances)).Msg("initializing PBS clients")
	for _, pbsInst := range cfg.PBSInstances {
		log.Info().
			Str("name", pbsInst.Name).
			Str("host", pbsInst.Host).
			Str("user", pbsInst.User).
			Bool("hasToken", pbsInst.TokenName != "").
			Msg("Configuring PBS instance")

		clientConfig := config.CreatePBSConfig(&pbsInst)
		clientConfig.Timeout = 60 * time.Second // Very generous timeout for slow PBS servers
		client, err := pbs.NewClient(clientConfig)
		if err != nil {
			monErr := errors.WrapConnectionError("create_pbs_client", pbsInst.Name, err)
			log.Error().
				Err(monErr).
				Str("instance", pbsInst.Name).
				Str("host", pbsInst.Host).
				Str("user", pbsInst.User).
				Bool("hasPassword", pbsInst.Password != "").
				Bool("hasToken", pbsInst.TokenValue != "").
				Msg("Failed to create PBS client - node will show as disconnected")
			// Set initial connection health to false for this node
			m.state.SetConnectionHealth("pbs-"+pbsInst.Name, false)
			continue
		}
		m.pbsClients[pbsInst.Name] = client
		log.Info().Str("instance", pbsInst.Name).Msg("PBS client created successfully")
		// Set initial connection health to true
		m.state.SetConnectionHealth("pbs-"+pbsInst.Name, true)
	}
}

func (m *Monitor) initPMGClients(cfg *config.Config) {
	log.Info().Int("count", len(cfg.PMGInstances)).Msg("initializing PMG clients")
	for _, pmgInst := range cfg.PMGInstances {
		log.Info().
			Str("name", pmgInst.Name).
			Str("host", pmgInst.Host).
			Str("user", pmgInst.User).
			Bool("hasToken", pmgInst.TokenName != "").
			Msg("Configuring PMG instance")

		clientConfig := config.CreatePMGConfig(&pmgInst)
		if clientConfig.Timeout <= 0 {
			clientConfig.Timeout = 45 * time.Second
		}

		client, err := pmg.NewClient(clientConfig)
		if err != nil {
			monErr := errors.WrapConnectionError("create_pmg_client", pmgInst.Name, err)
			log.Error().
				Err(monErr).
				Str("instance", pmgInst.Name).
				Str("host", pmgInst.Host).
				Str("user", pmgInst.User).
				Bool("hasPassword", pmgInst.Password != "").
				Bool("hasToken", pmgInst.TokenValue != "").
				Msg("Failed to create PMG client - gateway will show as disconnected")
			m.state.SetConnectionHealth("pmg-"+pmgInst.Name, false)
			continue
		}

		m.pmgClients[pmgInst.Name] = client
		log.Info().Str("instance", pmgInst.Name).Msg("PMG client created successfully")
		m.state.SetConnectionHealth("pmg-"+pmgInst.Name, true)
	}
}
