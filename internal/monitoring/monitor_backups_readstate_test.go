package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	pveapi "github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type canonicalBackupStorageClient struct {
	mockPVEClientExtra
}

func (c *canonicalBackupStorageClient) GetStorage(ctx context.Context, node string) ([]pveapi.Storage, error) {
	return []pveapi.Storage{
		{Storage: "local", Content: "backup", Type: "dir", Enabled: 1, Active: 1},
	}, nil
}

func (c *canonicalBackupStorageClient) GetStorageContent(ctx context.Context, node, storage string) ([]pveapi.StorageContent, error) {
	return []pveapi.StorageContent{
		{
			Volid:   "backup/vzdump-qemu-100-2026_03_11-10_00_00.vma.zst",
			VMID:    100,
			Size:    1024,
			CTime:   time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC).Unix(),
			Content: "backup",
		},
	}, nil
}

func backupReadStateResourceStore(resources []unifiedresources.Resource) *resourceOnlyStore {
	return &resourceOnlyStore{resources: resources}
}

func backupReadState(resources []unifiedresources.Resource) unifiedresources.ReadState {
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestResources(resources)
	return registry
}

func TestPopulateGuestNodeMapFromReadState_UsesCanonicalWorkloads(t *testing.T) {
	readState := backupReadState([]unifiedresources.Resource{
		{
			ID:     "vm-1",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "vm-100",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "node-from-store",
				VMID:     100,
			},
		},
		{
			ID:     "ct-1",
			Type:   unifiedresources.ResourceTypeSystemContainer,
			Name:   "ct-200",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "ct-node-from-store",
				VMID:     200,
			},
		},
	})

	guestNodeMap := map[int]string{}
	populateGuestNodeMapFromReadState(readState, "pve1", guestNodeMap)

	if guestNodeMap[100] != "node-from-store" {
		t.Fatalf("expected VM node from canonical read-state, got %q", guestNodeMap[100])
	}
	if guestNodeMap[200] != "ct-node-from-store" {
		t.Fatalf("expected container node from canonical read-state, got %q", guestNodeMap[200])
	}
}

func TestStorageNamesForNode_UsesCanonicalStoragePools(t *testing.T) {
	readState := backupReadState([]unifiedresources.Resource{
		{
			ID:     "storage-local",
			Type:   unifiedresources.ResourceTypeStorage,
			Name:   "local",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "node1",
			},
			Storage: &unifiedresources.StorageMeta{
				Content: "images,backup",
			},
		},
		{
			ID:     "storage-shared",
			Type:   unifiedresources.ResourceTypeStorage,
			Name:   "shared",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
			},
			Storage: &unifiedresources.StorageMeta{
				Content: "backup",
				Nodes:   []string{"node2", "node3"},
			},
		},
		{
			ID:     "storage-no-backup",
			Type:   unifiedresources.ResourceTypeStorage,
			Name:   "fast",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "node2",
			},
			Storage: &unifiedresources.StorageMeta{
				Content: "images",
			},
		},
	})

	got := storageNamesForNode(readState, "pve1", "node2")
	if len(got) != 1 || got[0] != "shared" {
		t.Fatalf("expected canonical backup storage names [shared], got %+v", got)
	}
}

func TestMonitorCalculateBackupOperationTimeout_UsesCanonicalReadState(t *testing.T) {
	resources := make([]unifiedresources.Resource, 0, 61)
	for i := 0; i < 61; i++ {
		resources = append(resources, unifiedresources.Resource{
			ID:     fmt.Sprintf("vm-%d", i),
			Type:   unifiedresources.ResourceTypeVM,
			Name:   fmt.Sprintf("vm-%d", i),
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "node1",
				VMID:     100 + i,
			},
		})
	}

	m := &Monitor{
		state:         models.NewState(),
		resourceStore: backupReadStateResourceStore(resources),
	}

	timeout := m.calculateBackupOperationTimeout("pve1")
	if want := 122 * time.Second; timeout != want {
		t.Fatalf("expected timeout %v from canonical workload count, got %v", want, timeout)
	}
}

