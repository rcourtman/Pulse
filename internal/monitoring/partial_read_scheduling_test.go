package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// pollAndReschedule runs one poll followed by the scheduler's reschedule step,
// as the worker loop does, and returns the task the scheduler queued next.
func pollAndReschedule(t *testing.T, m *Monitor, task ScheduledTask, poll func()) ScheduledTask {
	t.Helper()
	poll()
	m.rescheduleTask(task)
	next, ok := m.taskQueue.Get(task.InstanceType, task.InstanceName)
	if !ok {
		if m.deadLetterQueue.Size() == 0 {
			t.Fatal("poll was neither rescheduled nor dead-lettered")
		}
		return ScheduledTask{}
	}
	return next
}

func newPartialReadPMGMonitor(t *testing.T) (*Monitor, ScheduledTask) {
	t.Helper()
	cfg := allPMGScope()
	cfg.Name = "gateway"
	m, err := New(&config.Config{PMGInstances: []config.PMGInstance{cfg}})
	if err != nil {
		t.Fatalf("failed to create monitor: %v", err)
	}
	interval := m.baseIntervalForInstanceType(InstanceTypePMG)
	if interval <= 0 {
		t.Fatalf("no PMG base interval: %v", interval)
	}
	return m, ScheduledTask{InstanceName: "gateway", InstanceType: InstanceTypePMG, Interval: interval, NextRun: time.Now()}
}

// Issue #2960: one failed optional PMG read must not back off or dead-letter
// the connection. The failed reading stays missing and the error stays on the
// connection, while every other reading keeps its normal cadence.
func TestPMGOptionalReadFailureKeepsNormalSchedule(t *testing.T) {
	for _, op := range []string{"cluster", "backup/one", "queue/one", "mail", "count", "scores", "quarantine/spam", "quarantine/virus", "domains", "domain-stats"} {
		t.Run(op, func(t *testing.T) {
			m, task := newPartialReadPMGMonitor(t)
			f := &pmgScopeFixture{calls: map[string]int{}, fail: op}

			for i := 0; i <= m.maxRetryAttempts+1; i++ {
				next := pollAndReschedule(t, m, task, func() { m.pollPMGInstance(context.Background(), "gateway", f) })
				if next.Interval != task.Interval {
					t.Fatalf("poll %d: interval=%v, want normal %v", i, next.Interval, task.Interval)
				}
				task = next
			}
			if size := m.deadLetterQueue.Size(); size != 0 {
				t.Fatalf("optional read failure reached the dead-letter queue (%d tasks)", size)
			}
			if state, failures, _ := m.ensureBreaker(schedulerKey(InstanceTypePMG, "gateway")).State(); state != "closed" || failures != 0 {
				t.Fatalf("breaker=%s failures=%d after optional read failures", state, failures)
			}

			status := m.pollStatusMap["pmg::gateway"]
			if status.ConsecutiveFailures != 0 || status.LastSuccess.IsZero() {
				t.Fatalf("partial poll counted as a connection failure: %+v", status)
			}
			if status.LastErrorMessage == "" || status.LastErrorCategory != pollReadErrorCategory {
				t.Fatalf("failed read not left visible on the connection: %+v", status)
			}

			v := m.state.GetSnapshot().PMGInstances[0]
			if v.Status != "online" || v.ConnectionHealth != "healthy" {
				t.Fatalf("partial poll rewrote connection health: %s/%s", v.Status, v.ConnectionHealth)
			}
			nodeQueue := func(i int) bool { return len(v.Nodes) > i && v.Nodes[i].QueueStatus != nil }
			published := map[string]bool{
				"mail":         v.MailStats != nil,
				"count":        len(v.MailCount) > 0,
				"scores":       len(v.SpamDistribution) > 0,
				"quarantine":   v.Quarantine != nil,
				"domains":      len(v.RelayDomains) > 0,
				"domain-stats": len(v.DomainStats) > 0,
				"queue/one":    nodeQueue(0),
				"queue/two":    nodeQueue(1),
			}
			failed := map[string]bool{op: true}
			switch {
			case op == "cluster":
				// Node queues are read per discovered node.
				failed = map[string]bool{"queue/one": true, "queue/two": true}
			case strings.HasPrefix(op, "quarantine/"):
				// Totals require both categories.
				failed = map[string]bool{"quarantine": true}
			}
			for reading, ok := range published {
				if failed[reading] && ok {
					t.Errorf("failed %s reading was published", reading)
				}
				if !failed[reading] && !ok {
					t.Errorf("%s reading was not published beside the failed %s read", reading, op)
				}
			}

			// A complete poll clears the outstanding read error.
			f.fail = ""
			m.pollPMGInstance(context.Background(), "gateway", f)
			if status.LastErrorMessage != "" || status.LastErrorCategory != "" {
				t.Fatalf("complete poll left the read error: %+v", status)
			}
		})
	}
}

