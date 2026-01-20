package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// registerQueryTools registers infrastructure query tools
func (e *PulseToolExecutor) registerQueryTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_capabilities",
			Description: `Get server capabilities and connected agents.

Returns: JSON with control_level, enabled features, connected agent count and details.

Use when: You need to check if control features are enabled or verify agent connectivity before running commands.
Use this to confirm hostnames when choosing target_host.`,
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetCapabilities(ctx)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_get_url_content",
			Description: "Fetch content from a URL. Use to check if web services are responding or read API endpoints.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"url": {
						Type:        "string",
						Description: "The URL to fetch",
					},
				},
				Required: []string{"url"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetURLContent(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_infrastructure",
			Description: `Lightweight list of nodes, VMs, containers, and Docker hosts (summaries only).

Returns: JSON with nodes/vms/containers/docker_hosts arrays (summaries) and total counts.

Use when: You need a quick list or to find a resource without full topology. Prefer this over pulse_get_topology for large environments. Use filters to keep output small.
This is monitoring data from Pulse agents; prefer it over running commands for inventory or status checks.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Optional filter: nodes, vms, containers, or docker",
						Enum:        []string{"nodes", "vms", "containers", "docker"},
					},
					"status": {
						Type:        "string",
						Description: "Optional status filter (e.g. running, stopped, online)",
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
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListInfrastructure(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_search_resources",
			Description: `Search for resources by name or ID across nodes, VMs, containers, and Docker hosts/containers.

Returns: JSON with compact matches (type, id, name, status, node/host).

Use when: You need to locate a specific resource before calling pulse_get_resource or control tools. Use filters to keep output small.
This is monitoring data from Pulse agents; use it to identify targets before running commands.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"query": {
						Type:        "string",
						Description: "Substring to match against names, IDs, or image (Docker)",
					},
					"type": {
						Type:        "string",
						Description: "Optional filter: node, vm, container, docker, or docker_host",
						Enum:        []string{"node", "vm", "container", "docker", "docker_host"},
					},
					"status": {
						Type:        "string",
						Description: "Optional status/state filter",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 20)",
					},
					"offset": {
						Type:        "integer",
						Description: "Number of results to skip",
					},
				},
				Required: []string{"query"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeSearchResources(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_topology",
			Description: `Get live infrastructure state - all Proxmox nodes with VMs/LXC containers, and Docker hosts with containers.

Returns: JSON with proxmox.nodes[] (each with vms[], containers[], status, agent_connected), docker.hosts[] (each with containers[], status), and summary counts.

Use when: You need a full inventory or relationship view of infrastructure.
This is live monitoring data from Pulse agents; prefer it over running commands for status or inventory questions.

For targeted lookups or simple status checks, prefer pulse_search_resources or pulse_list_infrastructure to keep output small.

Options: Use include (proxmox/docker), summary_only, and max_* fields to reduce payload size.

This data is authoritative and updates every ~10 seconds. Trust it for status questions - no verification needed.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"include": {
						Type:        "string",
						Description: "Optional: limit to proxmox or docker data (default: all)",
						Enum:        []string{"all", "proxmox", "docker"},
					},
					"summary_only": {
						Type:        "boolean",
						Description: "If true, return only summary counts (no node/host lists)",
					},
					"max_nodes": {
						Type:        "integer",
						Description: "Max Proxmox nodes to include",
					},
					"max_vms_per_node": {
						Type:        "integer",
						Description: "Max VMs per node to include",
					},
					"max_containers_per_node": {
						Type:        "integer",
						Description: "Max containers per node to include",
					},
					"max_docker_hosts": {
						Type:        "integer",
						Description: "Max Docker hosts to include",
					},
					"max_docker_containers_per_host": {
						Type:        "integer",
						Description: "Max Docker containers per host to include",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetTopology(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_set_resource_url",
			Description: "Set the web URL for a resource in Pulse after discovering a web service.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_type": {
						Type:        "string",
						Description: "Type of resource: 'guest', 'docker', or 'host'",
						Enum:        []string{"guest", "docker", "host"},
					},
					"resource_id": {
						Type:        "string",
						Description: "The resource ID from context",
					},
					"url": {
						Type:        "string",
						Description: "The URL to set (empty to remove)",
					},
				},
				Required: []string{"resource_type", "resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeSetResourceURL(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_resource",
			Description: `Get detailed information about a specific VM, LXC container, or Docker container.

Returns: JSON with name, status, IPs, ports, labels, mounts, network config, CPU/memory stats.

Use when: You need detailed info about ONE specific resource (IPs, ports, config) that topology does not provide.

If you do not know the ID or name, use pulse_search_resources or pulse_list_infrastructure first.

Note: For simple status checks, use pulse_search_resources or pulse_list_infrastructure instead of full topology.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_type": {
						Type:        "string",
						Description: "Type: 'vm' (Proxmox VM), 'container' (Proxmox LXC), or 'docker' (Docker container)",
						Enum:        []string{"vm", "container", "docker"},
					},
					"resource_id": {
						Type:        "string",
						Description: "VMID number (e.g. '101') or container name",
					},
				},
				Required: []string{"resource_type", "resource_id"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetResource(ctx, args)
		},
	})
}

