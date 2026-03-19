package unifiedresources

import "testing"

func TestRefreshPolicyMetadata_ClassifiesRestrictedResources(t *testing.T) {
	resource := Resource{
		ID:     "vm-100",
		Name:   "customer-payments",
		Type:   ResourceTypeVM,
		Status: StatusOnline,
		Tags:   []string{"customer-data"},
		Identity: ResourceIdentity{
			Hostnames:   []string{"payments.internal"},
			IPAddresses: []string{"10.0.0.44"},
		},
		Canonical: &CanonicalIdentity{
			PlatformID: "payments.internal",
			PrimaryID:  "vm:100",
			Aliases:    []string{"vm-100"},
		},
	}

	RefreshPolicyMetadata(&resource)

	if resource.Policy == nil {
		t.Fatal("expected policy metadata")
	}
	if got := resource.Policy.Sensitivity; got != ResourceSensitivityRestricted {
		t.Fatalf("sensitivity = %q, want %q", got, ResourceSensitivityRestricted)
	}
	if got := resource.Policy.Routing.Scope; got != ResourceRoutingScopeLocalOnly {
		t.Fatalf("routing scope = %q, want %q", got, ResourceRoutingScopeLocalOnly)
	}
	if resource.Policy.Routing.AllowCloudSummary {
		t.Fatal("expected local-only resource to block cloud summary")
	}
	if !containsRedactionHint(resource.Policy.Routing.Redact, ResourceRedactionHostname) {
		t.Fatalf("expected hostname redaction hint, got %#v", resource.Policy.Routing.Redact)
	}
	if !containsRedactionHint(resource.Policy.Routing.Redact, ResourceRedactionIPAddress) {
		t.Fatalf("expected IP redaction hint, got %#v", resource.Policy.Routing.Redact)
	}
	if resource.AISafeSummary == "" {
		t.Fatal("expected aiSafeSummary")
	}
	if resource.AISafeSummary == resource.Name {
		t.Fatalf("aiSafeSummary leaked raw name: %q", resource.AISafeSummary)
	}
}

func TestRefreshPolicyMetadata_ClassifiesInfrastructureAsInternal(t *testing.T) {
	resource := Resource{
		ID:     "agent-1",
		Name:   "pve-node",
		Type:   ResourceTypeAgent,
		Status: StatusOnline,
		Agent: &AgentData{
			Hostname: "pve-node",
		},
	}

	RefreshCanonicalIdentity(&resource)
	RefreshPolicyMetadata(&resource)

	if resource.Policy == nil {
		t.Fatal("expected policy metadata")
	}
	if got := resource.Policy.Sensitivity; got != ResourceSensitivityInternal {
		t.Fatalf("sensitivity = %q, want %q", got, ResourceSensitivityInternal)
	}
	if got := resource.Policy.Routing.Scope; got != ResourceRoutingScopeCloudSummary {
		t.Fatalf("routing scope = %q, want %q", got, ResourceRoutingScopeCloudSummary)
	}
	if !resource.Policy.Routing.AllowCloudSummary {
		t.Fatal("expected internal resource to allow cloud summary")
	}
	if resource.Policy.Routing.AllowCloudRawSignals {
		t.Fatal("expected raw signals to remain local")
	}
}

func TestResourcePolicyPresentationLabels(t *testing.T) {
	if got := ResourceSensitivityLabel(ResourceSensitivityPublic); got != "Public" {
		t.Fatalf("sensitivity label = %q, want %q", got, "Public")
	}
	if got := ResourceRoutingScopeLabel(ResourceRoutingScopeLocalOnly); got != "Local Only" {
		t.Fatalf("routing label = %q, want %q", got, "Local Only")
	}
	if got := ResourceRedactionHintLabel(ResourceRedactionIPAddress); got != "IP Address" {
		t.Fatalf("redaction label = %q, want %q", got, "IP Address")
	}

	policy := &ResourcePolicy{
		Routing: ResourceRoutingPolicy{
			Redact: []ResourceRedactionHint{
				ResourceRedactionPath,
				ResourceRedactionHostname,
			},
		},
	}
	got := ResourcePolicyRedactionLabels(policy)
	if len(got) != 2 || got[0] != "Hostname" || got[1] != "Path" {
		t.Fatalf("redaction labels = %#v, want [Hostname Path]", got)
	}
}

func containsRedactionHint(hints []ResourceRedactionHint, want ResourceRedactionHint) bool {
	for _, hint := range hints {
		if hint == want {
			return true
		}
	}
	return false
}
