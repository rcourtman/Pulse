package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// The same archive pathname can exist on two independent local disks, e.g.
// after a guest and its backups move between nodes. Guest placement is not
// the archive's repository identity. This fixture only performs ordinary reads.
type localArchiveIdentityClient struct {
	*pveReconciliationClient
	kind        string
	storageType string
	shared      bool
}

func (c *localArchiveIdentityClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	rows, err := c.pveReconciliationClient.GetStorage(ctx, node)
	for i := range rows {
		rows[i].Storage = "local"
		rows[i].Type = c.storageType
		if c.shared {
			rows[i].Shared = 1
		}
	}
	return rows, err
}

func (c *localArchiveIdentityClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	rows, err := c.pveReconciliationClient.GetStorageContent(ctx, node, storage)
	for i := range rows {
		rows[i].VMID = 101
		rows[i].Volid = fmt.Sprintf("local:backup/vzdump-%s-101-2026_10_26-00_00_00.tar.zst", c.kind)
		rows[i].Format = "tar.zst"
		rows[i].Size = 1024
		if node == "node-b" && !c.shared && c.storageType == "dir" {
			rows[i].Size = 4096
		}
	}
	return rows, err
}

func localArchiveIdentityFixture(t *testing.T, kind string) (*pveReconciliationFixture, *localArchiveIdentityClient) {
	t.Helper()
	f := newPVEReconciliationFixture(t)
	f.m.state.UpdateVMs(nil)
	f.ids = f.ids[:1]
	if kind == "lxc" {
		f.m.state.UpdateContainers([]models.Container{{ID: makeGuestID("cluster", "node-b", 101), VMID: 101, Instance: "cluster", Node: "node-b", Name: "guest-101", Status: "running"}})
		f.ids[0] = unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "cluster", 101)
	} else {
		f.m.state.UpdateVMs([]models.VM{{ID: makeGuestID("cluster", "node-b", 101), VMID: 101, Instance: "cluster", Node: "node-b", Name: "guest-101", Status: "running"}})
	}
	return f, &localArchiveIdentityClient{pveReconciliationClient: f.client, kind: kind, storageType: "dir"}
}

func TestPVELocalArchiveIdentityLifecycle(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, reverse := range []bool{false, true} {
			for _, cold := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/reverse=%t/cold=%t", kind, reverse, cold), func(t *testing.T) {
					f, c := localArchiveIdentityFixture(t, kind)
					if reverse {
						f.nodes[0], f.nodes[1] = f.nodes[1], f.nodes[0]
					}
					poll := func() {
						f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
						f.wait(t)
					}
					poll()
					rows, points := f.m.state.GetSnapshot().PVEBackups.StorageBackups, f.points(t)
					if len(rows) != 2 || len(points) != 2 {
						t.Fatalf("independent same-path archives collapsed: live=%d durable=%d want2/2", len(rows), len(points))
					}
					before := map[string]models.StorageBackup{}
					for _, r := range rows {
						before[r.StorageNode] = r
						if r.Node != "node-b" {
							t.Fatalf("guest placement changed: %+v", r)
						}
						if p := points["pve-backup:"+r.ID]; p.SubjectResourceID != f.ids[0] || p.CompletedAt == nil || !p.CompletedAt.Equal(r.Time) {
							t.Fatalf("archive did not reach its canonical guest with exact date: %+v", p)
						}
					}
					if len(before) != 2 || before["node-a"].ID == before["node-b"].ID || before["node-a"].Size != 1024 || before["node-b"].Size != 4096 {
						t.Fatalf("repository identities or bytes merged: %+v", rows)
					}
					if cold {
						f.m.state.UpdateStorageBackupsForInstance("cluster", nil)
					}
					f.client.contentErr = fmt.Errorf("503 unavailable")
					f.client.at = f.client.at.Add(15 * time.Minute)
					for range 2 {
						poll()
						got := f.points(t)
						oldID := "pve-backup:" + before["node-b"].ID
						if len(got) != 2 || !reflect.DeepEqual(got[oldID], points[oldID]) {
							t.Fatal("unread same-path point was replaced or its evidence renewed by the peer")
						}
						f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
						raw := f.m.state.GetSnapshot().PVEBackups.StorageBackups
						want := 2
						if cold {
							want = 1
						}
						if len(raw) != want {
							t.Fatalf("partial live inventory=%d want%d", len(raw), want)
						}
						for _, r := range raw {
							if r.StorageNode == "node-b" && !reflect.DeepEqual(r, before["node-b"]) {
								t.Fatal("carried archive facts changed")
							}
							if r.StorageNode == "node-a" && (r.ID != before["node-a"].ID || !r.Time.Equal(f.client.at)) {
								t.Fatal("accepted peer lost stable identity or current observation")
							}
						}
					}
					f.client.contentErr = nil
					poll()
					if len(f.points(t)) != 2 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 2 {
						t.Fatal("complete recovery lost a repository")
					}
					// A change in guest placement must not rename either disk's archive.
					if kind == "lxc" {
						guests := f.m.state.GetSnapshot().Containers
						guests[0].Node = "node-a"
						f.m.state.UpdateContainers(guests)
					} else {
						guests := f.m.state.GetSnapshot().VMs
						guests[0].Node = "node-a"
						f.m.state.UpdateVMs(guests)
					}
					poll()
					for _, r := range f.m.state.GetSnapshot().PVEBackups.StorageBackups {
						if r.ID != before[r.StorageNode].ID || r.Node != "node-a" {
							t.Fatal("guest move changed archive identity or failed to update attribution")
						}
					}
					f.client.emptyB = true
					poll()
					if len(f.points(t)) != 1 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 1 {
						t.Fatal("accepted empty disk did not remove only its own archive")
					}
					if _, ok := f.points(t)["pve-backup:"+before["node-a"].ID]; !ok {
						t.Fatal("empty sibling erased the other disk")
					}
					f.client.emptyAll = true
					poll()
					if len(f.points(t)) != 0 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 0 {
						t.Fatal("complete absence did not clear inventory")
					}
				})
			}
		}
	}
}

