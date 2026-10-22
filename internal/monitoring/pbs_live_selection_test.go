package monitoring

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/stretchr/testify/require"
)

// Real HTTP collector -> capped raw/cache publication -> guest backup age and
// stored protection. Exercise mixed cache/refresh at the actual 5,000-row cap,
// not a smaller substitute or a changed limit. No installed PBS/guest is used.
func exercisePBSLiveSelectionKeepsNewerRefreshAheadOfOlderCache(t *testing.T) {
	f := newPBSPosturePollFixture(t, 2)
	const instance = "pbs-main"
	const groups = pbsBackupLiveStateLimit/pbsBackupSnapshotsPerGroupLimit + 1
	var phase atomic.Int32
	var mu sync.Mutex
	reads := map[int]int{}
	readCount := func(id int) int { mu.Lock(); defer mu.Unlock(); return reads[id] }
	latest := func(id int, p int32) int64 {
		at := f.backupAt - int64(id-100)*3600
		if id == 100 && p > 0 {
			at += 3600
			if p >= 2 {
				at += 3600
			}
		}
		return at
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		p := phase.Load()
		switch {
		case strings.HasSuffix(r.URL.Path, "/groups"):
			var rows []pbs.BackupGroup
			if p != 5 {
				// Provider order is deliberately oldest first.
				for i := groups - 1; i >= 0; i-- {
					rows = append(rows, pbs.BackupGroup{BackupType: "vm", BackupID: strconv.Itoa(100 + i), LastBackup: latest(100+i, p), BackupCount: pbsBackupSnapshotsPerGroupLimit})
				}
			} else {
				rows = []pbs.BackupGroup{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			id, err := strconv.Atoi(r.URL.Query().Get("backup-id"))
			require.NoError(t, err)
			mu.Lock()
			reads[id]++
			mu.Unlock()
			if id == 100 {
				// Let cache hits and other workers finish before the newest read.
				time.Sleep(25 * time.Millisecond)
				if p == 2 || p == 3 {
					status := http.StatusServiceUnavailable
					if p == 3 {
						status = http.StatusForbidden
					}
					http.Error(w, "fixture unreadable", status)
					return
				}
			}
			var snapshots []map[string]any
			for j := pbsBackupSnapshotsPerGroupLimit - 1; j >= 0; j-- {
				size, verification := 1024, "ok"
				if id == 100 && p == 1 {
					size, verification = 4096, "failed"
				}
				snapshots = append(snapshots, map[string]any{"backup-type": "vm", "backup-id": strconv.Itoa(id), "backup-time": latest(id, p) - int64(j), "size": size, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": verification}, "comment": fmt.Sprintf("guest-%d", id)})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": snapshots})
		default:
			t.Errorf("unexpected read: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var err error
	f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	require.NoError(t, err)
	key := func(id int) pbsBackupGroupKey {
		return pbsBackupGroupKey{datastore: "archive", backupType: "vm", backupID: strconv.Itoa(id)}
	}
	frames := map[string]any{}
	poll := func(p int32, label string) []models.PBSBackup {
		phase.Store(p)
		f.m.pollPBSBackups(context.Background(), instance, f.client, []models.PBSDatastore{{Name: "archive"}})
		// This fixture ingests the actual 5,000-row window, rather than the
		// one/two-row inventories served by f.wait's ten-second helper. Wait
		// for completion, not a latency SLO; retain all artifact assertions.
		deadline := time.Now().Add(90 * time.Second)
		for {
			f.m.recoveryIngestMu.Lock()
			running := f.m.recoveryIngestRunning
			f.m.recoveryIngestMu.Unlock()
			if !running {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("large PBS selection ingest did not complete")
			}
			time.Sleep(5 * time.Millisecond)
		}
		rows := f.m.PBSBackupsSnapshot()
		mapped, mapErr := proxmoxmapper.FromPBSBackupsWithEvidence(rows, buildPBSGuestCandidates(f.m.GetUnifiedReadStateOrSnapshot()), time.Now())
		require.NoError(t, mapErr)
		require.Len(t, mapped, len(rows), "real bounded artifacts must not be synthesized or lost in recovery mapping")
		if label != "" {
			frames[label] = map[string]any{"backups": rows, "postures": f.postures(t), "vms": f.m.state.GetSnapshot().VMs}
		}
		require.Zero(t, readCount(100+groups-1), "groups below the selected window must not be fetched, including after cache hits")
		return rows
	}
	initial := poll(0, "initial")
	require.Len(t, initial, pbsBackupLiveStateLimit)
	require.Equal(t, "100", initial[0].VMID)
	targetAt, siblingAt := f.m.pbsBackupCacheTimeFor(instance, key(100)), f.m.pbsBackupCacheTimeFor(instance, key(101))
	require.False(t, targetAt.IsZero())
	require.Equal(t, initial, poll(0, ""), "pure cache reuse must preserve group and snapshot order")
	require.Equal(t, 1, readCount(100))
	require.Equal(t, targetAt, f.m.pbsBackupCacheTimeFor(instance, key(100)))

	changed := poll(1, "changed")
	require.Len(t, changed, pbsBackupLiveStateLimit)
	require.Equal(t, "100", changed[0].VMID, "slow newest refresh must precede older cache hits")
	require.Equal(t, latest(100, 1), changed[0].BackupTime.Unix())
	require.Equal(t, int64(4096), changed[0].Size)
	require.False(t, changed[0].Verified)
	require.Equal(t, initial[pbsBackupSnapshotsPerGroupLimit:], changed[pbsBackupSnapshotsPerGroupLimit:], "unchanged older groups retain their exact evidence")
	require.Equal(t, 2, readCount(100))
	changedAt := f.m.pbsBackupCacheTimeFor(instance, key(100))
	require.True(t, changedAt.After(targetAt))
	require.Equal(t, siblingAt, f.m.pbsBackupCacheTimeFor(instance, key(101)))
	points, total, err := f.store.ListPoints(context.Background(), recovery.ListPointsOptions{SubjectResourceID: f.ids[0], Limit: 1000})
	require.NoError(t, err)
	require.Equal(t, 200, total, "capped enumeration must not delete older durable target history")
	require.Len(t, points, total)
	require.Equal(t, latest(100, 1), points[0].StartedAt.Unix())

	held := poll(2, "transient")
	require.Equal(t, changed, held, "failed refresh must hold exact target rows in their reserved positions")
	require.Equal(t, changedAt, f.m.pbsBackupCacheTimeFor(instance, key(100)))
	denied := poll(3, "denied")
	require.Equal(t, changed[pbsBackupSnapshotsPerGroupLimit:], denied, "denied target is withdrawn without replacing it with an unselected older group")
	require.Equal(t, changedAt, f.m.pbsBackupCacheTimeFor(instance, key(100)))
	recovered := poll(4, "recovered")
	require.Len(t, recovered, pbsBackupLiveStateLimit)
	require.Equal(t, latest(100, 4), recovered[0].BackupTime.Unix())
	require.True(t, recovered[0].Verified)
	require.Equal(t, 1, readCount(101), "independent unchanged group must not be re-fetched")
	for _, p := range f.postures(t) {
		require.Len(t, p.ProviderStates, 1)
		require.Equal(t, recovery.ProtectionHistoryPartial, p.ProviderStates[0].HistoryCompleteness, "selected window is not complete provider history")
	}
	require.Equal(t, latest(100, 4), f.m.state.GetSnapshot().VMs[0].LastBackup.Unix(), "guest backup age must consume the newest retained refresh")
	require.Empty(t, poll(5, "empty"))
	_, total, err = f.store.ListPoints(context.Background(), recovery.ListPointsOptions{SubjectResourceID: f.ids[0], Limit: 1000})
	require.NoError(t, err)
	require.Zero(t, total, "ordinary authoritative emptiness still reconciles deleted points")
	require.True(t, f.m.pbsBackupCacheTimeFor(instance, key(100)).IsZero())
	require.Len(t, poll(4, "restored"), pbsBackupLiveStateLimit)

	// Full actual rows, losslessly compressed to stay within proof-output
	// bounds. The browser consumes every row, not a hand-selected subset.
	payload, err := json.Marshal(frames)
	require.NoError(t, err)
	var compressed bytes.Buffer
	zipper := gzip.NewWriter(&compressed)
	_, err = zipper.Write(payload)
	require.NoError(t, err)
	require.NoError(t, zipper.Close())
	t.Logf("PBS_LIVE_SELECTION_GZIP_BASE64=%s", base64.StdEncoding.EncodeToString(compressed.Bytes()))
}

// A snapshot read can outgrow the count that reserved its slots, even below
// the live cap. Do not overwrite a sibling, certify complete history, or reuse
// the clipped group while the provider's count is still stale.
func exercisePBSLiveSelectionGrowthDoesNotRenewTruncatedCache(t *testing.T) {
	f := newPBSPosturePollFixture(t, 2)
	var phase, targetReads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/groups") {
			rows := []pbs.BackupGroup{}
			for _, id := range []string{"100", "101"} {
				count := 1
				if id == "100" && phase.Load() == 3 {
					count = 2
				}
				rows = append(rows, pbs.BackupGroup{BackupType: "vm", BackupID: id, LastBackup: f.backupAt, BackupCount: count})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/snapshots") {
			id := r.URL.Query().Get("backup-id")
			count := 1
			if id == "100" {
				targetReads.Add(1)
				if phase.Load() == 4 {
					http.Error(w, "fixture unavailable during count decrease", http.StatusServiceUnavailable)
					return
				}
				if phase.Load() > 0 && phase.Load() != 5 {
					count = 2
				}
			}
			rows := []map[string]any{}
			for i := 0; i < count; i++ {
				rows = append(rows, map[string]any{"backup-type": "vm", "backup-id": id, "backup-time": f.backupAt - int64(i), "size": 1024, "files": []string{"index.json.blob"}})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
			return
		}
		t.Errorf("unexpected read: %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()
	var err error
	f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	require.NoError(t, err)
	poll := func(p int32) {
		phase.Store(p)
		f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, []models.PBSDatastore{{Name: "archive"}})
		f.wait(t)
	}
	poll(0)
	key := pbsBackupGroupKey{datastore: "archive", backupType: "vm", backupID: "100"}
	f.m.setPBSBackupCacheTime("pbs-main", key, time.Now().Add(-2*pbsBackupCacheTTL))
	poll(1)
	require.Len(t, f.m.PBSBackupsSnapshot(), 2, "growing target cannot exceed its reservation")
	require.True(t, f.m.pbsBackupCacheTimeFor("pbs-main", key).IsZero(), "truncated data is not reusable")
	for _, p := range f.postures(t) {
		require.Equal(t, recovery.ProtectionHistoryPartial, p.ProviderStates[0].HistoryCompleteness)
	}
	poll(2)
	require.Equal(t, int32(3), targetReads.Load(), "stale count must not turn clipped cache into a complete read")
	poll(3)
	require.Len(t, f.m.PBSBackupsSnapshot(), 3)
	require.False(t, f.m.pbsBackupCacheTimeFor("pbs-main", key).IsZero())
	poll(3)
	require.Equal(t, int32(4), targetReads.Load(), "complete ordinary recovery resumes bounded cache reuse")
	for _, p := range f.postures(t) {
		require.Equal(t, recovery.ProtectionHistoryComplete, p.ProviderStates[0].HistoryCompleteness)
	}
	completeAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
	poll(4)
	require.Len(t, f.m.PBSBackupsSnapshot(), 3, "transient count decrease must retain the known target rows when capacity permits")
	require.Equal(t, completeAt, f.m.pbsBackupCacheTimeFor("pbs-main", key))
	poll(5)
	require.Len(t, f.m.PBSBackupsSnapshot(), 2, "ordinary count-decreasing recovery replaces only its own actual rows")
	require.True(t, f.m.pbsBackupCacheTimeFor("pbs-main", key).After(completeAt))
}
