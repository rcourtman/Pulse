package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestCheckHostHonoursCanonicalResourceIntentGrace(t *testing.T) {
	m := newTestManager(t)
	start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	now := start
	var tick time.Duration
	m.now = func() time.Time { return now }
	m.intentClock = func() time.Duration { return tick }

	m.mu.Lock()
	m.config.TimeThresholds = map[string]int{}
	m.config.AgentDefaults = ThresholdConfig{
		CPU: &HysteresisThreshold{Trigger: 80, Clear: 75},
	}
	m.mu.Unlock()

	document := NewAlertIntentPolicyDocument()
	document.Resources["agent-canonical"] = map[string]AlertIntentRule{
		MetricAlertIntentSignal("cpu"): {GraceSeconds: intPointer(240)},
	}
	if err := m.LoadIntentPolicies(document); err != nil {
		t.Fatal(err)
	}
	m.SetResourceIntentIdentityResolver(func(resourceID string) (string, bool) {
		if resourceID == "agent:host-1" {
			return "agent-canonical", true
		}
		return "", false
	})

	host := models.Host{
		ID:       "host-1",
		Hostname: "host-1",
		CPUUsage: 90,
		Status:   "online",
		LastSeen: start,
	}
	alertID := canonicalMetricStateID(hostResourceID(host.ID), "cpu")

	m.CheckHost(host)
	m.mu.RLock()
	_, exists := m.activeAlerts[alertID]
	m.mu.RUnlock()
	if exists {
		t.Fatal("alert activated before the canonical resource grace period started")
	}

	now = start.Add(239 * time.Second)
	tick = 239 * time.Second
	m.CheckHost(host)
	m.mu.RLock()
	_, exists = m.activeAlerts[alertID]
	m.mu.RUnlock()
	if exists {
		t.Fatal("alert activated before the canonical resource grace period elapsed")
	}

	now = start.Add(240 * time.Second)
	tick = 240 * time.Second
	m.CheckHost(host)
	m.mu.RLock()
	alert := m.activeAlerts[alertID]
	if alert == nil {
		m.mu.RUnlock()
		t.Fatal("alert did not activate when the canonical resource grace period elapsed")
	}
	alertStart := alert.StartTime
	m.mu.RUnlock()
	if !alertStart.Equal(start) {
		t.Fatalf("alert StartTime = %s, want first match %s", alertStart, start)
	}
}
