package monitoring

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/diskinventory"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

// inMemoryChartThreshold is the max duration reliably covered by the in-memory
// metrics buffer. With 1000 data-points at a 10 s polling interval the buffer
// holds ~2.8 h of data; 2 h is a safe conservative cut-off.
const inMemoryChartThreshold = 2 * time.Hour

// chartDownsampleTarget is the number of points returned per metric when
// falling back to the persistent store. 500 is more than enough for any
// sparkline or thumbnail chart.
const chartDownsampleTarget = 500

const (
	// When the requested chart window has no data (for example after running in
	// mock mode for a while), query a broader historical window so sparklines
	// can render immediately instead of waiting for enough fresh samples.
	chartGapFillLookbackMultiplier = 12
	chartGapFillLookbackMin        = 6 * time.Hour
	chartGapFillLookbackMax        = 7 * 24 * time.Hour
)

// For short chart ranges (<= inMemoryChartThreshold), we prefer in-memory data
// for freshness unless coverage is too shallow (for example after a restart).
// In that case we fall back to SQLite history to avoid mostly-empty charts.
const (
	shortRangeCoverageRatioNum = 3
	shortRangeCoverageRatioDen = 4
	shortRangeCoverageMaxSlack = 2 * time.Minute
)

const (
	mockChartPointTargetShortRange = 20
	mockChartPointTargetMedium     = 32
	mockChartPointTargetLong       = 48
	mockChartPointTargetExtended   = 64
)

const (
	mockDashboardWorkloadPrewarmDuration = time.Hour
	// Prewarming every guest creates an estate-sized chart cache once per mock
	// sampler tick. A first-viewport cohort removes cold-start latency while
	// keeping the rest available through the same lazy deterministic path.
	mockDashboardWorkloadPrewarmLimitPerType = 16
)

var storageChartMetricTypes = []string{"usage", "used", "avail", "total"}

type mockChartMetricMapCacheKey struct {
	kind         string
	resourceType string
	resourceID   string
	aux          string
	duration     time.Duration
}

// MonitorGuestMetricHistoryProvider optionally exposes source-native guest
// metric history through the canonical chart boundary when local Pulse history
// is shallow.
type MonitorGuestMetricHistoryProvider interface {
	GuestMetricHistory(m *Monitor, orgID string, resourceType string, duration time.Duration) map[string]map[string][]MetricPoint
}

// MonitorTargetedGuestMetricHistoryProvider avoids querying other appliances
// when a single resource needs a source-native chart fallback. The optional
// metric list lets the provider avoid retrieving unrelated series.
type MonitorTargetedGuestMetricHistoryProvider interface {
	GuestMetricHistoryForResource(ctx context.Context, m *Monitor, orgID, resourceType, resourceID string, duration time.Duration, metricTypes ...string) map[string][]MetricPoint
}

// GetGuestMetrics returns historical metrics for a guest
func (m *Monitor) GetGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint {
	if m == nil || m.metricsHistory == nil {
		return map[string][]MetricPoint{}
	}

	candidates := monitorGuestMetricKeyCandidates(guestID)
	best := map[string][]MetricPoint{}
	bestSpan := time.Duration(0)
	bestPoints := 0
	selected := false

	for _, candidate := range candidates {
		result := m.metricsHistory.GetAllGuestMetrics(candidate, duration)
		span := chartMapCoverageSpan(result)
		points := monitorMetricMapPointCount(result)
		if !selected || span > bestSpan || (span == bestSpan && points > bestPoints) {
			best = result
			bestSpan = span
			bestPoints = points
			selected = true
		}
	}

	if siblingKeys := m.proxmoxGuestMetricSiblingKeys("", guestID); len(siblingKeys) > 0 {
		for _, siblingKey := range siblingKeys {
			sibling := m.metricsHistory.GetAllGuestMetrics(siblingKey, duration)
			if len(sibling) == 0 {
				continue
			}
			best = spliceGuestMetricHistory(best, sibling)
		}
	}

	return best
}

// GetGuestMetricsInRange returns in-memory historical metrics for a guest within [start, end] inclusive.
func (m *Monitor) GetGuestMetricsInRange(guestID string, start, end time.Time) map[string][]MetricPoint {
	if m == nil || m.metricsHistory == nil {
		return map[string][]MetricPoint{}
	}

	candidates := monitorGuestMetricKeyCandidates(guestID)
	best := map[string][]MetricPoint{}
	bestSpan := time.Duration(0)
	bestPoints := 0
	selected := false

	for _, candidate := range candidates {
		result := m.metricsHistory.GetAllGuestMetricsInRange(candidate, start, end)
		span := chartMapCoverageSpan(result)
		points := monitorMetricMapPointCount(result)
		if !selected || span > bestSpan || (span == bestSpan && points > bestPoints) {
			best = result
			bestSpan = span
			bestPoints = points
			selected = true
		}
	}

	return best
}

func trailingLookbackForWindow(start, end time.Time) time.Duration {
	duration := end.Sub(start)
	if duration < 0 {
		duration = 0
	}
	lookback := time.Since(start)
	if lookback < duration {
		lookback = duration
	}
	return lookback
}

func clipMetricMapToWindow(metrics map[string][]MetricPoint, start, end time.Time) map[string][]MetricPoint {
	if len(metrics) == 0 {
		return map[string][]MetricPoint{}
	}
	clipped := make(map[string][]MetricPoint, len(metrics))
	for metric, points := range metrics {
		if kept := filterMetricsByWindow(points, start, end); len(kept) > 0 {
			clipped[metric] = kept
		}
	}
	return clipped
}

// GetGuestMetricsForWindowWithContext serves fallback/native guest metrics strictly
// within [start, end]. Each candidate is clipped to [start, end] before comparing
// coverage so out-of-window data cannot displace a better in-window series.
func (m *Monitor) GetGuestMetricsForWindowWithContext(ctx context.Context, inMemoryKey, sqlResourceType, sqlResourceID string, start, end time.Time, metricTypes ...string) map[string][]MetricPoint {
	requestedMetricTypes := normalizeChartMetricTypes(metricTypes)
	if mock.IsMockEnabled() {
		return filterChartMetricMap(mockGuestMetricsForWindow(sqlResourceType, sqlResourceID, start, end), requestedMetricTypes)
	}
	duration := end.Sub(start)
	if duration <= 0 {
		return map[string][]MetricPoint{}
	}

	best := filterChartMetricMap(m.GetGuestMetricsInRange(inMemoryKey, start, end), requestedMetricTypes)
	if len(requestedMetricTypes) > 0 && hasSufficientChartMapCoverageForMetrics(best, duration, requestedMetricTypes) {
		return best
	}

	if nativeHistory := m.nativeGuestMetricHistoryInWindow(ctx, sqlResourceType, start, end, requestedMetricTypes, sqlResourceID); len(nativeHistory) > 0 {
		if nativeMetrics, ok := nativeHistory[sqlResourceID]; ok {
			best = mergeGuestMetricHistory(best, filterChartMetricMap(nativeMetrics, requestedMetricTypes), duration)
		}
	}
	return best
}

// GetGuestMetricsForChart returns guest metrics optimised for chart display.
// Short ranges are served from the in-memory ring buffer; longer ranges fall
// back to the persistent SQLite store with LTTB downsampling.
//
// inMemoryKey is the key used in the in-memory buffer (e.g. "docker:abc123").
// sqlResourceType/sqlResourceID are the type/id used in the SQLite store
// (e.g. "dockerContainer"/"abc123").
func (m *Monitor) GetGuestMetricsForChart(inMemoryKey, sqlResourceType, sqlResourceID string, duration time.Duration, metricTypes ...string) map[string][]MetricPoint {
	return m.GetGuestMetricsForChartWithContext(context.Background(), inMemoryKey, sqlResourceType, sqlResourceID, duration, metricTypes...)
}

