package unifiedresources

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

func metricsFromProxmoxNode(node models.Node) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	cpuPercent := percentFromUsage(node.CPU)
	metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceProxmox}

	if node.Memory.Total > 0 {
		percent := percentFromUsage(node.Memory.Usage)
		metrics.Memory = &MetricValue{Used: &node.Memory.Used, Total: &node.Memory.Total, Percent: percent, Unit: "bytes", Source: SourceProxmox}
	}
	if node.Disk.Total > 0 {
		percent := percentFromUsage(node.Disk.Usage)
		metrics.Disk = &MetricValue{Used: &node.Disk.Used, Total: &node.Disk.Total, Percent: percent, Unit: "bytes", Source: SourceProxmox}
	}
	return metrics
}

func metricsFromHost(host models.Host) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	cpuPercent := percentFromUsage(host.CPUUsage)
	metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceAgent}
	if host.Memory.Total > 0 {
		percent := percentFromUsage(host.Memory.Usage)
		metrics.Memory = &MetricValue{Used: &host.Memory.Used, Total: &host.Memory.Total, Percent: percent, Unit: "bytes", Source: SourceAgent}
	}
	if len(host.Disks) > 0 {
		disk := host.Disks[0]
		if disk.Total > 0 {
			percent := percentFromUsage(disk.Usage)
			metrics.Disk = &MetricValue{Used: &disk.Used, Total: &disk.Total, Percent: percent, Unit: "bytes", Source: SourceAgent}
		}
	}
	if host.NetInRate > 0 {
		metrics.NetIn = &MetricValue{Value: host.NetInRate, Unit: "bytes/s", Source: SourceAgent}
	}
	if host.NetOutRate > 0 {
		metrics.NetOut = &MetricValue{Value: host.NetOutRate, Unit: "bytes/s", Source: SourceAgent}
	}
	if host.DiskReadRate > 0 {
		metrics.DiskRead = &MetricValue{Value: host.DiskReadRate, Unit: "bytes/s", Source: SourceAgent}
	}
	if host.DiskWriteRate > 0 {
		metrics.DiskWrite = &MetricValue{Value: host.DiskWriteRate, Unit: "bytes/s", Source: SourceAgent}
	}
	return metrics
}

func metricsFromDockerHost(host models.DockerHost) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	cpuPercent := percentFromUsage(host.CPUUsage)
	metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceDocker}
	if host.Memory.Total > 0 {
		percent := percentFromUsage(host.Memory.Usage)
		metrics.Memory = &MetricValue{Used: &host.Memory.Used, Total: &host.Memory.Total, Percent: percent, Unit: "bytes", Source: SourceDocker}
	}
	if len(host.Disks) > 0 {
		disk := host.Disks[0]
		if disk.Total > 0 {
			percent := percentFromUsage(disk.Usage)
			metrics.Disk = &MetricValue{Used: &disk.Used, Total: &disk.Total, Percent: percent, Unit: "bytes", Source: SourceDocker}
		}
	}
	return metrics
}

func metricsFromVM(vm models.VM) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	cpuPercent := percentFromUsage(vm.CPU)
	metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceProxmox}
	if vm.Memory.Total > 0 {
		percent := percentFromUsage(vm.Memory.Usage)
		metrics.Memory = &MetricValue{Used: &vm.Memory.Used, Total: &vm.Memory.Total, Percent: percent, Unit: "bytes", Source: SourceProxmox}
	}
	if vm.Disk.Total > 0 {
		percent := percentFromUsage(vm.Disk.Usage)
		metrics.Disk = &MetricValue{Used: &vm.Disk.Used, Total: &vm.Disk.Total, Percent: percent, Unit: "bytes", Source: SourceProxmox}
	}
	if vm.NetworkIn != 0 {
		metrics.NetIn = &MetricValue{Value: float64(vm.NetworkIn), Unit: "bytes/s", Source: SourceProxmox}
	}
	if vm.NetworkOut != 0 {
		metrics.NetOut = &MetricValue{Value: float64(vm.NetworkOut), Unit: "bytes/s", Source: SourceProxmox}
	}
	if vm.DiskRead != 0 {
		metrics.DiskRead = &MetricValue{Value: float64(vm.DiskRead), Unit: "bytes/s", Source: SourceProxmox}
	}
	if vm.DiskWrite != 0 {
		metrics.DiskWrite = &MetricValue{Value: float64(vm.DiskWrite), Unit: "bytes/s", Source: SourceProxmox}
	}
	return metrics
}

func metricsFromContainer(ct models.Container) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	cpuPercent := percentFromUsage(ct.CPU)
	metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceProxmox}
	if ct.Memory.Total > 0 {
		percent := percentFromUsage(ct.Memory.Usage)
		metrics.Memory = &MetricValue{Used: &ct.Memory.Used, Total: &ct.Memory.Total, Percent: percent, Unit: "bytes", Source: SourceProxmox}
	}
	if ct.Disk.Total > 0 {
		percent := percentFromUsage(ct.Disk.Usage)
		metrics.Disk = &MetricValue{Used: &ct.Disk.Used, Total: &ct.Disk.Total, Percent: percent, Unit: "bytes", Source: SourceProxmox}
	}
	if ct.NetworkIn != 0 {
		metrics.NetIn = &MetricValue{Value: float64(ct.NetworkIn), Unit: "bytes/s", Source: SourceProxmox}
	}
	if ct.NetworkOut != 0 {
		metrics.NetOut = &MetricValue{Value: float64(ct.NetworkOut), Unit: "bytes/s", Source: SourceProxmox}
	}
	if ct.DiskRead != 0 {
		metrics.DiskRead = &MetricValue{Value: float64(ct.DiskRead), Unit: "bytes/s", Source: SourceProxmox}
	}
	if ct.DiskWrite != 0 {
		metrics.DiskWrite = &MetricValue{Value: float64(ct.DiskWrite), Unit: "bytes/s", Source: SourceProxmox}
	}
	return metrics
}

func metricsFromDockerContainer(ct models.DockerContainer) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	cpuPercent := percentFromUsage(ct.CPUPercent)
	metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceDocker}
	if ct.MemoryLimit > 0 {
		percent := percentFromUsage(ct.MemoryPercent)
		metrics.Memory = &MetricValue{Used: &ct.MemoryUsage, Total: &ct.MemoryLimit, Percent: percent, Unit: "bytes", Source: SourceDocker}
	}
	return metrics
}

func percentFromUsage(value float64) float64 {
	if value <= 1.0 {
		return value * 100
	}
	return value
}