func (e *PulseToolExecutor) executeGetCapabilities(_ context.Context) (CallToolResult, error) {
	var agents []AgentInfo
	if e.agentServer != nil {
		connectedAgents := e.agentServer.GetConnectedAgents()
		for _, a := range connectedAgents {
			agents = append(agents, AgentInfo{
				Hostname:    a.Hostname,
				Version:     a.Version,
				Platform:    a.Platform,
				ConnectedAt: a.ConnectedAt.Format("2006-01-02T15:04:05Z"),
			})
		}
	}

	response := CapabilitiesResponse{
		ControlLevel: string(e.controlLevel),
		Features: FeatureFlags{
			MetricsHistory: e.metricsHistory != nil,
			Baselines:      e.baselineProvider != nil,
			Patterns:       e.patternProvider != nil,
			Alerts:         e.alertProvider != nil,
			Findings:       e.findingsProvider != nil,
			Backups:        e.backupProvider != nil,
			Storage:        e.storageProvider != nil,
			DiskHealth:     e.diskHealthProvider != nil,
			AgentProfiles:  e.agentProfileManager != nil,
			Control:        e.controlLevel != ControlLevelReadOnly && e.controlLevel != "",
		},
		ProtectedGuests: e.protectedGuests,
		ConnectedAgents: len(agents),
		Agents:          agents,
		Version:         ServerVersion,
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetURLContent(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return NewErrorResult(fmt.Errorf("url is required")), nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return NewJSONResult(URLFetchResponse{
			URL:   url,
			Error: err.Error(),
		}), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024))
	if err != nil {
		return NewJSONResult(URLFetchResponse{
			URL:   url,
			Error: fmt.Sprintf("error reading response: %v", err),
		}), nil
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return NewJSONResult(URLFetchResponse{
		URL:        url,
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       string(body),
	}), nil
}

func (e *PulseToolExecutor) executeListInfrastructure(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewErrorResult(fmt.Errorf("state provider not available")), nil
	}

	filterType, _ := args["type"].(string)
	filterStatus, _ := args["status"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	state := e.stateProvider.GetState()

	// Build a set of connected agent hostnames for quick lookup
	connectedAgentHostnames := make(map[string]bool)
	if e.agentServer != nil {
		for _, agent := range e.agentServer.GetConnectedAgents() {
			connectedAgentHostnames[agent.Hostname] = true
		}
	}

	response := InfrastructureResponse{
		Total: TotalCounts{
			Nodes:       len(state.Nodes),
			VMs:         len(state.VMs),
			Containers:  len(state.Containers),
			DockerHosts: len(state.DockerHosts),
		},
	}

	totalMatches := 0

	// Nodes
	if filterType == "" || filterType == "nodes" {
		count := 0
		for _, node := range state.Nodes {
			if filterStatus != "" && filterStatus != "all" && node.Status != filterStatus {
				continue
			}
			if count < offset {
				count++
				continue
			}
			if len(response.Nodes) >= limit {
				count++
				continue
			}
			response.Nodes = append(response.Nodes, NodeSummary{
				Name:           node.Name,
				Status:         node.Status,
				ID:             node.ID,
				AgentConnected: connectedAgentHostnames[node.Name],
			})
			count++
		}
		if filterType == "nodes" {
			totalMatches = count
		}
	}

	// VMs
	if filterType == "" || filterType == "vms" {
		count := 0
		for _, vm := range state.VMs {
			if filterStatus != "" && filterStatus != "all" && vm.Status != filterStatus {
				continue
			}
			if count < offset {
				count++
				continue
			}
			if len(response.VMs) >= limit {
				count++
				continue
			}
			response.VMs = append(response.VMs, VMSummary{
				VMID:   vm.VMID,
				Name:   vm.Name,
				Status: vm.Status,
				Node:   vm.Node,
				CPU:    vm.CPU * 100,
				Memory: vm.Memory.Usage * 100,
			})
			count++
		}
		if filterType == "vms" {
			totalMatches = count
		}
	}

	// Containers (LXC)
	if filterType == "" || filterType == "containers" {
		count := 0
		for _, ct := range state.Containers {
			if filterStatus != "" && filterStatus != "all" && ct.Status != filterStatus {
				continue
			}
			if count < offset {
				count++
				continue
			}
			if len(response.Containers) >= limit {
				count++
				continue
			}
			response.Containers = append(response.Containers, ContainerSummary{
				VMID:   ct.VMID,
				Name:   ct.Name,
				Status: ct.Status,
				Node:   ct.Node,
				CPU:    ct.CPU * 100,
				Memory: ct.Memory.Usage * 100,
			})
			count++
		}
		if filterType == "containers" {
			totalMatches = count
		}
	}

	// Docker hosts
	if filterType == "" || filterType == "docker" {
		count := 0
		for _, host := range state.DockerHosts {
			if count < offset {
				count++
				continue
			}
			if len(response.DockerHosts) >= limit {
				count++
				continue
			}
			dockerHost := DockerHostSummary{
				ID:             host.ID,
				Hostname:       host.Hostname,
				DisplayName:    host.DisplayName,
				ContainerCount: len(host.Containers),
				AgentConnected: connectedAgentHostnames[host.Hostname],
			}
			for _, c := range host.Containers {
				if filterStatus != "" && filterStatus != "all" && c.State != filterStatus {
					continue
				}
				dockerHost.Containers = append(dockerHost.Containers, DockerContainerSummary{
					ID:     c.ID,
					Name:   c.Name,
					State:  c.State,
					Image:  c.Image,
					Health: c.Health,
				})
			}
			if filterStatus != "" && filterStatus != "all" && len(dockerHost.Containers) == 0 {
				continue
			}
			response.DockerHosts = append(response.DockerHosts, dockerHost)
			count++
		}
		if filterType == "docker" {
			totalMatches = count
		}
	}

	if filterType != "" && (offset > 0 || totalMatches > limit) {
		response.Pagination = &PaginationInfo{
			Total:  totalMatches,
			Limit:  limit,
			Offset: offset,
		}
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetTopology(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewErrorResult(fmt.Errorf("state provider not available")), nil
	}

	include, _ := args["include"].(string)
	if include == "" {
		include = "all"
	}
	switch include {
	case "all", "proxmox", "docker":
	default:
		return NewErrorResult(fmt.Errorf("invalid include: %s. Use all, proxmox, or docker", include)), nil
	}

	summaryOnly, _ := args["summary_only"].(bool)
	maxNodes := intArg(args, "max_nodes", 0)
	maxVMsPerNode := intArg(args, "max_vms_per_node", 0)
	maxContainersPerNode := intArg(args, "max_containers_per_node", 0)
	maxDockerHosts := intArg(args, "max_docker_hosts", 0)
	maxDockerContainersPerHost := intArg(args, "max_docker_containers_per_host", 0)

	state := e.stateProvider.GetState()

	// Build a set of connected agent hostnames for quick lookup
	connectedAgentHostnames := make(map[string]bool)
	if e.agentServer != nil {
		for _, agent := range e.agentServer.GetConnectedAgents() {
			connectedAgentHostnames[agent.Hostname] = true
		}
	}

	// Check if control is enabled
	controlEnabled := e.controlLevel != ControlLevelReadOnly && e.controlLevel != ""

	includeProxmox := include == "all" || include == "proxmox"
	includeDocker := include == "all" || include == "docker"

	// Summary counters
	summary := TopologySummary{
		TotalNodes:         len(state.Nodes),
		TotalVMs:           len(state.VMs),
		TotalLXCContainers: len(state.Containers),
		TotalDockerHosts:   len(state.DockerHosts),
	}

	for _, node := range state.Nodes {
		if connectedAgentHostnames[node.Name] {
			summary.NodesWithAgents++
		}
	}
	for _, host := range state.DockerHosts {
		if connectedAgentHostnames[host.Hostname] {
			summary.DockerHostsWithAgents++
		}
	}

	// Build Proxmox topology - group VMs and containers by node
	nodeMap := make(map[string]*ProxmoxNodeTopology)
	if includeProxmox && !summaryOnly {
		for _, node := range state.Nodes {
			if maxNodes > 0 && len(nodeMap) >= maxNodes {
				break
			}
			hasAgent := connectedAgentHostnames[node.Name]
			nodeMap[node.Name] = &ProxmoxNodeTopology{
				Name:           node.Name,
				ID:             node.ID,
				Status:         node.Status,
				AgentConnected: hasAgent,
				CanExecute:     hasAgent && controlEnabled,
				VMs:            []TopologyVM{},
				Containers:     []TopologyLXC{},
			}
		}
	}

	ensureNode := func(name, id, status string) *ProxmoxNodeTopology {
		if !includeProxmox || summaryOnly {
			return nil
		}
		if node, exists := nodeMap[name]; exists {
			return node
		}
		if maxNodes > 0 && len(nodeMap) >= maxNodes {
			return nil
		}
		hasAgent := connectedAgentHostnames[name]
		nodeMap[name] = &ProxmoxNodeTopology{
			Name:           name,
			ID:             id,
			Status:         status,
			AgentConnected: hasAgent,
			CanExecute:     hasAgent && controlEnabled,
			VMs:            []TopologyVM{},
			Containers:     []TopologyLXC{},
		}
		return nodeMap[name]
	}

	// Add VMs to their nodes
	for _, vm := range state.VMs {
		if vm.Status == "running" {
			summary.RunningVMs++
		}

		nodeTopology := ensureNode(vm.Node, "", "unknown")
		if nodeTopology == nil {
			continue
		}

		nodeTopology.VMCount++
		if maxVMsPerNode <= 0 || len(nodeTopology.VMs) < maxVMsPerNode {
			nodeTopology.VMs = append(nodeTopology.VMs, TopologyVM{
				VMID:   vm.VMID,
				Name:   vm.Name,
				Status: vm.Status,
				CPU:    vm.CPU * 100,
				Memory: vm.Memory.Usage * 100,
				OS:     vm.OSName,
				Tags:   vm.Tags,
			})
		}
	}

	// Add containers to their nodes
	for _, ct := range state.Containers {
		if ct.Status == "running" {
			summary.RunningLXC++
		}

		nodeTopology := ensureNode(ct.Node, "", "unknown")
		if nodeTopology == nil {
			continue
		}

		nodeTopology.ContainerCount++
		if maxContainersPerNode <= 0 || len(nodeTopology.Containers) < maxContainersPerNode {
			nodeTopology.Containers = append(nodeTopology.Containers, TopologyLXC{
				VMID:      ct.VMID,
				Name:      ct.Name,
				Status:    ct.Status,
				CPU:       ct.CPU * 100,
				Memory:    ct.Memory.Usage * 100,
				OS:        ct.OSName,
				Tags:      ct.Tags,
				HasDocker: ct.HasDocker,
			})
		}
	}

	// Convert node map to slice
	proxmoxNodes := []ProxmoxNodeTopology{}
	if includeProxmox && !summaryOnly {
		for _, node := range nodeMap {
			proxmoxNodes = append(proxmoxNodes, *node)
		}
	}

	// Build Docker topology
	dockerHosts := []DockerHostTopology{}
	for _, host := range state.DockerHosts {
		hasAgent := connectedAgentHostnames[host.Hostname]
		runningCount := 0
		var containers []DockerContainerSummary

		for _, c := range host.Containers {
			if c.State == "running" {
				runningCount++
				summary.RunningDocker++
			}
			summary.TotalDockerContainers++

			if includeDocker && !summaryOnly {
				if maxDockerContainersPerHost <= 0 || len(containers) < maxDockerContainersPerHost {
					containers = append(containers, DockerContainerSummary{
						ID:     c.ID,
						Name:   c.Name,
						State:  c.State,
						Image:  c.Image,
						Health: c.Health,
					})
				}
			}
		}

		if includeDocker && !summaryOnly {
			if maxDockerHosts > 0 && len(dockerHosts) >= maxDockerHosts {
				continue
			}

			dockerHosts = append(dockerHosts, DockerHostTopology{
				Hostname:       host.Hostname,
				DisplayName:    host.DisplayName,
				AgentConnected: hasAgent,
				CanExecute:     hasAgent && controlEnabled,
				Containers:     containers,
				ContainerCount: len(host.Containers),
				RunningCount:   runningCount,
			})
		}
	}

	response := TopologyResponse{
		Proxmox: ProxmoxTopology{Nodes: proxmoxNodes},
		Docker:  DockerTopology{Hosts: dockerHosts},
		Summary: summary,
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeSetResourceURL(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)
	url, _ := args["url"].(string)

	if resourceType == "" {
		return NewErrorResult(fmt.Errorf("resource_type is required")), nil
	}
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	if e.metadataUpdater == nil {
		return NewTextResult("Metadata updater not available."), nil
	}

	if err := e.metadataUpdater.SetResourceURL(resourceType, resourceID, url); err != nil {
		return NewErrorResult(err), nil
	}

	result := map[string]interface{}{
		"success":       true,
		"resource_type": resourceType,
		"resource_id":   resourceID,
		"url":           url,
	}
	if url == "" {
		result["action"] = "cleared"
	} else {
		result["action"] = "set"
	}

	return NewJSONResult(result), nil
}

func (e *PulseToolExecutor) executeGetResource(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceType, _ := args["resource_type"].(string)
	resourceID, _ := args["resource_id"].(string)

	if resourceType == "" {
		return NewErrorResult(fmt.Errorf("resource_type is required")), nil
	}
	if resourceID == "" {
		return NewErrorResult(fmt.Errorf("resource_id is required")), nil
	}

	if e.stateProvider == nil {
		return NewTextResult("State information not available."), nil
	}

	state := e.stateProvider.GetState()

	switch resourceType {
	case "vm":
		for _, vm := range state.VMs {
			if fmt.Sprintf("%d", vm.VMID) == resourceID || vm.Name == resourceID || vm.ID == resourceID {
				response := ResourceResponse{
					Type:   "vm",
					ID:     vm.ID,
					Name:   vm.Name,
					Status: vm.Status,
					Node:   vm.Node,
					CPU: ResourceCPU{
						Percent: vm.CPU * 100,
						Cores:   vm.CPUs,
					},
					Memory: ResourceMemory{
						Percent: vm.Memory.Usage * 100,
						UsedGB:  float64(vm.Memory.Used) / (1024 * 1024 * 1024),
						TotalGB: float64(vm.Memory.Total) / (1024 * 1024 * 1024),
					},
					OS:   vm.OSName,
					Tags: vm.Tags,
				}
				if !vm.LastBackup.IsZero() {
					response.LastBackup = &vm.LastBackup
				}
				for _, nic := range vm.NetworkInterfaces {
					response.Networks = append(response.Networks, NetworkInfo{
						Name:      nic.Name,
						Addresses: nic.Addresses,
					})
				}
				return NewJSONResult(response), nil
			}
		}
		return NewJSONResult(map[string]interface{}{
			"error":       "not_found",
			"resource_id": resourceID,
			"type":        "vm",
		}), nil

	case "container":
		for _, ct := range state.Containers {
			if fmt.Sprintf("%d", ct.VMID) == resourceID || ct.Name == resourceID || ct.ID == resourceID {
				response := ResourceResponse{
					Type:   "container",
					ID:     ct.ID,
					Name:   ct.Name,
					Status: ct.Status,
					Node:   ct.Node,
					CPU: ResourceCPU{
						Percent: ct.CPU * 100,
						Cores:   ct.CPUs,
					},
					Memory: ResourceMemory{
						Percent: ct.Memory.Usage * 100,
						UsedGB:  float64(ct.Memory.Used) / (1024 * 1024 * 1024),
						TotalGB: float64(ct.Memory.Total) / (1024 * 1024 * 1024),
					},
					OS:   ct.OSName,
					Tags: ct.Tags,
				}
				if !ct.LastBackup.IsZero() {
					response.LastBackup = &ct.LastBackup
				}
				for _, nic := range ct.NetworkInterfaces {
					response.Networks = append(response.Networks, NetworkInfo{
						Name:      nic.Name,
						Addresses: nic.Addresses,
					})
				}
				return NewJSONResult(response), nil
			}
		}
		return NewJSONResult(map[string]interface{}{
			"error":       "not_found",
			"resource_id": resourceID,
			"type":        "container",
		}), nil

	case "docker":
		for _, host := range state.DockerHosts {
			for _, c := range host.Containers {
				if c.ID == resourceID || c.Name == resourceID || strings.HasPrefix(c.ID, resourceID) {
					response := ResourceResponse{
						Type:   "docker",
						ID:     c.ID,
						Name:   c.Name,
						Status: c.State,
						Host:   host.Hostname,
						Image:  c.Image,
						Health: c.Health,
						CPU: ResourceCPU{
							Percent: c.CPUPercent,
						},
						Memory: ResourceMemory{
							Percent: c.MemoryPercent,
							UsedGB:  float64(c.MemoryUsage) / (1024 * 1024 * 1024),
							TotalGB: float64(c.MemoryLimit) / (1024 * 1024 * 1024),
						},
						RestartCount: c.RestartCount,
						Labels:       c.Labels,
					}

					if c.UpdateStatus != nil && c.UpdateStatus.UpdateAvailable {
						response.UpdateAvailable = true
					}

					for _, p := range c.Ports {
						response.Ports = append(response.Ports, PortInfo{
							Private:  p.PrivatePort,
							Public:   p.PublicPort,
							Protocol: p.Protocol,
							IP:       p.IP,
						})
					}

					for _, n := range c.Networks {
						response.Networks = append(response.Networks, NetworkInfo{
							Name:      n.Name,
							Addresses: []string{n.IPv4},
						})
					}

					for _, m := range c.Mounts {
						response.Mounts = append(response.Mounts, MountInfo{
							Source:      m.Source,
							Destination: m.Destination,
							ReadWrite:   m.RW,
						})
					}

					return NewJSONResult(response), nil
				}
			}
		}
		return NewJSONResult(map[string]interface{}{
			"error":       "not_found",
			"resource_id": resourceID,
			"type":        "docker",
		}), nil

	default:
		return NewErrorResult(fmt.Errorf("invalid resource_type: %s. Use 'vm', 'container', or 'docker'", resourceType)), nil
	}
}

func (e *PulseToolExecutor) executeSearchResources(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	rawQuery, _ := args["query"].(string)
	query := strings.TrimSpace(rawQuery)
	if query == "" {
		return NewErrorResult(fmt.Errorf("query is required")), nil
	}

	typeFilter, _ := args["type"].(string)
	statusFilter, _ := args["status"].(string)
	limit := intArg(args, "limit", 20)
	offset := intArg(args, "offset", 0)
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	allowedTypes := map[string]bool{
		"":            true,
		"node":        true,
		"vm":          true,
		"container":   true,
		"docker":      true,
		"docker_host": true,
	}
	if !allowedTypes[typeFilter] {
		return NewErrorResult(fmt.Errorf("invalid type: %s. Use node, vm, container, docker, or docker_host", typeFilter)), nil
	}

	matchesQuery := func(query string, candidates ...string) bool {
		for _, candidate := range candidates {
			if candidate == "" {
				continue
			}
			if strings.Contains(strings.ToLower(candidate), query) {
				return true
			}
		}
		return false
	}

	queryLower := strings.ToLower(query)
	state := e.stateProvider.GetState()

	matches := make([]ResourceMatch, 0, limit)
	total := 0

	addMatch := func(match ResourceMatch) {
		if total < offset {
			total++
			return
		}
		if len(matches) >= limit {
			total++
			return
		}
		matches = append(matches, match)
		total++
	}

	if typeFilter == "" || typeFilter == "node" {
		for _, node := range state.Nodes {
			if statusFilter != "" && !strings.EqualFold(node.Status, statusFilter) {
				continue
			}
			if !matchesQuery(queryLower, node.Name, node.ID) {
				continue
			}
			addMatch(ResourceMatch{
				Type:   "node",
				ID:     node.ID,
				Name:   node.Name,
				Status: node.Status,
			})
		}
	}

	if typeFilter == "" || typeFilter == "vm" {
		for _, vm := range state.VMs {
			if statusFilter != "" && !strings.EqualFold(vm.Status, statusFilter) {
				continue
			}
			if !matchesQuery(queryLower, vm.Name, vm.ID, fmt.Sprintf("%d", vm.VMID)) {
				continue
			}
			addMatch(ResourceMatch{
				Type:   "vm",
				ID:     vm.ID,
				Name:   vm.Name,
				Status: vm.Status,
				Node:   vm.Node,
				VMID:   vm.VMID,
			})
		}
	}

	if typeFilter == "" || typeFilter == "container" {
		for _, ct := range state.Containers {
			if statusFilter != "" && !strings.EqualFold(ct.Status, statusFilter) {
				continue
			}
			if !matchesQuery(queryLower, ct.Name, ct.ID, fmt.Sprintf("%d", ct.VMID)) {
				continue
			}
			addMatch(ResourceMatch{
				Type:   "container",
				ID:     ct.ID,
				Name:   ct.Name,
				Status: ct.Status,
				Node:   ct.Node,
				VMID:   ct.VMID,
			})
		}
	}

	if typeFilter == "" || typeFilter == "docker_host" {
		for _, host := range state.DockerHosts {
			if statusFilter != "" && !strings.EqualFold(host.Status, statusFilter) {
				continue
			}
			if !matchesQuery(queryLower, host.ID, host.Hostname, host.DisplayName, host.CustomDisplayName) {
				continue
			}
			displayName := host.DisplayName
			if host.CustomDisplayName != "" {
				displayName = host.CustomDisplayName
			}
			if displayName == "" {
				displayName = host.Hostname
			}
			addMatch(ResourceMatch{
				Type:   "docker_host",
				ID:     host.ID,
				Name:   displayName,
				Status: host.Status,
				Host:   host.Hostname,
			})
		}
	}

	if typeFilter == "" || typeFilter == "docker" {
		for _, host := range state.DockerHosts {
			for _, c := range host.Containers {
				if statusFilter != "" && !strings.EqualFold(c.State, statusFilter) {
					continue
				}
				if !matchesQuery(queryLower, c.Name, c.ID, c.Image) {
					continue
				}
				addMatch(ResourceMatch{
					Type:   "docker",
					ID:     c.ID,
					Name:   c.Name,
					Status: c.State,
					Host:   host.Hostname,
					Image:  c.Image,
				})
			}
		}
	}

	response := ResourceSearchResponse{
		Query:   query,
		Matches: matches,
		Total:   total,
	}

	if offset > 0 || total > limit {
		response.Pagination = &PaginationInfo{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}
	}

	return NewJSONResult(response), nil
}

// Helper to get int args with default
func intArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case int64:
			return int(val)
		}
	}
	return defaultVal
}

