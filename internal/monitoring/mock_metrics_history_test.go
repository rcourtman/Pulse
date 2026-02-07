package monitoring

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestSeedMockMetricsHistory_PopulatesSeries(t *testing.T) {
	now := time.Now()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Status: "online",
				CPU:    0.33,
				Memory: models.Memory{Usage: 62, Total: 128 * 1024 * 1024 * 1024},
				Disk:   models.Disk{Usage: 41, Total: 1024, Used: 512},
			},
		},
		VMs: []models.VM{
			{
				ID:     "vm-100",
				Status: "running",
				CPU:    0.21,
				Memory: models.Memory{Usage: 47, Total: 8 * 1024 * 1024 * 1024},
				Disk:   models.Disk{Usage: 28, Total: 1024, Used: 256},
			},
		},
		Containers: []models.Container{
			{
				ID:     "ct-200",
				Status: "running",
				CPU:    0.09,
				Memory: models.Memory{Usage: 53, Total: 2 * 1024 * 1024 * 1024},
				Disk:   models.Disk{Usage: 17, Total: 512, Used: 128},
			},
		},
		Storage: []models.Storage{
			{
				ID:     "local",
				Status: "available",
				Total:  1000,
				Used:   420,
				Free:   580,
				Usage:  42,
			},
		},
		DockerHosts: []models.DockerHost{
			{
				ID:       "host-1",
				Status:   "online",
				CPUUsage: 22.5,
				Memory:   models.Memory{Usage: 58, Total: 16 * 1024 * 1024 * 1024},
				Disks: []models.Disk{
					{Total: 1000, Used: 600, Usage: 60},
				},
				Containers: []models.DockerContainer{
					{
						ID:                  "cont-1",
						State:               "running",
						CPUPercent:          3.3,
						MemoryPercent:       11.2,
						WritableLayerBytes:  10,
						RootFilesystemBytes: 100,
					},
				},
			},
		},
	}

	mh := NewMetricsHistory(1000, 24*time.Hour)
	seedMockMetricsHistory(mh, nil, state, now, time.Hour, 30*time.Second)

	nodeCPU := mh.GetNodeMetrics("node-1", "cpu", time.Hour)
	if len(nodeCPU) < 10 {
		t.Fatalf("expected seeded node cpu points, got %d", len(nodeCPU))
	}
	if got, want := nodeCPU[len(nodeCPU)-1].Value, state.Nodes[0].CPU*100; math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last node cpu point to match current, got=%v want=%v", got, want)
	}

	vmCPU := mh.GetGuestMetrics("vm-100", "cpu", time.Hour)
	if len(vmCPU) < 10 {
		t.Fatalf("expected seeded vm cpu points, got %d", len(vmCPU))
	}
	if got, want := vmCPU[len(vmCPU)-1].Value, state.VMs[0].CPU*100; math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last vm cpu point to match current, got=%v want=%v", got, want)
	}

	dockerCPU := mh.GetGuestMetrics("docker:cont-1", "cpu", time.Hour)
	if len(dockerCPU) < 10 {
		t.Fatalf("expected seeded docker container cpu points, got %d", len(dockerCPU))
	}
	if got, want := dockerCPU[len(dockerCPU)-1].Value, state.DockerHosts[0].Containers[0].CPUPercent; math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last docker cpu point to match current, got=%v want=%v", got, want)
	}
}

func TestSeedMockMetricsHistory_PopulatesKubernetesPodSeries(t *testing.T) {
	now := time.Now()

	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:     "cluster-1",
				Name:   "cluster-1",
				Status: "online",
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-1",
						Name:      "api-0",
						Namespace: "default",
						Phase:     "Running",
						Containers: []models.KubernetesPodContainer{
							{Name: "api", Ready: true},
						},
					},
				},
			},
		},
	}

	mh := NewMetricsHistory(1000, 24*time.Hour)
	seedMockMetricsHistory(mh, nil, state, now, time.Hour, 30*time.Second)

	metricID := kubernetesPodMetricID(state.KubernetesClusters[0], state.KubernetesClusters[0].Pods[0])
	if metricID == "" {
		t.Fatal("expected kubernetes pod metric id")
	}

	cpuSeries := mh.GetGuestMetrics(metricID, "cpu", time.Hour)
	if len(cpuSeries) < 10 {
		t.Fatalf("expected seeded kubernetes cpu points, got %d", len(cpuSeries))
	}

	current := kubernetesPodCurrentMetrics(state.KubernetesClusters[0], state.KubernetesClusters[0].Pods[0])
	if got, want := cpuSeries[len(cpuSeries)-1].Value, current["cpu"]; math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last kubernetes cpu point to match current, got=%v want=%v", got, want)
	}
}

