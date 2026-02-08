package unifiedresources

import (
	"hash/fnv"
	"math"
	"math/rand"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

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
	if host.NetInRate > 0 {
		metrics.NetIn = &MetricValue{Value: host.NetInRate, Unit: "bytes/s", Source: SourceDocker}
	}
	if host.NetOutRate > 0 {
		metrics.NetOut = &MetricValue{Value: host.NetOutRate, Unit: "bytes/s", Source: SourceDocker}
	}
	if host.DiskReadRate > 0 {
		metrics.DiskRead = &MetricValue{Value: host.DiskReadRate, Unit: "bytes/s", Source: SourceDocker}
	}
	if host.DiskWriteRate > 0 {
		metrics.DiskWrite = &MetricValue{Value: host.DiskWriteRate, Unit: "bytes/s", Source: SourceDocker}
	}
	return metrics
}

func metricsFromPBSInstance(instance models.PBSInstance) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	cpuPercent := percentFromUsage(instance.CPU)
	metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourcePBS}
	if instance.MemoryTotal > 0 {
		percent := percentFromUsage(instance.Memory)
		metrics.Memory = &MetricValue{
			Used:    &instance.MemoryUsed,
			Total:   &instance.MemoryTotal,
			Percent: percent,
			Unit:    "bytes",
			Source:  SourcePBS,
		}
	}
	return metrics
}