// GetGuestMetricsForChartWithContext lets a targeted native fallback stop
// abandoned work when its transport permits it. Local reads are unchanged.
func (m *Monitor) GetGuestMetricsForChartWithContext(ctx context.Context, inMemoryKey, sqlResourceType, sqlResourceID string, duration time.Duration, metricTypes ...string) map[string][]MetricPoint {
	requestedMetricTypes := normalizeChartMetricTypes(metricTypes)
	if mock.IsMockEnabled() {
		return filterChartMetricMap(m.mockGuestMetricsForChart(inMemoryKey, sqlResourceType, sqlResourceID, duration, m.GetGuestMetrics(inMemoryKey, duration)), requestedMetricTypes)
	}

	inMemoryResult := map[string][]MetricPoint{}
	if m.shouldLoadGuestChartInMemory(inMemoryKey, requestedMetricTypes, duration) {
		inMemoryResult = filterChartMetricMap(m.GetGuestMetrics(inMemoryKey, duration), requestedMetricTypes)
	}

	if hasSufficientChartMapCoverageForMetrics(inMemoryResult, duration, requestedMetricTypes) {
		return inMemoryResult
	}

	// inMemoryResult is a fresh map from GetGuestMetrics (slices already
	// owned); merge mutates and returns it in place — no clone needed.
	best := inMemoryResult
	if m.metricsStore != nil {
		if converted, ok := m.queryStoreMetricMapWithGapFillAliases(sqlResourceType, sqlResourceID, duration); ok {
			best = mergeGuestMetricHistory(best, filterChartMetricMap(converted, requestedMetricTypes), duration)
		}
	}
	if len(requestedMetricTypes) > 0 && hasSufficientChartMapCoverageForMetrics(best, duration, requestedMetricTypes) {
		return best
	}
	if nativeHistory := m.nativeGuestMetricHistory(ctx, sqlResourceType, duration, requestedMetricTypes, sqlResourceID); len(nativeHistory) > 0 {
		if nativeMetrics, ok := nativeHistory[sqlResourceID]; ok {
			best = mergeGuestMetricHistory(best, filterChartMetricMap(nativeMetrics, requestedMetricTypes), duration)
		}
	}
	return best
}

// GetNodeMetrics returns historical metrics for a node
func (m *Monitor) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
}

// GetNodeMetricsInRange returns in-memory historical metrics for a node within [start, end] inclusive.
func (m *Monitor) GetNodeMetricsInRange(nodeID, metricType string, start, end time.Time) []MetricPoint {
	if m == nil || m.metricsHistory == nil {
		return nil
	}
	return m.metricsHistory.GetNodeMetricsInRange(nodeID, metricType, start, end)
}

// GetNodeMetricsForWindow returns fallback node metrics strictly within [start, end].
func (m *Monitor) GetNodeMetricsForWindow(nodeID, metricType string, start, end time.Time) []MetricPoint {
	if mock.IsMockEnabled() {
		return mockNodeMetricsForWindow(nodeID, []string{metricType}, start, end)[metricType]
	}
	return m.GetNodeMetricsInRange(nodeID, metricType, start, end)
}

// GetNodeMetricsForChart returns node metrics for a single metric type,
// falling back to SQLite + LTTB for longer ranges.
func (m *Monitor) GetNodeMetricsForChart(nodeID, metricType string, duration time.Duration) []MetricPoint {
	var inMemoryPoints []MetricPoint
	if mock.IsMockEnabled() {
		if m != nil && m.metricsHistory != nil {
			inMemoryPoints = m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
		}
		return m.mockNodeMetricsForChart(nodeID, metricType, duration, inMemoryPoints)
	}
	if m != nil && m.shouldLoadNodeChartInMemory(nodeID, []string{metricType}, duration) {
		inMemoryPoints = m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
	}

	if m.metricsStore == nil {
		return inMemoryPoints
	}
	if hasSufficientChartSeriesCoverage(inMemoryPoints, duration) {
		return inMemoryPoints
	}

	downsampled, ok := m.queryStoreMetricSeriesWithGapFill("node", nodeID, metricType, duration)
	if !ok {
		return inMemoryPoints
	}
	if chartSeriesCoverageSpan(downsampled) <= chartSeriesCoverageSpan(inMemoryPoints) {
		return inMemoryPoints
	}
	return downsampled
}

// GetStorageMetrics returns historical metrics for storage
func (m *Monitor) GetStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	if m == nil || m.metricsHistory == nil {
		return map[string][]MetricPoint{}
	}
	return m.metricsHistory.GetAllStorageMetrics(storageID, duration)
}

// GetStorageMetricsInRange returns in-memory historical metrics for storage within [start, end] inclusive.
func (m *Monitor) GetStorageMetricsInRange(storageID string, start, end time.Time) map[string][]MetricPoint {
	if m == nil || m.metricsHistory == nil {
		return map[string][]MetricPoint{}
	}
	return m.metricsHistory.GetAllStorageMetricsInRange(storageID, start, end)
}

// GetDiskMetrics returns historical metrics for a physical disk.
func (m *Monitor) GetDiskMetrics(resourceID string, metricType string, duration time.Duration) []MetricPoint {
	if m == nil || m.metricsHistory == nil {
		return nil
	}
	return m.metricsHistory.GetDiskMetrics(resourceID, metricType, duration)
}

// GetDiskMetricsInRange returns in-memory historical metrics for a physical disk within [start, end] inclusive.
func (m *Monitor) GetDiskMetricsInRange(resourceID, metricType string, start, end time.Time) []MetricPoint {
	if m == nil || m.metricsHistory == nil {
		return nil
	}
	return m.metricsHistory.GetDiskMetricsInRange(resourceID, metricType, start, end)
}

// GetDiskMetricsForWindowWithSource selects fallback/native disk metrics strictly
// within [start, end]. Native candidates are clipped per provider before
// downsampling and coverage comparison so provenance reflects the in-window series.
func (m *Monitor) GetDiskMetricsForWindowWithSource(resourceID, metricType string, start, end time.Time) ([]MetricPoint, string) {
	if mock.IsMockEnabled() {
		return mockDiskMetricsForWindow(resourceID, []string{metricType}, start, end)[metricType], "mock_synthetic"
	}
	duration := end.Sub(start)
	if duration <= 0 {
		return nil, "memory"
	}

	best := cloneMetricSeries(m.GetDiskMetricsInRange(resourceID, metricType, start, end))
	source := "memory"
	if metricType == "smart_temp" && !hasSufficientChartSeriesCoverage(best, duration) {
		if clippedNative := m.nativePhysicalDiskTemperatureHistoryInWindow(start, end)[resourceID]; len(clippedNative) > 0 {
			clippedNative = lttb(clippedNative, chartDownsampleTarget)
			if shouldPreferMetricSeries(best, clippedNative, duration) {
				return cloneMetricSeries(clippedNative), "native"
			}
		}
	}
	return best, source
}

// GetDiskMetricsForChart returns physical-disk metrics optimized for chart
// display, preferring in-memory data for freshness and falling back to the
// persistent store and source-native temperature history when coverage is shallow.
func (m *Monitor) GetDiskMetricsForChart(resourceID string, metricType string, duration time.Duration) []MetricPoint {
	points, _ := m.GetDiskMetricsForChartWithSource(resourceID, metricType, duration)
	return points
}

// GetDiskMetricsForChartWithSource identifies the selected series, not every
// source that was tried. A native temperature series replaces, never splices
// with, the local observations.
func (m *Monitor) GetDiskMetricsForChartWithSource(resourceID string, metricType string, duration time.Duration) ([]MetricPoint, string) {
	inMemoryPoints := m.GetDiskMetrics(resourceID, metricType, duration)
	if mock.IsMockEnabled() {
		return m.mockDiskMetricsForChart(resourceID, metricType, duration, inMemoryPoints), "mock_synthetic"
	}

	if hasSufficientChartSeriesCoverage(inMemoryPoints, duration) {
		return inMemoryPoints, "memory"
	}

	best := cloneMetricSeries(inMemoryPoints)
	source := "memory"
	if converted, ok := m.queryStoreMetricMapWithGapFill("disk", resourceID, duration); ok {
		if candidate := converted[metricType]; shouldPreferMetricSeries(best, candidate, duration) {
			best = cloneMetricSeries(candidate)
			source = "store"
		}
	}
	if metricType == "smart_temp" && !hasSufficientChartSeriesCoverage(best, duration) {
		if nativePoints := m.nativePhysicalDiskTemperatureHistory(duration)[resourceID]; len(nativePoints) > 0 {
			nativePoints = lttb(nativePoints, chartDownsampleTarget)
			if shouldPreferMetricSeries(best, nativePoints, duration) {
				return cloneMetricSeries(nativePoints), "native"
			}
		}
	}

	return best, source
}

// GetStorageMetricsForChart returns storage metrics, falling back to SQLite +
// LTTB for longer ranges.
func (m *Monitor) GetStorageMetricsForChart(storageID string, duration time.Duration) map[string][]MetricPoint {
	series, _, _ := m.readStorageMetricHistory(storageID, duration, true)
	return series
}

// ReadStorageMetricHistory uses the same storage history selection as charts,
// but never fills an empty requested window with older observations. Analytical
// consumers must see store errors and synthetic provenance rather than treating
// an empty, failed or generated read as current operational evidence.
func (m *Monitor) ReadStorageMetricHistory(storageID string, duration time.Duration) (map[string][]MetricPoint, string, error) {
	return m.readStorageMetricHistory(storageID, duration, false)
}

