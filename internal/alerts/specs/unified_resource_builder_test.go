package specs

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestBuildUnifiedResourceAlertSpecsPreservesCanonicalResourceIDs(t *testing.T) {
	parentID := "agent:truenas-main"
	childID := "storage:tank"
	resources := []unifiedresources.Resource{
		{
			ID:   parentID,
			Type: unifiedresources.ResourceTypeAgent,
			Name: "truenas-main",
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "truenas",
				NativeID: "alert-1",
				Code:     "truenas_volume_status",
				Severity: storagehealth.RiskCritical,
				Summary:  "Pool tank is FAULTED",
			}},
		},
		{
			ID:       childID,
			Type:     unifiedresources.ResourceTypeStorage,
			Name:     "tank",
			ParentID: &parentID,
			Incidents: []unifiedresources.ResourceIncident{{
				Provider: "truenas",
				NativeID: "alert-1",
				Code:     "truenas_volume_status",
				Severity: storagehealth.RiskCritical,
				Summary:  "Pool tank is FAULTED",
			}},
		},
	}

	specs := BuildUnifiedResourceAlertSpecs(resources)
	if len(specs) != 4 {
		t.Fatalf("expected 4 specs, got %d", len(specs))
	}

	for _, spec := range specs {
		switch spec.ResourceID {
		case parentID:
			if spec.ResourceID != parentID {
				t.Fatalf("parent resource id = %q, want %q", spec.ResourceID, parentID)
			}
		case childID:
			if spec.ResourceID != childID {
				t.Fatalf("child resource id = %q, want %q", spec.ResourceID, childID)
			}
			if spec.ParentResourceID != parentID {
				t.Fatalf("child parent id = %q, want %q", spec.ParentResourceID, parentID)
			}
		default:
			t.Fatalf("unexpected resource id %q", spec.ResourceID)
		}
	}
}

func TestBuildUnifiedResourceAlertSpecsProviderIncidentIDsStable(t *testing.T) {
	startedAt := time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
	resource := unifiedresources.Resource{
		ID:   "storage:tank",
		Type: unifiedresources.ResourceTypeStorage,
		Name: "tank",
		Incidents: []unifiedresources.ResourceIncident{{
			Provider:  " truenas ",
			NativeID:  " alert-1 ",
			Code:      " truenas_volume_status ",
			Severity:  storagehealth.RiskWarning,
			Source:    " alerts-api ",
			Summary:   "Pool tank is DEGRADED",
			StartedAt: startedAt,
		}},
	}

	first := findSpec(t, BuildUnifiedResourceAlertSpecs([]unifiedresources.Resource{resource}), AlertSpecKindProviderIncident, resource.ID)

	updated := resource
	updated.Incidents = append([]unifiedresources.ResourceIncident(nil), resource.Incidents...)
	updated.Incidents[0].Summary = "Pool tank is still DEGRADED"
	updated.Incidents[0].StartedAt = startedAt.Add(30 * time.Minute)
	updated.Incidents[0].Source = "events-stream"

	second := findSpec(t, BuildUnifiedResourceAlertSpecs([]unifiedresources.Resource{updated}), AlertSpecKindProviderIncident, resource.ID)

	if first.ID != second.ID {
		t.Fatalf("provider incident spec id changed across rebuilds: %q vs %q", first.ID, second.ID)
	}
	if len(first.SuppressionKeys) != 1 {
		t.Fatalf("expected one suppression key, got %+v", first.SuppressionKeys)
	}
	if got := first.SuppressionKeys[0]; got != "truenas|alert-1|truenas_volume_status" {
		t.Fatalf("suppression key = %q", got)
	}
}

func TestBuildUnifiedResourceAlertSpecsExposeHierarchyAndSuppressionKeys(t *testing.T) {
	parentID := "pbs:main"
	childID := "storage:fast"
	sharedIncident := unifiedresources.ResourceIncident{
		Provider: "pulse",
		NativeID: "pbs-instance:pbs-main:capacity_runway_low",
		Code:     "capacity_runway_low",
		Severity: storagehealth.RiskCritical,
		Summary:  "PBS datastore fast is 96% full",
	}
	resources := []unifiedresources.Resource{
		{
			ID:        parentID,
			Type:      unifiedresources.ResourceTypePBS,
			Name:      "pbs-main",
			Incidents: []unifiedresources.ResourceIncident{sharedIncident},
		},
		{
			ID:        childID,
			Type:      unifiedresources.ResourceTypeStorage,
			Name:      "fast",
			ParentID:  &parentID,
			Incidents: []unifiedresources.ResourceIncident{sharedIncident},
		},
	}

	specs := BuildUnifiedResourceAlertSpecs(resources)
	parentRollup := findSpec(t, specs, AlertSpecKindResourceIncidentRollup, parentID)
	childIncident := findSpec(t, specs, AlertSpecKindProviderIncident, childID)

	if got := parentRollup.ChildResourceIDs; len(got) != 1 || got[0] != childID {
		t.Fatalf("parent child ids = %+v, want [%q]", got, childID)
	}
	if childIncident.ParentResourceID != parentID {
		t.Fatalf("child parent id = %q, want %q", childIncident.ParentResourceID, parentID)
	}
	if len(parentRollup.SuppressionKeys) != 1 || len(childIncident.SuppressionKeys) != 1 {
		t.Fatalf("expected shared suppression keys, got parent=%+v child=%+v", parentRollup.SuppressionKeys, childIncident.SuppressionKeys)
	}
	if parentRollup.SuppressionKeys[0] != childIncident.SuppressionKeys[0] {
		t.Fatalf("suppression keys do not match: parent=%q child=%q", parentRollup.SuppressionKeys[0], childIncident.SuppressionKeys[0])
	}
	if parentRollup.ResourceIncidentRollup == nil || parentRollup.ResourceIncidentRollup.IncidentCount != 1 {
		t.Fatalf("parent rollup = %+v", parentRollup.ResourceIncidentRollup)
	}
}

func findSpec(t *testing.T, specs []ResourceAlertSpec, kind AlertSpecKind, resourceID string) ResourceAlertSpec {
	t.Helper()

	for _, spec := range specs {
		if spec.Kind == kind && spec.ResourceID == resourceID {
			return spec
		}
	}

	t.Fatalf("missing spec kind=%q resource=%q in %+v", kind, resourceID, specs)
	return ResourceAlertSpec{}
}
