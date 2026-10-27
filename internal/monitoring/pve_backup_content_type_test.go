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

type backupContentTypeClient struct {
	PVEClientInterface
	storage     string
	row         proxmox.StorageContent
	storageType string
}

func (c *backupContentTypeClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	kind := c.storageType
	if kind == "" {
		kind = "dir"
	}
	return []proxmox.Storage{{Storage: c.storage, Type: kind, Content: "backup", Active: 1, Enabled: 1}}, nil
}

func (c *backupContentTypeClient) GetStorageContent(context.Context, string, string) ([]proxmox.StorageContent, error) {
	return []proxmox.StorageContent{c.row}, nil
}

// A repository label or incidental filename text cannot name a guest kind.
// Exercise the actual poller rather than a second copy of its classifier.
func TestPVEStorageBackupTypeUsesArtifactDiscriminator(t *testing.T) {
	for _, tc := range []struct {
		name, storage, volume, format, want string
	}{
		{"lxc-on-qemu-named-disk", "qemu-archive", "qemu-archive:backup/vzdump-lxc-101-2026_10_27.tar.zst", "tar.zst", "lxc"},
		{"qemu-on-lxc-named-disk", "lxc-archive", "lxc-archive:backup/vzdump-qemu-101-2026_10_27.vma.zst", "vma.zst", "qemu"},
		{"lxc-with-qemu-in-suffix", "local", "local:backup/vzdump-lxc-101-qemu.tar.zst", "tar.zst", "lxc"},
		{"qemu-with-lxc-in-suffix", "local", "local:backup/vzdump-qemu-101-lxc.vma.zst", "vma.zst", "qemu"},
		{"prefixless-lxc", "local", "backup/vzdump-lxc-101.tar.zst", "tar.zst", "lxc"},
		{"prefixless-qemu", "local", "backup/vzdump-qemu-101.vma.zst", "vma.zst", "qemu"},
		{"bare-lxc", "local", "vzdump-lxc-101.tar.zst", "tar.zst", "lxc"},
		{"bare-qemu", "local", "vzdump-qemu-101.vma.zst", "vma.zst", "qemu"},
		{"pbs-ct-on-qemu-named-disk", "qemu-pbs", "qemu-pbs:backup/ct/101/2026-10-27T00:00:00Z", "pbs-ct", "lxc"},
		{"pbs-vm-on-lxc-named-disk", "lxc-pbs", "lxc-pbs:backup/vm/101/2026-10-27T00:00:00Z", "pbs-vm", "qemu"},
		{"pbs-ct-no-format", "qemu-pbs", "qemu-pbs:backup/ct/101/2026-10-27T00:00:00Z", "", "lxc"},
		{"pbs-vm-no-format", "lxc-pbs", "lxc-pbs:backup/vm/101/2026-10-27T00:00:00Z", "", "qemu"},
		{"bare-pbs-ct", "qemu-pbs", "qemu-pbs:ct/101/2026-10-27T00:00:00Z", "", "lxc"},
		{"bare-pbs-vm", "lxc-pbs", "lxc-pbs:vm/101/2026-10-27T00:00:00Z", "", "qemu"},
		{"prefixless-pbs-ct", "qemu-pbs", "backup/ct/101/2026-10-27T00:00:00Z", "", "lxc"},
		{"prefixless-pbs-vm", "lxc-pbs", "backup/vm/101/2026-10-27T00:00:00Z", "", "qemu"},
		{"format-only-ct", "qemu-pbs", "qemu-pbs:backup/opaque", "pbs-ct", "lxc"},
		{"format-only-vm", "lxc-pbs", "lxc-pbs:backup/opaque", "pbs-vm", "qemu"},
		{"qemu-label-not-evidence", "qemu-archive", "qemu-archive:backup/opaque", "", "unknown"},
		{"lxc-label-not-evidence", "lxc-archive", "lxc-archive:backup/opaque", "", "unknown"},
		{"qemu-text-not-evidence", "local", "local:backup/notes-qemu-101", "", "unknown"},
		{"lxc-text-not-evidence", "local", "local:backup/notes-lxc-101", "", "unknown"},
		{"nested-vm-text-not-evidence", "local", "local:backup/notes/vm/101", "", "unknown"},
		{"nested-ct-text-not-evidence", "local", "local:backup/notes/ct/101", "", "unknown"},
		{"pbs-format-substring-not-evidence", "local", "local:backup/opaque", "not-pbs-ct", "unknown"},
		{"unrecognised-format", "local", "local:backup/opaque", "tar.zst", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
			c := &backupContentTypeClient{storage: tc.storage, row: proxmox.StorageContent{
				Volid: tc.volume, VMID: 101, Content: "backup", CTime: at.Unix(), Format: tc.format, Size: 4096,
			}}
			m := &Monitor{state: models.NewState()}
			m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, []proxmox.Node{{Node: "node"}}, map[string]string{"node": "online"})
			rows := m.state.GetSnapshot().PVEBackups.StorageBackups
			if len(rows) != 1 {
				t.Fatalf("live archives=%d want1", len(rows))
			}
			r := rows[0]
			if r.Type != tc.want {
				t.Errorf("archive type=%q want%q; volume=%q format=%q", r.Type, tc.want, r.Volid, r.Format)
			}
			if r.Volid != tc.volume || r.Format != tc.format || r.Storage != tc.storage || !r.Time.Equal(at) || r.CTime != at.Unix() || r.Size != 4096 || r.VMID != 101 || r.Instance != "cluster" || r.Node != "node" || r.StorageNode != "node" || r.IsPBS || r.InProgress {
				t.Fatalf("classification changed independent archive facts: %+v", r)
			}
		})
	}
}

