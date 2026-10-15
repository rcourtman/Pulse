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
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Exercise the real HTTP producer and the state replacement/cache consumers.
// No guest command, backup, database, native host or recovery action is used.
func TestPVEBackupTasksPublishOnlyCompletedEvidence(t *testing.T) {
	var phase atomic.Int32
	const now = int64(1792028400) // fixed synthetic task time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"},{"node":"pve2","status":"online"}]}`)
		case "/api2/json/nodes/pve1/tasks":
			fmt.Fprintf(w, `{"data":[{"upid":"single","node":"pve1","type":"vzdump","id":"100","status":"OK","starttime":%d,"endtime":%d}]}`, now-120, now)
		case "/api2/json/nodes/pve2/tasks":
			if phase.Load() == 1 {
				w.WriteHeader(503)
				fmt.Fprint(w, `{"errors":"fixture unavailable"}`)
				return
			}
			if phase.Load() == 2 {
				w.Header().Set("Content-Length", "1000")
				fmt.Fprint(w, `{"data":[]}`)
				return
			}
			fmt.Fprintf(w, `{"data":[{"upid":"running","node":"pve2","type":"vzdump","id":"101","starttime":%d}]}`, now)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	m := &Monitor{state: models.NewState()}
	m.pollBackupTasks(context.Background(), "proof", client)
	initial := m.state.GetSnapshot().PVEBackups
	sort.Slice(initial.BackupTasks, func(i, j int) bool { return initial.BackupTasks[i].ID < initial.BackupTasks[j].ID })
	if len(initial.BackupTasks) != 2 {
		t.Fatalf("initial tasks: %+v", initial.BackupTasks)
	}
	project := func(label string) {
		v := m.state.GetSnapshot().PVEBackups
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		t.Logf("PVE_TASK_PROJECTION_%s=%s", label, b)
	}
	project("initial")
	for _, p := range []int32{1, 2} {
		phase.Store(p)
		m.pollBackupTasks(context.Background(), "proof", client)
		actual := m.state.GetSnapshot().PVEBackups
		sort.Slice(actual.BackupTasks, func(i, j int) bool { return actual.BackupTasks[i].ID < actual.BackupTasks[j].ID })
		if !reflect.DeepEqual(actual, initial) {
			t.Errorf("incomplete phase %d replaced rows/original observation times: before=%+v after=%+v", p, initial, actual)
		}
	}
	project("retained")
	phase.Store(0)
	m.pollBackupTasks(context.Background(), "proof", client)
	recovered := m.state.GetSnapshot().PVEBackups.BackupTasks
	sort.Slice(recovered, func(i, j int) bool { return recovered[i].ID < recovered[j].ID })
	if len(recovered) != 2 || !recovered[0].ObservedAt.After(initial.BackupTasks[0].ObservedAt) {
		t.Fatalf("ordinary complete poll did not resume: %+v", recovered)
	}
	project("resumed")
}

func TestPVEFinishedLogRejectionDoesNotPoisonCache(t *testing.T) {
	var healthy atomic.Bool
	var logReads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"}]}`)
		case r.URL.Path == "/api2/json/nodes/pve1/tasks":
			fmt.Fprint(w, `{"data":[{"upid":"job","node":"pve1","type":"vzdump","status":"OK","starttime":1792028280,"endtime":1792028400}]}`)
		case strings.HasSuffix(r.URL.Path, "/log"):
			logReads.Add(1)
			if !healthy.Load() {
				w.Header().Set("Content-Length", "1000")
			}
			fmt.Fprint(w, `{"data":[{"n":1,"t":"INFO: Starting Backup of VM 100 (qemu)"},{"n":2,"t":"INFO: Finished Backup of VM 100 (00:01:00)"}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	m := &Monitor{state: models.NewState()}
	m.pollBackupTasks(context.Background(), "proof", client)
	if got := m.state.GetSnapshot().PVEBackups.BackupTasks; len(got) != 1 || got[0].VMID != 0 || len(m.vzdumpJobTaskCache) != 0 {
		t.Errorf("incomplete finished log published/cached guest success: %+v", got)
	}
	healthy.Store(true)
	m.pollBackupTasks(context.Background(), "proof", client)
	if got := m.state.GetSnapshot().PVEBackups.BackupTasks; len(got) != 2 || logReads.Load() != 2 {
		t.Errorf("ordinary complete log did not repair cache: %+v reads=%d", got, logReads.Load())
	}
	m.pollBackupTasks(context.Background(), "proof", client)
	if logReads.Load() != 2 {
		t.Error("completed immutable log unnecessarily refetched")
	}
}
