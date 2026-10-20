package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// Pinned synthetic TLS -> ordinary foreground/background collection -> retained
// artifacts/cache -> stored protection and the served resource projection. No
// backup, guest operation, credential replacement or immediate retry is used.
func TestPBSNamespaceIdentityLifecycle(t *testing.T) {
	f := newPBSPosturePollFixture(t, 3)
	const instance, nested = "pbs-main", "team/production"
	phases := []string{"initial", "path-conflict", "name-conflict", "root-conflict", "equivalent-aliases", "legacy-path", "legacy-name", "empty", "restored"}
	var phase atomic.Int32
	var mu sync.Mutex
	reads := make(map[int32][]string)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "PBSAPIToken=fixture@pbs!reader:fixture-only" {
			t.Errorf("unexpected provider mutation or credential change: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		p := phase.Load()
		mu.Lock()
		reads[p] = append(reads[p], r.URL.RequestURI())
		mu.Unlock()
		foreign := strings.HasPrefix(r.URL.Path, "/other/")
		mirror := strings.Contains(r.URL.Path, "/mirror/")
		ns := r.URL.Query().Get("ns")
		id, at := "100", f.backupAt+int64(p)*60
		if ns == nested {
			id, at = "101", f.backupAt
			if p >= 4 {
				at += 3600
			}
		} else if mirror {
			id = "102"
		}
		if foreign {
			id, at = "200", f.backupAt
		}
		body := `{"data":[]}`
		switch {
		case strings.HasSuffix(r.URL.Path, "/version"):
			body = `{"data":{"version":"3.4.2"}}`
		case strings.HasSuffix(r.URL.Path, "/nodes/localhost/status"):
			body = `{"data":{"cpu":0.1,"memory":{"used":512,"total":1024},"uptime":120}}`
		case strings.HasSuffix(r.URL.Path, "/admin/datastore"):
			body = `{"data":[{"store":"archive"},{"store":"mirror"}]}`
		case strings.HasSuffix(r.URL.Path, "/status"):
			body = `{"data":{"status":"available","total":1000,"used":250,"avail":750}}`
		case strings.HasSuffix(r.URL.Path, "/rrd"):
			if r.URL.Query().Get("cf") != "AVERAGE" || r.URL.Query().Get("timeframe") != "hour" {
				t.Error("ordinary RRD query changed")
			}
		case strings.HasSuffix(r.URL.Path, "/gc"):
			body = `{"data":{}}`
		case strings.HasSuffix(r.URL.Path, "/nodes/localhost/tasks"):
			if r.URL.Query().Get("typefilter") != "backup" || (r.URL.Query().Get("store") != "archive" && r.URL.Query().Get("store") != "mirror") {
				t.Error("unexpected backup task query")
			}
		case strings.HasSuffix(r.URL.Path, "/namespace"):
			if !mirror {
				body = `{"data":[{"ns":"team/production"}]}`
				switch p {
				case 1:
					body = `{"data":[{"ns":"team/other","path":"team/production"}]}`
				case 2:
					body = `{"data":[{"path":"team/other","name":"team/production"}]}`
				case 3:
					body = `{"data":[{"ns":"","path":"team/other"}]}`
				case 4:
					body = `{"data":[{"ns":"","path":"/","name":"/"},{"ns":"team/production","path":"team/production","name":"team/production","parent":"team"}]}`
				case 5:
					body = `{"data":[{"path":"team/production"}]}`
				case 6:
					body = `{"data":[{"name":"team/production"}]}`
				case 7:
					body = `{"data":[]}`
				}
			}
		case strings.HasSuffix(r.URL.Path, "/groups"):
			if (ns == "" || ns == nested) && !(p == 7 && !mirror) {
				body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":%q,"backup-count":1,"last-backup":%d}]}`, id, at)
			}
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":%q,"backup-time":%d,"size":1024,"files":["index.json.blob"],"verification":{"state":"ok"}}]}`, id, at)
		default:
			t.Errorf("unexpected provider read: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
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
	if !ok {
		t.Fatal("configured PBS client absent")
	}
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
		for _, b := range f.m.PBSBackupsSnapshot() {
			if b.Instance == name {
				out = append(out, b)
			}
		}
		return out
	}
	observations := make(map[string]any)
	defer func() {
		if dir := os.Getenv("PULSE_PBS_NAMESPACE_EXPORT"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Error(err)
				return
			}
			data, err := json.MarshalIndent(observations, "", "  ")
			if err == nil {
				err = os.WriteFile(filepath.Join(dir, "namespaces.json"), data, 0600)
			}
			if err != nil {
				t.Error(err)
			}
		}
	}()
	key := pbsBackupGroupKey{datastore: "archive", namespace: nested, backupType: "vm", backupID: "101"}
	project := func(p int32) {
		snapshot := f.m.state.GetSnapshot()
		front := f.m.buildBroadcastFrontendStateFromSnapshot(snapshot, f.m.mockModeFence.begin())
		mu.Lock()
		requests := append([]string(nil), reads[p]...)
		mu.Unlock()
		observations[phases[p]] = map[string]any{
			"resources": front.Resources, "backups": rows(instance), "postures": f.postures(t), "points": f.points(t, instance),
			"policy": recovery.DefaultProtectionPosturePolicy.Payload(), "cacheRead": f.m.pbsBackupCacheTimeFor(instance, key), "requests": requests,
		}
	}
	poll(0)
	f.assertProtected(t, instance)
	initial := rows(instance)
	var held models.PBSBackup
	for _, b := range initial {
		if b.VMID == "101" {
			held = b
		}
	}
	if held.Namespace != nested || len(initial) != 3 {
		t.Fatal("initial fixture did not establish all three scopes")
	}
	oldRead := time.Now().Add(-2 * pbsBackupCacheTTL)
	f.m.setPBSBackupCacheTime(instance, key, oldRead)
	other, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL + "/other", TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Fingerprint: pin, VerifySSL: true})
	if err != nil {
		t.Fatal(err)
	}
	f.m.pollPBSBackups(context.Background(), "pbs-other", other, []models.PBSDatastore{{Name: "archive"}})
	f.wait(t)
	otherRows, otherPoints := rows("pbs-other"), f.points(t, "pbs-other")
	project(0)
	for p := int32(1); p < int32(len(phases)); p++ {
		poll(p)
		project(p)
		if !reflect.DeepEqual(rows("pbs-other"), otherRows) || !reflect.DeepEqual(f.points(t, "pbs-other"), otherPoints) {
			t.Errorf("%s: collection crossed the independent connection boundary", phases[p])
		}
		current, postures := rows(instance), f.postures(t)
		mu.Lock()
		discoveryReads := 0
		for _, read := range reads[p] {
			if strings.HasSuffix(read, "/namespace") {
				discoveryReads++
			}
		}
		mu.Unlock()
		if discoveryReads != 2 {
			t.Errorf("%s: namespace reads=%d, want one per datastore without retry", phases[p], discoveryReads)
		}
		found, mirrorAdvanced := false, false
		for _, b := range current {
			if b.VMID == "101" {
				found = true
				if p <= 3 && !reflect.DeepEqual(b, held) {
					t.Errorf("%s: ambiguous discovery changed retained backup", phases[p])
				}
				if p >= 4 && b.BackupTime.Unix() != f.backupAt+3600 {
					t.Errorf("%s: ordinary recovery did not replace the nested observation", phases[p])
				}
			}
			if b.VMID == "102" && b.BackupTime.Unix() == f.backupAt+int64(p)*60 {
				mirrorAdvanced = true
			}
		}
		if !mirrorAdvanced {
			t.Errorf("%s: readable sibling did not advance", phases[p])
		}
		if p <= 3 {
			if !found || !f.m.pbsBackupCacheTimeFor(instance, key).Equal(oldRead) {
				t.Errorf("%s: ambiguous discovery erased or renewed retained namespace evidence", phases[p])
			}
			pointFound := false
			for _, point := range f.points(t, instance) {
				if point.SubjectResourceID == f.ids[1] && point.CompletedAt != nil && point.CompletedAt.Unix() == f.backupAt {
					pointFound = true
				}
			}
			if !pointFound {
				t.Errorf("%s: ambiguous discovery reconciled away the stored recovery point", phases[p])
			}
			for _, posture := range postures {
				if posture.State != recovery.ProtectionStateUnknown || len(posture.ProviderStates) != 1 || posture.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial {
					t.Errorf("%s: ambiguous discovery certified complete protection: %+v", phases[p], posture)
				}
			}
			mu.Lock()
			for _, read := range reads[p] {
				if strings.Contains(read, "ns=") {
					t.Errorf("%s: ambiguous discovery selected a nested path: %s", phases[p], read)
				}
			}
			mu.Unlock()
		} else if p == 7 {
			if found || len(current) != 1 || len(f.points(t, instance)) != 1 || !f.m.pbsBackupCacheTimeFor(instance, key).IsZero() {
				t.Error("complete empty enumeration failed to reconcile the archive, or lost the independent mirror")
			}
		} else {
			if !found || len(current) != 3 || len(f.points(t, instance)) != 3 || !f.m.pbsBackupCacheTimeFor(instance, key).After(oldRead) {
				t.Errorf("%s: ordinary complete discovery failed to resume/renew the exact scopes", phases[p])
			}
			for _, posture := range postures {
				if posture.State != recovery.ProtectionStateProtected || len(posture.ProviderStates) != 1 || posture.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryComplete {
					t.Errorf("%s: complete discovery did not restore current protection: %+v", phases[p], posture)
				}
			}
		}
	}
}