func (m *Monitor) readStorageMetricHistory(storageID string, duration time.Duration, allowGapFill bool) (map[string][]MetricPoint, string, error) {
	inMemoryResult := map[string][]MetricPoint{}
	if m != nil && m.metricsHistory != nil {
		inMemoryResult = m.metricsHistory.GetAllStorageMetrics(storageID, duration)
	}
	if m == nil {
		return inMemoryResult, "unavailable", nil
	}
	if mock.IsMockEnabled() {
		return m.mockStorageMetricsForChartCached(storageID, duration, inMemoryResult, m.GetUnifiedReadStateOrSnapshot), "mock_synthetic", nil
	}
	if m.metricsStore == nil {
		return inMemoryResult, "memory", nil
	}
	if allowGapFill && hasSufficientChartMapCoverageForMetrics(inMemoryResult, duration, storageChartMetricTypes) {
		return inMemoryResult, "memory", nil
	}

	// inMemoryResult is a fresh map from GetAllStorageMetrics; merge mutates
	// in place.
	best := inMemoryResult
	hadMemorySamples := monitorMetricMapPointCount(inMemoryResult) > 0
	converted, ok, err := m.queryStoreMetricHistory("storage", storageID, duration, allowGapFill)
	if err != nil {
		return best, "", err
	}
	if ok {
		best = mergeMetricHistory(best, converted, duration)
		source := "store"
		if hadMemorySamples {
			source = "store+memory"
		}
		return best, source, nil
	}
	return best, "memory", nil
}

// DiskChartEntry holds temperature chart data and display metadata for a
// single physical disk.
type DiskChartEntry struct {
	Name        string        `json:"name"`
	Node        string        `json:"node"`
	Instance    string        `json:"instance"`
	Temperature []MetricPoint `json:"temperature"`
}

// GetPhysicalDiskTemperatureCharts returns temperature time-series for all
// physical disks that have temperature data. The Monitor owns state access
// (monitoring is exempt from the ReadState-only rule), so this keeps state
// reads out of API handler code.
//
// Uses batch queries to load all disk metrics in 1-2 SQL calls instead of N
// (one per disk), avoiding N+1 query patterns on systems with many disks.
func (m *Monitor) GetPhysicalDiskTemperatureCharts(duration time.Duration) map[string]DiskChartEntry {
	if m == nil {
		return nil
	}
	mockEnabled := mock.IsMockEnabled()

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return nil
	}
	metricsTargetStore := m.currentMetricsTargetStore()

	// Phase 1: Collect disk metadata and resource IDs.
	type diskMeta struct {
		resourceID           string
		name                 string
		node                 string
		instance             string
		temperatureCollected bool
	}
	var disks []diskMeta
	for _, disk := range readState.PhysicalDisks() {
		if disk == nil || disk.Temperature() <= 0 {
			continue
		}
		resourceID := ""
		if metricsTargetStore != nil {
			if target := metricsTargetStore.MetricsTargetForResource(disk.ID()); target != nil {
				resourceID = strings.TrimSpace(target.ResourceID)
			}
		}
		if resourceID == "" {
			resourceID = strings.TrimSpace(disk.MetricResourceID())
		}
		if resourceID == "" {
			continue
		}
		name := strings.TrimSpace(disk.Model())
		if name == "" {
			name = strings.TrimSpace(disk.DevPath())
		}
		disks = append(disks, diskMeta{
			resourceID:           resourceID,
			name:                 name,
			node:                 strings.TrimSpace(disk.Node()),
			instance:             strings.TrimSpace(disk.Instance()),
			temperatureCollected: diskinventory.TemperatureCollected(disk.Temperature(), disk.Collection()),
		})
	}

	if len(disks) == 0 {
		return make(map[string]DiskChartEntry)
	}

	// Phase 2: Batch-load metrics for all disks (1-2 queries instead of N).
	resourceIDs := make([]string, len(disks))
	for i, d := range disks {
		resourceIDs[i] = d.resourceID
	}
	inMemoryHistory := make(map[string][]MetricPoint, len(disks))
	if m.metricsHistory != nil {
		for _, d := range disks {
			inMemoryHistory[d.resourceID] = m.metricsHistory.GetDiskMetrics(d.resourceID, "smart_temp", duration)
		}
	}
	var batchMetrics map[string]map[string][]MetricPoint
	var nativeHistory map[string][]MetricPoint
	if !mockEnabled {
		batchMetrics = m.queryStoreBatchMetricMapWithGapFill("disk", resourceIDs, duration, []string{"smart_temp"})
		nativeHistory = m.nativePhysicalDiskTemperatureHistory(duration)
	}

	// Phase 3: Build result entries.
	result := make(map[string]DiskChartEntry, len(disks))
	for _, d := range disks {
		tempPoints := cloneMetricSeries(inMemoryHistory[d.resourceID])
		if mockEnabled {
			tempPoints = m.mockDiskMetricsForChart(d.resourceID, "smart_temp", duration, tempPoints)
		} else {
			if resMetrics, ok := batchMetrics[d.resourceID]; ok {
				if pts, found := resMetrics["smart_temp"]; found {
					if shouldPreferMetricSeries(tempPoints, pts, duration) {
						tempPoints = cloneMetricSeries(pts)
					}
				}
			}
			if nativePoints, ok := nativeHistory[d.resourceID]; ok {
				nativePoints = lttb(nativePoints, chartDownsampleTarget)
				if shouldPreferMetricSeries(tempPoints, nativePoints, duration) {
					tempPoints = cloneMetricSeries(nativePoints)
				}
			}
		}

		// Neither a current nor a retained temperature is two historical
		// observations. Keep empty and single-point histories intact instead
		// of inventing a flat line. A collected reading still supplies current
		// disk metadata; a retained-only reading with no history supplies none.
		if len(tempPoints) == 0 && !d.temperatureCollected {
			continue
		}

		result[d.resourceID] = DiskChartEntry{
			Name:        d.name,
			Node:        d.node,
			Instance:    d.instance,
			Temperature: tempPoints,
		}
	}

	return result
}

func (m *Monitor) nativePhysicalDiskTemperatureHistory(duration time.Duration) map[string][]MetricPoint {
	if mock.IsMockEnabled() {
		return nil
	}

	providers := m.supplementalProviderSnapshot()
	if len(providers) == 0 {
		return nil
	}

	orgID := "default"
	if m != nil {
		if trimmed := strings.TrimSpace(m.GetOrgID()); trimmed != "" {
			orgID = trimmed
		}
	}

	history := make(map[string][]MetricPoint)
	for _, provider := range providers {
		historyProvider, ok := provider.(MonitorPhysicalDiskTemperatureHistoryProvider)
		if !ok {
			continue
		}
		nativeHistory := historyProvider.PhysicalDiskTemperatureHistory(m, orgID, duration)
		for resourceID, points := range nativeHistory {
			if strings.TrimSpace(resourceID) == "" || len(points) == 0 {
				continue
			}
			if existing, ok := history[resourceID]; !ok || chartSeriesCoverageSpan(points) > chartSeriesCoverageSpan(existing) {
				history[resourceID] = points
			}
		}
	}
	if len(history) == 0 {
		return nil
	}
	return history
}

func (m *Monitor) nativePhysicalDiskTemperatureHistoryInWindow(start, end time.Time) map[string][]MetricPoint {
	if mock.IsMockEnabled() || !end.After(start) {
		return nil
	}

	providers := m.supplementalProviderSnapshot()
	if len(providers) == 0 {
		return nil
	}

	orgID := "default"
	if m != nil {
		if trimmed := strings.TrimSpace(m.GetOrgID()); trimmed != "" {
			orgID = trimmed
		}
	}

	duration := end.Sub(start)
	lookback := trailingLookbackForWindow(start, end)
	history := make(map[string][]MetricPoint)
	for _, provider := range providers {
		historyProvider, ok := provider.(MonitorPhysicalDiskTemperatureHistoryProvider)
		if !ok {
			continue
		}
		nativeHistory := historyProvider.PhysicalDiskTemperatureHistory(m, orgID, lookback)
		for resourceID, points := range nativeHistory {
			if strings.TrimSpace(resourceID) == "" || len(points) == 0 {
				continue
			}
			clipped := filterMetricsByWindow(points, start, end)
			if len(clipped) == 0 {
				continue
			}
			if existing, ok := history[resourceID]; !ok || shouldPreferMetricSeries(existing, clipped, duration) {
				history[resourceID] = clipped
			}
		}
	}
	if len(history) == 0 {
		return nil
	}
	return history
}

