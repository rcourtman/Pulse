package alerts

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// ageDockerUpdateStateForTest moves the manager's recorded instants back by d,
// which is how the wall clock advancing by d looks from inside the manager.
// The alerts package reads time.Now() directly, so a multi-day run is
// simulated by ageing state rather than by sleeping.
func ageDockerUpdateStateForTest(m *Manager, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, alert := range m.activeAlerts {
		if alert == nil {
			continue
		}
		alert.StartTime = alert.StartTime.Add(-d)
		alert.LastSeen = alert.LastSeen.Add(-d)
		if alert.LastNotified != nil {
			at := alert.LastNotified.Add(-d)
			alert.LastNotified = &at
		}
	}
	for _, tracked := range []map[string]time.Time{m.dockerUpdateFirstSeen, m.dockerUpdateFirstSeenByIdentity, m.dockerUpdateLastObserved} {
		for key, at := range tracked {
			tracked[key] = at.Add(-d)
		}
	}
}

// An update that stays asserted must hold one open alert for days. Reports
// arrive every 30 minutes and the hourly housekeeping runs between them, as on
// a live server.
//
// #2353: on v6.3.2 the hourly sweep expired update tracking by first
// detection, the next report restarted the delay below its threshold without
// refreshing the open alert, and the stale-alert sweep resolved it a day
// later, just before the restarted delay re-fired it. Two independent repairs
// now hold the alert open; each case below fails when one of them regresses:
//   - tracking expiry must follow the latest observation, or a 48h delay
//     restarts every day and never opens;
//   - an open alert's start must stand in for tracking a restart lost, or the
//     restarted delay stops refreshing the alert and the stale-alert sweep
//     resolves it before the delay re-opens it.
func TestDockerUpdateAlertDoesNotCycleDailyWhileAsserted(t *testing.T) {
	cases := []struct {
		name          string
		delayHours    int
		trackingLost  time.Duration // restart: in-memory tracking dropped, saved alert kept
		simulateHours int
	}{
		{name: "24h delay", delayHours: 24, simulateHours: 96},
		{name: "48h delay", delayHours: 48, simulateHours: 120},
		{name: "24h delay across a restart", delayHours: 24, trackingLost: 30 * time.Hour, simulateHours: 96},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestManager(t)
			m.mu.Lock()
			m.config.DockerDefaults.UpdateAlertDelayHours = tc.delayHours
			m.mu.Unlock()

			host := models.DockerHost{ID: "host", AgentID: "agent", Hostname: "host", Containers: []models.DockerContainer{{
				ID: "container", Name: "web", Image: "mongo:7", State: "running",
				UpdateStatus: &models.DockerContainerUpdateStatus{
					UpdateAvailable: true, CurrentDigest: "sha256:old", LatestDigest: "sha256:new", LastChecked: time.Now(),
				},
			}}}

			var firings, resolutions atomic.Int32
			m.SubscribeLifecycleCallback(func(event LifecycleEvent) {
				switch event.Type {
				case eventlog.TypeFired, eventlog.TypeRefired:
					firings.Add(1)
				case eventlog.TypeResolved:
					resolutions.Add(1)
				}
			})

			delay := time.Duration(tc.delayHours) * time.Hour
			const step = 15 * time.Minute
			var openedStart time.Time
			for elapsed := time.Duration(0); elapsed <= time.Duration(tc.simulateHours)*time.Hour; elapsed += step {
				if elapsed > 0 {
					ageDockerUpdateStateForTest(m, step)
				}
				if tc.trackingLost > 0 && elapsed == tc.trackingLost {
					m.mu.Lock()
					clear(m.dockerUpdateFirstSeen)
					clear(m.dockerUpdateFirstSeenByIdentity)
					clear(m.dockerUpdateLastObserved)
					m.mu.Unlock()
				}
				// The server is down for the two hours before the restart:
				// no reports and no housekeeping.
				down := tc.trackingLost > 0 && elapsed >= tc.trackingLost-2*time.Hour && elapsed < tc.trackingLost
				switch {
				case down:
				case elapsed%time.Hour == 0, elapsed%time.Hour == 30*time.Minute:
					m.CheckDockerHost(host)
				case elapsed%time.Hour == 15*time.Minute:
					m.cleanupStaleMaps()
				}
				active := m.GetActiveAlerts()
				if elapsed < delay {
					if len(active) != 0 {
						t.Fatalf("%s: alert opened before its %s delay: %+v", elapsed, delay, active)
					}
					continue
				}
				if len(active) != 1 {
					t.Fatalf("%s: asserted update alert not open (firings=%d resolutions=%d)", elapsed, firings.Load(), resolutions.Load())
				}
				// Ageing moves the start back with the clock, so start+elapsed
				// is fixed for as long as the occurrence continues.
				start := active[0].StartTime.Add(elapsed)
				if openedStart.IsZero() {
					openedStart = start
				} else if d := start.Sub(openedStart); d > time.Second || d < -time.Second {
					t.Fatalf("%s: incident restarted (start moved by %s)", elapsed, d)
				}
			}
			if firings.Load() != 1 || resolutions.Load() != 0 {
				t.Fatalf("firings=%d resolutions=%d, want one opening and no resolution", firings.Load(), resolutions.Load())
			}

			// A genuine update still resolves.
			host.Containers[0].UpdateStatus = &models.DockerContainerUpdateStatus{
				UpdateAvailable: false, CurrentDigest: "sha256:new", LatestDigest: "sha256:new", LastChecked: time.Now(),
			}
			m.CheckDockerHost(host)
			if got := m.GetActiveAlerts(); len(got) != 0 || resolutions.Load() != 1 {
				t.Fatalf("current image did not resolve the alert: %+v resolutions=%d", got, resolutions.Load())
			}
		})
	}
}

// Removing the container still resolves an open update alert.
func TestDockerUpdateAlertResolvesWhenContainerRemoved(t *testing.T) {
	m := newTestManager(t)
	m.mu.Lock()
	m.config.DockerDefaults.UpdateAlertDelayHours = 24
	m.mu.Unlock()
	host := models.DockerHost{ID: "host", AgentID: "agent", Hostname: "host", Containers: []models.DockerContainer{{
		ID: "container", Name: "web", Image: "mongo:7", State: "running",
		UpdateStatus: &models.DockerContainerUpdateStatus{
			UpdateAvailable: true, CurrentDigest: "sha256:old", LatestDigest: "sha256:new", LastChecked: time.Now(),
		},
	}}}
	m.CheckDockerHost(host)
	ageDockerUpdateStateForTest(m, 25*time.Hour)
	m.CheckDockerHost(host)
	if got := m.GetActiveAlerts(); len(got) != 1 {
		t.Fatalf("pending update past its delay did not open: %+v", got)
	}
	host.Containers = nil
	m.CheckDockerHost(host)
	if got := m.GetActiveAlerts(); len(got) != 0 {
		t.Fatalf("removed container kept its update alert: %+v", got)
	}
}
