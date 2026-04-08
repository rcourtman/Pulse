package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor"
	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor/aws"
	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor/azure"
	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor/gcp"
	hvlibvirt "github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor/libvirt"
	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor/nutanix"
	"github.com/rcourtman/pulse-go-rewrite/pkg/hypervisor/vmware"
	"github.com/rs/zerolog/log"
)

const (
	defaultHypervisorPollInterval = 30 * time.Second
	minHypervisorPollInterval     = 10 * time.Second
)

// HypervisorMonitor manages polling for all non-Proxmox hypervisor providers.
// It runs alongside the existing PVE/PBS/PMG polling in Monitor without modifying it.
type HypervisorMonitor struct {
	registry *hypervisor.Registry
	state    *models.State
	mu       sync.RWMutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewHypervisorMonitor creates a new hypervisor monitor.
func NewHypervisorMonitor(state *models.State) *HypervisorMonitor {
	return &HypervisorMonitor{
		registry: hypervisor.NewRegistry(),
		state:    state,
	}
}

// Registry returns the hypervisor provider registry.
func (hm *HypervisorMonitor) Registry() *hypervisor.Registry {
	return hm.registry
}

// InitProviders creates and registers providers from the configuration.
func (hm *HypervisorMonitor) InitProviders(cfg *config.Config) error {
	for _, inst := range cfg.HypervisorInstances {
		if !inst.Enabled {
			log.Debug().Str("id", inst.ID).Str("type", inst.Type).Msg("Skipping disabled hypervisor instance")
			continue
		}

		provider, err := createProvider(inst)
		if err != nil {
			log.Error().Err(err).Str("id", inst.ID).Str("type", inst.Type).Msg("Failed to create hypervisor provider")
			continue
		}

		if err := hm.registry.Register(provider); err != nil {
			log.Error().Err(err).Str("id", inst.ID).Msg("Failed to register provider")
			continue
		}
	}
	return nil
}

// createProvider creates the appropriate provider based on the instance type.
func createProvider(inst config.HypervisorInstance) (hypervisor.Provider, error) {
	switch inst.Type {
	case "vmware":
		return vmware.New(vmware.Config{
			ID:         inst.ID,
			Name:       inst.Name,
			Host:       inst.Host,
			Username:   inst.Username,
			Password:   inst.Password,
			Insecure:   !inst.VerifySSL,
			Datacenter: inst.Datacenter,
		}), nil

	case "libvirt":
		return hvlibvirt.New(hvlibvirt.Config{
			ID:       inst.ID,
			Name:     inst.Name,
			Host:     inst.Host,
			Username: inst.Username,
			KeyFile:  inst.KeyFile,
			Insecure: !inst.VerifySSL,
		}), nil

	case "nutanix":
		return nutanix.New(nutanix.Config{
			ID:       inst.ID,
			Name:     inst.Name,
			Host:     inst.Host,
			Username: inst.Username,
			Password: inst.Password,
			Insecure: !inst.VerifySSL,
		}), nil

	case "aws":
		return aws.New(aws.Config{
			ID:        inst.ID,
			Name:      inst.Name,
			Region:    inst.Region,
			AccessKey: inst.AccessKey,
			SecretKey: inst.SecretKey,
			Profile:   inst.Profile,
			RoleARN:   inst.RoleARN,
		}), nil

	case "azure":
		return azure.New(azure.Config{
			ID:             inst.ID,
			Name:           inst.Name,
			SubscriptionID: inst.SubscriptionID,
			TenantID:       inst.TenantID,
			ClientID:       inst.ClientID,
			ClientSecret:   inst.ClientSecret,
		}), nil

	case "gcp":
		return gcp.New(gcp.Config{
			ID:              inst.ID,
			Name:            inst.Name,
			ProjectID:       inst.ProjectID,
			CredentialsJSON: inst.CredentialsJSON,
		}), nil

	default:
		return nil, fmt.Errorf("unsupported hypervisor type: %s", inst.Type)
	}
}

// Start begins polling all registered hypervisor providers.
func (hm *HypervisorMonitor) Start(ctx context.Context) {
	ctx, hm.cancel = context.WithCancel(ctx)

	providers := hm.registry.All()
	for id, p := range providers {
		// Connect to each provider.
		if err := p.Connect(ctx); err != nil {
			log.Error().Err(err).Str("id", id).Msg("Failed to connect hypervisor provider")
			continue
		}

		// Start polling goroutine for each provider.
		hm.wg.Add(1)
		go hm.pollProvider(ctx, p)
	}

	log.Info().Int("providers", len(providers)).Msg("Hypervisor monitor started")
}

// Stop gracefully stops all polling goroutines.
func (hm *HypervisorMonitor) Stop() {
	if hm.cancel != nil {
		hm.cancel()
	}
	hm.wg.Wait()
	hm.registry.Close()
	log.Info().Msg("Hypervisor monitor stopped")
}

// pollProvider runs the polling loop for a single provider.
func (hm *HypervisorMonitor) pollProvider(ctx context.Context, p hypervisor.Provider) {
	defer hm.wg.Done()

	interval := defaultHypervisorPollInterval
	providerID := p.ID()
	providerType := string(p.Type())

	log.Info().
		Str("id", providerID).
		Str("type", providerType).
		Dur("interval", interval).
		Msg("Starting hypervisor polling")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial poll.
	hm.doPoll(ctx, p)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.doPoll(ctx, p)
		}
	}
}

