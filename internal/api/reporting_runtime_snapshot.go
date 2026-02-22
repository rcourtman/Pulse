package api

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/extensions"
)

func (h *ReportingHandlers) getRuntimeStateSnapshot(ctx context.Context, orgID string) (extensions.ReportingStateSnapshot, bool) {
	_ = ctx
	if h == nil || h.mtMonitor == nil {
		return extensions.ReportingStateSnapshot{}, false
	}

	monitor, err := h.mtMonitor.GetMonitor(orgID)
	if err != nil || monitor == nil {
		return extensions.ReportingStateSnapshot{}, false
	}

	state := monitor.GetState()
	snapshot := extensions.ReportingStateSnapshot{
		Nodes:          make([]extensions.ReportingNodeSnapshot, 0, len(state.Nodes)),
		VMs:            make([]extensions.ReportingVMSnapshot, 0, len(state.VMs)),
		Containers:     make([]extensions.ReportingContainerSnapshot, 0, len(state.Containers)),
		ActiveAlerts:   make([]extensions.ReportingAlertSnapshot, 0, len(state.ActiveAlerts)),
		ResolvedAlerts: make([]extensions.ReportingAlertSnapshot, 0, len(state.RecentlyResolved)),
		LegacyBackups:  make([]extensions.ReportingLegacyBackupSnapshot, 0, len(state.PVEBackups.StorageBackups)),
	}

	for _, node := range state.Nodes {
		var temperature *float64
		if node.Temperature != nil && node.Temperature.CPUPackage > 0 {
			value := node.Temperature.CPUPackage
			temperature = &value
		}

		snapshot.Nodes = append(snapshot.Nodes, extensions.ReportingNodeSnapshot{
			ID:            node.ID,
			Name:          node.Name,
			DisplayName:   node.DisplayName,
			Status:        node.Status,
			Host:          node.Host,
			Instance:      node.Instance,
			Uptime:        node.Uptime,
			KernelVersion: node.KernelVersion,
			PVEVersion:    node.PVEVersion,
			CPUModel:      node.CPUInfo.Model,
			CPUCores:      node.CPUInfo.Cores,
			CPUSockets:    node.CPUInfo.Sockets,
			MemoryTotal:   node.Memory.Total,
			DiskTotal:     node.Disk.Total,
			LoadAverage:   append([]float64(nil), node.LoadAverage...),
			ClusterName:   node.ClusterName,
			IsCluster:     node.IsClusterMember,
			Temperature:   temperature,
		})
	}

	for _, vm := range state.VMs {
		snapshot.VMs = append(snapshot.VMs, extensions.ReportingVMSnapshot{
			ID:          vm.ID,
			VMID:        vm.VMID,
			Name:        vm.Name,
			Status:      vm.Status,
			Node:        vm.Node,
			Instance:    vm.Instance,
			Uptime:      vm.Uptime,
			OSName:      vm.OSName,
			OSVersion:   vm.OSVersion,
			IPAddresses: append([]string(nil), vm.IPAddresses...),
			CPUCores:    vm.CPUs,
			MemoryTotal: vm.Memory.Total,
			DiskTotal:   vm.Disk.Total,
			Tags:        append([]string(nil), vm.Tags...),
		})
	}

	for _, ct := range state.Containers {
		snapshot.Containers = append(snapshot.Containers, extensions.ReportingContainerSnapshot{
			ID:          ct.ID,
			VMID:        ct.VMID,
			Name:        ct.Name,
			Status:      ct.Status,
			Node:        ct.Node,
			Instance:    ct.Instance,
			Uptime:      ct.Uptime,
			OSName:      ct.OSName,
			IPAddresses: append([]string(nil), ct.IPAddresses...),
			CPUCores:    ct.CPUs,
			MemoryTotal: ct.Memory.Total,
			DiskTotal:   ct.Disk.Total,
			Tags:        append([]string(nil), ct.Tags...),
		})
	}

	for _, alert := range state.ActiveAlerts {
		snapshot.ActiveAlerts = append(snapshot.ActiveAlerts, extensions.ReportingAlertSnapshot{
			ResourceID: alert.ResourceID,
			Node:       alert.Node,
			Type:       alert.Type,
			Level:      alert.Level,
			Message:    alert.Message,
			Value:      alert.Value,
			Threshold:  alert.Threshold,
			StartTime:  alert.StartTime,
		})
	}

	for _, resolved := range state.RecentlyResolved {
		resolvedTime := resolved.ResolvedTime
		snapshot.ResolvedAlerts = append(snapshot.ResolvedAlerts, extensions.ReportingAlertSnapshot{
			ResourceID:   resolved.ResourceID,
			Node:         resolved.Node,
			Type:         resolved.Type,
			Level:        resolved.Level,
			Message:      resolved.Message,
			Value:        resolved.Value,
			Threshold:    resolved.Threshold,
			StartTime:    resolved.StartTime,
			ResolvedTime: &resolvedTime,
		})
	}

	for _, backup := range state.PVEBackups.StorageBackups {
		snapshot.LegacyBackups = append(snapshot.LegacyBackups, extensions.ReportingLegacyBackupSnapshot{
			VMID:      backup.VMID,
			Node:      backup.Node,
			Storage:   backup.Storage,
			Timestamp: backup.Time,
			Size:      backup.Size,
			Protected: backup.Protected,
			VolID:     backup.Volid,
		})
	}

	if h.resourceRegistry == nil {
		return snapshot, true
	}

	for _, resource := range h.resourceRegistry.List() {
		switch resource.Type {
		case unifiedresources.ResourceTypeStorage:
			storageNode := resource.ParentName
			if storageNode == "" && len(resource.Identity.Hostnames) > 0 {
				storageNode = resource.Identity.Hostnames[0]
			}

			var total, used, available int64
			var usagePerc float64
			if resource.Metrics != nil && resource.Metrics.Disk != nil {
				if resource.Metrics.Disk.Total != nil {
					total = *resource.Metrics.Disk.Total
				}
				if resource.Metrics.Disk.Used != nil {
					used = *resource.Metrics.Disk.Used
				}
				if total > 0 {
					available = total - used
				}
				usagePerc = resource.Metrics.Disk.Percent
				if usagePerc == 0 && total > 0 {
					usagePerc = (float64(used) / float64(total)) * 100
				}
			}

			var storageType, content string
			if resource.Storage != nil {
				storageType = resource.Storage.Type
				content = resource.Storage.Content
			}

			snapshot.Storage = append(snapshot.Storage, extensions.ReportingStorageSnapshot{
				Name:      resource.Name,
				Node:      storageNode,
				Type:      storageType,
				Status:    string(resource.Status),
				Total:     total,
				Used:      used,
				Available: available,
				UsagePerc: usagePerc,
				Content:   content,
			})

		case unifiedresources.ResourceTypePhysicalDisk:
			if resource.PhysicalDisk == nil {
				continue
			}

			diskNode := resource.ParentName
			if diskNode == "" && len(resource.Identity.Hostnames) > 0 {
				diskNode = resource.Identity.Hostnames[0]
			}

			disk := resource.PhysicalDisk
			snapshot.Disks = append(snapshot.Disks, extensions.ReportingDiskSnapshot{
				Node:        diskNode,
				Device:      disk.DevPath,
				Model:       disk.Model,
				Serial:      disk.Serial,
				Type:        disk.DiskType,
				Size:        disk.SizeBytes,
				Health:      disk.Health,
				Temperature: disk.Temperature,
				WearLevel:   disk.Wearout,
			})
		}
	}

	return snapshot, true
}
