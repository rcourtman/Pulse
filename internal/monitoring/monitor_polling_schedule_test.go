package monitoring

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const (
	scheduleFixtureNode    = "fixture-pve"
	scheduleFixtureCadence = 45 * time.Second
)

func newScheduleFixture(ctx context.Context, adaptive bool, readNodes func(context.Context) ([]proxmox.Node, error)) *Monitor {
	now := time.Now()
	physicalDisks := false
	m := &Monitor{
		config: &config.Config{PVEPollingInterval: scheduleFixtureCadence, AdaptivePollingEnabled: adaptive,
			AdaptivePollingMaxInterval: scheduleFixtureCadence, PVEInstances: []config.PVEInstance{{
				Name: scheduleFixtureNode, Host: "https://fixture-pve.invalid:8006", MonitorPhysicalDisks: &physicalDisks,
			}}},
		state: models.NewState(), alertManager: &alerts.Manager{}, runtimeCtx: ctx,
		taskQueue: NewTaskQueue(), startTime: now,
		metricsHistory: NewMetricsHistory(16, time.Hour),
		pollStatusMap:  make(map[string]*pollStatus),
		lastOutcome:    make(map[string]taskOutcome), failureCounts: make(map[string]int),
		lastClusterCheck: map[string]time.Time{scheduleFixtureNode: now},
		nodeLastOnline:   make(map[string]time.Time), nodeRRDMemCache: make(map[string]rrdMemCacheEntry),
		pveClients: map[string]PVEClientInterface{
			scheduleFixtureNode: &pollWorkFixturePVE{getNodes: readNodes},
		},
		backoffCfg: backoffConfig{Initial: 90 * time.Second}, maxRetryAttempts: 5,
	}
	if adaptive {
		m.scheduler = NewAdaptiveScheduler(SchedulerConfig{
			BaseInterval: scheduleFixtureCadence, MinInterval: scheduleFixtureCadence, MaxInterval: scheduleFixtureCadence,
		}, nil, nil, nil)
	}
	m.executor = newRealExecutor(m)
	return m
}

func runInitialScheduleFixturePoll(t *testing.T, ctx context.Context, m *Monitor) {
	t.Helper()
	m.poll(ctx, nil)
	task, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode)
	if !ok || task.NextRun.After(time.Now()) {
		t.Fatalf("initial poll must be immediately eligible: %+v, queued=%v", task, ok)
	}
	task, ok = m.taskQueue.WaitNext(ctx)
	if !ok {
		t.Fatal("could not claim the first planned poll")
	}
	m.executeAndRescheduleTask(ctx, task, nil, 0)
}

type lateScheduledPollFixture struct {
	before               models.StateSnapshot
	after                models.StateSnapshot
	scheduled, replanned time.Duration // relative to the first poll
	observed             time.Time
	calls                int
}

func observeLateScheduledPoll(t *testing.T, adaptive, shared bool, observe func(*testing.T, *Monitor, lateScheduledPollFixture)) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		start := time.Now()
		calls := 0
		m := newScheduleFixture(ctx, adaptive, func(context.Context) ([]proxmox.Node, error) {
			calls++
			cpu := 0.24
			if calls > 1 {
				cpu = 0.42
			}
			return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: cpu, Uptime: 36000}}, nil
		})
		runInitialScheduleFixturePoll(t, ctx, m)
		before := m.state.GetSnapshot()
		if len(before.Nodes) != 1 || before.Nodes[0].CPU != 0.24 {
			t.Fatalf("first scheduled poll did not publish 24%% CPU: %+v", before.Nodes)
		}
		pending, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode)
		if !ok || pending.NextRun.Sub(start) != scheduleFixtureCadence {
			t.Fatalf("expected the next poll at +%s: %+v, queued=%v", scheduleFixtureCadence, pending, ok)
		}

		// Model a worker not being available by the due tick: leave this
		// task in the real queue through two ordinary planning passes.
		time.Sleep(scheduleFixtureCadence)
		m.poll(ctx, nil)
		time.Sleep(2 * time.Second)
		m.poll(ctx, nil)
		replanned, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode)
		if !ok {
			t.Fatal("replanning removed the pending poll")
		}

		if shared {
			go m.dispatchTaskWorkers(ctx, newPollTaskWorkerLimiter(1))
		} else {
			go m.taskWorker(ctx, 0)
		}
		synctest.Wait()
		observe(t, m, lateScheduledPollFixture{
			before: before, after: m.state.GetSnapshot(), scheduled: pending.NextRun.Sub(start),
			replanned: replanned.NextRun.Sub(start), observed: time.Now(), calls: calls,
		})
	})
}

