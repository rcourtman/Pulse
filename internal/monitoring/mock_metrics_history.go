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
	"github.com/rcourtman/pulse-go-rewrite/internal/mockmodel"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/vmware"
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

func GenerateSeededSeriesForTimestamps(
	current float64,
	timestamps []time.Time,
	seed uint64,
	min float64,
	max float64,
	style SeriesStyle,
) []float64 {
	var mappedStyle mockmodel.SeriesStyle
	switch style {
	case stylePlateau:
		mappedStyle = mockmodel.StylePlateau
	case styleFlat:
		mappedStyle = mockmodel.StyleFlat
	default:
		mappedStyle = mockmodel.StyleSpiky
	}
	return mockmodel.SeriesForTimestamps(current, timestamps, seed, min, max, mappedStyle)
}

func GenerateSeededMetricSeriesForTimestamps(
	current float64,
	timestamps []time.Time,
	seed uint64,
	min float64,
	max float64,
	metricType string,
	style SeriesStyle,
) []float64 {
	return GenerateSeededMetricSeriesForTimestampsWithRole(current, timestamps, seed, min, max, metricType, style, "")
}

func GenerateSeededResourceMetricSeriesForTimestamps(
	current float64,
	timestamps []time.Time,
	resourceType string,
	resourceID string,
	metricType string,
	style SeriesStyle,
) []float64 {
	min, max := mock.MetricBoundsForResource(resourceType, resourceID, metricType)
	seed := mock.MetricSeed(resourceType, resourceID, metricType)
	role := mock.MetricRole(resourceType, resourceID)
	return GenerateSeededMetricSeriesForTimestampsWithRole(
		current,
		timestamps,
		seed,
		min,
		max,
		metricType,
		style,
		role,
	)
}

func GenerateSeededMetricSeriesForTimestampsWithRole(
	current float64,
	timestamps []time.Time,
	seed uint64,
	min float64,
	max float64,
	metricType string,
	style SeriesStyle,
	role string,
) []float64 {
	var mappedStyle mockmodel.SeriesStyle
	switch style {
	case stylePlateau:
		mappedStyle = mockmodel.StylePlateau
	case styleFlat:
		mappedStyle = mockmodel.StyleFlat
	default:
		mappedStyle = mockmodel.StyleSpiky
	}
	return mockmodel.SeriesForMetricTimestampsWithRole(current, timestamps, seed, min, max, metricType, mappedStyle, role)
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
// intervals for recent data, including the canonical terminal sample at now:
//
//	Last 2h:   1min intervals  (~120 points)
//	2h–24h:    2min intervals  (~660 points)
//	24h–end:   ~65min intervals (variable)
//
// This ensures short time ranges (1h, 4h) have enough data points without
// needing an API-level fallback layer, and it keeps seeded history on the same
// timeline the live mock sampler would have produced.
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

	if len(timestamps) == 0 || !timestamps[len(timestamps)-1].Equal(now) {
		timestamps = append(timestamps, now)
	}

	return timestamps
}

func normalizeMockMetricTimestamp(at time.Time, interval time.Duration) time.Time {
	if interval <= 0 {
		interval = time.Minute
	}
	return at.UTC().Truncate(interval)
}

func canonicalMetricSeries(resourceType, resourceID, metric string, timestamps []time.Time) []float64 {
	if len(timestamps) == 0 || strings.TrimSpace(resourceID) == "" {
		return nil
	}

	values := make([]float64, len(timestamps))
	for i, ts := range timestamps {
		values[i] = mock.SampleMetric(resourceType, resourceID, metric, ts)
	}
	return values
}

