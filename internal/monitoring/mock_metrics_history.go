package monitoring

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
	"github.com/rs/zerolog/log"
)

const (
	defaultMockSeedDuration   = 90 * 24 * time.Hour
	defaultMockSampleInterval = 1 * time.Minute // 1m for detailed recent charts
)

type mockMetricsSamplerConfig struct {
	SeedDuration   time.Duration
	SampleInterval time.Duration
}

func mockMetricsSamplerConfigFromEnv() mockMetricsSamplerConfig {
	seedDuration := parseDurationEnv("PULSE_MOCK_TRENDS_SEED_DURATION", defaultMockSeedDuration)
	sampleInterval := parseDurationEnv("PULSE_MOCK_TRENDS_SAMPLE_INTERVAL", defaultMockSampleInterval)

	// Guardrails to keep memory and CPU bounded in demo mode.
	if seedDuration < 5*time.Minute {
		seedDuration = 5 * time.Minute
	}
	if seedDuration > 90*24*time.Hour {
		seedDuration = 90 * 24 * time.Hour
	}
	if sampleInterval < 5*time.Second {
		sampleInterval = 5 * time.Second
	}
	if sampleInterval > 5*time.Minute {
		sampleInterval = 5 * time.Minute
	}

	// Ensure we can generate at least 2 points.
	if seedDuration < sampleInterval {
		seedDuration = sampleInterval
	}

	return mockMetricsSamplerConfig{
		SeedDuration:   seedDuration,
		SampleInterval: sampleInterval,
	}
}

func hashSeed(parts ...string) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

func kubernetesClusterMetricID(cluster models.KubernetesCluster) string {
	if value := strings.TrimSpace(cluster.ID); value != "" {
		return value
	}
	if value := strings.TrimSpace(cluster.Name); value != "" {
		return value
	}
	if value := strings.TrimSpace(cluster.DisplayName); value != "" {
		return value
	}
	return "k8s-cluster"
}

func kubernetesPodMetricID(cluster models.KubernetesCluster, pod models.KubernetesPod) string {
	clusterKey := kubernetesClusterMetricID(cluster)
	podKey := strings.TrimSpace(pod.UID)
	if podKey == "" {
		namespace := strings.TrimSpace(pod.Namespace)
		name := strings.TrimSpace(pod.Name)
		switch {
		case namespace != "" && name != "":
			podKey = namespace + "/" + name
		case name != "":
			podKey = name
		default:
			podKey = "pod"
		}
	}
	if clusterKey == "" || podKey == "" {
		return ""
	}
	return fmt.Sprintf("k8s:%s:pod:%s", clusterKey, podKey)
}

func kubernetesPodCurrentMetrics(cluster models.KubernetesCluster, pod models.KubernetesPod) map[string]float64 {
	clusterKey := kubernetesClusterMetricID(cluster)
	podKey := strings.TrimSpace(pod.UID)
	if podKey == "" {
		podKey = strings.TrimSpace(pod.Namespace) + "/" + strings.TrimSpace(pod.Name)
	}
	seed := hashSeed("k8s-pod-current", clusterKey, podKey)
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

	cpu := clampFloat(pod.UsageCPUPercent, 0, 100)
	memory := clampFloat(pod.UsageMemoryPercent, 0, 100)
	disk := clampFloat(pod.DiskUsagePercent, 0, 100)
	netIn := clampFloat(pod.NetInRate, 0, math.Max(pod.NetInRate, 0))
	netOut := clampFloat(pod.NetOutRate, 0, math.Max(pod.NetOutRate, 0))

	// When the live report has no usage sample yet, synthesize only the metrics
	// that exist in real production Kubernetes collection paths.
	if cpu <= 0 || memory <= 0 || disk <= 0 || netIn <= 0 || netOut <= 0 {
		switch phase {
		case "running":
			if cpu <= 0 {
				cpu = 7 + readiness*52 + rng.Float64()*14 - restartFactor*0.35
			}
			if memory <= 0 {
				memory = 26 + readiness*46 + rng.Float64()*14 + restartFactor*0.25
			}
			if disk <= 0 {
				disk = 17 + readiness*42 + rng.Float64()*16
			}
			if netIn <= 0 {
				netIn = 14 + readiness*220 + rng.Float64()*70 + restarts*2
			}
			if netOut <= 0 {
				netOut = 10 + readiness*180 + rng.Float64()*55 + restarts*1.6
			}
		case "pending":
			if cpu <= 0 {
				cpu = 2 + rng.Float64()*7
			}
			if memory <= 0 {
				memory = 14 + rng.Float64()*16
			}
			if disk <= 0 {
				disk = 8 + rng.Float64()*14
			}
			if netIn <= 0 {
				netIn = 1 + rng.Float64()*10
			}
			if netOut <= 0 {
				netOut = 1 + rng.Float64()*8
			}
		case "failed", "unknown":
			if cpu <= 0 {
				cpu = 1 + rng.Float64()*8
			}
			if memory <= 0 {
				memory = 9 + rng.Float64()*15 + restartFactor*0.5
			}
			if disk <= 0 {
				disk = 7 + rng.Float64()*16
			}
			if netIn <= 0 {
				netIn = 1 + rng.Float64()*14 + restarts*1.4
			}
			if netOut <= 0 {
				netOut = 1 + rng.Float64()*11 + restarts*1.2
			}
		default:
			if cpu <= 0 {
				cpu = 1 + rng.Float64()*6
			}
			if memory <= 0 {
				memory = 8 + rng.Float64()*12
			}
			if disk <= 0 {
				disk = 6 + rng.Float64()*12
			}
			if netIn <= 0 {
				netIn = 1 + rng.Float64()*7
			}
			if netOut <= 0 {
				netOut = 1 + rng.Float64()*6
			}
		}
		if totalContainers > 1 {
			multi := 1 + math.Min(float64(totalContainers-1)*0.08, 0.6)
			if cpu > 0 {
				cpu *= multi
			}
			if memory > 0 {
				memory *= 1 + math.Min(float64(totalContainers-1)*0.1, 0.8)
			}
			if disk > 0 {
				disk *= 1 + math.Min(float64(totalContainers-1)*0.07, 0.5)
			}
			if netIn > 0 {
				netIn *= 1 + math.Min(float64(totalContainers-1)*0.09, 0.7)
			}
			if netOut > 0 {
				netOut *= 1 + math.Min(float64(totalContainers-1)*0.08, 0.65)
			}
		}
	}

	return map[string]float64{
		"cpu":       clampFloat(cpu, 0, 100),
		"memory":    clampFloat(memory, 0, 100),
		"disk":      clampFloat(disk, 0, 100),
		"netin":     clampFloat(netIn, 0, math.Max(1800, netIn+50)),
		"netout":    clampFloat(netOut, 0, math.Max(1700, netOut+40)),
		"diskread":  0,
		"diskwrite": 0,
	}
}

