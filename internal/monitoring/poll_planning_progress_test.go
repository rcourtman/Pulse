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
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type heldPlanningRoster struct {
	testPollProvider
	hold func()
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
