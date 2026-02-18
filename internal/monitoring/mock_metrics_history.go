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
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
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

// HashSeed produces a deterministic uint64 from the given parts.
func HashSeed(parts ...string) uint64 {
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
	seed := HashSeed("k8s-pod-current", clusterKey, podKey)
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

// SeriesStyle controls the visual character of generated metric series.
type SeriesStyle int

const (
	// StyleSpiky: low baseline with sharp spike events (CPU, network/disk I/O).
	StyleSpiky SeriesStyle = iota
	// StylePlateau: stable level with slow drift between plateaus (memory).
	StylePlateau
	// StyleFlat: nearly constant with very gradual trend (disk usage, temperature).
	StyleFlat
)

// Package-level aliases for internal callers that use the unexported names.
const (
	styleSpiky   = StyleSpiky
	stylePlateau = StylePlateau
	styleFlat    = StyleFlat
)

// GenerateSeededSeries produces a deterministic metric series with the
// given style. Exported so the API chart layer can use the same generation.
func GenerateSeededSeries(current float64, points int, seed uint64, min, max float64, style SeriesStyle) []float64 {
	current = clampFloat(current, min, max)
	if points <= 1 {
		return []float64{current}
	}

	rng := rand.New(rand.NewSource(int64(seed)))
	span := math.Max(1, max-min)

	var raw []float64
	switch style {
	case styleSpiky:
		raw = generateSpikySeries(current, points, seed, min, max, span, rng)
	case stylePlateau:
		raw = generatePlateauSeries(current, points, min, max, span, rng)
	default:
		raw = generateFlatSeries(current, points, min, max, span, rng)
	}

	for i := range raw {
		raw[i] = clampFloat(raw[i], min, max)
	}
	raw[points-1] = current
	return raw
}

// generateSpikySeries produces a low baseline with occasional sharp spikes —
// matching how real CPU and I/O metrics behave (mostly idle, with bursts).
func generateSpikySeries(current float64, points int, seed uint64, min, max, span float64, rng *rand.Rand) []float64 {
	// Baseline: most resources idle low. Seed determines personality tier.
	personality := seed % 20
	r := rng.Float64()
	var baselineFraction float64
	switch {
	case personality == 0: // 5%: busy resource
		baselineFraction = 0.30 + r*0.15
	case personality <= 4: // 20%: moderate
		baselineFraction = 0.16 + r*0.14
	default: // 75%: idle
		baselineFraction = 0.04 + r*0.12
	}
	baseline := min + span*baselineFraction

	// Small drift over the series to avoid perfect flatness.
	baselineDrift := span * 0.03 * (rng.Float64()*2 - 1)

	raw := make([]float64, points)
	for i := range raw {
		progress := float64(i) / float64(points-1)
		raw[i] = baseline + baselineDrift*progress + rng.NormFloat64()*span*0.004
	}

	// Generate spike events: fast rise, power-law decay.
	spikeMinWidth := 3 + int(seed%6)
	spikeProb := 0.015 + float64(seed%10)*0.003
	for i := 0; i < points-spikeMinWidth; {
		if rng.Float64() < spikeProb {
			height := span * (0.10 + rng.Float64()*0.45)
			width := spikeMinWidth + rng.Intn(10)
			if i+width > points {
				width = points - i
			}
			for j := 0; j < width; j++ {
				progress := float64(j) / float64(width)
				var envelope float64
				if progress < 0.12 {
					envelope = progress / 0.12
				} else {
					decay := (progress - 0.12) / 0.88
					envelope = math.Pow(1-decay, 1.5)
				}
				raw[i+j] = math.Max(raw[i+j], baseline+height*envelope)
			}
			i += width + 2 + rng.Intn(5) // gap after spike
		} else {
			i++
		}
	}

	// Taper-shift so the last point approaches current without distorting spikes.
	diff := current - raw[points-1]
	taperLen := points / 8
	if taperLen < 5 {
		taperLen = 5
	}
	if taperLen > points {
		taperLen = points
	}
	for j := 0; j < taperLen; j++ {
		idx := points - 1 - j
		weight := float64(taperLen-j) / float64(taperLen)
		raw[idx] += diff * weight
	}
	return raw
}

// generatePlateauSeries produces stable levels with slow transitions —
// matching how real memory usage behaves (applications hold allocations).
func generatePlateauSeries(current float64, points int, min, max, span float64, rng *rand.Rand) []float64 {
	plateauCount := 3 + rng.Intn(4) // 3-6 plateaus

	// Generate plateau levels near current.
	levels := make([]float64, plateauCount)
	for i := range levels {
		offset := (rng.Float64()*2 - 1) * span * 0.12
		levels[i] = clampFloat(current+offset, min, max)
	}
	levels[plateauCount-1] = current + rng.NormFloat64()*span*0.01

	raw := make([]float64, points)
	segmentLen := points / plateauCount

	for i := 0; i < points; i++ {
		seg := i / segmentLen
		if seg >= plateauCount {
			seg = plateauCount - 1
		}
		nextSeg := seg + 1
		if nextSeg >= plateauCount {
			nextSeg = plateauCount - 1
		}

		segEnd := segmentLen
		if seg == plateauCount-1 {
			segEnd = points - seg*segmentLen
		}
		if segEnd <= 0 {
			segEnd = 1
		}
		posInSeg := i - seg*segmentLen
		progress := float64(posInSeg) / float64(segEnd)

		// Smooth transition in last 20% of each segment.
		var value float64
		if progress < 0.8 || seg == nextSeg {
			value = levels[seg]
		} else {
			t := (progress - 0.8) / 0.2
			t = t * t * (3 - 2*t) // smoothstep
			value = levels[seg]*(1-t) + levels[nextSeg]*t
		}
		raw[i] = value + rng.NormFloat64()*span*0.002
	}
	return raw
}

// generateFlatSeries produces a nearly constant series with very gradual
// trend — matching disk usage or temperature that barely changes.
func generateFlatSeries(current float64, points int, min, max, span float64, rng *rand.Rand) []float64 {
	raw := make([]float64, points)

	trendDir := 1.0
	if rng.Float64() < 0.33 {
		trendDir = -1.0
	}
	totalDrift := span * 0.02 * trendDir

	for i := range raw {
		progress := float64(i) / float64(points-1)
		raw[i] = current - totalDrift*(1-progress) + rng.NormFloat64()*span*0.001
	}
	return raw
}

// buildTieredTimestamps generates a sorted list of timestamps with denser
// intervals for recent data:
//
//	Last 2h:   1min intervals  (~120 points)
//	2h–24h:    2min intervals  (~660 points)
//	24h–end:   ~65min intervals (variable)
//
// This ensures short time ranges (1h, 4h) have enough data points without
// needing an API-level fallback layer.
func buildTieredTimestamps(now time.Time, totalDuration time.Duration) []time.Time {
	// Each segment covers [now - startOffset, now - endOffset) and is walked
	// chronologically from oldest to newest. Segments are defined oldest-first
	// so the resulting slice is in chronological order.
	type segment struct {
		startOffset time.Duration // how far back from now this segment starts
		endOffset   time.Duration // how far back from now this segment ends
		interval    time.Duration
	}
	segments := []segment{
		{totalDuration, 24 * time.Hour, 65 * time.Minute},
		{24 * time.Hour, 2 * time.Hour, 2 * time.Minute},
		{2 * time.Hour, 0, time.Minute},
	}

	var timestamps []time.Time

	for _, seg := range segments {
		startOff := seg.startOffset
		if startOff > totalDuration {
			startOff = totalDuration
		}
		endOff := seg.endOffset
		if endOff > totalDuration {
			endOff = totalDuration
		}
		if startOff <= endOff {
			continue
		}

		segStart := now.Add(-startOff)
		segEnd := now.Add(-endOff)

		for ts := segStart; ts.Before(segEnd); ts = ts.Add(seg.interval) {
			timestamps = append(timestamps, ts)
		}
	}

	// Add "now" as the final point
	if len(timestamps) > 0 {
		last := timestamps[len(timestamps)-1]
		if !last.Equal(now) {
			timestamps = append(timestamps, now)
		}
	}

	return timestamps
}

func seedMockMetricsHistory(mh *MetricsHistory, ms *metrics.Store, state models.StateSnapshot, now time.Time, seedDuration, interval time.Duration) {
	if mh == nil {
		return
	}
	if seedDuration <= 0 || interval <= 0 {
		return
	}

	// Build a tiered timestamp list so short time ranges (1h, 4h) have dense
	// data without needing an API-level fallback layer.
	//   Last 2h:   30s intervals  (~240 points)
	//   2h–24h:    2min intervals  (~660 points)
	//   24h–90d:   ~65min intervals (~1920 points)
	// Total: ~2820 points per metric per resource.
	seedTimestamps := buildTieredTimestamps(now, seedDuration)
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

		numPoints := len(seedTimestamps)
		cpuSeries := GenerateSeededSeries(node.CPU*100, numPoints, HashSeed("node", node.ID, "cpu"), 5, 85, styleSpiky)
		memSeries := GenerateSeededSeries(node.Memory.Usage, numPoints, HashSeed("node", node.ID, "memory"), 10, 85, stylePlateau)
		diskSeries := GenerateSeededSeries(node.Disk.Usage, numPoints, HashSeed("node", node.ID, "disk"), 5, 95, styleFlat)

		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
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

		numPoints := len(seedTimestamps)
		cpuSeries := GenerateSeededSeries(cpuPercent, numPoints, HashSeed(storeType, storeID, "cpu"), 0, 100, styleSpiky)
		memSeries := GenerateSeededSeries(memPercent, numPoints, HashSeed(storeType, storeID, "memory"), 0, 100, stylePlateau)
		var diskSeries []float64
		if includeDisk {
			diskSeries = GenerateSeededSeries(diskPercent, numPoints, HashSeed(storeType, storeID, "disk"), 0, 100, styleFlat)
		}
		ioMax := func(value float64) float64 {
			return math.Max(value*1.8, 1)
		}
		var diskReadSeries, diskWriteSeries, netInSeries, netOutSeries []float64
		if includeDiskIO {
			diskReadSeries = GenerateSeededSeries(diskRead, numPoints, HashSeed(storeType, storeID, "diskread"), 0, ioMax(diskRead), styleSpiky)
			diskWriteSeries = GenerateSeededSeries(diskWrite, numPoints, HashSeed(storeType, storeID, "diskwrite"), 0, ioMax(diskWrite), styleSpiky)
		}
		if includeNetwork {
			netInSeries = GenerateSeededSeries(netIn, numPoints, HashSeed(storeType, storeID, "netin"), 0, ioMax(netIn), styleSpiky)
			netOutSeries = GenerateSeededSeries(netOut, numPoints, HashSeed(storeType, storeID, "netout"), 0, ioMax(netOut), styleSpiky)
		}

		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
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

	log.Debug().Int("count", len(state.Nodes)).Msg("mock seeding: processing nodes")
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
	log.Debug().Int("total", len(state.VMs)).Int("running", runningVMs).Msg("mock seeding: processing VMs")
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
	log.Debug().Int("total", len(state.Containers)).Int("running", runningContainers).Msg("mock seeding: processing containers")
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
	log.Debug().Int("clusters", len(state.KubernetesClusters)).Int("pods", k8sPodCount).Msg("mock seeding: processing kubernetes pods")
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

	log.Debug().Int("count", len(state.Storage)).Msg("mock seeding: processing storage")
	for _, storage := range state.Storage {
		if storage.ID == "" {
			continue
		}
		numPoints := len(seedTimestamps)
		usageSeries := GenerateSeededSeries(storage.Usage, numPoints, HashSeed("storage", storage.ID, "usage"), 0, 100, styleFlat)

		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			mh.AddStorageMetric(storage.ID, "usage", usageSeries[i], ts)
			queueMetric("storage", storage.ID, "usage", usageSeries[i], ts)
		}

		// Ensure the latest point lands at "now" for full-range charts.
		mh.AddStorageMetric(storage.ID, "usage", storage.Usage, now)
		queueMetric("storage", storage.ID, "usage", storage.Usage, now)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	log.Debug().Int("count", len(state.PhysicalDisks)).Msg("mock seeding: processing physical disks")
	for _, disk := range state.PhysicalDisks {
		if disk.Temperature <= 0 {
			continue
		}
		resourceID := diskMetricsResourceID(disk)
		if resourceID == "" {
			continue
		}

		numPoints := len(seedTimestamps)
		tempSeries := GenerateSeededSeries(
			float64(disk.Temperature),
			numPoints,
			HashSeed("disk", resourceID, "smart_temp"),
			25,
			95,
			styleFlat,
		)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			queueMetric("disk", resourceID, "smart_temp", tempSeries[i], ts)
		}

		// Ensure the latest point lands at "now" for full-range charts.
		queueMetric("disk", resourceID, "smart_temp", float64(disk.Temperature), now)
	}

	log.Debug().Int("count", len(state.CephClusters)).Msg("mock seeding: processing ceph clusters")
	for _, cluster := range state.CephClusters {
		if cluster.TotalBytes <= 0 {
			continue
		}
		cephID := cluster.FSID
		if cephID == "" {
			cephID = cluster.ID
		}
		if cephID == "" {
			continue
		}

		numPoints := len(seedTimestamps)
		usageSeries := GenerateSeededSeries(
			cluster.UsagePercent,
			numPoints,
			HashSeed("ceph", cephID, "usage"),
			0,
			100,
			styleFlat,
		)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			queueMetric("ceph", cephID, "usage", usageSeries[i], ts)
		}

		// Ensure the latest point lands at "now" for full-range charts.
		queueMetric("ceph", cephID, "usage", cluster.UsagePercent, now)
	}

	log.Debug().Int("count", len(state.DockerHosts)).Msg("mock seeding: processing docker hosts")
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
			recordGuest("docker:"+container.ID, "dockerContainer", container.ID, container.CPUPercent, container.MemoryPercent, containerDisk, 0, 0, 0, 0, true, false, false)
		}
		time.Sleep(50 * time.Millisecond) // Add delay for docker hosts
	}

	log.Debug().Int("count", len(state.Hosts)).Msg("mock seeding: processing host agents")
	for _, host := range state.Hosts {
		if host.ID == "" || host.Status != "online" {
			continue
		}

		var diskPercent float64
		if len(host.Disks) > 0 {
			diskPercent = host.Disks[0].Usage
		}

		recordGuest("host:"+host.ID, "host", host.ID, host.CPUUsage, host.Memory.Usage, diskPercent, host.DiskReadRate, host.DiskWriteRate, host.NetInRate, host.NetOutRate, true, true, true)
		time.Sleep(50 * time.Millisecond)
	}

	// Seed TrueNAS pool/dataset disk-usage metrics
	fixtures := truenas.DefaultFixtures()
	log.Debug().Int("pools", len(fixtures.Pools)).Int("datasets", len(fixtures.Datasets)).Msg("mock seeding: processing TrueNAS fixtures")

	// System host: aggregated disk usage across all pools
	totalCap, totalUsed := int64(0), int64(0)
	for _, pool := range fixtures.Pools {
		totalCap += pool.TotalBytes
		totalUsed += pool.UsedBytes
	}
	systemDisk := float64(0)
	if totalCap > 0 {
		systemDisk = float64(totalUsed) / float64(totalCap) * 100
	}
	systemKey := "system:" + fixtures.System.Hostname
	recordGuest(systemKey, "truenas", fixtures.System.Hostname, 0, 0, systemDisk, 0, 0, 0, 0, true, false, false)

	for _, pool := range fixtures.Pools {
		poolKey := "pool:" + pool.Name
		diskPercent := float64(0)
		if pool.TotalBytes > 0 {
			diskPercent = float64(pool.UsedBytes) / float64(pool.TotalBytes) * 100
		}
		numPoints := len(seedTimestamps)
		diskSeries := GenerateSeededSeries(diskPercent, numPoints, HashSeed("pool", pool.Name, "disk"), 0, 100, styleFlat)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			mh.AddGuestMetric(poolKey, "disk", diskSeries[i], ts)
			queueMetric("pool", pool.Name, "disk", diskSeries[i], ts)
		}
		mh.AddGuestMetric(poolKey, "disk", diskPercent, now)
		queueMetric("pool", pool.Name, "disk", diskPercent, now)
	}

	for _, dataset := range fixtures.Datasets {
		dsKey := "dataset:" + dataset.Name
		totalBytes := dataset.UsedBytes + dataset.AvailBytes
		diskPercent := float64(0)
		if totalBytes > 0 {
			diskPercent = float64(dataset.UsedBytes) / float64(totalBytes) * 100
		}
		numPoints := len(seedTimestamps)
		diskSeries := GenerateSeededSeries(diskPercent, numPoints, HashSeed("dataset", dataset.Name, "disk"), 0, 100, styleFlat)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			mh.AddGuestMetric(dsKey, "disk", diskSeries[i], ts)
			queueMetric("dataset", dataset.Name, "disk", diskSeries[i], ts)
		}
		mh.AddGuestMetric(dsKey, "disk", diskPercent, now)
		queueMetric("dataset", dataset.Name, "disk", diskPercent, now)
	}

	if ms != nil && len(seedBatch) > 0 {
		ms.WriteBatchSync(seedBatch)
	}
	log.Debug().Msg("mock seeding: completed")
}