func (m *Monitor) currentMetricsTargetStore() MetricsTargetResourceStore {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	store, ok := m.resourceStore.(MetricsTargetResourceStore)
	if !ok {
		return nil
	}
	return store
}

// queryStoreBatchMetricMapWithGapFill queries metrics for multiple resource IDs
// of the same type in a single batch. If some resources have no data in the
// primary window, re-queries those with a broader lookback window.
// Returns map[resourceID]map[metricType][]MetricPoint (already downsampled).
func (m *Monitor) queryStoreBatchMetricMapWithGapFill(resourceType string, resourceIDs []string, duration time.Duration, metricTypes []string) map[string]map[string][]MetricPoint {
	if m == nil || m.metricsStore == nil || len(resourceIDs) == 0 {
		return nil
	}

	end := time.Now()
	start := end.Add(-duration)

	// queryBatch fetches and downsamples metrics for the given IDs.
	queryBatch := func(ids []string, from time.Time) map[string]map[string][]MetricPoint {
		sqlResult, err := m.metricsStore.QueryMetricValuesBatch(resourceType, ids, metricTypes, from, end, 0)
		if err != nil || len(sqlResult) == 0 {
			return nil
		}
		// The store returns detached canonical timestamp/value points. Keep
		// their ownership while applying the existing chart sampling target.
		for _, metricMap := range sqlResult {
			downsampleMetricMap(metricMap, chartDownsampleTarget)
		}
		return sqlResult
	}

	result := queryBatch(resourceIDs, start)
	if result == nil {
		result = make(map[string]map[string][]MetricPoint)
	}

	// Collect resource IDs that returned no data for gap-fill retry.
	var missing []string
	for _, id := range resourceIDs {
		if _, ok := result[id]; !ok {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		gapStart := end.Add(-chartGapFillLookbackWindow(duration))
		if gapResult := queryBatch(missing, gapStart); gapResult != nil {
			for id, metricMap := range gapResult {
				result[id] = metricMap
			}
		}
	}

	return result
}

// GuestChartRequest identifies a single guest for batch chart retrieval.
type GuestChartRequest struct {
	InMemoryKey   string // key in the in-memory ring buffer
	SQLResourceID string // resource_id in the SQLite store
}

func normalizeChartMetricTypes(metricTypes []string) []string {
	if len(metricTypes) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(metricTypes))
	normalized := make([]string, 0, len(metricTypes))
	for _, metricType := range metricTypes {
		canonical := strings.ToLower(strings.TrimSpace(metricType))
		if canonical == "" {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		normalized = append(normalized, canonical)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func filterChartMetricMap(metricMap map[string][]MetricPoint, metricTypes []string) map[string][]MetricPoint {
	if len(metricTypes) == 0 || len(metricMap) == 0 {
		return metricMap
	}

	filtered := make(map[string][]MetricPoint, len(metricTypes))
	for _, metricType := range metricTypes {
		if points, ok := metricMap[metricType]; ok {
			filtered[metricType] = points
		}
	}
	return filtered
}

func (m *Monitor) shouldLoadGuestChartInMemory(inMemoryKey string, metricTypes []string, duration time.Duration) bool {
	if m == nil || m.metricsHistory == nil {
		return false
	}
	if m.metricsStore == nil || duration <= inMemoryChartThreshold {
		return true
	}
	return m.metricsHistory.GuestMetricCoverageSpan(inMemoryKey, metricTypes, duration) >= chartCoverageRequiredSpan(duration)
}

func (m *Monitor) shouldLoadNodeChartInMemory(nodeID string, metricTypes []string, duration time.Duration) bool {
	if m == nil || m.metricsHistory == nil {
		return false
	}
	if m.metricsStore == nil || duration <= inMemoryChartThreshold {
		return true
	}
	return m.metricsHistory.NodeMetricCoverageSpan(nodeID, metricTypes, duration) >= chartCoverageRequiredSpan(duration)
}

// GetGuestMetricsForChartBatch returns chart metrics for multiple guests of the
// same SQL resource type, using batch SQL queries instead of N individual
// queries. Results are keyed by SQLResourceID.
func (m *Monitor) GetGuestMetricsForChartBatch(
	sqlResourceType string,
	requests []GuestChartRequest,
	duration time.Duration,
	metricTypes ...string,
) map[string]map[string][]MetricPoint {
	if m == nil || len(requests) == 0 {
		return nil
	}

	requestedMetricTypes := normalizeChartMetricTypes(metricTypes)

	if mock.IsMockEnabled() {
		result := make(map[string]map[string][]MetricPoint, len(requests))
		for _, req := range requests {
			inMemory := m.GetGuestMetrics(req.InMemoryKey, duration)
			result[req.SQLResourceID] = filterChartMetricMap(m.mockGuestMetricsForChart(
				req.InMemoryKey,
				sqlResourceType,
				req.SQLResourceID,
				duration,
				inMemory,
			), requestedMetricTypes)
		}
		return result
	}

	result := make(map[string]map[string][]MetricPoint, len(requests))

	// Phase 1: Check in-memory for all guests and identify which need fallback.
	var needFallback []string
	for _, req := range requests {
		inMemory := map[string][]MetricPoint{}
		if m.shouldLoadGuestChartInMemory(req.InMemoryKey, requestedMetricTypes, duration) {
			inMemory = filterChartMetricMap(m.GetGuestMetrics(req.InMemoryKey, duration), requestedMetricTypes)
		}
		result[req.SQLResourceID] = inMemory
		if hasSufficientChartMapCoverage(inMemory, duration) {
			continue
		}
		needFallback = append(needFallback, req.SQLResourceID)
	}

	if len(needFallback) == 0 {
		return result
	}

	// Phase 2: Batch-query store, trying all resource type candidates and
	// keeping the best coverage per ID. Ties on span are broken by total
	// point count, matching the single-resource alias resolution logic.
	storeResults := make(map[string]map[string][]MetricPoint)
	if m.metricsStore != nil {
		queryIDs := make([]string, 0, len(needFallback))
		querySeen := make(map[string]struct{}, len(needFallback))
		siblingToPrimary := make(map[string]string)
		for _, id := range needFallback {
			if _, ok := querySeen[id]; !ok {
				querySeen[id] = struct{}{}
				queryIDs = append(queryIDs, id)
			}
			for _, siblingID := range m.proxmoxGuestMetricSiblingKeys(sqlResourceType, id) {
				if _, ok := querySeen[siblingID]; !ok {
					querySeen[siblingID] = struct{}{}
					queryIDs = append(queryIDs, siblingID)
					siblingToPrimary[siblingID] = id
				}
			}
		}

		for _, candidate := range monitorStoreResourceTypeCandidates(sqlResourceType) {
			batch := m.queryStoreBatchMetricMapWithGapFill(candidate, queryIDs, duration, requestedMetricTypes)
			for id, metricMap := range batch {
				if primaryID, isSibling := siblingToPrimary[id]; isSibling {
					storeResults[primaryID] = spliceGuestMetricHistory(storeResults[primaryID], metricMap)
					continue
				}
				if existing, ok := storeResults[id]; ok {
					newSpan := chartMapCoverageSpan(metricMap)
					oldSpan := chartMapCoverageSpan(existing)
					if newSpan < oldSpan || (newSpan == oldSpan && monitorMetricMapPointCount(metricMap) <= monitorMetricMapPointCount(existing)) {
						continue
					}
				}
				storeResults[id] = metricMap
			}
		}
	}
	nativeResults := m.nativeGuestMetricHistory(context.Background(), sqlResourceType, duration, nil)

	// Phase 3: Merge fallback data per metric type.
	// result[id] is already owned (set from filterChartMetricMap output above);
	// merge mutates in place.
	for _, id := range needFallback {
		best := result[id]
		if storeData, ok := storeResults[id]; ok {
			best = mergeGuestMetricHistory(best, filterChartMetricMap(storeData, requestedMetricTypes), duration)
		}
		if nativeData, ok := nativeResults[id]; ok {
			best = mergeGuestMetricHistory(best, filterChartMetricMap(nativeData, requestedMetricTypes), duration)
		}
		result[id] = best
	}

	return result
}

// GetNodeMetricsForChartBatch returns chart metrics for multiple nodes,
// using batch SQL queries instead of N×M individual queries (where M is the
// number of metric types). Results are keyed by nodeID, then by metric type.
func (m *Monitor) GetNodeMetricsForChartBatch(
	nodeIDs []string,
	metricTypes []string,
	duration time.Duration,
) map[string]map[string][]MetricPoint {
	if m == nil || len(nodeIDs) == 0 {
		return nil
	}

	if mock.IsMockEnabled() {
		result := make(map[string]map[string][]MetricPoint, len(nodeIDs))
		for _, nodeID := range nodeIDs {
			nodeResult := make(map[string][]MetricPoint, len(metricTypes))
			for _, metricType := range metricTypes {
				var inMemoryPoints []MetricPoint
				if m != nil && m.metricsHistory != nil {
					inMemoryPoints = m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
				}
				nodeResult[metricType] = m.mockNodeMetricsForChart(nodeID, metricType, duration, inMemoryPoints)
			}
			result[nodeID] = nodeResult
		}
		return result
	}

	result := make(map[string]map[string][]MetricPoint, len(nodeIDs))

	// Phase 1: Check in-memory for all nodes and identify which need store.
	var needStore []string
	for _, nid := range nodeIDs {
		nodeResult := make(map[string][]MetricPoint, len(metricTypes))

		allSufficient := true
		loadInMemory := m.shouldLoadNodeChartInMemory(nid, metricTypes, duration)
		for _, mt := range metricTypes {
			if !loadInMemory {
				nodeResult[mt] = nil
				allSufficient = false
				continue
			}

			points := m.metricsHistory.GetNodeMetrics(nid, mt, duration)
			nodeResult[mt] = points
			if m.metricsStore != nil && !hasSufficientChartSeriesCoverage(points, duration) {
				allSufficient = false
			}
		}
		result[nid] = nodeResult
		if !allSufficient {
			needStore = append(needStore, nid)
		}
	}

	if len(needStore) == 0 {
		return result
	}

	// Phase 2: Batch-query store for all nodes that need it.
	batchResult := m.queryStoreBatchMetricMapWithGapFill("node", needStore, duration, metricTypes)

	// Phase 3: Merge — per metric type, use store data if better coverage.
	for _, nid := range needStore {
		storeData, ok := batchResult[nid]
		if !ok {
			continue
		}
		for _, mt := range metricTypes {
			storePoints, found := storeData[mt]
			if !found {
				continue
			}
			inMemory := result[nid][mt]
			if chartSeriesCoverageSpan(storePoints) <= chartSeriesCoverageSpan(inMemory) {
				continue
			}
			result[nid][mt] = storePoints
		}
	}

	return result
}

// GetStorageMetricsForChartBatch returns chart metrics for multiple storage
// pools, using batch SQL queries instead of N individual queries.
// Results are keyed by storageID.
func (m *Monitor) GetStorageMetricsForChartBatch(
	storageIDs []string,
	duration time.Duration,
) map[string]map[string][]MetricPoint {
	if m == nil || len(storageIDs) == 0 {
		return nil
	}

	result := make(map[string]map[string][]MetricPoint, len(storageIDs))

	if mock.IsMockEnabled() {
		// One read of the estate view for every pool that needs a total.
		readState := sync.OnceValue(m.GetUnifiedReadStateOrSnapshot)
		for _, sid := range storageIDs {
			inMemory := map[string][]MetricPoint{}
			if m.metricsHistory != nil {
				inMemory = m.metricsHistory.GetAllStorageMetrics(sid, duration)
			}
			result[sid] = m.mockStorageMetricsForChartCached(sid, duration, inMemory, readState)
		}
		return result
	}

	// Phase 1: Check in-memory for all storage pools.
	var needStore []string
	for _, sid := range storageIDs {
		inMemory := map[string][]MetricPoint{}
		if m.metricsHistory != nil {
			inMemory = m.metricsHistory.GetAllStorageMetrics(sid, duration)
		}
		if m.metricsStore == nil {
			result[sid] = inMemory
			continue
		}
		if hasSufficientChartMapCoverageForMetrics(inMemory, duration, storageChartMetricTypes) {
			result[sid] = inMemory
			continue
		}
		needStore = append(needStore, sid)
		result[sid] = inMemory // keep as fallback
	}

	if len(needStore) == 0 {
		return result
	}

	// Phase 2: Batch-query store.
	batchResult := m.queryStoreBatchMetricMapWithGapFill("storage", needStore, duration, storageChartMetricTypes)

	// Phase 3: Merge — use store data if better coverage.
	for _, id := range needStore {
		storeData, ok := batchResult[id]
		if !ok {
			continue
		}
		result[id] = mergeMetricHistory(result[id], storeData, duration)
	}

	return result
}

func (m *Monitor) nativeGuestMetricHistory(ctx context.Context, resourceType string, duration time.Duration, metricTypes []string, resourceIDs ...string) map[string]map[string][]MetricPoint {
	providers := m.supplementalProviderSnapshot()
	if len(providers) == 0 {
		return nil
	}

	orgID := "default"
	if m != nil {
		if trimmed := strings.TrimSpace(m.GetOrgID()); trimmed != "" {
			orgID = trimmed
		}
	}

	history := make(map[string]map[string][]MetricPoint)
	for _, provider := range providers {
		if ctx.Err() != nil {
			return nil
		}
		if targeted, ok := provider.(MonitorTargetedGuestMetricHistoryProvider); ok && len(resourceIDs) == 1 {
			id := resourceIDs[0]
			metricMap := targeted.GuestMetricHistoryForResource(ctx, m, orgID, resourceType, id, duration, metricTypes...)
			if len(metricMap) > 0 {
				history[id] = mergeGuestMetricHistory(history[id], metricMap, duration)
			}
			continue
		}
		historyProvider, ok := provider.(MonitorGuestMetricHistoryProvider)
		if !ok {
			continue
		}
		nativeHistory := historyProvider.GuestMetricHistory(m, orgID, resourceType, duration)
		for resourceID, metricMap := range nativeHistory {
			if strings.TrimSpace(resourceID) == "" || len(metricMap) == 0 {
				continue
			}
			existing := history[resourceID]
			history[resourceID] = mergeGuestMetricHistory(existing, metricMap, duration)
		}
	}
	if len(history) == 0 {
		return nil
	}
	return history
}

func (m *Monitor) nativeGuestMetricHistoryInWindow(ctx context.Context, resourceType string, start, end time.Time, metricTypes []string, resourceIDs ...string) map[string]map[string][]MetricPoint {
	if !end.After(start) {
		return nil
	}
	providers := m.supplementalProviderSnapshot()
	if len(providers) == 0 {
		return nil
	}

	orgID := "default"
	if m != nil {
		if trimmed := strings.TrimSpace(m.GetOrgID()); trimmed != "" {
			orgID = trimmed
		}
	}

	duration := end.Sub(start)
	lookback := trailingLookbackForWindow(start, end)
	history := make(map[string]map[string][]MetricPoint)
	for _, provider := range providers {
		if ctx.Err() != nil {
			return nil
		}
		if targeted, ok := provider.(MonitorTargetedGuestMetricHistoryProvider); ok && len(resourceIDs) == 1 {
			id := resourceIDs[0]
			metricMap := targeted.GuestMetricHistoryForResource(ctx, m, orgID, resourceType, id, lookback, metricTypes...)
			clipped := clipMetricMapToWindow(metricMap, start, end)
			if len(clipped) > 0 {
				history[id] = mergeGuestMetricHistory(history[id], clipped, duration)
			}
			continue
		}
		historyProvider, ok := provider.(MonitorGuestMetricHistoryProvider)
		if !ok {
			continue
		}
		nativeHistory := historyProvider.GuestMetricHistory(m, orgID, resourceType, lookback)
		for resourceID, metricMap := range nativeHistory {
			if strings.TrimSpace(resourceID) == "" || len(metricMap) == 0 {
				continue
			}
			clipped := clipMetricMapToWindow(metricMap, start, end)
			if len(clipped) == 0 {
				continue
			}
			history[resourceID] = mergeGuestMetricHistory(history[resourceID], clipped, duration)
		}
	}
	if len(history) == 0 {
		return nil
	}
	return history
}

func chartGapFillLookbackWindow(duration time.Duration) time.Duration {
	lookback := duration * chartGapFillLookbackMultiplier
	if lookback < chartGapFillLookbackMin {
		lookback = chartGapFillLookbackMin
	}
	if lookback > chartGapFillLookbackMax {
		lookback = chartGapFillLookbackMax
	}
	return lookback
}

func convertSeriesAndDownsample(points []metrics.MetricPoint, target int) []MetricPoint {
	converted := make([]MetricPoint, len(points))
	for i, p := range points {
		converted[i] = MetricPoint{Value: p.Value, Timestamp: p.Timestamp}
	}
	return lttb(converted, target)
}

func (m *Monitor) queryStoreMetricMapWithGapFill(resourceType, resourceID string, duration time.Duration) (map[string][]MetricPoint, bool) {
	result, ok, _ := m.queryStoreMetricHistory(resourceType, resourceID, duration, true)
	return result, ok
}

func (m *Monitor) queryStoreMetricHistory(resourceType, resourceID string, duration time.Duration, allowGapFill bool) (map[string][]MetricPoint, bool, error) {
	if m == nil || m.metricsStore == nil {
		return nil, false, nil
	}

	end := time.Now()
	start := end.Add(-duration)

	query := func(from time.Time) (map[string][]MetricPoint, bool, error) {
		sqlResult, err := m.metricsStore.QueryMetricValues(resourceType, resourceID, from, end, 0)
		if err != nil || len(sqlResult) == 0 {
			return nil, false, err
		}
		return downsampleMetricMap(sqlResult, chartDownsampleTarget), true, nil
	}

	if result, ok, err := query(start); ok || err != nil || !allowGapFill {
		return result, ok, err
	}

	return query(end.Add(-chartGapFillLookbackWindow(duration)))
}

func (m *Monitor) queryStoreMetricMapWithGapFillAliases(resourceType, resourceID string, duration time.Duration) (map[string][]MetricPoint, bool) {
	var (
		best       map[string][]MetricPoint
		bestSpan   time.Duration
		bestPoints int
		found      bool
	)

	for _, candidate := range monitorStoreResourceTypeCandidates(resourceType) {
		result, ok := m.queryStoreMetricMapWithGapFill(candidate, resourceID, duration)
		if !ok {
			continue
		}

		span := chartMapCoverageSpan(result)
		points := monitorMetricMapPointCount(result)
		if !found || span > bestSpan || (span == bestSpan && points > bestPoints) {
			best = result
			bestSpan = span
			bestPoints = points
			found = true
		}
	}

	if siblingKeys := m.proxmoxGuestMetricSiblingKeys(resourceType, resourceID); len(siblingKeys) > 0 {
		for _, siblingKey := range siblingKeys {
			for _, candidate := range monitorStoreResourceTypeCandidates(resourceType) {
				sibling, ok := m.queryStoreMetricMapWithGapFill(candidate, siblingKey, duration)
				if !ok || len(sibling) == 0 {
					continue
				}
				best = spliceGuestMetricHistory(best, sibling)
				found = true
			}
		}
	}

	return best, found
}

func (m *Monitor) queryStoreMetricSeriesWithGapFill(resourceType, resourceID, metricType string, duration time.Duration) ([]MetricPoint, bool) {
	if m == nil || m.metricsStore == nil {
		return nil, false
	}

	end := time.Now()
	start := end.Add(-duration)

	query := func(from time.Time) ([]MetricPoint, bool) {
		sqlPoints, err := m.metricsStore.Query(resourceType, resourceID, metricType, from, end, 0)
		if err != nil || len(sqlPoints) == 0 {
			return nil, false
		}
		return convertSeriesAndDownsample(sqlPoints, chartDownsampleTarget), true
	}

	if points, ok := query(start); ok {
		return points, true
	}

	return query(end.Add(-chartGapFillLookbackWindow(duration)))
}

// downsampleMetricMap samples detached store results at the existing chart
// target. These slices are owned by this read and never alias retained History.
func downsampleMetricMap(result map[string][]MetricPoint, target int) map[string][]MetricPoint {
	for metric, points := range result {
		result[metric] = lttb(points, target)
	}
	return result
}

func cloneMetricSeries(points []MetricPoint) []MetricPoint {
	if len(points) == 0 {
		return nil
	}
	cloned := make([]MetricPoint, len(points))
	copy(cloned, points)
	return cloned
}

func cloneMetricPointMap(metrics map[string][]MetricPoint) map[string][]MetricPoint {
	if len(metrics) == 0 {
		return map[string][]MetricPoint{}
	}
	cloned := make(map[string][]MetricPoint, len(metrics))
	for metricType, points := range metrics {
		cloned[metricType] = cloneMetricSeries(points)
	}
	return cloned
}

func mockChartPointTarget(duration time.Duration) int {
	switch {
	case duration <= time.Hour:
		return mockChartPointTargetShortRange
	case duration <= 6*time.Hour:
		return mockChartPointTargetMedium
	case duration <= 24*time.Hour:
		return mockChartPointTargetLong
	default:
		return mockChartPointTargetExtended
	}
}

// downsampleMetricSeriesForMockChart thins a mock series onto the shared mock
// chart grid (see sampleMetricSeriesOnMockChartGrid). It does not use LTTB:
// LTTB keeps the points whose values stand out, so each metric of a resource
// kept its own timestamps, and drawer history panels, which break a line
// where another series in the panel has a reading this one lacks, drew mock
// charts as scattered dots.
func downsampleMetricSeriesForMockChart(points []MetricPoint, duration time.Duration) []MetricPoint {
	return sampleMetricSeriesOnMockChartGrid(points, mockChartGridInterval(duration))
}

func downsampleMetricMapForMockChart(metrics map[string][]MetricPoint, duration time.Duration) map[string][]MetricPoint {
	if len(metrics) == 0 {
		return map[string][]MetricPoint{}
	}
	downsampled := make(map[string][]MetricPoint, len(metrics))
	for metricType, points := range metrics {
		downsampled[metricType] = downsampleMetricSeriesForMockChart(points, duration)
	}
	return downsampled
}

func normalizeMockChartMetricTypes(metricTypes []string) string {
	if len(metricTypes) == 0 {
		return ""
	}
	normalized := make([]string, 0, len(metricTypes))
	for _, metricType := range metricTypes {
		trimmed := strings.TrimSpace(metricType)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return ""
	}
	sort.Strings(normalized)
	return strings.Join(normalized, ",")
}

func (m *Monitor) readMockChartMetricMapCache(
	key mockChartMetricMapCacheKey,
) (map[string][]MetricPoint, bool) {
	if m == nil {
		return nil, false
	}

	m.mockChartCacheMu.RLock()
	defer m.mockChartCacheMu.RUnlock()

	if m.mockChartMapCache == nil {
		return nil, false
	}
	cached, ok := m.mockChartMapCache[key]
	if !ok {
		return nil, false
	}
	return cloneMetricPointMap(cached), true
}

func (m *Monitor) writeMockChartMetricMapCache(
	key mockChartMetricMapCacheKey,
	metrics map[string][]MetricPoint,
) map[string][]MetricPoint {
	cloned := cloneMetricPointMap(metrics)
	if m == nil {
		return cloned
	}

	m.mockChartCacheMu.Lock()
	defer m.mockChartCacheMu.Unlock()

	if m.mockChartMapCache == nil {
		m.mockChartMapCache = make(map[mockChartMetricMapCacheKey]map[string][]MetricPoint)
	}
	m.mockChartMapCache[key] = cloneMetricPointMap(cloned)
	return cloned
}

func (m *Monitor) invalidateMockChartCaches() {
	if m == nil {
		return
	}

	m.mockChartCacheMu.Lock()
	defer m.mockChartCacheMu.Unlock()

	if m.mockChartMapCache != nil {
		clear(m.mockChartMapCache)
	}
}

type mockWorkloadChartPrewarmRequests struct {
	vms              []GuestChartRequest
	containers       []GuestChartRequest
	pods             []GuestChartRequest
	dockerContainers []GuestChartRequest
}

func appendMockGuestChartRequest(
	requests []GuestChartRequest,
	seen map[string]struct{},
	inMemoryKey string,
	sqlResourceID string,
) []GuestChartRequest {
	inMemoryKey = strings.TrimSpace(inMemoryKey)
	sqlResourceID = strings.TrimSpace(sqlResourceID)
	if inMemoryKey == "" || sqlResourceID == "" {
		return requests
	}

	identity := inMemoryKey + "\x00" + sqlResourceID
	if _, ok := seen[identity]; ok {
		return requests
	}
	seen[identity] = struct{}{}

	return append(requests, GuestChartRequest{
		InMemoryKey:   inMemoryKey,
		SQLResourceID: sqlResourceID,
	})
}

func mockMetricIDFromTargetOrSource(target *unifiedresources.MetricsTarget, sourceID string) string {
	if target != nil {
		if metricID := strings.TrimSpace(target.ResourceID); metricID != "" {
			return metricID
		}
	}
	return strings.TrimSpace(sourceID)
}

func mockKubernetesPodChartMetricID(pod *unifiedresources.PodView) string {
	if pod == nil {
		return ""
	}

	clusterKey := strings.TrimSpace(pod.ClusterID())
	if clusterKey == "" {
		clusterKey = strings.TrimSpace(pod.ClusterName())
	}
	podKey := strings.TrimSpace(pod.PodUID())
	if podKey == "" {
		namespace := strings.TrimSpace(pod.Namespace())
		name := strings.TrimSpace(pod.Name())
		if namespace != "" || name != "" {
			podKey = namespace + "/" + name
		}
	}
	if clusterKey == "" || podKey == "" {
		return ""
	}
	return "k8s:" + clusterKey + ":pod:" + podKey
}

func mockWorkloadChartRequestsForReadState(
	readState unifiedresources.ReadState,
) mockWorkloadChartPrewarmRequests {
	if readState == nil {
		return mockWorkloadChartPrewarmRequests{}
	}

	requests := mockWorkloadChartPrewarmRequests{
		vms:              make([]GuestChartRequest, 0, len(readState.VMs())),
		containers:       make([]GuestChartRequest, 0, len(readState.Containers())),
		pods:             make([]GuestChartRequest, 0, len(readState.Pods())),
		dockerContainers: make([]GuestChartRequest, 0, len(readState.DockerContainers())),
	}
	seenVMs := make(map[string]struct{}, len(readState.VMs()))
	seenContainers := make(map[string]struct{}, len(readState.Containers()))
	seenPods := make(map[string]struct{}, len(readState.Pods()))
	seenDockerContainers := make(map[string]struct{}, len(readState.DockerContainers()))

	for _, vm := range readState.VMs() {
		if vm == nil {
			continue
		}
		metricID := mockMetricIDFromTargetOrSource(vm.MetricsTarget(), vm.SourceID())
		requests.vms = appendMockGuestChartRequest(requests.vms, seenVMs, metricID, metricID)
	}
	for _, container := range readState.Containers() {
		if container == nil {
			continue
		}
		metricID := mockMetricIDFromTargetOrSource(container.MetricsTarget(), container.SourceID())
		requests.containers = appendMockGuestChartRequest(
			requests.containers,
			seenContainers,
			metricID,
			metricID,
		)
	}
	for _, pod := range readState.Pods() {
		metricID := mockKubernetesPodChartMetricID(pod)
		requests.pods = appendMockGuestChartRequest(requests.pods, seenPods, metricID, metricID)
	}
	for _, container := range readState.DockerContainers() {
		if container == nil {
			continue
		}
		metricID := mockMetricIDFromTargetOrSource(container.MetricsTarget(), container.ContainerID())
		requests.dockerContainers = appendMockGuestChartRequest(
			requests.dockerContainers,
			seenDockerContainers,
			"docker:"+metricID,
			metricID,
		)
	}

	return requests
}

func boundMockWorkloadChartPrewarmRequests(
	requests mockWorkloadChartPrewarmRequests,
) mockWorkloadChartPrewarmRequests {
	bound := func(values []GuestChartRequest) []GuestChartRequest {
		if len(values) <= mockDashboardWorkloadPrewarmLimitPerType {
			return values
		}
		return values[:mockDashboardWorkloadPrewarmLimitPerType]
	}
	requests.vms = bound(requests.vms)
	requests.containers = bound(requests.containers)
	requests.pods = bound(requests.pods)
	requests.dockerContainers = bound(requests.dockerContainers)
	return requests
}

func (m *Monitor) prewarmMockWorkloadChartCaches(duration time.Duration) {
	if m == nil || duration <= 0 {
		return
	}

	requests := boundMockWorkloadChartPrewarmRequests(
		mockWorkloadChartRequestsForReadState(m.GetUnifiedStructureReadState()),
	)
	if len(requests.vms) > 0 {
		_ = m.GetGuestMetricsForChartBatch("vm", requests.vms, duration)
	}
	if len(requests.containers) > 0 {
		_ = m.GetGuestMetricsForChartBatch("container", requests.containers, duration)
	}
	if len(requests.pods) > 0 {
		_ = m.GetGuestMetricsForChartBatch("k8s", requests.pods, duration)
	}
	if len(requests.dockerContainers) > 0 {
		_ = m.GetGuestMetricsForChartBatch("dockerContainer", requests.dockerContainers, duration)
	}
}

func (m *Monitor) prewarmMockDashboardChartCaches() {
	if m == nil || !mock.IsMockEnabled() {
		return
	}

	m.prewarmMockWorkloadChartCaches(mockDashboardWorkloadPrewarmDuration)
}

func (m *Monitor) mockGuestMetricsForChart(
	inMemoryKey,
	sqlResourceType,
	sqlResourceID string,
	duration time.Duration,
	inMemoryResult map[string][]MetricPoint,
) map[string][]MetricPoint {
	key := mockChartMetricMapCacheKey{
		kind:         "guest",
		resourceType: strings.TrimSpace(sqlResourceType),
		resourceID:   strings.TrimSpace(sqlResourceID),
		aux:          strings.TrimSpace(inMemoryKey),
		duration:     duration,
	}
	if cached, ok := m.readMockChartMetricMapCache(key); ok {
		return cached
	}

	// Resolve mock coverage per series, not per resource. A resource can have a
	// full window of CPU/memory/disk while I/O or thermal series are absent; a
	// resource-level coverage check would then return the partial map unchanged
	// and leave whole drawer groups stuck in "Collecting history".
	synthetic := mockGuestMetricsForChart(sqlResourceType, sqlResourceID, duration)
	computed := make(map[string][]MetricPoint, len(inMemoryResult)+len(synthetic))
	for metricType, points := range inMemoryResult {
		if hasSufficientChartSeriesCoverage(points, duration) {
			computed[metricType] = downsampleMetricSeriesForMockChart(points, duration)
			continue
		}
		if fallback := synthetic[metricType]; len(fallback) > 0 {
			computed[metricType] = downsampleMetricSeriesForMockChart(fallback, duration)
			continue
		}
		computed[metricType] = downsampleMetricSeriesForMockChart(points, duration)
	}
	for metricType, points := range synthetic {
		if len(computed[metricType]) > 0 {
			continue
		}
		computed[metricType] = downsampleMetricSeriesForMockChart(points, duration)
	}
	return m.writeMockChartMetricMapCache(key, computed)
}

func (m *Monitor) mockNodeMetricsForChart(
	nodeID string,
	metricType string,
	duration time.Duration,
	inMemoryPoints []MetricPoint,
) []MetricPoint {
	cacheKey := mockChartMetricMapCacheKey{
		kind:         "node",
		resourceType: "node",
		resourceID:   strings.TrimSpace(nodeID),
		aux:          normalizeMockChartMetricTypes([]string{metricType}),
		duration:     duration,
	}
	if cached, ok := m.readMockChartMetricMapCache(cacheKey); ok {
		return cloneMetricSeries(cached[metricType])
	}

	computed := map[string][]MetricPoint{}
	if hasSufficientChartSeriesCoverage(inMemoryPoints, duration) {
		computed[metricType] = downsampleMetricSeriesForMockChart(inMemoryPoints, duration)
	} else {
		computed[metricType] = downsampleMetricSeriesForMockChart(
			mockNodeMetricsForChart(nodeID, []string{metricType}, duration)[metricType],
			duration,
		)
	}
	return cloneMetricSeries(m.writeMockChartMetricMapCache(cacheKey, computed)[metricType])
}

func (m *Monitor) mockDiskMetricsForChart(
	resourceID string,
	metricType string,
	duration time.Duration,
	inMemoryPoints []MetricPoint,
) []MetricPoint {
	cacheKey := mockChartMetricMapCacheKey{
		kind:         "disk",
		resourceType: "disk",
		resourceID:   strings.TrimSpace(resourceID),
		aux:          normalizeMockChartMetricTypes([]string{metricType}),
		duration:     duration,
	}
	if cached, ok := m.readMockChartMetricMapCache(cacheKey); ok {
		return cloneMetricSeries(cached[metricType])
	}

	computed := map[string][]MetricPoint{}
	if hasSufficientChartSeriesCoverage(inMemoryPoints, duration) {
		computed[metricType] = downsampleMetricSeriesForMockChart(inMemoryPoints, duration)
	} else {
		computed[metricType] = downsampleMetricSeriesForMockChart(
			mockDiskMetricsForChart(resourceID, []string{metricType}, duration)[metricType],
			duration,
		)
	}
	return cloneMetricSeries(m.writeMockChartMetricMapCache(cacheKey, computed)[metricType])
}

func (m *Monitor) mockStorageMetricsForChartCached(
	storageID string,
	duration time.Duration,
	inMemoryResult map[string][]MetricPoint,
	readState func() unifiedresources.ReadState,
) map[string][]MetricPoint {
	cacheKey := mockChartMetricMapCacheKey{
		kind:         "storage",
		resourceType: "storage",
		resourceID:   strings.TrimSpace(storageID),
		duration:     duration,
	}
	if cached, ok := m.readMockChartMetricMapCache(cacheKey); ok {
		return cached
	}

	var computed map[string][]MetricPoint
	if hasSufficientChartMapCoverageForMetrics(inMemoryResult, duration, storageChartMetricTypes) {
		computed = downsampleMetricMapForMockChart(inMemoryResult, duration)
	} else {
		computed = downsampleMetricMapForMockChart(
			m.mockStorageMetricsForChart(storageID, duration, inMemoryResult, readState),
			duration,
		)
	}
	return m.writeMockChartMetricMapCache(cacheKey, computed)
}

func mergeGuestMetricHistory(base, candidate map[string][]MetricPoint, duration time.Duration) map[string][]MetricPoint {
	return mergeMetricHistory(base, candidate, duration)
}

// mergeMetricHistory selects the better series per metric between base and
// candidate and returns the merged map.
//
// Ownership: caller transfers ownership of both maps (and their slice values)
// to this function. The returned map reuses base's storage when present and
// may share slices with candidate. Callers must not retain references to
// base or candidate after this call, and must not pass shared/borrowed maps.
func mergeMetricHistory(base, candidate map[string][]MetricPoint, duration time.Duration) map[string][]MetricPoint {
	if len(candidate) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string][]MetricPoint, len(candidate))
	}
	for metricType, candidateSeries := range candidate {
		if !shouldPreferMetricSeries(base[metricType], candidateSeries, duration) {
			continue
		}
		base[metricType] = candidateSeries
	}
	return base
}