type seededTrendClass int

const (
	trendStable seededTrendClass = iota
	trendGrowing
	trendDeclining
	trendVolatile
)

func pickTrendClass(seed uint64) seededTrendClass {
	switch seed % 4 {
	case 0:
		return trendStable
	case 1:
		return trendGrowing
	case 2:
		return trendDeclining
	default:
		return trendVolatile
	}
}

func generateSeededSeries(current float64, points int, seed uint64, min, max float64) []float64 {
	current = clampFloat(current, min, max)
	if points <= 1 {
		return []float64{current}
	}

	class := pickTrendClass(seed)
	rng := rand.New(rand.NewSource(int64(seed))) // Deterministic per resource/metric
	span := math.Max(1, max-min)
	profile := getSeededTrendProfile(class, seed, span)
	lastIdx := float64(points - 1)

	// Build a piecewise-linear baseline with random jumps at segment boundaries.
	anchorCount := profile.segmentMin + int(seed%uint64(profile.segmentMax-profile.segmentMin+1))
	if anchorCount > points {
		anchorCount = points
	}
	if anchorCount < 3 {
		anchorCount = 3
	}

	anchorValues := make([]float64, anchorCount)
	anchorIndexes := make([]int, anchorCount)
	for i := 0; i < anchorCount; i++ {
		anchorIndexes[i] = (i * (points - 1)) / (anchorCount - 1)
	}

	startTarget := current - profile.totalSlope
	anchorValues[0] = clampFloat(startTarget+rng.NormFloat64()*profile.anchorNoise, min, max)

	for i := 1; i < anchorCount; i++ {
		progress := float64(i) / float64(anchorCount-1)
		baseline := startTarget + (profile.totalSlope * progress)
		noise := rng.NormFloat64() * profile.anchorNoise

		jump := 0.0
		if i < anchorCount-1 && rng.Float64() < profile.jumpChance {
			jump = (rng.Float64()*2 - 1) * profile.jumpAmplitude
		}

		anchorValues[i] = clampFloat(baseline+noise+jump, min, max)
	}
	anchorValues[anchorCount-1] = current

	raw := make([]float64, points)
	for seg := 0; seg < anchorCount-1; seg++ {
		i0 := anchorIndexes[seg]
		i1 := anchorIndexes[seg+1]
		v0 := anchorValues[seg]
		v1 := anchorValues[seg+1]

		if i1 <= i0 {
			raw[i0] = v1
			continue
		}

		spanIdx := float64(i1 - i0)
		for i := i0; i <= i1; i++ {
			t := float64(i-i0) / spanIdx
			raw[i] = v0 + ((v1 - v0) * t)
		}
	}

	// Overlay jagged waveform components (triangle + saw), then inject sparse
	// burst events to create sharper inflections for sparkline readability.
	cycles := 2 + int(seed%5)
	phaseOffset := float64(seed%17) / 17.0
	for i := 0; i < points; i++ {
		progress := float64(i) / lastIdx
		waveInput := (progress * float64(cycles)) + phaseOffset

		tri := triangleWave(waveInput)
		saw := sawWave(waveInput * 0.7)
		jitterScale := 0.4 + (0.6 * (1 - progress))
		jitter := rng.NormFloat64() * profile.jitter * jitterScale

		raw[i] += (tri * profile.triangleAmplitude) + (saw * profile.sawAmplitude) + jitter
	}

	burstCount := int(math.Round(float64(points) * profile.burstDensity))
	if class == trendVolatile && burstCount < 2 {
		burstCount = 2
	}
	if burstCount > points/3 {
		burstCount = points / 3
	}
	for b := 0; b < burstCount; b++ {
		if points < 3 {
			break
		}
		center := 1 + rng.Intn(points-2)
		width := 1 + rng.Intn(maxInt(2, points/40+1))
		magnitude := (rng.Float64()*2 - 1) * profile.burstAmplitude

		start := center - width
		if start < 0 {
			start = 0
		}
		end := center + width
		if end >= points {
			end = points - 1
		}

		for i := start; i <= end; i++ {
			distance := math.Abs(float64(i - center))
			weight := 1 - (distance / float64(width+1))
			if weight < 0 {
				weight = 0
			}
			raw[i] += magnitude * weight
		}
	}

	if profile.stepSize > 0 {
		for i := 0; i < points; i++ {
			raw[i] = math.Round(raw[i]/profile.stepSize) * profile.stepSize
		}
	}

	// Shift so the last point exactly matches current.
	offset := current - raw[points-1]
	for i := range raw {
		raw[i] = clampFloat(raw[i]+offset, min, max)
	}
	raw[points-1] = current
	return raw
}