// recordTrueNASFixturesMetrics records disk usage metrics for TrueNAS pools and datasets.
func recordTrueNASFixturesMetrics(mh *MetricsHistory, ms *metrics.Store, ts time.Time) {
	fixtures := truenas.DefaultFixtures()

	totalCap, totalUsed := int64(0), int64(0)
	for _, pool := range fixtures.Pools {
		totalCap += pool.TotalBytes
		totalUsed += pool.UsedBytes
	}
	if totalCap > 0 {
		systemKey := "system:" + fixtures.System.Hostname
		systemDisk := float64(totalUsed) / float64(totalCap) * 100
		mh.AddGuestMetric(systemKey, "disk", systemDisk, ts)
		if ms != nil {
			ms.Write("truenas", fixtures.System.Hostname, "disk", systemDisk, ts)
		}
	}

	for _, pool := range fixtures.Pools {
		if pool.TotalBytes > 0 {
			poolKey := "pool:" + pool.Name
			diskPct := float64(pool.UsedBytes) / float64(pool.TotalBytes) * 100
			mh.AddGuestMetric(poolKey, "disk", diskPct, ts)
			if ms != nil {
				ms.Write("pool", pool.Name, "disk", diskPct, ts)
			}
		}
	}

	for _, dataset := range fixtures.Datasets {
		totalBytes := dataset.UsedBytes + dataset.AvailBytes
		if totalBytes > 0 {
			dsKey := "dataset:" + dataset.Name
			diskPct := float64(dataset.UsedBytes) / float64(totalBytes) * 100
			mh.AddGuestMetric(dsKey, "disk", diskPct, ts)
			if ms != nil {
				ms.Write("dataset", dataset.Name, "disk", diskPct, ts)
			}
		}
	}
}

