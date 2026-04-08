// Package proxmoxhv adapts the existing Proxmox VE client to the hypervisor.Provider interface.
// This is a thin adapter: the real logic stays in pkg/proxmox.
package proxmoxhv

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Adapter wraps a proxmox.Client to satisfy hypervisor.Provider, ConsoleProvider, and PowerProvider.
type Adapter struct {
	client     *proxmox.Client
	id         string
	name       string
	host       string
	connected  bool
}

// NewAdapter creates a new Proxmox provider adapter.
func NewAdapter(id, name string, client *proxmox.Client, host string) *Adapter {
	return &Adapter{
		client: client,
		id:     id,
		name:   name,
		host:   host,
	}
}

func (a *Adapter) Type() hypervisor.ProviderType { return hypervisor.ProviderProxmox }
func (a *Adapter) ID() string                     { return a.id }
func (a *Adapter) Name() string                   { return a.name }

func (a *Adapter) Connect(ctx context.Context) error {
	// Verify connectivity by fetching nodes.
	_, err := a.client.GetNodes(ctx)
	if err != nil {
		a.connected = false
		return fmt.Errorf("proxmox connect: %w", err)
	}
	a.connected = true
	return nil
}

func (a *Adapter) Close() error {
	a.connected = false
	return nil
}

func (a *Adapter) Healthy(ctx context.Context) bool {
	nodes, err := a.client.GetNodes(ctx)
	return err == nil && len(nodes) > 0
}

func (a *Adapter) GetNodes(ctx context.Context) ([]hypervisor.Node, error) {
	pveNodes, err := a.client.GetNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]hypervisor.Node, 0, len(pveNodes))
	for _, n := range pveNodes {
		out = append(out, mapNode(n, a.id, a.name))
	}
	return out, nil
}

