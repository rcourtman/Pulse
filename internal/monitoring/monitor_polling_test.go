package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

func TestBuildScheduledTasksUsesConfiguredIntervals(t *testing.T) {
	now := time.Now()
	cfg := &config.Config{
		PVEPollingInterval:          2 * time.Minute,
		PBSPollingInterval:          45 * time.Second,
		PMGPollingInterval:          90 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	monitor := &Monitor{
		config:     cfg,
		pveClients: map[string]PVEClientInterface{"pve-1": nil},
		pbsClients: map[string]*pbs.Client{"pbs-1": nil},
		pmgClients: map[string]*pmg.Client{"pmg-1": nil},
	}

	tasks := monitor.buildScheduledTasks(now)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	got := map[InstanceType]time.Duration{}
	for _, task := range tasks {
		if !task.NextRun.Equal(now) {
			t.Fatalf("expected NextRun to equal provided time, got %v", task.NextRun)
		}
		got[task.InstanceType] = task.Interval
	}

	if got[InstanceTypePVE] != cfg.PVEPollingInterval {
		t.Fatalf("expected PVE interval %v, got %v", cfg.PVEPollingInterval, got[InstanceTypePVE])
	}
	if got[InstanceTypePBS] != cfg.PBSPollingInterval {
		t.Fatalf("expected PBS interval %v, got %v", cfg.PBSPollingInterval, got[InstanceTypePBS])
	}
	if got[InstanceTypePMG] != cfg.PMGPollingInterval {
		t.Fatalf("expected PMG interval %v, got %v", cfg.PMGPollingInterval, got[InstanceTypePMG])
	}
}

func TestRescheduleTaskUsesInstanceIntervalWhenSchedulerDisabled(t *testing.T) {
	cfg := &config.Config{
		PVEPollingInterval:          75 * time.Second,
		AdaptivePollingBaseInterval: 10 * time.Second,
	}

	monitor := &Monitor{
		config:    cfg,
		taskQueue: NewTaskQueue(),
	}

	task := ScheduledTask{
		InstanceName: "pve-1",
		InstanceType: InstanceTypePVE,
		Interval:     0,
		NextRun:      time.Now(),
	}

	monitor.rescheduleTask(task)

	monitor.taskQueue.mu.Lock()
	entry, ok := monitor.taskQueue.entries[schedulerKey(task.InstanceType, task.InstanceName)]
	monitor.taskQueue.mu.Unlock()
	if !ok {
		t.Fatalf("expected task to be rescheduled in queue")
	}

	if entry.task.Interval != cfg.PVEPollingInterval {
		t.Fatalf("expected interval %v, got %v", cfg.PVEPollingInterval, entry.task.Interval)
	}

	remaining := time.Until(entry.task.NextRun)
	if remaining < cfg.PVEPollingInterval-2*time.Second || remaining > cfg.PVEPollingInterval+time.Second {
		t.Fatalf("expected NextRun about %v from now, got %v", cfg.PVEPollingInterval, remaining)
	}
}
