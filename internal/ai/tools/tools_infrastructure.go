package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	// Temperature/Sensor tools
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_temperatures",
			Description: `Get temperature and sensor data from hosts running Pulse unified agents.

Returns: CPU temperatures, NVMe/disk temps, fan speeds, and other sensor readings.

Use when: User asks about temperatures, thermal status, cooling, or hardware health.

Note: Only hosts with Pulse unified agent installed will report sensor data.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Optional: filter by specific hostname",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetTemperatures(ctx, args)
		},
	})

	// Ceph status tool
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_ceph_status",
			Description: `Get Ceph cluster status and health information.

Returns: Cluster health, OSD status, pool information, and any warnings.

Use when: User asks about Ceph storage, cluster health, or distributed storage status.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"cluster": {
						Type:        "string",
						Description: "Optional: specific Ceph cluster name",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetCephStatus(ctx, args)
		},
	})

	// Replication jobs tool
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_replication",
			Description: `Get Proxmox replication job status.

Returns: Replication jobs, their status, last sync times, and any errors.

Use when: User asks about replication, data sync, or disaster recovery status.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"vm_id": {
						Type:        "string",
						Description: "Optional: filter by specific VM ID",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetReplication(ctx, args)
		},
	})

	// ========== Snapshots & Backup Tools ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_snapshots",
			Description: `List VM and container snapshots.

Returns: JSON with snapshots array containing vmid, name, type, node, snapshot_name, time, description, vm_state, size.

Use when: User asks about snapshots, wants to check if a VM has snapshots, or needs snapshot inventory.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"guest_id": {
						Type:        "string",
						Description: "Optional: filter by specific VM or container ID",
					},
					"instance": {
						Type:        "string",
						Description: "Optional: filter by Proxmox instance",
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
			return exec.executeListSnapshots(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_pbs_jobs",
			Description: `List PBS backup, sync, verify, prune, and garbage collection jobs.

Returns: JSON with jobs array containing id, type, store, status, last_run, next_run, error.

Use when: User asks about PBS jobs, backup jobs status, sync jobs, verify jobs, or garbage collection.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: filter by PBS instance name or ID",
					},
					"job_type": {
						Type:        "string",
						Description: "Optional: filter by job type (backup, sync, verify, prune, garbage)",
						Enum:        []string{"backup", "sync", "verify", "prune", "garbage"},
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListPBSJobs(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_backup_tasks",
			Description: `List recent Proxmox backup tasks with status.

Returns: JSON with tasks array containing vmid, node, type, status, start_time, end_time, size, error.

Use when: User asks about recent backup tasks, backup history, or backup failures.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: filter by Proxmox instance",
					},
					"guest_id": {
						Type:        "string",
						Description: "Optional: filter by VM or container ID",
					},
					"status": {
						Type:        "string",
						Description: "Optional: filter by status (ok, error)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 50)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListBackupTasks(ctx, args)
		},
	})

	// ========== Host Diagnostics Tools ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_network_stats",
			Description: `Get network interface statistics for hosts.

Returns: JSON with hosts array, each containing interfaces with name, mac, rx_bytes, tx_bytes, speed, addresses.

Use when: User asks about network throughput, bandwidth usage, or network interface status.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Optional: filter by hostname",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetNetworkStats(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_diskio_stats",
			Description: `Get disk I/O statistics for hosts.

Returns: JSON with hosts array, each containing devices with read/write bytes, ops, and io time.

Use when: User asks about disk I/O, disk throughput, or storage performance.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Optional: filter by hostname",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetDiskIOStats(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_cluster_status",
			Description: `Get Proxmox cluster membership and quorum status.

Returns: JSON with cluster info including quorum status, total/online nodes, and per-node membership details.

Use when: User asks about cluster health, quorum, cluster membership, or node status in a cluster.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: filter by Proxmox instance",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetClusterStatus(ctx, args)
		},
	})

	// ========== Docker Swarm Tools ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_swarm_status",
			Description: `Get Docker Swarm cluster status for a host.

Returns: JSON with swarm status including node_id, node_role, local_state, control_available, cluster_id, cluster_name.

Use when: User asks about Docker Swarm status, swarm cluster health, or swarm membership.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Docker host name or ID (required)",
					},
				},
				Required: []string{"host"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetSwarmStatus(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_docker_services",
			Description: `List Docker Swarm services.

Returns: JSON with services array containing id, name, stack, image, mode, desired_tasks, running_tasks, update_status.

Use when: User asks about Docker services, swarm services, or service health.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Docker host name or ID (required)",
					},
					"stack": {
						Type:        "string",
						Description: "Optional: filter by stack name",
					},
				},
				Required: []string{"host"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListDockerServices(ctx, args)
		},
	})

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_docker_tasks",
			Description: `List Docker Swarm tasks for a service.

Returns: JSON with tasks array containing id, service_name, node_name, desired_state, current_state, error, started_at.

Use when: User asks about Docker tasks, service tasks, or task failures.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Docker host name or ID (required)",
					},
					"service": {
						Type:        "string",
						Description: "Optional: filter by service name or ID",
					},
				},
				Required: []string{"host"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListDockerTasks(ctx, args)
		},
	})

	// ========== Recent Tasks Tool ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_recent_tasks",
			Description: `List recent Proxmox tasks (backup, migration, etc).

Returns: JSON with tasks array containing id, node, type, status, start_time, end_time, vmid.

Use when: User asks about recent tasks, task history, or task failures. Note: Currently shows backup tasks as primary task source.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: filter by Proxmox instance",
					},
					"node": {
						Type:        "string",
						Description: "Optional: filter by node name",
					},
					"type": {
						Type:        "string",
						Description: "Optional: filter by task type (e.g., 'backup')",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 50)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListRecentTasks(ctx, args)
		},
	})

	// ========== Physical Disks Tool ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_list_physical_disks",
			Description: `List physical disks with SMART health data, SSD wearout, and temperatures.

