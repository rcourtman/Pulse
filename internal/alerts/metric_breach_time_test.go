package alerts

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Decode the public field rather than sharing the evaluator's choice of date.
// These controls also compile against a source which does not expose it yet.
func metricBreachTime(t *testing.T, status *models.MetricAlertStatus) time.Time {
	t.Helper()
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		LastBreachAt time.Time `json:"lastBreachAt"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	return wire.LastBreachAt
}

func TestMetricBreachTimeTracksTheEvaluatedBreach(t *testing.T) {
	m, clock := newMetricStatusTestManager(t)
	fired := fireNodeTemperatureAlert(t, m, clock, 85)
	lastBreach := fired.LastSeen
	for _, step := range []struct {
		name     string
		value    float64
		phase    string
		breaches bool
	}{
		{"held", 78, models.MetricAlertPhaseLatched, false},
		{"recovery starts", 72, models.MetricAlertPhaseRecovering, false},
		{"recovery advances", 75, models.MetricAlertPhaseRecovering, false},
		{"recovery resets", 77, models.MetricAlertPhaseLatched, false},
		{"exact trigger breaches", 80, models.MetricAlertPhaseBreaching, true},
		{"held after new breach", 78, models.MetricAlertPhaseLatched, false},
		{"higher breach", 90, models.MetricAlertPhaseBreaching, true},
		{"recovering after new breach", 74, models.MetricAlertPhaseRecovering, false},
	} {
		t.Run(step.name, func(t *testing.T) {
			clock.advance(30 * time.Second)
			checkMetricStatusNode(m, step.value)
			if step.breaches {
				lastBreach = clock.now
			}
			alert := activeNodeTemperatureAlert(t, m)
			if alert == nil || alert.MetricStatus == nil || alert.MetricStatus.Phase != step.phase {
				t.Fatalf("alert = %+v, want phase %s", alert, step.phase)
			}
			if got := metricBreachTime(t, alert.MetricStatus); !got.Equal(lastBreach) {
				t.Errorf("lastBreachAt = %v, want last evaluated breach %v", got, lastBreach)
			}
			if !alert.LastSeen.Equal(lastBreach) || !alert.MetricStatus.ObservedAt.Equal(clock.now) {
				t.Errorf("breach snapshot and live observation diverged: %+v", alert)
			}
		})
	}
}

func TestMetricBreachTimeRestoredWithoutLiveStatus(t *testing.T) {
	for _, durable := range []bool{false, true} {
		name := "recovery mirror"
		var options []ManagerOption
		if durable {
			name = "durable active state"
			options = []ManagerOption{WithDurableAlertStore()}
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			m := NewManagerWithDataDir(dir, options...)
			t.Cleanup(m.Stop)
			clock := &metricStatusClock{now: time.Now().UTC()}
			m.now = func() time.Time { return clock.now }
			m.intentClock = func() time.Duration { return clock.tick }
			cfg := m.GetConfig()
			cfg.NodeDefaults.Temperature = &HysteresisThreshold{Trigger: 80, Clear: 75}
			m.UpdateConfig(cfg)
			fired := fireNodeTemperatureAlert(t, m, clock, 85)
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			mirror := filepath.Join(dir, "alerts", "active-alerts.json")
			before, err := os.ReadFile(mirror)
			if err != nil {
				t.Fatal(err)
			}
			beforeInfo, err := os.Stat(mirror)
			if err != nil {
				t.Fatal(err)
			}

			clock.advance(time.Minute)
			checkMetricStatusNode(m, 78)
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(mirror)
			if err != nil {
				t.Fatal(err)
			}
			afterInfo, err := os.Stat(mirror)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) || !beforeInfo.ModTime().Equal(afterInfo.ModTime()) {
				t.Fatal("holding an alert rewrote its durable recovery mirror")
			}
			if bytes.Contains(after, []byte("metricStatus")) || bytes.Contains(after, []byte("lastBreachAt")) {
				t.Fatal("volatile metric status reached durable alert storage")
			}
			m.Stop()

			restored := NewManagerWithDataDir(dir, options...)
			t.Cleanup(restored.Stop)
			restored.UpdateConfig(cfg)
			clock.advance(2 * time.Minute)
			restored.now = func() time.Time { return clock.now }
			restored.intentClock = func() time.Duration { return clock.tick }
			alert := activeNodeTemperatureAlert(t, restored)
			if alert == nil || alert.MetricStatus != nil || !alert.LastSeen.Equal(fired.LastSeen) {
				t.Fatalf("restart must retain the breach, not a live reading: %+v", alert)
			}
			checkMetricStatusNode(restored, 78)
			alert = activeNodeTemperatureAlert(t, restored)
			if alert == nil || alert.MetricStatus == nil || alert.MetricStatus.Phase != models.MetricAlertPhaseLatched {
				t.Fatalf("first restored hold = %+v", alert)
			}
			if got := metricBreachTime(t, alert.MetricStatus); !got.Equal(fired.LastSeen) {
				t.Fatalf("restored hold dated breach %v, want %v, not restart time %v", got, fired.LastSeen, clock.now)
			}
			if !alert.MetricStatus.ObservedAt.Equal(clock.now) {
				t.Fatal("restored live status kept the pre-restart observation")
			}
		})
	}
}

func TestMetricBreachTimeUnknownUntilANewBreach(t *testing.T) {
	m, clock := newMetricStatusTestManager(t)
	fired := fireNodeTemperatureAlert(t, m, clock, 85)
	// An older checkpoint can have the incident and value but no date. The
	// occurrence's start or the next healthy reading cannot date that value.
	fired.LastSeen = time.Time{}
	fired.MetricStatus = nil
	if err := m.restoreActiveAlertSnapshots([]*Alert{fired}, "legacy checkpoint without breach time", true); err != nil {
		t.Fatal(err)
	}
	clock.advance(time.Minute)
	checkMetricStatusNode(m, 78)
	held := activeNodeTemperatureAlert(t, m)
	if held == nil || held.MetricStatus == nil || held.MetricStatus.Phase != models.MetricAlertPhaseLatched {
		t.Fatalf("restored unknown-date hold = %+v", held)
	}
	if got := metricBreachTime(t, held.MetricStatus); !got.IsZero() {
		t.Fatalf("unknown breach date became %v", got)
	}
	data, err := json.Marshal(held.MetricStatus)
	if err != nil || bytes.Contains(data, []byte("lastBreachAt")) {
		t.Fatalf("unknown date reached wire: %s / %v", data, err)
	}
	clock.advance(time.Minute)
	checkMetricStatusNode(m, 80)
	if got := metricBreachTime(t, activeNodeTemperatureAlert(t, m).MetricStatus); !got.Equal(clock.now) {
		t.Fatalf("new breach date = %v, want %v", got, clock.now)
	}
}
