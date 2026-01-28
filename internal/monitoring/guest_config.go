package monitoring

import (
	"context"
	"fmt"
	"strings"
)

// GetGuestConfig fetches Proxmox guest configuration for a VM or LXC container.
// If instance or node are empty, it attempts to resolve them from the current state.
func (m *Monitor) GetGuestConfig(ctx context.Context, guestType, instance, node string, vmid int) (map[string]interface{}, error) {
	if m == nil {
		return nil, fmt.Errorf("monitor not available")
	}
	if vmid <= 0 {
		return nil, fmt.Errorf("invalid vmid")
	}

	gt := strings.ToLower(strings.TrimSpace(guestType))
	if gt == "" {
		return nil, fmt.Errorf("guest type is required")
	}

	// Resolve instance/node from state if missing.
	if instance == "" || node == "" {
		m.mu.RLock()
		state := m.state
		m.mu.RUnlock()
		if state == nil {
			return nil, fmt.Errorf("state not available")
		}

		switch gt {
		case "container", "lxc":
			for _, ct := range state.Containers {
				if ct.VMID == vmid {
					if instance == "" {
						instance = ct.Instance
					}
					if node == "" {
						node = ct.Node
					}
					break
				}
			}
		case "vm":
			for _, vm := range state.VMs {
				if vm.VMID == vmid {
					if instance == "" {
						instance = vm.Instance
					}
					if node == "" {
						node = vm.Node
					}
					break
				}
			}
		default:
			return nil, fmt.Errorf("unsupported guest type: %s", guestType)
		}
	}

	if instance == "" || node == "" {
		return nil, fmt.Errorf("unable to resolve instance or node for guest")
	}

	m.mu.RLock()
	client := m.pveClients[instance]
	m.mu.RUnlock()
	if client == nil {
		return nil, fmt.Errorf("no PVE client for instance %s", instance)
	}

	switch gt {
	case "container", "lxc":
		return client.GetContainerConfig(ctx, node, vmid)
	case "vm":
		type vmConfigClient interface {
			GetVMConfig(ctx context.Context, node string, vmid int) (map[string]interface{}, error)
		}
		vmClient, ok := client.(vmConfigClient)
		if !ok {
			return nil, fmt.Errorf("VM config not supported by client")
		}
		return vmClient.GetVMConfig(ctx, node, vmid)
	default:
		return nil, fmt.Errorf("unsupported guest type: %s", guestType)
	}
}
