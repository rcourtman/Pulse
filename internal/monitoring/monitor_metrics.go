package monitoring

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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
	if m.metricsStore == nil {
		return inMemoryResult
	}
	if duration <= inMemoryChartThreshold && hasSufficientChartMapCoverage(inMemoryResult, duration) {
		return inMemoryResult
	}

	converted, ok := m.queryStoreMetricMapWithGapFillAliases(sqlResourceType, sqlResourceID, duration)
	if !ok {
		return inMemoryResult
	}
	if duration <= inMemoryChartThreshold && chartMapCoverageSpan(converted) <= chartMapCoverageSpan(inMemoryResult) {
		return inMemoryResult
	}
	return converted
}

// GetNodeMetrics returns historical metrics for a node
func (m *Monitor) GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint {
	return m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
}

// GetNodeMetricsForChart returns node metrics for a single metric type,
// falling back to SQLite + LTTB for longer ranges.
func (m *Monitor) GetNodeMetricsForChart(nodeID, metricType string, duration time.Duration) []MetricPoint {
	inMemoryPoints := m.metricsHistory.GetNodeMetrics(nodeID, metricType, duration)
	if m.metricsStore == nil {
		return inMemoryPoints
	}
	if duration <= inMemoryChartThreshold && hasSufficientChartSeriesCoverage(inMemoryPoints, duration) {
		return inMemoryPoints
	}

	downsampled, ok := m.queryStoreMetricSeriesWithGapFill("node", nodeID, metricType, duration)
	if !ok {
		return inMemoryPoints
	}
	if duration <= inMemoryChartThreshold && chartSeriesCoverageSpan(downsampled) <= chartSeriesCoverageSpan(inMemoryPoints) {
		return inMemoryPoints
	}
	return downsampled
}

// GetStorageMetrics returns historical metrics for storage
func (m *Monitor) GetStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint {
	return m.metricsHistory.GetAllStorageMetrics(storageID, duration)
}