Returns: JSON with disks array containing device path, model, serial, type (nvme/sata/sas), size, health status, wearout percentage (for SSDs), temperature, and RPM (for HDDs).

Use when: User asks about physical disk health, SSD wear levels, disk temperatures, or SMART status across Proxmox nodes.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"instance": {
						Type:        "string",
						Description: "Optional: filter by Proxmox instance",
					},
					"node": {
						Type:        "string",
						Description: "Optional: filter by node name",
					},
					"health": {
						Type:        "string",
						Description: "Optional: filter by health status (PASSED, FAILED, UNKNOWN)",
					},
					"type": {
						Type:        "string",
						Description: "Optional: filter by disk type (nvme, sata, sas)",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of results (default: 100)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeListPhysicalDisks(ctx, args)
		},
	})

	// ========== Host RAID Status Tool ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_host_raid_status",
			Description: `Get RAID array status from host agents, including degraded arrays, rebuild progress, and failed devices.

Returns: JSON with hosts array, each containing hostname and RAID arrays with device, level, state, device counts, rebuild percentage, and individual disk status.

Use when: User asks about RAID health, degraded arrays, RAID rebuilds, or disk failures in RAID arrays.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Optional: filter by host name or ID",
					},
					"state": {
						Type:        "string",
						Description: "Optional: filter by array state (clean, degraded, rebuilding)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetHostRAIDStatus(ctx, args)
		},
	})

	// ========== Host Ceph Details Tool ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_host_ceph_details",
			Description: `Get detailed Ceph cluster status from host agents, including health checks, OSD status, PG stats, monitor and manager status, and pool usage.

Returns: JSON with hosts array containing Ceph cluster details including FSID, health status and messages, monitor/manager maps, OSD up/down/in/out counts, PG statistics, and pool usage.

Use when: User asks about Ceph cluster health collected by host agents, OSD failures, Ceph performance metrics, or pool capacity. Note: This is from host agent collection, separate from Proxmox API Ceph data.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"host": {
						Type:        "string",
						Description: "Optional: filter by host name or ID",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetHostCephDetails(ctx, args)
		},
	})

	// ========== Resource Disks Tool ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_resource_disks",
			Description: `Get disk/filesystem information for VMs and containers, including mount points, usage, and capacity.

Returns: JSON with resources array containing VM/container ID, name, type, and disks array with device, mountpoint, total/used/free bytes, and usage percentage.

Use when: User asks about VM disk usage, container storage, filesystem capacity, or which guests are running low on disk space.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"resource_id": {
						Type:        "string",
						Description: "Optional: filter by specific VM or container ID",
					},
					"type": {
						Type:        "string",
						Description: "Optional: filter by type ('vm' or 'lxc')",
					},
					"instance": {
						Type:        "string",
						Description: "Optional: filter by Proxmox instance",
					},
					"min_usage": {
						Type:        "number",
						Description: "Optional: only show resources with disk usage above this percentage (0-100)",
					},
				},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetResourceDisks(ctx, args)
		},
	})

	// ========== Connection Health Tool ==========

	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_get_connection_health",
			Description: `Get connection health status for all monitored instances (Proxmox, PBS, PMG).

Returns: JSON with connections array showing instance IDs and their connected/disconnected status, plus summary counts.

Use when: User asks about connection issues, which instances are offline, connectivity problems, or wants to diagnose why data isn't updating.`,
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]PropertySchema{},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeGetConnectionHealth(ctx, args)
		},
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

// executeGetTemperatures returns temperature and sensor data from hosts
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

// executeGetCephStatus returns Ceph cluster status
func (e *PulseToolExecutor) executeGetCephStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterFilter, _ := args["cluster"].(string)

	state := e.stateProvider.GetState()

	if len(state.CephClusters) == 0 {
		return NewTextResult("No Ceph clusters found. Ceph may not be configured or data is not yet available."), nil
	}

	type CephSummary struct {
		Name    string                 `json:"name"`
		Health  string                 `json:"health"`
		Details map[string]interface{} `json:"details,omitempty"`
	}

	var results []CephSummary

	for _, cluster := range state.CephClusters {
		if clusterFilter != "" && cluster.Name != clusterFilter {
			continue
		}

		summary := CephSummary{
			Name:    cluster.Name,
			Health:  cluster.Health,
			Details: make(map[string]interface{}),
		}

		// Add relevant details
		if cluster.HealthMessage != "" {
			summary.Details["health_message"] = cluster.HealthMessage
		}
		if cluster.NumOSDs > 0 {
			summary.Details["osd_count"] = cluster.NumOSDs
			summary.Details["osds_up"] = cluster.NumOSDsUp
			summary.Details["osds_in"] = cluster.NumOSDsIn
			summary.Details["osds_down"] = cluster.NumOSDs - cluster.NumOSDsUp
		}
		if cluster.NumMons > 0 {
			summary.Details["monitors"] = cluster.NumMons
		}
		if cluster.TotalBytes > 0 {
			summary.Details["total_bytes"] = cluster.TotalBytes
			summary.Details["used_bytes"] = cluster.UsedBytes
			summary.Details["available_bytes"] = cluster.AvailableBytes
			summary.Details["usage_percent"] = cluster.UsagePercent
		}
		if len(cluster.Pools) > 0 {
			summary.Details["pools"] = cluster.Pools
		}

		results = append(results, summary)
	}

	if len(results) == 0 && clusterFilter != "" {
		return NewTextResult(fmt.Sprintf("Ceph cluster '%s' not found.", clusterFilter)), nil
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return NewTextResult(string(output)), nil
}

