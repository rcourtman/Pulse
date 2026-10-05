package monitoring

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// These deliberately use named nil collections: a nil slice/map can still
// have a custom wire representation, unlike a nil pointer marshaler.
type projectionNilSlice []string

func (projectionNilSlice) MarshalJSON() ([]byte, error) { return []byte(`["custom"]`), nil }

type projectionNilMap map[string]string

func (projectionNilMap) MarshalJSON() ([]byte, error) { return []byte(`{"custom":true}`), nil }

type projectionNilTextSlice []string

func (projectionNilTextSlice) MarshalText() ([]byte, error) { return []byte("custom-text"), nil }

type projectionNilTextMap map[string]string

func (projectionNilTextMap) MarshalText() ([]byte, error) { return []byte("custom-text"), nil }

type projectionInvalidJSON struct{}

func (projectionInvalidJSON) MarshalJSON() ([]byte, error) { return []byte(`{broken`), nil }

type projectionFailedJSON struct{}

func (projectionFailedJSON) MarshalJSON() ([]byte, error) {
	return nil, errors.New("synthetic encode failure")
}

func projectionRawJSONReference(value any) json.RawMessage {
	if value == nil {
		return nil
	}
	encoded, err := json.Marshal(value)
	if err != nil || string(encoded) == "null" {
		return nil
	}
	return encoded
}

func TestBroadcastRawJSONPreservesAbsentEmptyAndFailureSemantics(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{"untyped-nil", nil}, {"nil-facet", (*unifiedresources.K8sData)(nil)},
		{"nil-slice", []string(nil)}, {"nil-map", map[string]string(nil)},
		{"empty-slice", []string{}}, {"empty-map", map[string]string{}},
		{"named-nil-slice-marshaler", projectionNilSlice(nil)},
		{"named-nil-map-marshaler", projectionNilMap(nil)},
		{"named-nil-slice-text-marshaler", projectionNilTextSlice(nil)},
		{"named-nil-map-text-marshaler", projectionNilTextMap(nil)},
		{"nil-pointer-marshaler", (*projectionNilSlice)(nil)},
		{"invalid-json", projectionInvalidJSON{}}, {"encode-failure", projectionFailedJSON{}},
		{"invalid-float", math.NaN()}, {"raw-null", json.RawMessage(`null`)},
		{"unicode-html", map[string]string{"label": "<synthetic> & café \u2028"}},
		{"facet", &unifiedresources.K8sData{ClusterID: "synthetic", Labels: map[string]string{"env": "test"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, want := monitorRawJSON(tc.value), projectionRawJSONReference(tc.value)
			if !bytes.Equal(got, want) || (got == nil) != (want == nil) {
				t.Fatalf("raw JSON changed: got %q, want %q", got, want)
			}
		})
	}
}

func TestBroadcastAbsentFacetsDoNotAllocateNull(t *testing.T) {
	for _, value := range []any{(*unifiedresources.K8sData)(nil), (*unifiedresources.StorageMeta)(nil), []unifiedresources.ResourceIncident(nil)} {
		allocations := testing.AllocsPerRun(100, func() {
			if monitorRawJSON(value) != nil {
				panic("absent facet became present")
			}
		})
		if allocations != 0 {
			t.Fatalf("absent %T allocated discarded JSON: %.0f", value, allocations)
		}
	}
}

func TestBroadcastConversionBorrowedHelpersKeepOutputOwned(t *testing.T) {
	now := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	temperature := 42.5
	total, used := int64(1000), int64(250)
	resource := unifiedresources.Resource{ID: "pod:synthetic", Type: unifiedresources.ResourceTypePod, Name: "synthetic", LastSeen: now,
		Sources: []unifiedresources.DataSource{unifiedresources.SourceK8s}, Tags: []string{"public"},
		Identity:   unifiedresources.ResourceIdentity{Hostnames: []string{"synthetic"}, IPAddresses: []string{"192.0.2.1"}},
		Metrics:    &unifiedresources.ResourceMetrics{CPU: &unifiedresources.MetricValue{Percent: 0.25}, Memory: &unifiedresources.MetricValue{Total: &total, Used: &used}},
		Kubernetes: &unifiedresources.K8sData{ClusterID: "synthetic", PodPhase: "Running", Labels: map[string]string{"env": "synthetic"}, Temperature: &temperature, UptimeSeconds: 60},
	}
	before, err := json.Marshal(resource)
	if err != nil {
		t.Fatal(err)
	}
	projected := models.ConvertResourceToFrontend(monitorResourceToConvertInput(resource))
	projected.Tags[0] = "caller-change"
	projected.Sources[0] = "caller-change"
	projected.Labels["env"] = "caller-change"
	projected.Identity.IPs[0] = "caller-change"
	*projected.Temperature = 100
	*projected.Uptime = 100
	*projected.Memory.Total = 2000
	*projected.Memory.Used = 1000
	for _, raw := range []json.RawMessage{projected.Canonical, projected.Kubernetes, projected.MetricsTarget, projected.DiscoveryTarget, projected.Capabilities} {
		if len(raw) > 0 {
			raw[0] = 'x'
		}
	}
	after, err := json.Marshal(resource)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("converter or output caller mutated source resource")
	}
}
