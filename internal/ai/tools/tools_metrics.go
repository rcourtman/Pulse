package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// registerMetricsTools registers the pulse_metrics tool
func (e *PulseToolExecutor) registerMetricsTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_metrics",
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
						Description: "Filter by specific resource ID (for performance, baselines)",
					},
					"resource_type": {
						Type:        "string",
						Description: "Filter by resource type: vm, container, node (for performance, baselines)",
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
		// Normalize semantic type alias to metrics-level type
		if resourceType == "system-container" {
			resourceType = "container"
		}
		validTypes := map[string]bool{"vm": true, "container": true, "node": true}
		if !validTypes[resourceType] {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use vm, system-container, or node", resourceType)), nil
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

	response := MetricsResponse{
		Period: period,
	}

	if resourceID != "" {
		response.ResourceID = resourceID
		metrics, err := e.metricsHistory.GetResourceMetrics(resourceID, duration)
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
		return NewJSONResult(response), nil
	}

	summary, err := e.metricsHistory.GetAllMetricsSummary(duration)
	if err != nil {
		return NewErrorResult(err), nil
	}

	keys := make([]string, 0, len(summary))
	for id, metric := range summary {
		if resourceType != "" && strings.ToLower(metric.ResourceType) != resourceType {
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
		filtered[id] = summary[id]
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

	return NewJSONResult(response), nil
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
		// Normalize semantic type alias to metrics-level type
		if resourceType == "system-container" {
			resourceType = "container"
		}
		validTypes := map[string]bool{"vm": true, "container": true, "node": true}
		if !validTypes[resourceType] {
			return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use vm, system-container, or node", resourceType)), nil
		}
	}

	if e.baselineProvider == nil {
		return NewTextResult("Baseline data not available. The system needs time to learn normal behavior patterns."), nil
	}

	response := BaselinesResponse{
		Baselines: make(map[string]map[string]*MetricBaseline),
	}

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
		return NewJSONResult(response), nil
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
			typeIndex[fmt.Sprintf("%d", ct.VMID())] = "container"
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

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetPatterns(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.patternProvider == nil {
		return NewTextResult("Pattern detection not available. The system needs more historical data."), nil
	}

	response := PatternsResponse{
		Patterns:    e.patternProvider.GetPatterns(),
		Predictions: e.patternProvider.GetPredictions(),
	}

	// Ensure non-nil slices for clean JSON
	if response.Patterns == nil {
		response.Patterns = []Pattern{}
	}
	if response.Predictions == nil {
		response.Predictions = []Prediction{}
	}

	return NewJSONResult(response), nil
}

// ========== Temperature, Network, DiskIO, Physical Disks ==========

func (e *PulseToolExecutor) executeGetTemperatures(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	hostFilter, _ := args["host"].(string)

	state := e.stateProvider.GetState()

	type HostTemps struct {
		Hostname    string             `json:"hostname"`
		Platform    string             `json:"platform,omitempty"`
		CPU         map[string]float64 `json:"cpu_temps,omitempty"`
		Disks       map[string]float64 `json:"disk_temps,omitempty"`
		Fans        map[string]float64 `json:"fan_rpm,omitempty"`
		Other       map[string]float64 `json:"other_temps,omitempty"`
		LastUpdated string             `json:"last_updated,omitempty"`
	}

	var results []HostTemps

	for _, host := range state.Hosts {
		if hostFilter != "" && host.Hostname != hostFilter {
			continue
		}

		if len(host.Sensors.TemperatureCelsius) == 0 && len(host.Sensors.FanRPM) == 0 {
			continue
		}

		temps := HostTemps{
			Hostname: host.Hostname,
			Platform: host.Platform,
			CPU:      make(map[string]float64),
			Disks:    make(map[string]float64),
			Fans:     make(map[string]float64),
			Other:    make(map[string]float64),
		}

		// Categorize temperatures
		for name, value := range host.Sensors.TemperatureCelsius {
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
		for name, value := range host.Sensors.FanRPM {
			temps.Fans[name] = value
		}

		// Add additional sensors to Other
		for name, value := range host.Sensors.Additional {
			if _, exists := temps.CPU[name]; !exists {
				if _, exists := temps.Disks[name]; !exists {
					temps.Other[name] = value
				}
			}
		}

		results = append(results, temps)
	}

	if len(results) == 0 {
		if hostFilter != "" {
			return NewTextResult(fmt.Sprintf("No temperature data available for host '%s'. The host may not have a Pulse agent installed or sensors may not be available.", hostFilter)), nil
		}
		return NewTextResult("No temperature data available. Ensure Pulse unified agents are installed on hosts and lm-sensors is available."), nil
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return NewTextResult(string(output)), nil
}

func (e *PulseToolExecutor) executeGetNetworkStats(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	hostFilter, _ := args["host"].(string)

	state := e.stateProvider.GetState()

	var hosts []HostNetworkStatsSummary

	for _, host := range state.Hosts {
		if hostFilter != "" && host.Hostname != hostFilter {
			continue
		}

		if len(host.NetworkInterfaces) == 0 {
			continue
		}

		var interfaces []NetworkInterfaceSummary
		for _, iface := range host.NetworkInterfaces {
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
			Hostname:   host.Hostname,
			Interfaces: interfaces,
		})
	}

	// Also check Docker hosts for network stats
	for _, dockerHost := range state.DockerHosts {
		if hostFilter != "" && dockerHost.Hostname != hostFilter {
			continue
		}

		if len(dockerHost.NetworkInterfaces) == 0 {
			continue
		}

		// Check if we already have this host
		found := false
		for _, h := range hosts {
			if h.Hostname == dockerHost.Hostname {
				found = true
				break
			}
		}
		if found {
			continue
		}

		var interfaces []NetworkInterfaceSummary
		for _, iface := range dockerHost.NetworkInterfaces {
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
			Hostname:   dockerHost.Hostname,
			Interfaces: interfaces,
		})
	}

	if len(hosts) == 0 {
		if hostFilter != "" {
			return NewTextResult(fmt.Sprintf("No network statistics available for host '%s'.", hostFilter)), nil
		}
		return NewTextResult("No network statistics available. Ensure Pulse agents are reporting network data."), nil
	}

	response := NetworkStatsResponse{
		Hosts: hosts,
		Total: len(hosts),
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetDiskIOStats(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	hostFilter, _ := args["host"].(string)

	state := e.stateProvider.GetState()

	var hosts []HostDiskIOStatsSummary

	for _, host := range state.Hosts {
		if hostFilter != "" && host.Hostname != hostFilter {
			continue
		}

		if len(host.DiskIO) == 0 {
			continue
		}

		var devices []DiskIODeviceSummary
		for _, dio := range host.DiskIO {
			devices = append(devices, DiskIODeviceSummary{
				Device:     dio.Device,
				ReadBytes:  dio.ReadBytes,
				WriteBytes: dio.WriteBytes,
				ReadOps:    dio.ReadOps,
				WriteOps:   dio.WriteOps,
				IOTimeMs:   dio.IOTime,
			})
		}

		hosts = append(hosts, HostDiskIOStatsSummary{
			Hostname: host.Hostname,
			Devices:  devices,
		})
	}

	if len(hosts) == 0 {
		if hostFilter != "" {
			return NewTextResult(fmt.Sprintf("No disk I/O statistics available for host '%s'.", hostFilter)), nil
		}
		return NewTextResult("No disk I/O statistics available. Ensure Pulse agents are reporting disk I/O data."), nil
	}

	response := DiskIOStatsResponse{
		Hosts: hosts,
		Total: len(hosts),
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeListPhysicalDisks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	instanceFilter, _ := args["instance"].(string)
	nodeFilter, _ := args["node"].(string)
	healthFilter, _ := args["health"].(string)
	typeFilter, _ := args["type"].(string)
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

			node := r.ParentName
			if node == "" && len(r.Identity.Hostnames) > 0 {
				node = r.Identity.Hostnames[0]
			}

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

		return NewJSONResult(PhysicalDisksResponse{
			Disks:    disks,
			Total:    len(resources),
			Filtered: totalCount,
		}), nil
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