func metricsFromPMGInstance(instance models.PMGInstance) *ResourceMetrics {
	// PMG currently exposes aggregate queue/mail counters, not point-in-time
	// throughput rates. Return an empty metrics object to avoid mislabeling
	// aggregate totals as realtime I/O.
	return &ResourceMetrics{}
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

func metricsFromStorage(storage models.Storage) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	if storage.Total <= 0 {
		return metrics
	}

	used := storage.Used
	total := storage.Total
	percent := percentFromUsage(storage.Usage)
	if percent == 0 && total > 0 {
		percent = clampMetricValue((float64(used)/float64(total))*100, 0, 100)
	}
	metrics.Disk = &MetricValue{
		Used:    &used,
		Total:   &total,
		Percent: percent,
		Unit:    "bytes",
		Source:  SourceProxmox,
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
	if ct.RootFilesystemBytes > 0 {
		used := ct.WritableLayerBytes
		if used < 0 {
			used = 0
		}
		if used > ct.RootFilesystemBytes {
			used = ct.RootFilesystemBytes
		}
		percent := clampMetricValue((float64(used)/float64(ct.RootFilesystemBytes))*100, 0, 100)
		metrics.Disk = &MetricValue{Used: &used, Total: &ct.RootFilesystemBytes, Percent: percent, Unit: "bytes", Source: SourceDocker}
	}
	if ct.NetInRate > 0 {
		metrics.NetIn = &MetricValue{Value: ct.NetInRate, Unit: "bytes/s", Source: SourceDocker}
	}
	if ct.NetOutRate > 0 {
		metrics.NetOut = &MetricValue{Value: ct.NetOutRate, Unit: "bytes/s", Source: SourceDocker}
	}
	if ct.BlockIO != nil {
		if ct.BlockIO.ReadRateBytesPerSecond != nil && *ct.BlockIO.ReadRateBytesPerSecond > 0 {
			metrics.DiskRead = &MetricValue{Value: *ct.BlockIO.ReadRateBytesPerSecond, Unit: "bytes/s", Source: SourceDocker}
		}
		if ct.BlockIO.WriteRateBytesPerSecond != nil && *ct.BlockIO.WriteRateBytesPerSecond > 0 {
			metrics.DiskWrite = &MetricValue{Value: *ct.BlockIO.WriteRateBytesPerSecond, Unit: "bytes/s", Source: SourceDocker}
		}
	}
	if mock.IsMockEnabled() && (metrics.NetIn == nil || metrics.NetOut == nil || metrics.DiskRead == nil || metrics.DiskWrite == nil) {
		synthetic := syntheticDockerContainerIOMetrics(ct)
		if metrics.NetIn == nil {
			metrics.NetIn = &MetricValue{Value: synthetic.NetIn, Unit: "bytes/s", Source: SourceDocker}
		}
		if metrics.NetOut == nil {
			metrics.NetOut = &MetricValue{Value: synthetic.NetOut, Unit: "bytes/s", Source: SourceDocker}
		}
		if metrics.DiskRead == nil {
			metrics.DiskRead = &MetricValue{Value: synthetic.DiskRead, Unit: "bytes/s", Source: SourceDocker}
		}
		if metrics.DiskWrite == nil {
			metrics.DiskWrite = &MetricValue{Value: synthetic.DiskWrite, Unit: "bytes/s", Source: SourceDocker}
		}
	}
	return metrics
}

func metricsFromKubernetesCluster(cluster models.KubernetesCluster, linkedHosts []*models.Host) *ResourceMetrics {
	metrics := &ResourceMetrics{}

	var cpuSum float64
	var cpuCount int

	var memoryUsed int64
	var memoryTotal int64

	var diskUsed int64
	var diskTotal int64

	var netIn float64
	var netOut float64
	var diskRead float64
	var diskWrite float64
	var hasNetIn bool
	var hasNetOut bool
	var hasDiskRead bool
	var hasDiskWrite bool

	linkedNodeNames := make(map[string]struct{}, len(linkedHosts)*2)
	for _, host := range linkedHosts {
		if host == nil {
			continue
		}

		cpuSum += percentFromUsage(host.CPUUsage)
		cpuCount++

		hostName := strings.TrimSpace(host.Hostname)
		if hostName != "" {
			linkedNodeNames[strings.ToLower(hostName)] = struct{}{}
			linkedNodeNames[strings.ToLower(NormalizeHostname(hostName))] = struct{}{}
		}

		if host.Memory.Total > 0 {
			memoryTotal += host.Memory.Total
			memoryUsed += host.Memory.Used
		}

		if len(host.Disks) > 0 {
			disk := host.Disks[0]
			if disk.Total > 0 {
				diskTotal += disk.Total
				diskUsed += disk.Used
			}
		}

		if host.NetInRate > 0 {
			netIn += host.NetInRate
			hasNetIn = true
		}
		if host.NetOutRate > 0 {
			netOut += host.NetOutRate
			hasNetOut = true
		}
		if host.DiskReadRate > 0 {
			diskRead += host.DiskReadRate
			hasDiskRead = true
		}
		if host.DiskWriteRate > 0 {
			diskWrite += host.DiskWriteRate
			hasDiskWrite = true
		}
	}

	// Fall back to Kubernetes metrics API usage for nodes without a linked host-agent.
	for _, node := range cluster.Nodes {
		nodeName := strings.TrimSpace(node.Name)
		normalized := strings.ToLower(NormalizeHostname(nodeName))
		if _, linked := linkedNodeNames[strings.ToLower(nodeName)]; linked {
			continue
		}
		if normalized != "" {
			if _, linked := linkedNodeNames[normalized]; linked {
				continue
			}
		}

		if node.UsageCPUPercent > 0 {
			cpuSum += clampMetricValue(node.UsageCPUPercent, 0, 100)
			cpuCount++
		}
		if node.UsageMemoryBytes > 0 {
			total := node.AllocMemoryBytes
			if total <= 0 {
				total = node.CapacityMemoryBytes
			}
			if total > 0 {
				memoryUsed += node.UsageMemoryBytes
				memoryTotal += total
			}
		}
	}

	if cpuCount > 0 {
		cpuPercent := clampMetricValue(cpuSum/float64(cpuCount), 0, 100)
		metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceAgent}
	}
	if memoryTotal > 0 {
		percent := clampMetricValue((float64(memoryUsed)/float64(memoryTotal))*100, 0, 100)
		metrics.Memory = &MetricValue{Used: &memoryUsed, Total: &memoryTotal, Percent: percent, Unit: "bytes", Source: SourceAgent}
	}
	if diskTotal > 0 {
		percent := clampMetricValue((float64(diskUsed)/float64(diskTotal))*100, 0, 100)
		metrics.Disk = &MetricValue{Used: &diskUsed, Total: &diskTotal, Percent: percent, Unit: "bytes", Source: SourceAgent}
	}
	if hasNetIn {
		metrics.NetIn = &MetricValue{Value: netIn, Unit: "bytes/s", Source: SourceAgent}
	}
	if hasNetOut {
		metrics.NetOut = &MetricValue{Value: netOut, Unit: "bytes/s", Source: SourceAgent}
	}
	if hasDiskRead {
		metrics.DiskRead = &MetricValue{Value: diskRead, Unit: "bytes/s", Source: SourceAgent}
	}
	if hasDiskWrite {
		metrics.DiskWrite = &MetricValue{Value: diskWrite, Unit: "bytes/s", Source: SourceAgent}
	}
	return metrics
}

