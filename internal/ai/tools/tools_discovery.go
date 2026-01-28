package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// getCommandContext returns information about how to run commands on a resource.
// This helps the AI understand what commands to use with pulse_control.
type CommandContext struct {
	// How to run commands: "direct" (agent on resource), "via_host" (agent on parent host)
	Method string `json:"method"`
	// The target_host value to use with pulse_control
	TargetHost string `json:"target_host"`
	// Example command pattern (what to pass to pulse_control)
	Example string `json:"example"`
	// For containers running inside this resource (e.g., Docker in LXC)
	NestedExample string `json:"nested_example,omitempty"`
}

// getCLIAccessPattern returns context about the resource type.
// Does NOT prescribe how to access - the AI should determine that based on available agents.
func getCLIAccessPattern(resourceType, hostID, resourceID string) string {
	switch resourceType {
	case "lxc":
		return fmt.Sprintf("LXC container on Proxmox node '%s' (VMID %s)", hostID, resourceID)
	case "vm":
		return fmt.Sprintf("VM on Proxmox node '%s' (VMID %s)", hostID, resourceID)
	case "docker":
		return fmt.Sprintf("Docker container '%s' on host '%s'", resourceID, hostID)
	case "host":
		return fmt.Sprintf("Host '%s'", hostID)
	default:
		return ""
	}
}

// commonServicePaths contains typical log/config paths for well-known services
// These are fallbacks when discovery doesn't find specific paths
var commonServicePaths = map[string]struct {
	LogPaths    []string
	ConfigPaths []string
	DataPaths   []string
}{
	"jellyfin": {
		LogPaths:    []string{"/var/log/jellyfin/", "/config/log/"},
		ConfigPaths: []string{"/etc/jellyfin/", "/config/"},
		DataPaths:   []string{"/var/lib/jellyfin/", "/config/data/"},
	},
	"plex": {
		LogPaths:    []string{"/var/lib/plexmediaserver/Library/Application Support/Plex Media Server/Logs/"},
		ConfigPaths: []string{"/var/lib/plexmediaserver/Library/Application Support/Plex Media Server/"},
		DataPaths:   []string{"/var/lib/plexmediaserver/"},
	},
	"sonarr": {
		LogPaths:    []string{"/config/logs/"},
		ConfigPaths: []string{"/config/"},
		DataPaths:   []string{"/config/"},
	},
	"radarr": {
		LogPaths:    []string{"/config/logs/"},
		ConfigPaths: []string{"/config/"},
		DataPaths:   []string{"/config/"},
	},
	"prowlarr": {
		LogPaths:    []string{"/config/logs/"},
		ConfigPaths: []string{"/config/"},
		DataPaths:   []string{"/config/"},
	},
	"lidarr": {
		LogPaths:    []string{"/config/logs/"},
		ConfigPaths: []string{"/config/"},
		DataPaths:   []string{"/config/"},
	},
	"postgresql": {
		LogPaths:    []string{"/var/log/postgresql/", "/var/lib/postgresql/data/log/"},
		ConfigPaths: []string{"/etc/postgresql/", "/var/lib/postgresql/data/"},
		DataPaths:   []string{"/var/lib/postgresql/data/"},
	},
	"mysql": {
		LogPaths:    []string{"/var/log/mysql/", "/var/lib/mysql/"},
		ConfigPaths: []string{"/etc/mysql/"},
		DataPaths:   []string{"/var/lib/mysql/"},
	},
	"mariadb": {
		LogPaths:    []string{"/var/log/mysql/", "/var/lib/mysql/"},
		ConfigPaths: []string{"/etc/mysql/"},
		DataPaths:   []string{"/var/lib/mysql/"},
	},
	"nginx": {
		LogPaths:    []string{"/var/log/nginx/"},
		ConfigPaths: []string{"/etc/nginx/"},
		DataPaths:   []string{"/var/www/"},
	},
	"homeassistant": {
		LogPaths:    []string{"/config/home-assistant.log"},
		ConfigPaths: []string{"/config/"},
		DataPaths:   []string{"/config/"},
	},
	"frigate": {
		LogPaths:    []string{"/config/logs/"},
		ConfigPaths: []string{"/config/"},
		DataPaths:   []string{"/media/frigate/"},
	},
	"redis": {
		LogPaths:    []string{"/var/log/redis/"},
		ConfigPaths: []string{"/etc/redis/"},
		DataPaths:   []string{"/var/lib/redis/"},
	},
	"mongodb": {
		LogPaths:    []string{"/var/log/mongodb/"},
		ConfigPaths: []string{"/etc/mongod.conf"},
		DataPaths:   []string{"/var/lib/mongodb/"},
	},
	"grafana": {
		LogPaths:    []string{"/var/log/grafana/"},
		ConfigPaths: []string{"/etc/grafana/"},
		DataPaths:   []string{"/var/lib/grafana/"},
	},
	"prometheus": {
		LogPaths:    []string{"/var/log/prometheus/"},
		ConfigPaths: []string{"/etc/prometheus/"},
		DataPaths:   []string{"/var/lib/prometheus/"},
	},
	"influxdb": {
		LogPaths:    []string{"/var/log/influxdb/"},
		ConfigPaths: []string{"/etc/influxdb/"},
		DataPaths:   []string{"/var/lib/influxdb/"},
	},
}

