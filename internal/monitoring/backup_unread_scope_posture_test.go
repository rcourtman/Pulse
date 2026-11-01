package monitoring

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// One enabled but unreachable PBS storage on one node, beside ordinary
// readable archive storage on every node: the shape that turned every guest on
// a production connection to attention.
type pveDeadOffsiteStorageClient struct {
	*pveReconciliationClient
	offsiteErr error
}

const deadOffsiteStorage = "pbs-energy-offsite"

func (c *pveDeadOffsiteStorageClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	rows, err := c.pveReconciliationClient.GetStorage(ctx, node)
	if err == nil && node == "node-a" {
		rows = append(rows, proxmox.Storage{Storage: deadOffsiteStorage, Type: "pbs", Content: "backup", Shared: 1, Active: 1, Enabled: 1})
	}
	return rows, err
}

func (c *pveDeadOffsiteStorageClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	if storage == deadOffsiteStorage {
		if c.offsiteErr != nil {
			return nil, c.offsiteErr
		}
		return nil, nil
	}
	return c.pveReconciliationClient.GetStorageContent(ctx, node, storage)
}

func TestPVEUnreadStorageLeavesReadBackupsProtected(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	c := &pveDeadOffsiteStorageClient{
		pveReconciliationClient: f.client,
		offsiteErr:              fmt.Errorf("pbs-energy-offsite: error fetching datastores - 127.0.0.1:18007: connection refused"),
	}
	poll := func() {
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
		f.wait(t)
	}
	wantUnread := []recovery.ProtectionUnreadScope{{
		Kind: recovery.ProtectionUnreadScopeStorage, Node: "node-a", Storage: deadOffsiteStorage,
	}}
	assertNamed := func(p recovery.ProtectionPosture) {
		t.Helper()
		if len(p.ProviderStates) != 1 ||
			p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial ||
			!reflect.DeepEqual(p.ProviderStates[0].UnreadScopes, wantUnread) {
			t.Errorf("provider state lost the unread storage: %+v", p.ProviderStates)
		}
		if !strings.Contains(p.Explanation, "could not read "+deadOffsiteStorage+" on node-a") {
			t.Errorf("explanation does not name the unread storage: %q", p.Explanation)
		}
	}

	// Fresh backups on storages that read fine are protected; the dead
	// storage limits coverage and is named, but proves nothing about them.
	poll()
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateProtected || p.Coverage != recovery.ProtectionCoveragePartial ||
			p.Freshness != recovery.ProtectionFreshnessCurrent {
			t.Errorf("read backup was not protected beside one unread storage: %+v", p)
		}
		assertNamed(p)
	}

	// Node-b's archive is pruned while the offsite storage is still dead. The
	// partial poll cannot delete the stored point, but it did not re-read it,
	// so that retained point no longer supports protection.
	retained := f.points(t)
	f.client.at = f.client.at.Add(15 * time.Minute)
	f.client.emptyB = true
	poll()
	points := f.points(t)
	if len(points) != len(retained) {
		t.Fatalf("partial poll deleted history: points=%d want %d", len(points), len(retained))
	}
	for _, p := range f.postures(t) {
		switch p.SubjectResourceID {
		case f.ids[0]:
			if p.State != recovery.ProtectionStateProtected {
				t.Errorf("re-read backup lost protection: %+v", p)
			}
		case f.ids[1]:
			if p.State == recovery.ProtectionStateProtected || p.Coverage != recovery.ProtectionCoveragePartial {
				t.Errorf("retained point that the latest poll did not read claimed protection: %+v", p)
			}
		}
		assertNamed(p)
	}

	// Once the storage reads again the collection is complete and reconciles
	// the pruned archive away; the unread names disappear with the gap.
	c.offsiteErr = nil
	poll()
	for _, p := range f.postures(t) {
		if len(p.ProviderStates) > 0 && len(p.ProviderStates[0].UnreadScopes) != 0 {
			t.Errorf("complete collection kept unread scopes: %+v", p.ProviderStates)
		}
		if p.SubjectResourceID == f.ids[0] &&
			(p.State != recovery.ProtectionStateProtected || p.Coverage != recovery.ProtectionCoverageComplete) {
			t.Errorf("complete read did not restore full protection: %+v", p)
		}
		if p.SubjectResourceID == f.ids[1] && p.State == recovery.ProtectionStateProtected {
			t.Errorf("confirmed-absent archive still protected its guest: %+v", p)
		}
	}
}

func TestPVEUnreadProtectionScopesNameWhatWentUnread(t *testing.T) {
	got := pveUnreadProtectionScopes(pveUnreadReads{
		nodes: map[string]struct{}{"node-c": {}},
		local: map[pveLocalBackupStorage]struct{}{{node: "node-b", storage: "local"}: {}},
		shared: map[string]struct{}{
			"offsite": {},
			"nfs":     {},
		},
		sharedReads: map[pveLocalBackupStorage]struct{}{
			{node: "node-a", storage: "offsite"}: {},
			{node: "node-a", storage: "nfs"}:     {},
			{node: "node-b", storage: "nfs"}:     {},
		},
		denied: map[pveLocalBackupStorage]struct{}{
			{node: "node-c"}:                     {},
			{node: "node-b", storage: "nfs"}:     {},
			{node: "node-a", storage: "unknown"}: {},
		},
	})
	want := []recovery.ProtectionUnreadScope{
		{Kind: recovery.ProtectionUnreadScopeNode, Node: "node-c", AccessDenied: true},
		{Kind: recovery.ProtectionUnreadScopeStorage, Node: "node-b", Storage: "local"},
		// Several nodes failed the same shared storage: name the storage only.
		{Kind: recovery.ProtectionUnreadScopeStorage, Storage: "nfs", AccessDenied: true},
		{Kind: recovery.ProtectionUnreadScopeStorage, Node: "node-a", Storage: "offsite"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unread scopes = %+v\nwant %+v", got, want)
	}
}
