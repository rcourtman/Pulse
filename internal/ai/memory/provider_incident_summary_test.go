package memory

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"testing"
)

func TestProviderIncidentSummaryDoesNotInventThreshold(t *testing.T) {
	for _, typ := range []string{"resource-incident", "cpu"} {
		change := unifiedresources.ResourceChange{Metadata: map[string]interface{}{
			unifiedresources.MetadataAlertType:      typ,
			unifiedresources.MetadataAlertLevel:     "warning",
			unifiedresources.MetadataAlertValue:     float64(0),
			unifiedresources.MetadataAlertThreshold: float64(0),
		}}
		want := "Alert triggered: resource-incident (warning)"
		if typ == "cpu" {
			want = "Alert triggered: cpu (warning 0.0 >= 0.0)"
		}
		if got := incidentEventSummaryFromChange(change, IncidentEventAlertFired); got != want {
			t.Fatalf("%s: got %q, want %q", typ, got, want)
		}
	}
}
