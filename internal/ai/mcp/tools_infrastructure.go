package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// registerInfrastructureTools registers infrastructure context tools (backup, storage, disk health)
func (e *PulseToolExecutor) registerInfrastructureTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_list_backups",
			Description: "List backup status for VMs and containers. Shows last backup times, backup jobs, and identifies resources without recent backups.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "Optional: filter by specific VM or container ID",
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
			return exec.executeListBackups(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_list_storage",
			Description: "List storage pool information including usage, ZFS pool health, and Ceph cluster status.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"storage_id": {
						Type:        "string",
						Description: "Optional: specific storage ID for detailed info",
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
			return exec.executeListStorage(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_get_disk_health",
			Description: "Get disk health information including SMART data, RAID array status, and Ceph cluster health from host agents.",
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetDiskHealth(ctx, args)
		},
	})

	// Docker Updates Tools
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_docker_updates",
			Description: `List Docker containers with pending image updates.

Returns: JSON with containers that have newer images available in their registry, including image names, current/latest digests, and any check errors.

Use when: User asks about available Docker updates, which containers need updating, or wants to see update status across Docker hosts.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Optional: filter by Docker host name or ID",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListDockerUpdates(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_check_docker_updates",
			Description: `Trigger an update check for Docker containers on a host.

The Docker agent will check registries for newer images and report back. Results appear in pulse_list_docker_updates after the next agent report cycle (~30 seconds).

Use when: User wants to refresh/rescan for available Docker updates on a specific host.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Docker host name or ID to check for updates",
					},
				},
				Required: []string{"host"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeCheckDockerUpdates(ctx, args)
		},
		RequireControl: true,
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_update_docker_container",
			Description: `Update a Docker container to its latest image.

This pulls the latest image, stops the container, recreates it with the same configuration, and starts it. The old container is kept as a backup and automatically cleaned up after 5 minutes if the new container is stable.

Use when: User explicitly asks to update a specific Docker container to its latest version.

Do NOT use for: Checking what updates are available (use pulse_list_docker_updates), or just restarting a container (use pulse_control_docker).`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"container": {
						Type:        "string",
						Description: "Container name or ID to update",
					},
					"host": {
						Type:        "string",
						Description: "Docker host name or ID where the container runs",
					},
				},
				Required: []string{"container", "host"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeUpdateDockerContainer(ctx, args)
		},
		RequireControl: true,
	})
}

