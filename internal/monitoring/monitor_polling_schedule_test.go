package monitoring

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type planningTestProvider struct {
	testPollProvider
	listCalls int
}

func (p *planningTestProvider) ListInstances(_ *Monitor) []string {
	p.listCalls++
	return p.instances
}

func (p *planningTestProvider) FixedInstanceInterval(_ *Monitor, name string) time.Duration {
	if name == "alpha" {
		return time.Minute
	}
	return 2 * time.Minute
}

func TestPlanningListsNonBatchProviderOnceWithoutMutatingRoster(t *testing.T) {
	const providerType InstanceType = "fixture-provider"
	provider := &planningTestProvider{testPollProvider: testPollProvider{
		providerType: providerType, instances: []string{"zeta", "alpha"},
	}}
	m := &Monitor{pollProviders: map[InstanceType]PollProvider{providerType: provider}}
	got := m.describeInstancesForScheduler(t.Context())
	want := []InstanceDescriptor{
		{Name: "alpha", Type: providerType, FixedInterval: time.Minute},
		{Name: "zeta", Type: providerType, FixedInterval: 2 * time.Minute},
	}
	if !slices.Equal(got, want) {
		t.Errorf("descriptors = %+v, want %+v", got, want)
	}
	if provider.listCalls != 1 || !slices.Equal(provider.instances, []string{"zeta", "alpha"}) {
		t.Errorf("provider roster was read %d times (want one), after plan: %v", provider.listCalls, provider.instances)
	}
}

func TestPlanningPreservesBuiltinTargetOrderAndFirstDueTime(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			PBSPollingInterval: 40 * time.Second,
			PMGPollingInterval: 50 * time.Second,
			PVEPollingInterval: 30 * time.Second,
		},
		pbsClients: map[string]*pbs.Client{"zeta": nil, "alpha": nil},
		pmgClients: map[string]*pmg.Client{"zeta": nil, "alpha": nil},
		pveClients: map[string]PVEClientInterface{"zeta": nil, "alpha": nil},
	}
	now := time.Now()
	want := []ScheduledTask{
		{InstanceName: "alpha", InstanceType: InstanceTypePBS, NextRun: now, Interval: 40 * time.Second},
		{InstanceName: "zeta", InstanceType: InstanceTypePBS, NextRun: now, Interval: 40 * time.Second},
		{InstanceName: "alpha", InstanceType: InstanceTypePMG, NextRun: now, Interval: 50 * time.Second},
		{InstanceName: "zeta", InstanceType: InstanceTypePMG, NextRun: now, Interval: 50 * time.Second},
		{InstanceName: "alpha", InstanceType: InstanceTypePVE, NextRun: now, Interval: 30 * time.Second},
		{InstanceName: "zeta", InstanceType: InstanceTypePVE, NextRun: now, Interval: 30 * time.Second},
	}
	if got := m.buildScheduledTasks(now); !slices.Equal(got, want) {
		t.Fatalf("scheduled polls = %+v, want %+v", got, want)
	}
}

func TestMonitorAdaptivePlanUsesTrackedFreshnessForInventoryAndSuccessor(t *testing.T) {
	for _, name := range []string{"shared-tracker", "custom-scoring"} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				kind := InstanceTypePVE
				now := time.Now()
				tracker := NewStalenessTracker(nil)
				tracker.SetBounds(10*time.Second, time.Minute)
				tracker.setSnapshot(FreshnessSnapshot{
					InstanceType: kind, Instance: "alpha",
					LastSuccess: now.Add(-25 * time.Second), ChangeHash: "known",
				})
				tracker.setSnapshot(FreshnessSnapshot{
					InstanceType: kind, Instance: "zeta", LastError: now,
				})
				var source StalenessSource = tracker
				knownPriority, neverSucceededPriority := 25.0/60, 1.0
				if name == "custom-scoring" {
					source = mockStalenessSource{scores: map[string]float64{"pve:alpha": 0.6, "pve:zeta": 0.8}}
					knownPriority, neverSucceededPriority = 0.6, 0.8
				}
				observed := make(map[string]IntervalRequest)
				const cadence = 20 * time.Second
				m := &Monitor{
					pveClients:       map[string]PVEClientInterface{"alpha": nil, "unseen": nil, "zeta": nil},
					stalenessTracker: tracker,
					taskQueue:        NewTaskQueue(),
					scheduler: NewAdaptiveScheduler(DefaultSchedulerConfig(), source, testIntervalSelectorFunc(func(req IntervalRequest) time.Duration {
						observed[req.InstanceKey] = req
						return cadence
					}), nil),
				}
				// All three targets are due now, so priority breaks the tie.
				tasks := m.buildScheduledTasks(now)
				want := []ScheduledTask{
					{InstanceName: "zeta", InstanceType: kind, NextRun: now, Interval: cadence, Priority: neverSucceededPriority},
					{InstanceName: "alpha", InstanceType: kind, NextRun: now, Interval: cadence, Priority: knownPriority, Metadata: TaskMetadata{ChangeHash: "known"}},
					{InstanceName: "unseen", InstanceType: kind, NextRun: now, Interval: cadence},
				}
				if !slices.Equal(tasks, want) {
					t.Fatalf("inventory plan = %+v, want %+v", tasks, want)
				}
				key := schedulerKey(kind, "alpha")
				assertFreshness := func() {
					t.Helper()
					req, ok := observed[key]
					if !ok || !req.LastSuccess.Equal(now.Add(-25*time.Second)) || req.StalenessScore != knownPriority {
						t.Fatalf("interval selector received %+v (%t) for alpha", req, ok)
					}
				}
				assertFreshness()
				delete(observed, key)
				next, decision := m.nextScheduledPoll(want[1])
				wantNext := want[1]
				wantNext.NextRun = now.Add(cadence)
				if next != wantNext || decision != pollOnCadence {
					t.Fatalf("successor = %+v (decision %v), want %+v on cadence", next, decision, wantNext)
				}
				assertFreshness()
			})
		})
	}
}

