package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	recoverystore "github.com/rcourtman/pulse-go-rewrite/internal/recovery/store"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

type pbsPosturePollFixture struct {
	m        *Monitor
	store    *recoverystore.Store
	client   *pbs.Client
	mode     atomic.Int32
	backupAt int64
	ids      []string
}

func newPBSPosturePollFixture(t *testing.T, count int, backupTypes ...string) *pbsPosturePollFixture {
	t.Helper()
	m, manager := recoveryIngestTestMonitor(t)
	m.state = models.NewState()
	f := &pbsPosturePollFixture{m: m, backupAt: time.Now().Add(-14 * time.Hour).Truncate(time.Second).Unix()}
	backupType := "vm"
	if len(backupTypes) > 0 {
		backupType = backupTypes[0]
	}
	var vms []models.VM
	var containers []models.Container
	for i := 0; i < count; i++ {
		id := 100 + i
		node := fmt.Sprintf("node-%d", i%4)
		resourceType := unifiedresources.ResourceTypeVM
		if backupType == "ct" {
			resourceType = unifiedresources.ResourceTypeSystemContainer
			containers = append(containers, models.Container{ID: makeGuestID("cluster", node, id), VMID: id, Instance: "cluster", Node: node, Name: fmt.Sprintf("guest-%d", id), Status: "running"})
		} else {
			vms = append(vms, models.VM{ID: makeGuestID("cluster", node, id), VMID: id, Instance: "cluster", Node: node, Name: fmt.Sprintf("guest-%d", id), Status: "running"})
		}
		f.ids = append(f.ids, unifiedresources.ProxmoxGuestCanonicalID(resourceType, "cluster", id))
	}
	m.state.UpdateVMs(vms)
	m.state.UpdateContainers(containers)
	var err error
	f.store, err = manager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.wait(t); _ = f.store.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := f.mode.Load()
		if strings.HasSuffix(r.URL.Path, "/groups") {
			if mode == 4 {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			if mode == 5 {
				http.Error(w, "denied", http.StatusForbidden)
				return
			}
			if mode == 6 && r.URL.Query().Get("ns") == "unreadable" {
				http.Error(w, "denied", http.StatusForbidden)
				return
			}
			groups := make([]map[string]any, 0)
			if mode != 1 {
				for i := 0; i < count; i++ {
					groups = append(groups, map[string]any{"backup-type": backupType, "backup-id": strconv.Itoa(100 + i), "last-backup": f.backupAt, "backup-count": 1})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": groups})
		} else if strings.HasSuffix(r.URL.Path, "/snapshots") {
			id := r.URL.Query().Get("backup-id")
			if id == "100" && (mode == 2 || mode == 3) {
				status := http.StatusServiceUnavailable
				if mode == 3 {
					status = http.StatusForbidden
				}
				http.Error(w, "unreadable", status)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"backup-type": backupType, "backup-id": id, "backup-time": f.backupAt, "size": 1024, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}, "comment": "guest-" + id}}})
		} else {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *pbsPosturePollFixture) wait(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		f.m.recoveryIngestMu.Lock()
		running := f.m.recoveryIngestRunning
		f.m.recoveryIngestMu.Unlock()
		if !running {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("ordinary PBS ingest did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func (f *pbsPosturePollFixture) poll(t *testing.T, instance string, mode int32, datastores []models.PBSDatastore) {
	t.Helper()
	f.mode.Store(mode)
	// Deliberately expire snapshot reuse: group-count/time are unchanged when
	// only snapshot access or verification changes between ordinary polls.
	f.m.mu.Lock()
	clear(f.m.pbsBackupCacheTime)
	f.m.mu.Unlock()
	f.m.pollPBSBackups(context.Background(), instance, f.client, datastores, nil)
	f.wait(t)
}

func (f *pbsPosturePollFixture) points(t *testing.T, instance string) []recovery.RecoveryPoint {
	t.Helper()
	points, _, err := f.store.ListPoints(context.Background(), recovery.ListPointsOptions{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	var out []recovery.RecoveryPoint
	for _, p := range points {
		if p.RepositoryRef != nil && p.RepositoryRef.Namespace == instance {
			out = append(out, p)
		}
	}
	return out
}

func (f *pbsPosturePollFixture) postures(t *testing.T) []recovery.ProtectionPosture {
	t.Helper()
	postures, total, err := f.store.ListProtectionPostures(context.Background(), recovery.ProtectionPostureQuery{SubjectResourceIDs: f.ids})
	if err != nil || total != len(f.ids) {
		t.Fatalf("postures total=%d err=%v", total, err)
	}
	return postures
}

func (f *pbsPosturePollFixture) assertProtected(t *testing.T, instance string) {
	t.Helper()
	var raw []models.PBSBackup
	for _, b := range f.m.state.GetSnapshot().PBSBackups {
		if b.Instance == instance {
			raw = append(raw, b)
		}
	}
	mapped, err := proxmoxmapper.FromPBSBackupsWithEvidence(raw, buildPBSGuestCandidates(f.m.GetUnifiedReadStateOrSnapshot()), f.m.state.PBSBackupGuestInstances(), time.Now())
	if err != nil || len(mapped) != len(f.ids) {
		t.Fatalf("raw/mapped=%d/%d err=%v", len(raw), len(mapped), err)
	}
	for _, p := range mapped {
		vmid, _ := strconv.Atoi(p.Details["vmid"].(string))
		resourceType := unifiedresources.ResourceTypeVM
		if p.Details["backupType"] == "ct" {
			resourceType = unifiedresources.ResourceTypeSystemContainer
		}
		expected := unifiedresources.ProxmoxGuestCanonicalID(resourceType, "cluster", vmid)
		if p.SubjectResourceID != expected || p.Outcome != recovery.OutcomeSuccess {
			t.Fatalf("mapped subject=%s outcome=%s, want %s success", p.SubjectResourceID, p.Outcome, expected)
		}
	}
	persisted := f.points(t, instance)
	if len(persisted) != len(mapped) {
		t.Fatalf("persisted=%d mapped=%d", len(persisted), len(mapped))
	}
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateProtected || p.Verification != recovery.ProtectionVerificationVerified || p.LastSuccessfulPointAt == nil || p.LastSuccessfulPointAt.Unix() != f.backupAt {
			t.Fatalf("canonical posture=%+v", p)
		}
	}
	t.Logf("raw=%d mapped=%d persisted=%d protected=%d; original backup timestamp retained", len(raw), len(mapped), len(persisted), len(f.ids))
}

// A colliding VMID reads as one guest everywhere: on the badge, on the recovery
// point behind protection posture, and on the row the Backups table reads (#2681).
func TestPBSCollidingGuestReadsAsOneGuestOnEverySurface(t *testing.T) {
	backupAt := time.Now().Add(-14 * time.Hour).Truncate(time.Second)
	owners := map[string]string{"100": "home@pbs!token", "200": "home@pbs!token", "300": "hetzner@pbs!token"}
	for _, tc := range []struct {
		name string
		view bool
		want string
	}{
		// The owner token says home, but inference does not break a tie between
		// labels (#2292). Recovery points used to, against the badge.
		{name: "labels fit both guests", want: ""},
		{name: "an exclusive storage view settles the tie", view: true, want: "home"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, manager := recoveryIngestTestMonitor(t)
			m.state = models.NewState()
			m.state.UpdateVMs([]models.VM{
				{ID: makeGuestID("home", "pmx1", 100), VMID: 100, Instance: "home", Node: "pmx1", Name: "docker1", Status: "running"},
				{ID: makeGuestID("hetzner", "pmx1", 100), VMID: 100, Instance: "hetzner", Node: "pmx1", Name: "docker1", Status: "running"},
				{ID: makeGuestID("home", "pmx2", 200), VMID: 200, Instance: "home", Node: "pmx2", Name: "web", Status: "running"},
				{ID: makeGuestID("hetzner", "pmx3", 300), VMID: 300, Instance: "hetzner", Node: "pmx3", Name: "mail", Status: "running"},
			})
			if tc.view {
				m.state.UpdatePBSGuestConfirmationsForInstance("home", []models.PBSGuestConfirmation{
					{Storage: "pbs", BackupType: "vm", VMID: 100, Time: backupAt.Unix()},
				})
			}
			store, err := manager.StoreForOrg("default")
			if err != nil {
				t.Fatal(err)
			}
			f := &pbsPosturePollFixture{m: m, store: store}
			t.Cleanup(func() { f.wait(t); _ = store.Close() })
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/groups"):
					groups := make([]map[string]any, 0, len(owners))
					for id, owner := range owners {
						groups = append(groups, map[string]any{"backup-type": "vm", "backup-id": id, "last-backup": backupAt.Unix(), "backup-count": 1, "owner": owner})
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": groups})
				case strings.HasSuffix(r.URL.Path, "/snapshots"):
					id := r.URL.Query().Get("backup-id")
					snapshot := map[string]any{"backup-type": "vm", "backup-id": id, "backup-time": backupAt.Unix(), "size": 1024, "files": []string{"index.json.blob"}, "owner": owners[id]}
					if id == "100" {
						snapshot["comment"] = "docker1"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{snapshot}})
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
			if err != nil {
				t.Fatal(err)
			}
			m.pollPBSBackups(context.Background(), "pbs-main", client, []models.PBSDatastore{{Name: "main"}}, nil)
			f.wait(t)

			badges := make(map[string]bool)
			for _, vm := range m.state.GetSnapshot().VMs {
				if vm.VMID == 100 && vm.LastBackup.Equal(backupAt) {
					badges[vm.Instance] = true
				}
			}
			var table string
			var tableRows int
			for _, backup := range m.AttributedPBSBackupsSnapshot() {
				if backup.VMID == "100" {
					table = backup.GuestInstance
					tableRows++
				}
			}
			var subject string
			var points int
			persisted, _, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			for _, point := range persisted {
				if point.Details["vmid"] == "100" {
					subject = point.SubjectResourceID
					points++
				}
			}
			if tableRows != 1 || points != 1 {
				t.Fatalf("VM 100 rows=%d points=%d, want one of each", tableRows, points)
			}

			wantSubject := ""
			wantBadges := map[string]bool{}
			if tc.want != "" {
				wantSubject = unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, tc.want, 100)
				wantBadges[tc.want] = true
			}
			if table != tc.want {
				t.Errorf("table reads %q, want %q", table, tc.want)
			}
			if subject != wantSubject {
				t.Errorf("recovery point subject = %q, want %q", subject, wantSubject)
			}
			if !reflect.DeepEqual(badges, wantBadges) {
				t.Errorf("badges = %+v, want %+v", badges, wantBadges)
			}
		})
	}
}

// A failed read keeps its rows listed, but they stop teaching: the collision
// snapshot in the group that was read loses the source only those rows taught.
func TestPBSCarriedRowsStopTeachingGuestSources(t *testing.T) {
	backupAt := time.Now().Add(-14 * time.Hour).Truncate(time.Second)
	owners := map[string]string{"100": "home@pbs!token", "200": "home@pbs!token", "300": "hetzner@pbs!token"}
	var failTeacher, failAll atomic.Bool
	m, manager := recoveryIngestTestMonitor(t)
	m.state = models.NewState()
	m.state.UpdateVMs([]models.VM{
		{ID: makeGuestID("home", "pmx1", 100), VMID: 100, Instance: "home", Node: "pmx1", Name: "docker1", Status: "running"},
		{ID: makeGuestID("hetzner", "pmx1", 100), VMID: 100, Instance: "hetzner", Node: "pmx1", Name: "docker1", Status: "running"},
		{ID: makeGuestID("home", "pmx2", 200), VMID: 200, Instance: "home", Node: "pmx2", Name: "web", Status: "running"},
		{ID: makeGuestID("hetzner", "pmx3", 300), VMID: 300, Instance: "hetzner", Node: "pmx3", Name: "mail", Status: "running"},
	})
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	f := &pbsPosturePollFixture{m: m, store: store}
	t.Cleanup(func() { f.wait(t); _ = store.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/groups"):
			if failAll.Load() {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			groups := make([]map[string]any, 0, len(owners))
			for id, owner := range owners {
				groups = append(groups, map[string]any{"backup-type": "vm", "backup-id": id, "last-backup": backupAt.Unix(), "backup-count": 1, "owner": owner})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": groups})
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			id := r.URL.Query().Get("backup-id")
			if id == "200" && failTeacher.Load() {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"backup-type": "vm", "backup-id": id, "backup-time": backupAt.Unix(), "size": 1024, "files": []string{"index.json.blob"}, "owner": owners[id]}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	if err != nil {
		t.Fatal(err)
	}
	poll := func() map[string]string {
		m.mu.Lock()
		clear(m.pbsBackupCacheTime)
		m.mu.Unlock()
		m.pollPBSBackups(context.Background(), "pbs-main", client, []models.PBSDatastore{{Name: "main"}}, nil)
		f.wait(t)
		out := make(map[string]string)
		for _, backup := range m.AttributedPBSBackupsSnapshot() {
			out[backup.VMID] = backup.GuestInstance
		}
		return out
	}

	got := poll()
	if got["100"] != "home" || got["200"] != "home" || got["300"] != "hetzner" {
		t.Fatalf("after a complete read = %+v, want VM 100 on home through its owner token", got)
	}
	failTeacher.Store(true)
	got = poll()
	if got["200"] != "home" {
		t.Fatalf("after a failed read = %+v, want the carried row still on its own guest", got)
	}
	if got["100"] != "" {
		t.Fatalf("after a failed read = %+v, want VM 100 unattributed: only the carried row taught home's source", got)
	}

	// A poll that read nothing changes nothing: recovery points are not rewritten
	// then either, so every surface keeps its last answer until the next read.
	failTeacher.Store(false)
	if got = poll(); got["100"] != "home" {
		t.Fatalf("after recovery = %+v, want VM 100 back on home", got)
	}
	failAll.Store(true)
	if got = poll(); got["100"] != "home" || got["200"] != "home" || got["300"] != "hetzner" {
		t.Fatalf("after a poll that read nothing = %+v, want every row unchanged", got)
	}
}

// Adding a second installation with the same VMID and guest name withdraws a link
// the evidence no longer supports, on every surface. History must not put it back
// on the recovery point alone (#2681).
func TestPBSSecondInstallationWithdrawsUnsettledAttribution(t *testing.T) {
	backupAt := time.Now().Add(-14 * time.Hour).Truncate(time.Second)
	m, manager := recoveryIngestTestMonitor(t)
	m.state = models.NewState()
	home := models.VM{ID: makeGuestID("home", "pmx1", 100), VMID: 100, Instance: "home", Node: "pmx1", Name: "docker1", Status: "running"}
	m.state.UpdateVMs([]models.VM{home})
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	f := &pbsPosturePollFixture{m: m, store: store}
	t.Cleanup(func() { f.wait(t); _ = store.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/groups"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"backup-type": "vm", "backup-id": "100", "last-backup": backupAt.Unix(), "backup-count": 1}}})
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"backup-type": "vm", "backup-id": "100", "backup-time": backupAt.Unix(), "size": 1024, "files": []string{"index.json.blob"}, "comment": "docker1"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	if err != nil {
		t.Fatal(err)
	}
	read := func() (badge bool, subject, table string) {
		m.mu.Lock()
		clear(m.pbsBackupCacheTime)
		m.mu.Unlock()
		m.pollPBSBackups(context.Background(), "pbs-main", client, []models.PBSDatastore{{Name: "main"}}, nil)
		f.wait(t)
		for _, vm := range m.state.GetSnapshot().VMs {
			if vm.Instance == "home" {
				badge = vm.LastBackup.Equal(backupAt)
			}
		}
		points, _, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{Limit: 10})
		if err != nil || len(points) != 1 {
			t.Fatalf("points = %d, err = %v", len(points), err)
		}
		rows := m.AttributedPBSBackupsSnapshot()
		if len(rows) != 1 {
			t.Fatalf("rows = %d", len(rows))
		}
		return badge, points[0].SubjectResourceID, rows[0].GuestInstance
	}

	wantHome := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "home", 100)
	if badge, subject, table := read(); !badge || subject != wantHome || table != "home" {
		t.Fatalf("one installation: badge=%t subject=%q table=%q, want all on home", badge, subject, table)
	}
	m.state.UpdateVMs([]models.VM{home, {ID: makeGuestID("hetzner", "pmx1", 100), VMID: 100, Instance: "hetzner", Node: "pmx1", Name: "docker1", Status: "running"}})
	if badge, subject, table := read(); badge || subject != "" || table != "" {
		t.Fatalf("two installations: badge=%t subject=%q table=%q, want none naming a guest", badge, subject, table)
	}
}

func TestPBSOrdinaryPollRootNamespacePostureLifecycle(t *testing.T) {
	exercisePBSRootNamespacePostureLifecycle(t, "vm")
}

func TestPBSOrdinaryPollRootNamespaceContainerPostureLifecycle(t *testing.T) {
	exercisePBSRootNamespacePostureLifecycle(t, "ct")
}

func exercisePBSRootNamespacePostureLifecycle(t *testing.T, backupType string) {
	t.Helper()
	f := newPBSPosturePollFixture(t, 40, backupType)
	ds := []models.PBSDatastore{{Name: "archive"}}
	f.poll(t, "pbs-main", 0, ds)
	f.assertProtected(t, "pbs-main")
	f.poll(t, "pbs-main", 1, ds)
	if len(f.m.state.GetSnapshot().PBSBackups) != 0 || len(f.points(t, "pbs-main")) != 0 {
		t.Fatal("successful empty poll retained deleted backups")
	}
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateUnknown {
			t.Fatalf("empty posture=%+v, want unknown without a subject-linked source", p)
		}
	}
	f.poll(t, "pbs-main", 0, ds)
	f.assertProtected(t, "pbs-main")
	f.poll(t, "pbs-main", 4, ds)
	if len(f.points(t, "pbs-main")) != 40 || len(f.m.state.GetSnapshot().PBSBackups) != 40 {
		t.Fatal("transient group failure lost historical backups")
	}
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateUnknown || len(p.ProviderStates) != 1 || p.ProviderStates[0].JobState != recovery.OutcomeFailed || p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryUnavailable {
			t.Fatalf("failed posture=%+v", p)
		}
	}
	f.poll(t, "pbs-main", 0, ds)
	f.assertProtected(t, "pbs-main")
	f.poll(t, "pbs-main", 5, ds)
	if len(f.m.state.GetSnapshot().PBSBackups) != 0 || len(f.points(t, "pbs-main")) != 40 {
		t.Fatal("access denial must withdraw raw rows without deleting historical recovery facts")
	}
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateUnknown || len(p.ProviderStates) != 1 || p.ProviderStates[0].Permissions != operationaltrust.EvidencePermissionsDenied {
			t.Fatalf("denied posture=%+v", p)
		}
	}
	f.poll(t, "pbs-main", 0, ds)
	f.assertProtected(t, "pbs-main")
	// One connection's authoritative empty poll must not reconcile another.
	f.poll(t, "pbs-other", 0, ds)
	f.poll(t, "pbs-main", 1, ds)
	if len(f.points(t, "pbs-main")) != 0 || len(f.points(t, "pbs-other")) != 40 {
		t.Fatal("empty enumeration crossed the PBS instance boundary")
	}
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateProtected {
			t.Fatalf("other readable source lost protection: %+v", p)
		}
	}
}

func TestPBSOrdinaryPollSnapshotFailuresDoNotClaimCompleteHistory(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mode        int32
		permissions operationaltrust.EvidencePermissions
		state       recovery.ProtectionState
	}{
		{"transient", 2, operationaltrust.EvidencePermissionsUnknown, recovery.ProtectionStateAttention},
		{"denied", 3, operationaltrust.EvidencePermissionsPartial, recovery.ProtectionStateAttention},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPBSPosturePollFixture(t, 2)
			ds := []models.PBSDatastore{{Name: "archive"}}
			f.poll(t, "pbs-main", 0, ds)
			f.assertProtected(t, "pbs-main")
			f.poll(t, "pbs-main", tc.mode, ds)
			for attempt := 0; attempt < 2; attempt++ {
				points := f.points(t, "pbs-main")
				if len(points) != 2 {
					t.Errorf("incomplete snapshot enumeration reconciled away historical points: %d", len(points))
				}
				wantRaw := 2
				if tc.mode == 3 {
					wantRaw = 1
				}
				if got := len(f.m.state.GetSnapshot().PBSBackups); got != wantRaw {
					t.Errorf("raw snapshot rows=%d, want %d", got, wantRaw)
				}
				for _, p := range f.postures(t) {
					// Only guest 100's snapshot read fails; guest 101's snapshot
					// is re-read and stays protected under partial coverage.
					state := tc.state
					if p.SubjectResourceID == f.ids[1] {
						state = recovery.ProtectionStateProtected
					}
					if p.State != state || p.Coverage != recovery.ProtectionCoveragePartial || len(p.ProviderStates) != 1 || p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial || p.ProviderStates[0].Permissions != tc.permissions || p.ProviderStates[0].JobState != recovery.OutcomeWarning {
						t.Errorf("snapshot failure posture=%+v, want %s/partial/%s/warning", p, state, tc.permissions)
					}
				}
				// No TTL manipulation on the next poll: a failed refresh must not
				// become a healthy cached read while snapshot access is still lost.
				f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, ds, nil)
				f.wait(t)
			}
			f.poll(t, "pbs-main", 0, ds)
			f.assertProtected(t, "pbs-main")
		})
	}
}

