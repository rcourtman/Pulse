package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// Real read-only HTTP -> ordinary collector -> raw/cache and recovery consumers.
// No guest command, appliance, backup or restore operation is exercised.
func TestPBSNamespaceFailuresRespectReadScope(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			f := newPBSPosturePollFixture(t, 1)
			var phase atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method %s", r.Method)
				}
				p := phase.Load()
				foreign := strings.HasPrefix(r.URL.Path, "/other/")
				ns := r.URL.Query().Get("ns")
				id := "100"
				if foreign {
					id = "200" // An independent connection and subject.
				}
				if strings.HasSuffix(r.URL.Path, "/groups") {
					if !foreign && p == 6 {
						status := http.StatusForbidden
						if ns == "tenant/child" {
							status = http.StatusServiceUnavailable
						}
						http.Error(w, "fixture unavailable", status)
						return
					}
					if !foreign && ns == "tenant/child" {
						switch p {
						case 1:
							http.Error(w, "upstream API error 403 is not the wire status", http.StatusServiceUnavailable)
							return
						case 2:
							fmt.Fprint(w, `{"data":null}`)
							return
						case 3:
							http.Error(w, "fixture denied", http.StatusForbidden)
							return
						case 5:
							fmt.Fprint(w, `{"data":[]}`)
							return
						}
					}
					// The parent namespace can authoritatively empty independently
					// of its unreadable child; namespace matching is not by prefix.
					if !foreign && ns == "tenant" && p > 0 {
						fmt.Fprint(w, `{"data":[]}`)
						return
					}
				}
				backupAt, size := f.backupAt, 1024
				if !foreign && p > 0 {
					backupAt += 3600
					size = 2048
					if p >= 4 {
						backupAt += 3600
						size = 4096
						if ns == "tenant/child" {
							size = 3072
						}
					}
				}
				switch {
				case strings.HasSuffix(r.URL.Path, "/groups"):
					fmt.Fprintf(w, `{"data":[{"backup-type":"vm","backup-id":%q,"backup-count":1,"last-backup":%d}]}`, id, backupAt)
				case strings.HasSuffix(r.URL.Path, "/snapshots"):
					fmt.Fprintf(w, `{"data":[{"backup-type":"vm","backup-id":%q,"backup-time":%d,"size":%d,"files":["index.json.blob"],"verification":{"state":"ok"},"comment":%q}]}`, id, backupAt, size, "guest-"+id)
				default:
					t.Errorf("unexpected read %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := func(prefix string) *pbs.Client {
				c, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL + prefix, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Timeout: time.Second})
				if err != nil {
					t.Fatal(err)
				}
				return c
			}
			f.client = client("")
			datastores := []models.PBSDatastore{
				{Name: "archive", Namespaces: []models.PBSNamespace{{Path: "/"}, {Path: "tenant"}, {Path: "tenant/child"}}},
				{Name: "independent"},
			}
			if reverse {
				s := datastores[0].Namespaces
				s[0], s[2] = s[2], s[0]
			}
			poll := func(p int32) {
				phase.Store(p)
				f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, datastores)
				f.wait(t)
			}
			rows := func(instance string) []models.PBSBackup {
				var out []models.PBSBackup
				for _, row := range f.m.state.GetSnapshot().PBSBackups {
					if row.Instance == instance {
						out = append(out, row)
					}
				}
				sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
				return out
			}
			project := func(label string) {
				if reverse {
					return
				}
				payload, err := json.Marshal(map[string]any{"backups": f.m.state.GetSnapshot().PBSBackups, "postures": f.postures(t), "now": time.Now().UTC()})
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("PBS_NAMESPACE_PROJECTION_%s=%s", label, payload)
			}
			poll(0)
			if len(rows("pbs-main")) != 4 || len(f.points(t, "pbs-main")) != 4 {
				t.Fatal("initial fixture did not enumerate all datastore/namespace scopes")
			}
			project("initial")
			// This separate connection has no subject overlap with the target
			// provider's posture, but its raw and persisted points must survive.
			f.m.pollPBSBackups(context.Background(), "pbs-other", client("/other"), []models.PBSDatastore{{Name: "archive"}})
			f.wait(t)
			foreignRows := rows("pbs-other")
			foreignPoints := f.points(t, "pbs-other")
			if len(foreignRows) != 1 || len(foreignPoints) != 1 {
				t.Fatal("independent connection was not seeded")
			}
			var held models.PBSBackup
			for _, row := range rows("pbs-main") {
				if row.Namespace == "tenant/child" {
					held = row
				}
			}
			key := pbsBackupGroupKey{datastore: "archive", namespace: "tenant/child", backupType: "vm", backupID: "100"}
			oldRead := time.Now().Add(-2 * pbsBackupCacheTTL)
			f.m.setPBSBackupCacheTime("pbs-main", key, oldRead)
			for _, p := range []int32{1, 2} {
				poll(p)
				actual := rows("pbs-main")
				if len(actual) != 3 {
					t.Fatalf("phase %d dropped the transiently unreadable namespace or retained the successfully emptied parent: rows=%+v", p, actual)
				}
				for _, row := range actual {
					if row.Namespace == "tenant/child" {
						if !reflect.DeepEqual(row, held) {
							t.Fatal("unreadable namespace renewed or changed its original backup evidence")
						}
					} else if row.Size != 2048 || row.BackupTime.Unix() != f.backupAt+3600 {
						t.Fatal("unreadable namespace prevented independent fresh scope publication")
					}
				}
				if !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(oldRead) {
					t.Fatal("failed group read renewed its snapshot cache time")
				}
				for _, posture := range f.postures(t) {
					if posture.State == recovery.ProtectionStateProtected || posture.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial {
						t.Fatalf("held history falsely established complete protection: %+v", posture)
					}
				}
				project(fmt.Sprintf("partial_%d", p))
			}
			poll(3)
			if len(rows("pbs-main")) != 2 || len(f.points(t, "pbs-main")) < 4 {
				t.Fatal("access rejection must withdraw raw rows without deleting historical recovery facts")
			}
			project("denied")
			poll(4)
			if len(rows("pbs-main")) != 3 || len(f.points(t, "pbs-main")) != 3 || !f.m.pbsBackupCacheTimeFor("pbs-main", key).After(oldRead) {
				t.Fatal("complete recovery did not replace rows, publish matching read times and reconcile obsolete points")
			}
			project("recovered")
			poll(6)
			if got := rows("pbs-main"); len(got) != 1 || got[0].Namespace != "tenant/child" || got[0].Size != 3072 {
				t.Fatalf("mixed unavailable/denied scopes kept rejected raw rows or lost held history: %+v", got)
			}
			project("mixed_failure")
			poll(4)
			poll(5)
			if len(rows("pbs-main")) != 2 || len(f.points(t, "pbs-main")) != 2 || !f.m.pbsBackupCacheTimeFor("pbs-main", key).IsZero() {
				t.Fatal("successful scoped empty inventory did not withdraw its rows, cache and recovery points")
			}
			project("withdrawn")
			if !reflect.DeepEqual(rows("pbs-other"), foreignRows) || !reflect.DeepEqual(f.points(t, "pbs-other"), foreignPoints) {
				t.Fatal("partial failure or empty inventory crossed the PBS connection boundary")
			}
		})
	}
}