func BenchmarkMonitorAdaptivePlanFreshness(b *testing.B) {
	for _, size := range []int{256, 4096} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			now := time.Now()
			tracker := NewStalenessTracker(nil)
			m := &Monitor{
				pveClients:       make(map[string]PVEClientInterface, size),
				stalenessTracker: tracker,
				scheduler: NewAdaptiveScheduler(DefaultSchedulerConfig(), tracker,
					mockIntervalSelector{30 * time.Second}, nil),
			}
			for i := range size {
				name := fmt.Sprintf("site-%04d", i)
				m.pveClients[name] = nil
				tracker.setSnapshot(FreshnessSnapshot{
					InstanceType: InstanceTypePVE, Instance: name,
					LastSuccess: now.Add(-time.Duration(i%900) * 250 * time.Millisecond),
					ChangeHash:  "synthetic",
				})
			}
			m.buildScheduledTasks(now)
			b.ReportAllocs()
			for b.Loop() {
				m.buildScheduledTasks(now)
			}
		})
	}
}

func TestMonitorAdaptivePlanReadsCurrentSuccessor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		m := newScheduleFixture(t.Context(), true, func(context.Context) ([]proxmox.Node, error) {
			return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: 0.42, Uptime: 36000}}, nil
		})
		m.pbsClients = map[string]*pbs.Client{"slow": nil}
		source := gatedStalenessSource{reading: make(chan struct{}), release: make(chan struct{})}
		m.scheduler.staleness = source
		initial := m.scheduler.BuildPlan(start, []InstanceDescriptor{
			{Name: scheduleFixtureNode, Type: InstanceTypePVE},
		}, 0)[0]
		m.taskQueue.Upsert(initial)

		time.Sleep(10 * time.Second)
		planned := make(chan []ScheduledTask, 1)
		go func() { planned <- m.buildScheduledTasks(time.Now()) }()
		defer close(source.release)
		synctest.Wait()
		select {
		case <-source.reading:
		default:
			t.Fatal("periodic plan did not reach the peer's freshness read")
		}

		// The pending first poll could not get a worker before one cadence
		// elapsed. Its real successor is therefore due at the current time,
		// not at the elapsed slot that the older planner originally saw.
		time.Sleep(40 * time.Second)
		task, ok := m.taskQueue.WaitNext(t.Context())
		if !ok {
			t.Fatal("could not claim the due PVE poll")
		}
		m.executeAndRescheduleTask(t.Context(), task, nil, 0)
		workerNext, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode)
		if !ok || workerNext.NextRun.Sub(start) != 50*time.Second || workerNext.Interval != scheduleFixtureCadence {
			t.Fatalf("worker successor = %+v (queued %v), want +50s at %s", workerNext, ok, scheduleFixtureCadence)
		}
		nodes := m.state.GetSnapshot().Nodes
		if len(nodes) != 1 || nodes[0].CPU != 0.42 {
			t.Fatalf("worker did not publish a fresh PVE reading: %+v", nodes)
		}

		// Releasing the unrelated freshness read allows the original plan
		// to take its lock after the worker has updated the schedule cache.
		source.release <- struct{}{}
		for _, next := range <-planned {
			if next.InstanceType != InstanceTypePVE {
				continue
			}
			if !next.NextRun.Equal(workerNext.NextRun) || next.Interval != workerNext.Interval {
				t.Errorf("periodic plan produced +%s at %s; worker's newer slot is +%s at %s",
					next.NextRun.Sub(start), next.Interval, workerNext.NextRun.Sub(start), workerNext.Interval)
			}
			cached, exists := m.scheduler.LastScheduled(InstanceTypePVE, scheduleFixtureNode)
			if !exists || cached != next {
				t.Errorf("cached schedule = %+v (exists %v), want %+v", cached, exists, next)
			}
			return
		}
		t.Error("periodic plan omitted the PVE poll")
	})
}

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