func shouldPreferMetricSeries(current, candidate []MetricPoint, duration time.Duration) bool {
	if len(candidate) == 0 {
		return false
	}
	if len(current) == 0 {
		return true
	}

	candidateSpan := chartSeriesCoverageSpan(candidate)
	currentSpan := chartSeriesCoverageSpan(current)
	currentSufficient := hasSufficientChartSeriesCoverage(current, duration)
	candidateSufficient := hasSufficientChartSeriesCoverage(candidate, duration)
	switch {
	case currentSufficient && !candidateSufficient:
		return false
	case !currentSufficient && candidateSufficient:
		return true
	}

	if candidateSpan > currentSpan {
		return true
	}
	if candidateSpan < currentSpan {
		return false
	}
	return len(candidate) > len(current)
}

func chartCoverageRequiredSpan(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 0
	}

	required := (duration * shortRangeCoverageRatioNum) / shortRangeCoverageRatioDen
	slack := duration / 10
	if slack > shortRangeCoverageMaxSlack {
		slack = shortRangeCoverageMaxSlack
	}
	if required <= slack {
		return 0
	}
	return required - slack
}

func chartSeriesCoverageSpan(points []MetricPoint) time.Duration {
	if len(points) < 2 {
		return 0
	}

	oldest := points[0].Timestamp
	newest := points[0].Timestamp
	for i := 1; i < len(points); i++ {
		ts := points[i].Timestamp
		if ts.Before(oldest) {
			oldest = ts
		}
		if ts.After(newest) {
			newest = ts
		}
	}
	if !newest.After(oldest) {
		return 0
	}
	return newest.Sub(oldest)
}

