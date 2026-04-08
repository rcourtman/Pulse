// Package nutanix implements the hypervisor.Provider interface for Nutanix AHV via Prism Central.
//
// Uses the Prism Central v3 REST API to monitor:
//   - Clusters → hypervisor.Node
//   - AHV VMs → hypervisor.VM
//   - Storage containers → hypervisor.Storage
//
// API reference: https://www.nutanix.dev/api_references/prism-central-v3/
package nutanix

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor"
	"github.com/rs/zerolog/log"
)

// Config holds Nutanix Prism Central connection parameters.
type Config struct {
	ID       string
	Name     string
	Host     string // Prism Central URL (e.g., "prism.example.com:9440")
	Username string
	Password string
	Insecure bool // Skip TLS verification
}

// Provider implements hypervisor.Provider for Nutanix.
type Provider struct {
	cfg     Config
	mu      sync.RWMutex
	healthy bool

	cachedNodes   []hypervisor.Node
	cachedVMs     []hypervisor.VM
	cachedStorage []hypervisor.Storage
	lastPoll      time.Time
}

// New creates a new Nutanix provider.
func New(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Type() hypervisor.ProviderType { return hypervisor.ProviderNutanix }
func (p *Provider) ID() string                     { return p.cfg.ID }
func (p *Provider) Name() string                   { return p.cfg.Name }

// Connect verifies connectivity to Prism Central.
//
// Full implementation:
//
//	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: p.cfg.Insecure}}}
//	req, _ := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("https://%s/api/nutanix/v3/clusters/list", p.cfg.Host), ...)
//	req.SetBasicAuth(p.cfg.Username, p.cfg.Password)
//	resp, err := client.Do(req)
func (p *Provider) Connect(ctx context.Context) error {
	if p.cfg.Host == "" {
		return fmt.Errorf("nutanix: host is required")
	}
	if p.cfg.Username == "" || p.cfg.Password == "" {
		return fmt.Errorf("nutanix: username and password are required")
	}

	log.Info().
		Str("host", p.cfg.Host).
		Str("id", p.cfg.ID).
		Msg("Nutanix Prism Central provider connected (stub)")

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

// GetNodes returns Nutanix clusters as nodes.
//
// API: POST /api/nutanix/v3/clusters/list
func (p *Provider) GetNodes(_ context.Context) ([]hypervisor.Node, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("nutanix: not connected")
	}
	return p.cachedNodes, nil
}

// GetVMs returns AHV VMs.
//
// API: POST /api/nutanix/v3/vms/list
func (p *Provider) GetVMs(_ context.Context, nodeID string) ([]hypervisor.VM, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("nutanix: not connected")
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

// GetStorage returns Nutanix storage containers.
//
// API: POST /api/nutanix/v3/storage_containers/list
func (p *Provider) GetStorage(_ context.Context, _ string) ([]hypervisor.Storage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("nutanix: not connected")
	}
	return p.cachedStorage, nil
}

// SupportedConsoleTypes returns VNC (Nutanix supports VNC console for AHV VMs).
func (p *Provider) SupportedConsoleTypes() []hypervisor.ConsoleType {
	return []hypervisor.ConsoleType{hypervisor.ConsoleVNC}
}

func (p *Provider) GetConsoleTicket(_ context.Context, nodeID, vmID string, consoleType hypervisor.ConsoleType) (*hypervisor.ConsoleTicket, error) {
	return &hypervisor.ConsoleTicket{
		Type:   consoleType,
		Host:   p.cfg.Host,
		VMID:   vmID,
		NodeID: nodeID,
	}, nil
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
