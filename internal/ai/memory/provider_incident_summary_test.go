package memory

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"testing"
)

func TestProviderIncidentSummaryDoesNotInventThreshold(t *testing.T) {
	for _, typ := range []string{"resource-incident", "backup-storage-incident", "cpu"} {
		change := unifiedresources.ResourceChange{Metadata: map[string]interface{}{
			unifiedresources.MetadataAlertType:      typ,
			unifiedresources.MetadataAlertLevel:     "warning",
			unifiedresources.MetadataAlertValue:     float64(0),
			unifiedresources.MetadataAlertThreshold: float64(0),
		}}
		want := "Alert triggered: " + typ + " (warning)"
		if got := incidentEventSummaryFromChange(change, IncidentEventAlertFired); got != want {
			t.Fatalf("%s: got %q, want %q", typ, got, want)
		}
	}
}

func TestIncidentSummaryUsesSourceConditionWithoutInventingComparison(t *testing.T) {
	for _, message := range []string{"Backup datastore is 90% full and protects seven workloads", "Free space fell below the configured minimum"} {
		change := unifiedresources.ResourceChange{Reason: message, Metadata: map[string]any{
			unifiedresources.MetadataAlertType:      "backup-storage-incident",
			unifiedresources.MetadataAlertMessage:   message,
			unifiedresources.MetadataAlertValue:     0.0,
			unifiedresources.MetadataAlertThreshold: 0.0,
		}}
		if got := incidentEventSummaryFromChange(change, IncidentEventAlertFired); got != message {
			t.Fatalf("got %q, want recorded condition %q", got, message)
		}
	}
}
