package monitoring

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func fixtureGraphWithState(state models.StateSnapshot) mock.FixtureGraph {
	return mock.FixtureGraph{State: state}
}

func TestBuildTieredTimestamps_IncludesCanonicalTerminalNow(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)

	timestamps := buildTieredTimestamps(now, time.Hour)
	if len(timestamps) == 0 {
		t.Fatal("expected tiered timestamps")
	}

	last := timestamps[len(timestamps)-1]
	if !last.Equal(now) {
		t.Fatalf("expected seed timestamps to include terminal now, got %v with now=%v", last, now)
	}
}

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
		PhysicalDisks: []models.PhysicalDisk{
			{
				ID:          "disk-1",
				Node:        "node-1",
				Serial:      "SERIAL-LOCAL-1",
				Temperature: 41,
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
	seedMockMetricsHistory(mh, nil, fixtureGraphWithState(state), now, time.Hour, 30*time.Second)

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

	storageMetrics := mh.GetAllStorageMetrics("local", time.Hour)
	if len(storageMetrics["usage"]) < 10 || len(storageMetrics["used"]) < 10 || len(storageMetrics["avail"]) < 10 {
		t.Fatalf("expected seeded storage capacity history, got usage=%d used=%d avail=%d", len(storageMetrics["usage"]), len(storageMetrics["used"]), len(storageMetrics["avail"]))
	}
	last := len(storageMetrics["usage"]) - 1
	if got, want := storageMetrics["used"][last].Value, float64(state.Storage[0].Used); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last storage used point to match current, got=%v want=%v", got, want)
	}
	if got, want := storageMetrics["avail"][last].Value, float64(state.Storage[0].Free); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last storage avail point to match current, got=%v want=%v", got, want)
	}
	for i := range storageMetrics["usage"] {
		if diff := math.Abs(storageMetrics["used"][i].Value + storageMetrics["avail"][i].Value - float64(state.Storage[0].Total)); diff > 0.001 {
			t.Fatalf("expected storage used+avail to equal total at index %d, diff=%f", i, diff)
		}
	}

	diskTemps := mh.GetDiskMetrics("SERIAL-LOCAL-1", "smart_temp", time.Hour)
	if len(diskTemps) < 10 {
		t.Fatalf("expected seeded disk temperature history, got %d points", len(diskTemps))
	}
	if got, want := diskTemps[len(diskTemps)-1].Value, float64(state.PhysicalDisks[0].Temperature); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected last disk temp point to match current, got=%v want=%v", got, want)
	}
}

