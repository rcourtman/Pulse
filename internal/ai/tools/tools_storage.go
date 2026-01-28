package tools

import (
	"context"
	"fmt"
)

// registerStorageTools registers the consolidated pulse_storage tool
func (e *PulseToolExecutor) registerStorageTools() {
	e.registry.Register(RegisteredTool{
		Definition: Tool{
			Name: "pulse_storage",
			Description: `Query storage pools, backups, snapshots, Ceph, and replication.

Types:
- pools: Storage pool usage and health (ZFS, Ceph, LVM, etc.)
- config: Proxmox storage.cfg configuration
- backups: Backup status for VMs/containers (PBS and PVE)
- backup_tasks: Recent backup task history
- snapshots: VM/container snapshots
- ceph: Ceph cluster status from Proxmox API
- ceph_details: Detailed Ceph status from host agents
- replication: Proxmox replication job status
- pbs_jobs: PBS backup, sync, verify, prune jobs
- raid: Host RAID array status
- disk_health: SMART and RAID health from agents
- resource_disks: VM/container filesystem usage

Examples:
- List storage pools: type="pools"
- Get specific storage: type="pools", storage_id="local-lvm"
- Get backups for VM: type="backups", resource_id="101"
- Get Ceph status: type="ceph"
- Get replication jobs: type="replication"`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]PropertySchema{
					"type": {
						Type:        "string",
						Description: "Storage type to query",
						Enum:        []string{"pools", "config", "backups", "backup_tasks", "snapshots", "ceph", "ceph_details", "replication", "pbs_jobs", "raid", "disk_health", "resource_disks"},
					},
					"storage_id": {
						Type:        "string",
						Description: "Filter by storage ID (for pools, config)",
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
// All handler functions are implemented in tools_infrastructure.go
func (e *PulseToolExecutor) executeStorage(ctx context.Context, args map[string]interface{}) (CallToolResult, error) {
	storageType, _ := args["type"].(string)
	switch storageType {
	case "pools":
		return e.executeListStorage(ctx, args)
	case "config":
		return e.executeGetStorageConfig(ctx, args)
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
		return NewErrorResult(fmt.Errorf("unknown type: %s. Use: pools, config, backups, backup_tasks, snapshots, ceph, ceph_details, replication, pbs_jobs, raid, disk_health, resource_disks", storageType)), nil
	}
}
