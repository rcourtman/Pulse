package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestMetricsFromDockerHostIncludesIORates(t *testing.T) {
	host := models.DockerHost{
		CPUUsage:      12.5,
		NetInRate:     2048,
		NetOutRate:    4096,
		DiskReadRate:  8192,
		DiskWriteRate: 16384,
		Memory:        models.Memory{Total: 8, Used: 4, Usage: 50},
		Disks:         []models.Disk{{Total: 10, Used: 2, Usage: 20}},
	}

	metrics := metricsFromDockerHost(host)
	if metrics.NetIn == nil || metrics.NetIn.Value != host.NetInRate {
		t.Fatalf("expected netIn=%v, got %+v", host.NetInRate, metrics.NetIn)
	}
	if metrics.NetOut == nil || metrics.NetOut.Value != host.NetOutRate {
		t.Fatalf("expected netOut=%v, got %+v", host.NetOutRate, metrics.NetOut)
	}
	if metrics.DiskRead == nil || metrics.DiskRead.Value != host.DiskReadRate {
		t.Fatalf("expected diskRead=%v, got %+v", host.DiskReadRate, metrics.DiskRead)
	}
	if metrics.DiskWrite == nil || metrics.DiskWrite.Value != host.DiskWriteRate {
		t.Fatalf("expected diskWrite=%v, got %+v", host.DiskWriteRate, metrics.DiskWrite)
	}
}

func TestUnavailableMemoryDoesNotProjectOrOverwriteTrustedCrossSourceMetric(t *testing.T) {
	unavailable := models.UnavailableMemory(8 * 1024 * 1024 * 1024)

	for name, metrics := range map[string]*ResourceMetrics{
		"proxmox-node": metricsFromProxmoxNode(models.Node{Memory: unavailable}),
		"proxmox-vm":   metricsFromVM(models.VM{Memory: unavailable}),
		"proxmox-lxc":  metricsFromContainer(models.Container{Memory: unavailable}),
		"agent-host":   metricsFromHost(models.Host{Memory: unavailable}),
		"docker-host":  metricsFromDockerHost(models.DockerHost{Memory: unavailable}),
	} {
		t.Run(name, func(t *testing.T) {
			if metrics.Memory != nil {
				t.Fatalf("Memory = %+v, want nil for unavailable usage", metrics.Memory)
			}
		})
	}

	trustedTotal := int64(8 * 1024 * 1024 * 1024)
	trustedUsed := int64(4 * 1024 * 1024 * 1024)
	trusted := &ResourceMetrics{
		Memory: &MetricValue{
			Used:    &trustedUsed,
			Total:   &trustedTotal,
			Percent: 50,
			Source:  SourceAgent,
		},
	}
	merged := mergeMetrics(
		&Resource{Type: ResourceTypeVM, Proxmox: &ProxmoxData{VMID: 1501}},
		trusted,
		metricsFromVM(models.VM{Memory: unavailable}),
		SourceProxmox,
		time.Now().UTC(),
		nil,
		nil,
	)
	if merged.Memory == nil || merged.Memory.Percent != 50 || merged.Memory.Source != SourceAgent {
		t.Fatalf("merged memory = %+v, want trusted agent metric preserved", merged.Memory)
	}

	resource, _ := resourceFromVM(models.VM{VMID: 1501, Memory: unavailable})
	if resource.Metrics == nil || resource.Metrics.Memory != nil {
		t.Fatalf("canonical metrics = %+v, want absent unavailable memory metric", resource.Metrics)
	}
	if resource.Proxmox == nil || resource.Proxmox.Memory == nil || !resource.Proxmox.Memory.UsageUnavailable {
		t.Fatalf("Proxmox memory fallback = %+v, want explicit unavailable capacity", resource.Proxmox)
	}

	for name, tc := range map[string]struct {
		source   DataSource
		resource *Resource
	}{
		"proxmox": {
			source:   SourceProxmox,
			resource: &Resource{Proxmox: &ProxmoxData{Memory: &unavailable}},
		},
		"agent": {
			source: SourceAgent,
			resource: &Resource{Agent: &AgentData{Memory: &AgentMemoryMeta{
				Total:            unavailable.Total,
				UsageUnavailable: true,
			}}},
		},
		"docker": {
			source: SourceDocker,
			resource: &Resource{Docker: &DockerData{Memory: &AgentMemoryMeta{
				Total:            unavailable.Total,
				UsageUnavailable: true,
			}}},
		},
	} {
		t.Run("same-source-"+name, func(t *testing.T) {
			sameSource := &ResourceMetrics{Memory: &MetricValue{Percent: 50, Source: tc.source}}
			cleared := clearUnavailableSourceMemoryMetric(sameSource, tc.resource, tc.source)
			if cleared.Memory != nil {
				t.Fatalf("Memory = %+v, want same-source stale value cleared", cleared.Memory)
			}
		})
	}

	dockerResource, _ := resourceFromDockerHost(models.DockerHost{Memory: unavailable})
	if dockerResource.Docker == nil || dockerResource.Docker.Memory == nil || !dockerResource.Docker.Memory.UsageUnavailable {
		t.Fatalf("Docker memory fallback = %+v, want explicit unavailable capacity", dockerResource.Docker)
	}
}

