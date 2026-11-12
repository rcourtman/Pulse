package reducer

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCollectPendingResourceIDs(t *testing.T) {
	s := NewState()
	at := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	for _, resourceID := range []string{"agent:one/disk:a", "agent:one/disk:b", "agent:one-other/disk:a", ""} {
		for _, key := range []string{"cpu", "memory"} {
			s.ApplyMetric(MetricSignal{ResourceID: resourceID, Metric: key, Value: 85, ObservedAt: at}, MetricRule{Trigger: 80, DelaySeconds: 60})
		}
	}
	s.ApplyDiscrete(DiscreteSignal{ResourceID: "agent:one/disk:a", Key: "health", Matched: true, ObservedAt: at}, DiscreteRule{Confirmations: 3})
	s.SeedFiringIncident("agent:one/firing", "cpu", SeverityCritical, at, true, "operator", at)
	before := make(map[string]Incident)
	for key, incident := range s.incidents {
		before[key] = *incident
	}
	for _, tc := range []struct {
		name    string
		include func(string) bool
		want    map[string]struct{}
	}{
		{"all", nil, map[string]struct{}{"agent:one/disk:a": {}, "agent:one/disk:b": {}, "agent:one-other/disk:a": {}, "": {}}},
		{"one-host", func(id string) bool { return strings.HasPrefix(id, "agent:one/") }, map[string]struct{}{"agent:one/disk:a": {}, "agent:one/disk:b": {}}},
		{"none", func(string) bool { return false }, map[string]struct{}{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := map[string]struct{}{"already-collected": {}}
			s.CollectPendingResourceIDs(got, tc.include)
			s.CollectPendingResourceIDs(got, tc.include)
			want := map[string]struct{}{"already-collected": {}}
			for id := range tc.want {
				want[id] = struct{}{}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pending resources = %v, want %v", got, want)
			}
			for key, incident := range s.incidents {
				if !reflect.DeepEqual(*incident, before[key]) {
					t.Fatalf("collection changed incident %q", key)
				}
			}
			if len(s.incidents) != len(before) {
				t.Fatal("collection added or removed an incident")
			}
		})
	}

	// A later collection reads current state, not an independent cache that
	// can keep a cleared or firing resource pending.
	s.DropPendingForResource("agent:one/disk:a")
	for _, metric := range []string{"cpu", "memory"} {
		s.ApplyMetric(MetricSignal{ResourceID: "agent:one/disk:b", Metric: metric, Value: 85, ObservedAt: at.Add(time.Minute)}, MetricRule{Trigger: 80, DelaySeconds: 60})
	}
	got := make(map[string]struct{})
	s.CollectPendingResourceIDs(got, func(id string) bool { return strings.HasPrefix(id, "agent:one/") })
	if len(got) != 0 {
		t.Fatalf("cleared/firing resources remain pending: %v", got)
	}
	s.Reset()
	got = make(map[string]struct{})
	s.CollectPendingResourceIDs(got, nil)
	if len(got) != 0 {
		t.Fatalf("reset retained resources: %v", got)
	}
}