func TestFetchPBSBackupSnapshotsUsesBoundedWorkerPool(t *testing.T) {
	t.Parallel()

	const requestCount = 40

	var active int64
	var maxActive int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/admin/datastore/archive/snapshots") {
			current := atomic.AddInt64(&active, 1)
			defer atomic.AddInt64(&active, -1)

			for {
				previous := atomic.LoadInt64(&maxActive)
				if current <= previous || atomic.CompareAndSwapInt64(&maxActive, previous, current) {
					break
				}
			}

			time.Sleep(10 * time.Millisecond)

			backupID := r.URL.Query().Get("backup-id")
			_, _ = w.Write([]byte(fmt.Sprintf(
				`{"data":[{"backup-type":"vm","backup-id":%q,"backup-time":1700000000}]}`,
				backupID,
			)))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("failed to create PBS client: %v", err)
	}

	requests := make([]pbsBackupFetchRequest, 0, requestCount)
	for i := 0; i < requestCount; i++ {
		backupID := strconv.Itoa(1000 + i)
		requests = append(requests, pbsBackupFetchRequest{
			datastore: "archive",
			group: pbs.BackupGroup{
				BackupType:  "vm",
				BackupID:    backupID,
				LastBackup:  1700000000,
				BackupCount: 1,
			},
		})
	}

	m := &Monitor{}
	backups := m.fetchPBSBackupSnapshots(context.Background(), client, "pbs1", requests)
	if len(backups) != requestCount {
		t.Fatalf("expected %d fetched backups, got %d", requestCount, len(backups))
	}

	if got := atomic.LoadInt64(&maxActive); got > int64(pbsBackupSnapshotFetchWorkers) {
		t.Fatalf("expected at most %d concurrent PBS snapshot fetches, saw %d", pbsBackupSnapshotFetchWorkers, got)
	}
}

