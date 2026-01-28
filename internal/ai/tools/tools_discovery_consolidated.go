package tools

import (
	"context"
	"fmt"
)

// registerDiscoveryToolsConsolidated registers the consolidated pulse_discovery tool
func (e *PulseToolExecutor) registerDiscoveryToolsConsolidated() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_discovery",
			Description: `Get deep AI-discovered information about services (log paths, config locations, service details).

Actions:
- get: Trigger discovery and get detailed info for a specific resource. Use this when you need deep context about a container/VM (where logs are, config paths, service details). Requires resource_type, resource_id, and host_id - use pulse_query action="search" first if you don't know these.
- list: Search existing discoveries only. Will NOT find resources that haven't been discovered yet. Use action="get" to trigger discovery for new resources.

Workflow for investigating applications:
1. Use pulse_query action="search" to find the resource by name
2. Use pulse_discovery action="get" with the resource details to get deep context (log paths, config locations)
3. Use pulse_control type="command" to run commands (check logs, query app state, etc.)

Examples:
- Trigger discovery: action="get", resource_type="docker", resource_id="jellyfin", host_id="docker-host-1"
- Search existing: action="list", service_type="postgresql"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"action": {
						Type:        "string",
						Description: "Discovery action: get or list",
						Enum:        []string{"get", "list"},
					},
					"resource_type": {
						Type:        "string",
						Description: "For get: resource type (vm, lxc, docker, host)",
						Enum:        []string{"vm", "lxc", "docker", "host"},
					},
					"resource_id": {
						Type:        "string",
						Description: "For get: resource identifier (VMID, container name, hostname)",
					},
					"host_id": {
						Type:        "string",
						Description: "For get: node/host where resource runs",
					},
					"type": {
						Type:        "string",
						Description: "For list: filter by resource type",
						Enum:        []string{"vm", "lxc", "docker", "host"},
					},
					"host": {
						Type:        "string",
						Description: "For list: filter by host/node ID",
					},
					"service_type": {
						Type:        "string",
						Description: "For list: filter by service type (e.g., frigate, postgresql)",
					},
					"limit": {
						Type:        "integer",
						Description: "For list: maximum results (default: 50)",
					},
				},
				Required: []string{"action"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeDiscovery(ctx, args)
		},
	})
}

// executeDiscovery routes to the appropriate discovery handler based on action
// Handler functions are implemented in tools_discovery.go
func (e *PulseToolExecutor) executeDiscovery(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	action, _ := args["action"].(string)
	switch action {
	case "get":
		return e.executeGetDiscovery(ctx, args)
	case "list":
		return e.executeListDiscoveries(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown action: %s. Use: get, list", action)), nil
	}
}
