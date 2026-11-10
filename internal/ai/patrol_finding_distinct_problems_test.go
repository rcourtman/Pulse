package ai

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// One scan can find two different problems on one guest: its disk filled up,
// and it has never been backed up. They are different work for the operator
// and each gets its own investigation. Seen merged on a Proxmox lab: the
// finding that remained had the disk title and an investigation about backups.
func TestPatrolReportsOfDifferentProblemsOnOneGuestStaySeparate(t *testing.T) {
	for _, guestType := range []string{"system-container", "vm"} {
		t.Run(guestType, func(t *testing.T) {
			snapshot := models.StateSnapshot{}
			disk := models.Disk{Total: 8 << 30, Used: 7 << 30, Free: 1 << 30, Usage: 92}
			if guestType == "vm" {
				snapshot.VMs = []models.VM{{ID: "pmxlab:pmxlab:111", VMID: 111, Name: "nextcloud", Node: "pmxlab", Instance: "pmxlab", Status: "running", Type: "qemu", Disk: disk}}
			} else {
				snapshot.Containers = []models.Container{{ID: "pmxlab:pmxlab:111", VMID: 111, Name: "nextcloud", Node: "pmxlab", Instance: "pmxlab", Status: "running", Type: "lxc", Disk: disk}}
			}
			ps := NewPatrolService(nil, nil)
			adapter := newPatrolFindingCreatorAdapterState(ps, newPatrolRuntimeState(snapshot))

			diskID, diskNew, err := adapter.CreateFinding(tools.PatrolFindingInput{
				Key: "disk-high", Severity: "warning", Category: "capacity",
				ResourceID: "pmxlab:pmxlab:111", ResourceName: "nextcloud", ResourceType: guestType,
				Title:          "nextcloud disk jumped abruptly from 3% to 92% and has stayed there",
				Description:    "The root disk local:111/vm-111-disk-0.raw on storage local was flat at 3.3%, then reached 92% within 90 seconds.",
				Recommendation: "Find what wrote to the disk before it fills.",
				Evidence:       "Disk usage 92.0% for every sample since 20:03.",
			})
			if err != nil || !diskNew {
				t.Fatalf("disk report = (%q, %t, %v), want a new finding", diskID, diskNew, err)
			}
			backupID, backupNew, err := adapter.CreateFinding(tools.PatrolFindingInput{
				Key: "backup-never", Severity: "warning", Category: "backup",
				ResourceID: "pmxlab:pmxlab:111", ResourceName: "nextcloud", ResourceType: guestType,
				Title:          "nextcloud has never been backed up",
				Description:    "Last Backup: never. No backup job covers this guest and no backup of it exists on storage local, so a lost disk cannot be restored.",
				Recommendation: "Add the guest to a backup job.",
				Evidence:       "Guest metrics table shows Last Backup: never.",
			})
			if err != nil {
				t.Fatalf("backup report failed: %v", err)
			}

			active := ps.findings.GetActive(FindingSeverityInfo)
			if !backupNew || len(active) != 2 {
				t.Fatalf("two different problems became %d finding(s), backup report new=%t: %s", len(active), backupNew, describeDistinctFindings(active))
			}
			diskFinding, backupFinding := ps.findings.Get(diskID), ps.findings.Get(backupID)
			if diskFinding == nil || backupFinding == nil {
				t.Fatalf("a finding is missing: disk=%v backup=%v", diskFinding != nil, backupFinding != nil)
			}
			if diskFinding.Category != FindingCategoryCapacity || backupFinding.Category != FindingCategoryBackup {
				t.Fatalf("categories = %s and %s, want capacity and backup", diskFinding.Category, backupFinding.Category)
			}
			if diskFinding.Evidence != "Disk usage 92.0% for every sample since 20:03." || diskFinding.TimesRaised != 1 {
				t.Errorf("the disk finding took in the backup report: evidence %q, raised %d times", diskFinding.Evidence, diskFinding.TimesRaised)
			}

			// The same problem reported again under another key is still one finding.
			_, again, err := adapter.CreateFinding(tools.PatrolFindingInput{
				Key: "rootfs-nearly-full", Severity: "warning", Category: "capacity",
				ResourceID: "pmxlab:pmxlab:111", ResourceName: "nextcloud", ResourceType: guestType,
				Title:       "nextcloud disk jumped abruptly from 3% to 92% and has stayed there",
				Description: "Still at 92%.", Evidence: "Disk usage 92.0%.",
			})
			if err != nil || again {
				t.Fatalf("repeat of the disk problem under another key = (new %t, %v), want it merged", again, err)
			}
			if active := ps.findings.GetActive(FindingSeverityInfo); len(active) != 2 {
				t.Fatalf("after the repeat there are %d findings, want 2: %s", len(active), describeDistinctFindings(active))
			}
		})
	}
}

func describeDistinctFindings(findings []*Finding) string {
	out := ""
	for _, f := range findings {
		out += "{" + string(f.Category) + " " + f.Key + ": " + f.Title + "} "
	}
	return out
}
