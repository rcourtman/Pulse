package monitoring

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
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

var storageChartMetricTypes = []string{"usage", "used", "avail", "total"}
var storageSummaryChartMetricTypes = []string{"used", "avail"}

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

	return best
}

// GetGuestMetricsForChart returns guest metrics optimised for chart display.
// Short ranges are served from the in-memory ring buffer; longer ranges fall
// back to the persistent SQLite store with LTTB downsampling.
//
// inMemoryKey is the key used in the in-memory buffer (e.g. "docker:abc123").
// sqlResourceType/sqlResourceID are the type/id used in the SQLite store
// (e.g. "dockerContainer"/"abc123").
func (m *Monitor) GetGuestMetricsForChart(inMemoryKey, sqlResourceType, sqlResourceID string, duration time.Duration) map[string][]MetricPoint {
	inMemoryResult := m.GetGuestMetrics(inMemoryKey, duration)
	if mock.IsMockEnabled() {
		return m.mockGuestMetricsForChart(inMemoryKey, sqlResourceType, sqlResourceID, duration, inMemoryResult)
	}

	if hasSufficientChartMapCoverage(inMemoryResult, duration) {
		return inMemoryResult
	}

	best := cloneMetricPointMap(inMemoryResult)
	if m.metricsStore != nil {
		if converted, ok := m.queryStoreMetricMapWithGapFillAliases(sqlResourceType, sqlResourceID, duration); ok {
			best = mergeGuestMetricHistory(best, converted, duration)
		}
	}
	if nativeHistory := m.nativeGuestMetricHistory(sqlResourceType, duration); len(nativeHistory) > 0 {
		if nativeMetrics, ok := nativeHistory[sqlResourceID]; ok {
			best = mergeGuestMetricHistory(best, nativeMetrics, duration)
		}
	}
	return best
}

// GetNodeMetrics returns historical metrics for a node
func (m *Monitor) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
}

