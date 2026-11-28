package monitoring

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type heldAvailabilityRoster struct {
	availabilityPollProvider
	hold func()
}

func (p heldAvailabilityRoster) pollPlanningRoster(m *Monitor) []pollPlanInstance {
	roster := p.availabilityPollProvider.pollPlanningRoster(m)
	p.hold()
	return roster
}

func TestAvailabilityRetirementRejectsOldPlansAndWorkerSuccessors(t *testing.T) {
	for _, mode := range []string{"fixed", "adaptive"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(dir, "absent-test-legacy.key"))
			persistence := config.NewConfigPersistence(dir)
			retired := probeAgentTarget("retired-check", "")
			if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{retired}); err != nil {
				t.Fatal(err)
			}
			synctest.Test(t, func(t *testing.T) {
				ctx := t.Context()
				m := newScheduleFixture(ctx, mode == "adaptive", func(_ context.Context) ([]proxmox.Node, error) {
					return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: 0.42, Uptime: 36000}}, nil
				})
				m.configPersist = persistence
				m.RefreshAvailabilityTargets()
				held, release, planned := make(chan struct{}), make(chan struct{}), make(chan struct{})
				finish := sync.OnceFunc(func() { close(release) })
				defer finish()
				if err := m.RegisterPollProvider(heldAvailabilityRoster{hold: sync.OnceFunc(func() {
					close(held)
					<-release
				})}); err != nil {
					t.Fatal(err)
				}
				go func() {
					m.planScheduledPolls(ctx, time.Now())
					close(planned)
				}()
				<-held // the planner already read the roster that still included the target
				claimed, ok := m.taskQueue.WaitNext(ctx)
				if !ok || claimed.InstanceType != InstanceTypeAvailability || claimed.InstanceName != retired.ID {
					t.Fatalf("expected a worker claim for %s, got %+v %v", retired.ID, claimed, ok)
				}

				if err := persistence.SaveAvailabilityTargets(nil); err != nil {
					t.Fatal(err)
				}
				m.RefreshAvailabilityTargets()
				retry := claimed
				retry.NextRun = time.Now().Add(time.Minute)
				m.taskQueue.completeClaim(claimed, retry, pollAfterBackoff)
				if _, ok := m.taskQueue.Get(InstanceTypeAvailability, retired.ID); ok {
					t.Error("returning worker requeued the deleted availability check")
				}

				finish()
				<-planned
				if _, ok := m.taskQueue.Get(InstanceTypeAvailability, retired.ID); ok {
					t.Error("a plan made before the saved deletion requeued the check")
				}
				// Reject only the stale availability work; the same planning
				// pass must still deliver the unrelated PVE reading.
				peerCtx, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()
				task, ok := m.taskQueue.WaitNext(peerCtx)
				if !ok || task.InstanceType != InstanceTypePVE || task.InstanceName != scheduleFixtureNode {
					t.Fatalf("stale availability roster suppressed unrelated work: %+v %v", task, ok)
				}
				m.executeAndRescheduleTask(ctx, task, nil, 0)
				if node, ok := breakerFixtureNode(m, scheduleFixtureNode); !ok || node.CPU != 0.42 {
					t.Fatalf("PVE poll did not publish its reading: %+v %v", node, ok)
				}
				m.planScheduledPolls(ctx, time.Now())
				if _, ok := m.taskQueue.Get(InstanceTypeAvailability, retired.ID); ok {
					t.Error("an ordinary fresh plan contained the deleted check")
				}
				replacement := probeAgentTarget("new-check", "")
				if err := persistence.SaveAvailabilityTargets([]config.AvailabilityTarget{replacement}); err != nil {
					t.Fatal(err)
				}
				m.planScheduledPolls(ctx, time.Now())
				if _, ok := m.taskQueue.Get(InstanceTypeAvailability, replacement.ID); !ok {
					t.Error("a fresh plan could not admit a newly saved check")
				}
			})
		})
	}
}

func TestAvailabilityRefreshPreservesQueueWhenRosterUnreadable(t *testing.T) {
	local := probeAgentTarget("saved-check", "")
	m, writer, reader := newAvailabilityPlanningFixture(t, []config.AvailabilityTarget{local})
	m.RefreshAvailabilityTargets()
	before, ok := m.taskQueue.Get(InstanceTypeAvailability, local.ID)
	if !ok {
		t.Fatal("saved check was not scheduled")
	}
	if err := writer.SaveAvailabilityTargets(nil); err != nil {
		t.Fatal(err)
	}
	reader.err = errors.New("fixture could not read the catalog")
	m.RefreshAvailabilityTargets()
	if after, ok := m.taskQueue.Get(InstanceTypeAvailability, local.ID); !ok || after != before {
		t.Fatalf("unavailable catalog changed the existing schedule: %+v %v", after, ok)
	}
	reader.err = nil
	m.RefreshAvailabilityTargets()
	if _, ok := m.taskQueue.Get(InstanceTypeAvailability, local.ID); ok {
		t.Fatal("confirmed empty catalog still has an availability poll")
	}
}