func TestMetricsFromHostKeepsReportedPercentValues(t *testing.T) {
	host := models.Host{
		CPUUsage: 1,
		Memory:   models.Memory{Total: 1000, Used: 5, Usage: 0.5},
		Disks:    []models.Disk{{Total: 1000, Used: 2, Usage: 0.2}},
	}

	metrics := metricsFromHost(host)
	if metrics.CPU == nil || metrics.CPU.Percent != 1 {
		t.Fatalf("expected host cpu percent 1, got %+v", metrics.CPU)
	}
	if metrics.Memory == nil || metrics.Memory.Percent != 0.5 {
		t.Fatalf("expected host memory percent 0.5, got %+v", metrics.Memory)
	}
	if metrics.Disk == nil || metrics.Disk.Percent != 0.2 {
		t.Fatalf("expected host disk percent 0.2, got %+v", metrics.Disk)
	}
}

func TestMetricsFromDockerHostKeepsReportedPercentValues(t *testing.T) {
	host := models.DockerHost{
		CPUUsage: 1,
		Memory:   models.Memory{Total: 1000, Used: 5, Usage: 0.5},
		Disks:    []models.Disk{{Total: 1000, Used: 2, Usage: 0.2}},
	}

	metrics := metricsFromDockerHost(host)
	if metrics.CPU == nil || metrics.CPU.Percent != 1 {
		t.Fatalf("expected docker host cpu percent 1, got %+v", metrics.CPU)
	}
	if metrics.Memory == nil || metrics.Memory.Percent != 0.5 {
		t.Fatalf("expected docker host memory percent 0.5, got %+v", metrics.Memory)
	}
	if metrics.Disk == nil || metrics.Disk.Percent != 0.2 {
		t.Fatalf("expected docker host disk percent 0.2, got %+v", metrics.Disk)
	}
}

func TestMetricsFromDockerContainerKeepsCapacityPercentValues(t *testing.T) {
	container := models.DockerContainer{
		CPUPercent:    2,
		MemoryUsage:   5,
		MemoryLimit:   1000,
		MemoryPercent: 0.5,
	}

	metrics := metricsFromDockerContainer(container, 4)
	if metrics.CPU == nil || metrics.CPU.Percent != 0.5 || metrics.CPU.Value != 0.5 {
		t.Fatalf("expected docker container cpu percent 0.5, got %+v", metrics.CPU)
	}
	if metrics.Memory == nil || metrics.Memory.Percent != 0.5 {
		t.Fatalf("expected docker container memory percent 0.5, got %+v", metrics.Memory)
	}
}

