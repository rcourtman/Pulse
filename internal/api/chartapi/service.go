package chartapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api/apicontext"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

const workloadChartsCacheTTL = 3 * time.Second
const summaryChartsCacheTTL = 5 * time.Second

const (
	// All three chart routes share one retention budget. This stays modest for
	// low-memory container deployments while still holding several ordinary
	// large-estate responses.
	chartPayloadCacheMaxEntries = 64
	chartPayloadCacheMaxBytes   = 16 << 20

	infrastructureChartsCachePrefix = "infrastructure|"
	workloadsSummaryCachePrefix     = "workloads-summary|"
	workloadChartsCachePrefix       = "workload-charts|"
)

// MonitorResolver supplies the authenticated tenant monitor selected by the
// router context. Chart computation remains entirely owned by Service.
type MonitorResolver interface {
	MonitorForContext(context.Context) *monitoring.Monitor
}

// Service owns chart queries, aggregation, serialization, caching, and
// singleflight coordination independently of the HTTP router package.
type Service struct {
	resolver MonitorResolver

	chartPayloads              boundedChartPayloadCache
	workloadChartsComputeGroup singleflight.Group
}

func NewService(resolver MonitorResolver) *Service {
	return &Service{
		resolver: resolver,
		chartPayloads: newBoundedChartPayloadCache(
			chartPayloadCacheMaxEntries,
			chartPayloadCacheMaxBytes,
		),
	}
}

func (r *Service) getTenantMonitor(ctx context.Context) *monitoring.Monitor {
	if r == nil || r.resolver == nil {
		return nil
	}
	return r.resolver.MonitorForContext(ctx)
}

func storageChartsSelectedNodeName(resource unifiedresources.Resource) string {
	if name := strings.TrimSpace(resource.Name); name != "" {
		return name
	}
	if resource.TrueNAS != nil {
		if hostname := strings.TrimSpace(resource.TrueNAS.Hostname); hostname != "" {
			return hostname
		}
	}
	for _, hostname := range resource.Identity.Hostnames {
		if hostname = strings.TrimSpace(hostname); hostname != "" {
			return hostname
		}
	}
	return ""
}

func storageChartsSelectedNodeInstance(resource unifiedresources.Resource) string {
	if resource.Proxmox == nil {
		return ""
	}
	return strings.TrimSpace(resource.Proxmox.Instance)
}

