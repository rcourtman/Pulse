package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// An artifact's encoded subject and its returned VMID must agree before the
// collector may lend that archive to a current workload. Keep conflicting raw
// evidence visible rather than guessing which independent field is correct.
func TestPVEArchiveSubjectRequiresAgreement(t *testing.T) {
	for _, tc := range []struct{ name, path, format string }{
		{"other-vm", "vzdump-qemu-102-2026_10_29.vma.zst", "vma.zst"},
		{"other-lxc", "vzdump-lxc-102-2026_10_29.tar.zst", "tar.zst"},
		{"bare-other-vm", "vzdump-qemu-102.vma.zst", "vma.zst"},
		{"pbs-other-vm", "vm/102/2026-10-29T00:00:00Z", "pbs-vm"},
		{"pbs-other-ct", "ct/102/2026-10-29T00:00:00Z", "pbs-ct"},
		{"empty-vzdump-id", "vzdump-qemu-.vma.zst", "pbs-vm"},
		{"negative-vzdump-id", "vzdump-qemu--101.vma.zst", "pbs-vm"},
		{"nonnumeric-vzdump-id", "vzdump-lxc-unknown.tar.zst", "pbs-ct"},
		{"numeric-prefix-not-id", "vzdump-qemu-101other.vma.zst", "pbs-vm"},
		{"overflow-vzdump-id", "vzdump-qemu-999999999999999999999999.vma.zst", "pbs-vm"},
		{"empty-pbs-id", "vm//2026-10-29T00:00:00Z", "pbs-vm"},
		{"negative-pbs-id", "ct/-101/2026-10-29T00:00:00Z", "pbs-ct"},
		{"zero-pbs-id", "vm/0/2026-10-29T00:00:00Z", "pbs-vm"},
		{"nonnumeric-pbs-id", "ct/101other/2026-10-29T00:00:00Z", "pbs-ct"},
	} {
		for _, prefix := range []string{"archive:backup/", "backup/", ""} {
			t.Run(tc.name+"/"+prefix, func(t *testing.T) {
				row := proxmox.StorageContent{Volid: prefix + tc.path, VMID: 101, Content: "backup", CTime: 1700000000, Size: 4096, Format: tc.format}
				client := &backupContentTypeClient{storage: "archive", row: row}
				m := &Monitor{state: models.NewState()}
				m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, []proxmox.Node{{Node: "node"}}, map[string]string{"node": "online"})
				rows := m.state.GetSnapshot().PVEBackups.StorageBackups
				if len(rows) != 1 || rows[0].Type != "unknown" {
					t.Fatalf("inconsistent archive gained a guest kind: %+v", rows)
				}
				got := rows[0]
				if got.Volid != row.Volid || got.VMID != row.VMID || got.Format != row.Format || got.CTime != row.CTime || got.Size != int64(row.Size) || got.Node != "node" || got.StorageNode != "node" || got.Storage != "archive" || got.Instance != "cluster" {
					t.Fatalf("raw archive facts changed: %+v", got)
				}
			})
		}
	}
}

func TestPVEArchiveSubjectCannotConfirmAnotherPBSGuest(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		t.Run(kind, func(t *testing.T) {
			at := time.Now().UTC().Truncate(time.Second)
			client := &backupContentTypeClient{storage: "pbs", storageType: "pbs", row: proxmox.StorageContent{Volid: fmt.Sprintf("pbs:backup/%s/102/%s", kind, at.Format(time.RFC3339)), VMID: 102, Content: "backup", CTime: at.Unix(), Size: 4096, Format: "pbs-" + kind}}
			m := &Monitor{state: models.NewState(), config: &config.Config{PBSInstances: []config.PBSInstance{{Name: "direct-pbs"}}}}
			for _, vmid := range []int{102, 101, 102} {
				client.row.VMID = vmid
				m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, []proxmox.Node{{Node: "node"}}, map[string]string{"node": "online"})
				confirmations := m.state.PBSGuestConfirmationsForInstance("cluster")
				want := []models.PBSGuestConfirmation{{Storage: "pbs", BackupType: kind, VMID: 102, Time: at.Unix()}}
				if vmid == 101 {
					want = nil
				}
				if len(confirmations) != len(want) || len(want) > 0 && !reflect.DeepEqual(confirmations, want) {
					t.Fatalf("metadata VMID=%d confirmations=%+v want=%+v", vmid, confirmations, want)
				}
				if len(m.state.GetSnapshot().PVEBackups.StorageBackups) != 0 {
					t.Fatal("direct PBS inventory exclusion changed")
				}
			}
		})
	}
}