func seedMockMetricsHistory(mh *MetricsHistory, ms *metrics.Store, graph mock.FixtureGraph, now time.Time, seedDuration, interval time.Duration) {
	if mh == nil {
		return
	}
	state := graph.State
	if seedDuration <= 0 || interval <= 0 {
		return
	}
	now = normalizeMockMetricTimestamp(now, interval)

	// Build one canonical timestamp list so seeded history and subsequent live
	// mock ticks sample the same runtime timeline model.
	//   Last 2h:   1min intervals  (~120 points)
	//   2h–24h:    2min intervals  (~660 points)
	//   24h–90d:   ~65min intervals (~1920 points)
	seedTimestamps := buildTieredTimestamps(now, seedDuration)
	const seedBatchSize = 5000
	numPoints := len(seedTimestamps)
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
	recordStorageTimeline := func(storageID string, currentTotal float64) {
		if strings.TrimSpace(storageID) == "" || currentTotal <= 0 || numPoints == 0 {
			return
		}

		usageSeries := canonicalMetricSeries("storage", storageID, "usage", seedTimestamps)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			usage := clampFloat(usageSeries[i], 0, 100)
			used := currentTotal * (usage / 100.0)
			avail := math.Max(0, currentTotal-used)
			mh.AddStorageMetric(storageID, "usage", usage, ts)
			mh.AddStorageMetric(storageID, "used", used, ts)
			mh.AddStorageMetric(storageID, "avail", avail, ts)
			mh.AddStorageMetric(storageID, "total", currentTotal, ts)
			queueMetric("storage", storageID, "usage", usage, ts)
			queueMetric("storage", storageID, "used", used, ts)
			queueMetric("storage", storageID, "avail", avail, ts)
			queueMetric("storage", storageID, "total", currentTotal, ts)
		}

	}

	recordNode := func(node models.Node) {
		if node.ID == "" {
			return
		}

		cpuSeries := canonicalMetricSeries("node", node.ID, "cpu", seedTimestamps)
		memSeries := canonicalMetricSeries("node", node.ID, "memory", seedTimestamps)
		diskSeries := canonicalMetricSeries("node", node.ID, "disk", seedTimestamps)

		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			mh.AddNodeMetric(node.ID, "cpu", cpuSeries[i], ts)
			mh.AddNodeMetric(node.ID, "memory", memSeries[i], ts)
			mh.AddNodeMetric(node.ID, "disk", diskSeries[i], ts)
			queueMetric("node", node.ID, "cpu", cpuSeries[i], ts)
			queueMetric("node", node.ID, "memory", memSeries[i], ts)
			queueMetric("node", node.ID, "disk", diskSeries[i], ts)
		}

	}

	recordGuest := func(
		metricIDs []string, storeType, storeID string,
		includeDisk bool,
		includeDiskIO bool,
		includeNetwork bool,
	) {
		if len(metricIDs) == 0 || storeID == "" {
			return
		}
		uniqueMetricIDs := make([]string, 0, len(metricIDs))
		seenMetricIDs := make(map[string]struct{}, len(metricIDs))
		for _, rawID := range metricIDs {
			metricID := strings.TrimSpace(rawID)
			if metricID == "" {
				continue
			}
			if _, exists := seenMetricIDs[metricID]; exists {
				continue
			}
			seenMetricIDs[metricID] = struct{}{}
			uniqueMetricIDs = append(uniqueMetricIDs, metricID)
		}
		if len(uniqueMetricIDs) == 0 {
			return
		}

		cpuSeries := canonicalMetricSeries(storeType, storeID, "cpu", seedTimestamps)
		memSeries := canonicalMetricSeries(storeType, storeID, "memory", seedTimestamps)
		var diskSeries []float64
		if includeDisk {
			diskSeries = canonicalMetricSeries(storeType, storeID, "disk", seedTimestamps)
		}
		var diskReadSeries, diskWriteSeries, netInSeries, netOutSeries []float64
		if includeDiskIO {
			diskReadSeries = canonicalMetricSeries(storeType, storeID, "diskread", seedTimestamps)
			diskWriteSeries = canonicalMetricSeries(storeType, storeID, "diskwrite", seedTimestamps)
		}
		if includeNetwork {
			netInSeries = canonicalMetricSeries(storeType, storeID, "netin", seedTimestamps)
			netOutSeries = canonicalMetricSeries(storeType, storeID, "netout", seedTimestamps)
		}

		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			for _, metricID := range uniqueMetricIDs {
				mh.AddGuestMetric(metricID, "cpu", cpuSeries[i], ts)
				mh.AddGuestMetric(metricID, "memory", memSeries[i], ts)
			}
			queueMetric(storeType, storeID, "cpu", cpuSeries[i], ts)
			queueMetric(storeType, storeID, "memory", memSeries[i], ts)
			if includeDisk {
				for _, metricID := range uniqueMetricIDs {
					mh.AddGuestMetric(metricID, "disk", diskSeries[i], ts)
				}
				queueMetric(storeType, storeID, "disk", diskSeries[i], ts)
			}
			if includeDiskIO {
				for _, metricID := range uniqueMetricIDs {
					mh.AddGuestMetric(metricID, "diskread", diskReadSeries[i], ts)
					mh.AddGuestMetric(metricID, "diskwrite", diskWriteSeries[i], ts)
				}
				queueMetric(storeType, storeID, "diskread", diskReadSeries[i], ts)
				queueMetric(storeType, storeID, "diskwrite", diskWriteSeries[i], ts)
			}
			if includeNetwork {
				for _, metricID := range uniqueMetricIDs {
					mh.AddGuestMetric(metricID, "netin", netInSeries[i], ts)
					mh.AddGuestMetric(metricID, "netout", netOutSeries[i], ts)
				}
				queueMetric(storeType, storeID, "netin", netInSeries[i], ts)
				queueMetric(storeType, storeID, "netout", netOutSeries[i], ts)
			}
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
	log.Debug().Int("total", len(state.VMs)).Int("running", runningVMs).Msg("mock seeding: processing VMs (including stopped)")
	for _, vm := range state.VMs {
		recordGuest([]string{vm.ID}, "vm", vm.ID, true, true, true)
		time.Sleep(50 * time.Millisecond) // Reduced from 200ms for faster startup
	}

	runningContainers := 0
	for _, ct := range state.Containers {
		if ct.Status == "running" {
			runningContainers++
		}
	}
	log.Debug().Int("total", len(state.Containers)).Int("running", runningContainers).Msg("mock seeding: processing containers (including stopped)")
	for _, ct := range state.Containers {
		recordGuest([]string{ct.ID}, "container", ct.ID, true, true, true)
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
			recordGuest(
				[]string{metricID},
				"k8s",
				metricID,
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
		_, _, total, _ := normalizedStorageCapacityMetrics(storage.Total, storage.Used, storage.Free, storage.Usage)
		recordStorageTimeline(storage.ID, total)
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

		tempSeries := canonicalMetricSeries("disk", resourceID, "smart_temp", seedTimestamps)
		busySeries := canonicalMetricSeries("disk", resourceID, "disk", seedTimestamps)
		diskReadSeries := canonicalMetricSeries("disk", resourceID, "diskread", seedTimestamps)
		diskWriteSeries := canonicalMetricSeries("disk", resourceID, "diskwrite", seedTimestamps)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			mh.AddDiskMetric(resourceID, "smart_temp", tempSeries[i], ts)
			mh.AddDiskMetric(resourceID, "disk", busySeries[i], ts)
			mh.AddDiskMetric(resourceID, "diskread", diskReadSeries[i], ts)
			mh.AddDiskMetric(resourceID, "diskwrite", diskWriteSeries[i], ts)
			queueMetric("disk", resourceID, "smart_temp", tempSeries[i], ts)
			queueMetric("disk", resourceID, "disk", busySeries[i], ts)
			queueMetric("disk", resourceID, "diskread", diskReadSeries[i], ts)
			queueMetric("disk", resourceID, "diskwrite", diskWriteSeries[i], ts)
		}

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

		usageSeries := canonicalMetricSeries("ceph", cephID, "usage", seedTimestamps)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			queueMetric("ceph", cephID, "usage", usageSeries[i], ts)
		}

	}

	log.Debug().Int("count", len(state.DockerHosts)).Msg("mock seeding: processing docker hosts")
	for _, host := range state.DockerHosts {
		if host.ID == "" {
			continue
		}

		recordGuest(
			[]string{"dockerHost:" + host.ID},
			"dockerHost",
			host.ID,
			true,
			false,
			false,
		)

		for _, container := range host.Containers {
			if container.ID == "" {
				continue
			}
			recordGuest(
				[]string{"docker:" + container.ID},
				"dockerContainer",
				container.ID,
				true,
				false,
				false,
			)
		}
		time.Sleep(50 * time.Millisecond) // Add delay for docker hosts
	}

	log.Debug().Int("count", len(state.Hosts)).Msg("mock seeding: processing host agents")
	for _, host := range state.Hosts {
		if host.ID == "" || host.Status != "online" {
			continue
		}

		recordGuest(
			[]string{"agent:" + host.ID},
			"agent",
			host.ID,
			true,
			true,
			true,
		)
		time.Sleep(50 * time.Millisecond)
	}

	platformFixtures := graph.PlatformFixtures
	trueNASFixtures := platformFixtures.TrueNAS
	log.Debug().Int("pools", len(trueNASFixtures.Pools)).Int("datasets", len(trueNASFixtures.Datasets)).Msg("mock seeding: processing TrueNAS fixtures")

	// System host: aggregated disk usage across all pools
	systemMetricIDs := []string{
		"system:" + trueNASFixtures.System.Hostname,
		"agent:" + trueNASFixtures.System.Hostname,
	}
	recordGuest(
		systemMetricIDs,
		"agent",
		trueNASFixtures.System.Hostname,
		true,
		true,
		true,
	)

	for _, pool := range trueNASFixtures.Pools {
		poolKey := "pool:" + pool.Name
		diskSeries := canonicalMetricSeries("storage", poolKey, "usage", seedTimestamps)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			mh.AddGuestMetric(poolKey, "disk", diskSeries[i], ts)
		}
		recordStorageTimeline(poolKey, float64(pool.TotalBytes))
	}

	for _, dataset := range trueNASFixtures.Datasets {
		dsKey := "dataset:" + dataset.Name
		totalBytes := dataset.UsedBytes + dataset.AvailBytes
		diskSeries := canonicalMetricSeries("storage", dsKey, "usage", seedTimestamps)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			mh.AddGuestMetric(dsKey, "disk", diskSeries[i], ts)
		}
		recordStorageTimeline(dsKey, float64(totalBytes))
	}

	for _, disk := range trueNASFixtures.Disks {
		if disk.Temperature <= 0 {
			continue
		}
		resourceID := trueNASDiskMetricsResourceID(disk)
		if resourceID == "" {
			continue
		}

		tempSeries := canonicalMetricSeries("disk", resourceID, "smart_temp", seedTimestamps)
		busySeries := canonicalMetricSeries("disk", resourceID, "disk", seedTimestamps)
		diskReadSeries := canonicalMetricSeries("disk", resourceID, "diskread", seedTimestamps)
		diskWriteSeries := canonicalMetricSeries("disk", resourceID, "diskwrite", seedTimestamps)
		for i := 0; i < numPoints; i++ {
			ts := seedTimestamps[i]
			mh.AddDiskMetric(resourceID, "smart_temp", tempSeries[i], ts)
			mh.AddDiskMetric(resourceID, "disk", busySeries[i], ts)
			mh.AddDiskMetric(resourceID, "diskread", diskReadSeries[i], ts)
			mh.AddDiskMetric(resourceID, "diskwrite", diskWriteSeries[i], ts)
			queueMetric("disk", resourceID, "smart_temp", tempSeries[i], ts)
			queueMetric("disk", resourceID, "disk", busySeries[i], ts)
			queueMetric("disk", resourceID, "diskread", diskReadSeries[i], ts)
			queueMetric("disk", resourceID, "diskwrite", diskWriteSeries[i], ts)
		}
	}

	for _, app := range trueNASFixtures.Apps {
		appID := strings.TrimSpace(app.ID)
		if appID == "" {
			appID = strings.TrimSpace(app.Name)
		}
		if appID == "" {
			continue
		}
		recordGuest(
			[]string{"docker:" + appID},
			"dockerContainer",
			appID,
			true,
			true,
			true,
		)
	}

	vmwareFixtures := platformFixtures.VMware
	log.Debug().
		Int("hosts", len(vmwareFixtures.Hosts)).
		Int("vms", len(vmwareFixtures.VMs)).
		Int("datastores", len(vmwareFixtures.Datastores)).
		Msg("mock seeding: processing VMware fixtures")

	for _, host := range vmwareFixtures.Hosts {
		sourceID := vmware.SourceID(vmwareFixtures.ConnectionID, "host", host.Host)
		if sourceID == "" {
			continue
		}
		recordGuest(
			[]string{"agent:" + sourceID},
			"agent",
			sourceID,
			true,
			true,
			true,
		)
	}

	for _, guest := range vmwareFixtures.VMs {
		sourceID := vmware.SourceID(vmwareFixtures.ConnectionID, "vm", guest.VM)
		if sourceID == "" {
			continue
		}
		recordGuest(
			[]string{sourceID},
			"vm",
			sourceID,
			true,
			true,
			true,
		)
	}

	for _, datastore := range vmwareFixtures.Datastores {
		total := datastore.Capacity
		if total <= 0 {
			continue
		}
		sourceID := vmware.SourceID(vmwareFixtures.ConnectionID, "datastore", datastore.Datastore)
		if sourceID == "" {
			continue
		}
		recordStorageTimeline(sourceID, float64(total))
	}

	if ms != nil && len(seedBatch) > 0 {
		ms.WriteBatchSync(seedBatch)
	}
	log.Debug().Msg("mock seeding: completed")
}

