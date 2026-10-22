package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func exercisePBSGroupLatestChange(t *testing.T, kind, namespace string, direction int64, failureStatus int) {
	t.Helper()
	f := newPBSPosturePollFixture(t, 2, kind)
	const instance = "pbs-main"
	changedAt := f.backupAt + direction*3600
	var phase atomic.Int32
	var mu sync.Mutex
	reads := map[string]int{}
	count := func(id string) int { mu.Lock(); defer mu.Unlock(); return reads[id] }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := phase.Load()
		if r.URL.Query().Get("ns") != namespace {
			t.Errorf("unexpected namespace: %q", r.URL.Query().Get("ns"))
		}
		backupAt := func(id string) int64 {
			if id != "100" || p == 0 {
				return f.backupAt
			}
			if p == 4 {
				return f.backupAt + 7200
			}
			return changedAt
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/groups"):
			groups := []map[string]any{}
			for _, id := range []string{"100", "101"} {
				if id == "100" && p == 3 {
					continue
				}
				groups = append(groups, map[string]any{"backup-type": kind, "backup-id": id, "backup-count": 1, "last-backup": backupAt(id)})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": groups})
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			id := r.URL.Query().Get("backup-id")
			mu.Lock()
			reads[id]++
			mu.Unlock()
			if id == "100" && p == 1 && failureStatus != 0 {
				http.Error(w, "fixture unreadable", failureStatus)
				return
			}
			size, verification := 1024, "ok"
			if id == "100" && (p == 1 || p == 2) {
				size, verification = 2048, "failed"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"backup-type": kind, "backup-id": id, "backup-time": backupAt(id), "size": size, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": verification}, "comment": "guest-" + id}}})
		default:
			t.Errorf("unexpected read: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	var err error
	f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ds := []models.PBSDatastore{{Name: "archive", Namespaces: []models.PBSNamespace{{Path: namespace}}}}
	poll := func(p int32) {
		phase.Store(p)
		f.m.pollPBSBackups(context.Background(), instance, f.client, ds)
		f.wait(t)
	}
	project := func(label string) {
		// Only one representative journey is emitted for consuming browser proof.
		if kind != "vm" || namespace != "team/production" || direction != -1 || failureStatus != 0 {
			return
		}
		payload, err := json.Marshal(map[string]any{"backups": f.m.state.GetSnapshot().PBSBackups, "postures": f.postures(t), "now": time.Now().UTC()})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PBS_LATEST_PROJECTION_%s=%s", label, payload)
	}
	key := pbsBackupGroupKey{datastore: "archive", namespace: namespace, backupType: kind, backupID: "100"}
	sibling := pbsBackupGroupKey{datastore: "archive", namespace: namespace, backupType: kind, backupID: "101"}
	row := func(id string) *models.PBSBackup {
		for _, b := range f.m.state.GetSnapshot().PBSBackups {
			if b.VMID == id {
				return &b
			}
		}
		return nil
	}
	assertChanged := func() {
		t.Helper()
		b := row("100")
		if b == nil || b.BackupTime.Unix() != changedAt || b.Size != 2048 || b.Verified {
			t.Fatalf("changed group retained deleted/latest verification evidence: %+v; target reads=%d", b, count("100"))
		}
		points := f.points(t, instance)
		if len(points) != 2 {
			t.Fatalf("stored points=%d, want two actual current snapshots", len(points))
		}
		found := false
		for _, p := range points {
			if p.SubjectResourceID == f.ids[0] {
				found = true
				if p.StartedAt == nil || p.StartedAt.Unix() != changedAt {
					t.Fatalf("stored old target point: %+v", p)
				}
			}
		}
		if !found {
			t.Fatal("changed target missing from stored recovery points")
		}
		for _, p := range f.postures(t) {
			expected := recovery.ProtectionStateProtected
			if p.SubjectResourceID == f.ids[0] {
				expected = recovery.ProtectionStateAttention
			}
			if p.State != expected {
				t.Fatalf("posture=%+v, want %s", p, expected)
			}
		}
	}
	poll(0)
	f.assertProtected(t, instance)
	project("initial")
	targetRead, siblingRead := f.m.pbsBackupCacheTimeFor(instance, key), f.m.pbsBackupCacheTimeFor(instance, sibling)
	if targetRead.IsZero() || count("100") != 1 || count("101") != 1 {
		t.Fatal("initial inventory/cache not established")
	}
	poll(0)
	if count("100") != 1 || count("101") != 1 || !f.m.pbsBackupCacheTimeFor(instance, key).Equal(targetRead) {
		t.Fatal("unchanged groups lost bounded cache reuse")
	}
	poll(1)
	if count("100") != 2 {
		t.Fatalf("changed last-backup reused cache before TTL: target reads=%d, want 2", count("100"))
	}
	if failureStatus != 0 {
		b := row("100")
		if failureStatus == http.StatusForbidden {
			if b != nil {
				t.Fatalf("access rejection retained raw target: %+v", b)
			}
		} else if b == nil || b.BackupTime.Unix() != f.backupAt || !b.Verified {
			t.Fatalf("transient refresh failed to retain original raw row: %+v", b)
		}
		if len(f.points(t, instance)) != 2 || !f.m.pbsBackupCacheTimeFor(instance, key).Equal(targetRead) {
			t.Fatal("failed refresh deleted history or renewed cache")
		}
		wantState := recovery.ProtectionStateUnknown
		wantPermissions := operationaltrust.EvidencePermissionsUnknown
		if failureStatus == http.StatusForbidden {
			// A readable sibling plus rejected access is the existing partial-
			// permission Attention verdict, not a whole-provider outage.
			wantState = recovery.ProtectionStateAttention
			wantPermissions = operationaltrust.EvidencePermissionsPartial
		}
		for _, p := range f.postures(t) {
			if p.State != wantState || len(p.ProviderStates) != 1 ||
				p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial ||
				p.ProviderStates[0].Permissions != wantPermissions ||
				p.ProviderStates[0].JobState != recovery.OutcomeWarning {
				t.Fatalf("failed refresh posture=%+v, want %s/partial/%s/warning", p, wantState, wantPermissions)
			}
		}
		poll(2)
	} else {
		project("changed")
	}
	assertChanged()
	freshRead := f.m.pbsBackupCacheTimeFor(instance, key)
	if !freshRead.After(targetRead) || !f.m.pbsBackupCacheTimeFor(instance, sibling).Equal(siblingRead) || count("101") != 1 {
		t.Fatal("refresh did not renew only its changed group")
	}
	before := count("100")
	poll(2)
	assertChanged()
	if count("100") != before || !f.m.pbsBackupCacheTimeFor(instance, key).Equal(freshRead) {
		t.Fatal("unchanged replacement did not reuse its fresh cache")
	}
	poll(3)
	if row("100") != nil || row("101") == nil || len(f.points(t, instance)) != 1 || !f.m.pbsBackupCacheTimeFor(instance, key).IsZero() {
		t.Fatal("authoritative group removal lost sibling or retained deleted evidence")
	}
	project("withdrawn")
	poll(4)
	if len(f.m.state.GetSnapshot().PBSBackups) != 2 || len(f.points(t, instance)) != 2 {
		t.Fatal("ordinary recovery did not restore both actual snapshots")
	}
	for _, p := range f.postures(t) {
		wantTime := f.backupAt
		if p.SubjectResourceID == f.ids[0] {
			wantTime += 7200
		}
		if p.State != recovery.ProtectionStateProtected || p.LastSuccessfulPointAt == nil || p.LastSuccessfulPointAt.Unix() != wantTime {
			t.Fatalf("recovered posture=%+v, want current verified snapshot at %d", p, wantTime)
		}
	}
	if b := row("100"); b == nil || b.BackupTime.Unix() != f.backupAt+7200 || !b.Verified {
		t.Fatalf("ordinary recovery did not publish new actual snapshot: %+v", b)
	}
	project("recovered")
	t.Logf("kind=%s namespace=%q direction=%d failure=%d target_reads=%d sibling_reads=%d old=%s replacement=%s", kind, namespace, direction, failureStatus, count("100"), count("101"), strconv.FormatInt(f.backupAt, 10), strconv.FormatInt(changedAt, 10))
}
