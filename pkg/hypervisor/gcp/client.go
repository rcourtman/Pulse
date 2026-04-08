// Package gcp implements the hypervisor.Provider interface for Google Cloud Platform.
//
// Uses the Google Cloud Go SDK to monitor:
//   - Zones → hypervisor.Node (logical grouping)
//   - Compute Engine instances → hypervisor.VM
//   - Persistent disks → hypervisor.Storage
//   - Cloud Monitoring → metrics
//
// Required dependency: cloud.google.com/go/compute
package gcp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor"
	"github.com/rs/zerolog/log"
)

// Config holds GCP connection parameters.
type Config struct {
	ID              string
	Name            string
	ProjectID       string // GCP project ID
	Zone            string // Specific zone (empty = all zones in project)
	CredentialsJSON string // Service account JSON key (path or inline)
}

// Provider implements hypervisor.Provider for GCP.
type Provider struct {
	cfg     Config
	mu      sync.RWMutex
	healthy bool

	cachedNodes   []hypervisor.Node
	cachedVMs     []hypervisor.VM
	cachedStorage []hypervisor.Storage
	lastPoll      time.Time
}

// New creates a new GCP provider.
func New(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Type() hypervisor.ProviderType { return hypervisor.ProviderGCP }
func (p *Provider) ID() string                     { return p.cfg.ID }
func (p *Provider) Name() string                   { return p.cfg.Name }

// Connect initializes the GCP Compute Engine client.
//
// Full implementation:
//
//	client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentialsJSON([]byte(p.cfg.CredentialsJSON)))
//	p.instancesClient = client
//	p.disksClient, _ = compute.NewDisksRESTClient(ctx, ...)
func (p *Provider) Connect(ctx context.Context) error {
	if p.cfg.ProjectID == "" {
		return fmt.Errorf("gcp: project ID is required")
	}

	log.Info().
		Str("project", p.cfg.ProjectID).
		Str("id", p.cfg.ID).
		Msg("GCP provider connected (stub)")

	p.mu.Lock()
	p.healthy = true
	p.mu.Unlock()
	return nil
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.healthy = false
	return nil
}

func (p *Provider) Healthy(_ context.Context) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

// GetNodes returns GCP zones as logical nodes.
func (p *Provider) GetNodes(_ context.Context) ([]hypervisor.Node, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("gcp: not connected")
	}
	return p.cachedNodes, nil
}

// GetVMs returns Compute Engine instances.
//
// Full implementation:
//
//	iter := p.instancesClient.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{Project: p.cfg.ProjectID})
//	for {
//	    pair, err := iter.Next()
//	    if err == iterator.Done { break }
//	    for _, inst := range pair.Value.Instances {
//	        // Map GCE instance to hypervisor.VM
//	    }
//	}
func (p *Provider) GetVMs(_ context.Context, nodeID string) ([]hypervisor.VM, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("gcp: not connected")
	}
	if nodeID == "" {
		return p.cachedVMs, nil
	}
	var filtered []hypervisor.VM
	for _, vm := range p.cachedVMs {
		if vm.NodeID == nodeID {
			filtered = append(filtered, vm)
		}
	}
	return filtered, nil
}

func (p *Provider) GetContainers(_ context.Context, _ string) ([]hypervisor.Container, error) {
	return nil, nil
}

// GetStorage returns persistent disks.
func (p *Provider) GetStorage(_ context.Context, _ string) ([]hypervisor.Storage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("gcp: not connected")
	}
	return p.cachedStorage, nil
}

// UpdateCache is called by the polling loop.
func (p *Provider) UpdateCache(nodes []hypervisor.Node, vms []hypervisor.VM, storage []hypervisor.Storage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cachedNodes = nodes
	p.cachedVMs = vms
	p.cachedStorage = storage
	p.lastPoll = time.Now()
}
