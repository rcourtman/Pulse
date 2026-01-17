package mcp

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

Use when: You need to check if control features are enabled or verify agent connectivity before running commands.`,
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
			Name: "pulse_get_topology",
			Description: `Get live infrastructure state - all Proxmox nodes with VMs/LXC containers, and Docker hosts with containers.

Returns: JSON with 'proxmox.nodes[]' (each with vms[], containers[], status, agent_connected), 'docker.hosts[]' (each with containers[], status), and 'summary' counts.

Use when: Checking if something is running, finding a VM/container by name, getting current infrastructure state.

Example: User asks "is nginx running?" → call this, find nginx in the response, report its status field directly.

This data is authoritative and updates every ~10 seconds. Trust it for status questions - no verification needed.`,
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetTopology(ctx)
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

Use when: You need detailed info about ONE specific resource (IPs, ports, config) that topology doesn't provide.

Note: For simple status checks, use pulse_get_topology instead - it's faster and shows all resources.`,
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

	// Nodes
	if filterType == "" || filterType == "nodes" {
		for i, node := range state.Nodes {
			if i < offset {
				continue
			}
			if len(response.Nodes) >= limit {
				break
			}
			if filterStatus != "" && filterStatus != "all" && node.Status != filterStatus {
				continue
			}
			response.Nodes = append(response.Nodes, NodeSummary{
				Name:           node.Name,
				Status:         node.Status,
				ID:             node.ID,
				AgentConnected: connectedAgentHostnames[node.Name],
			})
		}
	}

	// VMs
	if filterType == "" || filterType == "vms" {
		for i, vm := range state.VMs {
			if i < offset {
				continue
			}
			if len(response.VMs) >= limit {
				break
			}
			if filterStatus != "" && filterStatus != "all" && vm.Status != filterStatus {
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
		}
	}

	// Containers (LXC)
	if filterType == "" || filterType == "containers" {
		for i, ct := range state.Containers {
			if i < offset {
				continue
			}
			if len(response.Containers) >= limit {
				break
			}
			if filterStatus != "" && filterStatus != "all" && ct.Status != filterStatus {
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
		}
	}

	// Docker hosts
	if filterType == "" || filterType == "docker" {
		for i, host := range state.DockerHosts {
			if i < offset {
				continue
			}
			if len(response.DockerHosts) >= limit {
				break
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
			response.DockerHosts = append(response.DockerHosts, dockerHost)
		}
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetTopology(_ context.Context) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewErrorResult(fmt.Errorf("state provider not available")), nil
	}

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

	// Build Proxmox topology - group VMs and containers by node
	nodeMap := make(map[string]*ProxmoxNodeTopology)

	// Initialize nodes from state
	for _, node := range state.Nodes {
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

	// Summary counters
	summary := TopologySummary{
		TotalNodes:         len(state.Nodes),
		TotalVMs:           len(state.VMs),
		TotalLXCContainers: len(state.Containers),
		TotalDockerHosts:   len(state.DockerHosts),
	}

	// Add VMs to their nodes
	for _, vm := range state.VMs {
		nodeTopology, exists := nodeMap[vm.Node]
		if !exists {
			// Create node entry if it doesn't exist (shouldn't happen normally)
			hasAgent := connectedAgentHostnames[vm.Node]
			nodeTopology = &ProxmoxNodeTopology{
				Name:           vm.Node,
				Status:         "unknown",
				AgentConnected: hasAgent,
				CanExecute:     hasAgent && controlEnabled,
				VMs:            []TopologyVM{},
				Containers:     []TopologyLXC{},
			}
			nodeMap[vm.Node] = nodeTopology
		}

		nodeTopology.VMs = append(nodeTopology.VMs, TopologyVM{
			VMID:   vm.VMID,
			Name:   vm.Name,
			Status: vm.Status,
			CPU:    vm.CPU * 100,
			Memory: vm.Memory.Usage * 100,
			OS:     vm.OSName,
			Tags:   vm.Tags,
		})
		nodeTopology.VMCount++

		if vm.Status == "running" {
			summary.RunningVMs++
		}
	}

	// Add containers to their nodes
	for _, ct := range state.Containers {
		nodeTopology, exists := nodeMap[ct.Node]
		if !exists {
			hasAgent := connectedAgentHostnames[ct.Node]
			nodeTopology = &ProxmoxNodeTopology{
				Name:           ct.Node,
				Status:         "unknown",
				AgentConnected: hasAgent,
				CanExecute:     hasAgent && controlEnabled,
				VMs:            []TopologyVM{},
				Containers:     []TopologyLXC{},
			}
			nodeMap[ct.Node] = nodeTopology
		}

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
		nodeTopology.ContainerCount++

		if ct.Status == "running" {
			summary.RunningLXC++
		}
	}

	// Convert node map to slice
	var proxmoxNodes []ProxmoxNodeTopology
	for _, node := range nodeMap {
		if node.AgentConnected {
			summary.NodesWithAgents++
		}
		proxmoxNodes = append(proxmoxNodes, *node)
	}

	// Build Docker topology
	var dockerHosts []DockerHostTopology
	for _, host := range state.DockerHosts {
		hasAgent := connectedAgentHostnames[host.Hostname]
		runningCount := 0
		var containers []DockerContainerSummary

		for _, c := range host.Containers {
			containers = append(containers, DockerContainerSummary{
				ID:     c.ID,
				Name:   c.Name,
				State:  c.State,
				Image:  c.Image,
				Health: c.Health,
			})
			summary.TotalDockerContainers++
			if c.State == "running" {
				runningCount++
				summary.RunningDocker++
			}
		}

		if hasAgent {
			summary.DockerHostsWithAgents++
		}

		dockerHosts = append(dockerHosts, DockerHostTopology{
			Hostname:       host.Hostname,
			DisplayName:    host.DisplayName,
			AgentConnected: hasAgent,
			CanExecute:     hasAgent && controlEnabled,
			Containers:     containers,
			ContainerCount: len(containers),
			RunningCount:   runningCount,
		})
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