// executeGetReplication returns replication job status
func (e *PulseToolExecutor) executeGetReplication(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	vmFilter, _ := args["vm_id"].(string)

	state := e.stateProvider.GetState()

	if len(state.ReplicationJobs) == 0 {
		return NewTextResult("No replication jobs found. Replication may not be configured."), nil
	}

	type ReplicationSummary struct {
		ID           string `json:"id"`
		GuestID      int    `json:"guest_id"`
		GuestName    string `json:"guest_name,omitempty"`
		GuestType    string `json:"guest_type,omitempty"`
		SourceNode   string `json:"source_node,omitempty"`
		TargetNode   string `json:"target_node"`
		Schedule     string `json:"schedule,omitempty"`
		Status       string `json:"status"`
		LastSync     string `json:"last_sync,omitempty"`
		NextSync     string `json:"next_sync,omitempty"`
		LastDuration string `json:"last_duration,omitempty"`
		Error        string `json:"error,omitempty"`
	}

	var results []ReplicationSummary

	for _, job := range state.ReplicationJobs {
		if vmFilter != "" && fmt.Sprintf("%d", job.GuestID) != vmFilter {
			continue
		}

		summary := ReplicationSummary{
			ID:         job.ID,
			GuestID:    job.GuestID,
			GuestName:  job.GuestName,
			GuestType:  job.GuestType,
			SourceNode: job.SourceNode,
			TargetNode: job.TargetNode,
			Schedule:   job.Schedule,
			Status:     job.Status,
		}

		if job.LastSyncTime != nil {
			summary.LastSync = job.LastSyncTime.Format("2006-01-02 15:04:05")
		}
		if job.NextSyncTime != nil {
			summary.NextSync = job.NextSyncTime.Format("2006-01-02 15:04:05")
		}
		if job.LastSyncDurationHuman != "" {
			summary.LastDuration = job.LastSyncDurationHuman
		}
		if job.Error != "" {
			summary.Error = job.Error
		}

		results = append(results, summary)
	}

	if len(results) == 0 && vmFilter != "" {
		return NewTextResult(fmt.Sprintf("No replication jobs found for VM %s.", vmFilter)), nil
	}

	output, _ := json.MarshalIndent(results, "", "  ")
	return NewTextResult(string(output)), nil
}

