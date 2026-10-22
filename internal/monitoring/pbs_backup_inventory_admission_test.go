package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// Real PBS HTTP -> ordinary collector -> raw history/cache -> recovery store
// and protection projection. Nothing starts a backup or contacts an appliance.
func TestPBSBackupInventoryAdmissionPreservesHistoryAndResumes(t *testing.T) {
	f := newPBSPosturePollFixture(t, 1)
	var phase atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := phase.Load()
		var body string
		switch {
		case strings.HasSuffix(r.URL.Path, "/groups"):
			body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-count":1,"last-backup":%d}]}`, f.backupAt)
			switch p {
			case 1:
				body = `{}`
			case 2:
				w.Header().Set("Content-Length", "2000")
			case 3:
				body = strings.Replace(body, `"backup-count":1,`, "", 1)
			case 9:
				body = `{"data":[]}`
			}
		case strings.HasSuffix(r.URL.Path, "/snapshots"):
			size := 1024
			if p == 8 {
				size = 2048
			}
			body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-time":%d,"size":%d,"files":["index.json.blob"],"verification":{"state":"ok"},"comment":"guest-100"}]}`, f.backupAt, size)
			switch p {
			case 4:
				body = `{"data":null}`
			case 5:
				w.Header().Set("Content-Length", "2000")
			case 6:
				body = strings.Replace(body, `"backup-id":"100"`, `"backup-id":"101"`, 1)
			case 7:
				body = strings.Replace(body, `"state":"ok"`, `"state":"failed","state":"ok"`, 1)
			}
		default:
			t.Errorf("unexpected read: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, body)
	}))
	defer server.Close()
	var err error
	f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ds := []models.PBSDatastore{{Name: "main"}}
	poll := func(p int32, instance string) {
		phase.Store(p)
		f.m.pollPBSBackups(context.Background(), instance, f.client, ds)
		f.wait(t)
	}
	project := func(label string) {
		payload, err := json.Marshal(map[string]any{
			"backups":  f.m.state.GetSnapshot().PBSBackups,
			"postures": f.postures(t),
			"now":      time.Now().UTC(),
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("PBS_INVENTORY_PROJECTION_%s=%s", label, payload)
	}
	poll(0, "pbs-main")
	f.assertProtected(t, "pbs-main")
	initial := f.m.state.GetSnapshot().PBSBackups
	project("initial")
	key := pbsBackupGroupKey{datastore: "main", backupType: "vm", backupID: "100"}
	oldRead := time.Now().Add(-2 * pbsBackupCacheTTL)
	f.m.setPBSBackupCacheTime("pbs-main", key, oldRead)
	for _, tc := range []struct {
		label string
		phase int32
	}{
		{"missing_groups", 1}, {"truncated_groups", 2}, {"missing_count", 3},
		{"missing_snapshots", 4}, {"truncated_snapshots", 5}, {"foreign_snapshot", 6},
		{"conflicting_verification", 7},
	} {
		t.Run(tc.label, func(t *testing.T) {
			poll(tc.phase, "pbs-main")
			if !reflect.DeepEqual(f.m.state.GetSnapshot().PBSBackups, initial) || len(f.points(t, "pbs-main")) != 1 {
				t.Fatal("incomplete inventory changed historical rows or reconciled away their stored recovery point")
			}
			if actual := f.m.pbsBackupCacheTimeFor("pbs-main", key); !actual.Equal(oldRead) {
				t.Fatalf("incomplete inventory renewed cache: got %v want %v", actual, oldRead)
			}
			for _, posture := range f.postures(t) {
				if posture.State != recovery.ProtectionStateUnknown || len(posture.ProviderStates) != 1 || posture.ProviderStates[0].HistoryCompleteness == recovery.ProtectionHistoryComplete {
					t.Fatalf("incomplete inventory claimed complete protection: %+v", posture)
				}
			}
			project(tc.label)
		})
	}
	poll(8, "pbs-main")
	f.assertProtected(t, "pbs-main")
	if rows := f.m.state.GetSnapshot().PBSBackups; len(rows) != 1 || rows[0].Size != 2048 || !f.m.pbsBackupCacheTimeFor("pbs-main", key).After(oldRead) {
		t.Fatalf("ordinary complete read did not replace/renew inventory: %+v", rows)
	}
	project("resumed")
	poll(9, "pbs-main")
	if len(f.m.state.GetSnapshot().PBSBackups) != 0 || len(f.points(t, "pbs-main")) != 0 {
		t.Fatal("authoritative empty array did not withdraw deleted history")
	}
	project("withdrawn")
	// A complete empty read still cannot delete another connection's history.
	poll(0, "pbs-other")
	poll(9, "pbs-main")
	if len(f.points(t, "pbs-other")) != 1 || len(f.m.state.GetSnapshot().PBSBackups) != 1 {
		t.Fatal("empty enumeration crossed the PBS connection boundary")
	}
}
