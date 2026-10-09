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

	// Resolve missing placement only from a unique current guest. A VMID is
	// unique within a PVE installation, not across every configured connection.
	if instance == "" || node == "" {
		m.mu.RLock()
		hasState := m.state != nil || m.resourceStore != nil
		m.mu.RUnlock()
		if !hasState {
			return nil, fmt.Errorf("state not available")
		}
		state := m.currentModeReadState()
		if state == nil {
			return nil, fmt.Errorf("state not available")
		}
		type placement struct{ instance, node string }
		matches := make(map[placement]bool)
		add := func(id int, candidateInstance, candidateNode string) {
			if id != vmid || instance != "" && instance != candidateInstance || node != "" && node != candidateNode {
				return
			}
			matches[placement{candidateInstance, candidateNode}] = true
		}
		switch gt {
		case "container", "lxc":
			for _, ct := range state.Containers() {
				if ct != nil {
					add(ct.VMID(), ct.Instance(), ct.Node())
				}
			}
		case "vm":
			for _, vm := range state.VMs() {
				if vm != nil {
					add(vm.VMID(), vm.Instance(), vm.Node())
				}
			}
		default:
			return nil, fmt.Errorf("unsupported guest type: %s", guestType)
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("guest placement is ambiguous; specify instance and node")
		}
		for match := range matches {
			instance, node = match.instance, match.node
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
