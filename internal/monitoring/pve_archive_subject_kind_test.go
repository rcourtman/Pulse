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

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// The provider may retain a removed VM's archives after its VMID is reused by
// an LXC (or the reverse). The artifact kind, not the new occupant, owns them.
// This drives ordinary read-only collection, ingest and posture APIs; no guest
// agent or backup operation is involved.
func TestPVEArchiveSubjectKindLifecycle(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	f.nodes = f.nodes[:1]
	ctID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "cluster", 101)
	vmID := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "cluster", 102)
	f.ids = []string{ctID, vmID}
	f.m.state.UpdateVMs([]models.VM{{ID: makeGuestID("cluster", "node-b", 102), VMID: 102, Instance: "cluster", Node: "node-b", Name: "current-vm", Status: "running"}})
	f.m.state.UpdateContainers([]models.Container{{ID: makeGuestID("cluster", "node-b", 101), VMID: 101, Instance: "cluster", Node: "node-b", Name: "current-lxc", Status: "running"}})
	oldAt := f.client.at
	rightAt := oldAt.Add(-time.Hour)
	rows := []proxmox.StorageContent{
		{Volid: "local:backup/vzdump-qemu-101.vma.zst", VMID: 101, CTime: oldAt.Unix(), Size: 2048, Content: "backup", Format: "vma.zst"},
		{Volid: "local:backup/vzdump-lxc-102.tar.zst", VMID: 102, CTime: oldAt.Unix(), Size: 1024, Content: "backup", Format: "tar.zst"},
	}
	client := &backupTypeLifecycleClient{backupContentTypeClient: backupContentTypeClient{storage: "local"}, rows: rows}
	phases := []string{"mismatched", "matched", "partial", "recovered", "empty"}
	frames := map[string]any{}
	var accepted map[string]recovery.RecoveryPoint
	for _, phase := range phases {
		switch phase {
		case "matched":
			client.rows = append(client.rows,
				proxmox.StorageContent{Volid: "local:backup/vzdump-lxc-101.tar.zst", VMID: 101, CTime: rightAt.Unix(), Size: 4096, Content: "backup", Format: "tar.zst"},
				proxmox.StorageContent{Volid: "local:backup/vzdump-qemu-102.vma.zst", VMID: 102, CTime: rightAt.Unix(), Size: 8192, Content: "backup", Format: "vma.zst"})
		case "partial":
			client.err = fmt.Errorf("503 unavailable")
		case "recovered":
			client.err = nil
		case "empty":
			client.rows = nil
		}
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
		f.wait(t)
		snapshot := f.m.state.GetSnapshot()
		points := f.points(t)
		postures := f.postures(t)
		want := rightAt
		if phase == "mismatched" || phase == "empty" {
			want = time.Time{}
		}
		for _, got := range []time.Time{snapshot.VMs[0].LastBackup, snapshot.Containers[0].LastBackup} {
			if !got.Equal(want) {
				t.Errorf("phase=%s guest LastBackup=%s want=%s: opposite-kind archive contaminated current occupant", phase, got, want)
			}
		}
		for _, row := range snapshot.PVEBackups.StorageBackups {
			point, ok := points["pve-backup:"+row.ID]
			if !ok {
				t.Fatalf("phase=%s missing archive point %s", phase, row.ID)
			}
			kind := unifiedresources.ResourceTypeVM
			if row.Type == "lxc" {
				kind = unifiedresources.ResourceTypeSystemContainer
			}
			expected := unifiedresources.ProxmoxGuestCanonicalID(kind, "cluster", row.VMID)
			if point.SubjectResourceID != expected {
				t.Errorf("phase=%s %s point subject=%s want=%s", phase, row.Type, point.SubjectResourceID, expected)
			}
			if point.CompletedAt == nil || !point.CompletedAt.Equal(row.Time) {
				t.Errorf("phase=%s lost archive date: %+v", phase, point)
			}
			wrong := (row.VMID == 101 && row.Type == "qemu") || (row.VMID == 102 && row.Type == "lxc")
			wantNode := "node-b"
			if wrong {
				wantNode = "node-a"
			}
			if row.Node != wantNode || row.StorageNode != "node-a" || point.SubjectRef == nil || point.SubjectRef.Class != wantNode {
				t.Errorf("phase=%s archive borrowed an opposite-kind placement or lost read origin: row=%+v point=%+v", phase, row, point)
			}
			if wrong && point.SubjectRef != nil && (point.SubjectRef.Name == "current-lxc" || point.SubjectRef.Name == "current-vm") {
				t.Errorf("phase=%s historical archive borrowed current occupant name: %+v", phase, point.SubjectRef)
			}
		}
		for _, p := range postures {
			if phase == "mismatched" || phase == "empty" || phase == "partial" {
				if p.State == recovery.ProtectionStateProtected {
					t.Errorf("phase=%s current occupant falsely protected: %+v", phase, p)
				}
			} else if p.State != recovery.ProtectionStateProtected || p.LastSuccessfulPointAt == nil || !p.LastSuccessfulPointAt.Equal(rightAt) {
				t.Errorf("phase=%s actual same-kind archive not selected: %+v", phase, p)
			}
		}
		if phase == "matched" {
			accepted = points
		}
		if phase == "partial" && !reflect.DeepEqual(accepted, points) {
			t.Error("unread inventory rewrote or renewed archive points")
		}
		if phase == "empty" && len(points) != 0 {
			t.Errorf("accepted absence retains %d archive points", len(points))
		}
		frames[phase] = map[string]any{"pve": map[string]any{"data": snapshot.PVEBackups}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points, "vms": snapshot.VMs, "containers": snapshot.Containers}
	}
	if os.Getenv("PULSE_ARCHIVE_SUBJECT_FRAMES") == "1" {
		hashes := map[string]string{}
		for _, p := range []string{"../models/models.go", "../recovery/mapper/proxmox/mapper.go", "monitor_backups.go"} {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			hashes[p] = fmt.Sprintf("%x", sha256.Sum256(b))
		}
		data, err := json.Marshal(map[string]any{"runtime_sha256": hashes, "resource_ids": f.ids, "frames": frames})
		if err != nil {
			t.Fatal(err)
		}
		t.Log("ARCHIVE_SUBJECT_FRAMES=" + string(data))
	}
}
