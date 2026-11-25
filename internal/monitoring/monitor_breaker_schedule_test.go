package monitoring

import (
	"context"
	"math/rand"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

const breakerFixturePeer = "healthy-peer"

type breakerScheduleFixture struct {
	adaptive, shared, terminal bool
	initialBackoff             time.Duration
}

type breakerScheduleObservation struct {
	targetReads, peerReads int
	retryAt, due           time.Time
}

// Drive ordinary planning, worker admission and the PVE collector. Only the
// remote reads, clock and jitter seed are controlled.
func observeBreakerSchedule(t *testing.T, fixture breakerScheduleFixture, observe func(string, *Monitor, breakerScheduleObservation)) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		start := time.Now()
		obs := breakerScheduleObservation{}
		m := newScheduleFixture(ctx, fixture.adaptive, func(context.Context) ([]proxmox.Node, error) {
			obs.targetReads++
			cpu := 0.24
			if obs.targetReads > 1 {
				if obs.targetReads <= 4 {
					return nil, context.DeadlineExceeded
				}
				cpu = 0.42
			}
			return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: cpu, Uptime: 36000}}, nil
		})
		// Use an explicit non-default port so a timeout in this fixture cannot
		// try the HTTPS port fallback.
		m.config.PVEInstances[0].Host = "https://branch-node.invalid:9443"
		peer := m.config.PVEInstances[0]
		peer.Name, peer.Host = breakerFixturePeer, "https://healthy-peer.invalid:9443"
		m.config.PVEInstances = append(m.config.PVEInstances, peer)
		m.lastClusterCheck[peer.Name] = start
		m.pveClients[peer.Name] = &pollWorkFixturePVE{getNodes: func(context.Context) ([]proxmox.Node, error) {
			obs.peerReads++
			return []proxmox.Node{{Node: breakerFixturePeer, Status: "online", CPU: 0.36, Uptime: 36000}}, nil
		}}
		m.deadLetterQueue = NewTaskQueue()
		m.circuitBreakers = make(map[string]*circuitBreaker)
		m.backoffCfg = backoffConfig{Initial: fixture.initialBackoff, Multiplier: 2, Jitter: 0.2, Max: 5 * time.Minute}
		const seed = 3
		m.rng = rand.New(rand.NewSource(seed))
		jitter := rand.New(rand.NewSource(seed))
		if fixture.terminal {
			m.maxRetryAttempts = 3 // exhaust while the breaker opens
		}

		if fixture.shared {
			go m.dispatchTaskWorkers(ctx, newPollTaskWorkerLimiter(1))
		} else {
			go m.taskWorker(ctx, 0)
		}
		m.poll(ctx, nil)
		synctest.Wait()
		observe("initial", m, obs)

		time.Sleep(scheduleFixtureCadence)
		synctest.Wait()
		for attempt := 0; attempt < 2; attempt++ {
			time.Sleep(m.backoffCfg.nextDelay(attempt, jitter.Float64()))
			synctest.Wait()
		}
		opened := time.Now()
		key := schedulerKey(InstanceTypePVE, scheduleFixtureNode)
		state, failures, retryAt := m.circuitBreakers[key].State()
		if state != "open" || failures != 3 || obs.targetReads != 4 || !retryAt.Equal(opened.Add(40*time.Second)) {
			t.Fatalf("fixture did not reach the third genuine failure: %s failures=%d reads=%d retry=%v", state, failures, obs.targetReads, retryAt.Sub(opened))
		}
		obs.retryAt = retryAt
		obs.due = retryAt
		if !fixture.terminal {
			if backoff := opened.Add(m.backoffCfg.nextDelay(2, jitter.Float64())); backoff.After(retryAt) {
				obs.due = backoff
			}
		}
		observe("opened", m, obs)

		// A normal plan while the breaker is open. The healthy peer also has
		// work due, with only one available slot.
		time.Sleep(time.Until(start.Add(2 * scheduleFixtureCadence)))
		m.poll(ctx, nil)
		synctest.Wait()
		observe("replanned", m, obs)

		time.Sleep(time.Until(obs.due) - time.Nanosecond)
		synctest.Wait()
		observe("waiting", m, obs)
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		observe("eligible", m, obs)

		// Compare eventual recovery too: with ordinary jitter, the baseline
		// runs its extra backoff and eventually makes the same successful read.
		time.Sleep(12 * time.Second)
		synctest.Wait()
		observe("settled", m, obs)
	})
}

func breakerFixtureNode(m *Monitor, name string) (models.Node, bool) {
	for _, node := range m.state.GetSnapshot().Nodes {
		if node.Name == name {
			return node, true
		}
	}
	return models.Node{}, false
}