func TestSeedMockMetricsHistory_AppendsSingleTerminalNowPoint(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	state := models.StateSnapshot{
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
	seedMockMetricsHistory(mh, nil, fixtureGraphWithState(state), now, time.Hour, 30*time.Second)

	memorySeries := mh.GetGuestMetrics("docker:cont-1", "memory", time.Hour)
	if len(memorySeries) < 2 {
		t.Fatalf("expected seeded docker memory points, got %d", len(memorySeries))
	}

	last := memorySeries[len(memorySeries)-1]
	if !last.Timestamp.Equal(now) {
		t.Fatalf("expected terminal docker memory timestamp %v, got %v", now, last.Timestamp)
	}

	nowCount := 0
	for _, point := range memorySeries {
		if point.Timestamp.Equal(now) {
			nowCount++
		}
	}
	if nowCount != 1 {
		t.Fatalf("expected exactly one terminal now point, got %d", nowCount)
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
	seedMockMetricsHistory(mh, nil, fixtureGraphWithState(state), now, time.Hour, 30*time.Second)

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
	seedMockMetricsHistory(mh, store, fixtureGraphWithState(state), now, 7*24*time.Hour, time.Minute)

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
	seedMockMetricsHistory(mh, store, fixtureGraphWithState(state), now, 7*24*time.Hour, time.Minute)

	points, err := store.Query("disk", "SERIAL-001", "smart_temp", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query disk smart_temp metrics: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected metrics store to have seeded smart_temp points for disk")
	}
}

func TestSeedMockMetricsHistory_SeedsVMwareMetricsStore(t *testing.T) {
	now := time.Now()
	state := models.StateSnapshot{}

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
	seedMockMetricsHistory(mh, store, mock.FixtureGraph{
		State: state,
		PlatformFixtures: mock.PlatformFixtures{
			VMware: vmware.DefaultFixtures(),
		},
	}, now, 7*24*time.Hour, time.Minute)

	hostPoints, err := store.Query("agent", "vc-mock-1:host:host-101", "cpu", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query VMware host cpu metrics: %v", err)
	}
	if len(hostPoints) == 0 {
		t.Fatal("expected metrics store to have seeded VMware host cpu points")
	}

	vmPoints, err := store.Query("vm", "vc-mock-1:vm:vm-201", "cpu", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query VMware VM cpu metrics: %v", err)
	}
	if len(vmPoints) == 0 {
		t.Fatal("expected metrics store to have seeded VMware VM cpu points")
	}

	storagePoints, err := store.Query("storage", "vc-mock-1:datastore:datastore-201", "usage", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query VMware datastore usage metrics: %v", err)
	}
	if len(storagePoints) == 0 {
		t.Fatal("expected metrics store to have seeded VMware datastore usage points")
	}

	storageUsedPoints, err := store.Query("storage", "vc-mock-1:datastore:datastore-201", "used", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query VMware datastore used metrics: %v", err)
	}
	if len(storageUsedPoints) == 0 {
		t.Fatal("expected metrics store to have seeded VMware datastore used points")
	}
}

func TestSeedMockMetricsHistory_SeedsTrueNASMetricsStore(t *testing.T) {
	now := time.Now()
	state := models.StateSnapshot{}
	fixtures := truenas.DefaultFixtures()

	if strings.TrimSpace(fixtures.System.Hostname) == "" {
		t.Fatal("expected canonical truenas system hostname fixture")
	}
	if len(fixtures.Datasets) == 0 {
		t.Fatal("expected canonical truenas dataset fixtures")
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
	seedMockMetricsHistory(mh, store, mock.FixtureGraph{
		State: state,
		PlatformFixtures: mock.PlatformFixtures{
			TrueNAS: fixtures,
		},
	}, now, 7*24*time.Hour, time.Minute)

	systemPoints, err := store.Query("agent", fixtures.System.Hostname, "disk", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query TrueNAS system disk metrics via canonical agent target: %v", err)
	}
	if len(systemPoints) == 0 {
		t.Fatal("expected metrics store to have seeded canonical TrueNAS system disk points")
	}

	agentPoints, err := store.Query("agent", fixtures.System.Hostname, "cpu", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query TrueNAS agent cpu metrics: %v", err)
	}
	if len(agentPoints) == 0 {
		t.Fatal("expected metrics store to have seeded canonical TrueNAS agent cpu points")
	}

	datasetPoints, err := store.Query("storage", "dataset:"+fixtures.Datasets[0].Name, "usage", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query canonical TrueNAS dataset usage metrics: %v", err)
	}
	if len(datasetPoints) == 0 {
		t.Fatal("expected metrics store to have seeded canonical TrueNAS dataset usage points")
	}

	poolUsedPoints, err := store.Query("storage", "pool:"+fixtures.Pools[0].Name, "used", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query TrueNAS pool used metrics: %v", err)
	}
	if len(poolUsedPoints) == 0 {
		t.Fatal("expected metrics store to have seeded TrueNAS pool used points")
	}

	diskTempPoints := mh.GetDiskMetrics(fixtures.Disks[0].Serial, "smart_temp", 7*24*time.Hour)
	if len(diskTempPoints) == 0 {
		t.Fatal("expected in-memory history to have seeded TrueNAS disk temperature points")
	}

	appPoints, err := store.Query("dockerContainer", "nextcloud", "cpu", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query TrueNAS app cpu metrics: %v", err)
	}
	if len(appPoints) == 0 {
		t.Fatal("expected metrics store to have seeded TrueNAS app cpu points")
	}
}

func TestSeedMockMetricsHistory_UsesCanonicalMockFixtureGraphForLegacyAndProviderFixtures(t *testing.T) {
	previous := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	t.Cleanup(func() {
		mock.SetEnabled(false)
		mock.SetMockConfig(previousConfig)
		if previous {
			mock.SetEnabled(true)
			mock.SetMockConfig(previousConfig)
		}
	})

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

	mock.SetEnabled(false)
	mock.SetEnabled(true)

	graph := mock.CurrentFixtureGraph()
	if len(graph.State.Nodes) == 0 {
		t.Fatal("expected canonical mock graph to include legacy nodes")
	}
	if strings.TrimSpace(graph.PlatformFixtures.TrueNAS.System.Hostname) == "" {
		t.Fatal("expected canonical mock graph to include TrueNAS fixtures")
	}
	if len(graph.PlatformFixtures.VMware.Hosts) == 0 {
		t.Fatal("expected canonical mock graph to include VMware fixtures")
	}

	now := time.Now()
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
	seedMockMetricsHistory(mh, store, graph, now, 7*24*time.Hour, time.Minute)

	nodePoints, err := store.Query("node", graph.State.Nodes[0].ID, "cpu", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query legacy mock node cpu metrics: %v", err)
	}
	if len(nodePoints) == 0 {
		t.Fatal("expected seeded legacy mock node cpu metrics from canonical graph state")
	}

	truenasPoints, err := store.Query("agent", graph.PlatformFixtures.TrueNAS.System.Hostname, "disk", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query canonical TrueNAS agent disk metrics: %v", err)
	}
	if len(truenasPoints) == 0 {
		t.Fatal("expected seeded canonical TrueNAS agent metrics from canonical graph fixtures")
	}

	vmwareHostID := "vc-mock-1:host:host-101"
	vmwarePoints, err := store.Query("agent", vmwareHostID, "cpu", now.Add(-7*24*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("failed to query VMware mock host cpu metrics: %v", err)
	}
	if len(vmwarePoints) == 0 {
		t.Fatal("expected seeded VMware metrics from canonical graph fixtures")
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
			ResourceType: "agent",
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

	points, err := store.Query("agent", "prod-node-1", "cpu", now.Add(-2*time.Hour), now.Add(time.Hour), 0)
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

func TestStartMockMetricsSampler_SeedsCanonicalMockResourceHistory(t *testing.T) {
	previousEnabled := mock.IsMockEnabled()
	previousConfig := mock.GetConfig()
	t.Cleanup(func() {
		mock.SetEnabled(false)
		mock.SetMockConfig(previousConfig)
		if previousEnabled {
			mock.SetEnabled(true)
			mock.SetMockConfig(previousConfig)
		}
	})

	cfg := mock.DefaultConfig
	cfg.NodeCount = 3
	cfg.DockerHostCount = 2
	cfg.DockerContainersPerHost = 5
	cfg.RandomMetrics = true

	mock.SetEnabled(false)
	mock.SetMockConfig(cfg)
	mock.SetEnabled(true)

	resources, _ := mock.UnifiedResourceSnapshot()
	if len(resources) == 0 {
		t.Fatal("expected canonical mock unified resources")
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources(resources)

	monitor := &Monitor{
		metricsHistory: NewMetricsHistory(1000, 24*time.Hour),
		state:          models.NewState(),
		resourceStore:  unifiedresources.NewMonitorAdapter(registry),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.startMockMetricsSampler(ctx)
	t.Cleanup(func() { monitor.stopMockMetricsSampler() })

	graph := mock.CurrentFixtureGraph()
	if len(graph.State.PhysicalDisks) == 0 {
		t.Fatal("expected proxmox physical disks in canonical mock graph")
	}
	if len(graph.State.DockerHosts) == 0 || len(graph.State.DockerHosts[0].Containers) == 0 {
		t.Fatal("expected docker app containers in canonical mock graph")
	}

	proxmoxDiskID := strings.TrimSpace(graph.State.PhysicalDisks[0].Serial)
	if proxmoxDiskID == "" {
		proxmoxDiskID = strings.TrimSpace(graph.State.PhysicalDisks[0].ID)
	}
	if proxmoxDiskID == "" {
		t.Fatal("expected proxmox physical disk metric id")
	}

	diskPoints := monitor.metricsHistory.GetDiskMetrics(proxmoxDiskID, "smart_temp", 7*24*time.Hour)
	if got := len(diskPoints); got < 300 {
		t.Fatalf("expected seeded in-memory proxmox disk history, got %d points for %q", got, proxmoxDiskID)
	}

	diskCharts := monitor.GetPhysicalDiskTemperatureCharts(7 * 24 * time.Hour)
	diskChart, ok := diskCharts[proxmoxDiskID]
	if !ok {
		t.Fatalf("expected disk chart for %q, got keys=%v", proxmoxDiskID, keysDiskCharts(diskCharts))
	}
	if got := len(diskChart.Temperature); got < 300 {
		t.Fatalf("expected seeded proxmox disk chart history, got %d points for %q", got, proxmoxDiskID)
	}

	dockerMetricID := strings.TrimSpace(graph.State.DockerHosts[0].Containers[0].ID)
	if dockerMetricID == "" {
		t.Fatal("expected docker app container metric id")
	}

	workloadMetrics := monitor.GetGuestMetricsForChartBatch(
		"dockerContainer",
		[]GuestChartRequest{{
			InMemoryKey:   "docker:" + dockerMetricID,
			SQLResourceID: dockerMetricID,
		}},
		7*24*time.Hour,
	)
	cpuPoints := workloadMetrics[dockerMetricID]["cpu"]
	memoryPoints := workloadMetrics[dockerMetricID]["memory"]
	if got := len(cpuPoints); got < 300 {
		t.Fatalf("expected seeded docker app cpu history, got %d points for %q", got, dockerMetricID)
	}
	if got := len(memoryPoints); got < 300 {
		t.Fatalf("expected seeded docker app memory history, got %d points for %q", got, dockerMetricID)
	}
}

func keysDiskCharts(charts map[string]DiskChartEntry) []string {
	keys := make([]string, 0, len(charts))
	for key := range charts {
		keys = append(keys, key)
	}
	return keys
}

func TestGenerateSeededSeries_Deterministic(t *testing.T) {
	seed := HashSeed("node", "deterministic", "cpu")
	seriesA := GenerateSeededSeries(57.3, 240, seed, 0, 100, styleSpiky)
	seriesB := GenerateSeededSeries(57.3, 240, seed, 0, 100, styleSpiky)

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
	series := GenerateSeededSeries(current, 360, HashSeed("storage", "local-zfs", "usage"), 10, 85, styleFlat)
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

func TestGenerateSeededSeries_SpikyProducesSpikes(t *testing.T) {
	series := GenerateSeededSeries(15, 360, 3, 0, 100, styleSpiky)
	if len(series) != 360 {
		t.Fatalf("expected 360 points, got %d", len(series))
	}

	// Most points should be near baseline (low), with some spikes above.
	lowCount := 0
	spikeCount := 0
	for _, v := range series[:len(series)-1] { // exclude last (forced to current)
		if v < 30 {
			lowCount++
		}
		if v > 40 {
			spikeCount++
		}
	}

	if lowCount < len(series)/2 {
		t.Fatalf("expected majority of points near baseline; only %d/%d below 30", lowCount, len(series))
	}
	if spikeCount < 3 {
		t.Fatalf("expected some spike events above 40; only got %d", spikeCount)
	}
}

func TestGenerateSeededSeriesForTimestamps_StableAcrossOverlappingWindows(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	seed := HashSeed("dockerContainer", "orion-2-f54579833f9c", "memory")

	fullWindow := make([]time.Time, 0, 25)
	for ts := now.Add(-24 * time.Hour); !ts.After(now); ts = ts.Add(time.Hour) {
		fullWindow = append(fullWindow, ts)
	}
	recentWindow := append([]time.Time(nil), fullWindow[len(fullWindow)-7:]...)

	fullSeries := GenerateSeededSeriesForTimestamps(51.9, fullWindow, seed, 0, 100, stylePlateau)
	recentSeries := GenerateSeededSeriesForTimestamps(51.9, recentWindow, seed, 0, 100, stylePlateau)

	if len(recentSeries) != len(recentWindow) {
		t.Fatalf("expected %d recent points, got %d", len(recentWindow), len(recentSeries))
	}

	offset := len(fullSeries) - len(recentSeries)
	for i := range recentSeries {
		if fullSeries[offset+i] != recentSeries[i] {
			t.Fatalf(
				"overlapping timestamp mismatch at index %d: full=%f recent=%f",
				i,
				fullSeries[offset+i],
				recentSeries[i],
			)
		}
	}
}

func TestGenerateSeededMetricSeriesForTimestamps_StableAcrossOverlappingWindows(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	seed := HashSeed("dockerContainer", "nebula-1", "netin")

	fullWindow := make([]time.Time, 0, 25)
	for ts := now.Add(-24 * time.Hour); !ts.After(now); ts = ts.Add(time.Hour) {
		fullWindow = append(fullWindow, ts)
	}
	recentWindow := append([]time.Time(nil), fullWindow[len(fullWindow)-7:]...)

	fullSeries := GenerateSeededMetricSeriesForTimestamps(320, fullWindow, seed, 0, 1200, "netin", styleSpiky)
	recentSeries := GenerateSeededMetricSeriesForTimestamps(320, recentWindow, seed, 0, 1200, "netin", styleSpiky)

	if len(recentSeries) != len(recentWindow) {
		t.Fatalf("expected %d recent points, got %d", len(recentWindow), len(recentSeries))
	}

	offset := len(fullSeries) - len(recentSeries)
	for i := range recentSeries {
		if fullSeries[offset+i] != recentSeries[i] {
			t.Fatalf(
				"overlapping metric timestamp mismatch at index %d: full=%f recent=%f",
				i,
				fullSeries[offset+i],
				recentSeries[i],
			)
		}
	}
}

func TestGenerateSeededMetricSeriesForTimestamps_UsesSameTimelineAsMockRuntime(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 0, 25)
	for ts := now.Add(-24 * time.Hour); !ts.After(now); ts = ts.Add(time.Hour) {
		timestamps = append(timestamps, ts)
	}

	cases := []struct {
		resourceType string
		resourceID   string
		metricType   string
		style        SeriesStyle
	}{
		{resourceType: "dockerContainer", resourceID: "orion-2-f54579833f9c", metricType: "memory", style: stylePlateau},
		{resourceType: "disk", resourceID: "SERIAL-LOCAL-1", metricType: "smart_temp", style: styleFlat},
	}

	for _, tc := range cases {
		current := mock.SampleMetric(tc.resourceType, tc.resourceID, tc.metricType, now)
		series := GenerateSeededResourceMetricSeriesForTimestamps(
			current,
			timestamps,
			tc.resourceType,
			tc.resourceID,
			tc.metricType,
			tc.style,
		)

		for i, ts := range timestamps {
			want := mock.SampleMetric(tc.resourceType, tc.resourceID, tc.metricType, ts)
			if diff := math.Abs(series[i] - want); diff > 1e-9 {
				t.Fatalf(
					"expected seeded %s/%s series to match runtime timeline at index %d: got=%f want=%f",
					tc.resourceType,
					tc.metricType,
					i,
					series[i],
					want,
				)
			}
		}
	}
}

func TestSeedMockMetricsHistory_StaysContinuousWithSubsequentLiveMockTicks(t *testing.T) {
	now := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	next := now.Add(time.Minute)
	const storageTotal = int64(2 * 1024 * 1024 * 1024 * 1024)

	storageUsageAt := func(at time.Time) (float64, int64, int64) {
		usage := mock.SampleMetric("storage", "storage-tail", "usage", at)
		used := int64(math.Round((float64(storageTotal) * usage) / 100.0))
		if used < 0 {
			used = 0
		}
		if used > storageTotal {
			used = storageTotal
		}
		return usage, used, storageTotal - used
	}

	vmMemoryNow := mock.SampleMetric("vm", "vm-tail", "memory", now)
	vmDiskNow := mock.SampleMetric("vm", "vm-tail", "disk", now)
	storageUsageNow, storageUsedNow, storageFreeNow := storageUsageAt(now)

	seedState := models.StateSnapshot{
		VMs: []models.VM{
			{
				ID:     "vm-tail",
				Status: "running",
				CPU:    mock.SampleMetric("vm", "vm-tail", "cpu", now) / 100.0,
				Memory: models.Memory{
					Usage: vmMemoryNow,
					Total: 16 * 1024 * 1024 * 1024,
					Used:  int64(math.Round((16 * 1024 * 1024 * 1024) * (vmMemoryNow / 100.0))),
				},
				Disk: models.Disk{
					Usage: vmDiskNow,
					Total: 512 * 1024 * 1024 * 1024,
					Used:  int64(math.Round((512 * 1024 * 1024 * 1024) * (vmDiskNow / 100.0))),
				},
				NetworkIn:  mock.SampleMetricInt("vm", "vm-tail", "netin", now),
				NetworkOut: mock.SampleMetricInt("vm", "vm-tail", "netout", now),
				DiskRead:   mock.SampleMetricInt("vm", "vm-tail", "diskread", now),
				DiskWrite:  mock.SampleMetricInt("vm", "vm-tail", "diskwrite", now),
			},
		},
		Storage: []models.Storage{
			{
				ID:     "storage-tail",
				Status: "available",
				Total:  storageTotal,
				Used:   storageUsedNow,
				Free:   storageFreeNow,
				Usage:  storageUsageNow,
			},
		},
	}

	mh := NewMetricsHistory(5000, 7*24*time.Hour)
	seedMockMetricsHistory(mh, nil, fixtureGraphWithState(seedState), now, 7*24*time.Hour, time.Minute)

	vmCPUSeeded := mh.GetGuestMetrics("vm-tail", "cpu", 7*24*time.Hour)
	if len(vmCPUSeeded) == 0 {
		t.Fatal("expected seeded vm cpu history")
	}
	for i, point := range vmCPUSeeded {
		want := mock.SampleMetric("vm", "vm-tail", "cpu", point.Timestamp)
		if diff := math.Abs(point.Value - want); diff > 1e-9 {
			t.Fatalf("expected seeded vm cpu point %d to follow canonical runtime timeline: got=%f want=%f ts=%v", i, point.Value, want, point.Timestamp)
		}
	}

	storageSeeded := mh.GetAllStorageMetrics("storage-tail", 7*24*time.Hour)["usage"]
	if len(storageSeeded) == 0 {
		t.Fatal("expected seeded storage usage history")
	}
	for i, point := range storageSeeded {
		want := mock.SampleMetric("storage", "storage-tail", "usage", point.Timestamp)
		if diff := math.Abs(point.Value - want); diff > 1e-9 {
			t.Fatalf("expected seeded storage usage point %d to follow canonical runtime timeline: got=%f want=%f ts=%v", i, point.Value, want, point.Timestamp)
		}
	}

	vmMemoryNext := mock.SampleMetric("vm", "vm-tail", "memory", next)
	vmDiskNext := mock.SampleMetric("vm", "vm-tail", "disk", next)
	storageUsageNext, storageUsedNext, storageFreeNext := storageUsageAt(next)
	liveState := models.StateSnapshot{
		VMs: []models.VM{
			{
				ID:     "vm-tail",
				Status: "running",
				CPU:    mock.SampleMetric("vm", "vm-tail", "cpu", next) / 100.0,
				Memory: models.Memory{
					Usage: vmMemoryNext,
					Total: 16 * 1024 * 1024 * 1024,
					Used:  int64(math.Round((16 * 1024 * 1024 * 1024) * (vmMemoryNext / 100.0))),
				},
				Disk: models.Disk{
					Usage: vmDiskNext,
					Total: 512 * 1024 * 1024 * 1024,
					Used:  int64(math.Round((512 * 1024 * 1024 * 1024) * (vmDiskNext / 100.0))),
				},
				NetworkIn:  mock.SampleMetricInt("vm", "vm-tail", "netin", next),
				NetworkOut: mock.SampleMetricInt("vm", "vm-tail", "netout", next),
				DiskRead:   mock.SampleMetricInt("vm", "vm-tail", "diskread", next),
				DiskWrite:  mock.SampleMetricInt("vm", "vm-tail", "diskwrite", next),
			},
		},
		Storage: []models.Storage{
			{
				ID:     "storage-tail",
				Status: "available",
				Total:  storageTotal,
				Used:   storageUsedNext,
				Free:   storageFreeNext,
				Usage:  storageUsageNext,
			},
		},
	}

	recordMockStateToMetricsHistory(mh, nil, fixtureGraphWithState(liveState), next)

	vmCPUAfterTick := mh.GetGuestMetrics("vm-tail", "cpu", 7*24*time.Hour)
	if got := vmCPUAfterTick[len(vmCPUAfterTick)-1].Timestamp; !got.Equal(next) {
		t.Fatalf("expected latest vm cpu point at %v, got %v", next, got)
	}
	if got, want := vmCPUAfterTick[len(vmCPUAfterTick)-1].Value, mock.SampleMetric("vm", "vm-tail", "cpu", next); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected live vm cpu tick to continue canonical runtime timeline: got=%f want=%f", got, want)
	}
	if got := vmCPUAfterTick[len(vmCPUAfterTick)-2].Timestamp; !got.Equal(now) {
		t.Fatalf("expected penultimate vm cpu point to remain anchored at seed now %v, got %v", now, got)
	}

	storageAfterTick := mh.GetAllStorageMetrics("storage-tail", 7*24*time.Hour)["usage"]
	if got := storageAfterTick[len(storageAfterTick)-1].Timestamp; !got.Equal(next) {
		t.Fatalf("expected latest storage usage point at %v, got %v", next, got)
	}
	if got, want := storageAfterTick[len(storageAfterTick)-1].Value, mock.SampleMetric("storage", "storage-tail", "usage", next); math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected live storage usage tick to continue canonical runtime timeline: got=%f want=%f", got, want)
	}
	if got := storageAfterTick[len(storageAfterTick)-2].Timestamp; !got.Equal(now) {
		t.Fatalf("expected penultimate storage point to remain anchored at seed now %v, got %v", now, got)
	}
}