// containsAny checks if s contains any of the substrings (case-insensitive)
func containsAny(s string, substrs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range substrs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// ========== Snapshots & Backup Tool Implementations ==========

func (e *PulseToolExecutor) executeListSnapshots(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	guestIDFilter, _ := args["guest_id"].(string)
	instanceFilter, _ := args["instance"].(string)
	limit := intArg(args, "limit", 100)
	offset := intArg(args, "offset", 0)

	state := e.stateProvider.GetState()

	// Build VM name map for enrichment
	vmNames := make(map[int]string)
	for _, vm := range state.VMs {
		vmNames[vm.VMID] = vm.Name
	}
	for _, ct := range state.Containers {
		vmNames[ct.VMID] = ct.Name
	}

	var snapshots []SnapshotSummary
	filteredCount := 0
	count := 0

	for _, snap := range state.PVEBackups.GuestSnapshots {
		// Apply filters
		if guestIDFilter != "" && fmt.Sprintf("%d", snap.VMID) != guestIDFilter {
			continue
		}
		if instanceFilter != "" && snap.Instance != instanceFilter {
			continue
		}

		filteredCount++

		// Apply pagination
		if count < offset {
			count++
			continue
		}
		if len(snapshots) >= limit {
			count++
			continue
		}

		snapshots = append(snapshots, SnapshotSummary{
			ID:           snap.ID,
			VMID:         snap.VMID,
			VMName:       vmNames[snap.VMID],
			Type:         snap.Type,
			Node:         snap.Node,
			Instance:     snap.Instance,
			SnapshotName: snap.Name,
			Description:  snap.Description,
			Time:         snap.Time,
			VMState:      snap.VMState,
			SizeBytes:    snap.SizeBytes,
		})
		count++
	}

	if snapshots == nil {
		snapshots = []SnapshotSummary{}
	}

	response := SnapshotsResponse{
		Snapshots: snapshots,
		Total:     len(state.PVEBackups.GuestSnapshots),
		Filtered:  filteredCount,
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeListPBSJobs(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.backupProvider == nil {
		return NewTextResult("Backup provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)
	jobTypeFilter, _ := args["job_type"].(string)

	pbsInstances := e.backupProvider.GetPBSInstances()

	if len(pbsInstances) == 0 {
		return NewTextResult("No PBS instances found. PBS monitoring may not be configured."), nil
	}

	var jobs []PBSJobSummary

	for _, pbs := range pbsInstances {
		if instanceFilter != "" && pbs.ID != instanceFilter && pbs.Name != instanceFilter {
			continue
		}

		// Backup jobs
		if jobTypeFilter == "" || jobTypeFilter == "backup" {
			for _, job := range pbs.BackupJobs {
				jobs = append(jobs, PBSJobSummary{
					ID:      job.ID,
					Type:    "backup",
					Store:   job.Store,
					Status:  job.Status,
					LastRun: job.LastBackup,
					NextRun: job.NextRun,
					Error:   job.Error,
					VMID:    job.VMID,
				})
			}
		}

		// Sync jobs
		if jobTypeFilter == "" || jobTypeFilter == "sync" {
			for _, job := range pbs.SyncJobs {
				jobs = append(jobs, PBSJobSummary{
					ID:      job.ID,
					Type:    "sync",
					Store:   job.Store,
					Status:  job.Status,
					LastRun: job.LastSync,
					NextRun: job.NextRun,
					Error:   job.Error,
					Remote:  job.Remote,
				})
			}
		}

		// Verify jobs
		if jobTypeFilter == "" || jobTypeFilter == "verify" {
			for _, job := range pbs.VerifyJobs {
				jobs = append(jobs, PBSJobSummary{
					ID:      job.ID,
					Type:    "verify",
					Store:   job.Store,
					Status:  job.Status,
					LastRun: job.LastVerify,
					NextRun: job.NextRun,
					Error:   job.Error,
				})
			}
		}

		// Prune jobs
		if jobTypeFilter == "" || jobTypeFilter == "prune" {
			for _, job := range pbs.PruneJobs {
				jobs = append(jobs, PBSJobSummary{
					ID:      job.ID,
					Type:    "prune",
					Store:   job.Store,
					Status:  job.Status,
					LastRun: job.LastPrune,
					NextRun: job.NextRun,
					Error:   job.Error,
				})
			}
		}

		// Garbage jobs
		if jobTypeFilter == "" || jobTypeFilter == "garbage" {
			for _, job := range pbs.GarbageJobs {
				jobs = append(jobs, PBSJobSummary{
					ID:           job.ID,
					Type:         "garbage",
					Store:        job.Store,
					Status:       job.Status,
					LastRun:      job.LastGarbage,
					NextRun:      job.NextRun,
					Error:        job.Error,
					RemovedBytes: job.RemovedBytes,
				})
			}
		}
	}

	if jobs == nil {
		jobs = []PBSJobSummary{}
	}

	response := PBSJobsResponse{
		Instance: instanceFilter,
		Jobs:     jobs,
		Total:    len(jobs),
	}

	return NewJSONResult(response), nil
}

func (e *PulseToolExecutor) executeListBackupTasks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)
	guestIDFilter, _ := args["guest_id"].(string)
	statusFilter, _ := args["status"].(string)
	limit := intArg(args, "limit", 50)

	state := e.stateProvider.GetState()

	// Build VM name map
	vmNames := make(map[int]string)
	for _, vm := range state.VMs {
		vmNames[vm.VMID] = vm.Name
	}
	for _, ct := range state.Containers {
		vmNames[ct.VMID] = ct.Name
	}

	var tasks []BackupTaskDetail
	filteredCount := 0

	for _, task := range state.PVEBackups.BackupTasks {
		// Apply filters
		if instanceFilter != "" && task.Instance != instanceFilter {
			continue
		}
		if guestIDFilter != "" && fmt.Sprintf("%d", task.VMID) != guestIDFilter {
			continue
		}
		if statusFilter != "" && !strings.EqualFold(task.Status, statusFilter) {
			continue
		}

		filteredCount++

		if len(tasks) >= limit {
			continue
		}

		tasks = append(tasks, BackupTaskDetail{
			ID:        task.ID,
			VMID:      task.VMID,
			VMName:    vmNames[task.VMID],
			Node:      task.Node,
			Instance:  task.Instance,
			Type:      task.Type,
			Status:    task.Status,
			StartTime: task.StartTime,
			EndTime:   task.EndTime,
			SizeBytes: task.Size,
			Error:     task.Error,
		})
	}

	if tasks == nil {
		tasks = []BackupTaskDetail{}
	}

	response := BackupTasksListResponse{
		Tasks:    tasks,
		Total:    len(state.PVEBackups.BackupTasks),
		Filtered: filteredCount,
	}

	return NewJSONResult(response), nil
}

// ========== Host Diagnostics Tool Implementations ==========

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

func (e *PulseToolExecutor) executeGetClusterStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)

	state := e.stateProvider.GetState()

	if len(state.Nodes) == 0 {
		return NewTextResult("No Proxmox nodes found."), nil
	}

	// Group nodes by cluster
	clusterMap := make(map[string]*PVEClusterStatus)
	standaloneNodes := []PVEClusterNodeStatus{}

	for _, node := range state.Nodes {
		if instanceFilter != "" && node.Instance != instanceFilter {
			continue
		}

		nodeStatus := PVEClusterNodeStatus{
			Name:            node.Name,
			Status:          node.Status,
			IsClusterMember: node.IsClusterMember,
			ClusterName:     node.ClusterName,
		}

		if node.IsClusterMember && node.ClusterName != "" {
			if _, exists := clusterMap[node.ClusterName]; !exists {
				clusterMap[node.ClusterName] = &PVEClusterStatus{
					Instance:    node.Instance,
					ClusterName: node.ClusterName,
					Nodes:       []PVEClusterNodeStatus{},
				}
			}
			clusterMap[node.ClusterName].Nodes = append(clusterMap[node.ClusterName].Nodes, nodeStatus)
			clusterMap[node.ClusterName].TotalNodes++
			if node.Status == "online" {
				clusterMap[node.ClusterName].OnlineNodes++
			}
		} else {
			standaloneNodes = append(standaloneNodes, nodeStatus)
		}
	}

	var clusters []PVEClusterStatus

	// Process clusters
	for _, cluster := range clusterMap {
		// Quorum is OK if more than half the nodes are online
		cluster.QuorumOK = cluster.OnlineNodes > cluster.TotalNodes/2
		clusters = append(clusters, *cluster)
	}

	// Add standalone nodes as individual "clusters"
	for _, node := range standaloneNodes {
		clusters = append(clusters, PVEClusterStatus{
			Instance:    instanceFilter,
			ClusterName: "",
			QuorumOK:    node.Status == "online",
			TotalNodes:  1,
			OnlineNodes: func() int {
				if node.Status == "online" {
					return 1
				}
				return 0
			}(),
			Nodes: []PVEClusterNodeStatus{node},
		})
	}

	if len(clusters) == 0 {
		return NewTextResult("No cluster information available."), nil
	}

	response := ClusterStatusResponse{
		Clusters: clusters,
	}

	return NewJSONResult(response), nil
}

// ========== Docker Swarm Tool Implementations ==========

func (e *PulseToolExecutor) executeGetSwarmStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	state := e.stateProvider.GetState()

	for _, host := range state.DockerHosts {
		if host.ID == hostArg || host.Hostname == hostArg || host.DisplayName == hostArg || host.CustomDisplayName == hostArg {
			if host.Swarm == nil {
				return NewTextResult(fmt.Sprintf("Docker host '%s' is not part of a Swarm cluster.", host.Hostname)), nil
			}

			response := SwarmStatusResponse{
				Host: host.Hostname,
				Status: DockerSwarmSummary{
					NodeID:           host.Swarm.NodeID,
					NodeRole:         host.Swarm.NodeRole,
					LocalState:       host.Swarm.LocalState,
					ControlAvailable: host.Swarm.ControlAvailable,
					ClusterID:        host.Swarm.ClusterID,
					ClusterName:      host.Swarm.ClusterName,
					Error:            host.Swarm.Error,
				},
			}

			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}

func (e *PulseToolExecutor) executeListDockerServices(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	stackFilter, _ := args["stack"].(string)

	state := e.stateProvider.GetState()

	for _, host := range state.DockerHosts {
		if host.ID == hostArg || host.Hostname == hostArg || host.DisplayName == hostArg || host.CustomDisplayName == hostArg {
			if len(host.Services) == 0 {
				return NewTextResult(fmt.Sprintf("No Docker services found on host '%s'. The host may not be a Swarm manager.", host.Hostname)), nil
			}

			var services []DockerServiceSummary
			filteredCount := 0

			for _, svc := range host.Services {
				if stackFilter != "" && svc.Stack != stackFilter {
					continue
				}

				filteredCount++

				updateStatus := ""
				if svc.UpdateStatus != nil {
					updateStatus = svc.UpdateStatus.State
				}

				services = append(services, DockerServiceSummary{
					ID:           svc.ID,
					Name:         svc.Name,
					Stack:        svc.Stack,
					Image:        svc.Image,
					Mode:         svc.Mode,
					DesiredTasks: svc.DesiredTasks,
					RunningTasks: svc.RunningTasks,
					UpdateStatus: updateStatus,
				})
			}

			if services == nil {
				services = []DockerServiceSummary{}
			}

			response := DockerServicesResponse{
				Host:     host.Hostname,
				Services: services,
				Total:    len(host.Services),
				Filtered: filteredCount,
			}

			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}

func (e *PulseToolExecutor) executeListDockerTasks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	hostArg, _ := args["host"].(string)
	if hostArg == "" {
		return NewErrorResult(fmt.Errorf("host is required")), nil
	}

	serviceFilter, _ := args["service"].(string)

	state := e.stateProvider.GetState()

	for _, host := range state.DockerHosts {
		if host.ID == hostArg || host.Hostname == hostArg || host.DisplayName == hostArg || host.CustomDisplayName == hostArg {
			if len(host.Tasks) == 0 {
				return NewTextResult(fmt.Sprintf("No Docker tasks found on host '%s'. The host may not be a Swarm manager.", host.Hostname)), nil
			}

			var tasks []DockerTaskSummary

			for _, task := range host.Tasks {
				if serviceFilter != "" && task.ServiceID != serviceFilter && task.ServiceName != serviceFilter {
					continue
				}

				tasks = append(tasks, DockerTaskSummary{
					ID:           task.ID,
					ServiceName:  task.ServiceName,
					NodeName:     task.NodeName,
					DesiredState: task.DesiredState,
					CurrentState: task.CurrentState,
					Error:        task.Error,
					StartedAt:    task.StartedAt,
				})
			}

			if tasks == nil {
				tasks = []DockerTaskSummary{}
			}

			response := DockerTasksResponse{
				Host:    host.Hostname,
				Service: serviceFilter,
				Tasks:   tasks,
				Total:   len(tasks),
			}

			return NewJSONResult(response), nil
		}
	}

	return NewTextResult(fmt.Sprintf("Docker host '%s' not found.", hostArg)), nil
}

// ========== Recent Tasks Tool Implementation ==========

func (e *PulseToolExecutor) executeListRecentTasks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)
	nodeFilter, _ := args["node"].(string)
	typeFilter, _ := args["type"].(string)
	limit := intArg(args, "limit", 50)

	state := e.stateProvider.GetState()

	var tasks []ProxmoxTaskSummary
	filteredCount := 0

	// Currently using backup tasks as the primary task source
	for _, task := range state.PVEBackups.BackupTasks {
		// Apply filters
		if instanceFilter != "" && task.Instance != instanceFilter {
			continue
		}
		if nodeFilter != "" && task.Node != nodeFilter {
			continue
		}
		// Match if type filter matches task.Type or "backup" (case-insensitive)
		if typeFilter != "" && !strings.EqualFold(task.Type, typeFilter) && !strings.EqualFold("backup", typeFilter) {
			continue
		}

		filteredCount++

		if len(tasks) >= limit {
			continue
		}

		tasks = append(tasks, ProxmoxTaskSummary{
			ID:        task.ID,
			Node:      task.Node,
			Instance:  task.Instance,
			Type:      "backup",
			Status:    task.Status,
			StartTime: task.StartTime,
			EndTime:   task.EndTime,
			VMID:      task.VMID,
		})
	}

	if tasks == nil {
		tasks = []ProxmoxTaskSummary{}
	}

	response := RecentTasksResponse{
		Tasks:    tasks,
		Total:    len(state.PVEBackups.BackupTasks),
		Filtered: filteredCount,
	}

	return NewJSONResult(response), nil
}

// ========== Physical Disks Tool Implementation ==========

func (e *PulseToolExecutor) executeListPhysicalDisks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	instanceFilter, _ := args["instance"].(string)
	nodeFilter, _ := args["node"].(string)
	healthFilter, _ := args["health"].(string)
	typeFilter, _ := args["type"].(string)
	limit := intArg(args, "limit", 100)

	state := e.stateProvider.GetState()

	if len(state.PhysicalDisks) == 0 {
		return NewTextResult("No physical disk data available. Physical disk information is collected from Proxmox nodes."), nil
	}

	var disks []PhysicalDiskSummary
	totalCount := 0

	for _, disk := range state.PhysicalDisks {
		// Apply filters
		if instanceFilter != "" && disk.Instance != instanceFilter {
			continue
		}
		if nodeFilter != "" && disk.Node != nodeFilter {
			continue
		}
		if healthFilter != "" && !strings.EqualFold(disk.Health, healthFilter) {
			continue
		}
		if typeFilter != "" && !strings.EqualFold(disk.Type, typeFilter) {
			continue
		}

		totalCount++

		if len(disks) >= limit {
			continue
		}

		summary := PhysicalDiskSummary{
			ID:          disk.ID,
			Node:        disk.Node,
			Instance:    disk.Instance,
			DevPath:     disk.DevPath,
			Model:       disk.Model,
			Serial:      disk.Serial,
			WWN:         disk.WWN,
			Type:        disk.Type,
			SizeBytes:   disk.Size,
			Health:      disk.Health,
			Used:        disk.Used,
			LastChecked: disk.LastChecked,
		}

		// Only include optional fields if they have meaningful values
		// Using pointers so that 0 values (valid for wearout and RPM) serialize correctly
		if disk.Wearout >= 0 {
			wearout := disk.Wearout
			summary.Wearout = &wearout
		}
		if disk.Temperature > 0 {
			temp := disk.Temperature
			summary.Temperature = &temp
		}
		if disk.RPM > 0 {
			rpm := disk.RPM
			summary.RPM = &rpm
		}

		disks = append(disks, summary)
	}

	if disks == nil {
		disks = []PhysicalDiskSummary{}
	}

	response := PhysicalDisksResponse{
		Disks:    disks,
		Total:    len(state.PhysicalDisks),
		Filtered: totalCount,
	}

	return NewJSONResult(response), nil
}

