package api

import (
	"encoding/json"
	"testing"

	unifiedresources "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// The review dialog gates its single-confirmation control on the projected
// capability class and renders blast-radius names from the projection. Both
// are read-time presentation and must never enter the persisted plan identity.
func TestContract_ActionReadProjectionCarriesCapabilityClassAndBlastRadiusPresentation(t *testing.T) {
	projection := actionAuditProjection{
		ActionAuditRecord: unifiedresources.ActionAuditRecord{
			ID:    "action-ceremony-contract",
			State: unifiedresources.ActionStatePending,
			Request: unifiedresources.ActionRequest{
				RequestID:      "request-ceremony-contract",
				ResourceID:     "app-container:7",
				CapabilityName: "update",
				Reason:         "Routine image update",
			},
			Plan: unifiedresources.ActionPlan{
				ActionID:             "action-ceremony-contract",
				PlanHash:             "sha256:stable-plan-identity",
				PredictedBlastRadius: []string{"app-container:7", "agent:host-1"},
			},
		},
		CapabilityAutoAuthorization: unifiedresources.AutoAuthorizeLowRisk,
		BlastRadius: []actionResourcePresentation{
			{ID: "app-container:7", Name: "heimdall", Type: unifiedresources.ResourceTypeAppContainer},
			{ID: "agent:host-1"},
		},
	}

	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal action read projection: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("decode action read projection: %v", err)
	}
	if wire["capabilityAutoAuthorization"] != "low_risk" {
		t.Fatalf("capabilityAutoAuthorization = %#v", wire["capabilityAutoAuthorization"])
	}
	blast, ok := wire["blastRadius"].([]any)
	if !ok || len(blast) != 2 {
		t.Fatalf("blastRadius = %#v", wire["blastRadius"])
	}
	named, ok := blast[0].(map[string]any)
	if !ok || named["id"] != "app-container:7" || named["name"] != "heimdall" {
		t.Fatalf("named blast radius entry = %#v", blast[0])
	}
	plan, ok := wire["plan"].(map[string]any)
	if !ok || plan["planHash"] != "sha256:stable-plan-identity" {
		t.Fatalf("plan identity changed in read projection: %#v", wire["plan"])
	}
	radius, ok := plan["predictedBlastRadius"].([]any)
	if !ok || len(radius) != 2 || radius[0] != "app-container:7" {
		t.Fatalf("persisted blast radius IDs changed: %#v", plan["predictedBlastRadius"])
	}
}

// Without a registry the projection must still return every blast-radius ID
// so the review surface can fall back to raw identifiers.
func TestProjectActionAuditFallsBackToRawBlastRadiusIDsWithoutRegistry(t *testing.T) {
	record := unifiedresources.ActionAuditRecord{
		ID: "action-no-registry",
		Request: unifiedresources.ActionRequest{
			ResourceID:     "app-container:7",
			CapabilityName: "update",
		},
		Plan: unifiedresources.ActionPlan{
			PredictedBlastRadius: []string{"app-container:7", "docker-network:9"},
		},
	}
	projection := projectActionAudit(record, nil)
	if projection.Resource != nil {
		t.Fatalf("resource presentation resolved without a registry: %#v", projection.Resource)
	}
	if projection.CapabilityAutoAuthorization != "" {
		t.Fatalf("capability class resolved without a registry: %q", projection.CapabilityAutoAuthorization)
	}
	if len(projection.BlastRadius) != 2 || projection.BlastRadius[0].ID != "app-container:7" || projection.BlastRadius[1].ID != "docker-network:9" {
		t.Fatalf("blast radius fallback = %#v", projection.BlastRadius)
	}
	if projection.BlastRadius[0].Name != "" {
		t.Fatalf("unresolvable entry gained a name: %#v", projection.BlastRadius[0])
	}
}