// handleCharts handles chart data requests
func (r *Service) HandleCharts(w http.ResponseWriter, req *http.Request) {
	log.Debug().Str("method", req.Method).Str("url", req.URL.String()).Msg("Charts endpoint hit")
	const inMemoryChartThreshold = 2 * time.Hour

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get time range from query parameters
	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "1h"
	}

	// Convert time range to duration.
	duration := parseChartsRangeDuration(timeRange)

	// Get tenant-specific monitor and current state
	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Tenant monitor is not available", http.StatusInternalServerError)
		return
	}
	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		http.Error(w, "State unavailable", http.StatusInternalServerError)
		return
	}
	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

	// Create chart data structure that matches frontend expectations
	chartData := make(map[string]VMChartData)
	nodeData := make(map[string]NodeChartData)

	currentTime := time.Now().UnixMilli() // JavaScript timestamp format
	oldestTimestamp := currentTime

	// Process VMs and Containers - batch-load historical data (1-2 SQL calls
	// per family instead of N).
	oldestTimestamp = collectGuestChartData(monitor, "vm", readState.VMs(), duration, chartData, currentTime, oldestTimestamp)
	oldestTimestamp = collectGuestChartData(monitor, "container", readState.Containers(), duration, chartData, currentTime, oldestTimestamp)

	// Process Storage - batch-load historical data (1-2 SQL calls instead of N).
	storageData := make(map[string]StorageChartData)
	spList := readState.StoragePools()
	storageIDs := make([]string, 0, len(spList))
	for _, sp := range spList {
		if sp == nil {
			continue
		}
		if sid := sp.SourceID(); sid != "" {
			storageIDs = append(storageIDs, sid)
		}
	}
	storageBatchMetrics := monitor.GetStorageMetricsForChartBatch(storageIDs, duration)
	for _, sp := range spList {
		if sp == nil {
			continue
		}
		sid := sp.SourceID()
		if sid == "" {
			continue
		}
		storageData[sid] = make(StorageChartData)
		if batchMetrics, ok := storageBatchMetrics[sid]; ok {
			if usagePoints, found := batchMetrics["usage"]; found && len(usagePoints) > 0 {
				storageData[sid]["disk"] = make([]MetricPoint, len(usagePoints))
				for i, point := range usagePoints {
					ts := point.Timestamp.UnixMilli()
					if ts < oldestTimestamp {
						oldestTimestamp = ts
					}
					storageData[sid]["disk"][i] = MetricPoint{
						Timestamp: ts,
						Value:     point.Value,
					}
				}
			}
		}
		if len(storageData[sid]["disk"]) == 0 {
			storageData[sid]["disk"] = []MetricPoint{
				{Timestamp: currentTime, Value: sp.DiskPercent()},
			}
		}
	}

	// Process Nodes - batch-load historical data (1-2 SQL calls instead of N×5).
	nodeMetricTypes := []string{"cpu", "memory", "disk", "netin", "netout"}
	nodeList := readState.Nodes()
	nodeIDs := make([]string, 0, len(nodeList))
	for _, node := range nodeList {
		if node == nil {
			continue
		}
		if nid := node.SourceID(); nid != "" {
			nodeIDs = append(nodeIDs, nid)
		}
	}
	nodeBatchMetrics := monitor.GetNodeMetricsForChartBatch(nodeIDs, nodeMetricTypes, duration)
	for _, node := range nodeList {
		if node == nil {
			continue
		}
		nid := node.SourceID()
		if nid == "" {
			continue
		}
		nodeData[nid] = make(NodeChartData)
		if batchMetrics, ok := nodeBatchMetrics[nid]; ok {
			for _, metricType := range nodeMetricTypes {
				points, found := batchMetrics[metricType]
				if !found {
					continue
				}
				nodeData[nid][metricType] = make([]MetricPoint, len(points))
				for i, point := range points {
					ts := point.Timestamp.UnixMilli()
					if ts < oldestTimestamp {
						oldestTimestamp = ts
					}
					nodeData[nid][metricType][i] = MetricPoint{
						Timestamp: ts,
						Value:     point.Value,
					}
				}
			}
		}
		for _, metricType := range nodeMetricTypes {
			if len(nodeData[nid][metricType]) == 0 {
				var value float64
				hasFallbackValue := true
				switch metricType {
				case "cpu":
					value = node.CPUPercent()
				case "memory":
					value = node.MemoryPercent()
				case "disk":
					value = node.DiskPercent()
				default:
					hasFallbackValue = false
				}
				if hasFallbackValue {
					nodeData[nid][metricType] = []MetricPoint{
						{Timestamp: currentTime, Value: value},
					}
				}
			}
		}
	}

	// Build guest type map with canonical v6 names.
	guestTypes := make(map[string]string)
	for _, vm := range readState.VMs() {
		if vm == nil {
			continue
		}
		if sid := vm.SourceID(); sid != "" {
			guestTypes[sid] = "vm"
		}
	}
	for _, ct := range readState.Containers() {
		if ct == nil {
			continue
		}
		if sid := ct.SourceID(); sid != "" {
			guestTypes[sid] = "system-container"
		}
	}
	for _, dc := range readState.DockerContainers() {
		if dc == nil {
			continue
		}
		if key := strings.TrimSpace(dc.ID()); key != "" {
			guestTypes[key] = "app-container"
		}
	}

	// Process Docker containers - batch-load historical data (1-2 SQL calls instead of N).
	dockerData := make(map[string]VMChartData)
	dcList := readState.DockerContainers()
	dcRequests := make([]monitoring.GuestChartRequest, 0, len(dcList))
	for _, dc := range dcList {
		_, request, ok := appContainerChartRequest(dc)
		if !ok {
			continue
		}
		dcRequests = append(dcRequests, request)
	}
	dcBatchMetrics := monitor.GetGuestMetricsForChartBatch("dockerContainer", dcRequests, duration, infrastructureSummaryMetricOrder...)
	for _, dc := range dcList {
		responseKey, request, ok := appContainerChartRequest(dc)
		if !ok {
			continue
		}
		dockerData[responseKey] = make(VMChartData)
		if batchMetrics, ok := dcBatchMetrics[request.SQLResourceID]; ok {
			oldestTimestamp = fillChartSeriesFromBatch(dockerData[responseKey], batchMetrics, oldestTimestamp)
		}
		if len(dockerData[responseKey]["cpu"]) == 0 {
			dockerData[responseKey]["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: dc.CPUPercent()}}
			dockerData[responseKey]["memory"] = []MetricPoint{{Timestamp: currentTime, Value: dc.MemoryPercent()}}
			dockerData[responseKey]["disk"] = []MetricPoint{{Timestamp: currentTime, Value: dc.DiskPercent()}}
		}
	}

	// Process Docker hosts - batch-load historical data (1-2 SQL calls instead of N).
	dockerHostData := make(map[string]VMChartData)
	dhList := readState.DockerHosts()
	dhRequests := make([]monitoring.GuestChartRequest, 0, len(dhList))
	for _, dh := range dhList {
		if dh == nil {
			continue
		}
		if dhID := dh.HostSourceID(); dhID != "" {
			dhRequests = append(dhRequests, monitoring.GuestChartRequest{
				InMemoryKey:   fmt.Sprintf("dockerHost:%s", dhID),
				SQLResourceID: dhID,
			})
		}
	}
	dhBatchMetrics := monitor.GetGuestMetricsForChartBatch("dockerHost", dhRequests, duration, infrastructureSummaryMetricOrder...)
	for _, dh := range dhList {
		if dh == nil {
			continue
		}
		dhID := dh.HostSourceID()
		if dhID == "" {
			continue
		}
		dockerHostData[dhID] = make(VMChartData)
		if batchMetrics, ok := dhBatchMetrics[dhID]; ok {
			oldestTimestamp = fillChartSeriesFromBatch(dockerHostData[dhID], batchMetrics, oldestTimestamp)
		}
		if len(dockerHostData[dhID]["cpu"]) == 0 {
			dockerHostData[dhID]["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: dh.CPUPercent()}}
			dockerHostData[dhID]["memory"] = []MetricPoint{{Timestamp: currentTime, Value: dh.MemoryPercent()}}
			var diskPercent float64
			if disks := dh.Disks(); len(disks) > 0 {
				diskPercent = disks[0].Usage
			}
			dockerHostData[dhID]["disk"] = []MetricPoint{{Timestamp: currentTime, Value: diskPercent}}
		}
	}

	// Process unified agents - batch-load historical data (1-2 SQL calls instead of N).
	agentData := make(map[string]VMChartData)
	hostList := readState.Hosts()
	agentRequests := make([]monitoring.GuestChartRequest, 0, len(hostList))
	for _, h := range hostList {
		_, request, ok := hostAgentChartRequest(h)
		if !ok {
			continue
		}
		agentRequests = append(agentRequests, request)
	}
	agentBatchMetrics := monitor.GetGuestMetricsForChartBatch("agent", agentRequests, duration, infrastructureSummaryMetricOrder...)
	for _, h := range hostList {
		hID, request, ok := hostAgentChartRequest(h)
		if !ok {
			continue
		}
		agentData[hID] = make(VMChartData)
		if batchMetrics, ok := agentBatchMetrics[request.SQLResourceID]; ok {
			oldestTimestamp = fillChartSeriesFromBatch(agentData[hID], batchMetrics, oldestTimestamp)
		}
		if len(agentData[hID]["cpu"]) == 0 {
			agentData[hID]["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: h.CPUPercent()}}
			agentData[hID]["memory"] = []MetricPoint{{Timestamp: currentTime, Value: h.MemoryPercent()}}
			agentData[hID]["disk"] = []MetricPoint{{Timestamp: currentTime, Value: h.DiskPercent()}}
		}
	}

	countChartPoints := func(metricsMap map[string]VMChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	countNodePoints := func(metricsMap map[string]NodeChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	countStoragePoints := func(metricsMap map[string]StorageChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	guestPoints := countChartPoints(chartData)
	nodePoints := countNodePoints(nodeData)
	storagePoints := countStoragePoints(storageData)
	dockerContainerPoints := countChartPoints(dockerData)
	dockerHostPoints := countChartPoints(dockerHostData)
	agentPoints := countChartPoints(agentData)

	response := ChartResponse{
		ChartData:      chartData,
		NodeData:       nodeData,
		StorageData:    storageData,
		DockerData:     dockerData,
		DockerHostData: dockerHostData,
		AgentData:      agentData,
		GuestTypes:     guestTypes,
		Timestamp:      currentTime,
		Stats: ChartStats{
			OldestDataTimestamp:   oldestTimestamp,
			Range:                 timeRange,
			RangeSeconds:          int64(duration / time.Second),
			MetricsStoreEnabled:   metricsStoreEnabled,
			PrimarySourceHint:     primarySourceHint,
			InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
			PointCounts: ChartPointCounts{
				Total:            guestPoints + nodePoints + storagePoints + dockerContainerPoints + dockerHostPoints + agentPoints,
				Guests:           guestPoints,
				Nodes:            nodePoints,
				Storage:          storagePoints,
				DockerContainers: dockerContainerPoints,
				DockerHosts:      dockerHostPoints,
				Agents:           agentPoints,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Int("guests", len(chartData)).
		Int("nodes", len(nodeData)).
		Int("storage", len(storageData)).
		Int("dockerContainers", len(dockerData)).
		Int("agents", len(agentData)).
		Str("range", timeRange).
		Msg("Chart data response sent")
}

func parseWorkloadMaxPoints(raw string) int {
	const (
		defaultMaxPoints = 180
		minMaxPoints     = 30
		maxMaxPoints     = 500
	)

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultMaxPoints
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return defaultMaxPoints
	}
	if value < minMaxPoints {
		return minMaxPoints
	}
	if value > maxMaxPoints {
		return maxMaxPoints
	}
	return value
}

func ParseWorkloadMaxPoints(raw string) int { return parseWorkloadMaxPoints(raw) }

func hostAgentChartRequest(host *unifiedresources.HostView) (string, monitoring.GuestChartRequest, bool) {
	if host == nil {
		return "", monitoring.GuestChartRequest{}, false
	}

	if agentID := strings.TrimSpace(host.AgentID()); agentID != "" {
		return agentID, monitoring.GuestChartRequest{
			InMemoryKey:   fmt.Sprintf("agent:%s", agentID),
			SQLResourceID: agentID,
		}, true
	}

	target := host.MetricsTarget()
	if target == nil {
		return "", monitoring.GuestChartRequest{}, false
	}

	metricID := strings.TrimSpace(target.ResourceID)
	if metricID == "" {
		return "", monitoring.GuestChartRequest{}, false
	}

	return metricID, monitoring.GuestChartRequest{
		InMemoryKey:   fmt.Sprintf("agent:%s", metricID),
		SQLResourceID: metricID,
	}, true
}

func appContainerChartMetricID(container *unifiedresources.DockerContainerView) string {
	if container == nil {
		return ""
	}

	if target := container.MetricsTarget(); target != nil {
		if metricID := strings.TrimSpace(target.ResourceID); metricID != "" {
			return metricID
		}
	}

	return strings.TrimSpace(container.ContainerID())
}

func appContainerChartRequest(container *unifiedresources.DockerContainerView) (string, monitoring.GuestChartRequest, bool) {
	if container == nil {
		return "", monitoring.GuestChartRequest{}, false
	}

	responseKey := strings.TrimSpace(container.ID())
	if responseKey == "" {
		responseKey = strings.TrimSpace(container.ContainerID())
	}
	metricID := appContainerChartMetricID(container)
	if responseKey == "" || metricID == "" {
		return "", monitoring.GuestChartRequest{}, false
	}

	return responseKey, monitoring.GuestChartRequest{
		InMemoryKey:   fmt.Sprintf("docker:%s", metricID),
		SQLResourceID: metricID,
	}, true
}

func canonicalGuestResponseKey(resourceID, instance, node string, vmid int) string {
	trimmedInstance := strings.TrimSpace(instance)
	trimmedNode := strings.TrimSpace(node)
	if trimmedInstance != "" && trimmedNode != "" && vmid > 0 {
		return fmt.Sprintf("%s:%s:%d", trimmedInstance, trimmedNode, vmid)
	}
	return strings.TrimSpace(resourceID)
}

func vmChartMetricID(vm *unifiedresources.VMView) string {
	if vm == nil {
		return ""
	}

	if target := vm.MetricsTarget(); target != nil {
		if metricID := strings.TrimSpace(target.ResourceID); metricID != "" {
			return metricID
		}
	}

	return strings.TrimSpace(vm.SourceID())
}

func vmChartRequest(vm *unifiedresources.VMView) (string, monitoring.GuestChartRequest, bool) {
	if vm == nil {
		return "", monitoring.GuestChartRequest{}, false
	}

	responseKey := canonicalGuestResponseKey(vm.ID(), vm.Instance(), vm.Node(), vm.VMID())
	metricID := vmChartMetricID(vm)
	if responseKey == "" || metricID == "" {
		return "", monitoring.GuestChartRequest{}, false
	}

	return responseKey, monitoring.GuestChartRequest{
		InMemoryKey:   metricID,
		SQLResourceID: metricID,
	}, true
}

func VMChartRequest(vm *unifiedresources.VMView) (string, monitoring.GuestChartRequest, bool) {
	return vmChartRequest(vm)
}

func systemContainerChartMetricID(container *unifiedresources.ContainerView) string {
	if container == nil {
		return ""
	}

	if target := container.MetricsTarget(); target != nil {
		if metricID := strings.TrimSpace(target.ResourceID); metricID != "" {
			return metricID
		}
	}

	return strings.TrimSpace(container.SourceID())
}

func systemContainerChartRequest(container *unifiedresources.ContainerView) (string, monitoring.GuestChartRequest, bool) {
	if container == nil {
		return "", monitoring.GuestChartRequest{}, false
	}

	responseKey := canonicalGuestResponseKey(container.ID(), container.Instance(), container.Node(), container.VMID())
	metricID := systemContainerChartMetricID(container)
	if responseKey == "" || metricID == "" {
		return "", monitoring.GuestChartRequest{}, false
	}

	return responseKey, monitoring.GuestChartRequest{
		InMemoryKey:   metricID,
		SQLResourceID: metricID,
	}, true
}

func SystemContainerChartRequest(container *unifiedresources.ContainerView) (string, monitoring.GuestChartRequest, bool) {
	return systemContainerChartRequest(container)
}

func capMetricPointSeriesByIndex(points []MetricPoint, maxPoints int) []MetricPoint {
	if len(points) <= maxPoints || maxPoints <= 0 {
		return points
	}
	if maxPoints == 1 {
		return []MetricPoint{points[len(points)-1]}
	}

	result := make([]MetricPoint, 0, maxPoints)
	step := float64(len(points)-1) / float64(maxPoints-1)
	prevIndex := -1

	for i := 0; i < maxPoints; i++ {
		index := int(float64(i)*step + 0.5)
		if index <= prevIndex {
			index = prevIndex + 1
		}
		if index >= len(points) {
			index = len(points) - 1
		}
		result = append(result, points[index])
		prevIndex = index
	}

	if result[len(result)-1].Timestamp != points[len(points)-1].Timestamp {
		result[len(result)-1] = points[len(points)-1]
	}
	return result
}

func CapMetricPointSeriesByIndex(points []MetricPoint, maxPoints int) []MetricPoint {
	return capMetricPointSeriesByIndex(points, maxPoints)
}

const (
	infrastructureSummaryMinSeriesPoints = 24
	infrastructureSummaryMaxSeriesPoints = 96
	workloadsSummaryMinSeriesPoints      = 24
	workloadsSummaryMaxSeriesPoints      = 96
)

const InfrastructureSummaryMaxSeriesPoints = infrastructureSummaryMaxSeriesPoints
const WorkloadsSummaryMaxSeriesPoints = workloadsSummaryMaxSeriesPoints

// capMetricPointSeries keeps mixed-cadence series visually proportional across
// the selected time window. Index-based capping over-selects recent dense
// samples, which bunches the right edge on long ranges.
func capMetricPointSeries(points []MetricPoint, maxPoints int) []MetricPoint {
	if len(points) <= maxPoints || maxPoints <= 0 {
		return points
	}
	if maxPoints == 1 {
		return []MetricPoint{points[len(points)-1]}
	}

	startTimestamp := points[0].Timestamp
	endTimestamp := points[len(points)-1].Timestamp
	if endTimestamp <= startTimestamp {
		return capMetricPointSeriesByIndex(points, maxPoints)
	}

	bucketSpan := float64(endTimestamp-startTimestamp) / float64(maxPoints-1)
	if bucketSpan < 1 {
		return capMetricPointSeriesByIndex(points, maxPoints)
	}

	type timeBucketRepresentative struct {
		point    MetricPoint
		distance float64
		ok       bool
	}

	buckets := make([]timeBucketRepresentative, maxPoints)
	for _, point := range points {
		index := int(math.Round(float64(point.Timestamp-startTimestamp) / bucketSpan))
		if index < 0 {
			index = 0
		}
		if index >= maxPoints {
			index = maxPoints - 1
		}

		targetTimestamp := float64(startTimestamp) + bucketSpan*float64(index)
		distance := math.Abs(float64(point.Timestamp) - targetTimestamp)
		current := buckets[index]
		if !current.ok ||
			distance < current.distance ||
			(distance == current.distance && point.Timestamp > current.point.Timestamp) {
			buckets[index] = timeBucketRepresentative{
				point:    point,
				distance: distance,
				ok:       true,
			}
		}
	}

	result := make([]MetricPoint, 0, maxPoints)
	result = append(result, points[0])
	lastAddedTimestamp := points[0].Timestamp
	for index := 1; index < maxPoints-1; index++ {
		bucket := buckets[index]
		if !bucket.ok {
			continue
		}
		if bucket.point.Timestamp <= lastAddedTimestamp {
			continue
		}
		result = append(result, bucket.point)
		lastAddedTimestamp = bucket.point.Timestamp
	}

	lastPoint := points[len(points)-1]
	if lastPoint.Timestamp <= lastAddedTimestamp {
		result[len(result)-1] = lastPoint
		return result
	}

	result = append(result, lastPoint)
	return result
}

func targetBoundedSummarySeriesPoints(duration time.Duration, minPoints, maxPoints int) int {
	if duration <= 0 {
		return minPoints
	}

	target := int(duration / time.Minute)
	if target < minPoints {
		target = minPoints
	}
	if target > maxPoints {
		target = maxPoints
	}
	if target < 2 {
		target = 2
	}
	return target
}

type infrastructureSummaryBucket struct {
	count          int
	sum            float64
	max            float64
	firstTimestamp int64
	lastTimestamp  int64
	lastValue      float64
}

func targetInfrastructureSummarySeriesPoints(duration time.Duration) int {
	return targetBoundedSummarySeriesPoints(
		duration,
		infrastructureSummaryMinSeriesPoints,
		infrastructureSummaryMaxSeriesPoints,
	)
}

func infrastructureChartsCacheKey(req *http.Request, timeRange string, requestedMetricNames []string) string {
	orgID := strings.TrimSpace(apicontext.OrgID(req.Context()))
	if orgID == "" {
		orgID = "default"
	}
	return orgID + "|" + strings.TrimSpace(timeRange) + "|" + strings.Join(requestedMetricNames, ",")
}

func (r *Service) cachedInfrastructureChartsPayload(key string, now time.Time) ([]byte, bool) {
	if r == nil || key == "" {
		return nil, false
	}
	return r.chartPayloads.get(infrastructureChartsCachePrefix+key, now)
}

func (r *Service) cacheInfrastructureChartsPayload(key string, payload []byte, now time.Time) {
	if r == nil || key == "" || len(payload) == 0 {
		return
	}
	r.chartPayloads.put(
		infrastructureChartsCachePrefix+key,
		payload,
		now.Add(summaryChartsCacheTTL),
		now,
	)
}

func targetWorkloadsSummarySeriesPoints(duration time.Duration) int {
	return targetBoundedSummarySeriesPoints(
		duration,
		workloadsSummaryMinSeriesPoints,
		workloadsSummaryMaxSeriesPoints,
	)
}

func workloadsSummaryChartsCacheKey(req *http.Request, timeRange, selectedNodeID string) string {
	orgID := strings.TrimSpace(apicontext.OrgID(req.Context()))
	if orgID == "" {
		orgID = "default"
	}
	return orgID + "|" + strings.TrimSpace(timeRange) + "|" + strings.TrimSpace(selectedNodeID)
}

func (r *Service) cachedWorkloadsSummaryChartsPayload(key string, now time.Time) ([]byte, bool) {
	if r == nil || key == "" {
		return nil, false
	}
	return r.chartPayloads.get(workloadsSummaryCachePrefix+key, now)
}

func (r *Service) cacheWorkloadsSummaryChartsPayload(key string, payload []byte, now time.Time) {
	if r == nil || key == "" || len(payload) == 0 {
		return
	}
	r.chartPayloads.put(
		workloadsSummaryCachePrefix+key,
		payload,
		now.Add(summaryChartsCacheTTL),
		now,
	)
}

func aggregateInfrastructureSummaryBucketValue(
	metricType string,
	bucket infrastructureSummaryBucket,
	isLastBucket bool,
) float64 {
	if bucket.count == 0 {
		return 0
	}
	if isLastBucket {
		return bucket.lastValue
	}

	switch metricType {
	case "memory", "disk":
		return bucket.sum / float64(bucket.count)
	default:
		return bucket.max
	}
}

// normalizeInfrastructureSummaryMetricPointSeries folds mixed-cadence history
// into equal-time buckets for the infrastructure summary endpoint so long-range
// sparklines do not bunch recent higher-resolution samples at the right edge.
func normalizeInfrastructureSummaryMetricPointSeries(
	points []MetricPoint,
	metricType string,
	duration time.Duration,
	windowEndMillis int64,
) []MetricPoint {
	targetPoints := targetInfrastructureSummarySeriesPoints(duration)
	if len(points) <= targetPoints || targetPoints < 2 || duration <= 0 {
		return points
	}

	durationMillis := int64(duration / time.Millisecond)
	if durationMillis <= 0 {
		return points
	}

	windowStartMillis := windowEndMillis - durationMillis
	bucketCount := targetPoints
	buckets := make([]infrastructureSummaryBucket, bucketCount)
	firstNonEmpty := -1
	lastNonEmpty := -1

	for _, point := range points {
		if point.Timestamp < windowStartMillis || point.Timestamp > windowEndMillis {
			continue
		}
		bucketIndex := int(((point.Timestamp - windowStartMillis) * int64(bucketCount)) / durationMillis)
		if bucketIndex < 0 {
			bucketIndex = 0
		}
		if bucketIndex >= bucketCount {
			bucketIndex = bucketCount - 1
		}

		bucket := &buckets[bucketIndex]
		if bucket.count == 0 {
			bucket.max = point.Value
			bucket.firstTimestamp = point.Timestamp
			if firstNonEmpty == -1 {
				firstNonEmpty = bucketIndex
			}
		} else if point.Value > bucket.max {
			bucket.max = point.Value
		}
		bucket.count++
		bucket.sum += point.Value
		bucket.lastTimestamp = point.Timestamp
		bucket.lastValue = point.Value
		lastNonEmpty = bucketIndex
	}

	if firstNonEmpty == -1 || lastNonEmpty == -1 {
		return points
	}

	result := make([]MetricPoint, 0, targetPoints)
	for bucketIndex := 0; bucketIndex < bucketCount; bucketIndex++ {
		bucket := buckets[bucketIndex]
		if bucket.count == 0 {
			continue
		}

		bucketStartMillis := windowStartMillis + (int64(bucketIndex)*durationMillis)/int64(bucketCount)
		bucketEndMillis := windowStartMillis + (int64(bucketIndex+1)*durationMillis)/int64(bucketCount)
		timestamp := bucketStartMillis + (bucketEndMillis-bucketStartMillis)/2
		switch bucketIndex {
		case firstNonEmpty:
			timestamp = bucket.firstTimestamp
		case lastNonEmpty:
			timestamp = bucket.lastTimestamp
		}

		result = append(result, MetricPoint{
			Timestamp: timestamp,
			Value: aggregateInfrastructureSummaryBucketValue(
				metricType,
				bucket,
				bucketIndex == lastNonEmpty,
			),
		})
	}

	if len(result) == 0 {
		return points
	}
	return result
}

func normalizeInfrastructureSummaryChartSeries(
	metrics map[string][]MetricPoint,
	duration time.Duration,
	windowEndMillis int64,
) {
	for metricType, points := range metrics {
		metrics[metricType] = normalizeInfrastructureSummaryMetricPointSeries(
			points,
			metricType,
			duration,
			windowEndMillis,
		)
	}
}

// sparklineMetrics lists the metric types consumed by summary sparklines
// and density maps. Metrics not in this set are omitted to keep payloads small.
// guestChartSourceView is the guest view subset the infrastructure summary
// chart builder consumes from VMs and LXC containers.
type guestChartSourceView interface {
	comparable
	SourceID() string
	CPUPercent() float64
	MemoryPercent() float64
	MemoryUsed() int64
	DiskPercent() float64
	NetIn() float64
	NetOut() float64
}

// collectGuestChartData batch-loads sparkline history for one proxmox guest
// family into chartData (1-2 SQL calls instead of N) and returns the updated
// oldest chart timestamp. Guests without history fall back to a single
// current-value point per metric.
func collectGuestChartData[V guestChartSourceView](
	monitor *monitoring.Monitor,
	storeType string,
	guests []V,
	duration time.Duration,
	chartData map[string]VMChartData,
	currentTime, oldestTimestamp int64,
) int64 {
	var zero V
	requests := make([]monitoring.GuestChartRequest, 0, len(guests))
	for _, g := range guests {
		if g == zero {
			continue
		}
		if id := g.SourceID(); id != "" {
			requests = append(requests, monitoring.GuestChartRequest{InMemoryKey: id, SQLResourceID: id})
		}
	}
	batch := monitor.GetGuestMetricsForChartBatch(storeType, requests, duration, guestSparklineMetricOrder...)
	for _, g := range guests {
		if g == zero {
			continue
		}
		id := g.SourceID()
		if id == "" {
			continue
		}
		chartData[id] = make(VMChartData)
		if batchMetrics, ok := batch[id]; ok {
			oldestTimestamp = fillChartSeriesFromBatch(chartData[id], batchMetrics, oldestTimestamp)
		}
		if len(chartData[id]["cpu"]) == 0 {
			chartData[id]["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: g.CPUPercent()}}
			chartData[id]["memory"] = []MetricPoint{{Timestamp: currentTime, Value: g.MemoryPercent()}}
			chartData[id]["memoryused"] = []MetricPoint{{Timestamp: currentTime, Value: float64(g.MemoryUsed())}}
			chartData[id]["disk"] = []MetricPoint{{Timestamp: currentTime, Value: g.DiskPercent()}}
			chartData[id]["netin"] = []MetricPoint{{Timestamp: currentTime, Value: g.NetIn()}}
			chartData[id]["netout"] = []MetricPoint{{Timestamp: currentTime, Value: g.NetOut()}}
		}
	}
	return oldestTimestamp
}

// fillChartSeriesFromBatch copies sparkline-eligible batch metric points
// into dst and returns the updated oldest chart timestamp. Shared by the
// per-family infrastructure summary chart loops.
func fillChartSeriesFromBatch(dst VMChartData, batchMetrics map[string][]monitoring.MetricPoint, oldestTimestamp int64) int64 {
	for metricType, points := range batchMetrics {
		if !sparklineMetrics[metricType] {
			continue
		}
		dst[metricType] = make([]MetricPoint, len(points))
		for i, point := range points {
			ts := point.Timestamp.UnixMilli()
			if ts < oldestTimestamp {
				oldestTimestamp = ts
			}
			dst[metricType][i] = MetricPoint{
				Timestamp: ts,
				Value:     point.Value,
			}
		}
	}
	return oldestTimestamp
}

var sparklineMetrics = map[string]bool{
	"cpu":        true,
	"memory":     true,
	"memoryused": true,
	"disk":       true,
	"diskread":   true,
	"diskwrite":  true,
	"netin":      true,
	"netout":     true,
}

var infrastructureSummaryMetricOrder = []string{
	"cpu",
	"memory",
	"disk",
	"diskread",
	"diskwrite",
	"netin",
	"netout",
}

var guestSparklineMetricOrder = []string{
	"cpu",
	"memory",
	"memoryused",
	"disk",
	"diskread",
	"diskwrite",
	"netin",
	"netout",
}

var workloadSummaryMetricOrder = []string{
	"cpu",
	"memory",
	"disk",
	"netin",
	"netout",
}

func parseInfrastructureSummaryRequestedMetrics(
	query url.Values,
) ([]string, map[string]bool, error) {
	rawValues, ok := query["metrics"]
	if !ok || len(rawValues) == 0 {
		requested := make(map[string]bool, len(infrastructureSummaryMetricOrder))
		for _, metricType := range infrastructureSummaryMetricOrder {
			requested[metricType] = true
		}
		return append([]string(nil), infrastructureSummaryMetricOrder...), requested, nil
	}

	requestedList := make([]string, 0, len(infrastructureSummaryMetricOrder))
	requestedSet := make(map[string]bool, len(infrastructureSummaryMetricOrder))
	invalid := make([]string, 0)

	for _, rawValue := range rawValues {
		for _, part := range strings.Split(rawValue, ",") {
			metricType := strings.TrimSpace(strings.ToLower(part))
			if metricType == "" {
				continue
			}
			if !sparklineMetrics[metricType] {
				invalid = append(invalid, metricType)
				continue
			}
			if requestedSet[metricType] {
				continue
			}
			requestedSet[metricType] = true
			requestedList = append(requestedList, metricType)
		}
	}

	if len(invalid) > 0 {
		return nil, nil, fmt.Errorf("invalid infrastructure metrics filter: %s", strings.Join(invalid, ", "))
	}
	if len(requestedList) == 0 {
		return nil, nil, fmt.Errorf("infrastructure metrics filter must include at least one valid metric")
	}
	return requestedList, requestedSet, nil
}

func convertMetricsForChart(
	metrics map[string][]monitoring.MetricPoint,
	oldestTimestamp *int64,
	maxPoints int,
) VMChartData {
	converted := make(VMChartData, len(metrics))
	for metricType, metricPoints := range metrics {
		if !sparklineMetrics[metricType] {
			continue
		}
		points := make([]MetricPoint, len(metricPoints))
		for i, point := range metricPoints {
			ts := point.Timestamp.UnixMilli()
			if ts < *oldestTimestamp {
				*oldestTimestamp = ts
			}
			points[i] = MetricPoint{
				Timestamp: ts,
				Value:     point.Value,
			}
		}
		converted[metricType] = capMetricPointSeries(points, maxPoints)
	}
	return converted
}

// guestLiveMetricsView is the slice of the unified workload view API needed
// to seed a chart from live values; VM and container views both satisfy it.
type guestLiveMetricsView interface {
	CPUPercent() float64
	MemoryPercent() float64
	MemoryUsed() int64
	DiskPercent() float64
	NetIn() float64
	NetOut() float64
}

// guestChartSeriesWithLiveFallback converts a guest's batched metric history
// into chart series, substituting single live-value points when no history
// exists yet so freshly added guests still chart.
func guestChartSeriesWithLiveFallback(
	metrics map[string][]monitoring.MetricPoint,
	guest guestLiveMetricsView,
	oldestTimestamp *int64,
	maxPoints int,
	currentTime int64,
) VMChartData {
	series := convertMetricsForChart(metrics, oldestTimestamp, maxPoints)
	if len(series["cpu"]) == 0 {
		series["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: guest.CPUPercent()}}
		series["memory"] = []MetricPoint{{Timestamp: currentTime, Value: guest.MemoryPercent()}}
		series["memoryused"] = []MetricPoint{{Timestamp: currentTime, Value: float64(guest.MemoryUsed())}}
		series["disk"] = []MetricPoint{{Timestamp: currentTime, Value: guest.DiskPercent()}}
		series["netin"] = []MetricPoint{{Timestamp: currentTime, Value: guest.NetIn()}}
		series["netout"] = []MetricPoint{{Timestamp: currentTime, Value: guest.NetOut()}}
	}
	return series
}

const (
	mockWorkloadMinSeriesPoints = 24
	mockWorkloadMaxSeriesPoints = 180
)

func targetMockSeriesPoints(duration time.Duration, maxPoints int) int {
	target := int(duration / (2 * time.Minute))
	if target < mockWorkloadMinSeriesPoints {
		target = mockWorkloadMinSeriesPoints
	}
	if maxPoints > 0 && target > maxPoints {
		target = maxPoints
	}
	if target > mockWorkloadMaxSeriesPoints {
		target = mockWorkloadMaxSeriesPoints
	}
	if target < 2 {
		target = 2
	}
	return target
}

func TargetMockSeriesPoints(duration time.Duration, maxPoints int) int {
	return targetMockSeriesPoints(duration, maxPoints)
}

// mockMetricStyle returns the series style for a given metric type.
func mockMetricStyle(metricType string) monitoring.SeriesStyle {
	switch metricType {
	case "cpu", "diskread", "diskwrite", "netin", "netout":
		return monitoring.StyleSpiky
	case "memory":
		return monitoring.StylePlateau
	default:
		return monitoring.StyleFlat
	}
}

// generateStyledMockSeries produces a MetricPoint slice using the style-based
// generator from the monitoring package.
func generateStyledMockSeries(
	nowMillis int64,
	duration time.Duration,
	numPoints int,
	current float64,
	resourceType string,
	resourceID string,
	metricType string,
) []MetricPoint {
	style := mockMetricStyle(metricType)

	durationMillis := int64(duration / time.Millisecond)
	if durationMillis <= 0 {
		durationMillis = int64(time.Minute / time.Millisecond)
	}
	step := durationMillis / int64(numPoints-1)
	if step <= 0 {
		step = 1
	}
	startMillis := nowMillis - durationMillis
	timestamps := make([]time.Time, numPoints)
	for i := 0; i < numPoints; i++ {
		timestamps[i] = time.UnixMilli(startMillis + int64(i)*step)
	}
	values := monitoring.GenerateSeededResourceMetricSeriesForTimestamps(
		current,
		timestamps,
		resourceType,
		resourceID,
		metricType,
		style,
	)
	points := make([]MetricPoint, numPoints)
	for i := 0; i < numPoints; i++ {
		points[i] = MetricPoint{
			Timestamp: startMillis + int64(i)*step,
			Value:     values[i],
		}
	}
	return points
}

func GenerateStyledMockSeries(
	nowMillis int64,
	duration time.Duration,
	numPoints int,
	current float64,
	resourceType string,
	resourceID string,
	metricType string,
) []MetricPoint {
	return generateStyledMockSeries(nowMillis, duration, numPoints, current, resourceType, resourceID, metricType)
}

func buildSyntheticMetricHistorySeries(
	now time.Time,
	duration time.Duration,
	maxPoints int,
	resourceType string,
	resourceID string,
	metricType string,
	current float64,
) []monitoring.MetricPoint {
	switch metricType {
	case "disk", "diskread", "diskwrite", "usage":
	case "smart_temp":
		if current <= 0 {
			return nil
		}
	default:
		return nil
	}

	numPoints := targetMockSeriesPoints(duration, maxPoints)
	series := generateStyledMockSeries(
		now.UnixMilli(), duration, numPoints,
		current, resourceType, resourceID, metricType,
	)

	converted := make([]monitoring.MetricPoint, len(series))
	for i, point := range series {
		converted[i] = monitoring.MetricPoint{
			Timestamp: time.UnixMilli(point.Timestamp),
			Value:     point.Value,
		}
	}

	return converted
}

func BuildSyntheticMetricHistorySeries(
	now time.Time,
	duration time.Duration,
	maxPoints int,
	resourceType string,
	resourceID string,
	metricType string,
	current float64,
) []monitoring.MetricPoint {
	return buildSyntheticMetricHistorySeries(now, duration, maxPoints, resourceType, resourceID, metricType, current)
}

func buildMockWorkloadMetricHistorySeries(
	now time.Time,
	duration time.Duration,
	maxPoints int,
	resourceType string,
	resourceID string,
	metricType string,
	current float64,
) []monitoring.MetricPoint {
	switch metricType {
	case "cpu", "memory", "disk":
	case "diskread", "diskwrite", "netin", "netout":
	default:
		return nil
	}

	numPoints := targetMockSeriesPoints(duration, maxPoints)
	series := generateStyledMockSeries(
		now.UnixMilli(), duration, numPoints,
		current, resourceType, resourceID, metricType,
	)

	converted := make([]monitoring.MetricPoint, len(series))
	for i, point := range series {
		converted[i] = monitoring.MetricPoint{
			Timestamp: time.UnixMilli(point.Timestamp),
			Value:     point.Value,
		}
	}

	return converted
}

func BuildMockWorkloadMetricHistorySeries(
	now time.Time,
	duration time.Duration,
	maxPoints int,
	resourceType string,
	resourceID string,
	metricType string,
	current float64,
) []monitoring.MetricPoint {
	return buildMockWorkloadMetricHistorySeries(now, duration, maxPoints, resourceType, resourceID, metricType, current)
}

// handleWorkloadCharts serves workload-only chart data used by workloads
// sparklines. It intentionally excludes infrastructure/storage chart payloads
// to keep requests small and stable for large fleets.
func (r *Service) HandleWorkloadCharts(w http.ResponseWriter, req *http.Request) {
	log.Debug().Str("method", req.Method).Str("url", req.URL.String()).Msg("Workload charts endpoint hit")
	const inMemoryChartThreshold = 2 * time.Hour

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "1h"
	}
	selectedNodeID := strings.TrimSpace(query.Get("node"))
	maxPointsRaw := query.Get("maxPoints")
	maxPoints := parseWorkloadMaxPoints(maxPointsRaw)
	duration := parseChartsRangeDuration(timeRange)

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Tenant monitor is not available", http.StatusInternalServerError)
		return
	}

	orgID := apicontext.OrgID(req.Context())
	if orgID == "" {
		orgID = "default"
	}
	cacheKey := orgID + "|" + timeRange + "|" + selectedNodeID + "|" + maxPointsRaw

	cacheKey = workloadChartsCachePrefix + cacheKey
	now := time.Now()
	if body, ok := r.chartPayloads.get(cacheKey, now); ok {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(body); err != nil {
			log.Error().Err(err).Msg("Failed to write cached workload chart data response")
		}
		return
	}

	v, err, _ := r.workloadChartsComputeGroup.Do(cacheKey, func() (any, error) {
		// Re-check cache inside the singleflight barrier in case an earlier
		// caller already populated it while we were queued.
		if body, ok := r.chartPayloads.get(cacheKey, time.Now()); ok {
			return body, nil
		}

		body, err := r.buildWorkloadChartsResponse(req.Context(), monitor, timeRange, selectedNodeID, maxPoints, duration, inMemoryChartThreshold)
		if err != nil {
			return nil, err
		}
		cachedAt := time.Now()
		r.chartPayloads.put(cacheKey, body, cachedAt.Add(workloadChartsCacheTTL), cachedAt)
		return body, nil
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to build workload chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(v.([]byte)); err != nil {
		log.Error().Err(err).Msg("Failed to write workload chart data response")
	}
}

// buildWorkloadChartsResponse runs the heavy compute path for handleWorkloadCharts
// and returns the marshaled JSON body. Extracted so the handler can wrap it
// with caching + singleflight.
func (r *Service) buildWorkloadChartsResponse(
	ctx context.Context,
	monitor *monitoring.Monitor,
	timeRange string,
	selectedNodeID string,
	maxPoints int,
	duration time.Duration,
	inMemoryChartThreshold time.Duration,
) ([]byte, error) {
	_ = ctx
	nodes := monitor.NodesSnapshot()
	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		return nil, fmt.Errorf("state unavailable")
	}
	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

	currentTime := time.Now().UnixMilli()
	oldestTimestamp := currentTime

	var selectedNode *models.Node
	if selectedNodeID != "" {
		for idx := range nodes {
			if nodes[idx].ID == selectedNodeID {
				selectedNode = &nodes[idx]
				break
			}
		}
		if selectedNode == nil {
			log.Debug().
				Str("selectedNodeID", selectedNodeID).
				Msg("Workload charts node filter not found in current state; falling back to global scope")
		}
	}

	matchesSelectedNode := func(instance, nodeName string) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		return strings.EqualFold(strings.TrimSpace(instance), strings.TrimSpace(selectedNode.Instance)) &&
			strings.EqualFold(strings.TrimSpace(nodeName), strings.TrimSpace(selectedNode.Name))
	}

	matchesSelectedDockerHostView := func(host *unifiedresources.DockerHostView) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		if host == nil {
			return false
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(host.Hostname()), nodeName) ||
			strings.EqualFold(strings.TrimSpace(host.Name()), nodeName)
	}

	matchesSelectedAgentHostView := func(host *unifiedresources.HostView) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		if host == nil {
			return false
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(host.Hostname()), nodeName) ||
			strings.EqualFold(strings.TrimSpace(host.Name()), nodeName) ||
			strings.EqualFold(strings.TrimSpace(host.AgentID()), nodeName) ||
			strings.EqualFold(strings.TrimSpace(host.ID()), nodeName)
	}

	matchesSelectedKubernetesPodView := func(pod *unifiedresources.PodView) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		if pod == nil {
			return false
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(pod.NodeName()), nodeName)
	}

	chartData := make(map[string]VMChartData)
	dockerData := make(map[string]VMChartData)

	guestTypes := make(map[string]string)

	vmList := make([]*unifiedresources.VMView, 0)
	vmResponseKeys := make([]string, 0)
	vmRequests := make([]monitoring.GuestChartRequest, 0)
	for _, vm := range readState.VMs() {
		if vm == nil {
			continue
		}
		if !matchesSelectedNode(vm.Instance(), vm.Node()) {
			continue
		}

		responseKey, request, ok := vmChartRequest(vm)
		if !ok {
			continue
		}

		vmList = append(vmList, vm)
		vmResponseKeys = append(vmResponseKeys, responseKey)
		vmRequests = append(vmRequests, request)
	}
	containerList := make([]*unifiedresources.ContainerView, 0)
	containerResponseKeys := make([]string, 0)
	containerRequests := make([]monitoring.GuestChartRequest, 0)
	for _, ct := range readState.Containers() {
		if ct == nil {
			continue
		}
		if !matchesSelectedNode(ct.Instance(), ct.Node()) {
			continue
		}

		responseKey, request, ok := systemContainerChartRequest(ct)
		if !ok {
			continue
		}

		containerList = append(containerList, ct)
		containerResponseKeys = append(containerResponseKeys, responseKey)
		containerRequests = append(containerRequests, request)
	}
	podList := make([]*unifiedresources.PodView, 0)
	podRequests := make([]monitoring.GuestChartRequest, 0)
	for _, pod := range readState.Pods() {
		if pod == nil {
			continue
		}
		if !matchesSelectedKubernetesPodView(pod) {
			continue
		}

		metricKey := kubernetesPodMetricIDFromView(pod)
		if metricKey == "" {
			continue
		}

		podList = append(podList, pod)
		podRequests = append(podRequests, monitoring.GuestChartRequest{InMemoryKey: metricKey, SQLResourceID: metricKey})
	}
	dockerHostsByID := make(map[string]*unifiedresources.DockerHostView, len(readState.DockerHosts()))
	for _, host := range readState.DockerHosts() {
		if host == nil {
			continue
		}
		dockerHostsByID[host.ID()] = host
	}
	agentHostsByID := make(map[string]*unifiedresources.HostView, len(readState.Hosts()))
	for _, host := range readState.Hosts() {
		if host == nil {
			continue
		}
		agentHostsByID[host.ID()] = host
	}

	dockerContainerList := make([]*unifiedresources.DockerContainerView, 0)
	dockerContainerRequests := make([]monitoring.GuestChartRequest, 0)
	dockerContainerKeys := make([]string, 0)
	for _, container := range readState.DockerContainers() {
		if container == nil {
			continue
		}

		if selectedNodeID != "" && selectedNode != nil {
			host := dockerHostsByID[container.ParentID()]
			if host != nil {
				if !matchesSelectedDockerHostView(host) {
					continue
				}
			} else {
				agentHost := agentHostsByID[container.ParentID()]
				if agentHost == nil || !matchesSelectedAgentHostView(agentHost) {
					continue
				}
			}
		}

		responseKey, request, ok := appContainerChartRequest(container)
		if !ok {
			continue
		}
		dockerContainerList = append(dockerContainerList, container)
		dockerContainerKeys = append(dockerContainerKeys, responseKey)
		dockerContainerRequests = append(dockerContainerRequests, request)
	}
	var (
		vmBatchMetrics              map[string]map[string][]monitoring.MetricPoint
		containerBatchMetrics       map[string]map[string][]monitoring.MetricPoint
		podBatchMetrics             map[string]map[string][]monitoring.MetricPoint
		dockerContainerBatchMetrics map[string]map[string][]monitoring.MetricPoint
	)
	var workloadChartsBatchWG sync.WaitGroup
	workloadChartsBatchWG.Add(4)
	go func() {
		defer workloadChartsBatchWG.Done()
		vmBatchMetrics = monitor.GetGuestMetricsForChartBatch("vm", vmRequests, duration, guestSparklineMetricOrder...)
	}()
	go func() {
		defer workloadChartsBatchWG.Done()
		containerBatchMetrics = monitor.GetGuestMetricsForChartBatch("container", containerRequests, duration, guestSparklineMetricOrder...)
	}()
	go func() {
		defer workloadChartsBatchWG.Done()
		podBatchMetrics = monitor.GetGuestMetricsForChartBatch("k8s", podRequests, duration, workloadSummaryMetricOrder...)
	}()
	go func() {
		defer workloadChartsBatchWG.Done()
		dockerContainerBatchMetrics = monitor.GetGuestMetricsForChartBatch("dockerContainer", dockerContainerRequests, duration, infrastructureSummaryMetricOrder...)
	}()
	workloadChartsBatchWG.Wait()

	for idx, vm := range vmList {
		responseKey := vmResponseKeys[idx]
		metricID := vmRequests[idx].SQLResourceID
		guestTypes[responseKey] = "vm"
		chartData[responseKey] = guestChartSeriesWithLiveFallback(vmBatchMetrics[metricID], vm, &oldestTimestamp, maxPoints, currentTime)
	}

	for idx, ct := range containerList {
		responseKey := containerResponseKeys[idx]
		metricID := containerRequests[idx].SQLResourceID
		guestTypes[responseKey] = "system-container"
		chartData[responseKey] = guestChartSeriesWithLiveFallback(containerBatchMetrics[metricID], ct, &oldestTimestamp, maxPoints, currentTime)
	}

	for _, pod := range podList {
		metricKey := kubernetesPodMetricIDFromView(pod)
		series := convertMetricsForChart(podBatchMetrics[metricKey], &oldestTimestamp, maxPoints)
		guestTypes[metricKey] = "k8s"

		if len(series["cpu"]) == 0 {
			series["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: pod.CPUPercent()}}
			series["memory"] = []MetricPoint{{Timestamp: currentTime, Value: pod.MemoryPercent()}}
			series["disk"] = []MetricPoint{{Timestamp: currentTime, Value: pod.DiskPercent()}}
			series["netin"] = []MetricPoint{{Timestamp: currentTime, Value: pod.NetInRate()}}
			series["netout"] = []MetricPoint{{Timestamp: currentTime, Value: pod.NetOutRate()}}
		}
		chartData[metricKey] = series
	}

	for idx, container := range dockerContainerList {
		responseKey := dockerContainerKeys[idx]
		metricID := dockerContainerRequests[idx].SQLResourceID
		series := convertMetricsForChart(dockerContainerBatchMetrics[metricID], &oldestTimestamp, maxPoints)
		guestTypes[responseKey] = "app-container"

		if len(series["cpu"]) == 0 {
			series["cpu"] = []MetricPoint{{Timestamp: currentTime, Value: container.CPUPercent()}}
			series["memory"] = []MetricPoint{{Timestamp: currentTime, Value: container.MemoryPercent()}}
			series["disk"] = []MetricPoint{{Timestamp: currentTime, Value: container.DiskPercent()}}
			series["netin"] = []MetricPoint{{Timestamp: currentTime, Value: container.NetInRate()}}
			series["netout"] = []MetricPoint{{Timestamp: currentTime, Value: container.NetOutRate()}}
		}
		dockerData[responseKey] = series
	}

	countChartPoints := func(metricsMap map[string]VMChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	guestPoints := countChartPoints(chartData)
	dockerContainerPoints := countChartPoints(dockerData)

	response := EmptyWorkloadChartsResponse()
	response.ChartData = chartData
	response.DockerData = dockerData
	response.GuestTypes = guestTypes
	response.Timestamp = currentTime
	response.Stats = ChartStats{
		OldestDataTimestamp:   oldestTimestamp,
		Range:                 timeRange,
		RangeSeconds:          int64(duration / time.Second),
		MetricsStoreEnabled:   metricsStoreEnabled,
		PrimarySourceHint:     primarySourceHint,
		InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
		PointCounts: ChartPointCounts{
			Total:            guestPoints + dockerContainerPoints,
			Guests:           guestPoints,
			DockerContainers: dockerContainerPoints,
		},
	}

	body, err := json.Marshal(response.NormalizeCollections())
	if err != nil {
		return nil, fmt.Errorf("marshal workload chart response: %w", err)
	}
	return body, nil
}

// parseChartsRangeDuration converts the UI chart range query (e.g. "5m", "1h")
// into a duration. This is shared by /api/charts and /api/charts/infrastructure
// to prevent drift.
func parseChartsRangeDuration(rangeStr string) time.Duration {
	switch rangeStr {
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "8h":
		return 8 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return time.Hour
	}
}

// handleInfrastructureCharts serves infrastructure-only chart data.
// This is intentionally narrower than /api/charts to reduce payload size and server-side compute
// for the Infrastructure page summary cards.
func (r *Service) HandleInfrastructureCharts(w http.ResponseWriter, req *http.Request) {
	log.Debug().Str("method", req.Method).Str("url", req.URL.String()).Msg("Infrastructure charts endpoint hit")
	const inMemoryChartThreshold = 2 * time.Hour

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get time range from query parameters
	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "1h"
	}
	requestedMetricNames, requestedMetrics, err := parseInfrastructureSummaryRequestedMetrics(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// Convert time range to duration.
	duration := parseChartsRangeDuration(timeRange)

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Tenant monitor is not available", http.StatusInternalServerError)
		return
	}
	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		http.Error(w, "State unavailable", http.StatusInternalServerError)
		return
	}
	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

	now := time.Now()
	cacheKey := infrastructureChartsCacheKey(req, timeRange, requestedMetricNames)
	if payload, ok := r.cachedInfrastructureChartsPayload(cacheKey, now); ok {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(payload); err != nil {
			log.Error().Err(err).Msg("Failed to write cached infrastructure chart data response")
		}
		return
	}

	currentTime := now.UnixMilli()
	oldestTimestamp := currentTime

	// Process Nodes - batch-load historical data (1-2 SQL calls instead of N×5).
	nodeMetricTypes := make([]string, 0, 5)
	for _, metricType := range []string{"cpu", "memory", "disk", "netin", "netout"} {
		if requestedMetrics[metricType] {
			nodeMetricTypes = append(nodeMetricTypes, metricType)
		}
	}
	nodeData := make(map[string]NodeChartData)
	nodeList := readState.Nodes()
	nodeIDs := make([]string, 0, len(nodeList))
	for _, node := range nodeList {
		if node == nil {
			continue
		}
		if nid := node.SourceID(); nid != "" {
			nodeIDs = append(nodeIDs, nid)
		}
	}
	nodeBatchMetrics := map[string]map[string][]monitoring.MetricPoint{}
	if len(nodeMetricTypes) > 0 {
		nodeBatchMetrics = monitor.GetNodeMetricsForChartBatch(nodeIDs, nodeMetricTypes, duration)
	}
	for _, node := range nodeList {
		if node == nil {
			continue
		}
		nid := node.SourceID()
		if nid == "" {
			continue
		}
		nodeData[nid] = make(NodeChartData)
		if batchMetrics, ok := nodeBatchMetrics[nid]; ok {
			for _, metricType := range nodeMetricTypes {
				points, found := batchMetrics[metricType]
				if !found {
					continue
				}
				nodeData[nid][metricType] = make([]MetricPoint, len(points))
				for i, point := range points {
					ts := point.Timestamp.UnixMilli()
					if ts < oldestTimestamp {
						oldestTimestamp = ts
					}
					nodeData[nid][metricType][i] = MetricPoint{
						Timestamp: ts,
						Value:     point.Value,
					}
				}
			}
		}
		for _, metricType := range nodeMetricTypes {
			if len(nodeData[nid][metricType]) > 0 {
				continue
			}
			var value float64
			hasFallbackValue := true
			switch metricType {
			case "cpu":
				value = node.CPUPercent()
			case "memory":
				value = node.MemoryPercent()
			case "disk":
				value = node.DiskPercent()
			default:
				hasFallbackValue = false
			}
			if hasFallbackValue {
				nodeData[nid][metricType] = []MetricPoint{
					{Timestamp: currentTime, Value: value},
				}
			}
		}
		normalizeInfrastructureSummaryChartSeries(nodeData[nid], duration, currentTime)
	}

	// Process Docker hosts - batch-load historical data (1-2 SQL calls instead of N).
	dockerHostData := make(map[string]VMChartData)
	dhList := readState.DockerHosts()
	dhRequests := make([]monitoring.GuestChartRequest, 0, len(dhList))
	for _, dh := range dhList {
		if dh == nil {
			continue
		}
		if dhID := dh.HostSourceID(); dhID != "" {
			dhRequests = append(dhRequests, monitoring.GuestChartRequest{
				InMemoryKey:   fmt.Sprintf("dockerHost:%s", dhID),
				SQLResourceID: dhID,
			})
		}
	}
	dhBatchMetrics := monitor.GetGuestMetricsForChartBatch("dockerHost", dhRequests, duration, requestedMetricNames...)
	for _, dh := range dhList {
		if dh == nil {
			continue
		}
		dhID := dh.HostSourceID()
		if dhID == "" {
			continue
		}
		dockerHostData[dhID] = make(VMChartData)
		if batchMetrics, ok := dhBatchMetrics[dhID]; ok {
			for metricType, points := range batchMetrics {
				if !requestedMetrics[metricType] {
					continue
				}
				dockerHostData[dhID][metricType] = make([]MetricPoint, len(points))
				for i, point := range points {
					ts := point.Timestamp.UnixMilli()
					if ts < oldestTimestamp {
						oldestTimestamp = ts
					}
					dockerHostData[dhID][metricType][i] = MetricPoint{
						Timestamp: ts,
						Value:     point.Value,
					}
				}
			}
		}
		for _, metricType := range requestedMetricNames {
			if len(dockerHostData[dhID][metricType]) > 0 {
				continue
			}
			var value float64
			hasFallbackValue := true
			switch metricType {
			case "cpu":
				value = dh.CPUPercent()
			case "memory":
				value = dh.MemoryPercent()
			case "disk":
				if disks := dh.Disks(); len(disks) > 0 {
					value = disks[0].Usage
				}
			default:
				hasFallbackValue = false
			}
			if hasFallbackValue {
				dockerHostData[dhID][metricType] = []MetricPoint{{Timestamp: currentTime, Value: value}}
			}
		}
		normalizeInfrastructureSummaryChartSeries(dockerHostData[dhID], duration, currentTime)
	}

	// Process unified agents - batch-load historical data (1-2 SQL calls instead of N).
	agentData := make(map[string]VMChartData)
	hostList := readState.Hosts()
	agentRequests := make([]monitoring.GuestChartRequest, 0, len(hostList))
	for _, h := range hostList {
		_, request, ok := hostAgentChartRequest(h)
		if !ok {
			continue
		}
		agentRequests = append(agentRequests, request)
	}
	agentBatchMetrics := monitor.GetGuestMetricsForChartBatch("agent", agentRequests, duration, requestedMetricNames...)
	for _, h := range hostList {
		hID, request, ok := hostAgentChartRequest(h)
		if !ok {
			continue
		}
		agentData[hID] = make(VMChartData)
		if batchMetrics, ok := agentBatchMetrics[request.SQLResourceID]; ok {
			for metricType, points := range batchMetrics {
				if !requestedMetrics[metricType] {
					continue
				}
				agentData[hID][metricType] = make([]MetricPoint, len(points))
				for i, point := range points {
					ts := point.Timestamp.UnixMilli()
					if ts < oldestTimestamp {
						oldestTimestamp = ts
					}
					agentData[hID][metricType][i] = MetricPoint{
						Timestamp: ts,
						Value:     point.Value,
					}
				}
			}
		}
		for _, metricType := range requestedMetricNames {
			if len(agentData[hID][metricType]) > 0 {
				continue
			}
			var value float64
			hasFallbackValue := true
			switch metricType {
			case "cpu":
				value = h.CPUPercent()
			case "memory":
				value = h.MemoryPercent()
			case "disk":
				value = h.DiskPercent()
			default:
				hasFallbackValue = false
			}
			if hasFallbackValue {
				agentData[hID][metricType] = []MetricPoint{{Timestamp: currentTime, Value: value}}
			}
		}
		normalizeInfrastructureSummaryChartSeries(agentData[hID], duration, currentTime)
	}

	countNodePoints := func(metricsMap map[string]NodeChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}
	countChartPoints := func(metricsMap map[string]VMChartData) int {
		total := 0
		for _, metricSeries := range metricsMap {
			for _, points := range metricSeries {
				total += len(points)
			}
		}
		return total
	}

	nodePoints := countNodePoints(nodeData)
	dockerHostPoints := countChartPoints(dockerHostData)
	agentPoints := countChartPoints(agentData)

	response := EmptyInfrastructureChartsResponse()
	response.NodeData = nodeData
	response.DockerHostData = dockerHostData
	response.AgentData = agentData
	response.Timestamp = currentTime
	response.Stats = ChartStats{
		OldestDataTimestamp:   oldestTimestamp,
		Range:                 timeRange,
		RangeSeconds:          int64(duration / time.Second),
		MetricsStoreEnabled:   metricsStoreEnabled,
		PrimarySourceHint:     primarySourceHint,
		InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
		PointCounts: ChartPointCounts{
			Total:       nodePoints + dockerHostPoints + agentPoints,
			Nodes:       nodePoints,
			DockerHosts: dockerHostPoints,
			Agents:      agentPoints,
		},
	}

	payload, err := json.Marshal(response.NormalizeCollections())
	if err != nil {
		log.Error().Err(err).Msg("Failed to encode infrastructure chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	payload = append(payload, '\n')
	r.cacheInfrastructureChartsPayload(cacheKey, payload, now)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(payload); err != nil {
		log.Error().Err(err).Msg("Failed to write infrastructure chart data response")
	}
}

type workloadSummaryBuckets struct {
	cpu     workloadSummaryMetricBucket
	memory  workloadSummaryMetricBucket
	disk    workloadSummaryMetricBucket
	network workloadSummaryMetricBucket
}

type workloadSummaryMetricBucket struct {
	sum   float64
	max   float64
	count int
}

func (bucket *workloadSummaryMetricBucket) add(value float64) {
	if bucket == nil {
		return
	}
	if bucket.count == 0 || value > bucket.max {
		bucket.max = value
	}
	bucket.sum += value
	bucket.count++
}

func (bucket workloadSummaryMetricBucket) average() float64 {
	if bucket.count == 0 {
		return 0
	}
	return bucket.sum / float64(bucket.count)
}

type workloadsSummarySnapshot struct {
	id      string
	name    string
	cpu     float64
	memory  float64
	disk    float64
	network float64
}

func workloadSummaryBucketTimestamp(timestampMs int64) int64 {
	const bucketSizeMs = int64(30_000)
	return (timestampMs / bucketSizeMs) * bucketSizeMs
}

func clampWorkloadPercent(value float64) float64 {
	if value != value {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func proxmoxModelCPURatioPercent(value float64) float64 {
	return clampWorkloadPercent(value * 100)
}

func ProxmoxModelCPURatioPercent(value float64) float64 {
	return proxmoxModelCPURatioPercent(value)
}

func clampNonNegativeWorkloadValue(value float64) float64 {
	if value != value {
		return 0
	}
	if value < 0 {
		return 0
	}
	return value
}

func kubernetesPodMetricIDFromView(pod *unifiedresources.PodView) string {
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
			podKey = fmt.Sprintf("%s/%s", namespace, name)
		}
	}
	if clusterKey == "" || podKey == "" {
		return ""
	}
	return fmt.Sprintf("k8s:%s:pod:%s", clusterKey, podKey)
}

func getOrCreateWorkloadBucket(buckets map[int64]*workloadSummaryBuckets, bucketTs int64) *workloadSummaryBuckets {
	if bucket, ok := buckets[bucketTs]; ok {
		return bucket
	}
	bucket := &workloadSummaryBuckets{}
	buckets[bucketTs] = bucket
	return bucket
}

func appendWorkloadMetricPoints(
	buckets map[int64]*workloadSummaryBuckets,
	points []monitoring.MetricPoint,
	target string,
	oldestTimestamp *int64,
) int {
	added := 0
	for _, point := range points {
		ts := point.Timestamp.UnixMilli()
		if ts <= 0 {
			continue
		}
		if ts < *oldestTimestamp {
			*oldestTimestamp = ts
		}
		bucketTs := workloadSummaryBucketTimestamp(ts)
		bucket := getOrCreateWorkloadBucket(buckets, bucketTs)
		value := clampNonNegativeWorkloadValue(point.Value)
		switch target {
		case "cpu", "memory", "disk":
			value = clampWorkloadPercent(value)
		}
		switch target {
		case "cpu":
			bucket.cpu.add(value)
		case "memory":
			bucket.memory.add(value)
		case "disk":
			bucket.disk.add(value)
		case "network":
			bucket.network.add(value)
		}
		added++
	}
	return added
}

func mergeWorkloadNetworkPoints(
	netIn []monitoring.MetricPoint,
	netOut []monitoring.MetricPoint,
) []monitoring.MetricPoint {
	totals := make(map[int64]float64)
	for _, point := range netIn {
		ts := point.Timestamp.UnixMilli()
		if ts <= 0 {
			continue
		}
		totals[ts] += clampNonNegativeWorkloadValue(point.Value)
	}
	for _, point := range netOut {
		ts := point.Timestamp.UnixMilli()
		if ts <= 0 {
			continue
		}
		totals[ts] += clampNonNegativeWorkloadValue(point.Value)
	}
	if len(totals) == 0 {
		return nil
	}
	keys := make([]int64, 0, len(totals))
	for ts := range totals {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	points := make([]monitoring.MetricPoint, 0, len(keys))
	for _, ts := range keys {
		points = append(points, monitoring.MetricPoint{
			Timestamp: time.UnixMilli(ts),
			Value:     totals[ts],
		})
	}
	return points
}

func buildWorkloadsSummaryMetric(
	buckets map[int64]*workloadSummaryBuckets,
	selector func(*workloadSummaryBuckets) workloadSummaryMetricBucket,
) WorkloadsSummaryMetricData {
	keys := make([]int64, 0, len(buckets))
	for ts := range buckets {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	data := WorkloadsSummaryMetricData{
		P50: make([]MetricPoint, 0, len(keys)),
		P95: make([]MetricPoint, 0, len(keys)),
	}
	for _, ts := range keys {
		bucket := selector(buckets[ts])
		if bucket.count == 0 {
			continue
		}
		data.P50 = append(data.P50, MetricPoint{
			Timestamp: ts,
			Value:     bucket.average(),
		})
		data.P95 = append(data.P95, MetricPoint{
			Timestamp: ts,
			Value:     bucket.max,
		})
	}
	return data
}

func summaryMetricPointCount(metric WorkloadsSummaryMetricData) int {
	return len(metric.P50) + len(metric.P95)
}

func normalizeWorkloadsSummaryMetricPointSeries(
	metric WorkloadsSummaryMetricData,
	duration time.Duration,
) WorkloadsSummaryMetricData {
	targetPoints := targetWorkloadsSummarySeriesPoints(duration)
	metric.P50 = capMetricPointSeries(metric.P50, targetPoints)
	metric.P95 = capMetricPointSeries(metric.P95, targetPoints)
	return metric
}

func latestSummaryMetricValue(points []monitoring.MetricPoint, fallback float64, clamp func(float64) float64) float64 {
	if len(points) == 0 {
		return clamp(fallback)
	}

	latest := points[0]
	for i := 1; i < len(points); i++ {
		if points[i].Timestamp.After(latest.Timestamp) {
			latest = points[i]
		}
	}
	return clamp(latest.Value)
}

func buildWorkloadsTopContributors(
	snapshots []workloadsSummarySnapshot,
	selector func(workloadsSummarySnapshot) float64,
) []WorkloadsSummaryContributor {
	contributors := make([]WorkloadsSummaryContributor, 0, len(snapshots))
	for _, snapshot := range snapshots {
		value := selector(snapshot)
		if value <= 0 {
			continue
		}
		contributors = append(contributors, WorkloadsSummaryContributor{
			ID:    snapshot.id,
			Name:  snapshot.name,
			Value: value,
		})
	}

	sort.Slice(contributors, func(i, j int) bool {
		if contributors[i].Value == contributors[j].Value {
			if contributors[i].Name == contributors[j].Name {
				return contributors[i].ID < contributors[j].ID
			}
			return contributors[i].Name < contributors[j].Name
		}
		return contributors[i].Value > contributors[j].Value
	})

	if len(contributors) > 3 {
		contributors = contributors[:3]
	}
	return contributors
}

func buildWorkloadsBlastRadius(
	snapshots []workloadsSummarySnapshot,
	selector func(workloadsSummarySnapshot) float64,
) WorkloadsSummaryBlastRadius {
	values := make([]float64, 0, len(snapshots))
	for _, snapshot := range snapshots {
		value := selector(snapshot)
		if value <= 0 {
			continue
		}
		values = append(values, value)
	}

	if len(values) == 0 {
		return WorkloadsSummaryBlastRadius{
			Scope:           "idle",
			Top3Share:       0,
			ActiveWorkloads: 0,
		}
	}

	sort.Slice(values, func(i, j int) bool { return values[i] > values[j] })
	total := 0.0
	for _, value := range values {
		total += value
	}

	topCount := 3
	if len(values) < topCount {
		topCount = len(values)
	}
	top3 := 0.0
	for i := 0; i < topCount; i++ {
		top3 += values[i]
	}

	share := 0.0
	if total > 0 {
		share = (top3 / total) * 100
	}

	scope := "distributed"
	switch {
	case share >= 80:
		scope = "concentrated"
	case share >= 55:
		scope = "mixed"
	}

	return WorkloadsSummaryBlastRadius{
		Scope:           scope,
		Top3Share:       share,
		ActiveWorkloads: len(values),
	}
}

// handleWorkloadsSummaryCharts serves compact, aggregate workload sparklines
// for the Workloads top cards. It intentionally avoids returning per-workload
// time series to keep payloads bounded for large fleets.
func (r *Service) HandleWorkloadsSummaryCharts(w http.ResponseWriter, req *http.Request) {
	log.Debug().Str("method", req.Method).Str("url", req.URL.String()).Msg("Workloads summary charts endpoint hit")
	const inMemoryChartThreshold = 2 * time.Hour

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "1h"
	}
	selectedNodeID := strings.TrimSpace(query.Get("node"))
	duration := parseChartsRangeDuration(timeRange)

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Tenant monitor is not available", http.StatusInternalServerError)
		return
	}
	nodes := monitor.NodesSnapshot()
	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		http.Error(w, "State unavailable", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	cacheKey := workloadsSummaryChartsCacheKey(req, timeRange, selectedNodeID)
	if payload, ok := r.cachedWorkloadsSummaryChartsPayload(cacheKey, now); ok {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
		return
	}

	mockModeEnabled := mock.IsMockEnabled()
	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

	currentTime := now.UnixMilli()
	currentTimeTime := now
	oldestTimestamp := currentTime
	buckets := make(map[int64]*workloadSummaryBuckets)
	guestPointCount := 0
	guestCounts := WorkloadsGuestCounts{}
	snapshots := make([]workloadsSummarySnapshot, 0, len(readState.VMs())+len(readState.Containers()))

	var selectedNode *models.Node
	if selectedNodeID != "" {
		for idx := range nodes {
			if nodes[idx].ID == selectedNodeID {
				selectedNode = &nodes[idx]
				break
			}
		}
		if selectedNode == nil {
			log.Debug().
				Str("selectedNodeID", selectedNodeID).
				Msg("Workloads summary node filter not found in current state; falling back to global scope")
		}
	}

	matchesSelectedNode := func(instance, nodeName string) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		return strings.EqualFold(strings.TrimSpace(instance), strings.TrimSpace(selectedNode.Instance)) &&
			strings.EqualFold(strings.TrimSpace(nodeName), strings.TrimSpace(selectedNode.Name))
	}

	matchesSelectedDockerHostView := func(host *unifiedresources.DockerHostView) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		if host == nil {
			return false
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(host.Hostname()), nodeName) ||
			strings.EqualFold(strings.TrimSpace(host.Name()), nodeName)
	}

	matchesSelectedKubernetesPodView := func(pod *unifiedresources.PodView) bool {
		if selectedNodeID == "" {
			return true
		}
		if selectedNode == nil {
			return true
		}
		if pod == nil {
			return false
		}
		nodeName := strings.TrimSpace(selectedNode.Name)
		if nodeName == "" {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(pod.NodeName()), nodeName)
	}

	vmList := make([]*unifiedresources.VMView, 0)
	vmResponseKeys := make([]string, 0)
	vmRequests := make([]monitoring.GuestChartRequest, 0)
	for _, vm := range readState.VMs() {
		if vm == nil {
			continue
		}
		if !matchesSelectedNode(vm.Instance(), vm.Node()) {
			continue
		}
		responseKey, request, ok := vmChartRequest(vm)
		if !ok {
			continue
		}
		vmList = append(vmList, vm)
		vmResponseKeys = append(vmResponseKeys, responseKey)
		vmRequests = append(vmRequests, request)
	}
	containerList := make([]*unifiedresources.ContainerView, 0)
	containerResponseKeys := make([]string, 0)
	containerRequests := make([]monitoring.GuestChartRequest, 0)
	for _, ct := range readState.Containers() {
		if ct == nil {
			continue
		}
		if !matchesSelectedNode(ct.Instance(), ct.Node()) {
			continue
		}
		responseKey, request, ok := systemContainerChartRequest(ct)
		if !ok {
			continue
		}
		containerList = append(containerList, ct)
		containerResponseKeys = append(containerResponseKeys, responseKey)
		containerRequests = append(containerRequests, request)
	}
	podList := make([]*unifiedresources.PodView, 0)
	podRequests := make([]monitoring.GuestChartRequest, 0)
	for _, pod := range readState.Pods() {
		if pod == nil {
			continue
		}
		if !matchesSelectedKubernetesPodView(pod) {
			continue
		}

		metricKey := kubernetesPodMetricIDFromView(pod)
		if metricKey == "" {
			continue
		}
		podList = append(podList, pod)
		podRequests = append(podRequests, monitoring.GuestChartRequest{InMemoryKey: metricKey, SQLResourceID: metricKey})
	}
	dockerHostsByID := make(map[string]*unifiedresources.DockerHostView, len(readState.DockerHosts()))
	for _, host := range readState.DockerHosts() {
		if host == nil {
			continue
		}
		dockerHostsByID[host.ID()] = host
	}

	dockerContainerList := make([]*unifiedresources.DockerContainerView, 0)
	dockerContainerRequests := make([]monitoring.GuestChartRequest, 0)
	for _, container := range readState.DockerContainers() {
		if container == nil {
			continue
		}
		if selectedNodeID != "" && selectedNode != nil {
			host := dockerHostsByID[container.ParentID()]
			if host == nil || !matchesSelectedDockerHostView(host) {
				continue
			}
		}
		containerID := strings.TrimSpace(container.ContainerID())
		if containerID == "" {
			continue
		}
		dockerContainerList = append(dockerContainerList, container)
		dockerContainerRequests = append(dockerContainerRequests, monitoring.GuestChartRequest{
			InMemoryKey:   fmt.Sprintf("docker:%s", containerID),
			SQLResourceID: containerID,
		})
	}
	var (
		vmBatchMetrics              map[string]map[string][]monitoring.MetricPoint
		containerBatchMetrics       map[string]map[string][]monitoring.MetricPoint
		podBatchMetrics             map[string]map[string][]monitoring.MetricPoint
		dockerContainerBatchMetrics map[string]map[string][]monitoring.MetricPoint
	)
	var workloadsSummaryBatchWG sync.WaitGroup
	workloadsSummaryBatchWG.Add(4)
	go func() {
		defer workloadsSummaryBatchWG.Done()
		vmBatchMetrics = monitor.GetGuestMetricsForChartBatch("vm", vmRequests, duration, workloadSummaryMetricOrder...)
	}()
	go func() {
		defer workloadsSummaryBatchWG.Done()
		containerBatchMetrics = monitor.GetGuestMetricsForChartBatch("container", containerRequests, duration, workloadSummaryMetricOrder...)
	}()
	go func() {
		defer workloadsSummaryBatchWG.Done()
		podBatchMetrics = monitor.GetGuestMetricsForChartBatch("k8s", podRequests, duration, workloadSummaryMetricOrder...)
	}()
	go func() {
		defer workloadsSummaryBatchWG.Done()
		dockerContainerBatchMetrics = monitor.GetGuestMetricsForChartBatch("dockerContainer", dockerContainerRequests, duration, workloadSummaryMetricOrder...)
	}()
	workloadsSummaryBatchWG.Wait()

	var guestSummaryPoints int
	snapshots, guestSummaryPoints = appendGuestWorkloadSummaries(vmList, vmResponseKeys, vmRequests, vmBatchMetrics, currentTimeTime, &guestCounts, buckets, snapshots, &oldestTimestamp)
	guestPointCount += guestSummaryPoints

	snapshots, guestSummaryPoints = appendGuestWorkloadSummaries(containerList, containerResponseKeys, containerRequests, containerBatchMetrics, currentTimeTime, &guestCounts, buckets, snapshots, &oldestTimestamp)
	guestPointCount += guestSummaryPoints

	for _, pod := range podList {
		metricKey := kubernetesPodMetricIDFromView(pod)

		guestCounts.Total++
		if strings.EqualFold(pod.PodPhase(), "running") {
			guestCounts.Running++
		} else {
			guestCounts.Stopped++
		}

		snapshot := workloadsSummarySnapshot{
			id:      metricKey,
			name:    strings.TrimSpace(pod.Namespace()),
			cpu:     clampWorkloadPercent(pod.CPUPercent()),
			memory:  clampWorkloadPercent(pod.MemoryPercent()),
			disk:    clampWorkloadPercent(pod.DiskPercent()),
			network: clampNonNegativeWorkloadValue(pod.NetInRate() + pod.NetOutRate()),
		}
		if name := strings.TrimSpace(pod.Name()); name != "" {
			if snapshot.name == "" {
				snapshot.name = name
			} else {
				snapshot.name = fmt.Sprintf("%s/%s", snapshot.name, name)
			}
		}
		if snapshot.name == "" {
			snapshot.name = metricKey
		}

		metrics := podBatchMetrics[metricKey]
		cpuPoints := metrics["cpu"]
		if len(cpuPoints) == 0 {
			cpuPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: pod.CPUPercent()}}
		}
		memoryPoints := metrics["memory"]
		if len(memoryPoints) == 0 {
			memoryPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: pod.MemoryPercent()}}
		}
		diskPoints := metrics["disk"]
		if len(diskPoints) == 0 {
			diskPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: pod.DiskPercent()}}
		}
		netInPoints := metrics["netin"]
		if len(netInPoints) == 0 {
			netInPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: pod.NetInRate()}}
		}
		netOutPoints := metrics["netout"]
		if len(netOutPoints) == 0 {
			netOutPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: pod.NetOutRate()}}
		}

		if mockModeEnabled {
			if len(cpuPoints) < mockWorkloadMinSeriesPoints {
				cpuPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, "k8s", metricKey, "cpu", snapshot.cpu)
			}
			if len(memoryPoints) < mockWorkloadMinSeriesPoints {
				memoryPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, "k8s", metricKey, "memory", snapshot.memory)
			}
			if len(diskPoints) < mockWorkloadMinSeriesPoints {
				diskPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, "k8s", metricKey, "disk", snapshot.disk)
			}
			if len(netInPoints) < mockWorkloadMinSeriesPoints {
				netInPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, "k8s", metricKey, "netin", pod.NetInRate())
			}
			if len(netOutPoints) < mockWorkloadMinSeriesPoints {
				netOutPoints = buildMockWorkloadMetricHistorySeries(currentTimeTime, duration, 0, "k8s", metricKey, "netout", pod.NetOutRate())
			}
		}

		networkPoints := mergeWorkloadNetworkPoints(netInPoints, netOutPoints)

		snapshot.cpu = latestSummaryMetricValue(cpuPoints, snapshot.cpu, clampWorkloadPercent)
		snapshot.memory = latestSummaryMetricValue(memoryPoints, snapshot.memory, clampWorkloadPercent)
		snapshot.disk = latestSummaryMetricValue(diskPoints, snapshot.disk, clampWorkloadPercent)
		snapshot.network = latestSummaryMetricValue(networkPoints, snapshot.network, clampNonNegativeWorkloadValue)

		guestPointCount += appendWorkloadMetricPoints(buckets, cpuPoints, "cpu", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, memoryPoints, "memory", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, diskPoints, "disk", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, networkPoints, "network", &oldestTimestamp)
		snapshots = append(snapshots, snapshot)
	}

	for _, container := range dockerContainerList {
		containerID := strings.TrimSpace(container.ContainerID())
		guestCounts.Total++
		containerState := strings.TrimSpace(container.ContainerState())
		isRunning := workloadSummaryStatusIsRunning(containerState, container.Status())
		if !isRunning && containerState == "" {
			isRunning = container.CPUPercent() > 0 ||
				container.MemoryPercent() > 0 ||
				container.NetInRate() > 0 ||
				container.NetOutRate() > 0
		}
		if isRunning {
			guestCounts.Running++
		} else {
			guestCounts.Stopped++
		}

		snapshot := workloadsSummarySnapshot{
			id:      containerID,
			name:    strings.TrimSpace(container.Name()),
			cpu:     clampWorkloadPercent(container.CPUPercent()),
			memory:  clampWorkloadPercent(container.MemoryPercent()),
			disk:    clampWorkloadPercent(container.DiskPercent()),
			network: 0,
		}
		if snapshot.name == "" {
			snapshot.name = containerID
		}

		metrics := dockerContainerBatchMetrics[containerID]
		cpuPoints := metrics["cpu"]
		if len(cpuPoints) == 0 {
			cpuPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: container.CPUPercent()}}
		}
		memoryPoints := metrics["memory"]
		if len(memoryPoints) == 0 {
			memoryPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: container.MemoryPercent()}}
		}
		diskPoints := metrics["disk"]
		if len(diskPoints) == 0 {
			diskPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: container.DiskPercent()}}
		}
		netInPoints := metrics["netin"]
		netOutPoints := metrics["netout"]

		networkPoints := mergeWorkloadNetworkPoints(netInPoints, netOutPoints)

		snapshot.cpu = latestSummaryMetricValue(cpuPoints, snapshot.cpu, clampWorkloadPercent)
		snapshot.memory = latestSummaryMetricValue(memoryPoints, snapshot.memory, clampWorkloadPercent)
		snapshot.disk = latestSummaryMetricValue(diskPoints, snapshot.disk, clampWorkloadPercent)
		snapshot.network = latestSummaryMetricValue(networkPoints, snapshot.network, clampNonNegativeWorkloadValue)

		guestPointCount += appendWorkloadMetricPoints(buckets, cpuPoints, "cpu", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, memoryPoints, "memory", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, diskPoints, "disk", &oldestTimestamp)
		guestPointCount += appendWorkloadMetricPoints(buckets, networkPoints, "network", &oldestTimestamp)
		snapshots = append(snapshots, snapshot)
	}

	cpuMetric := buildWorkloadsSummaryMetric(buckets, func(bucket *workloadSummaryBuckets) workloadSummaryMetricBucket {
		return bucket.cpu
	})
	memoryMetric := buildWorkloadsSummaryMetric(buckets, func(bucket *workloadSummaryBuckets) workloadSummaryMetricBucket {
		return bucket.memory
	})
	diskMetric := buildWorkloadsSummaryMetric(buckets, func(bucket *workloadSummaryBuckets) workloadSummaryMetricBucket {
		return bucket.disk
	})
	networkMetric := buildWorkloadsSummaryMetric(buckets, func(bucket *workloadSummaryBuckets) workloadSummaryMetricBucket {
		return bucket.network
	})
	cpuMetric = normalizeWorkloadsSummaryMetricPointSeries(cpuMetric, duration)
	memoryMetric = normalizeWorkloadsSummaryMetricPointSeries(memoryMetric, duration)
	diskMetric = normalizeWorkloadsSummaryMetricPointSeries(diskMetric, duration)
	networkMetric = normalizeWorkloadsSummaryMetricPointSeries(networkMetric, duration)

	summaryPointCount := summaryMetricPointCount(cpuMetric) +
		summaryMetricPointCount(memoryMetric) +
		summaryMetricPointCount(diskMetric) +
		summaryMetricPointCount(networkMetric)

	topContributors := WorkloadsSummaryContributors{
		CPU: buildWorkloadsTopContributors(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.cpu
		}),
		Memory: buildWorkloadsTopContributors(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.memory
		}),
		Disk: buildWorkloadsTopContributors(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.disk
		}),
		Network: buildWorkloadsTopContributors(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.network
		}),
	}

	blastRadius := WorkloadsSummaryBlastRadiusGroup{
		CPU: buildWorkloadsBlastRadius(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.cpu
		}),
		Memory: buildWorkloadsBlastRadius(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.memory
		}),
		Disk: buildWorkloadsBlastRadius(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.disk
		}),
		Network: buildWorkloadsBlastRadius(snapshots, func(snapshot workloadsSummarySnapshot) float64 {
			return snapshot.network
		}),
	}

	response := EmptyWorkloadsSummaryChartsResponse()
	response.CPU = cpuMetric
	response.Memory = memoryMetric
	response.Disk = diskMetric
	response.Network = networkMetric
	response.GuestCounts = guestCounts
	response.TopContributors = topContributors
	response.BlastRadius = blastRadius
	response.Timestamp = currentTime
	response.Stats = ChartStats{
		OldestDataTimestamp:   oldestTimestamp,
		Range:                 timeRange,
		RangeSeconds:          int64(duration / time.Second),
		MetricsStoreEnabled:   metricsStoreEnabled,
		PrimarySourceHint:     primarySourceHint,
		InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
		PointCounts: ChartPointCounts{
			Total:  summaryPointCount,
			Guests: guestPointCount,
		},
	}

	payload, err := json.Marshal(response.NormalizeCollections())
	if err != nil {
		log.Error().Err(err).Msg("Failed to encode workloads summary chart data response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	r.cacheWorkloadsSummaryChartsPayload(cacheKey, payload, now)

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(payload); err != nil {
		log.Error().Err(err).Msg("Failed to encode workloads summary chart data response")
		return
	}
}

// guestWorkloadSummaryView is the guest view subset the workloads summary
// loop consumes from VMs and LXC containers.
type guestWorkloadSummaryView interface {
	Status() unifiedresources.ResourceStatus
	Name() string
	CPUPercent() float64
	MemoryPercent() float64
	DiskPercent() float64
	NetIn() float64
	NetOut() float64
}

// appendGuestWorkloadSummaries accumulates workload-summary snapshots and
// chart points for one proxmox guest family (VMs or LXC containers),
// returning the extended snapshot slice and the number of points added.
func appendGuestWorkloadSummaries[V guestWorkloadSummaryView](
	guests []V,
	responseKeys []string,
	requests []monitoring.GuestChartRequest,
	batchMetrics map[string]map[string][]monitoring.MetricPoint,
	currentTimeTime time.Time,
	guestCounts *WorkloadsGuestCounts,
	buckets map[int64]*workloadSummaryBuckets,
	snapshots []workloadsSummarySnapshot,
	oldestTimestamp *int64,
) ([]workloadsSummarySnapshot, int) {
	added := 0
	for idx, g := range guests {
		responseKey := responseKeys[idx]
		metricID := requests[idx].SQLResourceID
		guestCounts.Total++
		if workloadSummaryStatusIsRunning("", g.Status()) {
			guestCounts.Running++
		} else {
			guestCounts.Stopped++
		}

		snapshot := workloadsSummarySnapshot{
			id:      responseKey,
			name:    strings.TrimSpace(g.Name()),
			cpu:     clampWorkloadPercent(g.CPUPercent()),
			memory:  clampWorkloadPercent(g.MemoryPercent()),
			disk:    clampWorkloadPercent(g.DiskPercent()),
			network: clampNonNegativeWorkloadValue(g.NetIn() + g.NetOut()),
		}
		if snapshot.name == "" {
			snapshot.name = responseKey
		}

		metrics := batchMetrics[metricID]
		cpuPoints := metrics["cpu"]
		if len(cpuPoints) == 0 {
			cpuPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: g.CPUPercent()}}
		}
		memoryPoints := metrics["memory"]
		if len(memoryPoints) == 0 {
			memoryPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: g.MemoryPercent()}}
		}
		diskPoints := metrics["disk"]
		if len(diskPoints) == 0 {
			diskPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: g.DiskPercent()}}
		}
		netInPoints := metrics["netin"]
		netOutPoints := metrics["netout"]
		if len(netInPoints) == 0 && len(netOutPoints) == 0 {
			netInPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: g.NetIn()}}
			netOutPoints = []monitoring.MetricPoint{{Timestamp: currentTimeTime, Value: g.NetOut()}}
		}

		networkPoints := mergeWorkloadNetworkPoints(netInPoints, netOutPoints)

		snapshot.cpu = latestSummaryMetricValue(cpuPoints, snapshot.cpu, clampWorkloadPercent)
		snapshot.memory = latestSummaryMetricValue(memoryPoints, snapshot.memory, clampWorkloadPercent)
		snapshot.disk = latestSummaryMetricValue(diskPoints, snapshot.disk, clampWorkloadPercent)
		snapshot.network = latestSummaryMetricValue(networkPoints, snapshot.network, clampNonNegativeWorkloadValue)

		added += appendWorkloadMetricPoints(buckets, cpuPoints, "cpu", oldestTimestamp)
		added += appendWorkloadMetricPoints(buckets, memoryPoints, "memory", oldestTimestamp)
		added += appendWorkloadMetricPoints(buckets, diskPoints, "disk", oldestTimestamp)
		added += appendWorkloadMetricPoints(buckets, networkPoints, "network", oldestTimestamp)
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, added
}

