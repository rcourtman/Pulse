package ai

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestPatrolLoopRestartRetainsRemainingInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := NewPatrolService(nil, nil)
		p.config = PatrolConfig{Enabled: true, Interval: 10 * time.Minute}
		p.stopCh = make(chan struct{})
		p.configChanged = make(chan struct{}, 1)
		completed := time.Now().Add(-5 * time.Minute)
		p.runHistoryStore.Add(PatrolRunRecord{
			ID: "before-restart", Type: "patrol", TriggerReason: "scheduled",
			StartedAt: completed.Add(-time.Minute), CompletedAt: completed, Status: "healthy",
		})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go p.patrolLoop(ctx)
		time.Sleep(initialPatrolStartDelay + time.Second)
		synctest.Wait()
		status := p.GetStatus()
		want := completed.Add(10 * time.Minute)
		if status.NextPatrolAt == nil || !status.NextPatrolAt.Equal(want) {
			t.Fatalf("restart postponed the existing cadence: next=%v, want=%v", status.NextPatrolAt, want)
		}
	})
}

func newScheduleTestPatrol(t *testing.T, persistence *mockPatrolHistoryPersistence, interval time.Duration) *PatrolService {
	t.Helper()
	p := NewPatrolService(nil, nil)
	p.config = PatrolConfig{Enabled: true, Interval: interval, RuntimeBlockedReason: "scheduler fixture has no provider"}
	p.stopCh = make(chan struct{})
	p.configChanged = make(chan struct{}, 1)
	if err := p.SetRunHistoryPersistence(persistence); err != nil {
		t.Fatal(err)
	}
	return p
}

func startScheduleTestLoop(p *PatrolService) func() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		p.patrolLoop(ctx)
	}()
	return func() { cancel(); <-done }
}

func requireNextPatrol(t *testing.T, p *PatrolService, want time.Time) {
	t.Helper()
	status := p.GetStatus()
	if status.NextPatrolAt == nil || !status.NextPatrolAt.Equal(want) {
		t.Fatalf("next check = %v, want %v", status.NextPatrolAt, want)
	}
}

func scheduleTestAttemptedAt(p *PatrolService) time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastBlockedAt
}

func TestPatrolLoopRepeatedRestartsPreserveDeadlineAndFire(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		due := time.Now().Add(5 * time.Minute)
		persistence := &mockPatrolHistoryPersistence{nextScheduledAt: due, scheduleInterval: 10 * time.Minute}
		for range 3 {
			p := newScheduleTestPatrol(t, persistence, 10*time.Minute)
			stop := startScheduleTestLoop(p)
			time.Sleep(initialPatrolStartDelay + time.Second)
			synctest.Wait()
			requireNextPatrol(t, p, due)
			if !scheduleTestAttemptedAt(p).IsZero() {
				t.Fatal("restart ran a duplicate check before its deadline")
			}
			stop()
		}
		p := newScheduleTestPatrol(t, persistence, 10*time.Minute)
		stop := startScheduleTestLoop(p)
		defer stop()
		time.Sleep(time.Until(due) - time.Second)
		synctest.Wait()
		if !scheduleTestAttemptedAt(p).IsZero() {
			t.Fatal("scheduled check ran early")
		}
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if at := scheduleTestAttemptedAt(p); !at.Equal(due) {
			t.Fatalf("actual check attempt at %v, want %v", at, due)
		}
		requireNextPatrol(t, p, due.Add(10*time.Minute))
		if got := persistence.scheduleSaves.Load(); got != 1 {
			t.Fatalf("deadline writes = %d, want one advancement and no restart rewrites", got)
		}
	})
}

func TestPatrolLoopOverdueRestartRunsOnceWithoutBacklog(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		persistence := &mockPatrolHistoryPersistence{nextScheduledAt: time.Now().Add(-25 * time.Minute), scheduleInterval: 10 * time.Minute}
		p := newScheduleTestPatrol(t, persistence, 10*time.Minute)
		stop := startScheduleTestLoop(p)
		defer stop()
		wantAttempt := time.Now().Add(initialPatrolStartDelay)
		time.Sleep(initialPatrolStartDelay + time.Second)
		synctest.Wait()
		if at := scheduleTestAttemptedAt(p); !at.Equal(wantAttempt) {
			t.Fatalf("overdue check at %v, want startup readiness at %v", at, wantAttempt)
		}
		requireNextPatrol(t, p, wantAttempt.Add(10*time.Minute))
		time.Sleep(time.Minute)
		synctest.Wait()
		if !scheduleTestAttemptedAt(p).Equal(wantAttempt) || persistence.scheduleSaves.Load() != 1 {
			t.Fatal("overdue restart replayed missed intervals")
		}
	})
}

func TestPatrolLoopIntervalResetSurvivesRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		persistence := &mockPatrolHistoryPersistence{nextScheduledAt: time.Now().Add(5 * time.Minute), scheduleInterval: 10 * time.Minute}
		p := newScheduleTestPatrol(t, persistence, 10*time.Minute)
		stop := startScheduleTestLoop(p)
		time.Sleep(initialPatrolStartDelay + time.Second)
		synctest.Wait()
		cfg := p.config
		cfg.Interval = 20 * time.Minute
		want := time.Now().Add(cfg.Interval)
		p.SetConfig(cfg)
		synctest.Wait()
		requireNextPatrol(t, p, want)
		stop()
		p = newScheduleTestPatrol(t, persistence, cfg.Interval)
		stop = startScheduleTestLoop(p)
		defer stop()
		time.Sleep(initialPatrolStartDelay + time.Second)
		synctest.Wait()
		requireNextPatrol(t, p, want)
	})
}

func TestPatrolLoopDisabledPreparationKeepsFirstStartupCheck(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		persistence := &mockPatrolHistoryPersistence{}
		p := newScheduleTestPatrol(t, persistence, 10*time.Minute)
		p.config.Enabled = false
		stop := startScheduleTestLoop(p)
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		stop()
		if !persistence.nextScheduledAt.IsZero() {
			t.Fatal("disabled preparation persisted a check that never ran")
		}
		p = newScheduleTestPatrol(t, persistence, 10*time.Minute)
		stop = startScheduleTestLoop(p)
		defer stop()
		want := time.Now().Add(initialPatrolStartDelay)
		time.Sleep(initialPatrolStartDelay + time.Second)
		synctest.Wait()
		if at := scheduleTestAttemptedAt(p); !at.Equal(want) {
			t.Fatalf("first startup check at %v, want %v", at, want)
		}
	})
}

func TestInitialPatrolScheduleLegacyHistoryAndChangedInterval(t *testing.T) {
	now := time.Now()
	p := NewPatrolService(nil, nil)
	p.runHistoryStore.Add(PatrolRunRecord{ID: "full", Type: "patrol", CompletedAt: now.Add(-8 * time.Minute)})
	for range 20 {
		p.runHistoryStore.Add(PatrolRunRecord{Type: "scoped", CompletedAt: now})
	}
	if err := p.runHistoryStore.SetSchedule(now.Add(12*time.Minute), 20*time.Minute); err != nil {
		t.Fatal(err)
	}
	next, trigger := p.initialPatrolSchedule(now, 10*time.Minute)
	if !next.Equal(now.Add(2*time.Minute)) || trigger != TriggerReasonScheduled {
		t.Fatalf("changed interval/retained full run schedule = %v %s", next, trigger)
	}
}
