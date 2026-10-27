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

// A positive VMID alone cannot turn an explicitly unclassified file into the
// current occupant's backup. Exercise real polling, live state, asynchronous
// ingestion and canonical posture, without a guest or backup operation.
func TestPVEUnknownArchiveKindLifecycle(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	f.nodes = f.nodes[:1]
	f.ids = []string{
		unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "cluster", 101),
		unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "cluster", 102),
		unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "other", 101),
	}
	f.m.state.UpdateVMs([]models.VM{
		{ID: makeGuestID("cluster", "node-b", 101), VMID: 101, Instance: "cluster", Node: "node-b", Name: "current-vm", Status: "running"},
		{ID: makeGuestID("other", "node-b", 101), VMID: 101, Instance: "other", Node: "node-b", Name: "current-vm", Status: "running"},
	})
	f.m.state.UpdateContainers([]models.Container{{ID: makeGuestID("cluster", "node-b", 102), VMID: 102, Instance: "cluster", Node: "node-b", Name: "current-lxc", Status: "running"}})
	unknown := []proxmox.StorageContent{
		{Volid: "qemu-archive:backup/opaque-101", VMID: 101, CTime: f.client.at.Unix(), Size: 2048, Content: "backup", Format: "tar.zst"},
		{Volid: "qemu-archive:backup/opaque-102", VMID: 102, CTime: f.client.at.Unix(), Size: 4096, Content: "backup", Format: "not-pbs-ct"},
	}
	typedAt := f.client.at.Add(-time.Hour)
	typed := []proxmox.StorageContent{
		{Volid: "qemu-archive:backup/vzdump-qemu-101.vma.zst", VMID: 101, CTime: typedAt.Unix(), Size: 8192, Content: "backup", Format: "vma.zst"},
		{Volid: "qemu-archive:backup/vzdump-lxc-102.tar.zst", VMID: 102, CTime: typedAt.Unix(), Size: 16384, Content: "backup", Format: "tar.zst"},
	}
	client := &backupTypeLifecycleClient{backupContentTypeClient: backupContentTypeClient{storage: "qemu-archive"}, rows: unknown}
	frames := map[string]any{}
	var accepted map[string]recovery.RecoveryPoint
	for _, phase := range []string{"unknown", "typed", "partial", "recovered", "unknown-again", "empty"} {
		switch phase {
		case "typed":
			client.rows = append(append([]proxmox.StorageContent(nil), unknown...), typed...)
		case "partial":
			client.err = fmt.Errorf("503 unavailable")
		case "recovered":
			client.err = nil
		case "unknown-again":
			client.rows = unknown
		case "empty":
			client.rows = nil
		}
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
		f.wait(t)
		snap, points, postures := f.m.state.GetSnapshot(), f.points(t), f.postures(t)
		rows := snap.PVEBackups.StorageBackups
		if len(rows) != len(client.rows) || len(points) != len(rows) {
			t.Fatalf("phase=%s lost artifacts: live=%d points=%d", phase, len(rows), len(points))
		}
		for _, row := range rows {
			p, ok := points["pve-backup:"+row.ID]
			if !ok || p.CompletedAt == nil || !p.CompletedAt.Equal(row.Time) || p.SizeBytes == nil || *p.SizeBytes != row.Size || p.Details["volid"] != row.Volid || p.Details["type"] != row.Type || row.StorageNode != "node-a" || row.InProgress {
				t.Fatalf("phase=%s archive facts changed: row=%+v point=%+v", phase, row, p)
			}
			if row.Type == "unknown" {
				if row.Node != "node-a" || p.SubjectResourceID != "" || p.SubjectRef != nil || p.Evidence != nil {
					t.Fatalf("phase=%s unknown archive borrowed occupant: row=%+v point=%+v", phase, row, p)
				}
			} else {
				i := row.VMID - 101
				if row.Node != "node-b" || p.SubjectResourceID != f.ids[i] || p.SubjectRef == nil || p.SubjectRef.Class != "node-b" {
					t.Fatalf("phase=%s typed attribution lost: row=%+v point=%+v", phase, row, p)
				}
			}
		}
		withTyped := phase == "typed" || phase == "partial" || phase == "recovered"
		want := time.Time{}
		if withTyped {
			want = typedAt
		}
		if !snap.VMs[0].LastBackup.Equal(want) || !snap.Containers[0].LastBackup.Equal(want) || !snap.VMs[1].LastBackup.IsZero() {
			t.Fatalf("phase=%s unknown file supplied/hid a canonical date: vms=%+v containers=%+v", phase, snap.VMs, snap.Containers)
		}
		for _, p := range postures {
			own := p.SubjectResourceID == f.ids[0] || p.SubjectResourceID == f.ids[1]
			if own && withTyped {
				if p.LastSuccessfulPointAt == nil || !p.LastSuccessfulPointAt.Equal(typedAt) || (phase != "partial" && p.State != recovery.ProtectionStateProtected) || (phase == "partial" && p.State == recovery.ProtectionStateProtected) {
					t.Fatalf("phase=%s wrong typed posture: %+v", phase, p)
				}
			} else if p.LastSuccessfulPointAt != nil || p.State == recovery.ProtectionStateProtected {
				t.Fatalf("phase=%s unknown file protected a guest: %+v", phase, p)
			}
		}
		if phase == "typed" {
			accepted = points
		}
		if phase == "partial" && !reflect.DeepEqual(accepted, points) {
			t.Fatal("unread inventory rewrote/renewed archive evidence")
		}
		frames[phase] = map[string]any{"pve": map[string]any{"data": snap.PVEBackups}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points, "vms": snap.VMs, "containers": snap.Containers}
	}
	if os.Getenv("PULSE_UNKNOWN_ARCHIVE_FRAMES") == "1" {
		hashes := map[string]string{}
		for _, path := range []string{"../models/models.go", "../recovery/mapper/proxmox/mapper.go", "monitor_backups.go"} {
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hashes[path] = fmt.Sprintf("%x", sha256.Sum256(b))
		}
		b, err := json.Marshal(map[string]any{"runtime_sha256": hashes, "resource_ids": f.ids, "frames": frames})
		if err != nil {
			t.Fatal(err)
		}
		t.Log("UNKNOWN_ARCHIVE_FRAMES=" + string(b))
	}
}