func TestScheduledPollRespectsBreakerAndBackoff(t *testing.T) {
	for _, tc := range []struct {
		name string
		breakerScheduleFixture
	}{
		{"fixed-local", breakerScheduleFixture{initialBackoff: 5 * time.Second}},
		{"adaptive-process", breakerScheduleFixture{adaptive: true, shared: true, initialBackoff: 5 * time.Second}},
		{"longer-backoff", breakerScheduleFixture{initialBackoff: 12 * time.Second}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observeBreakerSchedule(t, tc.breakerScheduleFixture, func(phase string, m *Monitor, obs breakerScheduleObservation) {
				key := schedulerKey(InstanceTypePVE, scheduleFixtureNode)
				switch phase {
				case "opened", "replanned", "waiting":
					queued, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode)
					if obs.targetReads != 4 || m.failureCounts[key] != 3 || m.deadLetterQueue.Size() != 0 {
						t.Errorf("%s: the deferred retry made or recorded another attempt: reads=%d failures=%d DLQ=%d",
							phase, obs.targetReads, m.failureCounts[key], m.deadLetterQueue.Size())
					}
					if !ok || !queued.NextRun.Equal(obs.due) {
						t.Errorf("%s: next queued poll relative to breaker deadline = %v (queued=%v), want %v",
							phase, queued.NextRun.Sub(obs.retryAt), ok, obs.due.Sub(obs.retryAt))
					}
					if phase == "replanned" {
						peer, ok := breakerFixtureNode(m, breakerFixturePeer)
						if !ok || peer.CPU != 0.36 || obs.peerReads != 3 {
							t.Errorf("healthy peer did not keep polling: reads=%d node=%+v", obs.peerReads, peer)
						}
					}
				case "eligible":
					node, ok := breakerFixtureNode(m, scheduleFixtureNode)
					points := m.metricsHistory.GetNodeMetrics(node.ID, "cpu", time.Hour)
					state, failures, retryAt := m.circuitBreakers[key].State()
					if !ok || obs.targetReads != 5 || node.CPU != 0.42 || state != "closed" || failures != 0 ||
						!retryAt.IsZero() || len(points) != 2 || points[1].Value != 42 || !m.pollStatusMap[key].LastSuccess.Equal(obs.due) {
						t.Errorf("eligible probe did not publish recovery and CPU History: reads=%d cpu=%.2f breaker=%s failures=%d points=%+v",
							obs.targetReads, node.CPU, state, failures, points)
					}
				}
			})
		})
	}
}

func TestBreakerDeferredClaimDoesNotRepeatDeadLetter(t *testing.T) {
	for _, shared := range []bool{false, true} {
		name := "local"
		if shared {
			name = "process"
		}
		t.Run(name, func(t *testing.T) {
			var original dlqInsight
			observeBreakerSchedule(t, breakerScheduleFixture{shared: shared, terminal: true, initialBackoff: 5 * time.Second}, func(phase string, m *Monitor, obs breakerScheduleObservation) {
				key := schedulerKey(InstanceTypePVE, scheduleFixtureNode)
				switch phase {
				case "opened", "replanned", "waiting":
					info := m.dlqInsightMap[key]
					if info == nil || m.deadLetterQueue.Size() != 1 || obs.targetReads != 4 {
						t.Fatalf("%s: lost the one real terminal result: reads=%d DLQ=%d info=%+v", phase, obs.targetReads, m.deadLetterQueue.Size(), info)
					}
					if phase == "opened" {
						original = *info
						if original.RetryCount != 1 {
							t.Fatalf("first exhausted read was recorded %d times", original.RetryCount)
						}
						return
					}
					if *info != original {
						t.Errorf("%s: no new read, but dead-letter count changed from %d to %d and time moved %v",
							phase, original.RetryCount, info.RetryCount, info.NextRetry.Sub(original.NextRetry))
					}
					queued, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode)
					if !ok || !queued.NextRun.Equal(obs.retryAt) {
						t.Errorf("%s: skipped claim did not return to the queue at the breaker deadline: queued=%v time=%v", phase, ok, queued.NextRun)
					}
				case "eligible":
					if obs.targetReads != 5 || !m.pollStatusMap[key].LastSuccess.Equal(obs.retryAt) {
						t.Errorf("the next probe at the breaker deadline did not recover: reads=%d success=%v", obs.targetReads, m.pollStatusMap[key].LastSuccess)
					}
				}
			})
		})
	}
}
