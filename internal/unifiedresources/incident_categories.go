package unifiedresources

import "strings"

const (
	IncidentCategoryProtection     = "protection"
	IncidentCategoryRebuild        = "rebuild"
	IncidentCategoryCapacity       = "capacity"
	IncidentCategoryRecoverability = "recoverability"
	IncidentCategoryAvailability   = "availability"
	IncidentCategoryDiskHealth     = "disk-health"
	IncidentCategoryWorkloadHealth = "workload-health"
	IncidentCategoryHealth         = "health"
)

func IncidentCategoryForResource(resource *Resource, incident ResourceIncident) string {
	if resource == nil {
		return IncidentCategoryHealth
	}

	code := strings.TrimSpace(incident.Code)
	switch code {
	case "raid_degraded", "raid_unavailable", "unraid_invalid_disks", "unraid_disabled_disks", "unraid_missing_disks", "unraid_parity_unavailable", "unraid_no_parity", "zfs_pool_state", "zfs_device_state", "zfs_device_missing":
		return IncidentCategoryProtection
	case "raid_rebuilding", "unraid_sync_active", "zfs_resilver_active", "zfs_scrub_active":
		return IncidentCategoryRebuild
	case "capacity_runway_low":
		if resource.Type == ResourceTypePBS || (resource.Storage != nil && IsBackupStorageResource(resource.Storage)) {
			return IncidentCategoryRecoverability
		}
		return IncidentCategoryCapacity
	case "pbs_datastore_state", "pbs_datastore_error", "backup_target_degraded":
		return IncidentCategoryRecoverability
	case "disk_failed", "disk_unavailable", "disk_smart_failed", "disk_wearout", "disk_health", "zfs_pool_errors", "zfs_device_errors", "zfs_scan_errors", "zfs_scan_failed":
		return IncidentCategoryDiskHealth
	case "availability_unreachable", "zfs_dataset_locked", "zfs_dataset_unmounted":
		return IncidentCategoryAvailability
	case "truenas_app_crashed", "truenas_app_stopped", "truenas_app_container_failed":
		return IncidentCategoryWorkloadHealth
	}

	if resource.Type == ResourceTypeNetworkEndpoint || len(AvailabilityChecksForResource(*resource)) > 0 {
		return IncidentCategoryAvailability
	}
	if resource.Type == ResourceTypePhysicalDisk {
		return IncidentCategoryDiskHealth
	}
	if resource.Type == ResourceTypePBS || (resource.Storage != nil && IsBackupStorageResource(resource.Storage)) {
		return IncidentCategoryRecoverability
	}
	if resource.Storage != nil {
		return IncidentCategoryHealth
	}
	return IncidentCategoryHealth
}
