package alerts

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/eventlog"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts/reducer"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const shadowGuestID = "pve:node:101"

func shadowGuestManager(t *testing.T, grace bool) (*Manager, *atomic.Int64) {
	t.Helper()
	m := newShadowFeedManager(t)
	configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())
	epoch := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	tick := &atomic.Int64{}
	m.mu.Lock()
	m.now = func() time.Time { return epoch.Add(time.Duration(tick.Load())) }
	m.intentClock = func() time.Duration { return time.Duration(tick.Load()) }
	m.mu.Unlock()
	if grace {
		document := NewAlertIntentPolicyDocument()
		seconds := 600
		document.Defaults[string(AlertIntentSignalOffline)] = AlertIntentRule{GraceSeconds: &seconds}
		if err := m.LoadIntentPolicies(document); err != nil {
			t.Fatal(err)
		}
	}
	return m, tick
}

func observeShadowGuest(m *Manager, kind, status string, onBoot bool) {
	if kind == "container" {
		m.CheckGuest(models.Container{
			ID: shadowGuestID, VMID: 101, Name: "database", Node: "node", Instance: "pve",
			Status: status, OnBoot: &onBoot,
		}, "pve")
		return
	}
	m.CheckGuest(models.VM{
		ID: shadowGuestID, VMID: 101, Name: "database", Node: "node", Instance: "pve",
		Status: status, OnBoot: &onBoot,
	}, "pve")
}

func assertShadowGuestParity(t *testing.T, m *Manager, wantFiring bool) {
	t.Helper()
	active := m.GetActiveAlerts()
	if len(active) != 0 && (len(active) != 1 || active[0].ResourceID != shadowGuestID) {
		t.Fatalf("unexpected live alerts: %+v", active)
	}
	if got := len(active) == 1; got != wantFiring {
		t.Fatalf("live firing = %t, want %t", got, wantFiring)
	}
	if got := m.ShadowDivergences(); got != 0 {
		t.Fatalf("false shadow divergences = %d: %+v", got, shadowDivergenceEvents(t, m))
	}
	if events := shadowDivergenceEvents(t, m); len(events) != 0 {
		t.Fatalf("false shadow events: %+v", events)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	core, coreOK := m.core.Incident(shadowGuestID, canonicalPoweredStateSpecID(shadowGuestID))
	shadow, shadowOK := m.shadow.state.Incident(shadowGuestID, canonicalPoweredStateSpecID(shadowGuestID))
	if coreOK != shadowOK || (coreOK && (core.State != shadow.State || core.Confirmations != shadow.Confirmations ||
		!core.PendingSince.Equal(shadow.PendingSince) || !core.StartedAt.Equal(shadow.StartedAt))) {
		t.Fatalf("guest reducers differ: core=%+v (exists=%t), shadow=%+v (exists=%t)", core, coreOK, shadow, shadowOK)
	}
}

// Regression for #3280: a running, paused, suspended or deliberately stopped
// guest interrupts the first pending run. A later stop must satisfy its own
// full grace/confirmation run; the shadow must not reuse the old evidence.
func TestShadowGuestRecoveryRestartsPendingRun(t *testing.T) {
	for _, kind := range []string{"vm", "container"} {
		for _, policy := range []string{"600-second-grace", "two-confirmations"} {
			for _, healthy := range []string{"running", "paused", "suspended", "onboot-disabled"} {
				t.Run(kind+"/"+policy+"/"+healthy, func(t *testing.T) {
					grace := policy == "600-second-grace"
					m, tick := shadowGuestManager(t, grace)
					var notifications atomic.Int64
					m.SetAlertCallback(func(*Alert) { notifications.Add(1) })
					observeShadowGuest(m, kind, "stopped", true)
					assertShadowGuestParity(t, m, false)
					tick.Store(int64(5 * time.Minute))
					if healthy == "onboot-disabled" {
						observeShadowGuest(m, kind, "stopped", false)
					} else {
						observeShadowGuest(m, kind, healthy, true)
					}
					tick.Store(int64(10 * time.Minute))
					observeShadowGuest(m, kind, "stopped", true)
					assertShadowGuestParity(t, m, false)
					if notifications.Load() != 0 || len(m.GetAlertHistory(100)) != 0 {
						t.Fatal("interrupted grace/confirmation run fired or notified")
					}
					if grace {
						tick.Store(int64(20*time.Minute - time.Second))
						observeShadowGuest(m, kind, "stopped", true)
						assertShadowGuestParity(t, m, false)
					}
					tick.Store(int64(20 * time.Minute))
					observeShadowGuest(m, kind, "stopped", true)
					assertShadowGuestParity(t, m, true)
					if got := notifications.Load(); got != 1 {
						t.Fatalf("notifications after genuine activation = %d, want 1", got)
					}
					if events := queryAlertEvents(t, m, eventlog.Filter{Types: []string{eventlog.TypeFired}}); len(events) != 1 {
						t.Fatalf("genuine firing events = %d, want 1", len(events))
					}
				})
			}
		}
	}
}

// Exercise active and restart-seeded shadow state as well as no-active-alert
// recovery. Expected-offline continues to hold activation after recovery;
// unsuppression must still wait the full new grace and fire normally.
func TestShadowGuestRecoveryAfterFiringAndFeedRestart(t *testing.T) {
	for _, kind := range []string{"vm", "container"} {
		for _, restart := range []bool{false, true} {
			name := kind + "/continuous-feed"
			if restart {
				name = kind + "/reseeded-feed"
			}
			t.Run(name, func(t *testing.T) {
				m, tick := shadowGuestManager(t, true)
				observeShadowGuest(m, kind, "stopped", true)
				tick.Store(int64(10 * time.Minute))
				observeShadowGuest(m, kind, "stopped", true)
				assertShadowGuestParity(t, m, true)
				if err := m.AcknowledgeAlert(canonicalPoweredStateStateID(shadowGuestID), "operator"); err != nil {
					t.Fatal(err)
				}
				if restart {
					m.EnableShadowFeed()
				}
				tick.Store(int64(11 * time.Minute))
				observeShadowGuest(m, kind, "running", true)
				assertShadowGuestParity(t, m, false)
				m.SetOperatorIntentContextResolver(func(string, time.Time) (OperatorIntentContext, bool) {
					return OperatorIntentContext{MonitoringMode: "expected_offline"}, true
				})
				tick.Store(int64(12 * time.Minute))
				observeShadowGuest(m, kind, "stopped", true)
				assertShadowGuestParity(t, m, false)
				tick.Store(int64(13 * time.Minute))
				observeShadowGuest(m, kind, "running", true)
				m.SetOperatorIntentContextResolver(nil)
				tick.Store(int64(14 * time.Minute))
				observeShadowGuest(m, kind, "stopped", true)
				assertShadowGuestParity(t, m, false)
				tick.Store(int64(24*time.Minute - time.Second))
				observeShadowGuest(m, kind, "stopped", true)
				assertShadowGuestParity(t, m, false)
				tick.Store(int64(24 * time.Minute))
				observeShadowGuest(m, kind, "stopped", true)
				assertShadowGuestParity(t, m, true)
				m.mu.RLock()
				incident, ok := m.shadow.state.Incident(shadowGuestID, canonicalPoweredStateSpecID(shadowGuestID))
				m.mu.RUnlock()
				if !ok || incident.State != reducer.StateFiring || !incident.Acknowledged {
					t.Fatalf("re-fire lost existing acknowledgement semantics: %+v", incident)
				}
			})
		}
	}
}