func metricsFromKubernetesNode(_ models.KubernetesCluster, node models.KubernetesNode, linkedHost *models.Host) *ResourceMetrics {
	// Kubernetes node usage comes from the host-agent module when running on the
	// same infrastructure node (unified agent). If no linked host exists, use
	// Kubernetes metrics API usage fields when available.
	if linkedHost != nil {
		return metricsFromHost(*linkedHost)
	}

	metrics := &ResourceMetrics{}
	if node.UsageCPUPercent > 0 {
		cpuPercent := clampMetricValue(node.UsageCPUPercent, 0, 100)
		metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceK8s}
	}
	if node.UsageMemoryBytes > 0 {
		total := node.AllocMemoryBytes
		if total <= 0 {
			total = node.CapacityMemoryBytes
		}
		if total > 0 {
			used := node.UsageMemoryBytes
			percent := clampMetricValue((float64(used)/float64(total))*100, 0, 100)
			metrics.Memory = &MetricValue{Used: &used, Total: &total, Percent: percent, Unit: "bytes", Source: SourceK8s}
		}
	}
	return metrics
}

func metricsFromKubernetesPod(cluster models.KubernetesCluster, pod models.KubernetesPod) *ResourceMetrics {
	metrics := &ResourceMetrics{}
	if pod.UsageCPUPercent > 0 {
		cpuPercent := clampMetricValue(pod.UsageCPUPercent, 0, 100)
		metrics.CPU = &MetricValue{Value: cpuPercent, Percent: cpuPercent, Unit: "percent", Source: SourceK8s}
	}
	if pod.UsageMemoryBytes > 0 {
		total := kubernetesPodMemoryTotalBytes(cluster, pod)
		if total > 0 {
			used := pod.UsageMemoryBytes
			percent := clampMetricValue((float64(used)/float64(total))*100, 0, 100)
			metrics.Memory = &MetricValue{Used: &used, Total: &total, Percent: percent, Unit: "bytes", Source: SourceK8s}
		} else if pod.UsageMemoryPercent > 0 {
			memPercent := clampMetricValue(pod.UsageMemoryPercent, 0, 100)
			metrics.Memory = &MetricValue{Value: memPercent, Percent: memPercent, Unit: "percent", Source: SourceK8s}
		}
	}
	if pod.DiskUsagePercent > 0 {
		diskPercent := clampMetricValue(pod.DiskUsagePercent, 0, 100)
		metrics.Disk = &MetricValue{Value: diskPercent, Percent: diskPercent, Unit: "percent", Source: SourceK8s}
	}
	if pod.NetInRate > 0 {
		metrics.NetIn = &MetricValue{Value: pod.NetInRate, Unit: "bytes/s", Source: SourceK8s}
	}
	if pod.NetOutRate > 0 {
		metrics.NetOut = &MetricValue{Value: pod.NetOutRate, Unit: "bytes/s", Source: SourceK8s}
	}

	if !mock.IsMockEnabled() {
		return metrics
	}

	values := syntheticKubernetesPodMetrics(cluster, pod)
	if metrics.CPU == nil {
		metrics.CPU = &MetricValue{Value: values.CPU, Percent: values.CPU, Unit: "percent", Source: SourceK8s}
	}
	if metrics.Memory == nil {
		metrics.Memory = &MetricValue{Value: values.Memory, Percent: values.Memory, Unit: "percent", Source: SourceK8s}
	}
	if metrics.Disk == nil {
		metrics.Disk = &MetricValue{Value: values.Disk, Percent: values.Disk, Unit: "percent", Source: SourceK8s}
	}
	if metrics.NetIn == nil {
		metrics.NetIn = &MetricValue{Value: values.NetIn, Unit: "bytes/s", Source: SourceK8s}
	}
	if metrics.NetOut == nil {
		metrics.NetOut = &MetricValue{Value: values.NetOut, Unit: "bytes/s", Source: SourceK8s}
	}
	return metrics
}