func TestSeedMockMetricsHistory_SeedsMetricsStore(t *testing.T) {
	now := time.Now()

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Status: "online",
				CPU:    0.33,
				Memory: models.Memory{Usage: 62, Total: 128 * 1024 * 1024 * 1024},
				Disk:   models.Disk{Usage: 41, Total: 1024, Used: 512},
			},
		},
		VMs: []models.VM{
			{
				ID:     "vm-100",
				Status: "running",
				CPU:    0.21,
				Memory: models.Memory{Usage: 47, Total: 8 * 1024 * 1024 * 1024},
				Disk:   models.Disk{Usage: 28, Total: 1024, Used: 256},
			},
		},
		KubernetesClusters: []models.KubernetesCluster{
			{
				ID:     "cluster-1",
				Name:   "cluster-1",
				Status: "online",
				Pods: []models.KubernetesPod{
					{
						UID:       "pod-1",
						Name:      "api-0",
						Namespace: "default",
						Phase:     "Running",
						Containers: []models.KubernetesPodContainer{
							{Name: "api", Ready: true},
						},
					},
				},
			},
		},
	}

	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw = 90 * 24 * time.Hour
	cfg.RetentionMinute = 90 * 24 * time.Hour
	cfg.RetentionHourly = 90 * 24 * time.Hour
	cfg.RetentionDaily = 90 * 24 * time.Hour
	cfg.WriteBufferSize = 500

	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	defer store.Close()

	mh := NewMetricsHistory(1000, 7*24*time.Hour)
	seedMockMetricsHistory(mh, store, state, now, 7*24*time.Hour, time.Minute)

	points, err := store.Query("vm", "vm-100", "cpu", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query metrics store: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected metrics store to have seeded points for 7d range")
	}
	metricID := kubernetesPodMetricID(state.KubernetesClusters[0], state.KubernetesClusters[0].Pods[0])
	if metricID == "" {
		t.Fatal("expected kubernetes pod metric id")
	}
	k8sPoints, err := store.Query("k8s", metricID, "cpu", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query kubernetes pod metrics store: %v", err)
	}
	if len(k8sPoints) == 0 {
		t.Fatal("expected metrics store to have seeded kubernetes pod points for 7d range")
	}
}

func TestSeedMockMetricsHistory_SeedsDiskTemperatureMetricsStore(t *testing.T) {
	now := time.Now()

	state := models.StateSnapshot{
		PhysicalDisks: []models.PhysicalDisk{
			{
				ID:          "disk-1",
				Instance:    "pve1",
				Node:        "node1",
				DevPath:     "/dev/nvme0n1",
				Serial:      "SERIAL-001",
				Temperature: 44,
			},
		},
	}

	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw = 90 * 24 * time.Hour
	cfg.RetentionMinute = 90 * 24 * time.Hour
	cfg.RetentionHourly = 90 * 24 * time.Hour
	cfg.RetentionDaily = 90 * 24 * time.Hour
	cfg.WriteBufferSize = 500

	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	defer store.Close()

	mh := NewMetricsHistory(1000, 7*24*time.Hour)
	seedMockMetricsHistory(mh, store, state, now, 7*24*time.Hour, time.Minute)

	points, err := store.Query("disk", "SERIAL-001", "smart_temp", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query disk smart_temp metrics: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected metrics store to have seeded smart_temp points for disk")
	}
}