func TestMetricsFromProxmoxGuestsNormalizesCPUOnceIndependentOfCoreCount(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cores int
	}{
		{name: "one-core", cores: 1},
		{name: "four-cores", cores: 4},
		{name: "eight-cores", cores: 8},
	} {
		t.Run("vm-"+tc.name, func(t *testing.T) {
			metrics := metricsFromVM(models.VM{CPU: 0.0058, CPUs: tc.cores})
			if metrics.CPU == nil {
				t.Fatal("VM CPU metric is nil")
			}
			if metrics.CPU.Percent != 0.58 || metrics.CPU.Value != 0.58 || metrics.CPU.Source != SourceProxmox {
				t.Fatalf("VM CPU = %+v, want canonical proxmox percent 0.58", metrics.CPU)
			}
		})

		t.Run("lxc-"+tc.name, func(t *testing.T) {
			metrics := metricsFromContainer(models.Container{CPU: 0.0058, CPUs: tc.cores})
			if metrics.CPU == nil {
				t.Fatal("LXC CPU metric is nil")
			}
			if metrics.CPU.Percent != 0.58 || metrics.CPU.Value != 0.58 || metrics.CPU.Source != SourceProxmox {
				t.Fatalf("LXC CPU = %+v, want canonical proxmox percent 0.58", metrics.CPU)
			}
		})
	}
}

func TestMetricsFromKubernetesClusterAggregatesLinkedHostReportedPercents(t *testing.T) {
	cluster := models.KubernetesCluster{ID: "cluster-1", Name: "cluster-1"}
	hosts := []*models.Host{
		{
			Hostname: "worker-1",
			CPUUsage: 1,
		},
		{
			Hostname: "worker-2",
			CPUUsage: 2,
		},
	}

	metrics := metricsFromKubernetesCluster(cluster, hosts)
	if metrics.CPU == nil || metrics.CPU.Percent != 1.5 {
		t.Fatalf("expected linked host cluster cpu percent 1.5, got %+v", metrics.CPU)
	}
}

func TestMetricsFromKubernetesPod_MockModeIncludesLiveLikeValues(t *testing.T) {
	enableMockMode(t)

	cluster := models.KubernetesCluster{
		ID:   "cluster-1",
		Name: "cluster-1",
	}
	pod := models.KubernetesPod{
		UID:       "pod-1",
		Name:      "api-0",
		Namespace: "default",
		Phase:     "Running",
		Restarts:  2,
		Containers: []models.KubernetesPodContainer{
			{Name: "api", Ready: true},
		},
	}

	metrics := metricsFromKubernetesPod(cluster, pod)
	if metrics.CPU == nil || metrics.CPU.Value <= 0 {
		t.Fatalf("expected non-zero kubernetes cpu metric, got %+v", metrics.CPU)
	}
	if metrics.Memory == nil || metrics.Memory.Value <= 0 {
		t.Fatalf("expected non-zero kubernetes memory metric, got %+v", metrics.Memory)
	}
	if metrics.Disk == nil || metrics.Disk.Value <= 0 {
		t.Fatalf("expected non-zero kubernetes disk metric, got %+v", metrics.Disk)
	}
	if metrics.NetIn == nil || metrics.NetIn.Value <= 0 {
		t.Fatalf("expected non-zero kubernetes netIn metric, got %+v", metrics.NetIn)
	}
	if metrics.NetOut == nil || metrics.NetOut.Value <= 0 {
		t.Fatalf("expected non-zero kubernetes netOut metric, got %+v", metrics.NetOut)
	}
	if metrics.DiskRead != nil || metrics.DiskWrite != nil {
		t.Fatalf("expected kubernetes pod disk I/O metrics to remain unset without production collection path, got read=%+v write=%+v", metrics.DiskRead, metrics.DiskWrite)
	}
}

func TestMetricsFromKubernetesNode_DoesNotSynthesizeUnsupportedUsageMetrics(t *testing.T) {
	enableMockMode(t)

	cluster := models.KubernetesCluster{
		ID:   "cluster-node-1",
		Name: "cluster-node-1",
	}
	node := models.KubernetesNode{
		UID:                 "node-1",
		Name:                "worker-a",
		Ready:               true,
		Unschedulable:       false,
		Roles:               []string{"worker"},
		CapacityCPU:         8,
		AllocCPU:            6,
		CapacityMemoryBytes: 32 * 1024 * 1024 * 1024,
		AllocMemoryBytes:    24 * 1024 * 1024 * 1024,
		CapacityPods:        120,
		AllocPods:           82,
	}

	metrics := metricsFromKubernetesNode(cluster, node, nil)
	if metrics.CPU != nil || metrics.Memory != nil || metrics.Disk != nil ||
		metrics.NetIn != nil || metrics.NetOut != nil || metrics.DiskRead != nil || metrics.DiskWrite != nil {
		t.Fatalf("expected kubernetes node usage metrics to be absent, got %+v", metrics)
	}
}