// GetStorageMetricsForChart returns storage metrics, falling back to SQLite +
// LTTB for longer ranges.
func (m *Monitor) GetStorageMetricsForChart(storageID string, duration time.Duration) map[string][]MetricPoint {
	inMemoryResult := m.metricsHistory.GetAllStorageMetrics(storageID, duration)
	if m.metricsStore == nil {
		return inMemoryResult
	}
	if duration <= inMemoryChartThreshold && hasSufficientChartMapCoverage(inMemoryResult, duration) {
		return inMemoryResult
	}

	converted, ok := m.queryStoreMetricMapWithGapFill("storage", storageID, duration)
	if !ok {
		return inMemoryResult
	}
	if duration <= inMemoryChartThreshold && chartMapCoverageSpan(converted) <= chartMapCoverageSpan(inMemoryResult) {
		return inMemoryResult
	}
	return converted
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

	state := m.GetState()

	// Phase 1: Collect disk metadata and resource IDs.
	type diskMeta struct {
		resourceID  string
		name        string
		node        string
		instance    string
		temperature int
	}
	var disks []diskMeta
	for _, disk := range state.PhysicalDisks {
		if disk.Temperature <= 0 {
			continue
		}
		resourceID := unifiedresources.PhysicalDiskMetricID(disk)
		if resourceID == "" {
			continue
		}
		name := disk.Model
		if name == "" {
			name = disk.DevPath
		}
		disks = append(disks, diskMeta{
			resourceID:  resourceID,
			name:        name,
			node:        disk.Node,
			instance:    disk.Instance,
			temperature: disk.Temperature,
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
	batchMetrics := m.queryStoreBatchMetricMapWithGapFill("disk", resourceIDs, duration)

	// Phase 3: Build result entries.
	result := make(map[string]DiskChartEntry, len(disks))
	for _, d := range disks {
		var tempPoints []MetricPoint
		if resMetrics, ok := batchMetrics[d.resourceID]; ok {
			if pts, found := resMetrics["smart_temp"]; found {
				tempPoints = pts
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

// queryStoreBatchMetricMapWithGapFill queries metrics for multiple resource IDs
// of the same type in a single batch. If some resources have no data in the
// primary window, re-queries those with a broader lookback window.
// Returns map[resourceID]map[metricType][]MetricPoint (already downsampled).
func (m *Monitor) queryStoreBatchMetricMapWithGapFill(resourceType string, resourceIDs []string, duration time.Duration) map[string]map[string][]MetricPoint {
	if m == nil || m.metricsStore == nil || len(resourceIDs) == 0 {
		return nil
	}

	end := time.Now()
	start := end.Add(-duration)

	// queryBatch fetches and downsamples metrics for the given IDs.
	queryBatch := func(ids []string, from time.Time) map[string]map[string][]MetricPoint {
		sqlResult, err := m.metricsStore.QueryAllBatch(resourceType, ids, from, end, 0)
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

	result := make(map[string]map[string][]MetricPoint, len(requests))

	// Phase 1: Check in-memory for all guests.
	var needStore []string
	for _, req := range requests {
		inMemory := m.GetGuestMetrics(req.InMemoryKey, duration)
		if m.metricsStore == nil {
			result[req.SQLResourceID] = inMemory
			continue
		}
		if duration <= inMemoryChartThreshold && hasSufficientChartMapCoverage(inMemory, duration) {
			result[req.SQLResourceID] = inMemory
			continue
		}
		needStore = append(needStore, req.SQLResourceID)
		result[req.SQLResourceID] = inMemory // keep as fallback
	}

	if len(needStore) == 0 {
		return result
	}

	// Phase 2: Batch-query store, trying all resource type candidates and
	// keeping the best coverage per ID. Ties on span are broken by total
	// point count, matching the single-resource alias resolution logic.
	storeResults := make(map[string]map[string][]MetricPoint)
	for _, candidate := range monitorStoreResourceTypeCandidates(sqlResourceType) {
		batch := m.queryStoreBatchMetricMapWithGapFill(candidate, needStore, duration)
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

	// Phase 3: Merge — use store data when it has better coverage.
	for _, id := range needStore {
		storeData, ok := storeResults[id]
		if !ok {
			continue
		}
		inMemory := result[id]
		if duration <= inMemoryChartThreshold && chartMapCoverageSpan(storeData) <= chartMapCoverageSpan(inMemory) {
			continue
		}
		result[id] = storeData
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

	result := make(map[string]map[string][]MetricPoint, len(nodeIDs))

	// Phase 1: Check in-memory for all nodes and identify which need store.
	var needStore []string
	for _, nid := range nodeIDs {
		nodeResult := make(map[string][]MetricPoint, len(metricTypes))
		allSufficient := true
		for _, mt := range metricTypes {
			points := m.metricsHistory.GetNodeMetrics(nid, mt, duration)
			nodeResult[mt] = points
			if m.metricsStore != nil && (duration > inMemoryChartThreshold || !hasSufficientChartSeriesCoverage(points, duration)) {
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
	batchResult := m.queryStoreBatchMetricMapWithGapFill("node", needStore, duration)

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
			if duration <= inMemoryChartThreshold && chartSeriesCoverageSpan(storePoints) <= chartSeriesCoverageSpan(inMemory) {
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

	// Phase 1: Check in-memory for all storage pools.
	var needStore []string
	for _, sid := range storageIDs {
		inMemory := m.metricsHistory.GetAllStorageMetrics(sid, duration)
		if m.metricsStore == nil {
			result[sid] = inMemory
			continue
		}
		if duration <= inMemoryChartThreshold && hasSufficientChartMapCoverage(inMemory, duration) {
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
	batchResult := m.queryStoreBatchMetricMapWithGapFill("storage", needStore, duration)

	// Phase 3: Merge — use store data if better coverage.
	for _, id := range needStore {
		storeData, ok := batchResult[id]
		if !ok {
			continue
		}
		inMemory := result[id]
		if duration <= inMemoryChartThreshold && chartMapCoverageSpan(storeData) <= chartMapCoverageSpan(inMemory) {
			continue
		}
		result[id] = storeData
	}

	return result
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
