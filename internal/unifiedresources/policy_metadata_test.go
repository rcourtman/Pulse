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

func TestResourcePolicyRedactionLabelsUseCanonicalOrder(t *testing.T) {
	policy := &ResourcePolicy{
		Routing: ResourceRoutingPolicy{
			Redact: []ResourceRedactionHint{
				ResourceRedactionAlias,
				ResourceRedactionPath,
				ResourceRedactionHostname,
				ResourceRedactionPlatformID,
			},
		},
	}

	got := ResourcePolicyRedactionLabels(policy)
	want := []string{"Hostname", "Platform ID", "Alias", "Path"}
	if len(got) != len(want) {
		t.Fatalf("redaction labels = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("redaction labels = %#v, want %#v", got, want)
		}
	}
}

func TestResourcePolicyCountSummariesUseCanonicalOrder(t *testing.T) {
	sensitivityCounts := map[ResourceSensitivity]int{
		ResourceSensitivityRestricted: 1,
		ResourceSensitivityPublic:     2,
		ResourceSensitivitySensitive:  3,
		ResourceSensitivityInternal:   4,
	}
	if got := ResourcePolicySensitivitySummaryFromCounts(sensitivityCounts); len(got) != 4 || got[0] != "2 Public" || got[1] != "4 Internal" || got[2] != "3 Sensitive" || got[3] != "1 Restricted" {
		t.Fatalf("sensitivity summary = %#v, want [2 Public 4 Internal 3 Sensitive 1 Restricted]", got)
	}

	routingCounts := map[ResourceRoutingScope]int{
		ResourceRoutingScopeLocalOnly:    5,
		ResourceRoutingScopeCloudSummary: 6,
		ResourceRoutingScopeLocalFirst:   7,
	}
	if got := ResourcePolicyRoutingSummaryFromCounts(routingCounts); len(got) != 3 || got[0] != "6 Cloud Summary" || got[1] != "7 Local First" || got[2] != "5 Local Only" {
		t.Fatalf("routing summary = %#v, want [6 Cloud Summary 7 Local First 5 Local Only]", got)
	}
}

func TestResourcePolicySummaryLines(t *testing.T) {
	policy := &ResourcePolicy{
		Sensitivity: ResourceSensitivityRestricted,
		Routing: ResourceRoutingPolicy{
			Scope:                ResourceRoutingScopeLocalOnly,
			AllowCloudSummary:    false,
			AllowCloudRawSignals: false,
			Redact: []ResourceRedactionHint{
				ResourceRedactionAlias,
				ResourceRedactionHostname,
				ResourceRedactionPath,
			},
		},
	}

	got := ResourcePolicySummaryLines(policy)
	want := []string{
		"Policy: sensitivity=Restricted, routing=Local Only, cloud_summary=false, cloud_raw_signals=false",
		"Redactions: Hostname, Alias, Path",
	}
	if len(got) != len(want) {
		t.Fatalf("summary lines = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("summary line[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResourcePolicyRedactsAndUsesAISafeSummary(t *testing.T) {
	policy := &ResourcePolicy{
		Sensitivity: ResourceSensitivitySensitive,
		Routing: ResourceRoutingPolicy{
			Scope:                ResourceRoutingScopeLocalFirst,
			AllowCloudSummary:    true,
			AllowCloudRawSignals: false,
			Redact: []ResourceRedactionHint{
				ResourceRedactionHostname,
				ResourceRedactionIPAddress,
			},
		},
	}

	if !ResourcePolicyRedacts(policy, ResourceRedactionHostname) {
		t.Fatal("expected hostname to be redacted")
	}
	if ResourcePolicyRedacts(policy, ResourceRedactionAlias) {
		t.Fatal("did not expect alias to be redacted")
	}
	if !ResourcePolicyUsesAISafeSummary("resource summary safe for remote AI use", policy) {
		t.Fatal("expected AI-safe summary to be used")
	}
	if ResourcePolicyUsesAISafeSummary("", policy) {
		t.Fatal("did not expect empty summary to be used")
	}
	if ResourcePolicyUsesAISafeSummary("resource summary safe for remote AI use", nil) {
		t.Fatal("did not expect nil policy to use AI-safe summary")
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