// ========== Host RAID Status Tool Implementation ==========

func (e *PulseToolExecutor) executeGetHostRAIDStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.diskHealthProvider == nil {
		return NewTextResult("Disk health provider not available."), nil
	}

	hostFilter, _ := args["host"].(string)
	stateFilter, _ := args["state"].(string)

	hosts := e.diskHealthProvider.GetHosts()

	var hostSummaries []HostRAIDSummary

	for _, host := range hosts {
		// Apply host filter
		if hostFilter != "" && host.ID != hostFilter && host.Hostname != hostFilter && host.DisplayName != hostFilter {
			continue
		}

		// Skip hosts without RAID arrays
		if len(host.RAID) == 0 {
			continue
		}

		var arrays []HostRAIDArraySummary

		for _, raid := range host.RAID {
			// Apply state filter
			if stateFilter != "" && !strings.EqualFold(raid.State, stateFilter) {
				continue
			}

			var devices []HostRAIDDeviceSummary
			for _, dev := range raid.Devices {
				devices = append(devices, HostRAIDDeviceSummary{
					Device: dev.Device,
					State:  dev.State,
					Slot:   dev.Slot,
				})
			}

			if devices == nil {
				devices = []HostRAIDDeviceSummary{}
			}

			arrays = append(arrays, HostRAIDArraySummary{
				Device:         raid.Device,
				Name:           raid.Name,
				Level:          raid.Level,
				State:          raid.State,
				TotalDevices:   raid.TotalDevices,
				ActiveDevices:  raid.ActiveDevices,
				WorkingDevices: raid.WorkingDevices,
				FailedDevices:  raid.FailedDevices,
				SpareDevices:   raid.SpareDevices,
				UUID:           raid.UUID,
				RebuildPercent: raid.RebuildPercent,
				RebuildSpeed:   raid.RebuildSpeed,
				Devices:        devices,
			})
		}

		if len(arrays) > 0 {
			if arrays == nil {
				arrays = []HostRAIDArraySummary{}
			}
			hostSummaries = append(hostSummaries, HostRAIDSummary{
				Hostname: host.Hostname,
				HostID:   host.ID,
				Arrays:   arrays,
			})
		}
	}

	if hostSummaries == nil {
		hostSummaries = []HostRAIDSummary{}
	}

	if len(hostSummaries) == 0 {
		if hostFilter != "" {
			return NewTextResult(fmt.Sprintf("No RAID arrays found for host '%s'.", hostFilter)), nil
		}
		return NewTextResult("No RAID arrays found across any hosts. RAID monitoring requires host agents to be configured."), nil
	}

	response := HostRAIDStatusResponse{
		Hosts: hostSummaries,
		Total: len(hostSummaries),
	}

	return NewJSONResult(response), nil
}

