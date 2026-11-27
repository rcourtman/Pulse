package monitoring

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func captureDeadLetterHealth(t *testing.T, m *Monitor, phase string) SchedulerHealthResponse {
	t.Helper()
	health := m.SchedulerHealth()
	body, err := json.Marshal(health)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("SCHEDULER_HEALTH %s %s", phase, body)
	return health
}

func deadLetterHealthRow(t *testing.T, health SchedulerHealthResponse, kind InstanceType, name string) InstanceHealth {
	t.Helper()
	key := schedulerKey(kind, name)
	for _, row := range health.Instances {
		if row.Key == key {
			return row
		}
	}
	t.Fatalf("no Scheduler Health row for %s", key)
	return InstanceHealth{}
}

func TestScheduledPollDeadLetterTracksLatestOutcome(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Hour)
		defer cancel()

		const peerName = "independent-peer"
		var targetReads, peerReads int
		canRead := false
		m := newScheduleFixture(ctx, false, func(context.Context) ([]proxmox.Node, error) {
			targetReads++
			if !canRead {
				return nil, context.DeadlineExceeded
			}
			return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: 0.42, Uptime: 36000}}, nil
		})
		// An explicit non-default port disables the alternate-port probe.
		m.config.PVEInstances[0].Host = "https://fixture-pve.invalid:9443"
		peer := m.config.PVEInstances[0]
		peer.Name, peer.Host = peerName, "https://independent-peer.invalid:9443"
		m.config.PVEInstances = append(m.config.PVEInstances, peer)
		m.pveClients[peerName] = &pollWorkFixturePVE{getNodes: func(context.Context) ([]proxmox.Node, error) {
			peerReads++
			return nil, context.DeadlineExceeded
		}}
		m.lastClusterCheck[peerName] = time.Now()
		m.deadLetterQueue = NewTaskQueue()
		reg := prometheus.NewRegistry()
		m.pollMetrics = newPollMetrics(reg)
		defer m.pollMetrics.trackDeadLetterQueue(ctx, m.deadLetterQueue)()
		handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
		m.circuitBreakers = make(map[string]*circuitBreaker)
		m.backoffCfg = backoffConfig{Initial: 5 * time.Second, Multiplier: 2, Max: 5 * time.Minute}

		runNext := func() ScheduledTask {
			t.Helper()
			task, ok := m.taskQueue.WaitNext(ctx)
			if !ok {
				t.Fatal("ordinary scheduling did not produce an eligible task")
			}
			m.executeAndRescheduleTask(ctx, task, m.pollWorkProgress(), 0)
			return task
		}
		gauges := func(phase string, wantTarget, wantPeer int) {
			t.Helper()
			assertDeadLetterScrape(t, handler, phase, map[metricKey]int{
				{"pve", scheduleFixtureNode}: wantTarget,
				{"pve", peerName}:            wantPeer,
			})
		}

		empty := m.deadLetterQueue.PeekAll(0)
		m.poll(ctx, nil)
		const terminalReads = 5
		for i := 0; i < 2*terminalReads; i++ {
			runNext()
		}
		gauges("failed", 1, 1)
		m.pollMetrics.UpdateDeadLetterCounts(empty)
		gauges("after-older-empty-refresh", 1, 1)
		failed := captureDeadLetterHealth(t, m, "failed")
		target := deadLetterHealthRow(t, failed, InstanceTypePVE, scheduleFixtureNode)
		failedPeer := deadLetterHealthRow(t, failed, InstanceTypePVE, peerName)
		if targetReads != terminalReads || peerReads != terminalReads ||
			failed.DeadLetter.Count != 2 || len(failed.DeadLetter.Tasks) != 2 || !failedPeer.DeadLetter.Present ||
			target.PollStatus.ConsecutiveFailures != terminalReads || target.PollStatus.LastSuccess != nil ||
			target.PollStatus.LastError == nil || target.Breaker.State != "open" || target.Breaker.RetryAt == nil ||
			!target.DeadLetter.Present || target.DeadLetter.NextRetry == nil ||
			!target.DeadLetter.NextRetry.After(*target.Breaker.RetryAt) {
			t.Fatalf("fixture did not reach two terminal failures: target reads=%d peer reads=%d", targetReads, peerReads)
		}

		// A new ordinary plan may queue a claim before the breaker allows it.
		// A skipped claim must not count as recovery, even if the source is now up.
		canRead = true
		time.Sleep(scheduleFixtureCadence)
		m.poll(ctx, nil)
		runNext()
		runNext()
		waiting := captureDeadLetterHealth(t, m, "breaker-wait")
		if targetReads != terminalReads || peerReads != terminalReads ||
			!reflect.DeepEqual(waiting.DeadLetter, failed.DeadLetter) ||
			!reflect.DeepEqual(deadLetterHealthRow(t, waiting, InstanceTypePVE, scheduleFixtureNode).DeadLetter, target.DeadLetter) {
			t.Fatal("a claim without a new reading changed a terminal failure")
		}

		captured := make(chan int, 1)
		publishRefresh, publishedRefresh := make(chan struct{}), make(chan struct{})
		go func() {
			tasks := m.deadLetterQueue.PeekAll(m.deadLetterQueue.Size())
			captured <- len(tasks)
			<-publishRefresh
			m.pollMetrics.UpdateDeadLetterCounts(tasks)
			close(publishedRefresh)
		}()
		finishRefresh := sync.OnceFunc(func() {
			close(publishRefresh)
			<-publishedRefresh
		})
		defer finishRefresh()
		if n := <-captured; n != 2 {
			t.Fatalf("refresh captured %d terminal entries, want 2", n)
		}

		// Ordinary scrapes keep reporting both parked polls while the breaker
		// waits. Sample near the retry so the recovered label is still within
		// the brief zero window, rather than already expired by a scrape gap.
		time.Sleep(time.Until(*target.Breaker.RetryAt) - time.Second)
		gauges("before-recovery", 1, 1)

		if task := runNext(); task.InstanceName != scheduleFixtureNode {
			t.Fatalf("expected the target to be the first eligible probe, got %+v", task)
		}
		gauges("recovered", 0, 1)
		finishRefresh()
		// The old refresh cannot put a recovered connection back in the gauge.
		gauges("after-overlapping-refresh", 0, 1)
		recovered := captureDeadLetterHealth(t, m, "recovered")
		now := deadLetterHealthRow(t, recovered, InstanceTypePVE, scheduleFixtureNode)
		peerNow := deadLetterHealthRow(t, recovered, InstanceTypePVE, peerName)
		node, published := breakerFixtureNode(m, scheduleFixtureNode)
		if targetReads != terminalReads+1 || peerReads != terminalReads || !published || node.CPU != 0.42 ||
			now.PollStatus.LastSuccess == nil || !now.PollStatus.LastSuccess.Equal(*target.Breaker.RetryAt) ||
			now.PollStatus.LastError != nil || now.PollStatus.ConsecutiveFailures != 0 || now.Breaker.State != "closed" {
			t.Fatalf("ordinary poll did not recover and publish CPU: target reads=%d peer reads=%d node=%+v row=%+v",
				targetReads, peerReads, node, now)
		}
		if next, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode); !ok || !next.NextRun.Equal(time.Now().Add(scheduleFixtureCadence)) {
			t.Errorf("recovery changed ordinary cadence: queued=%v, task=%+v", ok, next)
		}
		if recovered.DeadLetter.Count != 1 || len(recovered.DeadLetter.Tasks) != 1 ||
			recovered.DeadLetter.Tasks[0].Instance != peerName || now.DeadLetter != (InstanceDLQ{}) {
			t.Errorf("recovered connection still appears in the dead-letter snapshot: %+v, %+v", recovered.DeadLetter, now.DeadLetter)
		}
		if !reflect.DeepEqual(peerNow, failedPeer) {
			t.Errorf("recovering the target changed the peer's failure: %+v, want %+v", peerNow, failedPeer)
		}
	})
}

