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
			Name:        "pulse_get_capabilities",
			Description: "Get server capabilities, available features, and current configuration. Call this first to understand what tools and data are available.",
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
			Name:        "pulse_list_infrastructure",
			Description: "List all monitored infrastructure including VMs, containers, Docker hosts, and Proxmox nodes.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Filter by type: 'nodes', 'vms', 'containers', 'docker'. Omit for all.",
						Enum:        []string{"nodes", "vms", "containers", "docker"},
					},
					"status": {
						Type:        "string",
						Description: "Filter by status: 'running', 'stopped', 'all'. Default: all.",
						Enum:        []string{"running", "stopped", "all"},
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results per category (default: 100)",
					},
					"offset": {
						Type:        "integer",
						Description: "Number of results to skip per category",
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
			Name:        "pulse_get_resource",
			Description: "Get detailed information about a specific VM, container, or Docker container including IPs, ports, labels, mounts, and network configuration.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_type": {
						Type:        "string",
						Description: "Type of resource: 'vm', 'container', 'docker'",
						Enum:        []string{"vm", "container", "docker"},
					},
					"resource_id": {
						Type:        "string",
						Description: "The resource ID, VMID, or name to look up",
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
	connectedAgents := 0
	if e.agentServer != nil {
		connectedAgents = len(e.agentServer.GetConnectedAgents())
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
		ConnectedAgents: connectedAgents,
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
				Name:   node.Name,
				Status: node.Status,
				ID:     node.ID,
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