// recordTrueNASFixturesMetrics records live fixture ticks for TrueNAS host,
// storage, and app-container metrics.
func recordTrueNASFixturesMetrics(mh *MetricsHistory, ms *metrics.Store, fixtures mock.PlatformFixtures, ts time.Time) {
	snapshot := fixtures.TrueNAS

	if strings.TrimSpace(snapshot.System.Hostname) != "" {
		systemCPU := mock.SampleMetric("agent", snapshot.System.Hostname, "cpu", ts)
		systemMemory := mock.SampleMetric("agent", snapshot.System.Hostname, "memory", ts)
		systemDisk := mock.SampleMetric("agent", snapshot.System.Hostname, "disk", ts)
		systemDiskRead := mock.SampleMetric("agent", snapshot.System.Hostname, "diskread", ts)
		systemDiskWrite := mock.SampleMetric("agent", snapshot.System.Hostname, "diskwrite", ts)
		systemNetIn := mock.SampleMetric("agent", snapshot.System.Hostname, "netin", ts)
		systemNetOut := mock.SampleMetric("agent", snapshot.System.Hostname, "netout", ts)
		systemMetricIDs := []string{
			"system:" + snapshot.System.Hostname,
			"agent:" + snapshot.System.Hostname,
		}
		for _, metricID := range systemMetricIDs {
			mh.AddGuestMetric(metricID, "cpu", systemCPU, ts)
			mh.AddGuestMetric(metricID, "memory", systemMemory, ts)
			mh.AddGuestMetric(metricID, "disk", systemDisk, ts)
			mh.AddGuestMetric(metricID, "diskread", systemDiskRead, ts)
			mh.AddGuestMetric(metricID, "diskwrite", systemDiskWrite, ts)
			mh.AddGuestMetric(metricID, "netin", systemNetIn, ts)
			mh.AddGuestMetric(metricID, "netout", systemNetOut, ts)
		}
		if ms != nil {
			ms.Write("agent", snapshot.System.Hostname, "cpu", systemCPU, ts)
			ms.Write("agent", snapshot.System.Hostname, "memory", systemMemory, ts)
			ms.Write("agent", snapshot.System.Hostname, "disk", systemDisk, ts)
			ms.Write("agent", snapshot.System.Hostname, "diskread", systemDiskRead, ts)
			ms.Write("agent", snapshot.System.Hostname, "diskwrite", systemDiskWrite, ts)
			ms.Write("agent", snapshot.System.Hostname, "netin", systemNetIn, ts)
			ms.Write("agent", snapshot.System.Hostname, "netout", systemNetOut, ts)
		}
	}

	for _, pool := range snapshot.Pools {
		if pool.TotalBytes > 0 {
			poolKey := "pool:" + pool.Name
			total := float64(pool.TotalBytes)
			usage := clampFloat(mock.SampleMetric("storage", poolKey, "usage", ts), 0, 100)
			used := total * (usage / 100.0)
			avail := math.Max(0, total-used)
			mh.AddGuestMetric(poolKey, "disk", usage, ts)
			mh.AddStorageMetric(poolKey, "usage", usage, ts)
			mh.AddStorageMetric(poolKey, "used", used, ts)
			mh.AddStorageMetric(poolKey, "avail", avail, ts)
			mh.AddStorageMetric(poolKey, "total", total, ts)
			if ms != nil {
				ms.Write("pool", pool.Name, "disk", usage, ts)
				ms.Write("storage", poolKey, "usage", usage, ts)
				ms.Write("storage", poolKey, "used", used, ts)
				ms.Write("storage", poolKey, "avail", avail, ts)
				ms.Write("storage", poolKey, "total", total, ts)
			}
		}
	}

	for _, dataset := range snapshot.Datasets {
		totalBytes := dataset.UsedBytes + dataset.AvailBytes
		if totalBytes > 0 {
			dsKey := "dataset:" + dataset.Name
			total := float64(totalBytes)
			usage := clampFloat(mock.SampleMetric("storage", dsKey, "usage", ts), 0, 100)
			used := total * (usage / 100.0)
			avail := math.Max(0, total-used)
			mh.AddGuestMetric(dsKey, "disk", usage, ts)
			mh.AddStorageMetric(dsKey, "usage", usage, ts)
			mh.AddStorageMetric(dsKey, "used", used, ts)
			mh.AddStorageMetric(dsKey, "avail", avail, ts)
			mh.AddStorageMetric(dsKey, "total", total, ts)
			if ms != nil {
				ms.Write("dataset", dataset.Name, "disk", usage, ts)
				ms.Write("storage", dsKey, "usage", usage, ts)
				ms.Write("storage", dsKey, "used", used, ts)
				ms.Write("storage", dsKey, "avail", avail, ts)
				ms.Write("storage", dsKey, "total", total, ts)
			}
		}
	}

	for _, disk := range snapshot.Disks {
		if disk.Temperature <= 0 {
			continue
		}
		resourceID := trueNASDiskMetricsResourceID(disk)
		if resourceID == "" {
			continue
		}
		temp := mock.SampleMetric("disk", resourceID, "smart_temp", ts)
		busy := mock.SampleMetric("disk", resourceID, "disk", ts)
		diskRead := mock.SampleMetric("disk", resourceID, "diskread", ts)
		diskWrite := mock.SampleMetric("disk", resourceID, "diskwrite", ts)

		mh.AddDiskMetric(resourceID, "smart_temp", temp, ts)
		mh.AddDiskMetric(resourceID, "disk", busy, ts)
		mh.AddDiskMetric(resourceID, "diskread", diskRead, ts)
		mh.AddDiskMetric(resourceID, "diskwrite", diskWrite, ts)
		if ms != nil {
			ms.Write("disk", resourceID, "smart_temp", temp, ts)
			ms.Write("disk", resourceID, "disk", busy, ts)
			ms.Write("disk", resourceID, "diskread", diskRead, ts)
			ms.Write("disk", resourceID, "diskwrite", diskWrite, ts)
		}
	}

	for _, app := range snapshot.Apps {
		appID := strings.TrimSpace(app.ID)
		if appID == "" {
			appID = strings.TrimSpace(app.Name)
		}
		if appID == "" {
			continue
		}
		cpu := mock.SampleMetric("dockerContainer", appID, "cpu", ts)
		memPercent := mock.SampleMetric("dockerContainer", appID, "memory", ts)
		diskPercent := mock.SampleMetric("dockerContainer", appID, "disk", ts)
		diskRead := mock.SampleMetric("dockerContainer", appID, "diskread", ts)
		diskWrite := mock.SampleMetric("dockerContainer", appID, "diskwrite", ts)
		netIn := mock.SampleMetric("dockerContainer", appID, "netin", ts)
		netOut := mock.SampleMetric("dockerContainer", appID, "netout", ts)
		metricKey := "docker:" + appID
		mh.AddGuestMetric(metricKey, "cpu", cpu, ts)
		mh.AddGuestMetric(metricKey, "memory", memPercent, ts)
		mh.AddGuestMetric(metricKey, "disk", diskPercent, ts)
		mh.AddGuestMetric(metricKey, "diskread", diskRead, ts)
		mh.AddGuestMetric(metricKey, "diskwrite", diskWrite, ts)
		mh.AddGuestMetric(metricKey, "netin", netIn, ts)
		mh.AddGuestMetric(metricKey, "netout", netOut, ts)
		if ms != nil {
			ms.Write("dockerContainer", appID, "cpu", cpu, ts)
			ms.Write("dockerContainer", appID, "memory", memPercent, ts)
			ms.Write("dockerContainer", appID, "disk", diskPercent, ts)
			ms.Write("dockerContainer", appID, "diskread", diskRead, ts)
			ms.Write("dockerContainer", appID, "diskwrite", diskWrite, ts)
			ms.Write("dockerContainer", appID, "netin", netIn, ts)
			ms.Write("dockerContainer", appID, "netout", netOut, ts)
		}
	}
}