func chartMapCoverageSpan(metrics map[string][]MetricPoint) time.Duration {
	var best time.Duration
	for _, points := range metrics {
		span := chartSeriesCoverageSpan(points)
		if span > best {
			best = span
		}
	}
	return best
}

func monitorMetricMapPointCount(metrics map[string][]MetricPoint) int {
	total := 0
	for _, points := range metrics {
		total += len(points)
	}
	return total
}

func monitorGuestMetricKeyCandidates(key string) []string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return []string{""}
	}
	return []string{trimmed}
}

// proxmoxGuestMetricSiblingKeys returns sibling node metric keys
// ("instance:otherNode:vmid") for a Proxmox guest so chart reads survive live
// migration across cluster nodes. If Proxmox VE 9.3 persistent guest identity
// detected VMID reuse on this slot (a UID-separated canonical ID), sibling
// history is withheld so a replacement guest never inherits the deleted guest's
// charts.
func (m *Monitor) proxmoxGuestMetricSiblingKeys(resourceType, resourceID string) []string {
	if m == nil {
		return nil
	}
	normalizedType := strings.TrimSpace(resourceType)
	if normalizedType != "" && normalizedType != "vm" && normalizedType != "container" {
		return nil
	}
	instance, currentNode, vmid, ok := unifiedresources.ParseProxmoxGuestSourceID(resourceID)
	if !ok {
		return nil
	}
	readState := m.GetUnifiedReadState()
	if readState == nil {
		return nil
	}

	if normalizedType == "" || normalizedType == "vm" {
		expectedID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, instance, vmid)
		for _, vm := range readState.VMs() {
			if vm == nil || vm.VMID() != vmid || !strings.EqualFold(vm.Instance(), instance) {
				continue
			}
			if expectedID != "" && vm.ID() != expectedID {
				return nil
			}
		}
	}
	if normalizedType == "" || normalizedType == "container" {
		expectedID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, instance, vmid)
		for _, ct := range readState.Containers() {
			if ct == nil || ct.VMID() != vmid || !strings.EqualFold(ct.Instance(), instance) {
				continue
			}
			if expectedID != "" && ct.ID() != expectedID {
				return nil
			}
		}
	}

	seen := make(map[string]struct{})
	siblings := make([]string, 0)
	for _, node := range readState.Nodes() {
		if node == nil || !strings.EqualFold(node.Instance(), instance) {
			continue
		}
		nodeName := strings.TrimSpace(node.NodeName())
		if nodeName == "" || strings.EqualFold(nodeName, currentNode) {
			continue
		}
		siblingKey := makeGuestID(instance, nodeName, vmid)
		if _, dup := seen[siblingKey]; dup {
			continue
		}
		seen[siblingKey] = struct{}{}
		siblings = append(siblings, siblingKey)
	}
	sort.Strings(siblings)
	return siblings
}