func TestScheduledPollWakesAnIdleWorker(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		future, adaptive, process bool
	}{
		{name: "empty-local"},
		{name: "empty-process", process: true},
		{name: "future-local", future: true, adaptive: true},
		{name: "future-process", future: true, adaptive: true, process: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var readAt time.Time
				m := newScheduleFixture(ctx, tc.adaptive, func(context.Context) ([]proxmox.Node, error) {
					readAt = time.Now()
					return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: 0.42, Uptime: 36000}}, nil
				})
				if tc.future {
					m.taskQueue.Upsert(ScheduledTask{
						InstanceType: InstanceTypePBS, InstanceName: "future-pbs",
						NextRun: time.Now().Add(time.Minute),
					})
				}
				if tc.process {
					go m.dispatchTaskWorkers(ctx, newPollTaskWorkerLimiter(1))
				} else {
					go m.taskWorker(ctx, 0)
				}
				synctest.Wait()
				time.Sleep(time.Millisecond)

				due := time.Now()
				m.poll(ctx, nil)
				synctest.Wait()
				publishedOnAdmission := len(m.state.GetSnapshot().Nodes)
				// Keep observing through the old fixed recheck period so
				// the baseline also has a comparable real poll result.
				time.Sleep(250 * time.Millisecond)
				synctest.Wait()
				nodes := m.state.GetSnapshot().Nodes
				if len(nodes) != 1 || nodes[0].CPU != 0.42 || readAt.IsZero() {
					t.Fatalf("ordinary poll did not publish its node: %+v (read at %v)", nodes, readAt)
				}
				points := m.metricsHistory.GetNodeMetrics(nodes[0].ID, "cpu", time.Minute)
				if len(points) != 1 || points[0].Value != 42 {
					t.Fatalf("ordinary poll did not publish its CPU History point: %+v", points)
				}
				t.Logf("PVE read %s after due; node CPU %.0f%%, History points %d", readAt.Sub(due), nodes[0].CPU*100, len(points))
				if publishedOnAdmission != 1 || !readAt.Equal(due) {
					t.Errorf("newly due work waited %s despite an available worker", readAt.Sub(due))
				}
				if tc.future {
					if _, ok := m.taskQueue.Get(InstanceTypePBS, "future-pbs"); !ok {
						t.Error("a future peer must remain queued")
					}
				}
			})
		})
	}
}

func TestScheduledPollWorkerKeepsCadenceAcrossWaits(t *testing.T) {
	for _, pooled := range []bool{false, true} {
		name := "local"
		if pooled {
			name = "process"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				start := time.Now()
				reads := make(chan time.Duration, 4)
				m := newScheduleFixture(ctx, false, func(context.Context) ([]proxmox.Node, error) {
					reads <- time.Since(start)
					return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", Uptime: 36000}}, nil
				})
				task := ScheduledTask{
					InstanceType: InstanceTypePVE, InstanceName: scheduleFixtureNode,
					NextRun: start.Add(scheduleFixtureCadence), Interval: scheduleFixtureCadence,
				}
				m.taskQueue.Upsert(task)
				done := make(chan struct{})
				go func() {
					defer close(done)
					if pooled {
						m.dispatchTaskWorkers(ctx, newPollTaskWorkerLimiter(1))
					} else {
						m.taskWorker(ctx, 0)
					}
				}()
				assertRead := func(want ...time.Duration) {
					t.Helper()
					synctest.Wait()
					if len(reads) != len(want) {
						t.Fatalf("at %s, observed %d reads; want %d", time.Since(start), len(reads), len(want))
					}
					for _, at := range want {
						if got := <-reads; got != at {
							t.Fatalf("poll ran at %s; want exactly %s", got, at)
						}
					}
				}
				assertRead()

				for i := 1; i <= 2; i++ {
					time.Sleep(scheduleFixtureCadence - time.Nanosecond)
					assertRead()
					time.Sleep(time.Nanosecond)
					assertRead(time.Duration(i) * scheduleFixtureCadence)
				}

				// The worker is now waiting on its third periodic slot.
				// An earlier explicit slot must still wake and rearm it.
				task.NextRun = time.Now().Add(time.Second)
				m.taskQueue.Upsert(task)
				assertRead()
				time.Sleep(time.Second)
				assertRead(2*scheduleFixtureCadence + time.Second)
				cancel()
				synctest.Wait()
				select {
				case <-done:
				default:
					t.Fatal("worker did not exit when its wait was cancelled")
				}
			})
		})
	}
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
				var reads atomic.Int32
				m := newScheduleFixture(ctx, tc.adaptive, func(context.Context) ([]proxmox.Node, error) {
					if reads.Add(1) == 1 {
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
				if n := reads.Load(); n != 1 {
					t.Fatalf("the provider was retried before its backoff elapsed: reads=%d", n)
				}
				time.Sleep(time.Millisecond)
				synctest.Wait()
				if n, nodes := reads.Load(), m.state.GetSnapshot().Nodes; n != 2 || len(nodes) != 1 || nodes[0].CPU != 0.42 {
					t.Fatalf("the retry must collect at its scheduled time: reads=%d nodes=%+v", n, nodes)
				}
			})
		})
	}
}
