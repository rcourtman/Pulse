package agentcontext

import (
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// TestFormatKubernetesServicePortsBranchCov exercises every branch of
// formatKubernetesServicePorts: the empty-input early return, the regular
// formatting arm in the bounded loop, and the truncation ("N more") arm
// triggered once the loop index reaches the internal cap of 5.
func TestFormatKubernetesServicePortsBranchCov(t *testing.T) {
	five := []unified.K8sServicePort{
		{Port: 1, TargetPort: "a", Protocol: "TCP"},
		{Port: 2, TargetPort: "b", Protocol: "TCP"},
		{Port: 3, TargetPort: "c", Protocol: "TCP"},
		{Port: 4, TargetPort: "d", Protocol: "TCP"},
		{Port: 5, TargetPort: "e", Protocol: "TCP"},
	}

	tests := []struct {
		name  string
		ports []unified.K8sServicePort
		want  string
	}{
		{
			name:  "nil slice returns empty string",
			ports: nil,
			want:  "",
		},
		{
			name:  "empty slice returns empty string",
			ports: []unified.K8sServicePort{},
			want:  "",
		},
		{
			name: "single port formats as port:targetPort/protocol",
			ports: []unified.K8sServicePort{
				{Name: "http", Port: 80, TargetPort: "http", Protocol: "TCP", NodePort: 30080},
			},
			want: "80:http/TCP",
		},
		{
			name: "two ports joined with comma-space",
			ports: []unified.K8sServicePort{
				{Port: 80, TargetPort: "http", Protocol: "TCP"},
				{Port: 443, TargetPort: "https", Protocol: "TCP"},
			},
			want: "80:http/TCP, 443:https/TCP",
		},
		{
			name:  "exactly five ports renders every entry without truncation",
			ports: five,
			want:  "1:a/TCP, 2:b/TCP, 3:c/TCP, 4:d/TCP, 5:e/TCP",
		},
		{
			name: "six ports truncates to five plus one more suffix",
			ports: append(append([]unified.K8sServicePort{}, five...),
				unified.K8sServicePort{Port: 6, TargetPort: "f", Protocol: "TCP"},
			),
			want: "1:a/TCP, 2:b/TCP, 3:c/TCP, 4:d/TCP, 5:e/TCP, 1 more",
		},
		{
			name: "eight ports truncates to five plus three more suffix",
			ports: append(append([]unified.K8sServicePort{}, five...),
				unified.K8sServicePort{Port: 6, TargetPort: "f", Protocol: "TCP"},
				unified.K8sServicePort{Port: 7, TargetPort: "g", Protocol: "TCP"},
				unified.K8sServicePort{Port: 8, TargetPort: "h", Protocol: "TCP"},
			),
			want: "1:a/TCP, 2:b/TCP, 3:c/TCP, 4:d/TCP, 5:e/TCP, 3 more",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatKubernetesServicePorts(tc.ports)
			if got != tc.want {
				t.Fatalf("formatKubernetesServicePorts(%+v) = %q, want %q", tc.ports, got, tc.want)
			}
		})
	}
}