func recordVMwareFixturesMetrics(mh *MetricsHistory, ms *metrics.Store, fixtures mock.PlatformFixtures, ts time.Time) {
	snapshot := fixtures.VMware

	for _, host := range snapshot.Hosts {
		sourceID := vmware.SourceID(snapshot.ConnectionID, "host", host.Host)
		if sourceID == "" {
			continue
		}
		cpu := mock.SampleMetric("agent", sourceID, "cpu", ts)
		memory := mock.SampleMetric("agent", sourceID, "memory", ts)
		disk := mock.SampleMetric("agent", sourceID, "disk", ts)
		diskRead := mock.SampleMetric("agent", sourceID, "diskread", ts)
		diskWrite := mock.SampleMetric("agent", sourceID, "diskwrite", ts)
		netIn := mock.SampleMetric("agent", sourceID, "netin", ts)
		netOut := mock.SampleMetric("agent", sourceID, "netout", ts)
		mh.AddGuestMetric("agent:"+sourceID, "cpu", cpu, ts)
		mh.AddGuestMetric("agent:"+sourceID, "memory", memory, ts)
		mh.AddGuestMetric("agent:"+sourceID, "disk", disk, ts)
		mh.AddGuestMetric("agent:"+sourceID, "diskread", diskRead, ts)
		mh.AddGuestMetric("agent:"+sourceID, "diskwrite", diskWrite, ts)
		mh.AddGuestMetric("agent:"+sourceID, "netin", netIn, ts)
		mh.AddGuestMetric("agent:"+sourceID, "netout", netOut, ts)
		if ms != nil {
			ms.Write("agent", sourceID, "cpu", cpu, ts)
			ms.Write("agent", sourceID, "memory", memory, ts)
			ms.Write("agent", sourceID, "disk", disk, ts)
			ms.Write("agent", sourceID, "diskread", diskRead, ts)
			ms.Write("agent", sourceID, "diskwrite", diskWrite, ts)
			ms.Write("agent", sourceID, "netin", netIn, ts)
			ms.Write("agent", sourceID, "netout", netOut, ts)
		}
	}

	for _, guest := range snapshot.VMs {
		sourceID := vmware.SourceID(snapshot.ConnectionID, "vm", guest.VM)
		if sourceID == "" {
			continue
		}
		cpu := mock.SampleMetric("vm", sourceID, "cpu", ts)
		memory := mock.SampleMetric("vm", sourceID, "memory", ts)
		disk := mock.SampleMetric("vm", sourceID, "disk", ts)
		diskRead := mock.SampleMetric("vm", sourceID, "diskread", ts)
		diskWrite := mock.SampleMetric("vm", sourceID, "diskwrite", ts)
		netIn := mock.SampleMetric("vm", sourceID, "netin", ts)
		netOut := mock.SampleMetric("vm", sourceID, "netout", ts)
		mh.AddGuestMetric(sourceID, "cpu", cpu, ts)
		mh.AddGuestMetric(sourceID, "memory", memory, ts)
		mh.AddGuestMetric(sourceID, "disk", disk, ts)
		mh.AddGuestMetric(sourceID, "diskread", diskRead, ts)
		mh.AddGuestMetric(sourceID, "diskwrite", diskWrite, ts)
		mh.AddGuestMetric(sourceID, "netin", netIn, ts)
		mh.AddGuestMetric(sourceID, "netout", netOut, ts)
		if ms != nil {
			ms.Write("vm", sourceID, "cpu", cpu, ts)
			ms.Write("vm", sourceID, "memory", memory, ts)
			ms.Write("vm", sourceID, "disk", disk, ts)
			ms.Write("vm", sourceID, "diskread", diskRead, ts)
			ms.Write("vm", sourceID, "diskwrite", diskWrite, ts)
			ms.Write("vm", sourceID, "netin", netIn, ts)
			ms.Write("vm", sourceID, "netout", netOut, ts)
		}
	}

	for _, datastore := range snapshot.Datastores {
		sourceID := vmware.SourceID(snapshot.ConnectionID, "datastore", datastore.Datastore)
		if sourceID == "" {
			continue
		}
		usage := clampFloat(mock.SampleMetric("storage", sourceID, "usage", ts), 0, 100)
		total := datastore.Capacity
		used := int64(float64(total) * (usage / 100.0))
		if used > total {
			used = total
		}
		avail := total - used
		if avail < 0 {
			avail = 0
		}
		mh.AddStorageMetric(sourceID, "usage", usage, ts)
		mh.AddStorageMetric(sourceID, "used", float64(used), ts)
		mh.AddStorageMetric(sourceID, "avail", float64(avail), ts)
		mh.AddStorageMetric(sourceID, "total", float64(total), ts)
		if ms != nil {
			ms.Write("storage", sourceID, "usage", usage, ts)
			ms.Write("storage", sourceID, "used", float64(used), ts)
			ms.Write("storage", sourceID, "avail", float64(avail), ts)
			ms.Write("storage", sourceID, "total", float64(total), ts)
		}
	}
}

