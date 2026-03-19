package unifiedresources

import "testing"

func TestSummarizePolicyPosture(t *testing.T) {
	t.Parallel()

	resources := []Resource{
		{
			ID:   "public-1",
			Name: "public-vm",
			Type: ResourceTypeVM,
			Tags: []string{"public"},
		},
		{
			ID:     "internal-1",
			Name:   "agent-1",
			Type:   ResourceTypeAgent,
			Agent:  &AgentData{Hostname: "agent-1"},
			Status: StatusOnline,
		},
		{
			ID:     "sensitive-1",
			Name:   "db-1",
			Type:   ResourceTypeVM,
			Status: StatusOnline,
			Identity: ResourceIdentity{
				Hostnames:   []string{"db.internal"},
				IPAddresses: []string{"10.0.0.10"},
			},
			Canonical: &CanonicalIdentity{
				PlatformID: "db.internal",
				Aliases:    []string{"db-1"},
			},
		},
		{
			ID:     "restricted-1",
			Name:   "mail-gw",
			Type:   ResourceTypePMG,
			Status: StatusWarning,
			PMG:    &PMGData{Hostname: "mail.internal"},
		},
	}

	for i := range resources {
		RefreshPolicyMetadata(&resources[i])
	}

	summary := SummarizePolicyPosture(resources)
	if summary == nil {
		t.Fatal("expected posture summary")
	}
	if summary.TotalResources != 4 {
		t.Fatalf("total resources = %d, want 4", summary.TotalResources)
	}
	if got := summary.SensitivityCounts[ResourceSensitivityPublic]; got != 1 {
		t.Fatalf("public sensitivity count = %d, want 1", got)
	}
	if got := summary.SensitivityCounts[ResourceSensitivityInternal]; got != 1 {
		t.Fatalf("internal sensitivity count = %d, want 1", got)
	}
	if got := summary.SensitivityCounts[ResourceSensitivitySensitive]; got != 1 {
		t.Fatalf("sensitive sensitivity count = %d, want 1", got)
	}
	if got := summary.SensitivityCounts[ResourceSensitivityRestricted]; got != 1 {
		t.Fatalf("restricted sensitivity count = %d, want 1", got)
	}
	if got := summary.RoutingCounts[ResourceRoutingScopeCloudSummary]; got != 2 {
		t.Fatalf("cloud summary routing count = %d, want 2", got)
	}
	if got := summary.RoutingCounts[ResourceRoutingScopeLocalFirst]; got != 1 {
		t.Fatalf("local first routing count = %d, want 1", got)
	}
	if got := summary.RoutingCounts[ResourceRoutingScopeLocalOnly]; got != 1 {
		t.Fatalf("local only routing count = %d, want 1", got)
	}
	if got := summary.RedactionCounts[ResourceRedactionHostname]; got == 0 {
		t.Fatal("expected hostname redaction count")
	}
}
