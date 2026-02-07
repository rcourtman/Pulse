package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
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

func TestMetricsFromKubernetesPod_MockModeIncludesLiveLikeValues(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

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
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

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
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

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

func TestMetricsFromDockerContainerMockFallbackSynthesizesIO(t *testing.T) {
	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

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
