package monitoring

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/mockmodel"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	// SeedStore opts the sampler into persisting mock history to the metrics
	// store. Only the dev harness sets it (alongside pointing PULSE_DATA_DIR
	// at the isolated tmp/mock-data dir), because seeded mock rows must never
	// land in a metrics.db that also holds real history.
	SeedStore bool
}

type mockMetricsSeedCacheKey struct {
	fixtureRevision uint64
	seedAtUnixNano  int64
	seedDuration    time.Duration
	sampleInterval  time.Duration
	maxDataPoints   int
}

var mockMetricsSeedCache = struct {
	sync.Mutex
	key     mockMetricsSeedCacheKey
	history *MetricsHistory
}{}

func mockMetricsSamplerConfigFromEnv() mockMetricsSamplerConfig {
	seedDuration := parseDurationEnv("PULSE_MOCK_TRENDS_SEED_DURATION", defaultMockSeedDuration)
	sampleInterval := parseDurationEnv("PULSE_MOCK_TRENDS_SAMPLE_INTERVAL", defaultMockSampleInterval)
	seedStore := parseBoolEnv("PULSE_MOCK_SEED_METRICS_STORE", false)

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
		SeedStore:      seedStore,
	}
}

// Store-tier backfill plans for mock mode. Each tier is seeded over its own
// window at its own spacing: report queries read exactly one tier (picked by
// range, falling back only when it is empty), so each tier needs enough rows
// of its own for charts without exploding total row count:
//
//	daily:  30d window, 4h spacing  -> 180 rows/series, serves >7d queries
//	hourly:  7d window, 2h spacing  ->  84 rows/series, serves 24h-7d queries
//	minute: 24h window, 15m spacing ->  96 rows/series, serves 2h-24h queries
//
// That is 360 rows/series; at the default mock estate (~750 series) the seed
// stays under ~100MB of SQLite. The raw tier is left to the live mock tick.
// Mock mode extends hourly/daily retention to 90d (see monitoring.New), so
// seeded rows outlive their windows. Timestamps sit on the spacing grid, so
// restarts target the same rows (the store upserts on timestamp+tier) and
// only the gap since the previous boot is written.
type mockStoreSeedPlan struct {
	tier    metrics.Tier
	window  time.Duration
	spacing time.Duration
}

var mockStoreSeedPlans = []mockStoreSeedPlan{
	{tier: metrics.TierDaily, window: 30 * 24 * time.Hour, spacing: 4 * time.Hour},
	{tier: metrics.TierHourly, window: 7 * 24 * time.Hour, spacing: 2 * time.Hour},
	{tier: metrics.TierMinute, window: 24 * time.Hour, spacing: 15 * time.Minute},
}

const mockStoreSeedBatchSize = 8192

// mockStoreSeedTimestamps returns the spacing-aligned timestamps covering
// [now-window, now], oldest first.
func mockStoreSeedTimestamps(now time.Time, window, spacing time.Duration) []time.Time {
	if window <= 0 || spacing <= 0 {
		return nil
	}
	windowStart := now.Add(-window)
	start := windowStart.Truncate(spacing)
	if start.Before(windowStart) {
		start = start.Add(spacing)
	}
	var out []time.Time
	for ts := start; !ts.After(now); ts = ts.Add(spacing) {
		out = append(out, ts)
	}
	return out
}

// timestampsAfter returns the suffix of the sorted slice strictly after the
// given time.
func timestampsAfter(timestamps []time.Time, after time.Time) []time.Time {
	idx := sort.Search(len(timestamps), func(i int) bool {
		return timestamps[i].After(after)
	})
	return timestamps[idx:]
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
	return unifiedresources.CanonicalKubernetesClusterSourceID(cluster)
}

func kubernetesNodeMetricID(cluster models.KubernetesCluster, node models.KubernetesNode) string {
	return unifiedresources.CanonicalKubernetesNodeSourceID(kubernetesClusterMetricID(cluster), node)
}

