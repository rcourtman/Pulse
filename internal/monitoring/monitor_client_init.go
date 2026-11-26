package monitoring

import (
	"context"
	stderrors "errors"
	"net/url"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring/errors"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func (m *Monitor) initPVEClients(cfg *config.Config) {
	log.Info().Int("count", len(cfg.PVEInstances)).Msg("initializing PVE clients")
	for _, pve := range cfg.PVEInstances {
		if pve.Disabled {
			log.Info().Str("instance", pve.Name).Msg("Skipping PVE client init: instance is paused")
			continue
		}
		log.Info().
			Str("name", pve.Name).
			Str("host", pve.Host).
			Str("user", pve.User).
			Bool("hasToken", pve.TokenName != "").
			Msg("Configuring PVE instance")

		// Check if this is a cluster
		if pve.IsCluster && len(pve.ClusterEndpoints) > 0 {
			endpoints, endpointFingerprints, conflictingEndpoints := m.buildClusterEndpointsForInit(pve)

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
				conflictingEndpoints...,
			)
			m.replacePVEClient(pve.Name, clusterClient)
			log.Info().
				Str("instance", pve.Name).
				Str("cluster", pve.ClusterName).
				Int("endpoints", len(endpoints)).
				Msg("Cluster client created; awaiting first connectivity poll")
			continue
		}

		// Create regular client
		clientConfig := config.CreateProxmoxConfig(&pve)
		clientConfig.Timeout = cfg.ConnectionTimeout
		client, err := newProxmoxClientFunc(context.Background(), clientConfig)
		if err != nil {
			monErr := errors.WrapConnectionError("create_pve_client", pve.Name, pveStartupFailureError(err))
			log.Error().
				Err(monErr).
				Str("instance", pve.Name).
				Str("host", pve.Host).
				Str("user", pve.User).
				Bool("hasPassword", pve.Password != "").
				Bool("hasToken", pve.TokenValue != "").
				Msg("Failed to create PVE client - node will show as disconnected")
			m.publishPVEStartupFailure(pve, monErr)
			continue
		}
		m.replacePVEClient(pve.Name, client)
		log.Info().Str("instance", pve.Name).Msg("PVE client created; awaiting first connectivity poll")
	}
}

// pveStartupFailureError unwraps transport and TLS *url.Error failures that
// proxmox.NewClientWithContext wraps under "authentication failed: %w", so an
// unreachable or TLS-mismatched host is surfaced as Unreachable rather than
// misclassified as Unauthorized (invalid credentials).
func pveStartupFailureError(err error) error {
	var urlErr *url.Error
	if stderrors.As(err, &urlErr) && urlErr != nil {
		return urlErr
	}
	return err
}

// publishPVEStartupFailure records a PVE client construction or reconnect
// failure in scheduler health, staleness tracking, and connection health so
// Settings -> Infrastructure and connection-degraded alerts surface the real
// failure instead of staying indefinitely pending.
func (m *Monitor) publishPVEStartupFailure(pve config.PVEInstance, monErr error) {
	m.mu.Lock()
	if m.pollStatusMap == nil {
		m.pollStatusMap = make(map[string]*pollStatus)
	}
	m.mu.Unlock()
	m.recordTaskResult(InstanceTypePVE, pve.Name, monErr)
	if m.stalenessTracker != nil {
		m.stalenessTracker.UpdateError(InstanceTypePVE, pve.Name)
	}
	m.setProviderConnectionHealth(InstanceTypePVE, pve.Name, false)
}

func (m *Monitor) initPBSClients(cfg *config.Config) {
	log.Info().Int("count", len(cfg.PBSInstances)).Msg("initializing PBS clients")
	for _, pbsInst := range cfg.PBSInstances {
		if pbsInst.Disabled {
			log.Info().Str("instance", pbsInst.Name).Msg("Skipping PBS client init: instance is paused")
			continue
		}
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
			m.recordTaskResult(InstanceTypePBS, pbsInst.Name, monErr)
			if m.stalenessTracker != nil {
				m.stalenessTracker.UpdateError(InstanceTypePBS, pbsInst.Name)
			}
			m.publishPBSConnectionOutcome(models.PBSInstance{
				ID:       PBSMonitorResourceID(pbsInst.Name),
				Name:     pbsInst.Name,
				Host:     pbsInst.Host,
				GuestURL: pbsInst.GuestURL,
				Version:  "unknown",
				LastSeen: time.Now(),
			}, monErr)
			continue
		}
		m.replacePBSClient(pbsInst.Name, client)
		log.Info().
			Str("instance", pbsInst.Name).
			Msg("PBS client created; awaiting first connectivity poll")
	}
}

func (m *Monitor) initPMGClients(cfg *config.Config) {
	log.Info().Int("count", len(cfg.PMGInstances)).Msg("initializing PMG clients")
	for _, pmgInst := range cfg.PMGInstances {
		if pmgInst.Disabled {
			log.Info().Str("instance", pmgInst.Name).Msg("Skipping PMG client init: instance is paused")
			continue
		}
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
			m.publishPMGStartupFailure(pmgInst, monErr)
			continue
		}

		m.replacePMGClient(pmgInst.Name, client)
		log.Info().
			Str("instance", pmgInst.Name).
			Msg("PMG client created; awaiting first connectivity poll")
	}
}
