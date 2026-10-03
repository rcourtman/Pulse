package alerts

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func newCleanupRetentionManager(t *testing.T, dir string, durable bool) *Manager {
	t.Helper()
	var options []ManagerOption
	if durable {
		options = append(options, WithDurableAlertStore())
	}
	m := NewManagerWithDataDir(dir, options...)
	t.Cleanup(m.Stop)
	m.mu.Lock()
	m.config.Enabled = true
	m.config.ActivationState = ActivationActive
	m.config.AutoAcknowledgeAfterHours = 0
	m.config.MaxAlertAgeDays = 1
	m.config.MaxAcknowledgedAgeDays = 1
	m.config.DockerDefaults.UpdateAlertDelayHours = 24
	m.config.Schedule.Cooldown = 0 // Only the first notification per occurrence.
	m.config.FlappingEnabled = false
	m.mu.Unlock()
	return m
}

// Exercise the actual detector, callbacks, history, checkpoint and restart,
// not just a hand-seeded active map. A cached registry observation is a current
// report; missing/failed checks must not be invented as recovery either.
func TestCleanupContinuingDockerUpdateKeepsOccurrence(t *testing.T) {
	for _, durable := range []bool{false, true} {
		name := "JSON recovery"
		if durable {
			name = "durable active authority"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			m := newCleanupRetentionManager(t, dir, durable)
			host := models.DockerHost{ID: "retention-host", Hostname: "retention-host"}
			pending := &models.DockerContainerUpdateStatus{
				UpdateAvailable: true, CurrentDigest: "sha256:old", LatestDigest: "sha256:new",
				LastChecked: time.Now().Add(-6 * time.Hour),
			}
			container := models.DockerContainer{ID: "retention-container", Name: "web", Image: "mongo:7", UpdateStatus: pending}
			resourceID := DockerResourceID(host.ID, container.ID)
			trackingKey := dockerUpdateTrackingKey(host, container)
			first := time.Now().Add(-25 * time.Hour)
			m.mu.Lock()
			m.dockerUpdateFirstSeen[resourceID] = first
			m.dockerUpdateFirstSeenByIdentity[trackingKey] = first
			m.mu.Unlock()
			var deliveries, fired, resolved atomic.Int32
			m.SetAlertCallback(func(*Alert) { deliveries.Add(1) })
			m.SubscribeLifecycleCallback(func(event LifecycleEvent) {
				switch event.Type {
				case eventlog.TypeFired, eventlog.TypeRefired:
					fired.Add(1)
				case eventlog.TypeResolved:
					resolved.Add(1)
				}
			})
			evaluate := func() {
				m.checkDockerContainerImageUpdate(host, container, resourceID, container.Name, host.Hostname, host.Hostname)
			}
			evaluate()
			active := m.GetActiveAlerts()
			if len(active) != 1 || deliveries.Load() != 1 || fired.Load() != 1 {
				t.Fatalf("initial update did not fire once: active %d, callbacks %d, firings %d", len(active), deliveries.Load(), fired.Load())
			}
			initial := active[0]
			for _, status := range []*models.DockerContainerUpdateStatus{pending, nil, {Error: "registry unavailable"}, pending} {
				container.UpdateStatus = status
				evaluate()
				m.Cleanup(time.Hour)
				if got := len(m.GetActiveAlerts()); got != 1 {
					t.Errorf("cleanup hid the continuing condition: %d active alerts", got)
				}
				container.UpdateStatus = pending
				evaluate() // The next poll must not fire a replacement occurrence.
			}
			if got := deliveries.Load(); got != 1 {
				t.Errorf("continuing update dispatched %d notifications, want one", got)
			}
			if got := fired.Load(); got != 1 || resolved.Load() != 0 {
				t.Errorf("continuing lifecycle has %d firings / %d resolutions, want 1 / 0", got, resolved.Load())
			}
			active = m.GetActiveAlerts()
			if len(active) != 1 || active[0].ID != initial.ID || !active[0].StartTime.Equal(first) || active[0].Acknowledged || active[0].LastNotified == nil || !active[0].LastNotified.Equal(*initial.LastNotified) {
				t.Fatalf("continuing occurrence/notification identity changed: %+v", active)
			}
			if got := len(m.GetAlertHistory(0)); got != 1 {
				t.Errorf("continuing condition produced %d history rows, want one", got)
			}
			if durable {
				events := queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired, eventlog.TypeRefired, eventlog.TypeResolved}})
				if len(events) != 1 || events[0].Type != eventlog.TypeFired {
					t.Errorf("durable lifecycle invented transitions during cleanup: %+v", events)
				}
			}
			t.Logf("four cleanup/next-poll cycles: %d firing callbacks, %d dispatch callbacks", fired.Load(), deliveries.Load())
			lastSeen := active[0].LastSeen
			m.Stop() // Includes the real final active-state checkpoint.

			restarted := newCleanupRetentionManager(t, dir, durable)
			active = restarted.GetActiveAlerts()
			if len(active) != 1 || active[0].ID != initial.ID || !active[0].StartTime.Equal(first) || !active[0].LastSeen.Equal(lastSeen) || active[0].Acknowledged || active[0].LastNotified == nil || !active[0].LastNotified.Equal(*initial.LastNotified) {
				t.Fatalf("restart did not preserve the continuing unacknowledged occurrence: %+v", active)
			}
			var afterRestartDeliveries, afterRestartFirings, afterRestartResolutions atomic.Int32
			restarted.SetAlertCallback(func(*Alert) { afterRestartDeliveries.Add(1) })
			restarted.SubscribeLifecycleCallback(func(event LifecycleEvent) {
				switch event.Type {
				case eventlog.TypeFired, eventlog.TypeRefired:
					afterRestartFirings.Add(1)
				case eventlog.TypeResolved:
					afterRestartResolutions.Add(1)
				}
			})
			container.UpdateStatus = nil
			restarted.checkDockerContainerImageUpdate(host, container, resourceID, container.Name, host.Hostname, host.Hostname)
			restarted.Cleanup(time.Hour)
			if len(restarted.GetActiveAlerts()) != 1 || afterRestartFirings.Load() != 0 || afterRestartDeliveries.Load() != 0 {
				t.Fatal("restart/unknown report lost or re-fired the occurrence")
			}
			container.UpdateStatus = &models.DockerContainerUpdateStatus{LastChecked: time.Now()}
			restarted.checkDockerContainerImageUpdate(host, container, resourceID, container.Name, host.Hostname, host.Hostname)
			if len(restarted.GetActiveAlerts()) != 0 || afterRestartResolutions.Load() != 1 || afterRestartDeliveries.Load() != 0 {
				t.Fatal("affirmative clear did not resolve once without another firing notification")
			}
			history := restarted.GetAlertHistory(0)
			if len(history) != 1 || history[0].OperationalRecord == nil || history[0].OperationalRecord.ResolvedAt == nil {
				t.Fatalf("affirmative clear did not preserve one resolved history occurrence: %+v", history)
			}
			t.Log("persisted restart preserved identity, age, last observation and dispatch time; affirmative clear resolved once")
		})
	}
}