type guestMetricSource interface {
	GetID() string
	GetStatus() string
	GetCPU() float64
	GetMemory() models.Memory
	GetDisk() models.Disk
	GetDiskRead() int64
	GetDiskWrite() int64
	GetNetworkIn() int64
	GetNetworkOut() int64
}

type vmAdapter struct{ *models.VM }

func (v vmAdapter) GetID() string            { return v.VM.ID }
func (v vmAdapter) GetStatus() string        { return v.VM.Status }
func (v vmAdapter) GetCPU() float64          { return v.VM.CPU }
func (v vmAdapter) GetMemory() models.Memory { return v.VM.Memory }
func (v vmAdapter) GetDisk() models.Disk     { return v.VM.Disk }
func (v vmAdapter) GetDiskRead() int64       { return v.VM.DiskRead }
func (v vmAdapter) GetDiskWrite() int64      { return v.VM.DiskWrite }
func (v vmAdapter) GetNetworkIn() int64      { return v.VM.NetworkIn }
func (v vmAdapter) GetNetworkOut() int64     { return v.VM.NetworkOut }

type containerAdapter struct{ *models.Container }

func (c containerAdapter) GetID() string            { return c.Container.ID }
func (c containerAdapter) GetStatus() string        { return c.Container.Status }
func (c containerAdapter) GetCPU() float64          { return c.Container.CPU }
func (c containerAdapter) GetMemory() models.Memory { return c.Container.Memory }
func (c containerAdapter) GetDisk() models.Disk     { return c.Container.Disk }
func (c containerAdapter) GetDiskRead() int64       { return c.Container.DiskRead }
func (c containerAdapter) GetDiskWrite() int64      { return c.Container.DiskWrite }
func (c containerAdapter) GetNetworkIn() int64      { return c.Container.NetworkIn }
func (c containerAdapter) GetNetworkOut() int64     { return c.Container.NetworkOut }