func kubernetesPodMetricID(cluster models.KubernetesCluster, pod models.KubernetesPod) string {
	sourceID := unifiedresources.CanonicalKubernetesPodSourceID(kubernetesClusterMetricID(cluster), pod)
	return unifiedresources.CanonicalKubernetesPodMetricID(sourceID)
}

func kubernetesDeploymentMetricID(cluster models.KubernetesCluster, deployment models.KubernetesDeployment) string {
	return unifiedresources.CanonicalKubernetesDeploymentSourceID(kubernetesClusterMetricID(cluster), deployment)
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
	return mock.SampleMetricSeries(resourceType, resourceID, metric, timestamps)
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
	numPoints := len(seedTimestamps)

	// Store backfill state: aligned timestamps per tier plan (capped by the
	// seed duration) plus existing coverage so restarts only fill the gap.
	storeSeedStart := time.Now()
	storeRows := 0
	var storePlanTimestamps map[metrics.Tier][]time.Time
	var storeCoverage map[metrics.Tier]map[metrics.SeriesKey]time.Time
	var storeBatch []metrics.WriteMetric
	if ms != nil {
		storePlanTimestamps = make(map[metrics.Tier][]time.Time, len(mockStoreSeedPlans))
		storeCoverage = make(map[metrics.Tier]map[metrics.SeriesKey]time.Time, len(mockStoreSeedPlans))
		for _, plan := range mockStoreSeedPlans {
			window := plan.window
			if window > seedDuration {
				window = seedDuration
			}
			storePlanTimestamps[plan.tier] = mockStoreSeedTimestamps(now, window, plan.spacing)
			coverage, err := ms.MaxTimestampsForTier(plan.tier)
			if err != nil {
				log.Warn().Err(err).Str("tier", string(plan.tier)).Msg("mock seeding: failed to read store coverage; reseeding full window")
				coverage = nil
			}
			storeCoverage[plan.tier] = coverage
		}
	}

	flushStoreBatch := func() {
		if ms == nil || len(storeBatch) == 0 {
			return
		}
		ms.WriteBatchSync(storeBatch)
		storeBatch = storeBatch[:0]
	}

	queueStorePoint := func(resourceType, resourceID, metricType string, value float64, ts time.Time, tier metrics.Tier) {
		storeBatch = append(storeBatch, metrics.WriteMetric{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			MetricType:   metricType,
			Value:        value,
			Timestamp:    ts,
			Tier:         tier,
		})
		storeRows++
		if len(storeBatch) >= mockStoreSeedBatchSize {
			flushStoreBatch()
		}
	}

	// seedStoreGapTimestamps returns the plan timestamps still missing for the
	// series whose coverage is tracked under coverageKey.
	seedStoreGapTimestamps := func(plan mockStoreSeedPlan, coverageKey metrics.SeriesKey) []time.Time {
		timestamps := storePlanTimestamps[plan.tier]
		if len(timestamps) == 0 {
			return nil
		}
		if coverage := storeCoverage[plan.tier]; coverage != nil {
			if maxTs, ok := coverage[coverageKey]; ok {
				timestamps = timestampsAfter(timestamps, maxTs)
			}
		}
		return timestamps
	}

	seedStoreSeries := func(resourceType, resourceID string, metricNames ...string) {
		if ms == nil || strings.TrimSpace(resourceID) == "" {
			return
		}
		for _, metricType := range metricNames {
			for _, plan := range mockStoreSeedPlans {
				coverageKey := metrics.NormalizedSeriesKey(resourceType, resourceID, metricType)
				for _, ts := range seedStoreGapTimestamps(plan, coverageKey) {
					queueStorePoint(resourceType, resourceID, metricType, mock.SampleMetric(resourceType, resourceID, metricType, ts), ts, plan.tier)
				}
			}
		}
	}

	// seedStoreStorageSeries derives the used/avail/total companions from the
	// sampled usage percentage, mirroring how live storage ticks persist
	// capacity metrics.
	seedStoreStorageSeries := func(storageID string, currentTotal float64) {
		if ms == nil || strings.TrimSpace(storageID) == "" || currentTotal <= 0 {
			return
		}
		for _, plan := range mockStoreSeedPlans {
			coverageKey := metrics.NormalizedSeriesKey("storage", storageID, "usage")
			for _, ts := range seedStoreGapTimestamps(plan, coverageKey) {
				usage := clampFloat(mock.SampleMetric("storage", storageID, "usage", ts), 0, 100)
				used := currentTotal * (usage / 100.0)
				avail := math.Max(0, currentTotal-used)
				queueStorePoint("storage", storageID, "usage", usage, ts, plan.tier)
				queueStorePoint("storage", storageID, "used", used, ts, plan.tier)
				queueStorePoint("storage", storageID, "avail", avail, ts, plan.tier)
				queueStorePoint("storage", storageID, "total", currentTotal, ts, plan.tier)
			}
		}
	}
	recordStorageTimeline := func(storageID string, currentTotal float64) {
		if strings.TrimSpace(storageID) == "" || currentTotal <= 0 || numPoints == 0 {
			return
		}

		usageSeries := canonicalMetricSeries("storage", storageID, "usage", seedTimestamps)
		usedSeries := make([]float64, numPoints)
		availSeries := make([]float64, numPoints)
		totalSeries := make([]float64, numPoints)
		for i := 0; i < numPoints; i++ {
			usage := clampFloat(usageSeries[i], 0, 100)
			used := currentTotal * (usage / 100.0)
			usageSeries[i] = usage
			usedSeries[i] = used
			availSeries[i] = math.Max(0, currentTotal-used)
			totalSeries[i] = currentTotal
		}
		mh.addStorageMetricSeries(storageID, "usage", usageSeries, seedTimestamps)
		mh.addStorageMetricSeries(storageID, "used", usedSeries, seedTimestamps)
		mh.addStorageMetricSeries(storageID, "avail", availSeries, seedTimestamps)
		mh.addStorageMetricSeries(storageID, "total", totalSeries, seedTimestamps)
		seedStoreStorageSeries(storageID, currentTotal)
	}

	recordNode := func(node models.Node) {
		if node.ID == "" {
			return
		}

		cpuSeries := canonicalMetricSeries("node", node.ID, "cpu", seedTimestamps)
		memSeries := canonicalMetricSeries("node", node.ID, "memory", seedTimestamps)
		diskSeries := canonicalMetricSeries("node", node.ID, "disk", seedTimestamps)

		mh.addNodeMetricSeries(node.ID, "cpu", cpuSeries, seedTimestamps)
		mh.addNodeMetricSeries(node.ID, "memory", memSeries, seedTimestamps)
		mh.addNodeMetricSeries(node.ID, "disk", diskSeries, seedTimestamps)
		seedStoreSeries("node", node.ID, "cpu", "memory", "disk")
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

		for _, metricID := range uniqueMetricIDs {
			mh.addGuestMetricSeries(metricID, "cpu", cpuSeries, seedTimestamps)
			mh.addGuestMetricSeries(metricID, "memory", memSeries, seedTimestamps)
			if includeDisk {
				mh.addGuestMetricSeries(metricID, "disk", diskSeries, seedTimestamps)
			}
			if includeDiskIO {
				mh.addGuestMetricSeries(metricID, "diskread", diskReadSeries, seedTimestamps)
				mh.addGuestMetricSeries(metricID, "diskwrite", diskWriteSeries, seedTimestamps)
			}
			if includeNetwork {
				mh.addGuestMetricSeries(metricID, "netin", netInSeries, seedTimestamps)
				mh.addGuestMetricSeries(metricID, "netout", netOutSeries, seedTimestamps)
			}
		}

		storeMetrics := []string{"cpu", "memory"}
		if includeDisk {
			storeMetrics = append(storeMetrics, "disk")
		}
		if includeDiskIO {
			storeMetrics = append(storeMetrics, "diskread", "diskwrite")
		}
		if includeNetwork {
			storeMetrics = append(storeMetrics, "netin", "netout")
		}
		seedStoreSeries(storeType, storeID, storeMetrics...)
	}

	log.Debug().Int("count", len(state.Nodes)).Msg("mock seeding: processing nodes")
	for _, node := range state.Nodes {
		recordNode(node)
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
	}

	k8sNodeCount := 0
	k8sPodCount := 0
	k8sDeploymentCount := 0
	for _, cluster := range state.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		k8sNodeCount += len(cluster.Nodes)
		k8sPodCount += len(cluster.Pods)
		k8sDeploymentCount += len(cluster.Deployments)
	}
	log.Debug().
		Int("clusters", len(state.KubernetesClusters)).
		Int("nodes", k8sNodeCount).
		Int("pods", k8sPodCount).
		Int("deployments", k8sDeploymentCount).
		Msg("mock seeding: processing kubernetes resources")
	for _, cluster := range state.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		clusterMetricID := kubernetesClusterMetricID(cluster)
		if clusterMetricID != "" {
			recordGuest([]string{clusterMetricID}, "k8s", clusterMetricID, true, true, true)
		}
		for _, node := range cluster.Nodes {
			metricID := kubernetesNodeMetricID(cluster, node)
			if metricID == "" {
				continue
			}
			recordGuest([]string{metricID}, "k8s", metricID, true, true, true)
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
		for _, deployment := range cluster.Deployments {
			metricID := kubernetesDeploymentMetricID(cluster, deployment)
			if metricID == "" {
				continue
			}
			recordGuest([]string{metricID}, "k8s", metricID, true, true, true)
		}
	}

	log.Debug().Int("count", len(state.Storage)).Msg("mock seeding: processing storage")
	for _, storage := range state.Storage {
		if storage.ID == "" {
			continue
		}
		_, _, total, _ := normalizedStorageCapacityMetrics(storage.Total, storage.Used, storage.Free, storage.Usage)
		recordStorageTimeline(storage.ID, total)
	}

	// seedDiskTelemetry seeds the canonical smart_temp/busy/read/write disk
	// series for one physical disk, shared by the native and TrueNAS fixture
	// disk loops below.
	seedDiskTelemetry := func(resourceID string) {
		tempSeries := canonicalMetricSeries("disk", resourceID, "smart_temp", seedTimestamps)
		busySeries := canonicalMetricSeries("disk", resourceID, "disk", seedTimestamps)
		diskReadSeries := canonicalMetricSeries("disk", resourceID, "diskread", seedTimestamps)
		diskWriteSeries := canonicalMetricSeries("disk", resourceID, "diskwrite", seedTimestamps)
		mh.addDiskMetricSeries(resourceID, "smart_temp", tempSeries, seedTimestamps)
		mh.addDiskMetricSeries(resourceID, "disk", busySeries, seedTimestamps)
		mh.addDiskMetricSeries(resourceID, "diskread", diskReadSeries, seedTimestamps)
		mh.addDiskMetricSeries(resourceID, "diskwrite", diskWriteSeries, seedTimestamps)
		seedStoreSeries("disk", resourceID, "smart_temp", "disk", "diskread", "diskwrite")
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
		seedDiskTelemetry(resourceID)
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

		seedStoreSeries("ceph", cephID, "usage")
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
		poolKey := mock.TrueNASPoolMetricID(trueNASFixtures.System.Hostname, pool.Name)
		diskSeries := canonicalMetricSeries("storage", poolKey, "usage", seedTimestamps)
		mh.addGuestMetricSeries(poolKey, "disk", diskSeries, seedTimestamps)
		recordStorageTimeline(poolKey, float64(pool.TotalBytes))
	}

	for _, dataset := range trueNASFixtures.Datasets {
		dsKey := mock.TrueNASDatasetMetricID(trueNASFixtures.System.Hostname, dataset.Name)
		totalBytes := dataset.UsedBytes + dataset.AvailBytes
		diskSeries := canonicalMetricSeries("storage", dsKey, "usage", seedTimestamps)
		mh.addGuestMetricSeries(dsKey, "disk", diskSeries, seedTimestamps)
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
		seedDiskTelemetry(resourceID)
	}

	for _, app := range trueNASFixtures.Apps {
		appID := mock.TrueNASAppMetricID(trueNASFixtures.System.Hostname, app)
		if strings.TrimSpace(appID) == "" {
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

	flushStoreBatch()
	if ms != nil {
		log.Info().
			Int("rows", storeRows).
			Dur("took", time.Since(storeSeedStart)).
			Msg("mock seeding: metrics store backfill completed")
	}
	log.Debug().Msg("mock seeding: completed")
}

func prepareMockMetricsHistory(
	graph mock.FixtureGraph,
	fixtureRevision uint64,
	now time.Time,
	seedDuration time.Duration,
	interval time.Duration,
	maxDataPoints int,
	storeSink *metrics.Store,
) (*MetricsHistory, bool) {
	seedAt := normalizeMockMetricTimestamp(now, interval)
	if storeSink != nil {
		history := NewMetricsHistory(maxDataPoints, seedDuration)
		seedMockMetricsHistory(history, storeSink, graph, seedAt, seedDuration, interval)
		return history, false
	}

	key := mockMetricsSeedCacheKey{
		fixtureRevision: fixtureRevision,
		seedAtUnixNano:  seedAt.UnixNano(),
		seedDuration:    seedDuration,
		sampleInterval:  interval,
		maxDataPoints:   maxDataPoints,
	}

	mockMetricsSeedCache.Lock()
	defer mockMetricsSeedCache.Unlock()
	if mockMetricsSeedCache.history != nil && mockMetricsSeedCache.key == key {
		return mockMetricsSeedCache.history.clone(), true
	}

	template := NewMetricsHistory(maxDataPoints, seedDuration)
	seedMockMetricsHistory(template, nil, graph, seedAt, seedDuration, interval)
	mockMetricsSeedCache.key = key
	mockMetricsSeedCache.history = template
	return template.clone(), false
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
			poolKey := mock.TrueNASPoolMetricID(snapshot.System.Hostname, pool.Name)
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
				ms.Write("pool", poolKey, "disk", usage, ts)
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
			dsKey := mock.TrueNASDatasetMetricID(snapshot.System.Hostname, dataset.Name)
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
				ms.Write("dataset", dsKey, "disk", usage, ts)
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
		appID := mock.TrueNASAppMetricID(snapshot.System.Hostname, app)
		if strings.TrimSpace(appID) == "" {
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
		if temperature := nodePrimaryTemperatureCelsius(node.Temperature); temperature != nil {
			mh.AddNodeMetric(node.ID, "temperature", *temperature, ts)
		}

		if ms != nil {
			ms.Write("node", node.ID, "cpu", cpu, ts)
			ms.Write("node", node.ID, "memory", memory, ts)
			ms.Write("node", node.ID, "disk", disk, ts)
			if temperature := nodePrimaryTemperatureCelsius(node.Temperature); temperature != nil {
				ms.Write("node", node.ID, "temperature", *temperature, ts)
			}
		}
	}

	recordGuestMetrics(mh, ms, adaptVMs(state.VMs), "vm", ts)
	recordGuestMetrics(mh, ms, adaptContainers(state.Containers), "container", ts)

	recordKubernetesMetric := func(metricID string, includeDiskIO bool) {
		if metricID == "" {
			return
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

		if !includeDiskIO {
			return
		}

		diskRead := mock.SampleMetric("k8s", metricID, "diskread", ts)
		diskWrite := mock.SampleMetric("k8s", metricID, "diskwrite", ts)
		mh.AddGuestMetric(metricID, "diskread", diskRead, ts)
		mh.AddGuestMetric(metricID, "diskwrite", diskWrite, ts)
		if ms != nil {
			ms.Write("k8s", metricID, "diskread", diskRead, ts)
			ms.Write("k8s", metricID, "diskwrite", diskWrite, ts)
		}
	}

	for _, cluster := range state.KubernetesClusters {
		if cluster.Hidden {
			continue
		}
		recordKubernetesMetric(kubernetesClusterMetricID(cluster), true)
		for _, node := range cluster.Nodes {
			recordKubernetesMetric(kubernetesNodeMetricID(cluster, node), true)
		}
		for _, pod := range cluster.Pods {
			recordKubernetesMetric(kubernetesPodMetricID(cluster, pod), false)
		}
		for _, deployment := range cluster.Deployments {
			recordKubernetesMetric(kubernetesDeploymentMetricID(cluster, deployment), true)
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
		temperature := hostPrimaryTemperatureCelsius(host.Sensors)
		mh.AddGuestMetric(hostKey, "cpu", cpu, ts)
		mh.AddGuestMetric(hostKey, "memory", memory, ts)
		mh.AddGuestMetric(hostKey, "disk", disk, ts)
		mh.AddGuestMetric(hostKey, "diskread", diskread, ts)
		mh.AddGuestMetric(hostKey, "diskwrite", diskwrite, ts)
		mh.AddGuestMetric(hostKey, "netin", netin, ts)
		mh.AddGuestMetric(hostKey, "netout", netout, ts)
		if temperature != nil {
			mh.AddGuestMetric(hostKey, "temperature", *temperature, ts)
		}

		if ms != nil {
			ms.Write("agent", host.ID, "cpu", cpu, ts)
			ms.Write("agent", host.ID, "memory", memory, ts)
			ms.Write("agent", host.ID, "disk", disk, ts)
			ms.Write("agent", host.ID, "diskread", diskread, ts)
			ms.Write("agent", host.ID, "diskwrite", diskwrite, ts)
			ms.Write("agent", host.ID, "netin", netin, ts)
			ms.Write("agent", host.ID, "netout", netout, ts)
			if temperature != nil {
				ms.Write("agent", host.ID, "temperature", *temperature, ts)
			}
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

	graph, fixtureRevision := mock.CurrentFixtureGraphWithRevision()
	state := graph.State
	log.Info().
		Int("nodes", len(state.Nodes)).
		Int("vms", len(state.VMs)).
		Int("containers", len(state.Containers)).
		Dur("seedDuration", seedDuration).
		Dur("sampleInterval", cfg.SampleInterval).
		Msg("Mock metrics sampler: seeding historical data")
	// Persist mock history into the metrics store only when the dev harness
	// opted in via PULSE_MOCK_SEED_METRICS_STORE — scripts/hot-dev.sh and
	// scripts/toggle-mock.sh set it exactly where they point PULSE_DATA_DIR at
	// the isolated tmp/mock-data dir. Without the opt-in (e.g. a production
	// install flipping PULSE_MOCK_MODE on its real data dir) the store may
	// hold real history, which seeded mock rows must never touch, so trend
	// generation stays in-memory only.
	var storeSink *metrics.Store
	if cfg.SeedStore {
		storeSink = m.metricsStore
		if storeSink == nil {
			log.Warn().Msg("PULSE_MOCK_SEED_METRICS_STORE is set but the metrics store is unavailable; mock report history will stay empty")
		}
	}
	history, cacheHit := prepareMockMetricsHistory(
		graph,
		fixtureRevision,
		time.Now(),
		seedDuration,
		cfg.SampleInterval,
		maxPoints,
		storeSink,
	)
	m.mu.Lock()
	m.metricsHistory = history
	m.mu.Unlock()
	if cacheHit {
		log.Info().Msg("Mock metrics sampler: reused cached historical seed")
	}
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
				recordMockStateToMetricsHistory(m.metricsHistory, storeSink, mock.CurrentFixtureGraph(), normalizeMockMetricTimestamp(time.Now(), cfg.SampleInterval))
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
