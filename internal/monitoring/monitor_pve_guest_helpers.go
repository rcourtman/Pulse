package monitoring

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/rs/zerolog/log"
)

func seedPrevContainerOCI(instanceName string, prevState models.StateSnapshot) map[int]bool {
	prevContainerIsOCI := make(map[int]bool)
	for _, ct := range prevState.Containers {
		if ct.Instance != instanceName {
			continue
		}
		if ct.VMID <= 0 {
			continue
		}
		if ct.Type == "oci" || ct.IsOCI {
			prevContainerIsOCI[ct.VMID] = true
		}
	}
	return prevContainerIsOCI
}

func (m *Monitor) preserveGuestsForGracePeriod(
	instanceName string,
	resources []proxmox.ClusterResource,
	prevState models.StateSnapshot,
	nodeEffectiveStatus map[string]string,
	allVMs []models.VM,
	allContainers []models.Container,
) ([]models.VM, []models.Container) {
	// Preserve VMs and containers from nodes within grace period
	// The cluster/resources endpoint doesn't return VMs/containers from nodes Proxmox considers offline,
	// but we want to keep showing them if the node is within grace period
	// Count previous resources for this instance
	prevVMCount := 0
	prevContainerCount := 0
	for _, vm := range prevState.VMs {
		if vm.Instance == instanceName {
			prevVMCount++
		}
	}
	for _, container := range prevState.Containers {
		if container.Instance == instanceName {
			prevContainerCount++
		}
	}

	// Build map of which nodes are covered by current resources
	nodesWithResources := make(map[string]bool)
	for _, res := range resources {
		nodesWithResources[res.Node] = true
	}

	log.Debug().
		Str("instance", instanceName).
		Int("nodesInResources", len(nodesWithResources)).
		Int("totalVMsFromResources", len(allVMs)).
		Int("totalContainersFromResources", len(allContainers)).
		Int("prevVMs", prevVMCount).
		Int("prevContainers", prevContainerCount).
		Msg("Cluster resources received, checking for grace period preservation")

	// If we got ZERO resources but had resources before, and we have no node data,
	// this likely means the cluster health check failed. Preserve everything.
	if len(allVMs) == 0 && len(allContainers) == 0 &&
		(prevVMCount > 0 || prevContainerCount > 0) &&
		len(nodeEffectiveStatus) == 0 {
		log.Warn().
			Str("instance", instanceName).
			Int("prevVMs", prevVMCount).
			Int("prevContainers", prevContainerCount).
			Msg("Cluster returned zero resources but had resources before - likely cluster health issue, preserving all previous resources")

		// Preserve all previous VMs and containers for this instance
		for _, vm := range prevState.VMs {
			if vm.Instance == instanceName {
				allVMs = append(allVMs, vm)
			}
		}
		for _, container := range prevState.Containers {
			if container.Instance == instanceName {
				allContainers = append(allContainers, container)
			}
		}
	}

	// Check for nodes that are within grace period but not in cluster/resources response
	preservedVMCount := 0
	preservedContainerCount := 0
	for nodeName, effectiveStatus := range nodeEffectiveStatus {
		if effectiveStatus == "online" && !nodesWithResources[nodeName] {
			// This node is within grace period but Proxmox didn't return its resources
			// Preserve previous VMs and containers from this node
			vmsBefore := len(allVMs)
			containersBefore := len(allContainers)

			// Preserve VMs from this node
			for _, vm := range prevState.VMs {
				if vm.Instance == instanceName && vm.Node == nodeName {
					allVMs = append(allVMs, vm)
				}
			}

			// Preserve containers from this node
			for _, container := range prevState.Containers {
				if container.Instance == instanceName && container.Node == nodeName {
					allContainers = append(allContainers, container)
				}
			}

			vmsPreserved := len(allVMs) - vmsBefore
			containersPreserved := len(allContainers) - containersBefore
			preservedVMCount += vmsPreserved
			preservedContainerCount += containersPreserved

			log.Info().
				Str("instance", instanceName).
				Str("node", nodeName).
				Int("vmsPreserved", vmsPreserved).
				Int("containersPreserved", containersPreserved).
				Msg("Preserved VMs/containers from node in grace period")
		}
	}

	if preservedVMCount > 0 || preservedContainerCount > 0 {
		log.Info().
			Str("instance", instanceName).
			Int("totalPreservedVMs", preservedVMCount).
			Int("totalPreservedContainers", preservedContainerCount).
			Msg("Grace period preservation complete")
	}

	return allVMs, allContainers
}