func TestPVELocalArchiveSharedIdentityControls(t *testing.T) {
	for _, scope := range []string{"flag", "nfs", "cifs", "pbs"} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%t", scope, reverse), func(t *testing.T) {
				f, c := localArchiveIdentityFixture(t, "qemu")
				if scope == "flag" {
					c.shared = true
				} else {
					c.storageType = scope
				}
				if reverse {
					f.nodes[0], f.nodes[1] = f.nodes[1], f.nodes[0]
				}
				f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
				f.wait(t)
				rows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
				if len(rows) != 1 || len(f.points(t)) != 1 || rows[0].ID != "cluster-"+rows[0].Volid || rows[0].StorageNode != "" {
					t.Fatalf("shared deduplication/legacy identity changed: %+v", rows)
				}
			})
		}
	}
}

func TestPVELocalArchiveLegacyTransition(t *testing.T) {
	for _, cold := range []bool{false, true} {
		t.Run(fmt.Sprintf("cold=%t", cold), func(t *testing.T) {
			f, c := localArchiveIdentityFixture(t, "qemu")
			content, _ := c.GetStorageContent(context.Background(), "node-b", "local")
			old := models.StorageBackup{ID: "cluster-" + content[0].Volid, Instance: "cluster", Node: "node-b", StorageNode: "node-b", Storage: "local", Volid: content[0].Volid, VMID: 101, Type: "qemu", Time: f.client.at, CTime: f.client.at.Unix(), Size: 4096}
			legacy, err := proxmoxmapper.FromPVEStorageBackupsWithEvidence([]models.StorageBackup{old}, buildProxmoxGuestInfoIndex(f.m.backupReadStateForInstance("cluster")), f.client.at)
			if err != nil {
				t.Fatal(err)
			}
			other := old
			other.ID = "other-" + old.Volid
			other.Instance = "other"
			legacy = append(legacy, proxmoxmapper.FromPVEStorageBackups([]models.StorageBackup{other}, nil)...)
			f.m.ingestRecoveryPointsAsync(legacy)
			f.wait(t)
			original := f.points(t)
			if !cold {
				f.m.state.UpdateStorageBackupsForInstance("cluster", []models.StorageBackup{old})
			}
			f.client.contentErr = fmt.Errorf("503 unavailable")
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			got := f.points(t)
			if len(got) != 3 || !reflect.DeepEqual(got[legacy[0].ID], original[legacy[0].ID]) {
				t.Fatal("partial transition overwrote ambiguous legacy history instead of retaining it")
			}
			f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
			f.client.contentErr = nil
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			got = f.points(t)
			if len(got) != 3 {
				t.Fatalf("complete transition points=%d want two local plus other connection", len(got))
			}
			if _, exists := got[legacy[0].ID]; exists {
				t.Fatal("complete accepted scan retained obsolete unscoped identity")
			}
			if !reflect.DeepEqual(got[legacy[1].ID], original[legacy[1].ID]) {
				t.Fatal("transition modified another connection")
			}
		})
	}
}