func TestPMGPartialRecoveryRetiresTerminalDeadLetter(t *testing.T) {
	m, task := newPartialReadPMGMonitor(t)
	t.Cleanup(m.Stop)
	reg := prometheus.NewRegistry()
	m.pollMetrics = newPollMetrics(reg)
	defer m.pollMetrics.trackDeadLetterQueue(t.Context(), m.deadLetterQueue)()
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	key := makeMetricKey("pmg", task.InstanceName)
	fixture := &pmgScopeFixture{calls: map[string]int{}, fail: "version"}
	poll := func() { m.pollPMGInstance(t.Context(), task.InstanceName, fixture) }
	for i := 0; i < m.maxRetryAttempts && m.DeadLetterCount() == 0; i++ {
		pollAndReschedule(t, m, task, poll)
	}
	assertDeadLetterScrape(t, handler, "pmg-failed-before-health", map[metricKey]int{key: 1})
	before := captureDeadLetterHealth(t, m, "pmg-failed")
	if before.DeadLetter.Count != 1 || deadLetterHealthRow(t, before, InstanceTypePMG, task.InstanceName).PollStatus.LastError == nil {
		t.Fatal("fixture did not reach a terminal version error")
	}

	fixture.fail = "mail"
	next := pollAndReschedule(t, m, task, poll)
	assertDeadLetterScrape(t, handler, "pmg-partial-before-health", map[metricKey]int{key: 0})
	after := captureDeadLetterHealth(t, m, "pmg-partial")
	row := deadLetterHealthRow(t, after, InstanceTypePMG, task.InstanceName)
	if after.DeadLetter.Count != 0 || len(after.DeadLetter.Tasks) != 0 || row.DeadLetter != (InstanceDLQ{}) {
		t.Errorf("partial recovery left the old whole-connection failure in the dead-letter queue: %+v", after.DeadLetter)
	}
	if next.Interval != task.Interval || row.PollStatus.LastSuccess == nil || row.Breaker.State != "closed" ||
		row.PollStatus.LastError == nil || row.PollStatus.LastError.Category != pollReadErrorCategory ||
		row.PollStatus.ConsecutiveFailures != 0 {
		t.Errorf("partial recovery lost its cadence or outstanding reading error: %+v / %+v", next, row)
	}
	state := m.state.GetSnapshot().PMGInstances
	if len(state) != 1 || state[0].MailStats != nil || state[0].Quarantine == nil {
		t.Errorf("partial state hid healthy readings or published a failed one: %+v", state)
	}
}

func TestSchedulerDeadLettersExcludeCancelledRuntimeBeforeDetach(t *testing.T) {
	reg := prometheus.NewRegistry()
	pm := newPollMetrics(reg)
	handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	queue := NewTaskQueue()
	defer pm.trackDeadLetterQueue(ctx, queue)()
	queue.Upsert(ScheduledTask{InstanceType: InstanceTypePBS, InstanceName: "stopping"})
	assertDeadLetterScrape(t, handler, "before-cancel", map[metricKey]int{{"pbs", "stopping"}: 1})
	cancel() // The loop has not yet returned its registration.
	assertDeadLetterScrape(t, handler, "after-cancel-before-detach", map[metricKey]int{{"pbs", "stopping"}: 0})
}