func TestPVEStorageBackupTypeKeepsHostAndNonBackupAdmission(t *testing.T) {
	for _, kind := range []string{"backup", "vztmpl", "iso"} {
		t.Run(kind, func(t *testing.T) {
			c := &backupContentTypeClient{storage: "qemu-lxc-archive", row: proxmox.StorageContent{
				Volid: "qemu-lxc-archive:backup/vzdump-lxc-101.tar.zst", Content: kind, CTime: 1, Size: 1, Format: "pbs-vm",
			}}
			m := &Monitor{state: models.NewState()}
			m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, []proxmox.Node{{Node: "node"}}, map[string]string{"node": "online"})
			rows := m.state.GetSnapshot().PVEBackups.StorageBackups
			if kind != "backup" {
				if len(rows) != 0 {
					t.Fatal("non-backup admission changed")
				}
			} else if len(rows) != 1 || rows[0].Type != "host" {
				t.Fatalf("VMID-zero host fallback changed: %+v", rows)
			}
		})
	}
}

func TestPVEStorageBackupTypeKeepsTypedPBSConfirmations(t *testing.T) {
	for _, kind := range []string{"ct", "vm"} {
		t.Run(kind, func(t *testing.T) {
			at := time.Now().UTC().Truncate(time.Second)
			c := &backupContentTypeClient{storage: "qemu-lxc-pbs", storageType: "pbs", row: proxmox.StorageContent{
				Volid: fmt.Sprintf("qemu-lxc-pbs:backup/%s/101/%s", kind, at.Format(time.RFC3339)), VMID: 101, Content: "backup", CTime: at.Unix(), Size: 4096, Format: "pbs-" + kind,
			}}
			m := &Monitor{state: models.NewState(), config: &config.Config{PBSInstances: []config.PBSInstance{{Name: "direct-pbs"}}}}
			for _, instance := range []string{"estate-a", "estate-b"} {
				m.pollStorageBackupsWithNodes(context.Background(), instance, c, []proxmox.Node{{Node: "node"}}, map[string]string{"node": "online"})
				confirmations := m.state.PBSGuestConfirmationsForInstance(instance)
				want := []models.PBSGuestConfirmation{{Storage: c.storage, BackupType: kind, VMID: 101, Time: at.Unix()}}
				if !reflect.DeepEqual(confirmations, want) || len(m.state.GetSnapshot().PVEBackups.StorageBackups) != 0 {
					t.Fatalf("direct PBS exclusion/typed confirmation changed: instance=%s confirmations=%+v", instance, confirmations)
				}
			}
		})
	}
}

type backupTypeLifecycleClient struct {
	backupContentTypeClient
	rows []proxmox.StorageContent
	err  error
}

func (c *backupTypeLifecycleClient) GetStorageContent(context.Context, string, string) ([]proxmox.StorageContent, error) {
	return c.rows, c.err
}