// GetNodeMetricsForChart returns node metrics for a single metric type,
// falling back to SQLite + LTTB for longer ranges.
func (m *Monitor) GetNodeMetricsForChart(nodeID, metricType string, duration time.Duration) []MetricPoint {
	var inMemoryPoints []MetricPoint
	if m != nil && m.metricsHistory != nil {
		inMemoryPoints = m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
	}

	if mock.IsMockEnabled() {
		return m.mockNodeMetricsForChart(nodeID, metricType, duration, inMemoryPoints)
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

// GetDiskMetrics returns historical metrics for a physical disk.
func (m *Monitor) GetDiskMetrics(resourceID string, metricType string, duration time.Duration) []MetricPoint {
	if m == nil || m.metricsHistory == nil {
		return nil
	}
	return m.metricsHistory.GetDiskMetrics(resourceID, metricType, duration)
}

// GetDiskMetricsForChart returns physical-disk metrics optimized for chart
// display, preferring in-memory data for freshness and falling back to the
// persistent store when coverage is shallow.
func (m *Monitor) GetDiskMetricsForChart(resourceID string, metricType string, duration time.Duration) []MetricPoint {
	inMemoryPoints := m.GetDiskMetrics(resourceID, metricType, duration)
	if mock.IsMockEnabled() {
		return m.mockDiskMetricsForChart(resourceID, metricType, duration, inMemoryPoints)
	}

	if hasSufficientChartSeriesCoverage(inMemoryPoints, duration) {
		return inMemoryPoints
	}

	best := cloneMetricSeries(inMemoryPoints)
	if converted, ok := m.queryStoreMetricMapWithGapFill("disk", resourceID, duration); ok {
		if candidate := converted[metricType]; shouldPreferMetricSeries(best, candidate, duration) {
			best = cloneMetricSeries(candidate)
		}
	}

	return best
}

// GetStorageMetricsForChart returns storage metrics, falling back to SQLite +
// LTTB for longer ranges.
func (m *Monitor) GetStorageMetricsForChart(storageID string, duration time.Duration) map[string][]MetricPoint {
	inMemoryResult := map[string][]MetricPoint{}
	if m != nil && m.metricsHistory != nil {
		inMemoryResult = m.metricsHistory.GetAllStorageMetrics(storageID, duration)
	}
	if mock.IsMockEnabled() {
		return m.mockStorageMetricsForChartCached(storageID, duration, inMemoryResult)
	}
	if m.metricsStore == nil {
		return inMemoryResult
	}
	if hasSufficientChartMapCoverageForMetrics(inMemoryResult, duration, storageChartMetricTypes) {
		return inMemoryResult
	}

	best := cloneMetricPointMap(inMemoryResult)
	if converted, ok := m.queryStoreMetricMapWithGapFill("storage", storageID, duration); ok {
		best = mergeMetricHistory(best, converted, duration)
	}
	return best
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
		resourceID  string
		name        string
		node        string
		instance    string
		temperature int
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
			resourceID:  resourceID,
			name:        name,
			node:        strings.TrimSpace(disk.Node()),
			instance:    strings.TrimSpace(disk.Instance()),
			temperature: disk.Temperature(),
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

		// Sparklines require >= 2 points. If the store returned 0 or 1 points
		// but the disk has a live temperature reading, pad to 2 points so the
		// chart can render (flat line at current temperature).
		if len(tempPoints) < 2 {
			now := time.Now()
			tempPoints = []MetricPoint{
				{Timestamp: now.Add(-60 * time.Second), Value: float64(d.temperature)},
				{Timestamp: now, Value: float64(d.temperature)},
			}
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
		sqlResult, err := m.metricsStore.QueryMetricTypesBatch(resourceType, ids, metricTypes, from, end, 0)
		if err != nil || len(sqlResult) == 0 {
			return nil
		}
		// Convert metrics.MetricPoint → monitoring.MetricPoint and downsample.
		converted := make(map[string]map[string][]MetricPoint, len(sqlResult))
		for resID, metricMap := range sqlResult {
			converted[resID] = convertAndDownsample(metricMap, chartDownsampleTarget)
		}
		return converted
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

// GetGuestMetricsForChartBatch returns chart metrics for multiple guests of the
// same SQL resource type, using batch SQL queries instead of N individual
// queries. Results are keyed by SQLResourceID.
func (m *Monitor) GetGuestMetricsForChartBatch(
	sqlResourceType string,
	requests []GuestChartRequest,
	duration time.Duration,
) map[string]map[string][]MetricPoint {
	if m == nil || len(requests) == 0 {
		return nil
	}

	if mock.IsMockEnabled() {
		result := make(map[string]map[string][]MetricPoint, len(requests))
		for _, req := range requests {
			inMemory := m.GetGuestMetrics(req.InMemoryKey, duration)
			result[req.SQLResourceID] = m.mockGuestMetricsForChart(
				req.InMemoryKey,
				sqlResourceType,
				req.SQLResourceID,
				duration,
				inMemory,
			)
		}
		return result
	}

	result := make(map[string]map[string][]MetricPoint, len(requests))

	// Phase 1: Check in-memory for all guests and identify which need fallback.
	var needFallback []string
	for _, req := range requests {
		inMemory := m.GetGuestMetrics(req.InMemoryKey, duration)
		result[req.SQLResourceID] = inMemory
		if hasSufficientChartMapCoverage(inMemory, duration) {
			result[req.SQLResourceID] = inMemory
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
		for _, candidate := range monitorStoreResourceTypeCandidates(sqlResourceType) {
			batch := m.queryStoreBatchMetricMapWithGapFill(candidate, needFallback, duration, nil)
			for id, metricMap := range batch {
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
	nativeResults := m.nativeGuestMetricHistory(sqlResourceType, duration)

	// Phase 3: Merge fallback data per metric type.
	for _, id := range needFallback {
		best := cloneMetricPointMap(result[id])
		storeData, ok := storeResults[id]
		if ok {
			best = mergeGuestMetricHistory(best, storeData, duration)
		}
		nativeData, ok := nativeResults[id]
		if !ok {
			result[id] = best
			continue
		}
		result[id] = mergeGuestMetricHistory(best, nativeData, duration)
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
		for _, mt := range metricTypes {
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
		for _, sid := range storageIDs {
			inMemory := map[string][]MetricPoint{}
			if m.metricsHistory != nil {
				inMemory = m.metricsHistory.GetAllStorageMetrics(sid, duration)
			}
			result[sid] = m.mockStorageMetricsForChartCached(sid, duration, inMemory)
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

// GetStorageCapacityMetricsForSummaryBatch returns only the canonical capacity
// metrics required for the compact dashboard storage summary card.
func (m *Monitor) GetStorageCapacityMetricsForSummaryBatch(
	storageIDs []string,
	duration time.Duration,
) map[string]map[string][]MetricPoint {
	if m == nil || len(storageIDs) == 0 {
		return nil
	}

	result := make(map[string]map[string][]MetricPoint, len(storageIDs))

	if mock.IsMockEnabled() {
		for _, sid := range storageIDs {
			inMemory := map[string][]MetricPoint{}
			if m.metricsHistory != nil {
				inMemory = filterMetricPointMap(
					m.metricsHistory.GetAllStorageMetrics(sid, duration),
					storageSummaryChartMetricTypes,
				)
			}
			fullMetrics := m.mockStorageMetricsForChartCached(sid, duration, inMemory)
			result[sid] = filterMetricPointMap(fullMetrics, storageSummaryChartMetricTypes)
		}
		return result
	}

	var needStore []string
	for _, sid := range storageIDs {
		inMemory := map[string][]MetricPoint{}
		if m.metricsHistory != nil {
			inMemory = filterMetricPointMap(
				m.metricsHistory.GetAllStorageMetrics(sid, duration),
				storageSummaryChartMetricTypes,
			)
		}
		if m.metricsStore == nil {
			result[sid] = inMemory
			continue
		}
		if hasSufficientChartMapCoverageForMetrics(inMemory, duration, storageSummaryChartMetricTypes) {
			result[sid] = inMemory
			continue
		}
		needStore = append(needStore, sid)
		result[sid] = inMemory
	}

	if len(needStore) == 0 {
		return result
	}

	batchResult := m.queryStoreBatchMetricMapWithGapFill(
		"storage",
		needStore,
		duration,
		storageSummaryChartMetricTypes,
	)
	for _, sid := range needStore {
		storeData, ok := batchResult[sid]
		if !ok {
			continue
		}
		result[sid] = mergeMetricHistory(result[sid], storeData, duration)
	}

	return result
}

func (m *Monitor) GetStorageSummaryCapacityTrend(duration time.Duration) ([]MetricPoint, int64) {
	if m == nil {
		return nil, 0
	}

	if mock.IsMockEnabled() {
		return m.mockStorageSummaryCapacityTrendCached(duration)
	}

	readState := m.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return nil, 0
	}

	storageIDs := make([]string, 0, len(readState.StoragePools()))
	for _, pool := range readState.StoragePools() {
		if pool == nil {
			continue
		}

		storageID := strings.TrimSpace(pool.SourceID())
		if storageID == "" {
			continue
		}
		storageIDs = append(storageIDs, storageID)
	}

	return buildStorageSummaryCapacityTrend(
		m.GetStorageCapacityMetricsForSummaryBatch(storageIDs, duration),
	)
}

func (m *Monitor) nativeGuestMetricHistory(resourceType string, duration time.Duration) map[string]map[string][]MetricPoint {
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
	if m == nil || m.metricsStore == nil {
		return nil, false
	}

	end := time.Now()
	start := end.Add(-duration)

	query := func(from time.Time) (map[string][]MetricPoint, bool) {
		sqlResult, err := m.metricsStore.QueryAll(resourceType, resourceID, from, end, 0)
		if err != nil || len(sqlResult) == 0 {
			return nil, false
		}
		return convertAndDownsample(sqlResult, chartDownsampleTarget), true
	}

	if result, ok := query(start); ok {
		return result, true
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

// convertAndDownsample converts pkg/metrics.MetricPoint slices to
// internal/monitoring.MetricPoint slices and applies LTTB downsampling.
func convertAndDownsample(sqlResult map[string][]metrics.MetricPoint, target int) map[string][]MetricPoint {
	result := make(map[string][]MetricPoint, len(sqlResult))
	for metric, points := range sqlResult {
		converted := make([]MetricPoint, len(points))
		for i, p := range points {
			converted[i] = MetricPoint{Value: p.Value, Timestamp: p.Timestamp}
		}
		result[metric] = lttb(converted, target)
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

func downsampleMetricSeriesForMockChart(points []MetricPoint, duration time.Duration) []MetricPoint {
	target := mockChartPointTarget(duration)
	return lttb(cloneMetricSeries(points), target)
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

func (m *Monitor) prewarmMockDashboardChartCaches() {
	if m == nil || !mock.IsMockEnabled() {
		return
	}

	_, _ = m.mockStorageSummaryCapacityTrendCached(24 * time.Hour)
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

	var computed map[string][]MetricPoint
	switch {
	case hasSufficientChartMapCoverage(inMemoryResult, duration):
		computed = downsampleMetricMapForMockChart(inMemoryResult, duration)
	case len(sqlResourceID) > 0:
		if synthetic := mockGuestMetricsForChart(sqlResourceType, sqlResourceID, duration); len(synthetic) > 0 {
			computed = downsampleMetricMapForMockChart(synthetic, duration)
		}
	}
	if computed == nil {
		computed = downsampleMetricMapForMockChart(inMemoryResult, duration)
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
			m.mockStorageMetricsForChart(storageID, duration, inMemoryResult),
			duration,
		)
	}
	return m.writeMockChartMetricMapCache(cacheKey, computed)
}

func (m *Monitor) mockStorageSummaryCapacityTrendCached(duration time.Duration) ([]MetricPoint, int64) {
	cacheKey := mockChartMetricMapCacheKey{
		kind:         "storage-summary",
		resourceType: "storage",
		resourceID:   "__aggregate__",
		duration:     duration,
	}
	if cached, ok := m.readMockChartMetricMapCache(cacheKey); ok {
		return cached["capacity"], oldestMetricSeriesTimestamp(cached["capacity"])
	}

	computed := m.mockStorageSummaryCapacityTrend(duration)
	cached := m.writeMockChartMetricMapCache(cacheKey, map[string][]MetricPoint{
		"capacity": computed,
	})
	return cached["capacity"], oldestMetricSeriesTimestamp(cached["capacity"])
}

func mergeGuestMetricHistory(base, candidate map[string][]MetricPoint, duration time.Duration) map[string][]MetricPoint {
	return mergeMetricHistory(base, candidate, duration)
}

func mergeMetricHistory(base, candidate map[string][]MetricPoint, duration time.Duration) map[string][]MetricPoint {
	if len(candidate) == 0 {
		return cloneMetricPointMap(base)
	}

	merged := cloneMetricPointMap(base)
	for metricType, candidateSeries := range candidate {
		if !shouldPreferMetricSeries(merged[metricType], candidateSeries, duration) {
			continue
		}
		merged[metricType] = cloneMetricSeries(candidateSeries)
	}
	return merged
}

func filterMetricPointMap(metricMap map[string][]MetricPoint, metricTypes []string) map[string][]MetricPoint {
	if len(metricTypes) == 0 {
		return cloneMetricPointMap(metricMap)
	}
	filtered := make(map[string][]MetricPoint, len(metricTypes))
	for _, metricType := range metricTypes {
		if len(metricMap[metricType]) == 0 {
			continue
		}
		filtered[metricType] = cloneMetricSeries(metricMap[metricType])
	}
	return filtered
}

func buildStorageSummaryCapacityTrend(
	poolMetrics map[string]map[string][]MetricPoint,
) ([]MetricPoint, int64) {
	type aggregateBucket struct {
		used     float64
		avail    float64
		hasUsed  bool
		hasAvail bool
	}

	buckets := make(map[int64]*aggregateBucket)
	var oldestTimestamp int64
	for _, metrics := range poolMetrics {
		for _, point := range metrics["used"] {
			timestamp := point.Timestamp.UnixMilli()
			bucket := buckets[timestamp]
			if bucket == nil {
				bucket = &aggregateBucket{}
				buckets[timestamp] = bucket
			}
			bucket.used += point.Value
			bucket.hasUsed = true
			if oldestTimestamp == 0 || timestamp < oldestTimestamp {
				oldestTimestamp = timestamp
			}
		}
		for _, point := range metrics["avail"] {
			timestamp := point.Timestamp.UnixMilli()
			bucket := buckets[timestamp]
			if bucket == nil {
				bucket = &aggregateBucket{}
				buckets[timestamp] = bucket
			}
			bucket.avail += point.Value
			bucket.hasAvail = true
			if oldestTimestamp == 0 || timestamp < oldestTimestamp {
				oldestTimestamp = timestamp
			}
		}
	}

	if len(buckets) == 0 {
		return nil, oldestTimestamp
	}

	timestamps := make([]int64, 0, len(buckets))
	for timestamp := range buckets {
		timestamps = append(timestamps, timestamp)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})

	out := make([]MetricPoint, 0, len(timestamps))
	for _, timestamp := range timestamps {
		bucket := buckets[timestamp]
		if bucket == nil || !bucket.hasUsed || !bucket.hasAvail {
			continue
		}
		total := bucket.used + bucket.avail
		if math.IsNaN(total) || math.IsInf(total, 0) || total <= 0 {
			continue
		}
		out = append(out, MetricPoint{
			Timestamp: time.UnixMilli(timestamp),
			Value:     (bucket.used / total) * 100,
		})
	}

	return out, oldestTimestamp
}

func oldestMetricSeriesTimestamp(points []MetricPoint) int64 {
	var oldest int64
	for _, point := range points {
		timestamp := point.Timestamp.UnixMilli()
		if oldest == 0 || timestamp < oldest {
			oldest = timestamp
		}
	}
	return oldest
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