func (m *Monitor) recordGuestMetrics(allVMs []models.VM, allContainers []models.Container) {
	// Record guest metrics history for running guests (enables sparkline/trends view)
	now := time.Now()
	for _, vm := range allVMs {
		if vm.Status == "running" {
			m.metricsHistory.AddGuestMetric(vm.ID, "cpu", vm.CPU*100, now)
			m.metricsHistory.AddGuestMetric(vm.ID, "memory", vm.Memory.Usage, now)
			if vm.Disk.Usage >= 0 {
				m.metricsHistory.AddGuestMetric(vm.ID, "disk", vm.Disk.Usage, now)
			}
			if vm.DiskRead >= 0 {
				m.metricsHistory.AddGuestMetric(vm.ID, "diskread", float64(vm.DiskRead), now)
			}
			if vm.DiskWrite >= 0 {
				m.metricsHistory.AddGuestMetric(vm.ID, "diskwrite", float64(vm.DiskWrite), now)
			}
			if vm.NetworkIn >= 0 {
				m.metricsHistory.AddGuestMetric(vm.ID, "netin", float64(vm.NetworkIn), now)
			}
			if vm.NetworkOut >= 0 {
				m.metricsHistory.AddGuestMetric(vm.ID, "netout", float64(vm.NetworkOut), now)
			}
			// Also write to persistent store
			if m.metricsStore != nil {
				m.metricsStore.Write("vm", vm.ID, "cpu", vm.CPU*100, now)
				m.metricsStore.Write("vm", vm.ID, "memory", vm.Memory.Usage, now)
				if vm.Disk.Usage >= 0 {
					m.metricsStore.Write("vm", vm.ID, "disk", vm.Disk.Usage, now)
				}
				if vm.DiskRead >= 0 {
					m.metricsStore.Write("vm", vm.ID, "diskread", float64(vm.DiskRead), now)
				}
				if vm.DiskWrite >= 0 {
					m.metricsStore.Write("vm", vm.ID, "diskwrite", float64(vm.DiskWrite), now)
				}
				if vm.NetworkIn >= 0 {
					m.metricsStore.Write("vm", vm.ID, "netin", float64(vm.NetworkIn), now)
				}
				if vm.NetworkOut >= 0 {
					m.metricsStore.Write("vm", vm.ID, "netout", float64(vm.NetworkOut), now)
				}
			}
		}
	}
	for _, ct := range allContainers {
		if ct.Status == "running" {
			m.metricsHistory.AddGuestMetric(ct.ID, "cpu", ct.CPU*100, now)
			m.metricsHistory.AddGuestMetric(ct.ID, "memory", ct.Memory.Usage, now)
			if ct.Disk.Usage >= 0 {
				m.metricsHistory.AddGuestMetric(ct.ID, "disk", ct.Disk.Usage, now)
			}
			if ct.DiskRead >= 0 {
				m.metricsHistory.AddGuestMetric(ct.ID, "diskread", float64(ct.DiskRead), now)
			}
			if ct.DiskWrite >= 0 {
				m.metricsHistory.AddGuestMetric(ct.ID, "diskwrite", float64(ct.DiskWrite), now)
			}
			if ct.NetworkIn >= 0 {
				m.metricsHistory.AddGuestMetric(ct.ID, "netin", float64(ct.NetworkIn), now)
			}
			if ct.NetworkOut >= 0 {
				m.metricsHistory.AddGuestMetric(ct.ID, "netout", float64(ct.NetworkOut), now)
			}
			// Also write to persistent store
			if m.metricsStore != nil {
				m.metricsStore.Write("container", ct.ID, "cpu", ct.CPU*100, now)
				m.metricsStore.Write("container", ct.ID, "memory", ct.Memory.Usage, now)
				if ct.Disk.Usage >= 0 {
					m.metricsStore.Write("container", ct.ID, "disk", ct.Disk.Usage, now)
				}
				if ct.DiskRead >= 0 {
					m.metricsStore.Write("container", ct.ID, "diskread", float64(ct.DiskRead), now)
				}
				if ct.DiskWrite >= 0 {
					m.metricsStore.Write("container", ct.ID, "diskwrite", float64(ct.DiskWrite), now)
				}
				if ct.NetworkIn >= 0 {
					m.metricsStore.Write("container", ct.ID, "netin", float64(ct.NetworkIn), now)
				}
				if ct.NetworkOut >= 0 {
					m.metricsStore.Write("container", ct.ID, "netout", float64(ct.NetworkOut), now)
				}
			}
		}
	}
}
