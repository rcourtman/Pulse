package alerts

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Exercise the actual copied example through the metric runtime, not a second
// implementation of the comparisons in the guide. Delays/windows are disabled
// here to isolate eligibility; the guide explicitly retains those conditions.
func TestConfigurationHelpMetricExample(t *testing.T) {
	doc, err := os.ReadFile("../../docs/CONFIGURATION.md")
	if err != nil {
		t.Fatal(err)
	}
	_, alertsSection, ok := strings.Cut(string(doc), "## 🔔 Alerts (`alerts.json`)")
	if !ok {
		t.Fatal("missing alert help")
	}
	_, example, ok := strings.Cut(alertsSection, "```json\n")
	if !ok {
		t.Fatal("missing alert JSON example")
	}
	example, _, ok = strings.Cut(example, "\n```")
	if !ok {
		t.Fatal("unterminated alert JSON example")
	}
	var cfg AlertConfig
	if err := json.Unmarshal([]byte(example), &cfg); err != nil {
		t.Fatal(err)
	}
	threshold := cfg.GuestDefaults.CPU
	if threshold == nil || threshold.Trigger != 90 || threshold.Clear != 80 {
		t.Fatalf("copied CPU rule = %+v, update the documented boundary cases together", threshold)
	}

	for _, scenario := range []struct {
		name   string
		values []float64
		active []bool
	}{
		{"trigger equality, hold and clear equality", []float64{89.9, 90, 85, 80.1, 80}, []bool{false, true, true, true, false}},
		{"below clear", []float64{95, 79.9}, []bool{true, false}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			m := newTestManager(t)
			configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())
			for i, value := range scenario.values {
				m.checkMetric("help-vm", "Help VM", "help-node", "help-instance", "guest", "cpu", value, threshold, nil)
				id := canonicalMetricStateID("help-vm", "cpu")
				if got := testHasActiveAlert(t, m, id); got != scenario.active[i] {
					t.Fatalf("step %d value %g: active=%v, want %v", i, value, got, scenario.active[i])
				}
			}
		})
	}

	for _, off := range []struct {
		name  string
		value float64
	}{{"legacy zero is Off without recovery", 0}, {"current negative sentinel is Off without recovery", -1}} {
		t.Run(off.name, func(t *testing.T) {
			m := newTestManager(t)
			configureUnifiedEvalManager(t, m, unifiedEvalBaseConfig())
			id := canonicalMetricStateID("help-vm", "cpu")
			m.checkMetric("help-vm", "Help VM", "help-node", "help-instance", "guest", "cpu", 95, threshold, nil)
			assertAlertPresent(t, m, id)
			m.checkMetric("help-vm", "Help VM", "help-node", "help-instance", "guest", "cpu", 95, &HysteresisThreshold{Trigger: off.value}, nil)
			assertAlertMissing(t, m, id)
		})
	}
}
