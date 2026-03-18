package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// --- IncidentCategoryForResource ---

func TestIncidentCategoryForResource_NilResource(t *testing.T) {
	got := IncidentCategoryForResource(nil, ResourceIncident{Code: "any"})
	if got != IncidentCategoryHealth {
		t.Errorf("nil resource should default to %q, got %q", IncidentCategoryHealth, got)
	}
}

func TestIncidentCategoryForResource_ProtectionCodes(t *testing.T) {
	protectionCodes := []string{
		"raid_degraded", "raid_unavailable",
		"unraid_invalid_disks", "unraid_disabled_disks",
		"unraid_missing_disks", "unraid_parity_unavailable",
		"unraid_no_parity", "zfs_pool_state",
	}
	resource := &Resource{ID: "r-1", Type: ResourceTypeStorage}
	for _, code := range protectionCodes {
		got := IncidentCategoryForResource(resource, ResourceIncident{Code: code})
		if got != IncidentCategoryProtection {
			t.Errorf("code %q should map to %q, got %q", code, IncidentCategoryProtection, got)
		}
	}
}

func TestIncidentCategoryForResource_RebuildCodes(t *testing.T) {
	rebuildCodes := []string{"raid_rebuilding", "unraid_sync_active"}
	resource := &Resource{ID: "r-1"}
	for _, code := range rebuildCodes {
		got := IncidentCategoryForResource(resource, ResourceIncident{Code: code})
		if got != IncidentCategoryRebuild {
			t.Errorf("code %q should map to %q, got %q", code, IncidentCategoryRebuild, got)
		}
	}
}

func TestIncidentCategoryForResource_CapacityRunwayLow_RegularStorage(t *testing.T) {
	resource := &Resource{
		ID:   "r-1",
		Type: ResourceTypeStorage,
		Storage: &StorageMeta{
			ContentTypes: []string{"images", "rootdir"},
		},
	}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: "capacity_runway_low"})
	if got != IncidentCategoryCapacity {
		t.Errorf("capacity_runway_low on regular storage should be %q, got %q", IncidentCategoryCapacity, got)
	}
}

func TestIncidentCategoryForResource_CapacityRunwayLow_BackupStorage(t *testing.T) {
	resource := &Resource{
		ID:   "r-1",
		Type: ResourceTypeStorage,
		Storage: &StorageMeta{
			Protection: "backup-repository",
		},
	}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: "capacity_runway_low"})
	if got != IncidentCategoryRecoverability {
		t.Errorf("capacity_runway_low on backup storage should be %q, got %q", IncidentCategoryRecoverability, got)
	}
}

func TestIncidentCategoryForResource_CapacityRunwayLow_PBSResource(t *testing.T) {
	resource := &Resource{
		ID:   "r-1",
		Type: ResourceTypePBS,
	}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: "capacity_runway_low"})
	if got != IncidentCategoryRecoverability {
		t.Errorf("capacity_runway_low on PBS should be %q, got %q", IncidentCategoryRecoverability, got)
	}
}

func TestIncidentCategoryForResource_PBSDatastoreCodes(t *testing.T) {
	codes := []string{"pbs_datastore_state", "pbs_datastore_error", "backup_target_degraded"}
	resource := &Resource{ID: "r-1"}
	for _, code := range codes {
		got := IncidentCategoryForResource(resource, ResourceIncident{Code: code})
		if got != IncidentCategoryRecoverability {
			t.Errorf("code %q should map to %q, got %q", code, IncidentCategoryRecoverability, got)
		}
	}
}

func TestIncidentCategoryForResource_DiskHealthCodes(t *testing.T) {
	codes := []string{"disk_failed", "disk_unavailable", "disk_smart_failed", "disk_wearout", "disk_health"}
	resource := &Resource{ID: "r-1"}
	for _, code := range codes {
		got := IncidentCategoryForResource(resource, ResourceIncident{Code: code})
		if got != IncidentCategoryDiskHealth {
			t.Errorf("code %q should map to %q, got %q", code, IncidentCategoryDiskHealth, got)
		}
	}
}

func TestIncidentCategoryForResource_PhysicalDiskFallback(t *testing.T) {
	resource := &Resource{
		ID:   "r-1",
		Type: ResourceTypePhysicalDisk,
	}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: "unknown_issue"})
	if got != IncidentCategoryDiskHealth {
		t.Errorf("physical disk should fall back to %q, got %q", IncidentCategoryDiskHealth, got)
	}
}

func TestIncidentCategoryForResource_PBSFallback(t *testing.T) {
	resource := &Resource{
		ID:   "r-1",
		Type: ResourceTypePBS,
	}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: "some_other_code"})
	if got != IncidentCategoryRecoverability {
		t.Errorf("PBS resource should fall back to %q, got %q", IncidentCategoryRecoverability, got)
	}
}

