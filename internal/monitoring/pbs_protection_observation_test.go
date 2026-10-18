package monitoring

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/stretchr/testify/require"
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
			name:            "readable sibling namespace is not complete history",
			datastores:      1,
			fetches:         1,
			errors:          1,
			wantJob:         recovery.OutcomeWarning,
			wantHistory:     recovery.ProtectionHistoryPartial,
			wantPermissions: operationaltrust.EvidencePermissionsUnknown,
			wantReason:      "pbs_partial_enumeration",
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

// A count-preserving prune/import can move last-backup backwards. Exercise the
// ordinary HTTP collector and stored projection before the unchanged cache TTL
// expires; no native backup, guest, clock change or recovery action is involved.
func TestPBSGroupLatestChangeRefreshesBeforeCacheTTL(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, namespace := range []string{"", "team/production"} {
			for _, direction := range []int64{-1, 1} {
				t.Run(fmt.Sprintf("%s/%s/direction_%d", kind, namespace, direction), func(t *testing.T) {
					exercisePBSGroupLatestChange(t, kind, namespace, direction, 0)
				})
			}
		}
	}
}

func TestPBSGroupLatestChangePreservesFailedRefreshBoundaries(t *testing.T) {
	for _, namespace := range []string{"", "team/production"} {
		for _, status := range []int{http.StatusServiceUnavailable, http.StatusForbidden} {
			t.Run(fmt.Sprintf("%s/status_%d", namespace, status), func(t *testing.T) {
				exercisePBSGroupLatestChange(t, "vm", namespace, -1, status)
			})
		}
	}
}

// Failed namespace discovery must qualify the stored protection verdict.
func TestPBSNamespaceDiscoveryFailureIsNotEmptyInventory(t *testing.T) {
	exercisePBSNamespaceDiscovery(t, http.StatusServiceUnavailable, "fixture temporarily unavailable", false, false, "transient")
}

// Ordinary read-only foreground -> admitted background -> raw/cache and durable
// protection. A successful empty discovery is distinct from failure or scope-off.
type pbsDatastoreDiscoveryCase struct {
	name     string
	phase    int32
	disabled string
}

func TestPBSRootDatastoreDiscoveryRegression(t *testing.T) {
	exercisePBSRootDatastoreDiscovery(t, []pbsDatastoreDiscoveryCase{
		{"transient", 1, ""}, {"denied", 2, ""}, {"null", 4, ""}, {"missing", 5, ""}, {"conflicting", 6, ""},
	})
}

