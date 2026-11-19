package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rs/zerolog"
)

type pmgNodeQueuePhase struct {
	name                    string
	active, deferred, hold  int
	deferredBucket          string
	unread                  bool
	missingTotal, nullTotal string
	wantAgeSeconds          int64
	wantAge, wantAggregate  alerts.AlertLevel
	keepAge, resolvedAge    bool
	wantCounts              []string
}

type pmgNodeQueueScenario struct {
	name    string
	warning int
	phases  []pmgNodeQueuePhase
	// Count fixtures intentionally also seed gateway-wide incidents.
	checkCountRecovery bool
}

func pmgNodeQueueScenarios() []pmgNodeQueueScenario {
	allCounts := []string{"queue-deferred", "queue-hold", "queue-total"}
	scenarios := []pmgNodeQueueScenario{
		{name: "default", phases: []pmgNodeQueuePhase{
			{name: "independent", active: 1, deferred: 24, hold: 8, deferredBucket: "40m", wantAgeSeconds: 1200,
				wantAge: alerts.AlertLevelWarning, wantCounts: []string{"queue-deferred", "queue-hold"}},
			{name: "all-high", active: 43, deferred: 24, hold: 8, deferredBucket: "40m", wantAgeSeconds: 1200,
				wantAge: alerts.AlertLevelWarning, wantCounts: allCounts},
			{name: "unread-high", active: 43, deferred: 24, hold: 8, deferredBucket: "40m", unread: true,
				wantAge: alerts.AlertLevelWarning, keepAge: true, wantCounts: allCounts},
			{name: "ambiguous", deferred: 1, hold: 1, deferredBucket: "20m", wantAgeSeconds: 600,
				wantAge: alerts.AlertLevelWarning, keepAge: true},
			{name: "unread-ambiguous", deferred: 1, hold: 1, deferredBucket: "20m", unread: true,
				wantAge: alerts.AlertLevelWarning, keepAge: true},
			{name: "young", deferred: 1, hold: 1, deferredBucket: "5m", resolvedAge: true},
			{name: "drained", resolvedAge: true},
		}},
		// The existing 60% node scale turns a five-minute setting into
		// three minutes. A 0–4m observation can only clear the gateway.
		{name: "three-minute", warning: 5, phases: []pmgNodeQueuePhase{
			{name: "seed", deferred: 1, hold: 1, deferredBucket: "10m", wantAgeSeconds: 300,
				wantAge: alerts.AlertLevelWarning, wantAggregate: alerts.AlertLevelWarning},
			{name: "ambiguous", deferred: 1, hold: 1, deferredBucket: "5m", wantAge: alerts.AlertLevelWarning, keepAge: true},
			{name: "unread", deferred: 1, hold: 1, deferredBucket: "5m", unread: true, wantAge: alerts.AlertLevelWarning, keepAge: true},
			{name: "unaccounted", deferred: 1, hold: 1, wantAge: alerts.AlertLevelWarning, keepAge: true},
			{name: "held-only", hold: 1, resolvedAge: true},
		}},
		{name: "severity", phases: []pmgNodeQueuePhase{
			{name: "warning", active: 43, deferred: 24, hold: 8, deferredBucket: "80m", wantAgeSeconds: 2400,
				wantAge: alerts.AlertLevelWarning, wantAggregate: alerts.AlertLevelWarning, wantCounts: allCounts},
			{name: "critical", active: 43, deferred: 24, hold: 8, deferredBucket: "160m", wantAgeSeconds: 4800,
				wantAge: alerts.AlertLevelCritical, wantAggregate: alerts.AlertLevelCritical, wantCounts: allCounts},
			{name: "could-still-be-critical", active: 43, deferred: 24, hold: 8, deferredBucket: "80m", wantAgeSeconds: 2400,
				wantAge: alerts.AlertLevelCritical, keepAge: true, wantAggregate: alerts.AlertLevelCritical, wantCounts: allCounts},
			// 20–39m is fully above the scaled node warning (18m),
			// but still straddles the gateway warning (30m).
			{name: "confirmed-warning", active: 43, deferred: 24, hold: 8, deferredBucket: "40m", wantAgeSeconds: 1200,
				wantAge: alerts.AlertLevelWarning, wantAggregate: alerts.AlertLevelCritical, wantCounts: allCounts},
		}},
	}
	for _, queue := range []string{"deferred", "hold"} {
		partial := pmgNodeQueuePhase{
			wantCounts: []string{"queue-" + queue, "queue-total"},
		}
		if queue == "deferred" {
			partial.hold = 1
		} else {
			partial.deferred, partial.deferredBucket = 1, "5m"
		}
		for _, missing := range []bool{true, false} {
			sample := partial
			var shape string
			if missing {
				sample.missingTotal, shape = queue, "missing"
			} else {
				sample.nullTotal, shape = queue, "null"
			}
			sample.name = "partial"
			scenarios = append(scenarios, pmgNodeQueueScenario{
				name: "count-" + queue + "-" + shape, checkCountRecovery: true,
				phases: []pmgNodeQueuePhase{
					{name: "seed", active: 90, deferred: 27, hold: 12, deferredBucket: "5m", wantCounts: allCounts},
					sample,
					{name: "drained"},
				},
			})
		}
	}
	return scenarios
}

