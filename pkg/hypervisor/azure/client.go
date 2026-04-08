// Package azure implements the hypervisor.Provider interface for Microsoft Azure.
//
// Uses the Azure SDK for Go to monitor:
//   - Resource groups → hypervisor.Node (logical grouping)
//   - Azure VMs → hypervisor.VM
//   - Managed disks → hypervisor.Storage
//   - Azure Monitor → metrics
//
// Required dependency: github.com/Azure/azure-sdk-for-go
package azure

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor"
	"github.com/rs/zerolog/log"
)

// Config holds Azure connection parameters.
type Config struct {
	ID             string
	Name           string
	SubscriptionID string // Azure subscription ID
	TenantID       string // Azure AD tenant ID
	ClientID       string // Service principal client ID
	ClientSecret   string // Service principal client secret
	ResourceGroup  string // Specific resource group (empty = all)
}

// Provider implements hypervisor.Provider for Azure.
type Provider struct {
	cfg     Config
	mu      sync.RWMutex
	healthy bool

	cachedNodes   []hypervisor.Node
	cachedVMs     []hypervisor.VM
	cachedStorage []hypervisor.Storage
	lastPoll      time.Time
}

// New creates a new Azure provider.
func New(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Type() hypervisor.ProviderType { return hypervisor.ProviderAzure }
func (p *Provider) ID() string                     { return p.cfg.ID }
func (p *Provider) Name() string                   { return p.cfg.Name }

// Connect initializes the Azure SDK clients.
//
// Full implementation:
//
//	cred, err := azidentity.NewClientSecretCredential(p.cfg.TenantID, p.cfg.ClientID, p.cfg.ClientSecret, nil)
//	p.vmClient, _ = armcompute.NewVirtualMachinesClient(p.cfg.SubscriptionID, cred, nil)
//	p.diskClient, _ = armcompute.NewDisksClient(p.cfg.SubscriptionID, cred, nil)
func (p *Provider) Connect(ctx context.Context) error {
	if p.cfg.SubscriptionID == "" {
		return fmt.Errorf("azure: subscription ID is required")
	}
	if p.cfg.TenantID == "" || p.cfg.ClientID == "" || p.cfg.ClientSecret == "" {
		return fmt.Errorf("azure: tenant ID, client ID, and client secret are required")
	}

	log.Info().
		Str("subscription", p.cfg.SubscriptionID).
		Str("id", p.cfg.ID).
		Msg("Azure provider connected (stub)")

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

// GetNodes returns Azure resource groups as logical nodes.
func (p *Provider) GetNodes(_ context.Context) ([]hypervisor.Node, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("azure: not connected")
	}
	return p.cachedNodes, nil
}

// GetVMs returns Azure VMs.
//
// Full implementation:
//
//	pager := p.vmClient.NewListAllPager(nil)
//	for pager.More() {
//	    page, _ := pager.NextPage(ctx)
//	    for _, vm := range page.Value {
//	        // Map Azure VM to hypervisor.VM
//	    }
//	}
func (p *Provider) GetVMs(_ context.Context, nodeID string) ([]hypervisor.VM, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("azure: not connected")
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

// GetStorage returns Azure managed disks.
func (p *Provider) GetStorage(_ context.Context, _ string) ([]hypervisor.Storage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("azure: not connected")
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
