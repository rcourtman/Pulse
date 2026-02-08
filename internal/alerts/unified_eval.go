package alerts

import (
	"github.com/rs/zerolog/log"
)

// Override Key Format by Resource Type
//
// The config.Overrides map is keyed by resource ID. Each resource type
// uses the following canonical key format:
//
//	VM (Qemu):         "qemu-{VMID}"       e.g. "qemu-100"
//	Container (LXC):   "lxc-{VMID}"        e.g. "lxc-200"
//	Node:              node.ID             e.g. "node/pve-1"
//	Host (Agent):      host.ID             e.g. "host1" (without "host:" prefix)
//	Host Disk:         "host:{hostID}/disk:{mountpoint}" e.g. "host:host1/disk:root"
//	Storage:           storage.ID          e.g. "local-lvm"
//	PBS:               pbs.ID              e.g. "pbs-1"
//	PMG:               pmg.ID              e.g. "pmg-1"
//	Docker Container:  "docker:{hostID}/{containerID}"
//
// Legacy guest formats ("qemu-{node}-{VMID}", "{node}-{VMID}") are
// auto-migrated on access in getGuestThresholds (see filter_evaluation.go).
//
// CheckUnifiedResource looks up overrides by input.ID, which must match
// the canonical key format for the resource type.
// UnifiedResourceMetric holds a single metric value for unified evaluation.
type UnifiedResourceMetric struct {
	Value   float64
	Percent float64
}

// UnifiedResourceInput is the data needed to evaluate alerts for a unified resource.
// This avoids importing unifiedresources (which would cause an import cycle).
type UnifiedResourceInput struct {
	ID         string
	Type       string // lowercase: "vm", "lxc", "container", "host", "pbs", "storage", "pmg"
	Name       string
	Node       string
	Instance   string
	CPU        *UnifiedResourceMetric
	Memory     *UnifiedResourceMetric
	Disk       *UnifiedResourceMetric
	DiskRead   *UnifiedResourceMetric
	DiskWrite  *UnifiedResourceMetric
	NetworkIn  *UnifiedResourceMetric
	NetworkOut *UnifiedResourceMetric
}

// unifiedAlertType maps a resource type key to the alert system's display type.
func unifiedAlertType(typeKey string) string {
	switch typeKey {
	case "vm":
		return "VM"
	case "lxc", "container":
		return "Container"
	case "host":
		return "Host"
	case "node":
		return "Node"
	case "pbs":
		return "PBS"
	case "storage":
		return "Storage"
	case "pmg":
		return "PMG"
	default:
		return typeKey
	}
}

// isUnifiedGuestType returns true for resource types that support I/O metrics.
func isUnifiedGuestType(typeKey string) bool {
	switch typeKey {
	case "vm", "lxc", "container":
		return true
	default:
		return false
	}
}

// unifiedDefaultThresholds returns the default ThresholdConfig for a resource type key.
func (m *Manager) unifiedDefaultThresholds(typeKey string) ThresholdConfig {
	switch typeKey {
	case "vm", "lxc", "container":
		return cloneThresholdConfig(m.config.GuestDefaults)
	case "host":
		return cloneThresholdConfig(m.config.HostDefaults)
	case "node":
		return cloneThresholdConfig(m.config.NodeDefaults)
	case "pbs":
		return cloneThresholdConfig(m.config.PBSDefaults)
	case "storage":
		return ThresholdConfig{Usage: cloneThreshold(&m.config.StorageDefault)}
	default:
		return ThresholdConfig{}
	}
}

// evaluateUnifiedMetrics runs the common metric dispatch path for unified resources.
func (m *Manager) evaluateUnifiedMetrics(input *UnifiedResourceInput, thresholds ThresholdConfig, opts *metricOptions) {
	if input == nil {
		return
	}
	resourceType := unifiedAlertType(input.Type)

	if input.CPU != nil {
		m.checkMetric(input.ID, input.Name, input.Node, input.Instance, resourceType, "cpu", input.CPU.Percent, thresholds.CPU, opts)
	}
	if input.Memory != nil {
		m.checkMetric(input.ID, input.Name, input.Node, input.Instance, resourceType, "memory", input.Memory.Percent, thresholds.Memory, opts)
	}
	if input.Disk != nil {
		m.checkMetric(input.ID, input.Name, input.Node, input.Instance, resourceType, "disk", input.Disk.Percent, thresholds.Disk, opts)
	}

	// I/O metrics — only for guest resource types
	if isUnifiedGuestType(input.Type) {
		if input.DiskRead != nil {
			m.checkMetric(input.ID, input.Name, input.Node, input.Instance, resourceType, "diskRead", input.DiskRead.Value, thresholds.DiskRead, opts)
		}
		if input.DiskWrite != nil {
			m.checkMetric(input.ID, input.Name, input.Node, input.Instance, resourceType, "diskWrite", input.DiskWrite.Value, thresholds.DiskWrite, opts)
		}
		if input.NetworkIn != nil {
			m.checkMetric(input.ID, input.Name, input.Node, input.Instance, resourceType, "networkIn", input.NetworkIn.Value, thresholds.NetworkIn, opts)
		}
		if input.NetworkOut != nil {
			m.checkMetric(input.ID, input.Name, input.Node, input.Instance, resourceType, "networkOut", input.NetworkOut.Value, thresholds.NetworkOut, opts)
		}
	}

	// Storage-specific: usage metric
	if input.Type == "storage" && input.Disk != nil {
		m.checkMetric(input.ID, input.Name, input.Node, input.Instance, resourceType, "usage", input.Disk.Percent, thresholds.Usage, opts)
	}
}

// CheckUnifiedResource evaluates threshold-based metric alerts for a unified resource.
// It resolves thresholds (defaults + overrides) and calls checkMetric for each
// available metric. Discrete event alerts (offline, RAID, backup age, etc.)
// are NOT evaluated here — they remain in the typed Check* methods.
func (m *Manager) CheckUnifiedResource(input *UnifiedResourceInput) {
	if input == nil {
		return
	}

	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return
	}

	thresholds := m.unifiedDefaultThresholds(input.Type)
	if override, exists := m.config.Overrides[input.ID]; exists {
		thresholds = m.applyThresholdOverride(thresholds, override)
	}
	m.mu.RUnlock()

	if thresholds.Disabled {
		return
	}

	log.Debug().
		Str("resourceID", input.ID).
		Str("resourceName", input.Name).
		Str("resourceType", unifiedAlertType(input.Type)).
		Msg("Evaluating unified resource metrics")

	m.evaluateUnifiedMetrics(input, thresholds, nil)
}
