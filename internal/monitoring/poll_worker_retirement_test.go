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

// Hold a claim between dequeue and execution, without relying on a goroutine
// happening to lose a race to the settings refresh.
func TestRetiredPollClaimDoesNotEnterProvider(t *testing.T) {
	for _, readd := range []bool{false, true} {
		name := "deleted"
		if readd {
			name = "re-added"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
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

				check := ScheduledTask{InstanceType: InstanceTypeAvailability, InstanceName: "check", NextRun: start, Interval: time.Minute}
				m.taskQueue.Upsert(check)
				claimed, ok := m.taskQueue.WaitNext(ctx)
				if !ok || claimed != check {
					t.Fatalf("expected the original claim, got %+v, claimed=%v", claimed, ok)
				}
				m.taskQueue.refreshAvailabilitySchedule(nil)
				replacement := check
				replacement.NextRun = start.Add(10 * time.Second)
				if readd {
					m.taskQueue.refreshAvailabilitySchedule(map[string]ScheduledTask{"check": replacement})
				}

				m.executeAndRescheduleTask(ctx, claimed, nil, 0)
				if len(builds) != 0 || len(reads) != 0 {
					t.Fatalf("retired claim reached provider setup or the read: builds=%v reads=%v", builds, reads)
				}
				queued, ok := m.taskQueue.Get(check.InstanceType, check.InstanceName)
				if ok != readd || (ok && queued != replacement) {
					t.Fatalf("wanted replacement=%v at its saved cadence, got %+v, queued=%v", readd, queued, ok)
				}

				peer := ScheduledTask{InstanceType: InstanceTypePVE, InstanceName: "peer", NextRun: start, Interval: time.Minute}
				m.taskQueue.Upsert(peer)
				claimed, ok = m.taskQueue.WaitNext(ctx)
				if !ok || claimed != peer || !time.Now().Equal(start) {
					t.Fatalf("expected the live peer to be due immediately, got %+v, claimed=%v", claimed, ok)
				}
				m.executeAndRescheduleTask(ctx, claimed, nil, 0)
				m.taskQueue.Remove(peer.InstanceType, peer.InstanceName)
				if !slices.Equal(builds, []string{"peer"}) || !slices.Equal(reads, []string{"peer"}) {
					t.Errorf("expected only the live peer now: builds=%v reads=%v", builds, reads)
				}
				if !readd {
					return
				}
				claimed, ok = m.taskQueue.WaitNext(ctx)
				if !ok || claimed != replacement || !time.Now().Equal(replacement.NextRun) {
					t.Fatalf("expected replacement at its own deadline, got %+v, claimed=%v", claimed, ok)
				}
				m.executeAndRescheduleTask(ctx, claimed, nil, 0)
				want := []string{"peer", "check"}
				if !slices.Equal(builds, want) || !slices.Equal(reads, want) {
					t.Errorf("expected %v after the replacement deadline: builds=%v reads=%v", want, builds, reads)
				}
			})
		})
	}
}
