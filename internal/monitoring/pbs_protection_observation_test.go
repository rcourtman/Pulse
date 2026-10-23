package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverystore "github.com/rcourtman/pulse-go-rewrite/internal/recovery/store"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func TestBuildPBSProtectionProviderObservationMapsEvidenceQuality(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name             string
		datastores       int
		fetches          int
		errors           int
		terminalFailures int
		wantJob          recovery.Outcome
		wantHistory      recovery.ProtectionHistoryCompleteness
		wantPermissions  operationaltrust.EvidencePermissions
		wantReason       string
	}{
		{
			name:            "complete enumeration",
			datastores:      2,
			fetches:         2,
			wantJob:         recovery.OutcomeSuccess,
			wantHistory:     recovery.ProtectionHistoryComplete,
			wantPermissions: operationaltrust.EvidencePermissionsSufficient,
		},
		{
			name:            "partial transient enumeration",
			datastores:      2,
			fetches:         1,
			errors:          1,
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pbs_partial_enumeration",
		},
		{
			name:             "partial provider access",
			datastores:       2,
			fetches:          1,
			errors:           1,
			terminalFailures: 1,
			wantJob:          recovery.OutcomeWarning,
			wantHistory:      recovery.ProtectionHistoryPartial,
			wantPermissions:  operationaltrust.EvidencePermissionsPartial,
			wantReason:       "pbs_partial_provider_access",
		},
		{
			name:            "provider timeout",
			datastores:      2,
			errors:          2,
			wantJob:         recovery.OutcomeFailed,
			wantHistory:     recovery.ProtectionHistoryUnavailable,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pbs_collection_unavailable",
		},
		{
			name:             "provider denies every datastore",
			datastores:       2,
			errors:           2,
			terminalFailures: 2,
			wantJob:          recovery.OutcomeFailed,
			wantHistory:      recovery.ProtectionHistoryUnavailable,
			wantPermissions:  operationaltrust.EvidencePermissionsDenied,
			wantReason:       "pbs_provider_access_denied",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := buildPBSProtectionProviderObservation(
				"pbs-main",
				test.datastores,
				test.fetches,
				test.errors,
				test.terminalFailures,
				now,
			)
			if err != nil {
				t.Fatalf("buildPBSProtectionProviderObservation() error = %v", err)
			}
			if got.JobState != test.wantJob {
				t.Fatalf("job state = %q, want %q", got.JobState, test.wantJob)
			}
			if got.HistoryCompleteness != test.wantHistory {
				t.Fatalf(
					"history completeness = %q, want %q",
					got.HistoryCompleteness,
					test.wantHistory,
				)
			}
			if got.Permissions != test.wantPermissions {
				t.Fatalf("permissions = %q, want %q", got.Permissions, test.wantPermissions)
			}
			if test.wantReason == "" {
				if got.Evidence.Reason != nil {
					t.Fatalf("reason = %#v, want nil", got.Evidence.Reason)
				}
			} else if got.Evidence.Reason == nil ||
				got.Evidence.Reason.Code != test.wantReason {
				t.Fatalf("reason = %#v, want %q", got.Evidence.Reason, test.wantReason)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

// The bounded cache has 100 rows both before and after deleting the newest
// of 101 snapshots. A backwards last-backup value must invalidate it even
// though its retained count still matches and its TTL has not expired.
func TestPBSOrdinaryPollNewestRemovalInvalidatesCappedCache(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, failure := range []int{0, http.StatusServiceUnavailable, http.StatusForbidden} {
			t.Run(fmt.Sprintf("%s/refresh-status-%d", kind, failure), func(t *testing.T) {
				f := newPBSPosturePollFixture(t, 1, kind)
				var phase, snapshotReads atomic.Int32
				oldAt := time.Unix(f.backupAt, 0).Add(-8 * 24 * time.Hour).Unix()
				restoredAt := f.backupAt + 3600
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mode := phase.Load()
					latest, count := f.backupAt, pbsBackupSnapshotsPerGroupLimit+1
					if mode == 1 || mode == 2 {
						latest, count = oldAt, pbsBackupSnapshotsPerGroupLimit
					} else if mode == 3 {
						latest = restoredAt
					} else if mode == 4 {
						latest, count = 0, 0
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "/groups"):
						_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
							"backup-type": kind, "backup-id": "100", "last-backup": latest, "backup-count": count,
						}}})
					case strings.HasSuffix(r.URL.Path, "/snapshots"):
						snapshotReads.Add(1)
						if mode == 2 {
							http.Error(w, "synthetic unreadable inventory", failure)
							return
						}
						rows := make([]map[string]any, 0, count)
						for i := 0; i < count; i++ {
							at := oldAt - int64(i)*3600
							if i == 0 && (mode == 0 || mode == 3) {
								at = latest
							} else if mode == 0 || mode == 3 {
								at += 3600
							}
							rows = append(rows, map[string]any{"backup-type": kind, "backup-id": "100", "backup-time": at,
								"size": 1024, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}, "comment": "guest-100"})
						}
						_ = json.NewEncoder(w).Encode(map[string]any{"data": rows})
					default:
						http.NotFound(w, r)
					}
				}))
				t.Cleanup(server.Close)
				var err error
				f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
				if err != nil {
					t.Fatal(err)
				}
				ds := []models.PBSDatastore{{Name: "archive"}}
				poll := func(mode int32) {
					phase.Store(mode)
					// No cache clearing, TTL expiry or monitor replacement: these
					// are consecutive ordinary polls of the same connection.
					f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, ds, nil)
					f.wait(t)
				}
				frames := map[string]any{}
				capture := func(name string) {
					points := f.points(t, "pbs-main")
					frames[name] = map[string]any{
						"pbs":         map[string]any{"data": map[string]any{"backups": f.m.state.GetSnapshot().PBSBackups}},
						"postures":    map[string]any{"data": f.postures(t), "policy": recovery.DefaultProtectionPosturePolicy.Payload()},
						"point_count": len(points), "snapshot_reads": snapshotReads.Load(),
					}
				}
				assertLatest := func(at int64, state recovery.ProtectionState) {
					t.Helper()
					raw, points, postures := f.m.state.GetSnapshot().PBSBackups, f.points(t, "pbs-main"), f.postures(t)
					if len(raw) != pbsBackupSnapshotsPerGroupLimit || len(points) != pbsBackupSnapshotsPerGroupLimit || len(postures) != 1 {
						t.Fatalf("raw/persisted/postures=%d/%d/%d, want %d/%d/1", len(raw), len(points), len(postures), pbsBackupSnapshotsPerGroupLimit, pbsBackupSnapshotsPerGroupLimit)
					}
					for _, row := range raw {
						if row.BackupTime.Unix() > at {
							t.Errorf("removed newest archive is still offered: %s > %s", row.BackupTime, time.Unix(at, 0))
						}
					}
					for _, point := range points {
						if point.CompletedAt == nil || point.CompletedAt.Unix() > at || point.SubjectResourceID != f.ids[0] {
							t.Errorf("persisted point is removed or belongs to the wrong subject: %s at %v", point.ID, point.CompletedAt)
						}
					}
					p := postures[0]
					if p.State != state || p.LastSuccessfulPointAt == nil || p.LastSuccessfulPointAt.Unix() != at {
						t.Errorf("posture=%s latest=%v, want %s at %s", p.State, p.LastSuccessfulPointAt, state, time.Unix(at, 0))
					}
				}
				key := pbsBackupGroupKey{datastore: "archive", backupType: kind, backupID: "100"}
				poll(0)
				assertLatest(f.backupAt, recovery.ProtectionStateProtected)
				capture("observed")
				readAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
				poll(0)
				if snapshotReads.Load() != 1 || !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(readAt) {
					t.Error("unchanged healthy group reread or renewed its snapshot cache")
				}
				if failure != 0 {
					for range 2 {
						before := snapshotReads.Load()
						poll(2)
						if snapshotReads.Load() <= before {
							t.Error("backwards group metadata did not trigger a snapshot refresh")
						}
						p := f.postures(t)[0]
						if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 || p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial {
							t.Errorf("failed prune refresh claimed protected/complete: %+v", p)
						}
						if len(f.points(t, "pbs-main")) != pbsBackupSnapshotsPerGroupLimit || !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(readAt) {
							t.Error("failed prune refresh deleted history or renewed its cache")
						}
					}
					capture("unreadable")
				}
				before := snapshotReads.Load()
				poll(1)
				if snapshotReads.Load() != before+1 {
					t.Errorf("pruned newest did not refresh snapshots: %d -> %d", before, snapshotReads.Load())
				}
				assertLatest(oldAt, recovery.ProtectionStateAttention)
				capture("removed")
				readAt = f.m.pbsBackupCacheTimeFor("pbs-main", key)
				before = snapshotReads.Load()
				poll(1)
				assertLatest(oldAt, recovery.ProtectionStateAttention)
				if snapshotReads.Load() != before || !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(readAt) {
					t.Error("unchanged pruned inventory did not resume cheap TTL reuse")
				}
				poll(3)
				assertLatest(restoredAt, recovery.ProtectionStateProtected)
				capture("recovered")
				poll(4)
				if len(f.m.state.GetSnapshot().PBSBackups) != 0 || len(f.points(t, "pbs-main")) != 0 || !f.m.pbsBackupCacheTimeFor("pbs-main", key).IsZero() {
					t.Error("confirmed empty group did not clear its inventory, recovery points and cache")
				}
				capture("empty")
				t.Logf("kind=%s failure=%d retained-limit=%d snapshot-reads=%d; newest removal, failed refresh, stale posture and recovery checked", kind, failure, pbsBackupSnapshotsPerGroupLimit, snapshotReads.Load())
				if os.Getenv("PULSE_PBS_PRUNING_FRAMES") == "1" && kind == "vm" && failure == http.StatusServiceUnavailable {
					source, err := os.ReadFile("monitor_backups.go")
					if err != nil {
						t.Fatal(err)
					}
					data, err := json.Marshal(map[string]any{"runtime_sha256": fmt.Sprintf("%x", sha256.Sum256(source)),
						"observed_at": time.Now().UTC(), "resource_ids": f.ids, "frames": frames})
					if err != nil {
						t.Fatal(err)
					}
					fmt.Printf("PBS_PRUNING_FRAMES=%s\n", data)
				}
			})
		}
	}
}

