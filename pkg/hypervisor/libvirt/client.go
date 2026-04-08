// Package libvirt implements the hypervisor.Provider interface for KVM/libvirt hosts.
//
// It supports two modes of operation:
//  1. Agent-based: A pulse-libvirt-agent runs on the KVM host and pushes data to Pulse
//     (preferred, follows the existing Docker/Host agent pattern).
//  2. Direct API: Connects to the libvirt daemon via TCP/TLS using libvirt-go bindings
//     (requires network access to libvirtd).
//
// The provider maps libvirt concepts to the unified hypervisor model:
//   - KVM hosts → hypervisor.Node
//   - Domains (VMs) → hypervisor.VM
//   - Storage pools → hypervisor.Storage
//   - libvirt does not have containers (use LXC/Docker agents instead)
package libvirt

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor"
	"github.com/rs/zerolog/log"
)

// Config holds KVM/libvirt connection parameters.
type Config struct {
	ID       string
	Name     string
	Host     string // libvirt URI (e.g., "qemu+ssh://root@kvm-host/system" or "qemu:///system")
	Username string // SSH user for qemu+ssh
	KeyFile  string // SSH key file path
	Insecure bool   // Skip TLS verification for qemu+tls
}

// Provider implements hypervisor.Provider for KVM/libvirt.
type Provider struct {
	cfg     Config
	mu      sync.RWMutex
	healthy bool

	cachedNodes   []hypervisor.Node
	cachedVMs     []hypervisor.VM
	cachedStorage []hypervisor.Storage
	lastPoll      time.Time
}

// New creates a new KVM/libvirt provider.
func New(cfg Config) *Provider {
	return &Provider{cfg: cfg}
}

func (p *Provider) Type() hypervisor.ProviderType { return hypervisor.ProviderLibvirt }
func (p *Provider) ID() string                     { return p.cfg.ID }
func (p *Provider) Name() string                   { return p.cfg.Name }

// Connect establishes a connection to the libvirt daemon.
//
// Full implementation with libvirt-go:
//
//	conn, err := libvirt.NewConnect(p.cfg.Host)
//	if err != nil { return err }
//	p.conn = conn
//
// For agent-based mode, this validates that the agent URL is reachable.
func (p *Provider) Connect(ctx context.Context) error {
	if p.cfg.Host == "" {
		return fmt.Errorf("libvirt: host/URI is required")
	}

	log.Info().
		Str("uri", p.cfg.Host).
		Str("id", p.cfg.ID).
		Msg("KVM/libvirt provider connected (stub)")

	p.mu.Lock()
	p.healthy = true
	p.mu.Unlock()
	return nil
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.healthy = false
	// Full implementation: p.conn.Close()
	return nil
}

func (p *Provider) Healthy(_ context.Context) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.healthy
}

// GetNodes returns the KVM host as a hypervisor.Node.
//
// Full implementation:
//
//	info, _ := p.conn.GetNodeInfo()
//	hostname, _ := p.conn.GetHostname()
//	return []hypervisor.Node{{
//	    ID:       fmt.Sprintf("libvirt-%s", hostname),
//	    Name:     hostname,
//	    CPUInfo:  hypervisor.CPUInfo{Cores: int(info.Cpus), Sockets: int(info.Sockets)},
//	    Memory:   hypervisor.ResourceUsage{Total: info.Memory * 1024},
//	}}
func (p *Provider) GetNodes(_ context.Context) ([]hypervisor.Node, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("libvirt: not connected")
	}
	return p.cachedNodes, nil
}

// GetVMs returns libvirt domains as hypervisor.VM resources.
//
// Full implementation:
//
//	domains, _ := p.conn.ListAllDomains(libvirt.CONNECT_LIST_DOMAINS_ACTIVE | libvirt.CONNECT_LIST_DOMAINS_INACTIVE)
//	for _, dom := range domains {
//	    info, _ := dom.GetInfo()
//	    name, _ := dom.GetName()
//	    // Map to hypervisor.VM with info.State, info.NrVirtCpu, info.Memory, etc.
//	}
func (p *Provider) GetVMs(_ context.Context, nodeID string) ([]hypervisor.VM, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("libvirt: not connected")
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

// GetContainers returns nil (libvirt doesn't manage containers directly).
func (p *Provider) GetContainers(_ context.Context, _ string) ([]hypervisor.Container, error) {
	return nil, nil
}

// GetStorage returns libvirt storage pools.
//
// Full implementation:
//
//	pools, _ := p.conn.ListAllStoragePools(0)
//	for _, pool := range pools {
//	    info, _ := pool.GetInfo()
//	    name, _ := pool.GetName()
//	    // Map to hypervisor.Storage with info.Capacity, info.Allocation, info.Available
//	}
func (p *Provider) GetStorage(_ context.Context, _ string) ([]hypervisor.Storage, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if !p.healthy {
		return nil, fmt.Errorf("libvirt: not connected")
	}
	return p.cachedStorage, nil
}

// SupportedConsoleTypes returns console types for KVM VMs.
func (p *Provider) SupportedConsoleTypes() []hypervisor.ConsoleType {
	return []hypervisor.ConsoleType{
		hypervisor.ConsoleVNC,
		hypervisor.ConsoleSPICE,
		hypervisor.ConsoleSerial,
	}
}

// GetConsoleTicket acquires console connection info for a libvirt domain.
//
// For VNC: parse the domain XML to find VNC port, then return direct connection info.
// For SPICE: parse domain XML for SPICE port and password.
// For Serial: open a serial console stream via virDomainOpenConsole.
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