func exercisePBSRootDatastoreDiscovery(t *testing.T, cases []pbsDatastoreDiscoveryCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPBSPosturePollFixture(t, 2)
			const instance = "pbs-main"
			var phase atomic.Int32
			pollCtx, cancelPoll := context.WithCancel(context.Background())
			defer cancelPoll()
			var mu sync.Mutex
			reads := map[int32][]string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method, "no provider mutation")
				p := phase.Load()
				mu.Lock()
				reads[p] = append(reads[p], r.URL.Path)
				mu.Unlock()
				foreign := strings.HasPrefix(r.URL.Path, "/other/")
				id := "100"
				if strings.Contains(r.URL.Path, "/mirror/") {
					id = "101"
				}
				if foreign {
					id = "200"
				}
				at, size := f.backupAt, 1024
				if p > 0 && !foreign {
					at += 3600
					size = 4096
				}
				body := `{"data":[]}`
				switch {
				case strings.HasSuffix(r.URL.Path, "/version"):
					body = `{"data":{"version":"3.4.2"}}`
				case r.URL.Path == "/api2/json/nodes/localhost/status":
					body = `{"data":{"cpu":0.1,"memory":{"used":512,"total":1024},"uptime":120}}`
				case r.URL.Path == "/api2/json/admin/datastore":
					body = `{"data":[{"store":"archive"},{"store":"mirror"}]}`
					switch p {
					case 1:
						http.Error(w, "fixture unavailable", http.StatusServiceUnavailable)
						return
					case 2:
						http.Error(w, "fixture denied", http.StatusForbidden)
						return
					case 3:
						body = `{"data":[]}`
					case 4:
						body = `{"data":null}`
					case 5:
						body = `{}`
					case 6:
						body = `{"data":[],"data":[{"store":"mirror"}]}`
					case 7:
						body = `{"data":[{"store":"mirror"}]}`
					case 9:
						cancelPoll()
						http.Error(w, "fixture cancelled", http.StatusServiceUnavailable)
						return
					case 10, 11:
						// Replacement need not cancel the old foreground parent,
						// and may even reuse that exact context value.
						f.m.setRuntimeContext(context.Background(), nil)
						if p == 11 {
							http.Error(w, "old fixture denied", http.StatusForbidden)
							return
						}
						body = `{"data":[]}`
					}
				case strings.HasSuffix(r.URL.Path, "/status"):
					body = `{"data":{"status":"available","total":1000,"used":250,"avail":750}}`
				case strings.HasSuffix(r.URL.Path, "/groups"):
					body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":%q,"backup-count":1,"last-backup":%d}]}`, id, at)
				case strings.HasSuffix(r.URL.Path, "/snapshots"):
					body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":%q,"backup-time":%d,"size":%d,"files":["index.json.blob"],"verification":{"state":"ok"},"comment":%q}]}`, id, at, size, "guest-"+id)
				}
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			f.m.config = &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1, PBSInstances: []config.PBSInstance{{Name: instance, Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", MonitorDatastores: true, MonitorBackups: true}}}
			f.m.pollStatusMap = make(map[string]*pollStatus)
			f.m.initPBSClients(f.m.config)
			client, ok := f.m.getPBSClient(instance)
			require.True(t, ok)
			f.m.setRuntimeContext(context.Background(), nil)
			poll := func(p int32) {
				phase.Store(p)
				f.m.SetBackupPollingInterval(2 * time.Second)
				f.m.SetBackupPollingInterval(time.Second)
				ctx := context.Background()
				if p == 9 {
					ctx = pollCtx
				}
				f.m.pollPBSInstance(ctx, instance, client)
				waitForPBSBackupReturn(t, f.m)
				f.wait(t)
			}
			rows := func(instance string) []models.PBSBackup {
				out := []models.PBSBackup{}
				for _, row := range f.m.PBSBackupsSnapshot() {
					if row.Instance == instance {
						out = append(out, row)
					}
				}
				return out
			}
			project := func(label string) {
				if tc.name != "transient" && tc.name != "denied" && tc.name != "empty" {
					return
				}
				payload, err := json.Marshal(map[string]any{"backups": rows(instance), "postures": f.postures(t)})
				require.NoError(t, err)
				t.Logf("PBS_DATASTORE_BROWSER_%s_%s=%s", tc.name, label, base64.StdEncoding.EncodeToString(payload))
			}
			poll(0)
			f.assertProtected(t, instance)
			project("initial")
			initial, points := rows(instance), f.points(t, instance)
			key := pbsBackupGroupKey{datastore: "archive", backupType: "vm", backupID: "100"}
			readAt := f.m.pbsBackupCacheTimeFor(instance, key)
			require.False(t, readAt.IsZero())
			other, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL + "/other", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Timeout: time.Second})
			require.NoError(t, err)
			f.m.pollPBSBackups(context.Background(), "pbs-other", other, []models.PBSDatastore{{Name: "archive"}})
			f.wait(t)
			otherRows, otherPoints := rows("pbs-other"), f.points(t, "pbs-other")
			require.Len(t, otherRows, 1)
			require.Len(t, otherPoints, 1)
			switch tc.disabled {
			case "datastores":
				f.m.config.PBSInstances[0].MonitorDatastores = false
			case "backups":
				f.m.config.PBSInstances[0].MonitorBackups = false
			case "global":
				f.m.SetBackupPollingEnabled(false)
			}
			poll(tc.phase)
			project("observed")
			if tc.disabled != "" {
				require.Equal(t, initial, rows(instance), "scope-off must not invent an empty inventory")
				require.Equal(t, points, f.points(t, instance))
				f.assertProtected(t, instance)
			} else if tc.phase == 3 || tc.phase == 7 {
				want := 0
				if tc.phase == 7 {
					want = 1
				}
				require.Len(t, rows(instance), want, "complete datastore discovery must reconcile absence")
				require.Len(t, f.points(t, instance), want)
				require.Zero(t, f.m.pbsBackupCacheTimeFor(instance, key))
				for _, posture := range f.postures(t) {
					expected := recovery.ProtectionStateUnknown
					if tc.phase == 7 && posture.SubjectResourceID == f.ids[1] {
						expected = recovery.ProtectionStateProtected
					}
					require.Equal(t, expected, posture.State)
				}
			} else {
				require.Equal(t, points, f.points(t, instance), "failed discovery must not delete or re-observe historical points")
				state, permissions := recovery.ProtectionStateUnknown, operationaltrust.EvidencePermissionsUnknown
				if tc.phase == 2 {
					require.Empty(t, rows(instance), "access rejection withdraws reusable raw history")
					require.Zero(t, f.m.pbsBackupCacheTimeFor(instance, key))
					// No readable history is Unknown, even when access is known
					// denied. Attention applies to partially readable evidence.
					permissions = operationaltrust.EvidencePermissionsDenied
				} else {
					require.Equal(t, initial, rows(instance), "failed discovery must retain exact last-known artifacts")
					require.Equal(t, readAt, f.m.pbsBackupCacheTimeFor(instance, key), "unread history must not be freshened")
				}
				for _, posture := range f.postures(t) {
					require.Equal(t, state, posture.State, "failed discovery cannot leave previous protection healthy")
					require.Len(t, posture.ProviderStates, 1)
					require.Equal(t, recovery.ProtectionHistoryUnavailable, posture.ProviderStates[0].HistoryCompleteness)
					require.Equal(t, recovery.OutcomeFailed, posture.ProviderStates[0].JobState)
					require.Equal(t, permissions, posture.ProviderStates[0].Permissions)
				}
			}
			if tc.phase <= 6 {
				mu.Lock()
				for _, path := range reads[tc.phase] {
					require.False(t, strings.Contains(path, "/admin/datastore/"), "unknown/empty datastore discovery must not probe cached datastore paths: %s", path)
				}
				mu.Unlock()
			}
			f.m.config.PBSInstances[0].MonitorDatastores = true
			f.m.config.PBSInstances[0].MonitorBackups = true
			f.m.SetBackupPollingEnabled(true)
			poll(8)
			for _, posture := range f.postures(t) {
				require.Equal(t, recovery.ProtectionStateProtected, posture.State)
				require.NotNil(t, posture.LastSuccessfulPointAt)
				require.Equal(t, f.backupAt+3600, posture.LastSuccessfulPointAt.Unix())
				require.Len(t, posture.ProviderStates, 1)
				require.Equal(t, recovery.ProtectionHistoryComplete, posture.ProviderStates[0].HistoryCompleteness)
			}
			for _, row := range rows(instance) {
				require.Equal(t, int64(4096), row.Size)
				require.Equal(t, f.backupAt+3600, row.BackupTime.Unix())
			}
			project("recovered")
			poll(3)
			require.Empty(t, rows(instance))
			require.Empty(t, f.points(t, instance))
			require.Equal(t, otherRows, rows("pbs-other"), "empty inventory crossed a connection boundary")
			require.Equal(t, otherPoints, f.points(t, "pbs-other"))
			project("empty")
		})
	}
}