// Namespace discovery is part of backup enumeration, not just optional drawer
// metadata. Drive the public poll entry point and its background backup worker
// so a missing namespace cannot masquerade as an authoritative root-only scan.
type pbsNamespacePollFixture struct {
	m                *Monitor
	store            *recoverystore.Store
	client           *pbs.Client
	namespaceStatus  atomic.Int32
	namespaceRemoved atomic.Bool
	rootEmpty        atomic.Bool
	backupAt         atomic.Int64
	childReads       atomic.Int32
	ids              []string
}

func newPBSNamespacePollFixture(t *testing.T) *pbsNamespacePollFixture {
	t.Helper()
	m, manager := recoveryIngestTestMonitor(t)
	f := &pbsNamespacePollFixture{m: m}
	f.backupAt.Store(time.Now().UTC().Add(-time.Hour).Truncate(time.Second).Unix())
	m.state = models.NewState()
	m.state.UpdateVMs([]models.VM{
		{ID: makeGuestID("cluster", "node-a", 100), VMID: 100, Instance: "cluster", Node: "node-a", Name: "guest-100", Status: "running"},
		{ID: makeGuestID("cluster", "node-b", 101), VMID: 101, Instance: "cluster", Node: "node-b", Name: "guest-101", Status: "running"},
	})
	for _, id := range []int{100, 101} {
		f.ids = append(f.ids, unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "cluster", id))
	}
	var err error
	f.store, err = manager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.wait(t); _ = f.store.Close() })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var data any = []any{}
		switch r.URL.Path {
		case "/api2/json/version":
			data = map[string]any{"version": "3.4.2"}
		case "/api2/json/admin/datastore":
			data = []map[string]any{{"store": "archive"}}
		case "/api2/json/admin/datastore/archive/status":
			data = map[string]any{"total": 4096, "used": 1024, "avail": 3072}
		case "/api2/json/admin/datastore/archive/namespace":
			status := int(f.namespaceStatus.Load())
			if status == 200 {
				_, _ = w.Write([]byte(`{"data":`)) // incomplete, not a complete empty list
				return
			}
			if status != 0 {
				http.Error(w, "fixture namespace discovery unavailable", status)
				return
			}
			if !f.namespaceRemoved.Load() {
				data = []map[string]any{{"ns": "tenant"}}
			}
		case "/api2/json/admin/datastore/archive/groups":
			id := "100"
			if r.URL.Query().Get("ns") == "tenant" {
				id = "101"
				f.childReads.Add(1)
			}
			if id != "100" || !f.rootEmpty.Load() {
				data = []map[string]any{{"backup-type": "vm", "backup-id": id, "last-backup": f.backupAt.Load(), "backup-count": 1}}
			}
		case "/api2/json/admin/datastore/archive/snapshots":
			id := r.URL.Query().Get("backup-id")
			if r.URL.Query().Get("ns") == "tenant" {
				f.childReads.Add(1)
			}
			data = []map[string]any{{"backup-type": "vm", "backup-id": id, "backup-time": f.backupAt.Load(), "size": 1024,
				"files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}, "comment": "guest-" + id}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(server.Close)
	f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	if err != nil {
		t.Fatal(err)
	}
	m.config = &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1, PBSInstances: []config.PBSInstance{
		{Name: "pbs-main", Host: server.URL, MonitorDatastores: true, MonitorBackups: true},
		{Name: "pbs-other", Host: server.URL, MonitorDatastores: true, MonitorBackups: true},
	}}
	m.lastPBSBackupPoll = make(map[string]time.Time)
	return f
}

func (f *pbsNamespacePollFixture) wait(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		f.m.mu.RLock()
		polling := len(f.m.pbsBackupPollers) > 0
		f.m.mu.RUnlock()
		f.m.recoveryIngestMu.Lock()
		ingesting := f.m.recoveryIngestRunning
		f.m.recoveryIngestMu.Unlock()
		if !polling && !ingesting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("PBS namespace poll/ingest did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func (f *pbsNamespacePollFixture) poll(t *testing.T, instance string) {
	t.Helper()
	f.m.pollPBSInstance(context.Background(), instance, f.client)
	f.wait(t)
}

func (f *pbsNamespacePollFixture) points(t *testing.T, instance string) map[string]recovery.RecoveryPoint {
	t.Helper()
	points, _, err := f.store.ListPoints(context.Background(), recovery.ListPointsOptions{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]recovery.RecoveryPoint)
	for _, point := range points {
		if point.RepositoryRef != nil && point.RepositoryRef.Namespace == instance {
			out[point.ID] = point
		}
	}
	return out
}

func (f *pbsNamespacePollFixture) postures(t *testing.T) []recovery.ProtectionPosture {
	t.Helper()
	postures, total, err := f.store.ListProtectionPostures(context.Background(), recovery.ProtectionPostureQuery{SubjectResourceIDs: f.ids})
	if err != nil || total != len(f.ids) {
		t.Fatalf("postures total=%d err=%v", total, err)
	}
	return postures
}

func TestPBSNamespaceDiscoveryFailureRetainsRecoveryHistory(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusForbidden, http.StatusOK} {
		for _, cold := range []bool{false, true} {
			t.Run(fmt.Sprintf("status-%d/cold-%t", status, cold), func(t *testing.T) {
				f := newPBSNamespacePollFixture(t)
				f.poll(t, "pbs-main")
				before := f.points(t, "pbs-main")
				if len(before) != 2 {
					t.Fatalf("initial points=%d, want root and tenant", len(before))
				}
				for _, p := range f.postures(t) {
					if p.State != recovery.ProtectionStateProtected {
						t.Fatalf("healthy control=%+v", p)
					}
				}
				if cold {
					// Monitor replacement removes live inventory, not durable history.
					vms := f.m.state.GetSnapshot().VMs
					f.m.state = models.NewState()
					f.m.state.UpdateVMs(vms)
				}
				f.namespaceStatus.Store(int32(status))
				f.backupAt.Add(15 * 60)
				reads := f.childReads.Load()
				for range 2 {
					f.poll(t, "pbs-main")
					after := f.points(t, "pbs-main")
					if len(after) != 2 {
						t.Errorf("failed namespace discovery deleted durable history: %d points", len(after))
					}
					for id, point := range before {
						if point.Details["namespace"] == "tenant" && !reflect.DeepEqual(after[id], point) {
							t.Error("unseen namespace point was deleted or renewed without an observation")
						}
					}
					for _, point := range after {
						if point.Details["namespace"] == "" && (point.CompletedAt == nil || point.CompletedAt.Unix() != f.backupAt.Load()) {
							t.Error("independent successful root read did not update its point")
						}
					}
					permissions := operationaltrust.EvidencePermissionsUnknown
					if status == http.StatusForbidden {
						permissions = operationaltrust.EvidencePermissionsPartial
					}
					for _, p := range f.postures(t) {
						if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 ||
							p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial ||
							p.ProviderStates[0].Permissions != permissions || p.ProviderStates[0].JobState != recovery.OutcomeWarning {
							t.Errorf("discovery failure presented as complete/healthy: %+v", p)
						}
					}
					if f.childReads.Load() != reads {
						t.Error("failed discovery guessed a hidden namespace read")
					}
				}
			})
		}
	}
}

func TestPBSNamespaceDiscoveryRecoveryAndScopedRemoval(t *testing.T) {
	f := newPBSNamespacePollFixture(t)
	f.poll(t, "pbs-other")
	other := f.points(t, "pbs-other")
	f.poll(t, "pbs-main")
	f.namespaceStatus.Store(http.StatusServiceUnavailable)
	f.poll(t, "pbs-main")
	if len(f.points(t, "pbs-main")) != 2 {
		t.Error("transient discovery failure removed tenant history")
	}
	f.namespaceStatus.Store(0)
	f.poll(t, "pbs-main")
	if len(f.points(t, "pbs-main")) != 2 {
		t.Fatal("recovery did not restore namespace scope")
	}
	for _, p := range f.postures(t) {
		for _, provider := range p.ProviderStates {
			if provider.HistoryCompleteness != recovery.ProtectionHistoryComplete {
				t.Errorf("recovered discovery still incomplete: %+v", p)
			}
		}
	}
	f.namespaceRemoved.Store(true)
	f.poll(t, "pbs-main")
	if got := len(f.points(t, "pbs-main")); got != 1 {
		t.Errorf("authoritative namespace removal retained %d points, want root only", got)
	}
	if !reflect.DeepEqual(other, f.points(t, "pbs-other")) {
		t.Error("namespace removal crossed connection scope")
	}
	f.rootEmpty.Store(true)
	f.poll(t, "pbs-main")
	if len(f.points(t, "pbs-main")) != 0 {
		t.Error("authoritative complete empty scan retained points")
	}
	if !reflect.DeepEqual(other, f.points(t, "pbs-other")) {
		t.Error("empty scan crossed connection scope")
	}
}
