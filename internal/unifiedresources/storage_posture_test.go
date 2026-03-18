package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

// --- StorageRiskSummary ---

func TestStorageRiskSummary_NilRisk(t *testing.T) {
	got := StorageRiskSummary(nil)
	if got != "" {
		t.Errorf("expected empty string for nil risk, got %q", got)
	}
}

func TestStorageRiskSummary_ProtectionSummaryPrioritized(t *testing.T) {
	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "raid_degraded", Severity: storagehealth.RiskWarning, Summary: "RAID array is degraded"},
			{Code: "disk_usage_high", Severity: storagehealth.RiskMonitor, Summary: "Disk usage above 80%"},
		},
	}
	got := StorageRiskSummary(risk)
	if got != "RAID array is degraded" {
		t.Errorf("expected protection summary first, got %q", got)
	}
}

func TestStorageRiskSummary_RebuildSummaryAsSecondPriority(t *testing.T) {
	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "raid_rebuilding", Severity: storagehealth.RiskWarning, Summary: "RAID rebuild in progress"},
		},
	}
	got := StorageRiskSummary(risk)
	if got != "RAID rebuild in progress" {
		t.Errorf("expected rebuild summary, got %q", got)
	}
}

func TestStorageRiskSummary_FallsBackToFirstReason(t *testing.T) {
	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "disk_usage_high", Severity: storagehealth.RiskMonitor, Summary: "Disk usage above 80%"},
		},
	}
	got := StorageRiskSummary(risk)
	if got != "Disk usage above 80%" {
		t.Errorf("expected first reason summary, got %q", got)
	}
}

func TestStorageRiskSummary_EmptyReasons(t *testing.T) {
	risk := &StorageRisk{Reasons: nil}
	got := StorageRiskSummary(risk)
	if got != "" {
		t.Errorf("expected empty string for empty reasons, got %q", got)
	}
}

// --- StorageRiskSemantics ---

func TestStorageRiskSemantics_NilRisk(t *testing.T) {
	codes, protReduced, rebuild, protSummary, rebuildSummary := StorageRiskSemantics(nil)
	if codes != nil || protReduced || rebuild || protSummary != "" || rebuildSummary != "" {
		t.Error("nil risk should return zero values")
	}
}

func TestStorageRiskSemantics_RaidDegraded(t *testing.T) {
	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "raid_degraded", Severity: storagehealth.RiskCritical, Summary: "RAID degraded: 1 disk failed"},
		},
	}
	codes, protReduced, rebuild, protSummary, rebuildSummary := StorageRiskSemantics(risk)
	if len(codes) != 1 || codes[0] != "raid_degraded" {
		t.Errorf("expected [raid_degraded], got %v", codes)
	}
	if !protReduced {
		t.Error("expected protectionReduced=true")
	}
	if rebuild {
		t.Error("expected rebuildInProgress=false")
	}
	if protSummary != "RAID degraded: 1 disk failed" {
		t.Errorf("unexpected protectionSummary: %q", protSummary)
	}
	if rebuildSummary != "" {
		t.Errorf("unexpected rebuildSummary: %q", rebuildSummary)
	}
}

func TestStorageRiskSemantics_RaidRebuilding(t *testing.T) {
	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "raid_rebuilding", Severity: storagehealth.RiskWarning, Summary: "RAID rebuilding at 45%"},
		},
	}
	_, _, rebuild, _, rebuildSummary := StorageRiskSemantics(risk)
	if !rebuild {
		t.Error("expected rebuildInProgress=true")
	}
	if rebuildSummary != "RAID rebuilding at 45%" {
		t.Errorf("expected rebuild summary, got %q", rebuildSummary)
	}
}