// Run by the ordinary Go suite; only the export is optional evidence transport.
func TestPVELocalArchiveIdentityConsumerFrames(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		f, c := localArchiveIdentityFixture(t, kind)
		frames := map[string]any{}
		for _, phase := range []string{"observed", "partial", "recovered", "one-removed", "empty"} {
			switch phase {
			case "partial":
				f.client.contentErr = fmt.Errorf("503 unavailable")
			case "recovered":
				f.client.contentErr = nil
			case "one-removed":
				f.client.emptyB = true
			case "empty":
				f.client.emptyAll = true
			}
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			rows, points, postures := f.m.state.GetSnapshot().PVEBackups, f.points(t), f.postures(t)
			want := 2
			if phase == "one-removed" {
				want = 1
			}
			if phase == "empty" {
				want = 0
			}
			if len(rows.StorageBackups) != want || len(points) != want {
				t.Fatalf("phase=%s live=%d history=%d want%d", phase, len(rows.StorageBackups), len(points), want)
			}
			for _, p := range postures {
				if (phase == "observed" || phase == "recovered" || phase == "one-removed") && p.State != recovery.ProtectionStateProtected {
					t.Fatalf("accepted completed archives lost posture: %+v", p)
				}
				if (phase == "partial" || phase == "empty") && p.State == recovery.ProtectionStateProtected {
					t.Fatal("unread or absent archives claimed protection")
				}
			}
			frames[phase] = map[string]any{"pve": map[string]any{"data": rows}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points}
		}
		if os.Getenv("PULSE_LOCAL_ARCHIVE_FRAMES") == "1" {
			source, err := os.ReadFile("monitor_backups.go")
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(map[string]any{"kind": kind, "runtime_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "observed_at": time.Now().UTC(), "resource_ids": f.ids, "frames": frames})
			if err != nil {
				t.Fatal(err)
			}
			t.Log("LOCAL_ARCHIVE_FRAME=" + string(data))
		}
	}
}

func TestPVELocalArchiveIdentityTuple(t *testing.T) {
	// A black-box identity check: labels are not normalised, reordered or joined
	// ambiguously. Timestamp, size and the guest's placement do not name a disk.
	inputs := [][4]string{
		{"a-b", "c", "local", "volume"}, {"a", "b-c", "local", "volume"},
		{"a", "b", "c-local", "volume"}, {"A", "b", "c-local", "volume"},
		{"a", "B", "c-local", "volume"}, {"a", "b", "C-local", "volume"},
		{"a:b", "c", "local", "volume"}, {"a", "b:c", "local", "volume"},
		{"a", "b", "local:é", "volume"}, {"a", "b", "local:é", "Volume"},
		{"a", "b", "local", "é:volume"}, {"a", "b", "local:é", ":volume"},
	}
	seen := map[string]bool{}
	for _, in := range inputs {
		m := &Monitor{state: models.NewState()}
		c := &singleArchiveIdentityClient{storage: in[2], volume: in[3]}
		m.pollStorageBackupsWithNodes(context.Background(), in[0], c, []proxmox.Node{{Node: in[1]}}, map[string]string{in[1]: "online"})
		rows := m.state.GetSnapshot().PVEBackups.StorageBackups
		if len(rows) != 1 || seen[rows[0].ID] {
			t.Fatalf("exact repository tuple collided: input=%q rows=%+v", in, rows)
		}
		if !strings.HasPrefix(rows[0].ID, "pvelocal1:") {
			t.Errorf("local identity is not versioned separately from legacy shared IDs: %s", rows[0].ID)
		}
		seen[rows[0].ID] = true
	}
}

type singleArchiveIdentityClient struct {
	mockPVEClientExtra
	storage, volume string
}

func (c *singleArchiveIdentityClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return []proxmox.Storage{{Storage: c.storage, Type: "dir", Content: "backup", Active: 1, Enabled: 1}}, nil
}
func (c *singleArchiveIdentityClient) GetStorageContent(context.Context, string, string) ([]proxmox.StorageContent, error) {
	return []proxmox.StorageContent{{VMID: 101, Volid: c.volume, CTime: 1, Size: 1, Content: "backup"}}, nil
}