// Regression test for issue #1541: groups larger than the per-group limit
// must still be fetched for real, keeping verification, size, file, and
// per-snapshot time data for the newest bounded set. RC3 summarized such
// groups into a single synthesized entry, which surfaced as "Unverified",
// "No size", "PBS files not listed", and a collapsed backup timeline.
func TestPollPBSBackupsFetchesRealSnapshotsForLargeGroups(t *testing.T) {
	t.Parallel()

	const firstBackupTime = int64(1700000000)
	snapshotCount := pbsBackupSnapshotsPerGroupLimit + 3

	var snapshots strings.Builder
	snapshots.WriteString(`{"data":[`)
	for i := 0; i < snapshotCount; i++ {
		if i > 0 {
			snapshots.WriteByte(',')
		}
		_, _ = fmt.Fprintf(
			&snapshots,
			`{"backup-type":"vm","backup-id":"100","backup-time":%d,"size":2048,"owner":"root@pam","files":[{"filename":"drive-scsi0.img.fidx"}],"verification":{"state":"ok","upid":"UPID:pbs1"}}`,
			firstBackupTime+int64(i),
		)
	}
	snapshots.WriteString(`]}`)
	snapshotsJSON := snapshots.String()

	var snapshotCalls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/admin/datastore/archive/groups"):
			_, _ = w.Write([]byte(fmt.Sprintf(
				`{"data":[{"backup-type":"vm","backup-id":"100","last-backup":%d,"backup-count":%d}]}`,
				firstBackupTime+int64(snapshotCount-1),
				snapshotCount,
			)))
		case strings.Contains(r.URL.Path, "/admin/datastore/archive/snapshots"):
			atomic.AddInt64(&snapshotCalls, 1)
			_, _ = w.Write([]byte(snapshotsJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("failed to create PBS client: %v", err)
	}

	m := &Monitor{state: models.NewState()}
	m.pollPBSBackups(context.Background(), "pbs1", client, []models.PBSDatastore{{Name: "archive"}})

	if got := atomic.LoadInt64(&snapshotCalls); got != 1 {
		t.Fatalf("large PBS backup group fetched snapshots %d times, want 1", got)
	}

	snapshot := m.state.GetSnapshot()
	if len(snapshot.PBSBackups) != pbsBackupSnapshotsPerGroupLimit {
		t.Fatalf("retained backups = %d, want newest %d real snapshots", len(snapshot.PBSBackups), pbsBackupSnapshotsPerGroupLimit)
	}
	seenTimes := make(map[int64]struct{}, len(snapshot.PBSBackups))
	for i, backup := range snapshot.PBSBackups {
		if !backup.Verified {
			t.Fatalf("backup %d lost verification state: %+v", i, backup)
		}
		if backup.Size != 2048 {
			t.Fatalf("backup %d lost size: %+v", i, backup)
		}
		if len(backup.Files) != 1 || backup.Files[0] != "drive-scsi0.img.fidx" {
			t.Fatalf("backup %d lost file list: %+v", i, backup)
		}
		seenTimes[backup.BackupTime.Unix()] = struct{}{}
	}
	if len(seenTimes) != pbsBackupSnapshotsPerGroupLimit {
		t.Fatalf("backup times collapsed: %d distinct times, want %d", len(seenTimes), pbsBackupSnapshotsPerGroupLimit)
	}
	newestTime := firstBackupTime + int64(snapshotCount-1)
	oldestRetainedTime := newestTime - int64(pbsBackupSnapshotsPerGroupLimit-1)
	for want := oldestRetainedTime; want <= newestTime; want++ {
		if _, ok := seenTimes[want]; !ok {
			t.Fatalf("expected newest snapshots retained, missing backup time %d", want)
		}
	}

	// A second poll must reuse the bounded cache instead of refetching.
	m.pollPBSBackups(context.Background(), "pbs1", client, []models.PBSDatastore{{Name: "archive"}})
	if got := atomic.LoadInt64(&snapshotCalls); got != 1 {
		t.Fatalf("second poll refetched snapshots (%d calls), want cached reuse", got)
	}
	if got := len(m.state.GetSnapshot().PBSBackups); got != pbsBackupSnapshotsPerGroupLimit {
		t.Fatalf("retained backups after cached poll = %d, want %d", got, pbsBackupSnapshotsPerGroupLimit)
	}
}

func TestPollPBSBackupsBoundsLargeTopologyAcrossPolls(t *testing.T) {
	t.Parallel()

	const firstBackupTime = int64(1700000000)
	groupCount := pbsBackupLiveStateLimit + 2000
	var groups strings.Builder
	groups.WriteString(`{"data":[`)
	for i := 0; i < groupCount; i++ {
		if i > 0 {
			groups.WriteByte(',')
		}
		_, _ = fmt.Fprintf(
			&groups,
			`{"backup-type":"vm","backup-id":"%d","last-backup":%d,"backup-count":1}`,
			i,
			firstBackupTime+int64(i),
		)
	}
	groups.WriteString(`]}`)
	groupsJSON := groups.String()

	var snapshotCalls int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/admin/datastore/archive/groups"):
			_, _ = w.Write([]byte(groupsJSON))
		case strings.Contains(r.URL.Path, "/admin/datastore/archive/snapshots"):
			atomic.AddInt64(&snapshotCalls, 1)
			backupID := r.URL.Query().Get("backup-id")
			id, err := strconv.Atoi(backupID)
			if err != nil {
				http.Error(w, "bad backup-id", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(
				`{"data":[{"backup-type":"vm","backup-id":%q,"backup-time":%d,"size":1024,"verification":{"state":"ok"}}]}`,
				backupID,
				firstBackupTime+int64(id),
			)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("failed to create PBS client: %v", err)
	}

	m := &Monitor{state: models.NewState()}
	for cycle := 0; cycle < 3; cycle++ {
		m.pollPBSBackups(context.Background(), "pbs1", client, []models.PBSDatastore{{Name: "archive"}})

		snapshot := m.state.GetSnapshot()
		if got := len(snapshot.PBSBackups); got != pbsBackupLiveStateLimit {
			t.Fatalf("cycle %d PBS backup state size = %d, want %d", cycle, got, pbsBackupLiveStateLimit)
		}
		// Snapshot fetches must stay bounded by the live-state limit: newest
		// groups are fetched once, groups beyond the limit are never fetched,
		// and later cycles reuse the bounded cache without refetching.
		if got := atomic.LoadInt64(&snapshotCalls); got != int64(pbsBackupLiveStateLimit) {
			t.Fatalf("cycle %d cumulative snapshot fetches = %d, want %d", cycle, got, pbsBackupLiveStateLimit)
		}
		times := make(map[int64]struct{}, len(snapshot.PBSBackups))
		for _, backup := range snapshot.PBSBackups {
			if !backup.Verified {
				t.Fatalf("cycle %d backup lost verification state: %+v", cycle, backup)
			}
			times[backup.BackupTime.Unix()] = struct{}{}
		}
		if len(times) != pbsBackupLiveStateLimit {
			t.Fatalf("cycle %d distinct backup times = %d, want %d", cycle, len(times), pbsBackupLiveStateLimit)
		}
		newestTime := firstBackupTime + int64(groupCount-1)
		oldestRetainedTime := firstBackupTime + int64(groupCount-pbsBackupLiveStateLimit)
		if _, ok := times[newestTime]; !ok {
			t.Fatalf("cycle %d missing newest backup time %d", cycle, newestTime)
		}
		if _, ok := times[oldestRetainedTime]; !ok {
			t.Fatalf("cycle %d missing oldest retained backup time %d", cycle, oldestRetainedTime)
		}
		if _, ok := times[oldestRetainedTime-1]; ok {
			t.Fatalf("cycle %d retained backup older than the live-state window: %d", cycle, oldestRetainedTime-1)
		}
	}
}

func TestConvertPBSSnapshotsKeepsOnlyRecentBoundedSnapshots(t *testing.T) {
	t.Parallel()

	snapshots := make([]pbs.BackupSnapshot, 0, pbsBackupSnapshotsPerGroupLimit+3)
	for i := 0; i < pbsBackupSnapshotsPerGroupLimit+3; i++ {
		snapshots = append(snapshots, pbs.BackupSnapshot{
			BackupType: "vm",
			BackupID:   "100",
			BackupTime: int64(1700000000 + i),
		})
	}

	backups := convertPBSSnapshots("pbs1", "archive", "", snapshots)
	if len(backups) != pbsBackupSnapshotsPerGroupLimit {
		t.Fatalf("converted backups = %d, want %d", len(backups), pbsBackupSnapshotsPerGroupLimit)
	}
	if got, want := backups[0].BackupTime.Unix(), int64(1700000000+pbsBackupSnapshotsPerGroupLimit+2); got != want {
		t.Fatalf("first backup time = %d, want newest %d", got, want)
	}
	for i := 1; i < len(backups); i++ {
		if backups[i].BackupTime.After(backups[i-1].BackupTime) {
			t.Fatalf("backups not sorted newest first at %d: %s after %s", i, backups[i].BackupTime, backups[i-1].BackupTime)
		}
	}
}

func TestBuildPBSBackupCacheKeepsNewestPerGroup(t *testing.T) {
	t.Parallel()

	backups := make([]models.PBSBackup, 0, pbsBackupSnapshotsPerGroupLimit+3)
	for i := 0; i < pbsBackupSnapshotsPerGroupLimit+3; i++ {
		backupTime := time.Unix(int64(1700000000+i), 0)
		backups = append(backups, models.PBSBackup{
			ID:         fmt.Sprintf("pbs-pbs1-archive-vm-100-%d", backupTime.Unix()),
			Instance:   "pbs1",
			Datastore:  "archive",
			BackupType: "vm",
			VMID:       "100",
			BackupTime: backupTime,
		})
	}

	state := models.NewState()
	state.UpdatePBSBackups("pbs1", backups)
	m := &Monitor{state: state}

	entry := m.buildPBSBackupCache("pbs1")[pbsBackupGroupKey{
		datastore:  "archive",
		backupType: "vm",
		backupID:   "100",
	}]

	if len(entry.snapshots) != pbsBackupSnapshotsPerGroupLimit {
		t.Fatalf("cached snapshots = %d, want %d", len(entry.snapshots), pbsBackupSnapshotsPerGroupLimit)
	}
	if got, want := entry.snapshots[0].BackupTime.Unix(), int64(1700000000+pbsBackupSnapshotsPerGroupLimit+2); got != want {
		t.Fatalf("first cached backup time = %d, want newest %d", got, want)
	}
	for i := 1; i < len(entry.snapshots); i++ {
		if entry.snapshots[i].BackupTime.After(entry.snapshots[i-1].BackupTime) {
			t.Fatalf("cache snapshots not sorted newest first at %d: %s after %s", i, entry.snapshots[i].BackupTime, entry.snapshots[i-1].BackupTime)
		}
	}
}

func TestPollPBSBackupsPrunesCacheTimesForDeletedGroups(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/admin/datastore/archive/groups") {
			_, _ = w.Write([]byte(`{"data":[{"backup-type":"vm","backup-id":"100","last-backup":1700000000,"backup-count":1}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("failed to create PBS client: %v", err)
	}

	keepKey := pbsBackupGroupKey{
		datastore:  "archive",
		namespace:  "",
		backupType: "vm",
		backupID:   "100",
	}
	staleKey := pbsBackupGroupKey{
		datastore:  "archive",
		namespace:  "",
		backupType: "vm",
		backupID:   "999",
	}

	m := &Monitor{
		state: models.NewState(),
		pbsBackupCacheTime: map[string]map[pbsBackupGroupKey]time.Time{
			"pbs1": {
				keepKey:  time.Now(),
				staleKey: time.Now(),
			},
		},
	}
	m.state.UpdatePBSBackups("pbs1", []models.PBSBackup{
		{
			ID:         "pbs-pbs1-archive--vm-100-1700000000",
			Instance:   "pbs1",
			Datastore:  "archive",
			Namespace:  "",
			BackupType: "vm",
			VMID:       "100",
			BackupTime: time.Unix(1700000000, 0),
		},
		{
			ID:         "pbs-pbs1-archive--vm-999-1690000000",
			Instance:   "pbs1",
			Datastore:  "archive",
			Namespace:  "",
			BackupType: "vm",
			VMID:       "999",
			BackupTime: time.Unix(1690000000, 0),
		},
	})

	m.pollPBSBackups(context.Background(), "pbs1", client, []models.PBSDatastore{
		{Name: "archive"},
	})

	m.mu.RLock()
	perGroup := m.pbsBackupCacheTime["pbs1"]
	_, kept := perGroup[keepKey]
	_, stale := perGroup[staleKey]
	m.mu.RUnlock()

	if !kept {
		t.Fatal("expected current PBS group cache time to be retained")
	}
	if stale {
		t.Fatal("expected deleted PBS group cache time to be pruned")
	}

	snapshot := m.state.GetSnapshot()
	for _, backup := range snapshot.PBSBackups {
		if backup.Instance == "pbs1" && backup.VMID == "999" {
			t.Fatalf("expected stale backup to be removed with cache time, found: %+v", backup)
		}
	}
}

func TestMonitorPollGuestSnapshots_UsesCanonicalReadState(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		resourceStore: backupReadStateResourceStore([]unifiedresources.Resource{
			{
				ID:     "vm-store-100",
				Type:   unifiedresources.ResourceTypeVM,
				Name:   "vm100",
				Status: unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{
					Instance: "pve1",
					NodeName: "node1",
					VMID:     100,
				},
			},
			{
				ID:     "ct-store-200",
				Type:   unifiedresources.ResourceTypeSystemContainer,
				Name:   "ct200",
				Status: unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{
					Instance: "pve1",
					NodeName: "node1",
					VMID:     200,
				},
			},
		}),
	}

	client := &mockPVEClientSnapshots{
		snapshots: []pveapi.Snapshot{{Name: "snap1", SnapTime: 1234567890, Description: "from store"}},
	}

	m.pollGuestSnapshots(context.Background(), "pve1", client)

	snapshot := m.state.GetSnapshot()
	if len(snapshot.PVEBackups.GuestSnapshots) != 2 {
		t.Fatalf("expected guest snapshots from canonical workloads, got %+v", snapshot.PVEBackups.GuestSnapshots)
	}
}

func TestMonitorPollGuestSnapshots_RefreshesStaleCanonicalStoreForClusterGuest(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	state := models.NewState()
	state.UpdateVMsForInstance("homelab", []models.VM{{
		ID:       "homelab-pve-a-100",
		VMID:     100,
		Name:     "prod-vm",
		Node:     "pve-a",
		Instance: "homelab",
		Status:   "running",
		LastSeen: now,
	}})

	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	m := &Monitor{
		state:         state,
		resourceStore: adapter,
	}

	client := &backupStorageTimeoutSnapshotClient{
		snapshots: []pveapi.Snapshot{{
			Name:        "cluster-snap",
			SnapTime:    now.Unix(),
			Description: "from fresh clustered guest state",
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	m.pollGuestSnapshots(ctx, "homelab", client)

	if client.snapshotCalls == 0 {
		t.Fatal("expected guest snapshot polling to use fresh clustered guest state")
	}

	snapshot := state.GetSnapshot()
	if len(snapshot.PVEBackups.GuestSnapshots) != 1 {
		t.Fatalf("expected one guest snapshot from fresh clustered guest state, got %+v", snapshot.PVEBackups.GuestSnapshots)
	}
	if got := snapshot.PVEBackups.GuestSnapshots[0]; got.Name != "cluster-snap" || got.Node != "pve-a" || got.Instance != "homelab" {
		t.Fatalf("unexpected guest snapshot: %+v", got)
	}

	if vms := adapter.VMs(); len(vms) != 1 || vms[0].Instance() != "homelab" || vms[0].Node() != "pve-a" {
		t.Fatalf("expected canonical store to refresh from fresh clustered guest state, got %+v", vms)
	}
}

func TestMonitorPollStorageBackupsWithNodes_UsesCanonicalReadStateForGuestNodeLookup(t *testing.T) {
	m := &Monitor{
		state: models.NewState(),
		resourceStore: backupReadStateResourceStore([]unifiedresources.Resource{
			{
				ID:     "vm-store-100",
				Type:   unifiedresources.ResourceTypeVM,
				Name:   "vm100",
				Status: unifiedresources.StatusOnline,
				Proxmox: &unifiedresources.ProxmoxData{
					Instance: "pve1",
					NodeName: "node2",
					VMID:     100,
				},
			},
		}),
	}

	client := &canonicalBackupStorageClient{}
	nodes := []pveapi.Node{{Node: "node1", Status: "online"}}

	m.pollStorageBackupsWithNodes(context.Background(), "pve1", client, nodes, map[string]string{"node1": "online"})

	backups := m.state.GetSnapshot().PVEBackups.StorageBackups
	if len(backups) != 1 {
		t.Fatalf("expected one storage backup, got %+v", backups)
	}
	if backups[0].Node != "node2" {
		t.Fatalf("expected guest node from canonical read-state, got %q", backups[0].Node)
	}
}

func TestSyncGuestBackupTimesAndResourceStore_RefreshesCanonicalWorkloads(t *testing.T) {
	stale := time.Date(2026, 1, 10, 2, 0, 0, 0, time.UTC)
	fresh := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)

	state := models.NewState()
	state.UpdateVMsForInstance("homelab", []models.VM{{
		ID:         "homelab-minipc-vm-100",
		VMID:       100,
		Name:       "docker",
		Node:       "minipc",
		Instance:   "homelab",
		Status:     "running",
		LastBackup: stale,
		LastSeen:   fresh,
	}})
	state.UpdatePBSBackups("pbs-docker", []models.PBSBackup{{
		ID:         "pbs-docker/store/minipc/vm/100/2026-03-11T10:00:00Z",
		Instance:   "pbs-docker",
		Datastore:  "store",
		Namespace:  "minipc",
		BackupType: "vm",
		VMID:       "100",
		BackupTime: fresh,
		Comment:    "docker",
	}})

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(state.GetSnapshot())
	adapter := unifiedresources.NewMonitorAdapter(registry)
	m := &Monitor{
		state:         state,
		resourceStore: adapter,
	}

	m.syncGuestBackupTimesAndResourceStore()

	snapshot := state.GetSnapshot()
	if len(snapshot.VMs) != 1 || !snapshot.VMs[0].LastBackup.Equal(fresh) {
		t.Fatalf("expected state VM last backup %v, got %+v", fresh, snapshot.VMs)
	}

	vms := adapter.VMs()
	if len(vms) != 1 {
		t.Fatalf("expected one canonical VM, got %d", len(vms))
	}
	if got := vms[0].LastBackup(); !got.Equal(fresh) {
		t.Fatalf("expected canonical VM last backup %v, got %v", fresh, got)
	}
}

func TestSyncGuestBackupTimesAndResourceStore_ClearsCanonicalStaleBackup(t *testing.T) {
	stale := time.Date(2026, 1, 10, 2, 0, 0, 0, time.UTC)

	state := models.NewState()
	state.UpdateVMsForInstance("homelab", []models.VM{{
		ID:         "homelab-minipc-vm-100",
		VMID:       100,
		Name:       "docker",
		Node:       "minipc",
		Instance:   "homelab",
		Status:     "running",
		LastBackup: stale,
		LastSeen:   stale,
	}})

	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(state.GetSnapshot())
	adapter := unifiedresources.NewMonitorAdapter(registry)
	m := &Monitor{
		state:         state,
		resourceStore: adapter,
	}

	m.syncGuestBackupTimesAndResourceStore()

	snapshot := state.GetSnapshot()
	if len(snapshot.VMs) != 1 || !snapshot.VMs[0].LastBackup.IsZero() {
		t.Fatalf("expected state VM last backup to clear, got %+v", snapshot.VMs)
	}

	vms := adapter.VMs()
	if len(vms) != 1 {
		t.Fatalf("expected one canonical VM, got %d", len(vms))
	}
	if got := vms[0].LastBackup(); !got.IsZero() {
		t.Fatalf("expected canonical VM last backup to clear, got %v", got)
	}
}

func TestBuildPBSGuestCandidates_UsesCanonicalReadState(t *testing.T) {
	readState := backupReadState([]unifiedresources.Resource{
		{
			ID:     "vm-store-100",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "vm100",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "nodeA",
				VMID:     100,
			},
		},
		{
			ID:     "ct-store-200",
			Type:   unifiedresources.ResourceTypeSystemContainer,
			Name:   "ct200",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "nodeB",
				VMID:     200,
			},
		},
	})

	candidates := buildPBSGuestCandidates(readState)

	assertCandidate := func(key string, resourceType unifiedresources.ResourceType, node string, vmid int) {
		t.Helper()
		entries := candidates[key]
		if len(entries) != 1 {
			t.Fatalf("expected one candidate for %s, got %+v", key, entries)
		}
		if entries[0] != (proxmoxmapper.GuestCandidate{
			SourceID:     fmt.Sprintf("%s-store-%d", map[unifiedresources.ResourceType]string{unifiedresources.ResourceTypeVM: "vm", unifiedresources.ResourceTypeSystemContainer: "ct"}[resourceType], vmid),
			ResourceType: resourceType,
			DisplayName:  fmt.Sprintf("%s%d", map[unifiedresources.ResourceType]string{unifiedresources.ResourceTypeVM: "vm", unifiedresources.ResourceTypeSystemContainer: "ct"}[resourceType], vmid),
			InstanceName: "pve1",
			NodeName:     node,
			VMID:         vmid,
		}) {
			t.Fatalf("unexpected candidate for %s: %+v", key, entries[0])
		}
	}

	assertCandidate("vm:100", unifiedresources.ResourceTypeVM, "nodeA", 100)
	assertCandidate("ct:200", unifiedresources.ResourceTypeSystemContainer, "nodeB", 200)
}

func TestBuildProxmoxGuestInfoIndex_UsesCanonicalReadState(t *testing.T) {
	readState := backupReadState([]unifiedresources.Resource{
		{
			ID:     "vm-store-100",
			Type:   unifiedresources.ResourceTypeVM,
			Name:   "vm100",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "nodeA",
				VMID:     100,
			},
		},
		{
			ID:     "ct-store-200",
			Type:   unifiedresources.ResourceTypeSystemContainer,
			Name:   "ct200",
			Status: unifiedresources.StatusOnline,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "pve1",
				NodeName: "nodeB",
				VMID:     200,
			},
		},
	})

	index := buildProxmoxGuestInfoIndex(readState)

	assertInfo := func(key string, resourceType unifiedresources.ResourceType, name string) {
		t.Helper()
		info, ok := index[key]
		if !ok {
			t.Fatalf("expected info for %s, got none", key)
		}
		if info.ResourceType != resourceType {
			t.Errorf("expected type %v, got %v", resourceType, info.ResourceType)
		}
		if info.Name != name {
			t.Errorf("expected name %q, got %q", name, info.Name)
		}
	}

	assertInfo("pve1|nodeA|100", unifiedresources.ResourceTypeVM, "vm100")
	assertInfo("pve1|nodeB|200", unifiedresources.ResourceTypeSystemContainer, "ct200")
}