func TestStorageRiskSemantics_MixedCodes(t *testing.T) {
	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "raid_degraded", Severity: storagehealth.RiskCritical, Summary: "Degraded array"},
			{Code: "raid_rebuilding", Severity: storagehealth.RiskWarning, Summary: "Rebuilding"},
			{Code: "disk_usage_high", Severity: storagehealth.RiskMonitor, Summary: "High usage"},
		},
	}
	codes, protReduced, rebuild, protSummary, rebuildSummary := StorageRiskSemantics(risk)
	if len(codes) != 3 {
		t.Errorf("expected 3 codes, got %d", len(codes))
	}
	if !protReduced {
		t.Error("expected protectionReduced=true")
	}
	if !rebuild {
		t.Error("expected rebuildInProgress=true")
	}
	if protSummary != "Degraded array" {
		t.Errorf("expected first protection summary, got %q", protSummary)
	}
	if rebuildSummary != "Rebuilding" {
		t.Errorf("expected first rebuild summary, got %q", rebuildSummary)
	}
}

func TestStorageRiskSemantics_UnraidCodes(t *testing.T) {
	unraidProtectionCodes := []string{
		"unraid_invalid_disks", "unraid_disabled_disks",
		"unraid_missing_disks", "unraid_parity_unavailable", "unraid_no_parity",
	}
	for _, code := range unraidProtectionCodes {
		risk := &StorageRisk{
			Reasons: []StorageRiskReason{
				{Code: code, Severity: storagehealth.RiskWarning, Summary: "Test summary"},
			},
		}
		_, protReduced, _, _, _ := StorageRiskSemantics(risk)
		if !protReduced {
			t.Errorf("code %q should set protectionReduced=true", code)
		}
	}

	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "unraid_sync_active", Severity: storagehealth.RiskMonitor, Summary: "Parity sync in progress"},
		},
	}
	_, _, rebuild, _, _ := StorageRiskSemantics(risk)
	if !rebuild {
		t.Error("unraid_sync_active should set rebuildInProgress=true")
	}
}

func TestStorageRiskSemantics_ZFSPoolState(t *testing.T) {
	risk := &StorageRisk{
		Reasons: []StorageRiskReason{
			{Code: "zfs_pool_state", Severity: storagehealth.RiskCritical, Summary: "ZFS pool is DEGRADED"},
		},
	}
	_, protReduced, _, protSummary, _ := StorageRiskSemantics(risk)
	if !protReduced {
		t.Error("expected protectionReduced=true for zfs_pool_state")
	}
	if protSummary != "ZFS pool is DEGRADED" {
		t.Errorf("unexpected protection summary: %q", protSummary)
	}
}

// --- IsBackupStorageResource ---

func TestIsBackupStorageResource_NilStorage(t *testing.T) {
	if IsBackupStorageResource(nil) {
		t.Error("nil storage should not be backup")
	}
}

func TestIsBackupStorageResource_BackupProtection(t *testing.T) {
	storage := &StorageMeta{Protection: "backup-repository"}
	if !IsBackupStorageResource(storage) {
		t.Error("backup-repository protection should identify as backup storage")
	}
}

func TestIsBackupStorageResource_BackupProtectionCaseInsensitive(t *testing.T) {
	storage := &StorageMeta{Protection: "Backup-Repository"}
	if !IsBackupStorageResource(storage) {
		t.Error("backup-repository should match case-insensitively")
	}
}

func TestIsBackupStorageResource_BackupContentType(t *testing.T) {
	storage := &StorageMeta{ContentTypes: []string{"images", "backup"}}
	if !IsBackupStorageResource(storage) {
		t.Error("content type 'backup' should identify as backup storage")
	}
}

func TestIsBackupStorageResource_NoBackupIndicators(t *testing.T) {
	storage := &StorageMeta{ContentTypes: []string{"images", "rootdir"}}
	if IsBackupStorageResource(storage) {
		t.Error("non-backup storage should not be identified as backup")
	}
}

// --- StorageConsumerImpactSummary ---

func TestStorageConsumerImpactSummary_NilStorage(t *testing.T) {
	got := StorageConsumerImpactSummary(nil)
	if got != "" {
		t.Errorf("expected empty for nil storage, got %q", got)
	}
}

func TestStorageConsumerImpactSummary_ZeroConsumers(t *testing.T) {
	storage := &StorageMeta{ConsumerCount: 0}
	got := StorageConsumerImpactSummary(storage)
	if got != "" {
		t.Errorf("expected empty for zero consumers, got %q", got)
	}
}

