package monitoring

import (
	"context"
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

func TestProcessPollDispatcherReleasesRetiredClaim(t *testing.T) {
	for _, tc := range []struct {
		name  string
		readd bool
	}{
		{name: "deleted"},
		{name: "re-added", readd: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				limiter := newPollTaskWorkerLimiter(1)
				if !limiter.acquire(ctx) {
					t.Fatal("could not reserve the process slot")
				}
				start := time.Now()
				var builds, reads []string
				build := func(_ *Monitor, name string) (PollTask, error) {
					builds = append(builds, name)
					return PollTask{Run: func(context.Context) { reads = append(reads, name) }}, nil
				}
				m := &Monitor{
					taskQueue: NewTaskQueue(),
					pollProviders: map[InstanceType]PollProvider{
						InstanceTypeAvailability: pollProviderAdapter{instanceType: InstanceTypeAvailability, buildPollTask: build},
						InstanceTypePVE:          pollProviderAdapter{instanceType: InstanceTypePVE, buildPollTask: build},
					},
				}
				m.SetExecutor(newRealExecutor(m))
				retired := ScheduledTask{InstanceType: InstanceTypeAvailability, InstanceName: "check", NextRun: start, Interval: time.Minute}
				m.taskQueue.Upsert(retired)
				go m.dispatchTaskWorkers(ctx, limiter)
				synctest.Wait()
				if m.taskQueue.Size() != 0 || len(builds) != 0 {
					t.Fatalf("expected a claim waiting for the held slot, got queue=%d builds=%v", m.taskQueue.Size(), builds)
				}

				// A confirmed catalog removal can retire the claim even while
				// another monitor holds every process slot.
				peer := ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "peer", NextRun: start, Interval: time.Minute}
				m.taskQueue.Upsert(peer)
				m.taskQueue.refreshAvailabilitySchedule(nil)
				if tc.readd {
					replacement := retired
					replacement.NextRun = start.Add(10 * time.Second)
					m.taskQueue.Upsert(replacement)
				}
				synctest.Wait()
				if _, queued := m.taskQueue.Get(peer.InstanceType, peer.InstanceName); queued {
					t.Error("dispatcher still holds the retired claim instead of claiming the live peer")
				}
				if len(builds) != 0 {
					t.Fatalf("provider setup started without a process slot: %v", builds)
				}

				limiter.release()
				synctest.Wait()
				if !slices.Equal(builds, []string{"peer"}) || !slices.Equal(reads, []string{"peer"}) {
					t.Errorf("old claim reached provider setup or displaced the live peer: builds=%v reads=%v", builds, reads)
				}
				if !tc.readd {
					if _, queued := m.taskQueue.Get(retired.InstanceType, retired.InstanceName); queued {
						t.Error("deleted check returned to the queue")
					}
					return
				}
				replacement, queued := m.taskQueue.Get(retired.InstanceType, retired.InstanceName)
				if !queued || replacement.NextRun.Sub(start) != 10*time.Second {
					t.Fatalf("explicit replacement lost its due time: %+v, queued=%v", replacement, queued)
				}
				time.Sleep(time.Until(replacement.NextRun))
				synctest.Wait()
				if !slices.Equal(builds, []string{"peer", "check"}) || !slices.Equal(reads, []string{"peer", "check"}) {
					t.Errorf("replacement did not run once at its due time: builds=%v reads=%v", builds, reads)
				}
			})
		})
	}
}