func TestMetricsFromKubernetesCluster_DoesNotSynthesizeUnsupportedUsageMetrics(t *testing.T) {
	enableMockMode(t)

	cluster := models.KubernetesCluster{
		ID:   "cluster-1",
		Name: "cluster-1",
		Nodes: []models.KubernetesNode{
			{UID: "n1", Name: "n1", Ready: true},
			{UID: "n2", Name: "n2", Ready: true},
		},
		Pods: []models.KubernetesPod{
			{UID: "p1", Name: "api-0", Phase: "Running"},
			{UID: "p2", Name: "api-1", Phase: "Running"},
			{UID: "p3", Name: "worker-0", Phase: "Pending"},
		},
	}

	metrics := metricsFromKubernetesCluster(cluster, nil)
	if metrics.CPU != nil || metrics.Memory != nil || metrics.Disk != nil ||
		metrics.NetIn != nil || metrics.NetOut != nil || metrics.DiskRead != nil || metrics.DiskWrite != nil {
		t.Fatalf("expected kubernetes cluster usage metrics to be absent, got %+v", metrics)
	}
}

func TestMetricsFromKubernetesNode_UsesLinkedHostMetrics(t *testing.T) {
	cluster := models.KubernetesCluster{ID: "cluster-1", Name: "cluster-1"}
	node := models.KubernetesNode{UID: "node-1", Name: "worker-1"}
	host := &models.Host{
		Hostname:      "worker-1",
		CPUUsage:      47.5,
		Memory:        models.Memory{Total: 16 * 1024 * 1024 * 1024, Used: 8 * 1024 * 1024 * 1024, Usage: 50},
		Disks:         []models.Disk{{Total: 500 * 1024 * 1024 * 1024, Used: 200 * 1024 * 1024 * 1024, Usage: 40}},
		NetInRate:     1234,
		NetOutRate:    2345,
		DiskReadRate:  3456,
		DiskWriteRate: 4567,
	}

	metrics := metricsFromKubernetesNode(cluster, node, host)
	if metrics.CPU == nil || metrics.CPU.Value <= 0 {
		t.Fatalf("expected cpu metric from linked host, got %+v", metrics.CPU)
	}
	if metrics.Memory == nil || metrics.Memory.Total == nil || *metrics.Memory.Total == 0 {
		t.Fatalf("expected memory metric from linked host, got %+v", metrics.Memory)
	}
	if metrics.NetIn == nil || metrics.NetIn.Value != host.NetInRate {
		t.Fatalf("expected netIn=%v from linked host, got %+v", host.NetInRate, metrics.NetIn)
	}
	if metrics.DiskWrite == nil || metrics.DiskWrite.Value != host.DiskWriteRate {
		t.Fatalf("expected diskWrite=%v from linked host, got %+v", host.DiskWriteRate, metrics.DiskWrite)
	}
}

func TestMetricsFromKubernetesCluster_AggregatesLinkedHostMetrics(t *testing.T) {
	cluster := models.KubernetesCluster{ID: "cluster-1", Name: "cluster-1"}
	hosts := []*models.Host{
		{
			Hostname:      "worker-1",
			CPUUsage:      40,
			Memory:        models.Memory{Total: 16 * 1024, Used: 8 * 1024, Usage: 50},
			Disks:         []models.Disk{{Total: 1000, Used: 500, Usage: 50}},
			NetInRate:     100,
			NetOutRate:    200,
			DiskReadRate:  300,
			DiskWriteRate: 400,
		},
		{
			Hostname:      "worker-2",
			CPUUsage:      60,
			Memory:        models.Memory{Total: 16 * 1024, Used: 4 * 1024, Usage: 25},
			Disks:         []models.Disk{{Total: 1000, Used: 250, Usage: 25}},
			NetInRate:     300,
			NetOutRate:    500,
			DiskReadRate:  700,
			DiskWriteRate: 900,
		},
	}

	metrics := metricsFromKubernetesCluster(cluster, hosts)
	if metrics.CPU == nil || metrics.CPU.Value <= 0 {
		t.Fatalf("expected aggregated cpu metric, got %+v", metrics.CPU)
	}
	if metrics.Memory == nil || metrics.Memory.Total == nil || *metrics.Memory.Total == 0 {
		t.Fatalf("expected aggregated memory metric, got %+v", metrics.Memory)
	}
	if metrics.NetIn == nil || metrics.NetIn.Value != 400 {
		t.Fatalf("expected aggregated netIn=400, got %+v", metrics.NetIn)
	}
	if metrics.DiskWrite == nil || metrics.DiskWrite.Value != 1300 {
		t.Fatalf("expected aggregated diskWrite=1300, got %+v", metrics.DiskWrite)
	}
}