func (e *PulseToolExecutor) executeListBackups(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	resourceID, _ := args["resource_id"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	if e.backupProvider == nil {
		return NewTextResult("Backup information not available."), nil
	}

	backups := e.backupProvider.GetBackups()
	pbsInstances := e.backupProvider.GetPBSInstances()

	response := BackupsResponse{}

	// PBS Backups
	count := 0
	for _, b := range backups.PBS {
		if resourceID != "" && b.VMID != resourceID {
			continue
		}
		if count < offset {
			count++
			continue
		}
		if len(response.PBS) >= limit {
			break
		}
		response.PBS = append(response.PBS, PBSBackupSummary{
			VMID:       b.VMID,
			BackupType: b.BackupType,
			BackupTime: b.BackupTime,
			Instance:   b.Instance,
			Datastore:  b.Datastore,
			SizeGB:     float64(b.Size) / (1024 * 1024 * 1024),
			Verified:   b.Verified,
			Protected:  b.Protected,
		})
		count++
	}

	// PVE Backups
	count = 0
	for _, b := range backups.PVE.StorageBackups {
		if resourceID != "" && string(rune(b.VMID)) != resourceID {
			continue
		}
		if count < offset {
			count++
			continue
		}
		if len(response.PVE) >= limit {
			break
		}
		response.PVE = append(response.PVE, PVEBackupSummary{
			VMID:       b.VMID,
			BackupTime: b.Time,
			SizeGB:     float64(b.Size) / (1024 * 1024 * 1024),
			Storage:    b.Storage,
		})
		count++
	}

	// PBS Servers
	for _, pbs := range pbsInstances {
		server := PBSServerSummary{
			Name:   pbs.Name,
			Host:   pbs.Host,
			Status: pbs.Status,
		}
		for _, ds := range pbs.Datastores {
			server.Datastores = append(server.Datastores, DatastoreSummary{
				Name:         ds.Name,
				UsagePercent: ds.Usage * 100,
				FreeGB:       float64(ds.Free) / (1024 * 1024 * 1024),
			})
		}
		response.PBSServers = append(response.PBSServers, server)
	}

	// Recent tasks
	for _, t := range backups.PVE.BackupTasks {
		if len(response.RecentTasks) >= 20 {
			break
		}
		response.RecentTasks = append(response.RecentTasks, BackupTaskSummary{
			VMID:      t.VMID,
			Node:      t.Node,
			Status:    t.Status,
			StartTime: t.StartTime,
		})
	}

	// Ensure non-nil slices
	if response.PBS == nil {
		response.PBS = []PBSBackupSummary{}
	}
	if response.PVE == nil {
		response.PVE = []PVEBackupSummary{}
	}
	if response.PBSServers == nil {
		response.PBSServers = []PBSServerSummary{}
	}
	if response.RecentTasks == nil {
		response.RecentTasks = []BackupTaskSummary{}
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeListStorage(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	storageID, _ := args["storage_id"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	if e.storageProvider == nil {
		return NewTextResult("Storage information not available."), nil
	}

	storage := e.storageProvider.GetStorage()
	cephClusters := e.storageProvider.GetCephClusters()

	response := StorageResponse{}

	// Storage pools
	count := 0
	for _, s := range storage {
		if storageID != "" && s.ID != storageID && s.Name != storageID {
			continue
		}
		if count < offset {
			count++
			continue
		}
		if len(response.Pools) >= limit {
			break
		}

		pool := StoragePoolSummary{
			ID:           s.ID,
			Name:         s.Name,
			Type:         s.Type,
			Status:       s.Status,
			UsagePercent: s.Usage * 100,
			UsedGB:       float64(s.Used) / (1024 * 1024 * 1024),
			TotalGB:      float64(s.Total) / (1024 * 1024 * 1024),
			FreeGB:       float64(s.Free) / (1024 * 1024 * 1024),
			Content:      s.Content,
			Shared:       s.Shared,
		}

		if s.ZFSPool != nil {
			pool.ZFS = &ZFSPoolSummary{
				Name:           s.ZFSPool.Name,
				State:          s.ZFSPool.State,
				ReadErrors:     s.ZFSPool.ReadErrors,
				WriteErrors:    s.ZFSPool.WriteErrors,
				ChecksumErrors: s.ZFSPool.ChecksumErrors,
				Scan:           s.ZFSPool.Scan,
			}
		}

		response.Pools = append(response.Pools, pool)
		count++
	}

	// Ceph clusters
	for _, c := range cephClusters {
		response.CephClusters = append(response.CephClusters, CephClusterSummary{
			Name:          c.Name,
			Health:        c.Health,
			HealthMessage: c.HealthMessage,
			UsagePercent:  c.UsagePercent,
			UsedTB:        float64(c.UsedBytes) / (1024 * 1024 * 1024 * 1024),
			TotalTB:       float64(c.TotalBytes) / (1024 * 1024 * 1024 * 1024),
			NumOSDs:       c.NumOSDs,
			NumOSDsUp:     c.NumOSDsUp,
			NumOSDsIn:     c.NumOSDsIn,
			NumMons:       c.NumMons,
			NumMgrs:       c.NumMgrs,
		})
	}

	// Ensure non-nil slices
	if response.Pools == nil {
		response.Pools = []StoragePoolSummary{}
	}
	if response.CephClusters == nil {
		response.CephClusters = []CephClusterSummary{}
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeGetDiskHealth(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.diskHealthProvider == nil && e.storageProvider == nil {
		return NewTextResult("Disk health information not available."), nil
	}

	response := DiskHealthResponse{
		Hosts: []HostDiskHealth{},
	}

	// SMART and RAID data from host agents
	if e.diskHealthProvider != nil {
		hosts := e.diskHealthProvider.GetHosts()
		for _, host := range hosts {
			hostHealth := HostDiskHealth{
				Hostname: host.Hostname,
			}

			// SMART data
			for _, disk := range host.Sensors.SMART {
				hostHealth.SMART = append(hostHealth.SMART, SMARTDiskSummary{
					Device:      disk.Device,
					Model:       disk.Model,
					Health:      disk.Health,
					Temperature: disk.Temperature,
				})
			}

			// RAID arrays
			for _, raid := range host.RAID {
				hostHealth.RAID = append(hostHealth.RAID, RAIDArraySummary{
					Device:         raid.Device,
					Level:          raid.Level,
					State:          raid.State,
					ActiveDevices:  raid.ActiveDevices,
					WorkingDevices: raid.WorkingDevices,
					FailedDevices:  raid.FailedDevices,
					SpareDevices:   raid.SpareDevices,
					RebuildPercent: raid.RebuildPercent,
				})
			}

			// Ceph from agent
			if host.Ceph != nil {
				hostHealth.Ceph = &CephStatusSummary{
					Health:       host.Ceph.Health.Status,
					NumOSDs:      host.Ceph.OSDMap.NumOSDs,
					NumOSDsUp:    host.Ceph.OSDMap.NumUp,
					NumOSDsIn:    host.Ceph.OSDMap.NumIn,
					NumPGs:       host.Ceph.PGMap.NumPGs,
					UsagePercent: host.Ceph.PGMap.UsagePercent,
				}
			}

			// Only add if there's data
			if len(hostHealth.SMART) > 0 || len(hostHealth.RAID) > 0 || hostHealth.Ceph != nil {
				// Ensure non-nil slices
				if hostHealth.SMART == nil {
					hostHealth.SMART = []SMARTDiskSummary{}
				}
				if hostHealth.RAID == nil {
					hostHealth.RAID = []RAIDArraySummary{}
				}
				response.Hosts = append(response.Hosts, hostHealth)
			}
		}
	}

	return NewJSONResult(response), nil
}

// ========== Docker Updates Tool Implementations ==========

func (e *PulseToolExecutor) executeListDockerUpdates(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.updatesProvider == nil {
		return NewTextResult("Docker update information not available. Ensure updates provider is configured."), nil
	}

	hostFilter, _ := args["host"].(string)

	// Resolve host name to ID if needed
	hostID := e.resolveDockerHostID(hostFilter)

	updates := e.updatesProvider.GetPendingUpdates(hostID)

	// Ensure non-nil slice
	if updates == nil {
		updates = []ContainerUpdateInfo{}
	}

	response := DockerUpdatesResponse{
		Updates: updates,
		Total:   len(updates),
		HostID:  hostID,
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeCheckDockerUpdates(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.updatesProvider == nil {
		return NewTextResult("Docker update checking not available. Ensure updates provider is configured."), nil
	}

	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	// Resolve host name to ID
	hostID := e.resolveDockerHostID(hostArg)
	if hostID == "" {
		return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
	}

	hostName := e.getDockerHostName(hostID)

	// Control level check - suggest mode just returns the suggestion
	if e.controlLevel == ControlLevelSuggest {
		return NewTextResult(fmt.Sprintf("To check for Docker updates on host '%s', use the UI or API:\n\nPOST /api/agents/docker/hosts/%s/check-updates", hostName, hostID)), nil
	}

	// Trigger the update check
	cmdStatus, err := e.updatesProvider.TriggerUpdateCheck(hostID)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Failed to trigger update check: %v", err)), nil
	}

	response := DockerCheckUpdatesResponse{
		Success:   true,
		HostID:    hostID,
		HostName:  hostName,
		CommandID: cmdStatus.ID,
		Message:   "Update check command queued. Results will be available after the next agent report cycle (~30 seconds).",
		Command:   cmdStatus,
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeUpdateDockerContainer(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.updatesProvider == nil {
		return NewTextResult("Docker update functionality not available. Ensure updates provider is configured."), nil
	}

	containerArg, _ := args["container"].(string)
	hostArg, _ := args["host"].(string)

	if containerArg == "" {
		return NewErrorResult(fmt.Errorf("container is required")), nil
	}
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	// Check if update actions are enabled
	if !e.updatesProvider.IsUpdateActionsEnabled() {
		return NewTextResult("Docker container updates are disabled by server configuration. Set PULSE_DISABLE_DOCKER_UPDATE_ACTIONS=false or enable in Settings to allow updates."), nil
	}

	// Resolve container and host
	container, dockerHost, err := e.resolveDockerContainer(containerArg, hostArg)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Could not find container '%s' on host '%s': %v", containerArg, hostArg, err)), nil
	}

	containerName := trimContainerName(container.Name)

	// Control level handling
	if e.controlLevel == ControlLevelSuggest {
		return NewTextResult(fmt.Sprintf(`To update container '%s' on host '%s', use the UI or run:

POST /api/agents/docker/containers/update
{
  "hostId": "%s",
  "containerId": "%s",
  "containerName": "%s"
}`, containerName, dockerHost.Hostname, dockerHost.ID, container.ID, containerName)), nil
	}

	// Controlled mode - require approval
	if e.controlLevel == ControlLevelControlled {
		command := fmt.Sprintf("docker update %s", containerName)
		agentHostname := e.getAgentHostnameForDockerHost(dockerHost)
		approvalID := createApprovalRecord(command, "docker", container.ID, agentHostname, fmt.Sprintf("Update container %s to latest image", containerName))
		return NewTextResult(formatDockerUpdateApprovalNeeded(containerName, dockerHost.Hostname, approvalID)), nil
	}

	// Autonomous mode - execute directly
	cmdStatus, err := e.updatesProvider.UpdateContainer(dockerHost.ID, container.ID, containerName)
	if err != nil {
		return NewTextResult(fmt.Sprintf("Failed to queue update command: %v", err)), nil
	}

	response := DockerUpdateContainerResponse{
		Success:       true,
		HostID:        dockerHost.ID,
		ContainerID:   container.ID,
		ContainerName: containerName,
		CommandID:     cmdStatus.ID,
		Message:       fmt.Sprintf("Update command queued for container '%s'. The agent will pull the latest image and recreate the container.", containerName),
		Command:       cmdStatus,
	}

	return NewJSONResult(response), nil
}

// Helper methods for Docker updates

func (e *PulseToolExecutor) resolveDockerHostID(hostArg string) string {
	if hostArg == "" {
		return ""
	}
	if e.stateProvider == nil {
		return hostArg
	}

	state := e.stateProvider.GetState()
	for _, host := range state.DockerHosts {
		if host.ID == hostArg || host.Hostname == hostArg || host.DisplayName == hostArg {
			return host.ID
		}
	}
	return hostArg // Return as-is if not found (provider will handle error)
}

func (e *PulseToolExecutor) getDockerHostName(hostID string) string {
	if e.stateProvider == nil {
		return hostID
	}

	state := e.stateProvider.GetState()
	for _, host := range state.DockerHosts {
		if host.ID == hostID {
			if host.DisplayName != "" {
				return host.DisplayName
			}
			return host.Hostname
		}
	}
	return hostID
}

func formatDockerUpdateApprovalNeeded(containerName, hostName, approvalID string) string {
	payload := map[string]interface{}{
		"type":           "approval_required",
		"approval_id":    approvalID,
		"container_name": containerName,
		"docker_host":    hostName,
		"action":         "update",
		"command":        fmt.Sprintf("docker update %s (pull latest + recreate)", containerName),
		"how_to_approve": "Click the approval button in the chat to execute this update.",
		"do_not_retry":   true,
	}
	b, _ := json.Marshal(payload)
	return "APPROVAL_REQUIRED: " + string(b)
}

func trimLeadingSlash(name string) string {
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}