// Connection failures keep the scheduler's backoff and dead-letter handling.
func TestPMGVersionFailureStillBacksOffAndDeadLetters(t *testing.T) {
	m, task := newPartialReadPMGMonitor(t)
	f := &pmgScopeFixture{calls: map[string]int{}, fail: "version"}

	next := pollAndReschedule(t, m, task, func() { m.pollPMGInstance(context.Background(), "gateway", f) })
	if next.Interval == task.Interval {
		t.Fatalf("version failure kept the normal interval %v", next.Interval)
	}
	if status := m.pollStatusMap["pmg::gateway"]; status.ConsecutiveFailures != 1 || status.LastErrorCategory == pollReadErrorCategory {
		t.Fatalf("version failure not recorded as a connection failure: %+v", status)
	}
	for i := 0; i < m.maxRetryAttempts && m.deadLetterQueue.Size() == 0; i++ {
		pollAndReschedule(t, m, task, func() { m.pollPMGInstance(context.Background(), "gateway", f) })
	}
	if m.deadLetterQueue.Size() != 1 {
		t.Fatal("repeated version failures never reached the dead-letter queue")
	}
}

// Reads cut short by the poll's own cancellation are a whole-poll failure,
// not one broken endpoint, and keep their transient backoff.
func TestPMGCancelledPollWithFailedReadsBacksOff(t *testing.T) {
	m, task := newPartialReadPMGMonitor(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &pmgScopeFixture{calls: map[string]int{}, fail: "mail", onVersion: cancel}

	next := pollAndReschedule(t, m, task, func() { m.pollPMGInstance(ctx, "gateway", f) })
	if next.Interval == task.Interval || m.deadLetterQueue.Size() != 0 {
		t.Fatalf("cancelled poll interval=%v dlq=%d, want transient backoff", next.Interval, m.deadLetterQueue.Size())
	}
	if status := m.pollStatusMap["pmg::gateway"]; status.ConsecutiveFailures != 1 {
		t.Fatalf("cancelled poll not recorded as a failure: %+v", status)
	}
}

// PBS keeps optional read failures (datastores, node status) out of the
// scheduler outcome already; pin that so a future change cannot reintroduce
// the PMG coupling behind #2960.
func TestPBSOptionalReadFailureKeepsNormalSchedule(t *testing.T) {
	for _, failing := range []string{"/api2/json/admin/datastore", "/api2/json/nodes/localhost/status"} {
		t.Run(strings.TrimPrefix(failing, "/api2/json/"), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == failing {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, `{"message":"fixture read refused","data":null}`)
					return
				}
				switch r.URL.Path {
				case "/api2/json/version":
					fmt.Fprint(w, `{"data":{"version":"4.0.1","release":"1","repoid":"fixture"}}`)
				case "/api2/json/nodes/localhost/status":
					fmt.Fprint(w, `{"data":{"cpu":0.25,"memory":{"used":512,"total":1024},"uptime":100}}`)
				case "/api2/json/admin/datastore":
					fmt.Fprint(w, `{"data":[]}`)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
			if err != nil {
				t.Fatal(err)
			}
			m, err := New(&config.Config{PBSInstances: []config.PBSInstance{{
				Name: "pbs-main", Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", MonitorDatastores: true,
			}}})
			if err != nil {
				t.Fatalf("failed to create monitor: %v", err)
			}
			task := ScheduledTask{InstanceName: "pbs-main", InstanceType: InstanceTypePBS, Interval: m.baseIntervalForInstanceType(InstanceTypePBS), NextRun: time.Now()}

			for i := 0; i <= m.maxRetryAttempts+1; i++ {
				next := pollAndReschedule(t, m, task, func() { m.pollPBSInstance(context.Background(), "pbs-main", client) })
				if next.Interval != task.Interval {
					t.Fatalf("poll %d: interval=%v, want normal %v", i, next.Interval, task.Interval)
				}
				task = next
			}
			if size := m.deadLetterQueue.Size(); size != 0 {
				t.Fatalf("optional read failure reached the dead-letter queue (%d tasks)", size)
			}
			if status := m.pollStatusMap["pbs::pbs-main"]; status == nil || status.ConsecutiveFailures != 0 {
				t.Fatalf("optional read failure counted as a poll failure: %+v", status)
			}

			inst := pbsInstanceByName(t, m.state.GetSnapshot(), "pbs-main")
			if inst.Status != "online" || inst.Version != "4.0.1" {
				t.Fatalf("connection readings not published: %+v", inst)
			}
			nodeStatusFailed := failing == "/api2/json/nodes/localhost/status"
			if inst.NodeMetricsUnavailable != nodeStatusFailed {
				t.Fatalf("node metrics unavailable=%v with node status failed=%v", inst.NodeMetricsUnavailable, nodeStatusFailed)
			}
			if !nodeStatusFailed && inst.CPU != 0.25 {
				t.Fatalf("node CPU not published beside the failed datastore read: %v", inst.CPU)
			}
		})
	}
}
