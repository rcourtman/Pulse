package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// registerMetricsTools registers the pulse_metrics tool
func (e *PulseToolExecutor) registerMetricsTools() {
	e.registry.registerBuiltin(RegisteredTool{
		Definition: Tool{
			Name: agentcapabilities.PulseMetricsToolName,
			Description: `Get performance metrics, baselines, and sensor data.

Types:
- performance: Historical CPU/memory/disk metrics over 24h or 7d
- temperatures: CPU, disk, and sensor temperatures from hosts
- network: Network interface statistics (rx/tx bytes, speed)
- diskio: Disk I/O statistics (read/write bytes, ops)
- disks: Physical disk health (SMART, wearout, temperatures)
- baselines: Learned normal behavior baselines for resources
- patterns: Detected operational patterns and predictions

Examples:
- Get 24h metrics: type="performance", period="24h"
- Get VM metrics: type="performance", resource_id="101"
- Get host temps: type="temperatures", host="pve01"
- Get disk health: type="disks", node="pve01"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Metric type to query",
						Enum:        []string{"performance", "temperatures", "network", "diskio", "disks", "baselines", "patterns"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Filter by specific resource ID (for performance, temperatures, baselines)",
					},
					"resource_type": {
						Type:        "string",
						Description: "Filter by resource type: agent, vm, system-container (performance). The node alias is also accepted. Baselines accept vm, system-container, node.",
					},
					"host": {
						Type:        "string",
						Description: "Filter by hostname (for temperatures, network, diskio)",
					},
					"node": {
						Type:        "string",
						Description: "Filter by Proxmox node (for disks)",
					},
					"instance": {
						Type:        "string",
						Description: "Filter by Proxmox instance (for disks)",
					},
					"period": {
						Type:        "string",
						Description: "Time period for performance: 24h or 7d (default: 24h)",
						Enum:        []string{"24h", "7d"},
					},
					"health": {
						Type:        "string",
						Description: "Filter disks by health status: PASSED, FAILED, UNKNOWN",
					},
					"disk_type": {
						Type:        "string",
						Description: "Filter disks by type: nvme, sata, sas",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Number of results to skip",
					},
				},
				Required: []string{"type"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeMetrics(ctx, args)
		},
		Governance: ToolGovernance{
			ActionMode:      ToolActionRead,
			ApprovalPolicy:  ToolApprovalScopeOnly,
			ApprovalSummary: "no approval required",
			Summary:         "Reads performance, sensor, baseline, pattern, and disk-health data without changing state.",
		},
	})
}

// executeMetrics routes to the appropriate metrics handler based on type
func (e *PulseToolExecutor) executeMetrics(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	metricType, _ := args["type"].(string)
	switch metricType {
	case "performance":
		return e.executeGetMetrics(ctx, args)
	case "temperatures":
		return e.executeGetTemperatures(ctx, args)
	case "network":
		return e.executeGetNetworkStats(ctx, args)
	case "diskio":
		return e.executeGetDiskIOStats(ctx, args)
	case "disks":
		return e.executeListPhysicalDisks(ctx, args)
	case "baselines":
		return e.executeGetBaselines(ctx, args)
	case "patterns":
		return e.executeGetPatterns(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown type: %s. Use: performance, temperatures, network, diskio, disks, baselines, patterns", metricType)), nil
	}
}

func (e *PulseToolExecutor) executeGetMetrics(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	period, _ := args["period"].(string)
	resourceID, _ := args["resource_id"].(string)
	resourceType, _ := args["resource_type"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if resourceType != "" {
		if isLegacyMetricsResourceTypeInput(resourceType) {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use agent, vm, or system-container (node is an alias)", resourceType)), nil
		}
		resourceType = canonicalMetricsResourceType(resourceType)
		validTypes := map[string]bool{"agent": true, "vm": true, "system-container": true, "node": true}
		if !validTypes[resourceType] {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use agent, vm, or system-container (node is an alias)", resourceType)), nil
		}
	}

	if e.metricsHistory == nil {
		return NewTextResult("Metrics history not available. The system may still be collecting data."), nil
	}

	var duration time.Duration
	switch period {
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	default:
		duration = 24 * time.Hour
		period = "24h"
	}

	response := EmptyMetricsResponse()
	response.Period = period

	if resourceID != "" {
		response.ResourceID = resourceID
		metricsID := resourceID
		var metricsTarget *unifiedresources.MetricsTarget
		if e.unifiedResourceProvider != nil {
			resource, err := e.resolvePerformanceResource(resourceID, resourceType)
			if err != nil {
				return NewErrorResult(err), nil
			}
			response.ResourceID = resource.ID
			target := e.resourceMetricsTarget(resource)
			if target == nil || strings.TrimSpace(target.ResourceID) == "" {
				return NewTextResult(fmt.Sprintf("No metrics target is available for resource %s.", resource.ID)), nil
			}
			metricsID = target.ResourceID
			metricsTarget = target
		}
		var metrics []MetricPoint
		var err error
		if retained, ok := e.metricsHistory.(interface {
			GetResourceMetricsForTarget(unifiedresources.MetricsTarget, time.Duration) ([]MetricPoint, error)
		}); ok && metricsTarget != nil {
			metrics, err = retained.GetResourceMetricsForTarget(*metricsTarget, duration)
		} else {
			metrics, err = e.metricsHistory.GetResourceMetrics(metricsID, duration)
		}
		if err != nil {
			return NewErrorResult(err), nil
		}
		// Downsample to maxMetricPoints to prevent context window blowout.
		// 7d of per-minute data can be 10K-20K points (~1.6MB JSON).
		// 120 bucket-averaged points preserves trends while keeping output manageable.
		if len(metrics) > maxMetricPoints {
			response.OriginalCount = len(metrics)
			response.Downsampled = true
			metrics = downsampleMetrics(metrics, maxMetricPoints)
		}
		response.Points = metrics
		return NewJSONResult(response.NormalizeCollections()), nil
	}

	summary, err := e.metricsHistory.GetAllMetricsSummary(duration)
	if err != nil {
		return NewErrorResult(err), nil
	}

	keys := make([]string, 0, len(summary))
	for id, metric := range summary {
		metricType := canonicalMetricsResourceType(metric.ResourceType)
		if resourceType != "" && canonicalQueryResourceType(metricType) != canonicalQueryResourceType(resourceType) {
			continue
		}
		keys = append(keys, id)
	}
	sort.Strings(keys)

	filtered := make(map[string]ResourceMetricsSummary)
	total := 0
	for _, id := range keys {
		if total < offset {
			total++
			continue
		}
		if len(filtered) >= limit {
			total++
			continue
		}
		metric := summary[id]
		metric.ResourceType = canonicalMetricsResourceType(metric.ResourceType)
		filtered[id] = metric
		total++
	}

	if filtered == nil {
		filtered = map[string]ResourceMetricsSummary{}
	}

	response.Summary = filtered
	if offset > 0 || total > limit {
		response.Pagination = &PaginationInfo{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}
	}

	return NewJSONResult(response.NormalizeCollections()), nil
}

func (e *PulseToolExecutor) executeGetBaselines(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	resourceType, _ := args["resource_type"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if resourceType != "" {
		if isLegacyMetricsResourceTypeInput(resourceType) {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use vm, system-container, or node", resourceType)), nil
		}
		resourceType = canonicalMetricsResourceType(resourceType)
		validTypes := map[string]bool{"vm": true, "system-container": true, "node": true}
		if !validTypes[resourceType] {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use vm, system-container, or node", resourceType)), nil
		}
	}

	if e.baselineProvider == nil {
		return NewTextResult("Baseline data not available. The system needs time to learn normal behavior patterns."), nil
	}

	response := EmptyBaselinesResponse()

	if resourceID != "" {
		response.ResourceID = resourceID
		cpuBaseline := e.baselineProvider.GetBaseline(resourceID, "cpu")
		memBaseline := e.baselineProvider.GetBaseline(resourceID, "memory")

		if cpuBaseline != nil || memBaseline != nil {
			response.Baselines[resourceID] = make(map[string]*MetricBaseline)
			if cpuBaseline != nil {
				response.Baselines[resourceID]["cpu"] = cpuBaseline
			}
			if memBaseline != nil {
				response.Baselines[resourceID]["memory"] = memBaseline
			}
		}
		return NewJSONResult(response.NormalizeCollections()), nil
	}

	baselines := e.baselineProvider.GetAllBaselines()
	keys := make([]string, 0, len(baselines))
	var typeIndex map[string]string
	if resourceType != "" {
		typeIndex = make(map[string]string)
		rs, err := e.readStateForControl()
		if err != nil {
			return NewErrorResult(err), nil
		}
		for _, vm := range rs.VMs() {
			typeIndex[fmt.Sprintf("%d", vm.VMID())] = "vm"
		}
		for _, ct := range rs.Containers() {
			typeIndex[fmt.Sprintf("%d", ct.VMID())] = "system-container"
		}
		for _, node := range rs.Nodes() {
			if node.ID() != "" {
				typeIndex[node.ID()] = "node"
			}
		}
	}

	for id := range baselines {
		if resourceType != "" {
			if t, ok := typeIndex[id]; !ok || t != resourceType {
				continue
			}
		}
		keys = append(keys, id)
	}
	sort.Strings(keys)

	filtered := make(map[string]map[string]*MetricBaseline)
	total := 0
	for _, id := range keys {
		if total < offset {
			total++
			continue
		}
		if len(filtered) >= limit {
			total++
			continue
		}
		filtered[id] = baselines[id]
		total++
	}

	if filtered == nil {
		filtered = map[string]map[string]*MetricBaseline{}
	}

	response.Baselines = filtered
	if offset > 0 || total > limit {
		response.Pagination = &PaginationInfo{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}
	}

	return NewJSONResult(response.NormalizeCollections()), nil
}

func canonicalMetricsResourceType(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "container", "system-container":
		return "system-container"
	case "node":
		return "node"
	default:
		return normalized
	}
}

func isLegacyMetricsResourceTypeInput(resourceType string) bool {
	return strings.EqualFold(strings.TrimSpace(resourceType), "container")
}

func (e *PulseToolExecutor) executeGetPatterns(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.patternProvider == nil {
		return NewTextResult("Pattern detection not available. The system needs more historical data."), nil
	}

	response := EmptyPatternsResponse()
	response.Patterns = e.patternProvider.GetPatterns()
	response.Predictions = e.patternProvider.GetPredictions()

	return NewJSONResult(response.NormalizeCollections()), nil
}

// ========== Temperature, Network, DiskIO, Physical Disks ==========

func (e *PulseToolExecutor) executeGetTemperatures(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	hostFilter, _ := args["host"].(string)
	resourceFilter, _ := args["resource_id"].(string)
	hostFilter = strings.TrimSpace(hostFilter)
	resourceFilter = strings.TrimSpace(resourceFilter)

	rs, err := e.readStateForControl()
	if err != nil {
		return NewTextResult("State provider not available."), nil
	}

	type HostTemps struct {
		ResourceID  string             `json:"resource_id"`
		Source      string             `json:"source"`
		Hostname    string             `json:"hostname"`
		Platform    string             `json:"platform,omitempty"`
		CPU         map[string]float64 `json:"cpu_temps,omitempty"`
		Disks       map[string]float64 `json:"disk_temps,omitempty"`
		Fans        map[string]float64 `json:"fan_rpm,omitempty"`
		Other       map[string]float64 `json:"other_temps,omitempty"`
		LastUpdated string             `json:"last_updated,omitempty"`
	}

	var results []HostTemps

	for _, host := range rs.Hosts() {
		if host == nil {
			continue
		}

		hostname := strings.TrimSpace(host.Hostname())
		if hostname == "" {
			hostname = strings.TrimSpace(host.Name())
		}
		if (hostFilter != "" && hostname != hostFilter && host.Name() != hostFilter && host.ID() != hostFilter) ||
			(resourceFilter != "" && host.ID() != resourceFilter) {
			continue
		}

		sensors := host.Sensors()
		if sensors == nil {
			continue
		}
		if len(sensors.TemperatureCelsius) == 0 && len(sensors.FanRPM) == 0 && len(sensors.Additional) == 0 {
			continue
		}

		temps := HostTemps{
			ResourceID: host.ID(),
			Source:     "agent",
			Hostname:   hostname,
			Platform:   host.Platform(),
			CPU:        make(map[string]float64),
			Disks:      make(map[string]float64),
			Fans:       make(map[string]float64),
			Other:      make(map[string]float64),
		}

		// Categorize temperatures
		for name, value := range sensors.TemperatureCelsius {
			switch {
			case containsAny(name, "cpu", "core", "package"):
				temps.CPU[name] = value
			case containsAny(name, "nvme", "ssd", "hdd", "disk"):
				temps.Disks[name] = value
			default:
				temps.Other[name] = value
			}
		}

		// Add fan data
		for name, value := range sensors.FanRPM {
			temps.Fans[name] = value
		}

		// Add additional sensors to Other
		for name, value := range sensors.Additional {
			if _, exists := temps.CPU[name]; !exists {
				if _, exists := temps.Disks[name]; !exists {
					temps.Other[name] = value
				}
			}
		}

		results = append(results, temps)
	}

	// Both views come from the canonical resource registry. Keep observations
	// from linked providers distinct instead of silently choosing a reading or
	// merging sensors collected at different times.
	for _, node := range rs.Nodes() {
		if node == nil || (hostFilter != "" && node.Name() != hostFilter && node.NodeName() != hostFilter && node.ID() != hostFilter) ||
			(resourceFilter != "" && node.ID() != resourceFilter) {
			continue
		}
		details := node.TemperatureDetails()
		if !node.HasTemperature() && (details == nil || !details.Available) {
			continue
		}
		temps := HostTemps{
			ResourceID: node.ID(), Source: "proxmox", Hostname: node.NodeName(), Platform: "proxmox",
			CPU: make(map[string]float64), Disks: make(map[string]float64), Other: make(map[string]float64),
		}
		if temps.Hostname == "" {
			temps.Hostname = node.Name()
		}
		if node.HasTemperature() {
			temps.CPU["cpu_max"] = node.Temperature()
		}
		if details != nil && details.Available {
			if !details.LastUpdate.IsZero() {
				temps.LastUpdated = details.LastUpdate.Format(time.RFC3339)
			}
			if details.HasCPU {
				temps.CPU["cpu_package"] = details.CPUPackage
			}
			for _, core := range details.Cores {
				temps.CPU[fmt.Sprintf("cpu_core_%d", core.Core)] = core.Temp
			}
			for _, disk := range details.NVMe {
				temps.Disks[disk.Device] = disk.Temp
			}
			for _, disk := range details.SMART {
				if !disk.StandbySkipped {
					temps.Disks[disk.Device] = float64(disk.Temperature)
				}
			}
			for _, gpu := range details.GPU {
				for label, value := range map[string]float64{"edge": gpu.Edge, "junction": gpu.Junction, "memory": gpu.Mem} {
					if value != 0 {
						temps.Other[gpu.Device+"_"+label] = value
					}
				}
			}
		}
		if len(temps.CPU)+len(temps.Disks)+len(temps.Other) > 0 {
			results = append(results, temps)
		}
	}

	if len(results) == 0 {
		if resourceFilter != "" {
			return NewTextResult(fmt.Sprintf("No temperature data available for resource %q in the current canonical resource observations.", resourceFilter)), nil
		}
		if hostFilter != "" {
			return NewTextResult(fmt.Sprintf("No temperature data available for host '%s' in the current canonical resource observations.", hostFilter)), nil
		}
		return NewTextResult("No temperature data available in the current canonical resource observations."), nil
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return NewTextResult(string(output)), nil
}

func (e *PulseToolExecutor) executeGetNetworkStats(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	hostFilter, _ := args["host"].(string)

	rs, err := e.readStateForControl()
	if err != nil {
		return NewTextResult("State provider not available."), nil
	}

	var hosts []HostNetworkStatsSummary
	seenHostnames := map[string]bool{}

	for _, host := range rs.Hosts() {
		if host == nil {
			continue
		}
		hostname := strings.TrimSpace(host.Hostname())
		if hostname == "" {
			hostname = strings.TrimSpace(host.Name())
		}
		if hostFilter != "" && hostname != hostFilter {
			continue
		}

		networkInterfaces := host.NetworkInterfaces()
		if len(networkInterfaces) == 0 {
			continue
		}

		var interfaces []NetworkInterfaceSummary
		for _, iface := range networkInterfaces {
			interfaces = append(interfaces, NetworkInterfaceSummary{
				Name:      iface.Name,
				MAC:       iface.MAC,
				Addresses: iface.Addresses,
				RXBytes:   iface.RXBytes,
				TXBytes:   iface.TXBytes,
				SpeedMbps: iface.SpeedMbps,
			})
		}

		hosts = append(hosts, HostNetworkStatsSummary{
			Hostname:   hostname,
			Interfaces: interfaces,
		})
		seenHostnames[hostname] = true
	}

	// Also check Docker hosts for network stats
	for _, dockerHost := range rs.DockerHosts() {
		if dockerHost == nil {
			continue
		}
		hostname := strings.TrimSpace(dockerHost.Hostname())
		if hostname == "" {
			hostname = strings.TrimSpace(dockerHost.Name())
		}
		if hostFilter != "" && hostname != hostFilter {
			continue
		}

		networkInterfaces := dockerHost.NetworkInterfaces()
		if len(networkInterfaces) == 0 {
			continue
		}

		// Check if we already have this host
		if seenHostnames[hostname] {
			continue
		}

		var interfaces []NetworkInterfaceSummary
		for _, iface := range networkInterfaces {
			interfaces = append(interfaces, NetworkInterfaceSummary{
				Name:      iface.Name,
				MAC:       iface.MAC,
				Addresses: iface.Addresses,
				RXBytes:   iface.RXBytes,
				TXBytes:   iface.TXBytes,
				SpeedMbps: iface.SpeedMbps,
			})
		}

		hosts = append(hosts, HostNetworkStatsSummary{
			Hostname:   hostname,
			Interfaces: interfaces,
		})
		seenHostnames[hostname] = true
	}

	if len(hosts) == 0 {
		if hostFilter != "" {
			return NewTextResult(fmt.Sprintf("No network statistics available for host '%s'.", hostFilter)), nil
		}
		return NewTextResult("No network statistics available. Ensure Pulse agents are reporting network data."), nil
	}

	response := EmptyNetworkStatsResponse()
	response.Hosts = hosts
	response.Total = len(hosts)

	return NewJSONResult(response.NormalizeCollections()), nil
}

func (e *PulseToolExecutor) executeGetDiskIOStats(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	hostFilter, _ := args["host"].(string)

	rs, err := e.readStateForControl()
	if err != nil {
		return NewTextResult("State provider not available."), nil
	}

	var hosts []HostDiskIOStatsSummary

	for _, host := range rs.Hosts() {
		if host == nil {
			continue
		}
		hostname := strings.TrimSpace(host.Hostname())
		if hostname == "" {
			hostname = strings.TrimSpace(host.Name())
		}
		if hostFilter != "" && hostname != hostFilter {
			continue
		}

		diskIO := host.DiskIO()
		if len(diskIO) == 0 {
			continue
		}

		var devices []DiskIODeviceSummary
		for _, dio := range diskIO {
			devices = append(devices, DiskIODeviceSummary{
				Device:     dio.Device,
				ReadBytes:  dio.ReadBytes,
				WriteBytes: dio.WriteBytes,
				ReadOps:    dio.ReadOps,
				WriteOps:   dio.WriteOps,
				IOTimeMs:   dio.IOTimeMs,
			})
		}

		hosts = append(hosts, HostDiskIOStatsSummary{
			Hostname: hostname,
			Devices:  devices,
		})
	}

	if len(hosts) == 0 {
		if hostFilter != "" {
			return NewTextResult(fmt.Sprintf("No disk I/O statistics available for host '%s'.", hostFilter)), nil
		}
		return NewTextResult("No disk I/O statistics available. Ensure Pulse agents are reporting disk I/O data."), nil
	}

	response := EmptyDiskIOStatsResponse()
	response.Hosts = hosts
	response.Total = len(hosts)

	return NewJSONResult(response.NormalizeCollections()), nil
}

func (e *PulseToolExecutor) executeListPhysicalDisks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	instanceFilter, _ := args["instance"].(string)
	nodeFilter, _ := args["node"].(string)
	healthFilter, _ := args["health"].(string)
	typeFilter, _ := args["disk_type"].(string)
	limit := intArg(args, "limit", 100)

	// Prefer unified resources when available
	if e.unifiedResourceProvider != nil {
		resources := e.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypePhysicalDisk)
		if len(resources) == 0 {
			return NewTextResult("No physical disk data available. Physical disk information is collected from Proxmox nodes."), nil
		}

		var disks []PhysicalDiskSummary
		totalCount := 0

		for _, r := range resources {
			pd := r.PhysicalDisk
			if pd == nil {
				continue
			}

			node := canonicalPhysicalDiskHost(r)

			// Apply filters
			if instanceFilter != "" {
				// Instance is encoded in the resource ID as "{instance}-{node}-..."
				// Skip if no match found in tags or ID
				matched := false
				for _, tag := range r.Tags {
					if strings.EqualFold(tag, instanceFilter) {
						matched = true
						break
					}
				}
				if !matched && !strings.HasPrefix(r.ID, instanceFilter) {
					continue
				}
			}
			if nodeFilter != "" && !strings.EqualFold(node, nodeFilter) {
				continue
			}
			if healthFilter != "" && !strings.EqualFold(pd.Health, healthFilter) {
				continue
			}
			if typeFilter != "" && !strings.EqualFold(pd.DiskType, typeFilter) {
				continue
			}

			totalCount++
			if len(disks) >= limit {
				continue
			}

			summary := PhysicalDiskSummary{
				ID:          r.ID,
				Node:        node,
				DevPath:     pd.DevPath,
				Model:       pd.Model,
				Serial:      pd.Serial,
				WWN:         pd.WWN,
				Type:        pd.DiskType,
				SizeBytes:   pd.SizeBytes,
				Health:      pd.Health,
				Used:        pd.Used,
				LastChecked: r.LastSeen,
			}

			if pd.Wearout >= 0 {
				wearout := pd.Wearout
				summary.Wearout = &wearout
			}
			if pd.Temperature > 0 {
				temp := pd.Temperature
				summary.Temperature = &temp
			}
			if pd.RPM > 0 {
				rpm := pd.RPM
				summary.RPM = &rpm
			}

			disks = append(disks, summary)
		}

		if disks == nil {
			disks = []PhysicalDiskSummary{}
		}

		response := EmptyPhysicalDisksResponse()
		response.Disks = disks
		response.Total = len(resources)
		response.Filtered = totalCount
		return NewJSONResult(response.NormalizeCollections()), nil
	}

	return NewTextResult("No physical disk data available. Physical disk information is collected from Proxmox nodes."), nil
}

// maxMetricPoints is the maximum number of metric data points returned per resource.
// Beyond this, points are downsampled via bucket averaging to preserve trends
// while keeping the response size manageable for the LLM context window.
const maxMetricPoints = 120

// downsampleMetrics reduces a slice of MetricPoints to targetCount points
// by bucket-averaging. Each bucket covers an equal time span and the output
// point uses the bucket's midpoint timestamp with averaged metric values.
func downsampleMetrics(points []MetricPoint, targetCount int) []MetricPoint {
	if len(points) <= targetCount || targetCount <= 0 {
		return points
	}

	// Sort by timestamp (should already be sorted, but be safe)
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	bucketSize := len(points) / targetCount
	if bucketSize < 1 {
		bucketSize = 1
	}

	result := make([]MetricPoint, 0, targetCount)
	for i := 0; i < len(points); i += bucketSize {
		end := i + bucketSize
		if end > len(points) {
			end = len(points)
		}
		bucket := points[i:end]

		var sumCPU, sumMem, sumDisk float64
		hasDisk := false
		for _, p := range bucket {
			sumCPU += p.CPU
			sumMem += p.Memory
			if p.Disk != 0 {
				sumDisk += p.Disk
				hasDisk = true
			}
		}
		n := float64(len(bucket))

		// Use midpoint timestamp
		midIdx := len(bucket) / 2
		avg := MetricPoint{
			Timestamp: bucket[midIdx].Timestamp,
			CPU:       sumCPU / n,
			Memory:    sumMem / n,
		}
		if hasDisk {
			avg.Disk = sumDisk / n
		}
		result = append(result, avg)

		if len(result) >= targetCount {
			break
		}
	}

	return result
}
