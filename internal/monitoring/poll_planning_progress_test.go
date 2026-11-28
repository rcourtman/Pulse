package monitoring

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type heldPlanningRoster struct {
	testPollProvider
	hold func()
}

func TestCanceledPollPlanDoesNotPublishAfterRosterReturns(t *testing.T) {
	for _, mode := range []string{"fixed", "adaptive"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				m := newScheduleFixture(ctx, mode == "adaptive", nil)
				// A cycle that reached alert synchronization would replace
				// this marker with the alert manager's empty active set.
				m.state.UpdateActiveAlerts([]models.Alert{{ID: "existing-alert"}})
				held, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
				unblock := sync.OnceFunc(func() { close(release) })
				defer unblock()
				if err := m.RegisterPollProvider(heldPlanningRoster{
					testPollProvider: testPollProvider{providerType: "fixture"},
					hold: func() {
						close(held)
						<-release
					},
				}); err != nil {
					t.Fatal(err)
				}
				go func() {
					defer close(returned)
					m.poll(ctx, nil)
				}()
				<-held
				cancel()
				unblock()
				<-returned

				if depth := m.taskQueue.Size(); depth != 0 {
					t.Errorf("retired monitor enqueued %d poll(s)", depth)
				}
				if m.scheduler != nil {
					if task, ok := m.scheduler.LastScheduled(InstanceTypePVE, scheduleFixtureNode); ok {
						t.Errorf("retired monitor cached an adaptive poll: %+v", task)
					}
				}
				state := m.state.GetSnapshot()
				if state.Stats.PollingCycles != 0 {
					t.Errorf("retired monitor recorded %d poll cycle(s)", state.Stats.PollingCycles)
				}
				if len(state.ActiveAlerts) != 1 || state.ActiveAlerts[0].ID != "existing-alert" {
					t.Errorf("retired monitor reconciled alert state after cancellation: %+v", state.ActiveAlerts)
				}
				if m.activePollCount != 0 {
					t.Error("retired planner did not release its slot")
				}
			})
		})
	}
}

func (p heldPlanningRoster) pollPlanningRoster(*Monitor) []pollPlanInstance {
	p.hold()
	return nil
}

func TestSchedulerPlanningAgeWhileBothPollCyclesAreHeld(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		m := newScheduleFixture(ctx, true, func(context.Context) ([]proxmox.Node, error) {
			return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: 0.42, Uptime: 36000}}, nil
		})
		reg := prometheus.NewRegistry()
		m.pollMetrics = newPollMetrics(reg)
		defer m.pollMetrics.trackTaskQueue(ctx, m.taskQueue)()

		var calls atomic.Int32
		first, second := make(chan struct{}), make(chan struct{})
		releaseFirst := sync.OnceFunc(func() { close(first) })
		releaseSecond := sync.OnceFunc(func() { close(second) })
		defer releaseFirst()
		defer releaseSecond()
		if err := m.RegisterPollProvider(heldPlanningRoster{
			testPollProvider: testPollProvider{providerType: "fixture"},
			hold: func() {
				switch calls.Add(1) {
				case 1:
					<-first
					panic("fixture roster panic")
				case 2:
					<-second
				}
			},
		}); err != nil {
			t.Fatal(err)
		}

		scrape := func(age int) {
			t.Helper()
			const name = "pulse_scheduler_poll_planning_age_seconds"
			want := fmt.Sprintf("# HELP %[1]s Age in seconds of the oldest poll planning pass still running; zero if none.\n"+
				"# TYPE %[1]s gauge\n%[1]s %[2]d\n", name, age)
			if err := testutil.GatherAndCompare(reg, strings.NewReader(want), name); err != nil {
				t.Error(err)
			}
		}

		scrape(0)
		go m.poll(ctx, nil)
		synctest.Wait()
		time.Sleep(20 * time.Second)
		go m.poll(ctx, nil)
		synctest.Wait()
		time.Sleep(12 * time.Second)

		// A third cycle cannot admit a plan. Neither held cycle has reached
		// the queue, so queue and worker diagnostics cannot see this stall.
		m.poll(ctx, nil)
		if got := calls.Load(); got != 2 || m.taskQueue.Size() != 0 {
			t.Fatalf("expected two held planning passes and an empty queue, got calls=%d depth=%d", got, m.taskQueue.Size())
		}
		scrape(32)

		// Completing the newer pass cannot clear or reset the older one.
		releaseSecond()
		synctest.Wait()
		if _, ok := m.taskQueue.Get(InstanceTypePVE, scheduleFixtureNode); !ok {
			t.Fatal("returning planner did not queue the PVE poll")
		}
		time.Sleep(3 * time.Second)
		scrape(35)

		// A panic unwinds the older pass and releases both its slot and marker.
		releaseFirst()
		synctest.Wait()
		scrape(0)
		if m.activePollCount != 0 {
			t.Fatalf("planning slots left held after both returns: %d", m.activePollCount)
		}
	})
}