// spliceGuestMetricHistory merges chronological points from a guest's prior
// cluster node (sibling) with its current node (primary), preferring primary on
// identical timestamps.
func spliceGuestMetricHistory(primary, sibling map[string][]MetricPoint) map[string][]MetricPoint {
	if len(sibling) == 0 {
		return primary
	}
	if len(primary) == 0 {
		return cloneMetricPointMap(sibling)
	}
	out := cloneMetricPointMap(primary)
	for metricType, sibSeries := range sibling {
		if len(sibSeries) == 0 {
			continue
		}
		primSeries := out[metricType]
		if len(primSeries) == 0 {
			out[metricType] = cloneMetricSeries(sibSeries)
			continue
		}
		out[metricType] = spliceMetricPointSeries(primSeries, sibSeries)
	}
	return out
}

func spliceMetricPointSeries(primary, sibling []MetricPoint) []MetricPoint {
	byTimestamp := make(map[int64]MetricPoint, len(primary)+len(sibling))
	for _, pt := range sibling {
		byTimestamp[pt.Timestamp.UnixNano()] = pt
	}
	for _, pt := range primary {
		byTimestamp[pt.Timestamp.UnixNano()] = pt
	}
	merged := make([]MetricPoint, 0, len(byTimestamp))
	for _, pt := range byTimestamp {
		merged = append(merged, pt)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Timestamp.Before(merged[j].Timestamp)
	})
	return merged
}

func monitorStoreResourceTypeCandidates(resourceType string) []string {
	normalized := strings.TrimSpace(resourceType)
	switch normalized {
	case "agent":
		return []string{"agent"}
	case "dockerContainer":
		return []string{"dockerContainer", "docker"}
	case "docker":
		return []string{"docker", "dockerContainer"}
	default:
		return []string{normalized}
	}
}

func hasSufficientChartSeriesCoverage(points []MetricPoint, duration time.Duration) bool {
	return chartSeriesCoverageSpan(points) >= chartCoverageRequiredSpan(duration)
}

func hasSufficientChartMapCoverage(metrics map[string][]MetricPoint, duration time.Duration) bool {
	return chartMapCoverageSpan(metrics) >= chartCoverageRequiredSpan(duration)
}

func hasSufficientChartMapCoverageForMetrics(metrics map[string][]MetricPoint, duration time.Duration, metricTypes []string) bool {
	if len(metricTypes) == 0 {
		return hasSufficientChartMapCoverage(metrics, duration)
	}
	for _, metricType := range metricTypes {
		if !hasSufficientChartSeriesCoverage(metrics[metricType], duration) {
			return false
		}
	}
	return true
}
