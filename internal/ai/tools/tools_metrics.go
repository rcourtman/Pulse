package tools

import (
	"context"
	"fmt"
)

// registerMetricsTools registers the consolidated pulse_metrics tool
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
// All handler functions are implemented in tools_patrol.go and tools_infrastructure.go
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