// ========== Host Ceph Details Tool Implementation ==========

func (e *PulseToolExecutor) executeGetHostCephDetails(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.diskHealthProvider == nil {
		return NewTextResult("Disk health provider not available."), nil
	}

	hostFilter, _ := args["host"].(string)

	hosts := e.diskHealthProvider.GetHosts()

	var hostSummaries []HostCephSummary

	for _, host := range hosts {
		// Apply host filter
		if hostFilter != "" && host.ID != hostFilter && host.Hostname != hostFilter && host.DisplayName != hostFilter {
			continue
		}

		// Skip hosts without Ceph data
		if host.Ceph == nil {
			continue
		}

		ceph := host.Ceph

		// Build health messages from checks and summary
		var healthMessages []HostCephHealthMessage
		for checkName, check := range ceph.Health.Checks {
			msg := check.Message
			if msg == "" {
				msg = checkName
			}
			healthMessages = append(healthMessages, HostCephHealthMessage{
				Severity: check.Severity,
				Message:  msg,
			})
		}
		for _, summary := range ceph.Health.Summary {
			healthMessages = append(healthMessages, HostCephHealthMessage{
				Severity: summary.Severity,
				Message:  summary.Message,
			})
		}

		// Build monitor summary
		var monSummary *HostCephMonSummary
		if ceph.MonMap.NumMons > 0 {
			var monitors []HostCephMonitorSummary
			for _, mon := range ceph.MonMap.Monitors {
				monitors = append(monitors, HostCephMonitorSummary{
					Name:   mon.Name,
					Rank:   mon.Rank,
					Addr:   mon.Addr,
					Status: mon.Status,
				})
			}
			monSummary = &HostCephMonSummary{
				NumMons:  ceph.MonMap.NumMons,
				Monitors: monitors,
			}
		}

		// Build manager summary
		var mgrSummary *HostCephMgrSummary
		if ceph.MgrMap.NumMgrs > 0 || ceph.MgrMap.Available {
			mgrSummary = &HostCephMgrSummary{
				Available: ceph.MgrMap.Available,
				NumMgrs:   ceph.MgrMap.NumMgrs,
				ActiveMgr: ceph.MgrMap.ActiveMgr,
				Standbys:  ceph.MgrMap.Standbys,
			}
		}

		// Build pool summaries
		var pools []HostCephPoolSummary
		for _, pool := range ceph.Pools {
			pools = append(pools, HostCephPoolSummary{
				ID:             pool.ID,
				Name:           pool.Name,
				BytesUsed:      pool.BytesUsed,
				BytesAvailable: pool.BytesAvailable,
				Objects:        pool.Objects,
				PercentUsed:    pool.PercentUsed,
			})
		}

		if healthMessages == nil {
			healthMessages = []HostCephHealthMessage{}
		}
		if pools == nil {
			pools = []HostCephPoolSummary{}
		}

		hostSummaries = append(hostSummaries, HostCephSummary{
			Hostname: host.Hostname,
			HostID:   host.ID,
			FSID:     ceph.FSID,
			Health: HostCephHealthSummary{
				Status:   ceph.Health.Status,
				Messages: healthMessages,
			},
			MonMap: monSummary,
			MgrMap: mgrSummary,
			OSDMap: HostCephOSDSummary{
				NumOSDs: ceph.OSDMap.NumOSDs,
				NumUp:   ceph.OSDMap.NumUp,
				NumIn:   ceph.OSDMap.NumIn,
				NumDown: ceph.OSDMap.NumDown,
				NumOut:  ceph.OSDMap.NumOut,
			},
			PGMap: HostCephPGSummary{
				NumPGs:           ceph.PGMap.NumPGs,
				BytesTotal:       ceph.PGMap.BytesTotal,
				BytesUsed:        ceph.PGMap.BytesUsed,
				BytesAvailable:   ceph.PGMap.BytesAvailable,
				UsagePercent:     ceph.PGMap.UsagePercent,
				DegradedRatio:    ceph.PGMap.DegradedRatio,
				MisplacedRatio:   ceph.PGMap.MisplacedRatio,
				ReadBytesPerSec:  ceph.PGMap.ReadBytesPerSec,
				WriteBytesPerSec: ceph.PGMap.WriteBytesPerSec,
				ReadOpsPerSec:    ceph.PGMap.ReadOpsPerSec,
				WriteOpsPerSec:   ceph.PGMap.WriteOpsPerSec,
			},
			Pools:       pools,
			CollectedAt: ceph.CollectedAt,
		})
	}

	if hostSummaries == nil {
		hostSummaries = []HostCephSummary{}
	}

	if len(hostSummaries) == 0 {
		if hostFilter != "" {
			return NewTextResult(fmt.Sprintf("No Ceph data found for host '%s'.", hostFilter)), nil
		}
		return NewTextResult("No Ceph data found from host agents. Ceph monitoring requires host agents to be configured on Ceph nodes."), nil
	}

	response := HostCephDetailsResponse{
		Hosts: hostSummaries,
		Total: len(hostSummaries),
	}

	return NewJSONResult(response), nil
}