// getCommonServicePaths returns fallback paths for a service type
func getCommonServicePaths(serviceType string) (logPaths, configPaths, dataPaths []string) {
	// Normalize service type (lowercase, remove version numbers)
	normalized := strings.ToLower(serviceType)
	// Try to match against known services
	for key, paths := range commonServicePaths {
		if strings.Contains(normalized, key) {
			return paths.LogPaths, paths.ConfigPaths, paths.DataPaths
		}
	}
	return nil, nil, nil
}

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
						Description: "Node/host where the resource runs. For VM/LXC: the PVE node from 'node' field. For Docker: the Docker host. For host type: same as resource_id.",
					},
				},
				Required: []string{"resource_type", "resource_id", "host_id"},
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

func (e *PulseToolExecutor) executeGetDiscovery(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.discoveryProvider == nil {
		return NewTextResult("Discovery service not available."), nil
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
	if hostID == "" {
		return NewErrorResult(fmt.Errorf("host_id is required - use the 'node' field from search or get_resource results")), nil
	}

	// For LXC and VM types, resourceID should be a numeric VMID.
	// If a name was passed, try to resolve it to a VMID.
	if (resourceType == "lxc" || resourceType == "vm") && e.stateProvider != nil {
		if _, err := strconv.Atoi(resourceID); err != nil {
			// Not a number - try to resolve the name to a VMID
			state := e.stateProvider.GetState()
			resolved := false

			if resourceType == "lxc" {
				for _, c := range state.Containers {
					if strings.EqualFold(c.Name, resourceID) && c.Node == hostID {
						resourceID = fmt.Sprintf("%d", c.VMID)
						resolved = true
						break
					}
				}
			} else if resourceType == "vm" {
				for _, vm := range state.VMs {
					if strings.EqualFold(vm.Name, resourceID) && vm.Node == hostID {
						resourceID = fmt.Sprintf("%d", vm.VMID)
						resolved = true
						break
					}
				}
			}

			if !resolved {
				return NewErrorResult(fmt.Errorf("could not resolve resource name '%s' to a VMID on host '%s'", resourceID, hostID)), nil
			}
		}
	}

	// First try to get existing discovery
	discovery, err := e.discoveryProvider.GetDiscoveryByResource(resourceType, hostID, resourceID)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to get discovery: %w", err)), nil
	}

	// Compute CLI access pattern (always useful, even if discovery fails)
	cliAccess := getCLIAccessPattern(resourceType, hostID, resourceID)

	// If no discovery exists, trigger one
	if discovery == nil {
		discovery, err = e.discoveryProvider.TriggerDiscovery(ctx, resourceType, hostID, resourceID)
		if err != nil {
			// Even on failure, provide cli_access so AI can investigate manually
			return NewJSONResult(map[string]interface{}{
				"found":         false,
				"resource_type": resourceType,
				"resource_id":   resourceID,
				"host_id":       hostID,
				"cli_access":    cliAccess,
				"message":       fmt.Sprintf("Discovery failed: %v", err),
				"hint":          "Use pulse_control with type='command' to investigate. Try checking /var/log/ for logs.",
			}), nil
		}
	}

	if discovery == nil {
		// No discovery but provide cli_access for manual investigation
		return NewJSONResult(map[string]interface{}{
			"found":         false,
			"resource_type": resourceType,
			"resource_id":   resourceID,
			"host_id":       hostID,
			"cli_access":    cliAccess,
			"message":       "Discovery returned no data. The resource may not be accessible.",
			"hint":          "Use pulse_control with type='command' to investigate. Try listing /var/log/ or checking running processes.",
		}), nil
	}

	// Use fallback cli_access if discovery didn't provide one
	responseCLIAccess := discovery.CLIAccess
	if responseCLIAccess == "" {
		responseCLIAccess = cliAccess
	}

	// Use fallback paths for known services if discovery didn't find specific ones
	responseConfigPaths := discovery.ConfigPaths
	responseDataPaths := discovery.DataPaths
	var responseLogPaths []string

	if discovery.ServiceType != "" {
		fallbackLogPaths, fallbackConfigPaths, fallbackDataPaths := getCommonServicePaths(discovery.ServiceType)
		if len(responseConfigPaths) == 0 && len(fallbackConfigPaths) > 0 {
			responseConfigPaths = fallbackConfigPaths
		}
		if len(responseDataPaths) == 0 && len(fallbackDataPaths) > 0 {
			responseDataPaths = fallbackDataPaths
		}
		if len(fallbackLogPaths) > 0 {
			responseLogPaths = fallbackLogPaths
		}
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
		"cli_access":      responseCLIAccess,
		"config_paths":    responseConfigPaths,
		"data_paths":      responseDataPaths,
		"confidence":      discovery.Confidence,
		"discovered_at":   discovery.DiscoveredAt,
		"updated_at":      discovery.UpdatedAt,
	}

	// Add log paths if we have them (from fallback or discovery)
	if len(responseLogPaths) > 0 {
		response["log_paths"] = responseLogPaths
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

	// Add listening ports if present
	if len(discovery.Ports) > 0 {
		ports := make([]map[string]interface{}, 0, len(discovery.Ports))
		for _, p := range discovery.Ports {
			port := map[string]interface{}{
				"port":     p.Port,
				"protocol": p.Protocol,
			}
			if p.Process != "" {
				port["process"] = p.Process
			}
			if p.Address != "" {
				port["address"] = p.Address
			}
			ports = append(ports, port)
		}
		response["ports"] = ports
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

		// Add ports count
		if len(d.Ports) > 0 {
			result["ports_count"] = len(d.Ports)
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
