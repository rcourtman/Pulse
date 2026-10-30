package monitoring

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func waitPBSBudget(t *testing.T, ch <-chan struct{}, phase string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatalf("PBS worker did not reach %s", phase)
	}
}

func TestPBSBudgetOwnerWithdrawsLatePublication(t *testing.T) {
	for _, transition := range []string{"runtime cancellation", "runtime replacement", "client replacement", "client retirement"} {
		t.Run(transition, func(t *testing.T) {
			f := newPBSBudgetFixture(t, "vm", "datastore")
			f.collect(t, context.Background())
			before, raw := f.points(t, "pbs-main"), f.m.state.GetSnapshot().PBSBackups
			f.m.replacePBSClient("pbs-main", f.client)
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.m.setRuntimeContext(parent, nil)
			f.m.config = &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1}
			cfg := &config.PBSInstance{Name: "pbs-main", MonitorBackups: true, MonitorDatastores: true}
			f.at.Add(15 * 60)
			f.phase.Store(1)
			f.m.pollPBSBackupsAsync("pbs-main", f.client, cfg, f.datastores, nil, nil)
			waitPBSBudget(t, f.blocked, "later datastore read")
			f.m.mu.RLock()
			owner := f.m.pbsBackupPollers["pbs-main"]
			f.m.mu.RUnlock()
			if owner == nil || owner.ctx.Err() != nil {
				t.Fatal("background work lost its live runtime owner")
			}
			// A second ordinary cycle is not a second worker, even if cadence
			// says it is due while the first HTTP request is still in flight.
			f.m.pollPBSBackupsAsync("pbs-main", f.client, cfg, f.datastores, nil, nil)
			f.m.mu.RLock()
			if f.m.pbsBackupPollers["pbs-main"] != owner {
				t.Error("ordinary cycle stacked a backup worker")
			}
			f.m.mu.RUnlock()
			switch transition {
			case "runtime cancellation":
				cancel()
			case "runtime replacement":
				f.m.setRuntimeContext(context.Background(), nil) // Old parent intentionally remains live.
			case "client replacement":
				f.m.replacePBSClient("pbs-main", &pbs.Client{})
			case "client retirement":
				f.m.replacePBSClient("pbs-main", nil)
			}
			waitPBSBudget(t, owner.done, "withdrawn worker completion")
			f.wait(t)
			if !reflect.DeepEqual(raw, f.m.state.GetSnapshot().PBSBackups) || !reflect.DeepEqual(before, f.points(t, "pbs-main")) {
				t.Error("withdrawn owner published its earlier/late rows")
			}
			// Dropping an exhausted child deadline must never erase the owner.
			ctx := context.WithValue(context.Background(), pbsBackupPollContextKey{}, owner)
			if unlock, ok := f.m.beginPBSBackupPublication(context.WithoutCancel(ctx), "pbs-main"); ok {
				unlock()
				t.Error("late owner was admitted after its binding ended")
			}
			f.m.mu.RLock()
			at := f.m.lastPBSBackupPoll["pbs-main"]
			f.m.mu.RUnlock()
			if !at.IsZero() {
				t.Error("withdrawn worker postponed its replacement's first scan")
			}
			if transition != "runtime cancellation" {
				f.phase.Store(2)
				f.m.replacePBSClient("pbs-main", f.client)
				f.m.pollPBSBackupsAsync("pbs-main", f.client, cfg, f.datastores, nil, nil)
				deadline := time.Now().Add(10 * time.Second)
				for {
					f.m.mu.RLock()
					active := f.m.pbsBackupPollers["pbs-main"]
					f.m.mu.RUnlock()
					if active == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("replacement ordinary scan did not finish")
					}
					time.Sleep(time.Millisecond)
				}
				f.wait(t)
				for _, row := range f.m.state.GetSnapshot().PBSBackups {
					if row.BackupTime.Unix() != f.at.Load() {
						t.Error("ordinary replacement borrowed former client's cached snapshots")
					}
				}
			}
		})
	}
}

func TestPBSBudgetOldCompletionCannotReleaseNewClaim(t *testing.T) {
	m := &Monitor{}
	first, second := &pbs.Client{}, &pbs.Client{}
	m.replacePBSClient("reader", first)
	m.setRuntimeContext(context.Background(), nil)
	old := m.beginPBSBackupPoll("reader", first, time.Time{}, time.Now())
	if old == nil {
		t.Fatal("first claim was not admitted")
	}
	m.replacePBSClient("reader", second)
	next := time.Now()
	current := m.beginPBSBackupPoll("reader", second, time.Time{}, next)
	if current == nil {
		t.Fatal("new binding did not get an independent first claim")
	}
	m.finishPBSBackupPoll("reader", old)
	if m.pbsBackupPollers["reader"] != current || !m.lastPBSBackupPoll["reader"].Equal(next) {
		t.Error("old completion removed the new worker or changed its cadence")
	}
	ctx, cancel := context.WithDeadline(context.WithValue(current.ctx, pbsBackupPollContextKey{}, current), time.Now().Add(-time.Second))
	defer cancel()
	unlock, ok := m.beginPBSBackupPublication(context.WithoutCancel(ctx), "reader")
	if !ok {
		t.Fatal("spent child budget blocked live owner's local publication")
	}
	unlock()
	m.finishPBSBackupPoll("reader", current)
}