func TestStorageConsumerImpactSummary_SingleDependentNoNames(t *testing.T) {
	storage := &StorageMeta{ConsumerCount: 1}
	got := StorageConsumerImpactSummary(storage)
	if got != "Affects 1 dependent resource" {
		t.Errorf("got %q", got)
	}
}

func TestStorageConsumerImpactSummary_MultipleDependentsWithNames(t *testing.T) {
	storage := &StorageMeta{
		ConsumerCount: 5,
		TopConsumers: []StorageConsumerMeta{
			{Name: "vm-1"},
			{Name: "vm-2"},
			{Name: "vm-3"},
		},
	}
	got := StorageConsumerImpactSummary(storage)
	expected := "Affects 5 dependent resources: vm-1, vm-2, vm-3, and 2 more"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestStorageConsumerImpactSummary_ExactMatchNames(t *testing.T) {
	storage := &StorageMeta{
		ConsumerCount: 2,
		TopConsumers: []StorageConsumerMeta{
			{Name: "vm-1"},
			{Name: "vm-2"},
		},
	}
	got := StorageConsumerImpactSummary(storage)
	expected := "Affects 2 dependent resources: vm-1, vm-2"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestStorageConsumerImpactSummary_BackupStorage(t *testing.T) {
	storage := &StorageMeta{
		ConsumerCount: 3,
		Protection:    "backup-repository",
		TopConsumers: []StorageConsumerMeta{
			{Name: "web-server"},
		},
	}
	got := StorageConsumerImpactSummary(storage)
	expected := "Puts backups for 3 protected workloads at risk: web-server, and 2 more"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestStorageConsumerImpactSummary_BackupSingleWorkload(t *testing.T) {
	storage := &StorageMeta{
		ConsumerCount: 1,
		Protection:    "backup-repository",
	}
	got := StorageConsumerImpactSummary(storage)
	expected := "Puts backups for 1 protected workload at risk"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestStorageConsumerImpactSummary_SkipsEmptyNames(t *testing.T) {
	storage := &StorageMeta{
		ConsumerCount: 3,
		TopConsumers: []StorageConsumerMeta{
			{Name: ""},
			{Name: "  "},
			{Name: "valid-name"},
		},
	}
	got := StorageConsumerImpactSummary(storage)
	expected := "Affects 3 dependent resources: valid-name, and 2 more"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- StoragePostureSummary ---

func TestStoragePostureSummary_NilStorage(t *testing.T) {
	got := StoragePostureSummary(nil)
	if got != "" {
		t.Errorf("expected empty for nil storage, got %q", got)
	}
}

func TestStoragePostureSummary_RiskOnly(t *testing.T) {
	storage := &StorageMeta{
		RiskSummary: "RAID degraded",
	}
	got := StoragePostureSummary(storage)
	if got != "RAID degraded" {
		t.Errorf("expected risk summary only, got %q", got)
	}
}

func TestStoragePostureSummary_ConsumerOnly(t *testing.T) {
	storage := &StorageMeta{
		ConsumerImpactSummary: "Affects 3 resources",
	}
	got := StoragePostureSummary(storage)
	if got != "Affects 3 resources" {
		t.Errorf("expected consumer summary only, got %q", got)
	}
}

func TestStoragePostureSummary_CombinesRiskAndConsumer(t *testing.T) {
	storage := &StorageMeta{
		RiskSummary:           "RAID degraded",
		ConsumerImpactSummary: "Affects 5 dependent resources",
	}
	got := StoragePostureSummary(storage)
	expected := "RAID degraded. Affects 5 dependent resources"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestStoragePostureSummary_ComputesSummariesWhenNotPreset(t *testing.T) {
	storage := &StorageMeta{
		Risk: &StorageRisk{
			Reasons: []StorageRiskReason{
				{Code: "raid_degraded", Severity: storagehealth.RiskCritical, Summary: "Degraded array"},
			},
		},
		ConsumerCount: 2,
		TopConsumers: []StorageConsumerMeta{
			{Name: "vm-1"},
			{Name: "vm-2"},
		},
	}
	got := StoragePostureSummary(storage)
	expected := "Degraded array. Affects 2 dependent resources: vm-1, vm-2"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