func (a *Adapter) GetVMs(ctx context.Context, nodeID string) ([]hypervisor.VM, error) {
	if nodeID == "" {
		// Get VMs across all nodes.
		nodes, err := a.client.GetNodes(ctx)
		if err != nil {
			return nil, err
		}
		var all []hypervisor.VM
		for _, n := range nodes {
			vms, err := a.client.GetVMs(ctx, n.Node)
			if err != nil {
				continue
			}
			for _, vm := range vms {
				all = append(all, mapVM(vm, n.Node, a.id, a.name))
			}
		}
		return all, nil
	}
	pveVMs, err := a.client.GetVMs(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	out := make([]hypervisor.VM, 0, len(pveVMs))
	for _, vm := range pveVMs {
		out = append(out, mapVM(vm, nodeID, a.id, a.name))
	}
	return out, nil
}

func (a *Adapter) GetContainers(ctx context.Context, nodeID string) ([]hypervisor.Container, error) {
	if nodeID == "" {
		nodes, err := a.client.GetNodes(ctx)
		if err != nil {
			return nil, err
		}
		var all []hypervisor.Container
		for _, n := range nodes {
			cts, err := a.client.GetContainers(ctx, n.Node)
			if err != nil {
				continue
			}
			for _, ct := range cts {
				all = append(all, mapContainer(ct, n.Node, a.id, a.name))
			}
		}
		return all, nil
	}
	pveCTs, err := a.client.GetContainers(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	out := make([]hypervisor.Container, 0, len(pveCTs))
	for _, ct := range pveCTs {
		out = append(out, mapContainer(ct, nodeID, a.id, a.name))
	}
	return out, nil
}

func (a *Adapter) GetStorage(ctx context.Context, nodeID string) ([]hypervisor.Storage, error) {
	var pveStorage []proxmox.Storage
	var err error
	if nodeID == "" {
		pveStorage, err = a.client.GetAllStorage(ctx)
	} else {
		pveStorage, err = a.client.GetStorage(ctx, nodeID)
	}
	if err != nil {
		return nil, err
	}
	out := make([]hypervisor.Storage, 0, len(pveStorage))
	for _, s := range pveStorage {
		out = append(out, mapStorage(s, nodeID, a.id))
	}
	return out, nil
}

// SupportedConsoleTypes returns console types available for Proxmox VMs.
func (a *Adapter) SupportedConsoleTypes() []hypervisor.ConsoleType {
	return []hypervisor.ConsoleType{
		hypervisor.ConsoleVNC,
		hypervisor.ConsoleSPICE,
		hypervisor.ConsoleSSH,
		hypervisor.ConsoleSerial,
	}
}

// GetConsoleTicket acquires a VNC/SPICE ticket from the Proxmox API.
// The actual VNC proxy endpoint call will be added to pkg/proxmox/client.go separately.
func (a *Adapter) GetConsoleTicket(ctx context.Context, nodeID, vmID string, consoleType hypervisor.ConsoleType) (*hypervisor.ConsoleTicket, error) {
	return &hypervisor.ConsoleTicket{
		Type:   consoleType,
		Host:   a.host,
		VMID:   vmID,
		NodeID: nodeID,
	}, nil
}

// --- Mapping functions ---

func mapNode(n proxmox.Node, providerID, providerName string) hypervisor.Node {
	memPct := float64(0)
	if n.MaxMem > 0 {
		memPct = float64(n.Mem) / float64(n.MaxMem) * 100
	}
	diskPct := float64(0)
	if n.MaxDisk > 0 {
		diskPct = float64(n.Disk) / float64(n.MaxDisk) * 100
	}
	return hypervisor.Node{
		ID:               fmt.Sprintf("proxmox-%s-%s", providerID, n.Node),
		Name:             n.Node,
		ProviderType:     hypervisor.ProviderProxmox,
		ProviderID:       providerID,
		ProviderName:     providerName,
		Status:           n.Status,
		CPU:              n.CPU,
		Memory:           hypervisor.ResourceUsage{Used: n.Mem, Total: n.MaxMem, Free: n.MaxMem - n.Mem, Pct: memPct},
		Disk:             hypervisor.ResourceUsage{Used: n.Disk, Total: n.MaxDisk, Free: n.MaxDisk - n.Disk, Pct: diskPct},
		Uptime:           int64(n.Uptime),
		LastSeen:         time.Now(),
		CPUInfo:          hypervisor.CPUInfo{Cores: n.MaxCPU},
	}
}

func mapVM(vm proxmox.VM, node, providerID, providerName string) hypervisor.VM {
	memPct := float64(0)
	if vm.MaxMem > 0 {
		memPct = float64(vm.Mem) / float64(vm.MaxMem) * 100
	}
	diskPct := float64(0)
	if vm.MaxDisk > 0 {
		diskPct = float64(vm.Disk) / float64(vm.MaxDisk) * 100
	}
	var tags []string
	if vm.Tags != "" {
		tags = strings.Split(vm.Tags, ";")
	}

	powerState := hypervisor.VMStopped
	switch vm.Status {
	case "running":
		powerState = hypervisor.VMRunning
	case "paused":
		powerState = hypervisor.VMPaused
	}

	consoleTypes := []hypervisor.ConsoleType{hypervisor.ConsoleVNC}
	if vm.Status == "running" {
		consoleTypes = append(consoleTypes, hypervisor.ConsoleSSH)
	}

	return hypervisor.VM{
		ID:           fmt.Sprintf("proxmox-%s-qemu-%d", providerID, vm.VMID),
		Name:         vm.Name,
		NodeID:       node,
		NodeName:     node,
		ProviderType: hypervisor.ProviderProxmox,
		ProviderID:   providerID,
		ProviderName: providerName,
		Status:       vm.Status,
		PowerState:   powerState,
		CPU:          vm.CPU,
		CPUs:         vm.CPUs,
		Memory:       hypervisor.ResourceUsage{Used: vm.Mem, Total: vm.MaxMem, Free: vm.MaxMem - vm.Mem, Pct: memPct},
		Disk:         hypervisor.ResourceUsage{Used: vm.Disk, Total: vm.MaxDisk, Free: vm.MaxDisk - vm.Disk, Pct: diskPct},
		NetworkIn:    int64(vm.NetIn),
		NetworkOut:   int64(vm.NetOut),
		DiskRead:     int64(vm.DiskRead),
		DiskWrite:    int64(vm.DiskWrite),
		Uptime:       int64(vm.Uptime),
		Template:     vm.Template != 0,
		Tags:         tags,
		LastSeen:     time.Now(),
		ConsoleTypes: consoleTypes,
	}
}

func mapContainer(ct proxmox.Container, node, providerID, providerName string) hypervisor.Container {
	memPct := float64(0)
	if ct.MaxMem > 0 {
		memPct = float64(ct.Mem) / float64(ct.MaxMem) * 100
	}
	diskPct := float64(0)
	if ct.MaxDisk > 0 {
		diskPct = float64(ct.Disk) / float64(ct.MaxDisk) * 100
	}
	var tags []string
	if ct.Tags != "" {
		tags = strings.Split(ct.Tags, ";")
	}

	return hypervisor.Container{
		ID:           fmt.Sprintf("proxmox-%s-lxc-%s", providerID, strconv.Itoa(int(ct.VMID))),
		Name:         ct.Name,
		NodeID:       node,
		NodeName:     node,
		ProviderType: hypervisor.ProviderProxmox,
		ProviderID:   providerID,
		ProviderName: providerName,
		Status:       ct.Status,
		CPU:          ct.CPU,
		CPUs:         int(ct.CPUs),
		Memory:       hypervisor.ResourceUsage{Used: ct.Mem, Total: ct.MaxMem, Free: ct.MaxMem - ct.Mem, Pct: memPct},
		Disk:         hypervisor.ResourceUsage{Used: ct.Disk, Total: ct.MaxDisk, Free: ct.MaxDisk - ct.Disk, Pct: diskPct},
		NetworkIn:    int64(ct.NetIn),
		NetworkOut:   int64(ct.NetOut),
		Uptime:       int64(ct.Uptime),
		Tags:         tags,
		LastSeen:     time.Now(),
	}
}

func mapStorage(s proxmox.Storage, nodeID, providerID string) hypervisor.Storage {
	pct := float64(0)
	total := s.Total
	used := s.Used
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	status := "inactive"
	if s.Active == 1 {
		status = "active"
	}
	return hypervisor.Storage{
		ID:           fmt.Sprintf("proxmox-%s-storage-%s", providerID, s.Storage),
		Name:         s.Storage,
		NodeID:       nodeID,
		ProviderType: hypervisor.ProviderProxmox,
		ProviderID:   providerID,
		Type:         s.Type,
		Status:       status,
		Usage:        hypervisor.ResourceUsage{Used: used, Total: total, Free: s.Available, Pct: pct},
		Shared:       s.Shared == 1,
	}
}