func workloadSummaryStatusIsRunning(runtimeState string, status unifiedresources.ResourceStatus) bool {
	switch strings.ToLower(strings.TrimSpace(runtimeState)) {
	case "running", "online", "ok":
		return true
	case "stopped", "offline", "paused", "created", "dead", "exited":
		return false
	}

	switch status {
	case unifiedresources.StatusOnline:
		return true
	case unifiedresources.StatusWarning:
		// Warning is an attention state on a running workload (degraded
		// guest state, stale source data); power-off maps to StatusOffline.
		return true
	case unifiedresources.StatusOffline:
		return false
	}

	return false
}

// handleStorageCharts returns pool capacity and physical disk temperature
// time-series for the storage summary sparklines.
func (r *Service) HandleStorageCharts(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters
	query := req.URL.Query()
	rangeMinutes := 60 // default 1 hour
	if rangeStr := query.Get("range"); rangeStr != "" {
		if _, err := fmt.Sscanf(rangeStr, "%d", &rangeMinutes); err != nil {
			log.Warn().Err(err).Str("range", rangeStr).Msg("Invalid range parameter; using default")
		}
	}

	duration := time.Duration(rangeMinutes) * time.Minute
	selectedNodeID := strings.TrimSpace(query.Get("node"))

	// Use tenant-aware monitor
	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Monitor not available", http.StatusInternalServerError)
		return
	}
	readState := monitor.GetUnifiedReadStateOrSnapshot()
	if readState == nil {
		http.Error(w, "State unavailable", http.StatusInternalServerError)
		return
	}

	// Resolve node filter from canonical unified resources so storage charts use
	// the same node identity model as the frontend storage page.
	var selectedNodeName, selectedNodeInstance string
	if selectedNodeID != "" {
		found := false
		for _, resource := range monitor.GetUnifiedResources() {
			if strings.TrimSpace(resource.ID) != selectedNodeID {
				continue
			}
			selectedNodeName = storageChartsSelectedNodeName(resource)
			selectedNodeInstance = storageChartsSelectedNodeInstance(resource)
			if selectedNodeName != "" || selectedNodeInstance != "" {
				found = true
				break
			}
		}
		if !found {
			for _, n := range monitor.NodesSnapshot() {
				if n.ID == selectedNodeID {
					selectedNodeName = n.Name
					selectedNodeInstance = n.Instance
					found = true
					break
				}
			}
		}
		if !found {
			log.Debug().
				Str("selectedNodeID", selectedNodeID).
				Msg("Storage charts node filter not found in current state; falling back to global scope")
		}
	}
	matchesNode := func(nodeName, instance string) bool {
		if selectedNodeName == "" {
			return true
		}
		if !strings.EqualFold(strings.TrimSpace(nodeName), selectedNodeName) {
			return false
		}
		if selectedNodeInstance != "" && instance != "" {
			return strings.EqualFold(strings.TrimSpace(instance), selectedNodeInstance)
		}
		return true
	}

	// Build pool chart data from the canonical storage summary batch path so
	// the dashboard and storage page share one efficient history retrieval model.
	poolNames := make(map[string]string, len(readState.StoragePools()))
	storageIDs := make([]string, 0, len(readState.StoragePools()))
	for _, sp := range readState.StoragePools() {
		if sp == nil {
			continue
		}
		if !matchesNode(sp.Node(), sp.Instance()) {
			continue
		}
		sid := sp.SourceID()
		if sid == "" {
			continue
		}
		poolNames[sid] = sp.Name()
		storageIDs = append(storageIDs, sid)
	}

	poolMetrics := monitor.GetStorageMetricsForChartBatch(storageIDs, duration)
	pools := make(map[string]StoragePoolChartData, len(storageIDs))
	for _, sid := range storageIDs {
		metrics := poolMetrics[sid]
		pools[sid] = StoragePoolChartData{
			Name:  poolNames[sid],
			Usage: monitorPointsToAPI(metrics["usage"]),
			Used:  monitorPointsToAPI(metrics["used"]),
			Avail: monitorPointsToAPI(metrics["avail"]),
		}
	}

	// Build disk temperature chart data
	diskEntries := monitor.GetPhysicalDiskTemperatureCharts(duration)
	disks := make(map[string]StorageDiskChartData, len(diskEntries))
	for id, entry := range diskEntries {
		if !matchesNode(entry.Node, entry.Instance) {
			continue
		}
		disks[id] = StorageDiskChartData{
			Name:        entry.Name,
			Node:        entry.Node,
			Temperature: monitorPointsToAPI(entry.Temperature),
		}
	}

	resp := EmptyStorageChartsResponse()
	resp.Pools = pools
	resp.Disks = disks

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp.NormalizeCollections()); err != nil {
		log.Error().Err(err).Msg("Failed to encode storage chart data")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleStorageSummaryCharts serves a compact aggregate capacity trend for the
// dashboard storage card. It intentionally avoids returning per-pool and
// per-disk series so the dashboard does not overfetch the full storage page
// payload.
func (r *Service) HandleStorageSummaryCharts(w http.ResponseWriter, req *http.Request) {
	const inMemoryChartThreshold = 2 * time.Hour

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := req.URL.Query()
	timeRange := query.Get("range")
	if timeRange == "" {
		timeRange = "24h"
	}
	duration := parseChartsRangeDuration(timeRange)

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		http.Error(w, "Tenant monitor is not available", http.StatusInternalServerError)
		return
	}

	currentTime := time.Now().UnixMilli()
	capacity, oldestTimestamp := monitor.GetStorageSummaryCapacityTrend(duration)
	if oldestTimestamp == 0 {
		oldestTimestamp = currentTime
	}

	metricsStoreEnabled := monitor.GetMetricsStore() != nil
	primarySourceHint := "memory"
	if metricsStoreEnabled && duration > inMemoryChartThreshold {
		primarySourceHint = "store_or_memory_fallback"
	}

	resp := EmptyStorageSummaryTrendResponse()
	resp.Capacity = monitorPointsToAPI(capacity)
	resp.Timestamp = currentTime
	resp.Stats = ChartStats{
		OldestDataTimestamp:   oldestTimestamp,
		Range:                 timeRange,
		RangeSeconds:          int64(duration / time.Second),
		MetricsStoreEnabled:   metricsStoreEnabled,
		PrimarySourceHint:     primarySourceHint,
		InMemoryThresholdSecs: int64(inMemoryChartThreshold / time.Second),
		PointCounts: ChartPointCounts{
			Total:   len(resp.Capacity),
			Storage: len(resp.Capacity),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp.NormalizeCollections()); err != nil {
		log.Error().Err(err).Msg("Failed to encode storage summary chart data")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// monitorPointsToAPI converts monitoring MetricPoints (time.Time timestamps)
// to API MetricPoints (Unix millisecond timestamps) for JSON serialization.
func monitorPointsToAPI(points []monitoring.MetricPoint) []MetricPoint {
	if len(points) == 0 {
		return nil
	}
	out := make([]MetricPoint, len(points))
	for i, p := range points {
		out[i] = MetricPoint{Timestamp: p.Timestamp.UnixMilli(), Value: p.Value}
	}
	return out
}

func MonitorPointsToAPI(points []monitoring.MetricPoint) []MetricPoint {
	return monitorPointsToAPI(points)
}
