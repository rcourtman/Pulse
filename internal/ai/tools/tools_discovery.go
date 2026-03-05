package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// registerDiscoveryTools registers the pulse_discovery tool
func (e *PulseToolExecutor) registerDiscoveryTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_discovery",
			Description: `Get AI-discovered service details (log paths, config locations, ports). action="get" triggers discovery for a resource (requires resource_type, resource_id, target_id). action="list" searches existing discoveries. Use pulse_query action="search" first to find resource details.`,
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
						Description: "For get: canonical v6 resource type (vm, system-container, app-container, agent)",
						Enum:        []string{"vm", "system-container", "app-container", "agent"},
					},
					"resource_id": {
						Type:        "string",
						Description: "For get: resource identifier (VMID, container name, hostname)",
					},
					"target_id": {
						Type:        "string",
						Description: "For get/list: canonical target identifier (agent ID, node ID, or cluster ID)",
					},
					"type": {
						Type:        "string",
						Description: "For list: filter by resource type",
						Enum:        []string{"vm", "system-container", "app-container", "agent"},
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
func getCLIAccessPattern(resourceType, targetID, resourceID string) string {
	switch resourceType {
	case "system-container":
		return fmt.Sprintf("System container on node '%s' (VMID %s)", targetID, resourceID)
	case "vm":
		return fmt.Sprintf("VM on node '%s' (VMID %s)", targetID, resourceID)
	case "app-container":
		return fmt.Sprintf("Docker container '%s' on target '%s'", resourceID, targetID)
	case "agent":
		return fmt.Sprintf("Agent '%s'", targetID)
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

func canonicalDiscoveryTargetID(discovery *ResourceDiscoveryInfo, fallbackTargetID string) string {
	if discovery == nil {
		return strings.TrimSpace(fallbackTargetID)
	}
	targetID := strings.TrimSpace(discovery.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(fallbackTargetID)
	}
	return targetID
}

func (e *PulseToolExecutor) executeGetDiscovery(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.discoveryProvider == nil {
		return NewTextResult("Discovery service not available."), nil
	}

	resourceTypeRaw, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)
	targetID, _ := args["target_id"].(string)
	if isUnsupportedDiscoveryLegacyResourceTypeToken(resourceTypeRaw) {
		return NewErrorResult(fmt.Errorf("unsupported resource_type %q", strings.TrimSpace(resourceTypeRaw))), nil
	}
	resourceType := canonicalDiscoveryResourceType(resourceTypeRaw)
	providerResourceType := discoveryProviderResourceType(resourceType)
	targetID = strings.TrimSpace(targetID)

	if resourceType == "" {
		return NewErrorResult(fmt.Errorf("resource_type is required")), nil
	}
	if !isSupportedDiscoveryResourceType(resourceType) {
		return NewErrorResult(fmt.Errorf("unsupported resource_type %q", strings.TrimSpace(resourceTypeRaw))), nil
	}
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}
	if targetID == "" {
		return NewErrorResult(fmt.Errorf("target_id is required - use the node/agent field from search or get_resource results")), nil
	}

	// For system-container and VM types, resourceID should be a numeric VMID.
	// If a name was passed, try to resolve it to a VMID from typed ReadState.
	if resourceType == "system-container" || resourceType == "vm" {
		if _, err := strconv.Atoi(resourceID); err != nil {
			// Not a number - try to resolve the name to a VMID
			resolved := false

			rs, err := e.readStateForControl()
			if err == nil {
				if resourceType == "system-container" {
					for _, c := range rs.Containers() {
						if strings.EqualFold(c.Name(), resourceID) && nodeMatchesTargetID(c.Node(), targetID) {
							resourceID = fmt.Sprintf("%d", c.VMID())
							resolved = true
							break
						}
					}
				} else if resourceType == "vm" {
					for _, vm := range rs.VMs() {
						if strings.EqualFold(vm.Name(), resourceID) && nodeMatchesTargetID(vm.Node(), targetID) {
							resourceID = fmt.Sprintf("%d", vm.VMID())
							resolved = true
							break
						}
					}
				}
			}

			if !resolved {
				return NewErrorResult(fmt.Errorf("could not resolve resource name '%s' to a VMID on target '%s'", resourceID, targetID)), nil
			}
		}
	}

	// First try to get existing discovery
	discovery, err := e.discoveryProvider.GetDiscoveryByResource(providerResourceType, targetID, resourceID)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to get discovery: %w", err)), nil
	}

	// Compute CLI access pattern (always useful, even if discovery fails)
	cliAccess := getCLIAccessPattern(resourceType, targetID, resourceID)

	// If no discovery exists, trigger one
	if discovery == nil {
		discovery, err = e.discoveryProvider.TriggerDiscovery(ctx, providerResourceType, targetID, resourceID)
		if err != nil {
			// Distinguish transient errors (rate limits, timeouts) from genuine not-found.
			// Transient errors must surface as IsError so the model stops retrying.
			if isTransientError(err) {
				return CallToolResult{
					Content: []Content{{
						Type: "text",
						Text: fmt.Sprintf("Discovery temporarily unavailable: %v. Do NOT retry this call. Use pulse_control or a different approach to investigate the resource.", err),
					}},
					IsError: true,
				}, nil
			}

			// Genuine failure (e.g. resource doesn't exist) — keep existing behavior
			return NewJSONResult(map[string]interface{}{
				"found":         false,
				"resource_type": resourceType,
				"resource_id":   resourceID,
				"target_id":     targetID,
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
			"target_id":     targetID,
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

	discoveryTargetID := canonicalDiscoveryTargetID(discovery, targetID)

	// Return the discovery information
	responseResourceType := canonicalDiscoveryResourceType(discovery.ResourceType)
	if responseResourceType == "" {
		responseResourceType = resourceType
	}
	response := map[string]interface{}{
		"found":           true,
		"id":              discovery.ID,
		"resource_type":   responseResourceType,
		"resource_id":     discovery.ResourceID,
		"target_id":       discoveryTargetID,
		"agent_id":        discovery.AgentID,
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

// isTransientError checks whether an error is a transient API/infrastructure error
// (rate limit, timeout, temporary unavailability) rather than a genuine "not found".
// When true, the caller should return IsError:true so the model doesn't retry.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	transientPatterns := []string{
		"429",
		"503",
		"rate_limit",
		"rate limit",
		"ratelimit",
		"too many requests",
		"timeout",
		"context deadline exceeded",
		"failed after", // "failed after N retries"
		"temporarily",  // "temporarily unavailable"
		"server overloaded",
		"service unavailable",
		"connection refused",
		"connection reset",
		"broken pipe",
		"i/o timeout",
		"network unreachable",
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

// nodeMatchesTargetID checks if a node name matches a target ID which may be
// a plain node name ("delly") or a composite instance-node ID ("homelab-delly").
func nodeMatchesTargetID(nodeName, targetID string) bool {
	if strings.EqualFold(nodeName, targetID) {
		return true
	}
	// Check if target ID is a composite "instance-node" format ending with the node name.
	if strings.HasSuffix(strings.ToLower(targetID), "-"+strings.ToLower(nodeName)) {
		return true
	}
	return false
}

func isSupportedDiscoveryResourceType(value string) bool {
	switch value {
	case "vm", "system-container", "app-container", "agent":
		return true
	default:
		return false
	}
}

func isUnsupportedLegacyResourceTypeToken(value string) bool {
	return unifiedresources.IsUnsupportedLegacyResourceTypeAlias(value)
}

func isUnsupportedDiscoveryLegacyResourceTypeToken(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if isUnsupportedLegacyResourceTypeToken(normalized) {
		return true
	}
	switch normalized {
	case "docker", "docker-container":
		return true
	default:
		return false
	}
}

func canonicalDiscoveryResourceType(raw string) string {
	resourceType := strings.ToLower(strings.TrimSpace(raw))
	switch resourceType {
	case "docker", "docker-container", "app-container":
		return "app-container"
	default:
		return resourceType
	}
}

func discoveryProviderResourceType(canonical string) string {
	switch canonicalDiscoveryResourceType(canonical) {
	case "app-container":
		// Discovery storage still keys Docker/container runtime discoveries as "docker".
		return "docker"
	default:
		return canonicalDiscoveryResourceType(canonical)
	}
}

func (e *PulseToolExecutor) executeListDiscoveries(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.discoveryProvider == nil {
		return NewTextResult("Discovery service not available."), nil
	}

	filterType, _ := args["type"].(string)
	filterTargetID, _ := args["target_id"].(string)
	filterServiceType, _ := args["service_type"].(string)
	limit := intArg(args, "limit", 50)
	if isUnsupportedDiscoveryLegacyResourceTypeToken(filterType) {
		return NewErrorResult(fmt.Errorf("unsupported type %q", strings.TrimSpace(filterType))), nil
	}
	filterType = canonicalDiscoveryResourceType(filterType)
	providerFilterType := discoveryProviderResourceType(filterType)
	filterTargetID = strings.TrimSpace(filterTargetID)

	if filterType != "" && !isSupportedDiscoveryResourceType(filterType) {
		return NewErrorResult(fmt.Errorf("unsupported type %q", filterType)), nil
	}

	var discoveries []*ResourceDiscoveryInfo
	var err error

	// Get discoveries based on filters
	if filterType != "" {
		discoveries, err = e.discoveryProvider.ListDiscoveriesByType(providerFilterType)
	} else if filterTargetID != "" {
		discoveries, err = e.discoveryProvider.ListDiscoveriesByTarget(filterTargetID)
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
		discoveryTargetID := canonicalDiscoveryTargetID(d, "")
		result := map[string]interface{}{
			"id":              d.ID,
			"resource_type":   canonicalDiscoveryResourceType(d.ResourceType),
			"resource_id":     d.ResourceID,
			"target_id":       discoveryTargetID,
			"agent_id":        d.AgentID,
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
	if filterTargetID != "" {
		response["filter_target_id"] = filterTargetID
	}
	if filterServiceType != "" {
		response["filter_service_type"] = filterServiceType
	}

	return NewJSONResult(response), nil
}