func vmwareFloat64Metric(metrics *vmware.InventoryMetrics, pick func(*vmware.InventoryMetrics) *float64) float64 {
	if metrics == nil || pick == nil {
		return 0
	}
	value := pick(metrics)
	if value == nil {
		return 0
	}
	return *value
}

func vmwareDatastoreUsageByID(datastores []vmware.InventoryDatastore) map[string]float64 {
	out := make(map[string]float64, len(datastores))
	for _, datastore := range datastores {
		id := strings.TrimSpace(datastore.Datastore)
		if id == "" {
			continue
		}
		out[id] = vmwareDatastoreUsagePercent(datastore)
	}
	return out
}

func vmwareAverageDatastoreUsage(byID map[string]float64, ids []string) float64 {
	if len(ids) == 0 || len(byID) == 0 {
		return 0
	}
	var total float64
	var count int
	for _, id := range ids {
		usage, ok := byID[strings.TrimSpace(id)]
		if !ok {
			continue
		}
		total += usage
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func vmwareDatastoreUsagePercent(datastore vmware.InventoryDatastore) float64 {
	if datastore.Capacity <= 0 {
		return 0
	}
	used := datastore.Capacity - datastore.FreeSpace
	if used < 0 {
		used = 0
	}
	return clampFloat((float64(used)/float64(datastore.Capacity))*100, 0, 100)
}

type guestMetricSource interface {
	GetID() string
	GetStatus() string
}

type vmAdapter struct{ *models.VM }

func (v vmAdapter) GetID() string     { return v.VM.ID }
func (v vmAdapter) GetStatus() string { return v.VM.Status }

type containerAdapter struct{ *models.Container }

func (c containerAdapter) GetID() string     { return c.Container.ID }
func (c containerAdapter) GetStatus() string { return c.Container.Status }

func recordGuestMetrics[T guestMetricSource](mh *MetricsHistory, ms *metrics.Store, guests []T, prefix string, ts time.Time) {
	for _, guest := range guests {
		if guest.GetID() == "" || guest.GetStatus() != "running" {
			continue
		}

		id := guest.GetID()
		cpu := mock.SampleMetric(prefix, id, "cpu", ts)
		memory := mock.SampleMetric(prefix, id, "memory", ts)
		disk := mock.SampleMetric(prefix, id, "disk", ts)
		diskread := mock.SampleMetric(prefix, id, "diskread", ts)
		diskwrite := mock.SampleMetric(prefix, id, "diskwrite", ts)
		netin := mock.SampleMetric(prefix, id, "netin", ts)
		netout := mock.SampleMetric(prefix, id, "netout", ts)

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

func recordMockStateToMetricsHistory(mh *MetricsHistory, ms *metrics.Store, graph mock.FixtureGraph, ts time.Time) {
	if mh == nil {
		return
	}
	state := graph.State

	for _, node := range state.Nodes {
		if node.ID == "" || node.Status != "online" {
			continue
		}

		cpu := mock.SampleMetric("node", node.ID, "cpu", ts)
		memory := mock.SampleMetric("node", node.ID, "memory", ts)
		disk := mock.SampleMetric("node", node.ID, "disk", ts)
		mh.AddNodeMetric(node.ID, "cpu", cpu, ts)
		mh.AddNodeMetric(node.ID, "memory", memory, ts)
		mh.AddNodeMetric(node.ID, "disk", disk, ts)

		if ms != nil {
			ms.Write("node", node.ID, "cpu", cpu, ts)
			ms.Write("node", node.ID, "memory", memory, ts)
			ms.Write("node", node.ID, "disk", disk, ts)
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
			cpu := mock.SampleMetric("k8s", metricID, "cpu", ts)
			memory := mock.SampleMetric("k8s", metricID, "memory", ts)
			disk := mock.SampleMetric("k8s", metricID, "disk", ts)
			netIn := mock.SampleMetric("k8s", metricID, "netin", ts)
			netOut := mock.SampleMetric("k8s", metricID, "netout", ts)

			mh.AddGuestMetric(metricID, "cpu", cpu, ts)
			mh.AddGuestMetric(metricID, "memory", memory, ts)
			mh.AddGuestMetric(metricID, "disk", disk, ts)
			mh.AddGuestMetric(metricID, "netin", netIn, ts)
			mh.AddGuestMetric(metricID, "netout", netOut, ts)

			if ms != nil {
				ms.Write("k8s", metricID, "cpu", cpu, ts)
				ms.Write("k8s", metricID, "memory", memory, ts)
				ms.Write("k8s", metricID, "disk", disk, ts)
				ms.Write("k8s", metricID, "netin", netIn, ts)
				ms.Write("k8s", metricID, "netout", netOut, ts)
			}
		}
	}

	for _, storage := range state.Storage {
		if storage.ID == "" || storage.Status != "available" {
			continue
		}
		_, currentUsed, currentTotal, currentAvail := normalizedStorageCapacityMetrics(storage.Total, storage.Used, storage.Free, storage.Usage)
		total := currentTotal
		if total <= 0 {
			total = currentUsed + currentAvail
		}
		usage := mock.SampleMetric("storage", storage.ID, "usage", ts)
		usage = clampFloat(usage, 0, 100)
		used := total * (usage / 100.0)
		if used < 0 {
			used = 0
		}
		if total > 0 && used > total {
			used = total
		}
		avail := math.Max(0, total-used)
		mh.AddStorageMetric(storage.ID, "usage", usage, ts)
		mh.AddStorageMetric(storage.ID, "used", used, ts)
		mh.AddStorageMetric(storage.ID, "total", total, ts)
		mh.AddStorageMetric(storage.ID, "avail", avail, ts)

		if ms != nil {
			ms.Write("storage", storage.ID, "usage", usage, ts)
			ms.Write("storage", storage.ID, "used", used, ts)
			ms.Write("storage", storage.ID, "total", total, ts)
			ms.Write("storage", storage.ID, "avail", avail, ts)
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
		temp := mock.SampleMetric("disk", resourceID, "smart_temp", ts)
		busy := mock.SampleMetric("disk", resourceID, "disk", ts)
		diskRead := mock.SampleMetric("disk", resourceID, "diskread", ts)
		diskWrite := mock.SampleMetric("disk", resourceID, "diskwrite", ts)

		mh.AddDiskMetric(resourceID, "smart_temp", temp, ts)
		mh.AddDiskMetric(resourceID, "disk", busy, ts)
		mh.AddDiskMetric(resourceID, "diskread", diskRead, ts)
		mh.AddDiskMetric(resourceID, "diskwrite", diskWrite, ts)
		if ms != nil {
			ms.Write("disk", resourceID, "smart_temp", temp, ts)
			ms.Write("disk", resourceID, "disk", busy, ts)
			ms.Write("disk", resourceID, "diskread", diskRead, ts)
			ms.Write("disk", resourceID, "diskwrite", diskWrite, ts)
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

		hostKey := "dockerHost:" + host.ID
		cpu := mock.SampleMetric("dockerHost", host.ID, "cpu", ts)
		memory := mock.SampleMetric("dockerHost", host.ID, "memory", ts)
		disk := mock.SampleMetric("dockerHost", host.ID, "disk", ts)
		mh.AddGuestMetric(hostKey, "cpu", cpu, ts)
		mh.AddGuestMetric(hostKey, "memory", memory, ts)
		mh.AddGuestMetric(hostKey, "disk", disk, ts)

		if ms != nil {
			ms.Write("dockerHost", host.ID, "cpu", cpu, ts)
			ms.Write("dockerHost", host.ID, "memory", memory, ts)
			ms.Write("dockerHost", host.ID, "disk", disk, ts)
		}

		for _, container := range host.Containers {
			if container.ID == "" || container.State != "running" {
				continue
			}

			metricKey := "docker:" + container.ID
			cpu := mock.SampleMetric("dockerContainer", container.ID, "cpu", ts)
			memory := mock.SampleMetric("dockerContainer", container.ID, "memory", ts)
			disk := mock.SampleMetric("dockerContainer", container.ID, "disk", ts)
			mh.AddGuestMetric(metricKey, "cpu", cpu, ts)
			mh.AddGuestMetric(metricKey, "memory", memory, ts)
			mh.AddGuestMetric(metricKey, "disk", disk, ts)

			if ms != nil {
				ms.Write("dockerContainer", container.ID, "cpu", cpu, ts)
				ms.Write("dockerContainer", container.ID, "memory", memory, ts)
				ms.Write("dockerContainer", container.ID, "disk", disk, ts)
			}
		}
	}

	for _, host := range state.Hosts {
		if host.ID == "" || host.Status != "online" {
			continue
		}

		hostKey := "agent:" + host.ID
		cpu := mock.SampleMetric("agent", host.ID, "cpu", ts)
		memory := mock.SampleMetric("agent", host.ID, "memory", ts)
		disk := mock.SampleMetric("agent", host.ID, "disk", ts)
		diskread := mock.SampleMetric("agent", host.ID, "diskread", ts)
		diskwrite := mock.SampleMetric("agent", host.ID, "diskwrite", ts)
		netin := mock.SampleMetric("agent", host.ID, "netin", ts)
		netout := mock.SampleMetric("agent", host.ID, "netout", ts)
		mh.AddGuestMetric(hostKey, "cpu", cpu, ts)
		mh.AddGuestMetric(hostKey, "memory", memory, ts)
		mh.AddGuestMetric(hostKey, "disk", disk, ts)
		mh.AddGuestMetric(hostKey, "diskread", diskread, ts)
		mh.AddGuestMetric(hostKey, "diskwrite", diskwrite, ts)
		mh.AddGuestMetric(hostKey, "netin", netin, ts)
		mh.AddGuestMetric(hostKey, "netout", netout, ts)

		if ms != nil {
			ms.Write("agent", host.ID, "cpu", cpu, ts)
			ms.Write("agent", host.ID, "memory", memory, ts)
			ms.Write("agent", host.ID, "disk", disk, ts)
			ms.Write("agent", host.ID, "diskread", diskread, ts)
			ms.Write("agent", host.ID, "diskwrite", diskwrite, ts)
			ms.Write("agent", host.ID, "netin", netin, ts)
			ms.Write("agent", host.ID, "netout", netout, ts)
		}
	}

	// Record TrueNAS pool/dataset disk-usage live ticks
	recordTrueNASFixturesMetrics(mh, ms, graph.PlatformFixtures, ts)
	recordVMwareFixturesMetrics(mh, ms, graph.PlatformFixtures, ts)
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

func normalizedStorageCapacityMetrics(total, used, free int64, usage float64) (float64, float64, float64, float64) {
	totalValue := float64(total)
	usedValue := float64(used)
	freeValue := float64(free)

	if totalValue > 0 {
		if free >= 0 {
			derivedUsed := float64(total - free)
			if derivedUsed >= 0 && derivedUsed <= totalValue {
				usedValue = derivedUsed
				freeValue = totalValue - usedValue
			}
		} else {
			if usedValue < 0 {
				usedValue = 0
			}
			if usedValue > totalValue {
				usedValue = totalValue
			}
			freeValue = totalValue - usedValue
		}
		usage = (usedValue / totalValue) * 100
	}

	return clampFloat(usage, 0, 100), usedValue, totalValue, math.Max(0, freeValue)
}

func trueNASDiskMetricsResourceID(disk truenas.Disk) string {
	resourceID := strings.TrimSpace(disk.Serial)
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.ID)
	}
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.Name)
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

	graph := mock.CurrentFixtureGraph()
	state := graph.State
	log.Info().
		Int("nodes", len(state.Nodes)).
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Dur("seedDuration", seedDuration).
		Dur("sampleInterval", cfg.SampleInterval).
		Msg("Mock metrics sampler: seeding historical data")
	// Keep mock trend generation in-memory only so production history in the
	// persistent metrics store remains untouched while mock mode is active.
	seedMockMetricsHistory(m.metricsHistory, nil, graph, normalizeMockMetricTimestamp(time.Now(), cfg.SampleInterval), seedDuration, cfg.SampleInterval)
	m.invalidateMockChartCaches()
	m.prewarmMockDashboardChartCaches()

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
				recordMockStateToMetricsHistory(m.metricsHistory, nil, mock.CurrentFixtureGraph(), normalizeMockMetricTimestamp(time.Now(), cfg.SampleInterval))
				m.invalidateMockChartCaches()
				m.prewarmMockDashboardChartCaches()
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
		m.invalidateMockChartCaches()
		log.Info().Msg("mock metrics history sampler stopped")
	}
}