func TestPBSOrdinaryPollPartialNamespaceRetainsUnseenHistory(t *testing.T) {
	f := newPBSPosturePollFixture(t, 2)
	ds := []models.PBSDatastore{{Name: "archive", Namespaces: []models.PBSNamespace{{Path: ""}, {Path: "unreadable"}}}}
	f.poll(t, "pbs-main", 0, ds)
	if len(f.points(t, "pbs-main")) != 4 {
		t.Fatal("fixture did not enumerate both namespaces")
	}
	f.poll(t, "pbs-main", 6, ds)
	if len(f.points(t, "pbs-main")) != 4 {
		t.Error("partial namespace poll deleted unseen historical points")
	}
	// Both guests' root snapshots were re-read; only the unreadable
	// namespace's retained points went unconfirmed, and it is named.
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateProtected || p.Coverage != recovery.ProtectionCoveragePartial || len(p.ProviderStates) != 1 || p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial || p.ProviderStates[0].Permissions != operationaltrust.EvidencePermissionsPartial ||
			!reflect.DeepEqual(p.ProviderStates[0].UnreadScopes, []recovery.ProtectionUnreadScope{{Kind: recovery.ProtectionUnreadScopeNamespace, Storage: "archive", Namespace: "unreadable", AccessDenied: true}}) {
			t.Errorf("partial namespace posture=%+v", p)
		}
	}
	f.poll(t, "pbs-main", 0, ds)
	if len(f.points(t, "pbs-main")) != 4 {
		t.Fatal("recovery did not restore both namespaces")
	}
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateProtected {
			t.Errorf("recovered posture=%+v", p)
		}
	}
}

func TestPBSOrdinaryPollAmbiguousVMIDsStayUnlinked(t *testing.T) {
	f := newPBSPosturePollFixture(t, 1)
	vms := f.m.state.GetSnapshot().VMs
	vms = append(vms, models.VM{ID: makeGuestID("other-cluster", "node-0", 100), VMID: 100, Instance: "other-cluster", Node: "node-0", Name: "guest-100", Status: "running"})
	f.m.state.UpdateVMs(vms)
	f.ids = append(f.ids, unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "other-cluster", 100))
	f.poll(t, "pbs-main", 0, []models.PBSDatastore{{Name: "archive"}})
	points := f.points(t, "pbs-main")
	if len(points) != 1 || points[0].SubjectResourceID != "" {
		t.Fatalf("ambiguous root-namespace backup was linked: %+v", points)
	}
	for _, p := range f.postures(t) {
		if p.State != recovery.ProtectionStateUnknown {
			t.Fatalf("ambiguous VMID painted protected: %+v", p)
		}
	}
}
