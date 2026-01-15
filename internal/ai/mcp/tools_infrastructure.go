package mcp

import (
	"context"
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