func TestIncidentCategoryForResource_BackupStorageFallback(t *testing.T) {
	resource := &Resource{
		ID:   "r-1",
		Type: ResourceTypeStorage,
		Storage: &StorageMeta{
			ContentTypes: []string{"backup"},
		},
	}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: "some_other_code"})
	if got != IncidentCategoryRecoverability {
		t.Errorf("backup storage should fall back to %q, got %q", IncidentCategoryRecoverability, got)
	}
}

func TestIncidentCategoryForResource_StorageFallback(t *testing.T) {
	resource := &Resource{
		ID:      "r-1",
		Type:    ResourceTypeStorage,
		Storage: &StorageMeta{},
	}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: "generic"})
	if got != IncidentCategoryHealth {
		t.Errorf("generic storage should fall back to %q, got %q", IncidentCategoryHealth, got)
	}
}

func TestIncidentCategoryForResource_DefaultFallback(t *testing.T) {
	resource := &Resource{
		ID:   "r-1",
		Type: ResourceTypeVM,
	}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: "generic"})
	if got != IncidentCategoryHealth {
		t.Errorf("VM with unknown code should fall back to %q, got %q", IncidentCategoryHealth, got)
	}
}

func TestIncidentCategoryForResource_TrimmedCodeMatches(t *testing.T) {
	resource := &Resource{ID: "r-1"}
	got := IncidentCategoryForResource(resource, ResourceIncident{Code: " raid_degraded "})
	if got != IncidentCategoryProtection {
		t.Errorf("whitespace-padded code should still match, got %q", got)
	}
}

// --- incidentSummaryPreference ---

func TestIncidentSummaryPreference_EmptySummary(t *testing.T) {
	resource := &Resource{ID: "r-1"}
	got := incidentSummaryPreference(resource, ResourceIncident{Summary: ""})
	if got != 0 {
		t.Errorf("empty summary should have preference 0, got %d", got)
	}
}

func TestIncidentSummaryPreference_NilResource(t *testing.T) {
	got := incidentSummaryPreference(nil, ResourceIncident{Summary: "something"})
	if got != 0 {
		t.Errorf("nil resource should have preference 0, got %d", got)
	}
}

func TestIncidentSummaryPreference_MatchesStorageRiskSummary(t *testing.T) {
	resource := &Resource{
		ID: "r-1",
		Storage: &StorageMeta{
			RiskSummary: "RAID array degraded",
		},
	}
	got := incidentSummaryPreference(resource, ResourceIncident{Summary: "RAID array degraded"})
	if got <= 0 {
		t.Errorf("summary matching storage risk should have positive preference, got %d", got)
	}
}

func TestIncidentSummaryPreference_NoMatch(t *testing.T) {
	resource := &Resource{
		ID: "r-1",
		Storage: &StorageMeta{
			RiskSummary: "RAID array degraded",
		},
	}
	got := incidentSummaryPreference(resource, ResourceIncident{Summary: "Something completely different"})
	if got != 0 {
		t.Errorf("non-matching summary should have preference 0, got %d", got)
	}
}

// --- refreshResourceIncidentRollup (extended) ---

func TestRefreshResourceIncidentRollup_SummaryPreferenceBreaksTie(t *testing.T) {
	resource := &Resource{
		ID: "r-1",
		Storage: &StorageMeta{
			RiskSummary: "RAID array degraded",
		},
		Incidents: []ResourceIncident{
			{Code: "some_code", Severity: storagehealth.RiskWarning, Summary: "Non-preferred summary"},
			{Code: "raid_degraded", Severity: storagehealth.RiskWarning, Summary: "RAID array degraded"},
		},
	}
	refreshResourceIncidentRollup(resource)
	if resource.IncidentSummary != "RAID array degraded" {
		t.Errorf("expected preferred summary to win tie, got %q", resource.IncidentSummary)
	}
}

func TestRefreshResourceIncidentRollup_SetsCategory(t *testing.T) {
	resource := &Resource{
		ID:   "r-1",
		Type: ResourceTypeStorage,
		Incidents: []ResourceIncident{
			{Code: "raid_degraded", Severity: storagehealth.RiskCritical, Summary: "Degraded"},
		},
	}
	refreshResourceIncidentRollup(resource)
	if resource.IncidentCategory != IncidentCategoryProtection {
		t.Errorf("expected category %q, got %q", IncidentCategoryProtection, resource.IncidentCategory)
	}
}

func TestRefreshResourceIncidentRollup_AlphabeticBreaksLastTie(t *testing.T) {
	resource := &Resource{
		ID: "r-1",
		Incidents: []ResourceIncident{
			{Code: "z_code", Severity: storagehealth.RiskWarning, Summary: "Zebra issue"},
			{Code: "a_code", Severity: storagehealth.RiskWarning, Summary: "Alpha issue"},
		},
	}
	refreshResourceIncidentRollup(resource)
	// When severity and preference are equal, alphabetically earlier summary should win.
	if resource.IncidentSummary != "Alpha issue" {
		t.Errorf("expected alphabetically first summary to win tie, got %q", resource.IncidentSummary)
	}
}