// ========== Kubernetes Tools ==========

// registerKubernetesTools registers Kubernetes query tools
func (e *PulseToolExecutor) registerKubernetesTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_kubernetes_clusters",
			Description: `List Kubernetes clusters monitored by Pulse with health summary.

Returns: JSON with clusters array containing id, name, status, version, node count, pod count, deployment count.

Use when: User asks about Kubernetes clusters, wants an overview of K8s infrastructure, or needs to find a specific cluster.`,
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetKubernetesClusters(ctx)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_kubernetes_nodes",
			Description: `List nodes in a Kubernetes cluster with capacity and status.

Returns: JSON with nodes array containing name, ready status, roles, kubelet version, capacity (CPU, memory, pods), allocatable resources.

Use when: User asks about Kubernetes nodes, node health, or cluster capacity.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"cluster": {
						Type:        "string",
						Description: "Cluster name or ID (required)",
					},
				},
				Required: []string{"cluster"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetKubernetesNodes(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_kubernetes_pods",
			Description: `List pods in a Kubernetes cluster, optionally filtered by namespace or status.

Returns: JSON with pods array containing name, namespace, node, phase, restarts, containers with their states.

Use when: User asks about pods, wants to find a specific pod, or check pod health in a namespace.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"cluster": {
						Type:        "string",
						Description: "Cluster name or ID (required)",
					},
					"namespace": {
						Type:        "string",
						Description: "Optional: filter by namespace",
					},
					"status": {
						Type:        "string",
						Description: "Optional: filter by pod phase (Running, Pending, Failed, Succeeded)",
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
				Required: []string{"cluster"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetKubernetesPods(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_kubernetes_deployments",
			Description: `List deployments in a Kubernetes cluster with replica status.

Returns: JSON with deployments array containing name, namespace, desired/ready/available/updated replicas.

Use when: User asks about deployments, wants to check deployment health, or find unhealthy deployments.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"cluster": {
						Type:        "string",
						Description: "Cluster name or ID (required)",
					},
					"namespace": {
						Type:        "string",
						Description: "Optional: filter by namespace",
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
				Required: []string{"cluster"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetKubernetesDeployments(ctx, args)
		},
	})
}

func (e *PulseToolExecutor) executeGetKubernetesClusters(_ context.Context) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	state := e.stateProvider.GetState()

	if len(state.KubernetesClusters) == 0 {
		return NewTextResult("No Kubernetes clusters found. Kubernetes monitoring may not be configured."), nil
	}

	var clusters []KubernetesClusterSummary
	for _, c := range state.KubernetesClusters {
		readyNodes := 0
		for _, node := range c.Nodes {
			if node.Ready {
				readyNodes++
			}
		}

		displayName := c.DisplayName
		if c.CustomDisplayName != "" {
			displayName = c.CustomDisplayName
		}

		clusters = append(clusters, KubernetesClusterSummary{
			ID:              c.ID,
			Name:            c.Name,
			DisplayName:     displayName,
			Server:          c.Server,
			Version:         c.Version,
			Status:          c.Status,
			NodeCount:       len(c.Nodes),
			PodCount:        len(c.Pods),
			DeploymentCount: len(c.Deployments),
			ReadyNodes:      readyNodes,
		})
	}

	response := KubernetesClustersResponse{
		Clusters: clusters,
		Total:    len(clusters),
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetKubernetesNodes(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	state := e.stateProvider.GetState()

	// Find the cluster (also match CustomDisplayName)
	var cluster *KubernetesClusterSummary
	for _, c := range state.KubernetesClusters {
		if c.ID == clusterArg || c.Name == clusterArg || c.DisplayName == clusterArg || c.CustomDisplayName == clusterArg {
			displayName := c.DisplayName
			if c.CustomDisplayName != "" {
				displayName = c.CustomDisplayName
			}
			cluster = &KubernetesClusterSummary{
				ID:          c.ID,
				Name:        c.Name,
				DisplayName: displayName,
			}

			var nodes []KubernetesNodeSummary
			for _, node := range c.Nodes {
				nodes = append(nodes, KubernetesNodeSummary{
					UID:                     node.UID,
					Name:                    node.Name,
					Ready:                   node.Ready,
					Unschedulable:           node.Unschedulable,
					Roles:                   node.Roles,
					KubeletVersion:          node.KubeletVersion,
					ContainerRuntimeVersion: node.ContainerRuntimeVersion,
					OSImage:                 node.OSImage,
					Architecture:            node.Architecture,
					CapacityCPU:             node.CapacityCPU,
					CapacityMemoryBytes:     node.CapacityMemoryBytes,
					CapacityPods:            node.CapacityPods,
					AllocatableCPU:          node.AllocCPU,
					AllocatableMemoryBytes:  node.AllocMemoryBytes,
					AllocatablePods:         node.AllocPods,
				})
			}

			response := KubernetesNodesResponse{
				Cluster: cluster.DisplayName,
				Nodes:   nodes,
				Total:   len(nodes),
			}
			if response.Nodes == nil {
				response.Nodes = []KubernetesNodeSummary{}
			}
			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
}

func (e *PulseToolExecutor) executeGetKubernetesPods(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	namespaceFilter, _ := args["namespace"].(string)
	statusFilter, _ := args["status"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	state := e.stateProvider.GetState()

	// Find the cluster (also match CustomDisplayName)
	for _, c := range state.KubernetesClusters {
		if c.ID == clusterArg || c.Name == clusterArg || c.DisplayName == clusterArg || c.CustomDisplayName == clusterArg {
			displayName := c.DisplayName
			if c.CustomDisplayName != "" {
				displayName = c.CustomDisplayName
			}

			var pods []KubernetesPodSummary
			totalPods := 0
			filteredCount := 0

			for _, pod := range c.Pods {
				// Apply filters
				if namespaceFilter != "" && pod.Namespace != namespaceFilter {
					continue
				}
				if statusFilter != "" && !strings.EqualFold(pod.Phase, statusFilter) {
					continue
				}

				filteredCount++

				// Apply pagination
				if totalPods < offset {
					totalPods++
					continue
				}
				if len(pods) >= limit {
					totalPods++
					continue
				}

				var containers []KubernetesPodContainerSummary
				for _, container := range pod.Containers {
					containers = append(containers, KubernetesPodContainerSummary{
						Name:         container.Name,
						Ready:        container.Ready,
						State:        container.State,
						RestartCount: container.RestartCount,
						Reason:       container.Reason,
					})
				}

				pods = append(pods, KubernetesPodSummary{
					UID:        pod.UID,
					Name:       pod.Name,
					Namespace:  pod.Namespace,
					NodeName:   pod.NodeName,
					Phase:      pod.Phase,
					Reason:     pod.Reason,
					Restarts:   pod.Restarts,
					QoSClass:   pod.QoSClass,
					OwnerKind:  pod.OwnerKind,
					OwnerName:  pod.OwnerName,
					Containers: containers,
				})
				totalPods++
			}

			response := KubernetesPodsResponse{
				Cluster:  displayName,
				Pods:     pods,
				Total:    len(c.Pods),
				Filtered: filteredCount,
			}
			if response.Pods == nil {
				response.Pods = []KubernetesPodSummary{}
			}
			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
}

func (e *PulseToolExecutor) executeGetKubernetesDeployments(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	clusterArg, _ := args["cluster"].(string)
	if clusterArg == "" {
		return NewErrorResult(fmt.Errorf("cluster is required")), nil
	}

	namespaceFilter, _ := args["namespace"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	state := e.stateProvider.GetState()

	// Find the cluster (also match CustomDisplayName)
	for _, c := range state.KubernetesClusters {
		if c.ID == clusterArg || c.Name == clusterArg || c.DisplayName == clusterArg || c.CustomDisplayName == clusterArg {
			displayName := c.DisplayName
			if c.CustomDisplayName != "" {
				displayName = c.CustomDisplayName
			}

			var deployments []KubernetesDeploymentSummary
			filteredCount := 0
			count := 0

			for _, dep := range c.Deployments {
				// Apply namespace filter
				if namespaceFilter != "" && dep.Namespace != namespaceFilter {
					continue
				}

				filteredCount++

				// Apply pagination
				if count < offset {
					count++
					continue
				}
				if len(deployments) >= limit {
					count++
					continue
				}

				deployments = append(deployments, KubernetesDeploymentSummary{
					UID:               dep.UID,
					Name:              dep.Name,
					Namespace:         dep.Namespace,
					DesiredReplicas:   dep.DesiredReplicas,
					ReadyReplicas:     dep.ReadyReplicas,
					AvailableReplicas: dep.AvailableReplicas,
					UpdatedReplicas:   dep.UpdatedReplicas,
				})
				count++
			}

			response := KubernetesDeploymentsResponse{
				Cluster:     displayName,
				Deployments: deployments,
				Total:       len(c.Deployments),
				Filtered:    filteredCount,
			}
			if response.Deployments == nil {
				response.Deployments = []KubernetesDeploymentSummary{}
			}
			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Kubernetes cluster '%s' not found.", clusterArg)), nil
}