// TestAddMetricFactBranchCov exercises every branch of addMetricFact and the
// downstream empty-value skip in addAgentContextFact: the nil-metric early
// return, each arm of the metric switch (Percent, Value, Used/Total), the
// empty-value skip when no arm matches, and propagation of observedAt
// through both its nil and non-nil forms.
func TestAddMetricFactBranchCov(t *testing.T) {
	observedAt := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	t.Run("nil_metric_does_not_append", func(t *testing.T) {
		facts := []Fact{{Label: "preexisting", Value: "v"}}
		before := make([]Fact, len(facts))
		copy(before, facts)

		addMetricFact(&facts, "CPU", nil, &observedAt)

		if len(facts) != len(before) {
			t.Fatalf("nil metric appended: before=%d after=%d (%+v)", len(before), len(facts), facts)
		}
		if facts[0].Label != "preexisting" || facts[0].Value != "v" {
			t.Fatalf("existing slice mutated by nil metric: %+v", facts)
		}
	})

	t.Run("nil_slice_unchanged_when_metric_nil", func(t *testing.T) {
		var facts []Fact
		addMetricFact(&facts, "CPU", nil, &observedAt)
		if facts != nil {
			t.Fatalf("nil slice promoted to non-nil by nil metric: %+v", facts)
		}
	})

	t.Run("percent_arm_appends_percentage_with_non_nil_observedAt", func(t *testing.T) {
		facts := []Fact{}
		metric := &unified.MetricValue{Percent: 42.5}
		addMetricFact(&facts, "CPU", metric, &observedAt)

		if len(facts) != 1 {
			t.Fatalf("expected exactly 1 fact, got %d: %+v", len(facts), facts)
		}
		fact := facts[0]
		if fact.Label != "CPU" {
			t.Fatalf("Label = %q, want CPU", fact.Label)
		}
		if fact.Value != "42.5%" {
			t.Fatalf("Value = %q, want 42.5%%", fact.Value)
		}
		if fact.Source != agentContextSourceUnifiedResource {
			t.Fatalf("Source = %q, want %q", fact.Source, agentContextSourceUnifiedResource)
		}
		if fact.TrustTier != agentContextTrustRuntimeObserved {
			t.Fatalf("TrustTier = %q, want %q", fact.TrustTier, agentContextTrustRuntimeObserved)
		}
		if fact.ObservedAt == nil || !fact.ObservedAt.Equal(observedAt) {
			t.Fatalf("ObservedAt = %v, want %v", fact.ObservedAt, observedAt)
		}
		if fact.Redacted {
			t.Fatalf("Redacted = true, want false")
		}
	})

	t.Run("value_arm_appends_value_unit_with_nil_observedAt", func(t *testing.T) {
		facts := []Fact{}
		metric := &unified.MetricValue{Value: 1.5, Unit: "GB"}
		addMetricFact(&facts, "Memory", metric, nil)

		if len(facts) != 1 {
			t.Fatalf("expected exactly 1 fact, got %d: %+v", len(facts), facts)
		}
		fact := facts[0]
		if fact.Label != "Memory" {
			t.Fatalf("Label = %q, want Memory", fact.Label)
		}
		if fact.Value != "1.5 GB" {
			t.Fatalf("Value = %q, want \"1.5 GB\"", fact.Value)
		}
		if fact.ObservedAt != nil {
			t.Fatalf("ObservedAt = %v, want nil when caller passes nil", fact.ObservedAt)
		}
	})

	t.Run("value_arm_trims_surrounding_whitespace_in_unit", func(t *testing.T) {
		facts := []Fact{}
		metric := &unified.MetricValue{Value: 4.0, Unit: "  MiB "}
		addMetricFact(&facts, "Memory", metric, &observedAt)

		if len(facts) != 1 {
			t.Fatalf("expected exactly 1 fact, got %d: %+v", len(facts), facts)
		}
		if got := facts[0].Value; got != "4.0 MiB" {
			t.Fatalf("Value = %q, want \"4.0 MiB\" (unit trimmed)", got)
		}
	})

	t.Run("used_total_arm_appends_ratio_when_total_positive", func(t *testing.T) {
		facts := []Fact{}
		used := int64(100)
		total := int64(200)
		metric := &unified.MetricValue{Used: &used, Total: &total}
		addMetricFact(&facts, "Disk", metric, &observedAt)

		if len(facts) != 1 {
			t.Fatalf("expected exactly 1 fact, got %d: %+v", len(facts), facts)
		}
		if got := facts[0].Value; got != "100/200" {
			t.Fatalf("Value = %q, want \"100/200\"", got)
		}
		if got := facts[0].Label; got != "Disk" {
			t.Fatalf("Label = %q, want Disk", got)
		}
	})

	t.Run("used_total_arm_skipped_when_total_zero", func(t *testing.T) {
		facts := []Fact{}
		used := int64(50)
		total := int64(0)
		metric := &unified.MetricValue{Used: &used, Total: &total}
		addMetricFact(&facts, "Disk", metric, &observedAt)

		if len(facts) != 0 {
			t.Fatalf("expected 0 facts when Total<=0, got %d: %+v", len(facts), facts)
		}
	})

	t.Run("used_total_arm_skipped_when_total_negative", func(t *testing.T) {
		facts := []Fact{}
		used := int64(50)
		total := int64(-1)
		metric := &unified.MetricValue{Used: &used, Total: &total}
		addMetricFact(&facts, "Disk", metric, &observedAt)

		if len(facts) != 0 {
			t.Fatalf("expected 0 facts when Total<0, got %d: %+v", len(facts), facts)
		}
	})

	t.Run("used_total_arm_skipped_when_used_nil", func(t *testing.T) {
		facts := []Fact{}
		total := int64(100)
		metric := &unified.MetricValue{Total: &total}
		addMetricFact(&facts, "Disk", metric, &observedAt)

		if len(facts) != 0 {
			t.Fatalf("expected 0 facts when Used is nil, got %d: %+v", len(facts), facts)
		}
	})

	t.Run("zero_metric_produces_empty_value_and_skips_append", func(t *testing.T) {
		facts := []Fact{}
		metric := &unified.MetricValue{}
		addMetricFact(&facts, "Network in", metric, &observedAt)

		if len(facts) != 0 {
			t.Fatalf("expected 0 facts for zero metric, got %d: %+v", len(facts), facts)
		}
	})

	t.Run("percent_arm_takes_precedence_over_value_and_used_total", func(t *testing.T) {
		facts := []Fact{}
		used := int64(100)
		total := int64(200)
		metric := &unified.MetricValue{Percent: 75.0, Value: 9.9, Unit: "GB", Used: &used, Total: &total}
		addMetricFact(&facts, "CPU", metric, &observedAt)

		if len(facts) != 1 {
			t.Fatalf("expected exactly 1 fact, got %d: %+v", len(facts), facts)
		}
		if got := facts[0].Value; got != "75.0%" {
			t.Fatalf("Value = %q, want \"75.0%%\" (Percent wins)", got)
		}
	})

	t.Run("value_arm_takes_precedence_over_used_total", func(t *testing.T) {
		facts := []Fact{}
		used := int64(100)
		total := int64(200)
		metric := &unified.MetricValue{Value: 5.0, Unit: "GiB", Used: &used, Total: &total}
		addMetricFact(&facts, "Memory", metric, &observedAt)

		if len(facts) != 1 {
			t.Fatalf("expected exactly 1 fact, got %d: %+v", len(facts), facts)
		}
		if got := facts[0].Value; got != "5.0 GiB" {
			t.Fatalf("Value = %q, want \"5.0 GiB\" (Value wins over Used/Total)", got)
		}
	})

	t.Run("negative_value_takes_value_arm", func(t *testing.T) {
		facts := []Fact{}
		metric := &unified.MetricValue{Value: -3.5, Unit: "MB/s"}
		addMetricFact(&facts, "Network out", metric, &observedAt)

		if len(facts) != 1 {
			t.Fatalf("expected exactly 1 fact for negative Value, got %d: %+v", len(facts), facts)
		}
		if got := facts[0].Value; got != "-3.5 MB/s" {
			t.Fatalf("Value = %q, want \"-3.5 MB/s\"", got)
		}
	})
}
