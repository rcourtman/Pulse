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
	"sync"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

// Uses the real HTTP client, JSON decoder, poller and scheduler ledger. Display
// labels deliberately differ from both API node names; no PMG/guest is touched.
type pmgIdentityFixture struct {
	server *httptest.Server
	mu     sync.Mutex
	mode   string
	calls  map[string]int
}

func newPMGIdentityFixture(t *testing.T) *pmgIdentityFixture {
	t.Helper()
	f := &pmgIdentityFixture{mode: "strings", calls: map[string]int{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		mode := f.mode
		f.calls[r.URL.Path]++
		f.mu.Unlock()
		if r.Method != http.MethodGet {
			t.Errorf("unexpected mutation %s", r.Method)
			http.Error(w, "read only", 405)
			return
		}
		if r.Header.Get("Authorization") != "PMGAPIToken=reader@pmg!fixture:fixture" {
			t.Error("missing scoped fixture token")
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/version":
			if mode == "version-denied" {
				http.Error(w, "unauthorized", 401)
				return
			}
			fmt.Fprint(w, `{"data":{"version":"9.1.4"}}`)
		case "/api2/json/config/cluster/status":
			if r.URL.Query().Get("list_single_node") != "1" {
				t.Error("standalone enumeration omitted")
			}
			switch mode {
			case "cluster-error":
				http.Error(w, "fixture collection failed", 500)
			case "empty-cluster":
				fmt.Fprint(w, `{"data":[]}`)
			case "null-cluster":
				fmt.Fprint(w, `{"data":null}`)
			case "missing-name":
				fmt.Fprint(w, `{"data":[{"cid":"1","name":"mail-a"},{"cid":"2","name":" "}]}`)
			case "unused-cid":
				fmt.Fprint(w, `{"data":[{"cid":"1","name":"mail-a"},{"cid":"invalid","name":"mail-b"}]}`)
			case "malformed-name":
				fmt.Fprint(w, `{"data":[{"cid":"1","name":"mail-a"},{"cid":"2","name":42}]}`)
			case "numbers":
				fmt.Fprint(w, `{"data":[{"cid":1,"name":"mail-a","type":"master"},{"cid":2,"name":"mail-b","type":"slave"}]}`)
			case "mixed":
				fmt.Fprint(w, `{"data":[{"cid":1,"name":"mail-a","type":"master"},{"cid":"2","name":"mail-b","type":"slave"}]}`)
			default:
				fmt.Fprint(w, `{"data":[{"cid":"1","name":"mail-a","type":"master"},{"cid":"2","name":"mail-b","type":"slave"}]}`)
			}
		case "/api2/json/nodes/mail-a/postfix/queue", "/api2/json/nodes/mail-b/postfix/queue":
			if mode == "queue-denied" && strings.Contains(r.URL.Path, "mail-b") {
				http.Error(w, "forbidden", 403)
				return
			}
			fmt.Fprint(w, `{"data":{"active":5,"deferred":2,"hold":0,"incoming":1,"oldest_age":30}}`)
		case "/api2/json/nodes/mail-a/backup", "/api2/json/nodes/mail-b/backup":
			if mode == "backup-error" && strings.Contains(r.URL.Path, "mail-b") {
				http.Error(w, "fixture backup failed", 500)
				return
			}
			if mode == "empty-backups" {
				fmt.Fprint(w, `{"data":[]}`)
				return
			}
			fmt.Fprint(w, `{"data":[{"filename":"fixture-backup.tgz","size":"123","timestamp":"1700000000"}]}`)
		case "/api2/json/statistics/mail":
			fmt.Fprint(w, `{"data":{"count":"123","count_in":100,"count_out":23,"avptime":0.5}}`)
		case "/api2/json/statistics/mailcount":
			fmt.Fprint(w, `{"data":[{"time":1700000000,"count":123}]}`)
		case "/api2/json/statistics/spamscores":
			fmt.Fprint(w, `{"data":[{"level":"low","count":4}]}`)
		case "/api2/json/quarantine/spamstatus", "/api2/json/quarantine/virusstatus":
			fmt.Fprint(w, `{"data":{"count":2}}`)
		case "/api2/json/config/domains":
			fmt.Fprint(w, `{"data":[{"domain":"example.invalid"}]}`)
		case "/api2/json/statistics/domains":
			fmt.Fprint(w, `{"data":[{"domain":"example.invalid","count":123}]}`)
		default:
			t.Errorf("node routing escaped discovered inventory: %s", r.URL.Path)
			http.Error(w, "unknown node", 500)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}
func (f *pmgIdentityFixture) setMode(mode string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mode = mode
	f.calls = map[string]int{}
}
func (f *pmgIdentityFixture) requests() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]int{}
	for k, v := range f.calls {
		out[k] = v
	}
	return out
}
func (f *pmgIdentityFixture) client(t *testing.T) *pmg.Client {
	t.Helper()
	c, err := pmg.NewClient(pmg.ClientConfig{Host: f.server.URL, User: "reader@pmg", TokenName: "fixture", TokenValue: "fixture", VerifySSL: true})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestPMGDiscoveredNodeRouting(t *testing.T) {
	for _, mode := range []string{"numbers", "strings", "mixed", "unused-cid"} {
		t.Run(mode, func(t *testing.T) {
			f := newPMGIdentityFixture(t)
			f.setMode(mode)
			m := pmgScopeMonitor(allPMGScope())
			m.pollPMGInstance(context.Background(), "gateway", f.client(t))
			state := m.state.GetSnapshot()
			v := state.PMGInstances[0]
			if v.ConnectionID != "pmg:gateway" {
				t.Fatalf("configured connection identity lost: %+v", v)
			}
			if v.Status != "online" || len(v.Nodes) != 2 || v.Nodes[0].Name != "mail-a" || v.Nodes[1].Name != "mail-b" {
				t.Fatalf("node inventory = %+v", v)
			}
			for _, n := range v.Nodes {
				if n.QueueStatus == nil || n.QueueStatus.Total != 8 {
					t.Fatalf("queue = %+v", n)
				}
			}
			if len(state.PMGBackups) != 2 {
				t.Fatalf("backups = %+v", state.PMGBackups)
			}
			if v.MailStats == nil || v.MailStats.CountTotal != 123 || v.Quarantine == nil || len(v.DomainStats) != 1 {
				t.Fatal("selected readings lost")
			}
			status := m.pollStatusMap["pmg::gateway"]
			if status.ConsecutiveFailures != 0 || status.LastSuccess.IsZero() || status.LastErrorMessage != "" {
				t.Fatalf("failed collection = %+v", status)
			}
			calls := f.requests()
			if calls["/api2/json/config/cluster/status"] != 1 || calls["/api2/json/version"] != 1 {
				t.Fatalf("unexpected additional discovery: %v", calls)
			}
			for _, n := range []string{"mail-a", "mail-b"} {
				for _, path := range []string{"backup", "postfix/queue"} {
					if calls["/api2/json/nodes/"+n+"/"+path] != 1 {
						t.Fatalf("wrong node requests = %v", calls)
					}
				}
			}
		})
	}
}
func TestPMGNodeDiscoveryFailurePreservesAcceptedBackups(t *testing.T) {
	for _, mode := range []string{"cluster-error", "empty-cluster", "null-cluster", "missing-name", "malformed-name", "backup-error", "queue-denied", "version-denied"} {
		t.Run(mode, func(t *testing.T) {
			f := newPMGIdentityFixture(t)
			client := f.client(t)
			m := pmgScopeMonitor(allPMGScope())
			m.state.UpdatePMGBackups("other", []models.PMGBackup{{ID: "other-backup", Instance: "other", Node: "unrelated"}})
			m.pollPMGInstance(context.Background(), "gateway", client)
			before := m.state.GetSnapshot().PMGBackups
			sort.Slice(before, func(i, j int) bool { return before[i].ID < before[j].ID })
			success := m.pollStatusMap["pmg::gateway"].LastSuccess
			f.setMode(mode)
			m.pollPMGInstance(context.Background(), "gateway", client)
			v := m.state.GetSnapshot().PMGInstances[0]
			if v.ConnectionID != "pmg:gateway" {
				t.Fatalf("failed collection lost configured connection identity: %+v", v)
			}
			if mode != "version-denied" && (v.Status != "degraded" || v.ConnectionHealth != "degraded") {
				t.Fatalf("partial collection looks healthy: %+v", v)
			}
			status := m.pollStatusMap["pmg::gateway"]
			if status.ConsecutiveFailures != 1 || status.LastErrorMessage == "" || !status.LastSuccess.Equal(success) {
				t.Fatalf("failure hidden: %+v", status)
			}
			if strings.Contains(status.LastErrorMessage, "nodes/gateway") || strings.Contains(status.LastErrorMessage, "hostname lookup") {
				t.Fatal("display-label fallback created secondary error")
			}
			accepted := m.state.GetSnapshot().PMGBackups
			sort.Slice(accepted, func(i, j int) bool { return accepted[i].ID < accepted[j].ID })
			if !reflect.DeepEqual(accepted, before) {
				t.Fatal("incomplete read erased accepted backup inventory")
			}
			clusterFailed := mode != "backup-error" && mode != "queue-denied" && mode != "version-denied"
			if clusterFailed {
				if len(v.Nodes) != 0 {
					t.Fatalf("partial cluster accepted: %+v", v.Nodes)
				}
				for path := range f.requests() {
					if strings.Contains(path, "/nodes/") {
						t.Fatalf("node-specific request after discovery failure: %s", path)
					}
				}
				if v.MailStats == nil || v.MailStats.CountTotal != 123 {
					t.Fatal("unrelated successful collection suppressed")
				}
			}
			if mode == "queue-denied" && v.Nodes[1].QueueStatus != nil {
				t.Fatal("missing queue fabricated zero")
			}
			f.setMode("strings")
			m.pollPMGInstance(context.Background(), "gateway", client)
			if status.ConsecutiveFailures != 0 || status.LastErrorMessage != "" || len(m.state.GetSnapshot().PMGInstances[0].Nodes) != 2 {
				t.Fatal("ordinary poll did not recover")
			}
			f.setMode("empty-backups")
			m.pollPMGInstance(context.Background(), "gateway", client)
			after := m.state.GetSnapshot().PMGBackups
			if len(after) != 1 || after[0].Instance != "other" {
				b, _ := json.Marshal(after)
				t.Fatalf("complete empty response did not clear only target inventory: %s", b)
			}
		})
	}
}
