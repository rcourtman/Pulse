package alerts

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestAlertNoiseAuditorGroupsLegacyAndStableGuestDiskIdentity(t *testing.T) {
	guestID := BuildGuestKey("cluster", "node-a", 301)
	legacyResourceID := fmt.Sprintf("%s-disk-root-scsi0", guestID)
	stableResourceID := guestDiskResourceID(guestID, "root")
	start := time.Now().Add(-30 * time.Minute)

	report := AuditAlertNoise(nil, []Alert{
		{
			ID:           fmt.Sprintf("%s-disk", legacyResourceID),
			Type:         "disk",
			ResourceID:   legacyResourceID,
			ResourceName: "vm-301",
			StartTime:    start,
			LastSeen:     start.Add(5 * time.Minute),
			Metadata: map[string]interface{}{
				"mountpoint": "/",
				"device":     "scsi0",
			},
		},
		{
			ID:           fmt.Sprintf("%s-disk", stableResourceID),
			Type:         "disk",
			ResourceID:   stableResourceID,
			ResourceName: "vm-301",
			StartTime:    start,
			LastSeen:     start.Add(10 * time.Minute),
			Metadata: map[string]interface{}{
				"diskKey":        "root",
				"identityKey":    stableResourceID,
				"identitySource": "mountpoint",
			},
		},
	}, time.Time{})

	finding := requireNoiseFinding(t, report, AlertNoiseFindingIdentityChurn)
	if finding.Identity.Key != "guest:cluster:301/disk:root::disk" {
		t.Fatalf("identity key = %q, want guest:cluster:301/disk:root::disk", finding.Identity.Key)
	}
	if finding.Identity.Source != "guest.disk" {
		t.Fatalf("identity source = %q, want guest.disk", finding.Identity.Source)
	}

	wantAlertIDs := []string{
		fmt.Sprintf("%s-disk", legacyResourceID),
		fmt.Sprintf("%s-disk", stableResourceID),
	}
	if !reflect.DeepEqual(finding.AlertIDs, wantAlertIDs) {
		t.Fatalf("alert IDs = %#v, want %#v", finding.AlertIDs, wantAlertIDs)
	}

	wantResourceIDs := []string{legacyResourceID, stableResourceID}
	if !reflect.DeepEqual(finding.ResourceIDs, wantResourceIDs) {
		t.Fatalf("resource IDs = %#v, want %#v", finding.ResourceIDs, wantResourceIDs)
	}
}

func TestAlertNoiseAuditorDoesNotFlagDistinctMetadataIdentities(t *testing.T) {
	report := AuditAlertNoise(nil, []Alert{
		{
			ID:         "a-disk",
			Type:       "disk",
			ResourceID: "legacy-a",
			Metadata: map[string]interface{}{
				"identityKey": "guest-a/disk:root",
			},
		},
		{
			ID:         "b-disk",
			Type:       "disk",
			ResourceID: "legacy-b",
			Metadata: map[string]interface{}{
				"identityKey": "guest-b/disk:root",
			},
		},
	}, time.Time{})

	if len(report.Findings) != 0 {
		t.Fatalf("expected no findings for distinct metadata identities, got %#v", report.Findings)
	}
}

func TestAlertNoiseAuditorFallsBackToResourceAndType(t *testing.T) {
	report := AuditAlertNoise(nil, []Alert{
		{
			ID:         "node-a-cpu",
			Type:       "cpu",
			ResourceID: "node-a",
		},
		{
			ID:         "node-a-cpu-renamed",
			Type:       "cpu",
			ResourceID: "node-a",
		},
	}, time.Time{})

	finding := requireNoiseFinding(t, report, AlertNoiseFindingIdentityChurn)
	if finding.Identity.Key != "node-a::cpu" {
		t.Fatalf("identity key = %q, want node-a::cpu", finding.Identity.Key)
	}
	if finding.Identity.Source != "resourceId" {
		t.Fatalf("identity source = %q, want resourceId", finding.Identity.Source)
	}
}

func TestAlertNoiseAuditorFlagsDuplicateActiveGuestDiskIdentity(t *testing.T) {
	oldGuestID := BuildGuestKey("cluster", "node-a", 42)
	newGuestID := BuildGuestKey("cluster", "node-b", 42)
	oldResourceID := guestDiskResourceID(oldGuestID, "root")
	newResourceID := guestDiskResourceID(newGuestID, "root")

	report := AuditAlertNoise([]Alert{
		{
			ID:         fmt.Sprintf("%s-disk", oldResourceID),
			Type:       "disk",
			ResourceID: oldResourceID,
			Instance:   "cluster",
			Metadata: map[string]interface{}{
				"diskKey": "root",
			},
		},
		{
			ID:         fmt.Sprintf("%s-disk", newResourceID),
			Type:       "disk",
			ResourceID: newResourceID,
			Instance:   "cluster",
			Metadata: map[string]interface{}{
				"diskKey": "root",
			},
		},
	}, nil, time.Time{})

	finding := requireNoiseFinding(t, report, AlertNoiseFindingDuplicateActive)
	if finding.Identity.Key != "guest:cluster:42/disk:root::disk" {
		t.Fatalf("identity key = %q, want guest:cluster:42/disk:root::disk", finding.Identity.Key)
	}
	if finding.Count != 2 {
		t.Fatalf("duplicate active count = %d, want 2", finding.Count)
	}
}

func requireNoiseFinding(t *testing.T, report AlertNoiseReport, kind AlertNoiseFindingKind) AlertNoiseFinding {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.Kind == kind {
			return finding
		}
	}
	t.Fatalf("expected finding kind %q in %#v", kind, report.Findings)
	return AlertNoiseFinding{}
}