func recordGuestMetrics[T guestMetricSource](mh *MetricsHistory, ms *metrics.Store, guests []T, prefix string, ts time.Time) {
	for _, guest := range guests {
		if guest.GetID() == "" || guest.GetStatus() != "running" {
			continue
		}

		id := guest.GetID()
		cpu := guest.GetCPU() * 100
		memory := guest.GetMemory().Usage
		disk := guest.GetDisk().Usage
		diskread := float64(guest.GetDiskRead())
		diskwrite := float64(guest.GetDiskWrite())
		netin := float64(guest.GetNetworkIn())
		netout := float64(guest.GetNetworkOut())

		mh.AddGuestMetric(id, "cpu", cpu, ts)
		mh.AddGuestMetric(id, "memory", memory, ts)
		mh.AddGuestMetric(id, "disk", disk, ts)
		mh.AddGuestMetric(id, "diskread", diskread, ts)
		mh.AddGuestMetric(id, "diskwrite", diskwrite, ts)
		mh.AddGuestMetric(id, "netin", netin, ts)
		mh.AddGuestMetric(id, "netout", netout, ts)

		if ms != nil {
			ms.Write(prefix, id, "cpu", cpu, ts)
			ms.Write(prefix, id, "memory", memory, ts)
			ms.Write(prefix, id, "disk", disk, ts)
			ms.Write(prefix, id, "diskread", diskread, ts)
			ms.Write(prefix, id, "diskwrite", diskwrite, ts)
			ms.Write(prefix, id, "netin", netin, ts)
			ms.Write(prefix, id, "netout", netout, ts)
		}
	}
}

