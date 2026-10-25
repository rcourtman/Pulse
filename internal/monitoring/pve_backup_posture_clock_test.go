package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type postureClockClient struct {
	PVEClientInterface
	times map[string]time.Time
}

func (c *postureClockClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return []proxmox.Storage{{Storage: "local", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}, nil
}

func (c *postureClockClient) GetStorageContent(_ context.Context, node, _ string) ([]proxmox.StorageContent, error) {
	id := 101
	if node == "node-b" {
		id = 102
	}
	return []proxmox.StorageContent{{Volid: fmt.Sprintf("local:backup/vzdump-qemu-%d.vma.zst", id), VMID: id, CTime: c.times[node].Unix(), Size: 2048, Content: "backup", Format: "vma.zst"}}, nil
}

func TestPVEBackupPostureFutureInventory(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	future, recent := now.Add(48*time.Hour), now.Add(-time.Hour)
	client := &postureClockClient{times: map[string]time.Time{"node-a": future, "node-b": recent}}
	poll := func() {
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", client, f.nodes, f.online)
		f.wait(t)
	}
	poll()
	for _, posture := range f.postures(t) {
		if posture.SubjectResourceID == f.ids[0] {
			if posture.State != recovery.ProtectionStateAttention || posture.Freshness != recovery.ProtectionFreshnessUnknown || !posture.LastSuccessfulPointAt.Equal(future) {
				t.Errorf("future provider date claimed protection or lost its evidence: %+v", posture)
			}
		} else if posture.State != recovery.ProtectionStateProtected {
			t.Errorf("independent current backup lost protection: %+v", posture)
		}
	}
	for _, state := range []recovery.ProtectionState{recovery.ProtectionStateAttention, recovery.ProtectionStateProtected} {
		postures, total, err := f.store.ListProtectionPostures(context.Background(), recovery.ProtectionPostureQuery{State: state})
		if err != nil || total != 1 || len(postures) != 1 || (state == recovery.ProtectionStateAttention && postures[0].SubjectResourceID != f.ids[0]) {
			t.Errorf("state-filtered %s read: total=%d len=%d err=%v", state, total, len(postures), err)
		}
	}
	if len(f.points(t)) != 2 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 2 {
		t.Fatal("uncertain clock discarded provider artifacts")
	}
	client.times["node-a"] = recent
	poll()
	for _, posture := range f.postures(t) {
		if posture.State != recovery.ProtectionStateProtected || !posture.LastSuccessfulPointAt.Equal(recent) {
			t.Errorf("normal provider clock did not recover: %+v", posture)
		}
	}
}
