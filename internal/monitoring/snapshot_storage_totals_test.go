package monitoring

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type snapshotTotalClient struct {
	fakeSnapshotClient
	failedStorage string
}

func (c snapshotTotalClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	if storage == c.failedStorage {
		return nil, errors.New("synthetic unread storage")
	}
	return c.fakeSnapshotClient.GetStorageContent(ctx, node, storage)
}

func TestSnapshotStorageTotalsRequireCompleteNode(t *testing.T) {
	const gib = uint64(1 << 30)
	for _, gap := range []string{"none", "unread", "inactive", "missing-size", "overflow"} {
		t.Run(gap, func(t *testing.T) {
			client := snapshotTotalClient{fakeSnapshotClient: fakeSnapshotClient{
				storages: map[string][]proxmox.Storage{
					"node": {{Storage: "fast", Content: "images", Active: 1, Enabled: 1}, {Storage: "remote", Content: "images", Active: 1, Enabled: 1}},
					"peer": {{Storage: "fast", Content: "images", Active: 1, Enabled: 1}},
				},
				contents: map[string]map[string][]proxmox.StorageContent{
					"node": {
						"fast":   {{Volid: "fast:vm-100-disk-0@weekly", Content: "images", VMID: 100, Size: 15 * gib}},
						"remote": {{Volid: "remote:vm-100-disk-1@weekly", Content: "images", VMID: 100, Size: 45 * gib}},
					},
					"peer": {"fast": {{Volid: "fast:vm-200-disk-0@weekly", Content: "images", VMID: 200, Size: 12 * gib}}},
				},
			}}
			switch gap {
			case "unread":
				client.failedStorage = "remote"
			case "inactive":
				client.storages["node"][1].Active = 0
			case "missing-size":
				client.contents["node"]["remote"][0].Size = 0
			case "overflow":
				client.contents["node"]["remote"][0].Size = 1<<63 - 1
			}
			snapshots := []models.GuestSnapshot{
				{ID: "site-node-100-weekly", Instance: "site", Node: "node", VMID: 100, Name: "weekly"},
				{ID: "site-peer-200-weekly", Instance: "site", Node: "peer", VMID: 200, Name: "weekly"},
			}
			totals := (&Monitor{}).collectSnapshotSizes(context.Background(), "site", client, snapshots)
			want := int64(0)
			if gap == "none" {
				want = 60 * int64(gib)
			}
			if totals[snapshots[0].ID] != want || totals[snapshots[1].ID] != 12*int64(gib) {
				t.Errorf("partial sum became a complete size or blocked its healthy peer: got %v, want node=%d peer=%d", totals, want, 12*gib)
			}
			t.Logf("SNAPSHOT_TOTAL_BASELINE gap=%s totals=%v", gap, totals)
		})
	}
}

func TestSnapshotSizingIgnoresNonVolumeRows(t *testing.T) {
	client := snapshotTotalClient{fakeSnapshotClient: fakeSnapshotClient{
		storages: map[string][]proxmox.Storage{"node": {{Storage: "mixed", Content: "backup,images,rootdir", Active: 1, Enabled: 1}}},
		contents: map[string]map[string][]proxmox.StorageContent{"node": {"mixed": {
			{Volid: "mixed:vm-100-disk-0@weekly", Content: "images", VMID: 100, Size: 1024},
			{Volid: "mixed:backup/100@weekly", Content: "backup", VMID: 100, Size: 8192},
		}}},
	}}
	snap := models.GuestSnapshot{ID: "site-node-100-weekly", Instance: "site", Node: "node", VMID: 100, Name: "weekly"}
	if got := (&Monitor{}).collectSnapshotSizes(context.Background(), "site", client, []models.GuestSnapshot{snap})[snap.ID]; got != 1024 {
		t.Fatal(fmt.Sprintf("non-volume content became snapshot size: got %d want 1024", got))
	}
}