// Run ordinary client reads and the real PMG poll; the response of each qshape
// endpoint is the only input that changes between polls in a scenario.
func newPMGNodeQueuePoller(t *testing.T, warning int) (*alerts.Manager, func(pmgNodeQueuePhase) (*models.PMGQueueStatus, int32)) {
	t.Helper()
	manager, poll := newPMGQueuePoller(t, warning, "relay")
	return manager, func(input pmgNodeQueuePhase) (*models.PMGQueueStatus, int32) {
		nodes, reads := poll(input)
		return nodes[0].QueueStatus, reads
	}
}

func newPMGQueuePoller(t *testing.T, warning int, names ...string) (*alerts.Manager, func(...pmgNodeQueuePhase) ([]models.PMGNodeStatus, int32)) {
	t.Helper()
	t.Setenv("PULSE_DATA_DIR", t.TempDir())
	cluster := make([]map[string]string, 0, len(names))
	for _, name := range names {
		cluster = append(cluster, map[string]string{"name": name, "type": "master"})
	}
	var phase atomic.Pointer[map[string]pmgNodeQueuePhase]
	var qshape atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected fixture method %s", r.Method)
			http.Error(w, "only fixture reads", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/version":
			fmt.Fprint(w, `{"data":{"version":"9.0-fixture"}}`)
			return
		case "/api2/json/config/cluster/status":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": cluster})
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api2/json/nodes/")
		name, endpoint, _ := strings.Cut(path, "/")
		s, ok := (*phase.Load())[name]
		if !ok {
			t.Errorf("unexpected fixture path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		switch endpoint {
		case "backup":
			fmt.Fprint(w, `{"data":[]}`)
		case "postfix/qshape":
			qshape.Add(1)
			n, bucket := 0, "5m"
			queue := r.URL.Query().Get("queue")
			switch queue {
			case "active":
				n = s.active
			case "deferred":
				n, bucket = s.deferred, s.deferredBucket
			case "hold":
				if s.unread {
					http.Error(w, "fixture queue unreadable", http.StatusServiceUnavailable)
					return
				}
				n, bucket = s.hold, "1280m+"
			case "incoming":
			default:
				t.Errorf("unexpected qshape query %s", r.URL.RawQuery)
				http.Error(w, "unknown fixture queue", http.StatusBadRequest)
				return
			}
			row := map[string]any{"domain": "TOTAL", "total": n}
			if bucket != "" {
				row[bucket] = n
			}
			switch queue {
			case s.missingTotal:
				delete(row, "total")
			case s.nullTotal:
				row["total"] = nil
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{row}})
		default:
			t.Errorf("unexpected fixture path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client, err := pmg.NewClient(pmg.ClientConfig{Host: server.URL, TokenName: "fixture@pmg!reader", TokenValue: "synthetic-only"})
	if err != nil {
		t.Fatal(err)
	}
	m := pmgScopeMonitor(config.PMGInstance{Host: "https://pmg.example.invalid", MonitoringConfigured: true, MonitorQueues: true})
	m.alertManager = alerts.NewManager()
	t.Cleanup(m.alertManager.Stop)
	cfg := m.alertManager.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationActive
	cfg.PMGDefaults.QueueTotalWarning, cfg.PMGDefaults.QueueTotalCritical = 100, 200
	cfg.PMGDefaults.DeferredQueueWarn, cfg.PMGDefaults.DeferredQueueCritical = 25, 30
	cfg.PMGDefaults.HoldQueueWarn, cfg.PMGDefaults.HoldQueueCritical = 10, 20
	if warning > 0 {
		cfg.PMGDefaults.OldestMessageWarnMins = warning
	}
	m.alertManager.UpdateConfig(cfg)
	logger := zerolog.New(io.Discard)
	return m.alertManager, func(input ...pmgNodeQueuePhase) ([]models.PMGNodeStatus, int32) {
		t.Helper()
		if len(input) != len(names) {
			t.Fatalf("fixture received %d observations for %d nodes", len(input), len(names))
		}
		samples := make(map[string]pmgNodeQueuePhase, len(names))
		for i, name := range names {
			samples[name] = input[i]
		}
		phase.Store(&samples)
		before := qshape.Load()
		m.pollPMGInstance(logger.WithContext(context.Background()), "gateway", client)
		instances := m.state.GetSnapshot().PMGInstances
		if len(instances) != 1 || len(instances[0].Nodes) != len(names) || instances[0].Status != "online" {
			t.Fatalf("unexpected PMG publication: %+v", instances)
		}
		return instances[0].Nodes, qshape.Load() - before
	}
}

func TestPMGNodeQueuePollRecovery(t *testing.T) {
	for _, scenario := range pmgNodeQueueScenarios() {
		t.Run(scenario.name, func(t *testing.T) {
			manager, poll := newPMGNodeQueuePoller(t, scenario.warning)
			var priorAge *alerts.Alert
			var seededCounts map[string]alerts.Alert
			for _, input := range scenario.phases {
				queue, reads := poll(input)
				if input.unread {
					if queue != nil || reads != 3 {
						t.Fatalf("%s: unread queue published as %+v, reads=%d", input.name, queue, reads)
					}
				} else if queue == nil || reads != 4 || queue.Total != input.active+input.deferred+input.hold ||
					queue.Deferred != input.deferred || queue.Hold != input.hold || queue.OldestAge != input.wantAgeSeconds {
					t.Fatalf("%s: unexpected queue projection %+v, reads=%d", input.name, queue, reads)
				}

				var counts, aggregateCounts []string
				var age *alerts.Alert
				var ageLevel, aggregate alerts.AlertLevel
				activeCounts := make(map[string]alerts.Alert)
				for _, alert := range manager.GetActiveAlerts() {
					if alert.Type == "message-age" {
						if alert.Node == "relay" {
							if age != nil {
								t.Errorf("%s: duplicate node age alert %+v", input.name, alert)
							}
							age, ageLevel = &alert, alert.Level
						} else {
							if aggregate != "" {
								t.Errorf("%s: duplicate aggregate age alert %+v", input.name, alert)
							}
							aggregate = alert.Level
						}
					} else if alert.Node == "relay" {
						counts = append(counts, alert.Type)
						activeCounts[alert.ID] = alert
						if (alert.Level == alerts.AlertLevelCritical) != (alert.Type == "queue-deferred") {
							t.Errorf("%s: unexpected count alert %+v", input.name, alert)
						}
					} else {
						metric := alert.Type
						if metric == "queue-depth" {
							metric = "queue-total" // The existing aggregate total kind.
						}
						aggregateCounts = append(aggregateCounts, metric)
						activeCounts[alert.ID] = alert
					}
				}
				slices.Sort(counts)
				slices.Sort(aggregateCounts)
				if !slices.Equal(counts, input.wantCounts) || ageLevel != input.wantAge || aggregate != input.wantAggregate {
					t.Errorf("%s: active counts %v / node age %q / aggregate age %q; want %v / %q / %q",
						input.name, counts, ageLevel, aggregate, input.wantCounts, input.wantAge, input.wantAggregate)
				}
				if input.keepAge && (priorAge == nil || !reflect.DeepEqual(priorAge, age)) {
					t.Errorf("%s: uncertain read did not retain the previous age occurrence", input.name)
				}
				resolved := manager.GetRecentlyResolved()
				nodeAgeResolved := slices.ContainsFunc(resolved, func(a models.ResolvedAlert) bool {
					return a.Type == "message-age" && a.Node == "relay"
				})
				if nodeAgeResolved != input.resolvedAge {
					t.Errorf("%s: unexpected resolved age occurrence presence %v", input.name, nodeAgeResolved)
				}
				if !scenario.checkCountRecovery {
					if len(aggregateCounts) != 0 {
						t.Errorf("%s: unexpected aggregate count alerts %v", input.name, aggregateCounts)
					}
				} else {
					if !slices.Equal(aggregateCounts, input.wantCounts) {
						t.Errorf("%s: active aggregate counts %v; want %v", input.name, aggregateCounts, input.wantCounts)
					}
					if input.name == "seed" {
						seededCounts = activeCounts
					}
					for id, a := range activeCounts {
						if !reflect.DeepEqual(a, seededCounts[id]) {
							t.Errorf("%s: changed prior evidence for %s", input.name, id)
						}
					}
					resolutions := make(map[string]struct{})
					for _, a := range resolved {
						resolutions[a.ID] = struct{}{}
					}
					for id := range seededCounts {
						_, cleared := resolutions[id]
						_, active := activeCounts[id]
						if active == cleared {
							t.Errorf("%s: %s active=%v and resolved=%v; want exactly one", input.name, id, active, cleared)
						}
					}
					if len(activeCounts)+len(resolutions) != len(seededCounts) {
						t.Errorf("%s: counts changed incident identities or did not resolve exactly once", input.name)
					}
				}
				priorAge = age
			}
		})
	}
}
