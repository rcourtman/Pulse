package tools

import (
	"context"
	"fmt"
	"strings"
)

// registerDiscoveryTools registers AI-powered infrastructure discovery tools
func (e *PulseToolExecutor) registerDiscoveryTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_discovery",
			Description: `Get AI-discovered information about a specific resource (VM, LXC container, Docker container, or host).

Returns: JSON with service type, version, config paths, data paths, CLI access command, and discovered facts.

Use when: You need detailed context about a resource before proposing remediation commands, or when investigating what services are running on infrastructure.

The discovery includes:
- Service type and version (e.g., "Frigate NVR v0.13.2")
- Configuration file locations
- Data/storage paths
- CLI access command (e.g., "pct exec 101 -- <command>")
- Discovered facts (ports, GPUs, connected services, etc.)
- User-added notes

This information is critical for proposing correct remediation commands that match the actual service configuration.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_type": {
						Type:        "string",
						Description: "Type of resource: 'vm', 'lxc', 'docker', or 'host'",
						Enum:        []string{"vm", "lxc", "docker", "host"},
					},
					"resource_id": {
						Type:        "string",
						Description: "Resource identifier (VMID for VM/LXC, container name for Docker, hostname for host)",
					},
					"host_id": {
						Type:        "string",
						Description: "Optional: Host/node ID where the resource runs (required for Docker containers)",
					},
				},
				Required: []string{"resource_type", "resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetDiscovery(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_discoveries",
			Description: `List all AI-discovered infrastructure information with optional filtering.

Returns: JSON array of discoveries with service types, versions, and summaries.

Use when: You need an overview of what services are running across infrastructure, or want to find specific service types.

Filters:
- type: Filter by resource type (vm, lxc, docker, host)
- host: Filter by host/node ID
- service_type: Filter by discovered service type (e.g., "frigate", "postgresql")`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Optional: Filter by resource type",
						Enum:        []string{"vm", "lxc", "docker", "host"},
					},
					"host": {
						Type:        "string",
						Description: "Optional: Filter by host/node ID",
					},
					"service_type": {
						Type:        "string",
						Description: "Optional: Filter by discovered service type",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 50)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListDiscoveries(ctx, args)
		},
	})
}

func (e *PulseToolExecutor) executeGetDiscovery(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.discoveryProvider == nil {
		return NewTextResult("Discovery service not available. Run a discovery scan first."), nil
	}

	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)
	hostID, _ := args["host_id"].(string)

	if resourceType == "" {
		return NewErrorResult(fmt.Errorf("resource_type is required")), nil
	}
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	discovery, err := e.discoveryProvider.GetDiscoveryByResource(resourceType, hostID, resourceID)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to get discovery: %w", err)), nil
	}

	if discovery == nil {
		return NewJSONResult(map[string]interface{}{
			"found":         false,
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"message":       "No discovery data found for this resource. Run a discovery scan to gather information.",
		}), nil
	}

	// Return the discovery information
	response := map[string]interface{}{
		"found":           true,
		"id":              discovery.ID,
		"resource_type":   discovery.ResourceType,
		"resource_id":     discovery.ResourceID,
		"host_id":         discovery.HostID,
		"hostname":        discovery.Hostname,
		"service_type":    discovery.ServiceType,
		"service_name":    discovery.ServiceName,
		"service_version": discovery.ServiceVersion,
		"category":        discovery.Category,
		"cli_access":      discovery.CLIAccess,
		"config_paths":    discovery.ConfigPaths,
		"data_paths":      discovery.DataPaths,
		"confidence":      discovery.Confidence,
		"discovered_at":   discovery.DiscoveredAt,
		"updated_at":      discovery.UpdatedAt,
	}

	// Add facts if present
	if len(discovery.Facts) > 0 {
		facts := make([]map[string]string, 0, len(discovery.Facts))
		for _, f := range discovery.Facts {
			facts = append(facts, map[string]string{
				"category": f.Category,
				"key":      f.Key,
				"value":    f.Value,
			})
		}
		response["facts"] = facts
	}

	// Add user notes if present
	if discovery.UserNotes != "" {
		response["user_notes"] = discovery.UserNotes
	}

	// Add AI reasoning for context
	if discovery.AIReasoning != "" {
		response["ai_reasoning"] = discovery.AIReasoning
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeListDiscoveries(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.discoveryProvider == nil {
		return NewTextResult("Discovery service not available."), nil
	}

	filterType, _ := args["type"].(string)
	filterHost, _ := args["host"].(string)
	filterServiceType, _ := args["service_type"].(string)
	limit := intArg(args, "limit", 50)

	var discoveries []*ResourceDiscoveryInfo
	var err error

	// Get discoveries based on filters
	if filterType != "" {
		discoveries, err = e.discoveryProvider.ListDiscoveriesByType(filterType)
	} else if filterHost != "" {
		discoveries, err = e.discoveryProvider.ListDiscoveriesByHost(filterHost)
	} else {
		discoveries, err = e.discoveryProvider.ListDiscoveries()
	}

	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to list discoveries: %w", err)), nil
	}

	// Filter by service type if specified
	if filterServiceType != "" {
		filtered := make([]*ResourceDiscoveryInfo, 0)
		filterLower := strings.ToLower(filterServiceType)
		for _, d := range discoveries {
			if strings.Contains(strings.ToLower(d.ServiceType), filterLower) ||
				strings.Contains(strings.ToLower(d.ServiceName), filterLower) {
				filtered = append(filtered, d)
			}
		}
		discoveries = filtered
	}

	// Apply limit
	if len(discoveries) > limit {
		discoveries = discoveries[:limit]
	}

	// Build response
	results := make([]map[string]interface{}, 0, len(discoveries))
	for _, d := range discoveries {
		result := map[string]interface{}{
			"id":              d.ID,
			"resource_type":   d.ResourceType,
			"resource_id":     d.ResourceID,
			"host_id":         d.HostID,
			"hostname":        d.Hostname,
			"service_type":    d.ServiceType,
			"service_name":    d.ServiceName,
			"service_version": d.ServiceVersion,
			"category":        d.Category,
			"cli_access":      d.CLIAccess,
			"confidence":      d.Confidence,
			"updated_at":      d.UpdatedAt,
		}

		// Add key facts count
		if len(d.Facts) > 0 {
			result["facts_count"] = len(d.Facts)
		}

		results = append(results, result)
	}

	response := map[string]interface{}{
		"discoveries": results,
		"total":       len(results),
	}

	if filterType != "" {
		response["filter_type"] = filterType
	}
	if filterHost != "" {
		response["filter_host"] = filterHost
	}
	if filterServiceType != "" {
		response["filter_service_type"] = filterServiceType
	}

	return NewJSONResult(response), nil
}
