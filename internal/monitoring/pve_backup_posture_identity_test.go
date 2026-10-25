package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type postureIdentityClient struct {
	PVEClientInterface
	at time.Time
}

func (c *postureIdentityClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return []proxmox.Storage{{Storage: "local", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}, nil
}
func (c *postureIdentityClient) GetStorageContent(_ context.Context, _, _ string) ([]proxmox.StorageContent, error) {
	var out []proxmox.StorageContent
	for _, id := range []int{101, 102} {
		at := c.at.Add(-time.Duration(id-101) * time.Hour)
		out = append(out, proxmox.StorageContent{Volid: fmt.Sprintf("local:backup/vzdump-lxc-%d-%d.tar.zst", id, at.Unix()), VMID: id, CTime: at.Unix(), Size: 2048, Content: "backup", Format: "tar.zst"})
	}
	return out, nil
}

// Imported resource views can retain the retired node-scoped ID. Rollup keys
// already recognise that identity; canonical posture must read the same points.
func TestPVEBackupPostureRecognisesLegacySubjectIdentity(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			m, manager := recoveryIngestTestMonitor(t)
			m.state = models.NewState()
			store, err := manager.StoreForOrg("default")
			if err != nil {
				t.Fatal(err)
			}
			f := &pveReconciliationFixture{m: m, store: store}
			t.Cleanup(func() { f.wait(t); _ = store.Close() })
			now := time.Now().UTC().Truncate(time.Second)
			client := &postureIdentityClient{at: now.Add(-time.Hour)}
			var resources []unifiedresources.Resource
			var containers []models.Container
			for _, instance := range []string{"estate-a", "estate-b"} {
				for _, id := range []int{101, 102} {
					source := makeGuestID(instance, "node", id)
					canonical := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, instance, id)
					rid := canonical
					if legacy {
						rid = unifiedresources.SourceSpecificID(unifiedresources.ResourceTypeSystemContainer, unifiedresources.SourceProxmox, source)
					}
					f.ids = append(f.ids, canonical)
					containers = append(containers, models.Container{ID: source, Instance: instance, Node: "node", VMID: id, Name: fmt.Sprintf("guest-%d", id), Status: "running"})
					resources = append(resources, unifiedresources.Resource{ID: rid, Type: unifiedresources.ResourceTypeSystemContainer, Name: fmt.Sprintf("guest-%d", id), Status: unifiedresources.StatusOnline, Proxmox: &unifiedresources.ProxmoxData{Instance: instance, NodeName: "node", VMID: id, SourceID: source}})
				}
			}
			m.state.UpdateContainers(containers)
			m.resourceStore = backupReadStateResourceStore(resources)
			for _, instance := range []string{"estate-a", "estate-b"} {
				m.pollStorageBackupsWithNodes(context.Background(), instance, client, []proxmox.Node{{Node: "node"}}, map[string]string{"node": "online"})
			}
			f.wait(t)
			if len(m.state.GetSnapshot().PVEBackups.StorageBackups) != 4 {
				t.Fatal("fresh inventory lost a guest")
			}
			points := f.points(t)
			if len(points) != 4 {
				t.Fatalf("points=%d want 4", len(points))
			}
			for _, p := range f.postures(t) {
				if p.State != recovery.ProtectionStateProtected || p.LastSuccessfulPointAt == nil || p.LastSuccessfulPointAt.Before(now.Add(-2*time.Hour)) {
					t.Errorf("fresh inventory did not advance canonical subject %s: state=%s success=%v", p.SubjectResourceID, p.State, p.LastSuccessfulPointAt)
				}
			}

		})
	}
}