func TestDueScheduledPollSurvivesPlanningTicks(t *testing.T) {
	for _, tc := range []struct {
		name             string
		adaptive, shared bool
	}{
		{name: "fixed-local"},
		{name: "fixed-process", shared: true},
		{name: "adaptive-local", adaptive: true},
		{name: "adaptive-process", adaptive: true, shared: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observeLateScheduledPoll(t, tc.adaptive, tc.shared, func(t *testing.T, m *Monitor, observed lateScheduledPollFixture) {
				node := observed.after.Nodes[0]
				points := m.metricsHistory.GetNodeMetrics(node.ID, "cpu", time.Minute)
				t.Logf("due=%s after two replans=%s PVE reads=%d CPU=%.0f%% History points=%d",
					observed.scheduled, observed.replanned, observed.calls, node.CPU*100, len(points))
				if observed.replanned != observed.scheduled {
					t.Errorf("planning moved an unclaimed due poll from +%s to +%s", observed.scheduled, observed.replanned)
				}
				if observed.calls != 2 || node.CPU != 0.42 || len(points) != 2 || points[1].Value != 42 {
					t.Fatalf("available worker must publish the next PVE reading and History point: calls=%d CPU=%.2f points=%+v", observed.calls, node.CPU, points)
				}
				if next, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode); !ok || !next.NextRun.After(observed.observed) {
					t.Fatalf("late work should return to its ordinary cadence, got %+v, queued=%v", next, ok)
				}
			})
		})
	}
}

func TestScheduledRetryKeepsWorkerBackoffAcrossPlanningTicks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		adaptive bool
	}{
		{name: "fixed"},
		{name: "adaptive", adaptive: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				reads := 0
				m := newScheduleFixture(ctx, tc.adaptive, func(context.Context) ([]proxmox.Node, error) {
					reads++
					if reads == 1 {
						return nil, context.DeadlineExceeded
					}
					return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: 0.42, Uptime: 36000}}, nil
				})
				start := time.Now()
				runInitialScheduleFixturePoll(t, ctx, m)
				retry, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode)
				if !ok || retry.NextRun.Sub(start) != 90*time.Second {
					t.Fatalf("the worker should queue the normal 90s retry: %+v, queued=%v", retry, ok)
				}
				time.Sleep(time.Second)
				m.poll(ctx, nil)
				replanned, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode)
				if !ok || replanned.NextRun != retry.NextRun {
					t.Errorf("planning changed a recorded failure's retry from +%s to +%s, queued=%v",
						retry.NextRun.Sub(start), replanned.NextRun.Sub(start), ok)
				}
				go m.taskWorker(ctx, 0)
				time.Sleep(89*time.Second - time.Millisecond)
				synctest.Wait()
				if reads != 1 {
					t.Fatalf("the provider was retried before its backoff elapsed: reads=%d", reads)
				}
				time.Sleep(time.Millisecond)
				synctest.Wait()
				if nodes := m.state.GetSnapshot().Nodes; reads != 2 || len(nodes) != 1 || nodes[0].CPU != 0.42 {
					t.Fatalf("the retry must collect at its scheduled time: reads=%d nodes=%+v", reads, nodes)
				}
			})
		})
	}
}
