package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
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
					f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, ds)
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
