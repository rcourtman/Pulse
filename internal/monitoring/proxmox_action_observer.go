package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ProxmoxGuestObservation is a direct server-side Proxmox API read used by
// the governed action layer as an observer distinct from the node agent that
// executes guest lifecycle commands.
type ProxmoxGuestObservation struct {
	Instance   string
	Node       string
	VMID       int
	Kind       string
	Status     string
	Uptime     uint64
	ObservedAt time.Time
}

// ObserveProxmoxGuest reads current guest state through the configured
// Proxmox control-plane client. It does not use cached monitor state or the
// mutating node agent, so callers may classify a valid observation under the
// Proxmox control-plane trust domain.
func (m *Monitor) ObserveProxmoxGuest(ctx context.Context, instance, node string, vmid int, kind string) (ProxmoxGuestObservation, error) {
	if m == nil {
		return ProxmoxGuestObservation{}, fmt.Errorf("monitor unavailable")
	}
	instance = strings.TrimSpace(instance)
	node = strings.TrimSpace(node)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if instance == "" || node == "" || vmid <= 0 {
		return ProxmoxGuestObservation{}, fmt.Errorf("proxmox guest observer requires instance, node, and vmid")
	}
	client, ok := m.getPVEClient(instance)
	if !ok || client == nil {
		return ProxmoxGuestObservation{}, fmt.Errorf("proxmox client for instance %q unavailable", instance)
	}

	observation := ProxmoxGuestObservation{
		Instance: instance,
		Node:     node,
		VMID:     vmid,
		Kind:     kind,
	}
	switch kind {
	case "vm", "qemu":
		status, err := client.GetVMStatus(ctx, node, vmid)
		if err != nil {
			return ProxmoxGuestObservation{}, fmt.Errorf("observe Proxmox VM status: %w", err)
		}
		if status == nil {
			return ProxmoxGuestObservation{}, fmt.Errorf("observe Proxmox VM status: empty response")
		}
		observation.Kind = "vm"
		observation.Status = strings.ToLower(strings.TrimSpace(status.Status))
		observation.Uptime = status.Uptime
	case "ct", "lxc", "container":
		status, err := client.GetContainerStatus(ctx, node, vmid)
		if err != nil {
			return ProxmoxGuestObservation{}, fmt.Errorf("observe Proxmox CT status: %w", err)
		}
		if status == nil {
			return ProxmoxGuestObservation{}, fmt.Errorf("observe Proxmox CT status: empty response")
		}
		if observedVMID := int(status.VMID); observedVMID != 0 && observedVMID != vmid {
			return ProxmoxGuestObservation{}, fmt.Errorf("observe Proxmox CT status: vmid mismatch")
		}
		observation.Kind = "ct"
		observation.Status = strings.ToLower(strings.TrimSpace(status.Status))
		observation.Uptime = status.Uptime
	default:
		return ProxmoxGuestObservation{}, fmt.Errorf("unsupported Proxmox guest kind %q", kind)
	}
	if observation.Status == "" {
		return ProxmoxGuestObservation{}, fmt.Errorf("Proxmox guest observation omitted status")
	}
	observation.ObservedAt = time.Now().UTC()
	return observation, nil
}