func TestStartMockMetricsSampler_DoesNotClearExistingMetricsStoreData(t *testing.T) {
	t.Setenv("PULSE_MOCK_NODES", "1")
	t.Setenv("PULSE_MOCK_VMS_PER_NODE", "0")
	t.Setenv("PULSE_MOCK_LXCS_PER_NODE", "0")
	t.Setenv("PULSE_MOCK_DOCKER_HOSTS", "0")
	t.Setenv("PULSE_MOCK_DOCKER_CONTAINERS", "0")
	t.Setenv("PULSE_MOCK_GENERIC_HOSTS", "0")
	t.Setenv("PULSE_MOCK_K8S_CLUSTERS", "0")
	t.Setenv("PULSE_MOCK_K8S_NODES", "0")
	t.Setenv("PULSE_MOCK_K8S_PODS", "0")
	t.Setenv("PULSE_MOCK_K8S_DEPLOYMENTS", "0")
	t.Setenv("PULSE_MOCK_TRENDS_SEED_DURATION", "1h")
	t.Setenv("PULSE_MOCK_TRENDS_SAMPLE_INTERVAL", "5m")

	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.RetentionRaw = 90 * 24 * time.Hour
	cfg.RetentionMinute = 90 * 24 * time.Hour
	cfg.RetentionHourly = 90 * 24 * time.Hour
	cfg.RetentionDaily = 90 * 24 * time.Hour
	cfg.WriteBufferSize = 100

	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create metrics store: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().Truncate(time.Second)
	store.WriteBatchSync([]metrics.WriteMetric{
		{
			ResourceType: "host",
			ResourceID:   "prod-node-1",
			MetricType:   "cpu",
			Value:        55.0,
			Timestamp:    now.Add(-10 * time.Minute),
			Tier:         metrics.TierRaw,
		},
	})

	mock.SetEnabled(true)
	t.Cleanup(func() { mock.SetEnabled(false) })

	monitor := &Monitor{
		metricsHistory: NewMetricsHistory(1000, 24*time.Hour),
		metricsStore:   store,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.startMockMetricsSampler(ctx)
	t.Cleanup(func() { monitor.stopMockMetricsSampler() })

	points, err := store.Query("host", "prod-node-1", "cpu", now.Add(-2*time.Hour), now.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("failed to query preserved production metrics: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected existing production metric to remain after mock sampler start")
	}

	found := false
	for _, point := range points {
		if math.Abs(point.Value-55.0) < 1e-9 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find original production metric value after mock sampler start")
	}
}

func TestGenerateSeededSeries_Deterministic(t *testing.T) {
	seed := hashSeed("node", "deterministic", "cpu")
	seriesA := generateSeededSeries(57.3, 240, seed, 0, 100)
	seriesB := generateSeededSeries(57.3, 240, seed, 0, 100)

	if len(seriesA) != len(seriesB) {
		t.Fatalf("expected same length, got %d vs %d", len(seriesA), len(seriesB))
	}

	for i := range seriesA {
		if seriesA[i] != seriesB[i] {
			t.Fatalf("series mismatch at index %d: %f != %f", i, seriesA[i], seriesB[i])
		}
	}
}

func TestGenerateSeededSeries_BoundsAndAnchor(t *testing.T) {
	current := 42.0
	series := generateSeededSeries(current, 360, hashSeed("storage", "local-zfs", "usage"), 10, 85)
	if len(series) != 360 {
		t.Fatalf("expected 360 points, got %d", len(series))
	}
	if series[len(series)-1] != current {
		t.Fatalf("expected final point to match current value, got %.4f want %.4f", series[len(series)-1], current)
	}

	for i, v := range series {
		if v < 10 || v > 85 {
			t.Fatalf("point out of bounds at index %d: %.4f (expected 10..85)", i, v)
		}
	}
}

func TestGenerateSeededSeries_VolatileProducesJaggedChanges(t *testing.T) {
	// Seed 3 maps to trendVolatile via pickTrendClass(seed % 4).
	series := generateSeededSeries(63, 360, 3, 0, 100)
	if len(series) != 360 {
		t.Fatalf("expected 360 points, got %d", len(series))
	}

	kinks := 0
	for i := 2; i < len(series); i++ {
		s1 := series[i-1] - series[i-2]
		s2 := series[i] - series[i-1]
		if math.Abs(s2-s1) >= 0.9 {
			kinks++
		}
	}

	if kinks < 35 {
		t.Fatalf("expected jagged volatile series with many kinks; got %d", kinks)
	}
}