func kubernetesPodMemoryTotalBytes(cluster models.KubernetesCluster, pod models.KubernetesPod) int64 {
	nodeName := strings.TrimSpace(pod.NodeName)
	if nodeName == "" {
		return 0
	}
	for _, node := range cluster.Nodes {
		if !strings.EqualFold(strings.TrimSpace(node.Name), nodeName) {
			continue
		}
		if node.AllocMemoryBytes > 0 {
			return node.AllocMemoryBytes
		}
		if node.CapacityMemoryBytes > 0 {
			return node.CapacityMemoryBytes
		}
		return 0
	}
	return 0
}

type kubernetesPodSyntheticMetrics struct {
	CPU       float64
	Memory    float64
	Disk      float64
	NetIn     float64
	NetOut    float64
	DiskRead  float64
	DiskWrite float64
}

type dockerContainerSyntheticIOMetrics struct {
	NetIn     float64
	NetOut    float64
	DiskRead  float64
	DiskWrite float64
}

func syntheticDockerContainerIOMetrics(ct models.DockerContainer) dockerContainerSyntheticIOMetrics {
	seed := hashDockerMetricsSeed(ct.ID, ct.Name, ct.Image, ct.State)
	rng := rand.New(rand.NewSource(int64(seed)))

	state := strings.ToLower(strings.TrimSpace(ct.State))
	running := state == "running" || state == "restarting"
	activity := clampMetricValue((percentFromUsage(ct.CPUPercent)+percentFromUsage(ct.MemoryPercent))/180.0, 0.12, 1.25)
	if !running {
		activity = clampMetricValue(activity*0.32, 0.05, 0.45)
	}

	restartMultiplier := 1 + math.Min(float64(ct.RestartCount)*0.08, 0.55)
	netIn := (28 + activity*540 + rng.Float64()*210) * restartMultiplier
	netOut := (22 + activity*470 + rng.Float64()*180) * restartMultiplier
	diskRead := (16 + activity*320 + rng.Float64()*140) * restartMultiplier
	diskWrite := (14 + activity*300 + rng.Float64()*130) * restartMultiplier

	return dockerContainerSyntheticIOMetrics{
		NetIn:     clampMetricValue(netIn, 4, 3200),
		NetOut:    clampMetricValue(netOut, 3, 2800),
		DiskRead:  clampMetricValue(diskRead, 2, 2400),
		DiskWrite: clampMetricValue(diskWrite, 2, 2200),
	}
}