func adaptVMs(vms []models.VM) []vmAdapter {
	result := make([]vmAdapter, len(vms))
	for i := range vms {
		result[i] = vmAdapter{&vms[i]}
	}
	return result
}

func adaptContainers(cts []models.Container) []containerAdapter {
	result := make([]containerAdapter, len(cts))
	for i := range cts {
		result[i] = containerAdapter{&cts[i]}
	}
	return result
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

	recordGuestMetrics(mh, ms, adaptVMs(state.VMs), "vm", ts)
	recordGuestMetrics(mh, ms, adaptContainers(state.Containers), "container", ts)

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

	for _, cluster := range state.CephClusters {
		if cluster.TotalBytes <= 0 {
			continue
		}
		cephID := cluster.FSID
		if cephID == "" {
			cephID = cluster.ID
		}
		if cephID == "" {
			continue
		}

		if ms != nil {
			ms.Write("ceph", cephID, "usage", cluster.UsagePercent, ts)
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
				ms.Write("dockerContainer", container.ID, "cpu", container.CPUPercent, ts)
				ms.Write("dockerContainer", container.ID, "memory", container.MemoryPercent, ts)
				ms.Write("dockerContainer", container.ID, "disk", containerDisk, ts)
			}
		}
	}

	for _, host := range state.Hosts {
		if host.ID == "" || host.Status != "online" {
			continue
		}

		var diskPercent float64
		if len(host.Disks) > 0 {
			diskPercent = host.Disks[0].Usage
		}

		hostKey := "host:" + host.ID
		mh.AddGuestMetric(hostKey, "cpu", host.CPUUsage, ts)
		mh.AddGuestMetric(hostKey, "memory", host.Memory.Usage, ts)
		mh.AddGuestMetric(hostKey, "disk", diskPercent, ts)
		mh.AddGuestMetric(hostKey, "diskread", host.DiskReadRate, ts)
		mh.AddGuestMetric(hostKey, "diskwrite", host.DiskWriteRate, ts)
		mh.AddGuestMetric(hostKey, "netin", host.NetInRate, ts)
		mh.AddGuestMetric(hostKey, "netout", host.NetOutRate, ts)

		if ms != nil {
			ms.Write("host", host.ID, "cpu", host.CPUUsage, ts)
			ms.Write("host", host.ID, "memory", host.Memory.Usage, ts)
			ms.Write("host", host.ID, "disk", diskPercent, ts)
			ms.Write("host", host.ID, "diskread", host.DiskReadRate, ts)
			ms.Write("host", host.ID, "diskwrite", host.DiskWriteRate, ts)
			ms.Write("host", host.ID, "netin", host.NetInRate, ts)
			ms.Write("host", host.ID, "netout", host.NetOutRate, ts)
		}
	}

	// Record TrueNAS pool/dataset disk-usage live ticks
	recordTrueNASFixturesMetrics(mh, ms, ts)
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
		log.Debug().Msg("mock metrics sampler: nil context or monitor")
		return
	}
	if !mock.IsMockEnabled() {
		log.Debug().Msg("mock metrics sampler: mock mode not enabled")
		return
	}

	log.Info().Msg("mock metrics sampler: starting initialization")

	cfg := mockMetricsSamplerConfigFromEnv()
	seedDuration := cfg.SeedDuration
	// Reduced minimum from 7 days to 1 hour for faster startup on resource-constrained systems
	if seedDuration < time.Hour {
		seedDuration = time.Hour
	}
	// Tiered seeding produces ~2820 points per metric; allow headroom for
	// live updates on top of that.
	maxPoints := 3500

	m.mu.Lock()
	if m.mockMetricsCancel != nil {
		m.mu.Unlock()
		log.Debug().Msg("mock metrics sampler: already running")
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
		log.Info().Msg("mock metrics history sampler stopped")
	}
}