type seededTrendProfile struct {
	totalSlope        float64
	anchorNoise       float64
	jumpChance        float64
	jumpAmplitude     float64
	triangleAmplitude float64
	sawAmplitude      float64
	jitter            float64
	burstDensity      float64
	burstAmplitude    float64
	stepSize          float64
	segmentMin        int
	segmentMax        int
}

func getSeededTrendProfile(class seededTrendClass, seed uint64, span float64) seededTrendProfile {
	slopeFactor := 0.06 + (float64(seed%7) * 0.012)
	stepBase := span * (0.006 + float64(seed%4)*0.0015)

	switch class {
	case trendGrowing:
		return seededTrendProfile{
			totalSlope:        span * slopeFactor,
			anchorNoise:       span * 0.018,
			jumpChance:        0.16,
			jumpAmplitude:     span * 0.07,
			triangleAmplitude: span * 0.020,
			sawAmplitude:      span * 0.014,
			jitter:            span * 0.010,
			burstDensity:      0.015,
			burstAmplitude:    span * 0.11,
			stepSize:          stepBase,
			segmentMin:        6,
			segmentMax:        11,
		}
	case trendDeclining:
		return seededTrendProfile{
			totalSlope:        -span * slopeFactor,
			anchorNoise:       span * 0.018,
			jumpChance:        0.16,
			jumpAmplitude:     span * 0.07,
			triangleAmplitude: span * 0.020,
			sawAmplitude:      span * 0.014,
			jitter:            span * 0.010,
			burstDensity:      0.015,
			burstAmplitude:    span * 0.11,
			stepSize:          stepBase,
			segmentMin:        6,
			segmentMax:        11,
		}
	case trendVolatile:
		return seededTrendProfile{
			totalSlope:        span * ((float64(seed%5) - 2) * 0.015),
			anchorNoise:       span * 0.040,
			jumpChance:        0.28,
			jumpAmplitude:     span * 0.12,
			triangleAmplitude: span * 0.040,
			sawAmplitude:      span * 0.030,
			jitter:            span * 0.022,
			burstDensity:      0.030,
			burstAmplitude:    span * 0.18,
			stepSize:          stepBase * 0.8,
			segmentMin:        8,
			segmentMax:        14,
		}
	default:
		return seededTrendProfile{
			totalSlope:        span * ((float64(seed%5) - 2) * 0.01),
			anchorNoise:       span * 0.012,
			jumpChance:        0.10,
			jumpAmplitude:     span * 0.05,
			triangleAmplitude: span * 0.014,
			sawAmplitude:      span * 0.010,
			jitter:            span * 0.007,
			burstDensity:      0.008,
			burstAmplitude:    span * 0.08,
			stepSize:          stepBase,
			segmentMin:        5,
			segmentMax:        9,
		}
	}
}

func triangleWave(x float64) float64 {
	phase := x - math.Floor(x)
	return 1 - (4 * math.Abs(phase-0.5))
}

