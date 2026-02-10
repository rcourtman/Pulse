package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// registerStorageTools registers the pulse_storage tool
func (e *PulseToolExecutor) registerStorageTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name:        "pulse_storage",
			Description: `Query storage pools, backups, snapshots, Ceph, replication, RAID, and disk health. Use the "type" parameter to select what to query.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Storage type to query",
						Enum:        []string{"pools", "backups", "backup_tasks", "snapshots", "ceph", "ceph_details", "replication", "pbs_jobs", "raid", "disk_health", "resource_disks"},
					},
					"storage_id": {
						Type:        "string",
						Description: "Filter by storage ID (for pools)",
					},
					"resource_id": {
						Type:        "string",
						Description: "Filter by VM/container ID (for backups, snapshots, resource_disks)",
					},
					"guest_id": {
						Type:        "string",
						Description: "Filter by guest ID (for snapshots, backup_tasks)",
					},
					"vm_id": {
						Type:        "string",
						Description: "Filter by VM ID (for replication)",
					},
					"instance": {
						Type:        "string",
						Description: "Filter by Proxmox/PBS instance",
					},
					"node": {
						Type:        "string",
						Description: "Filter by node name",
					},
					"host": {
						Type:        "string",
						Description: "Filter by host (for raid, ceph_details)",
					},
					"cluster": {
						Type:        "string",
						Description: "Filter by Ceph cluster name",
					},
					"job_type": {
						Type:        "string",
						Description: "Filter PBS jobs by type: backup, sync, verify, prune, garbage",
						Enum:        []string{"backup", "sync", "verify", "prune", "garbage"},
					},
					"state": {
						Type:        "string",
						Description: "Filter RAID arrays by state: clean, degraded, rebuilding",
					},
					"status": {
						Type:        "string",
						Description: "Filter backup tasks by status: ok, error",
					},
					"resource_type": {
						Type:        "string",
						Description: "Filter by type: vm or lxc (for resource_disks)",
					},
					"min_usage": {
						Type:        "number",
						Description: "Only show resources with disk usage above this percentage (for resource_disks)",
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
				Required: []string{"type"},
			},
		},
		Handler: func(ctx context.Context, exec *PulseToolExecutor, args map[string]interface{}) (CallToolResult, error) {
			return exec.executeStorage(ctx, args)
		},
	})
}

// executeStorage routes to the appropriate storage handler based on type
func (e *PulseToolExecutor) executeStorage(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	storageType, _ := args["type"].(string)
	switch storageType {
	case "pools":
		return e.executeListStorage(ctx, args)

	case "backups":
		return e.executeListBackups(ctx, args)
	case "backup_tasks":
		return e.executeListBackupTasks(ctx, args)
	case "snapshots":
		return e.executeListSnapshots(ctx, args)
	case "ceph":
		return e.executeGetCephStatus(ctx, args)
	case "ceph_details":
		return e.executeGetHostCephDetails(ctx, args)
	case "replication":
		return e.executeGetReplication(ctx, args)
	case "pbs_jobs":
		return e.executeListPBSJobs(ctx, args)
	case "raid":
		return e.executeGetHostRAIDStatus(ctx, args)
	case "disk_health":
		return e.executeGetDiskHealth(ctx, args)
	case "resource_disks":
		return e.executeGetResourceDisks(ctx, args)
	default:
		return NewErrorResult(fmt.Errorf("unknown type: %s. Use: pools, backups, backup_tasks, snapshots, ceph, ceph_details, replication, pbs_jobs, raid, disk_health, resource_disks", storageType)), nil
	}
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
				UsagePercent: ds.Usage,
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

	response := StorageResponse{}

	// Storage pools
	count := 0
	var storageResources []unifiedresources.Resource
	if e.unifiedResourceProvider != nil {
		storageResources = e.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypeStorage)
	}
	for _, r := range storageResources {
		if r.Storage == nil {
			continue
		}
		if storageID != "" && r.ID != storageID && r.Name != storageID {
			continue
		}
		if count < offset {
			count++
			continue
		}
		if len(response.Pools) >= limit {
			break
		}

		pool := storagePoolSummaryFromResource(r)
		response.Pools = append(response.Pools, pool)
		count++
	}

	// Ceph clusters from unified resources
	if e.unifiedResourceProvider != nil {
		for _, r := range e.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypeCeph) {
			if r.Ceph == nil {
				continue
			}
			c := r.Ceph
			usedBytes, totalBytes := cephBytesFromResource(r)
			usagePercent := 0.0
			if totalBytes > 0 {
				usagePercent = float64(usedBytes) / float64(totalBytes) * 100
			}
			response.CephClusters = append(response.CephClusters, CephClusterSummary{
				Name:          r.Name,
				Health:        c.HealthStatus,
				HealthMessage: c.HealthMessage,
				UsagePercent:  usagePercent,
				UsedTB:        float64(usedBytes) / (1024 * 1024 * 1024 * 1024),
				TotalTB:       float64(totalBytes) / (1024 * 1024 * 1024 * 1024),
				NumOSDs:       c.NumOSDs,
				NumOSDsUp:     c.NumOSDsUp,
				NumOSDsIn:     c.NumOSDsIn,
				NumMons:       c.NumMons,
				NumMgrs:       c.NumMgrs,
			})
		}
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

func storagePoolSummaryFromResource(r unifiedresources.Resource) StoragePoolSummary {
	var (
		poolType     string
		content      string
		shared       bool
		node         string
		instance     string
		usedBytes    int64
		totalBytes   int64
		usagePercent float64
	)

	if r.Storage != nil {
		poolType = r.Storage.Type
		content = r.Storage.Content
		shared = r.Storage.Shared
		if content == "" && len(r.Storage.ContentTypes) > 0 {
			content = strings.Join(r.Storage.ContentTypes, ",")
		}
	}
	if r.Proxmox != nil {
		node = r.Proxmox.NodeName
		instance = r.Proxmox.Instance
	}
	if r.Metrics != nil && r.Metrics.Disk != nil {
		if r.Metrics.Disk.Used != nil {
			usedBytes = *r.Metrics.Disk.Used
		}
		if r.Metrics.Disk.Total != nil {
			totalBytes = *r.Metrics.Disk.Total
		}
		usagePercent = r.Metrics.Disk.Percent
	}
	if usagePercent == 0 && totalBytes > 0 {
		usagePercent = float64(usedBytes) / float64(totalBytes) * 100
	}

	freeBytes := int64(0)
	if totalBytes > usedBytes {
		freeBytes = totalBytes - usedBytes
	}

	active := r.Status != unifiedresources.StatusOffline
	enabled := r.Status != unifiedresources.StatusOffline

	id := r.ID
	if id == "" {
		id = r.Name
	}
	name := r.Name
	if name == "" {
		name = id
	}

	return StoragePoolSummary{
		ID:           id,
		Name:         name,
		Node:         node,
		Instance:     instance,
		Type:         poolType,
		Status:       string(r.Status),
		Enabled:      enabled,
		Active:       active,
		UsagePercent: usagePercent,
		UsedGB:       float64(usedBytes) / (1024 * 1024 * 1024),
		TotalGB:      float64(totalBytes) / (1024 * 1024 * 1024),
		FreeGB:       float64(freeBytes) / (1024 * 1024 * 1024),
		Content:      content,
		Shared:       shared,
	}
}

// cephBytesFromResource extracts used/total bytes from a unified Ceph resource.
// It prefers the Metrics.Disk values (which carry absolute bytes), falling back
// to summing pool-level data from CephMeta.
func cephBytesFromResource(r unifiedresources.Resource) (usedBytes, totalBytes int64) {
	if r.Metrics != nil && r.Metrics.Disk != nil && r.Metrics.Disk.Used != nil && r.Metrics.Disk.Total != nil {
		return *r.Metrics.Disk.Used, *r.Metrics.Disk.Total
	}
	if r.Ceph != nil {
		for _, p := range r.Ceph.Pools {
			usedBytes += p.StoredBytes
			totalBytes += p.StoredBytes + p.AvailableBytes
		}
	}
	return
}

func (e *PulseToolExecutor) executeGetDiskHealth(_ context.Context, _ map[string]interface{}) (CallToolResult, error) {
	if e.diskHealthProvider == nil {
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

// executeGetCephStatus returns Ceph cluster status
func (e *PulseToolExecutor) executeGetCephStatus(_ context.Context, args map[string]interface{}) (CallToolResult, error) {
	clusterFilter, _ := args["cluster"].(string)

	type CephSummary struct {
		Name    string                 `json:"name"`
		Health  string                 `json:"health"`
		Details map[string]interface{} `json:"details,omitempty"`
	}

	var results []CephSummary

	if e.unifiedResourceProvider != nil {
		resources := e.unifiedResourceProvider.GetByType(unifiedresources.ResourceTypeCeph)
		for _, r := range resources {
			if r.Ceph == nil {
				continue
			}
			if clusterFilter != "" && r.Name != clusterFilter {
				continue
			}
			c := r.Ceph
			summary := CephSummary{
				Name:    r.Name,
				Health:  c.HealthStatus,
				Details: make(map[string]interface{}),
			}
			if c.HealthMessage != "" {
				summary.Details["health_message"] = c.HealthMessage
			}
			if c.NumOSDs > 0 {
				summary.Details["osd_count"] = c.NumOSDs
				summary.Details["osds_up"] = c.NumOSDsUp
				summary.Details["osds_in"] = c.NumOSDsIn
				summary.Details["osds_down"] = c.NumOSDs - c.NumOSDsUp
			}
			if c.NumMons > 0 {
				summary.Details["monitors"] = c.NumMons
			}
			usedBytes, totalBytes := cephBytesFromResource(r)
			if totalBytes > 0 {
				summary.Details["total_bytes"] = totalBytes
				summary.Details["used_bytes"] = usedBytes
				summary.Details["available_bytes"] = totalBytes - usedBytes
				usagePercent := float64(usedBytes) / float64(totalBytes) * 100
				summary.Details["usage_percent"] = usagePercent
			}
			if len(c.Pools) > 0 {
				summary.Details["pools"] = c.Pools
			}
			results = append(results, summary)
		}
	}

	if len(results) == 0 {
		if clusterFilter != "" {
			return NewTextResult(fmt.Sprintf("Ceph cluster '%s' not found.", clusterFilter)), nil
		}
		return NewTextResult("No Ceph clusters found. Ceph may not be configured or data is not yet available."), nil
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
	if rs := e.getReadState(); rs != nil {
		for _, w := range rs.Workloads() {
			if w.VMID() > 0 && w.Name() != "" {
				vmNames[w.VMID()] = w.Name()
			}
		}
	} else {
		for _, vm := range state.VMs {
			vmNames[vm.VMID] = vm.Name
		}
		for _, ct := range state.Containers {
			vmNames[ct.VMID] = ct.Name
		}
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
	if rs := e.getReadState(); rs != nil {
		for _, w := range rs.Workloads() {
			if w.VMID() > 0 && w.Name() != "" {
				vmNames[w.VMID()] = w.Name()
			}
		}
	} else {
		for _, vm := range state.VMs {
			vmNames[vm.VMID] = vm.Name
		}
		for _, ct := range state.Containers {
			vmNames[ct.VMID] = ct.Name
		}
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