func TestPVEArchiveSubjectAgreementLifecycle(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	f.nodes = f.nodes[:1]
	f.ids = append(f.ids, unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "cluster", 103), unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "other", 101))
	f.m.state.UpdateVMs([]models.VM{
		{ID: makeGuestID("cluster", "node-b", 101), VMID: 101, Instance: "cluster", Node: "node-b", Name: "original-vm", Status: "running"},
		{ID: makeGuestID("cluster", "node-b", 102), VMID: 102, Instance: "cluster", Node: "node-b", Name: "archive-vm", Status: "running"},
		{ID: makeGuestID("other", "node-b", 101), VMID: 101, Instance: "other", Node: "node-b", Name: "namesake-vm", Status: "running"},
	})
	f.m.state.UpdateContainers([]models.Container{{ID: makeGuestID("cluster", "node-b", 103), VMID: 103, Instance: "cluster", Node: "node-b", Name: "independent-lxc", Status: "running"}})
	at := f.client.at
	client := &backupTypeLifecycleClient{backupContentTypeClient: backupContentTypeClient{storage: "archive"}, rows: []proxmox.StorageContent{
		{Volid: "archive:backup/vzdump-qemu-102-2026_10_29.vma.zst", VMID: 101, Content: "backup", CTime: at.Unix(), Size: 4096, Format: "vma.zst"},
		{Volid: "archive:backup/vzdump-qemu-101-older.vma.zst", VMID: 101, Content: "backup", CTime: at.Add(-time.Hour).Unix(), Size: 2048, Format: "vma.zst"},
		{Volid: "archive:backup/vzdump-lxc-103-2026_10_29.tar.zst", VMID: 103, Content: "backup", CTime: at.Unix(), Size: 8192, Format: "tar.zst"},
	}}
	frames := map[string]any{}
	var accepted map[string]recovery.RecoveryPoint
	var archiveID string
	for _, phase := range []string{"conflict", "corrected", "unread", "conflict-again", "recovered", "empty"} {
		switch phase {
		case "corrected", "recovered":
			client.rows[0].VMID = 102
			client.err = nil
		case "unread":
			client.err = fmt.Errorf("503 unavailable")
		case "conflict-again":
			client.err = nil
			client.rows[0].VMID = 101
		case "empty":
			client.rows = nil
		}
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
		f.wait(t)
		snap, points, postures := f.m.state.GetSnapshot(), f.points(t), f.postures(t)
		empty := phase == "empty"
		conflict := phase == "conflict" || phase == "conflict-again"
		want := 3
		if empty {
			want = 0
		}
		if len(snap.PVEBackups.StorageBackups) != want || len(points) != want {
			t.Fatalf("phase=%s raw inventory lost: live=%d points=%d", phase, len(snap.PVEBackups.StorageBackups), len(points))
		}
		if !empty {
			row := snap.PVEBackups.StorageBackups[0]
			for _, r := range snap.PVEBackups.StorageBackups {
				if r.Volid == client.rows[0].Volid {
					row = r
				}
			}
			if archiveID == "" {
				archiveID = row.ID
			}
			if row.ID != archiveID {
				t.Fatal("metadata correction renamed the raw archive")
			}
			point := points["pve-backup:"+archiveID]
			if row.Volid != client.rows[0].Volid || row.CTime != at.Unix() || row.Size != 4096 || point.CompletedAt == nil || !point.CompletedAt.Equal(at) || point.Details["volid"] != row.Volid || row.StorageNode != "node-a" {
				t.Fatalf("phase=%s raw evidence changed: row=%+v point=%+v", phase, row, point)
			}
			if conflict {
				if row.Type != "unknown" || row.Node != "node-a" || point.SubjectResourceID != "" || point.SubjectRef != nil || point.Evidence != nil {
					t.Fatalf("phase=%s conflicting archive borrowed a guest: row=%+v point=%+v", phase, row, point)
				}
			} else if row.Type != "qemu" || row.Node != "node-b" || point.SubjectResourceID != f.ids[1] {
				t.Fatalf("phase=%s matching subject lost: row=%+v point=%+v", phase, row, point)
			}
		}
		expected := []time.Time{at.Add(-time.Hour), at, {}, at}
		if conflict {
			expected[1] = time.Time{}
		}
		if empty {
			expected = make([]time.Time, 4)
		}
		dates := []time.Time{snap.VMs[0].LastBackup, snap.VMs[1].LastBackup, snap.VMs[2].LastBackup, snap.Containers[0].LastBackup}
		for i := range dates {
			if !dates[i].Equal(expected[i]) {
				t.Errorf("phase=%s guest%d LastBackup=%v want=%v", phase, i, dates[i], expected[i])
			}
		}
		for _, posture := range postures {
			index := -1
			for i, id := range f.ids {
				if id == posture.SubjectResourceID {
					index = i
				}
			}
			if index < 0 {
				t.Fatal("unexpected subject")
			}
			datesByPosture := []time.Time{expected[0], expected[1], expected[3], expected[2]}
			expectedDate := datesByPosture[index]
			if expectedDate.IsZero() {
				if posture.LastSuccessfulPointAt != nil || posture.State == recovery.ProtectionStateProtected {
					t.Fatalf("phase=%s unsupported protected subject: %+v", phase, posture)
				}
			} else if posture.LastSuccessfulPointAt == nil || !posture.LastSuccessfulPointAt.Equal(expectedDate) || phase != "unread" && posture.State != recovery.ProtectionStateProtected || phase == "unread" && posture.State == recovery.ProtectionStateProtected {
				t.Fatalf("phase=%s wrong dated posture: %+v", phase, posture)
			}
		}
		if phase == "corrected" {
			accepted = points
		}
		if phase == "unread" && !reflect.DeepEqual(accepted, points) {
			t.Fatal("failed read renewed or rewrote the accepted evidence")
		}
		frames[phase] = map[string]any{"pve": map[string]any{"data": snap.PVEBackups}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points, "vms": snap.VMs, "containers": snap.Containers}
	}
	if os.Getenv("PULSE_ARCHIVE_SUBJECT_FRAMES") == "1" {
		b, err := os.ReadFile("monitor_backups.go")
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(map[string]any{"runtime_sha256": fmt.Sprintf("%x", sha256.Sum256(b)), "resource_ids": f.ids, "frames": frames})
		if err != nil {
			t.Fatal(err)
		}
		t.Log("ARCHIVE_SUBJECT_FRAMES=" + string(data))
	}
}