// ========== Resource Disks Tool Implementation ==========

func (e *PulseToolExecutor) executeGetResourceDisks(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	resourceFilter, _ := args["resource_id"].(string)
	typeFilter, _ := args["type"].(string)
	instanceFilter, _ := args["instance"].(string)
	minUsage, _ := args["min_usage"].(float64)

	state := e.stateProvider.GetState()

	var resources []ResourceDisksSummary

	// Process VMs
	if typeFilter == "" || strings.EqualFold(typeFilter, "vm") {
		for _, vm := range state.VMs {
			// Apply filters
			if resourceFilter != "" && vm.ID != resourceFilter && fmt.Sprintf("%d", vm.VMID) != resourceFilter {
				continue
			}
			if instanceFilter != "" && vm.Instance != instanceFilter {
				continue
			}
			// Skip VMs without disk data
			if len(vm.Disks) == 0 {
				continue
			}

			var disks []ResourceDiskInfo
			maxUsage := 0.0

			for _, disk := range vm.Disks {
				if disk.Usage > maxUsage {
					maxUsage = disk.Usage
				}

				disks = append(disks, ResourceDiskInfo{
					Device:     disk.Device,
					Mountpoint: disk.Mountpoint,
					Type:       disk.Type,
					TotalBytes: disk.Total,
					UsedBytes:  disk.Used,
					FreeBytes:  disk.Free,
					Usage:      disk.Usage,
				})
			}

			// Apply min_usage filter
			if minUsage > 0 && maxUsage < minUsage {
				continue
			}

			if disks == nil {
				disks = []ResourceDiskInfo{}
			}

			resources = append(resources, ResourceDisksSummary{
				ID:       vm.ID,
				VMID:     vm.VMID,
				Name:     vm.Name,
				Type:     "vm",
				Node:     vm.Node,
				Instance: vm.Instance,
				Disks:    disks,
			})
		}
	}

	// Process containers
	if typeFilter == "" || strings.EqualFold(typeFilter, "lxc") {
		for _, ct := range state.Containers {
			// Apply filters
			if resourceFilter != "" && ct.ID != resourceFilter && fmt.Sprintf("%d", ct.VMID) != resourceFilter {
				continue
			}
			if instanceFilter != "" && ct.Instance != instanceFilter {
				continue
			}
			// Skip containers without disk data
			if len(ct.Disks) == 0 {
				continue
			}

			var disks []ResourceDiskInfo
			maxUsage := 0.0

			for _, disk := range ct.Disks {
				if disk.Usage > maxUsage {
					maxUsage = disk.Usage
				}

				disks = append(disks, ResourceDiskInfo{
					Device:     disk.Device,
					Mountpoint: disk.Mountpoint,
					Type:       disk.Type,
					TotalBytes: disk.Total,
					UsedBytes:  disk.Used,
					FreeBytes:  disk.Free,
					Usage:      disk.Usage,
				})
			}

			// Apply min_usage filter
			if minUsage > 0 && maxUsage < minUsage {
				continue
			}

			if disks == nil {
				disks = []ResourceDiskInfo{}
			}

			resources = append(resources, ResourceDisksSummary{
				ID:       ct.ID,
				VMID:     ct.VMID,
				Name:     ct.Name,
				Type:     "lxc",
				Node:     ct.Node,
				Instance: ct.Instance,
				Disks:    disks,
			})
		}
	}

	if resources == nil {
		resources = []ResourceDisksSummary{}
	}

	if len(resources) == 0 {
		if resourceFilter != "" {
			return NewTextResult(fmt.Sprintf("No disk data found for resource '%s'. Guest agent may not be installed or disk info unavailable.", resourceFilter)), nil
		}
		return NewTextResult("No disk data available for any VMs or containers. Disk details require guest agents to be installed and running."), nil
	}

	response := ResourceDisksResponse{
		Resources: resources,
		Total:     len(resources),
	}

	return NewJSONResult(response), nil
}

// ========== Connection Health Tool Implementation ==========

func (e *PulseToolExecutor) executeGetConnectionHealth(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.stateProvider == nil {
		return NewTextResult("State provider not available."), nil
	}

	state := e.stateProvider.GetState()

	if len(state.ConnectionHealth) == 0 {
		return NewTextResult("No connection health data available."), nil
	}

	var connections []ConnectionStatus
	connected := 0
	disconnected := 0

	for instanceID, isConnected := range state.ConnectionHealth {
		connections = append(connections, ConnectionStatus{
			InstanceID: instanceID,
			Connected:  isConnected,
		})
		if isConnected {
			connected++
		} else {
			disconnected++
		}
	}

	response := ConnectionHealthResponse{
		Connections:  connections,
		Total:        len(connections),
		Connected:    connected,
		Disconnected: disconnected,
	}

	return NewJSONResult(response), nil
}