func TestMetricsFromDockerContainerIncludesContainerIORates(t *testing.T) {
	readRate := 321.0
	writeRate := 654.0
	container := models.DockerContainer{
		ID:                  "ctr-1",
		Name:                "api",
		State:               "running",
		CPUPercent:          33,
		MemoryUsage:         1024,
		MemoryLimit:         2048,
		MemoryPercent:       50,
		NetworkRXBytes:      12_000,
		NetworkTXBytes:      19_000,
		NetInRate:           120,
		NetOutRate:          220,
		WritableLayerBytes:  200,
		RootFilesystemBytes: 1000,
		BlockIO: &models.DockerContainerBlockIO{
			ReadBytes:               5000,
			WriteBytes:              6000,
			ReadRateBytesPerSecond:  &readRate,
			WriteRateBytesPerSecond: &writeRate,
		},
	}

	metrics := metricsFromDockerContainer(container)
	if metrics.NetIn == nil || metrics.NetIn.Value != container.NetInRate {
		t.Fatalf("expected netIn=%v, got %+v", container.NetInRate, metrics.NetIn)
	}
	if metrics.NetOut == nil || metrics.NetOut.Value != container.NetOutRate {
		t.Fatalf("expected netOut=%v, got %+v", container.NetOutRate, metrics.NetOut)
	}
	if metrics.DiskRead == nil || metrics.DiskRead.Value != readRate {
		t.Fatalf("expected diskRead=%v, got %+v", readRate, metrics.DiskRead)
	}
	if metrics.DiskWrite == nil || metrics.DiskWrite.Value != writeRate {
		t.Fatalf("expected diskWrite=%v, got %+v", writeRate, metrics.DiskWrite)
	}
	if metrics.Disk == nil || metrics.Disk.Percent <= 0 {
		t.Fatalf("expected non-zero disk usage metric, got %+v", metrics.Disk)
	}
}

func TestMetricsFromDockerContainerUsesHostCapacityNormalizedCPU(t *testing.T) {
	container := models.DockerContainer{
		ID:         "ctr-cpu-1",
		Name:       "api",
		State:      "running",
		CPUPercent: 240,
	}

	metrics := metricsFromDockerContainer(container, 4)
	if metrics.CPU == nil {
		t.Fatal("expected Docker container CPU metric")
	}
	if metrics.CPU.Value != 60 || metrics.CPU.Percent != 60 {
		t.Fatalf("Docker container CPU metric = value %v percent %v, want 60", metrics.CPU.Value, metrics.CPU.Percent)
	}
}

func TestMetricsFromDockerContainerMockFallbackSynthesizesIO(t *testing.T) {
	enableMockMode(t)

	container := models.DockerContainer{
		ID:            "ctr-mock-1",
		Name:          "jobs",
		State:         "running",
		CPUPercent:    21,
		MemoryUsage:   1024,
		MemoryLimit:   4096,
		MemoryPercent: 25,
	}

	metrics := metricsFromDockerContainer(container)
	if metrics.NetIn == nil || metrics.NetIn.Value <= 0 {
		t.Fatalf("expected synthesized netIn > 0, got %+v", metrics.NetIn)
	}
	if metrics.NetOut == nil || metrics.NetOut.Value <= 0 {
		t.Fatalf("expected synthesized netOut > 0, got %+v", metrics.NetOut)
	}
	if metrics.DiskRead == nil || metrics.DiskRead.Value <= 0 {
		t.Fatalf("expected synthesized diskRead > 0, got %+v", metrics.DiskRead)
	}
	if metrics.DiskWrite == nil || metrics.DiskWrite.Value <= 0 {
		t.Fatalf("expected synthesized diskWrite > 0, got %+v", metrics.DiskWrite)
	}
}