// doPoll fetches data from a provider and updates the shared state.
func (hm *HypervisorMonitor) doPoll(ctx context.Context, p hypervisor.Provider) {
	providerID := p.ID()
	providerType := string(p.Type())

	if !p.Healthy(ctx) {
		log.Warn().Str("id", providerID).Msg("Hypervisor provider unhealthy, attempting reconnect")
		if err := p.Connect(ctx); err != nil {
			log.Error().Err(err).Str("id", providerID).Msg("Reconnect failed")
			return
		}
	}

	// Fetch nodes.
	nodes, err := p.GetNodes(ctx)
	if err != nil {
		log.Error().Err(err).Str("id", providerID).Str("type", providerType).Msg("Failed to get nodes")
		return
	}

	// Fetch VMs across all nodes.
	vms, err := p.GetVMs(ctx, "")
	if err != nil {
		log.Error().Err(err).Str("id", providerID).Str("type", providerType).Msg("Failed to get VMs")
	}

	// Fetch containers (if supported).
	containers, _ := p.GetContainers(ctx, "")

	// Fetch storage.
	storage, _ := p.GetStorage(ctx, "")

	// Map to models and merge into state.
	hm.mergeIntoState(p, nodes, vms, containers, storage)

	log.Debug().
		Str("id", providerID).
		Int("nodes", len(nodes)).
		Int("vms", len(vms)).
		Int("containers", len(containers)).
		Int("storage", len(storage)).
		Msg("Hypervisor poll complete")
}

// mergeIntoState converts hypervisor types to model types and merges them into the shared state.
func (hm *HypervisorMonitor) mergeIntoState(
	p hypervisor.Provider,
	nodes []hypervisor.Node,
	vms []hypervisor.VM,
	containers []hypervisor.Container,
	storage []hypervisor.Storage,
) {
	providerType := string(p.Type())

	// Convert nodes.
	modelNodes := make([]models.Node, 0, len(nodes))
	for _, n := range nodes {
		modelNodes = append(modelNodes, models.Node{
			ID:           n.ID,
			Name:         n.Name,
			DisplayName:  n.DisplayName,
			Platform:     providerType,
			ProviderID:   n.ProviderID,
			ProviderName: n.ProviderName,
			Status:       n.Status,
			CPU:          n.CPU,
			Memory: models.Memory{
				Used:  int64(n.Memory.Used),
				Total: int64(n.Memory.Total),
				Free:  int64(n.Memory.Free),
			},
			Uptime:   n.Uptime,
			LastSeen: n.LastSeen,
		})
	}

	// Convert VMs.
	modelVMs := make([]models.VM, 0, len(vms))
	for _, vm := range vms {
		consoleTypes := make([]string, len(vm.ConsoleTypes))
		for i, ct := range vm.ConsoleTypes {
			consoleTypes[i] = string(ct)
		}
		modelVMs = append(modelVMs, models.VM{
			ID:           vm.ID,
			Name:         vm.Name,
			Node:         vm.NodeName,
			Platform:     providerType,
			ProviderID:   vm.ProviderID,
			ProviderName: vm.ProviderName,
			ConsoleTypes: consoleTypes,
			Status:       vm.Status,
			CPU:          vm.CPU,
			CPUs:         vm.CPUs,
			Memory: models.Memory{
				Used:  int64(vm.Memory.Used),
				Total: int64(vm.Memory.Total),
				Free:  int64(vm.Memory.Free),
			},
			IPAddresses: vm.IPAddresses,
			OSName:      vm.OSName,
			NetworkIn:   vm.NetworkIn,
			NetworkOut:  vm.NetworkOut,
			DiskRead:    vm.DiskRead,
			DiskWrite:   vm.DiskWrite,
			Uptime:      vm.Uptime,
			Template:    vm.Template,
			Tags:        vm.Tags,
			LastSeen:    vm.LastSeen,
		})
	}

	// Convert containers.
	modelContainers := make([]models.Container, 0, len(containers))
	for _, ct := range containers {
		modelContainers = append(modelContainers, models.Container{
			ID:           ct.ID,
			Name:         ct.Name,
			Node:         ct.NodeName,
			Platform:     providerType,
			ProviderID:   ct.ProviderID,
			ProviderName: ct.ProviderName,
			Status:       ct.Status,
			CPU:          ct.CPU,
			CPUs:         ct.CPUs,
			Memory: models.Memory{
				Used:  int64(ct.Memory.Used),
				Total: int64(ct.Memory.Total),
				Free:  int64(ct.Memory.Free),
			},
			NetworkIn:  ct.NetworkIn,
			NetworkOut: ct.NetworkOut,
			Uptime:     ct.Uptime,
			Tags:       ct.Tags,
			LastSeen:   ct.LastSeen,
		})
	}

	// Merge into state.
	// The state is managed by the existing Monitor with its own mutex.
	// We append provider resources rather than replacing, so the state contains
	// resources from all sources (Proxmox + all hypervisor providers).
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Note: The actual merge into models.State needs to be coordinated with
	// the existing Monitor's state management. This is a placeholder that shows
	// the conversion. Integration with the Monitor's state will use the existing
	// state.SetNodes/SetVMs pattern or direct field access with proper locking.
	_ = modelNodes
	_ = modelVMs
	_ = modelContainers
}

// GetHealth returns health status for all providers.
func (hm *HypervisorMonitor) GetHealth(ctx context.Context) []hypervisor.ProviderHealth {
	return hm.registry.HealthCheck(ctx)
}