// This normal-suite lifecycle follows classification through live state,
// canonical dates, recovery ingestion and provider posture. Optional output
// transports those exact responses to the consuming Backups browser proof.
func TestPVEStorageBackupTypeConsumerLifecycle(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	f.nodes = f.nodes[:1]
	f.ids[0] = unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "cluster", 101)
	f.m.state.UpdateVMs([]models.VM{{ID: makeGuestID("cluster", "node-a", 102), VMID: 102, Instance: "cluster", Node: "node-a", Name: "build-vm", Status: "running"}})
	f.m.state.UpdateContainers([]models.Container{{ID: makeGuestID("cluster", "node-a", 101), VMID: 101, Instance: "cluster", Node: "node-a", Name: "media-lxc", Status: "running"}})
	c := &backupTypeLifecycleClient{backupContentTypeClient: backupContentTypeClient{storage: "qemu-archive"}}
	for i, kind := range []string{"lxc", "qemu"} {
		c.rows = append(c.rows, proxmox.StorageContent{Volid: fmt.Sprintf("qemu-archive:backup/vzdump-%s-%d-2026_10_27.tar.zst", kind, 101+i), VMID: 101 + i, Content: "backup", CTime: f.client.at.Add(-time.Duration(i) * time.Hour).Unix(), Size: uint64((i + 1) * 4096), Format: "tar.zst"})
	}
	frames := map[string]any{}
	var original map[string]recovery.RecoveryPoint
	var initialRows []models.StorageBackup
	for _, phase := range []string{"observed", "partial", "recovered", "empty"} {
		switch phase {
		case "partial":
			c.err = fmt.Errorf("503 unavailable")
		case "recovered":
			c.err = nil
		case "empty":
			c.rows = nil
		}
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
		f.wait(t)
		f.m.syncGuestBackupTimesAndResourceStore()
		snapshot, points := f.m.state.GetSnapshot(), f.points(t)
		rows := snapshot.PVEBackups.StorageBackups
		want := 2
		if phase == "empty" {
			want = 0
		}
		if len(rows) != want || len(points) != want {
			t.Fatalf("phase=%s live=%d points=%d want%d", phase, len(rows), len(points), want)
		}
		for _, r := range rows {
			index, kind := r.VMID-101, "lxc"
			if index == 1 {
				kind = "qemu"
			}
			p := points["pve-backup:"+r.ID]
			if r.Type != kind || p.Details["type"] != kind || p.SubjectResourceID != f.ids[index] || p.CompletedAt == nil || !p.CompletedAt.Equal(r.Time) {
				t.Fatalf("phase=%s archive kind/subject/date did not reach recovery: row=%+v point=%+v", phase, r, p)
			}
		}
		if phase == "observed" {
			original, initialRows = points, rows
		} else if phase == "partial" {
			if !reflect.DeepEqual(rows, initialRows) || !reflect.DeepEqual(points, original) {
				t.Fatal("partial read changed archive facts or renewed point evidence")
			}
		} else if phase == "recovered" {
			if !reflect.DeepEqual(rows, initialRows) {
				t.Fatal("successful reread changed identity or archive facts")
			}
		}
		if phase == "empty" {
			if !snapshot.VMs[0].LastBackup.IsZero() || !snapshot.Containers[0].LastBackup.IsZero() {
				t.Fatal("confirmed absence retained canonical dates")
			}
		} else if !snapshot.Containers[0].LastBackup.Equal(f.client.at) || !snapshot.VMs[0].LastBackup.Equal(f.client.at.Add(-time.Hour)) {
			t.Fatal("guest kind change lost or contaminated canonical backup dates")
		}
		postures := f.postures(t)
		for _, p := range postures {
			if phase == "partial" || phase == "empty" {
				if p.State == recovery.ProtectionStateProtected {
					t.Fatal("incomplete or absent read claimed protection")
				}
			} else if p.State != recovery.ProtectionStateProtected {
				t.Fatalf("known completed archive lost posture: %+v", p)
			}
		}
		frames[phase] = map[string]any{"pve": map[string]any{"data": snapshot.PVEBackups}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points, "vms": snapshot.VMs, "containers": snapshot.Containers}
	}
	if os.Getenv("PULSE_BACKUP_TYPE_FRAMES") == "1" {
		source, err := os.ReadFile("monitor_backups.go")
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(map[string]any{"runtime_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "resource_ids": f.ids, "frames": frames})
		if err != nil {
			t.Fatal(err)
		}
		t.Log("BACKUP_TYPE_FRAMES=" + string(data))
	}
}