func sawWave(x float64) float64 {
	phase := x - math.Floor(x)
	return (2 * phase) - 1
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func seedMockMetricsHistory(mh *MetricsHistory, ms *metrics.Store, state models.StateSnapshot, now time.Time, seedDuration, interval time.Duration) {
	if mh == nil {
		return
	}
	if seedDuration <= 0 || interval <= 0 {
		return
	}

	// Choose a seed interval that respects the requested sample interval
	// while keeping the total number of points bounded.
	seedInterval := interval
	if seedInterval <= 0 {
		seedInterval = 30 * time.Second
	}
	const maxSeedPoints = 2000
	if seedDuration/seedInterval > maxSeedPoints {
		seedInterval = seedDuration / maxSeedPoints
	}
	if seedInterval < 30*time.Second {
		seedInterval = 30 * time.Second
	}
	const seedBatchSize = 5000

	var seedBatch []metrics.WriteMetric
	queueMetric := func(resourceType, resourceID, metricType string, value float64, ts time.Time) {
		if ms == nil {
			return
		}
		seedBatch = append(seedBatch,
			metrics.WriteMetric{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				MetricType:   metricType,
				Value:        value,
				Timestamp:    ts,
				Tier:         metrics.TierHourly,
			},
			metrics.WriteMetric{
				ResourceType: resourceType,
				ResourceID:   resourceID,
				MetricType:   metricType,
				Value:        value,
				Timestamp:    ts,
				Tier:         metrics.TierDaily,
			},
		)

		if len(seedBatch) >= seedBatchSize {
			ms.WriteBatchSync(seedBatch)
			seedBatch = seedBatch[:0]
		}
	}

	recordNode := func(node models.Node) {
		if node.ID == "" {
			return
		}

		numPoints := int(seedDuration / seedInterval)
		cpuSeries := generateSeededSeries(node.CPU*100, numPoints, hashSeed("node", node.ID, "cpu"), 5, 85)
		memSeries := generateSeededSeries(node.Memory.Usage, numPoints, hashSeed("node", node.ID, "memory"), 10, 85)
		diskSeries := generateSeededSeries(node.Disk.Usage, numPoints, hashSeed("node", node.ID, "disk"), 5, 95)

		startTime := now.Add(-seedDuration)
		for i := 0; i < numPoints; i++ {
			ts := startTime.Add(time.Duration(i) * seedInterval)
			mh.AddNodeMetric(node.ID, "cpu", cpuSeries[i], ts)
			mh.AddNodeMetric(node.ID, "memory", memSeries[i], ts)
			mh.AddNodeMetric(node.ID, "disk", diskSeries[i], ts)
			queueMetric("node", node.ID, "cpu", cpuSeries[i], ts)
			queueMetric("node", node.ID, "memory", memSeries[i], ts)
			queueMetric("node", node.ID, "disk", diskSeries[i], ts)
		}

		// Ensure the latest point lands at "now" for full-range charts.
		mh.AddNodeMetric(node.ID, "cpu", node.CPU*100, now)
		mh.AddNodeMetric(node.ID, "memory", node.Memory.Usage, now)
		mh.AddNodeMetric(node.ID, "disk", node.Disk.Usage, now)
		queueMetric("node", node.ID, "cpu", node.CPU*100, now)
		queueMetric("node", node.ID, "memory", node.Memory.Usage, now)
		queueMetric("node", node.ID, "disk", node.Disk.Usage, now)
	}

	recordGuest := func(
		metricID, storeType, storeID string,
		cpuPercent, memPercent, diskPercent, diskRead, diskWrite, netIn, netOut float64,
		includeDisk bool,
		includeDiskIO bool,
		includeNetwork bool,
	) {
		if metricID == "" || storeID == "" {
			return
		}

		numPoints := int(seedDuration / seedInterval)
		cpuSeries := generateSeededSeries(cpuPercent, numPoints, hashSeed(storeType, storeID, "cpu"), 0, 100)
		memSeries := generateSeededSeries(memPercent, numPoints, hashSeed(storeType, storeID, "memory"), 0, 100)
		var diskSeries []float64
		if includeDisk {
			diskSeries = generateSeededSeries(diskPercent, numPoints, hashSeed(storeType, storeID, "disk"), 0, 100)
		}
		ioMax := func(value float64) float64 {
			return math.Max(value*1.8, 1)
		}
		var diskReadSeries, diskWriteSeries, netInSeries, netOutSeries []float64
		if includeDiskIO {
			diskReadSeries = generateSeededSeries(diskRead, numPoints, hashSeed(storeType, storeID, "diskread"), 0, ioMax(diskRead))
			diskWriteSeries = generateSeededSeries(diskWrite, numPoints, hashSeed(storeType, storeID, "diskwrite"), 0, ioMax(diskWrite))
		}
		if includeNetwork {
			netInSeries = generateSeededSeries(netIn, numPoints, hashSeed(storeType, storeID, "netin"), 0, ioMax(netIn))
			netOutSeries = generateSeededSeries(netOut, numPoints, hashSeed(storeType, storeID, "netout"), 0, ioMax(netOut))
		}

		startTime := now.Add(-seedDuration)
		for i := 0; i < numPoints; i++ {
			ts := startTime.Add(time.Duration(i) * seedInterval)
			mh.AddGuestMetric(metricID, "cpu", cpuSeries[i], ts)
			mh.AddGuestMetric(metricID, "memory", memSeries[i], ts)
			queueMetric(storeType, storeID, "cpu", cpuSeries[i], ts)
			queueMetric(storeType, storeID, "memory", memSeries[i], ts)
			if includeDisk {
				mh.AddGuestMetric(metricID, "disk", diskSeries[i], ts)
				queueMetric(storeType, storeID, "disk", diskSeries[i], ts)
			}
			if includeDiskIO {
				mh.AddGuestMetric(metricID, "diskread", diskReadSeries[i], ts)
				mh.AddGuestMetric(metricID, "diskwrite", diskWriteSeries[i], ts)
				queueMetric(storeType, storeID, "diskread", diskReadSeries[i], ts)
				queueMetric(storeType, storeID, "diskwrite", diskWriteSeries[i], ts)
			}
			if includeNetwork {
				mh.AddGuestMetric(metricID, "netin", netInSeries[i], ts)
				mh.AddGuestMetric(metricID, "netout", netOutSeries[i], ts)
				queueMetric(storeType, storeID, "netin", netInSeries[i], ts)
				queueMetric(storeType, storeID, "netout", netOutSeries[i], ts)
			}
		}

		// Ensure the latest point lands at "now" for full-range charts.
		mh.AddGuestMetric(metricID, "cpu", cpuPercent, now)
		mh.AddGuestMetric(metricID, "memory", memPercent, now)
		queueMetric(storeType, storeID, "cpu", cpuPercent, now)
		queueMetric(storeType, storeID, "memory", memPercent, now)
		if includeDisk {
			mh.AddGuestMetric(metricID, "disk", diskPercent, now)
			queueMetric(storeType, storeID, "disk", diskPercent, now)
		}
		if includeDiskIO {
			mh.AddGuestMetric(metricID, "diskread", diskRead, now)
			mh.AddGuestMetric(metricID, "diskwrite", diskWrite, now)
			queueMetric(storeType, storeID, "diskread", diskRead, now)
			queueMetric(storeType, storeID, "diskwrite", diskWrite, now)
		}
		if includeNetwork {
			mh.AddGuestMetric(metricID, "netin", netIn, now)
			mh.AddGuestMetric(metricID, "netout", netOut, now)
			queueMetric(storeType, storeID, "netin", netIn, now)
			queueMetric(storeType, storeID, "netout", netOut, now)
		}
	}

	log.Debug().Int("count", len(state.Nodes)).Msg("Mock seeding: processing nodes")
	for _, node := range state.Nodes {
		recordNode(node)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	runningVMs := 0
	for _, vm := range state.VMs {
		if vm.Status == "running" {
			runningVMs++
		}
	}
	log.Debug().Int("total", len(state.VMs)).Int("running", runningVMs).Msg("Mock seeding: processing VMs")
	for _, vm := range state.VMs {
		if vm.Status != "running" {
			continue
		}
		recordGuest(vm.ID, "vm", vm.ID, vm.CPU*100, vm.Memory.Usage, vm.Disk.Usage, float64(vm.DiskRead), float64(vm.DiskWrite), float64(vm.NetworkIn), float64(vm.NetworkOut), true, true, true)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	runningContainers := 0
	for _, ct := range state.Containers {
		if ct.Status == "running" {
			runningContainers++
		}
	}
	log.Debug().Int("total", len(state.Containers)).Int("running", runningContainers).Msg("Mock seeding: processing containers")
	for _, ct := range state.Containers {
		if ct.Status != "running" {
			continue
		}
		recordGuest(ct.ID, "container", ct.ID, ct.CPU*100, ct.Memory.Usage, ct.Disk.Usage, float64(ct.DiskRead), float64(ct.DiskWrite), float64(ct.NetworkIn), float64(ct.NetworkOut), true, true, true)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	k8sPodCount := 0
	for _, cluster := range state.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		k8sPodCount += len(cluster.Pods)
	}
	log.Debug().Int("clusters", len(state.KubernetesClusters)).Int("pods", k8sPodCount).Msg("Mock seeding: processing kubernetes pods")
	for _, cluster := range state.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		for _, pod := range cluster.Pods {
			metricID := kubernetesPodMetricID(cluster, pod)
			if metricID == "" {
				continue
			}
			current := kubernetesPodCurrentMetrics(cluster, pod)
			recordGuest(
				metricID,
				"k8s",
				metricID,
				current["cpu"],
				current["memory"],
				current["disk"],
				current["diskread"],
				current["diskwrite"],
				current["netin"],
				current["netout"],
				true,
				false,
				true,
			)
		}
		time.Sleep(30 * time.Millisecond)
	}

	log.Debug().Int("count", len(state.Storage)).Msg("Mock seeding: processing storage")
	for _, storage := range state.Storage {
		if storage.ID == "" {
			continue
		}
		numPoints := int(seedDuration / seedInterval)
		usageSeries := generateSeededSeries(storage.Usage, numPoints, hashSeed("storage", storage.ID, "usage"), 0, 100)

		startTime := now.Add(-seedDuration)
		for i := 0; i < numPoints; i++ {
			ts := startTime.Add(time.Duration(i) * seedInterval)
			mh.AddStorageMetric(storage.ID, "usage", usageSeries[i], ts)
			queueMetric("storage", storage.ID, "usage", usageSeries[i], ts)
		}

		// Ensure the latest point lands at "now" for full-range charts.
		mh.AddStorageMetric(storage.ID, "usage", storage.Usage, now)
		queueMetric("storage", storage.ID, "usage", storage.Usage, now)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	log.Debug().Int("count", len(state.PhysicalDisks)).Msg("Mock seeding: processing physical disks")
	for _, disk := range state.PhysicalDisks {
		if disk.Temperature <= 0 {
			continue
		}
		resourceID := diskMetricsResourceID(disk)
		if resourceID == "" {
			continue
		}

		numPoints := int(seedDuration / seedInterval)
		tempSeries := generateSeededSeries(
			float64(disk.Temperature),
			numPoints,
			hashSeed("disk", resourceID, "smart_temp"),
			25,
			95,
		)
		startTime := now.Add(-seedDuration)
		for i := 0; i < numPoints; i++ {
			ts := startTime.Add(time.Duration(i) * seedInterval)
			queueMetric("disk", resourceID, "smart_temp", tempSeries[i], ts)
		}

		// Ensure the latest point lands at "now" for full-range charts.
		queueMetric("disk", resourceID, "smart_temp", float64(disk.Temperature), now)
	}

	log.Debug().Int("count", len(state.DockerHosts)).Msg("Mock seeding: processing docker hosts")
	for _, host := range state.DockerHosts {
		if host.ID == "" {
			continue
		}

		var diskPercent float64
		var usedTotal, totalTotal int64
		for _, d := range host.Disks {
			if d.Total > 0 {
				usedTotal += d.Used
				totalTotal += d.Total
			}
		}
		if totalTotal > 0 {
			diskPercent = float64(usedTotal) / float64(totalTotal) * 100
		}

		recordGuest("dockerHost:"+host.ID, "dockerHost", host.ID, host.CPUUsage, host.Memory.Usage, diskPercent, 0, 0, 0, 0, true, false, false)

		for _, container := range host.Containers {
			if container.ID == "" || container.State != "running" {
				continue
			}

			var containerDisk float64
			if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
				containerDisk = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
				containerDisk = clampFloat(containerDisk, 0, 100)
			}
			recordGuest("docker:"+container.ID, "docker", container.ID, container.CPUPercent, container.MemoryPercent, containerDisk, 0, 0, 0, 0, true, false, false)
		}
		time.Sleep(50 * time.Millisecond) // Add delay for docker hosts
	}

	if ms != nil && len(seedBatch) > 0 {
		ms.WriteBatchSync(seedBatch)
	}
	log.Debug().Msg("Mock seeding: completed")
}

func recordMockStateToMetricsHistory(mh *MetricsHistory, ms *metrics.Store, state models.StateSnapshot, ts time.Time) {
	if mh == nil {
		return
	}

	for _, node := range state.Nodes {
		if node.ID == "" || node.Status != "online" {
			continue
		}
		mh.AddNodeMetric(node.ID, "cpu", node.CPU*100, ts)
		mh.AddNodeMetric(node.ID, "memory", node.Memory.Usage, ts)
		mh.AddNodeMetric(node.ID, "disk", node.Disk.Usage, ts)

		if ms != nil {
			ms.Write("node", node.ID, "cpu", node.CPU*100, ts)
			ms.Write("node", node.ID, "memory", node.Memory.Usage, ts)
			ms.Write("node", node.ID, "disk", node.Disk.Usage, ts)
		}
	}

	for _, vm := range state.VMs {
		if vm.ID == "" || vm.Status != "running" {
			continue
		}
		mh.AddGuestMetric(vm.ID, "cpu", vm.CPU*100, ts)
		mh.AddGuestMetric(vm.ID, "memory", vm.Memory.Usage, ts)
		mh.AddGuestMetric(vm.ID, "disk", vm.Disk.Usage, ts)
		mh.AddGuestMetric(vm.ID, "diskread", float64(vm.DiskRead), ts)
		mh.AddGuestMetric(vm.ID, "diskwrite", float64(vm.DiskWrite), ts)
		mh.AddGuestMetric(vm.ID, "netin", float64(vm.NetworkIn), ts)
		mh.AddGuestMetric(vm.ID, "netout", float64(vm.NetworkOut), ts)

		if ms != nil {
			ms.Write("vm", vm.ID, "cpu", vm.CPU*100, ts)
			ms.Write("vm", vm.ID, "memory", vm.Memory.Usage, ts)
			ms.Write("vm", vm.ID, "disk", vm.Disk.Usage, ts)
			ms.Write("vm", vm.ID, "diskread", float64(vm.DiskRead), ts)
			ms.Write("vm", vm.ID, "diskwrite", float64(vm.DiskWrite), ts)
			ms.Write("vm", vm.ID, "netin", float64(vm.NetworkIn), ts)
			ms.Write("vm", vm.ID, "netout", float64(vm.NetworkOut), ts)
		}
	}

	for _, ct := range state.Containers {
		if ct.ID == "" || ct.Status != "running" {
			continue
		}
		mh.AddGuestMetric(ct.ID, "cpu", ct.CPU*100, ts)
		mh.AddGuestMetric(ct.ID, "memory", ct.Memory.Usage, ts)
		mh.AddGuestMetric(ct.ID, "disk", ct.Disk.Usage, ts)
		mh.AddGuestMetric(ct.ID, "diskread", float64(ct.DiskRead), ts)
		mh.AddGuestMetric(ct.ID, "diskwrite", float64(ct.DiskWrite), ts)
		mh.AddGuestMetric(ct.ID, "netin", float64(ct.NetworkIn), ts)
		mh.AddGuestMetric(ct.ID, "netout", float64(ct.NetworkOut), ts)

		if ms != nil {
			ms.Write("container", ct.ID, "cpu", ct.CPU*100, ts)
			ms.Write("container", ct.ID, "memory", ct.Memory.Usage, ts)
			ms.Write("container", ct.ID, "disk", ct.Disk.Usage, ts)
			ms.Write("container", ct.ID, "diskread", float64(ct.DiskRead), ts)
			ms.Write("container", ct.ID, "diskwrite", float64(ct.DiskWrite), ts)
			ms.Write("container", ct.ID, "netin", float64(ct.NetworkIn), ts)
			ms.Write("container", ct.ID, "netout", float64(ct.NetworkOut), ts)
		}
	}

	for _, cluster := range state.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		for _, pod := range cluster.Pods {
			metricID := kubernetesPodMetricID(cluster, pod)
			if metricID == "" {
				continue
			}
			current := kubernetesPodCurrentMetrics(cluster, pod)
			mh.AddGuestMetric(metricID, "cpu", current["cpu"], ts)
			mh.AddGuestMetric(metricID, "memory", current["memory"], ts)
			if current["disk"] > 0 {
				mh.AddGuestMetric(metricID, "disk", current["disk"], ts)
			}
			if current["diskread"] > 0 {
				mh.AddGuestMetric(metricID, "diskread", current["diskread"], ts)
			}
			if current["diskwrite"] > 0 {
				mh.AddGuestMetric(metricID, "diskwrite", current["diskwrite"], ts)
			}
			if current["netin"] > 0 {
				mh.AddGuestMetric(metricID, "netin", current["netin"], ts)
			}
			if current["netout"] > 0 {
				mh.AddGuestMetric(metricID, "netout", current["netout"], ts)
			}

			if ms != nil {
				ms.Write("k8s", metricID, "cpu", current["cpu"], ts)
				ms.Write("k8s", metricID, "memory", current["memory"], ts)
				if current["disk"] > 0 {
					ms.Write("k8s", metricID, "disk", current["disk"], ts)
				}
				if current["diskread"] > 0 {
					ms.Write("k8s", metricID, "diskread", current["diskread"], ts)
				}
				if current["diskwrite"] > 0 {
					ms.Write("k8s", metricID, "diskwrite", current["diskwrite"], ts)
				}
				if current["netin"] > 0 {
					ms.Write("k8s", metricID, "netin", current["netin"], ts)
				}
				if current["netout"] > 0 {
					ms.Write("k8s", metricID, "netout", current["netout"], ts)
				}
			}
		}
	}

	for _, storage := range state.Storage {
		if storage.ID == "" || storage.Status != "available" {
			continue
		}
		mh.AddStorageMetric(storage.ID, "usage", storage.Usage, ts)
		mh.AddStorageMetric(storage.ID, "used", float64(storage.Used), ts)
		mh.AddStorageMetric(storage.ID, "total", float64(storage.Total), ts)
		mh.AddStorageMetric(storage.ID, "avail", float64(storage.Free), ts)

		if ms != nil {
			ms.Write("storage", storage.ID, "usage", storage.Usage, ts)
		}
	}

	for _, disk := range state.PhysicalDisks {
		if disk.Temperature <= 0 {
			continue
		}
		resourceID := diskMetricsResourceID(disk)
		if resourceID == "" {
			continue
		}

		if ms != nil {
			ms.Write("disk", resourceID, "smart_temp", float64(disk.Temperature), ts)
		}
	}

	for _, host := range state.DockerHosts {
		if host.ID == "" || host.Status != "online" {
			continue
		}

		var diskPercent float64
		var usedTotal, totalTotal int64
		for _, d := range host.Disks {
			if d.Total > 0 {
				usedTotal += d.Used
				totalTotal += d.Total
			}
		}
		if totalTotal > 0 {
			diskPercent = float64(usedTotal) / float64(totalTotal) * 100
		}

		hostKey := "dockerHost:" + host.ID
		mh.AddGuestMetric(hostKey, "cpu", host.CPUUsage, ts)
		mh.AddGuestMetric(hostKey, "memory", host.Memory.Usage, ts)
		mh.AddGuestMetric(hostKey, "disk", diskPercent, ts)

		if ms != nil {
			ms.Write("dockerHost", host.ID, "cpu", host.CPUUsage, ts)
			ms.Write("dockerHost", host.ID, "memory", host.Memory.Usage, ts)
			ms.Write("dockerHost", host.ID, "disk", diskPercent, ts)
		}

		for _, container := range host.Containers {
			if container.ID == "" || container.State != "running" {
				continue
			}

			var containerDisk float64
			if container.RootFilesystemBytes > 0 && container.WritableLayerBytes > 0 {
				containerDisk = float64(container.WritableLayerBytes) / float64(container.RootFilesystemBytes) * 100
				containerDisk = clampFloat(containerDisk, 0, 100)
			}

			metricKey := "docker:" + container.ID
			mh.AddGuestMetric(metricKey, "cpu", container.CPUPercent, ts)
			mh.AddGuestMetric(metricKey, "memory", container.MemoryPercent, ts)
			mh.AddGuestMetric(metricKey, "disk", containerDisk, ts)

			if ms != nil {
				ms.Write("docker", container.ID, "cpu", container.CPUPercent, ts)
				ms.Write("docker", container.ID, "memory", container.MemoryPercent, ts)
				ms.Write("docker", container.ID, "disk", containerDisk, ts)
			}
		}
	}
}

func diskMetricsResourceID(disk models.PhysicalDisk) string {
	resourceID := strings.TrimSpace(disk.Serial)
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.WWN)
	}
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.ID)
	}
	if resourceID == "" {
		resourceID = fmt.Sprintf("%s-%s-%s", disk.Instance, disk.Node, strings.ReplaceAll(disk.DevPath, "/", "-"))
	}
	return resourceID
}

