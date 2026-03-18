package unifiedresources

import "strings"

const (
	IncidentCategoryProtection     = "protection"
	IncidentCategoryRebuild        = "rebuild"
	IncidentCategoryCapacity       = "capacity"
	IncidentCategoryRecoverability = "recoverability"
	IncidentCategoryAvailability   = "availability"
	IncidentCategoryDiskHealth     = "disk-health"
	IncidentCategoryHealth         = "health"
)

func IncidentCategoryForResource(resource *Resource, incident ResourceIncident) string {
	if resource == nil {
		return IncidentCategoryHealth
	}

	code := strings.TrimSpace(incident.Code)
	switch code {
	case "raid_degraded", "raid_unavailable", "unraid_invalid_disks", "unraid_disabled_disks", "unraid_missing_disks", "unraid_parity_unavailable", "unraid_no_parity", "zfs_pool_state":
		return IncidentCategoryProtection
	case "raid_rebuilding", "unraid_sync_active":
		return IncidentCategoryRebuild
	case "capacity_runway_low":
		if resource.Type == ResourceTypePBS || (resource.Storage != nil && IsBackupStorageResource(resource.Storage)) {
			return IncidentCategoryRecoverability
		}
		return IncidentCategoryCapacity
	case "pbs_datastore_state", "pbs_datastore_error", "backup_target_degraded":
		return IncidentCategoryRecoverability
	case "disk_failed", "disk_unavailable", "disk_smart_failed", "disk_wearout", "disk_health":
		return IncidentCategoryDiskHealth
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