func syntheticKubernetesPodMetrics(cluster models.KubernetesCluster, pod models.KubernetesPod) kubernetesPodSyntheticMetrics {
	seed := hashKubernetesMetricsSeed(
		cluster.ID,
		cluster.Name,
		cluster.DisplayName,
		pod.UID,
		pod.Namespace,
		pod.Name,
	)
	rng := rand.New(rand.NewSource(int64(seed)))

	phase := strings.ToLower(strings.TrimSpace(pod.Phase))
	totalContainers := len(pod.Containers)
	if totalContainers <= 0 {
		totalContainers = 1
	}
	readyContainers := 0
	for _, container := range pod.Containers {
		if container.Ready {
			readyContainers++
		}
	}
	readiness := float64(readyContainers) / float64(totalContainers)
	if readiness <= 0 && phase == "running" {
		readiness = 0.35
	}

	restarts := float64(pod.Restarts)
	if restarts < 0 {
		restarts = 0
	}
	restartFactor := math.Min(restarts*1.6, 16)

	cpu := 0.0
	memory := 0.0
	disk := 0.0
	netIn := 0.0
	netOut := 0.0
	diskRead := 0.0
	diskWrite := 0.0

	switch phase {
	case "running":
		cpu = 7 + readiness*52 + rng.Float64()*14 - restartFactor*0.35
		memory = 26 + readiness*46 + rng.Float64()*14 + restartFactor*0.25
		disk = 17 + readiness*42 + rng.Float64()*16
		netIn = 14 + readiness*220 + rng.Float64()*70 + restarts*2
		netOut = 10 + readiness*180 + rng.Float64()*55 + restarts*1.6
		diskRead = 8 + readiness*75 + rng.Float64()*32 + restarts*1.8
		diskWrite = 6 + readiness*68 + rng.Float64()*28 + restarts*1.6
	case "pending":
		cpu = 2 + rng.Float64()*7
		memory = 14 + rng.Float64()*16
		disk = 8 + rng.Float64()*14
		netIn = 1 + rng.Float64()*10
		netOut = 1 + rng.Float64()*8
		diskRead = 1 + rng.Float64()*8
		diskWrite = 1 + rng.Float64()*8
	case "failed", "unknown":
		cpu = 1 + rng.Float64()*8
		memory = 9 + rng.Float64()*15 + restartFactor*0.5
		disk = 7 + rng.Float64()*16
		netIn = 1 + rng.Float64()*14 + restarts*1.4
		netOut = 1 + rng.Float64()*11 + restarts*1.2
		diskRead = 1 + rng.Float64()*10 + restarts*1.2
		diskWrite = 1 + rng.Float64()*10 + restarts*1.1
	default:
		cpu = 1 + rng.Float64()*6
		memory = 8 + rng.Float64()*12
		disk = 6 + rng.Float64()*12
		netIn = 1 + rng.Float64()*7
		netOut = 1 + rng.Float64()*6
		diskRead = 1 + rng.Float64()*6
		diskWrite = 1 + rng.Float64()*6
	}

	if totalContainers > 1 {
		multi := 1 + math.Min(float64(totalContainers-1)*0.08, 0.6)
		cpu *= multi
		memory *= 1 + math.Min(float64(totalContainers-1)*0.1, 0.8)
		disk *= 1 + math.Min(float64(totalContainers-1)*0.07, 0.5)
		netIn *= 1 + math.Min(float64(totalContainers-1)*0.09, 0.7)
		netOut *= 1 + math.Min(float64(totalContainers-1)*0.08, 0.65)
		diskRead *= 1 + math.Min(float64(totalContainers-1)*0.11, 0.8)
		diskWrite *= 1 + math.Min(float64(totalContainers-1)*0.1, 0.7)
	}

	return kubernetesPodSyntheticMetrics{
		CPU:       clampMetricValue(cpu, 0, 100),
		Memory:    clampMetricValue(memory, 0, 100),
		Disk:      clampMetricValue(disk, 0, 100),
		NetIn:     clampMetricValue(netIn, 0, math.Max(1800, netIn+50)),
		NetOut:    clampMetricValue(netOut, 0, math.Max(1700, netOut+40)),
		DiskRead:  clampMetricValue(diskRead, 0, math.Max(1200, diskRead+30)),
		DiskWrite: clampMetricValue(diskWrite, 0, math.Max(1200, diskWrite+30)),
	}
}

func hashKubernetesMetricsSeed(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

func hashDockerMetricsSeed(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

func clampMetricValue(value, min, max float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func percentFromUsage(value float64) float64 {
	if value <= 1.0 {
		return value * 100
	}
	return value
}