func (m *Monitor) startMockMetricsSampler(ctx context.Context) {
	if ctx == nil || m == nil {
		log.Debug().Msg("Mock metrics sampler: nil context or monitor")
		return
	}
	if !mock.IsMockEnabled() {
		log.Debug().Msg("Mock metrics sampler: mock mode not enabled")
		return
	}

	log.Info().Msg("Mock metrics sampler: starting initialization")

	cfg := mockMetricsSamplerConfigFromEnv()
	seedDuration := cfg.SeedDuration
	// Reduced minimum from 7 days to 1 hour for faster startup on resource-constrained systems
	if seedDuration < time.Hour {
		seedDuration = time.Hour
	}
	maxPoints := int(seedDuration / cfg.SampleInterval)

	m.mu.Lock()
	if m.mockMetricsCancel != nil {
		m.mu.Unlock()
		log.Debug().Msg("Mock metrics sampler: already running")
		return
	}
	samplerCtx, cancel := context.WithCancel(ctx)
	m.mockMetricsCancel = cancel
	m.metricsHistory = NewMetricsHistory(maxPoints, seedDuration)
	m.mu.Unlock()

	state := mock.GetMockState()
	log.Info().
		Int("nodes", len(state.Nodes)).
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Dur("seedDuration", seedDuration).
		Dur("sampleInterval", cfg.SampleInterval).
		Msg("Mock metrics sampler: seeding historical data")
	// Keep mock trend generation in-memory only so production history in the
	// persistent metrics store remains untouched while mock mode is active.
	seedMockMetricsHistory(m.metricsHistory, nil, state, time.Now(), seedDuration, cfg.SampleInterval)
	recordMockStateToMetricsHistory(m.metricsHistory, nil, state, time.Now())

	m.mockMetricsWg.Add(1)
	go func() {
		defer m.mockMetricsWg.Done()

		ticker := time.NewTicker(cfg.SampleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-samplerCtx.Done():
				return
			case <-ticker.C:
				if !mock.IsMockEnabled() {
					continue
				}
				recordMockStateToMetricsHistory(m.metricsHistory, nil, mock.GetMockState(), time.Now())
			}
		}
	}()

	log.Info().
		Dur("seedDuration", seedDuration).
		Dur("sampleInterval", cfg.SampleInterval).
		Msg("Mock metrics history sampler started")
}

func (m *Monitor) stopMockMetricsSampler() {
	if m == nil {
		return
	}

	m.mu.Lock()
	cancel := m.mockMetricsCancel
	m.mockMetricsCancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
		m.mockMetricsWg.Wait()
		log.Info().Msg("Mock metrics history sampler stopped")
	}
}