// Loss of an exact format discriminator must clear prior attribution even
// when the archive ID does not change. A successful reread may restore it.
func TestPVEUnknownArchiveKindWithdrawsPriorAttribution(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	f.nodes = f.nodes[:1]
	f.ids = f.ids[:1]
	client := &backupTypeLifecycleClient{backupContentTypeClient: backupContentTypeClient{storage: "local"}, rows: []proxmox.StorageContent{
		{Volid: "local:backup/opaque-101", VMID: 101, CTime: f.client.at.Unix(), Size: 2048, Content: "backup", Format: "pbs-vm"},
	}}
	var id string
	for _, format := range []string{"pbs-vm", "", "pbs-vm"} {
		client.rows[0].Format = format
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
		f.wait(t)
		rows, points := f.m.state.GetSnapshot().PVEBackups.StorageBackups, f.points(t)
		if len(rows) != 1 || len(points) != 1 {
			t.Fatal("same-ID attribution transition lost the archive")
		}
		if id == "" {
			id = rows[0].ID
		}
		if rows[0].ID != id {
			t.Fatal("kind recognition renamed the archive")
		}
		point := points["pve-backup:"+id]
		wantProtected := format != ""
		current := f.postures(t)[0]
		if wantProtected {
			if point.SubjectResourceID != f.ids[0] || current.State != recovery.ProtectionStateProtected {
				t.Fatalf("recognised format lost protection: point=%+v posture=%+v", point, current)
			}
		} else if point.SubjectResourceID != "" || current.State == recovery.ProtectionStateProtected || current.LastSuccessfulPointAt != nil || !f.m.state.GetSnapshot().VMs[0].LastBackup.IsZero() {
			t.Fatalf("unknown format retained prior attribution: point=%+v posture=%+v", point, current)
		}
		// Resource-ID reads re-evaluate. Also inspect the saved/list path so prior
		// protected rows cannot outlive the subject change there.
		saved, _, err := f.store.ListProtectionPostures(context.Background(), recovery.ProtectionPostureQuery{Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, posture := range saved {
			if posture.SubjectResourceID == f.ids[0] {
				found = true
				if !wantProtected && (posture.State == recovery.ProtectionStateProtected || posture.LastSuccessfulPointAt != nil) {
					t.Fatalf("saved posture retained prior subject: %+v", posture)
				}
			}
		}
		if wantProtected && !found {
			t.Fatal("recognised archive has no saved posture")
		}
	}
}
