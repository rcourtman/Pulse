package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

// Real read-only PBS HTTP -> ordinary foreground/background collection -> raw
// inventory, original age/cache and stored protection. No guest or backup is
// started; rate limits never authorise a retry, wider access or new credentials.
func TestPBSBackupRetryableStatusLifecycle(t *testing.T) {
	for _, stage := range []string{"datastores", "namespaces", "groups", "snapshots"} {
		for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusForbidden} {
			t.Run(fmt.Sprintf("%s/%d", stage, status), func(t *testing.T) {
				f := newPBSPosturePollFixture(t, 2)
				const instance = "pbs-main"
				var phase atomic.Int32 // initial, rejected twice, recovered, authoritative empty
				var mu sync.Mutex
				reads := map[int32][]string{}
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, http.MethodGet, r.Method)
					p := phase.Load()
					foreign := strings.HasPrefix(r.URL.Path, "/other/")
					mu.Lock()
					reads[p] = append(reads[p], r.URL.RequestURI())
					mu.Unlock()
					nested := r.URL.Query().Get("ns") == "team/production"
					readStage, body := "", `{"data":[]}`
					id, at, size := "100", f.backupAt, 1024
					if nested {
						id = "101"
					}
					if foreign {
						id = "200"
					} else if p > 0 {
						at, size = at+3600, 4096
					}
					switch {
					case strings.HasSuffix(r.URL.Path, "/version"):
						body = `{"data":{"version":"3.4.2"}}`
					case strings.HasSuffix(r.URL.Path, "/nodes/localhost/status"):
						body = `{"data":{"cpu":0.1,"memory":{"used":512,"total":1024},"uptime":120}}`
					case strings.HasSuffix(r.URL.Path, "/admin/datastore"):
						readStage = "datastores"
						if p != 4 {
							body = `{"data":[{"store":"archive"}]}`
						}
					case strings.HasSuffix(r.URL.Path, "/namespace"):
						readStage = "namespaces"
						body = `{"data":[{"ns":""},{"ns":"team/production"}]}`
					case strings.HasSuffix(r.URL.Path, "/groups"):
						if nested {
							readStage = "groups"
						}
						body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":%q,"backup-count":1,"last-backup":%d}]}`, id, at)
					case strings.HasSuffix(r.URL.Path, "/snapshots"):
						if nested {
							readStage = "snapshots"
						}
						body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":%q,"backup-time":%d,"size":%d,"files":["index.json.blob"],"verification":{"state":"ok"},"comment":%q}]}`, id, at, size, "guest-"+id)
					case strings.HasSuffix(r.URL.Path, "/status"):
						body = `{"data":{"status":"available","total":1000,"used":250,"avail":750}}`
					}
					if !foreign && (p == 1 || p == 2) && readStage == stage {
						w.Header().Set("Retry-After", "3600")
						// This synthetic private body must not influence status or
						// enter retained diagnostics, even if it contradicts HTTP.
						http.Error(w, "fixture-private-body API error 403: forbidden", status)
						return
					}
					fmt.Fprint(w, body)
				}))
				defer server.Close()
				pinBytes := sha256.Sum256(server.Certificate().Raw)
				pin := hex.EncodeToString(pinBytes[:])
				f.m.config = &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1, PBSInstances: []config.PBSInstance{{
					Name: instance, Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Fingerprint: pin, VerifySSL: true, MonitorDatastores: true, MonitorBackups: true,
				}}}
				f.m.pollStatusMap = make(map[string]*pollStatus)
				f.m.initPBSClients(f.m.config)
				client, ok := f.m.getPBSClient(instance)
				require.True(t, ok)
				f.m.setRuntimeContext(context.Background(), nil)
				poll := func(p int32) {
					phase.Store(p)
					f.m.SetBackupPollingInterval(2 * time.Second)
					f.m.SetBackupPollingInterval(time.Second)
					f.m.pollPBSInstance(context.Background(), instance, client)
					waitForPBSBackupReturn(t, f.m)
					f.wait(t)
				}
				rows := func(name string) []models.PBSBackup {
					out := []models.PBSBackup{}
					for _, row := range f.m.PBSBackupsSnapshot() {
						if row.Instance == name {
							out = append(out, row)
						}
					}
					return out
				}
				project := func(label string) {
					wire, err := json.Marshal(map[string]any{"backups": rows(instance), "postures": f.postures(t)})
					require.NoError(t, err)
					require.NotContains(t, string(wire), "fixture-private-body")
					t.Logf("PBS_RETRY_BROWSER_%s_%d_%s=%s", stage, status, label, base64.StdEncoding.EncodeToString(wire))
				}
				poll(0)
				f.assertProtected(t, instance)
				project("initial")
				initial := rows(instance)
				var held models.PBSBackup
				for _, row := range initial {
					if row.VMID == "101" {
						held = row
					}
				}
				require.Equal(t, "team/production", held.Namespace)
				key := pbsBackupGroupKey{datastore: "archive", namespace: held.Namespace, backupType: "vm", backupID: "101"}
				oldRead := time.Now().Add(-2 * pbsBackupCacheTTL)
				f.m.setPBSBackupCacheTime(instance, key, oldRead)
				other, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL + "/other", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Fingerprint: pin, VerifySSL: true, Timeout: time.Second})
				require.NoError(t, err)
				f.m.pollPBSBackups(context.Background(), "pbs-other", other, []models.PBSDatastore{{Name: "archive"}})
				f.wait(t)
				otherRows, otherPoints := rows("pbs-other"), f.points(t, "pbs-other")
				for _, p := range []int32{1, 2} {
					poll(p)
					current := rows(instance)
					if status == http.StatusForbidden {
						for _, row := range current {
							require.NotEqual(t, "101", row.VMID, "access-denied raw scope must withdraw")
						}
						if stage == "datastores" {
							require.Empty(t, current)
						}
					} else {
						require.Contains(t, current, held, "retryable status must retain exact last-known artifact")
						require.Equal(t, oldRead, f.m.pbsBackupCacheTimeFor(instance, key), "failed read must not renew freshness")
						if stage == "datastores" {
							require.ElementsMatch(t, initial, current)
						}
					}
					if stage != "datastores" {
						siblingFound := false
						for _, row := range current {
							if row.VMID == "100" {
								siblingFound = true
								require.Equal(t, int64(4096), row.Size, "readable sibling must advance")
							}
						}
						require.True(t, siblingFound, "readable sibling disappeared")
					}
					heldPoints := 0
					for _, point := range f.points(t, instance) {
						if point.SubjectResourceID == f.ids[1] {
							heldPoints++
							require.Equal(t, f.backupAt, point.CompletedAt.Unix(), "failure cannot mint a newer recovery point")
						}
					}
					require.Equal(t, 1, heldPoints, "failed enumeration must preserve the historical recovery fact, not add one")
					for _, posture := range f.postures(t) {
						require.Len(t, posture.ProviderStates, 1)
						provider := posture.ProviderStates[0]
						require.NotEqual(t, recovery.ProtectionStateProtected, posture.State)
						require.NotEqual(t, recovery.ProtectionHistoryComplete, provider.HistoryCompleteness)
						permissions := operationaltrust.EvidencePermissionsUnknown
						if status == http.StatusForbidden {
							permissions = operationaltrust.EvidencePermissionsPartial
							if stage == "datastores" {
								permissions = operationaltrust.EvidencePermissionsDenied
							}
						}
						require.Equal(t, permissions, provider.Permissions)
					}
					mu.Lock()
					rejectedReads := 0
					for _, uri := range reads[p] {
						if stage == "datastores" {
							require.NotContains(t, uri, "/admin/datastore/", "failed discovery cannot probe cached paths")
						}
						if strings.Contains(uri, "/"+map[string]string{"datastores": "datastore", "namespaces": "namespace", "groups": "groups", "snapshots": "snapshots"}[stage]) &&
							(stage == "datastores" || stage == "namespaces" || strings.Contains(uri, "ns=team%2Fproduction")) {
							rejectedReads++
						}
					}
					mu.Unlock()
					require.Equal(t, 1, rejectedReads, "no immediate Retry-After replay inside a poll")
					project(fmt.Sprintf("rejected%d", p))
				}
				poll(3)
				require.Len(t, rows(instance), 2)
				require.True(t, f.m.pbsBackupCacheTimeFor(instance, key).After(oldRead))
				for _, posture := range f.postures(t) {
					require.Equal(t, recovery.ProtectionStateProtected, posture.State)
					require.Equal(t, f.backupAt+3600, posture.LastSuccessfulPointAt.Unix())
				}
				project("recovered")
				poll(4)
				require.Empty(t, rows(instance), "complete empty discovery still owns removal")
				require.Empty(t, f.points(t, instance))
				require.Equal(t, otherRows, rows("pbs-other"))
				require.Equal(t, otherPoints, f.points(t, "pbs-other"))
				project("empty")
			})
		}
	}
}

func TestPBSBackupCacheHTTPStatusClassification(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 408, 409, 429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, "fixture-private-body API error 429: rate limited")
			}))
			defer server.Close()
			pin := sha256.Sum256(server.Certificate().Raw)
			client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, Fingerprint: hex.EncodeToString(pin[:]), VerifySSL: true, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Timeout: time.Second})
			require.NoError(t, err)
			_, err = client.GetDatastores(context.Background())
			require.Error(t, err)
			require.NotContains(t, err.Error(), "fixture-private-body")
			want := status == 408 || status == 429 || status >= 500
			for _, failure := range []error{err, fmt.Errorf("nested poll: %w", err)} {
				require.Equal(t, want, shouldReuseCachedPBSBackups(failure))
			}
		})
	}
}
