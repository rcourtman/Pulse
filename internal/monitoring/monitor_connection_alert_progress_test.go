package monitoring

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

func TestMonitorConnectionAlertChecksKeepObservationOrder(t *testing.T) {
	previousMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previousMock) })

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		origin := time.Now()
		var polls, reads, fires atomic.Int32
		m := newScheduleFixture(ctx, false, func(context.Context) ([]proxmox.Node, error) {
			polls.Add(1)
			return []proxmox.Node{{Node: scheduleFixtureNode, Status: "online", CPU: 0.24, Uptime: 36000}}, nil
		})
		m.alertManager = alerts.NewManagerWithDataDir(t.TempDir())
		defer m.alertManager.Stop()
		cfg := m.alertManager.GetConfig()
		cfg.Enabled = true
		cfg.ActivationState = alerts.ActivationActive
		m.alertManager.UpdateConfig(cfg)

		// The PVE collector above is independent. These are observations
		// from the Connections ledger, including a separate persistent PBS
		// warning which must stay visible.
		const interruptedID = "pve:fixture-branch"
		const peerID = "pbs:fixture-archive"
		peer := alerts.ConnectionSnapshot{
			ID: peerID, Name: "fixture-archive", Type: alerts.ConnectionTypePBS, Enabled: true,
			State: alerts.ConnectionStateDegraded, StateReason: "One datastore could not be read",
		}
		m.SetAlertPushCallback(func(alert *alerts.Alert) {
			if alert.ResourceID == interruptedID {
				fires.Add(1)
			}
		})
		held, release := make(chan struct{}), make(chan struct{})
		m.SetConnectionsSnapshotLister(func() []alerts.ConnectionSnapshot {
			reads.Add(1)
			tick := int(time.Since(origin) / scheduleFixtureCadence)
			observation := alerts.ConnectionSnapshot{
				ID: interruptedID, Name: "fixture-branch", Type: alerts.ConnectionTypePVE, Enabled: true,
				State: alerts.ConnectionStateUnreachable, StateReason: "No response from the connection",
			}
			if tick == 1 || tick == 4 || tick >= 9 {
				observation.State, observation.StateReason = alerts.ConnectionStateActive, ""
			}
			if tick == 4 {
				// The healthy reading has already been taken. Simulate a
				// blocked remainder of that pass before it applies.
				close(held)
				<-release
			}
			return []alerts.ConnectionSnapshot{peer, observation}
		})
		done := make(chan struct{})
		go func() {
			defer close(done)
			m.Start(ctx, nil)
		}()
		released := false
		defer func() {
			cancel()
			if !released {
				close(release)
			}
			<-done
		}()
		checkpoint := func(tick int) {
			time.Sleep(time.Until(origin.Add(time.Duration(tick)*scheduleFixtureCadence + time.Millisecond)))
			synctest.Wait()
		}
		active := func(resourceID string) *alerts.Alert {
			for _, alert := range m.alertManager.GetActiveAlerts() {
				if alert.ResourceID == resourceID {
					return &alert
				}
			}
			return nil
		}

		checkpoint(3)
		require.EqualValues(t, 3, reads.Load(), "an unblocked check uses each ordinary tick")
		require.Nil(t, active(interruptedID), "only two degraded observations so far")
		require.NotNil(t, active(peerID), "the third actual PBS observation raises its warning")
		checkpoint(6)
		select {
		case <-held:
		default:
			t.Fatal("fixture never held the healthy observation")
		}
		require.EqualValues(t, 7, polls.Load(), "provider polling must continue through the hold")
		peerAlert := active(peerID)
		require.NotNil(t, peerAlert, "keep the independent alert during the hold")
		require.Equal(t, alerts.AlertLevelWarning, peerAlert.Level)
		t.Logf("while held: reads=%d independent polls=%d interrupted dispatches=%d", reads.Load(), polls.Load(), fires.Load())
		// Report all phases on the old source, even after finding the false
		// alert, so the recovery and normal-read controls are comparable.
		if reads.Load() != 4 || active(interruptedID) != nil || fires.Load() != 0 {
			t.Error("newer ticks bypassed the held healthy observation")
		}
		close(release)
		released = true
		synctest.Wait()
		if reads.Load() != 5 || active(interruptedID) != nil {
			t.Error("release must apply the held reading then one fresh check, not replay each missed tick")
		}
		checkpoint(7)
		if active(interruptedID) != nil {
			t.Error("two checks after the healthy observation must not raise a new alert")
		}
		checkpoint(8)
		require.NotNil(t, active(interruptedID), "a real sustained outage still raises an alert")
		require.Equal(t, alerts.AlertLevelCritical, active(interruptedID).Level)
		require.EqualValues(t, 1, fires.Load(), "one dispatch for the confirmed outage")
		checkpoint(10)
		require.NotNil(t, active(interruptedID), "two healthy observations must not clear an active outage")
		checkpoint(11)
		require.Nil(t, active(interruptedID), "three healthy observations resolve the outage")
		require.NotNil(t, active(peerID), "recovering one connection must not clear its peer")
	})
}

func TestMonitorConnectionAlertChecksDrainWithoutStartingPendingWork(t *testing.T) {
	previousMock := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previousMock) })

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		m := newScheduleFixture(ctx, false, func(context.Context) ([]proxmox.Node, error) { return nil, nil })
		var admitted, returned atomic.Int32
		release := make(chan struct{})
		m.SetConnectionsSnapshotLister(func() []alerts.ConnectionSnapshot {
			admitted.Add(1)
			<-release
			returned.Add(1)
			return nil
		})
		done := make(chan struct{})
		go func() {
			defer close(done)
			m.Start(ctx, nil)
		}()
		released := false
		defer func() {
			cancel()
			if !released {
				close(release)
			}
			<-done
		}()
		time.Sleep(5*scheduleFixtureCadence + time.Millisecond)
		synctest.Wait()
		beforeStop := admitted.Load()
		cancel()
		synctest.Wait()
		select {
		case <-done:
			t.Error("Start returned while an accepted check still owned the alert store")
		default:
		}
		close(release)
		released = true
		<-done
		t.Logf("shutdown: entered before stop=%d entered in total=%d returned=%d", beforeStop, admitted.Load(), returned.Load())
		require.EqualValues(t, 1, beforeStop, "five ticks must not create five waiting reads")
		require.EqualValues(t, 1, admitted.Load(), "pending wake must not read after cancellation")
		require.EqualValues(t, 1, returned.Load(), "the accepted read must finish")
	})
}
